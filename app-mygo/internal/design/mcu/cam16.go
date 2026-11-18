package mcu

import "math"

// viewingConditions are CAM16's, here only the standard sRGB ones.
type viewingConditions struct {
	whitePoint               [3]float64
	adaptingLuminance        float64
	backgroundLstar          float64
	surround                 float64
	backgroundYToWhitePointY float64
	aw, nbb, ncb, c, nC      float64
	rgbD                     [3]float64
	fl, fLRoot, z            float64
}

var standardVC = makeViewingConditions()

func makeViewingConditions() viewingConditions {
	whitePoint := whitePointD65
	adaptingLuminance := 200.0 / math.Pi * yFromLstar(50.0) / 100.0
	backgroundLstar := math.Max(0.1, 50.0)
	surround := 2.0
	xyz := whitePoint
	rW := xyz[0]*0.401288 + xyz[1]*0.650173 + xyz[2]*-0.051461
	gW := xyz[0]*-0.250268 + xyz[1]*1.204414 + xyz[2]*0.045854
	bW := xyz[0]*-0.002079 + xyz[1]*0.048952 + xyz[2]*0.953127
	f := 0.8 + (surround / 10.0)
	var c float64
	if f >= 0.9 {
		c = lerp(0.59, 0.69, (f-0.9)*10.0)
	} else {
		c = lerp(0.525, 0.59, (f-0.8)*10.0)
	}
	d := f * (1.0 - ((1.0 / 3.6) * math.Exp((-adaptingLuminance-42.0)/92.0)))
	if d > 1.0 {
		d = 1.0
	} else if d < 0.0 {
		d = 0.0
	}
	nc := f
	rgbD := [3]float64{
		d*(100.0/rW) + 1.0 - d,
		d*(100.0/gW) + 1.0 - d,
		d*(100.0/bW) + 1.0 - d,
	}
	k := 1.0 / (5.0*adaptingLuminance + 1.0)
	k4 := k * k * k * k
	k4F := 1.0 - k4
	fl := (k4 * adaptingLuminance) + (0.1 * k4F * k4F * math.Pow(5.0*adaptingLuminance, 1.0/3.0))
	n := yFromLstar(backgroundLstar) / whitePoint[1]
	z := 1.48 + math.Sqrt(n)
	nbb := 0.725 / math.Pow(n, 0.2)
	ncb := nbb
	rgbAFactors := [3]float64{
		math.Pow(fl*rgbD[0]*rW/100.0, 0.42),
		math.Pow(fl*rgbD[1]*gW/100.0, 0.42),
		math.Pow(fl*rgbD[2]*bW/100.0, 0.42),
	}
	rgbA := [3]float64{
		(400.0 * rgbAFactors[0]) / (rgbAFactors[0] + 27.13),
		(400.0 * rgbAFactors[1]) / (rgbAFactors[1] + 27.13),
		(400.0 * rgbAFactors[2]) / (rgbAFactors[2] + 27.13),
	}
	aw := (40.0*rgbA[0] + 20.0*rgbA[1] + rgbA[2]) / 20.0 * nbb
	return viewingConditions{
		whitePoint: whitePoint, adaptingLuminance: adaptingLuminance,
		backgroundLstar: backgroundLstar, surround: surround,
		backgroundYToWhitePointY: n, aw: aw, nbb: nbb, ncb: ncb, c: c, nC: nc,
		rgbD: rgbD, fl: fl, fLRoot: math.Pow(fl, 0.25), z: z,
	}
}

// cam16 is a colour in CAM16.
type cam16 struct {
	hue, chroma, j, q, m, s, jstar, astar, bstar float64
}

func cam16FromInt(argb uint32) cam16 {
	xyz := xyzFromArgb(argb)
	return cam16FromXyz(xyz[0], xyz[1], xyz[2], &standardVC)
}

func cam16FromXyz(x, y, z float64, vc *viewingConditions) cam16 {
	rC := 0.401288*x + 0.650173*y - 0.051461*z
	gC := -0.250268*x + 1.204414*y + 0.045854*z
	bC := -0.002079*x + 0.048952*y + 0.953127*z
	rD := vc.rgbD[0] * rC
	gD := vc.rgbD[1] * gC
	bD := vc.rgbD[2] * bC
	rAF := math.Pow(vc.fl*math.Abs(rD)/100.0, 0.42)
	gAF := math.Pow(vc.fl*math.Abs(gD)/100.0, 0.42)
	bAF := math.Pow(vc.fl*math.Abs(bD)/100.0, 0.42)
	rA := signum(rD) * 400.0 * rAF / (rAF + 27.13)
	gA := signum(gD) * 400.0 * gAF / (gAF + 27.13)
	bA := signum(bD) * 400.0 * bAF / (bAF + 27.13)
	a := (11.0*rA + -12.0*gA + bA) / 11.0
	b := (rA + gA - 2.0*bA) / 9.0
	u := (20.0*rA + 20.0*gA + 21.0*bA) / 20.0
	p2 := (40.0*rA + 20.0*gA + bA) / 20.0
	atanDegrees := math.Atan2(b, a) * 180.0 / math.Pi
	hue := atanDegrees
	if atanDegrees < 0 {
		hue = atanDegrees + 360.0
	} else if atanDegrees >= 360 {
		hue = atanDegrees - 360
	}
	hueRadians := hue * math.Pi / 180.0
	ac := p2 * vc.nbb
	J := 100.0 * math.Pow(ac/vc.aw, vc.c*vc.z)
	Q := (4.0 / vc.c) * math.Sqrt(J/100.0) * (vc.aw + 4.0) * vc.fLRoot
	huePrime := hue
	if hue < 20.14 {
		huePrime = hue + 360
	}
	eHue := (1.0 / 4.0) * (math.Cos(huePrime*math.Pi/180.0+2.0) + 3.8)
	p1 := 50000.0 / 13.0 * eHue * vc.nC * vc.ncb
	t := p1 * math.Sqrt(a*a+b*b) / (u + 0.305)
	alpha := math.Pow(t, 0.9) * math.Pow(1.64-math.Pow(0.29, vc.backgroundYToWhitePointY), 0.73)
	C := alpha * math.Sqrt(J/100.0)
	M := C * vc.fLRoot
	s := 50.0 * math.Sqrt((alpha*vc.c)/(vc.aw+4.0))
	jstar := (1.0 + 100.0*0.007) * J / (1.0 + 0.007*J)
	mstar := math.Log(1.0+0.0228*M) / 0.0228
	return cam16{hue, C, J, Q, M, s, jstar, mstar * math.Cos(hueRadians), mstar * math.Sin(hueRadians)}
}

func (c cam16) xyzInViewingConditions(vc *viewingConditions) [3]float64 {
	alpha := 0.0
	if c.chroma != 0.0 && c.j != 0.0 {
		alpha = c.chroma / math.Sqrt(c.j/100.0)
	}
	t := math.Pow(alpha/math.Pow(1.64-math.Pow(0.29, vc.backgroundYToWhitePointY), 0.73), 1.0/0.9)
	hRad := c.hue * math.Pi / 180.0
	eHue := 0.25 * (math.Cos(hRad+2.0) + 3.8)
	ac := vc.aw * math.Pow(c.j/100.0, 1.0/vc.c/vc.z)
	p1 := eHue * (50000.0 / 13.0) * vc.nC * vc.ncb
	p2 := ac / vc.nbb
	hSin := math.Sin(hRad)
	hCos := math.Cos(hRad)
	gamma := 23.0 * (p2 + 0.305) * t / (23.0*p1 + 11*t*hCos + 108.0*t*hSin)
	a := gamma * hCos
	b := gamma * hSin
	rA := (460.0*p2 + 451.0*a + 288.0*b) / 1403.0
	gA := (460.0*p2 - 891.0*a - 261.0*b) / 1403.0
	bA := (460.0*p2 - 220.0*a - 6300.0*b) / 1403.0
	rCBase := math.Max(0, (27.13*math.Abs(rA))/(400.0-math.Abs(rA)))
	rC := signum(rA) * (100.0 / vc.fl) * math.Pow(rCBase, 1.0/0.42)
	gCBase := math.Max(0, (27.13*math.Abs(gA))/(400.0-math.Abs(gA)))
	gC := signum(gA) * (100.0 / vc.fl) * math.Pow(gCBase, 1.0/0.42)
	bCBase := math.Max(0, (27.13*math.Abs(bA))/(400.0-math.Abs(bA)))
	bC := signum(bA) * (100.0 / vc.fl) * math.Pow(bCBase, 1.0/0.42)
	rF := rC / vc.rgbD[0]
	gF := gC / vc.rgbD[1]
	bF := bC / vc.rgbD[2]
	return [3]float64{
		1.86206786*rF - 1.01125463*gF + 0.14918677*bF,
		0.38752654*rF + 0.62144744*gF - 0.00897398*bF,
		-0.01584150*rF - 0.03412294*gF + 1.04996444*bF,
	}
}
