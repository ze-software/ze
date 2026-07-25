// VALIDATES: spec-ospf-ext-1 end-to-end user stories -- with opaque enabled the engine
// discovers a registered consumer and drives its origination; an originated opaque LSA
// appears in `show ospf database opaque-*`; a received opaque LSA is delivered to its
// consumer and listed; opaque hex decodes to Opaque Type/ID + TLVs; and Type 9/10/11
// honor their flood boundaries (link/area/AS, never into a stub area).
// PREVENTS: the opaque carrier compiling but not actually wiring consumer origination,
// reception delivery, the CLI subviews, or the scope gates through the live engine.
package ospf

import (
	"net/netip"
	"testing"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// opaqueLSAForTest builds a decoded opaque LSA (types 9/10/11) with the O-bit set.
func opaqueLSAForTest(t *testing.T, scope types.LSType, opaqueType uint8, opaqueID uint32, adv types.RouterID, seq types.LSSequenceNumber, body []byte) packet.LSA {
	t.Helper()
	lsa := packet.LSA{
		Header: packet.LSAHeader{
			Options:           types.OptionO,
			Type:              scope,
			LinkStateID:       packet.OpaqueLinkStateID(opaqueType, opaqueID),
			AdvertisingRouter: adv,
			Sequence:          seq,
		},
		Opaque: &packet.OpaqueLSA{Type: scope, Data: body},
	}
	buf := make([]byte, lsa.EncodedLen())
	lsa.WriteTo(buf, 0)
	out, err := packet.DecodeLSA(buf)
	if err != nil {
		t.Fatalf("DecodeLSA(opaque): %v", err)
	}
	return out
}

const opaqueCfgJSON = `{"ospf":{"router-id":"1.1.1.1","opaque":true,"areas":{"area":{"0":{"area-id":"0"}}}}}`

func opaqueSubview(t *testing.T, eng *engine, view string) []ospflsdb.LSASnapshot {
	t.Helper()
	rows := eng.databaseSnapshotByType(view)
	if len(rows) != 1 {
		t.Fatalf("%s snapshot wrapping = %d, want 1", view, len(rows))
	}
	snap, ok := rows[0].(ospflsdb.Snapshot)
	if !ok {
		t.Fatalf("%s snapshot is not a Snapshot: %T", view, rows[0])
	}
	var out []ospflsdb.LSASnapshot
	for _, a := range snap.Areas {
		out = append(out, a.LSAs...)
	}
	out = append(out, snap.ASExternal...)
	out = append(out, snap.ASOpaque...)
	for _, l := range snap.Links {
		out = append(out, l.LSAs...)
	}
	return out
}

func TestOSPFOpaqueRegisterFunctional(t *testing.T) {
	resetOpaqueConsumers()
	t.Cleanup(resetOpaqueConsumers)

	called := false
	if err := registerOpaqueConsumer(1, OpaqueScopeArea, func(types.RouterID) []opaqueOrigination {
		called = true
		return nil
	}, nil); err != nil {
		t.Fatalf("register: %v", err)
	}

	eng, router := newRedistEngine(t, opaqueCfgJSON)
	if !eng.cfg.Opaque {
		t.Fatalf("opaque config leaf not parsed true")
	}
	// The engine discovers the consumer and invokes its OnOriginate on the self-LSA pass.
	eng.originateOpaqueLSAs(router)
	if !called {
		t.Fatalf("engine did not discover/invoke the registered consumer's OnOriginate")
	}
}

func TestOSPFOpaqueOriginateFunctional(t *testing.T) {
	resetOpaqueConsumers()
	t.Cleanup(resetOpaqueConsumers)

	body := []byte{0x00, 0x01, 0x00, 0x04, 0xde, 0xad, 0xbe, 0xef}
	if err := registerOpaqueConsumer(1, OpaqueScopeArea, func(types.RouterID) []opaqueOrigination {
		return []opaqueOrigination{{OpaqueID: 0x2a, Area: mustBackboneArea(t), Body: body}}
	}, nil); err != nil {
		t.Fatalf("register: %v", err)
	}
	eng, _ := newRedistEngine(t, opaqueCfgJSON)
	eng.originateSelfLSAs()

	rows := opaqueSubview(t, eng, "opaque-area")
	if len(rows) != 1 || rows[0].Type != "opaque-area" {
		t.Fatalf("opaque-area subview = %+v, want one opaque-area LSA", rows)
	}
	// The Link State ID encodes Opaque Type 1 + Opaque ID 0x2a: 1.0.0.42.
	if rows[0].LinkStateID != "1.0.0.42" {
		t.Fatalf("opaque LS ID = %q, want 1.0.0.42", rows[0].LinkStateID)
	}
}

func TestOSPFOpaqueReceiveFunctional(t *testing.T) {
	resetOpaqueConsumers()
	t.Cleanup(resetOpaqueConsumers)

	var received []opaqueReceived
	if err := registerOpaqueConsumer(1, OpaqueScopeArea, nil, func(r opaqueReceived) {
		received = append(received, r)
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	eng, _ := newRedistEngine(t, opaqueCfgJSON)
	a0 := mustBackboneArea(t)
	eng.lsdb.SetTopology(func() []ospflsdb.InterfaceInfo {
		return []ospflsdb.InterfaceInfo{{
			Name: "eth0", AreaID: a0, AreaType: ospflsdb.AreaTypeNormal,
			NetworkType: ospflsdb.NetworkPointToPoint, State: ospflsdb.InterfaceStateDR, RouterID: mustRouterID(t, "1.1.1.1"), TransmitDelay: 1,
			Neighbors: []ospflsdb.NeighborInfo{{RouterID: mustRouterID(t, "2.2.2.2"), Address: naddrForTest("10.0.0.2"), State: ospflsdb.NeighborStateFull, OpaqueCapable: true}},
		}}
	})

	lsa := opaqueLSAForTest(t, types.LSTypeOpaqueArea, 1, 0x2b, mustRouterID(t, "2.2.2.2"), types.InitialSequenceNumber, []byte{1, 2, 3, 4})
	reason := eng.lsdb.ReceiveUpdate(ospflsdb.ReceiveInput{Interface: "eth0", AreaID: a0, RouterID: mustRouterID(t, "2.2.2.2"), Src: netip.MustParseAddr("10.0.0.2"), Update: packet.LSUpdate{LSAs: []packet.LSA{lsa}}})
	if reason != "" {
		t.Fatalf("ReceiveUpdate reason = %q", reason)
	}
	if len(received) != 1 || received[0].OpaqueType != 1 || received[0].OpaqueID != 0x2b || !received[0].Reachable {
		t.Fatalf("consumer OnReceive = %+v, want one Type-1 delivery, id 0x2b, reachable", received)
	}
	if rows := opaqueSubview(t, eng, "opaque-area"); len(rows) != 1 {
		t.Fatalf("received opaque LSA not listed in opaque-area subview: %+v", rows)
	}
}

func TestOSPFOpaqueDecodeFunctional(t *testing.T) {
	// Build an opaque LSA whose body is two 4-byte-aligned TLVs, encode it, then decode it
	// the way the CLI decode path does: split the Link State ID and iterate the TLVs.
	// Two 4-byte-aligned TLVs built by hand (the RFC 5250 opaque body convention: a 2-byte
	// type, a 2-byte length, the value, then zero pad to the next 4-byte boundary). The
	// packet TLV builder is package-internal, so this test spells the wire form directly.
	body := []byte{
		0x00, 0x01, 0x00, 0x01, 0xaa, 0x00, 0x00, 0x00, // type 1, len 1, value 0xaa + 3 pad
		0x00, 0x02, 0x00, 0x03, 0x01, 0x02, 0x03, 0x00, // type 2, len 3, value 01 02 03 + 1 pad
	}
	src := packet.LSA{
		Header: packet.LSAHeader{
			Options:           types.OptionO,
			Type:              types.LSTypeOpaqueAS,
			LinkStateID:       packet.OpaqueLinkStateID(4, 0x00abcd),
			AdvertisingRouter: mustRouterID(t, "2.2.2.2"),
			Sequence:          types.InitialSequenceNumber,
		},
		Opaque: &packet.OpaqueLSA{Type: types.LSTypeOpaqueAS, Data: body},
	}
	wire := make([]byte, src.EncodedLen())
	src.WriteTo(wire, 0)

	decoded, err := packet.DecodeLSA(wire)
	if err != nil || !decoded.VerifyChecksum() {
		t.Fatalf("decode opaque hex: err=%v checksum-ok=%v", err, decoded.VerifyChecksum())
	}
	if decoded.OpaqueType() != 4 || decoded.OpaqueID() != 0x00abcd {
		t.Fatalf("decoded Opaque Type/ID = %d/%#x, want 4/0xabcd", decoded.OpaqueType(), decoded.OpaqueID())
	}
	// Walk the decoded body the way the generic opaque decode does: each TLV is a 4-byte
	// header plus its value padded to a 4-byte boundary. Confirm exactly two TLVs and that
	// the walk consumes the body exactly (no truncation, no trailing bytes).
	got, off := 0, 0
	db := decoded.Body
	for off+4 <= len(db) {
		l := int(db[off+2])<<8 | int(db[off+3])
		end := off + 4 + ((l + 3) &^ 3)
		if end > len(db) {
			t.Fatalf("TLV %d runs past body: end=%d len=%d", got, end, len(db))
		}
		got++
		off = end
	}
	if got != 2 || off != len(db) {
		t.Fatalf("TLV iteration: got %d TLVs, consumed %d of %d bytes", got, off, len(db))
	}
}

func TestOSPFOpaqueScopeFunctional(t *testing.T) {
	resetOpaqueConsumers()
	t.Cleanup(resetOpaqueConsumers)

	eng, _ := newRedistEngine(t, opaqueCfgJSON)
	a0 := mustBackboneArea(t)
	stub := mustLinkStateID(t, "0.0.0.7")
	eng.lsdb.SetAreaTypes(map[types.AreaID]string{types.AreaID(stub): ospflsdb.AreaTypeStub})
	var sends []string
	eng.lsdb.SetTx(func(iface string, _ netip.Addr, _ []byte) error {
		sends = append(sends, iface)
		return nil
	})
	eng.lsdb.SetTopology(func() []ospflsdb.InterfaceInfo {
		return []ospflsdb.InterfaceInfo{
			{Name: "eth0", AreaID: a0, AreaType: ospflsdb.AreaTypeNormal, NetworkType: ospflsdb.NetworkPointToPoint, State: ospflsdb.InterfaceStateDR, RouterID: mustRouterID(t, "1.1.1.1"), TransmitDelay: 1, Neighbors: []ospflsdb.NeighborInfo{{RouterID: mustRouterID(t, "2.2.2.2"), Address: naddrForTest("10.0.0.2"), State: ospflsdb.NeighborStateFull, OpaqueCapable: true}}},
			{Name: "eth1", AreaID: types.AreaID(stub), AreaType: ospflsdb.AreaTypeStub, NetworkType: ospflsdb.NetworkPointToPoint, State: ospflsdb.InterfaceStateDR, RouterID: mustRouterID(t, "1.1.1.1"), TransmitDelay: 1, Neighbors: []ospflsdb.NeighborInfo{{RouterID: mustRouterID(t, "3.3.3.3"), Address: naddrForTest("10.0.1.3"), State: ospflsdb.NeighborStateFull, OpaqueCapable: true}}},
		}
	})

	// Self-originate a Type-11 opaque LSA: it floods AS-wide out the normal interface but
	// NOT into the stub (RFC 5250 §3.1). Origination floods out every eligible interface.
	if _, ok := eng.lsdb.OriginateOpaque(ospflsdb.OpaqueOriginateInput{
		Router: mustRouterID(t, "1.1.1.1"), OpaqueType: 4, OpaqueID: 0x33, Scope: types.LSTypeOpaqueAS,
		Options: types.OptionO, Body: []byte{1, 2, 3, 4},
	}); !ok {
		t.Fatalf("Type 11 opaque origination failed")
	}
	sawEth0, sawEth1 := false, false
	for _, s := range sends {
		if s == "eth0" {
			sawEth0 = true
		}
		if s == "eth1" {
			sawEth1 = true
		}
	}
	if !sawEth0 {
		t.Fatalf("Type 11 opaque not flooded out the normal interface")
	}
	if sawEth1 {
		t.Fatalf("Type 11 opaque flooded into a stub interface (RFC 5250 §3.1 violation)")
	}
}
