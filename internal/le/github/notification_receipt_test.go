// Design: docs/contributing/github-integration.md -- validate external receipts before lifecycle conversion.
package github

import (
	"context"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"testing"
)

// TestNotificationReceiptBoundaryRejectsInvalidLifecycleCombinations proves
// semantically corrupt JSON state cannot authorize a send or invented recovery.
func TestNotificationReceiptBoundaryRejectsInvalidLifecycleCombinations(t *testing.T) {
	preview, approval := notificationTestPreview(t, notificationTestReport(101, true))
	initial, err := newNotificationReceipt(preview, approval)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []struct {
		name    string
		corrupt func(*notificationReceipt)
	}{
		{"failure without observed event", func(raw *notificationReceipt) {
			raw.FailureDigest, raw.FailureScope, raw.FailureMessageID = preview.fingerprint, preview.scope, "900"
		}},
		{"failure without attempted event", func(raw *notificationReceipt) {
			raw.SeenRunID, raw.SeenAttempt = 101, 1
			raw.FailureDigest, raw.FailureScope, raw.FailureMessageID = preview.fingerprint, preview.scope, "900"
		}},
		{"ready with unresolved reason", func(raw *notificationReceipt) { raw.StopReason = "unresolved" }},
		{"inflight without attempted event", func(raw *notificationReceipt) { raw.Mode = notificationReceiptInFlight }},
		{"blocked without attempted event", func(raw *notificationReceipt) { raw.Mode = notificationReceiptBlocked }},
		{"inflight with confirmation", func(raw *notificationReceipt) {
			raw.Mode, raw.StopReason = notificationReceiptInFlight, "unresolved"
			raw.SeenRunID, raw.SeenAttempt, raw.AttemptedRunID, raw.AttemptedAttempt = 101, 1, 101, 1
			raw.AttemptedKind, raw.MessageID = "failure", "900"
		}},
		{"inflight with mismatched current attempt", func(raw *notificationReceipt) {
			raw.Mode, raw.StopReason = notificationReceiptInFlight, "unresolved"
			raw.SeenRunID, raw.SeenAttempt, raw.AttemptedRunID, raw.AttemptedAttempt = 101, 2, 101, 1
			raw.AttemptedKind = "failure"
		}},
		{"unknown attempted kind", func(raw *notificationReceipt) {
			raw.SeenRunID, raw.SeenAttempt, raw.AttemptedRunID, raw.AttemptedAttempt = 101, 1, 101, 1
			raw.AttemptedKind = "unknown"
		}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			raw := initial
			fixture.corrupt(&raw)
			if _, err := decodeNotificationReceipt(raw); err == nil {
				t.Error("corrupt DTO became a private lifecycle state")
			}
			called := false
			sender := notificationSenderFixture(func(context.Context, notificationDestination, string) (notificationSendResult, error) {
				called = true
				return notificationSendResult{Outcome: notificationSendConfirmed, MessageID: "900"}, nil
			})
			if _, err := deliverNotification(t.Context(), &notificationMemoryStore{receipt: raw}, preview, approval, sender); err == nil {
				t.Error("corrupt DTO authorized delivery")
			}
			if called {
				t.Error("corrupt DTO reached sender")
			}
		})
	}
}

// TestNotificationReceiptKeepsDeliveredFailureAfterRejectedChangedFailure
// preserves valid older delivered history while recording the rejected attempt.
func TestNotificationReceiptKeepsDeliveredFailureAfterRejectedChangedFailure(t *testing.T) {
	preview, approval := notificationTestPreview(t, notificationTestReport(101, true))
	initial, err := newNotificationReceipt(preview, approval)
	if err != nil {
		t.Fatal(err)
	}
	store := &notificationMemoryStore{receipt: initial}
	sender := notificationSenderFixture(func(context.Context, notificationDestination, string) (notificationSendResult, error) {
		return notificationSendResult{Outcome: notificationSendConfirmed, MessageID: "900"}, nil
	})
	if _, err := deliverNotification(t.Context(), store, preview, approval, sender); err != nil {
		t.Fatal(err)
	}
	deliveredFailure := store.receipt.FailureDigest
	report := notificationTestReport(102, true)
	report.Runs[0].Jobs[0].Steps[0].Name = "Different failure"
	preview, approval = notificationTestPreview(t, report)
	rejected := notificationSenderFixture(func(context.Context, notificationDestination, string) (notificationSendResult, error) {
		return notificationSendResult{Outcome: notificationSendRejected}, nil
	})
	result, err := deliverNotification(t.Context(), store, preview, approval, rejected)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != "rejected" {
		t.Errorf("state = %s", result.State)
	}
	if store.receipt.FailureDigest != deliveredFailure {
		t.Error("rejection replaced confirmed failure history")
	}
	if store.receipt.FailureMessageID != "900" {
		t.Error("rejection lost prior confirmed message")
	}
	if store.receipt.AttemptedRunID != 102 {
		t.Error("rejection lost the attempted event")
	}
	state, err := decodeNotificationReceipt(store.receipt)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := state.(notificationReadyReceipt); !ok {
		t.Error("known rejected delivery did not return a validated ready variant")
	}
}

// TestNotificationFailureIdentitiesCannotAliasThroughColonNames keeps the
// same scope while moving failure between tuples whose colon joins collide.
func TestNotificationFailureIdentitiesCannotAliasThroughColonNames(t *testing.T) {
	makeReport := func(id uint64, firstFails bool) Report {
		report := notificationTestReport(id, true)
		first, second := "success", "failure"
		if firstFails {
			first, second = "failure", "success"
		}
		report.Runs[0].Jobs = []Job{
			{ID: 1, RunID: id, Name: "a", Status: "completed", Conclusion: "failure",
				Steps: []Step{{Number: 1, Name: "2:b", Status: "completed", Conclusion: first}}},
			{ID: 2, RunID: id, Name: "a:1", Status: "completed", Conclusion: "failure",
				Steps: []Step{{Number: 2, Name: "b", Status: "completed", Conclusion: second}}},
		}
		return report
	}
	preview, approval := notificationTestPreview(t, makeReport(101, true))
	initial, err := newNotificationReceipt(preview, approval)
	if err != nil {
		t.Fatal(err)
	}
	store := &notificationMemoryStore{receipt: initial}
	sends := 0
	sender := notificationSenderFixture(func(context.Context, notificationDestination, string) (notificationSendResult, error) {
		sends++
		return notificationSendResult{Outcome: notificationSendConfirmed, MessageID: "900"}, nil
	})
	if _, err := deliverNotification(t.Context(), store, preview, approval, sender); err != nil {
		t.Fatal(err)
	}
	preview, approval = notificationTestPreview(t, makeReport(102, false))
	result, err := deliverNotification(t.Context(), store, preview, approval, sender)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != "delivered" {
		t.Error("distinct colon-containing failure identities were deduplicated")
	}
	if sends != 2 {
		t.Errorf("sends = %d, want two different failure transitions", sends)
	}
}

// TestNotificationReceiptDigestErrorsPreserveFieldAndDecodeCause exercises
// actual receipt consumers and keeps parse failure distinct from missing data.
func TestNotificationReceiptDigestErrorsPreserveFieldAndDecodeCause(t *testing.T) {
	preview, approval := notificationTestPreview(t, notificationTestReport(101, true))
	base, err := newNotificationReceipt(preview, approval)
	if err != nil {
		t.Fatal(err)
	}
	base.SeenRunID, base.SeenAttempt, base.AttemptedRunID, base.AttemptedAttempt = 101, 1, 101, 1
	base.AttemptedKind, base.FailureMessageID, base.MessageID = "failure", "900", "900"
	base.FailureDigest, base.FailureScope = preview.fingerprint, preview.scope
	if err := notificationReceiptValid(base); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []struct {
		role    string
		corrupt func(*notificationReceipt)
	}{
		{"approval binding", func(raw *notificationReceipt) { raw.BindingDigest = strings.Repeat("g", 64) }},
		{"delivered-failure digest", func(raw *notificationReceipt) { raw.FailureDigest = strings.Repeat("g", 64) }},
		{"delivered-failure scope", func(raw *notificationReceipt) { raw.FailureScope = strings.Repeat("g", 64) }},
	} {
		t.Run(fixture.role, func(t *testing.T) {
			raw := base
			fixture.corrupt(&raw)
			err := notificationReceiptValid(raw)
			if err == nil {
				t.Fatal("invalid digest accepted")
			}
			if _, ok := errors.AsType[hex.InvalidByteError](err); !ok {
				t.Error("hexadecimal decode cause was discarded")
			}
			if !strings.Contains(err.Error(), fixture.role) {
				t.Error("receipt field role was lost")
			}
		})
	}
	base.BindingDigest = "a"
	if err := notificationReceiptValid(base); err == nil || !strings.Contains(err.Error(), "64 hexadecimal") {
		t.Error("digest length failure was not identified")
	}
	if _, err := newNotificationDestination("maintainers", "private-token", "456"); err == nil {
		t.Error("invalid destination identity accepted")
	} else {
		if !errors.Is(err, strconv.ErrSyntax) {
			t.Error("identity parse cause was discarded")
		}
		if strings.Contains(err.Error(), "private-token") {
			t.Error("identity validation disclosed untrusted input")
		}
	}
}
