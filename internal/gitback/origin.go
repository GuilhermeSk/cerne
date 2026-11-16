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
		ErrForeignClone, repoDir, redactURL(origin), redactURL(backend))
}

// redactURL drops the userinfo of a scheme URL (a token must not reach an
// error message or an HTTP body); the scp form's "git@" is not a secret.
func redactURL(u string) string {
	scheme, rest, ok := strings.Cut(u, "://")
	if !ok {
		return u
	}
	return scheme + "://" + rest[strings.LastIndex(rest, "@")+1:]
}

func sameBackend(a, b string) bool {
	a, b = fileURLToPath(strings.TrimSpace(a)), fileURLToPath(strings.TrimSpace(b))
	la, lb := IsLocalPath(a), IsLocalPath(b)
	if la != lb {
		return false
	}
	if !la {
		return canonRemote(a) == canonRemote(b)
	}
	a, b = expandLocal(a), expandLocal(b)
	if fa, err := os.Stat(a); err == nil {
		if fb, err := os.Stat(b); err == nil && os.SameFile(fa, fb) {
			return true // symlinks, case-insensitive volumes
		}
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func fileURLToPath(u string) string {
	rest, ok := strings.CutPrefix(u, "file://")
	if !ok {
		return u
	}
	if p, err := url.PathUnescape(rest); err == nil {
		return p
	}
	return rest
}

// expandLocal makes a local path absolute (against the process cwd; callers
// with a .cernerc directory resolve relative backends before calling).
func expandLocal(u string) string {
	if rest, ok := strings.CutPrefix(u, "~"); ok && (rest == "" || rest[0] == '/') {
		if home, err := os.UserHomeDir(); err == nil {
			u = home + rest
		}
	}
	if abs, err := filepath.Abs(u); err == nil {
		u = abs
	}
	return u
}

// canonRemote folds the spellings of one remote: scp form and ssh://, a
// trailing slash or ".git", the host's case (the path's case is kept) and any
// password/token.
func canonRemote(u string) string {
	if !strings.Contains(u, "://") { // scp form user@host:path
		if at := strings.Index(u, "@"); at >= 0 {
			if host, path, ok := strings.Cut(u[at+1:], ":"); ok {
				u = "ssh://" + u[:at+1] + host + "/" + strings.TrimLeft(path, "/")
			}
		} else if host, path, ok := strings.Cut(u, ":"); ok {
			u = "ssh://" + host + "/" + strings.TrimLeft(path, "/")
		}
	}
	pu, err := url.Parse(u)
	if err != nil {
		return strings.TrimRight(u, "/")
	}
	user := ""
	if pu.User != nil && pu.Scheme == "ssh" {
		user = pu.User.Username() + "@"
	}
	path := strings.TrimSuffix(strings.TrimRight(pu.Path, "/"), ".git")
	return pu.Scheme + "://" + user + strings.ToLower(pu.Host) + path
}
