package widgets

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
)

// Loader is loading.dart's ExpressiveLoader: a rounded cookie that morphs
// between lobe counts as it turns, for short waits.
type Loader struct {
	Size      float32
	Contained bool
	Color     ui.Color
}

// loaderShapes are the (lobes, amplitude) shapes it morphs through.
var loaderShapes = [][2]float64{{7, .10}, {4, .16}, {9, .08}, {5, .14}, {2, .20}, {6, .12}, {3, .18}}

// Show builds the loader.
func (l Loader) Show(c *ui.Context) ui.Element {
	s := design.S()
	size := l.Size
	if size == 0 {
		size = 48
	}
	fg := l.Color
	if fg.A == 0 {
		fg = s.Primary
		if l.Contained {
			fg = s.OnPrimaryContainer
		}
	}
	e := ui.Box(c).Size(size, size).Shrink(0).Role(ui.RoleProgress).Label(l10n.CommonLoading)
	if l.Contained {
		e = e.Background(s.PrimaryContainer).Radius(size / 2)
	}
	t := e.Loop("loader", 4200*time.Millisecond, ui.Linear)
	inner := size
	if l.Contained {
		inner = size * .79
	}
	return e.Draw(func(p *ui.Painter, r ui.Rect) {
		box := ui.Rect{X: r.X + (r.W-inner)/2, Y: r.Y + (r.H-inner)/2, W: inner, H: inner}
		path := morphPath(float64(t), box)
		p.FillPath(&path, fg)
		p.AnimationFrame()
	})
}

func morphPath(t float64, r ui.Rect) ui.Path {
	n := len(loaderShapes)
	pos := t * float64(n)
	i := int(math.Floor(pos)) % n
	local := pos - math.Floor(pos)
	m := float64(design.EaseInOutCubicEmphasized(float32(min(max((local-.45)/.55, 0), 1))))
	a, b := loaderShapes[i], loaderShapes[(i+1)%n]
	cx, cy := float64(r.X+r.W/2), float64(r.Y+r.H/2)
	base := float64(min(r.W, r.H)) / 2 * .86
	rot := t*math.Pi*2*3 + m*math.Pi/3
	var path ui.Path
	const steps = 120
	for k := 0; k <= steps; k++ {
		th := float64(k) / steps * math.Pi * 2
		ra := 1 + a[1]*math.Cos(a[0]*th)
		rb := 1 + b[1]*math.Cos(b[0]*th)
		rr := base * (ra + (rb-ra)*m) / (1 + math.Max(a[1], b[1]))
		x, y := float32(cx+math.Cos(th+rot)*rr), float32(cy+math.Sin(th+rot)*rr)
		if k == 0 {
			path.MoveTo(x, y)
		} else {
			path.LineTo(x, y)
		}
	}
	return *path.Close()
}
