package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
)

// ActivityRegion is activity_tracker.dart's: it counts reading or practice
// time while its element is shown and the user interacts with it. A page
// keeps one, shows it on its body every frame, and stops it on Dispose.
type ActivityRegion struct {
	clock         *core.ActivityClock
	kind, lecture string
	x, y          float32
	mark          any
}

// Show starts the clock for kind and lecture on e, and counts the pointer
// moving over it, keys while it has the focus and mark changing (the
// reader's position, a question answered) as interaction.
func (a *ActivityRegion) Show(c *ui.Context, e ui.Element, kind, lecture string, mark any) {
	if a.clock == nil || a.kind != kind || a.lecture != lecture {
		a.Stop()
		a.kind, a.lecture = kind, lecture
		if core.Tracker != nil {
			a.clock = core.Tracker.Clock(kind, lecture, true)
			if a.clock != nil {
				a.clock.Start()
			}
		}
	}
	if a.clock == nil {
		return
	}
	touched := mark != a.mark
	a.mark = mark
	if x, y, over := e.PointerPosition(); over && (x != a.x || y != a.y) {
		a.x, a.y = x, y
		touched = true
	}
	if e.FocusWithin() && len(c.KeysDown()) > 0 {
		touched = true
	}
	if touched {
		a.clock.Touch()
	}
}

// Stop stops the clock.
func (a *ActivityRegion) Stop() {
	if a.clock != nil {
		a.clock.Stop()
		a.clock = nil
	}
}
