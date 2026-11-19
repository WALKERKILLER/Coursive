package app

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/study"
)

// The canvas's pointer routing (study_canvas.dart's _down, _move, _up and
// _scroll; study_stylus.dart; study_shape_interaction.dart): one handler
// sees every press on the canvas first and either takes it, for ink,
// selection, panning and moving cards, or leaves it to the card under it,
// for its buttons, text and video controls.

// pointerID tells the pointers apart by kind; MyGo has one of each.
func pointerID(ev ui.InputEvent) int {
	switch ev.Pointer {
	case ui.PointerPen:
		return pointerPen
	case ui.PointerTouch:
		return pointerTouch
	}
	return pointerMouse
}

func (s *StudyCanvas) input(ev ui.InputEvent) bool {
	if s.doc == nil {
		return false
	}
	local := ptOf(ev.X, ev.Y)
	switch ev.Kind {
	case ui.InputPointerDown:
		return s.down(ev, local)
	case ui.InputPointerMove:
		if _, ok := s.pointers[pointerID(ev)]; !ok {
			s.hover(ev, local)
			return false
		}
		s.move(ev, local)
		return true
	case ui.InputPointerUp:
		if _, ok := s.pointers[pointerID(ev)]; !ok {
			s.trackStylusPresence(ev, false)
			return false
		}
		s.up(ev, local)
		return true
	case ui.InputScroll:
		return s.scroll(ev, local)
	case ui.InputZoom:
		return s.pinch(ev, local)
	}
	return false
}

func (s *StudyCanvas) hover(ev ui.InputEvent, local study.Pt) {
	s.trackStylusPresence(ev, true)
	if s.tool == toolEraser {
		s.updateEraserCursor(local)
	} else if s.eraserCursor != nil {
		s.eraserCursor = nil
	}
}

// touchBlocked keeps a palm from navigating while a pen writes.
func (s *StudyCanvas) touchBlocked() bool {
	return len(s.stylusPointers) > 0 || s.palmCooldown.active()
}

func (s *StudyCanvas) trackStylusPresence(ev ui.InputEvent, hover bool) {
	if ev.Pointer != ui.PointerPen {
		return
	}
	// Hover protects handwriting, but must not cancel deliberate navigation
	// or moving selected objects. Actual pen contact remains authoritative.
	if hover && !s.ink() && s.tool != toolEraser {
		return
	}
	switch ev.Kind {
	case ui.InputPointerDown:
		s.stylusPointers[pointerPen] = true
	case ui.InputPointerUp:
		delete(s.stylusPointers, pointerPen)
	}
	// Lift and cancel restart the grace period for a palm still resting
	// nearby.
	s.palmCooldown.stop()
	s.palmCooldown = studyAfter(time.Second, func() {})
	if len(s.touchPointers) == 0 {
		return
	}
	// A palm that touched first cannot resume navigation when the pen
	// lifts. Only a fresh down after the grace period may start another
	// gesture.
	for id := range s.touchPointers {
		delete(s.pointers, id)
	}
	clear(s.touchPointers)
	s.cancelCardHold()
	s.panCamera, s.pinchZoom, s.pinchWorld = nil, nil, nil
	s.dragCard, s.heldCardNeedsCheckpoint, s.resize = "", false, false
	s.start, s.last = nil, nil
	if len(s.pointers) == 0 {
		s.gesturing = false
	}
}

// pressureOf is how hard the pointer presses: a pen's own, a fixed .7 for
// the rest.
func pressureOf(ev ui.InputEvent) float64 {
	if ev.Pointer == ui.PointerPen {
		return clampF(float64(ev.Pressure), .05, 1)
	}
	return .7
}

func (s *StudyCanvas) strokeTool() string {
	if s.tool == toolHighlighter {
		return "highlighter"
	}
	return "pen"
}

func (s *StudyCanvas) anchorPage() int {
	if c := s.doc.Card(s.anchor); c != nil && c.Type == "pdfdoc" {
		return c.Page
	}
	return 0
}

// gestureStroke is a stroke of the gesture under way with points.
func (s *StudyCanvas) gestureStroke(points []study.InkPoint) *study.Stroke {
	b, id := s.gestureBrush, study.NewID()
	return &study.Stroke{ID: id, Gesture: id, Layer: s.doc.ActiveLayer, Points: points, Color: s.gestureColor,
		Width: s.gestureWidth, Brush: &b, Tool: s.strokeTool(), Anchor: s.anchor, Page: s.anchorPage()}
}

// hitCard is the topmost visible card at p that is no conversation.
func (s *StudyCanvas) hitCard(p study.Pt) *study.Card {
	d := s.doc
	for i := len(d.Cards) - 1; i >= 0; i-- {
		c := d.Cards[i]
		if c.Type != "conversation" && d.Visible(c.Layer) && c.Rect.Contains(p) {
			return c
		}
	}
	return nil
}

// onVideoSurface reports a press on a video card's picture, below its title
// and above its playback controls.
func (s *StudyCanvas) onVideoSurface(c *study.Card, p study.Pt) bool {
	if c == nil || (c.Type != "lecture" && c.Type != "video") || c.Collapsed {
		return false
	}
	bottom := c.Rect.B - 16 - 84
	if c.Type == "lecture" {
		bottom -= 48
	}
	return p.Y >= c.Rect.T+36 && p.Y < bottom
}

func (s *StudyCanvas) holdMaterial(local study.Pt, id int, card *study.Card) {
	d, tool := s.doc, s.tool
	if !d.CanTransformCard(card) {
		return
	}
	s.cardHoldOrigin = ptPtr(local)
	s.holdCue, s.holdCueStart = ptPtr(local), time.Now()
	s.cardHoldTimer = studyAfter(canvasCardHoldDelay, func() {
		s.cancelCardHold()
		_, held := s.pointers[id]
		if s.disposed || s.doc != d || s.tool != tool || s.erasing || s.shape != nil || s.pressureTimer.active() ||
			len(s.pointers) != 1 || !held || d.Card(card.ID) != card || !d.CanTransformCard(card) {
			return
		}
		// A touchdown hold selects an object; an endpoint hold after writing
		// belongs to shape assist (docs/decisions/0027-stylus-card-holds.md).
		if s.drawingPointer == id {
			s.drawingPointer = 0
			s.resetShape()
			s.pressureStart, s.pressureOrigin = nil, nil
			s.points, s.prefix, s.prefixEnd = nil, nil, 0
		}
		// A completed card hold takes over from selectable text and editors.
		s.cardTextEpoch++
		s.panCamera = nil
		s.detail.Clear()
		s.regionSelection, s.selectionReady, s.selectionBounds = nil, false, nil
		clear(s.selection)
		s.selection[card.ID] = true
		s.dragCard, s.resize = card.ID, false
		s.last = ptPtr(s.world(s.pointers[id]))
		s.heldCardNeedsCheckpoint = true
	})
}

// headerActionAt reports a press on a card's header buttons.
func (s *StudyCanvas) headerActionAt(card *study.Card, local study.Pt) bool {
	r, ok := s.headerActions[card.ID]
	if !ok {
		return false
	}
	x, y := float32(s.origin.X+local.X), float32(s.origin.Y+local.Y)
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

func (s *StudyCanvas) down(ev ui.InputEvent, local study.Pt) bool {
	id := pointerID(ev)
	touch, stylus := ev.Pointer == ui.PointerTouch, ev.Pointer == ui.PointerPen
	mouse := !touch && !stylus
	middle := ev.Button == 2
	if ev.Button == 1 {
		return false
	}
	s.trackStylusPresence(ev, false)
	if touch && s.touchBlocked() {
		return false
	}
	s.cancelCardHold()
	s.stopFly()
	d := s.doc
	hitPoint := s.world(local)
	hit := s.hitCard(hitPoint)
	if s.onVideoSurface(hit, hitPoint) {
		s.videoSurfacePointers[id] = true
	}
	// Static material bodies are part of the pan surface. Keep their title
	// actions and PDF paging controls interactive; ink still uses the pen.
	materialBody := false
	if hit != nil && (hit.Type == "pdfdoc" || hit.Type == "pdf" || hit.Type == "image") {
		bottom := hit.Rect.B
		if hit.Type == "pdfdoc" {
			bottom -= 36
		}
		materialBody = hitPoint.Y >= hit.Rect.T+36 && hitPoint.Y < bottom
	}
	isVideo := hit != nil && (hit.Type == "lecture" || hit.Type == "video")
	// Text cards reach the same touch hold router as media cards. Mouse text
	// selection and embedded controls keep their own behaviour.
	selectedCard := (s.tool == toolSelect || (s.ink() && !mouse)) &&
		!(s.ink() && s.pressureEraser && stylus && pressureOf(ev) >= s.pressureThreshold) &&
		!middle && hit != nil && s.selection[hit.ID] && s.detail.ObjectID == "" &&
		(hitPoint.Y < hit.Rect.T+36 || s.writing() || !isVideo || s.videoSurfacePointers[id]) &&
		(hit.Type != "pdfdoc" || hitPoint.Y < hit.Rect.B-36) && d.CanTransformCard(hit)
	directTextBody := (touch || (stylus && s.tool == toolSelect)) && hit != nil &&
		(hit.Type == "note" || hit.Type == "text" || hit.Type == "explanation" || hit.Type == "knowledge" || hit.Type == "practice")
	selectedRegion := s.tool == toolRegion && s.selectionReady && s.selectionBounds != nil &&
		s.selectionBounds.Inflate(16/d.Zoom).Contains(hitPoint) &&
		(len(s.selection) > 0 || len(s.detail.StrokeIDs) > 0 || study.HandleAt(*s.selectionBounds, hitPoint, 16/d.Zoom) >= 0)
	passiveVideoBody := isVideo && (s.writing() || s.tool == toolHand || s.videoSurfacePointers[id]) &&
		hitPoint.Y >= hit.Rect.T+36 && hitPoint.Y < hit.Rect.B-20
	if hit != nil && s.tool != toolHand && !middle {
		resizeHandle := mouse && !hit.Collapsed && hitPoint.Sub(hit.Rect.BottomRight()).Dist() < 28/d.Zoom
		content := hitPoint.Y >= hit.Rect.T+36 && !resizeHandle
		if s.headerActionAt(hit, local) ||
			(content && !materialBody && !directTextBody && !selectedRegion && !selectedCard && !passiveVideoBody && !s.writing()) {
			delete(s.videoSurfacePointers, id)
			return false
		}
	}
	if s.erasing || s.selectionDragging || len(s.lasso) > 0 {
		return true
	}
	if s.drawingPointer != 0 && id != s.drawingPointer {
		return true
	}
	if stylus {
		clear(s.pointers)
		s.panCamera = nil
	}
	s.pointers[id] = local
	if touch {
		s.touchPointers[id] = true
	}
	if len(s.pointers) > 1 || s.drawingPointer != 0 {
		return true
	}
	p := s.world(local)
	s.start, s.last = ptPtr(p), ptPtr(p)
	if s.tool == toolEraser && !touch {
		s.temporaryEraser = false
		s.beginErase(local, p)
		return true
	}
	if s.tool == toolRegion && (!touch || selectedRegion) {
		handle := study.SelectionHandle(-1)
		if s.selectionReady && s.selectionBounds != nil {
			handle = study.HandleAt(*s.selectionBounds, p, 16/d.Zoom)
		}
		if handle >= 0 {
			s.selectionHandle = int(handle)
			s.selectionResizeStart = s.selectionBounds
			s.selectionResizePolygon = append([]study.Pt(nil), s.selectionPolygon...)
			s.selectionDragging = true
			return true
		}
		if s.selectionReady && s.selectionBounds != nil && s.selectionBounds.Contains(p) &&
			(len(s.selection) > 0 || len(s.detail.StrokeIDs) > 0) {
			d.Checkpoint()
			s.selectionHandle = -1
			s.selectionDragging = true
			return true
		}
		s.selectionReady, s.selectionBounds = false, nil
		s.lasso = []study.Pt{p}
		s.detail.Clear()
		s.regionSelection = nil
		s.marquee = rectPtr(study.RectPts(p, p))
		clear(s.selection)
		return true
	}
	// Selection survives the pointer going up. Grabbing it again moves the
	// object, not the camera.
	if selectedCard {
		s.panCamera = nil
		s.dragCard = hit.ID
		s.resize = mouse && !hit.Collapsed && p.Sub(hit.Rect.BottomRight()).Dist() < 28/d.Zoom
		s.heldCardNeedsCheckpoint = true
		s.cardTextEpoch++
		return true
	}
	if touch && hit == nil {
		clear(s.selection)
		s.detail.Clear()
		s.selectionReady, s.selectionBounds = false, nil
	}
	if stylus && s.tool == toolSelect && hit != nil {
		s.holdMaterial(local, id, hit)
		return true
	}
	if touch || s.tool == toolHand || ((materialBody || passiveVideoBody) && s.tool == toolSelect) || middle {
		s.panCamera = ptPtr(d.Camera)
		s.start = ptPtr(local)
		touchCard := touch && hit != nil
		if (touchCard || materialBody || passiveVideoBody) && !stylus && !middle && (touch || s.tool == toolSelect) {
			s.holdMaterial(local, id, hit)
		}
		return true
	}
	if s.ink() {
		if !d.Editable(d.ActiveLayer) {
			s.toast(l10n.StudyLayerUnavailable)
			return true
		}
		total := 0
		for _, st := range d.Strokes {
			total += len(st.Points)
		}
		if len(d.Strokes) >= 29000 || total >= 490000 {
			s.toast(l10n.StudyInkLimit)
			return true
		}
		s.pointBudget = min(20000, 499900-total)
		s.drawingPointer = id
		s.pressureStart = ptPtr(local)
		s.resetShape()
		s.gestureBrush, s.gestureWidth, s.gestureColor = s.brush, s.penWidth, s.color
		s.points = []study.InkPoint{{P: p, Pressure: pressureOf(ev)}}
		s.prefix, s.prefixEnd = nil, 0
		s.anchor = ""
		for i := len(d.Cards) - 1; i >= 0; i-- {
			if c := d.Cards[i]; d.Visible(c.Layer) && c.Rect.Contains(p) {
				s.anchor = c.ID
				break
			}
		}
		if stylus && hit != nil {
			s.holdMaterial(local, id, hit)
		}
		s.watchPressure(ev, local)
		s.watchShape(local)
		return true
	}
	if hit != nil {
		if !s.selection[hit.ID] {
			s.detail.Clear()
			s.regionSelection = nil
			clear(s.selection)
			s.selection[hit.ID] = true
		}
		corner := !hit.Collapsed && p.Sub(hit.Rect.BottomRight()).Dist() < 28/d.Zoom
		if !stylus && d.CanTransformCard(hit) && (p.Y < hit.Rect.T+38 || corner) {
			d.Checkpoint()
			s.dragCard = hit.ID
			s.resize = corner
		}
	} else {
		clear(s.selection)
		s.detail.Clear()
		s.regionSelection = nil
		s.selectionReady, s.selectionBounds = false, nil
		s.panCamera = ptPtr(d.Camera)
		s.start = ptPtr(local)
	}
	return true
}

func (s *StudyCanvas) move(ev ui.InputEvent, local study.Pt) {
	id := pointerID(ev)
	d := s.doc
	if s.cardHoldOrigin != nil {
		pen := ev.Pointer == ui.PointerPen
		slop := 8.0
		if pen {
			slop = 6
		}
		if local.Sub(*s.cardHoldOrigin).Dist() > slop {
			s.cancelCardHold()
		} else if !pen {
			s.pointers[id] = local
			return
		}
	}
	s.pointers[id] = local
	s.gesturing = true
	if len(s.pointers) > 1 {
		return
	}
	p := s.world(local)
	if s.panCamera != nil {
		d.Camera = s.panCamera.Add(local).Sub(*s.start)
		s.cameraChanged()
		return
	}
	if s.drawingPointer == id {
		if s.shape != nil {
			s.adjustShape(p)
			return
		}
		s.watchPressure(ev, local)
		s.watchShape(local)
		previous := s.points[len(s.points)-1]
		delta := p.Sub(previous.P).Mul(1 - s.gestureBrush.Smoothing*.7)
		if delta.Dist() < .4/d.Zoom || len(s.points) >= s.pointBudget {
			return
		}
		steps := max(1, min(int(math.Ceil(delta.Dist()/4)), min(100, s.pointBudget-len(s.points))))
		pr := pressureOf(ev)
		for i := 1; i <= steps; i++ {
			t := float64(i) / float64(steps)
			s.points = append(s.points, study.InkPoint{P: previous.P.Add(delta.Mul(t)), Pressure: previous.Pressure + (pr-previous.Pressure)*t})
		}
		for len(s.points)-s.prefixEnd > 65 {
			s.prefix = append(s.prefix, s.gestureStroke(append([]study.InkPoint(nil), s.points[s.prefixEnd:s.prefixEnd+65]...)))
			s.prefixEnd += 64
		}
		return
	}
	if s.erasing {
		s.updateEraserCursor(local)
		if s.eraserMode == "lasso" {
			if len(s.eraserLasso) < 1024 && s.eraserLasso[len(s.eraserLasso)-1].Sub(p).Dist() > 3/d.Zoom {
				s.eraserLasso = append(s.eraserLasso, p)
			}
		} else {
			s.eraseAt(*s.last, p)
		}
		s.last = ptPtr(p)
		return
	}
	if s.selectionDragging && s.selectionBounds != nil {
		from := *s.selectionBounds
		delta := p.Sub(*s.last)
		if s.selectionHandle >= 0 {
			to := study.ResizeSelection(*s.selectionResizeStart, study.SelectionHandle(s.selectionHandle), p.Sub(*s.start), 24/d.Zoom)
			original := *s.selectionResizeStart
			var polygon []study.Pt
			if len(s.selectionResizePolygon) == 0 {
				polygon = []study.Pt{to.TopLeft(), to.TopRight(), to.BottomRight(), to.BottomLeft()}
			} else {
				for _, q := range s.selectionResizePolygon {
					polygon = append(polygon, study.Pt{X: to.L + (q.X-original.L)*to.W()/original.W(), Y: to.T + (q.Y-original.T)*to.H()/original.H()})
				}
			}
			s.setRegionSelection(to, polygon)
		} else {
			s.transformSelection(from, from.Shift(delta), nil, nil)
		}
		s.last = ptPtr(p)
		return
	}
	if s.dragCard != "" {
		s.moveHeldCard(p)
		return
	}
	if s.marquee != nil {
		if s.lassoRectangle {
			s.marquee = rectPtr(study.RectPts(*s.start, p))
		} else {
			s.marquee = rectPtr(s.marquee.Union(study.RectPts(p, p)))
		}
		if len(s.lasso) < 2048 && s.lasso[len(s.lasso)-1].Sub(p).Dist() > 2/d.Zoom {
			s.lasso = append(s.lasso, p)
		}
	}
}

func (s *StudyCanvas) moveHeldCard(p study.Pt) {
	d := s.doc
	c := d.Card(s.dragCard)
	if c == nil || !d.CanTransformCard(c) {
		return
	}
	old := c.Rect
	delta := p.Sub(*s.last)
	if delta == (study.Pt{}) {
		return
	}
	if s.heldCardNeedsCheckpoint {
		d.Checkpoint()
		s.heldCardNeedsCheckpoint = false
	}
	next := old.Shift(delta)
	ids := s.selection
	if s.resize {
		minW, minH := 280.0, 180.0
		if c.Type == "conversation" {
			minW, minH = 240, 160
		}
		next = study.RectLTWH(old.L, old.T, math.Max(minW, old.W()+delta.X), math.Max(minH, old.H()+delta.Y))
		ids = map[string]bool{c.ID: true}
	}
	d.TransformCards(ids, old, next)
	s.last = ptPtr(p)
}

func (s *StudyCanvas) up(ev ui.InputEvent, local study.Pt) {
	id := pointerID(ev)
	canceled := ev.Canceled
	s.trackStylusPresence(ev, false)
	d := s.doc
	s.cancelCardHold()
	delete(s.touchPointers, id)
	delete(s.videoSurfacePointers, id)
	if s.drawingPointer == id {
		s.cancelShapeHold()
	}
	s.pressureTimer.stop()
	s.pressureTimer = nil
	s.pressureOrigin, s.pressureStart = nil, nil
	delete(s.pointers, id)
	if len(s.pointers) == 0 {
		s.gesturing = false
	}
	if canceled && s.drawingPointer == id {
		s.resetShape()
		s.points, s.prefix, s.prefixEnd, s.drawingPointer = nil, nil, 0, 0
	}
	if s.drawingPointer == id {
		d.AddStroke(s.gestureStroke(append([]study.InkPoint(nil), s.points...)))
		s.commitShape()
		added := d.Strokes[len(d.Strokes)-1]
		s.detail.Clear()
		var region *study.Rect
		for _, st := range d.Strokes {
			if st.Gesture == added.Gesture {
				s.detail.StrokeIDs[st.ID] = true
				if region == nil {
					region = rectPtr(st.Bounds())
				} else {
					*region = region.Union(st.Bounds())
				}
			}
		}
		r := region.Inflate(16)
		s.selectionPolygon = nil
		s.regionSelection = rectPtr(r)
		clear(s.selection)
		if anchored := d.Card(s.anchor); anchored != nil {
			s.selection[anchored.ID] = true
			s.detail.ObjectID = anchored.ID
			r = r.Intersect(sourceBounds(anchored))
			s.detail.Region = rectPtr(normalizeRegion(r, sourceBounds(anchored)))
		} else {
			s.detail.Region = rectPtr(r)
		}
		s.points, s.prefix, s.prefixEnd, s.drawingPointer = nil, nil, 0, 0
		s.resetShape()
	}
	if len(s.pointers) > 0 {
		return
	}
	if s.erasing && s.eraserMode == "lasso" && !canceled {
		d.EraseLasso(s.eraserLasso)
		s.eraserLasso = nil
	}
	if s.tool == toolRegion && s.marquee != nil && !canceled {
		s.finishSelection()
	}
	if s.selectionHandle >= 0 && canceled && s.selectionResizeStart != nil {
		s.setRegionSelection(*s.selectionResizeStart, s.selectionResizePolygon)
	}
	// The composer's screenshot follows the resized region.
	if s.selectionHandle >= 0 && s.quickAsk && s.quickImages {
		s.previewQuickVision()
	}
	s.selectionHandle = -1
	s.selectionResizeStart, s.selectionResizePolygon = nil, nil
	s.selectionDragging, s.temporaryEraser = false, false
	s.eraserCursor = nil
	s.lasso, s.eraserLasso = nil, nil
	s.panCamera, s.start, s.last, s.marquee = nil, nil, nil, nil
	s.dragCard, s.heldCardNeedsCheckpoint, s.erasing = "", false, false
	s.pinchZoom, s.pinchWorld = nil, nil
}

// scrollTakesWheel reports a card under p that scrolls its own content.
func (s *StudyCanvas) scrollTakesWheel(p study.Pt) bool {
	c := s.hitCard(p)
	if c == nil || c.Collapsed || p.Y < c.Rect.T+36 {
		return false
	}
	switch c.Type {
	case "text", "note", "explanation", "knowledge", "practice":
		return true
	}
	return false
}

func (s *StudyCanvas) scroll(ev ui.InputEvent, local study.Pt) bool {
	if s.drawingPointer != 0 {
		return false
	}
	d := s.doc
	if ev.Mods&(ui.Ctrl|ui.Super) != 0 {
		s.stopFly()
		p := s.world(local)
		d.Zoom = clampF(d.Zoom*math.Exp(-float64(ev.DY)*.002), .05, 8)
		d.Camera = local.Sub(p.Mul(d.Zoom))
		s.cameraChanged()
		return true
	}
	if s.scrollTakesWheel(s.world(local)) {
		return false
	}
	s.stopFly()
	d.Camera = d.Camera.Sub(ptOf(ev.DX, ev.DY))
	s.cameraChanged()
	return true
}

// pinch is a touchpad's or touchscreen's pinch, as onPointerPanZoomUpdate
// and the two-finger branch of _move.
func (s *StudyCanvas) pinch(ev ui.InputEvent, local study.Pt) bool {
	if s.drawingPointer != 0 || s.erasing || ev.Scale <= 0 {
		return false
	}
	s.stopFly()
	s.cancelCardHold()
	s.dragCard, s.heldCardNeedsCheckpoint = "", false
	s.marquee, s.lasso, s.selectionDragging = nil, nil, false
	d := s.doc
	p := s.world(local)
	d.Zoom = clampF(d.Zoom*float64(ev.Scale), .05, 8)
	d.Camera = local.Sub(p.Mul(d.Zoom))
	s.pinchWorld = ptPtr(p)
	s.cameraChanged()
	return true
}

// Shapes: holding the pen still at the end of a stroke turns it into the
// shape it looks like; moving on then adjusts the shape.

func (s *StudyCanvas) cancelShapeHold() {
	s.shapeTimer.stop()
	s.shapeTimer = nil
	s.shapeHoldOrigin = nil
}

func (s *StudyCanvas) resetShape() {
	s.cancelShapeHold()
	s.shape, s.shapeHandle, s.shapePreview = nil, nil, nil
}

func (s *StudyCanvas) watchShape(local study.Pt) {
	if !s.presets[s.presetIndex].ShapeAssist || s.cardHoldTimer.active() || s.pressureTimer.active() || s.shape != nil || !s.ink() {
		s.cancelShapeHold()
		return
	}
	if s.shapeHoldOrigin != nil && local.Sub(*s.shapeHoldOrigin).Dist() <= 6 {
		return
	}
	s.cancelShapeHold()
	s.shapeHoldOrigin = ptPtr(local)
	pointer := s.drawingPointer
	s.shapeTimer = studyAfter(700*time.Millisecond, func() {
		s.shapeTimer = nil
		if s.disposed || s.drawingPointer != pointer || s.pressureTimer.active() || len(s.points) < 3 {
			return
		}
		pts := make([]study.Pt, len(s.points))
		pressure := 0.0
		for i, p := range s.points {
			pts[i] = p.P
			pressure += p.Pressure
		}
		shape := study.RecognizeShape(pts, s.doc.Zoom)
		at, ok := s.pointers[pointer]
		if shape == nil || !ok {
			return
		}
		s.shape = shape
		s.shapeHandle = ptPtr(s.world(at))
		pressure /= float64(len(s.points))
		s.shapePreview = nil
		for _, p := range shape.Points {
			s.shapePreview = append(s.shapePreview, study.InkPoint{P: p, Pressure: pressure})
		}
		s.prefix, s.prefixEnd = nil, 0
	})
}

func (s *StudyCanvas) adjustShape(point study.Pt) {
	// Ignore sensor jitter before the user deliberately adjusts the shape.
	points := s.shape.Points
	if point.Sub(*s.shapeHandle).Dist()*s.doc.Zoom > 6 {
		points = s.shape.Adjust(*s.shapeHandle, point)
	}
	pressure := s.shapePreview[0].Pressure
	s.shapePreview = nil
	for _, p := range points {
		s.shapePreview = append(s.shapePreview, study.InkPoint{P: p, Pressure: pressure})
	}
}

func (s *StudyCanvas) commitShape() {
	if s.shapePreview == nil {
		return
	}
	d := s.doc
	original := d.Strokes[len(d.Strokes)-1]
	// The first checkpoint restores the actual freehand input; a second undo
	// removes that stroke. No shape-only fields enter the sync format.
	d.Checkpoint()
	var kept []*study.Stroke
	for _, st := range d.Strokes {
		if st.Gesture != original.Gesture {
			kept = append(kept, st)
		}
	}
	shaped := original.With(original.Gesture, s.shapePreview)
	d.Strokes = append(kept, study.Chunks(shaped)...)
	d.Changed()
}

// The pressure eraser: pressing the pen down hard and holding it still
// where a stroke starts erases instead.

func (s *StudyCanvas) watchPressure(ev ui.InputEvent, local study.Pt) {
	if s.pressureStart != nil && local.Sub(*s.pressureStart).Dist() > 6 {
		s.pressureStart = nil
	}
	if s.pressureStart == nil || !s.pressureEraser || s.shape != nil || ev.Pointer != ui.PointerPen ||
		pressureOf(ev) < s.pressureThreshold {
		s.pressureTimer.stop()
		s.pressureTimer, s.pressureOrigin = nil, nil
		return
	}
	if s.pressureOrigin != nil && local.Sub(*s.pressureOrigin).Dist() <= 6 {
		return
	}
	s.pressureTimer.stop()
	s.pressureOrigin = ptPtr(local)
	s.pressureKeep = max(0, len(s.points)-1)
	pointer := s.drawingPointer
	s.pressureTimer = studyAfter(380*time.Millisecond, func() {
		s.pressureTimer = nil
		if s.disposed || s.drawingPointer != pointer || s.pressureOrigin == nil {
			return
		}
		at, ok := s.pointers[pointer]
		if !ok {
			return
		}
		d := s.doc
		s.cancelCardHold()
		s.resetShape()
		d.Checkpoint()
		// Keep writing before the hold; remove only the stationary tail.
		if s.pressureKeep > 1 {
			d.Strokes = append(d.Strokes, study.Chunks(s.gestureStroke(append([]study.InkPoint(nil), s.points[:s.pressureKeep]...)))...)
			d.Changed()
		}
		s.points, s.prefix, s.prefixEnd, s.drawingPointer = nil, nil, 0, 0
		s.temporaryEraser = true
		s.startErase(at, s.world(at))
	})
}
