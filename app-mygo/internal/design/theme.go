package design

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// CJKFallback is theme.dart's _cjkFallback, the families text falls back to
// after the chosen one.
var CJKFallback = []string{"HarmonyOS Sans SC", "Noto Sans CJK SC", "Source Han Sans SC", "PingFang SC", "Microsoft YaHei"}

// Theme is the ThemeData buildTheme makes: the colour scheme, the text
// theme and the preferences that change how components look.
type Theme struct {
	Scheme *Scheme
	Text   TextTheme
	Dark   bool

	// Glass is GlassStyle.enabled: floating chrome is frosted glass.
	Glass bool
	// Compact is VisualDensity.compact.
	Compact bool
	// ReduceMotion is MediaQuery.disableAnimations.
	ReduceMotion bool
	// TextScale is the user's TextScaler.linear factor.
	TextScale float32
	// Font is the family list text draws with.
	Font string

	ui *ui.Theme
}

var current = &Theme{TextScale: 1}

// T is the theme being built with, Theme.of(context).
func T() *Theme { return current }

// S is the colour scheme being built with, Theme.of(context).colorScheme.
func S() *Scheme { return current.Scheme }

// TT is the text theme being built with.
func TT() *TextTheme { return &current.Text }

// Use makes t the theme for what is built until the returned function
// restores the one before it, as a Theme widget does for its subtree.
func Use(t *Theme) (restore func()) {
	prev := current
	current = t
	return func() { current = prev }
}

// SystemSeeds turns the desktop's accent into the light and dark seeds the
// app takes from DynamicColorBuilder: dynamic_color makes a scheme of the
// accent in each brightness and the app seeds its own from their primary.
func SystemSeeds(accent ui.Color) (light, dark ui.Color) {
	return FromSeed(accent, TonalSpot, false, 0).Primary, FromSeed(accent, TonalSpot, true, 0).Primary
}

// NewTheme is buildScheme and buildTheme for one brightness. seed is the
// seed to use when the user chose dynamic colour and the desktop has an
// accent; otherwise the preference's seed.
func NewTheme(p Prefs, dark bool, systemSeed ui.Color) *Theme {
	seed := ARGB(p.Seed)
	if p.DynamicColor && systemSeed.A != 0 {
		seed = systemSeed
	}
	s := FromSeed(seed, p.Variant, dark, p.Contrast.Value())
	font := "Roboto"
	if p.FontFamily != "" {
		font = p.FontFamily
	}
	for _, f := range CJKFallback {
		font += ", " + f
	}
	t := &Theme{
		Scheme:       s,
		Text:         NewTextTheme(s),
		Dark:         dark,
		Glass:        p.Glass && p.Contrast != ContrastHigh,
		Compact:      p.CompactDensity,
		ReduceMotion: p.ReduceMotion,
		TextScale:    float32(p.TextScale),
		Font:         font,
	}
	t.ui = t.widgetTheme()
	return t
}

// UI is the theme MyGo's own widgets (text inputs, scroll bars, the focus
// ring, selections) take, in the scheme's colours.
func (t *Theme) UI() *ui.Theme { return t.ui }

func (t *Theme) widgetTheme() *ui.Theme {
	s := t.Scheme
	var w *ui.Theme
	if t.Dark {
		w = ui.DarkTheme()
	} else {
		w = ui.LightTheme()
	}
	w.Dark = t.Dark
	w.Background = s.Surface
	w.Surface = s.SurfaceContainerHigh
	w.SurfaceHover = AlphaBlend(WithOpacity(s.OnSurface, 0.08), s.SurfaceContainerHigh)
	w.SurfacePressed = AlphaBlend(WithOpacity(s.OnSurface, 0.1), s.SurfaceContainerHigh)
	w.Border = s.OutlineVariant
	w.Text = s.OnSurface
	w.TextMuted = s.OnSurfaceVariant
	w.Accent = s.Primary
	w.AccentHover = AlphaBlend(WithOpacity(s.OnPrimary, 0.08), s.Primary)
	w.AccentPressed = AlphaBlend(WithOpacity(s.OnPrimary, 0.1), s.Primary)
	w.AccentText = s.OnPrimary
	w.InactiveAccent = s.Primary
	w.Danger = s.Error
	w.Warning = s.Tertiary
	w.Success = s.Primary
	// TextSelectionThemeData's default: the primary colour at 40%.
	w.Selection = WithOpacity(s.Primary, 0.4)
	// Material draws focus as a state layer and an outline of its own, not
	// a ring around the control.
	w.Focus = ui.Transparent
	w.Inverse = s.InverseSurface
	w.InverseText = s.OnInverseSurface
	w.Scrollbar = WithOpacity(s.OnSurface, 0.38)
	w.FontSize = 14 * t.TextScale
	w.Font = t.Font
	w.TooltipDelay = 500 * time.Millisecond
	w.DisableAutoDimming = true
	return w
}

// StateLayer is the colour of a Material state layer: hover 8%, focus and
// press 10%, of the content colour over the container.
func StateLayer(container, content ui.Color, hovered, focused, pressed bool) ui.Color {
	switch {
	case pressed:
		return AlphaBlend(WithOpacity(content, 0.1), container)
	case focused:
		return AlphaBlend(WithOpacity(content, 0.1), container)
	case hovered:
		return AlphaBlend(WithOpacity(content, 0.08), container)
	}
	return container
}

// Motion returns a for animations, or an instant change when the user turned
// animations off.
func (t *Theme) Motion(d time.Duration, ease ui.Easing) ui.Animation {
	if t.ReduceMotion {
		return ui.Animation{Duration: time.Nanosecond, Ease: ui.Linear}
	}
	return ui.Animation{Duration: d, Ease: ease}
}
