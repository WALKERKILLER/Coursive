package server

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/walkerkiller/coursive/daemon/internal/agent"
	"github.com/walkerkiller/coursive/daemon/internal/events"
	"github.com/walkerkiller/coursive/daemon/internal/library"
	"github.com/walkerkiller/coursive/daemon/internal/store"
	"image"
	"image/png"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotMultipartAndCourseEvidenceHTTP(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	st, e := store.Open(filepath.Join(root, "library.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer st.Close()
	_ = st.UpsertCourse(ctx, store.Course{ID: "course", Title: "test"})
	_ = st.UpsertLectures(ctx, []store.Lecture{{ID: "one", CourseID: "course", Title: "one"}, {ID: "two", CourseID: "course", Title: "two"}})
	lib, e := library.Open(root, st)
	if e != nil {
		t.Fatal(e)
	}
	_, _, _, _ = lib.EnsureLecture(ctx, "one")
	_, _, _, _ = lib.EnsureLecture(ctx, "two")
	s := &Server{Token: "fixture", St: st, Lib: lib, Bus: events.New(), Agent: &agent.Supervisor{}}
	handler := s.Handler()
	call := func(method, path, kind string, body []byte) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, bytes.NewReader(body))
		r.Header.Set("Authorization", "Bearer fixture")
		r.Header.Set("Content-Type", kind)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	var pixels, body bytes.Buffer
	_ = png.Encode(&pixels, image.NewNRGBA(image.Rect(0, 0, 12, 9)))
	form := multipart.NewWriter(&body)
	part, _ := form.CreateFormFile("image", "selection.png")
	_, _ = part.Write(pixels.Bytes())
	_ = form.WriteField("selection", `{"bounds":[0,0,12,9],"sources":[]}`)
	_ = form.Close()
	base := "/api/lectures/one/study-canvas"
	w := call("POST", base+"/attachments", form.FormDataContentType(), body.Bytes())
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var a library.StudyAttachment
	if e = json.Unmarshal(w.Body.Bytes(), &a); e != nil {
		t.Fatal(e)
	}
	w = call("GET", base+"/attachments/"+a.ID+"/image", "", nil)
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), pixels.Bytes()) || w.Header().Get("Content-Type") != "image/png" {
		t.Fatal("image bytes", w.Code)
	}
	if w = call("GET", "/api/lectures/two/study-canvas/attachments/"+a.ID+"/image", "", nil); w.Code == 200 {
		t.Fatal("snapshot leaked to another lecture")
	}
	other, _ := st.Lecture(ctx, "two")
	rel := other.Dir + "/evidence.txt"
	_ = os.WriteFile(lib.Abs(rel), []byte("真实跨课次来源 😀"), 0600)
	w = call("POST", base+"/sources/search", "application/json", []byte(`{"query":"跨课次"}`))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var hits []library.StudyCourseSource
	_ = json.Unmarshal(w.Body.Bytes(), &hits)
	if len(hits) != 1 || hits[0].LectureID != "two" {
		t.Fatal(hits)
	}
	query, _ := json.Marshal(library.StudySourceRead{Source: hits[0], Limit: 12000})
	w = call("POST", base+"/sources/read", "application/json", query)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var evidence library.StudyCourseSource
	_ = json.Unmarshal(w.Body.Bytes(), &evidence)
	query, _ = json.Marshal(library.StudySourceRead{Source: evidence})
	w = call("POST", base+"/sources/resolve", "application/json", query)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	_ = os.WriteFile(lib.Abs(rel), []byte("changed"), 0600)
	w = call("POST", base+"/sources/resolve", "application/json", query)
	if w.Code != 409 {
		t.Fatal("changed source", w.Code, w.Body.String())
	}
}
