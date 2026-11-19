// Package pdf writes PDF files: text in embedded TrueType subsets, which
// keep a text layer, or in Courier; filled and stroked shapes; images;
// links within the document and out of it; and bookmarks.
package pdf

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"hash/fnv"
	"image"
	"image/draw"
	"math"
	"sort"
	"strings"
	"unicode/utf16"
)

// Color is an RGB colour, each part from 0 to 1.
type Color struct{ R, G, B float64 }

// Hex is the colour of 0xAARRGGBB; alpha is ignored.
func Hex(argb uint32) Color {
	return Color{float64(argb>>16&0xff) / 255, float64(argb>>8&0xff) / 255, float64(argb&0xff) / 255}
}

// Face is a font text is set in: an embedded TrueType font, or Courier.
type Face struct{ font *Font }

// Courier is the standard monospaced font; it draws printable ASCII.
var Courier = &Face{}

// TrueType is a face of f.
func TrueType(f *Font) *Face { return &Face{font: f} }

// Has reports whether the face draws r.
func (f *Face) Has(r rune) bool {
	if f.font == nil {
		return r >= 0x20 && r < 0x7f
	}
	return f.font.Has(r)
}

// Advance is r's width per em.
func (f *Face) Advance(r rune) float64 {
	if f.font == nil {
		return .6
	}
	return f.font.advance(f.font.glyph(r))
}

// Ascent is how far the face rises above its baseline, per em.
func (f *Face) Ascent() float64 {
	if f.font == nil {
		return .91
	}
	return f.font.Ascent
}

// Descent is how far it falls below, per em.
func (f *Face) Descent() float64 {
	if f.font == nil {
		return .22
	}
	return f.font.Descent
}

// Width is the width of s at size.
func (f *Face) Width(s string, size float64) float64 {
	w := 0.0
	for _, r := range s {
		w += f.Advance(r)
	}
	return w * size
}

type faceUse struct {
	face  *Face
	name  string
	index map[uint16]uint16
	order []uint16
	text  map[uint16]rune
}

func (u *faceUse) code(r rune) uint16 {
	g := u.face.font.glyph(r)
	n, ok := u.index[g]
	if !ok {
		n = uint16(len(u.order))
		u.index[g] = n
		u.order = append(u.order, g)
		u.text[n] = r
	}
	return n
}

// Image is a picture placed on pages.
type Image struct {
	name string
	img  image.Image
}

type annot struct {
	x, y, w, h float64
	uri, dest  string
}

type anchor struct {
	page *Page
	y    float64
}

type bookmark struct {
	title string
	level int
	at    anchor
}

// Doc is a document being written.
type Doc struct {
	Title string

	pages   []*Page
	faces   []*faceUse
	byFace  map[*Face]*faceUse
	images  []*Image
	anchors map[string]anchor
	marks   []bookmark
}

// New starts a document.
func New() *Doc { return &Doc{byFace: map[*Face]*faceUse{}, anchors: map[string]anchor{}} }

// Page is one page; its coordinates run from the top left in points.
type Page struct {
	W, H   float64
	doc    *Doc
	ops    bytes.Buffer
	annots []annot
}

// AddPage appends a page of w×h points.
func (d *Doc) AddPage(w, h float64) *Page {
	p := &Page{W: w, H: h, doc: d}
	d.pages = append(d.pages, p)
	return p
}

// Pages are the document's pages in order.
func (d *Doc) Pages() []*Page { return d.pages }

// Number is the page's number, from 1.
func (p *Page) Number() int {
	for i, q := range p.doc.pages {
		if q == p {
			return i + 1
		}
	}
	return 0
}

// Anchor names a place that links can go to.
func (d *Doc) Anchor(name string, p *Page, y float64) { d.anchors[name] = anchor{p, y} }

// Bookmark adds an entry to the document outline; level 1 is the top.
func (d *Doc) Bookmark(title string, level int, p *Page, y float64) {
	d.marks = append(d.marks, bookmark{title, max(level, 1), anchor{p, y}})
}

// Image adds a picture.
func (d *Doc) Image(img image.Image) *Image {
	im := &Image{name: fmt.Sprintf("Im%d", len(d.images)+1), img: img}
	d.images = append(d.images, im)
	return im
}

func (d *Doc) use(f *Face) *faceUse {
	u := d.byFace[f]
	if u == nil {
		u = &faceUse{face: f, name: fmt.Sprintf("F%d", len(d.faces)+1), index: map[uint16]uint16{0: 0}, order: []uint16{0}, text: map[uint16]rune{}}
		d.byFace[f] = u
		d.faces = append(d.faces, u)
	}
	return u
}

func num(v float64) string {
	s := fmt.Sprintf("%.3f", v)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	if s == "-0" || s == "" {
		return "0"
	}
	return s
}

func (p *Page) printf(format string, args ...any) { fmt.Fprintf(&p.ops, format, args...) }

func (p *Page) fill(c Color) { p.printf("%s %s %s rg\n", num(c.R), num(c.G), num(c.B)) }

func (p *Page) stroke(c Color, width float64) {
	p.printf("%s %s %s RG %s w\n", num(c.R), num(c.G), num(c.B), num(width))
}

// Text sets s from x with its baseline at y.
func (p *Page) Text(f *Face, size, x, y float64, c Color, s string) {
	if s == "" {
		return
	}
	u := p.doc.use(f)
	var body strings.Builder
	if f.font == nil {
		body.WriteByte('(')
		for _, r := range s {
			if r < 0x20 || r >= 0x7f {
				r = '?'
			}
			if r == '(' || r == ')' || r == '\\' {
				body.WriteByte('\\')
			}
			body.WriteRune(r)
		}
		body.WriteByte(')')
	} else {
		body.WriteByte('<')
		for _, r := range s {
			fmt.Fprintf(&body, "%04X", u.code(r))
		}
		body.WriteByte('>')
	}
	p.fill(c)
	p.printf("BT /%s %s Tf %s %s Td %s Tj ET\n", u.name, num(size), num(x), num(p.H-y), body.String())
}

// Rect fills a rectangle.
func (p *Page) Rect(x, y, w, h float64, c Color) {
	p.fill(c)
	p.printf("%s %s %s %s re f\n", num(x), num(p.H-y-h), num(w), num(h))
}

// RoundRect fills, strokes or both a rectangle whose corners, from the top
// left clockwise, have radii r; a nil colour skips that part.
func (p *Page) RoundRect(x, y, w, h float64, r [4]float64, fill *Color, stroke *Color, width float64) {
	const k = .5523
	top, bottom := p.H-y, p.H-y-h
	tl, tr, br, bl := r[0], r[1], r[2], r[3]
	p.printf("%s %s m\n", num(x+tl), num(top))
	p.printf("%s %s l\n", num(x+w-tr), num(top))
	if tr > 0 {
		p.printf("%s %s %s %s %s %s c\n", num(x+w-tr+k*tr), num(top), num(x+w), num(top-tr+k*tr), num(x+w), num(top-tr))
	}
	p.printf("%s %s l\n", num(x+w), num(bottom+br))
	if br > 0 {
		p.printf("%s %s %s %s %s %s c\n", num(x+w), num(bottom+br-k*br), num(x+w-br+k*br), num(bottom), num(x+w-br), num(bottom))
	}
	p.printf("%s %s l\n", num(x+bl), num(bottom))
	if bl > 0 {
		p.printf("%s %s %s %s %s %s c\n", num(x+bl-k*bl), num(bottom), num(x), num(bottom+bl-k*bl), num(x), num(bottom+bl))
	}
	p.printf("%s %s l\n", num(x), num(top-tl))
	if tl > 0 {
		p.printf("%s %s %s %s %s %s c\n", num(x), num(top-tl+k*tl), num(x+tl-k*tl), num(top), num(x+tl), num(top))
	}
	switch {
	case fill != nil && stroke != nil:
		p.fill(*fill)
		p.stroke(*stroke, width)
		p.printf("b\n")
	case fill != nil:
		p.fill(*fill)
		p.printf("f\n")
	case stroke != nil:
		p.stroke(*stroke, width)
		p.printf("s\n")
	default:
		p.printf("n\n")
	}
}

// Line strokes a line; dash, when given, is its on and off lengths.
func (p *Page) Line(x1, y1, x2, y2, width float64, c Color, dash ...float64) {
	p.stroke(c, width)
	if len(dash) == 2 {
		p.printf("[%s %s] 0 d ", num(dash[0]), num(dash[1]))
	}
	p.printf("%s %s m %s %s l S\n", num(x1), num(p.H-y1), num(x2), num(p.H-y2))
	if len(dash) == 2 {
		p.printf("[] 0 d\n")
	}
}

// Image draws im into a rectangle.
func (p *Page) Image(im *Image, x, y, w, h float64) {
	p.printf("q %s 0 0 %s %s %s cm /%s Do Q\n", num(w), num(h), num(x), num(p.H-y-h), im.name)
}

// Link makes a rectangle go to an anchor of the document.
func (p *Page) Link(x, y, w, h float64, name string) {
	p.annots = append(p.annots, annot{x: x, y: y, w: w, h: h, dest: name})
}

// URI makes a rectangle open uri.
func (p *Page) URI(x, y, w, h float64, uri string) {
	p.annots = append(p.annots, annot{x: x, y: y, w: w, h: h, uri: uri})
}

// textString is a PDF text string: ASCII as it is, anything else UTF-16.
func textString(s string) string {
	ascii := true
	for _, r := range s {
		if r < 0x20 || r >= 0x7f {
			ascii = false
		}
	}
	if ascii {
		r := strings.NewReplacer(`\`, `\\`, "(", `\(`, ")", `\)`)
		return "(" + r.Replace(s) + ")"
	}
	var b strings.Builder
	b.WriteString("<FEFF")
	for _, u := range utf16.Encode([]rune(s)) {
		fmt.Fprintf(&b, "%04X", u)
	}
	b.WriteString(">")
	return b.String()
}

type writer struct {
	out     bytes.Buffer
	offsets map[int]int
	next    int
}

func (w *writer) alloc() int {
	w.next++
	return w.next
}

func (w *writer) object(n int, body string) {
	w.offsets[n] = w.out.Len()
	fmt.Fprintf(&w.out, "%d 0 obj\n%s\nendobj\n", n, body)
}

func deflate(b []byte) []byte {
	var out bytes.Buffer
	z, _ := zlib.NewWriterLevel(&out, zlib.BestCompression)
	z.Write(b)
	z.Close()
	return out.Bytes()
}

func (w *writer) stream(n int, dict string, data []byte) {
	z := deflate(data)
	w.offsets[n] = w.out.Len()
	fmt.Fprintf(&w.out, "%d 0 obj\n<< %s /Filter /FlateDecode /Length %d >>\nstream\n", n, dict, len(z))
	w.out.Write(z)
	w.out.WriteString("\nendstream\nendobj\n")
}

// Bytes writes the document out.
func (d *Doc) Bytes() []byte {
	w := &writer{offsets: map[int]int{}}
	w.out.WriteString("%PDF-1.7\n%\xe2\xe3\xcf\xd3\n")
	catalog, pagesN, info, resources := w.alloc(), w.alloc(), w.alloc(), w.alloc()
	pageN := make([]int, len(d.pages))
	for i := range d.pages {
		pageN[i] = w.alloc()
	}
	ref := func(p *Page) int {
		for i, q := range d.pages {
			if q == p {
				return pageN[i]
			}
		}
		return pageN[0]
	}
	dest := func(a anchor) string { return fmt.Sprintf("[%d 0 R /XYZ 0 %s null]", ref(a.page), num(a.page.H-a.y)) }

	// Pages, their content and links come first: they choose the glyphs.
	var kids []string
	for i, p := range d.pages {
		content := w.alloc()
		w.stream(content, "", p.ops.Bytes())
		var annots []string
		for _, a := range p.annots {
			var action string
			if a.uri != "" {
				action = "/A << /S /URI /URI " + textString(a.uri) + " >>"
			} else if at, ok := d.anchors[a.dest]; ok {
				action = "/Dest " + dest(at)
			} else {
				continue
			}
			n := w.alloc()
			w.object(n, fmt.Sprintf("<< /Type /Annot /Subtype /Link /Rect [%s %s %s %s] /Border [0 0 0] %s >>",
				num(a.x), num(p.H-a.y-a.h), num(a.x+a.w), num(p.H-a.y), action))
			annots = append(annots, fmt.Sprintf("%d 0 R", n))
		}
		extra := ""
		if len(annots) > 0 {
			extra = " /Annots [" + strings.Join(annots, " ") + "]"
		}
		w.object(pageN[i], fmt.Sprintf("<< /Type /Page /Parent %d 0 R /MediaBox [0 0 %s %s] /Resources %d 0 R /Contents %d 0 R%s >>",
			pagesN, num(p.W), num(p.H), resources, content, extra))
		kids = append(kids, fmt.Sprintf("%d 0 R", pageN[i]))
	}
	w.object(pagesN, fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(d.pages)))

	var fonts, xobjects []string
	for _, u := range d.faces {
		n := w.alloc()
		fonts = append(fonts, fmt.Sprintf("/%s %d 0 R", u.name, n))
		if u.face.font == nil {
			w.object(n, "<< /Type /Font /Subtype /Type1 /BaseFont /Courier /Encoding /WinAnsiEncoding >>")
			continue
		}
		d.writeFont(w, n, u)
	}
	for _, im := range d.images {
		n := w.alloc()
		xobjects = append(xobjects, fmt.Sprintf("/%s %d 0 R", im.name, n))
		writeImage(w, n, im.img)
	}
	w.object(resources, fmt.Sprintf("<< /ProcSet [/PDF /Text /ImageC] /Font << %s >> /XObject << %s >> >>",
		strings.Join(fonts, " "), strings.Join(xobjects, " ")))

	outlines := ""
	if len(d.marks) > 0 {
		outlines = fmt.Sprintf(" /Outlines %d 0 R", d.writeOutline(w, dest))
	}
	w.object(catalog, fmt.Sprintf("<< /Type /Catalog /Pages %d 0 R%s >>", pagesN, outlines))
	w.object(info, fmt.Sprintf("<< /Title %s /Creator (Coursive) /Producer (Coursive) >>", textString(d.Title)))

	xref := w.out.Len()
	fmt.Fprintf(&w.out, "xref\n0 %d\n0000000000 65535 f \n", w.next+1)
	for n := 1; n <= w.next; n++ {
		fmt.Fprintf(&w.out, "%010d 00000 n \n", w.offsets[n])
	}
	fmt.Fprintf(&w.out, "trailer\n<< /Size %d /Root %d 0 R /Info %d 0 R >>\nstartxref\n%d\n%%%%EOF\n", w.next+1, catalog, info, xref)
	return w.out.Bytes()
}

func (d *Doc) writeFont(w *writer, n int, u *faceUse) {
	f := u.face.font
	program, order := f.subset(append([]uint16(nil), u.order...))
	h := fnv.New32a()
	for _, g := range order {
		fmt.Fprint(h, g, ",")
	}
	tag := []byte("AAAAAA")
	for i, v := 0, h.Sum32(); i < 6; i, v = i+1, v/26 {
		tag[i] = byte('A' + v%26)
	}
	base := string(tag) + "+" + strings.ReplaceAll(f.Name, " ", "")
	scale := 1000 / f.upm
	var widths strings.Builder
	for _, g := range order {
		fmt.Fprintf(&widths, "%d ", int(math.Round(float64(f.advances[g])*scale)))
	}
	cid, desc, file, cmap := w.alloc(), w.alloc(), w.alloc(), w.alloc()
	w.object(n, fmt.Sprintf("<< /Type /Font /Subtype /Type0 /BaseFont /%s /Encoding /Identity-H /DescendantFonts [%d 0 R] /ToUnicode %d 0 R >>", base, cid, cmap))
	w.object(cid, fmt.Sprintf("<< /Type /Font /Subtype /CIDFontType2 /BaseFont /%s /CIDSystemInfo << /Registry (Adobe) /Ordering (Identity) /Supplement 0 >> /FontDescriptor %d 0 R /CIDToGIDMap /Identity /DW 1000 /W [0 [%s]] >>",
		base, desc, strings.TrimSpace(widths.String())))
	w.object(desc, fmt.Sprintf("<< /Type /FontDescriptor /FontName /%s /Flags 4 /FontBBox [%d %d %d %d] /ItalicAngle 0 /Ascent %d /Descent %d /CapHeight %d /StemV 80 /FontFile2 %d 0 R >>",
		base, int(float64(f.bbox[0])*scale), int(float64(f.bbox[1])*scale), int(float64(f.bbox[2])*scale), int(float64(f.bbox[3])*scale),
		int(f.Ascent*1000), int(-f.Descent*1000), int(f.Ascent*1000*.6), file))
	w.stream(file, fmt.Sprintf("/Length1 %d", len(program)), program)
	w.stream(cmap, "", toUnicode(u.text))
}

func toUnicode(text map[uint16]rune) []byte {
	codes := make([]int, 0, len(text))
	for c := range text {
		codes = append(codes, int(c))
	}
	sort.Ints(codes)
	var b bytes.Buffer
	b.WriteString("/CIDInit /ProcSet findresource begin\n12 dict begin\nbegincmap\n/CIDSystemInfo << /Registry (Adobe) /Ordering (UCS) /Supplement 0 >> def\n/CMapName /Adobe-Identity-UCS def\n/CMapType 2 def\n1 begincodespacerange\n<0000> <FFFF>\nendcodespacerange\n")
	for i := 0; i < len(codes); i += 100 {
		chunk := codes[i:min(i+100, len(codes))]
		fmt.Fprintf(&b, "%d beginbfchar\n", len(chunk))
		for _, c := range chunk {
			fmt.Fprintf(&b, "<%04X> <", c)
			for _, u := range utf16.Encode([]rune{text[uint16(c)]}) {
				fmt.Fprintf(&b, "%04X", u)
			}
			b.WriteString(">\n")
		}
		b.WriteString("endbfchar\n")
	}
	b.WriteString("endcmap\nCMapName currentdict /CIDResource defineresource pop\nend\nend\n")
	return b.Bytes()
}

func writeImage(w *writer, n int, img image.Image) {
	b := img.Bounds()
	nrgba := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(nrgba, nrgba.Bounds(), img, b.Min, draw.Src)
	rgb := make([]byte, 0, 3*b.Dx()*b.Dy())
	alpha := make([]byte, 0, b.Dx()*b.Dy())
	opaque := true
	for i := 0; i < len(nrgba.Pix); i += 4 {
		rgb = append(rgb, nrgba.Pix[i], nrgba.Pix[i+1], nrgba.Pix[i+2])
		alpha = append(alpha, nrgba.Pix[i+3])
		opaque = opaque && nrgba.Pix[i+3] == 255
	}
	mask := ""
	if !opaque {
		m := w.alloc()
		w.stream(m, fmt.Sprintf("/Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceGray /BitsPerComponent 8", b.Dx(), b.Dy()), alpha)
		mask = fmt.Sprintf(" /SMask %d 0 R", m)
	}
	w.stream(n, fmt.Sprintf("/Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceRGB /BitsPerComponent 8%s", b.Dx(), b.Dy(), mask), rgb)
}

// writeOutline writes the bookmarks as a tree by their levels and
// returns the outline's object.
func (d *Doc) writeOutline(w *writer, dest func(anchor) string) int {
	type node struct {
		n        int
		mark     bookmark
		parent   *node
		children []*node
	}
	root := &node{n: w.alloc(), mark: bookmark{level: 0}}
	stack := []*node{root}
	var all []*node
	for _, m := range d.marks {
		for len(stack) > 1 && stack[len(stack)-1].mark.level >= m.level {
			stack = stack[:len(stack)-1]
		}
		parent := stack[len(stack)-1]
		nd := &node{n: w.alloc(), mark: m, parent: parent}
		parent.children = append(parent.children, nd)
		stack = append(stack, nd)
		all = append(all, nd)
	}
	var count func(*node) int
	count = func(nd *node) int {
		c := len(nd.children)
		for _, k := range nd.children {
			c += count(k)
		}
		return c
	}
	links := func(nd *node) string {
		if len(nd.children) == 0 {
			return ""
		}
		return fmt.Sprintf(" /First %d 0 R /Last %d 0 R /Count %d", nd.children[0].n, nd.children[len(nd.children)-1].n, count(nd))
	}
	w.object(root.n, "<< /Type /Outlines"+links(root)+" >>")
	for _, nd := range all {
		siblings := nd.parent.children
		var s strings.Builder
		fmt.Fprintf(&s, "<< /Title %s /Parent %d 0 R /Dest %s", textString(nd.mark.title), nd.parent.n, dest(nd.mark.at))
		for i, k := range siblings {
			if k != nd {
				continue
			}
			if i > 0 {
				fmt.Fprintf(&s, " /Prev %d 0 R", siblings[i-1].n)
			}
			if i+1 < len(siblings) {
				fmt.Fprintf(&s, " /Next %d 0 R", siblings[i+1].n)
			}
		}
		s.WriteString(links(nd) + " >>")
		w.object(nd.n, s.String())
	}
	return root.n
}
