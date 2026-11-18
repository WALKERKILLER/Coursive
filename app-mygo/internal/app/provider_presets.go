package app

import (
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/assets"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// provider_presets.dart: providers the user can add in one step, and
// their brand marks (assets/providers, from @lobehub/icons-static-svg,
// MIT; see its NOTICE).

// Region groups the presets.
type Region int

const (
	RegionGlobal Region = iota
	RegionCn
	RegionLocal
)

// Label is the region's heading.
func (r Region) Label() string {
	switch r {
	case RegionCn:
		return l10n.RegionChina
	case RegionLocal:
		return l10n.CommonLocal
	}
	return l10n.RegionGlobal
}

// ProviderPreset is a provider the user can add in one step. Pi names a
// provider built into the Pi SDK (only a key is needed); otherwise the
// preset becomes a custom endpoint in Pi's models.json.
type ProviderPreset struct {
	ID, Name string
	// Logo is the file name under assets/providers.
	Logo    string
	Pi      string
	BaseURL string
	// API is the endpoint's protocol; empty is openai-completions.
	API string
	// Catalog is the models.dev provider id.
	Catalog string
	KeyURL  string
	Blurb   string
	Region  Region
	// Mono is a single-colour logo, tinted with onSurface.
	Mono bool
	// NoKey is a local server that needs no key.
	NoKey   bool
	Aliases []string
}

// ProviderID is the id as stored in Pi.
func (p ProviderPreset) ProviderID() string {
	if p.Pi != "" {
		return p.Pi
	}
	return p.ID
}

// Builtin tells a provider built into Pi.
func (p ProviderPreset) Builtin() bool { return p.Pi != "" }

// APIKind is the endpoint's protocol.
func (p ProviderPreset) APIKind() string {
	if p.API == "" {
		return "openai-completions"
	}
	return p.API
}

// Matches tells whether the preset matches a search.
func (p ProviderPreset) Matches(q string) bool {
	s := strings.ToLower(q)
	if strings.Contains(strings.ToLower(p.Name), s) || strings.Contains(p.ID, s) || strings.Contains(p.Pi, s) || strings.Contains(strings.ToLower(p.Blurb), s) {
		return true
	}
	for _, a := range p.Aliases {
		if strings.Contains(strings.ToLower(a), s) {
			return true
		}
	}
	return false
}

// ProviderPresets are kProviderPresets: China, then global, then local.
var ProviderPresets = []ProviderPreset{
	{ID: "deepseek", Pi: "deepseek", Name: "DeepSeek", Logo: "deepseek-color.svg", Catalog: "deepseek", Region: RegionCn, KeyURL: "https://platform.deepseek.com/api_keys", Blurb: l10n.ProviderDeepseekBlurb, Aliases: []string{"深度求索"}},
	{ID: "siliconflow", Name: l10n.ProviderSiliconflow, Logo: "siliconcloud-color.svg", BaseURL: "https://api.siliconflow.cn/v1", Catalog: "siliconflow-cn", Region: RegionCn, KeyURL: "https://cloud.siliconflow.cn/account/ak", Blurb: l10n.ProviderSiliconflowBlurb, Aliases: []string{"SiliconFlow", "siliconcloud"}},
	{ID: "bailian", Name: l10n.ProviderBailian, Logo: "bailian-color.svg", BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", Catalog: "alibaba-cn", Region: RegionCn, KeyURL: "https://bailian.console.aliyun.com/?apiKey=1", Blurb: l10n.ProviderBailianBlurb, Aliases: []string{"qwen", "dashscope", "通义", "alibaba"}},
	{ID: "zhipu", Name: l10n.ProviderZhipu, Logo: "zhipu-color.svg", BaseURL: "https://open.bigmodel.cn/api/paas/v4", Catalog: "zhipuai", Region: RegionCn, KeyURL: "https://bigmodel.cn/usercenter/proj-mgmt/apikeys", Blurb: l10n.ProviderZhipuBlurb, Aliases: []string{"glm", "zhipuai", "chatglm"}},
	{ID: "moonshotai-cn", Pi: "moonshotai-cn", Name: l10n.ProviderMoonshot, Logo: "kimi-color.svg", Catalog: "moonshotai-cn", Region: RegionCn, KeyURL: "https://platform.moonshot.cn/console/api-keys", Blurb: l10n.ProviderMoonshotBlurb, Aliases: []string{"moonshot"}},
	{ID: "volcengine", Name: l10n.ProviderVolcengine, Logo: "doubao-color.svg", BaseURL: "https://ark.cn-beijing.volces.com/api/v3", Catalog: "volcengine", Region: RegionCn, KeyURL: "https://console.volcengine.com/ark/region:ark+cn-beijing/apiKey", Blurb: l10n.ProviderVolcengineBlurb, Aliases: []string{"doubao", "ark", "bytedance"}},
	{ID: "minimax-cn", Pi: "minimax-cn", Name: "MiniMax", Logo: "minimax-color.svg", Catalog: "minimax-cn", Region: RegionCn, KeyURL: "https://platform.minimaxi.com/user-center/basic-information/interface-key", Blurb: l10n.ProviderMinimaxBlurb},
	{ID: "hunyuan", Name: l10n.ProviderHunyuan, Logo: "hunyuan-color.svg", BaseURL: "https://api.hunyuan.cloud.tencent.com/v1", Region: RegionCn, KeyURL: "https://console.cloud.tencent.com/hunyuan/api-key", Blurb: l10n.ProviderHunyuanBlurb, Aliases: []string{"tencent"}},
	{ID: "qianfan", Name: l10n.ProviderQianfan, Logo: "wenxin-color.svg", BaseURL: "https://qianfan.baidubce.com/v2", Region: RegionCn, KeyURL: "https://console.bce.baidu.com/iam/#/iam/apikey/list", Blurb: l10n.ProviderQianfanBlurb, Aliases: []string{"ernie", "文心", "baidu"}},
	{ID: "stepfun", Name: l10n.ProviderStepfun, Logo: "stepfun-color.svg", BaseURL: "https://api.stepfun.com/v1", Catalog: "stepfun", Region: RegionCn, KeyURL: "https://platform.stepfun.com/interface-key", Blurb: l10n.ProviderStepfunBlurb, Aliases: []string{"stepfun", "step"}},
	{ID: "xiaomi", Pi: "xiaomi", Name: l10n.ProviderMimo, Logo: "xiaomimimo.svg", Mono: true, Catalog: "xiaomi", Region: RegionCn, KeyURL: "https://platform.xiaomimimo.com", Blurb: l10n.ProviderMimoBlurb, Aliases: []string{"mimo"}},
	{ID: "modelscope", Name: l10n.ProviderModelscope, Logo: "modelscope-color.svg", BaseURL: "https://api-inference.modelscope.cn/v1", Catalog: "modelscope", Region: RegionCn, KeyURL: "https://modelscope.cn/my/myaccesstoken", Blurb: l10n.ProviderModelscopeBlurb},
	{ID: "ai302", Name: "302.AI", Logo: "ai302-color.svg", BaseURL: "https://api.302.ai/v1", Catalog: "302ai", Region: RegionCn, KeyURL: "https://dash.302.ai/apis/list", Blurb: l10n.ProviderPayAsYouGoBlurb},
	{ID: "aihubmix", Name: "AiHubMix", Logo: "aihubmix-color.svg", BaseURL: "https://aihubmix.com/v1", Catalog: "aihubmix", Region: RegionCn, KeyURL: "https://console.aihubmix.com/token", Blurb: l10n.ProviderOneKeyBlurb},
	{ID: "ppio", Name: l10n.ProviderPpio, Logo: "ppio-color.svg", BaseURL: "https://api.ppinfra.com/v3/openai", Region: RegionCn, KeyURL: "https://ppio.com/settings/key-management", Blurb: l10n.ProviderPpioBlurb},
	{ID: "openai", Pi: "openai", Name: "OpenAI", Logo: "openai.svg", Mono: true, Catalog: "openai", KeyURL: "https://platform.openai.com/api-keys", Blurb: l10n.ProviderOpenaiBlurb, Aliases: []string{"gpt", "chatgpt"}},
	{ID: "anthropic", Pi: "anthropic", Name: "Anthropic", Logo: "claude-color.svg", Catalog: "anthropic", KeyURL: "https://console.anthropic.com/settings/keys", Blurb: l10n.ProviderAnthropicBlurb, Aliases: []string{"claude"}},
	{ID: "google", Pi: "google", Name: "Google Gemini", Logo: "gemini-color.svg", Catalog: "google", KeyURL: "https://aistudio.google.com/apikey", Blurb: l10n.ProviderGoogleBlurb, Aliases: []string{"gemini"}},
	{ID: "openrouter", Pi: "openrouter", Name: "OpenRouter", Logo: "openrouter-color.svg", Catalog: "openrouter", KeyURL: "https://openrouter.ai/keys", Blurb: l10n.ProviderOpenrouterBlurb},
	{ID: "xai", Pi: "xai", Name: "xAI", Logo: "grok.svg", Mono: true, Catalog: "xai", KeyURL: "https://console.x.ai", Blurb: l10n.ProviderXaiBlurb, Aliases: []string{"grok"}},
	{ID: "mistral", Pi: "mistral", Name: "Mistral", Logo: "mistral-color.svg", Catalog: "mistral", KeyURL: "https://console.mistral.ai/api-keys"},
	{ID: "groq", Pi: "groq", Name: "Groq", Logo: "groq.svg", Mono: true, Catalog: "groq", KeyURL: "https://console.groq.com/keys", Blurb: l10n.ProviderGroqBlurb},
	{ID: "together", Pi: "together", Name: "Together AI", Logo: "together-color.svg", Catalog: "togetherai", KeyURL: "https://api.together.ai/settings/api-keys"},
	{ID: "fireworks", Pi: "fireworks", Name: "Fireworks", Logo: "fireworks-color.svg", Catalog: "fireworks-ai", KeyURL: "https://app.fireworks.ai/settings/users/api-keys"},
	{ID: "cerebras", Pi: "cerebras", Name: "Cerebras", Logo: "cerebras-color.svg", Catalog: "cerebras", KeyURL: "https://cloud.cerebras.ai"},
	{ID: "huggingface", Pi: "huggingface", Name: "Hugging Face", Logo: "huggingface-color.svg", Catalog: "huggingface", KeyURL: "https://huggingface.co/settings/tokens"},
	{ID: "nvidia", Pi: "nvidia", Name: "NVIDIA NIM", Logo: "nvidia-color.svg", Catalog: "nvidia", KeyURL: "https://build.nvidia.com", Blurb: l10n.ProviderFreeTrialBlurb},
	{ID: "novita", Name: "Novita AI", Logo: "novita-color.svg", BaseURL: "https://api.novita.ai/openai", Catalog: "novita-ai", KeyURL: "https://novita.ai/settings/key-management"},
	{ID: "perplexity", Name: "Perplexity", Logo: "perplexity-color.svg", BaseURL: "https://api.perplexity.ai", Catalog: "perplexity", KeyURL: "https://www.perplexity.ai/settings/api", Blurb: l10n.ProviderPerplexityBlurb},
	{ID: "ollama", Name: "Ollama", Logo: "ollama.svg", Mono: true, BaseURL: "http://localhost:11434/v1", Region: RegionLocal, NoKey: true, KeyURL: "https://ollama.com/download", Blurb: l10n.ProviderOllamaBlurb},
	{ID: "lmstudio", Name: "LM Studio", Logo: "lmstudio.svg", Mono: true, BaseURL: "http://127.0.0.1:1234/v1", Catalog: "lmstudio", Region: RegionLocal, NoKey: true, KeyURL: "https://lmstudio.ai", Blurb: l10n.ProviderLmstudioBlurb}}

// PresetFor is the preset of a provider id.
func PresetFor(providerID string) *ProviderPreset {
	for i, p := range ProviderPresets {
		if p.ProviderID() == providerID || p.ID == providerID {
			return &ProviderPresets[i]
		}
	}
	return nil
}

var (
	logoMu  sync.Mutex
	logoSVG = map[string]*ui.SVG{}
)

func presetLogo(file string) *ui.SVG {
	logoMu.Lock()
	defer logoMu.Unlock()
	if s, ok := logoSVG[file]; ok {
		return s
	}
	var s *ui.SVG
	if data, err := assets.Bytes("providers/" + file); err == nil {
		s, _ = ui.ParseSVG(data)
	}
	logoSVG[file] = s
	return s
}

var (
	netLogoMu sync.Mutex
	netLogos  = map[string]*ui.SVG{}
	netLogoAt = map[string]bool{}
)

// netLogo is a models.dev logo, fetched once; nil until it arrives.
func netLogo(url string) *ui.SVG {
	netLogoMu.Lock()
	defer netLogoMu.Unlock()
	if s, ok := netLogos[url]; ok || netLogoAt[url] {
		return s
	}
	netLogoAt[url] = true
	go func() {
		data, err := widgets.FetchURL(url)
		var s *ui.SVG
		if err == nil {
			s, _ = ui.ParseSVG(data)
		}
		Post(func() {
			netLogoMu.Lock()
			netLogos[url] = s
			netLogoMu.Unlock()
		})
	}()
	return nil
}

// ProviderLogo is a provider's brand mark: the bundled SVG of a preset,
// the models.dev logo of a catalog entry, else a tonal monogram.
type ProviderLogo struct {
	ProviderID, Name string
	// Size is 40 when zero.
	Size       float32
	NetworkURL string
}

// Show builds the logo.
func (l ProviderLogo) Show(c *ui.Context) ui.Element {
	cs := design.S()
	size := l.Size
	if size == 0 {
		size = 40
	}
	inner := size * .6
	box := ui.Box(c).Size(size, size).Shrink(0).Center().Background(cs.SurfaceContainerLowest).Radius(size*.3).
		Border(1, design.WithOpacity(cs.OutlineVariant, .6))
	box.Children(func() {
		var svg *ui.SVG
		mono := true
		if p := PresetFor(l.ProviderID); p != nil {
			svg, mono = presetLogo(p.Logo), p.Mono
		} else if l.NetworkURL != "" {
			svg = netLogo(l.NetworkURL)
		}
		switch {
		case svg != nil && mono:
			ui.Icon(c, svg).Size(inner, inner).TextColor(cs.OnSurface)
		case svg != nil:
			ui.Image(c, svg).Size(inner, inner)
		default:
			t := l.Name
			if t == "" {
				t = l.ProviderID
			}
			if r := []rune(t); len(r) > 0 {
				t = strings.ToUpper(string(r[0]))
			}
			m3.Text(c, t, design.TextStyle{Size: size * .42, Weight: 700, Height: 1}.C(cs.Primary))
		}
	})
	return box
}
