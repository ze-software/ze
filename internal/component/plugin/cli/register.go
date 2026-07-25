// Design: docs/architecture/api/commands.md — plugin command ownership
//
// Register the `plugin` root command with the importable command registry.
// This is the owner package: the offline plugin CLI lives with
// internal/component/plugin, not under cmd/ze.
package cli

import "github.com/ze-software/ze/internal/component/command/registry"

func init() {
	registry.MustRegisterRootHandler("plugin", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		Description: "Plugin system",
		Mode:        "offline",
		Section:     registry.SectionSystem,
		Subs:        "<plugin-name> for plugin CLI, test for debugging",
	})
}
