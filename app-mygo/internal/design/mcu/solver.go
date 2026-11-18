package mcu

import "math"

var scaledDiscountFromLinrgb = [3][3]float64{
	{0.001200833568784504, 0.002389694492170889, 0.0002795742885861124},
	{0.0005891086651375999, 0.0029785502573438758, 0.0003270666104008398},
	{0.00010146692491640572, 0.0005364214359186694, 0.0032979401770712076},
}

var linrgbFromScaledDiscount = [3][3]float64{
	{1373.2198709594231, -1100.4251190754821, -7.278681089101213},
	{-271.815969077903, 559.6580465940733, -32.46047482791194},
	{1.9622899599665666, -57.173814538844006, 308.7233197812385},
}

var yFromLinrgb = [3]float64{0.2126, 0.7152, 0.0722}

func sanitizeRadians(angle float64) float64 { return math.Mod(angle+math.Pi*8, math.Pi*2) }

func trueDelinearized(c float64) float64 {
	n := c / 100.0
	var d float64
	if n <= 0.0031308 {
		d = n * 12.92
	} else {
		d = 1.055*math.Pow(n, 1.0/2.4) - 0.055
	}
	return d * 255.0
}

func chromaticAdaptation(component float64) float64 {
	af := math.Pow(math.Abs(component), 0.42)
	return signum(component) * 400.0 * af / (af + 27.13)
}

func hueOf(linrgb [3]float64) float64 {
	sd := matrixMultiply(linrgb, scaledDiscountFromLinrgb)
	rA := chromaticAdaptation(sd[0])
	gA := chromaticAdaptation(sd[1])
	bA := chromaticAdaptation(sd[2])
	a := (11.0*rA + -12.0*gA + bA) / 11.0
	b := (rA + gA - 2.0*bA) / 9.0
	return math.Atan2(b, a)
}

func areInCyclicOrder(a, b, c float64) bool {
	return sanitizeRadians(b-a) < sanitizeRadians(c-a)
}

func intercept(source, mid, target float64) float64 { return (mid - source) / (target - source) }

func lerpPoint(s [3]float64, t float64, e [3]float64) [3]float64 {
	return [3]float64{s[0] + (e[0]-s[0])*t, s[1] + (e[1]-s[1])*t, s[2] + (e[2]-s[2])*t}
}

func setCoordinate(s [3]float64, coordinate float64, e [3]float64, axis int) [3]float64 {
	return lerpPoint(s, intercept(s[axis], coordinate, e[axis]), e)
}

func isBounded(x float64) bool { return 0.0 <= x && x <= 100.0 }

func nthVertex(y float64, n int) [3]float64 {
	kR, kG, kB := yFromLinrgb[0], yFromLinrgb[1], yFromLinrgb[2]
	coordA := 100.0
	if n%4 <= 1 {
		coordA = 0.0
	}
	coordB := 100.0
	if n%2 == 0 {
		coordB = 0.0
	}
	none := [3]float64{-1, -1, -1}
	switch {
	case n < 4:
		g, b := coordA, coordB
		r := (y - g*kG - b*kB) / kR
		if isBounded(r) {
			return [3]float64{r, g, b}
		}
	case n < 8:
		b, r := coordA, coordB
		g := (y - r*kR - b*kB) / kG
		if isBounded(g) {
			return [3]float64{r, g, b}
		}
	default:
		r, g := coordA, coordB
		b := (y - r*kR - g*kG) / kB
		if isBounded(b) {
			return [3]float64{r, g, b}
		}
	}
	return none
}

func bisectToSegment(y, targetHue float64) ([3]float64, [3]float64) {
	left := [3]float64{-1, -1, -1}
	right := left
	leftHue, rightHue := 0.0, 0.0
	initialized, uncut := false, true
	for n := 0; n < 12; n++ {
		mid := nthVertex(y, n)
		if mid[0] < 0 {
			continue
		}
		midHue := hueOf(mid)
		if !initialized {
			left, right = mid, mid
			leftHue, rightHue = midHue, midHue
			initialized = true
			continue
		}
		if uncut || areInCyclicOrder(leftHue, midHue, rightHue) {
			uncut = false
			if areInCyclicOrder(leftHue, targetHue, midHue) {
				right, rightHue = mid, midHue
			} else {
				left, leftHue = mid, midHue
			}
		}
	}
	return left, right
}

func midpoint(a, b [3]float64) [3]float64 {
	return [3]float64{(a[0] + b[0]) / 2, (a[1] + b[1]) / 2, (a[2] + b[2]) / 2}
}

func criticalPlaneBelow(x float64) int { return int(math.Floor(x - 0.5)) }
func criticalPlaneAbove(x float64) int { return int(math.Ceil(x - 0.5)) }

func bisectToLimit(y, targetHue float64) [3]float64 {
	left, right := bisectToSegment(y, targetHue)
	leftHue := hueOf(left)
	for axis := 0; axis < 3; axis++ {
		if left[axis] == right[axis] {
			continue
		}
		var lPlane, rPlane int
		if left[axis] < right[axis] {
			lPlane = criticalPlaneBelow(trueDelinearized(left[axis]))
			rPlane = criticalPlaneAbove(trueDelinearized(right[axis]))
		} else {
			lPlane = criticalPlaneAbove(trueDelinearized(left[axis]))
			rPlane = criticalPlaneBelow(trueDelinearized(right[axis]))
		}
		for i := 0; i < 8; i++ {
			if abs(rPlane-lPlane) <= 1 {
				break
			}
			mPlane := int(math.Floor(float64(lPlane+rPlane) / 2.0))
			mid := setCoordinate(left, criticalPlanes[mPlane], right, axis)
			midHue := hueOf(mid)
			if areInCyclicOrder(leftHue, targetHue, midHue) {
				right, rPlane = mid, mPlane
			} else {
				left, leftHue, lPlane = mid, midHue, mPlane
			}
		}
	}
	return midpoint(left, right)
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func inverseChromaticAdaptation(adapted float64) float64 {
	a := math.Abs(adapted)
	base := math.Max(0, 27.13*a/(400.0-a))
	return signum(adapted) * math.Pow(base, 1.0/0.42)
}

func findResultByJ(hueRadians, chroma, y float64) uint32 {
	j := math.Sqrt(y) * 11.0
	vc := &standardVC
	tInnerCoeff := 1 / math.Pow(1.64-math.Pow(0.29, vc.backgroundYToWhitePointY), 0.73)
	eHue := 0.25 * (math.Cos(hueRadians+2.0) + 3.8)
	p1 := eHue * (50000.0 / 13.0) * vc.nC * vc.ncb
	hSin := math.Sin(hueRadians)
	hCos := math.Cos(hueRadians)
	for round := 0; round < 5; round++ {
		jNormalized := j / 100.0
		alpha := 0.0
		if chroma != 0.0 && j != 0.0 {
			alpha = chroma / math.Sqrt(jNormalized)
		}
		t := math.Pow(alpha*tInnerCoeff, 1.0/0.9)
		ac := vc.aw * math.Pow(jNormalized, 1.0/vc.c/vc.z)
		p2 := ac / vc.nbb
		gamma := 23.0 * (p2 + 0.305) * t / (23.0*p1 + 11*t*hCos + 108.0*t*hSin)
		a := gamma * hCos
		b := gamma * hSin
		rA := (460.0*p2 + 451.0*a + 288.0*b) / 1403.0
		gA := (460.0*p2 - 891.0*a - 261.0*b) / 1403.0
		bA := (460.0*p2 - 220.0*a - 6300.0*b) / 1403.0
		linrgb := matrixMultiply([3]float64{
			inverseChromaticAdaptation(rA),
			inverseChromaticAdaptation(gA),
			inverseChromaticAdaptation(bA),
		}, linrgbFromScaledDiscount)
		if linrgb[0] < 0 || linrgb[1] < 0 || linrgb[2] < 0 {
			return 0
		}
		fnj := yFromLinrgb[0]*linrgb[0] + yFromLinrgb[1]*linrgb[1] + yFromLinrgb[2]*linrgb[2]
		if fnj <= 0 {
			return 0
		}
		if round == 4 || math.Abs(fnj-y) < 0.002 {
			if linrgb[0] > 100.01 || linrgb[1] > 100.01 || linrgb[2] > 100.01 {
				return 0
			}
			return argbFromLinrgb(linrgb)
		}
		j = j - (fnj-y)*j/(2*fnj)
	}
	return 0
}

func solveToInt(hueDegrees, chroma, lstar float64) uint32 {
	if chroma < 0.0001 || lstar < 0.0001 || lstar > 99.9999 {
		return argbFromLstar(lstar)
	}
	hueDegrees = sanitizeDegreesDouble(hueDegrees)
	hueRadians := hueDegrees / 180 * math.Pi
	y := yFromLstar(lstar)
	if exact := findResultByJ(hueRadians, chroma, y); exact != 0 {
		return exact
	}
	return argbFromLinrgb(bisectToLimit(y, hueRadians))
}
