package main

import (
	"github.com/egoist/mygo/ui"
	"github.com/harukizmoe/mayu"
	"github.com/harukizmoe/mayu/theme"
)

// seed is Coursive's default seed colour, 湛蓝, from the Flutter client's
// preferences (app/lib/theme/prefs.dart). The MyGo client takes its palette
// from the same seed so the two look like one product.
var seed = ui.Hex("#3F6CB4")

// useTheme gives the frame Mayu's Material 3 theme built from Coursive's seed,
// and with it MyGo's own theme, which the plain ui controls follow.
func useTheme(c *ui.Context, dark, compact bool) *theme.Theme {
	density := theme.DensityStandard
	if compact {
		density = theme.DensityCompact
	}
	return mayu.Use(c, mayu.ThemeOptions{SeedColor: seed, Dark: dark, Density: density})
}
