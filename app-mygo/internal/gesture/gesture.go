package gesture

import "math"

// This file is the canvas's input brain, in plain Go with no UI types, so the
// rules can be replayed from recorded events in a test. The view feeds it and
// draws what it decides; the MyGo bridge question (which fields the host
// actually delivers) is answered by what a Pointer carries, not by the drawing.

// Kind is the device a pointer event came from, as the canvas cares about it.
// It mirrors Flutter's PointerDeviceKind, which the Flutter client branches on
// (app/lib/features/study_canvas/study_stylus.dart).
type Kind int

const (
	KindTouch Kind = iota
	KindStylus
	KindEraser
	// KindUnknown is a pointer whose device the host did not say. The Flutter
	// client never sees it; MyGo v0.10.0 delivers pen input as a mouse, so a
	// bridge that does not restore the kind lands here.
	KindUnknown
)

// Phase is what a pointer did since the last event.
type Phase int

const (
	PhaseDown Phase = iota
	PhaseMove
	PhaseUp
	PhaseCancel
	PhaseHover
)

// Pointer is one pointer event. Pressure is only meaningful when HasPressure:
// a device without a pressure sensor reports none, and MyGo's ui.Touch fills
// 0.5 instead of saying so, which would make a pressure eraser fire on
// hardware that cannot press.
type Pointer struct {
	ID          uint64
	Kind        Kind
	Phase       Phase
	X, Y        float32
	Pressure    float32
	HasPressure bool
}

// Card is a card on the canvas. Video and PDF cards behave the same to the
// gesture rules; only their drawing differs.
type Card struct {
	ID   string
	Kind string // "video" or "pdf"
	X, Y float32
	W, H float32
}

// Stroke is ink: one pen-down to pen-up.
type Stroke struct {
	ID    uint64
	Card  string // the card it was drawn on, or "" for the canvas
	Ink   []Point
	Width float32
}

// Point is one sample of a stroke.
type Point struct{ X, Y, P float32 }

// Gesture is what the canvas is doing with the fingers and the pen.
type Gesture int

const (
	GestureIdle Gesture = iota
	GesturePan
	GesturePinch
	GestureCardDrag
	GestureInk
)

// Rules are the timings the Flutter client uses
// (app/lib/features/study_canvas/study_canvas.dart, study_stylus.dart).
const (
	CardHoldMillis  = 600
	PalmCooldown    = 1000
	EraserThreshold = 0.45
)

// Canvas is the whole interaction state: the camera, the cards, the ink, and
// which fingers are doing what.
type Canvas struct {
	Camera  struct{ X, Y, Zoom float32 }
	Cards   []Card
	Strokes []Stroke

	gesture Gesture

	// touches are the fingers currently down, in the order they landed.
	touches []uint64
	// last is where each pointer was, by id.
	last map[uint64]Point

	// hold is the pending long-press on a card; holdFor is its card and
	// holdSince when the finger that may hold landed.
	holdFor   string
	holdStart Point
	holdSince float32
	holding   bool

	// drag is the card being dragged, and where it started.
	dragCard  string
	dragStart Point
	dragFrom  struct{ X, Y float32 }

	// pinch baseline, reset whenever the finger count changes.
	pinchDist  float32
	pinchWorld Point

	// ink is the stroke in progress, drawn by the stylus.
	ink *Stroke
	// palmUntil blocks touch input for a while after the pen lifts, so a palm
	// resting on the screen does not start panning.
	palmUntil float32
	now       float32

	// Suppressed counts touch events the palm rule swallowed, for tests and
	// for the on-screen debug readout.
	Suppressed int
	// Cancelled counts strokes a cancel event discarded.
	Cancelled int
}

// NewCanvas makes a canvas with the camera at the origin.
func NewCanvas(cards ...Card) *Canvas {
	c := &Canvas{Cards: cards, last: map[uint64]Point{}}
	c.Camera.Zoom = 1
	return c
}

// Advance moves the clock the hold and palm timers read, in milliseconds.
func (c *Canvas) Advance(ms float32) { c.now += ms }

// Now is the canvas's clock, in milliseconds since it was made.
func (c *Canvas) Now() float32 { return c.now }

// Gesture reports what the canvas is doing.
func (c *Canvas) Gesture() Gesture { return c.gesture }

// Holding reports a long press on a card that has not moved yet.
func (c *Canvas) Holding() bool { return c.holding }

// Dragging reports the card being dragged.
func (c *Canvas) Dragging() string { return c.dragCard }

// PenActive reports the stylus is down or inside its palm cooldown.
func (c *Canvas) PenActive() bool { return c.ink != nil || c.now < c.palmUntil }

// Apply feeds one pointer event and returns true when the canvas changed, so
// the view knows to ask for a frame.
func (c *Canvas) Apply(p Pointer) bool {
	switch p.Kind {
	case KindStylus, KindEraser:
		return c.applyPen(p)
	case KindUnknown:
		// The host did not say what this is. Treat it as a finger: the canvas
		// stays usable, but a pen that arrives here draws with the finger
		// rules, which is exactly the MyGo gap P03 has to close.
		return c.applyTouch(p)
	default:
		return c.applyTouch(p)
	}
}

func (c *Canvas) applyPen(p Pointer) bool {
	switch p.Phase {
	case PhaseDown:
		// A pen interrupts whatever the fingers were doing, as the Flutter
		// client's _trackStylusPresence does: the palm that touched first
		// cannot resume panning when the pen lifts.
		c.dropTouches()
		c.gesture = GestureInk
		card := c.cardAt(p.X, p.Y)
		c.ink = &Stroke{ID: p.ID, Card: card, Width: 2}
		c.ink.Ink = append(c.ink.Ink, Point{p.X, p.Y, p.Pressure})
		return true
	case PhaseMove:
		if c.ink == nil || c.ink.ID != p.ID {
			return false
		}
		c.ink.Ink = append(c.ink.Ink, Point{p.X, p.Y, p.Pressure})
		return true
	case PhaseUp:
		if c.ink == nil || c.ink.ID != p.ID {
			return false
		}
		c.ink.Ink = append(c.ink.Ink, Point{p.X, p.Y, p.Pressure})
		c.Strokes = append(c.Strokes, *c.ink)
		c.ink = nil
		c.gesture = GestureIdle
		c.palmUntil = c.now + PalmCooldown
		return true
	case PhaseCancel:
		// The system took the pen: the stroke goes away whole. A partial
		// stroke left behind is the ghost ink the acceptance rules forbid.
		if c.ink == nil || c.ink.ID != p.ID {
			return false
		}
		c.ink = nil
		c.Cancelled++
		c.gesture = GestureIdle
		c.palmUntil = c.now + PalmCooldown
		return true
	case PhaseHover:
		// Hover protects handwriting without cancelling a deliberate gesture.
		if c.ink == nil {
			c.palmUntil = c.now + PalmCooldown
		}
		return false
	}
	return false
}

func (c *Canvas) applyTouch(p Pointer) bool {
	// The palm rule: while the pen is down, or within the cooldown after it
	// lifted, a finger does nothing at all.
	if c.PenActive() {
		// Every touch the palm rule swallows counts, not only a landing one:
		// the readout tells a user whether their palm is being ignored.
		c.Suppressed++
		return false
	}

	switch p.Phase {
	case PhaseDown:
		c.touches = append(c.touches, p.ID)
		c.last[p.ID] = Point{p.X, p.Y, p.Pressure}
		switch len(c.touches) {
		case 1:
			// A finger on a card may become a card drag; a finger on the page
			// pans. Neither is decided until the finger moves or the hold
			// timer expires, which is the 600 ms rule.
			if card := c.cardAt(p.X, p.Y); card != "" {
				c.holdFor, c.holdStart, c.holding = card, Point{p.X, p.Y, 0}, false
				c.gesture = GestureIdle
			} else {
				c.gesture = GesturePan
			}
		case 2:
			// A second finger takes over: the pending card hold is cancelled
			// and any uncommitted drag preview is dropped.
			c.cancelHold()
			if c.dragCard != "" {
				if i := c.cardIndex(c.dragCard); i >= 0 {
					c.Cards[i].X, c.Cards[i].Y = c.dragFrom.X, c.dragFrom.Y
				}
				c.dragCard = ""
			}
			c.gesture = GesturePinch
			c.pinchDist = c.spread()
			c.pinchWorld = c.screenToWorld(c.center())
		default:
			// A third finger changes nothing but the baseline.
			c.pinchDist = c.spread()
			c.pinchWorld = c.screenToWorld(c.center())
		}
		return true

	case PhaseMove:
		prev, ok := c.last[p.ID]
		if !ok {
			return false
		}
		c.last[p.ID] = Point{p.X, p.Y, p.Pressure}
		switch c.gesture {
		case GesturePinch:
			if len(c.touches) < 2 {
				return false
			}
			mid := c.center()
			dist := c.spread()
			if c.pinchDist > 0 && dist > 0 {
				// Zoom about the fingers' midpoint so the world point under it
				// stays under it: the anchor rule decision 0018 keeps.
				anchor := c.screenToWorld(mid)
				c.Camera.Zoom = clamp(c.Camera.Zoom*dist/c.pinchDist, 0.25, 4)
				c.Camera.X = mid.X - anchor.X*c.Camera.Zoom
				c.Camera.Y = mid.Y - anchor.Y*c.Camera.Zoom
			}
			c.pinchDist = dist
			c.pinchWorld = c.screenToWorld(mid)
			return true
		case GestureCardDrag:
			if c.dragCard == "" {
				return false
			}
			if i := c.cardIndex(c.dragCard); i >= 0 {
				c.Cards[i].X = c.dragFrom.X + (p.X-c.dragStart.X)/c.Camera.Zoom
				c.Cards[i].Y = c.dragFrom.Y + (p.Y-c.dragStart.Y)/c.Camera.Zoom
			}
			return true
		case GesturePan:
			c.Camera.X += (p.X - prev.X)
			c.Camera.Y += (p.Y - prev.Y)
			return true
		default:
			// A pending card hold that moves becomes a drag, never a pan.
			if c.holdFor != "" && dist(p.X, p.Y, c.holdStart.X, c.holdStart.Y) > 4 {
				c.beginCardDrag(c.holdStart)
				c.dragStart = Point{p.X, p.Y, 0}
				if i := c.cardIndex(c.dragCard); i >= 0 {
					c.Cards[i].X = c.dragFrom.X + (p.X-c.holdStart.X)/c.Camera.Zoom
					c.Cards[i].Y = c.dragFrom.Y + (p.Y-c.holdStart.Y)/c.Camera.Zoom
				}
				return true
			}
			return false
		}

	case PhaseUp:
		delete(c.last, p.ID)
		c.touches = removeID(c.touches, p.ID)
		switch {
		case c.dragCard != "":
			c.dragCard = ""
			c.gesture = GestureIdle
		case c.holding:
			c.holding = false
			c.gesture = GestureIdle
		}
		if c.holdFor != "" && len(c.touches) == 0 {
			c.cancelHold()
		}
		if len(c.touches) < 2 && c.gesture == GesturePinch {
			// Lifting one of two fingers ends the pinch rather than jumping
			// the camera: the next move starts a fresh baseline.
			c.gesture = GestureIdle
			c.pinchDist = 0
		}
		if len(c.touches) == 0 {
			c.gesture = GestureIdle
		}
		return true

	case PhaseCancel:
		delete(c.last, p.ID)
		c.touches = removeID(c.touches, p.ID)
		c.cancelHold()
		if c.dragCard != "" {
			if i := c.cardIndex(c.dragCard); i >= 0 {
				c.Cards[i].X, c.Cards[i].Y = c.dragFrom.X, c.dragFrom.Y
			}
			c.dragCard = ""
		}
		c.pinchDist = 0
		if len(c.touches) == 0 {
			c.gesture = GestureIdle
		}
		return true
	}
	return false
}

// Tick expires the card hold: 600 ms without moving turns a press on a card
// into a whole-card drag.
func (c *Canvas) Tick() bool {
	if c.holding || c.holdFor == "" || c.PenActive() {
		return false
	}
	if len(c.touches) != 1 {
		return false
	}
	if c.now-c.holdStartTime() < CardHoldMillis {
		return false
	}
	c.beginCardDrag(c.holdStart)
	c.holding = true
	return true
}

// holdStartTime is when the finger that may hold landed. The canvas keeps it
// beside the pointer so a hold survives a slow frame.
func (c *Canvas) holdStartTime() float32 { return c.holdSince }

func (c *Canvas) beginCardDrag(from Point) {
	c.dragCard = c.holdFor
	c.dragStart = from
	if i := c.cardIndex(c.dragCard); i >= 0 {
		c.dragFrom.X, c.dragFrom.Y = c.Cards[i].X, c.Cards[i].Y
	}
	c.holdFor = ""
	c.holding = false
	c.gesture = GestureCardDrag
}

func (c *Canvas) cancelHold() {
	c.holdFor = ""
	c.holding = false
}

// EraseAt removes ink near a point, whole strokes only, when the device can
// press: without a pressure reading the eraser would fire on hardware that
// cannot tell a light touch from a firm one.
func (c *Canvas) EraseAt(p Pointer) bool {
	if !p.HasPressure || p.Pressure < EraserThreshold {
		return false
	}
	keep := c.Strokes[:0]
	removed := false
	for _, s := range c.Strokes {
		near := false
		for i, pt := range s.Ink {
			// Samples are sparse, so a stroke is erased where the line between
			// two of them passes, not only at the samples themselves.
			if i > 0 && segmentNear(s.Ink[i-1], pt, Point{X: p.X, Y: p.Y}, 12) {
				near = true
				break
			}
			if dist(pt.X, pt.Y, p.X, p.Y) < 12 {
				near = true
				break
			}
		}
		if near {
			removed = true
			continue
		}
		keep = append(keep, s)
	}
	c.Strokes = keep
	return removed
}

func (c *Canvas) dropTouches() {
	for _, id := range c.touches {
		delete(c.last, id)
	}
	c.touches = nil
	c.cancelHold()
	if c.dragCard != "" {
		if i := c.cardIndex(c.dragCard); i >= 0 {
			c.Cards[i].X, c.Cards[i].Y = c.dragFrom.X, c.dragFrom.Y
		}
		c.dragCard = ""
	}
	c.pinchDist = 0
}

func (c *Canvas) cardAt(x, y float32) string {
	w := c.screenToWorld(Point{X: x, Y: y})
	wx, wy := w.X, w.Y
	for i := len(c.Cards) - 1; i >= 0; i-- {
		card := c.Cards[i]
		if wx >= card.X && wx <= card.X+card.W && wy >= card.Y && wy <= card.Y+card.H {
			return card.ID
		}
	}
	return ""
}

func (c *Canvas) cardIndex(id string) int {
	for i := range c.Cards {
		if c.Cards[i].ID == id {
			return i
		}
	}
	return -1
}

func (c *Canvas) screenToWorld(p Point) Point {
	return Point{
		X: (p.X - c.Camera.X) / c.Camera.Zoom,
		Y: (p.Y - c.Camera.Y) / c.Camera.Zoom,
	}
}

// center is the midpoint of the first two fingers, so a third finger lands
// without moving the anchor the pinch zooms about.
func (c *Canvas) center() Point {
	if len(c.touches) < 2 {
		return Point{}
	}
	a, okA := c.last[c.touches[0]]
	b, okB := c.last[c.touches[1]]
	if !okA || !okB {
		return Point{}
	}
	return Point{X: (a.X + b.X) / 2, Y: (a.Y + b.Y) / 2}
}

func (c *Canvas) spread() float32 {
	if len(c.touches) < 2 {
		return 0
	}
	a, okA := c.last[c.touches[0]]
	b, okB := c.last[c.touches[1]]
	if !okA || !okB {
		return 0
	}
	return dist(a.X, a.Y, b.X, b.Y)
}

func removeID(ids []uint64, id uint64) []uint64 {
	out := ids[:0]
	for _, v := range ids {
		if v != id {
			out = append(out, v)
		}
	}
	return out
}

func dist(ax, ay, bx, by float32) float32 {
	dx, dy := ax-bx, ay-by
	return float32(math.Hypot(float64(dx), float64(dy)))
}

func clamp(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// segmentNear reports whether p is within r of the segment a-b.
func segmentNear(a, b, p Point, r float32) bool {
	dx, dy := b.X-a.X, b.Y-a.Y
	len2 := dx*dx + dy*dy
	if len2 == 0 {
		return dist(a.X, a.Y, p.X, p.Y) <= r
	}
	t := clamp(((p.X-a.X)*dx+(p.Y-a.Y)*dy)/len2, 0, 1)
	return dist(a.X+t*dx, a.Y+t*dy, p.X, p.Y) <= r
}
