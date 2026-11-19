package m3

import (
	"fmt"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
)

// Slider is Slider (year2023: false) with the app's SliderTheme: a 12 DIP
// track, its inactive part in secondaryContainer, a 4×44 handle with the
// track 6 DIPs from its centre on each side (trackGap), and a stop dot at
// the end.
type Slider struct {
	Min, Max  float64
	Divisions int
	// Label shows above the handle while it is dragged.
	Label    string
	Disabled bool
	// Active and Inactive replace the track colours.
	Active, Inactive ui.Color
	// TrackHeight replaces 12.
	TrackHeight float32
}

// SliderResult is a built slider: the base, and whether a drag ended
// this frame (onChangeEnd).
type SliderResult struct {
	ui.Element
	Ended bool
}

type sliderState struct{ pressed bool }

// Show builds the slider editing value.
func (sl Slider) Show(c *ui.Context, value *float64) SliderResult {
	s := cs()
	if sl.Max == sl.Min {
		sl.Max = sl.Min + 1
	}
	e := ui.SliderBase(c, value, sl.Min, sl.Max).Disabled(sl.Disabled).DimDisabled(false).FocusRing(false).
		Height(48).Padding(0, 20).FillWidth().Shrink(0)
	if sl.Divisions > 0 {
		e = e.Step((sl.Max - sl.Min) / float64(sl.Divisions))
	}
	if !sl.Disabled {
		e = e.Cursor(ui.CursorPointer)
	}
	st := ui.Local(e, "m3.slider", func() sliderState { return sliderState{} })
	pressed := e.Pressed()
	ended := st.pressed && !pressed
	st.pressed = pressed
	active, inactive, handle := s.Primary, s.SecondaryContainer, s.Primary
	if sl.Active.A != 0 {
		active, handle = sl.Active, sl.Active
	}
	if sl.Inactive.A != 0 {
		inactive = sl.Inactive
	}
	if sl.Disabled {
		active = design.WithOpacity(s.OnSurface, .38)
		handle = active
		inactive = design.WithOpacity(s.OnSurface, .12)
	}
	th := sl.TrackHeight
	if th == 0 {
		th = 12
	}
	frac := float32((*value - sl.Min) / (sl.Max - sl.Min))
	frac = min(max(frac, 0), 1)
	// The handle narrows to 2 at once while pressed, and the track's gap,
	// 6 from its centre, narrows with it: 4 from the handle either way.
	hw := float32(4)
	if pressed {
		hw = 2
	}
	label := sl.Label
	divs := sl.Divisions
	focus := e.FocusVisible()
	e = e.Draw(func(p *ui.Painter, r ui.Rect) {
		x0, x1 := r.X+20, r.X+r.W-20
		w := x1 - x0
		cy := r.Y + r.H/2
		hx := x0 + w*frac
		const gap = 4
		outer, inner := th/2, float32(2)
		top := cy - th/2
		if a := hx - gap - hw/2 - x0; a > 0 {
			p.FillRadii(ui.Rect{X: x0, Y: top, W: a, H: th}, [4]float32{outer, inner, inner, outer}, active)
		}
		if b := x1 - (hx + gap + hw/2); b > 0 {
			p.FillRadii(ui.Rect{X: hx + gap + hw/2, Y: top, W: b, H: th}, [4]float32{inner, outer, outer, inner}, inactive)
			p.FillPath((&ui.Path{}).Circle(x1-th/2, cy, 2), active)
		}
		if divs > 0 {
			for i := 0; i <= divs; i++ {
				tx := x0 + w*float32(i)/float32(divs)
				if tx > hx-gap-hw && tx < hx+gap+hw {
					continue
				}
				col := s.OnSecondaryContainer
				if tx < hx {
					col = s.OnPrimary
				}
				if tx-x0 < th/2 || x1-tx < th/2 {
					continue
				}
				p.FillPath((&ui.Path{}).Circle(tx, cy, 1), col)
			}
		}
		p.Fill(ui.Rect{X: hx - hw/2, Y: cy - 22, W: hw, H: 44}, handle, hw/2)
		if focus {
			p.Stroke(ui.Rect{X: hx - hw/2 - 3, Y: cy - 25, W: hw + 6, H: 50}, s.Secondary, 4, 2)
		}
		if pressed && label != "" {
			sp := ui.Span{Text: label, Size: tt().LabelLarge.Scaled(), Weight: 500, Color: s.OnInverseSurface}
			lw, lh := p.MeasureText(200, sp)
			bw, bh := max(lw+32, 48), lh+16
			bx, by := hx-bw/2, cy-22-8-bh
			p.Fill(ui.Rect{X: bx, Y: by, W: bw, H: bh}, s.InverseSurface, bh/2)
			p.RichText(bx+(bw-lw)/2, by+8, bw, sp)
		}
	})
	return SliderResult{e, ended}
}

// Fmt1 formats a slider label with one decimal, as toStringAsFixed(1).
func Fmt1(v float64) string { return fmt.Sprintf("%.1f", v) }
