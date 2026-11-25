// Design: docs/architecture/doctor-and-health-checks.md -- plugin infrastructure doctor diagnostic codes
// Related: register_doctor_codes.go -- codes registered by plugin/doctor, plugin/server, plugin/process and plugin/ipc

package doctor

import (
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestPluginInfraDoctorCodesDescribed verifies that every plugin-infrastructure
// doctor code registers metadata through its owning package's init(), and that
// every RelatedCodes cross-reference resolves once builtin codes are loaded.
//
// VALIDATES: `ze show diagnostic <code>` answers a title, description, at least
// one example, and valid related-code links for all plugin-infra codes.
// PREVENTS: a moved code losing its explanation or leaving a broken RelatedCodes link.
func TestPluginInfraDoctorCodesDescribed(t *testing.T) {
	diagnostic.RegisterBuiltinCodes()

	// The spellings are the spec's, and a renamed constant must not pass by
	// carrying its old meaning under a new spelling.
	spelled := map[string]string{
		server.CodeDoctorPluginConfigUnapplied:  "doctor-plugin-config-unapplied",
		codeDoctorPluginSetupFailed:             "doctor-plugin-setup-failed",
		process.CodeDoctorPluginStopCleanupLate: "doctor-plugin-stop-cleanup-late",
		server.CodeDoctorConfigReloadRefused:    "doctor-config-reload-refused",
	}
	for constant, want := range spelled {
		if constant != want {
			t.Errorf("plugin-infra code spelled %q, the spec names %q", constant, want)
		}
	}

	codes := []string{
		// Owned by plugin/doctor.
		codePluginMissing,
		codePluginShellMissing,
		codeDoctorPluginSetupFailed,
		codeDoctorPluginAutoloadUnresolved,
		codeDoctorPluginNoDataPlaneWriter,

		// Owned by plugin/ipc (registered in plugin/server to avoid an import cycle).
		ipc.ConnectBackRefusedCode,

		// Owned by plugin/process.
		process.CodeDoctorPluginStderrRelayStopped,
		process.CodeDoctorPluginRestartOverlap,
		process.CodeDoctorPluginStopUnfinished,
		process.CodeDoctorPluginDeliveryFailed,
		process.CodeDoctorPluginStopCleanupLate,
		process.CodeDoctorPluginShutdownOrder,

		// Owned by plugin/server.
		server.CodeDoctorPluginAnswerUndelivered,
		server.CodeDoctorPluginDrainIncomplete,
		server.CodeDoctorPluginSchemaChanged,
		server.CodeDoctorManagedClientNotifyDropped,
		server.CodeDoctorCommandEnsureRollbackFailed,
		server.CodeDoctorCommandDeprecatedUsed,
		server.CodeDoctorConfigReloadRefused,
		server.CodeDoctorHubManagedCollision,
		server.CodeDoctorPluginConfigUnapplied,
		server.CodeDoctorPluginFatalStop,
		server.CodeDoctorPluginCommandNotRegistered,
		server.CodeDoctorPluginSubscriptionDropped,
		server.CodeDoctorPluginDeliveryDisagreement,
		server.CodeDoctorPluginClaimUnbacked,
		server.CodeDoctorPluginPostStartupUndelivered,
		server.CodeDoctorPluginSchemaUndeclared,
		server.CodeDoctorPluginSchemaRefused,
		server.CodeDoctorPluginEnricherUnavailable,
		server.CodeDoctorManagedListenerUnbound,
		server.CodeDoctorManagedServerUnavailable,
		server.CodeDoctorManagedClientRejected,
		server.CodeDoctorPluginRemovalUnrecovered,
		server.CodeDoctorPluginCapabilitiesNotRestored,

		// Cross-cutting codes in diagnostic, verified here with every plugin/*
		// package linked so cross-package RelatedCodes targets also resolve.
		diagnostic.CodeDoctorInternalDefect,
		diagnostic.CodeDoctorPluginStopped,
		diagnostic.CodeDoctorPluginCrashed,
		diagnostic.CodeDoctorPluginStartupPhaseFailed,
		diagnostic.CodeDoctorPluginExternalUnsupported,
		diagnostic.CodeDoctorAuditRecordFailed,
		diagnostic.CodeDoctorPluginRequestRefused,
		diagnostic.CodeDoctorReportBusFull,
		diagnostic.CodeDoctorPluginPanic,
		diagnostic.CodeDoctorBuildRegistrationUnheld,
		diagnostic.CodeDoctorEnvInvalidValue,
	}

	for _, code := range codes {
		meta := diagnostic.Lookup(code)
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
			if diagnostic.Lookup(related) == nil {
				t.Errorf("%s: related code %s has no entry", code, related)
			}
		}
	}
}
