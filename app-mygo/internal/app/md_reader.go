package app

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
)

// md_reader.dart: long Markdown split into sections, with contents and
// in-document search.

// MdHeading is a heading of a reader document; Section is the section it
// opens.
type MdHeading struct {
	Level   int
	Title   string
	Section int
}

// MdDoc is Markdown split at headings (outside code fences), so long
// documents build lazily and the reader can jump to and track sections.
type MdDoc struct {
	Sections []string
	Headings []MdHeading
	lower    []string
}

var (
	mdHeadingRe = regexp.MustCompile(`^(#{1,4})\s+(.+?)\s*#*\s*$`)
	mdPlainRe   = regexp.MustCompile("[*_`]|\\$")
	mdLinkRe    = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
)

// NewMdDoc splits md.
func NewMdDoc(md string) *MdDoc {
	d := &MdDoc{}
	var buf strings.Builder
	fence := false
	for _, line := range strings.Split(md, "\n") {
		if mdFence.MatchString(line) {
			fence = !fence
		}
		if !fence {
			if m := mdHeadingRe.FindStringSubmatch(line); m != nil {
				if strings.TrimSpace(buf.String()) != "" || len(d.Sections) > 0 {
					d.Sections = append(d.Sections, buf.String())
				}
				buf.Reset()
				title := strings.TrimSpace(mdLinkRe.ReplaceAllString(mdPlainRe.ReplaceAllString(m[2], ""), "$1"))
				d.Headings = append(d.Headings, MdHeading{len(m[1]), title, len(d.Sections)})
			}
		}
		buf.WriteString(line + "\n")
	}
	if strings.TrimSpace(buf.String()) != "" {
		d.Sections = append(d.Sections, buf.String())
	}
	for _, s := range d.Sections {
		d.lower = append(d.lower, strings.ToLower(s))
	}
	return d
}

// Find returns the sections containing q (case-insensitive), in order.
func (d *MdDoc) Find(q string) []int {
	k := strings.ToLower(strings.TrimSpace(q))
	if k == "" {
		return nil
	}
	var out []int
	for i, s := range d.lower {
		if strings.Contains(s, k) {
			out = append(out, i)
		}
	}
	return out
}

// MdReader is MarkdownReader with its controller: a centred column of
// Markdown sections under a header, which a parent drives to jump, search
// and follow the position.
type MdReader struct {
	Doc       *MdDoc
	Header    func(c *ui.Context)
	LectureID string
	BaseDir   string
	OnRef     func(CoursiveRef)
	// MaxWidth replaces 760.
	MaxWidth float32
	// Padding replaces (lg, xl, huge, xl), in CSS order.
	Padding []float32
	// Current is the section at the top of the viewport; Progress the 0…1
	// reading progress.
	Current  int
	Progress float32

	list    ui.ListState
	flash   int
	flashed time.Time
	search  readerSearch
}

type readerSearch struct {
	q    string
	hits []int
	at   int
}

// NewMdReader starts a reader of doc, at the first passage containing
// initialQuery when it is not empty.
func NewMdReader(doc *MdDoc, initialQuery string) *MdReader {
	r := &MdReader{Doc: doc, flash: -1}
	if initialQuery != "" {
		r.Reveal(initialQuery)
	}
	return r
}

func (r *MdReader) offset() int {
	if r.Header != nil {
		return 1
	}
	return 0
}

// JumpToSection scrolls section i to the top; flash marks it a moment.
func (r *MdReader) JumpToSection(i int, flash bool) {
	r.list.ScrollTo(i+r.offset(), ui.Start)
	if flash {
		r.flash, r.flashed = i, time.Now()
		go func() { time.Sleep(1700 * time.Millisecond); core.Post(func() {}) }()
	}
}

// Reveal jumps to the first section containing q; false when absent.
func (r *MdReader) Reveal(q string) bool {
	hits := r.Doc.Find(q)
	if len(hits) == 0 {
		return false
	}
	r.JumpToSection(hits[0], true)
	return true
}

// Show builds the reader filling its parent.
func (r *MdReader) Show(c *ui.Context) ui.Element {
	cs, tt := design.S(), design.TT()
	if r.flash >= 0 && time.Since(r.flashed) > 1600*time.Millisecond {
		r.flash = -1
	}
	off := r.offset()
	n := len(r.Doc.Sections) + off
	maxW := r.MaxWidth
	if maxW == 0 {
		maxW = 760
	}
	pad := r.Padding
	if pad == nil {
		pad = []float32{design.SpaceLG, design.SpaceXL, design.SpaceHuge, design.SpaceXL}
	}
	st := tt.BodyLarge.H(1.75)
	list := ui.List(c, &r.list, n, func(k int) {
		ui.Box(c).FillWidth().AlignItems(ui.Center).Children(func() {
			col := ui.Box(c).FillWidth().MaxWidth(maxW)
			if r.Header != nil && k == 0 {
				col.Children(func() { r.Header(c) })
				return
			}
			i := k - off
			bg := ui.Color{}
			if r.flash == i {
				bg = design.WithOpacity(cs.TertiaryContainer, .45)
			}
			col = col.Padding(0, design.SpaceSM).Radius(design.CornerMD).
				Background(col.AnimateColor("flash", bg, design.T().Motion(design.Long2, design.Standard))).Selectable()
			col.Children(func() {
				CoursiveMarkdown{Data: r.Doc.Sections[i], Style: &st, LectureID: r.LectureID, BaseDir: r.BaseDir, OnRef: r.OnRef}.Show(c)
			})
		})
	}).Grow(1).FillWidth().Padding(pad...)
	first, last := r.list.Visible()
	if last >= first {
		cur := max(first-off, 0)
		prog := float32(1)
		if !r.list.AtEnd() && n > 1 {
			prog = float32(first) / float32(n-1)
		}
		r.Current, r.Progress = cur, prog
	}
	return list
}

// ShowToc is ReaderToc: the contents in the style of an M3 navigation
// drawer, indented by level, the section being read marked with a pill.
func (r *MdReader) ShowToc(c *ui.Context, onPicked func()) {
	cs, tt := design.S(), design.TT()
	hs := r.Doc.Headings
	if len(hs) == 0 {
		m3.Text(c, l10n.ReaderNoHeadings, tt.BodyMedium.C(cs.OnSurfaceVariant)).Padding(design.SpaceLG)
		return
	}
	top := hs[0].Level
	for _, h := range hs {
		top = min(top, h.Level)
	}
	active := 0
	for i, h := range hs {
		if h.Section <= r.Current {
			active = i
		}
	}
	ui.Scroll(c).Grow(1).FillWidth().Padding(design.SpaceSM, design.SpaceMD).Children(func() {
		for i, h := range hs {
			on := i == active
			depth := min(max(h.Level-top, 0), 3)
			row := ui.ButtonBase(c.Key(i)).FocusRing(false).FillWidth().Justify(ui.Start).Radius(design.CornerFull).
				Padding(10, design.SpaceLG, 10, design.SpaceLG+float32(depth)*design.SpaceMD).Margin(0, 0, 2, 0)
			if on {
				row = row.Background(cs.SecondaryContainer)
			}
			m3.InkLayer(row, cs.OnSurface, design.CornerFull, .08, .1, 0)
			if row.Clicked() {
				r.JumpToSection(h.Section, false)
				if onPicked != nil {
					onPicked()
				}
			}
			s := tt.BodyMedium
			color := cs.OnSurfaceVariant
			if depth == 0 {
				s, color = tt.LabelLarge, cs.OnSurface
			}
			if on {
				s, color = s.W(600), cs.OnSecondaryContainer
			}
			row.Children(func() { m3.Text(c, h.Title, s.C(color)).MaxLines(2) })
		}
	})
}

// ShowSearchBar is ReaderSearchBar: the query field, the match counter,
// previous and next. CloseSearch clears it for the next time.
func (r *MdReader) ShowSearchBar(c *ui.Context, onClose func()) {
	cs := design.S()
	st := &r.search
	goTo := func(d int) {
		if len(st.hits) == 0 {
			return
		}
		st.at = ((st.at+d)%len(st.hits) + len(st.hits)) % len(st.hits)
		r.JumpToSection(st.hits[st.at], true)
	}
	ui.Box(c).FillWidth().Padding(0, design.SpaceLG, design.SpaceSM, design.SpaceLG).Children(func() {
		in := m3.SearchBar{Hint: l10n.ReaderFindHint, AutoFocus: true, Height: 48, MaxWidth: -1,
			Leading: func() { m3.Icon(c, icons.SearchRounded, 24, cs.OnSurfaceVariant) },
			Trailing: func() {
				if st.q != "" {
					label := l10n.ReaderNoMatches
					if len(st.hits) > 0 {
						label = strconv.Itoa(st.at+1) + "/" + strconv.Itoa(len(st.hits))
					}
					m3.Text(c, label, design.TT().BodyMedium.C(cs.OnSurfaceVariant))
				}
				if (m3.IconButton{Icon: icons.KeyboardArrowUpRounded, Tooltip: l10n.ReaderPreviousMatch, Disabled: len(st.hits) == 0}).Show(c).Clicked() {
					goTo(-1)
				}
				if (m3.IconButton{Icon: icons.KeyboardArrowDownRounded, Tooltip: l10n.ReaderNextMatch, Disabled: len(st.hits) == 0}).Show(c).Clicked() {
					goTo(1)
				}
				if (m3.IconButton{Icon: icons.CloseRounded, Tooltip: l10n.CommonClose}).Show(c).Clicked() {
					onClose()
				}
			}}.Show(c, &st.q)
		if in.Changed() {
			st.hits, st.at = r.Doc.Find(st.q), 0
			if len(st.hits) > 0 {
				r.JumpToSection(st.hits[0], true)
			}
		}
		if in.Submitted() {
			goTo(1)
		}
	})
}

// CloseSearch forgets the query of the search bar.
func (r *MdReader) CloseSearch() { r.search = readerSearch{} }
