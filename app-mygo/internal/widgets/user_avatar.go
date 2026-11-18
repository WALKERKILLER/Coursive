package widgets

import (
	"strings"
	"unicode"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
)

// AvatarTones are the backgrounds of emoji and initial avatars; the first
// follows the theme.
var AvatarTones = []uint32{0, 0xFFE57373, 0xFFFFB74D, 0xFFDCE775, 0xFF81C784, 0xFF4DB6AC, 0xFF4FC3F7, 0xFF7986CB, 0xFFBA68C8, 0xFFA1887F}

// AvatarToneColors is the container and its content colour for a tone.
func AvatarToneColors(tone int) (ui.Color, ui.Color) {
	tone = min(max(tone, 0), len(AvatarTones)-1)
	s := design.S()
	if seed := AvatarTones[tone]; seed != 0 {
		s = design.FromSeed(design.ARGB(seed), design.TonalSpot, design.T().Dark, 0)
	}
	return s.PrimaryContainer, s.OnPrimaryContainer
}

// Avatar is AvatarView: an avatar of explicit values, for previews.
type Avatar struct {
	Kind  core.AvatarKind
	Value string
	Tone  int
	Frame core.AvatarFrame
	Name  string
	Size  float32
}

// UserAvatar is the configured user avatar.
func UserAvatar(c *ui.Context, p *core.UserProfile, name string, size float32) ui.Element {
	return Avatar{Kind: p.Kind, Value: p.Value, Tone: p.Tone, Frame: p.Frame, Name: name, Size: size}.Show(c)
}

// Show builds the avatar.
func (a Avatar) Show(c *ui.Context) ui.Element {
	size := a.Size
	if size == 0 {
		size = 40
	}
	bg, fg := AvatarToneColors(a.Tone)
	e := ui.Box(c).Size(size, size).Shrink(0)
	frame := a.Frame
	var bmp *ui.Bitmap
	failed := false
	switch a.Kind {
	case core.AvatarFile:
		bmp, failed = FileBitmap(a.Value)
	case core.AvatarURL:
		bmp, failed = NetBitmap(a.Value)
	}
	kind := a.Kind
	if (kind == core.AvatarFile || kind == core.AvatarURL) && failed {
		kind = core.AvatarInitial
	}
	clip := func(p *ui.Painter, r ui.Rect, fn func()) {
		if frame == core.FrameCircle || frame == "" {
			var path ui.Path
			path.Circle(r.X+r.W/2, r.Y+r.H/2, r.W/2)
			p.ClipPath(&path, fn)
			return
		}
		shape := ShapeClover8
		switch frame {
		case core.FrameSquircle:
			shape = ShapeSquircle
		case core.FrameCookie:
			shape = ShapeCookie9
		}
		path := ShapePath(ShapeRadii(shape), r, 0)
		p.ClipPath(&path, fn)
	}
	initial := "?"
	if n := []rune(strings.TrimSpace(a.Name)); len(n) > 0 {
		initial = string(unicode.ToUpper(n[0]))
	}
	return e.Draw(func(p *ui.Painter, r ui.Rect) {
		clip(p, r, func() {
			switch kind {
			case core.AvatarEmoji:
				p.Fill(r, bg, 0)
				drawCentered(p, r, a.Value, size*.54, 0, fg, 1.15)
			case core.AvatarArt:
				spec := CourseArtSpec("avatar:" + a.Value)
				hue := float64(artSeed("avatar:"+a.Value) % 360)
				s := design.FromSeed(hsl(hue, .5, .5), design.TonalSpot, design.T().Dark, 0)
				layout := spec.layout
				if layout == 2 {
					layout = 1
				}
				PaintCourseArt(p, r, spec, s, 0, 0, layout)
			case core.AvatarFile, core.AvatarURL:
				p.Fill(r, bg, 0)
				if bmp != nil {
					p.Image(bmp, r, ui.Cover)
				}
			default:
				p.Fill(r, bg, 0)
				drawCentered(p, r, initial, size*.42, 600, fg, 1)
			}
		})
	})
}

// artSeed is CourseArtSpec's seed for a key.
func artSeed(key string) uint32 {
	var h uint32 = 0x811c9dc5
	for _, u := range utf16Units(key) {
		h = (h ^ uint32(u)) * 0x01000193
	}
	return h
}

func drawCentered(p *ui.Painter, r ui.Rect, s string, size float32, weight int, c ui.Color, height float32) {
	st := design.TextStyle{Size: size, Weight: weight, Color: c, Height: height}
	span := st.Span(s)
	w, h := p.MeasureText(0, span)
	p.RichText(r.X+(r.W-w)/2, r.Y+(r.H-h)/2, 0, span)
}
