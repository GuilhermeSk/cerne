package lint

import (
	"regexp"
	"strings"
	"testing"
)

// TestWordFilterNeverHidesARegexMatch pins that the folded-literal filter is a
// necessary condition of the (?i) regexp: wherever the regex matches, the
// filter must let it through (F-0404). Kelvin sign and long s fold to k and s.
func TestWordFilterNeverHidesARegexMatch(t *testing.T) {
	lits := []string{"kelvin", "KELVIN", "Kelvin", "stock", "ſtock", "Café", "ÉTÉ", "F-0001", "ǅ"}
	texts := []string{
		"a kelvin b", "a KELVIN b", "a Kelvin b", "STOCK", "ſtock", "stock", "ſTOCK",
		"café", "CAFÉ", "été", "ÉTÉ", "f-0001", "x F-0001 y", "ǆ ǈ", "unrelated",
	}
	for _, l := range lits {
		w := newWord(l)
		for _, tx := range texts {
			rxHit := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(l) + `\b`).MatchString(tx)
			if got := w.match(tx, foldKey(tx)); got != rxHit {
				t.Errorf("lit %q text %q: match=%v, regexp=%v", l, tx, got, rxHit)
			}
			if rxHit && !strings.Contains(foldKey(tx), foldKey(l)) {
				t.Errorf("lit %q text %q: filter would hide a regexp match", l, tx)
			}
		}
	}
}
