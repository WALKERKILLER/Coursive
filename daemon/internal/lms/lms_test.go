package lms

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/walkerkiller/coursive/daemon/internal/notify"
	"github.com/walkerkiller/coursive/daemon/internal/sources"
)

type fakePlatform struct {
	mu      sync.Mutex
	courses []sources.LMSCourse
	as      map[string][]sources.Assignment
	anns    []sources.Announcement
	files   map[string][]sources.File
	fail    error
}

func (f *fakePlatform) LMSCourses(context.Context) ([]sources.LMSCourse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.courses, f.fail
}

func (f *fakePlatform) Assignments(_ context.Context, c string) ([]sources.Assignment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.as[c], nil
}

func (f *fakePlatform) Announcements(context.Context, []string, time.Time) ([]sources.Announcement, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.anns, nil
}

func (f *fakePlatform) LMSFiles(_ context.Context, c string) ([]sources.File, error) {
	return f.files[c], nil
}

func (f *fakePlatform) LMSFileAddress(_ context.Context, fl sources.File) (sources.Address, error) {
	return sources.Address{URL: "https://files.example/" + fl.ID}, nil
}

func at(d time.Duration) *time.Time { t := time.Now().Add(d); return &t }

func newService(t *testing.T, p *fakePlatform) (*Service, *[]notify.Item) {
	var got []notify.Item
	s := &Service{
		Path:    filepath.Join(t.TempDir(), "lms.json"),
		Sources: func() []Source { return []Source{{ID: "demo", Title: "Demo", Platform: p}} },
		Notify:  func(it notify.Item) bool { got = append(got, it); return true },
	}
	return s, &got
}

func demo() *fakePlatform {
	pts := 10.0
	return &fakePlatform{
		courses: []sources.LMSCourse{{ID: "c1", Title: "线性代数（2026春）"}, {ID: "c2", Title: "Physics"}},
		as: map[string][]sources.Assignment{
			"c1": {{ID: "a1", Title: "习题一", DueAt: at(3 * time.Hour), Points: &pts}, {ID: "a2", Title: "习题二", DueAt: at(72 * time.Hour)}},
			"c2": {{ID: "a3", Title: "Lab", DueAt: at(-time.Hour), Submitted: true}},
		},
		anns:  []sources.Announcement{{ID: "n1", Course: "c1", Title: "调课", HTML: "<p>下周&nbsp;停课</p>"}},
		files: map[string][]sources.File{"c1": {{ID: "f1", Name: "讲义/第1章.pdf", Size: 3}}},
	}
}

func kinds(items []notify.Item) string {
	var k []string
	for _, it := range items {
		k = append(k, it.Kind)
	}
	return strings.Join(k, ",")
}

func TestSyncNotifiesOnlyDueSoonFirst(t *testing.T) {
	p := demo()
	s, got := newService(t, p)
	if err := s.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if kinds(*got) != "assignment_due" || !strings.HasPrefix((*got)[0].ID, "lms:demo:due:a1") {
		t.Fatalf("first sync: %+v", *got)
	}
	*got = nil
	p.mu.Lock()
	p.as["c2"] = append(p.as["c2"], sources.Assignment{ID: "a4", Title: "Report", DueAt: at(200 * time.Hour)})
	p.anns = append(p.anns, sources.Announcement{ID: "n2", Course: "c2", Title: "Exam"})
	p.mu.Unlock()
	if err := s.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if k := kinds(*got); !strings.Contains(k, "assignment_new") || !strings.Contains(k, "announcement") || strings.Contains(k, "assignment_due") {
		t.Fatalf("second sync: %s", k)
	}
	// state survives a restart
	s2 := &Service{Path: s.Path}
	if len(s2.Items()) != 4 {
		t.Fatalf("items after reload: %+v", s2.Items())
	}
}

func TestItemsDoneHide(t *testing.T) {
	s, _ := newService(t, demo())
	s.Sync(context.Background())
	items := s.Items()
	if len(items) != 3 || items[0].ID != "lms:demo:a1" || items[0].Detail != "线性代数（2026春） · 10 分" {
		t.Fatalf("items: %+v", items)
	}
	if !items[2].Done {
		t.Fatal("submitted assignment should be done")
	}
	if err := s.SetDone("lms:demo:a1", true); err != nil || !s.Items()[0].Done {
		t.Fatal("set done", err)
	}
	if err := s.SetDone("lms:demo:a3", false); err != nil || s.Items()[2].Done {
		t.Fatal("undo submitted", err)
	}
	if err := s.Hide("lms:demo:a2"); err != nil || len(s.Items()) != 2 {
		t.Fatal("hide", err)
	}
	if err := s.Unhide("lms:demo:a2"); err != nil || len(s.Items()) != 3 {
		t.Fatal("unhide", err)
	}
	if err := s.SetDone("lms:demo:missing", true); !errors.Is(err, ErrNotFound) {
		t.Fatal("missing", err)
	}
	if !IsItem("lms:demo:a1") || IsItem("todo:1") {
		t.Fatal("IsItem")
	}
}

func TestLinks(t *testing.T) {
	s, _ := newService(t, demo())
	s.Sync(context.Background())
	refs := []Ref{{ID: "L1", Title: "线性代数"}, {ID: "P1", Title: "Physics A"}, {ID: "P2", Title: "Physics B"}}
	if !s.AutoLink(refs) {
		t.Fatal("expected a change")
	}
	if s.LinkedCourse("demo", "c1") != "L1" || s.LinkedCourse("demo", "c2") != "" {
		t.Fatalf("auto links: %q %q", s.LinkedCourse("demo", "c1"), s.LinkedCourse("demo", "c2"))
	}
	if err := s.SetLink("demo", "c2", "P2", false); err != nil {
		t.Fatal(err)
	}
	s.AutoLink(refs)
	if s.LinkedCourse("demo", "c2") != "P2" {
		t.Fatal("manual link overwritten")
	}
	if err := s.SetLink("demo", "c2", "", true); err != nil || s.LinkedCourse("demo", "c2") != "" {
		t.Fatal("auto reset", err)
	}
	if err := s.SetLink("demo", "nope", "P2", false); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if s.Items()[0].CourseID != "L1" {
		t.Fatal("linked item course")
	}
}

func TestStatusDetailAndErrors(t *testing.T) {
	p := demo()
	s, _ := newService(t, p)
	s.Sync(context.Background())
	st := s.Status()
	if len(st) != 1 || !st[0].SignedIn || len(st[0].Courses) != 2 || st[0].Courses[0].Open != 2 || st[0].Courses[0].Total != 2 {
		t.Fatalf("status: %+v", st)
	}
	d, err := s.Detail("demo", "c1")
	if err != nil || len(d.Assignments) != 2 || d.Assignments[0].ID != "a2" || len(d.Announcements) != 1 {
		t.Fatalf("detail: %+v %v", d, err)
	}
	if Plain(d.Announcements[0].HTML, 80) != "下周 停课" {
		t.Fatalf("plain: %q", Plain(d.Announcements[0].HTML, 80))
	}
	p.mu.Lock()
	p.fail = errors.New("boom")
	p.mu.Unlock()
	if err := s.Sync(context.Background()); err == nil {
		t.Fatal("expected error")
	}
	if st := s.Status(); st[0].Error != "boom" || len(st[0].Courses) != 2 {
		t.Fatalf("cached status: %+v", st)
	}
	// signed out: shown from cache
	s.Sources = func() []Source { return nil }
	if st := s.Status(); len(st) != 1 || st[0].SignedIn {
		t.Fatalf("signed out: %+v", st)
	}
	if err := s.Sync(context.Background()); !errors.Is(err, ErrNoSource) {
		t.Fatal(err)
	}
	s.Forget("demo")
	if len(s.Status()) != 0 {
		t.Fatal("forget")
	}
}

func TestDownload(t *testing.T) {
	s, _ := newService(t, demo())
	var fetched []string
	s.Fetch = func(_ context.Context, a sources.Address, dest string) error {
		fetched = append(fetched, a.URL)
		return os.WriteFile(dest, []byte("pdf"), 0o644)
	}
	dir := filepath.Join(t.TempDir(), "course")
	path, err := s.Download(context.Background(), "demo", "c1", "f1", dir)
	if err != nil || filepath.Base(path) != "讲义_第1章.pdf" {
		t.Fatal(path, err)
	}
	if _, err := s.Download(context.Background(), "demo", "c1", "f1", dir); err != nil || len(fetched) != 1 {
		t.Fatal("same file fetched again", fetched, err)
	}
	if _, err := s.Download(context.Background(), "demo", "c1", "nope", dir); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.Download(context.Background(), "other", "c1", "f1", dir); !errors.Is(err, ErrNoSource) {
		t.Fatal(err)
	}
}

func TestSafeName(t *testing.T) {
	if SafeName(" ../a:b?.pdf ", "x") != "_a_b_.pdf" || SafeName("..", "x") != "x" {
		t.Fatal(SafeName(" ../a:b?.pdf ", "x"))
	}
}
