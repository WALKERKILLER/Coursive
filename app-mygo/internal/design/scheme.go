package design

import (
	"math"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design/mcu"
)

// Scheme is Flutter's ColorScheme: every role, as ColorScheme.fromSeed
// computes it.
type Scheme struct {
	Dark bool

	Primary, OnPrimary, PrimaryContainer, OnPrimaryContainer                     ui.Color
	PrimaryFixed, PrimaryFixedDim, OnPrimaryFixed, OnPrimaryFixedVariant         ui.Color
	Secondary, OnSecondary, SecondaryContainer, OnSecondaryContainer             ui.Color
	SecondaryFixed, SecondaryFixedDim, OnSecondaryFixed, OnSecondaryFixedVariant ui.Color
	Tertiary, OnTertiary, TertiaryContainer, OnTertiaryContainer                 ui.Color
	TertiaryFixed, TertiaryFixedDim, OnTertiaryFixed, OnTertiaryFixedVariant     ui.Color
	Error, OnError, ErrorContainer, OnErrorContainer                             ui.Color
	Outline, OutlineVariant                                                      ui.Color
	Surface, OnSurface, SurfaceDim, SurfaceBright                                ui.Color
	SurfaceContainerLowest, SurfaceContainerLow, SurfaceContainer                ui.Color
	SurfaceContainerHigh, SurfaceContainerHighest, OnSurfaceVariant              ui.Color
	InverseSurface, OnInverseSurface, InversePrimary                             ui.Color
	Shadow, Scrim, SurfaceTint                                                   ui.Color
}

// Variant is DynamicSchemeVariant.
type Variant = mcu.Variant

const (
	TonalSpot  = mcu.TonalSpot
	Neutral    = mcu.Neutral
	Vibrant    = mcu.Vibrant
	Expressive = mcu.Expressive
	Fidelity   = mcu.Fidelity
	Content    = mcu.Content
	Monochrome = mcu.Monochrome
)

// ARGB is a colour from Flutter's 0xAARRGGBB.
func ARGB(v uint32) ui.Color {
	return ui.Color{A: uint8(v >> 24), R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v)}
}

// ToARGB is the colour as Flutter's 0xAARRGGBB.
func ToARGB(c ui.Color) uint32 {
	return uint32(c.A)<<24 | uint32(c.R)<<16 | uint32(c.G)<<8 | uint32(c.B)
}

type schemeKey struct {
	seed     uint32
	variant  Variant
	dark     bool
	contrast float64
}

var schemes = map[schemeKey]*Scheme{}

// FromSeed is ColorScheme.fromSeed. Schemes are kept, as the view asks for
// the same one every frame.
func FromSeed(seed ui.Color, v Variant, dark bool, contrast float64) *Scheme {
	k := schemeKey{ToARGB(seed) | 0xFF000000, v, dark, contrast}
	if s, ok := schemes[k]; ok {
		return s
	}
	m := mcu.FromSeed(k.seed, v, dark, contrast)
	s := &Scheme{
		Dark:    dark,
		Primary: ARGB(m.Primary), OnPrimary: ARGB(m.OnPrimary),
		PrimaryContainer: ARGB(m.PrimaryContainer), OnPrimaryContainer: ARGB(m.OnPrimaryContainer),
		PrimaryFixed: ARGB(m.PrimaryFixed), PrimaryFixedDim: ARGB(m.PrimaryFixedDim),
		OnPrimaryFixed: ARGB(m.OnPrimaryFixed), OnPrimaryFixedVariant: ARGB(m.OnPrimaryFixedVariant),
		Secondary: ARGB(m.Secondary), OnSecondary: ARGB(m.OnSecondary),
		SecondaryContainer: ARGB(m.SecondaryContainer), OnSecondaryContainer: ARGB(m.OnSecondaryContainer),
		SecondaryFixed: ARGB(m.SecondaryFixed), SecondaryFixedDim: ARGB(m.SecondaryFixedDim),
		OnSecondaryFixed: ARGB(m.OnSecondaryFixed), OnSecondaryFixedVariant: ARGB(m.OnSecondaryFixedVariant),
		Tertiary: ARGB(m.Tertiary), OnTertiary: ARGB(m.OnTertiary),
		TertiaryContainer: ARGB(m.TertiaryContainer), OnTertiaryContainer: ARGB(m.OnTertiaryContainer),
		TertiaryFixed: ARGB(m.TertiaryFixed), TertiaryFixedDim: ARGB(m.TertiaryFixedDim),
		OnTertiaryFixed: ARGB(m.OnTertiaryFixed), OnTertiaryFixedVariant: ARGB(m.OnTertiaryFixedVariant),
		Error: ARGB(m.Error), OnError: ARGB(m.OnError),
		ErrorContainer: ARGB(m.ErrorContainer), OnErrorContainer: ARGB(m.OnErrorContainer),
		Outline: ARGB(m.Outline), OutlineVariant: ARGB(m.OutlineVariant),
		Surface: ARGB(m.Surface), OnSurface: ARGB(m.OnSurface),
		SurfaceDim: ARGB(m.SurfaceDim), SurfaceBright: ARGB(m.SurfaceBright),
		SurfaceContainerLowest: ARGB(m.SurfaceContainerLowest), SurfaceContainerLow: ARGB(m.SurfaceContainerLow),
		SurfaceContainer: ARGB(m.SurfaceContainer), SurfaceContainerHigh: ARGB(m.SurfaceContainerHigh),
		SurfaceContainerHighest: ARGB(m.SurfaceContainerHighest), OnSurfaceVariant: ARGB(m.OnSurfaceVariant),
		InverseSurface: ARGB(m.InverseSurface), OnInverseSurface: ARGB(m.OnInverseSurface),
		InversePrimary: ARGB(m.InversePrimary),
		Shadow:         ARGB(m.Shadow), Scrim: ARGB(m.Scrim), SurfaceTint: ARGB(m.SurfaceTint),
	}
	schemes[k] = s
	return s
}

// WithOpacity is Color.withValues(alpha: a): the colour at that opacity.
func WithOpacity(c ui.Color, a float32) ui.Color {
	if a < 0 {
		a = 0
	}
	if a > 1 {
		a = 1
	}
	c.A = uint8(a*255 + 0.5)
	return c
}

// Lerp is Color.lerp, channel by channel in sRGB.
func Lerp(a, b ui.Color, t float32) ui.Color {
	l := func(x, y uint8) uint8 {
		v := float32(x) + (float32(y)-float32(x))*t
		if v < 0 {
			v = 0
		}
		if v > 255 {
			v = 255
		}
		return uint8(v + 0.5)
	}
	return ui.Color{R: l(a.R, b.R), G: l(a.G, b.G), B: l(a.B, b.B), A: l(a.A, b.A)}
}

// AlphaBlend is Color.alphaBlend: fg over bg.
func AlphaBlend(fg, bg ui.Color) ui.Color {
	fa := float32(fg.A) / 255
	if fa == 0 {
		return bg
	}
	ba := float32(bg.A) / 255 * (1 - fa)
	a := fa + ba
	ch := func(f, b uint8) uint8 {
		return uint8((float32(f)*fa+float32(b)*ba)/a + 0.5)
	}
	return ui.Color{R: ch(fg.R, bg.R), G: ch(fg.G, bg.G), B: ch(fg.B, bg.B), A: uint8(a*255 + 0.5)}
}

// Luminance is Color.computeLuminance.
func Luminance(c ui.Color) float64 {
	lin := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= 0.03928 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

func abs(x float64) float64 { return math.Abs(x) }
func cos(x float64) float64 { return math.Cos(x) }
func sin(x float64) float64 { return math.Sin(x) }
func tan(x float64) float64 { return math.Tan(x) }
