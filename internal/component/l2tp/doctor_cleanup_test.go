// Design: docs/guide/monitoring.md -- PPP cleanup doctor and health entry points

package l2tp

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/doctor"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/health"
	"github.com/ze-software/ze/internal/core/report"
)

// Goal: the daemon's doctor runner reaches the registered cleanup check.
// Method: raise one failure through PPP and ask the same runtime runner as show doctor.
func TestPPPCleanupDoctorRuntimeEntry(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	diagnostic.RegisterBuiltinCodes()
	if diagnostic.Lookup(diagnostic.CodeDoctorPPPCleanup) == nil {
		t.Fatal("cleanup code has no explanation")
	}
	ppp.ReportCleanupFailure(ppp.SessionIdentity{InterfaceName: "ppp7", NativeIndex: 9}, ppp.CleanupSession,
		fmt.Errorf("delete interface: %w", errors.New("backend unavailable")))
	var own []diagnostic.Diagnostic
	for _, finding := range doctor.RunRuntimeChecks(nil, nil) {
		if finding.Code == diagnostic.CodeDoctorPPPCleanup {
			own = append(own, finding)
		}
	}
	if len(own) != 1 {
		t.Fatalf("runtime cleanup findings = %+v", own)
	}
	if !strings.Contains(own[0].Message, "ppp7") {
		t.Fatalf("missing interface: %+v", own[0])
	}
	if !strings.Contains(own[0].Message, "delete interface: backend unavailable") {
		t.Fatalf("missing cause: %+v", own[0])
	}
	if own[0].Fix != ppp.CleanupFailureFix {
		t.Fatalf("missing recovery command: %+v", own[0])
	}
}

// Goal: the local process must not mistake its empty runtime state for the
// daemon's cleanup status. Method: inspect the real registration's phase.
func TestPPPCleanupDoctorRegistersOnlyRuntime(t *testing.T) {
	found := false
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhaseRuntime) {
		if check.Name == pppCleanupCheckName {
			found = true
		}
	}
	if !found {
		t.Fatal("runtime doctor does not declare PPP cleanup")
	}
	for _, phase := range []diagnostic.DoctorCheckPhase{diagnostic.DoctorPhasePreConfig, diagnostic.DoctorPhaseMissingConfig,
		diagnostic.DoctorPhaseParsedConfig, diagnostic.DoctorPhasePostConfig} {
		for _, check := range diagnostic.DoctorChecksForPhase(phase) {
			if check.Name == pppCleanupCheckName {
				t.Fatalf("cleanup reads a foreign process at phase %s", phase)
			}
		}
	}
}

// Goal: show health's registered row reads the same retained warning.
// Method: run the process health registry after a PPP transport cleanup fails.
func TestPPPCleanupHealthRegisteredProbe(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	ppp.ReportCleanupFailure(ppp.SessionIdentity{InterfaceName: "ppp7", NativeIndex: 9}, ppp.CleanupTransport, errors.New("close failed"))
	for _, row := range health.DefaultRegistry.Check().Components {
		if row.Name != pppCleanupHealthName {
			continue
		}
		if row.Status != health.StatusDegraded {
			t.Fatalf("failed cleanup health = %+v", row)
		}
		if !strings.Contains(row.Reason, "ppp7") {
			t.Fatalf("failed cleanup lacks identity: %+v", row)
		}
		return
	}
	t.Fatal("registered health row missing")
}

// Goal: report-bus eviction is not recovery. Method: evict the cleanup
// warning at the bus's test reset capacity, then ask the registered check.
func TestPPPCleanupDoctorKeepsEvictionUncertainty(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	ppp.ReportCleanupFailure(ppp.SessionIdentity{InterfaceName: "ppp7", NativeIndex: 9}, ppp.CleanupIPv6, errors.New("route removal failed"))
	// The report bus owns this bound; fill the capacity it actually exposes.
	capacity := report.Losses().WarningCap
	for i := range capacity + 1 {
		report.RaiseWarning("cleanup-health-fixture", "cleanup-health-fixture", fmt.Sprint(i), "test warning")
	}
	report.ClearSource("cleanup-health-fixture")
	findings := checkPPPCleanup(diagnostic.DoctorCheckContext{})
	if len(findings) != 1 {
		t.Fatalf("eviction appeared recovered: %+v", findings)
	}
	if findings[0].Code != diagnostic.CodeDoctorPPPCleanup {
		t.Fatalf("eviction code = %+v", findings)
	}
	if !strings.Contains(findings[0].Message, "report bus") {
		t.Fatalf("eviction cause = %+v", findings)
	}
}
