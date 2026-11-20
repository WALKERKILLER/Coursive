package m3

import (
	"math"
	"math/rand/v2"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
)

// inkSparkle is Flutter's ink_sparkle.frag (BSD-3-Clause, the Flutter
// authors), the shader of InkSparkle: a soft wave of the splash colour,
// a ring at its edge, and white sparkles in the ring, thinned by circle
// grids turning with the turbulence.
var inkSparkle = shader.MustCompile(`package main

var Color vec4
var Composite vec4
var Center vec2
var MaxRadius float
var ResolutionScale vec2
var NoiseScale vec2
var NoisePhase float
var Circle1 vec2
var Circle2 vec2
var Circle3 vec2
var Rotation1 vec2
var Rotation2 vec2
var Rotation3 vec2

func rotate2d(rad vec2) mat2 {
	return mat2(vec2(rad.x, -rad.y), vec2(rad.y, rad.x))
}

func softCircle(uv vec2, xy vec2, radius float, blur float) float {
	blurHalf := blur * 0.5
	d := distance(uv, xy)
	return 1.0 - smoothstep(1.0-blurHalf, 1.0+blurHalf, d/radius)
}

func circleGrid(resolution vec2, p vec2, xy vec2, rotation vec2, cellDiameter float) float {
	p = rotate2d(rotation)*(xy-p) + xy
	p = mod(p, vec2(cellDiameter)) / resolution
	cellUV := cellDiameter / resolution.y * 0.5
	r := 0.65 * cellUV
	return softCircle(p, vec2(cellUV), r, r*50.0)
}

func turbulence(uv vec2) float {
	uvScale := uv * vec2(0.8)
	g1 := circleGrid(vec2(0.8), uvScale, Circle1, Rotation1, 0.17)
	g2 := circleGrid(vec2(0.8), uvScale, Circle2, Rotation2, 0.2)
	g3 := circleGrid(vec2(0.8), uvScale, Circle3, Rotation3, 0.275)
	v := (g1*g1 + g2 - g3) * 0.5
	return clamp(0.45+0.8*v, 0.0, 1.0)
}

func softRing(uv vec2, xy vec2, radius float, thickness float, blur float) float {
	outer := softCircle(uv, xy, radius+thickness, blur)
	inner := softCircle(uv, xy, max(radius-thickness, 0.0), blur)
	return clamp(outer-inner, 0.0, 1.0)
}

func triangleNoise(n vec2) float {
	n = fract(n * vec2(5.3987, 5.4421))
	n += vec2(dot(n.yx, n+vec2(21.5351, 14.3137)))
	xy := n.x * n.y
	return fract(xy*95.4307) + fract(xy*75.04961) - 1.0
}

func threshold(v float, l float, h float) float {
	return step(l, v) * (1.0 - step(h, v))
}

func sparkle(uv vec2, t float) float {
	n := triangleNoise(uv)
	s := threshold(n, 0.0, 0.05)
	s += threshold(n+sin(3.14159265*(t+0.35)), 0.1, 0.15)
	s += threshold(n+sin(3.14159265*(t+0.7)), 0.2, 0.25)
	s += threshold(n+sin(3.14159265*(t+1.05)), 0.3, 0.35)
	return clamp(s, 0.0, 1.0) * 0.55
}

func Fragment(dst vec4, pos vec2) vec4 {
	alpha := Composite.x
	sparkleAlpha := Composite.y
	blur := Composite.z
	radius := max(MaxRadius*Composite.w, 0.0001)
	uv := pos * ResolutionScale
	densityUV := uv - mod(pos, NoiseScale)
	turb := turbulence(uv)
	ring := softRing(pos, Center, radius, 0.05*MaxRadius, blur)
	s := sparkle(densityUV, NoisePhase) * ring * turb * sparkleAlpha
	waveAlpha := softCircle(pos, Center, radius, blur) * alpha * Color.w
	wave := vec4(Color.xyz*waveAlpha, waveAlpha)
	return mix(wave, vec4(1.0), vec4(s))
}
`)

var (
	sparkleColor      = inkSparkle.Uniform("Color")
	sparkleComposite  = inkSparkle.Uniform("Composite")
	sparkleCenter     = inkSparkle.Uniform("Center")
	sparkleMaxRadius  = inkSparkle.Uniform("MaxRadius")
	sparkleResolution = inkSparkle.Uniform("ResolutionScale")
	sparkleNoiseScale = inkSparkle.Uniform("NoiseScale")
	sparkleNoisePhase = inkSparkle.Uniform("NoisePhase")
	sparkleCircles    = [3]shader.Uniform{inkSparkle.Uniform("Circle1"), inkSparkle.Uniform("Circle2"), inkSparkle.Uniform("Circle3")}
	sparkleRotations  = [3]shader.Uniform{inkSparkle.Uniform("Rotation1"), inkSparkle.Uniform("Rotation2"), inkSparkle.Uniform("Rotation3")}
	sparkleValues     = inkSparkle.NewValues()
)

// sparkleSeed is InkSparkle's turbulence seed, random for each splash so
// that splashes from the same point differ.
func sparkleSeed() float64 { return rand.Float64() * 1000 }

// paintSparkle paints splash s k of the way through its 617 ms in r,
// rounded by radius, as InkSparkle.paintFeature with the values
// _updateFragmentShader sets.
func paintSparkle(p *ui.Painter, r ui.Rect, radius float32, s *splash, k float32, color ui.Color) {
	// The animations of InkSparkle's TweenSequences.
	radiusScale := float32(1)
	if k < .75 {
		radiusScale = design.FastOutSlowIn(k / .75)
	}
	centerAt := min(radiusScale/.5, 1)
	alpha := float32(1)
	switch {
	case k < .13:
		alpha = k / .13
	case k > .4:
		alpha = 1 - (k-.4)/.6
	}
	sparkleA := float32(1)
	switch {
	case k < 13.0/90:
		sparkleA = k / (13.0 / 90)
	case k > 40.0/90:
		sparkleA = max(1-(k-40.0/90)/(50.0/90), 0)
	}
	cx := s.x + (r.W/2-s.x)*centerAt
	cy := s.y + (r.H/2-s.y)*centerAt
	target := float32(math.Hypot(float64(r.W), float64(r.H))) / 2 * sparkleReach

	const turbulenceScale = 1.5
	phase := s.seed + float64(radiusScale)
	rotate := math.Pi * 0.0078125
	rot := [3]float64{phase*rotate + 1.7*math.Pi, phase*-rotate + 2*math.Pi, phase*rotate + 2.75*math.Pi}
	circles := [3][2]float64{
		{turbulenceScale*0.5 + phase*0.01*math.Cos(turbulenceScale*0.55), turbulenceScale*0.5 + phase*0.01*math.Sin(turbulenceScale*0.55)},
		{turbulenceScale*0.2 + phase*-0.0066*math.Cos(turbulenceScale*0.45), turbulenceScale*0.2 + phase*-0.0066*math.Sin(turbulenceScale*0.45)},
		{turbulenceScale + phase*-0.0066*math.Cos(turbulenceScale*0.35), turbulenceScale + phase*-0.0066*math.Sin(turbulenceScale*0.35)},
	}
	v := sparkleValues.Color(sparkleColor, color).
		Vec4(sparkleComposite, alpha, sparkleA, 1, radiusScale).
		Vec2(sparkleCenter, cx, cy).
		Float(sparkleMaxRadius, target).
		Vec2(sparkleResolution, 1/r.W, 1/r.H).
		Vec2(sparkleNoiseScale, 2.1/r.W, 2.1/r.H).
		Float(sparkleNoisePhase, float32(phase/1000))
	for i := range 3 {
		v = v.Vec2(sparkleCircles[i], float32(circles[i][0]), float32(circles[i][1])).
			Vec2(sparkleRotations[i], float32(math.Cos(rot[i])), float32(math.Sin(rot[i])))
	}
	inkSparkle.Draw(p, r, shader.Options{Values: v, Radius: radius})
}
