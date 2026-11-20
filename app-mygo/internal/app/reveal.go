package app

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// listReveal is Scrollable.ensureVisible for a row of a ui.List, whose
// ListState.ScrollTo cannot glide: Show asks for a row to go to an
// alignment, 0 putting its top at the list's top and 1 its bottom at the
// list's bottom, along an animation. Build the rows with Rows and pass
// the list to Track. A row the list built glides there; one it did not,
// the list first jumps to (ListState.ScrollTo), as the Flutter pages
// jumpTo an estimate, and the row then goes the rest of the way along
// then, or at once when then is nil.
type listReveal struct {
	scroll     ui.ScrollState
	row        int
	align      float32
	anim, then *ui.Animation
	// tries counts the frames left to find the row; 0 asks for nothing.
	tries  int
	jumped bool
	end    *ui.Animation
	// top puts the row's top at align of the list's height, as an
	// animateTo the row's offset less a fraction of the viewport does.
	top   bool
	el    ui.Element
	built bool
}

// revealNearest is the alignment that scrolls as little as shows the row,
// as ScrollPositionAlignmentPolicy.keepVisibleAtEnd and AtStart do.
const revealNearest = -1

// Show asks for row to go to align along anim, along then after a jump.
func (v *listReveal) Show(row int, align float32, anim, then *ui.Animation) {
	v.row, v.align, v.anim, v.then, v.tries, v.jumped, v.end, v.top = row, align, anim, then, 4, false, nil, false
}

// ShowTop asks for row's top to go to frac of the list's height, as
// animateTo(offset - viewportDimension * frac) does.
func (v *listReveal) ShowTop(row int, frac float32, anim, then *ui.Animation) {
	v.Show(row, frac, anim, then)
	v.top = true
}

// End asks for the list's end along anim, as animateTo its last offset.
func (v *listReveal) End(anim ui.Animation) {
	v.tries, v.end = 0, &anim
}

// Rows wraps the list's row builder, keeping the row asked for.
func (v *listReveal) Rows(c *ui.Context, row func(i int)) func(i int) {
	return func(i int) {
		e := ui.Column(c).FillWidth()
		e.Children(func() { row(i) })
		if v.tries > 0 && i == v.row {
			v.el, v.built = e, true
		}
	}
}

// Track follows list, built with l, and moves it as asked.
func (v *listReveal) Track(c *ui.Context, list ui.Element, l *ui.ListState) ui.Element {
	v.move(c, list, l)
	// The container takes the offset as it is tracked, after the move.
	return list.TrackScroll(&v.scroll)
}

func (v *listReveal) move(c *ui.Context, list ui.Element, l *ui.ListState) {
	if v.end != nil {
		v.scroll.ScrollTo(v.scroll.X, math.MaxFloat32, *v.end)
		v.end = nil
	}
	if v.tries == 0 {
		return
	}
	built := v.built && v.el.LayoutBounds().H > 0
	v.built = false
	switch {
	case built:
		lb, rb := list.LayoutBounds(), v.el.LayoutBounds()
		h := rb.H
		if v.top {
			h = 0
		}
		to := min(max(v.scroll.Y+revealDelta(rb.Y-lb.Y, h, lb.H, v.align), 0), v.scroll.MaxY)
		anim := v.anim
		if v.jumped {
			anim = v.then
		}
		if anim == nil {
			v.scroll.Y = to
		} else {
			v.scroll.ScrollTo(v.scroll.X, to, *anim)
		}
		v.tries = 0
	case !v.jumped:
		l.ScrollTo(v.row, ui.Start)
		v.jumped = true
		v.tries--
	default:
		v.tries--
	}
	if v.tries > 0 {
		c.AnimationFrame()
	}
	v.el = ui.Element{}
}

// revealDelta is how far to scroll for what is y down a viewport of
// height view, and h high, to come to align.
func revealDelta(y, h, view, align float32) float32 {
	switch {
	case align >= 0:
		return y - align*(view-h)
	case y < 0:
		return y
	case y+h > view:
		return y + h - view
	}
	return 0
}

// ensureVisible glides scroll, tracking the scroll container viewport, to
// show e at align as Scrollable.ensureVisible does, along anim. It
// reports false while e is not laid out yet: ask again in the next frame.
func ensureVisible(scroll *ui.ScrollState, viewport, e ui.Element, align float32, anim ui.Animation) bool {
	vb, eb := viewport.LayoutBounds(), e.LayoutBounds()
	if eb.H == 0 || vb.H == 0 {
		return false
	}
	to := min(max(scroll.Y+revealDelta(eb.Y-vb.Y, eb.H, vb.H, align), 0), scroll.MaxY)
	scroll.ScrollTo(scroll.X, to, anim)
	return true
}
