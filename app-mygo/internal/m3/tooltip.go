package m3

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
)

// Tip gives anchor Material's tooltip: inverseSurface, Corners.xs,
// bodySmall in onInverseSurface, below the anchor as Flutter prefers,
// after the theme's 500 ms wait.
func Tip(c *ui.Context, anchor ui.Element, text string) ui.Element {
	if text == "" {
		return anchor
	}
	anchor = anchor.Description(text)
	ui.TooltipBase(c, anchor, func(tip ui.Element) {
		s := cs()
		tip.AttachTo(anchor, ui.AnchorBottom, ui.AnchorTop).Margin(4, 0, 0, 0).
			MinHeight(24).Padding(4, 8).Radius(design.CornerXS).Background(s.InverseSurface).
			MaxWidth(360).Justify(ui.Center).AttachedOrigin().
			Transition(ui.ElementTransition{Duration: 150 * time.Millisecond, ExitDuration: 75 * time.Millisecond,
				Enter: &ui.Motion{Opacity: 0}, Exit: &ui.Motion{Opacity: 0}, Shadows: true})
		tip.Children(func() { Text(c, text, tt().BodySmall.C(s.OnInverseSurface)) })
	})
	return anchor
}
