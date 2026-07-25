// VALIDATES: spec-ospf-ext-2 end-to-end user stories through the live engine -- the TE
// opaque consumer registers and is discovered; a configured TE link originates a
// Router-Address + Link LSA that appears in `show ospf database opaque-area`; a received TE
// LSA is delivered through the carrier into the TED and listed by `show ospf te-database`;
// an inter-AS link originates at the configured Type 10/11 scope; TE hex decodes to TLVs;
// and a received withdraw clears the TED entry.
// PREVENTS: the TE consumer compiling but not actually wired to origination, reception,
// the CLI views, or the withdraw path.
package ospf

import (
	"net/netip"
	"testing"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

const teFnOriginCfg = `{"ospf":{"router-id":"1.1.1.1","router-address":"9.9.9.9","opaque":true,
  "areas":{"area":{"0":{"area-id":"0"}}},
  "interfaces":{"interface":{"eth0":{"name":"eth0","area":"0","network-type":"point-to-point",
    "traffic-engineering":{"enable":true,"max-bandwidth":"1250000000","admin-group":"3"}}}}}}`

func teFnRegister(t *testing.T, cfgJSON string) *engine {
	t.Helper()
	resetOpaqueConsumers()
	t.Cleanup(resetOpaqueConsumers)
	eng, _ := newRedistEngine(t, cfgJSON)
	if err := registerTEConsumer(eng); err != nil {
		t.Fatalf("registerTEConsumer: %v", err)
	}
	return eng
}

func TestOSPFTERegisterFunctional(t *testing.T) {
	eng := teFnRegister(t, teCfgJSON)
	if !eng.cfg.Opaque {
		t.Fatalf("opaque leaf not parsed true")
	}
	// The engine discovers the registered TE consumer on the opaque-origination pass.
	if _, ok := lookupOpaqueConsumer(packet.TEOpaqueType); !ok {
		t.Fatalf("TE consumer (type 1) not discoverable")
	}
	if _, ok := lookupOpaqueConsumer(packet.InterAsTEOpaqueType); !ok {
		t.Fatalf("inter-AS TE consumer (type 6) not discoverable")
	}
}

func TestOSPFTEOriginateFunctional(t *testing.T) {
	eng := teFnRegister(t, teFnOriginCfg)
	eng.teOrig.setTopology(func() []ospflsdb.InterfaceInfo {
		return []ospflsdb.InterfaceInfo{p2pTopo("eth0", [4]byte{10, 0, 0, 1}, types.RouterID{2, 2, 2, 2}, "10.0.0.2")}
	})
	// The engine drives the registered consumer's OnOriginate and installs into the area store.
	eng.originateOpaqueLSAs(types.RouterID{1, 1, 1, 1})

	rows := opaqueSubview(t, eng, "opaque-area")
	links, routerAddr := 0, 0
	for i := range rows {
		if rows[i].Type != "opaque-area" {
			continue
		}
		// The Link State ID high byte is Opaque type 1 (RFC 3630 TE).
		if rows[i].LinkStateID == "1.0.0.0" {
			routerAddr++
		} else {
			links++
		}
	}
	if routerAddr != 1 || links < 1 {
		t.Fatalf("opaque-area TE LSAs: router-address=%d links=%d, want 1 and >=1", routerAddr, links)
	}
	// The opaque-area view decodes the TE bodies inline (AC-16), not as raw hex.
	decoded := eng.teOpaqueAreaDecode(OpaqueScopeArea)
	if len(decoded) < 2 {
		t.Fatalf("opaque-area TE decode returned %d, want the Router-Address + Link", len(decoded))
	}
}

// teLSAInto sets a live topology (interface eth0 in the backbone with a Full opaque-capable
// neighbor) so the LSDB accepts flooded TE opaque LSAs from adv.
func teLSAInto(t *testing.T, eng *engine, adv types.RouterID) {
	t.Helper()
	eng.lsdb.SetTopology(func() []ospflsdb.InterfaceInfo {
		return []ospflsdb.InterfaceInfo{{
			Name: "eth0", AreaID: types.BackboneArea, AreaType: ospflsdb.AreaTypeNormal,
			NetworkType: ospflsdb.NetworkPointToPoint, State: ospflsdb.InterfaceStateDR,
			RouterID: mustRouterID(t, "1.1.1.1"), TransmitDelay: 1,
			Neighbors: []ospflsdb.NeighborInfo{{RouterID: adv, Address: naddrForTest("10.0.0.2"), State: ospflsdb.NeighborStateFull, OpaqueCapable: true}},
		}}
	})
}

func TestOSPFTEReceiveFunctional(t *testing.T) {
	eng := teFnRegister(t, teCfgJSON)
	a0 := mustBackboneArea(t)
	adv := mustRouterID(t, "2.2.2.2")
	teLSAInto(t, eng, adv)

	lsa := opaqueLSAForTest(t, types.LSTypeOpaqueArea, packet.TEOpaqueType, 1, adv, types.InitialSequenceNumber, teLinkBody(t))
	if reason := eng.lsdb.ReceiveUpdate(ospflsdb.ReceiveInput{Interface: "eth0", AreaID: a0, RouterID: adv, Src: netip.MustParseAddr("10.0.0.2"), Update: packet.LSUpdate{LSAs: []packet.LSA{lsa}}}); reason != "" {
		t.Fatalf("ReceiveUpdate: %q", reason)
	}
	if n := len(eng.ted.Snapshot().Links); n != 1 {
		t.Fatalf("received TE LSA not in TED (%d links)", n)
	}
	view := teDatabaseView1(t, eng)
	if len(view.Links) != 1 || view.Links[0].LinkID != "2.2.2.2" {
		t.Fatalf("show ospf te-database missing the received link: %+v", view.Links)
	}
}

func TestOSPFTEInterASFunctional(t *testing.T) {
	const cfg = `{"ospf":{"router-id":"1.1.1.1","router-address":"9.9.9.9","opaque":true,
	  "areas":{"area":{"0":{"area-id":"0"}}},
	  "interfaces":{"interface":{"eth0":{"name":"eth0","area":"0","network-type":"point-to-point",
	    "traffic-engineering":{"enable":true,"inter-as":{"remote-as":"65001","remote-asbr-ipv4":"203.0.113.9","scope":"as"}}}}}}}`
	eng := teFnRegister(t, cfg)
	eng.originateOpaqueLSAs(types.RouterID{1, 1, 1, 1})
	// scope=as -> Type 11 (AS-wide) opaque LSA (RFC 5392 sec 3.1.1).
	rows := opaqueSubview(t, eng, "opaque-as")
	found := false
	for i := range rows {
		if rows[i].Type == "opaque-as" {
			found = true
		}
	}
	if !found {
		t.Fatalf("inter-AS TE LSA not flooded as Type 11 (opaque-as): %+v", rows)
	}

	// A received Type-11 inter-AS TE LSA (Opaque type 6) with Remote AS + IPv4 ASBR ID and no
	// Link ID is parsed into the TED (AC-7).
	adv := mustRouterID(t, "2.2.2.2")
	teLSAInto(t, eng, adv)
	body := packet.TELSA{IsLink: true, Link: packet.TELink{
		HasLinkType: true, LinkType: packet.TELinkTypePointToPoint,
		HasRemoteAS: true, RemoteAS: 65002, HasRemoteASBRv4: true, RemoteASBRv4: [4]byte{198, 51, 100, 7},
	}}.Encode()
	lsa := opaqueLSAForTest(t, types.LSTypeOpaqueAS, packet.InterAsTEOpaqueType, 5, adv, types.InitialSequenceNumber, body)
	if reason := eng.lsdb.ReceiveUpdate(ospflsdb.ReceiveInput{Interface: "eth0", AreaID: mustBackboneArea(t), RouterID: adv, Src: netip.MustParseAddr("10.0.0.2"), Update: packet.LSUpdate{LSAs: []packet.LSA{lsa}}}); reason != "" {
		t.Fatalf("ReceiveUpdate (type 11): %q", reason)
	}
	interAS := false
	for _, l := range eng.ted.Snapshot().Links {
		if l.Link.IsInterAS() && l.Link.RemoteAS == 65002 {
			interAS = true
		}
	}
	if !interAS {
		t.Fatalf("received Type-11 inter-AS LSA not in TED")
	}
}

func TestOSPFTEDecodeFunctional(t *testing.T) {
	// A TE opaque LSA (Opaque type 1) encodes, decodes with a valid checksum, splits to
	// Opaque type 1, and its body decodes to a Link TLV with sub-TLVs (User Story 4 / AC-16).
	src := packet.LSA{
		Header: packet.LSAHeader{
			Options: types.OptionO, Type: types.LSTypeOpaqueArea,
			LinkStateID:       packet.OpaqueLinkStateID(packet.TEOpaqueType, 1),
			AdvertisingRouter: mustRouterID(t, "2.2.2.2"), Sequence: types.InitialSequenceNumber,
		},
		Opaque: &packet.OpaqueLSA{Type: types.LSTypeOpaqueArea, Data: teLinkBody(t)},
	}
	wire := make([]byte, src.EncodedLen())
	src.WriteTo(wire, 0)
	decoded, err := packet.DecodeLSA(wire)
	if err != nil || !decoded.VerifyChecksum() {
		t.Fatalf("decode TE hex: err=%v checksum=%v", err, decoded.VerifyChecksum())
	}
	if decoded.OpaqueType() != packet.TEOpaqueType {
		t.Fatalf("opaque type = %d, want 1", decoded.OpaqueType())
	}
	lsa, err := packet.DecodeTELSA(decoded.Body)
	if err != nil || !lsa.IsLink || !lsa.Link.HasLinkID {
		t.Fatalf("TE body decode: err=%v link=%+v", err, lsa)
	}
}

func TestOSPFTEWithdrawFunctional(t *testing.T) {
	// test-relax: a MaxAge purge arriving within MinLSArrival of the install is throttled by
	// the LSDB (orthogonal, RFC 2328 behavior), so the withdraw is driven through the
	// carrier's delivery seam after a full-wire install; same reception->TED withdraw path.
	eng := teFnRegister(t, teCfgJSON)
	a0 := mustBackboneArea(t)
	adv := mustRouterID(t, "2.2.2.2")
	teLSAInto(t, eng, adv)

	// Install a link through the full wire reception path.
	lsa := opaqueLSAForTest(t, types.LSTypeOpaqueArea, packet.TEOpaqueType, 1, adv, types.InitialSequenceNumber, teLinkBody(t))
	if reason := eng.lsdb.ReceiveUpdate(ospflsdb.ReceiveInput{Interface: "eth0", AreaID: a0, RouterID: adv, Src: netip.MustParseAddr("10.0.0.2"), Update: packet.LSUpdate{LSAs: []packet.LSA{lsa}}}); reason != "" {
		t.Fatalf("ReceiveUpdate: %q", reason)
	}
	if len(eng.ted.Snapshot().Links) != 1 {
		t.Fatalf("link not installed in TED")
	}
	// The carrier delivers a MaxAge purge with Withdrawn set (RFC 2328 sec 14); the consumer
	// removes the TED entry (AC-14).
	eng.deliverOpaque(ospflsdb.OpaqueDelivery{
		Scope: types.LSTypeOpaqueArea, Area: a0, AdvertisingRouter: adv,
		OpaqueType: packet.TEOpaqueType, OpaqueID: 1, Body: teLinkBody(t), Withdrawn: true,
	})
	if n := len(eng.ted.Snapshot().Links); n != 0 {
		t.Fatalf("withdraw left %d stale TED links", n)
	}
}

func TestOSPFTEShowFunctional(t *testing.T) {
	eng := teFnRegister(t, teCfgJSON)
	a0 := mustBackboneArea(t)
	adv := mustRouterID(t, "2.2.2.2")
	teLSAInto(t, eng, adv)
	ra := opaqueLSAForTest(t, types.LSTypeOpaqueArea, packet.TEOpaqueType, 0, adv, types.InitialSequenceNumber,
		packet.TELSA{IsRouterAddress: true, RouterAddress: [4]byte{9, 9, 9, 9}}.Encode())
	// A re-originated link instance carries a higher sequence than the Router-Address LSA.
	link := opaqueLSAForTest(t, types.LSTypeOpaqueArea, packet.TEOpaqueType, 1, adv, types.InitialSequenceNumber+1, teLinkBody(t))
	if reason := eng.lsdb.ReceiveUpdate(ospflsdb.ReceiveInput{Interface: "eth0", AreaID: a0, RouterID: adv, Src: netip.MustParseAddr("10.0.0.2"), Update: packet.LSUpdate{LSAs: []packet.LSA{ra, link}}}); reason != "" {
		t.Fatalf("ReceiveUpdate: %q", reason)
	}
	view := teDatabaseView1(t, eng)
	if len(view.RouterAddresses) != 1 || view.RouterAddresses[0].Address != "9.9.9.9" {
		t.Fatalf("te-database router addresses = %+v", view.RouterAddresses)
	}
	if len(view.Links) != 1 || view.Links[0].TEMetric == nil {
		t.Fatalf("te-database links = %+v", view.Links)
	}
}
