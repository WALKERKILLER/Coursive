package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/walkerkiller/coursive/daemon/internal/agent"
	"github.com/walkerkiller/coursive/daemon/internal/events"
	"github.com/walkerkiller/coursive/daemon/internal/library"
	"github.com/walkerkiller/coursive/daemon/internal/store"
)

// A downloaded slide whose file was removed outside the app is listed
// without its path, so the clients load it through slideImage and its
// remote copy rather than a missing file.
func TestSlidesDropAMissingFile(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	st, err := store.Open(filepath.Join(root, "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	st.UpsertCourse(ctx, store.Course{ID: "c", Title: "演示"})
	st.UpsertLectures(ctx, []store.Lecture{{ID: "l", CourseID: "c", Title: "第一课"}})
	lib, err := library.Open(root, st)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "slides"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "slides", "0001.jpg"), []byte("jpg"), 0o644); err != nil {
		t.Fatal(err)
	}
	st.ReplaceSlides(ctx, "l", []store.Slide{
		{Idx: 1, RemoteURL: "https://media.example/1.jpg", Path: "slides/0001.jpg"},
		{Idx: 2, RemoteURL: "https://media.example/2.jpg", Path: "slides/0002.jpg"},
	})
	s := &Server{Token: "fixture", St: st, Lib: lib, Bus: events.New(), Agent: &agent.Supervisor{}}
	r := httptest.NewRequest("GET", "/api/lectures/l/slides", nil)
	r.Header.Set("Authorization", "Bearer fixture")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var got []store.Slide
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Path != "slides/0001.jpg" || got[1].Path != "" || got[1].RemoteURL == "" {
		t.Fatalf("slides: %+v", got)
	}
}
