package naming

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateNodeName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		maxLen  int
		wantErr string
	}{
		{"lowercase", "default", 64, ""},
		{"with-hyphens", "firewall-3", 64, ""},
		{"numeric", "100", 64, ""},
		{"single-char", "a", 64, ""},
		{"uppercase", "MyPeer", 255, ""},
		{"mixed-case", "Spine-1", 255, ""},
		{"underscore-first", "_internal", 255, ""},
		{"underscore-mid", "my_peer", 255, ""},
		{"dot-mid", "peer.1", 255, ""},
		{"all-allowed", "Peer_name-1.2", 255, ""},
		{"max-length-exact", strings.Repeat("a", 64), 64, ""},
		{"too-long", strings.Repeat("a", 65), 64, "length"},
		{"empty", "", 64, "length"},
		{"space", "my unit", 64, "invalid character"},
		{"starts-with-hyphen", "-test", 64, "invalid character"},
		{"starts-with-dot", ".test", 64, "invalid character"},
		{"slash", "a/b", 64, "invalid character"},
		{"colon", "a:b", 64, "invalid character"},
		{"at-sign", "user@host", 64, "invalid character"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateNodeName("test", tt.input, tt.maxLen)
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}
