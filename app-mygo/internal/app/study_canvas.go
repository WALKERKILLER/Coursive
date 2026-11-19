package app

import (
	"errors"
	"fmt"
	"math"
	"os"
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

// study_canvas.dart: a lecture's study canvas, an infinite page holding the
// lecture's video, its materials, notes, conversations and ink.

type studyTool int

const (
	toolSelect studyTool = iota
	toolHand
	toolPen
	toolHighlighter
	toolEraser
	toolRegion
)

// canvasCardHoldDelay is how long a finger rests on a card before it moves
// the card.
const canvasCardHoldDelay = 600 * time.Millisecond

// studyZoomInset is the zoom control's distance from the corner.
const studyZoomInset = design.SpaceMD

// Pointer ids: MyGo reports one pointer of each kind, and pinches as zoom
// events, so the kind tells the pointers apart.
const (
	pointerMouse = 1 + iota
	pointerPen
	pointerTouch
)

// studyTimer is a Timer of the canvas: fn runs on the UI thread unless
// stopped first.
type studyTimer struct{ done bool }

func studyAfter(d time.Duration, fn func()) *studyTimer {
	t := &studyTimer{}
	time.AfterFunc(d, func() {
		core.Post(func() {
			if !t.done {
				t.done = true
				fn()
			}
		})
	})
	return t
}

func (t *studyTimer) stop() {
	if t != nil {
		t.done = true
	}
}

func (t *studyTimer) active() bool { return t != nil && !t.done }

func f32(v float64) float32 { return float32(v) }

func ptOf(x, y float32) study.Pt { return study.Pt{X: float64(x), Y: float64(y)} }

func rectPtr(r study.Rect) *study.Rect { return &r }

func ptPtr(p study.Pt) *study.Pt { return &p }

// studyFly carries the view to a new place along one eased path; see flyTo.
type studyFly struct {
	start            time.Time
	z0, zoom         float64
	from, to, centre study.Pt
	doc              *study.Document
	duration         time.Duration
	active           bool
}

// StudyCanvas is StudyCanvasState.
type StudyCanvas struct {
	ctl              *LecturePlayer
	video, panel     func(c *ui.Context)
	onClassic        func()
	onBack           func()
	onAsk            func(string)
	fullscreen       bool
	doc              *study.Document
	loadErr, saveErr error

	// Enabled plugins and whether one offers a canvas tool, read in View.
	enabledPlugins map[string]bool
	hasCanvasTools bool

	tool                      studyTool
	color                     uint32
	penWidth                  float64
	brush, gestureBrush       study.Brush
	gestureWidth, eraserSize  float64
	gestureColor              uint32
	presetIndex               int
	presets                   []study.PenPreset
	eraserMode, paperStyle    string
	paperColor                uint32
	penPlacement, aiPlacement dockPlacement
	dockSmall                 bool
	eraserLasso, lasso        []study.Pt
	fly                       studyFly

	// holdCue is where a finger rests on a card while the hold to move it
	// runs.
	holdCue                          *study.Pt
	holdCueStart                     time.Time
	stageRect, selectionBounds       *study.Rect
	eraserCursor                     *study.Pt
	pressureOrigin, pressureStart    *study.Pt
	cursorRadius, pressureThreshold  float64
	pressureEraser, temporaryEraser  bool
	selectionReady                   bool
	selectionDragging                bool
	selectionToolbarDocked           bool
	selectionObstacles               []study.Rect
	selectionHandle                  int
	selectionResizeStart             *study.Rect
	selectionResizePolygon           []study.Pt
	lassoRectangle, captureLasso     bool
	selectionKinds                   map[study.SelectionKind]bool
	selectionBusy                    bool
	linkStart                        []study.Anchor
	linkStartBounds                  *study.Rect
	pressureTimer, shapeTimer        *studyTimer
	cardHoldTimer                    *studyTimer
	cardHoldOrigin                   *study.Pt
	heldCardNeedsCheckpoint          bool
	videoSurfacePointers             map[int]bool
	predictionEnabled, penButtons    bool
	shapeHoldOrigin, shapeHandle     *study.Pt
	shape                            *study.Shape
	shapePreview                     []study.InkPoint
	pressureKeep                     int
	copiedInk                        []*study.Stroke
	copiedCards                      []*study.Card
	copiedBounds                     *study.Rect
	selectionPolygon                 []study.Pt
	gesturing, stageOpen             bool
	quickAsk, quickSending           bool
	quickImages, quickCapturing      bool
	quickError, quickInput           string
	snapshotting, snapshotPreparing  bool
	status                           string
	importing, disposed              bool
	saveTimer                        *studyTimer
	stopSync                         func()
	syncCheck                        *time.Ticker
	remotePending, refreshing        bool
	remoteDocument                   study.M
	saving                           bool
	saveWaiters                      []func(bool)
	savedEdit                        int
	selection                        map[string]bool
	detail                           study.Selection
	regionSelection, sourceHighlight *study.Rect
	pointers                         map[int]study.Pt
	touchPointers, stylusPointers    map[int]bool
	palmCooldown                     *studyTimer
	points                           []study.InkPoint
	prefix                           []*study.Stroke
	prefixEnd                        int
	anchor                           string
	start, last, panCamera           *study.Pt
	marquee                          *study.Rect
	dragCard                         string
	cardTextEpoch                    int
	resize, erasing                  bool
	pinchZoom                        *float64
	pinchWorld                       *study.Pt
	drawingPointer                   int
	pointBudget                      int
	viewport                         study.Pt
	// origin is the canvas's top-left in the window, headerActions the
	// cards' header buttons there, from the last frame.
	origin        study.Pt
	headerActions map[string]ui.Rect
	studyParts
}

// NewStudyCanvas opens lecture ctl's canvas. video and panel build the
// player's view and side panel inside it.
func NewStudyCanvas(ctl *LecturePlayer, video, panel func(c *ui.Context), onClassic, onBack func(), onAsk func(string)) *StudyCanvas {
	s := &StudyCanvas{
		ctl: ctl, video: video, panel: panel, onClassic: onClassic, onBack: onBack, onAsk: onAsk,
		color: 0xff25344a, penWidth: 3, brush: study.DefaultBrush, gestureBrush: study.DefaultBrush,
		gestureWidth: 3, eraserSize: 18, gestureColor: 0xff25344a, presets: study.DefaultPresets(),
		eraserMode: "partial", paperStyle: "ruled", paperColor: 0xffffffff,
		penPlacement: dockPlacement{edge: dockBottom, offset: .5}, aiPlacement: dockPlacement{edge: dockRight, offset: .34},
		cursorRadius: 18, pressureThreshold: .8, pressureEraser: true, selectionHandle: -1, lassoRectangle: true,
		selectionKinds: map[study.SelectionKind]bool{}, videoSurfacePointers: map[int]bool{},
		predictionEnabled: true, penButtons: true, status: l10n.StudyOpening, selection: map[string]bool{},
		pointers: map[int]study.Pt{}, touchPointers: map[int]bool{}, stylusPointers: map[int]bool{},
		pointBudget: 20000, viewport: study.Pt{X: 1000, Y: 700}, headerActions: map[string]ui.Rect{},
	}
	for _, k := range study.SelectionKinds {
		s.selectionKinds[k] = true
	}
	s.detail.Clear()
	s.initParts()
	s.load(nil)
	s.syncCheck = time.NewTicker(5 * time.Second)
	tick := s.syncCheck.C
	go func() {
		for range tick {
			core.Post(func() {
				if !s.disposed && !m3.AnyRoute() {
					s.refreshFromSync()
				}
			})
		}
	}()
	s.stopSync = State.Events.Listen(func(e core.J) {
		if e.S("type") == "sync_done" && e.O("data").F("downloaded") > 0 {
			s.refreshFromSync()
		}
	})
	s.loadBrushSettings()
	s.loadLayout()
	return s
}

// Dispose stops the canvas's timers and listeners.
func (s *StudyCanvas) Dispose() {
	s.cancelCardHold()
	s.palmCooldown.stop()
	s.syncCheck.Stop()
	s.disposed = true
	s.saveTimer.stop()
	s.stopSync()
	s.pressureTimer.stop()
	s.shapeTimer.stop()
	s.disposeParts()
	if s.doc != nil {
		s.doc.OnChange = nil
	}
}

func (s *StudyCanvas) lectureID() string { return s.ctl.LectureID() }

func (s *StudyCanvas) draftKey() string { return "study-draft-" + s.lectureID() }

func (s *StudyCanvas) editingCanvas() bool {
	return s.importing || s.selectionBusy || (s.doc != nil && s.doc.Edit != s.savedEdit) || s.saving ||
		s.drawingPointer != 0 || s.erasing || s.dragCard != ""
}

func (s *StudyCanvas) wholeCardSelected() bool {
	return len(s.selection) > 0 && s.detail.ObjectID == ""
}

func (s *StudyCanvas) adopt(d *study.Document) {
	s.doc = d
	s.adoptThreads(d)
	d.OnChange = s.changed
}

func (s *StudyCanvas) load(synced study.M) {
	fetch := func() (any, error) {
		if synced != nil {
			return synced, nil
		}
		return State.Api().StudyCanvas(s.lectureID())
	}
	core.Async(fetch, func(data any, err error) {
		if s.disposed {
			return
		}
		if err != nil {
			s.loadErr = err
			return
		}
		m := plainMap(data)
		draft, hasDraft := core.Prefs.String(s.draftKey())
		if synced != nil && s.editingCanvas() {
			return
		}
		d, err := study.DocumentFrom(m)
		if err != nil {
			s.loadErr = err
			return
		}
		if hasDraft {
			local, derr := core.Decode([]byte(draft))
			lm := plainMap(local)
			ld, lerr := study.DocumentFrom(lm)
			if derr == nil && lerr == nil {
				s.remoteDocument = m
				s.remotePending = true
				s.adopt(ld)
				ld.Edit = 1
				s.savedEdit = 0
				s.status = "已恢复本机草稿，请选择保留版本"
				return
			}
		}
		if s.doc != nil {
			s.doc.OnChange = nil
		}
		s.adopt(d)
		s.savedEdit = d.Edit
		s.loadErr = nil
		s.status = l10n.StudySaved
	})
}

func (s *StudyCanvas) refreshFromSync() {
	if s.refreshing || s.disposed {
		return
	}
	s.refreshing = true
	core.Async(func() (any, error) { return State.Api().StudyCanvas(s.lectureID()) }, func(r any, err error) {
		s.refreshing = false
		remote := plainMap(r)
		d := s.doc
		if err != nil || d == nil || s.disposed || remote == nil {
			return
		}
		if h, _ := remote["base_hash"].(string); h == d.BaseHash {
			return
		}
		// Finish an edit already in progress. Its save rebases against the
		// latest revision, so the last saved whole canvas wins (ADR 0022).
		if s.editingCanvas() {
			return
		}
		s.load(remote)
	})
}

func (s *StudyCanvas) resolveRemote(keepLocal bool) {
	remote := s.remoteDocument
	if remote == nil {
		return
	}
	done := func() {
		if !s.remotePending {
			core.Prefs.Remove(s.draftKey())
		}
	}
	if keepLocal {
		s.doc.Revision = intOf(remote["revision"])
		s.doc.BaseHash, _ = remote["base_hash"].(string)
		s.remotePending = false
		s.flush(func(ok bool) {
			if !ok {
				s.remotePending = true
			}
			done()
		})
		return
	}
	s.remotePending = false
	core.Prefs.Remove(s.draftKey())
	s.load(nil)
	done()
}

// plainJSON is decoded JSON with its core.J objects as plain maps, as the
// study model reads them.
func plainJSON(v any) any {
	switch t := v.(type) {
	case core.J:
		return plainJSON(map[string]any(t))
	case map[string]any:
		m := make(map[string]any, len(t))
		for k, e := range t {
			m[k] = plainJSON(e)
		}
		return m
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = plainJSON(e)
		}
		return out
	}
	return v
}

func plainMap(v any) study.M {
	m, _ := plainJSON(v).(map[string]any)
	return m
}

func intOf(v any) int {
	f, _ := v.(float64)
	return int(f)
}

func (s *StudyCanvas) changed() {
	if s.disposed {
		return
	}
	if s.selectionReady {
		strokes, cards := map[string]bool{}, map[string]bool{}
		for _, st := range s.doc.Strokes {
			strokes[st.ID] = true
		}
		for _, c := range s.doc.Cards {
			cards[c.ID] = true
		}
		for id := range s.detail.StrokeIDs {
			if !strokes[id] {
				delete(s.detail.StrokeIDs, id)
			}
		}
		for id := range s.selection {
			if !cards[id] {
				delete(s.selection, id)
			}
		}
		s.selectionBounds = s.regionSelection
		if s.selectionBounds == nil {
			s.selectionBounds = s.boundsOfSelection()
		}
		if s.selectionBounds == nil && s.detail.ObjectID == "" {
			s.selectionReady = false
		}
	}
	s.saveTimer.stop()
	s.saveTimer = studyAfter(time.Second, s.saveWhenIdle)
	s.status = l10n.StudyPendingSave
}

func (s *StudyCanvas) saveWhenIdle() {
	if s.drawingPointer != 0 || s.erasing || s.dragCard != "" {
		s.saveTimer = studyAfter(time.Second, s.saveWhenIdle)
	} else {
		s.flush(nil)
	}
}

// flush saves the canvas now and reports whether it is saved.
func (s *StudyCanvas) flush(done func(bool)) {
	s.saveTimer.stop()
	if s.saving {
		// After the save under way, save again what changed since.
		s.saveWaiters = append(s.saveWaiters, func(ok bool) {
			if !ok {
				if done != nil {
					done(false)
				}
				return
			}
			s.flush(done)
		})
		return
	}
	d := s.doc
	if s.remotePending {
		if done != nil {
			done(false)
		}
		return
	}
	if d == nil || d.Edit == s.savedEdit {
		if done != nil {
			done(true)
		}
		return
	}
	s.saving = true
	s.saveLoop(d, 0, func(ok bool) {
		s.saving = false
		waiters := s.saveWaiters
		s.saveWaiters = nil
		if done != nil {
			done(ok)
		}
		for _, w := range waiters {
			w(ok)
		}
	})
}

func (s *StudyCanvas) saveLoop(d *study.Document, retries int, done func(bool)) {
	if d.Edit == s.savedEdit {
		s.saveErr = nil
		s.status = l10n.StudySaved
		done(true)
		return
	}
	fail := func(err error) {
		s.saveErr = err
		s.status = l10n.StudySaveFailed
		done(false)
	}
	edit := d.Edit
	s.status = l10n.StudySaving
	body := d.JSON()
	id := s.lectureID()
	core.Async(func() (any, error) { return State.Api().SaveStudyCanvas(id, body) }, func(out any, err error) {
		var ae *core.ApiError
		if errors.As(err, &ae) && ae.Status == 409 && retries < 2 {
			core.Async(func() (any, error) {
				remote, err := State.Api().StudyCanvas(id)
				if err != nil {
					return nil, err
				}
				// Retain the superseded saved document before replacing it.
				_, err = State.Api().Post("/api/lectures/"+core.Esc(id)+"/study-canvas/draft", remote)
				return remote, err
			}, func(r any, err error) {
				if err != nil {
					fail(err)
					return
				}
				remote := plainMap(r)
				version := intOf(remote["version"])
				if version > 5 {
					fail(errors.New(l10n.StudyUnknownVersion))
					return
				}
				d.FormatVersion = max(d.FormatVersion, version)
				d.Revision = intOf(remote["revision"])
				d.BaseHash, _ = remote["base_hash"].(string)
				s.saveLoop(d, retries+1, done)
			})
			return
		}
		if err != nil {
			fail(err)
			return
		}
		m := plainMap(out)
		d.Revision = intOf(m["revision"])
		d.BaseHash, _ = m["base_hash"].(string)
		s.savedEdit = edit
		s.saveLoop(d, retries, done)
	})
}

// ReturnFromSource is the system back action: it first returns from a
// followed citation.
func (s *StudyCanvas) ReturnFromSource() bool { return s.doc != nil && s.goBack() }

// Keys handles the canvas's shortcuts and reports whether one was used.
func (s *StudyCanvas) Keys(c *ui.Context) bool {
	if s.doc == nil || c.FocusTakesText() || m3.AnyRoute() {
		return false
	}
	key := func(m ui.Modifiers, k ui.Key) bool { return c.Shortcut(m, k) }
	// Esc steps back one level: composer, selection, then a followed
	// citation.
	if key(0, ui.KeyEscape) {
		if s.quickAsk {
			s.quickAsk = false
			return true
		}
		if s.selectionReady {
			s.clearSelection()
			return true
		}
		return s.goBack()
	}
	if key(ui.Alt, ui.KeyLeft) || key(ui.Super, ui.KeyBracketLeft) {
		return s.goBack()
	}
	if key(0, ui.KeyEnter) && s.tool == toolRegion && s.selectionReady && !s.quickAsk {
		s.ask()
		return true
	}
	if key(ui.Cmd|ui.Shift, ui.KeyZ) {
		s.doc.Redo()
		return true
	}
	if key(ui.Cmd, ui.KeyZ) {
		s.doc.Undo()
		return true
	}
	for k, t := range map[ui.Key]studyTool{ui.KeyV: toolSelect, ui.KeyH: toolHand, ui.KeyD: toolPen, ui.KeyE: toolEraser} {
		if key(0, k) && !c.ShortcutRepeated() {
			s.chooseTool(t)
			return true
		}
	}
	return false
}

// world is the canvas point under the local point p.
func (s *StudyCanvas) world(p study.Pt) study.Pt { return p.Sub(s.doc.Camera).Div(s.doc.Zoom) }

func (s *StudyCanvas) ink() bool { return s.tool == toolPen || s.tool == toolHighlighter }

func (s *StudyCanvas) writing() bool { return s.ink() || s.tool == toolEraser || s.tool == toolRegion }

func (s *StudyCanvas) cameraChanged() {
	s.saveTimer.stop()
	s.saveTimer = studyAfter(700*time.Millisecond, func() {
		if s.disposed || s.doc == nil {
			return
		}
		s.doc.Edit++
		s.flush(nil)
	})
}

func (s *StudyCanvas) cancelCardHold() {
	s.cardHoldTimer.stop()
	s.cardHoldTimer = nil
	s.cardHoldOrigin = nil
	s.holdCue = nil
}

// flyTo moves the view to camera at zoom along one eased path. The point at
// the centre of the window travels in a straight line while the zoom
// changes geometrically, so the way there stays readable. Any touch, wheel
// or zoom stops it where it is.
func (s *StudyCanvas) flyTo(camera study.Pt, zoom float64) {
	d := s.doc
	s.fly.active = false
	if design.T().ReduceMotion || (d.Camera.Sub(camera).Dist() < 1 && math.Abs(d.Zoom-zoom) < .001) {
		d.Zoom, d.Camera = zoom, camera
		s.cameraChanged()
		return
	}
	centre := s.viewport.Div(2)
	s.fly = studyFly{start: time.Now(), z0: d.Zoom, zoom: zoom, centre: centre, doc: d, duration: design.Long2, active: true,
		from: centre.Sub(d.Camera).Div(d.Zoom), to: centre.Sub(camera).Div(zoom)}
}

// stepFly moves a flight on to this frame.
func (s *StudyCanvas) stepFly(c *ui.Context) {
	f := &s.fly
	if !f.active {
		return
	}
	if f.doc != s.doc {
		f.active = false
		return
	}
	t := float64(time.Since(f.start)) / float64(f.duration)
	if t >= 1 {
		t, f.active = 1, false
	} else {
		c.AnimationFrame()
	}
	e := float64(design.Emphasized(float32(t)))
	d := s.doc
	d.Zoom = f.z0 * math.Pow(f.zoom/f.z0, e)
	d.Camera = f.centre.Sub(f.from.Lerp(f.to, e).Mul(d.Zoom))
	s.cameraChanged()
}

func (s *StudyCanvas) stopFly() { s.fly.active = false }

func (s *StudyCanvas) zoomBy(factor float64) {
	s.stopFly()
	d := s.doc
	centre := s.viewport.Div(2)
	p := s.world(centre)
	d.Zoom = clampF(d.Zoom*factor, .05, 8)
	d.Camera = centre.Sub(p.Mul(d.Zoom))
	s.cameraChanged()
}

func clampF(v, lo, hi float64) float64 { return math.Min(math.Max(v, lo), hi) }

func (s *StudyCanvas) fit() {
	d := s.doc
	var r *study.Rect
	for _, c := range d.Cards {
		if c.Type == "conversation" || !d.Visible(c.Layer) {
			continue
		}
		if r == nil {
			r = rectPtr(c.Rect)
		} else {
			*r = r.Union(c.Rect)
		}
	}
	if r == nil {
		r = rectPtr(study.RectLTWH(0, 0, 880, 550))
	}
	zoom := clampF(math.Min((s.viewport.X-100)/r.W(), (s.viewport.Y-100)/r.H()), .05, 1.5)
	s.flyTo(s.viewport.Div(2).Sub(r.Center().Mul(zoom)), zoom)
}

func (s *StudyCanvas) toast(msg string) { widgets.ShowSnack(msg, "", nil) }

func (s *StudyCanvas) expandPDFColumn(root *study.Card) {
	if s.importing {
		return
	}
	d := s.doc
	if err := d.BeginPDFColumn(root); err != nil {
		s.toast(err.Error())
		return
	}
	s.importing = true
	finish := func(err error) {
		s.importing = false
		if err != nil {
			s.toast(fmt.Sprintf("展开未完成：%v。已展开的页面和笔迹已保留，可点击“继续展开整列”重试", err))
		}
	}
	var next func(page int)
	alive := func() bool { return !s.disposed && s.doc == d && d.Card(root.ID) == root && root.PDFColumn == root.ID }
	next = func(page int) {
		for page <= root.PageCount && alive() && hasPDFPage(d, root.ID, page) {
			page++
		}
		if page > root.PageCount || !alive() {
			finish(nil)
			return
		}
		s.status = fmt.Sprintf("正在展开课件 %d / %d 页", page, root.PageCount)
		id, source := s.lectureID(), root.Source
		core.Async(func() (any, error) { return State.Api().StudyPDFPage(id, source, page) }, func(out any, err error) {
			if err != nil {
				finish(err)
				return
			}
			if !alive() {
				finish(nil)
				return
			}
			data := plainMap(out)
			if err := d.AppendPDFPage(root, page, data); err != nil {
				finish(err)
				return
			}
			s.flush(func(ok bool) {
				if !ok {
					finish(nil)
					return
				}
				next(page + 1)
			})
		})
	}
	s.flush(func(ok bool) {
		if !ok {
			finish(nil)
			return
		}
		next(1)
	})
}

func hasPDFPage(d *study.Document, root string, page int) bool {
	for _, c := range d.PDFPages(root) {
		if c.Page == page {
			return true
		}
	}
	return false
}

func (s *StudyCanvas) collapsePDFColumn(id string) {
	if err := s.doc.CollapsePDFColumn(id); err != nil {
		s.toast(err.Error())
		return
	}
	s.flush(nil)
}

var studyImportTypes = []string{"pdf", "png", "jpg", "jpeg", "gif", "webp", "txt", "md", "markdown", "mp4", "mov", "m4v", "webm", "mkv"}

// importMaterial adds a file, or with courseware a lecture file (all of
// the course's with wholeCourse), at the point at or the view's centre.
func (s *StudyCanvas) importMaterial(courseware string, wholeCourse bool, at *study.Pt) {
	if s.importing {
		return
	}
	run := func(path string) {
		d := s.doc
		if !d.Editable(d.ActiveLayer) {
			s.toast(l10n.StudyPickLayer)
			return
		}
		s.importing = true
		s.status = l10n.StudyImporting
		id := s.lectureID()
		core.Async(func() (any, error) {
			if courseware != "" {
				if wholeCourse {
					return State.Api().ImportStudyMaterial(id, courseware)
				}
				return State.Api().ImportStudyCourseware(id, courseware)
			}
			f, err := os.Open(path)
			if err != nil {
				return nil, err
			}
			defer f.Close()
			st, err := f.Stat()
			if err != nil {
				return nil, err
			}
			if st.Size() > 300<<20 {
				return nil, errors.New(l10n.StudyFileTooLarge)
			}
			return State.Api().ImportStudyAsset(id, st.Name(), f, st.Size())
		}, func(out any, err error) {
			finish := func() {
				s.importing = false
				if s.saveErr == nil {
					s.status = l10n.StudySaved
				}
			}
			if s.disposed {
				return
			}
			if err == nil {
				err = s.placeImported(out, at)
			}
			if err != nil {
				s.toast(l10n.StudyImportError(err.Error()))
				finish()
				return
			}
			s.flush(func(bool) { finish() })
		})
	}
	if courseware != "" {
		run("")
		return
	}
	core.PickFile(l10n.StudyMaterials, []core.FileFilter{{Name: l10n.StudyMaterials, Extensions: studyImportTypes}}, func(path string) {
		if path != "" && !s.disposed {
			run(path)
		}
	})
}

func (s *StudyCanvas) placeImported(out any, at *study.Pt) error {
	d := s.doc
	m := plainMap(out)
	var cards []*study.Card
	for _, a := range listOf(m["objects"]) {
		if o, ok := a.(study.M); ok {
			cards = append(cards, study.CardFrom(o))
		}
	}
	if len(cards) == 0 {
		return nil
	}
	if len(cards)+len(d.Cards) > 2000 {
		return errors.New(l10n.StudyCardLimit)
	}
	centre := s.world(s.viewport.Div(2))
	off := 120.0
	if at != nil {
		centre, off = *at, 60
	}
	d.Checkpoint()
	y := centre.Y - off
	for _, c := range cards {
		c.Layer = d.ActiveLayer
		c.Rect = study.RectLTWH(centre.X-160, y, c.Rect.W(), c.Rect.H())
		y += c.Rect.H() + 36
	}
	d.Cards = append(d.Cards, cards...)
	clear(s.selection)
	s.selection[cards[0].ID] = true
	d.Changed()
	s.tool = toolSelect
	return nil
}

func listOf(v any) []any {
	l, _ := v.([]any)
	return l
}

// editText adds a text card, or edits card's text.
func (s *StudyCanvas) editText(card *study.Card) {
	text := ""
	title := l10n.StudyAddText
	if card != nil {
		text, title = card.Text, l10n.StudyEditText
	}
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		m3.AlertDialog(c, m3.Alert{Title: title, Width: 500, Content: func() {
			m3.Field{Hint: l10n.StudyTextHint, MinLines: 5, MaxLines: 15, MaxLength: 200000, AutoFocus: true}.Show(c, &text)
		}, Actions: func() {
			if m3.TextButton(c, l10n.CommonCancel).Clicked() {
				r.Pop(nil)
			}
			if m3.FilledButton(c, l10n.CommonSave).Clicked() {
				r.Pop(text)
			}
		}})
	}, func(v any) {
		text, ok := v.(string)
		if !ok || s.disposed {
			return
		}
		d := s.doc
		layer := d.ActiveLayer
		if card != nil {
			layer = card.Layer
		}
		if !d.Editable(layer) {
			s.toast(l10n.StudyLocked)
			return
		}
		d.Checkpoint()
		if card != nil {
			card.Text = text
		} else {
			p := s.world(s.viewport.Div(2))
			c := &study.Card{ID: study.NewID(), Type: "text", Layer: d.ActiveLayer, Rect: study.RectLTWH(p.X-100, p.Y-100, 480, 360),
				Title: l10n.StudyTextCard, Text: text, Responses: map[string]string{}}
			d.Cards = append(d.Cards, c)
			clear(s.selection)
			s.selection[c.ID] = true
		}
		d.Changed()
	})
}

func (s *StudyCanvas) ask() {
	if s.selectionReady && s.selectionBounds != nil && s.tool == toolRegion {
		s.quickAsk = true
	} else {
		s.openConversation(false, true)
	}
}

// sendQuickQuestion sends the quick composer's question, or with outline
// drafts a review outline of the bound material, the question becoming its
// brief.
func (s *StudyCanvas) sendQuickQuestion(outline bool) {
	text := trimSpace(s.quickInput)
	if (text == "" && !outline) || s.quickSending {
		return
	}
	s.quickSending, s.quickError = true, ""
	if len(s.selection) == 0 && s.detail.ObjectID == "" && len(s.detail.StrokeIDs) == 0 {
		s.quickError = l10n.StudyQuickSendFailed(l10n.StudySelectionNeedsSource)
		s.quickSending = false
		return
	}
	send, task := text, ""
	if outline {
		task = "propose_outline"
		send = l10n.OutlineAsk
		if text != "" {
			send = l10n.OutlineAsk + "：" + text
		}
	}
	var snap *studySnapshot
	if s.quickImages {
		snap = s.quickSnapshot
	}
	s.startConversation(conversationStart{send: send, task: task, bind: true, allowImages: s.quickImages, snapshot: snap}, func(err error) {
		s.quickSending = false
		if err != nil {
			s.quickError = l10n.StudyQuickSendFailed(err.Error())
			return
		}
		s.quickInput = ""
		s.quickAsk, s.quickImages, s.quickSnapshot = false, false, nil
		s.tool = toolSelect
	})
}

// openPlayerPanel shows the player's full side panel (outline, AI tools)
// as a dialog.
func (s *StudyCanvas) openPlayerPanel() {
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		m3.Dialog(c, func() {
			ui.Column(c).Width(480).Height(f32(math.Min(600, s.viewport.Y-48))).Children(func() { s.panel(c) })
		})
	}, nil)
}

func (s *StudyCanvas) layers() {
	m3.ShowModalBottomSheet(m3.SheetOptions{}, func(c *ui.Context, r *m3.Route) {
		d := s.doc
		ui.Column(c).FillWidth().Height(330).Children(func() {
			m3.Tile{Title: l10n.StudyLayers, Trailing: func() {
				if m3.IconBtn(c, icons.Add, l10n.StudyAddLayer).Clicked() && len(d.Layers) < 32 {
					d.Checkpoint()
					l := study.Layer{ID: study.NewID(), Name: l10n.StudyLayerName(len(d.Layers) + 1), Visible: true}
					d.Layers = append(d.Layers, l)
					d.ActiveLayer = l.ID
					d.Changed()
				}
			}}.Show(c)
			ui.Scroll(c).FillWidth().Grow(1).Basis(0).Children(func() {
				for _, l := range d.Layers {
					vis, lock := icons.VisibilityOutlined, icons.LockOpenOutlined
					if !l.Visible {
						vis = icons.VisibilityOffOutlined
					}
					if l.Locked {
						lock = icons.LockOutline
					}
					id := l.ID
					toggle := func(change func(l *study.Layer)) {
						d.Checkpoint()
						if l := d.Layer(id); l != nil {
							change(l)
						}
						d.Changed()
					}
					ui.Box(c.Key("layer/" + id)).FillWidth().Children(func() {
						if (m3.Tile{Title: l.Name, Tap: true, Selected: d.ActiveLayer == id,
							Leading: func() {
								if m3.IconBtn(c, vis, l10n.StudyToggleVisible).Clicked() {
									toggle(func(l *study.Layer) { l.Visible = !l.Visible })
								}
							},
							Trailing: func() {
								if m3.IconBtn(c, lock, l10n.StudyToggleLocked).Clicked() {
									toggle(func(l *study.Layer) { l.Locked = !l.Locked })
								}
							}}).Show(c).Clicked() {
							d.ActiveLayer = id
						}
					})
				}
			})
		})
	})
}

// ExportRecovery saves a copy of the canvas to a file the user picks.
func (s *StudyCanvas) ExportRecovery() {
	if s.doc == nil {
		return
	}
	body := core.Encode(s.doc.JSON())
	widgets.PickSavePath("学习画布-"+s.lectureID()+".json", core.FileFilter{Name: "JSON", Extensions: []string{"json"}}, "", func(path string) {
		if path == "" {
			return
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			s.toast(l10n.StudyExportError(err.Error()))
			return
		}
		s.toast(l10n.StudyRecoverySaved)
	})
}

// Flush saves the canvas before the player leaves it.
func (s *StudyCanvas) Flush(done func(bool)) { s.flush(done) }

// visibleWorld is the canvas area in the window.
func (s *StudyCanvas) visibleWorld() study.Rect {
	d := s.doc
	return study.RectLTWH(-d.Camera.X/d.Zoom, -d.Camera.Y/d.Zoom, s.viewport.X/d.Zoom, s.viewport.Y/d.Zoom)
}

// Show builds the canvas.
func (s *StudyCanvas) Show(c *ui.Context) {
	s.ctx = c
	if s.fullscreen {
		s.video(c)
		return
	}
	if s.loadErr != nil {
		ui.Column(c).Fill().Justify(ui.Center).AlignItems(ui.Center).Children(func() {
			m3.Text(c, fmt.Sprintf("画布打开失败：%v", s.loadErr), design.TT().BodyMedium.C(design.S().OnSurface))
			if m3.TextButton(c, "重试").Clicked() {
				s.loadErr = nil
				s.load(nil)
			}
		})
		return
	}
	d := s.doc
	if d == nil {
		ui.Box(c).Fill().Center().Children(func() { m3.Circular(c, m3.Progress{}) })
		return
	}
	s.stepFly(c)
	s.enabledPlugins = s.activePlugins()
	s.hasCanvasTools = len(s.canvasToolContributions()) > 0
	ui.Column(c).Fill().Children(func() {
		area := ui.Box(c.Key("study-area")).FillWidth().Grow(1).Basis(0).Clip()
		if b := area.Bounds(); b.W > 0 {
			s.viewport = study.Pt{X: float64(b.W), Y: float64(b.H)}
		} else {
			s.viewport = study.Pt{X: float64(WindowWidth), Y: float64(WindowHeight)}
		}
		s.measureSelectionControls()
		s.selectionToolbarDocked = !s.snapshotting && s.selectionReady && !s.quickAsk && s.tool == toolRegion &&
			s.selectionBounds != nil && s.selectionMenuRect() == nil
		area.Children(func() { s.canvas(c, d) })
		if s.selectionToolbarDocked {
			ui.Box(c).FillWidth().Height(56).Shrink(0).Children(func() { s.selectionToolbar(c, true) })
		}
	})
}

func (s *StudyCanvas) moveDock(id string, next dockPlacement) {
	switch id {
	case "ai":
		s.aiPlacement = next
		s.stageOpen = true
	default:
		s.penPlacement = next
		s.dockSmall = false
	}
	s.saveLayout()
}

func (s *StudyCanvas) canvas(c *ui.Context, d *study.Document) {
	ids := map[string]bool{}
	for _, card := range d.Cards {
		ids[card.ID] = true
	}
	for id := range s.headerActions {
		if !ids[id] {
			delete(s.headerActions, id)
		}
	}
	stage := s.visibleWorld().Inflate(math.Max(s.viewport.X, s.viewport.Y) / d.Zoom)
	s.stageRect = &stage
	ui.Box(c).Fill().Clip().Children(func() {
		input := ui.Box(c.Key("study-input")).Absolute().Top(0).Left(0).Right(0).Bottom(0).Clip().HandleInput(s.input)
		if b := input.Bounds(); b.W > 0 {
			s.origin = ptOf(b.X, b.Y)
		}
		s.assetDrops(input)
		input.Children(func() {
			s.capture(c, d, stage)
			// The interaction feedback (lasso, handles, the eraser) and the
			// evidence over the material.
			ui.Box(c.Key("study-feedback")).Absolute().Top(0).Left(0).Right(0).Bottom(0).IgnoreInput(true).
				Draw(func(p *ui.Painter, r ui.Rect) { s.paintFeedback(p, r) })
			ui.Box(c.Key("study-evidence")).Absolute().Top(0).Left(0).Right(0).Bottom(0).IgnoreInput(true).
				Draw(func(p *ui.Painter, r ui.Rect) { s.paintEvidence(p, r) })
		})
		s.dropOutline(c)
		s.holdCueView(c)
		s.dockGroup(c, s.topBarsBottom()+8)
		s.fixedTopBars(c)
		s.agentStage(c)
		s.reviewPanel(c)
		s.returnPill(c)
		if !s.selectionToolbarDocked {
			s.selectionToolbar(c, false)
		}
		s.shelf(c)
		if s.viewport.X >= 600 {
			s.zoomControl(c, d)
		}
	})
}

// capture is what a snapshot of the canvas shows: paper, cards and ink.
func (s *StudyCanvas) capture(c *ui.Context, d *study.Document, stage study.Rect) {
	ch := studyChrome()
	ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Draw(func(p *ui.Painter, r ui.Rect) {
		paintPaper(p, r, d.Camera, d.Zoom, s.paperStyle, studyPaperTone(s.paperColor))
	})
	// Camera changes transform an already laid-out world; cards and ink are
	// laid out again only as the visible area leaves the bounded stage.
	z := f32(d.Zoom)
	tx, ty := f32(d.Camera.X+stage.L*d.Zoom), f32(d.Camera.Y+stage.T*d.Zoom)
	ui.Box(c.Key("study-world")).Absolute().Left(0).Top(0).Size(f32(stage.W()), f32(stage.H())).
		TransformOrigin(0, 0).Transform(ui.Scaling(z, z).Then(ui.Translation(tx, ty))).Children(func() {
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).IgnoreInput(true).Draw(func(p *ui.Painter, r ui.Rect) {
			paintConnections(p, r, d, stage, ch)
		})
		for _, card := range d.Cards {
			if card.Type == "conversation" || !d.Visible(card.Layer) ||
				!(card.Type == "lecture" || card.Type == "video" || card.Rect.Overlaps(stage)) {
				continue
			}
			s.placedCard(c, card, stage)
		}
		for _, o := range d.PluginObjects {
			if o.Rect().Overlaps(stage) {
				s.pluginObject(c, o, stage, s.enabledPlugins[o.PluginID()])
			}
		}
		dark := design.T().Dark
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).IgnoreInput(true).Draw(func(p *ui.Painter, r ui.Rect) {
			paintInkScene(p, r, d, s.prefix, stage, dark)
		})
	})
	ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).IgnoreInput(true).Draw(func(p *ui.Painter, r ui.Rect) {
		s.paintInkTail(p, r)
	})
}

// placedCard puts card at its place on the stage, turned by its quarter
// turns.
func (s *StudyCanvas) placedCard(c *ui.Context, card *study.Card, stage study.Rect) {
	r := card.Rect
	ui.Box(c.Key("study-card/"+card.ID)).Absolute().Left(f32(r.L-stage.L)).Top(f32(r.T-stage.T)).Size(f32(r.W()), f32(r.H())).Children(func() {
		w, h := r.W(), r.H()
		turns := ((card.QuarterTurns % 4) + 4) % 4
		if turns%2 == 1 {
			w, h = h, w
		}
		inner := ui.Box(c).Absolute().Left(f32((r.W()-w)/2)).Top(f32((r.H()-h)/2)).Size(f32(w), f32(h))
		if turns != 0 {
			inner = inner.Rotate(float32(90 * turns))
		}
		inner.Children(func() { s.card(c, card) })
		if card.QuarterTurns != 0 {
			col := studyChrome().inkFaint
			if s.selection[card.ID] {
				col = studyChrome().accent
			}
			ui.Box(c).Absolute().Right(0).Bottom(0).IgnoreInput(true).Children(func() { m3.Icon(c, icons.SouthEast, 12, col) })
		}
	})
}

func (s *StudyCanvas) zoomControl(c *ui.Context, d *study.Document) {
	e := ui.Row(c.Key("study-zoom")).Absolute().Right(studyZoomInset).Bottom(studyZoomInset).AlignItems(ui.Center).Padding(0, design.SpaceXXS)
	widgets.Glass{Radius: design.CornerFull}.Apply(e)
	s.recordControl(e)
	e.Children(func() {
		if (m3.IconButton{Icon: icons.RemoveRounded, Tooltip: l10n.StudyZoomOut, Size: 18, Box: 36}).Show(c).Clicked() {
			s.zoomBy(1 / 1.2)
		}
		m3.Text(c, fmt.Sprintf("%d%%", int(math.Round(d.Zoom*100))), design.TT().LabelMedium.C(design.S().OnSurface)).
			Width(44).TextAlign(ui.Center).FontFeatures("tnum")
		if (m3.IconButton{Icon: icons.AddRounded, Tooltip: l10n.StudyZoomIn, Size: 18, Box: 36}).Show(c).Clicked() {
			s.zoomBy(1.2)
		}
		if (m3.IconButton{Icon: icons.FitScreenOutlined, Tooltip: l10n.StudyFit, Size: 18, Box: 36}).Show(c).Clicked() {
			s.fit()
		}
	})
}

// studyPaperTone is the paper colour as shown: in dark mode each paper
// keeps a trace of its hue over the scheme's low container surface.
func studyPaperTone(color uint32) ui.Color {
	if !design.T().Dark {
		return design.ARGB(color)
	}
	a := float32(.06)
	if color == 0xffffffff {
		a = 0
	}
	return design.AlphaBlend(design.ARGB(color).Alpha(a), design.S().SurfaceContainerLow)
}

// paintPaper is _Paper: the paper's colour and its ruling, which moves with
// the camera and doubles its spacing as the view zooms out.
func paintPaper(p *ui.Painter, r ui.Rect, camera study.Pt, zoom float64, style string, tone ui.Color) {
	p.Fill(r, tone, 0)
	if style == "blank" {
		return
	}
	spacing := 14.0
	if zoom > 0 {
		spacing = 36 * zoom
		for spacing < 22 {
			spacing *= 2
		}
	}
	line := ui.Color{R: 0x26, G: 0x35, B: 0x4a, A: 0x18}
	if design.Luminance(tone) <= .5 {
		line = ui.Color{R: 255, G: 255, B: 255, A: 0x1f}
	}
	w, h := float64(r.W), float64(r.H)
	start := func(v float64) float64 {
		m := math.Mod(v, spacing)
		if m < 0 {
			m += spacing
		}
		return m
	}
	for y := start(camera.Y); y < h; y += spacing {
		if style == "ruled" || style == "grid" {
			p.Line(r.X, r.Y+f32(y), r.X+r.W, r.Y+f32(y), .7, line)
		} else {
			for x := start(camera.X); x < w; x += spacing {
				p.Fill(ui.Rect{X: r.X + f32(x) - .8, Y: r.Y + f32(y) - .8, W: 1.6, H: 1.6}, line, .8)
			}
		}
	}
	if style == "grid" {
		for x := start(camera.X); x < w; x += spacing {
			p.Line(r.X+f32(x), r.Y, r.X+f32(x), r.Y+r.H, .7, line)
		}
	}
}

// paintInkScene is _InkScene: the saved ink, and the live prefix of the
// stroke being drawn, on the stage.
func paintInkScene(p *ui.Painter, r ui.Rect, d *study.Document, live []*study.Stroke, stage study.Rect, dark bool) {
	printed := map[string]bool{}
	if dark {
		for _, c := range d.Cards {
			if study.OnPrintedPage(c.Type) {
				printed[c.ID] = true
			}
		}
	}
	at := study.Pt{X: float64(r.X) - stage.L, Y: float64(r.Y) - stage.T}
	draw := func(st *study.Stroke) {
		if d.StrokeVisible(st) && st.Bounds().Overlaps(stage) {
			color := st.Color
			if dark && !printed[st.Anchor] {
				color = study.InkOnDark(color, st.Tool)
			}
			paintInk(p, at, st.Points, color, st.Width, st.Tool, st.Brush)
		}
	}
	for _, st := range d.Strokes {
		draw(st)
	}
	for _, st := range live {
		draw(st)
	}
}

// paintInkTail is _InkTail: the stroke being drawn past its saved prefix,
// or the shape it is turning into.
func (s *StudyCanvas) paintInkTail(p *ui.Painter, r ui.Rect) {
	pts, start := s.points, s.prefixEnd
	if s.shapePreview != nil {
		pts, start = s.shapePreview, 0
	}
	if len(pts) == 0 || start >= len(pts) {
		return
	}
	d := s.doc
	color := s.gestureColor
	tool := s.strokeTool()
	if design.T().Dark {
		anchored := d.Card(s.anchor)
		if anchored == nil || !study.OnPrintedPage(anchored.Type) {
			color = study.InkOnDark(color, tool)
		}
	}
	brush := s.gestureBrush
	z := f32(d.Zoom)
	p.Transform(ui.Scaling(z, z).Then(ui.Translation(r.X+f32(d.Camera.X), r.Y+f32(d.Camera.Y))), 0, 0, func() {
		paintInk(p, study.Pt{}, pts[start:], color, s.gestureWidth, tool, &brush)
	})
}

// paintInk is paintInk: ink of points, moved by at, in one colour and
// width.
func paintInk(p *ui.Painter, at study.Pt, points []study.InkPoint, color uint32, width float64, tool string, brush *study.Brush) {
	if len(points) == 0 {
		return
	}
	alpha := 1.0
	if brush != nil {
		alpha = brush.Opacity
	} else if tool == "highlighter" {
		alpha = .3
	}
	col := design.ARGB(color).Alpha(f32(alpha))
	pt := func(i int) (float32, float32) { return f32(points[i].P.X + at.X), f32(points[i].P.Y + at.Y) }
	square := brush != nil && brush.SquareTip
	capOf := func(path *ui.Path) {
		if square {
			path.Cap(ui.SquareCap)
		} else {
			path.Cap(ui.RoundCap)
		}
	}
	if len(points) == 1 {
		x, y := pt(0)
		rad := f32(width * .5)
		p.Fill(ui.Rect{X: x - rad, Y: y - rad, W: 2 * rad, H: 2 * rad}, col, rad)
		return
	}
	if tool == "highlighter" {
		path := &ui.Path{}
		x, y := pt(0)
		path.MoveTo(x, y)
		for i := 1; i < len(points); i++ {
			x, y := pt(i)
			path.LineTo(x, y)
		}
		capOf(path)
		path.Join(ui.RoundJoin)
		p.StrokePath(path, f32(width), col)
		return
	}
	for i := 1; i < len(points); i++ {
		pr := (points[i].Pressure + points[i-1].Pressure) / 2
		w := width * (.35 + .65*pr)
		if brush != nil {
			w = width * brush.RadiusScale(pr)
		}
		x0, y0 := pt(i - 1)
		x1, y1 := pt(i)
		seg := &ui.Path{}
		seg.MoveTo(x0, y0)
		seg.LineTo(x1, y1)
		capOf(seg)
		p.StrokePath(seg, f32(w), col)
		if brush != nil && brush.Family == "pencil" {
			dx, dy := points[i].P.X-points[i-1].P.X, points[i].P.Y-points[i-1].P.Y
			if dist := math.Hypot(dx, dy); dist > 0 {
				nx, ny := -dy/dist*w*.22, dx/dist*w*.22
				grain := design.ARGB(color).Alpha(f32(brush.Opacity * .18))
				p.Line(x0+f32(nx), y0+f32(ny), x1+f32(nx), y1+f32(ny), f32(math.Max(.25, width*.08)), grain)
			}
		}
	}
}
