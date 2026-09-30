package reactor

import (
	"encoding/binary"
	"net"
	"net/netip"
	"sync"
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

// dispatchCapture records the UPDATE payloads a session hands to its consumers
// (the RIB plugins and the forward rails), which is everything the rest of Ze
// learns about a received route.
type dispatchCapture struct {
	mu       sync.Mutex
	payloads [][]byte
}

func (c *dispatchCapture) all() [][]byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([][]byte(nil), c.payloads...)
}

// setupCapturingSession brings a passive session to Established against a peer
// in AS peerAS (local AS 65001, so peerAS 65001 is internal and any other value
// external), with the RFC 8654 Extended Message capability on both sides when
// extended is set, and records every received UPDATE payload it dispatches.
func setupCapturingSession(t *testing.T, peerAS uint32, extended bool) (*Session, net.Conn, *dispatchCapture, func()) {
	t.Helper()

	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, peerAS, 0x01020301)
	settings.Connection = ConnectionPassive
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
	}
	peerCaps := []byte{
		65, 4, byte(peerAS >> 24), byte(peerAS >> 16), byte(peerAS >> 8), byte(peerAS),
		1, 4, 0, 1, 0, 1,
	}
	if extended {
		settings.Capabilities = append(settings.Capabilities, &capability.ExtendedMessage{})
		peerCaps = append(peerCaps, 6, 0)
	}

	session := NewSession(settings)
	capture := &dispatchCapture{}
	session.onMessageReceived = func(_ netip.Addr, _ msgtype.MessageType, _ []byte,
		wu *wireu.WireUpdate, _ bgpctx.ContextID, direction rpc.MessageDirection,
		_ BufHandle, _ map[string]any, _ string, _ uint64) bool {
		if direction == rpc.DirectionReceived && wu != nil {
			capture.mu.Lock()
			capture.payloads = append(capture.payloads, append([]byte(nil), wu.Payload()...))
			capture.mu.Unlock()
		}
		return false
	}
	require.NoError(t, session.Start())

	client, server := net.Pipe()
	cleanup := func() {
		session.timers.StopAll()
		session.stopSendHoldTimer()
		client.Close() //nolint:errcheck // test cleanup
		server.Close() //nolint:errcheck // test cleanup
	}
	_ = acceptWithReader(t, session, server, client)

	params := append([]byte{2, byte(len(peerCaps))}, peerCaps...)
	peerOpen := &message.Open{
		Version: 4, MyAS: uint16(peerAS), HoldTime: 90, BGPIdentifier: 0x01020302,
		OptionalParams: params,
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
	require.Equal(t, extended, session.extendedMessage, "the fixture's Extended Message state")

	return session, client, capture, cleanup
}

// ipv4Slash24s returns count distinct /24 NLRI encodings (4 octets each).
func ipv4Slash24s(count int) []byte {
	nlri := make([]byte, 0, 4*count)
	for i := range count {
		nlri = append(nlri, 24, 10, byte(i>>8), byte(i))
	}
	return nlri
}

// receivedUpdateBody builds an UPDATE body with no withdrawn routes.
func receivedUpdateBody(attrs, nlri []byte) []byte {
	body := make([]byte, 0, 4+len(attrs)+len(nlri))
	body = append(body, 0, 0, byte(len(attrs)>>8), byte(len(attrs)))
	body = append(body, attrs...)
	return append(body, nlri...)
}

// payloadSections splits a dispatched UPDATE payload into its three fields.
func payloadSections(t *testing.T, payload []byte) (withdrawn, attrs, nlri []byte) {
	t.Helper()
	require.GreaterOrEqual(t, len(payload), 4)
	withdrawnLen := int(binary.BigEndian.Uint16(payload[0:2]))
	require.GreaterOrEqual(t, len(payload), 4+withdrawnLen)
	withdrawn = payload[2 : 2+withdrawnLen]
	attrLen := int(binary.BigEndian.Uint16(payload[2+withdrawnLen : 4+withdrawnLen]))
	require.GreaterOrEqual(t, len(payload), 4+withdrawnLen+attrLen)
	attrs = payload[4+withdrawnLen : 4+withdrawnLen+attrLen]
	return withdrawn, attrs, payload[4+withdrawnLen+attrLen:]
}

// readOnce reads whatever the session wrote next, or nil after the deadline.
func readOnce(client net.Conn, wait time.Duration) []byte {
	_ = client.SetReadDeadline(time.Now().Add(wait))
	buf := make([]byte, 4096)
	n, _ := client.Read(buf)
	return append([]byte(nil), buf[:max(n, 0)]...)
}

// TestRFC4271LocalPrefFromExternalPeerNeverReachesTheRIB drives an UPDATE
// carrying LOCAL_PREF 200 through a live session and asserts what the session
// hands to its consumers, which is the only copy the RIB and best-path see.
//
// VALIDATES: from an internal peer the LOCAL_PREF reaches the consumers intact
// (code 5, value 200); from an external peer the same attribute is gone from
// the dispatched payload (its place taken by ze's ATTR_TOMBSTONE marker, whose
// value holds neither the code-5 header nor the value 200), while ORIGIN,
// AS_PATH, NEXT_HOP and the NLRI arrive
// unchanged and the session stays Established with no NOTIFICATION.
// PREVENTS: a session that computes the attribute-discard verdict and then
// dispatches the original bytes, so best-path would rank on a LOCAL_PREF an
// external peer chose.
//
// RFC requirement: RFC4271-5.1.5-3 positive -- LOCAL_PREF received from an internal peer is
// dispatched to the RIB with its value.
// RFC requirement: RFC4271-5.1.5-3 negative -- LOCAL_PREF received from an external peer is
// absent from the UPDATE the session dispatches; the rest of the UPDATE and the session survive.
func TestRFC4271LocalPrefFromExternalPeerNeverReachesTheRIB(t *testing.T) {
	localPref := []byte{0x40, 0x05, 0x04, 0, 0, 0, 200}
	nlri := []byte{0x08, 0x0a}

	t.Run("internal", func(t *testing.T) {
		session, client, capture, cleanup := setupCapturingSession(t, 65001, false)
		defer cleanup()
		attrs := append([]byte{
			0x40, 0x01, 0x01, 0x00,
			0x40, 0x02, 0x00,
			0x40, 0x03, 0x04, 192, 0, 2, 254,
		}, localPref...)
		go sendUpdateAndDrain(client, buildUpdateMsg(receivedUpdateBody(attrs, nlri)))

		require.NoError(t, session.ReadAndProcess())
		require.Equal(t, fsm.StateEstablished, session.State())
		got := capture.all()
		require.Len(t, got, 1)
		_, gotAttrs, gotNLRI := payloadSections(t, got[0])
		assert.Equal(t, attrs, gotAttrs, "an internal peer's LOCAL_PREF is kept, value and all")
		assert.Equal(t, nlri, gotNLRI)
	})

	t.Run("external", func(t *testing.T) {
		session, client, capture, cleanup := setupCapturingSession(t, 65002, false)
		defer cleanup()
		kept := []byte{
			0x40, 0x01, 0x01, 0x00,
			0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, 0xFD, 0xEA,
			0x40, 0x03, 0x04, 192, 0, 2, 254,
		}
		attrs := append(append([]byte(nil), kept...), localPref...)
		answer := make(chan []byte, 1)
		go func() {
			_, _ = client.Write(buildUpdateMsg(receivedUpdateBody(attrs, nlri)))
			answer <- readOnce(client, 200*time.Millisecond)
		}()

		require.NoError(t, session.ReadAndProcess())
		require.Equal(t, fsm.StateEstablished, session.State())
		got := capture.all()
		require.Len(t, got, 1, "the UPDATE is still processed")
		_, gotAttrs, gotNLRI := payloadSections(t, got[0])
		assert.NotContains(t, attrCodes(t, gotAttrs), 5, "LOCAL_PREF from an external peer is ignored")
		// The discarded attribute's place is taken by ze's ATTR_TOMBSTONE marker
		// (252, message/attr_discard.go), which carries no LOCAL_PREF value.
		assert.Equal(t, []int{1, 2, 3, 252}, attrCodes(t, gotAttrs))
		assert.Equal(t, kept, gotAttrs[:len(kept)], "every other attribute arrives unchanged")
		assert.Equal(t, nlri, gotNLRI)
		assert.Empty(t, <-answer, "ignoring the attribute sends no NOTIFICATION")
	})
}

// TestRFC8654ExtendedMessageSessionUsesRFC7606ErrorHandling sends UPDATEs
// longer than 4096 octets on a session that negotiated Extended Message and
// asserts RFC 7606 decides the outcome.
//
// VALIDATES: an extended UPDATE of 1100 /24 routes whose ORIGIN is malformed is
// treat-as-withdraw: no error, session Established, no NOTIFICATION, and the
// consumers receive every one of the 1100 routes as withdrawn with no
// attributes. An extended UPDATE whose declared attribute section runs past the
// message end is the RFC 7606 Section 3 session-reset case: NOTIFICATION UPDATE
// Message Error / Malformed Attribute List, session Idle, nothing dispatched.
// PREVENTS: the extended-message receive path bypassing RFC 7606, so a
// malformed attribute in a large UPDATE resets the session, or a malformed
// UPDATE is installed.
//
// RFC requirement: RFC8654-3-1 positive -- on a session that negotiated Extended Message, a
// malformed attribute in an UPDATE over 4096 octets is treat-as-withdraw and the session stays up.
// RFC requirement: RFC8654-3-1 negative -- on the same session, an over-4096-octet UPDATE whose
// attribute section overruns the message is refused with NOTIFICATION 3/1 and nothing is dispatched.
func TestRFC8654ExtendedMessageSessionUsesRFC7606ErrorHandling(t *testing.T) {
	nlri := ipv4Slash24s(1100)
	aspath := []byte{0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, 0xFD, 0xEA}
	nexthop := []byte{0x40, 0x03, 0x04, 192, 0, 2, 254}

	t.Run("treat-as-withdraw", func(t *testing.T) {
		session, client, capture, cleanup := setupCapturingSession(t, 65002, true)
		defer cleanup()
		attrs := append(append([]byte{0x40, 0x01, 0x02, 0x00, 0x00}, aspath...), nexthop...)
		msg := buildUpdateMsg(receivedUpdateBody(attrs, nlri))
		require.Greater(t, len(msg), 4096, "the UPDATE must need Extended Message")
		answer := make(chan []byte, 1)
		go func() {
			_, _ = client.Write(msg)
			answer <- readOnce(client, 200*time.Millisecond)
		}()

		require.NoError(t, session.ReadAndProcess(), "treat-as-withdraw does not end the read")
		require.Equal(t, fsm.StateEstablished, session.State(), "the session stays up")
		got := capture.all()
		require.Len(t, got, 1)
		withdrawn, gotAttrs, gotNLRI := payloadSections(t, got[0])
		assert.Equal(t, nlri, withdrawn, "every route of the UPDATE is withdrawn")
		assert.Empty(t, gotAttrs)
		assert.Empty(t, gotNLRI, "nothing stays announced")
		assert.Empty(t, <-answer, "treat-as-withdraw sends no NOTIFICATION")
	})

	t.Run("session-reset", func(t *testing.T) {
		session, client, capture, cleanup := setupCapturingSession(t, 65002, true)
		defer cleanup()
		attrs := append(append([]byte{0x40, 0x01, 0x01, 0x00}, aspath...), nexthop...)
		body := receivedUpdateBody(attrs, nlri)
		declared := len(attrs) + len(nlri) + 1
		body[2], body[3] = byte(declared>>8), byte(declared)
		msg := buildUpdateMsg(body)
		require.Greater(t, len(msg), 4096)
		answer := make(chan []byte, 1)
		go func() {
			_, _ = client.Write(msg)
			answer <- readOnce(client, 2*time.Second)
		}()

		require.Error(t, session.ReadAndProcess())
		require.Equal(t, fsm.StateIdle, session.State())
		assert.Empty(t, capture.all(), "an UPDATE whose sections cannot be trusted is not dispatched")
		assertNotification(t, <-answer, message.NotifyUpdateMessage, message.NotifyUpdateMalformedAttr, []byte{})
	})
}
