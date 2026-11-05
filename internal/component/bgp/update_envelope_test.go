// Design: docs/architecture/api/process-protocol.md -- source-bound BGP UPDATE envelope delivery.
package bgp

import (
	"encoding/json"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestParseUpdateEnvelopePreservesPayloadAndIdentity exercises the actual shared
// JSON parser used by the RIB, receive store, EPE, LS export and RPKI consumers.
// A receipt is transport authority, not route metadata; exact uint64 provenance
// and the received family survive unwrapping, while a mismatched identity fails.
func TestParseUpdateEnvelopePreservesPayloadAndIdentity(t *testing.T) {
	const epoch = uint64(18446744073709551610)
	const payload = `{"type":"bgp","bgp":{"message":{"type":"update","id":40,"direction":"received"},"peer":{"remote":{"address":"10.0.0.1","as":65001}},"source-policy-epoch":18446744073709551610,"update":{"attr":{"origin":"igp"},"nlri":{"ipv4/unicast":[{"action":"add","next-hop":"192.0.2.1","nlri":["10.0.0.0/24"]}]}},"route-meta":{"reactor-forwarded":false}}}`
	for _, mismatch := range []string{"", "peer", "message-id", "source-policy-epoch"} {
		name := mismatch
		if name == "" {
			name = "matching"
		}
		t.Run(name, func(t *testing.T) {
			metadata := map[string]any{"peer": "10.0.0.1", "message-id": uint64(40),
				"source-policy-epoch": epoch, "reactor-forwarded": true}
			if mismatch == "peer" {
				metadata[mismatch] = "10.0.0.99"
			}
			if mismatch == "message-id" {
				metadata[mismatch] = uint64(99)
			}
			if mismatch == "source-policy-epoch" {
				metadata[mismatch] = epoch - 1
			}
			encoded, err := json.Marshal(map[string]any{"namespace": "bgp", "event": "update",
				"payload": json.RawMessage(payload), "metadata": metadata})
			if err != nil {
				t.Fatal(err)
			}
			event, err := ParseEvent(encoded)
			if mismatch != "" {
				if err == nil {
					t.Fatal("shared parser accepted a receipt detached from its actual payload")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if event.GetEventType() != rpc.EventKindUpdate || event.GetMsgID() != 40 || event.GetPeerAddress() != "10.0.0.1" {
				t.Fatalf("envelope lost BGP identity: %+v", event)
			}
			if event.SourcePolicyEpoch != epoch {
				t.Fatalf("source epoch = %d, want exact uint64 %d", event.SourcePolicyEpoch, epoch)
			}
			if len(event.FamilyOps[family.IPv4Unicast]) != 1 {
				t.Fatalf("envelope lost its positive announcement: %+v", event.FamilyOps)
			}
		})
	}
}
