// Design: docs/architecture/config/yang-config-design.md — YANG schema handling
// Related: validator_reference.go — the per-value checks of the reference kinds
// Related: reference_path.go — schema navigation of a leafref path
// RFC: rfc/short/rfc7950.md -- Sections 9.9, 9.9.3 and 9.13, require-instance

package yang

import (
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/openconfig/goyang/pkg/yang"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// dataNode is one node of the config data tree during a reference walk: the
// schema nodes it instantiates (several for a section that several modules
// contribute to, one elsewhere, none for the root), its children, and the
// map key of a list entry.
type dataNode struct {
	entries []*yang.Entry
	data    map[string]any
	key     string
}

// ValidateReferences checks, over the whole config data tree, that every
// leafref and instance-identifier value whose require-instance is true
// refers to an instance that exists.
//
// RFC 7950 Section 9.9.3: "If "require-instance" is "true", it means that
// the instance being referred to MUST exist for the data to be valid."
//
// root holds one entry per top-level section, as Tree.ToMap answers it.
// Every section is walked against every conf module that defines it,
// because a reference may cross sections.
func (v *Validator) ValidateReferences(root map[string]any) []ValidationError {
	var errs []ValidationError
	frames := []dataNode{{data: root}}
	var tb textbuf.Buffer
	for section, value := range root {
		for _, entry := range v.topLevel(nil, section) {
			v.walkReferences(tb.Reset().Str(section).String(), entry, value, frames, &errs)
		}
	}
	return errs
}

// walkReferences walks one data node. frames holds the data nodes from the
// root down to the parent of value. The recursion is bounded by the schema
// depth: a data node exists only where its schema node does.
func (v *Validator) walkReferences(path string, entry *yang.Entry, value any, frames []dataNode, errs *[]ValidationError) {
	var tb textbuf.Buffer
	if isLeafNode(entry) {
		items := []string{}
		if entry.IsLeafList() {
			items = leafListItems(value)
		} else if str, ok := value.(string); ok {
			items = append(items, str)
		}
		for _, item := range items {
			if err := v.checkInstance(path, entry, entry.Type, item, frames); err != nil {
				appendValidationError(errs, path, err)
			}
		}
		return
	}
	children, ok := value.(map[string]any)
	if !ok {
		return
	}
	if !entry.IsList() {
		node := dataNode{entries: []*yang.Entry{entry}, data: children}
		v.walkChildren(path, node, append(slices.Clip(frames), node), errs)
		return
	}
	for key, raw := range children {
		entryData, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		entryPath := tb.Reset().Str(path).Byte('[').Str(key).Byte(']').String()
		node := dataNode{entries: []*yang.Entry{entry}, data: entryData, key: key}
		inner := append(slices.Clip(frames), node)
		keys := listKeys(entry)
		if len(keys) == 1 {
			if _, stored := entryData[keys[0]]; !stored {
				if keyLeaf, _ := schemaChild(entry, keys[0]); keyLeaf != nil {
					if err := v.checkInstance(entryPath, keyLeaf, keyLeaf.Type, key, inner); err != nil {
						appendValidationError(errs, entryPath, err)
					}
				}
			}
		}
		v.walkChildren(entryPath, node, inner, errs)
	}
}

// walkChildren walks the children of node, whose frame closes frames.
func (v *Validator) walkChildren(path string, node dataNode, frames []dataNode, errs *[]ValidationError) {
	var tb textbuf.Buffer
	for name, child := range node.data {
		childEntry, _ := childOf(node.entries, name)
		if childEntry == nil {
			continue // a node the schema does not describe is the parser's to refuse
		}
		v.walkReferences(tb.Reset().Str(path).Byte('/').Str(name).String(), childEntry, child, frames, errs)
	}
}

// appendValidationError records err found at path.
func appendValidationError(errs *[]ValidationError, path string, err error) {
	if valErr, ok := errors.AsType[*ValidationError](err); ok {
		*errs = append(*errs, *valErr)
		return
	}
	*errs = append(*errs, ValidationError{Path: path, Type: ErrTypeType, Message: err.Error()})
}

// checkInstance checks the require-instance constraint of one value.
//
// RFC 7950 Section 9.12: a union value "is validated consecutively against
// each member type, in the order they are specified in the "type"
// statement, until a match is found". The member that matches is the one
// whose instance rule applies.
func (v *Validator) checkInstance(path string, leaf *yang.Entry, yangType *yang.YangType, value string, frames []dataNode) error {
	//exhaustive:ignore // only the kinds with an instance rule, and the union that can hold them
	switch yangType.Kind {
	case yang.Yleafref:
		return v.checkLeafrefInstance(path, leaf, yangType, value, frames)
	case yang.YinstanceIdentifier:
		return v.checkInstanceIdentifierInstance(path, leaf, yangType, value, frames)
	case yang.Yunion:
		for _, member := range yangType.Type {
			if v.validateLeafValue(path, leaf, member, value) == nil {
				return v.checkInstance(path, leaf, member, value, frames)
			}
		}
	}
	return nil
}

// RFC 7950 Section 9.9: "If the "require-instance" property (Section 9.9.3)
// is "true", there MUST exist a node in the data tree, or a node with a
// default value in use (see Sections 7.6.1 and 7.7.2), of the referred
// schema tree leaf or leaf-list node with the same value as the leafref
// value in a valid data tree."
//
// checkLeafrefInstance checks that a leafref value equals an instance of the
// referred leaf, or that leaf's default where the default is in use.
func (v *Validator) checkLeafrefInstance(path string, leaf *yang.Entry, yangType *yang.YangType, value string, frames []dataNode) error {
	if yangType.OptionalInstance {
		return nil
	}
	target, refPath, err := v.leafrefTarget(leaf, yangType)
	if err != nil {
		return referenceValidationError(path, yangType, value, err.Error())
	}
	start := v.pathStartNodes(refPath.up, frames)
	instances := v.pathValues(leaf, start, refPath.steps, frames)
	for _, instance := range instances {
		if sameValue(target.Type, instance, value) {
			return nil
		}
	}
	var tb textbuf.Buffer
	tb.Str("no ").Str(target.Name).Str(" ").Quoted(value).Str(" exists at ").Quoted(yangType.Path)
	if len(instances) > 0 {
		tb.Str(": name one of ").Quoted(strings.Join(instances, " "))
	} else {
		tb.Str(": none is configured, so add one first")
	}
	return referenceValidationError(path, yangType, value, tb.String())
}

// pathStartNodes answers the data nodes a path's first step is read under:
// the root for an absolute path, and for a relative one the node up levels
// above the referring leaf, whose parent closes frames.
func (v *Validator) pathStartNodes(up int, frames []dataNode) []dataNode {
	if up == 0 {
		return frames[:1]
	}
	at := len(frames) - up
	if at < 0 {
		return nil
	}
	return frames[at : at+1]
}

// pathValues evaluates steps from start and answers the values of the leaf
// the last step names. A predicate's current() is the referring leaf, whose
// parent closes frames.
func (v *Validator) pathValues(leaf *yang.Entry, start []dataNode, steps []pathStep, frames []dataNode) []string {
	nodes := start
	for i, step := range steps {
		name := localName(step.node)
		if i == len(steps)-1 {
			return v.leafValues(nodes, name)
		}
		nodes = v.stepNodes(leaf, nodes, name, step.keys, frames)
	}
	return nil
}

// stepNodes answers the data nodes called name under nodes, keeping the list
// entries every key predicate selects. Under the root, name is a section,
// whose schema nodes are every module contributing to it.
func (v *Validator) stepNodes(leaf *yang.Entry, nodes []dataNode, name string, keys []pathKey, frames []dataNode) []dataNode {
	var next []dataNode
	for _, node := range nodes {
		entries := v.childEntries(leaf, node, name)
		if len(entries) == 0 {
			continue
		}
		data, ok := node.data[name].(map[string]any)
		if !ok {
			continue
		}
		next = append(next, v.childNodes(leaf, entries, data, keys, frames)...)
	}
	return next
}

// childEntries answers the schema nodes of node's child called name, and
// whether that child sits under a choice.
func (v *Validator) childEntries(leaf *yang.Entry, node dataNode, name string) []*yang.Entry {
	if node.entries == nil {
		return v.topLevel(leaf, name)
	}
	if child, _ := childOf(node.entries, name); child != nil {
		return []*yang.Entry{child}
	}
	return nil
}

// childNodes answers the data nodes a container or a list holds, filtered by
// the key predicates.
func (v *Validator) childNodes(leaf *yang.Entry, entries []*yang.Entry, data map[string]any, keys []pathKey, frames []dataNode) []dataNode {
	if !entries[0].IsList() {
		return []dataNode{{entries: entries, data: data}}
	}
	var out []dataNode
	for key, raw := range data {
		entryData, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		node := dataNode{entries: entries, data: entryData, key: key}
		if v.keysMatch(leaf, node, keys, frames) {
			out = append(out, node)
		}
	}
	return out
}

// keysMatch reports whether a list entry satisfies every key predicate: its
// key equals one of the values the predicate's current() path reads.
func (v *Validator) keysMatch(leaf *yang.Entry, node dataNode, keys []pathKey, frames []dataNode) bool {
	for _, key := range keys {
		actual, ok := listEntryKey(node, localName(key.node))
		if !ok {
			return false
		}
		start := v.pathStartNodes(key.up, frames)
		steps := make([]pathStep, 0, len(key.nodes))
		for _, nodeID := range key.nodes {
			steps = append(steps, pathStep{node: nodeID})
		}
		if !slices.Contains(v.pathValues(leaf, start, steps, frames), actual) {
			return false
		}
	}
	return true
}

// listEntryKey answers the value of key leaf name of a list entry: the leaf
// when the entry stores it, else its share of the entry's map key, which for
// a list with several keys is the canonical address "<first> <key2> <value2>
// ..." (docs/architecture/config/syntax.md).
func listEntryKey(node dataNode, name string) (string, bool) {
	if str, ok := node.data[name].(string); ok {
		return str, true
	}
	keys := listKeys(node.entries[0])
	fields := strings.Fields(node.key)
	if len(keys) == 0 || len(fields) == 0 {
		return "", false
	}
	if keys[0] == name {
		if len(keys) == 1 {
			return node.key, true
		}
		return fields[0], true
	}
	for i := 1; i+1 < len(fields); i += 2 {
		if fields[i] == name {
			return fields[i+1], true
		}
	}
	return "", false
}

// leafValues answers the values of leaf name under each of nodes: the
// configured value, the list key it is part of, or its default where the
// default is in use (RFC 7950 Section 7.6.1: the parent exists and the leaf
// is not under a choice whose other case is present).
func (v *Validator) leafValues(nodes []dataNode, name string) []string {
	var values []string
	for _, node := range nodes {
		if node.entries == nil {
			continue // Ze's data root holds sections, never a leaf
		}
		target, grouped := childOf(node.entries, name)
		if target == nil {
			continue
		}
		if raw, ok := node.data[name]; ok {
			if target.IsLeafList() {
				values = append(values, leafListItems(raw)...)
			} else if str, ok := raw.(string); ok {
				values = append(values, str)
			}
			continue
		}
		if node.key != "" && slices.Contains(listKeys(node.entries[0]), name) {
			if key, ok := listEntryKey(node, name); ok {
				values = append(values, key)
			}
			continue
		}
		if grouped {
			continue
		}
		values = append(values, entryDefaults(target)...)
	}
	return values
}

// entryDefaults answers the defaults of a leaf: its own, else its type's.
func entryDefaults(e *yang.Entry) []string {
	if len(e.Default) > 0 {
		return e.Default
	}
	if e.Type != nil && e.Type.HasDefault {
		return []string{e.Type.Default}
	}
	return nil
}

// sameValue reports whether two values of a type are the same value. An
// integer compares by number, so "+1" and "1" agree; every other kind
// compares as the tree stores it.
func sameValue(yangType *yang.YangType, a, b string) bool {
	if a == b {
		return true
	}
	if yangType == nil {
		return false
	}
	//exhaustive:ignore // only the integer kinds have a lexical form wider than their value
	switch yangType.Kind {
	case yang.Yint8, yang.Yint16, yang.Yint32, yang.Yint64:
		x, errA := strconv.ParseInt(a, 10, 64)
		y, errB := strconv.ParseInt(b, 10, 64)
		return errA == nil && errB == nil && x == y
	case yang.Yuint8, yang.Yuint16, yang.Yuint32, yang.Yuint64:
		x, errA := strconv.ParseUint(strings.TrimPrefix(a, "+"), 10, 64)
		y, errB := strconv.ParseUint(strings.TrimPrefix(b, "+"), 10, 64)
		return errA == nil && errB == nil && x == y
	}
	return false
}

// RFC 7950 Section 9.13: "All such leaf nodes MUST reference existing nodes
// or leaf or leaf-list nodes with their default value in use (see Sections
// 7.6.1 and 7.7.2) for the data to be valid."
//
// checkInstanceIdentifierInstance checks that the node an
// instance-identifier value names exists.
func (v *Validator) checkInstanceIdentifierInstance(path string, leaf *yang.Entry, yangType *yang.YangType, value string, frames []dataNode) error {
	if yangType.OptionalInstance {
		return nil
	}
	steps, ok := parseInstanceIdentifier(value)
	if !ok {
		return referenceValidationError(path, yangType, value, "not an instance-identifier")
	}
	schema, err := v.instanceSchema(leaf, steps)
	if err != nil {
		return referenceValidationError(path, yangType, value, err.Error())
	}
	if instanceExists(frames[0].data, schema, steps) {
		return nil
	}
	return referenceValidationError(path, yangType, value, "no node exists at "+value+": configure it first, or correct the path")
}

// instanceExists walks the data tree along an instance-identifier whose
// schema nodes instanceSchema resolved.
func instanceExists(root map[string]any, schema []*yang.Entry, steps []instanceStep) bool {
	data := root
	for i, step := range steps {
		node := schema[i]
		name := node.Name
		raw, present := data[name]
		last := i == len(steps)-1
		if !present {
			if !last || !isLeafNode(node) || step.hasValue {
				return false
			}
			return len(entryDefaults(node)) > 0
		}
		switch {
		case node.IsList():
			entries, ok := raw.(map[string]any)
			if !ok {
				return false
			}
			entryData, found := selectListEntry(node, entries, step)
			if !found {
				return false
			}
			data = entryData
		case node.IsLeafList():
			if !step.hasValue {
				return last
			}
			return slices.Contains(leafListItems(raw), step.value)
		case node.IsLeaf():
			return last
		default:
			children, ok := raw.(map[string]any)
			if !ok {
				return false
			}
			data = children
		}
	}
	return true
}

// selectListEntry answers the list entry an instance-identifier step selects:
// by its keys, or by position for a list with none.
func selectListEntry(list *yang.Entry, entries map[string]any, step instanceStep) (map[string]any, bool) {
	if step.pos > 0 {
		if step.pos > len(entries) {
			return nil, false
		}
		for _, raw := range entries {
			if entryData, ok := raw.(map[string]any); ok {
				return entryData, true
			}
		}
		return nil, false
	}
	for key, raw := range entries {
		entryData, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		node := dataNode{entries: []*yang.Entry{list}, data: entryData, key: key}
		if instanceKeysMatch(node, step.keys) {
			return entryData, true
		}
	}
	return nil, false
}

// instanceKeysMatch reports whether a list entry carries every key value.
func instanceKeysMatch(node dataNode, keys []instanceKey) bool {
	for _, key := range keys {
		actual, ok := listEntryKey(node, localName(key.node))
		if !ok {
			return false
		}
		if actual != key.value {
			return false
		}
	}
	return true
}
