// Design: docs/architecture/api/process-protocol.md -- payload-bound Peer identity.
package bgp

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestParseEventBindsEnvelopePeerOwner drives the public event decoder with
// exact-width identities and refuses another incarnation at the same address.
func TestParseEventBindsEnvelopePeerOwner(t *testing.T) {
	envelope := func(payloadOwner uint64, ownerField string) []byte {
		return fmt.Appendf(nil, `{"namespace":"bgp","event":"update","payload":{"type":"bgp","bgp":{"peer-owner":%d,"peer":{"remote":{"address":"192.0.2.1","as":65001}},"message":{"type":"update","id":42,"direction":"received"},"source-policy-epoch":9,"source-receipt":7,"update":{}}},"metadata":{"peer":"192.0.2.1",%s"message-id":42,"source-policy-epoch":9,"source-receipt":7,"reactor-forwarded":false}}`, payloadOwner, ownerField)
	}
	for _, owner := range []uint64{0, ^uint64(0)} {
		field := fmt.Sprintf(`"peer-owner":%d,`, owner)
		event, err := ParseEvent(envelope(owner, field))
		require.NoError(t, err)
		require.Equal(t, owner, event.PeerOwner)
		_, err = ParseEvent(envelope(owner^1, field))
		require.Error(t, err, "the same address and message ID cannot rebind the Peer incarnation")
	}
	for _, field := range []string{"", `"peer-owner":null,`} {
		_, err := ParseEvent(envelope(701, field))
		require.Error(t, err)
	}
}
