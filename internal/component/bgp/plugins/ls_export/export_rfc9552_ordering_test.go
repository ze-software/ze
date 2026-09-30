// Design: docs/architecture/wire/nlri-bgpls.md -- native origination contract
// RFC: rfc/short/rfc9552.md
// Related: export_encode.go -- encodeNativeLink sorts the Link Descriptors
//
// VALIDATES: the Link NLRI the native producer originates is in the canonical
// TLV order of RFC 9552 Section 5.1: ascending Type, and among TLVs of one
// Type ascending Length and then ascending Value. The producer repeats a Type
// whenever a link has several global addresses of one family, so the
// same-type rule is exercised on the output a collector receives.
// PREVENTS: a Link NLRI a BGP-LS Propagator must treat as malformed, and one
// link keyed two ways because its addresses were stored in another order.
package ls_export

import (
	"context"
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/linkstateevents"
)

// linkNLRITLV is one top-level TLV of an originated Link NLRI.
type linkNLRITLV struct {
	kind  uint16
	value string
}

// nativeLinkTLVs originates one IS-IS link with the given addresses and
// returns the top-level TLVs of its NLRI, in wire order, after the 13-octet
// NLRI header (Type, Length, Protocol-ID, Identifier).
func nativeLinkTLVs(t *testing.T, local, remote []string) []linkNLRITLV {
	t.Helper()
	e, capture := exportFixture(t)
	link := linkstateevents.Link{Local: nativeTestNode(), Remote: nativeTestNode()}
	for _, addr := range local {
		link.LocalAddresses = append(link.LocalAddresses, netip.MustParseAddr(addr))
	}
	for _, addr := range remote {
		link.RemoteAddresses = append(link.RemoteAddresses, netip.MustParseAddr(addr))
	}
	snapshot := &linkstateevents.Snapshot{Domain: linkstateevents.Domain{Protocol: linkstateevents.ISISLevel1},
		Generation: 1, Links: []linkstateevents.Link{link}}
	require.NoError(t, e.replace("isis", snapshot))
	require.NoError(t, e.reconcile(context.Background()))
	require.Len(t, capture.commands, 1)

	wire := exportCommandBytes(t, capture.commands[0], "nlri")[13:]
	var tlvs []linkNLRITLV
	for len(wire) > 0 {
		require.GreaterOrEqual(t, len(wire), 4)
		n := int(binary.BigEndian.Uint16(wire[2:4]))
		require.GreaterOrEqual(t, len(wire), 4+n)
		tlvs = append(tlvs, linkNLRITLV{kind: binary.BigEndian.Uint16(wire[:2]), value: string(wire[4 : 4+n])})
		wire = wire[4+n:]
	}
	return tlvs
}

// requireCanonicalLinkTLVs checks the Link Descriptor TLVs after the two node
// descriptors against the exact sequence Section 5.1 defines for these
// addresses, and that no pair of neighbors is out of order by Type, then by
// Length, then by Value.
func requireCanonicalLinkTLVs(t *testing.T, tlvs, want []linkNLRITLV) {
	t.Helper()
	require.GreaterOrEqual(t, len(tlvs), 2)
	require.Equal(t, uint16(256), tlvs[0].kind, "Local Node Descriptors first")
	require.Equal(t, uint16(257), tlvs[1].kind, "Remote Node Descriptors second")
	require.Equal(t, want, tlvs[2:])
	for i := 1; i < len(tlvs); i++ {
		before, after := tlvs[i-1], tlvs[i]
		require.LessOrEqual(t, before.kind, after.kind, "TLV %d follows %d", after.kind, before.kind)
		if before.kind != after.kind {
			continue
		}
		require.LessOrEqual(t, len(before.value), len(after.value), "same-type TLV %d longer before shorter", after.kind)
		if len(before.value) == len(after.value) {
			require.Less(t, before.value, after.value, "same-type TLV %d not ascending by Value", after.kind)
		}
	}
}

// addrTLV is the expected Link Descriptor TLV for one address.
func addrTLV(kind uint16, addr string) linkNLRITLV {
	return linkNLRITLV{kind: kind, value: string(netip.MustParseAddr(addr).AsSlice())}
}

// TestRFC9552NativeLinkTLVsCanonicalOrder originates a link whose addresses are
// stored in ascending order within each family, the order a well-behaved source
// hands over.
//
// RFC requirement: RFC9552-5.1-1 positive -- an originated Link NLRI carrying TLVs 259, 260, 261, 262 and the IS-IS MT-ID 263 emits them in ascending Type order, after TLVs 256 and 257 (§5.1).
// RFC requirement: RFC7752-3.1-2 positive -- the same originated Link NLRI is in ascending Type order, the rule RFC9552-5.1-1 restates (Section 3.1).
// RFC requirement: RFC9552-5.1-2 positive -- two TLV 259 and two TLV 261 of one NLRI are emitted ascending by Value; each repeated Type the producer emits has one fixed Length (4 for 259/260, 16 for 261/262), so the Length key ties and the check that no longer TLV precedes a shorter one of its Type holds (§5.1).
// RFC requirement: RFC7752-3.1-3 positive -- the same-type TLVs 259 and 261 are emitted ascending by Value compared leftmost octet first; every repeated Type the producer emits has one fixed Length, so the predecessor's "regardless of the length" comparison and RFC 9552's Length-then-Value comparison order this output identically (Section 3.1).
func TestRFC9552NativeLinkTLVsCanonicalOrder(t *testing.T) {
	tlvs := nativeLinkTLVs(t,
		[]string{"192.0.2.1", "192.0.2.7", "2001:db8::1", "2001:db8::9"},
		[]string{"198.51.100.2", "2001:db8:1::2"})
	requireCanonicalLinkTLVs(t, tlvs, []linkNLRITLV{
		addrTLV(259, "192.0.2.1"), addrTLV(259, "192.0.2.7"),
		addrTLV(260, "198.51.100.2"),
		addrTLV(261, "2001:db8::1"), addrTLV(261, "2001:db8::9"),
		addrTLV(262, "2001:db8:1::2"),
		{kind: 263, value: "\x00\x00"},
	})
}

// TestRFC9552NativeLinkTLVsReorderedSource hands the producer the same kind of
// link with every family interleaved and each family in descending order, the
// input that would emit a descending pair if the producer wrote the addresses
// in source order. The emitted NLRI is still canonical.
//
// RFC requirement: RFC9552-5.1-1 negative -- addresses stored IPv6 before IPv4 and remote before local in Type terms never produce a descending Type pair: the NLRI still reads 259, 260, 261, 262, 263 (§5.1).
// RFC requirement: RFC7752-3.1-2 negative -- the same interleaved source never produces a descending Type pair (Section 3.1).
// RFC requirement: RFC9552-5.1-2 negative -- same-type addresses stored in descending Value order (192.0.2.7 before 192.0.2.1, 2001:db8::9 before 2001:db8::1, and two remote TLV 260 and 262 likewise) are emitted ascending by Value, never in source order (§5.1).
// RFC requirement: RFC7752-3.1-3 negative -- same-type addresses stored in descending Value order are emitted ascending by Value compared leftmost octet first, never in source order (Section 3.1).
func TestRFC9552NativeLinkTLVsReorderedSource(t *testing.T) {
	tlvs := nativeLinkTLVs(t,
		[]string{"2001:db8::9", "192.0.2.7", "2001:db8::1", "192.0.2.1"},
		[]string{"2001:db8:1::9", "198.51.100.9", "2001:db8:1::2", "198.51.100.2"})
	requireCanonicalLinkTLVs(t, tlvs, []linkNLRITLV{
		addrTLV(259, "192.0.2.1"), addrTLV(259, "192.0.2.7"),
		addrTLV(260, "198.51.100.2"), addrTLV(260, "198.51.100.9"),
		addrTLV(261, "2001:db8::1"), addrTLV(261, "2001:db8::9"),
		addrTLV(262, "2001:db8:1::2"), addrTLV(262, "2001:db8:1::9"),
		{kind: 263, value: "\x00\x00"},
	})
}
