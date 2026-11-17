// Design: docs/architecture/doctor-and-health-checks.md -- the cross-cutting codes every area child names
// Related: codes.go -- builtinCodes, the metadata under test

package diagnostic

import (
	"strings"
	"testing"
)

// TestOrphanCodeDescriptionNamesTheLiveRemoval proves that the description
// `ze show diagnostic doctor-ddos-flowspec-orphan-announcement` prints names
// the two steps in order (the withdrawal the report prints, then
// `ze data rm meta/ddos-flowspec/announcement`, which works while ze runs),
// why the order matters, and the refusal while a ddos-flowspec plugin runs
// with its restart.
//
// VALIDATES: AC-2, the code description half: it says the same about the
// removal as the reports.
// METHOD: the registered description of the code, read for each fact and for
// the order of the two steps.
// PREVENTS: a description that still says the removal needs ze stopped, while
// the reports say it works while ze runs.
func TestOrphanCodeDescriptionNamesTheLiveRemoval(t *testing.T) {
	RegisterBuiltinCodes()

	meta := Lookup(CodeDoctorDDoSFlowspecOrphanAnnouncement)
	if meta == nil {
		t.Fatalf("%s: ze show diagnostic has no entry", CodeDoctorDDoSFlowspecOrphanAnnouncement)
	}
	description := meta.Description
	withdrawAt := strings.Index(description, "send the withdrawal the report prints")
	removalAt := strings.Index(description, "then remove the record with ze data rm meta/ddos-flowspec/announcement, which works while ze runs")
	if withdrawAt < 0 {
		t.Errorf("description %q does not name the withdrawal the report prints", description)
	}
	if removalAt < 0 {
		t.Errorf("description %q does not name the removal that works while ze runs", description)
	}
	if removalAt < withdrawAt {
		t.Errorf("description %q names the removal before the withdrawal", description)
	}
	if !strings.Contains(description, "a record removed before its rule is withdrawn leaves the rule at the peers with no trace in the store and no report to find it") {
		t.Errorf("description %q does not say why the withdrawal comes first", description)
	}
	if !strings.Contains(description, "ze refuses the removal while a ddos-flowspec plugin runs") {
		t.Errorf("description %q does not say that a running ddos-flowspec plugin refuses the removal", description)
	}
	if !strings.Contains(description, "ze request plugin restart ddos-flowspec") {
		t.Errorf("description %q does not name the restart for a running plugin", description)
	}
	if strings.Contains(description, "while ze is stopped") {
		t.Errorf("description %q still says the removal needs ze stopped", description)
	}
}

// TestStoreIntegrityDescriptionNamesTheLiveCommands verifies AC-15 of the data
// store maintenance spec: the description `ze show diagnostic
// doctor-store-integrity` prints names the local check and repair as commands
// that run while ze runs, names their daemon forms for a remote operator, and
// says installing the repaired store needs ze stopped. It no longer tells the
// operator to stop the daemon before checking or repairing.
func TestStoreIntegrityDescriptionNamesTheLiveCommands(t *testing.T) {
	RegisterBuiltinCodes()

	meta := Lookup(CodeDoctorStoreIntegrity)
	if meta == nil {
		t.Fatalf("%s: ze show diagnostic has no entry", CodeDoctorStoreIntegrity)
	}
	description := meta.Description
	for _, want := range []string{
		"ze data check and ze data repair --output <path> run while ze runs",
		"show data check and request data repair path <file>",
		"Installing the repaired store in place of the damaged one needs ze stopped",
	} {
		if !strings.Contains(description, want) {
			t.Errorf("description %q does not say %q", description, want)
		}
	}
	if strings.Contains(description, "Stop the daemon and use ze data check") {
		t.Errorf("description %q still says the check and repair need the daemon stopped", description)
	}
}

// TestCrossCuttingCodesDescribed registers the builtin codes as the binary
// entry point does, then looks up every code
// spec-doctor-coverage-12-plugin-infra names: the ten cross-cutting codes
// every area child cites first, then the plugin infrastructure and core
// classes.
//
// VALIDATES: AC-1. `ze show diagnostic <code>` answers a title, a description
// and at least one example for each code.
// PREVENTS: a doctor row or a log line carrying a code the operator cannot
// look up, and an area child that starts before its shared codes exist.
func TestCrossCuttingCodesDescribed(t *testing.T) {
	RegisterBuiltinCodes()

	crossCutting := []string{
		CodeDoctorInternalDefect,
		CodeDoctorPluginStopped,
		CodeDoctorPluginCrashed,
		CodeDoctorPluginConfigUnapplied,
		CodeDoctorPluginSetupFailed,
		CodeDoctorPluginStartupPhaseFailed,
		CodeDoctorPluginExternalUnsupported,
		CodeDoctorAuditRecordFailed,
		CodeDoctorPluginRequestRefused,
		CodeDoctorPluginStopCleanupLate,
	}
	pluginInfra := []string{
		CodeDoctorPluginAnswerUndelivered,
		CodeDoctorPluginDrainIncomplete,
		CodeDoctorReportBusFull,
		CodeDoctorPluginFatalStop,
		CodeDoctorPluginAutoloadUnresolved,
		CodeDoctorPluginCommandNotRegistered,
		CodeDoctorPluginSubscriptionDropped,
		CodeDoctorPluginDeliveryDisagreement,
		CodeDoctorPluginClaimUnbacked,
		CodeDoctorPluginPostStartupUndelivered,
		CodeDoctorPluginPanic,
		CodeDoctorPluginDeliveryFailed,
		CodeDoctorPluginStderrRelayStopped,
		CodeDoctorPluginRestartOverlap,
		CodeDoctorPluginStopUnfinished,
		CodeDoctorPluginSchemaChanged,
		CodeDoctorPluginSchemaUndeclared,
		CodeDoctorPluginSchemaRefused,
		CodeDoctorPluginEnricherUnavailable,
		CodeDoctorPluginConnectBackRefused,
		CodeDoctorManagedListenerUnbound,
		CodeDoctorManagedClientRejected,
		CodeDoctorManagedClientNotifyDropped,
		CodeDoctorPluginRemovalUnrecovered,
		CodeDoctorPluginCapabilitiesNotRestored,
		CodeDoctorCommandEnsureRollbackFailed,
		CodeDoctorCommandDeprecatedUsed,
		CodeDoctorPluginNoDataPlaneWriter,
		CodeDoctorEnvInvalidValue,
		CodeDoctorConfigReloadRefused,
	}

	// The spellings are the spec's, and a renamed constant must not pass by
	// carrying its old meaning under a new spelling.
	spelled := map[string]string{
		CodeDoctorInternalDefect:            "doctor-internal-defect",
		CodeDoctorPluginStopped:             "doctor-plugin-stopped",
		CodeDoctorPluginCrashed:             "doctor-plugin-crashed",
		CodeDoctorPluginConfigUnapplied:     "doctor-plugin-config-unapplied",
		CodeDoctorPluginSetupFailed:         "doctor-plugin-setup-failed",
		CodeDoctorPluginStartupPhaseFailed:  "doctor-plugin-startup-phase-failed",
		CodeDoctorPluginExternalUnsupported: "doctor-plugin-external-unsupported",
		CodeDoctorAuditRecordFailed:         "doctor-audit-record-failed",
		CodeDoctorPluginRequestRefused:      "doctor-plugin-request-refused",
		CodeDoctorPluginStopCleanupLate:     "doctor-plugin-stop-cleanup-late",
		CodeDoctorConfigReloadRefused:       "doctor-config-reload-refused",
	}
	for constant, want := range spelled {
		if constant != want {
			t.Errorf("cross-cutting code spelled %q, the spec names %q", constant, want)
		}
	}

	for _, code := range append(crossCutting, pluginInfra...) {
		meta := Lookup(code)
		if meta == nil {
			t.Errorf("%s: ze show diagnostic has no entry", code)
			continue
		}
		if meta.Title == "" {
			t.Errorf("%s: no title", code)
		}
		if meta.Description == "" {
			t.Errorf("%s: no description", code)
		}
		if len(meta.Examples) == 0 {
			t.Errorf("%s: no example", code)
		}
		for _, related := range meta.RelatedCodes {
			if Lookup(related) == nil {
				t.Errorf("%s: related code %s has no entry", code, related)
			}
		}
	}
}
