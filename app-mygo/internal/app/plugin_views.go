package app

import (
	"encoding/json"
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

// Declarative views: plugins send data, never UI code; the app owns every
// renderer (docs/specs/0004-plugin-platform.md, 视图目录).

// pluginViewBuilders draws the view kinds this app knows.
var pluginViewBuilders = map[string]func(c *ui.Context, data core.J){
	"checklist": ChecklistView,
}

// PluginView is a view from a tool result, an action or a coursive-view
// block.
func PluginView(c *ui.Context, view core.J) {
	kind := core.Str(view["kind"])
	data := objOrEmpty(view["data"])
	if b, ok := pluginViewBuilders[kind]; ok {
		b(c, data)
		return
	}
	unknownPluginView(c, kind, data)
}

// showPluginViewSheet shows a view in a sheet over the current screen.
func showPluginViewSheet(view core.J) {
	m3.ShowModalBottomSheet(m3.SheetOptions{ScrollControlled: true, MaxWidth: 760, HeightFactor: .7}, func(c *ui.Context, r *m3.Route) {
		ui.Scroll(c).FillWidth().Grow(1).Basis(0).Children(func() {
			ui.Column(c).FillWidth().Padding(0, design.SpaceXL, design.SpaceXL, design.SpaceXL).Children(func() {
				if t, ok := view["title"]; ok && t != nil {
					m3.Text(c, core.Str(t), design.TT().TitleLarge).Margin(0, 0, design.SpaceLG, 0)
				}
				PluginView(c, view)
			})
		})
	})
}

// runPluginEntry runs a contributed entry and presents its result.
// onPrompt replaces the default of asking the assistant in a sheet (the
// panel itself sends).
func runPluginEntry(e core.PluginEntry, ctx core.J, onPrompt func(string)) {
	if ctx == nil {
		ctx = core.J{}
	}
	core.Async(func() (core.J, error) { return Plugins.Run(e, ctx) }, func(r core.J, err error) {
		if err != nil {
			widgets.ShowSnack(l10n.PluginError(e.PluginTitle(), apiMessage(err)), "", nil)
			return
		}
		if t, ok := r["toast"].(string); ok {
			widgets.ShowSnack(t, "", nil)
		}
		if v := r.O("view"); v != nil {
			showPluginViewSheet(v)
		} else if p, ok := r["prompt"].(string); ok {
			if onPrompt != nil {
				onPrompt(p)
			} else {
				AskInSheet(p)
			}
		} else if u, ok := r["open"].(string); ok {
			if ref, ok := ParseRef(u); ok {
				OpenRef(ref, "", nil)
			} else if strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "http://") {
				OpenLink(u)
			}
		}
	})
}

// ChecklistView is a list of items with done states and a progress
// header, as reported by the plugin (read-only: a snapshot of a tool
// result). Data: {title?, items: [{text, done?, note?}]}; agent-operator
// items carry state (todo | done | failed) and evidence instead.
func ChecklistView(c *ui.Context, data core.J) {
	cs := design.S()
	tt := design.TT()
	items := core.Objs(data["items"])
	state := func(x core.J) string {
		if x["done"] == true {
			return "done"
		}
		return x.Or("state", "todo")
	}
	done := 0
	for _, x := range items {
		if state(x) == "done" {
			done++
		}
	}
	ui.Column(c).FillWidth().Padding(design.SpaceLG).Background(cs.SurfaceContainerLow).Radius(design.CornerLG).Border(1, cs.OutlineVariant).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			m3.Icon(c, icons.ChecklistRounded, 18, cs.Primary)
			m3.HGap(c, design.SpaceSM)
			title := l10n.PluginViewList
			if t, ok := data["title"]; ok && t != nil {
				title = core.Str(t)
			}
			m3.Text(c, title, tt.TitleSmall).Grow(1).Basis(0)
			m3.Text(c, strconv.Itoa(done)+" / "+strconv.Itoa(len(items)), tt.LabelMedium.C(cs.OnSurfaceVariant))
		})
		m3.VGap(c, design.SpaceSM)
		v := 0.0
		if len(items) > 0 {
			v = float64(done) / float64(len(items))
		}
		m3.Linear(c, m3.Progress{Value: m3.Of(v), Height: 6, Track: cs.SurfaceContainerHighest})
		m3.VGap(c, design.SpaceSM)
		for i, x := range items {
			st := state(x)
			ui.Row(c.Key(i)).FillWidth().AlignItems(ui.Start).Padding(design.SpaceXS, 0).Children(func() {
				// Status marks, not checkboxes: a pending item is a dot.
				ui.Box(c).Size(20, 20).Shrink(0).Center().Children(func() {
					switch st {
					case "done":
						m3.Icon(c, icons.CheckCircleRounded, 20, cs.Primary)
					case "failed":
						m3.Icon(c, icons.ErrorRounded, 20, cs.Error)
					default:
						ui.Box(c).Size(8, 8).Radius(4).Background(cs.Outline)
					}
				})
				m3.HGap(c, design.SpaceMD)
				ui.Column(c).Grow(1).Basis(0).Children(func() {
					if st == "done" {
						m3.Text(c, x.S("text"), tt.BodyMedium.C(cs.OnSurfaceVariant)).Strikethrough().DecorationColor(cs.OnSurfaceVariant)
					} else {
						m3.Text(c, x.S("text"), tt.BodyMedium.C(cs.OnSurface))
					}
					for _, k := range []string{"note", "evidence"} {
						if s := x.S(k); s != "" {
							m3.Text(c, s, tt.BodySmall.C(cs.OnSurfaceVariant))
						}
					}
				})
			})
		}
	})
}

// unknownPluginView is a view this app version cannot draw: it says so,
// and keeps the data at hand.
func unknownPluginView(c *ui.Context, kind string, data core.J) {
	cs := design.S()
	tt := design.TT()
	ui.Column(c).FillWidth().Background(cs.SurfaceContainerLow).Radius(design.CornerLG).Border(1, cs.OutlineVariant).Clip().Children(func() {
		m3.Expansion{
			Tile: m3.Tile{LeadingIcon: icons.WidgetsOutlined, IconColor: cs.OnSurfaceVariant, Title: l10n.PluginViewUnknown(kind),
				TitleStyle: &tt.TitleSmall, Subtitle: l10n.PluginViewUnsupported, NoRadius: true},
			Children: func() {
				b, _ := json.MarshalIndent(data, "", "  ")
				ui.Column(c).FillWidth().Padding(0, design.SpaceLG, design.SpaceLG, design.SpaceLG).Children(func() {
					m3.Clamp(m3.Text(c, string(b), tt.BodySmall.F("monospace")).Selectable(), 30)
				})
			},
		}.Show(c, "unknown")
	})
}

// coursiveViewBlock is a fenced ```coursive-view block ({"kind": ...,
// "data": ...}) in Markdown, drawn when an active plugin declares the kind
// in contributes.blocks; false leaves the block to the code renderer.
func coursiveViewBlock(c *ui.Context, lang, code string) bool {
	if lang != "coursive-view" {
		return false
	}
	v, err := core.Decode([]byte(code))
	m := core.Obj(v)
	if err != nil || m == nil {
		return false
	}
	kind, ok := m["kind"].(string)
	if !ok || !Plugins.RendersBlock(kind) {
		return false
	}
	PluginView(c, m)
	return true
}

// PluginMenuButton is a menu of the plugin entries for a surface, e.g. a
// note's toolbar; nothing when no active plugin contributes to it.
// noteViews and exportScope add those contributions under their own
// headings.
func PluginMenuButton(c *ui.Context, surface string, ctx core.J, noteViews bool, exportScope string) {
	type group struct {
		label   string
		entries []core.PluginEntry
	}
	var groups []group
	add := func(label string, entries []core.PluginEntry) {
		if len(entries) > 0 {
			groups = append(groups, group{label, entries})
		}
	}
	add("", Plugins.Entries("actions", surface, ""))
	if noteViews {
		add(l10n.PluginViewAlternative, Plugins.Entries("noteViews", "", ""))
	}
	if exportScope != "" {
		add(l10n.CommonExport, Plugins.Entries("exporters", "", exportScope))
	}
	if len(groups) == 0 {
		return
	}
	cs := design.S()
	m3.MenuButton(c, "plugins", icons.ExtensionOutlined, l10n.CommonPlugins, m3.Below, func() []m3.MenuItem {
		var items []m3.MenuItem
		for i, g := range groups {
			if i > 0 {
				items = append(items, m3.MenuItem{Divider: true})
			}
			if g.label != "" {
				items = append(items, m3.MenuItem{Header: true, Label: g.label, Color: cs.Primary})
			}
			for _, e := range g.entries {
				e := e
				items = append(items, m3.MenuItem{Label: e.Title, IconSize: 20, Trailing: e.PluginTitle(),
					LeadingFn: func(c *ui.Context) { m3.Icon(c, widgets.PluginIcon(e.Icon), 20, cs.OnSurfaceVariant) },
					OnTap:     func() { runPluginEntry(e, ctx, nil) }})
			}
		}
		return items
	})
}
