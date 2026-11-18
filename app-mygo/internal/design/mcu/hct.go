package mcu

// Hct is a colour in hue, chroma and tone, kept with the ARGB it solved to.
type Hct struct {
	Hue, Chroma, Tone float64
	argb              uint32
}

// HctFrom solves a hue, chroma and tone to the nearest sRGB colour.
func HctFrom(hue, chroma, tone float64) Hct { return HctFromInt(solveToInt(hue, chroma, tone)) }

// HctFromInt reads an ARGB colour.
func HctFromInt(argb uint32) Hct {
	c := cam16FromInt(argb)
	return Hct{Hue: c.hue, Chroma: c.chroma, Tone: lstarFromArgb(argb), argb: argb}
}

// ToInt is the colour as ARGB.
func (h Hct) ToInt() uint32 { return h.argb }
