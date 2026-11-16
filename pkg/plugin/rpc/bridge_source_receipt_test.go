// Design: docs/architecture/api/process-protocol.md -- pooled structured source provenance.
package rpc

import "testing"

// TestStructuredEventSourceReceiptPoolReset exercises the public pool boundary.
// PREVENTS: another event inheriting one source session's numeric identity.
func TestStructuredEventSourceReceiptPoolReset(t *testing.T) {
	event := GetStructuredEvent()
	event.SourceReceipt = 18446744073709551615
	PutStructuredEvent(event)
	next := GetStructuredEvent()
	defer PutStructuredEvent(next)
	if next.SourceReceipt != 0 {
		t.Fatalf("pooled event inherited receipt %d", next.SourceReceipt)
	}
}
