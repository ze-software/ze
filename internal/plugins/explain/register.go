// Design: docs/features/ai-first.md — explain command registration

// codegen:skip -- CLI command wired via cmd/ze/main.go, not a runtime plugin.

package explain

import (
	"github.com/ze-software/ze/internal/component/command/registry"
)

func init() {
	registry.RegisterRoot("explain", registry.Meta{
		Description: "Look up what a Ze diagnostic code means",
		Mode:        "offline",
		Section:     registry.SectionSystem,
		Subs:        "--json <code>",
	})
	registry.MustRegisterLocalMeta("explain", Run, registry.Meta{
		Description: "Explain one diagnostic code Ze printed.",
		LongHelp: "The answer gives the meaning of the code, its likely cause and the recommended " +
			"fix. Pass the code you read in a log line or an error message.",
	})
}
