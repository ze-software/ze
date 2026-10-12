// Design: docs/features/ai-first.md — show skills and show skill command registration

package skills

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
)

func init() {
	command.MustRegisterLocalMeta("show skills", showSkills, registry.Meta{
		ShortHelp: "List the agent skills this binary carries.",
		Description: "Each skill is a Markdown document bundled with the binary, so it always matches " +
			"the running version. show skill fetches one of them by name.",
		Mode: "offline",
	})
	command.MustRegisterLocalMeta("show skill", showSkill, registry.Meta{
		ShortHelp: "Fetch one bundled agent skill by name.",
		Description: "The skill is printed in its short form, or in full with examples and patterns. " +
			"show skills lists the names.",
		Mode: "offline",
	})
}
