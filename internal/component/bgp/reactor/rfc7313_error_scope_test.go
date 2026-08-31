// RFC: rfc/short/rfc7313.md — the scope of the ROUTE-REFRESH Message Error
// RFC: rfc/short/rfc4271.md — Message Header Error, Bad Message Length
// Overview: session_handlers.go — validateRouteRefreshLength, the decision under test
// Related: rfc2918_reserved_field_test.go — the fixture whose peer never sent capability 70
//
// RFC 7313 Section 5 scopes the error code it invents twice. It is "applicable only
// when a BGP speaker has received the 'Enhanced Route Refresh Capability' from a
// peer", and its length MUST names "Message Subtype 1 and 2". Outside both scopes a
// malformed ROUTE-REFRESH is a length error like any other, and RFC 2918 defines no
// error handling of its own (Sections 3 and 4), so RFC 4271 Section 6.1 answers.
//
// These tests drive the whole receive path over net.Pipe, so what they read is the
// octets ze puts on the wire rather than a return value.

package reactor

import (
	"net"
	"net/netip"
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

// routeRefreshSilenceWait is how long the peer waits for an answer before it calls
// the answer absent. net.Pipe is unbuffered and synchronous, so a NOTIFICATION ze
// writes is delivered to the waiting read at once; the wait only bounds the case
// where ze correctly writes nothing.
const routeRefreshSilenceWait = 250 * time.Millisecond

// routeRefreshExchange sends one ROUTE-REFRESH with the given body and returns the
// bytes ze answers with, empty when ze answers nothing, together with the error the
// receive path reported.
func routeRefreshExchange(t *testing.T, session *Session, client net.Conn, body []byte) ([]byte, error) {
	t.Helper()

	answer := make(chan []byte, 1)
	go func() {
		_, _ = client.Write(buildRouteRefreshMsg(body))
		if deadlineErr := client.SetReadDeadline(time.Now().Add(routeRefreshSilenceWait)); deadlineErr != nil {
			answer <- nil
			return
		}
		buf := make([]byte, 4096)
		n, _ := client.Read(buf)
		answer <- append([]byte(nil), buf[:max(n, 0)]...)
	}()

	err := session.ReadAndProcess()

	select {
	case raw := <-answer:
		return raw, err
	case <-time.After(5 * time.Second):
		t.Fatal("the peer goroutine never returned")
		return nil, err
	}
}

// assertNotification requires that raw is a NOTIFICATION carrying exactly this Error
// Code, this Error Subcode and this Data field. The Data field is asserted because
// both RFCs make it an obligation of its own, and each names a different value.
func assertNotification(t *testing.T, raw []byte, code message.NotifyErrorCode, subcode uint8, data []byte) {
	t.Helper()

	require.GreaterOrEqual(t, len(raw), message.HeaderLen+2,
		"expected a NOTIFICATION, got %d octet(s): %x", len(raw), raw)
	require.Equal(t, byte(msgtype.TypeNOTIFICATION), raw[18], "message type")
	assert.Equal(t, byte(code), raw[message.HeaderLen], "NOTIFICATION Error Code")
	assert.Equal(t, subcode, raw[message.HeaderLen+1], "NOTIFICATION Error Subcode")
	assert.Equal(t, data, raw[message.HeaderLen+2:], "NOTIFICATION Data field")
}

// countRouteRefreshDelivery makes the session count what reaches its two receive
// callbacks, so a refused or ignored message can be shown never to reach a plugin.
func countRouteRefreshDelivery(session *Session, messages, refreshes *int) {
	session.onMessageReceived = func(_ netip.Addr, msgType msgtype.MessageType, _ []byte, _ *wireu.WireUpdate, _ bgpctx.ContextID, _ rpc.MessageDirection, _ BufHandle, _ map[string]any, _ string) bool {
		if msgType == msgtype.TypeROUTEREFRESH {
			*messages++
		}
		return false
	}
	session.onRefreshRecv = func() {
		*refreshes++
	}
}

// headerLengthField renders the two octets RFC 4271 Section 6.1 requires in the Data
// field of a Bad Message Length NOTIFICATION: the erroneous Length field, which
// counts the 19-octet header.
func headerLengthField(bodyLen int) []byte {
	total := message.HeaderLen + bodyLen
	return []byte{byte(total >> 8), byte(total)}
}

// TestRouteRefreshBadLengthWithoutCapability70 drives a malformed ROUTE-REFRESH into a
// session whose peer advertised RFC 2918's capability 2 and never advertised RFC 7313's
// capability 70.
//
// VALIDATES: such a peer is answered under RFC 4271 Section 6.1, with Message Header
// Error (1) and Bad Message Length (2), carrying the erroneous Length field.
// PREVENTS: sending Error Code 7, which RFC 7313 invented, to a peer that never spoke
// RFC 7313 and has no assigned meaning for it. The third body octet is varied across
// every value a reader of RFC 7313 treats as meaningful, because without capability 70
// that octet is RFC 2918's Reserved field and can change nothing.
//
// RFC requirement: RFC7313-5-1 negative -- the ROUTE-REFRESH Message Error is NOT sent
// here. RFC 7313 Section 5 makes its error handling "applicable only when a BGP speaker
// has received the 'Enhanced Route Refresh Capability' from a peer", and
// validateRouteRefreshLength (session_handlers.go) reads what the peer advertised before
// it reads anything in the message.
// The answer is RFC 4271 Section 6.1 Bad Message Length. This test carries NO
// RFC4271-6.1-3 tag: that requirement is about an erroneous header Length field, and
// buildRouteRefreshMsg always writes a self-consistent header Length of 19+len(body), so
// no case here reaches it. Its {gap} in rfc/short/rfc4271.md is genuine.
func TestRouteRefreshBadLengthWithoutCapability70(t *testing.T) {
	bodies := []struct {
		name string
		body []byte
	}{
		{"empty", []byte{}},
		{"one_octet", []byte{0x00}},
		{"two_octets_no_subtype_field", []byte{0x00, 0x01}},
		{"three_octets_reserved_zero", []byte{0x00, 0x01, 0x00}},
		{"three_octets_reserved_borr_value", []byte{0x00, 0x01, 0x01}},
		{"five_octets_reserved_zero", []byte{0x00, 0x01, 0x00, 0x01, 0xFF}},
		{"five_octets_reserved_borr_value", []byte{0x00, 0x01, 0x01, 0x01, 0xFF}},
		{"five_octets_reserved_eorr_value", []byte{0x00, 0x01, 0x02, 0x01, 0xFF}},
		{"five_octets_reserved_unassigned", []byte{0x00, 0x01, 0x05, 0x01, 0xFF}},
	}

	for _, tt := range bodies {
		t.Run(tt.name, func(t *testing.T) {
			session, client, cleanup := setupEstablishedSessionRFC2918RouteRefresh(t)
			defer cleanup()

			require.False(t, session.negotiated.PeerAdvertised(capability.CodeEnhancedRouteRefresh),
				"the fixture is only meaningful while the peer sent no capability 70")

			var messages, refreshes int
			countRouteRefreshDelivery(session, &messages, &refreshes)

			raw, err := routeRefreshExchange(t, session, client, tt.body)

			require.Error(t, err, "a ROUTE-REFRESH body of the wrong length must be refused")
			assert.ErrorIs(t, err, ErrInvalidMessage)
			assertNotification(t, raw, message.NotifyMessageHeader, message.NotifyHeaderBadLength,
				headerLengthField(len(tt.body)))
			assert.Equal(t, 0, messages, "a refused ROUTE-REFRESH must not reach onMessageReceived")
			assert.Equal(t, 0, refreshes, "a refused ROUTE-REFRESH must not reach onRefreshRecv")
		})
	}
}

// TestRouteRefreshWellFormedWithoutCapability70DrawsNoNotification is the control for
// the test above.
//
// VALIDATES: the Bad Message Length answer is scoped to a body of the wrong length.
// PREVENTS: a receiver that refuses every ROUTE-REFRESH from an RFC 2918 peer, which
// would make the negative above pass for the wrong reason.
//
// RFC requirement: RFC7313-5-1 negative -- the length answer does not over-fire: a
// ROUTE-REFRESH whose body IS 4 octets raises nothing at all, no error is returned, and
// the session stays Established.
func TestRouteRefreshWellFormedWithoutCapability70DrawsNoNotification(t *testing.T) {
	session, client, cleanup := setupEstablishedSessionRFC2918RouteRefresh(t)
	defer cleanup()

	var messages, refreshes int
	countRouteRefreshDelivery(session, &messages, &refreshes)

	// AFI = 1 (IPv4), Reserved = 0, SAFI = 1 (Unicast).
	raw, err := routeRefreshExchange(t, session, client, []byte{0x00, 0x01, 0x00, 0x01})

	require.NoError(t, err)
	assert.Empty(t, raw, "a well-formed ROUTE-REFRESH earns no answer, got %x", raw)
	assert.Equal(t, fsm.StateEstablished, session.State())
	assert.Equal(t, 1, messages)
	assert.Equal(t, 1, refreshes)
}

// TestRouteRefreshBadLengthByMessageSubtype drives a malformed ROUTE-REFRESH into a
// session whose peer DID advertise capability 70, once for each Message Subtype RFC 7313
// Section 3.2 assigns.
//
// VALIDATES: the length MUST of RFC 7313 Section 5 covers Subtype 1 and 2 and stops
// there. A malformed BoRR or EoRR draws ROUTE-REFRESH Message Error (7) / Invalid
// Message Length (1) carrying the complete message; a malformed Subtype 0 request, which
// RFC 7313 left with RFC 2918, draws RFC 4271's Bad Message Length instead.
// PREVENTS: reading "the received ROUTE-REFRESH message" in that sentence as every
// ROUTE-REFRESH, which sends an error code RFC 7313 defined for BoRR and EoRR in answer
// to a plain RFC 2918 refresh request.
//
// RFC requirement: RFC7313-5-1 positive -- Subtype 1 and 2 with a body that is not 4
// octets draw Error Code 7, subcode 1.
// RFC requirement: RFC7313-5-2 positive -- that NOTIFICATION's Data field carries the
// complete ROUTE-REFRESH message, header included.
// RFC requirement: RFC7313-5-1 negative -- Subtype 0 is outside "Message Subtype 1 and
// 2", so it draws RFC 4271 Section 6.1 Bad Message Length rather than Error Code 7.
func TestRouteRefreshBadLengthByMessageSubtype(t *testing.T) {
	cases := []struct {
		name    string
		body    []byte
		code    message.NotifyErrorCode
		subcode uint8
	}{
		{"subtype_0_too_short", []byte{0x00, 0x01, 0x00}, message.NotifyMessageHeader, message.NotifyHeaderBadLength},
		{"subtype_0_too_long", []byte{0x00, 0x01, 0x00, 0x01, 0xFF}, message.NotifyMessageHeader, message.NotifyHeaderBadLength},
		{"subtype_1_borr_too_short", []byte{0x00, 0x01, 0x01}, message.NotifyRouteRefresh, message.NotifyRouteRefreshInvalidLength},
		{"subtype_1_borr_too_long", []byte{0x00, 0x01, 0x01, 0x01, 0xFF}, message.NotifyRouteRefresh, message.NotifyRouteRefreshInvalidLength},
		{"subtype_2_eorr_too_short", []byte{0x00, 0x01, 0x02}, message.NotifyRouteRefresh, message.NotifyRouteRefreshInvalidLength},
		{"subtype_2_eorr_too_long", []byte{0x00, 0x01, 0x02, 0x01, 0xFF}, message.NotifyRouteRefresh, message.NotifyRouteRefreshInvalidLength},
		{"no_subtype_field", []byte{0x00, 0x01}, message.NotifyMessageHeader, message.NotifyHeaderBadLength},
		{"empty_body", []byte{}, message.NotifyMessageHeader, message.NotifyHeaderBadLength},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			session, client, cleanup := setupEstablishedSession(t)
			defer cleanup()

			require.True(t, session.negotiated.PeerAdvertised(capability.CodeEnhancedRouteRefresh),
				"the fixture is only meaningful while the peer sent capability 70")

			var messages, refreshes int
			countRouteRefreshDelivery(session, &messages, &refreshes)

			raw, err := routeRefreshExchange(t, session, client, tt.body)

			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidMessage)

			data := headerLengthField(len(tt.body))
			if tt.code == message.NotifyRouteRefresh {
				data = buildRouteRefreshMsg(tt.body)
			}
			assertNotification(t, raw, tt.code, tt.subcode, data)
			assert.Equal(t, 0, messages, "a refused ROUTE-REFRESH must not reach onMessageReceived")
			assert.Equal(t, 0, refreshes, "a refused ROUTE-REFRESH must not reach onRefreshRecv")
		})
	}
}

// TestRouteRefreshUnknownSubtypeIsIgnoredWhateverItsLength drives a ROUTE-REFRESH whose
// Message Subtype is outside 0, 1 and 2 into a session whose peer advertised capability
// 70, at three body lengths.
//
// VALIDATES: "MUST ignore" wins over the length rule. The ignore sentence carries no
// length condition, while the length sentence is scoped to Subtype 1 and 2, so an
// unknown subtype earns no NOTIFICATION at any length and the session survives.
// PREVENTS: a length check placed ahead of the subtype read, which would answer an
// unassigned subtype with Error Code 7 and tear the session down. The malformed cases
// are the load-bearing ones: a 4-octet body reaches the same verdict through the
// subtype branch of handleRouteRefresh, so it alone would not show the ordering.
//
// RFC requirement: RFC7313-5-3 positive -- a Message Subtype other than 0, 1 or 2 is
// ignored: nothing is written back and the session stays Established.
// RFC requirement: RFC7313-5-1 negative -- the Invalid Message Length answer does not
// reach a subtype its sentence does not name, even when the body length is wrong.
func TestRouteRefreshUnknownSubtypeIsIgnoredWhateverItsLength(t *testing.T) {
	cases := []struct {
		name      string
		body      []byte
		delivered int
	}{
		// A 4-octet body is well formed, so it is delivered and then ignored by subtype.
		{"unassigned_subtype_well_formed", []byte{0x00, 0x01, 0x05, 0x01}, 1},
		{"reserved_subtype_255_well_formed", []byte{0x00, 0x01, 0xFF, 0x01}, 1},
		{"unassigned_subtype_too_short", []byte{0x00, 0x01, 0x05}, 0},
		{"unassigned_subtype_too_long", []byte{0x00, 0x01, 0x05, 0x01, 0xFF}, 0},
		{"reserved_subtype_255_too_long", []byte{0x00, 0x01, 0xFF, 0x01, 0xFF}, 0},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			session, client, cleanup := setupEstablishedSession(t)
			defer cleanup()

			var messages, refreshes int
			countRouteRefreshDelivery(session, &messages, &refreshes)

			raw, err := routeRefreshExchange(t, session, client, tt.body)

			require.NoError(t, err, "an ignored ROUTE-REFRESH must not end the session")
			assert.Empty(t, raw, "an ignored ROUTE-REFRESH earns no NOTIFICATION, got %x", raw)
			assert.Equal(t, fsm.StateEstablished, session.State())
			assert.Equal(t, tt.delivered, messages)
			assert.Equal(t, tt.delivered, refreshes)
		})
	}
}
