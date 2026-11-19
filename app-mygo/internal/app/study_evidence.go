package app

import (
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"regexp"
	"runtime"
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

// study_evidence.dart and study_source_navigation.dart: grounded answers'
// numbered citations, the view a citation leads to and the way back.
// Contract: docs/specs/0027-grounded-canvas-answers.md.

var (
	evidenceMarker = regexp.MustCompile(`\[\[e:([A-Za-z0-9_-]{1,12})\]\]`)
	answerMarkers  = regexp.MustCompile(`\[\[(?:e|next):[^\]\n]*\]\]`)
	legacyCitation = regexp.MustCompile(`\[[^\]\n]+\]\(coursive://citation/(e\d+)\)`)
)

// studyEvidenceMarkdown replaces host-issued markers with numbered links in
// the order of evidence; a marker without kept evidence is dropped.
func studyEvidenceMarkdown(answer string, evidence []core.J) string {
	order := map[string]int{}
	for i, e := range evidence {
		order[e.S("id")] = i + 1
	}
	return evidenceMarker.ReplaceAllStringFunc(answer, func(m string) string {
		id := evidenceMarker.FindStringSubmatch(m)[1]
		if n, ok := order[id]; ok {
			return fmt.Sprintf("[%d](coursive-evidence:%s)", n, id)
		}
		return ""
	})
}

// studyPlainAnswer is the answer as a reader keeps it: no markers.
func studyPlainAnswer(answer string) string { return answerMarkers.ReplaceAllString(answer, "") }

// studyUpgradeCitations loads answers from the first course-scope builds,
// which linked citations as [n](coursive://citation/eN) beside a citations
// table, as schema 2 evidence.
func studyUpgradeCitations(turn core.J) {
	result := core.Obj(turn["result"])
	if result == nil || result.I("schema_version") == 2 || result["citations"] == nil {
		return
	}
	var evidence []any
	ids := map[string]bool{}
	for _, c := range result.Os("citations") {
		if core.Obj(c["source"]) != nil && strings.HasPrefix(c.S("id"), "e") && len(evidence) < 24 {
			evidence = append(evidence, legacyEvidence(c))
			ids[c.S("id")] = true
		}
	}
	turn["answer"] = legacyCitation.ReplaceAllStringFunc(turn.S("answer"), func(m string) string {
		id := legacyCitation.FindStringSubmatch(m)[1]
		if ids[id] {
			return "[[e:" + id + "]]"
		}
		return ""
	})
	sources := result["sources"]
	if sources == nil {
		sources = []any{}
	}
	turn["result"] = core.J{"sources": sources, "schema_version": 2, "evidence": evidence, "suggestions": []any{}}
}

func legacyEvidence(c core.J) core.J {
	s := core.Obj(c["source"]).Copy()
	delete(s, "text")
	delete(s, "snippet")
	from, to, page := s.I("time_start_ms"), s.I("time_end_ms"), s.I("page")
	label := []rune(c.S("label"))
	kind, akind := "exact_text", "text"
	if to > from {
		kind, akind = "timestamped_transcript", "video"
	} else if page > 0 {
		akind = "pdf"
	}
	anchor := core.J{"id": s.S("id"), "workspace_id": "", "object_id": "", "kind": akind,
		"source_lecture_id": s.S("lecture_id"), "original_path": s.S("path"), "fingerprint": s.S("fingerprint")}
	if page > 0 {
		anchor["page"] = page
	}
	if to > from {
		anchor["time_start_ms"], anchor["time_end_ms"] = from, to
	}
	return core.J{"id": c.S("id"), "source_id": "course-" + s.S("id"), "kind": kind, "status": "verified",
		"label": string(label[:min(120, len(label))]), "anchor": anchor, "course_source": s}
}

// studyPinGeneration is a pinned answer's identity: its conversation turn,
// hashed (FNV-1a) when the key would pass the 100 characters a card's
// generation holds.
func studyPinGeneration(conversation, turn string) string {
	key := "canvas-turn:" + conversation + ":" + turn
	if len([]rune(key)) <= 100 && study.Len16(key) <= 100 {
		return key
	}
	h := fnv.New64a()
	h.Write([]byte(key))
	return fmt.Sprintf("canvas-turn:#%x", h.Sum64())
}

// studyTurnEvidence is a saved turn's schema 2 citations; nil for older
// turns.
func studyTurnEvidence(turn core.J) []core.J {
	result := core.Obj(turn["result"])
	if result == nil || result.I("schema_version") != 2 {
		return nil
	}
	out := result.Os("evidence")
	if out == nil {
		out = []core.J{}
	}
	return out
}

func anchorOf(v any) study.Anchor { return study.AnchorFrom(plainMap(v)) }

// evidenceView is where a citation points on the canvas, in world
// coordinates.
type evidenceView struct {
	card *study.Card
	// rects are text lines, a region or ink bounds; none means the whole
	// card.
	rects []study.Rect
	// page is a PDF page the card is not showing.
	page int
	// problem is why the citation cannot be followed.
	problem string
}

func (v evidenceView) bounds() *study.Rect {
	if len(v.rects) == 0 {
		if v.card == nil {
			return nil
		}
		return rectPtr(v.card.Rect)
	}
	b := v.rects[0]
	for _, r := range v.rects[1:] {
		b = b.Union(r)
	}
	return &b
}

// viewMark is a view left by following a citation.
type viewMark struct {
	camera                study.Pt
	zoom                  float64
	stageOpen, reviewPeek bool
	selection             map[string]bool
	thread                string
	region                *markRegion
	// card is the PDF card navigation turned, shownPage the page it went
	// to and page what it showed before.
	card      string
	shownPage int
	page      *markPage
	// video is the video navigation paused, at videoAt from videoBefore.
	video                string
	videoBefore, videoAt int
}

type markRegion struct {
	region  study.Rect
	bounds  *study.Rect
	polygon []study.Pt
	detail  study.Selection
}

type markPage struct {
	page   int
	path   string
	text   string
	boxes  []any
	aspect float64
}

// evidenceParts is the citation state of the canvas.
type evidenceParts struct {
	bookmarks         []*viewMark
	evidenceFocus     core.J
	evidenceSweep     time.Time
	evidenceLanding   time.Time
	sourceNavigations map[string]bool
}

func (s *StudyCanvas) evidenceCard(a study.Anchor) *study.Card {
	for _, c := range s.doc.Cards {
		if (a.ObjectID == "lecture" && c.Type == "lecture") || (a.ObjectID != "lecture" && c.ID == a.ObjectID) {
			return c
		}
	}
	return nil
}

// evidenceOf validates evidence against the canvas as it is now.
func (s *StudyCanvas) evidenceOf(evidence core.J) evidenceView {
	d := s.doc
	a := anchorOf(evidence["anchor"])
	if st := evidence.S("status"); st != "" && st != "verified" {
		return evidenceView{problem: l10n.StudyEvidenceStale}
	}
	// A course passage is checked against its file when it is followed.
	if core.Obj(evidence["course_source"]) != nil {
		return evidenceView{}
	}
	if a.WorkspaceID != "" && a.WorkspaceID != s.lectureID() {
		return evidenceView{problem: l10n.StudyEvidenceOtherLecture}
	}
	if a.Kind == "ink" {
		var b *study.Rect
		n := 0
		want := map[string]bool{}
		for _, id := range a.StrokeIDs {
			want[id] = true
		}
		for _, st := range d.Strokes {
			if want[st.ID] && d.Visible(st.Layer) {
				n++
				if b == nil {
					b = rectPtr(st.Bounds())
				} else {
					*b = b.Union(st.Bounds())
				}
			}
		}
		if n == 0 || n != len(a.StrokeIDs) {
			return evidenceView{problem: l10n.StudyEvidenceStale}
		}
		return evidenceView{rects: []study.Rect{*b}}
	}
	card := s.evidenceCard(a)
	if card == nil || !d.Visible(card.Layer) {
		return evidenceView{problem: l10n.StudyEvidenceMissing}
	}
	upright := card.QuarterTurns%4 == 0
	if card.Type == "pdfdoc" && a.Page > 0 && card.Page != a.Page {
		return evidenceView{card: card, page: a.Page}
	}
	if evidence.S("kind") == "exact_text" && a.End > a.Start {
		text := card.ContentText()
		if card.Type == "pdfdoc" {
			text = card.Text
		}
		if a.End > study.Len16(text) || (a.Quote != "" && study.Sub16(text, a.Start, a.End) != a.Quote) {
			return evidenceView{problem: l10n.StudyEvidenceStale}
		}
		if card.Type == "pdfdoc" && upright {
			if lines := textLines(card, a.Start, a.End); len(lines) > 0 {
				return evidenceView{card: card, rects: lines}
			}
		}
	}
	if a.Region != nil && upright && card.Type != "lecture" {
		b, r := sourceBounds(card), *a.Region
		return evidenceView{card: card, rects: []study.Rect{study.RectLTWH(b.L+r.L*b.W(), b.T+r.T*b.H(), r.W()*b.W(), r.H()*b.H())}}
	}
	return evidenceView{card: card}
}

// textLines are the cited range's lines on a page, from its word boxes.
func textLines(card *study.Card, start, end int) []study.Rect {
	b := sourceBounds(card)
	n := func(v any) float64 {
		f := core.Float(v)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return 0
		}
		return f
	}
	var lines []study.Rect
	for _, raw := range card.TextBoxes {
		box := core.Obj(raw)
		s, e := n(box["start"]), n(box["end"])
		if e <= float64(start) || s >= float64(end) {
			continue
		}
		r := study.RectLTWH(b.L+n(box["x"])*b.W(), b.T+n(box["y"])*b.H(), n(box["w"])*b.W(), n(box["h"])*b.H())
		// Words on one baseline read as one highlighted line.
		if k := len(lines) - 1; k >= 0 && math.Abs(r.Center().Y-lines[k].Center().Y) < lines[k].H()*.5 && r.L >= lines[k].L-1 {
			lines[k] = lines[k].Union(r)
		} else {
			lines = append(lines, r)
		}
	}
	return lines
}

// evidenceLocator is a short locator: page, time range, region or ink.
func (s *StudyCanvas) evidenceLocator(evidence core.J) string {
	a := anchorOf(evidence["anchor"])
	switch {
	case evidence.S("kind") == "timestamped_transcript":
		return core.FmtClock(a.TimeStartMs) + "–" + core.FmtClock(a.TimeEndMs)
	case evidence.S("kind") == "visual_observation":
		return l10n.StudyEvidenceSeen
	case a.Kind == "ink":
		return l10n.StudyEvidenceInk
	case a.Page > 0:
		return l10n.StudySourcePage(a.Page)
	}
	return evidence.S("label")
}

// staleEvidence are the citations whose material no longer holds.
func (s *StudyCanvas) staleEvidence(evidence []core.J) map[string]bool {
	out := map[string]bool{}
	if s.doc == nil {
		return out
	}
	for _, e := range evidence {
		if s.evidenceOf(e).problem != "" {
			out[e.S("id")] = true
		}
	}
	return out
}

// previewEvidence lights the cited material; nothing moves.
func (s *StudyCanvas) previewEvidence(evidence core.J) {
	if sameJ(s.evidenceFocus, evidence) {
		return
	}
	s.evidenceLanding = time.Time{}
	s.evidenceFocus = evidence
	if evidence != nil {
		s.evidenceSweep = time.Now()
	}
}

func sameJ(a, b core.J) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return fmt.Sprintf("%p", a) == fmt.Sprintf("%p", b)
}

// followEvidence validates, remembers the current view and carries it to
// evidence.
func (s *StudyCanvas) followEvidence(evidence core.J) {
	d := s.doc
	if d == nil {
		return
	}
	if core.Obj(evidence["course_source"]) != nil {
		s.followCourseEvidence(evidence)
		return
	}
	view := s.evidenceOf(evidence)
	if view.problem != "" {
		s.toast(view.problem)
		return
	}
	a := anchorOf(evidence["anchor"])
	card := view.card
	if view.page > 0 && card != nil {
		api := State.Api()
		core.Async(func() (any, error) { return api.StudyPDFPage(s.lectureID(), card.Source, view.page) }, func(out any, err error) {
			if s.disposed || s.doc != d {
				return
			}
			page := core.Obj(out)
			if err != nil || page == nil {
				s.toast(l10n.StudyEvidenceMissing)
				return
			}
			found := false
			for _, c := range d.Cards {
				found = found || c == card
			}
			if !found {
				return
			}
			text := page.S("text")
			if a.End > study.Len16(text) || (a.Quote != "" && study.Sub16(text, a.Start, a.End) != a.Quote) {
				s.toast(l10n.StudyEvidenceStale)
				return
			}
			s.landOnEvidence(evidence, view, page)
		})
		return
	}
	s.landOnEvidence(evidence, view, nil)
}

func (s *StudyCanvas) landOnEvidence(evidence core.J, view evidenceView, page core.J) {
	d := s.doc
	a := anchorOf(evidence["anchor"])
	card := view.card
	mark := &viewMark{camera: d.Camera, zoom: d.Zoom, stageOpen: s.stageOpen, selection: map[string]bool{},
		reviewPeek: s.reviewPeek, thread: s.activeConversation}
	for id := range s.selection {
		mark.selection[id] = true
	}
	if s.regionSelection != nil && s.selectionReady {
		r := &markRegion{region: *s.regionSelection, polygon: append([]study.Pt(nil), s.selectionPolygon...), detail: s.detail}
		r.detail.StrokeIDs = map[string]bool{}
		for id := range s.detail.StrokeIDs {
			r.detail.StrokeIDs[id] = true
		}
		if s.selectionBounds != nil {
			r.bounds = rectPtr(*s.selectionBounds)
		}
		mark.region = r
	}
	if page != nil && card != nil {
		mark.card, mark.shownPage = card.ID, view.page
		mark.page = &markPage{page: card.Page, path: card.Path, text: card.Text, boxes: card.TextBoxes, aspect: card.PageAspect}
		// Reading state, like the camera: no undo entry and no edit.
		if h := page.F("height"); h > 0 {
			card.PageAspect = page.F("width") / h
		}
		card.Page, card.Path, card.Text = view.page, page.S("file"), page.S("text")
		card.TextBoxes = core.List(plainJSON(page["boxes"]))
		view = s.evidenceOf(evidence)
	}
	if a.Kind == "video" && card != nil {
		// A cited moment is shown, not played over the learner.
		mark.video, mark.videoAt = card.ID, a.TimeStartMs
		if card.Type == "lecture" {
			mark.videoBefore = s.ctl.PositionMs()
			s.ctl.SeekTo(a.TimeStartMs)
			if s.ctl.Player != nil {
				s.ctl.Player.Pause()
			}
		} else {
			mark.videoBefore = s.videoTimes[card.ID]
			if v := s.videos[card.ID]; v != nil && v.player != nil {
				mark.videoBefore = int(v.player.Position().Milliseconds())
			}
			s.videoTimes[card.ID] = a.TimeStartMs
			_ = s.seekVideoPaused(card.ID, a.TimeStartMs)
		}
	}
	s.bookmarks = append(s.bookmarks, mark)
	if len(s.bookmarks) > 10 {
		s.bookmarks = s.bookmarks[1:]
	}
	// A narrow window gives the whole view to the source until the return.
	if s.viewport.X < 720 {
		s.stageOpen = false
		if s.review != nil {
			s.reviewPeek = true
		}
	}
	clear(s.selection)
	s.regionSelection = nil
	s.selectionReady = false
	s.flyToEvidence(view)
	s.evidenceFocus = evidence
	s.evidenceSweep = time.Now()
	s.evidenceLanding = time.Now()
}

// openArea is the part of the window showing the canvas, without the
// sidebar, the top controls and the pen dock.
func (s *StudyCanvas) openArea() study.Rect {
	area := study.Rect{L: sp(design.SpaceLG), T: s.topBarsBottom() + sp(design.SpaceXXXL), R: s.viewport.X - sp(design.SpaceLG), B: s.viewport.Y - sp(design.SpaceHuge)}
	// The open review takes its side, or the bottom of a narrow window.
	if r := s.reviewRect(); r != nil {
		switch {
		case s.reviewNarrow():
			area.B = math.Min(area.B, r.T-sp(design.SpaceSM))
		case r.Center().X < s.viewport.X/2:
			area.L = math.Max(area.L, r.R+sp(design.SpaceLG))
		default:
			area.R = math.Min(area.R, r.L-sp(design.SpaceLG))
		}
	}
	if s.stageOpen && s.viewport.X >= 720 {
		w := math.Min(420, math.Max(240, s.viewport.X-16)) + sp(design.SpaceLG)
		if s.aiPlacement.edge == dockLeft {
			area.L += w
		} else {
			area.R -= w
		}
	}
	return area
}

func (s *StudyCanvas) flyToEvidence(view evidenceView) {
	t := view.bounds()
	if t == nil {
		return
	}
	target := *t
	if card := view.card; card != nil && len(view.rects) > 0 && card.Type == "pdfdoc" {
		// Read a passage across the page's width, with the lines around it.
		page := sourceBounds(card)
		target = study.Rect{L: page.L, T: math.Max(page.T, target.T-page.H()*.12), R: page.R, B: math.Min(page.B, target.B+page.H()*.12)}
	}
	area := s.openArea()
	zoom := clampF(math.Min(area.W()/math.Max(1, target.W()), area.H()/math.Max(1, target.H())), .05, 2)
	s.flyTo(area.Center().Sub(target.Center().Mul(zoom)), zoom)
}

// goBack restores the newest view a citation left; false when there is
// none. Alt+← / ⌘[ follow PDF readers' back navigation, e.g. Zotero's.
func (s *StudyCanvas) goBack() bool {
	d := s.doc
	if d == nil || len(s.bookmarks) == 0 {
		return false
	}
	mark := s.bookmarks[len(s.bookmarks)-1]
	s.bookmarks = s.bookmarks[:len(s.bookmarks)-1]
	// Only a page the learner has not turned since is turned back.
	if card := d.Card(mark.card); card != nil && mark.page != nil && card.Page == mark.shownPage {
		p := mark.page
		card.Page, card.Path, card.Text, card.TextBoxes, card.PageAspect = p.page, p.path, p.text, p.boxes, p.aspect
	}
	if video := d.Card(mark.video); video != nil {
		if video.Type == "lecture" {
			if !s.ctl.Playing() && abs(s.ctl.PositionMs()-mark.videoAt) <= 1500 {
				s.ctl.SeekTo(mark.videoBefore)
			}
		} else {
			at, playing := s.videoTimes[video.ID], false
			if at == 0 {
				at = mark.videoAt
			}
			v := s.videos[video.ID]
			if v != nil && v.player != nil {
				at, playing = int(v.player.Position().Milliseconds()), v.player.Playing()
			}
			if !playing && abs(at-mark.videoAt) <= 1500 {
				s.videoTimes[video.ID] = mark.videoBefore
				if v != nil && v.player != nil {
					v.player.Seek(time.Duration(mark.videoBefore) * time.Millisecond)
				}
			}
		}
	}
	s.evidenceLanding = time.Time{}
	s.evidenceFocus = nil
	s.stageOpen, s.reviewPeek = mark.stageOpen, mark.reviewPeek
	clear(s.selection)
	for id := range mark.selection {
		if d.Card(id) != nil {
			s.selection[id] = true
		}
	}
	if d.Card(mark.thread) != nil {
		s.activeConversation = mark.thread
	}
	if r := mark.region; r != nil {
		det := r.detail
		s.detail.Clear()
		if d.Card(det.ObjectID) != nil {
			s.detail.ObjectID = det.ObjectID
		}
		s.detail.Start, s.detail.End, s.detail.Region = det.Start, det.End, det.Region
		ids := map[string]bool{}
		for _, st := range d.Strokes {
			ids[st.ID] = true
		}
		for id := range det.StrokeIDs {
			if ids[id] {
				s.detail.StrokeIDs[id] = true
			}
		}
		s.regionSelection = rectPtr(r.region)
		s.selectionBounds = rectPtr(r.region)
		if r.bounds != nil {
			s.selectionBounds = rectPtr(*r.bounds)
		}
		s.selectionPolygon = r.polygon
		s.selectionReady = true
	}
	s.flyTo(mark.camera, mark.zoom)
	return true
}

// sp is a design length in canvas units.
func sp(v float32) float64 { return float64(v) }

func isMac() bool { return runtime.GOOS == "darwin" }

// returnPill is 「返回原处」, shown while a citation's view can be left.
func (s *StudyCanvas) returnPill(c *ui.Context) {
	if len(s.bookmarks) == 0 {
		return
	}
	cs, tt := design.S(), design.TT()
	e := ui.ButtonBase(c.Key("study-return")).Absolute().Top(f32(s.topBarsBottom())+design.SpaceSM).Height(44).
		Padding(design.SpaceXS, design.SpaceLG, design.SpaceXS, design.SpaceMD).AlignItems(ui.Center).Cursor(ui.CursorPointer).
		Label(l10n.StudyReturn).
		Transition(design.Switcher(design.Medium1, design.EmphasizedDecelerate, design.EmphasizedAccelerate, ui.Motion{Opacity: 0, YPercent: -40}))
	// Centred by its width in the last frame.
	if b := e.Bounds(); b.W > 0 {
		e = e.Left(f32(s.viewport.X/2) - b.W/2)
	} else {
		e = e.Left(-10000)
		c.Invalidate()
	}
	widgets.Glass{Radius: design.CornerFull, Depth: design.GlassFloat}.Apply(e)
	s.recordControl(e)
	tip := l10n.StudyReturnKeys
	if isMac() {
		tip = l10n.StudyReturnKeysMac
	}
	m3.Tip(c, e, tip)
	m3.Ink(e, cs.OnSurface, design.CornerFull)
	e.Children(func() {
		m3.Icon(c, icons.UndoRounded, 18, cs.Primary)
		ui.Box(c).Width(design.SpaceSM)
		m3.Text(c, l10n.StudyReturn, tt.LabelLarge.C(cs.OnSurface))
		if n := len(s.bookmarks); n > 1 {
			ui.Box(c).Width(design.SpaceSM)
			m3.Text(c, fmt.Sprint(n), tt.LabelMedium.C(cs.OnSurfaceVariant)).FontFeatures("tnum")
		}
	})
	if e.Clicked() {
		s.goBack()
	}
}

// paintEvidence lights the cited material like a highlighter pass;
// material out of view gets a small marker on the nearest edge instead.
func (s *StudyCanvas) paintEvidence(p *ui.Painter, size ui.Rect) {
	evidence, d := s.evidenceFocus, s.doc
	if evidence == nil || d == nil {
		return
	}
	view := s.evidenceOf(evidence)
	b := view.bounds()
	if view.problem != "" || b == nil {
		return
	}
	fade := 1.0
	if !s.evidenceLanding.IsZero() {
		// A followed citation stays lit while the eye settles, then fades.
		t := float64(time.Since(s.evidenceLanding)) / float64(2400*time.Millisecond)
		if t >= 1 {
			s.evidenceLanding = time.Time{}
			s.evidenceFocus = nil
			return
		}
		k := clampF((t-.65)/.35, 0, 1)
		fade = 1 - k*k
		p.AnimationFrame()
	}
	sweep := 1.0
	if t := float64(time.Since(s.evidenceSweep)) / float64(design.Medium2); t < 1 {
		sweep = float64(design.EmphasizedDecelerate(f32(t)))
		p.AnimationFrame()
	}
	ch := studyChrome()
	accent := ch.accent
	screen := func(r study.Rect) ui.Rect {
		q := s.screenRect(r)
		return ui.Rect{X: size.X + f32(q.L), Y: size.Y + f32(q.T), W: f32(q.W()), H: f32(q.H())}
	}
	bounds := screen(*b)
	visible := ui.Rect{X: size.X + design.SpaceSM, Y: size.Y + design.SpaceSM, W: size.W - 2*design.SpaceSM, H: size.H - 2*design.SpaceSM}
	if !uiOverlaps(visible, bounds) {
		s.paintEvidenceEdge(p, visible, bounds, s.evidenceLocator(evidence), accent, ch.paper, fade)
		return
	}
	a := func(v float64) float32 { return f32(v * fade) }
	if len(view.rects) == 0 {
		// A whole card, or a page the card is not showing yet.
		r := ui.Rect{X: bounds.X - 4, Y: bounds.Y - 4, W: bounds.W + 8, H: bounds.H + 8}
		p.Shadow(r, design.CornerMD, 0, 0, 10, 0, accent.Alpha(a(.18)))
		p.Stroke(r, accent.Alpha(a(.8)), design.CornerMD, 2)
		if view.page > 0 {
			evidenceLabel(p, bounds.X+bounds.W-design.SpaceSM, bounds.Y+design.SpaceSM, s.evidenceLocator(evidence), accent, ch.paper, fade, true, false)
		}
		return
	}
	region := evidence.S("kind") != "exact_text"
	for _, rr := range view.rects {
		r := screen(rr)
		lit := ui.Rect{X: r.X - 2, Y: r.Y - 1, W: (r.W + 4) * f32(sweep), H: r.H + 2}
		alpha := .24
		if region {
			alpha = .1
		}
		p.Fill(lit, accent.Alpha(a(alpha)), 3)
		if region {
			p.Stroke(ui.Rect{X: r.X - 2, Y: r.Y - 2, W: r.W + 4, H: r.H + 4}, accent.Alpha(a(.85)), design.CornerXS, 1.5)
		}
	}
}

func uiOverlaps(a, b ui.Rect) bool {
	return a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H
}

func (s *StudyCanvas) paintEvidenceEdge(p *ui.Painter, visible, target ui.Rect, text string, accent, paper ui.Color, fade float64) {
	cx, cy := visible.X+visible.W/2, visible.Y+visible.H/2
	vx, vy := target.X+target.W/2-cx, target.Y+target.H/2-cy
	// Where the ray from the centre to the target leaves the visible area.
	kx, ky := float32(math.Inf(1)), float32(math.Inf(1))
	if vx != 0 {
		kx = (visible.W/2 - 28) / float32(math.Abs(float64(vx)))
	}
	if vy != 0 {
		ky = (visible.H/2 - 20) / float32(math.Abs(float64(vy)))
	}
	k := min(kx, ky)
	ax, ay := cx+vx*k, cy+vy*k
	p.FillPath((&ui.Path{}).Circle(ax, ay, 14), accent.Alpha(f32(.92*fade)))
	angle := math.Atan2(float64(vy), float64(vx))
	rot := func(x, y float64) (float32, float32) {
		sn, cn := math.Sincos(angle)
		return ax + f32(x*cn-y*sn), ay + f32(x*sn+y*cn)
	}
	x0, y0 := rot(5, 0)
	x1, y1 := rot(-3, -4.5)
	x2, y2 := rot(-3, 4.5)
	p.FillPath((&ui.Path{}).MoveTo(x0, y0).LineTo(x1, y1).LineTo(x2, y2).Close(), paper.Alpha(f32(fade)))
	end := vx > 0
	lx := ax + 20
	if end {
		lx = ax - 20
	}
	evidenceLabel(p, lx, ay, text, accent, paper, fade, end, true)
}

func evidenceLabel(p *ui.Painter, x, y float32, text string, accent, paper ui.Color, fade float64, alignEnd, middle bool) {
	if text == "" {
		return
	}
	span := ui.Span{Text: truncRunes(text, 28), Size: 11.5, Weight: 600, Color: paper.Alpha(f32(fade))}
	w, h := p.MeasureText(180, span)
	box := ui.Rect{X: x, Y: y, W: w + 16, H: h + 6}
	if alignEnd {
		box.X = x - w - 16
	}
	if middle {
		box.Y = y - (h+6)/2
	}
	p.Fill(box, accent.Alpha(f32(.92*fade)), box.H/2)
	p.RichText(box.X+8, box.Y+3, 180, span)
}

// focusSource shows a bound material's pill or a card's source like a
// citation.
func (s *StudyCanvas) focusSource(a study.Anchor) {
	kind := "source"
	if a.End > a.Start && a.Quote != "" {
		kind = "exact_text"
	}
	s.followEvidence(core.J{"id": a.ID, "kind": kind, "status": "verified", "anchor": a.JSON()})
}

// followCourseEvidence re-reads a course passage through the daemon first;
// a second click while that runs joins it instead of importing twice.
func (s *StudyCanvas) followCourseEvidence(evidence core.J) {
	cited := core.Obj(plainJSON(evidence["course_source"]))
	key := fmt.Sprintf("%s:%s:%s:%s", cited.S("id"), cited.S("fingerprint"), cited.S("offset"), cited.S("end"))
	if s.sourceNavigations[key] {
		return
	}
	s.sourceNavigations[key] = true
	d := s.doc
	go func() {
		defer onUI(func() { delete(s.sourceNavigations, key) })
		err := s.resolveCourseEvidence(d, evidence, cited)
		if err == nil {
			return
		}
		onUI(func() {
			if s.disposed {
				return
			}
			var apiErr *core.ApiError
			var fe formatError
			switch {
			case errors.As(err, &apiErr) && apiErr.Status == 409:
				// The file changed since the answer read it.
				s.toast(l10n.StudyEvidenceStale)
			case errors.As(err, &fe):
				s.toast(string(fe))
			default:
				s.toast(l10n.StudyCitationUnavailable)
			}
		})
	}()
}

// formatError is a FormatException: its message is shown as it is.
type formatError string

func (e formatError) Error() string { return string(e) }

var errGone = errors.New("canvas closed")

// resolveCourseEvidence finds the card holding a cited course passage,
// importing the material once, and follows the passage there. A changed
// file is reported, not guessed at.
func (s *StudyCanvas) resolveCourseEvidence(d *study.Document, evidence, cited core.J) error {
	api := State.Api()
	out, err := api.ResolveStudySource(s.lectureID(), cited)
	if err != nil {
		return err
	}
	source := core.Obj(plainJSON(out))
	gone := func() bool { return onUIValue(func() bool { return s.disposed || s.doc != d }) }
	if gone() {
		return nil
	}
	text := source.S("text")
	path := source.S("path")
	if source["video_path"] != nil {
		path = source.S("video_path")
	}
	lecture := source.S("lecture_id")
	page := source.I("page")
	exactPDF := source.S("kind") == "pdf" && page > 0
	video := source["video_path"] != nil
	excerpt := !video && !exactPDF
	identity := ""
	if excerpt {
		identity = fmt.Sprintf("excerpt:%s:%s:%s:%s", source.S("id"), source.S("offset"), source.S("end"), source.S("fingerprint"))
	}
	legacy := fmt.Sprintf("excerpt:%s:%s:%s", source.S("id"), source.S("offset"), source.S("fingerprint"))
	var card *study.Card
	var problem error
	onUI(func() {
		for _, c := range d.Cards {
			if c.OriginalPath != path || c.SourceLectureID != lecture {
				continue
			}
			match := false
			if excerpt {
				// Legacy cards omitted the end offset: reuse only an
				// unchanged full excerpt, never a shorter prefix.
				match = c.DocumentID == identity || (c.DocumentID == legacy && c.ContentText() == text)
			} else if video {
				match = c.Type == "video"
			} else {
				match = c.Type == "pdfdoc"
			}
			if match {
				card = c
				break
			}
		}
		if card != nil && !d.Visible(card.Layer) {
			problem = formatError(l10n.StudyEvidenceMissing)
		} else if card == nil && (!d.Editable(d.ActiveLayer) || len(d.Cards) >= 1998) {
			problem = formatError(l10n.StudyLocked)
		}
	})
	if problem != nil {
		return problem
	}
	if card == nil {
		if excerpt {
			onUI(func() {
				card = &study.Card{ID: study.NewID(), Type: "text", Layer: d.ActiveLayer, Rect: s.freeSpot(study.Pt{X: 480, Y: 360}),
					Title: evidence.S("label"), Text: text, OriginalPath: path, SourceLectureID: lecture, DocumentID: identity,
					Responses: map[string]string{}}
			})
		} else {
			imported, err := api.ImportStudyMaterial(s.lectureID(), path)
			if err != nil {
				return err
			}
			if gone() {
				return nil
			}
			objects := core.List(plainJSON(core.Obj(imported)["objects"]))
			if len(objects) == 0 {
				return formatError(l10n.StudyCitationUnavailable)
			}
			onUI(func() {
				card = study.CardFrom(plainMap(objects[0]))
				card.Layer = d.ActiveLayer
				card.Rect = s.freeSpot(study.Pt{X: card.Rect.W(), Y: card.Rect.H()})
			})
		}
		onUI(func() {
			d.Checkpoint()
			d.Cards = append(d.Cards, card)
			d.Changed()
		})
		if !s.flushWait() || gone() {
			return nil
		}
	}
	start, end, quote := 0, 0, ""
	if excerpt {
		end, quote = study.Len16(text), text
	} else if exactPDF {
		// The passage is lit where the page's own text holds it verbatim;
		// otherwise the page itself is the citation.
		shown := onUIValue(func() string {
			if card.Page == page {
				return card.Text
			}
			return ""
		})
		if shown == "" {
			out, err := api.StudyPDFPage(s.lectureID(), card.Source, page)
			if err != nil {
				return err
			}
			shown = core.Obj(out).S("text")
		}
		if gone() {
			return nil
		}
		words := strings.TrimSpace(text)
		if at := strings.Index(shown, words); words != "" && at >= 0 {
			start = study.Len16(shown[:at])
			end = start + study.Len16(words)
			quote = words
		}
	}
	onUI(func() {
		if excerpt && len(card.Sources) == 0 {
			q := []rune(text)
			card.Sources = append(card.Sources, study.Anchor{ID: evidence.S("id"), ObjectID: card.ID, Kind: "text",
				WorkspaceID: s.lectureID(), SourceLectureID: lecture, OriginalPath: path, Quote: string(q[:min(2000, len(q))])})
		}
		kind, akind := "source", "text"
		switch {
		case video:
			kind, akind = "timestamped_transcript", "video"
		case end > start:
			kind = "exact_text"
		}
		if !video && exactPDF {
			akind = "pdf"
		}
		a := study.Anchor{ID: evidence.S("id"), ObjectID: card.ID, Kind: akind, WorkspaceID: s.lectureID(), SourceLectureID: lecture,
			OriginalPath: path, Page: page, Start: start, End: end, TimeStartMs: source.I("time_start_ms"), TimeEndMs: source.I("time_end_ms"), Quote: quote}
		s.followEvidence(core.J{"id": evidence["id"], "label": evidence["label"], "kind": kind, "status": "verified", "anchor": a.JSON()})
	})
	return nil
}
