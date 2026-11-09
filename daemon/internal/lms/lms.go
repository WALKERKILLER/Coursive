// Package lms keeps learning-platform data from every signed-in source that
// announces the lms capability: a cache synced every 30 minutes and on
// demand, notifications for new assignments, near deadlines and new
// announcements, assignment due dates for the schedule, local done and hidden
// state, links between platform courses and library courses, and file
// downloads into the linked course. Platform APIs and sign-in live in the
// source providers.
package lms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/walkerkiller/coursive/daemon/internal/notify"
	"github.com/walkerkiller/coursive/daemon/internal/sources"
)

const (
	dueSoon   = 24 * time.Hour
	annWindow = 30 * 24 * time.Hour
	// Every is how often the daemon syncs learning platforms.
	Every = 30 * time.Minute
)

// Platform is a source's learning-platform data; *sources.Provider implements it.
type Platform interface {
	LMSCourses(ctx context.Context) ([]sources.LMSCourse, error)
	Assignments(ctx context.Context, course string) ([]sources.Assignment, error)
	Announcements(ctx context.Context, courses []string, since time.Time) ([]sources.Announcement, error)
	LMSFiles(ctx context.Context, course string) ([]sources.File, error)
	LMSFileAddress(ctx context.Context, f sources.File) (sources.Address, error)
}

// Source is one signed-in source with a learning platform.
type Source struct {
	ID       string
	Title    string
	Platform Platform
}

// Link ties a platform course to a library course; "" means not linked.
type Link struct {
	Course string `json:"course"`
	Manual bool   `json:"manual"`
}

type platformState struct {
	Title         string                          `json:"title"`
	Courses       []sources.LMSCourse             `json:"courses"`
	Assignments   map[string][]sources.Assignment `json:"assignments"`
	Announcements []sources.Announcement          `json:"announcements"`
	SyncedAt      int64                           `json:"synced_at"`
	Seen          map[string]bool                 `json:"seen,omitempty"`
	Links         map[string]Link                 `json:"links,omitempty"`
	Done          map[string]bool                 `json:"done,omitempty"`
	Hidden        map[string]bool                 `json:"hidden,omitempty"`
}

type stateFile struct {
	Version int                       `json:"version"`
	Sources map[string]*platformState `json:"sources"`
}

// Service is the learning-platform layer.
type Service struct {
	// Path is the state file (lms.json in the data directory).
	Path    string
	Sources func() []Source
	Notify  func(notify.Item) bool
	Publish func(kind string, data any)
	// Fetch downloads an address to a file.
	Fetch func(ctx context.Context, a sources.Address, dest string) error

	mu      sync.Mutex
	st      stateFile
	loaded  bool
	syncing map[string]bool
	errs    map[string]string
}

var (
	ErrNotFound = errors.New("没有这项学习平台内容")
	// ErrNoSource means no signed-in source offers a learning platform.
	ErrNoSource = errors.New("没有已登录的学习平台来源")
)

func (s *Service) loadLocked() {
	if s.loaded {
		return
	}
	s.loaded = true
	s.st = stateFile{Version: 1, Sources: map[string]*platformState{}}
	if b, err := os.ReadFile(s.Path); err == nil {
		json.Unmarshal(b, &s.st)
	}
	if s.st.Sources == nil {
		s.st.Sources = map[string]*platformState{}
	}
	s.syncing, s.errs = map[string]bool{}, map[string]string{}
}

func (s *Service) saveLocked() {
	b, _ := json.MarshalIndent(s.st, "", "  ")
	tmp := s.Path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		slog.Warn("lms: save failed", "err", err)
		return
	}
	os.Rename(tmp, s.Path)
}

func (s *Service) publish(kind string, data any) {
	if s.Publish != nil {
		s.Publish(kind, data)
	}
}

func (s *Service) platformLocked(id string) *platformState {
	ps := s.st.Sources[id]
	if ps == nil {
		ps = &platformState{}
		s.st.Sources[id] = ps
	}
	return ps
}

func (s *Service) source(id string) (Source, bool) {
	if s.Sources == nil {
		return Source{}, false
	}
	for _, src := range s.Sources() {
		if src.ID == id {
			return src, true
		}
	}
	return Source{}, false
}

// ---- sync ----

// Sync refreshes every signed-in learning platform; errors are recorded per source.
func (s *Service) Sync(ctx context.Context) error {
	if s.Sources == nil {
		return ErrNoSource
	}
	srcs := s.Sources()
	if len(srcs) == 0 {
		return ErrNoSource
	}
	var errs []error
	for _, src := range srcs {
		if err := s.syncOne(ctx, src); err != nil {
			errs = append(errs, fmt.Errorf("%s：%w", src.Title, err))
		}
	}
	return errors.Join(errs...)
}

func (s *Service) syncOne(ctx context.Context, src Source) error {
	s.mu.Lock()
	s.loadLocked()
	if s.syncing[src.ID] {
		s.mu.Unlock()
		return nil
	}
	s.syncing[src.ID] = true
	first := s.platformLocked(src.ID).SyncedAt == 0
	s.mu.Unlock()
	s.publish("lms_changed", map[string]string{"what": "syncing", "source": src.ID})

	err := func() error {
		cs, err := src.Platform.LMSCourses(ctx)
		if err != nil {
			return err
		}
		as := map[string][]sources.Assignment{}
		ids := make([]string, 0, len(cs))
		for _, co := range cs {
			a, err := src.Platform.Assignments(ctx, co.ID)
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				continue // a course can hide its assignments from students
			}
			as[co.ID] = a
			ids = append(ids, co.ID)
		}
		anns, err := src.Platform.Announcements(ctx, ids, time.Now().Add(-annWindow))
		if err != nil && ctx.Err() != nil {
			return ctx.Err()
		}
		s.mu.Lock()
		ps := s.platformLocked(src.ID)
		changed := assignmentsChanged(ps.Assignments, as)
		ps.Title, ps.Courses, ps.Assignments = src.Title, cs, as
		if err == nil {
			ps.Announcements = anns
		}
		ps.SyncedAt = time.Now().UnixMilli()
		items := s.newsLocked(src.ID, ps, first)
		s.saveLocked()
		s.mu.Unlock()
		for _, it := range items {
			if s.Notify != nil {
				s.Notify(it)
			}
		}
		if changed {
			s.publish("schedule_changed", map[string]string{"id": "lms"})
		}
		return nil
	}()
	s.mu.Lock()
	s.syncing[src.ID] = false
	if err != nil {
		s.errs[src.ID] = err.Error()
	} else {
		delete(s.errs, src.ID)
	}
	s.mu.Unlock()
	s.publish("lms_changed", map[string]string{"what": "synced", "source": src.ID})
	return err
}

func assignmentsChanged(a, b map[string][]sources.Assignment) bool {
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	return string(ja) != string(jb)
}

var tagRe = regexp.MustCompile(`<[^>]*>`)

// Plain turns an HTML body into at most n runes of text.
func Plain(html string, n int) string {
	t := strings.Join(strings.Fields(tagRe.ReplaceAllString(html, " ")), " ")
	t = strings.NewReplacer("&nbsp;", " ", "&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`).Replace(t)
	if utf8.RuneCountInString(t) > n {
		t = string([]rune(t)[:n]) + "…"
	}
	return t
}

func itemID(source, assignment string) string { return "lms:" + source + ":" + assignment }

// newsLocked lists notifications for unseen assignments, assignments due
// within a day and unseen announcements. The first sync only marks
// everything seen, except deadlines within a day.
func (s *Service) newsLocked(source string, ps *platformState, first bool) []notify.Item {
	if ps.Seen == nil {
		ps.Seen = map[string]bool{}
	}
	seen := map[string]bool{}
	var out []notify.Item
	now := time.Now()
	names := map[string]string{}
	for _, co := range ps.Courses {
		names[co.ID] = co.Title
	}
	mark := func(key string, it notify.Item) {
		seen[key] = true
		if !ps.Seen[key] && !first {
			it.ID, it.At = "lms:"+source+":"+key, now.UnixMilli()
			out = append(out, it)
		}
	}
	for cid, as := range ps.Assignments {
		lib := ps.Links[cid].Course
		for _, a := range as {
			if ps.Hidden[itemID(source, a.ID)] {
				continue
			}
			body := names[cid]
			if a.DueAt != nil {
				body += " · 截止 " + a.DueAt.In(time.Local).Format("1月2日 15:04")
			}
			mark("a:"+a.ID, notify.Item{Kind: "assignment_new", Title: "新作业：" + a.Title, Body: body, CourseID: lib, URL: a.URL})
			if a.DueAt != nil && !doneLocked(ps, source, a) && a.DueAt.After(now) && a.DueAt.Sub(now) < dueSoon {
				key := "due:" + a.ID
				seen[key] = true
				if !ps.Seen[key] {
					out = append(out, notify.Item{ID: "lms:" + source + ":" + key, Kind: "assignment_due", Title: "作业即将截止：" + a.Title,
						Body: body, CourseID: lib, URL: a.URL, At: now.UnixMilli()})
				}
			}
		}
	}
	for _, an := range ps.Announcements {
		mark("n:"+an.ID, notify.Item{Kind: "announcement", Title: "课程公告：" + an.Title,
			Body: names[an.Course] + " · " + Plain(an.HTML, 80), CourseID: ps.Links[an.Course].Course, URL: an.URL})
	}
	ps.Seen = seen
	return out
}

func doneLocked(ps *platformState, source string, a sources.Assignment) bool {
	if v, ok := ps.Done[itemID(source, a.ID)]; ok {
		return v
	}
	return a.Submitted
}

// ---- status ----

// CourseView is a platform course with its link and assignment counts.
type CourseView struct {
	sources.LMSCourse
	LibraryCourse string `json:"library_course"`
	Manual        bool   `json:"manual"`
	// Open counts assignments not yet done with a future due date.
	Open  int `json:"open"`
	Total int `json:"total"`
}

// SourceStatus is one learning platform's state.
type SourceStatus struct {
	Source string `json:"source"`
	Title  string `json:"title"`
	// SignedIn is false for a platform shown only from its cache.
	SignedIn bool         `json:"signed_in"`
	Syncing  bool         `json:"syncing"`
	Error    string       `json:"error,omitempty"`
	SyncedAt int64        `json:"synced_at"`
	Courses  []CourseView `json:"courses"`
}

// Status lists every signed-in learning platform and every cached one.
func (s *Service) Status() []SourceStatus {
	live := map[string]Source{}
	if s.Sources != nil {
		for _, src := range s.Sources() {
			live[src.ID] = src
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	ids := map[string]bool{}
	for id := range live {
		ids[id] = true
	}
	for id := range s.st.Sources {
		ids[id] = true
	}
	now := time.Now()
	out := []SourceStatus{}
	for id := range ids {
		ps := s.platformLocked(id)
		st := SourceStatus{Source: id, Title: ps.Title, Syncing: s.syncing[id], Error: s.errs[id], SyncedAt: ps.SyncedAt, Courses: []CourseView{}}
		if src, ok := live[id]; ok {
			st.SignedIn, st.Title = true, src.Title
		}
		for _, co := range ps.Courses {
			v := CourseView{LMSCourse: co, LibraryCourse: ps.Links[co.ID].Course, Manual: ps.Links[co.ID].Manual}
			for _, a := range ps.Assignments[co.ID] {
				v.Total++
				if !doneLocked(ps, id, a) && a.DueAt != nil && a.DueAt.After(now) {
					v.Open++
				}
			}
			st.Courses = append(st.Courses, v)
		}
		out = append(out, st)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Source < out[j].Source })
	return out
}

// ---- schedule ----

// Item is an assignment as a schedule entry.
type Item struct {
	ID         string
	Title      string
	Detail     string
	DueDate    string
	DueTime    string
	CourseID   string // linked library course
	CourseName string // platform course
	Done       bool
	URL        string
	UpdatedAt  int64
}

// itemLocked builds one assignment's schedule entry. DueDate and DueTime are
// empty when the platform gives no deadline.
func itemLocked(source string, ps *platformState, co sources.LMSCourse, a sources.Assignment) Item {
	detail := co.Title
	if a.Points != nil && *a.Points > 0 {
		detail += fmt.Sprintf(" · %g 分", *a.Points)
	}
	it := Item{ID: itemID(source, a.ID), Title: a.Title, Detail: detail, CourseID: ps.Links[co.ID].Course,
		CourseName: co.Title, Done: doneLocked(ps, source, a), URL: a.URL, UpdatedAt: ps.SyncedAt}
	if a.DueAt != nil {
		d := a.DueAt.In(time.Local)
		it.DueDate, it.DueTime = d.Format("2006-01-02"), d.Format("15:04")
	}
	return it
}

// Items lists assignments with a due date, without hidden ones.
func (s *Service) Items() []Item {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	var out []Item
	for source, ps := range s.st.Sources {
		for _, co := range ps.Courses {
			for _, a := range ps.Assignments[co.ID] {
				if a.DueAt == nil || ps.Hidden[itemID(source, a.ID)] {
					continue
				}
				out = append(out, itemLocked(source, ps, co, a))
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Item returns one assignment's schedule entry even when it has no due date.
// Hidden assignments are not part of the schedule and are reported as missing.
func (s *Service) Item(id string) (Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	source, ps, course, a, ok := s.findLocked(id)
	if !ok || ps.Hidden[id] {
		return Item{}, ErrNotFound
	}
	co := sources.LMSCourse{ID: course, Title: course}
	for _, c := range ps.Courses {
		if c.ID == course {
			co = c
			break
		}
	}
	return itemLocked(source, ps, co, a), nil
}

// IsItem reports whether a schedule id belongs to a learning platform.
func IsItem(id string) bool { return strings.HasPrefix(id, "lms:") }

// findLocked locates an assignment by its full namespaced id. Comparing the
// complete id keeps source and assignment ids opaque.
func (s *Service) findLocked(id string) (string, *platformState, string, sources.Assignment, bool) {
	for source, ps := range s.st.Sources {
		for course, as := range ps.Assignments {
			for _, a := range as {
				if itemID(source, a.ID) == id {
					return source, ps, course, a, true
				}
			}
		}
	}
	return "", nil, "", sources.Assignment{}, false
}

// SetDone overrides an assignment's done state locally; the platform is not changed.
func (s *Service) SetDone(id string, done bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	_, ps, _, a, ok := s.findLocked(id)
	if !ok {
		return ErrNotFound
	}
	if ps.Done == nil {
		ps.Done = map[string]bool{}
	}
	if done == a.Submitted {
		delete(ps.Done, id)
	} else {
		ps.Done[id] = done
	}
	s.saveLocked()
	return nil
}

// Hide removes an assignment from the schedule; Unhide puts it back.
func (s *Service) Hide(id string) error { return s.setHidden(id, true) }

func (s *Service) Unhide(id string) error { return s.setHidden(id, false) }

func (s *Service) setHidden(id string, hidden bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	_, ps, _, _, ok := s.findLocked(id)
	if !ok {
		return ErrNotFound
	}
	if ps.Hidden == nil {
		ps.Hidden = map[string]bool{}
	}
	if hidden {
		ps.Hidden[id] = true
	} else {
		delete(ps.Hidden, id)
	}
	s.saveLocked()
	return nil
}

// ---- links ----

// Ref is a library course for automatic linking.
type Ref struct{ ID, Title string }

var bracketRe = regexp.MustCompile(`[(（\[【][^)）\]】]*[)）\]】]`)

func normName(s string) string {
	s = strings.ToLower(bracketRe.ReplaceAllString(s, ""))
	return strings.Map(func(r rune) rune {
		if r <= ' ' || strings.ContainsRune(",，。.、:：;；!！?？\"“”'‘’()（）《》<>-_—·/\\", r) {
			return -1
		}
		return r
	}, s)
}

// AutoLink links unlinked platform courses to the library course with the
// same name; manual links stay.
func (s *Service) AutoLink(refs []Ref) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	changed := false
	for _, ps := range s.st.Sources {
		if ps.Links == nil {
			ps.Links = map[string]Link{}
		}
		for _, co := range ps.Courses {
			if ps.Links[co.ID].Manual {
				continue
			}
			want := match(co.Title, refs)
			if ps.Links[co.ID].Course != want {
				ps.Links[co.ID] = Link{Course: want}
				changed = true
			}
		}
	}
	if changed {
		s.saveLocked()
	}
	return changed
}

// match picks the one library course whose name equals or contains the
// platform name (or the reverse); ambiguous names stay unlinked.
func match(name string, refs []Ref) string {
	n := normName(name)
	if n == "" {
		return ""
	}
	var exact, part []string
	for _, r := range refs {
		m := normName(r.Title)
		switch {
		case m == "":
		case m == n:
			exact = append(exact, r.ID)
		case utf8.RuneCountInString(m) >= 3 && utf8.RuneCountInString(n) >= 3 && (strings.Contains(n, m) || strings.Contains(m, n)):
			part = append(part, r.ID)
		}
	}
	if len(exact) == 1 {
		return exact[0]
	}
	if len(exact) == 0 && len(part) == 1 {
		return part[0]
	}
	return ""
}

// SetLink links a platform course to a library course ("" unlinks); auto
// returns the choice to automatic linking.
func (s *Service) SetLink(source, course, library string, auto bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	ps := s.st.Sources[source]
	found := false
	if ps != nil {
		for _, co := range ps.Courses {
			found = found || co.ID == course
		}
	}
	if !found {
		return ErrNotFound
	}
	if ps.Links == nil {
		ps.Links = map[string]Link{}
	}
	if auto {
		delete(ps.Links, course)
	} else {
		ps.Links[course] = Link{Course: library, Manual: true}
	}
	s.saveLocked()
	return nil
}

// ---- course detail and files ----

// AssignmentView is an assignment with its effective done state.
type AssignmentView struct {
	sources.Assignment
	Done bool `json:"done"`
}

// CourseDetail is a cached course with its assignments and announcements.
type CourseDetail struct {
	Source        string                 `json:"source"`
	Course        CourseView             `json:"course"`
	Assignments   []AssignmentView       `json:"assignments"`
	Announcements []sources.Announcement `json:"announcements"`
}

// Detail returns a cached course, newest deadlines first.
func (s *Service) Detail(source, course string) (CourseDetail, error) {
	var d CourseDetail
	found := false
	for _, st := range s.Status() {
		if st.Source != source {
			continue
		}
		for _, v := range st.Courses {
			if v.ID == course {
				d.Course, found = v, true
			}
		}
	}
	if !found {
		return d, ErrNotFound
	}
	d.Source = source
	s.mu.Lock()
	defer s.mu.Unlock()
	ps := s.st.Sources[source]
	d.Assignments = []AssignmentView{}
	for _, a := range ps.Assignments[course] {
		d.Assignments = append(d.Assignments, AssignmentView{Assignment: a, Done: doneLocked(ps, source, a)})
	}
	sort.SliceStable(d.Assignments, func(i, j int) bool {
		a, b := d.Assignments[i].DueAt, d.Assignments[j].DueAt
		if a == nil || b == nil {
			return b == nil && a != nil
		}
		return a.After(*b)
	})
	d.Announcements = []sources.Announcement{}
	for _, an := range ps.Announcements {
		if an.Course == course {
			d.Announcements = append(d.Announcements, an)
		}
	}
	return d, nil
}

// Files lists a course's files live from the platform.
func (s *Service) Files(ctx context.Context, source, course string) ([]sources.File, error) {
	src, ok := s.source(source)
	if !ok {
		return nil, ErrNoSource
	}
	fs, err := src.Platform.LMSFiles(ctx, course)
	if fs == nil {
		fs = []sources.File{}
	}
	return fs, err
}

// Download saves one file into dir and returns its path. An existing file of
// the same name is kept unless the platform reports another size.
func (s *Service) Download(ctx context.Context, source, course, fileID, dir string) (string, error) {
	src, ok := s.source(source)
	if !ok {
		return "", ErrNoSource
	}
	fs, err := src.Platform.LMSFiles(ctx, course)
	if err != nil {
		return "", err
	}
	for _, f := range fs {
		if f.ID != fileID {
			continue
		}
		path := filepath.Join(dir, SafeName(f.Name, "file-"+f.ID))
		if fi, err := os.Stat(path); err == nil {
			if f.Size <= 0 || fi.Size() == f.Size {
				return path, nil
			}
			os.Remove(path) // the platform has another version
		}
		a, err := src.Platform.LMSFileAddress(ctx, f)
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
		return path, s.Fetch(ctx, a, path)
	}
	return "", ErrNotFound
}

// SafeName makes a platform file name safe for the library.
func SafeName(s, fallback string) string {
	s = strings.Map(func(r rune) rune {
		if r < ' ' || strings.ContainsRune(`/\:*?"<>|`, r) {
			return '_'
		}
		return r
	}, strings.TrimSpace(s))
	s = strings.Trim(s, ". ")
	if s == "" {
		return fallback
	}
	if r := []rune(s); len(r) > 120 {
		ext := []rune(filepath.Ext(s))
		if len(ext) > 16 {
			ext = nil
		}
		s = string(r[:120-len(ext)]) + string(ext)
	}
	return s
}

// CourseName returns a cached platform course name.
func (s *Service) CourseName(source, course string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	if ps := s.st.Sources[source]; ps != nil {
		for _, co := range ps.Courses {
			if co.ID == course {
				return co.Title
			}
		}
	}
	return ""
}

// LinkedCourse returns the library course a platform course is linked to.
func (s *Service) LinkedCourse(source, course string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	if ps := s.st.Sources[source]; ps != nil {
		return ps.Links[course].Course
	}
	return ""
}

// Forget drops a removed source's cache and state.
func (s *Service) Forget(source string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	if _, ok := s.st.Sources[source]; ok {
		delete(s.st.Sources, source)
		s.saveLocked()
	}
}
