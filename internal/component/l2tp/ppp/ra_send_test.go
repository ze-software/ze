// Related: ra_send.go -- the sender, the send loop, and the stop path.
// Related: ra_schedule.go -- the schedule this loop consults.
// RFC: rfc/full/rfc4861.txt -- Sections 6.2.4, 6.2.5, 6.2.6. RFC 4861 has no rfc/short/ summary and is not enrolled.

package ppp

import (
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"runtime"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/ipv6"

	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/ndp"
	"github.com/ze-software/ze/internal/test/sim"
)

// raRecorder is a raWriter and an io.Closer that records what the sender did,
// in which order, and at what time on the fake clock. A raw ICMPv6 socket needs
// Linux and root, so the recorder is how the ordering and the timing this
// package guarantees become observable in a unit test.
//
// Safe for concurrent use: the sender goroutine writes while the test reads.
type raRecorder struct {
	clk clock.Clock // timestamps each send; nil records the zero time

	sendErr  error // what WriteTo returns, for the gone-interface case
	closeErr error // what Close returns, for the gone-interface case

	mu        sync.Mutex
	steps     []string    // "send" or "close", in call order
	lifetimes []uint16    // Router Lifetime of each RA sent, in send order
	at        []time.Time // when each RA was sent, in send order
}

func (r *raRecorder) WriteTo(b []byte, _ *ipv6.ControlMessage, _ net.Addr) (int, error) {
	r.mu.Lock()
	r.steps = append(r.steps, "send")
	// RFC 4861 Section 4.2: Router Lifetime is the 16-bit field at octet 6,
	// after type, code, checksum, Cur Hop Limit and the flags octet.
	r.lifetimes = append(r.lifetimes, binary.BigEndian.Uint16(b[6:8]))
	var now time.Time
	if r.clk != nil {
		now = r.clk.Now()
	}
	r.at = append(r.at, now)
	r.mu.Unlock()

	if r.sendErr != nil {
		return 0, r.sendErr
	}
	return len(b), nil
}

func (r *raRecorder) Close() error {
	r.mu.Lock()
	r.steps = append(r.steps, "close")
	r.mu.Unlock()
	return r.closeErr
}

// sends returns how many Router Advertisements have been written.
func (r *raRecorder) sends() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.at)
}

// sentAt returns when advertisement i was written, on the recorder's clock.
func (r *raRecorder) sentAt(i int) time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.at[i]
}

// order returns the send and close calls in the order they were made.
func (r *raRecorder) order() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.steps...)
}

// sentLifetimes returns the Router Lifetime each advertisement carried, in
// send order.
func (r *raRecorder) sentLifetimes() []uint16 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]uint16(nil), r.lifetimes...)
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

// sleepClock records what the stop path asked the clock to sleep for, and moves
// the fake clock forward by that much.
// sim.FakeClock.Sleep returns at once, which is what keeps these tests fast and
// is also what leaves nothing to assert without this wrapper.
//
// The advance is what makes the ORDER of the stop path observable. Recording
// the duration alone says the stop path asked for the right wait; it says
// nothing about whether the wait came before the advertisement it exists to
// delay. With the clock moving, the send time carries that answer, so swapping
// the sleep and the send is a change a test can see.
type sleepClock struct {
	*sim.FakeClock
	slept []time.Duration
}

func (c *sleepClock) Sleep(d time.Duration) {
	c.slept = append(c.slept, d)
	c.Add(d)
}

// raStepInterval is how far each pass of advanceUntil moves the fake clock. It
// is far below MIN_DELAY_BETWEEN_RAS and MAX_RA_DELAY_TIME, so the sender never
// skips over a deadline it has not armed yet.
const raStepInterval = 10 * time.Millisecond

// advanceUntil steps the fake clock forward until the recorder holds want
// advertisements, and fails the test if that never happens.
//
// The sender arms its timer from the same fake clock and the test cannot see
// when it does, so the clock is stepped rather than jumped: a jump lands past a
// deadline the sender is about to compute from a now that has already moved.
// Every assertion that follows a step loop also bounds the send time from
// above, which is what catches a sender that fell behind rather than obeyed a
// timer.
func advanceUntil(t *testing.T, clk *sim.FakeClock, rec *raRecorder, want int) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for rec.sends() < want {
		if time.Now().After(deadline) {
			t.Fatalf("the sender wrote %d advertisements, want %d", rec.sends(), want)
		}
		clk.Add(raStepInterval)
		runtime.Gosched()
	}
}

// startTestRASenderLoop starts the loop on a fake clock and returns everything
// the test drives it with. The returned stop function cancels the loop and
// waits for it, so no goroutine outlives the test.
func startTestRASenderLoop(t *testing.T, seed uint64) (*raRecorder, *sim.FakeClock, chan struct{}, func()) {
	t.Helper()

	clk := sim.NewFakeClock(raTestStart)
	rec := &raRecorder{clk: clk}
	sched, _ := newRATestSchedule(seed)
	sched.clk = clk

	ctx, cancel := context.WithCancel(context.Background())
	rsCh := make(chan struct{})
	senderDone := make(chan struct{})
	go raSenderLoop(ctx, newTestRASender(rec), sched, rsCh, senderDone)

	// The loop advertises before it waits, so the first advertisement is
	// recorded without the clock moving.
	advanceUntil(t, clk, rec, 1)

	return rec, clk, rsCh, func() {
		cancel()
		<-senderDone
	}
}

// VALIDATES: RFC 4861 Section 6.2.6, "consecutive Router Advertisements sent to
// the all-nodes multicast address MUST be rate limited to no more than one
// advertisement every MIN_DELAY_BETWEEN_RAS seconds". A solicitation arriving
// right after an advertisement is answered when the window closes.
// PREVENTS: the defect this loop shipped with, where every solicitation
// coalesced onto rsCh produced an immediate advertisement, so a subscriber
// sending one solicitation per millisecond set Ze's send rate.
func TestRASenderLoopRateLimitsAnAnswerToASolicitation(t *testing.T) {
	rec, clk, rsCh, stop := startTestRASenderLoop(t, 21)
	defer stop()

	rsCh <- struct{}{}
	advanceUntil(t, clk, rec, 2)

	gap := rec.sentAt(1).Sub(rec.sentAt(0))
	if gap < ndp.MinDelayBetweenRAs {
		t.Errorf("the answer followed the previous advertisement by %v, want at least %v", gap, ndp.MinDelayBetweenRAs)
	}
	// The upper bound is what makes the lower bound evidence: a sender that
	// simply lagged behind the stepping clock would also pass the floor.
	if limit := ndp.MinDelayBetweenRAs + ndp.MaxRADelayTime + 30*raStepInterval; gap > limit {
		t.Errorf("the answer followed the previous advertisement by %v, want at most %v", gap, limit)
	}
}

// VALIDATES: RFC 4861 Section 6.2.6, "Router Advertisements sent in response to
// a Router Solicitation MUST be delayed by a random time between 0 and
// MAX_RA_DELAY_TIME seconds". Outside the rate-limit window the answer carries
// that delay and nothing more.
// PREVENTS: a loop that reads rsCh and sends, ignoring the schedule, and a loop
// that leaves a solicitation to the next unsolicited advertisement.
func TestRASenderLoopAnswersASolicitationWithinMaxRADelayTime(t *testing.T) {
	rec, clk, rsCh, stop := startTestRASenderLoop(t, 22)
	defer stop()

	// Step past the rate-limit window. The first unsolicited interval is at
	// least MAX_INITIAL_RTR_ADVERT_INTERVAL away, so nothing is due yet.
	for clk.Now().Sub(rec.sentAt(0)) < ndp.MinDelayBetweenRAs+time.Second {
		clk.Add(raStepInterval)
		runtime.Gosched()
	}
	if rec.sends() != 1 {
		t.Fatalf("the sender wrote %d advertisements before any solicitation, want 1", rec.sends())
	}

	solicitedAt := clk.Now()
	rsCh <- struct{}{}
	advanceUntil(t, clk, rec, 2)

	delay := rec.sentAt(1).Sub(solicitedAt)
	if delay < 0 || delay > ndp.MaxRADelayTime+30*raStepInterval {
		t.Errorf("the answer came %v after the solicitation, want 0 to %v", delay, ndp.MaxRADelayTime)
	}
}

// VALIDATES: RFC 4861 Section 6.2.6, "(If a single advertisement is sent in
// response to multiple solicitations, the delay is relative to the first
// solicitation.)" A burst of solicitations produces one advertisement.
// PREVENTS: one advertisement per solicitation, which is what rsCh coalescing
// alone leaves when the loop answers each token it reads.
func TestRASenderLoopAnswersABurstOfSolicitationsOnce(t *testing.T) {
	rec, clk, rsCh, stop := startTestRASenderLoop(t, 23)
	defer stop()

	for clk.Now().Sub(rec.sentAt(0)) < ndp.MinDelayBetweenRAs+time.Second {
		clk.Add(raStepInterval)
		runtime.Gosched()
	}

	// Every send blocks until the loop receives it, so all five reach the
	// schedule before the clock moves again.
	solicitedAt := clk.Now()
	for range 5 {
		rsCh <- struct{}{}
	}

	advanceUntil(t, clk, rec, 2)

	delay := rec.sentAt(1).Sub(solicitedAt)
	if delay < 0 || delay > ndp.MaxRADelayTime+30*raStepInterval {
		t.Errorf("the answer came %v after the first solicitation, want 0 to %v", delay, ndp.MaxRADelayTime)
	}

	// Nothing else is due for at least raMinRtrAdvInterval, so a second
	// answer here is one solicitation answered twice.
	for range 200 {
		clk.Add(raStepInterval)
		runtime.Gosched()
	}
	if got := rec.sends(); got != 2 {
		t.Errorf("five solicitations produced %d advertisements, want 2 (the initial one and one answer)", got)
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
	sched, _ := newRATestSchedule(1)
	senderDone := make(chan struct{})
	close(senderDone)

	canceled := false
	stopRASender(func() { canceled = true }, senderDone, newTestRASender(rec), sched, rec)

	if !canceled {
		t.Error("stopRASender did not cancel the sender goroutine")
	}

	want := []string{"send", "close"}
	steps := rec.order()
	if len(steps) != len(want) {
		t.Fatalf("steps = %v, want %v", steps, want)
	}
	for i, step := range want {
		if steps[i] != step {
			t.Fatalf("steps = %v, want %v", steps, want)
		}
	}

	if lifetimes := rec.sentLifetimes(); lifetimes[0] != 0 {
		t.Errorf("final RA Router Lifetime = %d, want 0", lifetimes[0])
	}
}

// VALIDATES: RFC 4861 Section 6.2.6. The final zero-lifetime advertisement is a
// multicast advertisement like any other, so it waits out the remainder of the
// MIN_DELAY_BETWEEN_RAS window and waits for nothing once that window closed.
// PREVENTS: a teardown that breaks the rate limit when a session ends straight
// after it advertised, and a teardown that pauses on every session.
func TestStopRASenderWaitsOutTheRateLimit(t *testing.T) {
	tests := []struct {
		name      string
		sinceLast time.Duration
		wantSleep time.Duration
	}{
		{name: "torn down one second after advertising", sinceLast: time.Second, wantSleep: ndp.MinDelayBetweenRAs - time.Second},
		{name: "torn down after the window closed", sinceLast: ndp.MinDelayBetweenRAs, wantSleep: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clk := &sleepClock{FakeClock: sim.NewFakeClock(raTestStart)}
			rec := &raRecorder{clk: clk}
			sched, _ := newRATestSchedule(2)
			sched.clk = clk

			sched.advertised()
			advertisedAt := clk.Now()
			clk.Add(test.sinceLast)

			senderDone := make(chan struct{})
			close(senderDone)
			stopRASender(func() {}, senderDone, newTestRASender(rec), sched, rec)

			if len(clk.slept) != 1 {
				t.Fatalf("the stop path slept %d time(s), want once", len(clk.slept))
			}
			if clk.slept[0] != test.wantSleep {
				t.Errorf("the stop path slept %v before the final advertisement, want %v", clk.slept[0], test.wantSleep)
			}
			// The wait is only worth anything if it happens BEFORE the send it
			// delays. Asserting the gap on the clock, rather than the duration
			// alone, is what fails when the two are the other way round.
			if rec.sends() != 1 {
				t.Fatalf("the stop path sent %d advertisement(s), want one", rec.sends())
			}
			if gap := rec.sentAt(0).Sub(advertisedAt); gap < ndp.MinDelayBetweenRAs {
				t.Errorf("the final advertisement left %v after the previous one, want at least %v", gap, ndp.MinDelayBetweenRAs)
			}
		})
	}
}

// VALIDATES: an abrupt teardown, where the interface is already gone and both
// the send and the close fail, still runs the whole stop path and returns.
// PREVENTS: a cease advertisement that is treated as mandatory, so a failure to
// reach a dead interface leaves the socket open or blocks session teardown.
func TestStopRASenderIsBestEffortWhenTheInterfaceIsGone(t *testing.T) {
	rec := &raRecorder{sendErr: net.ErrClosed, closeErr: net.ErrClosed}
	sched, _ := newRATestSchedule(1)
	senderDone := make(chan struct{})
	close(senderDone)

	stopRASender(func() {}, senderDone, newTestRASender(rec), sched, rec)

	steps := rec.order()
	if len(steps) != 2 || steps[1] != "close" {
		t.Fatalf("steps = %v, want the send attempt then the close", steps)
	}
}

// VALIDATES: stopRASender waits for the sender goroutine to leave before it
// sends the final Router Advertisement.
// PREVENTS: a periodic advertisement racing past the final one, which would
// restore the subscriber's default route for another raRouterLifetime seconds
// immediately after Ze told it the route was gone.
func TestStopRASenderWaitsForTheSenderGoroutine(t *testing.T) {
	rec := &raRecorder{}
	sched, _ := newRATestSchedule(1)
	senderDone := make(chan struct{})
	stopped := make(chan struct{})

	go func() {
		defer close(stopped)
		stopRASender(func() {}, senderDone, newTestRASender(rec), sched, rec)
	}()

	select {
	case <-stopped:
		t.Fatal("stopRASender returned before the sender goroutine finished")
	case <-time.After(50 * time.Millisecond):
	}

	if steps := rec.order(); len(steps) != 0 {
		t.Fatalf("steps = %v before the sender goroutine finished, want none", steps)
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
	sched, _ := newRATestSchedule(1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	senderDone := make(chan struct{})
	raSenderLoop(ctx, newTestRASender(rec), sched, nil, senderDone)

	select {
	case <-senderDone:
	default:
		t.Fatal("raSenderLoop returned without closing senderDone")
	}

	if rec.sends() != 1 {
		t.Fatalf("sent %d RAs on a canceled context, want 1", rec.sends())
	}
	if lifetimes := rec.sentLifetimes(); lifetimes[0] != raRouterLifetime {
		t.Errorf("Router Lifetime = %d, want %d", lifetimes[0], raRouterLifetime)
	}
}

// The real socket satisfies raWriter, and so does the recorder. Asserted here
// rather than in ra_send.go because ra_linux.go, the only place that assigns a
// real *ipv6.PacketConn, is compiled on Linux alone.
var (
	_ raWriter    = (*ipv6.PacketConn)(nil)
	_ raWriter    = (*raRecorder)(nil)
	_ io.Closer   = (*raRecorder)(nil)
	_ clock.Clock = (*sleepClock)(nil)
)
