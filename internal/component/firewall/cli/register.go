// Design: docs/architecture/api/commands.md — firewall command ownership
//
// Register the `firewall` root command with the importable command registry.
// This is the owner package: the firewall CLI lives with
// internal/component/firewall, not under cmd/ze. cmd/ze/main.go dispatches
// `ze firewall ...` through the registry handler registered here.

// codegen:skip -- cmd/ze/ze_core_dispatch.go carries the import. The skip was made because
// the harness registered a `firewall` SUITE root of its own and linked plugin/all, so
// a generated blank import put two `firewall` roots in one binary and
// MustRegisterRootHandler panicked at init. Since the harness became
// `le test <name>` (plan/spec-le-subject-first-command-tree.md, D-8) it registers
// no root, so that collision no longer exists; the skip stands until the
// composition root is regenerated without it.

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
