package tex

// The TeX layout of each structure, with the MATH table's constants, as
// MathML Core and LuaTeX read them.

func (l *layout) frac(n *frac, st style) box {
	c := &l.f.c
	if n.style >= 0 {
		st = style{n.style, st.cramped}
	}
	em := l.em(st)
	num := l.list(n.num, st.num())
	den := l.list(n.den, st.den())
	display := st.level == 0
	if n.style == 0 && n.rule {
		// \cfrac and \dfrac: display style above and below, as amsmath.
		num = l.list(n.num, style{1, st.cramped})
		den = l.list(n.den, style{1, true})
	}
	axis := c.axisHeight * em
	var up, down, t float32
	if n.rule {
		t = c.fracRuleThickness * em
		if display {
			up = max(c.fracNumDisplayShiftUp*em, c.fracNumDisplayGapMin*em+num.d+axis+t/2)
			down = max(c.fracDenomDisplayShiftDown*em, c.fracDenomDisplayGapMin*em+den.h-axis+t/2)
		} else {
			up = max(c.fracNumShiftUp*em, c.fracNumGapMin*em+num.d+axis+t/2)
			down = max(c.fracDenomShiftDown*em, c.fracDenomGapMin*em+den.h-axis+t/2)
		}
	} else {
		gapMin := c.stackGapMin * em
		up, down = c.stackTopShiftUp*em, c.stackBottomShiftDown*em
		if display {
			gapMin = c.stackDisplayGapMin * em
			up, down = c.stackTopDisplayShiftUp*em, c.stackBottomDisplayShiftDown*em
		}
		if gap := (up - num.d) - (den.h - down); gap < gapMin {
			up += (gapMin - gap) / 2
			down += (gapMin - gap) / 2
		}
	}
	pad := .12 * em
	w := max(num.w, den.w)
	out := box{w: w + 2*pad, h: up + num.h, d: down + den.d}
	out.place(num, pad+(w-num.w)/2, -up)
	out.place(den, pad+(w-den.w)/2, down)
	if n.rule {
		out.ops = append(out.ops, l.rect(pad, -axis-t/2, w, t))
	}
	if n.left == "" && n.right == "" {
		return out
	}
	size := out.h + out.d
	if display {
		size = max(size, 2.39*em)
	} else {
		size = max(size, 1.01*em)
	}
	out.w -= 2 * pad
	inner := box{w: w, h: out.h, d: out.d}
	inner.place(out, -pad, 0)
	return hbox(l.delimiter(n.left, size, st), inner, l.delimiter(n.right, size, st))
}

func (l *layout) radical(n *radical, st style) box {
	c := &l.f.c
	em := l.em(st)
	body := l.list(n.body, st.crampedOf())
	gap := c.radicalGap * em
	if st.level == 0 {
		gap = c.radicalDisplayGap * em
	}
	t := c.radicalRule * em
	g, _ := l.f.glyph('√')
	sign := l.stretch(g, body.h+body.d+gap+t, st, false)
	if delta := sign.h + sign.d - (body.h + body.d + gap + t); delta > 0 {
		gap += delta / 2
	}
	top := body.h + gap + t
	// The sign with its top at the rule's.
	sign = shifted(sign, top-sign.h)
	out := box{h: top + c.radicalExtraAscender*em, d: max(body.d, sign.d)}
	x := float32(0)
	if len(n.index) > 0 {
		idx := l.list(n.index, style{3, false})
		before := c.radicalKernBeforeDegree * em
		after := c.radicalKernAfterDegree * em
		raise := c.radicalDegreeBottomRaise*(sign.h+sign.d) - sign.d + idx.d
		x = max(before+idx.w+after, 0)
		out.place(idx, before, -raise)
		out.h = max(out.h, raise+idx.h)
	}
	out.place(sign, x, 0)
	x += sign.w
	out.place(body, x, 0)
	out.ops = append(out.ops, l.rect(x, -top, body.w, t))
	out.w = x + body.w
	return out
}

// stretch returns g's variant or assembly at least size high (or wide).
func (l *layout) stretch(g gid, size float32, st style, horizontal bool) box {
	em := l.em(st)
	base := l.glyphBox(g, st)
	have := base.h + base.d
	if horizontal {
		have = base.w
	}
	if have >= size {
		return base
	}
	cons := l.f.vert[g]
	if horizontal {
		cons = l.f.horiz[g]
	}
	if cons == nil {
		return base
	}
	best := base
	for _, v := range cons.variants {
		best = l.glyphBox(v.g, st)
		if v.advance*em >= size {
			return best
		}
	}
	if len(cons.parts) == 0 {
		return best
	}
	return l.assemble(cons.parts, size/em, st, horizontal)
}

func (l *layout) assemble(parts []part, size float32, st style, horizontal bool) box {
	em := l.em(st)
	minOverlap := l.f.overlap
	var seq []part
	for reps := 0; reps < 200; reps++ {
		seq = seq[:0]
		var full float32
		for _, p := range parts {
			n := 1
			if p.extender {
				n = reps
			}
			for i := 0; i < n; i++ {
				seq = append(seq, p)
				full += p.full
			}
		}
		if len(seq) > 0 && full-float32(len(seq)-1)*minOverlap >= size {
			break
		}
	}
	if len(seq) == 0 {
		return box{}
	}
	var full float32
	maxOverlap := float32(1e9)
	for i, p := range seq {
		full += p.full
		if i > 0 {
			maxOverlap = min(maxOverlap, seq[i-1].end, p.start)
		}
	}
	overlap := minOverlap
	if len(seq) > 1 {
		overlap = min(max((full-size)/float32(len(seq)-1), minOverlap), max(maxOverlap, minOverlap))
	}
	var out box
	pos := float32(0)
	for _, p := range seq {
		b := l.glyphBox(p.g, st)
		if horizontal {
			out.place(b, pos*em, 0)
			out.h, out.d = max(out.h, b.h), max(out.d, b.d)
		} else {
			out.place(b, 0, -(pos*em + b.d))
			out.w = max(out.w, b.w)
		}
		pos += p.full - overlap
	}
	total := (pos + overlap) * em
	if horizontal {
		out.w = total
	} else {
		// Bottom at the baseline: callers centre it.
		out.h, out.d = total, 0
	}
	return out
}

// delimiter returns the delimiter s at least size high, centred on the
// axis.
func (l *layout) delimiter(s string, size float32, st style) box {
	em := l.em(st)
	r := singleRune(s)
	if r == 0 {
		return kern(.12 * em)
	}
	g, ok := l.f.glyph(r)
	if alt, has := map[rune]rune{'|': '∣', '‖': '∥'}[r]; has && l.f.vert[g] == nil {
		if g2, ok2 := l.f.glyph(alt); ok2 {
			g, ok = g2, true
		}
	}
	if !ok {
		return l.symBox(&sym{text: s, font: fontRoman}, st)
	}
	b := l.stretch(g, size, st, false)
	axis := l.f.c.axisHeight * em
	return shifted(b, axis-(b.h-b.d)/2)
}

// leftDelimSize is TeX's size for \left and \right around h and d.
func (l *layout) leftDelimSize(h, d float32, st style) float32 {
	axis := l.f.c.axisHeight * l.em(st)
	delta := max(h-axis, d+axis)
	return max(2*delta*.901, 2*delta-.5*l.em(st))
}

func (l *layout) leftRight(n *leftRight, st style) box {
	saved := l.middle
	defer func() { l.middle = saved }()
	l.middle = 0
	body := l.list(n.body, st)
	size := l.leftDelimSize(body.h, body.d, st)
	for _, it := range n.body {
		if _, ok := it.(*middle); ok {
			l.middle = size
			body = l.list(n.body, st)
			break
		}
	}
	return hbox(l.delimiter(n.left, size, st), body, l.delimiter(n.right, size, st))
}

func (l *layout) op(n *opNode, st style) (box, bool) {
	em := l.em(st)
	if n.name != "" {
		var bs []box
		for _, r := range n.name {
			if g, ok := l.f.glyph(r); ok && r < 0x250 {
				b := l.glyphBox(g, st)
				b.italic = 0
				bs = append(bs, b)
			} else {
				return l.textBox(n.name, st, false, false, false), false
			}
		}
		b := hbox(bs...)
		b.char = false
		return b, false
	}
	g, ok := l.f.glyph(singleRune(n.text))
	if !ok {
		return l.textBox(n.text, st, false, false, false), false
	}
	b := l.glyphBox(g, st)
	if st.level == 0 {
		if cons := l.f.vert[g]; cons != nil {
			for _, v := range cons.variants {
				b = l.glyphBox(v.g, st)
				if v.advance >= l.f.c.displayOperatorMinHeight {
					break
				}
			}
		}
	}
	axis := l.f.c.axisHeight * em
	italic := b.italic
	b = shifted(b, axis-(b.h-b.d)/2)
	b.italic, b.char = italic, false
	return b, true
}

func (l *layout) scripts(n *scripts, st style) (class, box) {
	cls := Ord
	limits := false
	var base box
	switch b := n.base.(type) {
	case *opNode:
		cls = Op
		base, _ = l.op(b, st)
		limits = b.limits == 1 || b.limits == -1 && b.deflim && st.level == 0
		if b.limits == -1 && b.name == "" && b.deflim && st.level == 0 {
			limits = true
		}
	case *scriptsHolder:
		cls = Op
		base = l.list(b.body.items, st)
		limits = st.level == 0
	case *brace:
		cls = Op
		base = l.brace(b, st)
		limits = true
	default:
		cls, base = l.atom(n.base, st)
		if cls == Op {
			limits = st.level == 0
		}
	}
	var sup, sub box
	if n.hasSup {
		sup = l.list(n.sup, st.sup())
	}
	if n.hasSub {
		sub = l.list(n.sub, st.sub())
	}
	if limits {
		return cls, l.limits(base, sup, sub, n.hasSup, n.hasSub, st, base.italic)
	}
	return cls, l.attach(base, sup, sub, n.hasSup, n.hasSub, st, cls == Op)
}

// attach puts scripts beside base.
func (l *layout) attach(base, sup, sub box, hasSup, hasSub bool, st style, isOp bool) box {
	c := &l.f.c
	em := l.em(st)
	supEm := l.em(st.sup())
	var supUp, subDown float32
	if !base.char {
		supUp = base.h - c.supBaselineDropMax*supEm
		subDown = base.d + c.subBaselineDropMin*supEm
	}
	if hasSup {
		shift := c.supShiftUp * em
		if st.cramped {
			shift = c.supShiftUpCramped * em
		}
		supUp = max(supUp, shift, sup.d+c.supBottomMin*em)
	}
	if hasSub {
		subDown = max(subDown, c.subShiftDown*em, sub.h-c.subTopMax*em)
	}
	if hasSup && hasSub {
		if gap := (supUp - sup.d) - (sub.h - subDown); gap < c.subSupGapMin*em {
			subDown += c.subSupGapMin*em - gap
			if d := c.supBottomMaxWithSub*em - (supUp - sup.d); d > 0 {
				supUp += d
				subDown -= d
			}
			if gap := (supUp - sup.d) - (sub.h - subDown); gap < c.subSupGapMin*em {
				subDown += c.subSupGapMin*em - gap
			}
		}
	}
	out := base
	out.ops = append([]drawOp(nil), base.ops...)
	out.char, out.hasAcc = false, false
	after := c.spaceAfterScript * em
	w := base.w
	if hasSup {
		x := base.w
		if !isOp {
			x += base.italic
		}
		out.place(sup, x, -supUp)
		w = max(w, x+sup.w+after)
		out.h = max(out.h, supUp+sup.h)
	}
	if hasSub {
		x := base.w
		if isOp {
			x -= base.italic
		}
		out.place(sub, x, subDown)
		w = max(w, x+sub.w+after)
		out.d = max(out.d, subDown+sub.d)
	}
	out.w = w
	out.italic = 0
	return out
}

// limits puts sup above base and sub below, centred.
func (l *layout) limits(base, sup, sub box, hasSup, hasSub bool, st style, italic float32) box {
	c := &l.f.c
	em := l.em(st)
	w := max(base.w, sup.w, sub.w)
	out := box{w: w, h: base.h, d: base.d}
	out.place(base, (w-base.w)/2, 0)
	if hasSup {
		up := base.h + max(c.upperLimitBaselineRiseMin*em, c.upperLimitGapMin*em+sup.d)
		out.place(sup, (w-sup.w)/2+italic/2, -up)
		out.h = up + sup.h
	}
	if hasSub {
		down := base.d + max(c.lowerLimitBaselineDropMin*em, c.lowerLimitGapMin*em+sub.h)
		out.place(sub, (w-sub.w)/2-italic/2, down)
		out.d = down + sub.d
	}
	return out
}

func (l *layout) accent(n *accent, st style) box {
	c := &l.f.c
	em := l.em(st)
	body := l.list(n.body, st.crampedOf())
	g, ok := l.f.glyph(n.acc)
	if !ok {
		return body
	}
	var acc box
	if n.stretchy {
		acc = l.stretch(g, body.w, st, true)
	} else {
		acc = l.glyphBox(g, st)
	}
	attach := acc.accent
	if !acc.hasAcc {
		lft, rgt := l.f.ink(g)
		attach = (lft + rgt) / 2 * em
		if acc.w > 0 && !n.stretchy {
			attach = acc.w / 2
		}
		if n.stretchy {
			attach = acc.w / 2
		}
	}
	bodyAt := body.w / 2
	if body.char && body.hasAcc && !n.stretchy {
		bodyAt = body.accent
	}
	out := box{w: body.w, h: body.h, d: body.d, italic: body.italic, char: body.char}
	out.place(body, 0, 0)
	x := bodyAt - attach
	if n.acc >= 0x300 && n.acc < 0x370 || n.acc >= 0x20D0 && n.acc < 0x2100 {
		// A mark designed over the x-height: raised by what the base
		// stands taller.
		up := max(body.h-c.accentBaseHeight*em, 0)
		if n.under {
			out.place(acc, x, body.d+acc.h)
			out.d = body.d + acc.h + acc.d
			return out
		}
		out.place(acc, x, -up)
		out.h = max(body.h, up+acc.h)
		return out
	}
	// Arrows and other marks: just clear the base.
	gap := .1 * em
	if n.under {
		down := body.d + gap + acc.h
		out.place(acc, x, down)
		out.d = down + acc.d
		return out
	}
	up := max(body.h, c.accentBaseHeight*em) + gap + acc.d
	out.place(acc, x, -up)
	out.h = up + acc.h
	return out
}

func (l *layout) bar(n *bar, st style) box {
	c := &l.f.c
	em := l.em(st)
	body := l.list(n.body, st.crampedOf())
	out := box{w: body.w, h: body.h, d: body.d}
	out.place(body, 0, 0)
	if n.under {
		t := c.underbarRule * em
		y := body.d + c.underbarGap*em
		out.ops = append(out.ops, l.rect(0, y, body.w, t))
		out.d = y + t + c.underbarExtraDescender*em
		return out
	}
	t := c.overbarRule * em
	y := body.h + c.overbarGap*em + t
	out.ops = append(out.ops, l.rect(0, -y, body.w, t))
	out.h = y + c.overbarExtraAscender*em
	return out
}

func (l *layout) brace(n *brace, st style) box {
	em := l.em(st)
	body := l.list(n.body, st)
	g, ok := l.f.glyph(n.r)
	if !ok {
		return body
	}
	br := l.stretch(g, body.w, st, true)
	gap := .1 * em
	out := box{w: max(body.w, br.w), h: body.h, d: body.d}
	out.place(body, (out.w-body.w)/2, 0)
	if n.under {
		down := body.d + gap + br.h
		out.place(br, (out.w-br.w)/2, down)
		out.d = down + br.d
		return out
	}
	up := body.h + gap + br.d
	out.place(br, (out.w-br.w)/2, -up)
	out.h = up + br.h
	return out
}

func (l *layout) xarrow(n *stretchy, st style) box {
	em := l.em(st)
	over := l.list(n.over, st.sup())
	under := l.list(n.under, st.sub())
	w := max(over.w, under.w) + .6*em
	w = max(w, 1.2*em)
	var arrow box
	if n.r == '=' {
		t := l.f.c.fracRuleThickness * em
		axis := l.f.c.axisHeight * em
		arrow = box{w: w, h: axis + .12*em + t, d: 0}
		arrow.ops = append(arrow.ops, l.rect(0, -axis-.12*em-t/2, w, t), l.rect(0, -axis+.12*em-t/2, w, t))
	} else if g, ok := l.f.glyph(n.r); ok {
		arrow = l.stretch(g, w, st, true)
		if arrow.w < w {
			w = arrow.w
		}
	}
	w = max(w, arrow.w)
	gap := .12 * em
	out := box{w: w, h: arrow.h, d: arrow.d}
	out.place(arrow, (w-arrow.w)/2, 0)
	if len(n.over) > 0 {
		up := arrow.h + gap + over.d
		out.place(over, (w-over.w)/2, -up)
		out.h = up + over.h
	}
	if len(n.under) > 0 {
		down := arrow.d + gap + under.h
		out.place(under, (w-under.w)/2, down)
		out.d = down + under.d
	}
	pad := .14 * em
	res := box{w: w + 2*pad, h: out.h, d: out.d}
	res.place(out, pad, 0)
	return res
}

func (l *layout) array(n *array, st style) box {
	em := l.em(st)
	cell := style{1, false}
	switch n.kind {
	case arrAligned, arrGather:
		cell = style{0, false}
	case arrSmall:
		cell = style{2, false}
	}
	if st.level > cell.level {
		cell.level = st.level
	}
	cem := l.em(cell)
	strutH, strutD := .84*cem, .36*cem
	rowGap := float32(0)
	colGap := 1 * em
	switch n.kind {
	case arrAligned, arrGather:
		rowGap = .3 * em
	case arrCases:
		strutH, strutD = 1.008*cem, .432*cem
	case arrSmall:
		strutH, strutD = .7*cem, .3*cem
		colGap = .5 * em
	}
	cols := 0
	for _, r := range n.rows {
		cols = max(cols, len(r))
	}
	cells := make([][]box, len(n.rows))
	widths := make([]float32, cols)
	hs, ds := make([]float32, len(n.rows)), make([]float32, len(n.rows))
	for i, r := range n.rows {
		hs[i], ds[i] = strutH, strutD
		for j, items := range r {
			if n.kind == arrAligned && j%2 == 1 {
				items = append([]node{&group{}}, items...)
			}
			b := l.list(items, cell)
			cells[i] = append(cells[i], b)
			widths[j] = max(widths[j], b.w)
			hs[i], ds[i] = max(hs[i], b.h), max(ds[i], b.d)
		}
	}
	align := func(j int) byte {
		switch n.kind {
		case arrAligned:
			if j%2 == 0 {
				return 'r'
			}
			return 'l'
		case arrCases:
			return 'l'
		}
		if len(n.aligns) == 0 {
			return 'c'
		}
		if j < len(n.aligns) {
			return n.aligns[j]
		}
		return n.aligns[len(n.aligns)-1]
	}
	gapAfter := func(j int) float32 {
		switch n.kind {
		case arrAligned:
			if j%2 == 1 {
				return 2 * em
			}
			return 0
		}
		return colGap
	}
	xs := make([]float32, cols)
	x := float32(0)
	if n.kind == arrArray {
		x = .5 * em
	}
	for j := 0; j < cols; j++ {
		xs[j] = x
		x += widths[j]
		if j < cols-1 {
			x += gapAfter(j)
		}
	}
	if n.kind == arrArray {
		x += .5 * em
	}
	width := x
	var total float32
	for i := range n.rows {
		total += hs[i] + ds[i]
		if i > 0 {
			total += rowGap
		}
	}
	axis := l.f.c.axisHeight * em
	out := box{w: width, h: total/2 + axis, d: total/2 - axis}
	y := -out.h
	t := .04 * em
	for i, r := range cells {
		if i > 0 {
			y += rowGap
		}
		if i < len(n.hlines) && n.hlines[i] {
			out.ops = append(out.ops, l.rect(0, y-rowGap/2-t/2, width, t))
		}
		y += hs[i]
		for j, b := range r {
			cx := xs[j]
			switch align(j) {
			case 'c':
				cx += (widths[j] - b.w) / 2
			case 'r':
				cx += widths[j] - b.w
			}
			out.place(b, cx, y)
		}
		y += ds[i]
	}
	if len(n.hlines) > len(n.rows) && n.hlines[len(n.rows)] {
		out.ops = append(out.ops, l.rect(0, y-t/2, width, t))
	}
	for j, v := range n.vlines {
		if !v {
			continue
		}
		vx := float32(0)
		if j > 0 && j <= cols {
			vx = xs[j-1] + widths[j-1] + gapAfter(j-1)/2
			if j == cols {
				vx = width
			}
		}
		out.ops = append(out.ops, l.rect(vx-t/2, -out.h, t, out.h+out.d))
	}
	if n.left == "" && n.right == "" {
		return out
	}
	size := l.leftDelimSize(out.h, out.d, st)
	parts := []box{}
	if n.left != "" {
		parts = append(parts, l.delimiter(n.left, size, st))
		if n.kind == arrCases {
			parts = append(parts, kern(.17*em))
		}
	}
	parts = append(parts, out)
	if n.right != "" {
		if n.kind == arrCases {
			parts = append(parts, kern(.17*em))
		}
		parts = append(parts, l.delimiter(n.right, size, st))
	}
	return hbox(parts...)
}
