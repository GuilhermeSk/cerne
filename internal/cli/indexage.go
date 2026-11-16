package cli

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/henriquesakata/cerne/internal/gitback"
	"github.com/henriquesakata/cerne/internal/index"
)

// now is the clock of the index-age note; tests swap it.
var now = time.Now

// indexStaleAfter is the age past which search and ask warn (F-0394). Fixed:
// it is the limit a client project's session start already uses.
const indexStaleAfter = 24 * time.Hour

// indexSyncedAt reads when the index was last synced, nil when unknown. A read
// failure counts as unknown: search must not fail over a missing note. An index
// with no recorded time falls back to the clone's last fetch (F-0408), when
// that fetch is not newer than the index file: a newer fetch may hold entities
// the index never saw. Nothing is written back.
func indexSyncedAt(dbPath, repoDir string) *time.Time {
	t, ok, err := index.SyncedAt(dbPath)
	if err != nil {
		return nil
	}
	if ok {
		return &t
	}
	fetched, ok := gitback.FetchedAt(repoDir)
	if fi, serr := os.Stat(dbPath); !ok || serr != nil || fetched.After(fi.ModTime()) {
		return nil
	}
	return &fetched
}

// noteIndexAge prints, on stderr and before any result, that the index may
// miss what other machines wrote. A local-path backend has no other machine,
// so it stays silent there.
func noteIndexAge(w io.Writer, backend string, synced *time.Time) {
	if gitback.IsLocalPath(backend) {
		return
	}
	when := "last synced: unknown"
	if synced != nil {
		age := now().Sub(*synced)
		if age <= indexStaleAfter {
			return
		}
		when = fmt.Sprintf("last synced %dh ago", int(age.Hours()))
	}
	fmt.Fprintf(w, "note: search index %s; entities written on other machines may be missing (run cerne pull)\n", when)
}

// syncedAtJSON is the index_synced_at payload value: RFC 3339 or null.
func syncedAtJSON(synced *time.Time) *string {
	if synced == nil {
		return nil
	}
	s := synced.UTC().Format(time.RFC3339)
	return &s
}

// priorSyncedAt is the sync time dbPath already records, zero when none or no
// index: a rebuild passes it on, so only an index with no time falls back to
// the clone's last fetch.
func priorSyncedAt(dbPath string) time.Time {
	if _, err := os.Stat(dbPath); err != nil {
		return time.Time{}
	}
	t, ok, err := index.SyncedAt(dbPath)
	if err != nil || !ok {
		return time.Time{}
	}
	return t
}
