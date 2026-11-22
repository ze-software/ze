// Design: docs/contributing/ze-go-style.md -- commit-time error-wrap gate
// Overview: errorwrap.go -- duplicate error-prefix trace
package archerrorwrap

import (
	"errors"
	"go/scanner"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/le/commit/changecheck"
	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
)

// ze point: user-facing-errors/directives/an-error-names-the-cause-and-the-fix
//
// judgeChange refuses a commit whose added shipped Go lines introduce a
// fmt.Errorf wrap repeating a `: `-separated segment the wrapped error already
// carries, while leaving untouched legacy wraps alone.
func judgeChange(change changecheck.Change) (string, error) {
	paths := make([]string, 0, len(change.Paths))
	overlay := make(map[string]overlayFile, len(change.Paths)+len(change.Removed))
	addedByPath := make(map[string]map[int]bool, len(change.Paths))

	for _, rel := range change.Removed {
		if !judged(rel) {
			continue
		}
		overlay[rel] = overlayFile{present: false}
	}

	for _, rel := range change.Paths {
		if !judged(rel) {
			continue
		}
		after, present, err := change.After(rel)
		if err != nil {
			return "", err
		}
		overlay[rel] = overlayFile{text: after, present: present}
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
		paths = append(paths, rel)
		addedByPath[rel] = added
	}
	if len(paths) == 0 {
		return "", nil
	}
	slices.Sort(paths)

	module := ""
	if change.Root != "" {
		if mod, err := modulePath(change.Root); err == nil {
			module = mod
		}
	}

	ix := newIndexWithOverlay(change.Root, module, overlay)
	report, err := judgeFiles(ix, paths, func(rel string, wrap site) bool {
		return touchesAdded(addedByPath[rel], wrap.line, wrap.lastLine)
	})
	if err != nil {
		if _, isSyntax := errors.AsType[scanner.ErrorList](err); isSyntax {
			return "", nil //nolint:nilerr // syntax errors in carried files are reported by the compiler, not as error-wrap findings
		}
		return "", err
	}
	if len(report.Findings) == 0 {
		return "", nil
	}
	report.Base = repochanged.LineBase{Commit: "HEAD", Reason: "commit change set"}
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
