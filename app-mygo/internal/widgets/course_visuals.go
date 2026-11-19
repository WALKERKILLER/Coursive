package widgets

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
)

// course_visuals.dart: a course's generative cover, a recipe hashed from
// its id with four layouts, in a palette of its own. The hash and the
// random draws match the Dart client's, so a course gets the same cover in
// both.

// CourseScheme is the course's tonal palette: its own colour when it has
// one, else a hue hashed from its raw title.
func CourseScheme(c core.Course) *design.Scheme {
	var seed ui.Color
	col := c.Color()
	if strings.HasPrefix(col, "#") && len(col) == 7 {
		if v, err := strconv.ParseUint(col[1:], 16, 32); err == nil {
			seed = ui.RGB(uint8(v>>16), uint8(v>>8), uint8(v))
		}
	}
	if seed.A == 0 {
		var h int64
		for _, r := range c.RawTitle() {
			h = (h*31 + int64(r)) & 0x7fffffff
		}
		seed = hsl(float64(h%360), .45, .45)
	}
	return design.FromSeed(seed, design.TonalSpot, design.T().Dark, 0)
}

// hsl is HSLColor.toColor.
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

// dartRandom is dart:math's Random as the Dart VM implements it
// (math_patch.dart): multiply-with-carry on a 64-bit state, seeded through
// Thomas Wang's 64-bit mix, so the draws land on the Flutter client's
// numbers.
type dartRandom struct{ state uint64 }

func newDartRandom(seed uint32) *dartRandom {
	n := uint64(seed)
	n = ^n + n<<21
	n ^= n >> 24
	n *= 265
	n ^= n >> 14
	n *= 21
	n ^= n >> 28
	n += n << 31
	if n == 0 {
		n = 0x5a17
	}
	r := &dartRandom{state: n}
	// Cranked a couple of times to spread the seed's bits, as Random(seed).
	for range 4 {
		r.next()
	}
	return r
}

func (r *dartRandom) next() {
	r.state = 0xffffda61*(r.state&0xffffffff) + r.state>>32
}

func (r *dartRandom) intn(max int) int {
	m := uint64(max)
	if m&(m-1) == 0 {
		r.next()
		return int(r.state & 0xffffffff & (m - 1))
	}
	for {
		r.next()
		rnd := r.state & 0xffffffff
		res := rnd % m
		if rnd-res+m <= 1<<32 {
			return int(res)
		}
	}
}

func (r *dartRandom) float() float64 {
	return (float64(r.intn(1<<26))*(1<<27) + float64(r.intn(1<<27))) / (1 << 53)
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

// ArtSpec is a course's cover recipe.
type ArtSpec struct {
	main, partner, accent  OrganicShape
	layout, texture, tone  int
	rot, accentRot, jitter float64
	cluster                []clusterItem
}

var artCache = map[string]*ArtSpec{}

func artKey(c core.Course) string {
	if c.ID() == "" {
		return c.RawTitle()
	}
	return c.ID()
}

// CourseArtSpec is the recipe for a key, memoised.
func CourseArtSpec(key string) *ArtSpec {
	if s, ok := artCache[key]; ok {
		return s
	}
	var h uint32 = 0x811c9dc5
	for _, u := range utf16Units(key) {
		h = (h ^ uint32(u)) * 0x01000193
	}
	r := newDartRandom(h)
	s := &ArtSpec{}
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
		shape := all[r.intn(len(all))]
		x := .12 + .19*float64(i) + (r.float()-.5)*.06
		y := .3 + float64(i%2)*.38 + (r.float()-.5)*.14
		d := .3 + r.float()*.22
		s.cluster = append(s.cluster, clusterItem{shape, x, y, d, r.intn(3)})
	}
	artCache[key] = s
	return s
}

// utf16Units is String.codeUnits.
func utf16Units(s string) []uint16 {
	var out []uint16
	for _, r := range s {
		if r >= 0x10000 {
			r -= 0x10000
			out = append(out, uint16(0xD800+(r>>10)), uint16(0xDC00+(r&0x3FF)))
		} else {
			out = append(out, uint16(r))
		}
	}
	return out
}

// Layout is the recipe's layout, 0 to 3.
func (s *ArtSpec) Layout() int { return s.layout }

func (s *ArtSpec) ContainerOf(c *design.Scheme) ui.Color {
	return []ui.Color{c.PrimaryContainer, c.TertiaryContainer, c.SecondaryContainer}[s.tone]
}
func (s *ArtSpec) FillOf(c *design.Scheme) ui.Color {
	return []ui.Color{c.Primary, c.Tertiary, c.Secondary}[s.tone]
}
func (s *ArtSpec) AltOf(c *design.Scheme) ui.Color {
	return []ui.Color{c.Tertiary, c.Primary, c.Primary}[s.tone]
}
func (s *ArtSpec) OnOf(c *design.Scheme) ui.Color {
	return []ui.Color{c.OnPrimaryContainer, c.OnTertiaryContainer, c.OnSecondaryContainer}[s.tone]
}

func alpha(c ui.Color, a float64) ui.Color { return design.WithOpacity(c, float32(a)) }

// CourseGlyph is the small badge of a course: its signature shape with a
// nested accent, morphing while hover is set.
func CourseGlyph(c *ui.Context, course core.Course, size float32, hover bool) ui.Element {
	s := CourseScheme(course)
	spec := CourseArtSpec(artKey(course))
	e := ui.Box(c).Size(size, size).Shrink(0)
	t := float64(e.AnimateTo("glyph", map[bool]float32{true: 1, false: 0}[hover], design.T().Motion(design.Long2, design.Emphasized)))
	return e.Draw(func(p *ui.Painter, r ui.Rect) {
		outer := ShapePath(MorphRadii(spec.main, spec.partner, t), r, spec.rot+t*.5)
		p.FillPath(&outer, spec.ContainerOf(s))
		in := ui.Rect{X: r.X + r.W*.29, Y: r.Y + r.H*.29, W: r.W * .42, H: r.H * .42}
		inner := ShapePath(ShapeRadii(spec.accent), in, spec.accentRot-t*.8)
		p.FillPath(&inner, spec.FillOf(s))
	})
}

// CourseArt fills e with the course's cover, rounded by radius as by a
// ClipRRect: what an element draws is not clipped by its own Clip, which
// clips its children. t morphs the hero shape (hover, press); spin turns it
// slowly, in turns; layout < 0 uses the recipe's.
func CourseArt(e ui.Element, course core.Course, t, spin float64, layout int, radius float32) ui.Element {
	s := CourseScheme(course)
	spec := CourseArtSpec(artKey(course))
	if layout < 0 {
		layout = spec.layout
	}
	return e.Radius(radius).Clip().Draw(func(p *ui.Painter, r ui.Rect) {
		p.Clip(r, radius, func() { PaintCourseArt(p, r, spec, s, t, spin, layout) })
	})
}

// CourseArtTurning is CourseArt turning once every period from start, as a
// repeating AnimationController does; reduced motion holds it still. It
// repaints at 15 fps, which is smooth at so slow a turn.
func CourseArtTurning(e ui.Element, course core.Course, t float64, start time.Time, period time.Duration, radius float32) ui.Element {
	s := CourseScheme(course)
	spec := CourseArtSpec(artKey(course))
	still := design.T().ReduceMotion
	return e.Radius(radius).Clip().Draw(func(p *ui.Painter, r ui.Rect) {
		spin := 0.0
		if !still {
			spin = math.Mod(float64(p.Now().Sub(start))/float64(period), 1)
			p.AnimationFrameRate(15)
		}
		p.Clip(r, radius, func() { PaintCourseArt(p, r, spec, s, t, spin, spec.layout) })
	})
}

// PaintCourseArt is CourseArtPainter.paint.
func PaintCourseArt(p *ui.Painter, r ui.Rect, spec *ArtSpec, s *design.Scheme, t, spin float64, layout int) {
	w, h := float64(r.W), float64(r.H)
	bg, fill, alt, on := spec.ContainerOf(s), spec.FillOf(s), spec.AltOf(s), spec.OnOf(s)
	p.Fill(r, bg, 0)
	turn := spin * 2 * math.Pi
	mainRadii := MorphRadii(spec.main, spec.partner, t)
	sq := func(cx, cy, d float64) ui.Rect {
		return ui.Rect{X: r.X + float32(cx-d/2), Y: r.Y + float32(cy-d/2), W: float32(d), H: float32(d)}
	}
	fillPath := func(path ui.Path, c ui.Color) { p.FillPath(&path, c) }
	switch layout {
	case 0: // hero: one large form, an outlined companion and dot texture
		dots(p, ui.Rect{X: r.X + float32(w*.04), Y: r.Y + float32(h*.5), W: float32(w * .36), H: float32(h * .46)}, alpha(on, .16), h/14)
		fillPath(ShapePath(mainRadii, sq(w*.68, h*.56, h*1.02), spec.rot+turn+t*.45), fill)
		comp := ShapePath(ShapeRadii(spec.accent), sq(w*.2, h*.3, h*(.34+.04*t)), spec.accentRot-turn*1.6)
		p.StrokePath(&comp, float32(math.Max(1.5, h/70)), alpha(on, .55))
	case 1: // pair: two overlapping forms inside faint concentric rings
		rings(p, r.X+float32(w*.42), r.Y+float32(h*.55), h*.2, alpha(on, .1), h/9)
		fillPath(ShapePath(mainRadii, sq(w*.4, h*.56, h*.86), spec.rot+turn+t*.4), fill)
		fillPath(ShapePath(ShapeRadii(spec.accent), sq(w*.7, h*.42, h*.62), spec.accentRot-turn-t*.6), alpha(alt, .88))
	case 2: // cluster: a small constellation of mixed shapes
		lines(p, r, w, h, alpha(on, .08), h/8)
		for i, it := range spec.cluster {
			colour := []ui.Color{fill, alt, alpha(on, .22)}[it.colour]
			radii, d := ShapeRadii(it.shape), it.d
			if i == 1 {
				radii, d = mainRadii, it.d*(1+.12*t)
			}
			dir := 1.0
			if i%2 == 1 {
				dir = -1
			}
			fillPath(ShapePath(radii, sq(w*it.x, h*it.y, h*d), spec.rot*float64(i+1)+turn*dir), colour)
		}
	default: // peek: a huge form bleeding off the corner with a nested echo
		lines(p, r, w*.55, h*.55, alpha(on, .1), h/10)
		big := sq(w*.86, h*.98, h*1.7)
		fillPath(ShapePath(mainRadii, big, spec.rot+turn+t*.3), fill)
		echo := ui.Rect{X: big.X + big.W*.22, Y: big.Y + big.H*.22, W: big.W * .56, H: big.H * .56}
		fillPath(ShapePath(mainRadii, echo, spec.rot-turn+t*.9), alpha(bg, .9))
		fillPath(ShapePath(ShapeRadii(spec.accent), sq(w*.2, h*.32, h*.26), spec.accentRot+turn), alt)
	}
}

func dots(p *ui.Painter, r ui.Rect, c ui.Color, gap float64) {
	for y := float64(r.Y); y < float64(r.Y+r.H); y += gap {
		for x := float64(r.X); x < float64(r.X+r.W); x += gap {
			var circle ui.Path
			circle.Circle(float32(x), float32(y), float32(gap*.13))
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

// lines draws diagonal hatching over a w×h area from r's corner.
func lines(p *ui.Painter, r ui.Rect, w, h float64, c ui.Color, gap float64) {
	for x := -h; x < w; x += gap {
		var l ui.Path
		l.MoveTo(r.X+float32(x), r.Y+float32(h))
		l.LineTo(r.X+float32(x+h), r.Y)
		p.StrokePath(&l, 1.2, c)
	}
}

// AssetBadges marks the local artifacts of a lecture.
func AssetBadges(c *ui.Context, assets []string, segments int) {
	s := design.S()
	has := func(k string) bool {
		for _, a := range assets {
			if a == k {
				return true
			}
		}
		return false
	}
	type item struct {
		icon  icons.Icon
		label string
	}
	var items []item
	if has("video") {
		items = append(items, item{icons.MovieOutlined, l10n.AssetVideo})
	}
	if has("slides") {
		items = append(items, item{icons.SlideshowOutlined, "PPT"})
	}
	if has("courseware") {
		items = append(items, item{icons.AttachFileRounded, l10n.AssetCourseware})
	}
	if segments > 0 || has("transcript") {
		items = append(items, item{icons.SubjectRounded, l10n.AssetTranscript})
	}
	if len(items) == 0 {
		return
	}
	ui.Row(c).Wrap().Gap(design.SpaceXS).Children(func() {
		for _, it := range items {
			b := ui.Row(c).Padding(2, 6).Background(s.TertiaryContainer).Radius(design.CornerXS).AlignItems(ui.Center).Children(func() {
				m3.Icon(c, it.icon, 12, s.OnTertiaryContainer)
				m3.HGap(c, 3)
				m3.Text(c, it.label, design.TT().LabelSmall.C(s.OnTertiaryContainer))
			})
			m3.Tip(c, b, l10n.CourseDownloadedKind(it.label))
		}
	})
}

// JobProgressLine is a job's progress bar with its kind, stage and speed.
func JobProgressLine(c *ui.Context, job core.Job) ui.Element {
	s := design.S()
	parts := []string{core.KindLabel(job.Kind())}
	if job.Stage() != "" {
		parts = append(parts, job.Stage())
	}
	if job.Speed() > 0 {
		parts = append(parts, core.FmtSpeed(float64(job.Speed())))
	}
	return ui.Column(c).FillWidth().Children(func() {
		motion := design.T().Motion(design.Medium2, design.Standard)
		pr := m3.Progress{Radius: 4, Motion: &motion}
		if job.State() != "queued" {
			pr.Value = m3.Of(job.Progress())
		}
		m3.Linear(c, pr)
		m3.VGap(c, 4)
		m3.Clamp(m3.Text(c, strings.Join(parts, " · "), design.TT().LabelSmall.C(s.OnSurfaceVariant)), 1)
	})
}
