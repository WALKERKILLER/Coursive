package tex

import (
	"encoding/binary"
	"errors"
	"sort"
)

// privateUse is the code point that draws glyph 0 of the font's copy:
// glyph id is the character privateUse+id (planes 15 and 16).
const (
	privateUse    = 0xF0000
	privateUseEnd = 0x10FFFD
)

// glyphFont is the font with a cmap of its own: every glyph is the
// character privateUse+id, so that the text engine draws exactly that
// glyph, at any size, as it draws text, untouched by shaping. The layout
// picks glyphs by id, and many have no character of their own (script
// forms, size variants, assembly parts). The other tables are the font's.
func glyphFont(data []byte) ([]byte, int, error) {
	if len(data) < 12 {
		return nil, 0, errors.New("short font")
	}
	n := int(binary.BigEndian.Uint16(data[4:]))
	type table struct {
		tag  string
		data []byte
	}
	var tables []table
	glyphs := 0
	for i := range n {
		r := data[12+16*i:]
		tag := string(r[:4])
		off, ln := binary.BigEndian.Uint32(r[8:]), binary.BigEndian.Uint32(r[12:])
		if int(off+ln) > len(data) {
			return nil, 0, errors.New("table out of the font")
		}
		t := data[off : off+ln]
		switch tag {
		case "cmap":
			continue
		case "maxp":
			glyphs = int(binary.BigEndian.Uint16(t[4:]))
		}
		tables = append(tables, table{tag, t})
	}
	if glyphs < 2 || privateUse+glyphs-1 > privateUseEnd {
		return nil, 0, errors.New("no glyph count")
	}
	// One format 12 subtable, for Unicode full repertoire under both
	// platforms, with a single group.
	sub := make([]byte, 16+12)
	binary.BigEndian.PutUint16(sub[0:], 12)
	binary.BigEndian.PutUint32(sub[4:], uint32(len(sub)))
	binary.BigEndian.PutUint32(sub[12:], 1)
	binary.BigEndian.PutUint32(sub[16:], privateUse+1)
	binary.BigEndian.PutUint32(sub[20:], uint32(privateUse+glyphs-1))
	binary.BigEndian.PutUint32(sub[24:], 1)
	cmap := make([]byte, 4+8*2)
	binary.BigEndian.PutUint16(cmap[2:], 2)
	for i, enc := range [][2]uint16{{0, 4}, {3, 10}} {
		binary.BigEndian.PutUint16(cmap[4+8*i:], enc[0])
		binary.BigEndian.PutUint16(cmap[6+8*i:], enc[1])
		binary.BigEndian.PutUint32(cmap[8+8*i:], uint32(len(cmap)))
	}
	tables = append(tables, table{"cmap", append(cmap, sub...)})
	sort.Slice(tables, func(i, j int) bool { return tables[i].tag < tables[j].tag })

	n = len(tables)
	head := make([]byte, 12+16*n)
	copy(head, data[:4])
	binary.BigEndian.PutUint16(head[4:], uint16(n))
	es := 0
	for 1<<(es+1) <= n {
		es++
	}
	binary.BigEndian.PutUint16(head[6:], uint16(16<<es))
	binary.BigEndian.PutUint16(head[8:], uint16(es))
	binary.BigEndian.PutUint16(head[10:], uint16(16*n-16<<es))
	out := head
	headAt := -1
	for i, t := range tables {
		off := len(out)
		body := append([]byte(nil), t.data...)
		if t.tag == "head" && len(body) >= 12 {
			binary.BigEndian.PutUint32(body[8:], 0)
			headAt = off
		}
		r := out[12+16*i:]
		copy(r, t.tag)
		binary.BigEndian.PutUint32(r[4:], checksum(body))
		binary.BigEndian.PutUint32(r[8:], uint32(off))
		binary.BigEndian.PutUint32(r[12:], uint32(len(body)))
		out = append(out, body...)
		for len(out)%4 != 0 {
			out = append(out, 0)
		}
	}
	if headAt >= 0 {
		binary.BigEndian.PutUint32(out[headAt+8:], 0xB1B0AFBA-checksum(out))
	}
	return out, glyphs, nil
}

func checksum(b []byte) uint32 {
	var s uint32
	for i := 0; i < len(b); i += 4 {
		var w [4]byte
		copy(w[:], b[i:])
		s += binary.BigEndian.Uint32(w[:])
	}
	return s
}
