package app

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// Tab indices, shared with pages that switch tabs.
const (
	TabHome = iota
	TabCourses
	TabSchedule
	TabDownloads
	TabLibrary
	TabTasks
	TabPlugins
	TabSettings
)

type dest struct {
	label              string
	icon, selectedIcon icons.Icon
	screen             string
}

var dests = []dest{
	{l10n.ScreenHome, icons.HomeOutlined, icons.HomeRounded, l10n.ScreenHome},
	{l10n.CommonCourses, icons.SchoolOutlined, icons.SchoolRounded, l10n.NavCourses},
	{l10n.CommonSchedule, icons.EventNoteOutlined, icons.EventNoteRounded, l10n.CommonSchedule},
	{l10n.CommonDownload, icons.DownloadOutlined, icons.DownloadRounded, l10n.ScreenDownloads},
	{l10n.ScreenLibrary, icons.AutoStoriesOutlined, icons.AutoStoriesRounded, l10n.ScreenLibrary},
	{l10n.NavTasks, icons.ScheduleOutlined, icons.ScheduleRounded, l10n.CommonScheduledTasks},
	{l10n.CommonPlugins, icons.ExtensionOutlined, icons.ExtensionRounded, l10n.CommonPlugins},
	{l10n.CommonSettings, icons.TuneOutlined, icons.TuneRounded, l10n.CommonSettings},
}

// barTabs leave out 定时 and 插件, reached from 设置 and the assistant there.
var barTabs = []int{TabHome, TabCourses, TabSchedule, TabDownloads, TabLibrary, TabSettings}

// Shell is AppShell: the tabs, each with a navigator of its own, the rail
// or the phone dock, and the docked assistant.
type Shell struct {
	index     int
	navs      [8]*m3.Navigator
	built     map[int]bool
	switched  time.Time
	dockSmall bool
	// The vertical offset of the page's scroll last frame, for the dock.
	lastY    float32
	listened bool
}

// The shell, for pages outside the tab navigators (the assistant panel).
var shell *Shell

// BottomInset is the room the phone dock takes at the bottom, which pages
// keep clear as they would the view padding.
var BottomInset float32

// MainScroller is a page whose main scroll view shrinks the phone dock.
type MainScroller interface{ MainScroll() *ui.ScrollState }

// NewShell builds the shell on the home tab.
func NewShell() *Shell {
	s := &Shell{built: map[int]bool{0: true}}
	shell = s
	return s
}

func rootPage(i int) m3.Page {
	switch i {
	case TabHome:
		return NewHomePage()
	case TabCourses:
		return NewCoursesPage()
	case TabSchedule:
		return NewSchedulePage()
	case TabDownloads:
		return NewDownloadsPage()
	case TabLibrary:
		return NewLibraryPage()
	case TabTasks:
		return NewTasksPage()
	case TabPlugins:
		return NewPluginsPage()
	}
	return NewSettingsPage()
}

func (s *Shell) nav(i int) *m3.Navigator {
	if s.navs[i] == nil {
		s.navs[i] = m3.NewNavigator(rootPage(i))
	}
	return s.navs[i]
}

// GoTo switches tabs; the current tab's button pops it to its root.
func GoTo(tab int) {
	if shell != nil {
		shell.selectTab(tab)
	}
}

// CurrentTab is the selected tab.
func CurrentTab() int {
	if shell == nil {
		return TabHome
	}
	return shell.index
}

// PopTab pops the page on top of tab's navigator.
func PopTab(tab int) {
	if shell != nil && shell.navs[tab] != nil {
		shell.navs[tab].MaybePop()
	}
}

// PushTab switches to tab and opens page on top of it.
func PushTab(tab int, page m3.Page) {
	if shell == nil {
		return
	}
	shell.selectTab(tab)
	shell.nav(tab).Push(page)
}

func (s *Shell) selectTab(i int) {
	if i == s.index {
		s.nav(i).PopUntilRoot()
		return
	}
	s.index = i
	s.built[i] = true
	s.dockSmall = false
	s.switched = time.Now()
	Inbox.SetViewing(i == TabTasks)
	Screen.Set(map[string]string{"screen": dests[i].screen})
}

func (s *Shell) listen() {
	if s.listened {
		return
	}
	s.listened = true
	// Plugin notices (coursive.notify) show wherever the user is.
	Plugins.OnNotice = func(m string) { widgets.ShowSnack(m, "", nil) }
	// The bot's app operations, and the preferences it may read.
	Assistant.AppOps.Listen(func(op core.J) { RunBotOp(op) })
	// New replays, lectures and Canvas items.
	State.Events.Listen(func(e core.J) {
		if e.S("type") != "notification" {
			return
		}
		n := e.O("data")
		widgets.ShowSnack(n.S("title"), l10n.CommonView, func() { pendingNotification = n })
	})
	core.UI.OnChange = func() { Assistant.PushAppState(core.UI.JSON()) }
	Assistant.PushAppState(core.UI.JSON())
}

// pendingNotification is opened in the next build, which has a context.
var pendingNotification core.J

func (s *Shell) icon(c *ui.Context, i int, compact bool) widgets.NavItem {
	d := dests[i]
	it := widgets.NavItem{Icon: d.icon, SelectedIcon: d.selectedIcon, Label: d.label}
	switch {
	case i == TabDownloads:
		it.Count = State.ActiveJobCount()
	case i == TabSchedule:
		it.Count = Schedule.DueSoon()
	case i == TabTasks:
		it.Count = Inbox.Unread
	case i == TabPlugins:
		it.Count = Plugins.Attention()
	case i == TabSettings && compact:
		it.Dot = Inbox.Unread+Plugins.Attention() > 0
	}
	return it
}

// View builds the shell.
func (s *Shell) View(c *ui.Context) {
	s.listen()
	if pendingNotification != nil {
		n := pendingNotification
		pendingNotification = nil
		OpenNotification(c, n)
	}
	w, _ := c.Size()
	sc := design.SizeClassOf(w)
	cs := design.S()
	if sc == design.SizeCompact {
		s.compact(c)
		return
	}
	BottomInset = 0
	docked := sc >= design.SizeExpanded && core.UI.AssistantDocked
	ui.Row(c).Fill().Background(cs.Surface).Children(func() {
		ui.Column(c).FillHeight().Padding(design.SpaceSM, design.SpaceXS, design.SpaceLG, design.SpaceMD).Children(func() {
			var items []widgets.NavItem
			for i := range dests {
				items = append(items, s.icon(c, i, false))
			}
			// Docked, the rail has no leading control and no gap for one.
			var leading func()
			if !docked {
				leading = func() { assistantFab(c) }
			}
			tapped := widgets.Rail{
				Items: items, Selected: s.index,
				Brand:   func() { brandMark(c) },
				Leading: leading,
				Trailing: func() {
					ui.Column(c).AlignItems(ui.Center).Children(func() {
						NotificationBell(c)
						m3.VGap(c, design.SpaceSM)
						av := ui.ButtonBase(c).FocusRing(false).Radius(design.CornerFull).Cursor(ui.CursorPointer).Label(l10n.ShellProfileTooltip(UserDisplayName()))
						m3.InkWell(av, design.CornerFull)
						m3.Tip(c, av, l10n.ShellProfileTooltip(UserDisplayName()))
						av.Children(func() { UserAvatar(c, 40) })
						if av.Clicked() {
							ShowProfileDialog()
						}
					})
				},
			}.Show(c)
			if tapped >= 0 {
				s.selectTab(tapped)
			}
		})
		right := float32(design.SpaceLG)
		if docked {
			right = 0
		}
		ui.Column(c).Grow(1).Basis(0).FillHeight().Padding(design.SpaceSM, right, design.SpaceLG, 0).Children(func() {
			ui.Column(c).Fill().Radius(design.CornerXL).Clip().Background(cs.SurfaceContainerLowest).Children(func() { s.content(c) })
		})
		panel := ui.Box(c).FillHeight().Shrink(0).Clip().Transition(ui.ElementTransition{Duration: design.T().Motion(design.Medium4, nil).Duration, Ease: design.Emphasized, Width: true})
		panel.Children(func() {
			if !docked {
				return
			}
			pw := float32(380)
			if sc >= design.SizeLarge {
				pw = 440
			}
			ui.Box(c).FillHeight().Padding(design.SpaceSM, design.SpaceLG, design.SpaceLG, design.SpaceSM).Children(func() {
				ui.Column(c).Width(pw).FillHeight().Radius(design.CornerXL).Clip().Background(cs.SurfaceContainerLow).Children(func() {
					dockedAssistant.View(c)
				})
			})
		})
	})
}

func (s *Shell) compact(c *ui.Context) {
	cs := design.S()
	depth := s.nav(s.index).Depth()
	show := depth == 0 && !keyboardShown()
	selected := -1
	for i, t := range barTabs {
		if t == s.index {
			selected = i
		}
	}
	if selected < 0 {
		for i, t := range barTabs {
			if t == TabSettings {
				selected = i
			}
		}
	}
	BottomInset = 0
	if show {
		BottomInset = design.SpaceSM + widgets.DockClearance
	}
	ui.Box(c).Fill().Background(cs.Surface).Children(func() {
		ui.Column(c).Fill().Children(func() { s.content(c) })
		dock := ui.Box(c).Absolute().Left(design.SpaceLG).Right(design.SpaceLG).Bottom(design.SpaceSM)
		ease := design.EmphasizedAccelerate
		if show {
			ease = design.EmphasizedDecelerate
		}
		off := dock.AnimateTo("dock.slide", map[bool]float32{true: 0, false: 160}[show], design.T().Motion(design.Medium2, ease))
		dock.TranslatePercent(0, off).IgnoreInput(!show)
		dock.Children(func() {
			var items []widgets.NavItem
			for _, t := range barTabs {
				items = append(items, s.icon(c, t, true))
			}
			r := widgets.Dock{Items: items, Selected: selected, Minimized: s.dockSmall, Accessory: func() { assistantDockButton(c) }}.Show(c)
			if r.Expand {
				s.dockSmall = false
			}
			if r.Selected >= 0 {
				s.selectTab(barTabs[r.Selected])
			}
		})
	})
}

// keyboardShown is whether an on-screen keyboard covers the bottom.
func keyboardShown() bool { return false }

// content is the expired banners and the tabs, each keeping its navigator;
// switching fades through.
func (s *Shell) content(c *ui.Context) {
	for _, src := range State.Sources() {
		if src.Expired() {
			expiredBanner(c, src)
		}
	}
	ui.Box(c).Grow(1).Basis(0).FillWidth().Clip().Children(func() {
		nav := s.nav(s.index)
		el := time.Since(s.switched)
		t := float32(1)
		if !s.switched.IsZero() && !design.T().ReduceMotion {
			t = min(float32(el)/float32(design.Medium2), 1)
		}
		if t < 1 {
			c.Invalidate()
		}
		fade := design.EmphasizedDecelerate(min(max((t-.3)/.7, 0), 1))
		scale := .97 + .03*design.EmphasizedDecelerate(t)
		slot := ui.Box(c.Key(s.index)).Absolute().Top(0).Left(0).Right(0).Bottom(0).Opacity(fade).Scale(scale, scale)
		slot.Children(func() { nav.View(c) })
		s.trackScroll(nav)
	})
}

// trackScroll shrinks the dock while the page scrolls toward its end and
// brings it back on the way up.
func (s *Shell) trackScroll(nav *m3.Navigator) {
	ms, ok := nav.Top().(MainScroller)
	if !ok {
		return
	}
	st := ms.MainScroll()
	y := st.Y
	switch {
	case y > s.lastY+.5:
		s.dockSmall = y > design.SpaceXXL
	case y < s.lastY-.5:
		s.dockSmall = false
	}
	s.lastY = y
}

// brandMark is the app mark on the rail: a soft twelve-lobed cookie holding
// a play glyph.
func brandMark(c *ui.Context) {
	cs := design.S()
	b := ui.Box(c).Size(36, 36).Center().Draw(func(p *ui.Painter, r ui.Rect) {
		path := widgets.ShapePath(widgets.ShapeRadii(widgets.ShapeCookie12), r, 0)
		p.FillPath(&path, cs.PrimaryContainer)
		var dot ui.Path
		dot.Circle(r.X+r.W-4, r.Y+4, 3.5)
		p.FillPath(&dot, cs.Primary)
	})
	m3.Tip(c, b, "Coursive")
	b.Children(func() { m3.Icon(c, icons.PlayArrowRounded, 20, cs.OnPrimaryContainer) })
}

// assistantFab is the bot on the rail: tap to open it.
func assistantFab(c *ui.Context) {
	a := Assistant
	f := m3.FAB{Tooltip: a.BotName(), BG: design.S().SurfaceContainerHigh, Flat: true, Child: func() {
		BotAvatar{Size: 44, Shape: a.BotShape(), Color: a.BotColor(), Mood: a.Mood()}.Show(c)
	}}.Show(c)
	f.Label(l10n.BotMoodLabel(a.BotName(), a.Mood().Label()))
	if f.Clicked() {
		OpenAssistantPanel(c)
		if a.View != "bot" {
			a.ShowBot()
		}
	}
}

// assistantDockButton is the assistant beside the phone dock, in the same
// glass.
func assistantDockButton(c *ui.Context) {
	a := Assistant
	b := ui.Box(c).Size(widgets.DockHeight, widgets.DockHeight).Center().Cursor(ui.CursorPointer).
		Label(l10n.BotMoodLabel(a.BotName(), a.Mood().Label()))
	b = widgets.Glass{Radius: design.CornerFull}.Apply(b)
	m3.InkWell(b, design.CornerFull)
	m3.Tip(c, b, a.BotName())
	b.Children(func() { BotAvatar{Size: 40, Shape: a.BotShape(), Color: a.BotColor(), Mood: a.Mood()}.Show(c) })
	if b.Clicked() {
		OpenAssistantPanel(c)
		if a.View != "bot" {
			a.ShowBot()
		}
	}
}

// expiredBanner shows while a source's sign-in has expired; it opens the
// sign-in sheet.
func expiredBanner(c *ui.Context, src core.Source) {
	cs := design.S()
	row := ui.Row(c.Key("expired-"+src.ID())).FillWidth().Background(cs.ErrorContainer).Padding(design.SpaceMD, design.SpaceLG).
		AlignItems(ui.Center).Cursor(ui.CursorPointer)
	m3.InkWell(row, 0)
	row.Children(func() {
		m3.Icon(c, icons.LockClockOutlined, 24, cs.OnErrorContainer)
		m3.HGap(c, design.SpaceMD)
		m3.Text(c, l10n.SourceExpiredBanner(src.Title()), design.TT().BodyMedium.C(cs.OnErrorContainer)).Grow(1).Basis(0)
		m3.Text(c, l10n.LoginSignInAgain, design.TT().LabelLarge.C(cs.OnErrorContainer))
		m3.Icon(c, icons.ChevronRightRounded, 24, cs.OnErrorContainer)
	})
	if row.Clicked() {
		SignInSource(src)
	}
}
