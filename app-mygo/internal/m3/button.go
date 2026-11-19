package m3

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
)

// ButtonKind is which of Material's common buttons.
type ButtonKind int

const (
	// Filled is FilledButton.
	Filled ButtonKind = iota
	// Tonal is FilledButton.tonal.
	Tonal
	// Outlined is OutlinedButton.
	Outlined
	// TextKind is TextButton.
	TextKind
	// Elevated is ElevatedButton.
	Elevated
)

// Button is one of Material's common buttons. Leave Icon zero for none;
// Disabled is onPressed: null.
type Button struct {
	Kind     ButtonKind
	Label    string
	Icon     icons.Icon
	Disabled bool
	// IconAtEnd puts the icon after the label (iconAlignment: end).
	IconAtEnd bool
	// BG and FG replace the container and content colours (styleFrom).
	BG, FG ui.Color
	// Child replaces the label, as a button holding a spinner.
	Child func(fg ui.Color)
	// Padding replaces the horizontal padding when not zero.
	PadX float32
	// MinWidth replaces the 64 DIP minimum width when not zero.
	MinWidth float32
	// Height replaces the 40 DIP minimum height when not zero.
	Height  float32
	Tooltip string
	// Style replaces labelLarge.
	Style *design.TextStyle
	// Busy shows a spinner in place of the icon, as an action that runs.
	Busy bool
}

// colors are the container and content colours for the state.
func (b Button) colors() (bg, fg ui.Color) {
	s := cs()
	switch b.Kind {
	case Filled:
		bg, fg = s.Primary, s.OnPrimary
	case Tonal:
		bg, fg = s.SecondaryContainer, s.OnSecondaryContainer
	case Elevated:
		bg, fg = s.SurfaceContainerLow, s.Primary
	default:
		bg, fg = ui.Transparent, s.Primary
	}
	if b.BG.A != 0 {
		bg = b.BG
	}
	if b.FG.A != 0 {
		fg = b.FG
	}
	if b.Disabled {
		fg = DisabledContent()
		if b.Kind == Filled || b.Kind == Tonal || b.Kind == Elevated {
			bg = DisabledContainer()
		}
	}
	return bg, fg
}

// Show builds the button and returns it, to read Clicked from or style.
func (b Button) Show(c *ui.Context) ui.Element {
	bg, fg := b.colors()
	d := Density() * 4
	padX := float32(24)
	var padStart, padEnd float32
	padY := float32(10)
	if b.Kind == TextKind {
		padX, padY = 12, 8
		if b.Icon != 0 || b.Busy {
			padStart, padEnd = 12, 16
		}
	}
	if b.PadX != 0 {
		padX = b.PadX
		padStart, padEnd = 0, 0
	}
	if padStart == 0 {
		padStart, padEnd = padX, padX
	}
	padY = max(0, padY+d)
	minH := float32(40) + d
	if b.Height != 0 {
		minH = b.Height
	}
	minW := float32(64)
	if b.MinWidth != 0 {
		minW = b.MinWidth
	}
	e := ui.ButtonBase(c).Disabled(b.Disabled).DimDisabled(false).FocusRing(false).
		Row().AlignItems(ui.Center).Justify(ui.Center).Gap(8).
		MinWidth(minW).MinHeight(minH).Padding(padY, padEnd, padY, padStart).
		Radius(design.CornerFull).Shrink(0)
	if b.Disabled {
		e = e.Cursor(ui.CursorDefault)
	} else {
		e = e.Cursor(ui.CursorPointer)
	}
	if bg.A != 0 {
		e = e.Background(bg)
	}
	switch b.Kind {
	case Outlined:
		side := cs().Outline
		if b.Disabled {
			side = DisabledContainer()
		} else if e.FocusVisible() {
			side = cs().Primary
		}
		e = e.Border(1, side)
	case Elevated:
		if !b.Disabled {
			lvl := float32(1)
			if e.Hovered() && !e.Pressed() {
				lvl = 3
			}
			e = Elevation(e, lvl)
		}
	case Filled, Tonal:
		if !b.Disabled && e.Hovered() && !e.Pressed() {
			e = Elevation(e, 1)
		}
	}
	e = e.Transition(ui.ElementTransition{Duration: 200 * time.Millisecond, Colors: true, Shadows: true})
	if b.Tooltip != "" {
		e = Tip(c, e, b.Tooltip)
	}
	Ink(e, fg, design.CornerFull)
	style := tt().LabelLarge
	if b.Style != nil {
		style = *b.Style
	}
	style = style.C(fg)
	e.Children(func() {
		if b.Busy {
			Spinner(c, 18, 2, fg)
		} else if b.Icon != 0 && !b.IconAtEnd {
			Icon(c, b.Icon, 18, fg)
		}
		if b.Child != nil {
			b.Child(fg)
		} else {
			Clamp(Text(c, b.Label, style).TextAlign(ui.Center), 1)
		}
		if b.Icon != 0 && b.IconAtEnd && !b.Busy {
			Icon(c, b.Icon, 18, fg)
		}
	})
	if b.Label != "" {
		e = e.Label(b.Label)
	}
	return e
}

// FilledButton is FilledButton(child: Text(label)).
func FilledButton(c *ui.Context, label string) ui.Element {
	return Button{Kind: Filled, Label: label}.Show(c)
}

// TonalButton is FilledButton.tonal.
func TonalButton(c *ui.Context, label string) ui.Element {
	return Button{Kind: Tonal, Label: label}.Show(c)
}

// OutlinedButton is OutlinedButton(child: Text(label)).
func OutlinedButton(c *ui.Context, label string) ui.Element {
	return Button{Kind: Outlined, Label: label}.Show(c)
}

// TextButton is TextButton(child: Text(label)).
func TextButton(c *ui.Context, label string) ui.Element {
	return Button{Kind: TextKind, Label: label}.Show(c)
}

// IconButtonKind is IconButton's variant.
type IconButtonKind int

const (
	// Standard is IconButton.
	Standard IconButtonKind = iota
	// FilledIcon is IconButton.filled.
	FilledIcon
	// TonalIcon is IconButton.filledTonal.
	TonalIcon
	// OutlinedIcon is IconButton.outlined.
	OutlinedIcon
)

// IconButton is Material's icon button: a 40 DIP circle with a 24 DIP
// icon. Selected makes it a toggle (isSelected), showing SelectedIcon
// when set.
type IconButton struct {
	Kind         IconButtonKind
	Icon         icons.Icon
	SelectedIcon icons.Icon
	Tooltip      string
	Disabled     bool
	// Toggle makes it a toggle button, Selected its state.
	Toggle, Selected bool
	// Color replaces the icon's colour (IconButton(color:)).
	Color ui.Color
	// BG replaces the container colour.
	BG ui.Color
	// Size replaces the 24 DIP icon (iconSize).
	Size float32
	// Box replaces the 40 DIP container when not zero.
	Box float32
	// Square gives it Corners.sm corners in place of a circle.
	Radius float32
	// Child replaces the icon, in the content colour.
	Child func(fg ui.Color)
}

// Show builds the button.
func (b IconButton) Show(c *ui.Context) ui.Element {
	s := cs()
	var bg, fg ui.Color
	switch b.Kind {
	case FilledIcon:
		bg, fg = s.SurfaceContainerHighest, s.Primary
		if !b.Toggle || b.Selected {
			bg, fg = s.Primary, s.OnPrimary
		}
	case TonalIcon:
		bg, fg = s.SecondaryContainer, s.OnSecondaryContainer
		if b.Toggle && !b.Selected {
			bg, fg = s.SurfaceContainerHighest, s.OnSurfaceVariant
		}
	case OutlinedIcon:
		fg = s.OnSurfaceVariant
		if b.Toggle && b.Selected {
			bg, fg = s.InverseSurface, s.OnInverseSurface
		}
	default:
		fg = s.OnSurfaceVariant
		if b.Toggle && b.Selected {
			fg = s.Primary
		}
	}
	if b.Color.A != 0 {
		fg = b.Color
	}
	if b.BG.A != 0 {
		bg = b.BG
	}
	if b.Disabled {
		fg = DisabledContent()
		if b.Kind == FilledIcon || b.Kind == TonalIcon {
			bg = DisabledContainer()
		} else if b.Kind == OutlinedIcon && b.Toggle && b.Selected {
			bg = DisabledContainer()
		}
	}
	size := b.Size
	if size == 0 {
		size = 24
	}
	box := b.Box
	if box == 0 {
		box = 40 + Density()*4
	}
	radius := b.Radius
	if radius == 0 {
		radius = design.CornerFull
	}
	e := ui.ButtonBase(c).Disabled(b.Disabled).DimDisabled(false).FocusRing(false).
		Size(box, box).Center().Radius(radius).Shrink(0)
	if !b.Disabled {
		e = e.Cursor(ui.CursorPointer)
	}
	if bg.A != 0 {
		e = e.Background(bg)
	}
	if b.Kind == OutlinedIcon && !(b.Toggle && b.Selected) {
		side := s.Outline
		if b.Disabled {
			side = DisabledContainer()
		}
		e = e.Border(1, side)
	}
	if b.Toggle {
		e = e.Checked(b.Selected)
	}
	if b.Tooltip != "" {
		e = Tip(c, e.Label(b.Tooltip), b.Tooltip)
	}
	Ink(e, fg, radius)
	ic := b.Icon
	if b.Toggle && b.Selected && b.SelectedIcon != 0 {
		ic = b.SelectedIcon
	}
	e.Children(func() {
		if b.Child != nil {
			b.Child(fg)
			return
		}
		Icon(c, ic, size, fg)
	})
	return e
}

// IconBtn is IconButton(icon: Icon(ic), tooltip: tip).
func IconBtn(c *ui.Context, ic icons.Icon, tip string) ui.Element {
	return IconButton{Icon: ic, Tooltip: tip}.Show(c)
}

// FAB is FloatingActionButton (and .small and .extended) with the app's
// theme: primaryContainer, elevation 3 (4 hovered), Corners.lg.
type FAB struct {
	Icon     icons.Icon
	Label    string // .extended when not empty
	Small    bool
	Tooltip  string
	Disabled bool
	// BG replaces primaryContainer; Child the icon.
	BG    ui.Color
	Child func()
	// Flat is elevation 0.
	Flat bool
}

// Show builds the button.
func (f FAB) Show(c *ui.Context) ui.Element {
	s := cs()
	bg, fg := s.PrimaryContainer, s.OnPrimaryContainer
	if f.BG.A != 0 {
		bg = f.BG
		fg = s.OnSurface
	}
	e := ui.ButtonBase(c).Disabled(f.Disabled).DimDisabled(false).FocusRing(false).
		Background(bg).Radius(design.CornerLG).Cursor(ui.CursorPointer).Shrink(0)
	radius := design.CornerLG
	switch {
	case f.Label != "":
		e = e.Row().AlignItems(ui.Center).Height(56).MinWidth(80).Padding(0, 20, 0, 16).Gap(8)
	case f.Small:
		radius = design.CornerMD
		e = e.Size(40, 40).Center().Radius(radius)
	default:
		e = e.Size(56, 56).Center()
	}
	lvl := float32(3)
	if e.Hovered() && !f.Disabled {
		lvl = 4
	}
	if f.Flat {
		lvl = 0
	}
	e = Elevation(e, lvl).Transition(ui.ElementTransition{Duration: 200 * time.Millisecond, Shadows: true})
	if f.Tooltip != "" {
		e = Tip(c, e, f.Tooltip)
	}
	Ink(e, fg, radius)
	e.Children(func() {
		if f.Child != nil {
			f.Child()
		} else if f.Icon != 0 {
			Icon(c, f.Icon, 24, fg)
		}
		if f.Label != "" {
			Text(c, f.Label, tt().LabelLarge.C(fg)).NoWrap()
		}
	})
	return e
}
