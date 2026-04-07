// Design: (none — build tool)
//
// plugin_imports generates internal/plugin/all/all.go from register.go discovery.
//
// It scans plugin directories for register.go files that import plugin/registry,
// and internal/**/schema/register.go for YANG schema packages, then generates
// the blank-import file that triggers init() registration.
//
// Usage: go run scripts/codegen/plugin_imports.go
// Called by: go generate ./internal/plugin/all/...
//
//go:build ignore

package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	root, err := findModuleRoot()
	if err != nil {
		fatal(err)
	}

	module, err := readModulePath(filepath.Join(root, "go.mod"))
	if err != nil {
		fatal(err)
	}

	plugins, err := discoverPlugins(root, module)
	if err != nil {
		fatal(err)
	}

	schemas, err := discoverSchemaPackages(filepath.Join(root, "internal"), module)
	if err != nil {
		fatal(err)
	}

	output := filepath.Join(root, "internal", "component", "plugin", "all", "all.go")
	if err := generateAllGo(output, plugins, schemas); err != nil {
		fatal(err)
	}

	fmt.Printf("Generated %s with %d plugins, %d schemas\n", output, len(plugins), len(schemas))
}

func fatal(err error) {
	fmt.Println("plugin_imports:", err)
	os.Exit(1)
}

// findModuleRoot walks up from the current directory to find go.mod.
func findModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

// readModulePath reads the module path from go.mod.
func readModulePath(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module")), nil
		}
	}
	return "", fmt.Errorf("module directive not found in %s", path)
}

// pluginDirs lists directories (relative to repo root) that contain plugin register.go files.
var pluginDirs = []string{
	"internal/component/bgp/plugin",
	"internal/component/bgp/plugins",
	"internal/component/bgp/reactor/filter",
	"internal/component/iface",
	"internal/plugins",
}

// discoverPlugins finds plugin packages by looking for register.go files
// across all known plugin directories. Any register.go that is NOT in a
// schema/ subdirectory is treated as a plugin registration: this catches
// plugins registering via plugin/registry as well as those registering via
// component-local mechanisms (e.g. iface.RegisterBackend in ifacenetlink).
func discoverPlugins(root, module string) ([]string, error) {
	var plugins []string

	for _, rel := range pluginDirs {
		dir := filepath.Join(root, rel)
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || d.Name() != "register.go" {
				return nil
			}
			// Skip schema/register.go (handled by discoverSchemaPackages).
			if filepath.Base(filepath.Dir(path)) == "schema" {
				return nil
			}
			// Convert to full import path relative to module root.
			pkgRel, err := filepath.Rel(root, filepath.Dir(path))
			if err != nil {
				return err
			}
			plugins = append(plugins, module+"/"+pkgRel)
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}

	sort.Strings(plugins)
	return plugins, nil
}

// discoverSchemaPackages finds schema packages that register YANG modules.
// Scans for schema/register.go files that import the yang package, anywhere
// under internal/. Schema packages under internal/plugins/ are included
// because their plugin parent does not transitively import them.
func discoverSchemaPackages(internalDir, module string) ([]string, error) {
	var imports []string

	err := filepath.Walk(internalDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		// Only look at schema/register.go files
		if filepath.Base(path) != "register.go" || filepath.Base(filepath.Dir(path)) != "schema" {
			return nil
		}
		// Verify it imports the yang package (for RegisterModule)
		if !fileImports(path, "config/yang") {
			return nil
		}
		// Convert directory to import path
		schemaDir, _ := filepath.Rel(filepath.Dir(internalDir), filepath.Dir(path))
		imports = append(imports, module+"/"+schemaDir)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(imports)
	return imports, nil
}

// fileImports checks whether a Go source file imports a package matching the substring.
func fileImports(path, substr string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), substr) {
			return true
		}
	}
	return false
}

// generateAllGo writes the all.go file with blank imports for plugins and schemas.
func generateAllGo(path string, plugins, schemas []string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)

	fmt.Fprintln(w, "// Code generated by scripts/codegen/plugin_imports.go; DO NOT EDIT.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "// Package all imports all internal plugins and schema packages,")
	fmt.Fprintln(w, "// triggering their init() registration.")
	fmt.Fprintln(w, "//")
	fmt.Fprintln(w, "// To add a plugin, create internal/component/bgp/plugins/<name>/register.go with an init()")
	fmt.Fprintln(w, "// that calls registry.Register(). Then run: make generate")
	fmt.Fprintln(w, "package all")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "import (")

	// Schema packages first (infrastructure)
	if len(schemas) > 0 {
		fmt.Fprintln(w, "\t// Infrastructure schema packages — YANG module registration.")
		for _, imp := range schemas {
			fmt.Fprintf(w, "\t_ \"%s\"\n", imp)
		}
		fmt.Fprintln(w)
		fmt.Fprintln(w, "\t// Plugin packages — plugin + schema registration.")
	}

	for _, imp := range plugins {
		fmt.Fprintf(w, "\t_ \"%s\"\n", imp)
	}
	fmt.Fprintln(w, ")")
	fmt.Fprintln(w)

	return w.Flush()
}
