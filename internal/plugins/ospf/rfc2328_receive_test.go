// Design: docs/architecture/ospf/ospf-3-ip-transport.md -- RFC 2328 section 8.2 receive
// checks that the engine performs before a packet reaches a protocol handler: the Area ID
// and the AuType of the OSPF header, both against the receiving interface's configuration.

package ospf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// receiveEngine builds an engine with eth0 in the backbone and returns it with the eth0
// transport handle, so a test can dispatch a packet as if it arrived on eth0.
func receiveEngine(t *testing.T) (*engine, ospfConfig, *fakeHandle) {
	t.Helper()
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","network-type":"point-to-point"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	fb := &fakeBackend{}
	eng := newEngine(transport.New(fb))
	eng.setConfig(cfg)
	if err := eng.openInterfaces(); err != nil {
		t.Fatalf("openInterfaces: %v", err)
	}
	t.Cleanup(eng.shutdown)
	fb.mu.Lock()
	handle := fb.handles["eth0"]
	fb.mu.Unlock()
	if handle == nil {
		t.Fatal("eth0 transport handle missing")
	}
	return eng, cfg, handle
}

// helloFromPeer encodes a Hello from peer carrying area in its OSPF header.
func helloFromPeer(peer types.RouterID, area types.AreaID) []byte {
	hello := packet.Hello{HelloInterval: DefaultHelloInterval, Options: types.OptionE, Priority: 1, DeadInterval: uint32(DefaultDeadInterval)}
	p := packet.Packet{Header: packet.Header{Type: packet.PacketTypeHello, RouterID: peer, AreaID: area}, Hello: &hello}
	buf := make([]byte, p.EncodedLen())
	p.WriteTo(buf, 0)
	return buf
}

// RFC requirement: RFC2328-8.2-2 positive — a Hello whose header Area ID equals the receiving
// interface's configured area passes the area check: it is not dropped and its sender becomes
// a neighbor on that interface (acceptsArea, dispatch).
func TestOSPFReceiveAcceptsMatchingAreaID(t *testing.T) {
	eng, cfg, handle := receiveEngine(t)
	peer := ridOf("10.0.0.2")
	before := eng.dispatch.dropped()
	eng.dispatch.dispatch(transport.RawPacket{IfIndex: handle.ifindex, Src: netip.AddrFrom4([4]byte(peer)), Payload: helloFromPeer(peer, cfg.Areas[0].AreaID)})
	if got := eng.dispatch.dropped(); got != before {
		t.Fatalf("dropped = %d, want %d (matching Area ID must not be dropped)", got, before)
	}
	if rows := eng.neighborSnapshot(); len(rows) != 1 {
		t.Fatalf("neighbor rows = %d, want 1 after a Hello in the interface's area", len(rows))
	}
}

// RFC requirement: RFC2328-8.2-2 negative — a Hello whose header Area ID differs from the
// receiving interface's configured area is dropped before any handler runs: the drop counter
// rises by one and no neighbor forms (acceptsArea returns false, dispatch drops).
func TestOSPFReceiveDropsMismatchedAreaID(t *testing.T) {
	eng, _, handle := receiveEngine(t)
	peer := ridOf("10.0.0.2")
	before := eng.dispatch.dropped()
	other := types.AreaID{0, 0, 0, 1}
	eng.dispatch.dispatch(transport.RawPacket{IfIndex: handle.ifindex, Src: netip.AddrFrom4([4]byte(peer)), Payload: helloFromPeer(peer, other)})
	if got := eng.dispatch.dropped(); got != before+1 {
		t.Fatalf("dropped = %d, want %d (Area ID mismatch must be dropped)", got, before+1)
	}
	if rows := eng.neighborSnapshot(); len(rows) != 0 {
		t.Fatalf("neighbor rows = %d, want 0 after a Hello for another area", len(rows))
	}
}

// RFC requirement: RFC2328-8.2-3 positive — on an interface whose area uses simple-password
// authentication (AuType 1), a Hello carrying AuType 1 and the area's password passes the
// AuType check and reaches the Hello handler: nothing is dropped and the sender becomes a
// neighbor (verifyPacket, authStore.verify).
func TestOSPFReceiveAcceptsMatchingAuType(t *testing.T) {
	eng, cfg, handle := receiveEngine(t)
	eng.auth.configure(authCfg(keyConfig{KeyID: 1, Algorithm: packet.AuthSimple, Secret: "pw"}))
	peer := ridOf("10.0.0.2")
	signed := eng.signPacket("eth0", helloFromPeer(peer, cfg.Areas[0].AreaID))
	if got := packet.AuType(signed[15]); got != packet.AuTypeSimple {
		t.Fatalf("signed AuType = %d, want %d (simple password)", got, packet.AuTypeSimple)
	}
	before := eng.dispatch.dropped()
	eng.dispatch.dispatch(transport.RawPacket{IfIndex: handle.ifindex, Src: netip.AddrFrom4([4]byte(peer)), Payload: signed})
	if got := eng.dispatch.dropped(); got != before {
		t.Fatalf("dropped = %d, want %d (matching AuType must not be dropped)", got, before)
	}
	if rows := eng.neighborSnapshot(); len(rows) != 1 {
		t.Fatalf("neighbor rows = %d, want 1 after an authenticated Hello", len(rows))
	}
}

// RFC requirement: RFC2328-8.2-3 negative — on an interface whose area uses simple-password
// authentication (AuType 1), a Hello carrying AuType 0 (null) is dropped before any handler
// runs: the drop counter rises by one, the auth failure counter rises by one and no neighbor
// forms (authStore.verify reports autype-mismatch, verifyPacket returns false).
func TestOSPFReceiveDropsMismatchedAuType(t *testing.T) {
	eng, cfg, handle := receiveEngine(t)
	rec := &authFailRegistry{}
	eng.setMetrics(rec)
	eng.auth.configure(authCfg(keyConfig{KeyID: 1, Algorithm: packet.AuthSimple, Secret: "pw"}))
	peer := ridOf("10.0.0.2")
	unsigned := helloFromPeer(peer, cfg.Areas[0].AreaID)
	if got := packet.AuType(unsigned[15]); got != packet.AuTypeNull {
		t.Fatalf("unsigned AuType = %d, want %d (null)", got, packet.AuTypeNull)
	}
	before := eng.dispatch.dropped()
	eng.dispatch.dispatch(transport.RawPacket{IfIndex: handle.ifindex, Src: netip.AddrFrom4([4]byte(peer)), Payload: unsigned})
	if got := eng.dispatch.dropped(); got != before+1 {
		t.Fatalf("dropped = %d, want %d (AuType mismatch must be dropped)", got, before+1)
	}
	if rec.authFailures != 1 {
		t.Fatalf("ze_ospf_auth_failures_total increments = %d, want 1", rec.authFailures)
	}
	if rows := eng.neighborSnapshot(); len(rows) != 0 {
		t.Fatalf("neighbor rows = %d, want 0 after a Hello with the wrong AuType", len(rows))
	}
}
