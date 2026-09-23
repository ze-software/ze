// Pins the chunking of a per-flow batch across export packets: before the fix
// every record past the first datagram was silently dropped.

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

// TestFlowBatchSplitAcrossPackets sends 100 IPv4 flows through EncodeFlows and
// checks that every packet stays within MaxDatagramSize and every record
// arrives, with the packet count field summing to the batch.
func TestFlowBatchSplitAcrossPackets(t *testing.T) {
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

	enc := NewFlowEncoder(1, time.Now())
	const flows = 100
	batch := make([]flowexport.ConntrackFlow, flows)
	for i := range batch {
		batch[i] = flowexport.ConntrackFlow{
			SrcAddr: netip.MustParseAddr("10.0.0.1"), DstAddr: netip.MustParseAddr("10.0.0.2"),
			SrcPort: uint16(i), DstPort: 80, Protocol: 6, Bytes: 1, Packets: 1,
		}
	}
	n, err := enc.EncodeFlows(batch, s)
	if err != nil {
		t.Fatal(err)
	}
	if n != flows {
		t.Fatalf("records exported = %d, want %d", n, flows)
	}
	datagrams, _, _ := s.Stats()
	if datagrams < 2 {
		t.Fatalf("packets = %d, want the batch split across at least 2", datagrams)
	}
	buf := make([]byte, 2*flowexport.MaxDatagramSize)
	var seen int
	for range datagrams {
		if err := pc.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
			t.Fatal(err)
		}
		got, _, err := pc.ReadFrom(buf)
		if err != nil {
			t.Fatalf("packet did not arrive: %v", err)
		}
		if got > flowexport.MaxDatagramSize {
			t.Fatalf("packet of %d octets exceeds %d", got, flowexport.MaxDatagramSize)
		}
		// NetFlow v9 header: version(2) count(2) ...; count is the record total.
		seen += int(binary.BigEndian.Uint16(buf[2:]))
	}
	if seen != flows {
		t.Fatalf("records on the wire = %d, want %d", seen, flows)
	}
}
