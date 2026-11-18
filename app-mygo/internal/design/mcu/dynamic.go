package mcu

import "math"

// Variant is DynamicSchemeVariant.
type Variant int

const (
	TonalSpot Variant = iota
	Fidelity
	Monochrome
	Neutral
	Vibrant
	Expressive
	Content
)

// DynamicScheme is the palettes of one seed, brightness and contrast.
type DynamicScheme struct {
	Source                                Hct
	Variant                               Variant
	IsDark                                bool
	ContrastLevel                         float64
	Primary, Secondary, Tertiary          *TonalPalette
	Neutral, NeutralVariant, ErrorPalette *TonalPalette
}

type tonePolarity int

const (
	darker tonePolarity = iota
	lighter
	nearer
	farther
)

type toneDeltaPair struct {
	roleA, roleB *dynamicColor
	delta        float64
	polarity     tonePolarity
	stayTogether bool
}

type dynamicColor struct {
	name             string
	palette          func(*DynamicScheme) *TonalPalette
	tone             func(*DynamicScheme) float64
	isBackground     bool
	background       func(*DynamicScheme) *dynamicColor
	secondBackground func(*DynamicScheme) *dynamicColor
	contrastCurve    *contrastCurve
	toneDeltaPair    func(*DynamicScheme) toneDeltaPair
}

func (d *dynamicColor) argb(s *DynamicScheme) uint32 {
	return d.palette(s).getHct(d.getTone(s)).ToInt()
}

func (d *dynamicColor) getTone(s *DynamicScheme) float64 {
	decreasingContrast := s.ContrastLevel < 0
	if d.toneDeltaPair != nil {
		pair := d.toneDeltaPair(s)
		roleA, roleB := pair.roleA, pair.roleB
		delta := pair.delta
		bgTone := d.background(s).getTone(s)
		aIsNearer := pair.polarity == nearer ||
			(pair.polarity == lighter && !s.IsDark) ||
			(pair.polarity == darker && s.IsDark)
		near, far := roleB, roleA
		if aIsNearer {
			near, far = roleA, roleB
		}
		amNearer := d.name == near.name
		expansionDir := -1.0
		if s.IsDark {
			expansionDir = 1
		}
		nContrast := near.contrastCurve.get(s.ContrastLevel)
		fContrast := far.contrastCurve.get(s.ContrastLevel)
		nInitial := near.tone(s)
		nTone := nInitial
		if ratioOfTones(bgTone, nInitial) < nContrast {
			nTone = foregroundTone(bgTone, nContrast)
		}
		fInitial := far.tone(s)
		fTone := fInitial
		if ratioOfTones(bgTone, fInitial) < fContrast {
			fTone = foregroundTone(bgTone, fContrast)
		}
		if decreasingContrast {
			nTone = foregroundTone(bgTone, nContrast)
			fTone = foregroundTone(bgTone, fContrast)
		}
		if (fTone-nTone)*expansionDir < delta {
			fTone = clampDouble(0, 100, nTone+delta*expansionDir)
			if (fTone-nTone)*expansionDir < delta {
				nTone = clampDouble(0, 100, fTone-delta*expansionDir)
			}
		}
		if 50 <= nTone && nTone < 60 {
			if expansionDir > 0 {
				nTone = 60
				fTone = math.Max(fTone, nTone+delta*expansionDir)
			} else {
				nTone = 49
				fTone = math.Min(fTone, nTone+delta*expansionDir)
			}
		} else if 50 <= fTone && fTone < 60 {
			if pair.stayTogether {
				if expansionDir > 0 {
					nTone = 60
					fTone = math.Max(fTone, nTone+delta*expansionDir)
				} else {
					nTone = 49
					fTone = math.Min(fTone, nTone+delta*expansionDir)
				}
			} else if expansionDir > 0 {
				fTone = 60
			} else {
				fTone = 49
			}
		}
		if amNearer {
			return nTone
		}
		return fTone
	}
	answer := d.tone(s)
	if d.background == nil {
		return answer
	}
	bgTone := d.background(s).getTone(s)
	desiredRatio := d.contrastCurve.get(s.ContrastLevel)
	if ratioOfTones(bgTone, answer) < desiredRatio {
		answer = foregroundTone(bgTone, desiredRatio)
	}
	if decreasingContrast {
		answer = foregroundTone(bgTone, desiredRatio)
	}
	if d.isBackground && 50 <= answer && answer < 60 {
		if ratioOfTones(49, bgTone) >= desiredRatio {
			answer = 49
		} else {
			answer = 60
		}
	}
	if d.secondBackground != nil {
		bgTone1 := d.background(s).getTone(s)
		bgTone2 := d.secondBackground(s).getTone(s)
		upper := math.Max(bgTone1, bgTone2)
		lower := math.Min(bgTone1, bgTone2)
		if ratioOfTones(upper, answer) >= desiredRatio && ratioOfTones(lower, answer) >= desiredRatio {
			return answer
		}
		lightOption := contrastLighter(upper, desiredRatio)
		darkOption := contrastDarker(lower, desiredRatio)
		var availables []float64
		if lightOption != -1 {
			availables = append(availables, lightOption)
		}
		if darkOption != -1 {
			availables = append(availables, darkOption)
		}
		if tonePrefersLightForeground(bgTone1) || tonePrefersLightForeground(bgTone2) {
			if lightOption < 0 {
				return 100
			}
			return lightOption
		}
		if len(availables) == 1 {
			return availables[0]
		}
		if darkOption < 0 {
			return 0
		}
		return darkOption
	}
	return answer
}

func foregroundTone(bgTone, ratio float64) float64 {
	lighterTone := lighterUnsafe(bgTone, ratio)
	darkerTone := darkerUnsafe(bgTone, ratio)
	lighterRatio := ratioOfTones(lighterTone, bgTone)
	darkerRatio := ratioOfTones(darkerTone, bgTone)
	if tonePrefersLightForeground(bgTone) {
		negligible := math.Abs(lighterRatio-darkerRatio) < 0.1 && lighterRatio < ratio && darkerRatio < ratio
		if lighterRatio >= ratio || lighterRatio >= darkerRatio || negligible {
			return lighterTone
		}
		return darkerTone
	}
	if darkerRatio >= ratio || darkerRatio >= lighterRatio {
		return darkerTone
	}
	return lighterTone
}

func tonePrefersLightForeground(tone float64) bool { return dartRound(tone) < 60 }
