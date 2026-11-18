// Package mcu is a Go port of Google's material_color_utilities (Dart 0.13.0,
// the version Flutter's ColorScheme.fromSeed runs), so that a seed gives the
// same colours here as in the Flutter client. Apache License 2.0; see LICENSE.
package mcu

import "math"

func signum(x float64) float64 {
	switch {
	case x < 0:
		return -1
	case x == 0:
		return 0
	}
	return 1
}

func lerp(start, stop, amount float64) float64 { return (1.0-amount)*start + amount*stop }

func clampInt(lo, hi, v int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampDouble(lo, hi, v float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func sanitizeDegreesInt(d int) int {
	d %= 360
	if d < 0 {
		d += 360
	}
	return d
}

func sanitizeDegreesDouble(d float64) float64 {
	d = math.Mod(d, 360.0)
	if d < 0 {
		d += 360.0
	}
	return d
}

func differenceDegrees(a, b float64) float64 { return 180.0 - math.Abs(math.Abs(a-b)-180.0) }

func matrixMultiply(row [3]float64, m [3][3]float64) [3]float64 {
	return [3]float64{
		row[0]*m[0][0] + row[1]*m[0][1] + row[2]*m[0][2],
		row[0]*m[1][0] + row[1]*m[1][1] + row[2]*m[1][2],
		row[0]*m[2][0] + row[1]*m[2][1] + row[2]*m[2][2],
	}
}

// dartRound rounds half away from zero, as Dart's double.round does.
func dartRound(x float64) int { return int(math.Round(x)) }

var srgbToXyz = [3][3]float64{
	{0.41233895, 0.35762064, 0.18051042},
	{0.2126, 0.7152, 0.0722},
	{0.01932141, 0.11916382, 0.95034478},
}

var xyzToSrgb = [3][3]float64{
	{3.2413774792388685, -1.5376652402851851, -0.49885366846268053},
	{-0.9691452513005321, 1.8758853451067872, 0.04156585616912061},
	{0.05562093689691305, -0.20395524564742123, 1.0571799111220335},
}

var whitePointD65 = [3]float64{95.047, 100.0, 108.883}

func argbFromRgb(r, g, b int) uint32 {
	return 255<<24 | uint32(r&255)<<16 | uint32(g&255)<<8 | uint32(b&255)
}

func argbFromLinrgb(l [3]float64) uint32 {
	return argbFromRgb(delinearized(l[0]), delinearized(l[1]), delinearized(l[2]))
}

func redFromArgb(argb uint32) int   { return int(argb >> 16 & 255) }
func greenFromArgb(argb uint32) int { return int(argb >> 8 & 255) }
func blueFromArgb(argb uint32) int  { return int(argb & 255) }

func argbFromXyz(x, y, z float64) uint32 {
	m := xyzToSrgb
	lr := m[0][0]*x + m[0][1]*y + m[0][2]*z
	lg := m[1][0]*x + m[1][1]*y + m[1][2]*z
	lb := m[2][0]*x + m[2][1]*y + m[2][2]*z
	return argbFromRgb(delinearized(lr), delinearized(lg), delinearized(lb))
}

func xyzFromArgb(argb uint32) [3]float64 {
	r := linearized(redFromArgb(argb))
	g := linearized(greenFromArgb(argb))
	b := linearized(blueFromArgb(argb))
	return matrixMultiply([3]float64{r, g, b}, srgbToXyz)
}

func labFromArgb(argb uint32) [3]float64 {
	lr := linearized(redFromArgb(argb))
	lg := linearized(greenFromArgb(argb))
	lb := linearized(blueFromArgb(argb))
	m := srgbToXyz
	x := m[0][0]*lr + m[0][1]*lg + m[0][2]*lb
	y := m[1][0]*lr + m[1][1]*lg + m[1][2]*lb
	z := m[2][0]*lr + m[2][1]*lg + m[2][2]*lb
	fx := labF(x / whitePointD65[0])
	fy := labF(y / whitePointD65[1])
	fz := labF(z / whitePointD65[2])
	return [3]float64{116.0*fy - 16, 500.0 * (fx - fy), 200.0 * (fy - fz)}
}

func argbFromLstar(lstar float64) uint32 {
	c := delinearized(yFromLstar(lstar))
	return argbFromRgb(c, c, c)
}

func lstarFromArgb(argb uint32) float64 {
	y := xyzFromArgb(argb)[1]
	return 116.0*labF(y/100.0) - 16.0
}

func yFromLstar(lstar float64) float64 { return 100.0 * labInvf((lstar+16.0)/116.0) }

func lstarFromY(y float64) float64 { return labF(y/100.0)*116.0 - 16.0 }

func linearized(c int) float64 {
	n := float64(c) / 255.0
	if n <= 0.040449936 {
		return n / 12.92 * 100.0
	}
	return math.Pow((n+0.055)/1.055, 2.4) * 100.0
}

func delinearized(c float64) int {
	n := c / 100.0
	var d float64
	if n <= 0.0031308 {
		d = n * 12.92
	} else {
		d = 1.055*math.Pow(n, 1.0/2.4) - 0.055
	}
	return clampInt(0, 255, dartRound(d*255.0))
}

func labF(t float64) float64 {
	const e = 216.0 / 24389.0
	const kappa = 24389.0 / 27.0
	if t > e {
		return math.Pow(t, 1.0/3.0)
	}
	return (kappa*t + 16) / 116
}

func labInvf(ft float64) float64 {
	const e = 216.0 / 24389.0
	const kappa = 24389.0 / 27.0
	ft3 := ft * ft * ft
	if ft3 > e {
		return ft3
	}
	return (116*ft - 16) / kappa
}
