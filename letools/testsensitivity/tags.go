// Design: docs/architecture/testing/test-health.md -- which build tags a test can be run under
//
// tags.go answers one question: is a `//go:build` constraint SATISFIABLE by
// something this repository can actually run?
//
// The tag universe is DERIVED from the make files and the feature manifest,
// never hardcoded: a new gated feature must not silently make its tests
// orphans, and deleting a target must surface the tests it stranded.

package testsensitivity

import (
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// projectTag matches the build tags this repository owns. Non-project tags
// (linux, amd64, integration, cgo, go1.x) are treated as satisfiable, so the
// orphan detector only fires on a tag whose reachability this repository
// actually controls.
var projectTag = regexp.MustCompile(`^ze_[a-z0-9_]+$`)

// goTestTagsRE finds `go test ... -tags 'a b c'` (or -tags a) in the make files.
var goTestTagsRE = regexp.MustCompile(`go test[^\n]*?-tags[ =]'([^']*)'|go test[^\n]*?-tags[ =]([a-zA-Z0-9_,]+)`)

// makeVarRE finds `NAME = value` and `NAME := value` so $(GO_TEST_TAGS)
// expands.
var makeVarRE = regexp.MustCompile(`(?m)^([A-Z_][A-Z0-9_]*)\s*[:?]?=\s*(.*)$`)

// maxFreeTags bounds the brute-force satisfiability search. Real constraints
// carry a handful of tags; anything larger is assumed satisfiable rather than
// reported, because a guard must not manufacture a finding it did not actually
// prove.
const maxFreeTags = 16

// TagUniverse derives the set of project tags that some `go test -tags`
// invocation in the make files supplies, expanding make variables
// (GO_TEST_TAGS, ZE_FEATURES, ...) and the feature-gate manifest.
func TagUniverse(root string) (map[string]bool, error) {
	vars := map[string]string{}
	var sources []string

	mks, err := filepath.Glob(filepath.Join(root, "mk", "*.mk"))
	if err != nil {
		return nil, fmt.Errorf("glob mk/*.mk: %w", err)
	}
	makefiles := make([]string, 0, 1+len(mks))
	makefiles = append(makefiles, filepath.Join(root, "Makefile"))
	makefiles = append(makefiles, mks...)

	for _, path := range makefiles {
		raw, readErr := os.ReadFile(path) //nolint:gosec // fixed in-repo paths
		if readErr != nil {
			if path == filepath.Join(root, "Makefile") {
				return nil, fmt.Errorf("read %s: %w", path, readErr)
			}
			continue
		}
		text := string(raw)
		sources = append(sources, text)
		for _, match := range makeVarRE.FindAllStringSubmatch(text, -1) {
			if _, seen := vars[match[1]]; !seen {
				vars[match[1]] = match[2]
			}
		}
	}

	// ZE_FEATURES is defined as `$(shell awk ... feature-gates.txt)`, which this
	// parser cannot execute, so its value is supplied here from the manifest it
	// reads. Crucially this is bound as a make VARIABLE, not injected straight
	// into the universe: a tag reaches the universe only if some `go test -tags`
	// line actually references it.
	//
	// Seeding the universe from the manifest directly (the earlier version) made
	// the guard unable to fail. Deleting `$(ZE_FEATURES)` from GO_TEST_TAGS
	// would have stranded every feature-gated test, and the gate would still
	// have reported zero orphans -- fail-open on exactly the regression it
	// exists to catch.
	gates, err := os.ReadFile(filepath.Join(root, "feature-gates.txt")) //nolint:gosec // fixed in-repo path
	if err != nil {
		return nil, fmt.Errorf("read feature-gates.txt: %w", err)
	}
	var manifest []string
	for line := range strings.SplitSeq(string(gates), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && projectTag.MatchString(fields[0]) {
			manifest = append(manifest, fields[0])
		}
	}
	sort.Strings(manifest)
	vars["ZE_FEATURES"] = strings.Join(manifest, " ")

	universe := map[string]bool{}
	for _, text := range sources {
		for _, match := range goTestTagsRE.FindAllStringSubmatch(text, -1) {
			spec := match[1]
			if spec == "" {
				spec = match[2]
			}
			for _, tag := range ExpandTags(spec, vars, 0) {
				if projectTag.MatchString(tag) {
					universe[tag] = true
				}
			}
		}
	}
	return universe, nil
}

// ExpandTags splits a -tags spec into tags, resolving $(VAR) references against
// the make variables. It is depth-bounded so a self-referential variable cannot
// loop.
func ExpandTags(spec string, vars map[string]string, depth int) []string {
	if depth > 4 {
		return nil
	}
	var out []string
	for _, field := range strings.FieldsFunc(spec, func(r rune) bool { return r == ' ' || r == ',' || r == '\t' }) {
		if strings.HasPrefix(field, "$(") && strings.HasSuffix(field, ")") {
			name := strings.TrimSuffix(strings.TrimPrefix(field, "$("), ")")
			if value, ok := vars[name]; ok {
				out = append(out, ExpandTags(value, vars, depth+1)...)
			}
			continue
		}
		if strings.ContainsAny(field, "$()'\"") {
			continue
		}
		out = append(out, field)
	}
	return out
}

// TagOrphan reports whether a file's //go:build expression is UNSATISFIABLE
// given what the make files can supply, and the tags that make it so.
//
// This is a satisfiability question, not a single evaluation, and getting that
// wrong is the obvious trap: evaluating once with "every available tag is on"
// wrongly condemns every negated constraint. `//go:build !linux` (the non-Linux
// stubs) and `//go:build ze_core && !ze_web` (the compile-out checks that
// GO_TEST_CORE_TAGS exists to run) are both reachable, and both look dead to a
// single evaluation.
//
// The model: a project tag absent from the universe can only ever be false,
// because nothing passes it. Every other tag is free, since different targets
// pass different tag sets. The file is an orphan only when no assignment of the
// free tags satisfies the expression.
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
