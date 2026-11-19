package app

import (
	"strconv"
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

// MirrorsPage is 插件 → 下载线路: the GitHub mirrors the plugin index and
// source plugins download through, the last speed test of each, and the
// choice between 自动 and one mirror. The daemon checks what every route
// brings, so a mirror can only make a download faster.
// Contract: docs/specs/0025-plugin-marketplace.md
type MirrorsPage struct {
	st       core.J
	err      error
	busy     bool
	testing  bool
	name     string
	template string
	scroll   ui.ScrollState
	// changed runs once the routes change, so the market reads the index
	// again.
	changed func()
}

// OpenMirrors opens 下载线路; changed runs after the choice or the speed
// test changes.
func OpenMirrors(changed func()) {
	p := &MirrorsPage{changed: changed}
	p.load()
	m3.Nav().Push(p)
}

func (p *MirrorsPage) load() {
	core.Async(func() (any, error) { return api().Get("/api/sources/market/mirrors") }, func(v any, err error) {
		if err != nil {
			p.err = err
			return
		}
		p.st, p.err = core.Obj(v), nil
	})
}

// apply sends a request whose reply is the new state.
func (p *MirrorsPage) apply(req func() (any, error), then func()) {
	p.busy = true
	core.Async(req, func(v any, err error) {
		p.busy, p.testing = false, false
		if err != nil {
			widgets.ShowSnack(apiMessage(err), "", nil)
			return
		}
		p.st = core.Obj(v)
		if then != nil {
			then()
		}
		if p.changed != nil {
			p.changed()
		}
	})
}

func (p *MirrorsPage) test() {
	p.testing = true
	p.apply(func() (any, error) { return api().Post("/api/sources/market/mirrors/test", nil) }, nil)
}

// put saves mode and the user's mirrors.
func (p *MirrorsPage) put(mode string, custom []core.J, then func()) {
	if custom == nil {
		custom = []core.J{}
	}
	p.apply(func() (any, error) {
		return api().Put("/api/sources/market/mirrors", core.J{"mode": mode, "custom": custom})
	}, then)
}

func (p *MirrorsPage) custom() []core.J {
	var out []core.J
	for _, m := range p.st.Os("mirrors") {
		if m.IsTrue("custom") {
			out = append(out, core.J{"id": m.S("id"), "name": m.S("name"), "template": m.S("template"), "kinds": m["kinds"]})
		}
	}
	return out
}

func (p *MirrorsPage) add() {
	name, tpl := strings.TrimSpace(p.name), strings.TrimSpace(p.template)
	ids := map[string]bool{}
	for _, m := range p.st.Os("mirrors") {
		ids[m.S("id")] = true
	}
	id := ""
	for n := 1; id == "" || ids[id]; n++ {
		id = "custom-" + strconv.Itoa(n)
	}
	// A template by owner and repo serves raw files only.
	kinds := []string{"raw", "release", "archive"}
	if strings.Contains(tpl, "{owner}") {
		kinds = []string{"raw"}
	}
	custom := append(p.custom(), core.J{"id": id, "name": name, "template": tpl, "kinds": kinds})
	p.put(p.st.S("mode"), custom, func() { p.name, p.template = "", "" })
}

func (p *MirrorsPage) remove(id string) {
	var custom []core.J
	for _, m := range p.custom() {
		if m.S("id") != id {
			custom = append(custom, m)
		}
	}
	mode := p.st.S("mode")
	if mode == id {
		mode = "auto"
	}
	p.put(mode, custom, nil)
}

// View builds the page.
func (p *MirrorsPage) View(c *ui.Context) {
	if p.st == nil {
		m3.Scaffold{AppBar: &m3.AppBar{Title: l10n.SourcesMirrorsTitle}, Body: func() {
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
	results := map[string]core.J{}
	test := p.st.O("test")
	for _, r := range test.Os("results") {
		results[r.S("id")] = r
	}
	using := p.st.S("using")
	names := map[string]string{}
	for _, m := range p.st.Os("mirrors") {
		names[m.S("id")] = m.S("name")
	}
	SettingsSubpage(c, l10n.SourcesMirrorsTitle, &p.scroll, func() {
		m3.Text(c, l10n.SourcesMirrorsBody, tt.BodyMedium.C(cs.OnSurfaceVariant).H(1.6)).Margin(0, design.SpaceXS, 0, design.SpaceXS)
		SettingsGroup(c, l10n.SourcesMirrorsChoice, icons.RouteOutlined, nil, func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(0, 0, design.SpaceSM, 0).Children(func() {
				at := l10n.SourcesMirrorsUntested
				if t, err := time.Parse(time.RFC3339Nano, test.S("at")); err == nil && !t.IsZero() {
					at = l10n.SourcesMirrorsTestedAt(core.FmtRelative(t))
				}
				m3.Text(c, at, tt.BodyMedium.C(cs.OnSurfaceVariant)).Grow(1).Basis(0)
				b := m3.Button{Kind: m3.Tonal, Icon: icons.SpeedRounded, Label: l10n.SourcesMirrorsTest, Disabled: p.busy}
				if p.testing {
					b.Icon, b.Label = 0, l10n.SourcesMirrorsTesting
				}
				if b.Show(c).Clicked() && !b.Disabled {
					p.test()
				}
			})
			mode := p.st.S("mode")
			pick := func(id string) {
				if id != p.st.S("mode") {
					p.put(id, p.custom(), nil)
				}
			}
			sub := l10n.SourcesMirrorsAutoBody
			if mode == "auto" && names[using] != "" {
				sub = l10n.SourcesMirrorsUsing(names[using])
			}
			if m3.RadioTile(c.Key("auto"), m3.Tile{Title: l10n.SourcesMirrorsAuto, Subtitle: sub, Pad: true, PadX: 0, Disabled: p.busy}, &mode, "auto") {
				pick(mode)
			}
			for _, m := range p.st.Os("mirrors") {
				id := m.S("id")
				r := results[id]
				status, color := l10n.SourcesMirrorsUntested, cs.OnSurfaceVariant
				switch {
				case r == nil:
				case r.IsTrue("ok"):
					status = l10n.SourcesMirrorsResult(int(r.F("latency_ms")), core.FmtSpeed(r.F("speed")))
				default:
					status, color = l10n.SourcesMirrorsFailed(r.S("error")), cs.Error
				}
				if m.IsTrue("custom") {
					status = l10n.SourcesMirrorsCustom + " · " + status
				}
				t := m3.Tile{Title: m.S("name"), TitleLines: 1, Pad: true, PadX: 0, Disabled: p.busy, SubtitleLines: 2,
					SubtitleFn: func() { m3.Text(c, status, tt.BodyMedium.C(color)) }}
				if m.IsTrue("custom") {
					t.Trailing = func() {
						if (m3.IconButton{Icon: icons.DeleteOutlineRounded, Tooltip: l10n.CommonRemove, Disabled: p.busy}).Show(c).Clicked() {
							p.remove(id)
						}
					}
				}
				if m3.RadioTile(c.Key(id), t, &mode, id) {
					pick(mode)
				}
			}
		})
		SettingsGroup(c, l10n.SourcesMirrorsAdd, icons.AddRounded, nil, func() {
			m3.Field{Label: l10n.SourcesMirrorsName}.Show(c.Key("name"), &p.name)
			m3.VGap(c, design.SpaceMD)
			m3.Field{Label: l10n.SourcesMirrorsTemplate, Hint: "https://example.com/{url}",
				Helper: l10n.SourcesMirrorsTemplateHelper("{url}")}.Show(c.Key("template"), &p.template)
			m3.VGap(c, design.SpaceMD)
			ui.Row(c).FillWidth().Children(func() {
				ui.Box(c).Grow(1)
				ready := strings.TrimSpace(p.name) != "" && strings.HasPrefix(strings.TrimSpace(p.template), "https://")
				b := m3.Button{Kind: m3.Filled, Icon: icons.AddRounded, Label: l10n.CommonAdd, Disabled: p.busy || !ready}
				if b.Show(c).Clicked() && !b.Disabled {
					p.add()
				}
			})
		})
	})
}
