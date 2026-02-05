// Package yang provides YANG schema loading and validation for ze.
package yang

import (
	"embed"
	"fmt"

	"github.com/openconfig/goyang/pkg/yang"
)

//go:embed modules
var embeddedModules embed.FS

// Loader loads and resolves YANG modules.
type Loader struct {
	modules *yang.Modules
}

// NewLoader creates a new YANG module loader.
func NewLoader() *Loader {
	return &Loader{
		modules: yang.NewModules(),
	}
}

// LoadEmbedded loads the embedded core YANG modules (extensions, types, plugin).
// Module-specific YANG (hub, bgp) must be loaded separately via AddModuleFromText.
func (l *Loader) LoadEmbedded() error {
	// Core modules only - module-specific YANG lives in their respective packages
	files := []string{
		"modules/ze-extensions.yang", // Must be first - defines extensions used by other modules
		"modules/ze-types.yang",
		"modules/ze-plugin-conf.yang",
	}

	for _, path := range files {
		content, err := embeddedModules.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", path, err)
		}
		if err := l.AddModuleFromText(path, string(content)); err != nil {
			return fmt.Errorf("load %s: %w", path, err)
		}
	}

	return nil
}

// AddModuleFromText adds a YANG module from text content.
func (l *Loader) AddModuleFromText(name, content string) error {
	if err := l.modules.Parse(content, name); err != nil {
		return fmt.Errorf("parse YANG: %w", err)
	}
	return nil
}

// AddModuleFromFile adds a YANG module from a file path.
func (l *Loader) AddModuleFromFile(path string) error {
	if err := l.modules.Read(path); err != nil {
		return fmt.Errorf("read YANG file %s: %w", path, err)
	}
	return nil
}

// Resolve resolves all module dependencies and imports.
func (l *Loader) Resolve() error {
	// Process all modules to resolve imports
	errs := l.modules.Process()
	if len(errs) > 0 {
		return fmt.Errorf("resolve YANG modules: %v", errs)
	}
	return nil
}

// GetModule returns a loaded module by name.
func (l *Loader) GetModule(name string) *yang.Module {
	return l.modules.Modules[name]
}

// GetEntry returns the processed entry tree for a module.
// The entry tree has all imports resolved and mandatory fields properly set.
func (l *Loader) GetEntry(name string) *yang.Entry {
	mod := l.modules.Modules[name]
	if mod == nil {
		return nil
	}
	return yang.ToEntry(mod)
}

// ModuleNames returns the names of all loaded modules.
func (l *Loader) ModuleNames() []string {
	names := make([]string, 0, len(l.modules.Modules))
	for name := range l.modules.Modules {
		names = append(names, name)
	}
	return names
}
