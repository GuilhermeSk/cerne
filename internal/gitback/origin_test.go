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
