package design

import "github.com/egoist/mygo/ui"

// kappa is the control-point distance of a cubic Bézier quarter circle.
const kappa = 0.5522847498

// RoundedRectPath is r with corners of radius rad, as a path to stroke or
// fill with a gradient.
func RoundedRectPath(r ui.Rect, rad float32) *ui.Path {
	if m := min(r.W, r.H) / 2; rad > m {
		rad = m
	}
	k := rad * kappa
	x0, y0, x1, y1 := r.X, r.Y, r.X+r.W, r.Y+r.H
	p := &ui.Path{}
	p.MoveTo(x0+rad, y0).LineTo(x1-rad, y0).
		CubeTo(x1-rad+k, y0, x1, y0+rad-k, x1, y0+rad).
		LineTo(x1, y1-rad).
		CubeTo(x1, y1-rad+k, x1-rad+k, y1, x1-rad, y1).
		LineTo(x0+rad, y1).
		CubeTo(x0+rad-k, y1, x0, y1-rad+k, x0, y1-rad).
		LineTo(x0, y0+rad).
		CubeTo(x0, y0+rad-k, x0+rad-k, y0, x0+rad, y0).
		Close()
	return p
}

// ArcPath is a circular arc around (cx, cy), from start sweeping sweep
// radians clockwise from the right, as Canvas.drawArc strokes it.
func ArcPath(cx, cy, radius, start, sweep float64) *ui.Path {
	p := &ui.Path{}
	n := int(abs(sweep)/(3.14159265/2)) + 1
	step := sweep / float64(n)
	x, y := cx+radius*cos(start), cy+radius*sin(start)
	p.MoveTo(float32(x), float32(y))
	a := start
	for i := 0; i < n; i++ {
		b := a + step
		k := 4.0 / 3 * tan((b-a)/4) * radius
		x1, y1 := cx+radius*cos(a), cy+radius*sin(a)
		x2, y2 := cx+radius*cos(b), cy+radius*sin(b)
		p.CubeTo(float32(x1-k*sin(a)), float32(y1+k*cos(a)), float32(x2+k*sin(b)), float32(y2-k*cos(b)), float32(x2), float32(y2))
		a = b
	}
	return p
}
