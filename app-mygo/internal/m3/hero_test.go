package m3

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
)

// arcRect matches MaterialRectArcTween.lerp, as Flutter computes it for
// these rects.
func TestArcRectMatchesMaterialRectArcTween(t *testing.T) {
	cases := []struct {
		a, b ui.Rect
		want [3][4]float32
	}{
		{ui.Rect{X: 20, Y: 300, W: 160, H: 108}, ui.Rect{X: 24, Y: 80, W: 600, H: 150}, [3][4]float32{
			{21.750, 285.303, 285.480, 78.217}, {23.000, 241.984, 404.758, 77.038}, {23.750, 172.327, 511.485, 102.187}}},
		{ui.Rect{X: 24, Y: 80, W: 600, H: 150}, ui.Rect{X: 420, Y: 500, W: 160, H: 108}, [3][4]float32{
			{177.260, 118.776, 427.539, 204.609}, {305.389, 211.376, 285.648, 206.348}, {390.299, 344.726, 192.463, 167.976}}},
		{ui.Rect{X: 400, Y: 120, W: 200, H: 100}, ui.Rect{X: 30, Y: 90, W: 300, H: 180}, [3][4]float32{
			{306.448, 118.118, 225.532, 105.025}, {213.318, 112.477, 250.851, 120.080}, {121.030, 103.095, 275.743, 145.104}}},
	}
	for _, k := range cases {
		for i, at := range []float32{.25, .5, .75} {
			r := arcRect(k.a, k.b, at)
			got := [4]float32{r.X, r.Y, r.W, r.H}
			for j := range got {
				if math.Abs(float64(got[j]-k.want[i][j])) > .01 {
					t.Fatalf("%v → %v at %v: %v, Flutter %v", k.a, k.b, at, got, k.want[i])
				}
			}
		}
	}
}

// Pushing a page starts a flight from the Hero of the page below to the
// one of the page pushed, both at rest, and hides both meanwhile.
func TestHeroFliesBetweenPages(t *testing.T) {
	design.Use(design.NewTheme(design.DefaultPrefs(), false, ui.Color{}))
	type tag struct{}
	page := func(x, y, s float32, label string) PageFunc {
		return func(c *ui.Context) {
			Hero(c, ui.Box(c).Absolute().Left(x).Top(y).Size(s, s), tag{}, func(e ui.Element) {
				e.Children(func() { ui.Text(c, label) })
			})
		}
	}
	n := NewNavigator(page(10, 20, 40, "small"))
	tt := ui.NewTester(func(c *ui.Context) { n.View(c) }, 400, 400)
	tt.Frame()
	n.Push(page(200, 100, 120, "large"))
	for i := 0; i < 3; i++ {
		tt.Frame()
	}
	f := n.flights[tag{}]
	if f == nil {
		t.Fatalf("no flight: %v", n.flights)
	}
	if f.from != (ui.Rect{X: 10, Y: 20, W: 40, H: 40}) || f.to != (ui.Rect{X: 200, Y: 100, W: 120, H: 120}) {
		t.Fatalf("flight from %v to %v", f.from, f.to)
	}
}
