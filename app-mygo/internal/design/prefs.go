package design

import (
	"encoding/json"

	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
)

// ThemeMode is Flutter's ThemeMode.
type ThemeMode string

const (
	ThemeSystem ThemeMode = "system"
	ThemeLight  ThemeMode = "light"
	ThemeDark   ThemeMode = "dark"
)

// Contrast is ContrastLevel.
type Contrast string

const (
	ContrastStandard Contrast = "standard"
	ContrastMedium   Contrast = "medium"
	ContrastHigh     Contrast = "high"
)

// Contrasts lists the levels in the order the settings offer them.
var Contrasts = []Contrast{ContrastStandard, ContrastMedium, ContrastHigh}

// Value is the contrast level ColorScheme.fromSeed takes.
func (c Contrast) Value() float64 {
	switch c {
	case ContrastMedium:
		return 0.5
	case ContrastHigh:
		return 1
	}
	return 0
}

// Label is the level's name in the settings.
func (c Contrast) Label() string {
	switch c {
	case ContrastMedium:
		return l10n.LevelMedium
	case ContrastHigh:
		return l10n.LevelHigh
	}
	return l10n.LevelStandard
}

// Stream animations are GptMarkdownAnimation's names.
const (
	StreamNone       = "none"
	StreamTypewriter = "typewriter"
	StreamFade       = "fade"
	StreamBlurIn     = "blurIn"
	StreamWave       = "wave"
)

// Prefs is UiPrefs: presentation preferences kept on the device under the
// shared-preferences key "ui", in the same JSON the Flutter client writes.
type Prefs struct {
	ThemeMode      ThemeMode
	DynamicColor   bool
	Seed           uint32
	Variant        Variant
	Contrast       Contrast
	TextScale      float64
	FontFamily     string // "" is the system default (null in the JSON)
	CompactDensity bool
	ReduceMotion   bool
	Glass          bool

	ChatBubbles     bool
	ShowThinking    bool
	StreamAnimation string
	ChatTextScale   float64
	AssistantDocked bool

	LecturesNewestFirst bool

	DefaultSpeed  float64
	BoostSpeed    float64
	SeekStepSec   int
	AutoResume    bool
	SlideFollow   bool
	SubtitleScale float64
	ShowSubtitles bool

	OutlineDetailed bool
}

// DefaultPrefs are UiPrefs' initial values.
func DefaultPrefs() Prefs {
	return Prefs{
		ThemeMode: ThemeSystem, DynamicColor: true, Seed: 0xFF3F6CB4, Variant: TonalSpot,
		Contrast: ContrastStandard, TextScale: 1, Glass: true,
		ChatBubbles: true, ShowThinking: true, StreamAnimation: StreamFade, ChatTextScale: 1, AssistantDocked: true,
		DefaultSpeed: 1, BoostSpeed: 2, SeekStepSec: 10, AutoResume: true, SlideFollow: true,
		SubtitleScale: 1, ShowSubtitles: true,
	}
}

var variantNames = map[Variant]string{
	TonalSpot: "tonalSpot", Neutral: "neutral", Vibrant: "vibrant", Expressive: "expressive",
	Fidelity: "fidelity", Content: "content", Monochrome: "monochrome",
}

// Variants lists the scheme variants in kVariantLabels' order.
var Variants = []Variant{TonalSpot, Neutral, Vibrant, Expressive, Fidelity, Content, Monochrome}

// VariantLabel is kVariantLabels.
func VariantLabel(v Variant) string {
	switch v {
	case Neutral:
		return l10n.SchemeNeutral
	case Vibrant:
		return l10n.SchemeVibrant
	case Expressive:
		return l10n.SchemeExpressive
	case Fidelity:
		return l10n.SchemeFidelity
	case Content:
		return l10n.SchemeContent
	case Monochrome:
		return l10n.SchemeMonochrome
	}
	return l10n.SchemeTonalSpot
}

// SeedChoice is one of kSeedPalette's curated seeds.
type SeedChoice struct {
	Label string
	Seed  uint32
}

// SeedPalette is kSeedPalette.
var SeedPalette = []SeedChoice{
	{l10n.ColorAzure, 0xFF3F6CB4},
	{l10n.ColorIndigo, 0xFF2E6E6A},
	{l10n.ColorPine, 0xFF4F7A3A},
	{l10n.ColorAmber, 0xFFB0782A},
	{l10n.ColorCinnabar, 0xFFB4443A},
	{l10n.ColorBegonia, 0xFFB0507A},
	{l10n.ColorWisteria, 0xFF6E5AA8},
	{l10n.ColorGraphite, 0xFF5D6670},
}

// FontChoice is one of kFontChoices; Family "" is the system default.
type FontChoice struct {
	Label  string
	Family string
}

// FontChoices is kFontChoices.
var FontChoices = []FontChoice{
	{l10n.FontSystem, ""},
	{l10n.FontHarmonySans, "HarmonyOS Sans SC"},
	{l10n.FontSourceHanSans, "Noto Sans CJK SC"},
	{l10n.FontLxgwWenKai, "LXGW WenKai"},
	{l10n.FontInterWithSystem, "Inter"},
}

// JSON is the preferences in their stored form (UiPrefs.toJson).
func (p Prefs) JSON() map[string]any {
	var font any
	if p.FontFamily != "" {
		font = p.FontFamily
	}
	return map[string]any{
		"themeMode": string(p.ThemeMode), "dynamicColor": p.DynamicColor, "seed": p.Seed,
		"variant": variantNames[p.Variant], "contrast": string(p.Contrast), "textScale": p.TextScale,
		"fontFamily": font, "compactDensity": p.CompactDensity, "reduceMotion": p.ReduceMotion,
		"glass": p.Glass, "chatBubbles": p.ChatBubbles, "showThinking": p.ShowThinking,
		"streamAnimation": p.StreamAnimation, "chatTextScale": p.ChatTextScale,
		"assistantDocked": p.AssistantDocked, "lecturesNewestFirst": p.LecturesNewestFirst,
		"defaultSpeed": p.DefaultSpeed, "boostSpeed": p.BoostSpeed, "seekStepSec": p.SeekStepSec,
		"autoResume": p.AutoResume, "slideFollow": p.SlideFollow, "subtitleScale": p.SubtitleScale,
		"showSubtitles": p.ShowSubtitles, "outlineDetailed": p.OutlineDetailed,
	}
}

// Apply reads stored-form values over p, as UiPrefs._fromJson does: a key
// that is absent keeps its default, and a value of the wrong kind makes the
// whole patch fail (ok false) so the caller can keep what it had.
func (p *Prefs) Apply(m map[string]any) (ok bool) {
	next := *p
	defer func() {
		if recover() != nil {
			ok = false
			return
		}
		*p = next
		ok = true
	}()
	str := func(k, def string) string {
		if v, has := m[k]; has && v != nil {
			return v.(string)
		}
		return def
	}
	boolean := func(k string, def bool) bool {
		if v, has := m[k]; has && v != nil {
			return v.(bool)
		}
		return def
	}
	num := func(k string, def float64) float64 {
		if v, has := m[k]; has && v != nil {
			switch n := v.(type) {
			case float64:
				return n
			case int:
				return float64(n)
			case json.Number:
				f, err := n.Float64()
				if err != nil {
					panic(err)
				}
				return f
			}
			panic("not a number")
		}
		return def
	}
	switch mode := ThemeMode(str("themeMode", "system")); mode {
	case ThemeSystem, ThemeLight, ThemeDark:
		next.ThemeMode = mode
	default:
		panic("themeMode")
	}
	next.DynamicColor = boolean("dynamicColor", next.DynamicColor)
	next.Seed = uint32(num("seed", float64(next.Seed)))
	vname := str("variant", variantNames[next.Variant])
	found := false
	for v, n := range variantNames {
		if n == vname {
			next.Variant, found = v, true
		}
	}
	if !found {
		panic("variant")
	}
	switch c := Contrast(str("contrast", string(next.Contrast))); c {
	case ContrastStandard, ContrastMedium, ContrastHigh:
		next.Contrast = c
	default:
		panic("contrast")
	}
	next.TextScale = num("textScale", 1)
	next.FontFamily = str("fontFamily", "")
	next.CompactDensity = boolean("compactDensity", false)
	next.ReduceMotion = boolean("reduceMotion", false)
	next.Glass = boolean("glass", true)
	next.ChatBubbles = boolean("chatBubbles", true)
	next.ShowThinking = boolean("showThinking", true)
	switch a := str("streamAnimation", next.StreamAnimation); a {
	case StreamNone, StreamTypewriter, StreamFade, StreamBlurIn, StreamWave:
		next.StreamAnimation = a
	default:
		panic("streamAnimation")
	}
	next.ChatTextScale = num("chatTextScale", 1)
	next.AssistantDocked = boolean("assistantDocked", true)
	next.LecturesNewestFirst = boolean("lecturesNewestFirst", false)
	next.DefaultSpeed = num("defaultSpeed", 1)
	next.BoostSpeed = num("boostSpeed", 2)
	next.SeekStepSec = int(num("seekStepSec", 10))
	next.AutoResume = boolean("autoResume", true)
	next.SlideFollow = boolean("slideFollow", true)
	next.SubtitleScale = num("subtitleScale", 1)
	next.ShowSubtitles = boolean("showSubtitles", true)
	next.OutlineDetailed = boolean("outlineDetailed", false)
	return true
}
