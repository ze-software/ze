// Design: docs/architecture/config/yang-config-design.md — YANG schema handling
// Related: validator_instance.go — the reference walk this runs at one position
// Related: reference_source.go — the ze:reference sources it also asks
// RFC: rfc/short/rfc7950.md -- Section 9.9, the instances a leafref may name

package yang

import (
	"maps"
	"slices"

	"github.com/openconfig/goyang/pkg/yang"
)

// ReferenceInstances answers, sorted, the values the reference leaf at
// leafPath may hold in the config data tree root: the instances its leafref
// paths reach, including each leafref member of a union, and the names its
// ze:reference source lists. It runs the walk ValidateReferences checks a
// value with, at one position instead of over the whole tree, so a value it
// answers is one that check accepts; a `current()` predicate reads the data
// around the position.
//
// root is the tree as Tree.ToPluginMap lowers it. leafPath is the leaf's data
// path: the section, each container and list name, each list entry's map key
// after the name of its list, and the leaf's name last. A position the tree
// does not hold yet reads as empty, so a leaf completed inside an entry being
// written still reaches instances configured elsewhere. A path the schema
// does not describe, a leaf that is no reference, and a ze:reference source no
// component registered answer nil: commit names the unregistered source.
func (v *Validator) ReferenceInstances(root map[string]any, leafPath []string) []string {
	w := newReferenceWalk(v)
	leaf, frames := w.leafFrames(root, leafPath)
	if leaf == nil {
		return nil
	}
	found := make(map[string]string)
	w.typeInstances(leaf, leaf.Type, frames, found)
	values := slices.Collect(maps.Values(found))
	if source := GetReferenceExtension(leaf); source != "" {
		answer, err := w.sourceInstances(leaf, source, frames)
		if err == nil {
			values = append(values, answer.Names...)
		}
	}
	if len(values) == 0 {
		return nil
	}
	slices.Sort(values)
	return slices.Compact(values)
}

// leafFrames answers the schema node of the leaf at leafPath and the data
// nodes from the root down to its parent, numbered as the walk numbers the
// nodes it descends through. It answers a nil leaf for a path the schema does
// not describe or that names no leaf.
func (w *referenceWalk) leafFrames(root map[string]any, leafPath []string) (*yang.Entry, []dataNode) {
	if len(leafPath) < 2 {
		return nil, nil
	}
	frames := []dataNode{{data: root, id: w.nodeID()}}
	var entries []*yang.Entry
	data := root
	last := len(leafPath) - 1
	for i := 0; i < last; i++ {
		name := leafPath[i]
		if i == 0 {
			entries = w.v.topLevel(nil, name)
		} else {
			entries = childrenOf(entries, name)
		}
		if len(entries) == 0 {
			return nil, nil
		}
		if isLeafNode(entries[0]) {
			return nil, nil
		}
		data, _ = data[name].(map[string]any) // an absent node reads as empty
		node := dataNode{entries: entries}
		if entries[0].IsList() {
			i++
			if i == last {
				return nil, nil // a list name with no entry key names no leaf
			}
			node.key = leafPath[i]
			data, _ = data[node.key].(map[string]any) // an absent entry reads as empty
		}
		node.data = data
		node.id = w.nodeID()
		frames = append(frames, node)
	}
	leaf := childOf(entries, leafPath[last])
	if leaf == nil {
		return nil, nil
	}
	if !isLeafNode(leaf) {
		return nil, nil
	}
	return leaf, frames
}

// childrenOf answers the child called name of every parent that has one.
// Several modules declare one top-level container (system), each attaching
// its own children, so a node below it is looked up under every module's
// copy: the first copy alone may not hold the child the path names.
func childrenOf(parents []*yang.Entry, name string) []*yang.Entry {
	var children []*yang.Entry
	for _, parent := range parents {
		if child := schemaChild(parent, name); child != nil {
			children = append(children, child)
		}
	}
	return children
}

// typeInstances adds to found, keyed by canonical value, the instances the
// leafref paths of yangType reach from leaf, whose parent closes frames: its
// own path, or each leafref member of a union. The recursion is bounded by
// the union nesting the module declares.
func (w *referenceWalk) typeInstances(leaf *yang.Entry, yangType *yang.YangType, frames []dataNode, found map[string]string) {
	if yangType == nil {
		return
	}
	//exhaustive:ignore // only the leafref, whose path names instances, and the union that can hold one
	switch yangType.Kind {
	case yang.Yleafref:
		resolved := w.leafrefTarget(leaf, yangType)
		if resolved.err != nil {
			return // the leafref cannot resolve, so it names no instance; commit refuses its value
		}
		maps.Copy(found, w.instancesAt(leaf, resolved, yangType, frames))
	case yang.Yunion:
		for _, member := range yangType.Type {
			w.typeInstances(leaf, member, frames, found)
		}
	}
}

// IsReferenceLeaf reports whether a leaf or leaf-list names another config
// entry: its type is, or has a union member that is, a leafref or an
// instance-identifier, or it carries a ze:reference source. Completion asks it
// before lowering the tree for ReferenceInstances.
func IsReferenceLeaf(e *yang.Entry) bool {
	if e == nil {
		return false
	}
	if GetReferenceExtension(e) != "" {
		return true
	}
	return typeHoldsReference(e.Type)
}
