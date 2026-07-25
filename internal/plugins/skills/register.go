// Design: docs/features/ai-first.md — skills command registration

// codegen:skip -- CLI command wired via cmd/ze/main.go, not a runtime plugin.

package skills

import (
	"github.com/ze-software/ze/internal/component/command/registry"
)

func init() {
	registry.RegisterRoot("skills", registry.Meta{
		Description: "Agent skills matched to this Ze version",
		Mode:        "offline",
		Section:     registry.SectionSystem,
		Subs:        "list, get <name> [--full]",
	})
	registry.MustRegisterLocalMeta("skills", Run, registry.Meta{
		Description: "List or retrieve agent skill definitions matching this Ze version. Use 'get <name>' to fetch a specific skill.",
	})
}
