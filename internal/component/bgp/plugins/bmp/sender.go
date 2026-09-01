// RFC: rfc/short/rfc7854.md
// Design: docs/architecture/core-design.md -- BMP sender (outbound to collectors)
//
// Related: bmp.go -- plugin lifecycle, config parsing
// Related: bmp_events.go -- the reactor events written to these sessions
// Related: msg.go -- BMP message encoding
// Detail: sender_drain.go -- enqueue (producer side) and the drain goroutine

package bmp

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/network"
)

var (
	errNotConnected = errors.New("not connected")

	// errQueueOverflow is returned to a producer whose message did not fit in
	// the session's transmit queue. The message is NOT queued and the session
	// is being reset; the producer must not retry, and must not block.
	errQueueOverflow = errors.New("transmit queue overflow, session reset")
)

// RFC 7854 suggested reconnection intervals.
const (
	reconnectMin = 30 * time.Second
	reconnectMax = 720 * time.Second
	writeTimeout = 10 * time.Second

	// terminationWait bounds how long a teardown waits for an in-flight socket
	// write before giving up on sending the final Termination message. Short:
	// past it the collector is provably not reading.
	terminationWait = 1 * time.Second
	// flushPollInterval is the retry granularity of that wait.
	flushPollInterval = 2 * time.Millisecond
)

// senderSession manages a single outbound TCP connection to a BMP collector.
//
// Caller MUST call stop() to shut down the session goroutine, then
// wait on the WaitGroup that tracks it. run() waits for the drain goroutine
// before returning, so that WaitGroup covers both.
//
// Transmit path: producers never touch the socket. A write* method encodes into
// `scratch` and COPIES the finished message into the session's bounded transmit
// queue (txqueue.go); the drain goroutine started on first use is the only
// writer of queued bytes to the socket. That is what keeps a wedged collector
// from stalling a producer -- in particular the RIB best-change publisher, whose
// EventBus handler "MUST NOT block on I/O" (pkg/ze/eventbus.go).
//
// Scratch usage: writePeerUp/Down/RouteMonitoring/Mirroring/StatisticsReport all
// encode into `scratch` and then hand it to the queue. More than one
// goroutine reaches those methods:
//
//   - the bmp plugin's own delivery loop (RunBMPPlugin's OnStructuredEvent ->
//     handleStructuredEvent -> handleSenderState / route monitoring), and
//   - for RFC 9069 Loc-RIB monitoring, whichever goroutine publishes a RIB
//     best-change. Engine EventBus subscribers fire synchronously on the
//     PUBLISHER's goroutine (see SubscribeEngineEvent in
//     internal/component/plugin/server/engine_event.go), so bmp_locrib.go's
//     handleBestChange runs on the RIB plugin's delivery goroutine.
//
// writeMu therefore covers the whole encode-then-enqueue, not just the encode:
// without it two producers interleave in the one scratch array and a message
// goes out whose Common Header length does not describe its content, which
// desynchronizes the collector's framing for every message after it. Holding it
// across the copy into the transmit queue keeps each BMP message contiguous in
// the queue, and the single drain goroutine keeps it contiguous on the wire.
type senderSession struct {
	name          string
	address       string
	port          uint16
	sourceAddress string

	conn   net.Conn
	connMu sync.Mutex

	stopCh  chan struct{}
	stopCtx context.Context
	cancel  context.CancelFunc

	// writeMu guards scratch AND the order messages enter the transmit queue.
	// Every method that encodes into scratch or enqueues MUST hold it.
	writeMu sync.Mutex
	scratch []byte

	// flushMu serializes the actual socket writes. Only the drain goroutine and
	// the session's own goroutine (Initiation, Termination) write, and they must
	// not interleave halves of two BMP messages on the wire.
	flushMu sync.Mutex

	// stopOnce guards stop(), which two teardown paths can reach (a config
	// reload restarting the senders, and plugin shutdown). termOnce keeps the
	// session's final Termination to exactly one however many of those paths
	// race for it.
	stopOnce sync.Once
	termOnce sync.Once

	// drainMu guards txq/drainStarted/drainDone: the drain goroutine's whole
	// lifecycle. ensureDrain starts it on first use, run() waits for it.
	drainMu      sync.Mutex
	txq          *txQueue
	drainStarted bool
	drainDone    chan struct{}

	// txLimit is the transmit queue's byte bound. Zero means txQueueLimitBytes.
	txLimit int

	// retryWait is the base reconnection delay (RFC 7854 Section 3.2 suggests
	// backing off rather than redialing in a tight loop). Zero means
	// reconnectMin. A field rather than the bare constant so a test can drive a
	// full disconnect -> reconnect cycle without waiting out production timing.
	retryWait time.Duration

	// onPrimed, when set, runs on the session goroutine with writeMu HELD, in
	// the same critical section that publishes the connection. Whatever it
	// enqueues is therefore guaranteed to be the first thing on the new BMP
	// session after Initiation: every producer takes writeMu before it can
	// enqueue, so none can slip a Route Monitoring in front of the Peer Ups this
	// replays. It MUST use the *Locked write helpers and MUST NOT re-enter the
	// producer path.
	onPrimed func()

	// onConnected, when set, runs on the session goroutine after priming, with
	// no lock held. It is for work that re-enters the producer path -- the RFC
	// 9069 Loc-RIB dump request, whose replay batches call back into write*.
	onConnected func()

	// locRIBUpSent records whether the RFC 9069 Loc-RIB Peer Up has been sent on
	// the CURRENT connection. Cleared when the connection ends, because a new
	// BMP session starts with no state at the collector.
	locRIBUpSent atomic.Bool
}

// newSenderSession creates a sender session for the given collector.
func newSenderSession(name string, cfg collectorConfig) *senderSession {
	ctx, cancel := context.WithCancel(context.Background())
	return &senderSession{
		name:          name,
		address:       cfg.Address,
		port:          parseUint16(cfg.Port, DefaultPort),
		sourceAddress: cfg.SourceAddress,
		stopCh:        make(chan struct{}),
		stopCtx:       ctx,
		cancel:        cancel,
		txLimit:       txQueueLimitBytes,
		retryWait:     reconnectMin,
		// maxBMPMsgSize (65535) is the RFC 7854 ceiling. Allocating
		// this once per collector session keeps the BGP-UPDATE → BMP
		// Route Monitoring hot path allocation-free.
		scratch: make([]byte, maxBMPMsgSize),
	}
}

// targets reports whether this session already connects to what cfg names. A
// session that does keeps running across a config reload, so the collector on
// the far end keeps its BMP session and everything it has learned on it.
//
// The three fields compared are the whole of what decides the connection: the
// destination, the port, and the source address the dial binds. Everything else
// in a collectorConfig is the map key, which the caller matches on first.
func (ss *senderSession) targets(cfg collectorConfig) bool {
	if ss.address != cfg.Address {
		return false
	}
	if ss.port != parseUint16(cfg.Port, DefaultPort) {
		return false
	}
	return ss.sourceAddress == cfg.SourceAddress
}

// run is the long-lived goroutine for the sender session.
// It connects to the collector, sends the Initiation message,
// and enters a loop that reconnects on failure.
func (ss *senderSession) run() {
	defer ss.cancel()
	// The drain goroutine outlives no session: run() is the goroutine the
	// plugin's WaitGroup tracks, so it is the one that must not return until
	// the drain has exited.
	defer ss.waitDrain()
	addr := net.JoinHostPort(ss.address, strconv.Itoa(int(ss.port)))
	reconnectWait := ss.retryBase()

	for {
		if ss.isStopping() {
			return
		}

		logger().Info("bmp: sender connecting", "collector", ss.name, "address", addr)
		dialer := &network.RealDialer{Timeout: 10 * time.Second}
		if err := dialer.SetSourceAddress(ss.sourceAddress); err != nil {
			// Unreachable via YANG-validated config; fall back to OS-selected
			// source rather than never connecting.
			logger().Warn("bmp: invalid source-address, using OS default", "collector", ss.name, "error", err)
		}
		conn, err := dialer.DialContext(ss.stopCtx, "tcp", addr)
		if err != nil {
			if ss.isStopping() {
				return
			}
			logger().Warn("bmp: sender connect failed", "collector", ss.name, "error", err)
			if ss.waitOrStop(reconnectWait) {
				return
			}
			reconnectWait = min(reconnectWait*2, reconnectMax)
			continue
		}

		reconnectWait = ss.retryBase()
		logger().Info("bmp: sender connected", "collector", ss.name, "address", addr)

		// RFC 7854 Section 4.3: Initiation MUST be the first message on a new BMP
		// session. Publishing ss.conn is what makes this session reachable by the
		// concurrent producers in enqueueLocked, so it happens only AFTER Initiation
		// is on the wire -- otherwise a Peer Up or a Loc-RIB Route Monitoring
		// racing in from another goroutine reaches the collector first.
		//
		// A producer arriving while Initiation is in flight sees the nil conn and
		// returns errNotConnected immediately; it does NOT wait. sendInitiation
		// deliberately does not hold writeMu (see its doc comment) -- holding the
		// producers' lock across a socket write bounded only by writeTimeout is
		// exactly the stall the transmit queue exists to remove. Nothing is lost
		// by the drop: the publish below is immediately followed by priming the
		// session with a Peer Up for every established peer.
		//
		// Also note stop() cannot abort an in-flight Initiation: it closes
		// ss.conn, which is still nil for this window.
		if err := ss.sendInitiation(conn); err != nil {
			logger().Warn("bmp: sender initiation failed", "collector", ss.name, "error", err)
			closeLog(conn, "sender-init-fail")
			// Back off before redialing. Without this a collector that accepts the
			// TCP connection and then rejects the Initiation produces a tight
			// dial -> write -> close spin: reconnectWait was reset to reconnectMin
			// above and this branch never waits.
			if ss.waitOrStop(reconnectWait) {
				return
			}
			reconnectWait = min(reconnectWait*2, reconnectMax)
			continue
		}

		// A BMP session carries no state across a TCP connection: the collector
		// that just accepted knows nothing about the peers that came up while it
		// was away. Publishing the connection and queueing the Peer Ups happen
		// under ONE writeMu critical section, so no producer can enqueue a Route
		// Monitoring for a peer before that peer's Peer Up (RFC 7854 Section
		// 4.10 ordering). Neither step can block on the socket: publishing is a
		// pointer store and the Peer Ups go into the transmit queue.
		ss.writeMu.Lock()
		ss.connMu.Lock()
		ss.conn = conn
		ss.connMu.Unlock()
		if ss.onPrimed != nil {
			ss.onPrimed()
		}
		ss.writeMu.Unlock()

		// Then the work that re-enters the producer path (and so must not hold
		// writeMu): the full fresh Loc-RIB dump.
		if ss.onConnected != nil {
			ss.onConnected()
		}

		// Hold connection open until stopped or error.
		ss.holdConnection(conn)

		// Clear conn so concurrent producers see nil (not a closed conn), then
		// drop whatever is still queued: those messages belong to the BMP
		// session that just ended, and the next connection starts with a fresh
		// Initiation and a fresh dump (BIRD frees its whole TX queue the same
		// way on session down, proto/bmp/bmp.c:1197-1215).
		ss.clearConn()
		ss.resetQueue()
		ss.locRIBUpSent.Store(false)

		// Back off before redialing. Without this a session that is reset by a
		// transmit-queue overflow, or a collector that accepts and immediately
		// closes, produces a tight dial -> dump -> reset spin that hammers both
		// ends. RFC 7854 Section 3.2 asks for a backoff rather than a busy retry.
		if ss.waitOrStop(reconnectWait) {
			return
		}
	}
}

// retryBase returns the base reconnection delay, falling back to the RFC 7854
// suggested minimum when the field was never set (struct-literal test fixtures).
func (ss *senderSession) retryBase() time.Duration {
	if ss.retryWait <= 0 {
		return reconnectMin
	}
	return ss.retryWait
}

// clearConn sets the conn field to nil under lock.
func (ss *senderSession) clearConn() {
	ss.connMu.Lock()
	ss.conn = nil
	ss.connMu.Unlock()
}

// clearConnAndResetIf clears the conn field and drops the transmit queue, both
// only when the conn is still c.
//
// The two steps are ONE connMu critical section on purpose. Done separately --
// clearConnIf returning true, then q.reset() -- the window between them is a
// check-then-act: run() can publish the next connection and prime it with a
// Peer Up for every established peer in that gap, and the reset then silently
// discards those primed Peer Ups. The collector afterwards receives Route
// Monitoring for peers it was never told about (RFC 7854 Section 4.10) with
// nothing logged and nothing to recover it. The window is small enough that it
// has never been observed, which is exactly why it must be closed structurally
// rather than left to timing.
//
// Lock order is connMu then q.mu, matching every other two-lock path in this
// package (writeMu -> {connMu, drainMu, flushMu} -> q.mu); it never runs with
// writeMu held from the drain side, and the producer side takes writeMu first.
func (ss *senderSession) clearConnAndResetIf(c net.Conn, q *txQueue) {
	ss.connMu.Lock()
	defer ss.connMu.Unlock()

	if ss.conn != c {
		return
	}
	ss.conn = nil
	if q != nil {
		q.reset()
	}
}

// sendInitiation sends a BMP Initiation message to the collector.
//
// It deliberately does NOT take writeMu. writeMu is the producers' lock, and
// this is a blocking socket write bounded only by writeTimeout: holding it here
// would stall the RIB best-change publisher for up to ten seconds on every
// reconnect against a collector that accepts TCP and then stops reading --
// exactly the failure the transmit queue exists to remove. Contiguity on the
// wire comes from flushMu (inside writeRaw), and ordering comes from run()
// publishing ss.conn only after this returns: a producer arriving meanwhile
// takes writeMu, sees a nil conn and returns errNotConnected immediately
// instead of waiting. Nothing is lost by that -- run() primes the new session
// with a Peer Up for every established peer as soon as the conn is published.
func (ss *senderSession) sendInitiation(conn net.Conn) error {
	init := &Initiation{
		TLVs: []TLV{
			makeStringTLV(InitTLVSysName, "ze"),
			makeStringTLV(InitTLVSysDescr, "ze BGP daemon"),
		},
	}

	// Size: common header(6) + sysName TLV(4+2) + sysDescr TLV(4+14) = 30.
	// Fixed compile-time size — a size-constant regression fails to compile.
	// net.Conn.Write is an interface call so escape analysis moves this to
	// heap today; if the write path ever becomes a concrete type or a
	// provided scratch buffer, the same code path stays on the stack.
	var stack [CommonHeaderSize + TLVHeaderSize + 2 + TLVHeaderSize + 14]byte
	buf := stack[:]
	n := writeInitiation(buf, 0, init)
	return ss.writeRaw(conn, buf[:n])
}

// sendTermination sends the RFC 7854 Section 4.5 Termination message. It waits
// for the socket-write lock rather than the scratch lock: Termination is built
// on the stack and goes straight to the socket, so what it must not do is
// interleave with a drain write, not with a producer's encode.
//
// It gives up after terminationWait rather than waiting out an in-flight write:
// past that the collector is not reading, so the Termination would not reach it
// either, and every other collector's teardown is queued behind this one.
func (ss *senderSession) sendTermination(conn net.Conn) {
	if !ss.acquireFlush(terminationWait) {
		logger().Debug("bmp: skipping termination, collector write still in flight",
			"collector", ss.name)
		return
	}
	defer ss.flushMu.Unlock()
	ss.writeTerminationLocked(conn)
}

// terminateAndClose ends a BMP session the way RFC 7854 Section 4.5 asks: a
// Termination message, then the TCP close. Exactly one Termination is sent per
// session however many teardown paths race (stop() and holdConnection both
// call this).
func (ss *senderSession) terminateAndClose(conn net.Conn, why string) {
	ss.termOnce.Do(func() { ss.sendTermination(conn) })
	closeLog(conn, why)
}

// acquireFlush takes the socket-write lock, giving up after d.
func (ss *senderSession) acquireFlush(d time.Duration) bool {
	deadline := time.Now().Add(d)
	for {
		if ss.flushMu.TryLock() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(flushPollInterval)
	}
}

// writeTerminationLocked writes the Termination message.
//
// It uses terminationWait rather than writeTimeout as the write deadline: this
// is the last thing a dying session does, and a collector that cannot take 23
// bytes in a second is not going to take them in ten. Without the shorter
// deadline one wedged collector adds writeTimeout to plugin shutdown.
//
// Caller MUST hold flushMu.
func (ss *senderSession) writeTerminationLocked(conn net.Conn) {
	term := &Termination{
		TLVs: []TLV{
			makeStringTLV(TermTLVString, "shutting down"),
		},
	}

	// Size: common header(6) + TLV(4+13) = 23.
	// Fixed compile-time size — a size-constant regression fails to compile.
	// Escapes via net.Conn.Write today; pattern holds if the write path ever
	// becomes a concrete type.
	var stack [CommonHeaderSize + TLVHeaderSize + 13]byte
	buf := stack[:]
	n := writeTermination(buf, 0, term)
	if err := ss.writeDeadlineLocked(conn, buf[:n], terminationWait); err != nil {
		logger().Debug("bmp: sender termination write failed", "collector", ss.name, "error", err)
	}
}

// writeRaw writes data to a connection with a write deadline.
//
// It is the ONLY place BMP bytes reach a socket, and it holds flushMu for the
// whole call so the drain goroutine and the session goroutine (Initiation,
// Termination) can never interleave halves of two messages. The deadline
// applies per call, and the drain never hands it more than one queue page, so a
// slow-but-alive collector is not mistaken for a wedged one.
func (ss *senderSession) writeRaw(conn net.Conn, data []byte) error {
	ss.flushMu.Lock()
	defer ss.flushMu.Unlock()
	return ss.writeRawLocked(conn, data)
}

// writeRawLocked writes data to a connection with the standard write deadline.
// Caller MUST hold flushMu.
func (ss *senderSession) writeRawLocked(conn net.Conn, data []byte) error {
	return ss.writeDeadlineLocked(conn, data, writeTimeout)
}

// writeDeadlineLocked writes data to a connection, bounded by deadline.
// Caller MUST hold flushMu.
func (ss *senderSession) writeDeadlineLocked(conn net.Conn, data []byte, deadline time.Duration) error {
	if err := conn.SetWriteDeadline(time.Now().Add(deadline)); err != nil {
		return err
	}
	_, err := conn.Write(data)
	return err
}

// holdConnection blocks until stopCh is closed or the connection errors.
// It reads from the connection to detect remote close (BMP is unidirectional
// router->collector, but the collector might close the TCP).
// Termination is sent from this goroutine only, avoiding the stop/write race.
func (ss *senderSession) holdConnection(conn net.Conn) {
	var discardArr [1]byte
	discard := discardArr[:]
	for {
		if ss.isStopping() {
			ss.terminateAndClose(conn, "sender-hold-stop")
			return
		}

		if err := conn.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
			closeLog(conn, "sender-hold-deadline")
			return
		}
		_, err := conn.Read(discard)
		if err != nil {
			if ss.isStopping() {
				ss.terminateAndClose(conn, "sender-hold-stop")
				return
			}
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				continue // read deadline expired, connection still alive
			}
			logger().Info("bmp: sender connection lost", "collector", ss.name, "error", err)
			closeLog(conn, "sender-hold-lost")
			return
		}
	}
}

// stop signals the session goroutine to exit. Idempotent.
//
// It also ENDS the BMP session properly: RFC 7854 Section 4.5 requires a
// Termination message when the session is closed, and stop() is the path that
// closes the socket, so stop() is the path that must send it. Leaving it to
// holdConnection did not work -- by the time holdConnection's blocked Read
// returned, the socket stop() had just closed could no longer carry anything,
// so no collector ever received a Termination on shutdown.
func (ss *senderSession) stop() {
	ss.stopOnce.Do(func() {
		// Give the drain a bounded chance to write what is already queued. The
		// plugin's own teardown enqueues the RFC 9069 Loc-RIB Peer Down just
		// before calling this (RunBMPPlugin's defer), and closing stopCh first
		// would throw it away: the drain checks stopCh before it checks the
		// queue. Bounded, because a collector that has not drained in a second
		// is not going to.
		ss.flushPending(terminationWait)

		close(ss.stopCh)
		if ss.cancel != nil {
			ss.cancel() // cancel dial context
		}

		// Wake a drain parked waiting for bytes so it observes stopCh.
		if q := ss.queue(); q != nil {
			q.wake()
		}

		ss.connMu.Lock()
		c := ss.conn
		ss.connMu.Unlock()

		if c != nil {
			// Termination, then the close that unblocks holdConnection's Read.
			ss.terminateAndClose(c, "sender-stop")
		}

		// Publish the disconnect LAST. Without it ss.conn kept pointing at the
		// closed socket, so enqueueLocked's nil check passed and a producer that
		// arrived after stop() was told its message was queued -- onto a queue
		// whose drain has already exited and will never write it. Nilling here
		// makes those producers get errNotConnected, which is the truth.
		ss.clearConn()
	})
}

// flushPending waits, for at most d, until the transmit queue is empty. Used on
// teardown so the last messages a session was given still reach the collector.
func (ss *senderSession) flushPending(d time.Duration) {
	q := ss.queue()
	if q == nil {
		return
	}
	deadline := time.Now().Add(d)
	for q.bytesPending() > 0 {
		if time.Now().After(deadline) {
			logger().Debug("bmp: session stopped with unwritten messages",
				"collector", ss.name, "queued-bytes", q.bytesPending())
			return
		}
		// Poll interval; the loop returns as soon as the drain reports empty.
		time.Sleep(flushPollInterval)
	}
}

// isStopping returns true if stopCh has been closed.
func (ss *senderSession) isStopping() bool {
	select {
	case <-ss.stopCh:
		return true
	default: // active
		return false
	}
}

// waitOrStop sleeps for d or returns true if stopCh fires first.
func (ss *senderSession) waitOrStop(d time.Duration) bool {
	select {
	case <-ss.stopCh:
		return true
	case <-time.After(d):
		return false
	}
}

// scratchFor returns ss.scratch sliced to need bytes. Lazily allocates
// the maxBMPMsgSize buffer on first use so test fixtures that build a
// senderSession via struct literal (no newSenderSession) work without
// extra setup. Returns an error if need exceeds maxBMPMsgSize so the
// caller skips the write rather than truncating.
//
// Caller MUST hold writeMu: the returned slice aliases the one buffer every
// producer encodes into, and MUST stay owned until the matching enqueueLocked
// has copied it into the transmit queue.
func (ss *senderSession) scratchFor(need int) ([]byte, error) {
	if ss.scratch == nil {
		ss.scratch = make([]byte, maxBMPMsgSize)
	}
	if need > len(ss.scratch) {
		return nil, fmt.Errorf("bmp: message exceeds max size (%d > %d)", need, len(ss.scratch))
	}
	return ss.scratch[:need], nil
}

// writePeerUp encodes and sends the Peer Up of a MONITORED BGP peer, which
// carries no Peer Up Information TLV.
//
// The RFC 9069 Loc-RIB instance peer carries one, the VRF/Table Name, and its
// two producers take writeMu themselves so they call writePeerUpLocked directly
// (bmp_locrib.go ensureLocRIBPeerUp and primeLocRIBPeerUp).
func (ss *senderSession) writePeerUp(peer PeerHeader, localAddr [16]byte, localPort, remotePort uint16, sentOpen, recvOpen []byte) error {
	ss.writeMu.Lock()
	defer ss.writeMu.Unlock()
	return ss.writePeerUpLocked(peer, localAddr, localPort, remotePort, sentOpen, recvOpen, nil)
}

// writePeerUpLocked is writePeerUp for a caller that already holds writeMu --
// the session's own priming step (run() -> onPrimed), which queues the Peer Ups
// of every established peer in the same critical section that publishes the
// connection so nothing can precede them.
//
// Caller MUST hold writeMu.
func (ss *senderSession) writePeerUpLocked(peer PeerHeader, localAddr [16]byte, localPort, remotePort uint16, sentOpen, recvOpen []byte, infoTLVs []TLV) error {
	pu := &PeerUp{
		Peer:            peer,
		LocalAddress:    localAddr,
		LocalPort:       localPort,
		RemotePort:      remotePort,
		SentOpenMsg:     sentOpen,
		ReceivedOpenMsg: recvOpen,
		InfoTLVs:        infoTLVs,
	}

	tlvBytes := 0
	for i := range infoTLVs {
		tlvBytes += TLVHeaderSize + len(infoTLVs[i].Value)
	}
	buf, err := ss.scratchFor(CommonHeaderSize + PeerHeaderSize + peerUpFixedSize + len(sentOpen) + len(recvOpen) + tlvBytes)
	if err != nil {
		return err
	}
	n := writePeerUp(buf, 0, pu)
	return ss.enqueueLocked(buf[:n])
}

// writePeerDown encodes and sends a BMP Peer Down message.
func (ss *senderSession) writePeerDown(peer PeerHeader, reason uint8, data []byte) error {
	ss.writeMu.Lock()
	defer ss.writeMu.Unlock()
	return ss.writePeerDownLocked(peer, reason, data)
}

// writePeerDownLocked is writePeerDown for a caller that already holds writeMu
// -- the RFC 8671 Section 7.2 peer bounce (BMPPlugin.bounceMonitoredPeers),
// which queues a Peer Down and the Peer Up that answers it in one critical
// section so no Route Monitoring can land between the pair.
//
// Caller MUST hold writeMu.
func (ss *senderSession) writePeerDownLocked(peer PeerHeader, reason uint8, data []byte) error {
	pd := &PeerDown{
		Peer:   peer,
		Reason: reason,
		Data:   data,
	}

	buf, err := ss.scratchFor(CommonHeaderSize + PeerHeaderSize + 1 + len(data))
	if err != nil {
		return err
	}
	n := writePeerDown(buf, 0, pd)
	return ss.enqueueLocked(buf[:n])
}

// writeRouteMonitoring encodes and sends a BMP Route Monitoring message.
// bgpBody is the BGP message body only (no 16B marker, no 2B length, no 1B
// type) -- that is what bgptypes.RawMessage.RawBytes holds. The 19-byte BGP
// message header per RFC 4271 §4.1 is synthesized inline using msgType so the
// emitted PDU is a complete BGP message as RFC 7854 §4.6 Route Monitoring
// requires. In practice the caller always passes msgtype.TypeUPDATE (BMP
// Route Monitoring carries UPDATEs per RFC 7854) but the parameter makes the
// synthesized header explicit rather than hardcoded.
func (ss *senderSession) writeRouteMonitoring(peer PeerHeader, msgType msgtype.MessageType, bgpBody []byte) error {
	bgpPDULen := message.HeaderLen + len(bgpBody)
	total := CommonHeaderSize + PeerHeaderSize + bgpPDULen

	ss.writeMu.Lock()
	defer ss.writeMu.Unlock()

	buf, err := ss.scratchFor(total)
	if err != nil {
		return err
	}
	off := CommonHeaderSize
	off += writePeerHeader(buf, off, peer)
	// Synthesize BGP message header (RFC 4271 §4.1): Marker(16) + Length(2) + Type(1).
	copy(buf[off:], message.Marker[:])
	binary.BigEndian.PutUint16(buf[off+message.MarkerLen:], uint16(bgpPDULen)) //nolint:gosec // bgpPDULen bounded by scratch size (maxBMPMsgSize < 65535)
	buf[off+message.MarkerLen+2] = byte(msgType)
	off += message.HeaderLen
	copy(buf[off:], bgpBody)
	WriteCommonHeader(buf, 0, CommonHeader{Version: Version, Length: uint32(total), Type: MsgRouteMonitoring}) //nolint:gosec // total bounded by scratch size
	return ss.enqueueLocked(buf[:total])
}

// writeStatisticsReport encodes and sends a BMP Statistics Report.
//
// RFC 8671 Section 6.2: "Statistics report messages are not specific to
// Adj-RIB-In or Adj-RIB-Out and MUST have the O flag set to zero."
// The caller hands in the per-peer header it built for a monitored peer, and
// that header carries the O flag when the peer is monitored for Adj-RIB-Out.
// This function is the only producer of a Statistics Report, so the flag is
// cleared here: every caller is conformant and no caller has to remember.
func (ss *senderSession) writeStatisticsReport(peer PeerHeader, stats []StatEntry) error {
	peer.Flags &^= PeerFlagO

	sr := &statisticsReport{
		Peer:  peer,
		Stats: stats,
	}
	// Size: header + peer + count(4) + stats entries.
	need := CommonHeaderSize + PeerHeaderSize + 4
	for _, s := range stats {
		need += TLVHeaderSize + len(s.Value)
	}

	ss.writeMu.Lock()
	defer ss.writeMu.Unlock()

	buf, err := ss.scratchFor(need)
	if err != nil {
		return err
	}
	n := writeStatisticsReport(buf, 0, sr)
	return ss.enqueueLocked(buf[:n])
}

// writeRouteMirroring encodes and sends a BMP Route Mirroring message.
// RFC 7854 Section 4.7: wraps a complete BGP PDU in TLV type 0.
// bgpBody is the message body without the 19-byte BGP header; the header
// is synthesized inline (same pattern as writeRouteMonitoring).
func (ss *senderSession) writeRouteMirroring(peer PeerHeader, msgType msgtype.MessageType, bgpBody []byte) error {
	bgpPDULen := message.HeaderLen + len(bgpBody)
	tlvLen := TLVHeaderSize + bgpPDULen
	total := CommonHeaderSize + PeerHeaderSize + tlvLen

	ss.writeMu.Lock()
	defer ss.writeMu.Unlock()

	buf, err := ss.scratchFor(total)
	if err != nil {
		return err
	}

	off := CommonHeaderSize
	off += writePeerHeader(buf, off, peer)

	// TLV type 0 = BGP Message (RFC 7854 S4.7).
	binary.BigEndian.PutUint16(buf[off:], MirrorTLVBGPMsg)
	binary.BigEndian.PutUint16(buf[off+2:], uint16(bgpPDULen)) //nolint:gosec // bgpPDULen bounded by scratch size
	off += TLVHeaderSize

	// Synthesize BGP message header (RFC 4271 S4.1): Marker(16) + Length(2) + Type(1).
	copy(buf[off:], message.Marker[:])
	binary.BigEndian.PutUint16(buf[off+message.MarkerLen:], uint16(bgpPDULen)) //nolint:gosec // bounded
	buf[off+message.MarkerLen+2] = byte(msgType)
	off += message.HeaderLen
	copy(buf[off:], bgpBody)

	WriteCommonHeader(buf, 0, CommonHeader{Version: Version, Length: uint32(total), Type: MsgRouteMirroring}) //nolint:gosec // bounded
	return ss.enqueueLocked(buf[:total])
}

// makeStatGauge creates a StatEntry with a uint64 gauge value.
func makeStatGauge(typ uint16, value uint64) StatEntry {
	v := make([]byte, 8)
	binary.BigEndian.PutUint64(v, value)
	return StatEntry{Type: typ, Value: v}
}
