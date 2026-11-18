package mcu

import "math"

// The roles of material_dynamic_colors.dart. They refer to each other, so
// init fills them in.
var (
	mdcBackground, mdcOnBackground, mdcSurface, mdcSurfaceDim, mdcSurfaceBright                *dynamicColor
	mdcSurfaceContainerLowest, mdcSurfaceContainerLow, mdcSurfaceContainer                     *dynamicColor
	mdcSurfaceContainerHigh, mdcSurfaceContainerHighest, mdcOnSurface, mdcSurfaceVariant       *dynamicColor
	mdcOnSurfaceVariant, mdcInverseSurface, mdcInverseOnSurface, mdcOutline, mdcOutlineVariant *dynamicColor
	mdcShadow, mdcScrim, mdcSurfaceTint                                                        *dynamicColor
	mdcPrimary, mdcOnPrimary, mdcPrimaryContainer, mdcOnPrimaryContainer, mdcInversePrimary    *dynamicColor
	mdcSecondary, mdcOnSecondary, mdcSecondaryContainer, mdcOnSecondaryContainer               *dynamicColor
	mdcTertiary, mdcOnTertiary, mdcTertiaryContainer, mdcOnTertiaryContainer                   *dynamicColor
	mdcError, mdcOnError, mdcErrorContainer, mdcOnErrorContainer                               *dynamicColor
	mdcPrimaryFixed, mdcPrimaryFixedDim, mdcOnPrimaryFixed, mdcOnPrimaryFixedVariant           *dynamicColor
	mdcSecondaryFixed, mdcSecondaryFixedDim, mdcOnSecondaryFixed, mdcOnSecondaryFixedVariant   *dynamicColor
	mdcTertiaryFixed, mdcTertiaryFixedDim, mdcOnTertiaryFixed, mdcOnTertiaryFixedVariant       *dynamicColor
)

func isFidelity(s *DynamicScheme) bool   { return s.Variant == Fidelity || s.Variant == Content }
func isMonochrome(s *DynamicScheme) bool { return s.Variant == Monochrome }

func pick(dark bool, d, l float64) float64 {
	if dark {
		return d
	}
	return l
}

func highestSurface(s *DynamicScheme) *dynamicColor {
	if s.IsDark {
		return mdcSurfaceBright
	}
	return mdcSurfaceDim
}

func neutralP(s *DynamicScheme) *TonalPalette        { return s.Neutral }
func neutralVariantP(s *DynamicScheme) *TonalPalette { return s.NeutralVariant }
func primaryP(s *DynamicScheme) *TonalPalette        { return s.Primary }
func secondaryP(s *DynamicScheme) *TonalPalette      { return s.Secondary }
func tertiaryP(s *DynamicScheme) *TonalPalette       { return s.Tertiary }
func errorP(s *DynamicScheme) *TonalPalette          { return s.ErrorPalette }

func is(c **dynamicColor) func(*DynamicScheme) *dynamicColor {
	return func(*DynamicScheme) *dynamicColor { return *c }
}

func pair(a, b **dynamicColor, polarity tonePolarity, stay bool) func(*DynamicScheme) toneDeltaPair {
	return func(*DynamicScheme) toneDeltaPair { return toneDeltaPair{*a, *b, 10, polarity, stay} }
}

func surfaceTone(dark, light *contrastCurve, darkFixed, lightFixed float64) func(*DynamicScheme) float64 {
	return func(s *DynamicScheme) float64 {
		if s.IsDark {
			if dark == nil {
				return darkFixed
			}
			return dark.get(s.ContrastLevel)
		}
		if light == nil {
			return lightFixed
		}
		return light.get(s.ContrastLevel)
	}
}

func init() {
	mdcBackground = &dynamicColor{name: "background", palette: neutralP, isBackground: true,
		tone: func(s *DynamicScheme) float64 { return pick(s.IsDark, 6, 98) }}
	mdcOnBackground = &dynamicColor{name: "on_background", palette: neutralP,
		tone: func(s *DynamicScheme) float64 { return pick(s.IsDark, 90, 10) }, background: is(&mdcBackground),
		contrastCurve: curve(3, 3, 4.5, 7)}
	mdcSurface = &dynamicColor{name: "surface", palette: neutralP, isBackground: true,
		tone: func(s *DynamicScheme) float64 { return pick(s.IsDark, 6, 98) }}
	mdcSurfaceDim = &dynamicColor{name: "surface_dim", palette: neutralP, isBackground: true,
		tone: surfaceTone(nil, curve(87, 87, 80, 75), 6, 0)}
	mdcSurfaceBright = &dynamicColor{name: "surface_bright", palette: neutralP, isBackground: true,
		tone: surfaceTone(curve(24, 24, 29, 34), nil, 0, 98)}
	mdcSurfaceContainerLowest = &dynamicColor{name: "surface_container_lowest", palette: neutralP, isBackground: true,
		tone: surfaceTone(curve(4, 4, 2, 0), nil, 0, 100)}
	mdcSurfaceContainerLow = &dynamicColor{name: "surface_container_low", palette: neutralP, isBackground: true,
		tone: surfaceTone(curve(10, 10, 11, 12), curve(96, 96, 96, 95), 0, 0)}
	mdcSurfaceContainer = &dynamicColor{name: "surface_container", palette: neutralP, isBackground: true,
		tone: surfaceTone(curve(12, 12, 16, 20), curve(94, 94, 92, 90), 0, 0)}
	mdcSurfaceContainerHigh = &dynamicColor{name: "surface_container_high", palette: neutralP, isBackground: true,
		tone: surfaceTone(curve(17, 17, 21, 25), curve(92, 92, 88, 85), 0, 0)}
	mdcSurfaceContainerHighest = &dynamicColor{name: "surface_container_highest", palette: neutralP, isBackground: true,
		tone: surfaceTone(curve(22, 22, 26, 30), curve(90, 90, 84, 80), 0, 0)}
	mdcOnSurface = &dynamicColor{name: "on_surface", palette: neutralP,
		tone: func(s *DynamicScheme) float64 { return pick(s.IsDark, 90, 10) }, background: highestSurface,
		contrastCurve: curve(4.5, 7, 11, 21)}
	mdcSurfaceVariant = &dynamicColor{name: "surface_variant", palette: neutralVariantP, isBackground: true,
		tone: func(s *DynamicScheme) float64 { return pick(s.IsDark, 30, 90) }}
	mdcOnSurfaceVariant = &dynamicColor{name: "on_surface_variant", palette: neutralVariantP,
		tone: func(s *DynamicScheme) float64 { return pick(s.IsDark, 80, 30) }, background: highestSurface,
		contrastCurve: curve(3, 4.5, 7, 11)}
	mdcInverseSurface = &dynamicColor{name: "inverse_surface", palette: neutralP,
		tone: func(s *DynamicScheme) float64 { return pick(s.IsDark, 90, 20) }}
	mdcInverseOnSurface = &dynamicColor{name: "inverse_on_surface", palette: neutralP,
		tone: func(s *DynamicScheme) float64 { return pick(s.IsDark, 20, 95) }, background: is(&mdcInverseSurface),
		contrastCurve: curve(4.5, 7, 11, 21)}
	mdcOutline = &dynamicColor{name: "outline", palette: neutralVariantP,
		tone: func(s *DynamicScheme) float64 { return pick(s.IsDark, 60, 50) }, background: highestSurface,
		contrastCurve: curve(1.5, 3, 4.5, 7)}
	mdcOutlineVariant = &dynamicColor{name: "outline_variant", palette: neutralVariantP,
		tone: func(s *DynamicScheme) float64 { return pick(s.IsDark, 30, 80) }, background: highestSurface,
		contrastCurve: curve(1, 1, 3, 4.5)}
	mdcShadow = &dynamicColor{name: "shadow", palette: neutralP, tone: func(*DynamicScheme) float64 { return 0 }}
	mdcScrim = &dynamicColor{name: "scrim", palette: neutralP, tone: func(*DynamicScheme) float64 { return 0 }}
	mdcSurfaceTint = &dynamicColor{name: "surface_tint", palette: primaryP, isBackground: true,
		tone: func(s *DynamicScheme) float64 { return pick(s.IsDark, 80, 40) }}

	mdcPrimary = &dynamicColor{name: "primary", palette: primaryP, isBackground: true,
		tone: func(s *DynamicScheme) float64 {
			if isMonochrome(s) {
				return pick(s.IsDark, 100, 0)
			}
			return pick(s.IsDark, 80, 40)
		},
		background: highestSurface, contrastCurve: curve(3, 4.5, 7, 7),
		toneDeltaPair: pair(&mdcPrimaryContainer, &mdcPrimary, nearer, false)}
	mdcOnPrimary = &dynamicColor{name: "on_primary", palette: primaryP,
		tone: func(s *DynamicScheme) float64 {
			if isMonochrome(s) {
				return pick(s.IsDark, 10, 90)
			}
			return pick(s.IsDark, 20, 100)
		},
		background: is(&mdcPrimary), contrastCurve: curve(4.5, 7, 11, 21)}
	mdcPrimaryContainer = &dynamicColor{name: "primary_container", palette: primaryP, isBackground: true,
		tone: func(s *DynamicScheme) float64 {
			if isFidelity(s) {
				return s.Source.Tone
			}
			if isMonochrome(s) {
				return pick(s.IsDark, 85, 25)
			}
			return pick(s.IsDark, 30, 90)
		},
		background: highestSurface, contrastCurve: curve(1, 1, 3, 4.5),
		toneDeltaPair: pair(&mdcPrimaryContainer, &mdcPrimary, nearer, false)}
	mdcOnPrimaryContainer = &dynamicColor{name: "on_primary_container", palette: primaryP,
		tone: func(s *DynamicScheme) float64 {
			if isFidelity(s) {
				return foregroundTone(mdcPrimaryContainer.tone(s), 4.5)
			}
			if isMonochrome(s) {
				return pick(s.IsDark, 0, 100)
			}
			return pick(s.IsDark, 90, 30)
		},
		background: is(&mdcPrimaryContainer), contrastCurve: curve(3, 4.5, 7, 11)}
	mdcInversePrimary = &dynamicColor{name: "inverse_primary", palette: primaryP,
		tone:       func(s *DynamicScheme) float64 { return pick(s.IsDark, 40, 80) },
		background: is(&mdcInverseSurface), contrastCurve: curve(3, 4.5, 7, 7)}

	mdcSecondary = &dynamicColor{name: "secondary", palette: secondaryP, isBackground: true,
		tone:       func(s *DynamicScheme) float64 { return pick(s.IsDark, 80, 40) },
		background: highestSurface, contrastCurve: curve(3, 4.5, 7, 7),
		toneDeltaPair: pair(&mdcSecondaryContainer, &mdcSecondary, nearer, false)}
	mdcOnSecondary = &dynamicColor{name: "on_secondary", palette: secondaryP,
		tone: func(s *DynamicScheme) float64 {
			if isMonochrome(s) {
				return pick(s.IsDark, 10, 100)
			}
			return pick(s.IsDark, 20, 100)
		},
		background: is(&mdcSecondary), contrastCurve: curve(4.5, 7, 11, 21)}
	mdcSecondaryContainer = &dynamicColor{name: "secondary_container", palette: secondaryP, isBackground: true,
		tone: func(s *DynamicScheme) float64 {
			initial := pick(s.IsDark, 30, 90)
			if isMonochrome(s) {
				return pick(s.IsDark, 30, 85)
			}
			if !isFidelity(s) {
				return initial
			}
			return findDesiredChromaByTone(s.Secondary.Hue, s.Secondary.Chroma, initial, !s.IsDark)
		},
		background: highestSurface, contrastCurve: curve(1, 1, 3, 4.5),
		toneDeltaPair: pair(&mdcSecondaryContainer, &mdcSecondary, nearer, false)}
	mdcOnSecondaryContainer = &dynamicColor{name: "on_secondary_container", palette: secondaryP,
		tone: func(s *DynamicScheme) float64 {
			if isMonochrome(s) {
				return pick(s.IsDark, 90, 10)
			}
			if !isFidelity(s) {
				return pick(s.IsDark, 90, 30)
			}
			return foregroundTone(mdcSecondaryContainer.tone(s), 4.5)
		},
		background: is(&mdcSecondaryContainer), contrastCurve: curve(3, 4.5, 7, 11)}

	mdcTertiary = &dynamicColor{name: "tertiary", palette: tertiaryP, isBackground: true,
		tone: func(s *DynamicScheme) float64 {
			if isMonochrome(s) {
				return pick(s.IsDark, 90, 25)
			}
			return pick(s.IsDark, 80, 40)
		},
		background: highestSurface, contrastCurve: curve(3, 4.5, 7, 7),
		toneDeltaPair: pair(&mdcTertiaryContainer, &mdcTertiary, nearer, false)}
	mdcOnTertiary = &dynamicColor{name: "on_tertiary", palette: tertiaryP,
		tone: func(s *DynamicScheme) float64 {
			if isMonochrome(s) {
				return pick(s.IsDark, 10, 90)
			}
			return pick(s.IsDark, 20, 100)
		},
		background: is(&mdcTertiary), contrastCurve: curve(4.5, 7, 11, 21)}
	mdcTertiaryContainer = &dynamicColor{name: "tertiary_container", palette: tertiaryP, isBackground: true,
		tone: func(s *DynamicScheme) float64 {
			if isMonochrome(s) {
				return pick(s.IsDark, 60, 49)
			}
			if !isFidelity(s) {
				return pick(s.IsDark, 30, 90)
			}
			return fixIfDisliked(s.Tertiary.getHct(s.Source.Tone)).Tone
		},
		background: highestSurface, contrastCurve: curve(1, 1, 3, 4.5),
		toneDeltaPair: pair(&mdcTertiaryContainer, &mdcTertiary, nearer, false)}
	mdcOnTertiaryContainer = &dynamicColor{name: "on_tertiary_container", palette: tertiaryP,
		tone: func(s *DynamicScheme) float64 {
			if isMonochrome(s) {
				return pick(s.IsDark, 0, 100)
			}
			if !isFidelity(s) {
				return pick(s.IsDark, 90, 30)
			}
			return foregroundTone(mdcTertiaryContainer.tone(s), 4.5)
		},
		background: is(&mdcTertiaryContainer), contrastCurve: curve(3, 4.5, 7, 11)}

	mdcError = &dynamicColor{name: "error", palette: errorP, isBackground: true,
		tone:       func(s *DynamicScheme) float64 { return pick(s.IsDark, 80, 40) },
		background: highestSurface, contrastCurve: curve(3, 4.5, 7, 7),
		toneDeltaPair: pair(&mdcErrorContainer, &mdcError, nearer, false)}
	mdcOnError = &dynamicColor{name: "on_error", palette: errorP,
		tone:       func(s *DynamicScheme) float64 { return pick(s.IsDark, 20, 100) },
		background: is(&mdcError), contrastCurve: curve(4.5, 7, 11, 21)}
	mdcErrorContainer = &dynamicColor{name: "error_container", palette: errorP, isBackground: true,
		tone:       func(s *DynamicScheme) float64 { return pick(s.IsDark, 30, 90) },
		background: highestSurface, contrastCurve: curve(1, 1, 3, 4.5),
		toneDeltaPair: pair(&mdcErrorContainer, &mdcError, nearer, false)}
	mdcOnErrorContainer = &dynamicColor{name: "on_error_container", palette: errorP,
		tone: func(s *DynamicScheme) float64 {
			if isMonochrome(s) {
				return pick(s.IsDark, 90, 10)
			}
			return pick(s.IsDark, 90, 30)
		},
		background: is(&mdcErrorContainer), contrastCurve: curve(3, 4.5, 7, 11)}

	fixed := func(name string, p func(*DynamicScheme) *TonalPalette, mono, normal float64, a, b **dynamicColor) *dynamicColor {
		return &dynamicColor{name: name, palette: p, isBackground: true,
			tone: func(s *DynamicScheme) float64 {
				if isMonochrome(s) {
					return mono
				}
				return normal
			},
			background: highestSurface, contrastCurve: curve(1, 1, 3, 4.5),
			toneDeltaPair: pair(a, b, lighter, true)}
	}
	onFixed := func(name string, p func(*DynamicScheme) *TonalPalette, mono, normal float64, dim, bright **dynamicColor, cc *contrastCurve) *dynamicColor {
		return &dynamicColor{name: name, palette: p,
			tone: func(s *DynamicScheme) float64 {
				if isMonochrome(s) {
					return mono
				}
				return normal
			},
			background: is(dim), secondBackground: is(bright), contrastCurve: cc}
	}
	mdcPrimaryFixed = fixed("primary_fixed", primaryP, 40, 90, &mdcPrimaryFixed, &mdcPrimaryFixedDim)
	mdcPrimaryFixedDim = fixed("primary_fixed_dim", primaryP, 30, 80, &mdcPrimaryFixed, &mdcPrimaryFixedDim)
	mdcOnPrimaryFixed = onFixed("on_primary_fixed", primaryP, 100, 10, &mdcPrimaryFixedDim, &mdcPrimaryFixed, curve(4.5, 7, 11, 21))
	mdcOnPrimaryFixedVariant = onFixed("on_primary_fixed_variant", primaryP, 90, 30, &mdcPrimaryFixedDim, &mdcPrimaryFixed, curve(3, 4.5, 7, 11))
	mdcSecondaryFixed = fixed("secondary_fixed", secondaryP, 80, 90, &mdcSecondaryFixed, &mdcSecondaryFixedDim)
	mdcSecondaryFixedDim = fixed("secondary_fixed_dim", secondaryP, 70, 80, &mdcSecondaryFixed, &mdcSecondaryFixedDim)
	mdcOnSecondaryFixed = onFixed("on_secondary_fixed", secondaryP, 10, 10, &mdcSecondaryFixedDim, &mdcSecondaryFixed, curve(4.5, 7, 11, 21))
	mdcOnSecondaryFixedVariant = onFixed("on_secondary_fixed_variant", secondaryP, 25, 30, &mdcSecondaryFixedDim, &mdcSecondaryFixed, curve(3, 4.5, 7, 11))
	mdcTertiaryFixed = fixed("tertiary_fixed", tertiaryP, 40, 90, &mdcTertiaryFixed, &mdcTertiaryFixedDim)
	mdcTertiaryFixedDim = fixed("tertiary_fixed_dim", tertiaryP, 30, 80, &mdcTertiaryFixed, &mdcTertiaryFixedDim)
	mdcOnTertiaryFixed = onFixed("on_tertiary_fixed", tertiaryP, 100, 10, &mdcTertiaryFixedDim, &mdcTertiaryFixed, curve(4.5, 7, 11, 21))
	mdcOnTertiaryFixedVariant = onFixed("on_tertiary_fixed_variant", tertiaryP, 90, 30, &mdcTertiaryFixedDim, &mdcTertiaryFixed, curve(3, 4.5, 7, 11))
}

func findDesiredChromaByTone(hue, chroma, tone float64, byDecreasingTone bool) float64 {
	answer := tone
	closest := HctFrom(hue, chroma, tone)
	if closest.Chroma < chroma {
		peak := closest.Chroma
		for closest.Chroma < chroma {
			if byDecreasingTone {
				answer -= 1.0
			} else {
				answer += 1.0
			}
			potential := HctFrom(hue, chroma, answer)
			if peak > potential.Chroma {
				break
			}
			if math.Abs(potential.Chroma-chroma) < 0.4 {
				break
			}
			if math.Abs(potential.Chroma-chroma) < math.Abs(closest.Chroma-chroma) {
				closest = potential
			}
			peak = math.Max(peak, potential.Chroma)
		}
	}
	return answer
}
