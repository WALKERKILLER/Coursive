package m3

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
)

// Route is an overlay route pushed above the pages: a dialog or a sheet.
type Route struct {
	open bool
	// Build builds the route's content; d.Close pops it.
	Build func(c *ui.Context, r *Route)
	// Dismissible is barrierDismissible: a click outside or Escape closes it.
	Dismissible bool
	// OnClose runs once the route is popped, with the value Pop was given.
	OnClose func(result any)
	// OnDismiss replaces the close a dismissible route's Escape or scrim
	// click makes, as PopScope(canPop: false) does.
	OnDismiss func()
	kind      routeKind
	result    any
	id        int
	closed    bool
}

type routeKind int

const (
	dialogRoute routeKind = iota
	sheetRoute
	fullscreenRoute
	generalRoute
)

var (
	routes []*Route
	nextID int
)

// Pop closes the route with a result, Navigator.pop(context, result).
func (r *Route) Pop(result any) {
	if r.closed {
		return
	}
	r.closed, r.open, r.result = true, false, result
	for i, x := range routes {
		if x == r {
			routes = append(routes[:i], routes[i+1:]...)
			break
		}
	}
	if r.OnClose != nil {
		r.OnClose(result)
	}
}

// dismiss is a click on the scrim or Escape.
func (r *Route) dismiss() {
	if r.OnDismiss != nil {
		r.OnDismiss()
		return
	}
	r.Close()
}

// Close is Pop(nil).
func (r *Route) Close() { r.Pop(nil) }

// Open reports whether the route still shows.
func (r *Route) Open() bool { return !r.closed }

// ShowDialog is showDialog: build lays out the dialog (usually an
// AlertDialog or Dialog) and the route closes on a click on the scrim.
func ShowDialog(build func(c *ui.Context, r *Route), onClose func(any)) *Route {
	return push(&Route{Build: build, Dismissible: true, OnClose: onClose, kind: dialogRoute})
}

// ShowModalDialog is showDialog(barrierDismissible: false).
func ShowModalDialog(build func(c *ui.Context, r *Route), onClose func(any)) *Route {
	return push(&Route{Build: build, OnClose: onClose, kind: dialogRoute})
}

func push(r *Route) *Route {
	r.open = true
	r.id = nextID
	nextID++
	routes = append(routes, r)
	return r
}

// AnyRoute reports whether a dialog or sheet shows.
func AnyRoute() bool { return len(routes) > 0 }

// CloseAll pops every dialog and sheet.
func CloseAll() {
	for len(routes) > 0 {
		routes[len(routes)-1].Close()
	}
}

// Host builds the dialogs, sheets and snack bars above the window; the
// root view calls it last.
func Host(c *ui.Context) {
	for _, r := range append([]*Route(nil), routes...) {
		r := r
		switch r.kind {
		case sheetRoute:
			buildSheet(c, r)
		case generalRoute:
			buildGeneral(c, r)
		default:
			buildDialog(c, r)
		}
	}
	buildSnackBars(c)
}

func buildDialog(c *ui.Context, r *Route) {
	open := r.open
	ui.DialogBase(c.Key(r.id), &open, func(backdrop, panel ui.Element) {
		// The barrier is Colors.black54; the route fades in over 150 ms.
		backdrop.Background(ui.RGBA(0, 0, 0, 0x8A/255.0)).Padding(24, 40).
			Transition(ui.ElementTransition{Duration: 150 * time.Millisecond, Ease: design.EaseOut,
				Enter: &ui.Motion{Opacity: 0}, Exit: &ui.Motion{Opacity: 0}, Shadows: true})
		panel.Background(ui.Transparent).MaxHeightPercent(100)
		r.Build(c, r)
	})
	if !open && r.open {
		if r.Dismissible {
			r.dismiss()
		}
	}
}

// ShowGeneral is showGeneralDialog: build lays out the whole route over the
// black54 barrier, placing its panel itself; a click on the barrier closes a
// dismissible route.
func ShowGeneral(dismissible bool, build func(c *ui.Context, r *Route), onClose func(any)) *Route {
	return push(&Route{Build: build, Dismissible: dismissible, OnClose: onClose, kind: generalRoute})
}

func buildGeneral(c *ui.Context, r *Route) {
	open := r.open
	ui.DialogBase(c.Key(r.id), &open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0x8A/255.0)).
			Transition(ui.ElementTransition{Duration: design.Medium4, Ease: design.EmphasizedDecelerate,
				Enter: &ui.Motion{Opacity: 0}, Exit: &ui.Motion{Opacity: 0}, Shadows: true})
		panel.Background(ui.Transparent).Fill().Children(func() {
			barrier := ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0)
			if barrier.Clicked() && r.Dismissible {
				r.dismiss()
			}
			r.Build(c, r)
		})
	})
	if !open && r.open && r.Dismissible {
		r.dismiss()
	}
}

// Dialog is Dialog(child:) with the app's theme: surfaceContainerHigh,
// Corners.xl, at least 280 wide.
func Dialog(c *ui.Context, child func()) ui.Element {
	return ui.Column(c).MinWidth(280).Radius(design.CornerXL).Background(cs().SurfaceContainerHigh).
		Clip().Shrink(1).Children(child)
}

// Alert is AlertDialog's parts; leave out what the dialog has none of.
type Alert struct {
	Icon    icons.Icon
	Title   string
	Content func()
	// Message is plain content text, when Content is nil.
	Message string
	Actions func()
	// Width fixes the content's width, as a SizedBox around it does.
	Width float32
	// Scrollable scrolls the title and content together.
	Scrollable bool
}

// AlertDialog is AlertDialog with Material 3's paddings and styles.
func AlertDialog(c *ui.Context, a Alert) ui.Element {
	s := cs()
	hasContent := a.Content != nil || a.Message != ""
	return Dialog(c, func() {
		body := func() {
			if a.Icon != 0 {
				bottom := float32(24)
				if a.Title != "" {
					bottom = 16
				} else if hasContent {
					bottom = 0
				}
				ui.Row(c).Justify(ui.Center).Padding(24, 24, bottom, 24).Children(func() {
					Icon(c, a.Icon, 24, s.Secondary)
				})
			}
			if a.Title != "" {
				top, bottom := float32(24), float32(0)
				if a.Icon != 0 {
					top = 0
				}
				if !hasContent {
					bottom = 20
				}
				t := Text(c, a.Title, tt().HeadlineSmall.C(s.OnSurface)).Margin(top, 24, bottom, 24)
				if a.Icon != 0 {
					t.TextAlign(ui.Center)
				}
			}
			if hasContent {
				top := float32(16)
				if a.Title == "" && a.Icon == 0 {
					top = 24
				}
				var col ui.Element
				if a.Scrollable {
					col = ui.Column(c).Padding(top, 24, 24, 24)
				} else {
					col = ui.Scroll(c).Padding(top, 24, 24, 24).Shrink(1)
				}
				if a.Width > 0 {
					col = col.Width(a.Width + 48)
				}
				col.Children(func() {
					if a.Content != nil {
						a.Content()
					} else {
						Text(c, a.Message, tt().BodyMedium.C(s.OnSurfaceVariant))
					}
				})
			}
		}
		if a.Scrollable {
			ui.Scroll(c).Shrink(1).Children(body)
		} else {
			body()
		}
		if a.Actions != nil {
			ui.Row(c).Justify(ui.End).Wrap().Gap(8).Padding(0, 24, 24, 24).Children(a.Actions)
		}
	})
}

// Confirm shows an AlertDialog asking title and message, with a cancel and
// a confirm button, and calls yes when confirmed. danger colours the
// confirming button in the error colour.
func Confirm(title, message, cancel, ok string, danger bool, yes func()) {
	ShowDialog(func(c *ui.Context, r *Route) {
		AlertDialog(c, Alert{Title: title, Message: message, Actions: func() {
			if TextButton(c, cancel).Clicked() {
				r.Pop(false)
			}
			b := Button{Kind: Filled, Label: ok}
			if danger {
				b.BG, b.FG = cs().Error, cs().OnError
			}
			if b.Show(c).Clicked() {
				r.Pop(true)
			}
		}})
	}, func(v any) {
		if v == true && yes != nil {
			yes()
		}
	})
}
