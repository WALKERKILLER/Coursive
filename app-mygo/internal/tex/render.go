package tex

import (
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"
	ot "github.com/go-text/typesetting/font/opentype"
)

// Formula is typeset math: Width, and Height above and Depth below its
// baseline, in DIPs.
type Formula struct {
	Width, Height, Depth float32
	// Unknown lists the commands it did not know, which it shows as
	// written.
	Unknown []string

	ops   []drawOp
	paths map[string]*ui.Path
}

type key struct {
	tex     string
	size    float32
	display bool
}

var (
	cacheMu sync.Mutex
	cache   = map[key]*Formula{}
)

// Typeset lays tex out at size DIPs per em, in display or text style. It
// keeps what it typeset, as views typeset the same formulas every frame.
func Typeset(tex string, size float32, display bool) *Formula {
	k := key{tex, size, display}
	cacheMu.Lock()
	f, ok := cache[k]
	cacheMu.Unlock()
	if ok {
		return f
	}
	f = typeset(tex, size, display)
	cacheMu.Lock()
	if len(cache) > 2000 {
		clear(cache)
	}
	cache[k] = f
	cacheMu.Unlock()
	return f
}

func typeset(tex string, size float32, display bool) *Formula {
	nodes, bad := parse(tex)
	l := &layout{f: font(), size: size}
	st := style{level: 1}
	if display {
		st.level = 0
	}
	b := l.list(nodes, st)
	if l.tag != nil {
		b = hbox(b, kern(2*size), *l.tag)
	}
	return &Formula{Width: b.w, Height: b.h, Depth: b.d, Unknown: bad, ops: b.ops}
}

// Draw draws the formula with its baseline's left end at (x, y), in color
// where the TeX sets none. As the Flutter client's val_latex, it draws
// the glyphs as text, which the text engine renders more finely than a
// path fill, and puts its origin and its thin rules (fraction bars,
// overbars, table lines) on the device pixels, so that a rule shows as
// one crisp line rather than a grey smear over two.
func (f *Formula) Draw(p *ui.Painter, x, y float32, color ui.Color) {
	if f.paths == nil {
		f.build()
	}
	s := p.Scale()
	if s > 0 {
		x, y = round(x*s)/s, round(y*s)/s
	}
	p.Transform(ui.Translation(x, y), 0, 0, func() {
		for name, path := range f.paths {
			p.FillPath(path, colorOf(name, color))
		}
	})
	text := registerGlyphs()
	for _, o := range f.ops {
		c := colorOf(o.color, color)
		switch {
		case o.kind == opText:
			p.Glyphs(o.glyphs, x+o.x, y+o.y, c)
		case o.kind == opRect:
			rx, rw := rule(x+o.x, o.w, s)
			ry, rh := rule(y+o.y, o.h, s)
			p.Fill(ui.Rect{X: rx, Y: ry, W: rw, H: rh}, c, 0)
		case o.kind == opGlyph && !o.part && text:
			p.Glyphs(glyphRun(o.g, o.size), x+o.x, y+o.y, c)
		}
	}
}

// rule is one axis of a rule, from start for length DIPs, on the device
// pixels when it is thin (under four of them): its length rounds to
// whole pixels, at least one, about its centre. Longer ones stay, so
// that a bar still meets the glyph it joins exactly.
func rule(start, length, scale float32) (float32, float32) {
	dev := length * scale
	if scale <= 0 || dev <= 0 || dev >= 4 {
		return start, length
	}
	thick := max(1, round(dev))
	top := round((start+length/2)*scale - thick/2)
	return top / scale, thick / scale
}

func round(v float32) float32 { return float32(math.Round(float64(v))) }

// glyphFamily is the family the font's copy that draws glyphs by id is
// registered under (glyphFont).
const glyphFamily = "Coursive TeX Glyphs"

var (
	glyphOnce  sync.Once
	glyphReady bool
	glyphMu    sync.Mutex
	glyphRuns  = map[glyphKey][]ui.Glyph{}
)

type glyphKey struct {
	g    gid
	size float32
}

// Register adds the font that draws the glyphs of formulas as text; call
// it before the first frame. Without it, Draw fills their outlines.
func Register() { registerGlyphs() }

func registerGlyphs() bool {
	glyphOnce.Do(func() {
		data, _, err := glyphFont(fontData)
		glyphReady = err == nil && ui.RegisterFont(data, glyphFamily) == nil
	})
	return glyphReady
}

// glyphRun is glyph g at size DIPs per em, shaped once.
func glyphRun(g gid, size float32) []ui.Glyph {
	k := glyphKey{g, size}
	glyphMu.Lock()
	run, ok := glyphRuns[k]
	glyphMu.Unlock()
	if ok {
		return run
	}
	run = ui.Shape(string(rune(privateUse+int(g))), ui.Font{Family: glyphFamily, Size: size})
	glyphMu.Lock()
	if len(glyphRuns) > 8192 {
		clear(glyphRuns)
	}
	glyphRuns[k] = run
	glyphMu.Unlock()
	return run
}

// build gathers what fills as paths, by colour: the quadrilaterals, the
// parts of assemblies, and every glyph when the font that draws them as
// text did not register.
func (f *Formula) build() {
	fnt := font()
	text := registerGlyphs()
	f.paths = map[string]*ui.Path{}
	path := func(name string) *ui.Path {
		p := f.paths[name]
		if p == nil {
			p = &ui.Path{}
			f.paths[name] = p
		}
		return p
	}
	for _, o := range f.ops {
		switch o.kind {
		case opQuad:
			q := o.quad
			path(o.color).MoveTo(q[0], q[1]).LineTo(q[2], q[3]).LineTo(q[4], q[5]).LineTo(q[6], q[7]).Close()
		case opGlyph:
			if text && !o.part {
				continue
			}
			p := path(o.color)
			k := o.size / fnt.upem
			pt := func(s ot.SegmentPoint) (float32, float32) { return o.x + s.X*k, o.y - s.Y*k }
			open := false
			for _, s := range fnt.outline(o.g) {
				switch s.Op {
				case ot.SegmentOpMoveTo:
					if open {
						p.Close()
					}
					p.MoveTo(pt(s.Args[0]))
					open = true
				case ot.SegmentOpLineTo:
					p.LineTo(pt(s.Args[0]))
				case ot.SegmentOpQuadTo:
					cx, cy := pt(s.Args[0])
					x, y := pt(s.Args[1])
					p.QuadTo(cx, cy, x, y)
				case ot.SegmentOpCubeTo:
					c1x, c1y := pt(s.Args[0])
					c2x, c2y := pt(s.Args[1])
					x, y := pt(s.Args[2])
					p.CubeTo(c1x, c1y, c2x, c2y, x, y)
				}
			}
			if open {
				p.Close()
			}
		}
	}
}

var namedColors = map[string]ui.Color{
	"black": ui.RGB(0, 0, 0), "white": ui.RGB(255, 255, 255), "red": ui.RGB(237, 28, 36), "green": ui.RGB(0, 166, 81),
	"blue": ui.RGB(46, 49, 146), "cyan": ui.RGB(0, 174, 239), "magenta": ui.RGB(236, 0, 140), "yellow": ui.RGB(255, 242, 0),
	"orange": ui.RGB(247, 148, 29), "purple": ui.RGB(146, 39, 143), "brown": ui.RGB(121, 37, 0), "gray": ui.RGB(148, 149, 151),
	"grey": ui.RGB(148, 149, 151), "teal": ui.RGB(0, 130, 132), "violet": ui.RGB(88, 66, 155), "pink": ui.RGB(237, 145, 187),
	"lime": ui.RGB(141, 198, 63), "olive": ui.RGB(152, 158, 0), "darkgray": ui.RGB(85, 85, 85), "lightgray": ui.RGB(191, 191, 191),
	"error": ui.RGB(179, 38, 30),
}

func colorOf(name string, def ui.Color) ui.Color {
	if name == "" {
		return def
	}
	name = strings.TrimSpace(name)
	if c, ok := namedColors[strings.ToLower(name)]; ok {
		return c
	}
	hex := strings.TrimPrefix(name, "#")
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	if len(hex) == 6 {
		if v, err := strconv.ParseUint(hex, 16, 32); err == nil {
			return ui.RGB(uint8(v>>16), uint8(v>>8), uint8(v))
		}
	}
	return def
}
