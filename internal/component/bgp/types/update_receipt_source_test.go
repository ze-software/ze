// Design: docs/architecture/api/process-protocol.md -- original source session receipts.
package types

import (
	"encoding/json"
	"fmt"
	"testing"
)

// TestUpdateReceiptRequiresOriginalSource rejects an envelope that cannot bind
// retained routes to one original source session, including explicit null.
// VALIDATES: received UPDATE receipts require a precisely encoded source identity.
// PREVENTS: absent or malformed source authority being treated as an unrecorded zero.
func TestUpdateReceiptRequiresOriginalSource(t *testing.T) {
	for _, field := range []string{"", `,"source-receipt":null`, `,"source-receipt":-1`, `,"source-receipt":1.5`, `,"source-receipt":"7"`} {
		t.Run(field, func(t *testing.T) {
			metadata := json.RawMessage(`{"peer":"192.0.2.1","peer-owner":0,"message-id":42,"source-policy-epoch":9,"reactor-forwarded":false` + field + `}`)
			if _, err := ParseUpdateReceipt(metadata); err == nil {
				t.Fatal("UPDATE receipt accepted an absent or malformed original source identity")
			}
		})
	}
}

// TestUpdateReceiptPreservesSourceIdentity proves exact uint64 decoding and
// source binding, including the explicit unrecorded zero used by old bytes.
func TestUpdateReceiptPreservesSourceIdentity(t *testing.T) {
	for _, source := range []uint64{0, 18446744073709551615} {
		metadata := fmt.Appendf(nil, `{"peer":"192.0.2.1","peer-owner":0,"message-id":42,"source-policy-epoch":9,"source-receipt":%d,"reactor-forwarded":false}`, source)
		receipt, err := ParseUpdateReceipt(metadata)
		if err != nil {
			t.Fatal(err)
		}
		if receipt.SourceReceipt != source {
			t.Fatalf("source identity = %d, want %d", receipt.SourceReceipt, source)
		}
		if err := receipt.Validate("192.0.2.1", 0, 42, 9, source); err != nil {
			t.Fatalf("matching source identity refused: %v", err)
		}
		if err := receipt.Validate("192.0.2.1", 0, 42, 9, source^1); err == nil {
			t.Fatal("another source session accepted the receipt")
		}
	}
}
