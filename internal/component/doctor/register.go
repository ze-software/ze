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
		Description: "Verify kernel features, file descriptor limits, sockets, and required dependencies. Run this before first start or after platform changes.",
	})
	diagnostic.RegisterDoctorProvider(runChecks)
}
