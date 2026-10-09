// Design: docs/architecture/command-ownership.md -- YANG as data, generated glue wires it in
// Related: yangglue.go -- the generator that renders these imports into register.go

package yangglue

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	goyang "github.com/openconfig/goyang/pkg/yang"
)

// errUnheldDependency marks a module that imports or includes a module no
// schema package and no embedded bootstrap module holds.
var errUnheldDependency = errors.New("YANG module depends on a module no yang directory holds")

// schemaModule is one parsed .yang file: the module or submodule it declares,
// and the modules it imports or includes.
type schemaModule struct {
	name         string
	dependencies []string
}

// dependencyImports answers, for each schema package, the import paths of the
// other schema packages that register a module its modules import or include,
// sorted. A YANG import is the single declaration of the dependency, and the Go
// import derived from it makes every binary that links a module link what the
// module needs: the strict loader refuses an import it cannot resolve.
//
// A dependency held by the registry's own embedded modules (registryModules)
// needs no import, because every register.go already imports the registry.
func dependencyImports(root, module string, dirs []string) (map[string][]string, error) {
	// A tree with no embedded bootstrap modules holds none to skip; a
	// dependency on one then fails below as unheld.
	registryModules, err := parseDir(filepath.Join(root, "internal", registryPackage, "modules"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	held := map[string]string{}
	for _, parsed := range registryModules {
		held[parsed.name] = ""
	}

	parsedBy := map[string][]schemaModule{}
	for _, dir := range dirs {
		parsed, parseErr := parseDir(dir)
		if parseErr != nil {
			return nil, parseErr
		}
		parsedBy[dir] = parsed
		for _, schema := range parsed {
			held[schema.name] = dir
		}
	}

	imports := map[string][]string{}
	for _, dir := range dirs {
		for _, schema := range parsedBy[dir] {
			for _, dependency := range schema.dependencies {
				holder, found := held[dependency]
				if !found {
					return nil, fmt.Errorf("%w: %s imports %s", errUnheldDependency, schema.name, dependency)
				}
				if holder == "" {
					continue
				}
				if holder == dir {
					continue
				}
				importPath := module + "/" + relativeTo(root, holder)
				if !slices.Contains(imports[dir], importPath) {
					imports[dir] = append(imports[dir], importPath)
				}
			}
		}
		slices.Sort(imports[dir])
	}

	return imports, nil
}

// parseDir parses every .yang file of one directory with goyang, so a
// dependency is read from an import or include statement and never from a word
// in a description.
func parseDir(dir string) ([]schemaModule, error) {
	files, err := modulesIn(dir)
	if err != nil {
		return nil, err
	}

	parsed := make([]schemaModule, 0, len(files))
	for _, file := range files {
		path := filepath.Join(dir, file)
		content, readErr := os.ReadFile(path) //nolint:gosec // a build tool reads the checkout it was pointed at
		if readErr != nil {
			return nil, readErr
		}
		statements, parseErr := goyang.Parse(string(content), path)
		if parseErr != nil {
			return nil, parseErr
		}
		for _, top := range statements {
			schema := schemaModule{name: top.Argument}
			for _, statement := range top.SubStatements() {
				if statement.Keyword == "import" || statement.Keyword == "include" {
					schema.dependencies = append(schema.dependencies, statement.Argument)
				}
			}
			parsed = append(parsed, schema)
		}
	}

	return parsed, nil
}
