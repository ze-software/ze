// Design: docs/contributing/github-integration.md -- preview and approved failure/recovery state.
package github

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	leaction "github.com/ze-software/ze/internal/le/le/action"
)

func notificationTestReport(id uint64, failed bool) Report {
	repository := Repository
	conclusion := "success"
	if failed {
		conclusion = "failure"
	}
	return Report{Repository: Repository, Revision: testRevision, RequestedRunID: id, Complete: true,
		Runs: []Run{{ID: id, WorkflowID: 7, Name: "verify", Revision: testRevision, Attempt: 1,
			HeadRepository: &repository, HeadBranch: "main", WorkflowPath: ".github/workflows/verify.yml",
			Event: "push", Status: "completed", Conclusion: conclusion,
			URL: "https://github.com/" + Repository + "/actions/runs/" + strconv.FormatUint(id, 10), JobsComplete: true,
			Jobs: []Job{{ID: 1, RunID: id, Name: "verify shard 1", Status: "completed", Conclusion: conclusion,
				Steps: []Step{{Number: 1, Name: "Verify", Status: "completed", Conclusion: conclusion}}}}}}}
}

func notificationTestPreview(t *testing.T, report Report) (notificationPreview, notificationApproval) {
	t.Helper()
	destination, err := newNotificationDestination("maintainers", "123", "456")
	if err != nil {
		t.Fatal(err)
	}
	template, err := candidateNotificationTemplate()
	if err != nil {
		t.Fatal(err)
	}
	preview, err := buildNotificationPreview(report, template, destination)
	if err != nil {
		t.Fatal(err)
	}
	return preview, notificationApproval{BindingDigest: preview.ApprovalDigest}
}

type notificationMemoryStore struct {
	receipt  notificationReceipt
	loadErr  error
	failSave int
	saves    int
	locked   bool
}

func (store *notificationMemoryStore) withLock(_ context.Context, use func(notificationLedger) error) error {
	if store.locked {
		return errors.New("fixture scope already locked")
	}
	store.locked = true
	defer func() { store.locked = false }()
	return use(store)
}
func (store *notificationMemoryStore) load() (notificationReceipt, error) {
	return store.receipt, store.loadErr
}
func (store *notificationMemoryStore) save(receipt notificationReceipt) error {
	store.saves++
	if store.saves == store.failSave {
		return errors.New("fixture durable write failed")
	}
	store.receipt = receipt
	return nil
}

type notificationSenderFixture func(context.Context, notificationDestination, string) (notificationSendResult, error)

func (send notificationSenderFixture) Send(ctx context.Context, destination notificationDestination, text string) (notificationSendResult, error) {
	return send(ctx, destination, text)
}

// TestNotificationPreviewNeverSends drives the production action factory and
// reader seam, requiring rendered candidates with no delivery-capable argument.
func TestNotificationPreviewNeverSends(t *testing.T) {
	read := 0
	action := notificationPreviewAction(func(_ context.Context, id uint64) (Report, error) {
		read++
		return notificationTestReport(id, true), nil
	})
	payload, code := leaction.New(area, action).Answer([]string{"preview", "run", "101", "destination", "pending-owner-approval"})
	if code != 0 {
		t.Fatalf("preview code = %d", code)
	}
	if read != 1 {
		t.Fatalf("preview read %d reports", read)
	}
	preview, ok := payload.(notificationPreview)
	if !ok {
		t.Fatalf("preview payload = %T", payload)
	}
	if preview.DeliveryState != "preview-only" {
		t.Errorf("delivery state = %s", preview.DeliveryState)
	}
	if len(preview.Candidates) != 2 {
		t.Fatal("preview did not render both candidate transitions")
	}
	if !strings.Contains(preview.Candidates[0].Text, preview.Run.URL) {
		t.Error("failure preview omits the selected run link")
	}
	if _, err := newNotificationReceipt(preview, notificationApproval{BindingDigest: preview.ApprovalDigest}); err == nil {
		t.Error("a label-only preview authorized a delivery ledger")
	}
	found := false
	for _, registered := range Actions().Actions {
		if registered.Verb == "preview" {
			found = true
		}
		if registered.Verb == "send" {
			t.Error("delivery was exposed through the CLI before separate activation")
		}
	}
	if !found {
		t.Error("the production GitHub action table does not register preview")
	}
}

// TestNotificationTransitionsAndAmbiguousDelivery proves confirmed failures
// deduplicate, recovery follows delivery, and uncertainty persists across calls.
func TestNotificationTransitionsAndAmbiguousDelivery(t *testing.T) {
	preview, approval := notificationTestPreview(t, notificationTestReport(101, true))
	receipt, err := newNotificationReceipt(preview, approval)
	if err != nil {
		t.Fatal(err)
	}
	store := &notificationMemoryStore{receipt: receipt}
	var texts []string
	sender := notificationSenderFixture(func(_ context.Context, _ notificationDestination, text string) (notificationSendResult, error) {
		if store.receipt.Mode != notificationReceiptInFlight {
			t.Error("sender ran before a durable attempted-event receipt")
		}
		texts = append(texts, text)
		return notificationSendResult{Outcome: notificationSendConfirmed, MessageID: "900"}, nil
	})
	for _, event := range []struct {
		id     uint64
		failed bool
		want   string
	}{
		{101, true, "delivered"}, {102, true, "quiet"}, {103, false, "delivered"}, {104, false, "quiet"},
	} {
		preview, approval = notificationTestPreview(t, notificationTestReport(event.id, event.failed))
		result, err := deliverNotification(t.Context(), store, preview, approval, sender)
		if err != nil {
			t.Fatal(err)
		}
		if result.State != event.want {
			t.Errorf("event %d state = %s, want %s", event.id, result.State, event.want)
		}
	}
	if len(texts) != 2 {
		t.Fatalf("sent %d messages, want one failure and one recovery", len(texts))
	}
	if !strings.Contains(texts[1], "recovered") {
		t.Error("the second delivered message was not recovery")
	}
	preview, approval = notificationTestPreview(t, notificationTestReport(105, true))
	unknown := notificationSenderFixture(func(context.Context, notificationDestination, string) (notificationSendResult, error) {
		texts = append(texts, "attempted")
		return notificationSendResult{Outcome: notificationSendAmbiguous}, errors.New("private transport detail must not be persisted")
	})
	if result, err := deliverNotification(t.Context(), store, preview, approval, unknown); err == nil {
		t.Fatalf("ambiguous attempt succeeded: %+v", result)
	}
	for _, id := range []uint64{105, 106} {
		preview, approval = notificationTestPreview(t, notificationTestReport(id, true))
		if _, err := deliverNotification(t.Context(), store, preview, approval, unknown); err == nil {
			t.Error("unresolved delivery was retried")
		}
	}
	if len(texts) != 3 {
		t.Error("ambiguous delivery caused another send")
	}
	if strings.Contains(store.receipt.StopReason, "private transport") {
		t.Error("private transport details entered the receipt")
	}
}

// TestNotificationApprovalAndDurabilityFailClosed refuses changed bindings and
// storage failures before a message, and keeps failed confirmation writes blocked.
func TestNotificationApprovalAndDurabilityFailClosed(t *testing.T) {
	preview, approval := notificationTestPreview(t, notificationTestReport(101, true))
	receipt, err := newNotificationReceipt(preview, approval)
	if err != nil {
		t.Fatal(err)
	}
	sends := 0
	sender := notificationSenderFixture(func(context.Context, notificationDestination, string) (notificationSendResult, error) {
		sends++
		return notificationSendResult{Outcome: notificationSendConfirmed, MessageID: "900"}, nil
	})
	changed, err := newNotificationTemplate("Changed {workflow} {run_url}", "Changed recovery {run_url}")
	if err != nil {
		t.Fatal(err)
	}
	newPreview, err := buildNotificationPreview(notificationTestReport(101, true), changed, preview.event.destination)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := deliverNotification(t.Context(), &notificationMemoryStore{receipt: receipt}, newPreview, approval, sender); err == nil {
		t.Error("changed template retained an earlier approval")
	}
	destination, err := newNotificationDestination("maintainers", "124", "457")
	if err != nil {
		t.Fatal(err)
	}
	template, err := candidateNotificationTemplate()
	if err != nil {
		t.Fatal(err)
	}
	newPreview, err = buildNotificationPreview(notificationTestReport(101, true), template, destination)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := deliverNotification(t.Context(), &notificationMemoryStore{receipt: receipt}, newPreview, approval, sender); err == nil {
		t.Error("changed destination retained an earlier approval")
	}
	store := &notificationMemoryStore{receipt: receipt, failSave: 1}
	if _, err := deliverNotification(t.Context(), store, preview, approval, sender); err == nil {
		t.Error("failed pre-send persistence authorized a send")
	}
	if sends != 0 {
		t.Fatal("a message was sent before exact approval and durable intent")
	}
	store = &notificationMemoryStore{receipt: receipt, failSave: 2}
	if _, err := deliverNotification(t.Context(), store, preview, approval, sender); err == nil {
		t.Error("failed confirmation persistence claimed success")
	}
	if _, err := deliverNotification(t.Context(), store, preview, approval, sender); err == nil {
		t.Error("in-flight durable receipt allowed a retry")
	}
	if sends != 1 {
		t.Errorf("confirmation write failure produced %d sends", sends)
	}
}

// TestNotificationTrustAndRecoveryScopeKeepUnusableEvidenceQuiet exercises
// near misses of trusted identity and a successful result with skipped evidence.
func TestNotificationTrustAndRecoveryScopeKeepUnusableEvidenceQuiet(t *testing.T) {
	for _, mutate := range []func(*Report){
		func(report *Report) { report.Repository = "someone/ze" },
		func(report *Report) { report.Runs[0].HeadRepository = nil },
		func(report *Report) { report.Runs[0].HeadBranch = "topic" },
		func(report *Report) { report.Runs[0].Event = "pull_request" },
		func(report *Report) { report.Runs[0].WorkflowPath = ".github/workflows/unknown.yml" },
	} {
		report := notificationTestReport(101, true)
		mutate(&report)
		destination, err := newNotificationDestination("preview", "", "")
		if err != nil {
			t.Fatal(err)
		}
		template, err := candidateNotificationTemplate()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := buildNotificationPreview(report, template, destination); err == nil {
			t.Error("untrusted evidence produced a trusted preview")
		}
	}
	preview, approval := notificationTestPreview(t, notificationTestReport(101, true))
	receipt, err := newNotificationReceipt(preview, approval)
	if err != nil {
		t.Fatal(err)
	}
	store := &notificationMemoryStore{receipt: receipt}
	sends := 0
	sender := notificationSenderFixture(func(context.Context, notificationDestination, string) (notificationSendResult, error) {
		sends++
		return notificationSendResult{Outcome: notificationSendConfirmed, MessageID: "900"}, nil
	})
	if _, err := deliverNotification(t.Context(), store, preview, approval, sender); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Report){
		func(report *Report) { report.Complete = false },
		func(report *Report) { report.Runs[0].Jobs[0].Conclusion = "skipped" },
		func(report *Report) { report.Runs[0].Jobs[0].Name = "different scope" },
	} {
		report := notificationTestReport(102, false)
		mutate(&report)
		preview, approval = notificationTestPreview(t, report)
		result, err := deliverNotification(t.Context(), store, preview, approval, sender)
		if err != nil {
			t.Fatal(err)
		}
		if result.State != "quiet" {
			t.Error("incomplete or changed evidence claimed recovery")
		}
	}
	if sends != 1 {
		t.Errorf("unusable recovery evidence sent %d messages", sends)
	}
}

// TestNotificationRecoveryRequiresSuccessfulStepEvidence refuses a successful
// job whose steps did not establish the recovery represented by that verdict.
func TestNotificationRecoveryRequiresSuccessfulStepEvidence(t *testing.T) {
	for _, fixture := range []struct {
		name  string
		steps []Step
	}{
		{"missing steps", nil},
		{"failed continued step", []Step{{Number: 1, Name: "Verify", Status: "completed", Conclusion: "failure"}}},
		{"timed out step", []Step{{Number: 1, Name: "Verify", Status: "completed", Conclusion: "timed_out"}}},
		{"skipped step", []Step{{Number: 1, Name: "Verify", Status: "completed", Conclusion: "skipped"}}},
		{"unknown step", []Step{{Number: 1, Name: "Verify", Status: "completed", Conclusion: "unknown"}}},
		{"missing conclusion", []Step{{Number: 1, Name: "Verify", Status: "completed"}}},
		{"unfinished step", []Step{{Number: 1, Name: "Verify", Status: "in_progress", Conclusion: "success"}}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			failure, approval := notificationTestPreview(t, notificationTestReport(101, true))
			receipt, err := newNotificationReceipt(failure, approval)
			if err != nil {
				t.Fatal(err)
			}
			store := &notificationMemoryStore{receipt: receipt}
			sends := 0
			sender := notificationSenderFixture(func(context.Context, notificationDestination, string) (notificationSendResult, error) {
				sends++
				return notificationSendResult{Outcome: notificationSendConfirmed, MessageID: "900"}, nil
			})
			if _, err := deliverNotification(t.Context(), store, failure, approval, sender); err != nil {
				t.Fatal(err)
			}
			report := notificationTestReport(102, false)
			report.Runs[0].Jobs[0].Steps = fixture.steps
			preview, approval := notificationTestPreview(t, report)
			result, err := deliverNotification(t.Context(), store, preview, approval, sender)
			if err != nil {
				t.Fatal(err)
			}
			if result.State != "quiet" {
				t.Errorf("unproved recovery state = %s", result.State)
			}
			if store.receipt.FailureDigest == "" {
				t.Error("unproved step evidence cleared the delivered failure")
			}
			if sends != 1 {
				t.Errorf("unproved recovery sent %d messages, want initial failure only", sends)
			}
		})
	}
}

// TestNotificationRecoveryRetainsFailureWhenRecordedStepScopeShrinks compares
// job-associated step identities, without claiming their commands are unchanged.
func TestNotificationRecoveryRetainsFailureWhenRecordedStepScopeShrinks(t *testing.T) {
	report := notificationTestReport(101, true)
	report.Runs[0].Jobs[0].Steps = []Step{
		{Number: 1, Name: "Setup", Status: "completed", Conclusion: "success"},
		{Number: 2, Name: "Verify", Status: "completed", Conclusion: "failure"},
	}
	failure, approval := notificationTestPreview(t, report)
	receipt, err := newNotificationReceipt(failure, approval)
	if err != nil {
		t.Fatal(err)
	}
	store := &notificationMemoryStore{receipt: receipt}
	sends := 0
	sender := notificationSenderFixture(func(context.Context, notificationDestination, string) (notificationSendResult, error) {
		sends++
		return notificationSendResult{Outcome: notificationSendConfirmed, MessageID: "900"}, nil
	})
	if _, err := deliverNotification(t.Context(), store, failure, approval, sender); err != nil {
		t.Fatal(err)
	}
	success := notificationTestReport(102, false)
	success.Runs[0].Jobs[0].Steps = []Step{{Number: 1, Name: "Setup", Status: "completed", Conclusion: "success"}}
	preview, approval := notificationTestPreview(t, success)
	result, err := deliverNotification(t.Context(), store, preview, approval, sender)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != "quiet" {
		t.Error("removing a failed step established recovery")
	}
	if store.receipt.FailureDigest == "" {
		t.Error("removing a failed step cleared delivered failure")
	}
	if sends != 1 {
		t.Error("smaller evidence scope sent recovery")
	}
	success = notificationTestReport(103, false)
	success.Runs[0].Jobs[0].Steps = []Step{
		{Number: 1, Name: "Setup", Status: "completed", Conclusion: "success"},
		{Number: 2, Name: "Verify", Status: "completed", Conclusion: "success"},
	}
	preview, approval = notificationTestPreview(t, success)
	result, err = deliverNotification(t.Context(), store, preview, approval, sender)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != "delivered" {
		t.Errorf("same step scope did not recover: %s", result.State)
	}
	if store.receipt.FailureDigest != "" {
		t.Error("confirmed recovery did not clear failure")
	}
	if sends != 2 {
		t.Errorf("sends = %d, want failure and same-scope recovery", sends)
	}
}
