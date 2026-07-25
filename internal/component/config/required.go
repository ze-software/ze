// Design: docs/architecture/config/syntax.md — ze:required enforcement
// Related: yang_schema.go — YANG-to-schema conversion (parses ze:required)
// Related: schema.go — ListNode.Required field

package config

import (
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// RequiredViolation reports a missing ze:required field on a list entry.
type RequiredViolation struct {
	AnchorPath string // schema path to the list (e.g., "bgp/peer")
	EntryKey   string // list entry key (e.g., "london")
	FieldPath  string // missing descendant path (e.g., "connection/remote/ip")
	SetHint    string // suggested set command
}

// CheckRequired walks the schema for ListNodes carrying ze:required and
// validates that each present config instance has the required descendant.
// Operates on map[string]any (the output of Tree.ToMap or ResolveBGPTree).
func CheckRequired(schema *Schema, data map[string]any) []RequiredViolation {
	if schema == nil || len(data) == 0 {
		return nil
	}
	var violations []RequiredViolation
	walkRequired(schema.root, data, "", &violations)
	return violations
}

func walkRequired(node Node, data map[string]any, path string, violations *[]RequiredViolation) {
	switch n := node.(type) {
	case *ContainerNode:
		for _, name := range n.order {
			child := n.children[name]
			childPath := appendRequiredPath(path, name)
			childData, ok := data[name].(map[string]any)
			if !ok {
				continue
			}
			walkRequired(child, childData, childPath, violations)
		}

	case *ListNode:
		checkListRequired(n, data, path, violations)
		for _, entryKey := range sortedAnyMapKeys(data) {
			entryData, ok := data[entryKey].(map[string]any)
			if !ok {
				continue
			}
			for _, name := range n.order {
				child := n.children[name]
				childPath := appendRequiredPath(path, name)
				childData, ok := entryData[name].(map[string]any)
				if !ok {
					continue
				}
				walkRequired(child, childData, childPath, violations)
			}
		}
	}
}

func checkListRequired(listNode *ListNode, data map[string]any, anchorPath string, violations *[]RequiredViolation) {
	if len(listNode.Required) == 0 {
		return
	}

	entryKeys := sortedAnyMapKeys(data)
	for _, entryKey := range entryKeys {
		entryData, ok := data[entryKey].(map[string]any)
		if !ok {
			continue
		}
		for _, reqPath := range listNode.Required {
			if len(reqPath) == 0 || reqPath[0] == "" {
				continue
			}
			if !hasNestedMapValue(entryData, reqPath) {
				fieldStr := textbuf.Join(reqPath, "/")
				var tb textbuf.Buffer
				setPath := tb.Str(anchorPath).Byte(' ').Str(entryKey).Byte(' ').Join(reqPath, " ").String()
				setPath = strings.ReplaceAll(setPath, "/", " ")
				*violations = append(*violations, RequiredViolation{
					AnchorPath: anchorPath,
					EntryKey:   entryKey,
					FieldPath:  fieldStr,
					SetHint:    tb.Reset().Str("set ").Str(setPath).Str(" <value>").String(),
				})
			}
		}
	}
}

func hasNestedMapValue(m map[string]any, path []string) bool {
	current := m
	for i, key := range path {
		val, exists := current[key]
		if !exists {
			return false
		}
		if i == len(path)-1 {
			s, ok := val.(string)
			return !ok || s != ""
		}
		next, ok := val.(map[string]any)
		if !ok {
			return false
		}
		current = next
	}
	return false
}

// HasNonBGPRequired reports whether any list outside the "bgp" subtree
// carries ze:required fields. Callers use this to skip the ToMap cost
// when no non-BGP required fields exist.
func HasNonBGPRequired(schema *Schema) bool {
	if schema == nil || schema.root == nil {
		return false
	}
	for _, name := range schema.root.order {
		if name == string(ConfigTypeBGP) {
			continue
		}
		if hasRequiredInSubtree(schema.root.children[name]) {
			return true
		}
	}
	return false
}

func hasRequiredInSubtree(node Node) bool {
	switch n := node.(type) {
	case *ContainerNode:
		for _, name := range n.order {
			if hasRequiredInSubtree(n.children[name]) {
				return true
			}
		}
	case *ListNode:
		if len(n.Required) > 0 {
			return true
		}
		for _, name := range n.order {
			if hasRequiredInSubtree(n.children[name]) {
				return true
			}
		}
	}
	return false
}

func appendRequiredPath(prefix, name string) string {
	if prefix == "" {
		return name
	}
	var tb textbuf.Buffer
	return tb.Str(prefix).Byte('/').Str(name).String()
}

func sortedAnyMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
