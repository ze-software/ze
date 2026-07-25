// Design: docs/architecture/api/commands.md — l2tp command ownership
//
// Register the `l2tp` root command with the importable command registry.
// This is the owner package: the offline L2TP CLI (packet decode, show) lives
// with internal/component/l2tp, not under cmd/ze.
package cli

import "github.com/ze-software/ze/internal/component/command/registry"

func init() {
	registry.MustRegisterRootHandler("l2tp", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		Description: "L2TP tools",
		Mode:        "offline",
		Section:     registry.SectionConfiguration,
		Subs:        "",
	})
}
