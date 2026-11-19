package m3

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
)

// ctlStatus is an AnimationController's status.
type ctlStatus uint8

const (
	ctlDismissed ctlStatus = iota
	ctlForward
	ctlReverse
	ctlCompleted
)

// ctl is an AnimationController following a flag, as the controllers of
// Flutter's toggles do: forward() while on and reverse() while off, the
// value running linearly over d for the whole way, so that a turn on the
// way takes the part of d that is left.
type ctl struct {
	v      float32
	status ctlStatus
	// dir is the direction CurvedAnimation goes by: the one it set out in,
	// which a turn on the way keeps until the value comes to rest.
	dir ctlStatus
}

type ctlState struct {
	last float32
	dir  ctlStatus
}

func controller(e ui.Element, key string, on bool, d time.Duration) ctl {
	to := float32(0)
	if on {
		to = 1
	}
	st := ui.Local(e, key+"/ctl", func() ctlState { return ctlState{last: to, dir: ctlDismissed} })
	left := to - st.last
	if left < 0 {
		left = -left
	}
	run := max(time.Duration(float32(d)*left), time.Millisecond)
	v := e.AnimateTo(key, to, ui.Animation{Duration: run, Ease: ui.Linear})
	st.last = v
	c := ctl{v: v}
	switch {
	case e.Settled(key) && v == 0:
		c.status = ctlDismissed
	case e.Settled(key):
		c.status = ctlCompleted
	case on:
		c.status = ctlForward
	default:
		c.status = ctlReverse
	}
	if c.status == ctlDismissed || c.status == ctlCompleted || st.dir == ctlDismissed || st.dir == ctlCompleted {
		st.dir = c.status
	}
	c.dir = st.dir
	return c
}

// curved is CurvedAnimation(curve:, reverseCurve:) of the controller; a
// nil reverse runs curve both ways.
func (c ctl) curved(curve, reverse ui.Easing) float32 {
	if c.v <= 0 || c.v >= 1 {
		return c.v
	}
	if reverse != nil && c.dir == ctlReverse {
		return reverse(c.v)
	}
	return curve(c.v)
}

// toggleStates are a toggle's controllers besides its position: the
// radial reaction of a press (100ms) and the fades of hover and focus
// (50ms), as ToggleableStateMixin has them.
type toggleStates struct {
	press, hover, focus ctl
	hovered, focused    bool
}

func toggleReactions(e ui.Element, disabled, pressed bool) toggleStates {
	t := toggleStates{hovered: e.Hovered() && !disabled, focused: e.FocusVisible() && !disabled}
	t.press = controller(e, "m3.toggle.press", pressed && !disabled, 100*time.Millisecond)
	t.hover = controller(e, "m3.toggle.hover", t.hovered, 50*time.Millisecond)
	t.focus = controller(e, "m3.toggle.focus", t.focused, 50*time.Millisecond)
	return t
}

// reactionColors are the overlay colours of a toggle's radial reaction:
// pressed while off and on, hovered and focused in its state.
type reactionColors struct{ inactive, active, hover, focus ui.Color }

// paint is ToggleablePainter.paintRadialReaction around x, y: a circle of
// radius 20 while hovered or focused, growing with the press otherwise, in
// the pressed colour of the position faded to the hover and focus ones.
func (t toggleStates) paint(p *ui.Painter, x, y float32, rc reactionColors, position float32) {
	if t.press.v == 0 && t.hover.v == 0 && t.focus.v == 0 {
		return
	}
	c := design.Lerp(rc.inactive, rc.active, position)
	c = design.Lerp(c, rc.hover, t.hover.curved(design.FastOutSlowIn, nil))
	c = design.Lerp(c, rc.focus, t.focus.curved(design.FastOutSlowIn, nil))
	r := float32(20)
	if !t.hovered && !t.focused {
		r *= t.press.curved(design.FastOutSlowIn, nil)
	}
	if r > 0 && c.A > 0 {
		p.FillPath((&ui.Path{}).Circle(x, y, r), c)
	}
}

// pressedReaction is rc as a check box or radio button pressed shows it:
// hovered or focused, in the pressed colour of its state.
func pressedReaction(rc reactionColors, on bool, t toggleStates) reactionColors {
	if t.press.status == ctlForward || t.press.status == ctlCompleted {
		if on {
			rc.hover, rc.focus = rc.active, rc.active
		} else {
			rc.hover, rc.focus = rc.inactive, rc.inactive
		}
	}
	return rc
}
