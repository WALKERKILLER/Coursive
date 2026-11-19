package app

import (
	"math"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// SourceMarket is source_market.dart's 插件 → 发现 → 来源插件: the source
// plugins in the plugin index, each with install, sign-in and uninstall.
// The daemon owns these installs, so the section works without the
// assistant.
// Contract: docs/specs/0025-plugin-marketplace.md
type SourceMarket struct {
	listings []core.J
	loaded   bool
	// notice is why the daemon shows the last verified index, or none: "",
	// "unverified" or "stale".
	notice  string
	err     error
	loading bool
	busy    map[string]bool
}

func (m *SourceMarket) load() {
	if m.loading {
		return
	}
	m.loading = true
	api := State.Api()
	core.Async(func() (any, error) { return api.Get("/api/sources/market") }, func(v any, err error) {
		m.loading = false
		if err != nil {
			m.err = err
			return
		}
		r := core.Obj(v)
		m.listings = nil
		for _, p := range core.List(r["plugins"]) {
			m.listings = append(m.listings, core.Obj(p))
		}
		m.loaded, m.notice, m.err = true, r.S("notice"), nil
	})
}

// Show builds the section for the 发现 search and category filter.
func (m *SourceMarket) Show(c *ui.Context, query, category string) {
	cs := design.S()
	tt := design.TT()
	// The daemon may still be starting; load once it answers.
	if !State.Connected() {
		return
	}
	if !m.loaded && m.err == nil && !m.loading {
		m.load()
	}
	if category != "" && category != "source" {
		return
	}
	q := strings.ToLower(strings.TrimSpace(query))
	var shown []core.J
	for _, p := range m.listings {
		match := q == ""
		for _, k := range []string{"title", "summary", "id", "author"} {
			match = match || strings.Contains(strings.ToLower(p.S(k)), q)
		}
		if match {
			shown = append(shown, p)
		}
	}
	if m.loaded && len(m.listings) > 0 && len(shown) == 0 {
		return
	}
	notice := ""
	if m.err == nil && m.loaded {
		switch {
		case m.notice == "unverified" && len(m.listings) == 0:
			notice = l10n.SourcesMarketUnverifiedEmpty
		case m.notice == "unverified":
			notice = l10n.SourcesMarketUnverified
		case m.notice == "stale":
			notice = l10n.SourcesMarketStale
		}
	}
	retry := func() {
		if m3.TextButton(c, l10n.CommonRetry).Clicked() {
			m.load()
		}
	}
	ui.Column(c).FillWidth().Children(func() {
		ui.Row(c).AlignItems(ui.Center).Padding(design.SpaceXL, 0, design.SpaceMD, design.SpaceXS).Children(func() {
			m3.Icon(c, icons.SchoolOutlined, 20, cs.Primary)
			m3.HGap(c, design.SpaceSM)
			m3.Text(c, l10n.SourcesMarketTitle, tt.TitleMedium)
			if len(shown) > 0 {
				m3.HGap(c, design.SpaceSM)
				m3.Text(c, itoa(len(shown)), tt.TitleMedium.C(cs.OnSurfaceVariant))
			}
		})
		if notice != "" {
			marketNote(c, icons.GppMaybeOutlined, notice, false, func() {
				if m.notice == "stale" && m3.TextButton(c, l10n.SourcesMirrorsSwitch).Clicked() {
					OpenMirrors(m.load)
				}
				retry()
			})
			m3.VGap(c, design.SpaceMD)
		}
		switch {
		case m.err != nil:
			// The index is on GitHub; another route may reach it.
			marketNote(c, icons.CloudOffRounded, l10n.SourcesMarketFailed(apiMessage(m.err)), true, func() {
				if m3.TextButton(c, l10n.SourcesMirrorsSwitch).Clicked() {
					OpenMirrors(m.load)
				}
				retry()
			})
		case !m.loaded:
			ui.Box(c).FillWidth().Padding(design.SpaceLG).Center().Children(func() { m3.Circular(c, m3.Progress{}) })
		case len(m.listings) == 0 && notice != "":
		case len(m.listings) == 0:
			marketNote(c, icons.Inventory2Outlined, l10n.SourcesMarketEmpty, false, func() {
				if m3.TextButton(c, l10n.SourcesInstall).Clicked() {
					InstallSourcePlugin()
				}
			})
		default:
			g := ui.Grid(c).FillWidth().Gap(design.SpaceMD)
			cols := min(max(int(math.Floor(float64(widgets.Width(g, 900)/300))), 1), 4)
			g.Columns(cols).Children(func() {
				for _, p := range shown {
					ui.Box(c.Key(p.S("id"))).FillWidth().Children(func() { m.card(c, p) })
				}
			})
		}
	})
}

// card is _SourceCard: 安装 when absent; once installed, its sign-in state
// with 登录 and 卸载.
func (m *SourceMarket) card(c *ui.Context, p core.J) {
	cs := design.S()
	tt := design.TT()
	if m.busy == nil {
		m.busy = map[string]bool{}
	}
	id, version, official := p.S("id"), p.S("version"), p["official"] == true
	busy := m.busy[id]
	run := func(f func(done func(bool))) {
		m.busy[id] = true
		f(func(bool) { delete(m.busy, id) })
	}
	// Busy covers only the download and its verification; consent and
	// sign-in are sheets of their own.
	install := func() {
		m.busy[id] = true
		InstallListedSource(id, func() { delete(m.busy, id) }, nil)
	}
	var installed *core.SourcePlugin
	for _, x := range State.Plugins {
		if x.ID() == id {
			installed = &x
			break
		}
	}
	var source *core.Source
	if installed != nil {
		if s := installed.Sources(); len(s) > 0 {
			source = &s[0]
		}
	}
	icon := p.S("icon")
	if icon == "" {
		icon = "school"
	}
	if installed != nil && installed.Icon() != "" {
		icon = installed.Icon()
	}
	available := p["available"] == true
	update := installed != nil && available && installed.Version() != version && version != ""
	var status string
	statusColor := cs.OnSurfaceVariant
	signedIn := source != nil && source.SignedIn()
	switch {
	case installed == nil && available:
		status = l10n.SourcesVersion(version)
	case installed == nil:
		status = l10n.SourcesMarketUnsupported
	case !installed.Enabled():
		status = l10n.SourcesDisabled
	case signedIn:
		status, statusColor = l10n.SourceSignedInPlain, cs.Primary
		if source.Account() != "" {
			status = l10n.SourceSignedInAs(source.Account())
		}
	case source != nil && source.Expired():
		status, statusColor = l10n.SourceExpired, cs.Error
	default:
		status = l10n.SourceSignedOut
	}
	ui.Column(c).FillWidth().Height(148 + 2*design.SpaceLG).Padding(design.SpaceLG).Radius(design.CornerXL).Clip().Background(cs.SurfaceContainerLow).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			widgets.PluginGlyph{Icon: icon, ID: id, Size: 48}.Show(c)
			m3.HGap(c, design.SpaceMD)
			ui.Column(c).Grow(1).Basis(0).Children(func() {
				m3.Clamp(m3.Text(c, p.S("title"), tt.TitleMedium), 1)
				m3.VGap(c, design.SpaceXXS)
				ui.Row(c).AlignItems(ui.Center).Children(func() {
					by := p.S("author")
					if official {
						m3.Icon(c, icons.VerifiedRounded, 14, cs.Primary)
						m3.HGap(c, design.SpaceXXS)
						by = l10n.PluginsOfficial
					}
					m3.Clamp(m3.Text(c, by, tt.LabelMedium.C(cs.OnSurfaceVariant)), 1).Shrink(1)
				})
			})
		})
		m3.VGap(c, design.SpaceMD)
		ui.Box(c).FillWidth().Grow(1).Children(func() {
			m3.Clamp(m3.Text(c, p.S("summary"), tt.BodyMedium.C(cs.OnSurfaceVariant)), 2)
		})
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			if installed != nil {
				ic := icons.RadioButtonUncheckedRounded
				if signedIn {
					ic = icons.CheckCircleRounded
				}
				m3.Icon(c, ic, 16, statusColor)
				m3.HGap(c, design.SpaceXS)
			}
			m3.Clamp(m3.Text(c, status, tt.LabelLarge.C(statusColor)), 1).Grow(1).Basis(0)
			btn := func(b m3.Button) bool {
				m3.HGap(c, design.SpaceXS)
				b.Height = 32
				return b.Show(c).Clicked() && !b.Disabled
			}
			if installed == nil {
				b := m3.Button{Kind: m3.Tonal, Icon: icons.DownloadRounded, Label: l10n.CommonInstall, Disabled: !available || busy}
				if busy {
					b.Label = l10n.SourcesMarketVerifying
					b.Icon = 0
				}
				if btn(b) {
					install()
				}
				return
			}
			plugin := *installed
			if btn(m3.Button{Kind: m3.TextKind, Label: l10n.CommonUninstall, Disabled: busy}) {
				run(func(done func(bool)) { RemoveSourcePlugin(plugin, done) })
			}
			switch {
			case update:
				if btn(m3.Button{Kind: m3.Filled, Label: l10n.SourcesMarketUpdate(version), Disabled: busy}) {
					install()
				}
			case source != nil && installed.Enabled() && !signedIn && len(source.Fields()) > 0:
				label := l10n.LoginSignIn
				if source.Expired() {
					label = l10n.LoginSignInAgain
				}
				if btn(m3.Button{Kind: m3.Tonal, Label: label, Disabled: busy}) {
					SignInSource(*source)
				}
			}
		})
	})
}

func marketNote(c *ui.Context, ic icons.Icon, text string, isErr bool, action func()) {
	cs := design.S()
	bg, fg := cs.SurfaceContainer, cs.OnSurfaceVariant
	if isErr {
		bg, fg = cs.ErrorContainer, cs.OnErrorContainer
	}
	ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(design.SpaceSM, design.SpaceSM, design.SpaceSM, design.SpaceLG).Radius(design.CornerLG).Background(bg).Children(func() {
		m3.Icon(c, ic, 20, fg)
		m3.HGap(c, design.SpaceMD)
		m3.Text(c, text, design.TT().BodyMedium.C(fg)).Grow(1).Basis(0)
		m3.HGap(c, design.SpaceSM)
		action()
	})
}
