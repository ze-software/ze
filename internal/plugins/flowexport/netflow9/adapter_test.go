package netflow9

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

// TestNetflow9EncodeChunksManyInterfaces verifies counter records chunk across
// datagrams (no silent truncation) and Encode reports every record.
func TestNetflow9EncodeChunksManyInterfaces(t *testing.T) {
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

	enc := NewCounterEncoder(0, time.Unix(1716000000, 0))
	n, err := enc.Encode(snap, s)
	if err != nil {
		t.Fatal(err)
	}
	if n != count {
		t.Fatalf("records exported = %d, want %d", n, count)
	}

	datagrams, _, _ := s.Stats()
	if datagrams < 2 {
		t.Fatalf("datagrams = %d, want >= 2 (records should chunk across datagrams)", datagrams)
	}
}

// TestNetflow9SeqNumNotAdvancedOnSendError verifies the export-packet sequence
// number only advances when a datagram is actually sent. Advancing on a failed
// send would open a phantom gap at the collector (RFC 3954: the sequence counts
// packets sent). The sender's socket is closed up front so Send fails.
// RFC requirement: RFC3954-x-8 negative -- a failed Send does not advance the export sequence number (adapter.go:33-65); the cumulative per-observation-domain counter counts packets actually sent, so a send error opens no phantom gap.
func TestNetflow9SeqNumNotAdvancedOnSendError(t *testing.T) {
	s, err := flowexport.NewSender("127.0.0.1", 65000, "")
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Close() // force subsequent Send to fail

	enc := NewCounterEncoder(0, time.Unix(1716000000, 0))
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
