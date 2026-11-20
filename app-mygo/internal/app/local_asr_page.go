package app

import (
	"fmt"
	"math"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// lectureMinutes is the minutes to transcribe a 90-minute lecture at the
// measured speed.
func lectureMinutes(m core.J) float64 { return m.O("measured").F("rtf") * 90 }

func asrGB(mb float64) string {
	if mb >= 1024 {
		return core.Fixed(mb/1024, 1) + " GB"
	}
	return fmt.Sprintf("%d MB", int(math.Round(mb)))
}

func asrTagLabel(t string) string {
	switch t {
	case "recommended":
		return l10n.AsrModelRecommended
	case "fast":
		return l10n.AsrModelFastest
	case "accurate":
		return l10n.AsrModelAccurate
	}
	return ""
}

// benchMetrics is the benchmark metrics shown on the board, in board
// order.
func benchMetrics(reference core.J) []BenchMetric {
	machine := reference.Or("machine", l10n.BenchReferenceMachine)
	return []BenchMetric{
		{Label: l10n.BenchCer, Hint: l10n.BenchLowerIsBetter, Format: func(v float64) string { return core.Fixed(v, 1) + "%" }, Source: l10n.BenchCerSource},
		{Label: l10n.BenchTime, Hint: l10n.BenchTimeHint, Format: func(v float64) string {
			if v < 10 {
				return l10n.BenchMinutes(core.Fixed(v, 1))
			}
			return l10n.BenchMinutes(int(math.Round(v)))
		}, Source: l10n.BenchTimeSource(machine, reference.S("clip"))},
		{Label: l10n.BenchMemory, Hint: l10n.BenchMemoryHint, Format: func(v float64) string { return core.Fixed(v, 1) + " GB" }, Source: l10n.BenchMemorySource},
	}
}

func benchRows(models []core.J) []BenchRow {
	var rows []BenchRow
	for _, m := range models {
		r := BenchRow{ID: m.S("id"), Name: m.S("name"), Values: make([]*float64, 3)}
		for _, t := range m.Ss("tags") {
			if t == "recommended" {
				r.Tag = l10n.AsrModelRecommended
			}
		}
		if b := m.O("bench"); b.Has("cer") && b["cer"] != nil {
			v := b.F("cer")
			r.Values[0] = &v
		}
		if v := lectureMinutes(m); v > 0 {
			r.Values[1] = &v
		}
		if mb := m.O("measured").F("peak_mb"); mb > 0 {
			v := mb / 1024
			r.Values[2] = &v
		}
		rows = append(rows, r)
	}
	return rows
}

// LocalAsrPage is on-device recognition: pick a model with the benchmark
// board, download it from a chosen source and make it the "local"
// transcription channel.
type LocalAsrPage struct {
	st                core.J
	err               error
	focus             string
	focusMoved        bool
	custom, proxy     string
	customOn, proxyOn bool
	threadsDrag       *float64
	board             AsrBenchBoard
	scroll            ui.ScrollState
	cancel, closed    func()
}

// NewLocalAsrPage opens the page; closed runs as it is popped.
func NewLocalAsrPage(closed func()) m3.Page {
	p := &LocalAsrPage{closed: closed}
	p.load()
	p.cancel = State.Events.Listen(func(e core.J) {
		switch e.S("type") {
		case "asr_local":
			d := e.O("data")
			if st := p.st.O("status"); st != nil {
				st["install"] = d
			}
			if d.S("state") != "running" {
				p.load()
			}
		case "settings_changed":
			p.load()
		}
	})
	return p
}

// Dispose stops following the events.
func (p *LocalAsrPage) Dispose() {
	p.cancel()
	if p.closed != nil {
		p.closed()
	}
}

// MainScroll is the page's scroll.
func (p *LocalAsrPage) MainScroll() *ui.ScrollState { return &p.scroll }

func (p *LocalAsrPage) load() {
	core.Async(func() (any, error) { return api().Get("/api/asr/local") }, func(v any, err error) {
		if err != nil {
			p.err = err
			return
		}
		r := core.Obj(v)
		p.st, p.err = r, nil
		if p.focus == "" {
			p.focus = r.S("active")
			if p.focus == "" {
				for _, m := range r.O("status").Os("models") {
					for _, t := range m.Ss("tags") {
						if t == "recommended" && p.focus == "" {
							p.focus = m.S("id")
						}
					}
				}
			}
		}
		if p.custom == "" {
			p.custom = r.S("custom_url")
		}
		if p.proxy == "" {
			p.proxy = r.S("github_proxy")
		}
	})
}

func (p *LocalAsrPage) set(key string, value any) {
	core.Async(func() (any, error) { return api().Put("/api/settings", core.J{key: value}) }, func(_ any, err error) {
		if err != nil {
			widgets.ShowSnack(l10n.CommonSaveFailed(apiMessage(err)), "", nil)
			return
		}
		p.load()
	})
}

func (p *LocalAsrPage) call(method, path string, body any) {
	core.Async(func() (any, error) { return api().Call(method, path, body) }, func(_ any, err error) {
		if err != nil {
			widgets.ShowSnack(apiMessage(err), "", nil)
			return
		}
		p.load()
	})
}

func (p *LocalAsrPage) remove(m core.J) {
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		m3.AlertDialog(c, m3.Alert{Icon: icons.DeleteOutlineRounded, Title: l10n.AsrModelDeleteTitle(m.S("name")),
			Message: l10n.AsrModelDeleteBody(core.FmtBytes(m.F("size"))), Actions: func() {
				if m3.TextButton(c, l10n.CommonCancel).Clicked() {
					r.Pop(false)
				}
				if m3.FilledButton(c, l10n.CommonDelete).Clicked() {
					r.Pop(true)
				}
			}})
	}, func(v any) {
		if v == true {
			p.call("DELETE", "/api/asr/local/models/"+core.Esc(m.S("id")), nil)
		}
	})
}

// relocate picks a new folder for the engine and models; it asks whether
// the downloaded ones should move along. dir skips the picker (恢复默认).
func (p *LocalAsrPage) relocate(loc core.J, dir string) {
	current := loc.S("dir")
	apply := func(dir string, move bool) {
		core.Async(func() (any, error) {
			return api().Put("/api/asr/local/location", core.J{"dir": dir, "move": move})
		}, func(v any, err error) {
			if err != nil {
				widgets.ShowSnack(apiMessage(err), "", nil)
				return
			}
			st := p.st.Copy()
			st["status"] = core.Obj(v)
			p.st = st
			if !move {
				widgets.ShowSnack(l10n.AsrModelLocationChanged, "", nil)
			}
		})
	}
	choose := func(dir string) {
		if dir == "" || dir == current {
			return
		}
		used := loc.F("used")
		if used <= 0 {
			apply(dir, false)
			return
		}
		m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
			cs := design.S()
			tt := design.TT()
			m3.AlertDialog(c, m3.Alert{Icon: icons.DriveFileMoveOutline, Title: l10n.AsrMoveTitle, Width: 440, Content: func() {
				ui.Box(c).FillWidth().Padding(design.SpaceSM, design.SpaceMD).Radius(design.CornerMD).Background(cs.SurfaceContainerHighest).Children(func() {
					m3.Text(c, dir, tt.BodyMedium)
				})
				m3.VGap(c, design.SpaceLG)
				m3.Text(c, l10n.AsrMoveBody(core.FmtBytes(used)), tt.BodyMedium.C(cs.OnSurfaceVariant))
				m3.VGap(c, design.SpaceSM)
				m3.Text(c, l10n.AsrChangeOnlyBody, tt.BodyMedium.C(cs.OnSurfaceVariant))
			}, Actions: func() {
				if m3.TextButton(c, l10n.CommonCancel).Clicked() {
					r.Close()
				}
				if m3.TextButton(c, l10n.AsrChangeOnly).Clicked() {
					r.Pop(false)
				}
				if m3.FilledButton(c, l10n.AsrMove).Clicked() {
					r.Pop(true)
				}
			}})
		}, func(v any) {
			if move, ok := v.(bool); ok {
				apply(dir, move)
			}
		})
	}
	if dir != "" {
		choose(dir)
		return
	}
	widgets.PickDirectory(current, "", l10n.AsrModelLocation, choose)
}

// View builds the page.
func (p *LocalAsrPage) View(c *ui.Context) {
	m3.Scaffold{AppBar: &m3.AppBar{Title: l10n.AsrLocalTitle, Scrolled: p.scroll.Y > 0}, Body: func() {
		switch {
		case p.st != nil:
			p.body(c, p.st)
		case p.err != nil:
			widgets.ErrorState(c, p.err, p.load)
		default:
			widgets.CenteredLoader(c, "")
		}
	}}.Show(c)
}

func (p *LocalAsrPage) body(c *ui.Context, st core.J) {
	cs := design.S()
	tt := design.TT()
	status := st.O("status")
	if !status.IsTrue("supported") {
		widgets.EmptyState{Icon: icons.MemoryRounded, Title: l10n.AsrUnsupportedTitle, Body: l10n.AsrUnsupportedBody(status["platform"])}.Show(c)
		return
	}
	models := status.Os("models")
	install := status.O("install")
	if install == nil {
		install = core.J{}
	}
	active := st.S("active")
	var activeModel core.J
	for _, m := range models {
		if m.S("id") == active {
			activeModel = m
		}
	}
	loc := status.O("location")
	page := ui.Scroll(c).Fill().TrackScroll(&p.scroll)
	page.Children(func() {
		ui.Column(c).FillWidth().Padding(design.SpaceSM, design.SpaceLG, design.SpaceHuge, design.SpaceLG).Children(func() {
			widgets.ContentWidth(c, 960, func() {
				localAsrHero(c, activeModel, install, models)
				m3.VGap(c, design.SpaceXL)
				m3.Text(c, l10n.AsrHowToChoose, tt.TitleMedium)
				m3.VGap(c, design.SpaceXS)
				m3.Text(c, l10n.AsrHowToChooseBody, tt.BodyMedium.C(cs.OnSurfaceVariant))
				m3.VGap(c, design.SpaceMD)
				m3.Card(c, m3.CardOutlined).FillWidth().Padding(design.SpaceLG).Children(func() {
					p.board.Show(c, benchMetrics(status.O("reference")), benchRows(models), p.focus, func(id string) {
						p.focus, p.focusMoved = id, true
					})
				})
				m3.VGap(c, design.SpaceXL)
				m3.Text(c, l10n.CommonModel, tt.TitleMedium)
				m3.VGap(c, design.SpaceSM)
				for _, m := range models {
					m := m
					id := m.S("id")
					var inst core.J
					if install.S("model") == id {
						inst = install
					}
					card := ui.Box(c.Key("model:"+id)).FillWidth().Margin(0, 0, design.SpaceSM, 0)
					// _select's ensureVisible: alignment 0.3 over medium2
					// along emphasized.
					if p.focusMoved && p.focus == id {
						if p.focusMoved = !ensureVisible(&p.scroll, page, card, .3, design.T().Motion(design.Medium2, design.Emphasized)); p.focusMoved {
							c.AnimationFrame()
						}
					}
					card.Children(func() {
						p.modelCard(c, m, p.focus == id, active == id, inst, install.S("state") == "running")
					})
				}
				m3.VGap(c, design.SpaceXL)
				m3.Text(c, l10n.AsrStorage, tt.TitleMedium)
				m3.VGap(c, design.SpaceSM)
				p.storage(c, loc, install)
				m3.VGap(c, design.SpaceXL)
				m3.Text(c, l10n.AsrSource, tt.TitleMedium)
				m3.VGap(c, design.SpaceXS)
				m3.Text(c, l10n.AsrSourceVerify, tt.BodyMedium.C(cs.OnSurfaceVariant))
				m3.VGap(c, design.SpaceSM)
				p.sources(c, status.Os("sources"), st.Or("source", "hf-mirror"))
				m3.VGap(c, design.SpaceXL)
				p.threads(c, st, status)
			})
		})
	})
}

func localAsrHero(c *ui.Context, active, install core.J, models []core.J) {
	cs := design.S()
	tt := design.TT()
	running := install.S("state") == "running"
	moving := running && install.S("model") == ""
	var ic icons.Icon
	var title, body string
	switch {
	case moving:
		ic, title, body = icons.DriveFileMoveOutline, l10n.AsrMoving, l10n.AsrMovingBody
	case active != nil:
		ic, title, body = icons.OfflineBoltRounded, l10n.AsrUsingModel(active.S("name")), l10n.AsrUsingModelBody
	case running:
		name := l10n.CommonModel
		for _, m := range models {
			if m.S("id") == install.S("model") {
				name = m.S("name")
			}
		}
		ic, title, body = icons.DownloadingRounded, l10n.AsrDownloadingModel(name), l10n.AsrDownloadingModelBody
	default:
		ic, title, body = icons.ComputerRounded, l10n.AsrLocalHeroTitle, l10n.AsrLocalHeroBody
	}
	bg, icCol, titleCol, bodyCol := cs.SurfaceContainerHigh, cs.Primary, cs.OnSurface, cs.OnSurfaceVariant
	if active != nil {
		bg, icCol, titleCol, bodyCol = cs.PrimaryContainer, cs.OnPrimaryContainer, cs.OnPrimaryContainer, cs.OnPrimaryContainer
	}
	ui.Row(c).FillWidth().AlignItems(ui.Start).Padding(design.SpaceXL).Background(bg).Radius(design.CornerXL).Children(func() {
		m3.Icon(c, ic, 32, icCol)
		m3.HGap(c, design.SpaceLG)
		ui.Column(c).Grow(1).Basis(0).Children(func() {
			m3.Text(c, title, tt.TitleLarge.C(titleCol))
			m3.VGap(c, design.SpaceXS)
			m3.Text(c, body, tt.BodyMedium.C(bodyCol).H(1.5))
		})
	})
}

func asrTag(c *ui.Context, text string, strong bool) {
	cs := design.S()
	bg, fg := cs.SecondaryContainer, cs.OnSecondaryContainer
	if strong {
		bg, fg = cs.PrimaryContainer, cs.OnPrimaryContainer
	}
	ui.Box(c.Key("tag:"+text)).Padding(2, design.SpaceSM).Radius(design.CornerFull).Background(bg).Children(func() {
		m3.Text(c, text, design.TT().LabelSmall.C(fg))
	})
}

func (p *LocalAsrPage) modelCard(c *ui.Context, m core.J, focused, active bool, install core.J, busy bool) {
	cs := design.S()
	tt := design.TT()
	id := m.S("id")
	installed := m.IsTrue("installed")
	bench := m.O("bench")
	size, extra := m.F("size"), m.F("extra")
	mins := lectureMinutes(m)
	running := install.S("state") == "running"
	failed := install.S("state") == "failed"
	facts := []string{l10n.CommonDownloadSize(core.FmtBytes(size))}
	if mb := m.O("measured").F("peak_mb"); mb > 0 {
		facts = append(facts, l10n.AsrModelMemory(asrGB(mb)))
	}
	if mins > 0 {
		if mins < 10 {
			facts = append(facts, l10n.AsrModelSpeed(core.Fixed(mins, 1)))
		} else {
			facts = append(facts, l10n.AsrModelSpeed(int(math.Round(mins))))
		}
	}
	facts = append(facts, l10n.AsrModelCer(bench["cer"]))
	bg := cs.SurfaceContainerLow
	if active {
		bg = cs.SecondaryContainer
	}
	border := ui.Transparent
	if focused {
		border = cs.Primary
	}
	e := ui.ButtonBase(c).FocusRing(false).Cursor(ui.CursorPointer).Column().Justify(ui.Start).AlignItems(ui.Stretch).FillWidth().Radius(design.CornerLG).Background(bg).Border(2, border).
		Padding(design.SpaceMD, design.SpaceMD, design.SpaceMD, design.SpaceLG).Transition(ui.ElementTransition{Duration: design.Medium1, Ease: design.Standard, Colors: true})
	m3.InkWell(e, design.CornerLG)
	e.Children(func() {
		ui.Row(c).FillWidth().Wrap().GapX(design.SpaceSM).GapY(design.SpaceXS).AlignItems(ui.Center).Children(func() {
			if mark := asrModelMark(id); mark != nil {
				asrMarkImage(c, *mark, 20)
			}
			m3.Text(c, m.S("name"), tt.TitleMedium)
			for _, t := range m.Ss("tags") {
				if l := asrTagLabel(t); l != "" {
					asrTag(c, l, t == "recommended")
				}
			}
			if active {
				asrTag(c, l10n.CommonInUse, true)
			} else if installed {
				asrTag(c, l10n.CommonDownloaded, false)
			}
		})
		m3.VGap(c, 2)
		punct := l10n.AsrModelAutoPunctuation
		if m.IsTrue("punct") {
			punct = l10n.AsrModelPunctuated
		}
		m3.Text(c, m.S("org")+" · "+m.S("langs")+" · "+punct, tt.BodySmall.C(cs.OnSurfaceVariant))
		m3.VGap(c, design.SpaceSM)
		m3.Text(c, m.S("summary"), tt.BodyMedium)
		m3.VGap(c, design.SpaceXS)
		m3.Text(c, strings.Join(facts, " · "), tt.LabelMedium.C(cs.OnSurfaceVariant))
		if focused || running || failed {
			ui.Column(c.Key("more")).FillWidth().Padding(design.SpaceSM, 0, 0, 0).
				Transition(ui.ElementTransition{Duration: design.Medium2, Ease: design.Emphasized, Enter: &ui.Motion{Opacity: 0, Y: -6}, Shadows: true}).Children(func() {
				if note := bench.S("note"); note != "" {
					m3.Text(c, note, tt.BodySmall.C(cs.OnSurfaceVariant))
				}
				ui.Row(c).FillWidth().Wrap().Gap(design.SpaceXS).Children(func() {
					for _, l := range []struct {
						ic    icons.Icon
						label string
						url   string
					}{
						{icons.ArticleOutlined, l10n.AsrModelCerSource(bench.S("source")), bench.S("source_url")},
						{icons.GavelRounded, l10n.CommonLicense(m.S("license")), m.S("license_url")},
						{icons.OpenInNewRounded, l10n.AsrModelOriginal, m.S("upstream")},
					} {
						if (m3.Button{Kind: m3.TextKind, Icon: l.ic, Label: l.label}).Show(c.Key(l.label)).Clicked() {
							widgets.OpenPlainLink(c, l.url)
						}
					}
				})
			})
		}
		if running {
			m3.VGap(c, design.SpaceSM)
			asrProgress(c, install)
		}
		if failed {
			m3.VGap(c, design.SpaceSM)
			m3.Text(c, l10n.AsrModelDownloadFailed(install.S("error")), tt.BodySmall.C(cs.Error))
		}
		m3.VGap(c, design.SpaceSM)
		ui.Row(c).FillWidth().Justify(ui.End).AlignItems(ui.Center).Children(func() {
			switch {
			case running:
				if (m3.Button{Kind: m3.Outlined, Label: l10n.CommonPause}).Show(c).Clicked() {
					p.call("POST", "/api/asr/local/cancel", nil)
				}
			case !installed:
				label := l10n.AsrModelResume
				if !failed && install.S("state") != "canceled" {
					engine := ""
					if extra > 0 {
						engine = l10n.AsrModelIncludesEngine
					}
					label = l10n.AsrModelDownload(core.FmtBytes(size+extra), engine)
				}
				if (m3.Button{Kind: m3.Filled, Icon: icons.DownloadRounded, Label: label, Disabled: busy}).Show(c).Clicked() {
					p.call("POST", "/api/asr/local/install", core.J{"model": id})
				}
			default:
				if m3.TextButton(c, l10n.CommonDelete).Clicked() {
					p.remove(m)
				}
				m3.HGap(c, design.SpaceSM)
				if active {
					if (m3.Button{Kind: m3.Outlined, Label: l10n.CommonDisable}).Show(c).Clicked() {
						p.set("asr.local.model", "")
					}
				} else if (m3.Button{Kind: m3.Tonal, Label: l10n.AsrModelUse}).Show(c).Clicked() {
					p.set("asr.local.model", id)
				}
			}
		})
	})
	if e.Clicked() {
		p.focus = id
	}
}

func asrProgress(c *ui.Context, pr core.J) {
	cs := design.S()
	tt := design.TT()
	done, total, speed := pr.F("done"), pr.F("total"), pr.F("speed")
	bar := m3.Progress{Height: 6}
	if total > 0 {
		bar.Value = m3.Of(min(max(done/total, 0), 1))
	}
	var parts []string
	if s := pr.S("step"); s != "" {
		parts = append(parts, s)
	}
	if total > 0 {
		parts = append(parts, core.FmtBytes(done)+" / "+core.FmtBytes(total))
	}
	if speed > 0 {
		parts = append(parts, core.FmtSpeed(speed))
		if total > done {
			parts = append(parts, l10n.CommonTimeLeft(core.FmtDuration(int(math.Round((total-done)/speed)))))
		}
	}
	ui.Column(c).FillWidth().Children(func() {
		m3.Linear(c, bar)
		m3.VGap(c, design.SpaceXS)
		m3.Text(c, strings.Join(parts, " · "), tt.LabelSmall.C(cs.OnSurfaceVariant))
		if n := pr.S("note"); n != "" {
			m3.Text(c, n, tt.LabelSmall.C(cs.OnSurfaceVariant))
		}
	})
}

// storage is where the engine and models are kept, with the space they
// use and what is left on that disk.
func (p *LocalAsrPage) storage(c *ui.Context, loc, install core.J) {
	cs := design.S()
	tt := design.TT()
	// Below this, a model download may not fit.
	const low = 2 << 30
	dir := loc.S("dir")
	custom := loc.IsTrue("custom")
	used := loc.F("used")
	free := -1.0
	if loc.Has("free") {
		free = loc.F("free")
	}
	running := install.S("state") == "running"
	moving := running && install.S("model") == ""
	moveFailed := install.S("state") == "failed" && install.S("model") == ""
	isLow := free >= 0 && free < low
	m3.Card(c, m3.CardOutlined).FillWidth().Padding(design.SpaceLG).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Start).Children(func() {
			ic := icons.FolderOutlined
			if custom {
				ic = icons.FolderSpecialOutlined
			}
			ui.Box(c).Size(48, 48).Shrink(0).Center().Radius(design.CornerLG).Background(cs.SecondaryContainer).Children(func() {
				m3.Icon(c, ic, 24, cs.OnSecondaryContainer)
			})
			m3.HGap(c, design.SpaceLG)
			ui.Column(c).Grow(1).Basis(0).Children(func() {
				label := l10n.AsrDefaultLocation
				if custom {
					label = l10n.AsrCustomLocation
				}
				m3.Text(c, label, tt.LabelLarge.C(cs.Primary))
				m3.VGap(c, design.SpaceXXS)
				m3.Tip(c, m3.Clamp(m3.Text(c, dir, tt.BodyLarge), 2), dir)
				m3.VGap(c, design.SpaceXS)
				parts := []string{l10n.AsrUsedSpace(core.FmtBytes(used))}
				if free >= 0 {
					parts = append(parts, l10n.AsrFreeSpace(core.FmtBytes(free)))
				}
				col := cs.OnSurfaceVariant
				if isLow {
					col = cs.Error
				}
				m3.Text(c, strings.Join(parts, " · "), tt.BodySmall.C(col))
			})
		})
		if isLow && !moving {
			m3.VGap(c, design.SpaceMD)
			m3.Text(c, l10n.AsrLowSpace, tt.BodySmall.C(cs.Error))
		}
		if moving {
			m3.VGap(c, design.SpaceLG)
			asrProgress(c, install)
		}
		if moveFailed {
			m3.VGap(c, design.SpaceMD)
			m3.Text(c, install.S("error"), tt.BodySmall.C(cs.Error))
		}
		m3.VGap(c, design.SpaceLG)
		ui.Row(c).FillWidth().Wrap().Gap(design.SpaceSM).Children(func() {
			// Android keeps models inside the app, where no other folder
			// is reachable.
			if core.CanOpenFolders() {
				if (m3.Button{Kind: m3.Tonal, Icon: icons.DriveFileMoveOutline, Label: l10n.AsrChangeLocation, Disabled: running}).Show(c).Clicked() {
					p.relocate(loc, "")
				}
				if (m3.Button{Kind: m3.TextKind, Icon: icons.OpenInNewRounded, Label: l10n.CommonOpenInFileManager, Disabled: dir == ""}).Show(c).Clicked() {
					go core.OpenPath(dir)
				}
			}
			if custom && (m3.Button{Kind: m3.TextKind, Label: l10n.AsrResetLocation, Disabled: running}).Show(c).Clicked() {
				p.relocate(loc, loc.S("default"))
			}
		})
		if running && !moving {
			m3.VGap(c, design.SpaceXS)
			m3.Text(c, l10n.AsrLocationAfterDownload, tt.BodySmall.C(cs.OnSurfaceVariant))
		}
	})
}

// blurField commits a field when it is submitted or loses focus
// (onSubmitted, onTapOutside).
func blurField(e ui.Element, had *bool, commit func()) {
	focused := e.Focused()
	if e.Submitted() || (*had && !focused) {
		commit()
	}
	*had = focused
}

// sources is the download source choice, with a short guide per source.
func (p *LocalAsrPage) sources(c *ui.Context, sources []core.J, source string) {
	cs := design.S()
	tt := design.TT()
	guides := map[string]string{
		"hf-mirror":   l10n.AsrSourceHfMirror,
		"modelscope":  l10n.AsrSourceModelscope,
		"huggingface": l10n.AsrSourceHuggingface,
		"custom":      l10n.AsrSourceCustom,
	}
	ui.Column(c).FillWidth().Padding(design.SpaceSM, design.SpaceLG, design.SpaceLG, 0).Background(cs.SurfaceContainerLow).Radius(design.CornerLG).Clip().Children(func() {
		sel := source
		for _, s := range sources {
			id := s.S("id")
			sub := s.S("note")
			if source == id && guides[id] != "" {
				sub = guides[id]
			}
			if m3.RadioTile(c.Key(id), m3.Tile{Title: s.S("name"), Subtitle: sub, NoRadius: true}, &sel, id) {
				p.set("asr.local.source", id)
			}
		}
		if source == "custom" {
			ui.Box(c).FillWidth().Padding(design.SpaceSM, 0, 0, design.SpaceLG).Children(func() {
				e := m3.Field{Label: l10n.AsrSourceTemplate, Hint: "https://example.org/{repo}/resolve/main/{file}"}.Show(c.Key("custom"), &p.custom)
				blurField(e, &p.customOn, func() { p.set("asr.local.custom_url", strings.TrimSpace(p.custom)) })
			})
		}
		ui.Column(c).FillWidth().Padding(design.SpaceLG, 0, 0, design.SpaceLG).Children(func() {
			m3.Text(c, l10n.AsrEngineSource, tt.TitleSmall)
			m3.VGap(c, design.SpaceXS)
			m3.Text(c, l10n.AsrEngineSourceBody, tt.BodySmall.C(cs.OnSurfaceVariant).H(1.5))
			m3.VGap(c, design.SpaceSM)
			e := m3.Field{Label: l10n.AsrEngineProxy, Hint: "https://…/"}.Show(c.Key("proxy"), &p.proxy)
			blurField(e, &p.proxyOn, func() { p.set("asr.local.github_proxy", strings.TrimSpace(p.proxy)) })
		})
	})
}

func (p *LocalAsrPage) threads(c *ui.Context, st, status core.J) {
	cs := design.S()
	tt := design.TT()
	cpus := 4
	if status.Has("cpus") {
		cpus = status.I("cpus")
	}
	top := min(max(cpus, 2), 8)
	value := 4
	if st.Has("threads") {
		value = st.I("threads")
	}
	if p.threadsDrag != nil {
		value = int(math.Round(*p.threadsDrag))
	}
	ui.Column(c).FillWidth().Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			m3.Text(c, l10n.AsrThreads, tt.TitleMedium).Grow(1).Basis(0)
			m3.Text(c, l10n.AsrThreadCount(value), tt.LabelLarge.C(cs.Primary))
		})
		m3.Text(c, l10n.AsrThreadsHint(cpus), tt.BodySmall.C(cs.OnSurfaceVariant))
		v := float64(min(max(value, 1), top))
		r := (m3.Slider{Min: 1, Max: float64(top), Divisions: top - 1, Label: fmt.Sprint(value)}).Show(c, &v)
		if int(math.Round(v)) != value {
			p.threadsDrag = &v
		}
		if r.Ended {
			p.set("asr.local.threads", int(math.Round(v)))
			p.threadsDrag = nil
		}
	})
}
