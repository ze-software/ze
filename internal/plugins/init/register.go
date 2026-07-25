// Register the init root command with the command registry.

// codegen:skip -- CLI command wired via cmd/ze/main.go, not a runtime plugin.

package init

import (
	"github.com/ze-software/ze/internal/component/command/registry"
)

func init() {
	registry.MustRegisterRootHandler("init", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		Description: "Bootstrap database with SSH credentials",
		Mode:        "setup",
		Section:     registry.SectionSystem,
		Subs:        "--managed for fleet mode, --force to replace",
	})
}
