package app

import (
	"maps"
	"slices"
	"strconv"
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

// CoursePage is course_page.dart's: a course's header, its LMS and plugin
// sections, and its lectures by month, which can be picked to download.
type CoursePage struct {
	id       string
	course   *core.Course
	lectures []core.Lecture
	err      error
	loading  bool
	syncing  bool
	// localNewestFirst is the order of a local course, kept per page.
	localNewestFirst bool
	sel              map[string]bool
	cancel           func()
	scroll           ui.ScrollState
	spinStart        time.Time
	theme            courseTheme
	lms              *LmsSection
}

// courseTheme caches the theme the page builds with: the app's with the
// course's accents.
type courseTheme struct {
	base   *design.Theme
	color  string
	title  string
	scheme *design.Scheme
	theme  *design.Theme
}

// NewCoursePage opens a course and loads its lectures.
func NewCoursePage(id string) m3.Page {
	p := &CoursePage{id: id, loading: true, sel: map[string]bool{}, spinStart: time.Now()}
	if co, ok := State.Course(id); ok {
		p.course = &co
	}
	p.load(false, false)
	p.lms = NewLmsSection(id)
	p.cancel = State.Events.Listen(func(e core.J) {
		if e.S("type") == "lecture_changed" && core.Obj(e["data"]).S("course_id") == id {
			p.load(false, true)
		}
	})
	return p
}

// Dispose stops listening for lecture changes.
func (p *CoursePage) Dispose() {
	p.cancel()
	p.lms.Dispose()
}

// MainScroll is the page's scroll.
func (p *CoursePage) MainScroll() *ui.ScrollState { return &p.scroll }

func (p *CoursePage) load(sync, quiet bool) {
	if sync {
		p.syncing = true
	}
	api := State.Api()
	path := "/api/courses/" + core.Esc(p.id) + "/lectures"
	if sync {
		path += "?sync=1"
	}
	core.Async(func() (any, error) { return api.Get(path) }, func(v any, err error) {
		if err != nil {
			if !quiet {
				p.err = err
			}
		} else {
			r := core.Obj(v)
			co := core.Course{J: core.Obj(r["course"])}
			p.course = &co
			p.lectures = nil
			for _, l := range core.List(r["lectures"]) {
				p.lectures = append(p.lectures, core.Lecture{J: core.Obj(l)})
			}
			p.err = nil
		}
		p.loading, p.syncing = false, false
	})
}

func (p *CoursePage) open(l core.Lecture) {
	m3.RootNav.PushThen(NewPlayerPage(l.ID(), -1), func() { p.load(false, true) })
}

// courseScheme is the page's theme for the course: the app's with the
// course's primary and container colours.
func (p *CoursePage) courseTheme(co *core.Course) (*design.Theme, *design.Scheme) {
	base := design.T()
	if co == nil {
		return base, base.Scheme
	}
	ct := &p.theme
	if ct.base != base || ct.color != co.Color() || ct.title != co.RawTitle() {
		s := widgets.CourseScheme(*co)
		sch := *base.Scheme
		sch.Primary, sch.PrimaryContainer, sch.OnPrimaryContainer = s.Primary, s.PrimaryContainer, s.OnPrimaryContainer
		sch.SecondaryContainer, sch.OnSecondaryContainer = s.SecondaryContainer, s.OnSecondaryContainer
		t := *base
		t.Scheme = &sch
		*ct = courseTheme{base, co.Color(), co.RawTitle(), s, &t}
	}
	return ct.theme, ct.scheme
}

// View builds the page.
func (p *CoursePage) View(c *ui.Context) {
	course := p.course
	if co, ok := State.Course(p.id); ok {
		course = &co
	}
	base := design.S()
	theme, s := p.courseTheme(course)
	defer design.Use(theme)()
	tt := design.TT()
	compact := Compact()
	pad := design.SpaceXL
	if compact {
		pad = design.SpaceLG
	}
	local := course != nil && course.IsLocal()
	newestFirst := core.UI.LecturesNewestFirst
	if local {
		newestFirst = p.localNewestFirst
	}
	// Lectures are numbered from the first to the last, whichever way they
	// are shown.
	ordered := slices.Clone(p.lectures)
	slices.SortStableFunc(ordered, func(a, b core.Lecture) int {
		if a.BeginAt() != b.BeginAt() {
			return a.BeginAt() - b.BeginAt()
		}
		return strings.Compare(a.ID(), b.ID())
	})
	ordinal := map[string]int{}
	for i, l := range ordered {
		ordinal[l.ID()] = i + 1
	}
	list := ordered
	if newestFirst {
		list = slices.Clone(ordered)
		slices.Reverse(list)
	}
	var playable []core.Lecture
	downloaded := 0
	for _, l := range list {
		if l.HasPlayback() {
			playable = append(playable, l)
		}
		if len(l.Assets()) > 0 {
			downloaded++
		}
	}
	// Resume where the student left off: the last lecture with progress if
	// unfinished, else the next playable one; first lecture when untouched.
	var chrono []core.Lecture
	for _, l := range ordered {
		if l.HasPlayback() {
			chrono = append(chrono, l)
		}
	}
	var resume *core.Lecture
	last := -1
	for i, l := range chrono {
		if l.PositionMs() > 0 {
			last = i
		}
	}
	switch {
	case last < 0 && len(chrono) > 0:
		resume = &chrono[0]
	case last >= 0:
		l := chrono[last]
		if l.DurationS() > 0 && l.PositionMs() >= l.DurationS()*900 {
			if last+1 < len(chrono) {
				resume = &chrono[last+1]
			}
		} else {
			resume = &chrono[last]
		}
	}
	// Interleave month headings with lectures.
	type row struct {
		month   string
		lecture core.Lecture
	}
	var rows []row
	month := ""
	for _, l := range list {
		m := l10n.CourseUnscheduled
		if local {
			m = l10n.LocalCourse
		} else if d := l.Begin(); !d.IsZero() {
			m = l10n.DateYearMonth(d.Year(), int(d.Month()))
		}
		if m != month {
			rows = append(rows, row{month: m})
		}
		month = m
		rows = append(rows, row{lecture: l})
	}

	screen := map[string]string{"screen": l10n.ScreenCourse, "course_id": p.id}
	if course != nil {
		screen["course"] = course.Title()
	}
	ReportScreen(screen)
	selecting := len(p.sel) > 0
	allPicked := len(p.sel) == len(playable)
	var leading func()
	if selecting {
		leading = func() {
			if m3.IconBtn(c, icons.CloseRounded, "").Clicked() {
				clear(p.sel)
			}
		}
	}

	m3.Scaffold{
		BG: ui.Transparent,
		AppBar: &m3.AppBar{
			BG:       base.SurfaceContainerLowest,
			Scrolled: p.scroll.Y > 0,
			TitleFn: func() {
				key := "none"
				if selecting {
					key = "count"
				}
				ui.Box(c.Key(key)).Transition(design.Switcher(design.Short4, nil, nil, ui.Motion{Opacity: 0})).Children(func() {
					if selecting {
						m3.Text(c, l10n.CourseSelectedCount(len(p.sel)), tt.TitleLarge)
					}
				})
			},
			Leading: leading,
			Actions: func() {
				if selecting {
					label := l10n.CommonSelectAll
					if allPicked {
						label = l10n.CommonDeselectAll
					}
					if m3.TextButton(c, label).Clicked() {
						if allPicked {
							clear(p.sel)
						} else {
							for _, l := range playable {
								p.sel[l.ID()] = true
							}
						}
					}
					return
				}
				if local && m3.IconBtn(c, icons.VideoCallOutlined, l10n.LocalVideosAdd).Clicked() {
					OpenLocalCourseDialog(p.id, nil)
				}
				if course != nil {
					ui.Box(c).Padding(0, design.SpaceXS, 0, 0).Children(func() {
						if (m3.Button{Kind: m3.Tonal, Icon: icons.ForumOutlined, Label: l10n.CourseAssistant}).Show(c).Clicked() {
							OpenAssistantPanel(c)
							Assistant.OpenCourseAgent(p.id, course.Title(), false)
						}
					})
				}
				b := m3.IconButton{Icon: icons.SyncRounded, Tooltip: l10n.CourseRefreshSchedule, Disabled: p.syncing}
				if p.syncing {
					b.Child = func(fg ui.Color) { m3.Spinner(c, 20, 2.5, design.S().Primary) }
				}
				if b.Show(c).Clicked() {
					p.load(true, false)
				}
				if course != nil {
					PluginMenuButton(c, "course.menu", core.J{"course_id": p.id}, false, "course")
					co := *course
					m3.MenuButton(c, "more", icons.MoreVertRounded, l10n.CommonMore, m3.Over|m3.Popup, func() []m3.MenuItem { return courseMenu(co) })
				}
				m3.HGap(c, design.SpaceSM)
			},
		},
		FABInset: BottomInset,
		FAB: func() {
			ease := design.EmphasizedAccelerate
			scale := float32(0)
			if selecting {
				ease, scale = design.EmphasizedDecelerate, 1
			}
			f := ui.Box(c)
			v := f.AnimateTo("scale", scale, design.T().Motion(design.Medium2, ease))
			f.Scale(v, v).Hidden(!selecting && v == 0).Children(func() {
				n := len(p.sel)
				if (m3.FAB{Icon: icons.DownloadRounded, Label: l10n.CourseDownloadSelected(n), Disabled: !selecting}).Show(c).Clicked() {
					ids := slices.Sorted(maps.Keys(p.sel))
					ShowDownloadSheet(ids, "", nil, func() { clear(p.sel) })
				}
			})
		},
		Body: func() {
			sc := ui.Scroll(c).Fill().TrackScroll(&p.scroll)
			sc.Children(func() {
				ui.Column(c).FillWidth().MinHeight(WindowHeight - 64).Children(func() {
					if course != nil {
						widgets.ContentWidth(c, 0, func() {
							ui.Box(c).FillWidth().Padding(0, pad, design.SpaceLG, pad).Children(func() {
								p.header(c, *course, s, len(list), downloaded, resume)
							})
						})
					}
					widgets.ContentWidth(c, 0, func() {
						ui.Column(c).FillWidth().PaddingX(pad).Children(func() {
							p.lms.Show(c)
							UiSlotOutlet(c, "course.detail.sections", core.J{"course_id": p.id})
						})
					})
					fill := func(child func()) { ui.Column(c).FillWidth().Grow(1).MinHeight(320).Children(child) }
					switch {
					case p.loading:
						fill(func() { widgets.CenteredLoader(c, l10n.CourseLoadingSchedule) })
					case p.err != nil && len(list) == 0:
						fill(func() { widgets.ErrorState(c, p.err, func() { p.load(true, false) }) })
					default:
						widgets.ContentWidth(c, 0, func() {
							ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(design.SpaceSM, pad-design.SpaceSM, design.SpaceSM, pad).Children(func() {
								m3.Text(c, l10n.CourseLectureTotal(len(list)), tt.TitleMedium).Grow(1).Basis(0)
								order := l10n.CourseOldestFirst
								if newestFirst {
									order = l10n.CourseNewestFirst
								}
								turn := float32(180)
								if newestFirst {
									turn = 0
								}
								if (m3.Button{Kind: m3.TextKind, Label: order, Child: func(fg ui.Color) {
									ui.Row(c).AlignItems(ui.Center).Children(func() {
										ic := ui.Box(c).Size(18, 18)
										ic.Rotate(ic.AnimateTo("turn", turn, design.T().Motion(design.Medium2, design.Emphasized))).Children(func() {
											m3.Icon(c, icons.ArrowDownwardRounded, 18, fg)
										})
										m3.HGap(c, design.SpaceSM)
										m3.Text(c, order, tt.LabelLarge.C(fg))
									})
								}}).Show(c).Clicked() {
									if local {
										p.localNewestFirst = !newestFirst
									} else {
										core.UI.Update(func(x *design.Prefs) { x.LecturesNewestFirst = !newestFirst })
									}
								}
								if (m3.Button{Kind: m3.TextKind, Icon: icons.ChecklistRounded, Label: l10n.CourseBatchDownload, Disabled: len(playable) == 0}).Show(c).Clicked() {
									// Start with the lectures that have nothing downloaded; when
									// all have something, with all of them.
									var missing []core.Lecture
									for _, l := range playable {
										if len(l.Assets()) == 0 {
											missing = append(missing, l)
										}
									}
									if len(missing) == 0 {
										missing = playable
									}
									for _, l := range missing {
										p.sel[l.ID()] = true
									}
								}
							})
						})
						ui.Column(c).FillWidth().Padding(0, pad, 120, pad).Children(func() {
							for i, r := range rows {
								if r.month != "" {
									widgets.ContentWidth(c, 0, func() {
										top := design.SpaceLG
										if i == 0 {
											top = 0
										}
										ui.Row(c.Key("m-"+r.month)).FillWidth().AlignItems(ui.Center).Padding(top, 0, design.SpaceSM, design.SpaceXS).Children(func() {
											st := tt.LabelLarge.C(s.Primary)
											st.LetterSpacing = .3
											m3.Text(c, r.month, st)
											m3.HGap(c, design.SpaceMD)
											ui.Box(c).Grow(1).Height(1).Background(design.WithOpacity(base.OutlineVariant, .6))
										})
									})
									continue
								}
								l := r.lecture
								widgets.ContentWidth(c, 0, func() {
									p.lectureTile(c, l, ordinal[l.ID()], selecting)
								})
							}
						})
					}
				})
			})
		},
	}.Show(c)
}

func (p *CoursePage) header(c *ui.Context, course core.Course, scheme *design.Scheme, lectures, downloaded int, resume *core.Lecture) {
	tt := design.TT()
	compact := Compact()
	teachers := course.Teacher()
	if ts := course.Teachers(); len(ts) > 0 {
		teachers = strings.Join(ts, l10n.ListSeparator)
	}
	art := func(e ui.Element) {
		e = e.Transition(courseHero(course.ID()))
		hover := e.Hovered()
		t := e.AnimateTo("morph", map[bool]float32{true: 1, false: 0}[hover], design.T().Motion(design.Long4, design.Emphasized))
		widgets.CourseArtTurning(e, course, float64(t), p.spinStart, 90*time.Second, design.CornerXL)
	}
	info := func() {
		ui.Column(c).Children(func() {
			st := tt.HeadlineMedium
			if compact {
				st = tt.HeadlineSmall
			}
			m3.Text(c, course.Title(), st.C(scheme.OnSurface))
			if course.Alias() != "" {
				m3.Text(c, course.RawTitle(), tt.BodySmall.C(scheme.OnSurfaceVariant))
			}
			m3.VGap(c, design.SpaceSM)
			var parts []string
			if teachers != "" {
				parts = append(parts, teachers)
			}
			if course.College() != "" {
				parts = append(parts, course.College())
			}
			parts = append(parts, course.TermLabel())
			m3.Text(c, strings.Join(parts, " · "), tt.BodyMedium.C(scheme.OnSurfaceVariant))
			m3.VGap(c, design.SpaceLG)
			ui.Row(c).Wrap().Gap(design.SpaceSM).Children(func() {
				coursePill(c, icons.PlayLessonOutlined, l10n.CourseStatLectures(lectures), scheme)
				coursePill(c, icons.DownloadDoneRounded, l10n.CourseStatLocal(downloaded), scheme)
				coursePill(c, icons.TrendingUpRounded, l10n.CourseProgress(int(course.Progress()*100+.5)), scheme)
			})
			if resume == nil {
				return
			}
			started := resume.PositionMs() > 0
			m3.VGap(c, design.SpaceLG)
			when := resume.Date()
			if when == "" {
				when = resume.ShortTitle()
			}
			label, ic := l10n.CourseStart+" · "+when, icons.PlayArrowRounded
			if started {
				label, ic = l10n.CourseContinue+" · "+when+" "+core.FmtClock(resume.PositionMs()), icons.PlayCircleRounded
			}
			ui.Row(c).Children(func() {
				l := *resume
				if (m3.Button{Kind: m3.Filled, Icon: ic, Label: label, BG: scheme.Primary, FG: scheme.OnPrimary}).Show(c).Clicked() {
					p.open(l)
				}
			})
		})
	}
	card := ui.Column(c).FillWidth().Padding(design.SpaceSM).Radius(design.CornerXLIncreased + design.SpaceXS).
		Background(design.AlphaBlend(design.WithOpacity(scheme.Primary, .04), scheme.SurfaceContainerLow))
	card.Children(func() {
		if compact {
			art(ui.Box(c).FillWidth().Height(150))
			ui.Box(c).FillWidth().Padding(design.SpaceLG, design.SpaceMD, design.SpaceMD, design.SpaceMD).Children(info)
			return
		}
		ui.Row(c).FillWidth().AlignItems(ui.Stretch).Children(func() {
			ui.Column(c).Grow(1).Basis(0).Justify(ui.Center).Padding(design.SpaceXL).Children(info)
			art(ui.Box(c).Width(300).MinHeight(180).Shrink(0))
		})
	})
}

func coursePill(c *ui.Context, ic icons.Icon, text string, s *design.Scheme) {
	ui.Row(c).AlignItems(ui.Center).PaddingX(10).PaddingY(4).Radius(design.CornerFull).Background(s.SecondaryContainer).Children(func() {
		m3.Icon(c, ic, 14, s.OnSecondaryContainer)
		m3.HGap(c, 4)
		m3.Clamp(m3.Text(c, text, design.TT().LabelMedium.C(s.OnSecondaryContainer)), 1).Shrink(1)
	})
}

func (p *CoursePage) lectureTile(c *ui.Context, l core.Lecture, ordinal int, selecting bool) {
	ui.Box(c.Key(l.ID())).FillWidth().Padding(0, 0, design.SpaceXS, 0).Children(func() { p.lectureCard(c, l, ordinal, selecting) })
}

func (p *CoursePage) lectureCard(c *ui.Context, l core.Lecture, ordinal int, selecting bool) {
	cs := design.S()
	tt := design.TT()
	var active []core.Job
	for _, k := range []string{"video", "slides", "courseware", "transcript"} {
		if j, ok := State.JobFor(l.ID(), k); ok && j.Active() {
			active = append(active, j)
		}
	}
	d := l.Begin()
	watched := 0.0
	if l.DurationS() > 0 && l.PositionMs() > 0 {
		watched = min(max(float64(l.PositionMs())/float64(l.DurationS()*1000), 0), 1)
	}
	disabled := !l.HasPlayback()
	selected := p.sel[l.ID()]
	tap := func() {
		switch {
		case selecting && l.HasPlayback():
			if selected {
				delete(p.sel, l.ID())
			} else {
				p.sel[l.ID()] = true
			}
		case !selecting && l.HasPlayback():
			p.open(l)
		}
	}
	b := m3.Tappable(c, design.CornerLG).FillWidth()
	bg, rad := cs.SurfaceContainer, design.CornerLG
	switch {
	case selected:
		bg, rad = cs.SecondaryContainer, design.CornerXL
	case b.Hovered() && !disabled:
		bg = cs.SurfaceContainerHigh
	}
	anim := design.T().Motion(design.Medium1, design.Emphasized)
	b = b.Background(bg).Radius(b.AnimateTo("r", rad, anim)).Transition(ui.ElementTransition{Duration: anim.Duration, Ease: design.Emphasized, Colors: true})
	if b.LongPressed() && l.HasPlayback() {
		p.sel[l.ID()] = true
	}
	op := float32(1)
	if disabled {
		op = .55
	}
	b.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(design.SpaceMD, design.SpaceSM, design.SpaceMD, design.SpaceMD).Opacity(op).Children(func() {
			key := "date"
			if selecting {
				key = "cb"
			}
			ui.Box(c.Key(key)).Transition(design.Switcher(design.Short4, nil, nil, ui.Motion{Scale: scaleZero})).Children(func() {
				if selecting {
					ui.Box(c).Width(52).Center().Children(func() {
						v := selected
						if m3.Checkbox(c, &v, disabled).Clicked() {
							tap()
						}
					})
				} else {
					dateBlock(c, d)
				}
			})
			m3.HGap(c, design.SpaceMD)
			ui.Column(c).Grow(1).Basis(0).Children(func() {
				m3.Clamp(m3.Text(c, l.ShortTitle(), tt.TitleSmall), 1)
				m3.VGap(c, 2)
				parts := []string{l10n.LectureOrdinal(ordinal), core.FmtLectureDate(d)}
				if l.DurationS() > 0 {
					parts = append(parts, core.FmtDuration(l.DurationS()))
				}
				if l.Room() != "" {
					parts = append(parts, l.Room())
				}
				if disabled {
					if l.Upcoming() {
						parts = append(parts, l10n.LectureNotStarted)
					} else {
						parts = append(parts, l10n.LectureNoReplay)
					}
				}
				m3.Clamp(m3.Text(c, strings.Join(parts, " · "), tt.BodySmall.C(cs.OnSurfaceVariant)), 1)
				if len(l.Assets()) > 0 || l.Segments() > 0 {
					m3.VGap(c, design.SpaceXS)
					widgets.AssetBadges(c, l.Assets(), l.Segments())
				}
				if watched > 0 {
					m3.VGap(c, design.SpaceSM)
					ui.Box(c).Width(120).Children(func() {
						m3.Linear(c, m3.Progress{Value: m3.Of(watched), Height: 3, Radius: 2})
					})
				}
				ui.Column(c).FillWidth().Clip().Transition(ui.ElementTransition{Duration: design.T().Motion(design.Medium2, nil).Duration, Ease: design.Emphasized, Height: true}).Children(func() {
					if len(active) == 0 {
						return
					}
					m3.VGap(c, design.SpaceSM)
					for _, j := range active {
						ui.Box(c.Key(j.ID())).FillWidth().Children(func() { widgets.JobProgressLine(c, j) })
					}
				})
			})
			if !selecting && !disabled {
				if m3.IconBtn(c, icons.DownloadOutlined, l10n.CommonDownload).Clicked() {
					ShowDownloadSheet([]string{l.ID()}, l.ShortTitle(), nil, nil)
				}
				if (m3.IconButton{Kind: m3.TonalIcon, Icon: icons.PlayArrowRounded, Tooltip: l10n.CommonPlay}).Show(c).Clicked() {
					tap()
				}
			}
		})
	})
	if b.Clicked() {
		tap()
	}
}

// dateBlock is _DateBlock: the lecture's month and day.
func dateBlock(c *ui.Context, d time.Time) {
	cs := design.S()
	tt := design.TT()
	ui.Column(c).Size(52, 56).Radius(design.CornerMD).Background(cs.PrimaryContainer).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
		if d.IsZero() {
			m3.Icon(c, icons.EventOutlined, 24, cs.OnPrimaryContainer)
			return
		}
		m3.Text(c, l10n.DateMonth(int(d.Month())), tt.LabelSmall.C(cs.OnPrimaryContainer))
		m3.Text(c, strconv.Itoa(d.Day()), tt.TitleLarge.C(cs.OnPrimaryContainer).H(1.1).W(600))
	})
}
