package design

import "github.com/egoist/mygo/ui"

// TextStyle is the part of Flutter's TextStyle the app uses: a size, a line
// height as a multiple of it, letter spacing, a weight and a colour. A zero
// colour leaves the element's own.
type TextStyle struct {
	Size          float32
	Height        float32
	LetterSpacing float32
	Weight        int
	Color         ui.Color
	Italic        bool
	Family        string
}

// W is the style in another weight (copyWith(fontWeight:)).
func (s TextStyle) W(w int) TextStyle { s.Weight = w; return s }

// C is the style in a colour (copyWith(color:)).
func (s TextStyle) C(c ui.Color) TextStyle { s.Color = c; return s }

// S is the style at another size (copyWith(fontSize:)).
func (s TextStyle) S(size float32) TextStyle { s.Size = size; return s }

// H is the style with another line height (copyWith(height:)).
func (s TextStyle) H(h float32) TextStyle { s.Height = h; return s }

// L is the style with other letter spacing (copyWith(letterSpacing:)).
func (s TextStyle) L(l float32) TextStyle { s.LetterSpacing = l; return s }

// F is the style in another family (copyWith(fontFamily:)).
func (s TextStyle) F(f string) TextStyle { s.Family = f; return s }

// I is the style in italics.
func (s TextStyle) I() TextStyle { s.Italic = true; return s }

// Scaled is the font size after the user's text scale, as TextScaler.linear
// scales it; letter spacing and the line height multiple stay.
func (s TextStyle) Scaled() float32 { return s.Size * T().TextScale }

// Apply styles a text element.
func (s TextStyle) Apply(e ui.Element) ui.Element {
	e = e.FontSize(s.Scaled()).FontWeight(s.Weight)
	if s.Height > 0 {
		e = e.LineHeight(s.Height)
	}
	if s.LetterSpacing != 0 {
		e = e.LetterSpacing(s.LetterSpacing)
	}
	if s.Color.A != 0 || s.Color != (ui.Color{}) {
		e = e.TextColor(s.Color)
	}
	if s.Italic {
		e = e.Italic()
	}
	if s.Family != "" {
		e = e.Font(s.Family)
	}
	return e
}

// Span is a rich-text run in the style.
func (s TextStyle) Span(text string) ui.Span {
	sp := ui.Span{Text: text, Size: s.Scaled(), Weight: s.Weight, Color: s.Color, Italic: s.Italic, Font: s.Family}
	if s.LetterSpacing != 0 {
		sp.LetterSpacing = s.LetterSpacing
	}
	return sp
}

// TextTheme is Material 3's type scale (Typography.material2021; its dense
// geometry, which Chinese takes, is the same as englishLike's) with
// buildTheme's overrides applied.
type TextTheme struct {
	DisplayLarge, DisplayMedium, DisplaySmall    TextStyle
	HeadlineLarge, HeadlineMedium, HeadlineSmall TextStyle
	TitleLarge, TitleMedium, TitleSmall          TextStyle
	LabelLarge, LabelMedium, LabelSmall          TextStyle
	BodyLarge, BodyMedium, BodySmall             TextStyle
}

// NewTextTheme is buildTheme's text theme in the colours of s: display and
// headline in onSurface, body and title too, as Typography.material2021
// colours them.
func NewTextTheme(s *Scheme) TextTheme {
	c := s.OnSurface
	st := func(size, height, ls float32, w int) TextStyle {
		return TextStyle{Size: size, Height: height, LetterSpacing: ls, Weight: w, Color: c}
	}
	return TextTheme{
		DisplayLarge:   st(57, 1.12, -0.25, 400),
		DisplayMedium:  st(45, 1.16, 0, 400),
		DisplaySmall:   st(36, 1.22, -0.5, 500),
		HeadlineLarge:  st(32, 1.25, 0, 400),
		HeadlineMedium: st(28, 1.29, -0.25, 500),
		HeadlineSmall:  st(24, 1.33, 0, 500),
		TitleLarge:     st(22, 1.27, 0, 500),
		TitleMedium:    st(16, 1.5, 0.15, 500),
		TitleSmall:     st(14, 1.43, 0.1, 500),
		LabelLarge:     st(14, 1.43, 0.1, 500),
		LabelMedium:    st(12, 1.33, 0.5, 500),
		LabelSmall:     st(11, 1.45, 0.5, 500),
		BodyLarge:      st(16, 1.6, 0.5, 400),
		BodyMedium:     st(14, 1.55, 0.25, 400),
		BodySmall:      st(12, 1.33, 0.4, 400),
	}
}
