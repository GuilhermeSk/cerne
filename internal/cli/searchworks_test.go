package cli

import (
	"encoding/json"
	"errors"
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
