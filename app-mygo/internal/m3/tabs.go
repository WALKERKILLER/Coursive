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

// tabScroll is kTabScrollDuration, how long a tab change takes.
const tabScroll = 300 * time.Millisecond

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
			row := ui.Row(c).FillHeight()
			// The labels take their colours from the controller's animation
			// as _TabStyle does: the new tab and the one before it cross-fade
			// as far as the animated index has gone between them, over
			// 300 ms as Curves.ease, and the tabs passed on the way stay put.
			ch := ui.Local(row, "m3.tabs.change", func() tabChange { return tabChange{*selected, *selected} })
			if ch.cur != *selected {
				ch.prev, ch.cur = ch.cur, *selected
			}
			pos := row.AnimateTo("m3.tabs.pos", float32(*selected), design.T().Motion(tabScroll, design.Ease))
			moved := float32(1)
			if span := abs32(float32(ch.cur - ch.prev)); span > 0 {
				moved = clamp01(1 - abs32(pos-float32(ch.cur))/span)
			}
			row.Children(func() {
				for i, t := range tabs {
					on := i == *selected
					var near float32
					switch i {
					case ch.cur:
						near = moved
					case ch.prev:
						near = 1 - moved
					}
					selFG := s.Primary
					if o.Secondary {
						selFG = s.OnSurface
					}
					if o.Pill {
						selFG = s.OnSecondaryContainer
					}
					fg := design.Lerp(s.OnSurfaceVariant, selFG, near)
					if o.Pill {
						b := ui.ButtonBase(c.Key(i)).FocusRing(false).Row().AlignItems(ui.Center).
							FillHeight().Padding(0, 14).Cursor(ui.CursorPointer).Checked(on).Role(ui.RoleTab)
						// The side panel's overlayColor sets only the hover, 6%
						// onSurface over the whole tab in a stadium; the press
						// and focus keep the theme's InkWell colours.
						pillInk(b, s.OnSurface)
						b.Children(func() {
							if on {
								ui.Box(c).Absolute().Top(6).Bottom(6).Left(0).Right(0).Radius(design.CornerFull).Background(s.SecondaryContainer).
									Transition(ui.ElementTransition{Duration: tabScroll, Ease: design.Ease,
										Position: true, Size: true, Match: selected, Fly: true})
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
					// _TabsPrimaryDefaultsM3.overlayColor: hover and focus in
					// primary on the selected tab and onSurface on the rest, the
					// press and its splash in primary on all. Secondary tabs
					// take onSurface throughout.
					over, pressC := s.OnSurface, s.Primary
					if o.Secondary {
						pressC = s.OnSurface
					} else if on {
						over = s.Primary
					}
					press := design.WithOpacity(pressC, .1)
					InkColors(b, 0, design.WithOpacity(over, .08), design.WithOpacity(over, .1), press, press)
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
							ind.Background(col).Transition(ui.ElementTransition{Duration: tabScroll,
								Ease: design.Ease, Position: true, Size: true, Match: selected, Fly: true})
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

func pillInk(b ui.Element, on ui.Color) {
	focus, press, splash := ui.RGBA(0, 0, 0, .12), ui.RGBA(0xBC, 0xBC, 0xBC, 0x66/255.0), ui.RGBA(0xC8, 0xC8, 0xC8, 0x66/255.0)
	if design.T().Dark {
		focus, press = ui.RGBA(255, 255, 255, .12), ui.RGBA(0xCC, 0xCC, 0xCC, 0x40/255.0)
		splash = press
	}
	InkColors(b, design.CornerFull, design.WithOpacity(on, .06), focus, press, splash)
}

type tabChange struct{ prev, cur int }

type tabView struct{ last int }

// TabView is TabBarView: the pages of the tabs side by side in e, which it
// clips, sliding to the selected one over kTabScrollDuration along
// Curves.ease as TabBar's indicator moves. A change of more than one tab
// starts from the page next to the new one, as TabBarView does, so only
// two pages show on the way. page builds page i.
func TabView(c *ui.Context, e ui.Element, selected int, page func(i int)) ui.Element {
	st := ui.Local(e, "tabview", func() tabView { return tabView{last: selected} })
	if d := selected - st.last; d > 1 || d < -1 {
		from := selected - 1
		if d < 0 {
			from = selected + 1
		}
		e.Jump("page", float32(from))
	}
	st.last = selected
	off := e.AnimateTo("page", float32(selected), design.T().Motion(tabScroll, design.Ease))
	return e.Clip().Children(func() {
		for i := int(off) - 1; i <= int(off)+1; i++ {
			pos := float32(i) - off
			if i < 0 || pos <= -1 || pos >= 1 {
				continue
			}
			ui.Column(c.Key(i)).Absolute().Top(0).Bottom(0).Left(0).Right(0).TranslatePercent(pos*100, 0).Children(func() { page(i) })
		}
	})
}
