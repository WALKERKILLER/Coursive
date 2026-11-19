package widgets

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
)

// The dock's measures: the M3 flexible navigation bar's height, the lens
// inset in the pill and its concentric corner, and the room it takes above
// the window's bottom.
const (
	DockHeight    float32 = 64
	DockInset     float32 = design.SpaceXS
	DockLens              = DockHeight/2 - DockInset
	DockClearance         = DockHeight + design.SpaceMD
)

// Dock is glass_dock.dart's GlassNavDock: the phone's navigation bar as a
// glass pill floating over the page, with an accessory beside it. It
// shrinks to the current destination while the page scrolls down; the
// lens stretches toward a new destination before its trailing edge
// follows, and pressing and sliding along the pill carries it under the
// pointer, selecting where it is let go.
type Dock struct {
	Items     []NavItem
	Selected  int
	Minimized bool
	Accessory func()
}

// DockResult is what the user did: the destination chosen (or -1) and
// whether the minimized dock was tapped to expand.
type DockResult struct {
	Selected int
	Expand   bool
}

type dockState struct {
	from, to  float32
	toIndex   int
	movedAt   time.Time
	finger    float32
	scrubbing bool
	downX     float32
	lift      bool
	init      bool
}

// Show builds the dock.
func (d Dock) Show(c *ui.Context) DockResult {
	res := DockResult{Selected: -1}
	ui.Row(c).FillWidth().Height(DockHeight).AlignItems(ui.Center).Children(func() {
		ui.Row(c).Grow(1).Basis(0).FillHeight().AlignItems(ui.Center).Children(func() {
			if d.Minimized && d.Selected >= 0 {
				if d.collapsed(c) {
					res.Expand = true
				}
			} else {
				res.Selected = d.bar(c)
			}
		})
		if d.Accessory != nil {
			m3.HGap(c, design.SpaceSM)
			ui.Box(c).Size(DockHeight, DockHeight).Shrink(0).Children(d.Accessory)
		}
	})
	return res
}

func (d Dock) collapsed(c *ui.Context) bool {
	s := design.S()
	it := d.Items[d.Selected]
	pill := ui.Row(c.Key("dock-min")).Height(DockHeight).Padding(DockInset).Cursor(ui.CursorPointer).Label(it.Label).
		Transition(dockSwitch())
	pill = Glass{Radius: design.CornerFull}.Apply(pill)
	pill = m3.InkWell(pill, DockHeight/2)
	pill.Children(func() {
		ui.Row(c).FillHeight().PaddingX(design.SpaceLG).Background(s.SecondaryContainer).Radius(DockLens).AlignItems(ui.Center).Children(func() {
			navIcon(c, it, true, s.OnSecondaryContainer)
			m3.HGap(c, design.SpaceSM)
			m3.Text(c, it.Label, design.TT().LabelLarge.C(s.OnSecondaryContainer))
		})
	})
	return pill.Clicked()
}

func (d Dock) bar(c *ui.Context) int {
	s := design.S()
	tt := design.TT()
	n := len(d.Items)
	chosen := -1
	pill := ui.Box(c.Key("dock-bar")).FillWidth().Height(DockHeight).Padding(DockInset).
		Transition(dockSwitch())
	pill = Glass{Radius: design.CornerFull}.Apply(pill)
	st := ui.Local(pill, "dock", func() dockState { return dockState{} })
	w := pill.Bounds().W - 2*DockInset
	slot := float32(0)
	if n > 0 && w > 0 {
		slot = w / float32(n)
	}
	if !st.init || d.Selected != st.toIndex && !st.scrubbing {
		if st.init && slot > 0 {
			st.from = d.lensStart(st, slot)
			st.movedAt = time.Now()
		} else {
			st.movedAt = time.Time{}
		}
		st.toIndex, st.init = d.Selected, true
	}
	// Scrubbing: a press that slides along the pill carries the lens.
	x, _, ok := pill.PointerPosition()
	x -= DockInset
	if pill.Pressed() && ok && slot > 0 {
		if !st.lift {
			st.lift, st.downX = true, x
		}
		if st.scrubbing || abs32(x-st.downX) > 8 {
			st.scrubbing, st.finger = true, x
		}
	} else if st.lift {
		if st.scrubbing && slot > 0 {
			i := int(min(max(st.finger/slot, 0), float32(n-1)))
			st.from, st.movedAt = d.lensStart(st, slot), time.Now()
			st.scrubbing = false
			st.toIndex = i
			if i != d.Selected {
				chosen = i
			}
		}
		st.lift = false
	}
	over := st.toIndex
	if st.scrubbing {
		over = int(min(max(st.finger/slot, 0), float32(n-1)))
	}
	lift := pill.AnimateTo("dock.lift", map[bool]float32{true: 1, false: 0}[st.scrubbing], design.T().Motion(design.Short4, ui.Linear))
	lift = design.Emphasized(lift)
	pill.Children(func() {
		if (st.toIndex >= 0 || st.scrubbing) && slot > 0 {
			a, b := d.edges(st, slot)
			if time.Since(st.movedAt) < design.Medium4 {
				c.Invalidate()
			}
			lens := ui.Box(c).Absolute().Top(0).Bottom(0).Left(a).Width(b-a).Radius(DockLens).
				Background(design.WithOpacity(s.SecondaryContainer, 1-.28*lift)).Scale(1+.06*lift, 1+.06*lift)
			if lift > 0 {
				lens.Shadow(0, 2, 24, 0, design.WithOpacity(s.Shadow, .16*lift))
			}
		}
		ui.Row(c).Fill().Children(func() {
			for i, it := range d.Items {
				lit := i == over
				col := s.OnSurfaceVariant
				if lit {
					col = s.OnSecondaryContainer
				}
				dest := ui.Column(c.Key(i)).Grow(1).Basis(0).FillHeight().Justify(ui.Center).AlignItems(ui.Center).
					Role(ui.RoleTab).Checked(i == d.Selected).Label(it.Label).Cursor(ui.CursorPointer)
				dest = m3.InkWell(dest, DockLens)
				dest.Children(func() {
					navIcon(c, it, lit, col)
					m3.VGap(c, design.SpaceXXS)
					ls := tt.LabelMedium.C(col)
					ls.Size = min(ls.Size*design.T().TextScale, ls.Size*1.3) / design.T().TextScale
					m3.Text(c, it.Label, ls).NoWrap().Clip()
				})
				if dest.Clicked() && !st.scrubbing && chosen < 0 {
					chosen = i
				}
			}
		})
	})
	return chosen
}

// edges are the lens's start and end, from the start of the row: the
// leading edge runs ahead and the trailing one follows.
func (d Dock) edges(st *dockState, w float32) (float32, float32) {
	if st.scrubbing {
		s := min(max(st.finger-w/2, 0), w*float32(len(d.Items)-1))
		return s, s + w
	}
	b := float32(st.toIndex) * w
	t := float32(1)
	if !st.movedAt.IsZero() && !design.T().ReduceMotion {
		t = min(float32(time.Since(st.movedAt))/float32(design.Medium4), 1)
	}
	a := st.from
	if t >= 1 {
		a = b
	}
	lead, trail := design.EmphasizedDecelerate(t), design.Emphasized(t)
	if b >= a {
		return a + (b-a)*trail, a + (b-a)*lead + w
	}
	return a + (b-a)*lead, a + (b-a)*trail + w
}

func (d Dock) lensStart(st *dockState, w float32) float32 {
	a, _ := d.edges(st, w)
	return a
}

func abs32(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

// dockSwitch is the AnimatedSwitcher between the bar and its minimized
// form: a fade and a scale from 92%.
func dockSwitch() ui.ElementTransition {
	return ui.ElementTransition{Duration: design.T().Motion(design.Medium2, nil).Duration, Ease: design.EmphasizedDecelerate,
		Enter: &ui.Motion{Opacity: 0, Scale: .92}, ExitEase: design.EmphasizedAccelerate}
}
