package m3

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
)

// Page is a route's page: an object holding its state, which builds its
// view every frame while it shows, as a StatefulWidget's State does.
type Page interface {
	View(c *ui.Context)
}

// Disposer is a page that cleans up as it is popped (State.dispose).
type Disposer interface{ Dispose() }

// PopGuard is a page that may refuse to be popped (PopScope).
type PopGuard interface{ CanPop() bool }

// Navigator is a stack of pages, as Flutter's Navigator: the top one shows,
// and pushing or popping fades it forwards (FadeForwardsPageTransitionsBuilder,
// which the theme uses on desktops).
type Navigator struct {
	pages []navEntry
	next  int
	// The page leaving, while it animates away.
	leaving   *navEntry
	changedAt time.Time
	popping   bool
	// OnChange runs after the stack changes.
	OnChange func()
}

type navEntry struct {
	page Page
	id   int
	// then runs once the page is popped (push(...).then).
	then func()
}

// PageDuration is FadeForwardsPageTransitionsBuilder.kTransitionMilliseconds,
// which the Heroes of a page fly over too.
const PageDuration = 450 * time.Millisecond

// NewNavigator is a navigator showing root.
func NewNavigator(root Page) *Navigator {
	n := &Navigator{}
	n.pages = []navEntry{{page: root}}
	n.next = 1
	return n
}

var navStack []*Navigator

// Nav is the navigator of the page being built (Navigator.of(context)).
// Capture it while building to use it from callbacks.
func Nav() *Navigator {
	if len(navStack) == 0 {
		return nil
	}
	return navStack[len(navStack)-1]
}

// RootNav is the outermost navigator (rootNavigator: true).
var RootNav *Navigator

func (n *Navigator) changed() {
	if n.OnChange != nil {
		n.OnChange()
	}
}

// Push shows p above the current page.
func (n *Navigator) Push(p Page) {
	prev := n.pages[len(n.pages)-1]
	n.pages = append(n.pages, navEntry{page: p, id: n.next})
	n.next++
	n.leaving, n.popping, n.changedAt = &prev, false, time.Now()
	n.changed()
}

// PushThen is Push running then once p is popped.
func (n *Navigator) PushThen(p Page, then func()) {
	n.Push(p)
	n.pages[len(n.pages)-1].then = then
}

// Replace swaps the top page for p (pushReplacement).
func (n *Navigator) Replace(p Page) {
	top := n.pages[len(n.pages)-1]
	n.pages[len(n.pages)-1] = navEntry{page: p, id: n.next}
	n.next++
	if d, ok := top.page.(Disposer); ok {
		d.Dispose()
	}
	n.leaving, n.popping, n.changedAt = &top, false, time.Now()
	n.changed()
}

// CanPop reports whether there is a page below the top one.
func (n *Navigator) CanPop() bool { return len(n.pages) > 1 }

// MaybePop pops the top page unless it is the root or refuses.
func (n *Navigator) MaybePop() bool {
	if len(n.pages) <= 1 {
		return false
	}
	if g, ok := n.pages[len(n.pages)-1].page.(PopGuard); ok && !g.CanPop() {
		return false
	}
	n.Pop()
	return true
}

// Pop removes the top page.
func (n *Navigator) Pop() {
	if len(n.pages) <= 1 {
		return
	}
	top := n.pages[len(n.pages)-1]
	n.pages = n.pages[:len(n.pages)-1]
	if d, ok := top.page.(Disposer); ok {
		d.Dispose()
	}
	n.leaving, n.popping, n.changedAt = &top, true, time.Now()
	n.changed()
	if top.then != nil {
		top.then()
	}
}

// PopUntilRoot pops every page above the root (popUntil(isFirst)).
func (n *Navigator) PopUntilRoot() {
	for len(n.pages) > 1 {
		n.Pop()
	}
}

// PopUntil pops the pages above p, when p is in the stack
// (popUntil(route == p)).
func (n *Navigator) PopUntil(p Page) {
	found := false
	for _, e := range n.pages {
		found = found || e.page == p
	}
	for found && n.pages[len(n.pages)-1].page != p {
		n.Pop()
	}
}

// PopPage removes p wherever it is in the stack.
func (n *Navigator) PopPage(p Page) {
	for i := len(n.pages) - 1; i > 0; i-- {
		if n.pages[i].page == p {
			if i == len(n.pages)-1 {
				n.Pop()
				return
			}
			if d, ok := p.(Disposer); ok {
				d.Dispose()
			}
			then := n.pages[i].then
			n.pages = append(n.pages[:i], n.pages[i+1:]...)
			n.changed()
			if then != nil {
				then()
			}
			return
		}
	}
}

// Depth is the number of pages above the root.
func (n *Navigator) Depth() int { return len(n.pages) - 1 }

// Top is the page that shows.
func (n *Navigator) Top() Page { return n.pages[len(n.pages)-1].page }

// Root is the first page.
func (n *Navigator) Root() Page { return n.pages[0].page }

// Pages is the stack, bottom first.
func (n *Navigator) Pages() []Page {
	out := make([]Page, len(n.pages))
	for i, e := range n.pages {
		out[i] = e.page
	}
	return out
}

// View builds the top page, and the page leaving while it animates, in an
// element filling its parent.
func (n *Navigator) View(c *ui.Context) ui.Element {
	navStack = append(navStack, n)
	defer func() { navStack = navStack[:len(navStack)-1] }()
	el := time.Since(n.changedAt)
	if design.T().ReduceMotion {
		el = PageDuration
	}
	t := float32(el) / float32(PageDuration)
	if t >= 1 {
		n.leaving = nil
	}
	top := n.pages[len(n.pages)-1]
	ease := design.EaseInOutCubicEmphasized
	return ui.Box(c).Fill().Clip().Children(func() {
		if n.leaving != nil {
			c.Invalidate()
			// What the leaving page does: pushed over, it slides a quarter
			// to the left and fades in the first quarter; popped, it slides
			// a quarter to the right as it fades.
			lv := *n.leaving
			var dx, op float32
			if n.popping {
				dx = 25 * ease(t)
				op = 1 - min(t/.25, 1)
			} else {
				dx = -25 * ease(t)
				op = 1 - min(t/.25, 1)
			}
			ui.Column(c.Key(lv.id)).Absolute().Top(0).Left(0).Right(0).Bottom(0).Background(cs().Surface).
				TranslatePercent(dx, 0).Opacity(op).IgnoreInput(true).Children(func() { lv.page.View(c) })
			var tdx float32
			if n.popping {
				tdx = -25 * (1 - ease(t))
			} else {
				tdx = 25 * (1 - ease(t))
			}
			ui.Column(c.Key(top.id)).Absolute().Top(0).Left(0).Right(0).Bottom(0).
				TranslatePercent(tdx, 0).Opacity(min(t/.75, 1)).Children(func() { top.page.View(c) })
			return
		}
		ui.Column(c.Key(top.id)).Absolute().Top(0).Left(0).Right(0).Bottom(0).Children(func() { top.page.View(c) })
	})
}

// PageFunc makes a page of a function, for pages without state of their own.
type PageFunc func(c *ui.Context)

func (f PageFunc) View(c *ui.Context) { f(c) }
