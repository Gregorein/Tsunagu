package localsource

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type RelocateResult struct {
	MovedFiles int64
	MovedBytes int64
	NewPath    string
}

// Relocate points the scanner at a new local-source directory, optionally
// copying existing files over first. Unlike downloads, there's no separate
// DB-path rewrite step needed — a rescan (left to the caller) regenerates
// every manga_pages/novel_chapter_content/anime_episode_streams row from the
// new location, and prune() cleans up whatever's left pointing at the old one.
func (s *Scanner) Relocate(newPath string, migrate bool) (RelocateResult, error) {
	trimmed := strings.TrimSpace(newPath)
	if trimmed == "" {
		trimmed = filepath.Join(s.mediaDir, "local")
	}
	newAbs, err := filepath.Abs(trimmed)
	if err != nil {
		return RelocateResult{}, fmt.Errorf("resolve path: %w", err)
	}
	cur := filepath.Clean(s.LocalDir())
	if newAbs == cur {
		return RelocateResult{NewPath: newAbs}, nil
	}
	if isSubpath(cur, newAbs) || isSubpath(newAbs, cur) {
		return RelocateResult{}, fmt.Errorf("new local source path must not be nested inside the current one")
	}
	if err := os.MkdirAll(newAbs, 0o755); err != nil {
		return RelocateResult{}, fmt.Errorf("create %s: %w", newAbs, err)
	}
	probe := filepath.Join(newAbs, ".tsunagu-write-test")
	if err := os.WriteFile(probe, []byte("ok"), 0o644); err != nil {
		return RelocateResult{}, fmt.Errorf("path not writable: %w", err)
	}
	_ = os.Remove(probe)

	res := RelocateResult{NewPath: newAbs}
	if migrate {
		if info, statErr := os.Stat(cur); statErr == nil && info.IsDir() {
			files, bytes, cpErr := copyTree(cur, newAbs)
			res.MovedFiles = files
			res.MovedBytes = bytes
			if cpErr != nil {
				return res, fmt.Errorf("copy local source files: %w", cpErr)
			}
			_ = os.RemoveAll(cur)
		}
	}

	s.SetLocalDir(newAbs)
	return res, nil
}

func isSubpath(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func copyTree(src, dst string) (files int64, bytes int64, err error) {
	err = filepath.WalkDir(src, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, relErr := filepath.Rel(src, p)
		if relErr != nil {
			return relErr
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return infoErr
		}
		if st, statErr := os.Stat(target); statErr == nil && st.Size() == info.Size() {
			files++
			bytes += info.Size()
			return nil
		}
		if mkErr := os.MkdirAll(filepath.Dir(target), 0o755); mkErr != nil {
			return mkErr
		}
		if cpErr := copyFile(p, target); cpErr != nil {
			return cpErr
		}
		files++
		bytes += info.Size()
		return nil
	})
	return files, bytes, err
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}
