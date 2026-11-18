package study

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
)

// A reviewed outline as semantic blocks: the single source for the
// lecture note's Markdown and the exported PDF (spec 0028;
// outline_document.dart).

// OutlineChoices are the meanings a learner can give a handwritten mark,
// in display order.
var OutlineChoices = []string{"mastered", "key", "review", "custom", "keep"}

// OutlineChoiceLabel names a mark's meaning.
func OutlineChoiceLabel(choice, custom string) string {
	switch choice {
	case "mastered":
		return l10n.OutlineChoiceMastered
	case "key":
		return l10n.OutlineChoiceKey
	case "review":
		return l10n.OutlineChoiceReview
	case "custom":
		if c := strings.TrimSpace(custom); c != "" {
			return c
		}
		return l10n.OutlineChoiceCustom
	}
	return l10n.OutlineChoiceKeep
}

// OutlineProposal is the outline proposal of a turn, or nil.
func OutlineProposal(turn M) M {
	p := obj(obj(turn["result"])["outline_proposal"])
	if p != nil && num(p["version"], 0) == 1 {
		return p
	}
	return nil
}

// OutlineItems are the items of every section.
func OutlineItems(proposal M) []M {
	var out []M
	for _, s := range list(proposal["sections"]) {
		for _, i := range list(obj(s)["items"]) {
			if m, ok := i.(M); ok {
				out = append(out, m)
			}
		}
	}
	return out
}

// OutlineEvidenceIDs are the evidence an item cites.
func OutlineEvidenceIDs(item M) []string { return strs(item["evidence_ids"]) }

// OutlineVerified reports a fact one of whose citations still holds.
func OutlineVerified(item M, stale map[string]bool) bool {
	for _, id := range OutlineEvidenceIDs(item) {
		if !stale[id] {
			return true
		}
	}
	return false
}

// OutlineStats counts an outline's review state.
type OutlineStats struct{ Reviewed, Total, Stale, Marks, Unverified int }

// CountOutline is outlineStats.
func CountOutline(proposal M, stale map[string]bool) OutlineStats {
	var s OutlineStats
	for _, i := range OutlineItems(proposal) {
		s.Total++
		if i["review"] == "confirmed" {
			s.Reviewed++
		}
		if i["review"] == "needs_interpretation" {
			s.Marks++
		}
		for _, id := range OutlineEvidenceIDs(i) {
			if stale[id] {
				s.Stale++
				break
			}
		}
		if i["kind"] == "fact" && !OutlineVerified(i, stale) {
			s.Unverified++
		}
	}
	return s
}

// OutlineNoteHash is FNV-1a 64 of a saved note body; a different hash
// later means the note changed elsewhere.
func OutlineNoteHash(body string) string {
	h := uint64(0xcbf29ce484222325)
	for _, b := range []byte(body) {
		h = (h ^ uint64(b)) * 0x100000001b3
	}
	return fmt.Sprintf("%016x", h)
}

// OutlineNoteID is the note an outline is saved as; retries reuse it.
func OutlineNoteID(proposal M) string { return "outline-" + str(proposal["proposal_id"]) }

// OutlineBlock is a heading, an entry or a remark.
type OutlineBlock interface{ outlineBlock() }

// OutlineHeading is a heading of level 2 to 4; the title is level 1.
type OutlineHeading struct {
	Level int
	Text  string
}

// OutlineEntry is a list entry: an item, conflict, question or unread
// source. Item is the outline item it shows; marks find their ink by it.
type OutlineEntry struct {
	Text string
	Refs []int
	Tag  string
	Item M
}

// OutlineRemark is a short line under a heading.
type OutlineRemark struct{ Text string }

func (OutlineHeading) outlineBlock() {}
func (OutlineEntry) outlineBlock()   {}
func (OutlineRemark) outlineBlock()  {}

// OutlineSource is a cited source, numbered by first use.
type OutlineSource struct {
	Number                      int
	Label, Locator, Quote, Link string
	Stale                       bool
}

// OutlineDoc is OutlineExport: what a reviewed outline says.
type OutlineDoc struct {
	Title          string
	Body, Appendix []OutlineBlock
	Sources        []OutlineSource
}

// Blocks are the body then the appendix.
func (d OutlineDoc) Blocks() []OutlineBlock {
	return append(append([]OutlineBlock(nil), d.Body...), d.Appendix...)
}

func locator(e M) string {
	a := obj(e["anchor"])
	switch {
	case str(e["kind"]) == "timestamped_transcript":
		return core.FmtClock(toInt(a["time_start_ms"])) + "–" + core.FmtClock(toInt(a["time_end_ms"]))
	case str(e["kind"]) == "visual_observation":
		return l10n.StudyEvidenceSeen
	case a["kind"] == "ink":
		return l10n.StudyEvidenceInk
	case toInt(a["page"]) > 0:
		return l10n.StudySourcePage(toInt(a["page"]))
	}
	return ""
}

// EncodeComponent is Dart's Uri.encodeComponent.
func EncodeComponent(s string) string {
	var b strings.Builder
	for _, c := range []byte(s) {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.IndexByte("-_.!~*'()", c) >= 0 {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// link is a coursive:// link back to the cited place, when the app can
// open it.
func link(e M, lectureID string) string {
	a := obj(e["anchor"])
	if a["kind"] == "video" && a["object_id"] == "lecture" {
		lecture := lectureID
		if w := strOr(a["workspace_id"]); w != "" {
			lecture = w
		}
		return fmt.Sprintf("coursive://lecture/%s?t=%d", EncodeComponent(lecture), toInt(a["time_start_ms"])/1000)
	}
	path := strOr(a["original_path"])
	if path == "" {
		return ""
	}
	parts := strings.Split(path, "/")
	for i, p := range parts {
		parts[i] = EncodeComponent(p)
	}
	return "coursive://file/" + strings.Join(parts, "/")
}

// ExportOutline is outlineExport: the reviewed outline split into the
// body the learner accepts as notes and an appendix of what still needs
// checking: conflicts, uninterpreted marks, items marked for checking,
// open questions and sources that were not read.
func ExportOutline(proposal M, evidence []any, stale map[string]bool, lectureID string) OutlineDoc {
	byID := map[string]M{}
	for _, e := range evidence {
		if m, ok := e.(M); ok {
			byID[str(m["id"])] = m
		}
	}
	numbers := map[string]int{}
	var sources []OutlineSource
	refs := func(ids []string) []int {
		var out []int
		for _, id := range ids {
			e, ok := byID[id]
			if !ok {
				continue
			}
			n, seen := numbers[id]
			if !seen {
				sources = append(sources, OutlineSource{
					Number: len(sources) + 1, Label: strings.TrimSpace(strOr(e["label"])), Locator: locator(e),
					Quote: strings.TrimSpace(strOr(obj(e["anchor"])["quote"])), Link: link(e, lectureID),
					Stale: stale[id] || e["status"] != "verified",
				})
				n = len(sources)
				numbers[id] = n
			}
			out = append(out, n)
		}
		return out
	}
	var body []OutlineBlock
	var marks, checks []OutlineBlock
	for _, sv := range list(proposal["sections"]) {
		s, ok := sv.(M)
		if !ok {
			continue
		}
		heading := strings.TrimSpace(strOr(s["heading"]))
		level := 1
		if l, ok := isInt(s["level"]); ok {
			level = min(max(l, 1), 3)
		}
		// A section whose items all wait in the appendix has no heading.
		at := len(body)
		for _, iv := range list(s["items"]) {
			i, ok := iv.(M)
			if !ok {
				continue
			}
			text := strings.TrimSpace(strOr(i["text"]))
			if text == "" {
				continue
			}
			cited := refs(OutlineEvidenceIDs(i))
			choice := strOr(i["choice"])
			switch {
			case i["kind"] == "unresolved_mark":
				switch choice {
				case "mastered", "key", "review", "custom":
					body = append(body, OutlineEntry{text, cited, OutlineChoiceLabel(choice, strOr(i["choice_text"])), i})
				default:
					tag := l10n.OutlineTagUnconfirmed
					if choice == "keep" {
						tag = l10n.OutlineChoiceKeep
					}
					marks = append(marks, OutlineEntry{text, cited, tag, i})
				}
			case i["review"] == "to_check":
				checks = append(checks, OutlineEntry{text, cited, l10n.OutlineTagCheck, i})
			default:
				tag := ""
				if i["kind"] == "question" {
					tag = l10n.OutlineTagQuestion
				} else if i["kind"] == "fact" && !OutlineVerified(i, stale) {
					tag = l10n.OutlineTagUnverified
				}
				body = append(body, OutlineEntry{text, cited, tag, i})
			}
		}
		if heading != "" && len(body) > at {
			body = append(body[:at], append([]OutlineBlock{OutlineHeading{level + 1, heading}}, body[at:]...)...)
		}
	}
	var conflicts []OutlineBlock
	for _, cv := range list(proposal["conflicts"]) {
		c, ok := cv.(M)
		if !ok {
			continue
		}
		if t := strings.TrimSpace(strOr(c["text"])); t != "" {
			conflicts = append(conflicts, OutlineEntry{Text: t, Refs: refs(strs(c["evidence_ids"]))})
		}
	}
	plain := func(key string) []OutlineBlock {
		var out []OutlineBlock
		for _, q := range list(proposal[key]) {
			if t := strings.TrimSpace(str(q)); t != "" {
				out = append(out, OutlineEntry{Text: t})
			}
		}
		return out
	}
	groups := []struct {
		title   string
		entries []OutlineBlock
	}{
		{l10n.OutlineConflicts, conflicts}, {l10n.OutlineMarks, marks}, {l10n.OutlineChecks, checks},
		{l10n.OutlineQuestions, plain("unresolved_questions")}, {l10n.OutlineUnread, plain("missing_sources")},
	}
	var appendix []OutlineBlock
	for _, g := range groups {
		if len(g.entries) == 0 {
			continue
		}
		if appendix == nil {
			appendix = []OutlineBlock{OutlineHeading{2, l10n.OutlineAppendix}, OutlineRemark{l10n.OutlineAppendixNote}}
		}
		appendix = append(append(appendix, OutlineHeading{3, g.title}), g.entries...)
	}
	title := strings.TrimSpace(strOr(proposal["title"]))
	if title == "" {
		title = l10n.OutlineUntitled
	}
	return OutlineDoc{Title: title, Body: body, Appendix: appendix, Sources: sources}
}

var blankLines = regexp.MustCompile(`\n{3,}`)
var spaces = regexp.MustCompile(`\s+`)

// OutlineMarkdown is the lecture note body: headings, lists with numbered
// citations, the appendix and a source list whose coursive:// links sit
// beside readable file names, pages and times.
func OutlineMarkdown(doc OutlineDoc) string {
	var out strings.Builder
	out.WriteString("# " + doc.Title + "\n")
	entry := func(e OutlineEntry) {
		lines := strings.Split(e.Text, "\n")
		var refs strings.Builder
		for _, n := range e.Refs {
			fmt.Fprintf(&refs, "[%d]", n)
		}
		tail := strings.TrimLeftFunc(lines[len(lines)-1], unicode.IsSpace)
		// A closing fence or display formula keeps its own line.
		ownLine := strings.HasPrefix(tail, "```") || strings.HasPrefix(tail, "$$")
		head := ""
		if e.Tag != "" {
			head = "**" + e.Tag + "** "
		}
		out.WriteString("- " + head + lines[0])
		for _, l := range lines[1:] {
			out.WriteString("\n  " + l)
		}
		if refs.Len() > 0 {
			if ownLine {
				out.WriteString("\n  " + refs.String())
			} else {
				out.WriteString(" " + refs.String())
			}
		}
		out.WriteString("\n")
	}
	for _, b := range doc.Blocks() {
		switch b := b.(type) {
		case OutlineHeading:
			out.WriteString("\n" + strings.Repeat("#", b.Level) + " " + b.Text + "\n\n")
		case OutlineRemark:
			out.WriteString(b.Text + "\n\n")
		case OutlineEntry:
			entry(b)
		}
	}
	if len(doc.Sources) > 0 {
		out.WriteString("\n## " + l10n.OutlineSources + "\n\n")
		for _, s := range doc.Sources {
			var where []string
			for _, v := range []string{s.Label, s.Locator} {
				if v != "" {
					where = append(where, v)
				}
			}
			fmt.Fprintf(&out, "%d. %s", s.Number, strings.Join(where, " · "))
			if s.Quote != "" {
				out.WriteString(" — “" + shortQuote(s.Quote) + "”")
			}
			if s.Stale {
				out.WriteString("（" + l10n.OutlineSourceStale + "）")
			}
			if s.Link != "" {
				out.WriteString(" · [" + l10n.OutlineOpenSource + "](" + s.Link + ")")
			}
			out.WriteString("\n")
		}
	}
	return blankLines.ReplaceAllString(out.String(), "\n\n")
}

func shortQuote(quote string) string {
	q := spaces.ReplaceAllString(quote, " ")
	if utf8.RuneCountInString(q) > 80 {
		return string([]rune(q)[:80]) + "…"
	}
	return q
}

// RunKind is how a run of an item's text is set: the review panel and the
// PDF read the same runs, so a formula in one is a formula in the other.
type RunKind int

const (
	RunText RunKind = iota
	RunBold
	RunCode
	RunMath
	RunDisplayMath
)

// Run is one run of an item's text.
type Run struct {
	Kind RunKind
	Text string
}

// lazyClose finds the first close at or after from+1 runes past from,
// with no newline before it: the end of a lazy `.+?`.
func lazyClose(r []rune, from int, close string) int {
	c := []rune(close)
	for j := from + 1; j+len(c) <= len(r); j++ {
		if r[j-1] == '\n' {
			return -1
		}
		if string(r[j:j+len(c)]) == close {
			return j
		}
	}
	return -1
}

// OutlineInline is outlineInline: it splits one paragraph of an item into
// plain, bold, code and math runs. A single $ opens only before a
// non-space and closes only after one, and not before a digit, so prices
// such as "$5 and $6" stay text (Pandoc's rule).
func OutlineInline(text string) []Run {
	r := []rune(text)
	var out []Run
	at := 0
	match := func(i int) (RunKind, string, int, bool) {
		has := func(s string) bool { return strings.HasPrefix(string(r[i:min(len(r), i+len(s))]), s) }
		for _, d := range []struct {
			open, close string
			kind        RunKind
		}{{"$$", "$$", RunDisplayMath}, {`\[`, `\]`, RunDisplayMath}, {"**", "**", RunBold}} {
			if has(d.open) {
				if j := lazyClose(r, i+len(d.open), d.close); j >= 0 {
					return d.kind, string(r[i+len(d.open) : j]), j + len(d.close), true
				}
			}
		}
		if r[i] == '`' {
			j := i + 1
			for j < len(r) && r[j] != '`' && r[j] != '\n' {
				j++
			}
			if j < len(r) && r[j] == '`' && j > i+1 {
				return RunCode, string(r[i+1 : j]), j + 1, true
			}
		}
		if r[i] == '$' && i+1 < len(r) && !unicode.IsSpace(r[i+1]) {
			j := i + 1
			for j < len(r) && r[j] != '$' && r[j] != '\n' {
				j++
			}
			if j < len(r) && r[j] == '$' && j > i+1 && !unicode.IsSpace(r[j-1]) && !(j+1 < len(r) && unicode.IsDigit(r[j+1])) {
				return RunMath, string(r[i+1 : j]), j + 1, true
			}
		}
		if has(`\(`) {
			if j := lazyClose(r, i+2, `\)`); j >= 0 {
				return RunMath, string(r[i+2 : j]), j + 2, true
			}
		}
		return 0, "", 0, false
	}
	for i := 0; i < len(r); {
		kind, body, end, ok := match(i)
		if !ok {
			i++
			continue
		}
		if i > at {
			out = append(out, Run{RunText, string(r[at:i])})
		}
		if kind == RunMath || kind == RunDisplayMath {
			body = strings.TrimSpace(body)
		}
		out = append(out, Run{kind, body})
		at, i = end, end
	}
	if at < len(r) {
		out = append(out, Run{RunText, string(r[at:])})
	}
	return out
}
