// RFC: rfc/short/rfc2918.md — the ROUTE-REFRESH Reserved octet
// Overview: session_handlers.go — handleRouteRefresh, the receive path under test
// Related: session_test.go — setupEstablishedSession, which negotiates RFC 7313 instead
//
// RFC 2918 Section 3 gives the third octet of a ROUTE-REFRESH body one sentence. It ends
// "ignored by the receiver", and that half is the requirement under test.
//
// RFC 7313 Section 3.2 redefines the same octet as the Message Subtype. RFC 7313 Sections
// 4 and 5 apply "only when a BGP speaker has received the 'Enhanced Route Refresh
// Capability' from a peer". So this test needs a session that never received capability
// 70. Every other ROUTE-REFRESH harness in this package negotiates it.

package reactor

import (
	"net"
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// setupEstablishedSessionRFC2918RouteRefresh drives a session to Established. The peer
// advertises the Route Refresh Capability (code 2). It does not advertise the Enhanced
// Route Refresh Capability (code 70).
//
// The absent capability is the whole point of the helper. With code 70 negotiated the
// third body octet is RFC 7313's Message Subtype, and "ignored by the receiver" no longer
// describes what the receiver owes.
func setupEstablishedSessionRFC2918RouteRefresh(t *testing.T) (*Session, net.Conn, func()) {
	t.Helper()

	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.Connection = ConnectionPassive
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.RouteRefresh{},
	}

	session := NewSession(settings)
	require.NoError(t, session.Start())

	client, server := net.Pipe()
	cleanup := func() {
		session.timers.StopAll()
		session.stopSendHoldTimer()
		client.Close() //nolint:errcheck // test cleanup
		server.Close() //nolint:errcheck // test cleanup
	}

	_ = acceptWithReader(t, session, server, client)

	// Peer OPEN: ASN4, IPv4 unicast, Route Refresh. No capability 70.
	peerOpen := &message.Open{
		Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x01020302,
		OptionalParams: []byte{
			2, 14, // Capability optional parameter, 14 octets of capabilities
			65, 4, 0, 0, 0xFD, 0xEA, // ASN4 = 65002
			1, 4, 0, 1, 0, 1, // Multiprotocol IPv4/Unicast
			2, 0, // Route Refresh (code 2, length 0)
		},
	}
	openBytes := message.PackTo(peerOpen, nil)

	go func() {
		client.Write(openBytes) //nolint:errcheck // test goroutine
		buf := make([]byte, 4096)
		client.Read(buf) //nolint:errcheck // drain the KEEPALIVE ze sends
	}()

	require.NoError(t, session.ReadAndProcess())
	require.Equal(t, fsm.StateOpenConfirm, session.State())

	go func() {
		client.Write(message.PackTo(message.NewKeepalive(), nil)) //nolint:errcheck // test goroutine
	}()

	require.NoError(t, session.ReadAndProcess())
	require.Equal(t, fsm.StateEstablished, session.State())
	require.False(t, session.negotiated.EnhancedRouteRefresh,
		"the fixture is only meaningful while RFC 7313 is NOT negotiated")

	return session, client, cleanup
}

// TestRFC2918ReservedOctetIgnoredOnReceive drives the same ROUTE-REFRESH four times,
// changing only the Reserved octet, and requires the receiver to reach the same outcome
// every time.
//
// VALIDATES: a non-zero Reserved octet neither refuses the message nor withholds it from
// the consumers that perform the re-advertisement RFC 2918 Section 4 mandates.
// PREVENTS: a receiver that reads the octet on a session where RFC 7313 was never
// negotiated, which would drop a conforming peer's refresh request on the floor.
//
// The values span what a reader of RFC 7313 treats as meaningful. 0x01 and 0x02 are its
// BoRR and EoRR subtypes. 0xAA is unassigned and 0xFF is reserved. Under RFC 2918 alone
// none of them means anything, so all four must land where a zero octet lands. The zero
// case runs first, as the baseline the rest are held to.
//
// RFC requirement: RFC2918-3-4 positive -- the Reserved octet is "ignored by the
// receiver". handleRouteRefresh (session_handlers.go) returns before it reads the octet
// when negotiated.EnhancedRouteRefresh is false. The ROUTE-REFRESH therefore reaches
// onMessageReceived and onRefreshRecv, no NOTIFICATION is written, and the session stays
// Established.
func TestRFC2918ReservedOctetIgnoredOnReceive(t *testing.T) {
	reserved := []struct {
		name  string
		octet byte
	}{
		{"zero_baseline", 0x00},
		{"rfc7313_borr_value", 0x01},
		{"rfc7313_eorr_value", 0x02},
		{"unassigned", 0xAA},
		{"rfc7313_reserved_value", 0xFF},
	}

	for _, tt := range reserved {
		t.Run(tt.name, func(t *testing.T) {
			session, client, cleanup := setupEstablishedSessionRFC2918RouteRefresh(t)
			defer cleanup()

			var messageCallbacks int
			var refreshCallbacks int
			session.onMessageReceived = func(_ netip.Addr, msgType msgtype.MessageType, raw []byte, _ *wireu.WireUpdate, _ bgpctx.ContextID, _ rpc.MessageDirection, _ BufHandle, _ map[string]any, _ string) bool {
				if msgType == msgtype.TypeROUTEREFRESH {
					messageCallbacks++
					assert.Equal(t, []byte{0x00, 0x01, tt.octet, 0x01}, raw,
						"the body must reach the consumer byte-identical, Reserved octet included")
				}
				return false
			}
			session.onRefreshRecv = func() {
				refreshCallbacks++
			}

			// AFI = 1 (IPv4), Reserved = tt.octet, SAFI = 1 (Unicast).
			rrMsg := buildRouteRefreshMsg([]byte{0x00, 0x01, tt.octet, 0x01})
			go func() {
				client.Write(rrMsg) //nolint:errcheck // test goroutine
			}()

			require.NoError(t, session.ReadAndProcess(),
				"a ROUTE-REFRESH whose only oddity is its Reserved octet must not error")
			assert.Equal(t, 1, messageCallbacks,
				"the refresh must reach the consumer that re-advertises the Adj-RIB-Out")
			assert.Equal(t, 1, refreshCallbacks)
			assert.Equal(t, fsm.StateEstablished, session.State())

			// Nothing comes back. A read deadline turns "no NOTIFICATION" from an
			// assumption into an assertion. net.Pipe is unbuffered, so a written
			// NOTIFICATION would be read here instead.
			require.NoError(t, client.SetReadDeadline(time.Now().Add(200*time.Millisecond)))
			buf := make([]byte, 4096)
			n, readErr := client.Read(buf)
			require.ErrorIs(t, readErr, os.ErrDeadlineExceeded,
				"the Reserved octet is ignored, so it can raise no error. Got %d byte(s): %x",
				n, buf[:max(n, 0)])
		})
	}
}

// TestRFC2918ReservedOctetDoesNotExemptTheMessage pins the scope of "ignored".
//
// VALIDATES: the receiver ignores the Reserved octet's VALUE, not the message that carries
// it. RFC 2918 Section 3 fixes the body at 4 octets. A body of another length is still
// refused, and still withheld from the receive callbacks, when the octet is non-zero.
// PREVENTS: reading "ignored by the receiver" as "a non-zero Reserved octet makes the rest
// of the message unexaminable", which would let a malformed ROUTE-REFRESH through by
// setting one octet. Without this the positive above would pass on a receiver that
// accepted every ROUTE-REFRESH unconditionally.
//
// The NOTIFICATION code is deliberately not asserted. RFC 7313 Section 5 scopes its
// ROUTE-REFRESH Message Error to a session that received capability 70, and this session
// did not. What RFC 2918 owes here is refusal, and refusal is what is asserted.
//
// RFC requirement: RFC2918-3-4 negative -- the indifference is to the octet alone.
// validateRouteRefreshLength (session_handlers.go) runs before the octet is read. A
// 5-octet body carrying Reserved = 0xAA is refused with ErrInvalidMessage. It reaches
// neither onMessageReceived nor onRefreshRecv.
func TestRFC2918ReservedOctetDoesNotExemptTheMessage(t *testing.T) {
	session, client, cleanup := setupEstablishedSessionRFC2918RouteRefresh(t)
	defer cleanup()

	var messageCallbacks int
	var refreshCallbacks int
	session.onMessageReceived = func(_ netip.Addr, msgType msgtype.MessageType, _ []byte, _ *wireu.WireUpdate, _ bgpctx.ContextID, _ rpc.MessageDirection, _ BufHandle, _ map[string]any, _ string) bool {
		if msgType == msgtype.TypeROUTEREFRESH {
			messageCallbacks++
		}
		return false
	}
	session.onRefreshRecv = func() {
		refreshCallbacks++
	}

	// Five octets where RFC 2918 Section 3 defines four, with a non-zero Reserved octet.
	rrMsg := buildRouteRefreshMsg([]byte{0x00, 0x01, 0xAA, 0x01, 0xAA})
	go func() {
		client.Write(rrMsg) //nolint:errcheck // test goroutine
		buf := make([]byte, 4096)
		client.Read(buf) //nolint:errcheck // drain whatever ze answers with
	}()

	err := session.ReadAndProcess()
	require.Error(t, err, "a ROUTE-REFRESH body of the wrong length must be refused")
	assert.ErrorIs(t, err, ErrInvalidMessage)
	assert.Equal(t, 0, messageCallbacks,
		"a refused ROUTE-REFRESH must not reach the receive callbacks")
	assert.Equal(t, 0, refreshCallbacks)
}
