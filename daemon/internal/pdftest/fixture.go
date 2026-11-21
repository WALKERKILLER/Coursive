// Package pdftest provides shared synthetic input for PDF integration tests.
package pdftest

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// A real two-page PDF with independent text streams; no model is used here.
func WriteTwoPagePDF(t *testing.T, p string) {
	t.Helper()
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R 5 0 R] /Count 2 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 7 0 R >> >> /Contents 4 0 R >>",
		"",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 792 612] /Resources << /Font << /F1 7 0 R >> >> /Contents 6 0 R >>",
		"",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}
	for i, text := range []string{"Canvas PDF first page", "Second page learning material"} {
		body := "BT /F1 24 Tf 72 500 Td (" + text + ") Tj ET\n"
		objects[3+2*i] = fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(body), body)
	}
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := []int{0}
	for i, obj := range objects {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&b, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	if err := os.WriteFile(p, b.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
}

// Only absent external dependencies skip integration tests. Strict renderer
// verification fails instead; conversion errors always remain test failures.
func studyPDFDependencyMissing(t *testing.T, message string) {
	t.Helper()
	if os.Getenv("COURSIVE_REQUIRE_PDF_TESTS") == "1" {
		t.Fatal(message)
	}
	t.Skip(message + "; set COURSIVE_REQUIRE_PDF_TESTS=1 to require renderer coverage")
}

func RequirePoppler(t *testing.T) {
	t.Helper()
	for _, tool := range []string{"pdfinfo", "pdftoppm", "pdftotext"} {
		if _, err := exec.LookPath(tool); err != nil {
			studyPDFDependencyMissing(t, "Poppler integration requires "+tool+" (install poppler-utils)")
		}
	}
}

func RequirePagedRenderer(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "darwin" {
		exe, _ := os.Executable()
		if _, e := os.Stat(filepath.Join(filepath.Dir(exe), "coursive-study-pdf")); e != nil {
			studyPDFDependencyMissing(t, "run compiled test binary beside the built PDFKit helper")
		}
	} else {
		RequirePoppler(t)
	}
}
