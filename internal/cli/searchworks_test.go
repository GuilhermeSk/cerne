package cli

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

// worksFixture is perDocFixture plus two works about "parcelas".
func worksFixture(t *testing.T) (projDir string) {
	t.Helper()
	projDir = perDocFixture(t)
	ctx, err := resolvePaths(projDir)
	if err != nil {
		t.Fatal(err)
	}
	commitBackend(t, ctx.repoDir) // work start mutates from a clean HEAD
	for _, title := range []string{"Conciliar parcelas do acme", "Revisar fatura do orchard"} {
		if _, err := execCerne(t, "work", "start", title, "--agent", "ana", "--dir", projDir); err != nil {
			t.Fatalf("work start: %v", err)
		}
	}
	// work start refreshed the index before the docs were written by hand:
	// drop it so the first search rebuilds from the whole clone.
	if err := os.Remove(indexPathFor(ctx.repoDir)); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return projDir
}

// F-0408 crit. 2: the works group follows today's list, which is unchanged.
func TestSearchWorksGroupAfterTodaysList(t *testing.T) {
	plain, err := execCerne(t, "search", "parcelas", "--dir", perDocFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	out, err := execCerne(t, "search", "parcelas", "--dir", worksFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, plain) {
		t.Fatalf("today's list changed:\n%s\nwant prefix:\n%s", out, plain)
	}
	group := strings.TrimPrefix(out, plain)
	want := "\nWorks:\nW-0001 [active] ana — Conciliar parcelas do acme\n"
	if group != want {
		t.Fatalf("works group = %q, want %q", group, want)
	}
}

func TestSearchWorksNoGroupWithoutMatch(t *testing.T) {
	out, err := execCerne(t, "search", "cartão", "--dir", worksFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "Works:") {
		t.Fatalf("unexpected works group:\n%s", out)
	}
}

// F-0408 crit. 3: --json gains works and keeps the fields of today.
func TestSearchWorksJSON(t *testing.T) {
	out, err := execCerne(t, "search", "parcelas", "--json", "--dir", worksFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Results       []map[string]any `json:"results"`
		IndexSyncedAt any              `json:"index_synced_at"`
		Works         []map[string]any `json:"works"`
	}
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if len(m.Results) != 2 {
		t.Fatalf("results = %d, want 2", len(m.Results))
	}
	if len(m.Works) != 1 || m.Works[0]["id"] != "W-0001" || m.Works[0]["state"] != "active" ||
		m.Works[0]["actor"] != "ana" || m.Works[0]["title"] != "Conciliar parcelas do acme" {
		t.Fatalf("works = %v", m.Works)
	}
}

// F-0408 crit. 4: ask never carries works.
func TestAskContextIgnoresWorks(t *testing.T) {
	plain, err := execCerne(t, "ask", "parcelas", "--json", "--dir", perDocFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	withWorks, err := execCerne(t, "ask", "parcelas", "--json", "--dir", worksFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if plain != withWorks {
		t.Fatalf("ask context changed with works:\n%s\nvs\n%s", withWorks, plain)
	}
}

// F-0408 crit. 1: work writes refresh the index in place (no rebuild by hand).
func TestSearchWorksSeesWritesWithoutRebuild(t *testing.T) {
	projDir := perDocFixture(t)
	if _, err := execCerne(t, "search", "parcelas", "--dir", projDir); err != nil { // builds the index
		t.Fatal(err)
	}
	steps := [][]string{
		{"work", "start", "Auditar o cofre", "--agent", "ana"},
		{"work", "log", "W-0001", "decidido usar snapshotmirror"},
		{"work", "done", "W-0001", "--note", "fechado com vaultkey novo"},
	}
	for _, s := range steps {
		if out, err := execCerne(t, append(s, "--dir", projDir)...); err != nil {
			t.Fatalf("%v: %v\n%s", s, err, out)
		}
	}
	for _, q := range []string{"cofre", "snapshotmirror", "vaultkey"} {
		out, err := execCerne(t, "search", q, "--dir", projDir)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "W-0001 [done]") {
			t.Errorf("search %q missed the work:\n%s", q, out)
		}
	}
}

// F-0408 crit. 3: works is always present, [] when nothing matches.
func TestSearchWorksJSONEmptyArray(t *testing.T) {
	out, err := execCerne(t, "search", "cartão", "--json", "--dir", worksFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"works": []`) {
		t.Fatalf("want \"works\": [] in:\n%s", out)
	}
}

// dropWorksTable turns the index at db into the old schema (no works_fts).
func dropWorksTable(t *testing.T, db string) {
	t.Helper()
	conn, err := sql.Open("sqlite", db)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Exec(`DROP TABLE works_fts`); err != nil {
		t.Fatal(err)
	}
}

func hasWorksTable(t *testing.T, db string) bool {
	t.Helper()
	conn, err := sql.Open("sqlite", db)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var n int
	if err := conn.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name = 'works_fts'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n > 0
}

// An index built by a binary older than F-0408 has no works table: the first
// search rebuilds it once, and a current index is left alone afterwards.
func TestSearchRebuildsIndexWithoutWorksTableOnce(t *testing.T) {
	projDir := worksFixture(t)
	if _, err := execCerne(t, "search", "parcelas", "--dir", projDir); err != nil { // builds
		t.Fatal(err)
	}
	ctx, err := resolvePaths(projDir)
	if err != nil {
		t.Fatal(err)
	}
	db := indexPathFor(ctx.repoDir)
	dropWorksTable(t, db)

	out, err := execCerne(t, "search", "parcelas", "--dir", projDir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "W-0001 [active]") || !hasWorksTable(t, db) {
		t.Fatalf("old index not rebuilt (table %v):\n%s", hasWorksTable(t, db), out)
	}

	// Current index: a marker in the file survives the next search.
	conn, err := sql.Open("sqlite", db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`INSERT INTO works_fts (work_id, state, actor, raw_title, title, body) VALUES ('W-9999', 'active', 'x', 'marcador', 'marcador', '')`); err != nil {
		t.Fatal(err)
	}
	conn.Close()
	out, err = execCerne(t, "search", "marcador", "--dir", projDir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "W-9999") {
		t.Fatalf("a current index was rebuilt:\n%s", out)
	}
}

// manyWorksFixture: seven active works about kpack, none in any document.
func manyWorksFixture(t *testing.T) string {
	t.Helper()
	projDir := perDocFixture(t)
	ctx, err := resolvePaths(projDir)
	if err != nil {
		t.Fatal(err)
	}
	commitBackend(t, ctx.repoDir)
	for i := 1; i <= 7; i++ {
		if out, err := execCerne(t, "work", "start", fmt.Sprintf("Mexer no kpack %d", i), "--agent", "ana", "--dir", projDir); err != nil {
			t.Fatalf("work start: %v\n%s", err, out)
		}
	}
	return projDir
}

// Round 2: a capped group never hides silently; --json carries the total.
func TestSearchWorksMoreLineAndTotal(t *testing.T) {
	projDir := manyWorksFixture(t)
	out, err := execCerne(t, "search", "kpack", "--dir", projDir)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(out, "] ana — Mexer no kpack"); got != 5 {
		t.Fatalf("shown = %d, want 5:\n%s", got, out)
	}
	if !strings.Contains(out, "… 2 more works match — cerne search --works \"kpack\"\n") {
		t.Fatalf("no more line:\n%s", out)
	}
	js, err := execCerne(t, "search", "kpack", "--json", "--dir", projDir)
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Works      []map[string]any `json:"works"`
		WorksTotal int              `json:"works_total"`
	}
	if err := json.Unmarshal([]byte(js), &m); err != nil || len(m.Works) != 5 || m.WorksTotal != 7 {
		t.Fatalf("works = %d, total = %d, err = %v", len(m.Works), m.WorksTotal, err)
	}
}

// Round 2: --works prints only the works group, past the default cap.
func TestSearchWorksFlag(t *testing.T) {
	projDir := manyWorksFixture(t)
	out, stderr, err := execCerneSplit(t, "search", "--works", "kpack", "--dir", projDir)
	if err != nil {
		t.Fatalf("%v\n%s", err, stderr)
	}
	if got := strings.Count(out, "] ana — Mexer no kpack"); got != 7 {
		t.Fatalf("listed = %d, want 7:\n%s", got, out)
	}
	if strings.Contains(out, "F-0001") || strings.Contains(out, "more works") {
		t.Fatalf("--works must print only works:\n%s", out)
	}
	none, _, err := execCerneSplit(t, "search", "--works", "inexistente", "--dir", projDir)
	if err != nil || !strings.Contains(none, "No works match") {
		t.Fatalf("empty = %q, %v", none, err)
	}
	js, _, err := execCerneSplit(t, "search", "--works", "kpack", "--json", "--dir", projDir)
	if err != nil || !strings.Contains(js, `"works_total": 7`) || strings.Contains(js, `"results"`) {
		t.Fatalf("json = %s, %v", js, err)
	}
}

// The works group prints the original title, never the folded one.
func TestSearchWorksPrintOriginalTitle(t *testing.T) {
	projDir := perDocFixture(t)
	ctx, err := resolvePaths(projDir)
	if err != nil {
		t.Fatal(err)
	}
	commitBackend(t, ctx.repoDir)
	if out, err := execCerne(t, "work", "start", "upgradeIfNewer breaks pods", "--agent", "ana", "--dir", projDir); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	for _, args := range [][]string{{"search", "Newer"}, {"search", "--works", "Newer"}, {"search", "Newer", "--json"}} {
		out, err := execCerne(t, append(args, "--dir", projDir)...)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "upgradeIfNewer breaks pods") || strings.Contains(out, "upgrade If Newer") {
			t.Errorf("%v printed:\n%s", args, out)
		}
	}
}
