// Design: docs/architecture/testing/qemu-integration.md -- the populations derived from build tags
// Related: ../qemu/integration_coverage_test.go -- the integration population guard over this scan.
// Related: ../integration/dockerpeers.go -- the docker-peers verb over this scan.

// Package buildtag answers which test files in the tree a build tag selects.
// Each tag-gated population is derived from the tree by this one scanner, so
// adding a tagged package needs no edit to any list.
package buildtag

import (
	"errors"
	"go/build/constraint"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// scannedAreas are the checkout roots holding Go packages.
var scannedAreas = []string{"internal", "cmd"}

// identifiersMax bounds the tags one constraint may name. Needs decides by
// trying every assignment of the other tags, 2^(n-1) evaluations, so the bound
// keeps one hand-written constraint under a few thousand.
const identifiersMax = 12

// TestFile is one _test.go file and its //go:build constraint.
type TestFile struct {
	// Path is the file, relative to the checkout and slash-separated.
	Path string
	// Dir is the file's package directory, in the same form.
	Dir string
	// Constraint is the file's parsed //go:build expression, never nil.
	Constraint constraint.Expr
}

// Needs reports whether the file builds only with tag set: some assignment of
// the other tags it names selects it with tag set, and none selects it without.
// So `docker && linux` needs docker, while `!docker`, `integration && !docker`
// and `docker || integration` do not. The tag is matched as a whole word, so
// `integration_slow` never counts as `integration`.
func (f TestFile) Needs(tag string) bool {
	others := otherIdentifiers(f.Constraint, tag)
	if !satisfiable(f.Constraint, tag, true, others) {
		return false
	}
	return !satisfiable(f.Constraint, tag, false, others)
}

// Names reports whether the file's constraint mentions tag anywhere, under any
// operator, negation included. Needs answers which population selects a file;
// Names answers which runners the file is tied to, so `!docker && integration`
// and `docker || integration` both name docker though neither needs it. The tag
// is matched as a whole word, as in Needs.
func (f TestFile) Names(tag string) bool {
	named := false
	collectIdentifiers(f.Constraint, func(name string) {
		if name == tag {
			named = true
		}
	})
	return named
}

// TestFiles answers every _test.go file under internal/ and cmd/ that carries
// a //go:build constraint, skipping testdata and node_modules. A constraint
// that does not parse, or names more than identifiersMax tags, is an error
// naming the file: the go tool refuses the first, and Needs cannot judge the
// second within its bound.
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
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			rel = filepath.ToSlash(rel)
			line, found := buildLine(string(data))
			if !found {
				return nil
			}
			expression, parseErr := constraint.Parse(line)
			if parseErr != nil {
				return errors.New(rel + ": " + parseErr.Error() +
					". Fix: correct the //go:build line, which the go tool refuses too")
			}
			if len(otherIdentifiers(expression, "")) > identifiersMax {
				return errors.New(rel + ": the //go:build line names more than 12 tags, which the" +
					" build-tag scan cannot judge. Fix: split the constraint across files")
			}
			files = append(files, TestFile{Path: rel, Dir: filepath.ToSlash(filepath.Dir(rel)), Constraint: expression})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

// Packages answers, sorted, every package directory holding a _test.go file
// that needs tag (TestFile.Needs).
func Packages(root, tag string) ([]string, error) {
	files, err := TestFiles(root)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, file := range files {
		if file.Needs(tag) && !slices.Contains(dirs, file.Dir) {
			dirs = append(dirs, file.Dir)
		}
	}
	slices.Sort(dirs)
	return dirs, nil
}

// buildLine answers the first //go:build line before the package clause, and
// whether there is one. The go tool reads only the first.
func buildLine(source string) (string, bool) {
	head, _, _ := strings.Cut(source, "\npackage ")
	for line := range strings.SplitSeq(head, "\n") {
		if constraint.IsGoBuild(line) {
			return line, true
		}
	}
	return "", false
}

// otherIdentifiers answers the distinct tags expression names, except tag.
func otherIdentifiers(expression constraint.Expr, tag string) []string {
	var names []string
	collectIdentifiers(expression, func(name string) {
		if name != tag && !slices.Contains(names, name) {
			names = append(names, name)
		}
	})
	return names
}

// collectIdentifiers calls visit for each tag in expression. The recursion is
// bounded by the nesting of one //go:build line in a repository file, which
// constraint.Parse already limits.
func collectIdentifiers(expression constraint.Expr, visit func(string)) {
	switch node := expression.(type) {
	case *constraint.TagExpr:
		visit(node.Tag)
	case *constraint.NotExpr:
		collectIdentifiers(node.X, visit)
	case *constraint.AndExpr:
		collectIdentifiers(node.X, visit)
		collectIdentifiers(node.Y, visit)
	case *constraint.OrExpr:
		collectIdentifiers(node.X, visit)
		collectIdentifiers(node.Y, visit)
	}
}

// satisfiable reports whether some assignment of others makes expression hold
// with tag fixed to tagSet. TestFiles refuses a constraint naming more than
// identifiersMax tags, so the 2^len(others) walk is bounded.
func satisfiable(expression constraint.Expr, tag string, tagSet bool, others []string) bool {
	for assignment := range 1 << len(others) {
		holds := expression.Eval(func(name string) bool {
			if name == tag {
				return tagSet
			}
			return assignment&(1<<slices.Index(others, name)) != 0
		})
		if holds {
			return true
		}
	}
	return false
}
