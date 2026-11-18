package app

import (
	"fmt"
	"os"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// The 数据与同步 pane: sync between devices, backups, and what takes space.
// Contract: docs/specs/0014-data-sync-and-backup.md

var backupFilter = core.FileFilter{Name: l10n.BackupTypeLabel, Extensions: []string{"zip"}, MIMETypes: []string{"application/zip"}}

// DataSettings is the 数据与同步 pane.
type DataSettings struct {
	sync, estimate, storage core.J
	media                   bool
	// task is the export, restore or clean in progress, with bytes so
	// far and of how many.
	task        string
	done, total int64
	cancel      func()
}

// NewDataSettings loads the pane and follows sync events.
func NewDataSettings() *DataSettings {
	d := &DataSettings{sync: core.J{}}
	d.load()
	d.cancel = State.Events.Listen(func(e core.J) {
		switch t := e.S("type"); {
		case t == "sync_done" || t == "sync_changed":
			d.load()
		case t == "sync_progress" && !d.sync.IsTrue("running"):
			s := d.sync.Copy()
			s["running"] = true
			d.sync = s
		}
	})
	return d
}

// Dispose stops following the events.
func (d *DataSettings) Dispose() { d.cancel() }

func (d *DataSettings) load() {
	for _, x := range []struct {
		path string
		set  func(core.J)
	}{
		{"/api/sync", func(j core.J) { d.sync = j }},
		{"/api/data/estimate", func(j core.J) { d.estimate = j }},
		{"/api/storage", func(j core.J) { d.storage = j }},
	} {
		x := x
		core.Async(func() (any, error) { return api().Get(x.path) }, func(v any, err error) {
			if err != nil {
				v = core.J{}
			}
			if j := core.Obj(v); j != nil {
				x.set(j)
			} else {
				x.set(core.J{})
			}
		})
	}
}

func (d *DataSettings) export() {
	now := time.Now()
	name := l10n.BackupFileName(now.Year(), fmt.Sprintf("%02d", int(now.Month())), fmt.Sprintf("%02d", now.Day()))
	widgets.PickSavePath(name, backupFilter, l10n.CommonExport, func(path string) {
		if path == "" {
			return
		}
		e := d.estimate
		d.task, d.done = l10n.CommonExporting, 0
		d.total = int64(e.F("bytes"))
		if d.media {
			d.total += int64(e.F("media_bytes"))
		}
		q := map[string]string{}
		if d.media {
			q["media"] = "1"
		}
		core.Async(func() (int64, error) {
			return api().Download("/api/data/export", q, path, func(n int64) { core.Post(func() { d.done = n }) })
		}, func(n int64, err error) {
			d.task = ""
			if err != nil {
				widgets.ShowSnack(l10n.BackupExportFailed(apiMessage(err)), "", nil)
				return
			}
			widgets.ShowSnack(l10n.BackupSaved(core.FmtBytes(float64(n))), l10n.CommonShowInFolder, func() { go core.RevealPath(path) })
		})
	})
}

// progressReader reports what a reader has given so far.
type progressReader struct {
	f    *os.File
	sent int64
	on   func(int64)
}

func (r *progressReader) Read(b []byte) (int, error) {
	n, err := r.f.Read(b)
	r.sent += int64(n)
	r.on(r.sent)
	return n, err
}

func (d *DataSettings) restore() {
	core.PickFile(l10n.BackupRestoreFrom, []core.FileFilter{backupFilter}, func(path string) {
		if path == "" {
			return
		}
		f, err := os.Open(path)
		if err != nil {
			widgets.ShowSnack(l10n.CommonFilePickerFailed(err.Error()), "", nil)
			return
		}
		st, _ := f.Stat()
		length := st.Size()
		d.task, d.done, d.total = l10n.BackupReading, 0, length
		core.Async(func() (any, error) {
			defer f.Close()
			return api().Upload("/api/data/import", nil, &progressReader{f: f, on: func(n int64) { core.Post(func() { d.done = n }) }}, length)
		}, func(v any, err error) {
			d.task = ""
			if err != nil {
				widgets.ShowSnack(apiMessage(err), "", nil)
				return
			}
			preview := core.Obj(v)
			id := preview.S("id")
			showRestoreDialog(preview, func(mode string) {
				if mode == "" {
					go api().Delete("/api/data/import/" + core.Esc(id))
					return
				}
				d.apply(id, mode)
			})
		})
	})
}

func (d *DataSettings) apply(id, mode string) {
	d.task, d.done, d.total = l10n.BackupRestoring, 0, 0
	core.Async(func() (any, error) {
		return api().PostWait("/api/data/import/"+core.Esc(id)+"/apply", core.J{"mode": mode}, 30*time.Minute)
	}, func(v any, err error) {
		d.task = ""
		if err != nil {
			widgets.ShowSnack(l10n.BackupRestoreFailed(apiMessage(err)), "", nil)
			return
		}
		r := core.Obj(v)
		conflict := r.S("conflict")
		msg := l10n.BackupNothingToRestore
		if r.I("written") != 0 {
			msg = l10n.BackupRestored(r["written"], core.FmtBytes(r.F("bytes")))
		}
		if conflict != "" && core.CanOpenFolders() {
			widgets.ShowSnack(msg, l10n.BackupViewReplaced, func() { go core.RevealPath(conflict) })
		} else {
			widgets.ShowSnack(msg, "", nil)
		}
		d.load()
	})
}

func (d *DataSettings) clean(what, doing string) {
	run := func() {
		d.task = doing
		core.Async(func() (any, error) { return api().Post("/api/storage/clean", core.J{what: true}) }, func(v any, err error) {
			d.task = ""
			if err != nil {
				widgets.ShowSnack(l10n.StorageCleanFailed(apiMessage(err)), "", nil)
			} else {
				widgets.ShowSnack(l10n.StorageFreed(core.FmtBytes(core.Obj(v).F("freed"))), "", nil)
			}
			d.load()
		})
	}
	if what == "sync" {
		m3.Confirm(l10n.StorageCleanSyncTitle, l10n.StorageCleanSyncBody, l10n.CommonCancel, l10n.CommonEmpty, false, run)
		return
	}
	run()
}

// Show builds the pane.
func (d *DataSettings) Show(c *ui.Context) {
	cs := design.S()
	tt := design.TT()
	syncOn := d.sync.IsTrue("configured")
	syncErr := d.sync.O("last").S("error")
	e := d.estimate
	busy := d.task != ""
	ui.Column(c).FillWidth().Children(func() {
		settingsGroupPad(c, l10n.DataSyncTitle, icons.DevicesRounded, nil, [4]float32{design.SpaceSM, 0, design.SpaceSM, 0}, func() {
			sub := webdavSummary(d.sync)
			t := m3.Tile{Title: l10n.WebdavTitle, Subtitle: sub, SubtitleLines: 1, TrailingIcon: icons.ChevronRightRounded, Tap: true, NoRadius: true,
				Pad: true, PadX: design.SpaceXL, PadY: design.SpaceXS, Leading: func() {
					ui.Box(c).Size(40, 40).Center().Radius(12).Background(cs.PrimaryContainer).Children(func() {
						if d.sync.IsTrue("running") {
							m3.Spinner(c, 20, 2.5, cs.OnPrimaryContainer)
						} else {
							m3.Icon(c, icons.CloudSyncRounded, 24, cs.OnPrimaryContainer)
						}
					})
				}}
			if syncOn && syncErr != "" {
				t.SubtitleFn = func() { m3.Clamp(m3.Text(c, sub, tt.BodyMedium.C(cs.Error)), 1) }
			}
			if t.Show(c.Key("webdav")).Clicked() {
				m3.Nav().Push(NewWebDavPage(d.load))
			}
		})
		SettingsGroup(c, l10n.FolderSyncSettingsGroup, icons.FolderCopyOutlined, nil, func() {
			sub := l10n.FolderSyncSubtitle
			if d.sync.S("kind") == "folder" {
				sub = d.sync.S("url")
			}
			if (m3.Tile{Title: l10n.FolderSyncTitle, Subtitle: sub, TrailingIcon: icons.ChevronRightRounded, Tap: true, Pad: true}).Show(c.Key("folder")).Clicked() {
				m3.Nav().Push(NewFolderSyncPage(d.load))
			}
		})
		SettingsGroup(c, l10n.BackupTitle, icons.Inventory2Outlined, nil, func() {
			m3.Text(c, l10n.BackupIntro, tt.BodySmall.C(cs.OnSurfaceVariant))
			m3.VGap(c, design.SpaceSM)
			sub := "…"
			if e != nil {
				sub = l10n.BackupNoMedia
				if e.I("media_files") != 0 {
					sub = l10n.BackupMediaSize(e["media_files"], core.FmtBytes(e.F("media_bytes")))
				}
			}
			if ch, v := switchTile(c, "media", m3.Tile{Title: l10n.BackupIncludeMedia, Subtitle: sub, Disabled: busy || (e != nil && e.I("media_files") == 0)}, d.media); ch {
				d.media = v
			}
			if busy {
				ui.Column(c.Key("task")).FillWidth().Padding(design.SpaceMD, 0).
					Transition(ui.ElementTransition{Duration: design.Medium2, Ease: design.Emphasized, Enter: &ui.Motion{Opacity: 0, Y: -6}}).Children(func() {
					p := m3.Progress{}
					if d.total > 0 {
						p.Value = m3.Of(min(max(float64(d.done)/float64(d.total), 0), 1))
					}
					m3.Linear(c, p)
					m3.VGap(c, design.SpaceSM)
					text := d.task
					if d.done > 0 {
						text += " · " + core.FmtBytes(float64(d.done))
					}
					m3.Text(c, text, tt.BodySmall.C(cs.OnSurfaceVariant))
				})
			}
			m3.VGap(c, design.SpaceSM)
			ui.Row(c).FillWidth().Wrap().Gap(design.SpaceSM).AlignItems(ui.Center).Children(func() {
				label := l10n.BackupExport
				if e != nil {
					n := e.F("bytes")
					if d.media {
						n += e.F("media_bytes")
					}
					label = l10n.BackupExportSize(core.FmtBytes(n))
				}
				if (m3.Button{Kind: m3.Tonal, Icon: icons.ArchiveOutlined, Label: label, Disabled: busy}).Show(c).Clicked() {
					d.export()
				}
				if (m3.Button{Kind: m3.Outlined, Icon: icons.UnarchiveOutlined, Label: l10n.BackupRestoreFrom, Disabled: busy}).Show(c).Clicked() {
					d.restore()
				}
			})
		})
		d.storageGroup(c, busy)
	})
}

func showRestoreDialog(p core.J, done func(string)) {
	mode := "merge"
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		cs := design.S()
		tt := design.TT()
		created := time.UnixMilli(int64(p.F("created")))
		device := p.S("device")
		row := func(ic icons.Icon, label string, count int, col ui.Color) {
			if col.A == 0 {
				col = cs.OnSurfaceVariant
			}
			ui.Row(c.Key(label)).FillWidth().AlignItems(ui.Center).Padding(design.SpaceXS, 0).Children(func() {
				m3.Icon(c, ic, 18, col)
				m3.HGap(c, design.SpaceMD)
				m3.Text(c, label, tt.BodyMedium).Grow(1).Basis(0)
				m3.Text(c, core.Str(count), tt.TitleSmall)
			})
		}
		m3.AlertDialog(c, m3.Alert{Icon: icons.UnarchiveOutlined, Title: l10n.BackupRestore, Width: 440, Scrollable: true, Content: func() {
			from := ""
			if device != "" {
				from = l10n.BackupFromDevice(device)
			}
			m3.Text(c, l10n.BackupPreviewMeta(core.FmtLectureDate(created), fmt.Sprintf("%02d", created.Hour()), fmt.Sprintf("%02d", created.Minute()),
				from, p.I("files"), core.FmtBytes(p.F("bytes"))), tt.BodyMedium.C(cs.OnSurfaceVariant))
			m3.VGap(c, design.SpaceMD)
			row(icons.AddCircleOutlineRounded, l10n.RestoreNew, p.I("new"), cs.Primary)
			row(icons.UpdateRounded, l10n.RestoreNewer, p.I("newer"), ui.Color{})
			older := ui.Color{}
			if p.I("older") > 0 {
				older = cs.Tertiary
			}
			row(icons.HistoryRounded, l10n.RestoreOlder, p.I("older"), older)
			row(icons.CheckRounded, l10n.RestoreSame, p.I("same"), ui.Color{})
			if p.I("skipped") > 0 {
				row(icons.BlockRounded, l10n.RestoreSkipped, p.I("skipped"), ui.Color{})
			}
			m3.VGap(c, design.SpaceLG)
			m3.Segmented(c, []m3.Segment[string]{
				{Value: "merge", Label: l10n.RestoreMerge, Icon: icons.MergeRounded},
				{Value: "overwrite", Label: l10n.RestoreOverwrite, Icon: icons.RestoreRounded},
			}, &mode, m3.SegOpts{})
			m3.VGap(c, design.SpaceSM)
			hint := l10n.RestoreMergeHint
			if mode != "merge" {
				hint = l10n.RestoreOverwriteHint
			}
			m3.Text(c, hint, tt.BodySmall.C(cs.OnSurfaceVariant))
		}, Actions: func() {
			if m3.TextButton(c, l10n.CommonCancel).Clicked() {
				r.Close()
			}
			if (m3.Button{Kind: m3.Filled, Label: l10n.CommonRestore, Disabled: p.I("files") == 0}).Show(c).Clicked() {
				r.Pop(mode)
			}
		}})
	}, func(v any) {
		m, _ := v.(string)
		done(m)
	})
}

func storageColor(key string) ui.Color {
	cs := design.S()
	switch key {
	case "video":
		return cs.Primary
	case "slides":
		return cs.Tertiary
	case "courseware":
		return cs.Secondary
	case "text":
		return cs.PrimaryFixedDim
	case "index":
		return cs.Outline
	case "cache":
		return cs.Error
	case "sync":
		return cs.TertiaryFixedDim
	}
	return cs.OutlineVariant
}

func (d *DataSettings) storageGroup(c *ui.Context, busy bool) {
	cs := design.S()
	tt := design.TT()
	r := d.storage
	SettingsGroup(c, l10n.StorageTitle, icons.PieChartOutlineRounded, nil, func() {
		if len(r) == 0 {
			ui.Box(c).FillWidth().Padding(design.SpaceLG).Center().Children(func() { m3.Circular(c, m3.Progress{}) })
			return
		}
		total := r.F("total")
		var cats []core.J
		for _, x := range r.Os("categories") {
			if x.F("bytes") > 0 {
				cats = append(cats, x)
			}
		}
		clean := r.O("cleanable")
		root := r.S("library")
		ui.Row(c).FillWidth().AlignItems(ui.End).Children(func() {
			m3.Text(c, core.FmtBytes(total), tt.HeadlineSmall)
			m3.HGap(c, design.SpaceSM)
			m3.Text(c, l10n.CommonLibrary, tt.BodyMedium.C(cs.OnSurfaceVariant)).Margin(0, 0, 3, 0).Grow(1).Basis(0)
			if root != "" && core.CanOpenFolders() {
				if m3.IconBtn(c, icons.FolderOpenRounded, l10n.CommonOpenInFileManager).Clicked() {
					go core.OpenPath(root)
				}
			}
		})
		m3.VGap(c, design.SpaceMD)
		if total > 0 {
			// A rounded bar split into proportional parts with small gaps;
			// tiny parts stay visible.
			ui.Row(c).FillWidth().Height(12).Radius(design.CornerFull).Clip().Gap(3).Children(func() {
				for i, x := range cats {
					flex := min(max(int(x.F("bytes")/total*1000+.5), 8), 1000)
					ui.Box(c.Key(i)).Grow(float32(flex)).Basis(0).FillHeight().Background(storageColor(x.S("key")))
				}
			})
		}
		m3.VGap(c, design.SpaceMD)
		ui.Row(c).FillWidth().Wrap().GapX(design.SpaceLG).GapY(design.SpaceSM).Children(func() {
			for i, x := range cats {
				ui.Row(c.Key(i)).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(10, 10).Radius(5).Background(storageColor(x.S("key")))
					m3.HGap(c, design.SpaceSM)
					m3.Text(c, x.S("label"), tt.BodySmall)
					m3.HGap(c, design.SpaceXS)
					m3.Text(c, core.FmtBytes(x.F("bytes")), tt.BodySmall.C(cs.OnSurfaceVariant))
				})
			}
		})
		m3.VGap(c, design.SpaceMD)
		m3.Divider(c)
		cleanTile := func(key string, ic icons.Icon, title, sub, action string, disabled bool, run func()) {
			m3.Tile{LeadingIcon: ic, Title: title, Subtitle: sub, Pad: true, PadX: 0, Trailing: func() {
				if (m3.Button{Kind: m3.TextKind, Label: action, Disabled: disabled}).Show(c).Clicked() {
					run()
				}
			}}.Show(c.Key(key))
		}
		cache, sync := clean.F("cache"), clean.F("sync")
		sub := l10n.StorageCacheEmpty
		if cache > 0 {
			sub = l10n.StorageCacheCleanable(core.FmtBytes(cache))
		}
		cleanTile("cache", icons.CleaningServicesOutlined, l10n.StorageCache, sub, l10n.CommonClean, busy || cache == 0,
			func() { d.clean("cache", l10n.StorageCleaningCache) })
		sub = l10n.StorageNoFiles
		if sync > 0 {
			sub = l10n.StorageSyncSize(core.FmtBytes(sync))
		}
		cleanTile("sync", icons.RestoreFromTrashOutlined, l10n.StorageSync, sub, l10n.CommonEmpty, busy || sync == 0,
			func() { d.clean("sync", l10n.StorageEmptying) })
		cleanTile("compact", icons.CompressRounded, l10n.StorageCompact, l10n.StorageCompactHint, l10n.StorageCompactAction, busy,
			func() { d.clean("compact", l10n.StorageCompacting) })
		m3.Divider(c)
		for _, x := range []struct {
			label, key string
			icon       icons.Icon
		}{
			{l10n.StorageModels, "models", icons.GraphicEqRounded},
			{l10n.StorageAssistant, "assistant", icons.ForumOutlined},
			{l10n.StorageRuntime, "runtime", icons.TerminalRounded},
		} {
			if n := r.F(x.key); n > 0 {
				m3.Tile{Leading: func() { m3.Icon(c, x.icon, 20, cs.OnSurfaceVariant) }, Title: x.label, Dense: true, Pad: true, PadX: 0, Trailing: func() {
					m3.Text(c, core.FmtBytes(n), tt.BodyMedium.C(cs.OnSurfaceVariant))
				}}.Show(c.Key(x.key))
			}
		}
	})
}
