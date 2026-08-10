// Design: docs/architecture/api/commands.md — output formatting
// Related: pipe.go — YAML pipe operator uses these

package command

import (
	"sort"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// formatNumber displays integers without decimal points.
// JSON unmarshals all numbers as float64; this restores integer display.
func formatNumber(v any) any {
	if n, ok := v.(float64); ok {
		if n == float64(int64(n)) {
			return int64(n)
		}
	}
	return v
}

// RenderYAML formats a parsed JSON value as valid YAML.
func RenderYAML(data any) string {
	var b textbuf.Buffer
	writeValue(&b, data, "")
	return b.String()
}

// writeValue recursively writes a JSON value as valid YAML with indentation.
func writeValue(b *textbuf.Buffer, v any, indent string) {
	switch val := v.(type) {
	case map[string]any:
		writeMap(b, val, indent)
	case []any:
		for _, item := range val {
			if m, ok := item.(map[string]any); ok {
				writeMapItem(b, m, indent)
			} else {
				b.Str(indent).Str("- ")
				writeScalar(b, item)
				b.Byte('\n')
			}
		}
	case nil:
		b.Str(indent).Str("null\n")
	case bool:
		b.Str(indent).Bool(val).Byte('\n')
	case string:
		b.Str(indent).Str(val).Byte('\n')
	case float64:
		b.Str(indent)
		writeScalar(b, formatNumber(val))
		b.Byte('\n')
	}
}

func writeScalar(b *textbuf.Buffer, v any) {
	switch s := v.(type) {
	case string:
		b.Str(s)
	case int64:
		b.Int(s)
	case float64:
		b.Float(s, -1)
	case bool:
		b.Bool(s)
	default:
		b.Str("null")
	}
}

// writeMap writes a map with sorted keys at the given indentation.
func writeMap(b *textbuf.Buffer, m map[string]any, indent string) {
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
func writeKeyValue(b *textbuf.Buffer, key string, value any, indent string) {
	var tb textbuf.Buffer
	deeper := tb.Str(indent).Str("  ").String()
	switch child := value.(type) {
	case map[string]any:
		b.Str(indent).Str(key).Str(":\n")
		writeMap(b, child, deeper)
	case []any:
		if len(child) == 0 {
			b.Str(indent).Str(key).Str(": []\n")
		} else {
			b.Str(indent).Str(key).Str(":\n")
			writeValue(b, child, deeper)
		}
	case nil:
		b.Str(indent).Str(key).Str(": null\n")
	case bool:
		b.Str(indent).Str(key).Str(": ").Bool(child).Byte('\n')
	case string:
		b.Str(indent).Str(key).Str(": ").Str(child).Byte('\n')
	case float64:
		b.Str(indent).Str(key).Str(": ")
		if child == float64(int64(child)) {
			b.Int(int64(child))
		} else {
			b.Float(child, -1)
		}
		b.Byte('\n')
	}
}

// writeMapItem writes a map as a YAML sequence item (first key on the "- " line).
func writeMapItem(b *textbuf.Buffer, m map[string]any, indent string) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var tb textbuf.Buffer
	dashIndent := tb.Str(indent).Str("- ").String()
	contIndent := tb.Reset().Str(indent).Str("  ").String()
	for i, key := range keys {
		if i == 0 {
			writeKeyValue(b, key, m[key], dashIndent)
		} else {
			writeKeyValue(b, key, m[key], contIndent)
		}
	}
}
