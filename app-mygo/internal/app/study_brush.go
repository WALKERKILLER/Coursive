package app

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/study"
)

// study_brush_ui.dart, the pen popover of study_pen_palette.dart and the
// layout preferences of study_conversation_ui.dart.

var brushFamilies = []string{"ballpoint", "fountain", "pencil", "brush", "highlighter"}

var brushNames = map[string]string{"ballpoint": "圆珠笔", "fountain": "钢笔", "pencil": "铅笔", "brush": "软笔", "highlighter": "荧光笔"}

func isBrushName(name string) bool {
	for _, n := range brushNames {
		if n == name {
			return true
		}
	}
	return false
}

var penPalette = []uint32{
	0xff26354a, 0xff000000, 0xff566473, 0xff9aa4b0, 0xffe65365, 0xffb72c45, 0xffea7b3c, 0xffffb449,
	0xffffd34f, 0xffe4e99a, 0xffb1d75c, 0xff3ba97e, 0xff2d8986, 0xff43c9d7, 0xff2382ce, 0xff3c5bb3,
	0xff6254d7, 0xff9a68ce, 0xffc964b3, 0xfff2a7ca, 0xffb38e72, 0xff7d5f4c, 0xffffffff, 0xffcbd4e4,
}

var selectionKindNames = []string{"ink", "text", "document", "video", "other"}

func (s *StudyCanvas) loadBrushSettings() {
	if kinds, ok := core.Prefs.Strings("study.selection.kinds"); ok {
		clear(s.selectionKinds)
		for i, k := range study.SelectionKinds {
			for _, name := range kinds {
				if name == selectionKindNames[i] {
					s.selectionKinds[k] = true
				}
			}
		}
	}
	raw, ok := core.Prefs.String("study.brush-presets.v1")
	if !ok || len(raw) > 32000 {
		return
	}
	v, err := core.Decode([]byte(raw))
	data := plainMap(v)
	if err != nil || data == nil {
		return
	}
	var values []study.PenPreset
	for i, p := range listOf(data["presets"]) {
		m, ok := p.(map[string]any)
		if i >= 20 || !ok {
			break
		}
		pen := study.PenPresetFrom(m)
		if isBrushName(pen.Name) {
			pen.Name = brushNames[pen.Brush.Family]
		}
		values = append(values, pen)
	}
	if len(values) == 0 {
		return
	}
	num := func(k string, def float64) float64 {
		if f, ok := data[k].(float64); ok {
			return f
		}
		return def
	}
	s.presets = values
	s.presetIndex = max(0, min(int(num("selected", 0)), len(values)-1))
	s.eraserSize = clampF(num("eraser_size", 18), 3, 100)
	s.pressureEraser = data["pressure_eraser"] != false
	s.predictionEnabled = data["prediction"] != false
	s.penButtons = data["pen_buttons"] != false
	s.pressureThreshold = clampF(num("pressure_threshold", .8), .55, .98)
	s.eraserMode = "partial"
	if m, _ := data["eraser_mode"].(string); m == "stroke" || m == "lasso" || m == "partial" {
		s.eraserMode = m
	}
	pen := s.presets[s.presetIndex]
	s.brush, s.color, s.penWidth = pen.Brush, pen.Color, pen.Width
}

func (s *StudyCanvas) saveBrushSettings() {
	p := &s.presets[s.presetIndex]
	if isBrushName(p.Name) {
		p.Name = brushNames[s.brush.Family]
	}
	p.Color, p.Width, p.Brush = s.color, s.penWidth, s.brush
	presets := make([]any, len(s.presets))
	for i, pr := range s.presets {
		presets[i] = pr.JSON()
	}
	core.Prefs.SetString("study.brush-presets.v1", core.Encode(map[string]any{
		"presets": presets, "selected": s.presetIndex, "eraser_size": s.eraserSize, "eraser_mode": s.eraserMode,
		"pressure_eraser": s.pressureEraser, "prediction": s.predictionEnabled, "pen_buttons": s.penButtons,
		"pressure_threshold": s.pressureThreshold,
	}))
}

func (s *StudyCanvas) loadLayout() {
	raw, ok := core.Prefs.String("study.notein-layout.v2")
	if !ok {
		return
	}
	v, err := core.Decode([]byte(raw))
	data := plainMap(v)
	if err != nil || data == nil {
		return
	}
	s.paperStyle = "ruled"
	switch p, _ := data["paper"].(string); p {
	case "blank", "ruled", "dots", "grid":
		s.paperStyle = p
	}
	s.paperColor = 0xffffffff
	if f, ok := data["paper_color"].(float64); ok && paperColorKnown(uint32(f)) {
		s.paperColor = uint32(f)
	}
	s.dockSmall = data["pens_small"] == true
	s.penPlacement = parseDockPlacement(data["pens"], s.penPlacement)
	s.aiPlacement = parseDockPlacement(data["ai"], s.aiPlacement)
}

func (s *StudyCanvas) saveLayout() {
	core.Prefs.SetString("study.notein-layout.v2", core.Encode(map[string]any{
		"paper": s.paperStyle, "paper_color": s.paperColor, "pens": s.penPlacement.json(), "ai": s.aiPlacement.json(),
		"pens_small": s.dockSmall,
	}))
}

func (s *StudyCanvas) pickPen(index int) {
	if s.wholeCardSelected() {
		clear(s.selection)
		s.detail.Clear()
		s.selectionBounds, s.regionSelection = nil, nil
	}
	s.selectionReady = false
	s.eraserCursor = nil
	s.presetIndex = index
	p := s.presets[index]
	s.color, s.penWidth, s.brush = p.Color, p.Width, p.Brush
	s.tool = toolPen
	if p.Brush.Family == "highlighter" {
		s.tool = toolHighlighter
	}
	s.saveBrushSettings()
}

func (s *StudyCanvas) chooseTool(tool studyTool) {
	if s.tool == tool && (tool == toolPen || tool == toolHighlighter || tool == toolEraser) {
		if s.ink() && s.wholeCardSelected() {
			s.pickPen(s.presetIndex)
			return
		}
		s.penSettings(tool == toolEraser)
		return
	}
	s.eraserCursor = nil
	s.selectionReady = false
	if tool == toolPen || tool == toolHighlighter {
		for i, p := range s.presets {
			if (p.Brush.Family == "highlighter") == (tool == toolHighlighter) {
				s.pickPen(i)
				return
			}
		}
	}
	s.tool = tool
}

// quickPen applies a colour or width of the dock to the chosen pen, which
// it resumes, also after erasing.
func (s *StudyCanvas) quickPen(color *uint32, width *float64) {
	s.pickPen(s.presetIndex)
	if color != nil {
		s.color = *color
	}
	if width != nil {
		s.penWidth = *width
	}
	s.saveBrushSettings()
}

func (s *StudyCanvas) addPreset() {
	if len(s.presets) >= 20 {
		return
	}
	s.presets = append(s.presets, study.PenPreset{Name: fmt.Sprintf("%s %d", brushNames[s.brush.Family], len(s.presets)+1),
		Color: s.color, Width: s.penWidth, Brush: s.brush, ShapeAssist: s.presets[s.presetIndex].ShapeAssist})
	s.presetIndex = len(s.presets) - 1
}

// brushSlider is a labelled slider of the pen settings; it reports a change.
func brushSlider(c *ui.Context, title string, value *float64, lo, hi float64, header bool) bool {
	digits := 2
	if title == "笔刷粗细" {
		digits = 1
	}
	*value = clampF(*value, lo, hi)
	ui.Column(c).FillWidth().Children(func() {
		text := strconv.FormatFloat(*value, 'f', digits, 64)
		if header {
			m3.Text(c, title+" · "+text, design.TextStyle{Size: 12, Weight: 600, Color: design.S().OnSurface})
		} else {
			ui.Row(c).FillWidth().Children(func() {
				m3.Text(c, title, design.TextStyle{Size: 13, Color: design.S().OnSurface})
				ui.Spacer(c)
				m3.Text(c, text, design.TextStyle{Size: 12, Color: design.S().OnSurface})
			})
		}
	})
	return m3.Slider{Min: lo, Max: hi}.Show(c, value).Changed()
}

// brushPreview is _BrushPreview: a wave of the pen as it writes.
func brushPreview(c *ui.Context, height float32, color uint32, width float64, brush study.Brush) {
	ch := studyChrome()
	tool := "pen"
	if brush.Family == "highlighter" {
		tool = "highlighter"
	}
	ui.Box(c).FillWidth().Height(height).Clip().Draw(func(p *ui.Painter, r ui.Rect) {
		p.Fill(r, ch.surfaceSoft, 0)
		pts := make([]study.InkPoint, 100)
		for i := range pts {
			t := float64(i) / 99
			pts[i] = study.InkPoint{P: study.Pt{X: 24 + (float64(r.W)-48)*t, Y: float64(r.H)/2 + math.Sin(t*math.Pi*3)*22},
				Pressure: .1 + .9*math.Sin(t*math.Pi)}
		}
		b := brush
		paintInk(p, ptOf(r.X, r.Y), pts, color, width, tool, &b)
	})
}

var hexColor = regexp.MustCompile(`^[0-9a-fA-F]{6}$`)

// advancedPenSettings is the pen box: brushes, colours, widths and the
// eraser, in four tabs.
func (s *StudyCanvas) advancedPenSettings(eraser bool, tab int) {
	if eraser {
		tab = 3
	}
	hex := strings.ToUpper(fmt.Sprintf("%06x", s.color&0xffffff))
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		cs, ch := design.S(), studyChrome()
		changed := false
		change := func(v bool) {
			if v {
				changed = true
			}
		}
		m3.Dialog(c, func() {
			ui.Column(c).Width(f32(math.Min(400, float64(WindowWidth)-100))).Height(f32(math.Min(620, float64(WindowHeight)-48))).Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(16, 24, 0, 24).Children(func() {
					m3.Text(c, "笔盒", design.TextStyle{Size: 22, Weight: 700, Color: cs.OnSurface})
					ui.Spacer(c)
					if (m3.Button{Kind: m3.TextKind, Icon: icons.Add, Label: fmt.Sprintf("收藏 %d/20", len(s.presets)), Disabled: len(s.presets) >= 20}).Show(c).Clicked() {
						s.addPreset()
						changed = true
					}
					if (m3.IconButton{Icon: icons.DeleteOutline, Size: 19, Tooltip: "删除当前收藏", Disabled: len(s.presets) <= 1}).Show(c).Clicked() && len(s.presets) > 1 {
						s.presets = append(s.presets[:s.presetIndex], s.presets[s.presetIndex+1:]...)
						s.presetIndex = 0
						p := s.presets[0]
						s.color, s.penWidth, s.brush = p.Color, p.Width, p.Brush
						changed = true
					}
				})
				m3.TabBar(c, []m3.Tab{{Label: "笔刷"}, {Label: "颜色"}, {Label: "粗细"}, {Label: "橡皮"}}, &tab, m3.TabOpts{})
				ui.Box(c).FillWidth().Children(func() {
					brushPreview(c, 100, s.color, s.penWidth, s.brush)
					m3.Text(c, "实时笔迹预览", design.TextStyle{Size: 10, Color: cs.OnSurfaceVariant}).Absolute().Right(8).Bottom(8)
				})
				ui.Scroll(c).FillWidth().Grow(1).Basis(0).Children(func() {
					ui.Column(c).FillWidth().Padding(24).Children(func() {
						switch tab {
						case 0:
							ui.Row(c).Wrap().Gap(8).Children(func() {
								for _, f := range brushFamilies {
									if (m3.Chip{Kind: m3.ChoiceChip, Label: brushNames[f], Selected: s.brush.Family == f}).Show(c).Clicked() {
										s.brush.Family = f
										s.brush.Opacity = 1
										s.tool = toolPen
										if f == "highlighter" {
											s.brush.Opacity, s.tool = .35, toolHighlighter
										}
										changed = true
									}
								}
							})
							ui.Box(c).Height(20)
							change(m3.SwitchTile(c, m3.Tile{Title: "一笔成形", Subtitle: "画完停住约 0.7 秒；继续按住可调整", Pad: true}, &s.presets[s.presetIndex].ShapeAssist))
							change(brushSlider(c, "不透明度", &s.brush.Opacity, .05, 1, true))
							change(brushSlider(c, "压感响应", &s.brush.Pressure, 0, 1, true))
							change(brushSlider(c, "平滑程度", &s.brush.Smoothing, 0, 1, true))
							if s.brush.Family == "highlighter" {
								change(m3.Segmented(c, []m3.Segment[bool]{{Value: false, Label: "圆头"}, {Value: true, Label: "平头"}}, &s.brush.SquareTip, m3.SegOpts{}))
							}
							ui.Box(c).Height(12)
							m3.Text(c, "圆珠笔与荧光笔保持均匀宽度；钢笔、铅笔与软笔响应压力。鼠标使用固定压力。", design.TextStyle{Size: 12, Color: cs.OnSurfaceVariant})
						case 1:
							ui.Row(c).Wrap().Gap(12).Children(func() {
								for _, col := range penPalette {
									col := col
									sel := s.color == col
									bw, bc := float32(1), ch.line
									if sel {
										bw, bc = 3, ch.accent
									}
									e := ui.Box(c.Key(fmt.Sprintf("pen-color-%x", col))).Size(38, 38).Radius(19).Background(design.ARGB(col)).
										Border(bw, bc).Center().Cursor(ui.CursorPointer)
									if e.Clicked() {
										s.color = col
										hex = strings.ToUpper(fmt.Sprintf("%06x", col&0xffffff))
										changed = true
									}
									e.Children(func() {
										if sel {
											fg := ui.RGB(255, 255, 255)
											if design.Luminance(design.ARGB(col)) > .5 {
												fg = ui.RGB(0, 0, 0)
											}
											m3.Icon(c, icons.Check, 18, fg)
										}
									})
								}
							})
							ui.Box(c).Height(24)
							if (m3.Field{Label: "自定义 HEX 颜色", PrefixText: "#", MaxLength: 6, Counter: true}).Show(c, &hex).Changed() && hexColor.MatchString(hex) {
								v, _ := strconv.ParseUint(hex, 16, 32)
								s.color = 0xff000000 | uint32(v)
								changed = true
							}
						case 2:
							ui.Row(c).Wrap().Gap(8).Children(func() {
								for _, w := range []float64{1, 2, 3, 5, 8, 12, 20, 32} {
									if (m3.Chip{Kind: m3.ChoiceChip, Label: fmt.Sprint(int(w)), Selected: math.Abs(s.penWidth-w) < .05}).Show(c).Clicked() {
										s.penWidth = w
										changed = true
									}
								}
							})
							ui.Box(c).Height(24)
							change(brushSlider(c, "笔刷粗细", &s.penWidth, .2, 80, true))
							m3.Text(c, "宽度使用画布坐标；缩放画布时已有笔迹保持比例。", design.TextStyle{Size: 12, Color: cs.OnSurfaceVariant})
						default:
							if m3.Segmented(c, []m3.Segment[string]{{Value: "partial", Label: "局部擦除"}, {Value: "stroke", Label: "整笔擦除"}, {Value: "lasso", Label: "圈选整笔"}}, &s.eraserMode, m3.SegOpts{}) {
								s.tool = toolEraser
								changed = true
							}
							ui.Box(c).Height(16)
							change(m3.SwitchTile(c, m3.Tile{Title: "压感橡皮", Subtitle: "落笔时原地重压停留，临时擦除；画出笔迹后停留用于成形", Pad: true}, &s.pressureEraser))
							if s.pressureEraser {
								change(brushSlider(c, "触发压力（越小越灵敏）", &s.pressureThreshold, .55, .98, true))
							}
							ui.Box(c).Height(16)
							change(brushSlider(c, "橡皮半径", &s.eraserSize, 3, 100, true))
							m3.Text(c, "整笔模式会移除同一次落笔的全部分段；圈选模式在抬笔后擦除圈内笔画。锁定或隐藏图层保持不变。", design.TextStyle{Size: 12, Color: cs.OnSurfaceVariant})
						}
					})
				})
			})
		})
		if changed {
			s.saveBrushSettings()
		}
	}, nil)
}

// penSettings is _penSettings: a popover by the chosen pen (or the eraser)
// with what changes it most.
func (s *StudyCanvas) penSettings(eraser bool) {
	key := "study-tool-eraser"
	if !eraser {
		key = fmt.Sprintf("study-pen-%d", s.presetIndex)
		if s.presetIndex == 0 {
			key = "study-tool-pen"
		}
	}
	anchor, ok := s.penItems[key]
	if !ok {
		anchor = ui.Rect{X: WindowWidth/2 - 20, Y: WindowHeight/2 - 20, W: 40, H: 40}
	}
	advanced := false
	m3.ShowGeneral(true, func(c *ui.Context, r *m3.Route) {
		cs, ch := design.S(), studyChrome()
		changed := false
		change := func(v bool) {
			if v {
				changed = true
			}
		}
		size := study.Pt{X: float64(WindowWidth), Y: float64(WindowHeight)}
		width := math.Min(300, size.X-24)
		h := 490.0
		if eraser {
			h = 360
		} else if s.brush.Family == "pencil" {
			h = 530
		}
		height := math.Min(h, size.Y-48)
		rect := study.RectLTWH(float64(anchor.X), float64(anchor.Y), float64(anchor.W), float64(anchor.H))
		x := rect.Center().X - width/2
		switch s.penPlacement.edge {
		case dockLeft:
			x = rect.R + 12
		case dockRight:
			x = rect.L - width - 12
		}
		x = clampF(x, 12, math.Max(12, size.X-width-12))
		y := rect.T - height - 12
		if s.penPlacement.edge == dockTop {
			y = rect.B + 12
		}
		top, _, _, _ := c.SafeArea()
		y = clampF(y, float64(top)+12, math.Max(float64(top)+12, size.Y-height-12))
		pop := ui.Box(c.Key("study-pen-popover")).Absolute().Left(f32(x)).Top(f32(y)).Size(f32(width), f32(height)).
			Radius(20).Clip().Background(cs.SurfaceContainerLowest)
		m3.Elevation(pop, 4)
		pop.Children(func() {
			ui.Scroll(c).Fill().Children(func() {
				ui.Column(c).FillWidth().Padding(16).Children(func() {
					ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
						title := "橡皮"
						if !eraser {
							title = s.presets[s.presetIndex].Name
						}
						m3.Text(c, title, design.TextStyle{Size: 17, Weight: 600, Color: cs.OnSurface}).Grow(1)
						if !eraser && (m3.IconButton{Icon: icons.StarBorder, Size: 21, Tooltip: "收藏为新预设", Disabled: len(s.presets) >= 20}).Show(c).Clicked() {
							s.addPreset()
							changed = true
						}
						if (m3.IconButton{Icon: icons.Close, Size: 18, Tooltip: "关闭设置"}).Show(c).Clicked() {
							r.Pop(nil)
						}
					})
					m3.DividerIndent(c, 24, 0, 0)
					if eraser {
						ui.Row(c).Gap(6).Children(func() {
							for _, m := range []struct{ key, name string }{{"partial", "局部"}, {"stroke", "整笔"}, {"lasso", "圈选"}} {
								if (m3.Chip{Kind: m3.ChoiceChip, Label: m.name, Selected: s.eraserMode == m.key}).Show(c).Clicked() {
									s.eraserMode, s.tool = m.key, toolEraser
									changed = true
								}
							}
						})
						ui.Box(c).Height(18)
						change(brushSlider(c, "橡皮半径", &s.eraserSize, 3, 100, false))
						change(m3.SwitchTile(c, m3.Tile{Title: "压感橡皮", TitleStyle: &design.TextStyle{Size: 13}, Pad: true}, &s.pressureEraser))
					} else {
						m3.Text(c, "实时笔迹预览", design.TextStyle{Size: 10, Color: ch.inkFaint})
						brushPreview(c, 42, s.color, s.penWidth, s.brush)
						ui.Box(c).Height(16)
						change(m3.SwitchTile(c, m3.Tile{Title: "一笔成形", Subtitle: "画完停住约 0.7 秒；支持箭头和多边形", TitleStyle: &design.TextStyle{Size: 13}, Pad: true},
							&s.presets[s.presetIndex].ShapeAssist))
						if (m3.Button{Kind: m3.TextKind, Icon: icons.SettingsInputComponent, Label: "手写笔与预测"}).Show(c).Clicked() {
							r.Pop(nil)
							s.stylusSettings()
						}
						maxW := 20.0
						if s.brush.Family == "highlighter" {
							maxW = 80
						}
						change(brushSlider(c, "笔刷粗细", &s.penWidth, .2, maxW, false))
						if s.brush.Family != "ballpoint" && s.brush.Family != "highlighter" {
							change(brushSlider(c, "压感", &s.brush.Pressure, 0, 1, false))
						}
						if s.brush.Family == "highlighter" || s.brush.Family == "pencil" {
							change(brushSlider(c, "不透明度", &s.brush.Opacity, .05, 1, false))
						}
						change(brushSlider(c, "平滑程度", &s.brush.Smoothing, 0, 1, false))
					}
					if m3.TextButton(c, "更多画笔设置").Clicked() {
						advanced = true
						r.Pop(nil)
					}
				})
			})
		})
		if changed {
			s.saveBrushSettings()
		}
	}, func(any) {
		if advanced && !s.disposed {
			s.advancedPenSettings(eraser, 0)
		}
	})
}

// stylusSettings is _stylusSettings. Pen prediction and the pen's buttons
// come from Android's stylus plugin, which the desktop client has none of;
// the switch is kept for the shared preference.
func (s *StudyCanvas) stylusSettings() {
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		m3.AlertDialog(c, m3.Alert{Title: "手写笔", Width: 330, Content: func() {
			if m3.SwitchTile(c, m3.Tile{Title: "笔按钮切换橡皮", Subtitle: "悬空按键切换；按住侧键落笔临时擦除。需要设备上报标准笔按钮事件。", Pad: true}, &s.penButtons) {
				s.saveBrushSettings()
			}
			m3.Text(c, "M-Pencil 双击和轻捏尚未验证；本设置不代表设备支持这些手势。", design.TextStyle{Size: 12, Color: design.S().OnSurfaceVariant})
		}, Actions: func() {
			if m3.TextButton(c, "完成").Clicked() {
				r.Pop(nil)
			}
		}})
	}, nil)
}
