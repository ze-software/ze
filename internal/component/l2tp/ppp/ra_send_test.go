// Related: ra_send.go -- the sender, the advertised lifetimes, and the stop path.
// RFC: rfc/full/rfc4861.txt -- Sections 6.2.1, 6.2.5. RFC 4861 has no rfc/short/ summary and is not enrolled.

package ppp

import (
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"golang.org/x/net/ipv6"
)

// raRecorder is a raWriter and an io.Closer that records what the stop path did
// and in which order. A raw ICMPv6 socket needs Linux and root, so the recorder
// is how the ordering this package guarantees becomes observable in a unit test.
type raRecorder struct {
	steps     []string // "send" or "close", in call order
	lifetimes []uint16 // Router Lifetime of each RA sent, in send order
	sendErr   error    // what WriteTo returns, for the gone-interface case
	closeErr  error    // what Close returns, for the gone-interface case
}

func (r *raRecorder) WriteTo(b []byte, _ *ipv6.ControlMessage, _ net.Addr) (int, error) {
	r.steps = append(r.steps, "send")
	// RFC 4861 Section 4.2: Router Lifetime is the 16-bit field at octet 6,
	// after type, code, checksum, Cur Hop Limit and the flags octet.
	r.lifetimes = append(r.lifetimes, binary.BigEndian.Uint16(b[6:8]))
	if r.sendErr != nil {
		return 0, r.sendErr
	}
	return len(b), nil
}

func (r *raRecorder) Close() error {
	r.steps = append(r.steps, "close")
	return r.closeErr
}

func newTestRASender(rec *raRecorder) *raSender {
	return &raSender{
		conn:    rec,
		dst:     &net.UDPAddr{IP: net.ParseIP("ff02::1"), Zone: "ppp0"},
		ifIndex: 1,
		ifname:  "ppp0",
		logger:  slog.New(slog.DiscardHandler),
	}
}

// VALIDATES: stopRASender transmits one final Router Advertisement whose Router
// Lifetime is zero, and transmits it BEFORE it closes the socket.
// PREVENTS: a teardown that only closes the socket, which leaves the subscriber
// using a router that is gone until raRouterLifetime (1800 s) expires.
// RFC 4861 Section 6.2.5: an interface that ceases to advertise "SHOULD
// transmit one or more (but not more than MAX_FINAL_RTR_ADVERTISEMENTS) final
// multicast Router Advertisements on the interface with a Router Lifetime field
// of zero".
func TestStopRASenderSendsAZeroLifetimeRABeforeClose(t *testing.T) {
	rec := &raRecorder{}
	senderDone := make(chan struct{})
	close(senderDone)

	canceled := false
	stopRASender(func() { canceled = true }, senderDone, newTestRASender(rec), rec)

	if !canceled {
		t.Error("stopRASender did not cancel the sender goroutine")
	}

	want := []string{"send", "close"}
	if len(rec.steps) != len(want) {
		t.Fatalf("steps = %v, want %v", rec.steps, want)
	}
	for i, step := range want {
		if rec.steps[i] != step {
			t.Fatalf("steps = %v, want %v", rec.steps, want)
		}
	}

	if rec.lifetimes[0] != 0 {
		t.Errorf("final RA Router Lifetime = %d, want 0", rec.lifetimes[0])
	}
}

// VALIDATES: an abrupt teardown, where the interface is already gone and both
// the send and the close fail, still runs the whole stop path and returns.
// PREVENTS: a cease advertisement that is treated as mandatory, so a failure to
// reach a dead interface leaves the socket open or blocks session teardown.
func TestStopRASenderIsBestEffortWhenTheInterfaceIsGone(t *testing.T) {
	rec := &raRecorder{sendErr: net.ErrClosed, closeErr: net.ErrClosed}
	senderDone := make(chan struct{})
	close(senderDone)

	stopRASender(func() {}, senderDone, newTestRASender(rec), rec)

	if len(rec.steps) != 2 || rec.steps[1] != "close" {
		t.Fatalf("steps = %v, want the send attempt then the close", rec.steps)
	}
}

// VALIDATES: stopRASender waits for the sender goroutine to leave before it
// sends the final Router Advertisement.
// PREVENTS: a periodic advertisement racing past the final one, which would
// restore the subscriber's default route for another raRouterLifetime seconds
// immediately after Ze told it the route was gone.
func TestStopRASenderWaitsForTheSenderGoroutine(t *testing.T) {
	rec := &raRecorder{}
	senderDone := make(chan struct{})
	stopped := make(chan struct{})

	go func() {
		defer close(stopped)
		stopRASender(func() {}, senderDone, newTestRASender(rec), rec)
	}()

	select {
	case <-stopped:
		t.Fatal("stopRASender returned before the sender goroutine finished")
	case <-time.After(50 * time.Millisecond):
	}

	if len(rec.steps) != 0 {
		t.Fatalf("steps = %v before the sender goroutine finished, want none", rec.steps)
	}

	close(senderDone)

	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("stopRASender did not return after the sender goroutine finished")
	}
}

// VALIDATES: raSenderLoop advertises raRouterLifetime, not the cease lifetime,
// and closes senderDone when the context is canceled.
// PREVENTS: a steady-state advertisement that carries a zero Router Lifetime,
// and a stop path that blocks forever on a channel nobody closes.
func TestRASenderLoopAdvertisesTheRouterLifetime(t *testing.T) {
	rec := &raRecorder{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	senderDone := make(chan struct{})
	raSenderLoop(ctx, newTestRASender(rec), nil, senderDone)

	select {
	case <-senderDone:
	default:
		t.Fatal("raSenderLoop returned without closing senderDone")
	}

	if len(rec.lifetimes) != 1 {
		t.Fatalf("sent %d RAs on a canceled context, want 1", len(rec.lifetimes))
	}
	if rec.lifetimes[0] != raRouterLifetime {
		t.Errorf("Router Lifetime = %d, want %d", rec.lifetimes[0], raRouterLifetime)
	}
}

// VALIDATES: the periodic interval derives from the advertised Router Lifetime,
// so three unsolicited advertisements fit inside one lifetime window.
// PREVENTS: the two numbers drifting apart, after which one lost advertisement
// near expiry silently drops the subscriber's default route.
// RFC 4861 Section 6.2.1 gives AdvDefaultLifetime a default of
// 3 * MaxRtrAdvInterval.
func TestRAPeriodicIntervalDerivesFromTheRouterLifetime(t *testing.T) {
	lifetime := raRouterLifetime * time.Second

	if raPeriodicInterval*3 != lifetime {
		t.Errorf("3 * raPeriodicInterval = %v, want the router lifetime %v", raPeriodicInterval*3, lifetime)
	}
	if raPeriodicInterval >= lifetime {
		t.Errorf("raPeriodicInterval %v is not inside the router lifetime %v", raPeriodicInterval, lifetime)
	}
}

// The real socket satisfies raWriter, and so does the recorder. Asserted here
// rather than in ra_send.go because ra_linux.go, the only place that assigns a
// real *ipv6.PacketConn, is compiled on Linux alone.
var (
	_ raWriter  = (*ipv6.PacketConn)(nil)
	_ raWriter  = (*raRecorder)(nil)
	_ io.Closer = (*raRecorder)(nil)
)
