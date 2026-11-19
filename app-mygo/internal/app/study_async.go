package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
)

// The canvas's async work runs as the Dart awaits it: on its own goroutine,
// stepping onto the UI thread for each change of state. These are called
// from such goroutines only.

// onUI runs fn on the UI thread and waits for it.
func onUI(fn func()) {
	done := make(chan struct{})
	core.Post(func() {
		defer close(done)
		fn()
	})
	<-done
}

// onUIValue is onUI returning fn's value.
func onUIValue[T any](fn func() T) T {
	var v T
	onUI(func() { v = fn() })
	return v
}

// flushWait is flush, awaited.
func (s *StudyCanvas) flushWait() bool {
	ch := make(chan bool, 1)
	core.Post(func() { s.flush(func(ok bool) { ch <- ok }) })
	return <-ch
}

// awaitDialog shows a dialog and waits for the value it pops.
func awaitDialog(build func(c *ui.Context, r *m3.Route)) any {
	ch := make(chan any, 1)
	core.Post(func() { m3.ShowDialog(build, func(v any) { ch <- v }) })
	return <-ch
}
