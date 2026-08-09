// Design: docs/architecture/ike/ipsec-13-rekey-wire.md -- RFC 7296 Section 2.3 message-ID handling
// RFC: rfc/short/rfc7296.md -- retransmission carries the request's own Message ID (Section 2.1)

package engine

import (
	"bytes"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// VALIDATES: an INVALID_MESSAGE_ID notify that spends a Message ID keeps the datagram
// that carries it, and the owner loop repeats that exact datagram -- so the repeat
// carries the SAME Message ID and costs no second id (RFC 7296 Section 2.1).
// PREVENTS: a lost notify leaving NextMsgID one past the id the peer still expects.
// With a window of one, every later request on the SA is then out of window for the
// peer. Nothing this side sends is answered, and the SA stalls until the liveness
// budget tears it down.
func TestInvalidMsgIDNotifyIsRepeatable(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, _, ps := establishPSK(t)
	peerTr, myTr := rtxPeerLink(t)
	ini.PeerCfg.RemoteAddress = "127.0.0.1"
	if ini.remoteUDPAddr() == nil {
		t.Fatal("the initiator has no resolvable peer address")
	}

	sentID := ini.NextMsgID
	ps.sendInvalidMessageID(ini, 4242, myTr, log)

	first := rtxRecv(t, peerTr)
	if first == nil {
		t.Fatal("the INVALID_MESSAGE_ID notify never reached the peer")
	}
	if ini.NextMsgID != sentID+1 {
		t.Fatalf("the notify spent %d ids, and a request spends exactly one",
			ini.NextMsgID-sentID)
	}
	// The id is spent, so the datagram MUST be recoverable. This is the whole property.
	if len(ini.requestMsg) == 0 {
		t.Fatal("the notify spent a Message ID and stored no datagram, so a lost one " +
			"cannot be repeated and the peer's counter is left behind")
	}
	if !ini.requestOutstanding || ini.requestMsgID != sentID {
		t.Fatalf("the window does not hold the notify: outstanding=%v id=%d want %d",
			ini.requestOutstanding, ini.requestMsgID, sentID)
	}

	// A repeat is made only once the backoff has elapsed, and it is byte-identical.
	now := time.Now()
	if ini.shouldRetransmitRequest(now) {
		t.Fatal("the notify was repeated before its backoff elapsed")
	}
	for attempt := 1; attempt <= maxRequestRetransmits; attempt++ {
		ini.requestLastSent = now.Add(-time.Hour)
		ps.serviceRequestRetransmit(ini, nil, myTr, now, log)
		again := rtxRecv(t, peerTr)
		if again == nil {
			t.Fatalf("repeat %d never reached the peer", attempt)
		}
		if !bytes.Equal(again, first) {
			t.Fatalf("repeat %d is not the original request, so it does not carry its "+
				"Message ID (RFC 7296 Section 2.1)", attempt)
		}
		if ini.NextMsgID != sentID+1 {
			t.Fatalf("repeat %d spent a further Message ID; NextMsgID is %d, want %d",
				attempt, ini.NextMsgID, sentID+1)
		}
	}

	// The repeats are BOUNDED. An unbounded repeat of a rate-limited courtesy would
	// turn one replayed request into a stream of datagrams from this node.
	ini.requestLastSent = now.Add(-time.Hour)
	ps.serviceRequestRetransmit(ini, nil, myTr, now, log)
	rtxExpectSilence(t, peerTr, myTr, ini.remoteUDPAddr(),
		"a notify that has used every repeat")
}

// VALIDATES: the stored datagram is dropped on every path that frees the window, so no
// abandoned request is repeated after its window has moved on.
// PREVENTS: a notify's bytes surviving into a later exchange, where a repeat would put
// a stale Message ID back on the wire.
func TestInvalidMsgIDNotifyClearsOnRelease(t *testing.T) {
	log := slogutil.DiscardLogger()
	now := time.Now()

	for _, tc := range []struct {
		name string
		free func(sa *SA, msgID uint32)
	}{
		{"answered", func(sa *SA, msgID uint32) { sa.answerAuthenticatedResponse(msgID) }},
		{"retired", func(sa *SA, msgID uint32) { sa.retireRequest(msgID) }},
		{"released", func(sa *SA, _ uint32) { sa.releaseRequestWindow() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ini, _, ps := establishPSK(t)
			_, myTr := rtxPeerLink(t)
			ini.PeerCfg.RemoteAddress = "127.0.0.1"

			sentID := ini.NextMsgID
			ps.sendInvalidMessageID(ini, 7, myTr, log)
			if len(ini.requestMsg) == 0 {
				t.Fatal("the notify stored no datagram to begin with")
			}

			tc.free(ini, sentID)

			if len(ini.requestMsg) != 0 || ini.requestAttempts != 0 {
				t.Fatal("the freed window kept the request's bytes, so a later repeat " +
					"would put a Message ID the peer has moved past back on the wire")
			}
			ini.requestLastSent = now.Add(-time.Hour)
			if ini.shouldRetransmitRequest(now) {
				t.Fatal("a freed window still reports a request to repeat")
			}
		})
	}
}
