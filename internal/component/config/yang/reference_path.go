// Design: docs/architecture/config/yang-config-design.md — YANG schema handling
// Related: loader_grammar.go — parsePathArg fills leafrefPath
// Related: validator_reference.go — the leafref, identityref and instance-identifier checks
// RFC: rfc/short/rfc7950.md -- Sections 9.9, 9.13 and 14, reference paths

package yang

import (
	"slices"
	"strings"

	"github.com/openconfig/goyang/pkg/yang"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// leafrefPath is a parsed leafref path-arg. up is the count of "../" before
// the first step of a relative path, and zero for an absolute path, which
// the grammar never lets open with "../".
type leafrefPath struct {
	up    int
	steps []pathStep
}

// pathStep is one node-identifier of a path, with the predicates that select
// list entries by key.
type pathStep struct {
	node string // node-identifier as written: "name" or "prefix:name"
	keys []pathKey
}

// pathKey is one path-predicate: "[node = current()/../x/y]". up counts the
// ".." after current(), nodes are the node-identifiers after them.
type pathKey struct {
	node  string
	up    int
	nodes []string
}

// localName strips the prefix from a node-identifier. Ze's config data is
// keyed by local names alone, so a step matches a node by its local name.
func localName(nodeID string) string {
	if _, local, found := strings.Cut(nodeID, ":"); found {
		return local
	}
	return nodeID
}

// nodePrefix answers the prefix of a node-identifier, "" when it has none.
func nodePrefix(nodeID string) string {
	if prefix, _, found := strings.Cut(nodeID, ":"); found {
		return prefix
	}
	return ""
}

// isSchemaGroup reports whether e is a choice or a case. Neither is a data
// node, so a path step passes through them (RFC 7950 Section 6.5: a schema
// node identifier in a data path names data nodes only).
func isSchemaGroup(e *yang.Entry) bool {
	if e.IsChoice() {
		return true
	}
	return e.IsCase()
}

// schemaChild answers the data node called name under parent, looking
// through choice and case nodes. The recursion is bounded by the nesting depth
// of the loaded modules, which Ze builds from its own sources.
func schemaChild(parent *yang.Entry, name string) *yang.Entry {
	if child, ok := parent.Dir[name]; ok && !isSchemaGroup(child) {
		return child
	}
	for _, child := range parent.Dir {
		if !isSchemaGroup(child) {
			continue
		}
		if found := schemaChild(child, name); found != nil {
			return found
		}
	}
	return nil
}

// dataParent answers the data node above e, past any choice or case, and nil
// at a module root.
func dataParent(e *yang.Entry) *yang.Entry {
	parent := e.Parent
	for parent != nil && isSchemaGroup(parent) {
		parent = parent.Parent
	}
	return parent
}

// moduleRoot answers the module entry that holds e.
func moduleRoot(e *yang.Entry) *yang.Entry {
	for e.Parent != nil {
		e = e.Parent
	}
	return e
}

// topLevel answers every top-level data node called name. Ze merges a
// section several conf modules contribute to into one data node (see
// ValidateTreeAllModules), so a path's first step may stand for several
// schema nodes. Without a schema, as when a default is checked at schema
// build, only the module holding leaf answers.
func (v *Validator) topLevel(leaf *yang.Entry, name string) []*yang.Entry {
	var found []*yang.Entry
	if leaf != nil {
		if child := schemaChild(moduleRoot(leaf), name); child != nil {
			found = append(found, child)
		}
	}
	if v.schema == nil {
		return found
	}
	for _, modName := range v.schema.ConfModuleNames() {
		root := v.schema.GetEntry(modName)
		if root == nil {
			continue
		}
		child := schemaChild(root, name)
		if child == nil {
			continue
		}
		if len(found) > 0 && found[0] == child {
			continue
		}
		found = append(found, child)
	}
	return found
}

// childOf answers the data node called name under any of parents.
func childOf(parents []*yang.Entry, name string) *yang.Entry {
	for _, parent := range parents {
		if child := schemaChild(parent, name); child != nil {
			return child
		}
	}
	return nil
}

// isLeafNode reports whether e is a leaf or a leaf-list.
func isLeafNode(e *yang.Entry) bool {
	if e.IsLeaf() {
		return true
	}
	return e.IsLeafList()
}

// listKeys answers the key leaf names of a list, in key statement order.
func listKeys(list *yang.Entry) []string {
	return strings.Fields(list.Key)
}

// leafrefTarget resolves the schema node a leafref of leaf refers to.
//
// RFC 7950 Section 9.9.2: "The "path" statement, which is a substatement to
// the "type" statement, MUST be present if the type is "leafref".  It takes
// as an argument a string that MUST refer to a leaf or leaf-list node." The
// path is parsed here rather than kept from load, so the check reads the
// type it is given.
func (v *Validator) leafrefTarget(leaf *yang.Entry, typ *yang.YangType) (*yang.Entry, leafrefPath, error) {
	var tb textbuf.Buffer
	if typ.Path == "" {
		return nil, leafrefPath{}, refError(tb.Str("leafref has no path statement: add path \"<path to the referred leaf>\" to the type"))
	}
	path, err := parsePathArg(typ.Path)
	if err != nil {
		return nil, leafrefPath{}, refError(tb.Str("leafref path ").Quoted(typ.Path).Str(" does not parse: ").Str(err.Error()))
	}
	parents, err := v.pathStart(leaf, path)
	if err != nil {
		return nil, path, err
	}
	var node *yang.Entry
	for i, step := range path.steps {
		node = childOf(parents, localName(step.node))
		if node == nil {
			return nil, path, refError(tb.Str("leafref path ").Quoted(typ.Path).Str(" names no node ").Quoted(step.node).Str(": correct the path to name an existing leaf"))
		}
		if err := v.checkPathKeys(leaf, typ.Path, node, step); err != nil {
			return nil, path, err
		}
		if i < len(path.steps)-1 {
			parents = []*yang.Entry{node}
		}
	}
	if !isLeafNode(node) {
		return nil, path, refError(tb.Str("leafref path ").Quoted(typ.Path).Str(" ends at ").Quoted(node.Name).Str(", which is not a leaf or leaf-list: extend the path to a leaf"))
	}
	return node, path, nil
}

// pathStart answers the schema nodes a path's first step is looked up
// under: the top level for an absolute path, the node "../" climbs to for a
// relative one. The context node of the path is leaf itself (Section 9.9.2).
func (v *Validator) pathStart(leaf *yang.Entry, path leafrefPath) ([]*yang.Entry, error) {
	if path.up == 0 {
		return v.topLevelParents(leaf, localName(path.steps[0].node)), nil
	}
	node := leaf
	for range path.up {
		node = dataParent(node)
		if node == nil {
			var tb textbuf.Buffer
			return nil, refError(tb.Str("leafref path climbs ").Int(int64(path.up)).Str(" levels, above the module root: remove a \"../\""))
		}
	}
	if node.Parent == nil {
		return v.topLevelParents(leaf, localName(path.steps[0].node)), nil
	}
	return []*yang.Entry{node}, nil
}

// topLevelParents answers the module roots whose top level holds name, so
// childOf finds the first step in any of them.
func (v *Validator) topLevelParents(leaf *yang.Entry, name string) []*yang.Entry {
	nodes := v.topLevel(leaf, name)
	parents := make([]*yang.Entry, 0, len(nodes))
	for _, node := range nodes {
		parents = append(parents, dataParentOrRoot(node))
	}
	return parents
}

// dataParentOrRoot answers the entry whose Dir holds node: its parent, which
// for a top-level node is the module root.
func dataParentOrRoot(node *yang.Entry) *yang.Entry {
	if parent := dataParent(node); parent != nil {
		return parent
	}
	return node.Parent
}

// checkPathKeys checks a step's predicates: the step is a list, each key
// predicate names one of its keys, and its current() expression reaches a
// leaf.
//
// RFC 7950 Section 9.9.2: "Predicates are used only for constraining the
// values for the key nodes for list entries.  Each predicate consists of
// exactly one equality test per key".
func (v *Validator) checkPathKeys(leaf *yang.Entry, text string, node *yang.Entry, step pathStep) error {
	var tb textbuf.Buffer
	if len(step.keys) == 0 {
		return nil
	}
	if !node.IsList() {
		return refError(tb.Str("leafref path ").Quoted(text).Str(" puts a predicate on ").Quoted(node.Name).Str(", which is not a list: remove the predicate"))
	}
	seen := make(map[string]bool, len(step.keys))
	for _, key := range step.keys {
		name := localName(key.node)
		if !slices.Contains(listKeys(node), name) {
			return refError(tb.Str("leafref path ").Quoted(text).Str(" tests ").Quoted(name).Str(", which is not a key of list ").Quoted(node.Name).Str(": test only the keys ").Quoted(node.Key))
		}
		if seen[name] {
			return refError(tb.Str("leafref path ").Quoted(text).Str(" tests key ").Quoted(name).Str(" twice: give one equality test per key"))
		}
		seen[name] = true
		if _, err := v.keyExprTarget(leaf, key); err != nil {
			return refError(tb.Str("leafref path ").Quoted(text).Str(": ").Str(err.Error()))
		}
	}
	return nil
}

// keyExprTarget resolves a predicate's "current()/../x" to the leaf it reads.
func (v *Validator) keyExprTarget(leaf *yang.Entry, key pathKey) (*yang.Entry, error) {
	var tb textbuf.Buffer
	node := leaf
	for range key.up {
		node = dataParent(node)
		if node == nil {
			return nil, refError(tb.Str("current() climbs above the module root"))
		}
	}
	parents := []*yang.Entry{node}
	if node.Parent == nil {
		parents = v.topLevelParents(leaf, localName(key.nodes[0]))
	}
	var target *yang.Entry
	for _, nodeID := range key.nodes {
		target = childOf(parents, localName(nodeID))
		if target == nil {
			return nil, refError(tb.Str("current() expression names no node ").Quoted(nodeID))
		}
		parents = []*yang.Entry{target}
	}
	if !target.IsLeaf() {
		return nil, refError(tb.Str("current() expression ends at ").Quoted(target.Name).Str(", which is not a leaf"))
	}
	return target, nil
}

// referenceError is a refusal of a reference type's path or value, before
// the caller adds the config path it was found at.
type referenceError struct{ message string }

func (e *referenceError) Error() string { return e.message }

// refError builds a referenceError from tb.
func refError(tb *textbuf.Buffer) error {
	return &referenceError{message: tb.String()}
}
