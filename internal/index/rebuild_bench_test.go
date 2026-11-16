package index

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/henriquesakata/cerne/internal/domain"
	"github.com/henriquesakata/cerne/internal/store"
)

// benchBackend: 300 features with a spec, 100 notes and `works` works with a
// handoff on every 10th and a two-entry log on every 3rd (the shape of a
// real backend, where most works are a title and a note).
func benchBackend(b *testing.B, works int) string {
	b.Helper()
	repoDir := b.TempDir()
	now := time.Now().UTC()
	body := "Dividir compras em parcelas mensais sem juros, com conciliação no extrato do orchard. "
	for i := 1; i <= 300; i++ {
		id := fmt.Sprintf("F-%04d", i)
		if err := store.SaveFeature(repoDir, store.Feature{ID: id, Title: "Feature " + id, State: domain.StateCandidate, CreatedAt: now, UpdatedAt: now}); err != nil {
			b.Fatal(err)
		}
		dir := filepath.Join(repoDir, "content", "features", id)
		os.MkdirAll(dir, 0o755)
		os.WriteFile(filepath.Join(dir, "spec.md"), []byte(body+body+body), 0o644)
	}
	for i := 1; i <= works; i++ {
		id := fmt.Sprintf("W-%04d", i)
		w := store.Work{ID: id, Title: "Trabalho " + id, State: domain.WorkDone, Actor: "ana@laptop", Note: body, CreatedAt: now, UpdatedAt: now}
		if i%10 == 0 {
			w.HandoffAt = &now
			dir := filepath.Join(repoDir, "content", "works", id)
			os.MkdirAll(dir, 0o755)
			os.WriteFile(filepath.Join(dir, "handoff.md"), []byte(body), 0o644)
		}
		if err := store.SaveWork(repoDir, w); err != nil {
			b.Fatal(err)
		}
		if i%3 == 0 {
			for s := 1; s <= 2; s++ {
				store.SaveLogEntry(repoDir, id, store.LogEntry{Seq: s, UTC: "2026-11-01T10:00:00Z", Actor: "ana", RecordedBy: "s", Text: body})
			}
		}
	}
	return repoDir
}

func BenchmarkRebuild(b *testing.B) {
	for _, works := range []int{0, 1842} {
		b.Run(fmt.Sprintf("works=%d", works), func(b *testing.B) {
			repoDir := benchBackend(b, works)
			db := filepath.Join(b.TempDir(), "index.db")
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := Rebuild(repoDir, db, time.Time{}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
