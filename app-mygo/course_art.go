package main

import (
	"math"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/harukizmoe/mayu/theme"
)

// The course cover, ported from app/lib/widgets/course_visuals.dart: a recipe
// hashed from the course's id, four layouts, and a palette generated from a
// per-course seed. The hash and the random draws match the Dart client's, so a
// course gets the same cover in both.

// dartRandom reproduces dart:math's Random, a 48-bit linear congruential
// generator, so the draws below land on the same numbers as the Flutter
// client's.
type dartRandom struct{ state uint64 }

const lcgMask = (1 << 48) - 1

func newDartRandom(seed uint32) *dartRandom {
	return &dartRandom{state: (uint64(seed) ^ 0x5DEECE66D) & lcgMask}
}

func (r *dartRandom) nextState() uint64 {
	r.state = (r.state*0x5DEECE66D + 0xB) & lcgMask
	return r.state
}

func (r *dartRandom) intn(max int) int {
	return int((r.nextState() >> 16) % uint64(max))
}

func (r *dartRandom) float() float64 {
	a := r.nextState() >> 5
	b := r.nextState() >> 6
	return (float64(a)*67108864.0 + float64(b)) / 9007199254740992.0
}

// The shapes that read well as a hero form, and the softer accents.
var heroShapes = []OrganicShape{
	ShapeCookie9, ShapeCookie12, ShapeClover4, ShapeClover8, ShapeSunny,
	ShapeVerySunny, ShapeSoftBurst, ShapeFlower, ShapePuffy, ShapeCookie6,
	ShapePentagon, ShapeGem, ShapeArch, ShapeSquircle, ShapeCookie7,
}

var accentShapes = []OrganicShape{
	ShapeCircle, ShapePill, ShapeDiamond, ShapeTriangle, ShapeCookie4,
	ShapeOval, ShapeHeart, ShapeSquircle, ShapeClover4,
}

type clusterItem struct {
	shape  OrganicShape
	x, y   float64
	d      float64
	colour int
}

// CourseArtSpec is the deterministic recipe for one course's cover.
type CourseArtSpec struct {
	main, partner, accent  OrganicShape
	layout, texture, tone  int
	rot, accentRot, jitter float64
	cluster                []clusterItem
}

var artCache = map[string]*CourseArtSpec{}

// courseArtSpec is the recipe for a key, memoised as the Dart client does.
func courseArtSpec(key string) *CourseArtSpec {
	if s, ok := artCache[key]; ok {
		return s
	}
	var h uint32 = 0x811c9dc5
	for _, c := range []byte(key) {
		h = (h ^ uint32(c)) * 0x01000193
	}
	r := newDartRandom(h)
	s := &CourseArtSpec{}
	s.main = heroShapes[r.intn(len(heroShapes))]
	p := heroShapes[r.intn(len(heroShapes))]
	if p == s.main {
		p = ShapeCircle
	}
	s.partner = p
	s.accent = accentShapes[r.intn(len(accentShapes))]
	s.layout = r.intn(4)
	s.texture = r.intn(4)
	s.tone = r.intn(3)
	s.rot = r.float() * math.Pi
	s.accentRot = r.float() * math.Pi
	s.jitter = r.float()
	all := append(append([]OrganicShape{}, heroShapes...), accentShapes...)
	for i := 0; i < 5; i++ {
		s.cluster = append(s.cluster, clusterItem{
			shape:  all[r.intn(len(all))],
			x:      0.12 + 0.19*float64(i) + (r.float()-0.5)*0.06,
			y:      0.3 + map[bool]float64{true: 0, false: 0.38}[i%2 == 0] + (r.float()-0.5)*0.14,
			d:      0.3 + r.float()*0.22,
			colour: r.intn(3),
		})
	}
	artCache[key] = s
	return s
}

func (s *CourseArtSpec) containerOf(c theme.ColorScheme) ui.Color {
	return []ui.Color{c.PrimaryContainer, c.TertiaryContainer, c.SecondaryContainer}[s.tone]
}

func (s *CourseArtSpec) fillOf(c theme.ColorScheme) ui.Color {
	return []ui.Color{c.Primary, c.Tertiary, c.Secondary}[s.tone]
}

func (s *CourseArtSpec) altOf(c theme.ColorScheme) ui.Color {
	return []ui.Color{c.Tertiary, c.Primary, c.Primary}[s.tone]
}

func (s *CourseArtSpec) onOf(c theme.ColorScheme) ui.Color {
	return []ui.Color{c.OnPrimaryContainer, c.OnTertiaryContainer, c.OnSecondaryContainer}[s.tone]
}

// courseScheme is the per-course palette: the course's own colour when it has
// one, else a hue hashed from its raw title, through the same M3 generator the
// rest of the client uses.
func courseScheme(c Course, dark bool) theme.ColorScheme {
	var seed ui.Color
	if strings.HasPrefix(c.Color, "#") && len(c.Color) == 7 {
		if v, err := strconv.ParseUint(c.Color[1:], 16, 32); err == nil {
			seed = ui.RGB(uint8(v>>16), uint8(v>>8), uint8(v))
		}
	}
	if seed == (ui.Color{}) {
		key := c.RawTitle
		if key == "" {
			key = c.Title
		}
		var h int32
		for _, r := range key {
			h = (h*31 + int32(r)) & 0x7fffffff
		}
		seed = hsl(float64(((h%360)+360)%360), 0.45, 0.45)
	}
	return theme.SchemeFromSeed(seed, dark)
}

// hsl converts an HSL colour, as HSLColor.toColor does.
func hsl(h, s, l float64) ui.Color {
	h = math.Mod(h, 360)
	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := l - c/2
	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	to := func(v float64) uint8 { return uint8(math.Round((v + m) * 255)) }
	return ui.RGB(to(r), to(g), to(b))
}

// withAlpha returns c at the given opacity, as Flutter's withValues(alpha:).
func withAlpha(c ui.Color, a float64) ui.Color {
	c.A = uint8(math.Round(a * 255))
	return c
}

// drawCourseArt paints the cover for spec in r.
func drawCourseArt(p *ui.Painter, r ui.Rect, spec *CourseArtSpec, s theme.ColorScheme, t float64) {
	w, h := float64(r.W), float64(r.H)
	bg := spec.containerOf(s)
	fill := spec.fillOf(s)
	alt := spec.altOf(s)
	on := spec.onOf(s)

	p.Fill(r, bg, 0)
	sq := func(cx, cy, d float64) ui.Rect {
		return ui.Rect{X: r.X + float32(cx-d/2), Y: r.Y + float32(cy-d/2), W: float32(d), H: float32(d)}
	}
	off := func(x, y, d float64) ui.Rect { return sq(x*w, y*h, d) }

	mainRadii := morphRadii(spec.main, spec.partner, t)

	switch spec.layout {
	case 0: // hero: one large form, an outlined companion and dot texture
		dots(p, ui.Rect{X: r.X + float32(w*0.04), Y: r.Y + float32(h*0.5), W: float32(w * 0.36), H: float32(h * 0.46)},
			withAlpha(on, 0.16), h/14)
		path := shapePath(mainRadii, off(0.68, 0.56, h*1.02), spec.rot+t*0.45)
		p.FillPath(&path, fill)
		companion := shapePath(shapeRadii(spec.accent), off(0.2, 0.3, h*(0.34+0.04*t)), spec.accentRot)
		p.StrokePath(&companion, float32(math.Max(1.5, h/70)), withAlpha(on, 0.55))

	case 1: // pair: two overlapping forms inside faint concentric rings
		rings(p, r.X+float32(w*0.42), r.Y+float32(h*0.55), h*0.2, withAlpha(on, 0.1), h/9)
		path := shapePath(mainRadii, off(0.4, 0.56, h*0.86), spec.rot+t*0.4)
		p.FillPath(&path, fill)
		second := shapePath(shapeRadii(spec.accent), off(0.7, 0.42, h*0.62), spec.accentRot-t*0.6)
		p.FillPath(&second, withAlpha(alt, 0.88))

	case 2: // cluster: a small constellation of mixed shapes
		lines(p, r, withAlpha(on, 0.08), h/8)
		for i, item := range spec.cluster {
			colour := []ui.Color{fill, alt, withAlpha(on, 0.22)}[item.colour]
			radii := shapeRadii(item.shape)
			d := item.d
			if i == 1 {
				radii = mainRadii
				d = item.d * (1 + 0.12*t)
			}
			rot := spec.rot * float64(i+1)
			if i%2 == 0 {
				rot = -rot
			}
			path := shapePath(radii, off(item.x, item.y, h*d), rot)
			p.FillPath(&path, colour)
		}

	default: // peek: a huge form bleeding off the corner with a nested echo
		lines(p, ui.Rect{X: r.X, Y: r.Y, W: float32(w * 0.55), H: float32(h * 0.55)}, withAlpha(on, 0.1), h/10)
		big := off(0.86, 0.98, h*1.7)
		path := shapePath(mainRadii, big, spec.rot+t*0.3)
		p.FillPath(&path, fill)
		echo := shapePath(mainRadii, ui.Rect{
			X: big.X + big.W*0.22, Y: big.Y + big.H*0.22,
			W: big.W * 0.56, H: big.H * 0.56,
		}, spec.rot)
		p.FillPath(&echo, withAlpha(bg, 0.9))
		accent := shapePath(shapeRadii(spec.accent), off(0.2, 0.32, h*0.26), spec.accentRot)
		p.FillPath(&accent, alt)
	}
}

func dots(p *ui.Painter, r ui.Rect, c ui.Color, gap float64) {
	for y := float64(r.Y); y < float64(r.Y+r.H); y += gap {
		for x := float64(r.X); x < float64(r.X+r.W); x += gap {
			var circle ui.Path
			circle.Circle(float32(x), float32(y), float32(gap*0.13))
			p.FillPath(&circle, c)
		}
	}
}

func rings(p *ui.Painter, cx, cy float32, start float64, c ui.Color, gap float64) {
	for rad := start; rad < start+gap*6; rad += gap {
		var circle ui.Path
		circle.Circle(cx, cy, float32(rad))
		p.StrokePath(&circle, 1.2, c)
	}
}

func lines(p *ui.Painter, r ui.Rect, c ui.Color, gap float64) {
	w, h := float64(r.W), float64(r.H)
	for x := -h; x < w; x += gap {
		p.Line(float32(float64(r.X)+x), float32(float64(r.Y)+h), float32(float64(r.X)+x+h), float32(r.Y), 1.2, c)
	}
}
