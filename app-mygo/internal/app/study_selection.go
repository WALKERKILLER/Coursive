package app

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/study"
)

// study_selection_tools.dart, study_snapshot_capture.dart and
// study_attachment.dart: what a selection does past asking about it, and
// the screenshot of it a conversation reads.

// studySnapshot is a captured raster, independent of the canvas: a failed
// upload keeps these exact bytes, so a retry never takes another picture.
type studySnapshot struct {
	bytes      []byte
	selection  core.J
	attachment core.J
	err        error
	uploading  bool
	bmp        *ui.Bitmap
}

func (s *studySnapshot) bitmap() *ui.Bitmap {
	if s.bmp == nil {
		s.bmp, _ = ui.DecodeBitmap(s.bytes)
	}
	return s.bmp
}

// upload sends the screenshot once; called off the UI thread.
func (s *studySnapshot) upload(lecture string) (core.J, error) {
	if a := onUIValue(func() core.J { return s.attachment }); a != nil {
		return a, nil
	}
	onUI(func() { s.uploading, s.err = true, nil })
	out, err := State.Api().UploadStudyAttachment(lecture, s.bytes, s.selection)
	var a core.J
	onUI(func() {
		s.uploading = false
		if err != nil {
			s.err = err
			return
		}
		s.attachment = core.Obj(plainJSON(out))
		a = s.attachment
	})
	return a, err
}

func selectionKindLabel(kind study.SelectionKind) string {
	switch kind {
	case study.SelectInk:
		return l10n.StudySelectInk
	case study.SelectText:
		return l10n.StudySelectText
	case study.SelectDocument:
		return l10n.StudySelectDocument
	case study.SelectVideo:
		return l10n.StudySelectVideo
	}
	return l10n.StudySelectOther
}

func (s *StudyCanvas) editSelectionFilters() {
	chosen := map[study.SelectionKind]bool{}
	for k, v := range s.selectionKinds {
		chosen[k] = v
	}
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		m3.AlertDialog(c, m3.Alert{Title: l10n.StudySelectionFilters, Content: func() {
			ui.Column(c).FillWidth().Children(func() {
				for _, kind := range study.SelectionKinds {
					on := chosen[kind]
					if m3.CheckTile(c, m3.Tile{Title: selectionKindLabel(kind)}, &on) {
						chosen[kind] = on
					}
				}
			})
		}, Actions: func() {
			if m3.TextButton(c, l10n.CommonCancel).Clicked() {
				r.Pop(nil)
			}
			if m3.FilledButton(c, l10n.CommonSave).Clicked() {
				r.Pop(true)
			}
		}})
	}, func(v any) {
		if v != true || s.disposed {
			return
		}
		clear(s.selectionKinds)
		for k, on := range chosen {
			if on {
				s.selectionKinds[k] = true
			}
		}
		clear(s.selection)
		s.detail.Clear()
		s.selectionReady = false
		s.selectionBounds, s.regionSelection = nil, nil
		s.saveBrushSettings()
	})
}

// selectionActions is the bar's 「…」: the editing actions of a selection.
func (s *StudyCanvas) selectionActions(c *ui.Context) {
	content := len(s.selection) > 0 || len(s.detail.StrokeIDs) > 0
	ink := len(s.detail.StrokeIDs) > 0
	m3.Menu(c, "study-selection-actions", m3.BelowEnd|m3.Popup, func(t ui.Element, open bool) {
		ic := icons.MoreHoriz
		if s.selectionBusy {
			ic = icons.HourglassTop
		}
		t.Children(func() {
			m3.IconButton{Icon: ic, Tooltip: l10n.StudySelectionMore, Disabled: s.selectionBusy}.Show(c)
		})
	}, func() []m3.MenuItem {
		items := []m3.MenuItem{
			{Label: "复制", Disabled: !content, OnTap: s.copySelection},
			{Label: "粘贴", Disabled: s.copiedBounds == nil, OnTap: s.pasteSelection},
			{Label: "副本", Disabled: !content, OnTap: func() { s.copySelection(); s.pasteSelection() }},
			{Divider: true},
			{Label: l10n.StudyRotateSelection, OnTap: func() { s.selectionJob(s.rotateSelected) }},
			{Label: l10n.StudyEditShape, Disabled: !ink, OnTap: func() { s.selectionJob(s.editSelectedShape) }},
			{Label: l10n.StudyRecognizeInk, Disabled: !ink, OnTap: func() { s.selectionJob(s.recognizeSelectedInk) }},
		}
		if s.linkStart != nil {
			items = append(items, m3.MenuItem{Label: l10n.StudyLinkCancel, OnTap: func() { s.linkStart, s.linkStartBounds = nil, nil }})
		}
		return append(items,
			m3.MenuItem{Label: l10n.StudySelectionFilters, OnTap: s.editSelectionFilters},
			m3.MenuItem{Divider: true},
			m3.MenuItem{Label: "删除", Disabled: !content, OnTap: s.deleteSelection},
			m3.MenuItem{Label: l10n.StudySelectionClear, OnTap: s.clearSelection},
		)
	})
}

// selectionJob runs work off the UI thread, one at a time, and says what
// went wrong.
func (s *StudyCanvas) selectionJob(work func() error) {
	if s.selectionBusy || s.doc == nil {
		return
	}
	s.selectionBusy = true
	go func() {
		err := work()
		onUI(func() {
			s.selectionBusy = false
			if err != nil && !s.disposed && err != errGone {
				s.toast(l10n.StudySelectionError(err.Error()))
			}
		})
	}()
}

// checkSelectionEdit fails once the canvas changed since edit, or cannot
// take another card.
func (s *StudyCanvas) checkSelectionEdit(d *study.Document, edit int) error {
	return onUIValue(func() error {
		if s.disposed || s.doc != d || d.Edit != edit {
			return formatError(l10n.StudySelectionChanged)
		}
		if !d.Editable(d.ActiveLayer) {
			return formatError(l10n.StudyLocked)
		}
		if len(d.Cards) >= 1998 {
			return formatError(l10n.StudyCardLimit)
		}
		return nil
	})
}

// selectionSources are the anchors the daemon reads for query.
func (s *StudyCanvas) selectionSources(query core.J) ([]study.Anchor, error) {
	if !s.flushWait() {
		return nil, formatError(l10n.StudySelectionChanged)
	}
	gone := false
	onUI(func() {
		if s.disposed || s.doc == nil {
			gone = true
			return
		}
		query["revision"] = s.doc.Revision
	})
	if gone {
		return nil, errGone
	}
	out, err := State.Api().StudyContext(s.lectureID(), query)
	if err != nil {
		return nil, err
	}
	var anchors []study.Anchor
	for _, src := range core.Obj(plainJSON(out)).Os("sources") {
		anchors = append(anchors, anchorOf(src["anchor"]))
	}
	return anchors, nil
}

// selectionRasterRatio is how many pixels a screenshot takes per DIP: up to
// 4, within about 12 megapixels for the window.
func selectionRasterRatio(viewport study.Pt) float64 {
	return math.Min(4, math.Sqrt(12000000/math.Max(1, viewport.X*viewport.Y)))
}

// regionImage is a PNG of region as the canvas shows it, cut to polygon.
func (s *StudyCanvas) regionImage(region study.Rect, polygon []study.Pt) ([]byte, error) {
	var img *image.RGBA
	var screen study.Rect
	var problem error
	onUI(func() {
		d := s.doc
		if s.disposed || d == nil {
			problem = errGone
			return
		}
		screen = s.screenRect(region)
		view := study.RectLTWH(0, 0, s.viewport.X, s.viewport.Y).Inflate(.5)
		if screen.W() < 2 || screen.H() < 2 || !view.Contains(screen.TopLeft()) || !view.Contains(screen.BottomRight().Sub(study.Pt{X: .001, Y: .001})) {
			problem = formatError(l10n.StudyCaptureOutside)
			return
		}
		scale := math.Min(selectionRasterRatio(s.viewport), 1600/math.Max(screen.W(), screen.H()))
		w, h := int(math.Round(screen.W())), int(math.Round(screen.H()))
		stage := s.visibleWorld().Inflate(math.Max(s.viewport.X, s.viewport.Y) / d.Zoom)
		s.rendering = true
		defer func() { s.rendering = false }()
		img = ui.Render(func(c *ui.Context) {
			ui.Box(c).Fill().Clip().Children(func() {
				ui.Box(c).Absolute().Left(f32(-screen.L)).Top(f32(-screen.T)).Size(f32(s.viewport.X), f32(s.viewport.Y)).Children(func() {
					s.capture(c, d, stage)
				})
			})
		}, w, h, f32(scale))
	})
	if problem != nil {
		return nil, problem
	}
	if img == nil {
		return nil, formatError(l10n.StudyCaptureOutside)
	}
	if len(polygon) >= 3 {
		// Outside the lasso is transparent, as the clip path leaves it.
		b := img.Bounds()
		sx, sy := region.W()/float64(b.Dx()), region.H()/float64(b.Dy())
		for y := range b.Dy() {
			for x := range b.Dx() {
				p := study.Pt{X: region.L + (float64(x)+.5)*sx, Y: region.T + (float64(y)+.5)*sy}
				if !study.InPolygon(p, polygon) {
					img.SetRGBA(b.Min.X+x, b.Min.Y+y, color.RGBA{})
				}
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s *StudyCanvas) captureRegionCard() {
	s.selectionJob(func() error {
		var d *study.Document
		var edit int
		var region *study.Rect
		var query core.J
		onUI(func() { d, edit, region, query = s.doc, s.doc.Edit, s.regionSelection, s.selectedQuery() })
		if region == nil {
			return formatError(l10n.StudySelectionEmpty)
		}
		data, err := s.regionImage(*region, nil)
		if err != nil {
			return err
		}
		if err := s.checkSelectionEdit(d, edit); err != nil {
			return err
		}
		sources, err := s.selectionSources(query)
		if err != nil {
			return err
		}
		if err := s.checkSelectionEdit(d, edit); err != nil {
			return err
		}
		imported, err := State.Api().ImportStudyAsset(s.lectureID(), "selection-"+study.NewID()+".png", bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return err
		}
		if err := s.checkSelectionEdit(d, edit); err != nil {
			return err
		}
		objects := core.List(plainJSON(core.Obj(imported)["objects"]))
		if len(objects) != 1 {
			return formatError(l10n.StudyCaptureOutside)
		}
		onUI(func() {
			card := study.CardFrom(plainMap(objects[0]))
			card.Layer, card.Title, card.Sources = d.ActiveLayer, l10n.StudyCaptureCard, sources
			card.Rect = s.freeSpot(study.Pt{X: card.Rect.W(), Y: card.Rect.H()})
			d.Checkpoint()
			d.Cards = append(d.Cards, card)
			d.Changed()
			s.captureLasso, s.selectionReady, s.tool = false, false, toolSelect
			clear(s.selection)
			s.selection[card.ID] = true
			s.focusSource(study.Anchor{ID: study.NewID(), ObjectID: card.ID, Kind: "image"})
		})
		s.flushWait()
		return nil
	})
}

// linkSelection links two selections: the first is remembered, the
// second makes a pair of linked cards.
func (s *StudyCanvas) linkSelection() error {
	var d *study.Document
	var edit int
	var bounds *study.Rect
	var query core.J
	onUI(func() { d, edit, bounds, query = s.doc, s.doc.Edit, s.regionSelection, s.selectedQuery() })
	if bounds == nil {
		return formatError(l10n.StudySelectionEmpty)
	}
	sources, err := s.selectionSources(query)
	if err != nil {
		return err
	}
	if err := s.checkSelectionEdit(d, edit); err != nil {
		return err
	}
	if len(sources) == 0 {
		return formatError(l10n.StudySelectionEmpty)
	}
	var problem error
	onUI(func() {
		if s.linkStart == nil {
			s.linkStart, s.linkStartBounds = sources, rectPtr(*bounds)
			s.toast(l10n.StudyLinkPending)
			return
		}
		strokes := map[string]bool{}
		for _, st := range d.Strokes {
			strokes[st.ID] = true
		}
		for _, a := range s.linkStart {
			ok := d.Card(a.ObjectID) != nil
			if a.Kind == "ink" {
				ok = true
				for _, id := range a.StrokeIDs {
					ok = ok && strokes[id]
				}
			}
			if !ok {
				problem = formatError(l10n.StudySelectionChanged)
				return
			}
		}
		endpoint := func(at study.Rect, targets []study.Anchor) *study.Card {
			return &study.Card{ID: study.NewID(), Type: "knowledge", Layer: d.ActiveLayer, Rect: study.RectLTWH(at.R+24, at.T, 280, 180),
				Title: l10n.StudyLinkTitle, Blocks: []study.Block{{Type: "paragraph", Text: l10n.StudyLinkDescription}}, Sources: targets,
				Responses: map[string]string{}}
		}
		a, b := endpoint(*s.linkStartBounds, sources), endpoint(*bounds, s.linkStart)
		d.Checkpoint()
		d.Cards = append(d.Cards, a, b)
		d.Connections = append(d.Connections, study.Connection{ID: study.NewID(), From: a.ID, To: b.ID, Label: l10n.StudyLinkTitle})
		d.Changed()
		s.linkStart, s.linkStartBounds = nil, nil
	})
	if problem != nil {
		return problem
	}
	s.flushWait()
	return nil
}

func (s *StudyCanvas) rotateSelected() error {
	var problem error
	onUI(func() {
		d := s.doc
		if !d.CanRotateSelection(s.selection, s.detail.StrokeIDs) {
			problem = formatError(l10n.StudyRotateUnsupported)
			return
		}
		b := s.boundsOfSelection()
		if b == nil {
			return
		}
		cards, ink := map[string]bool{}, map[string]bool{}
		for id := range s.selection {
			cards[id] = true
		}
		for id := range s.detail.StrokeIDs {
			ink[id] = true
		}
		d.RotateSelection(cards, ink, b.Center())
		s.selectionPolygon = nil
		s.selectionBounds = s.boundsOfSelection()
		s.regionSelection = s.selectionBounds
		s.detail.Region = s.regionSelection
		s.detail.ObjectID = ""
	})
	if problem != nil {
		return problem
	}
	s.flushWait()
	return nil
}

// editSelectedShape edits a recognized shape's anchors.
func (s *StudyCanvas) editSelectedShape() error {
	var d *study.Document
	var edit int
	var ink []*study.Stroke
	var shape *study.Shape
	problem := onUIValue(func() error {
		d, edit = s.doc, s.doc.Edit
		for _, st := range d.Strokes {
			if s.detail.StrokeIDs[st.ID] {
				ink = append(ink, st)
			}
		}
		if len(s.selection) > 0 || len(ink) == 0 {
			return formatError(l10n.StudyShapeUnavailable)
		}
		gesture := ink[0].Gesture
		for _, st := range ink {
			if st.Gesture != gesture || !d.Editable(st.Layer) {
				return formatError(l10n.StudyShapeUnavailable)
			}
		}
		for _, st := range d.Strokes {
			if st.Gesture == gesture && (!d.Editable(st.Layer) || !s.detail.StrokeIDs[st.ID]) {
				return formatError(l10n.StudyShapeUnavailable)
			}
		}
		var points []study.Pt
		for _, st := range ink {
			for _, p := range st.Points {
				if len(points) == 0 || points[len(points)-1] != p.P {
					points = append(points, p.P)
				}
			}
		}
		if len(points) == 2 {
			points = []study.Pt{points[0], points[0].Lerp(points[1], .5), points[1]}
		}
		shape = study.RecognizeShape(points, d.Zoom)
		if shape == nil {
			return formatError(l10n.StudyShapeUnavailable)
		}
		return nil
	})
	if problem != nil {
		return problem
	}
	changed, ok := awaitDialog(func(c *ui.Context, r *m3.Route) { shapeAnchorDialog(c, r, shape) }).([]study.Pt)
	if !ok {
		return nil
	}
	if err := s.checkSelectionEdit(d, edit); err != nil {
		return err
	}
	onUI(func() {
		original := ink[0]
		points := make([]study.InkPoint, len(changed))
		for i, p := range changed {
			points[i] = study.InkPoint{P: p, Pressure: original.Points[0].Pressure}
		}
		replacement := &study.Stroke{ID: original.Gesture, Layer: original.Layer, Gesture: original.Gesture, Anchor: original.Anchor,
			Page: original.Page, Tool: original.Tool, Brush: original.Brush, Width: original.Width, Color: original.Color, Points: points}
		d.Checkpoint()
		d.Strokes = filterStrokes(d.Strokes, func(st *study.Stroke) bool { return st.Gesture != original.Gesture })
		chunks := study.Chunks(replacement)
		d.Strokes = append(d.Strokes, chunks...)
		d.Changed()
		s.detail.StrokeIDs = map[string]bool{}
		for _, st := range chunks {
			s.detail.StrokeIDs[st.ID] = true
		}
		s.selectionPolygon = nil
		s.selectionBounds = s.boundsOfSelection()
		s.regionSelection = s.selectionBounds
	})
	s.flushWait()
	return nil
}

// shapeAnchorDialog is _ShapeAnchorDialog: drag a shape's anchors, which
// snap to one another's lines.
func shapeAnchorDialog(c *ui.Context, r *m3.Route, shape *study.Shape) {
	type state struct {
		points   []study.Pt
		drag     int
		baseline []study.Pt
	}
	st := *ui.Local(ui.Box(c), "shape-anchor", func() *state {
		return &state{points: append([]study.Pt(nil), shape.Points...), drag: -1}
	})
	ellipse := shape.Kind == "circle" || shape.Kind == "ellipse"
	anchors := func() []study.Pt {
		if ellipse {
			b := study.Bounds(st.points)
			return []study.Pt{b.TopLeft(), b.BottomRight()}
		}
		var out []study.Pt
		for _, p := range st.points {
			if !slices.Contains(out, p) {
				out = append(out, p)
			}
		}
		return out
	}
	var bounds study.Rect
	for i, p := range shape.Points {
		cr := study.RectCircle(p, 1)
		if i == 0 {
			bounds = cr
		} else {
			bounds = bounds.Union(cr)
		}
	}
	bounds = bounds.Inflate(35)
	m3.AlertDialog(c, m3.Alert{Title: l10n.StudyEditShape, Width: 560, Content: func() {
		ui.Column(c).Width(560).Height(360).Children(func() {
			m3.Text(c, l10n.StudyShapeDragHint, design.TT().BodyMedium.C(design.S().OnSurface))
			area := ui.Box(c.Key("study-shape-editor")).FillWidth().Grow(1).Basis(0)
			ab := area.Bounds()
			scale := math.Min(float64(ab.W)/bounds.W(), float64(ab.H)/bounds.H())
			off := study.Pt{X: (float64(ab.W) - bounds.W()*scale) / 2, Y: (float64(ab.H) - bounds.H()*scale) / 2}.Sub(bounds.TopLeft().Mul(scale))
			world := func(x, y float32) study.Pt {
				return study.Pt{X: float64(x - ab.X), Y: float64(y - ab.Y)}.Sub(off).Div(scale)
			}
			area.HandleInput(func(ev ui.InputEvent) bool {
				if scale <= 0 {
					return false
				}
				switch ev.Kind {
				case ui.InputPointerDown:
					p, list := world(ev.X, ev.Y), anchors()
					st.drag = -1
					for i, a := range list {
						if a.Sub(p).Dist()*scale < 24 {
							st.drag = i
							break
						}
					}
					st.baseline = append([]study.Pt(nil), st.points...)
					return true
				case ui.InputPointerMove:
					if st.drag < 0 {
						return true
					}
					list := anchors()
					old := list[st.drag]
					target := world(ev.X, ev.Y)
					for _, p := range list {
						if p == old {
							continue
						}
						if math.Abs(p.X-target.X)*scale < 6 {
							target.X = p.X
						}
						if math.Abs(p.Y-target.Y)*scale < 6 {
							target.Y = p.Y
						}
					}
					if !ellipse {
						for _, p := range list {
							if p != old && p.Sub(target).Dist()*scale < 4 {
								return true
							}
						}
					}
					if ellipse {
						box := study.RectPts(list[0], list[1])
						next := study.RectPts(list[1-st.drag], target)
						if next.W() < 4/scale || next.H() < 4/scale {
							return true
						}
						for i, p := range st.points {
							st.points[i] = study.Pt{X: next.L + (p.X-box.L)*next.W()/box.W(), Y: next.T + (p.Y-box.T)*next.H()/box.H()}
						}
					} else {
						for i, p := range st.points {
							if p == old {
								st.points[i] = target
							}
						}
					}
					c.Invalidate()
					return true
				case ui.InputPointerUp:
					if ev.Canceled && st.baseline != nil {
						st.points = st.baseline
					}
					st.drag, st.baseline = -1, nil
					return true
				}
				return false
			}).Draw(func(p *ui.Painter, rr ui.Rect) {
				if len(st.points) == 0 || scale <= 0 {
					return
				}
				blue := design.ARGB(0xff42639a)
				v := func(q study.Pt) (float32, float32) {
					w := q.Mul(scale).Add(off)
					return rr.X + f32(w.X), rr.Y + f32(w.Y)
				}
				path := &ui.Path{}
				x, y := v(st.points[0])
				path.MoveTo(x, y)
				for _, q := range st.points[1:] {
					x, y := v(q)
					path.LineTo(x, y)
				}
				p.StrokePath(path, 2, blue)
				guide := design.ARGB(0x3342639a)
				for _, a := range anchors() {
					x, y := v(a)
					for gx := float32(0); gx < rr.W; gx += 10 {
						p.Line(rr.X+gx, y, rr.X+min(rr.W, gx+4), y, 1, guide)
					}
					for gy := float32(0); gy < rr.H; gy += 10 {
						p.Line(x, rr.Y+gy, x, rr.Y+min(rr.H, gy+4), 1, guide)
					}
					p.FillPath((&ui.Path{}).Circle(x, y, 7), blue)
				}
			})
		})
	}, Actions: func() {
		if m3.TextButton(c, l10n.CommonCancel).Clicked() {
			r.Pop(nil)
		}
		if m3.FilledButton(c, l10n.CommonSave).Clicked() {
			r.Pop(append([]study.Pt(nil), st.points...))
		}
	}})
}

// recognizeSelectedInk turns the selected handwriting into a text card,
// with the learner's consent and review.
func (s *StudyCanvas) recognizeSelectedInk() error {
	var d *study.Document
	var edit int
	var ink []*study.Stroke
	var query core.J
	onUI(func() {
		d, edit = s.doc, s.doc.Edit
		var ids []any
		for _, st := range d.Strokes {
			if s.detail.StrokeIDs[st.ID] && d.StrokeVisible(st) {
				ink = append(ink, st)
				ids = append(ids, st.ID)
			}
		}
		query = core.J{"objects": []any{}, "include_video": false, "selection": map[string]any{"stroke_ids": nonNilList(ids)}}
	})
	if len(ink) == 0 {
		return formatError(l10n.StudySelectionEmpty)
	}
	consent := awaitDialog(func(c *ui.Context, r *m3.Route) {
		m3.AlertDialog(c, m3.Alert{Title: l10n.StudyRecognizeInk, Message: l10n.StudyRecognizeConsent, Actions: func() {
			if m3.TextButton(c, l10n.CommonCancel).Clicked() {
				r.Pop(nil)
			}
			if m3.FilledButton(c, l10n.StudyRecognizeStart).Clicked() {
				r.Pop(true)
			}
		}})
	})
	if consent != true {
		return nil
	}
	if err := s.checkSelectionEdit(d, edit); err != nil {
		return err
	}
	sources, err := s.selectionSources(query)
	if err != nil {
		return err
	}
	if err := s.checkSelectionEdit(d, edit); err != nil {
		return err
	}
	bounds := ink[0].Bounds()
	for _, st := range ink[1:] {
		bounds = bounds.Union(st.Bounds())
	}
	bounds = bounds.Inflate(8)
	var data []byte
	for _, edge := range []float64{1000, 720, 480} {
		scale := math.Min(1, edge/math.Max(bounds.W(), bounds.H()))
		w, h := max(1, int(math.Ceil(bounds.W()*scale))), max(1, int(math.Ceil(bounds.H()*scale)))
		// A raster of the selected ink alone: no page or other
		// handwriting.
		img := onUIValue(func() *image.RGBA {
			return ui.Render(func(c *ui.Context) {
				ui.Box(c).Fill().Background(ui.RGB(255, 255, 255)).Draw(func(p *ui.Painter, r ui.Rect) {
					p.Transform(ui.Scaling(f32(scale), f32(scale)).Then(ui.Translation(r.X, r.Y)), 0, 0, func() {
						for _, st := range ink {
							paintInk(p, bounds.TopLeft().Neg(), st.Points, 0xff111111, st.Width, st.Tool, st.Brush)
						}
					})
				})
			}, w, h, 1)
		})
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return err
		}
		data = buf.Bytes()
		if len(data) <= 450000 {
			break
		}
	}
	if len(data) > 450000 {
		return formatError(l10n.StudySelectionImageLimit)
	}
	if err := s.checkSelectionEdit(d, edit); err != nil {
		return err
	}
	request := core.J{"lecture_id": s.lectureID(), "question": l10n.StudyRecognizePrompt, "selection": map[string]any(query), "allow_images": true,
		"visuals": []any{map[string]any{"source_id": "selected-ink", "mime_type": "image/png", "data": base64.StdEncoding.EncodeToString(data)}}}
	result, _ := awaitDialog(func(c *ui.Context, r *m3.Route) { inkRecognitionDialog(c, r, request) }).(string)
	if strings.TrimSpace(result) == "" {
		return nil
	}
	if err := s.checkSelectionEdit(d, edit); err != nil {
		return err
	}
	onUI(func() {
		d.Checkpoint()
		d.Cards = append(d.Cards, &study.Card{ID: study.NewID(), Type: "text", Layer: d.ActiveLayer, Rect: s.freeSpot(study.Pt{X: 480, Y: 320}),
			Title: l10n.StudyRecognizeInk, Text: result, Sources: sources, Responses: map[string]string{}})
		d.Changed()
	})
	s.flushWait()
	return nil
}

// inkRun is _InkRecognitionDialogState: a run reading the ink, polled until
// it answers, then the text to correct.
type inkRun struct {
	text, err, run string
	busy, stopped  bool
	finished       bool
}

func inkRecognitionDialog(c *ui.Context, r *m3.Route, request core.J) {
	box := ui.Box(c)
	st := *ui.Local(box, "ink-run", func() *inkRun {
		st := &inkRun{busy: true}
		api := State.Api()
		go func() {
			out, err := api.StartCanvasRun(request)
			if err != nil {
				onUI(func() { st.err, st.busy = err.Error(), false })
				return
			}
			id := core.Obj(out).S("id")
			if onUIValue(func() bool { st.run = id; return st.stopped }) {
				_, _ = api.CancelCanvasRun(id)
				return
			}
			for {
				out, err := api.CanvasRun(id)
				done := onUIValue(func() bool {
					if st.stopped {
						return true
					}
					if err != nil {
						st.err, st.busy = err.Error(), false
						return true
					}
					run := core.Obj(out)
					switch run.S("state") {
					case "done":
						st.finished, st.text, st.busy = true, run.S("answer"), false
						return true
					case "error", "cancelled":
						st.finished, st.busy = true, false
						st.err = run.S("error")
						if run["error"] == nil {
							st.err = l10n.StudyRecognizeFailed
						}
						return true
					}
					return false
				})
				if done {
					return
				}
				time.Sleep(700 * time.Millisecond)
			}
		}()
		return st
	})
	leave := func(v any) {
		st.stopped = true
		if !st.finished && st.run != "" {
			id := st.run
			go func() { _, _ = State.Api().CancelCanvasRun(id) }()
		}
		r.Pop(v)
	}
	title := l10n.StudyRecognizeReview
	if st.busy {
		title = l10n.StudyRecognizeWorking
	}
	box.Children(func() {
		m3.AlertDialog(c, m3.Alert{Title: title, Width: 520, Content: func() {
			switch {
			case st.busy:
				m3.Linear(c, m3.Progress{})
			case st.err != "":
				m3.Text(c, st.err, design.TT().BodyMedium.C(design.S().OnSurface))
			default:
				m3.Field{MinLines: 5, MaxLines: 12, MaxLength: 200000}.Show(c, &st.text)
			}
		}, Actions: func() {
			if m3.TextButton(c, l10n.CommonCancel).Clicked() {
				leave(nil)
			}
			if (m3.Button{Kind: m3.Filled, Label: l10n.StudyRecognizeInsert, Disabled: st.busy || st.err != ""}).Show(c).Clicked() {
				leave(st.text)
			}
		}})
	})
	if st.busy {
		c.After(250 * time.Millisecond)
	}
}

// prepareSelectionSnapshot is prepareSelectionSnapshotWait for the UI
// thread: done hears the screenshot there.
func (s *StudyCanvas) prepareSelectionSnapshot(done func(*studySnapshot, error)) {
	core.Async(s.prepareSelectionSnapshotWait, done)
}

// prepareSelectionSnapshotWait takes the selection's screenshot, with the
// frames of its videos in place, as one immutable attachment.
func (s *StudyCanvas) prepareSelectionSnapshotWait() (*studySnapshot, error) {
	var d *study.Document
	var region study.Rect
	var polygon []study.Pt
	var zoom float64
	var camera study.Pt
	var edit int
	var query core.J
	var videos []*study.Card
	problem := onUIValue(func() error {
		d = s.doc
		if d == nil || s.regionSelection == nil {
			return formatError(l10n.StudySelectionEmpty)
		}
		region, polygon = *s.regionSelection, append([]study.Pt(nil), s.selectionPolygon...)
		zoom, camera, edit = d.Zoom, d.Camera, d.Edit
		query = s.selectedQuery()
		var objects []any
		pages, times := map[string]any{}, map[string]any{}
		lecture := false
		for _, c := range d.Cards {
			if c.Type == "conversation" || !d.Visible(c.Layer) || !c.Rect.Overlaps(region) {
				continue
			}
			objects = append(objects, c.ID)
			switch c.Type {
			case "pdfdoc":
				pages[c.ID] = c.Page
			case "lecture":
				lecture = true
			case "video":
				times[c.ID] = s.videoTimes[c.ID]
			}
			if !c.Collapsed && (c.Type == "video" || c.Type == "lecture") {
				videos = append(videos, c)
			}
		}
		query["objects"], query["object_pages"], query["include_video"], query["object_video_ms"] = nonNilList(objects), pages, lecture, times
		return nil
	})
	if problem != nil {
		return nil, problem
	}
	check := func() error {
		return onUIValue(func() error {
			if s.disposed || s.doc != d || d.Edit != edit || d.Zoom != zoom || d.Camera != camera || s.regionSelection == nil || *s.regionSelection != region {
				return formatError(l10n.StudySelectionChanged)
			}
			return nil
		})
	}
	sources, err := s.selectionSources(query)
	if err != nil {
		return nil, err
	}
	if err := check(); err != nil {
		return nil, err
	}
	defer onUI(func() {
		clear(s.snapshotFrames)
		s.snapshotting = false
	})
	api := State.Api()
	// The canvas raster cannot hold a video's frames: the displayed video
	// is replaced by its frame for the capture, under the ink.
	for _, c := range videos {
		ms := core.Int(query["video_ms"])
		if c.Type != "lecture" {
			ms = core.Int(core.Obj(query["object_video_ms"])[c.ID])
		}
		out, err := api.PrepareStudyMedia(s.lectureID(), core.J{"object_id": c.ID, "from_ms": ms, "to_ms": ms + 1, "frame": true, "transcribe": false})
		if err != nil {
			return nil, err
		}
		task := core.Obj(out).S("id")
		err = func() error {
			defer func() { _, _ = api.CancelStudyMedia(s.lectureID(), task) }()
			deadline := time.Now().Add(150 * time.Second)
			for {
				if err := check(); err != nil {
					return err
				}
				out, err := api.StudyMediaTask(s.lectureID(), task)
				if err != nil {
					return err
				}
				cur := core.Obj(out)
				if cur.S("state") == "done" {
					file := cur.O("result").S("image_path")
					if file == "" {
						return formatError(l10n.StudySnapshotVideoFailed)
					}
					data, err := api.StudyImageBytes(file)
					if err != nil {
						return err
					}
					bmp, err := ui.DecodeBitmap(data)
					if err != nil {
						return err
					}
					if err := check(); err != nil {
						return err
					}
					onUI(func() { s.snapshotFrames[c.ID] = bmp })
					return nil
				}
				if cur.S("state") != "running" || time.Now().After(deadline) {
					return formatError(l10n.StudySnapshotVideoFailed)
				}
				time.Sleep(250 * time.Millisecond)
			}
		}()
		if err != nil {
			return nil, err
		}
	}
	if err := check(); err != nil {
		return nil, err
	}
	onUI(func() { s.snapshotting = true })
	data, err := s.regionImage(region, polygon)
	if err != nil {
		return nil, err
	}
	if err := check(); err != nil {
		return nil, err
	}
	if len(data) > 3*1024*1024 {
		return nil, formatError(l10n.StudySnapshotTooLarge)
	}
	poly := make([]any, len(polygon))
	for i, p := range polygon {
		poly[i] = []any{p.X, p.Y}
	}
	anchors := make([]any, len(sources))
	for i, a := range sources {
		anchors[i] = a.JSON()
	}
	return &studySnapshot{bytes: data, selection: core.J{"bounds": []any{region.L, region.T, region.W(), region.H()}, "polygon": poly, "sources": anchors}}, nil
}

// snapshotVideo is a video card's view, its prepared frame in place during
// a capture.
func (s *StudyCanvas) snapshotVideo(c *ui.Context, card *study.Card, child func()) {
	if s.rendering {
		ui.Box(c).Fill().Background(ui.RGB(0, 0, 0)).Children(func() {
			if bmp := s.snapshotFrames[card.ID]; bmp != nil && s.snapshotting {
				ui.Image(c, bmp).Fit(ui.Contain).Absolute().Top(0).Left(0).Right(0).Bottom(0)
			}
		})
		return
	}
	child()
}

// snapshotDragHandle attaches the selection's screenshot to the
// conversation: dragged to the composer, or tapped.
func (s *StudyCanvas) snapshotDragHandle(c *ui.Context) {
	busy := s.selectionBusy || s.snapshotPreparing
	box := ui.Box(c.Key("study-snapshot-attach"))
	if !busy {
		box = box.Drag(s.selectionDrop())
	}
	if box.Dragging() && !s.snapshotPreparing && s.pendingDrop != nil && !s.pendingDrop.started {
		s.startSelectionDrop(s.pendingDrop)
	}
	box.Children(func() {
		ic := icons.AddPhotoAlternateOutlined
		e := m3.IconButton{Icon: ic, Size: 20, Tooltip: l10n.StudySnapshotDrag, Disabled: busy}
		if s.snapshotPreparing {
			readingOrb(c, 10).Margin(13)
			return
		}
		// A tap attaches it too, for those who do not drag.
		if e.Show(c).Clicked() && !busy {
			s.selectionJob(func() error {
				onUI(func() { s.snapshotPreparing = true })
				defer onUI(func() { s.snapshotPreparing = false })
				drop := &studySelectionDrop{done: make(chan struct{})}
				drop.snap, drop.err = s.prepareSelectionSnapshotWait()
				close(drop.done)
				s.acceptSnapshotWait(drop)
				onUI(func() { s.stageOpen = true })
				return nil
			})
		}
	})
}

// selectionDrop is the drop a drag of the handle carries; its screenshot
// starts once the drag does.
func (s *StudyCanvas) selectionDrop() *studySelectionDrop {
	if s.pendingDrop == nil || s.pendingDrop.started {
		s.pendingDrop = &studySelectionDrop{done: make(chan struct{})}
	}
	return s.pendingDrop
}

func (s *StudyCanvas) startSelectionDrop(drop *studySelectionDrop) {
	drop.started = true
	s.snapshotPreparing = true
	go func() {
		drop.snap, drop.err = s.prepareSelectionSnapshotWait()
		close(drop.done)
		// Kept even when the drag ends outside the composer.
		onUI(func() {
			s.snapshotPreparing = false
			if drop.err != nil {
				s.toast(drop.err.Error())
			} else {
				s.stageOpen = true
			}
		})
	}()
}

func (s *StudyCanvas) acceptSnapshot(drop *studySelectionDrop) {
	if !drop.started {
		s.startSelectionDrop(drop)
	}
	go s.acceptSnapshotWait(drop)
}

func (s *StudyCanvas) acceptSnapshotWait(drop *studySelectionDrop) {
	<-drop.done
	if drop.err != nil {
		onUI(func() { s.toast(drop.err.Error()) })
		return
	}
	var cv *studyConversation
	onUI(func() {
		if s.disposed {
			return
		}
		if cv = s.activeStage(); cv == nil || cv.data == nil {
			cv = nil
			s.startConversation(conversationStart{snapshot: drop.snap}, nil)
		}
	})
	if cv != nil {
		cv.attachSnapshot(drop.snap)
	}
}
