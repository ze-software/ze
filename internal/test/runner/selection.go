// Design: docs/architecture/testing/ci-format.md -- common le test selection contract

package runner

import (
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Selection describes the common le test selection contract.
type Selection struct {
	All     bool
	Start   string
	Pattern string
	Args    []string
}

func (s Selection) requestsRun() bool {
	return s.All || s.Start != "" || s.Pattern != "" || len(s.Args) > 0
}

// Select activates tests matching the common le test selection contract.
func (ts *Tests) Select(sel Selection) (int, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	for _, rec := range ts.byNick {
		rec.Deactivate()
	}
	if !sel.requestsRun() {
		return 0, nil
	}

	candidates := ts.selectionCandidates(sel.Pattern)
	if len(candidates) == 0 {
		if sel.Pattern != "" {
			return 0, errNoPatternMatch(sel.Pattern)
		}
		return 0, nil
	}

	if sel.Start != "" {
		start := indexRecordSelector(candidates, sel.Start)
		if start < 0 {
			return 0, fmt.Errorf("start test %q not found", sel.Start)
		}
		candidates = candidates[start:]
	}

	if len(sel.Args) > 0 {
		if sel.All || sel.Start != "" {
			return 0, fmt.Errorf("test ids cannot be combined with --all or --start")
		}
		selected := 0
		for _, arg := range sel.Args {
			idx := indexRecordSelector(candidates, arg)
			if idx < 0 {
				return selected, fmt.Errorf("test %q not found", arg)
			}
			candidates[idx].Activate()
			selected++
		}
		return selected, nil
	}

	for _, rec := range candidates {
		rec.Activate()
	}
	return len(candidates), nil
}

func (ts *Tests) selectionCandidates(pattern string) []*Record {
	candidates := make([]*Record, 0, len(ts.ordered))
	for _, nick := range ts.ordered {
		rec := ts.byNick[nick]
		if pattern == "" || patternMatches(rec.Nick, rec.Name, pattern) {
			candidates = append(candidates, rec)
		}
	}
	return candidates
}

func indexRecordSelector(records []*Record, selector string) int {
	for i, rec := range records {
		if rec.Nick == selector || rec.Name == selector || rec.CIFile == selector {
			return i
		}
	}
	return -1
}

// patternMatches reports whether --pattern keeps the test whose id is nick and
// whose name is name: the pattern is a literal substring of the id or of the
// name, never a regex or a glob. Both suite drivers select through it.
//
// The .ci path is not matched. Discovery reads one flat <suite>/*.ci
// directory, so the path holds nothing per test beyond the name, and its
// leading part is the checkout location: a pattern that occurs there would
// select every test of the suite.
func patternMatches(nick, name, pattern string) bool {
	return strings.Contains(nick, pattern) || strings.Contains(name, pattern)
}

// errNoPatternMatch is the refusal of a --pattern that keeps no test, for both
// suite drivers. A selection of nothing would print usage and exit 0, so a
// mistyped pattern would read as a green run. The refusal names the matching
// rule, because a regex alternation or a glob typed as the pattern matches no
// test name and reads as a missing test.
func errNoPatternMatch(pattern string) error {
	problem := textbuf.New().Str("no tests matching pattern ").Quoted(pattern).Slice()
	return opererr.New(problem, patternRule, patternForms)
}

// patternRule is the why line of errNoPatternMatch: the rule patternMatches
// applies.
const patternRule = "--pattern keeps each test whose id or name contains this text as typed;" +
	" it is not a regex or a glob"

// patternForms is the fix line of errNoPatternMatch: the two selections that
// work. The runner does not know the command it serves, so it names the flag
// to add and the form to type rather than a whole command line.
const patternForms = "add --list to see every test name," +
	" or give each test name as its own argument in place of --pattern"

// disableAll deactivates every generic test.
func (ts *TestSet[T]) disableAll() {
	for _, test := range ts.tests {
		test.SetActive(false)
	}
}

// Select activates generic tests matching the common le test selection contract.
func (ts *TestSet[T]) Select(sel Selection) (int, error) {
	ts.disableAll()
	if !sel.requestsRun() {
		return 0, nil
	}

	candidates := ts.selectionCandidates(sel.Pattern)
	if len(candidates) == 0 {
		if sel.Pattern != "" {
			return 0, errNoPatternMatch(sel.Pattern)
		}
		return 0, nil
	}

	if sel.Start != "" {
		start := indexGenericSelector(candidates, sel.Start)
		if start < 0 {
			return 0, fmt.Errorf("start test %q not found", sel.Start)
		}
		candidates = candidates[start:]
	}

	if len(sel.Args) > 0 {
		if sel.All || sel.Start != "" {
			return 0, fmt.Errorf("test ids cannot be combined with --all or --start")
		}
		selected := 0
		for _, arg := range sel.Args {
			idx := indexGenericSelector(candidates, arg)
			if idx < 0 {
				return selected, fmt.Errorf("test %q not found", arg)
			}
			candidates[idx].SetActive(true)
			selected++
		}
		return selected, nil
	}

	for _, test := range candidates {
		test.SetActive(true)
	}
	return len(candidates), nil
}

func (ts *TestSet[T]) selectionCandidates(pattern string) []T {
	candidates := make([]T, 0, len(ts.tests))
	for _, test := range ts.tests {
		if pattern == "" || patternMatches(test.GetNick(), test.GetName(), pattern) {
			candidates = append(candidates, test)
		}
	}
	return candidates
}

func indexGenericSelector[T Testable](tests []T, selector string) int {
	for i, test := range tests {
		if test.GetNick() == selector || test.GetName() == selector {
			return i
		}
	}
	return -1
}
