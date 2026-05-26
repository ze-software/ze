// Design: docs/features/ai-first.md — doctor command registration

package doctor

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
	"codeberg.org/thomas-mangin/ze/internal/core/diagnostic"
)

func init() {
	cmdregistry.RegisterRoot("doctor", cmdregistry.Meta{
		Description: "Check system readiness for running Ze",
		Mode:        "offline",
		Section:     cmdregistry.SectionSystem,
		Subs:        "[--json] [<config-file>]",
	})
	cmdregistry.MustRegisterLocal("doctor", Run)
	diagnostic.RegisterDoctorProvider(runChecks)
}
