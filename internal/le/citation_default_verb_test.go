// VALIDATES: AC-25 of spec-le-cli-usability -- a cited le line is judged by
// the resolution dispatch runs (leaction.List.AtVerb), so a line whose verb
// the area's default action supplies (leaction.Action.Default) is held.
// PREVENTS: the citation judge reporting `le weekly source <file>` unheld while
// dispatch runs it, because the judge read the first word as a verb and the
// area's own Answer did not.
package le

import (
	"strings"
	"testing"

	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// TestCitationJudgeAcceptsWhatDispatchAccepts holds the citation judge to the
// default verb every area table declares.
//
// Goal: every line dispatch reaches through a default verb is held, and a
// first word that is neither a verb nor the default action's keyword is still
// unheld. Method: derive the cases from the registry, every area whose table
// declares a default action, and judge the bare line, the verb-explicit line,
// each of the default action's keywords in the verb's place, and a word the
// table does not hold. The three spellings the stale-citation sweep met are
// named, so a registry that stopped declaring them cannot pass vacuously.
func TestCitationJudgeAcceptsWhatDispatchAccepts(t *testing.T) {
	named := map[string]string{
		"weekly":            "source",
		"repo working-tree": "max-areas",
		"go extract":        "source",
	}
	seen := make(map[string]bool, len(named))
	for _, name := range leroot.Owned() {
		list, declared := leroot.ActionsOf(name)
		if !declared {
			continue
		}
		for _, row := range list.Actions {
			if !row.Default {
				continue
			}
			area := strings.Fields(name)
			judge := func(words ...string) error {
				return leroot.JudgeCitation(append(append([]string{}, area...), words...))
			}
			if err := judge(); err != nil {
				t.Errorf("bare `le %s` is unheld: %v", name, err)
			}
			if err := judge(row.Verb); err != nil {
				t.Errorf("`le %s %s` is unheld: %v", name, row.Verb, err)
			}
			positional := false
			for _, parameter := range row.Parameters {
				if parameter.Keyword == "" {
					positional = true
					continue
				}
				if err := judge(parameter.Keyword); err != nil {
					t.Errorf("`le %s %s`, which dispatch runs as `%s %s`, is unheld: %v",
						name, parameter.Keyword, row.Verb, parameter.Keyword, err)
				}
			}
			if keyword, ok := named[name]; ok {
				seen[name] = true
				if err := judge(keyword); err != nil {
					t.Errorf("`le %s %s` is unheld: %v", name, keyword, err)
				}
			}
			// A positional value takes any first word, so no word is unheld there.
			if positional {
				continue
			}
			if err := judge("zz-held-by-nothing"); err == nil {
				t.Errorf("`le %s zz-held-by-nothing` is held, yet neither the table nor %s's grammar holds the word",
					name, row.Verb)
			}
		}
	}
	for name := range named {
		if !seen[name] {
			t.Errorf("le %s declares no default action, so its keyword-first line is not dispatched", name)
		}
	}
	if err := leroot.JudgeCitation([]string{"repo", "working-tree", "max-areas", "999"}); err != nil {
		t.Errorf("`le repo working-tree max-areas 999` is unheld: %v", err)
	}
}
