package kernelbuilder

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const modulePrefix = "github.com/ze-software/ze/"

// VALIDATES: the builder image copies in every first-party package the binary
// it compiles depends on, transitively.
// PREVENTS: an import added to this package failing `go build` INSIDE the
// image, twenty minutes and a Docker daemon away from the author. That is what
// internal/core/diskspace did on 2026-09-03: the guard compiled and its unit
// tests passed on the host, and the kernel build died at Step 6/10 with "no
// required module provides package .../internal/core/diskspace".
//
// The image copies whole packages because Go compiles a package as a unit. Only
// worker.go runs in the container; driver.go and space.go are the host side and
// come along for the compile, so a HOST-side import has to be present in the
// image too.
func TestBuilderImageCopiesEveryFirstPartyImport(t *testing.T) {
	root := repoRoot(t)
	copied := builderStageCopies(t, root)

	// Seed with the two entry points the Dockerfile builds from.
	pending := []string{"tools/kernel-builder", "internal/appliance/kernelbuilder"}
	seen := map[string]bool{}
	for len(pending) > 0 {
		dir := pending[0]
		pending = pending[1:]
		if seen[dir] {
			continue
		}
		seen[dir] = true

		for _, missing := range uncopiedGoFiles(t, root, copied, dir) {
			t.Errorf("the builder image compiles %s but the Dockerfile never COPYs it;\n"+
				"add: COPY %s ./%s", missing, dir, dir)
		}
		pending = append(pending, firstPartyImports(t, filepath.Join(root, dir))...)
	}
}

// repoRoot walks up from the package directory to the module root.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the package directory")
		}
		dir = parent
	}
}

// builderStageCopies returns the repository-relative sources the Dockerfile's
// builder stage copies. It stops at the second FROM, because a path copied into
// the RUNTIME stage is not available to the compile.
func builderStageCopies(t *testing.T, root string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "tools", "kernel-builder", "Dockerfile"))
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}
	var copies []string
	stages := 0
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) == 0 {
			continue
		}
		switch strings.ToUpper(fields[0]) {
		case "FROM":
			stages++
		case "COPY":
			if stages == 1 && len(fields) >= 3 {
				copies = append(copies, fields[1:len(fields)-1]...)
			}
		}
	}
	if len(copies) == 0 {
		t.Fatal("the Dockerfile's builder stage copies nothing; the parser or the file changed shape")
	}
	return copies
}

// uncopiedGoFiles returns the non-test Go files in dir that the image does not
// receive. The granularity is the FILE, not the directory, because the Dockerfile
// copies tools/kernel-builder/main.go on its own rather than its directory. A
// directory-level check would call that package uncopied and be wrong, and a
// second file added beside main.go would break the image build unnoticed.
func uncopiedGoFiles(t *testing.T, root string, copies []string, dir string) []string {
	t.Helper()
	var missing []string
	for _, name := range goFiles(t, filepath.Join(root, dir)) {
		if !copied(copies, filepath.Join(dir, name)) {
			missing = append(missing, filepath.Join(dir, name))
		}
	}
	return missing
}

// copied reports whether path, or a directory holding it, is one of the copied
// sources.
func copied(copies []string, path string) bool {
	clean := filepath.Clean(path)
	for _, source := range copies {
		source = filepath.Clean(strings.TrimPrefix(source, "./"))
		if clean == source || strings.HasPrefix(clean, source+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// goFiles lists the non-test Go file names in dir.
func goFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		names = append(names, name)
	}
	return names
}

// firstPartyImports returns the repository-relative directories the Go files in
// dir import. Test files are excluded: the image compiles the package, not its
// tests.
func firstPartyImports(t *testing.T, dir string) []string {
	t.Helper()
	entries := goFiles(t, dir)
	var deps []string
	for _, name := range entries {
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, name), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, spec := range file.Imports {
			path := strings.Trim(spec.Path.Value, `"`)
			if rel, ok := strings.CutPrefix(path, modulePrefix); ok {
				deps = append(deps, rel)
			}
		}
	}
	return deps
}
