package app

import (
	"fmt"
	"math"
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

// tasks_page.dart: scheduled assistant tasks
// (docs/specs/0003-scheduled-agent-tasks.md).

// IsWorkdays tells whether a weekly schedule runs Monday to Friday only.
func IsWorkdays(s core.J) bool {
	days := map[int]bool{}
	for _, d := range s.L("days") {
		days[core.Int(d)] = true
	}
	if s.S("type") != "weekly" || len(days) != 5 {
		return false
	}
	for d := range days {
		if d > 5 {
			return false
		}
	}
	return true
}

func hm(t time.Time) string { return fmt.Sprintf("%02d:%02d", t.Hour(), t.Minute()) }

// FmtSchedule is fmtSchedule: 每天 21:30, 工作日 08:00, 每周一、五 08:00,
// 10月15日 07:00.
func FmtSchedule(s core.J) string {
	switch s.S("type") {
	case "daily":
		return l10n.ScheduleDaily(s.S("time"))
	case "weekly":
		days := s.L("days")
		if len(days) == 7 {
			return l10n.ScheduleDaily(s.S("time"))
		}
		if IsWorkdays(s) {
			return l10n.ScheduleWorkdays(s.S("time"))
		}
		var names []string
		for _, d := range days {
			if i := core.Int(d); i >= 1 && i <= 7 {
				names = append(names, core.Weekdays[i-1])
			}
		}
		return l10n.ScheduleWeekly(strings.Join(names, l10n.ListSeparator), s.S("time"))
	case "once":
		at, err := parseLocal(s.S("at"))
		if err != nil {
			return l10n.ScheduleOnce
		}
		return l10n.ScheduleOnceAt(int(at.Month()), at.Day(), hm(at))
	}
	return ""
}

// parseLocal reads an ISO time, local when it has no zone (DateTime.parse).
func parseLocal(s string) (time.Time, error) {
	for _, f := range []string{time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02"} {
		if t, err := time.ParseInLocation(f, s, time.Local); err == nil {
			return t.Local(), nil
		}
	}
	return time.Time{}, fmt.Errorf("bad time %q", s)
}

// FmtWhen is fmtWhen: 今天 21:30, 明天 08:00, 周五 08:00, 10月20日 08:00.
func FmtWhen(t, now time.Time) string {
	days := core.DaysBetween(now, t)
	var day string
	switch {
	case days == 0:
		day = l10n.CommonToday
	case days == 1:
		day = l10n.CommonTomorrow
	case days == -1:
		day = l10n.CommonYesterday
	case days > 1 && days < 7:
		day = l10n.WeekdayName(core.Weekdays[core.Weekday(t)-1])
	default:
		day = l10n.DateMonthDay(int(t.Month()), t.Day())
	}
	return day + " " + hm(t)
}

// runStatus is a run's status label and icon, and whether it went wrong.
func runStatus(s string) (string, icons.Icon, bool) {
	switch s {
	case "ok":
		return l10n.CommonFinished, icons.CheckCircleOutlineRounded, false
	case "error":
		return l10n.CommonFailed, icons.ErrorOutlineRounded, true
	case "budget":
		return l10n.RunCostLimit, icons.MoneyOffRounded, true
	case "aborted":
		return l10n.RunAborted, icons.StopCircleOutlined, false
	case "skipped":
		return l10n.RunMissed, icons.SkipNextOutlined, false
	}
	return s, icons.CircleOutlined, false
}

// openRunConversation opens the conversation of a run over the current
// screen.
func openRunConversation(session string) {
	a := Assistant
	if a.Busy() {
		widgets.ShowSnack(l10n.TasksBusy, "", nil)
		return
	}
	a.OpenSession(session)
	ShowAssistantSheet()
}

func msTime(v any) (time.Time, bool) {
	if f, ok := v.(float64); ok {
		return time.UnixMilli(int64(f)), true
	}
	return time.Time{}, false
}

func runDayLabel(t, now time.Time) string {
	switch core.DaysBetween(t, now) {
	case 0:
		return l10n.CommonToday
	case 1:
		return l10n.CommonYesterday
	}
	return l10n.DateMonthDayWeek(int(t.Month()), t.Day(), core.Weekdays[core.Weekday(t)-1])
}

// TaskWrite is what a run may change (the agent's TASK_WRITES); other
// writes are refused.
type TaskWrite struct {
	Key, Label string
	Icon       icons.Icon
}

var TaskWrites = []TaskWrite{
	{"notes", l10n.TasksWriteNotes, icons.EditNoteRounded},
	{"schedule", l10n.TasksWriteSchedule, icons.EventAvailableOutlined},
	{"quiz", l10n.TasksWriteQuizzes, icons.QuizOutlined},
}

func taskWriteLabel(k string) string {
	for _, w := range TaskWrites {
		if w.Key == k {
			return w.Label
		}
	}
	return k
}

// TaskTemplate is a ready-made task offered when the list is empty.
type TaskTemplate struct {
	Name   string
	Icon   icons.Icon
	Prompt string
	Sched  core.J
	Writes []string
}

func (t TaskTemplate) JSON() core.J {
	w := make([]any, len(t.Writes))
	for i, x := range t.Writes {
		w[i] = x
	}
	return core.J{"name": t.Name, "prompt": t.Prompt, "schedule": map[string]any(t.Sched), "writes": w}
}

var TaskTemplates = []TaskTemplate{
	{l10n.TaskTemplateReviewName, icons.NightlightOutlined, l10n.TaskTemplateReviewPrompt, core.J{"type": "daily", "time": "21:30"}, []string{"notes"}},
	{l10n.TaskTemplateQuizName, icons.QuizOutlined, l10n.TaskTemplateQuizPrompt, core.J{"type": "weekly", "days": []any{float64(1)}, "time": "08:00"}, []string{"quiz"}},
	{l10n.TaskTemplateDeadlinesName, icons.EventNoteOutlined, l10n.TaskTemplateDeadlinesPrompt, core.J{"type": "daily", "time": "08:00"}, nil},
}

// ---------------------------------------------------------------- page

// TasksPage is the 定时 tab: tasks beside the selected task's detail on
// wide windows, a list with pushed detail pages otherwise, and runs of
// every task.
type TasksPage struct {
	tasks    []core.J
	loaded   bool
	runs     []core.J
	err      error
	view     int // 0 tasks, 1 run history
	selected string
	filter   string
	cancel   func()
	scroll   ui.ScrollState
	side     ui.ScrollState
	detail   *TaskDetail
}

// NewTasksPage loads the tasks and follows their runs.
func NewTasksPage() m3.Page {
	p := &TasksPage{filter: "all"}
	p.load()
	p.cancel = State.Events.Listen(func(e core.J) {
		if e.S("type") == "agent_task" {
			p.load()
		}
	})
	return p
}

// MainScroll is the page's scroll.
func (p *TasksPage) MainScroll() *ui.ScrollState { return &p.scroll }

func (p *TasksPage) Dispose() {
	p.cancel()
	if p.detail != nil {
		p.detail.Dispose()
	}
}

func (p *TasksPage) load() {
	core.Async(func() ([2][]core.J, error) {
		t, err := api().Get("/agent/tasks")
		if err != nil {
			return [2][]core.J{}, err
		}
		r, err := api().Get("/agent/tasks/runs", map[string]string{"limit": "200"})
		return [2][]core.J{core.Objs(t), core.Objs(r)}, err
	}, func(v [2][]core.J, err error) {
		if err != nil {
			p.err = err
			return
		}
		p.tasks, p.runs, p.err, p.loaded = v[0], v[1], nil, true
		found := false
		for _, t := range p.tasks {
			found = found || t.S("id") == p.selected
		}
		if !found {
			p.selected = ""
		}
	})
}

func (p *TasksPage) edit(template core.J) {
	ShowTaskEditor(nil, template, func(saved core.J) {
		p.view, p.selected = 0, saved.S("id")
		p.load()
	})
}

func (p *TasksPage) toggle(t core.J, on bool) {
	core.Go(func() error { _, err := api().Put("/agent/tasks/"+t.S("id"), core.J{"enabled": on}); return err }, func(err error) {
		if err != nil {
			widgets.ShowSnack(err.Error(), "", nil)
		}
		p.load()
	})
}

func (p *TasksPage) open(t core.J) {
	m3.Nav().PushThen(newTaskDetailPage(t.S("id")), p.load)
}

// View builds the page.
func (p *TasksPage) View(c *ui.Context) {
	now := time.Now()
	w, _ := c.Size()
	compact := w < 600
	pad := float32(design.SpaceXL)
	if compact {
		pad = design.SpaceLG
	}
	ui.Column(c).Fill().Background(design.S().Surface).Children(func() {
		p.header(c, now, pad, compact)
		ui.Column(c).FillWidth().Grow(1).Basis(0).Children(func() {
			switch {
			case !p.loaded && p.err == nil:
				widgets.CenteredLoader(c, "")
			case !p.loaded:
				widgets.ErrorState(c, p.err, p.load)
			case p.view == 1:
				p.history(c, now, pad)
			case len(p.tasks) == 0:
				p.gallery(c, pad)
			case w >= 880:
				p.split(c, now, pad, w)
			default:
				ui.Scroll(c).Fill().TrackScroll(&p.scroll).Children(func() {
					ui.Column(c).FillWidth().Padding(0, pad, 120+BottomInset, pad).Children(func() {
						widgets.ContentWidth(c, 760, func() {
							for _, t := range p.tasks {
								t := t
								ui.Box(c.Key(t.S("id"))).FillWidth().Margin(0, 0, design.SpaceSM, 0).Children(func() {
									taskRow(c, t, now, false, func() { p.open(t) }, func(v bool) { p.toggle(t, v) })
								})
							}
						})
					})
				})
			}
		})
	})
}

func (p *TasksPage) header(c *ui.Context, now time.Time, pad float32, compact bool) {
	cs := design.S()
	tt := design.TT()
	sub := l10n.TasksSubtitle
	if len(p.tasks) > 0 {
		var on, upcoming []core.J
		for _, t := range p.tasks {
			if t.IsTrue("enabled") {
				on = append(on, t)
				if _, ok := t["next_run"].(float64); ok {
					upcoming = append(upcoming, t)
				}
			}
		}
		sort.SliceStable(upcoming, func(i, j int) bool { return upcoming[i].F("next_run") < upcoming[j].F("next_run") })
		parts := []string{l10n.TasksCount(len(p.tasks))}
		if len(on) == len(p.tasks) {
			parts = append(parts, l10n.CommonAllEnabled)
		} else {
			parts = append(parts, l10n.TasksEnabledCount(len(on)))
		}
		if len(upcoming) > 0 {
			at, _ := msTime(upcoming[0]["next_run"])
			parts = append(parts, l10n.TasksNextUp(upcoming[0].S("name"), FmtWhen(at, now)))
		}
		sub = strings.Join(parts, " · ")
	}
	top := float32(design.SpaceXXL)
	title := tt.DisplaySmall
	if compact {
		top, title = design.SpaceXL, tt.HeadlineMedium
	}
	widgets.ContentWidth(c, design.MaxContentWidth+200, func() {
		ui.Column(c).FillWidth().Padding(top, pad, design.SpaceLG, pad).Children(func() {
			m3.Text(c, l10n.CommonScheduledTasks, title)
			m3.VGap(c, design.SpaceXS)
			m3.Text(c, sub, tt.BodyLarge.C(cs.OnSurfaceVariant))
			m3.VGap(c, design.SpaceLG)
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				label := l10n.CommonTasks
				if len(p.tasks) > 0 {
					label = l10n.CommonTasksCount(len(p.tasks))
				}
				m3.Segmented(c, []m3.Segment[int]{{Value: 0, Icon: icons.ScheduleRounded, Label: label}, {Value: 1, Icon: icons.HistoryRounded, Label: l10n.TasksRuns}},
					&p.view, m3.SegOpts{NoCheck: true})
				ui.Box(c).Grow(1)
				if compact {
					if (m3.IconButton{Kind: m3.FilledIcon, Icon: icons.AddRounded, Tooltip: l10n.TasksNew}).Show(c).Clicked() {
						p.edit(nil)
					}
				} else if (m3.Button{Kind: m3.Filled, Icon: icons.AddRounded, Label: l10n.TasksNew}).Show(c).Clicked() {
					p.edit(nil)
				}
			})
		})
	})
}

func (p *TasksPage) split(c *ui.Context, now time.Time, pad, width float32) {
	cs := design.S()
	sel := p.tasks[0]
	for _, t := range p.tasks {
		if t.S("id") == p.selected {
			sel = t
		}
	}
	if p.detail == nil || p.detail.id != sel.S("id") {
		if p.detail != nil {
			p.detail.Dispose()
		}
		p.detail = NewTaskDetail(sel.S("id"), true)
		p.detail.OnChanged = p.load
		p.detail.OnDeleted = func() { p.selected = "" }
	}
	listW := float32(320)
	if width >= 1200 {
		listW = 380
	}
	ui.Column(c).Fill().AlignItems(ui.Center).Children(func() {
		ui.Row(c).FillWidth().MaxWidth(design.MaxContentWidth+200).Grow(1).Basis(0).AlignItems(ui.Stretch).Padding(0, pad, pad, pad-design.SpaceSM).Children(func() {
			ui.Scroll(c).Width(listW).FillHeight().TrackScroll(&p.side).Children(func() {
				ui.Column(c).FillWidth().Padding(0, 0, design.SpaceXL, 0).Children(func() {
					for _, t := range p.tasks {
						t := t
						ui.Box(c.Key(t.S("id"))).FillWidth().Margin(0, 0, design.SpaceXS, 0).Children(func() {
							taskRow(c, t, now, t.S("id") == sel.S("id"), func() { p.selected = t.S("id") }, nil)
						})
					}
				})
			})
			m3.HGap(c, design.SpaceMD)
			ui.Column(c).Grow(1).Basis(0).FillHeight().Background(cs.SurfaceContainerLow).Radius(design.CornerXL).Clip().Children(func() {
				p.detail.Show(c)
			})
		})
	})
}

func (p *TasksPage) history(c *ui.Context, now time.Time, pad float32) {
	cs := design.S()
	tt := design.TT()
	var runs []core.J
	for _, r := range p.runs {
		st := r.S("status")
		ok := true
		switch p.filter {
		case "ok":
			ok = st == "ok"
		case "failed":
			ok = st == "error" || st == "budget"
		case "skipped":
			ok = st == "skipped"
		}
		if ok {
			runs = append(runs, r)
		}
	}
	var order []string
	days := map[string][]core.J{}
	for _, r := range runs {
		at, ok := msTime(r["started"])
		if !ok {
			at = now
		}
		d := runDayLabel(at, now)
		if _, ok := days[d]; !ok {
			order = append(order, d)
		}
		days[d] = append(days[d], r)
	}
	lastVisit := Inbox.LastVisit
	ui.Scroll(c).Fill().TrackScroll(&p.scroll).Children(func() {
		ui.Column(c).FillWidth().Padding(0, pad, 120+BottomInset, pad).Children(func() {
			widgets.ContentWidth(c, 840, func() {
				ui.Row(c).FillWidth().Wrap().Gap(design.SpaceSM).Children(func() {
					for _, f := range [][2]string{{"all", l10n.CommonAll}, {"ok", l10n.CommonFinished}, {"failed", l10n.CommonFailed}, {"skipped", l10n.RunMissed}} {
						if m3.ChoiceChipOf(c.Key(f[0]), f[1], p.filter == f[0]) {
							p.filter = f[0]
						}
					}
				})
				if len(order) == 0 {
					msg := l10n.TasksRunsNoMatch
					if len(p.runs) == 0 {
						msg = l10n.TasksRunsEmpty
					}
					m3.Text(c, msg, tt.BodyMedium.C(cs.OnSurfaceVariant)).TextAlign(ui.Center).FillWidth().PaddingY(design.SpaceXXL)
				}
				for _, d := range order {
					m3.Text(c.Key("d-"+d), d, tt.TitleSmall.C(cs.OnSurfaceVariant)).Padding(design.SpaceXL, 0, design.SpaceSM, design.SpaceXS)
					ui.Column(c.Key("dl-" + d)).FillWidth().Background(cs.SurfaceContainerLow).Radius(design.CornerXL).Clip().Children(func() {
						for i, r := range days[d] {
							runTileView(c.Key(i), r, now, true, r.F("finished") > float64(lastVisit))
						}
					})
				}
			})
		})
	})
}

func (p *TasksPage) gallery(c *ui.Context, pad float32) {
	cs := design.S()
	tt := design.TT()
	ui.Scroll(c).Fill().TrackScroll(&p.scroll).Children(func() {
		ui.Column(c).FillWidth().Padding(design.SpaceSM, pad, design.SpaceXXXL, pad).Children(func() {
			widgets.ContentWidth(c, 900, func() {
				m3.Text(c, l10n.TasksIntroTitle, tt.TitleLarge)
				m3.VGap(c, design.SpaceXS)
				m3.Text(c, l10n.TasksIntroBody, tt.BodyMedium.C(cs.OnSurfaceVariant))
				m3.VGap(c, design.SpaceXL)
				box := ui.Row(c).FillWidth().Wrap().Gap(design.SpaceMD)
				bw := box.Bounds().W
				cols := 1
				switch {
				case bw >= 720:
					cols = 3
				case bw >= 460:
					cols = 2
				}
				w := (bw - design.SpaceMD*float32(cols-1)) / float32(cols)
				box.Children(func() {
					for i, t := range TaskTemplates {
						t := t
						card := m3.Tappable(c.Key(i), design.CornerXL).Background(cs.SurfaceContainerLow).Padding(design.SpaceLG)
						if w > 0 {
							card = card.Width(w)
						}
						card.Children(func() {
							ui.Column(c).FillWidth().Children(func() {
								ui.Box(c).Size(40, 40).Center().Background(cs.TertiaryContainer).Radius(13).Children(func() {
									m3.Icon(c, t.Icon, 20, cs.OnTertiaryContainer)
								})
								m3.VGap(c, design.SpaceMD)
								m3.Text(c, t.Name, tt.TitleMedium)
								m3.VGap(c, design.SpaceXXS)
								m3.Text(c, FmtSchedule(t.Sched), tt.LabelLarge.C(cs.Primary))
								m3.VGap(c, design.SpaceSM)
								m3.Clamp(m3.Text(c, t.Prompt, tt.BodySmall.C(cs.OnSurfaceVariant).H(1.5)), 3)
							})
						})
						if card.Clicked() {
							p.edit(t.JSON())
						}
					}
				})
				m3.VGap(c, design.SpaceLG)
				ui.Row(c).Children(func() {
					if (m3.Button{Kind: m3.Outlined, Icon: icons.EditNoteRounded, Label: l10n.TasksFromBlank}).Show(c).Clicked() {
						p.edit(nil)
					}
				})
			})
		})
	})
}

// taskGlyph is a tonal rounded tile whose icon hints at when the task
// runs.
func taskGlyph(c *ui.Context, task core.J, size float32) {
	cs := design.S()
	s := task.O("schedule")
	hour := 12
	if h, err := strconv.Atoi(strings.SplitN(s.S("time"), ":", 2)[0]); err == nil {
		hour = h
	}
	ic := icons.LightModeOutlined
	switch {
	case s.S("type") == "once":
		ic = icons.AlarmRounded
	case s.S("type") == "weekly" && IsWorkdays(s):
		ic = icons.WorkOutlineRounded
	case s.S("type") == "weekly":
		ic = icons.DateRangeRounded
	case hour >= 18 || hour < 5:
		ic = icons.NightlightOutlined
	case hour < 11:
		ic = icons.WbSunnyOutlined
	}
	// A stable tone per task, so neighbours differ.
	key := task.S("id")
	if key == "" {
		key = task.S("name")
	}
	tone := 0
	for _, u := range utf16Units(key) {
		tone += int(u)
	}
	tone %= 3
	bg, fg := cs.TertiaryContainer, cs.OnTertiaryContainer
	switch {
	case task["enabled"] == false:
		bg, fg = cs.SurfaceContainerHighest, cs.OnSurfaceVariant
	case tone == 0:
		bg, fg = cs.PrimaryContainer, cs.OnPrimaryContainer
	case tone == 1:
		bg, fg = cs.SecondaryContainer, cs.OnSecondaryContainer
	}
	ui.Box(c).Size(size, size).Shrink(0).Center().Background(bg).Radius(size * .32).Children(func() { m3.Icon(c, ic, size*.5, fg) })
}

// utf16Units are a string's UTF-16 code units, as Dart's codeUnits.
func utf16Units(s string) []uint16 {
	var out []uint16
	for _, r := range s {
		if r >= 0x10000 {
			r -= 0x10000
			out = append(out, uint16(0xD800+(r>>10)), uint16(0xDC00+(r&0x3FF)))
		} else {
			out = append(out, uint16(r))
		}
	}
	return out
}

// statePill is 运行中 / 排队中 / 已启用 / 已暂停 as a small pill.
func statePill(c *ui.Context, task core.J) {
	cs := design.S()
	label, bg, fg := l10n.CommonEnabled, cs.PrimaryContainer, cs.OnPrimaryContainer
	switch {
	case task.S("state") == "running":
		label, bg, fg = l10n.CommonRunning, cs.Primary, cs.OnPrimary
	case task.S("state") == "queued":
		label, bg, fg = l10n.CommonQueued, cs.SecondaryContainer, cs.OnSecondaryContainer
	case !task.IsTrue("enabled"):
		label, bg, fg = l10n.CommonPaused, cs.SurfaceContainerHighest, cs.OnSurfaceVariant
	}
	ui.Row(c).AlignItems(ui.Center).Padding(design.SpaceXXS, design.SpaceSM).Background(bg).Radius(design.CornerFull).Children(func() {
		ui.Box(c).Size(6, 6).Radius(3).Background(fg)
		m3.HGap(c, design.SpaceXS)
		m3.Text(c, label, design.TT().LabelMedium.C(fg))
	})
}

// taskRow is a task in the list. With onToggle it is a card with a switch
// (narrow windows); without, a selectable row beside the detail pane.
func taskRow(c *ui.Context, task core.J, now time.Time, selected bool, onTap func(), onToggle func(bool)) {
	cs := design.S()
	tt := design.TT()
	enabled := task.IsTrue("enabled")
	next, hasNext := msTime(task["next_run"])
	last := task.O("last_run")
	state := task.Or("state", "idle")
	var line string
	switch {
	case state == "running":
		line = l10n.TasksRunningNow
	case state == "queued":
		line = l10n.CommonQueued
	case !enabled:
		line = l10n.CommonPaused
	case !hasNext:
		line = l10n.TasksNoNextRun
	default:
		line = l10n.TasksNextRun(FmtWhen(next, now))
	}
	card := onToggle != nil
	bg, radius := ui.Transparent, design.CornerLG
	if card {
		bg, radius = cs.SurfaceContainerLow, design.CornerXL
	}
	if selected {
		bg = cs.SecondaryContainer
	}
	t := m3.Tappable(c, radius).FillWidth().Background(bg).Selected(selected)
	t.Children(func() {
		right := float32(design.SpaceMD)
		if card {
			right = design.SpaceSM
		}
		ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(design.SpaceMD, right, design.SpaceMD, design.SpaceMD).Children(func() {
			taskGlyph(c, task, 44)
			m3.HGap(c, design.SpaceMD)
			ui.Column(c).Grow(1).Basis(0).Children(func() {
				nc := cs.OnSurface
				if !enabled {
					nc = cs.OnSurfaceVariant
				}
				m3.Clamp(m3.Text(c, task.S("name"), tt.TitleMedium.C(nc)), 1)
				m3.VGap(c, design.SpaceXXS)
				m3.Clamp(m3.Text(c, FmtSchedule(task.O("schedule"))+" · "+line, tt.BodyMedium.C(cs.OnSurfaceVariant)), 1)
				if last != nil {
					m3.VGap(c, design.SpaceXS)
					runLine(c, last, now)
				}
			})
			if state == "running" {
				ui.Box(c).Padding(design.SpaceSM).Children(func() { m3.Spinner(c, 18, 2, cs.Primary) })
			}
			if card {
				on := enabled
				if m3.Switch(c, &on, false).Changed() {
					onToggle(on)
				}
			}
		})
	})
	if t.Clicked() {
		onTap()
	}
}

// runLine is 完成 · 今天 21:31 · $0.012 with a status icon.
func runLine(c *ui.Context, run core.J, now time.Time) {
	cs := design.S()
	label, ic, bad := runStatus(run.S("status"))
	color := cs.OnSurfaceVariant
	icC := color
	switch {
	case bad:
		color, icC = cs.Error, cs.Error
	case run.S("status") == "ok":
		icC = cs.Primary
	}
	parts := []string{label}
	if at, ok := msTime(run["started"]); ok {
		parts = append(parts, FmtWhen(at, now))
	}
	if cost := run.F("cost"); cost > 0 {
		parts = append(parts, core.FmtUsd(cost))
	}
	ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
		m3.Icon(c, ic, 16, icC)
		m3.HGap(c, design.SpaceXS)
		m3.Clamp(m3.Text(c, strings.Join(parts, " · "), design.TT().LabelMedium.C(color)).FontFeatures("tnum"), 1).Shrink(1)
	})
}

// runTileView is one run; with showTask it names the task (cross-task
// history).
func runTileView(c *ui.Context, run core.J, now time.Time, showTask, isNew bool) {
	cs := design.S()
	tt := design.TT()
	status := run.S("status")
	label, ic, bad := runStatus(status)
	started, _ := msTime(run["started"])
	secs := int(math.Round((run.F("finished") - run.F("started")) / 1000))
	dur := l10n.DurationSeconds(secs)
	if secs >= 60 {
		dur = l10n.DurationMinutesSeconds(secs/60, secs%60)
	}
	cost := run.F("cost")
	session := run.S("session_id")
	summary := run.S("summary")
	manual := ""
	if run.IsTrue("manual") {
		manual = l10n.TasksManualSuffix
	}
	bg, fg := cs.SurfaceContainerHighest, cs.OnSurfaceVariant
	switch {
	case bad:
		bg, fg = cs.ErrorContainer, cs.OnErrorContainer
	case status == "ok":
		bg, fg = cs.PrimaryContainer, cs.OnPrimaryContainer
	}
	var title, sub string
	if showTask {
		title = run.Or("name", l10n.TasksDeletedTask)
		if run.IsTrue("deleted") && run["name"] != nil {
			title += l10n.TasksDeletedSuffix
		}
		sub = label + " · " + hm(started) + manual
		if summary != "" {
			sub += "\n" + summary
		}
	} else {
		title = label + " · " + FmtWhen(started, now) + manual
		sub = summary
	}
	lines := 2
	if showTask {
		lines = 3
	}
	padX := float32(0)
	if showTask {
		padX = design.SpaceLG
	}
	tile := m3.Tile{
		Pad: true, PadX: padX, Tap: session != "", NoRadius: true, SubtitleLines: lines,
		Leading: func() { widgets.CircleAvatar(c, 36, bg, func() { m3.Icon(c, ic, 20, fg) }) },
		TitleFn: func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				m3.Clamp(m3.Text(c, title, tt.BodyLarge), 1).Shrink(1)
				if isNew {
					m3.HGap(c, design.SpaceSM)
					ui.Box(c).Size(8, 8).Radius(4).Background(cs.Primary).Label(l10n.CommonNewBadge)
				}
			})
		},
		Subtitle: sub,
	}
	if status != "skipped" {
		tail := dur
		if cost > 0 {
			tail += "\n" + core.FmtUsd(cost)
		}
		tile.Trailing = func() {
			m3.Text(c, tail, tt.LabelMedium.C(cs.OnSurfaceVariant)).TextAlign(ui.End).FontFeatures("tnum")
		}
	}
	if tile.Show(c).Clicked() && session != "" {
		openRunConversation(session)
	}
}

// ---------------------------------------------------------------- detail

// taskDetailPage is one task on a narrow window, pushed over the list.
type taskDetailPage struct{ d *TaskDetail }

func newTaskDetailPage(id string) *taskDetailPage {
	p := &taskDetailPage{d: NewTaskDetail(id, false)}
	p.d.OnDeleted = func() { m3.Nav().PopPage(p) }
	return p
}

func (p *taskDetailPage) Dispose() { p.d.Dispose() }

func (p *taskDetailPage) View(c *ui.Context) {
	m3.Scaffold{AppBar: &m3.AppBar{Scrolled: p.d.scroll.Y > 0}, Body: func() { p.d.Show(c) }}.Show(c)
}

// TaskDetail is what a task does, its controls, and its run history.
type TaskDetail struct {
	id string
	// Embedded shows it in the pane beside the list rather than as its own
	// page.
	Embedded  bool
	OnChanged func()
	OnDeleted func()

	task   core.J
	runs   []core.J
	err    error
	cancel func()
	scroll ui.ScrollState
}

// NewTaskDetail loads task id and follows its runs.
func NewTaskDetail(id string, embedded bool) *TaskDetail {
	d := &TaskDetail{id: id, Embedded: embedded}
	d.load()
	d.cancel = State.Events.Listen(func(e core.J) {
		if e.S("type") == "agent_task" && e.O("data").S("task_id") == id {
			d.load()
		}
	})
	return d
}

func (d *TaskDetail) Dispose() { d.cancel() }

func (d *TaskDetail) load() {
	id := d.id
	core.Async(func() ([2]any, error) {
		all, err := api().Get("/agent/tasks")
		if err != nil {
			return [2]any{}, err
		}
		var t core.J
		for _, x := range core.Objs(all) {
			if x.S("id") == id {
				t = x
			}
		}
		if t == nil {
			return [2]any{}, nil
		}
		runs, err := api().Get("/agent/tasks/" + id + "/runs")
		return [2]any{t, core.Objs(runs)}, err
	}, func(v [2]any, err error) {
		if err != nil {
			d.err = err
			return
		}
		t, _ := v[0].(core.J)
		runs, _ := v[1].([]core.J)
		d.task, d.runs, d.err = t, runs, nil
		if t == nil {
			d.err = fmt.Errorf("%s", l10n.TasksNotFound)
		}
	})
}

// act runs action, then refreshes this view and the list.
func (d *TaskDetail) act(action func() error, done string) {
	core.Go(action, func(err error) {
		if err != nil {
			widgets.ShowSnack(err.Error(), "", nil)
		} else if done != "" {
			widgets.ShowSnack(done, "", nil)
		}
		d.load()
		if d.OnChanged != nil {
			d.OnChanged()
		}
	})
}

func (d *TaskDetail) delete() {
	m3.Confirm(l10n.TasksDeleteTitle, l10n.TasksDeleteBody, l10n.CommonCancel, l10n.CommonDelete, false, func() {
		core.Go(func() error { _, err := api().Delete("/agent/tasks/" + d.id); return err }, func(err error) {
			if err != nil {
				widgets.ShowSnack(err.Error(), "", nil)
				return
			}
			if d.OnDeleted != nil {
				d.OnDeleted()
			}
			if d.OnChanged != nil {
				d.OnChanged()
			}
		})
	})
}

func fact(c *ui.Context, color ui.Color, ic icons.Icon, label, value string, bad bool) {
	cs := design.S()
	tt := design.TT()
	ui.Column(c).Grow(1).Basis(0).Padding(design.SpaceMD).Background(color).Radius(design.CornerLG).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			m3.Icon(c, ic, 16, cs.OnSurfaceVariant)
			m3.HGap(c, design.SpaceXS)
			m3.Clamp(m3.Text(c, label, tt.LabelMedium.C(cs.OnSurfaceVariant)), 1).Shrink(1)
		})
		m3.VGap(c, design.SpaceXS)
		vc := cs.OnSurface
		if bad {
			vc = cs.Error
		}
		m3.Clamp(m3.Text(c, value, tt.TitleMedium.C(vc)), 1)
	})
}

// Show builds the detail filling its parent.
func (d *TaskDetail) Show(c *ui.Context) {
	cs := design.S()
	tt := design.TT()
	t := d.task
	if t == nil {
		if d.err == nil {
			widgets.CenteredLoader(c, "")
		} else {
			widgets.ErrorState(c, d.err, d.load)
		}
		return
	}
	now := time.Now()
	enabled := t.IsTrue("enabled")
	state := t.Or("state", "idle")
	next, hasNext := msTime(t["next_run"])
	var last core.J
	if len(d.runs) > 0 {
		last = d.runs[0]
	}
	tile := cs.SurfaceContainerLow
	if d.Embedded {
		tile = cs.SurfaceContainer
	}
	pad := float32(design.SpaceXL)
	if Compact() {
		pad = design.SpaceLG
	}
	var writes []string
	for _, w := range t.Ss("writes") {
		writes = append(writes, taskWriteLabel(w))
	}
	top := float32(design.SpaceSM)
	if d.Embedded {
		top = design.SpaceXL
	}
	ui.Scroll(c).Fill().TrackScroll(&d.scroll).Children(func() {
		ui.Column(c).FillWidth().Padding(top, pad, design.SpaceXXXL, pad).Children(func() {
			widgets.ContentWidth(c, 760, func() {
				ui.Row(c).FillWidth().AlignItems(ui.Start).Children(func() {
					taskGlyph(c, t, 56)
					m3.HGap(c, design.SpaceLG)
					ui.Column(c).Grow(1).Basis(0).Children(func() {
						m3.Text(c, t.S("name"), tt.HeadlineSmall)
						m3.VGap(c, design.SpaceXS)
						ui.Row(c).FillWidth().Wrap().Gap(design.SpaceSM).AlignItems(ui.Center).Children(func() {
							statePill(c, t)
							m3.Text(c, FmtSchedule(t.O("schedule")), tt.BodyMedium.C(cs.OnSurfaceVariant))
						})
					})
					tip := l10n.CommonEnable
					if enabled {
						tip = l10n.CommonPause
					}
					on := enabled
					sw := m3.Switch(c, &on, false)
					m3.Tip(c, sw, tip)
					if sw.Changed() {
						d.act(func() error { _, err := api().Put("/agent/tasks/"+d.id, core.J{"enabled": on}); return err }, "")
					}
				})
				m3.VGap(c, design.SpaceLG)
				ui.Row(c).FillWidth().Wrap().Gap(design.SpaceSM).Children(func() {
					label := l10n.TasksRunNow
					switch state {
					case "running":
						label = l10n.TasksRunningNow
					case "queued":
						label = l10n.CommonQueued
					}
					if (m3.Button{Kind: m3.Tonal, Icon: icons.PlayArrowRounded, Label: label, Disabled: state != "idle"}).Show(c).Clicked() {
						d.act(func() error { _, err := api().Post("/agent/tasks/"+d.id+"/run", nil); return err }, l10n.TasksStarted)
					}
					if (m3.Button{Kind: m3.Outlined, Icon: icons.EditOutlined, Label: l10n.CommonEdit}).Show(c).Clicked() {
						ShowTaskEditor(t, nil, func(core.J) { d.act(func() error { return nil }, "") })
					}
					if (m3.Button{Kind: m3.TextKind, Icon: icons.DeleteOutlineRounded, Label: l10n.CommonDelete, FG: cs.Error}).Show(c).Clicked() {
						d.delete()
					}
				})
				m3.VGap(c, design.SpaceXL)
				ui.Row(c).FillWidth().Gap(design.SpaceSM).Children(func() {
					nextV := l10n.CommonNone
					switch {
					case !enabled:
						nextV = l10n.CommonPaused
					case hasNext:
						nextV = FmtWhen(next, now)
					}
					fact(c, tile, icons.UpdateRounded, l10n.TasksNextRunLabel, nextV, false)
					lastV, bad := l10n.TasksNeverRun, false
					if last != nil {
						lastV, _, bad = runStatus(last.S("status"))
					}
					fact(c, tile, icons.HistoryRounded, l10n.TasksLastResult, lastV, bad)
					limit := l10n.TasksCostLimit
					if t.IsTrue("catch_up") {
						limit = l10n.TasksCostLimitCatchUp
					}
					budget := .2
					if _, ok := t["budget_usd"].(float64); ok {
						budget = t.F("budget_usd")
					}
					fact(c, tile, icons.SavingsOutlined, limit, core.FmtUsd(budget), false)
				})
				m3.VGap(c, design.SpaceMD)
				ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
					ic, text := icons.LockOutlineRounded, l10n.TasksReadOnly
					if len(writes) > 0 {
						ic, text = icons.EditNoteRounded, l10n.TasksAllowed(strings.Join(writes, l10n.ListSeparator))
					}
					m3.Icon(c, ic, 18, cs.OnSurfaceVariant)
					m3.HGap(c, design.SpaceSM)
					m3.Text(c, text, tt.BodyMedium.C(cs.OnSurfaceVariant)).Grow(1).Basis(0)
				})
				m3.VGap(c, design.SpaceXL)
				m3.Text(c, l10n.TasksPrompt, tt.TitleSmall)
				m3.VGap(c, design.SpaceSM)
				ui.Box(c).FillWidth().Padding(design.SpaceLG).Background(tile).Radius(design.CornerLG).Children(func() {
					m3.Text(c, t.S("prompt"), tt.BodyMedium.H(1.6)).Selectable()
				})
				m3.VGap(c, design.SpaceXL)
				ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
					m3.Text(c, l10n.TasksRuns, tt.TitleSmall).Grow(1)
					if len(d.runs) > 0 {
						m3.Text(c, l10n.TasksRecentRuns(len(d.runs)), tt.LabelMedium.C(cs.OnSurfaceVariant))
					}
				})
				m3.VGap(c, design.SpaceXS)
				if len(d.runs) == 0 {
					m3.Text(c, l10n.TasksNoRunsYet, tt.BodyMedium.C(cs.OnSurfaceVariant))
				}
				for i, r := range d.runs {
					runTileView(c.Key(i), r, now, false, false)
				}
			})
		})
	})
}

// ---------------------------------------------------------------- editor

type taskEditor struct {
	task                 core.J
	isTemplate           bool
	name, prompt, budget string
	kind                 string
	tm                   m3.TimeOfDay
	days                 map[int]bool
	once                 time.Time
	catchUp              bool
	writes               map[string]bool
	saving               bool
	err                  string
	scroll               ui.ScrollState
	done                 func(core.J)
}

func parseTimeOfDay(s string) m3.TimeOfDay {
	p := strings.SplitN(s, ":", 2)
	h, err := strconv.Atoi(p[0])
	if err != nil {
		h = 21
	}
	m := 30
	if len(p) > 1 {
		if v, err := strconv.Atoi(p[1]); err == nil {
			m = v
		}
	}
	return m3.TimeOfDay{Hour: h, Minute: m}
}

// ShowTaskEditor creates (task nil) or edits a task; done receives the
// saved task.
func ShowTaskEditor(task, template core.J, done func(core.J)) {
	src := task
	if src == nil {
		src = template
	}
	if src == nil {
		src = core.J{}
	}
	s := src.O("schedule")
	e := &taskEditor{task: task, isTemplate: template != nil, name: src.S("name"), prompt: src.S("prompt"),
		kind: s.Or("type", "daily"), tm: parseTimeOfDay(s.Or("time", "21:30")), days: map[int]bool{},
		catchUp: src["catch_up"] != false, writes: map[string]bool{}, done: done}
	if s.S("type") == "" {
		e.kind = "daily"
	}
	if IsWorkdays(s) {
		e.kind = "workdays"
	}
	e.budget = "0.2"
	if v, ok := src["budget_usd"]; ok {
		e.budget = strconv.FormatFloat(core.Float(v), 'f', -1, 64)
	}
	days := s.L("days")
	if days == nil {
		days = []any{float64(1)}
	}
	for _, d := range days {
		e.days[core.Int(d)] = true
	}
	if at, err := parseLocal(s.S("at")); err == nil {
		e.once = at
	} else {
		e.once = time.Now().Add(time.Hour)
	}
	for _, w := range src.Ss("writes") {
		e.writes[w] = true
	}
	if Compact() {
		var page m3.Page
		page = m3.PageFunc(func(c *ui.Context) {
			e.view(c, func() { m3.RootNav.PopPage(page) })
		})
		m3.RootNav.Push(page)
		return
	}
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		_, h := c.Size()
		ui.Column(c).Width(560).Height(min(760, h-80)).Radius(design.CornerXL).Background(design.S().SurfaceContainerHigh).Clip().Shrink(1).Children(func() {
			e.view(c, r.Close)
		})
	}, nil)
}

func (e *taskEditor) hhmm() string { return fmt.Sprintf("%02d:%02d", e.tm.Hour, e.tm.Minute) }

func (e *taskEditor) schedule() core.J {
	switch e.kind {
	case "weekly":
		var days []int
		for d, on := range e.days {
			if on {
				days = append(days, d)
			}
		}
		sort.Ints(days)
		out := make([]any, len(days))
		for i, d := range days {
			out[i] = d
		}
		return core.J{"type": "weekly", "days": out, "time": e.hhmm()}
	case "workdays":
		return core.J{"type": "weekly", "days": []any{1, 2, 3, 4, 5}, "time": e.hhmm()}
	case "once":
		return core.J{"type": "once", "at": core.FmtIsoDay(e.once) + "T" + hm(e.once)}
	}
	return core.J{"type": "daily", "time": e.hhmm()}
}

func (e *taskEditor) save(close func()) {
	e.saving, e.err = true, ""
	budget, err := strconv.ParseFloat(strings.TrimSpace(e.budget), 64)
	if err != nil {
		budget = -1
	}
	var writes []string
	for w, on := range e.writes {
		if on {
			writes = append(writes, w)
		}
	}
	sort.Strings(writes)
	ws := make([]any, len(writes))
	for i, w := range writes {
		ws[i] = w
	}
	body := core.J{"name": strings.TrimSpace(e.name), "prompt": strings.TrimSpace(e.prompt), "schedule": map[string]any(e.schedule()),
		"budget_usd": budget, "catch_up": e.catchUp, "writes": ws}
	core.Async(func() (any, error) {
		if e.task == nil {
			return api().Post("/agent/tasks", body)
		}
		return api().Put("/agent/tasks/"+e.task.S("id"), body)
	}, func(v any, err error) {
		if err != nil {
			e.saving, e.err = false, err.Error()
			return
		}
		close()
		if e.done != nil {
			e.done(core.Obj(v))
		}
	})
}

func (e *taskEditor) view(c *ui.Context, close func()) {
	cs := design.S()
	tt := design.TT()
	label := func(t string) {
		m3.Text(c, t, tt.LabelLarge.C(cs.OnSurfaceVariant)).Padding(design.SpaceLG, 0, design.SpaceSM, 0)
	}
	title := l10n.TasksEditTitle
	if e.task == nil {
		title = l10n.TasksNewTitle
	}
	m3.Scaffold{
		BG: cs.SurfaceContainerHigh,
		AppBar: &m3.AppBar{Title: title, BG: cs.SurfaceContainerHigh, Scrolled: e.scroll.Y > 0,
			Leading: func() {
				if (m3.IconButton{Icon: icons.CloseRounded, Tooltip: l10n.CommonCancel}).Show(c).Clicked() {
					close()
				}
			},
			Actions: func() {
				if (m3.Button{Kind: m3.Filled, Label: l10n.CommonSave, Disabled: e.saving}).Show(c).Clicked() {
					e.save(close)
				}
				m3.HGap(c, design.SpaceMD)
			}},
		Body: func() {
			ui.Scroll(c).Fill().TrackScroll(&e.scroll).Children(func() {
				ui.Column(c).FillWidth().Padding(design.SpaceSM, design.SpaceXL, design.SpaceXXL, design.SpaceXL).Children(func() {
					if e.task == nil && !e.isTemplate {
						ui.Row(c).FillWidth().Wrap().Gap(design.SpaceSM).Children(func() {
							for i, t := range TaskTemplates {
								t := t
								if (m3.Chip{Kind: m3.ActionChip, Avatar: t.Icon, Label: t.Name}).Show(c.Key(i)).Clicked() {
									e.name, e.prompt = t.Name, t.Prompt
									e.kind = t.Sched.S("type")
									e.tm = parseTimeOfDay(t.Sched.S("time"))
									if d := t.Sched.L("days"); d != nil {
										e.days = map[int]bool{}
										for _, x := range d {
											e.days[core.Int(x)] = true
										}
									}
									e.writes = map[string]bool{}
									for _, w := range t.Writes {
										e.writes[w] = true
									}
								}
							}
						})
						m3.VGap(c, design.SpaceLG)
					}
					m3.Field{Label: l10n.CommonName, MaxLength: 40, Counter: true}.Show(c, &e.name)
					m3.VGap(c, design.SpaceSM)
					m3.Field{Label: l10n.TasksPromptLabel, MinLines: 4, MaxLines: 10, MaxLength: 4000, Counter: true}.Show(c, &e.prompt)
					label(l10n.CommonTime)
					m3.Segmented(c, []m3.Segment[string]{{Value: "daily", Label: l10n.CommonDaily}, {Value: "workdays", Label: l10n.TasksWorkdays},
						{Value: "weekly", Label: l10n.CommonWeekly}, {Value: "once", Label: l10n.ScheduleOnce}}, &e.kind, m3.SegOpts{NoCheck: true})
					m3.VGap(c, design.SpaceMD)
					if e.kind == "weekly" {
						ui.Row(c).FillWidth().Wrap().Gap(design.SpaceXS).Children(func() {
							for d := 1; d <= 7; d++ {
								if m3.FilterChipOf(c.Key(d), l10n.WeekdayName(core.Weekdays[d-1]), e.days[d]) {
									e.days[d] = !e.days[d]
								}
							}
						})
						m3.VGap(c, design.SpaceMD)
					}
					ui.Row(c).FillWidth().Wrap().Gap(design.SpaceSM).Children(func() {
						if e.kind == "once" {
							if (m3.Button{Kind: m3.Outlined, Icon: icons.CalendarTodayOutlined, Label: l10n.DateMonthDay(int(e.once.Month()), e.once.Day())}).Show(c).Clicked() {
								now := time.Now()
								init := e.once
								if init.Before(now) {
									init = now
								}
								m3.ShowDatePicker(m3.DatePicker{Initial: init, First: core.Midnight(now), Last: now.AddDate(0, 0, 365)}, func(d time.Time) {
									e.once = time.Date(d.Year(), d.Month(), d.Day(), e.once.Hour(), e.once.Minute(), 0, 0, time.Local)
								})
							}
						}
						tl := e.hhmm()
						init := e.tm
						if e.kind == "once" {
							tl = hm(e.once)
							init = m3.TimeOfDay{Hour: e.once.Hour(), Minute: e.once.Minute()}
						}
						if (m3.Button{Kind: m3.Outlined, Icon: icons.ScheduleRounded, Label: tl}).Show(c).Clicked() {
							m3.ShowTimePicker(init, func(t m3.TimeOfDay) {
								if e.kind == "once" {
									e.once = time.Date(e.once.Year(), e.once.Month(), e.once.Day(), t.Hour, t.Minute, 0, 0, time.Local)
								} else {
									e.tm = t
								}
							})
						}
					})
					label(l10n.TasksAllowLabel)
					ui.Row(c).FillWidth().Wrap().Gap(design.SpaceSM).Children(func() {
						for _, w := range TaskWrites {
							if (m3.Chip{Kind: m3.FilterChip, Avatar: w.Icon, Label: w.Label, Selected: e.writes[w.Key]}).Show(c.Key(w.Key)).Clicked() {
								e.writes[w.Key] = !e.writes[w.Key]
							}
						}
					})
					m3.Text(c, l10n.TasksAllowHint, tt.BodySmall.C(cs.OnSurfaceVariant)).Margin(design.SpaceXS, 0, 0, 0)
					label(l10n.TasksLimits)
					m3.Field{Label: l10n.TasksCostLimitLabel, PrefixText: "$ ", Helper: l10n.TasksCostLimitHelper}.Show(c, &e.budget)
					m3.SwitchTile(c, m3.Tile{Title: l10n.TasksCatchUp, Subtitle: l10n.TasksCatchUpHint, Pad: true}, &e.catchUp)
					if e.err != "" {
						m3.Text(c, e.err, tt.BodyMedium.C(cs.Error)).Margin(design.SpaceSM, 0, 0, 0)
					}
				})
			})
		},
	}.Show(c)
}
