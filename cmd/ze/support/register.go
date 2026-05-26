// Design: docs/architecture/core-design.md — support command registration

package support

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

func init() {
	cmdregistry.RegisterRoot("support", cmdregistry.Meta{
		Description: "Generate tech-support archive for troubleshooting",
		Mode:        "offline",
		Section:     cmdregistry.SectionSystem,
		Subs:        "[--module M] [--exclude M] [--since T] [--reason R] [--sensitive] [--json] [--list-modules]",
	})
	cmdregistry.MustRegisterLocal("support", Run)
}
