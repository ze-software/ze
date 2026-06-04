// Design: docs/architecture/system-architecture.md -- ze connect: SSH credential management

package connect

import "codeberg.org/thomas-mangin/ze/internal/component/command/registry"

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
