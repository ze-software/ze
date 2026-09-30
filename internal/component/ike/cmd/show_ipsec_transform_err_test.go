// VALIDATES: show vpn ipsec sa reports an unresolved ESP transform as an error
// PREVENTS: an empty esp-integrity reading as a negotiated value
package cmd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/engine"
)

// TestSAToMapChildUnresolvedTransformSaysSo proves that when Info names
// ESPTransformErr, the child-sa object carries esp-transform-error with its text and
// no esp-integrity key, since Info left that field unset.
//
// Method: a PeerInfo in the shape Info produces on a failed resolve.
func TestSAToMapChildUnresolvedTransformSaysSo(t *testing.T) {
	peers := map[string]engine.PeerInfo{
		"peer-alpha": {
			PeerName:        "peer-alpha",
			HasChild:        true,
			ESPEncryption:   "aes128gcm",
			ESPTransformErr: crypto.ErrUnsupportedAlgorithm,
		},
	}

	row := saToMap(&engine.SA{PeerName: "peer-alpha", State: engine.StateEstablished},
		time.Now(), peers, sadCounters{})

	child, ok := row["child-sa"].(map[string]any)
	require.True(t, ok, "no child-sa object")
	require.Equal(t, crypto.ErrUnsupportedAlgorithm.Error(), child["esp-transform-error"])
	_, hasIntegrity := child["esp-integrity"]
	require.False(t, hasIntegrity, "esp-integrity rendered beside an unresolved transform")
}
