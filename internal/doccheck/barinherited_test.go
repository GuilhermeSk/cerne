package doccheck

import (
	"errors"
	"strings"
	"testing"
)

// TestBarInherited (T-0007 crit. 4): the inherited form is a bar of its own
// kind — neither named nor exempt — and owes no judge question (the parent's
// carries it); with no id it is malformed, naming the parent's id as the fix.
func TestBarInherited(t *testing.T) {
	for _, tc := range []struct {
		name, spec, parent string
		inherited, named   bool
	}{
		{"pt-BR", "## Verificação\n\nBarra: herdada de E-0001\n", "E-0001", true, false},
		{"en", "## Verification\n\nBar: inherited from e-0002 — the epic's round covers it\n", "E-0002", true, false},
		{"named", "## Verificação\n\nBarra: work_gate do cerne — .cernerc\n", "", false, true},
		{"not applicable", "## Verificação\n\nBarra: não aplicável (sem artefato) — padrão alternativo: testes\n", "", false, false},
		{"waived", "## Verificação\n\nBarra: dispensada por decisão humana (\"pode\")\n", "", false, false},
		{"no bar", "## Verificação\n\nSó testes.\n", "", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parent, ok := BarInherited([]byte(tc.spec))
			if ok != tc.inherited || parent != tc.parent {
				t.Errorf("BarInherited = %q, %v; want %q, %v", parent, ok, tc.parent, tc.inherited)
			}
			if got := BarNamed([]byte(tc.spec)); got != tc.named {
				t.Errorf("BarNamed = %v, want %v", got, tc.named)
			}
			if q, err := SpecJudgeQuestion([]byte(tc.spec)); tc.inherited && (q != "" || err != nil) {
				t.Errorf("SpecJudgeQuestion on an inherited bar = %q, %v; want none charged", q, err)
			}
		})
	}
	_, err := SpecBar([]byte("Barra: herdada de\n"))
	if !errors.Is(err, ErrBarMalformed) || !strings.Contains(err.Error(), `"herdada de" needs the parent's id`) {
		t.Fatalf("inherited with no id: err = %v, want ErrBarMalformed naming the parent's id", err)
	}
}
