package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/walkerkiller/coursive/daemon/internal/agent"
	"github.com/walkerkiller/coursive/daemon/internal/events"
	"github.com/walkerkiller/coursive/daemon/internal/library"
	"github.com/walkerkiller/coursive/daemon/internal/store"
)

func TestCourseFileRoutesAndFormerFileURL(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	st, err := store.Open(filepath.Join(root, "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.UpsertCourse(ctx, store.Course{ID: "c", Title: "课程"}); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertLectures(ctx, []store.Lecture{{ID: "l", CourseID: "c", Title: "第一节"}}); err != nil {
		t.Fatal(err)
	}
	lib, err := library.Open(root, st)
	if err != nil {
		t.Fatal(err)
	}
	a, err := lib.ImportCourseware(ctx, "l", "原名.pdf", strings.NewReader("PDF bytes"), 1024)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Token: "fixture", St: st, Lib: lib, Bus: events.New(), Agent: &agent.Supervisor{}}
	handler := s.Handler()
	call := func(method, route, body string, authorized bool) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, route, strings.NewReader(body))
		if authorized {
			r.Header.Set("Authorization", "Bearer fixture")
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	base := "/api/courses/c/material-files"
	if w := call("GET", base, "", false); w.Code != 401 {
		t.Fatal(w.Code)
	}
	w := call("GET", base+"/read?path="+url.QueryEscape(a.Path), "", true)
	var inspected struct {
		Version string `json:"version"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &inspected) != nil || len(inspected.Version) != 64 {
		t.Fatal(w.Code, w.Body.String())
	}
	for query, code := range map[string]int{"page=abc&rev=" + inspected.Version: 400, "page=1&rev=" + strings.Repeat("0", 64): 409} {
		got := call("GET", "/api/lectures/l/courseware/page-preview?file="+url.QueryEscape(a.Path)+"&"+query, "", true)
		if got.Code != code {
			t.Fatalf("page preview %s: %d, want %d", query, got.Code, code)
		}
	}
	body, _ := json.Marshal(map[string]any{"edits": []library.CourseFileEdit{
		{Op: "rename", Path: a.Path, Name: "新名.pdf", Version: inspected.Version},
	}})
	w = call("POST", base+"/batches", string(body), true)
	var batch library.CourseFileBatch
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &batch) != nil || batch.State != "pending" {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := call("POST", "/api/material-file-batches/"+batch.ID+"/apply", "{}", true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = call("GET", "/api/lectures/l/courseware?former_path="+url.QueryEscape("原名.pdf"), "", true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "新名.pdf") {
		t.Fatal(w.Code, w.Body.String())
	}
	file := (&url.URL{Path: "/files/" + a.Path}).String()
	if w := call("GET", file, "", true); w.Code != 200 || w.Body.String() != "PDF bytes" {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := call("POST", "/api/material-file-batches/"+batch.ID+"/undo", "{}", true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestCourseReviewReusesOriginalNoteAndCard(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	st, err := store.Open(filepath.Join(root, "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	for _, c := range []store.Course{{ID: "os", Title: "操作系统"}, {ID: "other", Title: "其他课"}} {
		if err := st.UpsertCourse(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.UpsertLectures(ctx, []store.Lecture{
		{ID: "first", CourseID: "os", Title: "第一节"},
		{ID: "next", CourseID: "os", Title: "另一节"},
		{ID: "outside", CourseID: "other", Title: "外课"},
	}); err != nil {
		t.Fatal(err)
	}
	lib, err := library.Open(root, st)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Token: "fixture", St: st, Lib: lib, Bus: events.New(), Agent: &agent.Supervisor{}}
	handler := s.Handler()
	call := func(endpoint, body, token string, code int) map[string]string {
		t.Helper()
		request := httptest.NewRequest("POST", "/api/courses/os/"+endpoint, strings.NewReader(body))
		request.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, request)
		if w.Code != code {
			t.Fatalf("HTTP %d, wanted %d: %s", w.Code, code, w.Body.String())
		}
		var result map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	const body = `{"lecture_id":"first","body":"## 进程\n[课堂原文](coursive://lecture/first?t=20)"}`
	call("review-handout", body, "wrong", 401)
	call("review-handout", `{"lecture_id":"outside","body":"外课"}`, "fixture", 400)
	created := call("review-handout", body, "fixture", 200)
	call("material-folder", `{"name":"../test"}`, "fixture", 400)
	folder := call("material-folder", `{"name":"历年试卷"}`, "fixture", 200)
	if again := call("material-folder", `{"name":"历年试卷"}`, "fixture", 200); again["path"] != folder["path"] {
		t.Fatal("retry created a different folder")
	}
	note, err := st.Note(ctx, created["note_id"])
	if err != nil || !strings.HasSuffix(note.Path, ".md") || !strings.Contains(note.Body, "coursive://lecture/first?t=20") {
		t.Fatal(note, err)
	}
	old := note.Body
	note.BaseBody, note.Body = &old, note.Body+"\n\n我的补充"
	if _, err = lib.SaveNote(ctx, note); err != nil {
		t.Fatal(err)
	}
	retry := call("review-handout", `{"lecture_id":"next","body":"这段不能覆盖学生编辑"}`, "fixture", 200)
	if retry["note_id"] != note.ID || retry["card_id"] != created["card_id"] || retry["lecture_id"] != "first" {
		t.Fatalf("retry diverged from original: %v; %v", created, retry)
	}
	d, err := lib.StudyCanvas(ctx, "first")
	if err != nil || len(d.Objects) != 2 || d.Objects[1].DocumentID != note.ID || d.Objects[1].Text != "" {
		t.Fatalf("canvas should reference the note exactly once: %+v %v", d.Objects, err)
	}
	res, err := lib.StudyContext(ctx, "first", library.StudyContextRequest{Revision: d.Revision, Objects: []string{created["card_id"]}})
	if err != nil || len(res.Sources) != 1 || !strings.Contains(res.Sources[0].Text, "我的补充") {
		t.Fatalf("selection did not read the latest canonical note: %+v %v", res.Sources, err)
	}
}

func TestStaleScheduleApprovalPreservesUserEdit(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	before := store.ScheduleItem{ID: "exam", Source: "user", Kind: "exam", Title: "Exam", DueDate: "2026-12-01"}
	if err := st.PutScheduleItem(ctx, before); err != nil {
		t.Fatal(err)
	}
	newer := before
	newer.Title = "Rescheduled exam"
	if err := st.PutScheduleItem(ctx, newer); err != nil {
		t.Fatal(err)
	}
	after := before
	after.DueDate = "2026-12-05"
	if changed, err := st.UpdateScheduleItem(ctx, before, after); err != nil || changed {
		t.Fatal(changed, err)
	}
	body, err := json.Marshal(map[string]any{"base": before, "due_date": after.DueDate})
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("PATCH", "/api/schedule/exam", strings.NewReader(string(body)))
	r.SetPathValue("id", "exam")
	_, err = (&Server{St: st}).patchSchedule(httptest.NewRecorder(), r)
	if e, ok := err.(apiError); !ok || e.Code != 409 {
		t.Fatal(err)
	}
	current, err := st.ScheduleItem(ctx, "exam")
	if err != nil || current.Title != newer.Title || current.DueDate != before.DueDate {
		t.Fatal(current, err)
	}
}
