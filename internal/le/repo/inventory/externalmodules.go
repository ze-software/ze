// Design: docs/architecture/plugin/feature-gates.md -- committed module boundaries precede external effects.
package repoinventory

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"
)

type externalModuleNamespace struct {
	identity string
	manifest string
}

// externalModuleNamespaces reads declarations only, never nested package bodies.
// Vendor descendants retain their existing manifest authority; a copied go.mod
// there must not hide a root-vendored package or redefine its import namespace.
func (s *Snapshot) externalModuleNamespaces() ([]externalModuleNamespace, error) {
	var names []string
	for name := range s.blobs {
		if !strings.HasSuffix(name, "/go.mod") {
			continue
		}
		directory := path.Dir(name)
		if s.externalNestedModule(directory) != directory {
			continue
		}
		names = append(names, name)
	}
	slices.Sort(names)
	result := make([]externalModuleNamespace, 0, len(names))
	for _, name := range names {
		manifest, err := modfile.Parse(name, s.blobs[name], nil)
		if err != nil {
			return nil, fmt.Errorf("external nested module identity %s: %w", name, err)
		}
		if manifest.Module == nil {
			return nil, fmt.Errorf("external nested module identity %s has no module declaration", name)
		}
		if manifest.Module.Mod.Path == "" {
			return nil, fmt.Errorf("external nested module identity %s has an empty module declaration", name)
		}
		result = append(result, externalModuleNamespace{identity: manifest.Module.Mod.Path, manifest: name})
	}
	return result, nil
}

// check refuses unresolved dependency selection before syntax exclusion can
// discard its importer. A dotless declared module is not a stdlib certificate.
func (module externalModuleNamespace) check(identity, importer string) error {
	if identity != module.identity {
		if !strings.HasPrefix(identity, module.identity+"/") {
			return nil
		}
	}
	return fmt.Errorf("external source identity import %s from %s matches nested module %s; retain exact dependency selection before publishing inventory", identity, importer, module.manifest)
}

// externalNestedModule finds a retained module boundary above directory. The
// root module's manifest is not a boundary. Root replacements and workspaces
// MUST be refused before this boundary excludes a potential import directory.
// Vendor descendants overlay modules, so their boundaries stop before the first
// vendor segment. The vendor directory itself is an ordinary package path.
func (s *Snapshot) externalNestedModule(directory string) string {
	current := "."
	boundary := ""
	segments := strings.Split(directory, "/")
	for index, segment := range segments {
		if segment == "vendor" {
			if index+1 < len(segments) {
				break
			}
		}
		current = path.Join(current, segment)
		if current == "." {
			continue
		}
		if _, found := s.blobs[path.Join(current, "go.mod")]; found {
			boundary = current
		}
	}
	// The empty boundary keeps the parent module's directories in the universe.
	return boundary
}

// checkExternalModuleSelection refuses both certain imports and recursive
// potential imports that crossed a retained module boundary. A separate module
// never acquires the parent module's import identity from its directory name.
func (s *Snapshot) checkExternalModuleSelection() error {
	for directory := range s.packages {
		module := s.externalNestedModule(directory)
		if module != "" {
			return fmt.Errorf("external source identity selects %s across nested module %s/go.mod; retain its exact package selection before publishing inventory", directory, module)
		}
	}
	return nil
}
