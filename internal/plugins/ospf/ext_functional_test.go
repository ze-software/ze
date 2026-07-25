// VALIDATES: spec-ospf-ext-4 end-to-end user stories through the live engine -- the Extended
// Prefix/Link consumers register and are discovered; enabling extended-prefix/extended-link
// originates Opaque Type 7/8 LSAs through the ext-1 carrier that `show ospf database
// opaque-area`/`opaque-as` decode; a received Extended Prefix LSA is decoded and resolved; a
// registered sub-TLV codec is dispatched on receive; and the decode path splits the LS-ID to
// Opaque Type 7/8 with a decodable body.
// PREVENTS: the consumers compiling but not wired to origination, reception, the sub-TLV hook,
// or the CLI decode.
package ospf

import (
	"bytes"
	"net/netip"
	"testing"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

const extFnCfg = `{"ospf":{"router-id":"1.1.1.1","opaque":true,"extended-prefix":true,"extended-link":true,` +
	`"areas":{"area":{"0":{"area-id":"0"}}},` +
	`"interfaces":{"interface":{"eth0":{"name":"eth0","area":"0"}}}}}`

// extFnRegister builds a v4 engine from cfg with the Extended Prefix/Link opaque consumers
// registered, and (unless disabled) a canned stub-prefix topology installed as self LSAs.
func extFnRegister(t *testing.T) (*engine, types.RouterID) {
	t.Helper()
	resetOpaqueConsumers()
	t.Cleanup(resetOpaqueConsumers)
	eng, router := newRedistEngine(t, extFnCfg)
	if err := registerExtConsumers(eng); err != nil {
		t.Fatalf("registerExtConsumers: %v", err)
	}
	return eng, router
}

func TestOSPFExtRegisterFunctional(t *testing.T) {
	eng, _ := extFnRegister(t)
	if !eng.cfg.Opaque || !eng.cfg.ExtendedPrefix || !eng.cfg.ExtendedLink {
		t.Fatalf("config leaves not parsed: %+v", eng.cfg)
	}
	if _, ok := lookupOpaqueConsumer(packet.ExtPrefixOpaqueType); !ok {
		t.Fatalf("Extended Prefix consumer (Opaque Type 7) not discoverable")
	}
	if _, ok := lookupOpaqueConsumer(packet.ExtLinkOpaqueType); !ok {
		t.Fatalf("Extended Link consumer (Opaque Type 8) not discoverable")
	}
}

func TestOSPFExtPrefixOriginateFunctional(t *testing.T) {
	eng, _ := extFnRegister(t)
	eng.lsdb.SetTopology(func() []ospflsdb.InterfaceInfo {
		return []ospflsdb.InterfaceInfo{extStubIface("eth0", [4]byte{10, 0, 0, 1}, [4]byte{255, 255, 255, 0})}
	})
	// The engine drives OriginateFromTopology + the registered consumer's OnOriginate and
	// installs the Extended Prefix Opaque LSA into the area store.
	eng.originateSelfLSAs()

	db := eng.extOpaqueDecode(OpaqueScopeArea)
	found := false
	for _, p := range db.ExtendedPrefix {
		for _, tlv := range p.Prefixes {
			if tlv.RouteType == "intra-area" && tlv.Prefix == "10.0.0.0/24" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("Extended Prefix LSA for the connected /24 not originated/decoded: %+v", db.ExtendedPrefix)
	}
	// It rode the opaque carrier as an Opaque Type 7 area LSA.
	rows := opaqueSubview(t, eng, "opaque-area")
	seven := false
	for i := range rows {
		if rows[i].Type == "opaque-area" && rows[i].LinkStateID[:2] == "7." {
			seven = true
		}
	}
	if !seven {
		t.Fatalf("no Opaque Type 7 LSA in opaque-area: %+v", rows)
	}
}

func TestOSPFExtLinkOriginateFunctional(t *testing.T) {
	eng, _ := extFnRegister(t)
	eng.lsdb.SetTopology(func() []ospflsdb.InterfaceInfo {
		return []ospflsdb.InterfaceInfo{extP2PIface([4]byte{10, 0, 0, 1}, types.RouterID{2, 2, 2, 2}, "10.0.0.2")}
	})
	eng.originateSelfLSAs()

	db := eng.extOpaqueDecode(OpaqueScopeArea)
	if len(db.ExtendedLink) != 1 {
		t.Fatalf("want exactly one Extended Link LSA, got %d: %+v", len(db.ExtendedLink), db.ExtendedLink)
	}
	l := db.ExtendedLink[0]
	if l.LinkType != networkPointToPoint || l.LinkID != "2.2.2.2" || l.LinkData != "10.0.0.1" {
		t.Fatalf("Extended Link fields do not mirror the Router-LSA link: %+v", l)
	}
}

// extRecvInto sets a live topology so the LSDB accepts a flooded opaque LSA from adv.
func extRecvInto(t *testing.T, eng *engine, adv types.RouterID) {
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

func TestOSPFExtPrefixReceiveFunctional(t *testing.T) {
	eng, _ := extFnRegister(t)
	adv := mustRouterID(t, "2.2.2.2")
	extRecvInto(t, eng, adv)

	body := packet.EncodeExtPrefixLSA(packet.ExtPrefixLSA{Prefixes: []packet.ExtPrefixTLV{{
		RouteType: packet.ExtRouteTypeIntraArea, PrefixLength: 24, AF: packet.ExtPrefixAFIPv4Unicast, AddressPrefix: [4]byte{10, 5, 5, 0},
	}}})
	lsa := opaqueLSAForTest(t, types.LSTypeOpaqueArea, packet.ExtPrefixOpaqueType, 1, adv, types.InitialSequenceNumber, body)
	if reason := eng.lsdb.ReceiveUpdate(ospflsdb.ReceiveInput{Interface: "eth0", AreaID: mustBackboneArea(t), RouterID: adv, Src: netip.MustParseAddr("10.0.0.2"), Update: packet.LSUpdate{LSAs: []packet.LSA{lsa}}}); reason != "" {
		t.Fatalf("ReceiveUpdate: %q", reason)
	}
	if _, ok := eng.extRecv.lookupPrefix(adv, [5]byte{10, 5, 5, 0, 24}); !ok {
		t.Fatalf("received Extended Prefix not resolved into the receive store")
	}
	// The stored LSA decodes in `show ospf database opaque-area`.
	db := eng.extOpaqueDecode(OpaqueScopeArea)
	if len(db.ExtendedPrefix) == 0 {
		t.Fatalf("received Extended Prefix LSA not shown in opaque-area decode")
	}
}

func TestOSPFExtSubTLVHookFunctional(t *testing.T) {
	eng, _ := extFnRegister(t)
	resetExtSubTLVs()
	t.Cleanup(resetExtSubTLVs)
	var got []byte
	if err := registerPrefixSubTLV(7, extSubTLVCodec{Receive: func(v []byte) { got = append([]byte(nil), v...) }}); err != nil {
		t.Fatalf("registerPrefixSubTLV: %v", err)
	}
	adv := mustRouterID(t, "2.2.2.2")
	extRecvInto(t, eng, adv)
	body := packet.EncodeExtPrefixLSA(packet.ExtPrefixLSA{Prefixes: []packet.ExtPrefixTLV{{
		RouteType: packet.ExtRouteTypeIntraArea, PrefixLength: 32, AF: packet.ExtPrefixAFIPv4Unicast, AddressPrefix: [4]byte{10, 9, 9, 9},
		SubTLVs: []packet.ExtSubTLV{
			{Type: 7, Value: []byte{0xde, 0xad, 0xbe, 0xef}},
			{Type: 200, Value: []byte{1, 2}}, // unknown -> skipped without error
		},
	}}})
	lsa := opaqueLSAForTest(t, types.LSTypeOpaqueArea, packet.ExtPrefixOpaqueType, 1, adv, types.InitialSequenceNumber, body)
	if reason := eng.lsdb.ReceiveUpdate(ospflsdb.ReceiveInput{Interface: "eth0", AreaID: mustBackboneArea(t), RouterID: adv, Src: netip.MustParseAddr("10.0.0.2"), Update: packet.LSUpdate{LSAs: []packet.LSA{lsa}}}); reason != "" {
		t.Fatalf("ReceiveUpdate: %q", reason)
	}
	if !bytes.Equal(got, []byte{0xde, 0xad, 0xbe, 0xef}) {
		t.Fatalf("registered sub-TLV codec not dispatched, got %x", got)
	}
}

func TestOSPFExtDecodeFunctional(t *testing.T) {
	// An Extended Prefix opaque LSA (Opaque Type 7) encodes, decodes with a valid checksum,
	// splits to Opaque Type 7, and its body decodes to an Extended Prefix TLV (User Story 5).
	body := packet.EncodeExtPrefixLSA(packet.ExtPrefixLSA{Prefixes: []packet.ExtPrefixTLV{{
		RouteType: packet.ExtRouteTypeIntraArea, PrefixLength: 24, AF: packet.ExtPrefixAFIPv4Unicast, AddressPrefix: [4]byte{10, 1, 2, 0},
	}}})
	src := packet.LSA{
		Header: packet.LSAHeader{
			Options: types.OptionO, Type: types.LSTypeOpaqueArea,
			LinkStateID:       packet.OpaqueLinkStateID(packet.ExtPrefixOpaqueType, 3),
			AdvertisingRouter: mustRouterID(t, "2.2.2.2"), Sequence: types.InitialSequenceNumber,
		},
		Opaque: &packet.OpaqueLSA{Type: types.LSTypeOpaqueArea, Data: body},
	}
	wire := make([]byte, src.EncodedLen())
	src.WriteTo(wire, 0)
	decoded, err := packet.DecodeLSA(wire)
	if err != nil || !decoded.VerifyChecksum() {
		t.Fatalf("decode Extended Prefix hex: err=%v checksum=%v", err, decoded.VerifyChecksum())
	}
	if decoded.OpaqueType() != packet.ExtPrefixOpaqueType {
		t.Fatalf("opaque type = %d, want 7", decoded.OpaqueType())
	}
	lsa, err := packet.DecodeExtPrefixLSA(decoded.Body)
	if err != nil || len(lsa.Prefixes) != 1 || lsa.Prefixes[0].AddressPrefix != [4]byte{10, 1, 2, 0} {
		t.Fatalf("Extended Prefix body decode: err=%v lsa=%+v", err, lsa)
	}
}
