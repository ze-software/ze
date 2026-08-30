// Design: docs/architecture/api/commands.md — tacacs command ownership
//
// Register the `tacacs` root command with the importable command registry.
// This is the owner package: the offline TACACS+ client CLI lives with
// internal/component/tacacs, not under cmd/ze.
package cli

import "github.com/ze-software/ze/internal/component/command/registry"

// modeOffline is the help tag for a command that answers with no running
// daemon. The probe reads a config file and dials from this process, so it is
// one.
const modeOffline = "offline"

func init() {
	registry.MustRegisterRootHandler("tacacs", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		Description: "TACACS+ client helpers",
		Mode:        modeOffline,
		Section:     registry.SectionConfiguration,
		Subs:        "show",
	})
}
