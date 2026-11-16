package gitback

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// ErrForeignClone marks a cache clone whose origin is not the backend the
// project declares (F-0434).
var ErrForeignClone = errors.New("cache clone belongs to another backend")

// CheckOrigin refuses a clone in repoDir whose origin is another backend. The
// cache is keyed by project_id only, so two projects sharing an id would share
// one clone; the clone is never touched here (the cache dir also holds rooms
// and messages), the caller just stops.
func CheckOrigin(repoDir, backend string) error {
	origin, err := OriginURL(repoDir)
	if err != nil {
		return fmt.Errorf("reading the origin of %s: %w", repoDir, err)
	}
	if sameBackend(origin, backend) {
		return nil
	}
	return fmt.Errorf("%w: the cache clone at %s has origin %s but this project declares backend %s; "+
		"give this project its own CERNE_CACHE_DIR or a distinct project_id",
		ErrForeignClone, repoDir, origin, backend)
}

func sameBackend(a, b string) bool { return canonBackend(a) == canonBackend(b) }

// canonBackend folds the spellings of one repository together: file:// URLs,
// "~", relative paths, symlinks and a trailing slash. Remote URLs are only
// trimmed of the trailing slash.
func canonBackend(u string) string {
	u = strings.TrimSpace(u)
	if rest, ok := strings.CutPrefix(u, "file://"); ok {
		if p, err := url.PathUnescape(rest); err == nil {
			u = p
		} else {
			u = rest
		}
	}
	if !IsLocalPath(u) {
		return strings.TrimRight(u, "/")
	}
	if rest, ok := strings.CutPrefix(u, "~"); ok && (rest == "" || rest[0] == '/') {
		if home, err := os.UserHomeDir(); err == nil {
			u = home + rest
		}
	}
	if abs, err := filepath.Abs(u); err == nil {
		u = abs
	}
	if real, err := filepath.EvalSymlinks(u); err == nil {
		u = real
	}
	return filepath.Clean(u)
}
