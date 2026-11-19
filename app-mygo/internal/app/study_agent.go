package app

import (
	"math"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/study"
)

// study_agent_ui.dart: editing a generated knowledge card and the links
// between cards.

// knowledgeField is one block as a single text field: a table's rows with
// tab-separated cells, a list's items one per line.
func knowledgeField(b study.Block) string {
	switch b.Type {
	case "table":
		lines := []string{strings.Join(b.Columns, "\t")}
		for _, r := range b.Rows {
			lines = append(lines, strings.Join(r, "\t"))
		}
		return strings.Join(lines, "\n")
	case "list", "steps":
		return strings.Join(b.Items, "\n")
	}
	return b.Text
}

func knowledgeLabel(kind string) string {
	switch kind {
	case "table":
		return "表格：首行为列名，列用 Tab 分隔"
	case "list", "steps":
		return "每行一项"
	}
	return kind
}

func (s *StudyCanvas) editKnowledge(original *study.Card) {
	title := original.Title
	blocks := append([]study.Block(nil), original.Blocks...)
	values := make([]string, len(blocks))
	for i, b := range blocks {
		values[i] = knowledgeField(b)
	}
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		m3.AlertDialog(c, m3.Alert{Title: "编辑知识卡片", Width: 650, Content: func() {
			ui.Scroll(c).FillWidth().Children(func() {
				ui.Column(c).FillWidth().Children(func() {
					m3.Field{Label: "标题", MaxLength: 240, Counter: true}.Show(c.Key("knowledge-title"), &title)
					for i := range blocks {
						b := &blocks[i]
						ui.Column(c.Key(i)).FillWidth().Margin(design.SpaceMD, 0, 0, 0).Children(func() {
							m3.Field{Label: knowledgeLabel(b.Type), MinLines: 2, MaxLines: 9, MaxLength: 12000, Counter: true}.Show(c.Key("value"), &values[i])
							if b.Type == "exercise" {
								m3.Field{Label: "提示"}.Show(c.Key("hint"), &b.Hint)
								m3.Field{Label: "答案与解析", MinLines: 2, MaxLines: 5}.Show(c.Key("answer"), &b.Answer)
							}
						})
					}
				})
			})
		}, Actions: func() {
			if m3.TextButton(c, "取消").Clicked() {
				r.Pop(nil)
			}
			if m3.FilledButton(c, "保存").Clicked() {
				r.Pop(true)
			}
		}})
	}, func(v any) {
		if v != true || s.disposed || s.doc == nil {
			return
		}
		d := s.doc
		if !d.Editable(original.Layer) {
			s.toast("该图层不可编辑")
			return
		}
		for i := range blocks {
			b, v := &blocks[i], values[i]
			switch b.Type {
			case "table":
				rows := strings.Split(v, "\n")
				b.Columns, b.Rows = strings.Split(rows[0], "\t"), nil
				for _, row := range rows[1:] {
					b.Rows = append(b.Rows, strings.Split(row, "\t"))
				}
				ok := len(b.Columns) > 0 && len(b.Columns) <= 8
				for _, row := range b.Rows {
					ok = ok && len(row) == len(b.Columns)
				}
				if !ok {
					s.toast("表格各行的列数须相同")
					return
				}
			case "list", "steps":
				b.Items = strings.Split(v, "\n")
			default:
				b.Text = v
			}
		}
		d.Checkpoint()
		original.Title, original.Blocks = title, blocks
		d.Changed()
	})
}

// connectionSettings lists the links between cards, to relabel or remove.
func (s *StudyCanvas) connectionSettings() {
	m3.ShowModalBottomSheet(m3.SheetOptions{}, func(c *ui.Context, r *m3.Route) {
		d := s.doc
		if d == nil {
			return
		}
		cs, tt := design.S(), design.TT()
		ui.Column(c).FillWidth().Height(420).Children(func() {
			m3.Tile{Title: "知识关联", Subtitle: "关系随对象移动；更改与删除都可撤销。"}.Show(c)
			ui.Scroll(c).FillWidth().Grow(1).Basis(0).Children(func() {
				ui.Column(c).FillWidth().Children(func() {
					for i := range d.Connections {
						link := d.Connections[i]
						a, b := d.Card(link.From), d.Card(link.To)
						editable := a != nil && b != nil && d.Editable(a.Layer) && d.Editable(b.Layer)
						from, to := link.From, link.To
						if a != nil {
							from = a.Title
						}
						if b != nil {
							to = b.Title
						}
						id := link.ID
						t := m3.Tile{Title: from + " → " + to, Subtitle: link.Label, Tap: editable, Trailing: func() {
							if (m3.IconButton{Icon: icons.LinkOff, Tooltip: "移除关联", Disabled: !editable}).Show(c).Clicked() {
								d.Checkpoint()
								d.Connections = connectionsWithout(d.Connections, id)
								d.Changed()
							}
						}}.Show(c.Key(id))
						if editable && t.Clicked() {
							s.relabelConnection(id, link.Label)
						}
					}
					if len(d.Connections) == 0 {
						m3.Text(c, "还没有知识关联", tt.BodyMedium.C(cs.OnSurfaceVariant)).Margin(design.SpaceLG)
					}
				})
			})
		})
	})
}

func connectionsWithout(list []study.Connection, id string) []study.Connection {
	out := list[:0:0]
	for _, l := range list {
		if l.ID != id {
			out = append(out, l)
		}
	}
	return out
}

func (s *StudyCanvas) relabelConnection(id, label string) {
	input := label
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		m3.AlertDialog(c, m3.Alert{Title: "关系说明", Content: func() {
			m3.Field{MaxLength: 240, Counter: true, AutoFocus: true}.Show(c, &input)
		}, Actions: func() {
			if m3.TextButton(c, "取消").Clicked() {
				r.Pop(nil)
			}
			if m3.FilledButton(c, "保存").Clicked() {
				r.Pop(input)
			}
		}})
	}, func(v any) {
		text, ok := v.(string)
		if !ok || s.disposed || s.doc == nil {
			return
		}
		d := s.doc
		for i := range d.Connections {
			if d.Connections[i].ID == id {
				d.Checkpoint()
				d.Connections[i].Label = text
				d.Changed()
				return
			}
		}
	})
}

// paintConnections is _ConnectionPainter: a curve from edge to edge of two
// linked objects, a dot at its end and its label midway, the same width
// at any zoom.
func paintConnections(p *ui.Painter, r ui.Rect, d *study.Document, stage study.Rect, ch chrome) {
	if len(d.Connections) == 0 {
		return
	}
	rects := map[string]study.Rect{}
	for _, c := range d.Cards {
		if d.Visible(c.Layer) {
			rects[c.ID] = c.Rect
		}
	}
	for _, o := range d.PluginObjects {
		rects[o.ID()] = o.Rect()
	}
	edge := func(r study.Rect, toward study.Pt) study.Pt {
		delta := toward.Sub(r.Center())
		scale := math.Inf(1)
		if math.Abs(delta.X) >= .001 {
			scale = r.W() / 2 / math.Abs(delta.X)
		}
		if math.Abs(delta.Y) >= .001 {
			scale = math.Min(scale, r.H()/2/math.Abs(delta.Y))
		}
		if math.IsInf(scale, 1) {
			return r.Center()
		}
		return r.Center().Add(delta.Mul(scale))
	}
	z := float32(d.Zoom)
	at := func(q study.Pt) (float32, float32) { return r.X + f32(q.X-stage.L), r.Y + f32(q.Y-stage.T) }
	for _, link := range d.Connections {
		a, okA := rects[link.From]
		b, okB := rects[link.To]
		if !okA || !okB || !study.RectPts(a.Center(), b.Center()).Inflate(60).Overlaps(stage) {
			continue
		}
		start, end := edge(a, b.Center()), edge(b, a.Center())
		mid := start.Lerp(end, .5)
		sx, sy := at(start)
		ex, ey := at(end)
		mx, my := at(mid)
		path := (&ui.Path{}).MoveTo(sx, sy).CubeTo(mx, sy, mx, ey, ex, ey)
		p.StrokePath(path, 2/z, ch.accent.Alpha(.65))
		p.FillPath((&ui.Path{}).Circle(ex, ey, 4/z), ch.accent)
		if link.Label == "" {
			continue
		}
		span := ui.Span{Text: link.Label, Size: 12 / z, Color: ch.onAccentSoft, Background: ch.accentSoft}
		w, h := p.MeasureText(180/z, span)
		p.RichText(mx-w/2, my-h/2, 180/z, span)
	}
}
