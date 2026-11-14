// Design: docs/architecture/bgp/structural-forwarding.md -- the write budget of the route-server fast path
// Related: forward_rs_direct.go -- tryDirectWriteNoFlush, the writer under test
// Related: session_write_mutex.go -- armFastPathBudget and releaseFastPathWrites
// Related: session_write_budget_test.go -- setWriteBudget and budgetShort

package reactor

import (
	"bufio"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/test/sim"
)

// fastPathBufferOctets is the size of the destination's bufWriter in these
// tests, the size connectionEstablished gives it by default.
const fastPathBufferOctets = 16384

// fastPathBatchWrites is how many UPDATEs a test forwards to one destination
// when it needs the destination's bufWriter to overflow, so that a write
// reaches the socket inside the fast path. Each UPDATE is a header and four
// octets, so the buffer overflows well before the last of them.
const fastPathBatchWrites = 1000

// fastPathFrameOctets is what one forwarded UPDATE of these tests puts on the
// wire: a header and the four octets of fastPathTarget.body.
const fastPathFrameOctets = message.HeaderLen + 4

// fastPathTally is what a fastPathConn saw.
type fastPathTally struct {
	arms      int // write deadlines set to a time
	clears    int // write deadlines cleared
	writes    int // socket writes
	unbounded int // socket writes that went out with no write deadline
	expired   int // socket writes that went out under a deadline already passed
}

// fastPathConn is a destination socket that takes every write at once and
// records what bounded it: the deadlines set and cleared, and for each write
// whether a live deadline stood. It needs no reader. Safe for concurrent use.
type fastPathConn struct {
	now func() time.Time // the clock the session under test arms with

	mu       sync.Mutex
	deadline time.Time
	tally    fastPathTally
}

func (c *fastPathConn) Write(b []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tally.writes++
	if c.deadline.IsZero() {
		c.tally.unbounded++
		return len(b), nil
	}
	if !c.deadline.After(c.now()) {
		c.tally.expired++
	}
	return len(b), nil
}

func (c *fastPathConn) SetWriteDeadline(deadline time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if deadline.IsZero() {
		c.tally.clears++
	} else {
		c.tally.arms++
	}
	c.deadline = deadline
	return nil
}

// seen returns what the connection recorded so far.
func (c *fastPathConn) seen() fastPathTally {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tally
}

// age moves the deadline the socket carries into the past, as the passing of
// time does, without counting it as an arm. It returns the deadline it set.
func (c *fastPathConn) age() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deadline = c.now().Add(-time.Millisecond)
	return c.deadline
}

func (c *fastPathConn) Read([]byte) (int, error)        { return 0, os.ErrClosed }
func (c *fastPathConn) Close() error                    { return nil }
func (c *fastPathConn) LocalAddr() net.Addr             { return &net.TCPAddr{} }
func (c *fastPathConn) RemoteAddr() net.Addr            { return &net.TCPAddr{} }
func (c *fastPathConn) SetDeadline(time.Time) error     { return nil }
func (c *fastPathConn) SetReadDeadline(time.Time) error { return nil }

// fastPathTarget is one Established route-server destination and the item the
// fast path forwards to it. The item and its body slice live here, so a write
// allocates nothing of the fixture's.
type fastPathTarget struct {
	peer    *Peer
	session *Session
	item    fwdItem
	bodies  [1][]byte
	// body is the UPDATE body every test forwards: no withdrawn route, no
	// attribute, no NLRI. It names no route, so no write is suppressed as a
	// duplicate of the one before it.
	body [4]byte
}

// newFastPathTarget builds a destination as TestReactorForwardRSDirectWrite
// does. Its socket is one end of a net.Pipe, and the returned connection is
// the peer's end: a write to the destination blocks until that end reads.
func newFastPathTarget(tb testing.TB, addr string, peerAS uint32) (*fastPathTarget, net.Conn) {
	tb.Helper()
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(tb, err)
	peer, session, peerEnd := makeRSPeerWithSession(tb, addr, peerAS, ctx, ctxID)
	return &fastPathTarget{peer: peer, session: session}, peerEnd
}

// newFastPathSource builds the peer the forwarded UPDATEs come from, and the
// session whose read loop the fast path runs on.
func newFastPathSource(tb testing.TB) (*Peer, *Session) {
	tb.Helper()
	source, _ := newFastPathTarget(tb, "10.0.0.1", 65001)
	return source.peer, source.session
}

// record replaces the destination's socket with a fastPathConn that reads now
// as its clock, and returns it.
func (d *fastPathTarget) record(now func() time.Time) *fastPathConn {
	conn := &fastPathConn{now: now}
	d.session.mu.Lock()
	d.session.writeMu.Lock()
	d.session.conn = conn
	d.session.bufWriter = bufio.NewWriterSize(conn, fastPathBufferOctets)
	d.session.writeMu.Unlock()
	d.session.mu.Unlock()
	return conn
}

// write forwards the body from source through tryDirectWriteNoFlush, with
// the item reactorForwardRSSection builds.
func (d *fastPathTarget) write(source *Peer) (handled, delivered bool) {
	d.item = fwdItem{peer: d.peer, session: d.session, authority: adjOutForwarded}
	d.item.receivedPeer = source
	d.item.receivedGeneration = source.forwardGeneration.Load()
	d.item.sourcePolicyEpoch = source.sourcePolicyEpoch.Load()
	d.bodies[0] = d.body[:]
	d.item.rawBodies = d.bodies[:]
	handled, delivered, _ = tryDirectWriteNoFlush(&d.item)
	return handled, delivered
}

// writeBatch forwards count UPDATEs and returns how many were delivered.
func (d *fastPathTarget) writeBatch(source *Peer, count int) int {
	delivered := 0
	for range count {
		if _, ok := d.write(source); ok {
			delivered++
		}
	}
	return delivered
}

// budgetStands reports whether the destination's socket carries a write budget
// no release has cleared.
func (d *fastPathTarget) budgetStands() bool {
	d.session.writeMu.Lock()
	defer d.session.writeMu.Unlock()
	return d.session.writeBudgetArmed
}

// TestFastPathStalledDestinationDoesNotStallSource proves a destination that
// stops reading cannot hold the source peer's read goroutine.
//
// Method: one source forwards the same UPDATEs to two destinations, as its
// read loop does. Nobody reads the first destination's socket, so the write
// that overflows its bufWriter blocks in the fast path. The second is read
// throughout.
//
// VALIDATES: AC-6 -- the blocked write returns once the write budget passed,
// the stalled destination is retired and reported once as a peer that stopped
// reading, and the source goes on forwarding to the other destination.
// PREVENTS: a fast-path write with no deadline, which held the source's read
// goroutine and the destination's write path for as long as the peer kept its
// TCP window shut.
func TestFastPathStalledDestinationDoesNotStallSource(t *testing.T) {
	setWriteBudget(t, budgetShort)
	source, sourceSession := newFastPathSource(t)
	stalled, _ := newFastPathTarget(t, "10.0.0.2", 65002)
	healthy, healthyPeerEnd := newFastPathTarget(t, "10.0.0.3", 65003)

	var expiries atomic.Int32
	stalled.session.onWriteBudgetExpired = func() { expiries.Add(1) }

	var healthyOctets atomic.Int64
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := healthyPeerEnd.Read(buf)
			healthyOctets.Add(int64(n))
			if err != nil {
				return // The fixture's cleanup closed the pipe.
			}
		}
	}()

	type forwarded struct{ stalled, healthy int }
	loopDone := make(chan forwarded, 1)
	go func() {
		var count forwarded
		for range fastPathBatchWrites {
			if _, delivered := stalled.write(source); delivered {
				count.stalled++
			}
			if _, delivered := healthy.write(source); delivered {
				count.healthy++
			}
		}
		sourceSession.appendFwdDirty(stalled.peer, stalled.session)
		sourceSession.appendFwdDirty(healthy.peer, healthy.session)
		sourceSession.flushFwdDirty()
		loopDone <- count
	}()

	var count forwarded
	select {
	case count = <-loopDone:
	case <-time.After(budgetShort + runExitDeadline):
		t.Fatal("the source was still blocked in a fast-path write to a destination that reads nothing, long after the write budget")
	}

	require.Positive(t, count.stalled,
		"precondition: nothing was buffered for the stalled destination, so no write of it reached its socket")
	require.Less(t, count.stalled, fastPathBatchWrites,
		"precondition: every write to the stalled destination was delivered, so none met its shut socket")
	require.Equal(t, fastPathBatchWrites, count.healthy,
		"the source must go on forwarding to the destination that reads")

	reason := closeCause(stalled.session)
	require.ErrorIs(t, reason, ErrWriteBudgetExpired,
		"the stalled destination must be retired as a peer that accepted no data for the write budget")
	require.ErrorIs(t, reason, os.ErrDeadlineExceeded,
		"the close reason must keep the error of the write that failed")
	require.Equal(t, int32(1), expiries.Load(),
		"one retired destination must be reported once, however many writes followed the failed one")

	require.NoError(t, closeCause(healthy.session), "the destination that reads must keep its session")
	want := int64(fastPathBatchWrites * fastPathFrameOctets)
	require.Eventually(t, func() bool { return healthyOctets.Load() == want },
		runExitDeadline, 5*time.Millisecond,
		"every UPDATE forwarded to the destination that reads must reach its socket")
}

// TestFastPathArmsOncePerBatch proves what the fast path's write budget costs:
// one deadline call per destination per batch, not one per UPDATE.
//
// Method: a destination whose socket counts the deadlines set on it. The
// budget is the default 30 s, so no share of it passes during the test.
//
// VALIDATES: the first fast-path write of a batch arms the budget, the next
// ones arm nothing, a write that follows another writer's release arms once
// more, and the batch's flush leaves no budget standing.
// PREVENTS: a deadline call per forwarded UPDATE on the hottest path.
func TestFastPathArmsOncePerBatch(t *testing.T) {
	source, sourceSession := newFastPathSource(t)
	target, _ := newFastPathTarget(t, "10.0.0.2", 65002)
	conn := target.record(time.Now)

	require.Equal(t, 5, target.writeBatch(source, 5), "precondition: the fast path delivered nothing")
	require.Equal(t, fastPathTally{arms: 1}, conn.seen(), "five writes of one batch arm the budget once")
	require.True(t, target.budgetStands(), "the budget stands between the writes of a batch")

	// Another writer takes the path between two writes of the batch.
	require.NoError(t, target.session.acquireWrite(fwdWriteDeadline()))
	target.session.releaseWrites()
	require.Equal(t, fastPathTally{arms: 2, clears: 1}, conn.seen(),
		"precondition: the other writer did not arm and clear a budget of its own")

	require.Equal(t, 3, target.writeBatch(source, 3))
	require.Equal(t, fastPathTally{arms: 3, clears: 1}, conn.seen(),
		"a write after another writer's release arms once more, and the ones after it arm nothing")

	sourceSession.appendFwdDirty(target.peer, target.session)
	sourceSession.flushFwdDirty()
	require.Equal(t, fastPathTally{arms: 4, clears: 2, writes: 1}, conn.seen(),
		"the flush arms its own budget, writes the batch under it, and clears")
	require.False(t, target.budgetStands(), "the batch's flush leaves no budget standing")
}

// TestFastPathRenewsAnAgedBudget proves a batch that lasts does not write
// under a deadline armed at its start.
//
// Method: the session runs on a fake clock, moved between two writes of one
// batch to each side of the share of the budget the fast path lets pass.
//
// VALIDATES: a standing budget is trusted up to one sixteenth of its length
// and armed again after it (fastPathBudgetShare).
// PREVENTS: a source that stops in the middle of a message, or sends for
// longer than the budget, failing the write of a destination that reads.
func TestFastPathRenewsAnAgedBudget(t *testing.T) {
	source, _ := newFastPathSource(t)
	target, _ := newFastPathTarget(t, "10.0.0.2", 65002)
	fake := sim.NewFakeClock(time.Now())
	target.session.clock = fake
	conn := target.record(fake.Now)

	_, delivered := target.write(source)
	require.True(t, delivered, "precondition: the fast path delivered nothing")
	require.Equal(t, 1, conn.seen().arms)

	fake.Add(fwdWriteDeadline() / fastPathBudgetShare)
	_, delivered = target.write(source)
	require.True(t, delivered)
	require.Equal(t, 1, conn.seen().arms, "a budget with exactly its trusted share spent still stands")

	fake.Add(time.Nanosecond)
	_, delivered = target.write(source)
	require.True(t, delivered)
	require.Equal(t, 2, conn.seen().arms, "a budget past its trusted share is armed again")
	require.Zero(t, conn.seen().clears, "the fast path never clears")
}

// TestFastPathBudgetDoesNotLeak proves the budget the fast path leaves
// standing fails no later writer, whichever way its batch ended.
//
// Method: one fast-path write arms the budget, the batch ends in one of the
// ways below, and a budget still standing is aged past its end. Then either an
// ordinary writer or a new fast-path batch, large enough to reach the socket,
// writes. The socket counts every write that went out with no deadline or
// under one that had passed.
//
// The ending with no case here is a destination that left Established: its
// connection is closing, and the next one writes its OPEN, released through
// releaseWrites, before the fast path can reach it.
//
// VALIDATES: R-1 -- after the batch's flush, after a flush the forward worker
// took over, after a flush that met a held path and kept the session, and
// after a source that ended with no flush, every later socket write goes out
// under a live deadline.
// PREVENTS: a deadline armed for one batch and found expired by the next
// fast-path write, which would retire a destination that reads.
func TestFastPathBudgetDoesNotLeak(t *testing.T) {
	endings := []struct {
		name   string
		end    func(t *testing.T, sourceSession *Session, target *fastPathTarget)
		stands bool
	}{
		{name: "the batch was flushed", end: func(_ *testing.T, sourceSession *Session, target *fastPathTarget) {
			sourceSession.appendFwdDirty(target.peer, target.session)
			sourceSession.flushFwdDirty()
		}},
		{name: "the forward worker took the flush over", end: func(_ *testing.T, _ *Session, target *fastPathTarget) {
			// The item handOffForwardFlush builds, handled as the worker does.
			key := fwdKey{peerAddr: target.peer.settings.PeerKey()}
			fwdBatchHandler(key, []fwdItem{{peer: target.peer, session: target.session}})
		}},
		{name: "the flush met a held path and kept the session", stands: true, end: func(t *testing.T, sourceSession *Session, target *fastPathTarget) {
			target.session.writeMu.Lock()
			sourceSession.appendFwdDirty(target.peer, target.session)
			sourceSession.flushFwdDirty()
			target.session.writeMu.Unlock()
			require.Len(t, sourceSession.fwdDirty, 1, "precondition: the flush did not keep the session")
		}},
		{name: "the source ended with no flush", stands: true, end: func(*testing.T, *Session, *fastPathTarget) {}},
	}
	followers := []struct {
		name  string
		write func(t *testing.T, source *Peer, target *fastPathTarget)
	}{
		{name: "an ordinary writer", write: func(t *testing.T, _ *Peer, target *fastPathTarget) {
			require.NoError(t, target.session.acquireWrite(fwdWriteDeadline()))
			err := target.session.writeRawUpdateBody(target.body[:])
			if err == nil {
				err = target.session.flushWrites()
			}
			target.session.releaseWrites()
			require.NoError(t, err)
			require.False(t, target.budgetStands(), "an ordinary writer's release clears the budget")
		}},
		{name: "a fast-path batch", write: func(t *testing.T, source *Peer, target *fastPathTarget) {
			require.Equal(t, fastPathBatchWrites, target.writeBatch(source, fastPathBatchWrites),
				"every write of the later batch must be delivered")
		}},
	}

	for _, ending := range endings {
		for _, follower := range followers {
			t.Run(ending.name+", then "+follower.name, func(t *testing.T) {
				source, sourceSession := newFastPathSource(t)
				target, _ := newFastPathTarget(t, "10.0.0.2", 65002)
				conn := target.record(time.Now)

				_, delivered := target.write(source)
				require.True(t, delivered, "precondition: the fast path delivered nothing")
				require.True(t, target.budgetStands(), "precondition: the fast path armed no budget")

				ending.end(t, sourceSession, target)
				require.Equal(t, ending.stands, target.budgetStands(),
					"precondition: this ending leaves the budget standing or cleared, as the case says")
				if ending.stands {
					target.session.writeMu.Lock()
					target.session.writeBudgetEnd = conn.age()
					target.session.writeMu.Unlock()
				}
				before := conn.seen()

				follower.write(t, source, target)

				after := conn.seen()
				require.Greater(t, after.writes, before.writes,
					"precondition: the later writer never reached the socket, so no deadline was judged")
				require.Equal(t, before.unbounded, after.unbounded, "a later write went out with no write deadline")
				require.Zero(t, after.expired, "a later write went out under a deadline that had already passed")
				require.NoError(t, closeCause(target.session), "the destination must keep its session")
			})
		}
	}
}

// TestFastPathBudgetAddsNoAllocation proves the steps the write budget added
// to the fast path allocate nothing, on a real TCP socket as on the recording
// one: arming, trusting a standing budget, and the release that keeps it.
//
// Method: testing.AllocsPerRun over exactly those steps, under the write path
// as tryDirectWriteNoFlush runs them. It measures the added steps, not the
// whole fast path, so its bound is absolute and needs no figure from before
// the change.
//
// VALIDATES: AC-11 and A-6 -- no allocation is added per forwarded UPDATE,
// whether the write arms the budget or finds it standing.
// PREVENTS: a deadline call or a clock read that reaches the heap on the
// hottest path.
func TestFastPathBudgetAddsNoAllocation(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0") //nolint:noctx // Test code
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })                   //nolint:errcheck // test cleanup
	dialed, err := net.Dial("tcp", listener.Addr().String()) //nolint:noctx // Test code
	require.NoError(t, err)
	t.Cleanup(func() { dialed.Close() }) //nolint:errcheck // test cleanup
	accepted, err := listener.Accept()
	require.NoError(t, err)
	t.Cleanup(func() { accepted.Close() }) //nolint:errcheck // test cleanup

	sockets := []struct {
		name string
		conn func(target *fastPathTarget) net.Conn
	}{
		{name: "recording socket", conn: func(target *fastPathTarget) net.Conn { return target.record(time.Now) }},
		{name: "tcp socket", conn: func(*fastPathTarget) net.Conn { return dialed }},
	}
	for _, socket := range sockets {
		t.Run(socket.name, func(t *testing.T) {
			target, _ := newFastPathTarget(t, "10.0.0.2", 65002)
			conn := socket.conn(target)
			session := target.session
			session.mu.Lock()
			session.writeMu.Lock()
			session.conn = conn
			session.writeMu.Unlock()
			session.mu.Unlock()
			budget := fwdWriteDeadline()

			armed := testing.AllocsPerRun(1000, func() {
				session.writeMu.Lock()
				session.writeBudgetArmed = false // as after another writer's release
				if armErr := session.armFastPathBudget(budget); armErr != nil {
					t.Error(armErr)
				}
				session.releaseFastPathWrites()
			})
			require.Zero(t, armed, "arming the fast path's budget must not allocate")
			require.True(t, target.budgetStands(), "precondition: the measured step armed no budget")

			standing := testing.AllocsPerRun(1000, func() {
				session.writeMu.Lock()
				if armErr := session.armFastPathBudget(budget); armErr != nil {
					t.Error(armErr)
				}
				session.releaseFastPathWrites()
			})
			require.Zero(t, standing, "a write that finds the budget standing must not allocate")
		})
	}
}

// BenchmarkDirectWriteBudget measures the fast-path write of one UPDATE to a
// healthy destination, for the allocation count AC-11 compares.
//
// "standing" is the steady state of a batch: the budget stands and the write
// arms nothing. "armed" is the worst case, a write that follows another
// writer's release every time. "unarmed" runs the same write on a session with
// no connection installed, where armWriteDeadline has nothing to arm: it is
// the write as it was before the budget, plus the clock read. The three
// allocs/op figures MUST be equal. The time figures are owed from a host whose
// performance is known, not from the development VM.
func BenchmarkDirectWriteBudget(b *testing.B) {
	cases := []struct {
		name    string
		prepare func(target *fastPathTarget)
		before  func(target *fastPathTarget)
	}{
		{name: "standing", prepare: func(*fastPathTarget) {}, before: func(*fastPathTarget) {}},
		{name: "armed", prepare: func(*fastPathTarget) {}, before: func(target *fastPathTarget) {
			target.session.writeBudgetArmed = false // as after another writer's release
		}},
		{name: "unarmed", prepare: func(target *fastPathTarget) {
			target.session.conn = nil // bufWriter keeps the socket; nothing is left to arm
		}, before: func(*fastPathTarget) {}},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			source, _ := newFastPathSource(b)
			target, _ := newFastPathTarget(b, "10.0.0.2", 65002)
			target.record(time.Now)
			tc.prepare(target)
			if _, delivered := target.write(source); !delivered {
				b.Fatal("precondition: the fast path delivered nothing")
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				tc.before(target)
				if _, delivered := target.write(source); !delivered {
					b.Fatal("the fast path stopped delivering")
				}
			}
		})
	}
}
