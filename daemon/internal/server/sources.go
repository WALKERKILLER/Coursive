package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/walkerkiller/coursive/daemon/internal/lms"
	"github.com/walkerkiller/coursive/daemon/internal/sources"
	"github.com/walkerkiller/coursive/daemon/internal/store"
)

// Source plugins: installing them from a folder, enabling them, and signing
// each source in. Courses and lectures from a source are synced into the
// index with the source's id.
// Contract: docs/specs/0024-source-provider-plugins.md

func (s *Server) sourceRoutes(h func(string, func(http.ResponseWriter, *http.Request) (any, error))) {
	h("GET /api/sources", func(http.ResponseWriter, *http.Request) (any, error) {
		return map[string]any{"plugins": s.Sources.Plugins(), "platform": sources.Platform()}, nil
	})
	h("POST /api/sources/preview", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		from, err := pluginFolder(r)
		if err != nil {
			return nil, err
		}
		return s.Sources.Preview(from)
	})
	h("POST /api/sources/install", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		from, err := pluginFolder(r)
		if err != nil {
			return nil, err
		}
		in, err := s.Sources.Install(from)
		return s.started(r.Context(), in, err)
	})
	h("GET /api/sources/market", func(http.ResponseWriter, *http.Request) (any, error) {
		ls, err := s.Market.List()
		if err != nil {
			return nil, err
		}
		return map[string]any{"plugins": ls, "configured": s.Market.Index != "", "platform": sources.Platform()}, nil
	})
	h("POST /api/sources/market/{plugin}/stage", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
		defer cancel()
		return s.Market.Stage(ctx, r.PathValue("plugin"))
	})
	h("POST /api/sources/market/install", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		var in struct {
			Token string `json:"token"`
		}
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		p, err := s.Market.Install(in.Token)
		return s.started(r.Context(), p, err)
	})
	h("PUT /api/sources/{plugin}", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		var in struct {
			Enabled bool `json:"enabled"`
		}
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		return s.Sources.SetEnabled(r.PathValue("plugin"), in.Enabled)
	})
	h("DELETE /api/sources/{plugin}", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		id := r.PathValue("plugin")
		var forget []string
		for _, p := range s.Sources.Plugins() {
			if p.ID == id {
				for _, src := range p.Sources {
					forget = append(forget, src.ID)
				}
			}
		}
		if err := s.Sources.Uninstall(id); err != nil {
			return nil, err
		}
		for _, src := range forget {
			s.LMS.Forget(src)
		}
		s.relayed.reset()
		return ok(), nil
	})
	h("POST /api/sources/{plugin}/{source}/sign-in", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		var in struct {
			Fields map[string]string `json:"fields"`
		}
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		// Some platforms sign in through several services in turn.
		ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
		defer cancel()
		a, err := s.Sources.SignIn(ctx, sourceID(r), in.Fields)
		return s.signedIn(a, err)
	})
	h("POST /api/sources/{plugin}/{source}/answer", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		var in struct {
			Challenge string `json:"challenge"`
			Answer    string `json:"answer"`
		}
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
		defer cancel()
		a, err := s.Sources.Answer(ctx, sourceID(r), in.Challenge, strings.TrimSpace(in.Answer))
		return s.signedIn(a, err)
	})
	h("POST /api/sources/{plugin}/{source}/restart", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		p, err := s.Sources.Provider(sourceID(r))
		if err != nil {
			return nil, err
		}
		p.Restart()
		var a sources.Auth
		if err := p.Call(r.Context(), "auth.status", struct{}{}, &a); err != nil {
			return nil, err
		}
		return p.Snapshot(), nil
	})
	h("POST /api/sources/{plugin}/{source}/sign-out", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		s.relayed.reset()
		return s.Sources.SignOut(r.Context(), sourceID(r))
	})
}

// started asks a newly installed plugin's sources for their sign-in state,
// which starts them, so the reply already carries their sign-in fields.
func (s *Server) started(ctx context.Context, in sources.PluginInfo, err error) (any, error) {
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	s.Sources.RefreshPlugin(ctx, in.ID)
	for _, p := range s.Sources.Plugins() {
		if p.ID == in.ID {
			return p, nil
		}
	}
	return in, nil
}

func sourceID(r *http.Request) string { return r.PathValue("plugin") + "/" + r.PathValue("source") }

// pluginFolder reads the folder a plugin is installed from.
func pluginFolder(r *http.Request) (string, error) {
	var in struct {
		Path string `json:"path"`
	}
	if err := decode(r, &in); err != nil {
		return "", err
	}
	if strings.TrimSpace(in.Path) == "" {
		return "", badRequest("path 必填")
	}
	return in.Path, nil
}

func (s *Server) signedIn(a sources.Auth, err error) (any, error) {
	if err != nil {
		return nil, err
	}
	if a.State == sources.SignedIn {
		go func() {
			s.syncCourses(s.Ctx)
			s.lmsSync(s.Ctx)
		}()
	}
	return a, nil
}

// source returns the provider that owns a course.
func (s *Server) source(ctx context.Context, courseID string) (*sources.Provider, error) {
	if store.IsLocalCourse(courseID) {
		return nil, badRequest("本地课程没有来源")
	}
	return s.router().For(ctx, courseID)
}

func (s *Server) router() sources.Router {
	return sources.Router{Registry: s.Sources, SourceOf: func(ctx context.Context, course string) (string, error) {
		c, err := s.St.Course(ctx, course)
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return c.Source, err
	}}
}

// refreshable reports whether a course's source can still refresh it.
func (s *Server) refreshable(c store.Course) bool {
	if store.IsLocalCourse(c.ID) {
		return false
	}
	_, err := s.Sources.ForCourse(c.Source)
	return err == nil
}

// syncCourses stores the courses of every signed-in source.
func (s *Server) syncCourses(ctx context.Context) error {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()
	var errs []error
	n := 0
	for _, p := range s.Sources.SignedIn() {
		cs, err := p.Courses(ctx)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s：%w", p.Title, err))
			continue
		}
		for _, c := range cs {
			teachers, _ := json.Marshal(c.Teachers)
			if c.Teachers == nil {
				teachers = []byte("[]")
			}
			if err := s.St.UpsertCourse(ctx, store.Course{ID: c.ID, Source: p.ID, URL: c.URL, Title: c.Title, Teacher: c.Teacher,
				Teachers: teachers, College: c.College, Term: c.Term, Thumb: c.Thumb, StartAt: c.StartAt, EndAt: c.EndAt,
				LectureTotal: c.LectureTotal, Progress: c.Progress, LastSubID: c.LastLectureID, LastLearnAt: c.LastLearnAt}); err != nil {
				return err
			}
		}
		n += len(cs)
	}
	s.lastCourse = time.Now()
	s.Bus.Publish("courses_changed", map[string]int{"count": n})
	return errors.Join(errs...)
}

// syncLectures stores a course's lectures from its source.
func (s *Server) syncLectures(ctx context.Context, courseID string) error {
	if store.IsLocalCourse(courseID) {
		return nil
	}
	p, err := s.source(ctx, courseID)
	if err != nil {
		return err
	}
	ls, err := p.Lectures(ctx, courseID)
	if err != nil {
		return err
	}
	out := make([]store.Lecture, 0, len(ls))
	for _, l := range ls {
		out = append(out, store.Lecture{ID: l.ID, CourseID: courseID, Title: l.Title, Date: l.Date, BeginAt: l.BeginAt,
			EndAt: l.EndAt, Lecturer: l.Lecturer, Room: l.Room, HasPlayback: l.HasPlayback, URL: l.URL})
	}
	return s.St.UpsertLectures(ctx, out)
}

// sourceState summarises the sources for health and the event stream's hello.
func (s *Server) sourceState() map[string]any {
	installed, signed := 0, 0
	for _, p := range s.Sources.Providers() {
		installed++
		if p.SignedIn() {
			signed++
		}
	}
	return map[string]any{"installed": installed, "signed_in": signed}
}

// Source returns the provider that owns a course, for the job manager.
func (s *Server) Source(ctx context.Context, courseID string) (*sources.Provider, error) {
	return s.source(ctx, courseID)
}

// LMSSources lists the signed-in learning platforms, for the lms service.
func (s *Server) LMSSources() []lms.Source { return s.lmsSources() }
