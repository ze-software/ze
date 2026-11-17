// Design: docs/architecture/api/process-protocol.md -- payload-bound Peer identity.
package types

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestUpdateReceiptRequiresExactPeerOwner decodes the full numeric owner width
// and refuses malformed, absent or payload-rebound identities at the boundary.
func TestUpdateReceiptRequiresExactPeerOwner(t *testing.T) {
	const owner = uint64(18446744073709551615)
	for _, field := range []string{"", `,"peer-owner":null`, `,"peer-owner":-1`, `,"peer-owner":1.5`, `,"peer-owner":"7"`} {
		metadata := json.RawMessage(`{"peer":"192.0.2.1","message-id":42,"source-policy-epoch":9,"source-receipt":7,"reactor-forwarded":false` + field + `}`)
		_, err := ParseUpdateReceipt(metadata)
		require.Error(t, err)
	}
	for _, peerOwner := range []uint64{0, owner} {
		metadata := fmt.Appendf(nil, `{"peer":"192.0.2.1","peer-owner":%d,"message-id":42,"source-policy-epoch":9,"source-receipt":7,"reactor-forwarded":false}`, peerOwner)
		receipt, err := ParseUpdateReceipt(metadata)
		require.NoError(t, err)
		require.Equal(t, peerOwner, receipt.PeerOwner)
		require.NoError(t, receipt.Validate("192.0.2.1", peerOwner, 42, 9, 7))
		require.Error(t, receipt.Validate("192.0.2.1", peerOwner^1, 42, 9, 7))
	}
}
