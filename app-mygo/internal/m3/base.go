// Package m3 is Flutter's Material 3 widgets as the Flutter client shows
// them: each widget takes Flutter's M3 defaults (sizes, paddings, colours,
// state layers, shapes) with the overrides of the app's buildTheme, and
// draws on MyGo's bases, which handle the pointer, the keyboard and
// accessibility. The theme is design.T(), set for each frame.
package m3

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
)

// Short names for the theme being built with.
func cs() *design.Scheme    { return design.S() }
func tt() *design.TextTheme { return design.TT() }

// Density is VisualDensity's vertical and horizontal step: 0 standard, -2
// compact. Each step is 4 DIPs.
func Density() float32 {
	if design.T().Compact {
		return -2
	}
	return 0
}

// Elevation adds Material 3's shadow for a level in DIPs (0, 1, 3, 6, 8, 12).
func Elevation(e ui.Element, level float32) ui.Element {
	if level <= 0 {
		return e
	}
	k := ui.RGBA(0, 0, 0, .15)
	a := ui.RGBA(0, 0, 0, .3)
	switch {
	case level <= 1:
		return e.Shadow(0, 1, 3, 1, k).Shadow(0, 1, 2, 0, a)
	case level <= 3:
		return e.Shadow(0, 2, 6, 2, k).Shadow(0, 1, 2, 0, a)
	case level <= 6:
		return e.Shadow(0, 4, 8, 3, k).Shadow(0, 1, 3, 0, a)
	case level <= 8:
		return e.Shadow(0, 6, 10, 4, k).Shadow(0, 2, 3, 0, a)
	}
	return e.Shadow(0, 8, 12, 6, k).Shadow(0, 4, 4, 0, a)
}

// TintOpacity is ElevationOverlay's surface tint for an elevation.
func TintOpacity(elevation float32) float32 {
	steps := []struct{ e, o float32 }{{0, 0}, {1, .05}, {3, .08}, {6, .11}, {8, .12}, {12, .14}}
	for i := 1; i < len(steps); i++ {
		if elevation <= steps[i].e {
			lo, hi := steps[i-1], steps[i]
			return lo.o + (hi.o-lo.o)*(elevation-lo.e)/(hi.e-lo.e)
		}
	}
	return .14
}

// Tinted is a surface colour at an elevation with the scheme's tint.
func Tinted(surface ui.Color, elevation float32) ui.Color {
	return design.AlphaBlend(design.WithOpacity(cs().SurfaceTint, TintOpacity(elevation)), surface)
}

// Disabled colours: containers onSurface at 12%, content at 38%.
func DisabledContainer() ui.Color { return design.WithOpacity(cs().OnSurface, .12) }
func DisabledContent() ui.Color   { return design.WithOpacity(cs().OnSurface, .38) }

// splash is a press's sparkle; it starts with the frame that first paints
// it and is done once it has run its course.
type splash struct {
	x, y  float32
	start time.Time
	done  bool
}

type ink struct {
	pressed  bool
	splashes []splash
}

// sparkle is InkSparkle's duration; its wave grows to sparkleReach times
// half the box's diagonal.
const (
	sparkle      = 617 * time.Millisecond
	sparkleReach = 2.3
)

// sparkleStops are InkSparkle's soft circle out to 1.5 times its radius:
// solid to half of it, then a smoothstep to nothing.
var sparkleStops = func() []float32 {
	a := make([]float32, 9)
	for i := range a {
		x := .5 + float32(i)/8
		s := x - .5
		a[i] = 1 - s*s*(3-2*s)
	}
	return a
}()

// Ink gives e Material's state layer and press sparkle in overlay, the
// content colour: 8% while hovered, 10% while focused or pressed. radius
// is the corner radius it clips to. As InkWell's, the ink paints on the
// surface under e's children.
func Ink(e ui.Element, overlay ui.Color, radius float32) ui.Element {
	return InkLayer(e, overlay, radius, .08, .1, .1)
}

// InkLayer is Ink with the state layer's opacities given.
func InkLayer(e ui.Element, overlay ui.Color, radius float32, hover, focus, press float32) ui.Element {
	o := func(a float32) ui.Color { return design.WithOpacity(overlay, a) }
	return InkColors(e, radius, o(hover), o(focus), o(press), o(press))
}

// InkWell is a plain InkWell's look: the theme's hoverColor (4% black or
// white), focusColor (12%), the grey highlight while pressed and the grey
// sparkle (ThemeData's highlightColor and splashColor).
func InkWell(e ui.Element, radius float32) ui.Element {
	if design.T().Dark {
		return InkColors(e, radius, ui.RGBA(255, 255, 255, .04), ui.RGBA(255, 255, 255, .12),
			ui.RGBA(0xCC, 0xCC, 0xCC, 0x40/255.0), ui.RGBA(0xCC, 0xCC, 0xCC, 0x40/255.0))
	}
	return InkColors(e, radius, ui.RGBA(0, 0, 0, .04), ui.RGBA(0, 0, 0, .12),
		ui.RGBA(0xBC, 0xBC, 0xBC, 0x66/255.0), ui.RGBA(0xC8, 0xC8, 0xC8, 0x66/255.0))
}

// InkColors paints e's state layers in the colours of its states and the
// press sparkle in splash, as InkWell's highlights and InkSparkle do: the
// hover and focus layers fade in 50ms, the pressed one in 200ms, over the
// others, and each press starts a sparkle that runs its course whenever
// the press ends.
func InkColors(e ui.Element, radius float32, hover, focus, press, splashC ui.Color) ui.Element {
	st := ui.Local(e, "m3.ink", func() ink { return ink{} })
	enabled := !e.IsDisabled()
	pressed := e.Pressed() && enabled
	if pressed && !st.pressed {
		x, y, ok := e.PointerPosition()
		if !ok {
			b := e.Bounds()
			x, y = b.W/2, b.H/2
		}
		st.splashes = append(st.splashes, splash{x: x, y: y})
	}
	st.pressed = pressed
	on := func(b bool) float32 {
		if b {
			return 1
		}
		return 0
	}
	fade := design.T().Motion(50*time.Millisecond, ui.Linear)
	hoverA := e.AnimateTo("m3.ink.hover", on(enabled && e.Hovered()), fade)
	focusA := e.AnimateTo("m3.ink.focus", on(enabled && e.FocusVisible()), fade)
	pressA := e.AnimateTo("m3.ink.press", on(pressed), design.T().Motion(200*time.Millisecond, ui.Linear))
	live := st.splashes[:0]
	for _, s := range st.splashes {
		if !s.done {
			live = append(live, s)
		}
	}
	st.splashes = live
	if hoverA == 0 && focusA == 0 && pressA == 0 && len(live) == 0 {
		return e
	}
	return e.Draw(func(p *ui.Painter, r ui.Rect) {
		for _, l := range []struct {
			c ui.Color
			a float32
		}{{focus, focusA}, {hover, hoverA}, {press, pressA}} {
			if l.a > 0 && l.c.A > 0 {
				c := l.c
				c.A = uint8(float32(c.A)*l.a + .5)
				p.Fill(r, c, radius)
			}
		}
		t := p.Now()
		reach := sparkleReach * float32(math.Hypot(float64(r.W), float64(r.H))) / 2
		moving := false
		for i := range st.splashes {
			s := &st.splashes[i]
			if s.start.IsZero() {
				s.start = t
			}
			k := float32(t.Sub(s.start)) / float32(sparkle)
			if k >= 1 {
				s.done = true
				continue
			}
			moving = true
			if r.W <= 0 || r.H <= 0 {
				continue
			}
			grow := design.FastOutSlowIn(min(k/.75, 1))
			a := float32(1)
			switch {
			case k < .13:
				a = k / .13
			case k > .4:
				a = 1 - (k-.4)/.6
			}
			rad := reach * grow
			if a <= 0 || rad <= 0 {
				continue
			}
			// The wave drifts from the press to the middle as it grows.
			drift := min(grow*2, 1)
			cx := (s.x + (r.W/2-s.x)*drift) / r.W
			cy := (s.y + (r.H/2-s.y)*drift) / r.H
			if cx == 0 && cy == 0 {
				cx = 1e-6
			}
			stops := make([]ui.GradientStop, len(sparkleStops))
			for i, f := range sparkleStops {
				c := splashC
				c.A = uint8(float32(c.A)*a*f + .5)
				stops[i] = ui.GradientStop{Color: c, At: (.5 + float32(i)/8) / 1.5}
			}
			p.FillRadialGradient(r, ui.RadialGradient{Center: ui.Point{X: cx, Y: cy}, Radius: 1.5 * rad, Stops: stops}, radius)
		}
		if moving {
			p.AnimationFrame()
		}
	})
}
