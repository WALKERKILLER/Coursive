package widgets

import (
	"fmt"
	"math"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
)

// Width is the width e had in the last frame, or fallback before it has
// been laid out: LayoutBuilder's constraints a frame late.
func Width(e ui.Element, fallback float32) float32 {
	if w := e.Bounds().W; w > 0 {
		return w
	}
	return fallback
}

// PageWidth is Width for a page's root, which keeps its last width in
// *last: built anew as a page above it pops, the root is not laid out yet,
// and the window's width would lay its first frame out wider than the
// page is.
func PageWidth(e ui.Element, last *float32, window float32) float32 {
	if *last == 0 {
		*last = window
	}
	*last = Width(e, *last)
	return *last
}

// StatTile is a headline number with a label and an optional note.
type StatTile struct {
	Icon         icons.Icon
	Label, Value string
	Note         string
}

// Show builds the tile.
func (t StatTile) Show(c *ui.Context) ui.Element {
	cs := design.S()
	tt := design.TT()
	return ui.Column(c).FillWidth().Padding(design.SpaceMD, design.SpaceLG, design.SpaceMD, design.SpaceLG).
		Radius(design.CornerLG).Background(cs.SurfaceContainerLow).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			if t.Icon != 0 {
				m3.Icon(c, t.Icon, 16, cs.OnSurfaceVariant)
				m3.HGap(c, design.SpaceXS+2)
			}
			m3.Clamp(m3.Text(c, t.Label, tt.LabelMedium.C(cs.OnSurfaceVariant)), 1).Shrink(1)
		})
		m3.VGap(c, design.SpaceSM)
		m3.Text(c, t.Value, tt.HeadlineSmall).MaxLines(1).FontFeatures("tnum")
		if t.Note != "" {
			m3.VGap(c, design.SpaceXXS)
			m3.Clamp(m3.Text(c, t.Note, tt.BodySmall.C(cs.OnSurfaceVariant)), 1)
		}
	})
}

// StatGrid lays tiles out four across on wide windows, two on compact.
func StatGrid(c *ui.Context, tiles ...StatTile) ui.Element {
	g := ui.Grid(c).FillWidth().Gap(design.SpaceMD)
	cols := 2
	if Width(g, 0) >= 640 {
		cols = 4
	}
	return g.Columns(cols).Children(func() {
		for _, t := range tiles {
			t.Show(c)
		}
	})
}

// ChartSeries is a series' label and colour.
type ChartSeries struct {
	Label string
	Color ui.Color
}

// ChartLegend is the swatches and labels above a chart of several series.
func ChartLegend(c *ui.Context, series []ChartSeries) ui.Element {
	cs := design.S()
	tt := design.TT()
	return ui.Row(c).Wrap().GapX(design.SpaceLG).GapY(design.SpaceXS).Children(func() {
		for _, s := range series {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(10, 10).Radius(3).Background(s.Color)
				m3.HGap(c, design.SpaceXS+2)
				m3.Text(c, s.Label, tt.LabelMedium.C(cs.OnSurfaceVariant))
			})
		}
	})
}

// DailyBarChart is stacked daily bars on one y axis, with a recessive grid
// and a tooltip of the day under the pointer. Values[day][series] stack
// from the bottom.
type DailyBarChart struct {
	Days   []time.Time
	Values [][]float64
	Series []ChartSeries
	Format func(float64) string
	// Height is 208 when zero.
	Height float32
	Label  string
	// Background is the surface the chart sits on; stacked segments are
	// parted by gaps of it.
	Background ui.Color
}

func niceMax(v float64) float64 {
	if v <= 0 {
		return 1
	}
	p := math.Pow(10, math.Floor(math.Log10(v)))
	for _, m := range []float64{1, 2, 2.5, 5, 10} {
		if v <= m*p {
			return m * p
		}
	}
	return 10 * p
}

// Show builds the chart in its parent's width.
func (ch DailyBarChart) Show(c *ui.Context) ui.Element {
	cs := design.S()
	tt := design.TT()
	height := ch.Height
	if height == 0 {
		height = 208
	}
	const axisGap = design.SpaceSM
	const bottom = 22
	totals := make([]float64, len(ch.Values))
	peak := 0.0
	for i, d := range ch.Values {
		for _, v := range d {
			totals[i] += v
		}
		peak = math.Max(peak, totals[i])
	}
	top := niceMax(peak)
	label := tt.LabelSmall.C(cs.OnSurfaceVariant)
	gap := ch.Background
	if gap.A == 0 {
		gap = cs.SurfaceContainerLow
	}
	e := ui.Box(c).FillWidth().Height(height)
	if ch.Label != "" {
		e = e.Label(ch.Label)
	}
	width := Width(e, 0)
	axisW := float32(0)
	for _, v := range []float64{0, top / 2, top} {
		w, _ := c.MeasureText(0, label.Span(ch.Format(v)))
		axisW = max(axisW, w)
	}
	axisW += axisGap
	n := len(ch.Days)
	plotW := width - axisW
	slot := float32(0)
	if n > 0 {
		slot = plotW / float32(n)
	}
	hover := -1
	if x, _, ok := e.PointerPosition(); ok && (e.Hovered() || e.Pressed()) && n > 0 && x >= axisW && slot > 0 {
		hover = min(max(int((x-axisW)/slot), 0), n-1)
	}
	e.Draw(func(p *ui.Painter, r ui.Rect) {
		plotH := r.H - bottom
		y := func(v float64) float32 { return plotH - float32(v/top)*(plotH-6) }
		text := func(s string, x, yy float32, align int) {
			w, h := p.MeasureText(0, label.Span(s))
			switch align {
			case 1:
				x -= w
			case 2:
				x -= w / 2
			}
			p.RichText(r.X+x, r.Y+yy-h/2, 0, label.Span(s))
		}
		for _, v := range []float64{0, top / 2, top} {
			yy := float32(math.Round(float64(y(v)))) + .5
			col := cs.OutlineVariant
			if v != 0 {
				col = design.WithOpacity(col, .5)
			}
			p.Line(r.X+axisW, r.Y+yy, r.X+r.W, r.Y+yy, 1, col)
			text(ch.Format(v), axisW-axisGap, yy, 1)
		}
		if n == 0 {
			return
		}
		s := (r.W - axisW) / float32(n)
		barW := min(max(s*.62, 2), 18)
		if hover >= 0 {
			p.Fill(ui.Rect{X: r.X + axisW + s*float32(hover), Y: r.Y, W: s, H: plotH}, design.WithOpacity(cs.OnSurface, .06), 4)
		}
		for i := range n {
			cx := axisW + s*float32(i) + s/2
			base := plotH
			last := -1
			for k, v := range ch.Values[i] {
				if v > 0 {
					last = k
				}
			}
			for k, v := range ch.Values[i] {
				if v <= 0 {
					continue
				}
				hgt := max(1, float32(v/top)*(plotH-6))
				rect := ui.Rect{X: r.X + cx - barW/2, Y: r.Y + base - hgt, W: barW, H: hgt}
				col := ch.Series[k%len(ch.Series)].Color
				if k == last {
					rad := min(4, barW/2)
					p.FillRadii(rect, [4]float32{rad, rad, 0, 0}, col)
				} else {
					p.Fill(rect, col, 0)
				}
				base -= hgt
				// 2 px surface gap between stacked segments.
				if k != last && hgt > 3 {
					p.Fill(ui.Rect{X: r.X + cx - barW/2, Y: r.Y + base, W: barW, H: 2}, gap, 0)
				}
			}
		}
		// Date labels at least 56 apart; month starts on long ranges.
		every := max(1, int(math.Ceil(float64(56/s))))
		long := n > 62
		for i, d := range ch.Days {
			show := i%every == 0
			if long {
				show = d.Day() == 1
			}
			if !show {
				continue
			}
			lbl := fmt.Sprintf("%d/%d", int(d.Month()), d.Day())
			if long {
				lbl = l10n.DateMonth(int(d.Month()))
			}
			text(lbl, axisW+s*float32(i)+s/2, plotH+bottom/2+2, 2)
		}
	})
	e.Children(func() {
		if hover < 0 || hover >= n {
			return
		}
		const w = 168
		left := axisW + slot*float32(hover) + slot/2
		x := left + design.SpaceMD
		if left+design.SpaceMD+w > width {
			x = left - design.SpaceMD - w
		}
		x = min(max(x, 0), max(0, width-w))
		figs := tt.BodySmall.C(cs.OnInverseSurface)
		ui.Column(c).Absolute().Left(x).Top(0).Width(w).IgnoreInput(true).ZIndex(10).
			Padding(design.SpaceSM, design.SpaceMD, design.SpaceSM, design.SpaceMD).Radius(design.CornerSM).
			Background(cs.InverseSurface).Children(func() {
			m3.Text(c, core.FmtDayLabel(ch.Days[hover]), tt.LabelMedium.C(cs.OnInverseSurface))
			m3.VGap(c, design.SpaceXS)
			for i := len(ch.Series) - 1; i >= 0; i-- {
				s := ch.Series[i]
				ui.Row(c).FillWidth().AlignItems(ui.Center).Margin(2, 0, 0, 0).Children(func() {
					ui.Box(c).Size(8, 8).Radius(2).Background(s.Color)
					m3.HGap(c, design.SpaceXS+2)
					m3.Text(c, s.Label, figs).Grow(1).Basis(0)
					m3.Text(c, ch.Format(ch.Values[hover][i]), figs).FontFeatures("tnum")
				})
			}
			if len(ch.Series) > 1 {
				ui.Row(c).FillWidth().Margin(design.SpaceXS, 0, 0, 0).Children(func() {
					m3.Text(c, l10n.ChartTotal, figs.W(600)).Grow(1).Basis(0)
					m3.Text(c, ch.Format(totals[hover]), figs.W(600)).FontFeatures("tnum")
				})
			}
		})
	})
	return e
}

// RankedRow is a label, a thin bar in proportion and figures at the end.
// It reports a tap when Tap is set.
type RankedRow struct {
	Label, Sublabel string
	Fraction        float64
	Trailing        string
	Leading         func()
	Tap             bool
}

// Show builds the row.
func (r RankedRow) Show(c *ui.Context) bool {
	cs := design.S()
	tt := design.TT()
	var e ui.Element
	if r.Tap {
		e = m3.Tappable(c, design.CornerMD).FillWidth()
	} else {
		e = ui.Box(c).FillWidth()
	}
	e.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).PaddingX(design.SpaceSM).PaddingY(design.SpaceSM + 2).Children(func() {
			if r.Leading != nil {
				r.Leading()
				m3.HGap(c, design.SpaceMD)
			}
			ui.Column(c).Grow(1).Basis(0).Children(func() {
				ui.Row(c).FillWidth().Children(func() {
					m3.Clamp(m3.Text(c, r.Label, tt.BodyMedium), 1).Grow(1).Basis(0)
					m3.HGap(c, design.SpaceMD)
					m3.Text(c, r.Trailing, tt.BodyMedium).FontFeatures("tnum")
				})
				if r.Sublabel != "" {
					m3.VGap(c, 2)
					m3.Clamp(m3.Text(c, r.Sublabel, tt.BodySmall.C(cs.OnSurfaceVariant)), 1).FontFeatures("tnum")
				}
				m3.VGap(c, design.SpaceXS+2)
				ui.Box(c).FillWidth().Height(4).Radius(design.CornerFull).Clip().Background(cs.SurfaceContainerHighest).Children(func() {
					ui.Box(c).FillHeight().WidthPercent(float32(min(max(r.Fraction, 0), 1)) * 100).Background(cs.Primary)
				})
			})
		})
	})
	return r.Tap && e.Clicked()
}
