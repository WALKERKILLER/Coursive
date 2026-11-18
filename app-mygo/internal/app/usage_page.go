package app

import (
	"errors"
	"fmt"
	"math"
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

// usagePurpose is what a ledger `purpose` means to the user.
type usagePurpose struct {
	label string
	icon  icons.Icon
}

// usagePurposes is kUsagePurposes.
var usagePurposes = map[string]usagePurpose{
	"chat":              {l10n.UsagePurposeChat, icons.ChatBubbleOutlineRounded},
	"timeline":          {l10n.CommonOutline, icons.ViewTimelineOutlined},
	"vision":            {l10n.ModelTagVision, icons.ImageSearchOutlined},
	"title":             {l10n.CommonTitle, icons.TitleRounded},
	"test":              {l10n.UsagePurposeTest, icons.NetworkCheckRounded},
	"task":              {l10n.CommonScheduledTasks, icons.ScheduleRounded},
	"quiz":              {l10n.PanelQuiz, icons.QuizOutlined},
	"transcript-review": {l10n.ReviewStart, icons.SpellcheckRounded},
	"canvas":            {"画布 Agent", icons.AutoAwesomeOutlined},
	"memory":            {l10n.UsagePurposeMemory, icons.BedtimeOutlined},
}

func purposeLabel(p string) string {
	if u, ok := usagePurposes[p]; ok {
		return u.label
	}
	return p
}

// usageRange is UsageRange: the last days, or this year when 0.
type usageRange int

var usageRanges = []usageRange{7, 30, 90, 0}

func (r usageRange) label() string {
	switch r {
	case 7:
		return l10n.Range7Days
	case 30:
		return l10n.Range30Days
	case 90:
		return l10n.Range90Days
	}
	return l10n.RangeThisYear
}

func (r usageRange) bounds(now time.Time) (time.Time, time.Time) {
	today := widgets.DayOf(now)
	if r == 0 {
		return time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.Local), today
	}
	return today.AddDate(0, 0, -int(r)+1), today
}

// callFilter is CallFilter: the call log's filter; empty fields match
// everything.
type callFilter struct {
	from, to, model, purpose, sessionID string
	title                               string
}

func (f callFilter) query() map[string]string {
	q := map[string]string{}
	for k, v := range map[string]string{"from": f.from, "to": f.to, "model": f.model, "purpose": f.purpose, "session_id": f.sessionID} {
		if v != "" {
			q[k] = v
		}
	}
	return q
}

// allTokens is every token the model processed: fresh input, cache reads
// and writes, output.
func allTokens(t core.J) float64 {
	return t.F("input") + t.F("cache_read") + t.F("cache_write") + t.F("output")
}

type usageMetric int

const (
	usageCost usageMetric = iota
	usageTokens
)

type usageBreakdown int

const (
	byModel usageBreakdown = iota
	byPurpose
	bySession
)

// UsagePage is assistant usage from the agent's ledger: totals, a daily
// trend, breakdowns and individual calls
// (docs/specs/0000-usage-ledger-and-cost-stats.md).
type UsagePage struct {
	rng     usageRange
	metric  usageMetric
	by      usageBreakdown
	table   bool
	summary core.J
	recent  []core.J
	err     error
	loading bool
	// gen drops loads a newer one overtook.
	gen     int
	scroll  ui.ScrollState
	tableX  ui.ScrollState
	refresh ui.RefreshState
}

// NewUsagePage shows the last 30 days.
func NewUsagePage() m3.Page {
	p := &UsagePage{rng: 30}
	p.load()
	return p
}

func (p *UsagePage) load() {
	from, to := p.rng.bounds(time.Now())
	q := map[string]string{"from": core.FmtIsoDay(from), "to": core.FmtIsoDay(to)}
	p.loading = true
	p.gen++
	gen := p.gen
	api := State.Api()
	type result struct {
		summary core.J
		recent  []core.J
	}
	core.Async(func() (result, error) {
		var r result
		s, err := api.Get("/agent/usage/summary", q)
		if err != nil {
			return r, err
		}
		calls, err := api.Get("/agent/usage/calls", map[string]string{"from": q["from"], "to": q["to"], "limit": "12"})
		if err != nil {
			return r, err
		}
		r.summary, r.recent = core.Obj(s), core.Objs(calls)
		return r, nil
	}, func(r result, err error) {
		if gen != p.gen {
			return
		}
		p.loading, p.refresh.Refreshing = false, false
		if err != nil {
			p.err = err
			return
		}
		p.summary, p.recent, p.err = r.summary, r.recent, nil
	})
}

func (p *UsagePage) openCalls(f callFilter) { m3.RootNav.Push(newCallLogPage(f)) }

func (p *UsagePage) rangeFilter(f callFilter) callFilter {
	f.from, f.to = p.summary.S("from"), p.summary.S("to")
	return f
}

// View builds the page.
func (p *UsagePage) View(c *ui.Context) {
	s := p.summary
	bar := &m3.AppBar{Title: l10n.SettingsUsage, Scrolled: p.scroll.Y > 0, Actions: func() {
		if (m3.IconButton{Icon: icons.RefreshRounded, Tooltip: l10n.CommonRefresh, Disabled: p.loading}).Show(c).Clicked() {
			p.load()
		}
	}}
	if p.loading && s != nil {
		bar.Bottom = func() { m3.Linear(c, m3.Progress{Height: 2, NoTrack: true}) }
	}
	m3.Scaffold{AppBar: bar, Body: func() { p.body(c) }}.Show(c)
}

func (p *UsagePage) body(c *ui.Context) {
	s := p.summary
	if s == nil && p.loading {
		widgets.CenteredLoader(c, "")
		return
	}
	if s == nil {
		err := p.err
		if err == nil {
			err = errors.New(l10n.CommonNoData)
		}
		widgets.ErrorState(c, err, p.load)
		return
	}
	totals := s.O("totals")
	w, _ := c.Size()
	pad := design.Page
	if design.SizeClassOf(w) == design.SizeCompact {
		pad = design.PageCompact
	}
	sc := ui.Scroll(c).Fill().TrackScroll(&p.scroll)
	if sc.PullToRefresh(&p.refresh) {
		p.load()
	}
	sc.Children(func() {
		widgets.ContentWidth(c, 0, func() {
			ui.Column(c).FillWidth().Padding(design.SpaceSM, pad, design.SpaceXXXL, pad).Children(func() {
				ui.Row(c).Children(func() {
					segs := make([]m3.Segment[usageRange], len(usageRanges))
					for i, r := range usageRanges {
						segs[i] = m3.Segment[usageRange]{Value: r, Label: r.label()}
					}
					if m3.Segmented(c, segs, &p.rng, m3.SegOpts{NoCheck: true}) {
						p.load()
					}
				})
				m3.VGap(c, design.SpaceLG)
				usageTiles(c, totals, s.O("previous"), p.rng)
				m3.VGap(c, design.SpaceLG)
				if totals.F("calls") == 0 {
					ui.Column(c).FillWidth().Padding(design.SpaceXXL, 0, 0, 0).Children(func() {
						widgets.EmptyState{Icon: icons.InsightsOutlined, Title: l10n.UsageEmpty, Body: l10n.UsageEmptyBody}.Show(c)
					})
					return
				}
				usageSection(c, l10n.UsageDaily, func() { p.chartControls(c) }, func() { p.chart(c, s) })
				m3.VGap(c, design.SpaceLG)
				usageSection(c, l10n.UsageBreakdown, func() { p.breakdownControl(c) }, func() { p.breakdown(c, s) })
				m3.VGap(c, design.SpaceLG)
				usageSection(c, l10n.UsageRecentCalls, func() {
					if (m3.Button{Kind: m3.TextKind, Label: l10n.CommonAll}).Show(c).Clicked() {
						p.openCalls(p.rangeFilter(callFilter{title: l10n.UsageCallsInRange(p.rng.label())}))
					}
				}, func() {
					for i, call := range p.recent {
						callTile(c.Key(i), call)
					}
				})
			})
		})
	})
}

// usageSection is _section: a titled surfaceContainerLow card.
func usageSection(c *ui.Context, title string, trailing func(), child func()) {
	cs := design.S()
	ui.Column(c).FillWidth().Padding(design.SpaceMD, design.SpaceLG, design.SpaceLG, design.SpaceLG).
		Radius(design.CornerXL).Background(cs.SurfaceContainerLow).Children(func() {
		ui.Row(c).FillWidth().Height(40).AlignItems(ui.Center).Children(func() {
			m3.Clamp(m3.Text(c, title, design.TT().TitleMedium), 1).Grow(1).Shrink(1)
			if trailing != nil {
				trailing()
			}
		})
		m3.VGap(c, design.SpaceSM)
		child()
	})
}

func (p *UsagePage) chartControls(c *ui.Context) {
	ui.Row(c).AlignItems(ui.Center).Children(func() {
		m3.Segmented(c, []m3.Segment[usageMetric]{
			{Value: usageCost, Label: l10n.UsageCost},
			{Value: usageTokens, Label: "Tokens"},
		}, &p.metric, m3.SegOpts{NoCheck: true, Compact: true})
		m3.HGap(c, design.SpaceXS)
		tip := l10n.UsageTable
		if p.table {
			tip = l10n.UsageChart
		}
		if (m3.IconButton{Icon: icons.TableRowsOutlined, SelectedIcon: icons.BarChartRounded, Toggle: true, Selected: p.table, Tooltip: tip}).Show(c).Clicked() {
			p.table = !p.table
		}
	})
}

func (p *UsagePage) breakdownControl(c *ui.Context) {
	m3.Segmented(c, []m3.Segment[usageBreakdown]{
		{Value: byModel, Label: l10n.CommonModel},
		{Value: byPurpose, Label: l10n.UsagePurpose},
		{Value: bySession, Label: l10n.UsagePurposeChat},
	}, &p.by, m3.SegOpts{NoCheck: true, Compact: true})
}

func (p *UsagePage) chart(c *ui.Context, s core.J) {
	cs := design.S()
	days := s.Os("days")
	dates := make([]time.Time, len(days))
	values := make([][]float64, len(days))
	for i, d := range days {
		dates[i], _ = time.ParseInLocation("2006-01-02", d.S("date"), time.Local)
		if p.metric == usageCost {
			values[i] = []float64{d.F("cost")}
		} else {
			values[i] = []float64{d.F("input") + d.F("cache_write"), d.F("cache_read"), d.F("output")}
		}
	}
	bg := cs.SurfaceContainerLow
	series := []widgets.ChartSeries{{Label: l10n.UsageEstimatedCost, Color: cs.Primary}}
	format := core.FmtUsd
	label := l10n.UsageEstimatedCost
	if p.metric == usageTokens {
		series = []widgets.ChartSeries{
			{Label: l10n.UsageInput, Color: cs.Primary},
			{Label: l10n.UsageCacheHit, Color: design.AlphaBlend(cs.Primary.Alpha(.42), bg)},
			{Label: l10n.UsageOutput, Color: cs.Tertiary},
		}
		format = core.FmtCompact
		label = l10n.UsageTokens
	}
	if p.table {
		dailyTable(c, &p.tableX, dates, values, series, format)
		return
	}
	if len(series) > 1 {
		widgets.ChartLegend(c, series)
		m3.VGap(c, design.SpaceMD)
	}
	widgets.DailyBarChart{Days: dates, Values: values, Series: series, Format: format, Background: bg,
		Label: l10n.UsageChartSemantics(label)}.Show(c)
}

func (p *UsagePage) breakdown(c *ui.Context, s core.J) {
	cs := design.S()
	key := [...]string{byModel: "by_model", byPurpose: "by_purpose", bySession: "by_session"}[p.by]
	rows := s.Os(key)
	if len(rows) > 12 {
		rows = rows[:12]
	}
	if len(rows) == 0 {
		m3.Text(c, l10n.CommonNoRecords, design.TT().BodyMedium).Padding(design.SpaceLG)
		return
	}
	byCost := false
	for _, r := range rows {
		byCost = byCost || r.F("cost") > 0
	}
	value := func(r core.J) float64 {
		if byCost {
			return r.F("cost")
		}
		return allTokens(r)
	}
	top := 0.0
	for _, r := range rows {
		top = math.Max(top, value(r))
	}
	for i, r := range rows {
		k := fmt.Sprint(r["key"])
		label, hint := k, ""
		var ic icons.Icon
		f := callFilter{}
		switch p.by {
		case byModel:
			if j := strings.Index(k, "/"); j >= 0 {
				label, hint = k[j+1:], k[:j]
			}
			f.model = k
		case byPurpose:
			label = purposeLabel(k)
			ic = usagePurposes[k].icon
			f.purpose = k
		case bySession:
			label = r.S("title")
			if label == "" {
				label = l10n.UsageDeletedChat
			}
			f.sessionID = k
		}
		var sub []string
		if hint != "" {
			sub = append(sub, hint)
		}
		sub = append(sub, l10n.UsageCalls(core.FmtCompact(r.F("calls"))),
			l10n.UsageInputTokens(core.FmtCompact(r.F("input")+r.F("cache_read")+r.F("cache_write"))),
			l10n.ModelTagOutput(core.FmtCompact(r.F("output"))))
		trailing := core.FmtCompact(allTokens(r))
		if byCost {
			trailing = core.FmtUsd(r.F("cost"))
		}
		frac := 0.0
		if top > 0 {
			frac = value(r) / top
		}
		row := widgets.RankedRow{Label: label, Sublabel: strings.Join(sub, " · "), Trailing: trailing, Fraction: frac, Tap: true}
		if ic != 0 {
			row.Leading = func() { m3.Icon(c, ic, 20, cs.OnSurfaceVariant) }
		}
		if row.Show(c.Key(fmt.Sprint(p.by, i, k))) {
			f.title = label
			p.openCalls(p.rangeFilter(f))
		}
	}
}

// usageDelta is _Tiles._delta: the change against the span before.
func usageDelta(rng usageRange, now, before float64) string {
	span := l10n.UsageLastYear
	if rng != 0 {
		span = l10n.UsagePreviousDays(int(rng))
	}
	if before == 0 {
		if now == 0 {
			return ""
		}
		return l10n.UsageNoneIn(span)
	}
	pct := (now - before) / before * 100
	if math.Abs(pct) < .5 {
		return l10n.UsageSameAs(span)
	}
	dir := l10n.HeatmapLess
	if pct > 0 {
		dir = l10n.HeatmapMore
	}
	return l10n.UsageCompare(span, dir, int(math.Round(math.Abs(pct))))
}

// usageTiles is _Tiles: cost, tokens, calls and the cache hit rate.
func usageTiles(c *ui.Context, totals, previous core.J, rng usageRange) {
	prompt := totals.F("input") + totals.F("cache_read")
	errs := totals.F("errors")
	calls := usageDelta(rng, totals.F("calls"), previous.F("calls"))
	if errs > 0 {
		calls = l10n.UsageFailedCalls(core.FmtCompact(errs))
	}
	hit := widgets.StatTile{Icon: icons.BoltOutlined, Label: l10n.UsageCacheRate, Value: "—"}
	if prompt > 0 {
		h := totals.F("cache_read") / prompt
		d := 0
		if h < .1 {
			d = 1
		}
		hit.Value = core.Fixed(h*100, d) + "%"
		hit.Note = l10n.UsageCacheTokens(core.FmtCompact(totals.F("cache_read")))
	}
	widgets.StatGrid(c,
		widgets.StatTile{Icon: icons.PaymentsOutlined, Label: l10n.UsageEstimatedCost, Value: core.FmtUsd(totals.F("cost")),
			Note: usageDelta(rng, totals.F("cost"), previous.F("cost"))},
		widgets.StatTile{Icon: icons.DataUsageRounded, Label: "Tokens", Value: core.FmtCompact(allTokens(totals)),
			Note: l10n.UsageInputOutput(core.FmtCompact(prompt+totals.F("cache_write")), core.FmtCompact(totals.F("output")))},
		widgets.StatTile{Icon: icons.SwapVertRounded, Label: l10n.UsageCallCount, Value: core.FmtCompact(totals.F("calls")), Note: calls},
		hit,
	)
}

// dailyTable is _DailyTable: the chart's days with any usage, newest
// first, as a DataTable.
func dailyTable(c *ui.Context, st *ui.ScrollState, dates []time.Time, values [][]float64, series []widgets.ChartSeries, format func(float64) string) {
	cs := design.S()
	tt := design.TT()
	var rows []int
	for i := len(dates) - 1; i >= 0; i-- {
		for _, v := range values[i] {
			if v > 0 {
				rows = append(rows, i)
				break
			}
		}
	}
	if len(rows) == 0 {
		m3.Text(c, l10n.CommonNoRecords, tt.BodyMedium)
		return
	}
	head := []string{l10n.CommonDate}
	for _, s := range series {
		head = append(head, s.Label)
	}
	if len(series) > 1 {
		head = append(head, l10n.ChartTotal)
	}
	cols := len(head)
	tracks := make([]ui.Track, cols)
	for i := range tracks {
		tracks[i] = ui.FitContent()
	}
	figs := tt.BodySmall
	// cell is a DataTable cell: 24 margins at the ends, 56 between columns,
	// numeric columns to the right and a divider above each data row.
	cell := func(col int, text string, st design.TextStyle, h float32, divider bool) {
		left, right := float32(28), float32(28)
		if col == 0 {
			left = 24
		}
		if col == cols-1 {
			right = 24
		}
		e := ui.Row(c).Height(h).Padding(0, right, 0, left).AlignItems(ui.Center)
		if col > 0 {
			e.Justify(ui.End)
		}
		if divider {
			e.BorderWidth(1, 0, 0, 0).BorderColor(cs.OutlineVariant)
		}
		e.Children(func() { m3.Text(c, text, st).MaxLines(1).FontFeatures("tnum") })
	}
	widgets.HScroller{State: st}.Show(c, func() {
		ui.Grid(c).ColumnTracks(tracks...).Children(func() {
			for i, h := range head {
				cell(i, h, tt.TitleSmall, 36, false)
			}
			for _, r := range rows {
				cell(0, core.FmtDayLabel(dates[r]), figs, 36, true)
				total := 0.0
				for j, v := range values[r] {
					cell(j+1, format(v), figs, 36, true)
					total += v
				}
				if len(series) > 1 {
					cell(cols-1, format(total), figs, 36, true)
				}
			}
		})
	})
}

// callTile is CallTile: one ledger line, with when, which model and why,
// tokens in and out, and the cost.
func callTile(c *ui.Context, call core.J) {
	cs := design.S()
	tt := design.TT()
	t := time.UnixMilli(int64(call.F("ts"))).Local()
	ok := call["ok"] != false
	figs := tt.BodySmall.C(cs.OnSurfaceVariant)
	ms := call.F("ms")
	cached := call.F("cache_read")
	ui.Row(c).FillWidth().Padding(design.SpaceSM).AlignItems(ui.Start).Children(func() {
		ui.Column(c).Width(52).Shrink(0).Children(func() {
			m3.Text(c, fmt.Sprintf("%02d:%02d", t.Hour(), t.Minute()), tt.LabelLarge).FontFeatures("tnum")
			m3.Text(c, fmt.Sprintf("%d/%d", int(t.Month()), t.Day()), figs).FontFeatures("tnum")
		})
		m3.HGap(c, design.SpaceSM)
		ui.Column(c).Grow(1).Basis(0).Shrink(1).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				m3.Clamp(m3.Text(c, fmt.Sprint(call["model"]), tt.BodyMedium), 1).Shrink(1)
				m3.HGap(c, design.SpaceSM)
				m3.Text(c, purposeLabel(call.S("purpose")), tt.LabelSmall.C(cs.OnSurfaceVariant)).Shrink(0)
				if !ok {
					m3.HGap(c, design.SpaceSM)
					m3.Icon(c, icons.ErrorOutlineRounded, 14, cs.Error)
					m3.HGap(c, 2)
					m3.Text(c, l10n.CommonFailed, tt.LabelSmall.C(cs.Error)).Shrink(0)
				}
			})
			m3.VGap(c, 2)
			io := core.FmtCompact(call.F("input") + cached + call.F("cache_write"))
			if cached > 0 {
				io += l10n.UsageCachedSuffix(core.FmtCompact(cached))
			}
			parts := []string{io + " → " + core.FmtCompact(call.F("output"))}
			if ms > 0 {
				parts = append(parts, core.Fixed(ms/1000, 1)+"s")
			}
			if call.Has("ttft_ms") && call["ttft_ms"] != nil {
				parts = append(parts, l10n.UsageFirstToken(core.Fixed(call.F("ttft_ms")/1000, 1)))
			}
			if call.Has("title") && call["title"] != nil {
				parts = append(parts, fmt.Sprint(call["title"]))
			}
			m3.Clamp(m3.Text(c, strings.Join(parts, " · "), figs).FontFeatures("tnum"), 1)
		})
		m3.HGap(c, design.SpaceMD)
		m3.Text(c, core.FmtUsd(call.F("cost")), tt.BodyMedium).FontFeatures("tnum").Shrink(0)
	})
}

// callLogPage is CallLogPage: a paged, newest-first list of ledger lines
// for one filter.
type callLogPage struct {
	filter  callFilter
	calls   []core.J
	done    bool
	loading bool
	err     error
	scroll  ui.ScrollState
}

func newCallLogPage(f callFilter) *callLogPage {
	p := &callLogPage{filter: f}
	p.more()
	return p
}

const callPage = 60

func (p *callLogPage) more() {
	if p.loading || p.done {
		return
	}
	p.loading = true
	q := p.filter.query()
	q["limit"] = fmt.Sprint(callPage)
	if n := len(p.calls); n > 0 {
		last := p.calls[n-1]
		if last.Has("cursor") && last["cursor"] != nil {
			q["before"] = fmt.Sprint(last["cursor"])
		} else {
			q["before"] = fmt.Sprint(core.Int(last["ts"]))
		}
	}
	api := State.Api()
	core.Async(func() ([]core.J, error) {
		v, err := api.Get("/agent/usage/calls", q)
		return core.Objs(v), err
	}, func(page []core.J, err error) {
		p.loading = false
		if err != nil {
			p.err = err
			return
		}
		p.calls = append(p.calls, page...)
		p.done, p.err = len(page) < callPage, nil
	})
}

// View builds the page.
func (p *callLogPage) View(c *ui.Context) {
	m3.Scaffold{AppBar: &m3.AppBar{Title: p.filter.title, Scrolled: p.scroll.Y > 0}, Body: func() { p.body(c) }}.Show(c)
}

func (p *callLogPage) body(c *ui.Context) {
	switch {
	case len(p.calls) == 0 && p.err != nil:
		widgets.ErrorState(c, p.err, p.more)
		return
	case len(p.calls) == 0 && p.loading:
		widgets.CenteredLoader(c, "")
		return
	case len(p.calls) == 0:
		widgets.EmptyState{Icon: icons.ReceiptLongOutlined, Title: l10n.UsageNoCalls}.Show(c)
		return
	}
	if !p.done && p.scroll.MaxY > 0 && p.scroll.MaxY-p.scroll.Y < 400 {
		core.Post(p.more)
	}
	w, _ := c.Size()
	pad := design.Page
	if design.SizeClassOf(w) == design.SizeCompact {
		pad = design.PageCompact
	}
	ui.Scroll(c).Fill().TrackScroll(&p.scroll).Children(func() {
		ui.Column(c).FillWidth().PaddingY(design.SpaceSM).Children(func() {
			for i, call := range p.calls {
				if i > 0 {
					m3.DividerIndent(c, 1, design.SpaceXXXL+design.SpaceXL, 0)
				}
				widgets.ContentWidth(c.Key(i), 840, func() {
					ui.Column(c).FillWidth().PaddingX(pad).Children(func() { callTile(c, call) })
				})
			}
			if !p.done {
				ui.Row(c).FillWidth().Padding(design.SpaceLG).Justify(ui.Center).Children(func() {
					m3.Circular(c, m3.Progress{})
				})
			}
		})
	})
}
