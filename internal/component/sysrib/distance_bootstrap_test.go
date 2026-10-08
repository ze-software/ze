// The bootstrap-distance check. internal/core/rib/distance carries one Go copy
// of every protocol's default administrative distance, for the window before
// sysrib publishes the declaration. That copy has to hold the same numbers as
// the YANG leaves it stands in for, and nothing else in the tree would notice
// if it stopped: sysrib publishes the schema defaults at process start, so the
// copy is read only in that window.
//
// The Loc-RIB owns administrative distance. It is the one package that reads
// the seam to rank; sysrib publishes on it. A second reader would be a producer
// stamping a protocol-wide distance on its own routes, which is the shape the
// RIB-owned design replaced, so the check refuses one.
//
// VALIDATES: the bootstrap table equals the YANG declaration, key for key and
// value for value; and only the Loc-RIB asks the seam for a distance.
// PREVENTS: a silent disagreement between the bootstrap and the schema, and a
// protocol going back to stamping its own distance beside the RIB's.

package sysrib

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	ribdistance "github.com/ze-software/ze/internal/core/rib/distance"
)

const (
	// seamImportPath is the package that carries the declaration.
	seamImportPath = "github.com/ze-software/ze/internal/core/rib/distance"

	// seamReaderDir is the one package allowed to ask the seam for a distance.
	seamReaderDir = "internal/core/rib/locrib"
)

// TestBootstrapDistancesMatchTheDeclaration pins the bootstrap table to the
// product's own resolution of the YANG defaults.
func TestBootstrapDistancesMatchTheDeclaration(t *testing.T) {
	declared, err := parseAdminDistanceConfig("{}")
	if err != nil {
		t.Fatalf("resolving the declared distances: %v", err)
	}
	if len(declared) == 0 {
		t.Fatal("the schema declared no distance at all, so there was nothing to compare")
	}
	bootstrap := ribdistance.Bootstrap()

	for _, protocol := range slices.Sorted(maps.Keys(declared)) {
		d, ok := bootstrap[protocol]
		if !ok {
			t.Errorf("rib { distance { %s } } has no bootstrap value, so it ranks undeclared until sysrib publishes", protocol)
			continue
		}
		if int(d) != declared[protocol] {
			t.Errorf("bootstrap %s is %d but rib { distance { %s } } defaults to %d", protocol, d, protocol, declared[protocol])
		}
	}
	for _, protocol := range slices.Sorted(maps.Keys(bootstrap)) {
		if _, ok := declared[protocol]; !ok {
			t.Errorf("bootstrap names %q, which the schema does not declare", protocol)
		}
	}
}

// TestOnlyTheLocRIBReadsTheDistanceSeam walks every non-test Go file and
// refuses a call to the seam's Of or Resolve outside the Loc-RIB.
func TestOnlyTheLocRIBReadsTheDistanceSeam(t *testing.T) {
	root := repoRootForBootstraps(t)
	readers := 0
	for _, top := range []string{"internal", "pkg", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if entry.Name() == "testdata" || entry.Name() == "vendor" {
					return fs.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			if !readsSeam(t, path) {
				return nil
			}
			rel, relErr := filepath.Rel(root, filepath.Dir(path))
			if relErr != nil {
				return relErr
			}
			if filepath.ToSlash(rel) != seamReaderDir {
				t.Errorf("%s asks the distance seam for a distance; only %s ranks on it", path, seamReaderDir)
				return nil
			}
			readers++
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", top, err)
		}
	}
	if readers == 0 {
		t.Fatalf("no file in %s reads the seam: the walk found nothing, so it checked nothing", seamReaderDir)
	}
}

// readsSeam reports whether the file at path calls the seam's Of or Resolve.
func readsSeam(t *testing.T, path string) bool {
	t.Helper()
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: unreadable: %v", path, err)
	}
	if !strings.Contains(string(source), seamImportPath) {
		return false
	}
	file, err := parser.ParseFile(token.NewFileSet(), path, source, 0)
	if err != nil {
		t.Fatalf("%s: unparseable: %v", path, err)
	}
	alias := ""
	for _, spec := range file.Imports {
		importPath, unquoteErr := strconv.Unquote(spec.Path.Value)
		if unquoteErr != nil || importPath != seamImportPath {
			continue
		}
		alias = "distance"
		if spec.Name != nil {
			alias = spec.Name.Name
		}
	}
	found := false
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := selector.X.(*ast.Ident)
		if !ok || pkg.Name != alias {
			return true
		}
		if selector.Sel.Name == "Of" || selector.Sel.Name == "Resolve" {
			found = true
		}
		return true
	})
	return found
}

// repoRootForBootstraps walks up from the working directory to the tree root.
func repoRootForBootstraps(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod in any parent of the working directory, so the tree root is unknown")
		}
		dir = parent
	}
}
