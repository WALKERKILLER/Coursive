package mcu

import "math"

// TonalPalette is one hue and chroma at every tone.
type TonalPalette struct {
	Hue, Chroma float64
	KeyColor    Hct
}

// PaletteOf is TonalPalette.of: the palette's key colour is found by tone.
func PaletteOf(hue, chroma float64) *TonalPalette {
	return &TonalPalette{Hue: hue, Chroma: chroma, KeyColor: keyColor(hue, chroma)}
}

// PaletteFromHct is TonalPalette.fromHct.
func PaletteFromHct(h Hct) *TonalPalette {
	return &TonalPalette{Hue: h.Hue, Chroma: h.Chroma, KeyColor: h}
}

// Tone is the palette's colour at a tone, as ARGB.
func (p *TonalPalette) Tone(tone float64) uint32 { return solveToInt(p.Hue, p.Chroma, tone) }

func (p *TonalPalette) getHct(tone float64) Hct { return HctFrom(p.Hue, p.Chroma, tone) }

func keyColor(hue, requestedChroma float64) Hct {
	const pivotTone = 50
	const toneStepSize = 1
	const epsilon = 0.01
	cache := map[int]float64{}
	maxChroma := func(tone int) float64 {
		if v, ok := cache[tone]; ok {
			return v
		}
		v := HctFrom(hue, 200.0, float64(tone)).Chroma
		cache[tone] = v
		return v
	}
	lowerTone, upperTone := 0, 100
	for lowerTone < upperTone {
		midTone := (lowerTone + upperTone) / 2
		isAscending := maxChroma(midTone) < maxChroma(midTone+toneStepSize)
		sufficientChroma := maxChroma(midTone) >= requestedChroma-epsilon
		if sufficientChroma {
			if math.Abs(float64(lowerTone-pivotTone)) < math.Abs(float64(upperTone-pivotTone)) {
				upperTone = midTone
			} else {
				if lowerTone == midTone {
					return HctFrom(hue, requestedChroma, float64(lowerTone))
				}
				lowerTone = midTone
			}
		} else if isAscending {
			lowerTone = midTone + toneStepSize
		} else {
			upperTone = midTone
		}
	}
	return HctFrom(hue, requestedChroma, float64(lowerTone))
}
