// Register the init root command with the command registry.

package init

import (
	"github.com/ze-software/ze/internal/component/command/registry"
)

func init() {
	registry.MustRegisterRootHandler("init", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		ShortHelp: "Initialize a live store or appliance seed with SSH credentials",
		Mode:      "setup",
		Section:   registry.SectionSystem,
		Subs:      "--managed --force --yes --web-cert <address> --web-cert-name <name> --seed --from <blob>",
	})

	// Flag inventory for shell completion (registration over hardcoding).
	// Mirrors the flag.FlagSet declarations in main.go's Run.
	registry.RegisterCommandFlags("init", []registry.FlagSpec{
		{Name: flagManaged, Description: "enable managed (fleet) mode", ValueHint: registry.FlagValueNone},
		{Name: "--force", Description: "replace an existing store (moves the old one to .replaced-<date>)", ValueHint: registry.FlagValueNone},
		{Name: "--yes", Description: "skip the confirmation prompt (use with --force)", ValueHint: registry.FlagValueNone},
		{Name: "--web-cert", Description: "generate a TLS certificate for the web server listen address", ValueHint: registry.FlagValueNone},
		{Name: "--web-cert-name", Description: "extra DNS name for the TLS certificate SAN", ValueHint: registry.FlagValueNone},
		{Name: "--seed", Description: "create an appliance seed artifact without interface discovery", ValueHint: registry.FlagValueNone},
		{Name: "--from", Description: "import a local blob artifact into the live store", ValueHint: registry.FlagValueFile},
	})
}
