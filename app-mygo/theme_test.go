package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/harukizmoe/mayu/theme"
)

// TestThemeUsesTheFlutterPalette is the colour half of "the two clients look
// the same": the window's scheme must be the values Flutter produced, not what
// Mayu's own generator would make from the same seed.
func TestThemeUsesTheFlutterPalette(t *testing.T) {
	for _, dark := range []bool{false, true} {
		var got *theme.Theme
		ui.NewTester(func(c *ui.Context) {
			if got == nil {
				got = useTheme(c, dark, false)
			}
		}, 400, 300)
		if got == nil {
			t.Fatal("the view never built")
		}
		p, err := loadPalette(dark)
		if err != nil {
			t.Fatal(err)
		}
		roles := []struct {
			name string
			got  ui.Color
			want ui.Color
		}{
			{"primary", got.Scheme.Primary, p.Scheme.Primary},
			{"secondary", got.Scheme.Secondary, p.Scheme.Secondary},
			{"secondaryContainer", got.Scheme.SecondaryContainer, p.Scheme.SecondaryContainer},
			{"error", got.Scheme.Error, p.Scheme.Error},
			{"surface", got.Scheme.Surface, p.Scheme.Surface},
			{"onSurface", got.Scheme.OnSurface, p.Scheme.OnSurface},
			{"onSurfaceVariant", got.Scheme.OnSurfaceVariant, p.Scheme.OnSurfaceVariant},
			{"outlineVariant", got.Scheme.OutlineVariant, p.Scheme.OutlineVariant},
			{"surfaceContainerLow", got.Scheme.SurfaceContainerLow, p.Scheme.SurfaceContainerLow},
			{"surfaceContainerHigh", got.Scheme.SurfaceContainerHigh, p.Scheme.SurfaceContainerHigh},
			{"surfaceContainerHighest", got.Scheme.SurfaceContainerHighest, p.Scheme.SurfaceContainerHighest},
		}
		for _, r := range roles {
			if r.got != r.want {
				t.Errorf("dark=%v %s: theme %v, palette %v", dark, r.name, r.got, r.want)
			}
		}
	}
}

// TestPaletteHasEveryRole checks the exported file carries every role the
// client asks for, so a Flutter upgrade that adds one fails here rather than
// panicking in a window.
func TestPaletteHasEveryRole(t *testing.T) {
	for _, dark := range []bool{false, true} {
		p, err := loadPalette(dark)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{
			"primary", "onPrimary", "primaryContainer", "onPrimaryContainer",
			"secondary", "onSecondary", "secondaryContainer", "onSecondaryContainer",
			"tertiary", "onTertiary", "tertiaryContainer", "onTertiaryContainer",
			"error", "onError", "errorContainer", "onErrorContainer",
			"background", "onBackground", "surface", "onSurface",
			"surfaceVariant", "onSurfaceVariant", "outline", "outlineVariant",
			"shadow", "scrim", "inverseSurface", "inverseOnSurface",
			"surfaceContainerLowest", "surfaceContainerLow", "surfaceContainer",
			"surfaceContainerHigh", "surfaceContainerHighest",
		} {
			if _, ok := p.Roles[name]; !ok {
				t.Errorf("dark=%v palette lacks %s", dark, name)
			}
		}
	}
}
