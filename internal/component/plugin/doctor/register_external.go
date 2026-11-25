// Design: docs/architecture/doctor-and-health-checks.md -- plugin external blocks and the managed fleet
// Related: check_external.go -- the external name and in-process checks
// Related: check_managed_runtime.go -- the managed listener and client checks

package doctor

import (
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// doctorDependencyConfigTree is the dependency a check declares when it reads
// the config tree.
const doctorDependencyConfigTree = "config-tree"

// doctorDependencyManagedServer is the dependency a check declares when it
// reads the daemon's managed config server record.
const doctorDependencyManagedServer = "managed-server"

func init() {
	// The name check reads the parsed tree, so it still reports a config the
	// commit now refuses and ReadConfig would hide.
	registerOrPanic(diagnostic.DoctorCheck{
		Name:         externalNameCheckName,
		Phase:        diagnostic.DoctorPhaseParsedConfig,
		Order:        703,
		Component:    componentPlugin,
		Dependencies: []string{doctorDependencyConfigTree},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{ipc.ConnectBackRefusedCode},
		Check:        checkExternalPluginNames,
	})
	// The in-process check reads the plugins the loaded config starts and
	// each one's registration, both present in the operator's own process.
	registerOrPanic(diagnostic.DoctorCheck{
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
	registerOrPanic(diagnostic.DoctorCheck{
		Name:         managedUnboundCheckName,
		Phase:        diagnostic.DoctorPhaseRuntime,
		Order:        711,
		Component:    componentPlugin,
		Dependencies: []string{doctorDependencyManagedServer},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{server.CodeDoctorManagedListenerUnbound, server.CodeDoctorManagedServerUnavailable},
		Check:        checkManagedListenerUnbound,
	})
	registerOrPanic(diagnostic.DoctorCheck{
		Name:         managedRejectedCheckName,
		Phase:        diagnostic.DoctorPhaseRuntime,
		Order:        712,
		Component:    componentPlugin,
		Dependencies: []string{doctorDependencyManagedServer},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{server.CodeDoctorManagedClientRejected},
		Check:        checkManagedClientRejected,
	})
	registerOrPanic(diagnostic.DoctorCheck{
		Name:         "managed-device-record-unwritable",
		Phase:        diagnostic.DoctorPhaseRuntime,
		Order:        713,
		Component:    componentPlugin,
		Dependencies: []string{doctorDependencyManagedServer},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{server.CodeDoctorManagedDeviceRecordUnwritable},
		Check:        checkManagedDeviceRecordUnwritable,
	})
	registerOrPanic(diagnostic.DoctorCheck{
		Name:         "managed-device-record-unreadable",
		Phase:        diagnostic.DoctorPhaseRuntime,
		Order:        714,
		Component:    componentPlugin,
		Dependencies: []string{doctorDependencyManagedServer},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{server.CodeDoctorManagedDeviceRecordUnreadable},
		Check:        checkManagedDeviceRecordUnreadable,
	})
}

// registerOrPanic registers one check and panics on a refusal. Registration
// runs in init(), so a refusal is a compile-time Ze defect.
func registerOrPanic(check diagnostic.DoctorCheck) { //nolint:gocritic // hugeParam: registered once at init
	if err := diagnostic.RegisterDoctorCheck(check); err != nil {
		panic("BUG: plugin: doctor check registration (" + check.Name + "): " + err.Error())
	}
}
