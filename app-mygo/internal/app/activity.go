package app

import (
	"errors"
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

// activityKinds are the timed kinds in display order.
var activityKinds = []string{"watch", "read", "practice"}

func activityKindLabel(k string) string {
	switch k {
	case "watch":
		return l10n.ActivityWatching
	case "read":
		return l10n.ActivityReading
	case "practice":
		return l10n.CommonPractice
	}
	return k
}

// ActivityDay is one day of GET /api/activity.
type ActivityDay struct{ core.J }

func (d ActivityDay) Minutes() int      { return d.I("minutes") }
func (d ActivityDay) Asks() int         { return d.I("asks") }
func (d ActivityDay) NotesCreated() int { return d.I("notes_created") }
func (d ActivityDay) NotesEdited() int  { return d.I("notes_edited") }
func (d ActivityDay) Transcripts() int  { return d.I("transcripts") }
func (d ActivityDay) Quizzes() int      { return d.I("quizzes") }

// KindMinutes are the minutes of a timed kind.
func (d ActivityDay) KindMinutes(k string) int {
	return int(math.Round(float64(d.O("by_kind").I(k)) / 60))
}

// Active is the daemon's ActivityDay.Active: study time or any event.
func (d ActivityDay) Active() bool {
	return d.Minutes() >= 1 || d.Asks() > 0 || d.NotesCreated() > 0 || d.NotesEdited() > 0 || d.Transcripts() > 0 || d.Quizzes() > 0
}

// Intensity is the heatmap's value: minutes, with days of only events at
// the lowest step so that they match the streak.
func (d ActivityDay) Intensity() float64 {
	if d.Minutes() > 0 {
		return float64(d.Minutes())
	}
	if d.Active() {
		return .5
	}
	return 0
}

// Tooltip is the tooltip's title and lines.
func (d ActivityDay) Tooltip(day time.Time) (string, []string) {
	var lines []string
	switch {
	case !d.Active():
		lines = append(lines, l10n.ActivityNone)
	case d.Minutes() > 0:
		lines = append(lines, l10n.ActivityStudyTime(widgets.FmtMinutes(float64(d.Minutes()))))
	}
	if d.Minutes() > 0 {
		var parts []string
		for _, k := range activityKinds {
			if m := d.KindMinutes(k); m > 0 {
				parts = append(parts, activityKindLabel(k)+" "+strconv.Itoa(m))
			}
		}
		lines = append(lines, strings.Join(parts, " · "))
	}
	notes := d.NotesCreated() + d.NotesEdited()
	if d.Asks() > 0 || notes > 0 || d.Quizzes() > 0 {
		var parts []string
		if d.Quizzes() > 0 {
			parts = append(parts, l10n.ActivityQuizzes(d.Quizzes()))
		}
		if d.Asks() > 0 {
			parts = append(parts, l10n.ActivityAsks(d.Asks()))
		}
		if notes > 0 {
			parts = append(parts, l10n.ActivityNotes(notes))
		}
		lines = append(lines, strings.Join(parts, " · "))
	}
	return core.FmtDayLabel(day), lines
}

// Describe is the day for a screen reader.
func (d ActivityDay) Describe(day time.Time) string {
	what := l10n.ActivityNoStudy
	switch {
	case d.Minutes() > 0:
		what = l10n.ActivityStudyTime(widgets.FmtMinutes(float64(d.Minutes())))
	case d.Active():
		what = l10n.ActivityHasRecord
	}
	return l10n.ActivityDayLabel(widgets.FmtMonthDay(day), what)
}

// ActivityData is GET /api/activity for a range.
type ActivityData struct {
	Days                                 map[string]ActivityDay
	Current, Longest, Minutes, ActiveDay int
	Courses                              []core.J
}

func newActivityData(j core.J) *ActivityData {
	d := &ActivityData{Days: map[string]ActivityDay{}, Courses: j.Os("courses")}
	for _, day := range j.Os("days") {
		d.Days[day.S("date")] = ActivityDay{day}
	}
	d.Current, d.Longest = j.O("streak").I("current"), j.O("streak").I("longest")
	d.Minutes, d.ActiveDay = j.O("totals").I("minutes"), j.O("totals").I("active_days")
	return d
}

// On is the day d.
func (a *ActivityData) On(d time.Time) ActivityDay {
	if day, ok := a.Days[core.FmtIsoDay(d)]; ok {
		return day
	}
	return ActivityDay{core.J{}}
}

// MinutesSince sums the minutes from from to to.
func (a *ActivityData) MinutesSince(from, to time.Time) int {
	m := 0
	for d := widgets.DayOf(from); !d.After(to); d = d.AddDate(0, 0, 1) {
		m += a.On(d).Minutes()
	}
	return m
}

// Empty is whether no day of the range is active.
func (a *ActivityData) Empty() bool {
	for _, d := range a.Days {
		if d.Active() {
			return false
		}
	}
	return true
}

// liveActivity reloads when the daemon reports new activity or notes, at
// most every 2 s.
type liveActivity struct {
	cancel   func()
	debounce *time.Timer
}

func (l *liveActivity) listen(reload func()) {
	l.cancel = State.Events.Listen(func(e core.J) {
		if t := e.S("type"); t == "activity_changed" || t == "notes_changed" {
			if l.debounce != nil {
				l.debounce.Stop()
			}
			l.debounce = time.AfterFunc(2*time.Second, func() { core.Post(reload) })
		}
	})
}

func (l *liveActivity) Dispose() {
	if l.cancel != nil {
		l.cancel()
	}
	if l.debounce != nil {
		l.debounce.Stop()
	}
}

func fetchActivity(path string, q map[string]string, done func(core.J, error)) {
	api := State.Api()
	core.Async(func() (core.J, error) {
		v, err := api.Get(path, q)
		return core.Obj(v), err
	}, done)
}

// ActivityCard is the home card 学习足迹: as many recent weeks as fit, with
// this week's time and the streaks.
type ActivityCard struct {
	liveActivity
	data  *ActivityData
	today time.Time
}

// NewActivityCard loads the last year and follows the daemon's changes.
func NewActivityCard() *ActivityCard {
	a := &ActivityCard{}
	a.reload()
	a.listen(a.reload)
	return a
}

func (a *ActivityCard) reload() {
	a.today = widgets.DayOf(time.Now())
	from := widgets.WeekStart(a.today).AddDate(0, 0, -52*7)
	fetchActivity("/api/activity", map[string]string{"from": core.FmtIsoDay(from), "to": core.FmtIsoDay(a.today)}, func(j core.J, err error) {
		if err == nil {
			a.data = newActivityData(j)
		}
	})
}

// Show builds the card.
func (a *ActivityCard) Show(c *ui.Context) {
	cs := design.S()
	tt := design.TT()
	d := a.data
	box := ui.Column(c).FillWidth().Padding(design.SpaceLG, design.SpaceXL, design.SpaceLG, design.SpaceXL).
		Radius(design.CornerXL).Background(cs.SurfaceContainerLow)
	inner := widgets.Width(box, 600) - 2*design.SpaceXL
	box.Children(func() {
		weeks := widgets.HeatWeeksFor(inner, 12)
		from := widgets.WeekStart(a.today).AddDate(0, 0, -(weeks-1)*7)
		ui.Row(c).Wrap().GapX(design.SpaceXXL).GapY(design.SpaceMD).Children(func() {
			dash := func(s string) string {
				if d == nil {
					return "—"
				}
				return s
			}
			var week, cur, long string
			if d != nil {
				week = widgets.FmtMinutes(float64(d.MinutesSince(widgets.WeekStart(a.today), a.today)))
				cur, long = l10n.CommonDays(d.Current), l10n.ActivityLongestDays(d.Longest)
			}
			activityFigure(c, dash(week), l10n.ActivityThisWeek)
			activityFigure(c, dash(cur), l10n.ActivityStreak)
			activityFigure(c, dash(long), l10n.ActivityLongestStreak)
		})
		m3.VGap(c, design.SpaceLG)
		on := func(day time.Time) ActivityDay {
			if d == nil {
				return ActivityDay{core.J{}}
			}
			return d.On(day)
		}
		widgets.Heatmap{
			From: from, To: a.today, Today: a.today,
			Value:    func(day time.Time) float64 { return on(day).Intensity() },
			Describe: func(day time.Time) string { return on(day).Describe(day) },
			Tooltip:  func(day time.Time) (string, []string) { return on(day).Tooltip(day) },
			OnTap:    ShowActivityDay,
		}.Show(c)
		m3.VGap(c, design.SpaceMD)
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Box(c).Grow(1).Basis(0).Children(func() {
				if d != nil && d.Empty() {
					m3.Text(c, l10n.ActivityEmpty, tt.BodySmall.C(cs.OnSurfaceVariant))
				}
			})
			widgets.HeatLegend(c, 12)
		})
	})
}

func activityFigure(c *ui.Context, value, label string) {
	cs := design.S()
	tt := design.TT()
	ui.Column(c).Label(label + " " + value).Children(func() {
		m3.Text(c, label, tt.LabelMedium.C(cs.OnSurfaceVariant))
		m3.VGap(c, 2)
		m3.Text(c, value, tt.TitleLarge).FontFeatures("tnum")
	})
}

// ActivityPage is 学习统计: a year of activity with kind and course totals.
type ActivityPage struct {
	liveActivity
	today   time.Time
	year    int
	data    *ActivityData
	err     error
	loading bool
	scroll  ui.ScrollState
	heat    ui.ScrollState
	refresh ui.RefreshState
	// heatPlaced is set once the year's heatmap was scrolled to its start.
	heatPlaced bool
}

// NewActivityPage shows this year.
func NewActivityPage() *ActivityPage {
	p := &ActivityPage{today: widgets.DayOf(time.Now())}
	p.year = p.today.Year()
	p.reload()
	p.listen(p.reload)
	return p
}

func (p *ActivityPage) reload() {
	p.loading = true
	year := p.year
	y := strconv.Itoa(year)
	fetchActivity("/api/activity", map[string]string{"from": y + "-01-01", "to": y + "-12-31"}, func(j core.J, err error) {
		if year != p.year {
			return
		}
		p.loading, p.refresh.Refreshing = false, false
		if err != nil {
			p.err = err
			return
		}
		p.data, p.err = newActivityData(j), nil
	})
}

func (p *ActivityPage) setYear(y int) {
	p.year, p.data, p.heatPlaced = y, nil, false
	p.heat = ui.ScrollState{}
	p.reload()
}

// View builds the page.
func (p *ActivityPage) View(c *ui.Context) {
	tt := design.TT()
	m3.Scaffold{
		AppBar: &m3.AppBar{Title: l10n.ActivityStatsTitle, Scrolled: p.scroll.Y > 0, Actions: func() {
			if m3.IconBtn(c, icons.ChevronLeftRounded, l10n.ActivityPrevYear).Clicked() {
				p.setYear(p.year - 1)
			}
			m3.Text(c, strconv.Itoa(p.year), tt.TitleMedium).FontFeatures("tnum")
			if (m3.IconButton{Icon: icons.ChevronRightRounded, Tooltip: l10n.ActivityNextYear, Disabled: p.year >= p.today.Year()}).Show(c).Clicked() {
				p.setYear(p.year + 1)
			}
			m3.HGap(c, design.SpaceSM)
		}},
		Body: func() { p.body(c) },
	}.Show(c)
}

func (p *ActivityPage) body(c *ui.Context) {
	tt := design.TT()
	d := p.data
	if d == nil && p.loading {
		widgets.CenteredLoader(c, "")
		return
	}
	if d == nil {
		err := p.err
		if err == nil {
			err = errors.New(l10n.CommonNoData)
		}
		widgets.ErrorState(c, err, p.reload)
		return
	}
	w, _ := c.Size()
	pad := design.Page
	if design.SizeClassOf(w) == design.SizeCompact {
		pad = design.PageCompact
	}
	sc := ui.Scroll(c).Fill().TrackScroll(&p.scroll)
	if sc.PullToRefresh(&p.refresh) {
		p.reload()
	}
	sc.Children(func() {
		widgets.ContentWidth(c, 0, func() {
			ui.Column(c).FillWidth().Padding(design.SpaceSM, pad, design.SpaceXXXL, pad).Children(func() {
				widgets.StatGrid(c,
					widgets.StatTile{Icon: icons.ScheduleRounded, Label: l10n.ActivityDuration, Value: widgets.FmtMinutes(float64(d.Minutes))},
					widgets.StatTile{Icon: icons.EventAvailableOutlined, Label: l10n.ActivityDaysCount, Value: l10n.ActivityActiveDays(d.ActiveDay)},
					widgets.StatTile{Icon: icons.LocalFireDepartmentOutlined, Label: l10n.ActivityCurrentStreak, Value: l10n.CommonDays(d.Current)},
					widgets.StatTile{Icon: icons.EmojiEventsOutlined, Label: l10n.ActivityLongestStreak, Value: l10n.ActivityLongestDays(d.Longest)},
				)
				m3.VGap(c, design.SpaceLG)
				activitySection(c, l10n.ActivityYear(p.year), func() {
					from := time.Date(p.year, 1, 1, 0, 0, 0, 0, time.Local)
					to := time.Date(p.year, 12, 31, 0, 0, 0, 0, time.Local)
					cols := float32(len(widgets.HeatWeeks(from, to)))
					holder := ui.Box(c).FillWidth()
					width := widgets.Width(holder, 700)
					cell := float32(math.Floor(float64(min(max((width-widgets.HeatLabelWidth+widgets.HeatGap)/cols-widgets.HeatGap, 9), 13))))
					holder.Children(func() {
						if !p.heatPlaced && p.heat.MaxX > 0 {
							// reverse: this year opens at today's end.
							p.heatPlaced = true
							if p.year == p.today.Year() {
								p.heat.ScrollTo(p.heat.MaxX, 0, ui.Animation{})
							}
						}
						widgets.HScroller{State: &p.heat, Buttons: true}.Show(c, func() {
							widgets.Heatmap{
								From: from, To: to, Today: p.today, Cell: cell,
								Value:    func(day time.Time) float64 { return d.On(day).Intensity() },
								Describe: func(day time.Time) string { return d.On(day).Describe(day) },
								Tooltip:  func(day time.Time) (string, []string) { return d.On(day).Tooltip(day) },
								OnTap:    ShowActivityDay,
							}.Show(c)
						})
					})
					m3.VGap(c, design.SpaceMD)
					ui.Row(c).FillWidth().Justify(ui.End).Children(func() { widgets.HeatLegend(c, 12) })
				})
				m3.VGap(c, design.SpaceLG)
				activitySection(c, l10n.ActivityByKind, func() { p.kinds(c, d) })
				m3.VGap(c, design.SpaceLG)
				activitySection(c, l10n.ActivityByCourse, func() {
					if len(d.Courses) == 0 {
						m3.Text(c, l10n.ActivityByCourseEmpty, tt.BodyMedium).PaddingY(design.SpaceMD)
						return
					}
					top := max(1, d.Courses[0].I("minutes"))
					for _, co := range d.Courses {
						label := co.S("title")
						if label == "" {
							label = l10n.CommonUncategorized
						}
						id := co.S("course_id")
						if (widgets.RankedRow{Label: label, Fraction: float64(co.I("minutes")) / float64(top),
							Trailing: widgets.FmtMinutes(float64(co.I("minutes"))), Tap: id != ""}).Show(c.Key(id + label)) {
							m3.Nav().Push(NewCoursePage(id))
						}
					}
				})
			})
		})
	})
}

func (p *ActivityPage) kinds(c *ui.Context, d *ActivityData) {
	cs := design.S()
	tt := design.TT()
	totals := map[string]int{}
	top := 1
	asks, notes, quizzes := 0, 0, 0
	for _, day := range d.Days {
		for _, k := range activityKinds {
			totals[k] += day.O("by_kind").I(k)
		}
		asks += day.Asks()
		notes += day.NotesCreated()
		quizzes += day.Quizzes()
	}
	for _, v := range totals {
		top = max(top, v)
	}
	for _, k := range activityKinds {
		widgets.RankedRow{Label: activityKindLabel(k), Fraction: float64(totals[k]) / float64(top),
			Trailing: widgets.FmtMinutes(float64(totals[k]) / 60)}.Show(c.Key(k))
	}
	m3.Text(c, l10n.ActivityDaySummary(quizzes, asks, notes), tt.BodySmall.C(cs.OnSurfaceVariant)).Margin(design.SpaceSM, 0, 0, 0)
}

func activitySection(c *ui.Context, title string, child func()) {
	cs := design.S()
	ui.Column(c).FillWidth().Padding(design.SpaceMD, design.SpaceLG, design.SpaceLG, design.SpaceLG).
		Radius(design.CornerXL).Background(cs.SurfaceContainerLow).Children(func() {
		ui.Row(c).FillWidth().Height(40).AlignItems(ui.Center).Children(func() {
			m3.Text(c, title, design.TT().TitleMedium)
		})
		m3.VGap(c, design.SpaceSM)
		child()
	})
}

// ShowActivityDay is showActivityDay: a sheet of what happened on day.
func ShowActivityDay(day time.Time) {
	var (
		data   core.J
		err    error
		loaded bool
	)
	fetchActivity("/api/activity/day", map[string]string{"date": core.FmtIsoDay(day)}, func(j core.J, e error) {
		data, err, loaded = j, e, true
	})
	m3.ShowModalBottomSheet(m3.SheetOptions{ScrollControlled: true}, func(c *ui.Context, r *m3.Route) {
		_, h := c.Size()
		ui.Column(c).FillWidth().MaxHeight(h * .75).Children(func() {
			switch {
			case err != nil:
				ui.Box(c).FillWidth().Padding(design.SpaceXL).Children(func() { widgets.ErrorState(c, err, nil) })
			case !loaded:
				ui.Box(c).FillWidth().Height(200).Children(func() { widgets.CenteredLoader(c, "") })
			default:
				activityDaySheet(c, r, day, data)
			}
		})
	})
}

func activityDaySheet(c *ui.Context, r *m3.Route, day time.Time, j core.J) {
	cs := design.S()
	tt := design.TT()
	d := ActivityDay{j}
	lectures := activityByLecture(j.Os("lectures"))
	notes, transcribed, attempts := j.Os("notes"), j.Os("transcribed"), j.Os("attempts")
	var counts []string
	if d.Quizzes() > 0 {
		counts = append(counts, l10n.ActivityQuizzesDone(d.Quizzes()))
	}
	if d.Asks() > 0 {
		counts = append(counts, l10n.ActivityAsks(d.Asks()))
	}
	if d.NotesCreated() > 0 {
		counts = append(counts, l10n.ActivityNotesCreated(d.NotesCreated()))
	}
	if d.NotesEdited() > 0 {
		counts = append(counts, l10n.ActivityNotesEdited(d.NotesEdited()))
	}
	if d.Transcripts() > 0 {
		counts = append(counts, l10n.ActivityTranscripts(d.Transcripts()))
	}
	open := func(p m3.Page) {
		r.Close()
		m3.RootNav.Push(p)
	}
	sub := func(t string) {
		m3.Text(c, t, tt.TitleSmall.C(cs.Primary)).Padding(design.SpaceXL, design.SpaceXL, design.SpaceXS, design.SpaceXL)
	}
	ui.Scroll(c).FillWidth().Shrink(1).Children(func() {
		ui.Column(c).FillWidth().Padding(0, 0, design.SpaceXL, 0).Children(func() {
			ui.Column(c).FillWidth().PaddingX(design.SpaceXL).Children(func() {
				m3.Text(c, core.FmtDayLabel(day), tt.TitleLarge)
				m3.VGap(c, design.SpaceXS)
				line := l10n.ActivityDayEmpty
				switch {
				case d.Minutes() > 0:
					line = l10n.ActivityStudyTime(widgets.FmtMinutes(float64(d.Minutes())))
				case d.Active():
					line = l10n.ActivityUntimed
				}
				m3.Text(c, line, tt.BodyMedium.C(cs.OnSurfaceVariant))
				if d.Minutes() > 0 {
					m3.VGap(c, design.SpaceLG)
					ui.Row(c).Wrap().GapX(design.SpaceXXL).GapY(design.SpaceMD).Children(func() {
						for _, k := range activityKinds {
							activityFigure(c, widgets.FmtMinutes(float64(d.KindMinutes(k))), activityKindLabel(k))
						}
					})
				}
				if len(counts) > 0 {
					m3.VGap(c, design.SpaceLG)
					m3.Text(c, strings.Join(counts, " · "), tt.BodyMedium)
				}
			})
			if len(lectures) > 0 {
				sub(l10n.CommonCourses)
				for _, l := range lectures {
					var parts []string
					if t := l.S("course_title"); t != "" {
						parts = append(parts, t)
					}
					parts = append(parts, l.S("detail"))
					if (m3.Tile{LeadingIcon: icons.PlayCircleOutlineRounded, Title: l.S("title"), TitleLines: 1,
						Subtitle: strings.Join(parts, " · "), SubtitleLines: 1, Tap: true}).Show(c.Key("l" + l.S("lecture_id"))).Clicked() {
						open(NewPlayerPage(l.S("lecture_id"), -1))
					}
				}
			}
			if len(attempts) > 0 {
				sub(l10n.CommonPractice)
				for i, a := range attempts {
					trailing := a.S("correct") + "/" + a.S("total")
					if (m3.Tile{LeadingIcon: icons.QuizOutlined, Title: a.S("title"), TitleLines: 1, Tap: true,
						Trailing: func() { m3.Text(c, trailing, tt.TitleSmall).FontFeatures("tnum") }}).Show(c.Key("a" + strconv.Itoa(i))).Clicked() {
						open(NewPracticePage(a.S("quiz_id")))
					}
				}
			}
			if len(notes) > 0 {
				sub(l10n.CommonNotes)
				for _, n := range notes {
					what := l10n.CommonEdit
					if n["created"] == true {
						what = l10n.CommonNew
					}
					if (m3.Tile{LeadingIcon: icons.DescriptionOutlined, Title: n.S("title"), TitleLines: 1, Subtitle: what, Tap: true}).Show(c.Key("n" + n.S("id"))).Clicked() {
						open(NewNotePage(n.S("id")))
					}
				}
			}
			if len(transcribed) > 0 {
				sub(l10n.ActivityTranscriptsHeader)
				for _, t := range transcribed {
					if (m3.Tile{LeadingIcon: icons.SubjectRounded, Title: t.S("title"), TitleLines: 1, Subtitle: t.S("course_title"), Tap: true}).Show(c.Key("t" + t.S("lecture_id"))).Clicked() {
						open(NewPlayerPage(t.S("lecture_id"), -1))
					}
				}
			}
		})
	})
}

// activityByLecture merges the rows of each kind into one a lecture:
// "看课 32 分钟 · 阅读 5 分钟".
func activityByLecture(rows []core.J) []core.J {
	var order []string
	out := map[string]core.J{}
	secs := map[string]map[string]int{}
	for _, r := range rows {
		id := r.S("lecture_id")
		if _, ok := out[id]; !ok {
			out[id] = core.J{"lecture_id": id, "title": r["title"], "course_title": r["course_title"]}
			secs[id] = map[string]int{}
			order = append(order, id)
		}
		secs[id][r.S("kind")] += r.I("seconds")
	}
	var list []core.J
	for _, id := range order {
		s := secs[id]
		var parts []string
		total := 0
		for _, v := range s {
			total += v
		}
		for _, k := range activityKinds {
			if s[k] >= 60 {
				parts = append(parts, activityKindLabel(k)+" "+widgets.FmtMinutes(float64(s[k])/60))
			}
		}
		out[id]["detail"] = strings.Join(parts, " · ")
		out[id]["total"] = total
		list = append(list, out[id])
	}
	sort.SliceStable(list, func(a, b int) bool { return list[a].I("total") > list[b].I("total") })
	return list
}
