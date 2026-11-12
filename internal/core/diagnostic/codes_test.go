// Design: docs/architecture/doctor-and-health-checks.md -- the cross-cutting codes every area child names
// Related: codes.go -- builtinCodes, the metadata under test

package diagnostic

import "testing"

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
