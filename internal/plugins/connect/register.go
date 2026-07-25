// Design: docs/architecture/system-architecture.md -- ze connect: SSH credential management

// codegen:skip -- CLI command wired via cmd/ze/main.go, not a runtime plugin.

package connect

import "github.com/ze-software/ze/internal/component/command/registry"

func init() {
	registry.MustRegisterRootHandler("connect", func(_ *registry.RuntimeContext, args []string) int {
		return Run(args)
	}, registry.Meta{
		Description: "Manage SSH credentials for remote ze daemons",
		Mode:        "offline",
		Section:     registry.SectionSystem,
		Subs:        "add <host> [--port] [--user], list, remove <host> [--port], default <host> [--port]",
	})
}
