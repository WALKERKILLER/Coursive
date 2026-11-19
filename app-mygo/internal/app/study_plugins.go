package app

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/study"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// study_plugin_ui.dart: plugin objects on the study canvas
// (docs/specs/0032-ui-extension-protocol.md, Canvas plugin objects). They
// are drawn between the cards and the ink and never hit-tested, so a plugin
// cannot take a stroke or a selection; a tool changes them only through a
// checked batch that is one undo entry.

// canvasTools is the slot target plugin canvas tools render in.
const canvasTools = "canvas.toolbar.tools"

// activePlugins are the enabled plugin ids.
func (s *StudyCanvas) activePlugins() map[string]bool {
	out := map[string]bool{}
	for _, p := range Plugins.Active() {
		out[p.S("id")] = true
	}
	return out
}

// canvasToolContributions are the canvas tools of enabled plugins.
func (s *StudyCanvas) canvasToolContributions() []slotContribution {
	return slotContributions(canvasTools)
}

// pluginsOffered offers the plugin sheet when a plugin contributes a canvas
// tool or the canvas holds plugin objects.
func (s *StudyCanvas) pluginsOffered() bool {
	return len(s.doc.PluginObjects) > 0 || s.hasCanvasTools
}

func pluginTitle(id string) string {
	if p := Plugins.ByID(id); p != nil && p.S("title") != "" {
		return p.S("title")
	}
	return id
}

func (s *StudyCanvas) pluginObject(c *ui.Context, o *study.PluginObject, stage study.Rect, active bool) {
	r := o.Rect()
	w, h := r.W(), r.H()
	turns := o.QuarterTurns()
	box := ui.Box(c.Key("study-plugin/"+o.ID())).Absolute().Left(f32(r.L-stage.L)).Top(f32(r.T-stage.T)).Size(f32(w), f32(h)).
		IgnoreInput(true).Clip()
	box.Children(func() {
		// The content is laid out unrotated, then turned into the box.
		iw, ih := w, h
		if turns%2 == 1 {
			iw, ih = h, w
		}
		inner := ui.Box(c).Absolute().Left(f32((w-iw)/2)).Top(f32((h-ih)/2)).Size(f32(iw), f32(ih))
		if turns != 0 {
			inner = inner.Rotate(float32(90 * turns))
		}
		inner.Children(func() {
			if active {
				pluginCard(c, o, pluginTitle(o.PluginID()))
			} else {
				canvasFallback(c, o.Fallback(), nil)
			}
		})
	})
}

// pluginCard is _StudyPluginCard: an object whose plugin is enabled, drawn
// by the host from the stored content.
func pluginCard(c *ui.Context, o *study.PluginObject, plugin string) {
	cs, tt := design.S(), design.TT()
	muted := tt.LabelSmall.C(cs.OnSurfaceVariant)
	ui.Column(c).Fill().Padding(design.SpaceMD).Radius(design.CornerLG).Background(cs.SurfaceContainerLow).
		Border(1, cs.OutlineVariant).AlignItems(ui.Stretch).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			m3.Icon(c, icons.ExtensionOutlined, 16, cs.Primary)
			ui.Box(c).Width(design.SpaceXS)
			m3.Text(c, plugin, muted).MaxLines(1).Grow(1).Basis(0)
		})
		ui.Box(c).Height(design.SpaceXS)
		m3.Text(c, o.Title(), tt.TitleSmall.C(cs.OnSurface)).MaxLines(2)
		if text := o.Text(); text != "" {
			ui.Box(c).Height(design.SpaceXS)
			m3.Text(c, text, tt.BodySmall.C(cs.OnSurface)).Grow(1).Basis(0).Clip()
		} else {
			ui.Spacer(c)
		}
		if src := o.Source(); src != "" {
			m3.Text(c, src, muted).MaxLines(1)
		}
	})
}

// canvasFallback is CanvasFallback: an object whose plugin is missing, its
// title, source, summary and, with onExport, an export of its data.
func canvasFallback(c *ui.Context, m study.Fallback, onExport func(map[string]any)) {
	cs, tt := design.S(), design.TT()
	muted := tt.LabelSmall.C(cs.OnSurfaceVariant)
	ui.Column(c).FillWidth().Padding(design.SpaceLG).Radius(design.CornerLG).Background(cs.SurfaceContainerHigh).
		Border(1, cs.OutlineVariant).AlignItems(ui.Stretch).Children(func() {
		if m.ID == "" && m.Title == "" && m.Source == "" && m.Summary == "" && len(m.Data) == 0 {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				m3.Icon(c, icons.HelpOutlineRounded, 18, cs.OnSurfaceVariant)
				ui.Box(c).Width(design.SpaceSM)
				m3.Text(c, l10n.PluginCanvasFallbackUnreadable, muted).Grow(1).Basis(0)
			})
			return
		}
		title := m.Title
		if title == "" {
			title = l10n.PluginCanvasFallbackUntitled
		}
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			m3.Icon(c, icons.ExtensionOffRounded, 18, cs.OnSurfaceVariant)
			ui.Box(c).Width(design.SpaceSM)
			m3.Text(c, l10n.PluginCanvasFallbackMissing, muted).Grow(1).Basis(0)
		})
		ui.Box(c).Height(design.SpaceSM)
		m3.Text(c, title, tt.TitleSmall.C(cs.OnSurface))
		if m.Source != "" {
			ui.Box(c).Height(design.SpaceXS)
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				m3.Icon(c, icons.LinkRounded, 16, cs.OnSurfaceVariant)
				ui.Box(c).Width(design.SpaceXS)
				m3.Text(c, m.Source, muted).Grow(1).Basis(0)
			})
		}
		ui.Box(c).Height(design.SpaceSM)
		summary, col := m.Summary, cs.OnSurface
		if summary == "" {
			summary, col = l10n.PluginCanvasFallbackEmpty, cs.OnSurfaceVariant
		}
		m3.Text(c, summary, tt.BodySmall.C(col)).MaxLines(4)
		if onExport != nil && len(m.Data) > 0 {
			ui.Row(c).Justify(ui.End).Children(func() {
				if (m3.Button{Kind: m3.TextKind, Icon: icons.DownloadRounded, Label: l10n.PluginCanvasFallbackExport}).Show(c).Clicked() {
					onExport(map[string]any{"id": m.ID, "type": m.Type, "pluginId": m.PluginID, "schemaVersion": m.SchemaVersion,
						"revision": m.Revision, "title": m.Title, "source": m.Source, "summary": m.Summary, "data": m.Data})
				}
			})
		}
	})
}

// pluginContext is what a canvas tool may read as context.*: this lecture,
// the revision a batch is checked against, the centre of the view in canvas
// units and the plugin's own objects. Nothing of another plugin is passed.
func (s *StudyCanvas) pluginContext(pluginID string) core.J {
	d := s.doc
	centre := s.world(s.viewport.Mul(.5))
	objects := []any{}
	for _, o := range d.PluginObjects {
		if o.PluginID() == pluginID {
			objects = append(objects, map[string]any{"id": o.ID(), "revision": o.Revision()})
		}
	}
	return core.J{"lecture_id": s.lectureID(), "canvas_revision": d.PluginRevision, "canvas_x": centre.X, "canvas_y": centre.Y,
		"canvas_objects": objects}
}

func (s *StudyCanvas) plugins() {
	d := s.doc
	m3.ShowModalBottomSheet(m3.SheetOptions{ScrollControlled: true, MaxHeight: f32(s.viewport.Y * .75)}, func(c *ui.Context, r *m3.Route) {
		if s.doc != d {
			return
		}
		tools := s.canvasToolContributions()
		ui.Scroll(c).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Padding(0, 0, design.SpaceMD, 0).Children(func() {
				m3.Tile{Title: l10n.StudyPlugins}.Show(c)
				for _, t := range tools {
					ui.Box(c.Key("tool/"+t.Entry.PluginID()+"/"+t.Entry.ID)).FillWidth().Padding(design.SpaceXS, design.SpaceMD).Children(func() {
						PluginDeclarativeView(c, PluginDeclarative{PluginID: t.Entry.PluginID(), ViewID: t.Entry.ID, Document: t.Document,
							HostContext: s.pluginContext(t.Entry.PluginID()), OnViewPatch: s.pluginPatch})
					})
				}
				if len(tools) == 0 {
					m3.Tile{Title: l10n.StudyPluginsNoTools, Dense: true}.Show(c)
				}
				if len(d.PluginObjects) > 0 {
					m3.Tile{Title: l10n.StudyPluginObjects}.Show(c)
				}
				for _, o := range d.PluginObjects {
					o := o
					parts := []string{pluginTitle(o.PluginID())}
					if src := o.Source(); src != "" {
						parts = append(parts, src)
					}
					tile := m3.Tile{LeadingIcon: icons.ExtensionOutlined, Title: o.Title(), Subtitle: strings.Join(parts, " · "), Tap: true,
						Trailing: func() {
							ui.Row(c).Children(func() {
								if (m3.IconButton{Icon: icons.DownloadRounded, Tooltip: l10n.StudyPluginObjectExport}).Show(c).Clicked() {
									s.exportPluginObject(o)
								}
								if (m3.IconButton{Icon: icons.DeleteOutline, Tooltip: l10n.StudyPluginObjectRemove}).Show(c).Clicked() {
									d.RemovePluginObject(o.ID())
								}
							})
						}}.Show(c.Key("study-plugin-entry/" + o.ID()))
					if tile.Clicked() {
						r.Pop(nil)
						rect := o.Rect()
						s.flyTo(s.viewport.Mul(.5).Sub(rect.Center().Mul(d.Zoom)), d.Zoom)
					}
				}
			})
		})
	})
}

// pluginPatch commits a tool's canvas batch: checked whole against the
// current plugin objects, shown for approval when it overwrites or
// deletes, and refused whole when the canvas changed in between.
func (s *StudyCanvas) pluginPatch(pluginID string, patch PluginViewPatch) {
	d := s.doc
	raw, ok := patch.Patch["canvas"]
	if d == nil || !ok || raw == nil {
		return
	}
	batch, err := d.CheckPluginBatch(pluginID, plainJSON(raw))
	if err != nil {
		s.toast(l10n.StudyPluginBatchFailed(err.Error()))
		return
	}
	commit := func() {
		if err := d.CommitPluginBatch(batch); err != nil {
			s.toast(l10n.StudyPluginBatchFailed(err.Error()))
		}
	}
	if !batch.NeedsApproval() {
		commit()
		return
	}
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		cs, tt := design.S(), design.TT()
		m3.AlertDialog(c, m3.Alert{Title: l10n.StudyPluginBatchTitle(pluginTitle(batch.PluginID)), Content: func() {
			ui.Scroll(c).FillWidth().Children(func() {
				ui.Column(c).FillWidth().AlignItems(ui.Start).Children(func() {
					st := tt.BodyMedium.C(cs.OnSurfaceVariant)
					if batch.Label != "" {
						m3.Text(c, batch.Label, st)
					}
					for _, o := range batch.Created {
						m3.Text(c, l10n.StudyPluginBatchCreate(o.Title()), st)
					}
					for _, o := range batch.Updated {
						m3.Text(c, l10n.StudyPluginBatchUpdate(o.Title()), st)
					}
					for _, o := range batch.Deleted {
						m3.Text(c, l10n.StudyPluginBatchDelete(o.Title()), st)
					}
					ui.Box(c).Height(design.SpaceSM)
					m3.Text(c, l10n.StudyPluginBatchUndo, st)
				})
			})
		}, Actions: func() {
			if m3.TextButton(c, l10n.PluginProposalReject).Clicked() {
				r.Pop(false)
			}
			if m3.FilledButton(c, l10n.StudyPluginBatchApply).Clicked() {
				r.Pop(true)
			}
		}})
	}, func(v any) {
		if v == true && s.doc == d {
			commit()
		}
	})
}

func (s *StudyCanvas) exportPluginObject(o *study.PluginObject) {
	data, err := json.MarshalIndent(o.JSON(), "", "  ")
	if err != nil {
		s.toast(l10n.StudyExportError(err.Error()))
		return
	}
	widgets.PickSavePath(o.Title()+".json", core.FileFilter{Name: "JSON", Extensions: []string{"json"}}, "", func(path string) {
		if path == "" {
			return
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			s.toast(l10n.StudyExportError(err.Error()))
			return
		}
		s.toast(l10n.StudyPluginObjectExported(o.Title()))
	})
}
