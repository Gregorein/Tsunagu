package localsource_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"tsunagu/backend/internal/localsource"
)

func TestRelocateMigratesFilesAndSwitchesDir(t *testing.T) {
	q := openTestDB(t)
	mediaDir := t.TempDir()

	oldTitleDir := filepath.Join(mediaDir, "local", "manga", "Series")
	if err := os.MkdirAll(oldTitleDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeCBZ(t, filepath.Join(oldTitleDir, "Ch 1.cbz"), "01.jpg")

	s := localsource.New(q, mediaDir)
	ctx := context.Background()
	if _, err := s.Scan(ctx); err != nil {
		t.Fatalf("initial scan: %v", err)
	}

	newDir := filepath.Join(t.TempDir(), "elsewhere")
	res, err := s.Relocate(newDir, true)
	if err != nil {
		t.Fatalf("relocate: %v", err)
	}
	if res.MovedFiles < 1 {
		t.Errorf("MovedFiles = %d, want >= 1", res.MovedFiles)
	}

	// Old location should be gone, new one should hold the migrated file.
	if _, err := os.Stat(filepath.Join(mediaDir, "local")); !os.IsNotExist(err) {
		t.Errorf("old local dir still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(newDir, "manga", "Series", "Ch 1.cbz")); err != nil {
		t.Errorf("migrated file missing at new location: %v", err)
	}

	if got := s.LocalDir(); got != newDir {
		t.Errorf("LocalDir() = %q, want %q", got, newDir)
	}

	// A rescan from the new location should still find the series.
	res2, err := s.Scan(ctx)
	if err != nil {
		t.Fatalf("rescan after relocate: %v", err)
	}
	if res2.MediaTouched != 1 {
		t.Errorf("MediaTouched after relocate = %d, want 1", res2.MediaTouched)
	}
}

func TestRelocateRejectsNestedPath(t *testing.T) {
	q := openTestDB(t)
	mediaDir := t.TempDir()
	s := localsource.New(q, mediaDir)

	nested := filepath.Join(mediaDir, "local", "nested")
	if _, err := s.Relocate(nested, false); err == nil {
		t.Fatal("expected error relocating into a subdirectory of the current local dir")
	}
}

func TestRelocateSameDirIsNoop(t *testing.T) {
	q := openTestDB(t)
	mediaDir := t.TempDir()
	s := localsource.New(q, mediaDir)

	res, err := s.Relocate(filepath.Join(mediaDir, "local"), true)
	if err != nil {
		t.Fatalf("relocate: %v", err)
	}
	if res.MovedFiles != 0 {
		t.Errorf("MovedFiles = %d, want 0 for a same-dir relocate", res.MovedFiles)
	}
}
