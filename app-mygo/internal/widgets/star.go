package widgets

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// StarPath is Flutter's StarBorder outline (star_border.dart's
// _StarGenerator) in r, without rotation or squash: points rounded by
// pointRounding, valleys by valleyRounding, the conics drawn as cubics.
func StarPath(r ui.Rect, points int, innerRatio, pointRounding, valleyRounding float64) ui.Path {
	radius := float64(min(r.W, r.H)) / 2
	cx, cy := float64(r.X+r.W/2), float64(r.Y+r.H/2)
	inner := radius * (innerRatio*(1-.002) + .002)
	type pt struct{ x, y float64 }
	type info struct{ valley, point, va1, pa1, pa2, va2 pt }
	lerp := func(a, b pt, t float64) pt { return pt{a.x + (b.x-a.x)*t, a.y + (b.y-a.y)*t} }
	step := math.Pi / float64(points)
	angle := -math.Pi/2 - step
	valley := pt{cx + math.Cos(angle)*inner, cy + math.Sin(angle)*inner}
	var list []info
	for i := 0; i < points; i++ {
		angle += step
		p := pt{cx + math.Cos(angle)*radius, cy + math.Sin(angle)*radius}
		angle += step
		next := pt{cx + math.Cos(angle)*inner, cy + math.Sin(angle)*inner}
		list = append(list, info{valley, p, lerp(valley, p, valleyRounding), lerp(p, valley, pointRounding),
			lerp(p, next, pointRounding), lerp(next, p, valleyRounding)})
		valley = next
	}
	getAngle := func(a, b, c pt) float64 {
		if a == c || b == c || b == a {
			return 0
		}
		ux, uy, vx, vy := a.x-b.x, a.y-b.y, c.x-b.x, c.y-b.y
		dot := ux*vx + uy*vy
		m1, m2 := math.Inf(1), math.Inf(1)
		if b.x != a.x {
			m1 = -uy / -ux
		}
		if b.x != c.x {
			m2 = -vy / -vx
		}
		an := math.Abs(math.Atan2(m1-m2, 1+m1*m2))
		if dot < 0 {
			an += math.Pi
		}
		return an
	}
	weight := func(a float64) float64 { return math.Cos(math.Mod(a/2, math.Pi/2)) }
	mid := func(a, b, c, a1, c1 pt) pt {
		w := weight(getAngle(a, b, c)) / 2
		return pt{(a1.x/4 + b.x*w + c1.x/4) / (.5 + w), (a1.y/4 + b.y*w + c1.y/4) / (.5 + w)}
	}
	t, n := list[0], list[1]
	pm := mid(t.valley, t.point, n.valley, t.pa1, t.pa2)
	vm := mid(t.point, n.valley, n.point, t.va2, n.va1)
	maxD := 2 * math.Max(math.Hypot(vm.x-cx, vm.y-cy), math.Hypot(pm.x-cx, pm.y-cy))
	s := float64(min(r.W, r.H)) / maxD
	tr := func(p pt) (float32, float32) { return float32(cx + (p.x-cx)*s), float32(cy + (p.y-cy)*s) }
	pw := weight(getAngle(list[0].valley, list[0].point, list[1].valley))
	vw := weight(getAngle(list[1].point, list[1].valley, list[0].point))
	var path ui.Path
	conic := func(p0, p1, p2 pt, w float64) {
		k := 4 * w / (3 * (1 + w))
		ax, ay := tr(lerp(p0, p1, k))
		bx, by := tr(lerp(p2, p1, k))
		ex, ey := tr(p2)
		path.CubeTo(ax, ay, bx, by, ex, ey)
	}
	path.MoveTo(tr(list[0].pa1))
	for i, p := range list {
		np := list[(i+1)%len(list)]
		path.LineTo(tr(p.pa1))
		conic(p.pa1, p.point, p.pa2, pw)
		path.LineTo(tr(p.va2))
		conic(p.va2, np.valley, np.va1, vw)
	}
	return *path.Close()
}
