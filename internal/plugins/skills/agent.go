// Design: docs/features/ai-first.md — setting an operator's agent up to use Ze
// Related: register.go — the four commands' local and daemon registrations
// Related: main.go — the bundled guides the ze stub sends the agent to

package skills

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The four command paths. Each is a plain local command, answered by
// `ze <command>` at a shell, where the caller's own home is in reach, and a
// builtin RPC on the same YANG node, which answers `ze cli` and every daemon
// surface without reading or writing a file, as `show ping` is built
// (docs/architecture/api/commands.md, "Local commands already served by the
// daemon under the same path").
const (
	pathCreateAgentSkill = "create agent skill"
	pathShowAgentSkill   = "show agent skill"
	pathDeleteAgentSkill = "delete agent skill"
	pathShowAgentContext = "show agent context"
)

// The wire methods ze-skills-cmd.yang declares on the four nodes.
const (
	wireCreateAgentSkill = "ze-skills:create-agent-skill"
	wireShowAgentSkill   = "ze-skills:show-agent-skill"
	wireDeleteAgentSkill = "ze-skills:delete-agent-skill"
	wireShowAgentContext = "ze-skills:show-agent-context"
)

// notImplemented is the refusal every route of the four commands answers
// until the plan producer and the context producer exist. fix is the command
// that reads the guide instead, in the spelling of the route that refuses:
// `ze show skill ze` at a shell, `show skill ze` through the daemon.
func notImplemented(path, fix string) *opererr.Error {
	var problem textbuf.Buffer
	problem.Str(path).Str(" is not implemented yet")
	return opererr.New(problem.String(),
		"This build declares the command but carries none of the code that sets an agent up, "+
			"so nothing was read or changed. Read the guide the stub would point the agent at instead.",
		fix)
}

// refuseLocal is the shell route of a command that is not implemented yet: it
// prints the refusal and exits 1.
func refuseLocal(path string) int {
	return command.PrintLocalAnswer(path, command.LocalRefusal(1, command.LocalErrorLine(notImplemented(path, "ze show skill ze"))))
}

// refuseRemote is the daemon route of a command that is not implemented yet.
func refuseRemote(path string) *plugin.Response {
	return plugin.OperatorErrorResponse(notImplemented(path, "show skill ze"))
}

// createAgentSkillLocal is the shell route of create agent skill.
func createAgentSkillLocal(_ command.ValidatedArgs) int {
	return refuseLocal(pathCreateAgentSkill)
}

// showAgentSkillLocal is the shell route of show agent skill.
func showAgentSkillLocal(_ command.ValidatedArgs) int {
	return refuseLocal(pathShowAgentSkill)
}

// deleteAgentSkillLocal is the shell route of delete agent skill.
func deleteAgentSkillLocal(_ command.ValidatedArgs) int {
	return refuseLocal(pathDeleteAgentSkill)
}

// showAgentContextLocal is the shell route of show agent context.
func showAgentContextLocal(_ command.ValidatedArgs) int {
	return refuseLocal(pathShowAgentContext)
}

// handleCreateAgentSkill is the daemon command of create agent skill.
func handleCreateAgentSkill(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	return refuseRemote(pathCreateAgentSkill), nil
}

// handleShowAgentSkill is the daemon command of show agent skill.
func handleShowAgentSkill(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	return refuseRemote(pathShowAgentSkill), nil
}

// handleDeleteAgentSkill is the daemon command of delete agent skill.
func handleDeleteAgentSkill(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	return refuseRemote(pathDeleteAgentSkill), nil
}

// handleShowAgentContext is the daemon command of show agent context.
func handleShowAgentContext(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	return refuseRemote(pathShowAgentContext), nil
}
