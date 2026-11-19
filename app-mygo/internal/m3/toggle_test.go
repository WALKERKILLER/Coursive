package m3

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
)

// A toggle's controller runs as AnimationController.forward and reverse:
// linearly, a turn on the way taking the part of the duration left, and
// its curves keep the direction it set out in until it rests.
func TestControllerTurnsAsFlutterDoes(t *testing.T) {
	on := false
	var got ctl
	tt := ui.NewTester(func(c *ui.Context) {
		got = controller(ui.Box(c).Size(10, 10), "k", on, 300*time.Millisecond)
	}, 10, 10)
	if got.v != 0 || got.status != ctlDismissed {
		t.Fatalf("at rest: %+v", got)
	}
	on = true
	tt.Frame()
	tt.Advance(150 * time.Millisecond)
	if got.v < .45 || got.v > .55 || got.status != ctlForward {
		t.Fatalf("half way on: %+v", got)
	}
	on = false
	tt.Frame()
	tt.Advance(60 * time.Millisecond)
	if got.status != ctlReverse || got.dir != ctlForward {
		t.Fatalf("turned back: %+v", got)
	}
	if c := got.curved(design.EaseOutBack, design.Flipped(design.EaseOutBack)); c != design.EaseOutBack(got.v) {
		t.Fatalf("turned back along %v, not the forward curve", c)
	}
	tt.Advance(100 * time.Millisecond)
	if got.v != 0 || got.status != ctlDismissed {
		t.Fatalf("back after the half it went: %+v", got)
	}
}
