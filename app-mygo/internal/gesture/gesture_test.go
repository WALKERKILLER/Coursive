package gesture

import (
	"math"
	"testing"
)

// The tests below replay pointer events, which is the layer the checklist asks
// for: the recognition rules are plain Go, so gesture switches, cancels and the
// undo boundary are checked without a device. A device adds the question of
// whether the host delivers the fields at all, which is P03's other half.

func videoAndPDF() *Canvas {
	return NewCanvas(
		Card{ID: "video", Kind: "video", X: 100, Y: 100, W: 320, H: 180},
		Card{ID: "pdf", Kind: "pdf", X: 500, Y: 100, W: 320, H: 400},
	)
}

func touch(id uint64, phase Phase, x, y float32) Pointer {
	return Pointer{ID: id, Kind: KindTouch, Phase: phase, X: x, Y: y}
}

func pen(id uint64, phase Phase, x, y, pressure float32) Pointer {
	return Pointer{ID: id, Kind: KindStylus, Phase: phase, X: x, Y: y,
		Pressure: pressure, HasPressure: true}
}

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 0.01 }

// TestOneFingerPansTheCanvas is the base rule: a finger on the page moves the
// camera and nothing else.
func TestOneFingerPansTheCanvas(t *testing.T) {
	c := videoAndPDF()
	c.Apply(touch(1, PhaseDown, 900, 600))
	if c.Gesture() != GesturePan {
		t.Fatalf("gesture %v, want pan", c.Gesture())
	}
	c.Apply(touch(1, PhaseMove, 950, 640))
	if !near(c.Camera.X, 50) || !near(c.Camera.Y, 40) {
		t.Fatalf("camera %+v", c.Camera)
	}
	if len(c.Cards) != 2 || !near(c.Cards[0].X, 100) {
		t.Fatalf("panning moved a card: %+v", c.Cards)
	}
}

// TestHoldTurnsPressIntoCardDrag is the 600 ms rule: pressing a card without
// moving holds it, and then the card follows the finger while the camera stays.
func TestHoldTurnsPressIntoCardDrag(t *testing.T) {
	c := videoAndPDF()
	c.Apply(touch(1, PhaseDown, 200, 150))
	if c.Gesture() != GestureIdle || c.Holding() {
		t.Fatalf("a fresh press already decided: %v holding=%v", c.Gesture(), c.Holding())
	}
	c.Advance(CardHoldMillis)
	if !c.Tick() || !c.Holding() {
		t.Fatal("600 ms did not hold the card")
	}
	c.Apply(touch(1, PhaseMove, 240, 170))
	if c.Dragging() != "video" {
		t.Fatalf("dragging %q", c.Dragging())
	}
	if !near(c.Cards[0].X, 140) || !near(c.Cards[0].Y, 120) {
		t.Fatalf("card %+v", c.Cards[0])
	}
	if !near(c.Camera.X, 0) || !near(c.Camera.Y, 0) {
		t.Fatalf("dragging a card moved the camera: %+v", c.Camera)
	}
}

// TestMovingBeforeTheHoldDrags is the other side of the 600 ms rule: a press
// that moves becomes a card drag at once, and never pans.
func TestMovingBeforeTheHoldDrags(t *testing.T) {
	c := videoAndPDF()
	c.Apply(touch(1, PhaseDown, 200, 150))
	c.Advance(100)
	c.Apply(touch(1, PhaseMove, 230, 150))
	if c.Dragging() != "video" {
		t.Fatalf("dragging %q", c.Dragging())
	}
	if !near(c.Camera.X, 0) {
		t.Fatalf("camera %+v", c.Camera)
	}
}

// TestSecondFingerTakesOver is the cross-card case the checklist names: two
// fingers on a video card and a PDF card still belong to one canvas. The second
// finger cancels the pending hold and starts a pinch.
func TestSecondFingerTakesOver(t *testing.T) {
	c := videoAndPDF()
	c.Apply(touch(1, PhaseDown, 200, 150)) // on the video card
	c.Advance(200)
	c.Apply(touch(2, PhaseDown, 600, 300)) // on the PDF card
	if c.Gesture() != GesturePinch {
		t.Fatalf("gesture %v, want pinch", c.Gesture())
	}
	if c.Holding() {
		t.Fatal("the hold survived the second finger")
	}
	c.Advance(CardHoldMillis)
	if c.Tick() {
		t.Fatal("a hold started while pinching")
	}
	if !near(c.Cards[0].X, 100) || !near(c.Cards[1].X, 500) {
		t.Fatalf("pinching moved a card: %+v", c.Cards)
	}
}

// TestPinchKeepsTheAnchor is decision 0018's rule: the world point under the
// fingers' midpoint stays under it while the canvas zooms.
func TestPinchKeepsTheAnchor(t *testing.T) {
	c := videoAndPDF()
	c.Apply(touch(1, PhaseDown, 400, 300))
	c.Apply(touch(2, PhaseDown, 600, 300))

	// Each move zooms about the midpoint the fingers have after it, keeping
	// whatever world point was under that midpoint under it.
	step := func(id uint64, x, x1, x2 float32) {
		before := c.Camera
		mid := Point{X: (x1 + x2) / 2, Y: 300}
		world := Point{
			X: (mid.X - before.X) / before.Zoom,
			Y: (mid.Y - before.Y) / before.Zoom,
		}
		c.Apply(touch(id, PhaseMove, x, 300))
		got := c.screenToWorld(mid)
		if !near(got.X, world.X) || !near(got.Y, world.Y) {
			t.Fatalf("anchor moved from %+v to %+v (zoom %v)", world, got, c.Camera.Zoom)
		}
	}
	step(1, 300, 300, 600)
	step(2, 700, 300, 700)

	if !near(c.Camera.Zoom, 2) {
		t.Fatalf("zoom %v, want 2 after doubling the spread", c.Camera.Zoom)
	}
}

// TestLiftingOneFingerDoesNotJump: the camera must not snap when a pinch loses
// a finger; the next move starts a fresh baseline.
func TestLiftingOneFingerDoesNotJump(t *testing.T) {
	c := videoAndPDF()
	c.Apply(touch(1, PhaseDown, 400, 300))
	c.Apply(touch(2, PhaseDown, 600, 300))
	c.Apply(touch(1, PhaseMove, 300, 300))
	c.Apply(touch(2, PhaseMove, 700, 300))
	cam := c.Camera
	c.Apply(touch(2, PhaseUp, 700, 300))
	if c.Gesture() != GestureIdle {
		t.Fatalf("gesture %v after lifting one of two", c.Gesture())
	}
	if c.Camera != cam {
		t.Fatalf("camera jumped from %+v to %+v", cam, c.Camera)
	}
}

// TestPenInterruptsFingers: the pen takes the canvas from the fingers, and the
// palm that touched first cannot resume panning when the pen lifts.
func TestPenInterruptsFingers(t *testing.T) {
	c := videoAndPDF()
	c.Apply(touch(1, PhaseDown, 900, 600))
	if c.Gesture() != GesturePan {
		t.Fatalf("gesture %v", c.Gesture())
	}
	c.Apply(pen(9, PhaseDown, 200, 150, 0.6))
	if c.Gesture() != GestureInk {
		t.Fatalf("gesture %v, want ink", c.Gesture())
	}
	c.Apply(pen(9, PhaseMove, 210, 155, 0.7))
	c.Apply(pen(9, PhaseUp, 220, 160, 0.5))
	if len(c.Strokes) != 1 {
		t.Fatalf("strokes %d", len(c.Strokes))
	}
	if c.Strokes[0].Card != "video" {
		t.Fatalf("stroke attached to %q, want the video card", c.Strokes[0].Card)
	}

	// The palm, still down, must not move the camera.
	cam := c.Camera
	c.Apply(touch(1, PhaseMove, 950, 640))
	if c.Camera != cam {
		t.Fatalf("the palm panned during the cooldown: %+v", c.Camera)
	}
	if c.Suppressed == 0 {
		t.Fatal("the palm move was not recorded as suppressed")
	}
}

// TestPalmCooldownEnds: a fresh down after the cooldown works again.
func TestPalmCooldownEnds(t *testing.T) {
	c := videoAndPDF()
	c.Apply(pen(9, PhaseDown, 200, 150, 0.6))
	c.Apply(pen(9, PhaseUp, 200, 150, 0.6))
	c.Advance(PalmCooldown + 1)
	c.Apply(touch(1, PhaseDown, 900, 600))
	if c.Gesture() != GesturePan {
		t.Fatalf("gesture %v after the cooldown", c.Gesture())
	}
}

// TestCancelLeavesNoGhostInk is the hard acceptance rule: a cancelled stroke
// leaves nothing behind and is counted, not silently dropped.
func TestCancelLeavesNoGhostInk(t *testing.T) {
	c := videoAndPDF()
	c.Apply(pen(9, PhaseDown, 200, 150, 0.6))
	c.Apply(pen(9, PhaseMove, 240, 170, 0.7))
	c.Apply(Pointer{ID: 9, Kind: KindStylus, Phase: PhaseCancel, X: 240, Y: 170})
	if len(c.Strokes) != 0 {
		t.Fatalf("a cancel left %d strokes", len(c.Strokes))
	}
	if c.Cancelled != 1 {
		t.Fatalf("cancelled %d", c.Cancelled)
	}
	if c.Gesture() != GestureIdle {
		t.Fatalf("gesture %v after a cancel", c.Gesture())
	}
}

// TestCancelRestoresADraggedCard: a system cancel puts the card back, so a
// cancelled drag is not a half-move the user cannot undo.
func TestCancelRestoresADraggedCard(t *testing.T) {
	c := videoAndPDF()
	c.Apply(touch(1, PhaseDown, 200, 150))
	c.Apply(touch(1, PhaseMove, 260, 150))
	if !near(c.Cards[0].X, 160) {
		t.Fatalf("card %+v", c.Cards[0])
	}
	c.Apply(Pointer{ID: 1, Kind: KindTouch, Phase: PhaseCancel, X: 260, Y: 150})
	if !near(c.Cards[0].X, 100) {
		t.Fatalf("a cancel left the card at %v", c.Cards[0].X)
	}
}

// TestPressureEraserNeedsRealPressure is the MyGo gap in behavioural form.
// A device that reports no pressure must not erase: MyGo's ui.Touch fills 0.5
// for every touch, so a client that trusts the number would erase on hardware
// that cannot press.
func TestPressureEraserNeedsRealPressure(t *testing.T) {
	c := videoAndPDF()
	c.Apply(pen(9, PhaseDown, 200, 150, 0.6))
	c.Apply(pen(9, PhaseMove, 260, 150, 0.6))
	c.Apply(pen(9, PhaseUp, 260, 150, 0.6))
	if len(c.Strokes) != 1 {
		t.Fatalf("setup stroke missing: %d", len(c.Strokes))
	}

	noSensor := Pointer{ID: 10, Kind: KindStylus, Phase: PhaseDown, X: 220, Y: 150,
		Pressure: 0.5, HasPressure: false}
	if c.EraseAt(noSensor) {
		t.Fatal("erased with a pressure reading the device did not report")
	}
	if len(c.Strokes) != 1 {
		t.Fatalf("strokes %d after a refused erase", len(c.Strokes))
	}

	firm := Pointer{ID: 10, Kind: KindStylus, Phase: PhaseDown, X: 220, Y: 150,
		Pressure: 0.8, HasPressure: true}
	if !c.EraseAt(firm) {
		t.Fatal("a firm press did not erase")
	}
	if len(c.Strokes) != 0 {
		t.Fatalf("strokes %d after erasing", len(c.Strokes))
	}
}

// TestUnknownDeviceFallsBackToTouch is what a pen becomes when the host loses
// its kind, as MyGo v0.10.0 does in internal/mobile/surface.go: the canvas keeps
// working, but the pen draws by the finger rules.
func TestUnknownDeviceFallsBackToTouch(t *testing.T) {
	c := videoAndPDF()
	unknown := Pointer{ID: 9, Kind: KindUnknown, Phase: PhaseDown, X: 900, Y: 600}
	c.Apply(unknown)
	if c.Gesture() != GesturePan {
		t.Fatalf("gesture %v, want the finger fallback", c.Gesture())
	}
	if len(c.Strokes) != 0 {
		t.Fatal("an unknown device drew ink")
	}
}

// TestThirdFingerKeepsTheCanvas: a third finger changes the pinch baseline and
// nothing else, with no coordinate jump.
func TestThirdFingerKeepsTheCanvas(t *testing.T) {
	c := videoAndPDF()
	c.Apply(touch(1, PhaseDown, 400, 300))
	c.Apply(touch(2, PhaseDown, 600, 300))
	mid := c.center()
	before := c.screenToWorld(mid)
	c.Apply(touch(3, PhaseDown, 500, 500))
	if c.Gesture() != GesturePinch {
		t.Fatalf("gesture %v with three fingers", c.Gesture())
	}
	after := c.screenToWorld(c.center())
	if !near(before.X, after.X) || !near(before.Y, after.Y) {
		t.Fatalf("a third finger jumped the world from %+v to %+v", before, after)
	}
}
