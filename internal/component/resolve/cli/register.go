// Design: docs/architecture/api/commands.md — resolve command ownership
//
// Register the `resolve` root command with the importable command registry.
// This is the owner package: the offline DNS/IRR/Cymru/PeeringDB resolver CLI
// lives with internal/component/resolve, not under cmd/ze.
package cli

import "github.com/ze-software/ze/internal/component/command/registry"

func init() {
	registry.MustRegisterRootHandler("resolve", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		Description: "DNS resolver tools",
		Mode:        "offline",
		Section:     registry.SectionSystem,
		Subs:        "",
	})
}
