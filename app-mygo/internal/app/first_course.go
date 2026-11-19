package app

import (
	"math"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// FirstCoursePanel is first_course.dart's: 首页 without any course, one
// calm panel that starts the first one, from videos on this device or from
// a course platform plugin.
func FirstCoursePanel(c *ui.Context, width float32) {
	cs := design.S()
	tt := design.TT()
	wide := width >= 640
	box := ui.Box(c).FillWidth().Radius(design.CornerXL).Clip()
	enter := entered(c, box, "enter", design.Long4*3/2)
	hover := box.Hovered()
	pad := design.SpaceXL
	if wide {
		pad = design.SpaceXXL + design.SpaceSM
	}
	// A faint wash of the primary tone behind the shapes.
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		center := ui.Point{X: .5, Y: .225}
		rad := 1.1 * min(r.W, r.H)
		if wide {
			center, rad = ui.Point{X: .86, Y: .45}, .9*min(r.W, r.H)
		}
		p.FillRadialGradient(r, ui.RadialGradient{Center: center, Radius: rad, Stops: []ui.GradientStop{
			{Color: design.WithOpacity(cs.PrimaryContainer, .55), At: 0}, {Color: cs.SurfaceContainerLow, At: 1},
		}}, design.CornerXL)
	})
	text := func() {
		align := ui.Start
		ta := ui.Start
		if !wide {
			align, ta = ui.Center, ui.Center
		}
		ui.Column(c).AlignItems(align).Children(func() {
			st := tt.HeadlineSmall
			if wide {
				st = tt.HeadlineMedium
			}
			m3.Text(c, l10n.HomeFirstCourseTitle, st.W(500)).TextAlign(ta)
			m3.VGap(c, design.SpaceMD)
			m3.Text(c, l10n.HomeFirstCourseBody, tt.BodyLarge.C(cs.OnSurfaceVariant).H(1.6)).TextAlign(ta).MaxWidth(440)
			m3.VGap(c, design.SpaceXL)
			ui.Row(c).Wrap().Gap(design.SpaceMD).Justify(align).Children(func() {
				if (m3.Button{Kind: m3.Filled, Icon: icons.AddRounded, Label: l10n.LocalCourseCreate, Height: 48}).Show(c).Clicked() {
					StartLocalCourse()
				}
				if (m3.Button{Kind: m3.Outlined, Icon: icons.SchoolOutlined, Label: l10n.HomeFirstCourseConnect, Height: 48}).Show(c).Clicked() {
					OpenPlugins(false)
				}
			})
			m3.VGap(c, design.SpaceLG)
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				m3.Icon(c, icons.LockOutlineRounded, 16, cs.OnSurfaceVariant)
				m3.HGap(c, design.SpaceXS)
				m3.Text(c, l10n.HomeFirstCourseLocal, tt.LabelMedium.C(cs.OnSurfaceVariant)).Shrink(1)
			})
		})
	}
	box.Children(func() {
		size := float32(176)
		if wide {
			size = 232
		}
		if wide {
			ui.Row(c).FillWidth().Padding(pad).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Grow(1).Basis(0).Children(text)
				m3.HGap(c, design.SpaceXXL)
				constellation(c, enter, hover, size)
			})
			return
		}
		ui.Column(c).FillWidth().Padding(pad).AlignItems(ui.Center).Children(func() {
			constellation(c, enter, hover, size)
			m3.VGap(c, design.SpaceXL)
			text()
		})
	})
}

// constellation is three organic tiles for what a course holds: the
// recording, its transcript and the notes. They arrive one after another
// and lean in on hover.
func constellation(c *ui.Context, enter float32, lifted bool, s float32) {
	cs := design.S()
	type tile struct {
		shape     widgets.OrganicShape
		fill, ink ui.Color
		icon      icons.Icon
		x, y      float32
		extent    float32
		turn      float32
	}
	tiles := []tile{
		{widgets.ShapeCookie9, cs.PrimaryContainer, cs.OnPrimaryContainer, icons.PlayArrowRounded, .4, .52, .6, -.04},
		{widgets.ShapeSunny, cs.TertiaryContainer, cs.OnTertiaryContainer, icons.SubtitlesOutlined, .8, .24, .36, .05},
		{widgets.ShapePebble, cs.SecondaryContainer, cs.OnSecondaryContainer, icons.EditNoteRounded, .78, .8, .32, -.06},
	}
	type dot struct {
		x, y, r float32
		color   ui.Color
	}
	dots := []dot{{.1, .16, .06, cs.Primary}, {.12, .88, .04, cs.Tertiary}, {.97, .53, .035, cs.Secondary}}
	arrive := func(i int) float32 {
		return interval(enter, float32(i)*.14, float32(math.Min(1, .55+float64(i)*.14)), design.EmphasizedDecelerate)
	}
	ui.Box(c).Size(s, s).Shrink(0).Children(func() {
		for i, d := range dots {
			a := arrive(i + 1)
			ui.Box(c).Absolute().Left(d.x*s-d.r*s/2).Top(d.y*s-d.r*s/2).Size(d.r*s, d.r*s).
				Radius(design.CornerFull).Background(design.WithOpacity(d.color, .5)).Opacity(a).Scale(.6+.4*a, .6+.4*a)
		}
		for i, t := range tiles {
			a := arrive(i)
			turn := float32(0)
			if lifted {
				turn = t.turn
			}
			e := ui.Box(c).Absolute().Left((t.x-t.extent/2)*s).Top((t.y-t.extent/2)*s).Size(t.extent*s, t.extent*s).Center().
				Opacity(a).Scale(.6+.4*a, .6+.4*a)
			e = e.Rotate(e.AnimateTo("turn", turn*360, design.T().Motion(design.Long2, design.Emphasized)))
			shape, fill := t.shape, t.fill
			e.Draw(func(p *ui.Painter, r ui.Rect) {
				path := widgets.ShapePath(widgets.ShapeRadii(shape), r, 0)
				p.FillPath(&path, fill)
			}).Children(func() { m3.Icon(c, t.icon, t.extent*s*.42, t.ink) })
		}
	})
}
