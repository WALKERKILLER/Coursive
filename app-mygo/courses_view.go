package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/harukizmoe/mayu"
	"github.com/harukizmoe/mayu/components"
	"github.com/harukizmoe/mayu/theme"
)

// The course library, laid out as the Flutter client lays it out
// (app/lib/features/courses/courses_page.dart): a display heading, a search
// bar, a row of term chips, then a grid of covers.

// cardCoverHeight is the cover area inside a card, before its 4 dp inset.
const cardCoverHeight float32 = 108

// libraryView builds the page.
func (a *app) libraryView(c *ui.Context, t *theme.Theme) {
	width, _ := c.Size()
	compact := sizeClassOf(width) == SizeCompact
	pad := Page
	top := SpaceXXL
	if compact {
		pad = PageCompact
		top = SpaceXL
	}
	contentW := width - 2*pad
	if contentW > MaxContentWidth {
		contentW = MaxContentWidth
	}

	ui.Scroll(c).Fill().Background(t.Scheme.Surface).Children(func() {
		ui.Row(c).Fill().Justify(ui.Center).Children(func() {
			ui.Column(c).Width(contentW).Children(func() {
				ui.Column(c).Padding(top, pad, SpaceMD, pad).Gap(SpaceLG).Children(func() {
					a.heading(c, t, compact)
					if len(a.courses) > 0 {
						a.filters(c, t)
					}
				})
				a.results(c, t, pad, contentW)
				ui.Box(c).Height(96)
			})
		})
	})
}

func (a *app) heading(c *ui.Context, t *theme.Theme, compact bool) {
	ui.Row(c).AlignItems(ui.Center).Gap(SpaceSM).Children(func() {
		style := TypeDisplaySmall
		if compact {
			style = TypeHeadlineMedium
		}
		textStyle(ui.Text(c, "课程库"), style).Grow(1)
		if ui.Button(c, "来源").Clicked() {
			a.section = 2
		}
		if ui.Button(c, "同步").Disabled(a.loading).Clicked() {
			a.loadCourses()
		}
	})
}

func (a *app) filters(c *ui.Context, t *theme.Theme) {
	components.NewSearchBar(c, &a.query, "搜索课程", t)
	ui.Row(c).Gap(SpaceSM).Children(func() {
		for _, term := range a.terms() {
			term := term
			label := term
			if term == "local" {
				label = "本地课程"
			}
			mayu.Chip(c, label).Selected(a.term == term).OnClick(func() { a.term = term })
		}
		mayu.Chip(c, "全部学期").Selected(a.term == "").OnClick(func() { a.term = "" })
	})
}

// terms is every term in the library, newest first, as the Flutter page sorts
// them.
func (a *app) terms() []string {
	seen := map[string]bool{}
	var out []string
	for _, course := range a.courses {
		if course.Term == "" || seen[course.Term] {
			continue
		}
		seen[course.Term] = true
		out = append(out, course.Term)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(out)))
	return out
}

// selectedTerm is the term the page filters by: the newest unless the user
// picked another, and "" for all of them.
func (a *app) selectedTerm() string {
	if a.term != "" {
		if a.term == "all" {
			return ""
		}
		return a.term
	}
	if terms := a.terms(); len(terms) > 0 {
		return terms[0]
	}
	return ""
}

func (a *app) results(c *ui.Context, t *theme.Theme, pad, contentW float32) {
	switch a.state {
	case stateFailed:
		ui.Column(c).Padding(SpaceLG, pad, SpaceLG, pad).Gap(SpaceSM).Children(func() {
			textStyle(ui.Text(c, "本地服务没有启动。"), TypeBodyMedium).TextColor(t.Scheme.Error)
			textStyle(ui.Textf(c, "%v", a.daemonErr), TypeBodySmall).TextColor(t.Scheme.OnSurfaceVariant)
			if ui.Button(c, "重试").Clicked() {
				a.d.start()
			}
		})
		return
	case stateReady:
	default:
		ui.Column(c).Padding(SpaceLG, pad, SpaceLG, pad).Children(func() {
			textStyle(ui.Text(c, "正在启动本地服务…"), TypeBodyMedium).TextColor(t.Scheme.OnSurfaceVariant)
		})
		return
	}

	if a.loading {
		textStyle(ui.Text(c, "正在读取课程…"), TypeBodyMedium).TextColor(t.Scheme.OnSurfaceVariant)
		return
	}
	if a.fetchErr != nil {
		textStyle(ui.Textf(c, "读取课程失败：%v", a.fetchErr), TypeBodyMedium).TextColor(t.Scheme.Error)
		return
	}

	shown := a.filtered()
	if len(shown) == 0 {
		ui.Column(c).Padding(SpaceXL, pad, SpaceXL, pad).Children(func() {
			textStyle(ui.Text(c, "这里没有课程。"), TypeBodyMedium).TextColor(t.Scheme.OnSurfaceVariant)
		})
		return
	}

	cols := int(contentW / 280)
	if cols < 1 {
		cols = 1
	}
	if cols > 4 {
		cols = 4
	}
	cardW := (contentW - SpaceMD*float32(cols-1)) / float32(cols)

	ui.Column(c).Padding(0, pad, 0, pad).Gap(SpaceMD).Children(func() {
		for start := 0; start < len(shown); start += cols {
			end := start + cols
			if end > len(shown) {
				end = len(shown)
			}
			ui.Row(c).Gap(SpaceMD).Children(func() {
				for _, course := range shown[start:end] {
					a.courseCard(c, t, course, cardW)
				}
			})
		}
	})
}

func (a *app) filtered() []Course {
	term := a.selectedTerm()
	q := strings.TrimSpace(strings.ToLower(a.query))
	out := make([]Course, 0, len(a.courses))
	for _, c := range a.courses {
		if term != "" && c.Term != term {
			continue
		}
		if q != "" {
			hay := strings.ToLower(c.Title + " " + c.RawTitle + " " + c.Teacher)
			if !strings.Contains(hay, q) {
				continue
			}
		}
		out = append(out, c)
	}
	return out
}

// courseCard draws one course as the Flutter client's CourseCard does: a
// generative cover over the title, the teachers and term, and a progress line.
func (a *app) courseCard(c *ui.Context, t *theme.Theme, course Course, width float32) {
	scheme := courseScheme(course, a.dark)
	key := course.ID
	if key == "" {
		key = course.RawTitle
	}
	spec := courseArtSpec(key)

	ui.Box(c).Width(width).Radius(CornerXL).Background(t.Scheme.SurfaceContainer).Children(func() {
		ui.Column(c).Children(func() {
			ui.Box(c).Padding(SpaceXS, SpaceXS, 0, SpaceXS).Children(func() {
				ui.Box(c).Height(cardCoverHeight).Radius(CornerXL - SpaceXS).Clip().
					Draw(func(p *ui.Painter, r ui.Rect) {
						drawCourseArt(p, r, spec, scheme, 0)
					})
			})
			ui.Box(c).Padding(SpaceMD, SpaceLG, SpaceLG, SpaceLG).Children(func() {
				ui.Column(c).Gap(0).Children(func() {
					textStyle(ui.Text(c, course.Title), TypeTitleMedium).SingleLine()
					ui.Box(c).Height(2)
					meta := []string{}
					if course.Teacher != "" {
						meta = append(meta, course.Teacher)
					}
					if course.Term != "" {
						term := course.Term
						if term == "local" {
							term = "本地课程"
						}
						meta = append(meta, term)
					}
					textStyle(ui.Text(c, strings.Join(meta, " · ")), TypeBodySmall).
						TextColor(t.Scheme.OnSurfaceVariant).SingleLine()
					ui.Box(c).Height(SpaceMD)
					ui.Row(c).AlignItems(ui.Center).Gap(SpaceSM).Children(func() {
						a.progress(c, scheme, course.Progress)
						textStyle(ui.Textf(c, "%d 讲", course.LectureTotal), TypeLabelSmall).
							TextColor(t.Scheme.OnSurfaceVariant)
					})
				})
			})
		})
	})
}

// progress is the 4 dp line the card carries, filled to the course's progress.
func (a *app) progress(c *ui.Context, scheme theme.ColorScheme, value float64) {
	if value < 0 {
		value = 0
	}
	if value > 1 {
		value = 1
	}
	ui.Box(c).Height(4).Grow(1).Draw(func(p *ui.Painter, r ui.Rect) {
		p.Fill(r, withAlpha(scheme.PrimaryContainer, 0.5), 2)
		if value > 0 {
			p.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W * float32(value), H: r.H}, scheme.Primary, 2)
		}
	})
}

// courseCount is a small helper the tests read.
func courseCount(courses []Course) string { return fmt.Sprint(len(courses)) }
