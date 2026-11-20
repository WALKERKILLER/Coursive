package m3

import (
	"strconv"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
)

// A tab change slides the pages as TabBarView does: from the page next to
// the new one when it skips a tab, with the two pages side by side on the
// way, and the new page alone in place at the end.
func TestTabViewSlidesFromTheNeighbour(t *testing.T) {
	sel := 0
	tt := ui.NewTester(func(c *ui.Context) {
		TabView(c, ui.Box(c).Size(300, 200), &sel, 3, func(i int) {
			ui.Text(c, "page"+strconv.Itoa(i))
		})
	}, 300, 200)
	if !tt.HasText("page0") || tt.HasText("page1") {
		t.Fatalf("start: %v", tt.Texts())
	}
	sel = 2
	tt.Frame()
	tt.Advance(120 * time.Millisecond)
	if tt.HasText("page0") || !tt.HasText("page1") || !tt.HasText("page2") {
		t.Fatalf("on the way: %v", tt.Texts())
	}
	if r, _ := tt.Find("page2"); r.X <= 0 || r.X >= 300 {
		t.Fatalf("page2 at %v on the way", r)
	}
	tt.Advance(300 * time.Millisecond)
	if tt.HasText("page1") || !tt.HasText("page2") {
		t.Fatalf("end: %v", tt.Texts())
	}
	if r, _ := tt.Find("page2"); r.X != 0 {
		t.Fatalf("page2 at %v at the end", r)
	}
}

// Tab labels cross-fade as TabBar's _ChangeAnimation does: on a change
// that skips a tab, the old and new labels part way between their colours
// while the one passed on the way keeps the unselected colour.
func TestTabLabelsCrossFadeBetweenTheOldAndNewTab(t *testing.T) {
	design.Use(design.NewTheme(design.DefaultPrefs(), false, ui.Color{}))
	sel := 0
	tt := ui.NewTester(func(c *ui.Context) {
		TabBar(c, []Tab{{Label: "AAAA"}, {Label: "BBBB"}, {Label: "CCCC"}}, &sel, TabOpts{})
	}, 480, 48)
	ink := func(s string) [3]int {
		r, ok := tt.Find(s)
		if !ok {
			t.Fatalf("no %s", s)
		}
		img := tt.Image()
		bg := img.RGBAAt(int(r.X)-2, int(r.Y+r.H/2))
		var best [3]int
		far := -1
		for y := int(r.Y); y < int(r.Y+r.H); y++ {
			for x := int(r.X); x < int(r.X+r.W); x++ {
				p := img.RGBAAt(x, y)
				d := abs(int(p.R)-int(bg.R)) + abs(int(p.G)-int(bg.G)) + abs(int(p.B)-int(bg.B))
				if d > far {
					far, best = d, [3]int{int(p.R), int(p.G), int(p.B)}
				}
			}
		}
		return best
	}
	a0, b0, c0 := ink("AAAA"), ink("BBBB"), ink("CCCC")
	if a0 == c0 || b0 != c0 {
		t.Fatalf("start: %v %v %v", a0, b0, c0)
	}
	sel = 2
	tt.Frame()
	tt.Advance(150 * time.Millisecond)
	am, bm, cm := ink("AAAA"), ink("BBBB"), ink("CCCC")
	if bm != b0 {
		t.Fatalf("the tab passed lit up: %v, was %v", bm, b0)
	}
	if am == a0 || am == c0 || cm == c0 || cm == a0 {
		t.Fatalf("mid-way: %v %v, ends %v %v", am, cm, a0, c0)
	}
	tt.Advance(300 * time.Millisecond)
	if a, c := ink("AAAA"), ink("CCCC"); a != c0 || c != a0 {
		t.Fatalf("end: %v %v", a, c)
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// Fixed tabs share the bar's width, their labels in the middle of each.
func TestFixedTabsShareTheWidth(t *testing.T) {
	design.Use(design.NewTheme(design.DefaultPrefs(), false, ui.Color{}))
	sel := 0
	tt := ui.NewTester(func(c *ui.Context) {
		TabBar(c, []Tab{{Label: "AAAA"}, {Label: "BBBB"}, {Label: "CCCC"}}, &sel, TabOpts{})
	}, 480, 48)
	for i, s := range []string{"AAAA", "BBBB", "CCCC"} {
		r, _ := tt.Find(s)
		if mid := r.X + r.W/2; abs32(mid-float32(80+160*i)) > 1 {
			t.Fatalf("%s centred at %v, not %v", s, mid, 80+160*i)
		}
	}
}

// The primary indicator stretches as _applyElasticEffect: on a tap to the
// right its right edge leads and its left edge follows; at rest it lies
// under the label.
func TestPrimaryIndicatorStretches(t *testing.T) {
	m := &tabBar{
		tabs:   []ui.Rect{{X: 0, W: 100}, {X: 100, W: 100}, {X: 200, W: 100}},
		labels: []ui.Rect{{X: 30, W: 40}, {X: 130, W: 40}, {X: 230, W: 40}},
	}
	ch := &tabChange{prev: 0, cur: 1}
	if r := m.indicator(TabOpts{}, 1, 1, false, ch); r.X != 130 || r.W != 40 {
		t.Fatalf("at rest: %+v", r)
	}
	r := m.indicator(TabOpts{}, .3, 1, true, ch)
	left, right := (r.X-30)/100, (r.X+r.W-70)/100
	if !(right > .3 && left < .3) {
		t.Fatalf("a third of the way: left edge %.2f, right edge %.2f of the way", left, right)
	}
	// The secondary indicator slides as a whole, tab wide.
	if r := m.indicator(TabOpts{Secondary: true}, .5, 1, true, ch); r.X != 50 || r.W != 100 {
		t.Fatalf("secondary half way: %+v", r)
	}
}

// A finger swiping the pages of a TabView moves them with it, and let go
// past half a page, or flung, they settle on the next page, which becomes
// the selected tab.
func TestTabViewFollowsAFinger(t *testing.T) {
	design.Use(design.NewTheme(design.DefaultPrefs(), false, ui.Color{}))
	sel := 0
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Size(300, 250).Children(func() {
			TabBar(c, []Tab{{Label: "A"}, {Label: "B"}, {Label: "C"}}, &sel, TabOpts{})
			TabView(c, ui.Box(c).FillWidth().Grow(1).Basis(0), &sel, 3, func(i int) {
				ui.Text(c, "page"+strconv.Itoa(i))
			})
		})
	}, 300, 250)
	tt.FingerDown(250, 150)
	for x := float32(250); x > 100; x -= 10 {
		tt.FingerMove(x, 150)
		tt.Advance(16 * time.Millisecond)
	}
	if !tt.HasText("page0") || !tt.HasText("page1") {
		t.Fatalf("under the finger: %v", tt.Texts())
	}
	if r, _ := tt.Find("page1"); r.X <= 0 || r.X >= 300 {
		t.Fatalf("page1 at %v under the finger", r)
	}
	tt.FingerUp(100, 150)
	tt.Advance(time.Second)
	if sel != 1 || tt.HasText("page0") {
		t.Fatalf("let go: tab %d, %v", sel, tt.Texts())
	}
	if r, _ := tt.Find("page1"); r.X != 0 {
		t.Fatalf("page1 settled at %v", r)
	}
}
