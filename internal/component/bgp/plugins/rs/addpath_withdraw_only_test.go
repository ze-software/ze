// Design: docs/architecture/wire/nlri.md -- ADD-PATH framing of withdrawn routes
// RFC: rfc/short/rfc7911.md -- RFC7911-3-1, the Path Identifier in withdrawn routes

package rs

import (
	"net/netip"
	"testing"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// TestWithdrawOnlyUpdateKeepsPathIdentifier extracts the inventory records of
// an UPDATE that holds only the Withdrawn Routes field, withdrawing 10.0.0.0/8
// with Path Identifier 7, from a peer that negotiated ADD-PATH for ipv4/unicast.
//
// VALIDATES: one del record, for 10.0.0.0/8. A unicast record read off the
// wire does not keep the Path Identifier yet (journal row in
// zero-value-as-valid-answer.md), so the test proves the framing: without it
// the six octets read as 0.0.0.0/0 three times and then 8.0.0.0/7.
// PREVENTS: the ADD-PATH state read from AttrsWire, which is nil for an UPDATE
// with no attribute section, so the withdrawal was walked without its Path
// Identifier and the route server kept the route in its inventory.
func TestWithdrawOnlyUpdateKeepsPathIdentifier(t *testing.T) {
	t.Parallel()

	// RFC 7911 Section 3: Path Identifier (4 octets), Length (1 octet), Prefix.
	withdrawn := []byte{0, 0, 0, 7, 8, 10}
	body := append([]byte{0, byte(len(withdrawn))}, withdrawn...)
	body = append(body, 0, 0) // Total Path Attribute Length 0, no NLRI.

	ctx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: true})
	ctxID, err := bgpctx.Registry.Register(ctx)
	if err != nil {
		t.Fatalf("register context: %v", err)
	}
	wu := wireu.NewWireUpdate(body, ctxID)
	attrsWire, err := wu.Attrs()
	if err != nil {
		t.Fatalf("attributes: %v", err)
	}
	records := extractWireNLRIRecords(&bgptypes.RawMessage{RawBytes: body, AttrsWire: attrsWire, WireUpdate: wu})
	if records == nil {
		t.Fatal("no records extracted")
	}
	if len(records.records) != 1 {
		t.Fatalf("got %d records, want one withdrawal: %+v", len(records.records), records.records)
	}
	got := records.records[0]
	if got.action != actionDel {
		t.Fatalf("record action %q, want %q", got.action, actionDel)
	}
	if got.prefix != netip.MustParsePrefix("10.0.0.0/8") {
		t.Fatalf("record prefix %v, want 10.0.0.0/8", got.prefix)
	}
}
