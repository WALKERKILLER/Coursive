package m3

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
)

// Material's Chinese strings for the pickers (MaterialLocalizationZh).
const (
	datePickerHelp   = "选择日期"
	dateInputLabel   = "输入日期"
	dateHelpFormat   = "yyyy/mm/dd"
	dateInvalid      = "格式无效。"
	dateOutOfRange   = "超出范围。"
	inputModeLabel   = "切换到输入模式"
	calendarMode     = "切换到日历模式"
	timePickerHelp   = "选择时间"
	timeInputHelp    = "输入时间"
	timeHourLabel    = "小时"
	timeMinuteLabel  = "分钟"
	timeInputMode    = "切换到文本输入模式"
	timeDialMode     = "切换到表盘选择器模式"
	cancelLabel      = "取消"
	okLabel          = "确定"
	prevMonthTooltip = "上个月"
	nextMonthTooltip = "下个月"
	yearPickerLabel  = "选择年份"
)

var narrowWeekdays = []string{"日", "一", "二", "三", "四", "五", "六"}
var shortWeekdays = []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}

func midnight(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
}

// DatePicker is showDatePicker's options.
type DatePicker struct {
	Initial, First, Last time.Time
	// Help replaces 选择日期 (helpText).
	Help string
}

type datePickerState struct {
	selected, month time.Time
	years, input    bool
	text, err       string
	yearScroll      ui.ScrollState
	yearPlaced      bool
	dir             int
}

// ShowDatePicker is showDatePicker in Material 3: the calendar with a month
// header, the year grid, and the text input mode. done receives the chosen
// day, or nothing on cancel.
func ShowDatePicker(o DatePicker, done func(time.Time)) {
	if o.First.IsZero() {
		o.First = time.Date(1900, 1, 1, 0, 0, 0, 0, time.Local)
	}
	if o.Last.IsZero() {
		o.Last = time.Date(2100, 12, 31, 0, 0, 0, 0, time.Local)
	}
	init := midnight(o.Initial)
	if o.Initial.IsZero() {
		init = midnight(time.Now())
	}
	st := &datePickerState{selected: init, month: time.Date(init.Year(), init.Month(), 1, 0, 0, 0, 0, time.Local), dir: 1}
	help := o.Help
	if help == "" {
		help = datePickerHelp
	}
	ShowDialog(func(c *ui.Context, r *Route) { datePickerView(c, r, o, st, help) }, func(v any) {
		if d, ok := v.(time.Time); ok && done != nil {
			done(d)
		}
	})
}

func fmtMediumDate(d time.Time) string {
	return fmt.Sprintf("%d月%d日%s", int(d.Month()), d.Day(), shortWeekdays[int(d.Weekday())])
}

func fmtSlash(d time.Time) string {
	return fmt.Sprintf("%04d/%02d/%02d", d.Year(), int(d.Month()), d.Day())
}

func parseSlash(s string) (time.Time, bool) {
	parts := strings.FieldsFunc(strings.TrimSpace(s), func(r rune) bool { return r == '/' || r == '-' || r == '.' })
	if len(parts) != 3 {
		return time.Time{}, false
	}
	y, e1 := strconv.Atoi(parts[0])
	m, e2 := strconv.Atoi(parts[1])
	d, e3 := strconv.Atoi(parts[2])
	if e1 != nil || e2 != nil || e3 != nil || m < 1 || m > 12 || d < 1 || d > 31 {
		return time.Time{}, false
	}
	t := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.Local)
	if t.Day() != d {
		return time.Time{}, false
	}
	return t, true
}

func datePickerView(c *ui.Context, r *Route, o DatePicker, st *datePickerState, help string) {
	s := cs()
	t := tt()
	first, last := midnight(o.First), midnight(o.Last)
	ui.Column(c).Width(360).MaxWidth(360).Radius(design.CornerXL).Background(s.SurfaceContainerHigh).Clip().Children(func() {
		// Header: the help text, the chosen day and the mode switch.
		ui.Column(c).FillWidth().Padding(16, 12, 0, 24).Children(func() {
			Text(c, help, t.LabelLarge.C(s.OnSurfaceVariant))
			ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(20, 0, 8, 0).Children(func() {
				head := fmtMediumDate(st.selected)
				if st.input {
					head = dateInputLabel
				}
				Text(c, head, t.HeadlineLarge.C(s.OnSurfaceVariant)).Grow(1).Basis(0)
				ic, tip := icons.EditOutlined, inputModeLabel
				if st.input {
					ic, tip = icons.CalendarToday, calendarMode
				}
				if IconBtn(c, ic, tip).Clicked() {
					st.input = !st.input
					st.text, st.err = fmtSlash(st.selected), ""
				}
			})
		})
		ui.Box(c).FillWidth().Height(1).Background(s.OutlineVariant)
		if st.input {
			ui.Column(c).FillWidth().Padding(24, 24, 0, 24).Children(func() {
				in := Field{Label: dateInputLabel, Hint: dateHelpFormat, Error: st.err, AutoFocus: true}.Show(c, &st.text)
				if in.Submitted() {
					dateInputCommit(r, st, first, last)
				}
			})
		} else {
			datePickerCalendar(c, st, first, last)
		}
		ui.Row(c).FillWidth().Justify(ui.End).Gap(8).Padding(8, 12, 12, 12).Children(func() {
			if TextButton(c, cancelLabel).Clicked() {
				r.Close()
			}
			if TextButton(c, okLabel).Clicked() {
				if st.input {
					dateInputCommit(r, st, first, last)
				} else {
					r.Pop(st.selected)
				}
			}
		})
	})
}

func dateInputCommit(r *Route, st *datePickerState, first, last time.Time) {
	d, ok := parseSlash(st.text)
	switch {
	case !ok:
		st.err = dateInvalid
	case d.Before(first) || d.After(last):
		st.err = dateOutOfRange
	default:
		st.selected = d
		r.Pop(d)
	}
}

func datePickerCalendar(c *ui.Context, st *datePickerState, first, last time.Time) {
	s := cs()
	t := tt()
	today := midnight(time.Now())
	m := st.month
	// The month and year, opening the year grid, and the month arrows.
	ui.Row(c).FillWidth().Height(52).AlignItems(ui.Center).Padding(0, 4, 0, 16).Children(func() {
		yb := Tappable(c, design.CornerFull).Height(40).PaddingX(8)
		yb.Children(func() {
			ui.Row(c).AlignItems(ui.Center).FillHeight().Children(func() {
				Text(c, fmt.Sprintf("%d年%d月", m.Year(), int(m.Month())), t.TitleSmall.C(s.OnSurfaceVariant))
				turn := yb.AnimateTo("turn", map[bool]float32{true: 180, false: 0}[st.years], design.T().Motion(200*time.Millisecond, design.EaseInOut))
				Icon(c, icons.ArrowDropDown, 24, s.OnSurfaceVariant).Rotate(turn)
			})
		})
		yb.Label(yearPickerLabel)
		if yb.Clicked() {
			st.years = !st.years
			st.yearScroll, st.yearPlaced = ui.ScrollState{}, false
		}
		ui.Box(c).Grow(1)
		if !st.years {
			prev := m.AddDate(0, -1, 0)
			next := m.AddDate(0, 1, 0)
			if (IconButton{Icon: icons.ChevronLeft, Tooltip: prevMonthTooltip, Disabled: prev.AddDate(0, 1, -1).Before(first)}).Show(c).Clicked() {
				st.month, st.dir = prev, -1
			}
			if (IconButton{Icon: icons.ChevronRight, Tooltip: nextMonthTooltip, Disabled: next.After(last)}).Show(c).Clicked() {
				st.month, st.dir = next, 1
			}
		}
	})
	if st.years {
		years := ui.Scroll(c).FillWidth().Height(42*6 + 12).TrackScroll(&st.yearScroll)
		if st.yearScroll.MaxY > 0 && !st.yearPlaced {
			st.yearPlaced = true
			row := (m.Year() - first.Year()) / 3
			st.yearScroll.ScrollTo(0, max(0, float32(row)*52-2*52), ui.Animation{})
		}
		years.Children(func() {
			ui.Grid(c).FillWidth().Columns(3).GapY(0).Padding(0, 12).Children(func() {
				for y := first.Year(); y <= last.Year(); y++ {
					cell := ui.Box(c.Key(y)).Height(52).Center()
					cell.Children(func() {
						sel := y == m.Year()
						b := Tappable(c, design.CornerFull).Size(72, 36).Center()
						bg, fg := ui.Transparent, s.OnSurfaceVariant
						switch {
						case sel:
							bg, fg = s.Primary, s.OnPrimary
						case y == today.Year():
							fg = s.Primary
						}
						b = b.Background(bg)
						if y == today.Year() && !sel {
							b = b.Border(1, s.Primary)
						}
						b.Children(func() { Text(c, strconv.Itoa(y), t.BodyLarge.C(fg)) })
						if b.Clicked() {
							st.month = time.Date(y, m.Month(), 1, 0, 0, 0, 0, time.Local)
							if st.month.Before(time.Date(first.Year(), first.Month(), 1, 0, 0, 0, 0, time.Local)) {
								st.month = time.Date(first.Year(), first.Month(), 1, 0, 0, 0, 0, time.Local)
							}
							if st.month.After(last) {
								st.month = time.Date(last.Year(), last.Month(), 1, 0, 0, 0, 0, time.Local)
							}
							st.years = false
						}
					})
				}
			})
		})
		return
	}
	ui.Column(c).FillWidth().Padding(0, 12, 0, 12).Children(func() {
		ui.Grid(c).FillWidth().Columns(7).Children(func() {
			for _, w := range narrowWeekdays {
				ui.Box(c).Height(42).Center().Children(func() { Text(c, w, t.BodyLarge.C(s.OnSurface)) })
			}
		})
		lead := int(m.Weekday())
		days := m.AddDate(0, 1, -1).Day()
		grid := ui.Grid(c.Key(m.Format("2006-01"))).FillWidth().Columns(7).Height(42 * 6).AlignContent(ui.Start).
			Transition(ui.ElementTransition{Duration: 200 * time.Millisecond, Ease: design.EaseOut,
				Enter: &ui.Motion{Opacity: 0, XPercent: 10 * float32(st.dir)}})
		grid.Children(func() {
			for range lead {
				ui.Box(c).Height(42)
			}
			for d := 1; d <= days; d++ {
				day := time.Date(m.Year(), m.Month(), d, 0, 0, 0, 0, time.Local)
				disabled := day.Before(first) || day.After(last)
				sel := day.Equal(st.selected)
				ui.Box(c.Key(d)).Height(42).Center().Children(func() {
					b := Tappable(c, design.CornerFull).Size(40, 40).Center().Disabled(disabled)
					fg := s.OnSurface
					switch {
					case disabled:
						fg = DisabledContent()
					case sel:
						fg = s.OnPrimary
						b = b.Background(s.Primary)
					case day.Equal(today):
						fg = s.Primary
						b = b.Border(1, s.Primary)
					}
					b.Children(func() { Text(c, strconv.Itoa(d), t.BodyLarge.C(fg)) })
					if b.Clicked() && !disabled {
						st.selected = day
					}
				})
			}
		})
	})
}

// TimeOfDay is an hour and a minute.
type TimeOfDay struct{ Hour, Minute int }

type timePickerState struct {
	t       TimeOfDay
	minutes bool
	input   bool
	h, m    string
	err     bool
}

// ShowTimePicker is showTimePicker in Material 3 with the 24-hour dial the
// Chinese locale uses, and its text input mode.
func ShowTimePicker(initial TimeOfDay, done func(TimeOfDay)) {
	st := &timePickerState{t: initial}
	ShowDialog(func(c *ui.Context, r *Route) { timePickerView(c, r, st) }, func(v any) {
		if t, ok := v.(TimeOfDay); ok && done != nil {
			done(t)
		}
	})
}

func two(n int) string { return fmt.Sprintf("%02d", n) }

func timePickerView(c *ui.Context, r *Route, st *timePickerState) {
	s := cs()
	t := tt()
	ui.Column(c).Width(328).Radius(design.CornerXL).Background(s.SurfaceContainerHigh).Clip().Padding(24, 24, 0, 24).Children(func() {
		help := timePickerHelp
		if st.input {
			help = timeInputHelp
		}
		Text(c, help, t.LabelMedium.C(s.OnSurfaceVariant))
		m3VGap(c, 20)
		ui.Row(c).FillWidth().AlignItems(ui.Start).Children(func() {
			box := func(label string, value int, active bool, minutes bool) {
				bg, fg := s.SurfaceContainerHighest, s.OnSurface
				if active {
					bg, fg = s.PrimaryContainer, s.OnPrimaryContainer
				}
				ui.Column(c).Grow(1).Basis(0).Children(func() {
					if st.input {
						v := &st.h
						if minutes {
							v = &st.m
						}
						in := ui.TextInputBase(c.Key(label), v).Height(72).Radius(8).Background(bg).TextAlign(ui.Center).MaxLength(2)
						in = t.DisplayMedium.C(fg).Apply(in).CaretColor(s.Primary).Label(label)
						if in.Focused() && minutes != st.minutes {
							st.minutes = minutes
						}
						if in.Submitted() {
							timeInputCommit(r, st)
						}
						Text(c, label, t.BodySmall.C(s.OnSurfaceVariant)).Margin(7, 0, 0, 0)
						return
					}
					b := Tappable(c.Key(label), 8).FillWidth().Height(80).Center().Background(bg).Label(label)
					b.Children(func() { Text(c, two(value), t.DisplayLarge.C(fg)) })
					if b.Clicked() {
						st.minutes = minutes
					}
				})
			}
			box(timeHourLabel, st.t.Hour, !st.minutes, false)
			ui.Box(c).Width(24).Height(80).Center().Children(func() { Text(c, ":", t.DisplayLarge.C(s.OnSurface)) })
			box(timeMinuteLabel, st.t.Minute, st.minutes, true)
		})
		if st.input && st.err {
			Text(c, dateInvalid, t.BodySmall.C(s.Error)).Margin(8, 0, 0, 0)
		}
		if !st.input {
			ui.Row(c).FillWidth().Justify(ui.Center).Padding(36, 0, 24, 0).Children(func() { timeDial(c, st) })
		} else {
			m3VGap(c, 24)
		}
		ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(0, 0, 12, 0).Margin(0, -12, 0, -12).Children(func() {
			ic, tip := icons.Keyboard, timeInputMode
			if st.input {
				ic, tip = icons.AccessTime, timeDialMode
			}
			if IconBtn(c, ic, tip).Clicked() {
				st.input = !st.input
				st.h, st.m, st.err = two(st.t.Hour), two(st.t.Minute), false
			}
			ui.Box(c).Grow(1)
			if TextButton(c, cancelLabel).Clicked() {
				r.Close()
			}
			m3HGap(c, 8)
			if TextButton(c, okLabel).Clicked() {
				if st.input {
					timeInputCommit(r, st)
				} else {
					r.Pop(st.t)
				}
			}
		})
	})
}

func m3VGap(c *ui.Context, h float32) { VGap(c, h) }
func m3HGap(c *ui.Context, w float32) { HGap(c, w) }

func timeInputCommit(r *Route, st *timePickerState) {
	h, e1 := strconv.Atoi(strings.TrimSpace(st.h))
	m, e2 := strconv.Atoi(strings.TrimSpace(st.m))
	if e1 != nil || e2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		st.err = true
		return
	}
	st.t = TimeOfDay{h, m}
	r.Pop(st.t)
}

// timeDial is the clock face: hours 00–11 on the outer ring and 12–23 on
// the inner one, or the minutes by fives; dragging or clicking picks.
func timeDial(c *ui.Context, st *timePickerState) {
	s := cs()
	t := tt()
	const size = 256
	const r0 = size / 2
	const outer, inner = r0 - 28, r0 - 64
	e := ui.Box(c).Size(size, size).Shrink(0).Cursor(ui.CursorPointer)
	pick := func(x, y float32, final bool) {
		dx, dy := float64(x-r0), float64(y-r0)
		ang := math.Atan2(dx, -dy)
		if ang < 0 {
			ang += 2 * math.Pi
		}
		dist := math.Hypot(dx, dy)
		if st.minutes {
			m := int(math.Round(ang/(2*math.Pi)*60)) % 60
			st.t.Minute = m
			return
		}
		h := int(math.Round(ang/(2*math.Pi)*12)) % 12
		if dist < (outer+inner)/2 {
			h += 12
		}
		st.t.Hour = h
		if final {
			st.minutes = true
		}
	}
	if x, y, ok := e.PointerPosition(); ok && e.Pressed() {
		pick(x, y, false)
	}
	if e.Clicked() {
		if x, y, ok := e.PointerPosition(); ok {
			pick(x, y, true)
		}
	}
	type label struct {
		text   string
		angle  float64
		radius float32
		on     bool
	}
	var labels []label
	var selAngle float64
	var selR float32
	if st.minutes {
		for i := 0; i < 12; i++ {
			labels = append(labels, label{two(i * 5), float64(i) / 12 * 2 * math.Pi, outer, st.t.Minute == i*5})
		}
		selAngle, selR = float64(st.t.Minute)/60*2*math.Pi, outer
	} else {
		for i := 0; i < 12; i++ {
			labels = append(labels, label{two(i), float64(i) / 12 * 2 * math.Pi, outer, st.t.Hour == i})
			labels = append(labels, label{two(i + 12), float64(i) / 12 * 2 * math.Pi, inner, st.t.Hour == i+12})
		}
		selR = outer
		if st.t.Hour >= 12 {
			selR = inner
		}
		selAngle = float64(st.t.Hour%12) / 12 * 2 * math.Pi
	}
	anim := design.T().Motion(200*time.Millisecond, design.EaseInOut)
	sa := e.AnimateTo("angle", float32(selAngle), anim)
	sr := e.AnimateTo("radius", selR, anim)
	e.Draw(func(p *ui.Painter, rc ui.Rect) {
		cx, cy := rc.X+r0, rc.Y+r0
		p.FillPath((&ui.Path{}).Circle(cx, cy, r0), s.SurfaceContainerHighest)
		hx := cx + sr*float32(math.Sin(float64(sa)))
		hy := cy - sr*float32(math.Cos(float64(sa)))
		p.Line(cx, cy, hx, hy, 2, s.Primary)
		p.FillPath((&ui.Path{}).Circle(cx, cy, 4), s.Primary)
		p.FillPath((&ui.Path{}).Circle(hx, hy, 24), s.Primary)
		for _, l := range labels {
			st := t.BodyLarge.C(s.OnSurface)
			if l.radius == inner {
				st = t.BodyMedium.C(s.OnSurface)
			}
			x := cx + l.radius*float32(math.Sin(l.angle))
			y := cy - l.radius*float32(math.Cos(l.angle))
			// Inside the selector the numbers are onPrimary.
			if math.Hypot(float64(x-hx), float64(y-hy)) < 20 {
				st = st.C(s.OnPrimary)
			}
			w, h := p.MeasureText(0, st.Span(l.text))
			p.RichText(x-w/2, y-h/2, 0, st.Span(l.text))
		}
	})
}
