package app

import (
	"regexp"
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

const (
	notionLogo            = "notion.svg"
	notionIntegrationsURL = "https://www.notion.so/profile/integrations"
)

// notionSummary is one line for the Notion status, also used by the
// settings tile.
func notionSummary(st core.J) string {
	if !st.IsTrue("connected") {
		return l10n.NotionSubtitle
	}
	if st.IsTrue("running") {
		return l10n.CommonSyncing
	}
	parent := st.O("parent")
	if len(parent) == 0 {
		return l10n.NotionNoParent
	}
	if e := st.O("last").S("error"); e != "" {
		return e
	}
	auto := ""
	if st.IsTrue("auto") {
		auto = l10n.CommonAutoSuffix
	}
	pages := any(0)
	if st.Has("pages") {
		pages = st.I("pages")
	}
	return l10n.NotionStatus(parent["title"], pages, auto)
}

// NotionPage connects an integration and syncs the library into a page.
type NotionPage struct {
	st             core.J
	err            error
	connectError   string
	busy, obscure  bool
	token          string
	scroll         ui.ScrollState
	conflictsOpen  bool
	errorsOpen     bool
	cancel, closed func()
}

// NewNotionPage opens the Notion page; closed runs as it is popped.
func NewNotionPage(closed func()) m3.Page {
	p := &NotionPage{obscure: true, closed: closed}
	p.load()
	p.cancel = State.Events.Listen(func(e core.J) {
		switch e.S("type") {
		case "notion_progress":
			st := p.st.Copy()
			if st == nil {
				st = core.J{}
			}
			st["running"], st["progress"] = true, e["data"]
			p.st = st
		case "notion_synced", "notion_changed":
			p.load()
		}
	})
	return p
}

// Dispose stops following the events.
func (p *NotionPage) Dispose() {
	p.cancel()
	if p.closed != nil {
		p.closed()
	}
}

// MainScroll is the page's scroll.
func (p *NotionPage) MainScroll() *ui.ScrollState { return &p.scroll }

func (p *NotionPage) load() {
	core.Async(func() (any, error) { return api().Get("/api/notion") }, func(v any, err error) {
		if err != nil {
			p.err = err
			return
		}
		p.st, p.err = core.Obj(v), nil
	})
}

// do runs a request that answers with the status; failures show.
func (p *NotionPage) do(method, path string, body any) {
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

func (p *NotionPage) connect() {
	p.busy, p.connectError = true, ""
	token := strings.TrimSpace(p.token)
	core.Async(func() (any, error) { return api().Post("/api/notion/connect", core.J{"token": token}) }, func(v any, err error) {
		p.busy = false
		if err != nil {
			p.connectError = apiMessage(err)
			return
		}
		p.token, p.st = "", core.Obj(v)
	})
}

func (p *NotionPage) disconnect() {
	m3.Confirm(l10n.NotionDisconnectTitle, l10n.NotionDisconnectBody, l10n.CommonCancel, l10n.CommonDisconnect, false, func() {
		p.do("POST", "/api/notion/disconnect", nil)
	})
}

func (p *NotionPage) pickParent() {
	pages := p.st.I("pages")
	showNotionPagePicker(func(picked string) {
		if pages > 0 {
			m3.Confirm(l10n.NotionChangeParentTitle, l10n.NotionChangeParentBody, l10n.CommonCancel, l10n.CommonChange, false, func() {
				p.do("PUT", "/api/notion", core.J{"parent": picked})
			})
			return
		}
		p.do("PUT", "/api/notion", core.J{"parent": picked})
	})
}

// View builds the page.
func (p *NotionPage) View(c *ui.Context) {
	st := p.st
	if st == nil {
		m3.Scaffold{AppBar: &m3.AppBar{Title: "Notion"}, Body: func() {
			if p.err != nil {
				widgets.ErrorState(c, p.err, p.load)
			} else {
				widgets.CenteredLoader(c, "")
			}
		}}.Show(c)
		return
	}
	connected := st.IsTrue("connected")
	bot := st.O("bot")
	SettingsSubpage(c, "Notion", &p.scroll, func() {
		status := l10n.CommonNotConnected
		if connected {
			ws := bot.S("workspace")
			if ws == "" {
				ws = l10n.NotionWorkspace
			}
			status = l10n.NotionConnectedTo(ws)
		}
		IntegrationHeader(c, func() { BrandMark(c, notionLogo, 56, true) }, "Notion", status, l10n.NotionBody, connected)
		if connected {
			p.connected(c, st)
		} else {
			p.connectForm(c)
		}
	})
}

func (p *NotionPage) connectForm(c *ui.Context) {
	cs := design.S()
	tt := design.TT()
	SettingsGroup(c, l10n.NotionCreateIntegration, 0, nil, func() {
		m3.VGap(c, design.SpaceXS)
		SettingsSteps(c, []string{l10n.NotionStep1, l10n.NotionStep2, l10n.NotionStep3}, 1)
		ui.Row(c).FillWidth().Children(func() {
			if (m3.Button{Kind: m3.TextKind, Icon: icons.OpenInNewRounded, Label: l10n.NotionOpenIntegrations}).Show(c).Clicked() {
				widgets.OpenPlainLink(c, notionIntegrationsURL)
			}
		})
		m3.VGap(c, design.SpaceMD)
		tip := l10n.CommonShow
		ic := icons.VisibilityOutlined
		if !p.obscure {
			tip, ic = l10n.CommonHide, icons.VisibilityOffOutlined
		}
		f := (m3.Field{Label: l10n.NotionSecret, Hint: "ntn_…", Password: p.obscure, Error: p.connectError, Helper: l10n.NotionSecretHelper, SuffixFn: func() {
			if m3.IconBtn(c, ic, tip).Clicked() {
				p.obscure = !p.obscure
			}
		}}).Show(c.Key("secret"), &p.token)
		if f.Changed() {
			p.connectError = ""
		}
		empty := strings.TrimSpace(p.token) == ""
		if f.Submitted() && !empty && !p.busy {
			p.connect()
		}
		m3.VGap(c, design.SpaceLG)
		ui.Row(c).FillWidth().Children(func() {
			label := l10n.CommonConnect
			if p.busy {
				label = l10n.CommonConnecting
			}
			if (m3.Button{Kind: m3.Filled, Icon: icons.LinkRounded, Label: label, Disabled: p.busy || empty, Busy: p.busy}).Show(c).Clicked() {
				p.connect()
			}
		})
	})
	m3.Text(c, l10n.NotionShareHint, tt.BodySmall.C(cs.OnSurfaceVariant)).Margin(design.SpaceLG, design.SpaceXS, 0, design.SpaceXS)
}

func (p *NotionPage) connected(c *ui.Context, st core.J) {
	cs := design.S()
	tt := design.TT()
	parent := st.O("parent")
	bot := st.O("bot")
	root := st.S("root_url")
	SettingsGroup(c, l10n.NotionParent, 0, nil, func() {
		if len(parent) == 0 {
			m3.VGap(c, design.SpaceXS)
			SettingsSteps(c, []string{l10n.NotionParentStep1, l10n.NotionParentStep2, l10n.NotionParentStep3}, 1)
			m3.VGap(c, design.SpaceMD)
			ui.Row(c).FillWidth().Children(func() {
				if (m3.Button{Kind: m3.Tonal, Icon: icons.AddLinkRounded, Label: l10n.NotionChoosePage, Disabled: p.busy}).Show(c).Clicked() {
					p.pickParent()
				}
			})
			return
		}
		m3.Tile{Leading: func() { notionPageIcon(c, parent.S("icon")) }, Title: parent.S("title"), Subtitle: l10n.NotionParentHint, Pad: true, PadX: 0,
			Trailing: func() {
				if (m3.Button{Kind: m3.TextKind, Label: l10n.CommonChange, Disabled: p.busy || st.IsTrue("running")}).Show(c).Clicked() {
					p.pickParent()
				}
			}}.Show(c.Key("parent"))
		if strings.HasPrefix(root, "https://") {
			ui.Row(c).FillWidth().Children(func() {
				if (m3.Button{Kind: m3.TextKind, Icon: icons.OpenInNewRounded, Label: l10n.NotionOpenInNotion}).Show(c).Clicked() {
					widgets.OpenPlainLink(c, root)
				}
			})
		}
	})
	if len(parent) > 0 {
		p.syncGroup(c, st)
	}
	SettingsGroup(c, l10n.CommonConnect, 0, nil, func() {
		name := bot.S("name")
		if name == "" {
			name = l10n.NotionIntegration
		}
		m3.Tile{LeadingIcon: icons.KeyOutlined, Title: name, Subtitle: l10n.NotionSecretTail(st.S("token_last4")), Pad: true, PadX: 0, Trailing: func() {
			if (m3.Button{Kind: m3.TextKind, Label: l10n.CommonDisconnect, FG: cs.Error, Disabled: p.busy}).Show(c).Clicked() {
				p.disconnect()
			}
		}}.Show(c.Key("bot"))
	})
	m3.Text(c, l10n.NotionLimits, tt.BodySmall.C(cs.OnSurfaceVariant)).Margin(design.SpaceLG, design.SpaceXS, 0, design.SpaceXS)
}

func (p *NotionPage) syncGroup(c *ui.Context, st core.J) {
	cs := design.S()
	tt := design.TT()
	running := st.IsTrue("running")
	prog := st.O("progress")
	done, total := prog.I("done"), prog.I("total")
	last := st.O("last")
	conflicts, errs := last.Ss("conflicts"), last.Ss("errors")
	errText := last.S("error")
	at := last.I("at")
	SettingsGroup(c, l10n.CommonSync, 0, nil, func() {
		if ch, v := switchTile(c, "auto", m3.Tile{Title: l10n.NotionAutoSync, Subtitle: l10n.NotionAutoSyncHint, Disabled: p.busy}, st.IsTrue("auto")); ch {
			p.do("PUT", "/api/notion", core.J{"auto": v})
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
				text := l10n.CommonChecking
				if total > 0 {
					text = l10n.CommonSyncProgress(done, total)
				}
				m3.Text(c, text, tt.BodySmall.C(cs.OnSurfaceVariant))
			})
		case at != 0:
			ui.Box(c.Key("last")).FillWidth().Padding(0, 0, design.SpaceLG, 0).Children(func() { notionLastSync(c, at, last, errText) })
		}
		if !running && len(conflicts) > 0 {
			notionIssues(c, "conflicts", icons.EditNoteRounded, l10n.NotionConflicts(len(conflicts)), conflicts, false, &p.conflictsOpen)
		}
		if !running && len(errs) > 0 {
			notionIssues(c, "errors", icons.ErrorOutlineRounded, l10n.NotionErrors(len(errs)), errs, true, &p.errorsOpen)
		}
		ui.Row(c).FillWidth().Wrap().Gap(design.SpaceSM).Children(func() {
			label := l10n.CommonSyncNow
			if at == 0 {
				label = l10n.CommonStartSync
			}
			if (m3.Button{Kind: m3.Filled, Icon: icons.SyncRounded, Label: label, Disabled: running || p.busy}).Show(c).Clicked() {
				p.do("POST", "/api/notion/sync", core.J{"full": false})
			}
			if (m3.Button{Kind: m3.Outlined, Label: l10n.NotionFullCheck, Tooltip: l10n.NotionFullCheckHint, Disabled: running || p.busy || at == 0}).Show(c).Clicked() {
				p.do("POST", "/api/notion/sync", core.J{"full": true})
			}
		})
	})
}

func notionLastSync(c *ui.Context, at int, last core.J, errText string) {
	cs := design.S()
	tt := design.TT()
	var counts []string
	if n := last.I("created"); n > 0 {
		counts = append(counts, l10n.SyncCreated(n))
	}
	if n := last.I("updated"); n > 0 {
		counts = append(counts, l10n.SyncUpdated(n))
	}
	if n := last.I("trashed"); n > 0 {
		counts = append(counts, l10n.SyncTrashed(n))
	}
	counts = append(counts, l10n.SyncUnchanged(last.I("unchanged")))
	ui.Row(c).FillWidth().AlignItems(ui.Start).Children(func() {
		ic, col := icons.CheckCircleOutlineRounded, cs.Primary
		line, lineCol := strings.Join(counts, " · "), cs.OnSurfaceVariant
		if errText != "" {
			ic, col = icons.ErrorOutlineRounded, cs.Error
			line, lineCol = errText, cs.Error
		}
		m3.Icon(c, ic, 20, col)
		m3.HGap(c, design.SpaceMD)
		ui.Column(c).Grow(1).Basis(0).Children(func() {
			m3.Text(c, l10n.CommonLastSynced(core.FmtRelative(time.UnixMilli(int64(at)))), tt.BodyMedium)
			m3.Text(c, line, tt.BodySmall.C(lineCol))
		})
	})
}

// notionIssues is the pages kept or failed in the last sync, folded.
func notionIssues(c *ui.Context, key string, ic icons.Icon, title string, items []string, failed bool, open *bool) {
	cs := design.S()
	tt := design.TT()
	col := cs.Tertiary
	if failed {
		col = cs.Error
	}
	ui.Column(c.Key(key)).FillWidth().Margin(0, 0, design.SpaceMD, 0).Background(cs.SurfaceContainerHigh).Radius(design.CornerLG).Clip().Children(func() {
		m3.Expansion{Tile: m3.Tile{LeadingIcon: ic, IconColor: col, TitleFn: func() { m3.Text(c, title, tt.BodyMedium) }, Pad: true, NoRadius: true},
			Open: open, ChildPad: [4]float32{0, design.SpaceLG, design.SpaceMD, design.SpaceLG}, Children: func() {
				for i, s := range items {
					if i == 50 {
						break
					}
					m3.Text(c.Key(i), s, tt.BodySmall.C(cs.OnSurfaceVariant)).Margin(design.SpaceXXS, 0)
				}
				if len(items) > 50 {
					m3.Text(c, l10n.NotionMoreItems(len(items)-50), tt.BodySmall.C(cs.OnSurfaceVariant))
				}
			}}.Show(c, "x")
	})
}

func notionPageIcon(c *ui.Context, icon string) {
	cs := design.S()
	ui.Box(c).Size(40, 40).Shrink(0).Center().Background(cs.SurfaceContainerHighest).Radius(design.CornerMD).Children(func() {
		if icon == "" {
			m3.Icon(c, icons.DescriptionOutlined, 20, cs.OnSurfaceVariant)
			return
		}
		m3.Text(c, icon, design.TT().BodyLarge.S(20))
	})
}

var notionLink = regexp.MustCompile(`(?i)(notion\.(so|site)/|^[0-9a-f]{32}$|^[0-9a-f-]{36}$)`)

// notionPicker lists the pages shared with the integration; a pasted link
// works too.
type notionPicker struct {
	q        string
	pages    []core.J
	loaded   bool
	err      error
	seq      int
	debounce *time.Timer
}

func (n *notionPicker) search() {
	n.seq++
	seq := n.seq
	q := strings.TrimSpace(n.q)
	n.err = nil
	path := "/api/notion/pages"
	if q != "" && !notionLink.MatchString(q) {
		path += "?q=" + core.Esc(q)
	}
	core.Async(func() (any, error) { return api().Get(path) }, func(v any, err error) {
		if seq != n.seq {
			return
		}
		if err != nil {
			n.err = err
			return
		}
		n.pages, n.loaded = core.Objs(v), true
	})
}

func showNotionPagePicker(done func(string)) {
	n := &notionPicker{}
	n.search()
	m3.ShowModalBottomSheet(m3.SheetOptions{ScrollControlled: true, HeightFactor: .72, OnClose: func(v any) {
		if n.debounce != nil {
			n.debounce.Stop()
		}
		if id, ok := v.(string); ok && id != "" {
			done(id)
		}
	}}, func(c *ui.Context, r *m3.Route) {
		cs := design.S()
		tt := design.TT()
		q := strings.TrimSpace(n.q)
		link := notionLink.MatchString(q)
		ui.Column(c).FillWidth().Grow(1).Basis(0).Children(func() {
			m3.Text(c, l10n.NotionChooseParent, tt.TitleLarge).Margin(0, design.SpaceXL, design.SpaceXS, design.SpaceXL)
			m3.Text(c, l10n.NotionChooseParentHint, tt.BodySmall.C(cs.OnSurfaceVariant)).Margin(0, design.SpaceXL, design.SpaceMD, design.SpaceXL)
			ui.Box(c).FillWidth().Padding(0, design.SpaceLG).Children(func() {
				if (m3.SearchBar{Hint: l10n.NotionSearchHint, BG: cs.SurfaceContainerHighest, MaxWidth: -1,
					Leading: func() { m3.Icon(c, icons.SearchRounded, 24, cs.OnSurfaceVariant) }}).Show(c, &n.q).Changed() {
					if n.debounce != nil {
						n.debounce.Stop()
					}
					n.debounce = time.AfterFunc(350*time.Millisecond, func() { core.Post(n.search) })
				}
			})
			m3.VGap(c, design.SpaceSM)
			ui.Box(c).FillWidth().Grow(1).Basis(0).Children(func() {
				switch {
				case n.err != nil:
					widgets.ErrorState(c, n.err, n.search)
				case !n.loaded:
					widgets.CenteredLoader(c, "")
				default:
					ui.Scroll(c).Fill().Children(func() {
						ui.Column(c).FillWidth().Padding(design.SpaceSM).Children(func() {
							if link {
								if (m3.Tile{LeadingIcon: icons.LinkRounded, Title: l10n.NotionUseLink, Subtitle: q, SubtitleLines: 1, Tap: true, Radius: design.CornerFull}).Show(c.Key("link")).Clicked() {
									r.Pop(q)
								}
							}
							for _, pg := range n.pages {
								pg := pg
								title := pg.S("title")
								if title == "" {
									title = l10n.CommonUntitled
								}
								if (m3.Tile{Leading: func() { notionPageIcon(c, pg.S("icon")) }, Title: title, Tap: true, Radius: design.CornerFull}).Show(c.Key(pg.S("id"))).Clicked() {
									r.Pop(pg.S("id"))
								}
							}
							if len(n.pages) == 0 && !link {
								ui.Column(c).FillWidth().Padding(design.SpaceXL).AlignItems(ui.Center).Children(func() {
									title := l10n.NotionNoPages
									if q != "" {
										title = l10n.NotionNoMatch(q)
									}
									m3.Text(c, title, tt.TitleSmall)
									m3.VGap(c, design.SpaceSM)
									m3.Text(c, l10n.NotionShareSteps, tt.BodySmall.C(cs.OnSurfaceVariant)).TextAlign(ui.Center)
									m3.VGap(c, design.SpaceMD)
									if (m3.Button{Kind: m3.TextKind, Icon: icons.RefreshRounded, Label: l10n.CommonRefresh}).Show(c).Clicked() {
										n.search()
									}
								})
							}
						})
					})
				}
			})
		})
	})
}
