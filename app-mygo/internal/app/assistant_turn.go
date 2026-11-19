package app

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// The messages of assistant_panel.dart: the user's bubbles and the
// assistant's turns with their thinking, tool calls and cards.

type toolLabel struct {
	label string
	icon  icons.Icon
}

var toolLabels = map[string]toolLabel{
	"app_guide":         {l10n.ToolHelp, icons.MenuBookOutlined},
	"current_context":   {l10n.ToolCurrentContext, icons.VisibilityOutlined},
	"list_courses":      {l10n.ToolListCourses, icons.SchoolOutlined},
	"list_lectures":     {l10n.ToolListLectures, icons.EventNoteOutlined},
	"read_transcript":   {l10n.ToolReadTranscript, icons.SubjectRounded},
	"lecture_info":      {l10n.ToolLectureDetail, icons.InfoOutlineRounded},
	"search_library":    {l10n.ToolSearchLibrary, icons.ManageSearchRounded},
	"list_notes":        {l10n.ToolListNotes, icons.NotesRounded},
	"read_note":         {l10n.ToolReadNote, icons.DescriptionOutlined},
	"save_note":         {l10n.ToolSaveNote, icons.SaveOutlined},
	"suggest_actions":   {l10n.ToolPrepare, icons.TouchAppOutlined},
	"find_tools":        {l10n.ToolFindTools, icons.ExtensionOutlined},
	"mark_schedule":     {l10n.ToolMarkSchedule, icons.EventAvailableOutlined},
	"list_schedule":     {l10n.ToolListSchedule, icons.EventNoteOutlined},
	"update_schedule":   {l10n.ToolEditSchedule, icons.EditCalendarOutlined},
	"read_timeline":     {l10n.ToolReadOutline, icons.ViewTimelineOutlined},
	"generate_timeline": {l10n.OutlineGenerate, icons.ViewTimelineOutlined},
	"save_timeline":     {l10n.ToolSaveOutline, icons.ViewTimelineOutlined},
	"list_courseware":   {l10n.ToolListCourseware, icons.FolderOpenOutlined},
	"read_courseware":   {l10n.ToolReadCourseware, icons.ArticleOutlined},
	"search_courseware": {l10n.ToolSearchCourseware, icons.PlagiarismOutlined},
	"view_slides":       {l10n.ToolListSlides, icons.SlideshowOutlined},
	"view_images":       {l10n.ToolViewImage, icons.ImageOutlined},
	"create_quiz":       {l10n.ToolCreateQuiz, icons.QuizOutlined},
	"list_quizzes":      {l10n.ToolListQuizzes, icons.QuizOutlined},
	"read_quiz_results": {l10n.ToolReadAttempts, icons.FactCheckOutlined},
	"open_page":         {l10n.ToolOpenPage, icons.OpenInNewRounded},
	"player":            {l10n.ToolControlPlayer, icons.SmartDisplayOutlined},
	"read_settings":     {l10n.ToolReadSettings, icons.TuneRounded},
	"change_settings":   {l10n.ToolChangeSettings, icons.TuneRounded},
	"list_downloads":    {l10n.ToolListDownloads, icons.DownloadingRounded},
	"manage_downloads":  {l10n.ToolManageDownloads, icons.DownloadingRounded},
	"download":          {l10n.ToolDownload, icons.DownloadOutlined},
	"delete_files":      {l10n.ToolPrepareDelete, icons.DeleteSweepOutlined},
	"manage_course":     {l10n.ToolOrganizeCourses, icons.SchoolOutlined},
	"sync_courses":      {l10n.ToolSyncCourses, icons.SyncRounded},
	"create_task":       {l10n.ToolCreateTask, icons.ScheduleRounded},
	"propose_task":      {l10n.ToolCreateTask, icons.ScheduleRounded}, // before spec 0016
	"toggle_plugin":     {l10n.ToolTogglePlugin, icons.ExtensionOutlined},
	"memory_list":       {l10n.ToolListMemory, icons.PsychologyAltOutlined},
	"memory_read":       {l10n.ToolRecall, icons.PsychologyAltOutlined},
	"memory_write":      {l10n.ToolRemember, icons.EditNoteRounded},
	"memory_edit":       {l10n.ToolEditMemory, icons.EditNoteRounded},
	"memory_append":     {l10n.ToolRemember, icons.EditNoteRounded},
	"memory_delete":     {l10n.ToolForget, icons.AutoDeleteOutlined},
}

// chatStyle is a text style at the chat's text scale.
func chatStyle(s design.TextStyle) design.TextStyle {
	if k := float32(core.UI.ChatTextScale); k > 0 {
		s.Size *= k
	}
	return s
}

func userBubble(c *ui.Context, text string) {
	cs := design.S()
	ui.Row(c).FillWidth().Justify(ui.End).AlignItems(ui.End).Children(func() {
		ui.Box(c).Shrink(1).Margin(design.SpaceLG, 0, design.SpaceSM, 48).Padding(design.SpaceMD, design.SpaceLG).
			Background(cs.PrimaryContainer).Radius(design.CornerXL, design.CornerXL, design.CornerSM, design.CornerXL).Children(func() {
			m3.Text(c, text, chatStyle(design.TT().BodyMedium).C(cs.OnPrimaryContainer).H(1.5)).Selectable()
		})
		ui.Box(c).Margin(0, 0, design.SpaceSM, design.SpaceSM).Children(func() { UserAvatar(c, 28) })
	})
}

func blockArgs(b *ChatBlock) map[string]any {
	switch a := b.Args.(type) {
	case map[string]any:
		return a
	case core.J:
		return a
	case string:
		if strings.HasPrefix(a, "{") {
			var m map[string]any
			if json.Unmarshal([]byte(a), &m) == nil {
				return m
			}
		}
	}
	return nil
}

func (p *AssistantPanel) assistantTurn(c *ui.Context, msg *ChatMessage, last bool) {
	cs := design.S()
	tt := design.TT()
	a := Assistant
	// Bare timestamps refer to the lecture the turn's tools worked on,
	// else the open player.
	ctxLecture := ""
	for _, b := range msg.Blocks {
		if b.Kind == "tool" {
			if args := blockArgs(b); args != nil && args["lecture_id"] != nil {
				ctxLecture = core.Str(args["lecture_id"])
			}
		}
	}
	content := func() {
		item := func(f func()) {
			ui.Column(c).FillWidth().Margin(0, 0, design.SpaceXS, 0).Children(f)
		}
		for i, b := range msg.Blocks {
			b := b
			live := msg.Streaming && i == len(msg.Blocks)-1
			switch b.Kind {
			case "thinking":
				if core.UI.ShowThinking && strings.TrimSpace(b.Text) != "" {
					item(func() { p.thinkingBlock(c, b, live) })
				}
			case "tool":
				item(func() { p.toolCard(c, b) })
				if acts := b.Actions(); len(acts) > 0 {
					item(func() { p.actions(c, acts) })
				}
				d := b.Details
				if op := b.AppOp(); op != nil && !b.Error {
					item(func() { appChip(c, b, op) })
				}
				if pr := b.Proposal(); pr != nil {
					item(func() {
						ProposalCard(c, pr, func(done func(error)) { a.ApplyProposal(b, done) },
							func(done func(error)) { a.DismissProposal(b, done) }, func(done func(error)) { a.UndoProposal(b, done) })
					})
				}
				if s := d.O("schedule"); s != nil && b.Name == "mark_schedule" {
					item(func() { ScheduleChatCard(c, s, d.IsTrue("duplicate")) })
				}
				if b.Name == "view_slides" && d["slides"] != nil {
					var slides []int
					for _, x := range d.L("slides") {
						slides = append(slides, core.Int(x))
					}
					item(func() { p.slideStrip(c, b, d.S("lecture_id"), slides, d.S("mode")) })
				}
				if q := d.O("quiz"); b.Name == "create_quiz" && q != nil {
					quiz := ParseQuiz(q)
					card := p.quizzes[quiz.ID]
					if card == nil {
						card = NewQuizCard(quiz, true)
						p.quizzes[quiz.ID] = card
					}
					item(func() { card.Show(c.Key("quiz-"+quiz.ID), false) })
				}
				if n := d.O("note"); b.Name == "save_note" && n != nil {
					item(func() { p.noteCard(c, n) })
				}
				if v := d.O("view"); v != nil {
					item(func() { PluginView(c, v) })
				}
				if tl := d.Os("timeline"); len(tl) > 0 {
					item(func() { p.outlineCard(c, d.S("lecture_id"), tl) })
				}
			default:
				if b.Text != "" {
					st := chatStyle(tt.BodyMedium).H(1.65)
					item(func() {
						CoursiveMarkdown{Data: b.Text, Style: &st, LectureID: ctxLecture, Streaming: live, OnRef: func(r CoursiveRef) {
							if r.Kind == RefLecture {
								p.runAction(core.J{"kind": "open_lecture", "lecture_id": r.LectureID})
								return
							}
							OpenRef(r, ctxLecture, func(t CoursiveRef) {
								id := t.LectureID
								if id == "" {
									id = ctxLecture
								}
								p.runAction(core.J{"kind": "seek", "lecture_id": id, "seconds": float64(t.Seconds)})
							})
						}}.Show(c)
					})
				}
			}
		}
		if msg.Streaming && (len(msg.Blocks) == 0 || msg.Status != "") {
			status := msg.Status
			if status == "" {
				status = l10n.ChatThinking
			}
			ui.Row(c).FillWidth().AlignItems(ui.Center).PaddingY(design.SpaceSM).Children(func() {
				widgets.Loader{Size: 22}.Show(c)
				m3.HGap(c, design.SpaceSM)
				m3.Clamp(m3.Text(c, status, tt.BodySmall.C(cs.OnSurfaceVariant)), 1).Shrink(1)
			})
		}
		if msg.Error != "" {
			ui.Row(c).FillWidth().Margin(design.SpaceSM, 0, 0, 0).Padding(design.SpaceMD).Background(cs.ErrorContainer).
				Radius(design.CornerMD).AlignItems(ui.Start).Children(func() {
				m3.Icon(c, icons.ErrorOutlineRounded, 18, cs.OnErrorContainer)
				m3.HGap(c, design.SpaceSM)
				m3.Text(c, msg.Error, tt.BodySmall.C(cs.OnErrorContainer)).Grow(1).Basis(0)
			})
		}
		if !msg.Streaming {
			var retry func()
			if last {
				retry = a.RetryLast
			}
			turnFooter(c, msg, retry)
		}
	}
	ui.Column(c).FillWidth().PaddingY(design.SpaceSM).Children(func() {
		if core.UI.ChatBubbles {
			ui.Column(c).FillWidth().Padding(design.SpaceMD, design.SpaceLG, design.SpaceXS, design.SpaceLG).Background(cs.SurfaceContainer).
				Radius(design.CornerSM, design.CornerXL, design.CornerXL, design.CornerXL).Children(content)
		} else {
			content()
		}
	})
}

func appChip(c *ui.Context, b *ChatBlock, op core.J) {
	a := Assistant
	undoable := op.S("proposal_id") != ""
	ic := icons.CheckCircleOutlineRounded
	switch op.S("op") {
	case "open":
		ic = icons.OpenInNewRounded
	case "player":
		ic = icons.SmartDisplayOutlined
	}
	chip := AppOpChip{Label: AppOpLabel(op), Icon: ic, Done: op.IsTrue("undone"), ActionLabel: l10n.CommonUndo, DoneLabel: l10n.CommonUndone}
	if op.S("op") == "open" {
		chip.ActionLabel, chip.DoneLabel = l10n.CommonBack, l10n.ToolReturned
		if CanGoBackFrom(op) {
			chip.OnAction = func(done func(error)) {
				BotBack()
				done(nil)
			}
		}
	} else if undoable {
		chip.OnAction = func(done func(error)) { a.UndoApplied(b, done) }
	}
	ui.Row(c.Key(b)).Children(func() { chip.Show(c) })
}

func turnFooter(c *ui.Context, msg *ChatMessage, onRetry func()) {
	cs := design.S()
	var parts []string
	for _, b := range msg.Blocks {
		if b.Kind == "text" {
			parts = append(parts, b.Text)
		}
	}
	text := strings.Join(parts, "\n\n")
	var meta []string
	if msg.Model != "" {
		m := msg.Model
		if i := strings.LastIndex(m, "/"); i >= 0 {
			m = m[i+1:]
		}
		meta = append(meta, m)
	}
	if u := msg.Usage; u != nil {
		meta = append(meta, strconv.Itoa(u.I("input"))+"→"+strconv.Itoa(u.I("output"))+" tokens")
	}
	ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
		if text != "" && (m3.IconButton{Icon: icons.CopyRounded, Size: 16, Box: 32, Tooltip: l10n.CommonCopy}).Show(c).Clicked() {
			c.WriteClipboard(text)
			widgets.ShowSnack(l10n.CommonCopied, "", nil)
		}
		if onRetry != nil && (m3.IconButton{Icon: icons.RefreshRounded, Size: 16, Box: 32, Tooltip: l10n.CommonRegenerate}).Show(c).Clicked() {
			onRetry()
		}
		m3.HGap(c, design.SpaceXS)
		m3.Clamp(m3.Text(c, strings.Join(meta, " · "), design.TT().LabelSmall.C(cs.Outline)), 1).Shrink(1)
	})
}

func (p *AssistantPanel) thinkingBlock(c *ui.Context, b *ChatBlock, live bool) {
	cs := design.S()
	tt := design.TT()
	open, set := p.open[b]
	if !set {
		open = live
	}
	bg := cs.SurfaceContainerHigh
	if !open {
		bg = design.WithOpacity(bg, .6)
	}
	ui.Column(c.Key(b)).FillWidth().Radius(design.CornerLG).Background(bg).
		Transition(ui.ElementTransition{Duration: design.Medium2, Ease: design.Emphasized, Colors: true}).Children(func() {
		h := ui.ButtonBase(c).FocusRing(false).Radius(design.CornerLG).Cursor(ui.CursorPointer).Row().AlignItems(ui.Center).
			Padding(design.SpaceSM, design.SpaceMD)
		m3.InkWell(h, design.CornerLG)
		h.Children(func() {
			label := l10n.ChatThinkingProcess
			if live {
				widgets.Loader{Size: 16}.Show(c)
				label = l10n.ChatThinkingNow
			} else {
				m3.Icon(c, icons.PsychologyOutlined, 16, cs.OnSurfaceVariant)
			}
			m3.HGap(c, design.SpaceSM)
			m3.Text(c, label, tt.LabelMedium.C(cs.OnSurfaceVariant))
			m3.HGap(c, 4)
			turn := h.AnimateTo("turn", map[bool]float32{false: 0, true: 180}[open], design.T().Motion(design.Medium1, ui.Linear))
			m3.Icon(c, icons.ExpandMoreRounded, 18, cs.OnSurfaceVariant).Rotate(turn)
		})
		if h.Clicked() {
			p.open[b] = !open
		}
		body := ui.Box(c).FillWidth().Clip().Transition(ui.ElementTransition{Duration: design.Medium2, Ease: design.Emphasized, Height: true})
		body.Children(func() {
			if !open {
				return
			}
			sc := ui.Scroll(c).FillWidth().MaxHeight(240).Margin(0, design.SpaceMD, design.SpaceMD, design.SpaceMD)
			if live {
				sc = sc.TrackScroll(ui.Local(sc, "think", func() ui.ScrollState { return ui.ScrollState{Y: math.MaxFloat32} }))
			}
			sc.Children(func() {
				m3.Text(c, strings.TrimSpace(b.Text), tt.BodySmall.C(cs.OnSurfaceVariant).H(1.6)).Selectable()
			})
		})
	})
}

func (p *AssistantPanel) toolCard(c *ui.Context, b *ChatBlock) {
	cs := design.S()
	tt := design.TT()
	tl, ok := toolLabels[b.Name]
	if !ok {
		if pl := Plugins.ToolPlugin(b.Name); pl != nil {
			name := Plugins.ToolLabel(b.Name)
			if name == "" {
				name = b.Name
			}
			tl = toolLabel{pl.S("title") + " · " + name, widgets.PluginIcon(pl.S("icon"))}
		} else {
			tl = toolLabel{b.Name, icons.BuildOutlined}
		}
	}
	args := ""
	if b.Args != nil {
		if raw, err := json.MarshalIndent(b.Args, "", "  "); err == nil {
			args = string(raw)
		}
	}
	open := p.open[b]
	ui.Column(c.Key(b)).FillWidth().AlignItems(ui.Start).Children(func() {
		bg, fg := design.WithOpacity(cs.SecondaryContainer, .55), cs.OnSecondaryContainer
		if b.Running {
			bg = cs.SecondaryContainer
		}
		if b.Error {
			bg, fg = cs.ErrorContainer, cs.OnErrorContainer
		}
		pill := ui.ButtonBase(c).FocusRing(false).Radius(design.CornerFull).Cursor(ui.CursorPointer).Row().AlignItems(ui.Center).
			Padding(6, design.SpaceMD).Background(bg).Transition(ui.ElementTransition{Duration: design.Medium1, Ease: ui.Linear, Colors: true})
		m3.InkWell(pill, design.CornerFull)
		pill.Children(func() {
			switch {
			case b.Running:
				m3.Spinner(c, 14, 2, cs.Primary)
			case b.Error:
				m3.Icon(c, icons.ErrorOutlineRounded, 15, fg)
			default:
				m3.Icon(c, tl.icon, 15, fg)
			}
			m3.HGap(c, 6)
			m3.Text(c, tl.label, tt.LabelMedium.C(fg))
			if b.Ms > 0 {
				d := strconv.Itoa(b.Ms) + "ms"
				if b.Ms >= 1000 {
					d = strconv.FormatFloat(float64(b.Ms)/1000, 'f', 1, 64) + "s"
				}
				m3.Text(c, " · "+d, tt.LabelSmall.C(design.WithOpacity(cs.OnSecondaryContainer, .7)))
			}
		})
		if pill.Clicked() {
			p.open[b] = !open
		}
		box := ui.Box(c).FillWidth().Clip().Transition(ui.ElementTransition{Duration: design.Medium2, Ease: design.Emphasized, Height: true})
		box.Children(func() {
			if !open {
				return
			}
			ui.Column(c).FillWidth().Margin(design.SpaceXS, 0, 0, 0).Padding(design.SpaceMD).Background(cs.SurfaceContainerHigh).
				Radius(design.CornerMD).Children(func() {
				mono := tt.BodySmall.F("monospace")
				if args != "" && args != "{}" && args != "null" {
					m3.Text(c, l10n.ToolArgs, tt.LabelSmall.C(cs.Primary))
					m3.Text(c, args, mono).Selectable()
					m3.VGap(c, design.SpaceSM)
				}
				if b.Result != "" {
					m3.Text(c, l10n.ToolResult, tt.LabelSmall.C(cs.Primary))
					m3.Text(c, b.Result, mono).MaxLines(30).Selectable()
				}
			})
		})
	})
}

// outlineCard is chapters the assistant generated, each one a jump into
// the recording.
func (p *AssistantPanel) outlineCard(c *ui.Context, lectureID string, chapters []core.J) {
	cs := design.S()
	tt := design.TT()
	ui.Column(c).FillWidth().Margin(design.SpaceXS, 0).PaddingY(design.SpaceSM).Background(cs.SurfaceContainerHigh).Radius(design.CornerXL).Children(func() {
		m3.Text(c, l10n.ChatOutlineChapters(len(chapters)), tt.LabelLarge.C(cs.Primary)).Padding(design.SpaceXS, design.SpaceLG, design.SpaceSM, design.SpaceLG)
		for i, ch := range chapters {
			ch := ch
			row := ui.ButtonBase(c.Key(i)).FocusRing(false).Cursor(ui.CursorPointer).Row().FillWidth().AlignItems(ui.Start).Padding(6, design.SpaceLG)
			m3.InkWell(row, 0)
			row.Children(func() {
				m3.Text(c, core.FmtClock(ch.I("start_ms")), tt.LabelMedium.C(cs.Primary).H(1.6)).Width(56).Shrink(0).FontFeatures("tnum")
				m3.Text(c, ch.S("title"), tt.BodyMedium).Grow(1).Basis(0)
			})
			if row.Clicked() {
				p.runAction(core.J{"kind": "seek", "lecture_id": lectureID, "seconds": float64(ch.I("start_ms") / 1000)})
			}
		}
	})
}

// slideStrip is thumbnails of the slides the assistant looked at.
func (p *AssistantPanel) slideStrip(c *ui.Context, b *ChatBlock, lectureID string, slides []int, mode string) {
	cs := design.S()
	tt := design.TT()
	st := p.slides[b]
	if st == nil {
		st = &ui.ScrollState{}
		p.slides[b] = st
	}
	ui.Column(c).FillWidth().Children(func() {
		ui.Box(c).FillWidth().Height(72).Children(func() {
			widgets.HScroller{State: st, Fade: true, Gap: design.SpaceSM}.Show(c, func() {
				for _, idx := range slides {
					idx := idx
					t := m3.Tappable(c.Key(idx), design.CornerMD).Height(72).Width(128).Clip()
					m3.Tip(c, t, l10n.ChatSlideRef(idx))
					t.Children(func() {
						widgets.NetImage(c, SlideImageURL(lectureID, idx), func() {
							ui.Box(c).Fill().Center().Background(cs.SurfaceContainerHighest).Children(func() {
								m3.Icon(c, icons.BrokenImageOutlined, 18, cs.OnSurfaceVariant)
							})
						})
					})
					if t.Clicked() {
						OpenRef(CoursiveRef{Kind: RefSlide, LectureID: lectureID, Idx: idx}, lectureID, nil)
					}
				}
			})
		})
		if mode == "described" || mode == "unavailable" {
			text, col := l10n.ChatVisionTranscribed, cs.OnSurfaceVariant
			if mode == "unavailable" {
				text, col = l10n.ChatVisionUnavailable, cs.Error
			}
			m3.Text(c, text, tt.LabelSmall.C(col)).Margin(design.SpaceXS, 0, 0, 0)
		}
	})
}

// noteCard is a note the assistant saved, opening the reader.
func (p *AssistantPanel) noteCard(c *ui.Context, note core.J) {
	cs := design.S()
	tt := design.TT()
	t := m3.Tappable(c.Key("note-"+note.S("id")), design.CornerLG).FillWidth().Background(cs.SecondaryContainer)
	t.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(design.SpaceMD, design.SpaceMD, design.SpaceMD, design.SpaceLG).Children(func() {
			m3.Icon(c, icons.MenuBookRounded, 24, cs.OnSecondaryContainer)
			m3.HGap(c, design.SpaceMD)
			ui.Column(c).Grow(1).Basis(0).Children(func() {
				m3.Clamp(m3.Text(c, note.Or("title", l10n.CommonNotes), tt.TitleSmall.C(cs.OnSecondaryContainer)), 2)
				m3.Text(c, l10n.ChatSavedToLibrary, tt.BodySmall.C(design.WithOpacity(cs.OnSecondaryContainer, .8)))
			})
			m3.Icon(c, icons.ArrowForwardRounded, 24, cs.OnSecondaryContainer)
		})
	})
	if t.Clicked() {
		p.runAction(core.J{"kind": "open_note", "note_id": note.S("id")})
	}
}

func (p *AssistantPanel) actions(c *ui.Context, actions []core.J) {
	icon := func(k string) icons.Icon {
		switch k {
		case "seek":
			return icons.PlayCircleOutlineRounded
		case "open_lecture":
			return icons.OndemandVideoRounded
		case "open_course":
			return icons.SchoolOutlined
		case "open_note":
			return icons.DescriptionOutlined
		case "download":
			return icons.DownloadOutlined
		case "open_schedule":
			return icons.EventNoteOutlined
		}
		return icons.TouchAppOutlined
	}
	ui.Row(c).FillWidth().Wrap().Gap(design.SpaceSM).PaddingY(design.SpaceXS).Children(func() {
		for i, act := range actions {
			act := act
			label := act.S("label")
			if act.S("kind") == "seek" && act["seconds"] != nil {
				label += " · " + core.FmtClock(int(act.F("seconds")*1000+.5))
			}
			if (m3.Button{Kind: m3.Tonal, Icon: icon(act.S("kind")), Label: label}).Show(c.Key(i)).Clicked() {
				p.runAction(act)
			}
		}
	})
}
