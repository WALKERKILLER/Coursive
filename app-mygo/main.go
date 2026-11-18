// Command coursive-mygo is the MyGo native-UI client prototype for Coursive.
//
// It starts the existing Go daemon, speaks the daemon's HTTP API with the
// token it started it with, and draws the course list with MyGo's own UI and
// the Mayu Material 3 components. Nothing about the library, the data formats
// or the credentials changes: the daemon still owns all three.
//
// The window is meant to look like the Flutter client, not merely similar: the
// palette comes from Flutter's own ColorScheme (design/palette.json) and the
// spacing, shapes and type come from tokens.go, which a test holds against
// app/lib/theme/tokens.dart.
//
// See docs/specs/0033-mygo-client-migration.md and docs/mygo-migration-todo.md.
package main

import (
	"context"
	"log"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/harukizmoe/mayu"
	"github.com/harukizmoe/mayu/theme"
)

// sections are the rail's destinations. Only the library is built at this
// stage; the others say so instead of pretending.
var sections = []string{"课程库", "任务", "设置", "关于"}

type app struct {
	d   *daemon
	win *mygo.Window

	// daemonState mirrors the daemon's, pulled on the main thread.
	state     daemonState
	port      int
	library   string
	daemonErr error

	courses  []Course
	loading  bool
	fetchErr error

	section int
	dark    bool
	query   string
	// term is the term chip the page filters by; "" means the newest one.
	term string
}

func main() {
	a := &app{d: newDaemon()}
	mygo.App.WhenReady(func() {
		a.win = mygo.NewWindow(mygo.WindowOptions{
			Title:   "Coursive",
			Width:   1180,
			Height:  760,
			Content: ui.View(a.view),
		})
		a.d.onChange = func() {
			a.win.Update(a.pull)
		}
		a.d.start()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
	a.d.stop()
}

// pull copies the daemon's state into the view's state, on the main thread,
// and loads the course list the first time the daemon is ready.
func (a *app) pull() {
	state, port, library, err := a.d.snapshot()
	becameReady := state == stateReady && a.state != stateReady
	a.state, a.port, a.library, a.daemonErr = state, port, library, err
	if !becameReady {
		return
	}
	a.loadCourses()
}

func (a *app) loadCourses() {
	if a.loading {
		return
	}
	a.loading, a.fetchErr = true, nil
	c := newClient(a.port, a.d.token)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		cs, err := c.courses(ctx)
		a.win.Update(func() {
			a.loading = false
			if err != nil {
				a.fetchErr = err
				return
			}
			a.courses = cs
		})
	}()
}

func (a *app) view(c *ui.Context) {
	t := useTheme(c, a.dark, false)
	ui.Row(c).Fill().Children(func() {
		a.rail(c, t)
		ui.Column(c).Grow(1).FillHeight().Children(func() {
			switch a.section {
			case 0:
				a.libraryView(c, t)
			default:
				a.placeholder(c, t)
			}
		})
	})
}

func (a *app) rail(c *ui.Context, t *theme.Theme) {
	selected := a.section
	opts := mayu.NavigationRailOptions{
		Selected: &selected,
		OnSelect: func(i int) { a.section = i },
		Header: func() {
			textStyle(ui.Text(c, "Coursive"), TypeTitleMedium)
		},
		Footer: func() {
			if ui.Button(c, "主题").Clicked() {
				a.dark = !a.dark
			}
		},
	}
	for _, s := range sections {
		opts.Destinations = append(opts.Destinations, mayu.NavDestination{Label: s})
	}
	mayu.NavigationRail(c, opts)
}

// placeholder is a section the migration has not reached yet.
func (a *app) placeholder(c *ui.Context, t *theme.Theme) {
	width, _ := c.Size()
	ui.Column(c).Fill().Padding(SpaceLG, pagePadding(width)).Gap(SpaceSM).Children(func() {
		textStyle(ui.Text(c, sections[a.section]), TypeHeadlineMedium)
		textStyle(ui.Text(c, "阶段 0 只验证课程库这一条路径；这一页在阶段 1 才迁。"), TypeBodyMedium).
			TextColor(t.Scheme.OnSurfaceVariant)
	})
}
