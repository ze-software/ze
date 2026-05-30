// Design: docs/features/ai-first.md — explain command registration

package explain

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

func init() {
	cmdregistry.RegisterRoot("explain", cmdregistry.Meta{
		Description: "Look up what a Ze diagnostic code means",
		Mode:        "offline",
		Section:     cmdregistry.SectionSystem,
		Subs:        "--json <code>",
	})
	cmdregistry.MustRegisterLocalMeta("explain", Run, cmdregistry.Meta{
		Description: "Print the meaning, likely cause, and recommended fix for a Ze diagnostic code. Pass the code you saw in a log or error message.",
	})
}
