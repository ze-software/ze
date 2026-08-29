// VALIDATES: spec-ospf-ext-1 AC-2/AC-3/AC-4/R-1 -- opaque LSAs route to the store their
// RFC 5250 §3 scope selects (Type 9 link, Type 10 area, Type 11 AS-wide opaque), a
// link-local Type 9 stays on its arrival interface, and a Type 11 is discarded on a
// stub/NSSA interface and never flooded out one.
// PREVENTS: the pre-existing bug where Type 11 (no scope bits) routed to the per-area
// store, leaking an AS-wide LSA into one area or failing AS-wide flooding.
package lsdb

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// opaqueLSA builds a decoded opaque LSA (types 9/10/11) with the O-bit set, LS Age 0.
func opaqueLSA(t *testing.T, scope types.LSType, opaqueType uint8, opaqueID uint32, adv types.RouterID, seq types.LSSequenceNumber, body []byte) packet.LSA {
	t.Helper()
	lsa := packet.LSA{
		Header: packet.LSAHeader{
			Age:               0,
			Options:           types.OptionO,
			Type:              scope,
			LinkStateID:       packet.OpaqueLinkStateID(opaqueType, opaqueID),
			AdvertisingRouter: adv,
			Sequence:          seq,
		},
		Opaque: &packet.OpaqueLSA{Type: scope, Data: body},
	}
	return encodeDecodeLSA(t, lsa)
}

func opaqueTopology() []InterfaceInfo {
	return []InterfaceInfo{
		{
			Name: "eth0", AreaID: area("0.0.0.0"), AreaType: AreaTypeNormal,
			NetworkType: NetworkPointToPoint, State: InterfaceStateDR, RouterID: rid("1.1.1.1"), TransmitDelay: 1,
			Neighbors: []NeighborInfo{{RouterID: rid("2.2.2.2"), Address: naddr4("10.0.0.2"), State: NeighborStateFull, OpaqueCapable: true}},
		},
		{
			Name: "eth1", AreaID: area("0.0.0.0"), AreaType: AreaTypeNormal,
			NetworkType: NetworkPointToPoint, State: InterfaceStateDR, RouterID: rid("1.1.1.1"), TransmitDelay: 1,
			Neighbors: []NeighborInfo{{RouterID: rid("3.3.3.3"), Address: naddr4("10.0.1.3"), State: NeighborStateFull, OpaqueCapable: true}},
		},
	}
}

func TestOpaqueScopeRouting(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	db.SetTx((&txRecorder{}).Send)
	db.SetTopology(opaqueTopology)
	a0 := area("0.0.0.0")
	body := []byte{0xde, 0xad, 0xbe, 0xef}

	// Type 10 (area scope) -> per-area store.
	lsa10 := opaqueLSA(t, types.LSTypeOpaqueArea, 1, 0x10, rid("2.2.2.2"), types.InitialSequenceNumber, body)
	if !db.Install(a0, lsa10) {
		t.Fatalf("Type 10 opaque install rejected")
	}
	// RFC requirement: RFC5250-3.1-2 positive -- a Type 10 opaque LSA is bound to the receiving area's store
	if _, ok := db.LookupLSA(a0, lsa10.Header.Key()); !ok {
		t.Fatalf("Type 10 opaque not in the area store")
	}

	// Type 11 (AS scope) -> the AS-wide opaque store, visible from any area.
	lsa11 := opaqueLSA(t, types.LSTypeOpaqueAS, 4, 0x20, rid("2.2.2.2"), types.InitialSequenceNumber, body)
	if !db.Install(types.BackboneArea, lsa11) {
		t.Fatalf("Type 11 opaque install rejected")
	}
	// RFC requirement: RFC5250-3-1 positive -- opaque LS types 9/10/11 route to the store their flooding scope selects (Type 11 -> AS-wide)
	if _, ok := db.LookupLSA(area("0.0.0.9"), lsa11.Header.Key()); !ok {
		t.Fatalf("Type 11 opaque not visible AS-wide (routed to per-area store?)")
	}
	snap := db.Snapshot()
	if len(snap.ASOpaque) != 1 || snap.ASOpaque[0].Type != "opaque-as" {
		t.Fatalf("Type 11 not in AS-opaque snapshot: %+v", snap.ASOpaque)
	}
	if len(snap.ASExternal) != 0 {
		t.Fatalf("Type 11 leaked into the AS-external store: %+v", snap.ASExternal)
	}

	// Type 9 (link scope) -> the per-interface link store; Install must reject it so it
	// cannot land in an area store.
	lsa9 := opaqueLSA(t, types.LSTypeOpaqueLink, 1, 0x30, rid("2.2.2.2"), types.InitialSequenceNumber, body)
	// RFC requirement: RFC5250-3-1 negative -- a link-scoped Type 9 is refused the area store (scope is enforced, not accept-all)
	if db.Install(a0, lsa9) {
		t.Fatalf("Install accepted a link-scoped Type 9 opaque LSA into an area store")
	}
	if _, ok := db.installLink("eth0", a0, lsa9, false, true); !ok {
		t.Fatalf("installLink rejected Type 9 opaque LSA")
	}
	if _, ok := db.LookupLinkLSA("eth0", lsa9.Header.Key()); !ok {
		t.Fatalf("Type 9 opaque not in eth0 link store")
	}
	if _, ok := db.LookupLSA(a0, lsa9.Header.Key()); ok {
		t.Fatalf("Type 9 opaque wrongly visible in the area store")
	}
}

func TestOpaqueType9WrongInterfaceDiscarded(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	db.SetTopology(opaqueTopology)
	a0 := area("0.0.0.0")
	lsa9 := opaqueLSA(t, types.LSTypeOpaqueLink, 1, 0x30, rid("2.2.2.2"), types.InitialSequenceNumber, []byte{1, 2, 3, 4})

	db.ReceiveUpdate(ReceiveInput{Interface: "eth0", AreaID: a0, RouterID: rid("2.2.2.2"), Src: netip.MustParseAddr("10.0.0.2"), Update: packet.LSUpdate{LSAs: []packet.LSA{lsa9}}})

	// RFC 5250 §3.1: a Type-9 opaque LSA is bound to its arrival link -- present on eth0,
	// never installed on eth1, and never flooded out eth1.
	// RFC requirement: RFC5250-3.1-1 positive -- a Type 9 opaque LSA is stored on its arrival interface
	if _, ok := db.LookupLinkLSA("eth0", lsa9.Header.Key()); !ok {
		t.Fatalf("Type 9 opaque not stored on the arrival interface")
	}
	// RFC requirement: RFC5250-3.1-1 negative -- a Type 9 opaque LSA is not installed on a non-target interface
	if _, ok := db.LookupLinkLSA("eth1", lsa9.Header.Key()); ok {
		t.Fatalf("Type 9 opaque leaked to a non-target interface")
	}
	for _, s := range tx.sends {
		if s.iface == "eth1" && s.pkt.LSUpdate != nil {
			t.Fatalf("Type 9 opaque flooded out a non-target interface eth1")
		}
	}
	if db.retransmit[NeighborKey{Interface: "eth1", RouterID: rid("3.3.3.3")}][lsa9.Header.Key()] != nil {
		t.Fatalf("Type 9 opaque queued for retransmit on a non-target interface")
	}
}

func TestOpaqueType11StubDiscarded(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	stub := area("0.0.0.7")
	db.SetAreaTypes(map[types.AreaID]string{stub: AreaTypeStub})
	db.SetTopology(func() []InterfaceInfo {
		return []InterfaceInfo{{
			Name: "eth0", AreaID: stub, AreaType: AreaTypeStub,
			NetworkType: NetworkPointToPoint, State: InterfaceStateDR, RouterID: rid("1.1.1.1"), TransmitDelay: 1,
			Neighbors: []NeighborInfo{{RouterID: rid("2.2.2.2"), Address: naddr4("10.0.0.2"), State: NeighborStateFull, OpaqueCapable: true}},
		}}
	})
	lsa11 := opaqueLSA(t, types.LSTypeOpaqueAS, 4, 0x40, rid("2.2.2.2"), types.InitialSequenceNumber, []byte{1, 2, 3, 4})

	// Received on a stub interface -> discarded, not installed, not acknowledged (§3.1).
	db.ReceiveUpdate(ReceiveInput{Interface: "eth0", AreaID: stub, RouterID: rid("2.2.2.2"), Src: netip.MustParseAddr("10.0.0.2"), Update: packet.LSUpdate{LSAs: []packet.LSA{lsa11}}})
	// RFC requirement: RFC5250-3.1-3 negative -- a Type 11 opaque LSA received on a stub-area interface is discarded, not installed
	if _, ok := db.LookupLSA(types.BackboneArea, lsa11.Header.Key()); ok {
		t.Fatalf("Type 11 opaque installed despite arriving on a stub interface")
	}
	if len(tx.sends) != 0 {
		t.Fatalf("Type 11 on a stub interface produced traffic (should be silently discarded): %+v", tx.sends)
	}

	// A Type 11 in the AS-opaque store must not be flooded out a stub interface.
	// RFC requirement: RFC5250-3.1-3 positive -- a Type 11 opaque LSA is accepted into the AS-opaque store (valid outside a stub area)
	if !db.Install(types.BackboneArea, lsa11) {
		t.Fatalf("Type 11 install into AS-opaque store rejected")
	}
	tx.sends = nil
	db.floodExcept("", types.RouterID{}, types.BackboneArea, lsa11.Header.Key())
	for _, s := range tx.sends {
		if s.iface == "eth0" {
			t.Fatalf("Type 11 opaque flooded out a stub interface: %+v", tx.sends)
		}
	}
}

// opaqueTwoAreaTopology puts eth0 in area 0.0.0.0 and eth1 in area 0.0.0.2, so an
// area-scoped LSA has exactly one eligible outgoing interface and a cross-area leak is
// visible as a send on the wrong interface.
func opaqueTwoAreaTopology() []InterfaceInfo {
	return []InterfaceInfo{
		{
			Name: "eth0", AreaID: area("0.0.0.0"), AreaType: AreaTypeNormal,
			NetworkType: NetworkPointToPoint, State: InterfaceStateDR, RouterID: rid("1.1.1.1"), TransmitDelay: 1,
			Neighbors: []NeighborInfo{{RouterID: rid("2.2.2.2"), Address: naddr4("10.0.0.2"), State: NeighborStateFull, OpaqueCapable: true}},
		},
		{
			Name: "eth1", AreaID: area("0.0.0.2"), AreaType: AreaTypeNormal,
			NetworkType: NetworkPointToPoint, State: InterfaceStateDR, RouterID: rid("1.1.1.1"), TransmitDelay: 1,
			Neighbors: []NeighborInfo{{RouterID: rid("3.3.3.3"), Address: naddr4("10.0.1.3"), State: NeighborStateFull, OpaqueCapable: true}},
		},
	}
}

// TestOpaqueType10ConfinedToItsArea drives the AREA half of the Type-10 flooding scope.
// RFC 5250 sec 3.1: "If the Opaque LSA is type-10 (the flooding scope is area-local) and
// the area associated with the Opaque LSA (as identified during origination or from a
// received LSA's associated OSPF packet header) is not the same as the area associated
// with the target interface, the Opaque LSA MUST be discarded and not acknowledged."
//
// TestOpaqueScopeRouting installs a Type-10 into area 0.0.0.0 and looks it up in the same
// area, which holds whether or not the area is load-bearing: making dbForReadLocked serve
// a Type-10 from ANY area left that test green. Both interfaces there also sit in one
// area, so nothing exercised the send-side area match either.
//
// RFC requirement: RFC5250-3.1-2 positive -- a Type-10 opaque LSA is readable only through
// the area it was installed in (dbForReadLocked per-area branch, lsdb.go:323-332), and is
// flooded only out an interface whose AreaID equals that area (eligibleInterface default
// branch, flooding.go:411-413).
// RFC requirement: RFC5250-3.1-2 negative -- the same LSA is invisible from a different
// area and is never sent out that area's interface, so a Type-10 whose area differs from
// the target interface's area is not flooded on.
func TestOpaqueType10ConfinedToItsArea(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	tx := &txRecorder{}
	db.SetTx(tx.Send)
	db.SetTopology(opaqueTwoAreaTopology)
	home := area("0.0.0.0")
	other := area("0.0.0.2")

	lsa10 := opaqueLSA(t, types.LSTypeOpaqueArea, 1, 0x40, rid("2.2.2.2"), types.InitialSequenceNumber, []byte{0xca, 0xfe, 0xba, 0xbe})
	if !db.Install(home, lsa10) {
		t.Fatalf("Type 10 opaque install rejected")
	}
	key := lsa10.Header.Key()

	if _, ok := db.LookupLSA(home, key); !ok {
		t.Fatalf("Type 10 opaque not readable from the area it was installed in")
	}
	if _, ok := db.LookupLSA(other, key); ok {
		t.Fatalf("Type 10 opaque readable from area %v: an area-local LSA leaked across areas", other)
	}
	if _, ok := db.Lookup(other, key); ok {
		t.Fatalf("Type 10 opaque header visible from area %v", other)
	}

	db.floodExcept("", types.RouterID{}, home, key)
	if len(tx.sends) != 1 {
		t.Fatalf("an area-local Type-10 must be flooded out the one interface in its area, got %d sends: %+v", len(tx.sends), tx.sends)
	}
	if tx.sends[0].iface != "eth0" {
		t.Fatalf("Type 10 opaque flooded out %q, which is not in area %v", tx.sends[0].iface, home)
	}
	if db.retransmit[NeighborKey{Interface: "eth1", RouterID: rid("3.3.3.3")}][key] != nil {
		t.Fatalf("a neighbor in a different area was queued to retransmit an area-local Type-10")
	}
}
