package pdf

import (
	"encoding/binary"
	"errors"
	"math/bits"
	"sort"
	"unicode/utf16"
)

var errFont = errors.New("pdf: not a TrueType font")

func u16(b []byte, at int) int { return int(binary.BigEndian.Uint16(b[at:])) }
func u32(b []byte, at int) int { return int(binary.BigEndian.Uint32(b[at:])) }

// Font is a TrueType font with glyf outlines, parsed once and embedded as
// a subset of the glyphs each document sets.
type Font struct {
	// Name is the PostScript name.
	Name string
	// Ascent and Descent are hhea's, per em; Descent is positive.
	Ascent, Descent float64

	upm      float64
	tables   map[string][]byte
	advances []uint16
	lsbs     []uint16
	loca     []int
	cmap     map[rune]uint16
	bbox     [4]int
}

// ParseTTF parses a TrueType font.
func ParseTTF(b []byte) (f *Font, err error) {
	defer func() {
		if recover() != nil {
			f, err = nil, errFont
		}
	}()
	f = &Font{tables: map[string][]byte{}}
	for i, n := 0, u16(b, 4); i < n; i++ {
		at := 12 + 16*i
		off, size := u32(b, at+8), u32(b, at+12)
		f.tables[string(b[at:at+4])] = b[off : off+size]
	}
	for _, t := range []string{"head", "hhea", "hmtx", "maxp", "loca", "glyf", "cmap"} {
		if f.tables[t] == nil {
			return nil, errFont
		}
	}
	head, hhea, hmtx := f.tables["head"], f.tables["hhea"], f.tables["hmtx"]
	f.upm = float64(u16(head, 18))
	for i := range f.bbox {
		f.bbox[i] = int(int16(u16(head, 36+2*i)))
	}
	f.Ascent = float64(int16(u16(hhea, 4))) / f.upm
	f.Descent = -float64(int16(u16(hhea, 6))) / f.upm
	metrics, glyphs := u16(hhea, 34), u16(f.tables["maxp"], 4)
	f.advances, f.lsbs = make([]uint16, glyphs), make([]uint16, glyphs)
	for g := range glyphs {
		if g < metrics {
			f.advances[g], f.lsbs[g] = uint16(u16(hmtx, 4*g)), uint16(u16(hmtx, 4*g+2))
		} else {
			f.advances[g], f.lsbs[g] = f.advances[metrics-1], uint16(u16(hmtx, 4*metrics+2*(g-metrics)))
		}
	}
	loca, long := f.tables["loca"], u16(head, 50) == 1
	f.loca = make([]int, glyphs+1)
	for g := range f.loca {
		if long {
			f.loca[g] = u32(loca, 4*g)
		} else {
			f.loca[g] = 2 * u16(loca, 2*g)
		}
	}
	f.cmap = parseCmap(f.tables["cmap"])
	f.Name = postScriptName(f.tables["name"])
	return f, nil
}

// parseCmap reads the Unicode subtable, format 12 over format 4.
func parseCmap(d []byte) map[rune]uint16 {
	out := map[rune]uint16{}
	best, format := -1, 0
	for i, n := 0, u16(d, 2); i < n; i++ {
		at := 4 + 8*i
		platform, encoding := u16(d, at), u16(d, at+2)
		table := u32(d, at+4)
		f := u16(d, table)
		unicode := platform == 0 || platform == 3 && (encoding == 1 || encoding == 10)
		if unicode && (f == 12 || f == 4 && format != 12) {
			best, format = table, f
		}
	}
	switch {
	case best < 0:
	case format == 12:
		for g, n := 0, u32(d, best+12); g < n; g++ {
			at := best + 16 + 12*g
			start, end, glyph := u32(d, at), u32(d, at+4), u32(d, at+8)
			for c := start; c <= end; c++ {
				if gid := glyph + c - start; gid != 0 {
					out[rune(c)] = uint16(gid)
				}
			}
		}
	default:
		seg2 := u16(d, best+6)
		ends := best + 14
		starts := ends + seg2 + 2
		deltas := starts + seg2
		ranges := deltas + seg2
		for s := 0; s < seg2; s += 2 {
			start, end := u16(d, starts+s), u16(d, ends+s)
			delta, rng := int(int16(u16(d, deltas+s))), u16(d, ranges+s)
			for c := start; c <= end && c != 0xffff; c++ {
				var gid int
				if rng == 0 {
					gid = (c + delta) & 0xffff
				} else {
					gid = u16(d, ranges+s+rng+2*(c-start))
					if gid != 0 {
						gid = (gid + delta) & 0xffff
					}
				}
				if gid != 0 {
					out[rune(c)] = uint16(gid)
				}
			}
		}
	}
	return out
}

func postScriptName(d []byte) string {
	if len(d) < 6 {
		return "Font"
	}
	strings := u16(d, 4)
	for i, n := 0, u16(d, 2); i < n; i++ {
		at := 6 + 12*i
		platform, id := u16(d, at), u16(d, at+6)
		size, off := u16(d, at+8), strings+u16(d, at+10)
		if id != 6 || off+size > len(d) {
			continue
		}
		s := d[off : off+size]
		if platform == 3 || platform == 0 {
			u := make([]uint16, len(s)/2)
			for k := range u {
				u[k] = uint16(u16(s, 2*k))
			}
			return string(utf16.Decode(u))
		}
		return string(s)
	}
	return "Font"
}

// Has reports whether the font draws r.
func (f *Font) Has(r rune) bool { _, ok := f.cmap[r]; return ok }

// Runes are the characters the font draws.
func (f *Font) Runes() map[rune]uint16 { return f.cmap }

func (f *Font) glyph(r rune) uint16 { return f.cmap[r] }

// advance is a glyph's width per em.
func (f *Font) advance(g uint16) float64 {
	if int(g) >= len(f.advances) {
		return 0
	}
	return float64(f.advances[g]) / f.upm
}

func (f *Font) glyphData(g uint16) []byte {
	glyf := f.tables["glyf"]
	from, to := f.loca[g], f.loca[g+1]
	if to <= from || to > len(glyf) {
		return nil
	}
	return glyf[from:to]
}

// subset is a TrueType font of the glyphs in order, renumbered by their
// place in it; order grows by the parts of composite glyphs.
func (f *Font) subset(order []uint16) ([]byte, []uint16) {
	index := map[uint16]int{}
	for i, g := range order {
		index[g] = i
	}
	var glyf []byte
	loca := []int{0}
	for i := 0; i < len(order); i++ {
		data := append([]byte(nil), f.glyphData(order[i])...)
		if len(data) >= 10 && int16(u16(data, 0)) < 0 {
			for at := 10; at+4 <= len(data); {
				flags, part := u16(data, at), uint16(u16(data, at+2))
				n, ok := index[part]
				if !ok {
					n = len(order)
					index[part] = n
					order = append(order, part)
				}
				binary.BigEndian.PutUint16(data[at+2:], uint16(n))
				at += 4
				if flags&1 != 0 {
					at += 4
				} else {
					at += 2
				}
				switch {
				case flags&8 != 0:
					at += 2
				case flags&0x40 != 0:
					at += 4
				case flags&0x80 != 0:
					at += 8
				}
				if flags&0x20 == 0 {
					break
				}
			}
		}
		glyf = append(glyf, data...)
		for len(glyf)%4 != 0 {
			glyf = append(glyf, 0)
		}
		loca = append(loca, len(glyf))
	}
	n := len(order)
	tables := map[string][]byte{"glyf": glyf}
	tables["loca"] = make([]byte, 4*len(loca))
	for i, v := range loca {
		binary.BigEndian.PutUint32(tables["loca"][4*i:], uint32(v))
	}
	hmtx := make([]byte, 4*n)
	for i, g := range order {
		binary.BigEndian.PutUint16(hmtx[4*i:], f.advances[g])
		binary.BigEndian.PutUint16(hmtx[4*i+2:], f.lsbs[g])
	}
	tables["hmtx"] = hmtx
	copied := func(tag string) []byte { return append([]byte(nil), f.tables[tag]...) }
	tables["hhea"] = copied("hhea")
	binary.BigEndian.PutUint16(tables["hhea"][34:], uint16(n))
	tables["maxp"] = copied("maxp")
	binary.BigEndian.PutUint16(tables["maxp"][4:], uint16(n))
	tables["head"] = copied("head")
	binary.BigEndian.PutUint32(tables["head"][8:], 0)
	binary.BigEndian.PutUint16(tables["head"][50:], 1)
	// The PDF maps codes to glyphs itself; viewers only want a cmap.
	tables["cmap"] = []byte{0, 0, 0, 1, 0, 3, 0, 1, 0, 0, 0, 12,
		0, 4, 0, 24, 0, 0, 0, 2, 0, 2, 0, 0, 0, 0, 0xff, 0xff, 0, 0, 0xff, 0xff, 0, 1, 0, 0}
	if p := f.tables["post"]; len(p) >= 32 {
		tables["post"] = append([]byte(nil), p[:32]...)
		binary.BigEndian.PutUint32(tables["post"], 0x00030000)
	}
	for _, t := range []string{"OS/2", "name"} {
		if f.tables[t] != nil {
			tables[t] = copied(t)
		}
	}
	out := writeSfnt(tables)
	adjust := 0xb1b0afba - checksum(out)
	binary.BigEndian.PutUint32(out[sfntOffset(out, "head")+8:], adjust)
	return out, order
}

func checksum(b []byte) uint32 {
	var sum uint32
	for i := 0; i < len(b); i += 4 {
		var w [4]byte
		copy(w[:], b[i:])
		sum += binary.BigEndian.Uint32(w[:])
	}
	return sum
}

func writeSfnt(tables map[string][]byte) []byte {
	tags := make([]string, 0, len(tables))
	for t := range tables {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	n := len(tags)
	power := 1 << (bits.Len(uint(n)) - 1)
	out := make([]byte, 12+16*n)
	binary.BigEndian.PutUint32(out, 0x00010000)
	binary.BigEndian.PutUint16(out[4:], uint16(n))
	binary.BigEndian.PutUint16(out[6:], uint16(16*power))
	binary.BigEndian.PutUint16(out[8:], uint16(bits.Len(uint(power))-1))
	binary.BigEndian.PutUint16(out[10:], uint16(16*n-16*power))
	for i, t := range tags {
		data := tables[t]
		at := 12 + 16*i
		copy(out[at:], t)
		binary.BigEndian.PutUint32(out[at+4:], checksum(data))
		binary.BigEndian.PutUint32(out[at+8:], uint32(len(out)))
		binary.BigEndian.PutUint32(out[at+12:], uint32(len(data)))
		out = append(out, data...)
		for len(out)%4 != 0 {
			out = append(out, 0)
		}
	}
	return out
}

func sfntOffset(b []byte, tag string) int {
	for i, n := 0, u16(b, 4); i < n; i++ {
		if at := 12 + 16*i; string(b[at:at+4]) == tag {
			return u32(b, at+8)
		}
	}
	return 0
}
