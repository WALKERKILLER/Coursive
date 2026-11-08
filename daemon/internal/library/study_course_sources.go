package library

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Offsets address Representation in runes, never PDF or Flutter UTF-16 indices.
// Page=0 deliberately means excerpt, not an inferred original page.
type StudyCourseSource struct {
	VideoPath      string `json:"video_path,omitempty"`
	ID             string `json:"id"`
	Path           string `json:"path"`
	LectureID      string `json:"lecture_id"`
	Lecture        string `json:"lecture"`
	Label          string `json:"label"`
	Kind           string `json:"kind"`
	Availability   string `json:"availability"`
	Representation string `json:"representation"`
	OffsetUnit     string `json:"offset_unit"`
	Offset         int    `json:"offset"`
	End            int    `json:"end"`
	NextOffset     int    `json:"next_offset,omitempty"`
	Fingerprint    string `json:"fingerprint,omitempty"`
	Snippet        string `json:"snippet,omitempty"`
	Text           string `json:"text,omitempty"`
	Page           int    `json:"page,omitempty"`
	TimeStartMs    int64  `json:"time_start_ms,omitempty"`
	TimeEndMs      int64  `json:"time_end_ms,omitempty"`
}
type StudySourceRead struct {
	Source StudyCourseSource `json:"source"`
	Offset int               `json:"offset"`
	Limit  int               `json:"limit"`
}

func RuneOffsetUTF16(text string, offset int) (int, error) {
	r := []rune(text)
	if offset < 0 || offset > len(r) {
		return 0, ErrStudyInvalid
	}
	return len(utf16.Encode(r[:offset])), nil
}
func catalogStudySource(it CatalogItem) StudyCourseSource {
	return StudyCourseSource{ID: studyHash(it.Path), Path: it.Path, LectureID: it.LectureID, Lecture: it.Lecture, Label: it.Name, Kind: it.FileType, Availability: "unparsed", OffsetUnit: "rune"}
}
func (l *Library) studyCourseCatalog(ctx context.Context, lecture string) ([]CatalogItem, error) {
	lec, e := l.Store.Lecture(ctx, lecture)
	if e != nil {
		return nil, e
	}
	return l.CourseCatalog(ctx, lec.CourseID)
}
func readBoundedStudyFile(root *os.Root, rel string) ([]byte, error) {
	if path.Clean(rel) != rel || checkedStudyDir(root, path.Dir(rel), false) != nil {
		return nil, ErrStudyInvalid
	}
	st, e := root.Lstat(rel)
	if e != nil || !st.Mode().IsRegular() {
		return nil, ErrStudyInvalid
	}
	if st.Size() > 32<<20 {
		return nil, ErrStudyLimit
	}
	f, e := root.Open(rel)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, (32<<20)+1))
}
func (l *Library) ReadStudyCourseSource(ctx context.Context, lecture string, q StudySourceRead) (StudyCourseSource, error) {
	var out StudyCourseSource
	if q.Offset < 0 || q.Limit < 0 {
		return out, ErrStudyInvalid
	}
	if q.Limit == 0 || q.Limit > 12000 {
		q.Limit = 12000
	}
	items, e := l.studyCourseCatalog(ctx, lecture)
	if e != nil {
		return out, e
	}
	var item *CatalogItem
	for i := range items {
		if items[i].Path == q.Source.Path {
			item = &items[i]
			break
		}
	}
	if item == nil {
		return out, ErrStudyInvalid
	}
	out = catalogStudySource(*item)
	root, e := os.OpenRoot(l.Root)
	if e != nil {
		return out, e
	}
	defer root.Close()
	original, e := readBoundedStudyFile(root, item.Path)
	if e != nil {
		return out, e
	}
	text := original
	var blocks pageIndex
	var pageBytes []byte
	switch item.FileType {
	case "txt", "md", "srt", "vtt", "csv", "c", "h", "py":
		out.Representation = "file_text"
	default:
		out.Representation = "parsed_markdown"
		found := false
		assets, e := l.Courseware(ctx, item.LectureID)
		if e != nil {
			return out, e
		}
		for _, a := range assets {
			if a.Path != item.Path {
				continue
			}
			m := CoursewareInfo(a)
			if m.Parse != nil && m.Parse.State == "done" && strings.HasPrefix(m.Parse.MD, ParsedDir(item.Path)+"/") {
				text, e = readBoundedStudyFile(root, m.Parse.MD)
				if directory, err := root.Open(path.Dir(m.Parse.MD)); err == nil {
					entries, _ := directory.ReadDir(-1)
					directory.Close()
					for _, entry := range entries {
						if strings.HasSuffix(entry.Name(), "content_list.json") {
							pageBytes, _ = readBoundedStudyFile(root, path.Dir(m.Parse.MD)+"/"+entry.Name())
							_ = json.Unmarshal(pageBytes, &blocks)
							break
						}
					}
				}
				if e != nil {
					return out, e
				}
				found = true
			}
		}
		if !found {
			return out, nil
		}
	}
	if !utf8.Valid(text) {
		return out, ErrStudyInvalid
	}
	out.Fingerprint = studyHash(studyHash(string(original)) + "/" + studyHash(string(text)) + "/" + studyHash(string(pageBytes)))
	if q.Source.Fingerprint != "" && q.Source.Fingerprint != out.Fingerprint {
		return out, ErrStudyConflict
	}
	out.Availability = "readable"
	r := []rune(string(text))
	if q.Offset > len(r) {
		return out, ErrStudyInvalid
	}
	end := min(q.Offset+q.Limit, len(r))
	out.Offset = q.Offset
	out.End = end
	out.Text = string(r[q.Offset:end])
	// Only a complete, unambiguous parse block may supply an exact page. Partial
	// prefixes and image filenames are never page evidence.
	exact := strings.TrimSpace(out.Text)
	matchedPage := 0
	ambiguous := false
	if len([]rune(exact)) >= 12 {
		for _, block := range blocks {
			if strings.TrimSpace(block.Text) == exact {
				if matchedPage != 0 && matchedPage != block.Page+1 {
					ambiguous = true
				}
				matchedPage = block.Page + 1
			}
		}
	}
	if !ambiguous {
		out.Page = matchedPage
	}
	if end < len(r) {
		out.NextOffset = end
	}
	// Bound every verified sentence in the excerpt, not only the search hit.
	// Repeated text cannot establish a unique transcript time.
	if item.Category == "transcript" {
		segs, e := l.Store.Segments(ctx, item.LectureID, 0, 0)
		if e != nil {
			return out, e
		}
		matched := false
		for _, seg := range segs {
			if seg.Text == "" || !strings.Contains(out.Text, seg.Text) || strings.Count(string(text), seg.Text) != 1 {
				continue
			}
			at := strings.Index(string(text), seg.Text)
			start := len([]rune(string(text[:at])))
			stop := start + len([]rune(seg.Text))
			if start < q.Offset || stop > end {
				continue
			}
			if !matched || seg.StartMs < out.TimeStartMs {
				out.TimeStartMs = seg.StartMs
			}
			out.TimeEndMs = max(out.TimeEndMs, seg.EndMs)
			matched = true
		}
		if matched {
			for _, candidate := range items {
				if candidate.LectureID == item.LectureID && candidate.Category == "video" {
					out.VideoPath = candidate.Path
					break
				}
			}
		}
	}
	if q.Source.Fingerprint != "" && q.Source.VideoPath != out.VideoPath {
		return out, ErrStudyConflict
	}
	return out, nil
}
func (l *Library) ResolveStudyCourseSource(ctx context.Context, lecture string, q StudySourceRead) (StudyCourseSource, error) {
	if q.Source.Fingerprint == "" || q.Source.End <= q.Source.Offset {
		return StudyCourseSource{}, ErrStudyInvalid
	}
	q.Offset = q.Source.Offset
	q.Limit = q.Source.End - q.Source.Offset
	if q.Limit > 12000 {
		return StudyCourseSource{}, ErrStudyInvalid
	}
	out, e := l.ReadStudyCourseSource(ctx, lecture, q)
	if e == nil && (out.Availability != "readable" || out.End != q.Source.End) {
		return out, ErrStudyConflict
	}
	return out, e
}
func (l *Library) SearchStudyCourseSources(ctx context.Context, lecture, query string) ([]StudyCourseSource, error) {
	if len([]rune(query)) > 500 {
		return nil, ErrStudyInvalid
	}
	lec, e := l.Store.Lecture(ctx, lecture)
	if e != nil {
		return nil, e
	}
	items, e := l.studyCourseCatalog(ctx, lecture)
	if e != nil {
		return nil, e
	}
	root, e := os.OpenRoot(l.Root)
	if e != nil {
		return nil, e
	}
	defer root.Close()
	byPath := map[string]CatalogItem{}
	for _, it := range items {
		byPath[it.Path] = it
	}
	out := []StudyCourseSource{}
	seen := map[string]bool{}
	add := func(s StudyCourseSource) {
		if len(out) < 12 && !seen[s.ID] {
			seen[s.ID] = true
			out = append(out, s)
		}
	}
	lectures, e := l.Store.Lectures(ctx, lec.CourseID)
	if e != nil {
		return nil, e
	}
	ids := []string{}
	for _, v := range lectures {
		ids = append(ids, v.ID)
	}
	if strings.TrimSpace(query) != "" {
		hits, e := l.SearchCourseware(ctx, ids, query, 12)
		if e != nil {
			return nil, e
		}
		for _, h := range hits {
			if it, ok := byPath[h.File]; ok {
				s := catalogStudySource(it)
				s.Offset = h.Offset
				s.Snippet = h.Snippet
				s.Availability = "parsed"
				s.Representation = "parsed_markdown"
				add(s)
			}
		}
		hits2, e := l.Store.Search(ctx, query, lec.CourseID, 12)
		if e != nil {
			return nil, e
		}
		for _, h := range hits2 {
			for _, it := range items {
				if (h.Kind == "segment" && it.LectureID == h.LectureID && it.Category == "transcript" && it.FileType == "md") || (h.Kind == "note" && it.NoteID == h.NoteID) {
					s := catalogStudySource(it)
					s.TimeStartMs = h.StartMs
					if h.Kind == "segment" {
						segs, _ := l.Store.Segments(ctx, it.LectureID, h.StartMs, h.StartMs+1)
						raw, err := readBoundedStudyFile(root, it.Path)
						if err != nil {
							continue
						}
						for _, seg := range segs {
							if seg.StartMs == h.StartMs {
								if at := strings.Index(string(raw), seg.Text); at >= 0 {
									s.Offset = len([]rune(string(raw[:at])))
								}
							}
						}
					}
					s.Snippet = h.Snippet
					s.Availability = "text"
					add(s)
				}
			}
		}
	}
	terms := searchTerms(query)
	for _, it := range items {
		if len(out) >= 12 {
			break
		}
		if strings.Contains(it.Path, "/study-assets/") {
			continue
		}
		match := len(terms) == 0
		for _, term := range terms {
			if strings.Contains(strings.ToLower(it.Name), term) {
				match = true
			}
		}
		s := catalogStudySource(it)
		switch it.FileType {
		case "txt", "md", "srt", "vtt", "csv", "c", "h", "py":
			b, err := readBoundedStudyFile(root, it.Path)
			if err != nil || !utf8.Valid(b) {
				continue
			}
			text := string(b)
			for _, term := range terms {
				if idx := strings.Index(strings.ToLower(text), term); idx >= 0 {
					match = true
					s.Offset = len([]rune(strings.ToLower(text)[:idx]))
					break
				}
			}
			s.Availability = "text"
			r := []rune(text)
			start := min(s.Offset, len(r))
			s.Snippet = string(r[start:min(start+350, len(r))])
		}
		if match {
			add(s)
		}
	}
	return out, nil
}

// Conversations may retain stale evidence; shape is validated on save and the
// actual file is revalidated on click. Never reject history just for aging.
func validateStudyCitations(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	var result struct {
		Citations []struct {
			ID     string            `json:"id"`
			Number int               `json:"number"`
			Label  string            `json:"label"`
			Source StudyCourseSource `json:"source"`
		} `json:"citations"`
	}
	if json.Unmarshal(raw, &result) != nil {
		return ErrStudyInvalid
	}
	if len(result.Citations) > 20 {
		return ErrStudyLimit
	}
	seen := map[string]bool{}
	for _, c := range result.Citations {
		s := c.Source
		if !validStudyID(c.ID) || seen[c.ID] || c.Number < 1 || len(c.Label) > 2000 || len(s.Fingerprint) != 64 || s.OffsetUnit != "rune" || s.Offset < 0 || s.End <= s.Offset || s.End-s.Offset > 12000 || (s.Text != "" && len([]rune(s.Text)) != s.End-s.Offset) || s.Path == "" || path.Clean(s.Path) != s.Path || strings.HasPrefix(s.Path, "/") || strings.HasPrefix(s.Path, "../") {
			return ErrStudyInvalid
		}
		seen[c.ID] = true
	}
	return nil
}
