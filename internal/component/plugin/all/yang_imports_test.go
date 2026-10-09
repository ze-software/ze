package all

import (
	"errors"
	"go/build"
	"path/filepath"
	"strings"
	"testing"

	goyang "github.com/openconfig/goyang/pkg/yang"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// TestYANGImportsFollowGoImports: every YANG `import` or `include` edge that
// crosses from one registering Go package to another is mirrored by a Go import
// path from the importer's package to the imported module's registrar, so no
// binary can link a module without the module it needs.
//
// Method: the registry, populated by this package's composition root, gives
// each module's text and registrar. Each text is parsed with goyang, so the
// edges come from the statements and not from a word in a description. The Go
// side walks the import graph of the importer's package with go/build, over
// the checkout's own packages only, and stops at the first hit.
//
// VALIDATES: AC-25. The importer's package reaches the imported module's
// registrar, for every cross-package edge of the registered set.
// PREVENTS: a binary that links one `*/yang` package without the package
// registering what its module imports, which strict DefaultLoader refuses
// with "no such module" (the cmd/ze test binary did, for ze-route-refresh).
func TestYANGImportsFollowGoImports(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("find the checkout root: %v", err)
	}
	module, err := lepath.Module(root)
	if err != nil {
		t.Fatalf("read the module path: %v", err)
	}
	graph := goImportGraph{root: root, module: module, imports: map[string][]string{}}

	edges := 0
	for _, registered := range configyang.Modules() {
		for _, imported := range yangDependencies(t, registered) {
			registrar, found := configyang.ModuleRegistrar(imported)
			if !found {
				// The embedded bootstrap modules (ze-extensions, ze-types) are
				// loaded by the registry itself and have no registrar.
				continue
			}
			if registrar == registered.Registrar {
				continue
			}
			edges++
			if !graph.reaches(t, registered.Registrar, registrar) {
				t.Errorf("%s (registered by %s) imports %s, but %s does not import %s in Go",
					registered.Name, registered.Registrar, imported, registered.Registrar, registrar)
			}
		}
	}

	// Non-vacuity: the full registered set held 50 cross-package edges on
	// 2026-10-09 and the gate-free one 6, so the floor is one. A count of
	// zero means the registry or the parse no longer sees the edges.
	if edges == 0 {
		t.Fatal("no cross-package YANG import edge found: the registry or the parse no longer sees the edges")
	}
}

// yangDependencies answers the modules registered's module or submodule
// statement imports or includes.
func yangDependencies(t *testing.T, registered configyang.Module) []string {
	t.Helper()

	statements, err := goyang.Parse(registered.Content, registered.Name)
	if err != nil {
		t.Fatalf("parse %s: %v", registered.Name, err)
	}
	var dependencies []string
	for _, top := range statements {
		for _, statement := range top.SubStatements() {
			if statement.Keyword == "import" || statement.Keyword == "include" {
				dependencies = append(dependencies, statement.Argument)
			}
		}
	}
	return dependencies
}

// goImportGraph reads the Go imports of the checkout's own packages, once per
// package. Not safe for concurrent use.
type goImportGraph struct {
	root    string
	module  string
	imports map[string][]string
}

// reaches answers whether package source imports package target, directly or
// through other packages of the checkout. Bounded by the checkout's package
// count: each package is expanded once.
func (g goImportGraph) reaches(t *testing.T, source, target string) bool {
	t.Helper()

	seen := map[string]bool{source: true}
	queue := []string{source}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, imported := range g.importsOf(t, current) {
			if imported == target {
				return true
			}
			if seen[imported] {
				continue
			}
			seen[imported] = true
			queue = append(queue, imported)
		}
	}
	return false
}

// importsOf answers the checkout packages that package path imports in its
// non-test files, under the default build context.
func (g goImportGraph) importsOf(t *testing.T, path string) []string {
	t.Helper()

	if cached, ok := g.imports[path]; ok {
		return cached
	}
	relative := strings.TrimPrefix(path, g.module+"/")
	pkg, err := build.ImportDir(filepath.Join(g.root, filepath.FromSlash(relative)), 0)
	if _, noGo := errors.AsType[*build.NoGoError](err); noGo {
		// Every file carries a build tag the default context does not set.
		g.imports[path] = nil
		return nil
	}
	if err != nil {
		t.Fatalf("read the Go imports of %s: %v", path, err)
	}
	var local []string
	for _, imported := range pkg.Imports {
		if strings.HasPrefix(imported, g.module+"/") {
			local = append(local, imported)
		}
	}
	g.imports[path] = local
	return local
}
