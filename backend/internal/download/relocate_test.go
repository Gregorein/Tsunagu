package download

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"tsunagu/backend/internal/db"
	"tsunagu/backend/internal/db/sqlcgen"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRelocateMigrates(t *testing.T) {
	tmp := t.TempDir()
	oldRoot := filepath.Join(tmp, "old")
	newRoot := filepath.Join(tmp, "new")

	dlA := filepath.Join(oldRoot, "manga", "ExtA", "TitleA", "Ch1", "1.jpg")
	dlB := filepath.Join(oldRoot, "manga", "ExtA", "TitleA", "Ch1", "2.jpg")
	localFile := filepath.Join(oldRoot, "local", "manga", "HandAdded", "Ch1", "1.jpg")
	writeFile(t, dlA, "aaa")
	writeFile(t, dlB, "bbbb")
	writeFile(t, localFile, "keepme")

	conn, err := db.Open(filepath.Join(tmp, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Exec(`
		INSERT INTO media (id, external_id, content_type, title) VALUES (1, 'ext:a', 'manga', 'TitleA');
		INSERT INTO chapters (id, media_id, external_id) VALUES (10, 1, 'ch1'), (11, 1, 'local-ch1');
		INSERT INTO manga_pages (chapter_id, page_number, local_path) VALUES
			(10, 1, ?), (10, 2, ?), (11, 1, ?);
	`, dlA, dlB, localFile); err != nil {
		t.Fatal(err)
	}
	q := sqlcgen.New(conn)

	m := &Manager{q: q, mediaDir: oldRoot, downloadsDir: oldRoot}

	res, err := m.Relocate(context.Background(), newRoot, true)
	if err != nil {
		t.Fatalf("relocate: %v", err)
	}
	if res.MovedFiles != 2 || res.MovedBytes != 7 {
		t.Fatalf("moved %d files / %d bytes, want 2 / 7", res.MovedFiles, res.MovedBytes)
	}

	newA := filepath.Join(newRoot, "manga", "ExtA", "TitleA", "Ch1", "1.jpg")
	if b, err := os.ReadFile(newA); err != nil || string(b) != "aaa" {
		t.Fatalf("new file A: %q err=%v", b, err)
	}
	if _, err := os.Stat(filepath.Join(oldRoot, "manga")); !os.IsNotExist(err) {
		t.Fatalf("old manga/ still present: %v", err)
	}
	if b, err := os.ReadFile(localFile); err != nil || string(b) != "keepme" {
		t.Fatalf("local-source file was touched: %q err=%v", b, err)
	}

	pages, _ := q.ListAllMangaPagePaths(context.Background())
	for _, p := range pages {
		switch {
		case p.ChapterID == 10 && p.PageNumber == 1 && p.LocalPath.String != newA:
			t.Fatalf("download page path = %q, want %q", p.LocalPath.String, newA)
		case p.ChapterID == 11 && p.LocalPath.String != localFile:
			t.Fatalf("local-source row was rewritten: %q", p.LocalPath.String)
		}
	}
}

func TestRelocateNoMigrateJustSwitches(t *testing.T) {
	tmp := t.TempDir()
	oldRoot := filepath.Join(tmp, "old")
	newRoot := filepath.Join(tmp, "new")
	old := filepath.Join(oldRoot, "manga", "x", "y", "z", "1.jpg")
	writeFile(t, old, "data")

	conn, err := db.Open(filepath.Join(tmp, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	q := sqlcgen.New(conn)
	m := &Manager{q: q, mediaDir: oldRoot, downloadsDir: oldRoot}

	res, err := m.Relocate(context.Background(), newRoot, false)
	if err != nil {
		t.Fatalf("relocate: %v", err)
	}
	if res.MovedFiles != 0 {
		t.Fatalf("no-migrate moved %d files", res.MovedFiles)
	}
	if _, err := os.Stat(old); err != nil {
		t.Fatalf("no-migrate removed old file: %v", err)
	}
	if m.DownloadsDir() == newRoot {
		t.Fatal("Relocate must not switch dir itself; the resolver does that via config hook")
	}
}

func TestRelocateRejectsNested(t *testing.T) {
	tmp := t.TempDir()
	m := &Manager{mediaDir: tmp, downloadsDir: filepath.Join(tmp, "dl")}
	if _, err := m.Relocate(context.Background(), filepath.Join(tmp, "dl", "inner"), false); err == nil {
		t.Fatal("expected rejection of nested path")
	}
}
