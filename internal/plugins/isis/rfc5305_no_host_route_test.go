// Design: docs/architecture/isis/isis-9-spf-rib.md -- received LSPs to installed IPv4 routes.
// RFC: rfc/short/rfc5305.md -- Sections 3.2, 3.3 and 4.3 (no /32 from the TE addresses).
//
// VALIDATES: a received LSP whose Extended IS Reachability entry carries the
// IPv4 interface address (sub-TLV 6) and IPv4 neighbor address (sub-TLV 8), and
// which carries a Traffic Engineering Router ID (TLV 134), is used for SPF and
// its reachability is installed, while none of those three addresses becomes a
// /32 route in the Loc-RIB or a BGP-LS Prefix NLRI.
// PREVENTS: a host route for a TE address, which RFC 5305 bans because a router
// that does not support the sub-TLV would forward differently and loop.
// Method: the LSP goes through the codec and the receive LSDB path
// (metricEngine), the SPF runs into a real Loc-RIB, and the BGP-LS builder reads
// the same LSDB.

package isis

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

var (
	rfc5305InterfaceAddr = netip.MustParseAddr("203.0.113.1")
	rfc5305NeighborAddr  = netip.MustParseAddr("203.0.113.2")
	rfc5305RouterID      = netip.MustParseAddr("203.0.113.9")
)

// rfc5305Engine stores system 2's L1 LSP a second time, now with sub-TLV 6 and 8
// on its adjacency to the root and a TLV 134, beside the TLV 128 prefix
// metricEngine gave it.
func rfc5305Engine(t *testing.T) (*engine, *locrib.RIB) {
	t.Helper()
	peer := metricAdvertisement{level: lsdb.Level1, id: 2, cost: 10, metric: 10}
	e, loc := metricEngine(t, []metricAdvertisement{peer})

	metric, err := types.NewMetric(peer.cost)
	if err != nil {
		t.Fatal(err)
	}
	local, remote := rfc5305InterfaceAddr.As4(), rfc5305NeighborAddr.As4()
	subs := []byte{6, 4, local[0], local[1], local[2], local[3], 8, 4, remote[0], remote[1], remote[2], remote[3]}
	link := make([]byte, types.SourceIDLen+types.MetricLen+1, types.SourceIDLen+types.MetricLen+1+len(subs))
	off := types.NewSourceID(metricSystem(1), 0).WriteTo(link, 0)
	off += metric.WriteTo(link, off)
	link[off] = byte(len(subs))
	link = append(link, subs...)

	prefix := packet.NarrowIPReachTLV{Entries: []packet.NarrowIPReachEntry{{
		DefaultMetricValue: peer.metric, DelayMetric: 0x80, ExpenseMetric: 0x80, ErrorMetric: 0x80, Prefix: metricPrefix,
	}}}
	reach := make([]byte, prefix.EncodedLen())
	prefix.WriteTo(reach, 0)
	routerID := rfc5305RouterID.As4()

	lsp := packet.LSP{PDUType: packet.PDUTypeL1LSP, RemainingLifetime: 1200,
		LSPID:          types.NewLSPID(types.NewSourceID(metricSystem(peer.id), 0), 0),
		SequenceNumber: 2,
		TLVs: []packet.TLV{
			{Type: packet.TLVExtendedISReach, Value: link},
			{Type: reach[0], Value: reach[2:]},
			{Type: 134, Value: routerID[:]},
		}}
	raw := make([]byte, lsp.EncodedLen())
	lsp.WriteTo(raw, 0)
	decoded, err := packet.DecodePDU(raw)
	if err != nil {
		t.Fatal(err)
	}
	defer packet.ReleaseTLVs(decoded.LSP.TLVs)
	if result := e.lsdb.Receive(lsdb.Level1, decoded.LSP, raw, false); !result.Stored {
		t.Fatalf("LSP with the TE addresses not received: %+v", result)
	}
	e.spf.Run()
	return e, loc
}

// RFC requirement: RFC5305-3.2-1 positive -- with sub-TLV 6 (IPv4 interface address) on the received adjacency, the Loc-RIB Ze builds holds exactly the LSP's TLV 128 prefix, via the neighbor: the only IPv4 route installed is 198.51.100.0/24.
// RFC requirement: RFC5305-3.3-1 positive -- with sub-TLV 8 (IPv4 neighbor address) on the received adjacency, the Loc-RIB Ze builds holds exactly the LSP's TLV 128 prefix, via the neighbor: the only IPv4 route installed is 198.51.100.0/24.
// RFC requirement: RFC5305-4.3-4 positive -- with a TLV 134 (Traffic Engineering Router ID) in the received LSP, the Loc-RIB Ze builds holds exactly the LSP's TLV 128 prefix, via the neighbor: the only IPv4 route installed is 198.51.100.0/24.
func TestRFC5305TEAddressesLeaveOnlyReachabilityRoutes(t *testing.T) {
	_, loc := rfc5305Engine(t)

	group, ok := loc.Lookup(family.IPv4Unicast, metricPrefix)
	if !ok || len(group.Paths) != 1 {
		t.Fatalf("reachability route %s: group = %+v, present = %v", metricPrefix, group, ok)
	}
	if want := netip.AddrFrom4([4]byte{192, 0, 2, 2}); group.Paths[0].NextHop != want {
		t.Fatalf("reachability route %s via %s, want via %s", metricPrefix, group.Paths[0].NextHop, want)
	}
	if n := loc.Len(family.IPv4Unicast); n != 1 {
		loc.Iterate(family.IPv4Unicast, func(prefix netip.Prefix, _ locrib.PathGroup) bool {
			t.Logf("installed: %s", prefix)
			return true
		})
		t.Fatalf("installed %d IPv4 routes, want only %s", n, metricPrefix)
	}
}

// RFC requirement: RFC5305-3.2-1 negative -- the IPv4 interface address of received sub-TLV 6 (203.0.113.1) is decoded by the BGP-LS builder as a link address, yet no /32 for it is in the Loc-RIB and no BGP-LS Prefix NLRI carries it.
// RFC requirement: RFC5305-3.3-1 negative -- the IPv4 neighbor address of received sub-TLV 8 (203.0.113.2) is decoded by the BGP-LS builder as a link address, yet no /32 for it is in the Loc-RIB and no BGP-LS Prefix NLRI carries it.
// RFC requirement: RFC5305-4.3-4 negative -- the router ID of a received TLV 134 (203.0.113.9) is exported by the BGP-LS builder as node attribute 1028, yet no /32 for it is in the Loc-RIB and no BGP-LS Prefix NLRI carries it.
func TestRFC5305TEAddressesInjectNoHostRoute(t *testing.T) {
	e, loc := rfc5305Engine(t)

	var builder bgplsBuilder
	builder.build(e.lsdb.RawSnapshot(lsdb.Level1))
	decodedLocal, decodedRemote := false, false
	for i := range builder.snapshot.Links {
		for _, a := range builder.snapshot.Links[i].LocalAddresses {
			decodedLocal = decodedLocal || a == rfc5305InterfaceAddr
		}
		for _, a := range builder.snapshot.Links[i].RemoteAddresses {
			decodedRemote = decodedRemote || a == rfc5305NeighborAddr
		}
	}
	if !decodedLocal || !decodedRemote {
		t.Fatalf("BGP-LS builder did not decode the sub-TLV addresses: local %v remote %v, links %+v", decodedLocal, decodedRemote, builder.snapshot.Links)
	}
	routerID := rfc5305RouterID.As4()
	exported := false
	for i := range builder.snapshot.Nodes {
		exported = exported || rfc9552HasAttribute(builder.snapshot.Nodes[i].Attributes, 1028, routerID[:])
	}
	if !exported {
		t.Fatalf("BGP-LS builder did not export the TLV 134 router ID as node attribute 1028")
	}

	for _, addr := range []netip.Addr{rfc5305InterfaceAddr, rfc5305NeighborAddr, rfc5305RouterID} {
		host := netip.PrefixFrom(addr, 32)
		if _, ok := loc.Lookup(family.IPv4Unicast, host); ok {
			t.Errorf("Loc-RIB holds a /32 route for the TE address %s", host)
		}
		for i := range builder.snapshot.Prefixes {
			if builder.snapshot.Prefixes[i].Prefix.Contains(addr) {
				t.Errorf("BGP-LS Prefix NLRI %s carries the TE address %s", builder.snapshot.Prefixes[i].Prefix, addr)
			}
		}
	}
}
