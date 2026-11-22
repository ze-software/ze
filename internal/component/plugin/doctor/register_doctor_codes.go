// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/plugin/doctor.

package doctor

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorPluginSetupFailed        = "doctor-plugin-setup-failed"
	codeDoctorPluginAutoloadUnresolved = "doctor-plugin-autoload-unresolved"
	codeDoctorPluginNoDataPlaneWriter  = "doctor-plugin-no-dataplane-writer"
	exampleDoctorJSON                  = "ze doctor --json"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codePluginMissing,
		Title:       "Plugin binary not found",
		Description: "An external plugin binary referenced in the config is not on PATH.",
		Examples:    []string{exampleDoctorJSON},
	},
	{
		Code:  codePluginShellMissing,
		Title: "Shell for external plugins not found",
		Description: "The config names an external plugin and this host carries no shell at /bin/sh. " +
			"Ze starts an external plugin by giving the `run` string to `/bin/sh -c`, so every external plugin in the config fails to start and the daemon runs without them. " +
			"A gokrazy appliance image carries no shell utilities, which is where a host reaches this state. " +
			"Move the plugin to an `internal` block where a built-in plugin provides it, or install a shell on this host.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-plugin-shell-missing"},
		RelatedCodes: []string{"doctor-plugin-missing"},
	},
	{
		Code:  codeDoctorPluginSetupFailed,
		Title: "Plugin registration or setup failed",
		Description: "A plugin's registration or setup, which runs when the binary starts and before any config is read, failed and recorded why. " +
			"A hard failure stops the daemon from starting; a soft one leaves the plugin without the part that failed. " +
			"The row names the plugin and the recorded reason, and the reason names its fix.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-plugin-setup-failed"},
		RelatedCodes: []string{diagnostic.CodeDoctorPluginStartupPhaseFailed},
	},
	{
		Code:  codeDoctorPluginAutoloadUnresolved,
		Title: "Plugin dependencies missing from this build",
		Description: "The configuration selects a plugin that depends on another plugin this binary does not carry, because its build tag compiled it out. " +
			"The daemon refuses to start with this configuration. The row names the plugin, the dependency and the build tag: " +
			"use a build that carries the dependency, or remove the configuration that selects the plugin.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-plugin-autoload-unresolved"},
	},
	{
		Code:  codeDoctorPluginNoDataPlaneWriter,
		Title: "Data plane has no writer",
		Description: "A configured plugin produces routes for a data plane, and no plugin in the configuration programs that data plane, or its writer is compiled out of this build. " +
			"The daemon refuses to start or reload with this configuration. The row names the producer and the data plane: add a fib block, or configure the writer for that data plane.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-plugin-no-dataplane-writer"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: doctor: doctor code registration refused: " + err.Error())
		}
	}
}
