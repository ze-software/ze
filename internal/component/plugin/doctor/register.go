// Design: docs/features/ai-first.md -- plugin binary readiness check
// Related: check_managed_listener.go -- the managed listener collision check registered here

package doctor

import "github.com/ze-software/ze/internal/core/diagnostic"

// componentPlugin names the component every check here belongs to. The doctor
// runner reports it beside each diagnostic, so one spelling serves all three.
const componentPlugin = "plugin"

// codePluginMissing is the diagnostic code this check publishes and raises.
const codePluginMissing = "doctor-plugin-missing"

// codePluginShellMissing is the code the shell check publishes and raises. The
// shell is the second runtime dependency of an external plugin: the binary the
// run string names is the first, and nothing starts without both.
const codePluginShellMissing = "doctor-plugin-shell-missing"

func init() {
	if err := diagnostic.RegisterDoctorCheck(diagnostic.DoctorCheck{
		Name:         "plugin-binaries",
		Phase:        diagnostic.DoctorPhasePostConfig,
		Order:        700,
		Component:    componentPlugin,
		Dependencies: []string{"external-binary"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{codePluginMissing},
		Check:        checkPlugins,
	}); err != nil {
		panic("BUG: plugin: doctor check registration (plugin-binaries): " + err.Error())
	}

	// The shell check answers the other half of "can this external plugin
	// start". The binary check above reads the command the run string names;
	// this one reads the shell that command is given to, which an appliance
	// image does not carry.
	if err := diagnostic.RegisterDoctorCheck(diagnostic.DoctorCheck{
		Name:         "plugin-shell",
		Phase:        diagnostic.DoctorPhasePostConfig,
		Order:        702,
		Component:    componentPlugin,
		Dependencies: []string{"external-binary"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{codePluginShellMissing},
		Check:        checkPluginShell,
	}); err != nil {
		panic("BUG: plugin: doctor check registration (plugin-shell): " + err.Error())
	}

	// The managed listener check reads the config tree. It runs after the tree
	// is parsed and it declares that dependency. It sits beside the binary
	// check because the plugin component owns both producers it compares. Those
	// are the acceptor's address, and the hub server block list the managed
	// listener picks its own addresses from.
	if err := diagnostic.RegisterDoctorCheck(diagnostic.DoctorCheck{
		Name:         "hub-managed-listener",
		Phase:        diagnostic.DoctorPhasePostConfig,
		Order:        701,
		Component:    componentPlugin,
		Dependencies: []string{doctorDependencyConfigTree},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{codeHubManagedCollision},
		Check:        checkManagedListener,
	}); err != nil {
		panic("BUG: plugin: doctor check registration (hub-managed-listener): " + err.Error())
	}

	// The report bus check reads the daemon's own bus, so it runs at the
	// runtime phase, which only the daemon runs (a registry.DoctorCheckDef
	// cannot be runtime, validateDoctorCheckDef).
	if err := diagnostic.RegisterDoctorCheck(diagnostic.DoctorCheck{
		Name:         reportBusCheckName,
		Phase:        diagnostic.DoctorPhaseRuntime,
		Order:        710,
		Component:    componentPlugin,
		Dependencies: []string{"report-bus"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{diagnostic.CodeDoctorReportBusFull},
		Check:        checkReportBusFull,
	}); err != nil {
		panic("BUG: plugin: doctor check registration (" + reportBusCheckName + "): " + err.Error())
	}
}
