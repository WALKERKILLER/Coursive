package m3

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
)

// Switch is Material 3's switch with the app's thumb icon: a check on the
// thumb while on. It returns the base, whose Changed reports a toggle.
func Switch(c *ui.Context, on *bool, disabled bool) ui.Element {
	s := cs()
	e := ui.SwitchBase(c, on).Disabled(disabled).DimDisabled(false).FocusRing(false).
		Size(52, 32).Shrink(0)
	if !disabled {
		e = e.Cursor(ui.CursorPointer)
	}
	sel := *on
	track, thumb, border, iconC := s.SurfaceContainerHighest, s.Outline, s.Outline, s.OnPrimaryContainer
	if sel {
		track, thumb, border = s.Primary, s.OnPrimary, s.Primary
	}
	hovered, pressed := e.Hovered(), e.Pressed()
	if !disabled && (hovered || pressed || e.FocusVisible()) {
		if sel {
			thumb = s.PrimaryContainer
		} else {
			thumb = s.OnSurfaceVariant
		}
	}
	if disabled {
		if sel {
			track = design.WithOpacity(s.OnSurface, .12)
			thumb = s.Surface
			border = ui.Transparent
			iconC = design.WithOpacity(s.OnSurface, .38)
		} else {
			track = design.WithOpacity(s.SurfaceContainerHighest, .12)
			thumb = design.WithOpacity(s.OnSurface, .38)
			border = design.WithOpacity(s.OnSurface, .12)
		}
	}
	size := float32(16)
	if sel {
		size = 24
	}
	if pressed && !disabled {
		size = 28
	}
	anim := design.T().Motion(300*time.Millisecond, design.EaseOutCubic)
	pos := e.AnimateTo("m3.switch.pos", map[bool]float32{false: 0, true: 1}[sel], anim)
	sz := e.AnimateTo("m3.switch.size", size, design.T().Motion(250*time.Millisecond, design.EaseOutCubic))
	trackC := e.AnimateColor("m3.switch.track", track, anim)
	thumbC := e.AnimateColor("m3.switch.thumb", thumb, anim)
	borderC := e.AnimateColor("m3.switch.border", border, anim)
	var layer float32
	switch {
	case disabled:
	case pressed:
		layer = .1
	case e.FocusVisible():
		layer = .1
	case hovered:
		layer = .08
	}
	layerC := s.OnSurface
	if sel {
		layerC = s.Primary
	}
	return e.Draw(func(p *ui.Painter, r ui.Rect) {
		p.Fill(r, trackC, 16)
		if borderC.A != 0 {
			p.Stroke(r, borderC, 16, 2)
		}
		// The thumb's centre runs from 16 to 36 DIPs.
		cx := r.X + 16 + 20*pos
		cy := r.Y + 16
		if layer > 0 {
			p.FillPath((&ui.Path{}).Circle(cx, cy, 20), design.WithOpacity(layerC, layer))
		}
		p.FillPath((&ui.Path{}).Circle(cx, cy, sz/2), thumbC)
		if pos > .5 {
			IconPaint(p, icons.CheckRounded, cx-8, cy-8, 16, design.WithOpacity(iconC, min(1, (pos-.5)*2)))
		}
	})
}

// Checkbox is Material 3's check box: an 18 DIP box in a 40 DIP target.
func Checkbox(c *ui.Context, checked *bool, disabled bool) ui.Element {
	s := cs()
	e := ui.CheckboxBase(c, checked).Disabled(disabled).DimDisabled(false).FocusRing(false).
		Size(40, 40).Center().Shrink(0)
	if !disabled {
		e = e.Cursor(ui.CursorPointer)
	}
	on := *checked
	fill, side := s.Primary, s.OnSurfaceVariant
	if disabled {
		fill, side = DisabledContent(), DisabledContent()
	}
	var layer float32
	switch {
	case disabled:
	case e.Pressed():
		layer = .1
	case e.FocusVisible():
		layer = .1
	case e.Hovered():
		layer = .08
	}
	layerC := s.OnSurface
	if on {
		layerC = s.Primary
	}
	t := e.AnimateTo("m3.check", map[bool]float32{false: 0, true: 1}[on], design.T().Motion(200*time.Millisecond, ui.Linear))
	return e.Draw(func(p *ui.Painter, r ui.Rect) {
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		if layer > 0 {
			p.FillPath((&ui.Path{}).Circle(cx, cy, 20), design.WithOpacity(layerC, layer))
		}
		box := ui.Rect{X: cx - 9, Y: cy - 9, W: 18, H: 18}
		if t > 0 {
			p.Fill(box, design.WithOpacity(fill, t), 2)
			check := (&ui.Path{}).MoveTo(cx-5, cy).LineTo(cx-1.5, cy+3.5).LineTo(cx+5.5, cy-3.5)
			check = check.Trim(0, t).Cap(ui.SquareCap).Join(ui.MiterJoin)
			on := s.OnPrimary
			if disabled {
				on = s.Surface
			}
			p.StrokePath(check, 2, on)
		}
		if t < 1 {
			p.Stroke(ui.Rect{X: box.X + 1, Y: box.Y + 1, W: 16, H: 16}, design.WithOpacity(side, 1-t), 1, 2)
		}
	})
}

// Radio is Material 3's radio button: a 20 DIP ring in a 40 DIP target.
func Radio[T comparable](c *ui.Context, selected *T, value T, disabled bool) ui.Element {
	s := cs()
	e := ui.RadioBase(c, selected, value).Disabled(disabled).DimDisabled(false).FocusRing(false).
		Size(40, 40).Shrink(0)
	if !disabled {
		e = e.Cursor(ui.CursorPointer)
	}
	on := *selected == value
	ring := s.OnSurfaceVariant
	if on {
		ring = s.Primary
	}
	if disabled {
		ring = DisabledContent()
	}
	var layer float32
	switch {
	case disabled:
	case e.Pressed():
		layer = .1
	case e.FocusVisible():
		layer = .1
	case e.Hovered():
		layer = .08
	}
	layerC := s.OnSurface
	if on {
		layerC = s.Primary
	}
	dot := e.AnimateTo("m3.radio", map[bool]float32{false: 0, true: 1}[on], design.T().Motion(200*time.Millisecond, design.EaseOutCubic))
	ringC := e.AnimateColor("m3.radio.c", ring, design.T().Motion(200*time.Millisecond, ui.Linear))
	return e.Draw(func(p *ui.Painter, r ui.Rect) {
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		if layer > 0 {
			p.FillPath((&ui.Path{}).Circle(cx, cy, 20), design.WithOpacity(layerC, layer))
		}
		p.StrokePath((&ui.Path{}).Circle(cx, cy, 9), 2, ringC)
		if dot > 0 {
			p.FillPath((&ui.Path{}).Circle(cx, cy, 5*dot), ringC)
		}
	})
}
