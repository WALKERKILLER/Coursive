package m3

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
)

// Tips show and go as RawTooltip's: after the pointer rests 500 ms, at
// once on the next anchor while one shows, 100 ms after the pointer leaves,
// and not again after a click until the pointer comes back.
func TestTipsShowAndGoAsRawTooltip(t *testing.T) {
	design.Use(design.NewTheme(design.DefaultPrefs(), false, ui.Color{}))
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Gap(100).Padding(50).Children(func() {
			Tip(c, ui.ButtonBase(c.Key("a")).Size(40, 40), "tipA")
			Tip(c, ui.ButtonBase(c.Key("b")).Size(40, 40), "tipB")
		})
	}, 400, 300)
	tt.Frame()
	check := func(when, s string, want bool) {
		t.Helper()
		if tt.HasText(s) != want {
			t.Fatalf("%s: %s shows %v", when, s, !want)
		}
	}
	tt.Move(70, 70)
	tt.Advance(450 * time.Millisecond)
	check("resting 450 ms", "tipA", false)
	tt.Advance(100 * time.Millisecond)
	check("resting 550 ms", "tipA", true)
	tt.Move(210, 70)
	tt.Advance(16 * time.Millisecond)
	check("on the next anchor", "tipB", true)
	tt.Move(390, 290)
	tt.Advance(60 * time.Millisecond)
	check("60 ms after leaving", "tipB", true)
	tt.Advance(60 * time.Millisecond)
	check("120 ms after leaving", "tipB", false)
	tt.Move(70, 70)
	tt.Advance(600 * time.Millisecond)
	check("back on the first", "tipA", true)
	tt.Press(70, 70)
	tt.Advance(50 * time.Millisecond)
	tt.Release(70, 70)
	tt.Advance(800 * time.Millisecond)
	check("after a click", "tipA", false)
}
