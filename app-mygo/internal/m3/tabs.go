package m3

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
)

// Tab is a Tab of a TabBar.
type Tab struct {
	Label string
	Icon  icons.Icon
}

// TabOpts are TabBar's options.
type TabOpts struct {
	// Scrollable is isScrollable: tabs as wide as their labels, from the
	// start.
	Scrollable bool
	// Secondary is TabBar.secondary: an underline across the tab.
	Secondary bool
	// NoDivider drops the outlineVariant line below (dividerColor:
	// transparent).
	NoDivider bool
	// Pill is the player panel's look: 44 DIP tabs in labelLarge, the
	// selected one on a secondaryContainer pill 6 DIPs in from the top and
	// bottom, without a divider.
	Pill bool
}

// tabScroll is kTabScrollDuration, how long a tab change takes.
const tabScroll = 300 * time.Millisecond

// tabSync is the TabController a TabBar and the TabView of the same
// selected index share: the page the view shows while a drag, or the
// settling after it, moves it, which the bar's indicator and labels follow
// as TabBarView sets the controller's offset.
type tabSync struct {
	page  float32
	drag  bool
	frame time.Time
}

var tabSyncs = map[*int]*tabSync{}

func syncFor(selected *int) *tabSync {
	s := tabSyncs[selected]
	if s == nil {
		s = &tabSync{}
		tabSyncs[selected] = s
	}
	return s
}

type tabChange struct{ prev, cur int }

// tabBar is what a TabBar measured of its tabs in the last frame, from
// the start of the row: each tab's box and its label's.
type tabBar struct {
	tabs, labels []ui.Rect
}

// TabBar is Material 3's primary TabBar: labels in titleSmall, primary
// when selected, a 3 DIP indicator as wide as the label with rounded top
// corners, and a divider below. The indicator follows the controller as
// _IndicatorPainter does: stretching elastically between labels for the
// primary bar, sliding linearly between tabs for the secondary and pill
// ones; dragging the TabView of the same selected index carries it along.
// It returns whether the tab changed.
func TabBar(c *ui.Context, tabs []Tab, selected *int, o TabOpts) bool {
	s := cs()
	changed := false
	n := len(tabs)
	h := float32(48)
	if o.Pill {
		h, o.NoDivider = 44, true
	}
	for _, t := range tabs {
		if t.Icon != 0 && t.Label != "" {
			h = 64
		}
	}
	outer := ui.Column(c).FillWidth().Shrink(0)
	outer.Children(func() {
		var row ui.Element
		if o.Scrollable {
			row = ui.ScrollHorizontal(c).Scrollbars(false).Height(h).FillWidth()
		} else {
			row = ui.Row(c).Height(h).FillWidth()
		}
		row.Children(func() {
			row := ui.Row(c).FillHeight()
			if !o.Scrollable {
				row = row.FillWidth()
			}
			ch := ui.Local(row, "m3.tabs.change", func() tabChange { return tabChange{*selected, *selected} })
			moved := ch.cur != *selected
			if moved {
				ch.prev, ch.cur = ch.cur, *selected
			}
			m := ui.Local(row, "m3.tabs.measure", func() tabBar { return tabBar{} })
			// The controller's value: the TabView's page while a drag moves
			// it, else animated to the selected index over 300 ms along
			// Curves.ease.
			sync := syncFor(selected)
			dragged := sync.drag && c.Now().Sub(sync.frame) < 100*time.Millisecond
			var pos float32
			if dragged {
				pos = row.Jump("m3.tabs.pos", sync.page)
			} else {
				pos = row.AnimateTo("m3.tabs.pos", float32(*selected), design.T().Motion(tabScroll, design.Ease))
			}
			index := float32(*selected)
			changing := !dragged && pos != index
			progress := float32(1)
			if span := abs32(float32(ch.cur - ch.prev)); span > 0 {
				progress = clamp01(1 - abs32(pos-float32(ch.cur))/span)
			}
			selFG := s.Primary
			if o.Secondary {
				selFG = s.OnSurface
			}
			if o.Pill {
				selFG = s.OnSecondaryContainer
			}
			indicator := func() {
				if len(m.tabs) != n || n == 0 {
					return
				}
				r := m.indicator(o, pos, index, changing, ch)
				ind := ui.Box(c).Absolute().Left(r.X).Width(r.W).PassThrough()
				switch {
				case o.Pill:
					// An absolute box paints above the tabs in the flow; the
					// pill goes under them.
					ind.Top(6).Bottom(6).Radius(design.CornerFull).Background(s.SecondaryContainer).ZIndex(-1)
				case o.Secondary:
					ind.Bottom(0).Height(2).Background(s.Primary)
				default:
					ind.Bottom(0).Height(3).Radius(3, 3, 0, 0).Background(s.Primary)
				}
			}
			tabEls := make([]ui.Element, n)
			labels := make([]ui.Element, n)
			row.Children(func() {
				// The pill lies under the labels, the underline over the
				// tabs' ink.
				if o.Pill {
					indicator()
				}
				for i, t := range tabs {
					on := i == *selected
					// The labels take their colours from the controller as
					// _TabStyle does: on a tap, the new tab and the one before
					// it cross-fade as far as the controller has gone between
					// them, the tabs passed on the way staying put; while a
					// drag moves it, each as near the selected colour as the
					// page is to it.
					var near float32
					switch {
					case changing && i == ch.cur:
						near = progress
					case changing && i == ch.prev:
						near = 1 - progress
					case !changing:
						near = clamp01(1 - abs32(pos-float32(i)))
					}
					fg := design.Lerp(s.OnSurfaceVariant, selFG, near)
					if o.Pill {
						b := ui.ButtonBase(c.Key(i)).FocusRing(false).Row().AlignItems(ui.Center).
							FillHeight().Padding(0, 14).Cursor(ui.CursorPointer).Checked(on).Role(ui.RoleTab)
						// The side panel's overlayColor sets only the hover, 6%
						// onSurface over the whole tab in a stadium; the press
						// and focus keep the theme's InkWell colours.
						pillInk(b, s.OnSurface)
						tabEls[i] = b
						b.Children(func() {
							labels[i] = Text(c, t.Label, tt().LabelLarge.C(fg)).NoWrap()
						})
						if b.Clicked() && !on {
							*selected = i
							changed = true
						}
						continue
					}
					b := ui.ButtonBase(c.Key(i)).FocusRing(false).Column().AlignItems(ui.Center).Justify(ui.Center).
						FillHeight().Padding(0, 16).Cursor(ui.CursorPointer).Checked(on).Role(ui.RoleTab)
					if !o.Scrollable {
						b = b.Grow(1).Basis(0)
					}
					// _TabsPrimaryDefaultsM3.overlayColor: hover and focus in
					// primary on the selected tab and onSurface on the rest, the
					// press and its splash in primary on all. Secondary tabs
					// take onSurface throughout.
					over, pressC := s.OnSurface, s.Primary
					if o.Secondary {
						pressC = s.OnSurface
					} else if on {
						over = s.Primary
					}
					press := design.WithOpacity(pressC, .1)
					InkColors(b, 0, design.WithOpacity(over, .08), design.WithOpacity(over, .1), press, press)
					tabEls[i] = b
					b.Children(func() {
						labels[i] = ui.Column(c).AlignItems(ui.Center).Gap(2).Children(func() {
							if t.Icon != 0 {
								Icon(c, t.Icon, 24, fg)
							}
							if t.Label != "" {
								Text(c, t.Label, tt().TitleSmall.C(fg)).NoWrap()
							}
						})
					})
					if b.Clicked() && !on {
						*selected = i
						changed = true
					}
				}
				if !o.Pill {
					indicator()
				}
			})
			base := row.LayoutBounds()
			m.tabs, m.labels = m.tabs[:0], m.labels[:0]
			for i := range tabEls {
				r, l := tabEls[i].LayoutBounds(), labels[i].LayoutBounds()
				r.X, l.X = r.X-base.X, l.X-base.X
				m.tabs, m.labels = append(m.tabs, r), append(m.labels, l)
			}
			// A scrollable bar brings the selected tab to its middle as the
			// index changes, as _scrollToCurrentIndex does.
			if o.Scrollable && (moved || changed) && *selected < n {
				tabEls[*selected].ScrollIntoView(ui.ScrollIntoViewOptions{Inline: ui.ScrollCenter,
					Animation: &ui.Animation{Duration: tabScroll, Ease: design.Ease}})
			}
		})
		if !o.NoDivider {
			Divider(c)
		}
	})
	return changed
}

// indicator is _IndicatorPainter's rect for the controller at pos, the
// selected index at index: _applyElasticEffect for the primary bar's
// label indicator, _applyLinearEffect for the tab-wide ones.
func (m *tabBar) indicator(o TabOpts, pos, index float32, changing bool, ch *tabChange) ui.Rect {
	n := len(m.tabs)
	clampI := func(i int) int { return min(max(i, 0), n-1) }
	rect := func(i int) ui.Rect {
		if o.Pill || o.Secondary {
			return m.tabs[i]
		}
		return m.labels[i]
	}
	lerpRect := func(a, b ui.Rect, t float32) ui.Rect {
		return ui.Rect{X: lerp(a.X, b.X, t), W: lerp(a.X+a.W, b.X+b.W, t) - lerp(a.X, b.X, t)}
	}
	ceil := func(v float32) int { return int(math.Ceil(float64(v))) }
	floor := func(v float32) int { return int(math.Floor(float64(v))) }
	if o.Pill || o.Secondary {
		ltr := index > pos
		from := ceil(pos)
		if ltr {
			from = floor(pos)
		}
		from = clampI(from)
		to := from - 1
		if ltr {
			to = from + 1
		}
		to = clampI(to)
		return lerpRect(rect(from), rect(to), abs32(pos-float32(from)))
	}
	progressLeft := abs32(index - pos)
	to, from := clampI(ceil(pos)), 0
	from = clampI(to - 1)
	if progressLeft != 0 && changing {
		to, from = clampI(ch.cur), clampI(ch.prev)
	}
	toR, fromR := rect(to), rect(from)
	r := lerpRect(fromR, toR, abs32(pos-float32(from)))
	if pos == float32(n-1) {
		return r
	}
	var progress float32
	if changing {
		if d := abs32(float32(ch.cur - ch.prev)); d != 0 {
			progressLeft /= d
		}
		progress = 1 - clamp01(progressLeft)
	} else {
		progress = abs32(index - pos)
	}
	if progress == 1 {
		return r
	}
	decel := func(t float32) float32 { return float32(math.Sin(float64(t) * math.Pi / 2)) }
	accel := func(t float32) float32 { return 1 - float32(math.Cos(float64(t)*math.Pi/2)) }
	right := pos > index
	if changing {
		right = index > pos
	}
	lf, rf := decel(progress), accel(progress)
	if right {
		lf, rf = accel(progress), decel(progress)
	}
	var l, rr float32
	if changing || right {
		l, rr = lerp(fromR.X, toR.X, lf), lerp(fromR.X+fromR.W, toR.X+toR.W, rf)
	} else {
		l, rr = lerp(toR.X, fromR.X, lf), lerp(toR.X+toR.W, fromR.X+fromR.W, rf)
	}
	return ui.Rect{X: l, W: rr - l}
}

func pillInk(b ui.Element, on ui.Color) {
	focus, press, splash := ui.RGBA(0, 0, 0, .12), ui.RGBA(0xBC, 0xBC, 0xBC, 0x66/255.0), ui.RGBA(0xC8, 0xC8, 0xC8, 0x66/255.0)
	if design.T().Dark {
		focus, press = ui.RGBA(255, 255, 255, .12), ui.RGBA(0xCC, 0xCC, 0xCC, 0x40/255.0)
		splash = press
	}
	InkColors(b, design.CornerFull, design.WithOpacity(on, .06), focus, press, splash)
}

// tabView is TabBarView's state: the page shown, and whether a drag or
// the settling after it moves it.
type tabView struct {
	last int
	page float32
	drag bool
	held ui.Point
}

// The ballistic spring of PageScrollPhysics: ScrollPhysics' default,
// mass 0.5, stiffness 100 and a damping ratio of 1.1.
var tabSpring = ui.SpringOptions{Mass: .5, Stiffness: 100, Damping: 1.1 * 2 * 7.0710678, Precision: .0005}

// TabView is TabBarView: the n pages of the tabs side by side in e, which
// it clips, sliding to the selected one over kTabScrollDuration along
// Curves.ease as TabBar's indicator moves. A change of more than one tab
// starts from the page next to the new one, as TabBarView does, so only
// two pages show on the way. A finger or a pen drags the pages, as
// PageView with PageScrollPhysics and ClampingScrollPhysics: let go, they
// spring to the nearest page, or the next one the way a fling goes, and
// the page they rest on becomes the selected index. page builds page i.
func TabView(c *ui.Context, e ui.Element, selected *int, n int, page func(i int)) ui.Element {
	st := ui.Local(e, "tabview", func() tabView { return tabView{last: *selected, page: float32(*selected)} })
	sync := syncFor(selected)
	w := e.LayoutBounds().W
	last := float32(max(n-1, 0))
	// Held in place, the view takes a finger's or a pen's horizontal pan
	// from what scrolls vertically inside it; a mouse drags nothing, as
	// MaterialScrollBehavior's dragDevices have it.
	e = e.Draggable(&st.held, ui.DraggableOptions{Axis: ui.Horizontal, Bounds: &ui.Rect{}, Rigid: true})
	kind, _, pressed := e.PressedPointer()
	dx, _, held := e.Dragged()
	vx, _, ended := e.DragEnded()
	dragging := pressed && held && kind != ui.PointerMouse && w > 0 && (st.drag || e.Dragging())
	var off float32
	switch {
	case dragging:
		st.drag = true
		off = e.Jump("page", min(max(st.page-dx/w, 0), last))
		// Past a page from the index, the index moves to the nearest page,
		// as the controller's does under a drag.
		if abs32(off-float32(*selected)) > 1 {
			*selected = int(off + .5)
		}
	case st.drag && ended && w > 0:
		// Let go: the fling's velocity, in DIPs a second the way the pages
		// go, is none below kMinFlingVelocity and at most kMaxFlingVelocity;
		// the pages spring to the page they are on, or half a page further
		// the way a fling goes, rounded.
		v := -vx
		if abs32(v) < 50 {
			v = 0
		}
		v = min(max(v, -8000), 8000)
		target := st.page
		switch {
		case v < 0:
			target -= .5
		case v > 0:
			target += .5
		}
		target = min(max(float32(math.Round(float64(target))), 0), last)
		sp := tabSpring
		sp.Velocity = v / w
		*selected = int(target)
		off = e.AnimateTo("page", target, ui.Animation{Spring: sp})
	default:
		anim := design.T().Motion(tabScroll, design.Ease)
		if *selected != st.last {
			// A tap on a tab, or the app, chose a page.
			st.drag = false
			if d := *selected - st.last; d > 1 || d < -1 {
				from := *selected - 1
				if d < 0 {
					from = *selected + 1
				}
				e.Jump("page", float32(from))
			}
		} else if st.drag {
			anim = ui.Animation{Spring: tabSpring}
		}
		off = e.AnimateTo("page", float32(*selected), anim)
		if st.drag && e.Settled("page") {
			st.drag = false
		}
	}
	st.page = off
	st.last = *selected
	sync.page, sync.drag, sync.frame = off, st.drag, c.Now()
	return e.Clip().Children(func() {
		for i := int(off) - 1; i <= int(off)+1; i++ {
			pos := float32(i) - off
			if i < 0 || pos <= -1 || pos >= 1 {
				continue
			}
			ui.Column(c.Key(i)).Absolute().Top(0).Bottom(0).Left(0).Right(0).TranslatePercent(pos*100, 0).Children(func() { page(i) })
		}
	})
}
