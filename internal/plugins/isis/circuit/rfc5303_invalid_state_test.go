// Design: docs/architecture/isis/isis-5-adjacency.md -- TLV 240 receive guard.
package circuit

import (
	"reflect"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// TestRFC5303InvalidStateWireDiscard passes a malformed three-way state through
// the real packet decoder and circuit consumer, before and after adjacency Up.
// RFC requirement: RFC5303-3.2-7 negative -- invalid state PDUs create no neighbor and leave an existing Up neighbor, including its hold deadline, unchanged.
// RFC 5303 Section 3.2: "If the option is present and contains invalid Adjacency
// Three-Way State, the PDU SHALL be discarded and no further action is taken."
func TestRFC5303InvalidStateWireDiscard(t *testing.T) {
	for _, established := range []bool{false, true} {
		for _, size := range []int{1, 5, 15} {
			for _, invalid := range []byte{3, 255} {
				c := p2pCircuit(t, &fakeSender{mtu: 1500})
				pdu := buildPeerP2PHello(t, packet.CircuitL1L2, c.systemID)
				peer := types.SystemID{0, 0, 0, 0, 0, 2}
				if established {
					if tr := c.Receive(adjacency.SNPA{}, pdu); tr.State != adjacency.StateUp {
						t.Fatalf("valid setup did not reach Up: %+v", tr)
					}
				}
				before, existed := c.Table().Lookup(peer, adjacency.Level1)
				var saved adjacency.Adjacency
				if existed {
					saved = *before
				}
				decoded, err := packet.DecodePDU(pdu)
				if err != nil {
					t.Fatal(err)
				}
				h := decoded.P2PHello
				for i := range h.TLVs {
					if h.TLVs[i].Type == packet.TLVP2PThreeWay {
						h.TLVs[i].Value = h.TLVs[i].Value[:size]
						h.TLVs[i].Value[0] = invalid
					}
				}
				h.HoldingTime = types.HoldingTime(600)
				buf := make([]byte, h.EncodedLen())
				buf = buf[:h.WriteTo(buf, 0)]
				packet.ReleaseTLVs(h.TLVs)
				c.now = func() time.Time { return time.Unix(9000, 0) }
				tr := c.Receive(adjacency.SNPA{1, 2, 3, 4, 5, 6}, buf)
				if !tr.Rejected || tr.SessionUp || tr.SessionDown || tr.ForwardingChanged {
					t.Errorf("established=%v length=%d state=%d: transition=%+v", established, size, invalid, tr)
				}
				after, exists := c.Table().Lookup(peer, adjacency.Level1)
				if exists != existed {
					t.Fatalf("established=%v length=%d state=%d changed neighbor presence", established, size, invalid)
				}
				if existed {
					if !reflect.DeepEqual(*after, saved) {
						t.Fatalf("established=%v length=%d state=%d mutated neighbor: before=%+v after=%+v", established, size, invalid, saved, *after)
					}
				}
			}
		}
	}
}

// TestRFC5303InvalidStateBeforeValidDuplicate ensures a later valid TLV cannot
// hide an invalid state in an earlier option before the circuit admission guard.
// RFC requirement: RFC5303-3.2-7 negative -- an invalid TLV 240 state followed by a valid duplicate still discards the PDU without creating a neighbor.
func TestRFC5303InvalidStateBeforeValidDuplicate(t *testing.T) {
	c := p2pCircuit(t, &fakeSender{mtu: 1500})
	decoded, err := packet.DecodePDU(buildPeerP2PHello(t, packet.CircuitL1L2, c.systemID))
	if err != nil {
		t.Fatal(err)
	}
	h := decoded.P2PHello
	tlvs := h.TLVs
	defer packet.ReleaseTLVs(tlvs)
	h.TLVs = make([]packet.TLV, 0, len(tlvs)+1)
	h.TLVs = append(h.TLVs, packet.TLV{Type: packet.TLVP2PThreeWay, Value: []byte{255}})
	h.TLVs = append(h.TLVs, tlvs...)
	buf := make([]byte, h.EncodedLen())
	buf = buf[:h.WriteTo(buf, 0)]
	if tr := c.Receive(adjacency.SNPA{}, buf); !tr.Rejected {
		t.Fatalf("valid duplicate hid invalid three-way state: %+v", tr)
	}
	if c.Table().Len() != 0 {
		t.Fatal("invalid duplicate created a neighbor")
	}
}
