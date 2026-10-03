package reactor

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// Goal: prove the sender half of RFC 2918 Section 3's Reserved octet: every
// ROUTE-REFRESH request ze sends carries 0 in the octet between AFI and SAFI.
// Method: drive both producers of a request, the operator route-refresh
// (sendRouteRefresh) and the soft clear (SoftClearPeer), against an
// Established peer backed by a recording connection, and read the octet at
// body offset 2 of what reaches the wire.
//
// VALIDATES: RFC 2918 Section 3, Reserved set to 0 by the sender.
// PREVENTS: a request encoder that writes a non-zero Reserved octet.
//
// RFC requirement: RFC2918-3-4 positive -- a ROUTE-REFRESH request written by sendRouteRefresh and by SoftClearPeer carries AFI 1, Reserved 0 and SAFI 1 in its four body octets.
func TestRFC2918SentRequestReservedOctetIsZero(t *testing.T) {
	want := []byte{0, 1, 0, 1}

	adapter, _, conn := newRefreshPeer(t, true)
	err := adapter.sendRouteRefresh(selector.All(), uint16(family.AFIIPv4), uint8(family.SAFIUnicast),
		message.RouteRefreshNormal, plugin.OperatorSender())
	require.NoError(t, err)
	written := conn.written()
	require.Len(t, written, message.HeaderLen+4)
	require.Equal(t, want, written[message.HeaderLen:])

	adapter, _, conn = newRefreshPeer(t, true)
	_, err = adapter.SoftClearPeer(selector.All(), plugin.OperatorSender())
	require.NoError(t, err)
	written = conn.written()
	require.Len(t, written, message.HeaderLen+4)
	require.Equal(t, want, written[message.HeaderLen:])
}
