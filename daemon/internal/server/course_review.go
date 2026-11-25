package server

import (
	"net/http"

	"github.com/walkerkiller/coursive/daemon/internal/store"
)

func (s *Server) courseReview(_ http.ResponseWriter, r *http.Request) (any, error) {
	var input struct {
		LectureID string `json:"lecture_id"`
		Body      string `json:"body"`
	}
	if err := decode(r, &input); err != nil {
		return nil, err
	}
	var note store.Note
	var card string
	err := s.withCourseWorkspaceWrite(func() error {
		var err error
		note, card, err = s.Lib.CourseReview(r.Context(), r.PathValue("id"), input.LectureID, input.Body)
		return err
	})
	if err != nil {
		return nil, courseFileError(err)
	}
	return s.linkedNoteResult(r.PathValue("id"), note.LectureID, card, note.ID, note.Path), nil
}

func (s *Server) linkCourseNote(_ http.ResponseWriter, r *http.Request) (any, error) {
	var input struct {
		LectureID string `json:"lecture_id"`
		NoteID    string `json:"note_id"`
	}
	if err := decode(r, &input); err != nil {
		return nil, err
	}
	var card string
	err := s.withCourseWorkspaceWrite(func() error {
		var err error
		card, err = s.Lib.LinkCourseNote(r.Context(), r.PathValue("id"), input.LectureID, input.NoteID)
		return err
	})
	if err != nil {
		return nil, courseFileError(err)
	}
	return s.linkedNoteResult(r.PathValue("id"), input.LectureID, card, input.NoteID, ""), nil
}

func (s *Server) linkedNoteResult(course, lecture, card, note, path string) map[string]string {
	s.Bus.Publish("notes_changed", map[string]string{"course_id": course, "lecture_id": lecture, "id": note, "card_id": card})
	return map[string]string{"course_id": course, "lecture_id": lecture, "card_id": card, "note_id": note, "path": path}
}

func (s *Server) courseFolder(_ http.ResponseWriter, r *http.Request) (any, error) {
	var input struct {
		Name string `json:"name"`
	}
	if err := decode(r, &input); err != nil {
		return nil, err
	}
	var path string
	err := s.withCourseWorkspaceWrite(func() error {
		var err error
		path, err = s.Lib.EnsureCourseFolder(r.Context(), r.PathValue("id"), input.Name)
		return err
	})
	return map[string]string{"path": path}, courseFileError(err)
}

func (s *Server) withCourseWorkspaceWrite(write func() error) error {
	if s.Data == nil {
		return write()
	}
	return s.Data.WithLibraryWrite(write)
}
