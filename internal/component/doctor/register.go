// Design: docs/features/ai-first.md — doctor command registration

// codegen:skip -- CLI command wired via cmd/ze/main.go, not a runtime plugin.

package doctor

import (
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func init() {
	registry.RegisterRoot("doctor", registry.Meta{
		Description: "Check if this box is ready to run Ze",
		Mode:        "offline",
		Section:     registry.SectionSystem,
		Subs:        "[--json] [<config-file>]",
	})
	registry.MustRegisterLocalMeta("doctor", Run, registry.Meta{
		Description: "Check that this system is ready to run Ze.",
		LongHelp: "The checks cover kernel features, file descriptor limits, listening sockets and " +
			"the dependencies Ze needs. Run it before the first start, and again after a change " +
			"to the platform.",
	})
	diagnostic.RegisterDoctorProvider(runChecks)
}
