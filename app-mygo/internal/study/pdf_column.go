package study

import (
	"errors"
	"math"
	"sort"
)

// paper is where a PDF card's page sits inside its frame.
func paper(c *Card, aspect float64) Rect {
	height := c.Rect.H()
	if c.Collapsed {
		height = c.ExpandedH
	}
	chrome := 36.0
	if c.Type == "pdfdoc" {
		chrome = 72
	}
	area := RectLTWH(c.Rect.L, c.Rect.T+36, c.Rect.W(), math.Max(1, height-chrome))
	width := math.Min(area.W(), area.H()*aspect)
	return RectCenter(area.Center(), width, width/aspect)
}

func reanchor(s *Stroke, anchor string, page int, from, to Rect) *Stroke {
	sx, sy := to.W()/from.W(), to.H()/from.H()
	n := s.With(s.ID, make([]InkPoint, len(s.Points)))
	n.Anchor, n.Page, n.Width = anchor, page, s.Width*math.Sqrt(sx*sy)
	for i, p := range s.Points {
		n.Points[i] = InkPoint{Pt{to.L + (p.P.X-from.L)*sx, to.T + (p.P.Y-from.T)*sy}, p.Pressure}
	}
	return n
}

func (d *Document) has(c *Card) bool {
	for _, x := range d.Cards {
		if x == c {
			return true
		}
	}
	return false
}

// PDFPages are the page cards of the column of root, by page
// (study_pdf_column.dart).
func (d *Document) PDFPages(root string) []*Card {
	var out []*Card
	for _, c := range d.Cards {
		if c.PDFColumn == root && c.Type == "pdf" {
			out = append(out, c)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Page < out[j].Page })
	return out
}

var errLocked = errors.New("请先解锁文档和批注图层")

// BeginPDFColumn folds a PDF document into a header its pages hang under.
func (d *Document) BeginPDFColumn(root *Card) error {
	if root.Type != "pdfdoc" || !d.has(root) || !d.CanTransformCard(root) {
		return errLocked
	}
	if len(d.Cards)+root.PageCount-len(d.PDFPages(root.ID)) > 2000 {
		return errors.New("展开后超过画布 2000 张卡片上限")
	}
	if root.PDFColumn == root.ID {
		return nil
	}
	d.Checkpoint()
	root.PDFColumn = root.ID
	if !root.Collapsed {
		root.ExpandedH = root.Rect.H()
	}
	root.Collapsed = true
	root.Rect = RectLTWH(root.Rect.L, root.Rect.T, root.Rect.W(), 60)
	d.Changed()
	return nil
}

// AppendPDFPage hangs page of root's column under it, from the page data
// the daemon gave (width, height, file, text, boxes), moving the ink of
// that page over and the later pages down.
func (d *Document) AppendPDFPage(root *Card, page int, data M) error {
	if !d.has(root) || root.PDFColumn != root.ID {
		return nil
	}
	pages := d.PDFPages(root.ID)
	for _, c := range pages {
		if c.Page == page {
			return nil
		}
	}
	if !d.CanTransformCard(root) {
		return errLocked
	}
	for _, c := range pages {
		if !d.CanTransformCard(c) {
			return errLocked
		}
	}
	aspect := num(data["width"], math.NaN()) / num(data["height"], math.NaN())
	if !finite(aspect) || aspect <= 0 || page < 1 || page > root.PageCount {
		return errors.New("PDF 页面尺寸或页码无效")
	}
	top := root.Rect.B + 24
	for _, c := range pages {
		if c.Page < page {
			top = c.Rect.B + 24
		}
	}
	card := &Card{ID: NewID(), Type: "pdf", Layer: root.Layer,
		Rect:  RectLTWH(root.Rect.L, top, root.Rect.W(), root.Rect.W()/aspect+36),
		Title: root.Title, Source: root.Source, DocumentID: root.DocumentID,
		Path: str(data["file"]), Text: strOr(data["text"]), TextBoxes: list(data["boxes"]),
		Page: page, PageCount: root.PageCount, PageAspect: aspect, PDFColumn: root.ID, Responses: map[string]string{}}
	nextTop := card.Rect.B + 24
	for _, after := range pages {
		if after.Page <= page {
			continue
		}
		old := after.Rect
		d.TransformCards(map[string]bool{after.ID: true}, old, RectLTWH(root.Rect.L, nextTop, old.W(), old.H()))
		nextTop = after.Rect.B + 24
	}
	from, to := paper(root, aspect), paper(card, aspect)
	for i, s := range d.Strokes {
		if s.Anchor == root.ID && (s.Page == page || (s.Page == 0 && root.Page == page)) {
			d.Strokes[i] = reanchor(s, card.ID, page, from, to)
		}
	}
	d.Cards = append(d.Cards, card)
	d.Changed()
	return nil
}

// CollapsePDFColumn folds the pages of column id back into its document,
// with their ink and links.
func (d *Document) CollapsePDFColumn(id string) error {
	var root *Card
	for _, c := range d.Cards {
		if c.ID == id && c.Type == "pdfdoc" {
			root = c
		}
	}
	if root == nil {
		return errors.New("主文档已删除，请先撤销删除")
	}
	pages := d.PDFPages(id)
	if !d.CanTransformCard(root) {
		return errLocked
	}
	for _, c := range pages {
		if !d.CanTransformCard(c) {
			return errLocked
		}
	}
	d.Checkpoint()
	byID := map[string]*Card{}
	for _, p := range pages {
		byID[p.ID] = p
	}
	for i, s := range d.Strokes {
		if p := byID[s.Anchor]; p != nil {
			d.Strokes[i] = reanchor(s, root.ID, p.Page, paper(p, p.PageAspect), paper(root, p.PageAspect))
		}
	}
	var links []Connection
	for _, c := range d.Connections {
		if byID[c.From] != nil {
			c.From = root.ID
		}
		if byID[c.To] != nil {
			c.To = root.ID
		}
		if c.From != c.To {
			links = append(links, c)
		}
	}
	d.Connections = links
	var cards []*Card
	for _, c := range d.Cards {
		if byID[c.ID] == nil {
			cards = append(cards, c)
		}
	}
	d.Cards = cards
	root.PDFColumn, root.Collapsed = "", false
	root.Rect = RectLTWH(root.Rect.L, root.Rect.T, root.Rect.W(), math.Max(240, root.ExpandedH))
	d.Changed()
	return nil
}
