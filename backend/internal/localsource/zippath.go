package localsource

import "strings"

// A CBZ/ZIP page's local_path is a virtual "zip://<archive path>!<entry name>"
// URL rather than a real filesystem path, so the same local_path column can
// point either at a loose image file or at an entry inside an archive.
const zipPrefix = "zip://"

func ZipPagePath(archivePath, entryName string) string {
	return zipPrefix + archivePath + "!" + entryName
}

func ParseZipPagePath(p string) (archivePath, entryName string, ok bool) {
	rest, found := strings.CutPrefix(p, zipPrefix)
	if !found {
		return "", "", false
	}
	i := strings.LastIndex(rest, "!")
	if i < 0 {
		return "", "", false
	}
	return rest[:i], rest[i+1:], true
}
