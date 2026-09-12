package backup

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Info struct {
	Name      string
	Path      string
	Bytes     int64
	CreatedAt time.Time
	Kind      string
}

func CreateSnapshot(ctx context.Context, db *sql.DB, dir string) (Info, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Info{}, err
	}
	name := "tsunagu-" + time.Now().Format("20060102-150405") + ".db"
	dest := filepath.Join(dir, name)
	if _, err := db.ExecContext(ctx, "VACUUM INTO ?", dest); err != nil {
		return Info{}, fmt.Errorf("vacuum into %s: %w", dest, err)
	}
	info, err := os.Stat(dest)
	if err != nil {
		return Info{}, err
	}
	return Info{Name: name, Path: dest, Bytes: info.Size(), CreatedAt: info.ModTime(), Kind: "sqlite"}, nil
}

func List(dir string) ([]Info, error) {
	ents, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Info
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		var kind string
		switch {
		case strings.HasSuffix(e.Name(), ".db"):
			kind = "sqlite"
		case strings.HasSuffix(e.Name(), ".tachibk"):
			kind = "mihon"
		default:
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, Info{Name: e.Name(), Path: filepath.Join(dir, e.Name()), Bytes: info.Size(), CreatedAt: info.ModTime(), Kind: kind})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func Delete(dir, name string) error {
	clean := filepath.Base(name)
	if clean != name || clean == "." || clean == ".." {
		return fmt.Errorf("invalid backup name %q", name)
	}
	return os.Remove(filepath.Join(dir, clean))
}

func PruneSnapshots(dir string, keep int) error {
	if keep <= 0 {
		return nil
	}
	all, err := List(dir)
	if err != nil {
		return err
	}
	var sqliteOnly []Info
	for _, i := range all {
		if i.Kind == "sqlite" {
			sqliteOnly = append(sqliteOnly, i)
		}
	}
	if len(sqliteOnly) <= keep {
		return nil
	}
	for _, i := range sqliteOnly[keep:] {
		if err := os.Remove(i.Path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
