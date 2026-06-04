// Register the exabgp root command with the command registry.

package exabgp

import (
	"codeberg.org/thomas-mangin/ze/internal/component/command/registry"
)

func init() {
	registry.MustRegisterRootHandler("exabgp", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		Description: "ExaBGP bridge tools",
		Mode:        "offline",
		Section:     registry.SectionConfiguration,
		Subs:        "plugin, migrate",
	})
}
