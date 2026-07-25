// VALIDATES: the pure Prefix-SID resolution helpers of the TI-LFA SRResolver in
// sr_tilfa.go -- resolvePrefixSIDLabel (absolute V=1/L=1 label returned as-is vs an
// index resolved through the originator's SRGB, RFC 8665 §3.2/§5), preferPrefix (the
// determinism tie-break: prefer a /32 node prefix, then the numerically smaller
// address), and the srTILFAResolver.PrefixSIDLabel method end-to-end over a flooded
// Extended Prefix Opaque LSA.
// PREVENTS: emitting an index as an absolute MPLS label; resolving an index when the
// originator advertised no SRGB; a non-deterministic prefix choice when a router
// originates several Prefix-SIDs.
package ospf

import (
	"net/netip"
	"testing"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestResolvePrefixSIDLabel(t *testing.T) {
	srgb := sr.NewSRGB([]sr.LabelRange{{Base: 16000, Size: 100}})

	// Absolute label form (V=1/L=1): returned verbatim, SRGB irrelevant.
	abs := srRemotePrefixSID{SID: sr.PrefixSID{IsLabel: true, Label: 16050}}
	if lbl, ok := resolvePrefixSIDLabel(abs, sr.SRGB{}); !ok || lbl != 16050 {
		t.Fatalf("absolute label resolve = (%d,%v), want (16050,true) even with an empty SRGB", lbl, ok)
	}

	// Index form resolves through the originator's SRGB: base 16000 + index 5 = 16005.
	idx := srRemotePrefixSID{SID: sr.PrefixSID{Index: 5}}
	if lbl, ok := resolvePrefixSIDLabel(idx, srgb); !ok || lbl != 16005 {
		t.Fatalf("index resolve = (%d,%v), want (16005,true)", lbl, ok)
	}

	// Index form with no advertised SRGB cannot resolve.
	if _, ok := resolvePrefixSIDLabel(idx, sr.SRGB{}); ok {
		t.Fatalf("index form with an empty SRGB must not resolve to a label")
	}

	// Index outside the SRGB range cannot resolve (SRGB.Label returns false).
	if _, ok := resolvePrefixSIDLabel(srRemotePrefixSID{SID: sr.PrefixSID{Index: 100}}, srgb); ok {
		t.Fatalf("index past the SRGB size must not resolve (index 100 into a 100-label block)")
	}
}

func TestPreferPrefix(t *testing.T) {
	host := netip.MustParsePrefix("10.0.0.1/32")
	hostHi := netip.MustParsePrefix("10.0.0.9/32")
	net24 := netip.MustParsePrefix("10.0.0.0/24")
	net24b := netip.MustParsePrefix("10.0.1.0/24")

	// A /32 host prefix is always preferred over a non-/32.
	if !preferPrefix(host, net24) {
		t.Fatalf("a /32 candidate must be preferred over a /24 current")
	}
	if preferPrefix(net24, host) {
		t.Fatalf("a /24 candidate must NOT be preferred over a /32 current")
	}
	// Same mask length: the numerically smaller address wins (deterministic tie-break).
	if !preferPrefix(host, hostHi) {
		t.Fatalf("10.0.0.1/32 must be preferred over 10.0.0.9/32 (smaller address)")
	}
	if preferPrefix(hostHi, host) {
		t.Fatalf("10.0.0.9/32 must NOT be preferred over 10.0.0.1/32")
	}
	if preferPrefix(net24b, net24) {
		t.Fatalf("10.0.1.0/24 must NOT be preferred over 10.0.0.0/24")
	}
}

func TestPrefixSIDLabelNilEngine(t *testing.T) {
	// The resolver guards a nil engine (the IPv4-only install path leaves it nil for v6).
	if lbl, ok := (srTILFAResolver{}).PrefixSIDLabel(types.RouterID{1, 1, 1, 1}); ok || lbl != 0 {
		t.Fatalf("nil-engine PrefixSIDLabel = (%d,%v), want (0,false)", lbl, ok)
	}
}

func TestPrefixSIDLabelResolvesFromFloodedLSA(t *testing.T) {
	// End-to-end: a remote router (adv) floods an Extended Prefix Opaque LSA (RFC 7684)
	// for its /32 loopback carrying an absolute-label Prefix-SID (V=1/L=1). PrefixSIDLabel
	// walks the received Prefix-SIDs, matches the originator, and returns that label.
	eng, _ := extFnRegister(t)
	adv := mustRouterID(t, "3.3.3.3")
	extRecvInto(t, eng, adv)

	psValue := sr.EncodePrefixSIDValue(sr.PrefixSID{Flags: sr.SIDFlags{V: true, L: true}, Label: 16333})
	body := packet.EncodeExtPrefixLSA(packet.ExtPrefixLSA{Prefixes: []packet.ExtPrefixTLV{{
		RouteType:     packet.ExtRouteTypeIntraArea,
		PrefixLength:  32,
		AF:            packet.ExtPrefixAFIPv4Unicast,
		AddressPrefix: [4]byte{3, 3, 3, 3},
		SubTLVs:       []packet.ExtSubTLV{{Type: sr.V4TypePrefixSID, Value: psValue}},
	}}})
	lsa := opaqueLSAForTest(t, types.LSTypeOpaqueArea, packet.ExtPrefixOpaqueType, 1, adv, types.InitialSequenceNumber, body)
	if reason := eng.lsdb.ReceiveUpdate(ospflsdb.ReceiveInput{
		Interface: "eth0", AreaID: mustBackboneArea(t), RouterID: adv,
		Src: netip.MustParseAddr("10.0.0.2"), Update: packet.LSUpdate{LSAs: []packet.LSA{lsa}},
	}); reason != "" {
		t.Fatalf("ReceiveUpdate(ext-prefix): %q", reason)
	}

	lbl, ok := srTILFAResolver{e: eng}.PrefixSIDLabel(adv)
	if !ok || lbl != 16333 {
		t.Fatalf("PrefixSIDLabel(adv) = (%d,%v), want (16333,true) from the flooded absolute-label Prefix-SID", lbl, ok)
	}

	// A router that advertised no Prefix-SID does not resolve.
	if _, ok := (srTILFAResolver{e: eng}).PrefixSIDLabel(mustRouterID(t, "9.9.9.9")); ok {
		t.Fatalf("PrefixSIDLabel must not resolve a router that advertised no Prefix-SID")
	}
}
