package study

import (
	"math"
	"testing"
)

func polyTrace(corners []Pt) []Pt {
	var out []Pt
	for e := range corners {
		for i := 0; i < 20; i++ {
			p := corners[e].Lerp(corners[(e+1)%len(corners)], float64(i)/20)
			out = append(out, p.Add(Pt{math.Sin(float64(i)*2) * .6, math.Cos(float64(i)) * .5}))
		}
	}
	return append(out, corners[0])
}

func ellipseTrace(x, y, angle, turns float64) []Pt {
	var out []Pt
	for i := 0; i <= 100; i++ {
		a := float64(i) * math.Pi * 2 * turns / 100
		p := Pt{x * math.Cos(a), y * math.Sin(a)}.Mul(1 + .02*math.Sin(float64(i)*3))
		out = append(out, Pt{350 + p.X*math.Cos(angle) - p.Y*math.Sin(angle), 350 + p.X*math.Sin(angle) + p.Y*math.Cos(angle)})
	}
	return out
}

var wavyLine = []Pt{{250, 350}, {280, 353}, {310, 348}, {350, 352}, {390, 350}, {430, 352}, {470, 351}}

func scaled(ps []Pt, s float64) []Pt {
	out := make([]Pt, len(ps))
	for i, p := range ps {
		out[i] = p.Div(s)
	}
	return out
}

func TestShapesFitAtEveryZoom(t *testing.T) {
	examples := map[string][]Pt{
		"line":          wavyLine,
		"circle":        ellipseTrace(90, 90, .6, 1),
		"ellipse":       ellipseTrace(100, 60, .6, 1),
		"triangle":      polyTrace([]Pt{{300, 220}, {410, 400}, {200, 400}}),
		"rectangle":     polyTrace([]Pt{{220, 250}, {450, 270}, {440, 400}, {210, 380}}),
		"quadrilateral": polyTrace([]Pt{{250, 250}, {390, 250}, {450, 400}, {180, 400}}),
	}
	for kind, trace := range examples {
		for _, zoom := range []float64{.25, 1, 4} {
			s := RecognizeShape(scaled(trace, zoom), zoom)
			if s == nil || s.Kind != kind {
				t.Fatalf("%s at zoom %v: got %v", kind, zoom, s)
			}
			if kind != "line" && s.Points[0] != s.Points[len(s.Points)-1] {
				t.Fatalf("%s is not closed", kind)
			}
		}
	}
	f := RecognizeShape(examples["rectangle"], 1).Points
	a, b := f[1].Sub(f[0]), f[2].Sub(f[1])
	if math.Abs(a.Dot(b)) >= .001 || math.Abs(a.Y) <= 5 {
		t.Fatal("the rectangle is not square or lost its turn")
	}
}

func TestShapesRejectScribbles(t *testing.T) {
	dots := make([]Pt, 20)
	for i := range dots {
		dots[i] = Pt{40, 40}
	}
	var spiral []Pt
	for i := 0; i < 90; i++ {
		f := float64(i)
		spiral = append(spiral, Pt{300 + f*math.Cos(f/8), 300 + f*math.Sin(f/8)})
	}
	for i, trace := range [][]Pt{
		dots, ellipseTrace(3, 3, .6, 1), ellipseTrace(100, 60, .6, .65), ellipseTrace(100, 60, .6, 2), spiral,
		{{200, 200}, {400, 210}, {200, 240}, {400, 260}, {200, 300}, {390, 330}},
		polyTrace([]Pt{{100, 100}, {300, 300}, {100, 300}, {300, 100}}),
	} {
		if s := RecognizeShape(trace, 1); s != nil {
			t.Fatalf("trace %d fitted a %s", i, s.Kind)
		}
	}
}

func TestShapesAdjust(t *testing.T) {
	l := RecognizeShape(wavyLine, 1)
	moved := l.Adjust(wavyLine[len(wavyLine)-1], Pt{520, 420})
	if moved[0] != wavyLine[0] || moved[1] != (Pt{520, 420}) {
		t.Fatalf("line end %v", moved)
	}
	c := RecognizeShape(ellipseTrace(90, 90, .6, 1), 1)
	mc := c.Adjust(Pt{440, 350}, Pt{350, 530})
	want := c.Points[24].Sub(c.Points[0]).Dist() * 2
	if got := mc[24].Sub(mc[0]).Dist(); math.Abs(got-want) > 2 {
		t.Fatalf("circle scaled to %v, want %v", got, want)
	}
}

func TestArrowsAndPolygons(t *testing.T) {
	arrow := []Pt{{100, 200}, {400, 200}, {330, 145}, {400, 200}, {330, 255}}
	var trace []Pt
	for k := 1; k < len(arrow); k++ {
		for i := 0; i < 30; i++ {
			trace = append(trace, arrow[k-1].Lerp(arrow[k], float64(i)/30))
		}
	}
	trace = append(trace, arrow[len(arrow)-1])
	if s := RecognizeShape(trace, 1); s == nil || s.Kind != "arrow" {
		t.Fatalf("arrow fitted %v", s)
	}
	for _, sides := range []int{5, 6} {
		var corners []Pt
		for i := 0; i < sides; i++ {
			a := float64(i) * 2 * math.Pi / float64(sides)
			corners = append(corners, Pt{300 + 100*math.Cos(a), 300 + 100*math.Sin(a)})
		}
		if s := RecognizeShape(polyTrace(corners), 1); s == nil || s.Kind != "polygon" {
			t.Fatalf("%d sides fitted %v", sides, s)
		}
	}
	if s := RecognizeShape([]Pt{{100, 200}, {400, 200}, {330, 145}, {450, 260}, {500, 200}}, 1); s != nil {
		t.Fatalf("a zigzag fitted %s", s.Kind)
	}
}

func TestRoughShapes(t *testing.T) {
	rect := polyTrace([]Pt{{200, 240}, {440, 250}, {430, 410}, {210, 400}})
	var rough []Pt
	for i := 0; i < len(rect)-3; i++ {
		f := float64(i)
		rough = append(rough, rect[i].Add(Pt{6 * math.Sin(f*.39), 5 * math.Cos(f*.31)}))
	}
	if s := RecognizeShape(rough, 1); s == nil || s.Kind != "rectangle" {
		t.Fatalf("rough rectangle fitted %v", s)
	}
	if s := RecognizeShape(ellipseTrace(100, 96, .6, .9), 1); s == nil || s.Kind != "circle" {
		t.Fatalf("gapped circle fitted %v", s)
	}
}
