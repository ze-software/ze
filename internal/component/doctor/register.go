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
	// One producer (Answer) serves `ze doctor` here and the daemon's doctor
	// and show doctor (internal/component/doctor/cmd), so the daemon route is
	// the RPC that adds the plugin checks; the bridge leaves this path to it.
	command.MustRegisterLocalData("doctor", answerLocal, registry.Meta{
		ShortHelp: "Check that this system is ready to run Ze.",
		Description: "The checks cover kernel features, file descriptor limits, listening sockets and " +
			"the dependencies Ze needs. Run it before the first start, and again after a change " +
			"to the platform.",
	}, renderDoctor)
	registerDoctorOwnedChecks()
	diagnostic.RegisterDoctorProvider(runChecks)
}
