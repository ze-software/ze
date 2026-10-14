// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS client transport
// RFC: rfc/short/rfc3579.md -- RFC3579-3.3-2, one credential type in an Access-Request
// Related: client.go -- encodeRequest and oneCredentialType, the guard at the socket
// Related: rfc2869_credential_types_test.go -- credentialExchange, eapCredentialRequest

package radius

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRFC3579ARAPBesideEAPIsRefusedAtExchange drives the ARAP-Password pair of
// the exclusive credential rule through the public entry point.
//
// VALIDATES: Client.Exchange refuses an Access-Request carrying an EAP-Message
// beside an ARAP-Password (type 70, client.go credentialTypeARAPPassword), and
// the capture server reads no datagram.
// PREVENTS: the four-type set shrinking back to three: ze builds no
// ARAP-Password, so only a caller-supplied one reaches the guard, and only a
// test at the entry point shows the guard holds there.
//
// RFC requirement: RFC3579-3.3-2 negative -- an Access-Request carrying
// EAP-Message beside an ARAP-Password is refused by Client.Exchange with an
// error, and no datagram reaches the server (client.go encodeRequest,
// oneCredentialType).
func TestRFC3579ARAPBesideEAPIsRefusedAtExchange(t *testing.T) {
	arap := Attr{Type: credentialTypeARAPPassword, Value: make([]byte, 16)}
	datagrams, err := credentialExchange(t, func(id uint8) *Packet { return eapCredentialRequest(t, id, arap) })
	require.Error(t, err, "EAP-Message beside ARAP-Password")
	assert.Contains(t, err.Error(), "one credential type")
	assert.Empty(t, datagrams, "EAP-Message beside ARAP-Password reached the server")
}
