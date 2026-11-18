package tex

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// class is a TeX atom class, which sets the space between atoms.
type class uint8

const (
	Ord class = iota
	Op
	Bin
	Rel
	Open
	Close
	Punct
	Inner
)

type symbol struct {
	text string
	cls  class
}

// fontStyle is a math alphabet (\mathbf, \mathbb…).
type fontStyle uint8

const (
	fontDefault fontStyle = iota
	fontRoman
	fontItalic
	fontBold
	fontBoldItalic
	fontScript
	fontFraktur
	fontDouble
	fontSans
	fontSansBold
	fontMono
)

type node interface{}

type (
	// sym is a character or symbol command.
	sym struct {
		text string
		cls  class
		font fontStyle
	}
	// group is a braced list: one Ord atom.
	group struct {
		items []node
		cls   class
	}
	textRun struct {
		s    string
		bold bool
		ital bool
		bad  bool
	}
	frac struct {
		num, den    []node
		rule        bool
		style       int // -1 as the context, else the style
		left, right string
	}
	radical struct{ body, index []node }
	scripts struct {
		base     node
		sup, sub []node
		hasSup   bool
		hasSub   bool
	}
	opNode struct {
		text   string // a symbol, or
		name   string // an operator name
		limits int    // -1 as the operator says, 0 beside, 1 above and below
		deflim bool
	}
	leftRight struct {
		left, right string
		body        []node
	}
	middle   struct{ delim string }
	bigDelim struct {
		delim string
		size  int
		cls   class
	}
	accent struct {
		body     []node
		acc      rune
		stretchy bool
		under    bool
	}
	bar struct {
		body  []node
		under bool
	}
	space    struct{ em, mu float32 }
	styleSet struct{ style int }
	colorSet struct{ color string }
	colored  struct {
		color string
		body  []node
	}
	array struct {
		rows        [][][]node
		aligns      string
		vlines      []bool
		hlines      []bool
		kind        int // arrMatrix, arrAligned, arrCases, arrGather, arrSmall, arrArray
		left, right string
	}
	boxed   struct{ body []node }
	phantom struct {
		body []node
		w, v bool
	}
	smash struct{ body []node }
	stack struct {
		base        []node
		over, under []node
		cls         class
	}
	stretchy struct {
		r           rune
		over, under []node
	}
	brace struct {
		body  []node
		r     rune
		under bool
		label []node
	}
	cancel struct {
		body []node
		kind int
	}
	not      struct{ body node }
	ruleNode struct{ w, h float32 }
	tagNode  struct{ body []node }
)

const (
	arrMatrix = iota
	arrAligned
	arrCases
	arrGather
	arrSmall
	arrArray
)

type parser struct {
	s    string
	i    int
	font fontStyle
	// bad holds the commands it did not know.
	bad []string
}

func parse(s string) ([]node, []string) {
	p := &parser{s: s}
	out := p.list(func(t string) bool { return false })
	if t := p.peek(); t == `\\` || t == "&" {
		// Lines or alignment outside an environment, as models write
		// display math: aligned when it has &, else gathered.
		name := "gathered"
		if strings.Contains(s, "&") {
			name = "aligned"
		}
		q := &parser{s: s}
		return []node{q.env(name, s)}, q.bad
	}
	for p.i < len(p.s) {
		// A stray } or \right: skip it and go on.
		p.next()
		out = append(out, p.list(func(string) bool { return false })...)
	}
	return out, p.bad
}

// peek returns the next token without taking it: a command with its
// backslash, or one character.
func (p *parser) peek() string {
	j := p.i
	p.skipSpace()
	t := p.token()
	p.i = j
	return t
}

func (p *parser) skipSpace() {
	for p.i < len(p.s) {
		switch c := p.s[p.i]; {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			p.i++
		case c == '%':
			for p.i < len(p.s) && p.s[p.i] != '\n' {
				p.i++
			}
		default:
			return
		}
	}
}

func (p *parser) token() string {
	if p.i >= len(p.s) {
		return ""
	}
	if p.s[p.i] == '\\' {
		j := p.i + 1
		for j < len(p.s) && isLetter(p.s[j]) {
			j++
		}
		if j == p.i+1 && j < len(p.s) {
			_, n := utf8.DecodeRuneInString(p.s[j:])
			j += n
		}
		return p.s[p.i:j]
	}
	_, n := utf8.DecodeRuneInString(p.s[p.i:])
	return p.s[p.i : p.i+n]
}

func (p *parser) next() string {
	p.skipSpace()
	t := p.token()
	p.i += len(t)
	return t
}

func isLetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

// raw reads a braced argument as text.
func (p *parser) raw() string {
	p.skipSpace()
	if p.i >= len(p.s) {
		return ""
	}
	if p.s[p.i] != '{' {
		return p.next()
	}
	depth, start := 0, p.i+1
	for j := p.i; j < len(p.s); j++ {
		switch p.s[j] {
		case '\\':
			j++
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				p.i = j + 1
				return p.s[start:j]
			}
		}
	}
	p.i = len(p.s)
	return p.s[start:]
}

// optional reads [..] if there is one.
func (p *parser) optional() (string, bool) {
	p.skipSpace()
	if p.i >= len(p.s) || p.s[p.i] != '[' {
		return "", false
	}
	depth := 0
	for j := p.i; j < len(p.s); j++ {
		switch p.s[j] {
		case '{':
			depth++
		case '}':
			depth--
		case ']':
			if depth == 0 {
				v := p.s[p.i+1 : j]
				p.i = j + 1
				return v, true
			}
		}
	}
	return "", false
}

// arg parses one argument: a group, or one token.
func (p *parser) arg() []node {
	p.skipSpace()
	if p.i < len(p.s) && p.s[p.i] == '{' {
		p.i++
		items := p.list(func(t string) bool { return t == "}" })
		if p.peekRaw() == "}" {
			p.i++
		}
		return items
	}
	if p.i >= len(p.s) {
		return nil
	}
	if a := p.atom(); a != nil {
		return []node{a}
	}
	return nil
}

func (p *parser) peekRaw() string {
	if p.i >= len(p.s) {
		return ""
	}
	return p.token()
}

func (p *parser) sub(s string) []node {
	q := &parser{s: s, font: p.font}
	items := q.list(func(string) bool { return false })
	p.bad = append(p.bad, q.bad...)
	return items
}

// list parses atoms until stop says a token ends it (left unread), or the
// end. \over and its kin split it into a fraction.
func (p *parser) list(stop func(string) bool) []node {
	var out []node
	saved := p.font
	defer func() { p.font = saved }()
	for {
		t := p.peek()
		if t == "" || stop(t) || t == "}" || t == "&" || t == `\\` || t == `\cr` || t == `\end` || t == `\right` || t == `\middle` {
			return out
		}
		switch t {
		case `\over`, `\choose`, `\atop`, `\brace`, `\brack`:
			p.next()
			den := p.list(stop)
			f := &frac{num: out, den: den, rule: t == `\over`, style: -1}
			switch t {
			case `\choose`:
				f.left, f.right = "(", ")"
			case `\brace`:
				f.left, f.right = "{", "}"
			case `\brack`:
				f.left, f.right = "[", "]"
			}
			return []node{f}
		}
		a := p.atom()
		if a == nil {
			continue
		}
		out = append(out, p.scripts(a))
	}
}

// scripts takes the ^, _ and primes after a.
func (p *parser) scripts(a node) node {
	var s *scripts
	for {
		t := p.peek()
		switch t {
		case "^", "_":
			p.next()
			if s == nil {
				s = &scripts{base: a}
			}
			arg := p.arg()
			if t == "^" {
				s.sup, s.hasSup = append(s.sup, arg...), true
			} else {
				s.sub, s.hasSub = append(s.sub, arg...), true
			}
		case "'", "’":
			p.next()
			if s == nil {
				s = &scripts{base: a}
			}
			s.sup, s.hasSup = append([]node{&sym{text: "′", cls: Ord}}, s.sup...), true
		case `\limits`, `\nolimits`, `\displaylimits`:
			p.next()
			if o, ok := a.(*opNode); ok {
				o.limits = map[string]int{`\limits`: 1, `\nolimits`: 0, `\displaylimits`: -1}[t]
			}
		default:
			if s == nil {
				return a
			}
			return s
		}
	}
}

var fontCommands = map[string]fontStyle{
	`\mathrm`: fontRoman, `\mathup`: fontRoman, `\mathit`: fontItalic, `\mathbf`: fontBold, `\mathbfit`: fontBoldItalic,
	`\boldsymbol`: fontBoldItalic, `\bm`: fontBoldItalic, `\mathcal`: fontScript, `\mathscr`: fontScript,
	`\mathfrak`: fontFraktur, `\mathbb`: fontDouble, `\mathsf`: fontSans, `\mathbfsf`: fontSansBold, `\mathtt`: fontMono,
	`\mathnormal`: fontDefault, `\Bbb`: fontDouble,
}

var fontSwitches = map[string]fontStyle{
	`\rm`: fontRoman, `\it`: fontItalic, `\bf`: fontBold, `\cal`: fontScript, `\sf`: fontSans, `\tt`: fontMono,
}

var accents = map[string]rune{
	`\hat`: 0x302, `\check`: 0x30C, `\tilde`: 0x303, `\acute`: 0x301, `\grave`: 0x300, `\dot`: 0x307,
	`\ddot`: 0x308, `\dddot`: 0x20DB, `\ddddot`: 0x20DC, `\breve`: 0x306, `\bar`: 0x304, `\vec`: 0x20D7, `\mathring`: 0x30A,
}

var wideAccents = map[string]rune{
	`\widehat`: 0x302, `\widetilde`: 0x303, `\widecheck`: 0x30C, `\overrightarrow`: '→', `\overleftarrow`: '←',
	`\overleftrightarrow`: '↔', `\Overrightarrow`: '⇒', `\overrightharpoon`: '⇀', `\overleftharpoon`: '↼',
}

var underAccents = map[string]rune{
	`\underrightarrow`: '→', `\underleftarrow`: '←', `\underleftrightarrow`: '↔', `\utilde`: 0x330,
}

var braces = map[string]rune{
	`\overbrace`: '⏞', `\underbrace`: '⏟', `\overparen`: '⏜', `\underparen`: '⏝', `\overbracket`: '⎴', `\underbracket`: '⎵',
}

var xArrows = map[string]rune{
	`\xrightarrow`: '→', `\xleftarrow`: '←', `\xRightarrow`: '⇒', `\xLeftarrow`: '⇐', `\xleftrightarrow`: '↔',
	`\xLeftrightarrow`: '⇔', `\xmapsto`: '↦', `\xrightharpoonup`: '⇀', `\xrightharpoondown`: '⇁',
	`\xleftharpoonup`: '↼', `\xleftharpoondown`: '↽', `\xrightleftharpoons`: '⇌', `\xleftrightharpoons`: '⇋',
	`\xhookrightarrow`: '↪', `\xhookleftarrow`: '↩', `\xlongequal`: '=',
}

var namedOperators = map[string]bool{
	`\arcsin`: false, `\arccos`: false, `\arctan`: false, `\arctg`: false, `\arcctg`: false, `\arg`: false,
	`\ch`: false, `\cos`: false, `\cosec`: false, `\cosh`: false, `\cot`: false, `\cotg`: false, `\coth`: false,
	`\csc`: false, `\ctg`: false, `\cth`: false, `\deg`: false, `\dim`: false, `\exp`: false, `\hom`: false,
	`\ker`: false, `\lg`: false, `\ln`: false, `\log`: false, `\sec`: false, `\sin`: false, `\sinh`: false,
	`\sh`: false, `\tan`: false, `\tanh`: false, `\tg`: false, `\th`: false, `\det`: true, `\gcd`: true,
	`\inf`: true, `\lim`: true, `\liminf`: true, `\limsup`: true, `\max`: true, `\min`: true, `\Pr`: true,
	`\sup`: true, `\argmax`: true, `\argmin`: true,
}

var limitsByDefault = map[string]bool{
	`\sum`: true, `\prod`: true, `\coprod`: true, `\bigcup`: true, `\bigcap`: true, `\bigvee`: true,
	`\bigwedge`: true, `\bigoplus`: true, `\bigotimes`: true, `\bigodot`: true, `\biguplus`: true,
	`\bigsqcup`: true, `\bigsqcap`: true, `\intop`: true,
}

var spaces = map[string]float32{
	`\,`: 3, `\thinspace`: 3, `\:`: 4, `\>`: 4, `\medspace`: 4, `\;`: 5, `\thickspace`: 5, `\!`: -3,
	`\negthinspace`: -3, `\negmedspace`: -4, `\negthickspace`: -5, `\quad`: 18, `\qquad`: 36,
	`\ `: 6, `~`: 6, `\enspace`: 9, `\space`: 6, `\nobreakspace`: 6,
}

var bigSizes = map[string]int{
	`\big`: 1, `\Big`: 2, `\bigg`: 3, `\Bigg`: 4,
}

var classCommands = map[string]class{
	`\mathord`: Ord, `\mathop`: Op, `\mathbin`: Bin, `\mathrel`: Rel, `\mathopen`: Open,
	`\mathclose`: Close, `\mathpunct`: Punct, `\mathinner`: Inner,
}

var negations = map[string]string{
	"=": "≠", "<": "≮", ">": "≯", "≤": "≰", "≥": "≱", "∈": "∉", "∋": "∌", "⊂": "⊄", "⊃": "⊅",
	"⊆": "⊈", "⊇": "⊉", "≡": "≢", "∼": "≁", "≃": "≄", "≅": "≇", "≈": "≉", "∣": "∤", "∥": "∦",
	"→": "↛", "←": "↚", "↔": "↮", "⇒": "⇏", "⇐": "⇍", "⇔": "⇎", "∃": "∄", "≺": "⊀", "≻": "⊁",
	"⊢": "⊬", "⊨": "⊭", "⊑": "⋢", "⊒": "⋣", "◁": "⋪", "▷": "⋫",
}

// atom parses one atom, nil for what makes none (a switch it applied, a
// stray token).
func (p *parser) atom() node {
	t := p.next()
	if t == "" {
		return nil
	}
	switch t {
	case "{":
		items := p.list(func(t string) bool { return t == "}" })
		if p.peekRaw() == "}" {
			p.i++
		}
		return &group{items: items}
	case "}", "^", "_":
		return nil
	case "#":
		return nil
	}
	if t[0] != '\\' {
		return p.char(t)
	}
	if s, ok := spaces[t]; ok {
		return &space{mu: s}
	}
	if f, ok := fontCommands[t]; ok {
		saved := p.font
		p.font = f
		body := p.arg()
		p.font = saved
		return &group{items: body}
	}
	if f, ok := fontSwitches[t]; ok {
		p.font = f
		return nil
	}
	if a, ok := accents[t]; ok {
		return &accent{body: p.arg(), acc: a}
	}
	if a, ok := wideAccents[t]; ok {
		return &accent{body: p.arg(), acc: a, stretchy: true}
	}
	if a, ok := underAccents[t]; ok {
		return &accent{body: p.arg(), acc: a, stretchy: true, under: true}
	}
	if r, ok := braces[t]; ok {
		return &brace{body: p.arg(), r: r, under: strings.HasPrefix(t, `\under`)}
	}
	if r, ok := xArrows[t]; ok {
		under, _ := p.optional()
		return &stretchy{r: r, over: p.arg(), under: p.sub(under)}
	}
	if lim, ok := namedOperators[t]; ok {
		return &opNode{name: t[1:], limits: -1, deflim: lim}
	}
	if c, ok := classCommands[t]; ok {
		body := p.arg()
		if c == Op {
			return &scriptsHolder{&group{items: body, cls: Op}}
		}
		return &group{items: body, cls: c}
	}
	if size, ok := bigSizes[strings.TrimRight(t, "lrm")]; ok {
		cls := Ord
		switch t[len(t)-1] {
		case 'l':
			cls = Open
		case 'r':
			cls = Close
		case 'm':
			cls = Rel
		}
		return &bigDelim{delim: p.delim(), size: size, cls: cls}
	}
	switch t {
	case `\frac`, `\dfrac`, `\tfrac`, `\cfrac`:
		style := map[string]int{`\frac`: -1, `\dfrac`: 0, `\tfrac`: 1, `\cfrac`: 0}[t]
		return &frac{num: p.arg(), den: p.arg(), rule: true, style: style}
	case `\binom`, `\dbinom`, `\tbinom`:
		style := map[string]int{`\binom`: -1, `\dbinom`: 0, `\tbinom`: 1}[t]
		return &frac{num: p.arg(), den: p.arg(), style: style, left: "(", right: ")"}
	case `\genfrac`:
		l, r := p.raw(), p.raw()
		th := p.raw()
		st := p.raw()
		style := -1
		if n, err := strconv.Atoi(strings.TrimSpace(st)); err == nil {
			style = n
		}
		return &frac{left: delimText(l), right: delimText(r), rule: strings.TrimSpace(th) != "0pt", style: style, num: p.arg(), den: p.arg()}
	case `\sqrt`:
		idx, _ := p.optional()
		return &radical{index: p.sub(idx), body: p.arg()}
	case `\left`:
		return p.leftRight()
	case `\text`, `\textrm`, `\textup`, `\textnormal`, `\mbox`, `\hbox`, `\textsf`, `\texttt`, `\textmd`:
		return &textRun{s: textOf(p.raw())}
	case `\textbf`:
		return &textRun{s: textOf(p.raw()), bold: true}
	case `\textit`, `\emph`, `\textsl`:
		return &textRun{s: textOf(p.raw()), ital: true}
	case `\operatorname`, `\operatorname*`, `\operatornamewithlimits`:
		lim := strings.HasSuffix(t, "*") || t == `\operatornamewithlimits`
		if p.peekRaw() == "*" {
			p.i++
			lim = true
		}
		return &opNode{name: textOf(p.raw()), limits: -1, deflim: lim}
	case `\displaystyle`:
		return &styleSet{0}
	case `\textstyle`:
		return &styleSet{1}
	case `\scriptstyle`:
		return &styleSet{2}
	case `\scriptscriptstyle`:
		return &styleSet{3}
	case `\color`:
		c := p.raw()
		return &colorSet{color: c}
	case `\textcolor`, `\colorbox`:
		c := p.raw()
		return &colored{color: c, body: p.arg()}
	case `\overline`:
		return &bar{body: p.arg()}
	case `\underline`:
		return &bar{body: p.arg(), under: true}
	case `\boxed`, `\fbox`:
		return &boxed{body: p.arg()}
	case `\phantom`:
		return &phantom{body: p.arg(), w: true, v: true}
	case `\hphantom`:
		return &phantom{body: p.arg(), w: true}
	case `\vphantom`:
		return &phantom{body: p.arg(), v: true}
	case `\smash`:
		p.optional()
		return &smash{body: p.arg()}
	case `\mathstrut`, `\strut`:
		return &phantom{body: []node{&sym{text: "(", cls: Ord}}, v: true}
	case `\overset`, `\stackrel`:
		over := p.arg()
		base := p.arg()
		cls := Ord
		if t == `\stackrel` {
			cls = Rel
		}
		return &stack{base: base, over: over, cls: cls}
	case `\underset`:
		under := p.arg()
		return &stack{base: p.arg(), under: under}
	case `\substack`:
		return p.env("subarray", p.raw())
	case `\not`:
		next := p.atom()
		if s, ok := next.(*sym); ok {
			if n, ok := negations[s.text]; ok {
				return &sym{text: n, cls: Rel}
			}
		}
		return &not{body: next}
	case `\cancel`, `\bcancel`, `\xcancel`, `\sout`:
		return &cancel{body: p.arg(), kind: map[string]int{`\cancel`: 0, `\bcancel`: 1, `\xcancel`: 2, `\sout`: 3}[t]}
	case `\begin`:
		name := strings.TrimSpace(p.raw())
		return p.env(name, "")
	case `\pmod`:
		body := p.arg()
		return &group{items: append([]node{&space{mu: 18}, &sym{text: "(", cls: Open}, &opNode{name: "mod", limits: 0}, &space{mu: 6}}, append(body, &sym{text: ")", cls: Close})...)}
	case `\bmod`:
		return &group{items: []node{&opNode{name: "mod", limits: 0}}, cls: Bin}
	case `\mod`:
		return &group{items: []node{&space{mu: 18}, &opNode{name: "mod", limits: 0}, &space{mu: 6}}}
	case `\tag`, `\tag*`:
		return &tagNode{body: []node{&textRun{s: "(" + textOf(p.raw()) + ")"}}}
	case `\label`, `\ref`, `\eqref`:
		p.raw()
		return nil
	case `\nonumber`, `\notag`, `\nolimits`, `\limits`, `\displaylimits`, `\relax`, `\middle`, `\right`, `\hline`, `\allowbreak`, `\nobreak`, `\newline`, `\protect`:
		return nil
	case `\hspace`, `\hspace*`, `\hskip`, `\kern`, `\mkern`, `\mskip`:
		v := p.raw()
		if t == `\kern` || t == `\mkern` || t == `\hskip` || t == `\mskip` {
			v = p.length(v)
		}
		em, mu := parseLength(v)
		return &space{em: em, mu: mu}
	case `\rule`:
		p.optional()
		w, _ := parseLength(p.raw())
		h, _ := parseLength(p.raw())
		return &ruleNode{w: w, h: h}
	case `\href`:
		p.raw()
		return &group{items: p.arg()}
	case `\url`:
		return &textRun{s: p.raw()}
	case `\unicode`:
		v := p.raw()
		n, err := strconv.ParseInt(strings.TrimPrefix(strings.TrimPrefix(v, "0x"), "x"), map[bool]int{true: 16, false: 10}[strings.HasPrefix(v, "0x") || strings.HasPrefix(v, "x")], 32)
		if err != nil {
			return nil
		}
		return &sym{text: string(rune(n)), cls: Ord}
	case `\dots`, `\dotsc`, `\dotso`, `\dotsi`:
		return &sym{text: "…", cls: Inner}
	case `\dotsb`, `\dotsm`:
		return &sym{text: "⋯", cls: Inner}
	case `\lt`:
		return &sym{text: "<", cls: Rel}
	case `\gt`:
		return &sym{text: ">", cls: Rel}
	case `\{`:
		return &sym{text: "{", cls: Open}
	case `\}`:
		return &sym{text: "}", cls: Close}
	case `\\`:
		return nil
	}
	if s, ok := mathSymbols[t]; ok {
		if s.cls == Op {
			return &opNode{text: s.text, limits: -1, deflim: limitsByDefault[t]}
		}
		return &sym{text: s.text, cls: s.cls, font: p.symFont()}
	}
	if s, ok := textSymbols[t]; ok {
		return &sym{text: s, cls: Ord}
	}
	p.bad = append(p.bad, t)
	return &textRun{s: t, bad: true}
}

// scriptsHolder is \mathop{…}: an operator its scripts go above and below
// in display style.
type scriptsHolder struct{ body *group }

func (p *parser) symFont() fontStyle {
	if p.font == fontBold || p.font == fontBoldItalic {
		return fontBold
	}
	return fontDefault
}

// length reads a TeX length that follows \kern without braces.
func (p *parser) length(v string) string {
	if v == "" || v == "-" || strings.IndexFunc(v, unicode.IsLetter) < 0 {
		p.skipSpace()
		j := p.i
		for j < len(p.s) && isLetter(p.s[j]) {
			j++
		}
		v += p.s[p.i:j]
		p.i = j
	}
	return v
}

// parseLength turns a TeX length into ems or mus.
func parseLength(v string) (em, mu float32) {
	v = strings.TrimSpace(v)
	j := 0
	for j < len(v) && (v[j] == '-' || v[j] == '+' || v[j] == '.' || v[j] >= '0' && v[j] <= '9') {
		j++
	}
	n, err := strconv.ParseFloat(v[:j], 32)
	if err != nil {
		return 0, 0
	}
	f := float32(n)
	switch strings.TrimSpace(v[j:]) {
	case "mu":
		return 0, f
	case "em":
		return f, 0
	case "ex":
		return f * .43, 0
	case "pt":
		return f / 10, 0
	case "px":
		return f / 10, 0
	case "mm":
		return f * 2.845 / 10, 0
	case "cm":
		return f * 28.45 / 10, 0
	case "in":
		return f * 72.27 / 10, 0
	}
	return f / 10, 0
}

// textOf turns \text's argument into its text: symbol commands become
// their characters, braces and math stay as written without the $.
func textOf(s string) string {
	if !strings.ContainsAny(s, `\{}$~`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '{', '}', '$':
		case '~':
			b.WriteByte(' ')
		case '\\':
			j := i + 1
			for j < len(s) && isLetter(s[j]) {
				j++
			}
			if j == i+1 && j < len(s) {
				j++
			}
			cmd := s[i:j]
			if t, ok := textSymbols[cmd]; ok {
				b.WriteString(t)
			} else if m, ok := mathSymbols[cmd]; ok {
				b.WriteString(m.text)
			} else if sp, ok := spaces[cmd]; ok && sp > 0 {
				b.WriteByte(' ')
			}
			for j < len(s) && s[j] == ' ' && j > i+2 {
				j++
				break
			}
			i = j - 1
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

func (p *parser) char(t string) node {
	r, _ := utf8.DecodeRuneInString(t)
	switch {
	case t == "-" || t == "−":
		return &sym{text: "−", cls: Bin}
	case t == "*":
		return &sym{text: "∗", cls: Bin}
	case t == "~":
		return &space{mu: 6}
	case t == "`" || t == "\"":
		return &sym{text: t, cls: Ord}
	}
	if c, ok := literalClass[t]; ok {
		return &sym{text: t, cls: c, font: p.font}
	}
	if w, ok := wideDelims[t]; ok {
		return &sym{text: w.text, cls: w.cls}
	}
	if r >= 0x2E80 {
		// CJK, typed into math: text.
		j := p.i
		for j < len(p.s) {
			r2, n := utf8.DecodeRuneInString(p.s[j:])
			if r2 < 0x2E80 {
				break
			}
			j += n
		}
		s := t + p.s[p.i:j]
		p.i = j
		return &textRun{s: s}
	}
	return &sym{text: t, cls: unicodeClass(t), font: p.font}
}

var literalClass = map[string]class{
	"+": Bin, "=": Rel, "<": Rel, ">": Rel, ":": Rel, "(": Open, "[": Open, ")": Close, "]": Close,
	",": Punct, ";": Punct, "!": Close, "?": Close, ".": Ord, "/": Ord, "|": Ord, "@": Ord, "'": Ord,
}

var wideDelims = map[string]symbol{
	"（": {"(", Open}, "）": {")", Close}, "［": {"[", Open}, "］": {"]", Close}, "｛": {"{", Open}, "｝": {"}", Close},
	"，": {",", Punct}, "：": {":", Rel}, "；": {";", Punct}, "＝": {"=", Rel}, "＋": {"+", Bin},
}

var unicodeClasses map[string]class

func unicodeClass(t string) class {
	if unicodeClasses == nil {
		unicodeClasses = map[string]class{}
		for _, s := range mathSymbols {
			if _, ok := unicodeClasses[s.text]; !ok {
				unicodeClasses[s.text] = s.cls
			}
		}
		for k, v := range map[string]class{"⋅": Bin, "·": Bin, "⟂": Rel, "†": Bin, "‡": Bin, "∣": Rel, "∥": Rel, "|": Ord} {
			unicodeClasses[k] = v
		}
	}
	return unicodeClasses[t]
}

// delim reads a delimiter after \left, \right, \middle or \big.
func (p *parser) delim() string {
	return delimText(p.next())
}

// delimText is the delimiter t names, "" for none.
func delimText(t string) string {
	switch t {
	case ".", "":
		return ""
	case "<", `\lt`, `\langle`:
		return "⟨"
	case ">", `\gt`, `\rangle`:
		return "⟩"
	case "|", `\vert`, `\lvert`, `\rvert`, `\arrowvert`:
		return "|"
	case `\|`, `\Vert`, `\lVert`, `\rVert`, `\Arrowvert`:
		return "‖"
	case `\{`, `\lbrace`:
		return "{"
	case `\}`, `\rbrace`:
		return "}"
	case `\backslash`:
		return `\`
	case `\lbrack`:
		return "["
	case `\rbrack`:
		return "]"
	}
	if w, ok := wideDelims[t]; ok {
		return w.text
	}
	if s, ok := mathSymbols[t]; ok {
		return s.text
	}
	if t[0] == '\\' {
		return ""
	}
	return t
}

func (p *parser) leftRight() node {
	lr := &leftRight{left: p.delim()}
	for {
		lr.body = append(lr.body, p.list(func(string) bool { return false })...)
		switch p.peek() {
		case `\middle`:
			p.next()
			lr.body = append(lr.body, &middle{delim: p.delim()})
			continue
		case `\right`:
			p.next()
			lr.right = p.delim()
			return lr
		case "":
			return lr
		default:
			// A & or \\ inside \left…\right, which TeX refuses: keep it.
			p.next()
		}
	}
}

// env parses \begin{name}…\end{name}; body, when set, is the content
// instead (\substack).
func (p *parser) env(name, body string) node {
	q := p
	if body != "" {
		q = &parser{s: body}
	}
	a := &array{kind: arrMatrix, aligns: "c"}
	switch strings.TrimSuffix(name, "*") {
	case "matrix":
	case "pmatrix":
		a.left, a.right = "(", ")"
	case "bmatrix":
		a.left, a.right = "[", "]"
	case "Bmatrix":
		a.left, a.right = "{", "}"
	case "vmatrix":
		a.left, a.right = "|", "|"
	case "Vmatrix":
		a.left, a.right = "‖", "‖"
	case "smallmatrix":
		a.kind = arrSmall
	case "subarray":
		a.kind = arrSmall
		if body == "" {
			a.aligns = strings.TrimSpace(q.raw())
		}
	case "cases", "dcases":
		a.kind, a.aligns, a.left = arrCases, "ll", "{"
	case "rcases", "drcases":
		a.kind, a.aligns, a.right = arrCases, "ll", "}"
	case "aligned", "align", "alignat", "alignedat", "flalign", "split", "eqnarray":
		a.kind, a.aligns = arrAligned, "rl"
		if strings.HasPrefix(name, "alignat") || strings.HasPrefix(name, "alignedat") {
			q.raw()
		}
	case "gather", "gathered", "equation", "multline", "displaymath", "center":
		a.kind = arrGather
	case "array", "darray":
		a.kind = arrArray
		a.aligns = ""
		for _, c := range q.raw() {
			switch c {
			case 'l', 'c', 'r':
				a.aligns += string(c)
				a.vlines = append(a.vlines, false)
			case '|':
				for len(a.vlines) <= len(a.aligns) {
					a.vlines = append(a.vlines, false)
				}
				a.vlines[len(a.aligns)] = true
			case 'p', 'm', 'b':
				a.aligns += "l"
				a.vlines = append(a.vlines, false)
			}
		}
		for len(a.vlines) <= len(a.aligns) {
			a.vlines = append(a.vlines, false)
		}
	default:
		if body == "" {
			p.bad = append(p.bad, `\begin{`+name+`}`)
		}
	}
	row := [][]node{}
	a.hlines = []bool{false}
	for {
		for q.peek() == `\hline` || q.peek() == `\hdashline` {
			q.next()
			a.hlines[len(a.hlines)-1] = true
		}
		cell := q.list(func(string) bool { return false })
		row = append(row, cell)
		switch q.peek() {
		case "&":
			q.next()
			continue
		case `\\`, `\cr`:
			q.next()
			q.optional()
			a.rows = append(a.rows, row)
			a.hlines = append(a.hlines, false)
			row = nil
			continue
		case `\end`:
			q.next()
			q.raw()
		case "}", `\right`, `\middle`:
			if body == "" && q.i < len(q.s) {
				// Stray; skip it and go on.
				q.next()
				continue
			}
		}
		if len(row) > 1 || len(row) == 1 && len(row[0]) > 0 || len(a.rows) == 0 {
			a.rows = append(a.rows, row)
		} else {
			// A trailing \\ before \end.
			a.hlines = a.hlines[:len(a.hlines)-1]
		}
		for len(a.hlines) <= len(a.rows) {
			a.hlines = append(a.hlines, false)
		}
		return a
	}
}
