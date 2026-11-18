package app

import (
	"strings"
	"testing"
)

// A typical model-written note, as coursive_markdown_test.dart's.
const testNote = `### 静压强特性

- 斜面 $ABC$ 面积 $S$，三个坐标面面积 $S_x,\ S_y,\ S_z$ [01:35]：
  $$S_x = S\cos\alpha,\quad S_y = S\cos\beta$$
- 代入 $V=\tfrac13 S_x\,dx$，取极限 $dx\to 0$ 得 $p_x = p_n$。

速度 $ v = \frac{dx}{dt} $ 与区间 $[0:10]$ 无关，见 [12:03]–[13:40]。

\begin{aligned}
dp &= \rho\left(f_x dx + f_y dy + f_z dz\right) \\
dW &= f_x dx + f_y dy + f_z dz
\end{aligned}

梯度 \( \nabla p \) 与 \[ \vec f = \dfrac{1}{\rho}\nabla p \]

**结论：$p=p(x,y,z)$**，价格 $5 和 $10 不是公式。

` + "```python\ncost = \"$5\"  # [00:10] stays code\n```\n"

func TestPrepareMarkdown(t *testing.T) {
	out := PrepareMarkdown(testNote, "")
	for _, want := range []string{
		`$v = \frac{dx}{dt}$`,
		"$$\n\\begin{aligned}",
		"[01:35](coursive://seek?t=95)",
		"[12:03–13:40](coursive://seek?t=723&end=820)",
		`$[0:10]$`,
		"# [00:10] stays code",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	if got := PrepareMarkdown(`价格 $ 5 + $ 10`, ""); got != `价格 $ 5 + $ 10` {
		t.Errorf("prices changed: %q", got)
	}
	if got := LinkifyTimestamps("at [1:02:03] and [00:05](x)", "L1"); got != "at [1:02:03](coursive://lecture/L1?t=3723) and [00:05](x)" {
		t.Errorf("linkify: %q", got)
	}
}

func TestParseRef(t *testing.T) {
	if r, ok := ParseRef("coursive://lecture/L1"); !ok || r.Kind != RefLecture || r.LectureID != "L1" {
		t.Errorf("lecture: %+v", r)
	}
	if r, ok := ParseRef("coursive://lecture/L1?t=30"); !ok || r.Kind != RefTime || r.Seconds != 30 {
		t.Errorf("time: %+v", r)
	}
	if r, ok := ParseRef("coursive://file/课程/a%20b.png"); !ok || r.Kind != RefFile || r.Path != "课程/a b.png" {
		t.Errorf("file: %+v", r)
	}
	if r, ok := ParseRef("coursive-app://lecture/L1?t=723"); !ok || r.Kind != RefTime || r.LectureID != "L1" || r.Seconds != 723 {
		t.Errorf("app scheme: %+v", r)
	}
	if r, ok := ParseRef("coursive://slide/L2/4"); !ok || r.Kind != RefSlide || r.Idx != 4 {
		t.Errorf("slide: %+v", r)
	}
	for _, bad := range []string{"coursive://nothing/x", "https://example.com", "https://example.com/?t=1"} {
		if _, ok := ParseRef(bad); ok {
			t.Errorf("%s parsed", bad)
		}
	}
}

func TestChipLinks(t *testing.T) {
	got := chipLinks("打开 [第 3 节](coursive://lecture/L3)，[$x$](coursive://seek?t=1) [未知](coursive://nothing/x)", "")
	want := "打开 [$" + chipTeX + "l 第 3 节$](coursive://lecture/L3)，[$" + chipTeX + "t x$](coursive://seek?t=1) [未知](coursive://nothing/x)"
	if got != want {
		t.Errorf("chips:\n%s\n%s", got, want)
	}
}

func TestMdDoc(t *testing.T) {
	d := NewMdDoc("intro\n# One\nalpha\n```\n# not\n```\n## Two *b*\nBeta [x](y)\n")
	if len(d.Sections) != 3 || len(d.Headings) != 2 {
		t.Fatalf("sections %d headings %d", len(d.Sections), len(d.Headings))
	}
	if h := d.Headings[1]; h.Level != 2 || h.Title != "Two b" || h.Section != 2 {
		t.Errorf("heading: %+v", h)
	}
	if hits := d.Find("BETA"); len(hits) != 1 || hits[0] != 2 {
		t.Errorf("find: %v", hits)
	}
	if hits := d.Find("not"); len(hits) != 1 || hits[0] != 1 {
		t.Errorf("fenced heading split: %v", hits)
	}
}
