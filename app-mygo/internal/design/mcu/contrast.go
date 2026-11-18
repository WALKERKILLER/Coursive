package mcu

import "math"

func ratioOfTones(a, b float64) float64 {
	a = clampDouble(0.0, 100.0, a)
	b = clampDouble(0.0, 100.0, b)
	return ratioOfYs(yFromLstar(a), yFromLstar(b))
}

func ratioOfYs(y1, y2 float64) float64 {
	lighter := y2
	if y1 > y2 {
		lighter = y1
	}
	darker := y2
	if lighter == y2 {
		darker = y1
	}
	return (lighter + 5.0) / (darker + 5.0)
}

func contrastLighter(tone, ratio float64) float64 {
	if tone < 0.0 || tone > 100.0 {
		return -1.0
	}
	darkY := yFromLstar(tone)
	lightY := ratio*(darkY+5.0) - 5.0
	real := ratioOfYs(lightY, darkY)
	if delta := math.Abs(real - ratio); real < ratio && delta > 0.04 {
		return -1
	}
	v := lstarFromY(lightY) + 0.4
	if v < 0 || v > 100 {
		return -1
	}
	return v
}

func contrastDarker(tone, ratio float64) float64 {
	if tone < 0.0 || tone > 100.0 {
		return -1.0
	}
	lightY := yFromLstar(tone)
	darkY := ((lightY + 5.0) / ratio) - 5.0
	real := ratioOfYs(lightY, darkY)
	if delta := math.Abs(real - ratio); real < ratio && delta > 0.04 {
		return -1
	}
	v := lstarFromY(darkY) - 0.4
	if v < 0 || v > 100 {
		return -1
	}
	return v
}

func lighterUnsafe(tone, ratio float64) float64 {
	if v := contrastLighter(tone, ratio); v >= 0.0 {
		return v
	}
	return 100.0
}

func darkerUnsafe(tone, ratio float64) float64 {
	if v := contrastDarker(tone, ratio); v >= 0.0 {
		return v
	}
	return 0.0
}

func isDisliked(h Hct) bool {
	hue := float64(dartRound(h.Hue))
	return hue >= 90.0 && hue <= 111.0 && float64(dartRound(h.Chroma)) > 16.0 && float64(dartRound(h.Tone)) < 65.0
}

func fixIfDisliked(h Hct) Hct {
	if isDisliked(h) {
		return HctFrom(h.Hue, h.Chroma, 70.0)
	}
	return h
}

type contrastCurve struct{ low, normal, medium, high float64 }

func (c *contrastCurve) get(level float64) float64 {
	switch {
	case level <= -1.0:
		return c.low
	case level < 0.0:
		return lerp(c.low, c.normal, (level-(-1))/1)
	case level < 0.5:
		return lerp(c.normal, c.medium, (level-0)/0.5)
	case level < 1.0:
		return lerp(c.medium, c.high, (level-0.5)/0.5)
	}
	return c.high
}

func curve(low, normal, medium, high float64) *contrastCurve {
	return &contrastCurve{low, normal, medium, high}
}
