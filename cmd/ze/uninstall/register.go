// Design: docs/architecture/cli/plugin-modes.md — ze uninstall root handler registration

package uninstall

import "codeberg.org/thomas-mangin/ze/internal/component/command/registry"

func init() {
	registry.MustRegisterRootHandler("uninstall", func(_ *registry.RuntimeContext, args []string) int {
		return Dispatch(args)
	}, registry.Meta{
		Description: "Remove ze binary or systemd service",
		Mode:        "setup",
		Section:     registry.SectionSystem,
		SubsFunc:    Subcommands,
	})
}
