// Design: docs/architecture/config/yang-config-design.md -- the model is the one
// declaration of an enumeration's value set.
// Related: validator.go -- the entry walk this file reads an enumeration through.

package yang

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	gyang "github.com/openconfig/goyang/pkg/yang"
)

// modelLoader answers the default loader once for the life of the process.
//
// The model is embedded in the binary and registered by init(), so it cannot
// change while the process runs, and parsing every module costs enough that a
// guard calling EnumValues on each config apply would pay it again each time.
var modelLoader = sync.OnceValues(DefaultLoader)

// EnumValues answers every value of the enumeration the model declares at path,
// sorted, or an error naming why the model could not answer.
//
// path names the schema node: the top-level section, then each container, then
// the leaf, as ValidateTree takes it, for example "system/conntrack/module". It
// also reaches a command module, whose first part is the RPC root rather than a
// config section. Every loaded module is tried, in name order, because several
// modules contribute to one section: "environment" is declared by the web, the
// gNMI and the MCP modules, and only one of them holds any given leaf.
//
// This is the route for Go code that needs the SET rather than a mapping. A
// caller that holds its own copy of the values holds a second declaration of
// one fact, and the two drift: an operator then writes a value the model
// accepts and the Go side refuses (`ai/rules/principles.md`).
//
// The error is never traded for an empty slice. A guard that reads its allowed
// set from here refuses everything when the model cannot answer, which is the
// closed direction, and the caller states that in its own error.
//
// MUST NOT be called from an init function: the modules load through init(), so
// a caller that runs during init can see a model that is still being built.
func EnumValues(path string) ([]string, error) {
	loader, err := modelLoader()
	if err != nil {
		return nil, fmt.Errorf("load the YANG model: %w", err)
	}

	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		return nil, fmt.Errorf("resolve %q in the YANG model: the path names no section", path)
	}

	// findInEntry is the walk the validator uses for the same paths, so a path
	// this answers for is a path the validator also validates against.
	walk := NewValidator(loader)
	resolved := false
	for _, module := range loader.moduleNames() {
		entry := loader.GetEntry(module)
		if entry == nil || entry.Dir == nil {
			continue
		}
		if _, declares := entry.Dir[parts[0]]; !declares {
			continue
		}
		leaf, err := walk.findInEntry(entry, parts)
		if err != nil {
			continue
		}
		resolved = true
		if leaf.Type == nil || leaf.Type.Kind != gyang.Yenum || leaf.Type.Enum == nil {
			continue
		}
		// Names allocates and sorts a fresh slice on each call, so the caller
		// owns what it gets back.
		names := leaf.Type.Enum.Names()
		if len(names) == 0 {
			return nil, fmt.Errorf("%s declares no enumeration value in the YANG model", path)
		}
		return names, nil
	}

	if resolved {
		return nil, fmt.Errorf("%s is not an enumeration in the YANG model", path)
	}
	return nil, fmt.Errorf("resolve %s in the YANG model: no loaded module declares that path", path)
}

// EnumNamesDeclared answers the names of e in the order the module declares
// them, which is the order of their values.
//
// goyang's Names sorts alphabetically and loses the declaration order, and a
// surface that offers the values to an operator, such as a form dropdown, keeps
// the module's order: the module puts the default first and groups what belongs
// together. A surface that only asks whether a value is a member reads Names.
func EnumNamesDeclared(e *gyang.EnumType) []string {
	values := e.Values()
	slices.Sort(values)
	names := make([]string, len(values))
	for i, value := range values {
		names[i] = e.Name(value)
	}
	return names
}

// moduleNames answers every loaded module once, in name order, so two runs over
// one model read the modules in one order.
func (l *Loader) moduleNames() []string {
	names := l.ModuleNames()
	slices.Sort(names)
	return names
}
