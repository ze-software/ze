// Design: docs/architecture/api/process-protocol.md -- mandatory UPDATE receipt scalars.
// Related: update_receipt.go -- the BGP-owned receipt decoder.
package types

import (
	"fmt"
	"strconv"
)

// receiptUint64 preserves explicit zero without allocating a pointer per field.
// Its zero is absent; only successful JSON decoding marks the value present.
type receiptUint64 struct {
	value   uint64
	present bool
}

func (number *receiptUint64) UnmarshalJSON(data []byte) error {
	value, err := strconv.ParseUint(string(data), 10, 64)
	if err != nil {
		return fmt.Errorf("BGP UPDATE receipt requires a non-null decimal uint64: %w", err)
	}
	*number = receiptUint64{value: value, present: true}
	return nil
}

// receiptDecision preserves explicit false without treating absent or null as a
// cached-forwarding instruction. Only the two JSON booleans initialize it.
type receiptDecision uint8

const (
	receiptDecisionUnspecified receiptDecision = iota
	receiptDecisionDeferred
	receiptDecisionForwarded
)

func (decision *receiptDecision) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case "false":
		*decision = receiptDecisionDeferred
	case "true":
		*decision = receiptDecisionForwarded
	default:
		return fmt.Errorf("BGP UPDATE receipt requires a non-null JSON boolean decision")
	}
	return nil
}
