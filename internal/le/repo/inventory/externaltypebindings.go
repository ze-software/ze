// Design: docs/architecture/plugin/feature-gates.md -- completed package bindings are immutable structural source facts.
package repoinventory

import "go/ast"

type externalPackageBinding struct {
	object    *ast.Object
	ambiguous bool
}

// indexPackageBindings captures all scope map keys, including non-type shadows.
// Presence records a complete package even when every scope is empty or nil.
func (index *externalTypeIdentityIndex) indexPackageBindings(pkg *sourcePackage) {
	bindings := make(map[string]externalPackageBinding)
	index.bindings[pkg] = bindings
	for _, file := range pkg.files {
		if file.File.Scope == nil {
			continue
		}
		for name, object := range file.File.Scope.Objects {
			if object == nil {
				continue
			}
			current := bindings[name]
			if current.ambiguous {
				continue
			}
			if current.object != nil {
				if current.object != object {
					current.ambiguous = true
					bindings[name] = current
				}
				continue
			}
			bindings[name] = externalPackageBinding{object: object}
		}
	}
}

// packageBinding never memoizes a query or borrows another package's name table.
// A package outside this captured index retains the original pure fallback.
func (index *externalTypeIdentityIndex) packageBinding(pkg *sourcePackage, name string) (*ast.Object, string) {
	if pkg == nil {
		return nil, "retained type package is unavailable"
	}
	bindings, selected := index.bindings[pkg]
	if !selected {
		return externalPackageTypeBinding(pkg, name)
	}
	binding := bindings[name]
	if binding.ambiguous {
		return nil, "retained type package binding is ambiguous: " + name
	}
	return binding.object, ""
}
