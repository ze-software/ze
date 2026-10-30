// Design: docs/architecture/rib/unified-locrib.md -- the fields a Loc-RIB import filter reads.
//
// VALIDATES: the OSPFv3 external reader (v6ExternalReader) carries the External
// Route Tag of an AS-External-LSA and of an NSSA-LSA when bit T is set, and
// reports no tag when bit T is clear (RFC 5340 Section A.4.7).
// PREVENTS: an OSPFv3 route reaching the Loc-RIB with a tag it never had, or
// losing the one it had, so a Loc-RIB `match { tag [ ... ]; }` filter misjudges it.

package ospf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// TestV6ExternalReaderCarriesRouteTag reads, for each external LS type, one LSA
// with bit T and tag 4242 and one without bit T. Method: the untagged LSA
// carries a non-zero tag value in its struct, which the encoder MUST NOT write
// without bit T, so a reader that ignored HasRouteTag reports a tag and fails.
func TestV6ExternalReaderCarriesRouteTag(t *testing.T) {
	prefix, ok := netipToV6Prefix(netip.MustParsePrefix("10.7.0.0/24"), 0)
	if !ok {
		t.Fatal("netipToV6Prefix rejected an IPv4 prefix")
	}
	for _, lsType := range []ospfv3types.LSType{ospfv3types.LSTypeASExternal, ospfv3types.LSTypeNSSA} {
		tagged := ospfv3packet.LSA{Header: rfc5838V6Header(lsType, 1), External: &ospfv3packet.ExternalLSA{
			Metric: 9, Prefix: prefix, HasRouteTag: true, ExternalRouteTag: 4242,
		}}
		untagged := ospfv3packet.LSA{Header: rfc5838V6Header(lsType, 2), External: &ospfv3packet.ExternalLSA{
			Metric: 9, Prefix: prefix, ExternalRouteTag: 77,
		}}
		src := rfc5838V6Source(tagged, untagged)
		read := v6ExternalReader(src, afIPv4Unicast)

		rec, ok := read(types.BackboneArea, src.headers[0])
		if !ok {
			t.Fatalf("%v: tagged LSA yielded no record", lsType)
		}
		if !rec.HasTag || rec.Tag != 4242 {
			t.Errorf("%v: tagged record has tag %d (has-tag %v), want 4242", lsType, rec.Tag, rec.HasTag)
		}
		rec, ok = read(types.BackboneArea, src.headers[1])
		if !ok {
			t.Fatalf("%v: untagged LSA yielded no record", lsType)
		}
		if rec.HasTag {
			t.Errorf("%v: untagged record reports tag %d, want none (bit T clear)", lsType, rec.Tag)
		}
	}
}
