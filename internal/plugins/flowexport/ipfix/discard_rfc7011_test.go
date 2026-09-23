// RFC 7011 Section 10.3.2 defines UDP sequence numbers in terms of sent
// records. Failed sends across both address families must not advance them.

package ipfix

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

// TestRFC7011FailedSendsDoNotAdvanceSequence checks a multi-datagram batch
// through a closed socket. Both families report failure without counting
// unsent records in the next message's sequence number.
func TestRFC7011FailedSendsDoNotAdvanceSequence(t *testing.T) {
	s, err := flowexport.NewSender("127.0.0.1", 65000, "", flowexport.DatagramSizeDefault)
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Close() // every Send fails

	enc := NewFlowEncoder(1)
	per4 := maxFlowRecordsPerDatagram(false, s.MaxDatagram())
	flows4 := 2*per4 + 1 // three IPv4 datagrams
	batch := make([]flowexport.ConntrackFlow, 0, flows4+1)
	for i := range flows4 {
		batch = append(batch, flowexport.ConntrackFlow{
			SrcAddr: netip.MustParseAddr("10.0.0.1"), DstAddr: netip.MustParseAddr("10.0.0.2"),
			SrcPort: uint16(i), DstPort: 80, Protocol: 6, Bytes: 1, Packets: 1,
		})
	}
	batch = append(batch, flowexport.ConntrackFlow{
		SrcAddr: netip.MustParseAddr("2001:db8::1"), DstAddr: netip.MustParseAddr("2001:db8::2"),
		SrcPort: 1, DstPort: 80, Protocol: 6, Bytes: 1, Packets: 1,
	})

	sent, err := enc.EncodeFlows(batch, s)
	if err == nil {
		t.Fatal("expected a send error on a closed sender")
	}
	if sent != 0 {
		t.Fatalf("records reported sent = %d, want 0", sent)
	}
	if seq := s.Sequence(); seq != 0 {
		t.Fatalf("seqNum = %d after all %d records failed, want 0", seq, len(batch))
	}
}
