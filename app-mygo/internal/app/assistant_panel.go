package app

import (
	"math"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// assistant_panel.dart: the resident assistant, docked beside the shell
// on wide windows, full screen on phones, in a sheet, and as a tab inside
// the player.

// dockedAssistant is the shell's docked panel.
var dockedAssistant = NewAssistantPanel(true, false)

// OpenAssistantPanel opens the assistant: docks it on wide windows, full
// screen on phones.
func OpenAssistantPanel(c *ui.Context) {
	if design.SizeClassOf(WindowWidth) >= design.SizeExpanded {
		core.UI.Update(func(p *design.Prefs) { p.AssistantDocked = true })
		return
	}
	m3.RootNav.Push(newAssistantPage())
}

// OpenCourseAssistant opens a course's agent in the assistant (the course
// page's 课程助手).
func OpenCourseAssistant(c *ui.Context, courseID, title string) {
	OpenAssistantPanel(c)
	Assistant.OpenCourseAgent(courseID, title, false)
}

// assistantPage is the full-screen assistant of phones.
type assistantPage struct {
	panel  *AssistantPanel
	cancel func()
}

func newAssistantPage() *assistantPage {
	p := &assistantPage{panel: NewAssistantPanel(false, false)}
	p.panel.Leave = func() { m3.RootNav.PopPage(p) }
	// A full-screen panel steps aside when the bot opens a tab behind it.
	p.cancel = Assistant.AppOps.Listen(func(op core.J) {
		switch op.S("target") {
		case "home", "courses", "course", "note", "schedule", "downloads", "library", "tasks", "plugins", "settings":
			if op.S("op") == "open" {
				m3.RootNav.PopPage(p)
			}
		}
	})
	return p
}

func (p *assistantPage) Dispose() {
	p.cancel()
	p.panel.Dispose()
}

func (p *assistantPage) View(c *ui.Context) {
	top, right, bottom, left := c.SafeArea()
	ui.Column(c).Fill().Background(design.S().Surface).Padding(top, right, bottom, left).Children(func() { p.panel.View(c) })
}

// AssistantPanel is the assistant's state in one place it shows.
type AssistantPanel struct {
	Docked bool
	// Embedded is inside another surface (the player's tab, a sheet): no
	// close or undock controls.
	Embedded bool
	// Leave closes the surface the panel is in: the full-screen page, the
	// sheet, the player.
	Leave func()

	list       ui.ListState
	session    string
	input      string
	history    bool
	histScroll ui.ScrollState
	welcome    ui.ScrollState
	loaded     bool
	quizzes    map[string]*QuizCard
	open       map[*ChatBlock]bool
	runtime    *RuntimeSetupCard
	slides     map[*ChatBlock]*ui.ScrollState
}

// NewAssistantPanel is a panel docked beside other content or full
// screen.
func NewAssistantPanel(docked, embedded bool) *AssistantPanel {
	return &AssistantPanel{Docked: docked, Embedded: embedded, list: ui.ListState{FollowEnd: true},
		quizzes: map[string]*QuizCard{}, open: map[*ChatBlock]bool{}, slides: map[*ChatBlock]*ui.ScrollState{}}
}

// Dispose stops what the panel keeps running.
func (p *AssistantPanel) Dispose() {
	for _, q := range p.quizzes {
		q.Dispose()
	}
	p.quizzes = map[string]*QuizCard{}
	if p.runtime != nil {
		p.runtime.Dispose()
		p.runtime = nil
	}
}

func (p *AssistantPanel) send() {
	a := Assistant
	t := p.input
	if strings.TrimSpace(t) == "" || a.Busy() {
		return
	}
	p.input = ""
	a.Send(t)
	p.toBottom()
}

func (p *AssistantPanel) toBottom() { p.list.ScrollToEnd() }

func (p *AssistantPanel) leaveFullscreen() {
	if !p.Docked && !p.Embedded && p.Leave != nil {
		p.Leave()
	}
}

func (p *AssistantPanel) leaveEmbedded() {
	if p.Embedded && p.Leave != nil {
		p.Leave()
	}
}

// runAction runs one of a tool's offered follow-ups.
func (p *AssistantPanel) runAction(act core.J) {
	// Without its own lecture an action belongs to the open player, else
	// the lecture on screen.
	own := act.S("lecture_id")
	lectureID := own
	if lectureID == "" {
		lectureID = Screen.Value["lecture_id"]
	}
	courseID := act.S("course_id")
	_, hasSecs := act["seconds"].(float64)
	secs := int(math.Round(act.F("seconds")))
	switch act.S("kind") {
	case "seek":
		if pl, ok := Players.Value.(*LecturePlayer); ok && pl != nil && (own == "" || pl.LectureID() == own) {
			pl.SeekTo(secs * 1000)
			p.leaveFullscreen()
			revealPlayer(pl)
		} else if lectureID != "" {
			p.leaveFullscreen()
			m3.RootNav.Push(NewPlayerPage(lectureID, secs*1000))
		} else {
			widgets.ShowSnack(l10n.PlayerNoLectureAtTime, "", nil)
		}
	case "open_lecture":
		if lectureID == "" {
			widgets.ShowSnack(l10n.BotNoLecture, "", nil)
			return
		}
		p.leaveFullscreen()
		start := -1
		if hasSecs {
			start = secs * 1000
		}
		m3.RootNav.Push(NewPlayerPage(lectureID, start))
	case "open_course":
		if courseID == "" {
			widgets.ShowSnack(l10n.BotNoCourse, "", nil)
			return
		}
		p.leaveFullscreen()
		p.leaveEmbedded()
		PushTab(TabCourses, NewCoursePage(courseID))
	case "open_note":
		id := act.S("note_id")
		if id == "" {
			widgets.ShowSnack(l10n.BotNoNote, "", nil)
			return
		}
		p.leaveFullscreen()
		p.leaveEmbedded()
		PushTab(TabLibrary, NewNotePage(id))
	case "download":
		if lectureID == "" {
			widgets.ShowSnack(l10n.BotNoDownload, "", nil)
			return
		}
		ShowDownloadSheet([]string{lectureID}, "", act.Ss("kinds"), nil)
	case "open_schedule":
		p.leaveFullscreen()
		p.leaveEmbedded()
		GoTo(TabSchedule)
	default:
		widgets.ShowSnack(l10n.BotUnsupported, "", nil)
	}
}

// View builds the panel filling its parent.
func (p *AssistantPanel) View(c *ui.Context) {
	a := Assistant
	cs := design.S()
	if a.Ready() && !p.loaded {
		p.loaded = true
		if a.Models == nil {
			a.LoadModels(nil)
		}
		if len(a.Sessions) == 0 {
			a.RefreshSessions()
		}
	}
	if a.SessionID != p.session {
		p.session = a.SessionID
		p.list = ui.ListState{FollowEnd: true}
	}
	ui.Column(c).Fill().Children(func() {
		p.header(c)
		ui.Box(c).FillWidth().Grow(1).Basis(0).Clip().Children(func() {
			ui.Column(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Children(func() { p.body(c) })
			// Session history slides over the conversation.
			hist := ui.Column(c).Absolute().Top(0).Bottom(0).Width(320)
			x := hist.AnimateTo("assistant.history", map[bool]float32{false: -400, true: 0}[p.history],
				design.T().Motion(design.Medium4, design.Emphasized))
			hist = hist.Left(x)
			if x <= -399 {
				hist = hist.Hidden(true)
			}
			p.historyPanel(c, hist)
			if a.Ready() && len(a.Messages) > 0 && !a.LoadingSession && !p.list.AtEnd() {
				ui.Box(c).Absolute().Right(design.SpaceLG).Bottom(design.SpaceMD).Children(func() {
					if (m3.FAB{Small: true, Icon: icons.ArrowDownwardRounded, BG: cs.SurfaceContainerHighest}).Show(c).Clicked() {
						p.toBottom()
					}
				})
			}
		})
		if a.Ready() {
			p.composer(c)
		}
	})
}

func (p *AssistantPanel) header(c *ui.Context) {
	a := Assistant
	cs := design.S()
	tt := design.TT()
	isBot := a.View == "bot"
	course := a.Course
	if course == nil {
		course = a.PendingCourse
	}
	ui.Column(c).FillWidth().Padding(design.SpaceSM, design.SpaceSM, design.SpaceXS, design.SpaceSM).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			m3.HGap(c, design.SpaceXS)
			tip := a.BotName() + " · " + a.Mood().Label()
			av := ui.ButtonBase(c).FocusRing(false).Radius(design.CornerFull).Padding(design.SpaceXS).Cursor(ui.CursorPointer).Label(tip)
			m3.InkWell(av, design.CornerFull)
			m3.Tip(c, av, tip)
			av.Children(func() { BotAvatar{Size: 36, Shape: a.BotShape(), Color: a.BotColor(), Mood: a.Mood()}.Show(c) })
			if av.Clicked() {
				m3.RootNav.Push(NewBotPage())
			}
			m3.HGap(c, design.SpaceXS)
			ui.Box(c).Grow(1).Basis(0).Children(func() {
				if v, ok := AssistantViewSwitch(c, a.BotName(), a.View, a.Busy()); ok {
					p.history = false
					if v == "bot" {
						a.ShowBot()
					} else {
						a.ShowSessions()
					}
				}
			})
			if !isBot && (m3.IconButton{Icon: icons.AddCommentOutlined, Tooltip: l10n.ChatNewSession, Disabled: a.Busy()}).Show(c).Clicked() {
				a.NewChat()
			}
			if !p.Embedded {
				ic, tip := icons.CloseRounded, l10n.CommonClose
				if p.Docked {
					ic, tip = icons.LastPageRounded, l10n.CommonCollapse
				}
				if (m3.IconButton{Icon: ic, Tooltip: tip, Color: cs.OnSurfaceVariant}).Show(c).Clicked() {
					if p.Docked {
						core.UI.Update(func(pr *design.Prefs) { pr.AssistantDocked = false })
					} else if p.Leave != nil {
						p.Leave()
					}
				}
			}
		})
		m3.VGap(c, design.SpaceXS)
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			if !isBot {
				if (m3.IconButton{Icon: icons.ForumOutlined, Tooltip: l10n.ChatAllSessions, Toggle: true, Selected: p.history, Box: 32}).Show(c).Clicked() {
					p.history = !p.history
					if p.history {
						a.RefreshSessions()
					}
				}
			} else {
				m3.HGap(c, design.SpaceSM)
			}
			if !isBot && course != nil {
				title := course.Title
				if title == "" {
					title = l10n.CourseAssistant
				}
				ui.Row(c).Shrink(1).MinWidth(0).Margin(0, design.SpaceSM, 0, 0).Children(func() {
					ui.Row(c).Shrink(1).MinWidth(0).AlignItems(ui.Center).Height(24).Padding(0, 8).Gap(4).Radius(design.CornerSM).
						Background(cs.SecondaryContainer).Children(func() {
						m3.Icon(c, icons.SchoolOutlined, 16, cs.OnSecondaryContainer)
						m3.Clamp(m3.Text(c, title, tt.LabelMedium.C(cs.OnSecondaryContainer)), 1).Shrink(1)
					})
				})
			}
			ui.Row(c).Shrink(1).MinWidth(0).Children(func() { modelChip(c) })
			if a.Stats.O("context") != nil {
				contextMeter(c)
			}
		})
	})
}

// AssistantViewSwitch switches the assistant between the bot and the
// session list. Both names show in full where they fit; on a narrower
// panel the segments become icons with tooltips rather than cutting the
// name off. It reports a new choice.
func AssistantViewSwitch(c *ui.Context, botName, view string, disabled bool) (string, bool) {
	// Align(centerStart): the switch keeps its own width.
	box := ui.Row(c).FillWidth().AlignItems(ui.Center)
	st := design.TT().LabelLarge
	w1, _ := c.MeasureText(0, st.Span(botName))
	w2, _ := c.MeasureText(0, st.Span(l10n.ChatSessions))
	const chrome = 2*2*design.SpaceLG + 3
	labels := box.Bounds().W == 0 || box.Bounds().W >= w1+w2+chrome
	v := view
	changed := false
	box.Children(func() {
		segs := []m3.Segment[string]{{Value: "bot", Label: botName, Disabled: disabled}, {Value: "sessions", Label: l10n.ChatSessions, Disabled: disabled}}
		if !labels {
			segs = []m3.Segment[string]{{Value: "bot", Icon: icons.SmartToyOutlined, Tooltip: botName, Disabled: disabled},
				{Value: "sessions", Icon: icons.ForumOutlined, Tooltip: l10n.ChatSessions, Disabled: disabled}}
		}
		changed = m3.Segmented(c, segs, &v, m3.SegOpts{NoCheck: true, Compact: true})
	})
	return v, changed
}

func (p *AssistantPanel) historyPanel(c *ui.Context, e ui.Element) {
	cs := design.S()
	tt := design.TT()
	e = e.Background(cs.SurfaceContainerHigh).Radius(0, design.CornerXL, design.CornerXL, 0).Clip()
	m3.Elevation(e, 2)
	e.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(design.SpaceMD, design.SpaceSM, design.SpaceSM, design.SpaceLG).Children(func() {
			m3.Text(c, l10n.ChatAllSessions, tt.TitleSmall).Grow(1).Basis(0)
			if (m3.IconButton{Icon: icons.CloseRounded}).Show(c).Clicked() {
				p.history = false
			}
		})
		ui.Column(c).FillWidth().Grow(1).Basis(0).Children(func() { p.sessionList(c) })
	})
}

// sessionList is course agents grouped by course, then other chats and
// scheduled runs.
func (p *AssistantPanel) sessionList(c *ui.Context) {
	a := Assistant
	cs := design.S()
	tt := design.TT()
	groups := map[string][]ChatSession{}
	var order []string
	for _, s := range a.Sessions {
		if s.Kind() == "bot" {
			continue
		}
		g := l10n.ChatOtherSessions
		switch s.Kind() {
		case "course":
			g = s.Course()
			if g == "" {
				g = l10n.CourseAssistant
			}
		case "task":
			g = l10n.CommonScheduledTasks
		}
		if _, ok := groups[g]; !ok && g != l10n.ChatOtherSessions && g != l10n.CommonScheduledTasks {
			order = append(order, g)
		}
		groups[g] = append(groups[g], s)
	}
	for _, g := range []string{l10n.ChatOtherSessions, l10n.CommonScheduledTasks} {
		if len(groups[g]) > 0 {
			order = append(order, g)
		}
	}
	if len(order) == 0 {
		ui.Column(c).Fill().Center().Children(func() {
			m3.Text(c, l10n.ChatNoSessions, tt.BodyMedium.C(cs.OnSurfaceVariant))
		})
		return
	}
	ui.Scroll(c).Fill().TrackScroll(&p.histScroll).Children(func() {
		ui.Column(c).FillWidth().Padding(0, design.SpaceSM).Children(func() {
			for _, g := range order {
				m3.Clamp(m3.Text(c.Key("g-"+g), g, tt.LabelLarge.C(cs.Primary)), 1).Padding(design.SpaceMD, design.SpaceMD, design.SpaceXS, design.SpaceMD)
				for _, s := range groups[g] {
					p.sessionTile(c, s)
				}
			}
		})
	})
}

func (p *AssistantPanel) sessionTile(c *ui.Context, s ChatSession) {
	a := Assistant
	id := s.ID()
	t := m3.Tile{
		Title: s.Title(), TitleLines: 1, Subtitle: l10n.ChatSessionMeta(core.FmtRelative(s.Modified()), s.Messages()),
		Tap: true, Selected: id == a.SessionID, Radius: design.CornerFull,
		Trailing: func() {
			if (m3.IconButton{Icon: icons.DeleteOutlineRounded, Size: 20, Tooltip: l10n.CommonDelete}).Show(c).Clicked() {
				a.DeleteSession(id, nil)
			}
		},
	}
	if id == a.SessionID {
		t.TileColor = design.S().SecondaryContainer
	}
	if t.Show(c.Key("s-" + id)).Clicked() {
		a.OpenSession(id)
		p.history = false
	}
}

func (p *AssistantPanel) body(c *ui.Context) {
	a := Assistant
	cs := design.S()
	tt := design.TT()
	if !a.Ready() {
		if st := a.AgentState(); st != "disabled" && st != "crashed" {
			widgets.CenteredLoader(c, l10n.AssistantStarting)
			return
		}
		if p.runtime == nil {
			p.runtime = NewRuntimeSetupCard()
		}
		ui.Scroll(c).Fill().TrackScroll(&p.welcome).Children(func() {
			ui.Column(c).FillWidth().Padding(design.SpaceLG).Children(func() {
				m3.Text(c, l10n.AssistantNotStarted, tt.TitleMedium)
				if e := a.AgentError(); e != "" {
					m3.VGap(c, design.SpaceXS)
					m3.Text(c, e, tt.BodySmall.C(cs.OnSurfaceVariant))
				}
				m3.VGap(c, design.SpaceLG)
				ui.Column(c).FillWidth().AlignItems(ui.Center).Children(func() { p.runtime.Show(c) })
			})
		})
		return
	}
	if p.runtime != nil {
		p.runtime.Dispose()
		p.runtime = nil
	}
	if a.Models != nil && a.Selected() == nil {
		widgets.EmptyState{Icon: icons.KeyOutlined, Title: l10n.AssistantSetupModel, Body: l10n.AssistantSetupModelBody, Action: func() {
			if (m3.Button{Kind: m3.Filled, Icon: icons.TuneRounded, Label: l10n.AssistantSetupModelAction}).Show(c).Clicked() {
				m3.RootNav.Push(NewModelsPage())
			}
		}}.Show(c)
		return
	}
	if a.LoadingSession {
		widgets.CenteredLoader(c, "")
		return
	}
	if len(a.Messages) == 0 {
		p.welcomeView(c)
		return
	}
	msgs := a.Messages
	p.list.Key = func(i int) any { return msgs[i] }
	ui.List(c, &p.list, len(msgs), func(i int) {
		m := msgs[i]
		last := i == len(msgs)-1
		ui.Column(c).FillWidth().Padding(0, design.SpaceLG).Children(func() {
			if i == 0 {
				m3.VGap(c, design.SpaceMD)
			}
			if m.Role == "user" {
				userBubble(c, m.Text)
			} else {
				p.assistantTurn(c, m, last)
			}
			if last {
				m3.VGap(c, design.SpaceLG)
			}
		})
	}).Fill()
}

func (p *AssistantPanel) welcomeView(c *ui.Context) {
	a := Assistant
	cs := design.S()
	tt := design.TT()
	screen := Screen.Value
	inPlayer := Players.Value != nil
	isBot := a.View == "bot"
	course := a.PendingCourse
	if course == nil {
		course = a.Course
	}
	var suggestions []string
	if isBot {
		if inPlayer {
			suggestions = append(suggestions, l10n.SuggestSpeed)
		}
		if screen["course_id"] != "" {
			suggestions = append(suggestions, l10n.SuggestDownloadRecent)
		}
		suggestions = append(suggestions, l10n.SuggestBiggerText, l10n.SuggestRemember, l10n.SuggestWeeklyReview, l10n.SuggestPauseDownloads)
	} else {
		if inPlayer {
			suggestions = append(suggestions, l10n.SuggestSummarizeLecture, l10n.SuggestReviewNotes, l10n.SuggestConceptElsewhere)
		}
		if !inPlayer && course != nil {
			suggestions = append(suggestions, l10n.SuggestCourseRecent, l10n.SuggestUnwatched, l10n.SuggestQuizCourse)
		}
		suggestions = append(suggestions, l10n.SuggestWhatCanYouDo, l10n.SuggestRecentUnwatched, l10n.SuggestSearchMidterm)
	}
	take := 5
	if isBot {
		take = 4
	}
	if len(suggestions) > take {
		suggestions = suggestions[:take]
	}
	var title, body string
	switch {
	case isBot:
		title, body = l10n.BotIntroTitle(a.BotName()), l10n.BotIntroBody
	case course != nil:
		name := l10n.CommonCourses
		if course.Title != "" {
			name = l10n.CourseTitleQuoted(course.Title)
		}
		title, body = l10n.CourseAssistantTitle(name), l10n.CourseAssistantIntro(a.BotName())
	default:
		title, body = l10n.ChatWelcomeTitle, l10n.ChatWelcomeBody
	}
	ui.Scroll(c).Fill().TrackScroll(&p.welcome).Children(func() {
		ui.Column(c).FillWidth().Padding(design.SpaceXL).Children(func() {
			m3.VGap(c, design.SpaceLG)
			ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
				if isBot {
					b := ui.Box(c).Cursor(ui.CursorPointer)
					b.Children(func() { BotAvatar{Size: 112, Shape: a.BotShape(), Color: a.BotColor(), Mood: a.Mood()}.Show(c) })
					if b.Clicked() {
						m3.RootNav.Push(NewBotPage())
					}
					return
				}
				bg, fg, ic := cs.TertiaryContainer, cs.OnTertiaryContainer, icons.AutoAwesomeRounded
				if course != nil {
					bg, fg, ic = cs.SecondaryContainer, cs.OnSecondaryContainer, icons.SchoolRounded
				}
				ui.Box(c).Size(72, 72).Center().Draw(func(pt *ui.Painter, r ui.Rect) {
					path := widgets.StarPath(r, 6, .72, .6, .4)
					pt.FillPath(&path, bg)
				}).Children(func() { m3.Icon(c, ic, 30, fg) })
			})
			m3.VGap(c, design.SpaceLG)
			m3.Text(c, title, tt.HeadlineSmall).TextAlign(ui.Center).FillWidth()
			m3.VGap(c, design.SpaceXS)
			m3.Text(c, body, tt.BodyMedium.C(cs.OnSurfaceVariant).H(1.5)).TextAlign(ui.Center).FillWidth()
			if !isBot && course == nil && screen["course_id"] != "" {
				m3.VGap(c, design.SpaceMD)
				ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
					name := screen["course"]
					if name == "" {
						name = l10n.ChatCurrentCourse
					}
					if (m3.Button{Kind: m3.Tonal, Icon: icons.SchoolOutlined, Label: l10n.ChatSwitchCourse(name)}).Show(c).Clicked() {
						a.OpenCourseAgent(screen["course_id"], screen["course"], true)
					}
				})
			}
			m3.VGap(c, design.SpaceXL)
			for _, s := range suggestions {
				b := m3.Tappable(c.Key("sg-"+s), design.CornerLG).FillWidth().Background(cs.SurfaceContainerHigh).Margin(0, 0, design.SpaceSM, 0)
				b.Children(func() {
					ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(design.SpaceMD, design.SpaceLG).Children(func() {
						m3.Text(c, s, tt.BodyMedium).Grow(1).Basis(0)
						m3.Icon(c, icons.NorthEastRounded, 16, cs.OnSurfaceVariant)
					})
				})
				if b.Clicked() {
					p.input = s
					p.send()
				}
			}
		})
	})
}

func (p *AssistantPanel) composer(c *ui.Context) {
	a := Assistant
	cs := design.S()
	tt := design.TT()
	screen := Screen.Value
	inPlayer := Players.Value != nil
	ctxLabel := l10n.ChatContextPlaying
	if !inPlayer {
		where := screen["course"]
		if where == "" {
			where = screen["screen"]
		}
		ctxLabel = l10n.ChatContextScreen(where)
	}
	pctx := core.J{"session_id": a.SessionID}
	ui.Column(c).FillWidth().Padding(design.SpaceXS, design.SpaceMD, design.SpaceMD, design.SpaceMD).Children(func() {
		ui.Column(c).FillWidth().Background(cs.SurfaceContainerHighest).Radius(design.CornerXL).
			Padding(design.SpaceSM, design.SpaceSM, design.SpaceSM, design.SpaceXS).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				m3.HGap(c, design.SpaceMD)
				ic := icons.MyLocationRounded
				if inPlayer {
					ic = icons.PlayCircleOutlineRounded
				}
				m3.Icon(c, ic, 14, cs.OnSurfaceVariant)
				m3.HGap(c, 4)
				m3.Clamp(m3.Text(c, ctxLabel, tt.LabelSmall.C(cs.OnSurfaceVariant)), 1).Grow(1).Basis(0)
				m3.HGap(c, design.SpaceSM)
				ApprovalModeChip(c, a.Approval(), setApproval)
			})
			modeChips(c, pctx)
			ui.Row(c).FillWidth().AlignItems(ui.End).Children(func() {
				plusMenu(c, pctx, func(prompt string) {
					if a.Busy() {
						return
					}
					a.Send(prompt)
					p.toBottom()
				})
				var in ui.Element
				ui.Box(c).Grow(1).Basis(0).Children(func() {
					in = m3.Field{Hint: l10n.ChatInputHint, Borderless: true, MinLines: 1, MaxLines: 6, Pad: true, PadX: 0, PadY: 10,
						Style: &tt.BodyLarge}.Show(c, &p.input)
				})
				if f := in.Focused(); f != a.ComposerFocused {
					a.SetComposerFocused(f)
				}
				// Enter sends and Shift+Enter breaks the line; Shift+Tab
				// switches the approval mode.
				in.HandleInput(func(ev ui.InputEvent) bool {
					if ev.Kind != ui.InputKeyDown {
						return false
					}
					switch {
					case ev.Key == ui.KeyEnter && ev.Mods == 0:
						core.Post(p.send)
						return true
					case ev.Key == ui.KeyTab && ev.Mods == ui.Shift:
						core.Post(func() {
							m := "ask"
							if Assistant.Approval() == "ask" {
								m = "auto"
							}
							setApproval(m)
						})
						return true
					}
					return false
				})
				m3.HGap(c, design.SpaceSM)
				enter := ui.ElementTransition{Duration: design.Short4, Enter: &ui.Motion{Scale: .2, Opacity: 0}}
				if a.Busy() {
					ui.Box(c.Key("stop")).Transition(enter).Children(func() {
						if (m3.IconButton{Kind: m3.TonalIcon, Icon: icons.StopRounded, Tooltip: l10n.CommonStop}).Show(c).Clicked() {
							a.Stop()
						}
					})
				} else {
					ui.Box(c.Key("send")).Transition(enter).Children(func() {
						if (m3.IconButton{Kind: m3.FilledIcon, Icon: icons.ArrowUpwardRounded, Tooltip: l10n.ChatSend,
							Disabled: strings.TrimSpace(p.input) == ""}).Show(c).Clicked() {
							p.send()
						}
					})
				}
			})
		})
	})
}

var approvalIcons = map[string]icons.Icon{"ask": icons.FrontHandOutlined, "auto": icons.MotionPhotosAutoOutlined}

func approvalName(m string) string {
	if m == "ask" {
		return l10n.ApprovalAsk
	}
	return l10n.ApprovalAuto
}

func setApproval(mode string) {
	a := Assistant
	if mode == a.Approval() {
		return
	}
	a.SetApproval(mode, func(err error) {
		if err != nil {
			widgets.ShowSnack(l10n.ApprovalFailed, "", nil)
		}
	})
}

// ApprovalModeChip is the approval mode at the end of the composer's
// context line, like the permission modes of coding agents: 询问 turns
// the assistant's changes into cards the user applies, 自动 runs them with
// 撤销. Shift+Tab in the composer switches it. Only the accent marks 自动;
// nothing is filled, so the send button stays the one filled action.
func ApprovalModeChip(c *ui.Context, mode string, onChanged func(string)) {
	cs := design.S()
	tt := design.TT()
	fg := cs.OnSurfaceVariant
	if mode == "auto" {
		fg = cs.Primary
	}
	compact := Compact()
	tip := l10n.ApprovalTooltip
	note := l10n.ApprovalDeleteNote
	if !compact {
		tip += " · " + l10n.ApprovalSwitchHint
		note += " · " + l10n.ApprovalSwitchHint
	}
	m3.Menu(c, "approval", m3.BelowEnd, func(t ui.Element, open bool) {
		t = t.Row().AlignItems(ui.Center).MinHeight(32).Padding(0, design.SpaceSM).Radius(design.CornerFull).Cursor(ui.CursorPointer).
			Label(l10n.ApprovalTooltip + "：" + approvalName(mode))
		m3.InkWell(t, design.CornerFull)
		if !open {
			m3.Tip(c, t, tip)
		}
		t.Children(func() {
			// The icon swaps in from a quarter size, so a switch by
			// Shift+Tab is noticed without moving the composer.
			ui.Box(c.Key(mode)).Transition(ui.ElementTransition{Duration: design.Short4, Ease: design.EmphasizedDecelerate,
				Enter: &ui.Motion{Opacity: 0, Scale: .25}}).Children(func() { m3.Icon(c, approvalIcons[mode], 16, fg) })
			m3.HGap(c, design.SpaceXS)
			m3.Text(c, approvalName(mode), tt.LabelMedium.C(fg)).NoWrap()
			m3.HGap(c, design.SpaceXXS)
			m3.Icon(c, icons.UnfoldMoreRounded, 14, cs.OnSurfaceVariant)
		})
	}, func() []m3.MenuItem {
		var items []m3.MenuItem
		for _, m := range []string{"ask", "auto"} {
			m := m
			ic := cs.OnSurfaceVariant
			if m == mode {
				ic = cs.Primary
			}
			detail := l10n.ApprovalAutoDetail
			if m == "ask" {
				detail = l10n.ApprovalAskDetail
			}
			items = append(items, m3.MenuItem{
				LeadingFn: func(c *ui.Context) { m3.Icon(c, approvalIcons[m], 20, ic) },
				Custom: func(c *ui.Context) {
					ui.Column(c).Grow(1).MinWidth(168).MaxWidth(220).PaddingY(design.SpaceSM).Children(func() {
						m3.Text(c, approvalName(m), tt.LabelLarge)
						m3.VGap(c, design.SpaceXXS)
						m3.Text(c, detail, tt.BodySmall.C(cs.OnSurfaceVariant))
					})
					check := ui.Transparent
					if m == mode {
						check = cs.Primary
					}
					m3.Icon(c, icons.CheckRounded, 18, check)
				},
				OnTap: func() { onChanged(m) },
			})
		}
		items = append(items, m3.MenuItem{Panel: func(c *ui.Context) {
			m3.Text(c, note, tt.LabelSmall.C(cs.OnSurfaceVariant)).MaxWidth(264).Padding(design.SpaceXS, design.SpaceLG, design.SpaceSM, design.SpaceLG)
		}})
		return items
	})
}

func pluginActive(e core.PluginEntry) bool {
	return Plugins.ByID(e.PluginID()).O("status").IsTrue("active")
}

// plusMenu is the composer's "+" menu: plugin actions for the chat, and
// the way to manage plugins.
func plusMenu(c *ui.Context, pctx core.J, onPrompt func(string)) {
	cs := design.S()
	entries := Plugins.Entries("actions", "composer.plus", "")
	m3.Menu(c, "plus", m3.Below, func(t ui.Element, open bool) {
		t = t.Size(32, 32).Center().Radius(design.CornerFull).Cursor(ui.CursorPointer).Margin(0, design.SpaceXS, 6, 0).Label(l10n.ChatMoreTools)
		m3.InkWell(t, design.CornerFull)
		if !open {
			m3.Tip(c, t, l10n.ChatMoreTools)
		}
		t.Children(func() { m3.Icon(c, icons.AddCircleOutlineRounded, 24, cs.OnSurfaceVariant) })
	}, func() []m3.MenuItem {
		var items []m3.MenuItem
		for _, e := range entries {
			e := e
			it := m3.MenuItem{Label: e.Title, IconSize: 20, LeadingFn: func(c *ui.Context) { m3.Icon(c, widgets.PluginIcon(e.Icon), 20, cs.OnSurfaceVariant) },
				OnTap: func() { runPluginEntry(e, pctx, onPrompt) }}
			if pluginActive(e) {
				it.TrailingIcon, it.Color = icons.CheckRounded, ui.Color{}
			}
			items = append(items, it)
		}
		if len(entries) > 0 {
			items = append(items, m3.MenuItem{Divider: true})
		}
		ready := Plugins.Ready()
		items = append(items,
			m3.MenuItem{Icon: icons.ExploreOutlined, IconSize: 20, Label: l10n.PluginsDiscover, Disabled: !ready, OnTap: func() { OpenPlugins(false) }},
			m3.MenuItem{Icon: icons.ExtensionOutlined, IconSize: 20, Label: l10n.PluginsManage, Disabled: !ready, OnTap: func() { OpenPlugins(true) }})
		return items
	})
}

// modeChips are the composer modes that plugins switched on (a toggle
// action whose status is {active: true}), each removable in place.
func modeChips(c *ui.Context, pctx core.J) {
	var on []core.PluginEntry
	for _, e := range Plugins.Entries("actions", "composer.plus", "") {
		if pluginActive(e) {
			on = append(on, e)
		}
	}
	box := ui.Box(c).FillWidth().Clip().Transition(ui.ElementTransition{Duration: design.Medium2, Ease: design.Emphasized, Height: true})
	box.Children(func() {
		if len(on) == 0 {
			return
		}
		ui.Row(c).FillWidth().Wrap().Gap(design.SpaceSM).Padding(design.SpaceSM, 0, 0, design.SpaceMD).Children(func() {
			for _, e := range on {
				e := e
				r := m3.Chip{Kind: m3.InputChip, Label: e.Title, Avatar: widgets.PluginIcon(e.Icon), Selected: true, NoCheck: true, Compact: true,
					Deletable: true, DeleteTooltip: l10n.CommonClose}.Show(c.Key(e.PluginID() + e.ID))
				if r.Deleted {
					runPluginEntry(e, pctx, nil)
				}
			}
		})
	})
}

// JSONProviders are the providers of GET /agent/models.
func JSONProviders(models core.J) []core.J { return models.Os("providers") }

func modelChip(c *ui.Context) {
	a := Assistant
	cs := design.S()
	tt := design.TT()
	sel := a.Selected()
	m3.Menu(c, "model", m3.Below, func(t ui.Element, open bool) {
		t = t.Row().AlignItems(ui.Center).Height(32).Padding(0, 8).Radius(design.CornerFull).Background(cs.SurfaceContainerHigh).
			Cursor(ui.CursorPointer).Shrink(1).MinWidth(0).Label(a.ModelLabel())
		m3.InkWell(t, design.CornerFull)
		if t.Clicked() && a.Models == nil {
			a.LoadModels(nil)
		}
		t.Children(func() {
			ui.Row(c).Size(20, 20).Center().Shrink(0).Children(func() {
				if sel == nil {
					m3.Icon(c, icons.BoltRounded, 16, cs.Primary)
				} else {
					ProviderLogo{ProviderID: sel.S("provider"), Size: 20}.Show(c)
				}
			})
			m3.Clamp(m3.Text(c, a.ModelLabel(), tt.LabelLarge.C(cs.OnSurface)), 1).Padding(0, 8).Shrink(1)
		})
	}, func() []m3.MenuItem {
		var items []m3.MenuItem
		for _, pr := range JSONProviders(a.Models) {
			if !pr.IsTrue("configured") {
				continue
			}
			pr := pr
			var sub []m3.MenuItem
			n := 0
			for _, m := range pr.Os("models") {
				if !m.IsTrue("available") || n >= 12 {
					continue
				}
				n++
				m := m
				on := sel != nil && sel.S("provider") == pr.S("id") && sel.S("model") == m.S("id")
				it := m3.MenuItem{Label: m.S("name"), Checked: &on, IconSize: 18,
					OnTap: func() { a.SetPrefs(core.J{"provider": pr.S("id"), "model": m.S("id")}, nil) }}
				if m.IsTrue("reasoning") {
					it.TrailingIcon = icons.PsychologyOutlined
				}
				sub = append(sub, it)
			}
			items = append(items, m3.MenuItem{Label: pr.S("name"), Sub: sub,
				LeadingFn: func(c *ui.Context) { ProviderLogo{ProviderID: pr.S("id"), Name: pr.S("name"), Size: 22}.Show(c) }})
		}
		if sel.IsTrue("reasoning") {
			items = append(items, m3.MenuItem{Divider: true})
			it := m3.MenuItem{Icon: icons.PsychologyOutlined, IconSize: 18, Label: l10n.ThinkingTitle, Trailing: ThinkingLabel(a.Thinking()), Color: cs.OnSurface}
			if Compact() {
				it.OnTap = ShowThinkingSheet
			} else {
				it.Sub = []m3.MenuItem{{Panel: func(c *ui.Context) {
					ui.Column(c).Width(300).Padding(design.SpaceMD, design.SpaceLG).Children(func() {
						ThinkingControl(c, a.Thinking(), setThinking, false, true)
					})
				}}}
			}
			items = append(items, it)
		}
		items = append(items, m3.MenuItem{Divider: true},
			m3.MenuItem{Icon: icons.SearchRounded, IconSize: 18, Label: l10n.ModelsSearchAll, OnTap: ShowModelPicker},
			m3.MenuItem{Icon: icons.TuneRounded, IconSize: 18, Label: l10n.ModelsSettings, OnTap: func() { m3.RootNav.Push(NewModelsPage()) }})
		return items
	})
}

// contextMeter is a ring showing how full the model's context window is,
// from Pi session stats.
func contextMeter(c *ui.Context) {
	a := Assistant
	cs := design.S()
	tt := design.TT()
	cx := a.Stats.O("context")
	window := cx.F("window")
	used := cx.F("tokens")
	pct := cx.F("percent")
	if _, ok := cx["percent"].(float64); !ok && window > 0 {
		pct = used / window * 100
	}
	if pct <= 1 && used > 0 && window > 0 && used/window > .01 {
		pct *= 100
	}
	frac := min(max(pct/100, 0), 1)
	color := cs.Primary
	switch {
	case frac > .85:
		color = cs.Error
	case frac > .6:
		color = cs.Tertiary
	}
	tokens := a.Stats.O("tokens")
	cost := a.Stats.F("cost")
	m3.Menu(c, "context", m3.BelowEnd, func(t ui.Element, open bool) {
		tip := l10n.ContextUsageTooltip(int(math.Round(pct)))
		t = t.Size(40, 40).Center().Radius(design.CornerFull).Cursor(ui.CursorPointer).Label(tip)
		m3.InkWell(t, design.CornerFull)
		if !open {
			m3.Tip(c, t, tip)
		}
		v := float64(t.AnimateTo("ctx.frac", float32(frac), design.T().Motion(design.Long2, design.Emphasized)))
		t.Children(func() {
			if a.Compacting {
				m3.Spinner(c, 20, 2.5, cs.Primary)
				return
			}
			m3.Circular(c, m3.Progress{Value: m3.Of(v), Size: 22, Stroke: 3, Color: color, Track: cs.SurfaceContainerHighest})
		})
	}, func() []m3.MenuItem {
		hit := ""
		if cr := tokens.F("cacheRead"); cr > 0 {
			hit = l10n.ContextCacheHit(FmtTokens(cr))
		}
		return []m3.MenuItem{
			{Panel: func(c *ui.Context) {
				ui.Column(c).Padding(design.SpaceMD, design.SpaceLG, design.SpaceSM, design.SpaceLG).Children(func() {
					m3.Text(c, l10n.ContextUsage(FmtTokens(used), FmtTokens(window), int(math.Round(pct))), tt.TitleSmall)
					m3.VGap(c, 4)
					m3.Text(c, l10n.ContextTokens(FmtTokens(tokens.F("input")), FmtTokens(tokens.F("output")), hit), tt.BodySmall)
					if cost > 0 {
						m3.Text(c, l10n.ContextCost(core.Fixed(cost, 4)), tt.BodySmall)
					}
					m3.VGap(c, 4)
					m3.Text(c, l10n.ContextCompactHint, tt.BodySmall.C(cs.OnSurfaceVariant))
				})
			}},
			{Icon: icons.CompressRounded, IconSize: 18, Label: l10n.ContextCompactNow, Disabled: a.Busy() || a.Compacting, OnTap: func() { a.Compact(nil) }},
			{Icon: icons.InsightsOutlined, IconSize: 18, Label: l10n.UsageView, OnTap: func() { m3.RootNav.Push(NewUsagePage()) }},
			{Icon: icons.ScheduleRounded, IconSize: 18, Label: l10n.CommonScheduledTasks, OnTap: func() {
				// Close the assistant when it covers the window, then show
				// the tab.
				m3.CloseAll()
				m3.RootNav.PopUntilRoot()
				GoTo(TabTasks)
			}},
		}
	})
}
