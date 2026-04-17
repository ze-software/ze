// Design: docs/architecture/core-design.md — ze:backend commit-time feature gate
// Related: schema.go — Backend field on LeafNode/ContainerNode/ListNode
// Related: yang_schema.go — getBackendExtension reader and schema population
//
// Package config — backend feature gate.
//
// The gate walks a parsed JSON config tree alongside the schema and rejects
// configs that exercise features the active backend does not implement. A
// feature's support matrix is declared via `ze:backend "<names>"` on the
// relevant YANG node. Absent annotation means unrestricted (every registered
// backend is accepted).
//
// The gate is called by component plugins (iface today; fw-3 / fw-5 later)
// before Apply runs. A failure propagates back to the user as the commit
// rejection message. `ze config validate` (offline CLI) uses the same helper
// so validation matches daemon commit.

package config

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
)

// ValidateBackendFeatures walks the parsed JSON config tree under
// componentRoot against the schema and returns one error per node whose
// `ze:backend` annotation excludes activeBackend, unless a descendant with
// its own annotation accepts activeBackend (narrowest-annotation-wins).
//
// tree MUST be the JSON root delivered to the plugin (e.g.
// unmarshalled sdk.ConfigSection.Data). The walker descends into
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
func ValidateBackendFeatures(
	tree map[string]any,
	schema *Schema,
	componentRoot string,
	activeBackend string,
	backendLeafPath string,
) []error {
	if activeBackend == "" {
		return []error{fmt.Errorf("backend feature gate: no backend configured at %s", backendLeafPath)}
	}
	if schema == nil || tree == nil {
		return nil
	}
	rootNode := schema.Get(componentRoot)
	if rootNode == nil {
		return nil
	}
	subtree, ok := tree[componentRoot].(map[string]any)
	if !ok {
		return nil
	}

	var errs []error
	walkBackendNode(rootNode, subtree, "/"+componentRoot, activeBackend, &errs)
	return errs
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
func walkBackendNode(node Node, data any, path, active string, errs *[]error) (spoke, accepts bool) {
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
				childPath := path + "/" + key
				cs, ca := walkBackendNode(childNode, m[key], childPath, active, errs)
				descSpoke = descSpoke || cs
				descAccepts = descAccepts || ca
			}
		}
	case *ListNode:
		descSpoke, descAccepts, emittedAtList = walkBackendListMap(n, hasOwn, ownAccepts, ownAnnotation, data, path, active, errs)
	case *InlineListNode:
		descSpoke, descAccepts, emittedAtList = walkBackendInlineList(n, hasOwn, ownAccepts, ownAnnotation, data, path, active, errs)
	case *FlexNode:
		if m, ok := data.(map[string]any); ok {
			for _, key := range sortedKeysAny(m) {
				childNode := n.Get(key)
				if childNode == nil {
					continue
				}
				childPath := path + "/" + key
				cs, ca := walkBackendNode(childNode, m[key], childPath, active, errs)
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
		*errs = append(*errs, formatBackendError(path, active, ownAnnotation))
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
	path, active string,
	errs *[]error,
) (descSpoke, descAccepts, emittedAtList bool) {
	switch v := data.(type) {
	case map[string]any:
		for _, key := range sortedKeysAny(v) {
			entryPath := path + "/" + key
			entrySpoke, entryAccepts := walkBackendListEntry(list, v[key], entryPath, active, errs)
			if entrySpoke {
				descSpoke = true
			}
			if entryAccepts {
				descAccepts = true
			}
			if emitListEntryError(listHasOwn, listOwnAccepts, listAnnotation, entryPath, active, entrySpoke, errs) {
				emittedAtList = true
			}
		}
	case []any:
		for i, item := range v {
			entryPath := fmt.Sprintf("%s[%d]", path, i)
			entrySpoke, entryAccepts := walkBackendListEntry(list, item, entryPath, active, errs)
			if entrySpoke {
				descSpoke = true
			}
			if entryAccepts {
				descAccepts = true
			}
			if emitListEntryError(listHasOwn, listOwnAccepts, listAnnotation, entryPath, active, entrySpoke, errs) {
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
	path, active string,
	errs *[]error,
) (descSpoke, descAccepts, emittedAtList bool) {
	m, ok := data.(map[string]any)
	if !ok {
		return false, false, false
	}
	for _, key := range sortedKeysAny(m) {
		entryPath := path + "/" + key
		entry, ok := m[key].(map[string]any)
		if !ok {
			continue
		}
		entrySpoke, entryAccepts := walkInlineListEntryFields(list, entry, entryPath, active, errs)
		if entrySpoke {
			descSpoke = true
		}
		if entryAccepts {
			descAccepts = true
		}
		if emitListEntryError(listHasOwn, listOwnAccepts, listAnnotation, entryPath, active, entrySpoke, errs) {
			emittedAtList = true
		}
	}
	return descSpoke, descAccepts, emittedAtList
}

// walkBackendListEntry walks a single ListNode entry (the map that holds
// the entry's leaves and sub-containers). The entry's schema is the
// ListNode's child map, treated as an implicit ContainerNode for walking
// purposes. Returns (spoke, accepts) -- see walkBackendNode godoc.
func walkBackendListEntry(list *ListNode, entry any, path, active string, errs *[]error) (spoke, accepts bool) {
	m, ok := entry.(map[string]any)
	if !ok {
		return false, false
	}
	for _, key := range sortedKeysAny(m) {
		childNode := list.Get(key)
		if childNode == nil {
			continue
		}
		childPath := path + "/" + key
		cs, ca := walkBackendNode(childNode, m[key], childPath, active, errs)
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
func walkInlineListEntryFields(list *InlineListNode, entry map[string]any, path, active string, errs *[]error) (spoke, accepts bool) {
	for _, key := range sortedKeysAny(entry) {
		childNode := list.Get(key)
		if childNode == nil {
			continue
		}
		cs, ca := walkBackendNode(childNode, entry[key], path+"/"+key, active, errs)
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
	entryPath, active string,
	entrySpoke bool,
	errs *[]error,
) bool {
	if entrySpoke || !listHasOwn || listOwnAccepts {
		return false
	}
	*errs = append(*errs, formatBackendError(entryPath, active, listAnnotation))
	return true
}

// backendAnnotation returns the ze:backend list for a schema node plus a
// "has annotation" flag. Nil list with flag=true is impossible; either
// the node carries an annotation (flag=true, list non-empty) or it does
// not (flag=false, list nil). Covers every structural node type the
// schema walker can encounter; syntax-only leaf shapes (Freeform,
// MultiLeaf, BracketLeafList, ValueOrArray) are intentionally omitted
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
		path, active, strings.Join(supporting, " "),
	)
}

// sortedKeysAny returns the keys of m in sorted order for deterministic walk.
func sortedKeysAny(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ValidateBackendFeaturesJSON is a convenience wrapper that accepts the raw
// JSON string delivered in sdk.ConfigSection.Data. Components that already
// have a parsed map[string]any should call ValidateBackendFeatures directly.
//
// Returns a single error when the JSON fails to parse; otherwise delegates
// to ValidateBackendFeatures.
func ValidateBackendFeaturesJSON(
	data string,
	schema *Schema,
	componentRoot string,
	activeBackend string,
	backendLeafPath string,
) []error {
	if strings.TrimSpace(data) == "" {
		return nil
	}
	var root map[string]any
	if err := json.Unmarshal([]byte(data), &root); err != nil {
		return []error{fmt.Errorf("backend feature gate: parse config JSON: %w", err)}
	}
	return ValidateBackendFeatures(root, schema, componentRoot, activeBackend, backendLeafPath)
}
