package app

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// side_panel.dart, outline_tab.dart and transcript_review.dart: outline,
// transcript, materials and notes (and the assistant) beside or below the
// video.

var panelTabs = []string{"outline", "transcript", "materials", "notes", "assistant"}

// PlayerSidePanel is the panel's state; each tab keeps its own while
// another shows (AutomaticKeepAliveClientMixin).
type PlayerSidePanel struct {
	ctl *LecturePlayer
	tab int
	// Assistant builds the embedded assistant, the last tab.
	Assistant func(c *ui.Context)
	// OnAsk hands a prompt to the assistant.
	OnAsk func(string)

	outline    outlineTab
	transcript transcriptTab
	materials  materialsTab
	notes      notesTab
}

// NewPlayerSidePanel is the panel of ctl.
func NewPlayerSidePanel(ctl *LecturePlayer) *PlayerSidePanel {
	p := &PlayerSidePanel{ctl: ctl}
	p.outline.current = -1
	p.transcript = transcriptTab{current: -1, follow: true, review: transcriptReview{visible: true}}
	p.materials.slides.current, p.materials.slides.keyOnly = -1, true
	p.notes.quizzes = NewQuizList(ctl.LectureID())
	return p
}

// Dispose stops what the tabs keep running.
func (p *PlayerSidePanel) Dispose() {
	p.notes.close()
	p.notes.quizzes.Dispose()
	p.transcript.review.stop()
}

func (p *PlayerSidePanel) ask(text string) {
	if p.Assistant != nil {
		p.tab = 4
	}
	if p.OnAsk != nil {
		p.OnAsk(text)
	}
}

// Show builds the panel filling its parent.
func (p *PlayerSidePanel) Show(c *ui.Context) {
	ctl := p.ctl
	if ctl.PanelTab != "" {
		for i, n := range panelTabs {
			if n == ctl.PanelTab && (i < 4 || p.Assistant != nil) {
				p.tab = i
			}
		}
		ctl.PanelTab = ""
	}
	count := func(label string, n int) string {
		if n == 0 {
			return label
		}
		return label + " " + strconv.Itoa(n)
	}
	tabs := []m3.Tab{
		{Label: count(l10n.CommonOutline, len(ctl.Chapters))},
		{Label: l10n.CommonTranscript},
		{Label: count(l10n.PanelMaterials, len(ctl.Slides)+len(ctl.Courseware))},
		{Label: count(l10n.CommonNotes, len(ctl.Notes)+len(ctl.Bookmarks))},
	}
	if p.Assistant != nil {
		tabs = append(tabs, m3.Tab{Label: l10n.CommonAssistant})
	}
	var ask func(string)
	if p.OnAsk != nil {
		ask = p.ask
	}
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(design.SpaceXS, design.SpaceXS, design.SpaceXS, design.SpaceSM).Children(func() {
			ui.Box(c).Grow(1).Basis(0).Children(func() {
				m3.TabBar(c, tabs, &p.tab, m3.TabOpts{Scrollable: true, Pill: true})
			})
			pctx := core.J{"lecture_id": ctl.LectureID(), "t": ctl.PositionMs() / 1000}
			if ctl.Lecture != nil && ctl.Lecture.CourseID() != "" {
				pctx["course_id"] = ctl.Lecture.CourseID()
			}
			PluginMenuButton(c, "lecture.menu", pctx, false, "")
		})
		m3.TabView(c, ui.Column(c).FillWidth().Grow(1).Basis(0), p.tab, func(tab int) {
			switch tab {
			case 0:
				p.outline.show(c, ctl)
			case 1:
				p.transcript.show(c, ctl, ask)
			case 2:
				p.materials.show(c, ctl)
			case 3:
				p.notes.show(c, ctl, ask)
			default:
				if p.Assistant != nil {
					p.Assistant(c)
				}
			}
		})
	})
}

// userScroll marks the list's own scrolling, which follow-the-playhead
// waits on.
func userScroll(e ui.Element, at *time.Time) {
	e.HandleInput(func(ev ui.InputEvent) bool {
		if ev.Kind == ui.InputScroll || ev.Kind == ui.InputPointerDown {
			*at = time.Now()
		}
		return false
	})
}

// ---------------------------------------------------------------- outline

// skillClock is the skill's timeline format: minutes may exceed 59.
func skillClock(ms int) string {
	s := ms / 1000
	return fmt.Sprintf("%02d:%02d", s/60, s%60)
}

type outlineTab struct {
	list     ui.ListState
	current  int
	scrolled time.Time
}

func (o *outlineTab) generate(ctl *LecturePlayer) {
	ctl.GenerateOutline(core.UI.OutlineDetailed, func() {
		if e := ctl.OutlineError; e != "" {
			widgets.ShowSnack(l10n.OutlineFailed(e), "", nil)
		}
	})
}

// setDetail changes how finely outlines are cut; an existing outline is
// rewritten at the new grain, and undo puts the previous one back.
func (o *outlineTab) setDetail(ctl *LecturePlayer, detailed bool) {
	if core.UI.OutlineDetailed == detailed {
		return
	}
	core.UI.Update(func(p *design.Prefs) { p.OutlineDetailed = detailed })
	if len(ctl.Chapters) == 0 || ctl.OutlineBusy {
		return
	}
	old := ctl.Chapters
	ctl.GenerateOutline(detailed, func() {
		if e := ctl.OutlineError; e != "" {
			widgets.ShowSnack(l10n.OutlineFailed(e), "", nil)
			return
		}
		mode := l10n.OutlineConcise
		if detailed {
			mode = l10n.OutlineDetailed
		}
		widgets.ShowSnack(l10n.OutlineRedone(mode), l10n.CommonUndo, func() {
			core.UI.Update(func(p *design.Prefs) { p.OutlineDetailed = !detailed })
			ctl.RestoreOutline(old)
		})
	})
}

// grain is 精简 | 详细: the grain of the next outline (and of this one,
// rewritten).
func (o *outlineTab) grain(c *ui.Context, ctl *LecturePlayer, enabled bool) {
	v := core.UI.OutlineDetailed
	seg := ui.Box(c).Key("outline-detail").Shrink(0)
	m3.Tip(c, seg, l10n.OutlineDetailHint)
	seg.Children(func() {
		if m3.Segmented(c, []m3.Segment[bool]{{Value: false, Label: l10n.OutlineConcise, Disabled: !enabled}, {Value: true, Label: l10n.OutlineDetailed, Disabled: !enabled}}, &v,
			m3.SegOpts{NoCheck: true, Compact: true}) {
			o.setDetail(ctl, v)
		}
	})
}

func (o *outlineTab) show(c *ui.Context, ctl *LecturePlayer) {
	cs, tt := design.S(), design.TT()
	if len(ctl.Chapters) == 0 {
		switch {
		case ctl.OutlineBusy:
			outlineDrafting(c)
		case len(ctl.Segments) == 0:
			widgets.EmptyState{Icon: icons.ViewTimelineOutlined, Title: l10n.OutlineNeedsTranscript, Body: l10n.OutlineNeedsTranscriptBody,
				Action: func() {
					if (m3.Button{Kind: m3.Tonal, Icon: icons.SubjectRounded, Label: l10n.AsrStart}).Show(c).Clicked() {
						State.Enqueue([]string{ctl.LectureID()}, []string{"transcript"}, func(_ []core.Job, err error) {
							if err == nil {
								widgets.ShowSnack(l10n.AsrQueued, "", nil)
							}
						})
					}
				}}.Show(c)
		default:
			body, label := l10n.OutlineBody, l10n.OutlineGenerate
			if ctl.OutlineError != "" {
				body, label = l10n.OutlineLastError(ctl.OutlineError), l10n.CommonRetry
			}
			widgets.EmptyState{Icon: icons.ViewTimelineOutlined, Title: l10n.OutlineTitle, Body: body, Action: func() {
				ui.Column(c).AlignItems(ui.Center).Children(func() {
					o.grain(c, ctl, true)
					m3.VGap(c, design.SpaceLG)
					if (m3.Button{Kind: m3.Filled, Icon: icons.AutoAwesomeRounded, Label: label}).Show(c).Clicked() {
						o.generate(ctl)
					}
				})
			}}.Show(c)
		}
		return
	}
	pos := ctl.PositionMs()
	if i := ctl.ChapterAt(pos); i != o.current {
		o.current = i
		if i >= 0 && time.Since(o.scrolled) > 5*time.Second {
			o.list.ScrollTo(i, ui.Center)
		}
	}
	total := ctl.Chapters[len(ctl.Chapters)-1].EndMs()
	ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(design.SpaceSM, design.SpaceXS, 0, design.SpaceLG).Children(func() {
		label := l10n.OutlineSummary(len(ctl.Chapters), core.FmtClock(total))
		if ctl.OutlineBusy {
			label = l10n.OutlineRegenerating
		}
		m3.Text(c, label, tt.LabelLarge.C(cs.OnSurfaceVariant)).MaxLines(1).Grow(1).Basis(0)
		m3.HGap(c, design.SpaceSM)
		o.grain(c, ctl, !ctl.OutlineBusy)
		m3.MenuButton(c, "outline-more", icons.MoreVertRounded, l10n.OutlineOptions, m3.Below, func() []m3.MenuItem {
			return []m3.MenuItem{
				{Icon: icons.RefreshRounded, Label: l10n.CommonRegenerate, Disabled: ctl.OutlineBusy, OnTap: func() { o.generate(ctl) }},
				{Icon: icons.CopyRounded, Label: l10n.CommonCopy, OnTap: func() {
					var lines []string
					for _, ch := range ctl.Chapters {
						lines = append(lines, l10n.OutlineChapterLine(skillClock(ch.StartMs()), skillClock(ch.EndMs()), ch.Title()))
					}
					c.WriteClipboard(strings.Join(lines, "\n"))
					widgets.ShowSnack(l10n.OutlineCopied, "", nil)
				}},
				{Icon: icons.DeleteOutlineRounded, Label: l10n.CommonDelete, Disabled: ctl.OutlineBusy, OnTap: func() {
					old := ctl.Chapters
					ctl.DeleteOutline()
					widgets.ShowSnack(l10n.OutlineDeleted, l10n.CommonUndo, func() { ctl.RestoreOutline(old) })
				}},
			}
		})
	})
	ui.Box(c.Key(ctl.OutlineBusy)).FillWidth().Height(2).Transition(design.Switcher(design.Medium2, nil, nil, ui.Motion{Opacity: 0})).Children(func() {
		if ctl.OutlineBusy {
			m3.Linear(c, m3.Progress{Height: 2})
		}
	})
	list := ui.List(c, &o.list, len(ctl.Chapters), func(i int) {
		state := 1
		if i < o.current {
			state = 0
		} else if i > o.current {
			state = 2
		}
		chapterRow(c, ctl, ctl.Chapters[i], i+1, state, i == 0, i == len(ctl.Chapters)-1, pos)
	}).FillWidth().Grow(1).Basis(0).Padding(design.SpaceSM, design.SpaceSM, design.SpaceXXL, design.SpaceSM)
	userScroll(list, &o.scrolled)
}

// chapterRow is a chapter on the rail: state 0 passed, 1 playing, 2 ahead.
func chapterRow(c *ui.Context, ctl *LecturePlayer, ch core.Chapter, number, state int, first, last bool, pos int) {
	cs, tt := design.S(), design.TT()
	now := state == 1
	rail := cs.Primary
	if state == 2 {
		rail = cs.OutlineVariant
	}
	ui.Row(c).FillWidth().AlignItems(ui.Stretch).Children(func() {
		// The rail: a line through every chapter, a node per chapter.
		ui.Column(c).Width(36).Shrink(0).AlignItems(ui.Center).Children(func() {
			top := rail
			if first {
				top = ui.Transparent
			}
			ui.Box(c).Width(2).Height(14).Background(top)
			node := ui.Box(c).Shrink(0).Center().Radius(design.CornerFull)
			sz := node.Animate("size", map[bool]float32{true: 24, false: 20}[now], design.Medium2)
			node = node.Size(sz, sz)
			bg, fg := cs.Surface, cs.OnSurfaceVariant
			switch state {
			case 0:
				bg, fg = cs.PrimaryContainer, cs.OnPrimaryContainer
			case 1:
				bg, fg = cs.Primary, cs.OnPrimary
			default:
				node = node.Border(1.5, cs.OutlineVariant)
			}
			node.Background(bg).Children(func() {
				m3.Text(c, strconv.Itoa(number), tt.LabelSmall.S(10).W(700).C(fg))
			})
			below := rail
			if now {
				below = cs.OutlineVariant
			}
			if last {
				below = ui.Transparent
			}
			ui.Box(c).Width(2).Grow(1).MinHeight(4).Background(below)
		})
		ui.Box(c).Grow(1).Basis(0).Padding(0, 0, design.SpaceXS, 0).Children(func() {
			b := m3.Tappable(c, design.CornerLG).FillWidth().Column().Padding(10, design.SpaceMD)
			if now {
				b = b.Background(cs.SecondaryContainer)
			}
			b.Children(func() {
				st := tt.BodyLarge.C(cs.OnSurface)
				if now {
					st = tt.TitleSmall.C(cs.OnSecondaryContainer)
				} else if state == 0 {
					st = st.C(cs.OnSurfaceVariant)
				}
				m3.Text(c, ch.Title(), st.H(1.35))
				m3.VGap(c, 2)
				meta := cs.OnSurfaceVariant
				if now {
					meta = cs.OnSecondaryContainer
				}
				m3.Text(c, core.FmtClock(ch.StartMs())+" – "+core.FmtClock(ch.EndMs())+" · "+chapterMins(ch.EndMs()-ch.StartMs()),
					tt.LabelSmall.C(meta)).FontFeatures("tnum")
				if now {
					span := max(ch.EndMs()-ch.StartMs(), 1)
					f := min(max(float64(pos-ch.StartMs())/float64(span), 0), 1)
					m3.VGap(c, design.SpaceSM)
					m3.Linear(c, m3.Progress{Value: m3.Of(f), Height: 4, Radius: 2, Track: design.WithOpacity(cs.OnSecondaryContainer, .12)})
				}
			})
			if b.Clicked() {
				ctl.SeekTo(ch.StartMs())
			}
		})
	})
}

func chapterMins(ms int) string {
	m := int(float64(ms)/60000 + .5)
	if m < 1 {
		return l10n.DurationUnderMinute
	}
	return l10n.DurationMinutes(m)
}

// outlineDrafting is the placeholder while the model writes the outline.
func outlineDrafting(c *ui.Context) {
	cs, tt := design.S(), design.TT()
	widths := []float32{0.72, 0.55, 0.8, 0.62, 0.7, 0.5}
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).FillWidth().Padding(design.SpaceLG).Children(func() {
			m3.Text(c, l10n.OutlineWorking, tt.TitleSmall)
			m3.VGap(c, 2)
			m3.Text(c, l10n.OutlineWorkingHint, tt.BodySmall.C(cs.OnSurfaceVariant))
			m3.VGap(c, design.SpaceXL)
			// A pulse between 0.35 and 0.9 over 1.4 s, there and back.
			t := float64(time.Now().UnixMilli()%2800) / 1400
			if t > 1 {
				t = 2 - t
			}
			c.After(32 * time.Millisecond)
			op := float32(.35 + .55*float64(design.Standard(float32(t))))
			ui.Column(c).FillWidth().Opacity(op).Children(func() {
				for _, w := range widths {
					ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(0, 0, design.SpaceLG, 0).Children(func() {
						ui.Box(c).Size(20, 20).Shrink(0).Radius(design.CornerFull).Background(cs.SurfaceContainerHighest)
						m3.HGap(c, design.SpaceLG)
						ui.Column(c).Grow(1).Basis(0).Children(func() {
							ui.Column(c).WidthPercent(w * 100).Children(func() {
								ui.Box(c).FillWidth().Height(14).Radius(7).Background(cs.SurfaceContainerHighest)
								m3.VGap(c, 6)
								ui.Box(c).Size(72, 10).Radius(5).Background(cs.SurfaceContainerHigh)
							})
						})
					})
				}
			})
		})
	})
}

// ---------------------------------------------------------------- transcript

type transcriptTab struct {
	list     ui.ListState
	query    string
	current  int
	follow   bool
	scrolled time.Time
	review   transcriptReview
	reveal   int
}

func (t *transcriptTab) show(c *ui.Context, ctl *LecturePlayer, onAsk func(string)) {
	cs, tt := design.S(), design.TT()
	if len(ctl.Segments) == 0 {
		widgets.EmptyState{Icon: icons.SubjectRounded, Title: l10n.TranscriptEmpty, Body: l10n.TranscriptEmptyBody, Action: func() {
			if (m3.Button{Kind: m3.Tonal, Icon: icons.AutoAwesomeOutlined, Label: l10n.AsrStart}).Show(c).Clicked() {
				State.Enqueue([]string{ctl.LectureID()}, []string{"transcript"}, func(_ []core.Job, err error) {
					if err == nil {
						widgets.ShowSnack(l10n.AsrQueuedShowLater, "", nil)
					}
				})
			}
		}}.Show(c)
		return
	}
	q := strings.TrimSpace(t.query)
	marks := map[int][]core.ScheduleItem{}
	for _, it := range Schedule.ForLecture(ctl.LectureID()) {
		if it.TMs() < 0 {
			continue
		}
		if i := ctl.SegmentAt(it.TMs()); i >= 0 {
			marks[i] = append(marks[i], it)
		}
	}
	// The segment that opens each chapter.
	heads := map[int]core.Chapter{}
	if q == "" {
		seg := 0
		for _, ch := range ctl.Chapters {
			for seg < len(ctl.Segments) && ctl.Segments[seg].EndMs() <= ch.StartMs() {
				seg++
			}
			if _, ok := heads[seg]; seg < len(ctl.Segments) && !ok {
				heads[seg] = ch
			}
		}
	}
	var idx []int
	for i, s := range ctl.Segments {
		if q == "" || strings.Contains(s.Text(), q) {
			idx = append(idx, i)
		}
	}
	if i := ctl.SegmentAt(ctl.PositionMs()); i != t.current {
		t.current = i
		if t.follow && i >= 0 && q == "" && time.Since(t.scrolled) >= 5*time.Second {
			t.list.ScrollTo(i, ui.Center)
		}
	}
	suggestions := map[int][]core.J{}
	if t.review.visible && !t.review.data.IsTrue("stale") {
		for _, a := range t.review.data.Os("suggestions") {
			suggestions[a.I("segment")] = append(suggestions[a.I("segment")], a)
		}
	}
	t.review.show(c, ctl.LectureID())
	ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(design.SpaceSM, design.SpaceSM, 0, design.SpaceMD).Children(func() {
		ui.Box(c).Grow(1).Basis(0).Children(func() {
			suffix := ""
			if q != "" {
				suffix = l10n.TranscriptMatches(len(idx))
			}
			in := m3.SearchBar{Hint: l10n.TranscriptFindHint, Height: 40, MaxWidth: -1, Trailing: func() {
				if suffix != "" {
					m3.Text(c, suffix, tt.BodySmall.C(cs.OnSurfaceVariant)).Margin(0, design.SpaceMD, 0, 0)
				}
			}}.Show(c, &t.query)
			_ = in
		})
		tip := l10n.TranscriptFollowOff
		if t.follow {
			tip = l10n.TranscriptFollowOn
		}
		if (m3.IconButton{Icon: icons.MyLocationOutlined, SelectedIcon: icons.MyLocationRounded, Toggle: true, Selected: t.follow, Tooltip: tip}).Show(c).Clicked() {
			t.follow = !t.follow
			if t.follow && t.current >= 0 {
				t.scrolled = time.Time{}
				t.list.ScrollTo(t.current, ui.Center)
			}
		}
		m3.MenuButton(c, "transcript-more", icons.MoreVertRounded, "", m3.Below, func() []m3.MenuItem {
			items := []m3.MenuItem{{Icon: icons.CopyRounded, Label: l10n.TranscriptCopyAll, OnTap: func() {
				var lines []string
				for _, s := range ctl.Segments {
					lines = append(lines, "["+core.FmtClock(s.StartMs())+"] "+s.Text())
				}
				c.WriteClipboard(strings.Join(lines, "\n"))
				widgets.ShowSnack(l10n.TranscriptCopied, "", nil)
			}}}
			if onAsk != nil {
				items = append(items,
					m3.MenuItem{Icon: icons.AutoAwesomeOutlined, Label: l10n.PanelAskNotes, OnTap: func() { onAsk(l10n.PromptNotes) }},
					m3.MenuItem{Icon: icons.QuizOutlined, Label: l10n.PanelQuizMore, OnTap: func() { ShowQuizSheet(ctl.LectureID(), onAsk) }},
					m3.MenuItem{Icon: icons.SummarizeOutlined, Label: l10n.PanelSummarizeRecent, OnTap: func() { onAsk(l10n.PromptSummarizeRecent) }},
				)
			}
			return items
		})
	})
	list := ui.List(c, &t.list, len(idx), func(k int) {
		i := idx[k]
		s := ctl.Segments[i]
		ui.Column(c).FillWidth().Children(func() {
			if head, ok := heads[i]; ok {
				transcriptChapterHeader(c, head, i == 0, func() { ctl.PanelTab = "outline" })
			}
			on := i == t.current
			row := m3.Tappable(c, design.CornerMD).FillWidth().Row().Padding(6, design.SpaceSM).Margin(0, 0, 2, 0)
			if on {
				row = row.Background(cs.SecondaryContainer)
			}
			row.Children(func() {
				tc := cs.Primary
				if on {
					tc = cs.OnSecondaryContainer
				}
				m3.Text(c, core.FmtClock(s.StartMs()), tt.LabelSmall.C(tc).H(1.9)).FontFeatures("tnum").Width(52).Shrink(0)
				ui.Column(c).Grow(1).Basis(0).Children(func() {
					fg := cs.OnSurface
					if on {
						fg = cs.OnSecondaryContainer
					}
					reviewedTranscriptText(c, s.Text(), q, tt.BodyMedium.H(1.55).C(fg), suggestions[s.Idx()], func(id string) {
						t.review.dismiss(ctl.LectureID(), id)
					})
					for _, it := range marks[i] {
						transcriptMarkChip(c, it)
					}
				})
			})
			if row.Clicked() {
				ctl.SeekTo(s.StartMs())
			}
		})
	}).FillWidth().Grow(1).Basis(0).Padding(design.SpaceSM, design.SpaceSM, design.SpaceXXL, design.SpaceSM).Selectable()
	userScroll(list, &t.scrolled)
}

// transcriptChapterHeader is a chapter title woven into the transcript.
func transcriptChapterHeader(c *ui.Context, ch core.Chapter, first bool, onTap func()) {
	cs, tt := design.S(), design.TT()
	top := float32(design.SpaceLG)
	if first {
		top = design.SpaceXS
	}
	ui.Box(c).FillWidth().Padding(top, design.SpaceSM, design.SpaceXS, design.SpaceSM).Children(func() {
		b := m3.Tappable(c, design.CornerSM).FillWidth().Row().AlignItems(ui.Center)
		b.Children(func() {
			m3.Text(c, fmt.Sprintf("%02d", ch.Idx()+1), tt.LabelMedium.C(cs.Primary).W(700)).FontFeatures("tnum")
			m3.HGap(c, design.SpaceSM)
			m3.Text(c, ch.Title(), tt.TitleSmall.C(cs.Primary)).MaxLines(1).Grow(1).Basis(0)
		})
		if b.Clicked() {
			onTap()
		}
	})
}

// transcriptMarkChip is a schedule item the teacher mentioned at this line.
func transcriptMarkChip(c *ui.Context, it core.ScheduleItem) {
	st, tt := KindStyleOf(it.Kind()), design.TT()
	b := m3.Tappable(c, design.CornerFull).Row().AlignItems(ui.Center).AlignSelf(ui.Start).Padding(3, 10, 3, 8).
		Margin(design.SpaceXS, 0, 2, 0).Background(st.Container)
	b.Children(func() {
		ic := st.Icon
		if it.Done() {
			ic = icons.CheckRounded
		}
		m3.Icon(c, ic, 14, st.OnContainer)
		m3.HGap(c, 4)
		m3.Text(c, st.Label+" · "+it.Title()+" · "+fmtDue(it), tt.LabelSmall.C(st.OnContainer)).MaxLines(1).Shrink(1)
	})
	if b.Clicked() {
		ShowScheduleEditor(&it, time.Time{})
	}
}

// transcriptReview is TranscriptReviewBar: the model's guesses at
// misheard words, which stay beside the original text.
type transcriptReview struct {
	data    core.J
	busy    bool
	visible bool
	err     string
	version int
	started bool
	poll    *time.Timer
}

func (r *transcriptReview) path(id string) string { return "/agent/transcript-review/" + core.Esc(id) }

func (r *transcriptReview) running() bool {
	return r.data.S("state") == "running" && !r.data.IsTrue("stale")
}

func (r *transcriptReview) stop() {
	if r.poll != nil {
		r.poll.Stop()
	}
}

func (r *transcriptReview) publish(id string, v core.J) {
	r.data, r.err = v, ""
	r.stop()
	if r.running() {
		r.poll = time.AfterFunc(3*time.Second, func() { core.Post(func() { r.refresh(id) }) })
	}
}

func (r *transcriptReview) refresh(id string) {
	if r.busy {
		return
	}
	r.version++
	v := r.version
	core.Async(func() (any, error) { return api().Call("GET", r.path(id), nil) }, func(res any, err error) {
		if err != nil {
			r.err = err.Error()
			return
		}
		if v == r.version {
			r.publish(id, core.Obj(res))
		}
	})
}

func (r *transcriptReview) action(id, method string) {
	r.stop()
	r.version++
	v := r.version
	r.busy = true
	core.Async(func() (any, error) { return api().Call(method, r.path(id), core.J{}) }, func(res any, err error) {
		r.busy = false
		if err != nil {
			r.err = err.Error()
			return
		}
		if v == r.version {
			r.publish(id, core.Obj(res))
		}
	})
}

func (r *transcriptReview) dismiss(id, suggestion string) {
	core.Async(func() (any, error) { return api().DismissTranscriptSuggestion(id, suggestion) }, func(res any, err error) {
		if err != nil {
			widgets.ShowSnack(l10n.ReviewLoadFailed(err.Error()), "", nil)
			return
		}
		r.data = core.Obj(res)
	})
}

func (r *transcriptReview) start(id string) {
	m3.ShowDialog(func(c *ui.Context, rt *m3.Route) {
		m3.AlertDialog(c, m3.Alert{Title: l10n.ReviewStart, Message: l10n.ReviewConsent, Actions: func() {
			if m3.TextButton(c, l10n.CommonCancel).Clicked() {
				rt.Close()
			}
			if m3.FilledButton(c, l10n.ReviewStart).Clicked() {
				rt.Pop(true)
			}
		}})
	}, func(v any) {
		if v == true {
			r.action(id, "POST")
		}
	})
}

func (r *transcriptReview) status() string {
	if r.data.IsTrue("stale") {
		return l10n.ReviewStale
	}
	n := 0
	for _, s := range r.data.Os("suggestions") {
		if !s.IsTrue("dismissed") {
			n++
		}
	}
	switch r.data.S("state") {
	case "running":
		return l10n.ReviewProgress(r.data.I("completed"), r.data.I("total"))
	case "completed":
		return l10n.ReviewCompleted(n)
	case "cancelled":
		return l10n.ReviewCancelled
	case "error":
		return l10n.ReviewFailed
	case "interrupted":
		return l10n.ReviewInterrupted
	}
	return l10n.ReviewHint
}

func (r *transcriptReview) show(c *ui.Context, id string) {
	cs, tt := design.S(), design.TT()
	if !r.started {
		r.started = true
		r.refresh(id)
	}
	running := r.running()
	ui.Column(c).FillWidth().Padding(design.SpaceSM).Children(func() {
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(design.SpaceSM).Children(func() {
			ic, label := icons.SpellcheckRounded, l10n.ReviewStart
			if running {
				ic, label = icons.StopCircleOutlined, l10n.ReviewCancel
			}
			b := m3.Button{Kind: m3.Tonal, Icon: ic, Label: label, Disabled: r.busy}
			if r.busy {
				b.Child = func(fg ui.Color) {
					ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
						m3.Spinner(c, 16, 2, fg)
						m3.Text(c, label, tt.LabelLarge.C(fg))
					})
				}
			}
			if b.Show(c).Clicked() {
				if running {
					r.action(id, "DELETE")
				} else {
					r.start(id)
				}
			}
			if (m3.Chip{Kind: m3.FilterChip, Label: l10n.ReviewShow, Selected: r.visible}).Show(c).Clicked() {
				r.visible = !r.visible
			}
			if m3.IconBtn(c, icons.RefreshRounded, l10n.CommonRetry).Clicked() {
				r.refresh(id)
			}
		})
		muted := tt.LabelSmall.C(cs.OnSurfaceVariant)
		m3.Text(c, r.status(), muted)
		if n := r.data.I("rejected"); n > 0 {
			m3.Text(c, l10n.ReviewFiltered(n), muted)
		}
		if m := r.data.S("model"); m != "" {
			m3.Text(c, l10n.ReviewModel(m), muted)
		}
		if running {
			var v *float64
			if t := r.data.F("total"); t > 0 {
				v = m3.Of(r.data.F("completed") / t)
			}
			m3.Linear(c, m3.Progress{Value: v})
		}
		if r.err != "" {
			m3.Text(c, l10n.ReviewLoadFailed(r.err), tt.BodyMedium.C(cs.Error))
		}
	})
}

// reviewedTranscriptText keeps the original words; only typed,
// exact-match sidecar guesses are inserted. Search highlighting applies to
// the original text on its own.
func reviewedTranscriptText(c *ui.Context, text, query string, style design.TextStyle, suggestions []core.J, onDismiss func(string)) {
	cs := design.S()
	var valid []core.J
	for _, a := range suggestions {
		o := a.S("original")
		if !a.IsTrue("dismissed") && o != "" && strings.Count(text, o) == 1 {
			valid = append(valid, a)
		}
	}
	sort.SliceStable(valid, func(i, j int) bool {
		return strings.Index(text, valid[i].S("original")) < strings.Index(text, valid[j].S("original"))
	})
	var spans []ui.Span
	original := func(part string) {
		if query == "" {
			spans = append(spans, style.Span(part))
			return
		}
		at := 0
		for {
			pos := strings.Index(part[at:], query)
			if pos < 0 {
				break
			}
			spans = append(spans, style.Span(part[at:at+pos]))
			m := style.C(cs.OnTertiaryContainer).Span(query)
			m.Background = cs.TertiaryContainer
			spans = append(spans, m)
			at += pos + len(query)
		}
		spans = append(spans, style.Span(part[at:]))
	}
	at := 0
	for _, a := range valid {
		end := strings.Index(text, a.S("original")) + len(a.S("original"))
		if end < at {
			continue
		}
		original(text[at:end])
		g := style.C(cs.Tertiary).W(600).Span(l10n.ReviewGuess(a.S("guess")))
		g.Italic = true
		g.Background = design.WithOpacity(cs.TertiaryContainer, .35)
		spans = append(spans, g)
		at = end
	}
	original(text[at:])
	ui.RichText(c, spans...).FillWidth()
	if len(valid) > 0 {
		ui.Row(c).FillWidth().Wrap().Gap(design.SpaceXS).Children(func() {
			for _, a := range valid {
				if (m3.Chip{Kind: m3.ActionChip, Avatar: icons.InfoOutlineRounded, Label: a.S("original") + " → " + a.S("guess"), Compact: true}).Show(c).Clicked() {
					reviewDetails(a, onDismiss)
				}
			}
		})
	}
}

func reviewDetails(a core.J, onDismiss func(string)) {
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		m3.AlertDialog(c, m3.Alert{Title: l10n.ReviewGuess(a.S("guess")), Content: func() {
			ui.Scroll(c).FillWidth().MaxHeight(360).Children(func() {
				m3.Text(c, l10n.ReviewDetails(a.S("original"), a.S("reason"), a.S("evidence")), design.TT().BodyMedium).Selectable()
			})
		}, Actions: func() {
			if m3.TextButton(c, l10n.CommonClose).Clicked() {
				r.Close()
			}
			if m3.TextButton(c, l10n.ReviewDismiss).Clicked() {
				r.Pop(true)
			}
		}})
	}, func(v any) {
		if v == true {
			onDismiss(a.S("id"))
		}
	})
}

// ---------------------------------------------------------------- materials

// materialsTab holds platform slides and uploaded courseware under one tab.
type materialsTab struct {
	picked *bool
	slides slidesTab
}

func (m *materialsTab) show(c *ui.Context, ctl *LecturePlayer) {
	slides := len(ctl.Slides) > 0 || len(ctl.Courseware) == 0
	if m.picked != nil {
		slides = *m.picked
	}
	ui.Box(c).FillWidth().Padding(design.SpaceXS, design.SpaceMD, 0, design.SpaceMD).Children(func() {
		sl, cw := l10n.AssetSlides, l10n.AssetCourseware
		if len(ctl.Slides) > 0 {
			sl = l10n.PanelSlidesCount(len(ctl.Slides))
		}
		if len(ctl.Courseware) > 0 {
			cw = l10n.PanelCoursewareCount(len(ctl.Courseware))
		}
		v := slides
		if m3.Segmented(c, []m3.Segment[bool]{{Value: true, Icon: icons.SlideshowOutlined, Label: sl}, {Value: false, Icon: icons.DescriptionOutlined, Label: cw}}, &v,
			m3.SegOpts{NoCheck: true, Fill: true, Compact: true}) {
			m.picked = &v
		}
	})
	ui.Column(c).FillWidth().Grow(1).Basis(0).Key(slides).
		Transition(design.Switcher(design.Medium2, design.EmphasizedDecelerate, nil, ui.Motion{Opacity: 0})).Children(func() {
		if slides {
			m.slides.show(c, ctl)
		} else {
			ui.Scroll(c).Fill().Children(func() {
				CoursewareList(c, ctl.LectureID(), ctl.Courseware, ctl.ReloadCourseware)
			})
		}
	})
}

type slidesTab struct {
	list     ui.ListState
	keyOnly  bool
	current  int
	scrolled time.Time
}

func (s *slidesTab) items(ctl *LecturePlayer) []core.Slide {
	if !s.keyOnly {
		return ctl.Slides
	}
	var out []core.Slide
	for _, sl := range ctl.Slides {
		if sl.IsKey() {
			out = append(out, sl)
		}
	}
	if len(out) == 0 {
		return ctl.Slides
	}
	return out
}

func (s *slidesTab) show(c *ui.Context, ctl *LecturePlayer) {
	cs, tt := design.S(), design.TT()
	if len(ctl.Slides) == 0 {
		widgets.EmptyState{Icon: icons.SlideshowOutlined, Title: l10n.SlidesEmpty, Body: l10n.SlidesEmptyBody}.Show(c)
		return
	}
	list := s.items(ctl)
	pos := ctl.PositionMs()
	i := -1
	for k, sl := range list {
		if sl.TSec()*1000 > pos {
			break
		}
		i = k
	}
	if i != s.current {
		s.current = i
		if core.UI.SlideFollow && i >= 0 && time.Since(s.scrolled) > 4*time.Second {
			s.list.ScrollTo(i, ui.Center)
		}
	}
	hasKey, hasPath := false, false
	for _, sl := range ctl.Slides {
		hasKey = hasKey || sl.IsKey()
		hasPath = hasPath || sl.Path() != ""
	}
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(design.SpaceSM).Padding(design.SpaceSM, design.SpaceMD, 0, design.SpaceMD).Children(func() {
		// Without page-change marks there is nothing to filter.
		if hasKey && m3.FilterChipOf(c, l10n.SlidesPageOnly, s.keyOnly) {
			s.keyOnly = !s.keyOnly
		}
		if m3.FilterChipOf(c, l10n.SlidesFollow, core.UI.SlideFollow) {
			core.UI.Update(func(p *design.Prefs) { p.SlideFollow = !p.SlideFollow })
		}
		ui.Box(c).Grow(1)
		if !hasPath && m3.IconBtn(c, icons.DownloadOutlined, l10n.SlidesDownload).Clicked() {
			title := ""
			if ctl.Lecture != nil {
				title = ctl.Lecture.ShortTitle()
			}
			ShowDownloadSheet([]string{ctl.LectureID()}, title, nil, nil)
		}
	})
	l := ui.List(c, &s.list, len(list), func(k int) {
		sl := list[k]
		on := k == s.current
		b := m3.Tappable(c, design.CornerLG).FillWidth().Column().Margin(0, 0, design.SpaceMD, 0)
		b.Children(func() {
			frame := ui.Box(c).FillWidth().Radius(design.CornerLG)
			pad := frame.Animate("pad", map[bool]float32{true: 3, false: 0}[on], design.Medium2)
			frame = frame.Padding(pad)
			if on {
				frame = frame.Background(cs.Primary)
			}
			frame.Children(func() {
				r := float32(design.CornerLG)
				if on {
					r = design.CornerMD
				}
				ui.Box(c).FillWidth().AspectRatio(16.0 / 9).Radius(r).Clip().Background(cs.SurfaceContainerHighest).Center().Children(func() {
					widgets.NetImage(c, ctl.SlideURL(sl), nil)
				})
			})
			m3.VGap(c, 6)
			ui.Row(c).FillWidth().Children(func() {
				lc := cs.OnSurfaceVariant
				if on {
					lc = cs.Primary
				}
				m3.Text(c, l10n.SlidesIndex(sl.Idx()), tt.LabelMedium.C(lc)).Grow(1)
				m3.Text(c, core.FmtClock(sl.TSec()*1000), tt.LabelMedium.C(cs.OnSurfaceVariant)).FontFeatures("tnum")
			})
		})
		if b.DoubleClicked() {
			showSlideViewer(ctl, list, k)
		} else if b.Clicked() {
			ctl.SeekTo(sl.TSec() * 1000)
		}
	}).FillWidth().Grow(1).Basis(0).Padding(design.SpaceMD)
	userScroll(l, &s.scrolled)
}

// showSlideViewer pages through the slides full screen.
func showSlideViewer(ctl *LecturePlayer, list []core.Slide, start int) {
	cur := start
	v := &imageView{scale: 1}
	m3.ShowGeneral(true, func(c *ui.Context, r *m3.Route) {
		step := func(d int) {
			if n := min(max(cur+d, 0), len(list)-1); n != cur {
				cur, v.scale, v.x, v.y = n, 1, 0, 0
			}
		}
		if c.Shortcut(0, ui.KeyRight) {
			step(1)
		}
		if c.Shortcut(0, ui.KeyLeft) {
			step(-1)
		}
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Background(ui.RGBA(0, 0, 0, .87))
		area := ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Center().Clip()
		area.HandleInput(func(ev ui.InputEvent) bool {
			switch {
			case ev.Kind == ui.InputZoom:
				v.scale = min(max(v.scale*ev.Scale, 1), 5)
			case ev.Kind == ui.InputScroll && ev.Mods&ui.Ctrl != 0:
				if ev.DY < 0 {
					v.scale = min(v.scale*1.25, 5)
				} else if ev.DY > 0 {
					v.scale = max(v.scale*.8, 1)
				}
			default:
				return false
			}
			if v.scale == 1 {
				v.x, v.y = 0, 0
			}
			return true
		})
		area.Children(func() {
			bmp, _ := widgets.NetBitmap(ctl.SlideURL(list[cur]))
			if bmp == nil {
				m3.Spinner(c, 32, 3, pWhite)
				return
			}
			img := ui.Image(c, bmp).Key(cur).Fit(ui.Contain).MaxWidthPercent(100).MaxHeightPercent(100).
				Transition(ui.ElementTransition{Duration: design.Medium2, Ease: design.Emphasized, Enter: &ui.Motion{Opacity: 0}, Shadows: true})
			if dx, dy, ok := img.Dragged(); ok && v.scale > 1 {
				v.x, v.y = v.x+dx, v.y+dy
			}
			img.Scale(v.scale, v.scale).Translate(v.x, v.y)
		})
		ui.Box(c).Absolute().Top(design.SpaceLG).Right(design.SpaceLG).Children(func() {
			if (m3.IconButton{Kind: m3.TonalIcon, Icon: icons.CloseRounded, Tooltip: l10n.CommonClose}).Show(c).Clicked() {
				r.Close()
			}
		})
		ui.Row(c).Absolute().Bottom(design.SpaceXL).Left(0).Right(0).Justify(ui.Center).AlignItems(ui.Center).Gap(design.SpaceMD).Children(func() {
			if (m3.IconButton{Kind: m3.TonalIcon, Icon: icons.ChevronLeftRounded}).Show(c).Clicked() {
				step(-1)
			}
			if (m3.Button{Kind: m3.Filled, Icon: icons.PlayArrowRounded, Label: l10n.SlidesPosition(cur+1, len(list), core.FmtClock(list[cur].TSec()*1000))}).Show(c).Clicked() {
				ctl.SeekTo(list[cur].TSec() * 1000)
				r.Close()
			}
			if (m3.IconButton{Kind: m3.TonalIcon, Icon: icons.ChevronRightRounded}).Show(c).Clicked() {
				step(1)
			}
		})
	}, nil)
}

// ---------------------------------------------------------------- notes

type notesTab struct {
	editing *core.Note
	isNew   bool
	write   bool
	reader  *NoteInlineReader
	editor  *NoteEditor
	quizzes *QuizList
	opening string
}

func (n *notesTab) close() {
	if n.reader != nil {
		n.reader.Dispose()
		n.reader = nil
	}
	if n.editor != nil {
		n.editor.Dispose()
		n.editor = nil
	}
}

func (n *notesTab) seek(ctl *LecturePlayer, t CoursiveRef) {
	if t.LectureID == "" || t.LectureID == ctl.LectureID() {
		ctl.SeekTo(t.Seconds * 1000)
	} else {
		SeekOrOpen(t.LectureID, t.Seconds)
	}
}

func (n *notesTab) show(c *ui.Context, ctl *LecturePlayer, onAsk func(string)) {
	cs, tt := design.S(), design.TT()
	if n.editing != nil && !n.write {
		if n.reader == nil {
			n.reader = NewNoteInlineReader(*n.editing)
			n.reader.OnSeek = func(t CoursiveRef) { n.seek(ctl, t) }
			n.reader.OnBack = func() { n.close(); n.editing = nil }
			n.reader.OnEdit = func() { n.write = true }
		}
		n.reader.Show(c)
		return
	}
	if n.editing != nil || n.isNew {
		if n.editor == nil {
			courseID := ""
			if ctl.Lecture != nil {
				courseID = ctl.Lecture.CourseID()
			}
			n.editor = NewNoteEditor(n.editing, NoteEditorOptions{CourseID: courseID, LectureID: ctl.LectureID(), Timestamp: ctl.PositionMs,
				OnSeek: func(t CoursiveRef) { n.seek(ctl, t) }, Edit: true,
				OnSaved: func(saved core.Note) {
					ctl.ReloadNotes()
					if n.editing != nil {
						n.editing = &saved
						if n.reader != nil {
							n.reader.Set(saved)
						}
					}
				},
				OnClose: func() {
					if n.editing == nil {
						n.isNew = false
					}
					n.write = false
					if n.editor != nil {
						n.editor.Dispose()
						n.editor = nil
					}
				}})
		}
		ui.Column(c).Fill().Padding(design.SpaceMD).Children(func() { n.editor.Show(c) })
		return
	}
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).FillWidth().Padding(design.SpaceMD).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				m3.Text(c, l10n.CommonBookmarks, tt.TitleSmall).Grow(1)
				if (m3.Button{Kind: m3.TextKind, Icon: icons.BookmarkAddOutlined, Label: l10n.PanelCurrentPosition}).Show(c).Clicked() {
					ctl.AddBookmark("", nil)
				}
			})
			if len(ctl.Bookmarks) == 0 {
				m3.Text(c, l10n.BookmarksHint, tt.BodySmall.C(cs.OnSurfaceVariant)).Padding(design.SpaceSM, 0)
			}
			for _, b := range ctl.Bookmarks {
				ui.Box(c.Key(b.ID())).FillWidth().Children(func() {
					title := b.Label()
					if title == "" {
						title = l10n.CommonBookmarks
						if i := ctl.SegmentAt(b.TMs()); i >= 0 {
							title = ctl.Segments[i].Text()
						}
					}
					t := m3.Tile{Leading: func() {
						if m3.ActionChipOf(c, 0, core.FmtClock(b.TMs())) {
							ctl.SeekTo(b.TMs())
						}
					}, TitleFn: func() {
						m3.Text(c, title, tt.BodyMedium).MaxLines(2).Ellipsis("…")
					}, Trailing: func() {
						if (m3.IconButton{Icon: icons.CloseRounded, Size: 18}).Show(c).Clicked() {
							ctl.DeleteBookmark(b)
						}
					}, Tap: true, Pad: true, PadX: design.SpaceSM}.Show(c)
					if t.Clicked() {
						ctl.SeekTo(b.TMs())
					}
				})
			}
			m3.VGap(c, design.SpaceLG)
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				m3.Text(c, l10n.CommonNotes, tt.TitleSmall).Grow(1)
				if (m3.Button{Kind: m3.Tonal, Icon: icons.AddRounded, Label: l10n.CommonNew}).Show(c).Clicked() {
					n.isNew = true
				}
			})
			m3.VGap(c, design.SpaceSM)
			if len(ctl.Notes) == 0 {
				m3.Text(c, l10n.NotesHint, tt.BodySmall.C(cs.OnSurfaceVariant))
			}
			for _, note := range ctl.Notes {
				ic := icons.DescriptionOutlined
				if note.Source() == "agent" {
					ic = icons.AutoAwesomeOutlined
				}
				card := m3.CardColor(c.Key(note.ID()), cs.SurfaceContainerHigh).FillWidth().Margin(4, 0)
				card.Children(func() {
					t := m3.Tile{LeadingIcon: ic, Tap: true, Title: note.Title(), Subtitle: core.FmtRelative(time.Unix(int64(note.UpdatedAt()), 0))}.Show(c)
					if t.Clicked() {
						n.open(note.ID())
					}
				})
			}
			m3.VGap(c, design.SpaceLG)
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				m3.Text(c, l10n.CommonPractice, tt.TitleSmall).Grow(1)
				if onAsk != nil && (m3.Button{Kind: m3.Tonal, Icon: icons.AutoAwesomeOutlined, Label: l10n.PanelQuiz}).Show(c).Clicked() {
					ShowQuizSheet(ctl.LectureID(), onAsk)
				}
			})
			m3.VGap(c, design.SpaceSM)
			n.quizzes.Show(c, 1, nil, func() {
				m3.Text(c, l10n.PanelQuizHint, tt.BodySmall.C(cs.OnSurfaceVariant))
			})
		})
	})
}

func (n *notesTab) open(id string) {
	n.opening = id
	core.Async(func() (any, error) { return api().Get("/api/notes/" + core.Esc(id)) }, func(r any, err error) {
		if err != nil || n.opening != id {
			return
		}
		note := core.Note{J: core.Obj(r)}
		n.close()
		n.editing, n.write = &note, false
	})
}
