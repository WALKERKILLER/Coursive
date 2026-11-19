package m3

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
)

func TestSnackBarsShowOneAtATime(t *testing.T) {
	design.Use(design.NewTheme(design.DefaultPrefs(), false, ui.Color{}))
	defer ClearSnackBars()
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Size(600, 300)
		buildSnackBars(c)
	}, 600, 300)
	tt.Frame()
	ShowSnackBar(SnackBar{Message: "first", Duration: 400 * time.Millisecond})
	ShowSnackBar(SnackBar{Message: "second", Duration: 400 * time.Millisecond})
	tt.Frame()
	var gone, next time.Duration
	for at := 20 * time.Millisecond; at <= 1200*time.Millisecond; at += 20 * time.Millisecond {
		tt.Advance(20 * time.Millisecond)
		if _, ok := tt.Find("first"); !ok && gone == 0 {
			gone = at
		}
		if _, ok := tt.Find("second"); ok && next == 0 {
			next = at
		}
	}
	if gone == 0 || next == 0 {
		t.Fatalf("first went at %v, second came at %v", gone, next)
	}
	if wait := next - gone; wait < 240*time.Millisecond || wait > 280*time.Millisecond {
		t.Fatalf("second came %v after first went, want the 250 ms the first takes to go", wait)
	}
}
