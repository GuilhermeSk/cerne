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
	lits := []string{"kelvin", "KELVIN", "Kelvin", "stock", "ſtock", "Café", "ÉTÉ", "F-0001", "ǅ", "Orchard \u212aelvin gauge", "Orchard Kelvin gauge", ""}
	texts := []string{
		"a kelvin b", "a KELVIN b", "a Kelvin b", "STOCK", "ſtock", "stock", "ſTOCK",
		"café", "CAFÉ", "été", "ÉTÉ", "f-0001", "x F-0001 y", "ǆ ǈ", "unrelated", "ORCHARD KELVIN GAUGE", "orchard kelvin gauge", "orchard \u212aelvin gauge", "Kelvin", "x",
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

// The gate itself: a regex that WOULD match is not run when the folded literal
// is absent, and an empty literal falls through to the regex.
func TestWordGateSkipsTheRegex(t *testing.T) {
	w := word{rx: wordRx("abc"), lit: foldKey("zzz")}
	if !w.rx.MatchString("abc") {
		t.Fatal("fixture: the regex must match")
	}
	if w.match("abc", foldKey("abc")) {
		t.Fatal("the folded literal is absent: the regex must not run")
	}
	e := newWord("")
	for _, tx := range []string{"", "x y"} {
		if got, want := e.match(tx, foldKey(tx)), e.rx.MatchString(tx); got != want {
			t.Errorf("empty literal on %q: got %v, regex %v", tx, got, want)
		}
	}
}
