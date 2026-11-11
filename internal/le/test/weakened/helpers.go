// Design: docs/architecture/testing/test-health.md -- a helper's change is a change to the tests that call it
// Related: testweakened.go -- weakenedTests collects the changed helpers and calls attributeHelpers.
// Related: scope.go -- the Go unit inventory both sides are read through.
package testweakened

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/commit/hunks"
	"github.com/ze-software/ze/internal/le/repo/objects"
)

// testUnitPattern names the units `go test` runs as tests. TestMain is the
// package's harness, not a test, so it counts as a helper. Every other
// function in a `_test.go` file is a helper.
var testUnitPattern = regexp.MustCompile(`^(Test|Fuzz|Benchmark)`)

func isTestUnit(name string) bool {
	if name == "TestMain" {
		return false
	}
	return testUnitPattern.MatchString(name)
}

// testFileChange is what one compared Go test file tells helper attribution:
// the helpers whose text the commit changes, adds or removes, and the tests
// the baseline already holds, the only ones that can carry a verdict.
type testFileChange struct {
	path     string
	helpers  []string
	existing map[string]bool
}

// goTestFileChange compares one Go test file's two sides. The caller has
// already refused a baseline whose unit boundaries it cannot read
// (changedUnitNames).
func goTestFileChange(path, oldText, newText string) testFileChange {
	oldUnits := unitsByName(oldText)
	newUnits := unitsByName(newText)
	change := testFileChange{path: path, existing: make(map[string]bool, len(oldUnits))}
	for _, unit := range oldUnits {
		if isTestUnit(unit.name) {
			change.existing[unit.name] = true
		}
	}
	seen := make(map[string]bool, len(oldUnits)+len(newUnits))
	for _, unit := range slices.Concat(oldUnits, newUnits) {
		if unit.name == "" {
			continue
		}
		if isTestUnit(unit.name) {
			continue
		}
		if seen[unit.name] {
			continue
		}
		seen[unit.name] = true
		if unitText(oldUnits, unit.name) != unitText(newUnits, unit.name) {
			change.helpers = append(change.helpers, unit.name)
		}
	}
	return change
}

// packageChange gathers the compared test files of one package directory.
type packageChange struct {
	helpers map[string]bool
	files   map[string]map[string]bool
}

// changedHelperSet holds one packageChange per package directory.
type changedHelperSet map[string]*packageChange

func (s changedHelperSet) add(change testFileChange) {
	if change.path == "" {
		return
	}
	dir := filepath.Dir(change.path)
	held := s[dir]
	if held == nil {
		held = &packageChange{helpers: make(map[string]bool), files: make(map[string]map[string]bool)}
		s[dir] = held
	}
	held.files[change.path] = change.existing
	for _, name := range change.helpers {
		held.helpers[name] = true
	}
}

// hasHelpers reports whether any directory holds a changed helper.
func (s changedHelperSet) hasHelpers() bool {
	for _, held := range s {
		if len(held.helpers) != 0 {
			return true
		}
	}
	return false
}

// helperPackages is what attributeHelpers needs to read a package's test
// files as the commit leaves them: the content the commit carries for the
// paths it selects, and the anchor's content for every other file.
type helperPackages struct {
	root        string
	anchor      string
	selection   hunks.Selection
	paths       []string
	removed     []string
	renamePairs []RenamePair
}

// unitRefs is one function of a package's test files: whether it is a
// package-level function (no receiver), and every bare identifier its body
// names outside a selector's field or method name.
type unitRefs struct {
	path       string
	name       string
	standalone bool
	refs       map[string]bool
}

func (u unitRefs) references(names map[string]bool) bool {
	for name := range u.refs {
		if names[name] {
			return true
		}
	}
	return false
}

// attributeHelpers adds to changed every existing test of the commit's own
// files that calls a changed helper. The helper's own weakening stays its own
// finding, named by its own approval row, as the edit hook and the audit name
// it: a caller's verdict judged the caller's text, never the helper's.
//
// The call rule, read on the committed side of the package directory's
// `_test.go` files: a function calls a package-level function F when a bare
// identifier F appears in its body (a call `F(...)` or a value `F`, never the
// name after a selector's dot), directly or through other package-level
// helpers. A method helper reached only through a selector has no callers. A
// local variable that shadows F's name is still read as a call: that test
// becomes a changed unit too, so its live row is spent early and a stale one
// refuses. Only a test whose file the commit carries and whose baseline holds
// it is attributed, so a verdict for a file this commit does not touch, or for
// a new test, is never spent or refused here.
func attributeHelpers(source helperPackages, helpers changedHelperSet, changed []Finding) ([]Finding, []string) {
	var text textbuf.Buffer
	tree, err := objects.ResolveTree(context.Background(), source.root, source.anchor)
	if err != nil {
		return changed, []string{text.Str(cannotRunPrefix).Str("resolve ").Str(source.anchor).
			Str(" to read the tests that call a changed helper: ").Err(err).String()}
	}
	for _, dir := range slices.Sorted(maps.Keys(helpers)) {
		held := helpers[dir]
		if len(held.helpers) == 0 {
			continue
		}
		units, problem := source.packageUnits(tree, dir)
		if problem != "" {
			return changed, []string{text.Reset().Str(cannotRunPrefix).Str(problem).String()}
		}
		for _, name := range slices.Sorted(maps.Keys(held.helpers)) {
			for _, caller := range callingTests(units, name) {
				if !held.files[caller.path][caller.name] {
					continue
				}
				changed = addChanged(changed, changedUnit(caller.path, packageOf(dir), caller.name))
			}
		}
	}
	return changed, nil
}

// packageUnits reads every function of dir's `_test.go` files as the commit
// leaves them. A file the commit carries is read from its selected content and
// refuses when it does not parse. Any other file the anchor holds there is
// read from the anchor; one that does not parse there is skipped, so a broken
// file this commit does not carry never refuses it and lends no call.
func (source helperPackages) packageUnits(tree, dir string) ([]unitRefs, string) {
	gone := slices.Clone(source.removed)
	committed := make([]string, 0, len(source.paths))
	for _, pair := range source.renamePairs {
		gone = append(gone, pair.OldPath)
		if isGoTestIn(pair.NewPath, dir) {
			committed = append(committed, pair.NewPath)
		}
	}
	for _, path := range source.paths {
		if !isGoTestIn(path, dir) {
			continue
		}
		if slices.Contains(gone, path) {
			continue
		}
		if !slices.Contains(committed, path) {
			committed = append(committed, path)
		}
	}
	var units []unitRefs
	for _, path := range committed {
		content, problem := worktreeText(source.root, path, source.selection)
		if problem != "" {
			return nil, problem
		}
		read, err := fileRefs(path, content)
		if err != nil {
			return nil, "cannot read the helper calls in " + path + ": " + err.Error() +
				"; repair the file before preparing the commit"
		}
		units = append(units, read...)
	}
	listed, problem := anchorTestFiles(source.root, tree, dir)
	if problem != "" {
		return nil, problem
	}
	for _, path := range listed {
		if slices.Contains(gone, path) {
			continue
		}
		if slices.Contains(committed, path) {
			continue
		}
		content, _, err := objects.ReadPath(context.Background(), source.root, tree, path)
		if err != nil {
			return nil, err.Error()
		}
		read, err := fileRefs(path, string(content))
		if err != nil {
			continue // A file the commit does not carry lends no call; see the doc comment.
		}
		units = append(units, read...)
	}
	return units, ""
}

func isGoTestIn(path, dir string) bool {
	if !strings.HasSuffix(path, "_test.go") {
		return false
	}
	return filepath.Dir(path) == dir
}

// anchorTestFiles lists the `_test.go` files directly in dir at tree.
func anchorTestFiles(root, tree, dir string) ([]string, string) {
	pathspec := dir + "/"
	if dir == "." {
		pathspec = "."
	}
	stdout, stderr, code, started := gitCapture(root,
		"--literal-pathspecs", "ls-tree", "--name-only", "--full-name", "-z", tree, "--", pathspec)
	if !started {
		return nil, "list the test files of " + dir + ": " + stderr
	}
	if code != 0 {
		return nil, "list the test files of " + dir + ": " + strings.TrimSpace(stderr)
	}
	var files []string
	for entry := range strings.SplitSeq(stdout, "\x00") {
		if isGoTestIn(entry, dir) {
			files = append(files, entry)
		}
	}
	return files, ""
}

// fileRefs answers every function of one Go file with the bare identifiers
// its body names. The name after a selector's dot is a field or a method, so
// it is not read.
func fileRefs(path, content string) ([]unitRefs, error) {
	file, err := parser.ParseFile(token.NewFileSet(), path, content, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	units := make([]unitRefs, 0, len(file.Decls))
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if function.Body == nil {
			continue
		}
		refs := make(map[string]bool)
		var visit func(ast.Node) bool
		visit = func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.SelectorExpr:
				ast.Inspect(node.X, visit)
				return false
			case *ast.Ident:
				refs[node.Name] = true
			}
			return true
		}
		ast.Inspect(function.Body, visit)
		units = append(units, unitRefs{
			path: path, name: function.Name.Name, standalone: function.Recv == nil, refs: refs,
		})
	}
	return units, nil
}

// callingTests answers the tests among units that reach helper, directly or
// through other package-level helpers. A helper that is not a package-level
// function has no bare-name callers.
func callingTests(units []unitRefs, helper string) []unitRefs {
	standalone := false
	for _, unit := range units {
		if unit.name == helper && unit.standalone {
			standalone = true
		}
	}
	if !standalone {
		return nil
	}
	reached := map[string]bool{helper: true}
	// Each pass adds at least one helper or ends, so the loop runs at most
	// len(units)+1 passes.
	for grew := true; grew; {
		grew = false
		for _, unit := range units {
			if isTestUnit(unit.name) {
				continue
			}
			if !unit.standalone {
				continue
			}
			if reached[unit.name] {
				continue
			}
			if unit.references(reached) {
				reached[unit.name] = true
				grew = true
			}
		}
	}
	var tests []unitRefs
	for _, unit := range units {
		if !isTestUnit(unit.name) {
			continue
		}
		if unit.references(reached) {
			tests = append(tests, unit)
		}
	}
	return tests
}

func packageOf(dir string) string {
	if dir == "." {
		return ""
	}
	return filepath.Base(dir)
}

// addChanged appends unit to changed unless a unit of the same name is there.
func addChanged(changed []Finding, unit Finding) []Finding {
	for _, held := range changed {
		if held.Unit() == unit.Unit() {
			return changed
		}
	}
	return append(changed, unit)
}
