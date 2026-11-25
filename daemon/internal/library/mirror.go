package library

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

// Mirrored writes change the index and a JSON file in the library together
// (activity.json, quizzes/<id>.json). The file changes inside the index
// transaction, so a file that cannot be written keeps the change out of the
// index. Two windows remain, and are closed like this:
//   - Readers of the files: writers and Reindex hold l.mirrorMu, so Reindex
//     never imports a file whose transaction has not settled.
//   - A crash between the file change and the commit: before changing a
//     file, the write journals its previous bytes in .mirror-journal/, and
//     the transaction records the journal's token (store.MirrorCommitted).
//     recoverMirrors, run when the library opens and before Reindex, keeps
//     the files of committed tokens and restores the others.
//
// A journal whose token is missing because library.db itself was deleted is
// treated as uncommitted: the previous file returns.

const journalDir = ".mirror-journal"

type journalEntry struct {
	Path    string `json:"path"` // library-relative, slash separated
	Existed bool   `json:"existed"`
	Old     []byte `json:"old,omitempty"`
	MoveTo  string `json:"move_to,omitempty"`
	Rooted  bool   `json:"rooted,omitempty"`
}

type journal struct {
	Token   string         `json:"token"`
	Entries []journalEntry `json:"entries"`
}

// mirrorTx journals the file changes of one index transaction.
type mirrorTx struct {
	l      *Library
	j      journal
	failed bool // a restore failed: the journal stays for recovery
}

func (l *Library) beginMirror() *mirrorTx { return &mirrorTx{l: l, j: journal{Token: NewID()}} }

func (m *mirrorTx) token() string { return m.j.Token }

func (l *Library) journalPath(token string) string {
	return filepath.Join(l.Root, journalDir, token+".json")
}

// replace journals path's current content, then writes v as JSON to it.
// The returned undo does nothing: finish restores files once it knows the
// outcome, since a failed commit call does not always mean a rollback.
func (m *mirrorTx) replace(path string, v any) (func(), error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return m.change(path, append(b, '\n'), false)
}

// remove journals path's current content, then deletes it.
func (m *mirrorTx) remove(path string) (func(), error) { return m.change(path, nil, true) }

func (m *mirrorTx) record(e journalEntry) error {
	m.j.Entries = append(m.j.Entries, e)
	if err := m.save(); err != nil {
		m.j.Entries = m.j.Entries[:len(m.j.Entries)-1]
		return err
	}
	return nil
}

func (m *mirrorTx) move(root *os.Root, from, to string) error {
	info, err := root.Lstat(from)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() && !info.IsDir() {
		return ErrStudyInvalid
	}
	if err := vacantCoursePath(root, to); err != nil {
		return err
	}
	if err := m.record(journalEntry{Path: from, MoveTo: to}); err != nil {
		return err
	}
	if info.Mode().IsRegular() {
		// Link reserves the new name without replacing another file, including
		// a file created between the occupancy check and the move.
		if err := root.Link(from, to); err != nil {
			return err
		}
		return root.Remove(from)
	}
	return root.Rename(from, to)
}

func (m *mirrorTx) replaceRooted(root *os.Root, rel string, before, after []byte) error {
	if err := m.record(journalEntry{Path: rel, Existed: before != nil, Old: before, Rooted: true}); err != nil {
		return err
	}
	return writeStudyBytes(root, rel, after)
}

func (m *mirrorTx) change(path string, b []byte, remove bool) (func(), error) {
	old, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err := m.record(journalEntry{Path: m.l.Rel(path), Existed: err == nil, Old: old}); err != nil {
		return nil, err
	}
	if remove {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	} else if err := writeFileAtomic(path, b); err != nil {
		return nil, err
	}
	return func() {}, nil
}

func (m *mirrorTx) save() error {
	dir := filepath.Join(m.l.Root, journalDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(m.j)
	if err != nil {
		return err
	}
	if err := writeFileAtomic(m.l.journalPath(m.j.Token), b); err != nil {
		return err
	}
	return syncDir(dir)
}

// finish settles the files once the index transaction returned err.
func (m *mirrorTx) finish(ctx context.Context, err error) {
	if len(m.j.Entries) == 0 {
		return
	}
	committed := err == nil
	if !committed {
		ok, qerr := m.l.Store.MirrorCommitted(ctx, m.j.Token)
		if qerr != nil {
			log.Printf("mirror %s: outcome unknown, left for recovery: %v", m.j.Token, qerr)
			return
		}
		committed = ok
	}
	if !committed {
		for i := len(m.j.Entries) - 1; i >= 0; i-- {
			if rerr := m.l.restoreEntry(m.j.Entries[i]); rerr != nil {
				log.Printf("restore %s: %v", m.j.Entries[i].Path, rerr)
				m.failed = true
			}
		}
		if m.failed {
			return
		}
	}
	m.l.dropJournal(ctx, m.j.Token)
}

func (l *Library) restoreEntry(e journalEntry) error {
	if e.MoveTo != "" || e.Rooted {
		root, err := os.OpenRoot(l.Root)
		if err != nil {
			return err
		}
		defer root.Close()
		if e.MoveTo != "" {
			if err := checkedStudyDir(root, path.Dir(e.Path), false); err != nil {
				return err
			}
			if err := checkedStudyDir(root, path.Dir(e.MoveTo), false); err != nil {
				return err
			}
			beforeInfo, before := root.Lstat(e.Path)
			afterInfo, after := root.Lstat(e.MoveTo)
			if before == nil && os.IsNotExist(after) {
				return nil
			}
			if os.IsNotExist(before) && after == nil {
				return root.Rename(e.MoveTo, e.Path)
			}
			if before == nil && after == nil && os.SameFile(beforeInfo, afterInfo) {
				return root.Remove(e.MoveTo)
			}
			return fmt.Errorf("both or neither path exists for move %q to %q", e.Path, e.MoveTo)
		}
		if e.Existed {
			return writeStudyBytes(root, e.Path, e.Old)
		}
		if err := root.Remove(e.Path); !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	p := l.Abs(e.Path)
	if !e.Existed {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	return writeFileAtomic(p, e.Old)
}

func (l *Library) dropJournal(ctx context.Context, token string) {
	if err := os.Remove(l.journalPath(token)); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("journal %s: %v", token, err)
		return
	}
	if err := l.Store.ForgetMirrorCommit(ctx, token); err != nil {
		log.Printf("journal %s: %v", token, err)
	}
}

// recoverMirrors settles journals left by an interrupted write. The caller
// holds l.mirrorMu.
func (l *Library) recoverMirrors(ctx context.Context) error {
	dir := filepath.Join(l.Root, journalDir)
	ents, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	for _, d := range ents {
		p := filepath.Join(dir, d.Name())
		if !strings.HasSuffix(d.Name(), ".json") {
			os.Remove(p) // an interrupted journal write; its file change never started
			continue
		}
		var j journal
		b, err := os.ReadFile(p)
		if err == nil {
			err = json.Unmarshal(b, &j)
		}
		if err != nil || j.Token+".json" != d.Name() {
			return fmt.Errorf("mirror journal %s: unreadable", d.Name())
		}
		ok, err := l.Store.MirrorCommitted(ctx, j.Token)
		if err != nil {
			return err
		}
		if !ok {
			for i := len(j.Entries) - 1; i >= 0; i-- {
				if err := l.restoreEntry(j.Entries[i]); err != nil {
					return fmt.Errorf("restore %s: %w", j.Entries[i].Path, err)
				}
			}
			log.Printf("mirror journal %s: restored %d file(s) of an unfinished write", j.Token, len(j.Entries))
		}
		l.dropJournal(ctx, j.Token)
	}
	return nil
}

func syncDir(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	if runtime.GOOS == "windows" {
		// FlushFileBuffers rejects directory handles. The journal and
		// mirrored files themselves are flushed before replacement.
		return nil
	}
	return f.Sync()
}
