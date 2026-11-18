package app

import (
	"net/url"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// Sync between devices through a WebDAV folder.
// Contract: docs/specs/0014-data-sync-and-backup.md

type webdavPreset struct{ name, url, hint, userLabel, passLabel string }

func webdavPresets() []webdavPreset {
	return []webdavPreset{
		{l10n.WebdavJianguoyun, "https://dav.jianguoyun.com/dav/", l10n.WebdavJianguoyunHint, l10n.WebdavJianguoyunUser, ""},
		{"Nextcloud", "https://", l10n.WebdavNextcloudHint, "", ""},
		{"AList", "http://", l10n.WebdavAlistHint, "", l10n.CommonPassword},
		{l10n.CommonOther, "https://", l10n.WebdavOtherHint, "", l10n.CommonPassword},
	}
}

func urlHost(raw string) string {
	if u, err := url.Parse(raw); err == nil {
		return u.Hostname()
	}
	return ""
}

// webdavSummary is one line for the sync status, also used by the
// settings tile.
func webdavSummary(st core.J) string {
	if st.S("kind") == "folder" {
		return l10n.FolderSyncUsingCourseSync
	}
	if !st.IsTrue("configured") {
		return l10n.WebdavSubtitle
	}
	if st.IsTrue("running") {
		return l10n.CommonSyncing
	}
	last := st.O("last")
	if e := last.S("error"); e != "" {
		return e
	}
	when := l10n.WebdavNeverSynced
	if at := last.I("at"); at != 0 {
		when = l10n.WebdavSyncedAgo(core.FmtRelative(time.UnixMilli(int64(at))))
	}
	s := urlHost(st.S("url")) + " · " + when
	if st.IsTrue("auto") {
		s += l10n.CommonAutoSuffix
	}
	return s
}

// WebDavPage sets up and runs WebDAV sync.
type WebDavPage struct {
	st                                  core.J
	err                                 error
	formError                           string
	busy, obscure, editing, auto, media bool
	preset                              int
	url, user, pass, folder             string
	scroll                              ui.ScrollState
	cancel, closed                      func()
}

// NewWebDavPage opens the WebDAV page; closed runs as it is popped.
func NewWebDavPage(closed func()) m3.Page {
	p := &WebDavPage{obscure: true, auto: true, url: webdavPresets()[0].url, folder: "Coursive", closed: closed}
	p.load()
	p.cancel = State.Events.Listen(func(e core.J) {
		switch e.S("type") {
		case "sync_progress":
			st := p.st.Copy()
			if st == nil {
				st = core.J{}
			}
			st["running"], st["progress"] = true, e["data"]
			p.st = st
		case "sync_done", "sync_changed":
			p.load()
		}
	})
	return p
}

// Dispose stops following the events.
func (p *WebDavPage) Dispose() {
	p.cancel()
	if p.closed != nil {
		p.closed()
	}
}

// MainScroll is the page's scroll.
func (p *WebDavPage) MainScroll() *ui.ScrollState { return &p.scroll }

func (p *WebDavPage) load() {
	core.Async(func() (any, error) { return api().Get("/api/sync") }, func(v any, err error) {
		if err != nil {
			p.err = err
			return
		}
		p.st, p.err = core.Obj(v), nil
	})
}

func (p *WebDavPage) do(method, path string, body any) {
	p.busy = true
	core.Async(func() (any, error) { return api().Call(method, path, body) }, func(v any, err error) {
		p.busy = false
		if err != nil {
			widgets.ShowSnack(apiMessage(err), "", nil)
			return
		}
		p.st = core.Obj(v)
	})
}

func (p *WebDavPage) edit() {
	st := p.st
	u := st.S("url")
	p.editing, p.formError = true, ""
	p.url, p.user, p.pass, p.folder = u, st.S("user"), "", st.Or("folder", "Coursive")
	p.auto, p.media = st.IsTrue("auto"), st.IsTrue("media")
	presets := webdavPresets()
	p.preset = len(presets) - 1
	for i, pr := range presets {
		if len(pr.url) > 8 && strings.HasPrefix(u, pr.url) {
			p.preset = i
			break
		}
	}
}

func (p *WebDavPage) save() {
	p.busy, p.formError = true, ""
	keepPass := p.st.IsTrue("configured") && p.st.IsTrue("has_password") && p.pass == ""
	body := core.J{"url": strings.TrimSpace(p.url), "user": strings.TrimSpace(p.user), "folder": strings.TrimSpace(p.folder), "auto": p.auto, "media": p.media}
	if !keepPass {
		body["password"] = p.pass
	}
	first := !p.st.IsTrue("configured")
	core.Async(func() (any, error) { return api().Put("/api/sync", body) }, func(v any, err error) {
		p.busy = false
		if err != nil {
			p.formError = apiMessage(err)
			return
		}
		p.pass, p.st, p.editing = "", core.Obj(v), false
		if first {
			p.do("POST", "/api/sync/run", nil)
		}
	})
}

func (p *WebDavPage) disconnect() {
	m3.Confirm(l10n.WebdavStopTitle, l10n.WebdavStopBody, l10n.CommonCancel, l10n.WebdavStop, false, func() { p.do("DELETE", "/api/sync", nil) })
}

// View builds the page.
func (p *WebDavPage) View(c *ui.Context) {
	st := p.st
	if st == nil {
		m3.Scaffold{AppBar: &m3.AppBar{Title: l10n.WebdavTitle}, Body: func() {
			if p.err != nil {
				widgets.ErrorState(c, p.err, p.load)
			} else {
				widgets.CenteredLoader(c, "")
			}
		}}.Show(c)
		return
	}
	cs := design.S()
	tt := design.TT()
	on := st.IsTrue("configured") && st.S("kind") != "folder"
	SettingsSubpage(c, l10n.WebdavTitle, &p.scroll, func() {
		status := l10n.CommonNotSet
		if on {
			status = l10n.WebdavConnectedTo(urlHost(st.S("url")))
		}
		IntegrationHeader(c, func() {
			ui.Box(c).Size(56, 56).Shrink(0).Center().Radius(design.CornerLG).Background(cs.PrimaryContainer).Children(func() {
				m3.Icon(c, icons.CloudSyncRounded, 30, cs.OnPrimaryContainer)
			})
		}, l10n.WebdavTitle, status, l10n.WebdavBody, on)
		if !on || p.editing {
			p.form(c, on)
		} else {
			p.connected(c, st)
		}
		m3.Text(c, l10n.WebdavPrivacy, tt.BodySmall.C(cs.OnSurfaceVariant)).Margin(design.SpaceLG, design.SpaceXS, 0, design.SpaceXS)
	})
}

func (p *WebDavPage) form(c *ui.Context, on bool) {
	cs := design.S()
	tt := design.TT()
	presets := webdavPresets()
	preset := presets[p.preset]
	hasPass := p.st.IsTrue("has_password")
	ready := len(strings.TrimSpace(p.url)) > 8 && strings.TrimSpace(p.user) != "" && (p.pass != "" || (on && hasPass))
	title := l10n.WebdavConnect
	if on {
		title = l10n.WebdavEditAccount
	}
	SettingsGroup(c, title, 0, nil, func() {
		m3.VGap(c, design.SpaceXS)
		ui.Row(c).FillWidth().Wrap().Gap(design.SpaceSM).Children(func() {
			for i, pr := range presets {
				if (m3.Chip{Kind: m3.ChoiceChip, Label: pr.name, Selected: p.preset == i, Disabled: p.busy}).Show(c.Key(i)).Clicked() {
					p.preset, p.url, p.formError = i, pr.url, ""
				}
			}
		})
		m3.VGap(c, design.SpaceMD)
		ui.Row(c).FillWidth().AlignItems(ui.Start).Children(func() {
			m3.Icon(c, icons.InfoOutlineRounded, 18, cs.OnSurfaceVariant)
			m3.HGap(c, design.SpaceSM)
			m3.Text(c, preset.hint, tt.BodySmall.C(cs.OnSurfaceVariant)).Grow(1).Basis(0)
		})
		m3.VGap(c, design.SpaceLG)
		clear := func(e ui.Element) {
			if e.Changed() {
				p.formError = ""
			}
		}
		clear(m3.Field{Label: l10n.WebdavServer}.Show(c.Key("url"), &p.url))
		m3.VGap(c, design.SpaceMD)
		userLabel := preset.userLabel
		if userLabel == "" {
			userLabel = l10n.WebdavAccount
		}
		clear(m3.Field{Label: userLabel}.Show(c.Key("user"), &p.user))
		m3.VGap(c, design.SpaceMD)
		passLabel := preset.passLabel
		if passLabel == "" {
			passLabel = l10n.WebdavAppPassword
		}
		helper := l10n.WebdavPasswordSealed
		if on && hasPass {
			helper = l10n.WebdavKeepPassword
		}
		tip, ic := l10n.CommonShow, icons.VisibilityOutlined
		if !p.obscure {
			tip, ic = l10n.CommonHide, icons.VisibilityOffOutlined
		}
		clear(m3.Field{Label: passLabel, Helper: helper, Password: p.obscure, SuffixFn: func() {
			if m3.IconBtn(c, ic, tip).Clicked() {
				p.obscure = !p.obscure
			}
		}}.Show(c.Key("pass"), &p.pass))
		m3.VGap(c, design.SpaceMD)
		m3.Field{Label: l10n.WebdavFolder, Helper: l10n.WebdavFolderHelper}.Show(c.Key("folder"), &p.folder)
		m3.VGap(c, design.SpaceSM)
		if ch, v := switchTile(c, "auto", m3.Tile{Title: l10n.CommonAutoSync, Subtitle: l10n.WebdavAutoHint, Disabled: p.busy}, p.auto); ch {
			p.auto = v
		}
		if ch, v := switchTile(c, "media", m3.Tile{Title: l10n.WebdavMedia, Subtitle: l10n.WebdavMediaHint, Disabled: p.busy}, p.media); ch {
			p.media = v
		}
		if p.formError != "" {
			m3.Text(c, p.formError, tt.BodyMedium.C(cs.Error)).Margin(design.SpaceSM, 0, 0, 0)
		}
		m3.VGap(c, design.SpaceLG)
		ui.Row(c).FillWidth().Wrap().Gap(design.SpaceSM).Children(func() {
			label := l10n.WebdavConnectAndSync
			switch {
			case p.busy:
				label = l10n.CommonConnecting
			case on:
				label = l10n.CommonSave
			}
			if (m3.Button{Kind: m3.Filled, Icon: icons.CloudDoneOutlined, Label: label, Disabled: p.busy || !ready, Busy: p.busy}).Show(c).Clicked() {
				p.save()
			}
			if on && (m3.Button{Kind: m3.TextKind, Label: l10n.CommonCancel, Disabled: p.busy}).Show(c).Clicked() {
				p.editing = false
			}
		})
	})
}

func (p *WebDavPage) connected(c *ui.Context, st core.J) {
	cs := design.S()
	tt := design.TT()
	running := st.IsTrue("running")
	prog := st.O("progress")
	done, total := prog.I("done"), prog.I("total")
	last := st.O("last")
	errs := last.Ss("errors")
	at := last.I("at")
	conflictDir := last.S("conflict_dir")
	SettingsGroup(c, l10n.CommonSync, 0, nil, func() {
		if ch, v := switchTile(c, "auto", m3.Tile{Title: l10n.CommonAutoSync, Subtitle: l10n.WebdavAutoHint, Disabled: p.busy}, st.IsTrue("auto")); ch {
			p.do("POST", "/api/sync/auto", core.J{"auto": v})
		}
		m3.VGap(c, design.SpaceSM)
		switch {
		case running:
			ui.Column(c.Key("running")).FillWidth().Padding(0, 0, design.SpaceLG, 0).Children(func() {
				pr := m3.Progress{}
				if total > 0 {
					pr.Value = m3.Of(float64(done) / float64(total))
				}
				m3.Linear(c, pr)
				m3.VGap(c, design.SpaceSM)
				parts := []string{prog.Or("step", l10n.WebdavSyncing)}
				if total > 0 {
					parts = append(parts, core.Str(done)+" / "+core.Str(total))
				}
				if b := prog.F("bytes"); b > 0 {
					parts = append(parts, core.FmtBytes(b))
				}
				m3.Text(c, strings.Join(parts, " · "), tt.BodySmall.C(cs.OnSurfaceVariant))
			})
		case at != 0:
			ui.Box(c.Key("last")).FillWidth().Padding(0, 0, design.SpaceLG, 0).Children(func() { webdavLastSync(c, last) })
		}
		if !running && conflictDir != "" && last.I("conflicts") > 0 {
			action := ""
			if core.CanOpenFolders() {
				action = l10n.CommonView
			}
			settingsNotice(c, "conflicts", icons.DifferenceOutlined, l10n.WebdavConflicts(last["conflicts"]), action, func() { go core.RevealPath(conflictDir) }, false)
		}
		if !running && len(errs) > 0 {
			settingsNotice(c, "errors", icons.ErrorOutlineRounded, l10n.WebdavErrors(len(errs), errs[0]), "", nil, true)
		}
		ui.Row(c).FillWidth().Wrap().Gap(design.SpaceSM).Children(func() {
			label := l10n.CommonSyncNow
			if at == 0 {
				label = l10n.CommonStartSync
			}
			if (m3.Button{Kind: m3.Filled, Icon: icons.SyncRounded, Label: label, Disabled: running || p.busy}).Show(c).Clicked() {
				p.do("POST", "/api/sync/run", nil)
			}
		})
	})
	SettingsGroup(c, l10n.WebdavAccount, 0, nil, func() {
		m3.Tile{LeadingIcon: icons.DnsOutlined, Title: st.S("user"), Subtitle: st.S("url") + st.S("folder"), SubtitleLines: 2, Pad: true, PadX: 0}.Show(c.Key("user"))
		media := l10n.WebdavMediaOff
		if st.IsTrue("media") {
			media = l10n.WebdavMediaOn
		}
		m3.Tile{LeadingIcon: icons.MovieOutlined, Title: l10n.WebdavMediaTitle, Subtitle: media, Pad: true, PadX: 0}.Show(c.Key("media"))
		m3.VGap(c, design.SpaceSM)
		ui.Row(c).FillWidth().Wrap().Gap(design.SpaceSM).Children(func() {
			if (m3.Button{Kind: m3.Outlined, Label: l10n.CommonModify, Disabled: p.busy || running}).Show(c).Clicked() {
				p.edit()
			}
			if (m3.Button{Kind: m3.TextKind, Label: l10n.WebdavStop, FG: cs.Error, Disabled: p.busy || running}).Show(c).Clicked() {
				p.disconnect()
			}
		})
	})
}

func webdavLastSync(c *ui.Context, last core.J) {
	cs := design.S()
	tt := design.TT()
	errText := last.S("error")
	var counts []string
	if n := last.I("uploaded"); n > 0 {
		counts = append(counts, l10n.SyncUploaded(n))
	}
	if n := last.I("downloaded"); n > 0 {
		counts = append(counts, l10n.CommonDownloadSize(n))
	}
	if n := last.I("deleted"); n > 0 {
		counts = append(counts, l10n.SyncDeleted(n))
	}
	if n := last.F("bytes"); n > 0 {
		counts = append(counts, core.FmtBytes(n))
	}
	partial := last.IsTrue("partial")
	ic, col := icons.CheckCircleOutlineRounded, cs.Primary
	line, lineCol := l10n.SyncInSync, cs.OnSurfaceVariant
	if len(counts) > 0 {
		line = strings.Join(counts, " · ")
	}
	if errText != "" {
		line = errText
		if partial {
			ic, col, lineCol = icons.PauseCircleOutlineRounded, cs.Tertiary, cs.Tertiary
		} else {
			ic, col, lineCol = icons.ErrorOutlineRounded, cs.Error, cs.Error
		}
	}
	ui.Row(c).FillWidth().AlignItems(ui.Start).Children(func() {
		m3.Icon(c, ic, 20, col)
		m3.HGap(c, design.SpaceMD)
		ui.Column(c).Grow(1).Basis(0).Children(func() {
			m3.Text(c, l10n.CommonLastSynced(core.FmtRelative(time.UnixMilli(int64(last.I("at"))))), tt.BodyMedium)
			m3.Text(c, line, tt.BodySmall.C(lineCol))
		})
	})
}

// settingsNotice is a tonal note with an optional action.
func settingsNotice(c *ui.Context, key string, ic icons.Icon, text, action string, onAction func(), failed bool) {
	cs := design.S()
	col := cs.Tertiary
	if failed {
		col = cs.Error
	}
	ui.Row(c.Key(key)).FillWidth().AlignItems(ui.Center).Margin(0, 0, design.SpaceMD, 0).Padding(design.SpaceMD, design.SpaceSM, design.SpaceMD, design.SpaceLG).
		Background(cs.SurfaceContainerHigh).Radius(design.CornerLG).Children(func() {
		m3.Icon(c, ic, 20, col)
		m3.HGap(c, design.SpaceMD)
		m3.Text(c, text, design.TT().BodyMedium).Grow(1).Basis(0)
		if action != "" && m3.TextButton(c, action).Clicked() {
			onAction()
		}
	})
}
