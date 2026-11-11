// Design: docs/architecture/doctor-and-health-checks.md -- plugin external blocks and the managed fleet
// Related: check_external.go -- the external name and in-process checks
// Related: check_managed_runtime.go -- the managed listener and client checks

package doctor

import "github.com/ze-software/ze/internal/core/diagnostic"

// doctorDependencyConfigTree is the dependency a check declares when it reads
// the config tree.
const doctorDependencyConfigTree = "config-tree"

// doctorDependencyManagedServer is the dependency a check declares when it
// reads the daemon's managed config server record.
const doctorDependencyManagedServer = "managed-server"

func init() {
	// The name check reads the parsed tree, so it still reports a config the
	// commit now refuses and ReadConfig would hide.
	registerOrExit(diagnostic.DoctorCheck{
		Name:         externalNameCheckName,
		Phase:        diagnostic.DoctorPhaseParsedConfig,
		Order:        703,
		Component:    componentPlugin,
		Dependencies: []string{doctorDependencyConfigTree},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{diagnostic.CodeDoctorPluginConnectBackRefused},
		Check:        checkExternalPluginNames,
	})
	// The in-process check reads the plugins the loaded config starts and
	// each one's registration, both present in the operator's own process.
	registerOrExit(diagnostic.DoctorCheck{
		Name:         externalInProcessCheckName,
		Phase:        diagnostic.DoctorPhasePostConfig,
		Order:        704,
		Component:    componentPlugin,
		Dependencies: []string{doctorDependencyConfigTree},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{diagnostic.CodeDoctorPluginExternalUnsupported},
		Check:        checkExternalInProcess,
	})
	// The managed checks read the daemon's managed server, so they run at the
	// runtime phase, which only the daemon runs.
	registerOrExit(diagnostic.DoctorCheck{
		Name:         managedUnboundCheckName,
		Phase:        diagnostic.DoctorPhaseRuntime,
		Order:        711,
		Component:    componentPlugin,
		Dependencies: []string{doctorDependencyManagedServer},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{diagnostic.CodeDoctorManagedListenerUnbound},
		Check:        checkManagedListenerUnbound,
	})
	registerOrExit(diagnostic.DoctorCheck{
		Name:         managedRejectedCheckName,
		Phase:        diagnostic.DoctorPhaseRuntime,
		Order:        712,
		Component:    componentPlugin,
		Dependencies: []string{doctorDependencyManagedServer},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{diagnostic.CodeDoctorManagedClientRejected},
		Check:        checkManagedClientRejected,
	})
}

// registerOrExit registers one check and ends the process on a refusal, for
// the reason exitOnRegistrationFailure states.
func registerOrExit(check diagnostic.DoctorCheck) { //nolint:gocritic // hugeParam: registered once at init
	if err := diagnostic.RegisterDoctorCheck(check); err != nil {
		exitOnRegistrationFailure(err)
	}
}
