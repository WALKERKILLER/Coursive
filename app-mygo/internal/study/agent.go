package study

import (
	"errors"
	"strings"
)

// HasRun reports whether the assistant run id was applied already.
func (d *Document) HasRun(id string) bool { return containsStr(d.AppliedRuns, id) }

// ApplyAgentBatch applies an assistant run's staged operations as one undo
// entry, after validating every one; it returns the ids of the cards it
// created. expectedContents, when set, pins the content stamps of the
// cards (and "ink/"+id strokes) the run read.
func (d *Document) ApplyAgentBatch(result M, expectedEdit int, viewport Rect, allowUpdates bool, expectedContents map[string]string) ([]string, error) {
	runID := str(result["id"])
	if d.HasRun(runID) {
		return nil, nil
	}
	if expectedContents != nil {
		for key, want := range expectedContents {
			if id, ok := strings.CutPrefix(key, "ink/"); ok {
				var stroke *Stroke
				for _, s := range d.Strokes {
					if s.ID == id {
						stroke = s
						break
					}
				}
				if stroke == nil || Encode(stroke.JSON()) != want {
					return nil, errors.New("引用的笔迹已改变，请重新生成")
				}
				continue
			}
			if c := d.Card(key); c == nil || c.ContentStamp() != want {
				return nil, errors.New("引用资料或待修改内容已改变，请重新生成")
			}
		}
	}
	if expectedContents == nil && d.Edit != expectedEdit {
		return nil, errors.New("画布在生成期间已改变，请重新生成以免覆盖你的编辑")
	}
	if !d.Editable(d.ActiveLayer) {
		return nil, errors.New("请先选择可编辑图层")
	}
	ops, ok := result["operations"].([]any)
	if !ok {
		return nil, errors.New("未知画布操作")
	}
	if len(ops) > 20 {
		return nil, errors.New("单次最多 20 个画布操作")
	}
	anchors := map[string]Anchor{}
	for _, a := range list(result["sources"]) {
		anchors[str(obj(a)["id"])] = AnchorFrom(obj(a))
	}
	allowedEdits := map[string]bool{}
	for _, id := range strs(result["editable_ids"]) {
		allowedEdits[id] = true
	}
	var next []*Card
	for _, c := range d.Cards {
		next = append(next, c.Copy())
	}
	links := append([]Connection(nil), d.Connections...)
	var created []string
	citations := func(op M) ([]Anchor, error) {
		var out []Anchor
		for _, id := range list(op["source_ids"]) {
			a, ok := anchors[str(id)]
			if !ok {
				return nil, errors.New("生成物引用了本次没有提供的来源")
			}
			out = append(out, a)
		}
		return out, nil
	}
	blocks := func(op M) ([]Block, error) {
		values := list(op["blocks"])
		if len(values) == 0 || len(values) > 32 {
			return nil, errors.New("卡片内容结构无效")
		}
		var out []Block
		for _, v := range values {
			b := BlockFrom(obj(v))
			switch b.Type {
			case "heading", "paragraph", "list", "steps", "table", "code", "math", "exercise":
			default:
				return nil, errors.New("卡片内容超过限制或结构无效")
			}
			bad := Len16(b.PlainText()) > 12000 || len(b.Items) > 30 || len(b.Rows) > 30 || len(b.Columns) > 8
			if b.Type == "table" {
				bad = bad || len(b.Columns) == 0
				for _, r := range b.Rows {
					bad = bad || len(r) != len(b.Columns)
				}
			}
			if bad {
				return nil, errors.New("卡片内容超过限制或结构无效")
			}
			out = append(out, b)
		}
		return out, nil
	}
	place := func() (Rect, error) {
		for i := 0; i < 150; i++ {
			p := viewport.TopLeft().Add(Pt{30 + float64(i%3)*560, 30 + float64(i/3)*480})
			r := RectLTWH(p.X, p.Y, 520, 440)
			free := true
			for _, c := range next {
				if d.Visible(c.Layer) && c.Rect.Inflate(16).Overlaps(r) {
					free = false
					break
				}
			}
			if free {
				return r, nil
			}
		}
		return Rect{}, errors.New("附近空间不足，请移动到画布空白处重试")
	}
	find := func(id string) *Card {
		for _, c := range next {
			if c.ID == id {
				return c
			}
		}
		return nil
	}
	editableCard := func(id string) bool {
		c := find(id)
		return c != nil && d.Editable(c.Layer)
	}
	changes := false
	for _, raw := range ops {
		op := obj(raw)
		switch op["op"] {
		case "create":
			id, title, typ := str(op["id"]), str(op["title"]), str(op["kind"])
			switch typ {
			case "note", "explanation", "knowledge", "practice":
			default:
				return nil, errors.New("生成卡片标识或类型无效")
			}
			if find(id) != nil || Len16(title) > 240 {
				return nil, errors.New("生成卡片标识或类型无效")
			}
			r, err := place()
			if err != nil {
				return nil, err
			}
			bs, err := blocks(op)
			if err != nil {
				return nil, err
			}
			cs, err := citations(op)
			if err != nil {
				return nil, err
			}
			next = append(next, &Card{ID: id, Type: typ, Layer: d.ActiveLayer, Rect: r, Title: title, Blocks: bs,
				Sources: cs, Generation: runID, PageAspect: .75, Responses: map[string]string{}})
			created = append(created, id)
			changes = true
		case "update":
			if !allowUpdates {
				return nil, errors.New("修改已有卡片前需要确认预览")
			}
			c := find(str(op["object_id"]))
			if c == nil || !c.Generated() || !d.Editable(c.Layer) || !allowedEdits[c.ID] {
				return nil, errors.New("只能修改本次选中的可编辑知识卡片")
			}
			title := str(op["title"])
			if Len16(title) > 240 {
				return nil, errors.New("标题过长")
			}
			cs, err := citations(op)
			if err != nil {
				return nil, err
			}
			var cited []Anchor
			for _, a := range cs {
				if a.ObjectID == c.ID {
					cited = append(cited, c.Sources...)
				} else {
					cited = append(cited, a)
				}
			}
			bs, err := blocks(op)
			if err != nil {
				return nil, err
			}
			c.Title, c.Blocks, c.Responses, c.Sources = title, bs, map[string]string{}, cited
			changes = true
		case "connect":
			from, to, label := str(op["from"]), str(op["to"]), str(op["label"])
			if from == to || !editableCard(from) || !editableCard(to) || Len16(label) > 240 {
				return nil, errors.New("关联对象或标签无效")
			}
			links = append(links, Connection{str(op["id"]), from, to, label})
			changes = true
		case "focus":
		default:
			return nil, errors.New("未知画布操作")
		}
	}
	if len(next) > 2000 || len(links) > 4000 {
		return nil, errors.New("画布对象数量超过限制")
	}
	if changes {
		d.Checkpoint()
		d.AppliedRuns = append(d.AppliedRuns, runID)
		if len(d.AppliedRuns) > 256 {
			d.AppliedRuns = d.AppliedRuns[1:]
		}
		d.Cards, d.Connections = next, links
		d.Changed()
	}
	return created, nil
}
