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
	if t == nil {
		return nil
	}
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
// chain of leafrefs, through union members too, never comes back to leaf.
func (v *Validator) checkLeafrefSchema(leaf *yang.Entry, typ *yang.YangType) error {
	target, _, err := v.leafrefTarget(leaf, typ)
	if err != nil {
		return err
	}
	if !typ.OptionalInstance && representsConfig(leaf) && target.ReadOnly() {
		return fmt.Errorf("the leaf is configuration and requires an instance, but %s is state: "+
			"point the path at a config leaf, or add require-instance false", target.Path())
	}
	return v.checkLeafrefLoop(leaf, target)
}

// representsConfig reports whether leaf is configuration data: a config true
// node of the data tree. A node under an rpc or action input or output, or
// under a notification, is no configuration whatever its config property, so
// Section 9.9 lets its leafref refer to state. goyang answers ReadOnly false
// for an input node, because no config statement is above it.
func representsConfig(leaf *yang.Entry) bool {
	for node := leaf; node != nil; node = node.Parent {
		//exhaustive:ignore // a predicate over the entry kinds that hold no data tree node
		switch node.Kind {
		case yang.InputEntry, yang.OutputEntry, yang.NotificationEntry:
			return false
		}
	}
	return !leaf.ReadOnly()
}

// checkLeafrefLoop refuses a chain of leafrefs that leads from target back to
// leaf. A leaf whose type is a union follows every leafref member, so the
// chain branches; a loop that does not pass through leaf is refused by the
// check of a leaf on it. Each node is expanded once, so the walk is bounded by
// the number of leaves in the schema.
func (v *Validator) checkLeafrefLoop(leaf, target *yang.Entry) error {
	visited := map[*yang.Entry]bool{}
	pending := []*yang.Entry{target}
	for len(pending) > 0 {
		node := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if node == leaf {
			return fmt.Errorf("the leafref chain through %s comes back to %s: break the loop", target.Path(), leaf.Path())
		}
		if visited[node] {
			continue
		}
		visited[node] = true
		for _, member := range referenceMembers(node.Type) {
			next, _, err := v.leafrefTarget(node, member)
			if err != nil {
				continue // that leafref's own check reports its path
			}
			pending = append(pending, next)
		}
	}
	return nil
}
