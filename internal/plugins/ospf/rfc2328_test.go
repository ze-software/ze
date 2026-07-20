// VALIDATES: RFC 2328 Appendix A.3.3 and Appendix D.2 at the engine seam -- the synthetic
// virtual-link interface is built with no Interface MTU (so its Database Descriptions carry 0),
// and an interface configured for Simple-password authentication drops a packet whose 64-bit
// authentication field does not match the configured password.
// PREVENTS: a virtual link stamping a real MTU into its DD (peer MTU mismatch), and a wrong
// password being accepted because the drop lives only in the packet layer.
package ospf

import (
	"net/netip"
	"testing"
	"time"

	ospfiface "codeberg.org/thomas-mangin/ze/internal/plugins/ospf/iface"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/packet"
	ospfspf "codeberg.org/thomas-mangin/ze/internal/plugins/ospf/spf"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
)

// mtuRecorder captures the InterfaceMTU an interface reports for its neighbors. The event is
// built straight from the interface's own config (ospfiface neighborEventLocked, iface.go:531),
// so it is direct evidence of what startVirtualInterface configured.
type mtuRecorder struct {
	events []ospfiface.NeighborEvent
}

func (r *mtuRecorder) NeighborHello(ev ospfiface.NeighborEvent) { r.events = append(r.events, ev) }
func (r *mtuRecorder) NeighborDown(_ string, _ types.RouterID)  {}
func (r *mtuRecorder) AdjOK(_ string, _, _ types.RouterID)      {}
func (r *mtuRecorder) InterfaceDown(_ string)                   {}

// RFC requirement: RFC2328-A.3.3-1 positive -- the synthetic virtual-link interface is created
// with no Interface MTU, so the MTU it reports (and stamps into every Database Description it
// sends) is 0, while the real transit interface reports a non-zero MTU
// (startVirtualInterface leaves Config.InterfaceMTU unset, virtual_link.go:229-244; the real
// interface takes interfaceMTU(name), instance.go:922).
func TestRFC2328VirtualInterfaceHasNoMTU(t *testing.T) {
	const j = `{"ospf":{"router-id":"10.0.0.1",
		"areas":{"area":{
			"0.0.0.0":{"area-id":"0.0.0.0"},
			"0.0.0.1":{"area-id":"0.0.0.1","virtual-link":{"10.0.0.2":{}}}}},
		"interfaces":{"interface":{
			"eth0":{"area":"0.0.0.1","network-type":"point-to-point"},
			"lo":{"area":"0.0.0.0","network-type":"loopback"}}}}}`
	eng, _ := vlEngine(t, j)
	defer eng.shutdown()
	transit := vlArea(t, "0.0.0.1")
	peer := vlRID(t, "10.0.0.2")

	eng.onVirtualLinksResolved([]ospfspf.VirtualNeighborResult{{
		TransitArea: transit, Neighbor: peer, Reachable: true, Cost: 10,
		NextHops: []ospfspf.NextHop{{Addr: netip.MustParseAddr("192.0.2.2"), Interface: "eth0"}},
	}})
	vlname := virtualLinkName(virtualLinkKey{transit: transit, neighbor: peer})

	eng.mu.Lock()
	vlIface := eng.interfaces[vlname]
	realIface := eng.interfaces["eth0"]
	eng.mu.Unlock()
	if vlIface == nil || realIface == nil {
		t.Fatalf("interfaces missing: virtual=%v real=%v", vlIface != nil, realIface != nil)
	}

	hello := packet.Hello{
		HelloInterval: DefaultHelloInterval,
		Options:       types.OptionE,
		Priority:      1,
		DeadInterval:  uint32(DefaultDeadInterval),
	}
	vlRec := &mtuRecorder{}
	vlIface.SetNeighborSink(vlRec)
	vlIface.ReceiveDecodedHello(peer, netip.MustParseAddr("192.0.2.2"), hello, time.Unix(1, 0))
	if len(vlRec.events) != 1 {
		t.Fatalf("virtual interface neighbor events = %d, want 1", len(vlRec.events))
	}
	if got := vlRec.events[0].InterfaceMTU; got != 0 {
		t.Fatalf("virtual-link Interface MTU = %d, want 0 (RFC 2328 App A.3.3)", got)
	}
	if !vlRec.events[0].MTUIgnore {
		t.Fatalf("the virtual link must skip the MTU match, MTUIgnore = false")
	}

	realRec := &mtuRecorder{}
	realIface.SetNeighborSink(realRec)
	realIface.ReceiveDecodedHello(peer, netip.MustParseAddr("192.0.2.2"), hello, time.Unix(1, 0))
	if len(realRec.events) != 1 {
		t.Fatalf("real interface neighbor events = %d, want 1", len(realRec.events))
	}
	if realRec.events[0].InterfaceMTU == 0 {
		t.Fatalf("a real interface must carry a non-zero Interface MTU; the zero is virtual-link-only")
	}
}

// RFC requirement: RFC2328-D.2-1 positive -- with Simple-password (AuType 1) configured, a packet
// whose 64-bit authentication field carries the configured password is accepted
// (packet.Verify AuTypeSimple constant-time compare, auth_verify.go:218-221, reached through
// authStore.verify, auth_keystore.go:344-351).
// RFC requirement: RFC2328-D.2-1 negative -- a packet whose authentication field holds a
// different password is discarded with reason "password-mismatch", so a wrong password never
// reaches a handler (authStore.verify, auth_keystore.go:344-367).
func TestRFC2328SimplePasswordMismatchDiscarded(t *testing.T) {
	store := newAuthStore()
	store.configure(authCfg(keyConfig{KeyID: 1, Algorithm: packet.AuthSimple, Secret: "goodpass"}))
	peer := ridOf("2.2.2.2")

	mk := func(password string) []byte {
		p := packet.Packet{Header: packet.Header{Type: packet.PacketTypeHello, AuType: packet.AuTypeSimple}, Hello: &packet.Hello{NetworkMask: [4]byte{255, 255, 255, 0}, HelloInterval: 10, DeadInterval: 40}}
		buf := make([]byte, p.EncodedLen())
		n := p.WriteTo(buf, 0)
		signed, err := packet.Sign(buf[:n], packet.AuTypeSimple, packet.AuthKey{KeyID: 1, Algorithm: packet.AuthSimple, Secret: []byte(password)}, 0, [4]byte{})
		if err != nil {
			t.Fatalf("Sign(%q): %v", password, err)
		}
		return signed
	}

	if reason, ok := store.verify("eth0", peer, [4]byte{}, mk("goodpass")); !ok {
		t.Fatalf("the configured password was rejected: %s", reason)
	}
	reason, ok := store.verify("eth0", peer, [4]byte{}, mk("badpass"))
	if ok {
		t.Fatalf("a packet carrying the wrong Simple password was accepted")
	}
	if reason != "password-mismatch" {
		t.Fatalf("reject reason = %q, want password-mismatch", reason)
	}
}
