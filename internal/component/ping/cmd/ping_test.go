// Design: plan/learned/664-diag-5-active-probes.md -- ping argument parsing tests

package cmd

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/test/sim"
)

func TestPingParseArgsValid(t *testing.T) {
	dest, count, timeout, opts, err := parsePingArgs([]string{"127.0.0.1"})
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1", dest.String())
	assert.Equal(t, defaultPingCount, count)
	assert.Equal(t, defaultPingTimeout, timeout)
	assert.Equal(t, 0, opts.size, "size unset means the engine default")
}

func TestPingParseArgsWithCount(t *testing.T) {
	dest, count, _, _, err := parsePingArgs([]string{"10.0.0.1", "count", "3"})
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.1", dest.String())
	assert.Equal(t, 3, count)
}

func TestPingParseArgsWithTimeout(t *testing.T) {
	_, _, timeout, _, err := parsePingArgs([]string{"10.0.0.1", "timeout", "2s"})
	require.NoError(t, err)
	assert.Equal(t, 2*time.Second, timeout)
}

// TestPingParseArgsWithSize verifies `show ping <dest> size <bytes>` reaches opts.
//
// VALIDATES: the size argument parses and lands in pingOpts, which doPing uses
// to build the ICMP payload.
// PREVENTS: the web Ping tool's Packet Size field being silently dropped.
// handleShowPing used to pass a zero pingOpts and parsePingArgs had no size
// case, so an operator-chosen size never changed the packet.
func TestPingParseArgsWithSize(t *testing.T) {
	dest, _, _, opts, err := parsePingArgs([]string{"10.0.0.1", "size", "1400"})
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.1", dest.String())
	assert.Equal(t, 1400, opts.size)
}

// TestPingParseArgsSizeBounds pins the accepted size range.
//
// VALIDATES: 1 and maxPingSize are accepted; 0 and maxPingSize+1 are rejected.
// PREVENTS: a size that cannot fit an IP datagram reaching the ICMP engine, and
// drift from the range on the show/ping size leaf in ze-ping-cmd.yang.
func TestPingParseArgsSizeBounds(t *testing.T) {
	cases := []struct {
		name    string
		size    string
		want    int
		wantErr bool
	}{
		{name: "minimum", size: "1", want: 1},
		{name: "maximum", size: "65507", want: maxPingSize},
		{name: "zero", size: "0", wantErr: true},
		{name: "above maximum", size: "65508", wantErr: true},
		{name: "not a number", size: "big", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, opts, err := parsePingArgs([]string{"127.0.0.1", "size", tc.size})
			if tc.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "size must be")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, opts.size)
		})
	}
}

// TestPingParseArgsSizeMissingValue verifies a trailing `size` is rejected.
//
// VALIDATES: `size` without a value errors instead of being ignored.
// PREVENTS: silently pinging with the default payload when the operator asked
// for a specific size.
func TestPingParseArgsSizeMissingValue(t *testing.T) {
	_, _, _, _, err := parsePingArgs([]string{"127.0.0.1", "size"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "size requires a value")
}

// TestMonitorPingParseArgsInterval verifies `monitor ping` honors interval.
//
// VALIDATES: interval reaches the caller; the default applies when omitted.
// PREVENTS: the silent default this fixes. monitorPingLocal used parsePingArgs,
// which has no interval case, so `monitor ping <dest> interval 500ms` fell
// through to the destination branch and streamed at a hardcoded 1s -- while
// docs/guide/command-reference.md advertised the flag as working.
func TestMonitorPingParseArgsInterval(t *testing.T) {
	mp, err := parseMonitorPingArgs([]string{"10.0.0.1", "interval", "500ms"})
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.1", mp.Dest.String())
	assert.Equal(t, 500*time.Millisecond, mp.Interval)
	assert.Equal(t, defaultPingTimeout, mp.Timeout)

	mp, err = parseMonitorPingArgs([]string{"10.0.0.1"})
	require.NoError(t, err)
	assert.Equal(t, defaultPingMonitorInterval, mp.Interval, "omitted interval uses the default")
	assert.Equal(t, 0, mp.Count, "omitted count streams until interrupted")
	assert.Equal(t, 0, mp.Size, "omitted size uses the engine default payload")
}

// TestMonitorPingParseArgsCountAndSize verifies both reach the caller.
//
// VALIDATES: `monitor ping <dest> count 5 size 1400` parses both (AC: the
// streaming session bounds its probes and carries the payload).
// PREVENTS: the trap this fixes -- monitorPingLocal parsed both via the shared
// parsePingArgs and then discarded them, so an explicit request silently did
// nothing. Accept-and-ignore is banned by
// ai/rules/no-workarounds-for-missing-behavior.md.
func TestMonitorPingParseArgsCountAndSize(t *testing.T) {
	mp, err := parseMonitorPingArgs([]string{"10.0.0.1", "count", "5", "size", "1400"})
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.1", mp.Dest.String())
	assert.Equal(t, 5, mp.Count)
	assert.Equal(t, 1400, mp.Size)
}

// TestMonitorPingParseArgsBounds pins the accepted ranges.
//
// VALIDATES: interval 100ms-30s, count 1-100, size 1-65507; each rejects
// outside its range.
// PREVENTS: drift from the interactive CLI's own bounds (model_ping.go
// parsePingMonitorArgs), which must agree so `monitor ping` behaves the same
// with and without a daemon.
func TestMonitorPingParseArgsBounds(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "interval minimum", args: []string{"interval", "100ms"}},
		{name: "interval maximum", args: []string{"interval", "30s"}},
		{name: "interval below", args: []string{"interval", "99ms"}, wantErr: "interval must be"},
		{name: "interval above", args: []string{"interval", "31s"}, wantErr: "interval must be"},
		{name: "interval not a duration", args: []string{"interval", "soon"}, wantErr: "interval must be"},
		{name: "count minimum", args: []string{"count", "1"}},
		{name: "count maximum", args: []string{"count", "100"}},
		{name: "count zero", args: []string{"count", "0"}, wantErr: "count must be"},
		{name: "count above", args: []string{"count", "101"}, wantErr: "count must be"},
		{name: "size minimum", args: []string{"size", "1"}},
		{name: "size maximum", args: []string{"size", "65507"}},
		{name: "size zero", args: []string{"size", "0"}, wantErr: "size must be"},
		{name: "size above", args: []string{"size", "65508"}, wantErr: "size must be"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseMonitorPingArgs(append([]string{"127.0.0.1"}, tc.args...))
			if tc.wantErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

// TestMonitorPingParseArgsErrors covers the remaining rejection paths.
//
// VALIDATES: missing destination, a trailing keyword, and a second positional
// argument all error.
// PREVENTS: a trailing keyword or a stray token being silently swallowed.
func TestMonitorPingParseArgsErrors(t *testing.T) {
	_, err := parseMonitorPingArgs([]string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing destination")

	for _, arg := range []string{"interval", "timeout", "count", "size"} {
		_, err = parseMonitorPingArgs([]string{"127.0.0.1", arg})
		assert.Error(t, err, "trailing %s must error", arg)
		assert.Contains(t, err.Error(), "requires a value")
	}

	_, err = parseMonitorPingArgs([]string{"127.0.0.1", "8.8.8.8"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected argument")
}

// TestPingPayload verifies the bytes both ping engines put on the wire.
//
// VALIDATES: size 0 sends the default marker; size N sends exactly N bytes with
// the marker at the front.
// PREVENTS: `size N` reaching the parser but not the packet. This is the one
// part of the size path that cannot be driven end-to-end here -- raw ICMP needs
// CAP_NET_RAW, so doPing/streamPing cannot open a socket unprivileged -- and it
// is shared by both, so a regression would silently affect show ping and
// monitor ping together.
func TestPingPayload(t *testing.T) {
	assert.Equal(t, []byte("ze-ping"), pingPayload(0), "size 0 uses the default marker")
	assert.Equal(t, []byte("ze-ping"), pingPayload(-1), "negative size is treated as unset")

	p := pingPayload(1400)
	assert.Len(t, p, 1400, "payload is exactly the requested size")
	assert.Equal(t, []byte("ze-ping"), p[:7], "marker is copied to the front")
	assert.Equal(t, make([]byte, 1393), p[7:], "remainder is zero-filled")

	assert.Len(t, pingPayload(1), 1, "a 1-byte payload truncates the marker")
	assert.Len(t, pingPayload(maxPingSize), maxPingSize)
}

func TestPingParseArgsMissingDest(t *testing.T) {
	_, _, _, _, err := parsePingArgs([]string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing destination")
}

func TestPingParseArgsInvalidDest(t *testing.T) {
	_, _, _, _, err := parsePingArgs([]string{"not-an-ip"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid destination")
}

func TestPingParseArgsCountTooHigh(t *testing.T) {
	_, _, _, _, err := parsePingArgs([]string{"127.0.0.1", "count", "200"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "count must be")
}

func TestPingParseArgsCountZero(t *testing.T) {
	_, _, _, _, err := parsePingArgs([]string{"127.0.0.1", "count", "0"})
	assert.Error(t, err)
}

func TestPingParseArgsTimeoutTooHigh(t *testing.T) {
	_, _, _, _, err := parsePingArgs([]string{"127.0.0.1", "timeout", "60s"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout must be")
}

func TestPingParseArgsIPv6(t *testing.T) {
	dest, _, _, _, err := parsePingArgs([]string{"::1"})
	require.NoError(t, err)
	assert.True(t, dest.Is6())
}

// PREVENTS: garbage targets with shell metacharacters reaching DNS resolution.
func TestPingParseArgsShellMeta(t *testing.T) {
	bad := []string{
		"a;rm -rf /",
		"$(echo x)",
		"`id`",
		"host|cat",
		"host with space",
	}
	for _, target := range bad {
		t.Run(target, func(t *testing.T) {
			_, _, _, _, err := parsePingArgs([]string{target})
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid destination")
		})
	}
}

// --- show ping batch engine (doPingCtx -> runPingBatch) ---------------------
//
// These tests drive runPingBatch through the same fake pingConn + fake clock the
// streaming session tests use (fakePingConn, sim.NewFakeClock, defined in
// stream_test.go). They exercise the decoupled batch: paced sends, seq-keyed
// matching, per-probe timeouts. The old serial doPingCtx sent probe seq+1 only
// after seq's own reply-or-deadline, so a black-holed batch serialized to
// count*timeout (a `show ping count 100 timeout 30s` run took ~50 minutes) and a
// reply arriving after the next probe was dropped. No test could catch that: the
// serial engine opened a raw ICMP socket (CAP_NET_RAW), unreachable in a unit
// test -- which is why the bug survived. spec-fixit-show-ping-serial-pacing.

// startBatch launches runPingBatch on a v4 target with a fresh fake conn and the
// given clock, returning the conn to drive and a channel that yields the final
// result map when the batch completes.
func startBatch(t *testing.T, clk clock.Clock, interval, timeout time.Duration, count int) (*fakePingConn, chan map[string]any) {
	t.Helper()
	fc := newFakePingConn(clk)
	done := make(chan map[string]any, 1)
	go func() {
		res, err := runPingBatch(context.Background(), fc, clk, testDest(), interval, timeout, count, 0, testEcho, testEchoReply)
		// Every batch here resolves at least one probe, so any error is a real
		// regression; assert (goroutine-safe) and let res drive the caller.
		assert.NoError(t, err)
		done <- res
	}()
	return fc, done
}

// driveSends consumes the first `count` paced sends: seq 0 goes out immediately,
// the rest on ticker ticks one interval apart, asserting each seq in order. It
// leaves every probe in flight (none answered), the batch's starting state.
func driveSends(t *testing.T, fc *fakePingConn, clk *sim.FakeClock, interval time.Duration, count int) {
	t.Helper()
	r0 := <-fc.wrote
	if r0.seq != 0 {
		t.Fatalf("first send seq = %d, want 0", r0.seq)
	}
	for want := uint16(1); want < uint16(count); want++ {
		clk.Add(interval)
		clk.FireTickers()
		r := <-fc.wrote
		if r.seq != want {
			t.Fatalf("send seq = %d, want %d", r.seq, want)
		}
	}
}

func repliesBySeq(t *testing.T, res map[string]any) map[int]map[string]any {
	t.Helper()
	raw, ok := res["replies"].([]map[string]any)
	if !ok {
		t.Fatalf("replies is %T, want []map[string]any (offline.go asserts this type)", res["replies"])
	}
	bySeq := make(map[int]map[string]any, len(raw))
	for _, r := range raw {
		seq, _ := r["seq"].(int)
		bySeq[seq] = r
	}
	return bySeq
}

// TestDoPingBatchHealthyShape VALIDATES AC-4 and AC-5: a healthy batch returns
// every probe ok with its RTT and the exact result-map shape the serial engine
// produced, and it completes as soon as the last reply lands -- it never waits on
// a timeout. Sends are paced (interval apart), so seq k's RTT is (last send -
// send k): 40/30/20/10/0 ms for a 5-probe, 10ms-paced batch answered at once.
func TestDoPingBatchHealthyShape(t *testing.T) {
	clk := sim.NewFakeClock(testEpoch)
	const count = 5
	const interval = 10 * time.Millisecond
	fc, done := startBatch(t, clk, interval, 5*time.Second, count)
	pid := testPID()

	driveSends(t, fc, clk, interval, count) // now at epoch + 40ms; all 5 in flight

	// Answer every probe at the current instant. None has hit its 5s deadline, so
	// the batch must finish on the replies alone.
	for seq := range uint16(count) {
		fc.injectReply(pid, seq)
	}

	res := <-done

	assert.Equal(t, testDest().String(), res["destination"])
	assert.Equal(t, count, res["sent"], "every paced probe reached the wire")
	assert.Equal(t, count, res["received"])
	assert.Equal(t, 0.0, res["loss-percent"])

	replies, ok := res["replies"].([]map[string]any)
	require.True(t, ok, "replies must be []map[string]any for offline.go")
	require.Len(t, replies, count)
	// Sorted ascending by seq (deterministic order, matching the old serial run).
	for i, r := range replies {
		assert.Equal(t, i, r["seq"], "replies must be seq-ordered")
		assert.Equal(t, "ok", r["status"])
		assert.Contains(t, r, "rtt-ms")
	}
	assert.Equal(t, 0.0, res["min-rtt-ms"], "seq 4 answered at its own send instant")
	assert.Equal(t, 40.0, res["max-rtt-ms"], "seq 0 waited the full 40ms of pacing")
	assert.Equal(t, 20.0, res["avg-rtt-ms"], "(40+30+20+10+0)/5")
}

// TestDoPingBatchBoundedUnderLoss VALIDATES AC-1/AC-2/AC-5 together: with a mix
// of answered and lost probes, every probe is sent while earlier ones are still
// in flight (proving sends do not block on replies), the lost probes report
// timeout, the answered ones report ok, and the map shape holds. The boundedness
// is structural: all 5 probes are on the wire within 40ms of pacing and the
// three lost ones all expire from a SINGLE clock advance -- they were in flight
// simultaneously, not serialized at timeout each (which is what turned the
// black-hole case into count*timeout, ~50 minutes at count 100 timeout 30s).
//
// The answered probes are the two LATEST-sent (seq 3, 4): their deadlines
// (epoch+5030ms, +5040ms) sit beyond the advance, so their reapers cannot fire
// during it and their ok status is deterministic. Exact RTT values are asserted
// in TestDoPingBatchHealthyShape (stable clock) and TestSummarizePingReplies
// (pure), not here, because the injected reply's arrival timestamp races the
// clock advance -- a test artifact, not a production race.
func TestDoPingBatchBoundedUnderLoss(t *testing.T) {
	clk := sim.NewFakeClock(testEpoch)
	const count = 5
	const interval = 10 * time.Millisecond
	const timeout = 5 * time.Second
	fc, done := startBatch(t, clk, interval, timeout, count)
	pid := testPID()

	driveSends(t, fc, clk, interval, count) // all 5 in flight at epoch + 40ms

	// Answer seq 3 and 4 (deadlines beyond the advance below).
	fc.injectReply(pid, 3)
	fc.injectReply(pid, 4)

	// Advance to epoch+5025ms (40ms of pacing + 4985ms): a SINGLE advance past
	// seq 0/1/2's deadlines (5000/5010/5020ms) but short of seq 3/4's
	// (5030/5040ms). If the engine were serial, seq 1 could not even have been
	// sent while seq 0 was unanswered, so reaching here with all five in flight
	// and three expiring at once is exactly the fix.
	clk.Add(4985 * time.Millisecond)

	res := <-done

	assert.Equal(t, count, res["sent"], "every paced probe reached the wire")
	assert.Equal(t, 2, res["received"])
	assert.Equal(t, 60.0, res["loss-percent"], "3 of 5 lost")

	bySeq := repliesBySeq(t, res)
	require.Len(t, bySeq, count)
	for _, seq := range []int{3, 4} {
		assert.Equal(t, "ok", bySeq[seq]["status"], "seq %d answered", seq)
		assert.Contains(t, bySeq[seq], "rtt-ms")
	}
	for _, seq := range []int{0, 1, 2} {
		assert.Equal(t, "timeout", bySeq[seq]["status"], "seq %d black-holed", seq)
		assert.NotContains(t, bySeq[seq], "rtt-ms", "a timeout carries no rtt")
	}
}

// TestSummarizePingReplies VALIDATES AC-5: the aggregate the batch reports
// (sent/received/loss-percent and the min/avg/max summary) is exactly what the
// serial engine produced, driven purely from a collected replies slice with no
// clock or goroutines. It pins the loss math and that min/avg/max appear only
// when at least one probe was answered.
func TestSummarizePingReplies(t *testing.T) {
	dest := testDest()

	t.Run("mixed loss", func(t *testing.T) {
		replies := []map[string]any{
			{"seq": 0, "status": "ok", "rtt-ms": 40.0},
			{"seq": 1, "status": "timeout"},
			{"seq": 2, "status": "ok", "rtt-ms": 20.0},
			{"seq": 3, "status": "timeout"},
			{"seq": 4, "status": "ok", "rtt-ms": 0.0},
		}
		res := summarizePingReplies(dest, replies)
		assert.Equal(t, dest.String(), res["destination"])
		assert.Equal(t, 5, res["sent"])
		assert.Equal(t, 3, res["received"])
		assert.Equal(t, 40.0, res["loss-percent"], "(5-3)/5*100")
		assert.Equal(t, 0.0, res["min-rtt-ms"])
		assert.Equal(t, 40.0, res["max-rtt-ms"])
		assert.Equal(t, 20.0, res["avg-rtt-ms"], "(40+20+0)/3")
	})

	t.Run("all lost has no rtt summary", func(t *testing.T) {
		replies := []map[string]any{
			{"seq": 0, "status": "timeout"},
			{"seq": 1, "status": "timeout"},
		}
		res := summarizePingReplies(dest, replies)
		assert.Equal(t, 2, res["sent"])
		assert.Equal(t, 0, res["received"])
		assert.Equal(t, 100.0, res["loss-percent"])
		assert.NotContains(t, res, "min-rtt-ms")
		assert.NotContains(t, res, "avg-rtt-ms")
		assert.NotContains(t, res, "max-rtt-ms")
	})
}

// TestDoPingMatchesLateReply VALIDATES AC-3: a reply for seq 0 that arrives after
// seq 1 was already sent is attributed to seq 0 with its true RTT, not dropped.
// The old serial loop had stopped listening for seq 0 once it moved on, so a late
// reply was lost. interval/timeout are round here for clean RTT arithmetic.
func TestDoPingMatchesLateReply(t *testing.T) {
	clk := sim.NewFakeClock(testEpoch)
	const count = 2
	const interval = time.Second
	fc, done := startBatch(t, clk, interval, 30*time.Second, count)
	pid := testPID()

	driveSends(t, fc, clk, interval, count) // seq 0 at epoch, seq 1 at epoch+1s

	// A full 2s after it was sent, seq 0's reply finally arrives (well after
	// seq 1 went out). Then seq 1 answers, completing the batch.
	clk.Add(time.Second) // epoch + 2s
	fc.injectReply(pid, 0)
	fc.injectReply(pid, 1)

	res := <-done

	assert.Equal(t, count, res["sent"])
	assert.Equal(t, count, res["received"], "the late reply was attributed, not dropped")
	bySeq := repliesBySeq(t, res)
	assert.Equal(t, "ok", bySeq[0]["status"])
	assert.Equal(t, 2000.0, bySeq[0]["rtt-ms"], "seq 0: answered epoch+2s, sent epoch")
	assert.Equal(t, "ok", bySeq[1]["status"])
	assert.Equal(t, 1000.0, bySeq[1]["rtt-ms"], "seq 1: answered epoch+2s, sent epoch+1s")
}

// TestDoPingAllLostBounded VALIDATES the AC-2 worst case: a fully black-holed
// batch (the `show ping <blackhole> count 100 timeout 30s` scenario) still ends.
// Every probe is sent on cadence, each times out at its own deadline, and the run
// terminates -- it does not hang or serialize to count*timeout. received is 0, so
// no min/avg/max keys appear, exactly as the serial engine did with zero replies.
func TestDoPingAllLostBounded(t *testing.T) {
	clk := sim.NewFakeClock(testEpoch)
	const count = 3
	const interval = time.Second
	const timeout = 3 * time.Second
	fc, done := startBatch(t, clk, interval, timeout, count)

	driveSends(t, fc, clk, interval, count) // sends at epoch, +1s, +2s; none answered

	// Advance past the last probe's deadline (epoch+2s + 3s). All three reapers
	// fire; the batch drains and returns.
	clk.Add(timeout)

	res := <-done

	assert.Equal(t, count, res["sent"])
	assert.Equal(t, 0, res["received"])
	assert.Equal(t, 100.0, res["loss-percent"])
	assert.NotContains(t, res, "min-rtt-ms", "no replies means no rtt summary")
	assert.NotContains(t, res, "avg-rtt-ms")
	assert.NotContains(t, res, "max-rtt-ms")
	bySeq := repliesBySeq(t, res)
	require.Len(t, bySeq, count)
	for seq := range count {
		assert.Equal(t, "timeout", bySeq[seq]["status"], "seq %d", seq)
	}
}

// TestRunPingBatchCountZeroDoesNotHang VALIDATES the fail-closed guard: a count of
// zero (or below) must not reach runPingSession, whose count==0 contract means
// "stream until canceled" -- for a bounded batch that would be an unbounded hang.
// runPingBatch returns the empty, well-formed result immediately and closes the
// socket it was handed. It runs synchronously (no goroutine), so a hang would
// surface as a test timeout.
func TestRunPingBatchCountZeroDoesNotHang(t *testing.T) {
	clk := sim.NewFakeClock(testEpoch)
	fc := newFakePingConn(clk)

	res, err := runPingBatch(context.Background(), fc, clk, testDest(), 10*time.Millisecond, 5*time.Second, 0, 0, testEcho, testEchoReply)

	// count<=0 is the "nothing requested" guard, not a runtime failure: it returns
	// the well-formed empty map with no error (distinct from a count>0 batch that
	// tried and failed to send, which errors -- see TestRunPingBatchSendErrorFailsClosed).
	require.NoError(t, err)
	assert.Equal(t, testDest().String(), res["destination"])
	assert.Equal(t, 0, res["sent"])
	assert.Equal(t, 0, res["received"])
	replies, ok := res["replies"].([]map[string]any)
	require.True(t, ok, "empty batch still returns []map[string]any for offline.go")
	assert.Empty(t, replies)

	select {
	case <-fc.closed:
	default:
		t.Fatal("runPingBatch must close the socket it was handed, even for an empty batch")
	}
}

// TestRunPingBatchSendErrorFailsClosed VALIDATES the fail-closed guard against a
// total send failure: when the first (and every) WriteTo fails -- ENETUNREACH on
// a missing route, EPERM without CAP_NET_RAW -- no probe reaches the wire, so
// runPingSession emits nothing. runPingBatch must NOT summarize that empty result
// as a healthy sent=0/received=0/loss-percent=0 map (the fail-open pattern
// ai/rules/fail-closed-guards.md forbids: a transport failure rendered as a
// valid-looking 0%-loss answer). It returns errPingNoProbesSent instead, matching
// the serial engine's StatusError-on-write. The socket is still closed.
func TestRunPingBatchSendErrorFailsClosed(t *testing.T) {
	clk := sim.NewFakeClock(testEpoch)
	fc := newFakePingConn(clk)
	fc.setWriteErr(errors.New("network is unreachable"))

	res, err := runPingBatch(context.Background(), fc, clk, testDest(), 10*time.Millisecond, 5*time.Second, 5, 0, testEcho, testEchoReply)

	require.Error(t, err, "a batch that put no probe on the wire must not report success")
	assert.ErrorIs(t, err, errPingNoProbesSent)
	assert.Nil(t, res, "no misleading 0%-loss map on total send failure")

	select {
	case <-fc.closed:
	default:
		t.Fatal("runPingBatch must close the socket even when every send fails")
	}
}
