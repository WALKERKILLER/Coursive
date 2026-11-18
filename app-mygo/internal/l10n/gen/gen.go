// Package gen turns the Flutter client's ARB file into Go: a constant for
// each plain string and a function for each message with placeholders, so
// both clients read the same strings from app/lib/l10n/app_zh.arb. It
// follows gen-l10n with use-escaping: '{' is a literal brace, ” a quote,
// and {n, plural, =0{…} other{…}} picks by the count (Chinese has only
// "other", besides the exact matches).
package gen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/format"
	"go/token"
	"sort"
	"strconv"
	"strings"
)

type placeholder struct {
	Type string `json:"type"`
}

type meta struct {
	Placeholders map[string]placeholder `json:"placeholders"`
}

// node is a part of a message: literal text, a placeholder or a plural.
type node struct {
	text   string
	arg    string
	plural map[string][]node // "=0", "other", …
}

// Generate returns the Go source for an ARB document.
func Generate(arb []byte) ([]byte, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(arb, &raw); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(raw))
	for k := range raw {
		if !strings.HasPrefix(k, "@") {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var b bytes.Buffer
	b.WriteString("// Code generated from app/lib/l10n/app_zh.arb; DO NOT EDIT.\n\npackage l10n\n\nimport \"fmt\"\n\nvar _ = fmt.Sprint\n\n")
	for _, key := range keys {
		var msg string
		if err := json.Unmarshal(raw[key], &msg); err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		var m meta
		if r, ok := raw["@"+key]; ok {
			if err := json.Unmarshal(r, &m); err != nil {
				return nil, fmt.Errorf("@%s: %w", key, err)
			}
		}
		nodes, rest, err := parse(msg, false)
		if err != nil || rest != "" {
			return nil, fmt.Errorf("%s: cannot parse %q", key, msg)
		}
		name := strings.ToUpper(key[:1]) + key[1:]
		if len(m.Placeholders) == 0 {
			fmt.Fprintf(&b, "const %s = %s\n\n", name, strconv.Quote(literal(nodes)))
			continue
		}
		order, err := placeholderOrder(raw["@"+key])
		if err != nil {
			return nil, fmt.Errorf("@%s: %w", key, err)
		}
		params := make([]string, len(order))
		for i, p := range order {
			params[i] = ident(p) + " " + goType(m.Placeholders[p].Type)
		}
		fmt.Fprintf(&b, "func %s(%s) string {\n\treturn %s\n}\n\n", name, strings.Join(params, ", "), expr(nodes, m.Placeholders))
	}
	return format.Source(b.Bytes())
}

// placeholderOrder lists the placeholders as gen-l10n orders the
// parameters: as the metadata declares them.
func placeholderOrder(metaJSON json.RawMessage) ([]string, error) {
	var m struct {
		Placeholders json.RawMessage `json:"placeholders"`
	}
	if err := json.Unmarshal(metaJSON, &m); err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(m.Placeholders))
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	var names []string
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return nil, err
		}
		names = append(names, t.(string))
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return nil, err
		}
	}
	return names, nil
}

// ident is a placeholder's name as a Go parameter, which a keyword is not.
func ident(name string) string {
	if token.IsKeyword(name) {
		return name + "_"
	}
	return name
}

func goType(t string) string {
	switch t {
	case "int":
		return "int"
	case "String":
		return "string"
	}
	return "any"
}

func literal(nodes []node) string {
	var s strings.Builder
	for _, n := range nodes {
		s.WriteString(n.text)
	}
	return s.String()
}

func expr(nodes []node, ps map[string]placeholder) string {
	if len(nodes) == 0 {
		return `""`
	}
	parts := make([]string, 0, len(nodes))
	for _, n := range nodes {
		switch {
		case n.plural != nil:
			var sw strings.Builder
			sw.WriteString("func() string {\n\t\tswitch {\n")
			cases := make([]string, 0, len(n.plural))
			for k := range n.plural {
				if k != "other" {
					cases = append(cases, k)
				}
			}
			sort.Strings(cases)
			for _, k := range cases {
				fmt.Fprintf(&sw, "\t\tcase %s == %s:\n\t\t\treturn %s\n", ident(n.arg), strings.TrimPrefix(k, "="), expr(n.plural[k], ps))
			}
			fmt.Fprintf(&sw, "\t\t}\n\t\treturn %s\n\t}()", expr(n.plural["other"], ps))
			parts = append(parts, sw.String())
		case n.arg != "":
			switch goType(ps[n.arg].Type) {
			case "string":
				parts = append(parts, ident(n.arg))
			default:
				parts = append(parts, "fmt.Sprint("+ident(n.arg)+")")
			}
		default:
			parts = append(parts, strconv.Quote(n.text))
		}
	}
	return strings.Join(parts, " + ")
}

// parse reads a message up to the end, or up to the "}" closing a plural's
// case when nested, which it leaves in the rest.
func parse(s string, nested bool) ([]node, string, error) {
	var nodes []node
	var text strings.Builder
	flush := func() {
		if text.Len() > 0 {
			nodes = append(nodes, node{text: text.String()})
			text.Reset()
		}
	}
	for len(s) > 0 {
		c := s[0]
		switch {
		case c == '\'':
			if strings.HasPrefix(s, "''") {
				text.WriteByte('\'')
				s = s[2:]
				continue
			}
			end := strings.IndexByte(s[1:], '\'')
			if end < 0 {
				text.WriteString(s)
				s = ""
				continue
			}
			text.WriteString(s[1 : 1+end])
			s = s[2+end:]
		case c == '}':
			if nested {
				flush()
				return nodes, s, nil
			}
			text.WriteByte(c)
			s = s[1:]
		case c == '{':
			flush()
			end := strings.IndexAny(s, ",}")
			if end < 0 {
				return nil, "", fmt.Errorf("unclosed {")
			}
			name := strings.TrimSpace(s[1:end])
			if s[end] == '}' {
				nodes = append(nodes, node{arg: name})
				s = s[end+1:]
				continue
			}
			rest := s[end+1:]
			kindEnd := strings.IndexByte(rest, ',')
			if kindEnd < 0 || strings.TrimSpace(rest[:kindEnd]) != "plural" {
				return nil, "", fmt.Errorf("unsupported %q", s)
			}
			rest = strings.TrimLeft(rest[kindEnd+1:], " ")
			cases := map[string][]node{}
			for {
				rest = strings.TrimLeft(rest, " ")
				if strings.HasPrefix(rest, "}") {
					rest = rest[1:]
					break
				}
				open := strings.IndexByte(rest, '{')
				if open < 0 {
					return nil, "", fmt.Errorf("bad plural")
				}
				sel := strings.TrimSpace(rest[:open])
				body, after, err := parse(rest[open+1:], true)
				if err != nil || !strings.HasPrefix(after, "}") {
					return nil, "", fmt.Errorf("bad plural case")
				}
				cases[sel] = body
				rest = after[1:]
			}
			nodes = append(nodes, node{arg: name, plural: cases})
			s = rest
		default:
			text.WriteByte(c)
			s = s[1:]
		}
	}
	flush()
	return nodes, "", nil
}
