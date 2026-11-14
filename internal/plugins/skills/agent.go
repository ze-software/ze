// Design: docs/features/ai-first.md — setting an operator's agent up to use Ze
// Related: register.go — the four commands' local and daemon registrations
// Related: agent_skill.go — create, show and delete agent skill on both routes
// Related: main.go — the bundled guides the ze stub sends the agent to

package skills

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
