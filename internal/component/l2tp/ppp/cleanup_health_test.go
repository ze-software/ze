// Design: docs/guide/monitoring.md -- retained PPP cleanup failures

package ppp

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// Goal: a failed cleanup retains the created interface and wrapped cause.
// Method: report an actual handle identity and inspect the bounded report bus.
func TestCleanupFailureRetainsIdentityAndCause(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	identity := SessionIdentity{InterfaceName: "pppoe_session17", NativeIndex: 27}
	cause := fmt.Errorf("remove peer route: %w", errors.New("VPP connection lost"))
	ReportCleanupFailure(identity, CleanupSession, cause)
	warnings := report.Warnings()
	if len(warnings) != 1 {
		t.Fatalf("warnings = %+v", warnings)
	}
	issue := warnings[0]
	if issue.Source != CleanupReportSource {
		t.Fatalf("source = %q", issue.Source)
	}
	if issue.Code != diagnostic.CodeDoctorPPPCleanup {
		t.Fatalf("code = %q", issue.Code)
	}
	if issue.Fix != CleanupFailureFix {
		t.Fatalf("fix = %q", issue.Fix)
	}
	if issue.Detail["interface"] != identity.InterfaceName {
		t.Fatalf("interface detail = %v", issue.Detail)
	}
	if issue.Detail["native-index"] != identity.NativeIndex {
		t.Fatalf("index detail = %v", issue.Detail)
	}
	if issue.Detail["error"] != cause.Error() {
		t.Fatalf("cause detail = %v", issue.Detail)
	}
	if !strings.Contains(issue.Message, "removal is unconfirmed") {
		t.Fatalf("message = %q", issue.Message)
	}
}

// Goal: success, including success for the same identity, cannot erase an
// unconfirmed removal. Method: report both nil outcomes after a real failure.
func TestCleanupSuccessDoesNotClearFailure(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	identity := SessionIdentity{InterfaceName: "ppp7", NativeIndex: 9}
	ReportCleanupFailure(identity, CleanupIPv6, errors.New("route removal failed"))
	ReportCleanupFailure(SessionIdentity{InterfaceName: "ppp8", NativeIndex: 10}, CleanupIPv6, nil)
	ReportCleanupFailure(identity, CleanupIPv6, nil)
	if warnings := report.Warnings(); len(warnings) != 1 {
		t.Fatalf("success erased failure: %+v", warnings)
	}
}

// Goal: repeated failure of one resource updates its record rather than
// growing state. Method: report two causes under the same identity and step.
func TestCleanupFailureDeduplicatesItsResource(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	identity := SessionIdentity{InterfaceName: "ppp7", NativeIndex: 9}
	ReportCleanupFailure(identity, CleanupTransport, errors.New("first close failed"))
	ReportCleanupFailure(identity, CleanupTransport, errors.New("second close failed"))
	warnings := report.Warnings()
	if len(warnings) != 1 {
		t.Fatalf("warnings = %+v", warnings)
	}
	if warnings[0].Detail["error"] != "second close failed" {
		t.Fatalf("latest cause = %v", warnings[0].Detail)
	}
}

// Goal: a backend name longer than the bus's subject bound cannot hide a
// cleanup failure. Method: retain two distinct long names with one native index.
func TestCleanupFailureLongIdentityRemainsVisible(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	name := strings.Repeat("long-interface-", 30)
	for _, suffix := range []string{"first", "second"} {
		ReportCleanupFailure(SessionIdentity{InterfaceName: name + suffix, NativeIndex: 9}, CleanupTransport, errors.New("close failed"))
	}
	warnings := report.Warnings()
	if len(warnings) != 2 {
		t.Fatalf("long names hid a failure: %+v", warnings)
	}
	for _, issue := range warnings {
		if len(issue.Subject) > report.SubjectMax {
			t.Fatalf("subject exceeds report bound: %d", len(issue.Subject))
		}
		actual, ok := issue.Detail["interface"].(string)
		if !ok {
			t.Fatalf("identity missing: %v", issue.Detail)
		}
		if !strings.HasPrefix(actual, name) {
			t.Fatalf("identity changed: %q", actual)
		}
	}
}
