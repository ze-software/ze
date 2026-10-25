// Design: docs/architecture/forward-congestion-pool.md -- read pool exhaustion is backpressure
// Related: session_read.go -- awaitReadBuffer
// Related: bufmux.go -- bufWake

package reactor

import (
	"bufio"
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/test/sim"
)

// exhaustStdReadPool caps the shared byte budget at what the 4K pool already
// holds and takes every free buffer, so the next bufMuxStd.Get answers the
// zero handle. It returns release, which returns the held buffers and
// restores the budget once; the cleanup it registers calls it too. The caller
// MUST hold bufMuxGlobalMu.
func exhaustStdReadPool(t *testing.T) (release func()) {
	t.Helper()
	if bufMuxStd.mux.budget == nil {
		// No reactor started in this process yet. A reactor start wires the
		// same unlimited budget (initBufMuxBudget), which is what this cap
		// then lowers.
		initBufMuxBudget(0)
	}
	budget := bufMuxStd.mux.budget
	previous := budget.maxBytes.Load()
	// One Get first: a pool with no block has allocated nothing, and a cap of
	// zero bytes means unlimited (tryReserve).
	held := []BufHandle{bufMuxStd.Get()}
	if held[0].Buf == nil {
		t.Fatal("bufMuxStd refused the first buffer")
	}
	updateBufMuxBudget(budget.allocatedBytes())
	for {
		h := bufMuxStd.Get()
		if h.Buf == nil {
			break
		}
		held = append(held, h)
	}
	var once sync.Once
	release = func() {
		once.Do(func() {
			for _, h := range held {
				bufMuxStd.Return(h)
			}
			updateBufMuxBudget(previous)
		})
	}
	t.Cleanup(release)
	return release
}

// TestSessionReadWaitsForBufferWhenPoolExhausted proves a read pool at its
// byte budget is backpressure, not a session failure: Run stops reading (the
// peer's write stays blocked on the pipe), stays running, and reads the
// message once a buffer is returned. Method: hold every 4K buffer under a
// budget capped at the current allocation, start Run, write a KEEPALIVE,
// then return the buffers.
//
// VALIDATES: a received message waits for a read buffer instead of being lost
// with its session.
// PREVENTS: Run returning errReadBufferExhaustedPoolAtMaximum, which reset a
// healthy source session with no NOTIFICATION while a route server's recovery
// held the pool (TestOwedKeyLookupChunked under ze.fwd.pool.maxbytes=524288).
func TestSessionReadWaitsForBufferWhenPoolExhausted(t *testing.T) {
	bufMuxGlobalMu.Lock()
	defer bufMuxGlobalMu.Unlock()

	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	session := NewSession(settings)
	server, client := net.Pipe()
	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})
	session.mu.Lock()
	session.conn = server
	session.bufReader = bufio.NewReaderSize(server, 65536)
	session.bufWriter = bufio.NewWriterSize(server, 16384)
	session.mu.Unlock()

	release := exhaustStdReadPool(t)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- session.Run(ctx) }()

	written := make(chan error, 1)
	go func() {
		_, err := client.Write(message.PackTo(message.NewKeepalive(), nil))
		written <- err
	}()

	select {
	case err := <-result:
		t.Fatalf("Run returned %v while the read pool was exhausted, want it to wait for a buffer", err)
	case err := <-written:
		t.Fatalf("the KEEPALIVE was read (write err %v) with no read buffer free", err)
	case <-time.After(300 * time.Millisecond):
	}

	// Return the buffers: the wait wakes and the message is read.
	release()

	select {
	case err := <-written:
		if err != nil {
			t.Fatalf("KEEPALIVE write: %v", err)
		}
	case err := <-result:
		t.Fatalf("Run returned %v before reading the KEEPALIVE", err)
	case <-time.After(5 * time.Second):
		t.Fatal("the KEEPALIVE was not read after the read buffers were returned")
	}

	cancel()
	select {
	case <-result:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}

// TestSessionReadWaitEndsOnClose proves a read loop waiting for a buffer leaves
// when the session closes, so a pool that never refills cannot hold Run.
// Method: exhaust the 4K pool, start Run, cancel its context.
//
// VALIDATES: awaitReadBuffer's stop and ctx exits.
// PREVENTS: a session goroutine parked forever on an empty pool.
func TestSessionReadWaitEndsOnClose(t *testing.T) {
	bufMuxGlobalMu.Lock()
	defer bufMuxGlobalMu.Unlock()

	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	session := NewSession(settings)
	server, client := net.Pipe()
	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})
	session.mu.Lock()
	session.conn = server
	session.bufReader = bufio.NewReaderSize(server, 65536)
	session.bufWriter = bufio.NewWriterSize(server, 16384)
	session.mu.Unlock()

	exhaustStdReadPool(t)

	ctx, cancel := context.WithCancel(t.Context())
	result := make(chan error, 1)
	go func() { result <- session.Run(ctx) }()

	select {
	case err := <-result:
		t.Fatalf("Run returned %v while the read pool was exhausted, want it to wait", err)
	case <-time.After(200 * time.Millisecond):
	}
	cancel()
	select {
	case <-result:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after cancel while waiting for a read buffer")
	}
}

// bufferWaitSession builds a session at addr over a net.Pipe whose server end
// the session reads, with clk as its clock when clk is non-nil. It returns the
// session and the client end, the peer's side of the connection.
func bufferWaitSession(t *testing.T, addr string, clk *sim.FakeClock) (*Session, net.Conn) {
	t.Helper()
	settings := NewPeerSettings(netip.MustParseAddr(addr), 65001, 65002, 0x01020301)
	session := NewSession(settings)
	if clk != nil {
		session.SetClock(clk)
	}
	server, client := net.Pipe()
	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})
	session.mu.Lock()
	session.conn = server
	session.bufReader = bufio.NewReaderSize(server, 65536)
	session.bufWriter = bufio.NewWriterSize(server, 16384)
	session.mu.Unlock()
	return session, client
}

// readNotificationCodes reads one BGP message from conn and answers its
// NOTIFICATION error code and subcode, failing the test on anything else.
func readNotificationCodes(t *testing.T, conn net.Conn) (code, subcode uint8) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	header := make([]byte, message.HeaderLen)
	if _, err := io.ReadFull(conn, header); err != nil {
		t.Fatalf("no message reached the peer: %v", err)
	}
	if header[18] != byte(msgtype.TypeNOTIFICATION) {
		t.Fatalf("the peer received message type %d, want NOTIFICATION", header[18])
	}
	length := int(binary.BigEndian.Uint16(header[16:18]))
	body := make([]byte, length-message.HeaderLen)
	if _, err := io.ReadFull(conn, body); err != nil {
		t.Fatalf("NOTIFICATION body: %v", err)
	}
	return body[0], body[1]
}

// awaitReadPause installs a log sink as the session logger and answers a
// function that blocks until the read loop of the session at addr logged that
// it paused for a read buffer, failing the test after 5 s. The fake clock
// cannot fence the wait: its NewTimer never fires and arms nothing that
// AwaitTimers counts, so the pause WARN is the fence.
func awaitReadPause(t *testing.T, addr string) func() {
	t.Helper()
	sink := &syncBuffer{}
	lg := slog.New(slog.NewTextHandler(sink, &slog.HandlerOptions{Level: slog.LevelDebug}))
	t.Cleanup(swapSessionLogger(func() *slog.Logger { return lg }))
	return func() {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for {
			for line := range strings.SplitSeq(sink.String(), "\n") {
				if strings.Contains(line, "session read paused") && strings.Contains(line, "peer="+addr) {
					return
				}
			}
			if time.Now().After(deadline) {
				t.Fatalf("the read loop of %s never paused for a read buffer:\n%s", addr, sink.String())
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// TestSessionReadWaitHasNoBoundWithoutHoldTime proves a read-buffer wait on a
// session with a hold time of zero keeps the session: no NOTIFICATION reaches
// the peer and Run keeps running however long the pool stays exhausted, and
// the session reads again once a buffer returns. Method: fake clock, hold time
// 0, exhaust the 4K pool, wait for the pause WARN, advance the clock well
// past the 90 s the wait used to end at, check the peer received nothing, then release and read a
// KEEPALIVE.
//
// VALIDATES: awaitReadBuffer has no bound of its own, as FRR's read path has
// none (bgp_io.c bgp_process_reads re-arms the read on a full input queue).
// PREVENTS: ze ending a healthy session with Cease / Out of Resources for its
// own buffer shortage, which neither FRR nor BIRD does.
func TestSessionReadWaitHasNoBoundWithoutHoldTime(t *testing.T) {
	bufMuxGlobalMu.Lock()
	defer bufMuxGlobalMu.Unlock()

	clk := sim.NewFakeClock(time.Now())
	session, client := bufferWaitSession(t, "192.0.2.71", clk)
	session.timers.SetHoldTime(0)
	paused := awaitReadPause(t, "192.0.2.71")

	release := exhaustStdReadPool(t)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- session.Run(ctx) }()

	paused()
	// Step past the old 90 s bound several times over a few real
	// milliseconds, so a bound armed just after the WARN is passed too.
	for range 5 {
		clk.Add(2 * fsm.DefaultHoldTime)
		time.Sleep(20 * time.Millisecond)
	}

	if err := client.SetReadDeadline(time.Now().Add(300 * time.Millisecond)); err != nil {
		t.Fatalf("ze closed the connection while it waited for a read buffer (%v), "+
			"want the session kept until a buffer returns or the hold timer expires", err)
	}
	header := make([]byte, message.HeaderLen)
	if n, err := io.ReadFull(client, header); n != 0 {
		t.Fatalf("the peer received %d octets (err %v) while ze waited for a read buffer, "+
			"want nothing: the wait keeps the session", n, err)
	}
	select {
	case err := <-result:
		t.Fatalf("Run returned %v while waiting for a read buffer, want it to keep waiting", err)
	default:
	}
	if err := client.SetReadDeadline(time.Time{}); err != nil {
		t.Fatalf("clear read deadline: %v", err)
	}

	written := make(chan error, 1)
	go func() {
		_, err := client.Write(message.PackTo(message.NewKeepalive(), nil))
		written <- err
	}()
	release()
	select {
	case err := <-written:
		if err != nil {
			t.Fatalf("KEEPALIVE write: %v", err)
		}
	case err := <-result:
		t.Fatalf("Run returned %v before reading the KEEPALIVE", err)
	case <-time.After(5 * time.Second):
		t.Fatal("the KEEPALIVE was not read after the read buffers were returned")
	}
	cancel()
	select {
	case <-result:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}

// TestSessionReadWaitLeavesExpiryToHoldTimer proves a read-buffer wait does
// not end the session itself while a hold timer runs: the hold timer does,
// with Hold Timer Expired, when it comes due (RFC 4271 Section 8.2.2, Event
// 10, which ze answers with no reprieve: session.go, the OnHoldTimerExpires
// callback). Method: fake clock, hold time 9 s started, exhaust the 4K pool,
// wait for the pause WARN, advance to 100 ms before the hold deadline and check the peer received
// nothing, then advance past it and read the NOTIFICATION.
//
// VALIDATES: the wait sends nothing of its own, and its stop channel lets the
// hold timer's teardown end it.
// PREVENTS: a Cease / Out of Resources sent ahead of the hold timer, which
// neither FRR nor BIRD sends for a full input queue.
func TestSessionReadWaitLeavesExpiryToHoldTimer(t *testing.T) {
	bufMuxGlobalMu.Lock()
	defer bufMuxGlobalMu.Unlock()

	const holdTime = 9 * time.Second
	clk := sim.NewFakeClock(time.Now())
	session, client := bufferWaitSession(t, "192.0.2.72", clk)
	session.timers.SetHoldTime(holdTime)
	session.timers.StartHoldTimer()
	paused := awaitReadPause(t, "192.0.2.72")

	exhaustStdReadPool(t)

	result := make(chan error, 1)
	go func() { result <- session.Run(t.Context()) }()

	paused()
	time.Sleep(20 * time.Millisecond) // a bound armed just after the WARN
	clk.Add(holdTime - readBufferRecheck)
	if err := client.SetReadDeadline(time.Now().Add(300 * time.Millisecond)); err != nil {
		t.Fatalf("ze closed the connection while it waited for a read buffer (%v), "+
			"want the session kept until a buffer returns or the hold timer expires", err)
	}
	header := make([]byte, message.HeaderLen)
	if n, err := io.ReadFull(client, header); n != 0 {
		t.Fatalf("the peer received %d octets (err %v) before the hold deadline, "+
			"want nothing: the wait sends no NOTIFICATION of its own", n, err)
	}

	// The fake clock runs the hold timer's callback inside Add, and that
	// callback writes the NOTIFICATION to a pipe only this goroutine reads,
	// so the clock advances from another goroutine.
	advanced := make(chan struct{})
	go func() {
		clk.Add(2 * readBufferRecheck)
		close(advanced)
	}()
	code, subcode := readNotificationCodes(t, client)
	<-advanced
	if code != uint8(message.NotifyHoldTimerExpired) || subcode != 0 {
		t.Fatalf("the peer received NOTIFICATION %d/%d, want Hold Timer Expired %d/0",
			code, subcode, message.NotifyHoldTimerExpired)
	}
	select {
	case <-result:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after the hold timer ended the session")
	}
}

// TestSessionReadWaitRetriesWithoutReturn proves the wait retries Get on its
// own (readBufferRecheck) when capacity comes back with no Return to the
// session's pool: only the byte budget is raised, nothing wakes the pool's
// bufWake, and the message is still read. Method: exhaust the 4K pool, start
// Run, write a KEEPALIVE, raise the budget without returning a buffer.
//
// VALIDATES: the 100 ms recheck in awaitReadBuffer.
// PREVENTS: a reader asleep while auto-sizing had already raised the budget.
func TestSessionReadWaitRetriesWithoutReturn(t *testing.T) {
	bufMuxGlobalMu.Lock()
	defer bufMuxGlobalMu.Unlock()

	session, client := bufferWaitSession(t, "192.0.2.73", nil)
	exhaustStdReadPool(t)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- session.Run(ctx) }()

	written := make(chan error, 1)
	go func() {
		_, err := client.Write(message.PackTo(message.NewKeepalive(), nil))
		written <- err
	}()
	select {
	case err := <-written:
		t.Fatalf("the KEEPALIVE was read (write err %v) with no read buffer free", err)
	case err := <-result:
		t.Fatalf("Run returned %v while the read pool was exhausted", err)
	case <-time.After(250 * time.Millisecond):
	}

	// Room for a new block, and no buffer returned: only the recheck sees it.
	updateBufMuxBudget(0)

	select {
	case err := <-written:
		if err != nil {
			t.Fatalf("KEEPALIVE write: %v", err)
		}
	case err := <-result:
		t.Fatalf("Run returned %v before reading the KEEPALIVE", err)
	case <-time.After(2 * time.Second):
		t.Fatal("the KEEPALIVE was not read after the budget was raised with no Return")
	}
	cancel()
	select {
	case <-result:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}

// TestSessionReadWaitWarnsOncePerEpisode proves a wait logs its WARN once,
// however many rechecks it runs, and that the read which ends the wait logs
// the resume and clears the episode. Method: a log sink, exhaust the 4K pool,
// hold the wait through several rechecks, then release. A second episode is
// not driven here: the session is not Established, so the KEEPALIVE that
// ends the first one also ends Run.
//
// VALIDATES: readBufferStarved scopes the WARN to one episode.
// PREVENTS: a WARN every 100 ms while a pool stays exhausted, and an episode
// that never ends, so the next shortage logs nothing.
func TestSessionReadWaitWarnsOncePerEpisode(t *testing.T) {
	bufMuxGlobalMu.Lock()
	defer bufMuxGlobalMu.Unlock()

	sink := &syncBuffer{}
	lg := slog.New(slog.NewTextHandler(sink, &slog.HandlerOptions{Level: slog.LevelDebug}))
	t.Cleanup(swapSessionLogger(func() *slog.Logger { return lg }))
	const peer = "peer=192.0.2.74"
	count := func(text string) int {
		n := 0
		for line := range strings.SplitSeq(sink.String(), "\n") {
			if strings.Contains(line, text) && strings.Contains(line, peer) {
				n++
			}
		}
		return n
	}

	session, client := bufferWaitSession(t, "192.0.2.74", nil)
	release := exhaustStdReadPool(t)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- session.Run(ctx) }()

	keepalive := message.PackTo(message.NewKeepalive(), nil)
	written := make(chan error, 1)
	go func() {
		_, err := client.Write(keepalive)
		written <- err
	}()
	// Several rechecks pass while the pool stays exhausted.
	time.Sleep(5 * readBufferRecheck)
	if n := count("session read paused"); n != 1 {
		t.Fatalf("%d WARN lines during one wait episode, want 1:\n%s", n, sink.String())
	}
	// The WARN names the knobs by their full config path and says what
	// happens now: the session waits, TCP holds the peer back, and only the
	// hold timer can end it. No ze.fwd.pool.maxbytes is set here, so the pool
	// is auto-sized and the headroom leaf is the knob.
	for _, want := range []string{
		"environment reactor forward-pool-headroom",
		"environment reactor forward-pool-max-bytes",
		"the peer is held back by TCP",
		"Hold Timer Expired",
	} {
		if count(want) != 1 {
			t.Errorf("the WARN does not name %q:\n%s", want, sink.String())
		}
	}
	for _, stale := range []string{"nothing is dropped", "Out of Resources"} {
		if count(stale) != 0 {
			t.Errorf("the WARN still claims %q:\n%s", stale, sink.String())
		}
	}

	release()
	if err := <-written; err != nil {
		t.Fatalf("KEEPALIVE write: %v", err)
	}

	// The resume is logged once the read succeeds, which ends the episode:
	// readBufferStarved is cleared with it, so the next wait warns again.
	deadline := time.Now().Add(5 * time.Second)
	for count("session read resumed") < 1 {
		if time.Now().After(deadline) {
			t.Fatalf("the read after the wait logged no resume:\n%s", sink.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	if n := count("session read paused"); n != 1 {
		t.Fatalf("%d WARN lines for one wait episode, want 1:\n%s", n, sink.String())
	}
	cancel()
	select {
	case <-result:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}
