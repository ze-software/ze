// VALIDATES: the cut grammar of `le verify worktree`: neither keyword is the
// uncut run, `part <i> of <m>` is one piece, `all of <m>` is the sweep, and
// every malformed form is refused naming the form that works (moved from the
// debt-clear grammar test).
// PREVENTS: a guess at a half-written cut judging the wrong subset.
package verify

import (
	"strings"
	"testing"

	leaction "github.com/ze-software/ze/internal/le/le/action"
)

func TestWorktreeCutKeywordsTakeBothOrNeither(t *testing.T) {
	none := func(string) string { return "" }
	for name, accepted := range map[string]struct {
		args  leaction.Arguments
		want  Options
		sweep bool
	}{
		"neither keyword": {leaction.Arguments{}, Options{}, false},
		"part 3 of 6":     {leaction.Arguments{keywordPart: {"3"}, keywordOf: {"6"}}, Options{Part: 3, Parts: 6}, false},
		"all of 6":        {leaction.Arguments{keywordAll: {""}, keywordOf: {"6"}}, Options{Parts: 6}, true},
	} {
		run, err := worktreeFrom(accepted.args, none)
		if err != nil {
			t.Errorf("%s was refused: %v", name, err)
			continue
		}
		if run.options != accepted.want || run.sweep != accepted.sweep {
			t.Errorf("%s answered %+v sweep %v, want %+v sweep %v", name, run.options, run.sweep, accepted.want, accepted.sweep)
		}
	}
	for name, refused := range map[string]struct {
		args leaction.Arguments
		form string
	}{
		"all part 2 of 3": {leaction.Arguments{keywordAll: {""}, keywordPart: {"2"}, keywordOf: {"3"}}, "all of <m>"},
		"all without of":  {leaction.Arguments{keywordAll: {""}}, "all of <m>"},
		"all first-red":   {leaction.Arguments{keywordAll: {""}, keywordOf: {"3"}, keywordFirstRed: {""}}, "first-red"},
		"part 2 alone":    {leaction.Arguments{keywordPart: {"2"}}, "part 2 of 0"},
		"of 6 alone":      {leaction.Arguments{keywordOf: {"6"}}, "part 0 of 6"},
		"part 0 of 3":     {leaction.Arguments{keywordPart: {"0"}, keywordOf: {"3"}}, "part 1 of 6"},
		"part 4 of 3":     {leaction.Arguments{keywordPart: {"4"}, keywordOf: {"3"}}, "4"},
		"all of zero":     {leaction.Arguments{keywordAll: {""}, keywordOf: {"0"}}, "all of <m>"},
	} {
		_, err := worktreeFrom(refused.args, none)
		if err == nil {
			t.Errorf("%s was accepted", name)
			continue
		}
		if !strings.Contains(err.Error(), refused.form) {
			t.Errorf("%s refused as %q, which does not name %q", name, err, refused.form)
		}
	}
}
