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

// Dragging a switch's thumb moves it with the pointer, and the switch
// takes the side it is let go on; let go on the side it came from, it
// stays as it was, the release clicking nothing; a tap still toggles it.
func TestSwitchFollowsADrag(t *testing.T) {
	design.Use(design.NewTheme(design.DefaultPrefs(), false, ui.Color{}))
	on, changes := false, 0
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Padding(10).Children(func() {
			if Switch(c, &on, false).Changed() {
				changes++
			}
		})
	}, 100, 60)
	drag := func(from, to float32) {
		tt.Press(from, 26)
		tt.Advance(16 * time.Millisecond)
		for x := from; x != to; {
			x += max(min(to-x, 2), -2)
			tt.Move(x, 26)
			tt.Advance(16 * time.Millisecond)
		}
		tt.Release(to, 26)
		tt.Advance(400 * time.Millisecond)
	}
	drag(26, 34)
	if on || changes != 0 {
		t.Fatalf("let go short of half way: on %v, %d changes", on, changes)
	}
	drag(26, 46)
	if !on || changes != 1 {
		t.Fatalf("let go past half way: on %v, %d changes", on, changes)
	}
	drag(46, 26)
	if on || changes != 2 {
		t.Fatalf("dragged back: on %v, %d changes", on, changes)
	}
	drag(30, 30)
	if !on || changes != 3 {
		t.Fatalf("tapped: on %v, %d changes", on, changes)
	}
}
