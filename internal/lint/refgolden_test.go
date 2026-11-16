package lint

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/henriquesakata/cerne/internal/domain"
	"github.com/henriquesakata/cerne/internal/store"
)

var updateRefGolden = flag.Bool("update", false, "rewrite internal/lint/testdata/ref_golden.json")

// TestRefToSupersededGolden pins the ref-to-superseded findings over needles
// by id, title and imported identifier in mixed case, including the Unicode
// simple-fold cases the (?i) of the regexp covers (U+212A Kelvin, U+017F long s)
// (F-0404): the cheap pre-filter must never change them.
func TestRefToSupersededGolden(t *testing.T) {
	dir := t.TempDir()
	if err := store.InitLayout(dir, store.Project{Name: "Demo"}); err != nil {
		t.Fatal(err)
	}
	at := time.Unix(0, 0).UTC()
	feats := []store.Feature{
		{ID: "F-0001", Title: "Ledger settlement flow", State: domain.StateSuperseded, SupersededBy: "F-0002"},
		{ID: "F-0002", Title: "Ledger settlement v2", State: domain.StateInDevelopment},
		{ID: "F-0003", Title: "Orchard Kelvin sensor", State: domain.StateRejected},
		{ID: "F-0004", Title: "Acme ſtore ingest", State: domain.StateSuperseded, SupersededBy: "F-0002"},
		{ID: "F-0005", Title: "Café Münster export", State: domain.StateRejected},
		{ID: "F-0006", Title: "Docs", State: domain.StateInDevelopment},
		{ID: "F-0008", Title: "Orchard \u212aelvin gauge", State: domain.StateRejected},
		{ID: "F-0007", Title: "Imported thing", State: domain.StateSuperseded, SupersededBy: "F-0002"},
	}
	for _, f := range feats {
		f.CreatedAt = at
		if err := store.SaveFeature(dir, f); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SaveHistorical(dir, store.Historical{ID: "H-0001", Name: "Bills", Aliases: []string{"BOLETOS", "ſettle"}, SupersededBy: "Finance", State: domain.StateSuperseded, CreatedAt: at}); err != nil {
		t.Fatal(err)
	}
	docs := map[string]string{
		"F-0006": "see f-0001 and LEDGER SETTLEMENT FLOW.\nalso f-0003, orchard Kelvin sensor.\nACME STORE INGEST and acme ſtore ingest.\n" +
			"CAFÉ MÜNSTER EXPORT here; café münster export too.\nboletos and Settle, plus case 0090 notes.\nORCHARD KELVIN GAUGE and orchard kelvin gauge.\nf-00011 is not a mention.\n",
		"F-0002": "clean doc, mentions F-0001 <!-- lint:ok-superseded F-0001 -->\n",
	}
	for id, body := range docs {
		d := filepath.Join(dir, "content", "features", id)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "spec.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	d := filepath.Join(dir, "content", "features", "F-0007")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "spec.md"), []byte("---\nimported_from: docs/process/cases/0090_plugin-autonomous-agent/\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	all, err := Run(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := refToSuperseded(all)
	b, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	b = append(b, '\n')
	golden := filepath.Join("testdata", "ref_golden.json")
	if *updateRefGolden {
		if err := os.WriteFile(golden, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != string(b) {
		t.Fatalf("ref-to-superseded findings changed:\n got: %s\nwant: %s", b, want)
	}
}
