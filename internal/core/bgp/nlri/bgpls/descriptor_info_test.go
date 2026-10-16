// Design: docs/architecture/wire/nlri-bgpls.md — descriptor walker tests
// Related: descriptor_info.go — the walkers under test

package bgpls

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFormatRouterID verifies router ID formatting for different lengths.
//
// VALIDATES: Router ID formatted correctly for OSPF (4B), IS-IS (6/7B), pseudonode (8B).
// PREVENTS: Incorrect router ID display in JSON output.
func TestFormatRouterID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		id       []byte
		expected string
	}{
		{"ospf_4byte", []byte{1, 2, 3, 4}, "1.2.3.4"},
		{"isis_6byte", []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}, "010203040506"},
		{"isis_7byte_psn", []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, "01020304050607"},
		{"ospf_8byte_psn", []byte{1, 2, 3, 4, 5, 6, 7, 8}, "1.2.3.4,5.6.7.8"},
		{"unknown_3byte", []byte{0xAB, 0xCD, 0xEF}, "ABCDEF"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, formatRouterID(tt.id))
		})
	}
}
