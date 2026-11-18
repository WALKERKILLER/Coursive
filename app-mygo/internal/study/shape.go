package study

import (
	"math"
	"sort"
)

// Shape is StudyShape (study_shape.dart): a local geometric fit of a held
// stroke; its points stay ordinary editable ink.
type Shape struct {
	Kind   string
	Points []Pt
}

// Label names the shape.
func (s Shape) Label() string {
	switch s.Kind {
	case "line":
		return "直线"
	case "circle":
		return "圆"
	case "ellipse":
		return "椭圆"
	case "triangle":
		return "三角形"
	case "rectangle":
		return "矩形"
	case "arrow":
		return "箭头"
	case "polygon":
		return "多边形"
	}
	return "四边形"
}

// Adjust turns and scales the shape so that heldAt follows target; a line
// moves its end.
func (s Shape) Adjust(heldAt, target Pt) []Pt {
	if s.Kind == "line" {
		return []Pt{s.Points[0], axisSnap(s.Points[0], target)}
	}
	center := Bounds(s.Points).Center()
	from, to := heldAt.Sub(center), target.Sub(center)
	if from.Dist() < 1 {
		return s.Points
	}
	scale := clamp(to.Dist()/from.Dist(), .2, 5)
	angle := math.Atan2(to.Y, to.X) - math.Atan2(from.Y, from.X)
	out := make([]Pt, len(s.Points))
	for i, p := range s.Points {
		out[i] = center.Add(p.Sub(center).Rotate(angle).Mul(scale))
	}
	return out
}

// RecognizeShape is recognizeStudyShape: a conservative fit of one held
// stroke. Tolerances are in screen distance, so zooming does not turn a
// small dot into a large shape.
func RecognizeShape(input []Pt, zoom float64) *Shape {
	if len(input) < 3 || len(input) > 20000 || !finite(zoom) || zoom <= 0 {
		return nil
	}
	for _, p := range input {
		if !p.Finite() {
			return nil
		}
	}
	samples := resample(input, 128)
	if len(samples) < 3 {
		return nil
	}
	bounds := Bounds(samples)
	diagonal := bounds.Diagonal()
	if diagonal*zoom < 28 {
		return nil
	}
	first, last := samples[0], samples[len(samples)-1]
	length, chord := pathLength(samples), last.Sub(first).Dist()
	if chord*zoom >= 24 && length/chord < 1.12 {
		straight := true
		for _, p := range samples {
			if SegmentDistance(p, first, last) > math.Max(2.5/zoom, chord*.035) {
				straight = false
				break
			}
		}
		if straight {
			return &Shape{"line", []Pt{first, axisSnap(first, last)}}
		}
	}
	if a := arrow(samples, diagonal); a != nil {
		return a
	}
	if chord > math.Max(10/zoom, diagonal*.24) || bounds.ShortestSide()*zoom < 18 || length < diagonal*2 {
		return nil
	}
	ring := resample(append(append([]Pt(nil), samples...), first), 128)
	if p := polygon(ring, diagonal, length); p != nil {
		return p
	}
	return ellipse(ring, zoom)
}

func polygon(ring []Pt, diagonal, length float64) *Shape {
	// Split the closed trace into two open arcs before simplifying corners.
	a, b, distance := 0, 1, 0.0
	for i := 0; i < len(ring)-1; i++ {
		for j := i + 1; j < len(ring)-1; j++ {
			if d := ring[i].Sub(ring[j]).Dist2(); d > distance {
				distance, a, b = d, i, j
			}
		}
	}
	tolerance := diagonal * .04
	c1 := corners(ring[a:b+1], tolerance)
	other := append(append([]Pt(nil), ring[b:len(ring)-1]...), ring[:a+1]...)
	c2 := corners(other, tolerance)
	vertices := append(append([]Pt(nil), c1[:len(c1)-1]...), c2[:len(c2)-1]...)
	for pruned := true; pruned && len(vertices) > 3; {
		pruned = false
		n := len(vertices)
		for i := range vertices {
			if SegmentDistance(vertices[i], vertices[(i+n-1)%n], vertices[(i+1)%n]) < tolerance {
				vertices = append(vertices[:i:i], vertices[i+1:]...)
				pruned = true
				break
			}
		}
	}
	n := len(vertices)
	if n < 3 || n > 6 {
		return nil
	}
	turn, area := 0.0, 0.0
	for i := range vertices {
		p, q, r := vertices[i], vertices[(i+1)%n], vertices[(i+2)%n]
		u, v := q.Sub(p), r.Sub(q)
		cross := u.Cross(v)
		if u.Dist() < diagonal*.13 || math.Abs(cross) < u.Dist()*v.Dist()*.18 || turn*cross < 0 {
			return nil
		}
		turn = cross
		area += p.Cross(q)
	}
	closed := append(append([]Pt(nil), vertices...), vertices[0])
	if math.Abs(area) < diagonal*diagonal*.12 || length/pathLength(closed) > 1.15 {
		return nil
	}
	// Rounded arcs must not become polygons merely because simplification
	// left a few vertices: most samples must lie close to fitted edges.
	errSum := 0.0
	for _, p := range ring {
		best := math.Inf(1)
		for i := range vertices {
			best = math.Min(best, SegmentDistance(p, vertices[i], vertices[(i+1)%n]))
		}
		errSum += best
	}
	edgeTolerance := .013
	if n <= 4 {
		edgeTolerance = .02
	}
	if errSum/float64(len(ring)) > diagonal*edgeTolerance {
		return nil
	}
	if n == 3 {
		return &Shape{"triangle", closed}
	}
	if n > 4 {
		return &Shape{"polygon", closed}
	}
	var edges [4]Pt
	for i := range 4 {
		edges[i] = vertices[(i+1)%4].Sub(vertices[i])
	}
	square := true
	for i := range 4 {
		e, f := edges[i], edges[(i+1)%4]
		if math.Abs(e.Dot(f))/(e.Dist()*f.Dist()) >= .25 {
			square = false
		}
	}
	if square {
		axis := edges[0].Div(edges[0].Dist()).Sub(edges[2].Div(edges[2].Dist()))
		angle := math.Atan2(axis.Y, axis.X)
		turned := make([]Pt, len(vertices))
		for i, p := range vertices {
			turned[i] = p.Rotate(-angle)
		}
		box := Bounds(turned)
		out := []Pt{box.TopLeft(), box.TopRight(), box.BottomRight(), box.BottomLeft(), box.TopLeft()}
		for i, p := range out {
			out[i] = p.Rotate(angle)
		}
		return &Shape{"rectangle", out}
	}
	return &Shape{"quadrilateral", closed}
}

// arrow is a continuous shaft → tip → wing → tip → opposite wing gesture.
// The return to the tip tells an arrow from a zigzag word.
func arrow(samples []Pt, diagonal float64) *Shape {
	v := corners(samples, diagonal*.025)
	if len(v) != 5 {
		return nil
	}
	start, tip := v[0], v[1].Add(v[3]).Div(2)
	shaft := tip.Sub(start)
	length := shaft.Dist()
	if length < diagonal*.7 || v[1].Sub(v[3]).Dist() > length*.08 {
		return nil
	}
	axis := shaft.Div(length)
	normal := Pt{-axis.Y, axis.X}
	a, b := v[2].Sub(tip), v[4].Sub(tip)
	backA, backB := -a.Dot(axis), -b.Dot(axis)
	sideA, sideB := a.Dot(normal), b.Dot(normal)
	ratio := a.Dist() / b.Dist()
	if sideA*sideB >= 0 || backA < length*.1 || backB < length*.1 || backA > length*.45 || backB > length*.45 ||
		math.Abs(sideA) < length*.06 || math.Abs(sideB) < length*.06 || math.Abs(sideA) > length*.4 ||
		math.Abs(sideB) > length*.4 || ratio < .5 || ratio > 2 {
		return nil
	}
	back := (backA + backB) / 2
	side := (math.Abs(sideA) + math.Abs(sideB)) / 2
	sign := 1.0
	if sideA < 0 {
		sign = -1
	}
	return &Shape{"arrow", []Pt{start, tip,
		tip.Sub(axis.Mul(back)).Add(normal.Mul(side * sign)), tip,
		tip.Sub(axis.Mul(back)).Sub(normal.Mul(side * sign))}}
}

func ellipse(ring []Pt, zoom float64) *Shape {
	trace := ring[:len(ring)-1]
	var mean Pt
	for _, p := range trace {
		mean = mean.Add(p)
	}
	mean = mean.Div(float64(len(trace)))
	var xx, yy, xy float64
	for _, p := range trace {
		d := p.Sub(mean)
		xx += d.X * d.X
		yy += d.Y * d.Y
		xy += d.X * d.Y
	}
	angle := .5 * math.Atan2(2*xy, xx-yy)
	projected := make([]Pt, len(trace))
	for i, p := range trace {
		projected[i] = p.Sub(mean).Rotate(-angle)
	}
	box := Bounds(projected)
	center := box.Center()
	rx, ry := box.W()/2, box.H()/2
	if math.Min(rx, ry)*zoom < 9 {
		return nil
	}
	var errSum, turn, travel float64
	var previous, first float64
	started := false
	for _, p := range projected {
		d := p.Sub(center)
		x, y := d.X/rx, d.Y/ry
		deviation := math.Abs(math.Sqrt(x*x+y*y) - 1)
		if deviation > .24 {
			return nil
		}
		errSum += deviation
		theta := math.Atan2(y, x)
		if !started {
			first = theta
		} else {
			delta := angleDelta(theta - previous)
			turn += delta
			travel += math.Abs(delta)
		}
		started = true
		previous = theta
	}
	tail := angleDelta(first - previous)
	turn += tail
	travel += math.Abs(tail)
	if errSum/float64(len(trace)) > .09 || math.Abs(turn) < 5.8 || math.Abs(turn) > 6.7 || travel > 7.4 {
		return nil
	}
	circle := math.Min(rx, ry)/math.Max(rx, ry) > .91
	if circle {
		rx = (rx + ry) / 2
		ry = rx
	}
	p0 := projected[0].Sub(center)
	start := math.Atan2(p0.Y/ry, p0.X/rx)
	sign := 1.0
	if turn < 0 {
		sign = -1
	}
	points := make([]Pt, 0, 97)
	for i := range 96 {
		t := start + sign*2*math.Pi*float64(i)/96
		points = append(points, mean.Add(center.Add(Pt{rx * math.Cos(t), ry * math.Sin(t)}).Rotate(angle)))
	}
	kind := "ellipse"
	if circle {
		kind = "circle"
	}
	return &Shape{kind, append(points, points[0])}
}

func resample(points []Pt, count int) []Pt {
	length := pathLength(points)
	if length < .001 {
		return nil
	}
	result := []Pt{points[0]}
	edge, passed := 1, 0.0
	for i := 1; i < count-1; i++ {
		target := length * float64(i) / float64(count-1)
		for edge < len(points)-1 && passed+points[edge].Sub(points[edge-1]).Dist() < target {
			passed += points[edge].Sub(points[edge-1]).Dist()
			edge++
		}
		d := points[edge].Sub(points[edge-1]).Dist()
		t := 0.0
		if d >= .001 {
			t = clamp((target-passed)/d, 0, 1)
		}
		result = append(result, points[edge-1].Lerp(points[edge], t))
	}
	return append(result, points[len(points)-1])
}

func corners(points []Pt, tolerance float64) []Pt {
	farthest, distance := 0, tolerance
	first, last := points[0], points[len(points)-1]
	for i := 1; i < len(points)-1; i++ {
		if d := SegmentDistance(points[i], first, last); d > distance {
			farthest, distance = i, d
		}
	}
	if farthest == 0 {
		return []Pt{first, last}
	}
	head := corners(points[:farthest+1], tolerance)
	return append(append([]Pt(nil), head[:len(head)-1]...), corners(points[farthest:], tolerance)...)
}

func pathLength(points []Pt) float64 {
	r := 0.0
	for i := 1; i < len(points); i++ {
		r += points[i].Sub(points[i-1]).Dist()
	}
	return r
}

func angleDelta(v float64) float64 { return math.Atan2(math.Sin(v), math.Cos(v)) }

func axisSnap(start, end Pt) Pt {
	d := end.Sub(start)
	if math.Abs(d.Y) < math.Abs(d.X)*.07 {
		return Pt{end.X, start.Y}
	}
	if math.Abs(d.X) < math.Abs(d.Y)*.07 {
		return Pt{start.X, end.Y}
	}
	return end
}

// SelectionHandle is StudySelectionHandle (study_selection_geometry.dart):
// a handle that changes a region's crop, not the objects in it.
type SelectionHandle int

const (
	HandleTopLeft SelectionHandle = iota
	HandleTop
	HandleTopRight
	HandleRight
	HandleBottomRight
	HandleBottom
	HandleBottomLeft
	HandleLeft
)

// HandlePoints is studySelectionHandles: where each handle sits on r.
func HandlePoints(r Rect) [8]Pt {
	return [8]Pt{r.TopLeft(), r.TopCenter(), r.TopRight(), r.CenterRight(), r.BottomRight(), r.BottomCenter(), r.BottomLeft(), r.CenterLeft()}
}

// HandleAt is studySelectionHandleAt: the nearest handle within tolerance,
// else an edge; -1 for none.
func HandleAt(r Rect, p Pt, tolerance float64) SelectionHandle {
	pts := HandlePoints(r)
	order := []int{0, 1, 2, 3, 4, 5, 6, 7}
	sort.SliceStable(order, func(i, j int) bool { return pts[order[i]].Sub(p).Dist() < pts[order[j]].Sub(p).Dist() })
	if pts[order[0]].Sub(p).Dist() <= tolerance {
		return SelectionHandle(order[0])
	}
	if p.X >= r.L && p.X <= r.R {
		if math.Abs(p.Y-r.T) <= tolerance {
			return HandleTop
		}
		if math.Abs(p.Y-r.B) <= tolerance {
			return HandleBottom
		}
	}
	if p.Y >= r.T && p.Y <= r.B {
		if math.Abs(p.X-r.L) <= tolerance {
			return HandleLeft
		}
		if math.Abs(p.X-r.R) <= tolerance {
			return HandleRight
		}
	}
	return -1
}

// ResizeSelection is studyResizeSelection.
func ResizeSelection(r Rect, h SelectionHandle, delta Pt, minimum float64) Rect {
	left := h == HandleLeft || h == HandleTopLeft || h == HandleBottomLeft
	right := h == HandleRight || h == HandleTopRight || h == HandleBottomRight
	top := h == HandleTop || h == HandleTopLeft || h == HandleTopRight
	bottom := h == HandleBottom || h == HandleBottomLeft || h == HandleBottomRight
	out := r
	if left {
		out.L = math.Min(r.R-minimum, r.L+delta.X)
	}
	if top {
		out.T = math.Min(r.B-minimum, r.T+delta.Y)
	}
	if right {
		out.R = math.Max(r.L+minimum, r.R+delta.X)
	}
	if bottom {
		out.B = math.Max(r.T+minimum, r.B+delta.Y)
	}
	return out
}

func freeSpace(viewport Rect, blocked []Rect) []Rect {
	free := []Rect{viewport}
	for _, b := range blocked {
		var next []Rect
		for _, r := range free {
			next = append(next, subtract(r, b)...)
		}
		free = maximal(next)
	}
	return free
}

// SelectionMenuRect is studySelectionMenuRect: where the selection's
// toolbar fits in empty space, never over the selection; nil asks for a
// footer outside the canvas.
func SelectionMenuRect(selection, viewport Rect, obstacles []Rect) *Rect {
	blocked := []Rect{selection.Inflate(12)}
	for _, o := range obstacles {
		blocked = append(blocked, o.Inflate(6))
	}
	var choices []Rect
	for _, r := range freeSpace(viewport, blocked) {
		if r.W() < 144 || r.H() < 48 {
			continue
		}
		width := math.Min(600, r.W())
		x := clamp(selection.Center().X-width/2, r.L, r.R-width)
		y := clamp(selection.T-60, r.T, r.B-48)
		choices = append(choices, RectLTWH(x, y, width, 48))
	}
	score := func(r Rect) float64 { return r.Center().Sub(selection.TopCenter()).Dist() + (600-r.W())*.15 }
	sort.SliceStable(choices, func(i, j int) bool { return score(choices[i]) < score(choices[j]) })
	if len(choices) == 0 {
		return nil
	}
	return &choices[0]
}

// FloatingPosition is studyFloatingPosition: where a control of w×h fits
// in empty space by the selection, off its top-right corner if it can;
// false when nothing fits.
func FloatingPosition(selection, viewport Rect, obstacles []Rect, w, h, gap float64) (Pt, bool) {
	blocked := []Rect{selection.Inflate(gap)}
	for _, o := range obstacles {
		blocked = append(blocked, o.Inflate(6))
	}
	ideal := Pt{selection.R - w, selection.T - gap - h}
	var best Pt
	found, distance := false, math.Inf(1)
	for _, r := range freeSpace(viewport, blocked) {
		if r.W() < w || r.H() < h {
			continue
		}
		p := Pt{clamp(ideal.X, r.L, r.R-w), clamp(ideal.Y, r.T, r.B-h)}
		if d := p.Sub(ideal).Dist(); d < distance {
			distance, best, found = d, p, true
		}
	}
	return best, found
}

// maximal drops free rectangles another one already contains.
func maximal(free []Rect) []Rect {
	var out []Rect
	for i, r := range free {
		inside := false
		for j, o := range free {
			if j != i && o.L <= r.L && o.T <= r.T && o.R >= r.R && o.B >= r.B && (o != r || j < i) {
				inside = true
				break
			}
		}
		if !inside {
			out = append(out, r)
		}
	}
	return out
}

// subtract is the largest free rectangles of r around blocked. They
// overlap, so a control can use the space beside a small dock as well as
// above it.
func subtract(r, blocked Rect) []Rect {
	if !r.Overlaps(blocked) {
		return []Rect{r}
	}
	cut := r.Intersect(blocked)
	var out []Rect
	for _, c := range []Rect{{r.L, r.T, r.R, cut.T}, {r.L, cut.B, r.R, r.B}, {r.L, r.T, cut.L, r.B}, {cut.R, r.T, r.R, r.B}} {
		if !c.Empty() {
			out = append(out, c)
		}
	}
	return out
}
