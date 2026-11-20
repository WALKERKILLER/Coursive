package app

import (
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

// CourseCard is course_card.dart's: the course's generative cover, which
// morphs on hover, its title, teachers and term, and its progress. Menu,
// when set, gives the card a context menu. It reports a tap.
type CourseCard struct {
	Course core.Course
	Menu   func() []m3.MenuItem
}

// Show builds the card in its parent's width.
func (k CourseCard) Show(c *ui.Context) bool {
	co := k.Course
	cs := design.S()
	tt := design.TT()
	s := widgets.CourseScheme(co)
	teachers := co.Teacher()
	if ts := co.Teachers(); len(ts) > 0 {
		teachers = strings.Join(ts, l10n.ListSeparator)
	}
	e := m3.Tappable(c.Key("card-"+co.ID()), design.CornerXL).FillWidth()
	hover := e.Hovered()
	bg := cs.SurfaceContainer
	radius := design.CornerXL
	if hover {
		bg = design.AlphaBlend(design.WithOpacity(s.Primary, .06), cs.SurfaceContainer)
		radius = design.CornerXLIncreased
	}
	anim := design.T().Motion(design.Medium1, design.Emphasized)
	e = e.Radius(e.AnimateTo("radius", radius, anim)).Background(bg).
		Transition(ui.ElementTransition{Duration: anim.Duration, Ease: design.Emphasized, Colors: true})
	if k.Menu != nil {
		m3.ContextMenu(c, e, k.Menu)
	}
	e.Children(func() {
		ui.Column(c).FillWidth().Children(func() {
			ui.Box(c).FillWidth().Padding(design.SpaceXS, design.SpaceXS, 0, design.SpaceXS).Children(func() {
				ui.Box(c).FillWidth().Height(108).Children(func() {
					// The tags sit over the Hero, so they stay on the card as
					// the cover flies.
					art := ui.Box(c).Fill()
					t := art.AnimateTo("morph", map[bool]float32{true: 1, false: 0}[hover], design.T().Motion(design.Long4, design.Emphasized))
					m3.Hero(c, art, courseHero(co.ID()), func(e ui.Element) {
						widgets.CourseArt(e, co, float64(t), 0, -1, design.CornerXL-design.SpaceXS)
					})
					ui.Row(c).Absolute().Top(design.SpaceSM).Right(design.SpaceSM).Children(func() {
						if co.Pinned() {
							courseTag(c, s, icons.PushPinRounded, "")
						}
						if n := co.LocalLectures(); n > 0 {
							m3.HGap(c, design.SpaceXS)
							courseTag(c, s, icons.DownloadDoneRounded, strconv.Itoa(n))
						}
					})
				})
			})
			ui.Column(c).FillWidth().Padding(design.SpaceMD, design.SpaceLG, design.SpaceLG, design.SpaceLG).Children(func() {
				m3.Clamp(m3.Text(c, co.Title(), tt.TitleMedium), 1)
				m3.VGap(c, 2)
				var line []string
				if teachers != "" {
					line = append(line, teachers)
				}
				line = append(line, co.TermLabel())
				m3.Clamp(m3.Text(c, strings.Join(line, " · "), tt.BodySmall.C(cs.OnSurfaceVariant)), 1)
				m3.VGap(c, design.SpaceMD)
				ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
					ui.Box(c).Grow(1).Basis(0).Children(func() {
						m3.Linear(c, m3.Progress{Value: m3.Of(min(max(co.Progress(), 0), 1)), Color: s.Primary,
							Track: design.WithOpacity(s.PrimaryContainer, .5), Height: 4, Radius: 4})
					})
					m3.HGap(c, design.SpaceSM)
					m3.Text(c, l10n.CourseLectureCount(co.LectureTotal()), tt.LabelSmall.C(cs.OnSurfaceVariant))
				})
			})
		})
	})
	return e.Clicked()
}

// heroTag is Hero's tag 'art-<id>', within the navigator that pushes the
// course page: a card in another tab's navigator is another Hero.
type heroTag struct {
	course string
	nav    *m3.Navigator
}

// courseHero is the tag of a course's cover's Hero, which flies between
// its card and its page as the page is pushed or popped.
func courseHero(id string) heroTag { return heroTag{id, m3.Nav()} }

// courseTag is a small pill over the cover.
func courseTag(c *ui.Context, s *design.Scheme, ic icons.Icon, text string) {
	px := float32(6)
	if text != "" {
		px = 8
	}
	ui.Row(c).AlignItems(ui.Center).PaddingX(px).PaddingY(4).Radius(design.CornerFull).
		Background(design.WithOpacity(s.Surface, .82)).Children(func() {
		m3.Icon(c, ic, 14, s.Primary)
		if text != "" {
			m3.HGap(c, 3)
			m3.Text(c, text, design.TT().LabelSmall.C(s.OnSurface))
		}
	})
}

// scaleZero is ScaleTransition's scale of 0 as a Motion: a Motion's Scale
// of 0 keeps the size, so a thousandth stands for nothing.
const scaleZero = .001
