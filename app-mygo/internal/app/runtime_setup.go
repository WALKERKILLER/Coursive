package app

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
)

// runtime_setup.dart: what the assistant needs to run (GET
// /api/agent/runtime), see docs/specs/0008-assistant-runtime-setup.md.

// RuntimeStatus is GET /api/agent/runtime.
type RuntimeStatus struct{ core.J }

func (r RuntimeStatus) node() core.J        { return r.O("node") }
func (r RuntimeStatus) install() core.J     { return r.O("install") }
func (r RuntimeStatus) NodeOK() bool        { return r.node().IsTrue("ok") }
func (r RuntimeStatus) NodeVersion() string { return r.node().S("version") }
func (r RuntimeStatus) NodeSource() string  { return r.node().Or("source", "none") }
func (r RuntimeStatus) DepsOK() bool        { return r.O("deps").IsTrue("ok") }
func (r RuntimeStatus) State() string       { return r.install().Or("state", "idle") }
func (r RuntimeStatus) Step() string        { return r.install().S("step") }
func (r RuntimeStatus) Progress() float64   { return min(max(r.install().F("progress"), 0), 1) }
func (r RuntimeStatus) Error() string       { return r.install().S("error") }
func (r RuntimeStatus) Log() []string       { return r.install().Ss("log") }
func (r RuntimeStatus) Ready() bool         { return r.NodeOK() && r.DepsOK() }
func (r RuntimeStatus) Running() bool       { return r.State() == "running" }

// RuntimeSetupCard offers the one-click install of Node.js and the
// agent's dependencies and follows its progress through runtime events.
type RuntimeSetupCard struct {
	st      *RuntimeStatus
	err     error
	mirror  string
	cancel  func()
	logOpen bool
	logs    ui.ScrollState
}

// NewRuntimeSetupCard loads the status and listens for its changes.
func NewRuntimeSetupCard() *RuntimeSetupCard {
	r := &RuntimeSetupCard{mirror: "npmmirror"}
	r.cancel = State.Events.Listen(func(e core.J) {
		switch e.S("type") {
		case "runtime":
			if d := e.O("data"); d != nil && r.st != nil {
				j := r.st.J.Copy()
				j["install"] = map[string]any(d)
				r.st = &RuntimeStatus{j}
				if s := d.S("state"); s == "done" || s == "failed" {
					r.load()
				}
			}
		case "agent":
			r.load()
		}
	})
	r.load()
	return r
}

// Dispose stops listening.
func (r *RuntimeSetupCard) Dispose() { r.cancel() }

func (r *RuntimeSetupCard) load() {
	core.Async(func() (core.J, error) { return api().GetJ("/api/agent/runtime") }, func(j core.J, err error) {
		if err != nil {
			r.err = err
			return
		}
		r.st, r.err = &RuntimeStatus{j}, nil
	})
}

func (r *RuntimeSetupCard) install() {
	core.Async(func() (any, error) { return api().Post("/api/agent/runtime/install", core.J{"mirror": r.mirror}) }, func(v any, err error) {
		if err != nil {
			r.err = err
			return
		}
		r.st = &RuntimeStatus{core.Obj(v)}
	})
}

type setupStep int

const (
	stepTodo setupStep = iota
	stepActive
	stepDone
)

func stepRow(c *ui.Context, label, detail string, state setupStep) {
	cs := design.S()
	tt := design.TT()
	ui.Row(c).FillWidth().AlignItems(ui.Center).PaddingY(design.SpaceXS).Children(func() {
		ui.Box(c).Size(24, 24).Center().Shrink(0).Children(func() {
			switch state {
			case stepDone:
				m3.Icon(c, icons.CheckCircleRounded, 24, cs.Primary)
			case stepActive:
				m3.Spinner(c, 18, 2.5, cs.Primary)
			default:
				m3.Icon(c, icons.RadioButtonUncheckedRounded, 24, cs.Outline)
			}
		})
		m3.HGap(c, design.SpaceMD)
		m3.Text(c, label, tt.BodyLarge).Grow(1).Basis(0)
		m3.Clamp(m3.Text(c, detail, tt.BodySmall.C(cs.OnSurfaceVariant)), 1).TextAlign(ui.End).Shrink(1)
	})
}

// Show builds the card, at most 480 wide.
func (r *RuntimeSetupCard) Show(c *ui.Context) {
	cs := design.S()
	tt := design.TT()
	st := r.st
	ui.Column(c).FillWidth().MaxWidth(480).Background(cs.SurfaceContainerHigh).Radius(design.CornerMD).Padding(design.SpaceXL).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(48, 48).Center().Shrink(0).Background(cs.TertiaryContainer).Radius(design.CornerLG).Children(func() {
				m3.Icon(c, icons.DownloadForOfflineOutlined, 24, cs.OnTertiaryContainer)
			})
			m3.HGap(c, design.SpaceLG)
			title := l10n.RuntimeInstallTitle
			if st != nil && st.Ready() {
				title = l10n.RuntimeReady
			}
			m3.Text(c, title, tt.TitleLarge).Grow(1).Basis(0)
		})
		m3.VGap(c, design.SpaceMD)
		m3.Text(c, l10n.RuntimeIntro, tt.BodyMedium.C(cs.OnSurfaceVariant))
		m3.VGap(c, design.SpaceLG)
		switch {
		case st == nil && r.err == nil:
			m3.Linear(c, m3.Progress{})
			return
		case st == nil:
			m3.Text(c, l10n.RuntimeReadFailed(r.err.Error()), tt.BodyMedium.C(cs.Error))
			return
		}
		nodeDetail := l10n.RuntimeNodeMissing
		if st.NodeOK() {
			src := l10n.RuntimeSourceSystem
			if st.NodeSource() == "managed" {
				src = l10n.RuntimeSourceManaged
			}
			nodeDetail = st.NodeVersion() + " · " + src
		}
		nodeState := stepTodo
		if st.NodeOK() {
			nodeState = stepDone
		} else if st.Running() && strings.Contains(st.Step(), "Node") {
			nodeState = stepActive
		}
		stepRow(c, "Node.js", nodeDetail, nodeState)
		depsDetail, depsState := l10n.CommonNotInstalled, stepTodo
		if st.DepsOK() {
			depsDetail, depsState = l10n.CommonInstalled, stepDone
		} else if st.Running() && st.Step() == l10n.RuntimeInstallDeps {
			depsState = stepActive
		}
		stepRow(c, l10n.RuntimeDeps, depsDetail, depsState)
		startDetail, startState := l10n.RuntimeStartsAfterInstall, stepTodo
		if st.State() == "done" {
			startDetail, startState = l10n.RuntimeStarting, stepDone
		}
		stepRow(c, l10n.RuntimeStartAssistant, startDetail, startState)
		m3.VGap(c, design.SpaceMD)
		ui.Column(c).FillWidth().Clip().Transition(ui.ElementTransition{Duration: design.Medium2, Ease: design.Emphasized, Height: true}).Children(func() {
			switch {
			case st.Running():
				m3.Text(c, st.Step(), tt.LabelLarge)
				m3.VGap(c, design.SpaceSM)
				var v *float64
				if pr := st.Progress(); pr > 0 && pr < 1 {
					v = m3.Of(pr)
				}
				m3.Linear(c, m3.Progress{Value: v, Radius: design.CornerXS})
			case st.State() == "failed":
				ui.Box(c).FillWidth().Padding(design.SpaceMD).Background(cs.ErrorContainer).Radius(design.CornerMD).Children(func() {
					m3.Text(c, st.Error(), tt.BodyMedium.C(cs.OnErrorContainer))
				})
			}
		})
		if !st.Running() && !st.Ready() {
			m3.VGap(c, design.SpaceLG)
			m3.Text(c, l10n.CommonMirror, tt.LabelLarge.C(cs.OnSurfaceVariant))
			m3.VGap(c, design.SpaceSM)
			m3.Segmented(c, []m3.Segment[string]{
				{Value: "npmmirror", Label: l10n.RuntimeMirrorChina, Icon: icons.BoltRounded},
				{Value: "official", Label: l10n.RuntimeMirrorOfficial, Icon: icons.PublicRounded},
			}, &r.mirror, m3.SegOpts{})
		}
		if log := st.Log(); len(log) > 0 {
			m3.Expansion{Tile: m3.Tile{Title: l10n.RuntimeInstallLog, TitleStyle: &tt.LabelLarge, Pad: true}, Open: &r.logOpen, Children: func() {
				ui.Scroll(c).FillWidth().MaxHeight(200).TrackScroll(&r.logs).Background(cs.SurfaceContainerLowest).Radius(design.CornerSM).
					Padding(design.SpaceMD).Children(func() {
					m3.Text(c, strings.Join(log, "\n"), tt.BodySmall.F("monospace")).Selectable()
				})
			}}.Show(c, "runtime-log")
		}
		m3.VGap(c, design.SpaceLG)
		ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
			if st.Ready() {
				if (m3.Button{Kind: m3.Outlined, Icon: icons.RefreshRounded, Label: l10n.RuntimeRecheck}).Show(c).Clicked() {
					r.load()
				}
				return
			}
			label, ic := l10n.RuntimeInstall, icons.DownloadRounded
			switch {
			case st.Running():
				label = l10n.RuntimeInstalling
			case st.State() == "failed":
				label, ic = l10n.CommonRetry, icons.RefreshRounded
			}
			if (m3.Button{Kind: m3.Filled, Icon: ic, Label: label, Disabled: st.Running()}).Show(c).Clicked() {
				r.install()
			}
		})
	})
}

// ShowRuntimeSetup opens the setup card in a dialog (设置 → 助手 → 运行环境).
func ShowRuntimeSetup() {
	card := NewRuntimeSetupCard()
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		ui.Box(c).MaxWidth(480).Children(func() {
			card.Show(c)
			ui.Box(c).Absolute().Top(design.SpaceSM).Right(design.SpaceSM).Children(func() {
				if (m3.IconButton{Icon: icons.CloseRounded, Tooltip: l10n.CommonClose}).Show(c).Clicked() {
					r.Close()
				}
			})
		})
	}, func(any) { card.Dispose() })
}
