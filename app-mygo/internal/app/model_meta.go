package app

import (
	"math"
	"strconv"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
)

// model_meta.dart: token counts, prices and a model's capability chips.

// FmtTokens is fmtTokens: 128K, 1.5M.
func FmtTokens(n float64) string {
	switch {
	case n <= 0:
		return ""
	case n >= 1e6:
		d := 1
		if math.Mod(n, 1e6) == 0 {
			d = 0
		}
		return strconv.FormatFloat(n/1e6, 'f', d, 64) + "M"
	case n >= 1000:
		return strconv.Itoa(int(math.Round(n/1000))) + "K"
	}
	return strconv.FormatFloat(n, 'f', -1, 64)
}

// FmtPrice is fmtPrice: a price per million tokens.
func FmtPrice(v any) string {
	if v == nil {
		return ""
	}
	f := core.Float(v)
	switch {
	case f == 0:
		return "0"
	case f < .1:
		return strconv.FormatFloat(f, 'f', 3, 64)
	case f < 10:
		return strconv.FormatFloat(f, 'f', 2, 64)
	}
	return strconv.FormatFloat(f, 'f', 0, 64)
}

// ModelBadges are a model's capability and limit chips (models.dev and Pi
// catalog fields).
func ModelBadges(c *ui.Context, m core.J, dense bool) {
	cs := design.S()
	type badge struct {
		icon   icons.Icon
		text   string
		accent bool
	}
	var items []badge
	if m.IsTrue("reasoning") {
		items = append(items, badge{icons.PsychologyOutlined, l10n.ModelTagReasoning, true})
	}
	if m.IsTrue("vision") {
		items = append(items, badge{icons.ImageOutlined, l10n.ModelTagVision, true})
	}
	if ctx := m.F("context_window"); ctx > 0 {
		items = append(items, badge{icons.DataArrayRounded, l10n.ModelTagContext(FmtTokens(ctx)), false})
	}
	if out := m.F("max_tokens"); !dense && out > 0 {
		items = append(items, badge{icons.OutputRounded, l10n.ModelTagOutput(FmtTokens(out)), false})
	}
	cin, cout := m["cost_in"], m["cost_out"]
	if cin != nil && cout != nil && (core.Float(cin) > 0 || core.Float(cout) > 0) {
		items = append(items, badge{icons.PaymentsOutlined, "$" + FmtPrice(cin) + " / $" + FmtPrice(cout), false})
	}
	if cin != nil && cout != nil && core.Float(cin) == 0 && core.Float(cout) == 0 {
		items = append(items, badge{icons.RedeemOutlined, l10n.ModelTagFree, true})
	}
	if m.IsTrue("deprecated") {
		items = append(items, badge{icons.HistoryToggleOffRounded, l10n.ModelTagDeprecated, false})
	}
	ui.Row(c).Wrap().Gap(4).Children(func() {
		for _, b := range items {
			bg, fg := cs.SurfaceContainerHighest, cs.OnSurfaceVariant
			if b.accent {
				bg, fg = cs.SecondaryContainer, cs.OnSecondaryContainer
			}
			ui.Row(c).AlignItems(ui.Center).Gap(3).Padding(2, 6).Background(bg).Radius(design.CornerXS).Children(func() {
				m3.Icon(c, b.icon, 12, fg)
				m3.Text(c, b.text, design.TT().LabelSmall.C(fg))
			})
		}
	})
}
