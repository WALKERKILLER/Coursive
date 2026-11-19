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
	selectable := ch.Kind == FilterChip || ch.Kind == ChoiceChip || ch.Kind == InputChip
	selBG := bg
	if selectable {
		selBG = s.SecondaryContainer
	}
	if ch.Selected && selectable {
		fg, avatarC = s.OnSecondaryContainer, s.OnSecondaryContainer
	}
	if ch.BG.A != 0 {
		bg, selBG = ch.BG, ch.BG
	}
	if ch.FG.A != 0 {
		fg, avatarC = ch.FG, ch.FG
	}
	if ch.Side.A != 0 {
		side = ch.Side
	}
	if ch.Disabled {
		fg, avatarC, side = DisabledContent(), DisabledContent(), DisabledContainer()
		selBG = DisabledContainer()
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
	checks := !ch.NoCheck && (ch.Kind == FilterChip || ch.Kind == ChoiceChip)
	avatar := ch.Avatar != 0 || ch.AvatarFn != nil
	padL, padR := float32(8), float32(8)
	e = e.Row().AlignItems(ui.Center).Height(h).Padding(0, padR, 0, padL).Radius(radius).Shrink(0)
	// RawChip's controllers: the background goes to the selected colour
	// over 195 ms, the room for a check mark opens over 150 ms and the
	// check draws itself over the last 150 ms of the 195; going back, the
	// room closes over the 150 ms through Interval(.49, 1) and the check
	// fades through Interval(.74, 1) of the 195. The label changes at once.
	sel := controller(e, "m3.chip.select", ch.Selected, chipSelect)
	room := controller(e, "m3.chip.room", avatar || checks && ch.Selected, chipDrawer)
	tick := sel.curved(interval(1-chipCheck, 1, design.FastOutSlowIn), interval(1-chipCheckBack, 1, design.FastOutSlowIn))
	if bg := design.Lerp(bg, selBG, sel.curved(design.FastOutSlowIn, nil)); bg.A != 0 {
		e = e.Background(bg)
	}
	if !ch.NoSide {
		e = e.Border(1, side)
	}
	if ch.Tooltip != "" {
		e = Tip(c, e, ch.Tooltip)
	}
	if tap {
		// RawChip's InkWell takes the theme's colours, not a state layer.
		InkWell(e, radius)
	}
	style := tt().LabelLarge
	if ch.LabelStyle != nil {
		style = *ch.LabelStyle
	}
	var deleted bool
	e.Children(func() {
		// The avatar and the delete icon each take a square as tall as the
		// content: the label's line, 16 DIPs at least (contentSize).
		box := min(max(16, style.Size*style.Height), h)
		square := func(child func()) {
			ui.Row(c).Size(box, box).Shrink(0).Center().Children(child)
		}
		if w := box * room.curved(design.FastOutSlowIn, interval(1-chipDrawerBack, 1, design.FastOutSlowIn)); w > 0 {
			slot := ui.Row(c).Size(w, box).Shrink(0).Clip()
			switch {
			case avatar && ch.Selected && checks:
				slot.Children(func() { square(func() { Icon(c, icons.Check, 18, fg) }) })
			case ch.AvatarFn != nil:
				slot.Children(func() { square(func() { ch.AvatarFn(avatarC) }) })
			case avatar:
				slot.Children(func() { square(func() { Icon(c, ch.Avatar, 18, avatarC) }) })
			case checks && tick > 0:
				back := sel.status == ctlReverse
				slot.Draw(func(p *ui.Painter, r ui.Rect) {
					paintChipCheck(p, r.X+box*.125, r.Y+box*.125, box*.75, 2*box/24, fg, tick, back)
				})
			}
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
			d := ui.ButtonBase(c).FocusRing(false).Size(box, box).Center().Radius(box / 2).Cursor(ui.CursorPointer).
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

// RawChip's durations, and the parts of the select animation the check
// mark takes each way, and the room for it going back.
const (
	chipSelect     = 195 * time.Millisecond
	chipDrawer     = 150 * time.Millisecond
	chipCheck      = float32(150) / 195
	chipCheckBack  = float32(50) / 195
	chipDrawerBack = float32(100) / 195
)

// flipped is curve.flipped: an Exit going as a reversed CurvedAnimation
// comes back along curve.
func flipped(curve ui.Easing) ui.Easing { return func(t float32) float32 { return 1 - curve(1-t) } }

// interval is Interval(a, b, curve:).
func interval(a, b float32, curve ui.Easing) ui.Easing {
	return func(t float32) float32 { return curve(clamp01((t - a) / (b - a))) }
}

// paintChipCheck is _paintCheck: the check mark's two strokes drawn from
// the short side to the long side as t goes to 1 and, going back, whole,
// fading as t does.
func paintChipCheck(p *ui.Painter, x, y, size, stroke float32, col ui.Color, t float32, back bool) {
	if back {
		col, t = design.WithOpacity(col, t), 1
	}
	sx, sy := x+size*.15, y+size*.45
	mx, my := x+size*.4, y+size*.7
	ex, ey := x+size*.85, y+size*.25
	path := (&ui.Path{}).MoveTo(sx, sy)
	if t < .5 {
		path.LineTo(lerp(sx, mx, t*2), lerp(sy, my, t*2))
	} else {
		path.LineTo(mx, my).LineTo(lerp(mx, ex, (t-.5)*2), lerp(my, ey, (t-.5)*2))
	}
	p.StrokePath(path.Cap(ui.ButtCap).Join(ui.MiterJoin), stroke, col)
}
