package server

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"sort"
	"time"

	"github.com/walkerkiller/coursive/daemon/internal/library"
	"github.com/walkerkiller/coursive/daemon/internal/lms"
	"github.com/walkerkiller/coursive/daemon/internal/sources"
	"github.com/walkerkiller/coursive/daemon/internal/store"
)

// Learning-platform data from sources that announce lms: courses,
// assignments, announcements and files, plus links to library courses.

func (s *Server) lmsRoutes(h func(string, func(http.ResponseWriter, *http.Request) (any, error))) {
	h("GET /api/lms", func(http.ResponseWriter, *http.Request) (any, error) {
		return map[string]any{"sources": s.LMS.Status()}, nil
	})
	h("POST /api/lms/sync", func(http.ResponseWriter, *http.Request) (any, error) {
		if len(s.lmsSources()) == 0 {
			return nil, lmsErr(lms.ErrNoSource)
		}
		go s.lmsSync(s.Ctx)
		return ok(), nil
	})
	h("GET /api/lms/course", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		q := r.URL.Query()
		d, err := s.LMS.Detail(q.Get("source"), q.Get("id"))
		return d, lmsErr(err)
	})
	h("GET /api/lms/files", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		q := r.URL.Query()
		fs, err := s.LMS.Files(r.Context(), q.Get("source"), q.Get("course"))
		if err != nil {
			return nil, lmsErr(err)
		}
		// Addresses and their headers stay in the daemon.
		type file struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			Folder    string `json:"folder,omitempty"`
			Size      int64  `json:"size,omitempty"`
			UpdatedAt string `json:"updated_at,omitempty"`
		}
		out := make([]file, len(fs))
		for i, f := range fs {
			out[i] = file{f.ID, f.Name, f.Folder, f.Size, f.UpdatedAt}
		}
		return out, nil
	})
	h("POST /api/lms/download", s.lmsDownload)
	h("PUT /api/lms/link", s.lmsSetLink)
}

// lmsSources lists the signed-in sources that announce lms.
func (s *Server) lmsSources() []lms.Source {
	var out []lms.Source
	if s.Sources == nil {
		return nil
	}
	for _, p := range s.Sources.SignedIn() {
		if p.Has(sources.CapLMS) {
			out = append(out, lms.Source{ID: p.ID, Title: p.Title, Platform: p})
		}
	}
	return out
}

func lmsErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, lms.ErrNotFound):
		return apiError{http.StatusNotFound, err.Error()}
	case errors.Is(err, lms.ErrNoSource):
		return apiError{http.StatusConflict, err.Error()}
	}
	return err
}

// lmsSync syncs every learning platform and links its courses to library courses by name.
func (s *Server) lmsSync(ctx context.Context) error {
	if len(s.lmsSources()) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	s.lmsLink(ctx)
	err := s.LMS.Sync(ctx)
	if s.lmsLink(ctx) {
		s.Bus.Publish("schedule_changed", map[string]string{"id": "lms"})
	}
	return err
}

func (s *Server) lmsLink(ctx context.Context) bool {
	cs, err := s.St.Courses(ctx)
	if err != nil {
		return false
	}
	refs := make([]lms.Ref, 0, len(cs))
	for _, c := range cs {
		t := c.Title
		if c.Alias != "" {
			t = c.Alias
		}
		refs = append(refs, lms.Ref{ID: c.ID, Title: t})
	}
	return s.LMS.AutoLink(refs)
}

func (s *Server) lmsSetLink(_ http.ResponseWriter, r *http.Request) (any, error) {
	var in struct {
		Source        string `json:"source"`
		Course        string `json:"course"`
		LibraryCourse string `json:"library_course"`
		Auto          bool   `json:"auto"`
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	if in.LibraryCourse != "" {
		if _, err := s.St.Course(r.Context(), in.LibraryCourse); err != nil {
			return nil, badRequest("资料库中没有这门课程")
		}
	}
	if err := s.LMS.SetLink(in.Source, in.Course, in.LibraryCourse, in.Auto); err != nil {
		return nil, lmsErr(err)
	}
	if in.Auto {
		s.lmsLink(r.Context())
	}
	s.Bus.Publish("lms_changed", map[string]string{"what": "links", "source": in.Source})
	s.Bus.Publish("schedule_changed", map[string]string{"id": "lms"})
	return map[string]any{"sources": s.LMS.Status()}, nil
}

// lmsDownload saves a file into the linked library course's folder
// (<course>/学习平台/), or 学习平台/<platform course>/ when not linked.
func (s *Server) lmsDownload(_ http.ResponseWriter, r *http.Request) (any, error) {
	ctx := r.Context()
	var in struct {
		Source string `json:"source"`
		Course string `json:"course"`
		File   string `json:"file"`
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	name := s.LMS.CourseName(in.Source, in.Course)
	if name == "" {
		return nil, lmsErr(lms.ErrNotFound)
	}
	rel := "学习平台/" + library.Sanitize(name, "课程-"+in.Course)
	if c := s.LMS.LinkedCourse(in.Source, in.Course); c != "" {
		if dir, err := s.Lib.EnsureCourse(ctx, c); err == nil {
			rel = dir + "/学习平台"
		}
	}
	// Finish the file even if the app stops waiting.
	dctx, cancel := context.WithTimeout(s.Ctx, 15*time.Minute)
	defer cancel()
	p, err := s.LMS.Download(dctx, in.Source, in.Course, in.File, s.Lib.Abs(rel))
	if err != nil {
		return nil, lmsErr(err)
	}
	return map[string]string{"path": rel + "/" + filepath.Base(p)}, nil
}

// ---- schedule merge ----

// lmsItems returns learning-platform assignments as read-only schedule entries.
func (s *Server) lmsItems(ctx context.Context, q store.ScheduleQuery) []store.ScheduleItem {
	if s.LMS == nil || q.LectureID != "" {
		return nil
	}
	titles := map[string]string{}
	if cs, err := s.St.Courses(ctx); err == nil {
		for _, c := range cs {
			titles[c.ID] = c.Title
			if c.Alias != "" {
				titles[c.ID] = c.Alias
			}
		}
	}
	var out []store.ScheduleItem
	for _, it := range s.LMS.Items() {
		switch {
		case q.From != "" && it.DueDate < q.From, q.To != "" && it.DueDate > q.To,
			q.CourseID != "" && it.CourseID != q.CourseID, q.Open && it.Done:
			continue
		}
		title := titles[it.CourseID]
		if title == "" {
			title = it.CourseName
		}
		out = append(out, store.ScheduleItem{ID: it.ID, Kind: "homework", Title: it.Title, Detail: it.Detail, DueDate: it.DueDate,
			DueTime: it.DueTime, CourseID: it.CourseID, TMs: -1, Source: "lms", Done: it.Done, CreatedAt: it.UpdatedAt,
			UpdatedAt: it.UpdatedAt, CourseTitle: title, URL: it.URL})
	}
	return out
}

func mergeSchedule(a, b []store.ScheduleItem) []store.ScheduleItem {
	if len(b) == 0 {
		return a
	}
	out := append(a, b...)
	sort.SliceStable(out, func(i, j int) bool {
		x, y := out[i], out[j]
		if x.DueDate != y.DueDate {
			return x.DueDate < y.DueDate
		}
		if (x.DueTime == "") != (y.DueTime == "") {
			return x.DueTime == ""
		}
		if x.DueTime != y.DueTime {
			return x.DueTime < y.DueTime
		}
		return x.CreatedAt < y.CreatedAt
	})
	return out
}

func (s *Server) lmsScheduleItem(ctx context.Context, id string) (store.ScheduleItem, error) {
	for _, it := range s.lmsItems(ctx, store.ScheduleQuery{}) {
		if it.ID == id {
			return it, nil
		}
	}
	return store.ScheduleItem{}, lmsErr(lms.ErrNotFound)
}

// patchLMSItem changes only local state of an assignment: done, or
// hidden: false to undo a delete.
func (s *Server) patchLMSItem(r *http.Request, id string) (any, error) {
	var p map[string]any
	if err := decode(r, &p); err != nil {
		return nil, err
	}
	done, isDone := p["done"].(bool)
	hidden, isHidden := p["hidden"].(bool)
	var err error
	switch {
	case len(p) == 1 && isDone:
		err = s.LMS.SetDone(id, done)
	case len(p) == 1 && isHidden && !hidden:
		err = s.LMS.Unhide(id)
	default:
		return nil, badRequest("学习平台的作业只能标记完成；其他内容请在学习平台中修改")
	}
	if err != nil {
		return nil, lmsErr(err)
	}
	s.Bus.Publish("schedule_changed", map[string]string{"id": id})
	return s.lmsScheduleItem(r.Context(), id)
}
