// Design: docs/features/ai-first.md — skills command registration
// Related: agent.go — the four commands that set an operator's agent up

package skills

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// modeOffline is the help tag of a command a shell answers with no daemon.
const modeOffline = "offline"

func init() {
	// Both commands answer DATA, so their answers reach the pipe layer; the
	// drawings give the text route what the commands always printed.
	command.MustRegisterTextRenderer(keySkills, drawSkills)
	command.MustRegisterTextRenderer(keySkill, drawSkill)
	command.MustRegisterTextRenderer(keyAgentContext, drawAgentContext)
	command.MustRegisterTextRenderer(keyAgentSkill, drawAgentSkill)
	command.MustRegisterLocalData("show skills", showSkills, registry.Meta{
		ShortHelp: "List the agent skills this binary carries.",
		Description: "Each skill is a Markdown document bundled with the binary, so it always matches " +
			"the running version. show skill fetches one of them by name.",
		Mode: modeOffline,
	}, render)
	command.MustRegisterLocalData("show skill", showSkill, registry.Meta{
		ShortHelp: "Fetch one bundled agent skill by name.",
		Description: "The skill is printed in its short form, or in full with examples and patterns. " +
			"show skills lists the names.",
		Mode: modeOffline,
	}, render)

	// The agent setup commands are plain local commands, not local-data ones:
	// a person sets their agent up at a shell, where the handler reaches their
	// home, while `ze cli` is the router's CLI and asks the daemon, whose own
	// command below answers the stub to place and writes nothing. A local-data
	// registration would make `ze cli -c` install on the operator's machine and
	// the bridge serve the handler against the daemon's home.
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: wireCreateAgentSkill, Handler: handleCreateAgentSkill},
		pluginserver.RPCRegistration{WireMethod: wireShowAgentSkill, Handler: handleShowAgentSkill},
		pluginserver.RPCRegistration{WireMethod: wireDeleteAgentSkill, Handler: handleDeleteAgentSkill},
		pluginserver.RPCRegistration{WireMethod: wireShowAgentContext, Handler: handleShowAgentContext},
	)
	command.MustRegisterLocalMeta(pathCreateAgentSkill, createAgentSkillLocal, registry.Meta{
		ShortHelp: "Install the ze skill for the operator's agent harnesses.",
		Description: "Writes a stub skill named ze, which sends the agent to the guide this binary " +
			"carries, for every harness found or for the one agent names. For Claude Code and " +
			"Codex it also adds one session-start hook, which runs ze show agent context, to the " +
			"harness's settings file.",
		Mode: modeOffline,
	})
	command.MustRegisterLocalMeta(pathShowAgentSkill, showAgentSkillLocal, registry.Meta{
		ShortHelp: "Show whether the ze skill is set up for each agent harness.",
		Mode:      modeOffline,
	})
	command.MustRegisterLocalMeta(pathDeleteAgentSkill, deleteAgentSkillLocal, registry.Meta{
		ShortHelp: "Remove the ze skill from the operator's agent harnesses.",
		Description: "Acts on every harness found, or on the one agent <name> names; agent all " +
			"also spells every harness found. " +
			"Removes the stub and the session-start hook Ze installed, and nothing else.",
		Mode: modeOffline,
	})
	command.MustRegisterLocalMeta(pathShowAgentContext, showAgentContextLocal, registry.Meta{
		ShortHelp: "Show what an agent session is told about this Ze.",
		Mode:      modeOffline,
	})
}
