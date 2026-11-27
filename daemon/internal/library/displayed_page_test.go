package library

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestMatchSnapshotToDistinctPDFPage(t *testing.T) {
	const title = "Virtual addressing and relocation"
	const sentence = "Translate a logical address using the base register"
	const unrelated = "The teacher is still discussing the previous page"
	const raw = title + "\n" + sentence
	index := pageIndex{{Type: "text", Text: title, Page: 2}, {Type: "text", Text: sentence, Page: 2}, {Type: "text", Text: unrelated, Page: 3}, {Type: "text", Text: title, Page: 4}}
	got := visiblePageScores(raw, index)
	if len(got[3]) != 2 || len(got[4]) != 0 || len(got[5]) != 1 {
		t.Fatalf("unexpected matches: %v", got)
	}
}

func TestDisplayedPageAmbiguityAndRevision(t *testing.T) {
	l, _ := reviewFixture(t)
	ctx := context.Background()
	asset, err := l.ImportCourseware(ctx, "one", "input.pdf", strings.NewReader("%PDF-original"), 1024)
	if err != nil {
		t.Fatal(err)
	}
	parsed := ParsedDir(asset.Path)
	if err = os.MkdirAll(l.Abs(parsed), 0700); err != nil {
		t.Fatal(err)
	}
	const blocks = `[{"type":"text","text":"Shared heading for this chapter","page_idx":1},{"type":"text","text":"Another uniquely visible sentence about this page","page_idx":1}]`
	index := l.Abs(parsed + "/deck_content_list.json")
	if err = os.WriteFile(index, []byte(blocks), 0600); err != nil {
		t.Fatal(err)
	}
	if err = l.SetParse(ctx, asset, &ParseInfo{State: "done", Pages: 4, MD: parsed + "/full.md"}); err != nil {
		t.Fatal(err)
	}
	const ocr = "Shared heading for this chapter\nAnother uniquely visible sentence about this page"
	page, err := l.DisplayedPDFPage(ctx, "one", ocr)
	if err != nil || page == nil || page.Page != 2 || len(page.Revision) != 64 {
		t.Fatalf("match=%v err=%v", page, err)
	}
	if err = os.WriteFile(l.Abs(page.File), []byte("%PDF-changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = l.PreviewClassroomPage(ctx, "one", page.File, page.Revision, 2); err == nil {
		t.Fatal("changed PDF accepted")
	}
	ambiguous := strings.TrimSuffix(blocks, "]") + "," + strings.ReplaceAll(blocks[1:], "page_idx\":1", "page_idx\":2")
	if err = os.WriteFile(index, []byte(ambiguous), 0600); err != nil {
		t.Fatal(err)
	}
	match, err := l.DisplayedPDFPage(ctx, "one", ocr)
	if err != nil || match != nil {
		t.Fatalf("ambiguous match %v (%v)", match, err)
	}
}
