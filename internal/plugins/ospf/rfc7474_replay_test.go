// Design: docs/architecture/ospf/ospf-12-auth.md -- OSPFv2 cryptographic authentication and replay.
// Related: auth_keystore.go -- authStore.verify, the receive-side check these tests drive.
//
// VALIDATES: RFC 7474 section 2 receive rules on AuType 3 (extended sequence numbers): a packet
// is accepted only when its 64-bit sequence, boot count in the high word, is greater than the
// last accepted packet of that type from that neighbor; otherwise it is dropped as a replay.
// PREVENTS: a comparison that ignores the boot-count word, a mark shared across neighbors, and a
// mark shared across packet types.
package ospf

import (
	"context"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	"github.com/ze-software/ze/pkg/zefs"
)

// rfc7474Store is an authStore whose backbone interface eth0 uses an extended-sequence
// (AuType 3) HMAC-SHA-256 key chain.
func rfc7474Store() *authStore {
	s := newAuthStore()
	s.configure(ospfConfig{
		KeyChains:  []keyChainConfig{{Name: "kc1", ExtendedSequence: true, Keys: []keyConfig{{KeyID: 1, Algorithm: "hmac-sha-256", Secret: "k"}}}},
		Areas:      []areaConfig{{AreaID: types.BackboneArea, AuthKeyChain: "kc1"}},
		Interfaces: []interfaceConfig{{Name: "eth0", AreaID: types.BackboneArea, Authentication: authConfig{Mode: "inherit"}}},
	})
	return s
}

// rfc7474Packet signs one AuType 3 packet of the given type with the 64-bit sequence
// boot<<32 | low.
func rfc7474Packet(t *testing.T, pktType packet.PacketType, boot, low uint32) []byte {
	t.Helper()
	p := packet.Packet{Header: packet.Header{Type: pktType, AuType: packet.AuTypeCryptographicESN}}
	switch pktType {
	case packet.PacketTypeHello:
		p.Hello = &packet.Hello{NetworkMask: [4]byte{255, 255, 255, 0}, HelloInterval: 10, DeadInterval: 40}
	case packet.PacketTypeLSAck:
		p.LSAck = &packet.LSAck{}
	default:
		t.Fatalf("rfc7474Packet: packet type %d not built by this helper", pktType)
	}
	buf := make([]byte, p.EncodedLen())
	n := p.WriteTo(buf, 0)
	key := packet.AuthKey{KeyID: 1, Algorithm: "hmac-sha-256", Secret: []byte("k")}
	signed, err := packet.Sign(buf[:n], packet.AuTypeCryptographicESN, key, uint64(boot)<<32|uint64(low), [4]byte{})
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	return signed
}

// rfc7474Accept verifies wire from rid on eth0 and fails the test unless the verdict is want.
// wantReason is checked when the packet is dropped.
func rfc7474Accept(t *testing.T, s *authStore, rid types.RouterID, wire []byte, want bool, what string) {
	t.Helper()
	reason, ok := s.verify("eth0", rid, [4]byte{}, wire)
	if ok != want {
		t.Fatalf("%s: accepted = %v (reason %q), want %v", what, ok, reason, want)
	}
	if !want && reason != "replay" {
		t.Fatalf("%s: drop reason = %q, want replay", what, reason)
	}
}

// RFC requirement: RFC7474-2-5 positive -- AuType 3 Hellos from one neighbor are accepted while
// each 64-bit sequence is greater than the last accepted: boot 1 low 5, then boot 1 low 6, then
// boot 2 low 1 after a restart, whose low word is smaller but whose boot count is greater.
func TestRFC7474GreaterExtendedSequenceAccepted(t *testing.T) {
	// Goal: "greater" is the 64-bit comparison, boot count first. Method: three increasing
	// sequences, the last with a lower low word.
	s := rfc7474Store()
	peer := ridOf("2.2.2.2")
	rfc7474Accept(t, s, peer, rfc7474Packet(t, packet.PacketTypeHello, 1, 5), true, "boot 1 low 5")
	rfc7474Accept(t, s, peer, rfc7474Packet(t, packet.PacketTypeHello, 1, 6), true, "boot 1 low 6")
	rfc7474Accept(t, s, peer, rfc7474Packet(t, packet.PacketTypeHello, 2, 1), true, "boot 2 low 1 after restart")
}

// RFC requirement: RFC7474-2-5 negative -- after boot 2 low 1 is accepted, an AuType 3 Hello from
// the same neighbor at boot 1 low 0xFFFFFFF0 (larger low word, smaller boot count), at boot 2
// low 0 (smaller) and at boot 2 low 1 (equal) is not greater, and each is dropped as a replay.
func TestRFC7474NotGreaterExtendedSequenceDropped(t *testing.T) {
	// Goal: a sequence that is not greater is refused, including one whose low word alone
	// would compare greater. Method: accept boot 2 low 1, then offer three lesser values.
	s := rfc7474Store()
	peer := ridOf("2.2.2.2")
	rfc7474Accept(t, s, peer, rfc7474Packet(t, packet.PacketTypeHello, 2, 1), true, "boot 2 low 1")
	rfc7474Accept(t, s, peer, rfc7474Packet(t, packet.PacketTypeHello, 1, 0xFFFFFFF0), false, "boot 1 low 0xFFFFFFF0")
	rfc7474Accept(t, s, peer, rfc7474Packet(t, packet.PacketTypeHello, 2, 0), false, "boot 2 low 0")
	rfc7474Accept(t, s, peer, rfc7474Packet(t, packet.PacketTypeHello, 2, 1), false, "boot 2 low 1 again")
}

// RFC requirement: RFC7474-2-6 positive -- the last accepted sequence is kept per sending
// neighbor and per packet type: after neighbor 2.2.2.2's Hello at sequence 100, a Hello from
// neighbor 3.3.3.3 at sequence 5 and an LS Acknowledgment from 2.2.2.2 at sequence 5 are both
// accepted.
func TestRFC7474ReplayMarkPerNeighborAndType(t *testing.T) {
	// Goal: neither another neighbor nor another packet type shares the mark. Method: one high
	// mark, then a lower sequence on each of the two other slots.
	s := rfc7474Store()
	first, second := ridOf("2.2.2.2"), ridOf("3.3.3.3")
	rfc7474Accept(t, s, first, rfc7474Packet(t, packet.PacketTypeHello, 1, 100), true, "2.2.2.2 Hello 100")
	rfc7474Accept(t, s, second, rfc7474Packet(t, packet.PacketTypeHello, 1, 5), true, "3.3.3.3 Hello 5")
	rfc7474Accept(t, s, first, rfc7474Packet(t, packet.PacketTypeLSAck, 1, 5), true, "2.2.2.2 LS Ack 5")
}

// RFC requirement: RFC7474-2-6 negative -- a packet not greater than the last accepted of its
// type from its sender is a replay and is dropped: 2.2.2.2's Hello at 99 after its Hello at 100,
// and 3.3.3.3's Hello at 5 after its own Hello at 5, while 3.3.3.3's mark stays its own.
func TestRFC7474ReplayedPacketDropped(t *testing.T) {
	// Goal: each neighbor's own mark refuses its replays. Method: two neighbors with distinct
	// marks, each replayed.
	s := rfc7474Store()
	first, second := ridOf("2.2.2.2"), ridOf("3.3.3.3")
	rfc7474Accept(t, s, first, rfc7474Packet(t, packet.PacketTypeHello, 1, 100), true, "2.2.2.2 Hello 100")
	rfc7474Accept(t, s, second, rfc7474Packet(t, packet.PacketTypeHello, 1, 5), true, "3.3.3.3 Hello 5")
	rfc7474Accept(t, s, first, rfc7474Packet(t, packet.PacketTypeHello, 1, 99), false, "2.2.2.2 Hello 99")
	rfc7474Accept(t, s, second, rfc7474Packet(t, packet.PacketTypeHello, 1, 5), false, "3.3.3.3 Hello 5 again")
	rfc7474Accept(t, s, second, rfc7474Packet(t, packet.PacketTypeHello, 1, 6), true, "3.3.3.3 Hello 6")
}

// rfc7474SentSequence returns the 64-bit sequence a packet sent by the engine's send path
// carries, read back by the receive-side verifier.
func rfc7474SentSequence(t *testing.T, sent []byte) uint64 {
	t.Helper()
	if sent == nil {
		t.Fatal("signPacket returned nil: nothing was sent")
	}
	key := packet.AuthKey{KeyID: 1, Algorithm: "hmac-sha-256", Secret: []byte("k")}
	// RFC 7474 Section 5: the source is the fixture's interface address, not zero.
	seq, ok := packet.Verify(sent, packet.AuTypeCryptographicESN, key, [4]byte{192, 0, 2, 1})
	if !ok {
		t.Fatal("the sent packet does not verify as AuType 3")
	}
	for _, source := range [][4]byte{{}, {192, 0, 2, 2}} {
		if _, ok := packet.Verify(sent, packet.AuTypeCryptographicESN, key, source); ok {
			t.Fatalf("the sent packet verifies with wrong IP source %v", source)
		}
	}
	return seq
}

// RFC requirement: RFC7474-2-3 positive -- every OSPF packet the engine sends through its signer
// carries a low-order 32-bit sequence one greater than the packet sent before it: three Hellos
// signed by engine.signPacket on eth0 carry boot<<32|1, |2 and |3, each read back from the wire.
func TestRFC7474EverySentPacketIncrementsSequence(t *testing.T) {
	// Goal: the increment happens per SENT packet, on the send path. Method: send three
	// packets through engine.signPacket and decode each packet's sequence.
	installOSPFAddressBackend(t)
	installDaemonState(t)
	store := daemonStateClient{}
	if err := store.StatePut(context.Background(), zefs.KeyOSPFAuthBootCount.Key(), []byte{0, 0, 0, 7}); err != nil {
		t.Fatalf("StatePut: %v", err)
	}
	eng := newEngine(nil)
	eng.auth = rfc7474Store()
	eng.auth.setBootCount(7, func() (uint32, error) {
		return loadOSPFBootCount(context.Background(), store)
	})

	var sequences []uint64
	for range 3 {
		sent := eng.signPacket("eth0", rfc7474UnsignedHello())
		sequences = append(sequences, rfc7474SentSequence(t, sent))
	}
	for idx, seq := range sequences {
		if want := uint64(7)<<32 | uint64(idx+1); seq != want {
			t.Fatalf("packet %d sequence = %#x, want %#x (one increment per sent packet)", idx, seq, want)
		}
	}
}

// rfc7474UnsignedHello is an encoded AuType 0 Hello, the payload the send path signs.
func rfc7474UnsignedHello() []byte {
	p := packet.Packet{Header: packet.Header{Type: packet.PacketTypeHello}, Hello: &packet.Hello{NetworkMask: [4]byte{255, 255, 255, 0}, HelloInterval: 10, DeadInterval: 40}}
	buf := make([]byte, p.EncodedLen())
	n := p.WriteTo(buf, 0)
	return buf[:n]
}
