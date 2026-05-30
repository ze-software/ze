// Design: docs/architecture/core-design.md — support command registration

package support

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

func init() {
	cmdregistry.RegisterRoot("support", cmdregistry.Meta{
		Description: "Collect logs, config, and diagnostics into a support archive",
		Mode:        "offline",
		Section:     cmdregistry.SectionSystem,
		Subs:        "[--module M] [--exclude M] [--since T] [--reason R] [--sensitive] [--json] [--list-modules]",
	})
	cmdregistry.MustRegisterLocalMeta("support", Run, cmdregistry.Meta{
		Description: "Bundle logs, config, state, and diagnostics into one archive file. Send the result to support when reporting an issue.",
	})
}
