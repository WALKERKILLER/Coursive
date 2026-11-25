package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/walkerkiller/coursive/daemon/internal/store"
)

func readCourseReceipt(root *os.Root, id string) (courseFileReceipt, []byte, error) {
	var r courseFileReceipt
	if !courseBatchID.MatchString(id) {
		return r, nil, ErrStudyInvalid
	}
	dir, err := root.Lstat(courseBatchDir)
	if err != nil || !dir.IsDir() || dir.Mode()&os.ModeSymlink != 0 {
		return r, nil, ErrStudyInvalid
	}
	rel := courseReceiptPath(id)
	st, err := root.Lstat(rel)
	if err != nil {
		return r, nil, err
	}
	if !st.Mode().IsRegular() || st.Size() > 12<<20 {
		return r, nil, ErrStudyInvalid
	}
	b, err := root.ReadFile(rel)
	if err != nil {
		return r, nil, err
	}
	if err := json.Unmarshal(b, &r); err != nil || r.ID != id || len(r.Changes) == 0 || len(r.Changes) > 24 {
		return r, nil, ErrStudyInvalid
	}
	return r, b, nil
}

type courseFileManifest struct {
	path          string
	before, after []byte
}

func (l *Library) CommitCourseFileBatch(ctx context.Context, id string, undo bool) (CourseFileBatch, error) {
	l.mirrorMu.Lock()
	defer l.mirrorMu.Unlock()
	ctx = context.WithoutCancel(ctx)
	if err := l.recoverMirrors(ctx); err != nil {
		return CourseFileBatch{}, err
	}
	root, err := os.OpenRoot(l.Root)
	if err != nil {
		return CourseFileBatch{}, err
	}
	defer root.Close()
	r, previous, err := readCourseReceipt(root, id)
	if err != nil {
		return r.CourseFileBatch, err
	}
	want, result := "pending", "applied"
	if undo {
		want, result = "applied", "undone"
	}
	if r.State == result {
		return r.CourseFileBatch, nil
	}
	if r.State != want || (!undo && time.Since(time.Unix(r.Created, 0)) > time.Hour) {
		return r.CourseFileBatch, fileChanged("整理批次")
	}
	course, err := l.Store.Course(ctx, r.CourseID)
	if err != nil {
		return r.CourseFileBatch, err
	}
	changes := slices.Clone(r.Changes)
	if undo {
		for i := range changes {
			c := &changes[i]
			c.From, c.To = c.To, c.From
			c.BeforeAsset, c.AfterAsset = c.AfterAsset, c.BeforeAsset
			c.Version, c.AfterVersion = c.AfterVersion, c.Version
			c.OldBody, c.NewBody = c.NewBody, c.OldBody
			c.OldText, c.NewText = c.NewText, c.OldText
			if c.AfterAsset != nil {
				// Keep links created during the interval before undo.
				a := *c.AfterAsset
				meta := CoursewareInfo(a)
				meta.PreviousPaths = materialAliases(meta.PreviousPaths, c.From, c.To)
				a.Meta, err = json.Marshal(meta)
				if err != nil {
					return r.CourseFileBatch, err
				}
				c.AfterAsset = &a
			}
		}
	}
	jobs, err := l.Store.Jobs(ctx, "queued", "running", "paused")
	if err != nil {
		return r.CourseFileBatch, err
	}
	for _, job := range jobs {
		if slices.Contains(r.Lectures, job.LectureID) && job.Kind == "parse" {
			return r.CourseFileBatch, fmt.Errorf("%w：请等该节课件解析完成后重试", ErrStudyConflict)
		}
	}
	var assets []store.CourseAssetChange
	var notes []store.CourseNoteChange
	for _, c := range changes {
		if err := l.checkCourseFileChange(ctx, root, course, c); err != nil {
			return r.CourseFileBatch, err
		}
		if c.BeforeAsset != nil || c.AfterAsset != nil {
			assets = append(assets, store.CourseAssetChange{Before: c.BeforeAsset, After: c.AfterAsset})
		}
		if c.NoteID != "" {
			notes = append(notes, store.CourseNoteChange{ID: c.NoteID, Path: c.From, Before: c.OldBody, After: c.NewBody})
		}
	}
	manifests, err := l.courseFileManifests(ctx, root, course, r.Lectures, assets)
	if err != nil {
		return r.CourseFileBatch, err
	}
	for _, c := range changes {
		if err := checkedStudyDir(root, path.Dir(c.To), true); err != nil {
			return r.CourseFileBatch, err
		}
	}
	r.State = result
	receipt, err := json.Marshal(r)
	if err != nil {
		return r.CourseFileBatch, err
	}
	mirror := l.beginMirror()
	err = l.Store.CommitCourseFiles(ctx, mirror.token(), assets, notes, func() error {
		for _, c := range changes {
			hash, _, err := courseFileVersion(root, c.From)
			if err != nil || hash != c.Version {
				return fileChanged(c.From)
			}
			if c.From == c.To {
				if err := mirror.replaceRooted(root, c.From, c.OldText, c.NewText); err != nil {
					return err
				}
				continue
			}
			if err := mirror.move(root, c.From, c.To); err != nil {
				return err
			}
			if c.ParseTree {
				if err := mirror.move(root, ParsedDir(c.From), ParsedDir(c.To)); err != nil {
					return err
				}
			}
		}
		for _, m := range manifests {
			if b, err := root.ReadFile(m.path); err != nil || materialHash(b) != materialHash(m.before) {
				return fileChanged(m.path)
			}
			if err := mirror.replaceRooted(root, m.path, m.before, m.after); err != nil {
				return err
			}
		}
		return mirror.replaceRooted(root, courseReceiptPath(id), previous, receipt)
	})
	mirror.finish(ctx, err)
	if err != nil {
		r.State = want
	}
	return r.CourseFileBatch, err
}

func (l *Library) checkCourseFileChange(ctx context.Context, root *os.Root, course store.Course, c courseFileChange) error {
	if !hasCleanCoursePath(course, c.From) || !hasCleanCoursePath(course, c.To) {
		return ErrStudyInvalid
	}
	hash, _, err := courseFileVersion(root, c.From)
	if err != nil || hash != c.Version {
		return fileChanged(c.From)
	}
	if c.From == c.To {
		return nil
	}
	for _, a := range []*store.Asset{c.BeforeAsset, c.AfterAsset} {
		if a == nil {
			continue
		}
		lecture, err := l.Store.Lecture(ctx, a.LectureID)
		if err != nil || lecture.CourseID != course.ID || !strings.HasPrefix(a.Path, lecture.Dir+"/courseware/") ||
			(a == c.BeforeAsset && a.Path != c.From) || (a == c.AfterAsset && a.Path != c.To) {
			return fileChanged(a.Path)
		}
	}
	if c.BeforeAsset != nil {
		current, err := l.coursewareAt(ctx, c.BeforeAsset.LectureID, c.From)
		if err != nil || string(current.Meta) != string(c.BeforeAsset.Meta) {
			return fileChanged(c.From)
		}
		info, err := root.Lstat(ParsedDir(c.From))
		if c.ParseTree {
			if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return fileChanged(c.From)
			}
		} else if !os.IsNotExist(err) {
			return fileChanged(c.From)
		}
	}
	if err := vacantCoursePath(root, c.To); err != nil {
		return err
	}
	if err := vacantCoursePath(root, ParsedDir(c.To)); err != nil {
		return err
	}
	alias, err := l.Store.CoursewareAlias(ctx, c.To)
	if err == nil && (c.BeforeAsset == nil || alias.Path != c.BeforeAsset.Path) {
		return fileChanged(c.To)
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return nil
}

func (l *Library) courseFileManifests(ctx context.Context, root *os.Root, course store.Course, lectures []string, changes []store.CourseAssetChange) ([]courseFileManifest, error) {
	out := make([]courseFileManifest, 0, len(lectures))
	for _, id := range lectures {
		lec, err := l.Store.Lecture(ctx, id)
		if err != nil || lec.CourseID != course.ID || !hasCleanCoursePath(course, lec.Dir) {
			return nil, ErrStudyInvalid
		}
		rel := lec.Dir + "/lecture.json"
		previous, err := readBoundedStudyFile(root, rel)
		if err != nil {
			return nil, err
		}
		var manifest map[string]json.RawMessage
		if err := json.Unmarshal(previous, &manifest); err != nil || manifest == nil {
			return nil, ErrStudyInvalid
		}
		var marker string
		if json.Unmarshal(manifest["id"], &marker) != nil || marker != id {
			return nil, ErrStudyInvalid
		}
		indexed, err := l.Store.Assets(ctx, id)
		if err != nil {
			return nil, err
		}
		for _, change := range changes {
			if a := change.Before; a != nil && a.LectureID == id {
				indexed = slices.DeleteFunc(indexed, func(x store.Asset) bool { return x.Path == a.Path && x.Kind == a.Kind })
			}
			if a := change.After; a != nil && a.LectureID == id {
				indexed = append(indexed, *a)
			}
		}
		files := make([]map[string]any, 0, len(indexed))
		for _, a := range indexed {
			files = append(files, map[string]any{
				"kind": a.Kind, "path": strings.TrimPrefix(a.Path, lec.Dir+"/"), "bytes": a.Bytes, "meta": a.Meta,
			})
		}
		b, err := json.Marshal(files)
		if err != nil {
			return nil, err
		}
		manifest["files"] = b
		after, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			return nil, err
		}
		out = append(out, courseFileManifest{path: rel, before: previous, after: append(after, '\n')})
	}
	return out, nil
}

// FormerCourseware accepts an old path only while it still belongs to the
// course which owns the current indexed file.
func (l *Library) FormerCourseware(ctx context.Context, old string) (store.Asset, bool) {
	if !strings.HasPrefix(old, "courses/") || path.Clean(old) != old || strings.Contains(old, "\\") {
		return store.Asset{}, false
	}
	a, err := l.Store.CoursewareAlias(ctx, old)
	if err != nil {
		return store.Asset{}, false
	}
	lec, err := l.Store.Lecture(ctx, a.LectureID)
	if err != nil {
		return store.Asset{}, false
	}
	c, err := l.Store.Course(ctx, lec.CourseID)
	if err != nil || !hasCleanCoursePath(c, old) || !hasCleanCoursePath(c, a.Path) ||
		!strings.HasPrefix(a.Path, lec.Dir+"/courseware/") || a.Path == old {
		return store.Asset{}, false
	}
	return a, true
}

func (l *Library) currentCoursewarePath(ctx context.Context, rel string) string {
	if !strings.HasPrefix(rel, "courses/") || path.Clean(rel) != rel || strings.Contains(rel, "\\") {
		return rel
	}
	if _, err := os.Lstat(l.Abs(rel)); errors.Is(err, os.ErrNotExist) {
		if alias, ok := l.FormerCourseware(ctx, rel); ok {
			return alias.Path
		}
	}
	return rel
}

func (l *Library) FormerLectureCourseware(ctx context.Context, lecture, ref string) (store.Asset, bool) {
	from, err := l.Store.Lecture(ctx, lecture)
	if err != nil || from.Dir == "" || ref == "" {
		return store.Asset{}, false
	}
	paths := []string{ref}
	if path.Base(ref) == ref && !strings.Contains(ref, "\\") {
		ext := path.Ext(ref)
		name := Sanitize(strings.TrimSuffix(ref, ext), "课件") + strings.ToLower(ext)
		paths = []string{from.Dir + "/courseware/" + name, from.Dir + "/courseware/" + courseArchive + "/" + name}
	}
	for _, old := range paths {
		if a, ok := l.FormerCourseware(ctx, old); ok {
			to, err := l.Store.Lecture(ctx, a.LectureID)
			if err == nil && from.CourseID == to.CourseID {
				return a, true
			}
		}
	}
	return store.Asset{}, false
}

func (l *Library) OpenFormerCourseware(ctx context.Context, rel string) (*os.File, error) {
	old, suffix := rel, ""
	if asset, tail, parsed := strings.Cut(rel, ".parsed/"); parsed {
		old, suffix = asset, ".parsed/"+tail
	}
	a, ok := l.FormerCourseware(ctx, old)
	if !ok {
		return nil, os.ErrNotExist
	}
	root, err := os.OpenRoot(l.Root)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	target := a.Path + suffix
	if path.Clean(target) != target || strings.Contains(target, "\\") {
		return nil, ErrStudyInvalid
	}
	return courseFileOpen(root, target)
}
