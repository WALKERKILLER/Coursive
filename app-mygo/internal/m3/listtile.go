package m3

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
)

// Tile is a ListTile with the app's ListTileTheme: Corners.md, 16 DIPs of
// horizontal padding, icons in onSurfaceVariant.
type Tile struct {
	Leading      func()
	LeadingIcon  icons.Icon
	Title        string
	TitleFn      func()
	Subtitle     string
	SubtitleFn   func()
	Trailing     func()
	TrailingIcon icons.Icon
	TrailingText string
	// Tap makes the tile take clicks (onTap set).
	Tap      bool
	Selected bool
	Disabled bool
	// Dense is dense: true; Compact is visualDensity: VisualDensity.compact.
	Dense, Compact bool
	ThreeLine      bool
	// PadX replaces the horizontal content padding when Pad is set.
	Pad        bool
	PadX, PadY float32
	TileColor  ui.Color
	// SubtitleLines caps the subtitle's lines when not zero.
	SubtitleLines int
	TitleLines    int
	TitleStyle    *design.TextStyle
	// Radius replaces Corners.md; NoRadius makes it square.
	Radius   float32
	NoRadius bool
	Tooltip  string
	// IconColor replaces the leading and trailing icon colour.
	IconColor ui.Color
	// TextColor replaces the title colour.
	TextColor ui.Color
	// MinLeading is minLeadingWidth (24 by default... 40 for M2).
	MinLeading float32
	// Gap is horizontalTitleGap.
	Gap float32
}

// Show builds the tile; read Clicked from it when Tap is set.
func (t Tile) Show(c *ui.Context) ui.Element {
	s := cs()
	two := t.Subtitle != "" || t.SubtitleFn != nil
	h := float32(56)
	switch {
	case t.ThreeLine:
		h = 88
	case two:
		h = 72
	}
	if t.Dense {
		switch {
		case t.ThreeLine:
			h = 76
		case two:
			h = 64
		default:
			h = 48
		}
	}
	d := Density() * 4
	if t.Compact {
		d = -8
	}
	h += d
	padX, padY := float32(16), float32(8)
	if t.Pad {
		padX, padY = t.PadX, t.PadY
	}
	radius := t.Radius
	if radius == 0 {
		radius = design.CornerMD
	}
	if t.NoRadius {
		radius = 0
	}
	title, sub, ic := s.OnSurface, s.OnSurfaceVariant, s.OnSurfaceVariant
	if t.IconColor.A != 0 {
		ic = t.IconColor
	}
	if t.TextColor.A != 0 {
		title = t.TextColor
	}
	if t.Selected {
		title, ic = s.Primary, s.Primary
	}
	if t.Disabled {
		title, sub, ic = DisabledContent(), DisabledContent(), DisabledContent()
	}
	var e ui.Element
	if t.Tap {
		e = ui.ButtonBase(c).Disabled(t.Disabled).DimDisabled(false).FocusRing(false)
		if !t.Disabled {
			e = e.Cursor(ui.CursorPointer)
		}
	} else {
		e = ui.Box(c)
	}
	e = e.Row().AlignItems(ui.Center).MinHeight(h).Padding(padY, padX).Radius(radius).Gap(0).Shrink(0)
	if t.ThreeLine {
		e = e.AlignItems(ui.Start)
	}
	if t.TileColor.A != 0 {
		e = e.Background(t.TileColor)
	}
	if t.Tooltip != "" {
		e = Tip(c, e, t.Tooltip)
	}
	if t.Tap {
		Ink(e, s.OnSurface, radius)
	}
	gap := t.Gap
	if gap == 0 {
		gap = 16
	}
	minLeading := t.MinLeading
	if minLeading == 0 {
		minLeading = 24
	}
	e.Children(func() {
		if t.Leading != nil || t.LeadingIcon != 0 {
			ui.Row(c).MinWidth(minLeading).AlignItems(ui.Center).Shrink(0).MarginEnd(gap).Children(func() {
				if t.Leading != nil {
					t.Leading()
				} else {
					Icon(c, t.LeadingIcon, 24, ic)
				}
			})
		}
		ui.Column(c).Grow(1).Basis(0).Justify(ui.Center).Children(func() {
			if t.TitleFn != nil {
				t.TitleFn()
			} else if t.Title != "" {
				st := tt().BodyLarge
				if t.TitleStyle != nil {
					st = *t.TitleStyle
				}
				st = st.C(title)
				if t.TitleStyle != nil && t.TitleStyle.Color.A != 0 && !t.Selected && !t.Disabled {
					st.Color = t.TitleStyle.Color
				}
				el := Text(c, t.Title, st)
				if t.TitleLines > 0 {
					Clamp(el, t.TitleLines)
				}
			}
			if t.SubtitleFn != nil {
				t.SubtitleFn()
			} else if t.Subtitle != "" {
				el := Text(c, t.Subtitle, tt().BodyMedium.C(sub))
				if t.SubtitleLines > 0 {
					Clamp(el, t.SubtitleLines)
				} else if !t.ThreeLine {
					// A two-line tile's subtitle wraps freely unless capped.
				}
			}
		})
		if t.Trailing != nil || t.TrailingIcon != 0 || t.TrailingText != "" {
			ui.Row(c).AlignItems(ui.Center).Shrink(0).MarginStart(gap).Children(func() {
				switch {
				case t.Trailing != nil:
					t.Trailing()
				case t.TrailingIcon != 0:
					Icon(c, t.TrailingIcon, 24, ic)
				default:
					Text(c, t.TrailingText, tt().LabelSmall.C(s.OnSurfaceVariant))
				}
			})
		}
	})
	return e
}

// SwitchTile is SwitchListTile: the whole tile toggles the switch.
func SwitchTile(c *ui.Context, t Tile, on *bool) bool {
	t.Tap = true
	var sw ui.Element
	t.Trailing = func() { sw = Switch(c, on, t.Disabled) }
	e := t.Show(c)
	changed := sw.Changed()
	if e.Clicked() && !t.Disabled {
		*on = !*on
		changed = true
	}
	return changed
}

// CheckTile is CheckboxListTile, its box trailing.
func CheckTile(c *ui.Context, t Tile, checked *bool) bool {
	t.Tap = true
	var box ui.Element
	t.Trailing = func() { box = Checkbox(c, checked, t.Disabled) }
	e := t.Show(c)
	changed := box.Changed()
	if e.Clicked() && !t.Disabled {
		*checked = !*checked
		changed = true
	}
	return changed
}

// CheckTileLeading is CheckboxListTile(controlAffinity: leading).
func CheckTileLeading(c *ui.Context, t Tile, checked *bool) bool {
	t.Tap = true
	var box ui.Element
	t.Leading = func() { box = Checkbox(c, checked, t.Disabled) }
	e := t.Show(c)
	changed := box.Changed()
	if e.Clicked() && !t.Disabled {
		*checked = !*checked
		changed = true
	}
	return changed
}

// RadioTile is RadioListTile, its radio leading.
func RadioTile[T comparable](c *ui.Context, t Tile, selected *T, value T) bool {
	t.Tap = true
	var r ui.Element
	t.Leading = func() { r = Radio(c, selected, value, t.Disabled) }
	e := t.Show(c)
	changed := r.Changed()
	if e.Clicked() && !t.Disabled && *selected != value {
		*selected = value
		changed = true
	}
	return changed
}

// Divider is Divider() with the theme: 1 DIP of outlineVariant taking 1 DIP.
func Divider(c *ui.Context) ui.Element {
	return ui.Box(c).Height(1).FillWidth().Background(cs().OutlineVariant).Shrink(0)
}

// DividerIndent is Divider(indent:, endIndent:, height:).
func DividerIndent(c *ui.Context, height, indent, end float32) ui.Element {
	if height < 1 {
		height = 1
	}
	return ui.Column(c).Height(height).Justify(ui.Center).Padding(0, end, 0, indent).Shrink(0).Children(func() {
		ui.Box(c).Height(1).FillWidth().Background(cs().OutlineVariant)
	})
}

// VDivider is VerticalDivider().
func VDivider(c *ui.Context) ui.Element {
	return ui.Box(c).Width(1).FillHeight().Background(cs().OutlineVariant).Shrink(0)
}

type expansion struct{ open, init bool }

// Expansion is ExpansionTile: a tile whose click shows and hides children,
// with the expand_more arrow turning.
type Expansion struct {
	Tile
	Initially bool
	// Open, when not nil, holds the state outside.
	Open     *bool
	Children func()
	// ChildPad is childrenPadding.
	ChildPad [4]float32
}

// Show builds the tile and, while open, its children.
func (x Expansion) Show(c *ui.Context, key any) ui.Element {
	wrap := ui.Column(c.Key(key)).Shrink(0)
	st := ui.Local(wrap, "m3.exp", func() expansion { return expansion{open: x.Initially} })
	open := &st.open
	if x.Open != nil {
		open = x.Open
	}
	wrap.Children(func() {
		t := x.Tile
		t.Tap = true
		turn := wrap.AnimateTo("m3.exp.turn", map[bool]float32{false: 0, true: 180}[*open], design.T().Motion(200*time.Millisecond, design.EaseInOut))
		if t.Trailing == nil {
			t.Trailing = func() {
				col := cs().OnSurfaceVariant
				if *open {
					col = cs().Primary
				}
				Icon(c, icons.ExpandMore, 24, col).Rotate(turn)
			}
		}
		if *open {
			t.TextColor = cs().Primary
			t.IconColor = cs().Primary
		}
		t.NoRadius = true
		if t.Show(c).Clicked() {
			*open = !*open
		}
		if *open && x.Children != nil {
			p := x.ChildPad
			ui.Column(c).Padding(p[0], p[1], p[2], p[3]).Transition(ui.ElementTransition{Duration: 200 * time.Millisecond,
				Enter: &ui.Motion{Collapse: true, Opacity: 0}, Exit: &ui.Motion{Collapse: true, Opacity: 0}}).Children(x.Children)
		}
	})
	return wrap
}
