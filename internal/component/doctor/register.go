// Design: docs/features/ai-first.md — doctor command registration

package doctor

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func init() {
	registry.RegisterRoot("doctor", registry.Meta{
		ShortHelp: "Check if this box is ready to run Ze",
		Mode:      "offline",
		Section:   registry.SectionSystem,
		Subs:      "config <file>, kernel-capabilities, --json",
	})
	// One producer (Answer) serves `ze doctor` here and the daemon's show
	// doctor (internal/component/doctor/cmd), the RPC that adds the plugin
	// checks. doctor is an offline root, not a verb, so no YANG node sits at
	// it: DaemonPath names show doctor as its daemon form, whose leaves judge
	// the arguments here and which every daemon surface offers in its place.
	command.MustRegisterLocalData("doctor", answerLocal, registry.Meta{
		ShortHelp: "Check that this system is ready to run Ze.",
		Description: "The checks cover kernel features, file descriptor limits, listening sockets and " +
			"the dependencies Ze needs. Run it before the first start, and again after a change " +
			"to the platform.",
		DaemonPath: "show doctor",
	}, renderDoctor)
	registerDoctorOwnedChecks()
	// The report bus is core and owns no component, so the check that reads
	// its recent events back into show doctor lives here (checks_recent_events.go).
	if err := diagnostic.RegisterDoctorCheck(recentEventsCheck); err != nil {
		panic("BUG: recent-events doctor check registration refused: " + err.Error())
	}
	// The env package is core too, so the check that reports the values it
	// could not use lives here as well (checks_env_values.go).
	if err := diagnostic.RegisterDoctorCheck(envValuesCheck); err != nil {
		panic("BUG: env-values doctor check registration refused: " + err.Error())
	}
	// The privilege package is core too, so the check that reports the
	// capabilities the daemon lacks lives here as well (checks_daemon_capabilities.go).
	if err := diagnostic.RegisterDoctorCheck(daemonCapabilitiesCheck); err != nil {
		panic("BUG: daemon-capabilities doctor check registration refused: " + err.Error())
	}
	diagnostic.RegisterDoctorProvider(runChecks)
}
