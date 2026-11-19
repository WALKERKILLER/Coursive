package m3

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
)

// switchTransitional is the thumb's size half-way along the track, as
// _SwitchConfigM3 has it: the thumb is a stadium, stretched on its way.
var switchTransitional = [2]float32{34, 22}

// Switch is Material 3's switch with the app's thumb icon: a check on the
// thumb while on. It returns the base, whose Changed reports a toggle.
// The thumb moves as Flutter's does: over 300ms along easeOutBack, past
// its end and back, shrinking to the transitional size on the way and
// growing to 28 DIPs while pressed.
func Switch(c *ui.Context, on *bool, disabled bool) ui.Element {
	s := cs()
	e := ui.SwitchBase(c, on).Disabled(disabled).DimDisabled(false).FocusRing(false).
		Size(52, 32).Shrink(0)
	if !disabled {
		e = e.Cursor(ui.CursorPointer)
	}
	sel := *on
	ts := toggleReactions(e, disabled, switchPressed(c, e))
	lit := ts.hovered || ts.focused
	thumb := func(selected, pressed bool) ui.Color {
		switch {
		case disabled && selected:
			return s.Surface
		case disabled:
			return design.WithOpacity(s.OnSurface, .38)
		case selected && (pressed || lit):
			return s.PrimaryContainer
		case selected:
			return s.OnPrimary
		case pressed || lit:
			return s.OnSurfaceVariant
		}
		return s.Outline
	}
	activeTrack, inactiveTrack := s.Primary, s.SurfaceContainerHighest
	activeOutline, inactiveOutline := ui.Transparent, s.Outline
	activeIcon, inactiveIcon := s.OnPrimaryContainer, s.SurfaceContainerHighest
	if disabled {
		activeTrack = design.WithOpacity(s.OnSurface, .12)
		inactiveTrack = design.WithOpacity(s.SurfaceContainerHighest, .12)
		inactiveOutline = design.WithOpacity(s.OnSurface, .12)
		activeIcon = design.WithOpacity(s.OnSurface, .38)
		inactiveIcon = design.WithOpacity(s.SurfaceContainerHighest, .38)
	}
	overlay := func(selected bool, a float32) ui.Color {
		if selected {
			return design.WithOpacity(s.Primary, a)
		}
		return design.WithOpacity(s.OnSurface, a)
	}
	rc := reactionColors{inactive: overlay(false, .1), active: overlay(true, .1), hover: overlay(sel, .08), focus: overlay(sel, .1)}

	pos := controller(e, "m3.switch", sel, 300*time.Millisecond)
	position := pos.curved(design.EaseOutBack, design.Flipped(design.EaseOutBack))
	colorV := pos.curved(design.EaseOut, design.EaseIn)
	reaction := ts.press.curved(design.FastOutSlowIn, nil)
	// The thumb's radii, off and on, grow with the press while the thumb
	// rests at either end, and keep their size as it sets off.
	pr := ui.Local(e, "m3.switch.radii", func() [2]float32 { return [2]float32{8, 12} })
	if ts.press.status == ctlCompleted {
		*pr = [2]float32{lerp(8, 14, reaction), lerp(12, 14, reaction)}
	}
	if pos.v == 0 {
		*pr = [2]float32{lerp(8, 14, reaction), 12}
	}
	if pos.v == 1 {
		*pr = [2]float32{8, lerp(12, 14, reaction)}
	}
	off, onSize := [2]float32{pr[0] * 2, pr[0] * 2}, [2]float32{pr[1] * 2, pr[1] * 2}
	var size [2]float32
	switch {
	case ts.press.status == ctlCompleted:
		size = [2]float32{28, 28}
	case pos.status == ctlDismissed || pos.status == ctlForward:
		size = switchThumbForward(pos.v, off, onSize)
	default:
		size = switchThumbReverse(pos.v, off, onSize)
	}
	var tc ui.Color
	switch {
	case ts.press.status != ctlDismissed:
		tc = design.Lerp(thumb(false, true), thumb(true, true), colorV)
	case pos.status == ctlForward:
		tc = design.Lerp(thumb(false, true), thumb(true, false), colorV)
	case pos.status == ctlReverse:
		tc = design.Lerp(thumb(false, false), thumb(true, true), colorV)
	default:
		tc = design.Lerp(thumb(false, false), thumb(true, false), colorV)
	}
	tc = design.AlphaBlend(tc, s.Surface)
	trackC := design.Lerp(inactiveTrack, activeTrack, colorV)
	outlineC := design.Lerp(inactiveOutline, activeOutline, colorV)
	iconC := design.Lerp(inactiveIcon, activeIcon, colorV)
	return e.Draw(func(p *ui.Painter, r ui.Rect) {
		p.Fill(r, trackC, 16)
		if outlineC.A != 0 {
			p.Stroke(r, outlineC, 16, 2)
		}
		// The thumb's centre runs from 16 to 36 DIPs.
		cx := r.X + 16 + 20*position
		cy := r.Y + 16
		ts.paint(p, cx-size[0]/2+size[1]/2, cy, rc, position)
		p.Fill(ui.Rect{X: cx - size[0]/2, Y: cy - size[1]/2, W: size[0], H: size[1]}, tc, size[1]/2)
		if position >= .5 {
			IconPaint(p, icons.CheckRounded, cx-8, cy-8, 16, iconC)
		}
	})
}

// switchPressed is whether the switch shows its press: as Flutter's tap
// waits for its horizontal drag in the gesture arena, from kPressTimeout
// (100ms) after the pointer went down.
func switchPressed(c *ui.Context, e ui.Element) bool {
	since := ui.Local(e, "m3.switch.down", func() time.Time { return time.Time{} })
	if !e.Pressed() {
		*since = time.Time{}
		return false
	}
	if since.IsZero() {
		*since = c.Now()
	}
	if wait := 100*time.Millisecond - c.Now().Sub(*since); wait > 0 {
		c.After(wait)
		return false
	}
	return true
}

func lerp(a, b, t float32) float32 { return a + (b-a)*t }

func lerpSize(a, b [2]float32, t float32) [2]float32 {
	return [2]float32{lerp(a[0], b[0], t), lerp(a[1], b[1], t)}
}

var (
	switchGrow   = ui.CubicBezier(0.31, 0, 0.56, 1)
	switchSettle = ui.CubicBezier(0.2, 0, 0, 1)
)

// switchThumbForward is the thumb's size as it turns on, at the
// controller's value u: to the transitional size in the first 11%, to its
// size on by 83%.
func switchThumbForward(u float32, off, on [2]float32) [2]float32 {
	switch {
	case u < .11:
		return lerpSize(off, switchTransitional, switchGrow(u/.11))
	case u < .83:
		return lerpSize(switchTransitional, on, switchSettle((u-.11)/.72))
	}
	return on
}

// switchThumbReverse is its size as it turns off, the same way back.
func switchThumbReverse(u float32, off, on [2]float32) [2]float32 {
	switch {
	case u < .17:
		return off
	case u < .89:
		return lerpSize(off, switchTransitional, design.Flipped(switchSettle)((u-.17)/.72))
	}
	return lerpSize(switchTransitional, on, design.Flipped(switchGrow)(min((u-.89)/.11, 1)))
}

// Checkbox is Material 3's check box: an 18 DIP box in a 40 DIP target. It
// checks as Flutter's does over 200ms: the box fills in its first quarter
// while it shrinks by its border's width and back, and the check draws
// its two strokes in the second half.
func Checkbox(c *ui.Context, checked *bool, disabled bool) ui.Element {
	s := cs()
	e := ui.CheckboxBase(c, checked).Disabled(disabled).DimDisabled(false).FocusRing(false).
		Size(40, 40).Center().Shrink(0)
	if !disabled {
		e = e.Cursor(ui.CursorPointer)
	}
	on := *checked
	ts := toggleReactions(e, disabled, e.Pressed())
	fill, check := s.Primary, s.OnPrimary
	side, sideW, activeW := s.OnSurfaceVariant, float32(2), float32(0)
	if ts.hovered || ts.focused {
		side = s.OnSurface
	}
	if disabled {
		fill, check, side, activeW = DisabledContent(), s.Surface, DisabledContent(), 2
	}
	overlay := func(selected bool, a float32) ui.Color {
		if selected {
			return design.WithOpacity(s.Primary, a)
		}
		return design.WithOpacity(s.OnSurface, a)
	}
	// A press shows what the click makes of the box.
	rc := pressedReaction(reactionColors{inactive: overlay(true, .1), active: overlay(false, .1), hover: overlay(on, .08), focus: overlay(on, .1)}, on, ts)
	pos := controller(e, "m3.check", on, 200*time.Millisecond)
	t := pos.curved(design.EaseIn, design.EaseOut)
	return e.Draw(func(p *ui.Painter, r ui.Rect) {
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		ts.paint(p, cx, cy, rc, t)
		ox, oy := cx-9, cy-9
		inset := 1 - abs32(t-.5)*2
		box := ui.Rect{X: ox + inset, Y: oy + inset, W: 18 - inset*2, H: 18 - inset*2}
		fc := fill
		if t < .25 {
			fc = design.Lerp(ui.Transparent, fill, t*4)
		}
		if fc.A > 0 {
			p.Fill(box, fc, 2)
		}
		if t <= .5 {
			// The border goes to the checked one, of no width.
			w := lerp(sideW, activeW, t)
			bc := design.Lerp(side, ui.Transparent, t)
			if w > 0 && bc.A > 0 {
				p.Stroke(box, bc, 2, w)
			}
			return
		}
		k := (t - .5) * 2
		sx, sy := ox+18*.15, oy+18*.45
		mx, my := ox+18*.4, oy+18*.7
		ex, ey := ox+18*.85, oy+18*.25
		path := (&ui.Path{}).MoveTo(sx, sy)
		if k < .5 {
			path.LineTo(lerp(sx, mx, k*2), lerp(sy, my, k*2))
		} else {
			path.LineTo(mx, my).LineTo(lerp(mx, ex, (k-.5)*2), lerp(my, ey, (k-.5)*2))
		}
		p.StrokePath(path.Cap(ui.ButtCap).Join(ui.MiterJoin), 2, check)
	})
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// Radio is Material 3's radio button: a ring of radius 8 in a 40 DIP
// target, whose dot grows to 4.5 DIPs over 200ms as Flutter's does.
func Radio[T comparable](c *ui.Context, selected *T, value T, disabled bool) ui.Element {
	s := cs()
	e := ui.RadioBase(c, selected, value).Disabled(disabled).DimDisabled(false).FocusRing(false).
		Size(40, 40).Shrink(0)
	if !disabled {
		e = e.Cursor(ui.CursorPointer)
	}
	on := *selected == value
	ts := toggleReactions(e, disabled, e.Pressed())
	active, inactive := s.Primary, s.OnSurfaceVariant
	if ts.hovered || ts.focused {
		inactive = s.OnSurface
	}
	if disabled {
		active, inactive = DisabledContent(), DisabledContent()
	}
	overlay := func(sel bool, a float32) ui.Color {
		if sel {
			return design.WithOpacity(s.Primary, a)
		}
		return design.WithOpacity(s.OnSurface, a)
	}
	rc := pressedReaction(reactionColors{inactive: overlay(true, .1), active: overlay(false, .1), hover: overlay(on, .08), focus: overlay(on, .1)}, on, ts)
	pos := controller(e, "m3.radio", on, 200*time.Millisecond)
	t := pos.curved(design.EaseIn, design.EaseOut)
	return e.Draw(func(p *ui.Painter, r ui.Rect) {
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		ts.paint(p, cx, cy, rc, t)
		ring := design.Lerp(inactive, active, t)
		p.StrokePath((&ui.Path{}).Circle(cx, cy, 8), 2, ring)
		if pos.v > 0 {
			p.FillPath((&ui.Path{}).Circle(cx, cy, 4.5*t), ring)
		}
	})
}
