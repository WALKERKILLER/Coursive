package study

import (
	"fmt"
	"image"
	"math"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/pdf"
)

// OutlinePaper is the paper an outline PDF is set on.
type OutlinePaper int

const (
	PaperA4 OutlinePaper = iota
	PaperLetter
)

// OutlineLayout is the page an outline PDF is laid out on (spec 0028).
type OutlineLayout struct {
	Paper              OutlinePaper
	Landscape, Compact bool
}

const mm = 72 / 25.4

// Size is the page's width and height in points.
func (l OutlineLayout) Size() (float64, float64) {
	w, h := 210*mm, 297*mm
	if l.Paper == PaperLetter {
		w, h = 612, 792
	}
	if l.Landscape {
		return h, w
	}
	return w, h
}

// Margin is the margin on every side.
func (l OutlineLayout) Margin() float64 {
	if l.Compact {
		return 12 * mm
	}
	return 20 * mm
}

// OutlineTextBlock is a piece of an item's text: a paragraph, code, a
// display formula or a table.
type OutlineTextBlock interface{ outlineTextBlock() }

type (
	OutlineParagraph struct{ Text string }
	OutlineCode      struct{ Lines []string }
	OutlineMath      struct{ Tex string }
	OutlineTable     struct{ Rows [][]string }
)

func (OutlineParagraph) outlineTextBlock() {}
func (OutlineCode) outlineTextBlock()      {}
func (OutlineMath) outlineTextBlock()      {}
func (OutlineTable) outlineTextBlock()     {}

var tableRule = regexp.MustCompile(`^\s*\|?\s*:?-{3,}:?\s*(\|\s*:?-{3,}:?\s*)*\|?\s*$`)

func tableCells(line string) []string {
	l := strings.TrimSpace(line)
	l = strings.TrimPrefix(l, "|")
	l = strings.TrimSuffix(l, "|")
	cells := strings.Split(l, "|")
	for i, c := range cells {
		cells[i] = strings.TrimSpace(c)
	}
	return cells
}

// OutlineTextBlocks splits the Markdown an item may carry into blocks;
// inline marks stay in the text.
func OutlineTextBlocks(text string) []OutlineTextBlock {
	var out []OutlineTextBlock
	lines := strings.Split(text, "\n")
	var para []string
	flush := func() {
		if len(para) > 0 {
			out = append(out, OutlineParagraph{strings.Join(para, "\n")})
		}
		para = nil
	}
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		t := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(t, "```"):
			flush()
			var code []string
			for i++; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```"); i++ {
				code = append(code, lines[i])
			}
			out = append(out, OutlineCode{code})
		case strings.HasPrefix(t, "$$") || strings.HasPrefix(t, `\[`):
			flush()
			closer := `\]`
			if strings.HasPrefix(t, "$$") {
				closer = "$$"
			}
			body := t[2:]
			for !strings.HasSuffix(strings.TrimRight(body, " \t\r"), closer) && i+1 < len(lines) {
				i++
				body += "\n" + lines[i]
			}
			body = strings.TrimRight(body, " \t\r\n")
			body = strings.TrimSuffix(body, closer)
			if b := strings.TrimSpace(body); b != "" {
				out = append(out, OutlineMath{b})
			}
		case strings.Contains(t, "|") && i+1 < len(lines) && tableRule.MatchString(lines[i+1]):
			flush()
			rows := [][]string{tableCells(line)}
			i++
			for i+1 < len(lines) && strings.Contains(lines[i+1], "|") {
				i++
				rows = append(rows, tableCells(lines[i]))
			}
			out = append(out, OutlineTable{rows})
		case t == "":
			flush()
		default:
			para = append(para, t)
		}
	}
	flush()
	return out
}

// OutlineFormula is a formula of an outline, drawn before layout.
type OutlineFormula struct {
	Tex     string
	Display bool
}

// OutlineFormulas are the formulas of doc, each once.
func OutlineFormulas(doc OutlineDoc) []OutlineFormula {
	var out []OutlineFormula
	seen := map[OutlineFormula]bool{}
	add := func(f OutlineFormula) {
		if !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	for _, b := range doc.Blocks() {
		e, ok := b.(OutlineEntry)
		if !ok {
			continue
		}
		for _, t := range OutlineTextBlocks(e.Text) {
			switch t := t.(type) {
			case OutlineMath:
				add(OutlineFormula{t.Tex, true})
			case OutlineParagraph:
				for _, r := range OutlineInline(t.Text) {
					if r.Kind == RunMath || r.Kind == RunDisplayMath {
						add(OutlineFormula{r.Text, r.Kind == RunDisplayMath})
					}
				}
			}
		}
	}
	return out
}

// OutlineMissingGlyphs are the characters of doc the embedded font
// cannot draw, in order; formulas are pictures.
func OutlineMissingGlyphs(doc OutlineDoc, has func(rune) bool) []string {
	var out []string
	seen := map[rune]bool{}
	check := func(s string) {
		for _, r := range s {
			if r > 0x20 && r != 0x7f && !has(r) && !seen[r] {
				seen[r] = true
				out = append(out, string(r))
			}
		}
	}
	check(doc.Title)
	for _, b := range doc.Blocks() {
		switch b := b.(type) {
		case OutlineHeading:
			check(b.Text)
		case OutlineRemark:
			check(b.Text)
		case OutlineEntry:
			check(b.Tag)
			for _, t := range OutlineTextBlocks(b.Text) {
				switch t := t.(type) {
				case OutlineParagraph:
					for _, r := range OutlineInline(t.Text) {
						if r.Kind != RunMath && r.Kind != RunDisplayMath {
							check(r.Text)
						}
					}
				case OutlineCode:
					check(strings.Join(t.Lines, ""))
				case OutlineTable:
					for _, row := range t.Rows {
						check(strings.Join(row, ""))
					}
				}
			}
		}
	}
	for _, s := range doc.Sources {
		check(s.Label + s.Locator + s.Quote)
	}
	return out
}

// OutlineImage is a formula or ink picture drawn ahead of layout, sized
// in points. Baseline is a formula's baseline from its top; ink has none
// and sets it negative.
type OutlineImage struct {
	Image    image.Image
	W, H     float64
	Baseline float64
}

// OutlineMathKey and OutlineInkKey name the pictures of a formula and of
// a mark's ink.
func OutlineMathKey(tex string, display bool) string {
	if display {
		return "D:" + tex
	}
	return "I:" + tex
}

func OutlineInkKey(item M) string { return "ink:" + str(item["id"]) }

var (
	pdfInk    = pdf.Hex(0xff1d2125)
	pdfMuted  = pdf.Hex(0xff69707a)
	pdfRule   = pdf.Hex(0xffdadde2)
	pdfAccent = pdf.Hex(0xff2c6770)
	pdfAmber  = pdf.Hex(0xffa15f0a)
	pdfPaper  = pdf.Hex(0xfff4f5f7)
)

// piece is laid out content: drawn whole, or between its rows when it is
// taller than a page and has them.
type piece struct {
	h    float64
	draw func(p *pdf.Page, x, y float64)
	rows []piece
	code bool
}

type tocEntry struct {
	title string
	level int
	page  *pdf.Page
	y     float64
}

type outlinePdf struct {
	doc           OutlineDoc
	d             *pdf.Doc
	regular, bold *pdf.Face
	images        map[string]OutlineImage
	placed        map[string]*pdf.Image
	w, h, margin  float64
	cw, ch        float64
	bottom        float64
	page          *pdf.Page
	y             float64
	content       []*pdf.Page
	toc           []tocEntry
	deep          bool
}

func (o *outlinePdf) span(text string, bold bool, size float64, c pdf.Color) pdf.Span {
	f := o.regular
	if bold {
		f = o.bold
	}
	return pdf.Span{Text: text, Face: f, Fallback: o.regular, Size: size, Color: c}
}

func (o *outlinePdf) mono(text string, size float64) pdf.Span {
	return pdf.Span{Text: text, Face: pdf.Courier, Fallback: o.regular, Size: size, Color: pdfInk}
}

func (o *outlinePdf) image(key string) *pdf.Image {
	if im := o.placed[key]; im != nil {
		return im
	}
	im := o.d.Image(o.images[key].Image)
	o.placed[key] = im
	return im
}

// para lays spans out width wide, spacing between lines; rows are its
// lines, each with the spacing above it.
func (o *outlinePdf) para(spans []pdf.Span, width, spacing float64) piece {
	strut := [2]float64{o.regular.Ascent() * 10.5, o.regular.Descent() * 10.5}
	if len(spans) > 0 && spans[0].Box == nil {
		strut = [2]float64{spans[0].Face.Ascent() * spans[0].Size, spans[0].Face.Descent() * spans[0].Size}
	}
	lines := pdf.Layout(spans, width, strut)
	p := piece{h: pdf.Lines(lines, spacing)}
	p.draw = func(pg *pdf.Page, x, y float64) {
		for _, l := range lines {
			l.Draw(pg, x, y)
			y += l.Height() + spacing
		}
	}
	for i, l := range lines {
		gap := 0.0
		if i > 0 {
			gap = spacing
		}
		p.rows = append(p.rows, piece{h: gap + l.Height(), draw: func(pg *pdf.Page, x, y float64) { l.Draw(pg, x, y+gap) }})
	}
	return p
}

func (o *outlinePdf) newPage() {
	o.page = o.d.AddPage(o.w, o.h)
	o.content = append(o.content, o.page)
	o.y = o.margin
}

// need starts a page unless h fits on this one: pw.NewPage(freeSpace).
func (o *outlinePdf) need(h float64) {
	if o.bottom-o.y < h && o.y > o.margin+.01 {
		o.newPage()
	}
}

// put places p at x; side draws what stands beside its top, side tall.
func (o *outlinePdf) put(p piece, x float64, side func(pg *pdf.Page, y float64), sideH float64) {
	if p.h <= o.bottom-o.margin || len(p.rows) == 0 {
		o.need(max(p.h, sideH))
		if side != nil {
			side(o.page, o.y)
		}
		p.draw(o.page, x, o.y)
		o.y += max(p.h, sideH)
		return
	}
	for i, r := range p.rows {
		o.need(r.h)
		if i == 0 && side != nil {
			side(o.page, o.y)
		}
		r.draw(o.page, x, o.y)
		o.y += r.h
	}
}

func (o *outlinePdf) inline(text string, size float64) []pdf.Span {
	var spans []pdf.Span
	for _, r := range OutlineInline(text) {
		switch r.Kind {
		case RunText:
			spans = append(spans, o.span(r.Text, false, size, pdfInk))
		case RunBold:
			spans = append(spans, o.span(r.Text, true, size, pdfInk))
		case RunCode:
			s := o.mono(r.Text, size-1)
			s.Background = &pdfPaper
			spans = append(spans, s)
		default:
			key := OutlineMathKey(r.Text, r.Kind == RunDisplayMath)
			img, ok := o.images[key]
			if !ok {
				spans = append(spans, o.mono(r.Text, 9))
				continue
			}
			// The formula sits on the baseline by its own descent.
			ascent := img.Baseline
			if ascent < 0 {
				ascent = img.H * .78
			}
			im := o.image(key)
			spans = append(spans, pdf.Span{Box: &pdf.Box{W: img.W, Ascent: ascent, Descent: img.H - ascent, Draw: func(p *pdf.Page, x, y float64) {
				p.Image(im, x, y-ascent, img.W, img.H)
			}}})
		}
	}
	return spans
}

func (o *outlinePdf) refs(numbers []int) []pdf.Span {
	var spans []pdf.Span
	for _, n := range numbers {
		label := fmt.Sprint(n)
		w := o.bold.Width(label, 6.8)
		lift := 3.5 + o.bold.Descent()*6.8
		spans = append(spans, pdf.Span{Link: fmt.Sprintf("source-%d", n), Box: &pdf.Box{W: 1.5 + w, Ascent: 3.5 + (o.bold.Ascent()+o.bold.Descent())*6.8, Draw: func(p *pdf.Page, x, y float64) {
			p.Text(o.bold, 6.8, x+1.5, y-lift, pdfAccent, label)
		}}})
	}
	return spans
}

func (o *outlinePdf) tag(text string, warn bool) []pdf.Span {
	if text == "" {
		return nil
	}
	c := pdfAccent
	if warn {
		c = pdfAmber
	}
	return []pdf.Span{o.span(text+"  ", true, 8.8, c)}
}

// code is code in slices of eight lines; long code breaks between them,
// never inside a line.
func (o *outlinePdf) code(lines []string, width float64) []piece {
	var slices [][]string
	for i := 0; i < len(lines); i += 8 {
		slices = append(slices, lines[i:min(i+8, len(lines))])
	}
	if len(slices) == 0 {
		slices = [][]string{{""}}
	}
	var out []piece
	for i, slice := range slices {
		top, bottom, r := 0.0, 0.0, [4]float64{}
		if i == 0 {
			top, r[0], r[1] = 6, 3, 3
		}
		if i == len(slices)-1 {
			bottom, r[2], r[3] = 6, 3, 3
		}
		text := o.para([]pdf.Span{o.mono(strings.Join(slice, "\n"), 9)}, width-16, 2.5)
		h := top + text.h + bottom
		out = append(out, piece{h: h, code: true, draw: func(p *pdf.Page, x, y float64) {
			p.RoundRect(x, y, width, h, r, &pdfPaper, nil, 0)
			text.draw(p, x+8, y+top)
		}})
	}
	return out
}

func (o *outlinePdf) table(rows [][]string, width float64) piece {
	cols := 0
	for _, r := range rows {
		cols = max(cols, len(r))
	}
	natural := make([]float64, cols)
	sum := 0.0
	for c := range natural {
		for i, r := range rows {
			if c < len(r) {
				natural[c] = max(natural[c], o.span(r[c], i == 0, 9.5, pdfInk).Face.Width(r[c], 9.5)+10)
			}
		}
		natural[c] = max(natural[c], 16)
		sum += natural[c]
	}
	var out piece
	for i, r := range rows {
		cells := make([]piece, cols)
		h := 0.0
		for c := range cols {
			text := ""
			if c < len(r) {
				text = r[c]
			}
			cells[c] = o.para([]pdf.Span{o.span(text, i == 0, 9.5, pdfInk)}, natural[c]*width/sum-10, 4.5)
			h = max(h, cells[c].h+6)
		}
		last, header := i == len(rows)-1, i == 0
		row := piece{h: h, draw: func(p *pdf.Page, x, y float64) {
			if header {
				p.Rect(x, y, width, h, pdfPaper)
			}
			p.Line(x, y, x+width, y, .5, pdfRule)
			cx := x
			for c, cell := range cells {
				cell.draw(p, cx+5, y+(h-cell.h)/2)
				cx += natural[c] * width / sum
			}
			if last {
				p.Line(x, y+h, x+width, y+h, .5, pdfRule)
			}
		}}
		out.rows = append(out.rows, row)
		out.h += h
	}
	out.draw = func(p *pdf.Page, x, y float64) {
		for _, r := range out.rows {
			r.draw(p, x, y)
			y += r.h
		}
	}
	return out
}

func (o *outlinePdf) block(b OutlineTextBlock, width float64) []piece {
	switch b := b.(type) {
	case OutlineParagraph:
		return []piece{o.para(o.inline(b.Text, 10.5), width, 4.5)}
	case OutlineCode:
		return o.code(b.Lines, width)
	case OutlineMath:
		key := OutlineMathKey(b.Tex, true)
		img, ok := o.images[key]
		if !ok {
			return []piece{o.para([]pdf.Span{o.mono(b.Tex, 9)}, width, 0)}
		}
		w := min(img.W, o.cw-20)
		h := img.H * w / img.W
		im := o.image(key)
		return []piece{{h: h + 8, draw: func(p *pdf.Page, x, y float64) {
			p.Image(im, x+(width-w)/2, y+4, w, h)
		}}}
	case OutlineTable:
		return []piece{o.table(b.Rows, width)}
	}
	return nil
}

// estimate is a rough height, enough to keep a short item and its
// formula together.
func (o *outlinePdf) estimate(b OutlineTextBlock) float64 {
	switch b := b.(type) {
	case OutlineParagraph:
		w := 0.0
		for _, r := range b.Text {
			if r < 0x2e80 {
				w += 5.8
			} else {
				w += 10.5
			}
		}
		return 15*math.Ceil(w/(o.cw-13)) + 5
	case OutlineCode:
		return 12.5*float64(len(b.Lines)) + 17
	case OutlineMath:
		if img, ok := o.images[OutlineMathKey(b.Tex, true)]; ok {
			return img.H + 13
		}
		return 27
	case OutlineTable:
		return 17*float64(len(b.Rows)) + 8
	}
	return 0
}

func (o *outlinePdf) entry(e OutlineEntry, appendix bool) {
	parts := OutlineTextBlocks(e.Text)
	warn := e.Tag == l10n.OutlineTagUnverified || e.Tag == l10n.OutlineTagCheck || e.Tag == l10n.OutlineTagUnconfirmed
	var ink *OutlineImage
	if e.Item != nil {
		if img, ok := o.images[OutlineInkKey(e.Item)]; ok {
			ink = &img
		}
	}
	var last OutlineTextBlock
	if len(parts) > 0 {
		last = parts[len(parts)-1]
	}
	// Very long paragraphs may continue on the next page.
	if p, ok := last.(OutlineParagraph); ok && len(parts) == 1 && utf8.RuneCountInString(p.Text) > 700 && ink == nil {
		o.y += 2
		spans := append([]pdf.Span{o.span("•  ", false, 10.5, pdfMuted)}, o.tag(e.Tag, warn)...)
		spans = append(append(spans, o.inline(p.Text, 10.5)...), o.refs(e.Refs)...)
		o.put(o.para(spans, o.cw, 4.5), o.margin, nil, 0)
		o.y += 5
		return
	}
	width := o.cw - 13
	first := width
	if ink != nil {
		first -= 62
	}
	var pieces []piece
	at := func() float64 {
		if len(pieces) == 0 {
			return first
		}
		return width
	}
	for i, p := range parts {
		if para, ok := p.(OutlineParagraph); ok {
			var spans []pdf.Span
			if i == 0 {
				spans = o.tag(e.Tag, warn)
			}
			spans = append(spans, o.inline(para.Text, 10.5)...)
			if i == len(parts)-1 {
				spans = append(spans, o.refs(e.Refs)...)
			}
			pieces = append(pieces, o.para(spans, at(), 4.5))
			continue
		}
		if i == 0 && e.Tag != "" {
			pieces = append(pieces, o.para(o.tag(e.Tag, warn), at(), 4.5))
		}
		pieces = append(pieces, o.block(p, at())...)
	}
	if len(pieces) == 0 {
		pieces = append(pieces, o.para(o.tag(e.Tag, warn), at(), 4.5))
	}
	if _, ok := last.(OutlineParagraph); !ok && len(e.Refs) > 0 {
		pieces = append(pieces, o.para(o.refs(e.Refs), width, 4.5))
	}
	if len(parts) > 1 {
		// A short item stays whole; a long one keeps its opening with
		// what follows. Estimates run short on mixed scripts.
		total := 0.0
		for _, p := range parts {
			total += o.estimate(p)
		}
		total *= 1.25
		if total > o.ch*.4 {
			total = min(1.25*(o.estimate(parts[0])+o.estimate(parts[1])), o.ch*.6)
		}
		o.need(total)
	}
	bullet := "•"
	if appendix {
		bullet = "○"
	}
	for i, p := range pieces {
		gap := 5.0
		if i < len(pieces)-1 && len(parts) > 1 && p.code && pieces[i+1].code {
			gap = 0
		}
		if i > 0 {
			o.put(p, o.margin+13, nil, 0)
			o.y += gap
			continue
		}
		x, sideH := o.margin+13, 0.0
		if ink != nil {
			x, sideH = x+62, 41
		}
		o.put(p, x, func(pg *pdf.Page, y float64) {
			pg.Text(o.regular, 10.5, o.margin, y+o.regular.Ascent()*10.5, pdfMuted, bullet)
			if ink == nil {
				return
			}
			bx, by := o.margin+13, y+1
			pg.RoundRect(bx, by, 54, 40, [4]float64{3, 3, 3, 3}, nil, &pdfRule, .6)
			scale := min(48/ink.W, 34/ink.H)
			w, h := ink.W*scale, ink.H*scale
			pg.Image(o.image(OutlineInkKey(e.Item)), bx+3+(48-w)/2, by+3+(34-h)/2, w, h)
		}, sideH)
		o.y += gap
	}
}

func (o *outlinePdf) heading(h OutlineHeading) {
	if h.Level == 2 {
		o.need(96)
	} else {
		o.need(64)
	}
	size, color, top, bottom, pad := 10.8, pdfMuted, 10.0, 5.0, 0.0
	switch h.Level {
	case 2:
		size, color, top, bottom, pad = 14.5, pdfInk, 16, 8, 4
	case 3:
		size, color = 12, pdfInk
	}
	o.y += top
	text := o.para([]pdf.Span{o.span(h.Text, true, size, color)}, o.cw, 0)
	hh := text.h + pad
	o.need(hh)
	text.draw(o.page, o.margin, o.y)
	if h.Level == 2 {
		o.page.Line(o.margin, o.y+hh, o.margin+o.cw, o.y+hh, .6, pdfRule)
	}
	if h.Level == 2 || o.deep && h.Level == 3 {
		o.toc = append(o.toc, tocEntry{h.Text, h.Level - 1, o.page, o.y})
		o.d.Bookmark(h.Text, h.Level-1, o.page, o.y)
	}
	o.y += hh + bottom
}

func (o *outlinePdf) blocks(list []OutlineBlock, appendix bool) {
	for _, b := range list {
		switch b := b.(type) {
		case OutlineHeading:
			o.heading(b)
		case OutlineRemark:
			o.put(o.para([]pdf.Span{o.span(b.Text, false, 9.5, pdfMuted)}, o.cw, 4.5), o.margin, nil, 0)
			o.y += 6
		case OutlineEntry:
			o.entry(b, appendix)
		}
	}
}

func (o *outlinePdf) source(s OutlineSource) {
	width := o.cw - 18
	var where []string
	for _, v := range []string{s.Label, s.Locator} {
		if v != "" {
			where = append(where, v)
		}
	}
	head := []pdf.Span{o.span(strings.Join(where, " · "), false, 9.5, pdfInk)}
	if s.Stale {
		head = append(head, o.span("  "+l10n.OutlineSourceStale, true, 8.5, pdfAmber))
	}
	parts := []piece{o.para(head, width, 4.5)}
	if s.Quote != "" {
		parts = append(parts, o.para([]pdf.Span{o.span("“"+s.Quote+"”", false, 8.8, pdfMuted)}, width, 3))
	}
	if s.Link != "" {
		link := o.span(l10n.OutlineOpenSource, false, 8.5, pdfAccent)
		link.URI = s.Link
		parts = append(parts, o.para([]pdf.Span{link}, width, 4.5))
	}
	var p piece
	for _, part := range parts {
		p.h += part.h
		p.rows = append(p.rows, part.rows...)
	}
	p.draw = func(pg *pdf.Page, x, y float64) {
		for _, part := range parts {
			part.draw(pg, x, y)
			y += part.h
		}
	}
	name := fmt.Sprintf("source-%d", s.Number)
	o.put(p, o.margin+18, func(pg *pdf.Page, y float64) {
		o.d.Anchor(name, pg, y)
		pg.Text(o.bold, 9, o.margin, y+o.bold.Ascent()*9, pdfAccent, fmt.Sprint(s.Number))
	}, 0)
	o.y += 6
}

// ellipsize shortens s with … to fit width.
func ellipsize(f *pdf.Face, s string, size, width float64) string {
	if f.Width(s, size) <= width {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && f.Width(string(r)+"…", size) > width {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}

// BuildOutlinePdf lays doc out as a paginated PDF with a real text layer
// (spec 0028, ADR 0030): a cover, contents when there are enough
// sections, the body, the appendix and the sources, which citations link
// to. The bytes built here are the bytes previewed and saved.
func BuildOutlinePdf(doc OutlineDoc, layout OutlineLayout, regular, bold *pdf.Font, images map[string]OutlineImage, date time.Time) []byte {
	o := &outlinePdf{doc: doc, d: pdf.New(), regular: pdf.TrueType(regular), bold: pdf.TrueType(bold), images: images, placed: map[string]*pdf.Image{}}
	o.d.Title = doc.Title
	o.w, o.h = layout.Size()
	o.margin = layout.Margin()
	o.cw, o.ch = o.w-2*o.margin, o.h-2*o.margin
	footer := (o.regular.Ascent() + o.regular.Descent()) * 7.5
	o.bottom = o.h - o.margin - footer - 14

	var headings []OutlineHeading
	sections := 0
	for _, b := range doc.Blocks() {
		if h, ok := b.(OutlineHeading); ok {
			headings = append(headings, h)
			if h.Level == 2 {
				sections++
			}
		}
	}
	// Sections and the appendix's groups go into the contents when they fit.
	room := int(math.Floor((o.ch - 150) / 17))
	o.deep = len(headings)+1 <= room
	withSources := 0
	if len(doc.Sources) > 0 {
		withSources = 1
	}
	toc := sections+withSources >= 4

	items, bodySections := 0, 0
	for _, b := range doc.Body {
		switch b := b.(type) {
		case OutlineEntry:
			items++
		case OutlineHeading:
			if b.Level == 2 {
				bodySections++
			}
		}
	}
	cover := func() {
		o.put(o.para([]pdf.Span{o.span(doc.Title, true, 21, pdfInk)}, o.cw, 3), o.margin, nil, 0)
		o.y += 6
		meta := l10n.OutlinePdfMeta(date.Format("2006-01-02"), bodySections, items, len(doc.Sources))
		o.put(o.para([]pdf.Span{o.span(meta, false, 9, pdfMuted)}, o.cw, 4.5), o.margin, nil, 0)
		o.y += 14
	}
	var contents *pdf.Page
	if toc {
		contents = o.d.AddPage(o.w, o.h)
	}
	o.newPage()
	if !toc {
		cover()
	}
	o.blocks(doc.Body, false)
	o.blocks(doc.Appendix, true)
	if len(doc.Sources) > 0 {
		o.heading(OutlineHeading{2, l10n.OutlineSources})
		for _, s := range doc.Sources {
			o.source(s)
		}
	}

	total := len(o.d.Pages())
	for _, pg := range o.content {
		top := o.h - o.margin - footer
		pg.Line(o.margin, top-4, o.w-o.margin, top-4, .5, pdfRule)
		number := fmt.Sprintf("%d / %d", pg.Number(), total)
		nw := o.regular.Width(number, 7.5)
		base := top + o.regular.Ascent()*7.5
		pg.Text(o.regular, 7.5, o.margin, base, pdfMuted, ellipsize(o.regular, doc.Title, 7.5, o.cw-nw-12))
		pg.Text(o.regular, 7.5, o.w-o.margin-nw, base, pdfMuted, number)
	}
	if contents != nil {
		o.page, o.y = contents, o.margin
		cover()
		o.y += 10
		o.put(o.para([]pdf.Span{o.span(l10n.OutlineContents, true, 11, pdfMuted)}, o.cw, 0), o.margin, nil, 0)
		o.y += 8
		line := (o.regular.Ascent() + o.regular.Descent()) * 10
		for i, t := range o.toc {
			if o.y+line > o.h-o.margin {
				break
			}
			name := fmt.Sprintf("toc-%d", i)
			o.d.Anchor(name, t.page, t.y)
			number := fmt.Sprint(t.page.Number())
			nw := o.regular.Width(number, 10)
			x := o.margin + 10*float64(t.level-1)
			title := ellipsize(o.regular, t.title, 10, o.margin+o.cw-nw-24-x)
			tw := o.regular.Width(title, 10)
			base := o.y + o.regular.Ascent()*10
			contents.Text(o.regular, 10, x, base, pdfInk, title)
			contents.Line(x+tw+8, base, o.margin+o.cw-nw-8, base, .6, pdfMuted, .6, 2)
			contents.Text(o.regular, 10, o.margin+o.cw-nw, base, pdfInk, number)
			contents.Link(x, o.y, o.margin+o.cw-x, line, name)
			o.y += line + 2.5
		}
	}
	return o.d.Bytes()
}
