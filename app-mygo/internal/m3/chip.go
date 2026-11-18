package m3

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
)

// ChipKind is which of Material's chips.
type ChipKind int

const (
	// FilterChip and ChoiceChip toggle; with the theme's showCheckmark
	// they show a check while selected.
	FilterChip ChipKind = iota
	ChoiceChip
	// ActionChip runs an action, its avatar in primary.
	ActionChip
	// InputChip is a removable value.
	InputChip
	// PlainChip is Chip: no action.
	PlainChip
)

// Chip is a chip with the app's ChipTheme: Corners.sm, an outlineVariant
// side, labelLarge.
type Chip struct {
	Kind     ChipKind
	Label    string
	Avatar   icons.Icon
	AvatarFn func(fg ui.Color)
	Selected bool
	Disabled bool
	// NoCheck is showCheckmark: false.
	NoCheck bool
	// OnDelete shows the delete icon; DeleteClicked tells of a press.
	Deletable bool
	// DeleteIcon and DeleteTooltip replace Icons.cancel and its "Delete"
	// (deleteIcon, deleteButtonTooltipMessage).
	DeleteIcon    icons.Icon
	DeleteSize    float32
	DeleteTooltip string
	Tooltip       string
	// BG and FG replace the colours.
	BG, FG ui.Color
	// Side replaces the border colour; NoSide drops it.
	Side   ui.Color
	NoSide bool
	// Compact is visualDensity: compact.
	Compact bool
	// LabelStyle replaces labelLarge.
	LabelStyle *design.TextStyle
	// Radius replaces Corners.sm.
	Radius float32
}

// ChipResult is a built chip: its element, and whether its delete icon
// was pressed.
type ChipResult struct {
	ui.Element
	Deleted bool
}

// Show builds the chip.
func (ch Chip) Show(c *ui.Context) ChipResult {
	s := cs()
	bg := ui.Transparent
	fg := s.OnSurfaceVariant
	avatarC := s.Primary
	switch ch.Kind {
	case ActionChip, PlainChip:
		fg = s.OnSurface
	case InputChip:
		fg, avatarC = s.OnSurfaceVariant, s.OnSurfaceVariant
	}
	side := s.OutlineVariant
	if ch.Selected && (ch.Kind == FilterChip || ch.Kind == ChoiceChip || ch.Kind == InputChip) {
		bg, fg, avatarC = s.SecondaryContainer, s.OnSecondaryContainer, s.OnSecondaryContainer
	}
	if ch.BG.A != 0 {
		bg = ch.BG
	}
	if ch.FG.A != 0 {
		fg, avatarC = ch.FG, ch.FG
	}
	if ch.Side.A != 0 {
		side = ch.Side
	}
	if ch.Disabled {
		fg, avatarC, side = DisabledContent(), DisabledContent(), DisabledContainer()
		if ch.Selected {
			bg = DisabledContainer()
		}
	}
	h := float32(32) + Density()*4
	if ch.Compact {
		h = 32 - 8
	}
	radius := ch.Radius
	if radius == 0 {
		radius = design.CornerSM
	}
	tap := ch.Kind != PlainChip
	var e ui.Element
	if tap {
		e = ui.ButtonBase(c).Disabled(ch.Disabled).DimDisabled(false).FocusRing(false)
		if !ch.Disabled {
			e = e.Cursor(ui.CursorPointer)
		}
	} else {
		e = ui.Box(c)
	}
	check := ch.Selected && !ch.NoCheck && (ch.Kind == FilterChip || ch.Kind == ChoiceChip)
	hasAvatar := ch.Avatar != 0 || ch.AvatarFn != nil || check
	padL, padR := float32(8), float32(8)
	e = e.Row().AlignItems(ui.Center).Height(h).Padding(0, padR, 0, padL).Radius(radius).Shrink(0).
		Transition(ui.ElementTransition{Duration: 200 * time.Millisecond, Colors: true, Width: true})
	if bg.A != 0 {
		e = e.Background(bg)
	}
	if !ch.NoSide {
		e = e.Border(1, side)
	}
	if ch.Tooltip != "" {
		e = Tip(c, e, ch.Tooltip)
	}
	if tap {
		layer := s.OnSurfaceVariant
		if ch.Selected {
			layer = s.OnSecondaryContainer
		}
		Ink(e, layer, radius)
	}
	style := tt().LabelLarge
	if ch.LabelStyle != nil {
		style = *ch.LabelStyle
	}
	var deleted bool
	e.Children(func() {
		if hasAvatar {
			ui.Row(c).Size(18, 18).Center().Shrink(0).Children(func() {
				switch {
				case check:
					Icon(c, icons.Check, 18, fg)
				case ch.AvatarFn != nil:
					ch.AvatarFn(avatarC)
				default:
					Icon(c, ch.Avatar, 18, avatarC)
				}
			})
		}
		Text(c, ch.Label, style.C(fg)).NoWrap().Padding(0, 8)
		if ch.Deletable {
			ic, size, tip := icons.Cancel, float32(18), ch.DeleteTooltip
			if ch.DeleteIcon != 0 {
				ic = ch.DeleteIcon
			}
			if ch.DeleteSize > 0 {
				size = ch.DeleteSize
			}
			if tip == "" {
				tip = l10n.CommonDelete
			}
			d := ui.ButtonBase(c).FocusRing(false).Size(18, 18).Center().Radius(9).Cursor(ui.CursorPointer).
				Label(tip).Children(func() { Icon(c, ic, size, fg) })
			Tip(c, d, tip)
			deleted = d.Clicked()
		}
	})
	return ChipResult{e, deleted}
}

// FilterChipOf is FilterChip(label:, selected:) returning whether it was
// toggled this frame.
func FilterChipOf(c *ui.Context, label string, selected bool) bool {
	return Chip{Kind: FilterChip, Label: label, Selected: selected}.Show(c).Clicked()
}

// ChoiceChipOf is ChoiceChip(label:, selected:).
func ChoiceChipOf(c *ui.Context, label string, selected bool) bool {
	return Chip{Kind: ChoiceChip, Label: label, Selected: selected}.Show(c).Clicked()
}

// ActionChipOf is ActionChip(avatar: Icon(ic), label:).
func ActionChipOf(c *ui.Context, ic icons.Icon, label string) bool {
	return Chip{Kind: ActionChip, Avatar: ic, Label: label}.Show(c).Clicked()
}
