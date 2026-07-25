// Design: docs/architecture/api/commands.md — firewall command ownership
//
// Register the `firewall` root command with the importable command registry.
// This is the owner package: the firewall CLI lives with
// internal/component/firewall, not under cmd/ze. cmd/ze/main.go dispatches
// `ze firewall ...` through the registry handler registered here.
package cli

import "github.com/ze-software/ze/internal/component/command/registry"

func init() {
	registry.MustRegisterRootHandler("firewall", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		Description: "Firewall management",
		Mode:        "offline",
		Section:     registry.SectionConfiguration,
		Subs:        "show, apply",
	})
}
