# AI-First Design

## Meta

| Field | Value |
|-------|-------|
| Name | AI-First Design |
| Page | docs/features/ai-first.md |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/mcp, internal/plugins/skills |
| Real-path tests | test/plugin/mcp-discover-daemon.ci, test/plugin/mcp-tools-list-stateless.ci, test/plugin/mcp-tools-call-stateless.ci, test/ui/help-ai.ci, test/ui/help-ai-json.ci, test/ui/agent-skill-create-show.ci, test/ui/agent-skill-delete.ci, test/ui/agent-context.ci |
| RFCs | rfc9728 |
| Docs | docs/features/ai-first.md |
| Doc review | 2026-11-14: re-read the two sections 028062336c and 2388cd9e93 added to docs/features/ai-first.md, "Setting an agent up" and "Session context", against their producers: the harness table, stubText (the local and the remote body) and zeGuide (internal/plugins/skills/agent_skill.go); hookCommand, hookTimeoutSeconds, hookOwnerWords and ownsCommand, the Codex trust notes, readLayout and scanLayout (the byte-order mark, a non-object `hooks` value or group, the line and column), the 0o600 mode of a new file and the kept permission bits in replace, stillAsRead, and withoutGroup's empty-in-place for Codex (internal/plugins/skills/agent_hook.go); dialBound, guideCommand, printLocal and respond, whose daemon answer carries no directory (internal/plugins/skills/agent_context.go). The page matches them. Source review only. 2026-11-10: re-read the doctor `fix` paragraph ce58dddf53 added to docs/features/ai-first.md, the page's only change since 2026-11-04, against Diagnostic.Fix (internal/core/diagnostic/types.go: the bare command, `json:"fix,omitempty"`, separate from Repair and FixSafety), outputText and outputJSON (internal/component/doctor/doctor.go) reached through renderDoctor, and the daemon route's readinessAnswer, which marshals the same DoctorResult; the prose now says the text line opens with `fix:`. 2026-11-04: independently compared the Description and docs/features/ai-first.md against reference/help producers, MCP tool generation and dispatch, command-surface restrictions, diagnostics, bundled skills, commit preparation, and builtin/plugin discovery; registry-derived listings are distinguished from authored guidance, without universal CLI/MCP parity or freshness claims. Source review only; runtime evidence remains separately bounded. |
| Defect review | 2026-10-07: open: plan/release/fixes/spec-mcp-protected-resource-metadata-path.md (RFC 9728 metadata path) |

## Description

Self-describing CLI-as-API with MCP transport for AI assistants. `ze create agent skill` installs the `ze` skill in Claude Code, Codex, OpenCode and Pi, and a session-start hook in Claude Code and Codex, so an agent reads the guide the installed binary serves. <!-- source: internal/plugins/skills/agent_skill.go -- harnesses, stubText --> <!-- source: internal/plugins/skills/agent_hook.go -- hookCommand -->
