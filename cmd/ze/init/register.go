// Register the init root command with the command registry.

package init

import (
	"codeberg.org/thomas-mangin/ze/internal/component/command/registry"
)

func init() {
	registry.MustRegisterRootHandler("init", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		Description: "Bootstrap database with SSH credentials",
		Mode:        "setup",
		Section:     registry.SectionSystem,
		Subs:        "--managed for fleet mode, --force to replace",
	})
}
