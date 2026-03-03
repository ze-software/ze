// Design: docs/architecture/config/yang-config-design.md — YANG module registry
//
// Module registry provides init()-based YANG module registration.
// Each package that owns a YANG file registers it via RegisterModule in init().
// The Loader loads all registered modules via LoadRegistered().
package yang

// registeredModule holds a YANG module registered via init().
type registeredModule struct {
	name    string
	content string
}

// moduleRegistry holds all init()-registered YANG modules.
var moduleRegistry []registeredModule

// RegisterModule registers a YANG module for loading.
// Called from init() in packages that own YANG files.
// Order of registration does not matter — goyang resolves imports during Resolve().
func RegisterModule(name, content string) {
	moduleRegistry = append(moduleRegistry, registeredModule{name: name, content: content})
}

// LoadRegistered loads all init()-registered YANG modules into the loader.
// Call after LoadEmbedded() and before Resolve().
func (l *Loader) LoadRegistered() error {
	for _, mod := range moduleRegistry {
		if err := l.AddModuleFromText(mod.name, mod.content); err != nil {
			return err
		}
	}
	return nil
}
