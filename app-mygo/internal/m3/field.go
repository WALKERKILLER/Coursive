package m3

import (
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
)

// Field is a TextField with the app's InputDecorationTheme: outlined with
// Corners.sm, 16 DIPs of content padding, a label that floats onto the
// border, and helper or error text below.
type Field struct {
	Label, Hint, Helper, Error string
	Prefix, Suffix             icons.Icon
	// PrefixFn and SuffixFn build widgets in place of the icons, as an
	// IconButton clearing the field.
	PrefixFn, SuffixFn     func()
	PrefixText, SuffixText string
	Password               bool
	// MinLines and MaxLines make it a text area; MaxLines 0 with MinLines
	// set grows without limit (maxLines: null).
	MinLines, MaxLines int
	Multiline          bool
	Disabled, ReadOnly bool
	AutoFocus          bool
	MaxLength          int
	// Counter shows the length against MaxLength.
	Counter bool
	// Borderless is InputBorder.none.
	Borderless bool
	// Filled fills the field with Fill (filled: true).
	Filled bool
	Fill   ui.Color
	// PadX and PadY replace the content padding when not negative-zero:
	// set Pad to use them.
	Pad        bool
	PadX, PadY float32
	Style      *design.TextStyle
	// Radius replaces Corners.sm.
	Radius float32
	// Height fixes a text area's height.
	Height float32
	// Dense is isDense: smaller icon boxes.
	Dense bool
}

type fieldState struct{}

// Show builds the field editing value and returns the input, whose
// Changed and Submitted report edits and Enter.
func (f Field) Show(c *ui.Context, value *string) ui.Element {
	s := cs()
	style := tt().BodyLarge.C(s.OnSurface)
	if f.Style != nil {
		style = *f.Style
		if style.Color.A == 0 {
			style.Color = s.OnSurface
		}
	}
	if f.Disabled {
		style.Color = DisabledContent()
	}
	padX, padY := float32(16), float32(16)
	if f.Pad {
		padX, padY = f.PadX, f.PadY
	}
	radius := f.Radius
	if radius == 0 {
		radius = design.CornerSM
	}
	multi := f.Multiline || f.MinLines > 0 || f.MaxLines > 1
	iconBox := float32(48)
	if f.Dense {
		iconBox = 40
	}
	var input ui.Element
	outer := ui.Column(c).Shrink(0)
	var box ui.Element
	outer.Children(func() {
		box = ui.Row(c).AlignItems(ui.Center).Radius(radius).Cursor(ui.CursorText)
		if !multi {
			box = box.AlignItems(ui.Center)
		} else {
			box = box.AlignItems(ui.Start)
		}
		if f.Filled {
			fill := f.Fill
			if fill.A == 0 {
				fill = s.SurfaceContainerHighest
			}
			box = box.Background(fill)
		}
		box.Children(func() {
			if f.Prefix != 0 || f.PrefixFn != nil {
				ui.Row(c).Size(iconBox, iconBox).Center().Shrink(0).Children(func() {
					if f.PrefixFn != nil {
						f.PrefixFn()
					} else {
						Icon(c, f.Prefix, 24, s.OnSurfaceVariant)
					}
				})
			} else if !f.Borderless || padX > 0 {
				HGap(c, padX)
			}
			ui.Row(c).Grow(1).Basis(0).AlignItems(ui.Center).Gap(0).PaddingY(padY).Children(func() {
				if f.PrefixText != "" {
					Text(c, f.PrefixText, style.C(s.OnSurfaceVariant)).NoWrap()
				}
				if multi {
					input = ui.TextAreaBase(c, value)
					max := f.MaxLines
					if max == 0 {
						max = 1 << 20
					}
					min := f.MinLines
					if min == 0 {
						min = 1
					}
					input = input.Lines(min, max)
					if f.Height > 0 {
						input = input.Height(f.Height)
					}
				} else {
					input = ui.TextInputBase(c, value)
					if f.Password {
						input = input.Password()
					}
				}
				input = style.Apply(input.Grow(1).Basis(0)).CaretColor(s.Primary).
					SelectionColor(design.WithOpacity(s.Primary, .4)).Disabled(f.Disabled).ReadOnly(f.ReadOnly)
				if f.Label != "" {
					input = input.Label(f.Label)
				}
				if f.MaxLength > 0 {
					input = input.MaxLength(f.MaxLength)
				}
				if f.AutoFocus {
					input = input.AutoFocus()
				}
				if f.SuffixText != "" {
					Text(c, f.SuffixText, style.C(s.OnSurfaceVariant)).NoWrap()
				}
			})
			if f.Suffix != 0 || f.SuffixFn != nil {
				ui.Row(c).Size(iconBox, iconBox).Center().Shrink(0).Children(func() {
					if f.SuffixFn != nil {
						f.SuffixFn()
					} else {
						ic := s.OnSurfaceVariant
						if f.Error != "" {
							ic = s.Error
						}
						Icon(c, f.Suffix, 24, ic)
					}
				})
			} else if !f.Borderless || padX > 0 {
				HGap(c, padX)
			}
		})
		focused := input.Focused()
		if box.Clicked() && !focused && !f.Disabled {
			input.Focus()
		}
		empty := *value == ""
		floating := focused || !empty
		hint := f.Hint
		if f.Label != "" && !focused {
			hint = ""
		}
		if empty && hint != "" {
			input.Placeholder(hint).PlaceholderColor(s.OnSurfaceVariant)
		}
		anim := design.T().Motion(fieldTransition, design.FastOutSlowIn)
		fl := box.AnimateTo("m3.field.float", map[bool]float32{false: 0, true: 1}[floating || f.Label == ""], anim)
		hovered := box.Hovered() || input.Hovered()
		border, width := s.Outline, float32(1)
		labelC := s.OnSurfaceVariant
		switch {
		case f.Disabled:
			border, labelC = design.WithOpacity(s.OnSurface, .12), DisabledContent()
		case f.Error != "":
			border, labelC = s.Error, s.Error
			if focused {
				width = 2
			}
		case focused:
			border, width, labelC = s.Primary, 2, s.Primary
		case hovered:
			border = s.OnSurface
		}
		borderC := box.AnimateColor("m3.field.border", border, anim)
		bw := box.AnimateTo("m3.field.width", width, anim)
		lc := box.AnimateColor("m3.field.label", labelC, anim)
		label := f.Label
		restX := padX
		if f.Prefix != 0 || f.PrefixFn != nil {
			restX = iconBox
		}
		borderless := f.Borderless
		box.DrawOver(func(p *ui.Painter, r ui.Rect) {
			gapStart, gapEnd := float32(0), float32(0)
			if label != "" {
				size := 16 - 4*fl
				sp := ui.Span{Text: label, Size: size * design.T().TextScale, Color: lc, LetterSpacing: .5 - .1*fl}
				lw, lh := p.MeasureText(r.W, sp)
				x := restX + (16-restX)*fl
				restY := padY
				if !multi {
					restY = (r.H - lh) / 2
				}
				y := restY + (-lh/2-restY)*fl
				if fl > 0 && !borderless {
					gapStart, gapEnd = x-4, x+lw+4
				}
				if !(empty && !focused && hint != "" && fl == 0 && false) {
					p.RichText(r.X+x, r.Y+y, r.W, sp)
				}
			}
			if borderless {
				return
			}
			in := bw / 2
			rr := ui.Rect{X: r.X + in, Y: r.Y + in, W: r.W - bw, H: r.H - bw}
			if gapEnd <= gapStart {
				p.Stroke(rr, borderC, radius-in, bw)
				return
			}
			p.StrokePath(outlineWithGap(rr, radius-in, gapStart, gapEnd), bw, borderC)
		})
		// _HelperError: the error fades in over the helper, which fades
		// out, and comes down from a quarter of its height above, linearly
		// over 167 ms; going, it goes back the same way.
		shown := ui.Local(outer, "m3.field.shown", func() string { return "" })
		if f.Error != "" {
			*shown = f.Error
		}
		ev := outer.AnimateTo("m3.field.error", map[bool]float32{false: 0, true: 1}[f.Error != ""], design.T().Motion(fieldTransition, ui.Linear))
		if ev == 0 {
			*shown = ""
		}
		if *shown != "" || f.Helper != "" || f.Counter {
			ui.Row(c).Padding(4, 16, 0, 16).Gap(16).Children(func() {
				ui.Box(c).Grow(1).Basis(0).Children(func() {
					helper := Text(c, f.Helper, tt().BodySmall.C(s.OnSurfaceVariant))
					if *shown == "" {
						return
					}
					helper.Opacity(1 - ev).Absolute().Top(0).Left(0).Right(0)
					Text(c, *shown, tt().BodySmall.C(s.Error)).Opacity(ev).TranslatePercent(0, -25*(1-ev))
				})
				if f.Counter && f.MaxLength > 0 {
					Text(c, itoa(utf8.RuneCountInString(*value))+"/"+itoa(f.MaxLength), tt().BodySmall.C(s.OnSurfaceVariant))
				}
			})
		}
	})
	return input
}

// fieldTransition is InputDecorator's: the label floats and the border
// changes over 167 ms.
const fieldTransition = 167 * time.Millisecond

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// outlineWithGap is a rounded rectangle's outline with a gap in its top
// edge from x0 to x1 (relative to r), where a floating label sits.
func outlineWithGap(r ui.Rect, rad, x0, x1 float32) *ui.Path {
	k := rad * kappa
	L, T, R, B := r.X, r.Y, r.X+r.W, r.Y+r.H
	p := &ui.Path{}
	p.MoveTo(L+max(x1, rad), T).LineTo(R-rad, T).
		CubeTo(R-rad+k, T, R, T+rad-k, R, T+rad).
		LineTo(R, B-rad).
		CubeTo(R, B-rad+k, R-rad+k, B, R-rad, B).
		LineTo(L+rad, B).
		CubeTo(L+rad-k, B, L, B-rad+k, L, B-rad).
		LineTo(L, T+rad).
		CubeTo(L, T+rad-k, L+rad-k, T, L+rad, T).
		LineTo(L+max(x0, rad), T)
	return p
}

const kappa = 0.5522847498
