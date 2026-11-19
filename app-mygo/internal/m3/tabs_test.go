package m3

import (
	"strconv"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// A tab change slides the pages as TabBarView does: from the page next to
// the new one when it skips a tab, with the two pages side by side on the
// way, and the new page alone in place at the end.
func TestTabViewSlidesFromTheNeighbour(t *testing.T) {
	sel := 0
	tt := ui.NewTester(func(c *ui.Context) {
		TabView(c, ui.Box(c).Size(300, 200), sel, func(i int) {
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
