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

func providersOf(models core.J) []core.J { return models.Os("providers") }

func providerByID(models core.J, id string) core.J {
	for _, p := range providersOf(models) {
		if p.S("id") == id {
			return p
		}
	}
	return nil
}

// ModelsPage is the overview of the Pi SDK model pipeline: the default
// model, providers the user has set up, and the entry to the guided "add
// provider" flow.
type ModelsPage struct {
	persona       string
	personaFilled bool
	scroll        ui.ScrollState
}

// NewModelsPage opens the models page.
func NewModelsPage() m3.Page {
	Assistant.LoadModels(nil)
	return &ModelsPage{}
}

// MainScroll is the page's scroll.
func (p *ModelsPage) MainScroll() *ui.ScrollState { return &p.scroll }

func (p *ModelsPage) add() { m3.Nav().Push(NewAddProviderPage()) }

// View builds the page.
func (p *ModelsPage) View(c *ui.Context) {
	a := Assistant
	cs := design.S()
	tt := design.TT()
	m := a.Models
	var mine []core.J
	for _, pr := range providersOf(m) {
		if pr.IsTrue("configured") || pr.IsTrue("custom") {
			mine = append(mine, pr)
		}
	}
	sc := m3.Scaffold{FABInset: BottomInset, AppBar: &m3.AppBar{Title: l10n.CommonModel, Scrolled: p.scroll.Y > 0, Actions: func() {
		m3.MenuButton(c, "more", icons.MoreVertRounded, l10n.CommonMore, m3.BelowEnd, func() []m3.MenuItem {
			var items []m3.MenuItem
			if m.IsTrue("pi_cli_available") {
				items = append(items, m3.MenuItem{Icon: icons.DownloadForOfflineOutlined, Label: l10n.ModelsImportPi, OnTap: func() {
					a.ImportPi(func(s string, err error) {
						if err != nil {
							s = apiMessage(err)
						}
						widgets.ShowSnack(s, "", nil)
					})
				}})
			}
			return append(items, m3.MenuItem{Icon: icons.CloudSyncOutlined, Label: l10n.ModelsUpdateCatalog, OnTap: func() {
				a.CatalogProviders(true, func(_ []core.J, err error) {
					if err != nil {
						widgets.ShowSnack(apiMessage(err), "", nil)
						return
					}
					widgets.ShowSnack(l10n.ModelsCatalogUpdated, "", nil)
				})
			}})
		})
	}}}
	if m != nil {
		sc.FAB = func() {
			if (m3.FAB{Icon: icons.AddRounded, Label: l10n.ModelsAddProvider}).Show(c).Clicked() {
				p.add()
			}
		}
	}
	sc.Body = func() {
		switch {
		case !a.Ready():
			widgets.EmptyState{Icon: icons.SmartToyOutlined, Title: l10n.ModelsAssistantNotStarted, Body: l10n.ModelsAssistantNotStartedBody}.Show(c)
			return
		case m == nil && a.ModelsError != nil:
			widgets.ErrorState(c, a.ModelsError, func() { a.LoadModels(nil) })
			return
		case m == nil:
			widgets.CenteredLoader(c, "")
			return
		}
		ui.Scroll(c).Fill().TrackScroll(&p.scroll).Children(func() {
			ui.Column(c).FillWidth().Padding(design.SpaceSM, design.SpaceLG, 120, design.SpaceLG).Children(func() {
				widgets.ContentWidth(c, 820, func() {
					if a.Selected() != nil {
						p.current(c)
					} else {
						modelsOnboarding(c, p.add)
					}
					if m["error"] != nil {
						m3.VGap(c, design.SpaceMD)
						ui.Box(c).FillWidth().Padding(design.SpaceLG).Radius(design.CornerMD).Background(cs.ErrorContainer).Children(func() {
							m3.Text(c, l10n.ModelsConfigError(m["error"]), tt.BodyMedium.C(cs.OnErrorContainer))
						})
					}
					m3.VGap(c, design.SpaceXL)
					ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
						m3.Text(c, l10n.ModelsProviders, tt.TitleMedium).Grow(1).Basis(0)
						if (m3.Button{Kind: m3.TextKind, Icon: icons.AddRounded, Label: l10n.CommonAdd}).Show(c).Clicked() {
							p.add()
						}
					})
					m3.VGap(c, design.SpaceSM)
					if len(mine) == 0 {
						m3.Text(c, l10n.ModelsNoProviders, tt.BodyMedium.C(cs.OnSurfaceVariant)).Margin(design.SpaceLG, 0)
					}
					for _, pr := range mine {
						ui.Box(c.Key(pr.S("id"))).FillWidth().Margin(0, 0, design.SpaceSM, 0).Children(func() {
							providerTile(c, pr, a.Selected().S("provider") == pr.S("id"))
						})
					}
				})
			})
		})
	}
	sc.Show(c)
}

func modelsOnboarding(c *ui.Context, onStart func()) {
	cs := design.S()
	tt := design.TT()
	steps := []struct {
		icon        icons.Icon
		title, hint string
	}{
		{icons.StorefrontOutlined, l10n.ModelsStepProvider, l10n.ModelsStepProviderHint},
		{icons.KeyOutlined, l10n.ModelsStepKey, l10n.ModelsStepKeyHint},
		{icons.ChecklistRounded, l10n.ModelsStepModels, l10n.ModelsStepModelsHint},
	}
	ui.Column(c).FillWidth().Padding(design.SpaceXL).Radius(design.CornerXLIncreased).Background(cs.PrimaryContainer).Children(func() {
		m3.Text(c, l10n.ModelsOnboardingTitle, tt.HeadlineSmall.C(cs.OnPrimaryContainer))
		m3.VGap(c, design.SpaceLG)
		for i, s := range steps {
			ui.Row(c.Key(i)).FillWidth().AlignItems(ui.Center).Margin(0, 0, design.SpaceMD, 0).Children(func() {
				ui.Box(c).Size(32, 32).Radius(16).Shrink(0).Center().Background(cs.Primary).Children(func() {
					m3.Text(c, core.Str(i+1), tt.BodyMedium.C(cs.OnPrimary).W(600))
				})
				m3.HGap(c, design.SpaceMD)
				m3.Icon(c, s.icon, 24, cs.OnPrimaryContainer)
				m3.HGap(c, design.SpaceSM)
				ui.RichText(c,
					tt.TitleSmall.C(cs.OnPrimaryContainer).Span(s.title+"  "),
					tt.BodySmall.C(design.WithOpacity(cs.OnPrimaryContainer, .8)).Span(s.hint),
				).Grow(1).Basis(0)
			})
		}
		m3.VGap(c, design.SpaceSM)
		ui.Row(c).Children(func() {
			if (m3.Button{Kind: m3.Filled, Icon: icons.ArrowForwardRounded, Label: l10n.CommonGetStarted}).Show(c).Clicked() {
				onStart()
			}
		})
	})
}

func (p *ModelsPage) current(c *ui.Context) {
	a := Assistant
	cs := design.S()
	tt := design.TT()
	sel := a.Selected()
	info := a.SelectedModelInfo()
	prefs := a.Models.O("prefs")
	if !p.personaFilled {
		p.persona, p.personaFilled = prefs.S("persona"), true
	}
	providerName := sel.S("provider")
	if pr := providerByID(a.Models, sel.S("provider")); pr != nil {
		providerName = pr.S("name")
	}
	on := cs.OnPrimaryContainer
	ui.Column(c).FillWidth().Padding(design.SpaceXL).Radius(design.CornerXLIncreased).Background(cs.PrimaryContainer).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ProviderLogo{ProviderID: sel.S("provider"), Name: providerName, Size: 52}.Show(c)
			m3.HGap(c, design.SpaceLG)
			ui.Column(c).Grow(1).Basis(0).Children(func() {
				m3.Text(c, l10n.ModelsCurrent, tt.LabelMedium.C(design.WithOpacity(on, .8)))
				m3.Clamp(m3.Text(c, info.Or("name", sel.S("model")), tt.TitleLarge.C(on)), 1)
				m3.Clamp(m3.Text(c, providerName+" · "+sel.S("model"), tt.BodySmall.C(design.WithOpacity(on, .75))), 1)
			})
			if (m3.Button{Kind: m3.Tonal, Label: l10n.CommonChange}).Show(c).Clicked() {
				ShowModelPicker()
			}
		})
		if len(info) > 0 {
			m3.VGap(c, design.SpaceMD)
			ModelBadges(c, info, false)
		}
		if sel.IsTrue("reasoning") {
			m3.VGap(c, design.SpaceXL)
			ThinkingControl(c, a.Thinking(), func(k string) { a.SetPrefs(core.J{"thinking": k}, nil) }, true, true)
		}
		m3.VGap(c, design.SpaceLG)
		m3.Field{Label: l10n.ModelsPersona, Hint: l10n.ModelsPersonaHint, MinLines: 1, MaxLines: 4, Multiline: true, Filled: true,
			Fill: design.WithOpacity(cs.Surface, .7), SuffixFn: func() {
				if m3.IconBtn(c, icons.CheckRounded, l10n.CommonSave).Clicked() {
					a.SetPrefs(core.J{"persona": strings.TrimSpace(p.persona)}, func(err error) {
						if err == nil {
							widgets.ShowSnack(l10n.ModelsPersonaSaved, "", nil)
						}
					})
				}
			}}.Show(c.Key("persona"), &p.persona)
		auto := prefs["autoTitle"] != false
		if ch, v := switchTile(c, "autoTitle", m3.Tile{Title: l10n.ModelsAutoTitle, TitleStyle: ptr(tt.BodyLarge.C(on))}, auto); ch {
			a.SetPrefs(core.J{"autoTitle": v}, nil)
		}
	})
}

func ptr[T any](v T) *T { return &v }

func providerTile(c *ui.Context, p core.J, selected bool) {
	cs := design.S()
	tt := design.TT()
	available := 0
	for _, m := range p.Os("models") {
		if m.IsTrue("available") {
			available++
		}
	}
	name := p.S("name")
	if pr := PresetFor(p.S("id")); pr != nil {
		name = pr.Name
	}
	e := ui.ButtonBase(c).FocusRing(true).Cursor(ui.CursorPointer).Row().FillWidth().AlignItems(ui.Center).
		Padding(design.SpaceMD, design.SpaceMD, design.SpaceMD, design.SpaceLG).Radius(design.CornerLG).Background(cs.SurfaceContainer)
	m3.InkWell(e, design.CornerLG)
	e.Children(func() {
		net := ""
		if id := p.S("catalog_id"); id != "" {
			net = "https://models.dev/logos/" + id + ".svg"
		}
		ProviderLogo{ProviderID: p.S("id"), Name: p.S("name"), NetworkURL: net}.Show(c)
		m3.HGap(c, design.SpaceLG)
		ui.Column(c).Grow(1).Basis(0).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				m3.Clamp(m3.Text(c, name, tt.TitleSmall), 1).Shrink(1)
				if selected {
					m3.HGap(c, design.SpaceSM)
					ui.Box(c).Padding(1, 6).Radius(design.CornerFull).Background(cs.Primary).Children(func() {
						m3.Text(c, l10n.CommonInUse, tt.LabelSmall.C(cs.OnPrimary))
					})
				}
			})
			parts := []string{l10n.ModelsNoKey}
			col := cs.Error
			if p.IsTrue("configured") {
				parts[0], col = l10n.ModelsAvailableCount(available), cs.OnSurfaceVariant
			}
			if h := p.S("key_hint"); h != "" {
				parts = append(parts, h)
			}
			if p.IsTrue("custom") {
				parts = append(parts, l10n.CommonCustom)
			} else {
				parts = append(parts, l10n.CommonBuiltIn)
			}
			m3.Text(c, strings.Join(parts, " · "), tt.BodySmall.C(col))
		})
		m3.Icon(c, icons.ChevronRightRounded, 24, cs.OnSurfaceVariant)
	})
	if e.Clicked() {
		m3.Nav().Push(NewProviderSetupPage(providerSetupFrom{provider: p}))
	}
}

// ShowModelPicker is a searchable picker over every usable model of every
// configured provider.
func ShowModelPicker() {
	q := ""
	m3.ShowModalBottomSheet(m3.SheetOptions{ScrollControlled: true, MaxWidth: 640, HeightFactor: .75}, func(c *ui.Context, r *m3.Route) {
		a := Assistant
		cs := design.S()
		tt := design.TT()
		query := strings.ToLower(strings.TrimSpace(q))
		type group struct {
			p  core.J
			ms []core.J
		}
		var groups []group
		for _, p := range providersOf(a.Models) {
			if !p.IsTrue("configured") {
				continue
			}
			var ms []core.J
			for _, m := range p.Os("models") {
				if m.IsTrue("available") && (query == "" || strings.Contains(strings.ToLower(m.S("id")+" "+m.S("name")+" "+p.S("name")), query)) {
					ms = append(ms, m)
				}
			}
			if len(ms) > 0 {
				groups = append(groups, group{p, ms})
			}
		}
		ui.Column(c).FillWidth().Grow(1).Basis(0).Children(func() {
			ui.Box(c).FillWidth().Padding(0, design.SpaceXL, design.SpaceMD, design.SpaceXL).Children(func() {
				m3.SearchBar{Hint: l10n.ModelsSearch, AutoFocus: true, BG: cs.SurfaceContainerHighest, MaxWidth: -1,
					Leading: func() { m3.Icon(c, icons.SearchRounded, 24, cs.OnSurfaceVariant) }}.Show(c, &q)
			})
			if len(groups) == 0 {
				ui.Box(c).FillWidth().Grow(1).Basis(0).Center().Children(func() {
					m3.Text(c, l10n.ModelsNoneAvailable, tt.BodyMedium.C(cs.OnSurfaceVariant))
				})
				return
			}
			ui.Scroll(c).FillWidth().Grow(1).Basis(0).Children(func() {
				ui.Column(c).FillWidth().Padding(0, design.SpaceMD, design.SpaceXL, design.SpaceMD).Children(func() {
					for _, g := range groups {
						g := g
						pid := g.p.S("id")
						name := g.p.S("name")
						if pr := PresetFor(pid); pr != nil {
							name = pr.Name
						}
						ui.Row(c.Key("h:"+pid)).FillWidth().AlignItems(ui.Center).Padding(design.SpaceMD, design.SpaceSM, design.SpaceXS, design.SpaceSM).Children(func() {
							ProviderLogo{ProviderID: pid, Name: g.p.S("name"), Size: 24}.Show(c)
							m3.HGap(c, design.SpaceSM)
							m3.Text(c, name, tt.LabelLarge.C(cs.Primary))
						})
						for _, m := range g.ms {
							m := m
							sel := a.Selected()
							t := m3.Tile{Title: m.S("name"), Tap: true, Radius: design.CornerLG,
								Selected: sel.S("provider") == pid && sel.S("model") == m.S("id"),
								SubtitleFn: func() {
									ui.Box(c).Padding(4, 0, 0, 0).Children(func() { ModelBadges(c, m, true) })
								}}
							if t.Selected {
								t.TileColor = cs.SecondaryContainer
							}
							if t.Show(c.Key(pid + "/" + m.S("id"))).Clicked() {
								a.SetPrefs(core.J{"provider": pid, "model": m.S("id")}, nil)
								r.Close()
							}
						}
					}
				})
			})
		})
	})
}
