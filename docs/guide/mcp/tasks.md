# MCP Tasks

<!-- source: internal/component/mcp/tasks.go -- task registry -->
<!-- source: internal/component/mcp/streamable.go -- createTask, tasks/* dispatch -->
<!-- source: internal/component/mcp/task_state.go -- TaskState enum -->

Ze implements MCP 2025-11-25
[tasks](https://modelcontextprotocol.io/specification/2025-11-25/basic/utilities/tasks):
a client may request a long-running tool invocation run as a background
task, receive a `CreateTaskResult` immediately, poll status or subscribe
to notifications, and retrieve the final result when the task completes.

## Capability Negotiation

Clients that support tasks declare `capabilities.tasks = {}` at
`initialize`. The server checks `session.ClientSupportsTasks()` before
accepting task-augmented calls and before exposing `tasks/*` methods.
Clients without the capability receive `method not found` for all
`tasks/*` calls and `-32602` for `tools/call` with a `task` parameter.

The server advertises `"tasks": {}` in its capabilities response.

## Creating a Task

Pass `"task": {}` in the `tools/call` params:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "ze_rib",
    "arguments": {"action": "routes", "peer": "*"},
    "task": {"ttl": 60000}
  }
}
```

The server returns a `CreateTaskResult` immediately:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "taskId": "aBc123...",
    "status": "working"
  }
}
```

The optional `ttl` field (milliseconds) controls how long the terminal
result is retained after completion (default: 5 minutes, clamped to
1 second - 1 hour).

## Task State Machine

```
working -> completed   (dispatch succeeded)
working -> failed      (dispatch returned error)
working -> cancelled   (client or session close)
working -> input_required -> working  (mid-task elicitation)
```

Terminal states (`completed`, `failed`, `cancelled`) are sinks.
`input_required` is non-terminal: the task transitions back to `working`
after the client responds to the elicitation.

## Polling and Notifications

Poll with `tasks/get`:

```json
{"jsonrpc": "2.0", "id": 2, "method": "tasks/get", "params": {"taskId": "aBc123..."}}
```

Or subscribe to status notifications on the GET SSE stream. The server
emits `notifications/tasks/status` with `_meta.io.modelcontextprotocol/related-task`
on every state transition.

## Retrieving Results

When `tasks/get` shows `completed`, retrieve the tool output:

```json
{"jsonrpc": "2.0", "id": 3, "method": "tasks/result", "params": {"taskId": "aBc123..."}}
```

The result has the same shape as a synchronous `tools/call` response.
Calling `tasks/result` on a non-terminal task returns an error.

## Cancellation

```json
{"jsonrpc": "2.0", "id": 4, "method": "tasks/cancel", "params": {"taskId": "aBc123..."}}
```

The worker's context is canceled and the task transitions to `cancelled`.
Canceling a terminal task is a no-op (idempotent).

## Task Support Levels

Each tool advertises `execution.taskSupport` in its `tools/list`
descriptor:

| Level | Meaning |
|-------|---------|
| `optional` | Can be called with or without `task` (default) |
| `required` | Must be called as a task (e.g., `monitor bgp`, `show bgp rib`) |
| `forbidden` | Must not be called as a task (e.g., `clear bgp rib in`, `request bgp rib inject`) |

The level is derived from the YANG `ze:task-support` extension. Calling a
`required` tool without `task: {}` or a `forbidden` tool with `task: {}`
returns `-32602`.

## Identity Scoping

Tasks are scoped to the authenticated identity from the session.
`tasks/list` returns only the caller's tasks; `tasks/get`, `tasks/result`,
and `tasks/cancel` reject cross-identity lookups with a uniform
"not found" error that does not reveal whether the task exists for
another identity.

## Limits

| Limit | Default | Configurable via |
|-------|---------|-----------------|
| Concurrent tasks per identity | 8 | `TaskRegistryConfig.MaxConcurrent` |
| Terminal tasks retained | 128 | `TaskRegistryConfig.MaxTerminal` |
| Terminal result TTL | 5 min | `task.ttl` in request (clamped to 1s - 1h) |

Exceeding the concurrency cap returns `-32602 task concurrency cap
reached`. Terminal tasks are garbage-collected after their TTL expires.

## Session Lifecycle

When a session closes (client DELETE or TTL expiry), all in-flight tasks
for that session are canceled. Terminal tasks remain in the registry
for their TTL so the client can retrieve results even after
reconnecting with a new session (tasks are scoped by identity, not
session).
