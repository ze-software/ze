// Design: docs/architecture/ospf/ospf-ext-9-graceful-restart.md -- helper entry and how X is identified.
// Related: gr_helper.go -- graceOnReceive and graceRestarterV4, the OSPFv2 helper receive this drives.
//
// VALIDATES: RFC 3623 sec 3.1 check 1 and sec A on the OSPFv2 helper path Ze runs. On a
// broadcast, NBMA or Point-to-MultiPoint segment the helper identifies the restarting neighbor
// by the IP interface address TLV in the Grace-LSA body; on every other segment type it
// identifies X by the Grace-LSA's Advertising Router field.
// PREVENTS: a helper that decodes the IP interface address TLV and then ignores it, keying the
// helper session on the Advertising Router on shared media, so it helps the wrong neighbor or
// helps a neighbor the Grace-LSA does not name.
package ospf

import (
	"net/netip"
	"testing"
	"time"

	ospfneighbor "github.com/ze-software/ze/internal/plugins/ospf/neighbor"
	ospfpacket "github.com/ze-software/ze/internal/plugins/ospf/packet"
	ospftypes "github.com/ze-software/ze/internal/plugins/ospf/types"
)

var (
	// The neighbor Router IDs sort above the local one, so each neighbor is the DBD master and the
	// exchange in grIdentityNeighborFull opens with its Init|More|Master packet.
	grIdentityLocalRID = ospftypes.RouterID{10, 0, 0, 1}
	grIdentityRIDA     = ospftypes.RouterID{172, 16, 0, 2}
	grIdentityRIDB     = ospftypes.RouterID{172, 16, 0, 3}
	grIdentityAddrA    = netip.MustParseAddr("192.0.2.2")
	grIdentityAddrB    = netip.MustParseAddr("192.0.2.3")
)

// grIdentityEngine builds a GR-enabled OSPFv2 engine whose iface runs the network type kind, with the
// neighbor table configured for that interface and this router as the segment's DR, so every
// neighbor driven through the DBD exchange forms a Full adjacency.
func grIdentityEngine(t *testing.T, iface, kind string) *engine {
	t.Helper()
	e := grEnableEngine(t, false, time.Now())
	e.mu.Lock()
	e.cfg.RouterID = grIdentityLocalRID
	e.running[iface] = interfaceConfig{Name: iface, AreaID: ospftypes.BackboneArea, Enabled: true, NetworkType: networkType(kind)}
	e.mu.Unlock()
	e.neighbors.ConfigureInterface(ospfneighbor.InterfaceConfig{
		Name: iface, AreaID: ospftypes.BackboneArea, RouterID: grIdentityLocalRID,
		NetworkType: kind, Options: ospftypes.OptionE, DeadInterval: 40, InterfaceMTU: 1500,
		LocalDR: grIdentityLocalRID,
	})
	t.Cleanup(e.shutdown)
	return e
}

// grIdentityNeighborFull drives neighbor id at addr on iface to Full through the Hello and DBD
// exchange, the way the neighbor FSM reaches it from the wire.
func grIdentityNeighborFull(t *testing.T, e *engine, iface, networkType string, id ospftypes.RouterID, addr netip.Addr) {
	t.Helper()
	e.neighbors.Hello(ospfneighbor.HelloInput{
		InterfaceName: iface, AreaID: ospftypes.BackboneArea, LocalRouterID: grIdentityLocalRID,
		LocalDR: grIdentityLocalRID, NeighborID: id, Address: addr, Priority: 1, TwoWay: true,
		NetworkType: networkType, DeadInterval: 40, InterfaceMTU: 1500, Now: time.Now(),
	})
	if r := e.neighbors.HandleDBDesc(iface, id, ospfpacket.DBDesc{
		InterfaceMTU: 1500, Options: ospftypes.OptionE,
		Flags:      ospfpacket.DDFlagInit | ospfpacket.DDFlagMore | ospfpacket.DDFlagMaster,
		DDSequence: 7,
	}); r != "" {
		t.Fatalf("ExStart DD for %s: %s", id, r)
	}
	snap, ok := e.neighbors.Lookup(iface, id)
	if !ok {
		t.Fatalf("neighbor %s missing after ExStart", id)
	}
	seq := snap.DDSequence
	flags := uint8(0)
	if !snap.Master {
		seq++
		flags = ospfpacket.DDFlagMaster
	}
	if r := e.neighbors.HandleDBDesc(iface, id, ospfpacket.DBDesc{
		InterfaceMTU: 1500, Options: ospftypes.OptionE, Flags: flags, DDSequence: seq,
	}); r != "" {
		t.Fatalf("Exchange DD for %s: %s", id, r)
	}
	waitFor(t, func() bool {
		s, ok := e.neighbors.Lookup(iface, id)
		return ok && s.State == neighborStateFull
	})
}

// grIdentityReceive delivers a peer's OSPFv2 Grace-LSA through the production graceOnReceive
// hook, with the IP interface address TLV carrying addr when withAddr is set.
func grIdentityReceive(e *engine, iface string, advRouter ospftypes.RouterID, addr netip.Addr, withAddr bool) {
	var raw [4]byte
	if withAddr {
		raw = addr.As4()
	}
	e.graceOnReceive(opaqueReceived{
		OpaqueType:        ospfpacket.GraceOpaqueType,
		Interface:         iface,
		AdvertisingRouter: advRouter,
		Body:              grV4Body(120, 1, raw, withAddr),
	})
}

// TestRFC3623HelperIdentifiesRestarterByInterfaceAddress pins which neighbor the helper helps.
// Method: a broadcast segment carries two Full neighbors A and B. A Grace-LSA whose Advertising
// Router is A but whose IP interface address TLV names B's address must make Ze help B, and
// not A. On a point-to-point segment the same Grace-LSA (address naming another router) must
// make Ze help A, the Advertising Router.
//
// RFC requirement: RFC3623-A-4 positive -- on a broadcast segment the helper uses the Grace-LSA's
// IP interface address TLV to identify the restarting router: the Full neighbor at that address
// is the one Ze helps, whatever the Advertising Router field says; on a point-to-point segment
// the Advertising Router identifies X instead.
func TestRFC3623HelperIdentifiesRestarterByInterfaceAddress(t *testing.T) {
	e := grIdentityEngine(t, "eth0", ospftypes.NetworkBroadcast)
	grIdentityNeighborFull(t, e, "eth0", ospftypes.NetworkBroadcast, grIdentityRIDA, grIdentityAddrA)
	grIdentityNeighborFull(t, e, "eth0", ospftypes.NetworkBroadcast, grIdentityRIDB, grIdentityAddrB)

	grIdentityReceive(e, "eth0", grIdentityRIDA, grIdentityAddrB, true)
	if !e.gr.isHelping("eth0", grIdentityRIDB) {
		t.Fatalf("broadcast: the Grace-LSA names B's address %s, Ze must help B (%s)", grIdentityAddrB, grIdentityRIDB)
	}
	if e.gr.isHelping("eth0", grIdentityRIDA) {
		t.Fatalf("broadcast: Ze helps A (%s), the Advertising Router, which the IP interface address does not name", grIdentityRIDA)
	}
	if n := e.gr.helperSessionCount(); n != 1 {
		t.Fatalf("broadcast: %d helper sessions, want exactly 1 (B)", n)
	}

	p2p := grIdentityEngine(t, "eth1", ospftypes.NetworkPointToPoint)
	grIdentityNeighborFull(t, p2p, "eth1", ospftypes.NetworkPointToPoint, grIdentityRIDA, grIdentityAddrA)
	grIdentityReceive(p2p, "eth1", grIdentityRIDA, grIdentityAddrB, true)
	if !p2p.gr.isHelping("eth1", grIdentityRIDA) {
		t.Fatalf("point-to-point: the Advertising Router is A (%s), Ze must help A", grIdentityRIDA)
	}
}

// TestRFC3623HelperRefusesUnmatchedInterfaceAddress pins the refusal side of the same
// identification. Method: a broadcast segment carries Full neighbor A. A Grace-LSA from A whose
// IP interface address TLV names an address no neighbor holds, and one from A carrying no IP
// interface address TLV at all, must each leave Ze helping nobody, although A's Router ID
// matches a Full neighbor.
//
// RFC requirement: RFC3623-A-4 negative -- on a broadcast segment a Grace-LSA whose IP interface
// address names no neighbor, or that omits the IP interface address TLV, identifies no
// restarting router, so Ze enters helper mode for nobody, not for the Advertising Router.
func TestRFC3623HelperRefusesUnmatchedInterfaceAddress(t *testing.T) {
	e := grIdentityEngine(t, "eth0", ospftypes.NetworkBroadcast)
	grIdentityNeighborFull(t, e, "eth0", ospftypes.NetworkBroadcast, grIdentityRIDA, grIdentityAddrA)

	grIdentityReceive(e, "eth0", grIdentityRIDA, netip.MustParseAddr("192.0.2.99"), true)
	if n := e.gr.helperSessionCount(); n != 0 {
		t.Fatalf("unknown address: %d helper sessions, want 0 (no neighbor holds 192.0.2.99)", n)
	}

	grIdentityReceive(e, "eth0", grIdentityRIDA, netip.Addr{}, false)
	if n := e.gr.helperSessionCount(); n != 0 {
		t.Fatalf("no address TLV: %d helper sessions, want 0 (X cannot be identified)", n)
	}
}
