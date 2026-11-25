package server

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/walkerkiller/coursive/daemon/internal/datasync"
	"github.com/walkerkiller/coursive/daemon/internal/library"
)

func courseFileError(err error) error {
	if errors.Is(err, datasync.ErrBusy) {
		return apiError{http.StatusConflict, "资料正在同步，请稍后重试"}
	}
	if errors.Is(err, sql.ErrNoRows) {
		return apiError{http.StatusNotFound, "找不到课程或资料"}
	}
	return studyError(err)
}

func (s *Server) courseFiles(_ http.ResponseWriter, r *http.Request) (any, error) {
	q := r.URL.Query()
	offset, _ := strconv.Atoi(q.Get("offset"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	out, err := s.Lib.ListCourseFiles(r.Context(), r.PathValue("id"), q.Get("q"), offset, limit)
	return out, courseFileError(err)
}

func (s *Server) readCourseFile(_ http.ResponseWriter, r *http.Request) (any, error) {
	q := r.URL.Query()
	offset, _ := strconv.Atoi(q.Get("offset"))
	out, err := s.Lib.ReadCourseFile(r.Context(), r.PathValue("id"), q.Get("path"), offset)
	return out, courseFileError(err)
}

func (s *Server) previewCourseFiles(_ http.ResponseWriter, r *http.Request) (any, error) {
	var in struct {
		Edits []library.CourseFileEdit `json:"edits"`
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	out, err := s.Lib.PreviewCourseFiles(r.Context(), r.PathValue("id"), in.Edits)
	return out, courseFileError(err)
}

func (s *Server) applyCourseFiles(_ http.ResponseWriter, r *http.Request) (any, error) {
	return s.commitCourseFiles(r, false)
}

func (s *Server) undoCourseFiles(_ http.ResponseWriter, r *http.Request) (any, error) {
	return s.commitCourseFiles(r, true)
}

func (s *Server) commitCourseFiles(r *http.Request, undo bool) (any, error) {
	var batch library.CourseFileBatch
	write := func() error {
		var err error
		batch, err = s.Lib.CommitCourseFileBatch(r.Context(), r.PathValue("id"), undo)
		return err
	}
	var err error
	if s.Data == nil {
		err = write()
	} else {
		err = s.Data.WithLibraryWrite(write)
	}
	if err != nil {
		return nil, courseFileError(err)
	}
	for _, id := range batch.Lectures {
		s.publishLecture(r.Context(), id, "courseware")
	}
	s.Bus.Publish("notes_changed", map[string]string{"course_id": batch.CourseID})
	return batch, nil
}
