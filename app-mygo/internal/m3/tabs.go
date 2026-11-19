package m3

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
)

// Tab is a Tab of a TabBar.
type Tab struct {
	Label string
	Icon  icons.Icon
}

// TabOpts are TabBar's options.
type TabOpts struct {
	// Scrollable is isScrollable: tabs as wide as their labels, from the
	// start.
	Scrollable bool
	// Secondary is TabBar.secondary: an underline across the tab.
	Secondary bool
	// NoDivider drops the outlineVariant line below (dividerColor:
	// transparent).
	NoDivider bool
	// Pill is the player panel's look: 44 DIP tabs in labelLarge, the
	// selected one on a secondaryContainer pill 6 DIPs in from the top and
	// bottom, without a divider.
	Pill bool
}

// TabBar is Material 3's primary TabBar: labels in titleSmall, primary
// when selected, a 3 DIP indicator as wide as the label with rounded top
// corners, and a divider below. It returns whether the tab changed.
func TabBar(c *ui.Context, tabs []Tab, selected *int, o TabOpts) bool {
	s := cs()
	changed := false
	h := float32(48)
	if o.Pill {
		h, o.NoDivider = 44, true
	}
	for _, t := range tabs {
		if t.Icon != 0 && t.Label != "" {
			h = 64
		}
	}
	outer := ui.Column(c).FillWidth().Shrink(0)
	outer.Children(func() {
		var row ui.Element
		if o.Scrollable {
			row = ui.ScrollHorizontal(c).Scrollbars(false).Height(h).FillWidth()
		} else {
			row = ui.Row(c).Height(h).FillWidth()
		}
		row.Children(func() {
			ui.Row(c).FillHeight().Children(func() {
				for i, t := range tabs {
					on := i == *selected
					fg := s.OnSurfaceVariant
					if on {
						fg = s.Primary
						if o.Secondary {
							fg = s.OnSurface
						}
					}
					if o.Pill {
						if on {
							fg = s.OnSecondaryContainer
						}
						b := ui.ButtonBase(c.Key(i)).FocusRing(false).Row().AlignItems(ui.Center).
							FillHeight().Padding(0, 14).Cursor(ui.CursorPointer).Checked(on).Role(ui.RoleTab)
						b.Children(func() {
							if on {
								ui.Box(c).Absolute().Top(6).Bottom(6).Left(0).Right(0).Radius(design.CornerFull).Background(s.SecondaryContainer).
									Transition(ui.ElementTransition{Duration: 300 * time.Millisecond, Ease: design.FastOutSlowIn,
										Position: true, Size: true, Match: selected, Fly: true})
							}
							hover := ui.Box(c).Absolute().Top(6).Bottom(6).Left(0).Right(0).PassThrough()
							if b.Hovered() {
								hover.Radius(design.CornerFull).Background(design.WithOpacity(s.OnSurface, .06))
							}
							Text(c, t.Label, tt().LabelLarge.C(fg)).NoWrap()
						})
						if b.Clicked() && !on {
							*selected = i
							changed = true
						}
						continue
					}
					b := ui.ButtonBase(c.Key(i)).FocusRing(false).Column().AlignItems(ui.Center).Justify(ui.Center).
						FillHeight().Padding(0, 16).Cursor(ui.CursorPointer).Checked(on).Role(ui.RoleTab)
					if !o.Scrollable {
						b = b.Grow(1).Basis(0)
					}
					Ink(b, fg, 0)
					b.Children(func() {
						lab := ui.Column(c).AlignItems(ui.Center).Gap(2).Children(func() {
							if t.Icon != 0 {
								Icon(c, t.Icon, 24, fg)
							}
							if t.Label != "" {
								Text(c, t.Label, tt().TitleSmall.C(fg)).NoWrap()
							}
						})
						_ = lab
						ind := ui.Box(c).Height(3).Radius(3, 3, 0, 0).Absolute().Bottom(0)
						if on {
							col := s.Primary
							if o.Secondary {
								ind = ind.Height(2).Radius(0).Left(0).Right(0)
							} else {
								ind = ind.Left(16).Right(16)
							}
							ind.Background(col).Transition(ui.ElementTransition{Duration: 300 * time.Millisecond,
								Ease: design.FastOutSlowIn, Position: true, Size: true, Match: selected, Fly: true})
						}
					})
					if b.Clicked() && !on {
						*selected = i
						changed = true
					}
				}
			})
		})
		if !o.NoDivider {
			Divider(c)
		}
	})
	return changed
}
