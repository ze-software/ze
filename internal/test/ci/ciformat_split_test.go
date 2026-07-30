package ci

import "testing"

// VALIDATES: a colon inside a value is preserved, while a colon that introduces
// a real `key=` token still splits.
// PREVENTS: the silent-truncation class found on 2026-07-29, where 203 `.ci`
// assertions across 15 suites asserted only the text before their first colon.
// `expect=stdout:contains=error: no such peer` asserted `error`, which passes on
// almost any output -- a test that cannot fail.
func TestParseKVPairsKeepsColonsInsideValues(t *testing.T) {
	cases := []struct {
		name  string
		parts []string
		want  map[string]string
	}{
		{
			name:  "prose colon is part of the value",
			parts: []string{"stdout", "contains=error", " no such peer"},
			want:  map[string]string{"contains": "error: no such peer"},
		},
		{
			name:  "directive colon still splits",
			parts: []string{"output", "contains=aes-cbc", "timeout=25"},
			want:  map[string]string{"contains": "aes-cbc", "timeout": "25"},
		},
		{
			name:  "json wire needle survives whole",
			parts: []string{"stdout", `contains="resultType"`, `"task"`},
			want:  map[string]string{"contains": `"resultType":"task"`},
		},
		{
			name:  "url in a value is not split",
			parts: []string{"stdout", "contains=http", "//host", "8080/p"},
			want:  map[string]string{"contains": "http://host:8080/p"},
		},
		{
			name:  "plain pairs unchanged",
			parts: []string{"conn=1", "seq=2", "hex=DEADBEEF"},
			want:  map[string]string{"conn": "1", "seq": "2", "hex": "DEADBEEF"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseKVPairs(tc.parts)
			for k, want := range tc.want {
				if got[k] != want {
					t.Errorf("key %q = %q, want %q (full: %v)", k, got[k], want, got)
				}
			}
		})
	}
}

// Exactly the argument record_parse.go:280 passes for the real directive
// `expect=stdout:contains=name: peer1` (parts[1:] after the line is split).
func TestParseKVPairsRealWorldExpectLine(t *testing.T) {
	got := ParseKVPairs([]string{"contains=name", " peer1"})
	if got["contains"] != "name: peer1" {
		t.Fatalf("truncated: contains = %q, want %q", got["contains"], "name: peer1")
	}
}
