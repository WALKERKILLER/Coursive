package widgets

import (
	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
)

// HScroller is horizontal_scroller.dart's HorizontalScroller: a row that
// scrolls sideways, with previous and next buttons at the edges with more
// content while it is hovered, and optionally faded under them.
type HScroller struct {
	State   *ui.ScrollState
	Fade    bool
	Buttons bool
	// Step is how far a button scrolls; 0 is 80% of the view.
	Step float32
	Gap  float32
}

// Show builds the scroller around content's row of children.
func (h HScroller) Show(c *ui.Context, content func()) ui.Element {
	st := h.State
	before, after := st.X > 1, st.MaxX-st.X > 1
	box := ui.Box(c).FillWidth()
	hover := box.Hovered()
	box.Children(func() {
		sc := ui.ScrollHorizontal(c).FillWidth().Scrollbars(false).TrackScroll(st)
		if h.Gap > 0 {
			sc = sc.Gap(h.Gap)
		}
		if h.Fade && (before || after) {
			w := max(sc.Bounds().W, 1)
			f := min(32/w, .5)
			a := func(more bool) ui.Color {
				if more {
					return ui.Transparent
				}
				return ui.RGB(0, 0, 0)
			}
			sc = sc.MaskLinearGradient(ui.LinearGradient{Angle: 90, Stops: []ui.GradientStop{
				{Color: a(before), At: 0}, {Color: ui.RGB(0, 0, 0), At: f}, {Color: ui.RGB(0, 0, 0), At: 1 - f}, {Color: a(after), At: 1},
			}})
		}
		sc.Children(content)
		if !h.Buttons {
			return
		}
		view := sc.Bounds().W
		step := h.Step
		if step == 0 {
			step = view * .8
		}
		edge := func(back, visible bool) {
			b := ui.Box(c.Key(back)).Absolute().Top(0).Bottom(0).AlignItems(ui.Center).Justify(ui.Center)
			if back {
				b = b.Left(design.SpaceXS)
			} else {
				b = b.Right(design.SpaceXS)
			}
			op := b.AnimateTo("edge", map[bool]float32{true: 1, false: 0}[visible], design.T().Motion(design.Short4, design.EmphasizedDecelerate))
			b.Opacity(op).IgnoreInput(!visible).Scale(.8+.2*op, .8+.2*op)
			b.Children(func() {
				ic, tip := icons.ChevronRightRounded, l10n.ScrollRight
				if back {
					ic, tip = icons.ChevronLeftRounded, l10n.ScrollLeft
				}
				holder := ui.Box(c).Radius(design.CornerFull).Background(design.S().SurfaceContainerHighest)
				m3.Elevation(holder, 2)
				holder.Children(func() {
					if (m3.IconButton{Icon: ic, Tooltip: tip, Size: 20, Box: 32, Color: design.S().OnSurface}).Show(c).Clicked() {
						to := st.X + step
						if back {
							to = st.X - step
						}
						st.ScrollTo(min(max(to, 0), st.MaxX), st.Y, design.T().Motion(design.Long2, design.Emphasized))
					}
				})
			})
		}
		edge(true, hover && before)
		edge(false, hover && after)
	})
	return box
}
