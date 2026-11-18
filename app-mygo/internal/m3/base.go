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

type splash struct {
	x, y  float32
	start time.Time
	up    time.Time
}

type ink struct {
	pressed  bool
	splashes []splash
}

// sparkle is InkSparkle's duration.
const sparkle = 617 * time.Millisecond

// Ink gives e Material's state layer and press sparkle in overlay, the
// content colour: 8% while hovered, 10% while focused or pressed. radius
// is the corner radius it clips to.
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

// InkColors paints e's state layer in the colour of its state and the
// press sparkle in splash.
func InkColors(e ui.Element, radius float32, hover, focus, press, splashC ui.Color) ui.Element {
	st := ui.Local(e, "m3.ink", func() ink { return ink{} })
	pressed := e.Pressed() && !e.IsDisabled()
	now := time.Now()
	if pressed && !st.pressed {
		x, y, ok := e.PointerPosition()
		if !ok {
			b := e.Bounds()
			x, y = b.W/2, b.H/2
		}
		st.splashes = append(st.splashes, splash{x: x, y: y, start: now})
	}
	if !pressed && st.pressed {
		for i := range st.splashes {
			if st.splashes[i].up.IsZero() {
				st.splashes[i].up = now
			}
		}
	}
	st.pressed = pressed
	target := ui.Transparent
	switch {
	case e.IsDisabled():
	case pressed:
		target = press
	case e.FocusVisible():
		target = focus
	case e.Hovered():
		target = hover
	}
	layer := e.AnimateColor("m3.ink.layer", target, design.T().Motion(200*time.Millisecond, ui.Linear))
	live := st.splashes[:0]
	for _, s := range st.splashes {
		end := s.start.Add(sparkle)
		if !s.up.IsZero() && s.up.After(end) {
			end = s.up
		}
		if now.Before(end.Add(200*time.Millisecond)) || (pressed && s.up.IsZero()) {
			live = append(live, s)
		}
	}
	st.splashes = live
	if layer.A == 0 && len(live) == 0 {
		return e
	}
	splashes := append([]splash(nil), live...)
	return e.DrawOver(func(p *ui.Painter, r ui.Rect) {
		p.Clip(r, radius, func() {
			if layer.A > 0 {
				p.Fill(r, layer, radius)
			}
			t := p.Now()
			for _, s := range splashes {
				el := float32(t.Sub(s.start)) / float32(sparkle)
				dx := max(s.x, r.W-s.x)
				dy := max(s.y, r.H-s.y)
				target := float32(math.Hypot(float64(dx), float64(dy))) * 1.15
				prog := min(el, 1)
				rad := target * (1 - (1-prog)*(1-prog)*(1-prog))
				a := float32(1)
				if el < .1 {
					a = el / .1
				}
				if s.up.IsZero() {
					// Held: the sparkle stays at its size until release.
				} else {
					if fade := el - .6; fade > 0 {
						a *= max(0, 1-fade/.4)
					}
					if out := float32(t.Sub(s.up)) / float32(200*time.Millisecond); out > 0 {
						a *= max(0, 1-out)
					}
				}
				if a > 0 {
					c := splashC
					c.A = uint8(float32(c.A) * a)
					p.FillPath((&ui.Path{}).Circle(r.X+s.x, r.Y+s.y, rad), c)
				}
			}
			if len(splashes) > 0 {
				p.AnimationFrame()
			}
		})
	})
}
