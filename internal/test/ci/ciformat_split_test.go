package ci

import "testing"

// VALIDATES: a colon inside a value is preserved, while a colon that introduces
// a real `key=` token still splits.
// PREVENTS: the silent-truncation class found on 2026-07-29, where 203 `.ci`
// assertions across 15 suites asserted only the text before their first colon.
// `expect=stdout:contains=error: no such peer` asserted `error`, which passes on
// almost any output -- a test that cannot fail.
func TestParseKVPairsKeepsColonsInsideValues(t *testing.T) {
	// parts is what every caller passes: the segments AFTER `action=type`. A
	// leading type token here was a key-less segment the splitter used to drop,
	// and it now refuses one (TestParseKVPairsRefusesMalformedSegments).
	cases := []struct {
		name  string
		parts []string
		want  map[string]string
	}{
		{
			name:  "prose colon is part of the value",
			parts: []string{"contains=error", " no such peer"},
			want:  map[string]string{"contains": "error: no such peer"},
		},
		{
			name:  "directive colon still splits",
			parts: []string{"contains=aes-cbc", "timeout=25"},
			want:  map[string]string{"contains": "aes-cbc", "timeout": "25"},
		},
		{
			name:  "json wire needle survives whole",
			parts: []string{`contains="resultType"`, `"task"`},
			want:  map[string]string{"contains": `"resultType":"task"`},
		},
		{
			name:  "url in a value is not split",
			parts: []string{"contains=http", "//host", "8080/p"},
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
			got, err := ParseKVPairs(tc.parts)
			if err != nil {
				t.Fatalf("ParseKVPairs(%q): %v", tc.parts, err)
			}
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
	got, err := ParseKVPairs([]string{"contains=name", " peer1"})
	if err != nil {
		t.Fatal(err)
	}
	if got["contains"] != "name: peer1" {
		t.Fatalf("truncated: contains = %q, want %q", got["contains"], "name: peer1")
	}
}
