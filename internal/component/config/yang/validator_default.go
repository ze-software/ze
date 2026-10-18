// Design: docs/architecture/config/yang-config-design.md — YANG schema handling
// Related: validator_instance.go — the reference walk that reads defaults in use
// Related: module_default.go — the module notation a default is written in
// RFC: rfc/short/rfc7950.md -- Sections 7.6.1 and 7.9.3, defaults in use

package yang

import (
	"slices"

	"github.com/openconfig/goyang/pkg/yang"
)

// RFC 7950 Section 7.6.1: "The usage of the default value depends on the
// leaf's closest ancestor node in the schema tree that is not a non-presence
// container (see Section 7.5.1)" and "Otherwise, if this ancestor is a case
// node, the default value MUST be used if any node from the case exists in
// the data tree or the case node is the choice's default case, and if no
// nodes from any other case exist in the data tree."
//
// defaultInUse reports whether the default of e, absent from data, the
// children of its data parent, is in use. The data parent itself is in the
// tree, or is a non-presence container whose own use absentContainerInUse
// answered, so what remains is the choices between e and data: each case on
// the way must be the one in effect.
func defaultInUse(e *yang.Entry, data map[string]any) bool {
	return caseInEffect(e, data)
}

// absentContainerInUse reports whether e, absent from data, is a
// non-presence container that the reference walk enters as empty, so the
// defaults below it can be in use: a non-presence container is never the
// ancestor Section 7.6.1 tests, and every case between it and data must be
// in effect.
func absentContainerInUse(e *yang.Entry, data map[string]any) bool {
	if !isNonPresenceContainer(e) {
		return false
	}
	return caseInEffect(e, data)
}

// RFC 7950 Section 7.9.3: "The default values and nested default cases
// under the default case are used if none of the nodes under any of the
// cases are present."
//
// caseInEffect reports whether every case between e and its data parent is
// in effect over data: the case holds data and no other case does, or no
// case holds data and it is the choice's default case. The loop climbs one
// choice per pass, so it is bounded by the choice nesting of the module.
func caseInEffect(e *yang.Entry, data map[string]any) bool {
	node := e
	for node.Parent != nil && isSchemaGroup(node.Parent) {
		choice, branch := node.Parent, node
		if choice.IsCase() {
			branch = choice
			choice = choice.Parent
		}
		if choice == nil {
			return false
		}
		active := activeBranch(choice, data)
		if active == nil {
			if !slices.Contains(choice.Default, branch.Name) {
				return false
			}
		} else if active != branch {
			return false
		}
		node = choice
	}
	return true
}

// activeBranch answers the branch of choice, a case or a shorthand data
// node, that holds a node present in data, nil when none does.
func activeBranch(choice *yang.Entry, data map[string]any) *yang.Entry {
	for _, branch := range choice.Dir {
		if holdsData(branch, data) {
			return branch
		}
	}
	return nil
}

// holdsData reports whether a schema node, or a data node under it through
// cases and choices, is present in data. The recursion is bounded by the
// choice nesting of the module.
func holdsData(e *yang.Entry, data map[string]any) bool {
	if !isSchemaGroup(e) {
		_, present := data[e.Name]
		return present
	}
	for _, child := range e.Dir {
		if holdsData(child, data) {
			return true
		}
	}
	return false
}

// canonicalDefaults answers the defaults of a leaf or leaf-list, its own
// else its type's, each in the canonical form of the module notation it was
// written in (CanonicalModuleDefault): "0x10" is the value 16. A default the
// notation refuses is answered as written, because the schema build refuses
// the module for it.
func canonicalDefaults(e *yang.Entry) []string {
	defaults := e.Default
	if len(defaults) == 0 && e.Type != nil && e.Type.HasDefault {
		defaults = []string{e.Type.Default}
	}
	out := make([]string, 0, len(defaults))
	for _, def := range defaults {
		canonical, err := CanonicalModuleDefault(e, e.Type, def)
		if err != nil {
			canonical = def
		}
		out = append(out, canonical)
	}
	return out
}
