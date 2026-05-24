// Design: plan/spec-install-0-umbrella.md — ze uninstall command registration

package uninstall

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

func init() {
	cmdregistry.RegisterRoot("uninstall", cmdregistry.Meta{
		Description: "Remove ze binary, systemd unit, and optionally config",
		Mode:        "setup",
		Section:     cmdregistry.SectionSystem,
		Subs:        "[--prefix] [--purge] [--dry-run] [--yes]",
	})
}
