package m3

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
)

// A press paints its ink on the surface under the children, as InkWell
// does on its Material: an opaque child keeps its colour. The sparkle runs
// its course after a quick release, then the ink is gone and asks for no
// more frames.
func TestInkPaintsUnderChildrenAndSparklesThrough(t *testing.T) {
	design.Use(design.NewTheme(design.DefaultPrefs(), false, ui.Color{}))
	white, red := ui.RGB(255, 255, 255), ui.RGB(255, 0, 0)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(design.T().UI())
		Tappable(c, 0).Size(200, 100).Background(white).Children(func() {
			ui.Box(c).Absolute().Top(0).Left(0).Size(50, 50).Background(red)
		})
	}, 300, 100)
	at := func(x, y int) ui.Color {
		p := tt.Image().RGBAAt(x, y)
		return ui.RGB(p.R, p.G, p.B)
	}
	tt.Move(150, 50)
	tt.Press(150, 50)
	tt.Advance(30 * time.Millisecond)
	tt.Release(150, 50)
	tt.Move(250, 50)
	tt.Advance(300 * time.Millisecond)
	if got := at(25, 25); got != red {
		t.Fatalf("child under the ink: %v", got)
	}
	if got := at(150, 50); got == white {
		t.Fatalf("no sparkle 330ms after the press")
	}
	tt.Advance(400 * time.Millisecond)
	if got := at(150, 50); got != white {
		t.Fatalf("ink left after the sparkle: %v", got)
	}
	if tt.FramePending() {
		t.Fatalf("the ink still asks for frames")
	}
}
