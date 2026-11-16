package cli

import (
	"errors"
	"os"
	"os/exec"
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

// TestResolve_RefusesForeignClone pins F-0434 criterion 1: project B shares
// project A's project_id (hence its cache clone) but declares another
// backend; B is refused and A's backend never moves.
func TestResolve_RefusesForeignClone(t *testing.T) {
	projA, originA, _ := setupProject(t) // project_id "demo", cache C1
	headA := bareHead(t, originA)
	cacheA := cacheRootEnv(t)

	// B is initialized against its own cache, then pointed at A's cache.
	t.Setenv("CERNE_CACHE_DIR", t.TempDir())
	originB := newOrigin(t)
	projB := t.TempDir()
	if out, err := execInit(t, "--docs-language", "pt-BR", "--name", "demo", "--backend", originB, "--dir", projB); err != nil {
		t.Fatalf("init B: %v\n%s", err, out)
	}
	t.Setenv("CERNE_CACHE_DIR", cacheA)

	for _, args := range [][]string{
		{"brief", "--dir", projB},
		{"capability", "new", "Intruder", "--dir", projB},
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

func cacheRootEnv(t *testing.T) string {
	t.Helper()
	return os.Getenv("CERNE_CACHE_DIR")
}
