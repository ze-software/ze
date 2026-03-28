// Design: docs/architecture/config/syntax.md — YANG schema default application
// Related: schema.go — schema node types (LeafNode, ContainerNode, ListNode)
// Related: yang_schema.go — YANG-to-schema conversion

package config

import (
	"fmt"
	"strconv"
)

// ApplyDefaults fills in missing leaf values from YANG schema defaults.
// Non-presence containers are created as needed to hold child defaults.
// Presence containers are only processed if already present in the map.
// List entries are not created; defaults are applied to existing entries only.
func ApplyDefaults(m map[string]any, node Node) {
	var names []string
	var get func(string) Node

	switch n := node.(type) { //nolint:gocritic // type switch required by linter
	case *ContainerNode:
		names = n.Children()
		get = n.Get
	case *ListNode:
		names = n.Children()
		get = n.Get
	case *LeafNode:
		return // nothing to recurse into
	}

	if get == nil {
		return
	}

	for _, name := range names {
		applyChildDefault(m, name, get(name))
	}
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

// SchemaDefault returns the YANG default value for a dot-separated schema path.
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
