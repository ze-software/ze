// Register the signal + status root commands with the command registry.

// codegen:skip -- CLI command wired via cmd/ze/main.go, not a runtime plugin.

package signal

import (
	"github.com/ze-software/ze/internal/component/command/registry"
)

func init() {
	registry.MustRegisterRootHandler("signal", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		Description: "Send signals to the daemon via SSH",
		Mode:        "daemon",
		Section:     registry.SectionSystem,
		Subs:        "reload, stop, restart, quit",
	})
	registry.MustRegisterRootHandler("status", func(_ *registry.RuntimeContext, args []string) int {
		return RunStatus(args)
	}, registry.Meta{
		Description: "Check if daemon is running",
		Mode:        "daemon",
		Section:     registry.SectionSystem,
		Subs:        "exit 0 = running, 1 = not",
	})
}
