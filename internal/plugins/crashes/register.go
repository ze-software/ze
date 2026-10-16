// Design: docs/architecture/diagnostics/crash-capture.md -- offline crash file CLI

package crashes

import (
	"net"
	"os"

	"github.com/ze-software/ze/internal/component/command"
	pluginregistry "github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// componentName is the name this plugin answers to in the doctor registry, in
// the metrics injection seam and in the listing payload. One spelling, so a
// reader greps one word to find every surface.
const componentName = "crashes"

// doctorConfigLoaded is the doctor dependency token that says a config tree is
// available. Two of the three checks are silent without one.
const doctorConfigLoaded = "config-loaded"

func init() {
	// `show crashes [latest | name <file>]` is a daemon command (crashes-cmd).
	// When no daemon is reachable -- which is exactly when you inspect a crash,
	// since the daemon has died -- serve the same crash files in-process.
	// Registered as an offline fallback (never a plain local) so it does not
	// shadow the daemon command while the daemon is up.
	command.MustRegisterOfflineFallback("show crashes", offlineShowCrashes)

	// Crash capture owns three runtime dependencies, so it owns three checks,
	// three diagnostic codes, and the test that proves each one fires
	// (ai/rules/repo-maintenance.md). They run post-config because two of the
	// three are silent unless the operator asked for kernel capture.
	registerDoctorCheck(diagnostic.DoctorCheck{
		Name:         "crash-capture-armed",
		Phase:        diagnostic.DoctorPhasePostConfig,
		Order:        750,
		Component:    componentName,
		Dependencies: []string{doctorConfigLoaded},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{"doctor-crash-capture-unarmed"},
		Check:        checkCrashCaptureArmed,
	})
	registerDoctorCheck(diagnostic.DoctorCheck{
		Name:         "crash-capture-pstore",
		Phase:        diagnostic.DoctorPhasePostConfig,
		Order:        751,
		Component:    componentName,
		Dependencies: []string{doctorConfigLoaded},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{"doctor-crash-capture-pstore"},
		Check:        checkCrashCapturePstore,
	})
	registerDoctorCheck(diagnostic.DoctorCheck{
		Name:         "crash-directory-writable",
		Phase:        diagnostic.DoctorPhasePostConfig,
		Order:        752,
		Component:    componentName,
		Dependencies: []string{doctorConfigLoaded},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{"doctor-crash-directory-unwritable"},
		Check:        checkCrashDirectory,
	})

	// The gauges exist only once a metrics registry does, which is after this
	// init runs in every build. InjectPluginMetrics defers the hook until then
	// and drops it in a build with telemetry compiled out.
	pluginregistry.InjectPluginMetrics(componentName, bindMetrics)

	// The kernel writes its crash record during the fault, and the next
	// healthy boot recovers it: the daemon start path runs this boot effect
	// once, never on a reload. The registration declares no family, config
	// root, event or data-plane need, so nothing ever auto-loads its no-op
	// engine; it exists to carry the boot effect.
	if err := pluginregistry.Register(pluginregistry.Registration{
		Name:        componentName,
		Description: "Crash capture: kernel crash record harvest at daemon start, crash reports",
		RunEngine:   func(_ net.Conn) int { return 0 },
		CLIHandler:  func(_ []string) int { return 0 },
		BootEffect:  harvestAtDaemonStart,
	}); err != nil {
		// A lost registration loses the boot harvest, and a kernel crash
		// record nobody recovers is gone at the next fault. Fail loudly, as
		// every other plugin registration does.
		var tb textbuf.Buffer
		_ = tb.Str("crashes: registration failed: ").Err(err).Byte('\n').StdErr()
		os.Exit(1)
	}
}

// registerDoctorCheck reports a rejected registration on stderr and leaves the
// process running. A malformed check is a Ze defect, and this package is an
// offline fallback linked into every `ze` invocation, so failing the process
// here would make `ze --version` unusable over a diagnostic that reports
// nothing about the operator's machine.
func registerDoctorCheck(check diagnostic.DoctorCheck) {
	if err := diagnostic.RegisterDoctorCheck(check); err != nil {
		var tb textbuf.Buffer
		_ = tb.Str("crashes: doctor check registration: ").Err(err).Byte('\n').StdErr()
	}
}

// offlineShowCrashes adapts the daemon grammar (`show crashes [latest | name
// <file>]`) to RunShow, which takes the bare selector: `name <file>` selects one
// report, `latest` the newest, and no argument lists all.
func offlineShowCrashes(validated command.ValidatedArgs) int {
	args := validated.Tokens()
	if len(args) > 0 && args[0] == "name" {
		if len(args) < 2 {
			os.Stderr.WriteString("error: 'show crashes name' requires a filename\n") //nolint:errcheck // CLI error
			return 1
		}
		return RunShow([]string{args[1]})
	}
	return RunShow(args)
}
