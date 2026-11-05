// Design: docs/architecture/api/process-protocol.md -- BGP UPDATE forwarding receipts.
// Related: rawmessage.go -- engine-owned received and sent message facts.
package types

import (
	"encoding/json"
	"fmt"
	"net/netip"

	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// UpdateReceipt is the raw BGP-owned metadata of an enveloped UPDATE. It is a
// transport value, not filter-writable route metadata. Readers MUST Validate it
// against the decoded payload before using its forwarding decision. A decoded
// skipped list belongs to its reader; structured dispatch MUST clone the
// publisher's list before handing it to an asynchronous worker.
type UpdateReceipt struct {
	Peer              netip.Addr       `json:"peer"`
	MessageID         uint64           `json:"message-id"`
	SourcePolicyEpoch uint64           `json:"source-policy-epoch"`
	ReactorForwarded  bool             `json:"reactor-forwarded"`
	FastPathSkipped   []netip.AddrPort `json:"fast-path-skipped,omitempty"`
}

// ParseUpdateReceipt decodes one UPDATE envelope's mandatory metadata. The
// returned raw value MUST be bound to its actual payload with Validate. Other
// event kinds may omit generic envelope metadata and never call this parser.
func ParseUpdateReceipt(metadata json.RawMessage) (UpdateReceipt, error) {
	if len(metadata) == 0 {
		return UpdateReceipt{}, fmt.Errorf("BGP UPDATE envelope requires a forwarding receipt")
	}
	var fields struct {
		Peer              netip.Addr       `json:"peer"`
		MessageID         receiptUint64    `json:"message-id"`
		SourcePolicyEpoch receiptUint64    `json:"source-policy-epoch"`
		ReactorForwarded  receiptDecision  `json:"reactor-forwarded"`
		FastPathSkipped   []netip.AddrPort `json:"fast-path-skipped"`
	}
	if err := json.Unmarshal(metadata, &fields); err != nil {
		return UpdateReceipt{}, fmt.Errorf("BGP UPDATE receipt: %w", err)
	}
	if !fields.Peer.IsValid() {
		return UpdateReceipt{}, fmt.Errorf("BGP UPDATE receipt requires its source peer")
	}
	if !fields.MessageID.present {
		return UpdateReceipt{}, fmt.Errorf("BGP UPDATE receipt requires message-id")
	}
	if !fields.SourcePolicyEpoch.present {
		return UpdateReceipt{}, fmt.Errorf("BGP UPDATE receipt requires source-policy-epoch")
	}
	if fields.ReactorForwarded == receiptDecisionUnspecified {
		return UpdateReceipt{}, fmt.Errorf("BGP UPDATE receipt requires reactor-forwarded")
	}
	return UpdateReceipt{
		Peer: fields.Peer, MessageID: fields.MessageID.value,
		SourcePolicyEpoch: fields.SourcePolicyEpoch.value,
		ReactorForwarded:  fields.ReactorForwarded == receiptDecisionForwarded,
		FastPathSkipped:   fields.FastPathSkipped,
	}, nil
}

// Validate binds the receipt to the decoded payload's identity. A caller MUST
// run this before using ReactorForwarded or FastPathSkipped. A zero receipt is
// uninitialized and fails the peer comparison, never an accepted decision.
func (receipt UpdateReceipt) Validate(peer string, messageID, sourcePolicyEpoch uint64) error {
	address, err := netip.ParseAddr(peer)
	if err != nil {
		return fmt.Errorf("BGP UPDATE payload peer: %w", err)
	}
	if receipt.Peer != address {
		return fmt.Errorf("BGP UPDATE receipt peer does not match its payload")
	}
	if receipt.MessageID != messageID {
		return fmt.Errorf("BGP UPDATE receipt message ID does not match its payload")
	}
	if receipt.SourcePolicyEpoch != sourcePolicyEpoch {
		return fmt.Errorf("BGP UPDATE receipt policy epoch does not match its payload")
	}
	return nil
}

// UnwrapUpdateText returns a text UPDATE envelope's owned payload, raw receipt
// and explicit envelope presence. Its caller MUST bind a present receipt with
// Validate after parsing the ordinary text header. Bare text and other
// namespaces are returned unchanged with presence false: those are distinct
// subscription forms, not compatibility versions.
func UnwrapUpdateText(text string) (string, UpdateReceipt, bool, error) {
	if text == "" {
		return text, UpdateReceipt{}, false, nil
	}
	if text[0] != '{' {
		return text, UpdateReceipt{}, false, nil
	}
	envelope, err := rpc.ParseEventEnvelope(text)
	if err != nil {
		return "", UpdateReceipt{}, false, fmt.Errorf("BGP event envelope: %w", err)
	}
	if envelope.Namespace != bgpevents.Namespace {
		return text, UpdateReceipt{}, false, nil
	}
	if envelope.Event != bgpevents.EventUpdate {
		return text, UpdateReceipt{}, false, nil
	}
	var payload string
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return "", UpdateReceipt{}, false, fmt.Errorf("BGP text UPDATE envelope payload: %w", err)
	}
	receipt, err := ParseUpdateReceipt(envelope.Metadata)
	if err != nil {
		return "", UpdateReceipt{}, false, err
	}
	return payload, receipt, true, nil
}
