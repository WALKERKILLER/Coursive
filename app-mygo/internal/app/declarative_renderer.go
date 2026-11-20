package app

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
)

// DeclarativeRenderer is a versioned, host-owned renderer for plugin data
// trees. Plugins submit JSON; the host maps the whitelisted node types to
// native widgets and never evaluates plugin expressions or code. Keep one
// per mounted document: it holds what the user typed into its inputs.
type DeclarativeRenderer struct {
	MaxDepth, MaxNodes, MaxText int

	values map[string]any
	texts  map[string]*string
	tabs   map[string]*int
}

// Show builds document over state; buttons call onAction.
func (r *DeclarativeRenderer) Show(c *ui.Context, document, state core.J, onAction func(action string, args core.J)) {
	if r.MaxDepth == 0 {
		r.MaxDepth, r.MaxNodes, r.MaxText = 24, 2000, 20000
	}
	if r.values == nil {
		r.values, r.texts, r.tabs = map[string]any{}, map[string]*string{}, map[string]*int{}
	}
	merged := core.J{}
	for k, v := range state {
		merged[k] = v
	}
	for k, v := range r.values {
		merged[k] = v
	}
	n := nodeRenderer{r: r, state: merged, onAction: onAction}
	body, err := n.check(document)
	if err != nil {
		DeclarativeErrorView(c)
		return
	}
	n.build(c, body, 0, "0")
}

var errDeclarative = errors.New("declarative")

// check validates the whole document before anything is drawn, so a bad
// document shows the error view rather than half a page.
func (n *nodeRenderer) check(document core.J) (core.J, error) {
	if v, ok := document["schemaVersion"]; ok && v != nil && core.Float(v) != 1 {
		return nil, fmt.Errorf("%w: unsupported schemaVersion", errDeclarative)
	}
	body := document.O("body")
	if body == nil {
		return nil, fmt.Errorf("%w: missing body", errDeclarative)
	}
	count, err := n.count(body, 0)
	if err != nil {
		return nil, err
	}
	if count > n.r.MaxNodes {
		return nil, fmt.Errorf("%w: too many nodes: %d", errDeclarative, count)
	}
	return body, nil
}

func (n *nodeRenderer) count(node any, depth int) (int, error) {
	if j, ok := node.(core.J); ok {
		node = map[string]any(j)
	}
	if depth > n.r.MaxDepth {
		return 0, fmt.Errorf("%w: document is too deep", errDeclarative)
	}
	switch v := node.(type) {
	case []any:
		total := 0
		for _, x := range v {
			k, err := n.count(x, depth+1)
			if err != nil {
				return 0, err
			}
			total += k
		}
		return total, nil
	case map[string]any:
		j := core.J(v)
		if j.S("type") == "text" {
			if s := n.display(firstOf(j, "text", "ref")); len([]rune(s)) > n.r.MaxText {
				return 0, fmt.Errorf("%w: text is too long", errDeclarative)
			}
		}
		total := 1
		for _, key := range []string{"children", "items", "tabs"} {
			k, err := n.count(v[key], depth+1)
			if err != nil {
				return 0, err
			}
			total += k
		}
		return total, nil
	}
	return 0, nil
}

// DeclarativeErrorView says the view cannot be shown.
func DeclarativeErrorView(c *ui.Context) {
	cs := design.S()
	ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(design.SpaceLG).Background(cs.ErrorContainer).Radius(design.CornerLG).Children(func() {
		m3.Icon(c, icons.ErrorOutlineRounded, 24, cs.OnErrorContainer)
		m3.HGap(c, design.SpaceSM)
		m3.Text(c, l10n.PluginViewUnsupported, design.TT().BodyMedium.C(cs.OnErrorContainer)).Grow(1).Basis(0)
	})
}

type nodeRenderer struct {
	r        *DeclarativeRenderer
	state    core.J
	onAction func(string, core.J)
}

func firstOf(j core.J, keys ...string) any {
	for _, k := range keys {
		if v, ok := j[k]; ok && v != nil {
			return v
		}
	}
	return ""
}

func (n *nodeRenderer) children(node core.J) []core.J {
	raw := node["children"]
	if raw == nil {
		raw = node["items"]
	}
	return core.Objs(raw)
}

func (n *nodeRenderer) build(c *ui.Context, node core.J, depth int, path string) {
	cs := design.S()
	tt := design.TT()
	kids := n.children(node)
	each := func() {
		for i, k := range kids {
			p := path + "." + strconv.Itoa(i)
			ui.Column(c.Key(p)).FillWidth().Children(func() { n.build(c, k, depth+1, p) })
		}
	}
	switch t := node.S("type"); t {
	case "column":
		col := ui.Column(c)
		switch node.S("align") {
		case "start":
			col = col.AlignItems(ui.Start)
		case "end":
			col = col.AlignItems(ui.End)
		default:
			col = col.FillWidth().AlignItems(ui.Stretch)
		}
		col.Children(func() {
			for i, k := range kids {
				p := path + "." + strconv.Itoa(i)
				ui.Column(c.Key(p)).Children(func() { n.build(c, k, depth+1, p) })
			}
		})
	case "row":
		ui.Row(c).AlignItems(ui.Start).Children(func() {
			for i, k := range kids {
				p := path + "." + strconv.Itoa(i)
				ui.Column(c.Key(p)).Shrink(1).Children(func() { n.build(c, k, depth+1, p) })
			}
		})
	case "card":
		m3.Card(c, m3.CardElevated).FillWidth().Margin(design.SpaceXS, 0).Children(func() {
			ui.Column(c).FillWidth().Padding(design.SpaceLG).Children(func() {
				if len(kids) > 0 {
					n.build(c, kids[0], depth+1, path+".0")
				}
			})
		})
	case "text":
		st := tt.BodyMedium
		switch node.Or("variant", "body") {
		case "title":
			st = tt.TitleLarge
		case "subtitle":
			st = tt.TitleSmall
		case "label":
			st = tt.LabelLarge
		}
		e := m3.Text(c, n.display(firstOf(node, "text", "ref")), st)
		switch node.S("align") {
		case "center":
			e.FillWidth().TextAlign(ui.Center)
		case "end":
			e.FillWidth().TextAlign(ui.End)
		}
	case "button":
		n.button(c, node)
	case "input":
		n.input(c, node)
	case "list":
		ui.Column(c).FillWidth().Children(each)
	case "table":
		n.table(c, node)
	case "timeline":
		ui.Column(c).FillWidth().Children(func() {
			for i, it := range core.Objs(node["items"]) {
				ui.Row(c.Key(i)).FillWidth().AlignItems(ui.Start).Padding(design.SpaceXS, 0).Children(func() {
					m3.Text(c, n.display(firstOf(it, "time")), tt.LabelSmall).Width(56).Shrink(0)
					m3.Text(c, n.display(firstOf(it, "text")), tt.BodyMedium).Grow(1).Basis(0)
				})
			}
		})
	case "tabs":
		n.tabsNode(c, node, depth, path)
	case "spacer":
		m3.VGap(c, design.SpaceMD)
	case "divider":
		m3.DividerIndent(c, 16, 0, 0)
	default:
		if t == "" {
			t = "unknown"
		}
		ui.Row(c).FillWidth().AlignItems(ui.Center).Margin(design.SpaceXS, 0).Padding(design.SpaceMD).
			Background(cs.SurfaceContainerHighest).Radius(design.CornerMD).Children(func() {
			m3.Icon(c, icons.WidgetsOutlined, 18, cs.OnSurfaceVariant)
			m3.HGap(c, design.SpaceSM)
			m3.Text(c, l10n.PluginViewUnknown(t), tt.BodySmall).Grow(1).Basis(0)
		})
	}
}

func (n *nodeRenderer) button(c *ui.Context, node core.J) {
	press := node.O("onPress")
	action := node.S("action")
	if action == "" {
		action = press.S("action")
	}
	raw := node["args"]
	if raw == nil {
		raw = press["args"]
	}
	kind := m3.Filled
	switch node.Or("variant", "filled") {
	case "tonal":
		kind = m3.Tonal
	case "text":
		kind = m3.TextKind
	}
	ui.Row(c).Children(func() {
		if (m3.Button{Kind: kind, Label: n.display(firstOf(node, "text"))}).Show(c).Clicked() && action != "" && n.onAction != nil {
			args := core.Obj(n.resolve(raw))
			if args == nil {
				args = core.J{}
			}
			n.onAction(action, args)
		}
	})
}

func (n *nodeRenderer) input(c *ui.Context, node core.J) {
	id := node.S("id")
	if id == "" {
		id = node.S("ref")
	}
	if id == "" {
		id = "input"
	}
	v, ok := n.r.texts[id]
	if !ok {
		s := n.display(firstOf(node, "value"))
		v = &s
		n.r.texts[id] = v
	}
	ui.Column(c.Key("in:"+id)).FillWidth().Padding(design.SpaceXS, 0).Children(func() {
		if (m3.Field{Label: n.display(firstOf(node, "label")), Hint: n.display(firstOf(node, "hint"))}).Show(c, v).Changed() {
			n.r.values[id] = *v
		}
	})
}

func (n *nodeRenderer) table(c *ui.Context, node core.J) {
	cs := design.S()
	tt := design.TT()
	var cols []string
	for _, x := range node.L("columns") {
		cols = append(cols, n.display(x))
	}
	rows := node.L("rows")
	if len(cols) == 0 || len(rows) == 0 {
		return
	}
	cell := func(s string, st design.TextStyle) {
		ui.Box(c).Grow(1).Basis(0).Padding(design.SpaceSM).Border(.5, cs.OutlineVariant).Children(func() { m3.Text(c, s, st) })
	}
	ui.Column(c).FillWidth().Border(.5, cs.OutlineVariant).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Stretch).Children(func() {
			for i, s := range cols {
				ui.Box(c.Key(i)).Grow(1).Basis(0).Children(func() { cell(s, tt.LabelLarge) })
			}
		})
		for i, row := range rows {
			cells, ok := row.([]any)
			if !ok {
				continue
			}
			ui.Row(c.Key(i)).FillWidth().AlignItems(ui.Stretch).Children(func() {
				for j := range cols {
					s := ""
					if j < len(cells) {
						s = n.display(cells[j])
					}
					ui.Box(c.Key(j)).Grow(1).Basis(0).Children(func() { cell(s, tt.BodyMedium) })
				}
			})
		}
	})
}

func (n *nodeRenderer) tabsNode(c *ui.Context, node core.J, depth int, path string) {
	views := core.Objs(node["tabs"])
	if len(views) == 0 {
		return
	}
	sel, ok := n.r.tabs[path]
	if !ok {
		sel = new(int)
		n.r.tabs[path] = sel
	}
	if *sel >= len(views) {
		*sel = 0
	}
	var tabs []m3.Tab
	for _, t := range views {
		tabs = append(tabs, m3.Tab{Label: n.display(firstOf(t, "label"))})
	}
	ui.Column(c).FillWidth().Children(func() {
		m3.TabBar(c, tabs, sel, m3.TabOpts{})
		m3.TabView(c, ui.Box(c).FillWidth().Height(240), sel, len(views), func(tab int) {
			ui.Scroll(c).Fill().Children(func() {
				ui.Column(c).FillWidth().Children(func() {
					for i, k := range core.Objs(views[tab]["children"]) {
						p := path + ".t" + strconv.Itoa(tab) + "." + strconv.Itoa(i)
						ui.Column(c.Key(p)).FillWidth().Children(func() { n.build(c, k, depth+1, p) })
					}
				})
			})
		})
	})
}

// display is a bound value as display text; a missing binding is empty.
func (n *nodeRenderer) display(v any) string {
	r := n.resolve(v)
	switch x := r.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	}
	return core.Encode(r)
}

var bindingRe = regexp.MustCompile(`^\$\{([^}]+)\}$`)

func (n *nodeRenderer) resolve(v any) any {
	if j, ok := v.(core.J); ok {
		v = map[string]any(j)
	}
	switch x := v.(type) {
	case string:
		if m := bindingRe.FindStringSubmatch(x); m != nil {
			return n.readPath(m[1])
		}
		return x
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = n.resolve(e)
		}
		return out
	case map[string]any:
		if ref, ok := x["ref"].(string); ok {
			return n.readPath(ref)
		}
		out := map[string]any{}
		for k, e := range x {
			out[k] = n.resolve(e)
		}
		return out
	}
	return v
}

func (n *nodeRenderer) readPath(path string) any {
	var cur any = map[string]any(n.state)
	for _, part := range strings.Split(path, ".") {
		switch x := cur.(type) {
		case map[string]any:
			cur = x[part]
		case core.J:
			cur = x[part]
		case []any:
			i, err := strconv.Atoi(part)
			if err != nil || i < 0 || i >= len(x) {
				return nil
			}
			cur = x[i]
		default:
			return nil
		}
	}
	return cur
}
