package library

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/walkerkiller/coursive/daemon/internal/store"
)

const courseReviewLimit = 64 << 10

// CourseReview uses the course note as the source of truth and the lecture canvas
// as a view onto it. A retry after a failed canvas save resumes from the note.
func (l *Library) CourseReview(ctx context.Context, courseID, lectureID, body string) (store.Note, string, error) {
	lecture, err := l.Store.Lecture(ctx, lectureID)
	if err != nil {
		return store.Note{}, "", err
	}
	if lecture.CourseID != courseID {
		return store.Note{}, "", ErrStudyInvalid
	}
	course, err := l.Store.Course(ctx, courseID)
	if err != nil {
		return store.Note{}, "", err
	}
	sum := sha256.Sum256([]byte(courseID))
	noteID := fmt.Sprintf("review-%x", sum[:16])
	n, err := l.Store.Note(ctx, noteID)
	if errors.Is(err, sql.ErrNoRows) {
		if strings.TrimSpace(body) == "" {
			return store.Note{}, "", err
		}
		if len(body) > courseReviewLimit {
			return store.Note{}, "", ErrStudyLimit
		}
		d, canvasErr := l.StudyCanvas(ctx, lectureID)
		if canvasErr != nil {
			return store.Note{}, "", canvasErr
		}
		if _, err := linkedNoteLayer(d); err != nil {
			return store.Note{}, "", err
		}
		title := strings.TrimSpace(course.Alias)
		if title == "" {
			title = course.Title
		}
		runes := []rune(title)
		if len(runes) > 60 {
			title = string(runes[:60])
		}
		n, err = l.SaveNote(ctx, store.Note{ID: noteID, CourseID: courseID, LectureID: lectureID, Title: title + " · 复习讲义", Body: body, Source: "agent", CreateOnly: true})
		if errors.Is(err, ErrStudyConflict) {
			n, err = l.Store.Note(ctx, noteID)
		}
		if err != nil {
			return n, "", err
		}
	}
	if err != nil {
		return n, "", err
	}
	if n.CourseID != courseID || n.LectureID == "" {
		return n, "", ErrStudyConflict
	}
	card, err := l.LinkCourseNote(ctx, courseID, n.LectureID, n.ID)
	return n, card, err
}

func linkedNoteLayer(d StudyDocument) (string, error) {
	for _, layer := range d.Layers {
		if layer.Visible && !layer.Locked {
			return layer.ID, nil
		}
	}
	return "", ErrStudyConflict
}

// LinkCourseNote binds only an indexed note owned by the canvas's course.
func (l *Library) LinkCourseNote(ctx context.Context, courseID, lectureID, noteID string) (string, error) {
	le, err := l.Store.Lecture(ctx, lectureID)
	if err != nil {
		return "", err
	}
	n, err := l.Store.Note(ctx, noteID)
	if err != nil {
		return "", err
	}
	if le.CourseID != courseID || n.CourseID != courseID {
		return "", ErrStudyInvalid
	}
	sum := sha256.Sum256([]byte(n.ID))
	cardID := fmt.Sprintf("note-%x", sum[:16])
	l.studyMu.Lock()
	defer l.studyMu.Unlock()
	root, dir, err := l.studyRoot(ctx, lectureID)
	if err != nil {
		return "", err
	}
	defer root.Close()
	d, err := readStudy(root, dir+"/study-canvas.json")
	if err != nil {
		return "", err
	}
	x, y := 940.0, 0.0
	for _, card := range d.Objects {
		if card.Type == "text" && card.Generation == "linked-note" && card.DocumentID == n.ID {
			return card.ID, nil
		}
		if card.ID == cardID {
			return "", ErrStudyConflict
		}
		if card.ID == "lecture" {
			y = card.Y
		}
		x = math.Max(x, card.X+math.Hypot(card.W, card.H)+48)
	}
	layer, err := linkedNoteLayer(d)
	if err != nil {
		return "", err
	}
	d.Objects = append(d.Objects, StudyObject{ID: cardID, Type: "text", Layer: layer,
		X: x, Y: y, W: 600, H: 620, Title: n.Title, DocumentID: n.ID, Generation: "linked-note"})
	_, err = l.saveStudy(ctx, root, dir, lectureID, d)
	return cardID, err
}

// EnsureCourseFolder creates a single named folder inside the course intake.
func (l *Library) EnsureCourseFolder(ctx context.Context, courseID, name string) (string, error) {
	if strings.TrimSpace(name) != name || name == "" || name == "." || name == ".." || len(name) > 140 ||
		strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") || Sanitize(name, "") != name ||
		strings.ContainsAny(name, "/\\:\x00") {
		return "", ErrStudyInvalid
	}
	l.mirrorMu.Lock()
	defer l.mirrorMu.Unlock()
	c, err := l.Store.Course(ctx, courseID)
	if err != nil {
		return "", err
	}
	if c.Dir == "" {
		return "", ErrStudyInvalid
	}
	destination := c.Dir + "/" + courseInbox + "/" + name
	root, err := os.OpenRoot(l.Root)
	if err != nil {
		return "", err
	}
	defer root.Close()
	if err := checkedStudyDir(root, destination, true); err != nil {
		return "", err
	}
	return destination, nil
}
