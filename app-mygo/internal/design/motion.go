package design

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// Motion is the M3 motion tokens of tokens.dart.
var (
	Emphasized           = ui.CubicBezier(0.2, 0, 0, 1)
	EmphasizedDecelerate = ui.CubicBezier(0.05, 0.7, 0.1, 1)
	EmphasizedAccelerate = ui.CubicBezier(0.3, 0, 0.8, 0.15)
	Standard             = ui.CubicBezier(0.2, 0, 0, 1)
	StandardDecelerate   = ui.CubicBezier(0, 0, 0, 1)
	StandardAccelerate   = ui.CubicBezier(0.3, 0, 1, 1)
	// FastOutSlowIn is Flutter's Curves.fastOutSlowIn, which its widgets
	// animate along unless given another curve.
	FastOutSlowIn = ui.CubicBezier(0.4, 0, 0.2, 1)
	// EaseInOut is Curves.easeInOut, EaseOutCubic Curves.easeOutCubic.
	EaseInOut    = ui.CubicBezier(0.42, 0, 0.58, 1)
	EaseOutCubic = ui.CubicBezier(0.215, 0.61, 0.355, 1)
	EaseOut      = ui.CubicBezier(0, 0, 0.58, 1)
	// EaseIn is Curves.easeIn, EaseOutBack Curves.easeOutBack, which
	// overshoots its end a little before it settles.
	EaseIn      = ui.CubicBezier(0.42, 0, 1, 1)
	EaseOutBack = ui.CubicBezier(0.175, 0.885, 0.32, 1.275)
	// Ease is Curves.ease, which TabController.animateTo moves along.
	Ease = ui.CubicBezier(0.25, 0.1, 0.25, 1)
	// LegacyDecelerate is Easing.legacyDecelerate, the modal bottom sheet's.
	LegacyDecelerate = ui.CubicBezier(0, 0, 0.2, 1)
)

const (
	Short1  = 50 * time.Millisecond
	Short2  = 100 * time.Millisecond
	Short3  = 150 * time.Millisecond
	Short4  = 200 * time.Millisecond
	Medium1 = 250 * time.Millisecond
	Medium2 = 300 * time.Millisecond
	Medium3 = 350 * time.Millisecond
	Medium4 = 400 * time.Millisecond
	Long2   = 500 * time.Millisecond
	Long4   = 600 * time.Millisecond
)

// SpringFast and SpringDefault are the Expressive springs, as Flutter's
// SpringDescription has them.
var (
	SpringFast    = ui.Animation{Spring: ui.SpringOptions{Mass: 1, Stiffness: 1400, Damping: 67}}
	SpringDefault = ui.Animation{Spring: ui.SpringOptions{Mass: 1, Stiffness: 700, Damping: 48}}
)

// Curve is an animation along a curve for a duration.
func Curve(d time.Duration, ease ui.Easing) ui.Animation {
	return ui.Animation{Duration: d, Ease: ease}
}

// threePointCubic is Flutter's ThreePointCubic: two cubic Béziers joined
// at a midpoint.
func threePointCubic(a1x, a1y, b1x, b1y, mx, my, a2x, a2y, b2x, b2y float32) ui.Easing {
	first := ui.CubicBezier(a1x/mx, a1y/my, b1x/mx, b1y/my)
	sx, sy := 1-mx, 1-my
	second := ui.CubicBezier((a2x-mx)/sx, (a2y-my)/sy, (b2x-mx)/sx, (b2y-my)/sy)
	return func(t float32) float32 {
		if t <= 0 {
			return 0
		}
		if t >= 1 {
			return 1
		}
		if t < mx {
			return first(t/mx) * my
		}
		return second((t-mx)/sx)*sy + my
	}
}

// EaseInOutCubicEmphasized is Curves.easeInOutCubicEmphasized, the curve of
// Material's page transitions.
var EaseInOutCubicEmphasized = threePointCubic(0.05, 0, 0.133333, 0.06, 0.166666, 0.4, 0.208333, 0.82, 0.25, 1)

// Flipped is Curve.flipped: the curve run backwards in time.
func Flipped(e ui.Easing) ui.Easing {
	return func(t float32) float32 { return 1 - e(1-t) }
}

// Switcher is AnimatedSwitcher for an element keyed by what it shows: as
// the key changes, the new element comes from from along in while the old
// one goes back there at the same time, along out run in reverse as
// Flutter's reverseCurve is. A nil curve is linear, as AnimatedSwitcher's
// switchInCurve and switchOutCurve are by default. Only the switch animates: the element
// does not glide as the layout moves or resizes it, which Flutter never
// does either.
func Switcher(d time.Duration, in, out ui.Easing, from ui.Motion) ui.ElementTransition {
	if in == nil {
		in = ui.Linear
	}
	if out == nil {
		out = ui.Linear
	}
	a := T().Motion(d, in)
	if a.Duration == time.Nanosecond {
		a.Duration = -1
	}
	enter, exit := from, from
	return ui.ElementTransition{Duration: a.Duration, Ease: in, ExitEase: Flipped(out), Enter: &enter, Exit: &exit, Shadows: true}
}
