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

// namespaceRoot answers the entry root of module namespace, among the
// modules loaded with leaf, and nil when none of them is called namespace.
// A node is in the namespace of the module that instantiates it, which for a
// node another module augments in is the augmenting module, not the module
// holding the augment target (RFC 7950 Section 4.2.8).
func namespaceRoot(leaf *yang.Entry, namespace string) *yang.Entry {
	mod := leaf.Modules().Modules[namespace]
	if mod == nil {
		return nil
	}
	return yang.ToEntry(mod)
}

// leafNamespaceRoot answers the entry root of leaf's namespace module
// (namespaceRoot), and nil when that module is not known: a path from leaf
// then resolves to no node, and the leafref check refuses it naming the
// cause (newPathContext).
func leafNamespaceRoot(leaf *yang.Entry) *yang.Entry {
	namespace, err := leaf.InstantiatingModule()
	if err != nil {
		return nil
	}
	return namespaceRoot(leaf, namespace)
}

// topLevel answers every top-level data node called name that Ze's config
// data holds under one key: the node under own, the root of the referring
// node's namespace when there is one, and that of every conf module. Ze's
// config text carries no namespace, so a section several conf modules
// contribute to is one data node (see ValidateTreeAllModules), and the
// reference walk reads it through every schema node it stands for. Path
// resolution never uses this merged set: it looks an unprefixed name up in
// the referring node's namespace alone (topLevelParents). Without a schema,
// only own answers.
func (v *Validator) topLevel(own *yang.Entry, name string) []*yang.Entry {
	var found []*yang.Entry
	if own != nil {
		if child := schemaChild(own, name); child != nil {
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
	ctx, err := newPathContext(leaf, typ)
	if err != nil {
		return nil, path, refError(tb.Str("leafref path ").Quoted(typ.Path).Str(": ").Str(err.Error()))
	}
	parents, fromRoot, err := pathStart(ctx, path)
	if err != nil {
		return nil, path, err
	}
	var node *yang.Entry
	for i, step := range path.steps {
		node = childOf(parents, localName(step.node))
		if node == nil {
			tb.Str("leafref path ").Quoted(typ.Path).Str(" names no node ").Quoted(step.node)
			if fix := rootPathFix(ctx, path.steps, i); fromRoot && fix != "" {
				return nil, path, refError(tb.Str(fix))
			}
			return nil, path, refError(tb.Str(": correct the path to name an existing leaf"))
		}
		if err := checkPathPrefix(ctx, step.node, node); err != nil {
			return nil, path, refError(tb.Str("leafref path ").Quoted(typ.Path).Str(": ").Str(err.Error()))
		}
		if err := checkStepNamespace(ctx, step.node, node); err != nil {
			return nil, path, refError(tb.Str("leafref path ").Quoted(typ.Path).Str(": ").Str(err.Error()))
		}
		if err := checkPathKeys(ctx, typ.Path, node, step); err != nil {
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
// fromRoot reports that the first step is read at the top level, either way.
func pathStart(ctx *pathContext, path leafrefPath) (parents []*yang.Entry, fromRoot bool, err error) {
	if path.up == 0 {
		return topLevelParents(ctx, path.steps[0].node), true, nil
	}
	node := ctx.leaf
	for range path.up {
		node = dataParent(node)
		if node == nil {
			var tb textbuf.Buffer
			return nil, false, refError(tb.Str("leafref path climbs ").Int(int64(path.up)).Str(" levels, above the module root: remove a \"../\""))
		}
	}
	if node.Parent == nil {
		return topLevelParents(ctx, path.steps[0].node), true, nil
	}
	return []*yang.Entry{node}, false, nil
}

// RFC 7950 Section 6.4.1: "The root node has all top-level configuration
// data nodes in all modules as children."
//
// topLevelParents answers the module roots whose top level holds nodeID, so
// childOf finds the first step in any of them. A prefixed nodeID names one
// module, resolved in the module the path is written in, and the root of
// that module answers when it holds the node, wherever it sits among the
// loaded modules. When it does not, or the prefix is not declared, every
// loaded module whose top level holds the local name answers, so the step
// resolves and checkPathPrefix refuses the prefix with the fix. An
// unprefixed nodeID belongs to the namespace of the referring node
// (ctx.namespace, Section 6.4.1), so only that module's root answers, and
// only when its top level holds the node: for a leaf another module augments
// in, that is the augmenting module, never the module holding the augment
// target. A name no root answers for is refused by the caller, naming the
// prefixed path where another module holds it (rootPathFix).
func topLevelParents(ctx *pathContext, nodeID string) []*yang.Entry {
	name := localName(nodeID)
	if nodePrefix(nodeID) != "" {
		if root := prefixedModuleRoot(ctx, nodeID); root != nil {
			if schemaChild(root, name) != nil {
				return []*yang.Entry{root}
			}
		}
		return loadedTopLevelParents(ctx.leaf, name)
	}
	root := namespaceRoot(ctx.leaf, ctx.namespace)
	if root == nil {
		return nil
	}
	node := schemaChild(root, name)
	if node == nil {
		return nil
	}
	return []*yang.Entry{dataParentOrRoot(node)}
}

// prefixedModuleRoot answers the entry root of the module the prefix of
// nodeID names in the module the path is written in, and nil for an
// unprefixed nodeID, an undeclared prefix, or a path whose statement is not
// known. A submodule's prefix names the module it belongs to, whose entry
// holds the submodule's nodes.
func prefixedModuleRoot(ctx *pathContext, nodeID string) *yang.Entry {
	prefix := nodePrefix(nodeID)
	if prefix == "" {
		return nil
	}
	if ctx.stmt == nil {
		return nil
	}
	mod := yang.FindModuleByPrefix(ctx.stmt, prefix)
	if mod == nil {
		return nil
	}
	if mod.BelongsTo != nil {
		mod = mod.Modules.Modules[mod.BelongsTo.Name]
		if mod == nil {
			return nil
		}
	}
	return yang.ToEntry(mod)
}

// loadedTopLevelParents answers the root of every loaded module whose top
// level holds a data node called name, in module name order so the first
// match is the same on every load. A module goyang lists under both its name
// and its name with a revision answers once.
func loadedTopLevelParents(leaf *yang.Entry, name string) []*yang.Entry {
	modules := leaf.Modules().Modules
	names := make([]string, 0, len(modules))
	for modName := range modules {
		names = append(names, modName)
	}
	slices.Sort(names)
	seen := make(map[*yang.Module]bool, len(modules))
	var parents []*yang.Entry
	for _, modName := range names {
		mod := modules[modName]
		if seen[mod] {
			continue
		}
		seen[mod] = true
		if child := schemaChild(yang.ToEntry(mod), name); child != nil {
			parents = append(parents, dataParentOrRoot(child))
		}
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
//
// An unprefixed key name takes the referring node's namespace
// (checkStepNamespace).
func checkPathKeys(ctx *pathContext, text string, node *yang.Entry, step pathStep) error {
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
		keyLeaf := schemaChild(node, name)
		if err := checkPathPrefix(ctx, key.node, keyLeaf); err != nil {
			return refError(tb.Str("leafref path ").Quoted(text).Str(": ").Str(err.Error()))
		}
		if keyLeaf == nil {
			return refError(tb.Str("leafref path ").Quoted(text).Str(" tests key ").Quoted(name).Str(", which list ").Quoted(node.Name).Str(" names but does not define: correct the list's key statement"))
		}
		if err := checkStepNamespace(ctx, key.node, keyLeaf); err != nil {
			return refError(tb.Str("leafref path ").Quoted(text).Str(": ").Str(err.Error()))
		}
		if _, err := keyExprTarget(ctx, key); err != nil {
			return refError(tb.Str("leafref path ").Quoted(text).Str(": ").Str(err.Error()))
		}
	}
	return nil
}

// keyExprTarget resolves a predicate's "current()/../x" to the leaf it reads.
func keyExprTarget(ctx *pathContext, key pathKey) (*yang.Entry, error) {
	var tb textbuf.Buffer
	node := ctx.leaf
	for range key.up {
		node = dataParent(node)
		if node == nil {
			return nil, refError(tb.Str("current() climbs above the module root"))
		}
	}
	parents := []*yang.Entry{node}
	if node.Parent == nil {
		parents = topLevelParents(ctx, key.nodes[0])
	}
	var target *yang.Entry
	for _, nodeID := range key.nodes {
		target = childOf(parents, localName(nodeID))
		if target == nil {
			return nil, refError(tb.Str("current() expression names no node ").Quoted(nodeID))
		}
		if err := checkPathPrefix(ctx, nodeID, target); err != nil {
			return nil, err
		}
		if err := checkStepNamespace(ctx, nodeID, target); err != nil {
			return nil, err
		}
		parents = []*yang.Entry{target}
	}
	if !target.IsLeaf() {
		return nil, refError(tb.Str("current() expression ends at ").Quoted(target.Name).Str(", which is not a leaf"))
	}
	return target, nil
}

// pathContext is what a leafref path is resolved against: leaf, the
// referring node, is the context node every relative step and current()
// climbs from (RFC 7950 Section 9.9.2), and stmt is the type statement the
// path is written in, whose module declares the prefixes the path may use.
// The two differ when the path comes from a typedef of another module or a
// deviation, and stmt is nil when Ze cannot find that statement. namespace is
// the module of leaf, the namespace an unprefixed name of the path takes
// (RFC 7950 Section 6.4.1).
type pathContext struct {
	leaf      *yang.Entry
	stmt      *yang.Type
	namespace string
}

// newPathContext builds the context a leafref typ of leaf is resolved
// against. goyang gives a node a grouping instantiates the namespace of the
// module using the grouping, which is the namespace Section 6.4.1 names.
func newPathContext(leaf *yang.Entry, typ *yang.YangType) (*pathContext, error) {
	namespace, err := leaf.InstantiatingModule()
	if err != nil {
		var tb textbuf.Buffer
		return nil, refError(tb.Str("the module of the referring leaf ").Str(leaf.Path()).Str(" is not known: ").Str(err.Error()))
	}
	return &pathContext{leaf: leaf, stmt: pathStatement(leaf, typ), namespace: namespace}, nil
}

// pathStatement answers the type statement that writes the path of typ, one
// leafref type of leaf: the statement holding the path substatement, reached
// from leaf's own type statement through union members and the typedefs each
// names, or, when a deviation gave leaf its type, from the deviate's type
// statement. It answers nil when neither holds typ.
func pathStatement(leaf *yang.Entry, typ *yang.YangType) *yang.Type {
	var stmt *yang.Type
	switch node := leaf.Node.(type) {
	case *yang.Leaf:
		stmt = node.Type
	case *yang.LeafList:
		stmt = node.Type
	}
	if found := findPathStatement(stmt, typ); found != nil {
		return found
	}
	return deviationStatement(leaf, typ)
}

// RFC 7950 Section 7.20.3.2: "The argument "replace" replaces properties of
// the target node."
//
// deviationStatement answers the statement that writes the path of typ under
// the type substatement of a deviate in any loaded module or submodule.
// goyang copies a deviate's type onto the target entry, so the deviate that
// gave leaf typ is found by identity, and its module is the one the path is
// written in. goyang copies the type of an "add" deviate as it does a
// "replace" one, so both are searched: the path is resolved in the module
// that wrote it, whichever keyword carried it.
func deviationStatement(leaf *yang.Entry, typ *yang.YangType) *yang.Type {
	modules := leaf.Modules()
	for _, set := range []map[string]*yang.Module{modules.Modules, modules.SubModules} {
		for _, mod := range set {
			if found := moduleDeviationStatement(mod, typ); found != nil {
				return found
			}
		}
	}
	return nil
}

// moduleDeviationStatement answers the statement under one of mod's deviate
// type substatements that writes the path of typ.
func moduleDeviationStatement(mod *yang.Module, typ *yang.YangType) *yang.Type {
	for _, deviation := range mod.Deviation {
		for _, deviate := range deviation.Deviate {
			if found := findPathStatement(deviate.Type, typ); found != nil {
				return found
			}
		}
	}
	return nil
}

// findPathStatement answers the statement under t that writes the path of
// typ. goyang shares a union member's YangType between the union statement
// and every type that copies it, so typ is found by identity. The recursion
// is bounded by the union nesting and typedef chain of the loaded modules,
// which goyang resolved without a loop.
func findPathStatement(t *yang.Type, typ *yang.YangType) *yang.Type {
	if t == nil {
		return nil
	}
	if t.YangType == typ {
		return pathDefinition(t)
	}
	for _, member := range t.Type {
		if found := findPathStatement(member, typ); found != nil {
			return found
		}
	}
	return findPathStatement(typedefStatement(t), typ)
}

// pathDefinition answers the statement that holds the path t's type
// carries: t itself when it writes one, otherwise the type statement of the
// typedef t names, followed down the typedef chain.
func pathDefinition(t *yang.Type) *yang.Type {
	for t != nil && t.Path == nil {
		t = typedefStatement(t)
	}
	return t
}

// typedefStatement answers the type statement of the typedef t names, and
// nil when t names a built-in type.
func typedefStatement(t *yang.Type) *yang.Type {
	if t.YangType == nil {
		return nil
	}
	base := t.YangType.Base
	if base == t {
		return nil
	}
	return base
}

// RFC 7950 Section 6.4: "The XPath expressions MUST be syntactically
// correct, and all prefixes used MUST be present in the XPath context (see
// Section 6.4.1)."
// RFC 7950 Section 6.4.1: "The set of namespace declarations is the set of
// all "import" statements' prefix and namespace pairs in the module where the
// XPath expression is specified, and the "prefix" statement's prefix for the
// "namespace" statement's URI."
//
// checkPathPrefix checks the prefix of nodeID, one node-identifier of a
// leafref path, against node, the schema node its local name matched: the
// prefix is declared in the module the path is written in (ctx.stmt), which
// for a typedef is the module defining it, and names the module whose
// namespace holds node. An unprefixed name is checkStepNamespace's.
func checkPathPrefix(ctx *pathContext, nodeID string, node *yang.Entry) error {
	prefix := nodePrefix(nodeID)
	if prefix == "" {
		return nil
	}
	var tb textbuf.Buffer
	if node == nil {
		return refError(tb.Str("names no node ").Quoted(nodeID))
	}
	if ctx.stmt == nil {
		return noPathStatement(ctx, nodeID)
	}
	owner, err := node.InstantiatingModule()
	if err != nil {
		return refError(tb.Str("the module of ").Quoted(node.Name).Str(" is not known: ").Str(err.Error()))
	}
	declared := yang.FindModuleByPrefix(ctx.stmt, prefix)
	if declared == nil {
		tb.Str("prefix ").Quoted(prefix).Str(" of ").Quoted(nodeID).Str(" is neither the prefix of module ").Str(belongingModule(yang.RootNode(ctx.stmt))).Str(", where the path is written, nor one it imports: ")
		return refError(prefixFix(&tb, ctx, owner, localName(nodeID)))
	}
	declaredName := belongingModule(declared)
	if declaredName == owner {
		return nil
	}
	tb.Str("prefix ").Quoted(prefix).Str(" names module ").Str(declaredName).Str(", but ").Quoted(node.Name).Str(" belongs to module ").Str(owner).Str(": ")
	return refError(prefixFix(&tb, ctx, owner, localName(nodeID)))
}

// noPathStatement refuses nodeID because the module the path is written in,
// which declares the prefixes the path may use, is not known. pathStatement
// searches every statement goyang takes a leaf's type from, so this answers
// only when goyang gained a source Ze does not search.
func noPathStatement(ctx *pathContext, nodeID string) error {
	var tb textbuf.Buffer
	return refError(tb.Str("Ze found no type statement writing the path of ").Str(ctx.leaf.Path()).
		Str(" in the leaf's type, the typedefs it names, or a deviate of a loaded module, so ").Quoted(nodeID).
		Str(" cannot be resolved: Ze's search for the statement is incomplete, report it with the module text"))
}

// RFC 7950 Section 6.4.1: "Names without a namespace prefix belong to the
// same namespace as the identifier of the current node."
//
// checkStepNamespace checks an unprefixed nodeID of a leafref path against
// node, the schema node its local name matched: node must be in
// ctx.namespace, the referring node's module, which every unprefixed name of
// the path takes. A prefixed nodeID is checkPathPrefix's. Ze merges a
// section several conf modules contribute to into one config data node, and
// that changes no name's namespace: a path into another module's section
// writes that module's prefix.
func checkStepNamespace(ctx *pathContext, nodeID string, node *yang.Entry) error {
	if nodePrefix(nodeID) != "" {
		return nil
	}
	var tb textbuf.Buffer
	owner, err := node.InstantiatingModule()
	if err != nil {
		return refError(tb.Str("the module of ").Quoted(node.Name).Str(" is not known: ").Str(err.Error()))
	}
	if owner == ctx.namespace {
		return nil
	}
	if ctx.stmt == nil {
		return noPathStatement(ctx, nodeID)
	}
	tb.Quoted(nodeID).Str(" has no prefix, so it names module ").Str(ctx.namespace).
		Str(" (RFC 7950 Section 6.4.1), but ").Quoted(node.Name).Str(" belongs to module ").Str(owner).Str(": ")
	return refError(prefixFix(&tb, ctx, owner, nodeID))
}

// rootPathFix answers the fix for the steps of a path read from the top
// level, where step failed names no node: when another loaded module's top
// level holds every step by its local name, or else the steps up to failed,
// that module's path, each step prefixed as the module the path is written
// in declares it, and the import to add where it declares none. It answers
// "" when every step up to failed is prefixed, so no unprefixed name was
// held to the referring node's namespace, when no other module holds them,
// when a step carries a predicate, whose
// current() expression Ze does not rewrite, or when the written module is
// not known.
func rootPathFix(ctx *pathContext, steps []pathStep, failed int) string {
	if !slices.ContainsFunc(steps[:failed+1], func(step pathStep) bool { return nodePrefix(step.node) == "" }) {
		return ""
	}
	if fix := rootStepsFix(ctx, steps); fix != "" {
		return fix
	}
	return rootStepsFix(ctx, steps[:failed+1])
}

// rootStepsFix answers rootPathFix's fix for exactly steps.
func rootStepsFix(ctx *pathContext, steps []pathStep) string {
	if ctx.stmt == nil {
		return ""
	}
	for _, step := range steps {
		if len(step.keys) > 0 {
			return ""
		}
	}
	written := yang.RootNode(ctx.stmt)
	for _, parent := range loadedTopLevelParents(ctx.leaf, localName(steps[0].node)) {
		nodes := localChain(parent, steps)
		if nodes == nil {
			continue
		}
		var path textbuf.Buffer
		missing, holder := "", ""
		for _, node := range nodes {
			owner, err := node.InstantiatingModule()
			if err != nil {
				return ""
			}
			if holder == "" {
				holder = owner
			}
			prefix, imported := writtenPrefix(written, owner)
			if !imported && missing == "" {
				missing = "import " + owner + " { prefix " + prefix + "; }"
			}
			path.Byte('/').Str(prefix).Byte(':').Str(node.Name)
		}
		var tb textbuf.Buffer
		tb.Str(": an unprefixed name names a node of module ").Str(ctx.namespace).
			Str(" (RFC 7950 Section 6.4.1), and module ").Str(holder).Str(" holds this path: ")
		if missing != "" {
			tb.Str("add ").Quoted(missing).Str(" to module ").Str(belongingModule(written)).Str(" and ")
		}
		return tb.Str("write ").Quoted(path.String()).String()
	}
	return ""
}

// localChain answers the schema nodes steps name by their local names, the
// first under parent, and nil when a step names no node.
func localChain(parent *yang.Entry, steps []pathStep) []*yang.Entry {
	nodes := make([]*yang.Entry, 0, len(steps))
	for _, step := range steps {
		node := schemaChild(parent, localName(step.node))
		if node == nil {
			return nil
		}
		nodes = append(nodes, node)
		parent = node
	}
	return nodes
}

// writtenPrefix answers the prefix written, the module a path is written in,
// names module owner by: its own prefix, or the one it imports owner under.
// When written does not import owner, it answers owner's own prefix and
// false, so the caller names the import to add.
func writtenPrefix(written *yang.Module, owner string) (string, bool) {
	if belongingModule(written) == owner {
		return written.GetPrefix(), true
	}
	for _, imp := range written.Import {
		if imp.Name == owner {
			return imp.Prefix.Name, true
		}
	}
	if mod := written.Modules.Modules[owner]; mod != nil {
		return mod.GetPrefix(), false
	}
	return owner, false
}

// RFC 7950 Section 6.4.1: "Names without a namespace prefix belong to the
// same namespace as the identifier of the current node.  Inside a grouping,
// that namespace is affected by where the grouping is used (see Section
// 7.13)."
//
// prefixFix writes to tb the fix that names local in module owner from the
// module the path is written in. Inside a grouping used in owner, that is the
// bare name, because a prefix there names the module defining the grouping.
// Otherwise it is the written module's own prefix, or the prefix it imports
// owner under, and where it imports owner under none, the import to add.
func prefixFix(tb *textbuf.Buffer, ctx *pathContext, owner, local string) *textbuf.Buffer {
	written := yang.RootNode(ctx.stmt)
	writtenName := belongingModule(written)
	if writtenName == owner {
		return tb.Str("write ").Quoted(written.GetPrefix() + ":" + local)
	}
	if usedInOwner(ctx, owner) {
		return tb.Str("the path is written in a grouping of module ").Str(writtenName).Str(" used in module ").Str(owner).Str(", where an unprefixed name takes the namespace of the using module: write ").Quoted(local).Str(" with no prefix")
	}
	ownerPrefix, imported := writtenPrefix(written, owner)
	if imported {
		return tb.Str("write ").Quoted(ownerPrefix + ":" + local)
	}
	return tb.Str("module ").Str(writtenName).Str(" does not import ").Str(owner).Str(": add ").Quoted("import " + owner + " { prefix " + ownerPrefix + "; }").Str(" to module ").Str(writtenName).Str(" and write ").Quoted(ownerPrefix + ":" + local)
}

// usedInOwner reports whether the path is written inside a grouping that the
// referring leaf was instantiated from in module owner.
func usedInOwner(ctx *pathContext, owner string) bool {
	leafModule, err := ctx.leaf.InstantiatingModule()
	if err != nil {
		return false
	}
	if leafModule != owner {
		return false
	}
	for node := ctx.stmt.ParentNode(); node != nil; node = node.ParentNode() {
		if _, ok := node.(*yang.Grouping); ok {
			return true
		}
	}
	return false
}

// belongingModule answers the name of the module whose namespace mod's
// definitions carry: its own, or for a submodule the module it belongs to.
func belongingModule(mod *yang.Module) string {
	if mod.BelongsTo != nil {
		return mod.BelongsTo.Name
	}
	return mod.Name
}

// referenceError is a refusal of a reference type's path or value, before
// the caller adds the config path it was found at.
type referenceError struct{ message string }

func (e *referenceError) Error() string { return e.message }

// refError builds a referenceError from tb.
func refError(tb *textbuf.Buffer) error {
	return &referenceError{message: tb.String()}
}
