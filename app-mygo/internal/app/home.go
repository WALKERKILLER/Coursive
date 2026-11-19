package app

import (
	"math"
	"slices"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// HomePage is home_page.dart's: the greeting with a line of 一言, what is
// coming up, the activity heatmap, the lectures to continue, downloads in
// progress and this term's courses.
type HomePage struct {
	recent   []core.Lecture
	cancel   func()
	activity *ActivityCard
	scroll   ui.ScrollState
	carousel ui.ScrollState
	refresh  ui.RefreshState
	quote    quoteLine
}

// NewHomePage loads the recent lectures and follows their changes.
func NewHomePage() m3.Page {
	p := &HomePage{activity: NewActivityCard()}
	p.load(nil)
	p.cancel = State.Events.Listen(func(e core.J) {
		if t := e.S("type"); t == "lecture_changed" || t == "notes_changed" {
			p.load(nil)
		}
	})
	p.quote.init()
	return p
}

// Dispose stops listening.
func (p *HomePage) Dispose() {
	p.cancel()
	p.activity.Dispose()
}

// MainScroll is the page's scroll, which shrinks the dock.
func (p *HomePage) MainScroll() *ui.ScrollState { return &p.scroll }

func (p *HomePage) load(done func()) {
	api := State.Api()
	core.Async(func() ([]core.Lecture, error) {
		v, err := api.Get("/api/recent")
		var out []core.Lecture
		for _, j := range core.Objs(v) {
			out = append(out, core.Lecture{J: j})
		}
		return out, err
	}, func(r []core.Lecture, err error) {
		if err == nil {
			p.recent = r
		}
		if done != nil {
			done()
		}
	})
}

func greeting() string {
	h := time.Now().Hour()
	switch {
	case h < 5:
		return l10n.HomeGreetingNight
	case h < 11:
		return l10n.HomeGreetingMorning
	case h < 13:
		return l10n.HomeGreetingNoon
	case h < 18:
		return l10n.HomeGreetingAfternoon
	}
	return l10n.HomeGreetingEvening
}

func kindIcon(k string) icons.Icon {
	switch k {
	case "video":
		return icons.MovieOutlined
	case "slides":
		return icons.SlideshowOutlined
	case "courseware":
		return icons.AttachFileRounded
	case "transcript":
		return icons.SubjectRounded
	case "parse":
		return icons.DocumentScannerOutlined
	}
	return icons.DownloadOutlined
}

// View builds the page.
func (p *HomePage) View(c *ui.Context) {
	cs := design.S()
	compact := Compact()
	pad := design.SpaceXL
	if compact {
		pad = design.SpaceLG
	}
	latest := ""
	for _, co := range State.Courses {
		if co.Term() > latest {
			latest = co.Term()
		}
	}
	var term []core.Course
	for _, co := range State.Courses {
		if co.Term() == latest && !co.Hidden() {
			term = append(term, co)
		}
	}
	slices.SortStableFunc(term, func(a, b core.Course) int {
		switch {
		case a.LastLearnAt() > b.LastLearnAt():
			return -1
		case a.LastLearnAt() < b.LastLearnAt():
			return 1
		}
		return 0
	})
	var active []core.Job
	for _, j := range State.SortedJobs() {
		if j.Active() && len(active) < 3 {
			active = append(active, j)
		}
	}
	// Without any course, starting one is the whole point of the page.
	first := len(State.Courses) == 0 && !State.CoursesLoading && State.CoursesError == nil
	now := time.Now()

	sc := ui.Scroll(c).Fill().TrackScroll(&p.scroll)
	if sc.PullToRefresh(&p.refresh) {
		pending := 2
		done := func() {
			if pending--; pending == 0 {
				p.refresh.Refreshing = false
			}
		}
		State.LoadCoursesThen(true, false, done)
		p.load(done)
	}
	section := func(child func()) {
		widgets.ContentWidth(c, 0, func() { ui.Column(c).FillWidth().PaddingX(pad).Children(child) })
	}
	header := func(title string, action func()) {
		widgets.ContentWidth(c, 0, func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(design.SpaceXL, pad-design.SpaceSM, design.SpaceMD, pad).Children(func() {
				m3.Text(c, title, design.TT().TitleLarge).Grow(1).Basis(0)
				if action != nil {
					action()
				}
			})
		})
	}
	sc.Children(func() {
		widgets.ContentWidth(c, 0, func() {
			top := design.SpaceXXL
			if compact {
				top = design.SpaceXL
			}
			ui.Column(c).FillWidth().Padding(top, pad, design.SpaceLG, pad).Children(func() { p.welcome(c, now, compact) })
		})
		section(func() {
			ui.Row(c).FillWidth().Children(func() {
				if (m3.Button{Kind: m3.TextKind, Icon: icons.ChecklistRounded, Label: l10n.SetupTitle}).Show(c).Clicked() {
					OpenSettingsSection("diagnostics")
					GoTo(TabSettings)
				}
			})
		})
		if first {
			widgets.ContentWidth(c, 0, func() {
				box := ui.Column(c).FillWidth().Padding(design.SpaceSM, pad, design.SpaceLG, pad)
				w := widgets.Width(box, 800) - 2*pad
				box.Children(func() { FirstCoursePanel(c, w) })
			})
		}
		if len(Schedule.Upcoming(14)) > 0 {
			header(l10n.HomeUpNext, func() {
				if m3.TextButton(c, l10n.CommonSchedule).Clicked() {
					GoTo(TabSchedule)
				}
			})
			section(func() {
				UpcomingSchedule(c, func(d time.Time) {
					Focus.Day = &d
					GoTo(TabSchedule)
				})
			})
		}
		header(l10n.CommonActivity, func() {
			if m3.TextButton(c, l10n.HomeStatistics).Clicked() {
				m3.Nav().Push(NewActivityPage())
			}
		})
		section(func() { p.activity.Show(c) })
		if len(p.recent) > 0 {
			header(l10n.HomeContinueWatching, nil)
			widgets.ContentWidth(c, 0, func() {
				ui.Box(c).FillWidth().PaddingX(pad - 4).Children(func() { p.continueCarousel(c) })
			})
		}
		if len(active) > 0 {
			header(l10n.HomeDownloading, func() {
				if m3.TextButton(c, l10n.CommonAll).Clicked() {
					GoTo(TabDownloads)
				}
			})
			section(func() {
				m3.Card(c, m3.CardElevated).FillWidth().Children(func() {
					for _, j := range active {
						m3.Tile{LeadingIcon: kindIcon(j.Kind()), Title: j.Title(), TitleLines: 1,
							SubtitleFn: func() {
								ui.Box(c).FillWidth().Padding(design.SpaceXS, 0, 0, 0).Children(func() { widgets.JobProgressLine(c, j) })
							}}.Show(c.Key(j.ID()))
					}
				})
			})
		}
		if !first {
			title := l10n.HomeThisTermCourses
			if latest == "" {
				title = l10n.CommonCourses
			}
			header(title, func() {
				if m3.TextButton(c, l10n.HomeAllCourses).Clicked() {
					GoTo(TabCourses)
				}
			})
			switch {
			case State.CoursesLoading && len(State.Courses) == 0:
				ui.Box(c).FillWidth().Height(120).Padding(design.SpaceXXL).Children(func() { widgets.CenteredLoader(c, "") })
			case State.CoursesError != nil && len(State.Courses) == 0:
				widgets.ErrorState(c, State.CoursesError, func() { State.LoadCourses(true, false) })
			default:
				section(func() {
					g := ui.Grid(c).FillWidth().Gap(design.SpaceMD)
					cols := min(max(int(math.Floor(float64(widgets.Width(g, 900)/300))), 1), 4)
					items := term
					if len(items) > cols*2 {
						items = items[:cols*2]
					}
					g.Columns(cols).Children(func() {
						for _, co := range items {
							if (CourseCard{Course: co}).Show(c) {
								m3.Nav().Push(NewCoursePage(co.ID()))
							}
						}
					})
				})
			}
		}
		m3.VGap(c, design.SpaceHuge+BottomInset)
	})
	_ = cs
}

// welcome is the greeting header: the avatar and nickname with 一言.
func (p *HomePage) welcome(c *ui.Context, now time.Time, compact bool) {
	cs := design.S()
	tt := design.TT()
	avatar, gap := float32(72), design.SpaceXL
	if compact {
		avatar, gap = 56, design.SpaceLG
	}
	ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
		b := ui.ButtonBase(c.Key("avatar")).FocusRing(false).Cursor(ui.CursorPointer).Shrink(0).Label(l10n.ProfileEdit)
		hover := b.Hovered()
		anim := design.T().Motion(design.Medium2, design.Emphasized)
		turn := b.AnimateTo("turn", map[bool]float32{true: -.02 * 360, false: 0}[hover], anim)
		sc := b.AnimateTo("scale", map[bool]float32{true: 1.06, false: 1}[hover], anim)
		m3.Tip(c, b, l10n.ProfileEdit)
		b.Rotate(turn).Scale(sc, sc).Children(func() { UserAvatar(c, avatar) })
		if b.Clicked() {
			ShowProfileDialog()
		}
		m3.HGap(c, gap)
		ui.Column(c).Grow(1).Basis(0).Children(func() {
			m3.Text(c, l10n.HomeDate(int(now.Month()), now.Day(), core.Weekdays[core.Weekday(now)-1]), tt.LabelLarge.C(cs.Primary).L(.4))
			m3.VGap(c, design.SpaceXXS)
			st := tt.DisplaySmall
			if compact {
				st = tt.HeadlineMedium
			}
			ui.RichText(c, st.C(cs.OnSurfaceVariant).Span(l10n.HomeGreeting(greeting())), st.C(cs.OnSurface).Span(UserDisplayName())).
				MaxLines(2).Ellipsis("…")
		})
		// The navigation rail carries the bell on wider windows.
		if compact {
			NotificationBell(c)
		}
	})
	show := Profile.ShowQuote
	ui.Box(c).FillWidth().Clip().Transition(ui.ElementTransition{Duration: design.T().Motion(design.Medium4, nil).Duration, Ease: design.Emphasized, Height: true}).
		Children(func() {
			if show {
				ui.Box(c).FillWidth().Padding(design.SpaceMD, 0, 0, 0).Children(func() { p.quote.show(c, avatar, gap) })
			}
		})
}

// quoteLine is one line of 一言 in the header: an organic quote mark under
// the avatar (click for another line; it morphs while loading) and the
// text under the greeting.
type quoteLine struct {
	q         *core.Quote
	loading   bool
	spinStart time.Time
	// spinEnd is when the mark finishes its last turn after the load.
	spinEnd time.Time
}

func (l *quoteLine) init() {
	l.q = core.CurrentQuote()
	if l.q == nil {
		l.q = core.CachedQuote()
	}
	l.load(false)
}

func (l *quoteLine) load(refresh bool) {
	if l.loading {
		return
	}
	l.loading = true
	l.spinStart, l.spinEnd = time.Now(), time.Time{}
	types := Profile.QuoteTypes
	core.Async(func() (core.Quote, error) { return core.FetchQuote(types, refresh), nil }, func(q core.Quote, _ error) {
		core.SetQuote(q)
		l.q, l.loading = &q, false
		// Let the mark finish its cycle so that it settles at rest.
		el := time.Since(l.spinStart)
		cycles := el/(1100*time.Millisecond) + 1
		l.spinEnd = l.spinStart.Add(cycles * 1100 * time.Millisecond)
	})
}

func (l *quoteLine) spin(c *ui.Context) float64 {
	if l.spinStart.IsZero() || design.T().ReduceMotion {
		return 0
	}
	now := time.Now()
	if !l.loading && !l.spinEnd.IsZero() && !now.Before(l.spinEnd) {
		return 0
	}
	c.Invalidate()
	el := now.Sub(l.spinStart)
	return float64(el%(1100*time.Millisecond)) / float64(1100*time.Millisecond)
}

func (l *quoteLine) show(c *ui.Context, lead, gap float32) {
	cs := design.S()
	tt := design.TT()
	ui.Row(c).FillWidth().AlignItems(ui.Start).Children(func() {
		ui.Box(c).Width(lead).Shrink(0).Center().Children(func() {
			mark := ui.ButtonBase(c.Key("mark")).FocusRing(false).Size(40, 40).Cursor(ui.CursorPointer).Center().Label(l10n.HomeNextQuote)
			hover := mark.Hovered()
			m3.Tip(c, mark, l10n.HomeNextQuote)
			sc := mark.AnimateTo("scale", map[bool]float32{true: 1.08, false: 1}[hover], design.T().Motion(design.Medium2, design.Emphasized))
			h := float64(mark.AnimateTo("hover", map[bool]float32{true: 1, false: 0}[hover], design.T().Motion(design.Long2, design.Emphasized)))
			spin := l.spin(c)
			morph := math.Max(h*.35, math.Sin(spin*math.Pi))
			turn := spin + h*.04
			k := float32(h)
			if l.loading {
				k = 1
			}
			fill := design.Lerp(cs.TertiaryContainer, cs.Tertiary, k)
			ink := design.Lerp(cs.OnTertiaryContainer, cs.OnTertiary, k)
			mark.Scale(sc, sc).Draw(func(p *ui.Painter, r ui.Rect) {
				path := widgets.ShapePath(widgets.MorphRadii(widgets.ShapeCookie6, widgets.ShapeSoftBurst, min(max(morph, 0), 1)), r, turn*math.Pi*2/3)
				p.FillPath(&path, fill)
			}).Children(func() { m3.Icon(c, icons.FormatQuoteRounded, 20, ink) })
			if mark.Clicked() {
				l.load(true)
			}
		})
		m3.HGap(c, gap)
		ui.Box(c).Grow(1).Basis(0).Padding(2, 0, 0, 0).Children(func() {
			q := l.q
			key := ""
			if q != nil {
				key = q.Text
			}
			col := ui.Column(c.Key("q" + key)).FillWidth().Transition(ui.ElementTransition{
				Duration: design.T().Motion(design.Medium4, nil).Duration, Ease: design.EmphasizedDecelerate,
				Enter: &ui.Motion{Opacity: 0, YPercent: 12}})
			col.Children(func() {
				if q == nil {
					m3.VGap(c, 40)
					return
				}
				m3.Text(c, q.Text, tt.TitleMedium.W(400).C(cs.OnSurface).H(1.55).L(.2)).Selectable()
				if s := q.Source(); s != "" {
					m3.VGap(c, design.SpaceXXS)
					m3.Text(c, s, tt.BodySmall.C(cs.OnSurfaceVariant))
				}
			})
		})
	})
}

// continueCarousel is CarouselView of the recent lectures: 280 wide cards
// that shrink to 180 as they leave at the start.
func (p *HomePage) continueCarousel(c *ui.Context) {
	const extent, shrink = 280, 180
	n := len(p.recent)
	ui.Box(c).FillWidth().Height(200).Children(func() {
		widgets.HScroller{State: &p.carousel, Buttons: true, Step: extent}.Show(c, func() {
			x := p.carousel.X
			ui.Box(c).Width(float32(n) * extent).Height(200).Shrink(0).Children(func() {
				for i, l := range p.recent {
					start := float32(i)*extent - x
					vx, w := start, float32(extent)
					if start < 0 {
						w = max(extent+start, shrink)
						vx = min(0, start+extent-shrink)
					}
					card := m3.Tappable(c.Key(l.ID()), design.CornerXL).Absolute().Left(x + vx + 4).Top(0).Width(w - 8).Height(200).
						Background(design.S().SurfaceContainer).Clip()
					card.Children(func() {
						co, ok := State.Course(l.CourseID())
						continueCard(c, l, co, ok, w-8)
					})
					if card.Clicked() {
						m3.RootNav.Push(NewPlayerPage(l.ID(), -1))
					}
				}
			})
		})
	})
}

func continueCard(c *ui.Context, l core.Lecture, co core.Course, ok bool, width float32) {
	cs := design.S()
	tt := design.TT()
	dur := l.DurationS() * 1000
	prog := 0.0
	if dur > 0 {
		prog = min(max(float64(l.PositionMs())/float64(dur), 0), 1)
	}
	scheme := cs
	if ok {
		scheme = widgets.CourseScheme(co)
	}
	roomy := width > 220
	ui.Column(c).Fill().Children(func() {
		ui.Box(c).FillWidth().Grow(1).Basis(0).Clip().Children(func() {
			fallback := func() {
				f := ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Background(scheme.PrimaryContainer)
				if ok {
					widgets.CourseArt(f, co, 0, 0, 3, 0)
				}
			}
			if l.Thumb() != "" {
				widgets.NetImage(c, l.Thumb(), fallback)
			} else {
				fallback()
			}
			ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Draw(func(p *ui.Painter, r ui.Rect) {
				p.FillGradient(r, ui.LinearGradient{Angle: 180, Stops: []ui.GradientStop{
					{Color: ui.Transparent, At: .5}, {Color: ui.RGBA(0, 0, 0, .45), At: 1}}}, 0)
			})
			ui.Box(c).Absolute().Left(design.SpaceMD).Bottom(design.SpaceSM).Size(36, 36).Center().Draw(func(p *ui.Painter, r ui.Rect) {
				path := widgets.StarPath(r, 8, .86, .6, .4)
				p.FillPath(&path, scheme.PrimaryContainer)
			}).Children(func() { m3.Icon(c, icons.PlayArrowRounded, 22, scheme.OnPrimaryContainer) })
			if roomy {
				m3.Text(c, core.FmtClock(l.PositionMs())+" / "+core.FmtClock(dur), tt.LabelSmall.C(ui.RGB(255, 255, 255))).FontFeatures("tnum").
					Absolute().Right(design.SpaceMD).Bottom(design.SpaceMD)
			}
		})
		m3.Linear(c, m3.Progress{Value: m3.Of(prog), Height: 3, Color: scheme.Primary, Track: scheme.PrimaryContainer})
		ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(design.SpaceSM, design.SpaceMD, design.SpaceMD, design.SpaceMD).Children(func() {
			if ok {
				widgets.CourseGlyph(c, co, 22, false)
				m3.HGap(c, design.SpaceSM)
			}
			ui.Column(c).Grow(1).Basis(0).Children(func() {
				title := ""
				if ok {
					title = co.Title()
				}
				m3.Clamp(m3.Text(c, title, tt.LabelMedium.C(cs.OnSurfaceVariant)), 1)
				m3.Clamp(m3.Text(c, l.ShortTitle(), tt.TitleSmall), 1)
			})
		})
	})
}
