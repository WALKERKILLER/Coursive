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

// AddProviderPage is step 1 of the guided flow: pick a provider from
// curated presets, the models.dev directory, or start a blank compatible
// endpoint.
type AddProviderPage struct {
	q            string
	region       *Region
	catalog      []core.J
	catalogError string
	loaded       bool
	scroll       ui.ScrollState
}

// NewAddProviderPage opens the page and loads the models.dev directory.
func NewAddProviderPage() m3.Page {
	p := &AddProviderPage{}
	Assistant.CatalogProviders(false, func(v []core.J, err error) {
		if err != nil {
			p.catalogError = apiMessage(err)
			return
		}
		p.catalog, p.loaded = v, true
	})
	return p
}

// MainScroll is the page's scroll.
func (p *AddProviderPage) MainScroll() *ui.ScrollState { return &p.scroll }

func (p *AddProviderPage) open(from providerSetupFrom) { m3.Nav().Replace(NewProviderSetupPage(from)) }

// View builds the page.
func (p *AddProviderPage) View(c *ui.Context) {
	cs := design.S()
	tt := design.TT()
	q := strings.TrimSpace(p.q)
	configured := map[string]bool{}
	for _, pr := range providersOf(Assistant.Models) {
		if pr.IsTrue("configured") || pr.IsTrue("custom") {
			configured[pr.S("id")] = true
		}
	}
	var presets []ProviderPreset
	presetCatalog := map[string]bool{}
	for _, pr := range ProviderPresets {
		if pr.Catalog != "" {
			presetCatalog[pr.Catalog] = true
		}
		if (p.region == nil || pr.Region == *p.region) && (q == "" || pr.Matches(q)) {
			presets = append(presets, pr)
		}
	}
	var more []core.J
	if q != "" && (p.region == nil || *p.region != RegionLocal) {
		for _, e := range p.catalog {
			if !presetCatalog[e.S("id")] && strings.Contains(strings.ToLower(e.S("id")+" "+e.S("name")), strings.ToLower(q)) {
				more = append(more, e)
			}
		}
	}
	moreCount := len(p.catalog) - len(presetCatalog)
	header := func(title, trailing string) {
		ui.Row(c.Key("h:"+title)).FillWidth().AlignItems(ui.Center).Padding(design.SpaceXL, design.SpaceLG, design.SpaceSM, design.SpaceLG+design.SpaceXS).Children(func() {
			m3.Text(c, title, tt.TitleSmall.C(cs.Primary)).Grow(1).Basis(0)
			if trailing != "" {
				m3.Text(c, trailing, tt.LabelSmall)
			}
		})
	}
	m3.Scaffold{AppBar: &m3.AppBar{Title: l10n.ModelsAddProvider, Scrolled: p.scroll.Y > 0}, Body: func() {
		ui.Scroll(c).Fill().TrackScroll(&p.scroll).Children(func() {
			ui.Column(c).FillWidth().AlignItems(ui.Center).Children(func() {
				widgets.ContentWidth(c, 960, func() {
					ui.Column(c).FillWidth().Padding(0, design.SpaceLG, design.SpaceMD, design.SpaceLG).Children(func() {
						m3.SearchBar{Hint: l10n.AddProviderSearchHint, AutoFocus: true, BG: cs.SurfaceContainerHigh, MaxWidth: -1,
							Leading: func() { m3.Icon(c, icons.SearchRounded, 24, cs.OnSurfaceVariant) },
							Trailing: func() {
								if q != "" && m3.IconBtn(c, icons.CloseRounded, l10n.CommonClear).Clicked() {
									p.q = ""
								}
							}}.Show(c, &p.q)
						m3.VGap(c, design.SpaceMD)
						ui.Row(c).FillWidth().Wrap().Gap(design.SpaceSM).Children(func() {
							if m3.ChoiceChipOf(c.Key("all"), l10n.CommonAll, p.region == nil) {
								p.region = nil
							}
							for _, r := range []Region{RegionGlobal, RegionCn, RegionLocal} {
								r := r
								if m3.ChoiceChipOf(c.Key(int(r)), r.Label(), p.region != nil && *p.region == r) {
									p.region = &r
								}
							}
						})
					})
					grid := ui.Row(c).FillWidth().Wrap().Gap(design.SpaceSM).Padding(0, design.SpaceLG)
					w := widgets.Width(grid, 900) - 2*design.SpaceLG
					cols := min(max(int(w/220), 2), 4)
					cw := (w - design.SpaceSM*float32(cols-1)) / float32(cols)
					grid.Children(func() {
						for _, pr := range presets {
							pr := pr
							ui.Box(c.Key(pr.ID)).Width(cw).Children(func() {
								if presetTile(c, pr, configured[pr.ProviderID()]) {
									p.open(providerSetupFrom{preset: &pr})
								}
							})
						}
					})
					if len(presets) == 0 && len(more) == 0 {
						text := l10n.AddProviderLoadingCatalog
						if p.loaded {
							text = l10n.AddProviderNoMatch(q)
						}
						m3.Text(c, text, tt.BodyMedium.C(cs.OnSurfaceVariant)).TextAlign(ui.Center).Margin(design.SpaceXL)
					}
					if len(more) > 0 {
						header(l10n.AddProviderCatalog, l10n.AddProviderMatches(len(more)))
						ui.Column(c).FillWidth().Padding(0, design.SpaceLG).Children(func() {
							for _, e := range more {
								e := e
								t := m3.Tile{Leading: func() { ProviderLogo{ProviderID: e.S("id"), Name: e.S("name"), NetworkURL: e.S("logo")}.Show(c) },
									Title: e.S("name"), Subtitle: l10n.AddProviderCatalogEntry(e["models"], e["base_url"]), SubtitleLines: 1,
									Tap: true, Radius: design.CornerLG, TrailingIcon: icons.ChevronRightRounded}
								if configured[e.S("id")] {
									t.TrailingIcon = 0
									t.Trailing = func() { m3.Icon(c, icons.CheckCircleRounded, 24, cs.Primary) }
								}
								if t.Show(c.Key(e.S("id"))).Clicked() {
									p.open(providerSetupFrom{entry: e})
								}
							}
						})
					}
					header(l10n.AddProviderNotFound, "")
					ui.Column(c).FillWidth().Padding(0, design.SpaceLG).Children(func() {
						ui.Box(c).FillWidth().Radius(design.CornerLG).Background(cs.SurfaceContainer).Clip().Children(func() {
							t := m3.Tile{Title: l10n.ProviderCustom, Subtitle: l10n.AddProviderCompatibleHint, Tap: true, Radius: design.CornerLG, Leading: func() {
								ui.Box(c).Size(40, 40).Radius(20).Center().Background(cs.TertiaryContainer).Children(func() {
									m3.Icon(c, icons.AddLinkRounded, 24, cs.OnTertiaryContainer)
								})
							}}
							if t.Show(c).Clicked() {
								p.open(providerSetupFrom{})
							}
						})
						m3.VGap(c, design.SpaceMD)
						text := l10n.AddProviderSearchesMore(moreCount)
						switch {
						case p.catalogError != "":
							text = l10n.AddProviderCatalogError(p.catalogError)
						case !p.loaded:
							text = l10n.AddProviderCatalogLoading
						}
						m3.Text(c, text, tt.BodySmall.C(cs.OnSurfaceVariant))
						m3.VGap(c, design.SpaceHuge)
					})
				})
			})
		})
	}}.Show(c)
}

func presetTile(c *ui.Context, p ProviderPreset, added bool) bool {
	cs := design.S()
	tt := design.TT()
	e := ui.ButtonBase(c).FocusRing(true).Cursor(ui.CursorPointer).Column().FillWidth().Height(132).Padding(design.SpaceMD)
	hover := e.Hovered()
	bg, r := cs.SurfaceContainer, float32(design.CornerLG)
	if hover {
		bg, r = cs.SurfaceContainerHigh, design.CornerXL
	}
	motion := design.T().Motion(design.Medium1, design.Emphasized)
	e = e.Background(bg).Radius(e.AnimateTo("preset.r", r, motion)).Transition(ui.ElementTransition{Duration: design.Medium1, Ease: design.Emphasized, Colors: true})
	m3.InkWell(e, design.CornerLG)
	e.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Start).Children(func() {
			sc := e.AnimateTo("preset.s", map[bool]float32{true: 1.08, false: 1}[hover], design.T().Motion(design.Medium2, design.Emphasized))
			ProviderLogo{ProviderID: p.ProviderID(), Size: 44}.Show(c).Scale(sc, sc)
			ui.Box(c).Grow(1)
			if added {
				m3.Icon(c, icons.CheckCircleRounded, 20, cs.Primary)
			} else {
				label := l10n.AddProviderCompatible
				switch {
				case p.Builtin():
					label = l10n.CommonBuiltIn
				case p.Region == RegionLocal:
					label = l10n.CommonLocal
				}
				m3.Text(c, label, tt.LabelSmall.C(cs.Outline))
			}
		})
		ui.Box(c).Grow(1)
		m3.Clamp(m3.Text(c, p.Name, tt.TitleSmall), 1)
		blurb := p.Blurb
		if blurb == "" {
			blurb = p.BaseURL
			if p.Builtin() {
				blurb = l10n.AddProviderBuiltInKeyOnly
			}
		}
		m3.Clamp(m3.Text(c, blurb, tt.BodySmall.C(cs.OnSurfaceVariant)), 1)
	})
	return e.Clicked()
}
