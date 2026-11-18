// Command coursive-mygo is the MyGo native-UI client prototype for Coursive.
//
// It starts the existing Go daemon, speaks the daemon's HTTP API with the
// token it started it with, and draws the course list with MyGo's own UI and
// the Mayu Material 3 components. Nothing about the library, the data formats
// or the credentials changes: the daemon still owns all three.
//
// See docs/specs/0033-mygo-client-migration.md and docs/mygo-migration-todo.md.
package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/harukizmoe/mayu"
	"github.com/harukizmoe/mayu/components"
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
			ui.Text(c, "Coursive").FontSize(16).Bold()
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

func (a *app) libraryView(c *ui.Context, t *theme.Theme) {
	ui.Column(c).Fill().Padding(24, 16).Gap(12).Children(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "课程库").FontSize(28).Bold()
			ui.Spacer(c)
			if ui.Button(c, "刷新").Clicked() {
				a.loadCourses()
			}
		})

		switch a.state {
		case stateReady:
			a.readyBody(c, t)
		case stateFailed:
			a.failure(c, t)
		default:
			ui.Text(c, "正在启动本地服务…").FontSize(14).TextColor(t.Scheme.OnSurfaceVariant)
		}
	})
}

func (a *app) readyBody(c *ui.Context, t *theme.Theme) {
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Textf(c, "本地服务 %s · 端口 %d", a.state, a.port).FontSize(12).TextColor(t.Scheme.OnSurfaceVariant)
		ui.Spacer(c)
		ui.Textf(c, "共 %d 门课程", len(a.courses)).FontSize(12).TextColor(t.Scheme.OnSurfaceVariant)
	})
	components.NewTextField(c, &a.query, "搜索课程", fieldOutlined, t).Placeholder("按标题、教师或学期筛选")

	if a.loading {
		ui.Text(c, "正在读取课程…").FontSize(14).TextColor(t.Scheme.OnSurfaceVariant)
		return
	}
	if a.fetchErr != nil {
		ui.Textf(c, "读取课程失败：%v", a.fetchErr).FontSize(14).TextColor(t.Scheme.Error)
		return
	}

	shown := a.filtered()
	if len(shown) == 0 {
		ui.Text(c, "还没有课程。").FontSize(14).TextColor(t.Scheme.OnSurfaceVariant)
		return
	}
	// The scroll paints the page colour, so the tonal cards read as cards
	// instead of merging into one block.
	ui.Scroll(c).Grow(1).Background(t.Scheme.Background).Children(func() {
		ui.Column(c).Gap(8).Children(func() {
			for _, course := range shown {
				a.courseCard(c, t, course)
			}
		})
	})
}

func (a *app) filtered() []Course {
	q := strings.TrimSpace(strings.ToLower(a.query))
	if q == "" {
		return a.courses
	}
	out := make([]Course, 0, len(a.courses))
	for _, c := range a.courses {
		if strings.Contains(strings.ToLower(c.Title), q) ||
			strings.Contains(strings.ToLower(c.Teacher), q) ||
			strings.Contains(strings.ToLower(c.Term), q) {
			out = append(out, c)
		}
	}
	return out
}

func (a *app) courseCard(c *ui.Context, t *theme.Theme, course Course) {
	components.NewCard(c, cardFilled, t).Children(func() {
		ui.Column(c).Gap(6).Children(func() {
			ui.Text(c, course.Title).FontSize(16).Bold()
			meta := []string{}
			if course.Teacher != "" {
				meta = append(meta, course.Teacher)
			}
			if course.Term != "" {
				meta = append(meta, course.Term)
			}
			if course.College != "" {
				meta = append(meta, course.College)
			}
			if len(meta) > 0 {
				ui.Text(c, strings.Join(meta, " · ")).FontSize(12).TextColor(t.Scheme.OnSurfaceVariant)
			}
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				if course.LectureTotal > 0 {
					mayu.Chip(c, fmt.Sprintf("%d 讲", course.LectureTotal))
				}
				if course.Progress > 0 {
					mayu.Chip(c, fmt.Sprintf("进度 %.0f%%", course.Progress*100))
				}
				if course.LastLearnAt != "" {
					ui.Text(c, "上次 "+course.LastLearnAt).FontSize(11).TextColor(t.Scheme.OnSurfaceVariant)
				}
			})
		})
	})
}

func (a *app) failure(c *ui.Context, t *theme.Theme) {
	ui.Column(c).Gap(8).Children(func() {
		ui.Text(c, "本地服务没有启动。").FontSize(14).TextColor(t.Scheme.Error)
		ui.Textf(c, "%v", a.daemonErr).FontSize(12).TextColor(t.Scheme.OnSurfaceVariant)
		if ui.Button(c, "重试").Clicked() {
			a.d.start()
		}
	})
}

func (a *app) placeholder(c *ui.Context, t *theme.Theme) {
	ui.Column(c).Fill().Padding(24, 16).Gap(8).Children(func() {
		ui.Text(c, sections[a.section]).FontSize(28).Bold()
		ui.Text(c, "阶段 0 只验证课程库这一条路径；这一页在阶段 1 才迁。").
			FontSize(14).TextColor(t.Scheme.OnSurfaceVariant)
	})
}
