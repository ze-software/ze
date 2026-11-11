// Design: docs/architecture/doctor-and-health-checks.md -- plugin lifecycle checks
// Related: checks_runtime.go -- the runtime check over the plugin snapshot
// Related: check_setup.go -- the setup-outcome check
// Related: check_lifecycle_local.go -- the last-stop and auto-load checks
//
// The plugin lifecycle checks register here, apart from register.go, so the
// checks of each problem class sit with their own registration.

package doctor

import "github.com/ze-software/ze/internal/core/diagnostic"

// The lifecycle check names.
const (
	pluginStoppedCheckName    = "plugin-stopped"
	pluginSetupCheckName      = "plugin-setup"
	pluginFatalStopCheckName  = "plugin-fatal-stop"
	pluginAutoloadCheckName   = "plugin-autoload"
	pluginLifecycleOrderFirst = 720
)

func init() {
	checks := []diagnostic.DoctorCheck{
		{
			// Reads the running server's plugin snapshot, so it runs at the
			// runtime phase, which only the daemon runs.
			Name:         pluginStoppedCheckName,
			Phase:        diagnostic.DoctorPhaseRuntime,
			Order:        pluginLifecycleOrderFirst,
			Component:    componentPlugin,
			Dependencies: []string{"plugin-state"},
			Platforms:    []string{diagnostic.DoctorPlatformAny},
			Codes:        []string{diagnostic.CodeDoctorPluginStopped},
			Check:        checkPluginStopped,
		},
		{
			// The setup records are written in init(), before main(), in every
			// process of the binary, so the check answers on both routes.
			Name:         pluginSetupCheckName,
			Phase:        diagnostic.DoctorPhasePreConfig,
			Order:        pluginLifecycleOrderFirst + 1,
			Component:    componentPlugin,
			Dependencies: []string{"plugin-registry"},
			Platforms:    []string{diagnostic.DoctorPlatformAny},
			Codes:        []string{diagnostic.CodeDoctorPluginSetupFailed},
			Check:        checkPluginSetup,
		},
		{
			// Reads the last-stop record from the store the check is handed.
			Name:         pluginFatalStopCheckName,
			Phase:        diagnostic.DoctorPhasePreConfig,
			Order:        pluginLifecycleOrderFirst + 2,
			Component:    componentPlugin,
			Dependencies: []string{"store"},
			Platforms:    []string{diagnostic.DoctorPlatformAny},
			Codes:        []string{diagnostic.CodeDoctorPluginFatalStop, diagnostic.CodeDoctorPluginStopCleanupLate},
			Check:        checkPluginFatalStop,
		},
		{
			// Resolves the config's auto-load set, so it needs the parsed tree.
			Name:         pluginAutoloadCheckName,
			Phase:        diagnostic.DoctorPhasePostConfig,
			Order:        pluginLifecycleOrderFirst + 3,
			Component:    componentPlugin,
			Dependencies: []string{doctorDependencyConfigTree},
			Platforms:    []string{diagnostic.DoctorPlatformAny},
			Codes:        []string{diagnostic.CodeDoctorPluginAutoloadUnresolved},
			Check:        checkPluginAutoload,
		},
	}
	for i := range checks {
		if err := diagnostic.RegisterDoctorCheck(checks[i]); err != nil {
			exitOnRegistrationFailure(err)
		}
	}
}
