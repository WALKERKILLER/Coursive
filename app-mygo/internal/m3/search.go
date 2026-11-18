package m3

import (
	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
)

// SearchBar is SearchBar with the app's SearchBarTheme: no elevation,
// surfaceContainerHigh, 16 DIPs of padding, at least 56 high and at most
// 720 wide, fully rounded; hint in bodyLarge onSurfaceVariant.
type SearchBar struct {
	Hint     string
	Leading  func()
	Trailing func()
	// Height replaces the 56 DIP minimum.
	Height    float32
	AutoFocus bool
	BG        ui.Color
	// MaxWidth replaces 720; -1 is unbounded.
	MaxWidth float32
}

// Show builds the bar editing query and returns its input.
func (b SearchBar) Show(c *ui.Context, query *string) ui.Element {
	s := cs()
	h := b.Height
	if h == 0 {
		h = 56
	}
	bg := s.SurfaceContainerHigh
	if b.BG.A != 0 {
		bg = b.BG
	}
	maxW := b.MaxWidth
	if maxW == 0 {
		maxW = 720
	}
	var input ui.Element
	bar := ui.Row(c).AlignItems(ui.Center).MinHeight(h).Padding(0, 16).Gap(16).Radius(design.CornerFull).
		Background(bg).Cursor(ui.CursorText).Shrink(0)
	if maxW > 0 {
		bar = bar.MaxWidth(maxW)
	}
	bar.Children(func() {
		if b.Leading != nil {
			b.Leading()
		}
		input = ui.TextInputBase(c, query).Grow(1).Basis(0)
		input = tt().BodyLarge.C(s.OnSurface).Apply(input).CaretColor(s.Primary).
			SelectionColor(design.WithOpacity(s.Primary, .4)).Placeholder(b.Hint).PlaceholderColor(s.OnSurfaceVariant).Search()
		if b.AutoFocus {
			input = input.AutoFocus()
		}
		if b.Trailing != nil {
			ui.Row(c).AlignItems(ui.Center).Shrink(0).Children(b.Trailing)
		}
	})
	if bar.Clicked() && !input.Focused() {
		input.Focus()
	}
	InkLayer(bar, s.OnSurface, design.CornerFull, .08, 0, 0)
	return input
}

// Badge is Badge: a 6 DIP dot without a label, or a 16 DIP pill with
// labelSmall, in error and onError.
func Badge(c *ui.Context, label string) ui.Element {
	s := cs()
	if label == "" {
		return ui.Box(c).Size(6, 6).Radius(3).Background(s.Error).Shrink(0)
	}
	return ui.Row(c).Height(16).MinWidth(16).Padding(0, 4).Radius(8).Background(s.Error).Center().Shrink(0).
		Children(func() { Text(c, label, tt().LabelSmall.C(s.OnError)).NoWrap() })
}

// Badged puts a badge on child's top end corner, as Badge(child:) does.
func Badged(c *ui.Context, label string, show bool, child func()) ui.Element {
	return ui.Box(c).Shrink(0).Children(func() {
		child()
		if !show {
			return
		}
		if label == "" {
			Badge(c, "").Absolute().Top(-1).Right(-1)
		} else {
			Badge(c, label).Absolute().Top(-4).Left(12)
		}
	})
}
