package design

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// A switch fades the new child in while the old one fades out, as
// AnimatedSwitcher does, and then only the new one is left.
func TestSwitcherEntersAndExitsTogether(t *testing.T) {
	key := "a"
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Size(200, 100).Background(ui.RGB(255, 255, 255)).Children(func() {
			ui.Box(c.Key(key)).Size(200, 100).Background(ui.RGB(0, 0, 0)).
				Transition(Switcher(Medium2, nil, nil, ui.Motion{Opacity: 0}))
		})
	}, 200, 100)
	level := func() uint8 { return tt.Image().RGBAAt(100, 50).R }
	if l := level(); l != 0 {
		t.Fatalf("the first child shows at once: %d", l)
	}
	key = "b"
	tt.Frame()
	tt.Advance(Medium2 / 2)
	// Linear halves: the new one at half opacity over the old one at half,
	// over white, which a vanished old child would leave far lighter.
	if l := level(); l > 110 {
		t.Fatalf("half way the old child is gone: %d", l)
	}
	tt.Advance(Medium2)
	if l := level(); l != 0 {
		t.Fatalf("at the end: %d", l)
	}
}

func TestFlipped(t *testing.T) {
	f := Flipped(EmphasizedAccelerate)
	for _, x := range []float32{0, .3, .7, 1} {
		if got, want := f(x), 1-EmphasizedAccelerate(1-x); got != want {
			t.Errorf("%v: %v, want %v", x, got, want)
		}
	}
}
