// Design: docs/architecture/config/yang-config-design.md — YANG schema handling
// Related: loader.go — Resolve runs checkReferences once every other check passed
// Related: reference_path.go — leafrefTarget resolves a path against the schema
// RFC: rfc/short/rfc7950.md -- Sections 9.9 and 9.9.2, leafref paths

package yang

import (
	"errors"
	"fmt"

	"github.com/openconfig/goyang/pkg/yang"
)

// ErrLeafrefPath marks a leafref whose path the schema cannot honor: no path,
// a path naming no leaf, a config leaf referring to state, or a loop.
var ErrLeafrefPath = errors.New("YANG leafref path invalid")

// checkReferences checks every leafref in the resolved module set, over the
// entries goyang built, because a path is resolved against the schema tree.
func checkReferences(resolved *Resolved) error {
	v := NewValidator(resolved)
	var errs []error
	for _, name := range resolved.ModuleNames() {
		root := resolved.GetEntry(name)
		if root == nil {
			continue
		}
		v.checkEntryReferences(name, root, &errs)
	}
	return errors.Join(errs...)
}

// checkEntryReferences checks the leafrefs at and below e. The recursion is
// bounded by the depth of the loaded modules.
func (v *Validator) checkEntryReferences(module string, e *yang.Entry, errs *[]error) {
	if isLeafNode(e) && e.Type != nil {
		for _, member := range referenceMembers(e.Type) {
			if err := v.checkLeafrefSchema(e, member); err != nil {
				*errs = append(*errs, fmt.Errorf("%w: module %s: %s: %w", ErrLeafrefPath, module, e.Path(), err))
			}
		}
	}
	for _, child := range e.Dir {
		v.checkEntryReferences(module, child, errs)
	}
	if e.RPC != nil {
		if e.RPC.Input != nil {
			v.checkEntryReferences(module, e.RPC.Input, errs)
		}
		if e.RPC.Output != nil {
			v.checkEntryReferences(module, e.RPC.Output, errs)
		}
	}
}

// referenceMembers answers the leafref types a type is or holds as a union
// member, at any union depth.
func referenceMembers(t *yang.YangType) []*yang.YangType {
	if t.Kind == yang.Yleafref {
		return []*yang.YangType{t}
	}
	if t.Kind != yang.Yunion {
		return nil
	}
	var found []*yang.YangType
	for _, member := range t.Type {
		found = append(found, referenceMembers(member)...)
	}
	return found
}

// RFC 7950 Section 9.9: "If the referring node represents configuration
// data and the "require-instance" property (Section 9.9.3) is "true", the
// referred node MUST also represent configuration."
// RFC 7950 Section 9.9: "There MUST NOT be any circular chains of leafrefs."
//
// checkLeafrefSchema checks one leafref of leaf: its path names a leaf, a
// config leaf requiring an instance refers to config, and following the
// chain of leafrefs never comes back to a leaf already visited.
func (v *Validator) checkLeafrefSchema(leaf *yang.Entry, typ *yang.YangType) error {
	target, _, err := v.leafrefTarget(leaf, typ)
	if err != nil {
		return err
	}
	if !typ.OptionalInstance && !leaf.ReadOnly() && target.ReadOnly() {
		return fmt.Errorf("the leaf is configuration and requires an instance, but %s is state: "+
			"point the path at a config leaf, or add require-instance false", target.Path())
	}
	visited := map[*yang.Entry]bool{leaf: true}
	node := target
	// Each pass visits a new leaf or stops, so the loop is bounded by the
	// number of leaves in the schema.
	for {
		if visited[node] {
			return fmt.Errorf("the leafref chain through %s comes back to %s: break the loop", target.Path(), node.Path())
		}
		visited[node] = true
		if node.Type == nil || node.Type.Kind != yang.Yleafref {
			return nil
		}
		next, _, err := v.leafrefTarget(node, node.Type)
		if err != nil {
			return nil //nolint:nilerr // That leafref's own check reports its path, so the chain ends here.
		}
		node = next
	}
}
