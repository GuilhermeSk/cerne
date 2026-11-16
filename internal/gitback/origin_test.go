package gitback

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func cloneOf(t *testing.T, origin string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "repo")
	if err := Clone(origin, dir); err != nil {
		t.Fatalf("Clone: %v", err)
	}
	return dir
}

// TestCheckOrigin_EquivalentSpellings pins F-0434 criterion 3.
func TestCheckOrigin_EquivalentSpellings(t *testing.T) {
	bare := filepath.Join(t.TempDir(), "o.git")
	if err := InitBare(bare); err != nil {
		t.Fatal(err)
	}
	repo := cloneOf(t, bare)
	wd, _ := os.Getwd()
	rel, err := filepath.Rel(wd, bare)
	if err != nil {
		t.Fatal(err)
	}
	for name, spelling := range map[string]string{
		"absolute": bare, "relative": rel, "trailing slash": bare + "/", "file url": "file://" + bare,
	} {
		if err := CheckOrigin(repo, spelling); err != nil {
			t.Errorf("%s (%s): refused: %v", name, spelling, err)
		}
	}
}

func TestCheckOrigin_RefusesOtherBackend(t *testing.T) {
	a := filepath.Join(t.TempDir(), "a.git")
	b := filepath.Join(t.TempDir(), "b.git")
	for _, p := range []string{a, b} {
		if err := InitBare(p); err != nil {
			t.Fatal(err)
		}
	}
	err := CheckOrigin(cloneOf(t, a), b)
	if !errors.Is(err, ErrForeignClone) {
		t.Fatalf("err = %v, want ErrForeignClone", err)
	}
	if !strings.Contains(err.Error(), a) || !strings.Contains(err.Error(), b) {
		t.Fatalf("message must name both backends: %v", err)
	}
}

func TestSameBackend_Remotes(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"git@github.com:o/r.git", "ssh://git@github.com/o/r", true},
		{"git@GitHub.com:o/r", "ssh://git@github.com/o/r.git/", true},
		{"https://Host.example/o/r.git", "https://host.example/o/r/", true},
		{"https://user:tok@host.example/o/r", "https://host.example/o/r", true},
		{"git@github.com:o/r", "git@github.com:o/R", false},
		{"git@github.com:o/r", "git@github.com:o/r2", false},
		{"git@github.com:o/r", "/srv/o/r", false},
	} {
		if got := sameBackend(c.a, c.b); got != c.want {
			t.Errorf("sameBackend(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestSameBackend_LocalPaths(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real.git")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link.git")
	if err := os.Symlink(real, link); err != nil {
		t.Skip("no symlinks")
	}
	if !sameBackend(real, link) {
		t.Error("a symlinked path must equal its target")
	}
	t.Setenv("HOME", base)
	if !sameBackend("~/real.git", real) {
		t.Error("~ must expand to HOME")
	}
	if _, err := os.Stat(filepath.Join(base, "REAL.GIT")); err == nil && !sameBackend(real, filepath.Join(base, "REAL.GIT")) {
		t.Error("a case-differing path on a case-insensitive volume must be equal")
	}
	if sameBackend(real, filepath.Join(base, "other.git")) {
		t.Error("different paths must differ")
	}
}

func TestCheckOrigin_RedactsUserinfo(t *testing.T) {
	a := filepath.Join(t.TempDir(), "a.git")
	if err := InitBare(a); err != nil {
		t.Fatal(err)
	}
	err := CheckOrigin(cloneOf(t, a), "https://user:s3cret@host.example/o/r")
	if !errors.Is(err, ErrForeignClone) || strings.Contains(err.Error(), "s3cret") {
		t.Fatalf("err = %v, want ErrForeignClone without the token", err)
	}
}
