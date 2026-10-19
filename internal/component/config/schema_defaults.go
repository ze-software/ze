// Design: docs/architecture/config/syntax.md — YANG schema default application
// Related: schema.go — schema node types (LeafNode, ContainerNode, ListNode)
// Related: yang_schema.go — YANG-to-schema conversion
// Related: yang/validator_default.go — CaseInEffect, the case-in-effect predicate
// RFC: rfc/short/rfc7950.md -- Sections 7.6.1 and 7.9.3, defaults in use

package config

import (
	"fmt"
	"strconv"

	gyang "github.com/openconfig/goyang/pkg/yang"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// ApplyDefaults fills in missing leaf values from YANG schema defaults.
// Non-presence containers are created as needed to hold child defaults.
// Presence containers are only processed if already present in the map.
// List entries are not created; defaults are applied to existing entries only.
// A child a YANG choice holds gets its defaults only while its case is in
// effect (choiceMembers.skipsDefaults).
func ApplyDefaults(m map[string]any, node Node) {
	var names []string
	var get func(string) Node
	var cases choiceMembers

	switch n := node.(type) { //nolint:gocritic // type switch required by linter
	case *ContainerNode:
		names = n.Children()
		get = n.Get
		cases = n.cases
	case *ListNode:
		names = n.Children()
		get = n.Get
		cases = n.cases
	case *LeafNode:
		return // nothing to recurse into
	}

	if get == nil {
		return
	}

	for _, name := range names {
		child := get(name)
		if cases.skipsDefaults(name, child, m) {
			continue
		}
		applyChildDefault(m, name, child)
	}
}

// choiceMembers maps the name of each child that a YANG choice holds to its
// goyang entry. The schema flattens a choice, so the case never appears as a
// node; the entry keeps its parent chain through the case and the choice,
// which is what yang.CaseInEffect reads. A nil map holds no member.
type choiceMembers map[string]*gyang.Entry

// record keeps e under name when a choice holds it: its parent is a case, or
// the choice itself for a shorthand case.
func (c *choiceMembers) record(name string, e *gyang.Entry) {
	if e == nil || e.Parent == nil {
		return
	}
	if !e.Parent.IsChoice() && !e.Parent.IsCase() {
		return
	}
	if *c == nil {
		*c = make(choiceMembers)
	}
	(*c)[name] = e
}

// adopt copies the entry of name from source, when source holds one, so a
// child that one module contributes to a merged node keeps its case.
func (c *choiceMembers) adopt(source choiceMembers, name string) {
	e, ok := source[name]
	if !ok {
		return
	}
	if *c == nil {
		*c = make(choiceMembers)
	}
	(*c)[name] = e
}

// RFC 7950 Section 7.6.1: "Otherwise, if this ancestor is a case node, the
// default value MUST be used if any node from the case exists in the data tree
// or the case node is the choice's default case, and if no nodes from any
// other case exist in the data tree."
//
// skipsDefaults reports whether ApplyDefaults must leave child, named name in
// data, alone because the case that holds it is not in effect. A leaf and a
// non-presence container have the case as their closest ancestor that is not
// a non-presence container, so the case decides for them. A presence container
// and a list entry are themselves that ancestor for what sits below them
// (Section 7.6.1, third branch), and ApplyDefaults fills them only when they
// already exist, so the case never decides for them.
func (c choiceMembers) skipsDefaults(name string, child Node, data map[string]any) bool {
	e, ok := c[name]
	if !ok {
		return false
	}
	switch n := child.(type) { //nolint:gocritic // type switch: only the kinds whose ancestor is not the case
	case *ContainerNode:
		if n.Presence {
			return false
		}
	case *ListNode:
		return false
	}
	return !yang.CaseInEffect(e, data)
}

// applyChildDefault applies a single schema child's default to the map.
func applyChildDefault(m map[string]any, name string, child Node) {
	switch c := child.(type) { //nolint:gocritic // type switch required by linter
	case *LeafNode:
		if _, exists := m[name]; !exists && c.Default != "" {
			m[name] = c.Default
		}
	case *ContainerNode:
		applyContainerDefault(m, name, c)
	case *ListNode:
		// Apply defaults to each existing list entry.
		if entries, ok := m[name].(map[string]any); ok {
			for _, v := range entries {
				if entry, ok := v.(map[string]any); ok {
					ApplyDefaults(entry, c)
				}
			}
		}
	}
}

// applyContainerDefault handles default application for container nodes.
func applyContainerDefault(m map[string]any, name string, c *ContainerNode) {
	if c.Presence {
		// Presence container: only apply defaults if already configured.
		if sub, ok := m[name].(map[string]any); ok {
			ApplyDefaults(sub, c)
		}
		return
	}
	// Non-presence: create if child defaults exist.
	sub, existed := m[name].(map[string]any)
	if !existed {
		sub = make(map[string]any)
	}
	before := len(sub)
	ApplyDefaults(sub, c)
	if len(sub) > before || existed {
		m[name] = sub
	}
}

// SchemaDefault returns the YANG default value for a schema path (e.g., "bgp/peer/timer/hold-time").
// Returns empty string if the path doesn't exist, is not a leaf, or has no default.
func SchemaDefault(schema *Schema, path string) string {
	if schema == nil {
		return ""
	}
	node, err := schema.Lookup(path)
	if err != nil {
		return ""
	}
	leaf, ok := node.(*LeafNode)
	if !ok {
		return ""
	}
	return leaf.Default
}

// SchemaDefaultInt returns the integer YANG default for a schema path.
// Returns error if the YANG default is missing or unparseable.
func SchemaDefaultInt(schema *Schema, path string) (int, error) {
	s := SchemaDefault(schema, path)
	if s == "" {
		return 0, fmt.Errorf("YANG default missing: %s", path)
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("YANG default for %s not integer: %w", path, err)
	}
	return v, nil
}

// SchemaDefaultBool returns the boolean YANG default for a schema path.
// Returns error if the YANG default is missing.
func SchemaDefaultBool(schema *Schema, path string) (bool, error) {
	s := SchemaDefault(schema, path)
	if s == "" {
		return false, fmt.Errorf("YANG default missing: %s", path)
	}
	return s == configTrue || s == "1", nil
}

// SchemaDefaultFloat64 returns the float64 YANG default for a schema path.
// Returns error if the YANG default is missing or unparseable.
func SchemaDefaultFloat64(schema *Schema, path string) (float64, error) {
	s := SchemaDefault(schema, path)
	if s == "" {
		return 0, fmt.Errorf("YANG default missing: %s", path)
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("YANG default for %s not float: %w", path, err)
	}
	return v, nil
}

// SchemaDefaultString returns the string YANG default for a schema path.
// Returns error if the YANG default is missing.
func SchemaDefaultString(schema *Schema, path string) (string, error) {
	s := SchemaDefault(schema, path)
	if s == "" {
		return "", fmt.Errorf("YANG default missing: %s", path)
	}
	return s, nil
}

// SchemaDefaultOctal returns the octal integer YANG default for a schema path.
// The YANG value is a string like "0137" parsed as octal.
// Returns error if the YANG default is missing or unparseable.
func SchemaDefaultOctal(schema *Schema, path string) (int, error) {
	s := SchemaDefault(schema, path)
	if s == "" {
		return 0, fmt.Errorf("YANG default missing: %s", path)
	}
	v, err := strconv.ParseInt(s, 8, 32)
	if err != nil {
		return 0, fmt.Errorf("YANG default for %s not octal: %w", path, err)
	}
	return int(v), nil
}
