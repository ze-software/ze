// Design: docs/architecture/api/commands.md — sysctl command ownership
//
// Register the `sysctl` root command with the importable command registry.
// This is the owner package: the offline sysctl CLI lives with the sysctl
// plugin, not under cmd/ze. cmd/ze/main.go dispatches `ze sysctl ...` through
// the registry handler registered here.
package cli

import "github.com/ze-software/ze/internal/component/command/registry"

func init() {
	registry.MustRegisterRootHandler("sysctl", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		Description: "Kernel sysctl helpers",
		Mode:        "offline",
		Section:     registry.SectionConfiguration,
		Subs:        "",
	})
}
