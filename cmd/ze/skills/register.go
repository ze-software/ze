// Design: docs/features/ai-first.md — skills command registration

package skills

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

func init() {
	cmdregistry.RegisterRoot("skills", cmdregistry.Meta{
		Description: "Agent skills matched to this Ze version",
		Mode:        "offline",
		Section:     cmdregistry.SectionSystem,
		Subs:        "list, get <name> [--full]",
	})
	cmdregistry.MustRegisterLocalMeta("skills", Run, cmdregistry.Meta{
		Description: "List or retrieve agent skill definitions matching this Ze version. Use 'get <name>' to fetch a specific skill.",
	})
}
