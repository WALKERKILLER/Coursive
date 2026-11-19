package app

import (
	"fmt"
	"math"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/video"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/study"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// The cards of the canvas (study_canvas.dart's _card, _LocalVideo;
// study_pdf_ui.dart; _KnowledgeBody of study_agent_ui.dart).

type cardAction struct {
	label    string
	icon     icons.Icon
	run      func()
	disabled bool
}

func cardIcon(typ string) icons.Icon {
	switch typ {
	case "lecture", "video":
		return icons.SmartDisplayOutlined
	case "text":
		return icons.NotesRounded
	case "conversation":
		return icons.ForumOutlined
	case "image":
		return icons.ImageOutlined
	case "note", "explanation", "knowledge":
		return icons.LightbulbOutlineRounded
	case "practice":
		return icons.QuizOutlined
	}
	return icons.DescriptionOutlined
}

func (s *StudyCanvas) cardMore(card *study.Card) []cardAction {
	d := s.doc
	pdfColumn := card.Type == "pdfdoc" && card.PDFColumn != ""
	var more []cardAction
	if !card.Generated() {
		for _, src := range card.Sources {
			src := src
			kind := src.Kind
			if src.Page > 0 {
				kind = fmt.Sprintf("%s · %d", src.Kind, src.Page)
			}
			more = append(more, cardAction{l10n.StudySelectionSource(kind), icons.Link, func() { s.focusSource(src) }, false})
		}
	}
	if !pdfColumn && card.QuarterTurns == 0 {
		label, ic := l10n.StudyCardCollapse, icons.UnfoldLessRounded
		if card.Collapsed {
			label, ic = l10n.StudyCardExpand, icons.UnfoldMoreRounded
		}
		more = append(more, cardAction{label, ic, func() {
			d.Checkpoint()
			card.Collapsed = !card.Collapsed
			if card.Collapsed {
				card.ExpandedH = card.Rect.H()
				card.Rect = study.RectLTWH(card.Rect.L, card.Rect.T, card.Rect.W(), 60)
			} else {
				minH := 240.0
				if card.Type == "conversation" {
					minH = 160
				}
				card.Rect = study.RectLTWH(card.Rect.L, card.Rect.T, card.Rect.W(), math.Max(minH, card.ExpandedH))
			}
			d.Changed()
		}, false})
	}
	if card.PDFColumn != "" {
		col := card.PDFColumn
		more = append(more, cardAction{l10n.StudyCardFoldColumn, icons.VerticalAlignCenterRounded, func() { s.collapsePDFColumn(col) }, s.importing})
	}
	if card.Generated() {
		more = append(more, cardAction{l10n.StudyCardEditKnowledge, icons.EditOutlined, func() { s.editKnowledge(card) }, false})
	}
	if card.Type == "text" {
		more = append(more, cardAction{l10n.StudyEditText, icons.EditOutlined, func() { s.editText(card) }, false})
	}
	if card.Type != "lecture" && !pdfColumn {
		id := card.ID
		more = append(more, cardAction{l10n.StudyCardDelete, icons.DeleteOutlineRounded, func() {
			d.Checkpoint()
			d.Cards = filterCards(d.Cards, func(v *study.Card) bool { return v.ID != id })
			d.Connections = filterConnections(d.Connections, func(v study.Connection) bool { return v.From != id && v.To != id })
			d.Strokes = filterStrokes(d.Strokes, func(v *study.Stroke) bool { return v.Anchor != id })
			d.Changed()
		}, false})
	}
	return more
}

func (s *StudyCanvas) card(c *ui.Context, card *study.Card) {
	d := s.doc
	selected := !s.snapshotting && s.selection[card.ID]
	ch, cs := studyChrome(), design.S()
	dark := design.T().Dark
	side, sideW := cs.OnSurface.Alpha(.08), float32(1)
	if dark {
		side = cs.OnSurface.Alpha(.12)
	}
	if selected {
		side, sideW = ch.accent, 2
	}
	action := func(key, tip string, ic icons.Icon, disabled bool) bool {
		e := m3.IconButton{Icon: ic, Tooltip: tip, Size: 16, Box: 32, Color: cs.OnSurfaceVariant, Disabled: disabled}.Show(c)
		if key != "" {
			e = e.Key(key)
		}
		return e.Clicked() && !disabled
	}
	more := s.cardMore(card)
	e := ui.Column(c).Fill().Background(ch.paper).Radius(design.CornerLG).Border(sideW, side).Clip()
	// Dark cards separate by a tonal step; a shadow would only muddy.
	if !dark {
		e = e.Shadow(0, 6, 24, 0, ch.shadow)
	}
	ignoring := s.writing() || s.tool == toolHand || !d.Editable(card.Layer)
	e.Children(func() {
		head := ui.Row(c).FillWidth().Height(36).Shrink(0).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(cs.OnSurface.Alpha(.08))
		if card.Type == "lecture" {
			head = head.Background(ch.accentSoft)
		}
		head.IgnoreInput(ignoring).Children(func() {
			ui.Box(c).Width(design.SpaceMD)
			col := cs.OnSurfaceVariant
			if card.Type == "conversation" {
				col = cs.Primary
			}
			m3.Icon(c, cardIcon(card.Type), 16, col)
			ui.Box(c).Width(design.SpaceSM)
			title := card.Title
			if card.Type == "pdf" && card.PDFColumn != "" {
				title = l10n.StudyCardPdfPage(card.Title, card.Page, card.PageCount)
			}
			t := m3.Text(c, title, design.TT().LabelLarge.C(ch.ink)).MaxLines(1).Grow(1).Basis(0)
			m3.Tip(c, t, card.Title)
			acts := ui.Row(c.Key("study-card-actions/" + card.ID)).AlignItems(ui.Center)
			if b := acts.Bounds(); b.W > 0 {
				s.headerActions[card.ID] = b
			}
			acts.Children(func() {
				if card.Type == "pdfdoc" && (card.PDFColumn == "" || len(d.PDFPages(card.ID)) < card.PageCount) {
					tip := "继续展开整列"
					if card.PDFColumn == "" {
						tip = "在画布展开为一列"
					}
					if action("", tip, icons.ViewAgendaOutlined, s.importing) {
						s.expandPDFColumn(card)
					}
				}
				if card.Type != "conversation" {
					btn := m3.IconButton{Icon: icons.AddCommentOutlined, Tooltip: l10n.StudyCardAsk, Size: 16, Box: 32, Color: cs.OnSurfaceVariant}.Show(c)
					s.cardDragSource(c, btn, card)
					if btn.Clicked() {
						s.chooseConversation(card)
					}
				}
				if len(more) > 0 {
					m3.Menu(c, "card-more/"+card.ID, m3.BelowEnd, func(t ui.Element, open bool) {
						t.Size(32, 32).Radius(design.CornerFull).Center().Cursor(ui.CursorPointer)
						m3.Tip(c, t, l10n.StudyCardMore)
						m3.Ink(t, cs.OnSurfaceVariant, design.CornerFull)
						t.Children(func() { m3.Icon(c, icons.MoreHorizRounded, 16, cs.OnSurfaceVariant) })
					}, func() []m3.MenuItem {
						items := make([]m3.MenuItem, len(more))
						for i, a := range more {
							items[i] = m3.MenuItem{Icon: a.icon, IconSize: 18, Label: a.label, Disabled: a.disabled, OnTap: a.run}
						}
						return items
					})
				}
			})
			m3.Icon(c, icons.DragIndicatorRounded, 16, ch.inkFaint)
			ui.Box(c).Width(design.SpaceSM)
		})
		body := ui.Box(c).FillWidth().Grow(1).Basis(0).Clip().IgnoreInput(ignoring)
		body.Hidden(card.Collapsed).Children(func() {
			if card.Collapsed {
				return
			}
			s.cardBody(c, card)
		})
		ui.Row(c).FillWidth().Height(16).Shrink(0).Justify(ui.End).AlignItems(ui.End).IgnoreInput(true).Children(func() {
			if card.QuarterTurns == 0 {
				col := ch.inkFaint
				if selected {
					col = ch.accent
				}
				m3.Icon(c, icons.SouthEast, 12, col)
			}
		})
	})
}

func (s *StudyCanvas) cardBody(c *ui.Context, card *study.Card) {
	d := s.doc
	ch := studyChrome()
	api := State.Api()
	switch card.Type {
	case "pdfdoc":
		s.pdfBody(c, card)
	case "lecture":
		ui.Column(c).Fill().Children(func() {
			ui.Box(c).FillWidth().Grow(1).Basis(0).Children(func() { s.snapshotVideo(c, card, func() { s.video(c) }) })
			if (m3.Button{Kind: m3.TextKind, Icon: icons.SubtitlesOutlined, Label: l10n.StudyShelfFromVideo, Height: 48}).Show(c).
				Key("study-video-shelf").Clicked() {
				s.toggleShelf(shelfTranscript)
			}
		})
	case "image", "pdf":
		ui.Box(c).Fill().Children(func() {
			bmp, failed := widgets.NetBitmap(api.FileURL(card.Path))
			switch {
			case failed:
				ui.Box(c).Fill().Center().Children(func() { m3.Text(c, l10n.StudyImageFailed, design.TT().BodyMedium.C(ch.inkSoft)) })
			case bmp != nil:
				ui.Image(c, bmp).Fit(ui.Contain).Absolute().Top(0).Left(0).Right(0).Bottom(0)
			}
		})
	case "video":
		url := api.FileURL(card.Path)
		if card.Path == "" {
			url = api.StudyMediaURL(s.lectureID(), card.ID)
		}
		s.snapshotVideo(c, card, func() { s.localVideo(c, card.ID, url) })
	case "note", "explanation", "knowledge", "practice":
		ui.Box(c.Key(fmt.Sprintf("%s:%d", card.ID, s.cardTextEpoch))).Fill().Children(func() {
			s.knowledgeBody(c, card, d.Editable(card.Layer))
		})
	default:
		box := ui.Scroll(c.Key(fmt.Sprintf("%s:%d", card.ID, s.cardTextEpoch))).Fill().Selectable()
		box.Children(func() {
			ui.Text(c, card.Text).FontSize(16).LineHeight(1.65).TextColor(ch.paperInk).Margin(20).FillWidth()
		})
		lo, hi := 0, 0
		for _, t := range box.SelectedTexts() {
			lo, hi = min(t.Start, t.End), max(t.Start, t.End)
		}
		if old := s.textSelections[card.ID]; old != [2]int{lo, hi} {
			s.textSelections[card.ID] = [2]int{lo, hi}
			if lo != hi {
				clear(s.selection)
				s.selection[card.ID] = true
				s.detail.Clear()
				s.detail.ObjectID, s.detail.Start, s.detail.End = card.ID, runeToUTF16(card.Text, lo), runeToUTF16(card.Text, hi)
			}
		}
	}
}

// runeToUTF16 is the UTF-16 offset of rune i of text, the offsets the
// daemon and the Flutter client count in.
func runeToUTF16(text string, i int) int {
	n := 0
	for k, r := range []rune(text) {
		if k >= i {
			break
		}
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// pdfBody is _PDFBody: one page of a document, turned with the buttons
// below it.
func (s *StudyCanvas) pdfBody(c *ui.Context, card *study.Card) {
	st := s.pdfStates[card.ID]
	if st == nil {
		st = &pdfState{}
		s.pdfStates[card.ID] = st
	}
	page := func(number int) {
		if st.busy || number < 1 || number > card.PageCount {
			return
		}
		st.busy, st.err = true, ""
		id, source := s.lectureID(), card.Source
		core.Async(func() (any, error) { return State.Api().StudyPDFPage(id, source, number) }, func(out any, err error) {
			st.busy = false
			if err != nil {
				st.err = err.Error()
				return
			}
			m := plainMap(out)
			d := s.doc
			d.Checkpoint()
			w, _ := m["width"].(float64)
			h, _ := m["height"].(float64)
			if h > 0 {
				card.PageAspect = w / h
			}
			card.Page = number
			card.Path = fmt.Sprint(m["file"])
			card.Text = fmt.Sprint(m["text"])
			card.TextBoxes = listOf(m["boxes"])
			d.Changed()
		})
	}
	cs := design.S()
	ui.Column(c).Fill().Children(func() {
		ui.Box(c).FillWidth().Grow(1).Basis(0).Children(func() {
			bmp, failed := widgets.NetBitmap(State.Api().FileURL(card.Path))
			if failed {
				ui.Box(c).Fill().Center().Children(func() {
					if m3.TextButton(c, "重载此页").Clicked() {
						widgets.ForgetPicture(State.Api().FileURL(card.Path))
						page(card.Page)
					}
				})
			} else if bmp != nil {
				ui.Image(c, bmp).Fit(ui.Contain).Absolute().Top(0).Left(0).Right(0).Bottom(0)
			}
			if st.busy {
				ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Center().Children(func() { m3.Circular(c, m3.Progress{}) })
			}
		})
		if st.err != "" {
			m3.Text(c, st.err, design.TextStyle{Size: 11, Color: cs.Error}).MaxLines(2)
		}
		ui.Row(c).FillWidth().Height(36).Shrink(0).AlignItems(ui.Center).Children(func() {
			if (m3.IconButton{Icon: icons.ChevronLeft, Size: 18, Box: 36, Tooltip: "上一页", Disabled: st.busy || card.Page <= 1}).Show(c).Clicked() {
				page(card.Page - 1)
			}
			m3.Text(c, fmt.Sprintf("%d / %d", card.Page, card.PageCount), design.TextStyle{Size: 12, Color: cs.OnSurface})
			if (m3.IconButton{Icon: icons.ChevronRight, Size: 18, Box: 36, Tooltip: "下一页", Disabled: st.busy || card.Page >= card.PageCount}).Show(c).Clicked() {
				page(card.Page + 1)
			}
		})
	})
}

type pdfState struct {
	busy bool
	err  string
}

// localVideo is _LocalVideo: a video card's own player, opened once the
// canvas that names its media is saved.
type localVideo struct {
	player  *ui.VideoPlayer
	failed  bool
	opening bool
	seek    float64
}

func (s *StudyCanvas) localVideo(c *ui.Context, id, url string) {
	v := s.videos[id]
	if v == nil {
		v = &localVideo{opening: true}
		s.videos[id] = v
		// The media route resolves saved canvas objects. Publishing the
		// reference must finish before the player asks for its URL.
		s.flush(func(ok bool) {
			v.opening = false
			if !ok || s.disposed {
				v.failed = true
				return
			}
			p, err := video.OpenURL(url)
			if err != nil {
				v.failed = true
				return
			}
			p.Pause()
			if ms := s.videoTimes[id]; ms > 0 {
				p.Seek(time.Duration(ms) * time.Millisecond)
			}
			v.player = p
		})
	}
	if v.failed || (v.player != nil && v.player.Err() != nil) {
		ui.Box(c).Fill().Center().Children(func() { m3.Text(c, l10n.StudyLocalVideoError, design.TT().BodyMedium.C(design.S().OnSurface)) })
		return
	}
	ui.Column(c).Fill().Background(ui.RGB(0, 0, 0)).Children(func() {
		ui.Box(c).FillWidth().Grow(1).Basis(0).Children(func() {
			if v.player != nil {
				ui.Video(c, v.player).Fit(ui.Contain).Absolute().Top(0).Left(0).Right(0).Bottom(0)
			}
		})
		p := v.player
		if p == nil {
			return
		}
		s.videoTimes[id] = int(p.Position().Milliseconds())
		white := ui.RGB(255, 255, 255)
		dur := p.Duration().Seconds()
		v.seek = p.Position().Seconds()
		if (m3.Slider{Min: 0, Max: math.Max(1, dur), TrackHeight: 4}).Show(c, &v.seek).Changed() {
			p.Seek(time.Duration(v.seek * float64(time.Second)))
		}
		ui.Row(c).FillWidth().Height(48).AlignItems(ui.Center).Padding(0, design.SpaceSM).Children(func() {
			ic := icons.PlayArrowRounded
			if p.Playing() {
				ic = icons.PauseRounded
			}
			if (m3.IconButton{Icon: ic, Color: white}).Show(c).Clicked() {
				if p.Playing() {
					p.Pause()
				} else {
					p.Play()
				}
			}
			m3.Text(c, core.FmtClock(int(p.Position().Milliseconds()))+" / "+core.FmtClock(int(p.Duration().Milliseconds())),
				design.TextStyle{Size: 12, Color: white})
		})
		if p.Playing() {
			c.After(250 * time.Millisecond)
		}
	})
}

// seekVideoPaused shows the frame at ms of a video card's player, paused.
func (s *StudyCanvas) seekVideoPaused(id string, ms int) error {
	v := s.videos[id]
	if v == nil || v.player == nil || v.failed {
		return fmt.Errorf("Video unavailable")
	}
	v.player.Seek(time.Duration(ms) * time.Millisecond)
	v.player.Pause()
	return nil
}

func (s *StudyCanvas) disposeVideos() {
	for _, v := range s.videos {
		if v.player != nil {
			v.player.Close()
		}
	}
}

// knowledgeBody is _KnowledgeBody: a generated card's blocks, its
// exercises with the learner's answers, and its sources.
func (s *StudyCanvas) knowledgeBody(c *ui.Context, card *study.Card, editable bool) {
	ch, cs, tt := studyChrome(), design.S(), design.TT()
	d := s.doc
	sel := func(text string, style design.TextStyle) ui.Element {
		e := ui.Text(c, text).Selectable().FillWidth()
		return style.Apply(e)
	}
	body := tt.BodyMedium.C(ch.paperInk)
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).FillWidth().Padding(18).Children(func() {
			for i, b := range card.Blocks {
				i, b := i, b
				ui.Column(c).FillWidth().Margin(0, 0, 12, 0).Children(func() {
					switch b.Type {
					case "heading":
						sel(b.Text, design.TextStyle{Size: 19, Weight: 700, Color: ch.ink})
					case "list", "steps":
						for k, item := range b.Items {
							mark := "•"
							if b.Type == "steps" {
								mark = fmt.Sprintf("%d.", k+1)
							}
							sel(mark+" "+item, body).Margin(0, 0, 6, 0)
						}
					case "table":
						ui.ScrollHorizontal(c).FillWidth().Children(func() {
							tracks := make([]ui.Track, len(b.Columns))
							for k := range tracks {
								tracks[k] = ui.FitContent()
							}
							ui.Grid(c).ColumnTracks(tracks...).Children(func() {
								for _, col := range b.Columns {
									m3.Text(c, col, tt.TitleSmall.C(ch.ink)).Padding(12, 16)
								}
								for _, row := range b.Rows {
									for k := range b.Columns {
										v := ""
										if k < len(row) {
											v = row[k]
										}
										sel(v, body).Padding(12, 16).BorderWidth(1, 0, 0, 0).BorderColor(cs.OutlineVariant)
									}
								}
							})
						})
					case "code":
						ui.Box(c).FillWidth().Padding(12).Background(ch.surfaceSoft).Children(func() {
							sel(b.Language+"\n"+b.Text, design.TextStyle{Size: 13, Family: "monospace", Color: ch.ink})
						})
					case "math":
						MathBlock(c, b.Text, 16, ch.ink)
					case "exercise":
						sel(b.Text, body.W(600))
						key := fmt.Sprint(i)
						value := card.Responses[key]
						in := m3.Field{Label: "我的作答", Helper: "随工作区保存", MinLines: 2, MaxLines: 6, MaxLength: 4000, Disabled: !editable}.
							Show(c, &value).Key(card.ID + "/answer/" + key)
						if in.Focused() && !s.answering[card.ID+key] {
							s.answering[card.ID+key] = true
							d.Checkpoint()
						} else if !in.Focused() {
							delete(s.answering, card.ID+key)
						}
						if in.Changed() && editable {
							if card.Responses == nil {
								card.Responses = map[string]string{}
							}
							card.Responses[key] = value
							d.Changed()
						}
						if b.Hint != "" {
							m3.Expansion{Tile: m3.Tile{Title: "提示"}, Children: func() { sel(b.Hint, body).Padding(8) }}.Show(c, card.ID+"/hint/"+key)
						}
						if b.Answer != "" {
							m3.Expansion{Tile: m3.Tile{Title: "答案与解析"}, Children: func() { sel(b.Answer, body).Padding(8) }}.Show(c, card.ID+"/answer-x/"+key)
						}
					default:
						sel(b.Text, body)
					}
				})
			}
			if len(card.Sources) > 0 {
				m3.Divider(c)
				m3.Text(c, "引用来源", tt.LabelMedium.C(cs.OnSurface))
			}
			for k, a := range card.Sources {
				a := a
				var label string
				switch a.Kind {
				case "video":
					label = "视频 · " + core.FmtClock(a.TimeStartMs) + "–" + core.FmtClock(a.TimeEndMs)
				case "ink":
					label = fmt.Sprintf("手写选区 · %d 段", len(a.StrokeIDs))
				default:
					label = upper(a.Kind)
					if a.Page > 0 {
						label += fmt.Sprintf(" · 第 %d 页", a.Page)
					}
					label += " · " + truncRunes(a.Quote, 50)
				}
				if (m3.Button{Kind: m3.TextKind, Icon: icons.Link, Label: label}).Show(c).Key(fmt.Sprintf("source/%d", k)).Clicked() {
					s.focusSource(a)
				}
			}
		})
	})
}

func upper(s string) string {
	b := []byte(s)
	for i, ch := range b {
		if ch >= 'a' && ch <= 'z' {
			b[i] = ch - 32
		}
	}
	return string(b)
}

func truncRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
