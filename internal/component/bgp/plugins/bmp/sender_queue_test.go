package bmp

import (
	"bytes"
	"errors"
	"io"
	"net"
	"net/netip"
	"os"
	"sync"
	"testing"
	"time"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// --- test connections -------------------------------------------------------

// fakeAddr satisfies net.Addr for the fake connections below.
type fakeAddr struct{}

func (fakeAddr) Network() string { return "fake" }
func (fakeAddr) String() string  { return "fake" }

// wedgedConn is a collector that accepts a connection and then stops reading:
// every Write blocks until the conn is closed. It is the shape of the failure
// this whole queue exists for, and unlike a real socket it has no kernel buffer
// to absorb the first few hundred KB, so the stall is deterministic.
type wedgedConn struct {
	mu       sync.Mutex
	written  bytes.Buffer
	writes   int
	deadline time.Time
	closed   chan struct{}
	once     sync.Once
}

func newWedgedConn() *wedgedConn { return &wedgedConn{closed: make(chan struct{})} }

// Write records what the session TRIED to send (so a test can assert on it) and
// then blocks until the write deadline expires or the conn is closed -- exactly
// what a real socket to a collector that has stopped reading does.
func (c *wedgedConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	c.written.Write(p)
	c.writes++
	deadline := c.deadline
	c.mu.Unlock()

	var expired <-chan time.Time
	if !deadline.IsZero() {
		timer := time.NewTimer(time.Until(deadline))
		defer timer.Stop()
		expired = timer.C
	}
	select {
	case <-c.closed:
		return 0, net.ErrClosed
	case <-expired:
		return 0, os.ErrDeadlineExceeded
	}
}

func (c *wedgedConn) Read([]byte) (int, error) {
	<-c.closed
	return 0, io.EOF
}

func (c *wedgedConn) Close() error {
	c.once.Do(func() { close(c.closed) })
	return nil
}

func (c *wedgedConn) LocalAddr() net.Addr             { return fakeAddr{} }
func (c *wedgedConn) RemoteAddr() net.Addr            { return fakeAddr{} }
func (c *wedgedConn) SetDeadline(time.Time) error     { return nil }
func (c *wedgedConn) SetReadDeadline(time.Time) error { return nil }
func (c *wedgedConn) SetWriteDeadline(t time.Time) error {
	c.mu.Lock()
	c.deadline = t
	c.mu.Unlock()
	return nil
}
func (c *wedgedConn) isClosed() bool {
	select {
	case <-c.closed:
		return true
	default:
		return false
	}
}

// writeCount reports how many Write calls the session has made, so a test can
// wait for the drain to be blocked inside one instead of guessing.
func (c *wedgedConn) writeCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.writes
}

func (c *wedgedConn) bytesWritten() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]byte{}, c.written.Bytes()...)
}

// recordingConn accepts everything instantly and keeps a copy, so a test can
// assert on the exact BMP byte stream a session produced.
type recordingConn struct {
	mu     sync.Mutex
	buf    bytes.Buffer
	closed chan struct{}
	once   sync.Once
}

func newRecordingConn() *recordingConn { return &recordingConn{closed: make(chan struct{})} }

func (c *recordingConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.Write(p)
}

func (c *recordingConn) written() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]byte{}, c.buf.Bytes()...)
}

func (c *recordingConn) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.buf.Reset()
}

func (c *recordingConn) Read([]byte) (int, error) { <-c.closed; return 0, io.EOF }
func (c *recordingConn) Close() error {
	c.once.Do(func() { close(c.closed) })
	return nil
}
func (c *recordingConn) LocalAddr() net.Addr              { return fakeAddr{} }
func (c *recordingConn) RemoteAddr() net.Addr             { return fakeAddr{} }
func (c *recordingConn) SetDeadline(time.Time) error      { return nil }
func (c *recordingConn) SetReadDeadline(time.Time) error  { return nil }
func (c *recordingConn) SetWriteDeadline(time.Time) error { return nil }

// discardConn accepts everything instantly and keeps nothing. Used to measure
// the producer's allocations with a real drain goroutine running.
type discardConn struct {
	closed chan struct{}
	once   sync.Once
}

func newDiscardConn() *discardConn { return &discardConn{closed: make(chan struct{})} }

func (c *discardConn) Write(p []byte) (int, error) { return len(p), nil }
func (c *discardConn) Read([]byte) (int, error)    { <-c.closed; return 0, io.EOF }
func (c *discardConn) Close() error {
	c.once.Do(func() { close(c.closed) })
	return nil
}
func (c *discardConn) LocalAddr() net.Addr              { return fakeAddr{} }
func (c *discardConn) RemoteAddr() net.Addr             { return fakeAddr{} }
func (c *discardConn) SetDeadline(time.Time) error      { return nil }
func (c *discardConn) SetReadDeadline(time.Time) error  { return nil }
func (c *discardConn) SetWriteDeadline(time.Time) error { return nil }

// --- tests ------------------------------------------------------------------

// newTestSession returns a fully constructed sender session attached to conn,
// with its drain goroutine stopped at test end. It goes through
// newSenderSession so the fixture has the same stop context, scratch buffer and
// queue bound production has; only the connection is handed over rather than
// dialed.
func newTestSession(t *testing.T, name string, conn net.Conn) *senderSession {
	t.Helper()
	ss := newSenderSession(name, collectorConfig{})
	ss.conn = conn
	t.Cleanup(func() {
		ss.stop()
		ss.waitDrain()
	})
	return ss
}

// waitQueueDrained blocks until every enqueued byte has been written to the
// collector. Needed wherever a test used to rely on write* being synchronous:
// the producer returning now only means the message is queued.
func waitQueueDrained(t *testing.T, ss *senderSession) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		q := ss.queue()
		if q == nil || q.bytesPending() == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("transmit queue still holds %d bytes after 30s", q.bytesPending())
		}
		// Poll interval; the loop returns as soon as the queue reports empty.
		time.Sleep(time.Millisecond)
	}
}

// waitFor blocks until cond() is true, failing the test at the deadline. Used
// wherever a test must wait for the drain goroutine to reach a state: the drain
// is asynchronous, so waiting on the scheduler instead makes the test pass or
// fail on how busy the host is (ai/rules/completion.md).
func waitFor(t *testing.T, limit time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(limit)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("condition not met within %s", limit)
		}
		// Poll interval; the loop returns as soon as cond() holds.
		time.Sleep(time.Millisecond)
	}
}

// decodeBMPStream splits a recorded byte stream into decoded BMP messages. A
// trailing partial message (the session was cut mid-write) is ignored.
func decodeBMPStream(t *testing.T, raw []byte) []any {
	t.Helper()
	var out []any
	for off := 0; off+CommonHeaderSize <= len(raw); {
		ch, _, err := decodeCommonHeader(raw[off:], 0)
		if err != nil || int(ch.Length) < CommonHeaderSize {
			t.Fatalf("collector stream is not framed BMP at offset %d: %v", off, err)
		}
		end := off + int(ch.Length)
		if end > len(raw) {
			return out // truncated by the reset; nothing further was delivered
		}
		msg, err := DecodeMsg(raw[off:end])
		if err != nil {
			t.Fatalf("decode message at offset %d: %v", off, err)
		}
		out = append(out, msg)
		off = end
	}
	return out
}

// locRIBBatch builds a best-change batch with n distinct IPv4 best paths.
func locRIBBatch(n int, replayed bool) *ribevents.BestChangeBatch {
	b := &ribevents.BestChangeBatch{Protocol: "bgp", Family: family.IPv4Unicast}
	if replayed {
		b.ReplayID = 1
	}
	for i := range n {
		b.Changes = append(b.Changes, ribevents.BestChangeEntry{
			Action:  ribevents.BestChangeAdd,
			Prefix:  netip.MustParsePrefix("10.20." + string(rune('0'+i%10)) + ".0/24"),
			NextHop: netip.MustParseAddr("192.0.2.1"),
			ASPath:  []uint32{65001},
		})
	}
	return b
}

func TestHandleBestChangeDoesNotBlockOnWedgedCollector(t *testing.T) {
	// VALIDATES: the RIB best-change handler returns promptly while the
	// collector socket is wedged -- no socket write happens on the EventBus
	// subscriber goroutine (pkg/ze/eventbus.go: a handler "MUST NOT block on
	// I/O").
	// PREVENTS: the headline defect. Before the transmit queue, handleBestChange
	// did len(Changes) x len(senders) blocking conn.Write calls, each bounded
	// only by writeTimeout (10s), so one unresponsive collector stalled RIB
	// best-path publication for the whole product.
	conn := newWedgedConn()
	ss := newTestSession(t, "wedged", conn)

	bp := &BMPPlugin{
		senders:   []*senderSession{ss},
		openCache: map[string]*openPair{"10.0.0.1": {sent: makeBGPOpen(65000, 0x01020305)}},
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		bp.handleBestChange(locRIBBatch(8, false))
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("handleBestChange blocked on a wedged collector: the publisher goroutine is doing socket I/O")
	}
}

func TestSenderQueueOverflowResetsSessionWithoutTermination(t *testing.T) {
	// VALIDATES: when the bounded transmit queue fills, the session is reset --
	// the connection is closed, the queue is dropped, and the producer gets an
	// error instead of blocking. No Termination message is written (RFC 7854
	// Section 4.5 Termination is not the "out of resources" signal; BIRD defines
	// BMP_TERM_REASON_OOR and never sends it, proto/bmp/bmp.c:159,981).
	// PREVENTS: unbounded queue growth on a wedged collector (BIRD's own
	// pre-e6a100b3 bug), and silently dropping monitoring messages.
	conn := newWedgedConn()
	ss := newTestSession(t, "tiny", conn)
	ss.txLimit = 4096 // set before the first enqueue creates the queue

	peer := testPeerHeader()
	body := bytes.Repeat([]byte{0xEE}, 500)

	// Wait for the drain to be blocked INSIDE a write before filling the queue.
	// Without this the test races the scheduler: on a busy host (and always
	// under -cpu=1) the producer overflows the 4096-byte queue before the drain
	// is ever scheduled, so the session is reset with nothing written -- the
	// collector then has no bytes and the no-Termination assertion below has
	// nothing to examine. Waiting on the condition instead of on scheduling
	// luck also means the assertion can never pass vacuously
	// (ai/rules/completion.md).
	if err := ss.writeRouteMonitoring(peer, msgtype.TypeUPDATE, body); err != nil {
		t.Fatalf("first write: %v", err)
	}
	waitFor(t, 10*time.Second, func() bool { return conn.writeCount() > 0 })

	var overflowed bool
	for range 200 {
		err := ss.writeRouteMonitoring(peer, msgtype.TypeUPDATE, body)
		if errors.Is(err, errQueueOverflow) {
			overflowed = true
			break
		}
		if err != nil && !errors.Is(err, errNotConnected) {
			t.Fatalf("unexpected write error: %v", err)
		}
	}
	if !overflowed {
		t.Fatal("queue never overflowed with a 4096-byte limit and 100KB of messages")
	}

	if !conn.isClosed() {
		t.Error("overflow must reset the session by closing the collector connection")
	}
	ss.connMu.Lock()
	c := ss.conn
	ss.connMu.Unlock()
	if c != nil {
		t.Error("overflow must clear the session connection so producers stop enqueueing")
	}
	if q := ss.queue(); q != nil && q.bytesPending() != 0 {
		t.Errorf("queue still holds %d bytes after the reset, want 0", q.bytesPending())
	}
	written := conn.bytesWritten()
	if len(written) == 0 {
		// Without this the loop below never executes and the assertion passes
		// having observed nothing at all.
		t.Fatal("collector received no bytes at all; the no-Termination check would be vacuous")
	}
	for i, msg := range decodeBMPStream(t, written) {
		if _, ok := msg.(*Termination); ok {
			t.Errorf("message %d written to the collector is a Termination: overflow must be a bare TCP close", i)
		}
	}

	// The byte-stream check above CANNOT fail on its own, and that is the point
	// of this one. Swapping the overflow's closeLog for terminateAndClose leaves
	// the stream identical: sendTermination gives up after terminationWait
	// because it cannot take flushMu, which the drain is holding across the
	// wedged 10s write. So the absence of a Termination on the wire proves
	// nothing about the code path -- it is guaranteed by the wedge.
	//
	// termOnce is the discriminating signal: terminateAndClose consumes it
	// whether or not the write lands. If the overflow path engaged the
	// termination machinery at all, this Do body does not run.
	fired := false
	ss.termOnce.Do(func() { fired = true })
	if !fired {
		t.Error("overflow consumed the session's termination guard: it must be a bare TCP close, " +
			"not a Termination attempt that merely failed to acquire the write lock")
	}

	// The producer must keep working (returning errNotConnected, not blocking)
	// after the reset, so a stalled collector cannot wedge the RIB publisher.
	if err := ss.writeRouteMonitoring(peer, msgtype.TypeUPDATE, body); !errors.Is(err, errNotConnected) {
		t.Errorf("after a reset the producer gets %v, want errNotConnected", err)
	}
}

func TestSenderRouteMonitoringHotPathIsAllocationFree(t *testing.T) {
	// VALIDATES: encode-and-enqueue of a Route Monitoring message allocates
	// nothing -- the message is built in the per-session scratch buffer and
	// copied into pooled queue pages.
	// PREVENTS: the [][]byte handoff this design rejected. A queue of per-message
	// byte slices would allocate once per Route Monitoring message on the
	// BGP-UPDATE -> BMP hot path, which is exactly the property
	// newSenderSession's scratch comment records (ai/rules/performance.md,
	// ai/rules/performance.md).
	conn := newDiscardConn()
	ss := newTestSession(t, "alloc", conn)

	peer := testPeerHeader()
	body := bytes.Repeat([]byte{0xAB}, 120)

	// Warm the session: allocate the scratch buffer and the first queue page.
	for range 32 {
		if err := ss.writeRouteMonitoring(peer, msgtype.TypeUPDATE, body); err != nil {
			t.Fatalf("warm-up write: %v", err)
		}
	}

	allocs := testing.AllocsPerRun(500, func() {
		if err := ss.writeRouteMonitoring(peer, msgtype.TypeUPDATE, body); err != nil {
			t.Errorf("writeRouteMonitoring: %v", err)
		}
	})
	if allocs > 0 {
		t.Errorf("writeRouteMonitoring allocates %.2f objects per message, want 0", allocs)
	}
}

// BenchmarkSenderRouteMonitoring reports the allocation cost of the
// BGP-UPDATE -> BMP Route Monitoring path with the transmit queue in place.
// Run with -benchmem; B/op and allocs/op must stay at zero.
func BenchmarkSenderRouteMonitoring(b *testing.B) {
	conn := newDiscardConn()
	ss := newSenderSession("bench", collectorConfig{})
	ss.conn = conn
	b.Cleanup(func() {
		ss.stop()
		ss.waitDrain()
	})

	peer := testPeerHeader()
	body := bytes.Repeat([]byte{0xAB}, 120)

	b.ReportAllocs()
	for b.Loop() {
		if err := ss.writeRouteMonitoring(peer, msgtype.TypeUPDATE, body); err != nil {
			b.Fatalf("writeRouteMonitoring: %v", err)
		}
	}
}

func TestConfigureConcurrentWithEventDeliveryIsRaceFree(t *testing.T) {
	// VALIDATES: the route-monitoring-policy and route-mirroring config leaves
	// can be republished while events are being processed, without a data race
	// and without an event ever seeing half of each configuration.
	// PREVENTS: the plain field write/read this replaced. bp.routeMonitorPolicy
	// and bp.routeMirroring were assigned in OnConfigure with no lock and read
	// from the plugin's event-delivery goroutine. Benign today only because
	// Stage-2 configure completes before the delivery goroutine exists
	// (pkg/plugin/sdk/sdk.go registers the structured handler on the bridge only
	// after Stage 5 ready); any reload path that re-delivers configure makes it
	// a live race, and this test makes it one now. Run under -race.
	conn := newDiscardConn()
	ss := newTestSession(t, "cfg-race", conn)

	bp := &BMPPlugin{
		senders:    []*senderSession{ss},
		openCache:  make(map[string]*openPair),
		peerUps:    make(map[string]*peerUpState),
		dedupState: make(map[string]map[uint64]struct{}),
		stopCh:     make(chan struct{}),
	}

	se := &rpc.StructuredEvent{
		PeerAddress: "10.0.0.1",
		PeerAS:      65001,
		EventType:   rpc.EventKindUpdate,
		Direction:   rpc.DirectionReceived,
		RawMessage:  &bgptypes.RawMessage{Type: msgtype.TypeUPDATE, RawBytes: []byte{0x00, 0x00, 0x00, 0x00, 0x00}},
	}

	// Republish config while events are being delivered, both hard enough to
	// interleave. The assertion is the race detector plus "no panic": what is
	// under test is the memory model, not a message count.
	var wg sync.WaitGroup
	wg.Go(func() {
		for i := range 400 {
			bp.setSenderPolicy(policyFor(i), i%2 == 0)
		}
	})
	wg.Go(func() {
		for range 400 {
			bp.handleStructuredEvent(se)
		}
	})
	wg.Wait()

	// The last write must be the one that stuck: a snapshot read cannot leave
	// the fields holding a mix of two configurations.
	bp.setSenderPolicy("post-policy", true)
	bp.mu.RLock()
	gotPolicy, gotMirror := bp.routeMonitorPolicy, bp.routeMirroring
	bp.mu.RUnlock()
	if gotPolicy != "post-policy" || !gotMirror {
		t.Errorf("config after the last publish = (%q, %v), want (\"post-policy\", true)", gotPolicy, gotMirror)
	}
}

// policyFor cycles the three valid route-monitoring-policy values.
func policyFor(i int) string {
	switch i % 3 {
	case 0:
		return "all"
	case 1:
		return "pre-policy"
	default:
		return "post-policy"
	}
}

func TestStaleDrainDoesNotDiscardTheNextSessionsQueue(t *testing.T) {
	// VALIDATES: a drain still blocked writing to a connection that has since
	// been torn down must not drop the transmit queue -- by then the queue holds
	// the NEXT connection's priming messages.
	// PREVENTS: the silent RFC 7854 Section 4.10 break. A write can block for
	// writeTimeout, long enough for run() to redial, publish a new connection
	// and prime it with a Peer Up for every established peer. Wiping that queue
	// left the collector receiving Route Monitoring for peers it never saw come
	// up, with nothing logged and nothing to recover it -- worse than a reset,
	// because the session stays up and wrong.
	old := newWedgedConn()
	ss := newTestSession(t, "stale-drain", old)

	peer := testPeerHeader()
	body := bytes.Repeat([]byte{0x11}, 64)

	// Get the drain blocked inside Write on the OLD connection.
	if err := ss.writeRouteMonitoring(peer, msgtype.TypeUPDATE, body); err != nil {
		t.Fatalf("priming write: %v", err)
	}
	waitFor(t, 10*time.Second, func() bool { return old.writeCount() > 0 })

	// run() would now tear the old connection down, redial, and prime the new
	// one. Same sequence, driven directly.
	ss.clearConn()
	ss.resetQueue()
	fresh := newRecordingConn()
	t.Cleanup(func() { closeLog(fresh, "fresh-conn") })
	ss.connMu.Lock()
	ss.conn = fresh
	ss.connMu.Unlock()
	if err := ss.writePeerUp(peer, [16]byte{}, 179, 0, makeBGPOpen(65001, 1), makeBGPOpen(65002, 2)); err != nil {
		t.Fatalf("priming the new session: %v", err)
	}

	// Release the old connection: the stale write now fails and takes its
	// failure path while the new session's Peer Up is queued.
	closeLog(old, "release-stale-write")

	waitFor(t, 10*time.Second, func() bool { return len(fresh.written()) > 0 })
	msgs := decodeBMPStream(t, fresh.written())
	if len(msgs) == 0 {
		t.Fatal("the new session's queued Peer Up was discarded by the stale drain")
	}
	if _, ok := msgs[0].(*PeerUp); !ok {
		t.Errorf("new connection received %T first, want the primed *PeerUp", msgs[0])
	}
}

func TestSenderStopSendsTerminationToCollector(t *testing.T) {
	// VALIDATES: RFC 7854 Section 4.5 -- when ze closes a BMP session, the
	// collector receives a Termination message BEFORE the TCP connection goes
	// away. Driven through the production interaction (stop() racing the
	// holdConnection loop), not by calling sendTermination directly.
	// PREVENTS: the regression this test was written for: stop() closed the
	// socket first and holdConnection then wrote the Termination into a closed
	// connection, so no collector ever saw one. TestBMPSenderTermination could
	// not catch it -- it calls sendTermination on a live pipe.
	server, client := net.Pipe()
	defer closeLog(server, "server-pipe")

	ss := newTestSession(t, "term", client)

	var hold sync.WaitGroup
	hold.Go(func() { ss.holdConnection(client) })

	// Read whatever arrives until the connection ends.
	type readOut struct {
		sawTermination bool
	}
	result := make(chan readOut, 1)
	go func() {
		var out readOut
		for {
			if err := server.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
				break
			}
			msg, err := readBMPFromPipe(server)
			if err != nil {
				break
			}
			if _, ok := msg.(*Termination); ok {
				out.sawTermination = true
			}
		}
		result <- out
	}()

	ss.stop()
	hold.Wait()
	closeLog(client, "client-pipe")

	select {
	case out := <-result:
		if !out.sawTermination {
			t.Error("collector never received a Termination: RFC 7854 Section 4.5 requires it when the session is closed")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("collector reader did not finish")
	}
}
