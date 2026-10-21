// Design: docs/architecture/ospf/ospf-12-auth.md -- receive-side authentication.
// Related: auth_wiring.go -- verifyPacket, the dispatcher's authentication gate.
// Related: auth_keystore.go -- authStore.verify, the per-AuType check.
//
// VALIDATES: RFC 2328 Section 8.2, "The packet must be authenticated.", on the dispatcher a
// received packet really crosses: a packet whose AuType matches the area still has to pass
// the authentication procedure that AuType names before any handler sees it.
// PREVENTS: a receive path that checks the AuType field and then accepts any digest.
package ospf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// md5HelloFrom encodes a Hello from peer and signs it with keyed MD5 (AuType 2, Key ID 1)
// over secret, at cryptographic sequence number seq.
func md5HelloFrom(t *testing.T, eng *engine, secret string, seq uint64) []byte {
	t.Helper()
	peer := ridOf("10.0.0.2")
	hello := packet.Hello{HelloInterval: DefaultHelloInterval, Options: types.OptionE, Priority: 1, DeadInterval: uint32(DefaultDeadInterval)}
	p := packet.Packet{Header: packet.Header{Type: packet.PacketTypeHello, RouterID: peer, AreaID: eng.cfg.Areas[0].AreaID, AuType: packet.AuTypeCryptographic}, Hello: &hello}
	buf := make([]byte, p.EncodedLen())
	n := p.WriteTo(buf, 0)
	signed, err := packet.Sign(buf[:n], packet.AuTypeCryptographic, packet.AuthKey{KeyID: 1, Algorithm: packet.AuthMD5, Secret: []byte(secret)}, seq, [4]byte{})
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if got := packet.AuType(signed[15]); got != packet.AuTypeCryptographic {
		t.Fatalf("signed AuType = %d, want %d (cryptographic)", got, packet.AuTypeCryptographic)
	}
	return signed
}

// RFC requirement: RFC2328-8.2-4 positive -- on an interface whose area uses keyed-MD5 cryptographic authentication, a Hello whose MD5 digest was computed with the configured key passes authentication through the dispatcher: the dispatcher's drop counter does not move and the sender becomes a neighbor (verifyPacket, authStore.verify).
func TestRFC2328AuthenticatedPacketAccepted(t *testing.T) {
	eng, _, handle := receiveEngine(t)
	eng.auth.configure(authCfg(keyConfig{KeyID: 1, Algorithm: packet.AuthMD5, Secret: "goodkey"}))
	before := eng.dispatch.dropped()
	eng.dispatch.dispatch(transport.RawPacket{IfIndex: handle.ifindex, Src: netip.MustParseAddr("10.0.0.2"), Payload: md5HelloFrom(t, eng, "goodkey", 1)})
	if got := eng.dispatch.dropped(); got != before {
		t.Fatalf("dropped = %d, want %d (an authenticated Hello must not be dropped)", got, before)
	}
	if rows := eng.neighborSnapshot(); len(rows) != 1 {
		t.Fatalf("neighbor rows = %d, want 1 after an authenticated Hello", len(rows))
	}
}

// RFC requirement: RFC2328-8.2-4 negative -- on the same keyed-MD5 interface, a Hello whose AuType (2) matches the area but whose MD5 digest was computed with another key fails authentication and is dropped before any handler runs: the dispatcher's drop counter rises by one and no neighbor forms (verifyPacket, authStore.verify).
func TestRFC2328UnauthenticatedPacketDropped(t *testing.T) {
	eng, _, handle := receiveEngine(t)
	eng.auth.configure(authCfg(keyConfig{KeyID: 1, Algorithm: packet.AuthMD5, Secret: "goodkey"}))
	before := eng.dispatch.dropped()
	eng.dispatch.dispatch(transport.RawPacket{IfIndex: handle.ifindex, Src: netip.MustParseAddr("10.0.0.2"), Payload: md5HelloFrom(t, eng, "wrongkey", 1)})
	if got := eng.dispatch.dropped(); got != before+1 {
		t.Fatalf("dropped = %d, want %d (a Hello that fails authentication must be dropped)", got, before+1)
	}
	if rows := eng.neighborSnapshot(); len(rows) != 0 {
		t.Fatalf("neighbor rows = %d, want 0 after a Hello that fails authentication", len(rows))
	}
}
