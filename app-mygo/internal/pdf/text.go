package pdf

import "strings"

// Span is a run of rich text in one style, or an inline box when Box is
// set.
type Span struct {
	Text string
	// Face sets the text; Fallback the characters Face lacks.
	Face, Fallback *Face
	Size           float64
	Color          Color
	// Background, when set, fills behind the run.
	Background *Color
	// Link goes to an anchor of the document; URI opens a link.
	Link, URI string
	Box       *Box
}

// Box is an inline object: a picture or a widget drawn by Draw with its
// baseline at y.
type Box struct {
	W, Ascent, Descent float64
	Draw               func(p *Page, x, y float64)
}

type glyph struct {
	r      rune
	span   int
	face   *Face
	w      float64
	breaks bool // a line may break before it
	space  bool
}

// Line is a laid out line of rich text.
type Line struct {
	Ascent, Descent, Width float64
	spans                  []Span
	glyphs                 []glyph
}

// Height is the line's height.
func (l Line) Height() float64 { return l.Ascent + l.Descent }

// Lines measures lines with spacing between them.
func Lines(lines []Line, spacing float64) float64 {
	h := 0.0
	for i, l := range lines {
		h += l.Height()
		if i > 0 {
			h += spacing
		}
	}
	return h
}

// noStart are the characters a line does not begin with.
const noStart = "，。、；：？！）」』】》〉”’…·%,.;:?!)]}"

func wide(r rune) bool {
	return r >= 0x2e80 && r <= 0x9fff || r >= 0xac00 && r <= 0xd7af || r >= 0xf900 && r <= 0xfaff || r >= 0xfe30 && r <= 0xfe4f || r >= 0xff00 && r <= 0xffef
}

// Layout breaks spans into lines width wide. A line breaks after a space,
// around wide characters and at a newline; a word wider than a line
// breaks anywhere. strut is the ascent and descent of an empty line.
func Layout(spans []Span, width float64, strut [2]float64) []Line {
	var glyphs []glyph
	var prev rune
	for i, s := range spans {
		if s.Box != nil {
			glyphs = append(glyphs, glyph{span: i, w: s.Box.W, breaks: true})
			prev = 0xfffc
			continue
		}
		for _, r := range strings.ReplaceAll(s.Text, "\t", "    ") {
			face := s.Face
			if !face.Has(r) && s.Fallback != nil && s.Fallback.Has(r) {
				face = s.Fallback
			}
			g := glyph{r: r, span: i, face: face, w: face.Advance(r) * s.Size, space: r == ' '}
			g.breaks = prev == ' ' || prev == 0xfffc || (wide(r) || wide(prev) || prev == '-' || prev == '/') && !strings.ContainsRune(noStart, r)
			glyphs = append(glyphs, g)
			prev = r
		}
	}
	var lines []Line
	emit := func(gs []glyph) {
		for len(gs) > 0 && gs[len(gs)-1].space {
			gs = gs[:len(gs)-1]
		}
		l := Line{spans: spans, glyphs: gs}
		for _, g := range gs {
			s := spans[g.span]
			asc, desc := 0.0, 0.0
			if s.Box != nil {
				asc, desc = s.Box.Ascent, s.Box.Descent
			} else {
				asc, desc = g.face.Ascent()*s.Size, g.face.Descent()*s.Size
			}
			l.Ascent, l.Descent = max(l.Ascent, asc), max(l.Descent, desc)
			l.Width += g.w
		}
		if len(gs) == 0 {
			l.Ascent, l.Descent = strut[0], strut[1]
		}
		lines = append(lines, l)
	}
	start, x, last := 0, 0.0, -1
	for i := 0; i < len(glyphs); i++ {
		g := glyphs[i]
		if g.r == '\n' && spans[g.span].Box == nil {
			emit(glyphs[start:i])
			start, x, last = i+1, 0, -1
			continue
		}
		if i > start && g.breaks {
			last = i
		}
		if x+g.w > width && i > start && !g.space {
			cut := i
			if last > start {
				cut = last
			}
			emit(glyphs[start:cut])
			for cut < len(glyphs) && glyphs[cut].space {
				cut++
			}
			start, last, x = cut, -1, 0
			i = cut - 1
			continue
		}
		x += g.w
	}
	emit(glyphs[start:])
	return lines
}

// Draw sets the line from x with its top at y.
func (l Line) Draw(p *Page, x, y float64) {
	base := y + l.Ascent
	for i := 0; i < len(l.glyphs); {
		g := l.glyphs[i]
		s := l.spans[g.span]
		if s.Box != nil {
			s.Box.Draw(p, x, base)
			if s.Link != "" {
				p.Link(x, base-s.Box.Ascent, g.w, s.Box.Ascent+s.Box.Descent, s.Link)
			}
			x += g.w
			i++
			continue
		}
		j, w := i, 0.0
		var run strings.Builder
		for ; j < len(l.glyphs) && l.glyphs[j].span == g.span && l.glyphs[j].face == g.face; j++ {
			run.WriteRune(l.glyphs[j].r)
			w += l.glyphs[j].w
		}
		if s.Background != nil {
			p.Rect(x, base-g.face.Ascent()*s.Size, w, (g.face.Ascent()+g.face.Descent())*s.Size, *s.Background)
		}
		p.Text(g.face, s.Size, x, base, s.Color, run.String())
		top, h := base-g.face.Ascent()*s.Size, (g.face.Ascent()+g.face.Descent())*s.Size
		if s.Link != "" {
			p.Link(x, top, w, h, s.Link)
		}
		if s.URI != "" {
			p.URI(x, top, w, h, s.URI)
		}
		x += w
		i = j
	}
}
