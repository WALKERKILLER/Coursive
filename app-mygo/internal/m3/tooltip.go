package m3

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
)

// RawTooltip's timings with the app's TooltipThemeData: the pointer rests
// 500 ms before a tip shows, a tip lingers 100 ms after it leaves, and a
// long press on a touch screen shows it until 1.5 s after the finger
// lifts. A tip fades in over 150 ms and out over 75.
const (
	tipWait  = 500 * time.Millisecond
	tipExit  = 100 * time.Millisecond
	tipTouch = 1500 * time.Millisecond
	tipIn    = 150 * time.Millisecond
	tipOut   = 75 * time.Millisecond
)

// tip is a RawTooltipState: whether a mouse hovers it, the timer to show
// or dismiss it, and its controller's direction.
type tip struct {
	hovering bool
	// held keeps a press from counting as a new hover until the pointer
	// leaves, as clearing _activeHoveringPointerDevices does.
	held      bool
	pressed   bool
	long      bool
	showAt    time.Time
	dismissAt time.Time
	open      bool
	closedAt  time.Time
	area      float32
}

// tips are RawTooltip._openedTooltips, those showing or fading out, and
// the anchors hovered in this frame and the last, of which the smallest
// takes the hover as _ExclusiveMouseRegion gives it to the innermost
// tooltip.
var (
	tipsOpen        = map[*tip]bool{}
	tipsHovered     = map[*tip]float32{}
	tipsHoveredLast = map[*tip]float32{}
	tipsFrame       time.Time
)

func (t *tip) opened(now time.Time) bool {
	return t.open || now.Sub(t.closedAt) < tipOut
}

func (t *tip) show(c *ui.Context, now time.Time, wait time.Duration, touch time.Duration) {
	t.dismissAt = time.Time{}
	if !t.opened(now) && wait > 0 {
		t.showAt = now.Add(wait)
		c.After(wait)
		return
	}
	t.showAt, t.open = time.Time{}, true
	tipsOpen[t] = true
	if touch > 0 {
		t.dismissAt = now.Add(touch)
		c.After(touch)
	}
}

func (t *tip) dismiss(c *ui.Context, now time.Time, wait time.Duration) {
	t.showAt, t.dismissAt = time.Time{}, time.Time{}
	if !t.open {
		return
	}
	if wait > 0 {
		t.dismissAt = now.Add(wait)
		c.After(wait)
		return
	}
	t.open, t.closedAt = false, now
	c.After(tipOut)
}

// Tip gives anchor Material's tooltip: inverseSurface, Corners.xs,
// bodySmall in onInverseSurface, 4 DIPs below the anchor as Flutter
// prefers, shown and dismissed as RawTooltip does. Moving from one tip
// to the next while the first still shows shows the next at once.
func Tip(c *ui.Context, anchor ui.Element, text string) ui.Element {
	if text == "" {
		return anchor
	}
	anchor = anchor.Description(text)
	t := ui.Local(anchor, "m3.tip", func() *tip { return &tip{} })
	st := *t
	now := c.Now()
	if now != tipsFrame {
		tipsFrame = now
		tipsHovered, tipsHoveredLast = tipsHoveredLast, tipsHovered
		clear(tipsHovered)
	}
	for o := range tipsOpen {
		if !o.opened(now) {
			delete(tipsOpen, o)
		}
	}
	b := anchor.Bounds()
	st.area = b.W * b.H
	raw := anchor.Hovered()
	if raw {
		tipsHovered[st] = st.area
	}
	for _, m := range []map[*tip]float32{tipsHoveredLast, tipsHovered} {
		for o, a := range m {
			if o != st && a < st.area {
				raw = false
			}
		}
	}
	if !raw {
		st.held = false
	}
	switch {
	case raw && !st.hovering && !st.held:
		st.hovering = true
		wait := tipWait
		for o := range tipsOpen {
			if o != st && !o.hovering {
				o.dismiss(c, now, 0)
				wait = 0
			}
		}
		st.show(c, now, wait, 0)
	case !raw && st.hovering:
		st.hovering = false
		st.dismiss(c, now, tipExit)
	}
	// A press that is no long press dismisses the tip, as the global
	// pointer route does; a long press by a finger shows it until 1.5 s
	// after the finger lifts.
	pressed := anchor.Pressed()
	if pressed && !st.pressed {
		st.dismiss(c, now, 0)
		st.hovering, st.held, st.long = false, true, false
	}
	if anchor.LongPressed() {
		st.long = true
		st.show(c, now, 0, 0)
	}
	if !pressed && st.pressed && st.long {
		st.long = false
		st.dismiss(c, now, tipTouch)
	}
	st.pressed = pressed
	if !st.showAt.IsZero() && !now.Before(st.showAt) {
		st.show(c, now, 0, 0)
	}
	if !st.dismissAt.IsZero() && !now.Before(st.dismissAt) {
		st.dismiss(c, now, 0)
	}
	if st.open && c.KeyDown(ui.KeyEscape) {
		for o := range tipsOpen {
			o.dismiss(c, now, 0)
		}
	}
	if !st.open {
		return anchor
	}
	ui.Overlay(c, func() {
		s := cs()
		box := ui.Column(c.Key(anchor.ID())).Role(ui.RoleTooltip).IgnoreInput(true).
			AttachTo(anchor, ui.AnchorBottom, ui.AnchorTop).Margin(4, 0, 0, 0).
			MinHeight(24).Padding(4, 8).Radius(design.CornerXS).Background(s.InverseSurface).
			MaxWidth(360).Justify(ui.Center).AttachedOrigin().
			// RawTooltip fades in over 150 ms and out over 75, fastOutSlowIn
			// both ways.
			Transition(ui.ElementTransition{Duration: tipIn, ExitDuration: tipOut,
				Ease: design.FastOutSlowIn, ExitEase: flipped(design.FastOutSlowIn),
				Enter: &ui.Motion{Opacity: 0}, Exit: &ui.Motion{Opacity: 0}})
		box.Children(func() { Text(c, text, tt().BodySmall.C(s.OnInverseSurface)) })
	})
	return anchor
}
