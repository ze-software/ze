// Design: docs/features/ai-first.md — doctor command registration

package doctor

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
	"codeberg.org/thomas-mangin/ze/internal/core/diagnostic"
)

func init() {
	cmdregistry.RegisterRoot("doctor", cmdregistry.Meta{
		Description: "Check if this box is ready to run Ze",
		Mode:        "offline",
		Section:     cmdregistry.SectionSystem,
		Subs:        "[--json] [<config-file>]",
	})
	cmdregistry.MustRegisterLocalMeta("doctor", Run, cmdregistry.Meta{
		Description: "Verify kernel features, file descriptor limits, sockets, and required dependencies. Run this before first start or after platform changes.",
	})
	diagnostic.RegisterDoctorProvider(runChecks)
}
