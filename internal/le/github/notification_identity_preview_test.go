// Design: docs/contributing/github-integration.md -- identity previews compute a comparison binding without approval or delivery.
package github

import (
	"context"
	"testing"

	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// TestNotificationIdentityPreviewBindsPairedDestination drives the registered
// action factory with token-free identities and compares its exact binding.
func TestNotificationIdentityPreviewBindsPairedDestination(t *testing.T) {
	action := notificationPreviewAction(notificationActivationTestRead)
	payload, code := leaction.New(area, action).Answer([]string{"preview", "run", "101", "destination", "maintainers", "channel", "123", "webhook", "456"})
	if code != 0 {
		t.Fatalf("identity preview code = %d", code)
	}
	preview, ok := payload.(notificationPreview)
	if !ok {
		t.Fatalf("identity preview payload = %T", payload)
	}
	expected, _ := notificationTestPreview(t, notificationTestReport(101, true))
	if preview.ApprovalDigest != expected.ApprovalDigest {
		t.Error("identity preview did not bind the exact destination")
	}
	if preview.ChannelID != "123" {
		t.Error("identity preview lost channel identity")
	}
	if preview.WebhookID != "456" {
		t.Error("identity preview lost webhook identity")
	}
	if preview.DeliveryState != "preview-only" {
		t.Error("comparison identity issued approval or delivery")
	}
}

// TestNotificationIdentityPreviewRequiresBothIdentities proves a partial
// destination refuses before reading workflow evidence.
func TestNotificationIdentityPreviewRequiresBothIdentities(t *testing.T) {
	for _, keyword := range []string{"channel", "webhook"} {
		reads := 0
		action := notificationPreviewAction(func(context.Context, uint64) (Report, error) {
			reads++
			return notificationTestReport(101, true), nil
		})
		if _, code := leaction.New(area, action).Answer([]string{"preview", "run", "101", "destination", "maintainers", keyword, "123"}); code != 2 {
			t.Errorf("partial %s preview code = %d", keyword, code)
		}
		if reads != 0 {
			t.Errorf("partial %s preview read evidence", keyword)
		}
	}
}
