package widgets

import (
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
)

// Glass is glass.dart's GlassSurface: a frosted pane of a radius that floats
// over content, with a rim catching the light and a shadow outside it.
// Without 液态玻璃 (or in high contrast) the pane is an opaque tonal
// surface of the same shape and rim.
type Glass struct {
	Radius   float32
	Depth    design.GlassDepth
	Tint     ui.Color
	NoShadow bool
	// Selected draws the rim in the primary colour.
	Selected bool
}

// frost is the pane's material: the backdrop blurred, then the fill.
type frost struct {
	blur glass.Blur
	fill ui.Color
}

func (f frost) PaintMaterial(p *ui.Painter, box ui.Rect, radii [4]float32) {
	f.blur.PaintMaterial(p, box, radii)
	p.FillRadii(box, radii, f.fill)
}

// Apply makes e the pane; its children draw on the glass, clipped to it.
func (g Glass) Apply(e ui.Element) ui.Element {
	r := g.Radius
	if r == 0 {
		r = design.CornerXL
	}
	fill := design.GlassFill(g.Depth, g.Tint)
	e = e.Radius(r).Clip()
	if design.T().Glass {
		e = e.Material(frost{glass.Blur{Radius: design.GlassSigma(g.Depth)}, fill})
	} else {
		e = e.Background(fill)
	}
	if !g.NoShadow {
		e = design.GlassShadow(e, g.Depth)
	}
	sel := g.Selected
	return e.DrawOver(func(p *ui.Painter, b ui.Rect) { design.GlassRim(p, b, min(r, min(b.W, b.H)/2), sel) })
}
