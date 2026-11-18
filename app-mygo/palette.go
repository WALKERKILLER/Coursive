package main

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/egoist/mygo/ui"
	"github.com/harukizmoe/mayu/theme"
)

// paletteJSON is Flutter's own ColorScheme for Coursive's seed, written by
// app/test/design_system_export_test.dart. The client loads it instead of
// regenerating the palette, because two HCT implementations that agree to
// within a channel or two still disagree: pixel parity needs the same numbers,
// not the same algorithm.
//
//go:embed design/palette.json
var paletteJSON []byte

type paletteDoc struct {
	Seed  string            `json:"seed"`
	Light map[string]string `json:"light"`
	Dark  map[string]string `json:"dark"`
}

// argb reads the "#AARRGGBB" form Flutter writes. ui.Hex takes "#RRGGBBAA",
// so the channels are read here rather than passed through.
func argb(hex string) ui.Color {
	var v uint32
	if _, err := fmt.Sscanf(hex, "#%08X", &v); err != nil {
		panic("palette: bad colour " + hex)
	}
	return ui.Color{
		A: uint8(v >> 24),
		R: uint8(v >> 16),
		G: uint8(v >> 8),
		B: uint8(v),
	}
}

// Palette is Coursive's colour scheme for one brightness, with every role the
// Flutter client uses.
type Palette struct {
	Doc    paletteDoc
	Roles  map[string]ui.Color
	Scheme theme.ColorScheme
}

// loadPalette reads the exported palette and builds the Mayu scheme from it.
func loadPalette(dark bool) (*Palette, error) {
	var doc paletteDoc
	if err := json.Unmarshal(paletteJSON, &doc); err != nil {
		return nil, fmt.Errorf("palette: %w", err)
	}
	roles := doc.Light
	if dark {
		roles = doc.Dark
	}
	colors := make(map[string]ui.Color, len(roles))
	for name, hex := range roles {
		colors[name] = argb(hex)
	}
	need := func(name string) ui.Color {
		c, ok := colors[name]
		if !ok {
			panic("palette: missing role " + name)
		}
		return c
	}
	scheme := theme.ColorScheme{
		Dark: dark,

		Primary:            need("primary"),
		OnPrimary:          need("onPrimary"),
		PrimaryContainer:   need("primaryContainer"),
		OnPrimaryContainer: need("onPrimaryContainer"),

		Secondary:            need("secondary"),
		OnSecondary:          need("onSecondary"),
		SecondaryContainer:   need("secondaryContainer"),
		OnSecondaryContainer: need("onSecondaryContainer"),

		Tertiary:            need("tertiary"),
		OnTertiary:          need("onTertiary"),
		TertiaryContainer:   need("tertiaryContainer"),
		OnTertiaryContainer: need("onTertiaryContainer"),

		Error:            need("error"),
		OnError:          need("onError"),
		ErrorContainer:   need("errorContainer"),
		OnErrorContainer: need("onErrorContainer"),

		Background:       need("background"),
		OnBackground:     need("onBackground"),
		Surface:          need("surface"),
		OnSurface:        need("onSurface"),
		SurfaceVariant:   need("surfaceVariant"),
		OnSurfaceVariant: need("onSurfaceVariant"),
		Outline:          need("outline"),
		OutlineVariant:   need("outlineVariant"),
		Shadow:           need("shadow"),
		Scrim:            need("scrim"),
		InverseSurface:   need("inverseSurface"),
		InverseOnSurface: need("inverseOnSurface"),

		SurfaceContainerLowest:  need("surfaceContainerLowest"),
		SurfaceContainerLow:     need("surfaceContainerLow"),
		SurfaceContainer:        need("surfaceContainer"),
		SurfaceContainerHigh:    need("surfaceContainerHigh"),
		SurfaceContainerHighest: need("surfaceContainerHighest"),
	}
	return &Palette{Doc: doc, Roles: colors, Scheme: scheme}, nil
}
