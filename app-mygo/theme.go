package main

import (
	"github.com/egoist/mygo/ui"
	"github.com/harukizmoe/mayu"
	"github.com/harukizmoe/mayu/theme"
)

// seed is Coursive's default seed colour, 湛蓝, from the Flutter client's
// preferences (app/lib/theme/prefs.dart). The palette itself is not generated
// from it here: design/palette.json carries the values Flutter produced.
var seed = ui.Hex("#3F6CB4")

// useTheme gives the frame Coursive's palette and the Flutter client's metrics.
//
// Mayu registers its theme in the context so its components find it, but its
// own HCT generator does not produce the same numbers as Flutter's. The theme
// it registers is therefore replaced with the exported palette before anything
// draws, and MyGo's own theme is set from the same values, so a control from
// Mayu and a plain ui control agree with the Flutter client pixel for pixel
// wherever the engine allows.
func useTheme(c *ui.Context, dark, compact bool) *theme.Theme {
	density := theme.DensityStandard
	if compact {
		density = theme.DensityCompact
	}
	t := mayu.Use(c, mayu.ThemeOptions{SeedColor: seed, Dark: dark, Density: density})
	p, err := loadPalette(dark)
	if err != nil {
		// A broken palette is a build mistake, not a runtime condition: the
		// parity test in app/ catches it before this can happen.
		panic(err)
	}
	t.Scheme = p.Scheme
	c.SetTheme(&ui.Theme{
		Dark:           dark,
		Background:     p.Scheme.Background,
		Surface:        p.Scheme.SurfaceContainer,
		SurfaceHover:   p.Scheme.SurfaceContainerHigh,
		SurfacePressed: p.Scheme.SurfaceContainerHighest,
		Border:         p.Scheme.OutlineVariant,
		Text:           p.Scheme.OnSurface,
		TextMuted:      p.Scheme.OnSurfaceVariant,
		Accent:         p.Scheme.Primary,
		AccentHover:    p.Scheme.PrimaryContainer,
		AccentPressed:  p.Scheme.PrimaryContainer,
		AccentText:     p.Scheme.OnPrimary,
		Danger:         p.Scheme.Error,
		Warning:        ui.Hex("#FFFF9800"),
		Success:        ui.Hex("#FF4CAF50"),
		Selection:      p.Scheme.PrimaryContainer,
		Focus:          p.Scheme.Primary,
		Inverse:        p.Scheme.InverseSurface,
		InverseText:    p.Scheme.InverseOnSurface,
		// Buttons are StadiumBorder in the Flutter client, so the radius is the
		// full control height rather than a corner of the shape scale.
		Radius:   density.ControlHeight() / 2,
		Spacing:  density.SpacingUnit(),
		FontSize: density.BaseFontSize(),
	})
	return t
}

// textStyle applies one step of Coursive's type scale to an element.
func textStyle(e ui.Element, s Type) ui.Element {
	e.FontSize(s.Size)
	if s.Weight >= 600 {
		e.Bold()
	}
	return e
}
