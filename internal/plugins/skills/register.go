// Design: docs/features/ai-first.md — show skills and show skill command registration

package skills

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
)

func init() {
	// Both commands answer DATA, so their answers reach the pipe layer; the
	// drawings give the text route what the commands always printed.
	command.MustRegisterTextRenderer(keySkills, drawSkills)
	command.MustRegisterTextRenderer(keySkill, drawSkill)
	command.MustRegisterLocalData("show skills", showSkills, registry.Meta{
		ShortHelp: "List the agent skills this binary carries.",
		Description: "Each skill is a Markdown document bundled with the binary, so it always matches " +
			"the running version. show skill fetches one of them by name.",
		Mode: "offline",
	}, render)
	command.MustRegisterLocalData("show skill", showSkill, registry.Meta{
		ShortHelp: "Fetch one bundled agent skill by name.",
		Description: "The skill is printed in its short form, or in full with examples and patterns. " +
			"show skills lists the names.",
		Mode: "offline",
	}, render)
}
