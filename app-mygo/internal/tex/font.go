// Package tex typesets TeX math with Latin Modern Math, as the Flutter
// client's val_latex: an OpenType MATH font, the TeX layout rules for
// fractions, radicals, scripts, limits, delimiters, accents and arrays,
// and glyphs drawn from their outlines.
package tex

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"sync"

	gofont "github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
)

//go:embed latinmodern-math.otf
var fontData []byte

type gid = gofont.GID

// mathFont is the font with its MATH table, in ems.
type mathFont struct {
	face *gofont.Face
	upem float32
	c    constants

	italics map[gid]float32
	accents map[gid]float32
	shapes  map[gid]bool
	overlap float32
	vert    map[gid]*construction
	horiz   map[gid]*construction

	mu       sync.Mutex
	outlines map[gid][]ot.Segment
}

// constants are the MATH table's, in ems; the percents as fractions.
type constants struct {
	scriptScale, scriptScriptScale                                   float32
	delimitedSubFormulaMinHeight, displayOperatorMinHeight           float32
	mathLeading, axisHeight, accentBaseHeight, flattenedAccentBase   float32
	subShiftDown, subTopMax, subBaselineDropMin                      float32
	supShiftUp, supShiftUpCramped, supBottomMin, supBaselineDropMax  float32
	subSupGapMin, supBottomMaxWithSub, spaceAfterScript              float32
	upperLimitGapMin, upperLimitBaselineRiseMin                      float32
	lowerLimitGapMin, lowerLimitBaselineDropMin                      float32
	stackTopShiftUp, stackTopDisplayShiftUp                          float32
	stackBottomShiftDown, stackBottomDisplayShiftDown                float32
	stackGapMin, stackDisplayGapMin                                  float32
	stretchStackTopShiftUp, stretchStackBottomShiftDown              float32
	stretchStackGapAboveMin, stretchStackGapBelowMin                 float32
	fracNumShiftUp, fracNumDisplayShiftUp                            float32
	fracDenomShiftDown, fracDenomDisplayShiftDown                    float32
	fracNumGapMin, fracNumDisplayGapMin, fracRuleThickness           float32
	fracDenomGapMin, fracDenomDisplayGapMin                          float32
	skewedFracHGap, skewedFracVGap                                   float32
	overbarGap, overbarRule, overbarExtraAscender                    float32
	underbarGap, underbarRule, underbarExtraDescender                float32
	radicalGap, radicalDisplayGap, radicalRule, radicalExtraAscender float32
	radicalKernBeforeDegree, radicalKernAfterDegree                  float32
	radicalDegreeBottomRaise                                         float32
}

// construction is a glyph's larger variants and how to build it larger
// still.
type construction struct {
	variants []variant
	parts    []part
}

type variant struct {
	g       gid
	advance float32
}

type part struct {
	g                gid
	start, end, full float32
	extender         bool
}

var (
	loadOnce sync.Once
	theFont  *mathFont
)

func font() *mathFont {
	loadOnce.Do(func() {
		f, err := loadFont(fontData)
		if err != nil {
			panic("tex: " + err.Error())
		}
		theFont = f
	})
	return theFont
}

func loadFont(data []byte) (*mathFont, error) {
	ld, err := ot.NewLoader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	fnt, err := gofont.NewFont(ld)
	if err != nil {
		return nil, err
	}
	f := &mathFont{face: gofont.NewFace(fnt), upem: float32(fnt.Upem()), outlines: map[gid][]ot.Segment{}}
	raw, err := ld.RawTable(ot.MustNewTag("MATH"))
	if err != nil {
		return nil, err
	}
	f.parseMath(raw)
	return f, nil
}

type reader []byte

func (r reader) u16(at int) uint16 {
	if at < 0 || at+2 > len(r) {
		return 0
	}
	return binary.BigEndian.Uint16(r[at:])
}
func (r reader) i16(at int) int16 { return int16(r.u16(at)) }
func (r reader) sub(at int) reader {
	if at <= 0 || at >= len(r) {
		return nil
	}
	return r[at:]
}

// coverage returns the glyphs of a Coverage table, in coverage order.
func (r reader) coverage() []gid {
	var out []gid
	switch r.u16(0) {
	case 1:
		n := int(r.u16(2))
		for i := 0; i < n; i++ {
			out = append(out, gid(r.u16(4+2*i)))
		}
	case 2:
		n := int(r.u16(2))
		for i := 0; i < n; i++ {
			lo, hi, idx := r.u16(4+6*i), r.u16(6+6*i), int(r.u16(8+6*i))
			for g := int(lo); g <= int(hi); g++ {
				for len(out) <= idx+g-int(lo) {
					out = append(out, 0)
				}
				out[idx+g-int(lo)] = gid(g)
			}
		}
	}
	return out
}

func (f *mathFont) em(v int16) float32 { return float32(v) / f.upem }

func (f *mathFont) parseMath(t reader) {
	k := t.sub(int(t.u16(4)))
	val := func(i int) float32 { return f.em(k.i16(8 + 4*i)) }
	c := &f.c
	c.scriptScale = float32(k.i16(0)) / 100
	c.scriptScriptScale = float32(k.i16(2)) / 100
	c.delimitedSubFormulaMinHeight = float32(k.u16(4)) / f.upem
	c.displayOperatorMinHeight = float32(k.u16(6)) / f.upem
	fields := []*float32{
		&c.mathLeading, &c.axisHeight, &c.accentBaseHeight, &c.flattenedAccentBase,
		&c.subShiftDown, &c.subTopMax, &c.subBaselineDropMin,
		&c.supShiftUp, &c.supShiftUpCramped, &c.supBottomMin, &c.supBaselineDropMax,
		&c.subSupGapMin, &c.supBottomMaxWithSub, &c.spaceAfterScript,
		&c.upperLimitGapMin, &c.upperLimitBaselineRiseMin, &c.lowerLimitGapMin, &c.lowerLimitBaselineDropMin,
		&c.stackTopShiftUp, &c.stackTopDisplayShiftUp, &c.stackBottomShiftDown, &c.stackBottomDisplayShiftDown,
		&c.stackGapMin, &c.stackDisplayGapMin,
		&c.stretchStackTopShiftUp, &c.stretchStackBottomShiftDown, &c.stretchStackGapAboveMin, &c.stretchStackGapBelowMin,
		&c.fracNumShiftUp, &c.fracNumDisplayShiftUp, &c.fracDenomShiftDown, &c.fracDenomDisplayShiftDown,
		&c.fracNumGapMin, &c.fracNumDisplayGapMin, &c.fracRuleThickness, &c.fracDenomGapMin, &c.fracDenomDisplayGapMin,
		&c.skewedFracHGap, &c.skewedFracVGap,
		&c.overbarGap, &c.overbarRule, &c.overbarExtraAscender,
		&c.underbarGap, &c.underbarRule, &c.underbarExtraDescender,
		&c.radicalGap, &c.radicalDisplayGap, &c.radicalRule, &c.radicalExtraAscender,
		&c.radicalKernBeforeDegree, &c.radicalKernAfterDegree,
	}
	for i, p := range fields {
		*p = val(i)
	}
	c.radicalDegreeBottomRaise = float32(k.i16(8+4*len(fields))) / 100

	gi := t.sub(int(t.u16(6)))
	f.italics = valueTable(f, gi.sub(int(gi.u16(0))))
	f.accents = valueTable(f, gi.sub(int(gi.u16(2))))
	f.shapes = map[gid]bool{}
	if cov := gi.sub(int(gi.u16(4))); cov != nil {
		for _, g := range cov.coverage() {
			f.shapes[g] = true
		}
	}

	v := t.sub(int(t.u16(8)))
	f.overlap = float32(v.u16(0)) / f.upem
	vcov, hcov := v.sub(int(v.u16(2))), v.sub(int(v.u16(4)))
	nv, nh := int(v.u16(6)), int(v.u16(8))
	f.vert, f.horiz = map[gid]*construction{}, map[gid]*construction{}
	read := func(cov reader, n, first int, into map[gid]*construction) {
		if cov == nil {
			return
		}
		gs := cov.coverage()
		for i := 0; i < n && i < len(gs); i++ {
			if c := f.construction(v.sub(int(v.u16(10 + 2*(first+i))))); c != nil {
				into[gs[i]] = c
			}
		}
	}
	read(vcov, nv, 0, f.vert)
	read(hcov, nh, nv, f.horiz)
}

func valueTable(f *mathFont, t reader) map[gid]float32 {
	out := map[gid]float32{}
	if t == nil {
		return out
	}
	gs := t.sub(int(t.u16(0))).coverage()
	n := int(t.u16(2))
	for i := 0; i < n && i < len(gs); i++ {
		out[gs[i]] = f.em(t.i16(4 + 4*i))
	}
	return out
}

func (f *mathFont) construction(t reader) *construction {
	if t == nil {
		return nil
	}
	c := &construction{}
	n := int(t.u16(2))
	for i := 0; i < n; i++ {
		c.variants = append(c.variants, variant{gid(t.u16(4 + 4*i)), float32(t.u16(6+4*i)) / f.upem})
	}
	if a := t.sub(int(t.u16(0))); a != nil {
		np := int(a.u16(4))
		for i := 0; i < np; i++ {
			at := 6 + 10*i
			c.parts = append(c.parts, part{
				g:        gid(a.u16(at)),
				start:    float32(a.u16(at+2)) / f.upem,
				end:      float32(a.u16(at+4)) / f.upem,
				full:     float32(a.u16(at+6)) / f.upem,
				extender: a.u16(at+8)&1 != 0,
			})
		}
	}
	return c
}

// glyph returns r's glyph, false when the font lacks it.
func (f *mathFont) glyph(r rune) (gid, bool) {
	g, ok := f.face.NominalGlyph(r)
	return g, ok && g != 0
}

// metrics are a glyph's advance, height above and depth below the
// baseline, in ems.
func (f *mathFont) metrics(g gid) (w, h, d float32) {
	w = f.face.HorizontalAdvance(g) / f.upem
	if e, ok := f.face.GlyphExtents(g); ok {
		h = e.YBearing / f.upem
		d = (e.Height - e.YBearing) / f.upem
		if e.Height < 0 {
			d = -(e.YBearing + e.Height) / f.upem
		}
	}
	return w, max(h, -d, 0), max(d, -h, 0)
}

// ink is the glyph's ink box in ems: left, right.
func (f *mathFont) ink(g gid) (l, r float32) {
	if e, ok := f.face.GlyphExtents(g); ok {
		return e.XBearing / f.upem, (e.XBearing + e.Width) / f.upem
	}
	return 0, 0
}

func (f *mathFont) outline(g gid) []ot.Segment {
	f.mu.Lock()
	defer f.mu.Unlock()
	if s, ok := f.outlines[g]; ok {
		return s
	}
	var segs []ot.Segment
	if o, ok := f.face.GlyphData(g).(gofont.GlyphOutline); ok {
		segs = o.Segments
	}
	f.outlines[g] = segs
	return segs
}
