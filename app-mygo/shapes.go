package main

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// Coursive's organic shapes, ported from app/lib/widgets/organic_shapes.dart.
// Every shape is resampled to one polar form — a radius per angle around its
// centre — so any two of them morph by plain interpolation, and the course
// covers keep the same silhouettes as the Flutter client's.
type OrganicShape int

const (
	ShapeCircle OrganicShape = iota
	ShapeSquircle
	ShapeCookie4
	ShapeCookie6
	ShapeCookie7
	ShapeCookie9
	ShapeCookie12
	ShapeClover4
	ShapeClover8
	ShapeSunny
	ShapeVerySunny
	ShapeSoftBurst
	ShapeFlower
	ShapePuffy
	ShapeTriangle
	ShapePentagon
	ShapeDiamond
	ShapeGem
	ShapePill
	ShapeOval
	ShapeArch
	ShapeHeart
	ShapePebble
	ShapeMochi
	ShapeBean
	ShapeCapsule
	ShapeDroplet
	ShapeGhost
)

// shapeSamples is the resolution every shape is resampled to.
const shapeSamples = 180

var shapeCache = map[OrganicShape][]float64{}

// shapeRadii is the unit-radius polar form of s, index i at angle
// 2πi/N from -π/2, as shapeRadii in the Dart client.
func shapeRadii(s OrganicShape) []float64 {
	if r, ok := shapeCache[s]; ok {
		return r
	}
	r := normalise(polarOf(outline(s)))
	shapeCache[s] = r
	return r
}

// morphRadii interpolates between two shapes' radii.
func morphRadii(a, b OrganicShape, t float64) []float64 {
	ra, rb := shapeRadii(a), shapeRadii(b)
	if t <= 0 {
		return ra
	}
	if t >= 1 {
		return rb
	}
	out := make([]float64, shapeSamples)
	for i := range out {
		out[i] = ra[i] + (rb[i]-ra[i])*t
	}
	return out
}

// shapePath builds the smooth closed path for radii, rotated by rotation
// radians and scaled so its bounds fit inside r, centred, aspect preserved.
// The curve is Catmull-Rom through the samples, as the Dart client draws it.
func shapePath(radii []float64, r ui.Rect, rotation float64) ui.Path {
	n := len(radii)
	xs := make([]float64, n)
	ys := make([]float64, n)
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for i := 0; i < n; i++ {
		a := -math.Pi/2 + 2*math.Pi*float64(i)/float64(n) + rotation
		xs[i] = math.Cos(a) * radii[i]
		ys[i] = math.Sin(a) * radii[i]
		minX, maxX = math.Min(minX, xs[i]), math.Max(maxX, xs[i])
		minY, maxY = math.Min(minY, ys[i]), math.Max(maxY, ys[i])
	}
	s := math.Min(float64(r.W)/(maxX-minX), float64(r.H)/(maxY-minY))
	cx, cy := (minX+maxX)/2, (minY+maxY)/2
	mx := make([]float64, n)
	my := make([]float64, n)
	rcx, rcy := float64(r.X+r.W/2), float64(r.Y+r.H/2)
	for i := 0; i < n; i++ {
		mx[i] = rcx + (xs[i]-cx)*s
		my[i] = rcy + (ys[i]-cy)*s
	}
	var p ui.Path
	p.MoveTo(float32(mx[0]), float32(my[0]))
	for i := 0; i < n; i++ {
		p0, p1 := (i-1+n)%n, i
		p2, p3 := (i+1)%n, (i+2)%n
		c1x := mx[p1] + (mx[p2]-mx[p0])/6
		c1y := my[p1] + (my[p2]-my[p0])/6
		c2x := mx[p2] - (mx[p3]-mx[p1])/6
		c2y := my[p2] - (my[p3]-my[p1])/6
		p.CubeTo(float32(c1x), float32(c1y), float32(c2x), float32(c2y), float32(mx[p2]), float32(my[p2]))
	}
	return *p.Close()
}

type pt struct{ x, y float64 }

func bump(t float64, k int, p float64) float64 {
	return math.Pow((1+math.Cos(float64(k)*t))/2, p)
}

// outline is a dense, star-shaped outline around the origin.
func outline(s OrganicShape) []pt {
	const n = 720
	polar := func(r func(float64) float64) []pt {
		out := make([]pt, n)
		for i := 0; i < n; i++ {
			t := 2 * math.Pi * float64(i) / n
			a := t - math.Pi/2
			rr := r(t)
			out[i] = pt{math.Cos(a) * rr, math.Sin(a) * rr}
		}
		return out
	}
	switch s {
	case ShapeCircle:
		return polar(func(float64) float64 { return 1 })
	case ShapeSquircle:
		return polar(func(t float64) float64 {
			return math.Pow(math.Pow(math.Abs(math.Cos(t)), 4)+math.Pow(math.Abs(math.Sin(t)), 4), -0.25)
		})
	case ShapeCookie4:
		return polar(func(t float64) float64 { return 1 + 0.11*math.Cos(4*t) })
	case ShapeCookie6:
		return polar(func(t float64) float64 { return 1 + 0.085*math.Cos(6*t) })
	case ShapeCookie7:
		return polar(func(t float64) float64 { return 1 + 0.075*math.Cos(7*t) })
	case ShapeCookie9:
		return polar(func(t float64) float64 { return 1 + 0.065*math.Cos(9*t) })
	case ShapeCookie12:
		return polar(func(t float64) float64 { return 1 + 0.05*math.Cos(12*t) })
	case ShapeClover4:
		return polar(func(t float64) float64 { return 0.6 + 0.4*math.Pow(math.Abs(math.Cos(2*t)), 0.55) })
	case ShapeClover8:
		return polar(func(t float64) float64 { return 0.74 + 0.26*math.Pow(math.Abs(math.Cos(4*t)), 0.6) })
	case ShapeSunny:
		return polar(func(t float64) float64 { return 0.86 + 0.14*bump(t, 8, 1.6) })
	case ShapeVerySunny:
		return polar(func(t float64) float64 { return 0.8 + 0.2*bump(t, 12, 2.6) })
	case ShapeSoftBurst:
		return polar(func(t float64) float64 { return 0.78 + 0.22*bump(t, 10, 1.4) })
	case ShapeFlower:
		return polar(func(t float64) float64 { return 0.55 + 0.45*math.Pow(math.Abs(math.Cos(3*t)), 0.7) })
	case ShapePuffy:
		return polar(func(t float64) float64 { return 0.84 + 0.16*math.Pow(math.Abs(math.Cos(5*t)), 0.35) })
	case ShapeTriangle:
		return roundedPolygon(3, 0.32, 0)
	case ShapePentagon:
		return roundedPolygon(5, 0.26, 0)
	case ShapeDiamond:
		out := roundedPolygon(4, 0.24, 0)
		for i := range out {
			out[i] = pt{out[i].x * 0.82, out[i].y * 1.12}
		}
		return out
	case ShapeGem:
		out := roundedPolygon(6, 0.2, math.Pi/6)
		for i := range out {
			out[i] = pt{out[i].x * 0.86, out[i].y * 1.1}
		}
		return out
	case ShapePill:
		return stadium(1, 0.56)
	case ShapeOval:
		return polar(func(t float64) float64 {
			return 1 / math.Sqrt(math.Pow(math.Cos(t)/1.0, 2)+math.Pow(math.Sin(t)/0.72, 2))
		})
	case ShapeArch:
		return arch()
	case ShapePebble:
		return polar(func(t float64) float64 {
			return (1 + 0.035*math.Sin(2*t+0.6)) / math.Sqrt(math.Pow(math.Cos(t), 2)+math.Pow(math.Sin(t)/0.8, 2))
		})
	case ShapeMochi:
		return polar(func(t float64) float64 {
			return superellipse(t, 1, 0.86, 2.7) * (1 - 0.04*math.Cos(t))
		})
	case ShapeBean:
		return polar(func(t float64) float64 {
			a := math.Atan2(math.Sin(t), math.Cos(t))
			return superellipse(t, 1, 0.72, 2.2) * (1 - 0.1*math.Exp(-a*a/0.22))
		})
	case ShapeCapsule:
		return stadium(1, 0.74)
	case ShapeDroplet:
		return polar(func(t float64) float64 {
			return 0.74 + 0.26*math.Pow((1+math.Cos(t))/2, 3)
		})
	case ShapeGhost:
		return polar(func(t float64) float64 {
			if math.Cos(t) > 0 {
				return 0.86
			}
			return superellipse(t, 0.86, 0.98, 3.4)
		})
	case ShapeHeart:
		out := make([]pt, n)
		for i := 0; i < n; i++ {
			t := 2 * math.Pi * float64(i) / n
			x := 16 * math.Pow(math.Sin(t), 3)
			y := -(13*math.Cos(t) - 5*math.Cos(2*t) - 2*math.Cos(3*t) - math.Cos(4*t))
			out[i] = pt{x / 17, y/17 + 0.08}
		}
		return out
	}
	return polar(func(float64) float64 { return 1 })
}

// roundedPolygon is a regular k-gon with corners rounded by round, sampled by
// normal direction.
func roundedPolygon(k int, round, phase float64) []pt {
	verts := make([]pt, k)
	for i := 0; i < k; i++ {
		a := -math.Pi/2 + phase + 2*math.Pi*float64(i)/float64(k)
		verts[i] = pt{math.Cos(a) * (1 - round), math.Sin(a) * (1 - round)}
	}
	out := make([]pt, 720)
	for i := 0; i < 720; i++ {
		a := 2 * math.Pi * float64(i) / 720
		nx, ny := math.Cos(a), math.Sin(a)
		best, bd := verts[0], math.Inf(-1)
		for _, v := range verts {
			if d := v.x*nx + v.y*ny; d > bd {
				bd, best = d, v
			}
		}
		out[i] = pt{best.x + nx*round, best.y + ny*round}
	}
	return out
}

func stadium(w, h float64) []pt {
	r := h
	out := make([]pt, 720)
	for i := 0; i < 720; i++ {
		a := 2 * math.Pi * float64(i) / 720
		cx := -(w - r)
		if math.Cos(a) >= 0 {
			cx = w - r
		}
		out[i] = pt{cx + math.Cos(a)*r, math.Sin(a) * r}
	}
	return out
}

func arch() []pt {
	var out []pt
	for i := 0; i <= 180; i++ {
		a := math.Pi + math.Pi*float64(i)/180
		out = append(out, pt{math.Cos(a), math.Sin(a) - 0.15})
	}
	for i := 1; i <= 40; i++ {
		out = append(out, pt{1, -0.15 + 0.85*float64(i)/40})
	}
	const rc = 0.28
	for i := 0; i <= 30; i++ {
		a := 2 * math.Pi * float64(i) / 120
		out = append(out, pt{1 - rc + rc*math.Cos(a), 0.7 + rc*math.Sin(a)})
	}
	for i := 1; i < 40; i++ {
		out = append(out, pt{1 - rc - (2-2*rc)*float64(i)/40, 0.98})
	}
	for i := 0; i <= 30; i++ {
		a := math.Pi/2 + 2*math.Pi*float64(i)/120
		out = append(out, pt{-1 + rc + rc*math.Cos(a), 0.7 + rc*math.Sin(a)})
	}
	for i := 1; i < 40; i++ {
		out = append(out, pt{-1, 0.7 - 0.85*float64(i)/40})
	}
	return out
}

// superellipse is the radius at polar angle t, measured from the top.
func superellipse(t, a, b, n float64) float64 {
	return math.Pow(math.Pow(math.Abs(math.Sin(t)/a), n)+math.Pow(math.Abs(math.Cos(t)/b), n), -1/n)
}

// polarOf ray-casts the outline from its centroid at uniform angles.
func polarOf(pts []pt) []float64 {
	var cx, cy float64
	for _, p := range pts {
		cx, cy = cx+p.x, cy+p.y
	}
	cx, cy = cx/float64(len(pts)), cy/float64(len(pts))
	rel := make([]pt, len(pts))
	for i, p := range pts {
		rel[i] = pt{p.x - cx, p.y - cy}
	}
	out := make([]float64, shapeSamples)
	for i := 0; i < shapeSamples; i++ {
		a := -math.Pi/2 + 2*math.Pi*float64(i)/shapeSamples
		dx, dy := math.Cos(a), math.Sin(a)
		best := 0.0
		for j := range rel {
			if hit, ok := raySegment(dx, dy, rel[j], rel[(j+1)%len(rel)]); ok && hit > best {
				best = hit
			}
		}
		out[i] = best
	}
	return out
}

func raySegment(dx, dy float64, p, q pt) (float64, bool) {
	ex, ey := q.x-p.x, q.y-p.y
	den := dx*ey - dy*ex
	if math.Abs(den) < 1e-12 {
		return 0, false
	}
	t := (p.x*ey - p.y*ex) / den
	u := (p.x*dy - p.y*dx) / den
	if t <= 0 || u < -1e-9 || u > 1+1e-9 {
		return 0, false
	}
	return t, true
}

func normalise(r []float64) []float64 {
	m := 0.0
	for _, v := range r {
		m = math.Max(m, v)
	}
	if m == 0 {
		return r
	}
	for i := range r {
		r[i] /= m
	}
	return r
}
