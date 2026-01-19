package ci

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestParseKVPairs verifies key=value parsing with colon handling.
//
// VALIDATES: ParseKVPairs correctly extracts key=value pairs.
// PREVENTS: Values containing colons being truncated.
func TestParseKVPairs(t *testing.T) {
	tests := []struct {
		name  string
		parts []string
		want  map[string]string
	}{
		{
			name:  "simple_pairs",
			parts: []string{"conn=1", "seq=2"},
			want:  map[string]string{"conn": "1", "seq": "2"},
		},
		{
			name:  "hex_with_colons",
			parts: []string{"conn=1", "seq=1", "hex=FF:FF:FF:FF"},
			want:  map[string]string{"conn": "1", "seq": "1", "hex": "FF:FF:FF:FF"},
		},
		{
			name:  "json_with_colons",
			parts: []string{"conn=1", "seq=1", `json={"type":"keepalive"}`},
			want:  map[string]string{"conn": "1", "seq": "1", "json": `{"type":"keepalive"}`},
		},
		{
			name:  "text_with_colons",
			parts: []string{"conn=1", "seq=1", "text=hello:world:foo"},
			want:  map[string]string{"conn": "1", "seq": "1", "text": "hello:world:foo"},
		},
		{
			name:  "pattern_with_colons",
			parts: []string{"pattern=level=DEBUG:subsystem=server"},
			want:  map[string]string{"pattern": "level=DEBUG:subsystem=server"},
		},
		{
			name:  "empty_parts",
			parts: []string{},
			want:  map[string]string{},
		},
		{
			name:  "empty_value",
			parts: []string{"key="},
			want:  map[string]string{"key": ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseKVPairs(tt.parts)
			assert.Equal(t, tt.want, got)
		})
	}
}
