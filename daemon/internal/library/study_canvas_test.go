package library

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/walkerkiller/coursive/daemon/internal/store"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestStudyCanvasPersistenceRevisionAndLimits(t *testing.T) {
	l, _ := reviewFixture(t)
	ctx := context.Background()
	d, e := l.StudyCanvas(ctx, "one")
	if e != nil {
		t.Fatal(e)
	}
	if d.Revision != 0 || len(d.Objects) != 1 {
		t.Fatal(d)
	}
	d.Objects[0].PDFColumn = "column-owner"
	d.Objects[0].PositionLocked = true
	d.Version = studyPositionLockFormat
	d.Strokes = []StudyStroke{{ID: "s", Layer: "main", Tool: "pen", Color: 0xff111111, Width: 3, Points: [][3]float64{{1, 2, .5}, {5, 6, 1}}}}
	saved, e := l.SaveStudyCanvas(ctx, "one", d)
	if e != nil || saved.Revision != 1 {
		t.Fatal(e, saved)
	}
	if _, e = l.SaveStudyCanvas(ctx, "one", d); !errors.Is(e, ErrStudyConflict) {
		t.Fatal("lost update accepted", e)
	}
	again, e := l.StudyCanvas(ctx, "one")
	if e != nil || len(again.Strokes) != 1 || again.Strokes[0].Points[1][0] != 5 || again.Objects[0].PDFColumn != "column-owner" || !again.Objects[0].PositionLocked {
		t.Fatal(e, again)
	}
	invalid := again
	invalid.Strokes[0].Points[0][2] = 2
	if _, e = l.SaveStudyCanvas(ctx, "one", invalid); !errors.Is(e, ErrStudyInvalid) {
		t.Fatal("pressure accepted", e)
	}
	invalid = again
	invalid.Strokes = nil
	invalid.Layers = make([]StudyLayer, 33)
	if _, e = l.SaveStudyCanvas(ctx, "one", invalid); !errors.Is(e, ErrStudyLimit) {
		t.Fatal("layers accepted", e)
	}
	if _, e = l.StudyCanvas(ctx, "missing"); e == nil {
		t.Fatal("missing lecture accepted")
	}
}

func TestStudyCanvasPositionLockVersion(t *testing.T) {
	l, _ := reviewFixture(t)
	ctx := t.Context()
	doc, err := l.StudyCanvas(ctx, "one")
	if err != nil {
		t.Fatal(err)
	}
	doc.Version = 3
	doc.Objects[0].PositionLocked = true
	if _, err = l.SaveStudyCanvas(ctx, "one", doc); !errors.Is(err, ErrStudyInvalid) {
		t.Fatal("an unversioned position lock was accepted", err)
	}

	// A candidate version wrote this field without bumping its format.
	root, dir, err := l.studyRoot(ctx, "one")
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	doc.Revision = 1
	legacy, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err = writeStudyBytes(root, dir+"/study-canvas.json", legacy); err != nil {
		t.Fatal(err)
	}
	found, err := l.StudyCanvas(ctx, "one")
	if err != nil || !found.Objects[0].PositionLocked {
		t.Fatal("legacy lock was lost on read", err)
	}
	found.Objects[0].PositionLocked = false
	if _, err = l.SaveStudyCanvas(ctx, "one", found); !errors.Is(err, ErrStudyInvalid) {
		t.Fatal("older editor could remove an unversioned lock", err)
	}
	found.Version = 8
	saved, err := l.SaveStudyCanvas(ctx, "one", found)
	if err != nil || saved.Version != 8 || saved.Objects[0].PositionLocked {
		t.Fatal("version 8 unlock did not save", err)
	}
	backup, err := root.ReadFile(dir + "/study-canvas.pre-v8.json")
	if err != nil || string(backup) != string(legacy) {
		t.Fatal("migration backup did not preserve the original", err)
	}
	saved.Version = 7
	if _, err = l.SaveStudyCanvas(ctx, "one", saved); !errors.Is(err, ErrStudyInvalid) {
		t.Fatal("downgrade accepted after unlocking", err)
	}
}

func TestStudyCanvasFirstUseSurvivesIndexRebuild(t *testing.T) {
	l, _ := reviewFixture(t)
	ctx := context.Background()
	if e := l.Store.UpsertCourse(ctx, store.Course{ID: "stream-course", Title: "流媒体课程"}); e != nil {
		t.Fatal(e)
	}
	if e := l.Store.UpsertLectures(ctx, []store.Lecture{{ID: "stream-lecture", CourseID: "stream-course", Title: "尚未下载的课次", HasPlayback: true}}); e != nil {
		t.Fatal(e)
	}
	d, e := l.StudyCanvas(ctx, "stream-lecture")
	if e != nil {
		t.Fatal(e)
	}
	d, e = l.SaveStudyCanvas(ctx, "stream-lecture", d)
	if e != nil {
		t.Fatal(e)
	}
	rebuilt, e := store.Open(filepath.Join(t.TempDir(), "rebuilt.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer rebuilt.Close()
	restored, e := Open(l.Root, rebuilt)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = restored.Reindex(ctx); e != nil {
		t.Fatal(e)
	}
	again, e := restored.StudyCanvas(ctx, "stream-lecture")
	if e != nil || again.Revision != d.Revision {
		t.Fatal("streamed lecture canvas lost", e, again)
	}
}
func TestStudyCanvasConcurrentSaveAndSafeAssets(t *testing.T) {
	l, _ := reviewFixture(t)
	ctx := context.Background()
	d, e := l.StudyCanvas(ctx, "one")
	if e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Go(func() { _, e := l.SaveStudyCanvas(ctx, "one", d); errs <- e })
	}
	wg.Wait()
	close(errs)
	wins, conflicts := 0, 0
	for e := range errs {
		if e == nil {
			wins++
		} else if errors.Is(e, ErrStudyConflict) {
			conflicts++
		} else {
			t.Fatal(e)
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatal(wins, conflicts)
	}
	a, e := l.ImportStudyAsset(ctx, "one", "说明.md", strings.NewReader("# 课程\n测试"))
	if e != nil || len(a.Objects) != 1 || a.Objects[0].Text != "# 课程\n测试" {
		t.Fatal(a, e)
	}
	d, e = l.StudyCanvas(ctx, "one")
	if e != nil {
		t.Fatal(e)
	}
	d.Objects = append(d.Objects, a.Objects...)
	d, e = l.SaveStudyCanvas(ctx, "one", d)
	if e != nil {
		t.Fatal(e)
	}
	source, e := l.StudySource(ctx, "one", a.Objects[0].Source)
	if e != nil {
		t.Fatal(e)
	}
	source.Close()
	if _, e = l.StudySource(ctx, "one", "../../outside"); e == nil {
		t.Fatal("traversal accepted")
	}
	_, lec, _, e := l.EnsureLecture(ctx, "one")
	if e != nil {
		t.Fatal(e)
	}
	os.Symlink(t.TempDir(), l.Abs(lec.Dir+"/study-assets/linked"))
	d.Objects[1].Source = lec.Dir + "/study-assets/linked/private.txt"
	if _, e = l.SaveStudyCanvas(ctx, "one", d); !errors.Is(e, ErrStudyInvalid) {
		t.Fatal("symlink accepted", e)
	}
	d, e = l.StudyCanvas(ctx, "one")
	if e != nil {
		t.Fatal(e)
	}
	os.Symlink(t.TempDir(), l.Abs(lec.Dir+"/unsafe"))
	if _, e = l.StudySource(ctx, "one", lec.Dir+"/unsafe/file.md"); e == nil {
		t.Fatal("symlink source accepted")
	}
	if _, e = l.ImportStudyAsset(ctx, "one", "bad.md", strings.NewReader(string([]byte{0xff}))); !errors.Is(e, ErrStudyInvalid) {
		t.Fatal(e)
	}
	if _, e = l.ImportStudyAsset(ctx, "one", "large.txt", strings.NewReader(strings.Repeat("a", 800001))); !errors.Is(e, ErrStudyLimit) {
		t.Fatal(e)
	}
}

func TestStudyImportedMarkdownKeepsOriginalAndAnchoredNote(t *testing.T) {
	l, _ := reviewFixture(t)
	ctx := context.Background()
	const original = "# CPU\n\n**process** and process"
	imported, err := l.ImportStudyModule(ctx, "one", "课堂 讲义.MARKDOWN", strings.NewReader(original))
	if err != nil {
		t.Fatal(err)
	}
	card := imported.Objects[0]
	if card.Type != "text" || !strings.HasSuffix(card.Source, "/source.markdown") {
		t.Fatal(card)
	}
	card.Text += "\n\n我的复习提要"
	start := strings.LastIndex(original, "process")
	anchor := StudyAnchor{ID: "md-0-17-24", WorkspaceID: "one", Kind: "text", ObjectID: card.ID,
		Start: start, End: start + 7, Quote: "process", Fingerprint: "example"}
	note := StudyObject{ID: "note-word", Type: "text", Layer: "main", W: 320, H: 180,
		Title: "process", Text: "处理", Generation: "markdown-annotation", Sources: []StudyAnchor{anchor}}
	d, err := l.StudyCanvas(ctx, "one")
	if err != nil {
		t.Fatal(err)
	}
	d.Objects = append(d.Objects, card, note)
	if _, err := l.SaveStudyCanvas(ctx, "one", d); err != nil {
		t.Fatal(err)
	}
	reopened, err := l.StudyCanvas(ctx, "one")
	if err != nil {
		t.Fatal(err)
	}
	if len(reopened.Objects) != 3 || reopened.Objects[1].Text != card.Text ||
		len(reopened.Objects[2].Sources) != 1 || reopened.Objects[2].Sources[0].Start != start ||
		reopened.Objects[2].Text != note.Text {
		t.Fatal(reopened.Objects)
	}
	file, err := l.StudySource(ctx, "one", card.Source)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	raw, err := io.ReadAll(file)
	if err != nil || string(raw) != original {
		t.Fatalf("imported source changed: %q, %v", raw, err)
	}
}

func TestStudyCanvasSameRevisionRemoteReplacementCannotOverwrite(t *testing.T) {
	l, _ := reviewFixture(t)
	ctx := context.Background()
	d, e := l.StudyCanvas(ctx, "one")
	if e != nil {
		t.Fatal(e)
	}
	d, e = l.SaveStudyCanvas(ctx, "one", d)
	if e != nil {
		t.Fatal(e)
	}
	remote := d
	remote.Objects = append([]StudyObject{}, d.Objects...)
	remote.Objects[0].Title = "other device"
	remote.BaseHash = ""
	b, e := json.Marshal(remote)
	if e != nil {
		t.Fatal(e)
	}
	lec, _ := l.Store.Lecture(ctx, "one")
	p := filepath.Join(l.Root, lec.Dir, "study-canvas.json")
	os.WriteFile(p, b, 0600)
	if _, e = l.SaveStudyCanvas(ctx, "one", d); !errors.Is(e, ErrStudyConflict) {
		t.Fatal("same revision but different content was overwritten", e)
	}
	got, _ := os.ReadFile(p)
	if string(got) != string(b) {
		t.Fatal("remote content changed on rejection")
	}
}
