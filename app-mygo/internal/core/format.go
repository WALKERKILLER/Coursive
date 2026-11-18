package core

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
)

// Fixed is Dart's toStringAsFixed.
func Fixed(v float64, digits int) string { return strconv.FormatFloat(v, 'f', digits, 64) }

// FmtBytes is fmtBytes: "980 B", "1.5 MB", "120 GB".
func FmtBytes(b float64) string {
	if b < 1024 {
		return fmt.Sprintf("%d B", int(b))
	}
	units := []string{"KB", "MB", "GB", "TB"}
	v := b / 1024
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	d := 1
	if v >= 100 {
		d = 0
	}
	return Fixed(v, d) + " " + units[i]
}

func FmtSpeed(bps float64) string { return FmtBytes(bps) + "/s" }

func two(n int) string { return fmt.Sprintf("%02d", n) }

// FmtClock is a media position, "05:30" or "1:05:30".
func FmtClock(ms int) string {
	s := int(math.Floor(float64(ms) / 1000))
	h, m, sec := s/3600, (s%3600)/60, s%60
	if h > 0 {
		return fmt.Sprintf("%d:%s:%s", h, two(m), two(sec))
	}
	return two(m) + ":" + two(sec)
}

// FmtDuration is a length in hours and minutes; "" for none.
func FmtDuration(seconds int) string {
	if seconds <= 0 {
		return ""
	}
	h, m := seconds/3600, (seconds%3600)/60
	if h > 0 {
		if m > 0 {
			return l10n.DurationHoursMinutes(h, m)
		}
		return l10n.DurationHours(h)
	}
	return l10n.DurationMinutes(m)
}

// Weekdays are the short weekday names, Monday first (kWeekdays).
var Weekdays = []string{l10n.WeekdayShort1, l10n.WeekdayShort2, l10n.WeekdayShort3, l10n.WeekdayShort4, l10n.WeekdayShort5, l10n.WeekdayShort6, l10n.WeekdayShort7}

// Weekday is Dart's DateTime.weekday: Monday 1 … Sunday 7.
func Weekday(t time.Time) int {
	w := int(t.Weekday())
	if w == 0 {
		return 7
	}
	return w
}

// Midnight is the local calendar day of t.
func Midnight(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
}

// DaysBetween is today.difference(day).inDays for two local midnights.
func DaysBetween(a, b time.Time) int {
	return int(math.Round(Midnight(a).Sub(Midnight(b)).Hours() / 24))
}

// FmtLectureDate is fmtLectureDate: today, yesterday, tomorrow, a weekday
// this year or a full date.
func FmtLectureDate(d time.Time) string {
	if d.IsZero() {
		return ""
	}
	now := time.Now()
	diff := DaysBetween(now, d)
	hm := two(d.Hour()) + ":" + two(d.Minute())
	switch diff {
	case 0:
		return l10n.TimeToday(hm)
	case 1:
		return l10n.TimeYesterday(hm)
	case -1:
		return l10n.TimeTomorrow(hm)
	}
	wd := l10n.WeekdayName(Weekdays[Weekday(d)-1])
	if d.Year() == now.Year() {
		return l10n.DateMonthDayWeekday(int(d.Month()), d.Day(), wd)
	}
	return l10n.DateYearMonthDay(d.Year(), int(d.Month()), d.Day())
}

// FmtRelative is "刚刚", "5 分钟前" … then a date.
func FmtRelative(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return l10n.TimeJustNow
	case d < time.Hour:
		return l10n.TimeMinutesAgo(int(d.Minutes()))
	case d < 24*time.Hour:
		return l10n.TimeHoursAgo(int(d.Hours()))
	case d < 30*24*time.Hour:
		return l10n.TimeDaysAgo(int(d.Hours() / 24))
	}
	return l10n.DateMonthDay(int(t.Month()), t.Day())
}

// FmtUntil is the time left until t.
func FmtUntil(t time.Time) string {
	d := time.Until(t)
	if d < 0 {
		return l10n.TimeExpired
	}
	hours, mins := int(d.Hours()), int(d.Minutes())
	if hours >= 24 {
		return l10n.DurationDaysHours(hours/24, hours%24)
	}
	if mins >= 60 {
		return l10n.DurationHoursMinutesShort(hours, mins%60)
	}
	return l10n.DurationMinutesLong(mins)
}

// FmtClockTime is the time of day, "14:05:30".
func FmtClockTime(t time.Time) string {
	return two(t.Hour()) + ":" + two(t.Minute()) + ":" + two(t.Second())
}

// KindLabel is kindLabels: an asset kind's name.
func KindLabel(kind string) string {
	switch kind {
	case "video":
		return l10n.AssetVideo
	case "slides":
		return "PPT"
	case "courseware":
		return l10n.AssetCourseware
	case "transcript":
		return l10n.AssetTranscript
	case "audio":
		return l10n.AssetAudio
	case "parse":
		return l10n.AssetParse
	}
	return ""
}

// KindLabels lists the asset kinds in kindLabels' order.
var KindLabels = []string{"video", "slides", "courseware", "transcript", "audio", "parse"}

// FmtCompact is a compact count: 0, 980, 12.3K, 4.5M.
func FmtCompact(n float64) string {
	switch {
	case n >= 1e9:
		return Fixed(n/1e9, 1) + "B"
	case n >= 1e6:
		d := 1
		if n >= 1e7 {
			d = 0
		}
		return Fixed(n/1e6, d) + "M"
	case n >= 1e4:
		return Fixed(n/1e3, 0) + "K"
	case n >= 1e3:
		return Fixed(n/1e3, 1) + "K"
	}
	return strconv.Itoa(int(math.Round(n)))
}

// FmtUsd is an estimated cost; small amounts keep four decimals.
func FmtUsd(v float64) string {
	switch {
	case v == 0:
		return "$0"
	case v < 0.01:
		return "$" + Fixed(v, 4)
	case v < 100:
		return "$" + Fixed(v, 2)
	}
	return "$" + Fixed(v, 0)
}

// FmtDayLabel is "10月14日 周二".
func FmtDayLabel(d time.Time) string {
	return l10n.DateMonthDayWeek(int(d.Month()), d.Day(), Weekdays[Weekday(d)-1])
}

// FmtIsoDay is the local calendar day as YYYY-MM-DD.
func FmtIsoDay(d time.Time) string {
	return fmt.Sprintf("%04d-%02d-%02d", d.Year(), int(d.Month()), d.Day())
}
