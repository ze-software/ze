// Design: docs/architecture/config/yang-config-design.md — YANG module registration

package yang

import (
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/callsite"
)

// The local-data route (command.ServeLocal) judges a command's arguments
// against the leaves this model declares, and reads them from here.
func init() {
	command.RegisterArgDefSource(commandArgDefs)
}

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

// RegisterModuleForTest registers one more YANG module for the duration of a
// test and returns the function that restores the registry as it was. A test
// in another package uses it to make DefaultLoader fail, for example with a
// misspelled extension, and to prove its caller surfaces that error.
// MUST be paired with a call to the returned restore, typically through
// t.Cleanup. Not safe for concurrent use: a test that calls it MUST NOT run
// in parallel with another test that loads YANG.
func RegisterModuleForTest(name, content string) (restore func()) {
	saved := modules
	modules = append(slices.Clone(saved), Module{Name: name, Content: content, Registrar: callsite.Package(2)})
	return func() { modules = saved }
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
