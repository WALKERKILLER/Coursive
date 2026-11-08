package server

import (
	"bytes"
	"encoding/json"
	"github.com/walkerkiller/coursive/daemon/internal/library"
	"net/http"
	"strings"
	"time"
)

func (s *Server) studyAttachmentUpload(w http.ResponseWriter, r *http.Request) (any, error) {
	r.Body = http.MaxBytesReader(w, r.Body, library.StudyAttachmentLimit+(128<<10))
	if e := r.ParseMultipartForm(library.StudyAttachmentLimit + (128 << 10)); e != nil {
		return nil, badRequest("截图必须为不超过3MiB的PNG，请缩小选区")
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	f, _, e := r.FormFile("image")
	if e != nil {
		return nil, badRequest("缺少截图")
	}
	defer f.Close()
	var selection library.StudySnapshotSelection
	if e = json.Unmarshal([]byte(r.FormValue("selection")), &selection); e != nil {
		return nil, badRequest("选区元数据无效")
	}
	out, e := s.Lib.SaveStudyAttachment(r.Context(), r.PathValue("id"), f, selection)
	return out, studyError(e)
}
func (s *Server) studyAttachmentRead(w http.ResponseWriter, r *http.Request) (any, error) {
	out, b, e := s.Lib.StudyAttachment(r.Context(), r.PathValue("id"), r.PathValue("attachment"))
	if e != nil {
		return nil, studyError(e)
	}
	if strings.HasSuffix(r.URL.Path, "/image") {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "private, no-cache")
		w.Header().Set("ETag", `"`+out.SHA256+`"`)
		http.ServeContent(w, r, out.ID+".png", time.Time{}, bytes.NewReader(b))
		return nil, nil
	}
	return out, nil
}
func (s *Server) studyCourseSources(w http.ResponseWriter, r *http.Request) (any, error) {
	if strings.HasSuffix(r.URL.Path, "/search") {
		var q struct {
			Query string `json:"query"`
		}
		if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&q); e != nil {
			return nil, badRequest("检索请求无效")
		}
		out, e := s.Lib.SearchStudyCourseSources(r.Context(), r.PathValue("id"), q.Query)
		return out, studyError(e)
	}
	var q library.StudySourceRead
	if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10)).Decode(&q); e != nil {
		return nil, badRequest("来源请求无效")
	}
	if strings.HasSuffix(r.URL.Path, "/resolve") {
		out, e := s.Lib.ResolveStudyCourseSource(r.Context(), r.PathValue("id"), q)
		return out, studyError(e)
	}
	out, e := s.Lib.ReadStudyCourseSource(r.Context(), r.PathValue("id"), q)
	return out, studyError(e)
}
