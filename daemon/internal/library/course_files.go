package library

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/walkerkiller/coursive/daemon/internal/store"
)

const courseBatchDir = ".course-material-batches"
const courseInbox = "导入"
const courseArchive = "归档"
const courseTextLimit = 512 << 10

type CourseFileEdit struct {
	Op        string `json:"op"`
	Path      string `json:"path"`
	Version   string `json:"version"`
	LectureID string `json:"lecture_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Old       string `json:"old,omitempty"`
	New       string `json:"new,omitempty"`
}

type CourseFileBatch struct {
	ID       string   `json:"id"`
	CourseID string   `json:"course_id"`
	State    string   `json:"state"`
	Lines    []string `json:"lines"`
	Lectures []string `json:"lectures,omitempty"`
}

type courseFileChange struct {
	From, To, Version, AfterVersion string
	BeforeAsset, AfterAsset         *store.Asset
	NoteID                          string
	OldBody, NewBody                string
	OldText, NewText                []byte
	ParseTree                       bool
}

type courseFileReceipt struct {
	CourseFileBatch
	Created int64
	Changes []courseFileChange
}

var courseBatchID = regexp.MustCompile(`^\d{8}-[0-9a-f]{12}$`)

func fileProblem(message string) error { return fmt.Errorf("%w：%s", ErrStudyInvalid, message) }
func fileChanged(rel string) error {
	return fmt.Errorf("%w：资料已变化，请重新读取后再试（%s）", ErrStudyConflict, rel)
}

func hasCleanCoursePath(course store.Course, rel string) bool {
	if course.Dir == "" || path.Clean(rel) != rel || strings.Contains(rel, "\\") || !strings.HasPrefix(rel, course.Dir+"/") {
		return false
	}
	for _, part := range strings.Split(rel, "/") {
		if strings.HasPrefix(part, ".") {
			return false
		}
	}
	return true
}

func (l *Library) courseFiles(ctx context.Context, courseID string) (store.Course, []CatalogItem, error) {
	if strings.TrimSpace(courseID) == "" {
		return store.Course{}, nil, fileProblem("请指定课程")
	}
	c, err := l.Store.Course(ctx, courseID)
	if err != nil {
		return c, nil, err
	}
	items, err := l.CourseCatalog(ctx, courseID)
	return c, items, err
}

func (l *Library) ListCourseFiles(ctx context.Context, courseID, query string, offset, limit int) (any, error) {
	course, items, err := l.courseFiles(ctx, courseID)
	if err != nil {
		return nil, err
	}
	if offset < 0 || len(query) > 400 {
		return nil, ErrStudyInvalid
	}
	if limit <= 0 || limit > 50 {
		limit = 35
	}
	query = strings.ToLower(strings.TrimSpace(query))
	filtered := make([]CatalogItem, 0, len(items))
	for _, item := range items {
		if hasCleanCoursePath(course, item.Path) && (query == "" || strings.Contains(strings.ToLower(item.Name+" "+item.Path), query)) {
			filtered = append(filtered, item)
		}
	}
	offset = min(offset, len(filtered))
	end := min(offset+limit, len(filtered))
	excerpt := func() (any, error) {
		ids := []string{}
		lectures, err := l.Store.Lectures(ctx, courseID)
		if err != nil {
			return nil, err
		}
		for _, lec := range lectures {
			ids = append(ids, lec.ID)
		}
		if len(ids) == 0 {
			return nil, nil
		}
		return l.SearchCourseware(ctx, ids, query, 8)
	}
	result := map[string]any{"files": filtered[offset:end], "total": len(filtered), "next_offset": end}
	if course.Dir != "" {
		result["inbox"] = course.Dir + "/" + courseInbox
	}
	if query != "" && offset == 0 {
		hits, err := excerpt()
		if err != nil {
			return nil, err
		}
		indexHits, err := l.Store.Search(ctx, query, courseID, 8)
		if err != nil {
			return nil, err
		}
		result["courseware_hits"] = hits
		result["indexed_hits"] = indexHits
	}
	return result, nil
}

func courseFileOpen(root *os.Root, rel string) (*os.File, error) {
	if err := checkedStudyDir(root, path.Dir(rel), false); err != nil {
		return nil, err
	}
	st, err := root.Lstat(rel)
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, ErrStudyInvalid
	}
	f, err := root.Open(rel)
	if err != nil {
		return nil, err
	}
	current, err := f.Stat()
	if err != nil || !os.SameFile(st, current) {
		f.Close()
		return nil, fileChanged(rel)
	}
	return f, nil
}

func courseFileVersion(root *os.Root, rel string) (string, int64, error) {
	f, err := courseFileOpen(root, rel)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	return fmt.Sprintf("%x", h.Sum(nil)), n, err
}

func materialHash(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }

func (l *Library) ReadCourseFile(ctx context.Context, courseID, rel string, offset int) (any, error) {
	course, items, err := l.courseFiles(ctx, courseID)
	if err != nil {
		return nil, err
	}
	if offset < 0 || !hasCleanCoursePath(course, rel) {
		return nil, ErrStudyInvalid
	}
	item, found := lookupCourseFile(items, rel)
	if !found {
		return nil, fileProblem("找不到这份课程资料")
	}
	root, err := os.OpenRoot(l.Root)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	out := map[string]any{"file": item}
	if item.Bytes <= 128<<20 {
		version, _, err := courseFileVersion(root, rel)
		if err != nil {
			return nil, err
		}
		out["version"] = version
	}
	if item.LectureID != "" && item.Bytes <= 32<<20 {
		src, err := l.ReadStudyCourseSource(ctx, item.LectureID, StudySourceRead{Source: StudyCourseSource{Path: rel}, Offset: offset, Limit: 5000})
		if err != nil {
			return nil, err
		}
		out["source"] = src
	} else if item.FileType == "md" || item.FileType == "txt" || item.FileType == "csv" {
		b, err := readBoundedStudyFile(root, rel)
		if err != nil {
			return nil, err
		}
		rs := []rune(string(b))
		if offset > len(rs) {
			return nil, ErrStudyInvalid
		}
		end := min(offset+5000, len(rs))
		out["text"], out["next_offset"], out["more"] = string(rs[offset:end]), end, end < len(rs)
	}
	return out, nil
}

func lookupCourseFile(items []CatalogItem, rel string) (CatalogItem, bool) {
	for _, item := range items {
		if item.Path == rel {
			return item, true
		}
	}
	return CatalogItem{}, false
}

func (l *Library) PreviewCourseFiles(ctx context.Context, courseID string, edits []CourseFileEdit) (CourseFileBatch, error) {
	l.mirrorMu.Lock()
	defer l.mirrorMu.Unlock()
	course, items, err := l.courseFiles(ctx, courseID)
	if err != nil {
		return CourseFileBatch{}, err
	}
	if len(edits) == 0 || len(edits) > 24 {
		return CourseFileBatch{}, fileProblem("一批可整理 1–24 份资料")
	}
	root, err := os.OpenRoot(l.Root)
	if err != nil {
		return CourseFileBatch{}, err
	}
	defer root.Close()
	r := courseFileReceipt{
		CourseFileBatch: CourseFileBatch{ID: NewID(), CourseID: course.ID, State: "pending"},
		Created:         time.Now().Unix(),
	}
	seen := map[string]bool{}
	totalText := 0
	for _, e := range edits {
		if !hasCleanCoursePath(course, e.Path) || e.Version == "" {
			return CourseFileBatch{}, fileProblem("请先读取当前课程中的资料和版本")
		}
		item, ok := lookupCourseFile(items, e.Path)
		if !ok || item.Bytes > 128<<20 {
			return CourseFileBatch{}, fileProblem("只能整理目录中不超过 128 MiB 的资料")
		}
		c, line, err := l.planCourseFile(ctx, root, course, item, e)
		if err != nil {
			return CourseFileBatch{}, err
		}
		paths := []string{c.From}
		if c.To != c.From {
			paths = append(paths, c.To)
		}
		for _, p := range paths {
			key := strings.ToLower(p)
			if seen[key] {
				return CourseFileBatch{}, fileProblem("批次中同一源或目标路径出现多次")
			}
			seen[key] = true
		}
		totalText += len(c.OldText) + len(c.NewText)
		if totalText > 2<<20 {
			return CourseFileBatch{}, ErrStudyLimit
		}
		r.Changes = append(r.Changes, c)
		r.Lines = append(r.Lines, line)
		for _, a := range []*store.Asset{c.BeforeAsset, c.AfterAsset} {
			if a != nil && !slices.Contains(r.Lectures, a.LectureID) {
				r.Lectures = append(r.Lectures, a.LectureID)
			}
		}
	}
	if err := createCourseBatch(root, r); err != nil {
		return CourseFileBatch{}, err
	}
	return r.CourseFileBatch, nil
}

func (l *Library) planCourseFile(ctx context.Context, root *os.Root, course store.Course, item CatalogItem, e CourseFileEdit) (courseFileChange, string, error) {
	c := courseFileChange{From: item.Path, To: item.Path, Version: e.Version, AfterVersion: e.Version}
	hash, size, err := courseFileVersion(root, item.Path)
	if err != nil {
		return c, "", err
	}
	if hash != e.Version {
		return c, "", fileChanged(item.Path)
	}
	if e.Op == "edit_markdown" {
		err := l.planCourseMarkdown(ctx, root, course, item, e, &c)
		return c, "修改 " + item.Name + " · " + patchSnippet(e.Old) + " → " + patchSnippet(e.New), err
	}
	switch e.Op {
	case "import":
		if path.Dir(item.Path) != course.Dir+"/"+courseInbox {
			return c, "", fileProblem("请从本课的「导入」目录选取文件")
		}
	case "rename", "move", "archive":
		a, err := l.coursewareAt(ctx, item.LectureID, item.Path)
		if err != nil {
			return c, "", err
		}
		c.BeforeAsset = &a
	default:
		return c, "", fileProblem("不支持的整理操作")
	}
	lecture := e.LectureID
	if lecture == "" {
		lecture = item.LectureID
	}
	dest, err := l.Store.Lecture(ctx, lecture)
	if err != nil || dest.CourseID != course.ID || dest.Dir == "" || !hasCleanCoursePath(course, dest.Dir) {
		return c, "", fileProblem("请选择同一课程中已有本地资料目录的节次")
	}
	if (e.Op == "rename" || e.Op == "archive") && dest.ID != item.LectureID {
		return c, "", fileProblem("改名和归档应保留原节次，改变节次请使用移动")
	}
	ext := strings.ToLower(path.Ext(item.Path))
	if !CoursewareExts[ext] && ext != ".md" && ext != ".txt" {
		return c, "", fileProblem("该文件类型由原有下载或播放流程管理")
	}
	name := e.Name
	if name == "" {
		name = path.Base(item.Path)
	}
	if name != path.Base(name) || name == "" || strings.HasPrefix(name, ".") ||
		badChars.MatchString(name) || Sanitize(strings.TrimSuffix(name, path.Ext(name)), "") == "" ||
		strings.ToLower(path.Ext(name)) != ext || len(name) > 240 {
		return c, "", fileProblem("文件名无效，扩展名必须保持一致")
	}
	folder := dest.Dir + "/courseware"
	if e.Op == "archive" || (e.Op == "rename" && path.Base(path.Dir(item.Path)) == courseArchive) {
		folder += "/" + courseArchive
	}
	c.To = folder + "/" + name
	if strings.EqualFold(c.From, c.To) {
		return c, "", fileProblem("新位置必须与原位置不同")
	}
	if err := vacantCoursePath(root, c.To); err != nil {
		return c, "", err
	}
	alias, err := l.Store.CoursewareAlias(ctx, c.To)
	if err == nil && (c.BeforeAsset == nil || alias.Path != c.BeforeAsset.Path) {
		return c, "", fileChanged(c.To)
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return c, "", err
	}
	meta := CoursewareMeta{Name: name, Uploaded: true}
	created := time.Now().Unix()
	if old := c.BeforeAsset; old != nil {
		meta, created = CoursewareInfo(*old), old.CreatedAt
		meta.Name = name
		meta.PreviousPaths = materialAliases(meta.PreviousPaths, c.From, c.To)
		if len(meta.PreviousPaths) > 64 {
			return c, "", fileProblem("该课件已保留 64 条旧路径，请从资料页处理")
		}
		if st, statErr := root.Lstat(ParsedDir(c.From)); statErr == nil {
			if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
				return c, "", ErrStudyInvalid
			}
			c.ParseTree = true
		} else if !os.IsNotExist(statErr) {
			return c, "", statErr
		}
		if meta.Parse != nil && meta.Parse.MD != "" {
			suffix, ok := strings.CutPrefix(meta.Parse.MD, ParsedDir(c.From)+"/")
			if !ok || !c.ParseTree {
				return c, "", fileProblem("课件解析目录不完整")
			}
			meta.Parse.MD = ParsedDir(c.To) + "/" + suffix
		}
	}
	if err := vacantCoursePath(root, ParsedDir(c.To)); err != nil {
		return c, "", err
	}
	metadata, err := json.Marshal(meta)
	c.AfterAsset = &store.Asset{LectureID: dest.ID, Kind: "courseware", Path: c.To, Bytes: size, CreatedAt: created, Meta: metadata}
	return c, c.From + " → " + c.To, err
}

func materialAliases(old []string, previous, active string) []string {
	out := slices.DeleteFunc(slices.Clone(old), func(p string) bool { return p == active || p == previous })
	return append(out, previous)
}

func vacantCoursePath(root *os.Root, rel string) error {
	_, err := root.Lstat(rel)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return fileChanged(rel)
}

func (l *Library) coursewareAt(ctx context.Context, lecture, rel string) (store.Asset, error) {
	items, err := l.Courseware(ctx, lecture)
	if err != nil {
		return store.Asset{}, err
	}
	for _, a := range items {
		if a.Path == rel {
			return a, nil
		}
	}
	return store.Asset{}, fileProblem("只可移动已登记的课件，录像和字幕由原来的流程管理")
}

func (l *Library) planCourseMarkdown(ctx context.Context, root *os.Root, course store.Course, item CatalogItem, edit CourseFileEdit, c *courseFileChange) error {
	protected := slices.Contains(strings.Split(item.Path, "/"), "study-assets")
	if item.FileType != "md" || item.Category == "transcript" || protected || item.Bytes > courseTextLimit ||
		edit.Old == "" || edit.Old == edit.New || !utf8.ValidString(edit.New) {
		return fileProblem("只支持对课程 Markdown 正文执行不同的精确替换")
	}
	b, err := readBoundedStudyFile(root, item.Path)
	if err != nil {
		return err
	}
	if materialHash(b) != c.Version || !utf8.Valid(b) || len(b) > courseTextLimit {
		return fileChanged(item.Path)
	}
	header, body := "", string(b)
	if item.NoteID != "" {
		n, err := l.Store.Note(ctx, item.NoteID)
		if err != nil || n.Path != item.Path || n.CourseID != course.ID {
			return fileChanged(item.Path)
		}
		_, rest, ok := strings.Cut(body, "\n---\n")
		if !ok || !strings.HasPrefix(body, "---\n") || strings.TrimLeft(rest, "\n") != n.Body {
			return fileChanged(item.Path)
		}
		header, body = strings.TrimSuffix(body, n.Body), n.Body
		c.NoteID = n.ID
	}
	// Prefer an exact match. A pasted LF patch also works on CRLF documents.
	old, replacement := edit.Old, edit.New
	if strings.Contains(body, "\r\n") {
		old = strings.ReplaceAll(strings.ReplaceAll(old, "\r\n", "\n"), "\n", "\r\n")
		replacement = strings.ReplaceAll(strings.ReplaceAll(replacement, "\r\n", "\n"), "\n", "\r\n")
	}
	if strings.Count(body, old) != 1 {
		return fileProblem("待替换原文必须在正文中恰好出现一次")
	}
	c.OldBody, c.NewBody = body, strings.Replace(body, old, replacement, 1)
	if c.NoteID != "" && strings.TrimLeft(c.NewBody, "\n") != c.NewBody {
		return fileProblem("笔记正文不能以空行开头")
	}
	c.OldText, c.NewText = b, []byte(header+c.NewBody)
	if len(c.NewText) > courseTextLimit {
		return ErrStudyLimit
	}
	c.AfterVersion = materialHash(c.NewText)
	return nil
}

func patchSnippet(text string) string {
	rs := []rune(strings.Join(strings.Fields(text), " "))
	if len(rs) > 90 {
		return string(rs[:90]) + "…"
	}
	if len(rs) == 0 {
		return "（空）"
	}
	return string(rs)
}

func courseReceiptPath(id string) string { return courseBatchDir + "/" + id + ".json" }

func createCourseBatch(root *os.Root, r courseFileReceipt) error {
	if err := root.Mkdir(courseBatchDir, 0700); err != nil && !os.IsExist(err) {
		return err
	}
	if info, err := root.Lstat(courseBatchDir); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ErrStudyInvalid
	}
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return writeStudyBytes(root, courseReceiptPath(r.ID), b)
}
