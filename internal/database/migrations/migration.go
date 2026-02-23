package migrations

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

var _ sort.Interface = Migrations{}

// Migrations for easy sorting list of migrations.
type Migrations []Migration

// Len implements sort.Interface.
func (m Migrations) Len() int { return len(m) }

// Less implements sort.Interface.
func (m Migrations) Less(i, j int) bool { return m[i].Version < m[j].Version }

// Swap implements sort.Interface.
func (m Migrations) Swap(i, j int) { m[i], m[j] = m[j], m[i] }

// Migration information for UP and Down sql query.
type Migration struct {
	Version uint
	Name    string
	Up      string
	Down    string
}

// Parse and build migrations from disk.
func Parse(path string) (Migrations, error) {
	return FromFS(os.DirFS(path), ".")
}

// FromFS build migrations from file system.
func FromFS(directory fs.FS, root string) (Migrations, error) {
	var migrations Migrations
	err := fs.WalkDir(directory, root, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir():
			return nil
		case filepath.Ext(d.Name()) != migrationExt:
			return nil
		}

		f, err := directory.Open(path)
		if err != nil {
			return fmt.Errorf("os.Open: %w", err)
		}

		m, err := parse(f)
		if err != nil {
			return fmt.Errorf("parse: %w", err)
		}
		migrations = append(migrations, *m)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("fs.WalkDir: %w", err)
	}

	sort.Sort(migrations)

	return migrations, nil
}
