// Design: docs/architecture/bgp/structural-forwarding.md -- final recipient wire history.
// Related: forward_withdrawal_ownership_test.go -- real forward workers and Session writer.
package reactor

import (
	"bytes"
	"encoding/binary"
	"testing"

	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// TestForwardEmptyMPUnreachRecipient preserves route and control meaning across
// both encoding rails. A real advertisement establishes withdrawal ownership;
// later advertisements fence the entire worker output, not just its first frame.
// RFC 4724 Section 2: "For any other address family, it is an UPDATE message
// that contains only the MP_UNREACH_NLRI attribute [BGP-MP] with no withdrawn
// routes for that <AFI, SAFI>."
func TestForwardEmptyMPUnreachRecipient(t *testing.T) {
	for _, name := range []string{"same-context", "cross-context"} {
		t.Run(name, func(t *testing.T) {
			f := ownershipRailNew(t, nil)
			asn4 := name == "same-context"
			destinationContext := bgpctx.EncodingContextForASN4(asn4)
			destinationID, err := bgpctx.Registry.Register(destinationContext)
			if err != nil {
				t.Fatal(err)
			}
			if (f.ctxID == destinationID) != asn4 {
				t.Fatal("fixture did not establish the intended encoding-context difference")
			}
			for _, peer := range []*Peer{f.source, f.destination} {
				peer.negotiated.Store(&NegotiatedCapabilities{
					families: map[family.Family]bool{family.IPv4Unicast: true, family.IPv6Unicast: true},
					ASN4:     peer == f.source || asn4,
				})
			}
			f.destination.sendCtx.Store(destinationContext)
			f.destination.sendCtxID = destinationID
			f.destination.session.setSendCtxID(destinationID)
			f.source.refreshForwardFacts()
			f.destination.refreshForwardFacts()

			// No ADD-PATH is enabled in either EncodingContextForASN4 context.
			// Establish an initial-sync marker separately from the measured epoch.
			eor := []byte{0, 0, 0, 6, 0x80, 15, 3, 0, 2, 1}
			f.send(t, "cached", f.source, f.ctxID, eor)
			prefix := []byte{24, 198, 51, 100}
			// RFC 4271 Section 4.3: seed through the SAME source and Session writer.
			f.send(t, "cached", f.source, f.ctxID, ownershipRailBody(family.IPv4Unicast, false, prefix, 100))
			f.marker(t, "cached", 101)
			initial := emptyMPRecipientBodies(t, f.conn.written())
			if len(initial) != 3 {
				t.Fatalf("initial history has %d UPDATEs, want EOR, seed and fence", len(initial))
			}
			if !bytes.Equal(initial[0], eor) {
				t.Fatalf("initial standalone EOR = %x, want %x", initial[0], eor)
			}

			// RFC 7606 Section 5.1: accept a mixed field combination, but never
			// turn its empty MP component into a separate control message.
			mixed := []byte{0, 4, 24, 198, 51, 100, 0, 6, 0x80, 15, 3, 0, 2, 1}
			f.send(t, "cached", f.source, f.ctxID, mixed)
			f.marker(t, "cached", 102)
			history := emptyMPRecipientBodies(t, f.conn.written())
			after := history[len(initial):]
			withdrawal := []byte{0, 4, 24, 198, 51, 100, 0, 0}
			if len(after) != 2 {
				t.Fatalf("mixed epoch = %x, want exactly owned withdrawal and fence (no fabricated IPv6 EOR)", after)
			}
			if !bytes.Equal(after[0], withdrawal) {
				t.Fatalf("real withdrawal = %x, want %x", after[0], withdrawal)
			}

			// A blanket empty-MP suppression would pass the negative above.
			// RFC 4724 Section 2: explicitly sent standalone IPv6 EOR survives.
			f.send(t, "cached", f.source, f.ctxID, eor)
			f.marker(t, "cached", 103)
			complete := emptyMPRecipientBodies(t, f.conn.written())
			control := complete[len(history):]
			if len(control) != 2 {
				t.Fatalf("control epoch = %x, want genuine EOR and fence", control)
			}
			if !bytes.Equal(control[0], eor) {
				t.Fatalf("genuine standalone EOR = %x, want %x", control[0], eor)
			}
		})
	}
}

// emptyMPRecipientBodies reads every complete frame after a positive writer
// fence. A truncated frame is a failure, never an ignored tail of the proof.
// RFC 4271 Section 4.1: "This 2-octet unsigned integer indicates the total
// length of the message, including the header in octets."
// Header offsets: [0:16] marker, [16:18] length, [18] type, [19:] body.
func emptyMPRecipientBodies(t *testing.T, wire []byte) [][]byte {
	t.Helper()
	var bodies [][]byte
	for len(wire) > 0 {
		if len(wire) < 19 {
			t.Fatalf("truncated recipient header: %x", wire)
		}
		length := int(binary.BigEndian.Uint16(wire[16:18]))
		if length < 19 {
			t.Fatalf("invalid recipient length %d", length)
		}
		if length > len(wire) {
			t.Fatalf("truncated recipient frame: length %d, available %d", length, len(wire))
		}
		if wire[18] != 2 {
			t.Fatalf("unexpected recipient message type %d", wire[18])
		}
		bodies = append(bodies, wire[19:length])
		wire = wire[length:]
	}
	return bodies
}
