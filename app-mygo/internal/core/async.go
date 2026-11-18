package core

// Post runs fn on the UI thread and draws a new frame. main sets it to the
// window's Update before anything else runs.
var Post = func(fn func()) { fn() }

// Async runs work on its own goroutine and hands the result to done on the
// UI thread, as awaiting a Future in a State does.
func Async[T any](work func() (T, error), done func(T, error)) {
	go func() {
		v, err := work()
		if done != nil {
			Post(func() { done(v, err) })
		}
	}()
}

// Go runs work on its own goroutine, then done on the UI thread with its
// error.
func Go(work func() error, done func(error)) {
	go func() {
		err := work()
		if done != nil {
			Post(func() { done(err) })
		}
	}()
}

// Events is a broadcast of daemon events to subscribers on the UI thread.
type Events struct {
	next int
	subs map[int]func(J)
}

// Listen subscribes fn; the returned function unsubscribes it.
func (e *Events) Listen(fn func(J)) (cancel func()) {
	if e.subs == nil {
		e.subs = map[int]func(J){}
	}
	id := e.next
	e.next++
	e.subs[id] = fn
	return func() { delete(e.subs, id) }
}

// Emit hands ev to every subscriber.
func (e *Events) Emit(ev J) {
	for _, fn := range e.subs {
		fn(ev)
	}
}
