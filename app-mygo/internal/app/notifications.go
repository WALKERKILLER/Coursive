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

// NotificationIcon is the icon of a notification kind (daemon/internal/notify).
func NotificationIcon(kind string) icons.Icon {
	switch kind {
	case "lecture_ready":
		return icons.SmartDisplayOutlined
	case "lecture_added":
		return icons.EventAvailableOutlined
	case "assignment_new":
		return icons.AssignmentOutlined
	case "assignment_due":
		return icons.AlarmRounded
	case "announcement":
		return icons.CampaignOutlined
	}
	return icons.NotificationsNoneRounded
}

// OpenNotification opens what a notification points at: the replay, the
// course or a web page.
func OpenNotification(c *ui.Context, n core.J) {
	lecture, course, url := n.S("lecture_id"), n.S("course_id"), n.S("url")
	switch {
	case n.S("kind") == "lecture_ready" && lecture != "":
		m3.RootNav.Push(NewPlayerPage(lecture, -1))
	case url != "":
		widgets.OpenPlainLink(c, url)
	case course != "":
		m3.RootNav.Push(NewCoursePage(course))
	}
}

// NotificationBell is the bell with the unread count; it opens the center.
func NotificationBell(c *ui.Context) {
	unread := State.UnreadNotifications
	tip := l10n.CommonNotifications
	ic := icons.NotificationsNoneRounded
	if unread > 0 {
		tip = l10n.NotificationsTooltip(unread)
		ic = icons.NotificationsRounded
	}
	b := m3.IconButton{Tooltip: tip, Child: func(fg ui.Color) {
		label := ""
		if unread > 0 {
			label = itoa(unread)
		}
		m3.Badged(c, label, unread > 0, func() { m3.Icon(c, ic, 24, fg) })
	}}.Show(c)
	if b.Clicked() {
		ShowNotificationCenter(c)
	}
}

// ShowNotificationCenter is an M3 modal side sheet on wide windows and a
// bottom sheet on compact ones.
func ShowNotificationCenter(c *ui.Context) {
	list := &notificationList{}
	list.load()
	cancel := State.Events.Listen(func(e core.J) {
		if e.S("type") == "notification" {
			list.load()
		}
	})
	onClose := func(any) { cancel() }
	w, h := c.Size()
	if design.SizeClassOf(w) == design.SizeCompact {
		m3.ShowModalBottomSheet(m3.SheetOptions{ScrollControlled: true, OnClose: onClose}, func(c *ui.Context, r *m3.Route) {
			ui.Column(c).FillWidth().Height(h*.85 - 48).Children(func() { list.view(c, nil) })
		})
		return
	}
	m3.ShowGeneral(true, func(c *ui.Context, r *m3.Route) {
		ui.Row(c).Fill().Justify(ui.End).PassThrough().Children(func() {
			panel := ui.Column(c).Width(400).FillHeight().Margin(design.SpaceLG).Background(design.S().SurfaceContainerLow).
				Radius(design.CornerXL).Clip().
				Transition(ui.ElementTransition{Duration: design.T().Motion(design.Medium4, nil).Duration, Ease: design.EmphasizedDecelerate,
					ExitEase: design.EmphasizedAccelerate, Enter: &ui.Motion{XPercent: 25, Opacity: 0}, Exit: &ui.Motion{XPercent: 25, Opacity: 0}})
			m3.Elevation(panel, 1)
			panel.Children(func() { list.view(c, r.Close) })
		})
	}, onClose)
}

type notificationList struct {
	items  []core.J
	loaded bool
	err    error
	scroll ui.ScrollState
}

func (l *notificationList) load() {
	core.Async(func() (core.J, error) {
		r, err := api().Get("/api/notifications")
		return core.Obj(r), err
	}, func(r core.J, err error) {
		if err != nil {
			l.err = err
			return
		}
		l.items, l.loaded = r.Os("items"), true
	})
}

func (l *notificationList) read(id string) {
	for _, n := range l.items {
		if id == "" || n.S("id") == id {
			n["read"] = true
		}
	}
	body := core.J{"all": true}
	if id != "" {
		body = core.J{"ids": []string{id}}
	}
	go api().Post("/api/notifications/read", body)
}

func (l *notificationList) view(c *ui.Context, onClose func()) {
	s := design.S()
	tt := design.TT()
	unread := 0
	for _, n := range l.items {
		if !n.IsTrue("read") {
			unread++
		}
	}
	ui.Row(c).FillWidth().Padding(design.SpaceLG, design.SpaceSM, design.SpaceSM, design.SpaceXL).AlignItems(ui.Center).Children(func() {
		m3.Text(c, l10n.CommonNotifications, tt.TitleLarge.C(s.OnSurface)).Grow(1).Basis(0)
		if unread > 0 && m3.TextButton(c, l10n.NotificationsMarkAllRead).Clicked() {
			l.read("")
		}
		if onClose != nil && m3.IconBtn(c, icons.CloseRounded, l10n.CommonClose).Clicked() {
			onClose()
		}
	})
	m3.Text(c, l10n.NotificationsIntro, tt.BodySmall.C(s.OnSurfaceVariant)).PaddingX(design.SpaceXL)
	m3.VGap(c, design.SpaceSM)
	ui.Column(c).Grow(1).Basis(0).FillWidth().Children(func() {
		switch {
		case !l.loaded && l.err != nil:
			widgets.ErrorState(c, l.err, l.load)
		case !l.loaded:
			widgets.CenteredLoader(c, "")
		case len(l.items) == 0:
			widgets.EmptyState{Icon: icons.NotificationsNoneRounded, Title: l10n.NotificationsEmpty, Body: l10n.NotificationsEmptyBody}.Show(c)
		default:
			ui.Scroll(c).Fill().TrackScroll(&l.scroll).Children(func() {
				ui.Column(c).FillWidth().Padding(0, design.SpaceSM, design.SpaceLG, design.SpaceSM).Children(func() {
					for _, n := range l.items {
						l.tile(c, n, onClose)
					}
				})
			})
		}
	})
}

func (l *notificationList) tile(c *ui.Context, n core.J, onClose func()) {
	s := design.S()
	read := n.IsTrue("read")
	sub := []string{}
	if b := n.S("body"); b != "" {
		sub = append(sub, b)
	}
	if r := core.FmtRelative(time.Unix(int64(n.F("at")), 0)); r != "" {
		sub = append(sub, r)
	}
	ts := design.TT().BodyLarge.C(s.OnSurface)
	if !read {
		ts = ts.W(600)
	}
	t := m3.Tile{
		Tap: true, Radius: design.CornerLG, Title: n.S("title"), TitleStyle: &ts,
		Subtitle: strings.Join(sub, "\n"), SubtitleLines: 3,
		Leading: func() {
			bg, fg := s.SecondaryContainer, s.OnSecondaryContainer
			if read {
				bg, fg = s.SurfaceContainerHighest, s.OnSurfaceVariant
			}
			widgets.CircleAvatar(c, 40, bg, func() { m3.Icon(c, NotificationIcon(n.S("kind")), 20, fg) })
		},
	}
	if !read {
		t.Trailing = func() {
			ui.Box(c).Size(8, 8).Radius(4).Background(s.Primary).Label(l10n.NotificationsUnread)
		}
	}
	if t.Show(c.Key(n.S("id"))).Clicked() {
		l.read(n.S("id"))
		if onClose != nil {
			onClose()
		}
		OpenNotification(c, n)
	}
}

func itoa(n int) string { return core.Str(float64(n)) }
