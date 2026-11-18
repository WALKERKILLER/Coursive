package app

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
)

type since struct{ at time.Time }

// entered is how far an element is through an animation that starts when
// it first shows, from 0 to 1, as an AnimationController forwarded in
// initState; reduced motion starts it finished.
func entered(c *ui.Context, e ui.Element, key string, d time.Duration) float32 {
	st := ui.Local(e, key, func() since { return since{time.Now()} })
	if design.T().ReduceMotion || d <= 0 {
		return 1
	}
	t := float32(time.Since(st.at)) / float32(d)
	if t >= 1 {
		return 1
	}
	c.Invalidate()
	return t
}

// interval is Interval(begin, end, curve) of t.
func interval(t, begin, end float32, ease ui.Easing) float32 {
	v := min(max((t-begin)/(end-begin), 0), 1)
	if ease != nil {
		return ease(v)
	}
	return v
}
