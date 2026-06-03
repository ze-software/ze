// Design: docs/architecture/api/commands.md — traffic-control command ownership
//
// Register the `traffic-control` root command with the importable command
// registry. This is the owner package: the offline tc/VPP policer CLI lives
// with internal/component/traffic, not under cmd/ze.
package cli

import "codeberg.org/thomas-mangin/ze/internal/component/command/registry"

func init() {
	registry.MustRegisterRootHandler("traffic-control", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		Description: "Linux tc / VPP policer helpers",
		Mode:        "offline",
		Section:     registry.SectionConfiguration,
		Subs:        "",
	})
}
