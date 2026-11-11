// Design: docs/architecture/ike/ipsec-14-responder.md -- unauthenticated input during the handshake, both roles
// Related: protected_input.go -- protectedPayload and openProtected, the producers these tests hold

package engine

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/wire"
)

// VALIDATES: a message whose chain holds two Encrypted payloads is opened by its first
// one, the one every opener takes.
//
// METHOD: a 52-octet message is built by hand: the fixed header, an Encrypted payload of
// eight 0x11 octets whose Next Payload is 46, and a second Encrypted payload of eight
// 0x22 octets. wire.Message.ReadFrom parses both, and protectedPayload must answer the
// first.
//
// PREVENTS: two openers taking different payloads of one message (the initiator
// handlers took the last, decryptAndParse the first). RFC 7296 Section 3.14 makes the
// Encrypted payload the last payload, and ReadFrom does not enforce it, so the choice is
// the opener's and is pinned here.
func TestProtectedPayloadTakesTheFirstEncryptedPayload(t *testing.T) {
	const bodyOctets = 8
	payloadOctets := wire.GenericHeaderLen + bodyOctets
	total := wire.HeaderLen + 2*payloadOctets
	raw := make([]byte, total)
	raw[16] = wire.PayloadTypeSK
	raw[17] = 0x20 // Version 2.0.
	raw[18] = wire.ExchangeIKEAuth
	raw[19] = wire.FlagResponse
	binary.BigEndian.PutUint32(raw[24:28], uint32(total))
	first := wire.HeaderLen
	raw[first] = wire.PayloadTypeSK
	binary.BigEndian.PutUint16(raw[first+2:first+4], uint16(payloadOctets))
	copy(raw[first+wire.GenericHeaderLen:], bytes.Repeat([]byte{0x11}, bodyOctets))
	second := first + payloadOctets
	binary.BigEndian.PutUint16(raw[second+2:second+4], uint16(payloadOctets))
	copy(raw[second+wire.GenericHeaderLen:], bytes.Repeat([]byte{0x22}, bodyOctets))

	msg := parseMsg(t, raw)
	if len(msg.Payloads) != 2 {
		t.Fatalf("ReadFrom parsed %d payloads, want the 2 Encrypted payloads", len(msg.Payloads))
	}
	sk := protectedPayload(msg)
	if sk == nil {
		t.Fatal("protectedPayload found no Encrypted payload")
	}
	if !bytes.Equal(sk.CipherText, bytes.Repeat([]byte{0x11}, bodyOctets)) {
		t.Fatalf("protectedPayload took the Encrypted payload holding % x, want the first", sk.CipherText)
	}
}
