package mcu

import "math"

func rotatedHue(source Hct, hues, rotations []float64) float64 {
	sourceHue := source.Hue
	if len(rotations) == 1 {
		return sanitizeDegreesDouble(source.Hue + rotations[0])
	}
	for i := 0; i <= len(hues)-2; i++ {
		if hues[i] < sourceHue && sourceHue < hues[i+1] {
			return sanitizeDegreesDouble(sourceHue + rotations[i])
		}
	}
	return sourceHue
}

// NewDynamicScheme builds the scheme Flutter's _buildDynamicScheme does.
func NewDynamicScheme(seed uint32, v Variant, dark bool, contrast float64) *DynamicScheme {
	src := HctFromInt(seed)
	s := &DynamicScheme{Source: src, Variant: v, IsDark: dark, ContrastLevel: contrast, ErrorPalette: PaletteOf(25.0, 84.0)}
	h := src.Hue
	switch v {
	case TonalSpot:
		s.Primary = PaletteOf(h, 36.0)
		s.Secondary = PaletteOf(h, 16.0)
		s.Tertiary = PaletteOf(sanitizeDegreesDouble(h+60.0), 24.0)
		s.Neutral = PaletteOf(h, 6.0)
		s.NeutralVariant = PaletteOf(h, 8.0)
	case Neutral:
		s.Primary = PaletteOf(h, 12.0)
		s.Secondary = PaletteOf(h, 8.0)
		s.Tertiary = PaletteOf(h, 16.0)
		s.Neutral = PaletteOf(h, 2.0)
		s.NeutralVariant = PaletteOf(h, 2.0)
	case Vibrant:
		hues := []float64{0, 41, 61, 101, 131, 181, 251, 301, 360}
		s.Primary = PaletteOf(h, 200.0)
		s.Secondary = PaletteOf(rotatedHue(src, hues, []float64{18, 15, 10, 12, 15, 18, 15, 12, 12}), 24.0)
		s.Tertiary = PaletteOf(rotatedHue(src, hues, []float64{35, 30, 20, 25, 30, 35, 30, 25, 25}), 32.0)
		s.Neutral = PaletteOf(h, 10.0)
		s.NeutralVariant = PaletteOf(h, 12.0)
	case Expressive:
		hues := []float64{0, 21, 51, 121, 151, 191, 271, 321, 360}
		s.Primary = PaletteOf(sanitizeDegreesDouble(h+240.0), 40.0)
		s.Secondary = PaletteOf(rotatedHue(src, hues, []float64{45, 95, 45, 20, 45, 90, 45, 45, 45}), 24.0)
		s.Tertiary = PaletteOf(rotatedHue(src, hues, []float64{120, 120, 20, 45, 20, 15, 20, 120, 120}), 32.0)
		s.Neutral = PaletteOf(h+15.0, 8.0)
		s.NeutralVariant = PaletteOf(h+15.0, 12.0)
	case Fidelity, Content:
		s.Primary = PaletteOf(h, src.Chroma)
		s.Secondary = PaletteOf(h, math.Max(src.Chroma-32.0, src.Chroma*0.5))
		tc := newTemperatureCache(src)
		if v == Fidelity {
			s.Tertiary = PaletteFromHct(fixIfDisliked(tc.complement()))
		} else {
			a := tc.analogous(3, 6)
			s.Tertiary = PaletteFromHct(fixIfDisliked(a[len(a)-1]))
		}
		s.Neutral = PaletteOf(h, src.Chroma/8.0)
		s.NeutralVariant = PaletteOf(h, src.Chroma/8.0+4.0)
	case Monochrome:
		s.Primary = PaletteOf(h, 0.0)
		s.Secondary = PaletteOf(h, 0.0)
		s.Tertiary = PaletteOf(h, 0.0)
		s.Neutral = PaletteOf(h, 0.0)
		s.NeutralVariant = PaletteOf(h, 0.0)
	}
	return s
}

// Scheme is Flutter's ColorScheme, every role as ARGB.
type Scheme struct {
	Primary, OnPrimary, PrimaryContainer, OnPrimaryContainer                     uint32
	PrimaryFixed, PrimaryFixedDim, OnPrimaryFixed, OnPrimaryFixedVariant         uint32
	Secondary, OnSecondary, SecondaryContainer, OnSecondaryContainer             uint32
	SecondaryFixed, SecondaryFixedDim, OnSecondaryFixed, OnSecondaryFixedVariant uint32
	Tertiary, OnTertiary, TertiaryContainer, OnTertiaryContainer                 uint32
	TertiaryFixed, TertiaryFixedDim, OnTertiaryFixed, OnTertiaryFixedVariant     uint32
	Error, OnError, ErrorContainer, OnErrorContainer                             uint32
	Outline, OutlineVariant                                                      uint32
	Surface, OnSurface, SurfaceDim, SurfaceBright                                uint32
	SurfaceContainerLowest, SurfaceContainerLow, SurfaceContainer                uint32
	SurfaceContainerHigh, SurfaceContainerHighest, OnSurfaceVariant              uint32
	InverseSurface, OnInverseSurface, InversePrimary                             uint32
	Shadow, Scrim, SurfaceTint                                                   uint32
}

// FromSeed is ColorScheme.fromSeed.
func FromSeed(seed uint32, v Variant, dark bool, contrast float64) Scheme {
	s := NewDynamicScheme(seed, v, dark, contrast)
	g := func(d *dynamicColor) uint32 { return d.argb(s) }
	return Scheme{
		Primary: g(mdcPrimary), OnPrimary: g(mdcOnPrimary),
		PrimaryContainer: g(mdcPrimaryContainer), OnPrimaryContainer: g(mdcOnPrimaryContainer),
		PrimaryFixed: g(mdcPrimaryFixed), PrimaryFixedDim: g(mdcPrimaryFixedDim),
		OnPrimaryFixed: g(mdcOnPrimaryFixed), OnPrimaryFixedVariant: g(mdcOnPrimaryFixedVariant),
		Secondary: g(mdcSecondary), OnSecondary: g(mdcOnSecondary),
		SecondaryContainer: g(mdcSecondaryContainer), OnSecondaryContainer: g(mdcOnSecondaryContainer),
		SecondaryFixed: g(mdcSecondaryFixed), SecondaryFixedDim: g(mdcSecondaryFixedDim),
		OnSecondaryFixed: g(mdcOnSecondaryFixed), OnSecondaryFixedVariant: g(mdcOnSecondaryFixedVariant),
		Tertiary: g(mdcTertiary), OnTertiary: g(mdcOnTertiary),
		TertiaryContainer: g(mdcTertiaryContainer), OnTertiaryContainer: g(mdcOnTertiaryContainer),
		TertiaryFixed: g(mdcTertiaryFixed), TertiaryFixedDim: g(mdcTertiaryFixedDim),
		OnTertiaryFixed: g(mdcOnTertiaryFixed), OnTertiaryFixedVariant: g(mdcOnTertiaryFixedVariant),
		Error: g(mdcError), OnError: g(mdcOnError), ErrorContainer: g(mdcErrorContainer), OnErrorContainer: g(mdcOnErrorContainer),
		Outline: g(mdcOutline), OutlineVariant: g(mdcOutlineVariant),
		Surface: g(mdcSurface), OnSurface: g(mdcOnSurface), SurfaceDim: g(mdcSurfaceDim), SurfaceBright: g(mdcSurfaceBright),
		SurfaceContainerLowest: g(mdcSurfaceContainerLowest), SurfaceContainerLow: g(mdcSurfaceContainerLow),
		SurfaceContainer: g(mdcSurfaceContainer), SurfaceContainerHigh: g(mdcSurfaceContainerHigh),
		SurfaceContainerHighest: g(mdcSurfaceContainerHighest), OnSurfaceVariant: g(mdcOnSurfaceVariant),
		InverseSurface: g(mdcInverseSurface), OnInverseSurface: g(mdcInverseOnSurface), InversePrimary: g(mdcInversePrimary),
		Shadow: g(mdcShadow), Scrim: g(mdcScrim), SurfaceTint: g(mdcPrimary),
	}
}
