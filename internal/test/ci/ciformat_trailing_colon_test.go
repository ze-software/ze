// Design: docs/architecture/testing/ci-format.md -- literal assertion values.
// Related: ciformat.go -- the shared directive key/value parser.

package ci

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestParseKVPairsPreservesTrailingValueColons distinguishes literal value
// colons from the separator left when a complex field is extracted. In
// particular, the namespace needle ze-system: must not become ze-system and
// reject the legitimate ze-system-dns namespace.
func TestParseKVPairsPreservesTrailingValueColons(t *testing.T) {
	cases := []struct {
		name string
		line string
		want map[string]string
	}{
		{
			name: "namespace colon",
			line: `contains="wire-method": "ze-system:`,
			want: map[string]string{"contains": `"wire-method": "ze-system:`},
		},
		{
			name: "two literal colons",
			line: "contains=ready::",
			want: map[string]string{"contains": "ready::"},
		},
		{
			name: "colon is the whole value",
			line: "contains=:",
			want: map[string]string{"contains": ":"},
		},
		{
			name: "ordinary value",
			line: "contains=ready:timeout=25",
			want: map[string]string{"contains": "ready", "timeout": "25"},
		},
		{
			name: "literal colon before another key",
			line: "contains=ready::timeout=25",
			want: map[string]string{"contains": "ready:", "timeout": "25"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseKVPairs(strings.Split(tc.line, ":"))
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestParseKVPairsComplexSeparatorPreservesValueColons requires every complex
// field to keep its complete value while consuming only its own separator.
// A literal colon immediately before that separator belongs to the preceding
// value, and removing it would broaden that assertion too.
func TestParseKVPairsComplexSeparatorPreservesValueColons(t *testing.T) {
	for _, key := range []string{"json", "text", "hex", "pattern"} {
		t.Run(key, func(t *testing.T) {
			for _, prefix := range []struct {
				name string
				line string
				want map[string]string
			}{
				{"alone", "", map[string]string{key: "value:"}},
				{"after ordinary pair", "seq=1:", map[string]string{"seq": "1", key: "value:"}},
				{"after literal colon", "contains=ready::", map[string]string{"contains": "ready:", key: "value:"}},
			} {
				t.Run(prefix.name, func(t *testing.T) {
					got, err := ParseKVPairs(strings.Split(prefix.line+key+"=value:", ":"))
					require.NoError(t, err)
					require.Equal(t, prefix.want, got)
				})
			}
		})
	}
}
