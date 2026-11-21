package server

import (
	"context"
	"encoding/json"
	"github.com/walkerkiller/coursive/daemon/internal/agent"
	"github.com/walkerkiller/coursive/daemon/internal/events"
	"github.com/walkerkiller/coursive/daemon/internal/library"
	"github.com/walkerkiller/coursive/daemon/internal/pdftest"
	"github.com/walkerkiller/coursive/daemon/internal/store"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStudyCanvasAuthenticatedRoutes(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	st, e := store.Open(filepath.Join(root, "library.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer st.Close()
	st.UpsertCourse(ctx, store.Course{ID: "c", Title: "演示"})
	st.UpsertLectures(ctx, []store.Lecture{{ID: "l", CourseID: "c", Title: "第一课"}, {ID: "other", CourseID: "c", Title: "另一课"}})
	lib, e := library.Open(root, st)
	if e != nil {
		t.Fatal(e)
	}
	s := &Server{Token: "fixture", St: st, Lib: lib, Bus: events.New(), Agent: &agent.Supervisor{}}
	h := s.Handler()
	call := func(method, p, body, token string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, p, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	p := "/api/lectures/l/study-canvas"
	if w := call("GET", p, "", "wrong"); w.Code != 401 {
		t.Fatal(w.Code)
	}
	w := call("GET", p, "", "fixture")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var d library.StudyDocument
	if e = json.Unmarshal(w.Body.Bytes(), &d); e != nil {
		t.Fatal(e)
	}
	w = call("POST", p+"/assets?name=notes.md", "# study\nlocal text", "fixture")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var imported library.StudyImport
	json.Unmarshal(w.Body.Bytes(), &imported)
	d.Objects = append(d.Objects, imported.Objects...)
	b, _ := json.Marshal(d)
	if w = call("PUT", p, string(b), "fixture"); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	q, _ := json.Marshal(library.StudyContextRequest{Revision: 1, Objects: []string{imported.Objects[0].ID}, Selection: library.StudySelection{ObjectID: imported.Objects[0].ID, Start: 8, End: 13}})
	if w = call("POST", p+"/context", string(q), "wrong"); w.Code != 401 {
		t.Fatal("context auth", w.Code)
	}
	w = call("POST", p+"/context", string(q), "fixture")
	var scope library.StudyContext
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &scope) != nil || len(scope.Sources) != 1 || scope.Sources[0].Text != "local" {
		t.Fatal("actual selection", w.Code, w.Body.String())
	}
	if w = call("POST", "/api/lectures/other/study-canvas/context", string(q), "fixture"); w.Code != 409 {
		t.Fatal("cross-lecture context", w.Code)
	}
	if w = call("POST", p+"/context", `{"revision":0}`, "fixture"); w.Code != 409 {
		t.Fatal("stale context", w.Code)
	}
	if w = call("PUT", p, string(b), "fixture"); w.Code != 409 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = call("PUT", p, `{"broken":`, "fixture"); w.Code != 400 {
		t.Fatal(w.Code)
	}
	if w = call("POST", p+"/assets?path=../../outside", "", "fixture"); w.Code != 400 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = call("POST", "/api/lectures/other/study-canvas/assets?path="+imported.Objects[0].Source, "", "fixture"); w.Code != 400 {
		t.Fatal("cross-lecture asset accepted", w.Code, w.Body.String())
	}
	if w = call("GET", "/api/lectures/missing/study-canvas", "", "fixture"); w.Code != 404 {
		t.Fatal(w.Code)
	}

	conversation := p + "/conversations/chat"
	if w = call("GET", conversation, "", "wrong"); w.Code != 401 {
		t.Fatal("chat auth", w.Code)
	}
	if w = call("PUT", conversation, `{"version":1,"id":"chat","revision":0,"draft":"my draft","query":{"objects":[]},"turns":[{"id":"one","question":"q","answer":"a","state":"done","context_key":"scope"}]}`, "fixture"); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = call("GET", conversation, "", "fixture"); w.Code != 200 || !strings.Contains(w.Body.String(), "my draft") {
		t.Fatal("chat restore", w.Body.String())
	}
	if w = call("GET", "/api/lectures/other/study-canvas/conversations/chat", "", "fixture"); w.Code != 200 || strings.Contains(w.Body.String(), "my draft") {
		t.Fatal("chat scope", w.Body.String())
	}
	if w = call("POST", p+"/media", `{"object_id":"lecture","from_ms":0,"to_ms":180001}`, "fixture"); w.Code != 400 {
		t.Fatal("unbounded media request", w.Code)
	}
	w = call("GET", "/api/lectures/other/study-canvas", "", "fixture")
	if w.Code != 200 || strings.Contains(w.Body.String(), "local text") {
		t.Fatal("lecture state leaked", w.Code, w.Body.String())
	}
}

func TestStandaloneNotebookRoundTrip(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	st, err := store.Open(filepath.Join(root, "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err = st.UpsertCourse(ctx, store.Course{ID: "os", Title: "操作系统"}); err != nil {
		t.Fatal(err)
	}
	handler := func() http.Handler {
		lib, e := library.Open(root, st)
		if e != nil {
			t.Fatal(e)
		}
		return (&Server{Token: "fixture", St: st, Lib: lib, Bus: events.New(), Agent: &agent.Supervisor{}}).Handler()
	}
	h := handler()
	call := func(t *testing.T, method, path, body string, code int, out any) {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer fixture")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != code {
			t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body.String())
		}
		if out != nil {
			if e := json.Unmarshal(w.Body.Bytes(), out); e != nil {
				t.Fatal(e)
			}
		}
	}
	var n library.Notebook
	call(t, "POST", "/api/notebooks", `{"title":"进程与线程","course_id":"os"}`, 200, &n)
	base := "/api/notebooks/" + n.ID
	var d library.StudyDocument
	call(t, "GET", base+"/canvas", "", 200, &d)
	if len(d.Objects) != 0 {
		t.Fatal("expected empty canvas")
	}
	var imported library.StudyImport
	call(t, "POST", base+"/assets?name=snippet.md", "class notes", 200, &imported)
	d.Objects = imported.Objects
	d.Strokes = []library.StudyStroke{{ID: "a", Layer: "main", Tool: "pen", Width: 2, Points: [][3]float64{{1, 2, 1}, {3, 4, 1}}}}
	d.Camera = library.StudyCamera{X: 95, Y: -23, Z: 1.5}
	data, _ := json.Marshal(d)
	call(t, "PUT", base+"/canvas", string(data), 200, nil)
	call(t, "PUT", base+"/canvas", string(data), 409, nil)
	h = handler()
	var reopened library.StudyDocument
	call(t, "GET", base+"/canvas", "", 200, &reopened)
	if reopened.Camera != d.Camera || len(reopened.Strokes) != 1 || reopened.Objects[0].Text != "class notes" {
		t.Fatal("lost document state")
	}
	var items []library.CatalogItem
	call(t, "GET", "/api/library/catalog?course_id=os", "", 200, &items)
	if len(items) != 1 || items[0].CanvasID != n.ID {
		t.Fatal(items)
	}
	var docs []library.Notebook
	call(t, "GET", "/api/notebooks?course_id=os", "", 200, &docs)
	if len(docs) != 1 || docs[0] != n {
		t.Fatal(docs)
	}
	call(t, "GET", "/api/notebooks?course_id=other", "", 200, &docs)
	if len(docs) != 0 {
		t.Fatal(docs)
	}
	t.Run("paged PDF upload and owner-scoped page lookup", func(t *testing.T) {
		pdftest.RequirePagedRenderer(t)
		path := filepath.Join(t.TempDir(), "two.pdf")
		pdftest.WriteTwoPagePDF(t, path)
		payload, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var result library.StudyImport
		call(t, "POST", base+"/assets?name=two.pdf&paged=1", string(payload), 200, &result)
		if len(result.Objects) != 1 || result.Objects[0].Type != "pdfdoc" {
			t.Fatalf("expected a single paged PDF card, got %+v", result.Objects)
		}
		query := "/pdf-page?page=2&source=" + url.QueryEscape(result.Objects[0].Source)
		var page library.StudyPDFPage
		call(t, "GET", base+query, "", 200, &page)
		if page.Count != 2 || !strings.Contains(page.Text, "Second page") {
			t.Fatalf("unexpected page: %+v", page)
		}
		var other library.Notebook
		call(t, "POST", "/api/notebooks", `{"title":"Other"}`, 200, &other)
		call(t, "GET", "/api/notebooks/"+other.ID+query, "", 400, nil)
	})
}
