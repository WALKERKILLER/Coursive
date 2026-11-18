package m3

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
)

// SheetOptions are showModalBottomSheet's.
type SheetOptions struct {
	// ScrollControlled lets the sheet grow to the window's height
	// (isScrollControlled); otherwise it is at most 9/16 of it.
	ScrollControlled bool
	// NotDismissible keeps it open on a click on the scrim.
	NotDismissible bool
	// NoHandle hides the drag handle the theme shows.
	NoHandle bool
	OnClose  func(any)
	// MaxHeight caps the sheet's height when not zero.
	MaxHeight float32
}

var sheetOptions = map[*Route]SheetOptions{}

// ShowModalBottomSheet is showModalBottomSheet with the app's theme:
// surfaceContainerLow, Corners.xl at the top, a drag handle, at most 640
// wide.
func ShowModalBottomSheet(o SheetOptions, build func(c *ui.Context, r *Route)) *Route {
	r := &Route{Build: build, Dismissible: !o.NotDismissible, OnClose: o.OnClose, kind: sheetRoute}
	sheetOptions[r] = o
	inner := r.OnClose
	r.OnClose = func(v any) {
		delete(sheetOptions, r)
		if inner != nil {
			inner(v)
		}
	}
	return push(r)
}

func buildSheet(c *ui.Context, r *Route) {
	o := sheetOptions[r]
	open := r.open
	_, h := c.Size()
	maxH := h * 9 / 16
	if o.ScrollControlled {
		maxH = h
	}
	if o.MaxHeight > 0 {
		maxH = min(maxH, o.MaxHeight)
	}
	ui.DialogBase(c.Key(r.id), &open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0x8A/255.0)).Justify(ui.End).
			Transition(ui.ElementTransition{Duration: 250 * time.Millisecond, Enter: &ui.Motion{Opacity: 0}, Exit: &ui.Motion{Opacity: 0}})
		panel.Width(640).MaxWidthPercent(100).MaxHeight(maxH).Radius(design.CornerXL, design.CornerXL, 0, 0).
			Background(cs().SurfaceContainerLow).Clip().
			Transition(ui.ElementTransition{Duration: 250 * time.Millisecond, ExitDuration: 200 * time.Millisecond,
				Ease: design.EaseOutCubic, Enter: &ui.Motion{YPercent: 100}, Exit: &ui.Motion{YPercent: 100}})
		if !o.NoHandle {
			ui.Row(c).Justify(ui.Center).Height(48).AlignItems(ui.Center).Shrink(0).Children(func() {
				ui.Box(c).Size(32, 4).Radius(2).Background(design.WithOpacity(cs().OnSurfaceVariant, .4))
			})
		}
		r.Build(c, r)
	})
	if !open && r.open && r.Dismissible {
		r.Close()
	}
}

// AdaptiveSheet is showAdaptiveSheet: a dialog at most maxWidth wide on
// wide windows, a scroll-controlled bottom sheet on compact ones.
func AdaptiveSheet(c *ui.Context, maxWidth float32, dismissible bool, build func(c *ui.Context, r *Route), onClose func(any)) *Route {
	if maxWidth == 0 {
		maxWidth = 520
	}
	w, _ := c.Size()
	if design.SizeClassOf(w) != design.SizeCompact {
		r := &Route{Dismissible: dismissible, OnClose: onClose, kind: dialogRoute}
		r.Build = func(c *ui.Context, r *Route) {
			Dialog(c, func() {
				ui.Column(c).MaxWidth(maxWidth).Shrink(1).Children(func() { build(c, r) })
			})
		}
		return push(r)
	}
	return ShowModalBottomSheet(SheetOptions{ScrollControlled: true, NotDismissible: !dismissible, NoHandle: !dismissible, OnClose: onClose}, build)
}

// SheetPadding is sheetPadding: the inset of a sheet's body.
func SheetPadding(c *ui.Context, handle bool) (top, right, bottom, left float32) {
	w, _ := c.Size()
	top = design.SpaceXL
	if design.SizeClassOf(w) == design.SizeCompact && handle {
		top = 0
	}
	return top, design.SpaceXL, design.SpaceXL, design.SpaceXL
}
