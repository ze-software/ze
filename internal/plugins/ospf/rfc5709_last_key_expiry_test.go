// Design: docs/architecture/ospf/ospf-12-auth.md -- key lifetimes.
// Related: auth_wiring.go -- signPacket, the transport signer hook.
// Related: auth_keystore.go -- selectSendKey and verify.
//
// VALIDATES: RFC 5709 Section 3.2, "In the event that the last key associated with an
// interface expires, it is unacceptable to revert to an unauthenticated condition", in both
// directions: the engine still signs what it sends, and still refuses an unauthenticated
// packet it receives.
// PREVENTS: an interface whose key chain has fully expired sending AuType 0 packets, or
// accepting them.
package ospf

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
)

// expiredChainEngine returns an engine whose eth0 key chain holds one HMAC-SHA-256 key
// with its send and accept lifetimes both over at the engine's clock.
func expiredChainEngine(t *testing.T) *engine {
	t.Helper()
	eng := newEngine(transport.New(&fakeBackend{}))
	t.Cleanup(eng.shutdown)
	window := lifetimeConfig{Start: "2026-01-01T00:00:00Z", End: "2026-02-01T00:00:00Z"}
	eng.auth.configure(lifetimeAuthCfg(keyConfig{KeyID: 1, Algorithm: "hmac-sha-256", Secret: "lastkey", SendLifetime: window, AcceptLifetime: window}))
	eng.auth.now = func() time.Time { return rfc3339(t, "2027-01-01T00:00:00Z") }
	return eng
}

// unsignedHello encodes an AuType 0 Hello, the packet an unauthenticated condition sends.
func unsignedHello() []byte {
	hello := packet.Packet{Header: packet.Header{Type: packet.PacketTypeHello, RouterID: ridOf("2.2.2.2")}, Hello: &packet.Hello{NetworkMask: [4]byte{255, 255, 255, 0}, HelloInterval: 10, DeadInterval: 40}}
	buf := make([]byte, hello.EncodedLen())
	return buf[:hello.WriteTo(buf, 0)]
}

// TestRFC5709LastKeyExpiredStillSigns checks the send side after the last key expired.
// Goal: the engine's signer keeps the cryptographic AuType and the expired key rather than
// sending an unauthenticated packet. Method: expire the only key of eth0's chain, pass an
// AuType 0 Hello through signPacket, and read the AuType and Key ID of the result.
func TestRFC5709LastKeyExpiredStillSigns(t *testing.T) {
	eng := expiredChainEngine(t)

	// RFC requirement: RFC5709-3.2-2 positive -- after the only key of eth0's chain has
	// expired, a Hello passed through the engine's signer leaves with AuType 2 and Key ID 1,
	// not as an unauthenticated (AuType 0) packet.
	signed := eng.signPacket("eth0", unsignedHello())
	if signed == nil {
		t.Fatal("signPacket refused to sign once the last key expired")
	}
	if got := packet.AuType(binary.BigEndian.Uint16(signed[14:16])); got != packet.AuTypeCryptographic {
		t.Fatalf("Hello sent after the last key expired carries AuType %d, want cryptographic", got)
	}
	if signed[18] != 1 {
		t.Fatalf("Hello sent after the last key expired names Key ID %d, want 1", signed[18])
	}
}

// TestRFC5709LastKeyExpiredRefusesUnauthenticated checks the receive side after expiry.
// Goal: an interface whose last key expired does not fall back to accepting unauthenticated
// packets. Method: expire the only key of eth0's chain and hand an AuType 0 Hello from a
// neighbor to the key store's verify, the check the dispatcher runs on every packet.
func TestRFC5709LastKeyExpiredRefusesUnauthenticated(t *testing.T) {
	eng := expiredChainEngine(t)

	// RFC requirement: RFC5709-3.2-2 negative -- after the only key of eth0's chain has
	// expired, an unauthenticated (AuType 0) Hello received on eth0 is refused with reason
	// autype-mismatch.
	reason, accepted := eng.auth.verify("eth0", ridOf("2.2.2.2"), [4]byte{}, unsignedHello())
	if accepted {
		t.Fatal("eth0 accepted an unauthenticated Hello once its last key expired")
	}
	if reason != "autype-mismatch" {
		t.Fatalf("refusal reason %q, want autype-mismatch", reason)
	}
}

// lastKeyNotices subscribes to eng's RFC 5709 "last Authentication Key expiration"
// notification on a fresh bus and returns the slice the notices land in.
func lastKeyNotices(eng *engine) *[]lastKeyExpirationEvent {
	bus := newFakeBus()
	eng.setEventSink(newEventSink(bus))
	got := &[]lastKeyExpirationEvent{}
	LastKeyExpiration.Subscribe(bus, func(ev *lastKeyExpirationEvent) { *got = append(*got, *ev) })
	return got
}

// TestRFC5709LastKeyExpiredKeepsAdjacency checks two Ze routers whose shared last key
// expired. Goal: routing is not disrupted, because each treats the key as having an infinite
// lifetime, and each tells the network manager once. Method: two engines hold the same
// expired one-key chain; A signs two Hellos through its transport signer, B verifies them
// with the check the dispatcher runs on every packet, and both event buses are read.
func TestRFC5709LastKeyExpiredKeepsAdjacency(t *testing.T) {
	sender := expiredChainEngine(t)
	receiver := expiredChainEngine(t)
	sent := lastKeyNotices(sender)
	received := lastKeyNotices(receiver)

	// RFC requirement: RFC5709-3.2-5 positive -- when the only key of eth0's chain has
	// expired in both directions, Hellos signed by one engine with that key verify on the
	// other (the key keeps an infinite lifetime for signing and for verification), and each
	// engine emits exactly one last-key-expiration event naming eth0 and Key ID 1.
	for range 2 {
		signed := sender.signPacket("eth0", unsignedHello())
		if signed == nil {
			t.Fatal("signPacket refused to sign once the last key expired")
		}
		if reason, ok := receiver.auth.verify("eth0", ridOf("2.2.2.2"), [4]byte{}, signed); !ok {
			t.Fatalf("a Hello signed with the expired last key was refused (%s): the adjacency drops", reason)
		}
	}
	want := func(name string, got []lastKeyExpirationEvent, direction string) {
		t.Helper()
		if len(got) != 1 {
			t.Fatalf("%s emitted %d last-key-expiration events, want 1: %+v", name, len(got), got)
		}
		if got[0] != (lastKeyExpirationEvent{Interface: "eth0", KeyID: 1, Direction: direction}) {
			t.Fatalf("%s event %+v, want eth0 key 1 %s", name, got[0], direction)
		}
	}
	want("sender", *sent, "send")
	want("receiver", *received, "receive")
}

// TestRFC5709ExpiredKeyRefusedOnceNotLast checks where the infinite lifetime stops.
// Goal: it covers only the LAST key, "until ... a new key is configured". Method: the only
// key of eth0's chain expires, a Hello signed with it verifies, then the operator adds Key
// ID 2 with a live window and the same Hello bytes, re-signed at a higher sequence, are
// refused as accept-lifetime, and no notice is sent for the configured chain.
func TestRFC5709ExpiredKeyRefusedOnceNotLast(t *testing.T) {
	eng := expiredChainEngine(t)
	old := packet.AuthKey{KeyID: 1, Algorithm: "hmac-sha-256", Secret: []byte("lastkey")}

	if reason, ok := eng.auth.verify("eth0", ridOf("2.2.2.2"), [4]byte{}, signedHelloWith(t, old, 1)); !ok {
		t.Fatalf("while Key ID 1 is the last key its Hello is refused (%s)", reason)
	}

	expired := lifetimeConfig{Start: "2026-01-01T00:00:00Z", End: "2026-02-01T00:00:00Z"}
	live := lifetimeConfig{Start: "2026-12-01T00:00:00Z", End: "2027-12-01T00:00:00Z"}
	eng.auth.configure(lifetimeAuthCfg(
		keyConfig{KeyID: 1, Algorithm: "hmac-sha-256", Secret: "lastkey", SendLifetime: expired, AcceptLifetime: expired},
		keyConfig{KeyID: 2, Algorithm: "hmac-sha-256", Secret: "newkey", SendLifetime: live, AcceptLifetime: live},
	))
	notices := lastKeyNotices(eng)

	// RFC requirement: RFC5709-3.2-5 negative -- once a new key with a live window is
	// configured, the expired Key ID 1 is no longer the last key: a Hello signed with it is
	// refused with reason accept-lifetime, the signer moves to Key ID 2, and no
	// last-key-expiration event is emitted.
	reason, ok := eng.auth.verify("eth0", ridOf("2.2.2.2"), [4]byte{}, signedHelloWith(t, old, 2))
	if ok {
		t.Fatal("an expired key that is no longer the last key still verifies")
	}
	if reason != "accept-lifetime" {
		t.Fatalf("refusal reason %q, want accept-lifetime", reason)
	}
	signed := eng.signPacket("eth0", unsignedHello())
	if len(signed) < packet.CommonHeaderLen {
		t.Fatal("signPacket refused to sign with a live key configured")
	}
	if signed[18] != 2 {
		t.Fatalf("the signer names Key ID %d, want the new live Key ID 2", signed[18])
	}
	if len(*notices) != 0 {
		t.Fatalf("a chain with a live key emitted last-key-expiration events: %+v", *notices)
	}
}
