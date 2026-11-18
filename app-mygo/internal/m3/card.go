package m3

import (
	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
)

// CardKind is Card, Card.filled or Card.outlined.
type CardKind int

const (
	CardElevated CardKind = iota
	CardFilled
	CardOutlined
)

// Card is a Card with the app's CardTheme: no elevation, no margin,
// surfaceContainerLow, Corners.md, clipping its child. Filled cards take
// surfaceContainerHighest, outlined ones surface with an outlineVariant
// border.
func Card(c *ui.Context, kind CardKind) ui.Element {
	s := cs()
	e := ui.Column(c).Radius(design.CornerMD).Clip()
	switch kind {
	case CardFilled:
		e = e.Background(s.SurfaceContainerHighest)
	case CardOutlined:
		e = e.Background(s.Surface).Border(1, s.OutlineVariant)
	default:
		e = e.Background(s.SurfaceContainerLow)
	}
	return e
}

// CardColor is a card in another colour (Card(color:)).
func CardColor(c *ui.Context, color ui.Color) ui.Element {
	return ui.Column(c).Radius(design.CornerMD).Clip().Background(color)
}

// Tappable is InkWell(onTap:) filling its parent: a button base with the
// plain InkWell look; style its layout and read Clicked.
func Tappable(c *ui.Context, radius float32) ui.Element {
	e := ui.ButtonBase(c).FocusRing(false).Cursor(ui.CursorPointer)
	return InkWell(e, radius)
}
