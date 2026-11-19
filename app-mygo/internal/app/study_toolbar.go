package app

import (
	"math"
	"runtime"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/study"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// study_toolbar_ui.dart: the bars along the top of the canvas.
// Layout and interaction reference: Notein 1.2.607.0 on the user's tablet;
// independently drawn controls, provenance in drawing-controls.md.

var paperColors = []struct {
	color uint32
	name  string
}{{0xffffffff, "白色"}, {0xfffff6dd, "米色"}, {0xffeef5e8, "浅绿"}, {0xffeef0f4, "浅灰"}}

func paperColorKnown(c uint32) bool {
	for _, p := range paperColors {
		if p.color == c {
			return true
		}
	}
	return false
}

// topBarHeight is the height of the top controls.
const topBarHeight = 36

// topBarsInset leaves room on macOS for the window buttons.
func topBarsInset() float64 {
	if runtime.GOOS == "darwin" {
		return 28
	}
	return float64(design.SpaceSM)
}

func (s *StudyCanvas) stackTopBars() bool { return s.viewport.X < 900 }

func (s *StudyCanvas) topBarsBottom() float64 {
	if s.stackTopBars() {
		return topBarsInset() + 2*topBarHeight + float64(design.SpaceSM)
	}
	return topBarsInset() + topBarHeight
}

func (s *StudyCanvas) topAction(c *ui.Context, key, label string, ic icons.Icon, disabled, selected bool) bool {
	return s.action(c, key, label, ic, disabled, selected, true)
}

// topButtons puts related buttons in one glass capsule, as iOS 26 toolbars
// group them; a lone action keeps its own circle. Fewer panes also means
// fewer backdrop blurs over a playing video.
func (s *StudyCanvas) topButtons(c *ui.Context, id string, groups ...[]func()) {
	ui.Row(c.Key("study-top-" + id)).Height(topBarHeight).AlignItems(ui.Center).Shrink(0).Children(func() {
		for g, group := range groups {
			if g > 0 {
				gap := design.SpaceSM
				if s.viewport.X < 360 {
					gap = design.SpaceXS
				}
				ui.Box(c).Width(gap)
			}
			pad := float32(0)
			if len(group) > 1 {
				pad = design.SpaceXXS
			}
			e := ui.Row(c).AlignItems(ui.Center).Padding(0, pad)
			widgets.Glass{Radius: design.CornerFull}.Apply(e)
			s.recordControl(e)
			e.Children(func() {
				for _, a := range group {
					ui.Box(c).Size(topBarHeight, topBarHeight).Center().Children(a)
				}
			})
		}
	})
}

func (s *StudyCanvas) fixedTopBars(c *ui.Context) {
	nav, act := s.navigationActions(c), s.canvasActions(c)
	navigation := func() { s.topButtons(c, "navigation", nav[:1], nav[1:]) }
	actions := func() { s.topButtons(c, "actions", act[:len(act)-1], act[len(act)-1:]) }
	tools := func() {
		e := ui.Box(c.Key("study-top-tools")).Height(topBarHeight).Padding(0, design.SpaceXS).Shrink(1)
		widgets.Glass{Radius: design.CornerFull}.Apply(e)
		s.recordControl(e)
		e.Children(func() {
			ui.ScrollHorizontal(c).Scrollbars(false).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Height(topBarHeight).Children(func() { s.toolbarActions(c) })
			})
		})
	}
	ui.Box(c.Key("study-top-bars")).Absolute().Top(f32(topBarsInset())).Left(design.SpaceSM).Right(design.SpaceSM).Children(func() {
		if s.stackTopBars() {
			ui.Column(c).FillWidth().Children(func() {
				ui.Row(c).FillWidth().Children(func() {
					navigation()
					ui.Spacer(c)
					actions()
				})
				ui.Box(c).Height(design.SpaceSM)
				ui.Row(c).FillWidth().Justify(ui.Center).Children(tools)
			})
			return
		}
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			navigation()
			ui.Box(c).Width(design.SpaceLG)
			ui.Row(c).Grow(1).Basis(0).Justify(ui.Center).Children(tools)
			ui.Box(c).Width(design.SpaceLG)
			actions()
		})
	})
}

func (s *StudyCanvas) navigationActions(c *ui.Context) []func() {
	return []func(){
		func() {
			if s.topAction(c, "", "返回", icons.ArrowBack, false, false) {
				s.onBack()
			}
		},
		func() {
			if s.topAction(c, "", "页面与材料", icons.ViewSidebarOutlined, false, false) {
				s.pageIndex()
			}
		},
		func() {
			if s.topAction(c, "study-open-shelf", l10n.StudyShelf, icons.Inventory2Outlined, false, s.shelfTab != shelfNone) {
				s.toggleShelf(shelfNone)
			}
		},
		func() {
			if s.topAction(c, "", "手写 / 浏览", icons.DrawOutlined, false, s.ink()) {
				if s.ink() {
					s.chooseTool(toolSelect)
				} else {
					s.pickPen(s.presetIndex)
				}
			}
		},
	}
}

func (s *StudyCanvas) toolbarActions(c *ui.Context) {
	d := s.doc
	if s.topAction(c, "", l10n.CommonUndo, icons.Undo, !d.CanUndo(), false) {
		d.Undo()
	}
	if s.topAction(c, "", l10n.StudyRedo, icons.Redo, !d.CanRedo(), false) {
		d.Redo()
	}
	s.divider(c, false, true)
	if s.topAction(c, "study-tool-select", "箭头：选择对象 / 拖动空白或资料页面移动画布", icons.NearMeOutlined, false, s.tool == toolSelect) {
		s.chooseTool(toolSelect)
	}
	if s.topAction(c, "study-tool-hand", l10n.StudyHandTool, icons.PanToolOutlined, false, s.tool == toolHand) {
		s.chooseTool(toolHand)
	}
	if s.topAction(c, "study-tool-region", "套索 / 框选", icons.Gesture, false, s.tool == toolRegion) {
		s.chooseTool(toolRegion)
	}
	s.divider(c, false, true)
	if s.topAction(c, "", "插入文字笔记", icons.TextFields, false, false) {
		s.editText(nil)
	}
	if s.topAction(c, "", "插入图片、PDF 或文件", icons.ImageOutlined, false, false) {
		s.importMaterial("", false, nil)
	}
	if s.topAction(c, "", "课程资料", icons.FolderOpenOutlined, false, false) {
		s.courseMaterials()
	}
	s.divider(c, false, true)
	if s.topAction(c, "", l10n.StudyLayers, icons.LayersOutlined, false, false) {
		s.layers()
	}
	if s.pluginsOffered() && s.topAction(c, "study-plugins", l10n.StudyPlugins, icons.ExtensionOutlined, false, false) {
		s.plugins()
	}
	if s.topAction(c, "", "纸张背景", icons.WallpaperOutlined, false, false) {
		s.paperSettings()
	}
	if s.remotePending {
		if m3.TextButton(c, "保留本机").Clicked() {
			s.resolveRemote(true)
		}
		if m3.TextButton(c, "采用另一端").Clicked() {
			s.resolveRemote(false)
		}
	}
}

func (s *StudyCanvas) canvasActions(c *ui.Context) []func() {
	menuTrigger := func(t ui.Element, ic icons.Icon, tip string) {
		t.Size(32, 32).Radius(design.CornerFull).Center().Cursor(ui.CursorPointer)
		m3.Tip(c, t, tip)
		m3.Ink(t, design.S().OnSurfaceVariant, design.CornerFull)
		t.Children(func() { m3.Icon(c, ic, 20, design.S().OnSurfaceVariant) })
	}
	return []func(){
		func() {
			m3.Menu(c, "study-add", m3.Over|m3.Popup, func(t ui.Element, open bool) { menuTrigger(t, icons.Add, "添加模块") }, func() []m3.MenuItem {
				return []m3.MenuItem{
					{Label: "视频", OnTap: s.videoModules},
					{Label: "文件", OnTap: func() { s.importMaterial("", false, nil) }},
					{Label: "课程资料", OnTap: s.courseMaterials},
					{Label: "笔记", OnTap: func() { s.editText(nil) }},
					{Label: "AI 对话", OnTap: func() { s.openConversation(true, false) }},
				}
			})
		},
		func() {
			if s.topAction(c, "", "显示全部卡片", icons.OpenInFull, false, false) {
				s.fit()
			}
		},
		func() {
			ic := icons.CloudDoneOutlined
			if s.saveErr != nil {
				ic = icons.ErrorOutline
			}
			if s.topAction(c, "study-save-status", s.status, ic, false, false) {
				s.flush(nil)
			}
		},
		func() {
			title := "画布菜单"
			if l := s.ctl.Lecture; l != nil {
				title = l.ShortTitle()
			}
			m3.Menu(c, "study-menu", m3.Over|m3.Popup, func(t ui.Element, open bool) { menuTrigger(t, icons.MoreHoriz, title) }, func() []m3.MenuItem {
				header := "学习画布"
				if l := s.ctl.Lecture; l != nil {
					header = l.ShortTitle()
				}
				return []m3.MenuItem{
					{Label: header, Header: true},
					{Label: "显示全部材料", OnTap: s.fit},
					{Label: "画笔与橡皮设置", OnTap: func() { s.advancedPenSettings(false, 0) }},
					{Label: "管理知识关联", OnTap: s.connectionSettings},
					{Label: "导出画布副本", OnTap: s.ExportRecovery},
					{Divider: true},
					{Label: l10n.StudyExitCanvas, OnTap: s.onClassic},
				}
			})
		},
	}
}

func (s *StudyCanvas) paperSettings() {
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		ch := studyChrome()
		m3.AlertDialog(c, m3.Alert{Title: "纸张背景", Width: 280, Content: func() {
			ui.Row(c).Wrap().Gap(12).Children(func() {
				for _, st := range []struct{ key, name string }{{"blank", "空白"}, {"ruled", "横线"}, {"dots", "点阵"}, {"grid", "方格"}} {
					st := st
					border := ch.line
					if s.paperStyle == st.key {
						border = ch.accent
					}
					e := ui.Column(c.Key("paper/"+st.key)).Width(126).Padding(8).Radius(10).Border(1.5, border).AlignItems(ui.Center).
						Cursor(ui.CursorPointer).Checked(s.paperStyle == st.key)
					m3.Ink(e, ch.ink, 10)
					if e.Clicked() {
						s.paperStyle = st.key
						s.saveLayout()
					}
					e.Children(func() {
						tone := design.ARGB(s.paperColor)
						ui.Box(c).Size(108, 48).Draw(func(p *ui.Painter, r ui.Rect) { paintPaper(p, r, study.Pt{}, 0, st.key, tone) })
						ui.Box(c).Height(6)
						m3.Text(c, st.name, design.TT().BodyMedium.C(ch.ink))
					})
				}
			})
			ui.Box(c).Height(20)
			m3.Text(c, "底色", design.TT().BodyMedium.C(ch.ink))
			ui.Box(c).Height(8)
			ui.Row(c).Wrap().Gap(8).Children(func() {
				for _, pc := range paperColors {
					pc := pc
					if (m3.Chip{Kind: m3.ChoiceChip, Label: pc.name, BG: design.ARGB(pc.color), Selected: s.paperColor == pc.color,
						FG: design.ARGB(0xff25344a)}).Show(c).Clicked() {
						s.paperColor = pc.color
						s.saveLayout()
					}
				}
			})
		}, Actions: func() {
			if m3.TextButton(c, "完成").Clicked() {
				r.Pop(nil)
			}
		}})
	}, nil)
}

func (s *StudyCanvas) pageIndex() {
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		m3.Dialog(c, func() {
			ui.Column(c).Width(360).Height(f32(math.Min(620, float64(WindowHeight)-48))).Children(func() {
				m3.Tile{Title: "页面与材料"}.Show(c)
				ui.Scroll(c).FillWidth().Grow(1).Basis(0).Children(func() {
					for _, card := range s.doc.Cards {
						ic := icons.DescriptionOutlined
						switch card.Type {
						case "pdf", "pdfdoc":
							ic = icons.PictureAsPdfOutlined
						case "lecture", "video":
							ic = icons.PlayCircleOutline
						}
						title := card.Title
						if title == "" {
							title = card.Type
						}
						id := card.ID
						ui.Box(c.Key("page/" + id)).FillWidth().Children(func() {
							if (m3.Tile{LeadingIcon: ic, Title: title, TitleLines: 2, Tap: true}).Show(c).Clicked() {
								r.Pop(id)
							}
						})
					}
				})
			})
		})
	}, func(v any) {
		id, _ := v.(string)
		card := s.doc.Card(id)
		if card == nil || s.disposed {
			return
		}
		s.doc.Camera = s.viewport.Div(2).Sub(card.Rect.Center().Mul(s.doc.Zoom))
		s.cameraChanged()
	})
}
