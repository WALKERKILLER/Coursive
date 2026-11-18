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

// The 插件 tab: discover, install and manage plugins
// (docs/specs/0004-plugin-platform.md).

func pluginSurfaceLabel(s string) string {
	switch s {
	case "note.toolbar":
		return l10n.PluginSlotNoteToolbar
	case "lecture.menu":
		return l10n.PluginSlotLectureMenu
	case "course.menu":
		return l10n.PluginSlotCourseMenu
	case "schedule.item":
		return l10n.PluginSlotScheduleItem
	case "assignment.actions":
		return l10n.PluginSlotAssignment
	case "composer.plus":
		return l10n.PluginSlotComposer
	}
	return s
}

func pluginScopeLabel(s string) string {
	switch s {
	case "note":
		return l10n.CommonNotes
	case "course":
		return l10n.CommonCourses
	case "library":
		return l10n.CommonLibrary
	case "schedule":
		return l10n.CommonSchedule
	}
	return ""
}

var permissionIcons = map[string]icons.Icon{
	"library.read":  icons.LocalLibraryOutlined,
	"library.write": icons.EditNoteRounded,
	"views":         icons.DashboardCustomizeOutlined,
	"localhost":     icons.LanOutlined,
	"exec":          icons.TerminalRounded,
	"workspace":     icons.CreateNewFolderOutlined,
	"network":       icons.PublicRounded,
	"folder":        icons.FolderOpenOutlined,
}

func permissionIcon(id string) icons.Icon {
	if ic, ok := permissionIcons[id]; ok {
		return ic
	}
	return icons.KeyOutlined
}

// pluginsView is the view of the 插件 tab: 0 发现, 1 已安装. Links
// elsewhere set it before switching to the tab.
var pluginsView int

// OpenPlugins shows the 插件 tab, closing what covers the window first.
func OpenPlugins(installed bool) {
	m3.CloseAll()
	m3.RootNav.PopUntilRoot()
	pluginsView = 0
	if installed {
		pluginsView = 1
	}
	GoTo(TabPlugins)
}

func pluginNeedsAttention(p core.J) bool {
	return p.S("state") == "needs_consent" || p.S("state") == "error"
}

// setThirdParty turns third-party plugins on (after a warning) or off.
func setThirdParty(on bool, then func()) {
	apply := func() {
		core.Async(func() (bool, error) { return Plugins.SetThirdParty(on) }, func(allow bool, err error) {
			if err != nil {
				widgets.ShowSnack(apiMessage(err), "", nil)
			} else {
				Plugins.AllowThirdParty = allow
			}
			if then != nil {
				then()
			}
		})
	}
	if !on {
		apply()
		return
	}
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		m3.AlertDialog(c, m3.Alert{Icon: icons.ShieldOutlined, Title: l10n.PluginsAllowThirdPartyTitle, Message: l10n.PluginsAllowThirdPartyBody, Actions: func() {
			if m3.TextButton(c, l10n.CommonCancel).Clicked() {
				r.Pop(false)
			}
			if m3.FilledButton(c, l10n.CommonAllow).Clicked() {
				r.Pop(true)
			}
		}})
	}, func(v any) {
		if v == true {
			apply()
		}
	})
}

// PluginSettingsSection is the 插件 section in 设置: shortcuts into the
// 插件 tab.
func PluginSettingsSection(c *ui.Context) {
	cs := design.S()
	on := len(Plugins.Active())
	attention := Plugins.Attention()
	sub := l10n.PluginsAfterAssistant
	if Plugins.Ready() {
		if len(Plugins.Installed) == 0 {
			sub = l10n.PluginsNoneInstalled
		} else {
			suffix := ""
			if attention > 0 {
				suffix = l10n.PluginsAttentionSuffix(attention)
			}
			sub = l10n.PluginsEnabledSummary(on, suffix)
		}
	}
	ui.Column(c).FillWidth().Children(func() {
		t := m3.Tile{LeadingIcon: icons.ExtensionOutlined, Title: l10n.PluginsInstalled, Subtitle: sub, Tap: true, Disabled: !Plugins.Ready(), Pad: true}
		if attention > 0 {
			t.Trailing = func() { m3.Badge(c, itoa(attention)) }
		} else {
			t.TrailingIcon = icons.ChevronRightRounded
		}
		if t.Show(c.Key("installed")).Clicked() {
			OpenPlugins(true)
		}
		if (m3.Tile{LeadingIcon: icons.ExploreOutlined, Title: l10n.PluginsDiscover, Subtitle: l10n.PluginsTagline, TrailingIcon: icons.ChevronRightRounded,
			Tap: true, Disabled: !Plugins.Ready(), Pad: true}).Show(c.Key("discover")).Clicked() {
			OpenPlugins(false)
		}
	})
	_ = cs
}

// PluginsPage is the 插件 tab. 发现 browses the catalogs and opens a
// plugin in a side pane; 已安装 lists installed plugins beside the
// selected one's detail. Narrow windows push the detail instead.
type PluginsPage struct {
	catalog    core.J
	catalogErr error
	fetching   bool
	query      string
	category   string
	// picked is the plugin in 发现's side pane; shown is the pane's
	// content, kept while it closes.
	picked string
	shown  core.J
	// selected is the plugin beside 已安装's list.
	selected string

	market   SourceMarket
	scroll   ui.ScrollState
	side     ui.ScrollState
	featured ui.ScrollState
	details  map[string]*PluginDetailView
}

// NewPluginsPage builds the 插件 tab.
func NewPluginsPage() m3.Page {
	p := &PluginsPage{details: map[string]*PluginDetailView{}}
	if Plugins.Ready() {
		Plugins.Load(nil)
		p.loadCatalog()
	}
	return p
}

// MainScroll is the page's scroll.
func (p *PluginsPage) MainScroll() *ui.ScrollState { return &p.scroll }

func (p *PluginsPage) loadCatalog() {
	if p.fetching {
		return
	}
	p.fetching = true
	core.Async(Plugins.Catalog, func(r core.J, err error) {
		p.fetching = false
		if err != nil {
			p.catalogErr = err
			return
		}
		p.catalog, p.catalogErr = r, nil
		if p.shown != nil {
			if l := p.listing(p.shown.S("id")); l != nil {
				p.shown = l
			}
		}
	})
}

func (p *PluginsPage) refresh() {
	Plugins.Load(nil)
	p.loadCatalog()
}

func (p *PluginsPage) listings() []core.J { return p.catalog.Os("plugins") }

func (p *PluginsPage) listing(id string) core.J {
	for _, l := range p.listings() {
		if l.S("id") == id {
			return l
		}
	}
	return nil
}

func (p *PluginsPage) matches(l core.J) bool {
	if p.category != "" && !containsStr(l.Ss("categories"), p.category) {
		return false
	}
	q := strings.ToLower(strings.TrimSpace(p.query))
	if q == "" {
		return true
	}
	for _, k := range []string{"title", "summary", "description", "id", "author"} {
		if strings.Contains(strings.ToLower(l.S(k)), q) {
			return true
		}
	}
	return false
}

func containsStr(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func (p *PluginsPage) openDetail(entry core.J) {
	m3.Nav().PushThen(newPluginDetailPage(entry, p.loadCatalog), p.loadCatalog)
}

func (p *PluginsPage) install(l core.J) {
	InstallPlugin(l.S("source"), func(ok bool) {
		if ok {
			p.loadCatalog()
		}
	})
}

// detail is the kept detail view of entry, so its busy state and settings
// form survive rebuilding.
func (p *PluginsPage) detail(entry core.J, embedded bool, onClose func()) *PluginDetailView {
	id := entry.S("id")
	d, ok := p.details[id]
	if !ok {
		d = &PluginDetailView{}
		p.details[id] = d
	}
	d.Entry, d.Embedded, d.OnClose, d.OnChanged = entry, embedded, onClose, p.loadCatalog
	return d
}

// View builds the page.
func (p *PluginsPage) View(c *ui.Context) {
	if Plugins.Ready() && p.catalog == nil && p.catalogErr == nil && !p.fetching {
		p.loadCatalog()
	}
	// The page's own width, as LayoutBuilder gives it, not the window's:
	// the docked assistant takes part of the window.
	root := ui.Column(c).Fill().Background(design.S().Surface)
	w := widgets.Width(root, WindowWidth)
	compact := w < 600
	wide := w >= 880
	pad := float32(design.SpaceXL)
	if compact {
		pad = design.SpaceLG
	}
	view := pluginsView
	root.Children(func() {
		p.header(c, pad, compact)
		ui.Column(c).FillWidth().Grow(1).Basis(0).Key(view).Transition(ui.ElementTransition{Duration: design.Medium2,
			Ease: design.EmphasizedDecelerate, Enter: &ui.Motion{Opacity: 0}}).Children(func() {
			switch {
			case !Plugins.Ready() && view == 0:
				// Source plugins run in the daemon, so they install without
				// the assistant.
				ui.Scroll(c).Fill().TrackScroll(&p.scroll).Children(func() {
					ui.Column(c).FillWidth().Padding(0, pad, design.SpaceHuge+BottomInset, pad).Children(func() {
						widgets.ContentWidth(c, design.MaxContentWidth+200, func() {
							p.market.Show(c, "", "")
							m3.VGap(c, design.SpaceXXL)
							widgets.EmptyState{Icon: icons.ExtensionOutlined, Title: l10n.PluginsAfterAssistant, Body: l10n.PluginsNeedAssistant}.Show(c)
						})
					})
				})
			case !Plugins.Ready():
				widgets.EmptyState{Icon: icons.ExtensionOutlined, Title: l10n.PluginsAfterAssistant, Body: l10n.PluginsNeedAssistant}.Show(c)
			case view == 0:
				p.discover(c, pad, w, wide)
			default:
				p.installed(c, pad, w, wide)
			}
		})
	})
}

func (p *PluginsPage) header(c *ui.Context, pad float32, compact bool) {
	cs := design.S()
	tt := design.TT()
	n := len(Plugins.Installed)
	on := len(Plugins.Active())
	attention := Plugins.Attention()
	updates := 0
	for _, l := range p.listings() {
		if l.IsTrue("update") {
			updates++
		}
	}
	sub := l10n.PluginsSubtitle
	if n > 0 {
		parts := []string{l10n.PluginsInstalledCount(n)}
		if on == n {
			parts = append(parts, l10n.CommonAllEnabled)
		} else {
			parts = append(parts, l10n.PluginsEnabledCount(on))
		}
		if attention > 0 {
			parts = append(parts, l10n.PluginsAttentionCount(attention))
		}
		if updates > 0 {
			parts = append(parts, l10n.PluginsUpdatesCount(updates))
		}
		sub = strings.Join(parts, " · ")
	}
	top := float32(design.SpaceXXL)
	title := tt.DisplaySmall
	if compact {
		top, title = design.SpaceXL, tt.HeadlineMedium
	}
	widgets.ContentWidth(c, design.MaxContentWidth+200, func() {
		ui.Column(c).FillWidth().Padding(top, pad, design.SpaceLG, pad).Children(func() {
			m3.Text(c, l10n.CommonPlugins, title)
			m3.VGap(c, design.SpaceXS)
			m3.Text(c, sub, tt.BodyLarge.C(cs.OnSurfaceVariant))
			m3.VGap(c, design.SpaceLG)
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				label := l10n.CommonInstalled
				if n > 0 {
					label = l10n.PluginsInstalledTab(n)
				}
				m3.Segmented(c, []m3.Segment[int]{{Value: 0, Icon: icons.ExploreOutlined, Label: l10n.PluginsDiscoverTab},
					{Value: 1, Icon: icons.ExtensionOutlined, Label: label, Badge: attention > 0}}, &pluginsView, m3.SegOpts{NoCheck: true})
				ui.Box(c).Grow(1)
				p.sourcesMenu(c, compact)
			})
		})
	})
}

// sourcesMenu is where plugins come from: the third-party switch,
// catalogs and other sources, as a menu beside the view switcher.
func (p *PluginsPage) sourcesMenu(c *ui.Context, compact bool) {
	cs := design.S()
	m3.Menu(c, "sources", m3.Below, func(t ui.Element, open bool) {
		t = t.Row().AlignItems(ui.Center).Justify(ui.Center).Height(40).Radius(design.CornerFull).Background(cs.SecondaryContainer).
			Cursor(ui.CursorPointer).Label(l10n.PluginsSources)
		if compact {
			t = t.Width(40)
			if !open {
				m3.Tip(c, t, l10n.PluginsSources)
			}
		} else {
			t = t.Padding(0, 24, 0, 16).Gap(8)
		}
		m3.Ink(t, cs.OnSecondaryContainer, design.CornerFull)
		t.Children(func() {
			if compact {
				m3.Icon(c, icons.TuneRounded, 24, cs.OnSecondaryContainer)
				return
			}
			m3.Icon(c, icons.TuneRounded, 18, cs.OnSecondaryContainer)
			m3.Text(c, l10n.PluginsSourcesTab, design.TT().LabelLarge.C(cs.OnSecondaryContainer))
		})
	}, func() []m3.MenuItem {
		third := Plugins.AllowThirdParty
		ready := Plugins.Ready()
		return []m3.MenuItem{
			{Label: l10n.PluginsAllowThirdParty, Checked: &third, Disabled: !ready, OnTap: func() { setThirdParty(!third, p.refresh) }},
			{Divider: true},
			{Icon: icons.LibraryAddOutlined, IconSize: 20, Label: l10n.PluginsCatalogs, Disabled: !third, OnTap: func() { showCatalogsDialog(p.refresh) }},
			{Icon: icons.DriveFolderUploadOutlined, IconSize: 20, Label: l10n.PluginsInstallFrom, Disabled: !third, OnTap: showSourceDialog},
			{Divider: true},
			{Icon: icons.RefreshRounded, IconSize: 20, Label: l10n.CommonRefresh, Disabled: !ready, OnTap: p.refresh},
		}
	})
}

// pane is the detail of entry in a tonal pane, cross-fading between
// plugins.
func (p *PluginsPage) pane(c *ui.Context, e ui.Element, entry core.J, onClose func()) {
	e.Background(design.S().SurfaceContainerLow).Radius(design.CornerXL).Clip().Children(func() {
		ui.Column(c).Fill().Key(entry.S("id")).Transition(ui.ElementTransition{Duration: design.Medium2, Ease: design.EmphasizedDecelerate,
			Enter: &ui.Motion{Opacity: 0, X: 8}}).Children(func() {
			p.detail(entry, true, onClose).Show(c)
		})
	})
}

func (p *PluginsPage) discover(c *ui.Context, pad, width float32, wide bool) {
	cs := design.S()
	tt := design.TT()
	if p.catalog == nil {
		if p.catalogErr == nil {
			widgets.CenteredLoader(c, "")
		} else {
			widgets.ErrorState(c, p.catalogErr, p.loadCatalog)
		}
		return
	}
	all := p.listings()
	filtering := strings.TrimSpace(p.query) != "" || p.category != ""
	var shown, featured []core.J
	for _, l := range all {
		if p.matches(l) {
			shown = append(shown, l)
		}
		if !filtering && l.IsTrue("featured") {
			featured = append(featured, l)
		}
	}
	seen := map[string]bool{}
	for _, l := range all {
		for _, cat := range l.Ss("categories") {
			seen[cat] = true
		}
	}
	var cats []string
	for _, cat := range core.PluginCategories {
		if seen[cat] {
			cats = append(cats, cat)
		}
	}
	errs := p.catalog.L("errors")
	// Official plugins first, then one section per added catalog.
	var names []string
	groups := map[string][]core.J{}
	for _, l := range shown {
		name := ""
		if !l.IsTrue("official") {
			name = l.Or("catalog", l10n.PluginsOtherSource)
		}
		if _, ok := groups[name]; !ok {
			names = append(names, name)
		}
		groups[name] = append(groups[name], l)
	}
	open := func(l core.J) {
		if !wide {
			p.openDetail(l)
			return
		}
		p.picked, p.shown = l.S("id"), l
	}
	grid := func(plugins []core.J) {
		g := ui.Grid(c).FillWidth().Gap(design.SpaceMD)
		cols := min(max(int(math.Floor(float64(widgets.Width(g, 900)/300))), 1), 4)
		g.Columns(cols).Children(func() {
			for _, l := range plugins {
				l := l
				ui.Box(c.Key(l.S("id"))).FillWidth().Children(func() {
					marketCard(c, l, Plugins.ByID(l.S("id")) != nil, wide && p.picked == l.S("id"), func() { open(l) }, func() { p.install(l) })
				})
			}
		})
	}
	list := func() {
		ui.Scroll(c).Fill().TrackScroll(&p.scroll).Children(func() {
			ui.Column(c).FillWidth().Padding(0, pad, design.SpaceHuge+BottomInset, pad).Children(func() {
				m3.SearchBar{Hint: l10n.PluginsSearch, Height: 52, MaxWidth: 640, Leading: func() {
					ui.Box(c).Padding(0, 0, 0, design.SpaceSM).Children(func() { m3.Icon(c, icons.SearchRounded, 24, cs.OnSurfaceVariant) })
				}}.Show(c, &p.query)
				m3.VGap(c, design.SpaceMD)
				ui.Row(c).FillWidth().Wrap().Gap(design.SpaceSM).Children(func() {
					if m3.FilterChipOf(c.Key("all"), l10n.CommonAll, p.category == "") {
						p.category = ""
					}
					for _, cat := range cats {
						if m3.FilterChipOf(c.Key(cat), core.PluginCategory(cat), p.category == cat) {
							if p.category == cat {
								p.category = ""
							} else {
								p.category = cat
							}
						}
					}
				})
				if len(errs) > 0 {
					m3.VGap(c, design.SpaceLG)
					pluginBanner(c, icons.CloudOffRounded, l10n.PluginsCatalogErrors(len(errs)), l10n.CommonView, func() { showCatalogsDialog(p.loadCatalog) }, true)
				}
				if len(featured) > 0 {
					m3.VGap(c, design.SpaceXL)
					p.featuredRow(c, featured, open)
				}
				p.market.Show(c, p.query, p.category)
				if len(shown) == 0 {
					ui.Column(c).FillWidth().Padding(design.SpaceXXL, 0, 0, 0).Children(func() {
						widgets.EmptyState{Icon: icons.SearchOffRounded, Title: l10n.PluginsNoMatch, Body: l10n.PluginsNoMatchBody}.Show(c)
					})
				}
				for _, name := range names {
					name := name
					ui.Column(c.Key("g:" + name)).FillWidth().Children(func() {
						ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(design.SpaceXL, 0, design.SpaceMD, design.SpaceXS).Children(func() {
							ic, col, title := icons.PublicRounded, cs.OnSurfaceVariant, name
							if name == "" {
								ic, col, title = icons.VerifiedRounded, cs.Primary, l10n.PluginsOfficial
							}
							m3.Icon(c, ic, 20, col)
							m3.HGap(c, design.SpaceSM)
							m3.Clamp(m3.Text(c, title, tt.TitleMedium), 1).Shrink(1)
							m3.HGap(c, design.SpaceSM)
							m3.Text(c, itoa(len(groups[name])), tt.TitleMedium.C(cs.OnSurfaceVariant))
						})
						grid(groups[name])
					})
				}
				if !Plugins.AllowThirdParty && !filtering {
					m3.VGap(c, design.SpaceXXL)
					pluginBanner(c, icons.AddLinkRounded, l10n.PluginsMoreHint, l10n.CommonAllow, func() { setThirdParty(true, p.loadCatalog) }, false)
				}
			})
		})
	}
	if !wide {
		widgets.ContentWidth(c, design.MaxContentWidth+200, list)
		return
	}
	paneW := float32(380)
	if width >= 1280 {
		paneW = 440
	}
	ui.Column(c).Fill().AlignItems(ui.Center).Children(func() {
		ui.Row(c).FillWidth().MaxWidth(design.MaxContentWidth + 200).Grow(1).Basis(0).AlignItems(ui.Stretch).Children(func() {
			ui.Column(c).Grow(1).Basis(0).FillHeight().Children(list)
			box := ui.Box(c).FillHeight().Shrink(0).Clip()
			target := float32(0)
			if p.picked != "" {
				target = paneW + pad
			}
			bw := box.AnimateTo("plugins.pane", target, design.T().Motion(design.Medium4, design.Emphasized))
			box = box.Width(bw)
			box.Children(func() {
				if p.shown == nil || bw < 1 {
					return
				}
				inner := ui.Column(c).Absolute().Top(0).Bottom(0).Left(0).Width(paneW+pad).Padding(0, pad, pad, 0)
				inner.Children(func() {
					p.pane(c, ui.Column(c).Fill(), p.shown, func() { p.picked = "" })
				})
			})
		})
	})
}

func (p *PluginsPage) installed(c *ui.Context, pad, width float32, wide bool) {
	cs := design.S()
	tt := design.TT()
	switch {
	case !Plugins.Loaded:
		widgets.CenteredLoader(c, "")
		return
	case Plugins.Error != nil && len(Plugins.Installed) == 0:
		widgets.ErrorState(c, Plugins.Error, func() { Plugins.Load(nil) })
		return
	case len(Plugins.Installed) == 0:
		widgets.EmptyState{Icon: icons.ExtensionOutlined, Title: l10n.PluginsNoneInstalled, Body: l10n.PluginsEmptyBody, Action: func() {
			if (m3.Button{Kind: m3.Filled, Icon: icons.ExploreOutlined, Label: l10n.PluginsGoDiscover}).Show(c).Clicked() {
				pluginsView = 0
			}
		}}.Show(c)
		return
	}
	type group struct {
		title string
		items []core.J
	}
	groups := []group{{title: l10n.CommonNeedsAttention}, {title: l10n.CommonEnabled}, {title: l10n.CommonDisabled}}
	for _, pl := range Plugins.Installed {
		switch {
		case pluginNeedsAttention(pl):
			groups[0].items = append(groups[0].items, pl)
		case pl.S("state") == "enabled":
			groups[1].items = append(groups[1].items, pl)
		case pl.S("state") == "disabled":
			groups[2].items = append(groups[2].items, pl)
		}
	}
	var flat []core.J
	for _, g := range groups {
		flat = append(flat, g.items...)
	}
	if len(flat) == 0 {
		flat = Plugins.Installed
	}
	sel := flat[0]
	for _, pl := range flat {
		if pl.S("id") == p.selected {
			sel = pl
		}
	}
	toggle := func(pl core.J, v bool) {
		id := pl.S("id")
		core.Async(func() (core.J, error) { return Plugins.Update(id, core.J{"enabled": v}) }, func(r core.J, err error) {
			if err != nil {
				widgets.ShowSnack(apiMessage(err), "", nil)
				return
			}
			Plugins.Replace(r)
		})
	}
	rows := func(cards bool) {
		PluginPageLauncher(c)
		for gi, g := range groups {
			if len(g.items) == 0 {
				continue
			}
			ui.Column(c.Key(gi)).FillWidth().Children(func() {
				m3.Text(c, g.title+" · "+itoa(len(g.items)), tt.LabelLarge.C(cs.OnSurfaceVariant)).Margin(design.SpaceMD, 0, design.SpaceSM, design.SpaceMD)
				for _, pl := range g.items {
					pl := pl
					gap := float32(design.SpaceXS)
					if cards {
						gap = design.SpaceSM
					}
					ui.Box(c.Key(pl.S("id"))).FillWidth().Margin(0, 0, gap, 0).Children(func() {
						l := p.listing(pl.S("id"))
						var onToggle func(bool)
						if cards && pl.S("state") != "needs_consent" {
							onToggle = func(v bool) { toggle(pl, v) }
						}
						onTap := func() { p.selected = pl.S("id") }
						if cards {
							onTap = func() {
								if l != nil {
									p.openDetail(l)
								} else {
									p.openDetail(pl)
								}
							}
						}
						installedRow(c, pl, cards, l.IsTrue("update"), !cards && pl.S("id") == sel.S("id"), onTap, onToggle)
					})
				}
			})
		}
	}
	if !wide {
		ui.Scroll(c).Fill().TrackScroll(&p.scroll).Children(func() {
			ui.Column(c).FillWidth().Padding(0, pad, 120+BottomInset, pad).Children(func() {
				widgets.ContentWidth(c, 760, func() { rows(true) })
			})
		})
		return
	}
	listW := float32(320)
	if width >= 1200 {
		listW = 380
	}
	entry := p.listing(sel.S("id"))
	if entry == nil {
		entry = sel
	}
	ui.Column(c).Fill().AlignItems(ui.Center).Children(func() {
		ui.Row(c).FillWidth().MaxWidth(design.MaxContentWidth+200).Grow(1).Basis(0).AlignItems(ui.Stretch).Padding(0, pad, pad, pad-design.SpaceSM).Children(func() {
			ui.Scroll(c).Width(listW).FillHeight().TrackScroll(&p.side).Children(func() {
				ui.Column(c).FillWidth().Padding(0, 0, design.SpaceXL, 0).Children(func() { rows(false) })
			})
			m3.HGap(c, design.SpaceMD)
			p.pane(c, ui.Column(c).Grow(1).Basis(0).FillHeight(), entry, nil)
		})
	})
}

// pluginBanner is a one-line note with an action, in a tonal container.
func pluginBanner(c *ui.Context, ic icons.Icon, text, action string, onAction func(), isErr bool) {
	cs := design.S()
	bg, fg := cs.SurfaceContainer, cs.OnSurfaceVariant
	if isErr {
		bg, fg = cs.ErrorContainer, cs.OnErrorContainer
	}
	ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(design.SpaceSM, design.SpaceSM, design.SpaceSM, design.SpaceLG).Background(bg).Radius(design.CornerLG).Children(func() {
		m3.Icon(c, ic, 20, fg)
		m3.HGap(c, design.SpaceMD)
		m3.Text(c, text, design.TT().BodyMedium.C(fg)).Grow(1).Basis(0)
		m3.HGap(c, design.SpaceSM)
		if m3.TextButton(c, action).Clicked() {
			onAction()
		}
	})
}

// pluginPill is 已启用 / 需要授权 / 出错 / 已停用 / 可更新 as a small pill.
func pluginPill(c *ui.Context, pl core.J, update bool) {
	cs := design.S()
	label, bg, fg := l10n.CommonEnabled, cs.PrimaryContainer, cs.OnPrimaryContainer
	switch st := pl.S("state"); {
	case st == "needs_consent":
		label, bg, fg = l10n.PluginsNeedsGrant, cs.TertiaryContainer, cs.OnTertiaryContainer
	case st == "error":
		label, bg, fg = l10n.PluginsErrored, cs.ErrorContainer, cs.OnErrorContainer
	case st == "disabled":
		label, bg, fg = l10n.CommonDisabled, cs.SurfaceContainerHighest, cs.OnSurfaceVariant
	case update:
		label, bg, fg = l10n.PluginsUpdatable, cs.Primary, cs.OnPrimary
	}
	ui.Row(c).AlignItems(ui.Center).Padding(design.SpaceXXS, design.SpaceSM).Background(bg).Radius(design.CornerFull).Shrink(0).Children(func() {
		ui.Box(c).Size(6, 6).Radius(3).Background(fg)
		m3.HGap(c, design.SpaceXS)
		m3.Text(c, label, design.TT().LabelMedium.C(fg))
	})
}

// installedRow is an installed plugin: a card with a switch on narrow
// windows, a selectable row beside the detail pane otherwise.
func installedRow(c *ui.Context, pl core.J, card, update, selected bool, onTap func(), onToggle func(bool)) {
	cs := design.S()
	tt := design.TT()
	state := pl.S("state")
	isErr := state == "error"
	bg, radius := ui.Transparent, float32(design.CornerLG)
	if card {
		bg, radius = cs.SurfaceContainerLow, design.CornerXL
	}
	if selected {
		bg = cs.SecondaryContainer
	}
	t := m3.Tappable(c, radius).FillWidth().Background(bg).Selected(selected)
	t.Children(func() {
		right := float32(design.SpaceMD)
		if card {
			right = design.SpaceSM
		}
		ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(design.SpaceMD, right, design.SpaceMD, design.SpaceMD).Children(func() {
			widgets.PluginGlyph{Icon: pl.S("icon"), ID: pl.S("id"), Size: 44, Muted: state == "disabled"}.Show(c)
			m3.HGap(c, design.SpaceMD)
			ui.Column(c).Grow(1).Basis(0).Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
					st := tt.TitleMedium
					if state == "disabled" {
						st = st.C(cs.OnSurfaceVariant)
					}
					m3.Clamp(m3.Text(c, pl.S("title"), st), 1).Shrink(1)
					if pl.IsTrue("official") {
						m3.HGap(c, design.SpaceXS)
						m3.Icon(c, icons.VerifiedRounded, 16, cs.Primary)
					}
				})
				m3.VGap(c, design.SpaceXXS)
				text, col := pl.S("summary"), cs.OnSurfaceVariant
				if isErr {
					text, col = pl.S("error"), cs.Error
				}
				lines := 1
				if card {
					lines = 2
				}
				m3.Clamp(m3.Text(c, text, tt.BodyMedium.C(col)), lines)
				if pluginNeedsAttention(pl) || update {
					m3.VGap(c, design.SpaceXS)
					ui.Row(c).Children(func() { pluginPill(c, pl, update) })
				}
			})
			if onToggle != nil {
				on := pl.IsTrue("enabled")
				if m3.Switch(c, &on, false).Changed() {
					onToggle(on)
				}
			}
		})
	})
	if t.Clicked() {
		onTap()
	}
}

func (p *PluginsPage) featuredRow(c *ui.Context, plugins []core.J, onOpen func(core.J)) {
	cs := design.S()
	if len(plugins) == 1 {
		ui.Box(c).FillWidth().Height(220).Children(func() { heroCard(c, plugins[0], onOpen) })
		return
	}
	// CarouselView.weighted(flexWeights: [3, 1]): the first item takes
	// three quarters of the view, the next shows beside it.
	outer := ui.Box(c).FillWidth().Height(220)
	view := widgets.Width(outer, 800)
	outer.Children(func() {
		widgets.HScroller{State: &p.featured, Buttons: true}.Show(c, func() {
			ui.Row(c).Height(220).Gap(design.SpaceSM).Children(func() {
				for _, l := range plugins {
					l := l
					card := m3.Tappable(c.Key(l.S("id")), design.CornerXL).Width((view - design.SpaceSM) * .75).Height(220).Shrink(0).
						Background(cs.SurfaceContainer).Clip()
					card.Children(func() { heroBody(c, l) })
					if card.Clicked() {
						onOpen(l)
					}
				}
			})
		})
	})
}

func heroCard(c *ui.Context, l core.J, onOpen func(core.J)) {
	card := m3.Tappable(c, design.CornerXL).Fill().Clip()
	card.Children(func() { heroBody(c, l) })
	if card.Clicked() {
		onOpen(l)
	}
}

func heroBody(c *ui.Context, l core.J) {
	cs := design.S()
	tt := design.TT()
	examples := l.L("examples")
	body := ui.Row(c).Fill().AlignItems(ui.Center).Padding(design.SpaceXL).Draw(func(p *ui.Painter, r ui.Rect) {
		p.FillGradient(r, ui.LinearGradient{From: cs.PrimaryContainer, To: cs.TertiaryContainer, Angle: 135}, design.CornerXL)
	})
	b := body.Bounds()
	scale := design.T().TextScale
	body.Children(func() {
		ui.Column(c).Grow(1).Basis(0).Justify(ui.Center).Children(func() {
			m3.Text(c, l10n.PluginsFeatured, tt.LabelLarge.C(design.WithOpacity(cs.OnPrimaryContainer, .8)))
			m3.VGap(c, design.SpaceXS)
			m3.Clamp(m3.Text(c, l.S("title"), tt.HeadlineSmall.C(cs.OnPrimaryContainer)), 1)
			m3.VGap(c, design.SpaceSM)
			lines := 1
			if b.H-2*design.SpaceXL > 120*scale {
				lines = 2
			}
			m3.Clamp(m3.Text(c, l.S("summary"), tt.BodyMedium.C(cs.OnPrimaryContainer)), lines)
			if len(examples) > 0 && b.H-2*design.SpaceXL > 150*scale {
				m3.VGap(c, design.SpaceMD)
				ui.Row(c).Children(func() {
					ui.Box(c).Padding(6, design.SpaceMD).Background(design.WithOpacity(cs.Surface, .6)).Radius(design.CornerFull).Shrink(1).Children(func() {
						m3.Clamp(m3.Text(c, "“"+core.Str(examples[0])+"”", tt.LabelMedium), 1)
					})
				})
			}
		})
		if b.W > 420+2*design.SpaceXL {
			m3.HGap(c, design.SpaceXL)
			widgets.PluginGlyph{Icon: l.S("icon"), Size: 112, Color: design.WithOpacity(cs.Surface, .7), OnColor: cs.Primary}.Show(c)
		}
	})
}

func marketCard(c *ui.Context, l core.J, installed, selected bool, onTap, onInstall func()) {
	cs := design.S()
	tt := design.TT()
	update := l.IsTrue("update")
	official := l.IsTrue("official")
	var cats []string
	for _, id := range l.Ss("categories") {
		if s := core.PluginCategory(id); s != "" && len(cats) < 2 {
			cats = append(cats, s)
		}
	}
	bg := cs.SurfaceContainerLow
	if selected {
		bg = cs.SurfaceContainer
	}
	card := m3.Tappable(c, design.CornerXL).FillWidth().Background(bg).Selected(selected)
	if selected {
		card = card.Border(2, cs.Primary)
	}
	card.Children(func() {
		ui.Column(c).FillWidth().Height(148 + 2*design.SpaceLG).Padding(design.SpaceLG).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				widgets.PluginGlyph{Icon: l.S("icon"), ID: l.S("id"), Size: 48}.Show(c)
				m3.HGap(c, design.SpaceMD)
				ui.Column(c).Grow(1).Basis(0).Children(func() {
					m3.Clamp(m3.Text(c, l.S("title"), tt.TitleMedium), 1)
					m3.VGap(c, design.SpaceXXS)
					ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
						who := l.S("author")
						if who == "" {
							who = l.S("catalog")
						}
						if official {
							m3.Icon(c, icons.VerifiedRounded, 14, cs.Primary)
							m3.HGap(c, design.SpaceXXS)
							who = l10n.PluginsOfficial
						}
						m3.Clamp(m3.Text(c, who, tt.LabelMedium.C(cs.OnSurfaceVariant)), 1).Shrink(1)
					})
				})
			})
			m3.VGap(c, design.SpaceMD)
			ui.Column(c).FillWidth().Grow(1).Basis(0).Children(func() {
				m3.Clamp(m3.Text(c, l.S("summary"), tt.BodyMedium.C(cs.OnSurfaceVariant)), 2)
			})
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				for i, cat := range cats {
					ui.Box(c.Key(i)).Margin(0, design.SpaceXS, 0, 0).Padding(design.SpaceXXS, design.SpaceSM).Background(cs.SurfaceContainerHighest).Radius(design.CornerFull).Children(func() {
						m3.Text(c, cat, tt.LabelSmall.C(cs.OnSurfaceVariant))
					})
				}
				ui.Box(c).Grow(1)
				switch {
				case installed && !update:
					m3.Icon(c, icons.CheckCircleRounded, 16, cs.Primary)
					m3.HGap(c, design.SpaceXS)
					m3.Text(c, l10n.CommonInstalled, tt.LabelLarge.C(cs.Primary))
				case update:
					if (m3.Button{Kind: m3.Filled, Label: l10n.MemoryUpdated, Height: 32}).Show(c).Clicked() {
						onInstall()
					}
				default:
					if (m3.Button{Kind: m3.Tonal, Label: l10n.CommonInstall, Height: 32}).Show(c).Clicked() {
						onInstall()
					}
				}
			})
		})
	})
	if card.Clicked() {
		onTap()
	}
}
