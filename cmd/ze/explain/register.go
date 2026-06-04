// Design: docs/features/ai-first.md — explain command registration

package explain

import (
	"codeberg.org/thomas-mangin/ze/internal/component/command/registry"
)

func init() {
	registry.RegisterRoot("explain", registry.Meta{
		Description: "Look up what a Ze diagnostic code means",
		Mode:        "offline",
		Section:     registry.SectionSystem,
		Subs:        "--json <code>",
	})
	registry.MustRegisterLocalMeta("explain", Run, registry.Meta{
		Description: "Print the meaning, likely cause, and recommended fix for a Ze diagnostic code. Pass the code you saw in a log or error message.",
	})
}
