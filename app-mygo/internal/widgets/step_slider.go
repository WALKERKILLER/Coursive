package widgets

import (
	"math"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
)

// StepSlider is step_slider.dart: a discrete slider over a few named stops
// (M3 Expressive slider: 16 DIP track, 4 DIP bar handle with a gap on both
// sides, stop indicators), with each stop's label below it. The active
// track deepens from a light tone of Active to the full colour, so the
// position reads as intensity.
//
// The handle follows the pointer while dragging and snaps to the nearest
// stop on release; Show reports a committed change once.
type StepSlider struct {
	Labels   []string
	Disabled bool
	// Label names the slider for assistive technology.
	Label string
	// Active, Inactive and LabelColor replace primary, secondaryContainer
	// and onSurfaceVariant.
	Active, Inactive, LabelColor ui.Color
}

type stepSliderState struct {
	pos      float64
	pressed  bool
	shown    int
	dragging bool
}

const (
	stepHeight = 44 // handle height and touch target
	stepInset  = 8  // keeps the end stops off the rounded track ends
)

// Show builds the slider on *index and reports whether the user chose
// another stop.
func (s StepSlider) Show(c *ui.Context, index *int) bool {
	cs := design.S()
	tt := design.TT()
	n := len(s.Labels)
	last := n - 1
	if last < 1 {
		return false
	}
	active, inactive, label := cs.Primary, cs.SecondaryContainer, cs.OnSurfaceVariant
	if s.Active.A != 0 {
		active = s.Active
	}
	if s.Inactive.A != 0 {
		inactive = s.Inactive
	}
	if s.LabelColor.A != 0 {
		label = s.LabelColor
	}
	onActive, onInactive := cs.OnPrimary, cs.OnSecondaryContainer
	if s.Disabled {
		active = design.WithOpacity(cs.OnSurface, .38)
		inactive = design.WithOpacity(cs.OnSurface, .12)
		onActive, onInactive = cs.Surface, design.WithOpacity(cs.OnSurface, .38)
	}
	changed := false
	wrap := ui.Column(c).FillWidth().Shrink(0)
	st := ui.Local(wrap, "step-slider", func() *stepSliderState { return &stepSliderState{pos: float64(*index), shown: *index} })
	wrap.Children(func() {
		commit := func(i int) {
			i = min(max(i, 0), last)
			(*st).shown = i
			(*st).pos = float64(i)
			if i != *index {
				*index = i
				changed = true
			}
		}
		e := ui.SliderBase(c, &(*st).pos, 0, float64(last)).Disabled(s.Disabled).DimDisabled(false).FocusRing(false).
			FillWidth().Height(stepHeight).Padding(0, stepInset).Shrink(0)
		if s.Label != "" {
			e = e.Label(s.Label)
		}
		if !s.Disabled {
			e = e.Cursor(ui.CursorPointer)
		}
		pressed := e.Pressed()
		if pressed {
			(*st).dragging = true
			(*st).shown = int(math.Round((*st).pos))
		} else if (*st).dragging {
			(*st).dragging = false
			commit(int(math.Round((*st).pos)))
		} else if e.Changed() {
			// A key moved it.
			commit(int(math.Round((*st).pos)))
		} else if (*st).shown != *index {
			(*st).shown, (*st).pos = *index, float64(*index)
		}
		if e.Focused() && !s.Disabled {
			step := func(d int) { commit((*st).shown + d) }
			switch {
			case e.Shortcut(0, ui.KeyRight), e.Shortcut(0, ui.KeyUp):
				step(1)
			case e.Shortcut(0, ui.KeyLeft), e.Shortcut(0, ui.KeyDown):
				step(-1)
			case e.Shortcut(0, ui.KeyHome):
				commit(0)
			case e.Shortcut(0, ui.KeyEnd):
				commit(last)
			}
		}
		target := float32((*st).pos)
		frac := target / float32(last)
		if !(*st).dragging {
			frac = e.AnimateTo("step.pos", target, design.T().Motion(design.Medium1, design.EmphasizedDecelerate)) / float32(last)
		}
		hw := float32(4)
		if (*st).dragging {
			hw = 2
		}
		var focus ui.Color
		switch {
		case e.FocusVisible():
			focus = cs.Secondary
		case e.Hovered() && !s.Disabled:
			focus = design.WithOpacity(active, .24)
		}
		e.Draw(func(p *ui.Painter, r ui.Rect) {
			const track, gap, outer, inner, dot = 16, 6, 8, 2, 2
			w := r.W
			cy := r.Y + r.H/2
			span := w - 2*stepInset
			x := r.X + stepInset + frac*span
			top := cy - track/2
			start := design.Lerp(inactive, active, .45)
			if aEnd := x - gap - hw/2; aEnd-r.X > .5 {
				end := design.Lerp(start, active, (aEnd-r.X)/w)
				rect := ui.Rect{X: r.X, Y: top, W: aEnd - r.X, H: track}
				p.FillGradient(rect, ui.LinearGradient{From: start, To: end, Angle: 90}, outer)
				if rect.W > outer {
					p.FillRadii(ui.Rect{X: aEnd - outer, Y: top, W: outer, H: track}, [4]float32{0, inner, inner, 0}, end)
				}
			}
			if iStart := x + gap + hw/2; r.X+w-iStart > .5 {
				p.FillRadii(ui.Rect{X: iStart, Y: top, W: r.X + w - iStart, H: track}, [4]float32{inner, outer, outer, inner}, inactive)
			}
			for i := 0; i < n; i++ {
				sx := r.X + stepInset + float32(i)/float32(last)*span
				if float32(math.Abs(float64(sx-x))) < gap+hw {
					continue
				}
				col := design.WithOpacity(onInactive, .7)
				if sx < x {
					col = design.WithOpacity(onActive, .9)
				}
				p.FillPath((&ui.Path{}).Circle(sx, cy, dot), col)
			}
			h := ui.Rect{X: x - hw/2, Y: r.Y, W: hw, H: r.H}
			if focus.A != 0 {
				p.Stroke(ui.Rect{X: h.X - 3, Y: h.Y - 3, W: h.W + 6, H: h.H + 6}, focus, hw/2+3, 2)
			}
			p.Fill(h, active, hw/2)
		})
		m3.VGap(c, design.SpaceXS)
		// Labels centred under their stops, kept inside the bounds.
		shown := (*st).shown
		lst := tt.LabelMedium
		labels := ui.Box(c).FillWidth().Height(lst.Scaled()*1.4 + 2*design.SpaceXXS)
		if !s.Disabled {
			labels = labels.Cursor(ui.CursorPointer)
		}
		if labels.Clicked() && !s.Disabled {
			if px, _, ok := labels.PointerPosition(); ok {
				w := labels.Bounds().W
				i := int(math.Round(float64((px - stepInset) / max(1, w-2*stepInset) * float32(last))))
				commit(i)
			}
		}
		labels.Draw(func(p *ui.Painter, r ui.Rect) {
			span := r.W - 2*stepInset
			for i, l := range s.Labels {
				st := lst.C(label)
				if i == shown {
					st = st.W(600)
					if !s.Disabled {
						st.Color = active
					}
				}
				sp := st.Span(l)
				lw, _ := p.MeasureText(r.W, sp)
				center := r.X + stepInset + float32(i)/float32(last)*span
				left := min(max(center-lw/2, r.X), r.X+r.W-lw)
				p.RichText(left, r.Y+design.SpaceXXS, lw+1, sp)
			}
		})
	})
	return changed
}
