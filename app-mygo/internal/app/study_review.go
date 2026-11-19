package app

import (
	"encoding/json"
	"errors"
	"math"
	"os"
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
	"github.com/walkerkiller/coursive/app-mygo/internal/study"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// study_outline_review.dart: the review of an outline proposal. The
// learner accepts, rewrites, reorders, splits, merges, flags or deletes
// each item and gives each handwritten mark its meaning. Every change stays
// in the conversation's draft; only 「保存为课程笔记」 writes a note.

// outlineReview is _OutlineReviewState, kept for the review open.
type outlineReview struct {
	target  reviewTarget
	undo    []string
	editing string
	// edits are the text of the fields by id: items, headings, the title
	// and marks' custom meanings.
	edits            map[string]*string
	snapped          map[string]bool
	saving, conflict bool
	err, notice      string
	persist          *studyTimer
	list             ui.ListState
	focus, hover     string
	// pendingFocus is a row to select, giving the panel the keys.
	pendingFocus string
	wantKeys     bool
	editFocused  bool
}

type reviewParts struct{ outline *outlineReview }

// reviewConversation is the conversation of the review open.
func (s *StudyCanvas) reviewConversation() *studyConversation {
	if s.review == nil {
		return nil
	}
	return s.conversations[s.review.card]
}

// reviewTurn is the turn under review, while it still holds a proposal.
func (s *StudyCanvas) reviewTurn() core.J {
	cv := s.reviewConversation()
	if cv == nil || cv.data == nil {
		return nil
	}
	for _, t := range cv.turns() {
		if t.S("id") == s.review.turn && reviewProposal(t) != nil {
			return t
		}
	}
	return nil
}

// reviewProposal is the turn's proposal as plain maps, held in the turn so
// that edits to it are the draft's.
func reviewProposal(turn core.J) study.M {
	res := core.Obj(turn["result"])
	if res == nil {
		return nil
	}
	if j, ok := res["outline_proposal"].(core.J); ok {
		res["outline_proposal"] = plainMap(j)
	}
	p, _ := res["outline_proposal"].(map[string]any)
	if p == nil || study.OutlineProposal(study.M{"result": map[string]any{"outline_proposal": p}}) == nil {
		return nil
	}
	return p
}

// openReview opens the review beside the canvas. A window too narrow for
// both panels gives the review the side the conversation had.
func (s *StudyCanvas) openReview(cardID, turnID string) {
	if card := s.doc.Card(cardID); card != nil {
		s.mountConversation(card)
	}
	s.review = &reviewTarget{card: cardID, turn: turnID}
	s.reviewPeek = false
	if s.viewport.X < 1180 {
		s.stageOpen = false
	}
}

func (s *StudyCanvas) closeReview() {
	if r := s.outline; r != nil && r.persist != nil {
		r.persist.stop()
		r.persist = nil
		if cv := s.reviewConversation(); cv != nil {
			go cv.save()
		}
	}
	s.review, s.outline = nil, nil
	s.previewEvidence(nil)
}

func (s *StudyCanvas) reviewNarrow() bool { return s.viewport.X < 720 }

// reviewRect is where the open review sits, in canvas pixels; nil when
// closed.
func (s *StudyCanvas) reviewRect() *study.Rect {
	if s.review == nil || s.reviewTurn() == nil {
		return nil
	}
	vp := s.viewport
	top := math.Max(76, s.topBarsBottom()+sp(design.SpaceSM))
	if s.reviewNarrow() {
		h := vp.Y * .62
		if s.reviewPeek {
			h = 76
		}
		r := study.RectLTWH(sp(design.SpaceSM), vp.Y-h-sp(design.SpaceSM), vp.X-2*sp(design.SpaceSM), h)
		return &r
	}
	w := math.Min(400, vp.X*.42)
	left := sp(design.SpaceSM)
	if s.aiPlacement.edge == dockLeft {
		left = vp.X - w - sp(design.SpaceSM)
	}
	r := study.RectLTWH(left, top, w, vp.Y-top-sp(design.SpaceHuge))
	return &r
}

func evidenceList(evidence []core.J) []any {
	out := make([]any, len(evidence))
	for i, e := range evidence {
		out[i] = plainJSON(e)
	}
	return out
}

func (s *StudyCanvas) outlineExport(turn core.J) study.OutlineDoc {
	evidence := studyTurnEvidence(turn)
	return study.ExportOutline(reviewProposal(turn), evidenceList(evidence), s.staleEvidence(evidence), s.lectureID())
}

// markInk is the selected ink a mark points at, still on a visible layer.
func (s *StudyCanvas) markInk(item study.M) []*study.Stroke {
	ids := map[string]bool{}
	for _, id := range core.List(core.Obj(item["anchor"])["stroke_ids"]) {
		ids[core.Str(id)] = true
	}
	d := s.doc
	if len(ids) == 0 || d == nil {
		return nil
	}
	var out []*study.Stroke
	for _, st := range d.Strokes {
		if ids[st.ID] && d.Visible(st.Layer) {
			out = append(out, st)
		}
	}
	return out
}

type reviewRow struct {
	section, item study.M
}

func (r reviewRow) id() string {
	if r.item != nil {
		return core.Str(r.item["id"])
	}
	return core.Str(r.section["id"])
}

func sectionsOf(p study.M) []study.M {
	var out []study.M
	for _, v := range core.List(p["sections"]) {
		if m, ok := v.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func itemsOf(section study.M) []any { return core.List(section["items"]) }

func reviewRows(p study.M) []reviewRow {
	var rows []reviewRow
	for _, s := range sectionsOf(p) {
		rows = append(rows, reviewRow{section: s})
		for _, v := range itemsOf(s) {
			if m, ok := v.(map[string]any); ok {
				rows = append(rows, reviewRow{section: s, item: m})
			}
		}
	}
	return rows
}

// reviewCtx is a review open on a turn: what its operations work on.
type reviewCtx struct {
	s    *StudyCanvas
	r    *outlineReview
	cv   *studyConversation
	turn core.J
}

func (x reviewCtx) proposal() study.M { return reviewProposal(x.turn) }

// change records an undo step, applies change and keeps the draft.
func (x reviewCtx) change(change func(), snapshot bool) {
	r := x.r
	if snapshot {
		x.snap()
	}
	change()
	r.notice = ""
	if r.persist != nil {
		r.persist.stop()
	}
	cv := x.cv
	r.persist = studyAfter(500*time.Millisecond, func() {
		r.persist = nil
		go cv.save()
	})
}

func (x reviewCtx) snap() {
	b, _ := json.Marshal(x.proposal())
	x.r.undo = append(x.r.undo, string(b))
	if len(x.r.undo) > 60 {
		x.r.undo = x.r.undo[1:]
	}
}

func (x reviewCtx) undoLast() {
	r := x.r
	if len(r.undo) == 0 {
		return
	}
	var previous map[string]any
	if json.Unmarshal([]byte(r.undo[len(r.undo)-1]), &previous) != nil {
		return
	}
	r.undo = r.undo[:len(r.undo)-1]
	r.editing = ""
	clear(r.edits)
	x.change(func() { core.Obj(x.turn["result"])["outline_proposal"] = previous }, false)
}

// reorder moves the row from to the index to it has after the move: a flat
// list where a heading starts a section, so moving rows regroups them.
func (x reviewCtx) reorder(from, to int) {
	rows := reviewRows(x.proposal())
	if from < 0 || from >= len(rows) || to < 0 || to >= len(rows) {
		return
	}
	moved := rows[from]
	rows = append(rows[:from], rows[from+1:]...)
	rows = append(rows[:to], append([]reviewRow{moved}, rows[to:]...)...)
	if rows[0].item != nil {
		return
	}
	var next []any
	var last map[string]any
	for _, row := range rows {
		if row.item == nil {
			last = map[string]any{}
			for k, v := range row.section {
				last[k] = v
			}
			last["items"] = []any{}
			next = append(next, last)
		} else {
			last["items"] = append(last["items"].([]any), row.item)
		}
	}
	for _, sct := range next {
		if len(sct.(map[string]any)["items"].([]any)) > 20 {
			x.r.notice = l10n.OutlineTooManyItems
			return
		}
	}
	x.change(func() { x.proposal()["sections"] = next }, true)
}

func (x reviewCtx) rowIndex(id string) int {
	for i, row := range reviewRows(x.proposal()) {
		if row.id() == id {
			return i
		}
	}
	return -1
}

func (x reviewCtx) move(item study.M, by int) {
	rows := reviewRows(x.proposal())
	at := x.rowIndex(core.Str(item["id"]))
	to := at + by
	// Passing a heading moves the item into the neighbouring section.
	if at < 0 || to < 1 || to >= len(rows) {
		return
	}
	x.reorder(at, to)
	x.r.pendingFocus = core.Str(item["id"])
}

// locate is the section holding item and its place there.
func (x reviewCtx) locate(item study.M) (study.M, int) {
	id := core.Str(item["id"])
	for _, s := range sectionsOf(x.proposal()) {
		for i, v := range itemsOf(s) {
			if m, ok := v.(map[string]any); ok && core.Str(m["id"]) == id {
				return s, i
			}
		}
	}
	return nil, -1
}

func (x reviewCtx) freshID() string {
	used := map[string]bool{}
	for _, row := range reviewRows(x.proposal()) {
		used[row.id()] = true
	}
	n := len(used) + 1
	for used["i"+strconv.Itoa(n)] {
		n++
	}
	return "i" + strconv.Itoa(n)
}

func (x reviewCtx) setReview(item study.M, review string) {
	x.change(func() { item["review"] = review }, true)
}

func (x reviewCtx) toggleConfirm(item study.M) {
	if item["kind"] == "unresolved_mark" {
		return
	}
	if item["review"] == "confirmed" {
		x.setReview(item, "pending")
	} else {
		x.setReview(item, "confirmed")
	}
}

func (x reviewCtx) toggleCheck(item study.M) {
	if item["kind"] == "unresolved_mark" {
		return
	}
	if item["review"] == "to_check" {
		x.setReview(item, "pending")
	} else {
		x.setReview(item, "to_check")
	}
}

func (x reviewCtx) remove(item study.M) {
	s, at := x.locate(item)
	if s == nil {
		return
	}
	x.change(func() {
		items := itemsOf(s)
		s["items"] = append(append([]any{}, items[:at]...), items[at+1:]...)
	}, true)
}

var sentenceEnd = regexp.MustCompile(`[。！？；.!?;]\s*`)

// split splits after the first sentence, or in the middle.
func (x reviewCtx) split(item study.M) {
	s, at := x.locate(item)
	if s == nil || len(itemsOf(s)) >= 20 {
		return
	}
	text := core.Str(item["text"])
	if x.r.editing == core.Str(item["id"]) {
		if e := x.r.edits[x.r.editing]; e != nil {
			text = *e
		}
	}
	cut := len(text) / 2
	if m := sentenceEnd.FindStringIndex(text); m != nil && m[1] < len(text) {
		cut = m[1]
	}
	for cut < len(text) && !utf8Start(text[cut]) {
		cut++
	}
	head, tail := strings.TrimSpace(text[:cut]), strings.TrimSpace(text[cut:])
	if head == "" || tail == "" {
		return
	}
	x.r.editing = ""
	x.change(func() {
		item["text"] = head
		copyItem := map[string]any{}
		for k, v := range item {
			copyItem[k] = v
		}
		copyItem["id"], copyItem["text"] = x.freshID(), tail
		ids := []any{}
		for _, id := range study.OutlineEvidenceIDs(item) {
			ids = append(ids, id)
		}
		copyItem["evidence_ids"] = ids
		items := itemsOf(s)
		s["items"] = append(append(append([]any{}, items[:at+1]...), copyItem), items[at+1:]...)
	}, true)
}

func utf8Start(b byte) bool { return b&0xC0 != 0x80 }

func (x reviewCtx) mergeNext(item study.M) {
	s, at := x.locate(item)
	if s == nil || at+1 >= len(itemsOf(s)) {
		return
	}
	next, _ := itemsOf(s)[at+1].(map[string]any)
	if next == nil || next["kind"] == "unresolved_mark" || item["kind"] == "unresolved_mark" {
		return
	}
	x.change(func() {
		item["text"] = core.Str(item["text"]) + "\n" + core.Str(next["text"])
		seen := map[string]bool{}
		ids := []any{}
		for _, id := range append(study.OutlineEvidenceIDs(item), study.OutlineEvidenceIDs(next)...) {
			if !seen[id] && len(ids) < 24 {
				seen[id] = true
				ids = append(ids, id)
			}
		}
		item["evidence_ids"] = ids
		items := itemsOf(s)
		s["items"] = append(append([]any{}, items[:at+1]...), items[at+2:]...)
	}, true)
}

func (x reviewCtx) addItem(section study.M) {
	if len(itemsOf(section)) >= 20 {
		return
	}
	item := map[string]any{"id": x.freshID(), "kind": "summary", "text": "", "evidence_ids": []any{}, "review": "confirmed"}
	x.change(func() { section["items"] = append(itemsOf(section), item) }, true)
	x.edit(item)
}

func (x reviewCtx) deleteSection(section study.M) {
	p := x.proposal()
	sections := core.List(p["sections"])
	i := -1
	for k, v := range sections {
		if m, ok := v.(map[string]any); ok && core.Str(m["id"]) == core.Str(section["id"]) {
			i = k
		}
	}
	if i < 0 || len(sections) == 1 {
		return
	}
	x.change(func() {
		// Items stay: they join the section above.
		items := itemsOf(section)
		rest := append(append([]any{}, sections[:i]...), sections[i+1:]...)
		target := rest[max(0, i-1)].(map[string]any)
		room := 20 - len(itemsOf(target))
		target["items"] = append(itemsOf(target), items[:max(0, min(room, len(items)))]...)
		p["sections"] = rest
	}, true)
}

// choose gives a handwritten mark its meaning; only the learner does.
func (x reviewCtx) choose(item study.M, choice, text string) {
	x.change(func() {
		if choice == "" {
			delete(item, "choice")
			delete(item, "choice_text")
			item["review"] = "needs_interpretation"
			return
		}
		item["choice"], item["review"] = choice, "confirmed"
		if choice == "custom" {
			item["choice_text"] = text
		} else {
			delete(item, "choice_text")
		}
	}, true)
}

func (x reviewCtx) edit(item study.M) {
	id := core.Str(item["id"])
	text := core.Str(item["text"])
	x.r.edits[id] = &text
	x.snap()
	x.r.editing, x.r.editFocused = id, false
}

func (x reviewCtx) commit(item study.M) {
	id := core.Str(item["id"])
	text := ""
	if e := x.r.edits[id]; e != nil {
		text = strings.TrimSpace(*e)
	}
	x.r.editing = ""
	switch {
	case text == "":
		x.remove(item)
	case text != item["text"]:
		if r := []rune(text); len(r) > 1300 {
			text = string(r[:1300])
		}
		x.change(func() { item["text"] = text }, false)
	default:
		if len(x.r.undo) > 0 {
			x.r.undo = x.r.undo[:len(x.r.undo)-1]
		}
	}
	x.r.pendingFocus = id
}

func (x reviewCtx) cancelEdit(item study.M) {
	x.r.editing = ""
	if len(x.r.undo) > 0 {
		x.r.undo = x.r.undo[:len(x.r.undo)-1]
	}
	x.r.pendingFocus = core.Str(item["id"])
}

// jump carries the list to the next item that still needs attention.
func (x reviewCtx) jump(needs func(study.M) bool) {
	rows := reviewRows(x.proposal())
	var hits []int
	focused := -1
	for i, row := range rows {
		if row.item != nil && needs(row.item) {
			if row.id() == x.r.focus {
				focused = len(hits)
			}
			hits = append(hits, i)
		}
	}
	if len(hits) == 0 {
		return
	}
	target := hits[(focused+1)%len(hits)]
	x.r.list.ScrollTo(target, ui.Center)
	x.r.pendingFocus = rows[target].id()
}

func (x reviewCtx) markdown() string { return study.OutlineMarkdown(x.s.outlineExport(x.turn)) }

func (x reviewCtx) dirty() bool {
	p := x.proposal()
	return p["state"] == "accepted" && study.OutlineNoteHash(x.markdown()) != core.Str(p["note_hash"])
}

// save saves the reviewed outline as this lecture's note under its stable
// ID. A retry after a lost response finds its own note; a note changed
// elsewhere is never overwritten.
func (x reviewCtx) save(copy bool) {
	r := x.r
	if r.saving {
		return
	}
	if r.editing != "" {
		for _, row := range reviewRows(x.proposal()) {
			if row.item != nil && row.id() == r.editing {
				x.commit(row.item)
				break
			}
		}
	}
	p, md := x.proposal(), x.markdown()
	r.saving, r.err, r.conflict = true, "", false
	s, cv := x.s, x.cv
	course := ""
	if s.ctl.Lecture != nil {
		course = s.ctl.Lecture.CourseID()
	}
	lecture, title := s.lectureID(), core.Str(p["title"])
	id := core.Str(p["note_id"])
	if id == "" {
		id = study.OutlineNoteID(p)
	}
	if copy {
		id = study.OutlineNoteID(p) + "-" + strconv.FormatInt(time.Now().UnixMilli(), 36)
	}
	state, hash := core.Str(p["state"]), core.Str(p["note_hash"])
	api := State.Api()
	go func() {
		fail := func(err error) {
			onUI(func() {
				r.saving = false
				var ae *core.ApiError
				if errors.As(err, &ae) && ae.Status == 409 {
					r.conflict, r.err = true, l10n.OutlineSaveConflict
				} else {
					r.err = l10n.OutlineSaveFailed(err.Error())
				}
			})
		}
		var base *string
		if !copy {
			current, err := api.Get("/api/notes/" + core.Esc(id))
			var ae *core.ApiError
			if err != nil && !(errors.As(err, &ae) && ae.Status == 404) {
				fail(err)
				return
			}
			if err == nil {
				body := core.Obj(current).S("body")
				if body != md && (state != "accepted" || study.OutlineNoteHash(body) != hash) {
					onUI(func() { r.saving, r.conflict, r.err = false, true, l10n.OutlineSaveConflict })
					return
				}
				base = &body
			}
		}
		if base == nil || *base != md {
			body := map[string]any{"id": id, "course_id": course, "lecture_id": lecture, "title": title, "body": md, "source": "user"}
			if base != nil {
				body["base_body"] = *base
			}
			if _, err := api.Put("/api/notes", body); err != nil {
				fail(err)
				return
			}
		}
		onUI(func() {
			x.change(func() { p["state"], p["note_id"], p["note_hash"] = "accepted", id, study.OutlineNoteHash(md) }, false)
			if r.persist != nil {
				r.persist.stop()
				r.persist = nil
			}
		})
		ok := cv.save()
		onUI(func() {
			r.saving = false
			if !ok {
				r.err = l10n.OutlineStateNotRecorded
				return
			}
			r.notice = l10n.OutlineSaved
		})
	}()
}

func (x reviewCtx) exportMarkdown() {
	title := strings.TrimSpace(core.Str(x.proposal()["title"]))
	if title == "" {
		title = l10n.OutlineUntitled
	}
	md, r := x.markdown(), x.r
	widgets.PickSavePath(title+".md", core.FileFilter{Name: "Markdown", Extensions: []string{"md"}}, "", func(path string) {
		if path == "" {
			return
		}
		if err := os.WriteFile(path, []byte(md), 0o644); err != nil {
			r.err = l10n.OutlineExportFailed(err.Error())
			return
		}
		r.notice = l10n.OutlineExportSaved(path)
	})
}

func outlineAmber() ui.Color {
	if design.T().Dark {
		return design.ARGB(0xffe0a85a)
	}
	return design.ARGB(0xff9a5b08)
}

func (s *StudyCanvas) reviewPanel(c *ui.Context) {
	rect, cv, turn := s.reviewRect(), s.reviewConversation(), s.reviewTurn()
	if rect == nil || cv == nil || turn == nil {
		return
	}
	if s.outline == nil || s.outline.target != *s.review {
		s.outline = &outlineReview{target: *s.review, edits: map[string]*string{}, snapped: map[string]bool{}}
	}
	x := reviewCtx{s: s, r: s.outline, cv: cv, turn: turn}
	r := x.r
	narrow := s.reviewNarrow()
	peek := narrow && s.reviewPeek
	from := float32(-6)
	if !narrow && s.aiPlacement.edge == dockLeft {
		from = 6
	}
	panel := ui.Column(c.Key("study-outline-review")).Absolute().Left(f32(rect.L)).Top(f32(rect.T)).Size(f32(rect.W()), f32(rect.H())).
		Padding(design.SpaceMD, design.SpaceSM, design.SpaceMD, design.SpaceLG).AlignItems(ui.Stretch).
		Transition(ui.ElementTransition{Duration: design.Medium2, Ease: design.EmphasizedDecelerate, ExitDuration: design.Short4,
			ExitEase: design.EmphasizedAccelerate, Enter: &ui.Motion{Opacity: 0, XPercent: from}, Exit: &ui.Motion{Opacity: 0, XPercent: from}, Shadows: true})
	widgets.Glass{Radius: design.CornerXL, Depth: design.GlassPanel}.Apply(panel)
	s.recordControl(panel)
	panel = panel.Focusable()
	if r.editing == "" && panel.Shortcut(ui.Cmd, ui.KeyZ) {
		x.undoLast()
	}
	if r.editing == "" && panel.Shortcut(0, ui.KeyEscape) {
		s.closeReview()
		return
	}
	// The row selected takes the keys while the panel itself has the
	// focus, not a field in it.
	if r.editing == "" && r.focus != "" && panel.Focused() {
		for _, row := range reviewRows(x.proposal()) {
			if row.item != nil && row.id() == r.focus {
				x.rowKeys(panel, row.item)
				break
			}
		}
	}
	p := x.proposal()
	evidence := studyTurnEvidence(turn)
	stale := s.staleEvidence(evidence)
	stats := study.CountOutline(p, stale)
	rows := reviewRows(p)
	numbers := map[string]int{}
	for _, row := range rows {
		if row.item != nil {
			for _, id := range study.OutlineEvidenceIDs(row.item) {
				if _, ok := numbers[id]; !ok {
					numbers[id] = len(numbers) + 1
				}
			}
		}
	}
	byID := map[string]core.J{}
	for _, e := range evidence {
		byID[e.S("id")] = e
	}
	cs, tt := design.S(), design.TT()
	panel.Children(func() {
		x.header(c, stats, stale, narrow)
		if peek {
			return
		}
		ui.Box(c).Height(design.SpaceSM)
		ui.Box(c).FillWidth().Grow(1).Basis(0).Clip().Children(func() {
			if len(rows) == 0 {
				ui.Box(c).Fill().Center().Children(func() {
					m3.Text(c, l10n.OutlineEmpty, tt.BodyMedium.C(cs.OnSurfaceVariant))
				})
				return
			}
			r.list.Key = func(i int) any {
				if i < len(rows) {
					return rows[i].id()
				}
				return "outline-appendix"
			}
			r.list.Reorder = func(moved []int, to int) {
				if len(moved) != 1 || moved[0] >= len(rows) || to > len(rows) {
					return
				}
				from := moved[0]
				if to > from {
					to--
				}
				x.reorder(from, to)
			}
			hovered := ""
			ui.List(c, &r.list, len(rows)+1, func(i int) {
				if i == len(rows) {
					x.appendix(c, byID, numbers)
					return
				}
				row := rows[i]
				if row.item == nil {
					x.sectionRow(c, row.section, i)
					return
				}
				if x.itemRow(c, row.item, byID, numbers, stale) {
					hovered = row.id()
				}
			}).Fill().Padding(0, design.SpaceSM, design.SpaceMD, 0)
			// The hovered row previews its first live citation, or a mark's ink.
			if hovered != r.hover {
				r.hover = hovered
				var preview core.J
				for _, row := range rows {
					if row.item == nil || row.id() != hovered {
						continue
					}
					for _, id := range study.OutlineEvidenceIDs(row.item) {
						if e, ok := byID[id]; ok && !stale[id] {
							preview = e
							break
						}
					}
					if a, ok := row.item["anchor"].(map[string]any); ok && preview == nil {
						preview = core.J{"id": hovered, "kind": "visual_observation", "status": "verified", "anchor": a}
					}
				}
				s.previewEvidence(preview)
			}
		})
		x.footer(c, narrow)
	})
	if r.wantKeys {
		r.wantKeys = false
		panel.Focus()
	}
}

func (x reviewCtx) header(c *ui.Context, stats study.OutlineStats, stale map[string]bool, narrow bool) {
	s, r, p := x.s, x.r, x.proposal()
	cs, tt := design.S(), design.TT()
	title := r.edits["title"]
	if title == nil {
		t := core.Str(p["title"])
		title = &t
		r.edits["title"] = title
	}
	ui.Column(c).FillWidth().AlignItems(ui.Stretch).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			st := tt.TitleMedium.W(600)
			ui.Box(c).Grow(1).Basis(0).Children(func() {
				in := m3.Field{Borderless: true, MaxLength: 120, Pad: true, Style: &st}.Show(c.Key("outline-title"), title)
				x.snapOnce(in, "title")
				if in.Changed() {
					v := strings.TrimSpace(*title)
					if v == "" {
						v = l10n.OutlineUntitled
					}
					x.change(func() { p["title"] = v }, false)
				}
			})
			if narrow {
				peek := s.reviewPeek
				if m3.Segmented(c, []m3.Segment[bool]{{Value: false, Label: l10n.OutlinePeekOutline}, {Value: true, Label: l10n.OutlinePeekSource}},
					&peek, m3.SegOpts{NoCheck: true, Compact: true}) {
					s.reviewPeek = peek
				}
			}
			if (m3.IconButton{Icon: icons.UndoRounded, Size: 20, Tooltip: l10n.OutlineUndo, Disabled: len(r.undo) == 0}).Show(c.Key("outline-undo")).Clicked() {
				x.undoLast()
			}
			if (m3.IconButton{Icon: icons.CloseRounded, Size: 20, Tooltip: l10n.OutlineClose}).Show(c.Key("outline-close")).Clicked() {
				core.Post(s.closeReview)
			}
		})
		if narrow && s.reviewPeek {
			return
		}
		ui.Box(c).Height(design.SpaceXS)
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			m3.Text(c, l10n.OutlineReviewed(stats.Reviewed, stats.Total), tt.LabelMedium.C(cs.OnSurfaceVariant))
			ui.Box(c).Width(design.SpaceSM)
			bar := ui.Box(c).Grow(1).Basis(0)
			v := 0.0
			if stats.Total > 0 {
				v = float64(stats.Reviewed) / float64(stats.Total)
			}
			shown := float64(bar.Animate("progress", float32(v), design.Medium2))
			bar.Children(func() {
				m3.Linear(c, m3.Progress{Value: m3.Of(shown), Height: 3, Track: cs.SurfaceContainerHighest})
			})
		})
		if stats.Stale+stats.Marks+stats.Unverified == 0 {
			return
		}
		ui.Box(c).Height(design.SpaceSM)
		ui.Row(c).FillWidth().Wrap().Gap(design.SpaceXS).Children(func() {
			jump := func(key, label string, col ui.Color, needs func(study.M) bool) {
				if (m3.Chip{Kind: m3.ActionChip, Label: label, Compact: true, FG: col, Side: col.Alpha(.35)}).Show(c.Key(key)).Clicked() {
					x.jump(needs)
				}
			}
			if stats.Marks > 0 {
				jump("marks", l10n.OutlineMarksCount(stats.Marks), cs.Tertiary, func(i study.M) bool { return i["review"] == "needs_interpretation" })
			}
			if stats.Stale > 0 {
				jump("stale", l10n.OutlineStaleCount(stats.Stale), cs.Error, func(i study.M) bool {
					for _, id := range study.OutlineEvidenceIDs(i) {
						if stale[id] {
							return true
						}
					}
					return false
				})
			}
			if stats.Unverified > 0 {
				jump("unverified", l10n.OutlineUnverifiedCount(stats.Unverified), outlineAmber(), func(i study.M) bool {
					return i["kind"] == "fact" && !study.OutlineVerified(i, stale)
				})
			}
		})
	})
}

// snapOnce records an undo step the first time a field changes after it
// takes the focus, as the Flutter fields do on their tap.
func (x reviewCtx) snapOnce(in ui.Element, id string) {
	if !in.FocusWithin() {
		delete(x.r.snapped, id)
		return
	}
	if in.Changed() && !x.r.snapped[id] {
		x.r.snapped[id] = true
		x.snap()
	}
}

func (x reviewCtx) sectionRow(c *ui.Context, section study.M, index int) {
	cs, tt := design.S(), design.TT()
	id := core.Str(section["id"])
	level := int(core.Float(section["level"]))
	if level < 1 || level > 3 {
		level = 1
	}
	heading := x.r.edits[id]
	if heading == nil {
		h := core.Str(section["heading"])
		heading = &h
		x.r.edits[id] = heading
	}
	top := design.SpaceMD
	if index == 0 {
		top = 0
	}
	ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(top, 0, design.SpaceXS, 0).Children(func() {
		m3.Icon(c, icons.DragIndicatorRounded, 18, cs.Outline).Cursor(ui.CursorGrab)
		ui.Box(c).Width(design.SpaceXS)
		lv := ui.ButtonBase(c.Key("level")).Radius(design.CornerXS).Padding(design.SpaceXXS, design.SpaceXS).Tooltip(l10n.OutlineLevel).Cursor(ui.CursorPointer)
		m3.Ink(lv, cs.OnSurface, design.CornerXS)
		lv.Children(func() { m3.Text(c, "H"+strconv.Itoa(level), tt.LabelSmall.C(cs.Primary)).FontFeatures("tnum") })
		if lv.Clicked() {
			x.change(func() { section["level"] = float64(level%3 + 1) }, true)
		}
		ui.Box(c).Width(design.SpaceSM*float32(level-1) + design.SpaceXS)
		st := tt.TitleSmall.W(600)
		if level > 1 {
			st = tt.LabelLarge.W(600)
		}
		if level == 3 {
			st = st.C(cs.OnSurfaceVariant)
		}
		ui.Box(c).Grow(1).Basis(0).Children(func() {
			in := m3.Field{Borderless: true, Pad: true, Style: &st}.Show(c.Key("outline-heading-"+id), heading)
			x.snapOnce(in, id)
			if in.Changed() {
				v := *heading
				x.change(func() { section["heading"] = v }, false)
			}
		})
		sections := len(sectionsOf(x.proposal()))
		m3.MenuButton(c, "outline-section-menu-"+id, icons.MoreHorizRounded, l10n.OutlineMore, m3.BelowEnd, func() []m3.MenuItem {
			return []m3.MenuItem{
				{Icon: icons.AddRounded, Label: l10n.OutlineAddItem, OnTap: func() { x.addItem(section) }},
				{Icon: icons.DeleteOutlineRounded, Label: l10n.OutlineDeleteSection, Disabled: sections <= 1, OnTap: func() { x.deleteSection(section) }},
			}
		})
	})
}

// itemRow builds an item and reports whether the pointer is over it.
func (x reviewCtx) itemRow(c *ui.Context, item study.M, byID map[string]core.J, numbers map[string]int, stale map[string]bool) bool {
	s, r := x.s, x.r
	cs, tt := design.S(), design.TT()
	id := core.Str(item["id"])
	review := core.Str(item["review"])
	mark := item["kind"] == "unresolved_mark"
	var ids, broken []string
	for _, e := range study.OutlineEvidenceIDs(item) {
		if _, ok := byID[e]; ok {
			ids = append(ids, e)
			if stale[e] {
				broken = append(broken, e)
			}
		}
	}
	unverified := item["kind"] == "fact" && !study.OutlineVerified(item, stale)
	editing := r.editing == id
	dim := review != "confirmed" && !mark
	type tag struct {
		label string
		col   ui.Color
	}
	var tags []tag
	switch item["kind"] {
	case "question":
		tags = append(tags, tag{l10n.OutlineTagQuestion, cs.Primary})
	case "summary":
		tags = append(tags, tag{l10n.OutlineKindSummary, cs.OnSurfaceVariant})
	}
	if review == "to_check" {
		tags = append(tags, tag{l10n.OutlineTagCheck, outlineAmber()})
	}
	if unverified && len(broken) == 0 {
		tags = append(tags, tag{l10n.OutlineTagUnverified, outlineAmber()})
	}
	if len(broken) > 0 {
		tags = append(tags, tag{l10n.OutlineEvidenceGone, cs.Error})
	}
	row := ui.Row(c.Key("outline-row-"+id)).FillWidth().AlignItems(ui.Start).Radius(design.CornerMD).
		Padding(design.SpaceSM, 0, design.SpaceSM, design.SpaceXS).Margin(0, 0, design.SpaceXXS, 0)
	if r.pendingFocus == id {
		r.pendingFocus, r.focus, r.wantKeys = "", id, true
	}
	focused := r.focus == id
	bg := cs.Primary.Alpha(0)
	if focused {
		bg = cs.Primary.Alpha(.06)
	}
	row.Background(row.AnimateColor("bg", bg, ui.Animation{Duration: design.Short4}))
	if review == "to_check" {
		row.BorderWidth(0, 0, 0, 2).BorderColor(outlineAmber())
	}
	row.Children(func() {
		if mark {
			x.inkThumb(c, item)
		} else {
			x.reviewTick(c, review, item)
		}
		ui.Box(c).Width(design.SpaceSM)
		ui.Column(c).Grow(1).Basis(0).AlignItems(ui.Stretch).Children(func() {
			if editing {
				x.itemEditor(c, item)
			} else {
				col := cs.OnSurface
				if dim {
					col = cs.OnSurface.Alpha(.78)
				}
				st := tt.BodyMedium.C(col).H(1.45)
				text := strings.ReplaceAll(core.Str(item["text"]), "\n", "  \n")
				for _, e := range ids {
					text += " [" + strconv.Itoa(numbers[e]) + "](coursive-evidence:" + e + ")"
				}
				body := ui.ButtonBase(c.Key("text")).Column().AlignItems(ui.Stretch).FillWidth().FocusRing(false).Disabled(mark).DimDisabled(false)
				if !mark {
					body = body.Cursor(ui.CursorText)
				}
				body.Children(func() {
					md := CoursiveMarkdown{Data: text, Style: &st, LectureID: s.lectureID(), ShareSelection: true,
						OnEvidence: func(e string) {
							if ev, ok := byID[e]; ok {
								s.followEvidence(ev)
							}
						},
						Stale: func(e string) bool { return stale[e] }}.Show(c)
					if len(broken) > 0 {
						md.Opacity(.7)
					}
				})
				if !mark && body.Clicked() {
					x.edit(item)
				}
			}
			if len(tags) > 0 && !editing {
				ui.Box(c).Height(design.SpaceXS)
				ui.Row(c).FillWidth().Wrap().GapX(design.SpaceSM).GapY(design.SpaceXXS).AlignItems(ui.Center).Children(func() {
					for _, t := range tags {
						m3.Text(c, t.label, tt.LabelSmall.C(t.col).W(600).L(.2))
					}
					if len(broken) > 0 {
						if tinyAction(c, "keep", l10n.OutlineKeepAsCheck) {
							x.setReview(item, "to_check")
						}
						if tinyAction(c, "drop", l10n.OutlineDropEvidence) {
							keep := []any{}
							for _, e := range ids {
								if !stale[e] {
									keep = append(keep, e)
								}
							}
							x.change(func() { item["evidence_ids"] = keep }, true)
						}
					}
				})
			}
			if mark {
				x.markChoices(c, item)
			}
		})
		ui.Column(c).AlignItems(ui.Center).Children(func() {
			m3.MenuButton(c, "outline-menu-"+id, icons.MoreHorizRounded, l10n.OutlineMore, m3.BelowEnd, func() []m3.MenuItem {
				var items []m3.MenuItem
				if !mark {
					accept, check := l10n.OutlineAccept, l10n.OutlineToCheck
					ic := icons.DoneRounded
					if review == "confirmed" {
						accept, ic = l10n.OutlineUnaccept, icons.RemoveDoneRounded
					}
					if review == "to_check" {
						check = l10n.OutlineUncheck
					}
					items = append(items,
						m3.MenuItem{Icon: ic, Label: accept, Trailing: "A", OnTap: func() { x.toggleConfirm(item) }},
						m3.MenuItem{Icon: icons.FlagOutlined, Label: check, Trailing: "F", OnTap: func() { x.toggleCheck(item) }},
						m3.MenuItem{Icon: icons.EditOutlined, Label: l10n.OutlineEdit, Trailing: "Enter", OnTap: func() { x.edit(item) }},
						m3.MenuItem{Icon: icons.CallSplitRounded, Label: l10n.OutlineSplit, OnTap: func() { x.split(item) }},
						m3.MenuItem{Icon: icons.MergeRounded, Label: l10n.OutlineMerge, OnTap: func() { x.mergeNext(item) }},
					)
				}
				return append(items,
					m3.MenuItem{Icon: icons.ArrowUpwardRounded, Label: l10n.OutlineMoveUp, Trailing: "Alt+↑", OnTap: func() { x.move(item, -1) }},
					m3.MenuItem{Icon: icons.ArrowDownwardRounded, Label: l10n.OutlineMoveDown, Trailing: "Alt+↓", OnTap: func() { x.move(item, 1) }},
					m3.MenuItem{Icon: icons.DeleteOutlineRounded, Label: l10n.OutlineDelete, Danger: true, Trailing: "Del", OnTap: func() { x.remove(item) }},
				)
			})
			m3.Icon(c, icons.DragIndicatorRounded, 16, cs.OutlineVariant).Margin(design.SpaceXS).Cursor(ui.CursorGrab)
		})
	})
	// A press takes the row for keys; a field under it takes the focus
	// after.
	if row.Pressed() && !focused {
		r.focus, r.wantKeys = id, r.editing == ""
	}
	return row.Hovered()
}

func (x reviewCtx) rowKeys(row ui.Element, item study.M) {
	// row is the panel, which takes the keys of the row selected.
	switch {
	case row.Shortcut(ui.Alt, ui.KeyUp):
		x.move(item, -1)
	case row.Shortcut(ui.Alt, ui.KeyDown):
		x.move(item, 1)
	case row.Shortcut(0, ui.KeyA):
		x.toggleConfirm(item)
	case row.Shortcut(0, ui.KeyF):
		x.toggleCheck(item)
	case row.Shortcut(0, ui.KeyEnter), row.Shortcut(0, ui.KeyE):
		x.edit(item)
	case row.Shortcut(0, ui.KeyDelete), row.Shortcut(0, ui.KeyBackspace):
		x.remove(item)
	case row.Shortcut(0, ui.KeyDown), row.Shortcut(0, ui.KeyUp):
		by := 1
		if row.Shortcut(0, ui.KeyUp) {
			by = -1
		}
		var ids []int
		rows := reviewRows(x.proposal())
		at := -1
		for i, rr := range rows {
			if rr.item != nil {
				if rr.id() == core.Str(item["id"]) {
					at = len(ids)
				}
				ids = append(ids, i)
			}
		}
		if next := at + by; at >= 0 && next >= 0 && next < len(ids) {
			x.r.list.ScrollIntoView(ids[next])
			x.r.pendingFocus = rows[ids[next]].id()
		}
	}
}

func (x reviewCtx) itemEditor(c *ui.Context, item study.M) {
	r := x.r
	cs, tt := design.S(), design.TT()
	id := core.Str(item["id"])
	st := tt.BodyMedium
	in := m3.Field{Filled: true, Fill: cs.SurfaceContainerHighest.Alpha(.6), Borderless: true, Multiline: true, MinLines: 1, MaxLines: 12,
		Pad: true, PadX: design.SpaceSM, PadY: design.SpaceSM, Radius: design.CornerSM, Style: &st, Helper: l10n.OutlineEditHint,
		Dense: true, AutoFocus: true}.Show(c.Key("outline-edit-"+id), r.edits[id])
	if !r.editFocused {
		in.Focus()
	}
	in.HandleInput(func(ev ui.InputEvent) bool {
		if ev.Kind != ui.InputKeyDown {
			return false
		}
		switch {
		case ev.Key == ui.KeyEnter && ev.Mods == 0:
			core.Post(func() { x.commit(item) })
			return true
		case ev.Key == ui.KeyEscape && ev.Mods == 0:
			core.Post(func() { x.cancelEdit(item) })
			return true
		}
		return false
	})
	// A click elsewhere commits, as onTapOutside does.
	if in.FocusWithin() {
		r.editFocused = true
	} else if r.editFocused {
		core.Post(func() {
			if r.editing == id {
				x.commit(item)
			}
		})
	}
}

func tinyAction(c *ui.Context, key, label string) bool {
	cs, tt := design.S(), design.TT()
	e := ui.ButtonBase(c.Key(key)).Radius(design.CornerXS).Padding(design.SpaceXXS).Cursor(ui.CursorPointer)
	m3.Ink(e, cs.Primary, design.CornerXS)
	e.Children(func() {
		m3.Text(c, label, tt.LabelSmall.C(cs.Primary)).Underline().DecorationColor(cs.Primary.Alpha(.4))
	})
	return e.Clicked()
}

// reviewTick is _ReviewTick: a ring until accepted, a filled check after,
// a flag while it waits to be checked.
func (x reviewCtx) reviewTick(c *ui.Context, review string, item study.M) {
	cs := design.S()
	ic, col, tip := icons.RadioButtonUncheckedRounded, cs.Outline, l10n.OutlineAccept
	switch review {
	case "confirmed":
		ic, col, tip = icons.CheckCircleRounded, cs.Primary, l10n.OutlineUnaccept
	case "to_check":
		ic, col = icons.FlagCircleRounded, outlineAmber()
	}
	e := ui.ButtonBase(c.Key("tick")).Size(28, 28).Radius(14).Tooltip(tip).Cursor(ui.CursorPointer)
	m3.Ink(e, col, 14)
	e.Children(func() {
		m3.Icon(c, ic, 20, col).Key(review).
			Transition(design.Switcher(design.Short4, design.EmphasizedDecelerate, nil, ui.Motion{Opacity: 0, Scale: .6}))
	})
	if e.Clicked() {
		x.toggleConfirm(item)
	}
}

// inkThumb is _InkThumb: the handwriting a mark came from, drawn small; a
// click carries the canvas to it.
func (x reviewCtx) inkThumb(c *ui.Context, item study.M) {
	cs := design.S()
	strokes := x.s.markInk(item)
	tip := l10n.OutlineShowInk
	if len(strokes) == 0 {
		tip = l10n.OutlineNoInk
	}
	e := ui.ButtonBase(c.Key("ink")).Size(44, 44).Radius(design.CornerSM).Background(design.ARGB(0xfffdfcf8)).
		Border(1, cs.OutlineVariant).Tooltip(tip).Disabled(len(strokes) == 0).DimDisabled(false)
	if len(strokes) == 0 {
		e.Children(func() { m3.Icon(c, icons.GestureRounded, 18, cs.Outline) })
	} else {
		e.Cursor(ui.CursorPointer).Draw(func(p *ui.Painter, r ui.Rect) { paintInkFit(p, r, strokes, 5) })
	}
	if e.Clicked() {
		if a, ok := item["anchor"].(map[string]any); ok {
			x.s.followEvidence(core.J{"id": core.Str(item["id"]), "kind": "visual_observation", "status": "verified", "anchor": a})
		}
	}
}

// paintInkFit is paintStudyInk: strokes fitted into r, keeping their
// proportions.
func paintInkFit(p *ui.Painter, r ui.Rect, strokes []*study.Stroke, inset float32) {
	if len(strokes) == 0 {
		return
	}
	b := strokes[0].Bounds()
	for _, st := range strokes[1:] {
		b = b.Union(st.Bounds())
	}
	scale := float32(math.Min(float64(r.W-2*inset)/math.Max(b.W(), 1), float64(r.H-2*inset)/math.Max(b.H(), 1)))
	center := b.Center()
	t := ui.Translation(-f32(center.X), -f32(center.Y)).Then(ui.Scaling(scale, scale)).Then(ui.Translation(r.X+r.W/2, r.Y+r.H/2))
	p.Transform(t, 0, 0, func() {
		for _, st := range strokes {
			paintInk(p, study.Pt{}, st.Points, st.Color, st.Width, st.Tool, st.Brush)
		}
	})
}

func (x reviewCtx) markChoices(c *ui.Context, item study.M) {
	cs, tt := design.S(), design.TT()
	choice := core.Str(item["choice"])
	id := core.Str(item["id"])
	custom := x.r.edits[id+"-custom"]
	if custom == nil {
		v := core.Str(item["choice_text"])
		custom = &v
		x.r.edits[id+"-custom"] = custom
	}
	ui.Column(c).FillWidth().Padding(design.SpaceSM, 0, 0, 0).AlignItems(ui.Start).Children(func() {
		if choice == "" {
			m3.Text(c, l10n.OutlineMarkHint, tt.LabelSmall.C(cs.Tertiary))
		}
		ui.Box(c).Height(design.SpaceXS)
		ui.Row(c).FillWidth().Wrap().Gap(design.SpaceXS).Children(func() {
			for _, k := range study.OutlineChoices {
				on := choice == k
				if (m3.Chip{Kind: m3.FilterChip, Label: study.OutlineChoiceLabel(k, ""), Selected: on, NoCheck: true, Compact: true}).
					Show(c.Key("outline-choice-" + id + "-" + k)).Clicked() {
					switch {
					case on:
						x.choose(item, "", "")
					case k == "custom":
						text := strings.TrimSpace(*custom)
						if text == "" {
							text = l10n.OutlineChoiceCustom
						}
						x.choose(item, "custom", text)
					default:
						x.choose(item, k, "")
					}
				}
			}
		})
		if choice == "custom" {
			st := tt.BodySmall
			in := m3.Field{Hint: l10n.OutlineCustomHint, MaxLength: 60, Dense: true, Style: &st}.Show(c.Key("outline-custom-"+id), custom)
			x.snapOnce(in, id+"-custom")
			if in.Changed() {
				v := strings.TrimSpace(*custom)
				if v == "" {
					v = l10n.OutlineChoiceCustom
				}
				x.change(func() { item["choice_text"] = v }, false)
			}
		}
		if choice != "" && tinyAction(c, "undo-mark", l10n.OutlineMarkUndo) {
			x.choose(item, "", "")
		}
	})
}

func (x reviewCtx) appendix(c *ui.Context, byID map[string]core.J, numbers map[string]int) {
	s, p := x.s, x.proposal()
	cs, tt := design.S(), design.TT()
	var conflicts []core.J
	for _, v := range core.List(p["conflicts"]) {
		if m := core.Obj(v); m != nil {
			conflicts = append(conflicts, m)
		}
	}
	questions, missing := core.Strs(p["unresolved_questions"]), core.Strs(p["missing_sources"])
	if len(conflicts) == 0 && len(questions) == 0 && len(missing) == 0 {
		ui.Box(c).Height(design.SpaceSM)
		return
	}
	line := func(build func()) {
		ui.Row(c).FillWidth().AlignItems(ui.Start).Padding(0, 0, design.SpaceXS, 0).Children(func() {
			ui.Box(c).Size(5, 5).Radius(2.5).Background(cs.Outline).Margin(7, design.SpaceSM, 0, 0).Shrink(0)
			ui.Box(c).Grow(1).Basis(0).Children(build)
		})
	}
	group := func(title string, build func()) {
		ui.Column(c).FillWidth().Padding(design.SpaceMD, 0, 0, 0).AlignItems(ui.Stretch).Children(func() {
			m3.Text(c, title, tt.LabelMedium.C(cs.OnSurfaceVariant).W(600))
			ui.Box(c).Height(design.SpaceXS)
			build()
		})
	}
	st := tt.BodySmall.H(1.45)
	ui.Column(c).FillWidth().Margin(design.SpaceLG, 0, 0, 0).Padding(design.SpaceMD).Radius(design.CornerLG).
		Background(cs.SurfaceContainerHighest.Alpha(.45)).AlignItems(ui.Stretch).Children(func() {
		m3.Text(c, l10n.OutlineAppendix, tt.TitleSmall.W(600).C(cs.OnSurface))
		m3.Text(c, l10n.OutlineAppendixNote, tt.LabelSmall.C(cs.OnSurfaceVariant))
		if len(conflicts) > 0 {
			group(l10n.OutlineConflicts, func() {
				for i, cf := range conflicts {
					text := strings.ReplaceAll(cf.S("text"), "\n", "  \n")
					for _, e := range cf.Ss("evidence_ids") {
						if _, ok := byID[e]; !ok {
							continue
						}
						if _, ok := numbers[e]; !ok {
							numbers[e] = len(numbers) + 1
						}
						text += " [" + strconv.Itoa(numbers[e]) + "](coursive-evidence:" + e + ")"
					}
					ui.Box(c.Key(i)).FillWidth().Children(func() {
						line(func() {
							CoursiveMarkdown{Data: text, Style: &st, LectureID: s.lectureID(), OnEvidence: func(e string) {
								if ev, ok := byID[e]; ok {
									s.followEvidence(ev)
								}
							}}.Show(c)
						})
					})
				}
			})
		}
		if len(questions) > 0 {
			group(l10n.OutlineQuestions, func() {
				for _, q := range questions {
					line(func() { CoursiveMarkdown{Data: q, Style: &st, LectureID: s.lectureID()}.Show(c) })
				}
			})
		}
		if len(missing) > 0 {
			group(l10n.OutlineUnread, func() {
				for _, q := range missing {
					line(func() { m3.Text(c, q, tt.BodySmall.C(cs.OnSurfaceVariant)) })
				}
			})
		}
	})
}

func (x reviewCtx) footer(c *ui.Context, narrow bool) {
	s, r, p := x.s, x.r, x.proposal()
	cs, tt := design.S(), design.TT()
	accepted := p["state"] == "accepted"
	dirty := x.dirty()
	conversation := ""
	if card := s.doc.Card(x.cv.cardID); card != nil {
		conversation = card.ConversationID
	}
	status := r.err
	if status == "" {
		status = r.notice
	}
	if status == "" && accepted {
		status = l10n.OutlineSaved
		if dirty {
			status = l10n.OutlineDirty
		}
	}
	ui.Column(c).FillWidth().Padding(design.SpaceSM, design.SpaceSM, 0, 0).AlignItems(ui.Stretch).Children(func() {
		if status != "" {
			ui.Row(c.Key("outline-status")).FillWidth().AlignItems(ui.Center).Padding(0, 0, design.SpaceSM, 0).
				Transition(ui.ElementTransition{Duration: design.Short4, Enter: &ui.Motion{Opacity: 0, Collapse: true}, Exit: &ui.Motion{Opacity: 0, Collapse: true}, Shadows: true}).
				Children(func() {
					ic, col := icons.CheckCircleOutlineRounded, cs.OnSurfaceVariant
					switch {
					case r.err != "":
						ic, col = icons.ErrorOutlineRounded, cs.Error
					case dirty:
						ic = icons.EditNoteRounded
					}
					m3.Icon(c, ic, 16, col)
					ui.Box(c).Width(design.SpaceXS)
					m3.Text(c, status, tt.LabelSmall.C(col)).Grow(1).Basis(0)
					switch {
					case r.conflict:
						if m3.TextButton(c, l10n.OutlineSaveCopy).Clicked() {
							x.save(true)
						}
					case r.err != "":
						if m3.TextButton(c, l10n.OutlineRetry).Clicked() {
							x.save(false)
						}
					}
				})
		}
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			label, ic := l10n.OutlineSave, icons.NoteAddOutlined
			if accepted {
				label, ic = l10n.OutlineUpdate, icons.SyncRounded
			}
			if r.saving {
				label = l10n.OutlineSaving
			}
			ui.Box(c).Grow(1).Basis(0).Children(func() {
				b := m3.Button{Kind: m3.Filled, Label: label, Icon: ic, Disabled: r.saving || (accepted && !dirty), Busy: r.saving}
				if b.Show(c.Key("outline-save")).FillWidth().Clicked() {
					x.save(false)
				}
			})
			ui.Box(c).Width(design.SpaceXS)
			turn := x.turn
			m3.Menu(c, "outline-export", m3.BelowEnd, func(t ui.Element, open bool) {
				t.Children(func() {
					m3.IconButton{Kind: m3.OutlinedIcon, Icon: icons.IosShareRounded, Size: 18, Tooltip: l10n.OutlineExport}.Show(c.Key("outline-export"))
				})
			}, func() []m3.MenuItem {
				return []m3.MenuItem{
					{Icon: icons.DescriptionOutlined, Label: l10n.OutlineExportMarkdown, OnTap: x.exportMarkdown},
					{Icon: icons.PictureAsPdfOutlined, Label: l10n.OutlineExportPdf, OnTap: func() { s.exportOutlinePdf(turn) }},
				}
			})
			pinned := s.pinnedCard(turn, conversation, nil)
			if pinned == nil {
				if (m3.IconButton{Icon: icons.PushPinOutlined, Size: 18, Tooltip: "留在画布"}).Show(c.Key("outline-pin")).Clicked() {
					title := strings.TrimSpace(core.Str(p["title"]))
					s.pinAnswer(turn, conversation, x.markdown(), title)
				}
			} else if (m3.IconButton{Icon: icons.CheckRounded, Size: 18, Tooltip: "已在画布 · 查看"}).Show(c.Key("outline-pinned")).Clicked() {
				s.revealCard(pinned)
			}
		})
		if !narrow {
			m3.Text(c, l10n.OutlineKeys, tt.LabelSmall.C(cs.Outline)).TextAlign(ui.Center).Margin(design.SpaceSM, 0, 0, 0)
		}
	})
}
