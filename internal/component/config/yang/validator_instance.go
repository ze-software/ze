// Design: docs/architecture/config/yang-config-design.md — YANG schema handling
// Related: validator_reference.go — the per-value checks of the reference kinds
// Related: reference_path.go — schema navigation of a leafref path
// Related: validator_default.go — which defaults are in use
// RFC: rfc/short/rfc7950.md -- Sections 9.9, 9.9.3 and 9.13, require-instance

package yang

import (
	"errors"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/openconfig/goyang/pkg/yang"

	"github.com/ze-software/ze/internal/core/configorder"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// referenceChoicesMax bounds how many configured instances a refusal lists.
const referenceChoicesMax = 10

// dataNode is one node of the config data tree during a reference walk: the
// schema nodes it instantiates (several for a section that several modules
// contribute to, one elsewhere, none for the root), its children, the map key
// of a list entry, and, for a node the walk descended through, its config
// path, which keys the instance index.
type dataNode struct {
	entries []*yang.Entry
	data    map[string]any
	key     string
	path    string
}

// referenceKey names one reference type of one leaf.
type referenceKey struct {
	leaf     *yang.Entry
	yangType *yang.YangType
}

// referenceTarget is a leafref type resolved once per walk: the leaf it
// refers to and its parsed path, or why it does not resolve.
type referenceTarget struct {
	target *yang.Entry
	path   leafrefPath
	err    error
}

// referenceWalk is the state of one ValidateReferences call. It caches what
// does not change between the values it checks, so the walk is linear in the
// size of the data: which schema subtrees hold a reference, each leafref
// type's target, and the instances a predicate-free path reaches from one
// data node. It is used by one goroutine for one call.
type referenceWalk struct {
	v         *Validator
	errs      []ValidationError
	holds     map[*yang.Entry]bool
	targets   map[referenceKey]referenceTarget
	instances map[string]map[string]string // canonical value -> value as configured
}

// newReferenceWalk answers an empty walk over v's schema.
func newReferenceWalk(v *Validator) *referenceWalk {
	return &referenceWalk{
		v:         v,
		holds:     make(map[*yang.Entry]bool),
		targets:   make(map[referenceKey]referenceTarget),
		instances: make(map[string]map[string]string),
	}
}

// ValidateReferences checks, over the whole config data tree, that every
// leafref and instance-identifier value whose require-instance is true
// refers to an instance that exists.
//
// RFC 7950 Section 9.9.3: "If "require-instance" is "true", it means that
// the instance being referred to MUST exist for the data to be valid."
//
// root holds one entry per top-level section, as Tree.ToPluginMap answers
// it: the entry order of each list travels beside it (configorder), which a
// positional instance-identifier predicate counts in. Every section is walked
// against every conf module that defines it, because a reference may cross
// sections.
func (v *Validator) ValidateReferences(root map[string]any) []ValidationError {
	w := newReferenceWalk(v)
	frames := []dataNode{{data: root}}
	for section, value := range root {
		for _, entry := range v.topLevel(nil, section) {
			w.walk(section, entry, value, frames)
		}
	}
	return w.errs
}

// holdsReference reports whether the schema subtree at e holds a leafref or
// an instance-identifier, so the walk enters no subtree that cannot refuse.
// The recursion is bounded by the schema depth, which Ze builds from its own
// modules.
func (w *referenceWalk) holdsReference(e *yang.Entry) bool {
	if held, ok := w.holds[e]; ok {
		return held
	}
	held := false
	if isLeafNode(e) {
		held = typeHoldsReference(e.Type)
	} else {
		for _, child := range e.Dir {
			if w.holdsReference(child) {
				held = true
				break
			}
		}
	}
	w.holds[e] = held
	return held
}

// typeHoldsReference reports whether a type is, or has a union member that
// is, a leafref or an instance-identifier. The recursion is bounded by the
// union nesting the module declares.
func typeHoldsReference(t *yang.YangType) bool {
	if t == nil {
		return false
	}
	//exhaustive:ignore // a predicate over the reference kinds and the union that can hold them
	switch t.Kind {
	case yang.Yleafref, yang.YinstanceIdentifier:
		return true
	case yang.Yunion:
		return slices.ContainsFunc(t.Type, typeHoldsReference)
	}
	return false
}

// walk walks one data node. frames holds the data nodes from the root down
// to the parent of value. The recursion is bounded by the schema depth: a
// data node exists only where its schema node does.
func (w *referenceWalk) walk(path string, entry *yang.Entry, value any, frames []dataNode) {
	if !w.holdsReference(entry) {
		return
	}
	if isLeafNode(entry) {
		w.walkLeaf(path, entry, value, frames)
		return
	}
	children, ok := value.(map[string]any)
	if !ok {
		return
	}
	if !entry.IsList() {
		node := dataNode{entries: []*yang.Entry{entry}, data: children, path: path}
		w.walkChildren(node, append(slices.Clip(frames), node))
		return
	}
	var tb textbuf.Buffer
	keys := listKeys(entry)
	for key, raw := range children {
		entryData, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		entryPath := tb.Reset().Str(path).Byte('[').Str(key).Byte(']').String()
		node := dataNode{entries: []*yang.Entry{entry}, data: entryData, key: key, path: entryPath}
		inner := append(slices.Clip(frames), node)
		if len(keys) == 1 {
			w.checkMapKey(entryPath, entry, keys[0], key, inner)
		}
		w.walkChildren(node, inner)
	}
}

// walkLeaf checks every value a leaf or leaf-list holds.
func (w *referenceWalk) walkLeaf(path string, entry *yang.Entry, value any, frames []dataNode) {
	items := []string{}
	if entry.IsLeafList() {
		items = leafListItems(value)
	} else if str, ok := value.(string); ok {
		items = append(items, str)
	}
	for _, item := range items {
		if err := w.checkInstance(path, entry, entry.Type, item, frames); err != nil {
			w.appendError(path, err)
		}
	}
}

// checkMapKey checks the key of a single-key list entry that the tree keeps
// only as the entry's map key.
func (w *referenceWalk) checkMapKey(path string, list *yang.Entry, keyName, key string, frames []dataNode) {
	if _, stored := frames[len(frames)-1].data[keyName]; stored {
		return
	}
	keyLeaf := schemaChild(list, keyName)
	if keyLeaf == nil {
		return
	}
	if err := w.checkInstance(path, keyLeaf, keyLeaf.Type, key, frames); err != nil {
		w.appendError(path, err)
	}
}

// walkChildren walks the children of node, whose frame closes frames.
func (w *referenceWalk) walkChildren(node dataNode, frames []dataNode) {
	var tb textbuf.Buffer
	for name, child := range node.data {
		childEntry := childOf(node.entries, name)
		if childEntry == nil {
			continue // a node the schema does not describe is the parser's to refuse
		}
		w.walk(tb.Reset().Str(node.path).Byte('/').Str(name).String(), childEntry, child, frames)
	}
}

// appendError records err found at path.
func (w *referenceWalk) appendError(path string, err error) {
	if valErr, ok := errors.AsType[*ValidationError](err); ok {
		w.errs = append(w.errs, *valErr)
		return
	}
	w.errs = append(w.errs, ValidationError{Path: path, Type: ErrTypeReference, Message: err.Error()})
}

// checkInstance checks the require-instance constraint of one value.
//
// RFC 7950 Section 9.12: a union value "is validated consecutively against
// each member type, in the order they are specified in the "type"
// statement, until a match is found". The member that matches is the one
// whose instance rule applies.
func (w *referenceWalk) checkInstance(path string, leaf *yang.Entry, yangType *yang.YangType, value string, frames []dataNode) error {
	//exhaustive:ignore // only the kinds with an instance rule, and the union that can hold them
	switch yangType.Kind {
	case yang.Yleafref:
		return w.checkLeafrefInstance(path, leaf, yangType, value, frames)
	case yang.YinstanceIdentifier:
		return w.checkInstanceIdentifierInstance(path, leaf, yangType, value, frames)
	case yang.Yunion:
		for _, member := range yangType.Type {
			if w.v.validateLeafValue(path, leaf, member, value) == nil {
				return w.checkInstance(path, leaf, member, value, frames)
			}
		}
	}
	return nil
}

// leafrefTarget answers the target of a leafref type of leaf, resolved once
// per walk.
func (w *referenceWalk) leafrefTarget(leaf *yang.Entry, yangType *yang.YangType) referenceTarget {
	key := referenceKey{leaf: leaf, yangType: yangType}
	if resolved, ok := w.targets[key]; ok {
		return resolved
	}
	target, path, err := w.v.leafrefTarget(leaf, yangType)
	resolved := referenceTarget{target: target, path: path, err: err}
	w.targets[key] = resolved
	return resolved
}

// RFC 7950 Section 9.9: "If the "require-instance" property (Section 9.9.3)
// is "true", there MUST exist a node in the data tree, or a node with a
// default value in use (see Sections 7.6.1 and 7.7.2), of the referred
// schema tree leaf or leaf-list node with the same value as the leafref
// value in a valid data tree."
//
// checkLeafrefInstance checks that a leafref value equals an instance of the
// referred leaf, or that leaf's default where the default is in use.
func (w *referenceWalk) checkLeafrefInstance(path string, leaf *yang.Entry, yangType *yang.YangType, value string, frames []dataNode) error {
	if yangType.OptionalInstance {
		return nil
	}
	resolved := w.leafrefTarget(leaf, yangType)
	if resolved.err != nil {
		return missingReferenceError(path, yangType, value, resolved.err.Error())
	}
	instances := w.instancesAt(leaf, resolved, yangType, frames)
	if _, ok := instances[canonicalValue(resolved.target.Type, value)]; ok {
		return nil
	}
	return missingReferenceError(path, yangType, value, absentLeafrefMessage(resolved.target, yangType, value, instances))
}

// instancesAt answers the instances a leafref path reaches from the leaf
// whose parent closes frames, keyed by canonical value. A path with no
// predicate reaches the same instances from the same start node, so its
// answer is computed once per start node and reused for every value read
// from there: the reason the walk stays linear in the referred list.
func (w *referenceWalk) instancesAt(leaf *yang.Entry, resolved referenceTarget, yangType *yang.YangType, frames []dataNode) map[string]string {
	start := pathStartNodes(resolved.path.up, frames)
	if len(start) == 0 {
		return nil
	}
	if resolved.path.hasPredicate() {
		return instanceSet(resolved.target.Type, w.v.pathValues(leaf, start, resolved.path.steps, frames))
	}
	var tb textbuf.Buffer
	key := tb.Str(start[0].path).Byte(0).Str(yangType.Path).String()
	if set, ok := w.instances[key]; ok {
		return set
	}
	set := instanceSet(resolved.target.Type, w.v.pathValues(leaf, start, resolved.path.steps, frames))
	w.instances[key] = set
	return set
}

// hasPredicate reports whether any step of a path carries a key predicate,
// whose current() makes the instances depend on the referring leaf.
func (p leafrefPath) hasPredicate() bool {
	for _, step := range p.steps {
		if len(step.keys) > 0 {
			return true
		}
	}
	return false
}

// instanceSet indexes values by their canonical form under yangType.
func instanceSet(yangType *yang.YangType, values []string) map[string]string {
	set := make(map[string]string, len(values))
	for _, value := range values {
		set[canonicalValue(yangType, value)] = value
	}
	return set
}

// absentLeafrefMessage says which value is missing where, and what to
// configure: one of the instances that exist, or, when there is none, the
// node that would hold value.
func absentLeafrefMessage(target *yang.Entry, yangType *yang.YangType, value string, instances map[string]string) string {
	var tb textbuf.Buffer
	tb.Str("no ").Str(target.Name).Str(" ").Quoted(value).Str(" exists at ").Quoted(yangType.Path)
	if len(instances) == 0 {
		tb.Str(", and none is configured: ")
		parent := dataParent(target)
		if parent != nil && parent.IsList() && slices.Contains(listKeys(parent), target.Name) {
			tb.Str("add ").Str(parent.Name).Byte(' ').Str(value).Str(" under ").Str(schemaLocation(dataParent(parent)))
		} else {
			tb.Str("set ").Str(target.Name).Byte(' ').Str(value).Str(" under ").Str(schemaLocation(parent))
		}
		return tb.Str(" first, or correct the value").String()
	}
	names := slices.Sorted(maps.Values(instances))
	shown := names[:min(len(names), referenceChoicesMax)]
	tb.Str(": name one of ").Quoted(strings.Join(shown, " "))
	if len(names) > len(shown) {
		tb.Str(" (").Int(int64(len(names) - len(shown))).Str(" more configured)")
	}
	return tb.String()
}

// schemaLocation answers the config path of a schema node, its data
// ancestors joined by "/", or "the top level" for none.
func schemaLocation(e *yang.Entry) string {
	var names []string
	for node := e; node != nil && node.Parent != nil; node = dataParent(node) {
		names = append(names, node.Name)
	}
	if len(names) == 0 {
		return "the top level"
	}
	slices.Reverse(names)
	return strings.Join(names, "/")
}

// pathStartNodes answers the data nodes a path's first step is read under:
// the root for an absolute path, and for a relative one the node up levels
// above the referring leaf, whose parent closes frames.
func pathStartNodes(up int, frames []dataNode) []dataNode {
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
			return leafValues(nodes, name)
		}
		nodes = v.stepNodes(leaf, nodes, name, step.keys, frames)
	}
	return nil
}

// stepNodes answers the data nodes called name under nodes, keeping the list
// entries every key predicate selects. Under the root, name is a section,
// whose schema nodes are every module contributing to it. An absent
// non-presence container is answered as an empty node, because the defaults
// of the leaves below it are in use (RFC 7950 Section 7.6.1).
func (v *Validator) stepNodes(leaf *yang.Entry, nodes []dataNode, name string, keys []pathKey, frames []dataNode) []dataNode {
	var next []dataNode
	for _, node := range nodes {
		entries := v.childEntries(leaf, node, name)
		if len(entries) == 0 {
			continue
		}
		raw, present := node.data[name]
		if !present {
			if absentContainerInUse(entries[0], node.data) {
				next = append(next, dataNode{entries: entries})
			}
			continue
		}
		data, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		next = append(next, v.childNodes(leaf, entries, data, keys, frames)...)
	}
	return next
}

// childEntries answers the schema nodes of node's child called name.
func (v *Validator) childEntries(leaf *yang.Entry, node dataNode, name string) []*yang.Entry {
	if node.entries == nil {
		return v.topLevel(leaf, name)
	}
	if child := childOf(node.entries, name); child != nil {
		return []*yang.Entry{child}
	}
	return nil
}

// childNodes answers the data nodes a container or a list holds, filtered by
// the key predicates. A predicate on the single key of a list names the
// entries it selects, which are read by their map key rather than by a scan
// of the list.
func (v *Validator) childNodes(leaf *yang.Entry, entries []*yang.Entry, data map[string]any, keys []pathKey, frames []dataNode) []dataNode {
	if !entries[0].IsList() {
		return []dataNode{{entries: entries, data: data}}
	}
	var out []dataNode
	candidates, keyed := v.keyCandidates(leaf, entries[0], keys, frames)
	if keyed {
		for _, key := range candidates {
			entryData, ok := data[key].(map[string]any)
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

// keyCandidates answers the values a predicate on the single key of list
// allows, and whether there is such a predicate.
func (v *Validator) keyCandidates(leaf, list *yang.Entry, keys []pathKey, frames []dataNode) ([]string, bool) {
	names := listKeys(list)
	if len(names) != 1 {
		return nil, false
	}
	for _, key := range keys {
		if localName(key.node) == names[0] {
			return v.keyValues(leaf, key, frames), true
		}
	}
	return nil, false
}

// keyValues answers the values a predicate's current() path reads.
func (v *Validator) keyValues(leaf *yang.Entry, key pathKey, frames []dataNode) []string {
	start := pathStartNodes(key.up, frames)
	steps := make([]pathStep, 0, len(key.nodes))
	for _, nodeID := range key.nodes {
		steps = append(steps, pathStep{node: nodeID})
	}
	return v.pathValues(leaf, start, steps, frames)
}

// keysMatch reports whether a list entry satisfies every key predicate: its
// key equals one of the values the predicate's current() path reads.
func (v *Validator) keysMatch(leaf *yang.Entry, node dataNode, keys []pathKey, frames []dataNode) bool {
	for _, key := range keys {
		actual, ok := listEntryKey(node, localName(key.node))
		if !ok {
			return false
		}
		if !slices.Contains(v.keyValues(leaf, key, frames), actual) {
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
// default is in use (defaultInUse), in the canonical form of the module
// notation.
func leafValues(nodes []dataNode, name string) []string {
	var values []string
	for _, node := range nodes {
		if node.entries == nil {
			continue // Ze's data root holds sections, never a leaf
		}
		target := childOf(node.entries, name)
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
		if defaultInUse(target, node.data) {
			values = append(values, canonicalDefaults(target)...)
		}
	}
	return values
}

// canonicalValue answers the form two values of a type are compared in. An
// integer compares by number, so "+1", "01" and "1" agree; every other kind
// compares as the tree stores it. A value that does not parse is answered as
// written, so it equals only itself.
func canonicalValue(yangType *yang.YangType, value string) string {
	if yangType == nil {
		return value
	}
	var tb textbuf.Buffer
	//exhaustive:ignore // only the integer kinds have a lexical form wider than their value
	switch yangType.Kind {
	case yang.Yint8, yang.Yint16, yang.Yint32, yang.Yint64:
		if n, err := strconv.ParseInt(value, 10, 64); err == nil {
			return tb.Int(n).String()
		}
	case yang.Yuint8, yang.Yuint16, yang.Yuint32, yang.Yuint64:
		if n, err := strconv.ParseUint(strings.TrimPrefix(value, "+"), 10, 64); err == nil {
			return tb.Uint(n).String()
		}
	}
	return value
}

// RFC 7950 Section 9.13: "All such leaf nodes MUST reference existing nodes
// or leaf or leaf-list nodes with their default value in use (see Sections
// 7.6.1 and 7.7.2) for the data to be valid."
//
// checkInstanceIdentifierInstance checks that the node an
// instance-identifier value names exists.
func (w *referenceWalk) checkInstanceIdentifierInstance(path string, leaf *yang.Entry, yangType *yang.YangType, value string, frames []dataNode) error {
	if yangType.OptionalInstance {
		return nil
	}
	steps, ok := parseInstanceIdentifier(value)
	if !ok {
		return missingReferenceError(path, yangType, value, "not an instance-identifier: write /prefix:node[prefix:key='value']/prefix:leaf (RFC 7950 Section 14)")
	}
	schema, err := w.v.instanceSchema(leaf, steps)
	if err != nil {
		return missingReferenceError(path, yangType, value, err.Error())
	}
	exists, err := instanceExists(frames[0].data, schema, steps)
	if err != nil {
		return missingReferenceError(path, yangType, value, err.Error())
	}
	if exists {
		return nil
	}
	return missingReferenceError(path, yangType, value, "no node exists at "+value+": configure it first, or correct the path")
}

// instanceExists walks the data tree along an instance-identifier whose
// schema nodes instanceSchema resolved. A step through an absent
// non-presence container goes on into an empty node, and a last step at an
// absent leaf or leaf-list answers whether its default is in use (RFC 7950
// Section 7.6.1).
func instanceExists(root map[string]any, schema []*yang.Entry, steps []instanceStep) (bool, error) {
	data := root
	for i, step := range steps {
		node := schema[i]
		raw, present := data[node.Name]
		last := i == len(steps)-1
		if !present {
			if !last {
				if !absentContainerInUse(node, data) {
					return false, nil
				}
				data = nil
				continue
			}
			return absentLeafInUse(node, data, step), nil
		}
		switch {
		case node.IsList():
			entryData, found, err := selectListEntry(node, data, step)
			if err != nil {
				return false, err
			}
			if !found {
				return false, nil
			}
			data = entryData
		case node.IsLeafList():
			if !step.hasValue {
				return last, nil
			}
			return slices.Contains(leafListItems(raw), step.value), nil
		case node.IsLeaf():
			return last, nil
		default:
			children, ok := raw.(map[string]any)
			if !ok {
				return false, nil
			}
			data = children
		}
	}
	return true, nil
}

// absentLeafInUse reports whether an absent leaf or leaf-list stands in the
// data tree through its default: the default is in use, and for a
// leaf-list-predicate the default holds the value.
func absentLeafInUse(node *yang.Entry, data map[string]any, step instanceStep) bool {
	if !isLeafNode(node) {
		return false
	}
	if !defaultInUse(node, data) {
		return false
	}
	defaults := canonicalDefaults(node)
	if !step.hasValue {
		return len(defaults) > 0
	}
	want := canonicalValue(node.Type, step.value)
	for _, def := range defaults {
		if canonicalValue(node.Type, def) == want {
			return true
		}
	}
	return false
}

// selectListEntry answers the entry of list, a child of container, that an
// instance-identifier step selects: by its keys, read by the entry's map key,
// or by position for a list with none, counted in the order the data
// delivers (configorder). It answers false for no such entry, and an error when
// the data carries no order to count a position in.
func selectListEntry(list *yang.Entry, container map[string]any, step instanceStep) (map[string]any, bool, error) {
	if step.pos > 0 {
		entries, err := configorder.Entries(container, list.Name, "")
		if err != nil {
			var tb textbuf.Buffer
			return nil, false, refError(tb.Str("the entry order of list ").Quoted(list.Name).Str(" is not known, so position ").Int(int64(step.pos)).Str(" selects nothing: ").Str(err.Error()))
		}
		if step.pos > len(entries) {
			return nil, false, nil
		}
		return entries[step.pos-1].Map, true, nil
	}
	entries, ok := container[list.Name].(map[string]any)
	if !ok {
		return nil, false, nil
	}
	key := listEntryAddress(list, step.keys)
	entryData, ok := entries[key].(map[string]any)
	if !ok {
		return nil, false, nil
	}
	if !instanceKeysMatch(dataNode{entries: []*yang.Entry{list}, data: entryData, key: key}, step.keys) {
		return nil, false, nil
	}
	return entryData, true, nil
}

// listEntryAddress answers the map key of the list entry the key predicates
// name: the key value for a single-key list, else the canonical address
// "<first> <key2> <value2> ..." (docs/architecture/config/syntax.md).
func listEntryAddress(list *yang.Entry, keys []instanceKey) string {
	values := make(map[string]string, len(keys))
	for _, key := range keys {
		values[localName(key.node)] = key.value
	}
	var tb textbuf.Buffer
	for i, name := range listKeys(list) {
		if i > 0 {
			tb.Byte(' ').Str(name).Byte(' ')
		}
		tb.Str(values[name])
	}
	return tb.String()
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

// missingReferenceError is the ValidationError for a reference whose
// instance the data tree does not hold.
func missingReferenceError(path string, yangType *yang.YangType, value, message string) error {
	return &ValidationError{
		Path:     path,
		Type:     ErrTypeReference,
		Message:  message,
		Expected: yangType.Name,
		Got:      value,
	}
}
