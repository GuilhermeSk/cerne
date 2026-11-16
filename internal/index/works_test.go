package index

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/henriquesakata/cerne/internal/domain"
	"github.com/henriquesakata/cerne/internal/store"
)

func saveWorkT(t *testing.T, repoDir string, w store.Work) {
	t.Helper()
	now := time.Now().UTC()
	w.CreatedAt, w.UpdatedAt = now, now
	if err := store.SaveWork(repoDir, w); err != nil {
		t.Fatal(err)
	}
}

func workIDs(t *testing.T, db, q string) []string {
	t.Helper()
	res, err := SearchWorks(db, q, 5)
	if err != nil {
		t.Fatalf("SearchWorks(%q): %v", q, err)
	}
	var ids []string
	for _, r := range res {
		ids = append(ids, r.ID)
	}
	return ids
}

// seedWorks adds W-0001 (title, note, handoff, log) to a seeded backend.
func seedWorks(t *testing.T, repoDir string) {
	t.Helper()
	now := time.Now().UTC()
	saveWorkT(t, repoDir, store.Work{ID: "W-0001", Title: "rotacao de credencial do orchard",
		State: domain.WorkDone, Actor: "ana@laptop", Note: "fechado com vaultkey novo", HandoffAt: &now})
	dir := filepath.Join(repoDir, "content", "works", "W-0001")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "handoff.md"), []byte("falta o cutover do ledgerproxy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveLogEntry(repoDir, "W-0001", store.LogEntry{Seq: 1, UTC: "2026-11-01T10:00:00Z",
		Actor: "ana", RecordedBy: "s", Text: "decidido usar snapshotmirror"}); err != nil {
		t.Fatal(err)
	}
}

func TestWorksFoundByEachField(t *testing.T) {
	repoDir := seedBackend(t)
	seedWorks(t, repoDir)
	db := filepath.Join(t.TempDir(), "index.db")
	if err := Rebuild(repoDir, db, time.Time{}); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"orchard", "vaultkey", "ledgerproxy", "snapshotmirror"} {
		ids := workIDs(t, db, q)
		if len(ids) != 1 || ids[0] != "W-0001" {
			t.Errorf("SearchWorks(%q) = %v, want [W-0001]", q, ids)
		}
	}
	res, _ := SearchWorks(db, "orchard", 5)
	if r := res[0]; r.State != "done" || r.Actor != "ana@laptop" || r.Title != "rotacao de credencial do orchard" {
		t.Errorf("result = %+v", r)
	}
	// The entity search never sees works.
	if got := hits(t, db, "orchard"); len(got) != 0 {
		t.Errorf("SearchEntities saw a work: %v", got)
	}
}

func TestUpdateRefreshesTouchedWork(t *testing.T) {
	repoDir := seedBackend(t)
	seedWorks(t, repoDir)
	db := filepath.Join(t.TempDir(), "index.db")
	if err := Rebuild(repoDir, db, time.Time{}); err != nil {
		t.Fatal(err)
	}
	saveWorkT(t, repoDir, store.Work{ID: "W-0001", Title: "outro assunto gadgetron", State: domain.WorkDone, Actor: "ana@laptop"})
	saveWorkT(t, repoDir, store.Work{ID: "W-0002", Title: "novo trabalho frobnicate", State: domain.WorkActive, Actor: "bo"})
	if err := Update(repoDir, db, []string{"W-0001", "W-0002"}, time.Time{}); err != nil {
		t.Fatal(err)
	}
	if got := workIDs(t, db, "gadgetron"); len(got) != 1 {
		t.Errorf("edited work not found: %v", got)
	}
	if got := workIDs(t, db, "frobnicate"); len(got) != 1 {
		t.Errorf("new work not found: %v", got)
	}
	if got := workIDs(t, db, "orchard"); len(got) != 0 {
		t.Errorf("old title still found: %v", got)
	}
}

func TestIDsFromPathsWorks(t *testing.T) {
	got := IDsFromPaths([]string{"content/works/W-0001/handoff.md", "content/worklogs/W-0002/3.json", "entities/works/W-0003.json"})
	want := []string{"W-0001", "W-0002", "W-0003"}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("IDsFromPaths = %v, want %v", got, want)
	}
}

func TestSearchWorksOnIndexWithoutTable(t *testing.T) {
	repoDir, db := builtIndex(t)
	_ = repoDir
	conn, err := openDB(db)
	if err != nil {
		t.Fatal(err)
	}
	conn.Exec(`DROP TABLE works_fts`)
	conn.Close()
	if got, err := SearchWorks(db, "orchard", 5); err != nil || len(got) != 0 {
		t.Fatalf("SearchWorks on old index = %v, %v; want empty, nil", got, err)
	}
}

// SearchWorks reads the query and folds the text like the docs table does.
func TestWorksQueryAndFoldingLikeDocs(t *testing.T) {
	repoDir := seedBackend(t)
	now := time.Now().UTC()
	saveWorkT(t, repoDir, store.Work{ID: "W-0001", Title: "revisar WorkspaceSwitcher", State: domain.WorkActive, Actor: "bo", HandoffAt: &now})
	dir := filepath.Join(repoDir, "content", "works", "W-0001")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "handoff.md"), []byte("ver foo_bar e parseHTTPRequest\n"), 0o644)
	db := filepath.Join(t.TempDir(), "index.db")
	if err := Rebuild(repoDir, db, time.Time{}); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"foo_bar", "Switcher", "HTTP", "foo_bar parseHTTPRequest"} {
		if got := workIDs(t, db, q); len(got) != 1 {
			t.Errorf("SearchWorks(%q) = %v, want [W-0001] (Search finds it: %v)", q, got, hits(t, db, q))
		}
	}
}

func TestWorkHandoffIndexedWithoutHandoffAt(t *testing.T) {
	repoDir := seedBackend(t)
	saveWorkT(t, repoDir, store.Work{ID: "W-0001", Title: "x", State: domain.WorkActive, Actor: "bo"})
	dir := filepath.Join(repoDir, "content", "works", "W-0001")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "handoff.md"), []byte("passagem sem carimbo quuxmark\n"), 0o644)
	db := filepath.Join(t.TempDir(), "index.db")
	if err := Rebuild(repoDir, db, time.Time{}); err != nil {
		t.Fatal(err)
	}
	if got := workIDs(t, db, "quuxmark"); len(got) != 1 {
		t.Errorf("handoff without handoff_at not indexed: %v", got)
	}
}

func TestUpdateOnOldIndexDoesNotCreateWorksTable(t *testing.T) {
	repoDir, db := builtIndex(t)
	conn, err := openDB(db)
	if err != nil {
		t.Fatal(err)
	}
	conn.Exec(`DROP TABLE works_fts`)
	conn.Close()
	saveWorkT(t, repoDir, store.Work{ID: "W-0001", Title: "x frobnicate", State: domain.WorkActive, Actor: "bo"})
	saveNoteT(t, repoDir, "N-0009", "zebra listrada")
	if err := Update(repoDir, db, []string{"W-0001", "N-0009"}, time.Time{}); err != nil {
		t.Fatalf("Update on an old index: %v", err)
	}
	if HasWorks(db) {
		t.Fatal("Update created works_fts holding only the touched works")
	}
	if got := hits(t, db, "zebra"); len(got) != 1 {
		t.Errorf("note not updated: %v", got)
	}
}
