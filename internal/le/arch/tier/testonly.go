// Design: docs/architecture/module-tiers.md -- test-only helper packages
// Overview: tier.go -- the import audit this check reads
// Related: ../testonly/testonly.go -- the declaration of the test-only packages

package archtier

import (
	"cmp"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/arch/testonly"
)

// leTree is the development tooling tree, which is outside production.
const leTree = "internal/le"

// testOnlyViolations answers each shipped file outside internal/le that
// imports a test-only package, sorted by file.
func testOnlyViolations(module string, edges Edges) []corePair {
	var found []corePair
	for imported, importers := range edges {
		helper, ok := testOnlyHelper(module, imported)
		if !ok {
			continue
		}
		for _, importer := range importers {
			if testFile(importer) {
				continue
			}
			if underDir(importer, leTree) {
				continue
			}
			found = append(found, corePair{File: importer, Package: helper})
		}
	}
	slices.SortFunc(found, func(a, b corePair) int {
		return cmp.Or(cmp.Compare(a.File, b.File), cmp.Compare(a.Package, b.Package))
	})
	return found
}

// testOnlyHelper answers the module-relative test-only package imported
// names, when it is one. testonly.Packages is the declaration.
func testOnlyHelper(module, imported string) (string, bool) {
	if !underDir(imported, module) {
		return "", false
	}
	if imported == module {
		return "", false
	}
	return testonly.Owner(imported[len(module)+1:])
}

// testFile reports whether rel is a Go test file.
func testFile(rel string) bool {
	return strings.HasSuffix(rel, "_test.go")
}

// testOnlyImportGate refuses a shipped file outside internal/le importing a
// test-only helper. It has no baseline: no such importer has ever existed.
func testOnlyImportGate(module string, edges Edges) CheckResult {
	result := CheckResult{Name: "test-only-imports"}
	var tb textbuf.Buffer
	found := testOnlyViolations(module, edges)
	if len(found) == 0 {
		result.Page = tb.Str("OK: no shipped file imports a test-only helper package.\n").String()
		return result
	}
	tb.Str("FAIL: shipped file(s) import a test-only helper package:\n")
	for _, pair := range found {
		tb.Str("  ").Str(pair.File).Str(" imports ").Str(pair.Package).Byte('\n')
	}
	tb.Str("  Rule: a test-only helper imports `testing` and starts real containers, so only a _test.go file,")
	tb.Str(" or tooling under internal/le, imports it (docs/architecture/module-tiers.md, \"Test-only helpers\").\n")
	tb.Str("  Fix: move the importing code into a _test.go file.\n")
	result.Diagnosis = tb.String()
	result.Code = 2
	return result
}
