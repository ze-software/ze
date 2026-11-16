// Design: docs/architecture/api/process-protocol.md -- payload-bound original source identity.
package bgp

import (
	"fmt"
	"testing"
)

// TestParseEventRejectsReboundSourceReceipt exercises the public JSON reader.
// VALIDATES: the envelope's original session identity must match its payload.
// PREVENTS: an authentic forwarding decision authorizing a different source session.
func TestParseEventRejectsReboundSourceReceipt(t *testing.T) {
	// Match the formatter's peer.remote.address shape: a flat address is not
	// accepted by GetPeerAddress and would reject before testing this binding.
	envelope := func(sourceReceipt uint64) []byte {
		return fmt.Appendf(nil, `{"namespace":"bgp","event":"update","payload":{"type":"bgp","bgp":{"peer":{"remote":{"address":"192.0.2.1","as":65001}},"message":{"type":"update","id":42,"direction":"received"},"source-policy-epoch":9,"source-receipt":7,"update":{}}},"metadata":{"peer":"192.0.2.1","message-id":42,"source-policy-epoch":9,"source-receipt":%d,"reactor-forwarded":false}}`, sourceReceipt)
	}
	matching, err := ParseEvent(envelope(7))
	if err != nil {
		t.Fatalf("matching original source receipt was rejected: %v", err)
	}
	if matching.GetPeerAddress() != "192.0.2.1" || matching.GetMsgID() != 42 || matching.SourcePolicyEpoch != 9 {
		t.Fatalf("matching envelope lost its decoded identity: %#v", matching)
	}
	if _, err := ParseEvent(envelope(8)); err == nil {
		t.Fatal("UPDATE envelope accepted another source session's receipt")
	}
}
