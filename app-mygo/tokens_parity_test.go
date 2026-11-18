package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
)

// The design system has one source of truth: the Flutter client's tokens. This
// test reads that file and fails when the Go mirror drifts, so "the two clients
// look the same" is checked rather than asserted in a review.

const tokensPath = "../app/lib/theme/tokens.dart"

// dartConsts pulls `static const double <name> = <number>;` out of one class in
// tokens.dart.
func dartConsts(t *testing.T, src, class string) map[string]float32 {
	t.Helper()
	block := regexp.MustCompile(`abstract final class ` + class + ` \{([\s\S]*?)\n\}`).
		FindStringSubmatch(src)
	if block == nil {
		t.Fatalf("no class %s in %s", class, tokensPath)
	}
	out := map[string]float32{}
	for _, m := range regexp.MustCompile(`static const double (\w+) = (-?[0-9.]+);`).FindAllStringSubmatch(block[1], -1) {
		v, err := strconv.ParseFloat(m[2], 32)
		if err != nil {
			t.Fatalf("%s: %v", m[1], err)
		}
		out[m[1]] = float32(v)
	}
	return out
}

func TestSpaceMatchesTheFlutterTokens(t *testing.T) {
	src := readTokens(t)
	space := dartConsts(t, src, "Space")
	want := map[string]float32{
		"xxs": SpaceXXS, "xs": SpaceXS, "sm": SpaceSM, "md": SpaceMD,
		"lg": SpaceLG, "xl": SpaceXL, "xxl": SpaceXXL, "xxxl": SpaceXXXL, "huge": SpaceHuge,
	}
	for name, got := range want {
		if dart, ok := space[name]; !ok || dart != got {
			t.Errorf("Space.%s: Dart %v, Go %v", name, dart, got)
		}
	}
}

func TestCornersMatchTheFlutterTokens(t *testing.T) {
	src := readTokens(t)
	corners := dartConsts(t, src, "Corners")
	want := map[string]float32{
		"none": CornerNone, "xs": CornerXS, "sm": CornerSM, "md": CornerMD,
		"lg": CornerLG, "lgIncreased": CornerLGIncreased, "xl": CornerXL,
		"xlIncreased": CornerXLIncreased, "xxl": CornerXXL, "full": CornerFull,
	}
	for name, got := range want {
		if dart, ok := corners[name]; !ok || dart != got {
			t.Errorf("Corners.%s: Dart %v, Go %v", name, dart, got)
		}
	}
}

func TestPagePaddingMatchesTheFlutterTokens(t *testing.T) {
	src := readTokens(t)
	// static const EdgeInsets page = EdgeInsets.symmetric(horizontal: 24);
	re := regexp.MustCompile(`EdgeInsets\.symmetric\(horizontal: ([0-9.]+)\)`)
	m := re.FindAllStringSubmatch(src, -1)
	if len(m) < 2 {
		t.Fatalf("page paddings not found in %s", tokensPath)
	}
	page, _ := strconv.ParseFloat(m[0][1], 32)
	compact, _ := strconv.ParseFloat(m[1][1], 32)
	if float32(page) != Page {
		t.Errorf("page padding: Dart %v, Go %v", page, Page)
	}
	if float32(compact) != PageCompact {
		t.Errorf("compact page padding: Dart %v, Go %v", compact, PageCompact)
	}
}

func TestMaxContentWidthMatchesTheFlutterTokens(t *testing.T) {
	src := readTokens(t)
	m := regexp.MustCompile(`kMaxContentWidth = ([0-9.]+);`).FindStringSubmatch(src)
	if m == nil {
		t.Fatalf("kMaxContentWidth not found in %s", tokensPath)
	}
	v, _ := strconv.ParseFloat(m[1], 32)
	if float32(v) != MaxContentWidth {
		t.Errorf("kMaxContentWidth: Dart %v, Go %v", v, MaxContentWidth)
	}
}

// TestSizeClassBreakpointsMatchTheFlutterTokens checks the layout switches at
// the same pixel width in both clients.
func TestSizeClassBreakpointsMatchTheFlutterTokens(t *testing.T) {
	src := readTokens(t)
	block := regexp.MustCompile(`SizeClass sizeClassOf\(double width\) \{([\s\S]*?)\n\}`).FindStringSubmatch(src)
	if block == nil {
		t.Fatalf("sizeClassOf not found in %s", tokensPath)
	}
	var want []float32
	for _, m := range regexp.MustCompile(`width < ([0-9.]+)`).FindAllStringSubmatch(block[1], -1) {
		v, _ := strconv.ParseFloat(m[1], 32)
		want = append(want, float32(v))
	}
	if len(want) != 4 {
		t.Fatalf("expected four breakpoints, found %v", want)
	}
	// Just below each breakpoint is the lower class; at it, the next one.
	for _, b := range want {
		if sizeClassOf(b-1) == sizeClassOf(b) {
			t.Errorf("Go does not switch at %v", b)
		}
	}
	classes := []SizeClass{SizeCompact, SizeMedium, SizeExpanded, SizeLarge, SizeExtraLarge}
	for i, b := range want {
		if got := sizeClassOf(b); got != classes[i+1] {
			t.Errorf("at %v Go says %v, want %v", b, got, classes[i+1])
		}
	}
}

// TestSeedMatchesTheFlutterDefault checks the colour both clients derive their
// palette from.
func TestSeedMatchesTheFlutterDefault(t *testing.T) {
	prefs, err := os.ReadFile(filepath.Join("..", "app", "lib", "theme", "prefs.dart"))
	if err != nil {
		t.Fatalf("read prefs.dart: %v", err)
	}
	m := regexp.MustCompile(`Color seed = const Color\(0x([0-9A-Fa-f]{8})\)`).FindSubmatch(prefs)
	if m == nil {
		t.Fatal("default seed not found in prefs.dart")
	}
	want, err := strconv.ParseUint(string(m[1]), 16, 32)
	if err != nil {
		t.Fatal(err)
	}
	// Dart writes ARGB; ui.Color holds straight R, G, B, A.
	got := uint32(seed.A)<<24 | uint32(seed.R)<<16 | uint32(seed.G)<<8 | uint32(seed.B)
	if got != uint32(want) {
		t.Errorf("seed: Dart #%08X, Go #%08X", want, got)
	}
}

func readTokens(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(tokensPath)
	if err != nil {
		t.Fatalf("read %s: %v", tokensPath, err)
	}
	return string(b)
}
