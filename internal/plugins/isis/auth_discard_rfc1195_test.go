// Design: docs/architecture/isis/isis-10-auth.md -- receive-path authentication
// Related: auth_wiring.go -- verifyFrame, the dispatcher's verify hook
// Related: server.go -- dispatcher.dispatch, which drops a frame the hook refuses

package isis

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
)

// lspAuthDigestOffset is the offset of the first CRYPTO_AUTH digest octet in a
// signed LSP: the 27-octet LSP header, the TLV 10 type and length octets, the
// authentication type octet and the 2-octet Key ID (RFC 5310 sec 3).
const lspAuthDigestOffset = 27 + 2 + 1 + 2

// VALIDATES: a Level 1 LSP whose CRYPTO_AUTH digest does not match is discarded
// whole at the receive path: the dispatcher never hands it to the LSP handler, so
// no LSDB, adjacency or SNP state can see it. The same LSP with its digest intact
// reaches the handler exactly once.
// PREVENTS: a receive path that verifies, ignores the result, and processes the PDU.
// Method: the engine's own verify hook (setKeyStore) guards the real dispatcher; a
// recording handler replaces the L1 LSP handler, so delivery is observed directly.
//
// RFC requirement: RFC1195-3.9-1 positive -- an LSP with valid authentication information passes dispatcher.dispatch and reaches the L1 LSP handler once.
// RFC requirement: RFC1195-3.9-1 negative -- the same LSP with one digest octet changed is discarded entirely: the L1 LSP handler is never called.
// RFC requirement: RFC5310-3.5-1 positive -- a CRYPTO_AUTH LSP whose calculated data matches the received data is delivered to the L1 LSP handler.
// RFC requirement: RFC5310-3.5-1 negative -- a CRYPTO_AUTH LSP whose calculated data does not match the received data is discarded before the L1 LSP handler.
func TestRFC1195InvalidAuthLSPDiscardedAtReceive(t *testing.T) {
	e := newEngine(transport.New(transport.NewBackend()))
	e.setKeyStore(authTestConfig())

	delivered := 0
	e.dispatch.register(packet.PDUTypeL1LSP, func(transport.RawFrame) { delivered++ })

	signed := e.signLevelPDU(authTestLSP(levelOne))
	if signed[27] != byte(packet.TLVAuthentication) {
		t.Fatalf("octet 27 = %d, want TLV 10 first after signing", signed[27])
	}

	e.dispatch.dispatch(transport.RawFrame{IfIndex: 10, PDU: signed})
	if delivered != 1 {
		t.Fatalf("valid LSP delivered %d times, want 1", delivered)
	}

	forged := append([]byte(nil), signed...)
	forged[lspAuthDigestOffset] ^= 0x01
	e.dispatch.dispatch(transport.RawFrame{IfIndex: 10, PDU: forged})
	if delivered != 1 {
		t.Fatalf("LSP with invalid authentication reached the handler (deliveries %d, want 1)", delivered)
	}
}
