// Design: docs/architecture/config/yang-config-design.md — YANG schema handling
//
// Package yang provides YANG schema loading and validation for ze.
package yang

import (
	"embed"
	"fmt"
	"sort"
	"strings"

	"github.com/openconfig/goyang/pkg/yang"
)

// DefaultLoader creates a Loader with all embedded and registered modules
// loaded and resolved. Returns an error only if embedded loading fails
// (corrupted binary). Registered module and resolution errors are logged
// but non-fatal -- the command tree only needs -cmd.yang modules which
// import ze-extensions (embedded), not the full conf/api module set.
func DefaultLoader() (*Loader, error) {
	l := NewLoader()
	if err := l.LoadEmbedded(); err != nil {
		return nil, fmt.Errorf("YANG LoadEmbedded: %w", err)
	}
	_ = l.LoadRegistered() // Best-effort: some modules may not be imported in this context
	_ = l.Resolve()        // Best-effort: unresolved modules are skipped by tree walker
	return l, nil
}

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

// LoadEmbedded loads the embedded YANG library modules (extensions, types).
// These are true bootstrap modules with no domain content.
// Domain modules (hub-conf, bgp-conf, plugin-conf) are loaded via LoadRegistered().
func (l *Loader) LoadEmbedded() error {
	files := []string{
		"modules/ze-extensions.yang",
		"modules/ze-types.yang",
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

// LoadRegistered loads all init()-registered YANG modules into the loader.
// Call after LoadEmbedded() and before Resolve().
func (l *Loader) LoadRegistered() error {
	for _, mod := range modules {
		if err := l.AddModuleFromText(mod.Name, mod.Content); err != nil {
			return err
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

// ConfModuleNames returns sorted names of loaded config modules (suffix "-conf").
func (l *Loader) ConfModuleNames() []string {
	return l.moduleNamesBySuffix("-conf")
}

// APIModuleNames returns sorted names of loaded API modules (suffix "-api").
func (l *Loader) APIModuleNames() []string {
	return l.moduleNamesBySuffix("-api")
}

func (l *Loader) moduleNamesBySuffix(suffix string) []string {
	var names []string
	for name := range l.modules.Modules {
		if strings.HasSuffix(name, suffix) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
