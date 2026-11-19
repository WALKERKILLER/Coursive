package app

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/study"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// study_theme.dart and study_interaction.dart: the canvas's colours,
// erasing, region selection and what floats over a selection.

// chrome is StudyChrome: the theme's colours as the canvas uses them.
type chrome struct {
	ink, inkSoft, inkFaint, line               ui.Color
	surface, surfaceSoft, surfaceStrong        ui.Color
	accent, onAccent, accentSoft, onAccentSoft ui.Color
	danger, shadow, paper, paperInk            ui.Color
}

func studyChrome() chrome {
	cs := design.S()
	dark := design.T().Dark
	ch := chrome{
		ink: cs.OnSurface, inkSoft: cs.OnSurfaceVariant, inkFaint: cs.Outline, line: cs.OutlineVariant,
		surface: cs.Surface, surfaceSoft: cs.SurfaceContainerLow, surfaceStrong: cs.SurfaceContainerHigh,
		accent: cs.Primary, onAccent: cs.OnPrimary, accentSoft: cs.PrimaryContainer, onAccentSoft: cs.OnPrimaryContainer,
		danger: cs.Error, shadow: cs.Shadow.Alpha(.12), paper: ui.RGB(255, 255, 255), paperInk: design.ARGB(0xff25344a),
	}
	// Cards sit one tonal step above the paper (lighter in dark mode).
	if dark {
		ch.shadow, ch.paper, ch.paperInk = cs.Shadow.Alpha(.45), cs.SurfaceContainerHigh, cs.OnSurface
	}
	return ch
}

func (c chrome) accentWash() ui.Color { return c.accent.Alpha(.12) }

func (c chrome) inkWash() ui.Color { return c.ink.Alpha(.08) }

// normalizeRegion is _normalizeRegion: r as fractions of parent.
func normalizeRegion(r, parent study.Rect) study.Rect {
	return study.RectLTWH(clampF((r.L-parent.L)/parent.W(), 0, 1), clampF((r.T-parent.T)/parent.H(), 0, 1),
		clampF(r.W()/parent.W(), .000001, 1), clampF(r.H()/parent.H(), .000001, 1))
}

// sourceBounds is _sourceBounds: where a card shows its source, the page
// itself for a PDF.
func sourceBounds(card *study.Card) study.Rect {
	if card.Type != "pdfdoc" {
		return card.Rect
	}
	area := study.RectLTWH(card.Rect.L, card.Rect.T+36, card.Rect.W(), math.Max(1, card.Rect.H()-72))
	aspect := card.PageAspect
	if aspect <= 0 {
		aspect = 1
	}
	w, h := area.W(), area.W()/aspect
	if h > area.H() {
		h = area.H()
		w = h * aspect
	}
	return study.RectCenter(area.Center(), w, h)
}

func (s *StudyCanvas) updateEraserCursor(local study.Pt) {
	s.eraserCursor = ptPtr(local)
	s.cursorRadius = s.eraserSize
}

func (s *StudyCanvas) eraseAt(a, b study.Pt) {
	d := s.doc
	if s.eraserMode == "stroke" {
		d.EraseWhole(a, b, s.eraserSize/d.Zoom)
	} else {
		d.Erase(a, b, s.eraserSize/d.Zoom)
	}
}

func (s *StudyCanvas) beginErase(local, p study.Pt) {
	s.doc.Checkpoint()
	s.startErase(local, p)
}

func (s *StudyCanvas) startErase(local, p study.Pt) {
	s.erasing, s.selectionReady = true, false
	s.regionSelection = nil
	s.detail.Clear()
	clear(s.selection)
	s.last = ptPtr(p)
	s.eraserLasso = nil
	if s.eraserMode == "lasso" {
		s.eraserLasso = []study.Pt{p}
	} else {
		s.eraseAt(p, p)
	}
	s.updateEraserCursor(local)
}

func (s *StudyCanvas) finishSelection() {
	d, r := s.doc, *s.marquee
	if r.W()*d.Zoom < 5 || r.H()*d.Zoom < 5 {
		return
	}
	polygon := []study.Pt{r.TopLeft(), r.TopRight(), r.BottomRight(), r.BottomLeft()}
	if !s.lassoRectangle {
		polygon = append([]study.Pt(nil), s.lasso...)
	}
	s.setRegionSelection(r, polygon)
	s.quickAsk, s.quickError, s.quickImages = false, "", false
	if s.captureLasso {
		s.captureRegionCard()
	}
}

// rectInPolygon reports a rectangle wholly inside a polygon, as the
// rectangle less the loop being empty: its corners inside, no edge of the
// loop crossing one of its sides, and no corner of the loop within it,
// which would cut a piece out.
func rectInPolygon(r study.Rect, polygon []study.Pt) bool {
	corners := []study.Pt{r.TopLeft(), r.TopRight(), r.BottomRight(), r.BottomLeft()}
	for _, p := range corners {
		if !study.InPolygon(p, polygon) && !onPolygon(p, polygon) {
			return false
		}
	}
	for i, a := range polygon {
		if a.X > r.L && a.X < r.R && a.Y > r.T && a.Y < r.B {
			return false
		}
		b := polygon[(i+1)%len(polygon)]
		for j, c := range corners {
			if segmentsCross(a, b, c, corners[(j+1)%len(corners)]) {
				return false
			}
		}
	}
	return true
}

// onPolygon reports p on an edge of the polygon, which the loop's area
// holds as a path's fill does.
func onPolygon(p study.Pt, polygon []study.Pt) bool {
	for i, a := range polygon {
		b := polygon[(i+1)%len(polygon)]
		ab, ap := b.Sub(a), p.Sub(a)
		if l := ab.Dist(); l > 0 && math.Abs(ab.Cross(ap))/l < 1e-6 && ap.Dot(ab) >= 0 && ap.Dot(ab) <= ab.Dist2() {
			return true
		}
	}
	return false
}

// segmentsCross reports segments ab and cd crossing at a point inside both.
func segmentsCross(a, b, c, d study.Pt) bool {
	side := func(p, q, r study.Pt) float64 { return (q.X-p.X)*(r.Y-p.Y) - (q.Y-p.Y)*(r.X-p.X) }
	d1, d2, d3, d4 := side(c, d, a), side(c, d, b), side(a, b, c), side(a, b, d)
	return d1*d2 < 0 && d3*d4 < 0
}

func (s *StudyCanvas) setRegionSelection(r study.Rect, polygon []study.Pt) {
	d := s.doc
	s.detail.Clear()
	if s.selectionKinds[study.SelectInk] {
		for id := range d.StrokesInPolygon(polygon) {
			s.detail.StrokeIDs[id] = true
		}
	}
	clear(s.selection)
	// A loop on a document selects ink and visual context. It only moves the
	// whole document if the loop encloses the card, so a PDF does not move
	// by accident.
	for _, c := range d.Cards {
		if c.Type == "conversation" || !d.Visible(c.Layer) || !s.selectionKinds[study.CardSelectionKind(c)] {
			continue
		}
		if !r.Contains(c.Rect.TopLeft()) || !(c.Rect.R <= r.R && c.Rect.B <= r.B) {
			continue
		}
		// A free lasso's bounding rectangle is not its enclosed area.
		if rectInPolygon(c.Rect, polygon) {
			s.selection[c.ID] = true
		}
	}
	// Selection handles and the attachment crop show the same area,
	// including when the region holds only part of a card or some ink.
	region := r
	s.selectionPolygon = append([]study.Pt(nil), polygon...)
	s.regionSelection, s.selectionBounds = rectPtr(region), rectPtr(region)
	var anchored *study.Card
	for _, c := range d.Cards {
		b := sourceBounds(c)
		if c.Type != "conversation" && s.selectionKinds[study.CardSelectionKind(c)] && d.Visible(c.Layer) &&
			b.Contains(region.Center()) && b.Overlaps(region) {
			anchored = c
		}
	}
	if anchored != nil && len(s.selection) == 0 {
		s.detail.ObjectID = anchored.ID
		s.detail.Region = rectPtr(normalizeRegion(region.Intersect(sourceBounds(anchored)), sourceBounds(anchored)))
	} else {
		s.detail.Region = rectPtr(region)
	}
	// A resized region is a different picture.
	s.quickSnapshot = nil
	s.selectionReady = true
}

func (s *StudyCanvas) boundsOfSelection() *study.Rect {
	d := s.doc
	var out *study.Rect
	add := func(r study.Rect) {
		if out == nil {
			out = rectPtr(r)
		} else {
			*out = out.Union(r)
		}
	}
	for _, st := range d.Strokes {
		if s.detail.StrokeIDs[st.ID] {
			add(st.Bounds())
		}
	}
	for _, c := range d.Cards {
		if s.selection[c.ID] {
			add(c.Rect)
		}
	}
	return out
}

func (s *StudyCanvas) transformSelection(from, to study.Rect, color *uint32, width *float64) {
	d := s.doc
	// Anchored ink moves with its selected parent exactly once.
	ink := map[string]bool{}
	for _, st := range d.Strokes {
		if s.detail.StrokeIDs[st.ID] && !s.selection[st.Anchor] {
			ink[st.ID] = true
		}
	}
	if len(s.selection) > 0 && color == nil && width == nil {
		d.TransformCards(s.selection, from, to)
	}
	if color != nil || width != nil {
		ink = s.detail.StrokeIDs
	}
	d.TransformInk(ink, from, to, color, width)
	if b := s.boundsOfSelection(); b != nil {
		s.selectionBounds = b
	} else {
		s.selectionBounds = rectPtr(to)
	}
	s.selectionPolygon = nil
	s.regionSelection, s.detail.Region = s.selectionBounds, s.selectionBounds
	s.detail.ObjectID = ""
}

func (s *StudyCanvas) copySelection() {
	d := s.doc
	s.copiedCards = nil
	parents := map[string]bool{}
	for _, c := range d.Cards {
		if s.selection[c.ID] && c.Type != "lecture" {
			s.copiedCards = append(s.copiedCards, c.Copy())
			parents[c.ID] = true
		}
	}
	s.copiedInk = nil
	for _, st := range d.Strokes {
		if s.detail.StrokeIDs[st.ID] || parents[st.Anchor] {
			s.copiedInk = append(s.copiedInk, st)
		}
	}
	s.copiedBounds = s.selectionBounds
}

func (s *StudyCanvas) pasteSelection() {
	d := s.doc
	if s.copiedBounds == nil || (len(s.copiedInk) == 0 && len(s.copiedCards) == 0) {
		return
	}
	points := 0
	for _, st := range d.Strokes {
		points += len(st.Points)
	}
	for _, st := range s.copiedInk {
		points += len(st.Points)
	}
	if len(d.Cards)+len(s.copiedCards) > 2000 || len(d.Strokes)+len(s.copiedInk) > 29000 || points > 490000 {
		s.toast("画布内容已接近容量上限，请先整理后再粘贴")
		return
	}
	if !d.Editable(d.ActiveLayer) {
		s.toast(l10n.StudyLayerUnavailable)
		return
	}
	d.Checkpoint()
	old := *s.copiedBounds
	view := s.visibleWorld()
	delta := study.Pt{X: 28 / d.Zoom, Y: 28 / d.Zoom}
	if !view.Contains(old.Center()) {
		delta = view.Center().Sub(old.Center())
	}
	cardIDs, gestureIDs := map[string]string{}, map[string]string{}
	for _, c := range s.copiedCards {
		cardIDs[c.ID] = study.NewID()
	}
	for _, st := range s.copiedInk {
		if _, ok := gestureIDs[st.Gesture]; !ok {
			gestureIDs[st.Gesture] = study.NewID()
		}
	}
	s.detail.Clear()
	clear(s.selection)
	for _, c := range s.copiedCards {
		cp := c.Copy()
		cp.ID = cardIDs[c.ID]
		cp.PDFColumn = cardIDs[c.PDFColumn]
		cp.Layer = d.ActiveLayer
		cp.Rect = cp.Rect.Shift(delta)
		d.Cards = append(d.Cards, cp)
		s.selection[cp.ID] = true
	}
	for _, st := range s.copiedInk {
		anchor := st.Anchor
		if id, ok := cardIDs[anchor]; ok {
			anchor = id
		}
		pts := make([]study.InkPoint, len(st.Points))
		for i, p := range st.Points {
			pts[i] = study.InkPoint{P: p.P.Add(delta), Pressure: p.Pressure}
		}
		cp := st.With(study.NewID(), pts)
		cp.Layer, cp.Gesture, cp.Anchor = d.ActiveLayer, gestureIDs[st.Gesture], anchor
		d.Strokes = append(d.Strokes, cp)
		s.detail.StrokeIDs[cp.ID] = true
	}
	s.tool = toolRegion
	s.selectionBounds = s.boundsOfSelection()
	s.selectionPolygon = nil
	s.regionSelection, s.detail.Region = s.selectionBounds, s.selectionBounds
	s.selectionReady = true
	d.Changed()
}

func (s *StudyCanvas) deleteSelection() {
	d := s.doc
	cards, ink := map[string]bool{}, map[string]bool{}
	for _, c := range d.Cards {
		if s.selection[c.ID] && c.Type != "lecture" && d.CanTransformCard(c) {
			cards[c.ID] = true
		}
	}
	for _, st := range d.Strokes {
		if d.Editable(st.Layer) && (s.detail.StrokeIDs[st.ID] || cards[st.Anchor]) {
			ink[st.ID] = true
		}
	}
	if len(cards) == 0 && len(ink) == 0 {
		return
	}
	d.Checkpoint()
	d.Cards = filterCards(d.Cards, func(c *study.Card) bool { return !cards[c.ID] })
	d.Strokes = filterStrokes(d.Strokes, func(st *study.Stroke) bool { return !ink[st.ID] })
	d.Connections = filterConnections(d.Connections, func(c study.Connection) bool { return !cards[c.From] && !cards[c.To] })
	clear(s.selection)
	s.detail.Clear()
	s.selectionReady, s.selectionBounds, s.regionSelection = false, nil, nil
	d.Changed()
}

func filterCards(l []*study.Card, keep func(*study.Card) bool) []*study.Card {
	var out []*study.Card
	for _, c := range l {
		if keep(c) {
			out = append(out, c)
		}
	}
	return out
}

func filterStrokes(l []*study.Stroke, keep func(*study.Stroke) bool) []*study.Stroke {
	var out []*study.Stroke
	for _, c := range l {
		if keep(c) {
			out = append(out, c)
		}
	}
	return out
}

func filterConnections(l []study.Connection, keep func(study.Connection) bool) []study.Connection {
	var out []study.Connection
	for _, c := range l {
		if keep(c) {
			out = append(out, c)
		}
	}
	return out
}

func (s *StudyCanvas) clearSelection() {
	s.selectionReady, s.selectionBounds, s.regionSelection, s.quickAsk = false, nil, nil, false
	clear(s.selection)
	s.detail.Clear()
}

// bound is _Bound: a source the quick composer will send.
type bound struct {
	id, label, detail string
	icon              icons.Icon
	// readable: 1 readable text is present, 0 needs 「看图」, -1 unknown.
	readable int
}

// boundSources is what the selection binds, one entry per source, as the
// composer names it.
func (s *StudyCanvas) boundSources() []bound {
	d := s.doc
	var out []bound
	for _, c := range d.Cards {
		if c.Type == "conversation" || !d.Visible(c.Layer) || !(s.selection[c.ID] || s.detail.ObjectID == c.ID) {
			continue
		}
		label := c.Title
		if label == "" {
			label = l10n.StudyBoundCard
			if c.Type == "lecture" {
				label = l10n.StudyBoundLecture
			}
		}
		var detail []string
		if c.Type == "pdfdoc" && c.Page > 0 {
			detail = append(detail, l10n.StudySourcePage(c.Page))
		}
		if s.detail.ObjectID == c.ID && s.detail.Region != nil {
			detail = append(detail, l10n.StudyBoundRegion)
		}
		ic := icons.NotesRounded
		switch c.Type {
		case "lecture", "video":
			ic = icons.SmartDisplayOutlined
		case "image":
			ic = icons.ImageOutlined
		case "pdf", "pdfdoc":
			ic = icons.DescriptionOutlined
		}
		var region *study.Rect
		if s.detail.ObjectID == c.ID {
			region = s.detail.Region
		}
		out = append(out, bound{id: c.ID, label: label, detail: strings.Join(detail, " · "), icon: ic, readable: readable(c, region)})
	}
	if len(s.detail.StrokeIDs) > 0 {
		out = append(out, bound{id: "selected-ink", label: l10n.StudyBoundInk(len(s.detail.StrokeIDs)), icon: icons.GestureRounded, readable: 0})
	}
	return out
}

// readable is whether the daemon will find words to read: a page region
// reads the words at least half inside it, as studyRegionSpan does; -1 when
// only a run can tell (a transcript).
func readable(c *study.Card, region *study.Rect) int {
	if c.Type == "lecture" || c.Type == "video" {
		return -1
	}
	b2i := func(b bool) int {
		if b {
			return 1
		}
		return 0
	}
	if region == nil {
		if c.Type == "image" || c.Type == "pdf" {
			return 0
		}
		return b2i(strings.TrimSpace(c.ContentText()) != "")
	}
	if c.Type != "pdfdoc" || c.QuarterTurns%4 != 0 {
		return 0
	}
	n := func(v any) float64 {
		f, ok := v.(float64)
		if !ok || math.IsNaN(f) || math.IsInf(f, 0) {
			return 0
		}
		return f
	}
	for _, raw := range c.TextBoxes {
		b, _ := raw.(map[string]any)
		box := study.RectLTWH(n(b["x"]), n(b["y"]), n(b["w"]), n(b["h"]))
		if box.Empty() || n(b["end"]) <= n(b["start"]) {
			continue
		}
		hit := box.Intersect(*region)
		if hit.W() > 0 && hit.H() > 0 && hit.W()*hit.H() >= box.W()*box.H()/2 {
			return 1
		}
	}
	return 0
}

func (s *StudyCanvas) unbind(id string) {
	if id == "selected-ink" {
		clear(s.detail.StrokeIDs)
	} else if s.detail.ObjectID == id {
		s.detail.ObjectID, s.detail.Start, s.detail.End = "", 0, 0
		s.detail.Region = s.regionSelection
	}
	delete(s.selection, id)
}

func (s *StudyCanvas) setQuickVision(on bool) {
	s.quickImages, s.quickSnapshot, s.quickError = on, nil, ""
	if on {
		s.previewQuickVision()
	}
}

// previewQuickVision takes the screenshot 「看图」 will send, so the learner
// sees it first.
func (s *StudyCanvas) previewQuickVision() {
	if s.quickCapturing {
		return
	}
	s.quickCapturing = true
	s.prepareSelectionSnapshot(func(shot *studySnapshot, err error) {
		s.quickCapturing = false
		if !s.quickImages {
			return
		}
		if err != nil {
			s.quickError = l10n.StudyQuickSendFailed(err.Error())
			return
		}
		s.quickSnapshot = shot
	})
}

// recordControl notes where a fixed control is, for the selection's
// toolbar to keep clear of it.
func (s *StudyCanvas) recordControl(e ui.Element) {
	if b := e.Bounds(); b.W > 0 {
		s.controls = append(s.controls, study.RectLTWH(float64(b.X)-s.origin.X, float64(b.Y)-s.origin.Y, float64(b.W), float64(b.H)))
	}
}

// measureSelectionControls takes the controls the last frame placed as the
// obstacles of the selection's toolbar.
func (s *StudyCanvas) measureSelectionControls() {
	s.selectionObstacles, s.controls = s.controls, nil
}

func (s *StudyCanvas) screenRect(r study.Rect) study.Rect {
	d := s.doc
	return study.RectPts(r.TopLeft().Mul(d.Zoom).Add(d.Camera), r.BottomRight().Mul(d.Zoom).Add(d.Camera))
}

func (s *StudyCanvas) selectionMenuRect() *study.Rect {
	if s.selectionBounds == nil {
		return nil
	}
	top := s.topBarsBottom() + 8
	return study.SelectionMenuRect(s.screenRect(*s.selectionBounds),
		study.Rect{L: 12, T: top, R: math.Max(12, s.viewport.X-12), B: math.Max(top, s.viewport.Y-12)}, s.selectionObstacles)
}

// floating places a control 8 DIPs off anchor's top-right corner, below it
// when the top is covered, and 12 inside the canvas (_FloatingLayout),
// after tldraw's contextual toolbar and the M3 floating toolbar guidance.
func (s *StudyCanvas) floating(c *ui.Context, key string, anchor study.Rect, build func()) {
	const gap, edge = 8.0, 12.0
	e := ui.Box(c.Key(key)).Absolute()
	b := e.Bounds()
	w, h := float64(b.W), float64(b.H)
	if w == 0 {
		e = e.Left(-10000).Top(0).Opacity(0)
		c.Invalidate()
	} else {
		top := s.topBarsBottom()
		area := study.Rect{L: edge, T: top + edge, R: s.viewport.X - edge, B: s.viewport.Y - edge}
		at, ok := study.FloatingPosition(anchor, area, s.selectionObstacles, w, h, gap)
		if !ok {
			x := clampF(anchor.R-w, edge, math.Max(edge, s.viewport.X-w-edge))
			t, bottom := top+edge, s.viewport.Y-edge
			y := anchor.T - gap - h
			if y < t {
				y = anchor.B + gap
			}
			// Neither side has room: stay inside the window over the
			// selection.
			if y+h > bottom {
				y = clampF(anchor.T-gap-h, t, math.Max(t, bottom-h))
			}
			at = study.Pt{X: x, Y: y}
		}
		e = e.Left(f32(at.X)).Top(f32(at.Y))
	}
	appear(e).Children(build)
}

// appear is _Appear: a short scale and fade as a floating control appears.
func appear(e ui.Element) ui.Element {
	return e.Transition(ui.ElementTransition{Duration: design.Short4, Ease: design.EmphasizedDecelerate,
		Enter: &ui.Motion{Opacity: 0, Scale: .96}})
}

func (s *StudyCanvas) selectionToolbar(c *ui.Context, docked bool) {
	if s.tool != toolRegion || !s.selectionReady || s.selectionBounds == nil || s.selectionDragging {
		return
	}
	build := func() {
		if s.quickAsk {
			s.quickComposer(c)
		} else {
			s.selectionBar(c)
		}
	}
	key := "study-selection-menu"
	if s.quickAsk {
		key = "study-selection-composer"
	}
	// No room beside the selection: the bar waits below the canvas.
	if docked {
		ui.Box(c).Fill().Center().Children(func() { appear(ui.Box(c.Key(key))).Children(build) })
		return
	}
	s.floating(c, key, s.screenRect(*s.selectionBounds), build)
}

// barAction is _BarAction: a selection-bar verb; the primary one is
// filled.
func barAction(c *ui.Context, label string, ic icons.Icon, tooltip string, primary, compact, disabled bool) bool {
	var e ui.Element
	switch {
	case primary:
		e = m3.Button{Kind: m3.Tonal, Label: label, Icon: ic, Height: 44, Disabled: disabled}.Show(c)
	case compact:
		e = m3.IconButton{Icon: ic, Size: 18, Box: 44, Disabled: disabled}.Show(c)
	default:
		e = m3.Button{Kind: m3.TextKind, Label: label, Icon: ic, Height: 44, Disabled: disabled}.Show(c)
	}
	return m3.Tip(c, e, tooltip).Clicked() && !disabled
}

// selectionBar is three verbs for a selection, with editing in a menu.
func (s *StudyCanvas) selectionBar(c *ui.Context) {
	d, r := s.doc, *s.selectionBounds
	ink := len(s.detail.StrokeIDs) > 0
	// On a phone the secondary verbs keep their tooltips and drop their
	// labels; anything still too wide scrolls inside the bar.
	compact := s.viewport.X < 520
	e := ui.Box(c.Key("study-selection-bar")).Padding(design.SpaceXXS).MaxWidth(f32(math.Max(0, s.viewport.X-24)))
	widgets.Glass{Radius: design.CornerFull, Depth: design.GlassFloat}.Apply(e)
	e.Children(func() {
		ui.ScrollHorizontal(c).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				if barAction(c, l10n.StudyAskAI, icons.AutoAwesomeOutlined, l10n.StudyAskHint, true, false, false) {
					s.ask()
				}
				if barAction(c, l10n.StudyExcerpt, icons.CropRounded, l10n.StudyExcerptHint, false, compact, s.selectionBusy) {
					s.captureRegionCard()
				}
				label, ic, tip := l10n.StudyRelate, icons.LinkRounded, l10n.StudyRelateHint
				if s.linkStart != nil {
					label, ic, tip = l10n.StudyRelateHere, icons.AddLinkRounded, l10n.StudyRelateHereHint
				}
				if barAction(c, label, ic, tip, false, compact, s.selectionBusy) {
					s.selectionJob(s.linkSelection)
				}
				// Drag into the conversation, or tap, to attach the region's
				// pixels.
				s.snapshotDragHandle(c)
				if ink {
					m3.MenuButton(c, "study-selection-color", icons.PaletteOutlined, "选区改色", m3.Below, func() []m3.MenuItem {
						var items []m3.MenuItem
						for _, col := range penPalette[:20] {
							col := col
							items = append(items, m3.MenuItem{Label: fmt.Sprintf("#%06x", col&0xffffff), LeadingFn: func(c *ui.Context) {
								m3.Icon(c, icons.Circle, 24, design.ARGB(col))
							}, OnTap: func() {
								d.Checkpoint()
								s.transformSelection(r, r, &col, nil)
							}})
						}
						return items
					})
					m3.MenuButton(c, "study-selection-width", icons.LineWeight, "选区粗细", m3.Below, func() []m3.MenuItem {
						var items []m3.MenuItem
						for _, w := range []float64{1, 2, 3, 5, 8, 12} {
							w := w
							items = append(items, m3.MenuItem{Label: fmt.Sprint(int(w)), OnTap: func() {
								d.Checkpoint()
								s.transformSelection(r, r, nil, &w)
							}})
						}
						return items
					})
				}
				s.selectionActions(c)
			})
		})
	})
}

// quickComposer is 「问 AI」 in place: what is bound, the question, and
// whether pixels go too.
func (s *StudyCanvas) quickComposer(c *ui.Context) {
	cs, tt := design.S(), design.TT()
	bounds := s.boundSources()
	unreadable := len(bounds) > 0
	for _, b := range bounds {
		if b.readable != 0 {
			unreadable = false
		}
	}
	d := s.doc
	var more []*study.Card
	for _, c := range d.Cards {
		if c.Type != "conversation" && d.Visible(c.Layer) && !s.selection[c.ID] && s.detail.ObjectID != c.ID {
			more = append(more, c)
		}
	}
	// Nearby material first.
	centre := s.selectionBounds.Center()
	sort.SliceStable(more, func(i, j int) bool {
		return more[i].Rect.Center().Sub(centre).Dist() < more[j].Rect.Center().Sub(centre).Dist()
	})
	e := ui.Column(c.Key("study-quick-composer")).Width(f32(math.Min(380, s.viewport.X-24))).
		Padding(design.SpaceSM, design.SpaceSM, design.SpaceSM, design.SpaceMD).AlignItems(ui.Stretch)
	widgets.Glass{Radius: design.CornerLG, Depth: design.GlassFloat}.Apply(e)
	e.Children(func() {
		ui.Row(c).Wrap().Gap(design.SpaceXS).AlignItems(ui.Center).Children(func() {
			for _, b := range bounds {
				if boundChip(c, b, s.quickSending) {
					s.unbind(b.id)
				}
			}
			if len(more) > 0 {
				m3.Menu(c, "study-bind-more", m3.Below, func(t ui.Element, open bool) {
					t.Children(func() {
						m3.IconButton{Icon: icons.AddRounded, Size: 18, Box: 36, Tooltip: l10n.StudyBindMore, Disabled: s.quickSending}.Show(c)
					})
				}, func() []m3.MenuItem {
					var items []m3.MenuItem
					for _, card := range more[:min(12, len(more))] {
						id, label := card.ID, card.Title
						if label == "" {
							label = l10n.StudyBoundCard
						}
						items = append(items, m3.MenuItem{Label: label, OnTap: func() { s.selection[id] = true }})
					}
					return items
				})
			}
		})
		ui.Row(c).AlignItems(ui.End).Children(func() {
			ui.Box(c).Grow(1).Basis(0).Children(func() {
				in := m3.Field{Hint: l10n.StudyQuickHint, MaxLength: 4000, Borderless: true, AutoFocus: true, Pad: true, PadX: 0, PadY: design.SpaceMD}.
					Show(c, &s.quickInput).Key("study-quick-question")
				if in.Submitted() {
					s.sendQuickQuestion(false)
				}
			})
			if (m3.IconButton{Kind: m3.FilledIcon, Icon: icons.ArrowUpwardRounded, Tooltip: "发送问题", Disabled: s.quickSending}).Show(c).Clicked() {
				s.sendQuickQuestion(false)
			}
		})
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			avatar := icons.VisibilityOffOutlined
			if s.quickImages {
				avatar = icons.VisibilityOutlined
			}
			if (m3.Chip{Kind: m3.FilterChip, Label: l10n.StudyAllowImages, Tooltip: l10n.StudyAllowImagesHint, Avatar: avatar,
				Selected: s.quickImages, NoCheck: true, Compact: true, Disabled: s.quickSending}).Show(c).Clicked() {
				s.setQuickVision(!s.quickImages)
			}
			ui.Box(c).Width(design.SpaceSM)
			if s.quickImages && (s.quickCapturing || s.quickSnapshot != nil) {
				// The exact pixels that will be sent, lit while taken.
				readingGlow(c, s.quickCapturing, design.CornerXS, func() {
					if s.quickSnapshot == nil {
						ui.Box(c).Size(48, 36).Center().Children(func() { m3.Icon(c, icons.CropRounded, 18, cs.Outline) })
						return
					}
					ui.Image(c, s.quickSnapshot.bitmap()).Height(36).Fit(ui.Contain).Radius(design.CornerXS).
						Border(1, cs.OutlineVariant).Key("study-quick-shot")
				})
			}
			ui.Spacer(c)
			m3.Text(c, l10n.StudyQuickKeys, tt.LabelSmall.C(cs.Outline)).MaxLines(1).Shrink(1)
			// The chips above are the material it reads; 「看图」 is the
			// consent.
			if (m3.Button{Kind: m3.TextKind, Icon: icons.FormatListBulletedRounded, Label: l10n.OutlineQuick, Disabled: s.quickSending, Height: 32}).
				Show(c).Key("study-quick-outline").Clicked() {
				s.sendQuickQuestion(true)
			}
		})
		if unreadable && !s.quickImages {
			m3.Text(c, l10n.StudyQuickNeedsVision, tt.BodySmall.C(cs.OnSurfaceVariant)).Margin(design.SpaceXS, 0, 0, 0)
		}
		if s.quickError != "" {
			m3.Text(c, s.quickError, tt.BodySmall.C(cs.Error)).Margin(design.SpaceXS, 0, 0, 0)
		}
	})
}

// boundChip is _BoundChip; it reports its remove button pressed.
func boundChip(c *ui.Context, b bound, busy bool) bool {
	cs, tt := design.S(), design.TT()
	removed := false
	ui.Row(c.Key("study-bound-" + b.id)).Height(30).PaddingStart(design.SpaceSM).AlignItems(ui.Center).
		Background(cs.OnSurface.Alpha(.06)).Radius(design.CornerSM).Children(func() {
		m3.Icon(c, b.icon, 14, cs.OnSurfaceVariant)
		ui.Box(c).Width(design.SpaceXS)
		m3.Text(c, b.label, tt.LabelMedium.C(cs.OnSurface)).MaxLines(1).MaxWidth(150)
		if b.detail != "" {
			ui.Box(c).Width(design.SpaceXS)
			m3.Text(c, b.detail, tt.LabelSmall.C(cs.OnSurfaceVariant))
		}
		if b.readable >= 0 {
			ui.Box(c).Width(design.SpaceXS)
			ic, col, tip := icons.ImageSearchRounded, cs.Tertiary, l10n.StudyBoundNeedsVision
			if b.readable == 1 {
				ic, col, tip = icons.ShortTextRounded, cs.Primary, l10n.StudyBoundReadable
			}
			m3.Tip(c, m3.Icon(c, ic, 14, col), tip)
		}
		removed = (m3.IconButton{Icon: icons.CloseRounded, Size: 14, Box: 30, Tooltip: l10n.StudyBoundRemove, Disabled: busy}).Show(c).Clicked()
	})
	return removed && !busy
}

// paintFeedback is _InteractionFeedback: the lasso or region, its handles,
// and the eraser.
func (s *StudyCanvas) paintFeedback(p *ui.Painter, r ui.Rect) {
	d := s.doc
	ch := studyChrome()
	screen := func(q study.Pt) (float32, float32) {
		return r.X + f32(q.X*d.Zoom+d.Camera.X), r.Y + f32(q.Y*d.Zoom+d.Camera.Y)
	}
	dash := func(path *ui.Path) {
		path.Dash(0, 6, 4)
		p.StrokePath(path, 1.5, ch.accent)
	}
	loop := s.lasso
	if s.erasing {
		loop = s.eraserLasso
	}
	if len(loop) > 1 && !(s.lassoRectangle && !s.erasing) {
		path := &ui.Path{}
		for i, q := range loop {
			x, y := screen(q)
			if i == 0 {
				path.MoveTo(x, y)
			} else {
				path.LineTo(x, y)
			}
		}
		path.Close()
		p.FillPath(path, ch.accentWash())
		outline := &ui.Path{}
		for i, q := range loop {
			x, y := screen(q)
			if i == 0 {
				outline.MoveTo(x, y)
			} else {
				outline.LineTo(x, y)
			}
		}
		outline.Close()
		dash(outline)
	} else {
		box := s.marquee
		if box == nil && s.selectionReady {
			box = s.selectionBounds
		}
		if box == nil {
			box = s.sourceHighlight
		}
		if box != nil {
			x0, y0 := screen(box.TopLeft())
			x1, y1 := screen(box.BottomRight())
			path := &ui.Path{}
			path.MoveTo(x0, y0)
			path.LineTo(x1, y0)
			path.LineTo(x1, y1)
			path.LineTo(x0, y1)
			path.Close()
			dash(path)
			if s.selectionReady {
				for _, q := range study.HandlePoints(study.Rect{L: float64(x0), T: float64(y0), R: float64(x1), B: float64(y1)}) {
					hr := ui.Rect{X: f32(q.X) - 5, Y: f32(q.Y) - 5, W: 10, H: 10}
					p.Fill(hr, ch.paper, 5)
					p.Stroke(hr, ch.accent, 5, 1.5)
				}
			}
		}
	}
	if cur := s.eraserCursor; cur != nil && (s.tool == toolEraser || s.temporaryEraser) && s.eraserMode != "lasso" {
		rad := f32(s.cursorRadius)
		cr := ui.Rect{X: r.X + f32(cur.X) - rad, Y: r.Y + f32(cur.Y) - rad, W: 2 * rad, H: 2 * rad}
		p.Fill(cr, ch.paper.Alpha(.24), rad)
		p.Stroke(cr, ch.paper, rad, 3)
		p.Stroke(cr, ch.inkSoft, rad, 1.3)
		if s.temporaryEraser {
			p.Fill(ui.Rect{X: cr.X + rad - 2, Y: cr.Y + rad - 2, W: 4, H: 4}, ch.accent, 2)
		}
	}
}

// holdCueView is _HoldCue: a ring that fills under a finger resting on a
// card, so the hold that lifts the card reads as deliberate rather than
// as a stall. It stays hidden through the first moments, when most
// contacts become pans.
func (s *StudyCanvas) holdCueView(c *ui.Context) {
	at := s.holdCue
	if at == nil || design.T().ReduceMotion {
		return
	}
	const size = 44
	cs := design.S()
	started := s.holdCueStart
	ui.Box(c).Absolute().Left(f32(at.X)-size/2).Top(f32(at.Y)-size/2).Size(size, size).IgnoreInput(true).
		Draw(func(p *ui.Painter, r ui.Rect) {
			// Silent for the first quarter of the hold.
			const quiet = .25
			v := float64(time.Since(started)) / float64(canvasCardHoldDelay)
			if v < 1 {
				p.AnimationFrame()
			}
			t := clampF((v-quiet)/(1-quiet), 0, 1)
			opacity := design.EmphasizedDecelerate(f32(clampF(t*4, 0, 1)))
			if opacity <= 0 {
				return
			}
			rad := r.W/2 - 3
			cx, cy := r.X+r.W/2, r.Y+r.H/2
			// A soft disc keeps the ring legible over any material.
			p.Fill(ui.Rect{X: cx - rad - 2, Y: cy - rad - 2, W: 2 * (rad + 2), H: 2 * (rad + 2)}, cs.Surface.Alpha(.55*opacity), rad+2)
			p.Stroke(ui.Rect{X: cx - rad, Y: cy - rad, W: 2 * rad, H: 2 * rad}, cs.Primary.Alpha(.18*opacity), rad, 3)
			fill := design.Emphasized(f32(t))
			if fill <= 0 {
				return
			}
			arc := &ui.Path{}
			arc.Circle(cx, cy, rad)
			arc.Trim(0, fill)
			arc.Cap(ui.RoundCap)
			p.Transform(ui.Rotation(-90), cx, cy, func() { p.StrokePath(arc, 3, cs.Primary.Alpha(opacity)) })
		})
}
