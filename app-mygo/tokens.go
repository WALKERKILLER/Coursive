package main

// Coursive's design tokens, mirrored from the Flutter client so both clients
// measure and space the same way. The values are not free choices: they are
// copied from app/lib/theme/tokens.dart, and tokens_parity_test.go fails when
// the two drift apart. Change the Dart, then change this file.
const (
	// Space is the 8 dp spacing scale (M3 2026 spacing system).
	SpaceXXS  float32 = 2
	SpaceXS   float32 = 4
	SpaceSM   float32 = 8
	SpaceMD   float32 = 12
	SpaceLG   float32 = 16
	SpaceXL   float32 = 24
	SpaceXXL  float32 = 32
	SpaceXXXL float32 = 48
	SpaceHuge float32 = 64

	// Page is the horizontal page padding, and PageCompact on narrow windows.
	Page        float32 = 24
	PageCompact float32 = 16

	// Corners is the M3 shape scale, including the Expressive corners.
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

	// MaxContentWidth is the readable content width on large windows.
	MaxContentWidth float32 = 1040
)

// SizeClass is a window width class, at the same breakpoints as the Flutter
// client's sizeClassOf, so a layout switches at the same pixel on both.
type SizeClass int

const (
	SizeCompact SizeClass = iota
	SizeMedium
	SizeExpanded
	SizeLarge
	SizeExtraLarge
)

func sizeClassOf(width float32) SizeClass {
	switch {
	case width < 600:
		return SizeCompact
	case width < 840:
		return SizeMedium
	case width < 1200:
		return SizeExpanded
	case width < 1600:
		return SizeLarge
	default:
		return SizeExtraLarge
	}
}

// pagePadding is the horizontal page inset for a width.
func pagePadding(width float32) float32 {
	if sizeClassOf(width) == SizeCompact {
		return PageCompact
	}
	return Page
}

// Component metrics from app/lib/theme/theme.dart's buildTheme, which is where
// the Flutter client fixes the shape and padding of each control. Mayu's own
// defaults are close but not the same, so the view sets these explicitly.
const (
	// Buttons are StadiumBorder with EdgeInsets.symmetric(horizontal: 24, vertical: 10).
	ButtonPaddingX float32 = 24
	ButtonPaddingY float32 = 10

	// Cards: elevation 0, surfaceContainerLow, Corners.md, Clip.antiAlias.
	CardRadius    float32 = CornerMD
	CardElevation float32 = 0

	// Chips: Corners.sm, outlineVariant side, labelLarge.
	ChipRadius float32 = CornerSM

	// Text fields: outline border Corners.sm, contentPadding 16/16.
	FieldRadius   float32 = CornerSM
	FieldPaddingX float32 = 16
	FieldPaddingY float32 = 16

	// SearchBar: elevation 0, surfaceContainerHigh, minHeight 56, maxWidth 720.
	SearchBarMinHeight float32 = 56
	SearchBarMaxWidth  float32 = 720
	SearchBarPaddingX  float32 = 16

	// NavigationRail: 80 dp wide, labelMedium, indicator secondaryContainer,
	// groupAlignment -1.
	RailWidth    float32 = 80
	RailPaddingX float32 = 16

	// NavigationBar height on a narrow window.
	NavBarHeight float32 = 72
)

// Type is one step of the M3 type scale as Coursive sets it: the Flutter
// client starts from Material 3's scale and overrides the line heights and the
// weights of the larger roles.
type Type struct {
	Size       float32
	LineHeight float32
	Weight     int
}

// The scale, from Flutter's Material 3 defaults with Coursive's overrides in
// buildTheme applied: taller lines for CJK paragraphs, and a medium weight on
// the title and headline roles instead of regular.
var (
	TypeDisplayLarge   = Type{Size: 57, LineHeight: 64, Weight: 500}
	TypeDisplayMedium  = Type{Size: 45, LineHeight: 52, Weight: 500}
	TypeDisplaySmall   = Type{Size: 36, LineHeight: 44, Weight: 500}
	TypeHeadlineLarge  = Type{Size: 32, LineHeight: 40, Weight: 500}
	TypeHeadlineMedium = Type{Size: 28, LineHeight: 36, Weight: 500}
	TypeHeadlineSmall  = Type{Size: 24, LineHeight: 32, Weight: 500}
	TypeTitleLarge     = Type{Size: 22, LineHeight: 28, Weight: 500}
	TypeTitleMedium    = Type{Size: 16, LineHeight: 24, Weight: 500}
	TypeTitleSmall     = Type{Size: 14, LineHeight: 20, Weight: 500}
	TypeBodyLarge      = Type{Size: 16, LineHeight: 25.6, Weight: 400} // 1.6 line height
	TypeBodyMedium     = Type{Size: 14, LineHeight: 21.7, Weight: 400} // 1.55 line height
	TypeBodySmall      = Type{Size: 12, LineHeight: 16, Weight: 400}
	TypeLabelLarge     = Type{Size: 14, LineHeight: 20, Weight: 500}
	TypeLabelMedium    = Type{Size: 12, LineHeight: 16, Weight: 500}
	TypeLabelSmall     = Type{Size: 11, LineHeight: 16, Weight: 500}
)

// cjkFallback is the font fallback chain the Flutter client uses, so Chinese
// text picks the same family on both.
var cjkFallback = []string{
	"HarmonyOS Sans SC",
	"Noto Sans CJK SC",
	"Source Han Sans SC",
	"PingFang SC",
	"Microsoft YaHei",
}
