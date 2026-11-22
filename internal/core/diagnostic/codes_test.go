// Design: docs/architecture/doctor-and-health-checks.md -- the cross-cutting codes every area child names
// Related: codes.go -- builtinCodes, the metadata under test

package diagnostic

import (
	"strings"
	"testing"
)

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
// entry point does, then looks up every cross-cutting code owned by
// codes_plugin_infra.go. Single-owner plugin-infra codes live in their owning
// packages and are verified by internal/component/plugin/doctor/doctor_codes_test.go.
//
// VALIDATES: AC-1. `ze show diagnostic <code>` answers a title, a description
// and at least one example for each cross-cutting code.
// PREVENTS: a doctor row or a log line carrying a code the operator cannot
// look up, and an area child that starts before its shared codes exist.
func TestCrossCuttingCodesDescribed(t *testing.T) {
	RegisterBuiltinCodes()

	crossCutting := []string{
		CodeDoctorInternalDefect,
		CodeDoctorPluginStopped,
		CodeDoctorPluginCrashed,
		CodeDoctorPluginStartupPhaseFailed,
		CodeDoctorPluginExternalUnsupported,
		CodeDoctorAuditRecordFailed,
		CodeDoctorPluginRequestRefused,
		CodeDoctorReportBusFull,
		CodeDoctorPluginPanic,
		CodeDoctorBuildRegistrationUnheld,
		CodeDoctorEnvInvalidValue,
	}

	// The spellings are the spec's, and a renamed constant must not pass by
	// carrying its old meaning under a new spelling.
	spelled := map[string]string{
		CodeDoctorInternalDefect:            "doctor-internal-defect",
		CodeDoctorPluginStopped:             "doctor-plugin-stopped",
		CodeDoctorPluginCrashed:             "doctor-plugin-crashed",
		CodeDoctorPluginStartupPhaseFailed:  "doctor-plugin-startup-phase-failed",
		CodeDoctorPluginExternalUnsupported: "doctor-plugin-external-unsupported",
		CodeDoctorAuditRecordFailed:         "doctor-audit-record-failed",
		CodeDoctorPluginRequestRefused:      "doctor-plugin-request-refused",
		CodeDoctorReportBusFull:             "doctor-report-bus-full",
		CodeDoctorPluginPanic:               "doctor-plugin-panic",
		CodeDoctorBuildRegistrationUnheld:   "doctor-build-registration-unheld",
		CodeDoctorEnvInvalidValue:           "doctor-env-invalid-value",
	}
	for constant, want := range spelled {
		if constant != want {
			t.Errorf("cross-cutting code spelled %q, the spec names %q", constant, want)
		}
	}

	// Cross-references that cross into plugin/* (which diagnostic cannot import
	// without a cycle) are verified by TestPluginInfraDoctorCodesDescribed in
	// internal/component/plugin/doctor/doctor_codes_test.go; every other
	// RelatedCodes target resolves within diagnostic itself.
	externalRelated := map[string]bool{
		"doctor-plugin-setup-failed":   true,
		"doctor-plugin-schema-refused": true,
	}

	for _, code := range crossCutting {
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
			if externalRelated[related] {
				continue
			}
			if Lookup(related) == nil {
				t.Errorf("%s: related code %s has no entry", code, related)
			}
		}
	}
}
