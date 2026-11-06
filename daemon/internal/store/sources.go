package store

// ensureSourceColumns adds the source and platform URL columns to an index
// created before source plugins.
func (s *Store) ensureSourceColumns() error {
	for _, c := range []struct{ table, name, ddl string }{
		{"courses", "source", `ALTER TABLE courses ADD COLUMN source TEXT NOT NULL DEFAULT ''`},
		{"courses", "url", `ALTER TABLE courses ADD COLUMN url TEXT NOT NULL DEFAULT ''`},
		{"lectures", "url", `ALTER TABLE lectures ADD COLUMN url TEXT NOT NULL DEFAULT ''`},
	} {
		var n int
		if err := s.DB.QueryRow(`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name=?`, c.table, c.name).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			if _, err := s.DB.Exec(c.ddl); err != nil {
				return err
			}
		}
	}
	return nil
}
