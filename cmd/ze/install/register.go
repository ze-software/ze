// Design: docs/architecture/cli/plugin-modes.md — ze install root handler registration

package install

import "github.com/ze-software/ze/internal/component/command/registry"

func init() {
	registry.MustRegisterRootHandler("install", func(_ *registry.RuntimeContext, args []string) int {
		return Dispatch(args)
	}, registry.Meta{
		Description: "Install ze binary, systemd service, or provision remote devices",
		Mode:        "setup",
		Section:     registry.SectionSystem,
		SubsFunc:    Subcommands,
	})
}
