// Package study is the study canvas's model: the document and its cards,
// ink, layers and plugin objects, the geometry of erasing, selecting and
// fitting shapes, and the pure helpers its view uses. It mirrors
// app/lib/features/study_canvas, whose files it names.
package study

import "math"

// Pt is a point or a vector in world units (Dart's Offset).
type Pt struct{ X, Y float64 }

func (p Pt) Add(q Pt) Pt             { return Pt{p.X + q.X, p.Y + q.Y} }
func (p Pt) Sub(q Pt) Pt             { return Pt{p.X - q.X, p.Y - q.Y} }
func (p Pt) Mul(s float64) Pt        { return Pt{p.X * s, p.Y * s} }
func (p Pt) Div(s float64) Pt        { return Pt{p.X / s, p.Y / s} }
func (p Pt) Neg() Pt                 { return Pt{-p.X, -p.Y} }
func (p Pt) Dist() float64           { return math.Hypot(p.X, p.Y) }
func (p Pt) Dist2() float64          { return p.X*p.X + p.Y*p.Y }
func (p Pt) Finite() bool            { return finite(p.X) && finite(p.Y) }
func (p Pt) Dot(q Pt) float64        { return p.X*q.X + p.Y*q.Y }
func (p Pt) Cross(q Pt) float64      { return p.X*q.Y - p.Y*q.X }
func (p Pt) Lerp(q Pt, t float64) Pt { return Pt{p.X + (q.X-p.X)*t, p.Y + (q.Y-p.Y)*t} }
func (p Pt) Rotate(angle float64) Pt {
	s, c := math.Sincos(angle)
	return Pt{p.X*c - p.Y*s, p.X*s + p.Y*c}
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// Rect is an axis-aligned rectangle by its edges (Dart's Rect).
type Rect struct{ L, T, R, B float64 }

// RectLTWH is Rect.fromLTWH.
func RectLTWH(l, t, w, h float64) Rect { return Rect{l, t, l + w, t + h} }

// RectPts is Rect.fromPoints.
func RectPts(a, b Pt) Rect {
	return Rect{math.Min(a.X, b.X), math.Min(a.Y, b.Y), math.Max(a.X, b.X), math.Max(a.Y, b.Y)}
}

// RectCenter is Rect.fromCenter.
func RectCenter(c Pt, w, h float64) Rect { return Rect{c.X - w/2, c.Y - h/2, c.X + w/2, c.Y + h/2} }

// RectCircle is Rect.fromCircle.
func RectCircle(c Pt, r float64) Rect { return Rect{c.X - r, c.Y - r, c.X + r, c.Y + r} }

func (r Rect) W() float64             { return r.R - r.L }
func (r Rect) H() float64             { return r.B - r.T }
func (r Rect) Center() Pt             { return Pt{(r.L + r.R) / 2, (r.T + r.B) / 2} }
func (r Rect) TopLeft() Pt            { return Pt{r.L, r.T} }
func (r Rect) TopRight() Pt           { return Pt{r.R, r.T} }
func (r Rect) BottomLeft() Pt         { return Pt{r.L, r.B} }
func (r Rect) BottomRight() Pt        { return Pt{r.R, r.B} }
func (r Rect) TopCenter() Pt          { return Pt{(r.L + r.R) / 2, r.T} }
func (r Rect) BottomCenter() Pt       { return Pt{(r.L + r.R) / 2, r.B} }
func (r Rect) CenterLeft() Pt         { return Pt{r.L, (r.T + r.B) / 2} }
func (r Rect) CenterRight() Pt        { return Pt{r.R, (r.T + r.B) / 2} }
func (r Rect) Empty() bool            { return r.L >= r.R || r.T >= r.B }
func (r Rect) Inflate(d float64) Rect { return Rect{r.L - d, r.T - d, r.R + d, r.B + d} }
func (r Rect) Shift(d Pt) Rect        { return Rect{r.L + d.X, r.T + d.Y, r.R + d.X, r.B + d.Y} }
func (r Rect) Diagonal() float64      { return math.Hypot(r.W(), r.H()) }
func (r Rect) ShortestSide() float64 {
	return math.Min(math.Abs(r.W()), math.Abs(r.H()))
}

// Overlaps is Rect.overlaps: the open rectangles share area.
func (r Rect) Overlaps(o Rect) bool {
	return !(r.R <= o.L || o.R <= r.L || r.B <= o.T || o.B <= r.T)
}

// Contains is Rect.contains: left and top edges in, right and bottom out.
func (r Rect) Contains(p Pt) bool { return p.X >= r.L && p.X < r.R && p.Y >= r.T && p.Y < r.B }

// Intersect is Rect.intersect.
func (r Rect) Intersect(o Rect) Rect {
	return Rect{math.Max(r.L, o.L), math.Max(r.T, o.T), math.Min(r.R, o.R), math.Min(r.B, o.B)}
}

// Union is Rect.expandToInclude.
func (r Rect) Union(o Rect) Rect {
	return Rect{math.Min(r.L, o.L), math.Min(r.T, o.T), math.Max(r.R, o.R), math.Max(r.B, o.B)}
}

// Bounds is the smallest rectangle around points; zero for none.
func Bounds(points []Pt) Rect {
	if len(points) == 0 {
		return Rect{}
	}
	r := RectPts(points[0], points[0])
	for _, p := range points[1:] {
		r = r.Union(RectPts(p, p))
	}
	return r
}

func clamp(v, lo, hi float64) float64 { return math.Min(math.Max(v, lo), hi) }

// SegmentDistance is the distance from p to the segment ab.
func SegmentDistance(p, a, b Pt) float64 {
	v := b.Sub(a)
	t := 0.0
	if v.Dist2() != 0 {
		t = clamp(p.Sub(a).Dot(v)/v.Dist2(), 0, 1)
	}
	return p.Sub(a).Sub(v.Mul(t)).Dist()
}

// segmentsNear reports whether segments ab and cd cross or come within r.
func segmentsNear(a, b, c, d Pt, r float64) bool {
	v, w := b.Sub(a), d.Sub(c)
	if den := v.Cross(w); math.Abs(den) > 1e-8 {
		t, u := c.Sub(a).Cross(w)/den, c.Sub(a).Cross(v)/den
		if t >= 0 && t <= 1 && u >= 0 && u <= 1 {
			return true
		}
	}
	return math.Min(math.Min(SegmentDistance(a, c, d), SegmentDistance(b, c, d)),
		math.Min(SegmentDistance(c, a, b), SegmentDistance(d, a, b))) <= r
}

// InPolygon is the even-odd test of p against polygon.
func InPolygon(p Pt, polygon []Pt) bool {
	yes := false
	for i, j := 0, len(polygon)-1; i < len(polygon); j, i = i, i+1 {
		a, b := polygon[i], polygon[j]
		if (a.Y > p.Y) != (b.Y > p.Y) && p.X < (b.X-a.X)*(p.Y-a.Y)/(b.Y-a.Y)+a.X {
			yes = !yes
		}
	}
	return yes
}
