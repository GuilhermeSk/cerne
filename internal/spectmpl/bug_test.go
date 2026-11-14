package spectmpl

import (
	"slices"
	"strings"
	"testing"

	"github.com/henriquesakata/cerne/internal/doccheck"
	"github.com/henriquesakata/cerne/internal/doclang"
)

// T-0006 criterion 1: the report is three sections plus the pre-filled bar
// exemption, in both documentation languages.
func TestBugSpecScaffoldsReport(t *testing.T) {
	for _, l := range []doclang.Lang{doclang.PT, doclang.EN} {
		t.Run(string(l), func(t *testing.T) {
			body := BugBodyIn(l, "ctx here", "1. do x", "")
			if got, want := headings(body), doclang.Sections(doclang.Bug, l); !slices.Equal(got, want) {
				t.Fatalf("headings = %q, want %q", got, want)
			}
			for _, f := range []string{"ctx here", "1. do x"} {
				if !strings.Contains(body, f) {
					t.Errorf("field %q missing from body", f)
				}
			}
			bar, err := doccheck.SpecBar([]byte(body))
			if err != nil || !doccheck.BarExempt([]byte(body)) {
				t.Fatalf("SpecBar = %q, %v; want a valid exempt bar", bar, err)
			}
		})
	}
	if !strings.Contains(BugBodyIn(doclang.PT, "c", "s", "e"),
		"Barra: não aplicável (correção com teste de regressão) — padrão alternativo: teste de regressão vermelho antes da correção e verde depois") {
		t.Error("the pt-BR bar line is not the one the spec pins")
	}
}
