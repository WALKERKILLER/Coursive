package library

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
)

// PDFPreviewLimit bounds an exported document sent back for preview.
const PDFPreviewLimit = 40 << 20

// PDFPreview is one rendered page of a PDF the app generated, with the text
// its text layer yields, so the preview shows the bytes that will be saved.
type PDFPreview struct {
	Page   int    `json:"page"`
	Count  int    `json:"count"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Text   string `json:"text"`
	PNG    []byte `json:"png"`
}

// PreviewPDF renders page of the PDF read from r in a private temporary
// directory with the same renderer as study canvas pages. Nothing is kept.
func PreviewPDF(ctx context.Context, r io.Reader, page int) (PDFPreview, error) {
	var out PDFPreview
	b, e := io.ReadAll(io.LimitReader(r, PDFPreviewLimit+1))
	if e != nil {
		return out, e
	}
	if len(b) > PDFPreviewLimit {
		return out, ErrStudyLimit
	}
	if !bytes.HasPrefix(b, []byte("%PDF-")) {
		return out, ErrStudyInvalid
	}
	temp, e := os.MkdirTemp("", "coursive-pdf-preview-")
	if e != nil {
		return out, e
	}
	defer os.RemoveAll(temp)
	source := filepath.Join(temp, "export.pdf")
	if e = os.WriteFile(source, b, 0o600); e != nil {
		return out, e
	}
	p, e := renderStudyPage(ctx, source, temp, page)
	if e != nil {
		return out, e
	}
	image, e := os.ReadFile(filepath.Join(temp, p.File))
	if e != nil {
		return out, e
	}
	if len(image) > 8<<20 {
		return out, ErrStudyLimit
	}
	return PDFPreview{Page: page, Count: p.Count, Width: p.Width, Height: p.Height, Text: p.Text, PNG: image}, nil
}
