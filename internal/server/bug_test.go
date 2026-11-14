package server

import (
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/henriquesakata/cerne/internal/app"
	"github.com/henriquesakata/cerne/internal/domain"
	"github.com/henriquesakata/cerne/internal/store"
)

// T-0006 criterion 5: the UI form posts prefix "B" with the three fields; the
// card is born approved with a human origin, even when the server runs inside
// an agent session (the person at the form is the human).
func TestServerCreatesBug(t *testing.T) {
	t.Setenv("CERNE_ACTOR", "agent")
	repoDir, projDir, _ := newGitBackend(t)

	var got struct {
		Feature store.Feature `json:"feature"`
	}
	rec := doJSON(t, New(repoDir, projDir, ""), http.MethodPost, "/api/features",
		`{"prefix":"B","title":"Salvar quebra","context":"O salvar falha","steps":"1. salvar","expected":"salvar ok"}`, &got)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
	if f := got.Feature; !strings.HasPrefix(f.ID, "B-") || f.State != domain.StateApproved || f.Origin != "human" {
		t.Fatalf("feature = %+v, want a B- approved with human origin", f)
	}
	spec, err := os.ReadFile(app.FeatureDocPath(repoDir, got.Feature.ID, "spec.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"O salvar falha", "1. salvar", "salvar ok", "Barra: não aplicável"} {
		if !strings.Contains(string(spec), want) {
			t.Errorf("spec missing %q:\n%s", want, spec)
		}
	}

	// The form's origin is human by construction: no relayed quote is invented.
	evs, err := store.ReadAllEvents(repoDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range evs {
		if e.EntityID == got.Feature.ID && e.Type == "feature.created" {
			if _, ok := e.Payload["on_behalf"]; ok {
				t.Fatalf("payload = %v, want no on_behalf key", e.Payload)
			}
			if e.Payload["approved"] != "by report" {
				t.Fatalf("payload = %v, want approved=by report", e.Payload)
			}
		}
	}

	var errBody map[string]string
	rec = doJSON(t, New(repoDir, projDir, ""), http.MethodPost, "/api/features",
		`{"prefix":"B","title":"Sem relato"}`, &errBody)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty report status = %d, want 422; body: %s", rec.Code, rec.Body.String())
	}
}
