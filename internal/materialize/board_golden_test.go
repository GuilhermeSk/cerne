package materialize

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/henriquesakata/cerne/internal/domain"
	"github.com/henriquesakata/cerne/internal/store"
)

// seedBoardBackend: active work with a pending handoff, an active work with an
// old-style handoff date, a work idle past recentDays, a done work, and more
// than maxPendingHandoffs pending handoffs.
func seedBoardBackend(t *testing.T) (string, time.Time) {
	t.Helper()
	backend := seedBackend(t)
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	addWork(t, backend, "W-0001", "Migração", "ator", domain.WorkActive, now.Add(-5*time.Hour))
	addWork(t, backend, "W-0002", "Com passagem antiga", "ator", domain.WorkActive, now.Add(-72*time.Hour))
	at := now.Add(-48 * time.Hour)
	w, _ := store.LoadWork(backend, "W-0002")
	w.HandoffAt = &at
	if err := store.SaveWork(backend, w); err != nil {
		t.Fatal(err)
	}
	addWork(t, backend, "W-0003", "Parado", "ator", domain.WorkActive, now.AddDate(0, 0, -40))
	addWork(t, backend, "W-0004", "Parado antigo", "ator", domain.WorkActive, now.AddDate(0, 0, -60))
	addWork(t, backend, "W-0005", "Terminado", "ator", domain.WorkDone, now.Add(-time.Hour))
	for i := 1; i <= 12; i++ {
		ca := now.Add(-time.Duration(13-i) * time.Hour)
		h := store.Handoff{ID: fmt.Sprintf("HO-%04d", i), State: domain.HandoffPending, FromWork: fmt.Sprintf("W-%04d", 100+i), Actor: "agent:a@b", CreatedAt: ca, UpdatedAt: ca}
		if i == 12 {
			h.FromWork = "W-0001"
		}
		if err := store.SaveHandoff(backend, h); err != nil {
			t.Fatal(err)
		}
	}
	return backend, now
}

// F-0409 crit. 5: the CONTEXT.md "In progress" and "Pending handoffs" sections
// stay byte for byte what they were before the board builder was extracted.
func TestContextBoardGolden(t *testing.T) {
	backend, now := seedBoardBackend(t)
	docsDir := filepath.Join(t.TempDir(), ".docs")
	if _, err := MaterializeAt(backend, docsDir, "", false, 14, now); err != nil {
		t.Fatal(err)
	}
	got := readContext(t, docsDir)
	golden := filepath.Join("testdata", "context_board.golden")
	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatalf("CONTEXT.md differs from golden:\n--- got\n%s\n--- want\n%s", got, want)
	}
}
