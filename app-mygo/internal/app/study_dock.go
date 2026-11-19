package app

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/study"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// study_edge_dock.dart and study_pen_palette.dart: toolbars that dock to an
// edge of the canvas and move there by their handle, and the pen dock.
// Interaction reference: Notein 1.2.607.0; see docs/drawing-controls.md.

type dockEdge int

const (
	dockTop dockEdge = iota
	dockRight
	dockBottom
	dockLeft
)

var dockEdgeNames = []string{"top", "right", "bottom", "left"}

// studyDockMargin is the space between a dock and its window edge.
const studyDockMargin = design.SpaceSM

// dockPlacement is StudyDockPlacement: an edge and where along it.
type dockPlacement struct {
	edge   dockEdge
	offset float64
}

func (p dockPlacement) vertical() bool { return p.edge == dockLeft || p.edge == dockRight }

func (p dockPlacement) json() map[string]any {
	return map[string]any{"edge": dockEdgeNames[p.edge], "offset": p.offset}
}

func parseDockPlacement(data any, fallback dockPlacement) dockPlacement {
	m, ok := data.(map[string]any)
	if !ok {
		return fallback
	}
	edge := -1
	for i, n := range dockEdgeNames {
		if m["edge"] == n {
			edge = i
		}
	}
	off, ok := m["offset"].(float64)
	if edge < 0 || !ok || math.IsNaN(off) || math.IsInf(off, 0) {
		return fallback
	}
	return dockPlacement{dockEdge(edge), clampF(off, 0, 1)}
}

// studyDockRect prefers the requested position, then slides along the
// edge; when the edge is full, it takes another lane further in. It keeps
// out of the corners and the zoom control.
func studyDockRect(size, child study.Pt, p dockPlacement, occupied []study.Rect, inset float64) study.Rect {
	freeX, freeY := math.Max(0, size.X-child.X), math.Max(0, size.Y-child.Y)
	margin := float64(studyDockMargin)
	left := math.Min(freeX, inset+12)
	if p.vertical() {
		left = math.Min(freeX, margin)
	}
	rightEdge := 12.0
	if p.vertical() {
		rightEdge = margin
	}
	right := math.Max(left, freeX-rightEdge)
	top := math.Min(freeY, margin)
	if p.vertical() {
		top = math.Min(freeY, 64)
	}
	bottomEdge := margin
	if p.vertical() {
		bottomEdge = 64
	}
	bottom := math.Max(top, freeY-bottomEdge)
	low, high, length := left, right, child.X
	if p.vertical() {
		low, high, length = top, bottom, child.Y
	}
	desired := low + (high-low)*p.offset
	positions := []float64{desired, low, high}
	for _, r := range occupied {
		if p.vertical() {
			positions = append(positions, clampF(r.T-length-8, low, high), clampF(r.B+8, low, high))
		} else {
			positions = append(positions, clampF(r.L-length-8, low, high), clampF(r.R+8, low, high))
		}
	}
	seen := map[float64]bool{}
	var sorted []float64
	for _, v := range positions {
		if !seen[v] {
			seen[v] = true
			sorted = append(sorted, v)
		}
	}
	sort.SliceStable(sorted, func(i, j int) bool { return math.Abs(sorted[i]-desired) < math.Abs(sorted[j]-desired) })
	var fallback *study.Rect
	for lane := 0; lane <= len(occupied); lane++ {
		thick := child.Y
		if p.vertical() {
			thick = child.X
		}
		shift := float64(lane) * (thick + 8)
		for _, along := range sorted {
			var at study.Pt
			switch p.edge {
			case dockTop:
				at = study.Pt{X: along, Y: clampF(top+shift, 0, freeY)}
			case dockBottom:
				at = study.Pt{X: along, Y: clampF(bottom-shift, 0, freeY)}
			case dockLeft:
				at = study.Pt{X: clampF(left+shift, 0, freeX), Y: along}
			default:
				at = study.Pt{X: clampF(right-shift, 0, freeX), Y: along}
			}
			rect := study.RectLTWH(at.X, at.Y, child.X, child.Y)
			if fallback == nil {
				fallback = rectPtr(rect)
			}
			free := true
			for _, o := range occupied {
				if o.Inflate(4).Overlaps(rect) {
					free = false
					break
				}
			}
			if free {
				return rect
			}
		}
	}
	return *fallback
}

// edgeDock is StudyEdgeDock's options.
type edgeDock struct {
	id, label        string
	placement        dockPlacement
	children         func(vertical bool)
	trailing         func()
	thickness, scale float64
	centerHandle     bool
	startInset       float64
	dragIcon         icons.Icon
	showHandle       bool
}

// dockState is a dock's drag: where its centre floats while held, the
// placement it will snap to, and its animations.
type dockState struct {
	center    *study.Pt
	target    *dockPlacement
	snapFrom  study.Rect
	snapStart time.Time
	snapping  bool
	bloom     time.Time
	size      study.Pt
	rect      study.Rect
}

func (s *StudyCanvas) dockGroup(c *ui.Context, topInset float64) {
	docks := []edgeDock{s.penDock()}
	if !s.stageOpen {
		docks = append(docks, s.aiDock())
	}
	// All dock sizes are measured together, so collapsing, rotating and
	// resizing cannot make independently placed controls cover one another.
	var occupied []study.Rect
	if topInset > 0 {
		occupied = append(occupied, study.RectLTWH(0, 0, s.viewport.X, topInset-4))
	}
	if s.viewport.X >= 600 {
		occupied = append(occupied, study.RectLTWH(s.viewport.X-196, s.viewport.Y-68, 196, 68))
	}
	for _, dk := range docks {
		st := s.docks[dk.id]
		if st == nil {
			st = &dockState{}
			s.docks[dk.id] = st
		}
		placement := dk.placement
		if st.target != nil {
			placement = *st.target
		}
		docked := studyDockRect(s.viewport, st.size, placement, occupied, dk.startInset)
		rect := docked
		if st.center != nil {
			rect = study.RectCenter(*st.center, st.size.X, st.size.Y)
			if st.snapping {
				t := float64(time.Since(st.snapStart)) / float64(design.Medium1)
				if t >= 1 {
					// Settled: the dock opens out from the edge.
					target := *st.target
					st.center, st.target, st.snapping = nil, nil, false
					st.bloom = time.Now()
					s.moveDock(dk.id, target)
					rect = docked
				} else {
					c.AnimationFrame()
					e := float64(design.EmphasizedDecelerate(f32(t)))
					rect = study.Rect{L: st.snapFrom.L + (docked.L-st.snapFrom.L)*e, T: st.snapFrom.T + (docked.T-st.snapFrom.T)*e,
						R: st.snapFrom.R + (docked.R-st.snapFrom.R)*e, B: st.snapFrom.B + (docked.B-st.snapFrom.B)*e}
				}
			}
		}
		if st.center == nil {
			occupied = append(occupied, rect)
		}
		st.rect = rect
		s.edgeDock(c, dk, st, rect)
	}
}

func (s *StudyCanvas) dockMoveTo(st *dockState, center study.Pt) {
	c := study.Pt{X: clampF(center.X, 26, math.Max(26, s.viewport.X-26)), Y: clampF(center.Y, 26, math.Max(26, s.viewport.Y-26))}
	dist := []float64{c.Y, s.viewport.X - c.X, s.viewport.Y - c.Y, c.X}
	edge := 0
	for i, v := range dist {
		if v < dist[edge] {
			edge = i
		}
	}
	p := dockPlacement{edge: dockEdge(edge)}
	if p.vertical() {
		p.offset = clampF(c.Y/math.Max(1, s.viewport.Y), 0, 1)
	} else {
		p.offset = clampF(c.X/math.Max(1, s.viewport.X), 0, 1)
	}
	st.center, st.target = ptPtr(c), &p
}

func (s *StudyCanvas) edgeDock(c *ui.Context, dk edgeDock, st *dockState, rect study.Rect) {
	floating := st.center != nil
	vertical := dk.placement.vertical()
	centered := dk.centerHandle && !vertical
	ch := studyChrome()
	measured := st.size.X > 0
	outer := ui.Box(c.Key("study-dock-" + dk.id)).Absolute().Left(f32(rect.L)).Top(f32(rect.T))
	if !measured {
		outer = outer.Opacity(0)
		c.Invalidate()
	}
	// Pickup grows from the finger; the docked form from its edge.
	form := 1.0
	if !st.bloom.IsZero() && !design.T().ReduceMotion {
		t := float64(time.Since(st.bloom)) / float64(design.Medium2)
		if t < 1 {
			c.AnimationFrame()
			form = float64(design.EmphasizedDecelerate(f32(t)))
		} else {
			st.bloom = time.Time{}
		}
	}
	if form < 1 {
		from := .86
		if floating {
			from = .7
		}
		sc := f32(from + (1-from)*form)
		outer = outer.Opacity(f32(.4+.6*form)).Scale(sc, sc)
	}
	// The whole dock moves by a drag; its buttons keep their taps.
	if dx, dy, ok := outer.Dragged(); ok && (dx != 0 || dy != 0) {
		if st.center == nil {
			st.snapping = false
			st.bloom = time.Now()
			s.dockMoveTo(st, rect.Center())
		}
		s.dockMoveTo(st, st.center.Add(ptOf(dx, dy)))
	}
	if _, _, ok := outer.DragEnded(); ok && st.center != nil && !st.snapping {
		st.snapping, st.snapStart = true, time.Now()
		st.snapFrom = study.RectCenter(*st.center, st.size.X, st.size.Y)
	} else if outer.PressCanceled() && st.center != nil {
		st.center, st.target, st.snapping = nil, nil, false
		st.bloom = time.Now()
	}
	radius := f32(dk.thickness / 2)
	if floating {
		radius = 26
	} else if dk.centerHandle {
		radius = design.CornerLG
	}
	outer.Children(func() {
		// The pane is made inside the dock, so it lays out in it.
		pane := ui.Box(c.Key("study-dock-surface-" + dk.id))
		widgets.Glass{Radius: radius, Selected: floating}.Apply(pane)
		if floating {
			pane.Size(52, 52).Center().Children(func() { m3.Icon(c, dk.dragIcon, 24, design.S().OnSurface) })
			return
		}
		thick := f32(dk.thickness * dk.scale)
		extent := 0.0
		if vertical {
			extent = math.Max(1, s.viewport.Y-math.Max(64, s.topBarsBottom()+8)-64)
			pane = pane.Width(thick).MaxHeight(f32(extent))
		} else {
			extent = math.Max(1, s.viewport.X-dk.startInset-24)
			pane = pane.Height(thick).MaxWidth(f32(extent))
		}
		if b := pane.Bounds(); b.W > 0 {
			st.size = study.Pt{X: float64(b.W), Y: float64(b.H)}
		}
		s.recordControl(pane)
		strip := func() {
			flex := ui.Row(c).AlignItems(ui.Center)
			if vertical {
				flex = ui.Column(c).AlignItems(ui.Center)
			}
			flex.Shrink(1).Children(func() {
				if dk.showHandle && !centered {
					s.dockHandle(c, dk, vertical, false)
				}
				var scroll ui.Element
				if vertical {
					scroll = ui.Scroll(c)
				} else {
					scroll = ui.ScrollHorizontal(c)
				}
				scroll.Shrink(1).Scrollbars(false).Children(func() {
					inner := ui.Row(c).AlignItems(ui.Center)
					if vertical {
						inner = ui.Column(c).AlignItems(ui.Center)
					}
					inner.Children(func() { dk.children(vertical) })
				})
				if dk.trailing != nil {
					dk.trailing()
				}
			})
		}
		pane.Children(func() {
			if centered && dk.showHandle {
				ui.Column(c).AlignItems(ui.Center).Children(func() {
					s.dockHandle(c, dk, vertical, true)
					ui.Box(c).Shrink(1).Children(strip)
				})
				return
			}
			strip()
		})
		_ = ch
	})
}

func (s *StudyCanvas) dockHandle(c *ui.Context, dk edgeDock, vertical, centered bool) {
	ch := studyChrome()
	w, h := float32(24), f32(dk.thickness)
	if vertical {
		w, h = f32(dk.thickness), 24
	}
	if centered {
		w, h = 96, 16
	}
	e := ui.Box(c.Key("study-dock-handle-"+dk.id)).Size(w, h).Center().Cursor(ui.CursorGrab)
	m3.Tip(c, e, "拖动"+dk.label+"，松手贴边")
	e.Children(func() {
		if centered {
			ui.Box(c).Size(36, 4).Radius(3).Background(ch.inkFaint)
			return
		}
		ic := icons.MoreVert
		if vertical {
			ic = icons.DragHandle
		}
		m3.Icon(c, ic, 16, ch.inkFaint)
	})
}

func (s *StudyCanvas) aiDock() edgeDock {
	return edgeDock{id: "ai", label: "AI 入口", dragIcon: icons.AutoAwesomeOutlined, placement: s.aiPlacement, thickness: 52, scale: 1,
		children: func(bool) {
			c := s.ctx
			if (m3.IconButton{Icon: icons.AutoAwesomeOutlined, Tooltip: "打开 AI 对话"}).Show(c).Key("study-ai-tab").Clicked() {
				s.ask()
			}
		}}
}

// action is _action: a canvas tool button; a chosen tool sits in a tonal
// pill (M3 selected icon button).
func (s *StudyCanvas) action(c *ui.Context, key, label string, ic icons.Icon, disabled, selected, compact bool) bool {
	cs := design.S()
	box, size := float32(40), float32(22)
	if compact {
		box, size = 32, 20
	}
	b := m3.IconButton{Icon: ic, Tooltip: label, Disabled: disabled, Box: box, Size: size, Color: cs.OnSurface}
	if selected {
		b.BG, b.Color = cs.SecondaryContainer, cs.OnSecondaryContainer
	}
	if disabled {
		b.Color = cs.OnSurface.Alpha(.38)
	}
	e := b.Show(c)
	if key != "" {
		e = e.Key(key)
	}
	return e.Clicked() && !disabled
}

func (s *StudyCanvas) divider(c *ui.Context, vertical, compact bool) {
	gap := float32(16)
	if compact {
		gap = 8
	}
	col := design.S().OnSurface.Alpha(.14)
	if vertical {
		ui.Box(c).Size(28, gap).Center().Children(func() { ui.Box(c).Size(28, 1).Background(col) })
	} else {
		ui.Box(c).Size(gap, 28).Center().Children(func() { ui.Box(c).Size(1, 28).Background(col) })
	}
}

var quickColors = []uint32{0xff26354a, 0xff6254d7, 0xffe65365, 0xff7d5f4c, 0xff9a68ce, 0xff2382ce, 0xff2d8986, 0xffffd34f, 0xffea7b3c, 0xffffffff}

// penRotation turns the pens to point at the canvas from their edge.
func (s *StudyCanvas) penRotation() int {
	switch s.penPlacement.edge {
	case dockLeft:
		return 1
	case dockRight:
		return 3
	case dockTop:
		return 2
	}
	return 0
}

// palettePen is _palettePen: a pen of the dock, raised when chosen.
func (s *StudyCanvas) palettePen(c *ui.Context, key, label string, vertical, selected bool, paint func(p *ui.Painter, r ui.Rect), pick, settings func()) {
	w, h := float32(24), float32(38)
	if vertical {
		w, h = 36, 24
	}
	e := ui.Box(c.Key(key)).Size(w, h).Role(ui.RoleButton).Label(label).Selected(selected).Cursor(ui.CursorPointer)
	m3.Tip(c, e, label)
	if e.Clicked() {
		pick()
	}
	if settings != nil && (e.LongPressed() || e.RightClicked()) {
		settings()
	}
	if b := e.Bounds(); b.W > 0 {
		s.penItems[key] = b
	}
	turns := s.penRotation()
	lift := e.Animate("lift", map[bool]float32{true: 0, false: .1}[selected], design.Short3)
	e.Children(func() {
		iw, ih := w, h
		if turns%2 == 1 {
			iw, ih = h, w
		}
		ui.Box(c).Absolute().Left((w-iw)/2).Top((h-ih)/2).Size(iw, ih).Rotate(float32(90 * turns)).Children(func() {
			ui.Box(c).Fill().Padding(0, 1).Translate(0, lift*ih).Draw(paint)
		})
	})
}

func (s *StudyCanvas) penDock() edgeDock {
	thickness := 42.0
	if s.penPlacement.vertical() {
		thickness = 36
	}
	return edgeDock{id: "pens", label: "笔盒", dragIcon: icons.DrawOutlined, scale: 1.15, placement: s.penPlacement, thickness: thickness,
		centerHandle: true, showHandle: true,
		trailing: func() {
			c := s.ctx
			label, ic := "收起笔盒", icons.KeyboardArrowDown
			if s.dockSmall {
				label, ic = "展开笔盒", icons.DrawOutlined
			}
			if s.action(c, "", label, ic, false, false, true) {
				s.dockSmall = !s.dockSmall
				s.saveLayout()
			}
		},
		children: func(vertical bool) {
			c := s.ctx
			if s.dockSmall {
				return
			}
			if s.action(c, "", "画笔与橡皮设置", icons.PaletteOutlined, false, false, true) {
				s.advancedPenSettings(s.tool == toolEraser, 0)
			}
			for i := range s.presets {
				i := i
				key := fmt.Sprintf("study-pen-%d", i)
				if i == 0 {
					key = "study-tool-pen"
				}
				pen := s.presets[i]
				s.palettePen(c, key, pen.Name+"：点选，再点设置", vertical, i == s.presetIndex && s.ink(),
					func(p *ui.Painter, r ui.Rect) { paintPalettePen(p, r, pen.Color, pen.Brush.Family) },
					func() {
						if i == s.presetIndex && s.ink() && !s.wholeCardSelected() {
							s.penSettings(false)
						} else {
							s.pickPen(i)
						}
					},
					func() {
						s.pickPen(i)
						s.penSettings(false)
					})
			}
			s.palettePen(c, "study-tool-eraser", "橡皮：再点设置", vertical, s.tool == toolEraser,
				func(p *ui.Painter, r ui.Rect) { paintPalettePen(p, r, 0xffef8c95, "eraser") },
				func() { s.chooseTool(toolEraser) }, func() { s.penSettings(true) })
			s.palettePen(c, "study-tool-lasso", "套索", vertical, s.tool == toolRegion,
				func(p *ui.Painter, r ui.Rect) { paintPalettePen(p, r, 0xff8f949c, "lasso") },
				func() { s.chooseTool(toolRegion) }, nil)
			s.divider(c, vertical, true)
			if s.tool == toolRegion {
				if s.action(c, "", "自由套索", icons.Gesture, false, !s.lassoRectangle && !s.captureLasso, true) {
					s.lassoRectangle, s.captureLasso = false, false
				}
				if s.action(c, "", "矩形框选", icons.CropFree, false, s.lassoRectangle && !s.captureLasso, true) {
					s.lassoRectangle, s.captureLasso = true, false
				}
				if s.action(c, "", l10n.StudyCaptureLasso, icons.Screenshot, false, s.captureLasso, true) {
					s.lassoRectangle, s.captureLasso = true, true
				}
				if s.action(c, "", l10n.StudySelectionFilters, icons.FilterAltOutlined, false, false, true) {
					s.editSelectionFilters()
				}
				if s.action(c, "", "所选区域问 AI", icons.AutoAwesomeOutlined, false, false, true) {
					s.ask()
				}
				return
			}
			ch := studyChrome()
			grid := ui.Row(c).Wrap().Width(92).Height(34).Gap(3).GapY(2)
			if vertical {
				grid = ui.Column(c).Wrap().Width(34).Height(92).Gap(3).GapX(2)
			}
			grid.Children(func() {
				for _, col := range quickColors {
					col := col
					e := ui.Box(c.Key(fmt.Sprintf("study-pen-color-%d", col))).Size(16, 16).Radius(4).Background(design.ARGB(col)).
						Center().Cursor(ui.CursorPointer)
					if col == 0xffffffff {
						e = e.Border(1, ch.line)
					}
					m3.Tip(c, e, fmt.Sprintf("切换笔色 #%x", col&0xffffff))
					if e.Clicked() {
						s.quickPen(&col, nil)
					}
					if e.LongPressed() || e.RightClicked() {
						s.advancedPenSettings(false, 1)
					}
					e.Children(func() {
						if s.color == col && s.ink() {
							fg := ui.RGB(255, 255, 255)
							if design.Luminance(design.ARGB(col)) > .55 {
								fg = ui.RGB(0, 0, 0)
							}
							m3.Icon(c, icons.Check, 13, fg)
						}
					})
				}
			})
			widths := []float64{1, 3, 6}
			if s.brush.Family == "highlighter" {
				widths = []float64{12, 22, 32}
			}
			for _, w := range widths {
				w := w
				h := float32(34)
				if vertical {
					h = 30
				}
				sel := s.ink() && math.Abs(s.penWidth-w) < .05
				e := ui.Row(c).Size(36, h).AlignItems(ui.Center).Justify(ui.Center)
				if e.LongPressed() || e.RightClicked() {
					s.penSettings(false)
				}
				e.Children(func() {
					b := m3.IconButton{Tooltip: fmt.Sprintf("笔宽 %d", int(w)), Box: 24, Child: func(ui.Color) {
						border := ch.line
						if sel {
							border = ch.accent
						}
						ui.Box(c).Size(20, 20).Radius(10).Border(1.5, border).Center().Children(func() {
							ui.Box(c).Size(17, f32(math.Min(9, w+1))).Background(ch.ink).Rotate(-.65 * 180 / math.Pi)
						})
					}}.Show(c)
					if b.Clicked() {
						s.quickPen(nil, &w)
					}
					m3.Text(c, fmt.Sprint(int(w)), design.TextStyle{Size: 9, Height: 1.1, Color: ch.inkSoft})
				})
			}
		}}
}

// paintPalettePen is _PalettePenPainter: original vector artwork drawn for
// this app; no APK bitmaps or paths are used.
func paintPalettePen(p *ui.Painter, r ui.Rect, color uint32, family string) {
	scale := float32(math.Min(float64(r.W/32), float64(r.H/90)))
	ox, oy := r.X+(r.W-32*scale)/2, r.Y+r.H-90*scale
	tr := func(x, y float32) (float32, float32) { return ox + x*scale, oy + y*scale }
	rect := func(x, y, w, h float32) ui.Rect {
		x0, y0 := tr(x, y)
		return ui.Rect{X: x0, Y: y0, W: w * scale, H: h * scale}
	}
	poly := func(pts ...float32) *ui.Path {
		path := &ui.Path{}
		for i := 0; i < len(pts); i += 2 {
			x, y := tr(pts[i], pts[i+1])
			if i == 0 {
				path.MoveTo(x, y)
			} else {
				path.LineTo(x, y)
			}
		}
		path.Close()
		return path
	}
	ink := design.ARGB(color)
	shade := ui.LinearGradient{Angle: 90, Stops: []ui.GradientStop{{Color: design.ARGB(0xffb9bcc3), At: 0}, {Color: ui.RGB(255, 255, 255), At: .35},
		{Color: design.ARGB(0xffd8dbe0), At: .75}, {Color: design.ARGB(0xffa0a5af), At: 1}}}
	barrel := rect(7, 39, 18, 58)
	if family != "pencil" && family != "fountain" {
		p.FillGradient(barrel, shade, 5*scale)
	}
	switch family {
	case "eraser":
		p.FillGradient(rect(7, 18, 18, 31), ui.LinearGradient{From: design.ARGB(0xfff4b3b7), To: design.ARGB(0xffdf7e89), Angle: 90}, 7*scale)
		p.Fill(rect(7, 46, 18, 5), design.ARGB(0xffb4b6be), 0)
	case "highlighter":
		p.Fill(rect(8, 24, 16, 20), ink.Alpha(.7), 5*scale)
		p.FillPath(poly(10, 24, 11, 15, 21, 12, 22, 24), ink)
		p.Fill(rect(7, 65, 18, 10), ink, 0)
	case "pencil":
		// A faceted wooden pencil stays distinct even at the narrow dock size.
		p.Fill(rect(5, 37, 22, 60), design.ARGB(0xffe5aa31), 0)
		p.Fill(rect(11, 37, 10, 60), design.ARGB(0xffffcf65), 0)
		p.FillPath(poly(16, 5, 27, 39, 20, 35, 12, 40, 5, 39), design.ARGB(0xffdcb897))
		p.FillPath(poly(16, 5, 19, 19, 13, 19), design.ARGB(0xff292d35))
		p.Fill(rect(5, 72, 22, 9), ink, 0)
	case "fountain":
		p.Fill(barrel, ink, 5*scale)
		p.Fill(rect(10, 50, 3, 40), ui.RGBA(255, 255, 255, .38), 1.5*scale)
		gold := ui.LinearGradient{Angle: 90, Stops: []ui.GradientStop{{Color: design.ARGB(0xffb68230), At: 0}, {Color: design.ARGB(0xffffe7a0), At: .5},
			{Color: design.ARGB(0xffc19543), At: 1}}}
		p.FillPathGradient(poly(16, 3, 29, 28, 22, 47, 10, 47, 3, 28), gold)
		x0, y0 := tr(16, 5)
		x1, y1 := tr(16, 35)
		p.Line(x0, y0, x1, y1, 1.8*scale, design.ARGB(0xff343942))
		cx, cy := tr(16, 29)
		p.Fill(ui.Rect{X: cx - 2.7*scale, Y: cy - 2.7*scale, W: 5.4 * scale, H: 5.4 * scale}, design.ARGB(0xff343942), 2.7*scale)
		p.FillGradient(rect(7, 47, 18, 5), gold, 0)
		p.FillGradient(rect(7, 84, 18, 3), gold, 0)
	case "lasso":
		p.FillPath(poly(16, 9, 23, 39, 9, 39), design.ARGB(0xff454a53))
		for y := float32(20); y < 40; y += 5 {
			x0, y0 := tr(10, y+4)
			x1, y1 := tr(22, y)
			p.Line(x0, y0, x1, y1, 2*scale, ui.RGBA(255, 255, 255, .7))
		}
		p.Stroke(rect(12, 54, 10, 15), design.ARGB(0xff666b73), 5*scale, 1.2*scale)
	default:
		tip := float32(13)
		if family == "brush" {
			tip = 7
		}
		path := &ui.Path{}
		x, y := tr(16, tip)
		path.MoveTo(x, y)
		cx, cy := tr(20, 28)
		x, y = tr(21, 43)
		path.QuadTo(cx, cy, x, y)
		x, y = tr(11, 43)
		path.LineTo(x, y)
		cx, cy = tr(12, 26)
		x, y = tr(16, tip)
		path.QuadTo(cx, cy, x, y)
		path.Close()
		p.FillPath(path, ink)
		p.Fill(rect(7, 72, 18, 7), ink, 0)
	}
}
