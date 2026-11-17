// Design: docs/architecture/api/process-protocol.md -- selected refresh markers.
// Related: selected.go -- the same authenticated destination output stream.
package rpc

import (
	"context"
	"errors"
	"time"
)

const MethodSelectRefresh = "ze-plugin-engine:select-refresh"

// SelectedRefreshAction is open protocol input. Normal refresh requests are
// deliberately absent: this operation brackets the selecting owner's output.
type SelectedRefreshAction uint8

const (
	SelectedRefreshUnspecified SelectedRefreshAction = iota
	SelectedRefreshBegin
	SelectedRefreshEnd
)

// SelectRefreshInput narrows the captured destination; none of these fields
// grants authority. The engine authenticates the caller and the exact peer.
type SelectRefreshInput struct {
	Destination        string                `json:"destination"`
	PeerOwner          uint64                `json:"peer-owner"`
	DestinationSession uint64                `json:"destination-session"`
	Family             string                `json:"family"`
	Action             SelectedRefreshAction `json:"action"`
	DeadlineMS         int64                 `json:"deadline-ms"`
}

func (input SelectRefreshInput) Validate(now time.Time) error {
	if input.Destination == "" {
		return errors.New("select-refresh destination is missing; preserve the original peer address")
	}
	if len(input.Destination) > 128 {
		return errors.New("select-refresh destination is too long; preserve the original peer address")
	}
	if input.PeerOwner == 0 {
		return errors.New("select-refresh peer owner is zero; preserve the original peer-owner")
	}
	if input.DestinationSession == 0 {
		return errors.New("select-refresh destination session is zero; preserve the original UP or refresh token")
	}
	if input.Family == "" {
		return errors.New("select-refresh family is missing; name one negotiated family")
	}
	if len(input.Family) > 64 {
		return errors.New("select-refresh family is too long; use its registered family name")
	}
	switch input.Action {
	case SelectedRefreshBegin, SelectedRefreshEnd:
	case SelectedRefreshUnspecified:
		return errors.New("select-refresh action is missing; choose begin or end")
	default:
		return errors.New("select-refresh action is unknown; choose begin or end")
	}
	if input.DeadlineMS <= now.UnixMilli() {
		return errors.New("select-refresh deadline expired; retry with a live deadline")
	}
	if input.DeadlineMS > now.Add(CancellableRequestTimeout).UnixMilli() {
		return errors.New("select-refresh deadline exceeds thirty seconds; use a bounded request")
	}
	return nil
}

// SelectedRefreshOutcome explicitly distinguishes a final write from the
// negotiated absence of enhanced markers. Neither zero nor nil means success.
type SelectedRefreshOutcome uint8

const (
	SelectedRefreshOutcomeUnspecified SelectedRefreshOutcome = iota
	SelectedRefreshWritten
	SelectedRefreshNotNegotiated
)

// SelectRefreshOutput echoes the entire requested marker identity. Written
// MUST follow final socket completion; queue admission MUST NOT produce it.
type SelectRefreshOutput struct {
	Destination        string                 `json:"destination"`
	PeerOwner          uint64                 `json:"peer-owner"`
	DestinationSession uint64                 `json:"destination-session"`
	Family             string                 `json:"family"`
	Action             SelectedRefreshAction  `json:"action"`
	Outcome            SelectedRefreshOutcome `json:"outcome"`
}

func (output *SelectRefreshOutput) Validate(input SelectRefreshInput) error {
	if output == nil {
		return errors.New("select-refresh final receipt is missing; reconcile the captured destination")
	}
	if output.Destination != input.Destination {
		return errors.New("select-refresh receipt names another destination; reconcile the captured destination")
	}
	if output.PeerOwner != input.PeerOwner {
		return errors.New("select-refresh receipt names another peer owner; reconcile the captured destination")
	}
	if output.DestinationSession != input.DestinationSession {
		return errors.New("select-refresh receipt names another session; reconcile the captured destination")
	}
	if output.Family != input.Family {
		return errors.New("select-refresh receipt names another family; reconcile the requested family")
	}
	if output.Action != input.Action {
		return errors.New("select-refresh receipt names another action; reconcile the requested marker")
	}
	switch output.Action {
	case SelectedRefreshBegin, SelectedRefreshEnd:
	case SelectedRefreshUnspecified:
		return errors.New("select-refresh receipt has no action; request a complete receipt")
	default:
		return errors.New("select-refresh receipt has an unknown action; request a complete receipt")
	}
	switch output.Outcome {
	case SelectedRefreshWritten, SelectedRefreshNotNegotiated:
		return nil
	case SelectedRefreshOutcomeUnspecified:
		return errors.New("select-refresh receipt has no outcome; request a complete receipt")
	default:
		return errors.New("select-refresh receipt has an unknown outcome; request a complete receipt")
	}
}

// SelectRefreshCallContext uses the selected stream's existing deadline clamp.
// The caller MUST cancel after remote cancellation and final completion join.
func SelectRefreshCallContext(ctx context.Context, input *SelectRefreshInput) (context.Context, context.CancelFunc) {
	envelope := SelectRoutesInput{DeadlineMS: input.DeadlineMS}
	work, cancel := SelectedCallContext(ctx, &envelope)
	input.DeadlineMS = envelope.DeadlineMS
	return work, cancel
}
