// Design: docs/contributing/ze-go-style.md -- commit-time compound-guard gate
// Overview: compoundguard.go -- splittable `||` guard detection
package archcompoundguard

import (
	"go/token"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/le/commit/changecheck"
	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
)

// ze point: go-standards/directives/guard-with-early-returns-one-fact-per-guard
//
// judgeChange refuses a commit whose added shipped Go lines introduce a
// splittable terminating `if a || b { leave }` guard, while leaving untouched
// legacy guards alone.
func judgeChange(change changecheck.Change) (string, error) {
	paths := make([]string, 0, len(change.Paths))
	for _, rel := range change.Paths {
		if !judged(rel) {
			continue
		}
		paths = append(paths, rel)
	}
	if len(paths) == 0 {
		return "", nil
	}
	slices.Sort(paths)

	fset := token.NewFileSet()
	var findings Findings
	for _, rel := range paths {
		after, present, err := change.After(rel)
		if err != nil {
			return "", err
		}
		if !present {
			continue
		}
		var before string
		if change.Before != nil {
			prev, existed, beforeErr := change.Before(rel)
			if beforeErr != nil {
				return "", beforeErr
			}
			if existed {
				before = prev
			}
		}
		added := addedLines(before, after)
		if len(added) == 0 {
			continue
		}
		guards, err := ScanSource(fset, rel, after)
		if err != nil {
			// Syntax errors are caught by the compiler, not as compound-guard findings.
			continue
		}
		for _, guard := range guards {
			if touchesAdded(added, guard.Line, guard.LastLine) {
				findings = append(findings, Finding{File: rel, Line: guard.Line, Fn: guard.Fn})
			}
		}
	}
	if len(findings) == 0 {
		return "", nil
	}
	report := CheckReport{
		Base:     repochanged.LineBase{Commit: "HEAD", Reason: "commit change set"},
		Files:    len(paths),
		Findings: findings,
	}
	return report.Text(), nil
}

func addedLines(before, after string) map[int]bool {
	oldCounts := make(map[string]int)
	if before != "" {
		for line := range strings.SplitSeq(before, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				oldCounts[trimmed]++
			}
		}
	}
	added := make(map[int]bool)
	for idx, line := range strings.Split(after, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if oldCounts[trimmed] > 0 {
			oldCounts[trimmed]--
			continue
		}
		added[idx+1] = true
	}
	return added
}

func touchesAdded(added map[int]bool, first, last int) bool {
	for line := first; line <= last; line++ {
		if added[line] {
			return true
		}
	}
	return false
}
