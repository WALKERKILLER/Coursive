package design

import "github.com/egoist/mygo/ui"

// GlassDepth is how much a glass pane lifts off what is under it.
type GlassDepth int

const (
	// GlassBar is toolbars and docks: thin frost, light tint.
	GlassBar GlassDepth = iota
	// GlassPanel is message bubbles and panels holding text.
	GlassPanel
	// GlassFloat is menus and the composer: nearly opaque.
	GlassFloat
)

// GlassFill is GlassSurface.fillOf: the fill a pane paints over its frost.
// tint replaces the neutral tint when its alpha is not zero.
func GlassFill(depth GlassDepth, tint ui.Color) ui.Color {
	t := T()
	s := t.Scheme
	base := tint
	if tint.A == 0 {
		if t.Dark {
			base = Lerp(s.SurfaceContainerHigh, s.SurfaceBright, .35)
		} else {
			base = s.SurfaceContainerLowest
		}
	}
	if !t.Glass {
		return AlphaBlend(base, s.Surface)
	}
	var a float32
	switch depth {
	case GlassBar:
		a = pick(t.Dark, .58, .62)
	case GlassPanel:
		a = pick(t.Dark, .74, .78)
	default:
		a = pick(t.Dark, .86, .88)
	}
	return WithOpacity(base, a)
}

// GlassSigma is the frost's blur for a depth.
func GlassSigma(depth GlassDepth) float32 {
	switch depth {
	case GlassBar:
		return 16
	case GlassPanel:
		return 22
	}
	return 28
}

func pick(dark bool, d, l float32) float32 {
	if dark {
		return d
	}
	return l
}

// GlassRim draws a pane's rim over r, as glass.dart's _Rim: a hairline
// outside that keeps the edge on light content, and a light catching the
// upper edge that fades along the sides, or the primary colour when the
// pane is selected.
func GlassRim(p *ui.Painter, r ui.Rect, radius float32, selected bool) {
	t := T()
	s := t.Scheme
	p.Stroke(r, ui.RGBA(0, 0, 0, pick(t.Dark, .5, .07)), radius, .6)
	in := ui.Rect{X: r.X + .5, Y: r.Y + .5, W: r.W - 1, H: r.H - 1}
	ir := radius - .5
	if ir < 0 {
		ir = 0
	}
	if selected {
		p.Stroke(in, s.Primary, ir, 1.5)
		return
	}
	light := ui.RGBA(255, 255, 255, pick(t.Dark, .2, .85))
	end := ui.RGBA(255, 255, 255, pick(t.Dark, .05, .3))
	path := RoundedRectPath(in, ir)
	p.StrokePathGradient(path, 1, ui.LinearGradient{
		Angle: 180,
		Stops: []ui.GradientStop{{Color: light, At: 0}, {Color: WithOpacity(light, 0), At: .55}, {Color: end, At: 1}},
	})
}

// GlassShadow adds the pane's outer shadow, glass.dart's _OuterShadow: a
// soft drop below and a tight one under the edge. MyGo draws box shadows
// only outside the box, so the frost stays clear as Flutter's clip keeps it.
func GlassShadow(e ui.Element, depth GlassDepth) ui.Element {
	t := T()
	a := pick(t.Dark, .42, .1)
	deep := depth != GlassBar
	y, blur := float32(6), float32(12)
	if deep {
		y, blur = 10, 18
	}
	// MaskFilter.blur takes a sigma; CSS's blur radius is twice it.
	return e.Shadow(0, y, blur*2, 0, ui.RGBA(0, 0, 0, a)).Shadow(0, 1, 4, 0, ui.RGBA(0, 0, 0, a*.6))
}

// BoxShadowBlur is the blur of a shadow for Flutter's BoxShadow of
// blurRadius radius: Flutter blurs with a sigma of 0.57735 radius + 0.5,
// and CSS's blur, which Shadow takes, is twice the sigma.
func BoxShadowBlur(radius float32) float32 { return 2 * (radius*0.57735 + 0.5) }
