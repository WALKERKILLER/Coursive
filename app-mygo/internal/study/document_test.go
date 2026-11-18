package study

import (
	"math"
	"strings"
	"testing"
)

func line(layer, anchor string) *Stroke {
	s := &Stroke{ID: NewID(), Layer: layer, Anchor: anchor, Color: 0xff111111, Width: 3}
	s.Gesture = s.ID
	for i := 0; i <= 200; i++ {
		s.Points = append(s.Points, InkPoint{Pt{float64(i), 50}, .7})
	}
	return s
}

func roundTrip(t *testing.T, d *Document) *Document {
	t.Helper()
	r, err := DocumentFrom(obj(Clone(d.JSON())))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestLockedAnnotationPinsItsCard(t *testing.T) {
	d := NewDocument()
	card := &Card{ID: "card", Type: "text", Layer: "main", Rect: RectLTWH(0, 0, 200, 100)}
	d.Cards = append(d.Cards, card)
	d.Layers = append(d.Layers, Layer{ID: "annotation", Name: "批注", Visible: true, Locked: true})
	d.AddStroke(line("annotation", "card"))
	d.TransformCards(map[string]bool{"card": true}, card.Rect, card.Rect.Shift(Pt{50, 50}))
	if card.Rect.TopLeft() != (Pt{}) || d.Strokes[0].Points[0].P != (Pt{0, 50}) {
		t.Fatal("a card moved while its annotation layer was locked")
	}
	d.Layers[1].Locked, d.Layers[1].Visible = false, false
	d.TransformCards(map[string]bool{"card": true}, card.Rect, card.Rect.Shift(Pt{50, 50}))
	if card.Rect.TopLeft() != (Pt{50, 50}) || d.Strokes[0].Points[0].P != (Pt{50, 100}) {
		t.Fatal("the card and its ink did not move")
	}
}

func TestLongGesturesChunkAndUndoAsOne(t *testing.T) {
	d := NewDocument()
	d.AddStroke(line("main", ""))
	if len(d.Strokes) != 4 {
		t.Fatalf("%d chunks, want 4", len(d.Strokes))
	}
	for _, s := range d.Strokes {
		if len(s.Points) > 65 {
			t.Fatal("a chunk is longer than 65 points")
		}
	}
	r := roundTrip(t, d)
	last := r.Strokes[len(r.Strokes)-1]
	if last.Points[len(last.Points)-1].P != (Pt{200, 50}) {
		t.Fatal("the last point did not round trip")
	}
	d.Undo()
	if len(d.Strokes) != 0 {
		t.Fatal("undo left ink")
	}
	d.Redo()
	if len(d.Strokes) != 4 {
		t.Fatal("redo did not restore the chunks")
	}
}

func TestEraserSplitsAndRespectsLayers(t *testing.T) {
	d := NewDocument()
	d.AddStroke(line("main", ""))
	d.Checkpoint()
	if !d.Erase(Pt{100, 0}, Pt{100, 100}, 10) {
		t.Fatal("nothing erased")
	}
	for _, s := range d.Strokes {
		for _, p := range s.Points {
			if math.Abs(p.P.X-100) <= 11 {
				t.Fatal("ink left under the eraser")
			}
		}
	}
	d.Undo()
	found := false
	for _, s := range d.Strokes {
		for _, p := range s.Points {
			found = found || p.P.X == 100
		}
	}
	if !found {
		t.Fatal("undo did not restore the erased ink")
	}
	d.Layers[0].Locked = true
	if d.Erase(Pt{0, 50}, Pt{200, 50}, 20) {
		t.Fatal("erased a locked layer")
	}
	d.Layers[0].Locked, d.Layers[0].Visible = false, false
	if d.Erase(Pt{0, 50}, Pt{200, 50}, 20) {
		t.Fatal("erased a hidden layer")
	}
}

func TestResizeMovesInkAndContextSkipsHidden(t *testing.T) {
	d := NewDocument()
	d.Cards = append(d.Cards, &Card{ID: "card", Type: "text", Layer: "main", Rect: RectLTWH(0, 0, 200, 100), Text: "有界的课程材料", Title: "课件"})
	d.AddStroke(line("main", "card"))
	d.Checkpoint()
	d.TransformCards(map[string]bool{"card": true}, RectLTWH(0, 0, 200, 100), RectLTWH(50, 50, 400, 200))
	first, last := d.Strokes[0], d.Strokes[len(d.Strokes)-1]
	if first.Points[0].P != (Pt{50, 150}) || last.Points[len(last.Points)-1].P != (Pt{450, 150}) {
		t.Fatal("the ink did not follow the card")
	}
	if !strings.Contains(d.ContextText(map[string]bool{"card": true}), "有界的课程材料") {
		t.Fatal("context lacks the card")
	}
	d.Layers[0].Visible = false
	if strings.Contains(d.ContextText(map[string]bool{"card": true}), "有界的课程材料") {
		t.Fatal("context has a hidden card")
	}
}

func TestInkOnDark(t *testing.T) {
	if got := InkOnDark(0xffffd34f, "highlighter"); got != 0xffffd34f {
		t.Fatal("a highlighter changed")
	}
	if got := InkOnDark(0xffeeeeee, "pen"); got != 0xffeeeeee {
		t.Fatal("light ink changed")
	}
	_, _, l, _ := hsl(InkOnDark(0xff25344a, "pen"))
	if l < .73 || l > .93 {
		t.Fatalf("dark ink lightness %v", l)
	}
}

func TestLassoSelectsWholeGestures(t *testing.T) {
	d := NewDocument()
	d.AddStroke(line("main", ""))
	box := []Pt{{90, 40}, {110, 40}, {110, 60}, {90, 60}}
	ids := d.StrokesInPolygon(box)
	if len(ids) != len(d.Strokes) {
		t.Fatalf("selected %d of %d chunks of one gesture", len(ids), len(d.Strokes))
	}
	if len(d.StrokesInPolygon([]Pt{{0, 0}, {10, 0}, {10, 10}})) != 0 {
		t.Fatal("a loop away from the ink selected it")
	}
	if !d.EraseLasso(box) || len(d.Strokes) != 0 {
		t.Fatal("lasso erase left the gesture")
	}
}
