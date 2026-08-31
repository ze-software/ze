// Design: docs/research/l2tpv2-ze-integration.md -- RADIUS accounting
// Related: acct.go -- interimLoop, sendAcctInterimUpdate, sendAcctPacket
//
// VALIDATES: that one session never has two interim Accounting generations
// outstanding at once.
// PREVENTS: the shape RFC 2869 Section 2.1 names. Every interim record is
// CUMULATIVE, so two generations racing in the retransmission queue let the
// server record the older totals last and see the session's counters go
// backwards. Dispatching each tick into its own goroutine, or shortening the
// interval below the retransmission budget without bounding the queue, produces
// exactly that.

package l2tpauthradius

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/radius"
)

// rfc2869StallServer counts Accounting-Requests and never answers one, so every
// send runs its full retransmission budget before the caller is released.
func rfc2869StallServer(t *testing.T, seen *atomic.Int64) string {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, radius.MaxPacketLen)
		for {
			n, _, readErr := conn.ReadFromUDP(buf)
			if readErr != nil {
				return
			}
			if pkt, decErr := radius.Decode(buf[:n]); decErr == nil && pkt.Code == radius.CodeAccountingReq {
				seen.Add(1)
			}
		}
	}()
	t.Cleanup(func() {
		conn.Close() //nolint:errcheck // test cleanup
		<-done
	})
	return conn.LocalAddr().String()
}

// rfc2869InterimHarness wires a radiusAcct to serverAddr and returns the loop's
// arguments, so a case only has to choose the tick interval.
func rfc2869InterimHarness(t *testing.T, serverAddr string, timeout time.Duration, retries int) (*radiusAcct, *acctSession) {
	t.Helper()
	client, err := radius.NewClient(radius.ClientConfig{
		Servers: []radius.Server{{Address: serverAddr, SharedKey: []byte("interimtest")}},
		Timeout: timeout,
		Retries: retries,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { closeQuietly(client) })

	acct := newRADIUSAcct()
	acct.setClient(client, "test-nas", 300*time.Second, serverAddr, nil, "")

	sess := &acctSession{
		tunnelID:   1,
		sessionID:  2,
		username:   "alice",
		peerAddr:   "10.0.0.1",
		acctSessID: "sess-interim",
		startTime:  time.Now(),
	}
	return acct, sess
}

func closeQuietly(c interface{ Close() error }) {
	c.Close() //nolint:errcheck // test cleanup
}

// TestRFC2869InterimLoopSendsOnItsInterval proves the loop does send, so the
// bound the case below asserts is a bound on real traffic and not on silence.
//
// RFC requirement: RFC2869-2.1-1 positive -- interimLoop sends an interim
// Accounting-Request once per interval while the session is up (acct.go).
func TestRFC2869InterimLoopSendsOnItsInterval(t *testing.T) {
	var seen atomic.Int64
	addr := rfc2869StallServer(t, &seen)
	// A short retransmission budget keeps each generation brief, so the interval
	// rather than the budget sets the pace.
	acct, sess := rfc2869InterimHarness(t, addr, 10*time.Millisecond, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	acct.interimLoop(ctx, nil, sess, "test-nas", nil, 30*time.Millisecond)

	if got := seen.Load(); got < 2 {
		t.Fatalf("the server saw %d interim Accounting-Request(s) in 500ms at a 30ms "+
			"interval; the loop is not sending", got)
	}
}

// TestRFC2869InterimLoopKeepsOneGenerationOutstanding ticks far faster than the
// retransmission budget. A loop that queued a generation per tick would put
// roughly 1s/10ms = 100 requests on the wire; a loop that keeps one outstanding
// is bounded by the budget instead.
//
// RFC 2869 Section 2.1: "Since all the information is cumulative, a NAS MUST
// ensure that only a single generation of an interim Accounting message for a
// given session is present in the retransmission queue at any given time."
//
// RFC requirement: RFC2869-2.1-1 negative -- a tick that arrives while a
// generation is still in the retransmission queue starts no second generation,
// so the send rate is bounded by the retransmission budget and not by the tick
// interval (interimLoop, acct.go).
func TestRFC2869InterimLoopKeepsOneGenerationOutstanding(t *testing.T) {
	var seen atomic.Int64
	addr := rfc2869StallServer(t, &seen)
	// Retries 2 with a 100ms first timeout: 100ms then 200ms, so one generation
	// occupies the queue for about 300ms.
	acct, sess := rfc2869InterimHarness(t, addr, 100*time.Millisecond, 2)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	acct.interimLoop(ctx, nil, sess, "test-nas", nil, 10*time.Millisecond)

	// One generation is 3 datagrams over about 300ms, so 1s admits at most 4
	// generations. Twelve datagrams leaves room for scheduling jitter and still
	// refuses the 100 a per-tick dispatch would produce.
	const maxDatagrams = 12
	got := seen.Load()
	if got > maxDatagrams {
		t.Fatalf("the server saw %d interim datagram(s) in 1s at a 10ms tick with a "+
			"300ms retransmission budget; at most %d fit if only one generation is "+
			"outstanding at a time", got, maxDatagrams)
	}
	if got == 0 {
		t.Fatal("the server saw no interim datagram; the bound above is vacuous")
	}
}
