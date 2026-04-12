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
	"sync"
	"time"
)

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
type senderSession struct {
	name    string
	address string
	port    uint16

	conn   net.Conn
	connMu sync.Mutex

	stopCh  chan struct{}
	stopCtx context.Context
	cancel  context.CancelFunc
}

// newSenderSession creates a sender session for the given collector.
func newSenderSession(name string, cfg collectorConfig) *senderSession {
	ctx, cancel := context.WithCancel(context.Background())
	return &senderSession{
		name:    name,
		address: cfg.Address,
		port:    parseUint16(cfg.Port, DefaultPort),
		stopCh:  make(chan struct{}),
		stopCtx: ctx,
		cancel:  cancel,
	}
}

// run is the long-lived goroutine for the sender session.
// It connects to the collector, sends the Initiation message,
// and enters a loop that reconnects on failure.
func (ss *senderSession) run() {
	defer ss.cancel()
	addr := net.JoinHostPort(ss.address, fmt.Sprintf("%d", ss.port))
	reconnectWait := reconnectMin

	for {
		if ss.isStopping() {
			return
		}

		logger().Info("bmp: sender connecting", "collector", ss.name, "address", addr)
		conn, err := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ss.stopCtx, "tcp", addr)
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

		ss.connMu.Lock()
		ss.conn = conn
		ss.connMu.Unlock()

		reconnectWait = reconnectMin
		logger().Info("bmp: sender connected", "collector", ss.name, "address", addr)

		if err := ss.sendInitiation(conn); err != nil {
			logger().Warn("bmp: sender initiation failed", "collector", ss.name, "error", err)
			ss.clearConn()
			closeLog(conn, "sender-init-fail")
			continue
		}

		// Hold connection open until stopped or error.
		ss.holdConnection(conn)

		// Clear conn so concurrent writeMsg callers see nil (not a closed conn).
		ss.clearConn()
	}
}

// clearConn sets the conn field to nil under lock.
func (ss *senderSession) clearConn() {
	ss.connMu.Lock()
	ss.conn = nil
	ss.connMu.Unlock()
}

// sendInitiation sends a BMP Initiation message to the collector.
func (ss *senderSession) sendInitiation(conn net.Conn) error {
	init := &Initiation{
		TLVs: []TLV{
			MakeStringTLV(InitTLVSysName, "ze"),
			MakeStringTLV(InitTLVSysDescr, "ze BGP daemon"),
		},
	}

	// Size: common header(6) + sysName TLV(4+2) + sysDescr TLV(4+14) = 30.
	buf := make([]byte, CommonHeaderSize+TLVHeaderSize+2+TLVHeaderSize+14)
	n := WriteInitiation(buf, 0, init)
	return ss.writeRaw(conn, buf[:n])
}

// sendTermination sends a BMP Termination message before closing.
// Called only from the session's own goroutine (holdConnection), never concurrently.
func (ss *senderSession) sendTermination(conn net.Conn) {
	term := &Termination{
		TLVs: []TLV{
			MakeStringTLV(TermTLVString, "shutting down"),
		},
	}

	// Size: common header(6) + TLV(4+13) = 23.
	buf := make([]byte, CommonHeaderSize+TLVHeaderSize+13)
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

// writeMsg writes a pre-encoded BMP message to the collector connection.
// Returns error if the connection is not available or the write fails.
func (ss *senderSession) writeMsg(data []byte) error {
	ss.connMu.Lock()
	c := ss.conn
	ss.connMu.Unlock()

	if c == nil {
		return fmt.Errorf("not connected")
	}

	return ss.writeRaw(c, data)
}

// holdConnection blocks until stopCh is closed or the connection errors.
// It reads from the connection to detect remote close (BMP is unidirectional
// router->collector, but the collector might close the TCP).
// Termination is sent from this goroutine only, avoiding the stop/write race.
func (ss *senderSession) holdConnection(conn net.Conn) {
	discard := make([]byte, 1)
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
	buf := make([]byte, CommonHeaderSize+PeerHeaderSize+peerUpFixedSize+len(sentOpen)+len(recvOpen))
	n := WritePeerUp(buf, 0, pu)
	return ss.writeMsg(buf[:n])
}

// writePeerDown encodes and sends a BMP Peer Down message.
func (ss *senderSession) writePeerDown(peer PeerHeader, reason uint8, data []byte) error {
	pd := &PeerDown{
		Peer:   peer,
		Reason: reason,
		Data:   data,
	}
	buf := make([]byte, CommonHeaderSize+PeerHeaderSize+1+len(data))
	n := WritePeerDown(buf, 0, pd)
	return ss.writeMsg(buf[:n])
}

// writeRouteMonitoring encodes and sends a BMP Route Monitoring message.
func (ss *senderSession) writeRouteMonitoring(peer PeerHeader, bgpUpdate []byte) error {
	rm := &RouteMonitoring{
		Peer:      peer,
		BGPUpdate: bgpUpdate,
	}
	buf := make([]byte, CommonHeaderSize+PeerHeaderSize+len(bgpUpdate))
	n := WriteRouteMonitoring(buf, 0, rm)
	return ss.writeMsg(buf[:n])
}

// writeStatisticsReport encodes and sends a BMP Statistics Report.
func (ss *senderSession) writeStatisticsReport(peer PeerHeader, stats []StatEntry) error {
	sr := &StatisticsReport{
		Peer:  peer,
		Stats: stats,
	}
	// Size: header + peer + count(4) + stats entries.
	size := CommonHeaderSize + PeerHeaderSize + 4
	for _, s := range stats {
		size += TLVHeaderSize + len(s.Value)
	}
	buf := make([]byte, size)
	n := WriteStatisticsReport(buf, 0, sr)
	return ss.writeMsg(buf[:n])
}

// makeStatGauge creates a StatEntry with a uint64 gauge value.
func makeStatGauge(typ uint16, value uint64) StatEntry {
	v := make([]byte, 8)
	binary.BigEndian.PutUint64(v, value)
	return StatEntry{Type: typ, Value: v}
}
