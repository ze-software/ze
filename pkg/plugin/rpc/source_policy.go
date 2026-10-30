// Design: docs/architecture/api/process-protocol.md -- source-policy lifecycle.
package rpc

import (
	"encoding/json"
	"fmt"
	"time"
)

// MarshalText encodes a known lifecycle operation and refuses unspecified values.
func (op SourcePolicyOperation) MarshalText() ([]byte, error) {
	switch op {
	case SourcePolicyPrepare:
		return []byte("prepare"), nil
	case SourcePolicyApply:
		return []byte("apply"), nil
	case SourcePolicyAbort:
		return []byte("abort"), nil
	case SourcePolicyFinalize:
		return []byte("finalize"), nil
	case SourcePolicyUnspecified:
		return nil, fmt.Errorf("source-policy operation is unspecified")
	default: // Open boundary: callers may supply an unknown numeric value.
		return nil, fmt.Errorf("unknown source-policy operation %d", op)
	}
}

// UnmarshalText refuses unknown operations rather than invoking an owner.
func (op *SourcePolicyOperation) UnmarshalText(text []byte) error {
	switch string(text) {
	case "prepare":
		*op = SourcePolicyPrepare
	case "apply":
		*op = SourcePolicyApply
	case "abort":
		*op = SourcePolicyAbort
	case "finalize":
		*op = SourcePolicyFinalize
	default:
		return fmt.Errorf("unknown source-policy operation %q", text)
	}
	return nil
}

// sourcePolicyFields refuses absent and null values, including required zeros.
func sourcePolicyFields(data []byte, names ...string) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for _, name := range names {
		value, ok := fields[name]
		if !ok {
			return fmt.Errorf("source-policy requires %s", name)
		}
		if string(value) == "null" {
			return fmt.Errorf("source-policy requires non-null %s", name)
		}
	}
	return nil
}

// UnmarshalJSON preserves explicit zero frontiers while refusing absent identity.
func (source *SourcePolicyIdentity) UnmarshalJSON(data []byte) error {
	if err := sourcePolicyFields(data, "peer", "peer-owner", "session-generation", "policy-epoch", "receive-frontier", "template-generation"); err != nil {
		return err
	}
	type identity SourcePolicyIdentity
	var decoded identity
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*source = SourcePolicyIdentity(decoded)
	return nil
}

// UnmarshalJSON refuses missing and null required fields before publishing input.
func (input *SourcePolicyInput) UnmarshalJSON(data []byte) error {
	if err := sourcePolicyFields(data, "transaction-id", "operation", "source", "next-policy-epoch", "deadline-ms"); err != nil {
		return err
	}
	type request SourcePolicyInput
	var decoded request
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*input = SourcePolicyInput(decoded)
	return nil
}

// UnmarshalJSON requires the entire echoed receipt, including zero target epochs.
func (out *SourcePolicyOutput) UnmarshalJSON(data []byte) error {
	if err := sourcePolicyFields(data, "transaction-id", "operation", "source", "next-policy-epoch", "status"); err != nil {
		return err
	}
	type receipt SourcePolicyOutput
	var decoded receipt
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*out = SourcePolicyOutput(decoded)
	return nil
}

// Validate rejects malformed or expired raw requests on both transport boundaries.
// The coordinator MUST additionally enforce monotonic targets across operations.
func (input SourcePolicyInput) Validate(now time.Time) error {
	if input.TransactionID == "" {
		return fmt.Errorf("source-policy requires a transaction")
	}
	if len(input.TransactionID) > 256 {
		return fmt.Errorf("source-policy transaction exceeds 256 bytes")
	}
	if input.Source.Peer == "" {
		return fmt.Errorf("source-policy requires a source peer")
	}
	if len(input.Source.Peer) > 256 {
		return fmt.Errorf("source-policy peer exceeds 256 bytes")
	}
	if input.Source.PeerOwner == 0 {
		return fmt.Errorf("source-policy requires a peer owner")
	}
	if input.Source.SessionGeneration == 0 {
		return fmt.Errorf("source-policy requires a session generation")
	}
	if input.Source.PolicyEpoch == 0 {
		return fmt.Errorf("source-policy requires an original policy epoch")
	}
	switch input.Operation {
	case SourcePolicyPrepare, SourcePolicyFinalize:
		if input.NextPolicyEpoch != 0 {
			return fmt.Errorf("source-policy prepare/finalize requires a zero target epoch")
		}
	case SourcePolicyApply:
		if input.NextPolicyEpoch <= input.Source.PolicyEpoch {
			return fmt.Errorf("source-policy apply requires a target epoch newer than the captured one")
		}
	case SourcePolicyAbort:
		// Zero aborts a transaction that was prepared and never applied: the
		// owner releases its pins and changes nothing else. A nonzero target
		// aborts after apply, and owners restamp the restored paths to it.
		// Which of the two an owner's stage admits is the owner's ledger's
		// decision (internal/core/bgp/sourcepolicy).
		if input.NextPolicyEpoch == 0 {
			break
		}
		if input.NextPolicyEpoch <= input.Source.PolicyEpoch {
			return fmt.Errorf("source-policy abort requires a zero target (before apply) or one newer than the captured epoch")
		}
	case SourcePolicyUnspecified:
		return fmt.Errorf("source-policy operation is unspecified")
	default: // Open boundary: a raw request may contain an unknown operation.
		return fmt.Errorf("unknown source-policy operation %d", input.Operation)
	}
	if input.DeadlineMS <= 0 {
		return fmt.Errorf("source-policy requires an absolute deadline")
	}
	if !now.Before(time.UnixMilli(input.DeadlineMS)) {
		return fmt.Errorf("source-policy deadline expired")
	}
	return nil
}
