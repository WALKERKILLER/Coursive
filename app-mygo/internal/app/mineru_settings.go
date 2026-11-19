package app

import (
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

const mineruTokenPage = "https://mineru.net/apiManage/token"

func mineruDate(t time.Time) string { return l10n.DateYearMonthDay(t.Year(), int(t.Month()), t.Day()) }

// MinerUSettings is MinerU courseware parsing: API token, options and
// today's page quota. The token stays sealed in the daemon; only its last
// four characters come back.
type MinerUSettings struct {
	st                     core.J
	token                  string
	editing, busy, obscure bool
	test                   string
	testOk                 bool
}

// NewMinerUSettings loads the state.
func NewMinerUSettings() *MinerUSettings {
	m := &MinerUSettings{obscure: true}
	core.Async(func() (any, error) { return api().Get("/api/mineru") }, func(v any, err error) {
		if err == nil {
			m.st = core.Obj(v)
		}
	})
	return m
}

func (m *MinerUSettings) put(body core.J, then func()) {
	core.Async(func() (any, error) { return api().Put("/api/mineru", body) }, func(v any, err error) {
		if err != nil {
			widgets.ShowSnack(l10n.CommonSaveFailed(apiMessage(err)), "", nil)
		} else {
			m.st = core.Obj(v)
		}
		if then != nil {
			then()
		}
	})
}

func (m *MinerUSettings) saveToken() {
	t := strings.TrimSpace(m.token)
	if t == "" {
		return
	}
	m.busy = true
	m.put(core.J{"token": t}, func() {
		m.token, m.busy, m.editing = "", false, false
		m.runTest()
	})
}

func (m *MinerUSettings) runTest() {
	m.busy, m.test = true, ""
	core.Async(func() (any, error) { return api().PostWait("/api/mineru/test", nil, 30*time.Second) }, func(v any, err error) {
		m.busy = false
		if err != nil {
			m.testOk, m.test = false, apiMessage(err)
			return
		}
		r := core.Obj(v)
		m.testOk = r.IsTrue("ok")
		m.test = l10n.CommonConnectionOk
		if !m.testOk {
			m.test = r.S("error")
		}
	})
}

// Show builds the token card, the quota, the options and recent parses.
func (m *MinerUSettings) Show(c *ui.Context) {
	cs := design.S()
	tt := design.TT()
	st := m.st
	if st == nil {
		ui.Box(c).FillWidth().Padding(design.SpaceLG).Center().Children(func() { m3.Circular(c, m3.Progress{}) })
		return
	}
	configured := st.IsTrue("configured")
	opts := st.O("options")
	if opts == nil {
		opts = core.J{}
	}
	used, daily, files := st.I("today_pages"), 1000, st.I("today_files")
	if st.Has("daily_pages") {
		daily = st.I("daily_pages")
	}
	exp := st.I("token_exp")
	recent := st.Os("recent")
	withOpt := func(k string, v any) core.J {
		o := opts.Copy()
		o[k] = v
		return core.J{"options": o}
	}
	ui.Column(c).FillWidth().Children(func() {
		m3.Text(c, l10n.MineruIntro, tt.BodySmall.C(cs.OnSurfaceVariant).H(1.6)).Margin(design.SpaceSM, 0)
		m3.VGap(c, design.SpaceSM)
		ui.Column(c.Key("token")).FillWidth().Padding(design.SpaceLG).Background(cs.SurfaceContainerHigh).Radius(design.CornerLG).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				brandIcon(c, "mineru.png", 36, design.CornerSM, false)
				m3.HGap(c, design.SpaceMD)
				ui.Column(c).Grow(1).Basis(0).Children(func() {
					m3.Text(c, "MinerU", tt.TitleMedium)
					sub := l10n.MineruLite
					if configured {
						sub = l10n.MineruPrecise(st.S("token_tail"))
					}
					m3.Text(c, sub, tt.BodySmall.C(cs.OnSurfaceVariant))
				})
				if configured && !m.editing {
					if (m3.Button{Kind: m3.TextKind, Label: l10n.CommonTest, Disabled: m.busy}).Show(c).Clicked() {
						m.runTest()
					}
					if m3.IconBtn(c, icons.EditOutlined, l10n.MineruChangeToken).Clicked() {
						m.editing = true
					}
				}
			})
			if exp > 0 && !m.editing {
				at := time.Unix(int64(exp), 0)
				days := int(time.Until(at).Hours() / 24)
				var text string
				col := cs.OnSurfaceVariant
				if time.Until(at) < 0 {
					text, col = l10n.MineruTokenExpired(mineruDate(at)), cs.Error
				} else {
					left := ""
					if days < 14 {
						left, col = l10n.MineruTokenDaysLeft(days), cs.Error
					}
					text = l10n.MineruTokenExpires(mineruDate(at), left)
				}
				m3.Text(c, text, tt.BodySmall.C(col)).Margin(design.SpaceSM, 0, 0, 48)
			}
			if m.test != "" && !m.editing {
				ic, col := icons.ErrorOutlineRounded, cs.Error
				if m.testOk {
					ic, col = icons.CheckCircleRounded, cs.Primary
				}
				ui.Row(c).FillWidth().AlignItems(ui.Center).Margin(design.SpaceSM, 0, 0, 48).Children(func() {
					m3.Icon(c, ic, 16, col)
					m3.HGap(c, design.SpaceXS)
					m3.Text(c, m.test, tt.BodySmall.C(col)).Grow(1).Basis(0)
				})
			}
			if !configured || m.editing {
				ui.Column(c.Key("edit")).FillWidth().Padding(design.SpaceMD, 0, 0, 0).
					Transition(ui.ElementTransition{Duration: design.Medium2, Ease: design.Emphasized, Enter: &ui.Motion{Opacity: 0, Y: -8}, Shadows: true}).Children(func() {
					f := (m3.Field{Label: "API Token", Hint: l10n.MineruTokenHint, Password: m.obscure, SuffixFn: func() {
						ic := icons.VisibilityOutlined
						if !m.obscure {
							ic = icons.VisibilityOffOutlined
						}
						if m3.IconBtn(c, ic, "").Clicked() {
							m.obscure = !m.obscure
						}
					}}).Show(c, &m.token)
					if f.Submitted() {
						m.saveToken()
					}
					m3.VGap(c, design.SpaceSM)
					ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
						if (m3.Button{Kind: m3.TextKind, Icon: icons.OpenInNewRounded, Label: l10n.MineruGetToken}).Show(c).Clicked() {
							widgets.OpenPlainLink(c, mineruTokenPage)
						}
						ui.Box(c).Grow(1)
						if configured {
							if (m3.Button{Kind: m3.TextKind, Label: l10n.CommonRemove, FG: cs.Error, Disabled: m.busy}).Show(c).Clicked() {
								m.put(core.J{"token": ""}, func() { m.editing = false })
							}
							if (m3.Button{Kind: m3.TextKind, Label: l10n.CommonCancel}).Show(c).Clicked() {
								m.editing = false
							}
						}
						m3.HGap(c, design.SpaceXS)
						if (m3.Button{Kind: m3.Filled, Label: l10n.CommonSave, Disabled: m.busy}).Show(c).Clicked() {
							m.saveToken()
						}
					})
				})
			}
		})
		m3.VGap(c, design.SpaceLG)
		if configured {
			ui.Row(c).FillWidth().AlignItems(ui.End).Children(func() {
				m3.Text(c, l10n.MineruQuotaToday, tt.TitleSmall)
				ui.Box(c).Grow(1)
				col := cs.OnSurface
				if used >= daily {
					col = cs.Error
				}
				m3.Text(c, core.Str(used), tt.TitleMedium.C(col))
				m3.Text(c, l10n.MineruQuotaPages(daily), tt.BodySmall.C(cs.OnSurfaceVariant))
			})
			m3.VGap(c, design.SpaceSM)
			p := m3.Progress{Value: m3.Of(min(max(float64(used)/float64(max(daily, 1)), 0), 1)), Height: 8}
			if used >= daily {
				p.Color = cs.Error
			}
			m3.Linear(c, p)
			m3.VGap(c, design.SpaceXS)
			maxPages := any(200)
			if st.Has("max_pages") {
				maxPages = st["max_pages"]
			}
			m3.Text(c, l10n.MineruQuotaBody(files, daily, core.FmtBytes(st.F("max_bytes")), maxPages), tt.BodySmall.C(cs.OnSurfaceVariant).H(1.5))
			m3.VGap(c, design.SpaceLG)
			m3.Text(c, l10n.MineruModel, tt.TitleSmall)
			m3.VGap(c, design.SpaceSM)
			model := opts.Or("model", "vlm")
			if m3.Segmented(c, []m3.Segment[string]{
				{Value: "vlm", Label: l10n.MineruVlm, Icon: icons.AutoAwesomeOutlined},
				{Value: "pipeline", Label: "Pipeline", Icon: icons.AccountTreeOutlined},
			}, &model, m3.SegOpts{}) {
				m.put(withOpt("model", model), nil)
			}
			m3.VGap(c, design.SpaceXS)
			hint := l10n.MineruVlmHint
			if opts.S("model") == "pipeline" {
				hint = l10n.MineruPipelineHint
			}
			m3.Text(c, hint, tt.BodySmall.C(cs.OnSurfaceVariant))
			if ch, v := switchTile(c, "ocr", m3.Tile{Title: l10n.MineruForceOcr, Subtitle: l10n.MineruForceOcrHint}, opts.IsTrue("ocr")); ch {
				m.put(withOpt("ocr", v), nil)
			}
		} else {
			m3.Text(c, l10n.MineruLiteBody, tt.BodySmall.C(cs.OnSurfaceVariant).H(1.5))
			m3.VGap(c, design.SpaceLG)
		}
		if ch, v := switchTile(c, "auto", m3.Tile{Title: l10n.MineruAutoParse, Subtitle: l10n.MineruAutoParseHint}, st.IsTrue("auto")); ch {
			m.put(core.J{"auto": v}, nil)
		}
		if len(recent) > 0 {
			m3.VGap(c, design.SpaceSM)
			m3.Text(c, l10n.MineruRecent, tt.TitleSmall)
			m3.VGap(c, design.SpaceXS)
			for i, u := range recent {
				if i == 5 {
					break
				}
				ui.Row(c.Key(i)).FillWidth().AlignItems(ui.Center).Padding(3, 0).Children(func() {
					ic, col := icons.ErrorOutlineRounded, cs.Error
					if u.IsTrue("ok") {
						ic, col = icons.CheckCircleOutlineRounded, cs.Primary
					}
					m3.Icon(c, ic, 16, col)
					m3.HGap(c, design.SpaceSM)
					m3.Clamp(m3.Text(c, u.S("file"), tt.BodySmall), 1).Grow(1).Basis(0)
					var parts []string
					if u.IsTrue("ok") {
						parts = append(parts, l10n.CommonPages(u["pages"]))
					}
					if u.S("mode") == "flash" {
						parts = append(parts, l10n.MineruModeLite)
					} else {
						parts = append(parts, l10n.MineruModePrecise)
					}
					parts = append(parts, core.FmtRelative(time.Unix(int64(u.I("at")), 0)))
					m3.Text(c, strings.Join(parts, " · "), tt.LabelSmall.C(cs.OnSurfaceVariant))
				})
			}
		}
	})
}
