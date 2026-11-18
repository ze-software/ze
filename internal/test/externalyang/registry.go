// Design: docs/architecture/testing/runner-architecture.md -- shared fixture schema publication.

package externalyang

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// ConfigDeclaration is the origin and module a fixture declares at Stage 1.
// Its zero value is not a declaration and registration refuses it.
type ConfigDeclaration struct {
	Plugin string
	Module Module
}

// configDeclarationRegistry has a usable empty zero value. Registration happens
// only during initialization; snapshots are safe for concurrent reads afterward.
type configDeclarationRegistry struct {
	modules map[string]ConfigDeclaration
}

var fixtureConfigDeclarations configDeclarationRegistry

func mustRegisterConfigDeclaration(declaration ConfigDeclaration) {
	if err := fixtureConfigDeclarations.register(declaration); err != nil {
		panic("BUG: register fixture config declaration: " + err.Error())
	}
}

func (r *configDeclarationRegistry) register(declaration ConfigDeclaration) error {
	if declaration.Plugin == "" {
		return errors.New("fixture config declaration has no plugin")
	}
	if declaration.Module.Name == "" {
		return errors.New("fixture config declaration has no module name")
	}
	if declaration.Module.Namespace == "" {
		return errors.New("fixture config declaration has no namespace")
	}
	if declaration.Module.Text == "" {
		return errors.New("fixture config declaration has no module text")
	}
	if declaration.Module.Refused {
		return fmt.Errorf("fixture module %q is declared refused", declaration.Module.Name)
	}
	if _, exists := r.modules[declaration.Module.Name]; exists {
		return fmt.Errorf("fixture config module %q is already declared", declaration.Module.Name)
	}
	if r.modules == nil {
		r.modules = make(map[string]ConfigDeclaration)
	}
	r.modules[declaration.Module.Name] = declaration
	return nil
}

func (r *configDeclarationRegistry) snapshot() []ConfigDeclaration {
	declarations := make([]ConfigDeclaration, 0, len(r.modules))
	for _, declaration := range r.modules {
		declarations = append(declarations, declaration)
	}
	slices.SortFunc(declarations, func(a, b ConfigDeclaration) int {
		return strings.Compare(a.Module.Name, b.Module.Name)
	})
	return declarations
}

// ConfigDeclarations returns a caller-owned, name-sorted snapshot of the
// accepted config modules registered by each fixture's metadata owner.
func ConfigDeclarations() []ConfigDeclaration {
	return fixtureConfigDeclarations.snapshot()
}
