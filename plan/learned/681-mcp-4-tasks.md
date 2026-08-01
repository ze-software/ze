# 681 -- MCP 4 Tasks

## Context

MCP 2025-11-25 adds task-augmented `tools/call`: a client passes
`task: {}` on a call, the server returns `CreateTaskResult` immediately
and runs the dispatch in a background goroutine, and the client polls
status or subscribes to notifications. Ze's Phases 1-3 shipped the
Streamable HTTP transport, OAuth identity, and elicitation, but every
`tools/call` was synchronous. Long-running commands (RIB dumps, route
monitoring) blocked the POST until completion. Goal: land the task
registry, worker goroutines, `tasks/*` methods, mid-task elicitation
integration, and `execution.taskSupport` in tool descriptors.

## Decisions

- **Task registry is a standalone struct** (`taskRegistry`), not merged
  into the session registry. Tasks are identity-scoped (not
  session-scoped in the registry) per the MCP spec, so they need their
  own index. The session owns the notification stream; the task owns the
  worker lifecycle. Connection between them: session ID on the task
  entry, `CancelAllForSession` on session close.

- **Worker goroutines receive ctx from registry.Create**, not from the
  HTTP request. The task outlives the POST that created it. Cancel
  propagates through `entry.cancel()` in `taskRegistry.Cancel` and
  `CancelAllForSession`.

- **`TaskElicit` wraps `session.Elicit` with state transitions** rather
  than modifying Elicit itself. The task transitions to
  `input_required` before the elicit frame is emitted, and back to
  `working` on accept. This keeps the elicit code path unchanged for
  non-task callers.

- **`execution.taskSupport` derived from YANG `ze:task-support`
  extension**, not a hardcoded tool-name list. `CommandInfo.TaskSupport`
  flows from YANG through the command lister into `groupCommands` and
  `buildToolDef`. The wire values (`optional`, `required`, `forbidden`)
  are emitted in every tool descriptor.

- **handler.go kept for ze-chaos.** The spec called for deletion, but
  `ze-chaos` uses the legacy `Handler()` factory with its own
  `ToolProvider`. Deleting would break a separate binary. The goconst
  lint issue was fixed. Migration of ze-chaos is deferred.

- **Session `onExpire` callback** connects session GC to task
  cancellation. The session registry calls `onExpire(sessionID)` for
  each expired session before closing it. `NewStreamable` sets this to
  `taskReg.CancelAllForSession`.

- **Tasks capability advertised in initialize.** The server includes
  `"tasks": {}` in its capabilities response so clients know `tasks/*`
  methods are available.

## What Worked

- The correlation-map pattern from Phase 3 (elicitation) translated
  cleanly: task state lives in its own registry but the notification
  delivery uses the same `session.Send(frame)` path.
- TDD caught the TTL clamping bug early: client-requested TTL of 100ms
  was clamped to `minTaskTTL` (1 second) by `clampTTL`, making the
  expiry test fail until the test was fixed to use the registry default.
- The `lookupTaskSupport` method keeps enforcement in `callTool` clean:
  two checks (forbidden+task, required+no-task) before dispatch.

## What to Watch

- `tasks/list` pagination is deferred. Current design returns all tasks
  for the identity (bounded by the concurrency cap). If the cap grows
  beyond ~100, pagination becomes necessary.
- The correlation map was NOT renamed from `correlations` to `pending`
  as the spec suggested. The Phase 3 elicit-specific naming still works
  because tasks use a separate registry, not the correlation map.

## Files

| File | Change |
|------|--------|
| `internal/component/mcp/task_state.go` | New: typed `TaskState uint8` enum |
| `internal/component/mcp/tasks.go` | New: task registry, worker, `TaskElicit`, notifications |
| `internal/component/mcp/session.go` | `clientTasks` bit, `ClientSupportsTasks()`, `onExpire` callback |
| `internal/component/mcp/streamable.go` | `tasks/*` dispatch, `createTask`, `parseTasksCapability`, `lookupTaskSupport`, tasks capability in initialize |
| `internal/component/mcp/handler.go` | `callParams.Task` field, goconst fix |
| `internal/component/mcp/tools.go` | `TaskSupportLevel`, `execution.taskSupport` in tool descriptors |
| `internal/component/command/node.go` | `TaskSupport` field |
| `internal/component/config/yang/command.go` | `GetTaskSupportExtension`, `PathToTaskSupport` |
| `internal/component/config/yang/modules/ze-extensions.yang` | `ze:task-support` extension |
| `cmd/ze/hub/main.go` | Wire `TaskSupport` through command lister |
| `cmd/ze/hub/service_mcp.go` | `buildTaskSupportMap`, `parseTaskSupportLevel` |
| `docs/architecture/mcp/overview.md` | Task section, capability table, roadmap |
| `docs/architecture/api/commands.md` | `tasks/*` methods, `notifications/tasks/status` |
| `internal/test/cli/cmd_mcp.go` | `--tasks` flag, task directives (task-call/get/result/cancel/list/wait), `$LAST` substitution |
| `internal/component/bgp/plugins/cmd/monitor/yang/ze-monitor-cmd.yang` | `ze:task-support required` on monitor bgp/event |
| `internal/component/bgp/plugins/cmd/rib/yang/ze-rib-cmd.yang` | `ze:task-support required` on routes, `forbidden` on clear/inject/withdraw |
| `internal/component/cmd/subscribe/yang/ze-cli-subscribe-cmd.yang` | `ze:task-support required` on subscribe |
| `docs/guide/mcp/tasks.md` | New: user guide for MCP tasks |
| `docs/guide/mcp/overview.md` | Task directives in test client table |
| `docs/features.md` | MCP row updated with tasks |
| `docs/comparison.md` | MCP tasks row added |
| `docs/functional-tests.md` | Task test section (3c) |
| `test/plugin/task-rib-routes.ci` | New: end-to-end task create/poll/result |
| `test/plugin/task-cancel.ci` | New: task cancellation |
| `test/plugin/task-forbidden.ci` | New: forbidden tool rejection |
| `test/plugin/task-identity-scope.ci` | New: identity scoping isolation |
