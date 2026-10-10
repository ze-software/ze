package appliance

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/appliance/kernelbuilder"
)

const kernelBuilderPkg = "internal/appliance/kernelbuilder"

// hashedBuilderSources answers the files the cached-kernel key hashes, from
// the repository root.
func hashedBuilderSources(t *testing.T) map[string]bool {
	t.Helper()
	paths, ok := kernelBuilderSources()
	if !ok {
		t.Fatal("kernelBuilderSources found no build inputs from the repository root")
	}
	hashed := make(map[string]bool, len(paths))
	for _, path := range paths {
		hashed[filepath.ToSlash(path)] = true
	}
	return hashed
}

// TestKernelCacheKeyCoversEveryBuildInput proves the cached-kernel key hashes
// every input that decides a kernel's bytes: every file under
// tools/kernel-builder, every BuildSources file, every Ze package those Go files
// import, and that no BuildSources file reaches a value or function declared in
// a host-side kernelbuilder file, where an edit would change the build and keep
// the key.
//
// VALIDATES: round-2 NOTE 1, the key covers what reaches the build.
// PREVENTS: a stale kernel served after an edit to code the build runs.
func TestKernelCacheKeyCoversEveryBuildInput(t *testing.T) {
	t.Chdir(filepath.Join("..", ".."))
	hashed := hashedBuilderSources(t)

	err := filepath.WalkDir(kernelBuilderDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && !hashed[filepath.ToSlash(path)] {
			t.Errorf("the key leaves out %s, which the build container is made from", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	buildFiles := make(map[string]bool, len(kernelbuilder.BuildSources))
	for _, name := range kernelbuilder.BuildSources {
		buildFiles[name] = true
		if !hashed[kernelBuilderPkg+"/"+name] {
			t.Errorf("the key leaves out %s/%s, a BuildSources file", kernelBuilderPkg, name)
		}
	}

	// Each hashed Go file's Ze imports are hashed whole: one level, checked over
	// the whole hashed set, is the transitive closure.
	fset := token.NewFileSet()
	for path := range hashed {
		if !strings.HasSuffix(path, ".go") {
			continue
		}
		file, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		for _, spec := range file.Imports {
			dir, inModule := strings.CutPrefix(strings.Trim(spec.Path.Value, `"`), "github.com/ze-software/ze/")
			if !inModule || dir == kernelBuilderPkg {
				continue
			}
			for _, goFile := range nonTestGoFiles(t, dir) {
				if !hashed[goFile] {
					t.Errorf("%s imports %s, and the key leaves out %s", path, dir, goFile)
				}
			}
		}
	}

	// Which kernelbuilder file declares each package-level function, variable
	// and constant. Types are left out: a field set alone produces nothing.
	declaredIn := map[string]string{}
	parsed := map[string]*ast.File{}
	for _, path := range nonTestGoFiles(t, kernelBuilderPkg) {
		file, parseErr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		name := filepath.Base(path)
		parsed[name] = file
		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				if decl.Recv != nil {
					continue
				}
				if decl.Name.Name == "init" && !buildFiles[name] {
					t.Errorf("%s declares init, which runs in the build container, outside BuildSources", path)
				}
				declaredIn[decl.Name.Name] = name
			case *ast.GenDecl:
				if decl.Tok == token.TYPE || decl.Tok == token.IMPORT {
					continue
				}
				for _, spec := range decl.Specs {
					if value, isValue := spec.(*ast.ValueSpec); isValue {
						for _, ident := range value.Names {
							declaredIn[ident.Name] = name
						}
					}
				}
			}
		}
	}
	for name := range buildFiles {
		file := parsed[name]
		if file == nil {
			t.Fatalf("BuildSources names %s, which %s does not hold", name, kernelBuilderPkg)
		}
		// A selector's Sel names a field, a method or another package's
		// declaration, never one of this package's.
		selected := map[*ast.Ident]bool{}
		ast.Inspect(file, func(node ast.Node) bool {
			if selector, isSelector := node.(*ast.SelectorExpr); isSelector {
				selected[selector.Sel] = true
			}
			return true
		})
		ast.Inspect(file, func(node ast.Node) bool {
			ident, isIdent := node.(*ast.Ident)
			if !isIdent || selected[ident] {
				return true
			}
			if home, found := declaredIn[ident.Name]; found && !buildFiles[home] {
				t.Errorf("%s/%s uses %s, declared in the host-side %s, which the key does not hash: move it into a BuildSources file", kernelBuilderPkg, name, ident.Name, home)
			}
			return true
		})
	}
}

// TestKernelCacheKeyIgnoresHostSideEdits proves an edit to a host-side
// kernelbuilder file (a probe, a measurement, a refusal's wording) keeps the
// key, and an edit to a BuildSources file changes it. It runs over a copy of
// the package, beside the real tools/kernel-builder and Ze packages.
//
// VALIDATES: round-2 NOTE 1, a probe or message edit stales no cached kernel.
// PREVENTS: a thirty-minute rebuild per arch after a wording change.
func TestKernelCacheKeyIgnoresHostSideEdits(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	mirror := t.TempDir()
	for _, name := range []string{"go.mod", "tools"} {
		if err := os.Symlink(filepath.Join(root, name), filepath.Join(mirror, name)); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(filepath.Join(root, "internal"))
	if err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(mirror, filepath.FromSlash(kernelBuilderPkg))
	if err := os.MkdirAll(pkg, 0o750); err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() == "appliance" {
			continue
		}
		if err := os.Symlink(filepath.Join(root, "internal", entry.Name()), filepath.Join(mirror, "internal", entry.Name())); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range nonTestGoFiles(t, filepath.Join(root, filepath.FromSlash(kernelBuilderPkg))) {
		data, readErr := os.ReadFile(path) //nolint:gosec // test reads the package it mirrors
		if readErr != nil {
			t.Fatal(readErr)
		}
		if err := os.WriteFile(filepath.Join(pkg, filepath.Base(path)), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(mirror)
	key := func() string {
		paths, ok := kernelBuilderSources()
		if !ok {
			t.Fatal("kernelBuilderSources found no build inputs in the mirror")
		}
		hash, ok := cacheFileHashPaths(paths)
		if !ok {
			t.Fatal("cacheFileHashPaths could not read a build input in the mirror")
		}
		return hash
	}
	edit := func(name string) {
		file, openErr := os.OpenFile(filepath.Join(pkg, name), os.O_APPEND|os.O_WRONLY, 0)
		if openErr != nil {
			t.Fatal(openErr)
		}
		if _, err := file.WriteString("\n// an edit\n"); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}
	before := key()
	var hostSide []string
	for _, path := range nonTestGoFiles(t, pkg) {
		if name := filepath.Base(path); !slices.Contains(kernelbuilder.BuildSources, name) {
			hostSide = append(hostSide, name)
			edit(name)
		}
	}
	for _, want := range []string{"space.go", "qemu.go", "driver.go"} {
		if !slices.Contains(hostSide, want) {
			t.Errorf("%s is not host-side, so the probes and refusals it holds would stale every kernel", want)
		}
	}
	if after := key(); after != before {
		t.Errorf("an edit to the host-side files %v changed the key %s to %s", hostSide, before, after)
	}
	for _, name := range kernelbuilder.BuildSources {
		previous := key()
		edit(name)
		if key() == previous {
			t.Errorf("an edit to %s, a BuildSources file, kept the key", name)
		}
	}
}

func nonTestGoFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.FromSlash(dir))
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			files = append(files, filepath.ToSlash(filepath.Join(dir, name)))
		}
	}
	return files
}
