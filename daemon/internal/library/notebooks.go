package library

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"regexp"
	"strings"
)

// Notebook identifies an independently opened vector canvas. CourseID is optional.
type Notebook struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	CourseID string `json:"course_id"`
}

var notebookID = regexp.MustCompile("^[0-9a-f]{32}$")

func readNotebook(root *os.Root, id string) (Notebook, error) {
	var n Notebook
	if !notebookID.MatchString(id) {
		return n, ErrStudyInvalid
	}
	dir := "notebooks/" + id
	if err := checkedStudyDir(root, dir, false); err != nil {
		return n, err
	}
	f, err := root.OpenFile(dir+"/notebook.json", os.O_RDONLY, 0)
	if err != nil {
		return n, err
	}
	defer f.Close()
	if err = json.NewDecoder(io.LimitReader(f, 8192)).Decode(&n); err != nil {
		return n, err
	}
	if n.ID != id {
		return n, ErrStudyInvalid
	}
	return n, nil
}

func (l *Library) CreateNotebook(ctx context.Context, title, courseID string) (Notebook, error) {
	l.studyMu.Lock()
	defer l.studyMu.Unlock()
	title = strings.TrimSpace(title)
	n := Notebook{ID: studyID(), Title: title, CourseID: courseID}
	if title == "" || len(title) > 512 {
		return n, ErrStudyInvalid
	}
	if courseID != "" {
		if _, err := l.Store.Course(ctx, courseID); err != nil {
			return n, err
		}
	}
	r, err := os.OpenRoot(l.Root)
	if err != nil {
		return n, err
	}
	defer r.Close()
	dir := "notebooks/" + n.ID
	if err = checkedStudyDir(r, dir, true); err != nil {
		return n, err
	}
	d := emptyStudy()
	d.Objects = []StudyObject{}
	d.Camera.Z = 1
	b, err := json.Marshal(d)
	if err != nil {
		return n, err
	}
	if err = writeStudyBytes(r, dir+"/study-canvas.json", b); err != nil {
		return n, err
	}
	meta, err := json.Marshal(n)
	if err != nil {
		return n, err
	}
	return n, writeStudyBytes(r, dir+"/notebook.json", meta)
}

func (l *Library) Notebooks(courseID string) ([]Notebook, error) {
	out := []Notebook{}
	r, err := os.OpenRoot(l.Root)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	f, err := r.Open("notebooks")
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	entries, err := f.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() || !notebookID.MatchString(entry.Name()) {
			continue
		}
		n, e := readNotebook(r, entry.Name())
		if errors.Is(e, os.ErrNotExist) {
			continue
		}
		if e != nil {
			return nil, e
		}
		if courseID == "" || courseID == n.CourseID {
			out = append(out, n)
		}
	}
	return out, nil
}

func (l *Library) NotebookCanvas(ctx context.Context, id string, next *StudyDocument) (StudyDocument, error) {
	l.studyMu.Lock()
	defer l.studyMu.Unlock()
	r, dir, e := l.notebookRoot(id)
	if e != nil {
		return StudyDocument{}, e
	}
	defer r.Close()
	if _, e = r.Lstat(dir + "/study-canvas.json"); e != nil {
		return StudyDocument{}, e
	}
	if next == nil {
		return readStudy(r, dir+"/study-canvas.json")
	}
	return l.saveStudy(ctx, r, dir, "", *next)
}

func (l *Library) notebookRoot(id string) (*os.Root, string, error) {
	r, e := os.OpenRoot(l.Root)
	if e != nil {
		return nil, "", e
	}
	if _, e = readNotebook(r, id); e != nil {
		r.Close()
		return nil, "", e
	}
	return r, "notebooks/" + id, nil
}
