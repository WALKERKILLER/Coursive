package app

import (
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

// KindStyle is a schedule kind's look, from colour scheme roles.
type KindStyle struct {
	Label                          string
	Icon                           icons.Icon
	Container, OnContainer, Accent ui.Color
}

var scheduleKinds = []string{"homework", "exam", "activity", "deadline", "task"}

// KindStyleOf is KindStyle.of.
func KindStyleOf(kind string) KindStyle {
	cs := design.S()
	switch kind {
	case "homework":
		return KindStyle{l10n.CommonAssignments, icons.EditNoteRounded, cs.PrimaryContainer, cs.OnPrimaryContainer, cs.Primary}
	case "exam":
		return KindStyle{l10n.ScheduleKindExam, icons.QuizOutlined, cs.TertiaryContainer, cs.OnTertiaryContainer, cs.Tertiary}
	case "activity":
		return KindStyle{l10n.ScheduleKindEvent, icons.Groups2Outlined, cs.SecondaryContainer, cs.OnSecondaryContainer, cs.Secondary}
	case "deadline":
		return KindStyle{l10n.ScheduleKindDeadline, icons.FlagOutlined, cs.ErrorContainer, cs.OnErrorContainer, cs.Error}
	}
	return KindStyle{l10n.ScheduleTodos, icons.TaskAltRounded, cs.SurfaceContainerHighest, cs.OnSurfaceVariant, cs.Outline}
}

func weekdayLabel(d time.Time) string { return l10n.WeekdayName(core.Weekdays[core.Weekday(d)-1]) }

// fmtDue is when an item is due: today, tomorrow… with its time.
func fmtDue(it core.ScheduleItem) string {
	d := it.Day()
	diff := core.DaysBetween(d, DayOf(time.Now()))
	var rel string
	switch diff {
	case 0:
		rel = l10n.CommonToday
	case 1:
		rel = l10n.CommonTomorrow
	case 2:
		rel = l10n.CommonDayAfterTomorrow
	case -1:
		rel = l10n.CommonYesterday
	default:
		rel = l10n.DateMonthDay(int(d.Month()), d.Day())
	}
	parts := []string{rel}
	if diff > 1 || diff < -1 {
		parts = append(parts, weekdayLabel(d))
	}
	if it.DueTime() != "" {
		parts = append(parts, it.DueTime())
	}
	return strings.Join(parts, " ")
}

func fmtCountdown(it core.ScheduleItem) string {
	diff := core.DaysBetween(it.Day(), DayOf(time.Now()))
	switch {
	case it.Done():
		return l10n.CommonDone
	case diff < 0:
		return l10n.ScheduleOverdueDays(-diff)
	case diff == 0:
		return l10n.CommonToday
	}
	return l10n.ScheduleDaysLeft(diff)
}

// DateLeaf is a calendar leaf: the month on a tinted band, the day large
// below.
func DateLeaf(c *ui.Context, date time.Time, st KindStyle, size float32, muted bool) {
	cs := design.S()
	tt := design.TT()
	if size == 0 {
		size = 52
	}
	ui.Column(c).Size(size, size*1.12).Shrink(0).Radius(design.CornerMD).Clip().Background(cs.SurfaceContainerHighest).Children(func() {
		band, on := st.Container, st.OnContainer
		if muted {
			band, on = cs.SurfaceContainerHigh, cs.OnSurfaceVariant
		}
		ui.Box(c).FillWidth().Height(size * .32).Background(band).Center().Children(func() {
			m3.Text(c, l10n.DateMonth(int(date.Month())), tt.LabelSmall.C(on).H(1))
		})
		ui.Column(c).FillWidth().Grow(1).Basis(0).Center().AlignItems(ui.Center).Children(func() {
			fg := cs.OnSurface
			if muted {
				fg = cs.OnSurfaceVariant
			}
			m3.Text(c, strconv.Itoa(date.Day()), tt.TitleLarge.C(fg).H(1).W(600)).NoWrap()
			m3.Text(c, weekdayLabel(date), tt.LabelSmall.C(cs.OnSurfaceVariant).S(10).H(1.3)).NoWrap()
		})
	})
}

// KindLabel is a small tonal label with the kind's icon.
func KindLabel(c *ui.Context, st KindStyle) {
	ui.Row(c).AlignItems(ui.Center).Padding(2, 8, 2, 6).Radius(design.CornerFull).Background(st.Container).Children(func() {
		m3.Icon(c, st.Icon, 13, st.OnContainer)
		m3.HGap(c, 3)
		m3.Text(c, st.Label, design.TT().LabelSmall.C(st.OnContainer))
	})
}

// LmsBadge marks an item from a learning platform.
func LmsBadge(c *ui.Context) {
	cs := design.S()
	ui.Row(c).AlignItems(ui.Center).Padding(2, 8, 2, 6).Radius(design.CornerFull).Border(1, cs.OutlineVariant).Children(func() {
		m3.Icon(c, icons.CloudSyncOutlined, 13, cs.OnSurfaceVariant)
		m3.HGap(c, 3)
		m3.Text(c, l10n.LmsBadge, design.TT().LabelSmall.C(cs.OnSurfaceVariant))
	})
}

var easeOutBack = ui.CubicBezier(.175, .885, .32, 1.275)

// DoneCheck is a round check that fills with a short spring; it reports a
// toggle.
func DoneCheck(c *ui.Context, key any, done bool, color ui.Color) bool {
	cs := design.S()
	b := ui.ButtonBase(c.Key(key)).FocusRing(false).Size(40, 40).Center().Shrink(0).Cursor(ui.CursorPointer).
		Role(ui.RoleCheckBox).Checked(done)
	m3.Ink(b, cs.OnSurface, 22)
	t := b.AnimateTo("done", map[bool]float32{true: 1, false: 0}[done], design.T().Motion(design.Medium2, easeOutBack))
	k := min(max(t, 0), 1)
	b.Children(func() {
		ui.Box(c).Size(22, 22).Radius(11).Background(design.Lerp(ui.Transparent, color, k)).
			Border(2, design.Lerp(cs.Outline, color, k)).Center().Children(func() {
			sc := min(max(t, 0), 1.2)
			m3.Icon(c, icons.CheckRounded, 15, cs.Surface).Scale(sc, sc)
		})
	})
	return b.Clicked()
}

// openMention jumps to where an item was mentioned in its lecture.
func openMention(it core.ScheduleItem) {
	if it.LectureID() == "" {
		return
	}
	SeekOrOpen(it.LectureID(), max(it.TMs(), 0)/1000)
}

// ScheduleTile is one agenda row.
func ScheduleTile(c *ui.Context, it core.ScheduleItem, showDate bool) {
	cs := design.S()
	tt := design.TT()
	st := KindStyleOf(it.Kind())
	var meta []string
	if showDate {
		meta = append(meta, fmtDue(it))
	} else if it.DueTime() != "" {
		meta = append(meta, it.DueTime())
	}
	if it.CourseTitle() != "" {
		meta = append(meta, it.CourseTitle())
	}
	op := float32(1)
	if it.Done() {
		op = .6
	}
	tile := m3.Tappable(c.Key("tile-"+it.ID()), design.CornerLG).FillWidth().Background(cs.SurfaceContainerLow).Clip()
	tile = tile.Opacity(tile.AnimateTo("op", op, design.T().Motion(design.Medium2, nil)))
	tile.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Start).Padding(design.SpaceSM, design.SpaceXS, design.SpaceSM, design.SpaceXS).Children(func() {
			if DoneCheck(c, "check", it.Done(), st.Accent) {
				Schedule.Patch(it, core.J{"done": !it.Done()}, func(err error) {
					if err != nil {
						widgets.ShowSnack(l10n.CommonUpdateFailed(err), "", nil)
					}
				})
			}
			ui.Column(c).Grow(1).Basis(0).Padding(design.SpaceSM, 0, 0, 0).Children(func() {
				title := m3.Text(c, it.Title(), tt.TitleSmall)
				if it.Done() {
					title.Strikethrough().DecorationColor(cs.OnSurfaceVariant)
				}
				m3.VGap(c, design.SpaceXS)
				ui.Row(c).Wrap().GapX(design.SpaceSM).GapY(design.SpaceXS).AlignItems(ui.Center).Children(func() {
					KindLabel(c, st)
					if it.IsLms() {
						LmsBadge(c)
					}
					if len(meta) > 0 {
						m3.Text(c, strings.Join(meta, " · "), tt.BodySmall.C(cs.OnSurfaceVariant))
					}
					if it.Overdue() {
						m3.Text(c, fmtCountdown(it), tt.LabelSmall.C(cs.Error))
					}
				})
				if it.Quote() != "" {
					m3.VGap(c, design.SpaceSM)
					scheduleMention(c, it)
				}
			})
			ctx := core.J{"schedule_id": it.ID()}
			if it.CourseID() != "" {
				ctx["course_id"] = it.CourseID()
			}
			if it.LectureID() != "" {
				ctx["lecture_id"] = it.LectureID()
			}
			PluginMenuButton(c, "schedule.item", ctx, false, "")
			m3.MenuButton(c, "more", icons.MoreVertRounded, l10n.CommonMore, m3.BelowEnd, func() []m3.MenuItem {
				remove := func() {
					Schedule.Remove(it, func(err error) {
						if err != nil {
							return
						}
						what := l10n.CommonDeleted
						if it.IsLms() {
							what = l10n.ScheduleHiddenFromSchedule
						}
						widgets.ShowSnack(l10n.ScheduleRemovedNotice(what, it.Title()), l10n.CommonUndo, func() { Schedule.Restore(it, nil) })
					})
				}
				if it.IsLms() {
					var items []m3.MenuItem
					if it.URL() != "" {
						items = append(items, m3.MenuItem{Icon: icons.OpenInNewRounded, Label: l10n.ScheduleOpenOnPlatform, OnTap: func() { openURL(it.URL()) }})
					}
					return append(items, m3.MenuItem{Icon: icons.VisibilityOffOutlined, Label: l10n.ScheduleHide, OnTap: remove})
				}
				return []m3.MenuItem{
					{Icon: icons.EditOutlined, Label: l10n.CommonEdit, OnTap: func() { ShowScheduleEditor(&it, time.Time{}) }},
					{Icon: icons.DeleteOutlineRounded, Label: l10n.CommonDelete, OnTap: remove},
				}
			})
		})
	})
	if tile.Clicked() {
		if it.IsLms() {
			openURL(it.URL())
		} else {
			ShowScheduleEditor(&it, time.Time{})
		}
	}
}

// scheduleMention is the words that mentioned the item, with a jump to the
// recording.
func scheduleMention(c *ui.Context, it core.ScheduleItem) {
	cs := design.S()
	tt := design.TT()
	ui.Row(c).FillWidth().AlignItems(ui.Start).Children(func() {
		ic := icons.FormatQuoteRounded
		if it.Source() == "agent" {
			ic = icons.AutoAwesomeOutlined
		}
		m3.Icon(c, ic, 14, cs.OnSurfaceVariant).Margin(2, design.SpaceSM, 0, 0)
		m3.Clamp(m3.Text(c, "“"+it.Quote()+"”", tt.BodySmall.C(cs.OnSurfaceVariant).H(1.45)), 2).Grow(1).Basis(0)
		if it.LectureID() != "" && it.TMs() >= 0 {
			m3.HGap(c, design.SpaceSM)
			r := m3.Chip{Kind: m3.ActionChip, Avatar: icons.PlayArrowRounded, Label: core.FmtClock(it.TMs()),
				Tooltip: l10n.ScheduleJumpToSource, Compact: true}.Show(c.Key("jump"))
			if r.Clicked() {
				openMention(it)
			}
		}
	})
}

// openURL opens a web link of an item from the learning platform.
func openURL(u string) { OpenLink(u) }

type scheduleEditor struct {
	item          *core.ScheduleItem
	title, detail string
	kind          string
	date          time.Time
	tm            string
	course        string
	busy          bool
	kinds         ui.ScrollState
}

// ShowScheduleEditor creates or edits an item: a dialog on wide windows, a
// sheet on compact ones. Items of a learning platform open there instead.
func ShowScheduleEditor(item *core.ScheduleItem, day time.Time) {
	if item != nil && item.IsLms() {
		openURL(item.URL())
		return
	}
	e := &scheduleEditor{item: item, kind: "homework"}
	switch {
	case item != nil:
		e.title, e.detail, e.kind, e.date, e.tm, e.course = item.Title(), item.Detail(), item.Kind(), item.Day(), item.DueTime(), item.CourseID()
	case !day.IsZero():
		e.date = day
	default:
		e.date = DayOf(time.Now()).AddDate(0, 0, 7)
	}
	if !Compact() {
		m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
			ui.Column(c).MaxWidth(520).Radius(design.CornerXL).Background(design.S().SurfaceContainerHigh).Clip().Shrink(1).Children(func() {
				e.view(c, r, true)
			})
		}, nil)
		return
	}
	m3.ShowModalBottomSheet(m3.SheetOptions{ScrollControlled: true}, func(c *ui.Context, r *m3.Route) { e.view(c, r, false) })
}

func (e *scheduleEditor) save(r *m3.Route) {
	if strings.TrimSpace(e.title) == "" {
		return
	}
	e.busy = true
	fields := core.J{"title": strings.TrimSpace(e.title), "kind": e.kind, "detail": strings.TrimSpace(e.detail),
		"due_date": IsoDate(e.date), "due_time": e.tm, "course_id": e.course}
	fail := func(err error) {
		e.busy = false
		widgets.ShowSnack(l10n.CommonSaveFailed(err), "", nil)
	}
	if e.item == nil {
		Schedule.Add(fields, func(_ core.ScheduleItem, err error) {
			if err != nil {
				fail(err)
				return
			}
			r.Close()
		})
		return
	}
	Schedule.Patch(*e.item, fields, func(err error) {
		if err != nil {
			fail(err)
			return
		}
		r.Close()
	})
}

func (e *scheduleEditor) view(c *ui.Context, r *m3.Route, wide bool) {
	cs := design.S()
	tt := design.TT()
	top := float32(0)
	if wide {
		top = design.SpaceXL
	}
	ui.Scroll(c).FillWidth().Shrink(1).Children(func() {
		ui.Column(c).FillWidth().Padding(top, design.SpaceXL, design.SpaceXL, design.SpaceXL).Children(func() {
			head := l10n.ScheduleNew
			if e.item != nil {
				head = l10n.ScheduleEdit
			}
			m3.Text(c, head, tt.HeadlineSmall)
			m3.VGap(c, design.SpaceLG)
			in := m3.Field{Label: l10n.CommonTitle, Hint: l10n.ScheduleTitleHint, AutoFocus: e.item == nil, MaxLength: 80, Counter: true}.Show(c, &e.title)
			if in.Submitted() {
				e.save(r)
			}
			m3.VGap(c, design.SpaceSM)
			widgets.HScroller{State: &e.kinds, Fade: true}.Show(c, func() {
				var segs []m3.Segment[string]
				for _, k := range scheduleKinds {
					st := KindStyleOf(k)
					segs = append(segs, m3.Segment[string]{Value: k, Label: st.Label, Icon: st.Icon})
				}
				m3.Segmented(c, segs, &e.kind, m3.SegOpts{NoCheck: true})
			})
			m3.VGap(c, design.SpaceLG)
			ui.Row(c).Wrap().Gap(design.SpaceSM).Children(func() {
				d := e.date
				if (m3.Chip{Kind: m3.InputChip, Avatar: icons.EventRounded,
					Label: l10n.DateYearMonthDayWeek(d.Year(), int(d.Month()), d.Day(), core.Weekdays[core.Weekday(d)-1])}).Show(c.Key("date")).Clicked() {
					m3.ShowDatePicker(m3.DatePicker{Initial: e.date, First: time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local),
						Last: time.Date(2100, 1, 1, 0, 0, 0, 0, time.Local), Help: l10n.ScheduleDueDate}, func(t time.Time) { e.date = t })
				}
				label := e.tm
				if label == "" {
					label = l10n.ScheduleAllDay
				}
				tc := m3.Chip{Kind: m3.InputChip, Avatar: icons.ScheduleRounded, Label: label, Deletable: e.tm != ""}.Show(c.Key("time"))
				if tc.Deleted {
					e.tm = ""
				} else if tc.Clicked() {
					init := m3.TimeOfDay{Hour: 23, Minute: 59}
					if p := strings.Split(e.tm, ":"); len(p) == 2 {
						h, _ := strconv.Atoi(p[0])
						m, _ := strconv.Atoi(p[1])
						init = m3.TimeOfDay{Hour: h, Minute: m}
					}
					m3.ShowTimePicker(init, func(t m3.TimeOfDay) {
						e.tm = two(t.Hour) + ":" + two(t.Minute)
					})
				}
			})
			m3.VGap(c, design.SpaceLG)
			entries := []m3.DropdownEntry[string]{{Value: "", Label: l10n.ScheduleNoCourse}}
			for _, co := range State.Courses {
				if !co.Hidden() {
					entries = append(entries, m3.DropdownEntry[string]{Value: co.ID(), Label: co.Title()})
				}
			}
			m3.Dropdown(c, "course", m3.DropdownOpts{Label: l10n.CommonCourses, LeadingIcon: icons.SchoolOutlined}, entries, &e.course)
			m3.VGap(c, design.SpaceLG)
			m3.Field{Label: l10n.ScheduleNote, Hint: l10n.ScheduleNoteHint, MinLines: 2, MaxLines: 5}.Show(c, &e.detail)
			if e.item != nil && e.item.Quote() != "" {
				m3.VGap(c, design.SpaceLG)
				ui.Box(c).FillWidth().Padding(design.SpaceMD).Radius(design.CornerMD).Background(cs.SurfaceContainerHigh).Children(func() {
					scheduleMention(c, *e.item)
				})
			}
			m3.VGap(c, design.SpaceXL)
			ui.Row(c).FillWidth().Justify(ui.End).Gap(design.SpaceSM).Children(func() {
				if m3.TextButton(c, l10n.CommonCancel).Clicked() {
					r.Close()
				}
				ok := l10n.CommonSave
				if e.item == nil {
					ok = l10n.CommonAdd
				}
				if (m3.Button{Kind: m3.Filled, Label: ok, Disabled: e.busy}).Show(c).Clicked() {
					e.save(r)
				}
			})
		})
	})
}

func two(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

// ScheduleChatCard is what the assistant shows after mark_schedule: the
// item as a calendar leaf with its source, and undo, done and open.
func ScheduleChatCard(c *ui.Context, data core.J, duplicate bool) {
	cs := design.S()
	tt := design.TT()
	live, ok := Schedule.ByID(data.S("id"))
	it := live
	if !ok {
		it = core.ScheduleItem{J: data}
	}
	removed := !ok && Schedule.Loaded
	st := KindStyleOf(it.Kind())
	card := ui.Column(c.Key("sched-"+data.S("id"))).FillWidth().Margin(design.SpaceXS, 0).
		Padding(design.SpaceMD, design.SpaceSM, design.SpaceXS, design.SpaceMD).Radius(design.CornerXL).Background(cs.SurfaceContainerHigh)
	t := entered(c, card, "in", design.Long2)
	k := design.EmphasizedDecelerate(t)
	card.Opacity(k).Translate(0, 8*(1-k)).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Start).Children(func() {
			DateLeaf(c, it.Day(), st, 52, removed)
			m3.HGap(c, design.SpaceMD)
			ui.Column(c).Grow(1).Basis(0).Children(func() {
				ts := tt.TitleSmall
				if removed {
					ts = ts.C(cs.OnSurfaceVariant)
				}
				title := m3.Text(c, it.Title(), ts)
				if removed || it.Done() {
					title.Strikethrough()
				}
				m3.VGap(c, design.SpaceXS)
				ui.Row(c).Wrap().GapX(design.SpaceSM).GapY(design.SpaceXS).AlignItems(ui.Center).Children(func() {
					KindLabel(c, st)
					line := fmtDue(it) + " · " + fmtCountdown(it)
					switch {
					case removed:
						line = l10n.CommonUndone
					case duplicate:
						line = l10n.ScheduleAlreadyAdded
					}
					m3.Text(c, line, tt.BodySmall.C(cs.OnSurfaceVariant))
				})
				if it.Quote() != "" {
					m3.VGap(c, design.SpaceSM)
					scheduleMention(c, it)
				}
			})
		})
		ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
			if removed {
				if (m3.Button{Kind: m3.TextKind, Icon: icons.UndoRounded, Label: l10n.CommonRestore}).Show(c).Clicked() {
					Schedule.Restore(it, nil)
				}
				return
			}
			if m3.TextButton(c, l10n.CommonUndo).Clicked() {
				Schedule.Remove(it, nil)
			}
			done := l10n.CommonFinished
			if it.Done() {
				done = l10n.ScheduleMarkUndone
			}
			if m3.TextButton(c, done).Clicked() {
				Schedule.Patch(it, core.J{"done": !it.Done()}, nil)
			}
			if m3.TextButton(c, l10n.ToolListSchedule).Clicked() {
				d := it.Day()
				Focus.Day = &d
				// The card can sit in a full-screen assistant or the player:
				// show the tab on top.
				m3.RootNav.PopUntilRoot()
				m3.CloseAll()
				GoTo(TabSchedule)
			}
		})
	})
}

// SchedulePage is the month calendar beside an agenda of homework, exams
// and activities.
type SchedulePage struct {
	selected, month time.Time
	all, monthOpen  bool
	reveal          bool
	dir             int
	scroll          ui.ScrollState
}

// NewSchedulePage starts at today.
func NewSchedulePage() m3.Page {
	p := &SchedulePage{selected: DayOf(time.Now()), dir: 1}
	p.month = time.Date(p.selected.Year(), p.selected.Month(), 1, 0, 0, 0, 0, time.Local)
	return p
}

// MainScroll is the agenda's scroll, which shrinks the dock.
func (p *SchedulePage) MainScroll() *ui.ScrollState { return &p.scroll }

func (p *SchedulePage) selectDay(d time.Time) {
	p.selected = DayOf(d)
	p.month = time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, time.Local)
	if p.selected.Before(DayOf(time.Now())) {
		p.all = true
	}
	p.reveal = true
}

// View builds the page.
func (p *SchedulePage) View(c *ui.Context) {
	if Focus.Day != nil {
		d := *Focus.Day
		Focus.Day = nil
		p.selectDay(d)
	}
	cs := design.S()
	tt := design.TT()
	open, overdue := 0, 0
	for _, i := range Schedule.Items {
		if !i.Done() {
			open++
		}
		if i.Overdue() {
			overdue++
		}
	}
	ReportScreen(map[string]string{"screen": l10n.CommonSchedule, "open_items": strconv.Itoa(open),
		"overdue": strconv.Itoa(overdue), "selected_day": IsoDate(p.selected)})
	root := ui.Box(c).Fill()
	wide := widgets.Width(root, 1000) >= 760
	root.Children(func() {
		header := func() {
			pad := design.SpaceLG
			if wide {
				pad = design.SpaceXL
			}
			ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(pad, design.SpaceLG, design.SpaceSM, pad).Children(func() {
				ui.Column(c).Grow(1).Basis(0).Children(func() {
					m3.Text(c, l10n.CommonSchedule, tt.HeadlineMedium)
					m3.VGap(c, 2)
					parts := []string{l10n.ScheduleNoTodos}
					if open > 0 {
						parts[0] = l10n.ScheduleOpenTodos(open)
					}
					col := cs.OnSurfaceVariant
					if overdue > 0 {
						parts = append(parts, l10n.ScheduleOverdue(overdue))
						col = cs.Error
					}
					m3.Text(c, strings.Join(parts, " · "), tt.BodyMedium.C(col))
				})
				m3.Segmented(c, []m3.Segment[bool]{{Value: false, Label: l10n.ScheduleTodos}, {Value: true, Label: l10n.CommonAll}}, &p.all, m3.SegOpts{NoCheck: true})
			})
		}
		calendar := func() { p.calendar(c, !wide && !p.monthOpen, !wide) }
		agenda := func() {
			switch {
			case !Schedule.Loaded:
				widgets.CenteredLoader(c, "")
			case len(Schedule.Items) == 0:
				widgets.EmptyState{Icon: icons.EventAvailableOutlined, Title: l10n.ScheduleEmpty, Body: l10n.ScheduleEmptyBody, Action: func() {
					if (m3.Button{Kind: m3.Tonal, Icon: icons.AddRounded, Label: l10n.ScheduleAdd}).Show(c).Clicked() {
						ShowScheduleEditor(nil, p.selected)
					}
				}}.Show(c)
			default:
				p.agenda(c)
			}
		}
		m3.Scaffold{
			BG: ui.Transparent,
			FAB: func() {
				if (m3.FAB{Icon: icons.AddRounded, Label: l10n.CommonNew}).Show(c).Clicked() {
					day := p.selected
					if day.Before(DayOf(time.Now())) {
						day = time.Time{}
					}
					ShowScheduleEditor(nil, day)
				}
			},
			FABInset: BottomInset,
			Body: func() {
				ui.Column(c).Fill().Children(func() {
					header()
					if wide {
						ui.Row(c).FillWidth().Grow(1).Basis(0).AlignItems(ui.Start).Children(func() {
							ui.Box(c).Width(340+design.SpaceXL+design.SpaceMD).Shrink(0).Padding(design.SpaceSM, design.SpaceMD, design.SpaceXL, design.SpaceXL).Children(calendar)
							ui.Box(c).Grow(1).Basis(0).FillHeight().Children(agenda)
						})
						return
					}
					ui.Box(c).FillWidth().PaddingX(design.SpaceMD).Children(calendar)
					ui.Box(c).FillWidth().Grow(1).Basis(0).Children(agenda)
				})
			},
		}.Show(c)
	})
}

func (p *SchedulePage) shift(delta int, collapsed bool) {
	p.dir = delta
	if collapsed {
		p.selectDay(p.selected.AddDate(0, 0, 7*delta))
		return
	}
	p.month = p.month.AddDate(0, delta, 0)
}

func (p *SchedulePage) calendar(c *ui.Context, collapsed, toggle bool) {
	cs := design.S()
	tt := design.TT()
	m := p.month
	if collapsed {
		m = time.Date(p.selected.Year(), p.selected.Month(), 1, 0, 0, 0, 0, time.Local)
	}
	today := DayOf(time.Now())
	showToday := !p.selected.Equal(today) || !m.Equal(time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.Local))
	var days []time.Time
	if collapsed {
		start := p.selected.AddDate(0, 0, -(core.Weekday(p.selected) - 1))
		for i := range 7 {
			days = append(days, start.AddDate(0, 0, i))
		}
	} else {
		start := m.AddDate(0, 0, -(core.Weekday(m) - 1))
		for i := range 42 {
			days = append(days, start.AddDate(0, 0, i))
		}
	}
	ui.Column(c).FillWidth().Padding(design.SpaceSM, design.SpaceSM, design.SpaceXS, design.SpaceSM).Radius(design.CornerXL).Background(cs.SurfaceContainerLow).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			m3.HGap(c, design.SpaceSM)
			m3.Text(c, l10n.DateYearMonth(m.Year(), int(m.Month())), tt.TitleMedium)
			if showToday {
				m3.HGap(c, design.SpaceXS)
				if m3.TextButton(c, l10n.CommonToday).Clicked() {
					p.selectDay(today)
				}
			}
			ui.Box(c).Grow(1)
			prev, next := l10n.SchedulePrevMonth, l10n.ScheduleNextMonth
			if collapsed {
				prev, next = l10n.SchedulePrevWeek, l10n.ScheduleNextWeek
			}
			if m3.IconBtn(c, icons.ChevronLeftRounded, prev).Clicked() {
				p.shift(-1, collapsed)
			}
			if m3.IconBtn(c, icons.ChevronRightRounded, next).Clicked() {
				p.shift(1, collapsed)
			}
		})
		ui.Grid(c).FillWidth().Columns(7).Children(func() {
			for _, w := range core.Weekdays {
				ui.Box(c).Center().Children(func() { m3.Text(c, w, tt.LabelSmall.C(cs.OnSurfaceVariant)) })
			}
		})
		m3.VGap(c, design.SpaceXS)
		anim := design.T().Motion(design.Medium4, design.Emphasized)
		holder := ui.Box(c).FillWidth().Clip().Transition(ui.ElementTransition{Duration: anim.Duration, Ease: design.Emphasized, Height: true})
		cellW := widgets.Width(holder, 320) / 7
		holder.Children(func() {
			key := strconv.FormatBool(collapsed) + IsoDate(days[0])
			ui.Grid(c.Key(key)).FillWidth().Columns(7).
				Transition(design.Switcher(design.Medium2, design.EmphasizedDecelerate, nil, ui.Motion{Opacity: 0, XPercent: 8 * float32(p.dir)})).Children(func() {
				for _, d := range days {
					p.dayCell(c, d, collapsed || d.Month() == m.Month(), d.Equal(today), d.Equal(p.selected), cellW/.92)
				}
			})
		})
		if toggle {
			tip := l10n.ScheduleCollapseMonth
			if collapsed {
				tip = l10n.ScheduleExpandMonth
			}
			t := ui.ButtonBase(c).FocusRing(false).FillWidth().Height(28).Center().Cursor(ui.CursorPointer).Label(tip)
			m3.Tip(c, t, tip)
			turn := t.AnimateTo("turn", map[bool]float32{true: 0, false: 180}[collapsed], design.T().Motion(design.Medium2, nil))
			t.Children(func() { m3.Icon(c, icons.ExpandMoreRounded, 24, cs.OnSurfaceVariant).Rotate(turn) })
			if t.Clicked() {
				p.monthOpen = !p.monthOpen
			}
		}
	})
}

func (p *SchedulePage) dayCell(c *ui.Context, day time.Time, inMonth, today, selected bool, height float32) {
	cs := design.S()
	tt := design.TT()
	items := Schedule.OnDay(day)
	fill, fg := ui.Transparent, cs.OnSurface
	switch {
	case today:
		fill, fg = cs.Primary, cs.OnPrimary
	case selected:
		fill, fg = cs.SecondaryContainer, cs.OnSecondaryContainer
	case !inMonth:
		fg = design.WithOpacity(cs.OnSurface, .38)
	}
	count := ""
	if len(items) > 0 {
		count = l10n.ScheduleDayItems(len(items))
	}
	cell := ui.ButtonBase(c.Key(IsoDate(day))).FocusRing(false).FillWidth().Height(height).Cursor(ui.CursorPointer).
		Column().AlignItems(ui.Center).Justify(ui.Center).Label(l10n.ScheduleDayLabel(int(day.Month()), day.Day(), count)).Checked(selected)
	cell.Children(func() {
		anim := design.T().Motion(design.Medium1, design.Emphasized)
		circle := ui.Box(c).Size(34, 34).Radius(17).Center().Background(fill).
			Transition(ui.ElementTransition{Duration: anim.Duration, Ease: design.Emphasized, Colors: true})
		m3.Ink(circle, cs.OnSurface, 17)
		if selected && today {
			ring := cs.SecondaryContainer
			circle = circle.DrawOver(func(p *ui.Painter, r ui.Rect) {
				p.Stroke(ui.Rect{X: r.X - 1, Y: r.Y - 1, W: r.W + 2, H: r.H + 2}, ring, 18, 2)
			})
		}
		st := tt.BodyMedium.C(fg)
		if today || selected {
			st = st.W(600)
		}
		circle.Children(func() { m3.Text(c, strconv.Itoa(day.Day()), st) })
		m3.VGap(c, 3)
		ui.Row(c).Height(5).Children(func() {
			for i, it := range items {
				if i == 3 {
					break
				}
				col := KindStyleOf(it.Kind()).Accent
				switch {
				case it.Done():
					col = cs.OutlineVariant
				case it.Overdue():
					col = cs.Error
				}
				ui.Box(c).Size(5, 5).MarginX(1).Radius(2.5).Background(col)
			}
		})
	})
	if cell.Clicked() {
		p.selectDay(day)
	}
}

func (p *SchedulePage) agenda(c *ui.Context) {
	cs := design.S()
	tt := design.TT()
	today := IsoDate(DayOf(time.Now()))
	var overdue []core.ScheduleItem
	byDay := map[string][]core.ScheduleItem{}
	for _, i := range Schedule.Items {
		if !p.all && i.Overdue() {
			overdue = append(overdue, i)
		}
		if p.all || (!i.Done() && i.DueDate() >= today) {
			byDay[i.DueDate()] = append(byDay[i.DueDate()], i)
		}
	}
	// The selected day always has a row, so that an empty day offers to add.
	sel := IsoDate(p.selected)
	if _, ok := byDay[sel]; !ok {
		byDay[sel] = nil
	}
	var days []string
	for k := range byDay {
		days = append(days, k)
	}
	sortStrings(days)
	reveal := p.reveal
	p.reveal = false
	ui.Scroll(c).Fill().TrackScroll(&p.scroll).Children(func() {
		ui.Column(c).FillWidth().Padding(0, design.SpaceLG, 96+BottomInset, design.SpaceMD).Children(func() {
			if len(overdue) > 0 {
				ui.Row(c).AlignItems(ui.Center).Padding(design.SpaceSM, 0, design.SpaceSM, design.SpaceSM).Children(func() {
					m3.Icon(c, icons.ErrorOutlineRounded, 18, cs.Error)
					m3.HGap(c, design.SpaceSM)
					m3.Text(c, l10n.ScheduleOverdueHeader, tt.TitleSmall.C(cs.Error))
				})
				for _, i := range overdue {
					ui.Box(c.Key("o"+i.ID())).FillWidth().Padding(0, 0, design.SpaceSM, 0).Children(func() { ScheduleTile(c, i, true) })
				}
				m3.VGap(c, design.SpaceMD)
			}
			lastMonth := ""
			now := time.Now()
			for _, k := range days {
				d := core.ParseDay(k)
				month := strconv.Itoa(d.Year()) + "-" + strconv.Itoa(int(d.Month()))
				if month != lastMonth {
					lastMonth = month
					label := l10n.DateYearMonth(d.Year(), int(d.Month()))
					if d.Year() == now.Year() {
						label = l10n.DateMonth(int(d.Month()))
					}
					m3.Text(c.Key("m"+month), label, tt.TitleSmall.C(cs.Primary)).Padding(design.SpaceLG, 0, design.SpaceSM, design.SpaceSM)
				}
				row := ui.Box(c.Key("d" + k)).FillWidth()
				if reveal && k == sel {
					row.ScrollIntoView(ui.ScrollIntoViewOptions{Block: ui.ScrollStart, Animation: &ui.Animation{Duration: design.Long2, Ease: design.Emphasized}})
				}
				row.Children(func() { p.dayRow(c, d, byDay[k], k == today, k == sel) })
			}
		})
	})
}

func (p *SchedulePage) dayRow(c *ui.Context, day time.Time, items []core.ScheduleItem, today, selected bool) {
	cs := design.S()
	tt := design.TT()
	ui.Row(c).FillWidth().AlignItems(ui.Start).Padding(0, 0, design.SpaceMD, 0).Children(func() {
		b := m3.Tappable(c, design.CornerLG).Width(56).Shrink(0)
		b.Children(func() {
			ui.Column(c).FillWidth().AlignItems(ui.Center).Children(func() {
				m3.VGap(c, design.SpaceXS)
				wc := cs.OnSurfaceVariant
				if today {
					wc = cs.Primary
				}
				m3.Text(c, weekdayLabel(day), tt.LabelMedium.C(wc))
				m3.VGap(c, 2)
				fill, fg := ui.Transparent, cs.OnSurface
				switch {
				case today:
					fill, fg = cs.Primary, cs.OnPrimary
				case selected:
					fill, fg = cs.SecondaryContainer, cs.OnSecondaryContainer
				}
				ui.Box(c).Size(40, 40).Radius(20).Center().Background(fill).
					Transition(ui.ElementTransition{Duration: design.T().Motion(design.Medium1, nil).Duration, Ease: ui.Linear, Colors: true}).
					Children(func() { m3.Text(c, strconv.Itoa(day.Day()), tt.TitleLarge.C(fg)) })
			})
		})
		if b.Clicked() {
			p.selectDay(day)
		}
		m3.HGap(c, design.SpaceSM)
		ui.Column(c).Grow(1).Basis(0).Children(func() {
			if len(items) == 0 {
				ui.Row(c).FillWidth().Height(56).Margin(design.SpaceXS, 0, 0, 0).PaddingX(design.SpaceMD).AlignItems(ui.Center).
					Border(1, cs.OutlineVariant).Radius(design.CornerLG).Children(func() {
					m3.Text(c, l10n.ScheduleDayEmpty, tt.BodyMedium.C(cs.OnSurfaceVariant)).Grow(1).Basis(0)
					if (m3.Button{Kind: m3.TextKind, Icon: icons.AddRounded, Label: l10n.CommonAdd}).Show(c).Clicked() {
						ShowScheduleEditor(nil, day)
					}
				})
				return
			}
			for _, i := range items {
				ui.Box(c.Key(i.ID())).FillWidth().Padding(0, 0, design.SpaceSM, 0).Children(func() { ScheduleTile(c, i, false) })
			}
		})
	})
}

// UpcomingSchedule is the compact list of what is coming, for the home
// page.
func UpcomingSchedule(c *ui.Context, onOpen func(time.Time)) {
	items := Schedule.Upcoming(14)
	if len(items) > 4 {
		items = items[:4]
	}
	if len(items) == 0 {
		return
	}
	cs := design.S()
	tt := design.TT()
	g := ui.Grid(c).FillWidth().Gap(design.SpaceMD)
	cols := min(max(int(widgets.Width(g, 800)/260), 1), 4)
	n := cols
	if cols == 1 {
		n = 3
	}
	if n < len(items) {
		items = items[:n]
	}
	g.Columns(cols).Children(func() {
		for _, it := range items {
			tile := m3.Tappable(c.Key(it.ID()), design.CornerLG).Background(cs.SurfaceContainer).Clip()
			tile.Children(func() {
				ui.Row(c).FillWidth().Padding(design.SpaceMD).AlignItems(ui.Center).Children(func() {
					DateLeaf(c, it.Day(), KindStyleOf(it.Kind()), 44, false)
					m3.HGap(c, design.SpaceMD)
					ui.Column(c).Grow(1).Basis(0).Children(func() {
						m3.Clamp(m3.Text(c, it.Title(), tt.TitleSmall), 1)
						m3.VGap(c, 2)
						parts := []string{fmtCountdown(it)}
						course := it.CourseTitle()
						if course == "" {
							if co, ok := State.Course(it.CourseID()); ok {
								course = co.Title()
							}
						}
						if course != "" {
							parts = append(parts, course)
						}
						col := cs.OnSurfaceVariant
						if it.Overdue() {
							col = cs.Error
						}
						m3.Clamp(m3.Text(c, strings.Join(parts, " · "), tt.BodySmall.C(col)), 1)
					})
				})
			})
			if tile.Clicked() {
				onOpen(it.Day())
			}
		}
	})
}
