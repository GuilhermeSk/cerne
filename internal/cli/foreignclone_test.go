package cli

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/henriquesakata/cerne/internal/gitback"
)

func bareHead(t *testing.T, bare string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", bare, "for-each-ref").CombinedOutput()
	if err != nil {
		t.Fatalf("for-each-ref %s: %v\n%s", bare, err, out)
	}
	return strings.TrimSpace(string(out))
}

// foreignPair returns project A (owner of the cache clone), its bare backend,
// and project B, which shares A's project_id and cache but declares another
// backend. The cache env is left pointing at A's cache.
func foreignPair(t *testing.T) (projA, originA, projB string) {
	t.Helper()
	projA, originA, _ = setupProject(t) // project_id "demo"
	cacheA := cacheRootEnv(t)
	// B is initialized against its own cache, then pointed at A's cache.
	t.Setenv("CERNE_CACHE_DIR", t.TempDir())
	projB = t.TempDir()
	if out, err := execInit(t, "--docs-language", "pt-BR", "--name", "demo", "--backend", newOrigin(t), "--dir", projB); err != nil {
		t.Fatalf("init B: %v\n%s", err, out)
	}
	t.Setenv("CERNE_CACHE_DIR", cacheA)
	return projA, originA, projB
}

// TestResolve_RefusesForeignClone pins F-0434 criterion 1: project B shares
// project A's project_id (hence its cache clone) but declares another
// backend; every command that reads or writes the clone refuses it and A's
// backend never moves.
func TestResolve_RefusesForeignClone(t *testing.T) {
	projA, originA, projB := foreignPair(t)
	headA := bareHead(t, originA)
	for _, args := range [][]string{
		{"brief", "--dir", projB},
		{"capability", "new", "Intruder", "--dir", projB},
		{"push", "--dir", projB},
		{"pull", "--dir", projB},
		{"sync", "--dir", projB},
		{"msg", "read", "--session", "s1", "--dir", projB},
	} {
		out, err := execCerne(t, args...)
		if !errors.Is(err, gitback.ErrForeignClone) {
			t.Fatalf("%v: err = %v, want ErrForeignClone\n%s", args, err, out)
		}
	}
	if got := bareHead(t, originA); got != headA {
		t.Fatalf("A's backend moved: %s -> %s", headA, got)
	}
	if _, err := execCerne(t, "brief", "--dir", projA); err != nil {
		t.Fatalf("A must keep working: %v", err)
	}
}

// A relative backend in .cernerc is relative to the .cernerc directory, not
// the process cwd (F-0434).
func TestResolve_RelativeBackendIsRelativeToCernerc(t *testing.T) {
	projA, originA, _ := setupProject(t)
	rel, err := filepath.Rel(projA, originA)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(projA, ".cernerc")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), originA) {
		t.Fatalf(".cernerc does not hold the backend path:\n%s", raw)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(raw), originA, rel, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	if out, err := execCerne(t, "brief", "--dir", projA); err != nil {
		t.Fatalf("brief from another cwd: %v\n%s", err, out)
	}
}

func cacheRootEnv(t *testing.T) string {
	t.Helper()
	return os.Getenv("CERNE_CACHE_DIR")
}
