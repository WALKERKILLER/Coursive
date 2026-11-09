package main

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode/utf16"
)

// The Go native baseline: the pilot tasks of
// internal/moejsexec/testdata/pilot/index.js ported to Go, giving the same
// results on the same input (native_test.go checks them against the golden
// file Node and MoeJS are checked against).

var fillers = []string{"就是说", "那个", "然后", "嗯", "呃", "啊"}

// jsSpace is the whitespace String.prototype.trim removes.
func jsSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0xa0, 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000, 0xfeff:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

var spaces = regexp.MustCompile(` {2,}`)

func fold(text string) string {
	var b strings.Builder
	for _, r := range text {
		switch {
		case r >= 0xff01 && r <= 0xff5e:
			b.WriteRune(r - 0xfee0)
		case r == 0x3000 || r == '\t' || r == '\n' || r == '\r':
			b.WriteByte(' ')
		default:
			b.WriteRune(r)
		}
	}
	return strings.TrimFunc(spaces.ReplaceAllString(b.String(), " "), jsSpace)
}

func dropFillers(t string) string {
	for changed := true; changed; {
		changed = false
		for _, f := range fillers {
			if strings.HasPrefix(t, f) {
				t = strings.TrimLeft(t[len(f):], "，,、 ")
				changed = true
			}
		}
	}
	return t
}

func word(b byte) bool {
	return b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b >= '0' && b <= '9'
}

func join(a, b string) string {
	if a == "" {
		return b
	}
	if b != "" && word(a[len(a)-1]) && word(b[0]) {
		return a + " " + b
	}
	return a + b
}

// jsLength is String.prototype.length: UTF-16 code units.
func jsLength(s string) int { return len(utf16.Encode([]rune(s))) }

func sentenceEnd(s string) bool {
	for _, end := range []string{"。", "！", "？", ".", "!", "?"} {
		if strings.HasSuffix(s, end) {
			return true
		}
	}
	return false
}

func num(v any, fallback float64) float64 {
	if f, ok := v.(float64); ok {
		return f
	}
	return fallback
}

func normalizeSubtitles(input map[string]any) any {
	gapMs, maxChars := num(input["gapMs"], 1500), num(input["maxChars"], 120)
	type cue struct {
		start, end float64
		text       string
	}
	var cues []*cue
	repeated := 0
	raw, _ := input["cues"].([]any)
	for _, c := range raw {
		m := c.(map[string]any)
		text := dropFillers(fold(m["text"].(string)))
		if text == "" {
			continue
		}
		if n := len(cues); n > 0 && cues[n-1].text == text {
			cues[n-1].end = m["end"].(float64)
			repeated++
			continue
		}
		cues = append(cues, &cue{m["start"].(float64), m["end"].(float64), text})
	}
	paragraphs := []map[string]any{}
	for _, c := range cues {
		if n := len(paragraphs); n > 0 {
			p := paragraphs[n-1]
			text := p["text"].(string)
			if c.start-p["end"].(float64) <= gapMs && float64(jsLength(text)+jsLength(c.text)) <= maxChars && !(sentenceEnd(text) && jsLength(text) >= 40) {
				p["text"], p["end"], p["cues"] = join(text, c.text), c.end, p["cues"].(int)+1
				continue
			}
		}
		paragraphs = append(paragraphs, map[string]any{"start": c.start, "end": c.end, "text": c.text, "cues": 1})
	}
	return map[string]any{"paragraphs": paragraphs, "stats": map[string]any{
		"cues": len(raw), "kept": len(cues), "repeated": repeated, "paragraphs": len(paragraphs)}}
}

func groupNotes(input map[string]any) any {
	notes := append([]any(nil), input["notes"].([]any)...)
	sort.SliceStable(notes, func(i, j int) bool {
		a, b := notes[i].(map[string]any), notes[j].(map[string]any)
		if a["updated"] != b["updated"] {
			return a["updated"].(float64) > b["updated"].(float64)
		}
		return a["id"].(string) < b["id"].(string)
	})
	var order []string
	groups := map[string][]any{}
	untagged := []any{}
	for _, n := range notes {
		note := n.(map[string]any)
		seen := map[string]bool{}
		var tags []string
		raw, _ := note["tags"].([]any)
		for _, t := range raw {
			tag := strings.ToLower(fold(fmt.Sprint(t)))
			if tag != "" && !seen[tag] {
				seen[tag] = true
				tags = append(tags, tag)
			}
		}
		if len(tags) == 0 {
			untagged = append(untagged, note["id"])
		}
		title, _ := note["title"].(string)
		for _, tag := range tags {
			if _, ok := groups[tag]; !ok {
				order = append(order, tag)
			}
			groups[tag] = append(groups[tag], map[string]any{"id": note["id"], "title": fold(title)})
		}
	}
	byTag := make([]map[string]any, 0, len(order))
	for _, tag := range order {
		byTag = append(byTag, map[string]any{"tag": tag, "count": len(groups[tag]), "notes": groups[tag]})
	}
	sort.SliceStable(byTag, func(i, j int) bool {
		a, b := byTag[i], byTag[j]
		if a["count"] != b["count"] {
			return a["count"].(int) > b["count"].(int)
		}
		return a["tag"].(string) < b["tag"].(string)
	})
	return map[string]any{"byTag": byTag, "untagged": untagged}
}

var todoLine = regexp.MustCompile(`^\s*(TODO|待办)[:：]`)

func onEvent(input map[string]any) any {
	p, _ := input["payload"].(map[string]any)
	switch input["type"] {
	case "lecture.progress":
		percent, rewinds := num(p["percent"], math.NaN()), num(p["rewinds"], math.NaN())
		if percent >= 90 {
			return map[string]any{"action": "tag", "tags": []string{"watched"}, "reason": fmt.Sprintf("看完 %d%%", int(math.Floor(percent+0.5)))}
		}
		if percent >= 50 && rewinds >= 3 {
			return map[string]any{"action": "remind", "tags": []string{"review"}, "reason": "反复回看，建议复习"}
		}
	case "note.saved":
		text, _ := p["text"].(string)
		todos := 0
		for _, line := range strings.Split(text, "\n") {
			if todoLine.MatchString(line) {
				todos++
			}
		}
		if todos > 0 {
			return map[string]any{"action": "remind", "tags": []string{"todo"}, "reason": fmt.Sprintf("%d 条待办", todos)}
		}
	}
	return map[string]any{"action": "ignore"}
}

var native = map[string]func(map[string]any) any{
	"normalizeSubtitles": normalizeSubtitles,
	"groupNotes":         groupNotes,
	"onEvent":            onEvent,
}
