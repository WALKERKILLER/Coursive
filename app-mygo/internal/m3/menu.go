package m3

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
)

// MenuItem is an item of a menu: MenuItemButton, PopupMenuItem,
// CheckedPopupMenuItem, a divider, or a submenu.
type MenuItem struct {
	Icon     icons.Icon
	Label    string
	Subtitle string
	// Trailing is text after the label, as a shortcut.
	Trailing     string
	TrailingIcon icons.Icon
	Disabled     bool
	// Danger colours the item in the error colour.
	Danger bool
	// Checked shows a check before the label (CheckedPopupMenuItem).
	Checked *bool
	// Divider makes the item a divider (PopupMenuDivider, Divider()).
	Divider bool
	// Header is a label that takes no clicks.
	Header bool
	// Sub makes the item open a submenu (SubmenuButton).
	Sub []MenuItem
	// Color replaces the label's colour.
	Color ui.Color
	// Custom builds the item's content in place of the label.
	Custom func(c *ui.Context)
	// IconSize replaces the 24 DIP icons.
	IconSize float32
	// LeadingFn builds the leading widget in place of Icon, as a logo.
	LeadingFn func(c *ui.Context)
	// Panel builds content that is not an item, as a slider or a summary
	// among the items (a Padding child of menuChildren).
	Panel func(c *ui.Context)
	// KeepOpen leaves the menu open after a click.
	KeepOpen bool
	OnTap    func()
}

// MenuStyle places a menu.
type MenuStyle int

const (
	// Below is MenuAnchor's placement: below the trigger, from its start.
	Below MenuStyle = iota
	// Over is PopupMenuButton's: over the trigger, from its end.
	Over
	// BelowEnd is below the trigger, aligned to its end.
	BelowEnd
	// Popup, added to a placement, opens and closes the menu as
	// PopupMenuButton and showMenu do; MenuAnchor's menus, without it,
	// come and go at once.
	Popup MenuStyle = 1 << 8
)

type menuState struct{ open, shown bool }

// Menu builds a menu button: trigger styles the button (the base's
// trigger, which takes the clicks) and items fill the menu, with the app's
// menu theme: surfaceContainer, Corners.md, 8 DIPs above and below,
// elevation 3.
func Menu(c *ui.Context, key any, style MenuStyle, trigger func(t ui.Element, open bool), items func() []MenuItem) ui.Element {
	wrap := ui.Box(c.Key(key)).Shrink(0)
	st := ui.Local(wrap, "m3.menu", func() menuState { return menuState{} })
	wrap.Children(func() {
		m := ui.MenuBase(c, &st.open)
		if !st.open {
			st.shown = false
		}
		t := m.Trigger.FocusRing(false)
		trigger(t, st.open)
		m.Popup(func(panel ui.Element) {
			list := items()
			stylePanel(panel)
			var fade func(int) float32
			if style&Popup != 0 {
				fade = popupMotion(panel, &st.shown, len(list))
			}
			switch style &^ Popup {
			case Over:
				panel.AttachTo(t, ui.AnchorTopRight, ui.AnchorTopRight)
			case BelowEnd:
				panel.AttachTo(t, ui.AnchorBottomRight, ui.AnchorTopRight)
			default:
				panel.AttachTo(t, ui.AnchorBottomLeft, ui.AnchorTopLeft)
			}
			buildItems(c, m, list, fade)
		})
	})
	return wrap
}

func stylePanel(panel ui.Element) {
	panel = panel.MinWidth(112).MaxWidth(320).PaddingY(8).Radius(design.CornerMD).
		Background(cs().SurfaceContainer)
	Elevation(panel, 3)
}

const popupDuration = 300 * time.Millisecond

// popupMotion animates a menu's panel as _PopupMenu does over 300 ms: the
// panel fades in over the first third while it unfolds from its top end,
// its width over the first unit of the time and its height over n units,
// a unit being 1/(n+1.5), and its items fade in one after another, each
// over 1.5 units. It returns each item's opacity, or nil once open.
func popupMotion(panel ui.Element, shown *bool, n int) func(int) float32 {
	if !*shown {
		panel.Jump("m3.popup", 0)
		*shown = true
	}
	panel.Transition(ui.ElementTransition{Duration: popupDuration, ExitEase: popupClose, Exit: &ui.Motion{Opacity: 0}})
	t := panel.AnimateTo("m3.popup", 1, design.T().Motion(popupDuration, ui.Linear))
	if t >= 1 {
		return nil
	}
	unit := 1 / (float32(n) + 1.5)
	w, h := clamp01(t/unit), clamp01(t/(unit*float32(max(n, 1))))
	// The clip reaches past the edges by about the shadow, as Material's
	// shadow follows its box while it unfolds.
	const reach = 16
	r := panel.LayoutBounds()
	panel.Opacity(clamp01(3*t)).ClipInset(-reach, -reach, (1-h)*r.H-reach*h, (1-w)*r.W-reach*w, design.CornerMD)
	return func(i int) float32 { return clamp01((t - float32(i+1)*unit) / (1.5 * unit)) }
}

// popupClose is how far a closing popup menu has gone at t of its Exit.
// The route runs back through Interval(0, 2/3): the panel folds up over
// the last 45% of the 300 ms and fades over its last 67. The Exit's copy
// takes no clip, so it fades as much as the folding panel shows less.
func popupClose(t float32) float32 { return clamp01((t - .6) / .4) }

func clamp01(v float32) float32 { return min(max(v, 0), 1) }

func buildItems(c *ui.Context, m ui.MenuParts, items []MenuItem, fade func(int) float32) {
	s := cs()
	for i, it := range items {
		it := it
		op := float32(1)
		if fade != nil {
			op = fade(i)
		}
		if it.Divider {
			ui.Box(c.Key(i)).Height(1).Margin(8, 0).Background(s.OutlineVariant).Shrink(0).Opacity(op)
			continue
		}
		if it.Header {
			Text(c.Key(i), it.Label, tt().LabelLarge.C(s.OnSurfaceVariant)).Padding(8, 12).Opacity(op)
			continue
		}
		if it.Panel != nil {
			ui.Column(c.Key(i)).Shrink(0).Opacity(op).Children(func() { it.Panel(c) })
			continue
		}
		var e ui.Element
		switch {
		case len(it.Sub) > 0:
			e = m.SubmenuItem()
		case it.Checked != nil:
			e = m.CheckItem(*it.Checked)
		default:
			e = m.Item()
		}
		if it.KeepOpen {
			e = e.KeepOpen()
		}
		e = e.Disabled(it.Disabled).DimDisabled(false).Row().AlignItems(ui.Center).
			MinHeight(48+Density()*4).Padding(0, 12).Gap(12)
		if op < 1 {
			e = e.Opacity(op)
		}
		fg, ic := s.OnSurface, s.OnSurfaceVariant
		if it.Danger {
			fg, ic = s.Error, s.Error
		}
		if it.Color.A != 0 {
			fg = it.Color
		}
		if it.Disabled {
			fg, ic = DisabledContent(), DisabledContent()
		}
		if !it.Disabled {
			e = e.Cursor(ui.CursorPointer)
			if e.Highlighted() {
				e = e.Background(design.WithOpacity(s.OnSurface, .08))
			}
		}
		isz := it.IconSize
		if isz == 0 {
			isz = 24
		}
		e.Children(func() {
			if it.Checked != nil {
				if *it.Checked {
					Icon(c, icons.Check, 24, ic)
				} else {
					HGap(c, 24)
				}
			}
			if it.LeadingFn != nil {
				it.LeadingFn(c)
			} else if it.Icon != 0 {
				Icon(c, it.Icon, isz, ic)
			}
			if it.Custom != nil {
				it.Custom(c)
			} else {
				ui.Column(c).Grow(1).Children(func() {
					Text(c, it.Label, tt().LabelLarge.C(fg)).NoWrap()
					if it.Subtitle != "" {
						Text(c, it.Subtitle, tt().BodySmall.C(s.OnSurfaceVariant))
					}
				})
			}
			if it.Trailing != "" {
				Text(c, it.Trailing, tt().LabelLarge.C(s.OnSurfaceVariant)).NoWrap()
			}
			if it.TrailingIcon != 0 {
				Icon(c, it.TrailingIcon, isz, ic)
			}
			if len(it.Sub) > 0 {
				Icon(c, icons.ArrowRight, 24, ic)
			}
		})
		if len(it.Sub) > 0 {
			m.Submenu(e, func(panel ui.Element) {
				stylePanel(panel)
				buildItems(c, m, it.Sub, nil)
			})
			continue
		}
		if e.Clicked() && it.OnTap != nil {
			it.OnTap()
		}
	}
}

// MenuButton is an icon button opening a menu: PopupMenuButton with its
// default more_vert icon, or MenuAnchor around an IconButton.
func MenuButton(c *ui.Context, key any, ic icons.Icon, tooltip string, style MenuStyle, items func() []MenuItem) ui.Element {
	if ic == 0 {
		ic = icons.MoreVert
	}
	return Menu(c, key, style, func(t ui.Element, open bool) {
		t.Size(40+Density()*4, 40+Density()*4).Center().Radius(design.CornerFull).Cursor(ui.CursorPointer)
		if tooltip != "" {
			// A tooltip is its semantic label too.
			t.Label(tooltip)
			if !open {
				Tip(c, t, tooltip)
			}
		}
		Ink(t, cs().OnSurfaceVariant, design.CornerFull)
		t.Children(func() { Icon(c, ic, 24, cs().OnSurfaceVariant) })
	}, items)
}

type ctxMenu struct{ open, shown bool }

// ContextMenu gives target a menu on a right click or a long press, with
// the app's menu look.
func ContextMenu(c *ui.Context, target ui.Element, items func() []MenuItem) ui.Element {
	st := ui.Local(target, "m3.ctx", func() ctxMenu { return ctxMenu{} })
	m := ui.ContextMenuBase(c, target, &st.open)
	if !st.open {
		st.shown = false
	}
	m.Popup(func(panel ui.Element) {
		// The menu is showMenu's.
		list := items()
		stylePanel(panel)
		buildItems(c, m, list, popupMotion(panel, &st.shown, len(list)))
	})
	return target
}
