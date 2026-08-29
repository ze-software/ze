// Design: docs/architecture/testing/test-health.md -- reachable test build tags
//
// The tag universe is derived from the same feature manifest the native Go
// toolchain uses. A file is an orphan when no native test tag assignment can
// satisfy its build constraint.

package testsensitivity

import (
	"go/ast"
	"go/build/constraint"
	"go/token"
	"maps"
	"regexp"
	"sort"

	"github.com/ze-software/ze/internal/le/featuretags"
	"github.com/ze-software/ze/internal/le/verify/lint"
)

// projectTag matches the build tags this repository owns. Non-project tags
// (linux, amd64, integration, cgo, go1.x) are treated as satisfiable, so the
// orphan detector only fires on a tag whose reachability this repository
// actually controls.
var projectTag = regexp.MustCompile(`^ze_[a-z0-9_]+$`)

// maxFreeTags bounds the brute-force satisfiability search. Real constraints
// carry a handful of tags; anything larger is assumed satisfiable rather than
// reported, because a guard must not manufacture a finding it did not actually
// prove.
const maxFreeTags = 16

// tagUniverse returns every project tag the native action population supplies.
func tagUniverse(root string) (map[string]bool, error) {
	features, err := featuretags.DaemonTags(root)
	if err != nil {
		return nil, err
	}
	universe := make(map[string]bool)
	for _, tag := range verifylint.ReachableProjectTags(features) {
		if projectTag.MatchString(tag) {
			universe[tag] = true
		}
	}
	return universe, nil
}

// TagOrphan reports whether a file's //go:build expression is unsatisfiable
// with the tags native tests can supply.
//
// This is a satisfiability question rather than one fixed evaluation. A project
// tag absent from the universe is always false; every available tag is free
// because different native actions can select different tag sets. The file is
// an orphan only when no assignment of the free tags satisfies the expression.
func TagOrphan(file *ast.File, universe map[string]bool) (bool, []string) {
	for _, group := range file.Comments {
		for _, comment := range group.List {
			// Only a comment BEFORE the package clause can be a build
			// constraint. Without this bound, a comment merely quoting a build
			// line -- which checker docs and specs in this repository do
			// routinely -- was read as the file's own constraint, producing a
			// finding against a file that in fact builds everywhere. A guard
			// must not manufacture a finding it did not prove.
			if comment.Pos() >= file.Package || !constraint.IsGoBuild(comment.Text) {
				continue
			}
			expr, err := constraint.Parse(comment.Text)
			if err != nil {
				// An unparseable constraint is not evidence of an orphan; the
				// Go toolchain would have rejected the file long before here.
				return false, nil
			}
			free, unreachable := classifyTags(expr, universe)
			if satisfiable(expr, free, unreachable) {
				return false, nil
			}
			if len(unreachable) == 0 {
				// Self-contradictory over otherwise-reachable tags, e.g.
				// `ze_core && !ze_core`. Report the constraint itself rather
				// than an empty "requires" list.
				return true, []string{comment.Text}
			}
			return true, unreachable
		}
	}
	return false, nil
}

// classifyTags splits the expression's tags into those that may take either
// value and those pinned to false because no target supplies them.
func classifyTags(expr constraint.Expr, universe map[string]bool) (free, unreachable []string) {
	seen := map[string]bool{}
	// Eval visits every tag; the value it answers is irrelevant here.
	_ = expr.Eval(func(tag string) bool {
		if seen[tag] {
			return false
		}
		seen[tag] = true
		if projectTag.MatchString(tag) && !universe[tag] {
			unreachable = append(unreachable, tag)
			return false
		}
		free = append(free, tag)
		return false
	})
	sort.Strings(free)
	sort.Strings(unreachable)
	return free, unreachable
}

// satisfiable reports whether some assignment of the free tags makes the
// expression true, with the unreachable tags pinned to false.
func satisfiable(expr constraint.Expr, free, unreachable []string) bool {
	if len(free) > maxFreeTags {
		return true
	}
	pinned := map[string]bool{}
	for _, tag := range unreachable {
		pinned[tag] = false
	}
	for mask := range 1 << len(free) {
		assign := make(map[string]bool, len(pinned)+len(free))
		maps.Copy(assign, pinned)
		for i, tag := range free {
			assign[tag] = mask&(1<<i) != 0
		}
		if expr.Eval(func(tag string) bool { return assign[tag] }) {
			return true
		}
	}
	return false
}

// buildLine answers the line the file's build constraint sits on, and 1 when it
// has none.
func buildLine(fset *token.FileSet, file *ast.File) int {
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if constraint.IsGoBuild(comment.Text) {
				return fset.Position(comment.Pos()).Line
			}
		}
	}
	return 1
}
