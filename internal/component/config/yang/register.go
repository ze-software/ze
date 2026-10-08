// Design: docs/architecture/config/yang-config-design.md — YANG module registration

package yang

import (
	"strings"

	"github.com/ze-software/ze/internal/core/callsite"
)

// Module holds a YANG module registered via init().
type Module struct {
	Name    string
	Content string
	// Registrar is the import path of the package that called
	// RegisterModule. The wire-method prefix of a command node no builtin
	// handler serves is derived from it, the same way a handler's prefix is
	// derived from the package that registered the handler.
	Registrar string
}

var modules []Module

// RegisterModule registers a YANG module for loading.
// Called from init() in packages that own YANG files. Each module is stamped
// with the import path of the calling package (Registrar).
// Order of registration does not matter — goyang resolves imports during Resolve().
func RegisterModule(name, content string) {
	modules = append(modules, Module{Name: name, Content: content, Registrar: callsite.Package(2)})
}

// Modules returns all registered YANG modules.
func Modules() []Module {
	return modules
}

// ModuleRegistrar answers the package that registered the module named
// module (the name its `module` statement carries, registered as
// "<module>.yang"), and false when no package registered one.
func ModuleRegistrar(module string) (string, bool) {
	for _, m := range modules {
		if strings.TrimSuffix(m.Name, ".yang") == module {
			return m.Registrar, true
		}
	}
	return "", false
}
