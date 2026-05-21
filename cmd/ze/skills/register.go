// Design: docs/features/ai-first.md — skills command registration

package skills

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

func init() {
	cmdregistry.RegisterRoot("skills", cmdregistry.Meta{
		Description: "Version-matched Ze skills for agents",
		Mode:        "offline",
		Subs:        "list, get <name> [--full]",
	})
	cmdregistry.MustRegisterLocal("skills", Run)
}
