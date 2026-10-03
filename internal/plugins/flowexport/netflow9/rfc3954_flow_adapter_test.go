package netflow9

import (
	"context"
	"encoding/binary"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

// recvDatagrams reads up to n datagrams from pc with a short deadline.
func recvDatagrams(t *testing.T, pc net.PacketConn, n int) [][]byte {
	t.Helper()
	var out [][]byte
	_ = pc.SetReadDeadline(time.Now().Add(2 * time.Second))
	for range n {
		buf := make([]byte, 2048)
		nr, _, err := pc.ReadFrom(buf)
		if err != nil {
			break
		}
		out = append(out, buf[:nr])
	}
	return out
}

func newLoopbackEncoderTarget(t *testing.T) (net.PacketConn, *flowexport.Sender) {
	t.Helper()
	var lc net.ListenConfig
	pc, err := lc.ListenPacket(context.Background(), "udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr, ok := pc.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("unexpected address type")
	}
	s, err := flowexport.NewSender("127.0.0.1", addr.Port, "", flowexport.DatagramSizeDefault)
	if err != nil {
		t.Fatal(err)
	}
	return pc, s
}

// TestNetflow9FlowSeqNumPerPacket is a regression test for the bug where the
// flow encoder advanced the sequence number by the record count instead of by
// one per export packet. RFC 3954 Section 5.1: the sequence number counts
// export packets. Two EncodeFlows calls (3 then 2 IPv4 flows) must produce
// datagrams whose header sequence numbers differ by exactly 1.
// RFC requirement: RFC3954-x-8 positive -- two EncodeFlows calls from one FlowEncoder (one observation domain) produce datagrams whose header sequence numbers differ by exactly 1 (flow_adapter.go:96-147); the counter is cumulative and advances per export packet.
func TestNetflow9FlowSeqNumPerPacket(t *testing.T) {
	pc, s := newLoopbackEncoderTarget(t)
	defer func() { _ = pc.Close() }()
	defer func() { _ = s.Close() }()

	enc := NewFlowEncoder(0, time.Unix(1716000000, 0))

	mk := func(n int) []flowexport.ConntrackFlow {
		flows := make([]flowexport.ConntrackFlow, n)
		for i := range flows {
			flows[i] = flowexport.ConntrackFlow{
				SrcAddr:  netip.MustParseAddr("192.0.2.1"),
				DstAddr:  netip.MustParseAddr("198.51.100.1"),
				Protocol: 6,
				Bytes:    100,
				Packets:  2,
			}
		}
		return flows
	}

	if _, err := enc.EncodeFlows(mk(3), s); err != nil {
		t.Fatal(err)
	}
	if _, err := enc.EncodeFlows(mk(2), s); err != nil {
		t.Fatal(err)
	}

	dgs := recvDatagrams(t, pc, 2)
	if len(dgs) != 2 {
		t.Fatalf("received %d datagrams, want 2", len(dgs))
	}
	// Sequence number is at header offset 12 (version2+count2+sysUpTime4+unixSecs4).
	seq0 := binary.BigEndian.Uint32(dgs[0][12:])
	seq1 := binary.BigEndian.Uint32(dgs[1][12:])
	if seq1-seq0 != 1 {
		t.Errorf("sequence advanced by %d across two packets, want 1 (RFC 3954 counts packets, not records)", seq1-seq0)
	}
}

// TestNetflow9FailedFlowSendDoesNotCreateLoss checks that a refused UDP
// write leaves the current transport session's sequence unchanged.
func TestNetflow9FailedFlowSendDoesNotCreateLoss(t *testing.T) {
	pc, sender := newLoopbackEncoderTarget(t)
	defer func() { _ = pc.Close() }()
	defer func() { _ = sender.Close() }()
	enc := NewFlowEncoder(0, time.Now())
	flows := []flowexport.ConntrackFlow{{
		SrcAddr: netip.MustParseAddr("192.0.2.1"),
		DstAddr: netip.MustParseAddr("198.51.100.1"),
	}}
	if sent, err := enc.EncodeFlows(flows, sender); err != nil || sent != 1 {
		t.Fatalf("initial export sent %d records, error %v", sent, err)
	}
	datagrams := recvDatagrams(t, pc, 1)
	if len(datagrams) != 1 {
		t.Fatalf("received %d datagrams, want 1", len(datagrams))
	}
	want := binary.BigEndian.Uint32(datagrams[0][12:]) + 1
	if err := sender.Close(); err != nil {
		t.Fatal(err)
	}
	if sent, err := enc.EncodeFlows(flows, sender); err == nil || sent != 0 {
		t.Fatalf("closed sender exported %d records, error %v", sent, err)
	}
	if seq := sender.Sequence(); seq != want {
		t.Fatalf("failed sender sequence = %d, want %d", seq, want)
	}
}

// TestNetflow9FlowFamilySplit verifies IPv4 and IPv6 flows are sent as separate
// datagrams (distinct templates) and the returned count sums both families.
func TestNetflow9FlowFamilySplit(t *testing.T) {
	pc, s := newLoopbackEncoderTarget(t)
	defer func() { _ = pc.Close() }()
	defer func() { _ = s.Close() }()

	enc := NewFlowEncoder(0, time.Unix(1716000000, 0))
	flows := []flowexport.ConntrackFlow{
		{SrcAddr: netip.MustParseAddr("192.0.2.1"), DstAddr: netip.MustParseAddr("198.51.100.1"), Protocol: 6},
		{SrcAddr: netip.MustParseAddr("2001:db8::1"), DstAddr: netip.MustParseAddr("2001:db8::2"), Protocol: 6},
	}
	n, err := enc.EncodeFlows(flows, s)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("records exported = %d, want 2", n)
	}
	dgs := recvDatagrams(t, pc, 2)
	if len(dgs) != 2 {
		t.Fatalf("received %d datagrams, want 2 (one per family)", len(dgs))
	}
}
