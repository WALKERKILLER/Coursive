package m3

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
)

// DropdownEntry is a choice of a Dropdown.
type DropdownEntry[T comparable] struct {
	Value T
	Label string
	Icon  icons.Icon
}

// DropdownOpts are DropdownMenu's (and DropdownButtonFormField's) options.
type DropdownOpts struct {
	Label       string
	LeadingIcon icons.Icon
	Helper      string
	Disabled    bool
	// Width fixes the width; zero fills the parent (expandedInsets: zero).
	Width float32
}

type dropdownState struct{ open bool }

// Dropdown is DropdownMenu in the outlined look of the app's
// InputDecorationTheme: the label on the border, the choice inside, and a
// menu of the entries as wide as the field. It reports a change.
func Dropdown[T comparable](c *ui.Context, key any, o DropdownOpts, entries []DropdownEntry[T], selected *T) bool {
	s := cs()
	t := tt()
	changed := false
	label := ""
	for _, e := range entries {
		if e.Value == *selected {
			label = e.Label
		}
	}
	wrap := ui.Column(c.Key(key)).Shrink(0)
	if o.Width > 0 {
		wrap = wrap.Width(o.Width)
	} else {
		wrap = wrap.FillWidth()
	}
	st := ui.Local(wrap, "m3.dropdown", func() dropdownState { return dropdownState{} })
	wrap.Children(func() {
		m := ui.MenuBase(c, &st.open)
		tr := m.Trigger.FocusRing(false).FillWidth().Height(56).Row().AlignItems(ui.Center).Radius(design.CornerSM).Disabled(o.Disabled)
		if !o.Disabled {
			tr = tr.Cursor(ui.CursorPointer)
		}
		if o.Label != "" {
			tr = tr.Label(o.Label)
		}
		tr.Children(func() {
			fg := s.OnSurface
			if o.Disabled {
				fg = DisabledContent()
			}
			if o.LeadingIcon != 0 {
				ui.Row(c).Size(48, 48).Center().Shrink(0).Children(func() { Icon(c, o.LeadingIcon, 24, s.OnSurfaceVariant) })
			} else {
				HGap(c, 16)
			}
			Clamp(Text(c, label, t.BodyLarge.C(fg)), 1).Grow(1).Basis(0)
			ui.Row(c).Size(48, 48).Center().Shrink(0).Children(func() {
				ic := icons.ArrowDropDown
				if st.open {
					ic = icons.ArrowDropUp
				}
				Icon(c, ic, 24, s.OnSurfaceVariant)
			})
		})
		anim := design.T().Motion(200*time.Millisecond, design.FastOutSlowIn)
		border, width, labelC := s.Outline, float32(1), s.OnSurfaceVariant
		switch {
		case o.Disabled:
			border, labelC = design.WithOpacity(s.OnSurface, .12), DisabledContent()
		case st.open:
			border, width, labelC = s.Primary, 2, s.Primary
		case tr.Hovered():
			border = s.OnSurface
		}
		borderC := tr.AnimateColor("m3.dd.border", border, anim)
		bw := tr.AnimateTo("m3.dd.width", width, anim)
		lc := tr.AnimateColor("m3.dd.label", labelC, anim)
		floating := label != ""
		fl := tr.AnimateTo("m3.dd.float", map[bool]float32{true: 1, false: 0}[floating], anim)
		restX := float32(16)
		if o.LeadingIcon != 0 {
			restX = 48
		}
		lbl := o.Label
		tr.DrawOver(func(p *ui.Painter, r ui.Rect) {
			gs, ge := float32(0), float32(0)
			if lbl != "" {
				size := 16 - 4*fl
				sp := ui.Span{Text: lbl, Size: size * design.T().TextScale, Color: lc, LetterSpacing: .5 - .1*fl}
				lw, lh := p.MeasureText(r.W, sp)
				x := restX + (16-restX)*fl
				ry := (r.H - lh) / 2
				y := ry + (-lh/2-ry)*fl
				if fl > 0 {
					gs, ge = x-4, x+lw+4
				}
				if fl > 0 || !floating {
					p.RichText(r.X+x, r.Y+y, r.W, sp)
				}
			}
			in := bw / 2
			rr := ui.Rect{X: r.X + in, Y: r.Y + in, W: r.W - bw, H: r.H - bw}
			if ge <= gs {
				p.Stroke(rr, borderC, design.CornerSM-in, bw)
				return
			}
			p.StrokePath(outlineWithGap(rr, design.CornerSM-in, gs, ge), bw, borderC)
		})
		w := tr.Bounds().W
		m.Popup(func(panel ui.Element) {
			stylePanel(panel)
			panel = panel.AttachTo(tr, ui.AnchorBottomLeft, ui.AnchorTopLeft).MaxHeight(320)
			if w > 0 {
				panel.MinWidth(w).MaxWidth(w)
			}
			var items []MenuItem
			for _, e := range entries {
				e := e
				it := MenuItem{Label: e.Label, Icon: e.Icon, OnTap: func() {
					if *selected != e.Value {
						*selected = e.Value
						changed = true
					}
				}}
				if e.Value == *selected {
					it.Color = s.OnSurface
				}
				items = append(items, it)
			}
			ui.Scroll(c).FillWidth().Shrink(1).Children(func() { buildItems(c, m, items, nil) })
		})
		if o.Helper != "" {
			Text(c, o.Helper, t.BodySmall.C(s.OnSurfaceVariant)).Padding(4, 16, 0, 16)
		}
	})
	return changed
}
