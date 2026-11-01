package labeled_test

import (
	"encoding/hex"
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	labeled "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/labeled"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
)

// TestLabeledUnicastAppendJSONMatchesDecoder checks that the in-process JSON
// writer and the RPC hex decoder give one labeled-unicast NLRI the same JSON,
// the RFC 7911 Path Identifier included when the route carries one.
//
// VALIDATES: AppendJSON writes "path-id" under ADD-PATH, zero included, and
// omits it without ADD-PATH, byte for byte as DecodeNLRIHex's output marshals.
// PREVENTS: a labeled ADD-PATH route reaching the UPDATE event and the
// policy `prefix` field with no identifier, so no prefix remove can name it.
func TestLabeledUnicastAppendJSONMatchesDecoder(t *testing.T) {
	t.Parallel()
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel}
	tests := []struct {
		name    string
		pathID  uint32
		addPath bool
		want    string
	}{
		{"no ADD-PATH", 0, false, `{"labels":[[100,1601]],"prefix":"10.0.0.0/24"}`},
		{"ADD-PATH identifier 7", 7, true, `{"labels":[[100,1601]],"path-id":7,"prefix":"10.0.0.0/24"}`},
		{"ADD-PATH identifier 0", 0, true, `{"labels":[[100,1601]],"path-id":0,"prefix":"10.0.0.0/24"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			n := labeled.NewLabeledUnicast(fam, netip.MustParsePrefix("10.0.0.0/24"), []uint32{100}, tt.pathID, tt.addPath)
			got := string(n.AppendJSON(nil))
			assert.Equal(t, tt.want, got, "AppendJSON")

			wire := make([]byte, nlri.LenWithContext(n, tt.addPath))
			nlri.WriteNLRI(n, wire, 0, tt.addPath)
			decoded, err := labeled.DecodeNLRIHex(fam.String(), hex.EncodeToString(wire), tt.addPath, false)
			require.NoError(t, err)
			marshaled, err := json.Marshal(decoded)
			require.NoError(t, err)
			assert.Equal(t, string(marshaled), got, "AppendJSON must match the decoder")
		})
	}
}
