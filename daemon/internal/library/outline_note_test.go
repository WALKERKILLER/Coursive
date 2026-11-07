package library

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/walkerkiller/coursive/daemon/internal/store"
)

// Saving a reviewed outline (spec 0028) reuses one stable note ID: a retry
// after a lost response adds nothing, and a stale base never overwrites an
// edit made elsewhere. The canvas document is not touched.
func TestOutlineNoteSaveIsIdempotentAndGuarded(t *testing.T) {
	l, _ := reviewFixture(t)
	ctx := context.Background()
	canvas := func() []byte {
		d, e := l.StudyCanvas(ctx, "one")
		if e != nil {
			t.Fatal(e)
		}
		raw, _ := json.Marshal(d)
		return raw
	}
	before := canvas()
	note := store.Note{ID: "outline-0f8c2d3e-1a2b-4c5d-8e9f-0a1b2c3d4e5f", LectureID: "one", Title: "缓存失效复习", Source: "user", Body: "# 缓存失效复习\n\n- 缓存未命中时要读主存 [1]\n"}
	first, e := l.SaveNote(ctx, note)
	if e != nil {
		t.Fatal(e)
	}
	again, e := l.SaveNote(ctx, note)
	if e != nil || again.Path != first.Path {
		t.Fatal("retry moved or failed", e, first.Path, again.Path)
	}
	notes, _ := l.Store.Notes(ctx, "", "one")
	if len(notes) != 1 || notes[0].ID != note.ID {
		t.Fatal("retry created another note", notes)
	}
	elsewhere := note
	base := note.Body
	elsewhere.BaseBody, elsewhere.Body = &base, note.Body+"- 另一台设备补充\n"
	if _, e = l.SaveNote(ctx, elsewhere); e != nil {
		t.Fatal(e)
	}
	stale := note
	stale.BaseBody, stale.Body = &base, note.Body+"- 本机修改\n"
	if _, e = l.SaveNote(ctx, stale); !errors.Is(e, ErrStudyConflict) {
		t.Fatal("stale base overwrote the other device", e)
	}
	kept, _ := os.ReadFile(l.Abs(first.Path))
	if !bytes.Contains(kept, []byte("另一台设备补充")) || bytes.Contains(kept, []byte("本机修改")) {
		t.Fatal("note lost the other device's edit", string(kept))
	}
	if !bytes.Equal(before, canvas()) {
		t.Fatal("saving a note changed the canvas")
	}
}
