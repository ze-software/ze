// Design: docs/architecture/ospf/ospf-12-auth.md -- cryptographic sequence number.
// Related: auth_wiring.go -- signPacket, the transport signer hook.
// Related: auth_keystore.go -- signKey, the per-interface send counter.
//
// VALIDATES: RFC 5709 Section 3.1, "Third, the 32-bit cryptographic sequence number is set in
// accordance with the procedures in RFC 2328, Appendix D", where RFC 2328 Section D.4.3 (5)
// sets it "to a non-decreasing value (i.e., a value at least as large as the last value sent
// out the interface)", on the engine's real signing hook.
// PREVENTS: a send counter that restarts or goes backwards when the signing key changes.
package ospf

import (
	"encoding/binary"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
)

// signedSequence signs one Hello through the engine's transport signer for eth0 and returns
// the Key ID and the 32-bit Cryptographic Sequence Number it carries.
func signedSequence(t *testing.T, eng *engine) (keyID byte, sequence uint32) {
	t.Helper()
	hello := packet.Packet{Header: packet.Header{Type: packet.PacketTypeHello}, Hello: &packet.Hello{NetworkMask: [4]byte{255, 255, 255, 0}, HelloInterval: 10, DeadInterval: 40}}
	buf := make([]byte, hello.EncodedLen())
	n := hello.WriteTo(buf, 0)
	signed := eng.signPacket("eth0", buf[:n])
	if signed == nil {
		t.Fatal("signPacket refused to sign a Hello on eth0")
	}
	if packet.AuType(binary.BigEndian.Uint16(signed[14:16])) != packet.AuTypeCryptographic {
		t.Fatalf("signed Hello carries AuType %d, want cryptographic", binary.BigEndian.Uint16(signed[14:16]))
	}
	// RFC 2328 Figure 18: the 8-octet auth field at offset 16 holds 0x0000, Key ID,
	// Auth Data Len, then the 32-bit Cryptographic sequence number.
	return signed[18], binary.BigEndian.Uint32(signed[20:24])
}

// TestRFC5709CryptoSequenceNonDecreasingOnInterface checks the sequence numbers eth0 sends.
// Goal: every packet the engine signs on an interface carries a Cryptographic Sequence
// Number at least as large as the last one sent there, including after a configuration
// reload switches the signing key. Method: configure an HMAC-SHA-256 key chain, sign three
// Hellos through signPacket, reload with a different key, sign two more, and compare the
// sequence field of each packet with the one before it.
func TestRFC5709CryptoSequenceNonDecreasingOnInterface(t *testing.T) {
	eng := newEngine(transport.New(&fakeBackend{}))
	defer eng.shutdown()
	eng.auth.configure(authCfg(keyConfig{KeyID: 1, Algorithm: "hmac-sha-256", Secret: "firstkey"}))

	// RFC requirement: RFC5709-3.1-3 positive -- five Hellos signed on eth0 through the
	// engine's signer carry a non-decreasing 32-bit Cryptographic Sequence Number, and the
	// two signed after a reload that changes the key from Key ID 1 to Key ID 2 are not below
	// the last one sent with Key ID 1.
	var last uint32
	for index := range 5 {
		if index == 3 {
			eng.auth.configure(authCfg(keyConfig{KeyID: 2, Algorithm: "hmac-sha-256", Secret: "secondkey"}))
		}
		keyID, sequence := signedSequence(t, eng)
		wantKey := byte(1)
		if index >= 3 {
			wantKey = 2
		}
		if keyID != wantKey {
			t.Fatalf("packet %d signed with Key ID %d, want %d", index, keyID, wantKey)
		}
		if index > 0 && sequence < last {
			t.Fatalf("packet %d carries sequence %d, below the %d sent before it on eth0", index, sequence, last)
		}
		last = sequence
	}
}
