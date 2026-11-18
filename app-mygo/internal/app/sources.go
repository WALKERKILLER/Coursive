package app

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// SourcesPage is sources_page.dart's 课程 → 来源: installed source
// plugins, each source's sign-in state, installing a plugin from a folder
// and removing one.
// Contract: docs/specs/0024-source-provider-plugins.md
type SourcesPage struct {
	scroll  ui.ScrollState
	refresh ui.RefreshState
}

// NewSourcesPage shows the source plugins.
func NewSourcesPage() m3.Page { return &SourcesPage{} }

// MainScroll is the page's scroll.
func (p *SourcesPage) MainScroll() *ui.ScrollState { return &p.scroll }

// View builds the page.
func (p *SourcesPage) View(c *ui.Context) {
	cs := design.S()
	tt := design.TT()
	pad := design.SpaceXL
	if Compact() {
		pad = design.SpaceLG
	}
	m3.Scaffold{
		AppBar: &m3.AppBar{Title: l10n.SourcesTitle, Scrolled: p.scroll.Y > 0},
		Body: func() {
			sc := ui.Scroll(c).Fill().TrackScroll(&p.scroll)
			if sc.PullToRefresh(&p.refresh) {
				State.LoadSourcesThen(func() { p.refresh.Refreshing = false })
			}
			sc.Children(func() {
				if len(State.Plugins) == 0 {
					ui.Column(c).FillWidth().MinHeight(WindowHeight - 64).Children(func() {
						widgets.EmptyState{Icon: icons.ExtensionOutlined, Title: l10n.SourcesEmptyTitle, Body: l10n.SourcesEmptyBody, Action: func() {
							if (m3.Button{Kind: m3.Filled, Icon: icons.CreateNewFolderOutlined, Label: l10n.SourcesInstall}).Show(c).Clicked() {
								InstallSourcePlugin()
							}
						}}.Show(c)
					})
					return
				}
				ui.Column(c).FillWidth().Padding(design.SpaceSM, pad, design.SpaceHuge, pad).Children(func() {
					widgets.ContentWidth(c, 840, func() {
						ui.Column(c).FillWidth().Children(func() {
							m3.Text(c, l10n.SourcesIntro, tt.BodyMedium.C(cs.OnSurfaceVariant))
							m3.VGap(c, design.SpaceXL)
							for _, pl := range State.Plugins {
								ui.Box(c.Key(pl.ID())).FillWidth().Children(func() { sourcePluginCard(c, pl) })
								m3.VGap(c, design.SpaceLG)
							}
							ui.Row(c).Children(func() {
								if (m3.Button{Kind: m3.Outlined, Icon: icons.CreateNewFolderOutlined, Label: l10n.SourcesInstall}).Show(c).Clicked() {
									InstallSourcePlugin()
								}
							})
						})
					})
				})
			})
		},
	}.Show(c)
}

func apiMessage(err error) string {
	if e, ok := err.(*core.ApiError); ok {
		return e.Message
	}
	return err.Error()
}

// InstallSourcePlugin installs a source plugin from a folder after the user
// reviews what it may reach, then offers to sign its first source in.
func InstallSourcePlugin() {
	widgets.PickDirectory("", l10n.CommonChoose, l10n.SourcesChooseFolder, func(dir string) {
		if dir == "" {
			return
		}
		api := State.Api()
		core.Async(func() (any, error) { return api.Post("/api/sources/preview", core.J{"path": dir}) }, func(v any, err error) {
			if err != nil {
				widgets.ShowSnack(l10n.SourcesCannotRead(err.Error()), "", nil)
				return
			}
			consentAndInstall(core.SourcePlugin{J: core.Obj(v)}, func() error {
				_, err := api.Post("/api/sources/install", core.J{"path": dir})
				return err
			}, nil)
		})
	})
}

// InstallListedSource installs source plugin id from the plugin index: the
// daemon downloads and verifies it, the user reviews what it may reach,
// then it is moved into place. onStaged runs once the download is verified
// or failed; done gets whether it was installed.
func InstallListedSource(id string, onStaged func(), done func(bool)) {
	api := State.Api()
	core.Async(func() (any, error) { return api.Post("/api/sources/market/"+core.Esc(id)+"/stage", nil) }, func(v any, err error) {
		if onStaged != nil {
			onStaged()
		}
		if err != nil {
			widgets.ShowSnack(l10n.SourcesCannotInstall(err.Error()), "", nil)
			if done != nil {
				done(false)
			}
			return
		}
		staged := core.Obj(v)
		consentAndInstall(core.SourcePlugin{J: core.Obj(staged["plugin"])}, func() error {
			_, err := api.Post("/api/sources/market/install", core.J{"token": staged["token"]})
			return err
		}, done)
	})
}

// consentAndInstall shows what p may reach, runs install once the user
// agrees, then offers to sign its first source in.
func consentAndInstall(p core.SourcePlugin, install func() error, done func(bool)) {
	finish := func(ok bool) {
		if done != nil {
			done(ok)
		}
	}
	m3.AdaptiveSheet(nil, 0, true, func(c *ui.Context, r *m3.Route) { sourceConsent(c, r, p) }, func(v any) {
		if v != true {
			finish(false)
			return
		}
		core.Async(func() (struct{}, error) { return struct{}{}, install() }, func(_ struct{}, err error) {
			if err != nil {
				widgets.ShowSnack(l10n.SourcesCannotInstall(err.Error()), "", nil)
				finish(false)
				return
			}
			State.LoadSourcesThen(func() {
				widgets.ShowSnack(l10n.SourcesInstalled(p.Title()), "", nil)
				for _, x := range State.Plugins {
					if x.ID() != p.ID() {
						continue
					}
					if srcs := x.Sources(); len(srcs) > 0 && len(srcs[0].Fields()) > 0 && !srcs[0].SignedIn() {
						SignInSource(srcs[0])
					}
					break
				}
				finish(true)
			})
		})
	})
}

// RemoveSourcePlugin uninstalls a source plugin after confirmation; done
// gets whether it was removed.
func RemoveSourcePlugin(p core.SourcePlugin, done func(bool)) {
	finish := func(ok bool) {
		if done != nil {
			done(ok)
		}
	}
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		m3.AlertDialog(c, m3.Alert{Icon: icons.DeleteOutlineRounded, Title: l10n.SourcesRemoveTitle(p.Title()), Message: l10n.SourcesRemoveBody, Actions: func() {
			if m3.TextButton(c, l10n.CommonCancel).Clicked() {
				r.Pop(false)
			}
			if m3.FilledButton(c, l10n.SourcesRemovePlugin).Clicked() {
				r.Pop(true)
			}
		}})
	}, func(v any) {
		if v != true {
			finish(false)
			return
		}
		api := State.Api()
		core.Async(func() (any, error) { return api.Delete("/api/sources/" + core.Esc(p.ID())) }, func(_ any, err error) {
			if err != nil {
				widgets.ShowSnack(err.Error(), "", nil)
				finish(false)
				return
			}
			State.LoadSourcesThen(func() {
				State.LoadCourses(false, true)
				finish(true)
			})
		})
	})
}

// SignInSource signs a source in and syncs its courses.
func SignInSource(s core.Source) {
	ShowSignInSheet(s, func(ok bool) {
		if !ok {
			return
		}
		widgets.ShowSnack(l10n.SourceSignedIn(s.Title()), "", nil)
		State.LoadSourcesThen(func() { State.LoadCourses(true, false) })
	})
}

// sourceConsent is _Consent: what a plugin may do, shown before it is
// installed.
func sourceConsent(c *ui.Context, r *m3.Route, p core.SourcePlugin) {
	cs := design.S()
	tt := design.TT()
	icon := p.Icon()
	if icon == "" {
		icon = "school"
	}
	ui.Scroll(c).FillWidth().Shrink(1).Children(func() {
		ui.Column(c).FillWidth().Padding(m3.SheetPadding(c, true)).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				widgets.PluginGlyph{Icon: icon, ID: p.ID(), Size: 48}.Show(c)
				m3.HGap(c, design.SpaceLG)
				m3.Text(c, l10n.SourcesConsentTitle(p.Title()), tt.HeadlineSmall).Grow(1).Basis(0)
			})
			m3.VGap(c, design.SpaceXS)
			m3.Text(c, l10n.SourcesVersion(p.Version()), tt.BodySmall.C(cs.OnSurfaceVariant))
			if p.Summary() != "" {
				m3.VGap(c, design.SpaceLG)
				m3.Text(c, p.Summary(), tt.BodyLarge)
			}
			m3.VGap(c, design.SpaceXL)
			m3.Text(c, l10n.SourcesConsentHosts, tt.TitleSmall)
			m3.VGap(c, design.SpaceSM)
			ui.Row(c).Wrap().Gap(design.SpaceSM).Children(func() {
				for _, h := range p.Hosts() {
					m3.Chip{Kind: m3.PlainChip, Avatar: icons.LanguageRounded, Label: h}.Show(c)
				}
			})
			m3.VGap(c, design.SpaceXL)
			ui.Row(c).FillWidth().Padding(design.SpaceLG).Radius(design.CornerLG).Background(cs.SurfaceContainerHighest).Children(func() {
				m3.Icon(c, icons.ShieldOutlined, 24, cs.OnSurfaceVariant)
				m3.HGap(c, design.SpaceMD)
				m3.Text(c, l10n.SourcesConsentBody, tt.BodyMedium).Grow(1).Basis(0)
			})
			if !p.Available() {
				m3.VGap(c, design.SpaceLG)
				widgets.InlineError(c, l10n.SourcesUnavailable)
			}
			m3.VGap(c, design.SpaceXL)
			ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
				if m3.TextButton(c, l10n.CommonCancel).Clicked() {
					r.Pop(false)
				}
				m3.HGap(c, design.SpaceSM)
				if (m3.Button{Kind: m3.Filled, Label: l10n.SourcesInstallPlugin, Disabled: !p.Available()}).Show(c).Clicked() {
					r.Pop(true)
				}
			})
		})
	})
}

func sourcePluginCard(c *ui.Context, p core.SourcePlugin) {
	cs := design.S()
	tt := design.TT()
	icon := p.Icon()
	if icon == "" {
		icon = "school"
	}
	ui.Column(c).FillWidth().Radius(design.CornerXL).Background(cs.SurfaceContainerLow).Padding(design.SpaceLG, design.SpaceSM, design.SpaceLG, design.SpaceXL).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Start).Children(func() {
			widgets.PluginGlyph{Icon: icon, ID: p.ID(), Size: 48}.Show(c)
			m3.HGap(c, design.SpaceLG)
			ui.Column(c).Grow(1).Basis(0).Children(func() {
				m3.Text(c, p.Title(), tt.TitleLarge)
				m3.VGap(c, design.SpaceXXS)
				line := l10n.SourcesVersion(p.Version())
				if !p.Enabled() {
					line += " · " + l10n.SourcesDisabled
				}
				m3.Text(c, line, tt.BodySmall.C(cs.OnSurfaceVariant))
			})
			m3.MenuButton(c, "more", 0, l10n.CommonMore, m3.Over, func() []m3.MenuItem {
				toggle := l10n.CommonEnable
				if p.Enabled() {
					toggle = l10n.CommonDisable
				}
				return []m3.MenuItem{
					{Label: toggle, OnTap: func() {
						api := State.Api()
						core.Async(func() (any, error) {
							return api.Put("/api/sources/"+core.Esc(p.ID()), core.J{"enabled": !p.Enabled()})
						}, func(_ any, err error) {
							if err != nil {
								widgets.ShowSnack(err.Error(), "", nil)
							}
							State.LoadSources()
						})
					}},
					{Label: l10n.SourcesRemovePlugin, OnTap: func() { RemoveSourcePlugin(p, nil) }},
				}
			})
		})
		if p.Summary() != "" {
			m3.VGap(c, design.SpaceSM)
			m3.Text(c, p.Summary(), tt.BodyMedium.C(cs.OnSurfaceVariant)).Padding(0, design.SpaceLG, 0, 0)
		}
		if p.Enabled() {
			for _, s := range p.Sources() {
				m3.VGap(c, design.SpaceMD)
				ui.Box(c.Key(s.ID())).FillWidth().Children(func() { sourceRow(c, s) })
			}
		}
	})
}

func sourceRow(c *ui.Context, s core.Source) {
	cs := design.S()
	failed := s.Process() == "error"
	var ic icons.Icon
	var color ui.Color
	var status string
	switch {
	case failed:
		ic, color, status = icons.ErrorOutlineRounded, cs.Error, l10n.SourceFailed(s.Error())
	case s.Expired():
		ic, color, status = icons.LockClockOutlined, cs.Error, l10n.SourceExpired
	case s.SignedIn():
		ic, color, status = icons.CheckCircleRounded, cs.Primary, l10n.SourceSignedInPlain
		if s.Account() != "" {
			status = l10n.SourceSignedInAs(s.Account())
		}
	default:
		ic, color, status = icons.RadioButtonUncheckedRounded, cs.OnSurfaceVariant, l10n.SourceSignedOut
	}
	api := State.Api()
	post := func(path string) {
		core.Async(func() (any, error) { return api.Post("/api/sources/"+core.Esc(s.ID())+path, nil) }, func(_ any, err error) {
			if err != nil {
				widgets.ShowSnack(err.Error(), "", nil)
			}
			State.LoadSources()
		})
	}
	ui.Column(c).FillWidth().Children(func() {
		t := m3.Tile{Pad: true, PadX: 0, LeadingIcon: ic, IconColor: color, Title: s.Title(), Subtitle: status, Trailing: func() {
			switch {
			case failed:
				if (m3.Button{Kind: m3.Tonal, Label: l10n.SourceRestart}).Show(c).Clicked() {
					post("/restart")
				}
			case s.SignedIn():
				if m3.TextButton(c, l10n.SourceSignOut).Clicked() {
					post("/sign-out")
				}
			default:
				label := l10n.LoginSignIn
				if s.Expired() {
					label = l10n.LoginSignInAgain
				}
				if (m3.Button{Kind: m3.Tonal, Label: label, Disabled: len(s.Fields()) == 0}).Show(c).Clicked() {
					SignInSource(s)
				}
			}
		}}
		if failed || s.Expired() {
			t.SubtitleFn = func() { m3.Text(c, status, design.TT().BodyMedium.C(cs.Error)) }
		}
		ui.Box(c).FillWidth().Padding(0, design.SpaceSM, 0, 0).Children(func() { t.Show(c) })
		if s.SignedIn() && s.HasLms() {
			ui.Row(c).Padding(0, 0, 0, 56).Children(func() {
				if (m3.Button{Kind: m3.TextKind, Icon: icons.AssignmentOutlined, Label: l10n.LmsTitle}).Show(c).Clicked() {
					m3.Nav().Push(NewLmsPage())
				}
			})
		}
	})
}

// ShowSignInSheet signs a source in with the form its provider declares,
// then any challenge it asks for; done gets true once signed in.
// Contract: docs/specs/0024-source-provider-plugins.md
func ShowSignInSheet(s core.Source, done func(bool)) {
	sh := &signIn{source: s, values: map[string]*string{}, challenge: s.Challenge()}
	for _, f := range s.Fields() {
		v := ""
		sh.values[f.Key()] = &v
	}
	m3.AdaptiveSheet(nil, 0, false, sh.view, func(v any) { done(v == true) })
}

type signIn struct {
	source    core.Source
	values    map[string]*string
	answer    string
	challenge *core.SignInChallenge
	busy      bool
	err       string
	invalid   map[string]string
}

func (s *signIn) validate() bool {
	s.invalid = map[string]string{}
	if s.challenge == nil {
		for _, f := range s.source.Fields() {
			if strings.TrimSpace(*s.values[f.Key()]) == "" {
				s.invalid[f.Key()] = l10n.SourceFieldRequired(f.Title())
			}
		}
	} else if s.challenge.Kind() != "qr" && strings.TrimSpace(s.answer) == "" {
		s.invalid["answer"] = l10n.SourceFieldRequired(l10n.SourceChallengeAnswer)
	}
	return len(s.invalid) == 0
}

func (s *signIn) submit(r *m3.Route) {
	if s.busy || !s.validate() {
		return
	}
	s.busy, s.err = true, ""
	api := State.Api()
	path := "/api/sources/" + core.Esc(s.source.ID())
	var body core.J
	if ch := s.challenge; ch == nil {
		fields := core.J{}
		for k, v := range s.values {
			fields[k] = strings.TrimSpace(*v)
		}
		path, body = path+"/sign-in", core.J{"fields": fields}
	} else {
		path, body = path+"/answer", core.J{"challenge": ch.ID(), "answer": strings.TrimSpace(s.answer)}
	}
	core.Async(func() (any, error) { return api.Post(path, body) }, func(v any, err error) {
		s.busy = false
		if err != nil {
			s.err = apiMessage(err)
			return
		}
		auth := core.Obj(v)
		if auth.S("state") == "signed_in" {
			r.Pop(true)
			return
		}
		s.answer = ""
		s.challenge = nil
		if ch := auth.O("challenge"); ch != nil {
			s.challenge = &core.SignInChallenge{J: ch}
		} else {
			s.err = l10n.SourceSignInIncomplete
		}
	})
}

func (s *signIn) view(c *ui.Context, r *m3.Route) {
	cs := design.S()
	tt := design.TT()
	ch := s.challenge
	fields := s.source.Fields()
	ui.Scroll(c).FillWidth().Shrink(1).Children(func() {
		ui.Column(c).FillWidth().Padding(m3.SheetPadding(c, true)).Children(func() {
			m3.Text(c, l10n.SourceSignInTitle(s.source.Title()), tt.HeadlineSmall)
			m3.VGap(c, design.SpaceSM)
			body := l10n.SourceSignInBody
			if ch != nil {
				body = ch.Prompt()
			}
			m3.Text(c, body, tt.BodyMedium.C(cs.OnSurfaceVariant))
			m3.VGap(c, design.SpaceXL)
			if ch == nil {
				for i, f := range fields {
					if i > 0 {
						m3.VGap(c, design.SpaceLG)
					}
					helper := ""
					if !f.Remember() {
						helper = l10n.SourceFieldNotStored
					}
					in := m3.Field{Label: f.Title(), AutoFocus: i == 0, Disabled: s.busy, Password: f.Kind() == "password",
						Helper: helper, Error: s.invalid[f.Key()]}.Show(c.Key(f.Key()), s.values[f.Key()])
					if i == len(fields)-1 && in.Submitted() {
						s.submit(r)
					}
				}
			} else {
				img := widgets.DataImage(ch.Image())
				if img != nil {
					ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
						// A challenge image is shown at its natural size.
						ui.Image(c, img).Radius(design.CornerSM).Clip().Label(ch.Prompt())
					})
				}
				if ch.Kind() != "qr" {
					if img != nil {
						m3.VGap(c, design.SpaceLG)
					}
					if len(s.answer) > 64 {
						s.answer = s.answer[:64]
					}
					in := m3.Field{Label: l10n.SourceChallengeAnswer, AutoFocus: true, Disabled: s.busy, MaxLength: 64,
						Error: s.invalid["answer"]}.Show(c.Key("answer-"+ch.ID()), &s.answer)
					if in.Submitted() {
						s.submit(r)
					}
				}
			}
			if s.err != "" {
				m3.VGap(c, design.SpaceLG)
				widgets.InlineError(c, s.err)
			}
			m3.VGap(c, design.SpaceXL)
			ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
				if (m3.Button{Kind: m3.TextKind, Label: l10n.CommonCancel, Disabled: s.busy}).Show(c).Clicked() {
					r.Pop(false)
				}
				m3.HGap(c, design.SpaceSM)
				label := l10n.LoginSignIn
				if ch != nil {
					label = l10n.SourceSignInContinue
				}
				b := m3.Button{Kind: m3.Filled, Label: label, Disabled: s.busy}
				if s.busy {
					b.Child = func(fg ui.Color) { m3.Spinner(c, 20, 2.5, cs.OnPrimary) }
				}
				if b.Show(c).Clicked() {
					s.submit(r)
				}
			})
		})
	})
}
