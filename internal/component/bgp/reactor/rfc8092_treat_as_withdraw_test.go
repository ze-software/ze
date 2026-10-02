package reactor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
)

// rfc8092SessionAttrs returns ORIGIN IGP, an empty AS_PATH (an internal
// peer), NEXT_HOP 192.0.2.254 and a LARGE_COMMUNITY attribute carrying value.
func rfc8092SessionAttrs(value []byte) []byte {
	attrs := []byte{
		0x40, 0x01, 0x01, 0x00,
		0x40, 0x02, 0x00,
		0x40, 0x03, 0x04, 192, 0, 2, 254,
		0xc0, 0x20, byte(len(value)),
	}
	return append(attrs, value...)
}

// TestRFC8092MalformedLargeCommunityWithdrawsTheRoutes drives an UPDATE whose
// LARGE_COMMUNITY is 10 octets long through a live, Established session and
// asserts what the session hands to the RIB and the forward rails.
//
// Method: the UPDATE announces 10.0.0.0/24. Treat-as-withdraw (RFC 7606
// Section 2) means the route is handled as withdrawn: the one dispatched
// UPDATE carries 10.0.0.0/24 in its Withdrawn Routes field, no NLRI and no
// path attributes, the session stays Established, and no NOTIFICATION is sent.
//
// RFC requirement: RFC8092-6-4 positive -- an UPDATE whose LARGE_COMMUNITY
// length is 10 is dispatched as a withdrawal of its route (withdrawn
// 18 0a 00 00, no attributes, no NLRI), with no NOTIFICATION and the session
// still Established.
func TestRFC8092MalformedLargeCommunityWithdrawsTheRoutes(t *testing.T) {
	session, client, capture, cleanup := setupCapturingSession(t, 65001, false)
	defer cleanup()
	nlri := []byte{0x18, 0x0a, 0x00, 0x00}
	attrs := rfc8092SessionAttrs([]byte{0x00, 0x01, 0x00, 0x02, 0x00, 0x03, 0x00, 0x04, 0x00, 0x05})

	answer := make(chan []byte, 1)
	go func() {
		_, _ = client.Write(buildUpdateMsg(receivedUpdateBody(attrs, nlri)))
		answer <- readOnce(client, 200*time.Millisecond)
	}()

	require.NoError(t, session.ReadAndProcess())
	require.Equal(t, fsm.StateEstablished, session.State())
	got := capture.all()
	require.Len(t, got, 1, "the malformed UPDATE is dispatched once, as a withdrawal")
	gotWithdrawn, gotAttrs, gotNLRI := payloadSections(t, got[0])
	assert.Equal(t, nlri, gotWithdrawn, "the announced route is withdrawn")
	assert.Empty(t, gotAttrs, "the withdrawal carries no path attributes")
	assert.Empty(t, gotNLRI, "nothing is announced")
	assert.Empty(t, <-answer, "treat-as-withdraw sends no NOTIFICATION")
}

// TestRFC8092WellFormedLargeCommunityKeepsTheRoutes is the counterpart: the
// same UPDATE with a 12-octet LARGE_COMMUNITY is dispatched unchanged, as an
// announcement.
//
// RFC requirement: RFC8092-6-4 negative -- an UPDATE whose LARGE_COMMUNITY
// length is 12 is dispatched with its attributes and NLRI 18 0a 00 00
// unchanged and an empty Withdrawn Routes field.
func TestRFC8092WellFormedLargeCommunityKeepsTheRoutes(t *testing.T) {
	session, client, capture, cleanup := setupCapturingSession(t, 65001, false)
	defer cleanup()
	nlri := []byte{0x18, 0x0a, 0x00, 0x00}
	attrs := rfc8092SessionAttrs([]byte{
		0x00, 0x00, 0xfd, 0xe9,
		0x00, 0x00, 0x00, 0x01,
		0x00, 0x00, 0x00, 0x02,
	})

	answer := make(chan []byte, 1)
	go func() {
		_, _ = client.Write(buildUpdateMsg(receivedUpdateBody(attrs, nlri)))
		answer <- readOnce(client, 200*time.Millisecond)
	}()

	require.NoError(t, session.ReadAndProcess())
	require.Equal(t, fsm.StateEstablished, session.State())
	got := capture.all()
	require.Len(t, got, 1)
	gotWithdrawn, gotAttrs, gotNLRI := payloadSections(t, got[0])
	assert.Empty(t, gotWithdrawn, "nothing is withdrawn")
	assert.Equal(t, attrs, gotAttrs, "the attributes arrive unchanged")
	assert.Equal(t, nlri, gotNLRI, "the route is announced")
	assert.Empty(t, <-answer, "a well-formed UPDATE draws no NOTIFICATION")
}
