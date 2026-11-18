package app

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// CoursesPage is courses_page.dart's: the course grid with search, the
// term chips and the hidden courses.
type CoursesPage struct {
	query string
	// term is nil for the latest term, "" for all.
	term       *string
	showHidden bool
	scroll     ui.ScrollState
	terms      ui.ScrollState
	refresh    ui.RefreshState
}

// NewCoursesPage shows the latest term.
func NewCoursesPage() m3.Page { return &CoursesPage{} }

// MainScroll is the page's scroll, which shrinks the dock.
func (p *CoursesPage) MainScroll() *ui.ScrollState { return &p.scroll }

func (p *CoursesPage) create() {
	StartLocalCourse(func(string) {
		local := "local"
		p.term = &local
	})
}

func openSources() { m3.Nav().Push(NewSourcesPage()) }

// View builds the page.
func (p *CoursesPage) View(c *ui.Context) {
	cs := design.S()
	tt := design.TT()
	compact := Compact()
	pad := design.SpaceXL
	if compact {
		pad = design.SpaceLG
	}
	var terms []string
	for _, co := range State.Courses {
		if !slices.Contains(terms, co.Term()) {
			terms = append(terms, co.Term())
		}
	}
	slices.SortFunc(terms, func(a, b string) int { return strings.Compare(b, a) })
	term := ""
	if p.term != nil {
		term = *p.term
	} else {
		for _, t := range terms {
			if t != "local" {
				term = t
				break
			}
		}
		if term == "" && len(terms) > 0 {
			term = terms[0]
		}
	}
	q := strings.ToLower(strings.TrimSpace(p.query))
	var list []core.Course
	hidden := 0
	for _, co := range State.Courses {
		if co.Hidden() && (term == "" || co.Term() == term) {
			hidden++
		}
		if term != "" && co.Term() != term {
			continue
		}
		if co.Hidden() && !p.showHidden {
			continue
		}
		if q != "" {
			match := strings.Contains(strings.ToLower(co.Title()), q) || strings.Contains(strings.ToLower(co.RawTitle()), q) ||
				strings.Contains(co.Teacher(), q)
			for _, t := range co.Teachers() {
				match = match || strings.Contains(t, q)
			}
			if !match {
				continue
			}
		}
		list = append(list, co)
	}
	slices.SortStableFunc(list, func(a, b core.Course) int {
		if a.Pinned() != b.Pinned() {
			if a.Pinned() {
				return -1
			}
			return 1
		}
		return strings.Compare(b.LastLearnAt(), a.LastLearnAt())
	})
	none := len(State.Courses) == 0
	setTerm := func(t string) { p.term = &t }

	body := func() {
		sc := ui.Scroll(c).Fill().TrackScroll(&p.scroll)
		if sc.PullToRefresh(&p.refresh) {
			State.LoadCoursesThen(State.AnySignedIn(), false, func() { p.refresh.Refreshing = false })
		}
		sc.Children(func() {
			ui.Column(c).FillWidth().MinHeight(WindowHeight - 100).Children(func() {
				widgets.ContentWidth(c, 0, func() {
					top := design.SpaceXXL
					if compact {
						top = design.SpaceXL
					}
					ui.Column(c).FillWidth().Padding(top, pad, design.SpaceMD, pad).Children(func() {
						ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
							st := tt.DisplaySmall
							if compact {
								st = tt.HeadlineMedium
							}
							m3.Text(c, l10n.CommonCourses, st).Grow(1).Basis(0)
							if m3.IconBtn(c, icons.ExtensionOutlined, l10n.SourcesTitle).Clicked() {
								openSources()
							}
							if State.AnySignedIn() {
								b := m3.IconButton{Icon: icons.SyncRounded, Tooltip: l10n.CoursesSync, Disabled: State.CoursesLoading}
								if State.CoursesLoading {
									b.Child = func(fg ui.Color) { m3.Spinner(c, 20, 2.5, cs.Primary) }
								}
								if b.Show(c).Clicked() {
									State.LoadCourses(true, false)
								}
							}
						})
						if none {
							return
						}
						m3.VGap(c, design.SpaceLG)
						m3.SearchBar{Hint: l10n.CoursesSearchHint, BG: cs.SurfaceContainerHigh, MaxWidth: -1,
							Leading: func() { m3.Icon(c, icons.SearchRounded, 24, cs.OnSurface) },
							Trailing: func() {
								if p.query != "" && m3.IconBtn(c, icons.CloseRounded, "").Clicked() {
									p.query = ""
								}
							}}.Show(c, &p.query)
						m3.VGap(c, design.SpaceMD)
						widgets.HScroller{State: &p.terms, Fade: true}.Show(c, func() {
							ui.Row(c).Gap(design.SpaceSM).Children(func() {
								for _, t := range terms {
									label := core.Course{J: core.J{"term": t}}.TermLabel()
									if t == "local" {
										label = l10n.LocalCourse
									}
									if m3.ChoiceChipOf(c.Key(t), label, term == t) {
										setTerm(t)
									}
								}
								if m3.ChoiceChipOf(c.Key("all"), l10n.CoursesAllTerms, term == "") {
									setTerm("")
								}
								if hidden > 0 && m3.FilterChipOf(c.Key("hidden"), l10n.CoursesHiddenCount(hidden), p.showHidden) {
									p.showHidden = !p.showHidden
								}
							})
						})
					})
				})
				fill := func(child func()) { ui.Column(c).FillWidth().Grow(1).MinHeight(360).Children(child) }
				switch {
				case State.CoursesLoading && none:
					fill(func() { widgets.CenteredLoader(c, l10n.CoursesSyncing) })
				case State.CoursesError != nil && none:
					fill(func() { widgets.ErrorState(c, State.CoursesError, func() { State.LoadCourses(true, false) }) })
				case none:
					fill(func() {
						widgets.EmptyState{Icon: icons.VideoLibraryOutlined, Title: l10n.CoursesEmptyTitle, Body: l10n.CoursesEmptyBody, Action: func() {
							ui.Column(c).AlignItems(ui.Center).Children(func() {
								if (m3.Button{Kind: m3.Filled, Icon: icons.AddRounded, Label: l10n.LocalCourseCreate, Disabled: !State.Connected()}).Show(c).Clicked() {
									p.create()
								}
								m3.VGap(c, design.SpaceSM)
								if m3.TextButton(c, l10n.CoursesEmptySources).Clicked() {
									openSources()
								}
							})
						}}.Show(c)
					})
				case len(list) == 0:
					fill(func() {
						switch {
						case q != "":
							widgets.EmptyState{Icon: icons.SearchOffRounded, Title: l10n.CoursesNoMatch(p.query), Action: func() {
								if m3.TextButton(c, l10n.CoursesClearSearch).Clicked() {
									p.query = ""
								}
							}}.Show(c)
						case term != "":
							widgets.EmptyState{Icon: icons.EventBusyOutlined, Title: l10n.CoursesEmptyTerm, Action: func() {
								if m3.TextButton(c, l10n.CoursesViewAllTerms).Clicked() {
									setTerm("")
								}
							}}.Show(c)
						default:
							widgets.EmptyState{Icon: icons.VisibilityOffOutlined, Title: l10n.CoursesAllHidden, Action: func() {
								if m3.TextButton(c, l10n.CoursesShowHidden).Clicked() {
									p.showHidden = true
								}
							}}.Show(c)
						}
					})
				default:
					widgets.ContentWidth(c, 0, func() {
						g := ui.Grid(c).FillWidth().Gap(design.SpaceMD).Padding(design.SpaceSM, pad, 96+BottomInset, pad)
						cols := min(max(int(math.Floor(float64((widgets.Width(g, 900)-2*pad)/280))), 1), 4)
						g.Columns(cols).Children(func() {
							for _, co := range list {
								op := float32(1)
								if co.Hidden() {
									op = .55
								}
								cell := ui.Box(c.Key(co.ID())).FillWidth()
								cell = cell.Opacity(cell.AnimateTo("op", op, design.T().Motion(design.Medium1, nil)))
								cell.Children(func() {
									if (CourseCard{Course: co, Menu: func() []m3.MenuItem { return courseMenu(co) }}).Show(c) {
										m3.Nav().Push(NewCoursePage(co.ID()))
									}
								})
							}
						})
					})
				}
			})
		})
	}
	m3.Scaffold{
		BG:       ui.Transparent,
		FABInset: BottomInset,
		FAB: func() {
			// The empty state carries the create action until a course exists.
			if none {
				return
			}
			if (m3.FAB{Icon: icons.AddRounded, Label: l10n.LocalCourseCreate, Disabled: !State.Connected()}).Show(c).Clicked() {
				p.create()
			}
		},
		Body: body,
	}.Show(c)
}

// courseMenu is showCourseMenu's items: pin, rename, colour and hide.
func courseMenu(co core.Course) []m3.MenuItem {
	pin, pinIcon := l10n.CoursesPin, icons.PushPinRounded
	if co.Pinned() {
		pin, pinIcon = l10n.CoursesUnpin, icons.PushPinOutlined
	}
	hide, hideIcon := l10n.CommonHide, icons.VisibilityOffOutlined
	if co.Hidden() {
		hide, hideIcon = l10n.CoursesUnhide, icons.VisibilityOutlined
	}
	patch := func(j core.J) { State.PatchCourse(co.ID(), j, nil) }
	return []m3.MenuItem{
		{Icon: pinIcon, Label: pin, OnTap: func() { patch(core.J{"pinned": !co.Pinned()}) }},
		{Icon: icons.EditOutlined, Label: l10n.CommonRename, OnTap: func() {
			renameCourse(co, func(name string) { patch(core.J{"alias": name}) })
		}},
		{Icon: icons.PaletteOutlined, Label: l10n.CoursesColor, OnTap: func() {
			courseColor(co, func(color string) { patch(core.J{"color": color}) })
		}},
		{Icon: hideIcon, Label: hide, OnTap: func() { patch(core.J{"hidden": !co.Hidden()}) }},
	}
}

func renameCourse(co core.Course, done func(string)) {
	name := co.Alias()
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		m3.AlertDialog(c, m3.Alert{Icon: icons.EditOutlined, Title: l10n.CoursesRenameTitle, Content: func() {
			ui.Column(c).FillWidth().Children(func() {
				m3.Text(c, l10n.CoursesOriginalName(co.RawTitle()), design.TT().BodySmall)
				m3.VGap(c, design.SpaceMD)
				in := m3.Field{Label: l10n.CoursesDisplayName, Helper: l10n.CoursesDisplayNameHelper, AutoFocus: true}.Show(c, &name)
				if in.Submitted() {
					r.Pop(strings.TrimSpace(name))
				}
			})
		}, Actions: func() {
			if m3.TextButton(c, l10n.CommonCancel).Clicked() {
				r.Close()
			}
			if m3.FilledButton(c, l10n.CommonSave).Clicked() {
				r.Pop(strings.TrimSpace(name))
			}
		}})
	}, func(v any) {
		if s, ok := v.(string); ok {
			done(s)
		}
	})
}

func courseColor(co core.Course, done func(string)) {
	hex := func(v uint32) string { return fmt.Sprintf("#%06X", v&0xFFFFFF) }
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		m3.AlertDialog(c, m3.Alert{Title: l10n.CoursesColor, Content: func() {
			ui.Row(c).Wrap().Gap(design.SpaceMD).MaxWidth(320).Children(func() {
				if courseSwatch(c, "auto", nil, co.Color() == "") {
					r.Pop("")
				}
				for _, sc := range design.SeedPalette {
					col := design.ARGB(sc.Seed)
					if courseSwatch(c, sc.Label, &col, strings.EqualFold(co.Color(), hex(sc.Seed))) {
						r.Pop(hex(sc.Seed))
					}
				}
			})
		}, Actions: func() {
			if m3.TextButton(c, l10n.CommonCancel).Clicked() {
				r.Close()
			}
		}})
	}, func(v any) {
		if s, ok := v.(string); ok {
			done(s)
		}
	})
}

func courseSwatch(c *ui.Context, key string, color *ui.Color, selected bool) bool {
	cs := design.S()
	bg := cs.SurfaceContainerHighest
	if color != nil {
		bg = *color
	}
	rad := float32(22)
	if selected {
		rad = design.CornerMD
	}
	b := ui.ButtonBase(c.Key(key)).FocusRing(false).Size(44, 44).Center().Cursor(ui.CursorPointer).Background(bg).Label(key)
	b = b.Radius(b.AnimateTo("r", rad, design.T().Motion(design.Short4, nil)))
	border := ui.Transparent
	if selected {
		border = cs.OnSurface
	}
	b = b.Border(2, border)
	m3.Ink(b, cs.OnSurface, 22)
	b.Children(func() {
		switch {
		case color == nil:
			m3.Icon(c, icons.AutoAwesomeOutlined, 18, cs.OnSurfaceVariant)
		case selected:
			m3.Icon(c, icons.CheckRounded, 24, ui.RGB(255, 255, 255))
		}
	})
	return b.Clicked()
}

type downloadKind struct {
	kind, label, hint string
	icon              icons.Icon
}

func downloadKinds() []downloadKind {
	return []downloadKind{
		{"video", l10n.AssetVideo, l10n.DownloadVideoHint, icons.MovieOutlined},
		{"slides", l10n.AssetSlides, l10n.DownloadSlidesHint, icons.SlideshowOutlined},
		{"transcript", l10n.AssetAsr, l10n.DownloadAsrHint, icons.SubjectRounded},
	}
}

// ShowDownloadSheet is showDownloadSheet: what to fetch for one or more
// lectures; kinds preselects the options, else the last choice.
func ShowDownloadSheet(lectureIDs []string, title string, kinds []string, done func()) {
	var suggested []string
	for _, k := range kinds {
		for _, dk := range downloadKinds() {
			if dk.kind == k {
				suggested = append(suggested, k)
			}
		}
	}
	saved := suggested
	if len(saved) == 0 {
		if v, ok := core.Prefs.Strings("download.kinds"); ok {
			saved = v
		} else {
			saved = []string{"video", "slides"}
		}
	}
	api := State.Api()
	core.Async(func() (bool, error) {
		st, err := api.Get("/api/settings")
		return core.Obj(st)["transcript.delete_video"] == true, err
	}, func(deleteVideo bool, _ error) {
		sel := map[string]bool{}
		for _, k := range saved {
			sel[k] = true
		}
		m3.ShowModalBottomSheet(m3.SheetOptions{ScrollControlled: true}, func(c *ui.Context, r *m3.Route) {
			downloadSheet(c, r, len(lectureIDs), title, sel, deleteVideo)
		}).OnClose = func(v any) {
			picked, _ := v.([]string)
			if len(picked) == 0 {
				if done != nil {
					done()
				}
				return
			}
			core.Prefs.SetStrings("download.kinds", picked)
			State.Enqueue(lectureIDs, picked, func(jobs []core.Job, err error) {
				if err != nil {
					widgets.ShowSnack(l10n.DownloadQueueFailed(err), "", nil)
				} else {
					widgets.ShowSnack(l10n.DownloadQueued(len(jobs)), "", nil)
				}
				if done != nil {
					done()
				}
			})
		}
	})
}

func downloadSheet(c *ui.Context, r *m3.Route, count int, title string, sel map[string]bool, deleteVideo bool) {
	cs := design.S()
	tt := design.TT()
	ffmpeg := State.Health["ffmpeg"] == true
	// Options that cannot run show as off and are not sent.
	var picked []string
	for _, k := range downloadKinds() {
		if sel[k.kind] && (k.kind != "transcript" || ffmpeg) {
			picked = append(picked, k.kind)
		}
	}
	ui.Column(c).FillWidth().MaxWidth(560).Padding(0, design.SpaceXL, design.SpaceLG, design.SpaceXL).Children(func() {
		head := l10n.DownloadLectures(count)
		if count == 1 {
			head = title
			if head == "" {
				head = l10n.DownloadThisLecture
			}
		}
		m3.Text(c, head, tt.HeadlineSmall)
		m3.VGap(c, design.SpaceXS)
		m3.Text(c, l10n.DownloadSavedToLibrary, tt.BodyMedium.C(cs.OnSurfaceVariant))
		m3.VGap(c, design.SpaceLG)
		for _, k := range downloadKinds() {
			hint := k.hint
			enabled := k.kind != "transcript" || ffmpeg
			if !enabled {
				hint = l10n.DownloadNeedsFfmpeg
			}
			if downloadOption(c, k, hint, sel[k.kind], enabled) {
				sel[k.kind] = !sel[k.kind]
			}
			m3.VGap(c, design.SpaceXS)
		}
		ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(design.SpaceXS, design.SpaceXS, 0, design.SpaceXS).Children(func() {
			m3.Icon(c, icons.InfoOutlineRounded, 18, cs.OnSurfaceVariant)
			m3.HGap(c, design.SpaceSM)
			m3.Text(c, l10n.DownloadNoCourseware, tt.BodySmall.C(cs.OnSurfaceVariant)).Grow(1).Basis(0)
		})
		ui.Box(c).FillWidth().Clip().Transition(ui.ElementTransition{Duration: design.T().Motion(design.Medium2, nil).Duration, Ease: design.Emphasized, Height: true}).Children(func() {
			if !(deleteVideo && sel["transcript"] && ffmpeg) {
				return
			}
			ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(design.SpaceSM, 0, 0, 0).Children(func() {
				m3.Icon(c, icons.AutoDeleteOutlined, 18, cs.Tertiary)
				m3.HGap(c, design.SpaceSM)
				msg := l10n.DownloadDeleteLocalVideoAfterAsr
				if sel["video"] {
					msg = l10n.DownloadDeleteVideoAfterAsr
				}
				m3.Text(c, msg, tt.BodySmall.C(cs.OnSurfaceVariant)).Grow(1).Basis(0)
			})
		})
		m3.VGap(c, design.SpaceLG)
		label := l10n.DownloadStart
		if len(picked) == 0 {
			label = l10n.DownloadChooseContent
		}
		b := (m3.Button{Kind: m3.Filled, Icon: icons.DownloadRounded, Label: label, Disabled: len(picked) == 0, Height: 52}).Show(c).FillWidth()
		if b.Clicked() {
			r.Pop(picked)
		}
	})
}

func downloadOption(c *ui.Context, k downloadKind, hint string, selected, enabled bool) bool {
	cs := design.S()
	tt := design.TT()
	on := selected && enabled
	bg, rad := cs.SurfaceContainerHigh, design.CornerLG
	if on {
		bg, rad = cs.SecondaryContainer, design.CornerXL
	}
	b := m3.Tappable(c.Key(k.kind), design.CornerLG).FillWidth().Disabled(!enabled)
	anim := design.T().Motion(design.Medium1, design.Emphasized)
	b = b.Background(bg).Radius(b.AnimateTo("r", rad, anim)).Transition(ui.ElementTransition{Duration: anim.Duration, Ease: design.Emphasized, Colors: true})
	op := float32(1)
	if !enabled {
		op = .5
	}
	b.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(design.SpaceMD, design.SpaceLG).Opacity(op).Children(func() {
			ic, title, sub := cs.OnSurfaceVariant, cs.OnSurface, cs.OnSurfaceVariant
			if on {
				ic, title, sub = cs.OnSecondaryContainer, cs.OnSecondaryContainer, design.WithOpacity(cs.OnSecondaryContainer, .8)
			}
			m3.Icon(c, k.icon, 24, ic)
			m3.HGap(c, design.SpaceLG)
			ui.Column(c).Grow(1).Basis(0).Children(func() {
				m3.Text(c, k.label, tt.TitleSmall.C(title))
				m3.Text(c, hint, tt.BodySmall.C(sub))
			})
			key := "off"
			if on {
				key = "on"
			}
			ui.Box(c.Key(key)).Transition(ui.ElementTransition{Duration: design.Short4, Enter: &ui.Motion{Scale: 0}}).Children(func() {
				if on {
					m3.Icon(c, icons.CheckCircleRounded, 24, cs.Primary)
				} else {
					m3.Icon(c, icons.CircleOutlined, 24, cs.Outline)
				}
			})
		})
	})
	return b.Clicked() && enabled
}
