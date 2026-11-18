package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// thinking_control.dart: 思考强度.

// ThinkingLevels are Pi's thinking levels, weakest first.
var ThinkingLevels = [][2]string{
	{"off", l10n.CommonClose}, {"minimal", l10n.ThinkingMinimal}, {"low", l10n.ThinkingLow},
	{"medium", l10n.LevelMedium}, {"high", l10n.LevelHigh}, {"xhigh", l10n.ThinkingMax},
}

var thinkingHints = map[string]string{
	"off": l10n.ThinkingHintOff, "minimal": l10n.ThinkingHintMinimal, "low": l10n.ThinkingHintLow,
	"medium": l10n.ThinkingHintMedium, "high": l10n.ThinkingHintHigh, "xhigh": l10n.ThinkingHintMax,
}

// ThinkingLabel is the label of a thinking level, e.g. 中.
func ThinkingLabel(level string) string {
	for _, l := range ThinkingLevels {
		if l[0] == level {
			return l[1]
		}
	}
	return ThinkingLevels[3][1]
}

// ThinkingControl is 思考强度 as a stepped slider with a one-line hint for
// the chosen level. onContainer adapts the colours to a primary container
// background.
func ThinkingControl(c *ui.Context, value string, onChanged func(string), onContainer, showTitle bool) {
	cs := design.S()
	tt := design.TT()
	fg, muted := cs.OnSurface, cs.OnSurfaceVariant
	var inactive ui.Color
	if onContainer {
		fg, muted = cs.OnPrimaryContainer, design.WithOpacity(cs.OnPrimaryContainer, .72)
		inactive = design.WithOpacity(cs.OnPrimaryContainer, .1)
	}
	labels := make([]string, len(ThinkingLevels))
	idx := 3
	for i, l := range ThinkingLevels {
		labels[i] = l[1]
		if l[0] == value {
			idx = i
		}
	}
	ui.Column(c).FillWidth().Children(func() {
		if showTitle {
			m3.Text(c, l10n.ThinkingTitle, tt.LabelLarge.C(fg))
			m3.VGap(c, design.SpaceXS)
		}
		if (widgets.StepSlider{Labels: labels, Label: l10n.ThinkingTitle, Disabled: onChanged == nil, Inactive: inactive, LabelColor: muted}).Show(c, &idx) {
			onChanged(ThinkingLevels[idx][0])
		}
		m3.VGap(c, design.SpaceSM)
		m3.Text(c.Key(value), thinkingHints[value], tt.BodySmall.C(muted)).
			Transition(ui.ElementTransition{Duration: design.Short3, Enter: &ui.Motion{Opacity: 0}})
	})
}

// ShowThinkingSheet is the phone variant of the composer's 思考强度
// entry: a bottom sheet.
func ShowThinkingSheet() {
	m3.ShowModalBottomSheet(m3.SheetOptions{}, func(c *ui.Context, r *m3.Route) {
		a := Assistant
		ui.Column(c).FillWidth().Padding(0, design.SpaceXL, design.SpaceXL, design.SpaceXL).Children(func() {
			m3.Text(c, l10n.ThinkingTitle, design.TT().TitleLarge)
			m3.VGap(c, design.SpaceXS)
			m3.Text(c, l10n.ThinkingSheetBody, design.TT().BodyMedium.C(design.S().OnSurfaceVariant))
			m3.VGap(c, design.SpaceXL)
			ThinkingControl(c, a.Thinking(), setThinking, false, false)
		})
	})
}

func setThinking(k string) { Assistant.SetPrefs(core.J{"thinking": k}, nil) }
