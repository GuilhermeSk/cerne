package index

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/henriquesakata/cerne/internal/store"
)

// Works live in their own FTS table, so adding them never moves the bm25
// statistics (and the order) of the docs table (F-0408).
const (
	dropWorksSQL   = `DROP TABLE IF EXISTS works_fts`
	createWorksSQL = `CREATE VIRTUAL TABLE IF NOT EXISTS works_fts USING fts5(
		work_id, state UNINDEXED, actor UNINDEXED, title, body)`
	insertWorkSQL = `INSERT INTO works_fts (work_id, state, actor, title, body) VALUES (?, ?, ?, ?, ?)`
	searchWorkSQL = `SELECT work_id, state, actor, title FROM works_fts
		WHERE works_fts MATCH ? ORDER BY bm25(works_fts, 1.0, 0, 0, 4.0, 1.0) LIMIT ?`
)

// workRow is one work to insert: body joins the closing note, the handoff and
// the log entries' texts.
type workRow struct {
	id, state, actor, title, body string
}

// WorkResult is one work found by SearchWorks.
type WorkResult struct {
	ID, State, Actor, Title string
}

// workRowOf reads a work's handoff and log. logged, when not nil, is the set of
// works that have a log directory: the rest skip the lookup.
func workRowOf(repoDir string, w store.Work, logged map[string]bool) (workRow, error) {
	handoff, _, err := readDoc(filepath.Join(repoDir, "content", "works", w.ID, "handoff.md"))
	if err != nil {
		return workRow{}, err
	}
	var log []store.LogEntry
	if logged == nil || logged[w.ID] {
		var err error
		if log, err = store.LoadLog(repoDir, w.ID); err != nil {
			return workRow{}, err
		}
	}
	parts := []string{w.Note, handoff}
	for _, e := range log {
		parts = append(parts, e.Text)
	}
	return workRow{w.ID, string(w.State), w.Actor, w.Title, strings.TrimSpace(strings.Join(parts, "\n"))}, nil
}

func collectWorkRows(repoDir string) ([]workRow, error) {
	works, err := store.LoadAllWorks(repoDir)
	if err != nil {
		return nil, err
	}
	// One listing instead of one failed lookup per work with no log.
	logged := map[string]bool{}
	dirs, err := os.ReadDir(filepath.Join(repoDir, "content", "worklogs"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	for _, d := range dirs {
		logged[d.Name()] = true
	}
	rows := make([]workRow, 0, len(works))
	for _, w := range works {
		r, err := workRowOf(repoDir, w, logged)
		if err != nil {
			return nil, err
		}
		rows = append(rows, r)
	}
	return rows, nil
}

// collectWorkRowsFor is collectWorkRows restricted to the W- ids in ids.
func collectWorkRowsFor(repoDir string, ids []string) ([]workRow, error) {
	var rows []workRow
	for _, id := range slices.Sorted(slices.Values(ids)) {
		if !strings.HasPrefix(id, "W-") {
			continue
		}
		w, err := store.LoadWork(repoDir, id)
		if errors.Is(err, store.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		r, err := workRowOf(repoDir, w, nil)
		if err != nil {
			return nil, err
		}
		rows = append(rows, r)
	}
	return rows, nil
}

func insertWorkRows(tx *sql.Tx, rows []workRow) error {
	stmt, err := tx.Prepare(insertWorkSQL)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, r := range rows {
		if _, err := stmt.Exec(r.id, r.state, r.actor, foldIdentifiers(r.title), foldIdentifiers(r.body)); err != nil {
			return err
		}
	}
	return nil
}

// SearchWorks finds works matching query, best bm25 first, up to limit. Like
// Search it retries a syntax error quoted, and a miss on a multi-keyword plain
// query with the keywords ORed. An index built before works were indexed has
// no table: that is no works, not an error.
func SearchWorks(dbPath, query string, limit int) ([]WorkResult, error) {
	if _, err := os.Stat(dbPath); err != nil {
		return nil, err
	}
	db, err := openDB(dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	res, err := runWorkQuery(db, query, limit)
	if err != nil {
		res, err = runWorkQuery(db, quotePhrase(query), limit)
	}
	if err == nil && len(res) == 0 && !ftsSyntax.MatchString(query) && len(keywordTerms(query)) >= 2 {
		res, err = runWorkQuery(db, Keywords(query), limit)
	}
	if err != nil && strings.Contains(err.Error(), "no such table") {
		return nil, nil
	}
	return res, err
}

func runWorkQuery(db *sql.DB, query string, limit int) ([]WorkResult, error) {
	rows, err := db.Query(searchWorkSQL, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WorkResult
	for rows.Next() {
		var r WorkResult
		if err := rows.Scan(&r.ID, &r.State, &r.Actor, &r.Title); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
