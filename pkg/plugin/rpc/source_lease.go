// Design: docs/architecture/api/process-protocol.md -- original source leases.
// Related: cancel.go -- release uses acknowledged request cancellation.
package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"time"
)

const MethodSourceLease = "ze-bgp:source-lease"

// SourceLeaseAction is open plugin input. Zero confers no lifecycle action.
type SourceLeaseAction uint8

const (
	SourceLeaseUnspecified SourceLeaseAction = iota
	SourceLeaseAcquire
	SourceLeaseRetain
	SourceLeaseRelease
)

// SourceLeaseInput is raw input, not proof of authority. Its bounded scalar
// shape fits either transport frame; the engine validates the issuing Session
// and the authenticated process's current grants separately.
type SourceLeaseInput struct {
	Action        SourceLeaseAction `json:"action"`
	SourceReceipt uint64            `json:"source-receipt"`
	PeerOwner     uint64            `json:"peer-owner"`
	DeadlineMS    int64             `json:"deadline-ms"`
}

func (input SourceLeaseInput) Validate(now time.Time) error {
	// The action comes from a plugin; every named value is explicit and an
	// unknown action refuses without changing a lease.
	switch input.Action {
	case SourceLeaseAcquire, SourceLeaseRetain, SourceLeaseRelease:
	case SourceLeaseUnspecified:
		return errors.New("source-lease action is missing; send acquire, retain or release")
	default:
		return errors.New("source-lease action is unknown; send acquire, retain or release")
	}
	if input.SourceReceipt == 0 {
		return errors.New("source-lease receipt is zero; use the original received source-receipt")
	}
	if input.Action != SourceLeaseRelease {
		if input.PeerOwner == 0 {
			return errors.New("source-lease peer owner is zero; preserve the original received peer-owner")
		}
	}
	if input.DeadlineMS <= now.UnixMilli() {
		return errors.New("source-lease deadline expired; retry with a live deadline")
	}
	if input.DeadlineMS > now.Add(CancellableRequestTimeout).UnixMilli() {
		return errors.New("source-lease deadline exceeds thirty seconds; send a bounded request")
	}
	return nil
}

// SourceLeaseOutput settles exactly one lifecycle action. Zero and nil are not
// successful answers. Release is reported only after final writes have joined.
type SourceLeaseOutput struct {
	Action        SourceLeaseAction `json:"action"`
	SourceReceipt uint64            `json:"source-receipt"`
	PeerOwner     uint64            `json:"peer-owner"`
}

func (output *SourceLeaseOutput) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for _, name := range []string{"action", "source-receipt", "peer-owner"} {
		field, exists := fields[name]
		if !exists {
			return errors.New("source-lease answer is incomplete; return the settled action, source-receipt and peer-owner")
		}
		if bytes.Equal(field, []byte("null")) {
			return errors.New("source-lease answer is null; return the settled action, source-receipt and peer-owner")
		}
	}
	type plain SourceLeaseOutput
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*output = SourceLeaseOutput(decoded)
	return nil
}

func (output *SourceLeaseOutput) Validate(input SourceLeaseInput) error {
	if output == nil {
		return errors.New("source-lease answer is missing; return the settled action, source-receipt and peer-owner")
	}
	if output.SourceReceipt == 0 {
		return errors.New("source-lease answer has no receipt; echo the original source-receipt")
	}
	if output.SourceReceipt != input.SourceReceipt {
		return errors.New("source-lease answer names another source; echo the original source-receipt")
	}
	if output.PeerOwner != input.PeerOwner {
		return errors.New("source-lease answer names another peer owner; echo the requested peer-owner")
	}
	if output.Action != SourceLeaseRelease {
		if output.PeerOwner == 0 {
			return errors.New("source-lease answer has no peer owner; echo the original received peer-owner")
		}
	}
	if output.Action != input.Action {
		return errors.New("source-lease answer settled another action; echo the requested action")
	}
	// A reply is open plugin/engine boundary input; an unknown action is not a
	// settled lifecycle even when an invalid caller requested the same value.
	switch output.Action {
	case SourceLeaseAcquire, SourceLeaseRetain, SourceLeaseRelease:
		return nil
	case SourceLeaseUnspecified:
		return errors.New("source-lease answer has no action; echo the settled action")
	default:
		return errors.New("source-lease answer has an unknown action; echo the settled action")
	}
}

// SourceLeaseCallContext stamps the SDK deadline. The returned cancel MUST be
// called after the request, including its remote cancellation join, finishes.
func SourceLeaseCallContext(ctx context.Context, input *SourceLeaseInput) (context.Context, context.CancelFunc) {
	work, cancel := context.WithTimeout(ctx, CancellableRequestTimeout)
	if input.DeadlineMS > 0 {
		deadline, bounded := work.Deadline()
		if bounded {
			if time.UnixMilli(input.DeadlineMS).Before(deadline) {
				cancel()
				work, cancel = context.WithDeadline(ctx, time.UnixMilli(input.DeadlineMS))
			}
		}
	}
	deadline, bounded := work.Deadline()
	if bounded {
		input.DeadlineMS = deadline.UnixMilli()
	}
	return work, cancel
}
