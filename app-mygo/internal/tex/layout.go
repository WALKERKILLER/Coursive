package tex

import (
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

type opKind uint8

const (
	opGlyph opKind = iota
	opRect
	opQuad
	opText
)

// drawOp is one thing a formula draws, relative to its origin on the
// baseline, y down: a glyph at (x, y) on its baseline, a rectangle from
// (x, y), a quadrilateral, or text the system shaped.
type drawOp struct {
	kind   opKind
	x, y   float32
	w, h   float32
	quad   [8]float32
	g      gid
	size   float32
	glyphs []ui.Glyph
	color  string
	// part marks a glyph of an assembly, whose parts overlap: they fill
	// as one path, so that the overlaps show no seams.
	part bool
}

// box is laid-out math: its width, height above and depth below the
// baseline, in DIPs.
type box struct {
	w, h, d float32
	ops     []drawOp
	italic  float32
	accent  float32
	char    bool
	hasAcc  bool
}

func (b *box) place(c box, x, y float32) {
	for _, o := range c.ops {
		o.x += x
		o.y += y
		if o.kind == opQuad {
			for i := 0; i < 8; i += 2 {
				o.quad[i] += x
				o.quad[i+1] += y
			}
		}
		b.ops = append(b.ops, o)
	}
}

// hbox puts boxes side by side on one baseline.
func hbox(bs ...box) box {
	var out box
	for _, b := range bs {
		out.place(b, out.w, 0)
		out.w += b.w
		out.h = max(out.h, b.h)
		out.d = max(out.d, b.d)
	}
	if len(bs) > 0 {
		out.italic = bs[len(bs)-1].italic
	}
	return out
}

// shifted moves b up by up.
func shifted(b box, up float32) box {
	var out box
	out.place(b, 0, -up)
	out.w, out.h, out.d = b.w, b.h+up, b.d-up
	out.italic = b.italic
	return out
}

func kern(w float32) box { return box{w: w} }

func (l *layout) rect(x, y, w, h float32) drawOp {
	return drawOp{kind: opRect, x: x, y: y, w: w, h: h, color: l.color}
}

func (l *layout) quad(pts ...float32) drawOp {
	o := drawOp{kind: opQuad, color: l.color}
	copy(o.quad[:], pts)
	return o
}

// style is TeX's: display, text, script, scriptscript; cramped.
type style struct {
	level   int
	cramped bool
}

func (s style) sup() style { return style{min(max(s.level, 1)+1, 3), s.cramped} }
func (s style) sub() style { return style{min(max(s.level, 1)+1, 3), true} }
func (s style) num() style { return style{min(s.level+1, 3), s.cramped} }
func (s style) den() style { return style{min(s.level+1, 3), true} }
func (s style) crampedOf() style {
	s.cramped = true
	return s
}

type layout struct {
	f      *mathFont
	size   float32
	color  string
	middle float32
	tag    *box
}

func (l *layout) em(st style) float32 {
	switch st.level {
	case 2:
		return l.size * l.f.c.scriptScale
	case 3:
		return l.size * l.f.c.scriptScriptScale
	}
	return l.size
}

func (l *layout) glyphBox(g gid, st style) box {
	em := l.em(st)
	w, h, d := l.f.metrics(g)
	b := box{w: w * em, h: h * em, d: d * em, char: true}
	b.ops = []drawOp{{kind: opGlyph, g: g, size: em, color: l.color}}
	b.italic = l.f.italics[g] * em
	if a, ok := l.f.accents[g]; ok {
		b.accent, b.hasAcc = a*em, true
	} else {
		b.accent = b.w / 2
	}
	return b
}

// textBox shapes s with the system's fonts.
func (l *layout) textBox(s string, st style, bold, ital, bad bool) box {
	em := l.em(st)
	if !bold && !ital && !bad {
		// Latin text in the font's own roman, as TeX's \text.
		var bs []box
		ok := true
		for _, r := range s {
			if r == ' ' {
				bs = append(bs, kern(em*.333))
				continue
			}
			g, has := l.f.glyph(r)
			if !has || r > 0x24F {
				ok = false
				break
			}
			b := l.glyphBox(g, st)
			b.italic = 0
			bs = append(bs, b)
		}
		if ok {
			b := hbox(bs...)
			b.char = false
			return b
		}
	}
	font := ui.Font{Size: em, Italic: ital}
	if bold {
		font.Weight = 700
	}
	gs := ui.Shape(s, font)
	var w float32
	for _, g := range gs {
		w = max(w, g.X+g.Advance)
	}
	color := l.color
	if bad {
		color = "error"
	}
	return box{w: w, h: em * .78, d: em * .22, ops: []drawOp{{kind: opText, glyphs: gs, color: color}}}
}

// symBox lays a character out in its math alphabet.
func (l *layout) symBox(s *sym, st style) box {
	var bs []box
	for _, r := range s.text {
		r = mathAlpha(r, s.font)
		g, ok := l.f.glyph(r)
		if !ok {
			return l.textBox(s.text, st, false, false, false)
		}
		bs = append(bs, l.glyphBox(g, st))
	}
	if len(bs) == 1 {
		return bs[0]
	}
	return hbox(bs...)
}

// Spaces between atoms, in mus: thin 3, medium 4, thick 5; negative only
// in display and text styles.
var spacing = [8][8]int8{
	Ord:   {0, 3, -4, -5, 0, 0, 0, -3},
	Op:    {3, 3, 0, -5, 0, 0, 0, -3},
	Bin:   {-4, -4, 0, 0, -4, 0, 0, -4},
	Rel:   {-5, -5, 0, 0, -5, 0, 0, -5},
	Open:  {0, 0, 0, 0, 0, 0, 0, 0},
	Close: {0, 3, -4, -5, 0, 0, 0, -3},
	Punct: {-3, -3, 0, -3, -3, -3, -3, -3},
	Inner: {-3, 3, -4, -5, -3, 0, -3, -3},
}

type item struct {
	cls  class
	b    box
	kern bool
}

func (l *layout) list(items []node, st style) box {
	saved := l.color
	defer func() { l.color = saved }()
	var its []item
	for _, n := range items {
		switch n := n.(type) {
		case *styleSet:
			st = style{n.style, st.cramped}
			continue
		case *colorSet:
			l.color = n.color
			continue
		case *space:
			em := l.em(st)
			its = append(its, item{b: kern(em * (n.em + n.mu/18)), kern: true})
			continue
		case *tagNode:
			b := l.list(n.body, style{level: 1})
			l.tag = &b
			continue
		}
		cls, b := l.atom(n, st)
		its = append(its, item{cls: cls, b: b})
	}
	// A binary operator with nothing to join is ordinary.
	prev := -1
	for i := range its {
		if its[i].kern {
			continue
		}
		if its[i].cls == Bin {
			if prev < 0 {
				its[i].cls = Ord
			} else {
				switch its[prev].cls {
				case Bin, Op, Rel, Open, Punct:
					its[i].cls = Ord
				}
			}
		}
		if prev >= 0 && its[prev].cls == Bin {
			switch its[i].cls {
			case Rel, Close, Punct:
				its[prev].cls = Ord
			}
		}
		prev = i
	}
	if prev >= 0 && its[prev].cls == Bin {
		its[prev].cls = Ord
	}
	mu := l.em(st) / 18
	var bs []box
	last := -1
	for i, it := range its {
		if it.kern {
			bs = append(bs, it.b)
			continue
		}
		if last >= 0 {
			s := spacing[its[last].cls][it.cls]
			if s < 0 && st.level < 2 {
				s = -s
			} else if s < 0 {
				s = 0
			}
			if s > 0 {
				bs = append(bs, kern(float32(s)*mu))
			}
		}
		bs = append(bs, it.b)
		last = i
	}
	out := hbox(bs...)
	if len(its) == 1 && !its[0].kern {
		out.char, out.accent, out.hasAcc = its[0].b.char, its[0].b.accent, its[0].b.hasAcc
		out.italic = its[0].b.italic
	} else {
		out.accent = out.w / 2
	}
	return out
}

func (l *layout) atom(n node, st style) (class, box) {
	switch n := n.(type) {
	case *sym:
		return n.cls, l.symBox(n, st)
	case *group:
		return n.cls, l.list(n.items, st)
	case *scriptsHolder:
		return Op, l.list(n.body.items, st)
	case *textRun:
		return Ord, l.textBox(n.s, st, n.bold, n.ital, n.bad)
	case *frac:
		return Inner, l.frac(n, st)
	case *radical:
		return Ord, l.radical(n, st)
	case *scripts:
		return l.scripts(n, st)
	case *opNode:
		b, _ := l.op(n, st)
		return Op, b
	case *leftRight:
		return Inner, l.leftRight(n, st)
	case *middle:
		return Ord, l.delimiter(n.delim, l.middle, st)
	case *bigDelim:
		sizes := [5]float32{0, 1.2, 1.8, 2.4, 3.0}
		return n.cls, l.delimiter(n.delim, sizes[n.size]*l.em(st), st)
	case *accent:
		return Ord, l.accent(n, st)
	case *bar:
		return Ord, l.bar(n, st)
	case *array:
		return Ord, l.array(n, st)
	case *boxed:
		b := l.list(n.body, st)
		em := l.em(st)
		pad, t := .3*em, .04*em
		out := box{w: b.w + 2*pad + 2*t, h: b.h + pad + t, d: b.d + pad + t}
		out.place(b, pad+t, 0)
		out.ops = append(out.ops, l.rect(0, -out.h, out.w, t), l.rect(0, out.d-t, out.w, t),
			l.rect(0, -out.h, t, out.h+out.d), l.rect(out.w-t, -out.h, t, out.h+out.d))
		return Ord, out
	case *phantom:
		b := l.list(n.body, st)
		out := box{}
		if n.w {
			out.w = b.w
		}
		if n.v {
			out.h, out.d = b.h, b.d
		}
		return Ord, out
	case *smash:
		b := l.list(n.body, st)
		b.h, b.d = 0, 0
		return Ord, b
	case *stack:
		base := l.list(n.base, st)
		return n.cls, l.limits(base, l.list(n.over, st.sup()), l.list(n.under, st.sub()), len(n.over) > 0, len(n.under) > 0, st, 0)
	case *stretchy:
		return Rel, l.xarrow(n, st)
	case *brace:
		return Op, l.brace(n, st)
	case *cancel:
		b := l.list(n.body, st)
		t := .05 * l.em(st)
		line := func(x0, y0, x1, y1 float32) {
			b.ops = append(b.ops, l.quad(x0-t/2, y0, x0+t/2, y0, x1+t/2, y1, x1-t/2, y1))
		}
		switch n.kind {
		case 0:
			line(0, b.d, b.w, -b.h)
		case 1:
			line(0, -b.h, b.w, b.d)
		case 2:
			line(0, b.d, b.w, -b.h)
			line(0, -b.h, b.w, b.d)
		case 3:
			a := l.f.c.axisHeight * l.em(st)
			b.ops = append(b.ops, l.rect(0, -a-t/2, b.w, t))
		}
		return Ord, b
	case *not:
		cls, b := l.atom(n.body, st)
		if n.body == nil {
			cls, b = Rel, box{w: .3 * l.em(st)}
		}
		em := l.em(st)
		t := .05 * em
		top, bot := -max(b.h, .7*em), max(b.d, .2*em)
		cx := b.w / 2
		b.ops = append(b.ops, l.quad(cx-.15*em-t/2, bot, cx-.15*em+t/2, bot, cx+.15*em+t/2, top, cx+.15*em-t/2, top))
		return cls, b
	case *ruleNode:
		em := l.size
		b := box{w: n.w * em, h: n.h * em}
		b.ops = []drawOp{l.rect(0, -b.h, b.w, b.h)}
		return Ord, b
	}
	return Ord, box{}
}

// mathAlpha maps r into a math alphabet: letters italic by default, as
// TeX sets them, and Greek capitals upright.
func mathAlpha(r rune, f fontStyle) rune {
	if f == fontRoman {
		return r
	}
	latin := func(caps, small rune, holes map[rune]rune) rune {
		if h, ok := holes[r]; ok {
			return h
		}
		switch {
		case r >= 'A' && r <= 'Z':
			return caps + r - 'A'
		case r >= 'a' && r <= 'z':
			return small + r - 'a'
		}
		return 0
	}
	greek := func(base rune) rune {
		switch {
		case r >= 0x391 && r <= 0x3A9:
			return base + r - 0x391
		case r == 0x2207:
			return base + 25
		case r >= 0x3B1 && r <= 0x3C9:
			return base + 26 + r - 0x3B1
		}
		if i := map[rune]rune{0x2202: 51, 0x3F5: 52, 0x3D1: 53, 0x3F0: 54, 0x3D5: 55, 0x3F1: 56, 0x3D6: 57}[r]; i != 0 {
			return base + i
		}
		return 0
	}
	digit := func(base rune) rune {
		if r >= '0' && r <= '9' {
			return base + r - '0'
		}
		return 0
	}
	var out rune
	switch f {
	case fontDefault:
		out = latin(0x1D434, 0x1D44E, map[rune]rune{'h': 0x210E})
		if out == 0 && (r >= 0x3B1 && r <= 0x3C9 || r == 0x3F5 || r == 0x3D1 || r == 0x3F0 || r == 0x3D5 || r == 0x3F1 || r == 0x3D6) {
			out = greek(0x1D6E2)
		}
	case fontItalic:
		out = latin(0x1D434, 0x1D44E, map[rune]rune{'h': 0x210E})
		if out == 0 {
			out = greek(0x1D6E2)
		}
	case fontBold:
		out = latin(0x1D400, 0x1D41A, nil)
		if out == 0 {
			out = greek(0x1D6A8)
		}
		if out == 0 {
			out = digit(0x1D7CE)
		}
	case fontBoldItalic:
		out = latin(0x1D468, 0x1D482, nil)
		if out == 0 {
			out = greek(0x1D71C)
		}
		if out == 0 {
			out = digit(0x1D7CE)
		}
	case fontScript:
		out = latin(0x1D49C, 0x1D4B6, map[rune]rune{'B': 0x212C, 'E': 0x2130, 'F': 0x2131, 'H': 0x210B, 'I': 0x2110,
			'L': 0x2112, 'M': 0x2133, 'R': 0x211B, 'e': 0x212F, 'g': 0x210A, 'o': 0x2134})
	case fontFraktur:
		out = latin(0x1D504, 0x1D51E, map[rune]rune{'C': 0x212D, 'H': 0x210C, 'I': 0x2111, 'R': 0x211C, 'Z': 0x2128})
	case fontDouble:
		out = latin(0x1D538, 0x1D552, map[rune]rune{'C': 0x2102, 'H': 0x210D, 'N': 0x2115, 'P': 0x2119, 'Q': 0x211A, 'R': 0x211D, 'Z': 0x2124})
		if out == 0 {
			out = digit(0x1D7D8)
		}
	case fontSans:
		out = latin(0x1D5A0, 0x1D5BA, nil)
		if out == 0 {
			out = digit(0x1D7E2)
		}
	case fontSansBold:
		out = latin(0x1D5D4, 0x1D5EE, nil)
		if out == 0 {
			out = digit(0x1D7EC)
		}
	case fontMono:
		out = latin(0x1D670, 0x1D68A, nil)
		if out == 0 {
			out = digit(0x1D7F6)
		}
	}
	if out == 0 {
		return r
	}
	return out
}

// singleRune is s's one rune, 0 when it has more.
func singleRune(s string) rune {
	r, n := utf8.DecodeRuneInString(s)
	if n != len(s) {
		return 0
	}
	return r
}
