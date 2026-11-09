package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/walkerkiller/coursive/daemon/internal/events"
	"github.com/walkerkiller/coursive/daemon/internal/lms"
	"github.com/walkerkiller/coursive/daemon/internal/sources"
	"github.com/walkerkiller/coursive/daemon/internal/store"
)

// lmsTestPlatform serves a fixed assignment list so the HTTP boundary can be
// exercised without a source-provider process.
type lmsTestPlatform struct {
	assignments []sources.Assignment
}

func (p *lmsTestPlatform) LMSCourses(context.Context) ([]sources.LMSCourse, error) {
	return []sources.LMSCourse{{ID: "c1", Title: "Course"}}, nil
}

func (p *lmsTestPlatform) Assignments(context.Context, string) ([]sources.Assignment, error) {
	return p.assignments, nil
}

func (p *lmsTestPlatform) Announcements(context.Context, []string, time.Time) ([]sources.Announcement, error) {
	return nil, nil
}

func (p *lmsTestPlatform) LMSFiles(context.Context, string) ([]sources.File, error) {
	return nil, nil
}

func (p *lmsTestPlatform) LMSFileAddress(context.Context, sources.File) (sources.Address, error) {
	return sources.Address{}, nil
}

// Marking an assignment done must be acknowledged even when the assignment is
// not part of the due-dated schedule view. Canvas assignments without a due
// date are visible in the platform course but not in Items(), and the local
// override is still persisted.
func TestPatchLMSAssignmentDoneWithoutDueDate(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	st, err := store.Open(filepath.Join(root, "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	due := time.Now().Add(time.Hour)
	platform := &lmsTestPlatform{assignments: []sources.Assignment{
		{ID: "due", Course: "c1", Title: "Due", DueAt: &due},
		{ID: "no-due", Course: "c1", Title: "No due"},
	}}
	statePath := filepath.Join(root, "lms.json")
	svc := &lms.Service{Path: statePath, Sources: func() []lms.Source {
		return []lms.Source{{ID: "demo/main", Title: "Demo", Platform: platform}}
	}}
	if err := svc.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	s := &Server{Token: "test", St: st, Bus: events.New(), LMS: svc}
	h := s.Handler()
	for _, id := range []string{"lms:demo/main:due", "lms:demo/main:no-due"} {
		req := httptest.NewRequest(http.MethodPatch, "/api/schedule/"+url.PathEscape(id), strings.NewReader(`{"done":true}`))
		req.Header.Set("Authorization", "Bearer test")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: PATCH returned %d: %s", id, w.Code, w.Body.String())
		}
		var got map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got["id"] != id || got["done"] != true {
			t.Fatalf("%s: unexpected response %v", id, got)
		}
	}
	// The override must survive a daemon restart; the platform is not changed.
	reloaded := &lms.Service{Path: statePath}
	for _, id := range []string{"lms:demo/main:due", "lms:demo/main:no-due"} {
		it, err := reloaded.Item(id)
		if err != nil || !it.Done {
			t.Fatalf("%s: done state not persisted: %+v, %v", id, it, err)
		}
	}
}
