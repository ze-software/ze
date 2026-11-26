// Design: docs/architecture/core-design.md — ze:backend commit-time feature gate
// Related: dataplane_verdict.go — evaluateDataPlaneVerdict invokes the whole-tree walk at both config doors and ze doctor
// Related: schema.go — Backend field on LeafNode/ContainerNode/ListNode
// Related: yang_schema.go — getBackendExtension reader and schema population
//
// Package config — backend feature gate.
//
// The gate walks a parsed config tree as written (before any schema default is
// materialized) alongside the schema and rejects configs that exercise features
// the active backend does not implement. A feature's support matrix is declared
// via `ze:backend "<names>"` on the relevant YANG node. Absent annotation means
// unrestricted (every registered backend is accepted).
//
// The config component runs one whole-tree walk at its two config doors
// (LoadConfig and verifyTree) and in ze doctor (doctor-dataplane-mismatch),
// judging each root against the declared DataPlaneSelector whose roots claim
// that subtree, or against the primary data-plane selector for nodes outside
// every declared selector root. No component calls the gate on its own.

package config

import (
	"fmt"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// treePathStep records one structural step along the walked config tree path
// so start-time exclusion can build a concrete Tree.RemovePath target directly
// from schema node kinds without reparsing display strings.
type treePathStep struct {
	name        string
	listKey     string
	isListEntry bool
	isLeaf      bool
}

// backendMismatch is one ze:backend exclusion emitted by the walker.
type backendMismatch struct {
	path       string
	active     string
	supporting []string
	steps      []treePathStep
}

// backendWalkSelector describes one declared selector that governs a set of
// config subtrees during the engine-owned ze:backend walk.
type backendWalkSelector struct {
	leafPath   string
	display    string
	roots      []string
	active     string
	defaultVal string
	valid      bool
}

// backendWalkFinding is one mismatch or empty-backend host verdict produced by
// walkBackendFeaturesTree for rendering by evaluateDataPlaneVerdict.
type backendWalkFinding struct {
	selector    backendWalkSelector
	mismatch    backendMismatch
	hostVerdict bool
}

// walkBackendFeaturesTree walks every root present in treeMap against schema,
// judging each subtree against the declared selector whose Roots claim that
// path, or against primary for nodes outside every declared selector root (for
// example container networks). treeMap MUST be lowered from the tree as written
// (Tree.ToPluginMap), before any schema default is materialized.
//
// Subtrees governed by a selector with valid == false are skipped because the
// verdict has already refused or deferred that selector.
func walkBackendFeaturesTree(
	treeMap map[string]any,
	schema *Schema,
	primary backendWalkSelector,
	secondaries []backendWalkSelector,
) []backendWalkFinding {
	if schema == nil {
		return nil
	}
	if len(treeMap) == 0 {
		return nil
	}

	selectorByPath := make(map[string]backendWalkSelector)
	hasNestedPrefix := make(map[string]bool)
	registerWalkSelector := func(sel backendWalkSelector) {
		for _, root := range sel.roots {
			clean := strings.TrimPrefix(root, PathSep)
			if clean == "" {
				continue
			}
			selectorByPath[clean] = sel
			parts := SplitPath(clean)
			for i := 1; i < len(parts); i++ {
				hasNestedPrefix[textbuf.Join(parts[:i], PathSep)] = true
			}
		}
	}
	registerWalkSelector(primary)
	for _, sec := range secondaries {
		registerWalkSelector(sec)
	}

	emittedHostVerdict := make(map[string]bool)
	var findings []backendWalkFinding
	for _, key := range sortedKeysAny(treeMap) {
		rootNode, err := schema.Lookup(key)
		if err != nil {
			continue
		}
		if rootNode == nil {
			continue
		}
		walkSubtreeWithSelectors(
			rootNode,
			treeMap[key],
			key,
			primary,
			selectorByPath,
			hasNestedPrefix,
			emittedHostVerdict,
			&findings,
		)
	}
	return findings
}

// walkSubtreeWithSelectors dispatches relPath to the selector claiming it,
// descends container children when a nested selector claims a descendant of
// relPath (such as "traffic/control" or "system/conntrack"), or falls back to
// primary when no selector claims relPath or any descendant.
func walkSubtreeWithSelectors(
	node Node,
	data any,
	relPath string,
	primary backendWalkSelector,
	selectorByPath map[string]backendWalkSelector,
	hasNestedPrefix, emittedHostVerdict map[string]bool,
	findings *[]backendWalkFinding,
) {
	if data == nil {
		return
	}
	if sel, claimed := selectorByPath[relPath]; claimed {
		walkSubtreeUnderSelector(node, data, relPath, sel, emittedHostVerdict, findings)
		return
	}
	if hasNestedPrefix[relPath] {
		m, ok := data.(map[string]any)
		if !ok {
			walkSubtreeUnderSelector(node, data, relPath, primary, emittedHostVerdict, findings)
			return
		}
		var pb textbuf.Buffer
		for _, key := range sortedKeysAny(m) {
			childNode := childSchemaNode(node, key)
			if childNode == nil {
				continue
			}
			childRel := pb.Reset().Str(relPath).Str(PathSep).Str(key).String()
			walkSubtreeWithSelectors(
				childNode,
				m[key],
				childRel,
				primary,
				selectorByPath,
				hasNestedPrefix,
				emittedHostVerdict,
				findings,
			)
		}
		return
	}
	walkSubtreeUnderSelector(node, data, relPath, primary, emittedHostVerdict, findings)
}

// childSchemaNode returns the named child schema node of a container or flex
// node, or nil for other node kinds.
func childSchemaNode(node Node, key string) Node {
	switch n := node.(type) {
	case *ContainerNode:
		return n.Get(key)
	case *FlexNode:
		return n.Get(key)
	default:
		return nil
	}
}

// walkSubtreeUnderSelector runs walkBackendNode on one subtree under sel and
// appends every resulting finding.
func walkSubtreeUnderSelector(
	node Node,
	data any,
	relPath string,
	sel backendWalkSelector,
	emittedHostVerdict map[string]bool,
	findings *[]backendWalkFinding,
) {
	if !sel.valid {
		return
	}
	if sel.active == "" {
		if emittedHostVerdict[sel.leafPath] {
			return
		}
		emittedHostVerdict[sel.leafPath] = true
		*findings = append(*findings, backendWalkFinding{
			selector:    sel,
			hostVerdict: true,
		})
		return
	}
	var pb textbuf.Buffer
	pb.Byte('/').Str(relPath)
	initSteps := containerPathSteps(relPath)
	var mismatches []backendMismatch
	walkBackendNode(node, data, pb.String(), initSteps, sel.active, &mismatches)
	for _, m := range mismatches {
		*findings = append(*findings, backendWalkFinding{
			selector: sel,
			mismatch: m,
		})
	}
}

// containerPathSteps builds the initial container step slice for a
// PathSep-separated root path (for example, "interface" or "traffic/control").
func containerPathSteps(relPath string) []treePathStep {
	parts := SplitPath(strings.TrimPrefix(relPath, PathSep))
	steps := make([]treePathStep, len(parts))
	for i, seg := range parts {
		steps[i] = treePathStep{name: seg}
	}
	return steps
}

// appendTreeStep appends step to a non-aliasing copy of steps.
func appendTreeStep(steps []treePathStep, step treePathStep) []treePathStep {
	return append(slices.Clip(steps), step)
}

// withListEntryStep returns a copy of steps whose final step addresses key in
// the current list.
func withListEntryStep(steps []treePathStep, key string) []treePathStep {
	out := slices.Clone(steps)
	if len(out) == 0 {
		return out
	}
	out[len(out)-1].listKey = key
	out[len(out)-1].isListEntry = true
	return out
}

// backendMismatchRemovalPath turns a mismatch's structured tree steps into a
// concrete Tree.RemovePath target for start-time exclusion.
//
// A mismatched container or list entry removes that exact node. A mismatched
// leaf inside a list entry (such as a firewall rule leaf or a tunnel leaf)
// removes the nearest enclosing list entry so Ze never approximates a rule or
// tunnel by stripping one leaf. A leaf outside any list entry removes the leaf
// itself.
func backendMismatchRemovalPath(steps []treePathStep) string {
	if len(steps) == 0 {
		return ""
	}
	end := len(steps)
	if steps[len(steps)-1].isLeaf {
		for i := len(steps) - 2; i >= 0; i-- {
			if steps[i].isListEntry {
				end = i + 1
				break
			}
		}
	}
	var tb textbuf.Buffer
	for i, step := range steps[:end] {
		if i > 0 {
			tb.Str(PathSep)
		}
		tb.Str(step.name)
		if step.isListEntry {
			if step.listKey != "" {
				tb.Byte('[').Str(step.listKey).Byte(']')
			}
		}
	}
	return tb.String()
}

// validateBackendFeatures walks the parsed JSON config tree under
// componentRoot against the schema and returns one error per node whose
// `ze:backend` annotation excludes activeBackend, unless a descendant with
// its own annotation accepts activeBackend (narrowest-annotation-wins).
//
// tree MUST be the parsed config map. The walker descends into
// tree[componentRoot] only; other roots are ignored.
//
// activeBackend is the backend name selected at backendLeafPath (included
// in the error text so the user knows where to change it). An empty
// activeBackend returns a single error asking the user to configure a
// backend — the component layer typically rejects the empty case sooner.
//
// The returned slice is nil when the config conforms. Order matches
// depth-first tree walk; sibling keys are visited in sorted order so
// output is deterministic.
//
// Concurrent calls with distinct tree/errs slices are safe (schema is
// immutable post-build, and the walker mutates no package-level state).
// A single call is NOT re-entrant within itself: it appends to the
// caller's errs slice via pointer; do not share errs across calls.
func validateBackendFeatures(
	tree map[string]any,
	schema *Schema,
	componentRoot string,
	activeBackend string,
	backendLeafPath string,
) []error {
	if activeBackend == "" {
		// The callers pass the configured backend, else this OS's default, so
		// an empty one means this host has no default: a host verdict.
		return []error{diagnostic.HostVerdict(fmt.Errorf("backend feature gate: no backend configured at %s", backendLeafPath))}
	}
	if schema == nil {
		return nil
	}
	if tree == nil {
		return nil
	}
	// componentRoot may be a nested PathSep path (e.g. "traffic/control");
	// resolve both the schema node and the JSON subtree by walking each segment.
	rootNode, err := schema.Lookup(componentRoot)
	// refuse-by-default: guards walkBackendNode -- an unknown root carries no ze:backend annotation to evaluate
	if err != nil {
		return nil
	}
	if rootNode == nil {
		return nil
	}
	subtree, ok := treeAtPath(tree, componentRoot)
	// refuse-by-default: guards walkBackendNode -- an absent component subtree carries no feature to reject
	if !ok {
		return nil
	}

	var pb textbuf.Buffer
	pb.Byte('/').Str(componentRoot)
	var mismatches []backendMismatch
	walkBackendNode(rootNode, subtree, pb.String(), containerPathSteps(componentRoot), activeBackend, &mismatches)
	if len(mismatches) == 0 {
		return nil
	}
	errs := make([]error, len(mismatches))
	for i, m := range mismatches {
		errs[i] = formatBackendError(m.path, m.active, m.supporting)
	}
	return errs
}

// treeAtPath walks a parsed JSON config map along a PathSep-separated path,
// descending one map level per segment. Returns the leaf map and true when
// every segment resolves to a nested object; false otherwise.
func treeAtPath(tree map[string]any, path string) (map[string]any, bool) {
	cur := tree
	for _, seg := range SplitPath(path) {
		next, ok := cur[seg].(map[string]any)
		if !ok {
			return nil, false
		}
		cur = next
	}
	return cur, true
}

// walkBackendNode walks node alongside data. Returns two flags used to
// bubble information up to the caller so narrowest-annotation-wins applies
// to both accepts-override-rejects (AC-9) and reject-suppresses-outer-reject:
//
//   - spoke: this node OR some descendant has its own ze:backend annotation.
//     When true, a narrower annotation has already expressed a verdict for
//     the subtree; the outer node MUST NOT also emit a list-level or
//     container-level error (that would be double-reporting).
//
//   - accepts: this node's own annotation accepts activeBackend OR some
//     descendant's annotation accepts it. Used by list callers to decide
//     per-entry whether to suppress the list-level rejection.
//
// Lists are evaluated per-entry: if the list has a rejecting annotation,
// each entry is independently checked; an override inside entry A cannot
// mask a missing override inside entry B.
func walkBackendNode(node Node, data any, path string, steps []treePathStep, active string, mismatches *[]backendMismatch) (spoke, accepts bool) {
	var tb textbuf.Buffer
	if data == nil {
		return false, false
	}

	ownAnnotation, hasOwn := backendAnnotation(node)
	ownAccepts := hasOwn && slices.Contains(ownAnnotation, active)

	descSpoke := false
	descAccepts := false
	emittedAtList := false

	switch n := node.(type) {
	case *ContainerNode:
		if m, ok := data.(map[string]any); ok {
			for _, key := range sortedKeysAny(m) {
				childNode := n.Get(key)
				if childNode == nil {
					continue
				}
				childPath := tb.Reset().Str(path).Byte('/').Str(key).String()
				childSteps := appendTreeStep(steps, treePathStep{name: key})
				cs, ca := walkBackendNode(childNode, m[key], childPath, childSteps, active, mismatches)
				descSpoke = descSpoke || cs
				descAccepts = descAccepts || ca
			}
		}
	case *ListNode:
		descSpoke, descAccepts, emittedAtList = walkBackendListMap(n, hasOwn, ownAccepts, ownAnnotation, data, path, steps, active, mismatches)
	case *InlineListNode:
		descSpoke, descAccepts, emittedAtList = walkBackendInlineList(n, hasOwn, ownAccepts, ownAnnotation, data, path, steps, active, mismatches)
	case *FlexNode:
		if m, ok := data.(map[string]any); ok {
			for _, key := range sortedKeysAny(m) {
				childNode := n.Get(key)
				if childNode == nil {
					continue
				}
				childPath := tb.Reset().Str(path).Byte('/').Str(key).String()
				childSteps := appendTreeStep(steps, treePathStep{name: key})
				cs, ca := walkBackendNode(childNode, m[key], childPath, childSteps, active, mismatches)
				descSpoke = descSpoke || cs
				descAccepts = descAccepts || ca
			}
		}
	}

	// Narrowest-wins: only emit at this level if no narrower annotation
	// (self or descendant) already spoke for this subtree. A descendant
	// that emitted its own rejection has already informed the user, so
	// the outer annotation stays silent. Per-entry list emission is
	// accounted for via emittedAtList to avoid double-emitting from the
	// list helpers.
	if !emittedAtList && hasOwn && !ownAccepts && !descSpoke {
		mismatchSteps := slices.Clone(steps)
		if len(mismatchSteps) > 0 {
			if _, isLeaf := node.(*LeafNode); isLeaf {
				mismatchSteps[len(mismatchSteps)-1].isLeaf = true
			}
		}
		*mismatches = append(*mismatches, backendMismatch{
			path:       path,
			active:     active,
			supporting: ownAnnotation,
			steps:      mismatchSteps,
		})
	}

	return hasOwn || descSpoke, ownAccepts || descAccepts
}

// walkBackendListMap walks a ListNode's entries from either a map (keyed
// by list key value) or a JSON array. When the list itself has a rejecting
// annotation, each entry is checked independently: an entry whose
// descendants have not spoken (no own annotation anywhere in the subtree)
// inherits the list-level rejection and emits an error at its own path,
// while an entry whose descendants DID speak (accept or reject at a
// narrower level) is left to them — the list-level error is suppressed so
// the user sees the narrower diagnostic, not both.
//
// Returns (descSpoke, descAccepts, emittedAtList):
//   - descSpoke: some entry OR its descendants had their own annotation.
//   - descAccepts: some entry or descendant's annotation accepts active.
//   - emittedAtList: this function emitted at least one list-level error,
//     so the caller MUST NOT also emit a whole-list error on top.
func walkBackendListMap(
	list *ListNode,
	listHasOwn, listOwnAccepts bool,
	listAnnotation []string,
	data any,
	path string,
	steps []treePathStep,
	active string,
	mismatches *[]backendMismatch,
) (descSpoke, descAccepts, emittedAtList bool) {
	var tb textbuf.Buffer
	switch v := data.(type) {
	case map[string]any:
		for _, key := range sortedKeysAny(v) {
			entryPath := tb.Reset().Str(path).Byte('/').Str(key).String()
			entrySteps := withListEntryStep(steps, key)
			entrySpoke, entryAccepts := walkBackendListEntry(list, v[key], entryPath, entrySteps, active, mismatches)
			if entrySpoke {
				descSpoke = true
			}
			if entryAccepts {
				descAccepts = true
			}
			if emitListEntryError(listHasOwn, listOwnAccepts, listAnnotation, entryPath, entrySteps, active, entrySpoke, mismatches) {
				emittedAtList = true
			}
		}
	case []any:
		for i, item := range v {
			var bp textbuf.Buffer
			entryPath := bp.Reset().Str(path).Byte('[').Int(int64(i)).Byte(']').String()
			entrySteps := slices.Clone(steps)
			if itemMap, ok := item.(map[string]any); ok {
				if list.KeyName != "" {
					if rawKey, ok := itemMap[list.KeyName].(string); ok {
						if rawKey != "" {
							entrySteps = withListEntryStep(steps, rawKey)
						}
					}
				}
			}
			entrySpoke, entryAccepts := walkBackendListEntry(list, item, entryPath, entrySteps, active, mismatches)
			if entrySpoke {
				descSpoke = true
			}
			if entryAccepts {
				descAccepts = true
			}
			if emitListEntryError(listHasOwn, listOwnAccepts, listAnnotation, entryPath, entrySteps, active, entrySpoke, mismatches) {
				emittedAtList = true
			}
		}
	}
	return descSpoke, descAccepts, emittedAtList
}

// walkBackendInlineList mirrors walkBackendListMap for InlineListNode.
// InlineListNode data arrives as map[key]->entry (JSON object with the
// list's key value as the property name) -- same shape as ListNode's map
// form. Per-entry semantics match walkBackendListMap so the two list
// shapes behave identically under narrowest-wins.
func walkBackendInlineList(
	list *InlineListNode,
	listHasOwn, listOwnAccepts bool,
	listAnnotation []string,
	data any,
	path string,
	steps []treePathStep,
	active string,
	mismatches *[]backendMismatch,
) (descSpoke, descAccepts, emittedAtList bool) {
	var tb textbuf.Buffer
	m, ok := data.(map[string]any)
	if !ok {
		return false, false, false
	}
	for _, key := range sortedKeysAny(m) {
		entryPath := tb.Reset().Str(path).Byte('/').Str(key).String()
		entry, ok := m[key].(map[string]any)
		if !ok {
			continue
		}
		entrySteps := withListEntryStep(steps, key)
		entrySpoke, entryAccepts := walkInlineListEntryFields(list, entry, entryPath, entrySteps, active, mismatches)
		if entrySpoke {
			descSpoke = true
		}
		if entryAccepts {
			descAccepts = true
		}
		if emitListEntryError(listHasOwn, listOwnAccepts, listAnnotation, entryPath, entrySteps, active, entrySpoke, mismatches) {
			emittedAtList = true
		}
	}
	return descSpoke, descAccepts, emittedAtList
}

// walkBackendListEntry walks a single ListNode entry (the map that holds
// the entry's leaves and sub-containers). The entry's schema is the
// ListNode's child map, treated as an implicit ContainerNode for walking
// purposes. Returns (spoke, accepts) -- see walkBackendNode godoc.
func walkBackendListEntry(list *ListNode, entry any, path string, steps []treePathStep, active string, mismatches *[]backendMismatch) (spoke, accepts bool) {
	var tb textbuf.Buffer
	m, ok := entry.(map[string]any)
	if !ok {
		return false, false
	}
	for _, key := range sortedKeysAny(m) {
		childNode := list.Get(key)
		if childNode == nil {
			continue
		}
		childPath := tb.Reset().Str(path).Byte('/').Str(key).String()
		childSteps := appendTreeStep(steps, treePathStep{name: key})
		cs, ca := walkBackendNode(childNode, m[key], childPath, childSteps, active, mismatches)
		if cs {
			spoke = true
		}
		if ca {
			accepts = true
		}
	}
	return spoke, accepts
}

// walkInlineListEntryFields is the InlineListNode counterpart to
// walkBackendListEntry: iterates the entry's field data, dispatching each
// field to walkBackendNode against the inline list's child schema.
// Extracted so walkBackendInlineList stays symmetric with walkBackendListMap.
func walkInlineListEntryFields(list *InlineListNode, entry map[string]any, path string, steps []treePathStep, active string, mismatches *[]backendMismatch) (spoke, accepts bool) {
	var tb textbuf.Buffer
	for _, key := range sortedKeysAny(entry) {
		childNode := list.Get(key)
		if childNode == nil {
			continue
		}
		childPath := tb.Reset().Str(path).Byte('/').Str(key).String()
		childSteps := appendTreeStep(steps, treePathStep{name: key})
		cs, ca := walkBackendNode(childNode, entry[key], childPath, childSteps, active, mismatches)
		if cs {
			spoke = true
		}
		if ca {
			accepts = true
		}
	}
	return spoke, accepts
}

// emitListEntryError handles the per-entry list-level error emission shared
// between walkBackendListMap and walkBackendInlineList. Emits at entryPath
// when the list carries a rejecting annotation AND no narrower annotation
// inside the entry has already spoken. Returns true when it emitted so the
// caller can track whether a list-level error has already landed.
func emitListEntryError(
	listHasOwn, listOwnAccepts bool,
	listAnnotation []string,
	entryPath string,
	entrySteps []treePathStep,
	active string,
	entrySpoke bool,
	mismatches *[]backendMismatch,
) bool {
	if entrySpoke {
		return false
	}
	if !listHasOwn {
		return false
	}
	if listOwnAccepts {
		return false
	}
	*mismatches = append(*mismatches, backendMismatch{
		path:       entryPath,
		active:     active,
		supporting: listAnnotation,
		steps:      slices.Clone(entrySteps),
	})
	return true
}

// backendAnnotation returns the ze:backend list for a schema node plus a
// "has annotation" flag. Nil list with flag=true is impossible; either
// the node carries an annotation (flag=true, list non-empty) or it does
// not (flag=false, list nil). Covers every structural node type the
// schema walker can encounter; syntax-only leaf shapes (Freeform,
// multiLeaf, bracketLeafList, valueOrArray) are intentionally omitted
// because they have no children to gate and carrying a Backend field on
// them would be unused scaffolding.
func backendAnnotation(node Node) ([]string, bool) {
	switch n := node.(type) {
	case *LeafNode:
		if len(n.Backend) > 0 {
			return n.Backend, true
		}
	case *ContainerNode:
		if len(n.Backend) > 0 {
			return n.Backend, true
		}
	case *ListNode:
		if len(n.Backend) > 0 {
			return n.Backend, true
		}
	case *InlineListNode:
		if len(n.Backend) > 0 {
			return n.Backend, true
		}
	case *FlexNode:
		if len(n.Backend) > 0 {
			return n.Backend, true
		}
	}
	return nil, false
}

// formatBackendError renders one mismatch as a user-facing error. The path
// is the full YANG path of the rejected node, active is the chosen backend,
// supporting is the list the node declared. No Go symbols, no file paths.
func formatBackendError(path, active string, supporting []string) error {
	return fmt.Errorf(
		"%s: feature not supported by backend %q (supported: %s)",
		path, active, textbuf.Join(supporting, " "),
	)
}

// sortedKeysAny returns the keys of m in sorted order for deterministic walk.
func sortedKeysAny(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}
