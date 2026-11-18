package app

import (
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// KnowledgeSettings is the 知识库 pane: where the library lives, and where
// it goes.
type KnowledgeSettings struct {
	settings              *ServerSettings
	courseTpl, lectureTpl string
	notion                core.J
	cancel                func()
}

// NewKnowledgeSettings loads the Notion status and follows its events.
func NewKnowledgeSettings(s *ServerSettings) *KnowledgeSettings {
	k := &KnowledgeSettings{settings: s}
	k.loadNotion()
	k.cancel = State.Events.Listen(func(e core.J) {
		if t := e.S("type"); strings.HasPrefix(t, "notion_") && t != "notion_progress" {
			k.loadNotion()
		}
	})
	return k
}

// Dispose stops following the events.
func (k *KnowledgeSettings) Dispose() { k.cancel() }

// fill takes the saved templates once they arrive, without clobbering
// typing.
func (k *KnowledgeSettings) fill() {
	if k.settings.Values == nil {
		return
	}
	tpl := k.settings.Values.O("library.templates")
	if k.courseTpl == "" {
		k.courseTpl = tpl.Or("course", "{title}")
	}
	if k.lectureTpl == "" {
		k.lectureTpl = tpl.Or("lecture", "{date} {title}")
	}
}

func (k *KnowledgeSettings) loadNotion() {
	core.Async(func() (any, error) { return api().Get("/api/notion") }, func(v any, err error) {
		if err == nil {
			k.notion = core.Obj(v)
		}
	})
}

// Show builds the pane.
func (k *KnowledgeSettings) Show(c *ui.Context) {
	k.fill()
	cs := design.S()
	tt := design.TT()
	v := k.settings.Values
	vault := v.S("obsidian.vault")
	notionOn := k.notion.IsTrue("connected")
	ui.Column(c).FillWidth().Children(func() {
		settingsGroupPad(c, l10n.KnowledgeConnectApps, 0, nil, [4]float32{design.SpaceSM, 0, design.SpaceSM, 0}, func() {
			sub := l10n.KnowledgeObsidianSubtitle
			if vault != "" {
				sub = filepath.Base(vault) + " / " + v.S("obsidian.folder")
				if v.IsTrue("obsidian.auto") {
					sub += l10n.KnowledgeAutoExportSuffix
				}
			}
			if integrationTile(c, "obsidian", func() { BrandMark(c, "obsidian.svg", 0, false) }, "Obsidian", sub, vault != "", false) {
				m3.Nav().Push(NewObsidianPage(k.settings))
			}
			notionErr := notionOn && k.notion.O("last").S("error") != ""
			if integrationTile(c, "notion", func() { BrandMark(c, notionLogo, 0, true) }, "Notion", notionSummary(k.notion), notionOn, notionErr) {
				m3.Nav().Push(NewNotionPage(k.loadNotion))
			}
		})
		SettingsGroup(c, l10n.CommonLibrary, 0, nil, func() {
			root := State.Daemon.LibraryRoot()
			t := m3.Tile{Title: l10n.CommonLocation, Subtitle: root, Pad: true, PadX: 0}
			if core.CanOpenFolders() {
				t.Trailing = func() {
					if m3.IconBtn(c, icons.OpenInNewRounded, l10n.CommonOpen).Clicked() {
						go core.OpenPath(root)
					}
				}
			}
			t.Show(c.Key("root"))
			m3.VGap(c, design.SpaceSM)
			m3.Text(c, l10n.KnowledgeFolderTemplates, tt.TitleSmall)
			m3.VGap(c, design.SpaceXS)
			m3.Text(c, l10n.KnowledgeFolderTemplatesHint, tt.BodySmall.C(cs.OnSurfaceVariant))
			m3.VGap(c, design.SpaceMD)
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				ui.Box(c).Grow(1).Basis(0).Children(func() {
					m3.Field{Label: l10n.KnowledgeCourseFolder, Dense: true}.Show(c.Key("course"), &k.courseTpl)
				})
				m3.HGap(c, design.SpaceMD)
				ui.Box(c).Grow(1).Basis(0).Children(func() {
					m3.Field{Label: l10n.KnowledgeLectureFolder, Dense: true}.Show(c.Key("lecture"), &k.lectureTpl)
				})
				m3.HGap(c, design.SpaceSM)
				if (m3.Button{Kind: m3.Tonal, Label: l10n.CommonSave}).Show(c).Clicked() {
					body := core.J{"course": strings.TrimSpace(k.courseTpl), "lecture": strings.TrimSpace(k.lectureTpl)}
					k.settings.Set("library.templates", body)
					core.Async(func() (any, error) { return api().Put("/api/settings", core.J{"library.templates": body}) }, func(r any, err error) {
						if err != nil {
							widgets.ShowSnack(l10n.CommonSaveFailed(apiMessage(err)), "", nil)
							return
						}
						if m := core.Obj(r); m != nil {
							k.settings.Values = m
						}
						widgets.ShowSnack(l10n.KnowledgeTemplatesSaved, "", nil)
					})
				}
			})
			m3.VGap(c, design.SpaceMD)
			if settingsTile(c, "export", m3.Tile{LeadingIcon: icons.IosShareRounded, Title: l10n.KnowledgeSkillExport, Subtitle: l10n.KnowledgeSkillExportHint}) {
				exportSkill()
			}
		})
	})
}

// exportSkill writes the library as an agent skill into a folder.
func exportSkill() {
	root := ""
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		m3.AlertDialog(c, m3.Alert{Title: l10n.KnowledgeSkillExportAction, Width: 440, Content: func() {
			if (widgets.FolderField{Label: l10n.KnowledgeTargetFolder, Helper: l10n.KnowledgeTargetFolderHelper, AutoFocus: true}).Show(c, &root) {
				r.Pop(strings.TrimSpace(root))
			}
		}, Actions: func() {
			if m3.TextButton(c, l10n.CommonCancel).Clicked() {
				r.Close()
			}
			if m3.FilledButton(c, l10n.CommonExport).Clicked() {
				r.Pop(strings.TrimSpace(root))
			}
		}})
	}, func(v any) {
		dir, _ := v.(string)
		if dir == "" {
			return
		}
		core.Async(func() (any, error) { return api().Post("/api/library/export", core.J{"root": dir}) }, func(r any, err error) {
			if err != nil {
				widgets.ShowSnack(l10n.CommonExportFailed(apiMessage(err)), "", nil)
				return
			}
			widgets.ShowSnack(l10n.KnowledgeExported(core.Obj(r)["exported"]), "", nil)
		})
	})
}

// integrationTile is a connected app's row: its mark, status and a
// 已连接 pill.
func integrationTile(c *ui.Context, key string, mark func(), title, subtitle string, on, failed bool) bool {
	cs := design.S()
	tt := design.TT()
	t := m3.Tile{Leading: mark, Title: title, Subtitle: subtitle, SubtitleLines: 1, Tap: true, Pad: true, PadX: design.SpaceXL, PadY: design.SpaceXS,
		NoRadius: true, Trailing: func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				if on {
					ui.Box(c).Padding(design.SpaceXXS, design.SpaceSM).Radius(design.CornerFull).Background(cs.SecondaryContainer).Children(func() {
						m3.Text(c, l10n.CommonConnected, tt.LabelSmall.C(cs.OnSecondaryContainer))
					})
				}
				m3.HGap(c, design.SpaceXS)
				m3.Icon(c, icons.ChevronRightRounded, 24, cs.OnSurfaceVariant)
			})
		}}
	if failed {
		t.SubtitleFn = func() { m3.Clamp(m3.Text(c, subtitle, tt.BodyMedium.C(cs.Error)), 1) }
	}
	return t.Show(c.Key(key)).Clicked()
}
