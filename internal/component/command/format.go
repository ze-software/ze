// Design: docs/architecture/api/commands.md — output formatting
// Related: pipe.go — YAML pipe operator uses these

package command

import (
	"fmt"
	"sort"
	"strings"
)

// FormatNumber displays integers without decimal points.
// JSON unmarshals all numbers as float64; this restores integer display.
func FormatNumber(v any) any {
	if n, ok := v.(float64); ok {
		if n == float64(int64(n)) {
			return int64(n)
		}
	}
	return v
}

// RenderYAML formats a parsed JSON value as valid YAML.
func RenderYAML(data any) string {
	var b strings.Builder
	writeValue(&b, data, "")
	return b.String()
}

// writeValue recursively writes a JSON value as valid YAML with indentation.
func writeValue(b *strings.Builder, v any, indent string) {
	switch val := v.(type) {
	case map[string]any:
		writeMap(b, val, indent)
	case []any:
		for _, item := range val {
			if m, ok := item.(map[string]any); ok {
				writeMapItem(b, m, indent)
			} else {
				fmt.Fprintf(b, "%s- %v\n", indent, FormatNumber(item))
			}
		}
	case nil:
		fmt.Fprintf(b, "%snull\n", indent)
	case bool:
		fmt.Fprintf(b, "%s%v\n", indent, val)
	case string:
		fmt.Fprintf(b, "%s%s\n", indent, val)
	case float64:
		fmt.Fprintf(b, "%s%v\n", indent, FormatNumber(val))
	}
}

// writeMap writes a map with sorted keys at the given indentation.
func writeMap(b *strings.Builder, m map[string]any, indent string) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		writeKeyValue(b, key, m[key], indent)
	}
}

// writeKeyValue writes a single key-value pair with proper YAML formatting.
func writeKeyValue(b *strings.Builder, key string, value any, indent string) {
	switch child := value.(type) {
	case map[string]any:
		fmt.Fprintf(b, "%s%s:\n", indent, key)
		writeMap(b, child, indent+"  ")
	case []any:
		if len(child) == 0 {
			fmt.Fprintf(b, "%s%s: []\n", indent, key)
		} else {
			fmt.Fprintf(b, "%s%s:\n", indent, key)
			writeValue(b, child, indent+"  ")
		}
	case nil:
		fmt.Fprintf(b, "%s%s: null\n", indent, key)
	case bool:
		fmt.Fprintf(b, "%s%s: %v\n", indent, key, child)
	case string:
		fmt.Fprintf(b, "%s%s: %s\n", indent, key, child)
	case float64:
		fmt.Fprintf(b, "%s%s: %v\n", indent, key, FormatNumber(child))
	}
}

// writeMapItem writes a map as a YAML sequence item (first key on the "- " line).
func writeMapItem(b *strings.Builder, m map[string]any, indent string) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for i, key := range keys {
		if i == 0 {
			writeKeyValue(b, key, m[key], indent+"- ")
		} else {
			writeKeyValue(b, key, m[key], indent+"  ")
		}
	}
}
