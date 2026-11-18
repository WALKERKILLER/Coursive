package widgets

import (
	"math"
	"slices"
	"sort"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
)

// DayOf is midnight of d's local day.
func DayOf(d time.Time) time.Time { return core.Midnight(d) }

// WeekStart is the Monday of the week holding d.
func WeekStart(d time.Time) time.Time {
	d = DayOf(d)
	return d.AddDate(0, 0, -(core.Weekday(d) - 1))
}

// HeatWeeks are the week columns from from to to, Monday first; days out
// of the range are zero so that a column always has seven slots.
func HeatWeeks(from, to time.Time) [][7]time.Time {
	a, b := DayOf(from), DayOf(to)
	var weeks [][7]time.Time
	for w := WeekStart(a); !w.After(b); w = w.AddDate(0, 0, 7) {
		var col [7]time.Time
		for i := range 7 {
			d := w.AddDate(0, 0, i)
			if !d.Before(a) && !d.After(b) {
				col[i] = d
			}
		}
		weeks = append(weeks, col)
	}
	return weeks
}

// HeatThresholds are the upper bounds of levels 1–3: the quartiles of the
// values above zero, so that light and heavy users both see contrast.
func HeatThresholds(values []float64) [3]float64 {
	var v []float64
	for _, x := range values {
		if x > 0 {
			v = append(v, x)
		}
	}
	if len(v) == 0 {
		return [3]float64{}
	}
	sort.Float64s(v)
	q := func(p float64) float64 {
		i := float64(len(v)-1) * p
		lo, hi := int(math.Floor(i)), int(math.Ceil(i))
		return v[lo] + (v[hi]-v[lo])*(i-float64(lo))
	}
	return [3]float64{q(.25), q(.5), q(.75)}
}

// HeatLevel is 0 for no activity, else 1–4 against the thresholds.
func HeatLevel(value float64, th [3]float64) int {
	if value <= 0 {
		return 0
	}
	for i := range 3 {
		if value <= th[i] {
			return i + 1
		}
	}
	return 4
}

// HeatColors are the five cell colours: an empty step, then four toward
// primary, from the scheme so that dark mode has its own ramp.
func HeatColors(s *design.Scheme) []ui.Color {
	out := []ui.Color{s.SurfaceContainerHighest}
	for _, t := range []float32{.3, .52, .76, 1} {
		out = append(out, design.Lerp(s.SurfaceContainerHighest, s.Primary, t))
	}
	return out
}

// Heatmap metrics.
const (
	HeatGap         float32 = 3
	HeatLabelWidth  float32 = 28
	HeatMonthHeight float32 = 18
)

// HeatWeeksFor is how many columns fit width at cell size.
func HeatWeeksFor(width, cell float32) int {
	if cell == 0 {
		cell = 12
	}
	return min(max(int(math.Floor(float64((width-HeatLabelWidth+HeatGap)/(cell+HeatGap)))), 1), 53)
}

// Heatmap is ContributionHeatmap: a calendar heatmap, one column a week,
// Monday on top, with a tooltip over the hovered day and arrow keys
// between days.
type Heatmap struct {
	From, To time.Time
	// Today: days after it are blank; zero is now.
	Today    time.Time
	Value    func(day time.Time) float64
	Describe func(day time.Time) string
	Tooltip  func(day time.Time) (string, []string)
	OnTap    func(day time.Time)
	// Cell is the side of a day; 0 is 12.
	Cell float32
}

type heatState struct {
	focused     [2]int
	haveFocused bool
}

// Show builds the heatmap at its natural size.
func (h Heatmap) Show(c *ui.Context) ui.Element {
	cs := design.S()
	tt := design.TT()
	cell := h.Cell
	if cell == 0 {
		cell = 12
	}
	today := DayOf(h.Today)
	if h.Today.IsZero() {
		today = DayOf(time.Now())
	}
	weeks := HeatWeeks(h.From, h.To)
	var vals []float64
	for _, w := range weeks {
		for _, d := range w {
			if !d.IsZero() && !d.After(today) {
				vals = append(vals, h.Value(d))
			}
		}
	}
	th := HeatThresholds(vals)
	step := cell + HeatGap
	width := HeatLabelWidth + float32(len(weeks))*step - HeatGap
	height := HeatMonthHeight + 7*step - HeatGap
	at := func(col, row int) (time.Time, bool) {
		if col < 0 || col >= len(weeks) || row < 0 || row >= 7 {
			return time.Time{}, false
		}
		d := weeks[col][row]
		return d, !d.IsZero() && !d.After(today)
	}
	hit := func(x, y float32) (int, int, bool) {
		x, y = x-HeatLabelWidth, y-HeatMonthHeight
		if x < 0 || y < 0 {
			return 0, 0, false
		}
		col, row := int(x/step), int(y/step)
		if col >= len(weeks) || row >= 7 || x-float32(col)*step > cell || y-float32(row)*step > cell {
			return 0, 0, false
		}
		_, ok := at(col, row)
		return col, row, ok
	}
	last := func() (int, int, bool) {
		for col := len(weeks) - 1; col >= 0; col-- {
			for row := 6; row >= 0; row-- {
				if _, ok := at(col, row); ok {
					return col, row, true
				}
			}
		}
		return 0, 0, false
	}

	e := ui.Box(c).Size(width, height).Shrink(0).Focusable()
	if h.OnTap != nil {
		e = e.Cursor(ui.CursorPointer)
	}
	st := ui.Local(e, "heat", func() heatState { return heatState{} })
	focus := e.Focused()
	if focus && !st.haveFocused {
		st.focused[0], st.focused[1], st.haveFocused = last()
	}
	hcol, hrow, hovering := 0, 0, false
	if x, y, ok := e.PointerPosition(); ok && e.Hovered() {
		hcol, hrow, hovering = hit(x, y)
	}
	if e.Clicked() && hovering {
		st.focused, st.haveFocused = [2]int{hcol, hrow}, true
		if d, ok := at(hcol, hrow); ok && h.OnTap != nil {
			h.OnTap(d)
		}
	}
	if focus && st.haveFocused {
		move := func(dc, dr int) {
			n := [2]int{st.focused[0] + dc, st.focused[1] + dr}
			if _, ok := at(n[0], n[1]); ok {
				st.focused = n
			}
		}
		switch {
		case e.Shortcut(0, ui.KeyLeft):
			move(-1, 0)
		case e.Shortcut(0, ui.KeyRight):
			move(1, 0)
		case e.Shortcut(0, ui.KeyUp):
			move(0, -1)
		case e.Shortcut(0, ui.KeyDown):
			move(0, 1)
		case e.Shortcut(0, ui.KeyEnter), e.Shortcut(0, ui.KeySpace):
			if d, ok := at(st.focused[0], st.focused[1]); ok && h.OnTap != nil {
				h.OnTap(d)
			}
		}
	}
	shown, showing := [2]int{hcol, hrow}, hovering
	if !showing && focus && st.haveFocused {
		shown, showing = st.focused, true
	}
	if showing {
		if d, ok := at(shown[0], shown[1]); ok && h.Describe != nil {
			e.Label(h.Describe(d))
		}
	}
	colors := HeatColors(cs)
	label := tt.LabelSmall.C(cs.OnSurfaceVariant)
	rect := func(col, row int) ui.Rect {
		return ui.Rect{X: HeatLabelWidth + float32(col)*step, Y: HeatMonthHeight + float32(row)*step, W: cell, H: cell}
	}
	months := map[int]int{}
	for col, w := range weeks {
		for _, d := range w {
			if !d.IsZero() && d.Day() == 1 {
				months[col] = int(d.Month())
			}
		}
	}
	if len(weeks) > 0 {
		if _, ok := months[0]; !ok {
			early := false
			for col := range months {
				if col < 3 {
					early = true
				}
			}
			if !early {
				for _, d := range weeks[0] {
					if !d.IsZero() {
						months[0] = int(d.Month())
						break
					}
				}
			}
		}
	}
	cols := make([]int, 0, len(months))
	for col := range months {
		cols = append(cols, col)
	}
	slices.Sort(cols)
	var focusedCell *[2]int
	if focus && st.haveFocused {
		f := st.focused
		focusedCell = &f
	}
	var hoverCell *[2]int
	if hovering {
		hv := [2]int{hcol, hrow}
		hoverCell = &hv
	}
	e.Draw(func(p *ui.Painter, r ui.Rect) {
		rect := func(col, row int) ui.Rect {
			x := rect(col, row)
			x.X += r.X
			x.Y += r.Y
			return x
		}
		{
			for _, wd := range []struct {
				row int
				s   string
			}{{0, l10n.WeekdayMon}, {2, l10n.WeekdayWed}, {4, l10n.WeekdayFri}} {
				_, th := p.MeasureText(0, label.Span(wd.s))
				cr := rect(0, wd.row)
				p.RichText(r.X, cr.Y+cr.H/2-th/2, 0, label.Span(wd.s))
			}
			lastCol := -3
			for _, col := range cols {
				if col-lastCol < 3 {
					continue
				}
				p.RichText(rect(col, 0).X, r.Y, 0, label.Span(l10n.DateMonthOf(months[col])))
				lastCol = col
			}
			rad := min(3, cell/4)
			for col := range weeks {
				for row := range 7 {
					d, ok := at(col, row)
					if !ok {
						continue
					}
					cr := rect(col, row)
					p.Fill(cr, colors[HeatLevel(h.Value(d), th)], rad)
					if d.Equal(today) {
						p.Stroke(ui.Rect{X: cr.X - 1, Y: cr.Y - 1, W: cr.W + 2, H: cr.H + 2}, cs.Outline, rad+1, 1.5)
					}
				}
			}
			for _, pc := range []*[2]int{focusedCell, hoverCell} {
				if pc == nil {
					continue
				}
				cr := rect(pc[0], pc[1])
				p.Stroke(ui.Rect{X: cr.X - 1.5, Y: cr.Y - 1.5, W: cr.W + 3, H: cr.H + 3}, cs.OnSurface, rad+1.5, 2)
			}
		}
	})
	e.Children(func() {
		if !showing || h.Tooltip == nil {
			return
		}
		d, ok := at(shown[0], shown[1])
		if !ok {
			return
		}
		title, lines := h.Tooltip(d)
		const w = 176
		ax := HeatLabelWidth + float32(shown[0])*step + cell/2
		ay := HeatMonthHeight + float32(shown[1])*step
		x := min(max(ax-w/2, 0), max(0, width-w))
		figs := tt.BodySmall.C(cs.OnInverseSurface)
		ui.Column(c).Absolute().Left(x).Top(ay-design.SpaceSM).Width(w).TranslatePercent(0, -100).IgnoreInput(true).
			Padding(design.SpaceSM, design.SpaceMD, design.SpaceSM, design.SpaceMD).Radius(design.CornerSM).
			Background(cs.InverseSurface).ZIndex(10).Children(func() {
			m3.Text(c, title, tt.LabelMedium.C(cs.OnInverseSurface))
			for _, l := range lines {
				m3.VGap(c, 2)
				m3.Text(c, l, figs).FontFeatures("tnum")
			}
		})
	})
	return e
}

// HeatLegend is "少 ▢▢▢▢▢ 多" under a heatmap.
func HeatLegend(c *ui.Context, cell float32) ui.Element {
	if cell == 0 {
		cell = 12
	}
	cs := design.S()
	style := design.TT().LabelSmall.C(cs.OnSurfaceVariant)
	return ui.Row(c).AlignItems(ui.Center).Children(func() {
		m3.Text(c, l10n.HeatmapLess, style)
		m3.HGap(c, design.SpaceXS)
		for _, col := range HeatColors(cs) {
			ui.Box(c).Size(cell, cell).MarginX(HeatGap / 2).Radius(min(3, cell/4)).Background(col)
		}
		m3.HGap(c, design.SpaceXS)
		m3.Text(c, l10n.HeatmapMore, style)
	})
}

// FmtMinutes is "42 分钟" or "1 小时 5 分".
func FmtMinutes(minutes float64) string {
	m := int(math.Round(minutes))
	if m < 60 {
		return l10n.DurationMinutes(m)
	}
	if m%60 == 0 {
		return l10n.DurationHoursOnly(m / 60)
	}
	return l10n.DurationHoursMinutesOf(m/60, m%60)
}

// FmtMonthDay is "10月14日".
func FmtMonthDay(d time.Time) string { return l10n.DateMonthDay(int(d.Month()), d.Day()) }
