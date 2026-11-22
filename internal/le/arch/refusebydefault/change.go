// Design: docs/contributing/ze-go-style.md -- commit-time refuse-by-default gate
// Overview: refusebydefault.go -- the shapes each finding names
package archrefusebydefault

import (
	"errors"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/le/commit/changecheck"
	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
)

// ze point: principles/directives/a-wrong-value-must-not-look-like-a-right-one
// ze point: go-standards/directives/log-a-dropped-error-at-warn-or-above
//
// judgeChange refuses a commit whose added shipped Go lines introduce an
// unannotated refuse-by-default shape or malformed guard annotation, while
// leaving untouched legacy shapes alone.
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
	report := CheckReport{
		Scope:     "commit change set",
		Base:      repochanged.LineBase{Commit: "HEAD", Reason: "commit change set"},
		Files:     len(paths),
		Findings:  Findings{},
		Annotated: Findings{},
	}
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
		diskPath, diskErr := existingDiskPath(change.Root, rel)
		if diskErr != nil {
			return "", diskErr
		}
		hits, found, err := ScanSource(fset, rel, after, diskPath)
		if err != nil {
			// Syntax errors are caught by the compiler, not as refuse-by-default findings.
			continue
		}
		verdict := judgeHits(rel, hits, found, func(hit Hit) bool {
			for _, anchor := range hit.Anchors {
				if touchesAdded(added, anchor.first, anchor.last) {
					return true
				}
			}
			return false
		})
		report.Findings = append(report.Findings, verdict.refused...)
		report.Annotated = append(report.Annotated, verdict.accepted...)
	}
	if len(report.Findings) == 0 {
		return "", nil
	}
	return report.Text(), nil
}

func existingDiskPath(root, rel string) (string, error) {
	if root == "" {
		return "", nil
	}
	full := filepath.Join(root, filepath.FromSlash(rel))
	if _, err := os.Stat(filepath.Dir(full)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	return full, nil
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
