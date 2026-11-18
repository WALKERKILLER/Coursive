package app

import (
	"regexp"
	"sort"
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

// showSourceDialog installs a plugin from a folder, npm or Git.
func showSourceDialog() {
	kind, text := 0, ""
	hints := []string{"/home/me/my-plugin", "@scope/coursive-plugin@1.0.0", "https://github.com/me/plugin#v1.0.0"}
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		m3.AlertDialog(c, m3.Alert{Title: l10n.PluginsInstallOther, Width: 440, Content: func() {
			ui.Column(c).FillWidth().Children(func() {
				m3.Segmented(c, []m3.Segment[int]{{Value: 0, Label: l10n.CommonFolder, Icon: icons.FolderOutlined}, {Value: 1, Label: "npm"}, {Value: 2, Label: "Git"}},
					&kind, m3.SegOpts{Fill: true})
				m3.VGap(c, design.SpaceLG)
				f := m3.Field{Hint: hints[kind], AutoFocus: true}
				if kind == 0 {
					f.SuffixFn = func() {
						if m3.IconBtn(c, icons.FolderOpenOutlined, l10n.PickerChooseFolder).Clicked() {
							widgets.PickDirectory(strings.TrimSpace(text), "", l10n.PickerChooseFolder, func(dir string) {
								if dir != "" {
									text = dir
								}
							})
						}
					}
				} else {
					f.Helper = l10n.PluginsPinnedVersion
				}
				f.Show(c, &text)
			})
		}, Actions: func() {
			if m3.TextButton(c, l10n.CommonCancel).Clicked() {
				r.Close()
			}
			v := strings.TrimSpace(text)
			if (m3.Button{Kind: m3.Filled, Label: l10n.CommonNext, Disabled: v == ""}).Show(c).Clicked() {
				source := []string{"path:", "npm:", "git:"}[kind] + v
				r.Close()
				InstallPlugin(source, nil)
			}
		}})
	}, nil)
}

// showCatalogsDialog lists the added catalogs, with their load errors, and
// adds or removes one.
func showCatalogsDialog(onDone func()) {
	var catalogs []core.J
	var errs []core.J
	busy := false
	link := ""
	load := func() {
		core.Async(Plugins.Catalog, func(r core.J, err error) {
			if err == nil {
				catalogs, errs = r.Os("catalogs"), r.Os("errors")
			}
		})
	}
	run := func(work func() error, then func()) {
		busy = true
		core.Go(work, func(err error) {
			busy = false
			if err != nil {
				widgets.ShowSnack(apiMessage(err), "", nil)
				return
			}
			if then != nil {
				then()
			}
			load()
		})
	}
	load()
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		cs := design.S()
		m3.AlertDialog(c, m3.Alert{Title: l10n.PluginsCatalog, Width: 480, Content: func() {
			ui.Column(c).FillWidth().Children(func() {
				(m3.Tile{LeadingIcon: icons.VerifiedRounded, Title: l10n.PluginsOfficial, Subtitle: l10n.PluginsBundled, Pad: true}).Show(c.Key("official"))
				for _, cat := range catalogs {
					u := cat.S("url")
					t := m3.Tile{LeadingIcon: icons.PublicRounded, Title: u, TitleLines: 1, Pad: true, Trailing: func() {
						if (m3.IconButton{Icon: icons.DeleteOutlineRounded, Tooltip: l10n.CommonRemove, Disabled: busy}).Show(c).Clicked() {
							run(func() error { return Plugins.RemoveCatalog(u) }, nil)
						}
					}}
					for _, e := range errs {
						if e.S("url") == u {
							msg := l10n.PluginsLoadFailed(e["error"])
							t.SubtitleFn = func() { m3.Text(c, msg, design.TT().BodyMedium.C(cs.Error)) }
							break
						}
					}
					t.Show(c.Key(u))
				}
				m3.VGap(c, design.SpaceSM)
				(m3.Field{Hint: l10n.PluginsCatalogHint, SuffixFn: func() {
					if (m3.IconButton{Icon: icons.AddRounded, Tooltip: l10n.CommonAdd, Disabled: busy}).Show(c).Clicked() {
						v := strings.TrimSpace(link)
						run(func() error { return Plugins.AddCatalog(v) }, func() { link = "" })
					}
				}}).Show(c, &link)
			})
		}, Actions: func() {
			if m3.TextButton(c, l10n.CommonFinished).Clicked() {
				r.Close()
			}
		}})
	}, func(any) {
		if onDone != nil {
			onDone()
		}
	})
}

// showBusy is a dialog with a spinner that only the caller closes.
func showBusy(label string) *m3.Route {
	return m3.ShowModalDialog(func(c *ui.Context, r *m3.Route) {
		m3.AlertDialog(c, m3.Alert{Content: func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				m3.Circular(c, m3.Progress{})
				m3.HGap(c, design.SpaceXL)
				m3.Text(c, label, design.TT().BodyMedium).Grow(1).Basis(0)
			})
		}})
	}, nil)
}

// InstallPlugin fetches source, asks for consent with the exact
// permissions, and installs; done reports whether it did.
func InstallPlugin(source string, done func(bool)) {
	finish := func(ok bool) {
		if done != nil {
			done(ok)
		}
	}
	busy := showBusy(l10n.PluginsFetching)
	core.Async(func() (core.J, error) { return Plugins.Preview(source) }, func(preview core.J, err error) {
		busy.Close()
		if err != nil {
			widgets.ShowSnack(l10n.PluginsFetchFailed(apiMessage(err)), "", nil)
			finish(false)
			return
		}
		showConsent(preview, func(ok bool) {
			if !ok {
				finish(false)
				return
			}
			busy := showBusy(l10n.PluginsInstalling)
			core.Async(func() (core.J, error) { return Plugins.Confirm(preview.S("token")) }, func(p core.J, err error) {
				busy.Close()
				if err != nil {
					widgets.ShowSnack(l10n.PluginsInstallFailed(apiMessage(err)), "", nil)
					finish(false)
					return
				}
				Plugins.Replace(p)
				if p.S("state") == "error" {
					widgets.ShowSnack(l10n.PluginsInstalledLoadFailed(p["error"]), "", nil)
				} else {
					widgets.ShowSnack(l10n.PluginsInstalledToast(p["title"]), "", nil)
				}
				finish(true)
			})
		})
	})
}

// showConsent lists exactly what an install or update grants before
// anything happens.
func showConsent(preview core.J, done func(bool)) {
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		cs := design.S()
		tt := design.TT()
		m := objOrEmpty(preview["manifest"])
		update := preview.O("update")
		added := map[string]bool{}
		for _, p := range update.L("added_permissions") {
			added[core.Str(p)] = true
		}
		perms := m.Os("permissions")
		official := preview.IsTrue("official")
		title := l10n.PluginsInstallTitle(m["title"])
		if update != nil {
			title = l10n.PluginsUpdateTitle(m["title"])
		}
		dialog := m3.Dialog(c, func() {
			ui.Scroll(c).Shrink(1).Children(func() {
				ui.Column(c).Width(440+48).Padding(24, 24, 0, 24).Children(func() {
					ui.Row(c).FillWidth().Justify(ui.Center).Children(func() { widgets.PluginGlyph{Icon: m.S("icon"), Size: 56}.Show(c) })
					m3.VGap(c, design.SpaceLG)
					m3.Text(c, title, tt.HeadlineSmall).FillWidth().TextAlign(ui.Center)
					m3.VGap(c, design.SpaceLG)
					var meta []string
					switch {
					case official:
						meta = append(meta, l10n.PluginsOfficial)
					case m.S("author") != "":
						meta = append(meta, m.S("author"))
					}
					meta = append(meta, "v"+m.S("version"))
					if update != nil {
						meta = append(meta, l10n.PluginsCurrentVersion(update["from"]))
					}
					m3.Text(c, strings.Join(meta, " · "), tt.BodySmall.C(cs.OnSurfaceVariant)).FillWidth().TextAlign(ui.Center)
					if !official {
						m3.VGap(c, design.SpaceLG)
						ui.Row(c).FillWidth().AlignItems(ui.Start).Padding(design.SpaceMD).Background(cs.TertiaryContainer).Radius(design.CornerMD).Children(func() {
							m3.Icon(c, icons.ShieldOutlined, 18, cs.OnTertiaryContainer)
							m3.HGap(c, design.SpaceSM)
							m3.Text(c, l10n.PluginsThirdPartyWarning(preview["source"]), tt.BodySmall.C(cs.OnTertiaryContainer)).Grow(1).Basis(0)
						})
					}
					m3.VGap(c, design.SpaceLG)
					head := l10n.PluginsWillBeAble
					if len(perms) == 0 {
						head = l10n.PluginsNoPermissions
					}
					m3.Text(c, head, tt.TitleSmall)
					for i, p := range perms {
						id := p.S("id")
						sensitive := core.SensitivePermissions[id]
						col := cs.OnSurfaceVariant
						var ts *design.TextStyle
						if sensitive {
							col = cs.Tertiary
							s := tt.BodyMedium.W(600)
							ts = &s
						}
						t := m3.Tile{LeadingIcon: permissionIcon(id), IconColor: col, Title: p.S("label"), TitleStyle: ts, Dense: true, Pad: true}
						if d, ok := p["detail"]; ok && d != nil {
							t.Subtitle = core.Str(d)
						}
						if added[id] {
							t.Trailing = func() { m3.Text(c, l10n.PluginsNewPermissions, tt.LabelSmall.C(cs.Tertiary)) }
						}
						t.Show(c.Key(i))
					}
				})
			})
			ui.Row(c).Justify(ui.End).Gap(8).Padding(24, 24, 24, 24).Children(func() {
				if m3.TextButton(c, l10n.CommonCancel).Clicked() {
					r.Pop(false)
				}
				label := l10n.CommonInstall
				if update != nil {
					label = l10n.MemoryUpdated
				}
				if m3.FilledButton(c, label).Clicked() {
					r.Pop(true)
				}
			})
		})
		_ = dialog
	}, func(v any) { done(v == true) })
}

// pluginDetailPage is one plugin on its own page, pushed over the list on
// narrow windows.
type pluginDetailPage struct{ d *PluginDetailView }

func newPluginDetailPage(entry core.J, onChanged func()) *pluginDetailPage {
	pg := &pluginDetailPage{d: &PluginDetailView{Entry: entry, OnChanged: onChanged}}
	// A plugin that is in no catalog has nothing left to show once removed.
	if _, ok := entry["catalog"]; !ok || entry["catalog"] == nil {
		pg.d.OnUninstalled = func() { m3.Nav().PopPage(pg) }
	}
	return pg
}

func (pg *pluginDetailPage) View(c *ui.Context) {
	m3.Scaffold{AppBar: &m3.AppBar{}, Body: func() { pg.d.Show(c) }}.Show(c)
}

// PluginDetailView is what a plugin does, what it may do, where it appears
// and its settings.
type PluginDetailView struct {
	// Entry is a catalog listing or an installed plugin.
	Entry core.J
	// Embedded shows it in the pane beside a list rather than as its own
	// page.
	Embedded                          bool
	OnClose, OnChanged, OnUninstalled func()
	busy                              bool
	form                              *pluginSettingsForm
	formKey                           string
	scroll                            ui.ScrollState
}

func (d *PluginDetailView) run(work func() error, then func()) {
	d.busy = true
	core.Go(work, func(err error) {
		d.busy = false
		if err != nil {
			widgets.ShowSnack(apiMessage(err), "", nil)
		} else if then != nil {
			then()
		}
		if d.OnChanged != nil {
			d.OnChanged()
		}
	})
}

func (d *PluginDetailView) update(id string, body core.J, then func()) {
	var r core.J
	d.run(func() (err error) { r, err = Plugins.Update(id, body); return }, func() {
		Plugins.Replace(r)
		if then != nil {
			then()
		}
	})
}

// Show builds the view.
func (d *PluginDetailView) Show(c *ui.Context) {
	cs := design.S()
	tt := design.TT()
	id := d.Entry.S("id")
	installed := Plugins.ByID(id)
	p := d.Entry
	if installed != nil {
		p = installed
	}
	state := installed.S("state")
	update := installed != nil && d.Entry.IsTrue("update")
	source, hasSource := d.Entry["source"].(string)
	perms := p.Os("permissions")
	contributes := p.O("contributes")
	type place struct {
		title, where, icon string
	}
	var places []place
	for _, a := range contributes.Os("actions") {
		places = append(places, place{a.S("title"), pluginSurfaceLabel(a.S("surface")), a.S("icon")})
	}
	for _, a := range contributes.Os("noteViews") {
		places = append(places, place{a.S("title"), l10n.PluginsContribAlternative, a.S("icon")})
	}
	for _, a := range contributes.Os("exporters") {
		places = append(places, place{a.S("title"), l10n.PluginsContribExport(pluginScopeLabel(a.S("scope"))), a.S("icon")})
	}
	pages := contributes.Os("pages")
	for _, a := range pages {
		places = append(places, place{a.S("title"), l10n.PluginsContribPage, a.S("icon")})
	}
	examples := p.L("examples")
	schema := p.O("settings")
	hasSettings := installed != nil && len(schema.O("properties")) > 0
	tools, skills := installed.Os("tools"), installed.Os("skills")
	official := p.IsTrue("official")
	homepage := p.S("homepage")
	tile := cs.SurfaceContainerLow
	if d.Embedded {
		tile = cs.SurfaceContainer
	}
	pad := float32(design.SpaceXL)
	if Compact() {
		pad = design.SpaceLG
	}
	var meta []string
	if official {
		meta = append(meta, l10n.PluginsOfficial)
	} else if a := p.S("author"); a != "" {
		meta = append(meta, a)
	}
	if v := p.S("version"); v != "" {
		meta = append(meta, "v"+v)
	}
	if update {
		meta = append(meta, l10n.PluginsVersionAvailable(d.Entry["version"]))
	}
	section := func(key, title string, ic icons.Icon, children func()) {
		ui.Column(c.Key(key)).FillWidth().Padding(design.SpaceXL, 0, 0, 0).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Padding(0, 0, design.SpaceSM, design.SpaceXS).Children(func() {
				m3.Icon(c, ic, 18, cs.Primary)
				m3.HGap(c, design.SpaceSM)
				m3.Text(c, title, tt.TitleSmall.C(cs.Primary))
			})
			ui.Column(c).FillWidth().Background(tile).Radius(design.CornerXL).Padding(design.SpaceSM, design.SpaceLG).Children(children)
		})
	}
	top := float32(design.SpaceSM)
	if d.Embedded {
		top = design.SpaceXL
	}
	ui.Scroll(c).Fill().TrackScroll(&d.scroll).Children(func() {
		ui.Column(c).FillWidth().Padding(top, pad, design.SpaceXXXL, pad).Children(func() {
			widgets.ContentWidth(c, 760, func() {
				ui.Row(c).FillWidth().AlignItems(ui.Start).Children(func() {
					size := float32(80)
					if d.Embedded {
						size = 64
					}
					widgets.PluginGlyph{Icon: p.S("icon"), ID: id, Size: size, Muted: state == "disabled"}.Show(c)
					m3.HGap(c, design.SpaceLG)
					ui.Column(c).Grow(1).Basis(0).Children(func() {
						m3.Text(c, p.S("title"), tt.HeadlineSmall)
						m3.VGap(c, design.SpaceXS)
						ui.Row(c).FillWidth().Wrap().Gap(design.SpaceSM).AlignItems(ui.Center).Children(func() {
							if installed != nil {
								pluginPill(c, installed, update)
							}
							ui.Row(c).AlignItems(ui.Center).Shrink(1).Children(func() {
								if official {
									m3.Icon(c, icons.VerifiedRounded, 16, cs.Primary)
									m3.HGap(c, design.SpaceXS)
								}
								m3.Text(c, strings.Join(meta, " · "), tt.BodyMedium.C(cs.OnSurfaceVariant)).Shrink(1)
							})
						})
					})
					if installed != nil && state != "needs_consent" {
						on := installed.IsTrue("enabled")
						tip := l10n.CommonEnable
						if on {
							tip = l10n.CommonDisable
						}
						sw := m3.Switch(c, &on, d.busy)
						m3.Tip(c, sw, tip)
						if sw.Changed() {
							d.update(id, core.J{"enabled": on}, nil)
						}
					}
					if d.OnClose != nil && m3.IconBtn(c, icons.CloseRounded, l10n.CommonClose).Clicked() {
						d.OnClose()
					}
				})
				m3.VGap(c, design.SpaceLG)
				m3.Text(c, p.S("summary"), tt.BodyLarge)
				m3.VGap(c, design.SpaceLG)
				ui.Row(c).FillWidth().Wrap().Gap(design.SpaceSM).Children(func() {
					if installed == nil && (m3.Button{Kind: m3.Filled, Icon: icons.DownloadRounded, Label: l10n.CommonInstall, Disabled: d.busy || !hasSource}).Show(c).Clicked() {
						d.busy = true
						InstallPlugin(source, func(bool) {
							d.busy = false
							if d.OnChanged != nil {
								d.OnChanged()
							}
						})
					}
					if state == "needs_consent" && (m3.Button{Kind: m3.Filled, Icon: icons.VerifiedUserOutlined, Label: l10n.PluginsReviewGrant, Disabled: d.busy}).Show(c).Clicked() {
						d.consentUpdate(installed)
					}
					if update && (m3.Button{Kind: m3.Filled, Icon: icons.UpgradeRounded, Label: l10n.MemoryUpdated, Disabled: d.busy}).Show(c).Clicked() {
						d.busy = true
						InstallPlugin(source, func(bool) {
							d.busy = false
							if d.OnChanged != nil {
								d.OnChanged()
							}
						})
					}
					if state == "enabled" {
						for i, pg := range pages {
							pg := pg
							if (m3.Button{Kind: m3.Tonal, Icon: widgets.PluginIcon(pg.S("icon")), Label: pg.S("title")}).Show(c.Key(i)).Clicked() {
								e := core.NewPluginEntry(installed, "pages", pg)
								if page := newPluginPage(e); page.HostRendered() || page.Renderer == "web-ui" {
									m3.RootNav.Push(newPluginPageOutlet(page))
								} else {
									runPluginEntry(e, core.J{}, nil)
								}
							}
						}
					}
					if installed != nil && (m3.Button{Kind: m3.Outlined, Icon: icons.DeleteOutlineRounded, Label: l10n.CommonUninstall, Disabled: d.busy}).Show(c).Clicked() {
						d.uninstall(installed)
					}
					if strings.HasPrefix(homepage, "https://") && (m3.Button{Kind: m3.TextKind, Icon: icons.OpenInNewRounded, Label: l10n.PluginsHomepage}).Show(c).Clicked() {
						OpenLink(homepage)
					}
				})
				if state == "error" {
					m3.VGap(c, design.SpaceLG)
					ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(design.SpaceMD).Background(cs.ErrorContainer).Radius(design.CornerLG).Children(func() {
						m3.Icon(c, icons.ErrorOutlineRounded, 24, cs.OnErrorContainer)
						m3.HGap(c, design.SpaceSM)
						m3.Text(c, installed.S("error"), tt.BodySmall.C(cs.OnErrorContainer)).Selectable().Grow(1).Basis(0)
					})
				}
				if desc := p.S("description"); desc != "" {
					m3.VGap(c, design.SpaceXL)
					m3.Text(c, desc, tt.BodyLarge.H(1.6).C(cs.OnSurfaceVariant))
				}
				if len(examples) > 0 {
					m3.VGap(c, design.SpaceXL)
					head := l10n.PluginsTryAfterEnable
					if state == "enabled" {
						head = l10n.PluginsTryAsking
					}
					m3.Text(c, head, tt.TitleSmall)
					m3.VGap(c, design.SpaceSM)
					ui.Row(c).FillWidth().Wrap().Gap(design.SpaceSM).Children(func() {
						for i, e := range examples {
							s := core.Str(e)
							if state == "enabled" {
								if m3.ActionChipOf(c.Key(i), icons.AutoAwesomeOutlined, s) {
									AskInSheet(s)
								}
							} else {
								(m3.Chip{Kind: m3.PlainChip, Label: s, Avatar: icons.FormatQuoteRounded}).Show(c.Key(i))
							}
						}
					})
				}
				if len(tools)+len(skills) > 0 {
					section("caps", l10n.PluginsCapabilities, icons.AutoAwesomeOutlined, func() {
						for i, t := range tools {
							(m3.Tile{LeadingIcon: icons.BuildOutlined, Title: t.S("label"), Subtitle: t.S("name"), Pad: true}).Show(c.Key("t" + strconv.Itoa(i)))
						}
						for i, k := range skills {
							(m3.Tile{LeadingIcon: icons.MenuBookOutlined, Title: k.S("name"), Subtitle: k.S("description"), Pad: true}).Show(c.Key("k" + strconv.Itoa(i)))
						}
					})
				}
				if len(places) > 0 {
					section("places", l10n.PluginsAppearsIn, icons.WidgetsOutlined, func() {
						for i, pl := range places {
							(m3.Tile{LeadingIcon: widgets.PluginIcon(pl.icon), Title: pl.title, Subtitle: pl.where, Pad: true}).Show(c.Key(i))
						}
					})
				}
				section("perms", l10n.PluginsPermissions, icons.ShieldOutlined, func() {
					if len(perms) == 0 {
						(m3.Tile{LeadingIcon: icons.LockOutlineRounded, Title: l10n.PluginsNoPermissions, Subtitle: l10n.PluginsGuidanceOnly, Pad: true}).Show(c)
						return
					}
					for i, x := range perms {
						t := m3.Tile{LeadingIcon: permissionIcon(x.S("id")), Title: x.S("label"), Pad: true}
						if core.SensitivePermissions[x.S("id")] {
							t.IconColor = cs.Tertiary
						}
						if v, ok := x["detail"]; ok && v != nil {
							t.Subtitle = core.Str(v)
						}
						t.Show(c.Key(i))
					}
				})
				if hasSettings {
					key := id + "-" + installed.S("version")
					if d.form == nil || d.formKey != key {
						d.form, d.formKey = newPluginSettingsForm(schema, installed.O("settings_values")), key
					}
					section("settings", l10n.CommonSettings, icons.TuneRounded, func() {
						d.form.Show(c, func(v core.J) {
							d.update(id, core.J{"settings": v}, func() { widgets.ShowSnack(l10n.PluginsSettingsSaved, "", nil) })
						})
					})
				}
				m3.VGap(c, design.SpaceXL)
				ui.Row(c).FillWidth().Wrap().Gap(design.SpaceLG).Children(func() {
					if lic := p.S("license"); lic != "" {
						m3.Text(c, l10n.CommonLicense(lic), tt.BodySmall.C(cs.OnSurfaceVariant))
					}
					if src := p.S("source"); src != "" {
						m3.Text(c, l10n.PluginsSource(src), tt.BodySmall.C(cs.OnSurfaceVariant))
					}
				})
			})
		})
	})
}

func (d *PluginDetailView) consentUpdate(p core.J) {
	var added []any
	for _, x := range p.Os("needs_permissions") {
		added = append(added, x["id"])
	}
	showConsent(core.J{"official": p["official"], "source": p["source"], "manifest": map[string]any(p),
		"update": map[string]any{"from": p["version"], "added_permissions": added}}, func(ok bool) {
		if ok {
			d.update(p.S("id"), core.J{"grant": true}, nil)
		}
	})
}

func (d *PluginDetailView) uninstall(p core.J) {
	id := p.S("id")
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		m3.AlertDialog(c, m3.Alert{Title: l10n.PluginsUninstallTitle(p["title"]), Message: l10n.PluginsUninstallBody, Actions: func() {
			if m3.TextButton(c, l10n.CommonCancel).Clicked() {
				r.Pop(false)
			}
			if m3.FilledButton(c, l10n.CommonUninstall).Clicked() {
				r.Pop(true)
			}
		}})
	}, func(v any) {
		if v != true {
			return
		}
		d.run(func() error { return Plugins.Uninstall(id) }, func() {
			Plugins.Removed(id)
			if d.OnUninstalled != nil {
				d.OnUninstalled()
			}
		})
	})
}

// pluginSettingsForm is a form for a manifest's settings schema (the JSON
// Schema subset).
type pluginSettingsForm struct {
	props  []string
	schema map[string]core.J
	values core.J
	text   map[string]*string
	errs   map[string]string
}

func newPluginSettingsForm(schema, values core.J) *pluginSettingsForm {
	f := &pluginSettingsForm{schema: map[string]core.J{}, values: core.J{}, text: map[string]*string{}, errs: map[string]string{}}
	for k, v := range values {
		f.values[k] = v
	}
	props := schema.O("properties")
	// Decoding loses the manifest's key order, so the fields go by key.
	for k := range props {
		f.props = append(f.props, k)
	}
	sort.Strings(f.props)
	for _, k := range f.props {
		p := props.O(k)
		f.schema[k] = p
		if p.S("type") == "boolean" || p["enum"] != nil {
			continue
		}
		cur, ok := f.values[k]
		if !ok || cur == nil {
			cur = p["default"]
		}
		s := ""
		switch v := cur.(type) {
		case nil:
		case []any:
			var parts []string
			for _, x := range v {
				parts = append(parts, plainString(x))
			}
			s = strings.Join(parts, ", ")
		default:
			s = plainString(v)
		}
		f.text[k] = &s
	}
	return f
}

func plainString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	}
	return core.Str(v)
}

var settingsSplit = regexp.MustCompile(`[,，\s]+`)

func (f *pluginSettingsForm) parse(p core.J, s string) (any, bool) {
	s = strings.TrimSpace(s)
	num := func(s string, integer bool) (any, bool) {
		if integer {
			n, err := strconv.Atoi(s)
			return float64(n), err == nil
		}
		n, err := strconv.ParseFloat(s, 64)
		return n, err == nil
	}
	switch p.S("type") {
	case "integer":
		return num(s, true)
	case "number":
		return num(s, false)
	case "array":
		t := p.O("items").S("type")
		out := []any{}
		for _, x := range settingsSplit.Split(s, -1) {
			if x == "" {
				continue
			}
			if t == "string" {
				out = append(out, x)
				continue
			}
			v, ok := num(x, t == "integer")
			if !ok {
				return nil, false
			}
			out = append(out, v)
		}
		return out, true
	}
	return s, true
}

func (f *pluginSettingsForm) save(onSave func(core.J)) {
	out := core.J{}
	for k, v := range f.values {
		out[k] = v
	}
	f.errs = map[string]string{}
	for _, k := range f.props {
		t, ok := f.text[k]
		if !ok {
			continue
		}
		p := f.schema[k]
		v, ok := f.parse(p, *t)
		n, isNum := v.(float64)
		_, hasMin := p["minimum"].(float64)
		_, hasMax := p["maximum"].(float64)
		switch {
		case !ok:
			kind := l10n.PluginsSettingNumber
			if p.S("type") == "string" {
				kind = l10n.PluginsSettingText
			}
			f.errs[k] = l10n.PluginsSettingRequired(kind)
		case isNum && ((hasMin && n < p.F("minimum")) || (hasMax && n > p.F("maximum"))):
			lo, hi := any(""), any("")
			if hasMin {
				lo = plainString(p["minimum"])
			}
			if hasMax {
				hi = plainString(p["maximum"])
			}
			f.errs[k] = l10n.PluginsSettingRange(lo, hi)
		default:
			out[k] = v
		}
	}
	if len(f.errs) == 0 {
		onSave(out)
	}
}

// Show builds the form; onSave gets the values once they are valid.
func (f *pluginSettingsForm) Show(c *ui.Context, onSave func(core.J)) {
	tt := design.TT()
	cs := design.S()
	ui.Column(c).FillWidth().Children(func() {
		for _, k := range f.props {
			k := k
			p := f.schema[k]
			title := p.Or("title", k)
			desc, hasDesc := p["description"].(string)
			cur, ok := f.values[k]
			if !ok || cur == nil {
				cur = p["default"]
			}
			switch {
			case p.S("type") == "boolean":
				on := cur == true
				t := m3.Tile{Title: title, Pad: true}
				if hasDesc {
					t.Subtitle = desc
				}
				if m3.SwitchTile(c.Key(k), t, &on) {
					f.values[k] = on
				}
			case p["enum"] != nil:
				ui.Column(c.Key(k)).FillWidth().Padding(design.SpaceSM, 0).Children(func() {
					m3.Text(c, title, tt.BodyLarge)
					if hasDesc {
						m3.Text(c, desc, tt.BodyMedium.C(cs.OnSurfaceVariant))
					}
					m3.VGap(c, design.SpaceSM)
					ui.Row(c).FillWidth().Wrap().Gap(design.SpaceSM).Children(func() {
						for i, o := range p.L("enum") {
							o := o
							if m3.ChoiceChipOf(c.Key(i), plainString(o), cur == o) {
								f.values[k] = o
							}
						}
					})
				})
			default:
				fld := m3.Field{Label: title, Error: f.errs[k]}
				if hasDesc {
					fld.Helper = desc
				}
				if p.S("format") == "path" {
					fld.SuffixFn = func() {
						if m3.IconBtn(c, icons.FolderOpenOutlined, l10n.PickerChooseFolder).Clicked() {
							widgets.PickDirectory(strings.TrimSpace(*f.text[k]), "", title, func(dir string) {
								if dir != "" {
									*f.text[k] = dir
								}
							})
						}
					}
				}
				ui.Column(c.Key(k)).FillWidth().Padding(design.SpaceSM, 0).Children(func() {
					in := fld.Show(c, f.text[k])
					if p.S("type") == "integer" && in.Changed() {
						*f.text[k] = strings.Map(func(r rune) rune {
							if r >= '0' && r <= '9' {
								return r
							}
							return -1
						}, *f.text[k])
					}
				})
			}
		}
		m3.VGap(c, design.SpaceSM)
		ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
			if (m3.Button{Kind: m3.Tonal, Label: l10n.PluginsSaveSettings}).Show(c).Clicked() {
				f.save(onSave)
			}
		})
	})
}
