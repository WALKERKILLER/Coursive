package m3

import (
	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
)

// Segment is a ButtonSegment.
type Segment[T comparable] struct {
	Value    T
	Label    string
	Icon     icons.Icon
	Tooltip  string
	Disabled bool
	// Badge puts a small badge on the icon (Badge(isLabelVisible:)).
	Badge bool
}

// SegOpts are SegmentedButton's options.
type SegOpts struct {
	// NoCheck is showSelectedIcon: false.
	NoCheck bool
	// Compact is visualDensity: compact (the theme keeps standard).
	Compact bool
	// Fill stretches the segments across the width.
	Fill bool
	// Style replaces labelLarge.
	Style *design.TextStyle
}

// Segmented is SegmentedButton with single selection: it returns whether
// the choice changed.
func Segmented[T comparable](c *ui.Context, segs []Segment[T], selected *T, o SegOpts) bool {
	changed := false
	segmentedRow(c, len(segs), o, func(i int) (bool, Segment[T]) { return segs[i].Value == *selected, segs[i] },
		func(i int) {
			if *selected != segs[i].Value {
				*selected = segs[i].Value
				changed = true
			}
		})
	return changed
}

// SegmentedMulti is SegmentedButton(multiSelectionEnabled: true); empty
// is emptySelectionAllowed.
func SegmentedMulti[T comparable](c *ui.Context, segs []Segment[T], set map[T]bool, empty bool, o SegOpts) bool {
	changed := false
	segmentedRow(c, len(segs), o, func(i int) (bool, Segment[T]) { return set[segs[i].Value], segs[i] },
		func(i int) {
			v := segs[i].Value
			if set[v] {
				n := 0
				for _, on := range set {
					if on {
						n++
					}
				}
				if n <= 1 && !empty {
					return
				}
				delete(set, v)
			} else {
				set[v] = true
			}
			changed = true
		})
	return changed
}

func segmentedRow[T comparable](c *ui.Context, n int, o SegOpts, seg func(int) (bool, Segment[T]), tap func(int)) {
	s := cs()
	h := float32(40) + Density()*4
	if o.Compact {
		h = 32
	}
	row := ui.Row(c).Height(h).Radius(design.CornerFull).Border(1, s.Outline).Clip().Shrink(0)
	if o.Fill {
		row = row.FillWidth()
	}
	style := tt().LabelLarge
	if o.Style != nil {
		style = *o.Style
	}
	row.Children(func() {
		for i := 0; i < n; i++ {
			on, sg := seg(i)
			if i > 0 {
				ui.Box(c).Width(1).FillHeight().Background(s.Outline).Shrink(0)
			}
			bg, fg := ui.Transparent, s.OnSurface
			if on {
				bg, fg = s.SecondaryContainer, s.OnSecondaryContainer
			}
			if sg.Disabled {
				fg = DisabledContent()
			}
			b := ui.ButtonBase(c.Key(i)).Disabled(sg.Disabled).DimDisabled(false).FocusRing(false).
				Row().AlignItems(ui.Center).Justify(ui.Center).Gap(8).Padding(0, 12).FillHeight().
				Background(bg).Checked(on)
			if !sg.Disabled {
				b = b.Cursor(ui.CursorPointer)
			}
			if o.Fill {
				b = b.Grow(1).Basis(0)
			}
			if sg.Tooltip != "" {
				b = Tip(c, b, sg.Tooltip)
			}
			Ink(b, fg, 0)
			b.Children(func() {
				switch {
				case on && !o.NoCheck:
					Icon(c, icons.Check, 18, fg)
				case sg.Icon != 0:
					Badged(c, "", sg.Badge, func() { Icon(c, sg.Icon, 18, fg) })
				}
				if sg.Label != "" {
					Clamp(Text(c, sg.Label, style.C(fg)), 1)
				}
			})
			if sg.Label != "" {
				b.Label(sg.Label)
			} else if sg.Tooltip != "" {
				b.Label(sg.Tooltip)
			}
			if b.Clicked() {
				tap(i)
			}
		}
	})
}
