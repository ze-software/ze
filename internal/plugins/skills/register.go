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
		Description: "List the agent skills this binary carries, or fetch one by name.",
		LongHelp: "Each skill is a Markdown document bundled with the binary, so it always matches " +
			"the running version. One skill is fetched by name, in its short form or in full.",
	})
}
