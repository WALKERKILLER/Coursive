package library

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type zeros struct{}

func (zeros) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

func TestPreviewPDFRendersTheSameBytesAndKeepsNothing(t *testing.T) {
	if runtime.GOOS == "darwin" {
		exe, _ := os.Executable()
		if _, e := os.Stat(filepath.Join(filepath.Dir(exe), "coursive-study-pdf")); e != nil {
			studyPDFDependencyMissing(t, "run compiled library.test beside the built PDFKit helper")
		}
	} else {
		requireStudyPoppler(t)
	}
	temp := t.TempDir()
	t.Setenv("TMPDIR", temp)
	source := filepath.Join(t.TempDir(), "export.pdf")
	writeStudyPDFFixture(t, source)
	b, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	out, e := PreviewPDF(t.Context(), bytes.NewReader(b), 2)
	if e != nil {
		t.Fatal(e)
	}
	if out.Page != 2 || out.Count != 2 || !strings.Contains(out.Text, "Second page learning") || !bytes.HasPrefix(out.PNG, []byte("\x89PNG")) || out.Width == 0 || out.Height == 0 {
		t.Fatal("page 2 not rendered from the given bytes", out.Page, out.Count, out.Text, len(out.PNG))
	}
	if left, _ := os.ReadDir(temp); len(left) != 0 {
		t.Fatal("preview left files behind", left)
	}
	if _, e = PreviewPDF(t.Context(), bytes.NewReader(b), 3); !errors.Is(e, ErrStudyLimit) {
		t.Fatal("page past the end rendered", e)
	}
	if _, e = PreviewPDF(t.Context(), strings.NewReader("<html>"), 1); !errors.Is(e, ErrStudyInvalid) {
		t.Fatal("non-PDF accepted", e)
	}
	if _, e = PreviewPDF(t.Context(), io.MultiReader(strings.NewReader("%PDF-1.7\n"), io.LimitReader(zeros{}, PDFPreviewLimit)), 1); !errors.Is(e, ErrStudyLimit) {
		t.Fatal("oversized PDF accepted", e)
	}
}
