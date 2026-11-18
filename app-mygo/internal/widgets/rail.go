package widgets

import (
	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
)

// NavItem is a destination of the rail and the dock, with the count its
// badge shows (Badge.count) or a bare dot.
type NavItem struct {
	Icon, SelectedIcon icons.Icon
	Label              string
	Count              int
	Dot                bool
}

func navIcon(c *ui.Context, it NavItem, selected bool, color ui.Color) {
	ic := it.Icon
	if selected {
		ic = it.SelectedIcon
	}
	label := ""
	if it.Count > 0 {
		label = itoa(it.Count)
		if it.Count > 999 {
			label = "999+"
		}
	}
	m3.Badged(c, label, it.Count > 0 || it.Dot, func() { m3.Icon(c, ic, 24, color) })
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// RailWidth is the M3 navigation rail's width.
const RailWidth float32 = 80

// Rail is glass_rail.dart's GlassNavRail: a glass column of destinations
// between controls pinned to its top and bottom. On a short window the brand
// steps aside first, then the destinations drop their labels (a tooltip
// keeps each name), and then they scroll, fading at the edge with more.
type Rail struct {
	Items    []NavItem
	Selected int
	Brand    func()
	Leading  func()
	Trailing func()
}

type railState struct {
	scroll ui.ScrollState
	height float32
}

// Show builds the rail and returns the destination tapped, or -1.
func (r Rail) Show(c *ui.Context) int {
	tapped := -1
	pane := ui.Column(c).Width(RailWidth).FillHeight().Shrink(0).AlignItems(ui.Center)
	pane = Glass{Radius: design.CornerXL}.Apply(pane)
	st := ui.Local(pane, "rail", func() railState { return railState{} })
	h := pane.Bounds().H
	pane.Children(func() {
		m3.VGap(c, design.SpaceLG)
		if r.Brand != nil && (h == 0 || h >= 600) {
			r.Brand()
			m3.VGap(c, design.SpaceLG)
		}
		if r.Leading != nil {
			r.Leading()
			m3.VGap(c, design.SpaceMD)
		}
		box := ui.Column(c).Grow(1).Basis(0).FillWidth()
		avail := box.Bounds().H
		if avail > 0 {
			st.height = avail
		}
		box.Children(func() { tapped = r.destinations(c, st) })
		if r.Trailing != nil {
			m3.VGap(c, design.SpaceMD)
			r.Trailing()
		}
		m3.VGap(c, design.SpaceLG)
	})
	return tapped
}

func (r Rail) destinations(c *ui.Context, st *railState) int {
	s := design.S()
	const labelled, bare float32 = 64, 48
	n := len(r.Items)
	labels := st.height == 0 || st.height >= float32(n)*labelled
	h, top := bare, (bare-32)/2
	if labels {
		h, top = labelled, design.SpaceXS
	}
	tapped := -1
	sc := ui.Scroll(c).Fill().Scrollbars(false).TrackScroll(&st.scroll)
	off, maxOff := st.scroll.Y, st.scroll.MaxY
	less, more := off > .5, maxOff-off > .5
	if less || more {
		edge := design.SpaceXL / max(st.height, 1)
		a := func(b bool) ui.Color {
			if b {
				return ui.Transparent
			}
			return ui.RGB(0, 0, 0)
		}
		sc = sc.MaskLinearGradient(ui.LinearGradient{Angle: 180, Stops: []ui.GradientStop{
			{Color: a(less), At: 0}, {Color: ui.RGB(0, 0, 0), At: edge}, {Color: ui.RGB(0, 0, 0), At: 1 - edge}, {Color: a(more), At: 1},
		}})
	}
	sc.Children(func() {
		ui.Box(c).FillWidth().Height(float32(n) * h).Children(func() {
			if r.Selected >= 0 {
				ind := ui.Box(c).Absolute().Left((RailWidth-56)/2).Size(56, 32).Background(s.SecondaryContainer).Radius(16)
				y := ind.AnimateTo("rail.ind", float32(r.Selected)*h+top, design.T().Motion(design.Medium4, design.Emphasized))
				ind.Top(y)
			}
			ui.Column(c).Absolute().Top(0).Left(0).Right(0).Children(func() {
				for i, it := range r.Items {
					sel := i == r.Selected
					tile := ui.Column(c.Key(i)).Height(h).Width(RailWidth - 8).MarginX(design.SpaceXS).AlignItems(ui.Center).
						Role(ui.RoleTab).Selected(sel).Label(it.Label).Cursor(ui.CursorPointer)
					tile = m3.InkWell(tile, design.CornerLG)
					tile.Children(func() {
						m3.VGap(c, top)
						col := s.OnSurfaceVariant
						if sel {
							col = s.OnSecondaryContainer
						}
						ui.Box(c).Height(32).Center().Children(func() { navIcon(c, it, sel, col) })
						if labels {
							m3.VGap(c, design.SpaceXS)
							lc, w := s.OnSurfaceVariant, 500
							if sel {
								lc, w = s.OnSurface, 600
							}
							st := design.TT().LabelMedium.C(lc).W(w)
							st.Size = min(st.Size*design.T().TextScale, st.Size*1.2) / design.T().TextScale
							m3.Clamp(m3.Text(c, it.Label, st), 1)
						}
					})
					if !labels {
						m3.Tip(c, tile, it.Label)
					}
					if tile.Clicked() {
						tapped = i
					}
				}
			})
		})
	})
	return tapped
}
