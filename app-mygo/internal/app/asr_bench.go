package app

import (
	"math"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// BenchMetric is one measure on the board; shorter bars are always
// better.
type BenchMetric struct {
	Label, Hint string
	Format      func(float64) string
	// Source is where the numbers come from, shown in the footnote and
	// tooltips.
	Source string
}

// BenchRow is a model's values, one per metric; nil when it was not
// measured.
type BenchRow struct {
	ID, Name, Tag string
	Values        []*float64
}

// AsrBenchBoard is the benchmark board for choosing a model: one row per
// model and one small bar chart per measure, each with its own scale,
// sorted by the chosen measure. On narrow windows only the chosen measure
// is drawn.
type AsrBenchBoard struct {
	sort int
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

// Show builds the board; onSelect runs with a row's id.
func (b *AsrBenchBoard) Show(c *ui.Context, metrics []BenchMetric, rows []BenchRow, selected string, onSelect func(string)) {
	cs := design.S()
	tt := design.TT()
	sorted := append([]BenchRow(nil), rows...)
	val := func(r BenchRow, m int) float64 {
		if v := r.Values[m]; v != nil {
			return *v
		}
		return math.Inf(1)
	}
	sort.SliceStable(sorted, func(i, j int) bool { return val(sorted[i], b.sort) < val(sorted[j], b.sort) })
	maxes := make([]float64, len(metrics))
	for m := range metrics {
		top := 0.0
		for _, r := range rows {
			if v := r.Values[m]; v != nil {
				top = math.Max(top, *v)
			}
		}
		maxes[m] = niceMax(top)
	}
	root := ui.Column(c).FillWidth()
	w := widgets.Width(root, 640)
	wide := w >= 620
	shown := []int{b.sort}
	if wide {
		shown = shown[:0]
		for i := range metrics {
			shown = append(shown, i)
		}
	}
	nameW := float32(168)
	if !wide {
		nameW = min(140, w*.38)
	}
	root.Children(func() {
		ui.Row(c).FillWidth().Children(func() {
			var segs []m3.Segment[int]
			for i, m := range metrics {
				label := m.Label
				if wide {
					label = l10n.BenchSortBy(m.Label)
				}
				segs = append(segs, m3.Segment[int]{Value: i, Label: label})
			}
			m3.Segmented(c, segs, &b.sort, m3.SegOpts{NoCheck: true})
		})
		m3.VGap(c, design.SpaceMD)
		ui.Row(c).FillWidth().Children(func() {
			ui.Box(c).Width(nameW).Shrink(0)
			for _, m := range shown {
				ui.Column(c.Key(m)).Grow(1).Basis(0).Padding(0, design.SpaceMD, 0, 0).Children(func() {
					st := tt.LabelMedium.C(cs.OnSurfaceVariant)
					if m == b.sort {
						st = tt.LabelMedium.C(cs.OnSurface).W(600)
					}
					m3.Text(c, metrics[m].Label, st)
					m3.Text(c, metrics[m].Hint, tt.LabelSmall.C(cs.OnSurfaceVariant))
				})
			}
		})
		m3.VGap(c, design.SpaceXS)
		for _, r := range sorted {
			b.row(c, metrics, r, shown, maxes, nameW, r.ID == selected, onSelect)
		}
		m3.VGap(c, design.SpaceSM)
		for _, m := range shown {
			m3.Text(c.Key(m), l10n.BenchMetricSource(metrics[m].Label, metrics[m].Source), tt.LabelSmall.C(cs.OnSurfaceVariant).H(1.5))
		}
	})
}

func (b *AsrBenchBoard) row(c *ui.Context, metrics []BenchMetric, r BenchRow, shown []int, maxes []float64, nameW float32, sel bool, onSelect func(string)) {
	cs := design.S()
	tt := design.TT()
	var parts []string
	for m, met := range metrics {
		v := l10n.BenchUntested
		if r.Values[m] != nil {
			v = met.Format(*r.Values[m])
		}
		parts = append(parts, met.Label+" "+v)
	}
	summary := strings.Join(parts, l10n.ListComma)
	valueStyle := tt.LabelSmall.C(cs.OnSurface)
	e := ui.ButtonBase(c.Key("row:"+r.ID)).FocusRing(true).Cursor(ui.CursorPointer).Row().FillWidth().AlignItems(ui.Center).
		Margin(0, 0, design.SpaceXXS, 0).Padding(design.SpaceSM).Radius(design.CornerMD).Label(l10n.BenchRowLabel(r.Name, summary))
	bg := ui.Transparent
	switch {
	case sel:
		bg = cs.SecondaryContainer
	case e.Hovered():
		bg = cs.SurfaceContainerHighest
	}
	e = e.Background(bg).Transition(ui.ElementTransition{Duration: design.Short4, Ease: design.Standard, Colors: true})
	m3.Tip(c, e, r.Name+"\n"+summary)
	e.Children(func() {
		ui.Row(c).Width(nameW - design.SpaceSM).Shrink(0).AlignItems(ui.Center).Children(func() {
			st := tt.BodyMedium.C(cs.OnSurface).W(500)
			if sel {
				st = tt.BodyMedium.C(cs.OnSecondaryContainer).W(600)
			}
			m3.Clamp(m3.Text(c, r.Name, st), 1).Shrink(1)
			if r.Tag != "" {
				m3.HGap(c, design.SpaceXS)
				m3.Icon(c, icons.StarRounded, 14, cs.Primary)
			}
		})
		for _, m := range shown {
			cell := ui.Row(c.Key(m)).Grow(1).Basis(0).AlignItems(ui.Center).Padding(0, design.SpaceMD, 0, 0)
			cw := widgets.Width(cell, 120) - design.SpaceMD
			cell.Children(func() {
				v := r.Values[m]
				label, st := l10n.BenchUntested, valueStyle.C(cs.OnSurfaceVariant)
				if v != nil {
					label, st = metrics[m].Format(*v), valueStyle
				}
				bar := ui.Box(c).Height(10).Shrink(0).Background(cs.Primary).Radius(0, 4, 4, 0)
				m3.HGap(c, design.SpaceSM)
				lab := m3.Text(c, label, st)
				target := float32(0)
				if v != nil {
					track := max(0, cw-widgets.Width(lab, 32)-design.SpaceSM)
					target = max(4, track*float32(min(max(*v/maxes[m], 0), 1)))
				}
				d := design.Medium2
				if core.UI.ReduceMotion {
					d = 0
				}
				bar.Width(bar.AnimateTo("bench.w", target, design.T().Motion(d, design.Emphasized)))
			})
		}
	})
	if e.Clicked() && onSelect != nil {
		onSelect(r.ID)
	}
}
