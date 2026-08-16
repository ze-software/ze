// Tests for the streaming ping session (stream.go).
//
// VALIDATES: probe cadence stays at `interval` under loss, late/out-of-order
// replies are attributed to their own sequence, timeouts fire at each probe's
// own deadline, bounded runs end after the last probe resolves, id/source
// filtering is preserved, and teardown closes the channel exactly once without
// leaking a goroutine (including the real-clock reaper path under -race).
// PREVENTS: regression to the old serial loop, where blocking in ReadFrom until
// each probe's reply-or-deadline degraded send cadence to ~max(interval,
// timeout) under loss and discarded a reply that arrived after the next probe
// (spec plan/spec-fixit-ping-monitor-cadence.md, AC-1..AC-8).
//
// Most tests drive runPingSession through a fake pingConn and a fake clock
// (internal/test/sim), so loss, delay, reordering, timeout and teardown are all
// deterministic with no raw socket (CAP_NET_RAW) and no wall-clock sleeps in the
// assertions. One test uses clock.RealClock so the production AfterFunc reaper
// goroutine and its teardown are exercised under -race.

package cmd

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"net/netip"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/test/sim"
)

var testEpoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

const (
	testEcho      = byte(8)
	testEchoReply = byte(0)
	v6Echo        = byte(128)
	v6EchoReply   = byte(129)
)

func testPID() uint16 { return uint16(os.Getpid() & 0xffff) }

type writeRec struct {
	seq uint16
	at  time.Time
}

type injectedPkt struct {
	data []byte
	from net.Addr
}

// fakePingConn is a deterministic pingConn. WriteTo hands each send to the test
// over `wrote` (so the test synchronizes on the send actually happening), and
// ReadFrom delivers reply packets the test injects, blocking until one arrives
// or Close is called. The unbuffered channels make every send/inject a
// happens-before synchronization point.
type fakePingConn struct {
	clk       clock.Clock
	wrote     chan writeRec
	readCh    chan injectedPkt
	closed    chan struct{}
	closeOnce sync.Once

	mu       sync.Mutex
	writeErr error
}

func newFakePingConn(clk clock.Clock) *fakePingConn {
	return &fakePingConn{
		clk:    clk,
		wrote:  make(chan writeRec),
		readCh: make(chan injectedPkt),
		closed: make(chan struct{}),
	}
}

func (fc *fakePingConn) setWriteErr(err error) {
	fc.mu.Lock()
	fc.writeErr = err
	fc.mu.Unlock()
}

func (fc *fakePingConn) WriteTo(p []byte, _ net.Addr) (int, error) {
	fc.mu.Lock()
	werr := fc.writeErr
	fc.mu.Unlock()
	if werr != nil {
		return 0, werr
	}
	seq := binary.BigEndian.Uint16(p[6:8])
	select {
	case fc.wrote <- writeRec{seq: seq, at: fc.clk.Now()}:
	case <-fc.closed:
	}
	return len(p), nil
}

func (fc *fakePingConn) ReadFrom(p []byte) (int, net.Addr, error) {
	select {
	case ip := <-fc.readCh:
		n := copy(p, ip.data)
		return n, ip.from, nil
	case <-fc.closed:
		return 0, nil, net.ErrClosed
	}
}

func (fc *fakePingConn) Close() error {
	fc.closeOnce.Do(func() { close(fc.closed) })
	return nil
}

// inject pushes a raw ICMP reply of the given type for (id, seq) from `from`. It
// blocks until the receiver goroutine consumes it: a synchronization point.
func (fc *fakePingConn) inject(replyType byte, id, seq uint16, from net.Addr) {
	pkt := make([]byte, 8)
	pkt[0] = replyType
	binary.BigEndian.PutUint16(pkt[4:6], id)
	binary.BigEndian.PutUint16(pkt[6:8], seq)
	select {
	case fc.readCh <- injectedPkt{data: pkt, from: from}:
	case <-fc.closed:
	}
}

// injectReply injects a well-formed v4 echo reply (nil source => source check
// is skipped, matching a kernel that hands back no address).
func (fc *fakePingConn) injectReply(id, seq uint16) { fc.inject(testEchoReply, id, seq, nil) }

func testDest() netip.Addr { return netip.MustParseAddr("192.0.2.1") }

// startSession launches runPingSession on a v4 target with a fresh fake conn and
// the given clock, returning the handles the tests drive.
func startSession(clk clock.Clock, interval, timeout time.Duration, count int) (*fakePingConn, chan map[string]any, context.CancelFunc) {
	return startSessionOn(clk, testDest(), testEcho, testEchoReply, interval, timeout, count)
}

func startSessionOn(clk clock.Clock, dest netip.Addr, echo, echoReply byte, interval, timeout time.Duration, count int) (*fakePingConn, chan map[string]any, context.CancelFunc) {
	fc := newFakePingConn(clk)
	out := make(chan map[string]any, 64)
	ctx, cancel := context.WithCancel(context.Background())
	go runPingSession(ctx, fc, clk, dest, interval, timeout, count, 0, echo, echoReply, out)
	return fc, out, cancel
}

// TestStreamPingCadenceHoldsUnderLoss VALIDATES AC-1/AC-2: probe cadence holds
// at `interval` under 100% loss. No reply is ever injected; the sends must still
// be spaced exactly one interval apart. Against the old serial design this test
// would hang at <-fc.wrote (no reply is ever delivered), which is the failure.
func TestStreamPingCadenceHoldsUnderLoss(t *testing.T) {
	clk := sim.NewFakeClock(testEpoch)
	fc, out, cancel := startSession(clk, time.Second, 5*time.Second, 0)
	defer cancel()

	r0 := <-fc.wrote // first probe: immediate
	if r0.seq != 0 {
		t.Fatalf("first send seq = %d, want 0", r0.seq)
	}

	prev := r0
	for want := uint16(1); want <= 3; want++ {
		clk.Add(time.Second)
		clk.FireTickers()
		r := <-fc.wrote
		if r.seq != want {
			t.Fatalf("send seq = %d, want %d", r.seq, want)
		}
		if gap := r.at.Sub(prev.at); gap != time.Second {
			t.Fatalf("send %d spacing = %v, want 1s (cadence degraded under loss)", want, gap)
		}
		prev = r
	}

	// No reply was ever delivered, yet four probes went out on cadence.
	select {
	case m := <-out:
		t.Fatalf("unexpected reply with no injected packets: %v", m)
	default:
	}
}

// TestStreamPingTimeoutAtOwnDeadline VALIDATES AC-2: a lost probe reports timeout
// at ITS OWN deadline. Advancing to just below the deadline must emit nothing;
// only reaching the deadline fires the timeout. This pins the reaper duration
// (a regression arming it with `interval` instead of `timeout` would fire early
// and fail the "nothing yet" check).
func TestStreamPingTimeoutAtOwnDeadline(t *testing.T) {
	clk := sim.NewFakeClock(testEpoch)
	fc, out, cancel := startSession(clk, time.Second, 3*time.Second, 0)
	defer cancel()

	r0 := <-fc.wrote
	if r0.seq != 0 {
		t.Fatalf("first send seq = %d, want 0", r0.seq)
	}

	// Just short of seq 0's 3s deadline: nothing must be reported yet.
	clk.Add(3*time.Second - time.Millisecond)
	select {
	case m := <-out:
		t.Fatalf("probe reported before its deadline: %v", m)
	default:
	}

	clk.Add(time.Millisecond) // now exactly at the deadline
	m := <-out
	if got, _ := m["seq"].(int); got != 0 {
		t.Fatalf("timeout seq = %v, want 0", m["seq"])
	}
	if got, _ := m["status"].(string); got != "timeout" {
		t.Fatalf("status = %q, want timeout", got)
	}
	if _, ok := m["rtt-ms"]; ok {
		t.Fatalf("timeout reply carried rtt-ms: %v", m)
	}
}

// TestStreamPingMatchesLateReply VALIDATES AC-3: a reply for seq N that arrives
// AFTER probe N+1 was sent is attributed to N with its true RTT, not dropped.
func TestStreamPingMatchesLateReply(t *testing.T) {
	clk := sim.NewFakeClock(testEpoch)
	fc, out, cancel := startSession(clk, time.Second, 10*time.Second, 0)
	defer cancel()

	pid := testPID()

	r0 := <-fc.wrote // seq 0 at epoch
	if r0.seq != 0 {
		t.Fatalf("first send seq = %d, want 0", r0.seq)
	}

	clk.Add(time.Second)
	clk.FireTickers()
	r1 := <-fc.wrote // seq 1 at epoch+1s; seq 0 still in flight
	if r1.seq != 1 {
		t.Fatalf("second send seq = %d, want 1", r1.seq)
	}

	// Now, a full 2s after it was sent, seq 0's reply finally arrives.
	clk.Add(time.Second) // epoch+2s
	fc.injectReply(pid, 0)

	m := <-out
	if got, _ := m["seq"].(int); got != 0 {
		t.Fatalf("late reply attributed to seq %v, want 0 (dropped?)", m["seq"])
	}
	if got, _ := m["status"].(string); got != "ok" {
		t.Fatalf("status = %q, want ok", got)
	}
	rtt, _ := m["rtt-ms"].(float64)
	if rtt != 2000.0 {
		t.Fatalf("rtt-ms = %v, want 2000 (arrival epoch+2s - sent epoch)", rtt)
	}
}

// TestStreamPingMixedLossOutOfOrder VALIDATES AC-2/AC-3 together: with three
// probes in flight, replies arrive out of order (seq 2 then seq 1), seq 0 is
// lost and must time out at its OWN deadline, and the send cadence stayed at
// interval throughout. This is the recovering-lossy-path the fix exists for.
func TestStreamPingMixedLossOutOfOrder(t *testing.T) {
	clk := sim.NewFakeClock(testEpoch)
	fc, out, cancel := startSession(clk, time.Second, 3*time.Second, 0)
	defer cancel()

	pid := testPID()

	sends := make([]writeRec, 3)
	sends[0] = <-fc.wrote // seq 0 at epoch
	for i := 1; i < 3; i++ {
		clk.Add(time.Second)
		clk.FireTickers()
		sends[i] = <-fc.wrote // seq i at epoch+i s
	}
	for i, r := range sends {
		if r.seq != uint16(i) {
			t.Fatalf("send %d seq = %d, want %d", i, r.seq, i)
		}
		if i > 0 {
			if gap := sends[i].at.Sub(sends[i-1].at); gap != time.Second {
				t.Fatalf("send %d spacing = %v, want 1s", i, gap)
			}
		}
	}

	// Now at epoch+2s. Reply seq 2 first (out of order), then seq 1.
	fc.injectReply(pid, 2)
	m := <-out
	if got, _ := m["seq"].(int); got != 2 {
		t.Fatalf("first resolved seq = %v, want 2 (out-of-order)", m["seq"])
	}
	if rtt, _ := m["rtt-ms"].(float64); rtt != 0.0 {
		t.Fatalf("seq 2 rtt-ms = %v, want 0 (sent and answered at epoch+2s)", rtt)
	}

	fc.injectReply(pid, 1)
	m = <-out
	if got, _ := m["seq"].(int); got != 1 {
		t.Fatalf("second resolved seq = %v, want 1", m["seq"])
	}
	if rtt, _ := m["rtt-ms"].(float64); rtt != 1000.0 {
		t.Fatalf("seq 1 rtt-ms = %v, want 1000 (sent epoch+1s, answered epoch+2s)", rtt)
	}

	// seq 0 was lost: it must time out at its own 3s deadline (epoch+3s).
	clk.Add(time.Second) // epoch+3s
	m = <-out
	if got, _ := m["seq"].(int); got != 0 {
		t.Fatalf("timed-out seq = %v, want 0", m["seq"])
	}
	if got, _ := m["status"].(string); got != "timeout" {
		t.Fatalf("seq 0 status = %q, want timeout", got)
	}
}

// TestStreamPingCountCompletesAfterLastReply VALIDATES AC-4: `count 5` ends only
// after the 5th probe RESOLVES, and every one of the 5 appears on the channel.
func TestStreamPingCountCompletesAfterLastReply(t *testing.T) {
	clk := sim.NewFakeClock(testEpoch)
	const count = 5
	fc, out, cancel := startSession(clk, time.Second, 5*time.Second, count)
	defer cancel()

	pid := testPID()

	// Drive all five sends: seq 0 immediate, the rest on ticks.
	r0 := <-fc.wrote
	if r0.seq != 0 {
		t.Fatalf("first send seq = %d, want 0", r0.seq)
	}
	for want := uint16(1); want < count; want++ {
		clk.Add(time.Second)
		clk.FireTickers()
		r := <-fc.wrote
		if r.seq != want {
			t.Fatalf("send seq = %d, want %d", r.seq, want)
		}
	}

	// Reply to every probe; each must surface on the channel.
	seen := make(map[int]bool)
	for seq := range uint16(count) {
		fc.injectReply(pid, seq)
		m := <-out
		s, _ := m["seq"].(int)
		if st, _ := m["status"].(string); st != "ok" {
			t.Fatalf("seq %d status = %q, want ok", s, st)
		}
		seen[s] = true
	}
	if len(seen) != count {
		t.Fatalf("saw %d distinct replies, want %d: %v", len(seen), count, seen)
	}

	// After the last probe resolves the session ends and closes the channel.
	if _, ok := <-out; ok {
		t.Fatal("channel not closed after count reached")
	}
}

// TestStreamPingCountNoTrailingIdle VALIDATES AC-5: with a long interval the
// session has no trailing idle: once the last probe resolves it ends without
// waiting another interval.
func TestStreamPingCountNoTrailingIdle(t *testing.T) {
	clk := sim.NewFakeClock(testEpoch)
	fc, out, cancel := startSession(clk, 30*time.Second, 5*time.Second, 1)
	defer cancel()

	pid := testPID()

	r0 := <-fc.wrote
	if r0.seq != 0 {
		t.Fatalf("first send seq = %d, want 0", r0.seq)
	}

	fc.injectReply(pid, 0)
	m := <-out
	if got, _ := m["status"].(string); got != "ok" {
		t.Fatalf("status = %q, want ok", got)
	}

	// The channel must close WITHOUT the test advancing the 30s interval.
	if _, ok := <-out; ok {
		t.Fatal("channel not closed; session idled after last reply")
	}
}

// TestStreamPingCancelClosesOnce VALIDATES AC-6/AC-7: canceling mid-flight tears
// both goroutines down, closes the channel exactly once, and leaks no goroutine.
// Run under -race for AC-7. (Reaper-goroutine teardown is covered separately by
// TestStreamPingRealClockReaperTeardown, since FakeClock spawns no reaper.)
func TestStreamPingCancelClosesOnce(t *testing.T) {
	base := runtime.NumGoroutine()

	clk := sim.NewFakeClock(testEpoch)
	fc, out, cancel := startSession(clk, time.Second, 5*time.Second, 0)

	<-fc.wrote // first probe is out; the session is live
	cancel()

	// The channel must close (draining it must terminate). A double close would
	// panic in runPingSession, so reaching here at all proves single close.
	for range out {
	}
	if _, ok := <-out; ok {
		t.Fatal("channel reopened after close")
	}

	waitGoroutines(t, base)
}

// TestStreamPingRealClockReaperTeardown VALIDATES AC-6/AC-7 for the PRODUCTION
// clock path: clock.RealClock.AfterFunc runs each timeout reaper in its own
// goroutine, so this is the only test that exercises the reaper<->done teardown
// handshake and the timer.Stop() vs live-reaper race. Run under -race.
func TestStreamPingRealClockReaperTeardown(t *testing.T) {
	base := runtime.NumGoroutine()

	rc := clock.RealClock{}
	// Short real durations: the ticker keeps sending and each probe arms a real
	// reaper goroutine that fires ~timeout later.
	fc, out, cancel := startSessionOn(rc, testDest(), testEcho, testEchoReply, 5*time.Millisecond, 20*time.Millisecond, 0)

	// The sender blocks in WriteTo until the write is consumed; drain writes so
	// probes keep flowing and reaper goroutines accumulate and fire. The drain
	// MUST keep consuming until the conn is actually closed (teardown): a probe
	// caught mid-WriteTo needs a consumer, and conn.Close (which closes fc.closed
	// and lets WriteTo return) only runs after main leaves WriteTo. Exiting the
	// drain on any earlier signal could wedge main in WriteTo -> no teardown.
	go func() {
		for {
			select {
			case <-fc.wrote:
			case <-fc.closed:
				return
			}
		}
	}()

	// No replies are injected, so the first resolution must be a real-clock
	// timeout produced by a reaper goroutine.
	m := <-out
	if got, _ := m["status"].(string); got != "timeout" {
		t.Fatalf("status = %q, want timeout (real-clock reaper)", got)
	}

	cancel()
	for range out {
	}
	if _, ok := <-out; ok {
		t.Fatal("channel reopened after close")
	}

	waitGoroutines(t, base)
}

// the id/source/v6 filter tests were rewritten (not weakened) to
// remove a false-green found in review. The old versions injected a bad reply
// then a good reply for the SAME seq with no clock advance, so accepting the bad
// reply was observationally identical to rejecting it. The two-probe ordering
// tests below FAIL if the reject branch is deleted. Per-test assertions are now
// shared via sendTwo/assertFirstOutIsSeq1, so the inline t.Fatalf count in this
// region drops even though coverage strengthened.
//
// sendTwo drives a session to two in-flight probes (seq 0 immediate, seq 1 on a
// tick) so a filter test can inject a BAD reply for seq 0 and a GOOD reply for
// seq 1: because the receiver is serial, if the reject branch were removed the
// bad seq-0 reply would surface on `out` BEFORE seq 1. Asserting the first
// emitted reply is seq 1 therefore fails if the filter is deleted (no
// false-green, and no clock/reaper race).
func sendTwo(t *testing.T, clk *sim.FakeClock, fc *fakePingConn) {
	t.Helper()
	if r := <-fc.wrote; r.seq != 0 {
		t.Fatalf("first send seq = %d, want 0", r.seq)
	}
	clk.Add(time.Second)
	clk.FireTickers()
	if r := <-fc.wrote; r.seq != 1 {
		t.Fatalf("second send seq = %d, want 1", r.seq)
	}
}

func assertFirstOutIsSeq1(t *testing.T, out <-chan map[string]any, what string) {
	t.Helper()
	m := <-out
	if got, _ := m["seq"].(int); got != 1 {
		t.Fatalf("first emitted reply seq = %v, want 1 (%s leaked through)", m["seq"], what)
	}
	if got, _ := m["status"].(string); got != "ok" {
		t.Fatalf("status = %q, want ok", got)
	}
}

// TestStreamPingIDMismatchIgnored VALIDATES the preserved id filter (AC-8 /
// security checklist): a reply whose ICMP identifier is not ours is dropped.
// Deleting the id check would make the wrong-id seq-0 reply resolve seq 0 and
// surface before seq 1, failing this test.
func TestStreamPingIDMismatchIgnored(t *testing.T) {
	clk := sim.NewFakeClock(testEpoch)
	fc, out, cancel := startSession(clk, time.Second, 10*time.Second, 0)
	defer cancel()

	pid := testPID()
	sendTwo(t, clk, fc)

	fc.inject(testEchoReply, pid^0x1, 0, nil) // wrong identifier for seq 0: must drop
	fc.injectReply(pid, 1)                    // good reply for seq 1
	assertFirstOutIsSeq1(t, out, "wrong-id reply")
}

// TestStreamPingWrongSourceIgnored VALIDATES the preserved source-address filter
// (spec behavior-to-preserve, line 79): a reply from an address other than the
// target is ignored. Deleting the source check would make the wrong-source seq-0
// reply resolve seq 0 and surface before seq 1.
func TestStreamPingWrongSourceIgnored(t *testing.T) {
	clk := sim.NewFakeClock(testEpoch)
	fc, out, cancel := startSession(clk, time.Second, 10*time.Second, 0)
	defer cancel()

	pid := testPID()
	sendTwo(t, clk, fc)

	// Correct id+seq for seq 0 but from the wrong host: dropped by the source check.
	fc.inject(testEchoReply, pid, 0, &net.IPAddr{IP: net.ParseIP("198.51.100.9")})
	fc.injectReply(pid, 1) // good reply for seq 1 from the target (nil source)
	assertFirstOutIsSeq1(t, out, "wrong-source reply")
}

// TestStreamPingIPv6Matching VALIDATES that the session matches ICMPv6 echo
// replies (type 129) against a v6 target AND rejects a v4-type (0) reply.
// Deleting the type check would make the type-0 seq-0 reply resolve seq 0 and
// surface before the type-129 seq-1 reply.
func TestStreamPingIPv6Matching(t *testing.T) {
	clk := sim.NewFakeClock(testEpoch)
	dest := netip.MustParseAddr("2001:db8::1")
	fc, out, cancel := startSessionOn(clk, dest, v6Echo, v6EchoReply, time.Second, 10*time.Second, 0)
	defer cancel()

	pid := testPID()
	sendTwo(t, clk, fc)

	fc.inject(testEchoReply, pid, 0, nil) // v4-type (0) reply for seq 0: rejected by v6 type check
	fc.inject(v6EchoReply, pid, 1, nil)   // correct v6 echo reply (type 129) for seq 1
	assertFirstOutIsSeq1(t, out, "v4-type reply")
}

// TestStreamPingWriteErrorEndsSession VALIDATES the write-error unwind
// (stream.go send()): a failing WriteTo stops sending, and with nothing left in
// flight the unbounded session ends and closes the channel.
func TestStreamPingWriteErrorEndsSession(t *testing.T) {
	clk := sim.NewFakeClock(testEpoch)
	fc := newFakePingConn(clk)
	fc.setWriteErr(errors.New("write failed"))

	out := make(chan map[string]any, 64)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go runPingSession(ctx, fc, clk, testDest(), time.Second, 5*time.Second, 0, 0, testEcho, testEchoReply, out)

	// The first send fails; nothing is in flight, so the session ends at once
	// and closes the channel without emitting a probe result.
	if m, ok := <-out; ok {
		t.Fatalf("unexpected result after write error: %v", m)
	}
}

// TestStreamPingDuplicateAndUnknownReplyIgnored VALIDATES AC-3 hardening: a
// duplicate reply for an already-resolved probe is dropped and does not
// double-report, and a reply for an unknown seq is ignored.
func TestStreamPingDuplicateAndUnknownReplyIgnored(t *testing.T) {
	clk := sim.NewFakeClock(testEpoch)
	fc, out, cancel := startSession(clk, time.Second, 10*time.Second, 0)
	defer cancel()

	pid := testPID()

	<-fc.wrote // seq 0
	fc.injectReply(pid, 0)
	m := <-out
	if got, _ := m["seq"].(int); got != 0 {
		t.Fatalf("first reply seq = %v, want 0", m["seq"])
	}

	// A duplicate for seq 0 and a reply for a never-sent seq 99 must both be
	// dropped: the next thing on the channel is the seq 1 reply, nothing else.
	fc.injectReply(pid, 0)
	fc.inject(testEchoReply, pid, 99, nil)

	clk.Add(time.Second)
	clk.FireTickers()
	<-fc.wrote // seq 1
	fc.injectReply(pid, 1)

	m = <-out
	if got, _ := m["seq"].(int); got != 1 {
		t.Fatalf("next reply seq = %v, want 1 (duplicate/unknown leaked?)", m["seq"])
	}
}

// waitGoroutines waits (bounded) for the goroutine count to return to base, the
// repo convention for a leak assertion (a genuine hang is caught by the
// deadline). The receiver is joined before out closes, so once the channel is
// observed closed only the main goroutine (and, on the real clock, spent reaper
// goroutines) remain to unwind.
func waitGoroutines(t *testing.T, base int) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for runtime.NumGoroutine() > base {
		select {
		case <-deadline:
			t.Fatalf("goroutine leak: %d > baseline %d", runtime.NumGoroutine(), base)
		default:
			time.Sleep(time.Millisecond)
		}
	}
}
