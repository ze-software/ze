// Design: docs/architecture/api/process-protocol.md — applied event settlement.

package rpc

import (
	"encoding/json"
	"fmt"
	"time"
)

// MethodEventSettlement acknowledges asynchronous work after event application.
const MethodEventSettlement = "ze-plugin-callback:event-settlement"

// EventSettlementErrorBytesMax bounds a failed settlement receipt.
const EventSettlementErrorBytesMax = 4096

// CallbackAnswerWriteMax bounds a worker's failure-answer write after its work
// deadline. A caller needing that late failure MUST reserve reply time inside
// its own deadline; the allowance never authorizes successful work after expiry.
const CallbackAnswerWriteMax = time.Second

// EventSettlementInput is a raw settlement request. The engine MUST first await
// event application; the plugin MUST finish covered asynchronous work before
// acknowledging. DeadlineMS bounds both handler execution and queue waiting.
type EventSettlementInput struct {
	DeadlineMS int64 `json:"deadline-ms"`
}

// Validate refuses an unset or expired bound on either transport.
func (input EventSettlementInput) Validate(now time.Time) error {
	if input.DeadlineMS <= 0 {
		return fmt.Errorf("event settlement requires a positive deadline-ms; send a future deadline")
	}
	if input.DeadlineMS <= now.UnixMilli() {
		return fmt.Errorf("event settlement deadline has expired; retry with a future deadline")
	}
	return nil
}

// UnmarshalJSON refuses absent and null deadlines before publishing the input.
func (input *EventSettlementInput) UnmarshalJSON(data []byte) error {
	var decoded struct {
		DeadlineMS *int64 `json:"deadline-ms"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if decoded.DeadlineMS == nil {
		return fmt.Errorf("event settlement requires deadline-ms; send an explicit future deadline")
	}
	*input = EventSettlementInput{DeadlineMS: *decoded.DeadlineMS}
	return nil
}

// EventSettlementOutput is a completed handler receipt, never queue admission.
// DeadlineMS echoes the exact request bound. Zero is not a successful receipt.
type EventSettlementOutput struct {
	DeadlineMS int64  `json:"deadline-ms"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
}

// UnmarshalJSON refuses a missing outcome or correlation, including JSON null.
func (output *EventSettlementOutput) UnmarshalJSON(data []byte) error {
	var decoded struct {
		DeadlineMS *int64  `json:"deadline-ms"`
		Status     *string `json:"status"`
		Error      string  `json:"error,omitempty"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if decoded.DeadlineMS == nil {
		return fmt.Errorf("event settlement receipt requires deadline-ms")
	}
	if decoded.Status == nil {
		return fmt.Errorf("event settlement receipt requires status")
	}
	*output = EventSettlementOutput{DeadlineMS: *decoded.DeadlineMS, Status: *decoded.Status, Error: decoded.Error}
	return nil
}

// Validate checks exact correlation and the closed receipt outcomes. A failed
// receipt is valid data; the caller MUST still return its Error as a failure.
func (output EventSettlementOutput) Validate(input EventSettlementInput) error {
	if output.DeadlineMS <= 0 {
		return fmt.Errorf("event settlement receipt requires a positive deadline-ms")
	}
	if output.DeadlineMS != input.DeadlineMS {
		return fmt.Errorf("event settlement receipt deadline does not match the request")
	}
	if len(output.Error) > EventSettlementErrorBytesMax {
		return fmt.Errorf("event settlement receipt error exceeds 4096 bytes")
	}
	switch output.Status {
	case StatusOK:
		if output.Error != "" {
			return fmt.Errorf("successful event settlement receipt contains an error")
		}
	case StatusError:
		if output.Error == "" {
			return fmt.Errorf("failed event settlement receipt omits its cause")
		}
	default: // Open RPC boundary: an unknown outcome never acknowledges work.
		return fmt.Errorf("unknown event settlement receipt status %q", output.Status)
	}
	return nil
}
