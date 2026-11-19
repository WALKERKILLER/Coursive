package app

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
)

// reading_glow.dart: the light that marks what the assistant is reading, a
// band of colour travelling around the edge of the material after the edge
// light Apple Intelligence uses, and a shimmer through the words naming it.

// readingHues are the light's hues, cool to warm and back to cool.
var readingHues = []ui.Color{
	design.ARGB(0xff5f8bff), design.ARGB(0xffa66cff), design.ARGB(0xffff6fae),
	design.ARGB(0xffff9757), design.ARGB(0xffffcc66), design.ARGB(0xff45d0c0),
}

// readingPhase is where a light with period turns now, from 0 to 1; a
// still light stays at 0.
func readingPhase(period time.Duration) float64 {
	if design.T().ReduceMotion {
		return 0
	}
	return float64(time.Now().UnixNano()%int64(period)) / float64(period)
}

// readingConic sweeps the hues around twice, turned by t: t from 0 to 1
// turns half a circle and loops without a seam.
func readingConic(t, opacity float64) ui.ConicGradient {
	hues := append(append(append([]ui.Color{}, readingHues...), readingHues...), readingHues[0])
	stops := make([]ui.GradientStop, len(hues))
	for i, h := range hues {
		stops[i] = ui.GradientStop{Color: h.Alpha(f32(opacity)), At: float32(i) / float32(len(hues)-1)}
	}
	// SweepGradient starts at the right; ConicGradient's angle is from
	// upwards.
	return ui.ConicGradient{Stops: stops, Angle: f32(90 + t*180)}
}

// paintReadingLight paints the light along r: a wide soft halo, a glow and
// a crisp edge, all turning with t.
func paintReadingLight(p *ui.Painter, r ui.Rect, radius float32, t float64, halo, strength float64) {
	path := design.RoundedRectPath(r, min(radius, min(r.W, r.H)/2))
	// The halo is layered strokes fading outwards: a painter has no blur.
	for i, k := range []float64{1, .66, .33} {
		p.StrokePathConicGradient(path, f32(halo*k), readingConic(t, .38*strength*float64(i+1)/3))
	}
	p.StrokePathConicGradient(path, f32(halo*.4), readingConic(t, .7*strength))
	p.StrokePathConicGradient(path, 1.6, readingConic(t, strength))
}

// readingGlow wraps child in the reading light while active.
func readingGlow(c *ui.Context, active bool, radius float32, child func()) {
	readingGlowHalo(c, active, radius, 6, child)
}

func readingGlowHalo(c *ui.Context, active bool, radius float32, halo float64, child func()) {
	e := ui.Box(c)
	if active {
		c.AnimationFrame()
		e = e.DrawOver(func(p *ui.Painter, r ui.Rect) {
			paintReadingLight(p, r, radius, readingPhase(1800*time.Millisecond), halo, 1)
		})
	}
	e.Children(child)
}

// readingOrb is a small turning orb: the assistant is at work.
func readingOrb(c *ui.Context, size float32) ui.Element {
	c.AnimationFrame()
	return ui.Box(c).Size(size, size).Draw(func(p *ui.Painter, r ui.Rect) {
		t := readingPhase(1600 * time.Millisecond)
		cx, cy, rad := r.X+r.W/2, r.Y+r.H/2, min(r.W, r.H)/2
		halo := (&ui.Path{}).Circle(cx, cy, rad*1.4)
		p.FillPathConicGradient(halo, readingConic(t, .25))
		p.FillPathConicGradient((&ui.Path{}).Circle(cx, cy, rad), readingConic(t, 1))
	})
}

// readingShimmerColor is the text colour a shimmer passing through it
// gives now: base between passes, the hues as the light goes by.
func readingShimmerColor(active bool, base ui.Color) ui.Color {
	if !active || design.T().ReduceMotion {
		return base
	}
	t := readingPhase(2200 * time.Millisecond)
	// The light crosses in the middle third of the period.
	k := math.Max(0, 1-math.Abs(t-.5)*6)
	if k == 0 {
		return base
	}
	h := readingHues[int(t*3)%3]
	return lerpColor(base, h, k)
}

func lerpColor(a, b ui.Color, t float64) ui.Color {
	mix := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t) }
	return ui.Color{R: mix(a.R, b.R), G: mix(a.G, b.G), B: mix(a.B, b.B), A: mix(a.A, b.A)}
}
