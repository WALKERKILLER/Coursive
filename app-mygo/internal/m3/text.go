package m3

import (
	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
)

// Text is Text(s, style: st).
func Text(c *ui.Context, s string, st design.TextStyle) ui.Element {
	return st.Apply(ui.Text(c, s))
}

// Body is text in the default style, bodyMedium in onSurface.
func Body(c *ui.Context, s string) ui.Element { return Text(c, s, tt().BodyMedium) }

// Muted is bodyMedium in onSurfaceVariant.
func Muted(c *ui.Context, s string) ui.Element {
	return Text(c, s, tt().BodyMedium.C(cs().OnSurfaceVariant))
}

// Clamp limits text to n lines with an ellipsis (overflow: ellipsis).
func Clamp(e ui.Element, n int) ui.Element { return e.MaxLines(n).Ellipsis("…") }

// Icon is Icon(icon, size:, color:): a glyph of the Material icon font in
// a square of its size. Icons don't follow the text scale.
func Icon(c *ui.Context, ic icons.Icon, size float32, color ui.Color) ui.Element {
	if size == 0 {
		size = 24
	}
	return ui.Text(c, ic.String()).Font(icons.Family).FontSize(size).FixedLineHeight(size).
		FontWeight(400).LetterSpacing(0).Size(size, size).TextAlign(ui.Center).TextColor(color).
		NoWrap().Unselectable().AccessHidden(true)
}

// Gap is SizedBox(width: w) or (height: h) in a row or column.
func Gap(c *ui.Context, size float32) ui.Element { return ui.Box(c).Size(size, size).Shrink(0) }

// HGap is SizedBox(width:).
func HGap(c *ui.Context, w float32) ui.Element { return ui.Box(c).Width(w).Shrink(0) }

// VGap is SizedBox(height:).
func VGap(c *ui.Context, h float32) ui.Element { return ui.Box(c).Height(h).Shrink(0) }

// IconPaint draws an icon's glyph in a square of size at (x, y), in a Draw.
func IconPaint(p *ui.Painter, ic icons.Icon, x, y, size float32, color ui.Color) {
	p.RichText(x, y, size*2, ui.Span{Text: ic.String(), Font: icons.Family, Size: size, Color: color, NoLetterSpacing: true})
}
