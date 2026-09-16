package localsource_test

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"testing"

	tdb "tsunagu/backend/internal/db"
	"tsunagu/backend/internal/db/sqlcgen"
	"tsunagu/backend/internal/localsource"
)

func openTestDB(t *testing.T) *sqlcgen.Queries {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	conn, err := tdb.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return sqlcgen.New(conn)
}

func writeCBZ(t *testing.T, path string, pageNames ...string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create cbz: %v", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	for _, name := range pageNames {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create entry %q: %v", name, err)
		}
		if _, err := w.Write([]byte("fake-image-bytes-" + name)); err != nil {
			t.Fatalf("write entry %q: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close cbz: %v", err)
	}
}

func TestScanCBZChapter(t *testing.T) {
	q := openTestDB(t)
	mediaDir := t.TempDir()

	titleDir := filepath.Join(mediaDir, "local", "manga", "Solo Leveling")
	if err := os.MkdirAll(titleDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeCBZ(t, filepath.Join(titleDir, "Chapter 1.cbz"), "01.jpg", "02.jpg", "10.jpg")
	writeCBZ(t, filepath.Join(titleDir, "Chapter 2.cbz"), "01.jpg")

	s := localsource.New(q, mediaDir)
	ctx := context.Background()

	res, err := s.Scan(ctx)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if res.MediaTouched != 1 {
		t.Errorf("MediaTouched = %d, want 1", res.MediaTouched)
	}
	if res.ChaptersAdded != 2 {
		t.Errorf("ChaptersAdded = %d, want 2", res.ChaptersAdded)
	}
	if res.FilesLinked != 4 {
		t.Errorf("FilesLinked = %d, want 4", res.FilesLinked)
	}

	media, err := q.GetLocalMediaByExternalID(ctx, "local:manga/Solo Leveling")
	if err != nil {
		t.Fatalf("get media: %v", err)
	}

	chapters, err := q.ListChaptersByMedia(ctx, media.ID)
	if err != nil {
		t.Fatalf("list chapters: %v", err)
	}
	if len(chapters) != 2 {
		t.Fatalf("got %d chapters, want 2", len(chapters))
	}

	var ch1 sqlcgen.Chapter
	for _, c := range chapters {
		if c.Title.String == "Chapter 1" {
			ch1 = c
		}
	}
	if ch1.ID == 0 {
		t.Fatalf("Chapter 1 not found among: %+v", chapters)
	}

	pages, err := q.ListMangaPages(ctx, ch1.ID)
	if err != nil {
		t.Fatalf("list pages: %v", err)
	}
	if len(pages) != 3 {
		t.Fatalf("got %d pages for Chapter 1, want 3", len(pages))
	}
	// Pages should be naturally sorted: 01, 02, 10 (not lexicographic 01, 02, 10 which happens
	// to match here, so also check the page numbering assigns sequential 1..3).
	for i, p := range pages {
		if p.PageNumber != int64(i+1) {
			t.Errorf("page[%d].PageNumber = %d, want %d", i, p.PageNumber, i+1)
		}
		archivePath, entryName, ok := localsource.ParseZipPagePath(p.LocalPath.String)
		if !ok {
			t.Fatalf("page[%d].LocalPath = %q is not a zip page path", i, p.LocalPath.String)
		}
		if archivePath != filepath.Join(titleDir, "Chapter 1.cbz") {
			t.Errorf("page[%d] archivePath = %q", i, archivePath)
		}
		if entryName == "" {
			t.Errorf("page[%d] entryName is empty", i)
		}
	}
}

func TestScanCBZRescanIsIdempotent(t *testing.T) {
	q := openTestDB(t)
	mediaDir := t.TempDir()

	titleDir := filepath.Join(mediaDir, "local", "manga", "Series")
	if err := os.MkdirAll(titleDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeCBZ(t, filepath.Join(titleDir, "Ch 1.cbz"), "01.jpg")

	s := localsource.New(q, mediaDir)
	ctx := context.Background()

	if _, err := s.Scan(ctx); err != nil {
		t.Fatalf("first scan: %v", err)
	}
	res, err := s.Scan(ctx)
	if err != nil {
		t.Fatalf("second scan: %v", err)
	}
	if res.ChaptersAdded != 0 {
		t.Errorf("second scan ChaptersAdded = %d, want 0 (already exists)", res.ChaptersAdded)
	}
	if res.FilesLinked != 1 {
		t.Errorf("second scan FilesLinked = %d, want 1 (pages re-linked)", res.FilesLinked)
	}
}

func TestScanCBZPruneOnDelete(t *testing.T) {
	q := openTestDB(t)
	mediaDir := t.TempDir()

	titleDir := filepath.Join(mediaDir, "local", "manga", "Series")
	if err := os.MkdirAll(titleDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cbzPath := filepath.Join(titleDir, "Ch 1.cbz")
	writeCBZ(t, cbzPath, "01.jpg", "02.jpg")

	s := localsource.New(q, mediaDir)
	ctx := context.Background()
	if _, err := s.Scan(ctx); err != nil {
		t.Fatalf("scan: %v", err)
	}

	// Delete the archive outright (without recreating it, so the chapter is
	// no longer picked up by the scan) and confirm the now-orphaned page
	// rows get pruned via gone()'s zip-aware existence check.
	if err := os.Remove(cbzPath); err != nil {
		t.Fatalf("remove: %v", err)
	}

	res, err := s.Scan(ctx)
	if err != nil {
		t.Fatalf("rescan: %v", err)
	}
	if res.RowsPruned < 1 {
		t.Errorf("RowsPruned = %d, want >= 1", res.RowsPruned)
	}
}
