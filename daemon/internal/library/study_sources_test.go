package library

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/walkerkiller/coursive/daemon/internal/store"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

func snapshotPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 24, 16))
	img.Set(1, 1, color.NRGBA{R: 255, A: 255})
	var b bytes.Buffer
	if e := png.Encode(&b, img); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func TestSnapshotImmutabilityAndConversationV2(t *testing.T) {
	l, _ := reviewFixture(t)
	ctx := context.Background()
	pngBytes := snapshotPNG(t)
	selection := StudySnapshotSelection{Bounds: []float64{10, 20, 240, 160}, Sources: []StudyAnchor{{WorkspaceID: "one", ID: "a", ObjectID: "old-card"}, {WorkspaceID: "one", ID: "b", ObjectID: "second-card"}}}
	a, e := l.SaveStudyAttachment(ctx, "one", bytes.NewReader(pngBytes), selection)
	if e != nil {
		t.Fatal(e)
	}
	if a.Width != 24 || a.Height != 16 || a.SHA256 != studyHash(string(pngBytes)) || len(a.Selection.Sources) != 2 {
		t.Fatal(a)
	}
	for _, bad := range [][]byte{[]byte("not PNG"), append(pngBytes, make([]byte, StudyAttachmentLimit)...)} {
		if _, e := l.SaveStudyAttachment(ctx, "one", bytes.NewReader(bad), selection); e == nil {
			t.Fatal("invalid image accepted")
		}
	}
	c, e := l.StudyConversation(ctx, "one", "test", nil)
	if e != nil {
		t.Fatal(e)
	}
	c.Draft = "legacy"
	c, e = l.StudyConversation(ctx, "one", "test", &c)
	if e != nil {
		t.Fatal(e)
	}
	lec, _ := l.Store.Lecture(ctx, "one")
	original, _ := os.ReadFile(l.Abs(lec.Dir + "/study-conversations/test.json"))
	c.Version = 2
	c.ContextScope = "course"
	c.DraftAttachments = []StudyAttachment{a}
	c, e = l.StudyConversation(ctx, "one", "test", &c)
	if e != nil {
		t.Fatal(e)
	}
	backup, e := os.ReadFile(l.Abs(lec.Dir + "/study-conversations/test.pre-v2.json"))
	if e != nil || !bytes.Equal(backup, original) {
		t.Fatal("backup", e)
	}
	c.DraftAttachments = nil
	c.Turns = []StudyTurn{{ID: "turn", Question: "q", State: "done", ContextScope: "course", Attachments: []StudyAttachment{a}}}
	c, e = l.StudyConversation(ctx, "one", "test", &c)
	if e != nil {
		t.Fatal(e)
	}
	reopened, e := l.StudyConversation(ctx, "one", "test", nil)
	if e != nil || reopened.Turns[0].Attachments[0].SHA256 != a.SHA256 {
		t.Fatal(reopened, e)
	}
	_, actual, e := l.StudyAttachment(ctx, "one", a.ID)
	if e != nil || !bytes.Equal(actual, pngBytes) {
		t.Fatal("pixels changed", e)
	}
	c.Version = 1
	if _, e = l.StudyConversation(ctx, "one", "test", &c); !errors.Is(e, ErrStudyInvalid) {
		t.Fatal("downgrade", e)
	}
	c.Version = 2
	c.Turns[0].Attachments[0].Width++
	if _, e = l.StudyConversation(ctx, "one", "test", &c); e == nil {
		t.Fatal("forged metadata accepted")
	}
	if _, _, e = l.StudyAttachment(ctx, "one", "../escape"); e == nil {
		t.Fatal("traversal")
	}
	_ = os.WriteFile(l.Abs(a.Resource), []byte("changed"), 0600)
	if _, _, e = l.StudyAttachment(ctx, "one", a.ID); !errors.Is(e, ErrStudyConflict) {
		t.Fatal("corrupt hash", e)
	}
}
func TestCourseEvidenceIsolationUnicodeAndChange(t *testing.T) {
	l, _ := reviewFixture(t)
	ctx := context.Background()
	lec, _ := l.Store.Lecture(ctx, "one")
	rel := lec.Dir + "/material.txt"
	body := "A😀计算机缓存证据\n末尾"
	if e := os.WriteFile(l.Abs(rel), []byte(body), 0600); e != nil {
		t.Fatal(e)
	}
	hits, e := l.SearchStudyCourseSources(ctx, "one", "缓存")
	if e != nil || len(hits) == 0 {
		t.Fatal(hits, e)
	}
	source := StudyCourseSource{Path: rel}
	out, e := l.ReadStudyCourseSource(ctx, "one", StudySourceRead{Source: source, Offset: 1, Limit: 4})
	if e != nil || out.Text != "😀计算机" || out.OffsetUnit != "rune" || out.NextOffset != 5 || out.Page != 0 {
		t.Fatal(out, e)
	}
	if offset, e := RuneOffsetUTF16(body, 2); e != nil || offset != 3 {
		t.Fatal(offset, e)
	}
	if _, e = l.ResolveStudyCourseSource(ctx, "one", StudySourceRead{Source: out}); e != nil {
		t.Fatal(e)
	}
	// Saved citations retain the verified identity/range; text is fetched again.
	ref := out
	ref.Text = ""
	ref.Snippet = ""
	result, _ := json.Marshal(map[string]any{"citations": []any{map[string]any{"id": "e1", "number": 1, "label": "source", "source": ref}}})
	c, _ := l.StudyConversation(ctx, "one", "citation-history", nil)
	c.Version = 2
	c.Turns = []StudyTurn{{ID: "answer", Question: "q", State: "done", Result: result}}
	if _, e = l.StudyConversation(ctx, "one", c.ID, &c); e != nil {
		t.Fatal("compact citation save", e)
	}
	resolved, e := l.ResolveStudyCourseSource(ctx, "one", StudySourceRead{Source: ref})
	if e != nil || resolved.Text != out.Text {
		t.Fatal("compact citation resolve", resolved, e)
	}
	if e = l.Store.UpsertCourse(ctx, store.Course{ID: "other", Title: "other"}); e != nil {
		t.Fatal(e)
	}
	_ = l.Store.UpsertLectures(ctx, []store.Lecture{{ID: "two", CourseID: "other", Title: "second"}})
	_, _, _, _ = l.EnsureLecture(ctx, "two")
	if _, e = l.ReadStudyCourseSource(ctx, "two", StudySourceRead{Source: source}); e == nil {
		t.Fatal("cross course")
	}
	if _, _, e = l.StudyAttachment(ctx, "two", "missing"); e == nil {
		t.Fatal("cross lecture snapshot")
	}
	if e = os.WriteFile(l.Abs(rel), []byte(body+"changed"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = l.ResolveStudyCourseSource(ctx, "one", StudySourceRead{Source: out}); !errors.Is(e, ErrStudyConflict) {
		t.Fatal("stale evidence", e)
	}
	if e = os.Remove(l.Abs(rel)); e != nil {
		t.Fatal(e)
	}
	if _, e = l.ResolveStudyCourseSource(ctx, "one", StudySourceRead{Source: out}); e == nil {
		t.Fatal("deleted evidence")
	}
}
func TestParsedEvidenceDoesNotGuessPage(t *testing.T) {
	l, _ := reviewFixture(t)
	ctx := context.Background()
	a, e := l.ImportCourseware(ctx, "one", "slides.pptx", bytes.NewReader([]byte("fixture-original")), 1000)
	if e != nil {
		t.Fatal(e)
	}
	dir := ParsedDir(a.Path)
	_ = os.MkdirAll(l.Abs(dir), 0700)
	text := "这是一段完整的解析块原文，不能依靠前缀猜测页码。"
	_ = os.WriteFile(l.Abs(dir+"/full.md"), []byte(text), 0600)
	blocks, _ := json.Marshal([]map[string]any{{"text": text, "page_idx": 2, "type": "text"}})
	_ = os.WriteFile(l.Abs(dir+"/content_list.json"), blocks, 0600)
	if e = l.SetParse(ctx, a, &ParseInfo{State: "done", MD: dir + "/full.md"}); e != nil {
		t.Fatal(e)
	}
	full, e := l.ReadStudyCourseSource(ctx, "one", StudySourceRead{Source: StudyCourseSource{Path: a.Path}})
	if e != nil || full.Page != 3 || full.Representation != "parsed_markdown" {
		t.Fatal(full, e)
	}
	partial, e := l.ReadStudyCourseSource(ctx, "one", StudySourceRead{Source: StudyCourseSource{Path: a.Path}, Limit: 16})
	if e != nil || partial.Page != 0 {
		t.Fatal("heuristic page", partial, e)
	}
	_ = os.WriteFile(l.Abs(dir+"/content_list.json"), []byte("[]"), 0600)
	if _, e = l.ResolveStudyCourseSource(ctx, "one", StudySourceRead{Source: full}); !errors.Is(e, ErrStudyConflict) {
		t.Fatal("page mapping changed", e)
	}
}

func TestTranscriptEvidenceStartsAtActualHitAndBoundsVideoTime(t *testing.T) {
	l, segs := reviewFixture(t)
	ctx := context.Background()
	lec, _ := l.Store.Lecture(ctx, "one")
	_ = os.WriteFile(l.Abs(lec.Dir+"/recording.mp4"), []byte("local video fixture"), 0600)
	hits, e := l.SearchStudyCourseSources(ctx, "one", "TLB")
	if e != nil {
		t.Fatal(e)
	}
	var source StudyCourseSource
	for _, h := range hits {
		if h.TimeStartMs == segs[1].StartMs {
			source = h
			break
		}
	}
	if source.Path == "" || source.Offset == 0 {
		t.Fatal("missing actual hit offset", hits)
	}
	out, e := l.ReadStudyCourseSource(ctx, "one", StudySourceRead{Source: source, Offset: source.Offset})
	if e != nil || out.TimeStartMs != segs[1].StartMs || out.VideoPath == "" {
		t.Fatal(out, e)
	}
	if _, e = l.ResolveStudyCourseSource(ctx, "one", StudySourceRead{Source: out}); e != nil {
		t.Fatal(e)
	}
	short, e := l.ReadStudyCourseSource(ctx, "one", StudySourceRead{Source: source, Offset: source.Offset, Limit: 1})
	if e != nil || short.TimeStartMs != 0 || short.VideoPath != "" {
		t.Fatal("timestamp advertised without words", short, e)
	}
}

func TestTranscriptReadTimeCoversLaterQuotedSentences(t *testing.T) {
	l, segs := reviewFixture(t)
	ctx := context.Background()
	lec, _ := l.Store.Lecture(ctx, "one")
	_ = os.WriteFile(l.Abs(lec.Dir+"/recording.mp4"), []byte("local video"), 0600)
	out, e := l.ReadStudyCourseSource(ctx, "one", StudySourceRead{Source: StudyCourseSource{Path: lec.Dir + "/transcript.md", TimeStartMs: segs[0].StartMs}})
	if e != nil || out.TimeStartMs != segs[0].StartMs || out.TimeEndMs != segs[1].EndMs || out.VideoPath == "" {
		t.Fatal(out, e)
	}
	resolved, e := l.ResolveStudyCourseSource(ctx, "one", StudySourceRead{Source: out})
	if e != nil || resolved.TimeEndMs != out.TimeEndMs {
		t.Fatal(resolved, e)
	}
}
