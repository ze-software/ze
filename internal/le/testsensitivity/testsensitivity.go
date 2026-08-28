// Design: docs/architecture/testing/test-health.md -- test-sensitivity ratchet
//
// Package testsensitivity finds tests that cannot do their job, which no count
// of tests can reveal:
//
//  1. assert-nothing: a Test function with no reachable failure call. It
//     executes code, moves coverage, and passes unconditionally. Deleting the
//     body of the function under test would not turn it red.
//
//  2. tag-orphan: a _test.go file whose //go:build constraint requires a
//     project tag (ze_*) that no native test action supplies. The file compiles
//     nowhere, runs nowhere, and reads as coverage from directory listings.
//
// Both are counted and ratcheted: the committed floors in
// test/health/sensitivity-baseline.json may only go DOWN (lower the floor in
// the same change that improves the number).
//
// Populations: the CHECK scans the WORKING TREE (the ratchet must catch an
// inert test before it is committed, not blame the next change); the TRACKED
// report scans git's index (the generated page must be reproducible from a
// clean checkout).

package testsensitivity

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// testRoots are the in-repo first-party test populations.
var testRoots = []string{"internal", "cmd", "pkg", "test"}

// BaselinePath is the committed floor file, relative to the tree.
const BaselinePath = "test/health/sensitivity-baseline.json"

// gitListDeadline bounds the one git call this package makes. Listing an index
// is milliseconds, so a run past this is a wedged process rather than a slow
// one.
const gitListDeadline = 2 * time.Minute

// Population says which set of test files a scan reads.
type Population int

const (
	// WorkingTree is what you are about to commit. This is right for the
	// ratchet: an inert test must be caught by the run that precedes its
	// commit, not by the next one, which would blame an unrelated change.
	WorkingTree Population = iota
	// Tracked is what a clean checkout contains. This is right for the
	// generated page, which is byte-compared by a staleness gate: if untracked
	// work in progress moved the numbers, every developer with a scratch test
	// file would publish a page that CI disagrees with.
	Tracked
)

// Scan walks the in-repo test roots and runs both detectors.
//
// It fails closed: a scan that finds no test files at all is a broken scan, not
// a clean tree, and an empty tag universe means the make files did not parse.
func Scan(root string, population Population) (Result, error) {
	universe, err := tagUniverse(root)
	if err != nil {
		return Result{}, err
	}
	if len(universe) == 0 {
		return Result{}, fmt.Errorf("derived an empty test-tag universe from %s: the make files did not parse", root)
	}

	files, err := collectTestFiles(root, population)
	if err != nil {
		return Result{}, err
	}
	if len(files) == 0 {
		return Result{}, fmt.Errorf("found no _test.go files under %v: refusing to report a clean tree", testRoots)
	}

	result := Result{
		AssertNothing: []Finding{},
		TagOrphan:     []Finding{},
		FilesScanned:  len(files),
		TagUniverse:   sortedKeys(universe),
	}

	// index resolves a helper that lives in another first-party package. It is
	// built once for the whole scan so each helper package is parsed at most
	// once.
	index := newPkgIndex(root)

	// Group by directory so a helper defined in a sibling test file resolves.
	byDir := map[string][]string{}
	for _, path := range files {
		dir := filepath.Dir(path)
		byDir[dir] = append(byDir[dir], path)
	}

	for _, dir := range sortedKeys(byDir) {
		fset := token.NewFileSet()
		parsed := map[string]*ast.File{}
		for _, path := range byDir[dir] {
			file, parseErr := parser.ParseFile(fset, path, nil, parser.ParseComments)
			if parseErr != nil {
				// A file this gate cannot parse must not be silently skipped.
				return Result{}, fmt.Errorf("parse %s: %w", path, parseErr)
			}
			parsed[path] = file

			orphan, tags := TagOrphan(file, universe)
			if !orphan {
				continue
			}
			result.TagOrphan = append(result.TagOrphan, Finding{
				File:   rel(root, path),
				Line:   buildLine(fset, file),
				Reason: ReasonTagOrphan,
				Detail: strings.Join(tags, ", "),
			})
		}

		// byDir[dir] is already sorted (files came from a sorted walk), so the
		// per-package helper index is built in a fixed order.
		funcs := packageFuncs(parsed, byDir[dir])
		for _, path := range byDir[dir] {
			file := parsed[path]
			sc := scope{
				pkgFuncs: funcs[pkgKey(file.Name.Name)],
				aliases:  assertAliases(file),
				imports:  fileImports(file),
				index:    index,
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || !isTestFunc(fn) {
					continue
				}
				result.TestsScanned++
				if hasEscape(fn, file) || canFail(fn.Body, sc.withTesting(testingIdents(fn)), 1) {
					continue
				}
				result.AssertNothing = append(result.AssertNothing, Finding{
					File:   rel(root, path),
					Test:   fn.Name.Name,
					Line:   fset.Position(fn.Pos()).Line,
					Reason: ReasonAssertNothing,
				})
			}
		}
	}

	sortFindings(result.AssertNothing)
	sortFindings(result.TagOrphan)
	// Valid is set by the ratchet, never here: an unconditional `true` was what
	// made the JSON half of the check unable to fail.
	return result, nil
}

// collectTestFiles gathers the test files to judge.
//
// A missing test root is an error in both populations. Skipping it silently
// would let a mis-set root, or an unreadable directory, shrink the count to
// something the ratchet then happily accepts -- and the floor writer would bake
// that shrunken number in permanently.
func collectTestFiles(root string, population Population) ([]string, error) {
	if population == Tracked {
		return trackedTestFiles(root)
	}

	var files []string
	for _, testRoot := range testRoots {
		dir := filepath.Join(root, testRoot)
		if _, err := os.Stat(dir); err != nil {
			return nil, fmt.Errorf("test root %s is missing or unreadable: %w", dir, err)
		}
		err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				switch entry.Name() {
				case "vendor", "testdata", "node_modules", ".git":
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(path, "_test.go") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk %s: %w", dir, err)
		}
	}
	sort.Strings(files)
	return files, nil
}

// trackedTestFiles lists the _test.go files git has in its index, so the result
// is reproducible from any clean checkout of the same commit.
func trackedTestFiles(root string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitListDeadline)
	defer cancel()
	//nolint:gosec // root is the checkout le resolved, and every other argument is this file's own literal
	out, err := exec.CommandContext(ctx, "git", "-C", root, "ls-files", "-z", "--", "*_test.go").Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files in %s: %w", root, err)
	}

	var files []string
	for name := range strings.SplitSeq(string(out), "\x00") {
		if name == "" {
			continue
		}
		parts := strings.Split(name, "/")
		if len(parts) == 0 || !underTestRoot(parts[0]) || excludedDir(parts) {
			continue
		}

		abs := filepath.Join(root, filepath.FromSlash(name))
		// git ls-files reads the INDEX; a test file deleted or moved in the
		// working tree is still listed until that deletion is staged. Parsing
		// it would fail the whole run with a bare "no such file", which any
		// developer mid-refactor would hit before they can commit. Skip the
		// absent entry -- there is no test content to judge, and on the clean
		// checkout this population describes, the deletion is committed and the
		// entry is simply gone.
		//
		// Deliberately narrow: ONLY a not-exist error is tolerated. An
		// unreadable-but-present file still fails the run rather than silently
		// shrinking the count the ratchet accepts.
		if _, statErr := os.Stat(abs); statErr != nil {
			if os.IsNotExist(statErr) {
				// Said out loud, on stderr, because the count a developer sees
				// is smaller than the one git holds and nothing else would say
				// why. It is a notice about the RUN rather than part of any
				// answer, so it never reaches the payload.
				var tb textbuf.Buffer
				fmt.Fprintln(os.Stderr, tb.Str("test-sensitivity: skipping ").Str(name). //nolint:errcheck // CLI output
														Str(": tracked by git but absent from the working tree (an unstaged delete or move)").String())
				continue
			}
			return nil, fmt.Errorf("stat tracked test file %s: %w", name, statErr)
		}
		files = append(files, abs)
	}
	sort.Strings(files)
	return files, nil
}

// underTestRoot reports whether a path's first element is one of the trees that
// hold first-party tests.
func underTestRoot(first string) bool { return slices.Contains(testRoots, first) }

// excludedDir reports whether any element of a path names a tree this gate
// never judges.
func excludedDir(parts []string) bool {
	for _, part := range parts {
		if part == "vendor" || part == "testdata" || part == "node_modules" {
			return true
		}
	}
	return false
}

// parseGo parses one Go file with its comments, which is what both the helper
// index and the scan need.
func parseGo(path string) (*ast.File, error) {
	return parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
}

// rel answers a path relative to the tree, which is the name a reader can open.
func rel(root, path string) string {
	if relative, err := filepath.Rel(root, path); err == nil {
		return filepath.ToSlash(relative)
	}
	return filepath.ToSlash(path)
}

// sortFindings orders findings by file then line, so two runs over one tree
// answer the same document.
func sortFindings(findings []Finding) {
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		return findings[i].Line < findings[j].Line
	})
}

// sortedKeys answers a map's keys in order.
func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}
