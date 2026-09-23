package ipfix

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

// TestIPFIXEncodeChunksManyInterfaces verifies that a device with more
// interface counter records than fit one datagram produces several datagrams
// (no buffer overflow / panic), and that Encode reports every record.
func TestIPFIXEncodeChunksManyInterfaces(t *testing.T) {
	var lc net.ListenConfig
	pc, err := lc.ListenPacket(context.Background(), "udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pc.Close() }()
	addr, ok := pc.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("unexpected address type")
	}

	s, err := flowexport.NewSender("127.0.0.1", addr.Port, "", flowexport.DatagramSizeDefault)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	const count = 100
	ifaces := make([]flowexport.InterfaceCounters, count)
	for i := range ifaces {
		ifaces[i].IfIndex = uint32(i + 1)
	}
	snap := flowexport.CounterSnapshot{Time: time.Unix(1716000000, 0), Interfaces: ifaces}

	enc := NewCounterEncoder(0)
	n, err := enc.Encode(snap, s)
	if err != nil {
		t.Fatal(err)
	}
	if n != count {
		t.Fatalf("records exported = %d, want %d", n, count)
	}

	// 100 records of CounterRecordSize() do not fit one 1400-byte datagram.
	datagrams, _, _ := s.Stats()
	if datagrams < 2 {
		t.Fatalf("datagrams = %d, want >= 2 (records should chunk across datagrams)", datagrams)
	}
}

// TestIPFIXSeqNumNotAdvancedOnSendError closes the sender and checks that
// records rejected by the socket do not count as sent. RFC 7011 Section
// 10.3.2 defines the UDP sequence as the total Data Records sent.
func TestIPFIXSeqNumNotAdvancedOnSendError(t *testing.T) {
	s, err := flowexport.NewSender("127.0.0.1", 65000, "", flowexport.DatagramSizeDefault)
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Close() // force subsequent Send to fail

	enc := NewCounterEncoder(0)
	seqBefore := s.Sequence()
	snap := flowexport.CounterSnapshot{
		Time:       time.Unix(1716000000, 0),
		Interfaces: []flowexport.InterfaceCounters{{IfIndex: 1}},
	}
	sent, err := enc.Encode(snap, s)
	if err == nil {
		t.Fatal("expected a send error on a closed sender")
	}
	if sent != 0 {
		t.Fatalf("records reported sent = %d after a failed send, want 0", sent)
	}
	if seq := s.Sequence(); seq != seqBefore {
		t.Errorf("seqNum = %d after a failed send, want %d", seq, seqBefore)
	}
}
