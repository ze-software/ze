// Design: docs/architecture/api/commands.md — tacacs command ownership
//
// Register the `tacacs` root command with the importable command registry.
// This is the owner package: the offline TACACS+ client CLI lives with
// internal/component/tacacs, not under cmd/ze.
package cli

import "github.com/ze-software/ze/internal/component/command/registry"

func init() {
	registry.MustRegisterRootHandler("tacacs", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		Description: "TACACS+ client helpers",
		Mode:        "offline",
		Section:     registry.SectionConfiguration,
		Subs:        "",
	})
}
