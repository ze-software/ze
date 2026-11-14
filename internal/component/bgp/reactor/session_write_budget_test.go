// Design: docs/architecture/core-design.md -- the write budget every session write carries
// Related: session_write_mutex.go -- the write path acquisition under test
// Related: session_write.go -- the writers, releaseWrites, sendUpdateHeld
// Related: session_wedge_test.go -- wedgeEstablished and the close-path tests

package reactor

import (
	"log/slog"
	"net"
	"net/netip"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/test/sim"
)

// budgetShort is the session write budget of the tests that wait one out.
const budgetShort = 200 * time.Millisecond

// setWriteBudget replaces the session write budget (ze.fwd.write.deadline) for
// one test. The value is a package global, so a test that calls this MUST NOT
// run in parallel with another.
func setWriteBudget(t *testing.T, budget time.Duration) {
	t.Helper()
	previous := fwdWriteDeadlineNs.Swap(int64(budget))
	t.Cleanup(func() { fwdWriteDeadlineNs.Store(previous) })
}

// budgetReader is the peer end of the pipe: it reads until told to stop, or
// until the pipe dies, and keeps every octet ze sent after its OPEN.
type budgetReader struct {
	client net.Conn
	done   chan struct{}
	// pauseNs is how long the reader waits before each read, in nanoseconds. A
	// net.Pipe write completes only when it is read, so this is how long each of
	// ze's flushes takes.
	pauseNs atomic.Int64

	mu     sync.Mutex
	stream []byte
}

// run is the reader's loop. It ends on the first failed read: stop's expired
// read deadline, ze's close, or the fixture's cleanup.
func (r *budgetReader) run() {
	defer close(r.done)
	buf := make([]byte, 4096)
	for {
		if pause := time.Duration(r.pauseNs.Load()); pause > 0 {
			time.Sleep(pause)
		}
		n, err := r.client.Read(buf)
		r.mu.Lock()
		r.stream = append(r.stream, buf[:n]...)
		r.mu.Unlock()
		if err != nil {
			return
		}
	}
}

// stop makes the peer end stop reading for good, with the connection left
// open: from here every write ze makes to it blocks.
func (r *budgetReader) stop(t *testing.T) {
	t.Helper()
	require.NoError(t, r.client.SetReadDeadline(time.Unix(1, 0)))
	select {
	case <-r.done:
	case <-time.After(runExitDeadline):
		t.Fatal("precondition: the peer end did not stop reading")
	}
}

// updates returns the length of each complete UPDATE the peer end has read, in
// arrival order. RFC 4271 Section 4.1: the header is a 16-octet marker, a
// 2-octet length and a 1-octet type.
func (r *budgetReader) updates() []int {
	r.mu.Lock()
	defer r.mu.Unlock()

	var lengths []int
	rest := r.stream
	for len(rest) >= message.HeaderLen {
		length := int(rest[16])<<8 | int(rest[17])
		if length < message.HeaderLen || length > len(rest) {
			break
		}
		if rest[18] == byte(msgtype.TypeUPDATE) {
			lengths = append(lengths, length)
		}
		rest = rest[length:]
	}
	return lengths
}

// budgetPeer is one Established session whose peer end the test controls.
type budgetPeer struct {
	session   *Session
	reader    *budgetReader
	runResult <-chan error
}

// budgetEstablished drives a real Session to Established over a net.Pipe with
// Run executing, as wedgeEstablished does, with a peer end the test can pause
// or stop. The FSM timers run on a fake clock that nothing advances, so no
// KEEPALIVE is written behind the test's back; the session's own clock is the
// real one, because a net.Pipe deadline is judged against real time.
func budgetEstablished(t *testing.T) *budgetPeer {
	t.Helper()

	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.Connection = ConnectionPassive
	settings.ReceiveHoldTime = wedgeHold
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
	}

	session := NewSession(settings)
	session.timers.SetClock(sim.NewFakeClock(time.Now()))
	require.NoError(t, session.Start())

	server, client := net.Pipe()
	_ = acceptWithReader(t, session, server, client)
	reader := &budgetReader{client: client, done: make(chan struct{})}
	go reader.run()

	runResult := rfc9687Run(t, session)

	go func() { _, _ = client.Write(rfc9687PeerOpen(uint16(wedgeHold / time.Second))) }()
	require.Eventually(t, func() bool { return session.State() == fsm.StateOpenConfirm },
		runExitDeadline, 5*time.Millisecond,
		"precondition: ze did not reach OpenConfirm, so the peer OPEN was not accepted")

	go func() { _, _ = client.Write(message.PackTo(message.NewKeepalive(), nil)) }()
	require.Eventually(t, func() bool { return session.State() == fsm.StateEstablished },
		runExitDeadline, 5*time.Millisecond,
		"precondition: the session never reached Established, so there is nothing to write to")

	return &budgetPeer{session: session, reader: reader, runResult: runResult}
}

// budgetAnnounce is a well-formed announcement of 198.51.<third>.0/24. Each
// test message names its own prefix, so none is a duplicate the session
// suppresses.
func budgetAnnounce(third byte) *message.Update {
	return &message.Update{
		PathAttributes: []byte{0x40, 1, 1, 0, 0x40, 2, 0, 0x40, 3, 4, 192, 0, 2, 99},
		NLRI:           []byte{24, 198, 51, third},
	}
}

// TestUnboundedWritersAreBounded proves every writer the census of
// spec-bgp-session-write-wedge marked unbounded now gives up. One sub-test per
// writer: the session is Established over a pipe, the peer end stops reading
// with the connection open, and the writer is called with a 200 ms budget.
//
// VALIDATES: AC-3 -- the write returns a deadline error once its budget has
// passed, the session is retired with that error, and Run returns.
// PREVENTS: a writer blocked in the socket for as long as the peer keeps its
// window shut, holding the write path every close waits on.
func TestUnboundedWritersAreBounded(t *testing.T) {
	route := bgptypes.RouteSpec{
		Prefix:  netip.MustParsePrefix("198.51.100.0/24"),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.0.2.99")),
	}
	body := message.PackTo(budgetAnnounce(100), nil)[message.HeaderLen:]
	refresh := &message.RouteRefresh{AFI: 1, SAFI: 1, Subtype: message.RouteRefreshNormal}

	writers := []struct {
		name  string
		write func(s *Session) error
	}{
		{"SendUpdate", func(s *Session) error { return s.SendUpdate(budgetAnnounce(100)) }},
		{"sendUpdateHeld", func(s *Session) error {
			s.holdWrites()
			defer s.releaseWrites()
			return s.sendUpdateHeld(budgetAnnounce(100))
		}},
		{"SendAnnounce", func(s *Session) error {
			return s.SendAnnounce(route, netip.Addr{}, 65001, false, true, false)
		}},
		{"sendWithdraw", func(s *Session) error {
			return s.sendWithdraw(netip.MustParsePrefix("198.51.100.0/24"), false)
		}},
		{"sendRawUpdateBody", func(s *Session) error { return s.sendRawUpdateBody(body) }},
		{"SendRawMessage-packet", func(s *Session) error {
			return s.SendRawMessage(0, message.PackTo(refresh, nil))
		}},
		{"SendRawMessage-update", func(s *Session) error {
			return s.SendRawMessage(uint8(msgtype.TypeUPDATE), body)
		}},
		{"SendRawMessage-refresh", func(s *Session) error {
			return s.SendRawMessage(uint8(msgtype.TypeROUTEREFRESH), []byte{0, 1, 0, 1})
		}},
	}

	for _, writer := range writers {
		t.Run(writer.name, func(t *testing.T) {
			setWriteBudget(t, budgetShort)
			p := budgetEstablished(t)
			p.reader.stop(t)

			started := time.Now()
			written := make(chan error, 1)
			go func() { written <- writer.write(p.session) }()

			select {
			case err := <-written:
				require.ErrorIs(t, err, os.ErrDeadlineExceeded,
					"the writer must fail on its write budget; nil means it wrote nothing and proved nothing")
				require.GreaterOrEqual(t, time.Since(started), budgetShort/2,
					"precondition: the writer never blocked on the peer end, so no budget was waited out")
			case <-time.After(budgetShort + wedgeSlack):
				t.Fatal("the writer was still blocked " + (budgetShort + wedgeSlack).String() +
					" after it started: its write carries no budget")
			}

			select {
			case <-p.runResult:
			case <-time.After(runExitDeadline):
				t.Fatal("Run did not return within " + runExitDeadline.String() +
					" of the write passing its budget: the session was not retired")
			}
			p.session.writeMu.Lock()
			retiredBy := p.session.writeFailed
			p.session.writeMu.Unlock()
			require.ErrorIs(t, retiredBy, os.ErrDeadlineExceeded,
				"the session must be retired by the write that passed its budget")
		})
	}
}

// TestWriteBudgetDoesNotLeak proves a released acquisition leaves no write
// deadline on the socket. One writer sends through the acquisition with a
// 200 ms budget, the test lets that budget pass, then takes writeMu by hand,
// the way the route-server fast path does, and writes with no budget of its
// own: a deadline the first writer left armed would fail that write.
//
// VALIDATES: R-1 -- the release clears the deadline the acquisition armed.
// PREVENTS: a healthy session retired with an i/o timeout by the budget of a
// write that completed long before.
func TestWriteBudgetDoesNotLeak(t *testing.T) {
	setWriteBudget(t, budgetShort)
	p := budgetEstablished(t)

	require.NoError(t, p.session.SendUpdate(budgetAnnounce(100)))

	// The first writer's budget has to be in the past for a leak to show.
	time.Sleep(2 * budgetShort)

	p.session.writeMu.Lock()
	err := p.session.writeRawUpdateBody(message.PackTo(budgetAnnounce(101), nil)[message.HeaderLen:])
	if err == nil {
		err = p.session.flushWrites()
	}
	p.session.writeMu.Unlock()

	require.NoError(t, err, "a write after a released acquisition met a stale write deadline")
	require.Eventually(t, func() bool { return len(p.reader.updates()) == 2 },
		runExitDeadline, 5*time.Millisecond, "both UPDATEs must reach the peer")
	require.Equal(t, fsm.StateEstablished, p.session.State())
}

// TestInitialSyncOutlastsBudget proves the write budget bounds one message, not
// a hold. It makes the calls the held phases of Peer.sendInitialRoutes make
// (holdWrites, one sendUpdateHeld per route, then the End-of-RIB, then
// releaseWrites) against a peer that reads every message but takes 100 ms over
// each, with a 500 ms budget: the hold lasts longer than the budget in total
// while no single flush comes near it.
//
// VALIDATES: AC-7 -- every route and the End-of-RIB reach the peer and the
// session stays Established.
// PREVENTS: a budget armed once per hold, which drops a healthy peer in the
// middle of a large initial sync (R-2).
func TestInitialSyncOutlastsBudget(t *testing.T) {
	const (
		budget     = 500 * time.Millisecond
		readPause  = 100 * time.Millisecond
		routeCount = 8
	)
	setWriteBudget(t, budget)
	p := budgetEstablished(t)
	p.reader.pauseNs.Store(int64(readPause))

	started := time.Now()
	func() {
		p.session.holdWrites()
		defer p.session.releaseWrites()

		for i := range routeCount {
			require.NoError(t, p.session.sendUpdateHeld(budgetAnnounce(byte(100+i))),
				"route %d of the held phase was cut", i)
		}
		// RFC 4724 Section 2: the End-of-RIB marker for IPv4 unicast is an
		// UPDATE with no withdrawn routes, attributes or NLRI.
		require.NoError(t, p.session.sendUpdateHeld(&message.Update{}), "the End-of-RIB was cut")
	}()
	require.Greater(t, time.Since(started), budget,
		"precondition: the hold did not outlast the budget, so it proves nothing about a long sync")

	require.Eventually(t, func() bool { return len(p.reader.updates()) == routeCount+1 },
		runExitDeadline, 5*time.Millisecond, "every route and the End-of-RIB must reach the peer")
	lengths := p.reader.updates()
	require.Equal(t, message.HeaderLen+4, lengths[routeCount], "the last UPDATE must be the End-of-RIB")

	require.Equal(t, fsm.StateEstablished, p.session.State())
	select {
	case err := <-p.runResult:
		t.Fatalf("the session ended during a healthy sync: %v", err)
	default: // Run is still serving the session.
	}
}

// TestKeepaliveGivesUpOnBusyWritePath proves the KEEPALIVE timer callback does
// not park behind a writer that keeps the write path. The session is
// Established, the test holds writeMu the way a stuck writer does, and the
// production OnKeepaliveTimerExpires callback is fired through the timers'
// clock. The wait it is allowed is the control budget, ten seconds here, so
// this test takes that long.
//
// VALIDATES: R-8 -- the callback returns within the control budget with the
// path still held, logs that the KEEPALIVE was not sent, and leaves the session
// Established and its connection open.
// PREVENTS: one goroutine parked for good per keepalive interval on a session
// whose peer stopped reading.
func TestKeepaliveGivesUpOnBusyWritePath(t *testing.T) {
	logs := captureSessionLog(t, slog.LevelDebug)
	p := wedgeEstablished(t, 0)
	require.True(t, p.session.timers.IsKeepaliveTimerRunning(),
		"precondition: the keepalive timer must be armed to expire")

	// The keepalive timer is the earliest one on the timers' clock: the hold
	// timer is three times further away.
	next, pending := p.timerClock.NextTimer()
	require.True(t, pending, "precondition: no timer is pending on the timers' clock")

	release := holdWritePath(p.session)
	defer release()

	fired := make(chan struct{})
	go func() {
		defer close(fired)
		p.timerClock.Add(next.Sub(p.timerClock.Now()))
	}()
	joinOnCleanup(t, fired, "the keepalive timer callback")

	budget := p.session.controlWriteDeadline()
	select {
	case <-fired:
	case <-time.After(budget + wedgeSlack):
		t.Fatal("the keepalive callback was still parked " + (budget + wedgeSlack).String() +
			" after it fired: it waits on the write path without a bound")
	}

	require.Contains(t, logs.String(), "keepalive send failed",
		"precondition: the callback that returned must be the keepalive one")
	require.Contains(t, logs.String(), errWritePathBusy.Error(),
		"the log must say the KEEPALIVE was not sent because the write path was busy")
	// The test still holds the write path, so writeFailed is its to read.
	require.NoError(t, p.session.writeFailed, "giving up on a busy write path must not retire the session")
	require.Equal(t, fsm.StateEstablished, p.session.State())
	select {
	case err := <-p.peerEnded:
		t.Fatalf("the connection was closed by a KEEPALIVE that only gave up: %v", err)
	default: // The connection is still open.
	}
}
