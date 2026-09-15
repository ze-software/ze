// Design: docs/architecture/api/commands.md — firewall command ownership
//
// Register the `firewall` root command with the importable command registry.
// This is the owner package: the firewall CLI lives with
// internal/component/firewall, not under cmd/ze. cmd/ze/main.go dispatches
// `ze firewall ...` through the registry handler registered here.

// codegen:skip -- ze-test registers a `firewall` SUITE root of its own
// (internal/test/cli/register.go, registerCIRoot), and the ze-test binary imports
// plugin/all. A generated blank import would put two `firewall` roots in that one
// binary, and MustRegisterRootHandler panics on the duplicate at init, which takes
// every functional suite down. cmd/ze/ze_core_dispatch.go carries the import instead.

package cli

import "github.com/ze-software/ze/internal/component/command/registry"

func init() {
	registry.MustRegisterRootHandler("firewall", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		ShortHelp: "Firewall management",
		Mode:      "offline",
		Section:   registry.SectionConfiguration,
		Subs:      "show, apply",
	})
}
