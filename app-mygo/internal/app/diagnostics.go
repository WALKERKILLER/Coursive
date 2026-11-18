package app

import (
	"math"
	"regexp"
	"runtime"
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

type diagHealth int

const (
	diagOK diagHealth = iota
	diagAttention
	diagPending
)

type diagCheck struct {
	icon          icons.Icon
	title, detail string
	health        diagHealth
	action        func(c *ui.Context)
}

// Diagnostics is 设置 → 诊断: whether the background service,
// transcription and the assistant work, where things are, the logs, and a
// restart.
type Diagnostics struct {
	runtime, tools, models, asr core.J
	agentLog                    []string
	loading                     bool
	pdfBackend                  string
}

// NewDiagnostics runs the checks.
func NewDiagnostics() *Diagnostics {
	d := &Diagnostics{}
	d.load()
	return d
}

func (d *Diagnostics) load() {
	if !State.Connected() || d.loading {
		return
	}
	d.loading = true
	d.runtime, d.tools, d.models, d.asr, d.pdfBackend = nil, nil, nil, nil, ""
	paths := []string{"/api/health", "/api/agent/runtime", "/api/agent/status", "/agent/models", "/api/asr"}
	// One unavailable service must not hide the other preparation checks.
	core.Async(func() ([]core.J, error) {
		out := make([]core.J, len(paths))
		done := make(chan struct{}, len(paths))
		for i, p := range paths {
			go func() {
				defer func() { done <- struct{}{} }()
				ch := make(chan core.J, 1)
				go func() {
					v, err := api().Get(p)
					if err != nil {
						ch <- nil
						return
					}
					ch <- core.Obj(v)
				}()
				select {
				case out[i] = <-ch:
				case <-time.After(12 * time.Second):
				}
			}()
		}
		for range paths {
			<-done
		}
		return out, nil
	}, func(r []core.J, _ error) {
		d.tools, d.pdfBackend = r[0].O("media_tools"), r[0].S("pdf_backend")
		d.runtime = r[1]
		d.agentLog = nil
		for _, l := range r[2].L("logs") {
			d.agentLog = append(d.agentLog, core.Str(l))
		}
		d.models, d.asr = r[3], r[4]
		d.loading = false
		State.RefreshHealth()
	})
}

func diagPlatform() string {
	switch runtime.GOOS {
	case "linux":
		return "Linux"
	case "windows":
		return "Windows"
	case "darwin":
		return "macOS"
	case "android":
		return "Android"
	}
	return runtime.GOOS
}

func (d *Diagnostics) asrEnabled() bool {
	for _, ch := range d.asr.Os("channels") {
		if ch.IsTrue("enabled") {
			return true
		}
	}
	return false
}

func tonalAction(label string, run func()) func(c *ui.Context) {
	return func(c *ui.Context) {
		if (m3.Button{Kind: m3.Tonal, Label: label}).Show(c).Clicked() {
			run()
		}
	}
}

func (d *Diagnostics) checks() []diagCheck {
	h := State.Health
	agent := State.Agent
	node := d.runtime.O("node")
	daemon := State.Daemon
	ready := daemon.State() == core.DaemonReady
	agentState := agent.S("state")
	agentError := strings.TrimSpace(agent.S("error"))
	var out []diagCheck

	detail, health := l10n.CommonStarting, diagPending
	switch daemon.State() {
	case core.DaemonReady:
		v := h.S("version")
		if v == "" {
			v = "—"
		}
		detail, health = l10n.DiagDaemonDetail(v, daemon.Port()), diagOK
	case core.DaemonFailed:
		detail, health = daemon.Error(), diagAttention
		if detail == "" {
			detail = l10n.DiagNotStarted
		}
	}
	out = append(out, diagCheck{icons.DnsOutlined, l10n.DiagDaemon, detail, health, nil})
	out = append(out, d.mediaCheck(false), d.mediaCheck(true))

	mc := diagCheck{icon: icons.AutoAwesomeOutlined, title: l10n.SetupModelTitle, detail: l10n.SetupUnknown, health: diagPending,
		action: tonalAction(l10n.SetupConfigure, func() { m3.Nav().Push(NewModelsPage()) })}
	if d.models != nil {
		sel := d.models.O("selected")
		if d.models["selected"] == nil {
			mc.detail, mc.health = l10n.SetupModelMissing, diagAttention
		} else {
			vision := l10n.SetupVisionMissing
			if sel.IsTrue("vision") || d.models.IsTrue("vision_ready") {
				vision = l10n.SetupVisionConfigured
			}
			mc.detail, mc.health = l10n.SetupModelConfigured+"\n"+vision, diagOK
		}
	}
	out = append(out, mc)

	ac := diagCheck{icon: icons.RecordVoiceOverOutlined, title: l10n.SetupAsrTitle, detail: l10n.SetupUnknown, health: diagPending,
		action: tonalAction(l10n.SetupConfigure, func() { OpenSettingsSection("transcript") })}
	if d.asr != nil {
		ac.detail, ac.health = l10n.SetupAsrMissing, diagAttention
		if d.asrEnabled() {
			ac.detail, ac.health = l10n.SetupAsrConfigured, diagOK
		}
	}
	out = append(out, ac)

	var agentDetail string
	switch agentState {
	case "ready":
		agentDetail = l10n.CommonRunning
		if v := node["version"]; v != nil {
			agentDetail = l10n.DiagAgentRunningNode(v)
		}
	case "starting":
		agentDetail = l10n.CommonStarting
	default:
		agentDetail = agentError
		if agentDetail == "" {
			agentDetail = l10n.DiagAgentNotRunning
		}
	}
	agentHealth := diagAttention
	switch {
	case !ready || agentState == "starting":
		agentHealth = diagPending
	case agentState == "ready":
		agentHealth = diagOK
	}
	gc := diagCheck{icon: icons.SmartToyOutlined, title: l10n.CommonAssistant, detail: agentDetail, health: agentHealth}
	if ready && agentState != "ready" && agentState != "starting" {
		gc.action = tonalAction(l10n.DiagSetupRuntime, ShowRuntimeSetup)
	}
	return append(out, gc)
}

func (d *Diagnostics) mediaCheck(pdf bool) diagCheck {
	if pdf && d.pdfBackend == "unsupported" {
		return diagCheck{icons.PictureAsPdfOutlined, l10n.SetupPdfTitle, l10n.SetupPdfUnsupported, diagAttention, nil}
	}
	nativePdf := pdf && d.pdfBackend == "pdfkit"
	names := []string{"ffmpeg", "ffprobe"}
	switch {
	case nativePdf:
		names = []string{"study_pdf"}
	case pdf:
		names = []string{"pdfinfo", "pdftoppm", "pdftotext"}
	}
	known := !pdf || d.pdfBackend != ""
	var missing []string
	for _, n := range names {
		if _, ok := d.tools[n].(bool); !ok {
			known = false
		}
		if d.tools[n] != true {
			missing = append(missing, n)
		}
	}
	c := diagCheck{icon: icons.GraphicEqRounded, title: l10n.SetupMediaTitle}
	if pdf {
		c.icon, c.title = icons.PictureAsPdfOutlined, l10n.SetupPdfTitle
	}
	switch {
	case !known:
		c.detail, c.health = l10n.SetupUnknown, diagPending
	case len(missing) == 0:
		c.detail, c.health = l10n.SetupToolsPresent, diagOK
	case nativePdf:
		c.detail, c.health = l10n.SetupPdfHelperMissing, diagAttention
	default:
		c.detail, c.health = l10n.SetupToolsMissing(strings.Join(missing, ", ")), diagAttention
	}
	if !known || len(missing) > 0 {
		c.action = func(ctx *ui.Context) {
			if m3.TextButton(ctx, l10n.SetupInstallHelp).Clicked() {
				ShowMediaToolsHelp(pdf)
			}
		}
	}
	return c
}

func (d *Diagnostics) report(checks []diagCheck) string {
	lines := []string{"Coursive " + State.Health.S("version") + " · " + diagPlatform()}
	for _, c := range checks {
		lines = append(lines, l10n.DiagReportLine(c.title, c.detail))
	}
	if lib := State.Health.S("library"); lib != "" {
		lines = append(lines, l10n.DiagReportLibrary(lib))
	}
	return strings.Join(lines, "\n")
}

// Show builds the pane.
func (d *Diagnostics) Show(c *ui.Context) {
	checks := d.checks()
	library := State.Health.S("library")
	ui.Column(c).FillWidth().Children(func() {
		m3.VGap(c, design.SpaceXL)
		m3.Text(c, l10n.SetupIntro, design.TT().BodyMedium)
		ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
			if (m3.Button{Kind: m3.TextKind, Icon: icons.RefreshRounded, Label: l10n.SetupRecheck, Disabled: d.loading}).Show(c).Clicked() {
				d.load()
			}
		})
		diagSummary(c, checks, d.loading, "Coursive "+State.Health.S("version")+" · "+diagPlatform(), func() {
			c.WriteClipboard(d.report(checks))
			widgets.ShowSnack(l10n.DiagCopied, "", nil)
		})
		diagHeading(c, l10n.DiagStatus)
		diagSegmented(c, "checks", len(checks), func(i int) { diagCheckTile(c, checks[i]) })
		if library != "" {
			diagHeading(c, l10n.CommonLocation)
			diagSegmented(c, "paths", 1, func(int) { diagPathTile(c, l10n.CommonLibrary, library) })
		}
		diagHeading(c, l10n.DiagLogs)
		diagSegmented(c, "logs", 2, func(i int) {
			if i == 0 {
				diagLogTile(c, icons.ReceiptLongOutlined, l10n.DiagDaemonLog, State.Daemon.Log(), nil)
			} else {
				diagLogTile(c, icons.SmartToyOutlined, l10n.DiagAgentLog, d.agentLog, d.load)
			}
		})
		diagHeading(c, l10n.DiagMaintenance)
		diagSegmented(c, "maint", 1, func(int) {
			diagTile(c, icons.RestartAltRounded, ui.Color{}, l10n.DiagRestart, l10n.DiagRestartHint, func() {
				if (m3.Button{Kind: m3.Outlined, Label: l10n.DiagRestartAction}).Show(c).Clicked() {
					go State.Daemon.Restart()
				}
			}, nil, nil)
		})
	})
}

// diagSummary is the overall verdict, in words rather than a dashboard.
func diagSummary(c *ui.Context, checks []diagCheck, loading bool, subtitle string, onCopy func()) {
	cs := design.S()
	tt := design.TT()
	issues, pending := 0, false
	for _, ch := range checks {
		issues += map[bool]int{true: 1}[ch.health == diagAttention]
		pending = pending || ch.health == diagPending
	}
	title, ic, bg, fg := l10n.DiagAllGood, icons.CheckRounded, cs.PrimaryContainer, cs.OnPrimaryContainer
	switch {
	case issues > 0:
		title, ic, bg, fg = l10n.DiagIssues(issues), icons.PriorityHighRounded, cs.ErrorContainer, cs.OnErrorContainer
	case pending:
		title, ic, bg, fg = l10n.SetupUnknown, icons.MoreHorizRounded, cs.SurfaceContainerHighest, cs.OnSurfaceVariant
		if loading {
			title = l10n.CommonChecking
		}
	}
	ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(0, design.SpaceXS).Children(func() {
		ui.Box(c).Size(56, 56).Shrink(0).Center().Radius(design.CornerLGIncreased).Background(bg).
			Transition(ui.ElementTransition{Duration: design.Medium2, Ease: design.Emphasized, Colors: true}).Children(func() {
			m3.Icon(c, ic, 28, fg)
		})
		m3.HGap(c, design.SpaceLG)
		ui.Column(c).Grow(1).Basis(0).Children(func() {
			m3.Text(c, title, tt.HeadlineSmall)
			m3.VGap(c, design.SpaceXXS)
			m3.Text(c, subtitle, tt.BodyMedium.C(cs.OnSurfaceVariant))
		})
		m3.HGap(c, design.SpaceSM)
		if (m3.IconButton{Kind: m3.TonalIcon, Icon: icons.ContentCopyRounded, Tooltip: l10n.DiagCopy, Size: 20}).Show(c).Clicked() {
			onCopy()
		}
	})
}

func diagHeading(c *ui.Context, text string) {
	m3.Text(c, text, design.TT().LabelLarge.C(design.S().OnSurfaceVariant)).Margin(design.SpaceXL, design.SpaceLG, design.SpaceSM, design.SpaceLG)
}

// diagSegmented is the M3 Expressive segmented list: one tile per item,
// 2dp apart, with the group's large corners only at its ends.
func diagSegmented(c *ui.Context, key string, n int, item func(i int)) {
	cs := design.S()
	outer, inner := float32(design.CornerLGIncreased), float32(design.CornerXS)
	ui.Column(c.Key(key)).FillWidth().Gap(design.SpaceXXS).Children(func() {
		for i := 0; i < n; i++ {
			top, bottom := inner, inner
			if i == 0 {
				top = outer
			}
			if i == n-1 {
				bottom = outer
			}
			ui.Column(c.Key(i)).FillWidth().Background(cs.SurfaceContainer).Radius(top, top, bottom, bottom).Clip().Children(func() { item(i) })
		}
	})
}

func diagTile(c *ui.Context, leading icons.Icon, leadingColor ui.Color, title, subtitle string, trailing, below, onTap func()) {
	cs := design.S()
	tt := design.TT()
	if leadingColor.A == 0 {
		leadingColor = cs.OnSurfaceVariant
	}
	var e ui.Element
	if onTap != nil {
		e = ui.ButtonBase(c).FocusRing(false).Cursor(ui.CursorPointer).Column()
		m3.InkWell(e, 0)
	} else {
		e = ui.Column(c)
	}
	e = e.FillWidth().Padding(design.SpaceMD, design.SpaceLG)
	e.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(40, 40).Shrink(0).Center().Radius(design.CornerMD).Background(cs.SurfaceContainerHighest).Children(func() {
				m3.Icon(c, leading, 20, leadingColor)
			})
			m3.HGap(c, design.SpaceLG)
			ui.Column(c).Grow(1).Basis(0).Children(func() {
				m3.Text(c, title, tt.TitleSmall)
				if subtitle != "" {
					m3.VGap(c, design.SpaceXXS)
					m3.Text(c, subtitle, tt.BodySmall.C(cs.OnSurfaceVariant))
				}
			})
			if trailing != nil {
				m3.HGap(c, design.SpaceMD)
				trailing()
			}
		})
		if below != nil {
			ui.Row(c).FillWidth().Padding(design.SpaceSM, 0, 0, 40+design.SpaceLG).Children(below)
		}
	})
	if onTap != nil && e.Clicked() {
		onTap()
	}
}

func diagCheckTile(c *ui.Context, ch diagCheck) {
	cs := design.S()
	col := ui.Color{}
	if ch.health == diagAttention {
		col = cs.Error
	}
	var below func()
	if ch.action != nil {
		below = func() { ch.action(c) }
	}
	diagTile(c, ch.icon, col, ch.title, ch.detail, func() {
		label, dot := l10n.DiagChecking, cs.Outline
		text := cs.OnSurfaceVariant
		switch ch.health {
		case diagOK:
			label, dot = l10n.DiagOk, cs.Primary
		case diagAttention:
			label, dot, text = l10n.CommonNeedsAttention, cs.Error, cs.Error
		}
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(8, 8).Radius(4).Background(dot)
			m3.HGap(c, design.SpaceSM)
			m3.Text(c, label, design.TT().LabelMedium.C(text))
		})
	}, below, nil)
}

func diagPathTile(c *ui.Context, label, path string) {
	var tap func()
	if core.CanOpenFolders() {
		tap = func() { go core.OpenPath(path) }
	}
	diagTile(c, icons.FolderOutlined, ui.Color{}, label, path, func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			if (m3.IconButton{Icon: icons.ContentCopyOutlined, Tooltip: l10n.CommonCopyPath, Size: 20}).Show(c).Clicked() {
				c.WriteClipboard(path)
				widgets.ShowSnack(l10n.CommonPathCopied, "", nil)
			}
			if core.CanOpenFolders() && (m3.IconButton{Icon: icons.OpenInNewRounded, Tooltip: l10n.CommonOpenFolder, Size: 20}).Show(c).Clicked() {
				go core.OpenPath(path)
			}
		})
	}, nil, tap)
}

var logError = regexp.MustCompile(`(?i)\b(error|failed|panic|fatal)\b|错误|失败`)

func logStyle(line string) design.TextStyle {
	cs := design.S()
	col := cs.OnSurfaceVariant
	if logError.MatchString(line) {
		col = cs.Error
	}
	return design.TT().BodySmall.F("monospace").S(12).H(1.5).C(col)
}

// diagLogTile is a log's last lines as a preview; it opens the whole log.
func diagLogTile(c *ui.Context, ic icons.Icon, title string, lines []string, onOpen func()) {
	cs := design.S()
	tail := lines
	if len(tail) > 3 {
		tail = tail[len(tail)-3:]
	}
	sub := l10n.DiagLogEmpty
	if len(lines) > 0 {
		sub = l10n.DiagLogLines(len(lines))
	}
	var below func()
	if len(tail) > 0 {
		below = func() {
			ui.Column(c).Grow(1).Basis(0).Padding(design.SpaceSM, design.SpaceMD).Radius(design.CornerSM).Background(cs.SurfaceContainerLowest).Children(func() {
				for i, l := range tail {
					m3.Clamp(m3.Text(c.Key(i), l, logStyle(l)), 1)
				}
			})
		}
	}
	diagTile(c, ic, ui.Color{}, title, sub, func() { m3.Icon(c, icons.ChevronRightRounded, 24, cs.OnSurfaceVariant) }, below, func() {
		if onOpen != nil {
			onOpen()
		}
		showLogViewer(title, lines)
	})
}

// showLogViewer is the whole log, newest at the bottom: full screen on
// phones, a large dialog elsewhere.
func showLogViewer(title string, lines []string) {
	compact := Compact()
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		cs := design.S()
		tt := design.TT()
		body := func() {
			ui.Column(c).Fill().Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(design.SpaceLG, design.SpaceSM, design.SpaceSM, design.SpaceXL).Children(func() {
					ui.Column(c).Grow(1).Basis(0).Children(func() {
						m3.Text(c, title, tt.TitleLarge)
						sub := l10n.DiagLogEmpty
						if len(lines) > 0 {
							sub = l10n.DiagLogRecent(len(lines))
						}
						m3.Text(c, sub, tt.BodySmall.C(cs.OnSurfaceVariant))
					})
					if (m3.IconButton{Icon: icons.ContentCopyRounded, Tooltip: l10n.CommonCopyAll, Disabled: len(lines) == 0}).Show(c).Clicked() {
						c.WriteClipboard(strings.Join(lines, "\n"))
						widgets.ShowSnack(l10n.DiagLogCopied, "", nil)
					}
					if m3.IconBtn(c, icons.CloseRounded, l10n.CommonClose).Clicked() {
						r.Close()
					}
				})
				box := ui.Box(c).FillWidth().Grow(1).Basis(0).Margin(0, design.SpaceMD, design.SpaceMD, design.SpaceMD).
					Background(cs.SurfaceContainerLowest).Radius(design.CornerLG).Clip()
				// Opens at the newest line, as reverse: true.
				st := ui.Local(box, "log.scroll", func() ui.ScrollState { return ui.ScrollState{Y: math.MaxFloat32} })
				box.Children(func() {
					if len(lines) == 0 {
						ui.Box(c).Fill().Center().Children(func() {
							m3.Text(c, l10n.DiagLogNothing, tt.BodyMedium.C(cs.OnSurfaceVariant))
						})
						return
					}
					ui.Scroll(c).Fill().TrackScroll(st).Children(func() {
						ui.Column(c).FillWidth().Padding(design.SpaceLG).Children(func() {
							for i, l := range lines {
								m3.Text(c.Key(i), l, logStyle(l)).Selectable()
							}
						})
					})
				})
			})
		}
		if compact {
			ui.Box(c).Fill().Background(cs.Surface).Children(body)
			return
		}
		ui.Box(c).Width(760).MaxHeight(600).Height(600).Background(cs.SurfaceContainerHigh).Radius(design.CornerXL).Clip().Children(body)
	}, nil)
}
