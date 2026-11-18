package m3

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
)

// Progress is a progress indicator's value; Indeterminate when nil.
type Progress struct {
	Value *float64
	Color ui.Color
	Track ui.Color
	// NoTrack is backgroundColor: transparent.
	NoTrack bool
	// Height replaces the linear indicator's 4 DIPs (minHeight).
	Height float32
	// Size replaces the circular indicator's 48 DIPs (with padding).
	Size   float32
	Stroke float32
	Radius float32
}

func pv(v float64) *float64 { return &v }

// Of is a determinate value.
func Of(v float64) *float64 { return pv(v) }

// Linear is LinearProgressIndicator (year2023: false): a 4 DIP bar with
// rounded ends, a 4 DIP gap before the secondaryContainer track and a stop
// dot at its end.
func Linear(c *ui.Context, p Progress) ui.Element {
	s := cs()
	col, track := s.Primary, s.SecondaryContainer
	if p.Color.A != 0 {
		col = p.Color
	}
	if p.Track.A != 0 {
		track = p.Track
	}
	if p.NoTrack {
		track = ui.Transparent
	}
	h := p.Height
	if h == 0 {
		h = 4
	}
	rad := p.Radius
	if rad == 0 {
		rad = h / 2
	}
	e := ui.Box(c).Height(h).FillWidth().Shrink(0).Role(ui.RoleProgress)
	var value float32 = -1
	if p.Value != nil {
		value = float32(min(max(*p.Value, 0), 1))
		value = e.AnimateTo("m3.linear", value, design.T().Motion(250*time.Millisecond, design.EaseInOut))
		e = e.Range(0, 1, float64(value))
	}
	phase := float32(0)
	if value < 0 {
		phase = e.Loop("m3.linear.loop", 1800*time.Millisecond, ui.Linear)
	}
	const gap = 4
	return e.Draw(func(pt *ui.Painter, r ui.Rect) {
		seg := func(a, b float32, cl ui.Color) {
			if b-a <= 0 || cl.A == 0 {
				return
			}
			pt.Fill(ui.Rect{X: r.X + a*r.W, Y: r.Y, W: (b - a) * r.W, H: r.H}, cl, rad)
		}
		g := gap / max(r.W, 1)
		if value >= 0 {
			seg(0, value, col)
			if value+g < 1 {
				seg(value+g, 1, track)
				// The stop indicator: a dot of the bar's colour at the end.
				if track.A != 0 {
					d := min(r.H, 4)
					pt.FillPath((&ui.Path{}).Circle(r.X+r.W-r.H/2, r.Y+r.H/2, d/2), col)
				}
			}
			return
		}
		// Two bars running across, as Flutter's indeterminate curves move
		// them (line 1 and line 2 of LinearProgressIndicator).
		t := phase
		h1e := curve(t, 0, 750.0/1800, 0.2, 0, 0.8, 1)
		h1s := curve(t, 333.0/1800, 1083.0/1800, 0.4, 0, 1, 1)
		h2e := curve(t, 1000.0/1800, 1567.0/1800, 0, 0, 0.65, 1)
		h2s := curve(t, 1267.0/1800, 1, 0.1, 0, 0.45, 1)
		bars := [][2]float32{{h1s, h1e}, {h2s, h2e}}
		// The track fills what the bars leave, with the gap.
		prev := float32(0)
		segs := [][2]float32{}
		for _, b := range bars {
			lo, hi := min(b[0], b[1]), max(b[0], b[1])
			if hi > lo {
				segs = append(segs, [2]float32{lo, hi})
			}
		}
		if len(segs) == 2 && segs[1][0] < segs[0][0] {
			segs[0], segs[1] = segs[1], segs[0]
		}
		for _, b := range segs {
			seg(prev, b[0]-g, track)
			seg(b[0], b[1], col)
			prev = b[1] + g
		}
		seg(prev, 1, track)
		pt.AnimationFrame()
	})
}

// curve maps a loop's time t to a bar end: still before begin, moving
// along a cubic Bézier until end, then still.
func curve(t, begin, end, x1, y1, x2, y2 float32) float32 {
	if t <= begin {
		return 0
	}
	if t >= end {
		return 1
	}
	return ui.CubicBezier(x1, y1, x2, y2)((t - begin) / (end - begin))
}

// Circular is CircularProgressIndicator (year2023: false): a 4 DIP arc
// with rounded caps and a gap before its secondaryContainer track.
func Circular(c *ui.Context, p Progress) ui.Element {
	s := cs()
	col, track := s.Primary, s.SecondaryContainer
	if p.Color.A != 0 {
		col = p.Color
	}
	if p.Track.A != 0 {
		track = p.Track
	}
	if p.NoTrack {
		track = ui.Transparent
	}
	size := p.Size
	if size == 0 {
		size = 48
	}
	stroke := p.Stroke
	if stroke == 0 {
		stroke = 4
	}
	e := ui.Box(c).Size(size, size).Shrink(0).Role(ui.RoleProgress)
	var value float32 = -1
	if p.Value != nil {
		value = float32(min(max(*p.Value, 0), 1))
		value = e.AnimateTo("m3.circular", value, design.T().Motion(250*time.Millisecond, design.EaseInOut))
		e = e.Range(0, 1, float64(value))
	}
	phase := float32(0)
	if value < 0 {
		phase = e.Loop("m3.circular.loop", 1333*4*time.Millisecond, ui.Linear)
	}
	return e.Draw(func(pt *ui.Painter, r ui.Rect) {
		// The 2024 indicator keeps 4 DIPs of padding inside its box.
		pad := float32(4)
		if size < 36 {
			pad = size / 12
		}
		rad := (min(r.W, r.H) - 2*pad - stroke) / 2
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		arc := func(start, sweep float64, cl ui.Color) {
			if sweep <= 0.001 || cl.A == 0 {
				return
			}
			pt.StrokePath(design.ArcPath(float64(cx), float64(cy), float64(rad), start, sweep).Cap(ui.RoundCap), stroke, cl)
		}
		gapAngle := float64((4 + stroke) / rad)
		top := -math.Pi / 2
		if value >= 0 {
			sweep := float64(value) * 2 * math.Pi
			arc(top, sweep, col)
			if rest := 2*math.Pi - sweep - 2*gapAngle; rest > 0 && value > 0 {
				arc(top+sweep+gapAngle, rest, track)
			} else if value == 0 {
				arc(top, 2*math.Pi, track)
			}
			return
		}
		// Material's indeterminate spinner: the head and tail chase each
		// other while the whole turns, four cycles per loop.
		t := float64(phase) * 4
		cycle := math.Floor(t)
		f := t - cycle
		ease := ui.CubicBezier(0.4, 0, 0.2, 1)
		head := float64(ease(float32(min(f*2, 1))))
		tail := float64(ease(float32(max(f*2-1, 0))))
		rot := float64(phase)*2*math.Pi*1.5 + cycle*(3*math.Pi/2)
		start := top + rot + tail*(3*math.Pi/2)
		sweep := 0.05 + (head-tail)*(3*math.Pi/2)
		arc(start, sweep, col)
		if track.A != 0 {
			arc(start+sweep+gapAngle, 2*math.Pi-sweep-2*gapAngle, track)
		}
		pt.AnimationFrame()
	})
}

// Spinner is the small inline CircularProgressIndicator(strokeWidth: w)
// that buttons and tiles hold, in a box of size.
func Spinner(c *ui.Context, size, stroke float32, color ui.Color) ui.Element {
	return Circular(c, Progress{Size: size, Stroke: stroke, Color: color, NoTrack: true})
}
