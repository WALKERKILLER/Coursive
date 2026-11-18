package widgets

import (
	"encoding/base64"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
)

// PluginIcons are the Material icons a manifest may name; anything else
// shows the puzzle piece.
var PluginIcons = map[string]icons.Icon{
	"extension":         icons.ExtensionOutlined,
	"psychology_alt":    icons.PsychologyAltOutlined,
	"account_tree":      icons.AccountTreeOutlined,
	"hub":               icons.HubOutlined,
	"schema":            icons.SchemaOutlined,
	"insights":          icons.InsightsOutlined,
	"show_chart":        icons.ShowChartRounded,
	"functions":         icons.FunctionsRounded,
	"calculate":         icons.CalculateOutlined,
	"style":             icons.StyleOutlined,
	"layers":            icons.LayersOutlined,
	"quiz":              icons.QuizOutlined,
	"school":            icons.SchoolOutlined,
	"menu_book":         icons.MenuBookOutlined,
	"auto_stories":      icons.AutoStoriesOutlined,
	"edit_note":         icons.EditNoteRounded,
	"sticky_note_2":     icons.StickyNote2Outlined,
	"checklist":         icons.ChecklistRounded,
	"task_alt":          icons.TaskAltRounded,
	"assignment":        icons.AssignmentOutlined,
	"code":              icons.CodeRounded,
	"terminal":          icons.TerminalRounded,
	"data_object":       icons.DataObjectRounded,
	"bug_report":        icons.BugReportOutlined,
	"smart_toy":         icons.SmartToyOutlined,
	"event":             icons.EventOutlined,
	"calendar_month":    icons.CalendarMonthOutlined,
	"sync":              icons.SyncRounded,
	"cloud_sync":        icons.CloudSyncOutlined,
	"link":              icons.LinkRounded,
	"output":            icons.OutputRounded,
	"download":          icons.DownloadOutlined,
	"image":             icons.ImageOutlined,
	"palette":           icons.PaletteOutlined,
	"lightbulb":         icons.LightbulbOutlineRounded,
	"translate":         icons.TranslateRounded,
	"record_voice_over": icons.RecordVoiceOverOutlined,
	"timeline":          icons.TimelineRounded,
	"bubble_chart":      icons.BubbleChartOutlined,
	"science":           icons.ScienceOutlined,
}

// PluginIcon is pluginIcon: the icon a manifest names.
func PluginIcon(name string) icons.Icon {
	if ic, ok := PluginIcons[name]; ok {
		return ic
	}
	return icons.ExtensionOutlined
}

var (
	dataImages   = map[string]ui.ImageSource{}
	dataImagesMu sync.Mutex
)

// DataImage decodes a data:image/…;base64, URL, a bitmap or an SVG; nil
// for anything else. Decoded images are kept, so rebuilding does not decode
// again.
func DataImage(url string) ui.ImageSource {
	if !strings.HasPrefix(url, "data:image/") {
		return nil
	}
	dataImagesMu.Lock()
	defer dataImagesMu.Unlock()
	if img, ok := dataImages[url]; ok {
		return img
	}
	comma := strings.IndexByte(url, ',')
	if comma < 0 || !strings.HasSuffix(url[:comma], ";base64") {
		return nil
	}
	data, err := base64.StdEncoding.DecodeString(url[comma+1:])
	if err != nil {
		return nil
	}
	var img ui.ImageSource
	if strings.HasPrefix(url, "data:image/svg+xml") {
		if s, err := ui.ParseSVG(data); err == nil {
			img = s
		}
	} else if b, err := ui.DecodeBitmap(data); err == nil {
		img = b
	}
	if img == nil {
		return nil
	}
	if len(dataImages) >= 64 {
		for k := range dataImages {
			delete(dataImages, k)
			break
		}
	}
	dataImages[url] = img
	return img
}

// PluginGlyph is plugins_page.dart's: a plugin's icon, its logo on white
// or its symbol on a container, a rounded square or, from 64 DIPs, a
// scalloped star. Color and OnColor replace the container colours picked
// from the id.
type PluginGlyph struct {
	Icon, ID       string
	Size           float32
	Muted          bool
	Color, OnColor ui.Color
}

// Show builds the glyph.
func (g PluginGlyph) Show(c *ui.Context) ui.Element {
	cs := design.S()
	size := g.Size
	if size == 0 {
		size = 48
	}
	star := size >= 64
	shape := func(e ui.Element, bg ui.Color) ui.Element {
		if star {
			return e.Draw(func(p *ui.Painter, r ui.Rect) {
				path := StarPath(r, 9, .86, .55, .45)
				p.FillPath(&path, bg)
			})
		}
		return e.Radius(size * .32).Background(bg)
	}
	e := ui.Box(c).Size(size, size).Shrink(0).Center()
	if img := DataImage(g.Icon); img != nil {
		// A logo keeps its own colors on white, in light and dark themes.
		pad := size * .12
		if star {
			pad = size * .2
		}
		op := float32(1)
		if g.Muted {
			op = .5
		}
		return shape(e.Padding(pad), ui.RGB(255, 255, 255)).Children(func() {
			ui.Image(c, img).Fill().Fit(ui.Contain).Opacity(op)
		})
	}
	var bg, fg ui.Color
	switch {
	case g.Muted:
		bg, fg = cs.SurfaceContainerHighest, cs.Outline
	case g.Color.A > 0:
		bg, fg = g.Color, g.OnColor
		if fg.A == 0 {
			fg = cs.OnPrimaryContainer
		}
	default:
		sum := 0
		for _, u := range g.ID {
			sum += int(u)
		}
		switch sum % 3 {
		case 0:
			bg, fg = cs.PrimaryContainer, cs.OnPrimaryContainer
		case 1:
			bg, fg = cs.SecondaryContainer, cs.OnSecondaryContainer
		default:
			bg, fg = cs.TertiaryContainer, cs.OnTertiaryContainer
		}
	}
	return shape(e, bg).Children(func() { m3.Icon(c, PluginIcon(g.Icon), size*.5, fg) })
}
