// Design: docs/guide/monitoring.md -- independently retained PPP cleanup causes

package ppp

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/core/report"
)

// Final network cleanup and owner release concern the same created interface,
// but either cause must remain inspectable if both operations fail.
func TestCleanupNetworkAndSessionFailuresRemainDistinct(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	identity := SessionIdentity{InterfaceName: "pppoe_session17", NativeIndex: 27}
	ReportCleanupFailure(identity, CleanupNetwork, errors.New("network address withdrawal refused"))
	ReportCleanupFailure(identity, CleanupSession, errors.New("session owner release refused"))
	warnings := report.Warnings()
	if len(warnings) != 2 {
		t.Fatalf("retained warnings = %+v, want both cleanup operations", warnings)
	}
	causes := make(map[string]string)
	for _, warning := range warnings {
		operation, ok := warning.Detail["operation"].(string)
		if !ok {
			t.Fatalf("operation = %v, want string", warning.Detail["operation"])
		}
		cause, ok := warning.Detail["error"].(string)
		if !ok {
			t.Fatalf("cause = %v, want string", warning.Detail["error"])
		}
		causes[operation] = cause
	}
	if causes["network"] != "network address withdrawal refused" || causes["session"] != "session owner release refused" {
		t.Fatalf("retained operation causes = %v", causes)
	}
}
