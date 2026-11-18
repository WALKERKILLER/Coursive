package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/tex"
)

// Math is set 1.1 times the text's size, as the Flutter client's: Latin
// Modern beside the UI font reads small otherwise.
const mathScale = 1.1

// MathInline makes inline math an object of its text.
func MathInline(src string, size float32, color ui.Color) (markdown.InlineObject, bool) {
	f := tex.Typeset(src, size*mathScale, false)
	if f.Width == 0 {
		return markdown.InlineObject{}, false
	}
	return markdown.InlineObject{Width: f.Width, Height: f.Height, Depth: f.Depth, Draw: func(p *ui.Painter, r ui.Rect) {
		f.Draw(p, r.X, r.Y+f.Height, color)
	}}, true
}

// MathBlock builds display math, centred, scrolling sideways when wide.
func MathBlock(c *ui.Context, src string, size float32, color ui.Color) {
	f := tex.Typeset(src, size*mathScale, true)
	ui.ScrollHorizontal(c).FillWidth().Children(func() {
		ui.Box(c).MinWidthPercent(100).AlignItems(ui.Center).Padding(design.SpaceXS, 0).Children(func() {
			ui.Box(c).Width(f.Width).Height(f.Height+f.Depth).Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) {
				f.Draw(p, r.X, r.Y+f.Height, color)
			})
		})
	})
}
