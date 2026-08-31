// Design: docs/architecture/core-design.md -- support command registration

// codegen:skip -- CLI command wired via cmd/ze/main.go, not a runtime plugin.

package support

import (
	"github.com/ze-software/ze/internal/component/command/registry"
	impl "github.com/ze-software/ze/internal/component/support"
)

func init() {
	registry.RegisterRoot("support", registry.Meta{
		Description: "Collect logs, config, and diagnostics into a support archive",
		Mode:        "offline",
		Section:     registry.SectionSystem,
		Subs:        "[--module M] [--exclude M] [--since T] [--reason R] [--sensitive] [--json] [--list-modules]",
	})
	registry.MustRegisterLocalMeta("support", impl.Run, registry.Meta{
		Description: "Collect logs, config, state and diagnostics into one archive.",
		LongHelp: "Send the archive to support when you report an issue. Modules can be selected or " +
			"excluded, and a time window narrows what the archive holds.",
	})
}
