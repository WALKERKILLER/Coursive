// Package design is the Flutter client's design system in Go: the tokens of
// app/lib/theme/tokens.dart, the colour scheme ColorScheme.fromSeed builds
// (through package mcu), the Material 3 type scale with buildTheme's
// overrides, the motion curves and the glass of app/lib/theme/glass.dart.
// None of it is a free choice: each value is copied from the Dart, and
// tokens_test.go parses the Dart and fails when the two drift apart.
package design

// Space is the 8 dp spacing scale (M3 2026 spacing system).
const (
	SpaceXXS  float32 = 2
	SpaceXS   float32 = 4
	SpaceSM   float32 = 8
	SpaceMD   float32 = 12
	SpaceLG   float32 = 16
	SpaceXL   float32 = 24
	SpaceXXL  float32 = 32
	SpaceXXXL float32 = 48
	SpaceHuge float32 = 64

	// Page is Space.page's horizontal inset, PageCompact Space.pageCompact's.
	Page        float32 = 24
	PageCompact float32 = 16
)

// Corners is the M3 shape scale, including the Expressive increased corners.
const (
	CornerNone        float32 = 0
	CornerXS          float32 = 4
	CornerSM          float32 = 8
	CornerMD          float32 = 12
	CornerLG          float32 = 16
	CornerLGIncreased float32 = 20
	CornerXL          float32 = 28
	CornerXLIncreased float32 = 32
	CornerXXL         float32 = 48
	CornerFull        float32 = 999
)

// MaxContentWidth is kMaxContentWidth, the readable width on large windows.
const MaxContentWidth float32 = 1040

// SizeClass is a window width class (dp).
type SizeClass int

const (
	SizeCompact SizeClass = iota
	SizeMedium
	SizeExpanded
	SizeLarge
	SizeExtraLarge
)

// SizeClassOf is sizeClassOf.
func SizeClassOf(width float32) SizeClass {
	switch {
	case width < 600:
		return SizeCompact
	case width < 840:
		return SizeMedium
	case width < 1200:
		return SizeExpanded
	case width < 1600:
		return SizeLarge
	}
	return SizeExtraLarge
}

// PagePadding is the horizontal page inset for a window width.
func PagePadding(width float32) float32 {
	if SizeClassOf(width) == SizeCompact {
		return PageCompact
	}
	return Page
}
