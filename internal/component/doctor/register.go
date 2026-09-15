// Design: docs/features/ai-first.md — doctor command registration

package doctor

import (
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func init() {
	registry.RegisterRoot("doctor", registry.Meta{
		ShortHelp: "Check if this box is ready to run Ze",
		Mode:      "offline",
		Section:   registry.SectionSystem,
		Subs:      "[--json] [<config-file>]",
	})
	registry.MustRegisterLocalMeta("doctor", Run, registry.Meta{
		ShortHelp: "Check that this system is ready to run Ze.",
		Description: "The checks cover kernel features, file descriptor limits, listening sockets and " +
			"the dependencies Ze needs. Run it before the first start, and again after a change " +
			"to the platform.",
	})
	registerDoctorOwnedChecks()
	diagnostic.RegisterDoctorProvider(runChecks)
}
