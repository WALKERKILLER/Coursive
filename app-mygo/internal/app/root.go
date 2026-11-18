package app

import (
	"net/url"
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

// themeKey is what a theme is built from; the scheme is rebuilt only when
// it changes.
type themeKey struct {
	prefs design.Prefs
	dark  bool
	seed  ui.Color
}

var (
	themeFor  themeKey
	themeLast *design.Theme
	gate      string
	listening bool
)

// theme is CoursiveApp's: the scheme of the preferences in the brightness
// the theme mode asks for, seeded from the desktop's accent when dynamic
// colour is on.
func theme(c *ui.Context) *design.Theme {
	dark := c.Theme().Dark
	switch core.UI.ThemeMode {
	case design.ThemeLight:
		dark = false
	case design.ThemeDark:
		dark = true
	}
	var seed ui.Color
	if accent := c.Preferences().Accent; accent.A != 0 {
		light, darkSeed := design.SystemSeeds(accent)
		seed = light
		if dark {
			seed = darkSeed
		}
	}
	k := themeKey{core.UI.Prefs, dark, seed}
	if themeLast == nil || k != themeFor {
		themeFor, themeLast = k, design.NewTheme(core.UI.Prefs, dark, seed)
	}
	return themeLast
}

// Root is the window's content: the theme, RootGate's fade-through between
// workspace setup, the daemon's failure, its start and the shell, and the
// dialogs over all of them.
func Root(c *ui.Context) {
	t := theme(c)
	design.Use(t)
	c.SetTheme(t.UI())
	if !listening {
		listening = true
		listenTaskRuns()
	}
	Tick()

	d := State.Daemon
	key := "shell"
	switch {
	case d.State() == core.DaemonSetup:
		key = "setup"
	case d.State() == core.DaemonFailed:
		key = "failed"
	case !State.Connected():
		key = "starting"
	}
	if key != gate {
		gate = key
		if key == "shell" {
			Post(linksReady)
		}
	}
	dur := t.Motion(design.Long2, nil).Duration
	ui.Box(c).Fill().Background(design.S().Surface).Children(func() {
		ui.Column(c.Key(key)).Absolute().Top(0).Left(0).Right(0).Bottom(0).
			Transition(ui.ElementTransition{
				Duration: dur, Ease: design.EmphasizedDecelerate,
				ExitDuration: dur, ExitEase: design.EmphasizedAccelerate,
				Enter: &ui.Motion{Opacity: 0, Scale: .96},
				Exit:  &ui.Motion{Opacity: 0, Scale: .96},
			}).Children(func() {
			switch key {
			case "setup":
				workspaceSetup(c)
			case "failed":
				widgets.EmptyState{
					Icon: icons.PowerOffRounded, Title: l10n.AppDaemonFailed, Body: d.Error(),
					Action: func() {
						if m3.TonalButton(c, l10n.AppRestart).Clicked() {
							d.Restart()
						}
					},
				}.Show(c)
			case "starting":
				widgets.CenteredLoader(c, l10n.CommonStarting)
			default:
				if m3.RootNav == nil {
					m3.RootNav = m3.NewNavigator(NewShell())
				}
				m3.RootNav.View(c)
			}
		})
	})
	m3.Host(c)
}

// Post runs fn after the frame being built.
func Post(fn func()) { go core.Post(fn) }

// listenTaskRuns is TaskRunListener: a snack bar for each finished agent
// task run, with a way to its conversation.
func listenTaskRuns() {
	State.Events.Listen(func(e core.J) {
		if e.S("type") != "agent_task" {
			return
		}
		d := e.O("data")
		if d.S("phase") != "finished" || d.S("status") == "skipped" {
			return
		}
		label, _, bad := runStatus(d.S("status"))
		session := d.S("session_id")
		why := ""
		if bad && d.S("summary") != "" {
			why = l10n.TasksRunReason(d.S("summary"))
		}
		action := ""
		if session != "" {
			action = l10n.CommonView
		}
		widgets.ShowSnack(l10n.TasksRunNotice(d.S("name"), label, why), action, func() { openRunConversation(session) })
	})
}

// Links from other programs (coursive-app://lecture/<id>?t=<seconds>, as an
// exported note's timestamps are) wait for the shell, then open.
var (
	pendingLinks []string
	linksOpen    bool
)

// ReceiveLink takes a link the operating system handed the app.
func ReceiveLink(raw string) {
	if !strings.HasPrefix(raw, "coursive-app:") {
		return
	}
	pendingLinks = append(pendingLinks, raw)
	flushLinks()
}

func linksReady() {
	linksOpen = true
	flushLinks()
}

func flushLinks() {
	if !linksOpen || m3.RootNav == nil {
		return
	}
	links := pendingLinks
	pendingLinks = nil
	for _, raw := range links {
		u, err := url.Parse(raw)
		if err != nil || u.Host != "lecture" {
			continue
		}
		id := strings.Trim(u.Path, "/")
		if i := strings.IndexByte(id, '/'); i >= 0 {
			id = id[:i]
		}
		if id == "" {
			continue
		}
		if t, err := strconv.Atoi(u.Query().Get("t")); err == nil {
			SeekOrOpen(id, t)
		} else {
			m3.RootNav.Push(NewPlayerPage(id, -1))
		}
	}
	if len(links) > 0 && ShowWindow != nil {
		ShowWindow()
	}
}

// ShowWindow raises the window; main sets it.
var ShowWindow func()
