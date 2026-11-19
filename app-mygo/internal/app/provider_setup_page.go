package app

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

var apiKinds = []m3.DropdownEntry[string]{
	{Value: "openai-completions", Label: l10n.ApiOpenaiCompatible},
	{Value: "openai-responses", Label: "OpenAI Responses"},
	{Value: "anthropic-messages", Label: "Anthropic Messages"},
	{Value: "google-generative-ai", Label: "Google Gemini"},
}

func apiLabel(kind string) string {
	for _, k := range apiKinds {
		if k.Value == kind {
			return k.Label
		}
	}
	return kind
}

// providerSetupFrom is where the setup starts: a preset, a models.dev
// entry, a provider already set up, or none of them (a blank endpoint).
type providerSetupFrom struct {
	preset   *ProviderPreset
	entry    core.J
	provider core.J
}

// ProviderSetupPage is steps 2–3 of the guided flow: connect (key,
// endpoint), choose models (pulled from the provider and enriched by
// models.dev), then save and test.
type ProviderSetupPage struct {
	from                                   providerSetupFrom
	key, base, id, q, manual               string
	api, catalogID, name, keyURL, netLogo  string
	builtin, needsKey, isNew, showKey, adv bool
	// chosen is the models picked for a custom endpoint, in order.
	chosen        map[string]core.J
	order         []string
	offered       []core.J
	offeredLoaded bool
	offeredFrom   string
	loadingModels bool
	modelsError   string
	saving        bool
	test          string
	testOk        bool
	scroll        ui.ScrollState
}

// NewProviderSetupPage opens the setup.
func NewProviderSetupPage(from providerSetupFrom) m3.Page {
	p := &ProviderSetupPage{from: from, api: "openai-completions", needsKey: true, isNew: true, chosen: map[string]core.J{}}
	switch pr, e, x := from.preset, from.entry, from.provider; {
	case pr != nil:
		p.name, p.builtin, p.id, p.base = pr.Name, pr.Builtin(), pr.ProviderID(), pr.BaseURL
		p.api, p.catalogID, p.keyURL, p.needsKey = pr.APIKind(), pr.Catalog, pr.KeyURL, !pr.NoKey
	case e != nil:
		p.name, p.id, p.base, p.api = e.S("name"), e.S("id"), e.S("base_url"), e.S("api")
		p.catalogID, p.keyURL, p.netLogo = e.S("id"), e.S("doc"), e.S("logo")
	case x != nil:
		p.isNew = false
		p.name = x.S("name")
		p.builtin = !x.IsTrue("custom")
		p.id, p.base = x.S("id"), x.S("base_url")
		if apiLabel(x.S("api")) != x.S("api") {
			p.api = x.S("api")
		}
		p.catalogID = x.S("catalog_id")
		if preset := PresetFor(x.S("id")); preset != nil {
			p.name, p.keyURL, p.needsKey = preset.Name, preset.KeyURL, !preset.NoKey
			if p.catalogID == "" {
				p.catalogID = preset.Catalog
			}
		}
		if p.catalogID != "" {
			p.netLogo = "https://models.dev/logos/" + p.catalogID + ".svg"
		}
		for _, m := range x.Os("models") {
			p.choose(m)
		}
	default:
		p.name, p.adv = l10n.ProviderCustom, true
	}
	// A preset that was already added opens in edit mode.
	if p.isNew && from.preset != nil {
		if ex := providerByID(Assistant.Models, from.preset.ProviderID()); ex != nil && (ex.IsTrue("configured") || ex.IsTrue("custom")) {
			p.isNew = false
			if !p.builtin {
				for _, m := range ex.Os("models") {
					p.choose(m)
				}
			}
		}
	}
	return p
}

// MainScroll is the page's scroll.
func (p *ProviderSetupPage) MainScroll() *ui.ScrollState { return &p.scroll }

func (p *ProviderSetupPage) choose(m core.J) {
	id := m.S("id")
	if _, ok := p.chosen[id]; !ok {
		p.order = append(p.order, id)
	}
	p.chosen[id] = m
}

func (p *ProviderSetupPage) unchoose(id string) {
	delete(p.chosen, id)
	for i, x := range p.order {
		if x == id {
			p.order = append(p.order[:i], p.order[i+1:]...)
			break
		}
	}
}

func (p *ProviderSetupPage) registered() core.J {
	return providerByID(Assistant.Models, strings.TrimSpace(p.id))
}

func (p *ProviderSetupPage) piModels() []core.J { return p.registered().Os("models") }

func (p *ProviderSetupPage) configured() bool { return p.registered().IsTrue("configured") }

func (p *ProviderSetupPage) fetchModels() {
	p.loadingModels, p.modelsError = true, ""
	provider := ""
	if !p.isNew {
		provider = strings.TrimSpace(p.id)
	}
	Assistant.Discover(provider, strings.TrimSpace(p.base), p.api, strings.TrimSpace(p.key), p.catalogID, func(list []core.J, err error) {
		p.loadingModels = false
		if err != nil {
			p.modelsError = apiMessage(err)
			return
		}
		p.offered, p.offeredLoaded, p.offeredFrom = list, true, l10n.ProviderSource
	})
}

func (p *ProviderSetupPage) fromCatalog() {
	if p.catalogID == "" {
		return
	}
	p.loadingModels = true
	Assistant.CatalogModels(p.catalogID, func(list []core.J, err error) {
		p.loadingModels = false
		if err != nil {
			p.modelsError = apiMessage(err)
			return
		}
		p.offered, p.offeredLoaded, p.offeredFrom, p.modelsError = list, true, "models.dev", ""
	})
}

func (p *ProviderSetupPage) addManual() {
	id := strings.TrimSpace(p.manual)
	if id == "" {
		return
	}
	m := core.J{"id": id}
	for _, o := range p.offered {
		if o.S("id") == id {
			m = o
		}
	}
	p.choose(m)
	p.manual = ""
}

func (p *ProviderSetupPage) save() {
	a := Assistant
	id := strings.TrimSpace(p.id)
	key := strings.TrimSpace(p.key)
	if !p.builtin && len(p.chosen) == 0 {
		widgets.ShowSnack(l10n.ProviderPickModel, "", nil)
		return
	}
	if p.isNew && p.needsKey && key == "" && !p.configured() {
		widgets.ShowSnack(l10n.ProviderKeyMissing, "", nil)
		return
	}
	p.saving, p.test = true, ""
	fail := func(err error) {
		p.saving = false
		widgets.ShowSnack(apiMessage(err), "", nil)
	}
	saved := func() {
		p.isNew = false
		var models []core.J
		if p.builtin {
			for _, m := range p.piModels() {
				if m.IsTrue("available") {
					models = append(models, m)
				}
			}
		} else {
			for _, id := range p.order {
				models = append(models, p.chosen[id])
			}
		}
		finish := func() {
			widgets.ShowSnack(l10n.ProviderSaved(p.name), "", nil)
			if len(models) == 0 {
				p.saving = false
				return
			}
			p.runTest(models[0].S("id"), func() { p.saving = false })
		}
		if a.Selected() == nil && len(models) > 0 {
			a.SetPrefs(core.J{"provider": id, "model": models[0].S("id")}, func(err error) {
				if err != nil {
					fail(err)
					return
				}
				finish()
			})
			return
		}
		finish()
	}
	if p.builtin {
		if key == "" {
			saved()
			return
		}
		a.SetKey(id, key, func(err error) {
			if err != nil {
				fail(err)
				return
			}
			saved()
		})
		return
	}
	var ms []any
	for _, mid := range p.order {
		m := p.chosen[mid]
		e := core.J{"id": m["id"], "reasoning": m.IsTrue("reasoning"), "vision": m.IsTrue("vision")}
		if m["name"] != nil && m.S("name") != m.S("id") {
			e["name"] = m["name"]
		}
		for _, k := range []string{"context_window", "max_tokens", "cost_in", "cost_out", "cache_read"} {
			if m[k] != nil {
				e[k] = m[k]
			}
		}
		ms = append(ms, e)
	}
	body := core.J{"base_url": strings.TrimSpace(p.base), "api": p.api, "models": ms}
	if key != "" || p.isNew {
		switch {
		case p.needsKey:
			body["api_key"] = key
		case key == "":
			body["api_key"] = "local"
		default:
			body["api_key"] = key
		}
	}
	if p.catalogID != "" {
		body["catalog_id"] = p.catalogID
	}
	a.UpsertProvider(id, body, func(err error) {
		if err != nil {
			fail(err)
			return
		}
		saved()
	})
}

func (p *ProviderSetupPage) runTest(model string, done func()) {
	p.test = ""
	Assistant.TestProvider(strings.TrimSpace(p.id), model, func(r core.J, err error) {
		if err != nil {
			p.testOk, p.test = false, apiMessage(err)
		} else {
			p.testOk, p.test = true, l10n.ProviderTestOk(r["model"], r["latency_ms"])
		}
		done()
	})
}

func (p *ProviderSetupPage) remove() {
	title, body := l10n.ProviderDeleteTitle(p.name), l10n.ProviderDeleteBody
	if p.builtin {
		title, body = l10n.ProviderRemoveKeyTitle(p.name), l10n.ProviderRemoveKeyBody
	}
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		m3.AlertDialog(c, m3.Alert{Icon: icons.DeleteOutlineRounded, Title: title, Message: body, Actions: func() {
			if m3.TextButton(c, l10n.CommonCancel).Clicked() {
				r.Pop(false)
			}
			if m3.FilledButton(c, l10n.CommonDelete).Clicked() {
				r.Pop(true)
			}
		}})
	}, func(v any) {
		if v != true {
			return
		}
		done := func(err error) {
			if err != nil {
				widgets.ShowSnack(apiMessage(err), "", nil)
				return
			}
			m3.Nav().MaybePop()
		}
		if p.builtin {
			Assistant.SetKey(strings.TrimSpace(p.id), "", done)
		} else {
			Assistant.RemoveProvider(strings.TrimSpace(p.id), done)
		}
	})
}

// View builds the page.
func (p *ProviderSetupPage) View(c *ui.Context) {
	cs := design.S()
	tt := design.TT()
	preset := p.from.preset
	if preset == nil {
		preset = PresetFor(strings.TrimSpace(p.id))
	}
	title := p.name
	if p.isNew {
		title = l10n.ProviderAddTitle(p.name)
	}
	m3.Scaffold{
		AppBar: &m3.AppBar{Title: title, Scrolled: p.scroll.Y > 0, Actions: func() {
			if !p.isNew && m3.IconBtn(c, icons.DeleteOutlineRounded, l10n.CommonDelete).Clicked() {
				p.remove()
			}
			m3.HGap(c, design.SpaceSM)
		}},
		Bottom: func() { p.bottomBar(c) },
		Body: func() {
			ui.Scroll(c).Fill().TrackScroll(&p.scroll).Children(func() {
				ui.Column(c).FillWidth().Padding(0, design.SpaceLG, design.SpaceXXL, design.SpaceLG).Children(func() {
					widgets.ContentWidth(c, 760, func() {
						ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
							ProviderLogo{ProviderID: strings.TrimSpace(p.id), Name: p.name, Size: 64, NetworkURL: p.netLogo}.Show(c)
							m3.HGap(c, design.SpaceLG)
							ui.Column(c).Grow(1).Basis(0).Children(func() {
								m3.Text(c, p.name, tt.HeadlineSmall)
								sub := apiLabel(p.api)
								switch {
								case preset != nil && preset.Blurb != "":
									sub = preset.Blurb
								case p.builtin:
									sub = l10n.ProviderBuiltIn
								}
								m3.Text(c, sub, tt.BodyMedium.C(cs.OnSurfaceVariant))
							})
						})
						m3.VGap(c, design.SpaceXL)
						providerStep(c, 1, l10n.CommonConnect, p.configured(), func() { p.connect(c) })
						done := len(p.chosen) > 0
						if p.builtin {
							done = p.configured()
						}
						providerStep(c, 2, l10n.CommonModel, done, func() {
							if p.builtin {
								p.builtinModels(c)
							} else {
								p.customModels(c)
							}
						})
					})
				})
			})
		},
	}.Show(c)
}

func (p *ProviderSetupPage) connect(c *ui.Context) {
	cs := design.S()
	tt := design.TT()
	if p.needsKey {
		hint := l10n.ProviderKeyHint
		if p.configured() {
			hint = l10n.ProviderKeySaved(p.registered().S("key_hint"))
		}
		tip, ic := l10n.CommonShow, icons.VisibilityOutlined
		if p.showKey {
			tip, ic = l10n.CommonHide, icons.VisibilityOffOutlined
		}
		m3.Field{Label: "API Key", Hint: hint, Prefix: icons.KeyOutlined, Password: !p.showKey, SuffixFn: func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				if m3.IconBtn(c, icons.ContentPasteRounded, l10n.CommonPaste).Clicked() {
					if s := strings.TrimSpace(c.ReadClipboard()); s != "" {
						p.key = s
					}
				}
				if m3.IconBtn(c, ic, tip).Clicked() {
					p.showKey = !p.showKey
				}
			})
		}}.Show(c.Key("key"), &p.key)
	} else {
		m3.Text(c, l10n.ProviderLocalNoKey, tt.BodyMedium.C(cs.OnSurfaceVariant))
	}
	if p.keyURL != "" {
		ui.Row(c).FillWidth().Children(func() {
			label := l10n.ProviderDownloadApp(p.name)
			if p.needsKey {
				label = l10n.ProviderGetKey(p.name)
			}
			if (m3.Button{Kind: m3.TextKind, Icon: icons.OpenInNewRounded, Label: label}).Show(c).Clicked() {
				OpenLink(p.keyURL)
			}
		})
	}
	if p.builtin {
		return
	}
	m3.VGap(c, design.SpaceSM)
	m3.Field{Label: "Base URL", Prefix: icons.LinkRounded}.Show(c.Key("base"), &p.base)
	m3.VGap(c, design.SpaceSM)
	adv := ui.ButtonBase(c).FocusRing(true).Cursor(ui.CursorPointer).Row().AlignItems(ui.Center).Padding(design.SpaceSM, 0).Radius(design.CornerSM)
	turn := adv.AnimateTo("adv.turn", map[bool]float32{true: 180, false: 0}[p.adv], design.T().Motion(design.Medium1, design.Standard))
	adv.Children(func() {
		m3.Text(c, l10n.CommonAdvanced, tt.LabelLarge.C(cs.Primary))
		m3.Icon(c, icons.ExpandMoreRounded, 24, cs.Primary).Rotate(turn)
	})
	if adv.Clicked() {
		p.adv = !p.adv
	}
	if p.adv {
		ui.Column(c.Key("adv")).FillWidth().Transition(ui.ElementTransition{Duration: design.Medium2, Ease: design.Emphasized, Enter: &ui.Motion{Opacity: 0, Y: -6}}).Children(func() {
			m3.Dropdown(c, "api", m3.DropdownOpts{Label: l10n.ProviderApi}, apiKinds, &p.api)
			m3.VGap(c, design.SpaceSM)
			m3.Field{Label: l10n.ProviderId, Helper: l10n.ProviderIdHelper, Disabled: !p.isNew}.Show(c.Key("id"), &p.id)
		})
	}
}

func (p *ProviderSetupPage) bottomBar(c *ui.Context) {
	cs := design.S()
	tt := design.TT()
	ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(design.SpaceMD, design.SpaceLG, design.SpaceMD+BottomInset, design.SpaceLG).Background(cs.SurfaceContainer).Children(func() {
		key := "hint"
		if p.test != "" {
			key = p.test
		}
		ui.Box(c.Key("status:" + key)).Grow(1).Basis(0).Transition(design.Switcher(design.Medium1, nil, nil, ui.Motion{Opacity: 0})).Children(func() {
			if p.test == "" {
				hint := l10n.ProviderChosenModels(len(p.chosen))
				if p.builtin {
					hint = l10n.ProviderTestsOnSave
				}
				m3.Text(c, hint, tt.BodySmall.C(cs.OnSurfaceVariant))
				return
			}
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				ic, col, text := icons.ErrorOutlineRounded, cs.Error, cs.Error
				if p.testOk {
					ic, col, text = icons.CheckCircleRounded, cs.Primary, cs.OnSurface
				}
				m3.Icon(c, ic, 20, col)
				m3.HGap(c, design.SpaceSM)
				m3.Clamp(m3.Text(c, p.test, tt.BodySmall.C(text)), 2).Grow(1).Basis(0)
			})
		})
		m3.HGap(c, design.SpaceMD)
		label := l10n.CommonSave
		if p.isNew {
			label = l10n.ProviderSaveAndTest
		}
		if (m3.Button{Kind: m3.Filled, Icon: icons.CheckRounded, Label: label, Disabled: p.saving, Busy: p.saving}).Show(c).Clicked() {
			p.save()
		}
	})
}

func (p *ProviderSetupPage) filter(l []core.J) []core.J {
	q := strings.ToLower(strings.TrimSpace(p.q))
	if q == "" {
		return l
	}
	var out []core.J
	for _, m := range l {
		if strings.Contains(strings.ToLower(m.S("id")+" "+m.S("name")), q) {
			out = append(out, m)
		}
	}
	return out
}

func (p *ProviderSetupPage) searchField(c *ui.Context) {
	m3.Field{Hint: l10n.ModelsSearch, Prefix: icons.SearchRounded, Dense: true, Filled: true, Borderless: true, Radius: design.CornerFull}.Show(c.Key("q"), &p.q)
}

func (p *ProviderSetupPage) builtinModels(c *ui.Context) {
	cs := design.S()
	tt := design.TT()
	a := Assistant
	list := p.filter(p.piModels())
	id := strings.TrimSpace(p.id)
	m3.Text(c, l10n.ProviderBuiltInHint, tt.BodySmall.C(cs.OnSurfaceVariant))
	m3.VGap(c, design.SpaceSM)
	p.searchField(c)
	m3.VGap(c, design.SpaceSM)
	for i, m := range list {
		if i == 80 {
			break
		}
		m := m
		sel := a.Selected()
		trailing := m.S("id")
		if trailing == m.S("name") {
			trailing = ""
		}
		t := m3.Tile{Title: m.S("name"), Tap: true, Pad: true, PadX: design.SpaceSM, Radius: design.CornerMD, Disabled: !m.IsTrue("available"),
			Selected:   sel.S("provider") == id && sel.S("model") == m.S("id"),
			SubtitleFn: func() { ui.Box(c).Padding(4, 0, 0, 0).Children(func() { ModelBadges(c, m, true) }) },
			Trailing:   func() { m3.Text(c, trailing, tt.LabelSmall.C(cs.Outline)) }}
		if t.Selected {
			t.TileColor = cs.SecondaryContainer
		}
		if t.Show(c.Key(m.S("id"))).Clicked() {
			a.SetPrefs(core.J{"provider": id, "model": m.S("id")}, nil)
		}
	}
	if len(list) > 80 {
		m3.Text(c, l10n.ProviderMoreModels(len(list)-80), tt.BodySmall)
	}
}

func (p *ProviderSetupPage) customModels(c *ui.Context) {
	cs := design.S()
	tt := design.TT()
	canFetch := !p.needsKey || strings.TrimSpace(p.key) != "" || !p.isNew
	if len(p.chosen) > 0 {
		m3.Text(c, l10n.ProviderChosen, tt.LabelLarge)
		m3.VGap(c, design.SpaceXS)
		ui.Row(c).FillWidth().Wrap().Gap(design.SpaceXS).Children(func() {
			for _, id := range append([]string(nil), p.order...) {
				m := p.chosen[id]
				ch := m3.Chip{Kind: m3.InputChip, Label: id, Deletable: true}
				if m.IsTrue("reasoning") {
					ch.Avatar = icons.PsychologyOutlined
				}
				if ch.Show(c.Key(id)).Deleted {
					p.unchoose(id)
				}
			}
		})
		m3.VGap(c, design.SpaceMD)
	}
	ui.Row(c).FillWidth().Wrap().Gap(design.SpaceSM).Children(func() {
		label := l10n.ProviderFetchNeedsKey
		if canFetch {
			label = l10n.ProviderFetchModels
		}
		if (m3.Button{Kind: m3.Tonal, Icon: icons.CloudDownloadOutlined, Label: label, Disabled: p.loadingModels || !canFetch, Busy: p.loadingModels}).Show(c).Clicked() {
			p.fetchModels()
		}
		if p.catalogID != "" && (m3.Button{Kind: m3.Outlined, Icon: icons.MenuBookOutlined, Label: l10n.ProviderFromCatalog, Disabled: p.loadingModels}).Show(c).Clicked() {
			p.fromCatalog()
		}
	})
	if p.modelsError != "" {
		m3.VGap(c, design.SpaceSM)
		m3.Text(c, p.modelsError, tt.BodySmall.C(cs.Error))
		if p.catalogID != "" {
			m3.Text(c, l10n.ProviderNoModelList, tt.BodySmall)
		}
	}
	if p.offeredLoaded {
		offered := p.filter(p.offered)
		m3.VGap(c, design.SpaceMD)
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Box(c).Grow(1).Basis(0).Children(func() { p.searchField(c) })
			m3.HGap(c, design.SpaceSM)
			m3.Text(c, l10n.ProviderOfferedCount(len(offered), p.offeredFrom), tt.LabelSmall.C(cs.OnSurfaceVariant))
		})
		m3.VGap(c, design.SpaceXS)
		ui.Box(c).FillWidth().MaxHeight(440).Radius(design.CornerLG).Background(cs.SurfaceContainerLow).Clip().Children(func() {
			if len(offered) == 0 {
				m3.Text(c, l10n.ProviderNoMatch, tt.BodyMedium).Margin(design.SpaceLG)
				return
			}
			ui.Scroll(c).FillWidth().Children(func() {
				ui.Column(c).FillWidth().Padding(design.SpaceXS, 0).Children(func() {
					for _, m := range offered {
						m := m
						id := m.S("id")
						_, on := p.chosen[id]
						t := m3.Tile{Dense: true, NoRadius: true, TitleFn: func() { m3.Text(c, id, tt.BodyMedium) }, SubtitleFn: func() {
							if m["known"] == false {
								m3.Text(c, l10n.ProviderNotInCatalog, tt.LabelSmall.C(cs.Outline))
							} else {
								ModelBadges(c, m, true)
							}
						}}
						if m3.CheckTileLeading(c.Key(id), t, &on) {
							if on {
								p.choose(m)
							} else {
								p.unchoose(id)
							}
						}
					}
				})
			})
		})
	}
	m3.VGap(c, design.SpaceMD)
	ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
		ui.Box(c).Grow(1).Basis(0).Children(func() {
			if (m3.Field{Label: l10n.ProviderManualModel, Hint: l10n.ProviderManualModelHint, Dense: true}).Show(c.Key("manual"), &p.manual).Submitted() {
				p.addManual()
			}
		})
		m3.HGap(c, design.SpaceSM)
		if (m3.IconButton{Kind: m3.TonalIcon, Icon: icons.AddRounded}).Show(c).Clicked() {
			p.addManual()
		}
	})
	m3.VGap(c, design.SpaceXS)
	m3.Text(c, l10n.ProviderCatalogNote, tt.BodySmall.C(cs.OnSurfaceVariant))
}

func providerStep(c *ui.Context, n int, title string, done bool, child func()) {
	cs := design.S()
	tt := design.TT()
	ui.Column(c.Key(n)).FillWidth().Margin(0, 0, design.SpaceLG, 0).Padding(design.SpaceLG).Radius(design.CornerXL).Background(cs.SurfaceContainer).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			bg := cs.SecondaryContainer
			if done {
				bg = cs.Primary
			}
			ui.Box(c).Size(28, 28).Radius(14).Center().Background(bg).Transition(ui.ElementTransition{Duration: design.Medium2, Colors: true}).Children(func() {
				if done {
					m3.Icon(c, icons.CheckRounded, 16, cs.OnPrimary)
				} else {
					m3.Text(c, core.Str(n), tt.LabelLarge.C(cs.OnSecondaryContainer))
				}
			})
			m3.HGap(c, design.SpaceMD)
			m3.Text(c, title, tt.TitleMedium)
		})
		m3.VGap(c, design.SpaceMD)
		child()
	})
}
