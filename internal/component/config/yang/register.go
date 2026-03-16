// Design: docs/architecture/config/yang-config-design.md — YANG module registration

package yang

// Module holds a YANG module registered via init().
type Module struct {
	Name    string
	Content string
}

var modules []Module

// RegisterModule registers a YANG module for loading.
// Called from init() in packages that own YANG files.
// Order of registration does not matter — goyang resolves imports during Resolve().
func RegisterModule(name, content string) {
	modules = append(modules, Module{Name: name, Content: content})
}

// Modules returns all registered YANG modules.
func Modules() []Module {
	return modules
}
