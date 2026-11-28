// Design: docs/architecture/config/yang-config-design.md — YANG schema handling
// Related: loader.go — Resolve and DefaultLoader join these checks
// RFC: rfc/short/rfc7950.md -- Sections 7.2.2, 7.12, 7.18.2, 7.19, 9.4.4 and 9.6.4, structures goyang accepts
package yang

import (
	"cmp"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/openconfig/goyang/pkg/yang"
)

// ErrCyclicGrouping marks a `grouping` that references itself directly or
// indirectly through `uses` or nested `grouping` statements (RFC 7950 Section
// 7.12). goyang's ToEntry recurses before caching, so any cycle overflows the
// goroutine stack unless refused before Modules.Process runs.
var ErrCyclicGrouping = errors.New("cyclic YANG grouping")

// ErrCyclicIdentity marks an `identity` whose `base` chain reaches itself
// directly or indirectly (RFC 7950 Section 7.18.2). goyang's addChildren
// recurses over derived identities with no visited guard, so any cycle
// overflows the goroutine stack unless refused before Modules.Process runs.
var ErrCyclicIdentity = errors.New("cyclic YANG identity")

// ErrLengthOrder marks a `length` statement whose parts overlap or descend.
// goyang sorts and coalesces the parts before it validates them (types_builtin.go,
// parseChildRanges: "The output range is sorted and coalesced"), so the fault
// never reaches its own check.
var ErrLengthOrder = errors.New("YANG length parts not disjoint and ascending")

// ErrEnumRestriction marks an `enum` statement in a restricted enumeration
// that names no assigned name of the base type, or changes its value. goyang
// builds the restricted type's values afresh from the restricting statements
// and never compares them with the base type.
var ErrEnumRestriction = errors.New("YANG enum restriction departs from the base type")

// ErrEnumValue marks an enumeration whose assigned values break RFC 7950.
var ErrEnumValue = errors.New("YANG enum value invalid")

// ErrExtensionSubstatement marks a substatement under an extension statement
// that is not a YANG statement, that the block holding it does not admit, or
// whose argument, substatement counts or block break its Section 14 rule. goyang keeps an
// extension statement as raw text (ast.go, build: "it might be an
// extension"), so it never reads these substatements.
var ErrExtensionSubstatement = errors.New("invalid YANG statement under an extension")

// ErrIncludeBelongsTo marks an `include` of a submodule that belongs to a
// module other than the including module, or other than the module the
// including submodule belongs to. goyang finds an included submodule by its
// name alone (modules.go, FindModule: "we should check the BelongsTo field
// below?"), so it merges a foreign submodule's definitions without a word.
var ErrIncludeBelongsTo = errors.New("YANG submodule included outside the module it belongs to")

// checkStructure joins the structural checks goyang does not perform over
// every loaded module and submodule, module by module in sourceModules order.
func (l *Loader) checkStructure() error {
	mods := l.modules.sourceModules()
	errs := make([]error, 0, len(mods))
	for _, mod := range mods {
		errs = append(errs, moduleStructureErrors(l.modules.source(mod))...)
	}
	return errors.Join(errs...)
}

// moduleStructureErrors answers the structural refusals of one module or
// submodule, in the order a caller reads the joined error in: its includes,
// its lengths, its enumerations, then the statements under its extensions.
// The module's goyang AST is walked once (newModuleTypes), and the length and
// the enum check both read that one walk.
func moduleStructureErrors(source moduleSource) []error {
	mod := source.module
	types := newModuleTypes(mod)
	return slices.Concat(
		// RFC 7950 Section 7.2.2
		moduleIncludeErrors(mod),
		// RFC 7950 Section 9.4.4
		types.lengthErrors(),
		// RFC 7950 Sections 9.6.4 and 9.6.4.2
		types.enumErrors(),
		// RFC 7950 Section 7.19
		moduleExtensionSubstatementErrors(source),
	)
}

// moduleIncludeErrors returns one error for each `include` in mod, a module
// or a submodule, whose submodule belongs to another module than mod's own.
// An include goyang found no submodule for is its own refusal ("no such
// submodule"), so it has no belongs-to to compare here.
func moduleIncludeErrors(mod *yang.Module) []error {
	owner := mod.Name
	if mod.BelongsTo != nil {
		owner = mod.BelongsTo.Name
	}
	var errs []error
	for _, include := range mod.Include {
		submodule := includedSubmodule(mod, include)
		if submodule == nil {
			continue
		}
		if submodule.BelongsTo == nil {
			errs = append(errs, fmt.Errorf("%w: module %s: %s: include %s: submodule %s has no belongs-to statement;"+
				" add `belongs-to %s { prefix ...; }` to submodule %s",
				ErrIncludeBelongsTo, mod.Name, include.Source.Location(), include.Name, submodule.Name, owner, submodule.Name))
			continue
		}
		// RFC 7950 Section 7.2.2: "A submodule MUST only be included by either
		// the module to which it belongs or another submodule that belongs to
		// that module."
		if submodule.BelongsTo.Name == owner {
			continue
		}
		errs = append(errs, fmt.Errorf("%w: module %s: %s: include %s: submodule %s belongs to module %s, not %s,"+
			" so its definitions cannot join module %s (RFC 7950 Section 7.2.2);"+
			" remove this include, or include %s from module %s or one of its submodules",
			ErrIncludeBelongsTo, mod.Name, include.Source.Location(), include.Name, submodule.Name,
			submodule.BelongsTo.Name, owner, owner, submodule.Name, submodule.BelongsTo.Name))
	}
	return errs
}

// includedSubmodule answers the submodule include names: the one goyang bound
// while processing, else the one loaded under the include's revision or bare
// name, the two keys goyang stores a submodule under. It never reads a file,
// unlike goyang's FindModule, so a check cannot load a module the caller did
// not add.
func includedSubmodule(mod *yang.Module, include *yang.Include) *yang.Module {
	if include.Module != nil {
		return include.Module
	}
	if include.RevisionDate != nil {
		if submodule := mod.Modules.SubModules[include.Name+"@"+include.RevisionDate.Name]; submodule != nil {
			return submodule
		}
	}
	return mod.Modules.SubModules[include.Name]
}

// checkPreProcess runs pre-process guards over every loaded module and
// submodule before goyang's Modules.Process runs, refusing grouping and
// identity cycles that would otherwise overflow goyang's recursion stack.
// All cross-module and submodule lookups stay strictly in memory and never
// read files from disk.
func (l *Loader) checkPreProcess() error {
	mods := l.modules.heldModulesSorted()
	errs := slices.Concat(
		// RFC 7950 Section 7.12
		cyclicGroupingErrors(mods),
		// RFC 7950 Section 7.18.2
		cyclicIdentityErrors(mods),
	)
	return errors.Join(errs...)
}

// heldModulesSorted answers every distinct loaded module and submodule in
// deterministic order, deduplicated by pointer because goyang keys revisioned
// modules under both bare and name@revision keys.
func (m *sourcedModules) heldModulesSorted() []*yang.Module {
	held := m.held()
	mods := make([]*yang.Module, 0, len(held))
	for mod := range held {
		mods = append(mods, mod)
	}
	slices.SortFunc(mods, func(a, b *yang.Module) int {
		if order := cmp.Compare(a.FullName(), b.FullName()); order != 0 {
			return order
		}
		return cmp.Compare(yang.Source(a), yang.Source(b))
	})
	return mods
}

// groupingHolder is any goyang AST node that can declare local groupings.
type groupingHolder interface {
	Groupings() []*yang.Grouping
}

// cycleVisitState tracks iterative three-color DFS progress over a finite
// dependency graph: unvisited, on the active recursion stack, or fully explored.
type cycleVisitState uint8

const (
	cycleUnvisited cycleVisitState = iota
	cycleVisiting
	cycleDone
)

// groupingEdge is one directed dependency from a grouping to another grouping,
// either via a `uses` statement or via a nested `grouping` declaration that
// goyang's ToEntry eagerly converts.
type groupingEdge struct {
	target   *yang.Grouping
	location string
}

// groupingFrame is one active vertex on the iterative DFS stack in
// cyclicGroupingErrors.
type groupingFrame struct {
	node *yang.Grouping
	next int
}

// identityEdge is one directed base dependency from an identity to its base
// identity.
type identityEdge struct {
	target   *yang.Identity
	location string
}

// identityFrame is one active vertex on the iterative DFS stack in
// cyclicIdentityErrors.
type identityFrame struct {
	node *yang.Identity
	next int
}

// importedModule answers the module imp names from the in-memory module set,
// preferring an exact revision match when revision-date is given. It never
// reads a file from disk.
func importedModule(mod *yang.Module, imp *yang.Import) *yang.Module {
	if imp.Module != nil {
		return imp.Module
	}
	if mod.Modules == nil {
		return nil
	}
	if imp.RevisionDate != nil {
		if target := mod.Modules.Modules[imp.Name+"@"+imp.RevisionDate.Name]; target != nil {
			return target
		}
	}
	return mod.Modules.Modules[imp.Name]
}

// owningModule answers the main module for a submodule via its belongs-to
// statement when loaded in memory, or mod itself for a module.
func owningModule(mod *yang.Module) *yang.Module {
	if mod == nil {
		return nil
	}
	if mod.BelongsTo == nil {
		return mod
	}
	if mod.Modules == nil {
		return mod
	}
	if parent := mod.Modules.Modules[mod.BelongsTo.Name]; parent != nil {
		return parent
	}
	return mod
}

// resolveModulePrefixInMemory resolves prefix in mod's lexical module scope
// using only modules already loaded in memory, reporting whether one was found.
func resolveModulePrefixInMemory(mod *yang.Module, prefix string) (*yang.Module, bool) {
	if mod == nil {
		return nil, false
	}
	if prefix == "" {
		return mod, true
	}
	if prefix == mod.GetPrefix() {
		return mod, true
	}
	for _, imp := range mod.Import {
		if imp.Prefix == nil {
			continue
		}
		if imp.Prefix.Name == prefix {
			target := importedModule(mod, imp)
			if target == nil {
				return nil, false
			}
			return target, true
		}
	}
	return nil, false
}

// reachableModulesInMemory answers mod and every module or submodule sharing
// its namespace through belongs-to and transitive include statements, using
// only modules already loaded in memory.
func reachableModulesInMemory(mod *yang.Module) []*yang.Module {
	if mod == nil {
		return nil
	}
	visited := map[*yang.Module]bool{mod: true}
	result := []*yang.Module{mod}
	pending := []*yang.Module{mod}
	if owner := owningModule(mod); owner != nil {
		if !visited[owner] {
			visited[owner] = true
			result = append(result, owner)
			pending = append(pending, owner)
		}
	}
	// Bounded by the finite set of loaded modules and submodules via visited.
	for len(pending) > 0 {
		current := pending[0]
		pending = pending[1:]
		for _, inc := range current.Include {
			if inc.Module == nil {
				if current.Modules == nil {
					continue
				}
			}
			sub := includedSubmodule(current, inc)
			if sub == nil {
				continue
			}
			if visited[sub] {
				continue
			}
			visited[sub] = true
			result = append(result, sub)
			pending = append(pending, sub)
		}
	}
	return result
}

// moduleGroupingNodes collects every *yang.Grouping declared anywhere in mod's
// AST in deterministic pre-order, using substatementFieldTable with an explicit
// stack.
func moduleGroupingNodes(mod *yang.Module) []*yang.Grouping {
	if mod == nil {
		return nil
	}
	table := substatementFieldTable()
	var groupings []*yang.Grouping
	pending := []reflect.Value{reflect.ValueOf(mod)}
	for len(pending) > 0 {
		node := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if grouping, isGrouping := reflect.TypeAssert[*yang.Grouping](node); isGrouping {
			groupings = append(groupings, grouping)
		}
		before := len(pending)
		pending = appendSubstatementNodes(pending, node.Elem(), table)
		slices.Reverse(pending[before:])
	}
	return groupings
}

// groupingEdges answers the direct grouping dependencies of g: every `uses`
// statement anywhere in g's body (outside nested groupings) and every direct
// nested `grouping` inside g's body. Nested groupings are included as edges
// because goyang's ToEntry eagerly expands nested groupings before caching the
// enclosing grouping, so a nested grouping using an outer grouping overflows
// the stack even when the outer grouping never uses the nested one.
func groupingEdges(g *yang.Grouping) []groupingEdge {
	table := substatementFieldTable()
	var edges []groupingEdge
	var pending []reflect.Value
	pending = appendSubstatementNodes(pending, reflect.ValueOf(g).Elem(), table)
	slices.Reverse(pending)
	for len(pending) > 0 {
		node := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if nested, isGrouping := reflect.TypeAssert[*yang.Grouping](node); isGrouping {
			edges = append(edges, groupingEdge{target: nested, location: yang.Source(nested)})
			// nested is its own vertex in the grouping graph; do not attribute
			// its internal uses edges to g.
			continue
		}
		if uses, isUses := reflect.TypeAssert[*yang.Uses](node); isUses {
			if target, found := resolveGroupingInMemory(uses, uses.Name); found {
				edges = append(edges, groupingEdge{target: target, location: yang.Source(uses)})
			}
		}
		before := len(pending)
		pending = appendSubstatementNodes(pending, node.Elem(), table)
		slices.Reverse(pending[before:])
	}
	return edges
}

// resolveGroupingInMemory resolves a `uses` grouping reference strictly from
// loaded in-memory ASTs following RFC 7950 Section 5.5 lexical scoping:
// unprefixed or own-prefix names walk up the enclosing ParentNode chain before
// searching the module and its included submodules; foreign-prefix names search
// the imported module's top-level groupings and its included submodules.
func resolveGroupingInMemory(uses *yang.Uses, ref string) (*yang.Grouping, bool) {
	root := yang.RootNode(uses)
	if root == nil {
		return nil, false
	}
	prefix, localName, hasPrefix := strings.Cut(ref, ":")
	if !hasPrefix {
		localName = ref
		prefix = ""
	}
	isLocal := prefix == ""
	if !isLocal {
		isLocal = prefix == root.GetPrefix()
	}
	if isLocal {
		for ancestor := uses.ParentNode(); ancestor != nil; ancestor = ancestor.ParentNode() {
			if found, ok := findDirectGrouping(ancestor, localName); ok {
				return found, true
			}
		}
		return findTopLevelGrouping(root, localName)
	}
	imported, found := resolveModulePrefixInMemory(root, prefix)
	if !found {
		return nil, false
	}
	return findTopLevelGrouping(imported, localName)
}

// findDirectGrouping returns a grouping named name declared directly on node
// and whether one was found.
func findDirectGrouping(node yang.Node, name string) (*yang.Grouping, bool) {
	holder, ok := node.(groupingHolder)
	if !ok {
		return nil, false
	}
	for _, g := range holder.Groupings() {
		if g.Name == name {
			return g, true
		}
	}
	return nil, false
}

// findTopLevelGrouping searches mod and its reachable submodules in memory for
// a top-level grouping named name and reports whether one was found.
func findTopLevelGrouping(mod *yang.Module, name string) (*yang.Grouping, bool) {
	for _, candidate := range reachableModulesInMemory(mod) {
		for _, g := range candidate.Grouping {
			if g.Name == name {
				return g, true
			}
		}
	}
	return nil, false
}

// cyclicGroupingErrors detects every direct or indirect grouping cycle across
// mods using iterative three-color DFS, returning one error wrapping
// ErrCyclicGrouping per cycle back-edge.
func cyclicGroupingErrors(mods []*yang.Module) []error {
	all := make([]*yang.Grouping, 0, len(mods))
	for _, mod := range mods {
		all = append(all, moduleGroupingNodes(mod)...)
	}
	if len(all) == 0 {
		return nil
	}
	edges := make(map[*yang.Grouping][]groupingEdge, len(all))
	for _, g := range all {
		edges[g] = groupingEdges(g)
	}
	state := make(map[*yang.Grouping]cycleVisitState, len(all))
	var errs []error
	for _, start := range all {
		if state[start] != cycleUnvisited {
			continue
		}
		state[start] = cycleVisiting
		stack := []groupingFrame{{node: start}}
		for len(stack) > 0 {
			top := &stack[len(stack)-1]
			outgoing := edges[top.node]
			if top.next >= len(outgoing) {
				state[top.node] = cycleDone
				stack = stack[:len(stack)-1]
				continue
			}
			edge := outgoing[top.next]
			top.next++
			switch state[edge.target] {
			case cycleUnvisited:
				state[edge.target] = cycleVisiting
				stack = append(stack, groupingFrame{node: edge.target})
			case cycleVisiting:
				errs = append(errs, cyclicGroupingError(stack, edge))
			case cycleDone:
				// Acyclic subgraph already explored (e.g. diamond DAG).
			}
		}
	}
	return errs
}

// cycleError records the declaring module alongside a pre-process grouping or
// identity cycle error so multi-module Join blames the offending module without
// running post-Process checks on an unprocessed AST.
type cycleError struct {
	module string
	err    error
}

func (e *cycleError) Error() string { return e.err.Error() }

func (e *cycleError) Unwrap() error { return e.err }

// cyclicGroupingError formats one ErrCyclicGrouping refusal naming the
// module, source location, offending grouping, and cycle chain.
func cyclicGroupingError(stack []groupingFrame, closing groupingEdge) error {
	startIndex := 0
	for i := range stack {
		if stack[i].node == closing.target {
			startIndex = i
			break
		}
	}
	nodes := make([]yang.Node, 0, len(stack)-startIndex+1)
	for i := startIndex; i < len(stack); i++ {
		nodes = append(nodes, stack[i].node)
	}
	nodes = append(nodes, closing.target)
	closingNode := stack[len(stack)-1].node
	modName := nodeModuleName(closingNode)
	location := closing.location
	if location == "" {
		location = yang.Source(closingNode)
	}
	return &cycleError{
		module: modName,
		err: fmt.Errorf("%w: module %s: %s: grouping %s has a circular dependency (%s) (RFC 7950 Section 7.12)",
			ErrCyclicGrouping, modName, location, closingNode.Name, formatCycleChain(nodes)),
	}
}

// resolveIdentityInMemory resolves an identity `base` reference strictly from
// loaded in-memory modules and submodules, reporting whether one was found.
func resolveIdentityInMemory(mod *yang.Module, ref string) (*yang.Identity, bool) {
	if mod == nil {
		return nil, false
	}
	prefix, localName, hasPrefix := strings.Cut(ref, ":")
	if !hasPrefix {
		localName = ref
		prefix = ""
	}
	targetMod := mod
	if prefix != "" {
		if prefix != mod.GetPrefix() {
			resolved, found := resolveModulePrefixInMemory(mod, prefix)
			if !found {
				return nil, false
			}
			targetMod = resolved
		}
	}
	for _, candidate := range reachableModulesInMemory(targetMod) {
		for _, id := range candidate.Identity {
			if id.Name == localName {
				return id, true
			}
		}
	}
	return nil, false
}

// cyclicIdentityErrors detects every direct or indirect identity base cycle
// across mods using iterative three-color DFS, returning one error wrapping
// ErrCyclicIdentity per cycle back-edge.
func cyclicIdentityErrors(mods []*yang.Module) []error {
	var all []*yang.Identity
	for _, mod := range mods {
		all = append(all, mod.Identity...)
	}
	if len(all) == 0 {
		return nil
	}
	edges := make(map[*yang.Identity][]identityEdge, len(all))
	for _, id := range all {
		root := yang.RootNode(id)
		for _, base := range id.Base {
			target, found := resolveIdentityInMemory(root, base.Name)
			if !found {
				continue
			}
			edges[id] = append(edges[id], identityEdge{
				target:   target,
				location: yang.Source(base),
			})
		}
	}
	state := make(map[*yang.Identity]cycleVisitState, len(all))
	var errs []error
	for _, start := range all {
		if state[start] != cycleUnvisited {
			continue
		}
		state[start] = cycleVisiting
		stack := []identityFrame{{node: start}}
		for len(stack) > 0 {
			top := &stack[len(stack)-1]
			outgoing := edges[top.node]
			if top.next >= len(outgoing) {
				state[top.node] = cycleDone
				stack = stack[:len(stack)-1]
				continue
			}
			edge := outgoing[top.next]
			top.next++
			switch state[edge.target] {
			case cycleUnvisited:
				state[edge.target] = cycleVisiting
				stack = append(stack, identityFrame{node: edge.target})
			case cycleVisiting:
				errs = append(errs, cyclicIdentityError(stack, edge))
			case cycleDone:
				// Acyclic subgraph already explored (e.g. multi-base diamond DAG).
			}
		}
	}
	return errs
}

// cyclicIdentityError formats one ErrCyclicIdentity refusal naming the module,
// source location, offending identity, and cycle chain.
func cyclicIdentityError(stack []identityFrame, closing identityEdge) error {
	startIndex := 0
	for i := range stack {
		if stack[i].node == closing.target {
			startIndex = i
			break
		}
	}
	nodes := make([]yang.Node, 0, len(stack)-startIndex+1)
	for i := startIndex; i < len(stack); i++ {
		nodes = append(nodes, stack[i].node)
	}
	nodes = append(nodes, closing.target)
	closingNode := stack[len(stack)-1].node
	modName := nodeModuleName(closingNode)
	location := closing.location
	if location == "" {
		location = yang.Source(closingNode)
	}
	return &cycleError{
		module: modName,
		err: fmt.Errorf("%w: module %s: %s: identity %s has a circular base dependency (%s) (RFC 7950 Section 7.18.2)",
			ErrCyclicIdentity, modName, location, closingNode.Name, formatCycleChain(nodes)),
	}
}

// nodeModuleName answers the declaring module or submodule name of node.
func nodeModuleName(node yang.Node) string {
	if root := yang.RootNode(node); root != nil {
		return root.Name
	}
	return "unknown"
}

// formatCycleChain renders nodes as "a -> b -> a" when all nodes belong to one
// module, or "mod-a:a -> mod-b:b -> mod-a:a" when the cycle crosses modules or
// submodules.
func formatCycleChain(nodes []yang.Node) string {
	crossModule := false
	firstMod := ""
	if len(nodes) > 0 {
		firstMod = nodeModuleName(nodes[0])
	}
	for _, node := range nodes[1:] {
		if nodeModuleName(node) != firstMod {
			crossModule = true
			break
		}
	}
	parts := make([]string, len(nodes))
	for i, node := range nodes {
		if crossModule {
			parts[i] = nodeModuleName(node) + ":" + node.NName()
			continue
		}
		parts[i] = node.NName()
	}
	return strings.Join(parts, " -> ")
}

// moduleTypes is the `type` nodes of one module's goyang AST, in the order
// moduleTypeNodes meets them, beside the module's name for the refusal text.
// newModuleTypes is its only constructor, and the length and enum checks are
// its methods: neither holds the module, so neither can walk it again. The
// zero value is invalid; only newModuleTypes builds one.
type moduleTypes struct {
	module string
	types  []*yang.Type
}

// newModuleTypes walks mod's goyang AST once and answers its `type` nodes.
func newModuleTypes(mod *yang.Module) moduleTypes {
	return moduleTypes{module: mod.Name, types: moduleTypeNodes(mod)}
}

// lengthErrors returns one error for each `length` statement of the module
// whose parts, in the order written, are not disjoint and ascending over the
// bounds of the type the statement restricts.
func (m moduleTypes) lengthErrors() []error {
	var errs []error
	for _, typ := range m.types {
		if typ.Length == nil {
			continue
		}
		// RFC 7950 Section 9.4.4
		err := lengthOrderError(typ)
		if err == nil {
			continue
		}
		errs = append(errs, fmt.Errorf("%w: module %s: %s: length %q: %w",
			ErrLengthOrder, m.module, yang.Source(typ.Length), typ.Length.Name, err))
	}
	return errs
}

// lengthOrderError answers why the length statement of typ is out of order,
// or nil when its parts are disjoint and ascending.
func lengthOrderError(typ *yang.Type) error {
	restricted, err := restrictedLengthSpan(typ)
	if err != nil {
		if namesTypeBound(typ.Length.Name) {
			return err
		}
		// No part names min or max, so no bound of the restricted type is
		// read and any span serves.
		restricted = unrestrictedLength
	}
	return lengthPartsAscending(typ.Length.Name, restricted)
}

// namesTypeBound reports whether a length expression names "min" or "max",
// the bounds of the type it restricts.
func namesTypeBound(expression string) bool {
	for part := range strings.SplitSeq(expression, "|") {
		lower, upper, _ := strings.Cut(part, "..")
		for _, bound := range []string{lower, upper} {
			bound = strings.TrimSpace(bound)
			if bound == boundaryMin || bound == boundaryMax {
				return true
			}
		}
	}
	return false
}

// boundaryMin and boundaryMax are the Section 14 "min-keyword" and
// "max-keyword", the special bounds of a length or range expression.
const (
	boundaryMin = "min"
	boundaryMax = "max"
)

// lengthSpan is a closed span of lengths, lower to upper. newLengthSpan is its
// only constructor and refuses a lower bound above the upper one, so a
// lengthSpan in hand never descends. The zero value is the span of length 0.
type lengthSpan struct {
	lower uint64
	upper uint64
}

// newLengthSpan answers the span lower..upper, or an error when it descends.
func newLengthSpan(lower, upper uint64) (lengthSpan, error) {
	if lower > upper {
		return lengthSpan{}, fmt.Errorf("lower bound %d is above upper bound %d", lower, upper)
	}
	return lengthSpan{lower: lower, upper: upper}, nil
}

// unrestrictedLength is the span of a string or binary that no length
// restricts. RFC 7950 Section 9.4.4: "An implementation is not required to
// support a length value larger than 18446744073709551615." goyang applies
// the same span (types.go, Type.resolve: "parentRange := Uint64Range").
var unrestrictedLength = lengthSpan{lower: 0, upper: ^uint64(0)}

// restrictedLengthSpan answers the lengths accepted by the type typ's length
// statement restricts: the effective length of the typedef typ derives from,
// which goyang resolved through the whole derivation chain into the base
// type node's YangType, or unrestrictedLength when no type in the chain
// carries a length.
//
// A type goyang never resolved has no YangType: it sits in a grouping no
// schema node uses. Its derivation is then unknown unless it names a built-in
// type directly, so the answer is an error rather than a guessed span.
func restrictedLengthSpan(typ *yang.Type) (lengthSpan, error) {
	if typ.YangType == nil {
		kind := yang.TypeKindFromName[typ.Name]
		if kind == yang.Ystring || kind == yang.Ybinary {
			return unrestrictedLength, nil
		}
		return lengthSpan{}, fmt.Errorf("type %s is unresolved, so min and max name no bounds", typ.Name)
	}
	base := typ.YangType.Base
	if base == nil || base.YangType == nil {
		return unrestrictedLength, nil
	}
	parts := base.YangType.Length
	if len(parts) == 0 {
		return unrestrictedLength, nil
	}
	first, last := parts[0].Min, parts[len(parts)-1].Max
	if first.Negative || last.Negative {
		return lengthSpan{}, fmt.Errorf("type %s has a negative length", base.Name)
	}
	return newLengthSpan(first.Value, last.Value)
}

// lengthPartsAscending checks a length expression in the order it is written,
// over restricted, the lengths accepted by the type it restricts.
//
// RFC 7950 Section 9.4.4: "Length-restricting values MUST NOT be negative.
// If multiple values or ranges are given, they all MUST be disjoint and MUST
// be in ascending order." The parts are read in the order written.
func lengthPartsAscending(expression string, restricted lengthSpan) error {
	var previous lengthSpan
	for i, part := range strings.Split(expression, "|") {
		span, err := lengthPartSpan(part, restricted)
		if err != nil {
			return err
		}
		if i > 0 && span.lower <= previous.upper {
			return fmt.Errorf("part %q does not start after the part before it", strings.TrimSpace(part))
		}
		previous = span
	}
	return nil
}

// lengthPartSpan answers the span of one length part, an explicit value or
// "lower..upper", with min and max read over restricted.
func lengthPartSpan(part string, restricted lengthSpan) (lengthSpan, error) {
	lowerText, upperText, isRange := strings.Cut(part, "..")
	lower, err := lengthBound(lowerText, restricted)
	if err != nil {
		return lengthSpan{}, err
	}
	if !isRange {
		return newLengthSpan(lower, lower)
	}
	upper, err := lengthBound(upperText, restricted)
	if err != nil {
		return lengthSpan{}, err
	}
	return newLengthSpan(lower, upper)
}

// lengthBound answers one length boundary: "min", "max", or a non-negative
// integer (RFC 7950 Section 14, length-boundary).
//
// RFC 7950 Section 9.4.4: "A length value is a non-negative integer or one of
// the special values "min" or "max". "min" and "max" mean the minimum and
// maximum lengths accepted for the type being restricted, respectively." So
// min and max read restricted's bounds.
func lengthBound(text string, restricted lengthSpan) (uint64, error) {
	bound := strings.TrimSpace(text)
	if bound == boundaryMin {
		return restricted.lower, nil
	}
	if bound == boundaryMax {
		return restricted.upper, nil
	}
	value, err := strconv.ParseUint(bound, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("boundary %q is neither min, max nor a non-negative integer", bound)
	}
	return value, nil
}

// enumErrors returns one error for each enumeration of the module whose
// values RFC 7950 Section 9.6.4.2 forbids, and for each `enum` statement in a
// restricted enumeration of the module that departs from its base type.
//
// goyang builds a restricted type's values afresh from the restricting
// statements, so both checks read the values parseEnumAssignment computes
// from the statements.
// An enumeration is checked whether or not goyang resolved it: one in a
// grouping no schema node uses is still a statement the RFC binds. A
// restriction goyang never resolved has no YangType: it restricts nothing
// this loader can follow, and is skipped.
func (m moduleTypes) enumErrors() []error {
	var errs []error
	for _, typ := range m.types {
		if len(typ.Enum) == 0 {
			continue
		}
		if typ.Name == enumerationType {
			// RFC 7950 Section 9.6.4.2
			if _, err := assignEnumValues(typ); err != nil {
				errs = append(errs, fmt.Errorf("module %s: %w", m.module, err))
			}
			continue
		}
		if typ.YangType == nil {
			continue
		}
		if typ.YangType.Kind != yang.Yenum {
			continue
		}
		// RFC 7950 Sections 9.6.4 and 9.6.4.2
		errs = append(errs, enumRestrictionErrors(m.module, typ)...)
	}
	return errs
}

// enumRestrictionErrors checks each enum statement of typ, a restriction of a
// derived enumeration in the module named module, against the type it
// restricts.
//
// RFC 7950 Section 9.6.4: "When an existing enumeration type is restricted,
// the set of assigned names in the new type MUST be a subset of the base
// type's set of assigned names. The value of such an assigned name MUST NOT
// be changed."
//
// RFC 7950 Section 9.6.4.2: "When an existing enumeration type is restricted,
// the "value" statement MUST either have the same value as in the base type
// or not be present, in which case the value is the same as in the base type."
// The base value is the one the root enumeration assigns (assignEnumValues),
// which every restriction keeps.
func enumRestrictionErrors(module string, typ *yang.Type) []error {
	base := nearestEnumBase(typ)
	if base == nil {
		return []error{fmt.Errorf("%w: module %s: %s: type %s restricts no enumeration",
			ErrEnumRestriction, module, yang.Source(typ), typ.Name)}
	}
	assigned, err := parseEnumAssignment(base)
	if err != nil {
		return []error{fmt.Errorf("%w: module %s: %s: type %s: %w",
			ErrEnumRestriction, module, yang.Source(typ), typ.Name, err)}
	}
	var errs []error
	for _, enum := range typ.Enum {
		baseValue, held := assigned.Value(enum.Name)
		if !held {
			errs = append(errs, fmt.Errorf("%w: module %s: %s: enum %q is not an assigned name of type %s",
				ErrEnumRestriction, module, yang.Source(enum), enum.Name, typ.Name))
			continue
		}
		if enum.Value == nil {
			continue
		}
		value, err := strconv.ParseInt(strings.TrimSpace(enum.Value.Name), 10, 64)
		if err != nil {
			errs = append(errs, fmt.Errorf("%w: module %s: %s: enum %q: value %q is not an integer",
				ErrEnumRestriction, module, yang.Source(enum), enum.Name, enum.Value.Name))
			continue
		}
		if value != baseValue {
			errs = append(errs, fmt.Errorf("%w: module %s: %s: enum %q: value %d, the base type assigns %d",
				ErrEnumRestriction, module, yang.Source(enum), enum.Name, value, baseValue))
		}
	}
	return errs
}

// nearestEnumBase answers, for a restricted enumeration typ, the nearest type
// it derives from that lists enum statements, whose names bound typ's, or nil
// when the derivation reaches none. The loop follows the typedef chain goyang
// resolved, so its length is the module's derivation depth.
func nearestEnumBase(typ *yang.Type) *yang.Type {
	for node := typ.YangType.Base; node != nil; node = resolvedBase(node) {
		if len(node.Enum) > 0 {
			return node
		}
	}
	return nil
}

// resolvedBase answers the type node goyang resolved typ against, or nil when
// goyang resolved none.
func resolvedBase(typ *yang.Type) *yang.Type {
	if typ.YangType == nil {
		return nil
	}
	return typ.YangType.Base
}

// moduleTypeNodes answers every `type` node of mod's goyang AST. The walk is
// an explicit stack over the substatement fields goyang's own parser fills
// (isSubstatementField), so it reaches each node the module text declares.
// It reads a node's substatement fields from its type's entry in
// substatementFieldTable, never from the struct tags. goyang's back
// references (Parent, Include.Module, Type.YangType) are no substatement
// fields, so the AST is a tree to the walk and each node is visited once.
func moduleTypeNodes(mod *yang.Module) []*yang.Type {
	if mod == nil {
		return nil
	}
	table := substatementFieldTable()
	var types []*yang.Type
	pending := []reflect.Value{reflect.ValueOf(mod)}
	for len(pending) > 0 {
		node := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if typ, isType := reflect.TypeAssert[*yang.Type](node); isType {
			types = append(types, typ)
		}
		pending = appendSubstatementNodes(pending, node.Elem(), table)
	}
	return types
}

// appendSubstatementNodes appends the AST nodes held in node's substatement
// fields, each a pointer to a struct or a slice of them, skipping a nil
// pointer, which holds no node. The order is the fields' order, then the
// slice's, so the walk meets the nodes in the same order whatever the table.
func appendSubstatementNodes(pending []reflect.Value, node reflect.Value, table substatementFields) []reflect.Value {
	indexes, known := table[node.Type()]
	if !known {
		panic("BUG: goyang node type " + node.Type().String() + " is not reachable from yang.Module through substatement fields")
	}
	for _, i := range indexes {
		value := node.Field(i)
		// goyang's substatement fields are pointers or slices of pointers (substatementNodeType); nothing else holds a node
		//exhaustive:ignore
		switch value.Kind() {
		case reflect.Pointer:
			if !value.IsNil() {
				pending = append(pending, value)
			}
		case reflect.Slice:
			for j := range value.Len() {
				element := value.Index(j)
				if !element.IsNil() {
					pending = append(pending, element)
				}
			}
		}
	}
	return pending
}

// substatementFields maps a goyang AST node type, a struct type, to the
// indexes of its substatement fields (isSubstatementField), ascending.
type substatementFields map[reflect.Type][]int

// substatementFieldTable answers the substatement fields of every goyang AST
// node type a module's walk can meet, read from the struct tags once per
// process rather than once per node. The tags are fixed when goyang is
// compiled, so the answer never changes. Its entry count is bounded by the
// struct types goyang's yang package declares, those yang.Module reaches
// through substatement fields; nothing a module holds adds an entry. Built on
// the first walk, not at package load. Safe for concurrent use: sync.OnceValue
// builds the map once, and every caller only reads it.
var substatementFieldTable = sync.OnceValue(func() substatementFields {
	return buildSubstatementFields(reflect.TypeFor[yang.Module]())
})

// buildSubstatementFields answers the substatement fields of root, a struct
// type, and of every struct type root reaches through them. The walk is an
// explicit stack over types, each expanded once, so it ends within the
// number of struct types goyang declares.
func buildSubstatementFields(root reflect.Type) substatementFields {
	table := substatementFields{}
	pending := []reflect.Type{root}
	for len(pending) > 0 {
		node := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if _, built := table[node]; built {
			continue
		}
		var indexes []int
		for i := range node.NumField() {
			field := node.Field(i)
			if !isSubstatementField(field) {
				continue
			}
			indexes = append(indexes, i)
			pending = append(pending, substatementNodeType(field))
		}
		table[node] = indexes
	}
	return table
}

// substatementNodeType answers the struct type of the nodes field holds,
// field being a pointer to one or a slice of such pointers. goyang declares
// no other shape of substatement field and the walk could not read one, so
// another shape is a Ze defect met on the first walk of any module, never the
// fault of a module's text.
func substatementNodeType(field reflect.StructField) reflect.Type {
	holder := field.Type
	if holder.Kind() == reflect.Slice {
		holder = holder.Elem()
	}
	if holder.Kind() != reflect.Pointer {
		panic("BUG: goyang substatement field " + field.Name + " is " + field.Type.String() + ", not a pointer to a node or a slice of them")
	}
	if holder.Elem().Kind() != reflect.Struct {
		panic("BUG: goyang substatement field " + field.Name + " is " + field.Type.String() + ", whose nodes are not structs")
	}
	return holder.Elem()
}

// isSubstatementField reports whether f is a field goyang fills from a
// substatement: its `yang` tag names a keyword, which is lower case, where
// the bookkeeping tags (Name, Statement, Parent, Ext) are capitalized.
func isSubstatementField(f reflect.StructField) bool {
	keyword := substatementKeyword(f)
	if keyword == "" {
		return false
	}
	return f.Type.Kind() == reflect.Pointer || f.Type.Kind() == reflect.Slice
}

// substatementKeyword answers the YANG keyword f's `yang` tag names, or ""
// when the tag is absent or names a bookkeeping field.
func substatementKeyword(f reflect.StructField) string {
	keyword, _, _ := strings.Cut(f.Tag.Get("yang"), ",")
	if keyword == "" {
		return ""
	}
	if !unicode.IsLower(rune(keyword[0])) {
		return ""
	}
	return keyword
}

// moduleExtensionSubstatementErrors returns one error for each statement
// under an extension statement of source's module that breaks the Section
// 14 grammar. source carries the text the module was parsed from, for the
// block a statement's rule requires and goyang does not record.
//
// RFC 7950 Section 7.19: "Syntactically, the substatements MUST be YANG
// statements, including extensions defined using "extension" statements.
// YANG statements in extensions MUST follow the syntactical rules in
// Section 14." Each statement under an extension is resolved to the Section
// 14 rule its parent's block names, and then checked against that rule
// (extensionSubstatementError).
func moduleExtensionSubstatementErrors(source moduleSource) []error {
	mod := source.module
	if mod.Source == nil {
		return nil
	}
	grammar := rfc7950Grammar()
	// production is the Section 14 rule statement was resolved to, nil when
	// no extension holds statement or statement is an extension usage.
	type visit struct {
		statement  *yang.Statement
		production *statementProduction
	}
	var errs []error
	pending := []visit{{statement: mod.Source}}
	for len(pending) > 0 {
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		statement := current.statement
		var block *statementBlock
		if current.production != nil {
			block = current.production.block
		}
		if isExtensionUsage(statement) {
			block = grammar.extension
		}
		if block == nil {
			for _, sub := range statement.SubStatements() {
				pending = append(pending, visit{statement: sub})
			}
			continue
		}
		children, childErrs := resolveSubstatements(grammar, block, statement)
		for _, err := range childErrs {
			errs = append(errs, fmt.Errorf("%w: module %s: %w", ErrExtensionSubstatement, mod.Name, err))
		}
		for i, sub := range statement.SubStatements() {
			pending = append(pending, visit{statement: sub, production: children[i]})
		}
		if current.production == nil {
			continue
		}
		// RFC 7950 Section 7.19
		if err := extensionSubstatementError(grammar, current.production, statement, children, source); err != nil {
			errs = append(errs, fmt.Errorf("%w: module %s: %s: %w",
				ErrExtensionSubstatement, mod.Name, statement.Location(), err))
		}
	}
	return errs
}

// isExtensionUsage reports whether statement is an extension usage, whose
// keyword is "prefix:identifier" (RFC 7950 Section 14, unknown-statement).
func isExtensionUsage(statement *yang.Statement) bool {
	return strings.Contains(statement.Keyword, ":")
}

// resolveSubstatements answers, for each substatement of statement, the
// production of block it resolves to, nil for an extension usage or a
// substatement that resolves to none, and one error for each of those.
func resolveSubstatements(grammar *yangGrammar, block *statementBlock, statement *yang.Statement) ([]*statementProduction, []error) {
	subs := statement.SubStatements()
	children := make([]*statementProduction, len(subs))
	var errs []error
	for i, sub := range subs {
		if isExtensionUsage(sub) {
			continue
		}
		production, err := resolveProduction(grammar, block, sub)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", sub.Location(), err))
			continue
		}
		children[i] = production
	}
	return children, errs
}

// resolveProduction answers the production of block statement follows: the
// one, among the productions block admits for its keyword, whose argument
// rule its argument matches. The context is block, the parent's rule, so
// "augment" resolves to augment-stmt in body-stmts and to uses-augment-stmt
// under uses, and "deviate" to the one of its four rules its argument names.
func resolveProduction(grammar *yangGrammar, block *statementBlock, statement *yang.Statement) (*statementProduction, error) {
	candidates := block.admitted[statement.Keyword]
	if len(candidates) == 0 {
		if _, known := grammar.keywords[statement.Keyword]; !known {
			return nil, fmt.Errorf("%q is not a YANG statement", statement.Keyword)
		}
		return nil, fmt.Errorf("%s is not a substatement of %s", statement.Keyword, block.owner)
	}
	var matching []*statementProduction
	errs := make([]error, 0, len(candidates))
	for _, candidate := range candidates {
		err := argumentError(candidate, statement)
		if err == nil {
			matching = append(matching, candidate)
			continue
		}
		errs = append(errs, err)
	}
	if len(matching) == 0 {
		return nil, errors.Join(errs...)
	}
	if len(matching) > 1 {
		// TestSameKeywordProductionsHaveDisjointArguments keeps this a Ze defect.
		return nil, fmt.Errorf("BUG: %s %q matches %d rules of %s", statement.Keyword, statement.Argument, len(matching), block.owner)
	}
	return matching[0], nil
}

// argumentError answers why statement's argument breaks production's
// argument rule, or nil when it matches: absent when the rule takes none,
// present and matching the rule otherwise.
func argumentError(production *statementProduction, statement *yang.Statement) error {
	argument, hasArgument := statement.Arg()
	if production.argument == "" {
		if hasArgument {
			return fmt.Errorf("%s takes no argument", statement.Keyword)
		}
		return nil
	}
	if !hasArgument {
		return fmt.Errorf("%s requires an argument", statement.Keyword)
	}
	if err := checkArgument(production.argument, argument); err != nil {
		return fmt.Errorf("%s argument: %w", statement.Keyword, err)
	}
	return nil
}

// extensionSubstatementError answers why statement, resolved to production
// under an extension, breaks production's Section 14 rule, or nil when it
// does not. children holds the production each substatement resolved to, nil
// for an extension usage or one that resolved to none, whose error
// resolveSubstatements already gave.
//
// Two rules are checked here, the keyword and argument having chosen
// production. The block: a rule that opens it with a bare "{" requires it,
// and goyang records no block, so where an empty block would match, the
// module text is read (statementHasBlock). The counts: each repetition of the
// rule's block bounds how many substatements of a rule it takes, "[x]" at
// most one, "*x" any, "1*x" at least one, a bare "x" exactly one, and the
// counts together must match the block (yangGrammar.matches), which is where
// "deviate-not-supported-stmt / 1*(deviate-add-stmt / ...)" and the
// alternatives of type-body-stmts are decided.
func extensionSubstatementError(grammar *yangGrammar, production *statementProduction, statement *yang.Statement,
	children []*statementProduction, source moduleSource) error {
	block := production.block
	counts := make(childCounts, len(block.slots))
	for _, child := range children {
		if child == nil {
			continue
		}
		counts[block.slots[child]]++
	}
	if production.form == blockRequired && block.emptyAccepted && len(statement.SubStatements()) == 0 {
		hasBlock, err := statementHasBlock(source, statement)
		if err != nil {
			return fmt.Errorf("%s: %w", production.rule, err)
		}
		if !hasBlock {
			return fmt.Errorf("%s requires a block", production.rule)
		}
	}
	for keyword, productions := range block.admitted {
		for _, admitted := range productions {
			low, high := grammar.occurrences(block.content, admitted)
			found := counts[block.slots[admitted]]
			if high != repeatUnbounded && found > high {
				return fmt.Errorf("%s appears %d times in %s, %s admits at most %d", keyword, found, statement.Keyword, production.rule, high)
			}
			if found < low {
				return fmt.Errorf("%s requires at least %d %s, found %d", statement.Keyword, low, keyword, found)
			}
		}
	}
	if !grammar.matches(block, counts) {
		return fmt.Errorf("the substatements of %s do not match %s", statement.Keyword, production.rule)
	}
	return nil
}
