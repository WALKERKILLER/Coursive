package app

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// settingsSection is SettingsNav.section: a category to open, set from
// elsewhere (the bot's open_page settings) and taken by the page.
var settingsSection string

// OpenSettingsSection opens a settings category from elsewhere: it pops the
// settings tab to its root, whose next build opens the category.
func OpenSettingsSection(section string) {
	settingsSection = section
	if shell != nil && shell.navs[TabSettings] != nil {
		shell.navs[TabSettings].PopUntilRoot()
	}
}

// settingsCategory is one row in the list pane and the pane it opens.
type settingsCategory struct {
	id, title string
	icon      icons.Icon
	summary   string
	// items is what the pane holds, for search.
	items []string
}

func settingsCategories() []settingsCategory {
	return []settingsCategory{
		{"account", l10n.SettingsAccount, icons.AccountCircleOutlined, l10n.SettingsAccountHint,
			[]string{l10n.SettingsProfileKeyword, l10n.SourcesTitle, l10n.LoginSignIn, l10n.SourceSignOut, l10n.LmsTitle}},
		{"appearance", l10n.SettingsAppearance, icons.PaletteOutlined, l10n.SettingsAppearanceHint,
			[]string{l10n.SettingsDarkMode, l10n.SettingsThemeColor, l10n.SettingsSystemAccent, l10n.SettingsSchemeStyle, l10n.SettingsContrast,
				l10n.SettingsFont, l10n.SettingsTextSize, l10n.SettingsCompact, l10n.SettingsReduceMotion, l10n.SettingsDefaultSpeed,
				l10n.SettingsBoostSpeed, l10n.SettingsSeekStepKeyword, l10n.SettingsSubtitleSize, l10n.SettingsResume, l10n.SettingsShowSubtitles,
				l10n.SettingsSlidesFollow, l10n.SettingsShortcuts}},
		{"assistant", l10n.SettingsAssistant, icons.AutoAwesomeOutlined, l10n.SettingsAssistantHint,
			[]string{l10n.SettingsModels, l10n.SettingsVisionModel, l10n.SettingsBotKeyword, l10n.SettingsRuntimeKeyword, l10n.SettingsUsage,
				l10n.CommonScheduledTasks, l10n.SettingsDockKeyword, l10n.SettingsBubblesKeyword, l10n.SettingsShowThinking,
				l10n.SettingsStreamAnimation, l10n.SettingsChatTextSize, l10n.CommonPlugins}},
		{"transfer", l10n.SettingsTransfer, icons.DownloadOutlined, l10n.SettingsTransferHint,
			[]string{l10n.SettingsConnectionsKeyword, l10n.SettingsVideoParallel, l10n.SettingsRateLimit, l10n.SettingsStream, l10n.AssetAsr,
				l10n.SettingsAsrChannels, l10n.SettingsLocalModels, l10n.AsrDeleteVideo, l10n.SettingsMineru}},
		{"knowledge", l10n.SettingsKnowledge, icons.LocalLibraryOutlined, l10n.SettingsKnowledgeHint,
			[]string{"Obsidian", "Notion", l10n.SettingsLibraryLocation, l10n.KnowledgeFolderTemplates, l10n.SettingsSkillExport}},
		{"data", l10n.SettingsData, icons.CloudSyncOutlined, l10n.SettingsDataHint,
			[]string{"WebDAV", l10n.WebdavJianguoyun, "Nextcloud", l10n.DataSyncTitle, l10n.CommonAutoSync, l10n.SettingsSyncMedia,
				l10n.BackupExport, l10n.BackupRestore, l10n.StorageTitle, l10n.SettingsCleanCache, l10n.StorageCompact}},
		{"notifications", l10n.CommonNotifications, icons.NotificationsOutlined, l10n.SettingsNotificationsHint,
			[]string{l10n.SettingsWatchKeyword, l10n.WatchCheckNow, l10n.SettingsRemindersKeyword}},
		{"diagnostics", l10n.SettingsDiagnostics, icons.MonitorHeartOutlined, l10n.SettingsDiagnosticsHint,
			[]string{l10n.SettingsDaemonKeyword, "ffmpeg", "Pi Agent", l10n.DiagDaemonLog, l10n.DiagAgentLog, l10n.SettingsRestartKeyword}},
		{"about", l10n.SettingsAbout, icons.InfoOutlineRounded, l10n.SettingsAboutHint,
			[]string{l10n.AboutCheckUpdates, l10n.AboutRepo, l10n.AboutFeedback, "GitHub", "OTA"}},
	}
}

// settingsAliases maps the bot's section names (agent/src/operate.mjs) to
// categories.
var settingsAliases = map[string]string{
	"account":    "account",
	"appearance": "appearance", "player": "appearance",
	"assistant": "assistant", "models": "assistant", "plugins": "assistant",
	"downloads": "transfer", "transcript": "transfer", "courseware": "transfer",
	"knowledge": "knowledge", "obsidian": "knowledge", "notion": "knowledge",
	"data": "data", "sync": "data", "backup": "data", "storage": "data",
	"notifications": "notifications",
	"diagnostics":   "diagnostics",
	"about":         "about",
}

// settingsTwoPaneWidth is the width from which the list and the open
// category sit side by side.
const settingsTwoPaneWidth = 840

// SettingsPage is the 设置 tab.
type SettingsPage struct {
	settings ServerSettings
	query    string
	selected string
	twoPane  bool
	// nav is the settings tab's navigator, where categories open.
	nav *m3.Navigator
	// pending is a deep-linked category waiting for the first layout.
	pending string

	list   ui.ScrollState
	panes  map[string]*ui.ScrollState
	asr    *AsrSettings
	mineru *MinerUSettings
	watch  *WatchSettings
	know   *KnowledgeSettings
	data   *DataSettings
	about  *AboutSection
	diag   *Diagnostics
}

// NewSettingsPage builds the 设置 tab.
func NewSettingsPage() m3.Page {
	p := &SettingsPage{selected: "account", twoPane: true, panes: map[string]*ui.ScrollState{}}
	p.settings.Load()
	return p
}

// MainScroll is the list's scroll.
func (p *SettingsPage) MainScroll() *ui.ScrollState { return &p.list }

func (p *SettingsPage) category(id string) settingsCategory {
	for _, c := range settingsCategories() {
		if c.id == id {
			return c
		}
	}
	return settingsCategories()[0]
}

// resetPanes drops the panes' state, as Flutter rebuilds a pane's
// widgets when another category opens, so each reloads when shown again.
func (p *SettingsPage) resetPanes() {
	if p.know != nil {
		p.know.Dispose()
	}
	if p.data != nil {
		p.data.Dispose()
	}
	p.asr, p.mineru, p.watch, p.know, p.data, p.about, p.diag = nil, nil, nil, nil, nil, nil, nil
	p.settings.Load()
}

func (p *SettingsPage) open(id string) {
	if id != p.selected || !p.twoPane {
		p.resetPanes()
	}
	p.selected = id
	if p.twoPane {
		return
	}
	cat := p.category(id)
	var page m3.Page
	page = m3.PageFunc(func(c *ui.Context) {
		m3.Scaffold{AppBar: &m3.AppBar{Title: cat.title}, Body: func() { p.pane(c, cat, false) }}.Show(c)
	})
	p.nav.Push(page)
}

// View builds the page.
func (p *SettingsPage) View(c *ui.Context) {
	ReportScreen(map[string]string{"screen": l10n.CommonSettings})
	p.nav = m3.Nav()
	if s := settingsSection; s != "" {
		settingsSection = ""
		if id, ok := settingsAliases[s]; ok {
			p.query, p.pending = "", id
		}
	}
	root := ui.Row(c).Fill().AlignItems(ui.Stretch)
	w := widgets.Width(root, WindowWidth)
	p.twoPane = w >= settingsTwoPaneWidth
	// A deep link opens its category once the page is laid out, so that it
	// knows whether to push it or show it beside the list.
	if id := p.pending; id != "" {
		if root.Bounds().W > 0 {
			p.pending = ""
			Post(func() { p.open(id) })
		} else {
			c.Invalidate()
		}
	}
	root.Children(func() {
		if !p.twoPane {
			ui.Column(c).Grow(1).Basis(0).FillHeight().Children(func() { p.listPane(c) })
			return
		}
		lw := float32(320)
		if w >= 1200 {
			lw = 360
		}
		ui.Column(c).Width(lw).FillHeight().Shrink(0).Children(func() { p.listPane(c) })
		cat := p.category(p.selected)
		d := design.Medium2
		if core.UI.ReduceMotion {
			d = 0
		}
		ui.Column(c).Grow(1).Basis(0).FillHeight().Key(cat.id).Transition(ui.ElementTransition{Duration: d, Ease: design.EmphasizedDecelerate,
			Enter: &ui.Motion{Opacity: 0, Y: 12}}).Children(func() { p.pane(c, cat, true) })
	})
}

// listPane is the categories, or the search results.
func (p *SettingsPage) listPane(c *ui.Context) {
	compact := Compact()
	cs := design.S()
	tt := design.TT()
	q := strings.ToLower(strings.TrimSpace(p.query))
	pad := float32(design.SpaceXL)
	if compact {
		pad = design.SpaceLG
	}
	left, right := pad, pad
	if p.twoPane {
		left, right = design.SpaceLG, design.SpaceSM
	}
	top := float32(design.SpaceXXL)
	title := tt.HeadlineLarge
	if compact {
		top, title = design.SpaceXL, tt.HeadlineMedium
	}
	ui.Scroll(c).Fill().TrackScroll(&p.list).Children(func() {
		ui.Column(c).FillWidth().Padding(top, right, design.SpaceHuge+BottomInset, left).Children(func() {
			widgets.ContentWidth(c, 720, func() {
				m3.Text(c, l10n.CommonSettings, title).Margin(0, 0, 0, design.SpaceSM)
				m3.VGap(c, design.SpaceLG)
				m3.SearchBar{Hint: l10n.SettingsSearch, MaxWidth: -1, Leading: func() { m3.Icon(c, icons.SearchRounded, 24, cs.OnSurfaceVariant) },
					Trailing: func() {
						if q != "" && m3.IconBtn(c, icons.CloseRounded, l10n.CommonClear).Clicked() {
							p.query = ""
						}
					}}.Show(c, &p.query)
				m3.VGap(c, design.SpaceLG)
				if q == "" {
					for _, cat := range settingsCategories() {
						cat := cat
						if p.categoryTile(c, cat, p.twoPane && cat.id == p.selected) {
							p.open(cat.id)
						}
					}
					return
				}
				p.results(c, q)
			})
		})
	})
}

func (p *SettingsPage) results(c *ui.Context, q string) {
	cs := design.S()
	type hit struct {
		cat  settingsCategory
		item string
	}
	var hits []hit
	for _, cat := range settingsCategories() {
		for _, item := range append([]string{cat.title}, cat.items...) {
			if strings.Contains(strings.ToLower(item), q) {
				hits = append(hits, hit{cat, item})
			}
		}
	}
	if len(hits) == 0 {
		m3.Text(c, l10n.SettingsNoMatch(strings.TrimSpace(p.query)), design.TT().BodyMedium.C(cs.OnSurfaceVariant)).Margin(design.SpaceXL, design.SpaceLG)
		return
	}
	for i, h := range hits {
		h := h
		t := m3.Tile{LeadingIcon: h.cat.icon, IconColor: cs.OnSurfaceVariant, Title: h.item, Tap: true, Radius: design.CornerFull}
		if h.item != h.cat.title {
			t.Subtitle = h.cat.title
		}
		if t.Show(c.Key(i)).Clicked() {
			p.open(h.cat.id)
		}
	}
}

// categoryTile is a category row; the account row shows who is signed
// in.
func (p *SettingsPage) categoryTile(c *ui.Context, cat settingsCategory, selected bool) bool {
	cs := design.S()
	account := cat.id == "account"
	title := cat.title
	if account {
		title = UserDisplayName()
	}
	t := m3.Tile{Title: title, TitleLines: 1, Subtitle: cat.summary, SubtitleLines: 1, Tap: true, Selected: selected,
		Radius: design.CornerFull, Pad: true, PadX: design.SpaceMD, PadY: design.SpaceXXS}
	if selected {
		t.TileColor, t.TextColor = cs.SecondaryContainer, cs.OnSecondaryContainer
	}
	t.Leading = func() {
		if account {
			UserAvatar(c, 40)
			return
		}
		bg, fg := cs.SurfaceContainerHigh, cs.OnSurfaceVariant
		if selected {
			bg, fg = design.WithOpacity(cs.Surface, .6), cs.OnSecondaryContainer
		}
		ui.Box(c).Size(40, 40).Center().Radius(design.CornerMD).Background(bg).
			Transition(ui.ElementTransition{Duration: design.Medium1, Ease: design.Emphasized, Colors: true}).Children(func() {
			m3.Icon(c, cat.icon, 22, fg)
		})
	}
	var clicked bool
	ui.Box(c.Key(cat.id)).FillWidth().Margin(0, 0, design.SpaceXXS, 0).Children(func() { clicked = t.Show(c).Clicked() })
	return clicked
}

// pane is a category's settings; header shows its title and summary (the
// two-pane layout).
func (p *SettingsPage) pane(c *ui.Context, cat settingsCategory, header bool) {
	cs := design.S()
	tt := design.TT()
	pad := float32(design.SpaceXL)
	if Compact() {
		pad = design.SpaceLG
	}
	top, left, right := float32(design.SpaceSM), pad, pad
	if header {
		top, left, right = design.SpaceXXL, design.SpaceLG, design.SpaceXL
	}
	st, ok := p.panes[cat.id]
	if !ok {
		st = &ui.ScrollState{}
		p.panes[cat.id] = st
	}
	ui.Scroll(c.Key("pane:" + cat.id)).Fill().TrackScroll(st).Children(func() {
		ui.Column(c).FillWidth().Padding(top, right, design.SpaceHuge+BottomInset, left).Children(func() {
			widgets.ContentWidth(c, 760, func() {
				if header {
					m3.Text(c, cat.title, tt.HeadlineSmall).Margin(0, 0, 0, design.SpaceXS)
					m3.Text(c, cat.summary, tt.BodyMedium.C(cs.OnSurfaceVariant)).Margin(design.SpaceXS, 0, 0, design.SpaceXS)
				}
				p.sections(c, cat.id)
			})
		})
	})
}

func (p *SettingsPage) sections(c *ui.Context, id string) {
	switch id {
	case "account":
		m3.VGap(c, design.SpaceXL)
		accountSection(c)
		SettingsGroup(c, l10n.SourcesTitle, icons.ExtensionOutlined, nil, func() { sourcesEntry(c) })
	case "appearance":
		SettingsGroup(c, l10n.SettingsAppearanceGroup, icons.PaletteOutlined, nil, func() { appearanceSettings(c) })
		SettingsGroup(c, l10n.CommonPlay, icons.PlayCircleOutlineRounded, nil, func() { playerSettings(c) })
	case "assistant":
		SettingsGroup(c, l10n.CommonAssistant, icons.AutoAwesomeOutlined, nil, func() { assistantSettings(c) })
		SettingsGroup(c, l10n.CommonPlugins, icons.ExtensionOutlined, nil, func() { PluginSettingsSection(c) })
	case "transfer":
		SettingsGroup(c, l10n.CommonDownload, icons.DownloadOutlined, nil, func() { p.downloads(c) })
		if p.asr == nil {
			p.asr = NewAsrSettings()
		}
		SettingsGroup(c, l10n.AssetTranscript, icons.GraphicEqRounded, nil, func() { p.asr.Show(c, &p.settings) })
		if p.mineru == nil {
			p.mineru = NewMinerUSettings()
		}
		SettingsGroup(c, l10n.AssetParse, 0, func() { brandIcon(c, "mineru.png", 18, 4, false) }, func() { p.mineru.Show(c) })
	case "knowledge":
		if p.know == nil {
			p.know = NewKnowledgeSettings(&p.settings)
		}
		p.know.Show(c)
	case "data":
		if p.data == nil {
			p.data = NewDataSettings()
		}
		p.data.Show(c)
	case "notifications":
		if p.watch == nil {
			p.watch = NewWatchSettings()
		}
		SettingsGroup(c, l10n.SettingsWatchGroup, icons.NotificationsOutlined, nil, func() { p.watch.Show(c, &p.settings) })
	case "about":
		if p.about == nil {
			p.about = NewAboutSection()
		}
		p.about.Show(c)
	default:
		if p.diag == nil {
			p.diag = NewDiagnostics()
		}
		p.diag.Show(c)
	}
}

func (p *SettingsPage) downloads(c *ui.Context) {
	cs := design.S()
	tt := design.TT()
	s := &p.settings
	conns := s.Num("download.max_conns", 8)
	par := s.Num("download.video_parallel", 2)
	limit := int(s.Num("download.rate_limit", 0))
	stream := core.Str(s.Values["download.stream"])
	ui.Column(c).FillWidth().Children(func() {
		sliderTile{Title: l10n.SettingsConnections, Subtitle: l10n.SettingsConnectionsHint, Min: 1, Max: 16, Divisions: 15,
			Label: l10n.SettingsConnectionsValue(int(conns + .5))}.Show(c, conns,
			func(v float64) { s.Set("download.max_conns", float64(int(v+.5))) }, func(v float64) { s.Put("download.max_conns", int(v+.5)) })
		sliderTile{Title: l10n.SettingsVideoParallel, Min: 1, Max: 4, Divisions: 3, Label: l10n.SettingsVideoParallelValue(int(par + .5))}.Show(c, par,
			func(v float64) { s.Set("download.video_parallel", float64(int(v+.5))) }, func(v float64) { s.Put("download.video_parallel", int(v+.5)) })
		chipRow := func(key, title string, children func()) {
			ui.Column(c.Key(key)).FillWidth().Padding(design.SpaceSM, 0).Children(func() {
				m3.Text(c, title, tt.BodyLarge)
				m3.VGap(c, design.SpaceSM)
				ui.Row(c).FillWidth().Wrap().Gap(design.SpaceSM).Children(children)
			})
		}
		chipRow("rate", l10n.SettingsRateLimit, func() {
			for i, o := range []struct {
				v int
				l string
			}{{0, l10n.CommonUnlimited}, {2 << 20, "2 MB/s"}, {5 << 20, "5 MB/s"}, {10 << 20, "10 MB/s"}, {20 << 20, "20 MB/s"}} {
				if m3.ChoiceChipOf(c.Key(i), o.l, limit == o.v) {
					s.Put("download.rate_limit", o.v)
				}
			}
		})
		chipRow("stream", l10n.SettingsStream, func() {
			for i, o := range [][2]string{{"", l10n.CommonAuto}, {l10n.StreamScreen, l10n.StreamScreen}, {l10n.StreamTeacher, l10n.StreamTeacher}, {l10n.StreamStudent, l10n.StreamStudent}} {
				if m3.ChoiceChipOf(c.Key(i), o[1], stream == o[0]) {
					s.Put("download.stream", o[0])
				}
			}
		})
		m3.Text(c, l10n.SettingsStreamHint, tt.BodySmall.C(cs.OnSurfaceVariant))
	})
}

// accountSection is the local profile.
func accountSection(c *ui.Context) {
	cs := design.S()
	tt := design.TT()
	ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(design.SpaceXL).Background(cs.SecondaryContainer).Radius(design.CornerXLIncreased).Children(func() {
		av := ui.ButtonBase(c).FocusRing(false).Radius(design.CornerFull).Cursor(ui.CursorPointer).Label(l10n.ProfileEdit)
		m3.InkWell(av, design.CornerFull)
		m3.Tip(c, av, l10n.ProfileEdit)
		av.Children(func() { UserAvatar(c, 56) })
		if av.Clicked() {
			ShowProfileDialog()
		}
		m3.HGap(c, design.SpaceLG)
		ui.Column(c).Grow(1).Basis(0).Children(func() {
			m3.Clamp(m3.Text(c, UserDisplayName(), tt.TitleLarge.C(cs.OnSecondaryContainer)), 1)
			m3.Text(c, l10n.SettingsProfileLocal, tt.BodyMedium.C(design.WithOpacity(cs.OnSecondaryContainer, .8)))
		})
		if (m3.Button{Kind: m3.Tonal, Icon: icons.EditOutlined, Label: l10n.ProfileEdit}).Show(c).Clicked() {
			ShowProfileDialog()
		}
	})
}

// sourcesEntry is the rows for sources and their learning platforms.
func sourcesEntry(c *ui.Context) {
	ss := State.Sources()
	signed, lms := 0, false
	for _, s := range ss {
		if s.SignedIn() {
			signed++
			lms = lms || s.HasLms()
		}
	}
	sub := l10n.SourcesNone
	if len(ss) > 0 {
		sub = l10n.SourcesSummary(len(ss), signed)
	}
	ui.Column(c).FillWidth().Children(func() {
		if settingsTile(c, "sources", m3.Tile{LeadingIcon: icons.ExtensionOutlined, Title: l10n.SourcesTitle, Subtitle: sub}) {
			m3.Nav().Push(NewSourcesPage())
		}
		if lms && settingsTile(c, "lms", m3.Tile{LeadingIcon: icons.AssignmentOutlined, Title: l10n.LmsTitle, Subtitle: l10n.LmsEntryHint}) {
			m3.Nav().Push(NewLmsPage())
		}
	})
}

func appearanceSettings(c *ui.Context) {
	cs := design.S()
	tt := design.TT()
	p := core.UI.Prefs
	hasSystem := c.Preferences().Accent.A != 0
	update := core.UI.Update
	ui.Column(c).FillWidth().Children(func() {
		m3.VGap(c, design.SpaceSM)
		mode := p.ThemeMode
		if m3.Segmented(c, []m3.Segment[design.ThemeMode]{
			{Value: design.ThemeSystem, Icon: icons.BrightnessAutoOutlined, Label: l10n.ThemeSystem},
			{Value: design.ThemeLight, Icon: icons.LightModeOutlined, Label: l10n.ThemeLight},
			{Value: design.ThemeDark, Icon: icons.DarkModeOutlined, Label: l10n.ThemeDark},
		}, &mode, m3.SegOpts{}) {
			update(func(x *design.Prefs) { x.ThemeMode = mode })
		}
		m3.VGap(c, design.SpaceMD)
		if hasSystem {
			if ch, v := switchTile(c, "dynamic", m3.Tile{Title: l10n.SettingsUseSystemAccent, Subtitle: l10n.SettingsUseSystemAccentHint}, p.DynamicColor); ch {
				update(func(x *design.Prefs) { x.DynamicColor = v })
			}
		}
		op := float32(1)
		if hasSystem && p.DynamicColor {
			op = .4
		}
		ui.Column(c).FillWidth().Opacity(op).Transition(ui.ElementTransition{Duration: design.Medium1}).Children(func() {
			m3.Text(c, l10n.SettingsThemeColor, tt.TitleSmall)
			m3.VGap(c, design.SpaceSM)
			ui.Row(c).FillWidth().Wrap().Gap(design.SpaceMD).Children(func() {
				for i, s := range design.SeedPalette {
					s := s
					on := p.Seed == s.Seed
					r := float32(22)
					border := ui.Transparent
					if on {
						r, border = design.CornerMD, cs.OnSurface
					}
					b := ui.ButtonBase(c.Key(i)).FocusRing(true).Size(44, 44).Center().Background(design.ARGB(s.Seed)).Border(2, border).
						Cursor(ui.CursorPointer).Label(s.Label)
					b = b.Radius(b.AnimateTo("seed.r", r, design.T().Motion(design.Medium1, design.Emphasized)))
					m3.Tip(c, b, s.Label)
					b.Children(func() {
						if on {
							m3.Icon(c, icons.CheckRounded, 24, ui.RGB(255, 255, 255))
						}
					})
					if b.Clicked() {
						update(func(x *design.Prefs) { x.Seed, x.DynamicColor = s.Seed, false })
					}
				}
			})
		})
		m3.VGap(c, design.SpaceLG)
		m3.Text(c, l10n.SettingsSchemeStyle, tt.TitleSmall)
		m3.VGap(c, design.SpaceSM)
		ui.Row(c).FillWidth().Wrap().Gap(design.SpaceSM).Children(func() {
			for i, v := range design.Variants {
				v := v
				if m3.ChoiceChipOf(c.Key(i), design.VariantLabel(v), p.Variant == v) {
					update(func(x *design.Prefs) { x.Variant = v })
				}
			}
		})
		m3.VGap(c, design.SpaceLG)
		m3.Text(c, l10n.SettingsContrast, tt.TitleSmall)
		m3.VGap(c, design.SpaceSM)
		var segs []m3.Segment[design.Contrast]
		for _, k := range design.Contrasts {
			segs = append(segs, m3.Segment[design.Contrast]{Value: k, Label: k.Label()})
		}
		contrast := p.Contrast
		if m3.Segmented(c, segs, &contrast, m3.SegOpts{}) {
			update(func(x *design.Prefs) { x.Contrast = contrast })
		}
		m3.VGap(c, design.SpaceLG)
		var fonts []m3.DropdownEntry[string]
		for _, f := range design.FontChoices {
			fonts = append(fonts, m3.DropdownEntry[string]{Value: f.Family, Label: f.Label})
		}
		font := p.FontFamily
		if m3.Dropdown(c, "font", m3.DropdownOpts{Label: l10n.SettingsFont}, fonts, &font) {
			update(func(x *design.Prefs) { x.FontFamily = font })
		}
		sliderTile{Title: l10n.SettingsTextSize, Min: .85, Max: 1.3, Divisions: 9, Label: strconv.Itoa(int(p.TextScale*100+.5)) + "%"}.Show(c, p.TextScale,
			func(v float64) { update(func(x *design.Prefs) { x.TextScale = v }) }, nil)
		if ch, v := switchTile(c, "compact", m3.Tile{Title: l10n.SettingsCompact}, p.CompactDensity); ch {
			update(func(x *design.Prefs) { x.CompactDensity = v })
		}
		if ch, v := switchTile(c, "motion", m3.Tile{Title: l10n.SettingsReduceMotion}, p.ReduceMotion); ch {
			update(func(x *design.Prefs) { x.ReduceMotion = v })
		}
		high := p.Contrast == design.ContrastHigh
		sub := l10n.SettingsGlassHint
		if high {
			sub = l10n.SettingsGlassHighContrast
		}
		if ch, v := switchTile(c, "glass", m3.Tile{Title: l10n.SettingsGlass, Subtitle: sub, Disabled: high}, p.Glass); ch {
			update(func(x *design.Prefs) { x.Glass = v })
		}
	})
}

func playerSettings(c *ui.Context) {
	cs := design.S()
	p := core.UI.Prefs
	update := core.UI.Update
	speed := func(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) + "×" }
	ui.Column(c).FillWidth().Children(func() {
		sliderTile{Title: l10n.SettingsDefaultSpeed, Min: .5, Max: 3, Divisions: 10, Label: speed(p.DefaultSpeed)}.Show(c, p.DefaultSpeed,
			func(v float64) { update(func(x *design.Prefs) { x.DefaultSpeed = float64(int(v*4+.5)) / 4 }) }, nil)
		sliderTile{Title: l10n.SettingsBoostSpeed, Subtitle: l10n.SettingsBoostSpeedHint, Min: 1.5, Max: 4, Divisions: 10, Label: speed(p.BoostSpeed)}.Show(c, p.BoostSpeed,
			func(v float64) { update(func(x *design.Prefs) { x.BoostSpeed = float64(int(v*4+.5)) / 4 }) }, nil)
		sliderTile{Title: l10n.SettingsSeekStep, Min: 5, Max: 30, Divisions: 5, Label: l10n.CommonSecondsValue(p.SeekStepSec)}.Show(c, float64(p.SeekStepSec),
			func(v float64) { update(func(x *design.Prefs) { x.SeekStepSec = int(v + .5) }) }, nil)
		sliderTile{Title: l10n.SettingsSubtitleSize, Min: .8, Max: 1.6, Divisions: 8, Label: strconv.Itoa(int(p.SubtitleScale*100+.5)) + "%"}.Show(c, p.SubtitleScale,
			func(v float64) { update(func(x *design.Prefs) { x.SubtitleScale = v }) }, nil)
		if ch, v := switchTile(c, "resume", m3.Tile{Title: l10n.SettingsResume}, p.AutoResume); ch {
			update(func(x *design.Prefs) { x.AutoResume = v })
		}
		if ch, v := switchTile(c, "subs", m3.Tile{Title: l10n.SettingsShowSubtitlesHint}, p.ShowSubtitles); ch {
			update(func(x *design.Prefs) { x.ShowSubtitles = v })
		}
		if ch, v := switchTile(c, "follow", m3.Tile{Title: l10n.SettingsSlidesFollowHint}, p.SlideFollow); ch {
			update(func(x *design.Prefs) { x.SlideFollow = v })
		}
		m3.VGap(c, design.SpaceSM)
		m3.Text(c, l10n.SettingsShortcutsBody, design.TT().BodySmall.C(cs.OnSurfaceVariant).H(1.6))
	})
}

func assistantSettings(c *ui.Context) {
	cs := design.S()
	tt := design.TT()
	p := core.UI.Prefs
	update := core.UI.Update
	a := Assistant
	ui.Column(c).FillWidth().Children(func() {
		sub := l10n.CommonNotConfigured
		if sel := a.Selected(); sel != nil {
			sub = sel.S("provider") + " / " + sel.S("model")
		}
		if settingsTile(c, "models", m3.Tile{LeadingIcon: icons.MemoryRounded, Title: l10n.SettingsModels, Subtitle: sub}) {
			m3.RootNav.Push(NewModelsPage())
		}
		visionModelTile(c)
		if settingsTile(c, "bot", m3.Tile{Leading: func() {
			BotAvatar{Size: 28, Shape: a.BotShape(), Color: a.BotColor(), Still: true}.Show(c)
		}, Title: a.BotName(), Subtitle: l10n.SettingsBotHint}) {
			m3.RootNav.Push(NewBotPage())
		}
		rt := l10n.SettingsRuntimeMissing
		if a.Ready() {
			rt = l10n.SettingsRuntimeReady
		}
		if settingsTile(c, "runtime", m3.Tile{LeadingIcon: icons.TerminalRounded, Title: l10n.SettingsRuntime, Subtitle: rt}) {
			ShowRuntimeSetup()
		}
		if settingsTile(c, "usage", m3.Tile{LeadingIcon: icons.InsightsOutlined, Title: l10n.SettingsUsage, Subtitle: l10n.SettingsUsageHint}) {
			m3.RootNav.Push(NewUsagePage())
		}
		if settingsTile(c, "tasks", m3.Tile{LeadingIcon: icons.ScheduleRounded, Title: l10n.CommonScheduledTasks, Subtitle: l10n.SettingsTasksHint}) {
			GoTo(TabTasks)
		}
		if ch, v := switchTile(c, "dock", m3.Tile{Title: l10n.SettingsDock}, p.AssistantDocked); ch {
			update(func(x *design.Prefs) { x.AssistantDocked = v })
		}
		if ch, v := switchTile(c, "bubbles", m3.Tile{Title: l10n.SettingsBubbles}, p.ChatBubbles); ch {
			update(func(x *design.Prefs) { x.ChatBubbles = v })
		}
		if ch, v := switchTile(c, "thinking", m3.Tile{Title: l10n.SettingsShowThinking}, p.ShowThinking); ch {
			update(func(x *design.Prefs) { x.ShowThinking = v })
		}
		ui.Column(c.Key("anim")).FillWidth().Padding(design.SpaceSM, 0).Children(func() {
			m3.Text(c, l10n.SettingsStreamAnimation, tt.BodyLarge)
			m3.VGap(c, design.SpaceSM)
			ui.Row(c).FillWidth().Wrap().Gap(design.SpaceSM).Children(func() {
				for i, o := range [][2]string{{design.StreamNone, l10n.CommonNone}, {design.StreamFade, l10n.StreamFade},
					{design.StreamBlurIn, l10n.StreamBlur}, {design.StreamTypewriter, l10n.StreamTypewriter}, {design.StreamWave, l10n.StreamWave}} {
					o := o
					if m3.ChoiceChipOf(c.Key(i), o[1], p.StreamAnimation == o[0]) {
						update(func(x *design.Prefs) { x.StreamAnimation = o[0] })
					}
				}
			})
		})
		sliderTile{Title: l10n.SettingsChatTextSize, Min: .85, Max: 1.3, Divisions: 9, Label: strconv.Itoa(int(p.ChatTextScale*100+.5)) + "%"}.Show(c, p.ChatTextScale,
			func(v float64) { update(func(x *design.Prefs) { x.ChatTextScale = v }) }, nil)
		_ = cs
	})
}

// visionModelTile picks the model that transcribes slides and figures for
// text-only chat models (the assistant's eyes; it never takes part in
// the reasoning).
func visionModelTile(c *ui.Context) {
	cs := design.S()
	a := Assistant
	vision := a.Models.O("prefs").S("vision")
	direct := a.Selected().IsTrue("vision")
	ready := a.Models.IsTrue("vision_ready")
	var sub string
	switch {
	case direct:
		fb := ""
		if vision != "" {
			fb = l10n.VisionNativeFallback(vision)
		}
		sub = l10n.VisionNative(fb)
	case vision == "":
		sub = l10n.VisionNeeded
	case ready:
		sub = l10n.VisionUsing(vision)
	default:
		sub = l10n.VisionUnavailable(vision)
	}
	t := m3.Tile{LeadingIcon: icons.ImageSearchRounded, Title: l10n.SettingsVisionModel, Subtitle: sub}
	if !direct && (vision == "" || !ready) {
		t.SubtitleFn = func() { m3.Text(c, sub, design.TT().BodyMedium.C(cs.Error)) }
	}
	if settingsTile(c, "vision", t) {
		pickVisionModel(vision)
	}
}

func pickVisionModel(current string) {
	type option struct{ id, name, provider string }
	var options []option
	for _, p := range Assistant.Models.Os("providers") {
		for _, m := range p.Os("models") {
			if m.IsTrue("available") && m.IsTrue("vision") {
				options = append(options, option{p.S("id") + "/" + m.S("id"), m.Or("name", m.S("id")), p.Or("name", p.S("id"))})
			}
		}
	}
	m3.ShowModalBottomSheet(m3.SheetOptions{ScrollControlled: true, HeightFactor: .7}, func(c *ui.Context, r *m3.Route) {
		cs := design.S()
		tt := design.TT()
		ui.Column(c).FillWidth().Grow(1).Basis(0).Children(func() {
			m3.Text(c, l10n.SettingsVisionModel, tt.TitleLarge).Margin(0, design.SpaceXL, design.SpaceSM, design.SpaceXL)
			m3.Text(c, l10n.VisionBody, tt.BodySmall.C(cs.OnSurfaceVariant)).Margin(0, design.SpaceXL, design.SpaceMD, design.SpaceXL)
			ui.Scroll(c).FillWidth().Grow(1).Basis(0).Children(func() {
				ui.Column(c).FillWidth().Children(func() {
					sel := current
					if m3.RadioTile(c.Key("none"), m3.Tile{Title: l10n.VisionNone}, &sel, "") {
						r.Pop(sel)
					}
					for _, o := range options {
						if m3.RadioTile(c.Key(o.id), m3.Tile{Title: o.name, Subtitle: o.provider}, &sel, o.id) {
							r.Pop(sel)
						}
					}
					if len(options) == 0 {
						m3.Text(c, l10n.VisionNoModels, tt.BodyMedium).Margin(design.SpaceXL)
					}
				})
			})
		})
	}).OnClose = func(v any) {
		picked, ok := v.(string)
		if !ok || picked == current {
			return
		}
		Assistant.SetPrefs(core.J{"vision": picked}, func(err error) {
			if err != nil {
				widgets.ShowSnack(l10n.CommonSaveFailed(apiMessage(err)), "", nil)
			}
		})
	}
}
