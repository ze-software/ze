// Design: docs/architecture/core-design.md -- route reflection on the forward path
// Related: forward_rr_test.go -- the reflection harness and the CLUSTER_LIST cases

package reactor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRFC4456OriginatorIDIsTheOriginatorsBGPIdentifier proves the ORIGINATOR_ID a route
// reflector writes is the originator's BGP Identifier, told apart from the originator's
// transport address.
//
// RFC 4456 Section 8: "This attribute will carry the BGP Identifier of the originator of
// the route in the local AS."
//
// Method: the reflection harness uses a client source whose address is 10.0.0.1 and whose
// BGP Identifier (the router id from its OPEN) is 10.11.12.13, so a producer reading the
// address instead of the Identifier writes different bytes. A route with no ORIGINATOR_ID
// is reflected to a non-client and must carry 10.11.12.13. A route that already carries
// ORIGINATOR_ID 9.9.9.9, set by the originator's own reflector earlier in the local AS,
// must keep 9.9.9.9 rather than take the neighbor's Identifier.
//
// VALIDATES: ORIGINATOR_ID is the originator's BGP Identifier, created or kept.
// PREVENTS: an ORIGINATOR_ID filled from the neighbor address, from the reflector's own
// router id, or re-stamped at each reflection hop.
//
// RFC requirement: RFC4456-8-4 positive -- a reflected client route with no ORIGINATOR_ID carries the source's BGP Identifier 10.11.12.13, and a route already carrying ORIGINATOR_ID 9.9.9.9 keeps it.
// RFC requirement: RFC4456-8-4 negative -- the ORIGINATOR_ID written is neither the source's transport address 10.0.0.1, nor the reflector's router id 1.2.3.2, nor the neighbor's Identifier over an existing originator.
func TestRFC4456OriginatorIDIsTheOriginatorsBGPIdentifier(t *testing.T) {
	const sourceRouterID = 0x0A0B0C0D // 10.11.12.13, distinct from the source address 10.0.0.1.
	identifier := []byte{10, 11, 12, 13}
	sourceAddress := []byte{10, 0, 0, 1}
	reflectorID := []byte{1, 2, 3, 2}

	created := rrForwardFromRouterID(t, rrBodyBase(), true, false, sourceRouterID)
	require.NotNil(t, created, "a client's route is reflected to a non-client")
	createdID := decodeBodyAttrs(t, created)[9]
	require.Equal(t, identifier, createdID, "ORIGINATOR_ID is the originator's BGP Identifier")
	require.NotEqual(t, sourceAddress, createdID, "ORIGINATOR_ID is not the originator's address")
	require.NotEqual(t, reflectorID, createdID, "ORIGINATOR_ID is not the reflector's own id")

	carried := []byte{
		0, 0, // WithdrawnLen = 0
		0, 41, // TotalPathAttrLen = 41
		0x40, 1, 1, 0, // ORIGIN igp
		0x40, 2, 6, 2, 1, 0, 0, 0xFD, 0xE9, // AS_PATH [65001]
		0x40, 3, 4, 10, 0, 0, 254, // NEXT_HOP
		0x40, 5, 4, 0, 0, 0, 100, // LOCAL_PREF
		0x80, 4, 4, 0, 0, 0, 50, // MED
		0x80, 9, 4, 9, 9, 9, 9, // ORIGINATOR_ID 9.9.9.9 (already present)
		24, 192, 0, 2, // NLRI 192.0.2.0/24
	}
	kept := rrForwardFromRouterID(t, carried, true, false, sourceRouterID)
	require.NotNil(t, kept, "a client's route is reflected to a non-client")
	keptID := decodeBodyAttrs(t, kept)[9]
	require.Equal(t, []byte{9, 9, 9, 9}, keptID, "an existing ORIGINATOR_ID names the originator and is kept")
	require.NotEqual(t, identifier, keptID, "the neighbor's Identifier does not replace the originator's")
}
