package message

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestRFC7432EthernetSegmentRouteNeedsESImportNotESILabel proves that only the
// ES-Import Route Target satisfies the Ethernet Segment route's community rule.
//
// VALIDATES: an Ethernet Segment route with the ES-Import Route Target (type
// 0x06, sub-type 0x02) builds and carries it; the same route offered the ESI
// Label community (type 0x06, sub-type 0x01), which shares the type octet, with
// or without an ordinary RT, is refused with no UPDATE.
// PREVENTS: a guard that matches the EVPN type octet 0x06 alone and accepts the
// ESI Label as if it were the ES-Import Route Target.
//
// RFC requirement: RFC7432-8.1.1-2 positive -- the Ethernet Segment route carrying the
// ES-Import Route Target builds, and the UPDATE's extended communities are exactly it.
// RFC requirement: RFC7432-8.1.1-2 negative -- the ESI Label community (0x06/0x01) alone, or
// with an ordinary RT, in place of ES-Import is refused with ErrEVPNOrigination and no UPDATE.
func TestRFC7432EthernetSegmentRouteNeedsESImportNotESILabel(t *testing.T) {
	update, err := rfc7432Build(t, rfc7432SegmentNLRI(1), rfc7432ESImport)
	require.NoError(t, err)
	_, _, communities, found := attribute.AttrFind(update.PathAttributes, attribute.AttrExtCommunity)
	require.True(t, found)
	require.Equal(t, rfc7432ESImport, communities)

	require.Equal(t, rfc7432ESImport[0], rfc7432ESILabel[0], "the substitute must share the type octet")
	for _, offered := range [][][]byte{{rfc7432ESILabel}, {rfc7432ESILabel, rfc7432RT}} {
		update, err := rfc7432Build(t, rfc7432SegmentNLRI(1), offered...)
		require.ErrorIs(t, err, ErrEVPNOrigination)
		require.Nil(t, update)
	}
}
