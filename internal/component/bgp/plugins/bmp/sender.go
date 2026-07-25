// RFC: rfc/short/rfc7854.md
// Design: docs/architecture/core-design.md -- BMP sender (outbound to collectors)
//
// Related: bmp.go -- plugin lifecycle, config parsing
// Related: msg.go -- BMP message encoding

package bmp

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/network"
)

var errNotConnected = errors.New("not connected")

// RFC 7854 suggested reconnection intervals.
const (
	reconnectMin = 30 * time.Second
	reconnectMax = 720 * time.Second
	writeTimeout = 10 * time.Second
)

// senderSession manages a single outbound TCP connection to a BMP collector.
//
// Caller MUST call stop() to shut down the session goroutine, then
// wait on the WaitGroup that tracks it.
//
// Scratch usage: writePeerUp/Down/RouteMonitoring/Mirroring/StatisticsReport all
// encode into `scratch` and then flush it to the TCP connection. More than one
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
// writeMu therefore covers the whole encode-then-flush, not just the flush:
// without it two producers interleave in the one scratch array and a message
// goes out whose Common Header length does not describe its content, which
// desynchronises the collector's framing for every message after it. Holding it
// across the write also keeps each BMP message contiguous on the wire without
// depending on any net.Conn write-atomicity that net.Conn does not promise.
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

	// writeMu guards scratch AND the ordering of writes to conn. Every method
	// that encodes into scratch or writes to the collector MUST hold it.
	writeMu sync.Mutex
	scratch []byte
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
		// maxBMPMsgSize (65535) is the RFC 7854 ceiling. Allocating
		// this once per collector session keeps the BGP-UPDATE → BMP
		// Route Monitoring hot path allocation-free.
		scratch: make([]byte, maxBMPMsgSize),
	}
}

// run is the long-lived goroutine for the sender session.
// It connects to the collector, sends the Initiation message,
// and enters a loop that reconnects on failure.
func (ss *senderSession) run() {
	defer ss.cancel()
	addr := net.JoinHostPort(ss.address, strconv.Itoa(int(ss.port)))
	reconnectWait := reconnectMin

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

		reconnectWait = reconnectMin
		logger().Info("bmp: sender connected", "collector", ss.name, "address", addr)

		// RFC 7854 Section 4.3: Initiation MUST be the first message on a new BMP
		// session. Publishing ss.conn is what makes this session reachable by the
		// concurrent producers in sendLocked, so it happens only AFTER Initiation
		// is on the wire -- otherwise a Peer Up or a Loc-RIB Route Monitoring
		// racing in from another goroutine reaches the collector first.
		//
		// A producer that arrives while Initiation is in flight BLOCKS on writeMu
		// (sendInitiation holds it) rather than seeing the nil conn: the nil-conn
		// drop window is only the few instructions between sendInitiation's
		// deferred unlock and the publish below. It is bounded by writeTimeout,
		// and it is the same wait such a producer already had before ss.conn was
		// moved -- concurrent conn.Write calls serialize on the socket's own write
		// lock (internal/poll.FD.Write holds it across the whole call).
		//
		// Also note stop() cannot abort an in-flight Initiation: it closes
		// ss.conn, which is still nil for this window.
		if err := ss.sendInitiation(conn); err != nil {
			logger().Warn("bmp: sender initiation failed", "collector", ss.name, "error", err)
			closeLog(conn, "sender-init-fail")
			// Back off before redialling. Without this a collector that accepts the
			// TCP connection and then rejects the Initiation produces a tight
			// dial -> write -> close spin: reconnectWait was reset to reconnectMin
			// above and this branch never waits.
			if ss.waitOrStop(reconnectWait) {
				return
			}
			reconnectWait = min(reconnectWait*2, reconnectMax)
			continue
		}

		ss.connMu.Lock()
		ss.conn = conn
		ss.connMu.Unlock()

		// Hold connection open until stopped or error.
		ss.holdConnection(conn)

		// Clear conn so concurrent sendLocked callers see nil (not a closed conn).
		ss.clearConn()
	}
}

// clearConn sets the conn field to nil under lock.
func (ss *senderSession) clearConn() {
	ss.connMu.Lock()
	ss.conn = nil
	ss.connMu.Unlock()
}

// sendInitiation sends a BMP Initiation message to the collector. It takes
// writeMu so the Initiation stays contiguous on the wire; run() publishes
// ss.conn only after this returns, so no other producer can precede it.
func (ss *senderSession) sendInitiation(conn net.Conn) error {
	ss.writeMu.Lock()
	defer ss.writeMu.Unlock()

	init := &Initiation{
		TLVs: []TLV{
			MakeStringTLV(InitTLVSysName, "ze"),
			MakeStringTLV(InitTLVSysDescr, "ze BGP daemon"),
		},
	}

	// Size: common header(6) + sysName TLV(4+2) + sysDescr TLV(4+14) = 30.
	// Fixed compile-time size — a size-constant regression fails to compile.
	// net.Conn.Write is an interface call so escape analysis moves this to
	// heap today; if the write path ever becomes a concrete type or a
	// provided scratch buffer, the same code path stays on the stack.
	var stack [CommonHeaderSize + TLVHeaderSize + 2 + TLVHeaderSize + 14]byte
	buf := stack[:]
	n := WriteInitiation(buf, 0, init)
	return ss.writeRaw(conn, buf[:n])
}

// sendTermination sends a BMP Termination message before closing. Called from
// the session's own goroutine (holdConnection), but ss.conn is still published
// at that point, so it takes writeMu to stay contiguous against a producer that
// is mid-message.
func (ss *senderSession) sendTermination(conn net.Conn) {
	ss.writeMu.Lock()
	defer ss.writeMu.Unlock()

	term := &Termination{
		TLVs: []TLV{
			MakeStringTLV(TermTLVString, "shutting down"),
		},
	}

	// Size: common header(6) + TLV(4+13) = 23.
	// Fixed compile-time size — a size-constant regression fails to compile.
	// Escapes via net.Conn.Write today; pattern holds if the write path ever
	// becomes a concrete type.
	var stack [CommonHeaderSize + TLVHeaderSize + 13]byte
	buf := stack[:]
	n := WriteTermination(buf, 0, term)
	if err := ss.writeRaw(conn, buf[:n]); err != nil {
		logger().Debug("bmp: sender termination write failed", "collector", ss.name, "error", err)
	}
}

// writeRaw writes data to a connection with a write deadline.
func (ss *senderSession) writeRaw(conn net.Conn, data []byte) error {
	if err := conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
		return err
	}
	_, err := conn.Write(data)
	return err
}

// sendLocked writes a pre-encoded BMP message to the collector connection.
// Returns error if the connection is not available or the write fails.
//
// Caller MUST hold writeMu: data is almost always a slice of the shared scratch
// buffer, and the lock is what keeps this message's bytes contiguous against the
// other producer goroutines described on senderSession.
func (ss *senderSession) sendLocked(data []byte) error {
	ss.connMu.Lock()
	c := ss.conn
	ss.connMu.Unlock()

	if c == nil {
		return errNotConnected
	}

	return ss.writeRaw(c, data)
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
			ss.sendTermination(conn)
			closeLog(conn, "sender-hold-stop")
			return
		}

		if err := conn.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
			closeLog(conn, "sender-hold-deadline")
			return
		}
		_, err := conn.Read(discard)
		if err != nil {
			if ss.isStopping() {
				ss.sendTermination(conn)
				closeLog(conn, "sender-hold-stop")
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

// stop signals the session goroutine to exit.
func (ss *senderSession) stop() {
	close(ss.stopCh)
	ss.cancel() // cancel dial context

	// Close conn to unblock holdConnection's Read.
	ss.connMu.Lock()
	c := ss.conn
	ss.connMu.Unlock()

	if c != nil {
		// This unblocks holdConnection's Read, which then sees isStopping()
		// and sends Termination before returning.
		closeLog(c, "sender-stop")
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
// producer encodes into, and MUST stay owned until the matching sendLocked
// has flushed it.
func (ss *senderSession) scratchFor(need int) ([]byte, error) {
	if ss.scratch == nil {
		ss.scratch = make([]byte, maxBMPMsgSize)
	}
	if need > len(ss.scratch) {
		return nil, fmt.Errorf("bmp: message exceeds max size (%d > %d)", need, len(ss.scratch))
	}
	return ss.scratch[:need], nil
}

// writePeerUp encodes and sends a BMP Peer Up message.
func (ss *senderSession) writePeerUp(peer PeerHeader, localAddr [16]byte, localPort, remotePort uint16, sentOpen, recvOpen []byte) error {
	pu := &PeerUp{
		Peer:            peer,
		LocalAddress:    localAddr,
		LocalPort:       localPort,
		RemotePort:      remotePort,
		SentOpenMsg:     sentOpen,
		ReceivedOpenMsg: recvOpen,
	}

	ss.writeMu.Lock()
	defer ss.writeMu.Unlock()

	buf, err := ss.scratchFor(CommonHeaderSize + PeerHeaderSize + peerUpFixedSize + len(sentOpen) + len(recvOpen))
	if err != nil {
		return err
	}
	n := WritePeerUp(buf, 0, pu)
	return ss.sendLocked(buf[:n])
}

// writePeerDown encodes and sends a BMP Peer Down message.
func (ss *senderSession) writePeerDown(peer PeerHeader, reason uint8, data []byte) error {
	pd := &PeerDown{
		Peer:   peer,
		Reason: reason,
		Data:   data,
	}

	ss.writeMu.Lock()
	defer ss.writeMu.Unlock()

	buf, err := ss.scratchFor(CommonHeaderSize + PeerHeaderSize + 1 + len(data))
	if err != nil {
		return err
	}
	n := WritePeerDown(buf, 0, pd)
	return ss.sendLocked(buf[:n])
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
	off += WritePeerHeader(buf, off, peer)
	// Synthesize BGP message header (RFC 4271 §4.1): Marker(16) + Length(2) + Type(1).
	copy(buf[off:], message.Marker[:])
	binary.BigEndian.PutUint16(buf[off+message.MarkerLen:], uint16(bgpPDULen)) //nolint:gosec // bgpPDULen bounded by scratch size (maxBMPMsgSize < 65535)
	buf[off+message.MarkerLen+2] = byte(msgType)
	off += message.HeaderLen
	copy(buf[off:], bgpBody)
	WriteCommonHeader(buf, 0, CommonHeader{Version: Version, Length: uint32(total), Type: MsgRouteMonitoring}) //nolint:gosec // total bounded by scratch size
	return ss.sendLocked(buf[:total])
}

// writeStatisticsReport encodes and sends a BMP Statistics Report.
func (ss *senderSession) writeStatisticsReport(peer PeerHeader, stats []StatEntry) error {
	sr := &StatisticsReport{
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
	n := WriteStatisticsReport(buf, 0, sr)
	return ss.sendLocked(buf[:n])
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
	off += WritePeerHeader(buf, off, peer)

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
	return ss.sendLocked(buf[:total])
}

// makeStatGauge creates a StatEntry with a uint64 gauge value.
func makeStatGauge(typ uint16, value uint64) StatEntry {
	v := make([]byte, 8)
	binary.BigEndian.PutUint64(v, value)
	return StatEntry{Type: typ, Value: v}
}
