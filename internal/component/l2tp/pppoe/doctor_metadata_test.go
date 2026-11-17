// Design: docs/guide/monitoring.md -- retained PPP admission cleanup failures
// Related: realm_relay.go -- actual realm handback failure producer
package pppoe

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// TestRealmAdmissionReleaseFailureReachesDoctor drives a genuine admitted
// request through the realm actor's handback callback. A failed handback must
// retain its subscriber, access and cause in both operator sinks; a successful
// handback must leave both sinks empty. Neither result invents native identity.
func TestRealmAdmissionReleaseFailureReachesDoctor(t *testing.T) {
	for _, question := range []struct {
		name    string
		failure error
	}{
		{name: "failed handback", failure: errors.New("original admission retirement is unconfirmed")},
		{name: "successful handback"},
	} {
		t.Run(question.name, func(t *testing.T) {
			report.ResetForTest()
			t.Cleanup(report.ResetForTest)
			h := newRealmHarness(t)
			var log bytes.Buffer
			h.relay.logger = slog.New(slog.NewJSONHandler(&log, nil))
			var captured ppp.EventAuthRequest
			h.relay.released = func(request ppp.EventAuthRequest) error {
				captured = request
				return question.failure
			}
			h.handoff().Bridged(nil)
			if question.failure != nil {
				waitSignal(t, h.sink.canceled, "the failed admission call cancel")
			} else {
				h.handoff().Ended("the remote call ended")
				waitSignal(t, h.ended, "the successfully relayed session end")
			}
			h.relay.stopAll()
			if captured.Control.Attempt() != h.attempt {
				t.Fatal("handback callback did not retain the original admitted request")
			}
			released, refusals := h.driver.counts()
			if released != 1 {
				t.Fatalf("released=%d, want one release", released)
			}
			if len(refusals) != 0 {
				t.Fatalf("authentication refusals=%v, want none", refusals)
			}
			warnings := report.Warnings()
			if question.failure == nil {
				if len(warnings) != 0 {
					t.Fatalf("successful handback raised cleanup warnings: %+v", warnings)
				}
				if log.Len() != 0 {
					t.Fatalf("successful handback logged a failure: %s", log.Bytes())
				}
				return
			}
			if len(warnings) != 1 {
				t.Fatalf("handback warnings = %+v, want one retained cleanup failure", warnings)
			}
			issue := warnings[0]
			if issue.Source != ppp.CleanupReportSource {
				t.Fatalf("handback warning has the wrong source: %+v", issue)
			}
			if issue.Code != diagnostic.CodeDoctorPPPCleanup {
				t.Fatalf("handback warning does not name the PPP cleanup doctor: %+v", issue)
			}
			if issue.Fix != ppp.CleanupFailureFix {
				t.Errorf("warning fix = %q, want %q", issue.Fix, ppp.CleanupFailureFix)
			}
			for _, identity := range []string{captured.SubscriberID, "access interface 3", "example.net"} {
				if !strings.Contains(issue.Message, identity) {
					t.Errorf("doctor message %q lost original identity %q", issue.Message, identity)
				}
			}
			if issue.Detail["subscriber-id"] != captured.SubscriberID {
				t.Errorf("original subscriber lost: %+v", issue.Detail)
			}
			if issue.Detail["ifindex"] != 3 {
				t.Errorf("original access lost: %+v", issue.Detail)
			}
			if issue.Detail["realm"] != "example.net" {
				t.Errorf("original realm lost: %+v", issue.Detail)
			}
			if issue.Detail["error"] != question.failure.Error() {
				t.Errorf("handback cause lost: %+v", issue.Detail)
			}
			if _, invented := issue.Detail["native-index"]; invented {
				t.Errorf("handback invented native identity: %+v", issue.Detail)
			}
			var entry map[string]any
			if err := json.Unmarshal(log.Bytes(), &entry); err != nil {
				t.Fatal(err)
			}
			if entry["code"] != diagnostic.CodeDoctorPPPCleanup {
				t.Errorf("log doctor code lost: %+v", entry)
			}
			if entry["fix"] != ppp.CleanupFailureFix {
				t.Errorf("log fix lost: %+v", entry)
			}
		})
	}
}
