// Register the passwd root command with the command registry.

// codegen:skip -- CLI command wired via cmd/ze/main.go, not a runtime plugin.

package passwd

import (
	"github.com/ze-software/ze/internal/component/command/registry"
)

func init() {
	registry.MustRegisterRootHandler("passwd", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		Description: "Change stored SSH/HTTP passwords",
		Mode:        "setup",
		Section:     registry.SectionConfiguration,
	})
}
