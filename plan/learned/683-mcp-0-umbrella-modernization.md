# 683 -- MCP 0 Umbrella (Protocol Modernization)

## Context

The MCP umbrella (spec-mcp-0-umbrella) coordinated five phases of MCP
protocol modernization. All five phases have landed:

| Phase | Spec | Learned | Delivers |
|-------|------|---------|----------|
| 1 | spec-mcp-1-streamable-http | 636 | Streamable HTTP transport, sessions, SSE, Origin validation |
| 2 | spec-mcp-2-remote-oauth | 638 | Remote binding, OAuth 2.1 resource server, per-identity bearer list |
| 3 | spec-mcp-3-elicitation | 640 | Server-initiated `elicitation/create`, POST-to-SSE upgrade |
| 4 | spec-mcp-4-tasks | 681 | Task registry, `tasks/*` methods, worker goroutines, mid-task elicit |
| 5 | spec-mcp-5-apps | 682 | Resources capability, `ui://` scheme, embedded UI bundles |

## Decisions

- Phases landed independently. Each was its own spec with its own TDD
  cycle and learned summary. The umbrella tracked ordering and cross-
  cutting concerns but never went through `/implement` itself.

- The protocol version stayed at `2025-06-18` throughout. Task-augmented
  requests use capability negotiation, not a version bump to `2025-11-25`.

- `handler.go` (legacy 2024-11-05) was NOT deleted. `ze-chaos` still
  uses the legacy `Handler()` factory. Migration is deferred.

## What Worked

- The phased approach prevented the "partial wiring" trap. Each phase
  delivered a self-contained, testable feature with `make ze-verify`
  passing before the next phase started.

- The YANG extension pattern (`ze:task-support`, `ze:ui-resource`)
  proved composable. Each phase that added tool-descriptor metadata
  followed the same walker -> map -> CommandInfo -> buildToolDef chain.

- Session capability bits (`clientElicit`, `clientTasks`, `clientResources`)
  kept the capability gate simple: one bool per feature, checked before
  dispatch.

## What to Watch

- Auth mode `oauth` (Phase 2) is implemented as resource-server
  validation only. Ze does not run an authorization server. The AS
  metadata discovery path needs real-world testing with an external AS.

- `resources/updated` notifications and `resources/list` pagination are
  deferred. Both become relevant if UI bundles grow or become mutable.

## Files

All files are documented in the per-phase learned summaries (636, 638,
640, 681, 682). The umbrella added no files of its own.
