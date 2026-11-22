// Design: docs/contributing/ze-go-style.md -- commit-time Go formatting and newly added style checks
//
// gostyle.go registers a diff-scoped change-set check into `./le commit create`
// backed by internal/le/go/style so Ze's Go formatting and AST-level style
// rules hold identically across every agent harness (Claude, Codex,
// Antigravity, and shell writes), without tripping pre-existing legacy debt.

package repo

import (
	"strings"

	"github.com/ze-software/ze/internal/le/commit/changecheck"
	gostyle "github.com/ze-software/ze/internal/le/go/style"
)

const goStyleCheckName = "repo go-style check"

// ze point: go-standards/directives/never-write-these-forbidden-go-patterns
//
// CheckAddedGoStyle refuses commits carrying un-gofmt'd or mis-grouped Go files
// or newly added non-test Go lines that introduce forbidden patterns.
func CheckAddedGoStyle(change changecheck.Change) (string, error) {
	var hits []gostyle.Finding
	for _, rel := range change.Paths {
		if !strings.HasSuffix(rel, ".go") {
			continue
		}
		if strings.Contains(rel, "/testdata/") {
			continue
		}
		after, present, err := change.After(rel)
		if err != nil {
			return "", err
		}
		if !present {
			continue
		}
		if gostyle.IsGenerated(after) {
			continue
		}
		if fmtHit := gostyle.CheckFormat(rel, after); fmtHit != nil {
			hits = append(hits, *fmtHit)
		}
		if strings.HasSuffix(rel, "_test.go") {
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
		hits = append(hits, gostyle.CheckAdded(rel, before, after, gostyle.Options{})...)
	}
	return gostyle.FormatFindings(hits), nil
}
