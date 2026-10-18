// Design: docs/architecture/testing/qemu-integration.md -- the populations derived from build tags
// Related: ../qemu/integration_coverage_test.go -- the integration population guard over this scan.
// Related: ../integration/dockerpeers.go -- the docker-peers verb over this scan.

// Package buildtag answers which test files in the tree a build tag selects.
// Each tag-gated population is derived from the tree by this one scanner, so
// adding a tagged package needs no edit to any list.
package buildtag

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// scannedAreas are the checkout roots holding Go packages.
var scannedAreas = []string{"internal", "cmd"}

// TestFile is one _test.go file and the terms of its //go:build constraint.
type TestFile struct {
	// Path is the file, relative to the checkout and slash-separated.
	Path string
	// Dir is the file's package directory, in the same form.
	Dir string
	// Terms are the identifiers the constraint names, negated ones included.
	Terms []string
}

// Names reports whether the file's constraint names tag as a term. The tag
// is matched as a whole word, so `integration_slow` never counts as
// `integration`.
func (f TestFile) Names(tag string) bool { return slices.Contains(f.Terms, tag) }

// TestFiles answers every _test.go file under internal/ and cmd/ that carries
// a //go:build constraint, skipping testdata and node_modules.
//
// The constraint is read from the file's leading comment block: a //go:build
// line must appear before the package clause, so a bounded read of the head is
// the whole of it and the file never needs parsing.
func TestFiles(root string) ([]TestFile, error) {
	var files []TestFile
	for _, area := range scannedAreas {
		top := filepath.Join(root, area)
		if _, err := os.Stat(top); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(top, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "testdata" {
					return filepath.SkipDir
				}
				if d.Name() == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(d.Name(), "_test.go") {
				return nil
			}
			data, readErr := os.ReadFile(path) //nolint:gosec // a path this walk produced under the checkout
			if readErr != nil {
				return readErr
			}
			terms := constraintTerms(string(data))
			if len(terms) == 0 {
				return nil
			}
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			rel = filepath.ToSlash(rel)
			files = append(files, TestFile{Path: rel, Dir: filepath.ToSlash(filepath.Dir(rel)), Terms: terms})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

// Packages answers, sorted, every package directory holding a _test.go file
// whose constraint names tag.
func Packages(root, tag string) ([]string, error) {
	files, err := TestFiles(root)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, file := range files {
		if file.Names(tag) && !slices.Contains(dirs, file.Dir) {
			dirs = append(dirs, file.Dir)
		}
	}
	slices.Sort(dirs)
	return dirs, nil
}

// constraintTerms answers the identifiers of the //go:build lines before the
// package clause.
func constraintTerms(source string) []string {
	head, _, _ := strings.Cut(source, "\npackage ")
	var terms []string
	for line := range strings.SplitSeq(head, "\n") {
		expression, found := strings.CutPrefix(line, "//go:build ")
		if !found {
			continue
		}
		terms = append(terms, strings.FieldsFunc(expression, func(r rune) bool {
			return r == ' ' || r == '(' || r == ')' || r == '&' || r == '|' || r == '!' || r == ','
		})...)
	}
	return terms
}
