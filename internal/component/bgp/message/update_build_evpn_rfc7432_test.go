// Design: docs/architecture/wire/nlri-evpn.md -- locally originated EVPN validation
// RFC: rfc/short/rfc7432.md

package message

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// rfc7432SegmentNLRI builds an Ethernet Segment route (type 4) whose RD
// carries the given RD type: [4][23][RD:8][ESI:10][32][IPv4:4].
func rfc7432SegmentNLRI(rdType byte) []byte {
	return []byte{4, 23,
		0, rdType, 192, 0, 2, 1, 0, 1,
		0, 1, 2, 3, 4, 5, 6, 7, 8, 9,
		32, 192, 0, 2, 1}
}

// rfc7432PerESNLRI builds an Ethernet A-D per ES route (type 1, MAX-ET tag,
// zero label): [1][25][RD:8][ESI:10][0xffffffff][label:3].
func rfc7432PerESNLRI() []byte {
	return []byte{1, 25,
		0, 1, 192, 0, 2, 1, 0, 1,
		0, 1, 2, 3, 4, 5, 6, 7, 8, 9,
		0xff, 0xff, 0xff, 0xff, 0, 0, 0}
}

var (
	rfc7432ESImport = []byte{0x06, 0x02, 0, 0x11, 0x22, 0x33, 0x44, 0x55}
	rfc7432RT       = []byte{0x00, 0x02, 0xfd, 0xe8, 0, 0, 0, 1}
	rfc7432ESILabel = []byte{0x06, 0x01, 0, 0, 0, 0, 0, 0}
)

func rfc7432Build(t *testing.T, nlri []byte, communities ...[]byte) (*Update, error) {
	t.Helper()
	var ext []byte
	for _, c := range communities {
		ext = append(ext, c...)
	}
	builder := NewUpdateBuilder(65001, false, true, false)
	return builder.BuildEVPN(EVPNParams{NLRI: nlri, NextHop: netip.MustParseAddr("192.0.2.1"),
		Origin: attribute.OriginIGP, ExtCommunityBytes: ext})
}

// VALIDATES: a locally originated Ethernet Segment route with a Type 1 RD and
// an ES-Import Route Target builds an UPDATE.
// PREVENTS: the origination guard refusing a conforming Ethernet Segment route.
func TestRFC7432EthernetSegmentOriginationAccepted(t *testing.T) {
	// RFC requirement: RFC7432-8.1.1-1 positive -- an Ethernet Segment route whose RD is Type 1 builds an UPDATE.
	// RFC requirement: RFC7432-8.1.1-2 positive -- an Ethernet Segment route carrying an ES-Import Route Target builds an UPDATE that carries it.
	update, err := rfc7432Build(t, rfc7432SegmentNLRI(1), rfc7432ESImport)
	require.NoError(t, err)
	_, _, communities, found := attribute.AttrFind(update.PathAttributes, attribute.AttrExtCommunity)
	require.True(t, found)
	require.Equal(t, rfc7432ESImport, communities)
}

// VALIDATES: the origination guard refuses an Ethernet Segment route whose RD
// is not Type 1, and one whose advertisement lacks the ES-Import Route Target.
// PREVENTS: originating an Ethernet Segment route RFC 7432 Section 8.1.1 forbids.
func TestRFC7432EthernetSegmentOriginationRefused(t *testing.T) {
	// RFC requirement: RFC7432-8.1.1-1 negative -- Type 0 and Type 2 RDs on an Ethernet Segment route are refused with no UPDATE.
	for _, rdType := range []byte{0, 2} {
		update, err := rfc7432Build(t, rfc7432SegmentNLRI(rdType), rfc7432ESImport)
		require.ErrorIs(t, err, ErrEVPNOrigination, "RD type %d", rdType)
		require.Nil(t, update)
	}
	// RFC requirement: RFC7432-8.1.1-2 negative -- an Ethernet Segment route with only an ordinary RT, or no community, is refused with no UPDATE.
	for _, communities := range [][][]byte{{rfc7432RT}, nil} {
		update, err := rfc7432Build(t, rfc7432SegmentNLRI(1), communities...)
		require.ErrorIs(t, err, ErrEVPNOrigination)
		require.Nil(t, update)
	}
}

// VALIDATES: an Ethernet A-D per ES route is originated only with the ESI
// Label extended community that distributes the ESI label.
// PREVENTS: a per-ES A-D route that omits the ESI label its peers need.
func TestRFC7432PerESRouteDistributesESILabel(t *testing.T) {
	// RFC requirement: RFC7432-8.3-2 positive -- an Ethernet A-D per ES route carrying the ESI Label extended community and an RT builds an UPDATE that carries both.
	update, err := rfc7432Build(t, rfc7432PerESNLRI(), rfc7432ESILabel, rfc7432RT)
	require.NoError(t, err)
	_, _, communities, found := attribute.AttrFind(update.PathAttributes, attribute.AttrExtCommunity)
	require.True(t, found)
	require.Equal(t, append(append([]byte{}, rfc7432ESILabel...), rfc7432RT...), communities)
	// RFC requirement: RFC7432-8.3-2 negative -- an Ethernet A-D per ES route without the ESI Label extended community is refused with no UPDATE.
	update, err = rfc7432Build(t, rfc7432PerESNLRI(), rfc7432RT)
	require.ErrorIs(t, err, ErrEVPNOrigination)
	require.Nil(t, update)
}
