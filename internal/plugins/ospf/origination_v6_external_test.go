// VALIDATES: spec-ospf-af-unify -- OSPFv3 ASBR redistribution origination. A redistributed
// IPv6 route becomes an AS-External-LSA (0x4005) in the AS-wide store, and the router's
// Router-LSA gains the E-bit (RFC 5340 App A.4.3/A.4.7). PREVENTS: a v6 ASBR that never
// advertises its externals, or a Router-LSA without the E-bit so peers do not treat it as
// an ASBR.
package ospf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

func TestOSPFv6OriginateExternal(t *testing.T) {
	e := newV6OriginEngine()
	router := types.RouterID{172, 30, 0, 2}
	p, ok := netipToV6Prefix(netip.MustParsePrefix("2001:db8:e::/64"), 0)
	if !ok {
		t.Fatal("netipToV6Prefix failed")
	}
	lsid := v6SummaryLSID(1)

	if !e.v6OriginateExternalLSA(router, lsid, p, true, 40, 0) {
		t.Fatal("v6OriginateExternalLSA returned false")
	}

	// AS-External is AS-wide: visible from any area (here the backbone).
	lsa, found := e.lsdb.LookupLSA(types.BackboneArea, v6ExternalKey(router, lsid))
	if !found {
		t.Fatal("AS-External LSA not installed in the AS-wide store")
	}
	if !ospfv3packet.VerifyLSAChecksum(lsa.RawBytes) {
		t.Fatal("AS-External Fletcher checksum invalid")
	}
	decoded, err := ospfv3packet.DecodeLSA(lsa.RawBytes)
	if err != nil {
		t.Fatalf("DecodeLSA: %v", err)
	}
	body, err := decoded.DecodeExternal()
	if err != nil {
		t.Fatalf("DecodeExternal: %v", err)
	}
	if !body.ExternalType2 || body.Metric != 40 {
		t.Errorf("type2/metric = %v/%d, want true/40", body.ExternalType2, body.Metric)
	}
	if gp, _ := v6PrefixToNetip(body.Prefix, afIPv6Unicast); gp != netip.MustParsePrefix("2001:db8:e::/64") {
		t.Errorf("prefix = %s, want 2001:db8:e::/64", gp)
	}

	// The Router-LSA must now carry the E-bit (this router is an ASBR).
	h, ok := e.v6OriginateRouter(types.BackboneArea, router, ospfv3types.OptV6|ospfv3types.OptR, nil, false, false, false, false)
	if !ok {
		t.Fatal("v6OriginateRouter returned false")
	}
	rlsa, _ := e.lsdb.LookupLSA(types.BackboneArea, h.Key())
	rdec, err := ospfv3packet.DecodeLSA(rlsa.RawBytes)
	if err != nil {
		t.Fatalf("DecodeLSA(router): %v", err)
	}
	rbody, err := rdec.DecodeRouter()
	if err != nil {
		t.Fatalf("DecodeRouter: %v", err)
	}
	if rbody.Flags&ospfv3packet.RouterFlagE == 0 {
		t.Error("Router-LSA E-bit not set despite a self-originated AS-External")
	}
}

func TestOSPFv6ExternalMetricMaskedTo24Bits(t *testing.T) {
	// RFC 5340 App A.4.7: the AS-External metric is a 24-bit field. A redistributed metric above
	// 0x00ffffff is clamped to the low 24 bits (matching the NSSA path and the v4 encoder), not
	// silently truncated on the wire. Boundary: the last valid 24-bit value is unchanged, the
	// first over-range value loses its high byte, and an arbitrary over-range value keeps its low
	// 24 bits.
	e := newV6OriginEngine()
	router := types.RouterID{172, 30, 0, 2}
	p, ok := netipToV6Prefix(netip.MustParsePrefix("2001:db8:e7::/64"), 0)
	if !ok {
		t.Fatal("netipToV6Prefix failed")
	}
	cases := []struct{ in, want uint32 }{
		{0x00ffffff, 0x00ffffff}, // last valid 24-bit value, unchanged
		{0x01000000, 0x00000000}, // first over-24-bit value, high byte masked off
		{0x01abcdef, 0x00abcdef}, // arbitrary over-range value, low 24 bits kept
	}
	for i, c := range cases {
		lsid := v6SummaryLSID(uint32(i + 1)) // distinct LSID per case avoids re-origination rate-limiting
		if !e.v6OriginateExternalLSA(router, lsid, p, true, c.in, 0) {
			t.Fatalf("v6OriginateExternalLSA(%#x) returned false", c.in)
		}
		lsa, found := e.lsdb.LookupLSA(types.BackboneArea, v6ExternalKey(router, lsid))
		if !found {
			t.Fatalf("AS-External LSA not installed for metric %#x", c.in)
		}
		decoded, err := ospfv3packet.DecodeLSA(lsa.RawBytes)
		if err != nil {
			t.Fatalf("DecodeLSA: %v", err)
		}
		body, err := decoded.DecodeExternal()
		if err != nil {
			t.Fatalf("DecodeExternal: %v", err)
		}
		if body.Metric != c.want {
			t.Errorf("metric %#x originated as %#x, want %#x (24-bit clamp)", c.in, body.Metric, c.want)
		}
	}
}
