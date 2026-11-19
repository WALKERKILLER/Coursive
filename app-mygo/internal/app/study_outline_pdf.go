package app

import (
	"encoding/base64"
	"errors"
	"image"
	"math"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/assets"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/pdf"
	"github.com/walkerkiller/coursive/app-mygo/internal/study"
	"github.com/walkerkiller/coursive/app-mygo/internal/tex"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// outlineFonts are the embedded Noto Sans SC subset, regular and bold,
// parsed once.
var outlineFonts = sync.OnceValues(func() ([2]*pdf.Font, error) {
	var out [2]*pdf.Font
	for i, name := range []string{"Regular", "Bold"} {
		b, err := assets.Bytes("fonts/NotoSansSC-" + name + ".ttf")
		if err != nil {
			return out, err
		}
		if out[i], err = pdf.ParseTTF(b); err != nil {
			return out, err
		}
	}
	return out, nil
})

// rasterize draws w×h DIPs of paint offscreen at four device pixels per
// DIP: a picture for the PDF, sized in points.
func rasterize(w, h float32, paint func(p *ui.Painter, r ui.Rect)) image.Image {
	return ui.Render(func(c *ui.Context) {
		ui.Box(c).Width(w).Height(h).Draw(paint)
	}, int(w), int(h), 4)
}

// outlineFormulaImage is a formula drawn ahead of layout; nil when it
// does not typeset, so it stays as its TeX source. The app sets inline
// math a tenth above text size; so does the 10.5 pt body.
func outlineFormulaImage(f study.OutlineFormula) *study.OutlineImage {
	size := float32(11.5)
	if f.Display {
		size = 12
	}
	t := tex.Typeset(f.Tex, size, f.Display)
	if t.Width <= 1 {
		return nil
	}
	w, h := float32(math.Ceil(float64(t.Width))), float32(math.Ceil(float64(t.Height+t.Depth)))
	img := rasterize(w, h, func(p *ui.Painter, r ui.Rect) { t.Draw(p, r.X, r.Y+t.Height, design.ARGB(0xff1d2125)) })
	return &study.OutlineImage{Image: img, W: float64(w), H: float64(h), Baseline: float64(t.Height)}
}

// outlinePdfDialog lays the outline out as a PDF and previews pages of
// exactly the bytes 「保存 PDF」 writes; the daemon renders them with the
// canvas's page renderer. Changing the paper builds new bytes and a new
// preview.
type outlinePdfDialog struct {
	doc      study.OutlineDoc
	markdown func() string
	images   map[string]study.OutlineImage
	ready    bool
	closed   atomic.Bool

	layout          study.OutlineLayout
	built           map[study.OutlineLayout][]byte
	bytes           []byte
	page            ui.ImageSource
	number, count   int
	token           int
	busy, textLayer bool
	err, previewErr string
	notice          string
}

func (s *StudyCanvas) exportOutlinePdf(turn core.J) {
	doc := s.outlineExport(turn)
	d := &outlinePdfDialog{doc: doc, markdown: func() string { return study.OutlineMarkdown(s.outlineExport(turn)) },
		built: map[study.OutlineLayout][]byte{}, number: 1, busy: true}
	// Marked handwriting is drawn beside its item.
	inks := map[string][]*study.Stroke{}
	for _, b := range doc.Blocks() {
		if e, ok := b.(study.OutlineEntry); ok && e.Item != nil && core.Str(e.Item["kind"]) == "unresolved_mark" {
			if strokes := s.markInk(e.Item); len(strokes) > 0 {
				inks[study.OutlineInkKey(e.Item)] = strokes
			}
		}
	}
	go d.prepare(inks)
	m3.ShowDialog(d.view, func(any) {
		d.closed.Store(true)
		d.token++
	})
}

// prepare draws the formulas and ink before layout, one picture per turn
// of the UI thread, then builds the PDF.
func (d *outlinePdfDialog) prepare(inks map[string][]*study.Stroke) {
	images := map[string]study.OutlineImage{}
	for i, f := range study.OutlineFormulas(d.doc) {
		if i >= 200 || d.closed.Load() {
			break
		}
		if img := onUIValue(func() *study.OutlineImage { return outlineFormulaImage(f) }); img != nil {
			images[study.OutlineMathKey(f.Tex, f.Display)] = *img
		}
	}
	for key, strokes := range inks {
		if d.closed.Load() {
			break
		}
		img := onUIValue(func() image.Image {
			return rasterize(96, 72, func(p *ui.Painter, r ui.Rect) { paintInkFit(p, r, strokes, 5) })
		})
		images[key] = study.OutlineImage{Image: img, W: 96, H: 72, Baseline: -1}
	}
	core.Post(func() {
		d.images, d.ready = images, true
		d.build()
	})
}

func (d *outlinePdfDialog) build() {
	d.token++
	token := d.token
	d.busy, d.err, d.previewErr = true, "", ""
	if !d.ready || d.closed.Load() {
		return
	}
	if b := d.built[d.layout]; b != nil {
		d.bytes = b
		d.show(d.number)
		return
	}
	doc, layout, images := d.doc, d.layout, d.images
	go func() {
		var out []byte
		var problem string
		fonts, err := outlineFonts()
		switch {
		case err != nil:
			problem = l10n.OutlineExportFailed(err.Error())
		default:
			if missing := study.OutlineMissingGlyphs(doc, fonts[0].Has); len(missing) > 0 {
				problem = l10n.OutlinePdfMissing(strings.Join(missing[:min(6, len(missing))], ""), len(missing))
			} else {
				out = study.BuildOutlinePdf(doc, layout, fonts[0], fonts[1], images, time.Now())
			}
		}
		core.Post(func() {
			if token != d.token {
				return
			}
			if problem != "" {
				d.busy, d.err = false, problem
				return
			}
			d.built[layout], d.bytes = out, out
			d.show(d.number)
		})
	}()
}

func (d *outlinePdfDialog) show(number int) {
	bytes, token := d.bytes, d.token
	if bytes == nil {
		return
	}
	d.busy = true
	core.Async(func() (any, error) { return State.Api().PreviewExportPDF(bytes, number) }, func(out any, err error) {
		if token != d.token {
			return
		}
		d.busy = false
		var ae *core.ApiError
		if errors.As(err, &ae) && ae.Status == 413 && number > 1 {
			// Past the end after a paper change.
			d.show(1)
			return
		}
		if err != nil {
			d.previewErr = l10n.OutlinePdfPreviewFailed(err.Error())
			return
		}
		m := core.Obj(out)
		png, err := base64.StdEncoding.DecodeString(core.Str(m["png"]))
		var page ui.ImageSource
		if err == nil {
			page, err = ui.DecodeBitmap(png)
		}
		if err != nil {
			d.previewErr = l10n.OutlinePdfPreviewFailed(err.Error())
			return
		}
		d.count, d.number, d.page = core.Int(m["count"]), number, page
		d.textLayer = strings.TrimSpace(core.Str(m["text"])) != ""
		d.previewErr = ""
	})
}

func (d *outlinePdfDialog) relayout(next study.OutlineLayout) {
	if next == d.layout {
		return
	}
	d.layout = next
	d.build()
}

func (d *outlinePdfDialog) save(markdown bool) {
	name, ext, filter := d.doc.Title, ".pdf", core.FileFilter{Name: "PDF", Extensions: []string{"pdf"}}
	if markdown {
		ext, filter = ".md", core.FileFilter{Name: "Markdown", Extensions: []string{"md"}}
	}
	widgets.PickSavePath(name+ext, filter, "", func(path string) {
		if path == "" {
			return
		}
		data := d.bytes
		if markdown {
			data = []byte(d.markdown())
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			d.err = l10n.OutlineExportFailed(err.Error())
			return
		}
		d.notice = l10n.OutlineExportSaved(path)
	})
}

func (d *outlinePdfDialog) view(c *ui.Context, r *m3.Route) {
	cs, tt := design.S(), design.TT()
	wide := WindowWidth >= 760
	w, h := min(920, WindowWidth-80), min(720, WindowHeight-48)
	m3.Dialog(c, func() {
		ui.Column(c).Width(w).Height(h).Padding(design.SpaceXL).Children(func() {
			m3.Text(c, l10n.OutlinePdfTitle, tt.TitleLarge.C(cs.OnSurface))
			m3.Text(c, d.doc.Title, tt.BodyMedium.C(cs.OnSurfaceVariant)).MaxLines(1)
			m3.VGap(c, design.SpaceLG)
			body := ui.Column(c).Gap(design.SpaceMD)
			if wide {
				body = ui.Row(c).Gap(design.SpaceXL)
			}
			body.Grow(1).Basis(0).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
				d.preview(c)
				controls := ui.Scroll(c).Shrink(1)
				if wide {
					controls = controls.Width(240).Shrink(0)
				}
				controls.Children(func() { d.controls(c, r) })
			})
		})
	})
}

func (d *outlinePdfDialog) preview(c *ui.Context) {
	cs, tt := design.S(), design.TT()
	pw, ph := d.layout.Size()
	aspect := float32(pw / ph)
	ui.Column(c.Key("outline-pdf-preview")).Grow(1).Basis(0).MinHeight(160).MinWidth(0).Padding(design.SpaceLG).
		Radius(design.CornerLG).Background(cs.SurfaceContainerHighest.Alpha(.5)).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Basis(0).MinHeight(0).Width(0).FillWidth().AlignItems(ui.Center).Justify(ui.Center).Draw(func(p *ui.Painter, r ui.Rect) {
			pr := r
			if r.W/r.H > aspect {
				pr.W = r.H * aspect
				pr.X += (r.W - pr.W) / 2
			} else {
				pr.H = r.W / aspect
				pr.Y += (r.H - pr.H) / 2
			}
			p.Shadow(pr, 2, 0, 6, 18, 0, ui.RGBA(0, 0, 0, .12))
			p.Fill(pr, ui.RGB(255, 255, 255), 2)
			if d.page != nil {
				p.Image(d.page, pr, ui.Contain)
				if d.busy {
					p.Fill(pr, ui.RGBA(255, 255, 255, .5), 2)
				}
			}
		}).Children(func() {
			switch {
			case d.page != nil:
			case d.previewErr != "":
				m3.Text(c, d.previewErr, tt.BodySmall.C(cs.Error)).TextAlign(ui.Center).Padding(design.SpaceLG).MaxWidth(320)
			case d.err == "":
				m3.Circular(c, m3.Progress{})
			default:
				m3.Icon(c, icons.PictureAsPdfOutlined, 24, cs.Outline)
			}
		})
		m3.VGap(c, design.SpaceMD)
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			if (m3.IconButton{Icon: icons.ChevronLeftRounded, Tooltip: l10n.OutlinePrevPage, Disabled: d.busy || d.number <= 1}).Show(c.Key("prev")).Clicked() {
				d.show(d.number - 1)
			}
			label := "–"
			if d.count > 0 {
				label = l10n.OutlinePdfPage(d.number, d.count)
			}
			m3.Text(c, label, tt.LabelLarge.C(cs.OnSurface))
			if (m3.IconButton{Icon: icons.ChevronRightRounded, Tooltip: l10n.OutlineNextPage, Disabled: d.busy || d.number >= d.count}).Show(c.Key("next")).Clicked() {
				d.show(d.number + 1)
			}
		})
	})
}

// pdfChoice is a labelled segmented choice of the PDF's layout.
func pdfChoice[T comparable](c *ui.Context, label string, value T, segs []m3.Segment[T], change func(T)) {
	cs, tt := design.S(), design.TT()
	ui.Column(c.Key(label)).Children(func() {
		m3.Text(c, label, tt.LabelMedium.C(cs.OnSurfaceVariant))
		m3.VGap(c, design.SpaceXS)
		if m3.Segmented(c, segs, &value, m3.SegOpts{NoCheck: true, Compact: true}) {
			change(value)
		}
		m3.VGap(c, design.SpaceMD)
	})
}

func (d *outlinePdfDialog) controls(c *ui.Context, r *m3.Route) {
	cs, tt := design.S(), design.TT()
	l := d.layout
	ui.Column(c).AlignItems(ui.Stretch).Children(func() {
		pdfChoice(c, l10n.OutlinePaper, l.Paper, []m3.Segment[study.OutlinePaper]{
			{Value: study.PaperA4, Label: "A4"}, {Value: study.PaperLetter, Label: "Letter"},
		}, func(v study.OutlinePaper) { l.Paper = v; d.relayout(l) })
		pdfChoice(c, l10n.OutlineOrientation, l.Landscape, []m3.Segment[bool]{
			{Value: false, Label: l10n.OutlinePortrait}, {Value: true, Label: l10n.OutlineLandscape},
		}, func(v bool) { l.Landscape = v; d.relayout(l) })
		pdfChoice(c, l10n.OutlineMargins, l.Compact, []m3.Segment[bool]{
			{Value: false, Label: l10n.OutlineMarginStandard}, {Value: true, Label: l10n.OutlineMarginCompact},
		}, func(v bool) { l.Compact = v; d.relayout(l) })
		if d.bytes != nil && d.err == "" {
			var facts []string
			if d.textLayer {
				facts = append(facts, l10n.OutlinePdfTextLayer)
			}
			facts = append(facts, core.FmtBytes(float64(len(d.bytes))))
			m3.Text(c, strings.Join(facts, " · "), tt.LabelSmall.C(cs.OnSurfaceVariant))
		}
		if d.err != "" || d.notice != "" {
			text, color := d.notice, cs.Primary
			if d.err != "" {
				text, color = d.err, cs.Error
			}
			m3.Text(c, text, tt.LabelSmall.C(color)).Margin(design.SpaceSM, 0, 0, 0)
		}
		m3.VGap(c, design.SpaceLG)
		if (m3.Button{Kind: m3.Filled, Label: l10n.OutlinePdfSave, Icon: icons.DownloadRounded,
			Disabled: d.bytes == nil || d.err != "" || d.busy}).Show(c.Key("save")).Clicked() {
			d.save(false)
		}
		m3.VGap(c, design.SpaceXS)
		if m3.TextButton(c.Key("md"), l10n.OutlinePdfMarkdown).Clicked() {
			d.save(true)
		}
		if m3.TextButton(c.Key("cancel"), l10n.CommonCancel).Clicked() {
			r.Close()
		}
	})
}
