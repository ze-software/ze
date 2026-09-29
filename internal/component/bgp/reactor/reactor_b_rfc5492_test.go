package reactor

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
)

// newTwoRequiredOpenSession returns an OpenSent session that advertises and
// requires both Route Refresh (code 2) and Extended Message (code 6), with the
// recording connection and the octet count its own OPEN took.
func newTwoRequiredOpenSession(t *testing.T) (*Session, *recordingConn, int) {
	t.Helper()
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.Connection = ConnectionPassive
	settings.ReceiveHoldTime = 90 * time.Second
	settings.Capabilities = []capability.Capability{&capability.RouteRefresh{}, &capability.ExtendedMessage{}}
	settings.RequiredCapabilities = []capability.Code{capability.CodeRouteRefresh, capability.CodeExtendedMessage}
	session := NewSession(settings)
	require.NoError(t, session.Start())
	conn := &recordingConn{}
	require.NoError(t, session.Accept(conn))
	require.Equal(t, fsm.StateOpenSent, session.State())
	t.Cleanup(func() {
		require.NoError(t, session.Stop())
		session.closeConn()
	})
	return session, conn, len(conn.written())
}

// openBodyWithCapabilities returns the peer's OPEN body carrying the given
// capability TLV octets in one Capabilities optional parameter, or no optional
// parameter when caps is empty.
func openBodyWithCapabilities(caps []byte) []byte {
	body := validOpenBody()
	if len(caps) == 0 {
		return body
	}
	body[9] = byte(2 + len(caps))
	body = append(body, 2, byte(len(caps)))
	return append(body, caps...)
}

// Goal: prove the Unsupported Capability NOTIFICATION ze sends lists exactly
// the capabilities that caused it: every required capability the peer did not
// send, and none it did send or that ze does not know.
// Method: an OpenSent session that requires Route Refresh and Extended Message
// receives three peer OPENs through handleOpen; the octets written to the
// connection after ze's own OPEN are read back.
//
// VALIDATES: the Data field of the sent NOTIFICATION, on the wire.
// PREVENTS: a cause missing from the Data, or a capability listed that caused nothing.
//
// RFC requirement: RFC5492-3-1 positive -- a peer OPEN with neither required capability draws one NOTIFICATION 2/7 whose Data is exactly Route Refresh then Extended Message, each code with length 0: both causes are in the message.
// RFC requirement: RFC5492-3-1 negative -- a peer OPEN carrying Route Refresh and the unknown code 254 but not Extended Message draws a NOTIFICATION 2/7 whose Data is exactly Extended Message with length 0: the Route Refresh the peer sent and the unknown 254 are not listed; a peer OPEN carrying both writes no NOTIFICATION.
// RFC requirement: RFC5492-5-1 positive -- with two required capabilities missing, the Data of the sent NOTIFICATION lists the set of both, Route Refresh and Extended Message, in four octets.
// RFC requirement: RFC5492-5-1 negative -- with one of the two missing, the Data lists only that one (two octets), so the set is the causing set and not the required set.
func TestRFC5492UnsupportedCapabilityDataListsTheCausingSet(t *testing.T) {
	notifyData := func(written []byte) []byte {
		require.Len(t, splitMessages(t, written), 1, "exactly one message: the NOTIFICATION")
		require.GreaterOrEqual(t, len(written), message.HeaderLen+2)
		require.Equal(t, byte(msgtype.TypeNOTIFICATION), written[18])
		require.Equal(t, byte(message.NotifyOpenMessage), written[message.HeaderLen])
		require.Equal(t, message.NotifyOpenUnsupportedCapability, written[message.HeaderLen+1])
		return written[message.HeaderLen+2:]
	}

	s, conn, sent := newTwoRequiredOpenSession(t)
	require.ErrorIs(t, s.handleOpen(openBodyWithCapabilities(nil)), ErrInvalidState)
	require.Equal(t, fsm.StateIdle, s.State())
	require.Equal(t, []byte{2, 0, 6, 0}, notifyData(conn.written()[sent:]))

	s, conn, sent = newTwoRequiredOpenSession(t)
	require.ErrorIs(t, s.handleOpen(openBodyWithCapabilities([]byte{2, 0, 254, 2, 0xab, 0xcd})), ErrInvalidState)
	require.Equal(t, fsm.StateIdle, s.State())
	require.Equal(t, []byte{6, 0}, notifyData(conn.written()[sent:]))

	s, conn, sent = newTwoRequiredOpenSession(t)
	require.NoError(t, s.handleOpen(openBodyWithCapabilities([]byte{2, 0, 6, 0})))
	require.Equal(t, fsm.StateOpenConfirm, s.State())
	for _, b := range splitMessages(t, conn.written()[sent:]) {
		require.NotEqual(t, byte(msgtype.TypeNOTIFICATION), b[18], "both required capabilities present: no NOTIFICATION")
	}
}

// splitMessages cuts a byte stream of whole BGP messages at their length fields.
func splitMessages(t *testing.T, stream []byte) [][]byte {
	t.Helper()
	var messages [][]byte
	for len(stream) > 0 {
		require.GreaterOrEqual(t, len(stream), message.HeaderLen)
		length := int(stream[16])<<8 | int(stream[17])
		require.GreaterOrEqual(t, len(stream), length)
		messages = append(messages, stream[:length])
		stream = stream[length:]
	}
	return messages
}
