// Design: docs/features/ai-first.md — explain command registration

package explain

import (
	"github.com/ze-software/ze/internal/component/command/registry"
)

func init() {
	registry.RegisterRoot("explain", registry.Meta{
		ShortHelp: "Look up what a Ze diagnostic code means",
		Mode:      "offline",
		Section:   registry.SectionSystem,
		Subs:      "--json <code>",
	})
	registry.MustRegisterLocalMeta("explain", Run, registry.Meta{
		ShortHelp: "Explain one diagnostic code Ze printed.",
		Description: "The answer gives the meaning of the code, its likely cause and the recommended " +
			"fix. Pass the code you read in a log line or an error message.",
	})
}
