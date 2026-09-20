// VALIDATES: RFC 8362 Section 2 -- the RFC 8362 Extended LSAs ze originates carry the
// U-bit SET in the two LS Type octets that go on the wire, and the RFC 5340 base LSAs
// originated by the same engine still carry it clear.
// PREVENTS: an Extended LSA stranded at the first router that does not implement RFC 8362,
// which takes every RFC 8666 Prefix-SID ze advertises with it; and a blanket U-bit that
// would corrupt the base types' wire values.
package ospf

import (
	"encoding/binary"
	"net/netip"
	"testing"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// wireLSType reads the LS Type field out of an OSPFv3 LSA as it was encoded for the wire.
// RFC 5340 Appendix A.4.1 lays the LSA header out as LS Age (2 octets) then LS Type
// (2 octets), so the field is at offset 2. Reading the stored bytes rather than the LSDB
// key is the point of these tests: the key is ze's own value, and the octets are what a
// peer sees.
func wireLSType(t *testing.T, eng *engine, area types.AreaID, key types.LSAKey) uint16 {
	t.Helper()
	lsa, ok := eng.lsdb.LookupLSA(area, key)
	if !ok {
		t.Fatalf("no LSA stored for key %+v", key)
	}
	if len(lsa.RawBytes) < 4 {
		t.Fatalf("LSA for key %+v carries %d raw octets, too short for an LS Type", key, len(lsa.RawBytes))
	}
	return binary.BigEndian.Uint16(lsa.RawBytes[2:4])
}

// originateSRExtendedLSAs turns segment routing on for router and runs the OSPFv3 SR
// origination pass, so the Extended LSAs under test are built by the production path
// rather than by a test encoder.
func originateSRExtendedLSAs(t *testing.T, eng *engine, router types.RouterID) {
	t.Helper()
	nbr := types.RouterID{2, 2, 2, 2}
	const adjLabel uint32 = 40001
	eng.srAdj = &srAdjManager{labels: map[srAdjKey]srAdjRecord{
		{iface: "eth0", router: nbr}: {label: adjLabel, adj: sr.AdjSID{
			Flags: sr.AdjSIDFlags{V: true, L: true}, Label: adjLabel, IsLabel: true,
		}},
	}}
	srWire.set(router, sr.SRConfig{
		Enabled:  true,
		SRGB:     []sr.LabelRange{{Base: 16000, Size: 8000}},
		Prefixes: []sr.PrefixSIDConfig{{Prefix: netip.MustParsePrefix("2001:db8::1/128"), Index: 1, NodeSID: true}},
	})
	t.Cleanup(func() { srWire.set(router, sr.SRConfig{}) })

	byArea := map[types.AreaID][]ospflsdb.InterfaceInfo{types.BackboneArea: {{
		Name: "eth0", NetworkType: types.NetworkPointToPoint, InterfaceID: 5, Cost: 10,
		Neighbors: []ospflsdb.NeighborInfo{{RouterID: nbr, State: ospflsdb.NeighborStateFull, InterfaceID: 6}},
	}}}
	keep := map[ospflsdb.SelfLSARef]struct{}{}
	if n := eng.v6OriginateSR(router, byArea, false, []types.AreaID{types.BackboneArea}, keep); n == 0 {
		t.Fatalf("the SR origination pass originated no Extended LSA")
	}
}

// RFC requirement: RFC8362-2-1 positive -- every RFC 8362 Extended LSA ze puts on the wire
// carries the U-bit set in its LS Type field. The SR origination pass builds an
// E-Router-LSA and an E-Intra-Area-Prefix-LSA (v6OriginateSR, sr_origination_v6.go), and
// the two octets stored for the wire read 0xA021 and 0xA029, the values RFC 8362 Section 2
// assigns. With the U-bit clear (0x2021 / 0x2029) RFC 5340 Appendix A.4.2.1 tells a router
// that does not recognize the function code to "Treat the LSA as if it had link-local
// flooding scope", so the LSA and its RFC 8666 Prefix-SID stop at the first such router.
func TestRFC8362ExtendedLSAsSetUBitOnTheWire(t *testing.T) {
	eng := newV6RIEngine(t)
	router := types.RouterID{1, 1, 1, 1}
	eng.lsdb.SetSelfRouterID(router)
	originateSRExtendedLSAs(t, eng, router)

	cases := []struct {
		name string
		key  types.LSAKey
		want uint16
	}{
		{"E-Router-LSA", v6ERouterKey(router), 0xA021},
		{"E-Intra-Area-Prefix-LSA", v6EIntraAreaPrefixKey(router), 0xA029},
	}
	for _, c := range cases {
		got := wireLSType(t, eng, types.BackboneArea, c.key)
		if got != c.want {
			t.Errorf("%s wire LS Type = 0x%04X, want 0x%04X", c.name, got, c.want)
		}
		if got&0x8000 == 0 {
			t.Errorf("%s wire LS Type 0x%04X has the U-bit clear", c.name, got)
		}
	}
}

// RFC requirement: RFC8362-2-1 negative -- the U-bit is set for the Extended LSAs and for
// nothing else: the same OSPFv3 engine originates an AS-External-LSA whose wire LS Type is
// 0x4005, U-bit CLEAR, because RFC 5340 Appendix A.4.2.1 assigns U=0 to every base type.
// A blanket U-bit would make the base LSAs unrecognizable to a conformant peer, so this
// pins the requirement to the Extended types the RFC 8362 registry covers.
func TestRFC8362BaseLSAsKeepUBitClearOnTheWire(t *testing.T) {
	eng := newV6RIEngine(t)
	router := types.RouterID{1, 1, 1, 1}
	eng.lsdb.SetSelfRouterID(router)

	prefix, ok := netipToV6Prefix(netip.MustParsePrefix("2001:db8:1::/48"), 0)
	if !ok {
		t.Fatalf("2001:db8:1::/48 must convert to an OSPFv3 wire prefix")
	}
	lsid := v6SummaryLSID(1)
	if !eng.v6OriginateExternalLSA(router, lsid, prefix, true, 20, 0) {
		t.Fatalf("the AS-External-LSA was not originated")
	}
	got := wireLSType(t, eng, types.BackboneArea, v6ExternalKey(router, lsid))
	if got != uint16(ospfv3types.LSTypeASExternal) {
		t.Errorf("AS-External-LSA wire LS Type = 0x%04X, want 0x4005", got)
	}
	if got&0x8000 != 0 {
		t.Errorf("AS-External-LSA wire LS Type 0x%04X must keep the U-bit clear", got)
	}
}
