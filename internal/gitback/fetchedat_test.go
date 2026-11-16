package gitback

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFetchedAt(t *testing.T) {
	repoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, ok := FetchedAt(repoDir); ok {
		t.Fatal("FetchedAt ok without a FETCH_HEAD")
	}
	fh := filepath.Join(repoDir, ".git", "FETCH_HEAD")
	if err := os.WriteFile(fh, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 11, 10, 8, 0, 0, 0, time.UTC)
	if err := os.Chtimes(fh, want, want); err != nil {
		t.Fatal(err)
	}
	got, ok := FetchedAt(repoDir)
	if !ok || !got.Equal(want) {
		t.Fatalf("FetchedAt = %v, %v; want %v, true", got, ok, want)
	}
}
