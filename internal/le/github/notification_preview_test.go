// Design: docs/contributing/github-integration.md -- preview presentation cannot alter delivery authority.
package github

import (
	"context"
	"testing"
)

// TestNotificationPreviewPresentationCannotChangeDeliveryAuthority exercises
// the public Run pointer independently of the privately owned event snapshot.
func TestNotificationPreviewPresentationCannotChangeDeliveryAuthority(t *testing.T) {
	preview, approval := notificationTestPreview(t, notificationTestReport(101, true))
	receipt, err := newNotificationReceipt(preview, approval)
	if err != nil {
		t.Fatal(err)
	}
	expectedText := preview.Candidates[0].Text
	preview.Run.ID = 999
	preview.Run.Name = "changed display"
	preview.Run.URL = "https://example.com/untrusted"
	preview.Run = nil
	preview.Candidates[0].Text = "changed display candidate"
	sends := 0
	sender := notificationSenderFixture(func(_ context.Context, _ notificationDestination, text string) (notificationSendResult, error) {
		sends++
		if text != expectedText {
			t.Error("display mutation changed privately captured text")
		}
		return notificationSendResult{Outcome: notificationSendConfirmed, MessageID: "900"}, nil
	})
	result, err := deliverNotification(t.Context(), &notificationMemoryStore{receipt: receipt}, preview, approval, sender)
	if err != nil {
		t.Fatal(err)
	}
	if result.RunID != 101 {
		t.Error("display mutation changed selected event")
	}
	if sends != 1 {
		t.Error("constructed preview lost its private authority")
	}
	if _, err := newNotificationReceipt(notificationPreview{}, approval); err == nil {
		t.Error("zero preview initialized state")
	}
	if _, err := deliverNotification(t.Context(), &notificationMemoryStore{receipt: receipt}, notificationPreview{}, approval, sender); err == nil {
		t.Error("zero preview authorized a sender")
	}
	if sends != 1 {
		t.Error("zero preview reached sender")
	}
}
