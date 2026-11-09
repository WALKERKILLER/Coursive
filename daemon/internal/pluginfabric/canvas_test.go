package pluginfabric

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
)

// canvasObject is a valid object the tests vary one field of.
func canvasObject() CanvasObject {
	return CanvasObject{
		ID:            "obj-1",
		Type:          "review-lab.card",
		SchemaVersion: CanvasSchemaVersion,
		Geometry:      CanvasGeometry{X: 40, Y: 60, W: 320, H: 240},
		Content: CanvasContent{
			Title: "符号解释",
			Text:  "先展开定义，再代入。",
			Data:  map[string]any{"steps": 3},
		},
		SourceAnchors:     []CanvasAnchor{{Kind: AnchorLecture, LectureID: "lecture-1", StartMs: 12_000, EndMs: 34_000}},
		CreatedBy:         CanvasOrigin{PluginID: "review-lab", PluginVersion: "1.0.0"},
		Revision:          1,
		InteractionPolicy: CanvasInteractionPolicy{Mode: CanvasMove},
	}
}

func TestCanvasObjectAcceptsEveryAnchorKind(t *testing.T) {
	cases := map[string]CanvasAnchor{
		"lecture interval": {Kind: AnchorLecture, LectureID: "lecture-1", StartMs: 0, EndMs: 90_000},
		"lecture from":     {Kind: AnchorLecture, LectureID: "lecture-1"},
		"pdf page":         {Kind: AnchorPDF, File: "第 2 讲/讲义.pdf", Page: 3},
		"pdf range":        {Kind: AnchorPDF, File: "第 2 讲/讲义.pdf", Page: 3, PageEnd: 5},
		"courseware file":  {Kind: AnchorCourseware, File: "第 2 讲/课件.pptx"},
		"transcript":       {Kind: AnchorTranscript, LectureID: "lecture-1", StartMs: 12_000, EndMs: 20_000, Text: "所以这里用的是链式法则。"},
		"handwriting":      {Kind: AnchorStroke, LectureID: "lecture-1", StrokeIDs: []string{"stroke-1", "stroke-2"}},
	}
	for name, anchor := range cases {
		t.Run(name, func(t *testing.T) {
			o := canvasObject()
			o.SourceAnchors = []CanvasAnchor{anchor}
			if err := o.Validate(); err != nil {
				t.Fatalf("Validate: %v", err)
			}
		})
	}
}

func TestCanvasObjectReportsEveryDefect(t *testing.T) {
	cases := []struct {
		name   string
		change func(*CanvasObject)
		code   string
		path   string
	}{
		{"missing id", func(o *CanvasObject) { o.ID = "" }, "id-missing", "id"},
		{"id with a path", func(o *CanvasObject) { o.ID = "a/b" }, "id-invalid", "id"},
		{"missing type", func(o *CanvasObject) { o.Type = "" }, "type-missing", "type"},
		{"unqualified type", func(o *CanvasObject) { o.Type = "card" }, "type-invalid", "type"},
		{"missing schema version", func(o *CanvasObject) { o.SchemaVersion = 0 }, "schema-missing", "schemaVersion"},
		{"newer schema version", func(o *CanvasObject) { o.SchemaVersion = 2 }, "schema-unsupported", "schemaVersion"},
		{"tiny width", func(o *CanvasObject) { o.Geometry.W = 4 }, "geometry-invalid", "geometry.w"},
		{"unbounded height", func(o *CanvasObject) { o.Geometry.H = CanvasMaxCoordinate * 2 }, "geometry-invalid", "geometry.h"},
		{"nan coordinate", func(o *CanvasObject) { o.Geometry.X = math.NaN() }, "geometry-invalid", "geometry.x"},
		{"rotation out of range", func(o *CanvasObject) { o.Geometry.Rotation = 4 }, "geometry-invalid", "geometry.rotation"},
		{"missing title", func(o *CanvasObject) { o.Content.Title = " " }, "content-invalid", "content.title"},
		{"text too large", func(o *CanvasObject) { o.Content.Text = strings.Repeat("字", CanvasMaxText) }, "content-invalid", "content.text"},
		{"data too large", func(o *CanvasObject) { o.Content.Data = map[string]any{"blob": strings.Repeat("x", CanvasMaxData)} }, "content-invalid", "content.data"},
		{"missing plugin", func(o *CanvasObject) { o.CreatedBy.PluginID = "" }, "origin-missing", "createdBy.pluginId"},
		{"plugin version", func(o *CanvasObject) { o.CreatedBy.PluginVersion = "1.0" }, "origin-invalid", "createdBy.pluginVersion"},
		{"revision zero", func(o *CanvasObject) { o.Revision = 0 }, "revision-invalid", "revision"},
		{"missing interaction mode", func(o *CanvasObject) { o.InteractionPolicy.Mode = "" }, "interaction-missing", "interactionPolicy.mode"},
		{"unknown interaction mode", func(o *CanvasObject) { o.InteractionPolicy.Mode = "drag" }, "interaction-invalid", "interactionPolicy.mode"},
		{"stylus on a fixed object", func(o *CanvasObject) {
			o.InteractionPolicy = CanvasInteractionPolicy{Mode: CanvasFixed, StylusHitTest: true}
		}, "interaction-invalid", "interactionPolicy.stylusHitTest"},
		{"unknown anchor kind", func(o *CanvasObject) {
			o.SourceAnchors = []CanvasAnchor{{Kind: "slide"}}
		}, "anchor-kind", "sourceAnchors[0].kind"},
		{"lecture without an id", func(o *CanvasObject) {
			o.SourceAnchors = []CanvasAnchor{{Kind: AnchorLecture, StartMs: 1, EndMs: 2}}
		}, "anchor-invalid", "sourceAnchors[0].lectureId"},
		{"inverted lecture interval", func(o *CanvasObject) {
			o.SourceAnchors = []CanvasAnchor{{Kind: AnchorLecture, LectureID: "lecture-1", StartMs: 9_000, EndMs: 1_000}}
		}, "anchor-invalid", "sourceAnchors[0].startMs"},
		{"lecture interval past the bound", func(o *CanvasObject) {
			o.SourceAnchors = []CanvasAnchor{{Kind: AnchorLecture, LectureID: "lecture-1", EndMs: CanvasMaxLectureMs + 1}}
		}, "anchor-invalid", "sourceAnchors[0].startMs"},
		{"pdf page zero", func(o *CanvasObject) {
			o.SourceAnchors = []CanvasAnchor{{Kind: AnchorPDF, File: "讲义.pdf"}}
		}, "anchor-invalid", "sourceAnchors[0].page"},
		{"pdf range backwards", func(o *CanvasObject) {
			o.SourceAnchors = []CanvasAnchor{{Kind: AnchorPDF, File: "讲义.pdf", Page: 5, PageEnd: 3}}
		}, "anchor-invalid", "sourceAnchors[0].pageEnd"},
		{"pdf outside the course folder", func(o *CanvasObject) {
			o.SourceAnchors = []CanvasAnchor{{Kind: AnchorPDF, File: "../讲义.pdf", Page: 1}}
		}, "anchor-invalid", "sourceAnchors[0].file"},
		{"courseware with a page", func(o *CanvasObject) {
			o.SourceAnchors = []CanvasAnchor{{Kind: AnchorCourseware, File: "课件.pptx", Page: 2}}
		}, "anchor-invalid", "sourceAnchors[0].page"},
		{"transcript without text", func(o *CanvasObject) {
			o.SourceAnchors = []CanvasAnchor{{Kind: AnchorTranscript, LectureID: "lecture-1", StartMs: 1_000, EndMs: 2_000}}
		}, "anchor-invalid", "sourceAnchors[0].text"},
		{"transcript of no length", func(o *CanvasObject) {
			o.SourceAnchors = []CanvasAnchor{{Kind: AnchorTranscript, LectureID: "lecture-1", StartMs: 2_000, EndMs: 2_000, Text: "嗯"}}
		}, "anchor-invalid", "sourceAnchors[0].startMs"},
		{"handwriting without strokes", func(o *CanvasObject) {
			o.SourceAnchors = []CanvasAnchor{{Kind: AnchorStroke}}
		}, "anchor-invalid", "sourceAnchors[0].strokeIds"},
		{"anchor from another kind", func(o *CanvasObject) {
			o.SourceAnchors = []CanvasAnchor{{Kind: AnchorLecture, LectureID: "lecture-1", File: "讲义.pdf"}}
		}, "anchor-invalid", "sourceAnchors[0].file"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o := canvasObject()
			c.change(&o)
			err := o.Validate()
			if err == nil {
				t.Fatal("Validate accepted an invalid object")
			}
			if got := Code(err); got != c.code {
				t.Fatalf("code %q, want %q (%v)", got, c.code, err)
			}
			var problems *Error
			if !errors.As(err, &problems) {
				t.Fatalf("error %T is not a diagnosable problem list", err)
			}
			if !strings.Contains(err.Error(), c.path) {
				t.Fatalf("message %q does not name %q", err, c.path)
			}
		})
	}
}

func TestCanvasObjectValidateAcceptsTheContractShape(t *testing.T) {
	// The JSON names are the contract spec 0032 publishes; decoding a literal
	// fails if a field is renamed.
	raw := `{
		"id": "obj-1", "type": "review-lab.card", "schemaVersion": 1,
		"geometry": {"x": 40, "y": 60, "w": 320, "h": 240},
		"content": {"title": "符号解释", "text": "先展开定义。", "data": {"steps": 3}},
		"sourceAnchors": [{"kind": "pdf", "file": "讲义.pdf", "page": 3, "pageEnd": 5}],
		"createdBy": {"pluginId": "review-lab", "pluginVersion": "1.0.0"},
		"revision": 2,
		"interactionPolicy": {"mode": "move", "stylusHitTest": true}
	}`
	var o CanvasObject
	if err := json.Unmarshal([]byte(raw), &o); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if err := o.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if o.Geometry.W != 320 || o.SourceAnchors[0].PageEnd != 5 || o.Content.Data["steps"] != float64(3) {
		t.Fatalf("decoded %+v", o)
	}
	if !o.InteractionPolicy.StylusHitTest || o.CreatedBy.PluginID != "review-lab" || o.Revision != 2 {
		t.Fatalf("decoded %+v", o)
	}
}

func TestCanvasApplyCommitsOneBatch(t *testing.T) {
	c := NewCanvas()
	first := canvasObject()
	second := canvasObject()
	second.ID = "obj-2"
	second.Revision = 1
	commit, err := c.Apply(context.Background(), CanvasBatch{Revision: 0, Objects: []CanvasObject{first, second}, Label: "插入复习卡片"})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if commit.Revision != 1 || c.Revision() != 1 {
		t.Fatalf("revision %d/%d, want 1", commit.Revision, c.Revision())
	}
	if got := c.Objects(); len(got) != 2 || got[0].ID != "obj-1" || got[1].ID != "obj-2" {
		t.Fatalf("objects %+v", got)
	}
	if _, ok := c.Object("obj-2"); !ok {
		t.Fatal("obj-2 was not written")
	}
}

func TestCanvasApplyRejectsTheWholeBatchOnOneInvalidObject(t *testing.T) {
	c := NewCanvas()
	good := canvasObject()
	bad := canvasObject()
	bad.ID = "obj-2"
	bad.Content.Title = ""
	before := snapshot(c)

	_, err := c.Apply(context.Background(), CanvasBatch{Revision: 0, Objects: []CanvasObject{good, bad}})
	if err == nil {
		t.Fatal("Apply committed a batch with an invalid object")
	}
	if got := Code(err); got != "content-invalid" {
		t.Fatalf("code %q, want content-invalid (%v)", got, err)
	}
	if !strings.Contains(err.Error(), "objects[1].content.title") {
		t.Fatalf("message %q does not name the offending object", err)
	}
	assertUnchanged(t, c, before)
}

func TestCanvasApplyRejectsAStaleRevision(t *testing.T) {
	c := NewCanvas()
	if _, err := c.Apply(context.Background(), CanvasBatch{Revision: 0, Objects: []CanvasObject{canvasObject()}}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	before := snapshot(c)

	update := canvasObject()
	update.Revision = 2
	update.Content.Text = "改过的解释"
	_, err := c.Apply(context.Background(), CanvasBatch{Revision: 0, Objects: []CanvasObject{update}})
	if !errors.Is(err, ErrCanvasConflict) {
		t.Fatalf("error %v, want ErrCanvasConflict", err)
	}
	assertUnchanged(t, c, before)
}

func TestCanvasApplyRejectsAnObjectRevisionMismatch(t *testing.T) {
	c := NewCanvas()
	if _, err := c.Apply(context.Background(), CanvasBatch{Revision: 0, Objects: []CanvasObject{canvasObject()}}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	before := snapshot(c)

	cases := map[string]CanvasObject{
		"a create must start at 1": func() CanvasObject {
			o := canvasObject()
			o.ID = "obj-2"
			o.Revision = 2
			return o
		}(),
		"an update moves one step": func() CanvasObject {
			o := canvasObject()
			o.Revision = 3
			return o
		}(),
	}
	for name, o := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := c.Apply(context.Background(), CanvasBatch{Revision: 1, Objects: []CanvasObject{o}})
			if !errors.Is(err, ErrCanvasConflict) {
				t.Fatalf("error %v, want ErrCanvasConflict", err)
			}
			assertUnchanged(t, c, before)
		})
	}
}

func TestCanvasApplyRejectsCancellation(t *testing.T) {
	c := NewCanvas()
	if _, err := c.Apply(context.Background(), CanvasBatch{Revision: 0, Objects: []CanvasObject{canvasObject()}}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	before := snapshot(c)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	update := canvasObject()
	update.Revision = 2
	_, err := c.Apply(ctx, CanvasBatch{Revision: 1, Objects: []CanvasObject{update}})
	if !errors.Is(err, ErrCanvasCanceled) {
		t.Fatalf("error %v, want ErrCanvasCanceled", err)
	}
	assertUnchanged(t, c, before)
}

func TestCanvasApplyRejectsAnEmptyOrOversizedBatch(t *testing.T) {
	c := NewCanvas()
	if _, err := c.Apply(context.Background(), CanvasBatch{Revision: 0}); Code(err) != "batch-empty" {
		t.Fatalf("empty batch error %v, want batch-empty", err)
	}
	objects := make([]CanvasObject, CanvasMaxObjects+1)
	for i := range objects {
		objects[i] = canvasObject()
	}
	if _, err := c.Apply(context.Background(), CanvasBatch{Revision: 0, Objects: objects}); !errors.Is(err, ErrCanvasLimit) {
		t.Fatalf("oversized batch error %v, want ErrCanvasLimit", err)
	}
	if c.Revision() != 0 || len(c.Objects()) != 0 {
		t.Fatal("a rejected batch changed the canvas")
	}
}

func TestCanvasApplyDeleteAndUndo(t *testing.T) {
	ctx := context.Background()
	c := NewCanvas()
	if _, err := c.Apply(ctx, CanvasBatch{Revision: 0, Objects: []CanvasObject{canvasObject()}, Label: "插入"}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	commit, err := c.Apply(ctx, CanvasBatch{Revision: 1, Delete: []string{"obj-1"}, Label: "删除"})
	if err != nil {
		t.Fatalf("Apply delete: %v", err)
	}
	if len(commit.Deleted) != 1 || commit.Deleted[0] != "obj-1" {
		t.Fatalf("commit %+v", commit)
	}
	if _, ok := c.Object("obj-1"); ok {
		t.Fatal("obj-1 survived its deletion")
	}

	undo, err := c.Undo(ctx)
	if err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if undo.Label != "删除" || undo.Revision != 3 {
		t.Fatalf("undo %+v", undo)
	}
	if o, ok := c.Object("obj-1"); !ok || o.Content.Title != "符号解释" {
		t.Fatalf("undo did not restore the object: %+v", o)
	}

	// Undo is itself a revision other windows see, and it unwinds in order.
	if _, err := c.Undo(ctx); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if _, ok := c.Object("obj-1"); ok {
		t.Fatal("the second undo did not remove the created object")
	}
	if _, err := c.Undo(ctx); !errors.Is(err, ErrCanvasNothingToUndo) {
		t.Fatalf("error %v, want ErrCanvasNothingToUndo", err)
	}
	if c.Revision() != 4 {
		t.Fatalf("revision %d, want 4", c.Revision())
	}
}

func TestCanvasApplyRejectsDeletingAnObjectThatIsGone(t *testing.T) {
	c := NewCanvas()
	_, err := c.Apply(context.Background(), CanvasBatch{Revision: 0, Delete: []string{"obj-1"}})
	if !errors.Is(err, ErrCanvasConflict) {
		t.Fatalf("error %v, want ErrCanvasConflict", err)
	}
	if c.Revision() != 0 {
		t.Fatalf("revision %d, want 0", c.Revision())
	}
}

func TestCanvasFallbackKeepsAnObjectReadableWithoutItsPlugin(t *testing.T) {
	o := canvasObject()
	o.SourceAnchors = []CanvasAnchor{
		{Kind: AnchorLecture, LectureID: "lecture-1", StartMs: 12_000, EndMs: 34_000},
		{Kind: AnchorPDF, File: "第 2 讲/讲义.pdf", Page: 3, PageEnd: 5},
		{Kind: AnchorTranscript, LectureID: "lecture-1", StartMs: 12_000, EndMs: 20_000, Text: "所以这里用的是链式法则。"},
		{Kind: AnchorStroke, StrokeIDs: []string{"stroke-1"}},
	}
	o.Content.Text = strings.Repeat("解", CanvasFallbackSummary+50)

	f := FallbackFor(o)
	if f.ID != "obj-1" || f.Type != "review-lab.card" || f.PluginID != "review-lab" || f.Revision != 1 {
		t.Fatalf("fallback lost the object identity: %+v", f)
	}
	if f.Title != "符号解释" {
		t.Fatalf("title %q", f.Title)
	}
	wantSource := "课程视频 00:12–00:34；PDF 第 3–5 页；字幕 00:12–00:20 等 4 处"
	if f.Source != wantSource {
		t.Fatalf("source %q, want %q", f.Source, wantSource)
	}
	if n := len([]rune(f.Summary)); n != CanvasFallbackSummary+1 || !strings.HasSuffix(f.Summary, "…") {
		t.Fatalf("summary has %d runes, want a marked cut at %d", n, CanvasFallbackSummary)
	}
	if f.Data["steps"] != 3 {
		t.Fatalf("exportable data %+v", f.Data)
	}
	if len(f.Anchors) != 4 {
		t.Fatalf("anchors %+v", f.Anchors)
	}

	// Data is a copy: editing the fallback never edits the stored object.
	f.Data["steps"] = 99
	if o.Content.Data["steps"] != 3 {
		t.Fatal("the fallback shares the object's data map")
	}
}

func TestCanvasFallbackOfAnEmptyObjectStillNamesIt(t *testing.T) {
	f := FallbackFor(CanvasObject{ID: "obj-9", Type: "review-lab.card", SchemaVersion: 1, Revision: 4})
	if f.ID != "obj-9" || f.Type != "review-lab.card" || f.Revision != 4 {
		t.Fatalf("fallback %+v", f)
	}
	if f.Title != "" || f.Source != "" || f.Summary != "" || f.Data != nil {
		t.Fatalf("fallback invented content: %+v", f)
	}
}

// canvasSnapshot is the state a rejected batch must leave untouched.
type canvasSnapshot struct {
	objects  []CanvasObject
	revision int64
	undo     int
}

func snapshot(c *Canvas) canvasSnapshot {
	return canvasSnapshot{objects: c.Objects(), revision: c.Revision(), undo: len(c.undo)}
}

// assertUnchanged proves a rejected batch wrote no object, moved no revision and
// pushed no undo entry, so no failure commits a partial change.
func assertUnchanged(t *testing.T, c *Canvas, want canvasSnapshot) {
	t.Helper()
	if got := c.Objects(); !reflect.DeepEqual(got, want.objects) {
		t.Fatalf("objects changed on failure: %+v, want %+v", got, want.objects)
	}
	if got := c.Revision(); got != want.revision {
		t.Fatalf("revision %d on failure, want %d", got, want.revision)
	}
	if got := len(c.undo); got != want.undo {
		t.Fatalf("undo stack grew from %d to %d on failure", want.undo, got)
	}
}
