package m3

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
)

// SnackBar is a floating snack bar with the app's theme: 480 wide,
// inverseSurface, Corners.xs, its action in inversePrimary.
type SnackBar struct {
	Message string
	// Content replaces Message.
	Content  func(c *ui.Context)
	Action   string
	OnAction func()
	Duration time.Duration
	// CloseIcon shows the close button (showCloseIcon).
	CloseIcon bool
	shown     time.Time
	id        int
	gone      bool
}

var (
	snacks    []*SnackBar
	snackNext int
)

// snackKey keys a bar apart from the dialogs beside it in the overlay,
// whose keys are route numbers too.
type snackKey int

// ShowSnackBar is ScaffoldMessenger.showSnackBar: it queues the bar behind
// the one showing.
func ShowSnackBar(s SnackBar) {
	if s.Duration == 0 {
		s.Duration = 4 * time.Second
	}
	s.id = snackNext
	snackNext++
	snacks = append(snacks, &s)
}

// Snack shows a message.
func Snack(msg string) { ShowSnackBar(SnackBar{Message: msg}) }

// SnackNow hides the current bar and shows msg at once
// (hideCurrentSnackBar, then showSnackBar).
func SnackNow(msg string) {
	HideSnackBar()
	Snack(msg)
}

// SnackAction shows a message with an action.
func SnackAction(msg, label string, fn func()) {
	ShowSnackBar(SnackBar{Message: msg, Action: label, OnAction: fn})
}

// HideSnackBar is hideCurrentSnackBar.
func HideSnackBar() {
	if len(snacks) > 0 {
		snacks = snacks[1:]
	}
}

// ClearSnackBars is clearSnackBars.
func ClearSnackBars() { snacks = nil }

func buildSnackBars(c *ui.Context) {
	if len(snacks) == 0 {
		return
	}
	s := snacks[0]
	now := time.Now()
	if s.shown.IsZero() {
		s.shown = now
	}
	if now.Sub(s.shown) >= s.Duration {
		snacks = snacks[1:]
		c.Invalidate()
		return
	}
	c.After(s.Duration - now.Sub(s.shown))
	sch := cs()
	w, _ := c.Size()
	width := min(float32(480), w-16)
	ui.Overlay(c, func() {
		bar := ui.Row(c.Key(snackKey(s.id))).Absolute().Bottom(10).Left((w-width)/2).Width(width).
			MinHeight(48).AlignItems(ui.Center).Padding(0, 8, 0, 16).Gap(8).
			Radius(design.CornerXS).Background(sch.InverseSurface)
		bar = Elevation(bar, 6).Transition(ui.ElementTransition{Duration: 250 * time.Millisecond,
			Ease: design.FastOutSlowIn, Enter: &ui.Motion{Opacity: 0}, Exit: &ui.Motion{Opacity: 0}})
		bar.Children(func() {
			ui.Column(c).Grow(1).PaddingY(14).Children(func() {
				if s.Content != nil {
					s.Content(c)
				} else {
					Text(c, s.Message, tt().BodyMedium.C(sch.OnInverseSurface))
				}
			})
			if s.Action != "" {
				if (Button{Kind: TextKind, Label: s.Action, FG: sch.InversePrimary}).Show(c).Clicked() {
					if s.OnAction != nil {
						s.OnAction()
					}
					HideSnackBar()
				}
			}
			if s.CloseIcon {
				if (IconButton{Icon: icons.CloseRounded, Color: sch.OnInverseSurface}).Show(c).Clicked() {
					HideSnackBar()
				}
			}
		})
	})
}
