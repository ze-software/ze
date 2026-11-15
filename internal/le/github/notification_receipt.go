// Design: docs/contributing/github-integration.md -- receipt DTOs cross the ledger boundary, private variants own transitions.
package github

import (
	"context"
	"errors"
	"fmt"
)

type notificationTransitionKind uint8

const (
	notificationKindUnspecified notificationTransitionKind = iota
	notificationKindFailure
	notificationKindRecovery
)

func decodeNotificationKind(raw string) (notificationTransitionKind, error) {
	switch raw {
	case "":
		return notificationKindUnspecified, nil
	case notificationWireFailure:
		return notificationKindFailure, nil
	case notificationWireRecovery:
		return notificationKindRecovery, nil
	default:
		return notificationKindUnspecified, errors.New("notification transition kind is unknown")
	}
}

func (kind notificationTransitionKind) word() string {
	switch kind {
	case notificationKindFailure:
		return notificationWireFailure
	case notificationKindRecovery:
		return notificationWireRecovery
	case notificationKindUnspecified:
		return ""
	default:
		panic("BUG: unknown validated notification transition kind")
	}
}

// All fields are private validated facts. The lifecycle tag belongs to each
// variant's type and cannot drift independently from an exported DTO field.
type notificationReceiptFacts struct {
	binding                                       string
	workflowID                                    uint64
	workflowPath                                  string
	seenRunID, seenAttempt                        uint64
	failureDigest, failureScope, failureMessageID string
	attemptedRunID, attemptedAttempt              uint64
	attemptedKind                                 notificationTransitionKind
	messageID                                     string
}

type notificationReceiptState interface {
	facts() notificationReceiptFacts
	encode() notificationReceipt
}

type notificationReadyReceipt struct{ state notificationReceiptFacts }
type notificationInFlightReceipt struct{ state notificationReceiptFacts }
type notificationBlockedReceipt struct {
	state  notificationReceiptFacts
	reason string
}

func (ready notificationReadyReceipt) facts() notificationReceiptFacts     { return ready.state }
func (flight notificationInFlightReceipt) facts() notificationReceiptFacts { return flight.state }
func (blocked notificationBlockedReceipt) facts() notificationReceiptFacts { return blocked.state }

func decodeNotificationReceipt(raw notificationReceipt) (notificationReceiptState, error) {
	if err := notificationReceiptValid(raw); err != nil {
		return nil, err
	}
	kind, err := decodeNotificationKind(raw.AttemptedKind)
	if err != nil {
		return nil, err
	}
	state := notificationReceiptFacts{binding: raw.BindingDigest, workflowID: raw.WorkflowID, workflowPath: raw.WorkflowPath,
		seenRunID: raw.SeenRunID, seenAttempt: raw.SeenAttempt, failureDigest: raw.FailureDigest,
		failureScope: raw.FailureScope, failureMessageID: raw.FailureMessageID,
		attemptedRunID: raw.AttemptedRunID, attemptedAttempt: raw.AttemptedAttempt,
		attemptedKind: kind, messageID: raw.MessageID}
	switch raw.Mode {
	case notificationReceiptReady:
		return notificationReadyReceipt{state: state}, nil
	case notificationReceiptInFlight:
		return notificationInFlightReceipt{state: state}, nil
	case notificationReceiptBlocked:
		return notificationBlockedReceipt{state: state, reason: raw.StopReason}, nil
	case notificationReceiptUnspecified:
		return nil, errors.New("notification receipt mode is uninitialized")
	default:
		return nil, errors.New("notification receipt mode has no private lifecycle variant")
	}
}

// encode is confined to the DTO/ledger boundary. Lifecycle operations below
// manipulate private facts, never a mutable public mode/payload pair.
func (state notificationReceiptFacts) encode(mode notificationReceiptMode, reason string) notificationReceipt {
	return notificationReceipt{Version: 1, BindingDigest: state.binding, WorkflowID: state.workflowID,
		WorkflowPath: state.workflowPath, Mode: mode, SeenRunID: state.seenRunID, SeenAttempt: state.seenAttempt,
		FailureDigest: state.failureDigest, FailureScope: state.failureScope, FailureMessageID: state.failureMessageID,
		AttemptedRunID: state.attemptedRunID, AttemptedAttempt: state.attemptedAttempt,
		AttemptedKind: state.attemptedKind.word(), MessageID: state.messageID, StopReason: reason}
}
func (ready notificationReadyReceipt) encode() notificationReceipt {
	return ready.state.encode(notificationReceiptReady, "")
}
func (flight notificationInFlightReceipt) encode() notificationReceipt {
	return flight.state.encode(notificationReceiptInFlight, "delivery acknowledgement is unresolved")
}
func (blocked notificationBlockedReceipt) encode() notificationReceipt {
	return blocked.state.encode(notificationReceiptBlocked, blocked.reason)
}

func (ready notificationReadyReceipt) observe(preview notificationPreview) (notificationReadyReceipt, error) {
	if preview.event.binding == "" || ready.state.binding != preview.event.binding {
		return notificationReadyReceipt{}, errors.New("notification observation has another approval binding")
	}
	if ready.state.workflowID != preview.event.workflowID || ready.state.workflowPath != preview.event.workflowPath {
		return notificationReadyReceipt{}, errors.New("notification observation belongs to another workflow")
	}
	if preview.verdict != notificationFailed && preview.verdict != notificationPassed {
		return notificationReadyReceipt{}, errors.New("notification observation has no usable completed verdict")
	}
	if preview.event.runID < ready.state.seenRunID {
		return notificationReadyReceipt{}, errors.New("notification observation precedes recorded history")
	}
	if preview.event.runID == ready.state.seenRunID && preview.event.attempt <= ready.state.seenAttempt {
		return notificationReadyReceipt{}, errors.New("notification observation was already processed")
	}
	state := ready.state
	state.seenRunID, state.seenAttempt = preview.event.runID, preview.event.attempt
	return notificationReadyReceipt{state: state}, nil
}

func (ready notificationReadyReceipt) prepare(preview notificationPreview, kind notificationTransitionKind) (notificationInFlightReceipt, error) {
	if kind != notificationKindFailure && kind != notificationKindRecovery {
		return notificationInFlightReceipt{}, errors.New("notification attempt has no known transition")
	}
	expected, _, _ := notificationTransition(preview, ready)
	if kind != expected {
		return notificationInFlightReceipt{}, errors.New("notification attempt does not match its validated ready transition")
	}
	observed, err := ready.observe(preview)
	if err != nil {
		return notificationInFlightReceipt{}, err
	}
	state := observed.state
	state.attemptedRunID, state.attemptedAttempt = preview.event.runID, preview.event.attempt
	state.attemptedKind, state.messageID = kind, ""
	return notificationInFlightReceipt{state: state}, nil
}

// Only a successful durable save produces this private capability. A decoded
// interrupted in-flight receipt has no route to it and cannot resume sending.
type notificationPersistedAttempt struct{ flight notificationInFlightReceipt }

func (flight notificationInFlightReceipt) persist(ledger notificationLedger) (notificationPersistedAttempt, error) {
	if err := notificationReceiptValid(flight.encode()); err != nil {
		return notificationPersistedAttempt{}, err
	}
	if err := ledger.save(flight.encode()); err != nil {
		return notificationPersistedAttempt{}, err
	}
	return notificationPersistedAttempt{flight: flight}, nil
}

func (attempt notificationPersistedAttempt) send(ctx context.Context, sender notificationSender, destination notificationDestination, text string) (notificationSendResult, error) {
	return sender.Send(ctx, destination, text)
}

func (attempt notificationPersistedAttempt) resolve(preview notificationPreview, result notificationSendResult, sendErr error) (notificationReceiptState, string, string) {
	state := attempt.flight.state
	if result.Outcome == notificationSendConfirmed && sendErr == nil {
		if err := notificationSnowflake(result.MessageID); err != nil {
			reason := fmt.Errorf("delivery confirmation identity is invalid; reconcile the attempted event: %w", err).Error()
			return notificationBlockedReceipt{state: state, reason: reason}, notificationWireBlocked, reason
		}
		state.messageID = result.MessageID
		if state.attemptedKind == notificationKindFailure {
			state.failureDigest, state.failureScope, state.failureMessageID = preview.fingerprint, preview.scope, result.MessageID
		} else {
			state.failureDigest, state.failureScope, state.failureMessageID = "", "", ""
		}
		return notificationReadyReceipt{state: state}, "delivered", ""
	}
	if result.Outcome == notificationSendRejected {
		return notificationReadyReceipt{state: state}, "rejected", "transport confirmed that no message was delivered; this event is not retried automatically"
	}
	reason := "delivery result is ambiguous; reconcile the attempted event and destination"
	return notificationBlockedReceipt{state: state, reason: reason}, notificationWireBlocked, reason
}
