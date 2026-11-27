package store

import (
	"context"
	"database/sql"
)

type CourseAssetChange struct {
	Before *Asset
	After  *Asset
}

type CourseNoteChange struct {
	ID, Path, Before, After string
}

func (s *Store) CommitCourseFiles(ctx context.Context, token string, assets []CourseAssetChange, notes []CourseNoteChange, write func() error) error {
	return s.mirrored(ctx, token, func(tx *sql.Tx) error {
		for _, c := range assets {
			if a := c.Before; a != nil {
				if _, err := tx.ExecContext(ctx, `DELETE FROM assets WHERE lecture_id=? AND kind=? AND path=?`, a.LectureID, a.Kind, a.Path); err != nil {
					return err
				}
			}
			if a := c.After; a != nil {
				if _, err := tx.ExecContext(ctx, `INSERT INTO assets(lecture_id,kind,path,bytes,created_at,meta_json) VALUES(?,?,?,?,?,?)`,
					a.LectureID, a.Kind, a.Path, a.Bytes, a.CreatedAt, string(a.Meta)); err != nil {
					return err
				}
			}
		}
		for _, n := range notes {
			r, err := tx.ExecContext(ctx, `UPDATE notes SET body=?,updated_at=? WHERE id=? AND path=? AND body=?`,
				n.After, now(), n.ID, n.Path, n.Before)
			if err != nil {
				return err
			}
			if count, err := r.RowsAffected(); err != nil || count != 1 {
				return sql.ErrNoRows
			}
		}
		return nil
	}, func(*sql.Tx) (func(), error) {
		return func() {}, write()
	})
}

func (s *Store) CoursewareAlias(ctx context.Context, previous string) (Asset, error) {
	var a Asset
	var meta string
	err := s.DB.QueryRowContext(ctx, `SELECT lecture_id,kind,path,bytes,created_at,meta_json FROM assets
	WHERE kind='courseware' AND json_valid(meta_json) AND EXISTS (
	  SELECT 1 FROM json_each(meta_json,'$.previous_paths') WHERE value=?
	) ORDER BY path LIMIT 1`, previous).
		Scan(&a.LectureID, &a.Kind, &a.Path, &a.Bytes, &a.CreatedAt, &meta)
	a.Meta = []byte(meta)
	return a, err
}
