package m3

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
)

// heroKey is a Hero by its navigator, the page it is on and its tag.
type heroKey struct {
	nav  *Navigator
	page int
	tag  any
}

// hero is a Hero as a frame built it: its element and what it shows.
type hero struct {
	el      ui.Element
	shuttle func(e ui.Element)
	built   time.Time
}

// heroes are the Heroes built lately.
var heroes = map[heroKey]*hero{}

// heroFlight is a _HeroFlight: from the Hero on the page leaving to the
// one on the page coming, at rest in the window, showing what the one
// coming shows.
type heroFlight struct {
	from, to ui.Rect
	shuttle  func(e ui.Element)
}

// Hero is Flutter's Hero under MaterialApp's HeroController: as a page of
// its navigator is pushed or popped, the Hero with the same tag on the
// page leaving and on the page coming flies from one's place to the
// other's, along MaterialRectArcTween's arcs and Curves.fastOutSlowIn
// over the page transition, above both pages, showing what the coming one
// shows; both stay in place unseen meanwhile. show builds the Hero's
// content in e, and in the flight's element.
func Hero(c *ui.Context, e ui.Element, tag any, show func(e ui.Element)) ui.Element {
	n := Nav()
	if n != nil {
		heroes[heroKey{n, n.building, tag}] = &hero{el: e, shuttle: show, built: c.Now()}
		if _, ok := n.flights[tag]; ok {
			e = e.Opacity(0)
		}
	}
	show(e)
	return e
}

// flyHeroes starts the flights of the page transition, once the Heroes of
// the page coming are laid out, and builds them in box, the navigator's
// element, t of the way through the transition.
func (n *Navigator) flyHeroes(c *ui.Context, box ui.Element, from, to int, t float32) {
	now := c.Now()
	for k, h := range heroes {
		if now.Sub(h.built) > time.Second {
			delete(heroes, k)
		}
	}
	if !n.flown {
		ready := true
		flights := map[any]*heroFlight{}
		for k, a := range heroes {
			if k.nav != n || k.page != from || a.built != now {
				continue
			}
			b := heroes[heroKey{n, to, k.tag}]
			if b == nil || b.built != now {
				continue
			}
			fr, tr := a.el.LayoutBounds(), b.el.LayoutBounds()
			if tr.W == 0 || tr.H == 0 {
				ready = false
				continue
			}
			flights[k.tag] = &heroFlight{from: fr, to: tr, shuttle: b.shuttle}
		}
		// The page coming takes a frame to lay out its Heroes.
		if ready && n.frames > 0 {
			n.flights, n.flown = flights, true
		}
		n.frames++
	}
	if len(n.flights) == 0 {
		return
	}
	nb := box.LayoutBounds()
	p := design.FastOutSlowIn(t)
	for tag, f := range n.flights {
		r := arcRect(f.from, f.to, p)
		e := ui.Box(c.Key(heroFlightKey{tag})).Absolute().Left(r.X-nb.X).Top(r.Y-nb.Y).Size(r.W, r.H).IgnoreInput(true)
		f.shuttle(e)
	}
}

type heroFlightKey struct{ tag any }

// arcRect is MaterialRectArcTween's rect t of the way from a to b: the
// corners of the diagonal that best follows the way between their centres
// go along MaterialPointArcTween's arcs.
func arcRect(a, b ui.Rect, t float32) ui.Rect {
	if t <= 0 {
		return a
	}
	if t >= 1 {
		return b
	}
	corner := func(r ui.Rect, i int) [2]float64 {
		x, y := float64(r.X), float64(r.Y)
		if i == 1 || i == 3 {
			x += float64(r.W)
		}
		if i >= 2 {
			y += float64(r.H)
		}
		return [2]float64{x, y}
	}
	// The corners topLeft, topRight, bottomLeft, bottomRight, and the
	// diagonals as _allDiagonals has them.
	diagonals := [4][2]int{{0, 3}, {3, 0}, {1, 2}, {2, 1}}
	cx := float64(b.X+b.W/2) - float64(a.X+a.W/2)
	cy := float64(b.Y+b.H/2) - float64(a.Y+a.H/2)
	best, most := diagonals[0], math.Inf(-1)
	for _, d := range diagonals {
		p, q := corner(a, d[0]), corner(a, d[1])
		dx, dy := q[0]-p[0], q[1]-p[1]
		l := math.Hypot(dx, dy)
		if s := cx*dx/l + cy*dy/l; s > most {
			best, most = d, s
		}
	}
	p := arcPoint(corner(a, best[0]), corner(b, best[0]), float64(t))
	q := arcPoint(corner(a, best[1]), corner(b, best[1]), float64(t))
	x0, x1 := math.Min(p[0], q[0]), math.Max(p[0], q[0])
	y0, y1 := math.Min(p[1], q[1]), math.Max(p[1], q[1])
	return ui.Rect{X: float32(x0), Y: float32(y0), W: float32(x1 - x0), H: float32(y1 - y0)}
}

// arcPoint is MaterialPointArcTween's point t of the way from a to b.
func arcPoint(a, b [2]float64, t float64) [2]float64 {
	dx, dy := b[0]-a[0], b[1]-a[1]
	adx, ady := math.Abs(dx), math.Abs(dy)
	if adx <= 2 || ady <= 2 {
		return [2]float64{a[0] + dx*t, a[1] + dy*t}
	}
	dist := math.Hypot(dx, dy)
	sign := func(v float64) float64 {
		switch {
		case v > 0:
			return 1
		case v < 0:
			return -1
		}
		return 0
	}
	var radius, cx, cy, from, to float64
	sweep := func() float64 { return 2 * math.Asin(dist/(2*radius)) }
	if adx < ady {
		// c is (b.x, a.y): its distance from a is |dx|.
		radius = dist * dist / adx / 2
		cx, cy = b[0]+radius*sign(a[0]-b[0]), b[1]
		if a[0] < b[0] {
			from, to = sweep()*sign(a[1]-b[1]), 0
		} else {
			from, to = math.Pi+sweep()*sign(b[1]-a[1]), math.Pi
		}
	} else {
		// Its distance from b is |dy|.
		radius = dist * dist / ady / 2
		cx, cy = a[0], a[1]+sign(b[1]-a[1])*radius
		if a[1] < b[1] {
			from = -math.Pi / 2
			to = from + sweep()*sign(b[0]-a[0])
		} else {
			from = math.Pi / 2
			to = from + sweep()*sign(a[0]-b[0])
		}
	}
	angle := from + (to-from)*t
	return [2]float64{cx + math.Cos(angle)*radius, cy + math.Sin(angle)*radius}
}
