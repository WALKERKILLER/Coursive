package app

import (
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// coursive_markdown.dart and timestamp_links.dart: the app's Markdown, with
// coursive:// references as chips.

// RefKind is the kind of a coursive:// reference.
type RefKind int

const (
	// RefTime is a moment of a lecture; an empty LectureID is the lecture
	// in context.
	RefTime RefKind = iota
	// RefSlide is one of a lecture's slides.
	RefSlide
	// RefCourseware is a passage of uploaded courseware.
	RefCourseware
	// RefLecture is a whole lecture: the player opens where the user left
	// off.
	RefLecture
	// RefFile is a file of the course library, by its library-relative
	// path.
	RefFile
)

// CoursiveRef is a coursive:// reference inside Markdown.
type CoursiveRef struct {
	Kind        RefKind
	LectureID   string
	Seconds     int
	Idx         int
	File, Query string
	Path        string
}

// ParseTimeLink is TimeLink.parse: a coursive:// timestamp link.
func ParseTimeLink(raw string) (lectureID string, seconds int, ok bool) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "coursive" && u.Scheme != "coursive-app") {
		return "", 0, false
	}
	t, err := strconv.Atoi(u.Query().Get("t"))
	if err != nil {
		return "", 0, false
	}
	switch seg := segments(u); {
	case u.Host == "seek":
		return "", t, true
	case u.Host == "lecture" && len(seg) > 0:
		return seg[0], t, true
	}
	return "", 0, false
}

func segments(u *url.URL) []string {
	var out []string
	for _, s := range strings.Split(u.EscapedPath(), "/") {
		if s == "" {
			continue
		}
		if v, err := url.PathUnescape(s); err == nil {
			s = v
		}
		out = append(out, s)
	}
	return out
}

// ParseRef is CoursiveRef.parse.
func ParseRef(raw string) (CoursiveRef, bool) {
	u, err := url.Parse(raw)
	// coursive-app:// is the same link as exported for other programs.
	if err != nil || (u.Scheme != "coursive" && u.Scheme != "coursive-app") {
		return CoursiveRef{}, false
	}
	if id, t, ok := ParseTimeLink(raw); ok {
		return CoursiveRef{Kind: RefTime, LectureID: id, Seconds: t}, true
	}
	seg := segments(u)
	switch {
	case u.Host == "slide" && len(seg) >= 2:
		idx, err := strconv.Atoi(seg[1])
		if err != nil {
			return CoursiveRef{}, false
		}
		return CoursiveRef{Kind: RefSlide, LectureID: seg[0], Idx: idx}, true
	case u.Host == "courseware" && len(seg) > 0:
		q := u.Query()
		return CoursiveRef{Kind: RefCourseware, LectureID: seg[0], File: q.Get("file"), Query: q.Get("q")}, true
	case u.Host == "lecture" && len(seg) > 0:
		return CoursiveRef{Kind: RefLecture, LectureID: seg[0]}, true
	case u.Host == "file" && len(seg) > 0:
		return CoursiveRef{Kind: RefFile, Path: strings.Join(seg, "/")}, true
	}
	return CoursiveRef{}, false
}

const tsPattern = `(\d{1,2}:\d{2}(?::\d{2})?)`

var (
	// A range [a]–[b], or a single [a], not already followed by a link
	// target (checked by hand: Go's regexp has no lookahead).
	tsRef        = regexp.MustCompile(`\[` + tsPattern + `\](?:\s*[–—~-]\s*\[` + tsPattern + `\])?`)
	fenceSplit   = regexp.MustCompile("(?m)^(```|~~~)")
	inlineCodeRe = regexp.MustCompile("`[^`\n]*`")
)

// ParseClock is parseClock: "1:02:03" in seconds.
func ParseClock(s string) (int, bool) {
	n := 0
	for _, p := range strings.Split(s, ":") {
		v, err := strconv.Atoi(p)
		if err != nil {
			return 0, false
		}
		n = n*60 + v
	}
	return n, true
}

// LinkifyTimestamps rewrites bare [mm:ss], [h:mm:ss] and [a]–[b]
// references into coursive:// links so Markdown renders them as tappable
// timestamps. With lectureID the link names its lecture; otherwise it means
// "this lecture". Code spans and fenced blocks are left untouched.
func LinkifyTimestamps(md, lectureID string) string {
	if !strings.Contains(md, ":") {
		return md
	}
	var out strings.Builder
	at, inFence := 0, false
	for _, m := range fenceSplit.FindAllStringIndex(md, -1) {
		part := md[at:m[0]]
		if inFence {
			out.WriteString(part)
		} else {
			out.WriteString(linkText(part, lectureID))
		}
		out.WriteString(md[m[0]:m[1]])
		at, inFence = m[1], !inFence
	}
	if inFence {
		out.WriteString(md[at:])
	} else {
		out.WriteString(linkText(md[at:], lectureID))
	}
	return out.String()
}

func linkText(text, lectureID string) string {
	var b strings.Builder
	at := 0
	for _, m := range inlineCodeRe.FindAllStringIndex(text, -1) {
		b.WriteString(LinkifyPlain(text[at:m[0]], lectureID))
		b.WriteString(text[m[0]:m[1]])
		at = m[1]
	}
	b.WriteString(LinkifyPlain(text[at:], lectureID))
	return b.String()
}

// LinkifyPlain is LinkifyTimestamps for text known to hold no code or math.
func LinkifyPlain(s, lectureID string) string {
	var b strings.Builder
	at := 0
	for _, m := range tsRef.FindAllStringSubmatchIndex(s, -1) {
		if m[1] < len(s) && s[m[1]] == '(' {
			continue
		}
		start, ok := ParseClock(s[m[2]:m[3]])
		if !ok {
			continue
		}
		label, end := s[m[2]:m[3]], -1
		if m[4] >= 0 {
			if e, ok := ParseClock(s[m[4]:m[5]]); ok {
				end = e
			}
			label += "–" + s[m[4]:m[5]]
		}
		base := "coursive://seek"
		if lectureID != "" {
			base = "coursive://lecture/" + lectureID
		}
		b.WriteString(s[at:m[0]])
		b.WriteString("[" + label + "](" + base + "?t=" + strconv.Itoa(start))
		if end >= 0 {
			b.WriteString("&end=" + strconv.Itoa(end))
		}
		b.WriteString(")")
		at = m[1]
	}
	b.WriteString(s[at:])
	return b.String()
}

// Regions that must reach the renderer untouched, in priority order, plus
// the two repairs: bare math environments and $ x $ with padding spaces.
var (
	mdRegions = []*regexp.Regexp{
		regexp.MustCompile(`^\$\$[\s\S]*?\$\$`),
		regexp.MustCompile(`^\\\[[\s\S]*?\\\]`),
		regexp.MustCompile(`^\\\([\s\S]*?\\\)`),
		regexp.MustCompile("^`[^`\n]*`"),
	}
	mdEnv      = regexp.MustCompile(`^\\begin\{((?:equation|align|gather|multline|aligned|gathered|alignat|cases|array|split|[pbvBV]?matrix)\*?)\}`)
	mdSpaced   = regexp.MustCompile(`^\$ +([^$\n]+?) +\$`)
	mdInline   = regexp.MustCompile(`^\$[^$\n]+?\$`)
	mdFence    = regexp.MustCompile("^\\s*(```|~~~)")
	mdMathy    = regexp.MustCompile(`[\\^_=]`)
	mdCJK      = regexp.MustCompile(`[\x{3400}-\x{9fff}]`)
	coursiveMd = regexp.MustCompile(`\[([^\[\]\n]+)\]\((coursive(?:-app)?://[^)\s]+)\)`)
)

// PrepareMarkdown prepares model-written Markdown for rendering. Fenced and
// inline code and all math stay as written; bare \begin{aligned}… blocks
// get $$ delimiters, $ x $ loses its padding when it is clearly math, and
// [mm:ss] timestamps elsewhere become coursive:// links.
func PrepareMarkdown(md, lectureID string) string {
	var out, text strings.Builder
	fence := false
	flush := func() {
		if text.Len() > 0 {
			out.WriteString(prepareText(text.String(), lectureID))
			text.Reset()
		}
	}
	for _, line := range strings.Split(md, "\n") {
		isFence := mdFence.MatchString(line)
		if fence || isFence {
			flush()
			out.WriteString(line + "\n")
			if isFence {
				fence = !fence
			}
		} else {
			text.WriteString(line + "\n")
		}
	}
	flush()
	s := out.String()
	if strings.HasSuffix(md, "\n") {
		return s
	}
	return s[:len(s)-1]
}

func prepareText(s, lectureID string) string {
	var b strings.Builder
	plainFrom := 0
	emit := func(at int) { b.WriteString(LinkifyPlain(s[plainFrom:at], lectureID)) }
	for i := 0; i < len(s); {
		c := s[i]
		if c != '$' && c != '\\' && c != '`' {
			i++
			continue
		}
		rest := s[i:]
		matched := false
		for _, re := range mdRegions {
			if m := re.FindString(rest); m != "" {
				emit(i)
				b.WriteString(m)
				i += len(m)
				plainFrom, matched = i, true
				break
			}
		}
		if matched {
			continue
		}
		if m := mdEnv.FindStringSubmatch(rest); m != nil {
			end := `\end{` + m[1] + `}`
			if k := strings.Index(rest, end); k >= 0 {
				emit(i)
				b.WriteString("\n$$\n" + rest[:k+len(end)] + "\n$$\n")
				i += k + len(end)
				plainFrom = i
				continue
			}
		}
		prevOK := i == 0 || (s[i-1] != '\\' && s[i-1] != '$')
		if c == '$' && prevOK {
			if m := mdSpaced.FindStringSubmatch(rest); m != nil && !strings.HasPrefix(rest[len(m[0]):], "$") {
				emit(i)
				body := m[1]
				if mdMathy.MatchString(body) && !mdCJK.MatchString(body) {
					b.WriteString("$" + body + "$")
				} else {
					b.WriteString(m[0])
				}
				i += len(m[0])
				plainFrom = i
				continue
			}
			if m := mdInline.FindString(rest); m != "" && !strings.HasPrefix(rest[len(m):], "$") {
				emit(i)
				b.WriteString(m)
				i += len(m)
				plainFrom = i
				continue
			}
		}
		i++
	}
	emit(len(s))
	return b.String()
}

// chipTeX marks a coursive:// link's label so the view draws it as a chip:
// the link's text becomes math the InlineMath hook turns into the chip, and
// the link around it still takes the click.
const chipTeX = `\coursivechip`

func chipLinks(md string, lectureID string) string {
	var out strings.Builder
	fence := false
	for k, line := range strings.Split(md, "\n") {
		if k > 0 {
			out.WriteByte('\n')
		}
		if mdFence.MatchString(line) {
			fence = !fence
		}
		if fence || !strings.Contains(line, "coursive") {
			out.WriteString(line)
			continue
		}
		out.WriteString(coursiveMd.ReplaceAllStringFunc(line, func(m string) string {
			sub := coursiveMd.FindStringSubmatch(m)
			ref, ok := ParseRef(sub[2])
			if !ok {
				return m
			}
			kind := "t"
			switch ref.Kind {
			case RefTime:
				if ref.LectureID != "" && ref.LectureID != lectureID {
					kind = "x"
				}
			case RefSlide:
				kind = "s"
			case RefCourseware:
				kind = "c"
			case RefLecture:
				kind = "l"
			case RefFile:
				kind = "f"
			}
			label := strings.NewReplacer("$", "", "\\", "").Replace(sub[1])
			return "[$" + chipTeX + kind + " " + label + "$](" + sub[2] + ")"
		}))
	}
	return out.String()
}

var evidenceMd = regexp.MustCompile(`\[(\d+)\]\(coursive-evidence:([A-Za-z0-9_-]{1,12})\)`)

// evidenceLinks marks numbered citations so the view draws them as raised
// numbers, struck through when stale says the material no longer holds.
func evidenceLinks(md string, stale func(string) bool) string {
	return evidenceMd.ReplaceAllStringFunc(md, func(m string) string {
		sub := evidenceMd.FindStringSubmatch(m)
		kind := "e"
		if stale != nil && stale(sub[2]) {
			kind = "E"
		}
		return "[$" + chipTeX + kind + " " + sub[1] + "$](coursive-evidence:" + sub[2] + ")"
	})
}

// evidenceChip is _CitationMark's number: a small pill raised above the
// line.
func evidenceChip(label string, size float32, stale bool) markdown.InlineObject {
	cs := design.S()
	bg, fg := cs.Primary.Alpha(.1), cs.Primary
	if stale {
		bg, fg = cs.OnSurface.Alpha(.06), cs.Outline
	}
	span := ui.Span{Text: label, Size: size * .68, Weight: 700, Color: fg, Features: "tnum"}
	l := ui.ShapeRichText([]ui.Span{span}, ui.Font{Size: size * .68, Weight: 700}, 0)
	tw, th := l.Size()
	w := max(size*1.05, tw+7) + 3
	h := th + 1
	lift := size * .32
	return markdown.InlineObject{Width: w, Height: h + lift, Depth: 0, Draw: func(p *ui.Painter, r ui.Rect) {
		box := ui.Rect{X: r.X + 1.5, Y: r.Y, W: r.W - 3, H: h}
		p.Fill(box, bg, box.H/2)
		p.RichText(box.X+(box.W-tw)/2, box.Y+(box.H-th)/2, 0, span)
		if stale {
			p.Line(box.X+3, box.Y+box.H/2, box.X+box.W-3, box.Y+box.H/2, 1, fg)
		}
	}}
}

// drawChip makes the inline object of a chip: a timestamp in a pill, a
// reference in a tonal square.
func chipObject(tex string, size float32) (markdown.InlineObject, bool) {
	if !strings.HasPrefix(tex, chipTeX) || len(tex) < len(chipTeX)+2 {
		return markdown.InlineObject{}, false
	}
	kind, label := tex[len(chipTeX)], tex[len(chipTeX)+2:]
	cs := design.S()
	if kind == 'e' || kind == 'E' {
		return evidenceChip(label, size, kind == 'E'), true
	}
	bg, fg := cs.SecondaryContainer, cs.OnSecondaryContainer
	radius := size
	ic, icSize := icons.PlayArrowRounded, size*1.1
	switch kind {
	case 'x':
		ic, icSize = icons.OpenInNewRounded, size*.9
	case 's', 'c', 'l', 'f':
		bg, fg, radius = cs.TertiaryContainer, cs.OnTertiaryContainer, design.CornerSM
		icSize = size * .95
		ic = map[byte]icons.Icon{'s': icons.SlideshowOutlined, 'c': icons.DescriptionOutlined, 'l': icons.OndemandVideoRounded, 'f': icons.AttachFileRounded}[kind]
	}
	span := ui.Span{Text: label, Size: size * .88, Weight: 600, Color: fg, Features: "tnum"}
	l := ui.ShapeRichText([]ui.Span{span}, ui.Font{Size: size * .88, Weight: 600}, 0)
	tw, th := l.Size()
	w := 1 + 4 + icSize + 2 + tw + 7 + 1
	h := max(th, icSize) + 2
	asc := h * .78
	return markdown.InlineObject{Width: w, Height: asc, Depth: h - asc, Draw: func(p *ui.Painter, r ui.Rect) {
		box := ui.Rect{X: r.X + 1, Y: r.Y, W: r.W - 2, H: r.H}
		p.Fill(box, bg, min(radius, box.H/2))
		m3.IconPaint(p, ic, box.X+4, box.Y+(box.H-icSize)/2, icSize, fg)
		p.RichText(box.X+4+icSize+2, box.Y+(box.H-th)/2, 0, span)
	}}, true
}

// mdState is a view's options, kept between frames.
type mdState struct{ opts markdown.Options }

// CoursiveMarkdown is the app's Markdown renderer for chat, notes and
// parsed courseware: math, tappable timestamps and references, and images
// from the course library.
type CoursiveMarkdown struct {
	Data  string
	Style *design.TextStyle
	// LectureID is the lecture bare [mm:ss] timestamps refer to (empty: the
	// one in context).
	LectureID string
	// BaseDir is the library-relative folder relative image paths resolve
	// against (a note's folder, or a parsed courseware folder).
	BaseDir string
	OnRef   func(CoursiveRef)
	// OnImage replaces the zoomable viewer a tap on an image opens.
	OnImage   func(url string)
	Streaming bool
	// ShareSelection leaves the selection to a Selectable container
	// around, as one across a chat's messages.
	ShareSelection bool
	// OnEvidence follows a numbered citation, a link to
	// coursive-evidence:<id>; Stale tells the citations whose material
	// no longer holds.
	OnEvidence func(id string)
	Stale      func(id string) bool
}

// Show builds the Markdown in its parent's width.
func (m CoursiveMarkdown) Show(c *ui.Context) ui.Element {
	cs := design.S()
	st := design.TT().BodyMedium
	if m.Style != nil {
		st = *m.Style
	}
	text := st.Color
	if text.A == 0 {
		text = cs.OnSurface
	}
	w := ui.Box(c).FillWidth()
	s := ui.Local(w, "coursive.md", func() *mdState { return &mdState{} })
	o := &(*s).opts
	lh := float32(1.5)
	if st.Height > 0 {
		// Flutter's height is times the size; MyGo's times the font's own
		// line height, about 1.2 times the size.
		lh = st.Height / 1.2
	}
	o.Extensions = parser.GFM | parser.ExtFootnotes | parser.ExtMath | parser.ExtDollarMath | parser.ExtAlerts
	o.Theme = &markdown.Theme{
		FontSize: st.Size, Font: st.Family, LineHeight: lh,
		Text: text, Muted: cs.OnSurfaceVariant, Link: cs.Primary, Strong: text,
		Code: text, CodeBackground: cs.SurfaceContainerHighest,
		QuoteBar: cs.OutlineVariant, QuoteText: cs.OnSurfaceVariant, Rule: cs.OutlineVariant,
		CodeBlockBackground: cs.SurfaceContainerHigh, CodeBlockBorder: cs.OutlineVariant, CodeBlockRadius: design.CornerMD,
		TableBorder: cs.OutlineVariant, TableHeader: cs.SurfaceContainerHigh, Checkbox: cs.Primary,
	}
	o.ShareSelection = m.ShareSelection
	onRef := m.OnRef
	o.OnLink = func(u string) {
		if id, ok := strings.CutPrefix(u, "coursive-evidence:"); ok {
			if m.OnEvidence != nil {
				m.OnEvidence(id)
			}
			return
		}
		if ref, ok := ParseRef(u); ok && onRef != nil {
			onRef(ref)
			return
		}
		widgets.OpenPlainLink(c, u)
	}
	o.InlineMath = func(tex string, size float32) (markdown.InlineObject, bool) {
		if obj, ok := chipObject(tex, size); ok {
			return obj, true
		}
		return MathInline(tex, size, text)
	}
	o.Math = func(c *ui.Context, tex string) { MathBlock(c, tex, st.Size, text) }
	if o.LoadImage == nil {
		o.LoadImage = loadMarkdownImage
	}
	baseDir, onImage := m.BaseDir, m.OnImage
	o.Block = func(c *ui.Context, d *parser.Document, i int) bool {
		if b := d.Block(i); b.Kind == parser.CodeBlock && b.Fenced && !b.Open {
			return coursiveViewBlock(c, b.Language(), d.Lines(i))
		}
		return imageParagraph(c, d, i, baseDir, onImage)
	}
	src := PrepareMarkdown(m.Data, m.LectureID)
	if onRef != nil {
		src = chipLinks(src, m.LectureID)
	}
	if m.OnEvidence != nil {
		src = evidenceLinks(src, m.Stale)
	}
	w.Children(func() {
		if m.Streaming {
			markdown.Stream(c, src, o)
		} else {
			markdown.View(c, src, o)
		}
	})
	return w
}

// ResolveImageURL turns an image reference from Markdown into a fetchable
// URL.
func ResolveImageURL(raw, baseDir string) string {
	api := State.Api()
	if ref, ok := ParseRef(raw); ok && ref.Kind == RefSlide {
		return SlideImageURL(ref.LectureID, ref.Idx)
	}
	if strings.HasPrefix(raw, "coursive://file/") {
		rel, _ := url.PathUnescape(raw[len("coursive://file/"):])
		return api.FileURL(rel)
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "data:") {
		return raw
	}
	rel, _ := url.PathUnescape(raw)
	return api.FileURL(path.Clean(path.Join(baseDir, rel)))
}

// SlideImageURL is slideImageUrl.
func SlideImageURL(lectureID string, idx int) string {
	api := State.Api()
	return api.BaseURL + "/api/lectures/" + lectureID + "/slides/" + strconv.Itoa(idx) + "/image?token=" + api.Token
}

func loadMarkdownImage(u string) (ui.ImageSource, error) {
	if img := widgets.DataImage(u); img != nil {
		return img, nil
	}
	data, err := widgets.FetchURL(u)
	if err != nil {
		return nil, err
	}
	return ui.DecodeImage(data)
}

// imageParagraph builds a paragraph of images alone as _MdImage does: at
// most 420 DIPs high, rounded, opening the viewer on a tap.
func imageParagraph(c *ui.Context, d *parser.Document, i int, baseDir string, onImage func(string)) bool {
	b := d.Block(i)
	if b.Kind != parser.Paragraph {
		return false
	}
	in := d.Inlines(i)
	var urls []string
	for k := 0; k < len(in); k = in[k].Next {
		switch n := &in[k]; n.Kind {
		case parser.Image:
			if n.Pending {
				return false
			}
			urls = append(urls, n.Dest)
		case parser.SoftBreak, parser.LineBreak:
		case parser.Text:
			if strings.TrimSpace(n.Text) != "" {
				return false
			}
		default:
			return false
		}
	}
	if len(urls) == 0 {
		return false
	}
	ui.Column(c).FillWidth().Children(func() {
		for k, raw := range urls {
			u := ResolveImageURL(raw, baseDir)
			ui.Box(c.Key(k)).Padding(design.SpaceSM, 0).Children(func() { mdImage(c, u, onImage) })
		}
	})
	return true
}

func mdImage(c *ui.Context, u string, onImage func(string)) {
	cs := design.S()
	bmp, failed := widgets.NetBitmap(u)
	switch {
	case failed:
		ui.Row(c).AlignItems(ui.Center).Padding(design.SpaceMD).Background(cs.SurfaceContainerHighest).Radius(design.CornerMD).Children(func() {
			m3.Icon(c, icons.BrokenImageOutlined, 18, cs.OnSurfaceVariant)
			m3.HGap(c, design.SpaceSM)
			m3.Text(c, l10n.MarkdownImageUnavailable, design.TT().BodyMedium.C(cs.OnSurfaceVariant))
		})
	case bmp == nil:
		ui.Box(c).Height(48)
	default:
		iw, ih := bmp.Size()
		h := min(float32(ih), 420)
		img := ui.ButtonBase(c).FocusRing(false).Cursor(ui.CursorZoomIn).MaxWidthPercent(100).Height(h).Width(h * float32(iw) / float32(ih)).Radius(design.CornerMD).Clip()
		img = img.Opacity(img.AnimateTo("in", 1, design.T().Motion(design.Medium1, nil)))
		if img.Clicked() {
			if onImage != nil {
				onImage(u)
			} else {
				ShowImageViewer(u)
			}
		}
		img.Children(func() { ui.Image(c, bmp).Fill().Fit(ui.Contain) })
	}
}

type imageView struct {
	scale float32
	x, y  float32
}

// ShowImageViewer is showImageViewer: a full-window, zoomable image.
func ShowImageViewer(u string) {
	v := &imageView{scale: 1}
	m3.ShowGeneral(true, func(c *ui.Context, r *m3.Route) {
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Background(ui.RGBA(0, 0, 0, .87))
		area := ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Center().Clip()
		area.HandleInput(func(ev ui.InputEvent) bool {
			switch {
			case ev.Kind == ui.InputZoom:
				v.scale = min(max(v.scale*ev.Scale, 1), 6)
			case ev.Kind == ui.InputScroll && !ev.Precise:
				if ev.DY < 0 {
					v.scale = min(v.scale*1.25, 6)
				} else if ev.DY > 0 {
					v.scale = max(v.scale*.8, 1)
				}
			case ev.Kind == ui.InputScroll:
				v.x, v.y = v.x-ev.DX, v.y-ev.DY
			default:
				return false
			}
			if v.scale == 1 {
				v.x, v.y = 0, 0
			}
			return true
		})
		var off ui.Point
		area.Children(func() {
			bmp, _ := widgets.NetBitmap(u)
			if bmp == nil {
				m3.Spinner(c, 32, 3, ui.RGB(255, 255, 255))
				return
			}
			img := ui.Image(c, bmp).Fit(ui.Contain).MaxWidthPercent(100).MaxHeightPercent(100)
			if dx, dy, ok := img.Dragged(); ok && v.scale > 1 {
				v.x, v.y = v.x+dx, v.y+dy
			}
			img.Scale(v.scale, v.scale).Translate(v.x+off.X, v.y+off.Y)
		})
		ui.Box(c).Absolute().Top(design.SpaceLG).Right(design.SpaceLG).Children(func() {
			if (m3.IconButton{Kind: m3.TonalIcon, Icon: icons.CloseRounded, Tooltip: l10n.CommonClose}).Show(c).Clicked() {
				r.Close()
			}
		})
	}, nil)
}
