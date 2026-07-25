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

	s, err := flowexport.NewSender("127.0.0.1", addr.Port, "")
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

// TestIPFIXSeqNumNotAdvancedOnSendError verifies the IPFIX sequence number only
// advances when a datagram is actually sent. RFC 7011: the sequence number
// counts Data Records sent; advancing on a failed send would misreport records
// the collector never received. The sender's socket is closed up front so Send
// fails.
func TestIPFIXSeqNumNotAdvancedOnSendError(t *testing.T) {
	s, err := flowexport.NewSender("127.0.0.1", 65000, "")
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Close() // force subsequent Send to fail

	enc := NewCounterEncoder(0)
	seqBefore := enc.seqNum
	snap := flowexport.CounterSnapshot{
		Time:       time.Unix(1716000000, 0),
		Interfaces: []flowexport.InterfaceCounters{{IfIndex: 1}},
	}
	if _, err := enc.Encode(snap, s); err == nil {
		t.Fatal("expected a send error on a closed sender")
	}
	if enc.seqNum != seqBefore {
		t.Errorf("seqNum advanced to %d after a failed send, want %d", enc.seqNum, seqBefore)
	}
}
