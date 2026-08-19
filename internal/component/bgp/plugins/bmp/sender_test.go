package bmp

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	"github.com/ze-software/ze/internal/component/bgp/message"
)

// RFC requirement: RFC7854-x-6 positive -- the first message the sender writes
// immediately after connecting is the Initiation message.
func TestBMPSenderConnects(t *testing.T) {
	// VALIDATES: AC-23 -- sender connects outbound TCP to collector
	// PREVENTS: sender goroutine crash on startup

	// Start a mock collector (TCP listener).
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() {
		if err := ln.Close(); err != nil {
			t.Logf("close listener: %v", err)
		}
	}()

	addr, _ := ln.Addr().(*net.TCPAddr)
	ss := newSenderSession("test", collectorConfig{
		Address: "127.0.0.1",
		Port:    strconv.Itoa(addr.Port),
	})

	// Accept connection from sender in background.
	connCh := make(chan net.Conn, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		connCh <- c
	}()

	// Start sender in background.
	var wg sync.WaitGroup
	wg.Go(ss.run)

	// Wait for connection.
	var collectorConn net.Conn
	select {
	case collectorConn = <-connCh:
	case <-time.After(5 * time.Second):
		t.Fatal("sender did not connect within timeout")
	}

	// Read Initiation message from collector's perspective.
	headerBuf := make([]byte, CommonHeaderSize)
	if _, err := io.ReadFull(collectorConn, headerBuf); err != nil {
		t.Fatalf("read header: %v", err)
	}
	ch, _, err := decodeCommonHeader(headerBuf, 0)
	if err != nil {
		t.Fatalf("decode header: %v", err)
	}
	if ch.Type != MsgInitiation {
		t.Errorf("first message type = %d, want %d (Initiation)", ch.Type, MsgInitiation)
	}

	// Cleanup.
	ss.stop()
	wg.Wait()
	closeLog(collectorConn, "test-collector")
}

// RFC requirement: RFC7854-x-7 positive -- the Initiation message carries the
// sysName TLV (type 2).
func TestBMPSenderInitiation(t *testing.T) {
	// VALIDATES: AC-25 -- Initiation sent with sysName and sysDescr

	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() {
		if err := ln.Close(); err != nil {
			t.Logf("close: %v", err)
		}
	}()

	addr, _ := ln.Addr().(*net.TCPAddr)
	ss := newSenderSession("test", collectorConfig{
		Address: "127.0.0.1",
		Port:    strconv.Itoa(addr.Port),
	})

	connCh := make(chan net.Conn, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		connCh <- c
	}()

	var wg sync.WaitGroup
	wg.Go(ss.run)

	var collectorConn net.Conn
	select {
	case collectorConn = <-connCh:
	case <-time.After(5 * time.Second):
		t.Fatal("sender did not connect")
	}

	// Read full Initiation message.
	headerBuf := make([]byte, CommonHeaderSize)
	if _, err := io.ReadFull(collectorConn, headerBuf); err != nil {
		t.Fatalf("read header: %v", err)
	}
	ch, _, err := decodeCommonHeader(headerBuf, 0)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	msgBuf := make([]byte, ch.Length)
	copy(msgBuf, headerBuf)
	if _, err := io.ReadFull(collectorConn, msgBuf[CommonHeaderSize:]); err != nil {
		t.Fatalf("read body: %v", err)
	}

	msg, err := DecodeMsg(msgBuf)
	if err != nil {
		t.Fatalf("decode msg: %v", err)
	}
	init, ok := msg.(*Initiation)
	if !ok {
		t.Fatalf("expected *Initiation, got %T", msg)
	}

	// Verify sysName and sysDescr TLVs present.
	var foundName, foundDescr bool
	for _, tlv := range init.TLVs {
		if tlv.Type == InitTLVSysName && string(tlv.Value) == "ze" {
			foundName = true
		}
		if tlv.Type == InitTLVSysDescr {
			foundDescr = true
		}
	}
	if !foundName {
		t.Error("Initiation missing sysName=ze")
	}
	if !foundDescr {
		t.Error("Initiation missing sysDescr")
	}

	ss.stop()
	wg.Wait()
	closeLog(collectorConn, "test-collector")
}

// readBMPFromPipe reads one complete BMP message from a pipe connection.
// Must be called concurrently with the write side (net.Pipe is unbuffered).
func readBMPFromPipe(conn net.Conn) (any, error) {
	headerBuf := make([]byte, CommonHeaderSize)
	if _, err := io.ReadFull(conn, headerBuf); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	ch, _, err := decodeCommonHeader(headerBuf, 0)
	if err != nil {
		return nil, fmt.Errorf("decode header: %w", err)
	}
	msgBuf := make([]byte, ch.Length)
	copy(msgBuf, headerBuf)
	if _, err := io.ReadFull(conn, msgBuf[CommonHeaderSize:]); err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	return DecodeMsg(msgBuf)
}

// pipeResult holds the result of an async pipe read.
type pipeResult struct {
	msg any
	err error
}

// asyncRead starts reading a BMP message from the pipe in a goroutine.
func asyncRead(conn net.Conn) <-chan pipeResult {
	ch := make(chan pipeResult, 1)
	go func() {
		msg, err := readBMPFromPipe(conn)
		ch <- pipeResult{msg, err}
	}()
	return ch
}

// RFC requirement: RFC7854-x-8 positive -- Peer Up carries both the sent and the
// received OPEN messages, echoed byte-for-byte in the emitted PeerUp.
func TestBMPSenderPeerUp(t *testing.T) {
	// VALIDATES: AC-26 -- Peer Up sent with OPEN messages

	server, client := net.Pipe()
	defer closeLog(server, "server-pipe")
	defer closeLog(client, "client-pipe")

	ss := &senderSession{name: "test", conn: client, stopCh: make(chan struct{})}

	sentOpen := makeBGPOpen(65001, 0x01020304)
	recvOpen := makeBGPOpen(65002, 0x05060708)
	peer := testPeerHeader()

	result := asyncRead(server)

	if err := ss.writePeerUp(peer, [16]byte{}, 179, 54321, sentOpen, recvOpen); err != nil {
		t.Fatalf("writePeerUp: %v", err)
	}

	r := <-result
	if r.err != nil {
		t.Fatalf("read: %v", r.err)
	}
	pu, ok := r.msg.(*PeerUp)
	if !ok {
		t.Fatalf("expected *PeerUp, got %T", r.msg)
	}
	if !bytes.Equal(pu.SentOpenMsg, sentOpen) {
		t.Error("sent OPEN mismatch")
	}
	if !bytes.Equal(pu.ReceivedOpenMsg, recvOpen) {
		t.Error("received OPEN mismatch")
	}
}

// RFC requirement: RFC7854-x-10 positive -- Peer Down carries a Reason code:
// WritePeerDown emits the reason byte and the decoded PeerDown reports it.
func TestBMPSenderPeerDown(t *testing.T) {
	// VALIDATES: AC-27 -- Peer Down with correct reason code

	server, client := net.Pipe()
	defer closeLog(server, "server-pipe")
	defer closeLog(client, "client-pipe")

	ss := &senderSession{name: "test", conn: client, stopCh: make(chan struct{})}
	peer := testPeerHeader()

	result := asyncRead(server)

	if err := ss.writePeerDown(peer, PeerDownDeconfigured, nil); err != nil {
		t.Fatalf("writePeerDown: %v", err)
	}

	r := <-result
	if r.err != nil {
		t.Fatalf("read: %v", r.err)
	}
	pd, ok := r.msg.(*PeerDown)
	if !ok {
		t.Fatalf("expected *PeerDown, got %T", r.msg)
	}
	if pd.Reason != PeerDownDeconfigured {
		t.Errorf("reason = %d, want %d", pd.Reason, PeerDownDeconfigured)
	}
}

// RFC requirement: RFC7854-x-9 positive -- a Route Monitoring message wraps a
// complete BGP UPDATE PDU (RFC 4271 header + body, type UPDATE).
func TestBMPSenderRouteMonitoring(t *testing.T) {
	// VALIDATES: AC-28 -- Route Monitoring wraps BGP UPDATE with a
	// synthesized RFC 4271 §4.1 header (marker + length + type=UPDATE).

	server, client := net.Pipe()
	defer closeLog(server, "server-pipe")
	defer closeLog(client, "client-pipe")

	ss := &senderSession{name: "test", conn: client, stopCh: make(chan struct{})}
	body := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	peer := testPeerHeader()

	result := asyncRead(server)

	if err := ss.writeRouteMonitoring(peer, msgtype.TypeUPDATE, body); err != nil {
		t.Fatalf("writeRouteMonitoring: %v", err)
	}

	r := <-result
	if r.err != nil {
		t.Fatalf("read: %v", r.err)
	}
	rm, ok := r.msg.(*RouteMonitoring)
	if !ok {
		t.Fatalf("expected *RouteMonitoring, got %T", r.msg)
	}

	wantLen := message.HeaderLen + len(body)
	if len(rm.BGPUpdate) != wantLen {
		t.Fatalf("BGPUpdate length = %d, want %d (header + body)", len(rm.BGPUpdate), wantLen)
	}
	if !bytes.Equal(rm.BGPUpdate[:message.MarkerLen], message.Marker[:]) {
		t.Errorf("marker mismatch: got %x, want %x", rm.BGPUpdate[:message.MarkerLen], message.Marker[:])
	}
	gotLen := uint16(rm.BGPUpdate[message.MarkerLen])<<8 | uint16(rm.BGPUpdate[message.MarkerLen+1])
	if int(gotLen) != wantLen {
		t.Errorf("length field = %d, want %d", gotLen, wantLen)
	}
	if rm.BGPUpdate[message.MarkerLen+2] != byte(msgtype.TypeUPDATE) {
		t.Errorf("type = %d, want %d (UPDATE)", rm.BGPUpdate[message.MarkerLen+2], msgtype.TypeUPDATE)
	}
	if !bytes.Equal(rm.BGPUpdate[message.HeaderLen:], body) {
		t.Errorf("body mismatch: got %x, want %x", rm.BGPUpdate[message.HeaderLen:], body)
	}
}

func TestBMPSenderStatistics(t *testing.T) {
	// VALIDATES: AC-29 -- Statistics Report with per-peer counters

	server, client := net.Pipe()
	defer closeLog(server, "server-pipe")
	defer closeLog(client, "client-pipe")

	ss := &senderSession{name: "test", conn: client, stopCh: make(chan struct{})}
	peer := testPeerHeader()
	stats := []StatEntry{
		makeStatGauge(StatPrefixesRejected, 42),
		makeStatGauge(StatRoutesAdjRIBIn, 1000),
	}

	result := asyncRead(server)

	if err := ss.writeStatisticsReport(peer, stats); err != nil {
		t.Fatalf("writeStatisticsReport: %v", err)
	}

	r := <-result
	if r.err != nil {
		t.Fatalf("read: %v", r.err)
	}
	sr, ok := r.msg.(*statisticsReport)
	if !ok {
		t.Fatalf("expected *StatisticsReport, got %T", r.msg)
	}
	if len(sr.Stats) != 2 {
		t.Fatalf("stats count = %d, want 2", len(sr.Stats))
	}
	if sr.Stats[0].Type != StatPrefixesRejected {
		t.Errorf("stat[0] type = %d, want %d", sr.Stats[0].Type, StatPrefixesRejected)
	}
}

// RFC requirement: RFC7854-x-11 positive -- a Termination message is produced
// when the session is being closed on shutdown.
func TestBMPSenderTermination(t *testing.T) {
	// VALIDATES: AC-34 -- Termination sent on shutdown

	server, client := net.Pipe()
	defer closeLog(server, "server-pipe")
	defer closeLog(client, "client-pipe")

	ss := &senderSession{name: "test", conn: client, stopCh: make(chan struct{})}

	result := asyncRead(server)

	ss.sendTermination(client)

	r := <-result
	if r.err != nil {
		t.Fatalf("read: %v", r.err)
	}
	if _, ok := r.msg.(*Termination); !ok {
		t.Fatalf("expected *Termination, got %T", r.msg)
	}
}

// VALIDATES: startSender is idempotent -- calling it twice leaves ONE sender
// session per collector, not two.
// PREVENTS: doubling every collector's BMP stream, socket and goroutine. The
// old startSender appended to bp.senders with no preceding stopSenders, while
// its call-site neighbor startLocRIB documents itself as idempotent across
// reloads; the asymmetry read as unintentional and this pins the fix.
//
// LATENT, not live, and deliberately tested anyway. Stage-2 configure is sent
// by deliverConfigRPC (internal/component/plugin/server/startup.go:736), whose
// only caller chain is engineStartupSink.deliverConfig -> runStartupHandshake
// -> handleProcessStartupRPC, i.e. once per plugin PROCESS startup, so a config
// reload does not re-deliver it today. The guard exists so that if a reload
// path ever does, it cannot silently double the sender set.
func TestStartSenderIsIdempotent(t *testing.T) {
	// Port 1 on a loopback address: newSenderSession only records the address,
	// and run() dials in its own goroutine, so nothing here depends on a
	// connection being established. stopSenders cancels those goroutines.
	cfg := &senderConfig{
		Collectors: map[string]collectorConfig{
			"one": {Address: "127.0.0.1", Port: "1"},
			"two": {Address: "127.0.0.1", Port: "1"},
		},
	}

	bp := &BMPPlugin{}
	t.Cleanup(func() {
		bp.stopSenders()
		bp.sessions.Wait()
	})

	bp.startSender(cfg)
	bp.mu.RLock()
	afterFirst := len(bp.senders)
	bp.mu.RUnlock()
	if afterFirst != len(cfg.Collectors) {
		t.Fatalf("first startSender: got %d senders, want %d", afterFirst, len(cfg.Collectors))
	}

	bp.startSender(cfg)
	bp.mu.RLock()
	afterSecond := len(bp.senders)
	bp.mu.RUnlock()
	if afterSecond != len(cfg.Collectors) {
		t.Fatalf("second startSender: got %d senders, want %d (sender set doubled)", afterSecond, len(cfg.Collectors))
	}
}

// NOTE (resolved 2026-07-27): the bounded transmit queue landed, and none of
// the tests in this file needed changing -- including the four that carry RFC
// 7854 tags (RFC7854-x-8 through x-11).
//
// The socket write DID move onto a per-session drain goroutine, so a test that
// hands a senderSession a connection and then reads it back does need a drain
// running. It gets one from the shipped code: enqueueLocked starts the drain on
// first use (sender_drain.go ensureDrain), so these fixtures exercise exactly
// the asynchronous path production takes -- encode into scratch, copy into the
// queue, drain writes the socket. There is no nil-queue synchronous fallback,
// which the previous note rightly ruled out as coverage that proves nothing.
//
// What a new test here DOES have to account for: a write* call returning nil
// means the message is QUEUED, not that it is on the wire. If a test closes the
// connection or asserts on socket bytes right after the call, wait for the
// queue first (waitQueueDrained in sender_queue_test.go).
