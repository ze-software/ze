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
		Subs:      "--managed --force --yes --web-cert <address> --web-cert-name <name> --seed",
	})
}
