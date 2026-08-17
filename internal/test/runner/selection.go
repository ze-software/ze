// Design: docs/architecture/testing/ci-format.md -- common ze-test selection contract

package runner

import (
	"fmt"
	"strings"
)

// Selection describes the common ze-test selection contract.
type Selection struct {
	All     bool
	Start   string
	Pattern string
	Args    []string
}

func (s Selection) requestsRun() bool {
	return s.All || s.Start != "" || s.Pattern != "" || len(s.Args) > 0
}

// Select activates tests matching the common ze-test selection contract.
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
			return 0, fmt.Errorf("no tests matching pattern %q", sel.Pattern)
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
		if pattern == "" || recordMatches(rec, pattern) {
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

func recordMatches(rec *Record, pattern string) bool {
	return strings.Contains(rec.Nick, pattern) || strings.Contains(rec.Name, pattern) || strings.Contains(rec.CIFile, pattern)
}

// disableAll deactivates every generic test.
func (ts *TestSet[T]) disableAll() {
	for _, test := range ts.tests {
		test.SetActive(false)
	}
}

// Select activates generic tests matching the common ze-test selection contract.
func (ts *TestSet[T]) Select(sel Selection) (int, error) {
	ts.disableAll()
	if !sel.requestsRun() {
		return 0, nil
	}

	candidates := ts.selectionCandidates(sel.Pattern)
	if len(candidates) == 0 {
		if sel.Pattern != "" {
			return 0, fmt.Errorf("no tests matching pattern %q", sel.Pattern)
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
		if pattern == "" || genericMatches(test, pattern) {
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

func genericMatches[T Testable](test T, pattern string) bool {
	return strings.Contains(test.GetNick(), pattern) || strings.Contains(test.GetName(), pattern)
}
