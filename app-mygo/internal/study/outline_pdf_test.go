package study

import (
	"bytes"
	"image"
	"image/color"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/walkerkiller/coursive/app-mygo/internal/pdf"
)

func outlineFonts(t *testing.T) (*pdf.Font, *pdf.Font) {
	t.Helper()
	var fonts []*pdf.Font
	for _, name := range []string{"Regular", "Bold"} {
		b, err := os.ReadFile("../assets/fonts/NotoSansSC-" + name + ".ttf")
		if err != nil {
			t.Fatal(err)
		}
		f, err := pdf.ParseTTF(b)
		if err != nil {
			t.Fatal(err)
		}
		fonts = append(fonts, f)
	}
	return fonts[0], fonts[1]
}

func sampleOutline() OutlineDoc {
	var body []OutlineBlock
	for s := 1; s <= 4; s++ {
		body = append(body, OutlineHeading{2, "第" + string(rune('0'+s)) + "节 示例章节"}, OutlineRemark{"一段说明文字。"})
		body = append(body,
			OutlineEntry{Text: "牛顿第二定律 $F = ma$ 说明 **合力** 与加速度成正比，`code` 也行。", Refs: []int{1, 2}},
			OutlineEntry{Text: "推导如下：\n$$\\int_0^1 x^2\\,dx = \\frac13$$\n然后得出结论。", Refs: []int{2}, Tag: "未核实"},
			OutlineEntry{Text: "```go\nfunc main() {\n\tfmt.Println(\"你好\")\n}\n```", Refs: []int{3}},
			OutlineEntry{Text: "| 量 | 单位 |\n|---|---|\n| 力 | N |\n| 质量 | kg |"},
		)
	}
	body = append(body, OutlineEntry{Text: strings.Repeat("很长的一段文字，用来测试跨页续排。Long English words wrap at spaces too. ", 30)})
	return OutlineDoc{
		Title: "示例大纲 Sample Outline",
		Body:  body,
		Appendix: []OutlineBlock{OutlineHeading{2, "待核对"}, OutlineHeading{3, "冲突"},
			OutlineEntry{Text: "一处冲突", Tag: "待查", Item: M{"id": "m1"}}},
		Sources: []OutlineSource{
			{Number: 1, Label: "lecture.mp4", Locator: "01:02–01:30", Quote: "一段引用", Link: "coursive://lecture/x?t=62"},
			{Number: 2, Label: "slides.pdf", Locator: "第 3 页", Stale: true},
			{Number: 3, Label: "notes.md"},
		},
	}
}

func TestOutlinePdf(t *testing.T) {
	regular, bold := outlineFonts(t)
	doc := sampleOutline()
	if missing := OutlineMissingGlyphs(doc, regular.Has); len(missing) > 0 {
		t.Fatalf("missing %q", missing)
	}
	ink := image.NewRGBA(image.Rect(0, 0, 96, 72))
	for i := range 72 {
		ink.Set(i, i, color.RGBA{200, 0, 0, 255})
	}
	images := map[string]OutlineImage{OutlineInkKey(M{"id": "m1"}): {Image: ink, W: 96, H: 72, Baseline: -1}}
	out := BuildOutlinePdf(doc, OutlineLayout{}, regular, bold, images, time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))
	if !bytes.HasPrefix(out, []byte("%PDF-1.7")) || len(out) > 400_000 {
		t.Fatalf("%d bytes", len(out))
	}
	if path := os.Getenv("OUTLINE_PDF"); path != "" {
		os.WriteFile(path, out, 0o644)
	}
}

func TestOutlineTextBlocks(t *testing.T) {
	got := OutlineTextBlocks("a\nb\n\n```\nx\n```\n$$\ny\n$$\n| h | k |\n|---|---|\n| 1 | 2 |")
	if len(got) != 4 {
		t.Fatalf("%#v", got)
	}
	if p := got[0].(OutlineParagraph); p.Text != "a\nb" {
		t.Fatal(p)
	}
	if m := got[2].(OutlineMath); m.Tex != "y" {
		t.Fatal(m)
	}
	if tb := got[3].(OutlineTable); len(tb.Rows) != 2 || tb.Rows[1][1] != "2" {
		t.Fatal(tb)
	}
}
