package reactor

import (
	"encoding/binary"
	"io"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
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
// external), with independently selected RFC 8654 advertisements, and records
// every received UPDATE payload it dispatches.
func setupCapturingSession(t *testing.T, peerAS uint32, localExtended, peerExtended bool) (*Session, net.Conn, *dispatchCapture, func()) {
	t.Helper()
	// Earlier reactor fixtures can leave the global read budget auto-sized for
	// very small peers. These wire-format tests are not pool-exhaustion tests.
	bufMuxGlobalMu.Lock()
	if budget := bufMuxStd.mux.budget; budget != nil {
		previous := budget.maxBytes.Load()
		updateBufMuxBudget(0)
		t.Cleanup(func() {
			bufMuxGlobalMu.Lock()
			updateBufMuxBudget(previous)
			bufMuxGlobalMu.Unlock()
		})
	}
	bufMuxGlobalMu.Unlock()

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
	if localExtended {
		settings.Capabilities = append(settings.Capabilities, &capability.ExtendedMessage{})
	}
	if peerExtended {
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
	require.Equal(t, localExtended, session.extendedMessage, "the fixture's advertised receive permission")

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
		session, client, capture, cleanup := setupCapturingSession(t, 65001, false, false)
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
		session, client, capture, cleanup := setupCapturingSession(t, 65002, false, false)
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
		session, client, capture, cleanup := setupCapturingSession(t, 65002, true, true)
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
		session, client, capture, cleanup := setupCapturingSession(t, 65002, true, true)
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

// TestRFC8654ExtendedAttributeDiscard isolates each RFC 7606 discard condition
// in an otherwise valid extended UPDATE and compares its valid counterpart.
// MUTATION: Skip ApplyAttrDiscard in enforceRFC7606: the malformed attribute
// reaches the captured consumer and fails the explicit absence assertion.
// RFC requirement: RFC8654-3-1 positive -- valid ATOMIC_AGGREGATE and four-octet AGGREGATOR survive extended UPDATE reception with all routes and other attributes intact.
// RFC requirement: RFC8654-3-1 negative -- malformed ATOMIC_AGGREGATE or AGGREGATOR alone is discarded, not accepted, withdrawn or session-reset, by both extended UPDATE readers.
func TestRFC8654ExtendedAttributeDiscard(t *testing.T) {
	for _, tc := range []struct {
		name             string
		code             attribute.AttributeCode
		valid, malformed []byte
	}{
		{"atomic-aggregate", attribute.AttrAtomicAggregate, []byte{0x40, 6, 0}, []byte{0x40, 6, 1, 0}},
		{"aggregator", attribute.AttrAggregator,
			[]byte{0xc0, 7, 8, 0, 0, 0xfd, 0xea, 192, 0, 2, 1},
			[]byte{0xc0, 7, 7, 0, 0, 0xfd, 0xea, 192, 0, 2}},
	} {
		for _, malformed := range []bool{false, true} {
			for _, coalesced := range []bool{false, true} {
				t.Run(tc.name+"/malformed="+strconv.FormatBool(malformed)+"/coalesced="+strconv.FormatBool(coalesced), func(t *testing.T) {
					session, client, capture, cleanup := setupCapturingSession(t, 65002, true, false)
					defer cleanup()
					kept := fatalLengthAnnouncement().PathAttributes
					extra := tc.valid
					if malformed {
						extra = tc.malformed
					}
					attrs := append(append([]byte(nil), kept...), extra...)
					nlri := ipv4Slash24s(1100)
					frame := buildUpdateMsg(receivedUpdateBody(attrs, nlri))
					require.Greater(t, len(frame), message.MaxMsgLen)
					answer := make(chan []byte, 1)
					go func() {
						data, _ := io.ReadAll(client)
						answer <- data
					}()
					written := make(chan error, 1)
					go func() {
						_, err := client.Write(frame)
						written <- err
					}()
					if coalesced {
						require.NoError(t, session.readAndProcessCoalesced(session.Conn(), session.bufReader))
					} else {
						require.NoError(t, session.ReadAndProcess())
					}
					require.NoError(t, <-written)
					got := capture.all()
					require.Len(t, got, 1)
					withdrawn, gotAttrs, gotNLRI := payloadSections(t, got[0])
					require.Empty(t, withdrawn)
					require.Equal(t, nlri, gotNLRI)
					require.GreaterOrEqual(t, len(gotAttrs), len(kept))
					require.Equal(t, kept, gotAttrs[:len(kept)])
					_, _, value, found := attribute.AttrFind(gotAttrs, tc.code)
					if malformed {
						require.False(t, found, "the malformed attribute must not reach route consumers")
					} else {
						require.True(t, found)
						require.Equal(t, extra[3:], value)
						require.Equal(t, attrs, gotAttrs, "a valid attribute is not discarded")
					}
					require.Equal(t, fsm.StateEstablished, session.State())
					cleanup()
					require.Empty(t, <-answer, "attribute discard emits no NOTIFICATION")
				})
			}
		}
	}
}

// TestRFC8654ExtendedDuplicateAttributes observes keep-first at the real session
// consumer, for recognized and unrecognized attributes, through both readers.
// RFC 7606 Section 3(g): "If any other attribute (whether recognized or
// unrecognized) appears more than once in an UPDATE message, then all the
// occurrences of the attribute other than the first one SHALL be discarded and
// the UPDATE message will continue to be processed."
// MUTATION: Skip DuplicateRanges stripping in enforceRFC7606: exact attributes
// differ even though AttrFind would still return the correct first value.
// RFC requirement: RFC8654-3-1 positive -- single recognized and unrecognized attributes in extended UPDATEs reach consumers with only RFC 4271 Section 9's mandated Partial-bit normalization.
// RFC requirement: RFC8654-3-1 negative -- later recognized and unrecognized duplicates alone are removed; the first occurrence and every NLRI survive on the same session.
func TestRFC8654ExtendedDuplicateAttributes(t *testing.T) {
	for _, coalesced := range []bool{false, true} {
		t.Run("coalesced="+strconv.FormatBool(coalesced), func(t *testing.T) {
			session, client, capture, cleanup := setupCapturingSession(t, 65002, true, false)
			defer cleanup()
			answers := make(chan []byte, 1)
			go func() {
				data, _ := io.ReadAll(client)
				answers <- data
			}()
			kept := append(fatalLengthAnnouncement().PathAttributes, 0xc0, 99, 2, 0xaa, 0xbb)
			// RFC 4271 Section 9: "If an optional transitive attribute is
			// unrecognized, the Partial bit (the third high-order bit) in the
			// attribute flags octet is set to 1, and the attribute is retained
			// for propagation to other BGP speakers."
			// publishBase runs after keep-first; retain exact flags in the oracle
			// rather than either expecting the input C0 or masking flag changes.
			published := append(fatalLengthAnnouncement().PathAttributes, 0xe0, 99, 2, 0xaa, 0xbb)
			nlri := ipv4Slash24s(1100)
			for _, duplicate := range []bool{false, true} {
				attrs := append([]byte(nil), kept...)
				if duplicate {
					attrs = append(attrs,
						0x40, 1, 1, 1, // Later valid ORIGIN differs from the first.
						0xc0, 99, 1, 0xcc, // Later unknown attribute differs in length too.
						0x40, 1, 1, 2, // Every later occurrence must disappear.
						0xc0, 99, 2, 0xdd, 0xee)
				}
				frame := buildUpdateMsg(receivedUpdateBody(attrs, nlri))
				require.Greater(t, len(frame), message.MaxMsgLen)
				written := make(chan error, 1)
				go func() {
					_, err := client.Write(frame)
					written <- err
				}()
				if coalesced {
					require.NoError(t, session.readAndProcessCoalesced(session.Conn(), session.bufReader))
				} else {
					require.NoError(t, session.ReadAndProcess())
				}
				require.NoError(t, <-written)
				got := capture.all()
				expected := 1
				if duplicate {
					expected = 2
				}
				require.Len(t, got, expected)
				withdrawn, gotAttrs, gotNLRI := payloadSections(t, got[expected-1])
				require.Empty(t, withdrawn)
				require.Equal(t, published, gotAttrs, "only later duplicate ranges and the required Partial bit change")
				require.Equal(t, nlri, gotNLRI)
				require.Equal(t, fsm.StateEstablished, session.State())
			}
			cleanup()
			require.Empty(t, <-answers, "keep-first emits no NOTIFICATION")
		})
	}
}

// TestRFC8654ExtendedDuplicateMPResets isolates the MP exception to keep-first.
// RFC 7606 Section 3(g): "If the MP_REACH_NLRI attribute or the
// MP_UNREACH_NLRI [RFC4760] attribute appears more than once in the UPDATE
// message, then a NOTIFICATION message MUST be sent with the Error Subcode
// 'Malformed Attribute List'."
// MUTATION: Remove either in-loop duplicate-MP reset in
// ValidateUpdateRFC7606AddPath: that case dispatches instead of NOTIFICATION 3/1.
// RFC requirement: RFC8654-3-1 positive -- a single well-formed MP_REACH or MP_UNREACH in an extended UPDATE is processed without resetting.
// RFC requirement: RFC8654-3-1 negative -- a second identical MP_REACH or MP_UNREACH causes exact NOTIFICATION 3/1, Idle and no additional consumer dispatch.
func TestRFC8654ExtendedDuplicateMPResets(t *testing.T) {
	for _, mp := range [][]byte{
		{0x80, 14, 13, 0, 1, 1, 4, 192, 0, 2, 1, 0, 24, 203, 0, 114},
		{0x80, 15, 7, 0, 1, 1, 24, 203, 0, 114},
	} {
		t.Run(strconv.Itoa(int(mp[1])), func(t *testing.T) {
			session, client, capture, cleanup := setupCapturingSession(t, 65002, true, false)
			defer cleanup()
			attrs := append(fatalLengthAnnouncement().PathAttributes, mp...)
			nlri := ipv4Slash24s(1100)
			answers := make(chan []byte, 1)
			go func() {
				data, _ := io.ReadAll(client)
				answers <- data
			}()
			for _, duplicate := range []bool{false, true} {
				if duplicate {
					attrs = append(attrs, mp...)
				}
				frame := buildUpdateMsg(receivedUpdateBody(attrs, nlri))
				require.Greater(t, len(frame), message.MaxMsgLen)
				written := make(chan error, 1)
				go func() {
					_, err := client.Write(frame)
					written <- err
				}()
				err := session.ReadAndProcess()
				require.NoError(t, <-written)
				if duplicate {
					require.Error(t, err)
					require.Equal(t, fsm.StateIdle, session.State())
				} else {
					require.NoError(t, err)
					require.Equal(t, fsm.StateEstablished, session.State())
					got := capture.all()
					require.Len(t, got, 1)
					require.Equal(t, receivedUpdateBody(attrs, nlri), got[0])
				}
				require.Len(t, capture.all(), 1, "duplicate MP never reaches consumers")
			}
			cleanup()
			assertNotification(t, <-answers, message.NotifyUpdateMessage, message.NotifyUpdateMalformedAttr, []byte{})
		})
	}
}
