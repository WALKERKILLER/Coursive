package library

import (
	"bytes"
	"context"
	"encoding/json"
	"image/png"
	"io"
	"os"
)

const StudyAttachmentLimit = 3 << 20

// Selection is frozen before the sidebar changes the canvas viewport. Sources
// describe provenance, never authority to read arbitrary files.
type StudySnapshotSelection struct {
	Bounds  []float64     `json:"bounds"`
	Polygon [][]float64   `json:"polygon,omitempty"`
	Sources []StudyAnchor `json:"sources"`
}
type StudyAttachment struct {
	ID        string                 `json:"id"`
	Resource  string                 `json:"resource"`
	SHA256    string                 `json:"sha256"`
	MimeType  string                 `json:"mime_type"`
	Width     int                    `json:"width"`
	Height    int                    `json:"height"`
	Selection StudySnapshotSelection `json:"selection"`
}

func readStudyAttachment(root *os.Root, dir, id string) (StudyAttachment, []byte, error) {
	var a StudyAttachment
	if !validStudyID(id) {
		return a, nil, ErrStudyInvalid
	}
	base := dir + "/study-assets/attachments/" + id
	if checkedStudyDir(root, dir+"/study-assets/attachments", false) != nil {
		return a, nil, ErrStudyInvalid
	}
	for _, name := range []string{base + ".json", base + ".png"} {
		st, e := root.Lstat(name)
		if e != nil || !st.Mode().IsRegular() || st.Size() > StudyAttachmentLimit {
			return a, nil, ErrStudyInvalid
		}
	}
	meta, e := root.ReadFile(base + ".json")
	if e != nil {
		return a, nil, e
	}
	if json.Unmarshal(meta, &a) != nil || a.ID != id || a.Resource != base+".png" {
		return a, nil, ErrStudyInvalid
	}
	b, e := root.ReadFile(a.Resource)
	if e != nil {
		return a, nil, e
	}
	if studyHash(string(b)) != a.SHA256 {
		return a, nil, ErrStudyConflict
	}
	return a, b, nil
}
func (l *Library) StudyAttachment(ctx context.Context, lecture, id string) (StudyAttachment, []byte, error) {
	l.studyMu.Lock()
	defer l.studyMu.Unlock()
	root, dir, e := l.studyRoot(ctx, lecture)
	if e != nil {
		return StudyAttachment{}, nil, e
	}
	defer root.Close()
	return readStudyAttachment(root, dir, id)
}
func (l *Library) SaveStudyAttachment(ctx context.Context, lecture string, r io.Reader, selection StudySnapshotSelection) (StudyAttachment, error) {
	var a StudyAttachment
	b, e := io.ReadAll(io.LimitReader(r, StudyAttachmentLimit+1))
	if e != nil {
		return a, e
	}
	if len(b) > StudyAttachmentLimit {
		return a, ErrStudyLimit
	}
	config, e := png.DecodeConfig(bytes.NewReader(b))
	if e != nil || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 12_000_000 {
		return a, ErrStudyInvalid
	}
	if _, e = png.Decode(bytes.NewReader(b)); e != nil {
		return a, ErrStudyInvalid
	}
	if len(selection.Bounds) != 4 || selection.Bounds[2] <= 0 || selection.Bounds[3] <= 0 || len(selection.Polygon) > 4096 || len(selection.Sources) > 24 {
		return a, ErrStudyInvalid
	}
	for _, p := range selection.Polygon {
		if len(p) != 2 {
			return a, ErrStudyInvalid
		}
	}
	for _, s := range selection.Sources {
		if s.WorkspaceID != "" && s.WorkspaceID != lecture {
			return a, ErrStudyInvalid
		}
	}
	meta, _ := json.Marshal(selection)
	if len(meta) > 64<<10 {
		return a, ErrStudyLimit
	}
	l.studyMu.Lock()
	defer l.studyMu.Unlock()
	root, dir, e := l.studyRoot(ctx, lecture)
	if e != nil {
		return a, e
	}
	defer root.Close()
	dir += "/study-assets/attachments"
	if e = checkedStudyDir(root, dir, true); e != nil {
		return a, e
	}
	a = StudyAttachment{ID: studyID(), SHA256: studyHash(string(b)), MimeType: "image/png", Width: config.Width, Height: config.Height, Selection: selection}
	a.Resource = dir + "/" + a.ID + ".png"
	if e = writeStudyBytes(root, a.Resource, b); e != nil {
		return a, e
	}
	meta, _ = json.Marshal(a)
	e = writeStudyBytes(root, dir+"/"+a.ID+".json", meta)
	return a, e
}
func validateStudyAttachments(root *os.Root, dir string, attachments []StudyAttachment) error {
	if len(attachments) > 2 {
		return ErrStudyLimit
	}
	seen := map[string]bool{}
	for _, a := range attachments {
		actual, _, e := readStudyAttachment(root, dir, a.ID)
		if e != nil {
			return e
		}
		left, _ := json.Marshal(actual)
		right, _ := json.Marshal(a)
		if seen[a.ID] || !bytes.Equal(left, right) {
			return ErrStudyInvalid
		}
		seen[a.ID] = true
	}
	return nil
}
