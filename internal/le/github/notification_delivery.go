// Design: docs/contributing/github-integration.md -- receipt durability precedes every approved delivery attempt.
// Related: notification_store.go -- atomic locked receipts; notification_discord.go -- bounded transport.
package github

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
)

// notificationApproval is a supplied owner-approval receipt. This package
// verifies its binding but never issues approval. The activation boundary MUST
// load this operator authority from a private, owner-provisioned file.
type notificationApproval struct {
	BindingDigest string `json:"binding-digest"`
}

type notificationReceiptMode uint8

const (
	notificationReceiptUnspecified notificationReceiptMode = iota
	notificationReceiptReady
	notificationReceiptInFlight
	notificationReceiptBlocked
)

// notificationReceipt is the bounded on-disk boundary representation. Every
// load is validated before use; the zero value is not an initial state.
type notificationReceipt struct {
	Version          uint8                   `json:"version"`
	BindingDigest    string                  `json:"binding-digest"`
	WorkflowID       uint64                  `json:"workflow-id"`
	WorkflowPath     string                  `json:"workflow-path"`
	Mode             notificationReceiptMode `json:"mode"`
	SeenRunID        uint64                  `json:"seen-run-id"`
	SeenAttempt      uint64                  `json:"seen-attempt"`
	ScanRunID        uint64                  `json:"scan-run-id,omitempty"`
	ScanAttempt      uint64                  `json:"scan-attempt,omitempty"`
	FailureDigest    string                  `json:"delivered-failure-digest"`
	FailureScope     string                  `json:"delivered-failure-scope"`
	FailureMessageID string                  `json:"delivered-failure-message-id"`
	AttemptedRunID   uint64                  `json:"attempted-run-id"`
	AttemptedAttempt uint64                  `json:"attempted-attempt"`
	AttemptedKind    string                  `json:"attempted-kind"`
	MessageID        string                  `json:"message-id"`
	StopReason       string                  `json:"stop-reason"`
}

// newNotificationReceipt constructs a deliberate empty ledger for one exact
// approved binding/workflow. Store initialization MUST refuse an existing file.
func newNotificationReceipt(preview notificationPreview, approval notificationApproval) (notificationReceipt, error) {
	if err := checkNotificationApproval(preview, approval); err != nil {
		return notificationReceipt{}, err
	}
	ready := notificationReadyReceipt{state: notificationReceiptFacts{binding: preview.event.binding,
		workflowID: preview.event.workflowID, workflowPath: preview.event.workflowPath}}
	return ready.encode(), nil
}

func checkNotificationApproval(preview notificationPreview, approval notificationApproval) error {
	if preview.event == nil {
		return errors.New("notification preview was not constructed")
	}
	if preview.event.binding == "" {
		return errors.New("notification preview was not constructed")
	}
	if approval.BindingDigest != preview.event.binding {
		return errors.New("notification template or destination has no matching exact approval; prepare a fresh preview")
	}
	if preview.event.destination.channel == "" {
		return errors.New("notification destination has no approved channel and webhook identities")
	}
	return nil
}

func notificationReceiptValid(receipt notificationReceipt) error {
	if receipt.Version != 1 {
		return errors.New("notification receipt version is unknown")
	}
	if err := notificationHash(receipt.BindingDigest); err != nil {
		return fmt.Errorf("notification receipt approval binding is invalid: %w", err)
	}
	if receipt.WorkflowID == 0 {
		return errors.New("notification receipt workflow identity is missing")
	}
	if receipt.WorkflowPath == "" {
		return errors.New("notification receipt workflow path is missing")
	}
	if len(receipt.WorkflowPath) > 160 {
		return errors.New("notification receipt workflow path exceeds its bound")
	}
	switch receipt.Mode {
	case notificationReceiptReady:
		if receipt.StopReason != "" {
			return errors.New("ready notification receipt contains an unresolved stop reason")
		}
	case notificationReceiptInFlight, notificationReceiptBlocked:
		if receipt.AttemptedRunID == 0 {
			return errors.New("blocked notification receipt has no attempted event")
		}
		if receipt.AttemptedAttempt == 0 {
			return errors.New("blocked notification receipt has no attempted run attempt")
		}
		if receipt.AttemptedRunID != receipt.SeenRunID {
			return errors.New("unresolved notification receipt does not identify its current observed event")
		}
		if receipt.AttemptedAttempt != receipt.SeenAttempt {
			return errors.New("unresolved notification receipt does not identify its current observed event")
		}
		if receipt.MessageID != "" {
			return errors.New("unresolved notification receipt claims a confirmed current message")
		}
		if receipt.StopReason == "" {
			return errors.New("unresolved notification receipt has no stop reason")
		}
	case notificationReceiptUnspecified:
		return errors.New("notification receipt mode is uninitialized")
	default:
		return errors.New("notification receipt mode is unknown")
	}
	if receipt.SeenRunID == 0 {
		if receipt.SeenAttempt != 0 {
			return errors.New("notification receipt has an attempt without a seen run")
		}
	} else {
		if receipt.SeenAttempt == 0 {
			return errors.New("notification receipt has no seen attempt")
		}
	}
	if receipt.ScanRunID == 0 {
		if receipt.ScanAttempt != 0 {
			return errors.New("notification scan has an attempt without a run")
		}
	} else {
		if receipt.ScanAttempt == 0 {
			return errors.New("notification scan has no attempt")
		}
	}
	if receipt.FailureDigest == "" {
		if receipt.FailureScope != "" {
			return errors.New("notification receipt has recovery scope without a delivered failure")
		}
		if receipt.FailureMessageID != "" {
			return errors.New("notification receipt has a failure message without its identity")
		}
	} else {
		if err := notificationHash(receipt.FailureDigest); err != nil {
			return fmt.Errorf("notification delivered-failure digest is invalid: %w", err)
		}
		if err := notificationHash(receipt.FailureScope); err != nil {
			return fmt.Errorf("notification delivered-failure scope is invalid: %w", err)
		}
		if err := notificationSnowflake(receipt.FailureMessageID); err != nil {
			return fmt.Errorf("notification receipt confirmed failure message identity is invalid: %w", err)
		}
		if receipt.SeenRunID == 0 {
			return errors.New("notification receipt claims a delivered failure without an observed run")
		}
		if receipt.AttemptedRunID == 0 {
			return errors.New("notification receipt claims a delivered failure without an attempted event")
		}
	}
	if receipt.MessageID != "" {
		if err := notificationSnowflake(receipt.MessageID); err != nil {
			return fmt.Errorf("notification receipt message identity is invalid: %w", err)
		}
	}
	if receipt.AttemptedRunID == 0 {
		if receipt.AttemptedAttempt != 0 {
			return errors.New("notification receipt has an attempted number without a run")
		}
		if receipt.AttemptedKind != "" {
			return errors.New("notification receipt has an attempted kind without a run")
		}
		if receipt.MessageID != "" {
			return errors.New("notification receipt has a message without an attempted run")
		}
	} else {
		if receipt.AttemptedAttempt == 0 {
			return errors.New("notification receipt attempted run has no attempt number")
		}
		switch receipt.AttemptedKind {
		case notificationWireFailure, notificationWireRecovery:
		default:
			return errors.New("notification receipt attempted kind is unknown")
		}
		if receipt.AttemptedRunID > receipt.SeenRunID {
			return errors.New("notification receipt attempted run exceeds its observed run")
		}
		if receipt.AttemptedRunID == receipt.SeenRunID {
			if receipt.AttemptedAttempt > receipt.SeenAttempt {
				return errors.New("notification receipt attempted number exceeds its observed attempt")
			}
		}
	}
	if len(receipt.StopReason) > 256 {
		return errors.New("notification receipt stop reason exceeds its bound")
	}
	return nil
}

func notificationHash(value string) error {
	if len(value) != 64 {
		return fmt.Errorf("digest must contain 64 hexadecimal characters, received %d", len(value))
	}
	_, err := hex.DecodeString(value)
	if err != nil {
		return fmt.Errorf("digest hexadecimal decoding failed: %w", err)
	}
	return nil
}

// notificationLedger is available only while a store holds its exclusive lock.
// save MUST durably publish a complete receipt before it answers success.
type notificationLedger interface {
	load() (notificationReceipt, error)
	save(notificationReceipt) error
}

// notificationStore serializes the entire read/attempt/confirmation sequence.
// Implementations MUST NOT release ownership during the sender call.
type notificationStore interface {
	withLock(context.Context, func(notificationLedger) error) error
}

type notificationSendOutcome uint8

const (
	notificationSendUnspecified notificationSendOutcome = iota
	notificationSendConfirmed
	notificationSendRejected
	notificationSendAmbiguous
)

type notificationSendResult struct {
	Outcome   notificationSendOutcome
	MessageID string
}

// notificationSender attempts at most one message and classifies uncertainty.
// It MUST bind the concrete transport to destination and MUST NOT retry a POST.
type notificationSender interface {
	Send(context.Context, notificationDestination, string) (notificationSendResult, error)
}

type notificationDeliveryReport struct {
	State       string `json:"state"`
	Destination string `json:"destination"`
	RunID       uint64 `json:"run-id"`
	Attempt     uint64 `json:"attempt"`
	Kind        string `json:"kind"`
	MessageID   string `json:"message-id"`
	Reason      string `json:"reason"`
}

// deliverNotification requires exact approval and a bound transport from the
// private activation boundary. A durable in-flight receipt makes a crash or
// lost acknowledgment a reconciliation stop.
func deliverNotification(ctx context.Context, store notificationStore, preview notificationPreview, approval notificationApproval, sender notificationSender) (notificationDeliveryReport, error) {
	if preview.event == nil {
		return notificationDeliveryReport{State: notificationWireBlocked}, errors.New("notification preview was not constructed")
	}
	report := notificationDeliveryReport{State: notificationWireBlocked, Destination: preview.event.destination.label,
		RunID: preview.event.runID, Attempt: preview.event.attempt}
	if err := checkNotificationApproval(preview, approval); err != nil {
		report.Reason = err.Error()
		return report, err
	}
	if store == nil {
		return report, errors.New("notification delivery has no receipt store")
	}
	if sender == nil {
		return report, errors.New("notification delivery has no sender")
	}
	err := store.withLock(ctx, func(ledger notificationLedger) error {
		raw, err := ledger.load()
		if err != nil {
			return err
		}
		state, err := decodeNotificationReceipt(raw)
		if err != nil {
			return err
		}
		facts := state.facts()
		if facts.binding != preview.event.binding {
			return errors.New("notification receipt belongs to another template or destination; reconcile its delivery history")
		}
		if facts.workflowID != preview.event.workflowID {
			return errors.New("notification receipt belongs to another workflow identity")
		}
		if facts.workflowPath != preview.event.workflowPath {
			return errors.New("notification receipt belongs to another workflow path")
		}
		ready, ok := state.(notificationReadyReceipt)
		if !ok {
			report.Reason = "a previous delivery is unresolved; reconcile its attempted event before further delivery"
			return errors.New(report.Reason)
		}
		kind, text, reason := notificationTransition(preview, ready)
		report.Kind, report.Reason = kind.word(), reason
		if kind == notificationKindUnspecified {
			report.State = notificationWireQuiet
			if preview.verdict == notificationUnjudged {
				return nil
			}
			if preview.event.runID < facts.seenRunID {
				return nil
			}
			if preview.event.runID == facts.seenRunID {
				if preview.event.attempt <= facts.seenAttempt {
					return nil
				}
			}
			observed, err := ready.observe(preview)
			if err != nil {
				return err
			}
			return ledger.save(observed.encode())
		}
		flight, err := ready.prepare(preview, kind)
		if err != nil {
			return err
		}
		attempt, err := flight.persist(ledger)
		if err != nil {
			return fmt.Errorf("persist notification attempt before send: %w", err)
		}
		result, sendErr := attempt.send(ctx, sender, preview.event.destination, text)
		resolved, deliveryState, deliveryReason := attempt.resolve(preview, result, sendErr)
		report.State, report.Reason = deliveryState, deliveryReason
		if deliveryState == notificationWireDelivered {
			report.MessageID = result.MessageID
		}
		if err := ledger.save(resolved.encode()); err != nil {
			return fmt.Errorf("persist notification result; the durable attempted event remains unresolved: %w", err)
		}
		if report.State == notificationWireBlocked {
			return errors.New(report.Reason)
		}
		return nil
	})
	if err != nil {
		report.State = notificationWireBlocked
		if report.Reason == "" {
			report.Reason = err.Error()
		}
	}
	return report, err
}

func notificationTransition(preview notificationPreview, ready notificationReadyReceipt) (notificationTransitionKind, string, string) {
	receipt := ready.state
	if preview.event.runID < receipt.seenRunID {
		return notificationKindUnspecified, "", "older run remains separately identified"
	}
	if preview.event.runID == receipt.seenRunID {
		if preview.event.attempt <= receipt.seenAttempt {
			return notificationKindUnspecified, "", "this event was already processed"
		}
	}
	switch preview.verdict {
	case notificationFailed:
		if receipt.failureDigest == preview.fingerprint {
			return notificationKindUnspecified, "", "the delivered failure is unchanged"
		}
		return notificationKindFailure, preview.event.failureText, "initial or changed failure"
	case notificationPassed:
		if receipt.failureDigest == "" {
			return notificationKindUnspecified, "", "no delivered failure precedes this result"
		}
		if receipt.failureScope != preview.scope {
			return notificationKindUnspecified, "", "changed job scope does not establish recovery"
		}
		return notificationKindRecovery, preview.event.recoveryText, "the delivered failure's job scope recovered"
	case notificationUnspecified, notificationUnjudged:
		return notificationKindUnspecified, "", "active, incomplete or unusable evidence cannot establish a transition"
	default:
		panic("BUG: unknown validated notification evidence verdict")
	}
}
