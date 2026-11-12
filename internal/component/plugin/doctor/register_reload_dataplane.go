// Design: docs/architecture/doctor-and-health-checks.md -- plugin lifecycle checks
// Related: check_reload_records.go -- the two reload checks
// Related: check_dataplane_writer.go -- the data-plane writer check
//
// The reload and data-plane checks register here, apart from register.go,
// so each problem class sits with its own registration.

package doctor

import "github.com/ze-software/ze/internal/core/diagnostic"

// The check names and their place in the order.
const (
	removalUnrecoveredCheckName      = "plugin-removal-recovery"
	capabilitiesNotRestoredCheckName = "plugin-capability-restore"
	dataPlaneWriterCheckName         = "plugin-dataplane-writer"
	reloadRecordsOrderFirst          = 740
	dataPlaneWriterOrder             = 719
)

func init() {
	checks := []diagnostic.DoctorCheck{
		{
			// Reads the running server's reload records, so only the daemon runs it.
			Name:         removalUnrecoveredCheckName,
			Phase:        diagnostic.DoctorPhaseRuntime,
			Order:        reloadRecordsOrderFirst,
			Component:    componentPlugin,
			Dependencies: []string{pluginStateDependency},
			Platforms:    []string{diagnostic.DoctorPlatformAny},
			Codes:        []string{diagnostic.CodeDoctorPluginRemovalUnrecovered},
			Check:        checkRemovalUnrecovered,
		},
		{
			Name:         capabilitiesNotRestoredCheckName,
			Phase:        diagnostic.DoctorPhaseRuntime,
			Order:        reloadRecordsOrderFirst + 1,
			Component:    componentPlugin,
			Dependencies: []string{pluginStateDependency},
			Platforms:    []string{diagnostic.DoctorPlatformAny},
			Codes:        []string{diagnostic.CodeDoctorPluginCapabilitiesNotRestored},
			Check:        checkCapabilitiesNotRestored,
		},
		{
			// Resolves the config's auto-load set, so it needs the parsed tree,
			// and answers on the local route before ze starts.
			Name:         dataPlaneWriterCheckName,
			Phase:        diagnostic.DoctorPhasePostConfig,
			Order:        dataPlaneWriterOrder,
			Component:    componentPlugin,
			Dependencies: []string{doctorDependencyConfigTree},
			Platforms:    []string{diagnostic.DoctorPlatformAny},
			Codes:        []string{diagnostic.CodeDoctorPluginNoDataPlaneWriter},
			Check:        checkDataPlaneWriter,
		},
	}
	for i := range checks {
		if err := diagnostic.RegisterDoctorCheck(checks[i]); err != nil {
			exitOnRegistrationFailure(err)
		}
	}
}
