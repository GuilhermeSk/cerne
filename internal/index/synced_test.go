package index

import (
	"database/sql"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestRebuildRecordsSyncedAt(t *testing.T) {
	repoDir := seedBackend(t)
	db := filepath.Join(t.TempDir(), "index.db")
	at := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	if err := Rebuild(repoDir, db, at); err != nil {
		t.Fatal(err)
	}
	got, ok, err := SyncedAt(db)
	if err != nil || !ok || !got.Equal(at) {
		t.Fatalf("SyncedAt = %v, %v, %v; want %v, true, nil", got, ok, err, at)
	}
}

func TestRebuildZeroTimeRecordsNothing(t *testing.T) {
	repoDir := seedBackend(t)
	db := filepath.Join(t.TempDir(), "index.db")
	if err := Rebuild(repoDir, db, time.Time{}); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := SyncedAt(db); err != nil || ok {
		t.Fatalf("SyncedAt ok=%v err=%v; want false, nil", ok, err)
	}
}

func TestRebuildZeroTimeKeepsNoStaleTime(t *testing.T) {
	repoDir := seedBackend(t)
	db := filepath.Join(t.TempDir(), "index.db")
	at := time.Now().UTC()
	if err := Rebuild(repoDir, db, at); err != nil {
		t.Fatal(err)
	}
	if err := Rebuild(repoDir, db, time.Time{}); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := SyncedAt(db); ok {
		t.Fatal("a rebuild without a time must not keep the previous synced_at")
	}
}

func TestSyncedAtOnIndexWithoutMetaTable(t *testing.T) {
	db := filepath.Join(t.TempDir(), "old.db")
	conn, err := sql.Open("sqlite", db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`CREATE TABLE x(a)`); err != nil {
		t.Fatal(err)
	}
	conn.Close()
	if _, ok, err := SyncedAt(db); err != nil || ok {
		t.Fatalf("SyncedAt ok=%v err=%v; want false, nil", ok, err)
	}
}

func TestRebuildConcurrentLeavesWholeIndex(t *testing.T) {
	repoDir := seedBackend(t)
	db := filepath.Join(t.TempDir(), "index.db")
	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = Rebuild(repoDir, db, time.Now())
		}()
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatalf("concurrent Rebuild: %v", err)
		}
	}
	res, err := Search(db, "Postgres", 5)
	if err != nil || len(res) == 0 {
		t.Fatalf("index incomplete: %v, %v", res, err)
	}
}

// fakeFetch gives repoDir a FETCH_HEAD last touched at t.
func fakeFetch(t *testing.T, repoDir string, at time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	fh := filepath.Join(repoDir, ".git", "FETCH_HEAD")
	if err := os.WriteFile(fh, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(fh, at, at); err != nil {
		t.Fatal(err)
	}
}

func TestRebuildZeroTimeUsesFetchHead(t *testing.T) {
	repoDir := seedBackend(t)
	at := time.Date(2026, 11, 10, 8, 0, 0, 0, time.UTC)
	fakeFetch(t, repoDir, at)
	db := filepath.Join(t.TempDir(), "index.db")
	if err := Rebuild(repoDir, db, time.Time{}); err != nil {
		t.Fatal(err)
	}
	got, ok, err := SyncedAt(db)
	if err != nil || !ok || !got.Equal(at) {
		t.Fatalf("SyncedAt = %v, %v, %v; want %v, true, nil", got, ok, err, at)
	}
}

func TestUpdateZeroTimeFillsMissingFromFetchHead(t *testing.T) {
	repoDir := seedBackend(t)
	db := filepath.Join(t.TempDir(), "index.db")
	if err := Rebuild(repoDir, db, time.Time{}); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 11, 10, 8, 0, 0, 0, time.UTC)
	fakeFetch(t, repoDir, at)
	if err := Update(repoDir, db, []string{"F-0001"}, time.Time{}); err != nil {
		t.Fatal(err)
	}
	got, ok, _ := SyncedAt(db)
	if !ok || !got.Equal(at) {
		t.Fatalf("SyncedAt = %v, %v; want %v", got, ok, at)
	}
	// An explicit time already recorded is never replaced by the fetch time.
	explicit := time.Date(2026, 11, 14, 8, 0, 0, 0, time.UTC)
	if err := Update(repoDir, db, []string{"F-0001"}, explicit); err != nil {
		t.Fatal(err)
	}
	if err := Update(repoDir, db, []string{"F-0001"}, time.Time{}); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := SyncedAt(db); !got.Equal(explicit) {
		t.Fatalf("SyncedAt = %v; want %v kept", got, explicit)
	}
}
