package study

import (
	"reflect"
	"strings"
	"testing"
)

func penStroke(id string, pressure float64, brush *Brush, anchor string) *Stroke {
	return &Stroke{ID: id, Gesture: id, Layer: "main", Anchor: anchor, Color: 0xff111111, Width: 20, Brush: brush,
		Points: []InkPoint{{Pt{10, 50}, pressure}, {Pt{100, 50}, pressure}}}
}

func region(y float64) []Pt { return []Pt{{30, y}, {80, y}, {80, y + 2}, {30, y + 2}} }

func ids(s ...string) map[string]bool {
	m := map[string]bool{}
	for _, v := range s {
		m[v] = true
	}
	return m
}

func TestLegacyPagedAnchor(t *testing.T) {
	a := AnchorFrom(M{"id": "object-page", "object_id": "page", "kind": "pdfdoc", "page": 3.0})
	if a.JSON()["kind"] != "pdf" || a.Page != 3 || a.ObjectID != "page" {
		t.Fatalf("%+v", a)
	}
}

func TestOutlineSelectsByPaintedWidth(t *testing.T) {
	full := DefaultBrush
	full.Pressure = 1
	d := NewDocument()
	d.Strokes = []*Stroke{penStroke("wide", 1, &full, "")}
	if !reflect.DeepEqual(d.StrokesInPolygon(region(57)), ids("wide")) {
		t.Fatal("wide")
	}
	d.Strokes = []*Stroke{penStroke("thin", 0, &full, "")}
	if len(d.StrokesInPolygon(region(57))) != 0 || !reflect.DeepEqual(d.StrokesInPolygon(region(49)), ids("thin")) {
		t.Fatal("thin")
	}
	d.Strokes = []*Stroke{penStroke("cap", 1, nil, "")}
	if !reflect.DeepEqual(d.StrokesInPolygon([]Pt{{1, 47}, {4, 47}, {4, 53}, {1, 53}}), ids("cap")) {
		t.Fatal("cap")
	}
}

func TestRotationRoundTripsAndUndoes(t *testing.T) {
	d := NewDocument()
	d.Cards = []*Card{{ID: "image", Type: "image", Layer: "main", Rect: RectLTWH(10, 20, 100, 200)}}
	d.Strokes = []*Stroke{penStroke("ink", 1, nil, "image")}
	original := d.Strokes[0].Points[0].P
	d.RotateSelection(ids("image"), ids("ink"), Pt{})
	if d.Cards[0].Rect != RectLTWH(-220, 10, 200, 100) || d.Cards[0].QuarterTurns != 1 || d.Strokes[0].Points[0].P != (Pt{-50, 10}) {
		t.Fatalf("%+v %v", d.Cards[0].Rect, d.Strokes[0].Points[0].P)
	}
	r, _ := DocumentFrom(obj(Clone(d.JSON())))
	if r.Cards[0].QuarterTurns != 1 || r.JSON()["version"] != 4 {
		t.Fatal("round trip")
	}
	d.Undo()
	if d.Strokes[0].Points[0].P != original || d.Cards[0].QuarterTurns != 0 {
		t.Fatal("undo")
	}
	d.Redo()
	for range 3 {
		d.RotateSelection(ids("image"), ids("ink"), Pt{})
	}
	if d.Cards[0].QuarterTurns != 0 || d.Strokes[0].Points[0].P != original {
		t.Fatal("four turns")
	}
}

func TestUnsupportedSelectionDoesNotRotate(t *testing.T) {
	d := NewDocument()
	d.Cards = []*Card{{ID: "video", Type: "video", Layer: "main", Rect: RectLTWH(0, 0, 200, 150)}}
	d.Strokes = []*Stroke{penStroke("ink", 1, nil, "")}
	d.RotateSelection(ids("video"), ids("ink"), Pt{})
	if d.CanRotateSelection(ids("video"), ids("ink")) || d.Edit != 0 || d.CanUndo() {
		t.Fatal("a video rotated")
	}
	d.Cards = []*Card{{ID: "image", Type: "image", Layer: "main", Rect: RectLTWH(0, 0, 200, 150)}}
	d.Layers[0].Locked = true
	if d.CanRotateSelection(ids("image"), ids("ink")) {
		t.Fatal("a locked image may rotate")
	}
}

func pdfRoot() *Card {
	return &Card{ID: "pdf", Type: "pdfdoc", Layer: "main", Rect: RectLTWH(40, 180, 600, 472), Title: "操作系统课件.pdf",
		Page: 2, PageCount: 3, PageAspect: 1.5, Source: "study-assets/book/source.pdf", Path: "study-assets/book/page-2.png",
		Responses: map[string]string{}}
}

func pdfPage(n int) M {
	return obj(Clone(M{"file": "study-assets/book/page-" + str(float64(n)) + ".png", "width": 1200, "height": 800, "text": "Page", "boxes": []any{}}))
}

func dot(anchor string, page int, p Pt) *Stroke {
	id := NewID()
	return &Stroke{ID: id, Gesture: id, Layer: "main", Anchor: anchor, Page: page, Points: []InkPoint{{p, .7}}, Color: 0xff000000, Width: 3}
}

func TestPDFColumnKeepsPageInk(t *testing.T) {
	d := NewDocument()
	root := pdfRoot()
	d.Cards = append(d.Cards, root)
	original := dot(root.ID, 2, Pt{140, 316})
	d.Strokes = append(d.Strokes, original)
	if err := d.BeginPDFColumn(root); err != nil {
		t.Fatal(err)
	}
	for n := 1; n <= 3; n++ {
		if err := d.AppendPDFPage(root, n, pdfPage(n)); err != nil {
			t.Fatal(err)
		}
	}
	pages := d.PDFPages(root.ID)
	if len(pages) != 3 || pages[0].Page != 1 || pages[2].Page != 3 || pages[1].Rect.T <= pages[0].Rect.B {
		t.Fatal("pages")
	}
	if d.Strokes[0].Anchor != pages[1].ID || d.Strokes[0].ID != original.ID {
		t.Fatal("the ink did not move to its page")
	}
	added := dot(pages[2].ID, 0, pages[2].Rect.TopLeft().Add(Pt{100, 120}))
	d.AddStroke(added)
	d.TransformCards(ids(pages[2].ID), pages[2].Rect, pages[2].Rect.Shift(Pt{100, 50}))
	reopened, _ := DocumentFrom(obj(Clone(d.JSON())))
	if err := reopened.CollapsePDFColumn(root.ID); err != nil {
		t.Fatal(err)
	}
	if len(reopened.Cards) != 1 || reopened.Strokes[0].Page != 2 || reopened.Strokes[1].Page != 3 {
		t.Fatal("collapse")
	}
	for _, s := range reopened.Strokes {
		if s.Anchor != root.ID {
			t.Fatal("ink left on a page")
		}
	}
	if reopened.Strokes[0].Points[0].P.Sub(original.Points[0].P).Dist() >= .001 {
		t.Fatal("the ink moved")
	}
	saved, _ := DocumentFrom(obj(Clone(reopened.JSON())))
	again := saved.Cards[0]
	saved.BeginPDFColumn(again)
	for n := 1; n <= 3; n++ {
		saved.AppendPDFPage(again, n, pdfPage(n))
	}
	if saved.Strokes[0].ID != original.ID || saved.Strokes[1].ID != added.ID {
		t.Fatal("ids")
	}
	for _, s := range saved.Strokes {
		if !saved.StrokeVisible(s) {
			t.Fatal("hidden ink")
		}
	}
}

func runsOf(text string) string {
	names := []string{"text", "bold", "code", "math", "displayMath"}
	var out []string
	for _, r := range OutlineInline(text) {
		out = append(out, names[r.Kind]+":"+r.Text)
	}
	return strings.Join(out, " | ")
}

func TestOutlineInline(t *testing.T) {
	for in, want := range map[string]string{
		`**粗** ` + "`c`" + ` $a^2$ \(b\) $$c$$ \[d\] 价格 $5 和 $6`: "bold:粗 | text:  | code:c | text:  | math:a^2 | text:  | math:b | " +
			`text:  | displayMath:c | text:  | displayMath:d | text: 价格 $5 和 $6`,
		`即 $$x=1$$。`: "text:即  | displayMath:x=1 | text:。",
		`$ x $ 不是公式`: `text:$ x $ 不是公式`,
	} {
		if got := runsOf(in); got != want {
			t.Errorf("%s\n got %s\nwant %s", in, got, want)
		}
	}
}

func TestSelectionGeometry(t *testing.T) {
	r := RectLTWH(100, 100, 200, 100)
	if HandleAt(r, Pt{101, 99}, 12) != HandleTopLeft || HandleAt(r, Pt{200, 102}, 12) != HandleTop || HandleAt(r, Pt{200, 150}, 12) != -1 {
		t.Fatal("handles")
	}
	if got := ResizeSelection(r, HandleLeft, Pt{500, 0}, 20); got.L != 280 {
		t.Fatalf("minimum %v", got)
	}
	m := SelectionMenuRect(r, RectLTWH(0, 0, 800, 600), nil)
	if m == nil || m.Overlaps(r.Inflate(12)) {
		t.Fatal("the menu covers the selection")
	}
	if _, ok := FloatingPosition(r, RectLTWH(0, 0, 800, 600), nil, 40, 40, 8); !ok {
		t.Fatal("no room for a control")
	}
}
