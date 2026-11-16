package gitback

import (
	"os"
	"path/filepath"
	"time"
)

// FetchedAt is when repoDir last fetched its remote: the mtime of
// .git/FETCH_HEAD, which git rewrites on every fetch. ok is false when the
// clone never fetched (no FETCH_HEAD).
func FetchedAt(repoDir string) (t time.Time, ok bool) {
	fi, err := os.Stat(filepath.Join(repoDir, ".git", "FETCH_HEAD"))
	if err != nil {
		return time.Time{}, false
	}
	return fi.ModTime(), true
}
