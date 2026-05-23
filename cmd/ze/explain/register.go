// Design: docs/features/ai-first.md — explain command registration

package explain

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

func init() {
	cmdregistry.RegisterRoot("explain", cmdregistry.Meta{
		Description: "Explain a diagnostic code",
		Mode:        "offline",
		Section:     cmdregistry.SectionSystem,
		Subs:        "--json <code>",
	})
	cmdregistry.MustRegisterLocal("explain", Run)
}
