package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
)

// Goal: prove a well-formed ROUTE-REFRESH whose Message Subtype is outside 0, 1
// and 2 is ignored by the session itself: no plugin, no subscriber and no
// refresh counter ever sees it.
// Method: drive a 4-octet body with subtypes 3, 5, 42 and 255 into an
// Established session whose peer advertised capability 70, and count what
// reaches onMessageReceived and onRefreshRecv. The subtypes 0, 1 and 2 go
// through the same exchange and must be delivered.
//
// VALIDATES: RFC 7313 Section 5, an unknown subtype is ignored before delivery,
// at the same point the malformed-length unknown subtype is ignored.
// PREVENTS: a receive path that hands an unknown subtype to every subscriber
// and leaves the ignore to whichever consumer happens to test the octet.
//
// RFC requirement: RFC7313-5-3 positive -- a 4-octet ROUTE-REFRESH with Message Subtype 3, 5, 42 or 255 from a peer that advertised capability 70 reaches neither onMessageReceived nor onRefreshRecv, earns no NOTIFICATION, and the session stays Established.
// RFC requirement: RFC7313-5-3 negative -- a 4-octet ROUTE-REFRESH with Message Subtype 0, 1 or 2 through the same exchange is not ignored: it reaches onMessageReceived and onRefreshRecv once each.
func TestRouteRefreshUnknownSubtypeNeverDelivered(t *testing.T) {
	cases := []struct {
		name      string
		subtype   byte
		delivered int
	}{
		{"subtype_3", 3, 0},
		{"subtype_5", 5, 0},
		{"subtype_42", 42, 0},
		{"subtype_255_reserved", 0xFF, 0},
		{"subtype_0_normal", 0, 1},
		{"subtype_1_borr", 1, 1},
		{"subtype_2_eorr", 2, 1},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			session, client, cleanup := setupEstablishedSession(t)
			defer cleanup()

			var messages, refreshes int
			countRouteRefreshDelivery(session, &messages, &refreshes)

			raw, err := routeRefreshExchange(t, session, client, []byte{0x00, 0x01, tt.subtype, 0x01})

			require.NoError(t, err)
			assert.Empty(t, raw, "no NOTIFICATION, got %x", raw)
			assert.Equal(t, fsm.StateEstablished, session.State())
			assert.Equal(t, tt.delivered, messages, "onMessageReceived deliveries")
			assert.Equal(t, tt.delivered, refreshes, "onRefreshRecv calls")
		})
	}
}
