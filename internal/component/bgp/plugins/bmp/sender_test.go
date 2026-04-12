package bmp

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

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
	ss := newSenderSession(collectorConfig{
		Name:    "test",
		Address: "127.0.0.1",
		Port:    uint16(addr.Port),
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
	ch, _, err := DecodeCommonHeader(headerBuf, 0)
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
	ss := newSenderSession(collectorConfig{
		Name:    "test",
		Address: "127.0.0.1",
		Port:    uint16(addr.Port),
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
	ch, _, err := DecodeCommonHeader(headerBuf, 0)
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
	ch, _, err := DecodeCommonHeader(headerBuf, 0)
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

func TestBMPSenderRouteMonitoring(t *testing.T) {
	// VALIDATES: AC-28 -- Route Monitoring wraps BGP UPDATE

	server, client := net.Pipe()
	defer closeLog(server, "server-pipe")
	defer closeLog(client, "client-pipe")

	ss := &senderSession{name: "test", conn: client, stopCh: make(chan struct{})}
	bgpUpdate := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	peer := testPeerHeader()

	result := asyncRead(server)

	if err := ss.writeRouteMonitoring(peer, bgpUpdate); err != nil {
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
	if !bytes.Equal(rm.BGPUpdate, bgpUpdate) {
		t.Errorf("BGP update mismatch: got %x, want %x", rm.BGPUpdate, bgpUpdate)
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
	sr, ok := r.msg.(*StatisticsReport)
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
