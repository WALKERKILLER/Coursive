package app

import (
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
)

// FolderSyncPage is the course-scoped portable library, carried by the
// NAS's WebDAV server. Contract: docs/specs/0023-course-folder-sync.md.
type FolderSyncPage struct {
	status, courses            core.J
	selected                   map[string]bool
	url, user, pass, folder    string
	busy, media, auto, obscure bool
	err                        string
	poll                       poller
	scroll                     ui.ScrollState
	closed                     func()
}

// NewFolderSyncPage opens the page; closed runs as it is popped.
func NewFolderSyncPage(closed func()) m3.Page {
	p := &FolderSyncPage{status: core.J{}, courses: core.J{}, selected: map[string]bool{}, media: true, auto: true, obscure: true, closed: closed}
	p.load()
	return p
}

// Dispose tells the opener.
func (p *FolderSyncPage) Dispose() {
	if p.closed != nil {
		p.closed()
	}
}

// MainScroll is the page's scroll.
func (p *FolderSyncPage) MainScroll() *ui.ScrollState { return &p.scroll }

func (p *FolderSyncPage) settings() core.J {
	var ids []string
	for id, on := range p.selected {
		if on {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	s := core.J{"kind": "folder", "url": strings.TrimSpace(p.url), "user": strings.TrimSpace(p.user), "folder": strings.TrimSpace(p.folder),
		"course_ids": ids, "media": p.media, "auto": p.auto}
	if ids == nil {
		s["course_ids"] = []string{}
	}
	if p.pass != "" {
		s["password"] = p.pass
	}
	return s
}

func (p *FolderSyncPage) count() int {
	n := 0
	for _, on := range p.selected {
		if on {
			n++
		}
	}
	return n
}

func (p *FolderSyncPage) load() {
	core.Async(func() (any, error) { return api().Get("/api/sync") }, func(v any, err error) {
		if err != nil {
			p.err = apiMessage(err)
			return
		}
		s := core.Obj(v)
		p.status = s
		if p.url == "" && s.S("url") != "" {
			p.url, p.user, p.folder = s.S("url"), s.S("user"), s.S("folder")
			p.media, p.auto = s.IsTrue("media"), s.IsTrue("auto")
			if s.S("kind") == "folder" {
				for _, id := range s.Ss("course_ids") {
					p.selected[id] = true
				}
			}
		}
		if len(p.courses) == 0 && p.count() > 0 {
			p.preview(nil)
		}
	})
}

func (p *FolderSyncPage) preview(done func(core.J, error)) {
	settings := p.settings()
	core.Async(func() (any, error) { return api().PreviewFolderSync(settings) }, func(v any, err error) {
		r := core.Obj(v)
		if err == nil {
			p.courses = r.O("courses")
		}
		if done != nil {
			done(r, err)
		}
	})
}

func (p *FolderSyncPage) refreshCourses() {
	p.busy, p.err = true, ""
	p.preview(func(_ core.J, err error) {
		p.busy = false
		if err != nil {
			p.err = apiMessage(err)
		}
	})
}

// ready: a stored password is reused when the field is left empty, so
// only a first connection has to type it.
func (p *FolderSyncPage) ready() bool {
	return len(strings.TrimSpace(p.url)) > 8 && strings.TrimSpace(p.user) != "" &&
		(p.pass != "" || (p.status.IsTrue("has_password") && p.status.S("kind") == "folder")) && p.count() > 0
}

func (p *FolderSyncPage) connect() {
	p.busy, p.err = true, ""
	p.preview(func(pv core.J, err error) {
		if err != nil {
			p.busy, p.err = false, apiMessage(err)
			return
		}
		mb := core.Fixed(pv.F("bytes")/1e6, 1)
		remote := core.Fixed(pv.F("incoming_bytes")/1e6, 1)
		msg := l10n.FolderSyncSelected(p.count()) + "\n\n" +
			l10n.FolderSyncPreview(pv.I("files"), mb, pv.I("incoming_files"), remote) + "\n\n" + l10n.FolderSyncPrivacy(p.media)
		m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
			m3.AlertDialog(c, m3.Alert{Title: l10n.FolderSyncConfirmTitle, Message: msg, Actions: func() {
				if m3.TextButton(c, l10n.CommonCancel).Clicked() {
					r.Pop(false)
				}
				if m3.FilledButton(c, l10n.FolderSyncConfirm).Clicked() {
					r.Pop(true)
				}
			}})
		}, func(v any) {
			if v != true {
				p.busy = false
				return
			}
			settings := p.settings()
			core.Async(func() (any, error) { return api().Put("/api/sync", settings) }, func(_ any, err error) {
				p.busy = false
				if err != nil {
					p.err = apiMessage(err)
					return
				}
				p.pass = ""
				p.run()
			})
		})
	})
}

func (p *FolderSyncPage) run() {
	kind := p.status.S("kind")
	core.Go(func() error {
		if kind == "folder" {
			_, err := api().SyncNow()
			return err
		}
		return nil
	}, func(err error) {
		if err != nil {
			p.err = apiMessage(err)
		}
		p.load()
	})
}

func (p *FolderSyncPage) conflicts() {
	core.Async(func() (any, error) { return api().Get("/api/sync/folder/conflicts") }, func(v any, err error) {
		if err != nil {
			p.err = apiMessage(err)
			return
		}
		rows := core.Objs(v)
		m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
			m3.AlertDialog(c, m3.Alert{Title: l10n.FolderSyncConflicts, Width: 620, Scrollable: true, Content: func() {
				if len(rows) == 0 {
					m3.Text(c, l10n.FolderSyncNoConflicts, design.TT().BodyMedium)
				}
				for i, row := range rows {
					row := row
					if (m3.Tile{Title: row.S("path"), Subtitle: core.Str(row["bytes"]) + " B", Tap: true}).Show(c.Key(i)).Clicked() {
						p.inspect(row)
					}
				}
			}, Actions: func() {
				if m3.TextButton(c, l10n.CommonClose).Clicked() {
					r.Close()
				}
			}})
		}, nil)
	})
}

func (p *FolderSyncPage) inspect(row core.J) {
	id := row.S("id")
	core.Async(func() (any, error) { return api().Get("/api/sync/folder/conflicts/" + core.Esc(id)) }, func(v any, err error) {
		if err != nil {
			p.err = apiMessage(err)
			return
		}
		detail := core.Obj(v)
		body := detail.Or("body", l10n.FolderSyncBinaryCopy)
		m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
			m3.AlertDialog(c, m3.Alert{Title: l10n.FolderSyncOtherCopy, Width: 650, Scrollable: true, Content: func() {
				m3.Text(c, body, design.TT().BodyMedium).Selectable()
			}, Actions: func() {
				if m3.TextButton(c, l10n.FolderSyncLater).Clicked() {
					r.Close()
				}
				if m3.TextButton(c, l10n.FolderSyncKeepCurrent).Clicked() {
					r.Pop("keep")
				}
				if m3.FilledButton(c, l10n.FolderSyncUseThis).Clicked() {
					r.Pop("restore")
				}
			}})
		}, func(v any) {
			action, _ := v.(string)
			if action == "" {
				return
			}
			core.Async(func() (any, error) {
				return api().Post("/api/sync/folder/conflicts/"+core.Esc(id)+"/resolve", core.J{"action": action, "expected_hash": detail["current_hash"]})
			}, func(_ any, err error) {
				if err != nil {
					p.err = apiMessage(err)
				}
			})
		})
	})
}

// View builds the page; it polls every 5 seconds while it shows.
func (p *FolderSyncPage) View(c *ui.Context) {
	p.poll.Every(5*time.Second, p.load)
	cs := design.S()
	tt := design.TT()
	last := p.status.O("last")
	running := p.status.IsTrue("running")
	on := p.status.IsTrue("configured") && p.status.S("kind") == "folder"
	host := urlHost(strings.TrimSpace(p.url))
	SettingsSubpage(c, l10n.FolderSyncTitle, &p.scroll, func() {
		status := l10n.CommonNotSet
		if on && host != "" {
			status = l10n.WebdavConnectedTo(host)
		}
		IntegrationHeader(c, func() {
			ui.Box(c).Size(56, 56).Shrink(0).Center().Radius(design.CornerLG).Background(cs.PrimaryContainer).Children(func() {
				m3.Icon(c, icons.FolderCopyOutlined, 30, cs.OnPrimaryContainer)
			})
		}, l10n.FolderSyncTitle, status, l10n.FolderSyncBody, on)
		SettingsGroup(c, l10n.FolderSyncAccount, icons.DnsOutlined, nil, func() {
			m3.VGap(c, design.SpaceXS)
			if (m3.Field{Label: l10n.WebdavServer, Helper: l10n.FolderSyncAccountHint}).Show(c.Key("url"), &p.url).Changed() {
				p.err = ""
			}
			m3.VGap(c, design.SpaceMD)
			m3.Field{Label: l10n.WebdavAccount}.Show(c.Key("user"), &p.user)
			m3.VGap(c, design.SpaceMD)
			helper := l10n.WebdavPasswordSealed
			if on && p.status.IsTrue("has_password") {
				helper = l10n.WebdavKeepPassword
			}
			tip, ic := l10n.CommonShow, icons.VisibilityOutlined
			if !p.obscure {
				tip, ic = l10n.CommonHide, icons.VisibilityOffOutlined
			}
			m3.Field{Label: l10n.WebdavAppPassword, Helper: helper, Password: p.obscure, SuffixFn: func() {
				if m3.IconBtn(c, ic, tip).Clicked() {
					p.obscure = !p.obscure
				}
			}}.Show(c.Key("pass"), &p.pass)
			m3.VGap(c, design.SpaceMD)
			m3.Field{Label: l10n.WebdavFolder, Helper: l10n.WebdavFolderHelper}.Show(c.Key("folder"), &p.folder)
		})
		SettingsGroup(c, l10n.FolderSyncCourses, icons.MenuBookOutlined, nil, func() {
			if len(p.courses) == 0 {
				m3.Text(c, l10n.FolderSyncNoCourses, tt.BodyMedium.C(cs.OnSurfaceVariant)).Margin(design.SpaceSM, 0)
			}
			ids := make([]string, 0, len(p.courses))
			for id := range p.courses {
				ids = append(ids, id)
			}
			sort.Slice(ids, func(i, j int) bool { return p.courses.S(ids[i]) < p.courses.S(ids[j]) })
			for _, id := range ids {
				v := p.selected[id]
				if m3.CheckTile(c.Key(id), m3.Tile{Title: p.courses.S(id), Disabled: p.busy, Pad: true, PadX: 0}, &v) {
					p.selected[id] = v
				}
			}
			m3.VGap(c, design.SpaceSM)
			ui.Row(c).FillWidth().Children(func() {
				if (m3.Button{Kind: m3.Outlined, Icon: icons.RefreshRounded, Label: l10n.FolderSyncRefresh, Disabled: p.busy || strings.TrimSpace(p.url) == ""}).Show(c).Clicked() {
					p.refreshCourses()
				}
			})
		})
		SettingsGroup(c, l10n.FolderSyncOptions, icons.TuneRounded, nil, func() {
			if ch, v := switchTile(c, "media", m3.Tile{Title: l10n.WebdavMedia, Subtitle: l10n.WebdavMediaHint, Disabled: p.busy}, p.media); ch {
				p.media = v
			}
			if ch, v := switchTile(c, "auto", m3.Tile{Title: l10n.CommonAutoSync, Subtitle: l10n.FolderSyncAutoHint, Disabled: p.busy}, p.auto); ch {
				p.auto = v
			}
		})
		if p.err != "" {
			m3.Text(c, p.err, tt.BodyMedium.C(cs.Error)).Margin(design.SpaceMD, 0, 0, 0)
		}
		if e := last.S("error"); e != "" {
			m3.Text(c, e, tt.BodyMedium.C(cs.Error)).Margin(design.SpaceMD, 0, 0, 0)
		}
		if running {
			ui.Box(c).FillWidth().Padding(design.SpaceMD, 0, 0, 0).Children(func() { m3.Linear(c, m3.Progress{}) })
		} else if last.Has("at") && last["at"] != nil {
			m3.Text(c, l10n.FolderSyncLastRun(last.I("uploaded"), last.I("downloaded"), last.I("conflicts")), tt.BodySmall.C(cs.OnSurfaceVariant)).Margin(design.SpaceMD, 0, 0, 0)
		}
		m3.VGap(c, design.SpaceLG)
		ui.Row(c).FillWidth().Wrap().Gap(design.SpaceSM).Children(func() {
			if (m3.Button{Kind: m3.Filled, Icon: icons.CloudDoneOutlined, Label: l10n.FolderSyncConnect, Disabled: p.busy || !p.ready(), Busy: p.busy}).Show(c).Clicked() {
				p.connect()
			}
			if (m3.Button{Kind: m3.Outlined, Icon: icons.SyncRounded, Label: l10n.FolderSyncCheck, Disabled: p.busy || running || !on}).Show(c).Clicked() {
				p.run()
			}
			if (m3.Button{Kind: m3.TextKind, Icon: icons.CallSplitRounded, Label: l10n.FolderSyncConflicts}).Show(c).Clicked() {
				p.conflicts()
			}
		})
		m3.Text(c, l10n.WebdavPrivacy, tt.BodySmall.C(cs.OnSurfaceVariant)).Margin(design.SpaceLG, design.SpaceXS, 0, design.SpaceXS)
	})
}
