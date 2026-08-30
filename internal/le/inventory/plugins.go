// Design: (none -- build tool)
//
// Detail: inventory.go -- the walk this file borrows; report.go -- the answer.
//
// plugins.go answers what a plugin IS, beyond what registry.Registration
// states: the package directory it registers from, and the YANG files that sit
// beside that package. The public plugin catalog shows both, and neither is a
// field of the registration, so both are DERIVED from the registration rather
// than declared a second time.

package inventory

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// modulePrefix is this module's import path, which every package path a
// registration resolves to starts with.
const modulePrefix = "github.com/ze-software/ze/"

// yangDirectory is the directory name a package keeps its YANG modules in.
const yangDirectory = "yang"

// Plugins answers every registered plugin, with the two derived fields the
// public catalog shows added to what the registration states.
//
// The answer is in registry order, which registry.All sorts by name.
func Plugins(root string) ([]Plugin, error) {
	yangPaths, err := discoverYANGPaths(root)
	if err != nil {
		return nil, err
	}
	return pluginsFrom(root, registry.All(), yangPaths)
}

// pluginsFrom shapes one set of registrations into the catalog's own view.
func pluginsFrom(root string, registrations []*registry.Registration, yangPaths map[string]string) ([]Plugin, error) {
	plugins := make([]Plugin, 0, len(registrations))
	for _, reg := range registrations {
		source, err := pluginPackageDir(reg)
		if err != nil {
			return nil, err
		}
		files, err := pluginYANGFiles(root, source, reg.YANG, yangPaths)
		if err != nil {
			return nil, err
		}
		plugins = append(plugins, Plugin{
			Name:                 reg.Name,
			Description:          reg.Description,
			Families:             reg.Families,
			Capabilities:         reg.CapabilityCodes,
			Dependencies:         reg.Dependencies,
			OptionalDependencies: reg.OptionalDependencies,
			ConfigRoots:          reg.ConfigRoots,
			RFCs:                 reg.RFCs,
			Features:             reg.Features,
			SourceDir:            source,
			YANGFiles:            files,
			HasYANG:              reg.YANG != "",
			HasDecoder:           reg.InProcessNLRIDecoder != nil,
			HasEncoder:           reg.InProcessNLRIEncoder != nil,
		})
	}
	return plugins, nil
}

// pluginPackageDir answers the repository-relative directory a plugin
// registers from.
//
// The registration carries no path, so the answer comes from the ENGINE
// FUNCTION it declares: a compiled function knows the package it was defined
// in, and registry.Register refuses a registration whose RunEngine is nil. A
// path derived this way cannot drift from the code, which a `SourceDir` field
// each plugin typed out for itself would do the first time a package moved.
//
// The retired extractor read the same fact from the file system, by globbing
// every register.go under internal/ and taking each match's directory.
func pluginPackageDir(reg *registry.Registration) (string, error) {
	engine := reflect.ValueOf(reg.RunEngine)
	if !engine.IsValid() || engine.IsNil() {
		return "", fmt.Errorf("plugin %q declares no engine, so its source directory cannot be derived", reg.Name)
	}
	symbol := runtime.FuncForPC(engine.Pointer())
	if symbol == nil {
		return "", fmt.Errorf("plugin %q: its engine resolves to no compiled function", reg.Name)
	}
	pkg := packageOf(symbol.Name())
	if !strings.HasPrefix(pkg, modulePrefix) {
		return "", fmt.Errorf("plugin %q registers from %q, which is outside this module", reg.Name, pkg)
	}
	return strings.TrimPrefix(pkg, modulePrefix), nil
}

// packageOf answers the package path of a fully qualified function name.
//
// A name is "<package path>.<function>", and a package path holds dots of its
// own in its domain, so the split is at the first dot AFTER the last slash. A
// method value and a closure add further dots after that one, and both leave
// the package path in front of it.
func packageOf(qualified string) string {
	slash := strings.LastIndex(qualified, "/")
	dot := strings.Index(qualified[slash+1:], ".")
	if dot < 0 {
		return qualified
	}
	return qualified[:slash+1+dot]
}

// pluginYANGFiles answers every YANG file that sits beside a plugin, as sorted
// repository-relative slash paths.
//
// The directory is found from the module the registration CARRIES, because a
// plugin may register a module that lives outside its own package: the core
// `bgp` plugin registers from internal/component/bgp/plugin and its schema is
// in internal/component/bgp/yang. A registration naming no module falls back to
// the yang directory beside the package, which is where every other plugin
// keeps one.
//
// Every file in that directory is answered, not only the registered module. A
// reader of the catalog wants the schema a plugin ships, and a plugin's api and
// cmd modules are registered by their own packages.
func pluginYANGFiles(root, source, module string, yangPaths map[string]string) ([]string, error) {
	directory := yangModuleDir(module, yangPaths)
	if directory == "" {
		directory = filepath.ToSlash(filepath.Join(source, yangDirectory))
	}
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(directory)))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", directory, err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), yangSuffix) {
			continue
		}
		files = append(files, directory+"/"+entry.Name())
	}
	sort.Strings(files)
	return files, nil
}

// yangModuleDir answers the directory holding the module a registration
// carries, or the empty string when the registration names none.
func yangModuleDir(module string, yangPaths map[string]string) string {
	name := yangModuleName(module)
	if name == "" {
		return ""
	}
	path, found := yangPaths[name+yangSuffix]
	if !found {
		return ""
	}
	return filepath.ToSlash(filepath.Dir(path))
}

// yangModuleName answers the module a YANG document declares.
//
// It reads the first "module" or "submodule" statement, which YANG requires to
// be the document's own opening statement. Comments and blank lines before it
// are skipped, so a license header does not hide the name.
func yangModuleName(document string) string {
	for line := range strings.SplitSeq(document, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") ||
			strings.HasPrefix(trimmed, "*") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 2 || (fields[0] != "module" && fields[0] != "submodule") {
			return ""
		}
		return strings.TrimSuffix(fields[1], "{")
	}
	return ""
}
