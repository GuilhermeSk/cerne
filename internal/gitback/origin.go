package gitback

import (
	"bufio"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ErrForeignClone marks a cache clone whose origin is not the backend the
// project declares (F-0434).
var ErrForeignClone = errors.New("cache clone belongs to another backend")

// ErrUnverifiableClone marks a cache clone whose origin cannot be read (no
// origin remote, unreadable config): it cannot be proven to be ours (F-0434).
var ErrUnverifiableClone = errors.New("cache clone origin cannot be verified")

// CheckOrigin refuses a clone in repoDir whose origin is another backend. The
// cache is keyed by project_id only, so two projects sharing an id would share
// one clone; the clone is never touched here (the cache dir also holds rooms
// and messages), the caller just stops.
func CheckOrigin(repoDir, backend string) error {
	origin, err := readOrigin(repoDir)
	if err != nil {
		return fmt.Errorf("%w: %s: %v", ErrUnverifiableClone, repoDir, err)
	}
	if sameBackend(origin, backend) {
		return nil
	}
	return fmt.Errorf("%w: the cache clone at %s has origin %s but this project declares backend %s; "+
		"give this project its own CERNE_CACHE_DIR or a distinct project_id",
		ErrForeignClone, repoDir, redactURL(origin), redactURL(backend))
}

// readOrigin reads remote "origin"'s url straight from .git/config (no repo
// open: this runs on every command). A relative origin is relative to the
// clone, as git runs inside it. A gitfile (worktree) falls back to go-git.
func readOrigin(repoDir string) (string, error) {
	f, err := os.Open(filepath.Join(repoDir, ".git", "config"))
	if err != nil {
		if fi, serr := os.Stat(filepath.Join(repoDir, ".git")); serr == nil && !fi.IsDir() {
			return OriginURL(repoDir)
		}
		return "", err
	}
	defer f.Close()
	in := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") {
			in = strings.EqualFold(strings.Join(strings.Fields(strings.Trim(line, "[]")), " "), `remote "origin"`)
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok && in && strings.EqualFold(strings.TrimSpace(k), "url") {
			v = strings.Trim(strings.TrimSpace(v), `"`)
			if v != "" {
				if isLocalBackend(v) && !filepath.IsAbs(v) && !strings.HasPrefix(v, "~") && !strings.HasPrefix(v, "file://") {
					v = filepath.Join(repoDir, v)
				}
				return v, nil
			}
		}
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	return "", errors.New("no origin url")
}

// redactURL drops the userinfo of a scheme URL (a token must not reach an
// error message or an HTTP body) and the password of an scp-form one. The
// authority ends at the first "/", so an "@" in the path stays intact.
func redactURL(u string) string {
	if scheme, rest, ok := strings.Cut(u, "://"); ok {
		auth, path, hasPath := strings.Cut(rest, "/")
		auth = auth[strings.LastIndex(auth, "@")+1:]
		if hasPath {
			return scheme + "://" + auth + "/" + path
		}
		return scheme + "://" + auth
	}
	if at := strings.Index(u, "@"); at > 0 && !strings.Contains(u[:at], "/") {
		if c := strings.Index(u[:at], ":"); c >= 0 {
			return u[:c] + u[at:]
		}
	}
	return u
}

var winDrive = regexp.MustCompile(`^[A-Za-z]:[\\/]`)

// isLocalBackend is IsLocalPath plus Windows drive paths, which the scp-like
// pattern would otherwise take for host "C".
// A user-less scp form (host:path), which git accepts, is a remote too.
func isLocalBackend(u string) bool {
	return winDrive.MatchString(u) || (IsLocalPath(u) && !bareSCP.MatchString(u))
}

var bareSCP = regexp.MustCompile(`^[A-Za-z0-9_.-]+:`)

func sameBackend(a, b string) bool {
	a, b = fileURLToPath(strings.TrimSpace(a)), fileURLToPath(strings.TrimSpace(b))
	la, lb := isLocalBackend(a), isLocalBackend(b)
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

// fileURLToPath turns file:///x, file://localhost/x and file:///C:/x into a
// path.
func fileURLToPath(u string) string {
	rest, ok := strings.CutPrefix(u, "file://")
	if !ok {
		return u
	}
	rest = strings.TrimPrefix(rest, "localhost")
	if p, err := url.PathUnescape(rest); err == nil {
		rest = p
	}
	if len(rest) > 2 && rest[0] == '/' && winDrive.MatchString(rest[1:]) {
		rest = rest[1:]
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

var defaultPorts = map[string]string{"ssh": "22", "https": "443", "http": "80"}

// canonRemote folds the spellings of one remote: scp form and ssh://, the
// user and password, the scheme's default port, a trailing slash or ".git",
// and the host's case (the path's case is kept).
func canonRemote(u string) string {
	scheme, host, port, path := "ssh", "", "", ""
	if s, rest, ok := strings.Cut(u, "://"); ok {
		pu, err := url.Parse(s + "://" + rest)
		if err != nil {
			return strings.TrimRight(u, "/")
		}
		scheme, host, port, path = pu.Scheme, pu.Hostname(), pu.Port(), pu.Path
	} else {
		rest := u[strings.Index(u, "@")+1:] // -1+1 = 0 when there is no user
		host, path, _ = strings.Cut(rest, ":")
	}
	if port == defaultPorts[scheme] {
		port = ""
	}
	if port != "" {
		port = ":" + port
	}
	path = "/" + strings.TrimLeft(strings.TrimSuffix(strings.TrimRight(path, "/"), ".git"), "/")
	return scheme + "://" + strings.ToLower(host) + port + path
}
