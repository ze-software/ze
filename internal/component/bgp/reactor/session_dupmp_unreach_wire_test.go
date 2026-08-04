// RFC: rfc/short/rfc7606.md -- Section 3.g duplicate MP attributes
// Overview: session_validation.go -- rfc7606SessionReset, the producer this observes
// Related: session_test.go -- TestSessionRFC7606SessionResetNotification, the MP_REACH leg
//
// VALIDATES: a duplicate MP_UNREACH_NLRI puts a NOTIFICATION with code 3 subcode 1 ON THE
// WIRE, observed by reading the bytes a real peer would receive.
//
// PREVENTS: the MP_UNREACH half of RFC 7606 Section 3.g holding by construction rather
// than by observation. Before this test, every unit cited for RFC7606-3.g-1 that read a
// NOTIFICATION off the wire used a duplicate MP_REACH. The MP_UNREACH leg reached the same
// notification only because both funnel through Session.rfc7606SessionReset
// (session_validation.go). That is true today and nothing pinned it: a change routing the
// MP_UNREACH leg to a different notification, or to a silent drop, would have left every
// cited test green. An independent RFC audit named this as the one remaining gap on the
// requirement that could hide a real defect.

package reactor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
)

// RFC requirement: RFC7606-3.g-1 negative -- a second MP_UNREACH_NLRI puts a NOTIFICATION carrying code 3 subcode 1 on the wire, read back as a peer would receive it rather than inferred from the shared reset helper.
func TestSessionRFC7606DuplicateMPUnreachNotificationOnTheWire(t *testing.T) {
	session, client, callbackCount, cleanup := setupEstablishedSessionEBGP(t)
	defer cleanup()

	// One MP_UNREACH_NLRI withdrawing 10.0.0.0/8 from ipv4/unicast. RFC 4760 Section 4
	// gives MP_UNREACH no next-hop and no reserved octet, so the value is AFI, SAFI and
	// then the withdrawn NLRI.
	mpUnreach := []byte{
		0x00, 0x01, // AFI = 1 (IPv4)
		0x01,       // SAFI = 1 (unicast)
		0x08, 0x0a, // withdrawn NLRI: 10.0.0.0/8
	}

	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH (empty)
	}
	// Twice. The second is the violation Section 3.g names.
	for range 2 {
		pathAttrs = append(pathAttrs, 0x80, 0x0f, byte(len(mpUnreach)))
		pathAttrs = append(pathAttrs, mpUnreach...)
	}

	update := make([]byte, 0, 64)
	update = append(update, 0x00, 0x00, byte(len(pathAttrs)>>8), byte(len(pathAttrs)))
	update = append(update, pathAttrs...)

	updateMsg := buildUpdateMsg(update)

	var received []byte
	done := make(chan struct{})
	go func() {
		client.Write(updateMsg) //nolint:errcheck // test goroutine
		buf := make([]byte, 4096)
		n, _ := client.Read(buf) //nolint:errcheck // read NOTIFICATION
		received = buf[:n]
		close(done)
	}()

	err := session.ReadAndProcess()
	require.Error(t, err, "RFC 7606 Section 3.g: a duplicate MP_UNREACH_NLRI must reset the session")
	require.Contains(t, err.Error(), "session reset")

	require.Equal(t, fsm.StateIdle, session.State(), "the session must be Idle after a Section 3.g reset")
	require.Equal(t, 0, *callbackCount, "no route may be dispatched from an UPDATE that resets the session")

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the NOTIFICATION")
	}

	// The wire bytes are the point of this test. Everything above is reachable through
	// the return value; only these last four assertions prove what the peer receives.
	require.GreaterOrEqual(t, len(received), message.HeaderLen+2, "NOTIFICATION too short to carry a subcode")
	hdr, hdrErr := message.ParseHeader(received[:message.HeaderLen])
	require.NoError(t, hdrErr)
	require.Equal(t, msgtype.TypeNOTIFICATION, hdr.Type, "the peer must receive a NOTIFICATION")

	notifBody := received[message.HeaderLen:]
	// RFC 4271 Section 6.3: UPDATE Message Error is code 3.
	require.Equal(t, byte(message.NotifyUpdateMessage), notifBody[0],
		"error code must be 3, UPDATE Message Error")
	// RFC 7606 Section 3.g names the subcode: "a NOTIFICATION message MUST be sent with
	// the Error Subcode 'Malformed Attribute List'".
	require.Equal(t, message.NotifyUpdateMalformedAttr, notifBody[1],
		"error subcode must be 1, Malformed Attribute List")
}
