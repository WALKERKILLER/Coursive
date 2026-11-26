package library

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
	"unicode/utf8"
)

// DisplayedPage is a match against the text visible in a classroom snapshot.
// It carries no recording position. A teacher can discuss another page.
type DisplayedPage struct {
	File     string `json:"file"`
	Name     string `json:"name"`
	Page     int    `json:"page"`
	Text     string `json:"text"`
	Revision string `json:"revision"`
}

func visiblePageScores(ocr string, index pageIndex) map[int][]string {
	visible := squash(ocr)
	byPage := make(map[int][]string)
	for _, block := range index {
		if block.Page < 0 || block.Type != "text" {
			continue
		}
		text := strings.TrimSpace(block.Text)
		match := squash(text)
		if utf8.RuneCountInString(match) < 6 || !strings.Contains(visible, match) {
			continue
		}
		unique := true
		for _, prev := range byPage[block.Page+1] {
			if strings.Contains(squash(prev), match) || strings.Contains(match, squash(prev)) {
				unique = false
				break
			}
		}
		if unique {
			byPage[block.Page+1] = append(byPage[block.Page+1], text)
		}
	}
	return byPage
}

func (l *Library) DisplayedPDFPage(ctx context.Context, lecture, ocr string) (*DisplayedPage, error) {
	assets, err := l.Courseware(ctx, lecture)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(l.Root)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	var winner *DisplayedPage
	for _, asset := range assets {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		meta := CoursewareInfo(asset)
		if !strings.EqualFold(path.Ext(asset.Path), ".pdf") || meta.Parse == nil || meta.Parse.State != "done" {
			continue
		}
		dir := path.Dir(meta.Parse.MD)
		names, err := fs.Glob(root.FS(), path.Join(dir, "*content_list.json"))
		if err != nil || len(names) != 1 {
			continue
		}
		f, err := courseFileOpen(root, names[0])
		if err != nil {
			continue
		}
		const limit = 16 << 20
		data, err := io.ReadAll(io.LimitReader(f, limit+1))
		f.Close()
		var index pageIndex
		if err != nil || len(data) > limit || json.Unmarshal(data, &index) != nil {
			continue
		}
		for page, lines := range visiblePageScores(ocr, index) {
			// A shared heading or footer alone cannot identify a page.
			text := strings.Join(lines, "\n")
			if len(lines) < 2 || utf8.RuneCountInString(squash(text)) < 30 || page > meta.Parse.Pages && meta.Parse.Pages > 0 {
				continue
			}
			if winner != nil {
				return nil, nil
			}
			winner = &DisplayedPage{File: asset.Path, Name: meta.Name, Page: page, Text: text}
		}
	}
	if winner != nil {
		winner.Revision, _, err = courseFileVersion(root, winner.File)
		if err != nil {
			return nil, err
		}
	}
	return winner, nil
}

// PreviewClassroomPage reopens only an enrolled lecture asset and hashes the same bytes it renders.
func (l *Library) PreviewClassroomPage(ctx context.Context, lecture, file, revision string, page int) (PDFPreview, error) {
	var out PDFPreview
	asset, err := l.coursewareAt(ctx, lecture, file)
	if err != nil {
		return out, err
	}
	if page < 1 || page > 5000 || !strings.EqualFold(path.Ext(asset.Path), ".pdf") || len(revision) != 64 {
		return out, ErrStudyInvalid
	}
	root, err := os.OpenRoot(l.Root)
	if err != nil {
		return out, err
	}
	defer root.Close()
	f, err := courseFileOpen(root, asset.Path)
	if err != nil {
		return out, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, PDFPreviewLimit+1))
	if err != nil {
		return out, err
	}
	if len(data) > PDFPreviewLimit {
		return out, ErrStudyLimit
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != revision {
		return out, fileChanged(file)
	}
	return PreviewPDF(ctx, bytes.NewReader(data), page)
}
