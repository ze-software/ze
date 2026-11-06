// Design: docs/architecture/plugin/feature-gates.md -- exact committed package identities.
package repoinventory

import (
	"fmt"
	"strings"

	"golang.org/x/mod/modfile"
)

// externalVendorPackages accepts only the ordinary unreplaced vendor manifest.
// Its output maps full package paths, never module prefixes guessed from a name.
func externalVendorPackages(snapshot *Snapshot, manifest *modfile.File) (map[string]bool, error) {
	required := make(map[string]string)
	for _, requirement := range manifest.Require {
		required[requirement.Mod.Path] = requirement.Mod.Version
	}
	packages := make(map[string]bool)
	modules := make(map[string]bool)
	module := ""
	for _, line := range strings.Split(string(snapshot.blobs["vendor/modules.txt"]), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "## ") {
			if module == "" {
				return nil, fmt.Errorf("external vendor identity metadata has no module owner")
			}
			continue
		}
		if strings.HasPrefix(line, "# ") {
			fields := strings.Fields(line)
			if len(fields) != 3 {
				return nil, fmt.Errorf("external vendor identity has an unsupported module or replacement: %s", line)
			}
			module = fields[1]
			if externalParentModuleOverlap(module) {
				return nil, fmt.Errorf("external parent namespace overlaps vendor module %s; retain one committed module owner before publishing inventory", module)
			}
			if required[module] != fields[2] {
				return nil, fmt.Errorf("external vendor identity for %s disagrees with go.mod", module)
			}
			if modules[module] {
				return nil, fmt.Errorf("external vendor identity repeats module %s", module)
			}
			modules[module] = true
			continue
		}
		if module == "" {
			return nil, fmt.Errorf("external vendor identity package has no module: %s", line)
		}
		owned := line == module || strings.HasPrefix(line, module+"/")
		if !owned {
			return nil, fmt.Errorf("external vendor identity package %s is outside module %s", line, module)
		}
		if len(strings.Fields(line)) != 1 {
			return nil, fmt.Errorf("external vendor identity has malformed package %q", line)
		}
		if packages[line] {
			return nil, fmt.Errorf("external vendor identity repeats package %s", line)
		}
		packages[line] = true
	}
	for directory := range snapshot.directories {
		if snapshot.externalNestedModule(directory) != "" {
			continue
		}
		if strings.Contains(directory, "/vendor/") {
			return nil, fmt.Errorf("external source identity has unresolved nested vendor directory %s", directory)
		}
		if !strings.HasPrefix(directory, "vendor/") {
			continue
		}
		name := strings.TrimPrefix(directory, "vendor/")
		if !packages[name] {
			return nil, fmt.Errorf("external vendor identity missing for retained directory %s", directory)
		}
	}
	return packages, nil
}

func (s *externalSource) acyclic() error {
	complete := make(map[string]bool)
	active := make(map[string]bool)
	var visit func(string, int) error
	visit = func(directory string, depth int) error {
		if active[directory] {
			return fmt.Errorf("external import cycle reaches %s; repair the committed package graph", directory)
		}
		if complete[directory] {
			return nil
		}
		if depth == 128 {
			return fmt.Errorf("external identity graph exceeds 128 import steps")
		}
		active[directory] = true
		for _, identity := range s.imports[directory] {
			next := "vendor/" + identity
			if strings.HasPrefix(identity, modulePrefix) {
				next = strings.TrimPrefix(identity, modulePrefix)
			}
			if _, retained := s.snapshot.packages[next]; !retained {
				continue
			}
			if err := visit(next, depth+1); err != nil {
				return err
			}
		}
		delete(active, directory)
		complete[directory] = true
		return nil
	}
	for directory := range s.snapshot.packages {
		if err := visit(directory, 0); err != nil {
			return err
		}
	}
	return nil
}
