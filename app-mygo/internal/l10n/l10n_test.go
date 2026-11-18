package l10n

import (
	"bytes"
	"os"
	"testing"

	"github.com/walkerkiller/coursive/app-mygo/internal/l10n/gen"
)

// TestGeneratedFromTheARB keeps strings_gen.go what the ARB file says.
func TestGeneratedFromTheARB(t *testing.T) {
	arb, err := os.ReadFile("../../../app/lib/l10n/app_zh.arb")
	if err != nil {
		t.Skip("the Flutter client's ARB file is not beside this module")
	}
	want, err := gen.Generate(arb)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile("strings_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("strings_gen.go is stale; run go generate ./internal/l10n")
	}
}

func TestMessages(t *testing.T) {
	cases := map[string]string{
		DurationHoursMinutes(1, 30):    "1 小时 30 分",
		StudyThreadTurns(0):            "还没有提问",
		StudyThreadTurns(3):            "3 轮问答",
		LocalCourseSubmitWithVideos(2): "创建并导入 2 个视频",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
	if want := "适合学校、实验室或自建的 Hugging Face 兼容镜像。填写地址模板，{repo} 会换成模型仓库名，{file} 换成文件路径，例如 https://hf-mirror.com/{repo}/resolve/main/{file}。"; AsrSourceCustom != want {
		t.Errorf("escaping: %q", AsrSourceCustom)
	}
}
