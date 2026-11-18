// Design: docs/contributing/github-integration.md -- activation refusals expose a registered readiness diagnosis without credentials.
package github

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestNotificationActivationReadinessDiagnostic drives absent authority and
// absent transport through the actual action factory and checks safe reports.
func TestNotificationActivationReadinessDiagnostic(t *testing.T) {
	if diagnostic.Lookup("github-notification-unavailable") == nil {
		t.Fatal("notification readiness code has no registered description")
	}
	fixture := notificationActivationTestFixture(t)
	notificationActivationTestReady(t, fixture)
	for _, config := range []string{fixture.config + ".missing", fixture.config} {
		action := notificationActivationTestArea(notificationActivationTestRead, nil)
		payload, code := action.Answer([]string{"delivery", "run", "101", "config", config})
		if code == 0 {
			t.Fatal("missing authority or transport reported readiness")
		}
		refusal, ok := payload.(notificationActivationRefusal)
		if !ok {
			t.Fatalf("refusal payload = %T", payload)
		}
		if refusal.Diagnostic != "github-notification-unavailable" {
			t.Errorf("readiness diagnosis = %s", refusal.Diagnostic)
		}
		if refusal.State != "blocked" {
			t.Errorf("unavailable readiness state = %s", refusal.State)
		}
		body, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), "fixture-private-token") {
			t.Error("credential reached the readiness report")
		}
	}
}
