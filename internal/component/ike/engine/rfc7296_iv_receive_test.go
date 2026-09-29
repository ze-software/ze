// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- the Encrypted payload
// Related: rfc7296_encrypt_test.go -- TestSKAcceptsAnyIVOnReceipt, cbcSKPair
// VALIDATES: the receive path accepts the IVs a strict recipient would be tempted to refuse,
// because a sender MUST NOT choose them: a repeated IV and the previous message's final
// ciphertext block (RFC 7296 Section 3.14).
// PREVENTS: a recipient that polices the sender's IV rule and drops a message the RFC
// requires it to accept.

package engine

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/wire"
)

// TestRFC7296RecipientAcceptsAnIVTheSenderShouldNotHaveChosen pushes decryptAndParse toward
// refusing an IV.
//
// Goal: RFC 7296 Section 3.14 makes the IV unpredictable on the SENDER's side and makes
// the RECIPIENT accept any value. The IVs that break the sender's rule are exactly the
// ones a recipient that checked the rule would refuse. Method: three CBC messages under
// the same keys: one with IV X, one reusing X, and one whose IV is the previous message's
// final ciphertext block, the predictable choice. Each MUST decrypt to its Delete payload.
//
// RFC 7296 Section 3.14: "Senders MUST select a new unpredictable IV for every message;
// recipients MUST accept any value."
//
// RFC requirement: RFC7296-3.14-3 negative -- IVs that violate the sender's rule are still accepted on receipt: a message reusing the previous message's IV and a message whose IV is the previous message's final ciphertext block both decrypt to their Delete payload.
func TestRFC7296RecipientAcceptsAnIVTheSenderShouldNotHaveChosen(t *testing.T) {
	sa, peer := cbcSKPair(t)
	integTrunc := int(sa.Proposal.Integrity.TruncatedLength)
	if integTrunc == 0 {
		integTrunc = 16
	}
	del := &wire.PayloadDelete{ProtocolID: wire.ProtocolIKE}
	innerBuf := make([]byte, wire.GenericHeaderLen+del.Len())
	gh := wire.GenericHeader{Length: uint16(wire.GenericHeaderLen + del.Len())}
	gh.WriteTo(innerBuf, 0)
	del.WriteTo(innerBuf, wire.GenericHeaderLen)
	padLen := 16 - ((len(innerBuf) + 1) % 16)
	if padLen == 16 {
		padLen = 0
	}

	accept := func(name string, raw []byte) {
		t.Helper()
		inner, err := decryptAndParse(peer, parseMsg(t, raw), raw)
		if err != nil {
			t.Fatalf("%s: decryptAndParse = %v, want the message accepted", name, err)
		}
		if len(inner) != 1 {
			t.Fatalf("%s: recovered %d inner payloads, want 1", name, len(inner))
		}
		if _, ok := inner[0].Payload.(*wire.PayloadDelete); !ok {
			t.Fatalf("%s: inner payload = %T, want *wire.PayloadDelete", name, inner[0].Payload)
		}
	}

	iv := bytes.Repeat([]byte{0x5a}, 16)
	first := buildCBCSKWithIVAndPad(t, sa, innerBuf, 3, iv, padLen)
	accept("first message", first)
	second := buildCBCSKWithIVAndPad(t, sa, innerBuf, 4, iv, padLen)
	accept("a repeated IV", second)

	ctEnd := len(second) - integTrunc
	chained := append([]byte(nil), second[ctEnd-16:ctEnd]...)
	third := buildCBCSKWithIVAndPad(t, sa, innerBuf, 5, chained, padLen)
	accept("the previous final ciphertext block as IV", third)
}
