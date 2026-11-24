// Design: docs/architecture/exabgp-bridge.md -- register the `ze exabgp` root command and flags.

// codegen:skip -- CLI command wired via cmd/ze/main.go, not a runtime plugin.

package exabgp

import (
	"github.com/ze-software/ze/internal/component/command/registry"
)

const modeOffline = "offline"

func init() {
	registry.MustRegisterRootHandler("exabgp", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		ShortHelp: "ExaBGP bridge tools",
		Mode:      modeOffline,
		Section:   registry.SectionConfiguration,
		Subs:      "plugin, migrate",
	})

	// Flag inventory for shell completion (registration over hardcoding).
	// Mirrors the flag.FlagSet declarations in main.go's cmdPlugin/cmdMigrate.
	registry.RegisterCommandFlags("exabgp plugin", []registry.FlagSpec{
		{Name: "--family", Description: "address family to enable (repeatable)", ValueHint: registry.FlagValueFamily},
		{Name: "--route-refresh", Description: "advertise the route-refresh capability", ValueHint: registry.FlagValueNone},
		{Name: "--add-path", Description: "advertise the add-path capability", ValueHint: registry.FlagValueText},
		{Name: "--encoder", Description: "script event encoding: text or json (default: text)", ValueHint: registry.FlagValueText},
		{Name: "--form", Description: "message rendering form modifier <[neighbor:]direction-form> (repeatable)", ValueHint: registry.FlagValueText},
	})
	registry.RegisterCommandFlags("exabgp migrate", []registry.FlagSpec{
		{Name: "--dry-run", Description: "preview conversion diagnostics without printing converted config", ValueHint: registry.FlagValueNone},
		{Name: "--env", Description: "migrate an ExaBGP INI environment file instead", ValueHint: registry.FlagValueFile},
	})
}
