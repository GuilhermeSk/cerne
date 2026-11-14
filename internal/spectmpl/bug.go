package spectmpl

import (
	"strings"

	"github.com/henriquesakata/cerne/internal/doclang"
)

// bugBar is the bar line a correction report is born with: the existing
// exemption for a fix with a regression test (T-0006), in each language.
var bugBar = map[doclang.Lang]string{
	doclang.PT: "Barra: não aplicável (correção com teste de regressão) — padrão alternativo: teste de regressão vermelho antes da correção e verde depois",
	doclang.EN: "Bar: not applicable (fix with a regression test) — alternative standard: a red regression test before the fix and green after",
}

// BugBarIn is the pre-filled bar line of a bug report in language l.
func BugBarIn(l doclang.Lang) string {
	if b, ok := bugBar[l]; ok {
		return b
	}
	return bugBar[doclang.PT]
}

var bugGuide = map[doclang.Lang][3]string{
	doclang.PT: {"O que acontece e onde (versão, comando, tela).", "Passos numerados até o defeito.", "O que deveria acontecer × o que aconteceu."},
	doclang.EN: {"What happens and where (version, command, screen).", "Numbered steps to the defect.", "What should happen × what happened."},
}

// BugBodyIn is the spec of a B- bug report (T-0006): the three fields in
// language l, then the pre-filled bar exemption. An empty field keeps its
// guide as a comment, so a half-filled report still reads as one.
func BugBodyIn(l doclang.Lang, context, steps, expected string) string {
	if _, ok := bugBar[l]; !ok {
		l = doclang.PT
	}
	var b strings.Builder
	for i, v := range []string{context, steps, expected} {
		b.WriteString("## " + doclang.Sections(doclang.Bug, l)[i] + "\n")
		if v = strings.TrimSpace(v); v != "" {
			b.WriteString(v + "\n\n")
		} else {
			b.WriteString("<!-- " + bugGuide[l][i] + " -->\n\n")
		}
	}
	return b.String() + bugBar[l] + "\n"
}
