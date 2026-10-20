// Design: docs/architecture/wire/nlri-bgpls.md -- native OSPFv3 Extended LSA export.
//
// VALIDATES: RFC 8362 Sections 3.2 and 3.3, a Router-Link TLV is read only from an
// E-Router-LSA and an Attached-Routers TLV only from an E-Network-LSA, so the same TLV
// carried by another Extended LSA is ignored.
// PREVENTS: a TLV type that is only meaningful in one Extended LSA inventing topology when
// it appears in another one.
package ospf

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/linkstateevents"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	v3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	v3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// rfc8362RouterLinkTLV returns a Router-Link TLV (RFC 8362 Section 3.2): one
// point-to-point link of metric 10 from interface 7 to interface 8 of neighbor.
func rfc8362RouterLinkTLV(neighbor types.RouterID) v3packet.ExtendedTLV {
	value := make([]byte, 16)
	value[0] = v3packet.RouterLinkTypeP2P
	binary.BigEndian.PutUint16(value[2:4], 10)
	binary.BigEndian.PutUint32(value[4:8], 7)
	binary.BigEndian.PutUint32(value[8:12], 8)
	copy(value[12:16], neighbor[:])
	return v3packet.ExtendedTLV{Type: extTLVRouterLink, Value: value}
}

// rfc8362AttachedRoutersTLV returns an Attached-Routers TLV (RFC 8362 Section 3.3, type 2)
// listing routers.
func rfc8362AttachedRoutersTLV(routers ...types.RouterID) v3packet.ExtendedTLV {
	var value []byte
	for _, router := range routers {
		value = append(value, router[:]...)
	}
	return v3packet.ExtendedTLV{Type: 2, Value: value}
}

// rfc8362PrefixValue returns an Intra/Inter-Area-Prefix TLV value (RFC 8362 Sections 3.4
// and 3.7): metric 10 and the /64 prefix 2001:db8:0:<word>::/64.
func rfc8362PrefixValue(word byte) []byte {
	return []byte{0, 0, 0, 10, 64, 0, 0, 0, 0x20, 1, 0x0d, 0xb8, 0, 0, 0, word}
}

// rfc8362EIntraBody returns an E-Intra-Area-Prefix-LSA body (RFC 8362 Section 4.8) that
// references router's E-Router-LSA, followed by tlvs.
func rfc8362EIntraBody(router types.RouterID, tlvs ...v3packet.ExtendedTLV) []byte {
	header := make([]byte, eIntraPrefixHeaderLen)
	binary.BigEndian.PutUint16(header[2:4], uint16(v3types.LSTypeERouter))
	copy(header[8:12], router[:])
	return append(header, v3packet.EncodeExtendedLSABody(v3packet.ExtendedLSA{TLVs: tlvs})...)
}

// rfc8362EFixedBody returns an E-Router-LSA or E-Network-LSA body: four octets of
// flags and options, then tlvs.
func rfc8362EFixedBody(tlvs ...v3packet.ExtendedTLV) []byte {
	return append(make([]byte, 4), v3packet.EncodeExtendedLSABody(v3packet.ExtendedLSA{TLVs: tlvs})...)
}

// rfc8362Install installs one OSPFv3 LSA from router in the backbone of e's LSDB.
func rfc8362Install(t *testing.T, e *engine, router types.RouterID, typ v3types.LSType, id uint32, body []byte) {
	t.Helper()
	lsa := bgplsNativeV3(t, v3types.RouterID(router), typ, id, body, types.InitialSequenceNumber, false)
	if !e.lsdb.Install(types.BackboneArea, lsa) {
		t.Fatalf("install LS type %04x from %v", typ, router)
	}
}

// rfc8362RemoteRouters returns the remote router IDs of every exported link.
func rfc8362RemoteRouters(links []linkstateevents.Link) [][]byte {
	out := make([][]byte, 0, len(links))
	for i := range links {
		out = append(out, links[i].Remote.RouterID)
	}
	return out
}

// TestRFC8362RouterLinkTLVReadFromERouter proves the positive half of Section 3.2.
//
// Goal: a Router-Link TLV in its own LSA, the E-Router-LSA, becomes topology.
// Method: install one E-Router-LSA from 2.2.2.2 carrying one point-to-point Router-Link TLV
// into the LSDB, and read the BGP-LS snapshot the native exporter publishes. Exactly that
// link is exported, from 2.2.2.2 to 4.4.4.4 with interface IDs 7 and 8.
//
// RFC requirement: RFC8362-3.2-1 positive -- an E-Router-LSA installed in the LSDB has its
// Router-Link TLV exported through BGP-LS as exactly one link, from the advertising router
// 2.2.2.2 to neighbor 4.4.4.4 with local interface ID 7 and remote interface ID 8.
func TestRFC8362RouterLinkTLVReadFromERouter(t *testing.T) {
	e, _, events := bgplsTestSource(t, true)
	rfc8362Install(t, e, types.RouterID{2, 2, 2, 2}, v3types.LSTypeERouter, 0,
		rfc8362EFixedBody(rfc8362RouterLinkTLV(types.RouterID{4, 4, 4, 4})))

	snapshot := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool { return len(s.Links) != 0 })
	if len(snapshot.Links) != 1 {
		t.Fatalf("links = %+v, want exactly the one Router-Link TLV", snapshot.Links)
	}
	link := snapshot.Links[0]
	if !bytes.Equal(link.Local.RouterID, []byte{2, 2, 2, 2}) || !bytes.Equal(link.Remote.RouterID, []byte{4, 4, 4, 4}) {
		t.Fatalf("link endpoints = %v -> %v, want 2.2.2.2 -> 4.4.4.4", link.Local.RouterID, link.Remote.RouterID)
	}
	if !link.HasLinkIDs || link.LocalID != 7 || link.RemoteID != 8 {
		t.Fatalf("link interface IDs = %d/%d (present %v), want 7/8", link.LocalID, link.RemoteID, link.HasLinkIDs)
	}
}

// TestRFC8362RouterLinkTLVIgnoredOutsideERouter proves the negative half of Section 3.2.
//
// Goal: a Router-Link TLV carried by another Extended LSA is ignored.
// Method: install one E-Intra-Area-Prefix-LSA from 2.2.2.2, referencing its E-Router-LSA,
// that carries a well-formed Router-Link TLV beside an Intra-Area-Prefix TLV. The prefix TLV
// is exported, which proves the LSA was read, and no link is exported at all.
//
// RFC requirement: RFC8362-3.2-1 negative -- a well-formed Router-Link TLV carried in an
// E-Intra-Area-Prefix-LSA exports no BGP-LS link, while the Intra-Area-Prefix TLV beside it
// in the same LSA is exported as 2001:db8::/64.
func TestRFC8362RouterLinkTLVIgnoredOutsideERouter(t *testing.T) {
	e, _, events := bgplsTestSource(t, true)
	router := types.RouterID{2, 2, 2, 2}
	rfc8362Install(t, e, router, v3types.LSTypeEIntraAreaPrefix, 1, rfc8362EIntraBody(router,
		rfc8362RouterLinkTLV(types.RouterID{4, 4, 4, 4}),
		v3packet.ExtendedTLV{Type: extTLVIntraAreaPrefix, Value: rfc8362PrefixValue(0)}))

	snapshot := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool { return len(s.Prefixes) != 0 })
	if len(snapshot.Prefixes) != 1 || snapshot.Prefixes[0].Prefix != netip.MustParsePrefix("2001:db8::/64") {
		t.Fatalf("prefixes = %+v, want only 2001:db8::/64", snapshot.Prefixes)
	}
	if len(snapshot.Links) != 0 {
		t.Fatalf("a Router-Link TLV outside the E-Router-LSA was exported: %+v", snapshot.Links)
	}
}

// TestRFC8362AttachedRoutersTLVReadFromENetwork proves the positive half of Section 3.3.
//
// Goal: an Attached-Routers TLV in its own LSA, the E-Network-LSA, becomes topology.
// Method: install one E-Network-LSA from DR 2.2.2.2, Link State ID 5, listing 2.2.2.2 and
// 3.3.3.3. The exporter publishes one link from the pseudonode 2.2.2.2/5 to each router.
//
// RFC requirement: RFC8362-3.3-1 positive -- an E-Network-LSA installed in the LSDB has its
// Attached-Routers TLV exported through BGP-LS as exactly two links, from the pseudonode
// 2.2.2.2 interface 5 to 2.2.2.2 and to 3.3.3.3.
func TestRFC8362AttachedRoutersTLVReadFromENetwork(t *testing.T) {
	e, _, events := bgplsTestSource(t, true)
	rfc8362Install(t, e, types.RouterID{2, 2, 2, 2}, v3types.LSTypeENetwork, 5,
		rfc8362EFixedBody(rfc8362AttachedRoutersTLV(types.RouterID{2, 2, 2, 2}, types.RouterID{3, 3, 3, 3})))

	snapshot := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool { return len(s.Links) != 0 })
	if len(snapshot.Links) != 2 {
		t.Fatalf("links = %+v, want one per attached router", snapshot.Links)
	}
	pseudonode := []byte{2, 2, 2, 2, 0, 0, 0, 5}
	want := [][]byte{{2, 2, 2, 2}, {3, 3, 3, 3}}
	for i := range snapshot.Links {
		if !bytes.Equal(snapshot.Links[i].Local.RouterID, pseudonode) {
			t.Fatalf("link %d local = %v, want pseudonode %v", i, snapshot.Links[i].Local.RouterID, pseudonode)
		}
	}
	got := rfc8362RemoteRouters(snapshot.Links)
	if !bytes.Equal(got[0], want[0]) || !bytes.Equal(got[1], want[1]) {
		t.Fatalf("attached routers = %v, want %v", got, want)
	}
}

// TestRFC8362AttachedRoutersTLVIgnoredOutsideENetwork proves the negative half of
// Section 3.3.
//
// Goal: an Attached-Routers TLV carried by another Extended LSA is ignored.
// Method: install one E-Router-LSA from 2.2.2.2 that carries a Router-Link TLV to 4.4.4.4
// and an Attached-Routers TLV listing 3.3.3.3. The Router-Link TLV is exported, which proves
// the LSA was read, and it is the only link: nothing reaches 3.3.3.3.
//
// RFC requirement: RFC8362-3.3-1 negative -- an Attached-Routers TLV listing 3.3.3.3 carried
// in an E-Router-LSA exports no BGP-LS link, while the Router-Link TLV to 4.4.4.4 in the same
// LSA is exported as the only link.
func TestRFC8362AttachedRoutersTLVIgnoredOutsideENetwork(t *testing.T) {
	e, _, events := bgplsTestSource(t, true)
	rfc8362Install(t, e, types.RouterID{2, 2, 2, 2}, v3types.LSTypeERouter, 0, rfc8362EFixedBody(
		rfc8362RouterLinkTLV(types.RouterID{4, 4, 4, 4}),
		rfc8362AttachedRoutersTLV(types.RouterID{3, 3, 3, 3})))

	snapshot := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool { return len(s.Links) != 0 })
	got := rfc8362RemoteRouters(snapshot.Links)
	if len(got) != 1 || !bytes.Equal(got[0], []byte{4, 4, 4, 4}) {
		t.Fatalf("link remotes = %v, want only the Router-Link TLV's 4.4.4.4", got)
	}
}
