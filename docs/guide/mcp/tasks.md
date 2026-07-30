# MCP Tasks

<!-- source: internal/component/mcp/tasks.go -- task registry -->
<!-- source: internal/component/mcp/streamable_tools.go -- createTask, tasks/* dispatch -->
<!-- source: internal/component/mcp/task_state.go -- TaskState enum -->

Some Ze commands take a long time: streaming monitors, full RIB dumps, pings and
traceroutes. Rather than hold an HTTP request open for the whole run, Ze answers
those calls with a **task handle** and lets the client poll for the result.

This is the `io.modelcontextprotocol/tasks` extension. Tasks were an
experimental part of the core protocol in `2025-11-25`; in `2026-07-28` they are
an optional extension, and both the negotiation and the method set changed.

## The server decides, not the client

This is the change most likely to surprise you if you used the earlier revision.

There is no per-call opt-in. A client no longer passes `task: {}` on a
`tools/call` to request background execution. Instead the **server** decides,
per tool, from the `ze:task-support` annotation on the command's YANG. The
client declares once per request that it understands task handles, and then
handles whichever result shape arrives.

<!-- source: internal/component/mcp/streamable_tools.go -- callTool eligibility decision -->

| Annotation | What the server does |
|------------|----------------------|
| `required` | Always returns a task handle (to a client that declared the extension) |
| `forbidden` | Never returns a task handle; the call runs synchronously |
| `optional` (default) | The call runs synchronously |

`forbidden` is a safety property, not a formality. It marks the commands that
mutate state -- `clear bgp rib in`, `clear bgp rib out`, `request bgp rib inject`
and `request bgp rib withdraw` -- and it is what stops the server-directed rule
from putting a route injection into the background.

When a tool groups several commands, a single `forbidden` action makes the whole
group forbidden. The precedence fails closed: the cost of being wrong that way
is one long command running synchronously, and the cost of being wrong the other
way is auto-tasking a mutation.

<!-- source: internal/component/mcp/tools.go -- groupTaskSupport -->

## Declaring the extension

Extension support is declared per request, in the `_meta` block every
`2026-07-28` request carries, under the `extensions` key:

```json
{
  "_meta": {
    "io.modelcontextprotocol/protocolVersion": "2026-07-28",
    "io.modelcontextprotocol/clientCapabilities": {
      "extensions": {
        "io.modelcontextprotocol/tasks": {}
      }
    }
  }
}
```

<!-- source: internal/component/mcp/meta.go -- parseClientCapabilities -->

The bare `tasks` member that `2025-11-25` used is **not** accepted. `tasks` is
not a member of `ClientCapabilities` in this revision, and honoring the old
spelling would push an unsolicited task handle at a client that only ever agreed
to the older, client-directed model.

The server advertises the same identifier in `server/discover`:

```json
{
  "capabilities": {
    "tools": {},
    "resources": {},
    "extensions": {
      "io.modelcontextprotocol/ui": {},
      "io.modelcontextprotocol/tasks": {}
    }
  }
}
```

<!-- source: internal/component/mcp/discover.go -- serverCapabilities -->

That advertisement is load-bearing. A client's set of legal `resultType` values
is the core set plus the values contributed by extensions the server advertises,
so `resultType: "task"` is only interpretable because this row is present.

### If you do not declare it

Your call still works. A client that has not adopted the extension gets the
ordinary synchronous result, with `resultType: "complete"` -- the command runs,
it just holds the request open for its duration.

The extension is an optimization over a synchronous call, never a precondition
for the work. Refusing the call instead would make every `required` command
unreachable to any client that has not adopted an optional extension.

The `tasks/*` methods themselves are a different matter: those *are* the
extension, so calling one without declaring it is refused with `-32021`
(`MissingRequiredClientCapability`) and HTTP 400, carrying
`data.requiredCapabilities` naming the extension.

<!-- source: internal/component/mcp/streamable_tools.go -- failMissingTasksCapability -->

## Creating a task

Nothing special: an ordinary `tools/call`.

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "ze_show_bgp",
    "arguments": { "action": "rib" },
    "_meta": {
      "io.modelcontextprotocol/protocolVersion": "2026-07-28",
      "io.modelcontextprotocol/clientCapabilities": {
        "extensions": { "io.modelcontextprotocol/tasks": {} }
      }
    }
  }
}
```

Because `show bgp rib` is annotated `required`, the server registers a task,
starts a worker, and answers immediately with a `CreateTaskResult`:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "resultType": "task",
    "taskId": "aBc123...",
    "status": "working",
    "ttlMs": 300000,
    "pollIntervalMs": 1000,
    "_meta": {
      "io.modelcontextprotocol/serverInfo": { "name": "ze-mcp", "version": "2.0.0" }
    }
  }
}
```

<!-- source: internal/component/mcp/streamable_tools.go -- createTask -->

`ttlMs` is how long the terminal result is retained once the task finishes.
`pollIntervalMs` is how often to poll, and it is derived rather than fixed: it is
capped at half the TTL, so a client that obeys the hint always polls at least
twice inside the retention window and cannot sleep past its own result.

<!-- source: internal/component/mcp/tasks.go -- retentionHints -->

The task is registered before the response is written, so a client that polls
the instant it reads the handle always finds it.

## Polling

`tasks/get` is the only way to observe a task. This revision has no
server-to-client stream on this transport, so nothing is pushed.

```json
{ "jsonrpc": "2.0", "id": 2, "method": "tasks/get", "params": { "taskId": "aBc123..." } }
```

A working task reports its status and nothing else. A **terminal** task carries
its outcome in the same response: `result` when it completed, `error` when it
failed.

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "resultType": "complete",
    "taskId": "aBc123...",
    "status": "completed",
    "result": { "content": [ { "type": "text", "text": "..." } ] }
  }
}
```

<!-- source: internal/component/mcp/tasks.go -- TaskInfo.toWire -->

That is why there is no `tasks/result`: the payload rides on the poll. The
result is stored before the state goes terminal, so a client that sees
`completed` and reads `result` from the same response can never observe a
finished task with an empty payload.

<!-- source: internal/component/mcp/tasks.go -- runTaskWorker -->

## Task state machine

```
working -> completed   (dispatch succeeded)
working -> failed      (dispatch returned an error, or the execution deadline expired)
working -> cancelled   (client called tasks/cancel)
```

All three are terminal sinks, and a terminal task never transitions again.

<!-- source: internal/component/mcp/task_state.go -- TaskState, IsTerminal -->

The extension defines a fifth state, `input_required`, for a task whose work
needs more information mid-flight. Ze cannot enter it -- see
[Known limitations](#known-limitations).

## Updating a task

`tasks/update` is the client-to-server direction the extension adds, for
answering a task's outstanding input requests.

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tasks/update",
  "params": { "taskId": "aBc123...", "inputResponses": { } }
}
```

Ze verifies you own the task and acknowledges with an empty result. Any
`inputResponses` keys are ignored: Ze raises no input requests, so there is
nothing for them to satisfy, and the extension requires unknown or
already-satisfied keys to be tolerated rather than rejected.

The `taskId` is not tolerated the same way. It is the one thing the handler acts
on, and an unknown or foreign id is rejected before anything else is read.

<!-- source: internal/component/mcp/streamable_tools.go -- tasksUpdate -->

## Cancellation

```json
{ "jsonrpc": "2.0", "id": 4, "method": "tasks/cancel", "params": { "taskId": "aBc123..." } }
```

Cancellation is cooperative: the worker's context is canceled and the task
transitions to `cancelled`. Canceling a task that already reached a terminal
state is a no-op -- it keeps the state it reached, and its result is not
destroyed.

Ze acknowledges with an empty result, exactly as it does for `tasks/update`. It
deliberately does not report the resulting state: because cancellation is
cooperative, a status read at acknowledgement time can be stale before you parse
it, so `tasks/get` is the call that answers "what state is it in now".

<!-- source: internal/component/mcp/streamable_tools.go -- tasksCancel -->
<!-- source: internal/component/mcp/tasks.go -- taskRegistry.Cancel -->

## Identity scoping

A task belongs to the authenticated principal that created it. `tasks/get`,
`tasks/update` and `tasks/cancel` all reject cross-principal lookups with a
uniform "not found", identical to the answer an id that never existed gets, so
the reply cannot be used to probe for another principal's task ids.

<!-- source: internal/component/mcp/tasks.go -- byIdentity, Create, Get -->

Identity comes from the per-request authenticator, never from a request body
field. Because it is re-derived per request rather than bound to a session, any
later request authenticating as the same principal sees the same tasks.

Under `auth-mode none` every caller is the same anonymous principal, so there is
no isolation to speak of. Use `bearer-list` or `oauth` if isolation matters.

There is no way to enumerate tasks. `tasks/list` was removed in this revision,
so a client tracks the ids it was given.

## Limits

| Limit | Default | Source |
|-------|---------|--------|
| Concurrent tasks per principal | 8 | `TaskRegistryConfig.MaxConcurrent` |
| Retained terminal tasks per principal | 128 | `TaskRegistryConfig.MaxTerminal` |
| Terminal result retention (TTL) | 5 minutes, clamped to 1s..1h | `TaskRegistryConfig.TTL` |
| Worker execution deadline | 10 minutes | `TaskRegistryConfig.ExecDeadline` |

<!-- source: internal/component/mcp/tasks.go -- defaults, activeCount, clampTaskTTL, sweep -->

Exceeding the concurrency cap returns `-32602 task concurrency cap reached`.

These are Go fields on `StreamableConfig.Tasks` that nothing in production
populates; there is no YANG leaf or environment variable for any of them.

### The execution deadline

Every worker runs under a server-side deadline. Past it, the registry forces the
task terminal with a `failed` status and an error naming the deadline, releasing
the concurrency slot it held.

This is a liveness bound, not a retention one, and it is why removing sessions
did not leave a hole. The TTL sweep only deletes tasks that already reached a
terminal state, so it cannot see a worker that never returns; under the earlier
revision the session reaper was the only thing that could force one terminal.
Canceling the worker's context is not sufficient on its own either -- a wedged
dispatch may never observe cancellation -- so the registry transitions the entry
whether or not the goroutine ever comes back.

<!-- source: internal/component/mcp/tasks.go -- sweep, defaultTaskExecDeadline -->

## What changed from `2025-11-25`

| | `2025-11-25` | `2026-07-28` |
|---|---|---|
| Negotiation | `capabilities.tasks = {}` at `initialize`, held as session state | `extensions["io.modelcontextprotocol/tasks"]` on every request |
| Who decides | The client, per call, via `params.task` | The server, per tool, from `ze:task-support` |
| Creation result | `{"taskId":..., "status":...}` | `resultType: "task"` with `ttlMs` and `pollIntervalMs` |
| Fetch result | `tasks/result` (blocking) | Removed. Poll `tasks/get`; terminal states carry `result` or `error` |
| Enumerate | `tasks/list` | Removed |
| Mid-flight input | -- | `tasks/update` (Ze implements it; see below) |
| Scope | Session and identity | Principal only |
| Status push | `notifications/tasks/status` on the GET stream | Removed with the stream; polling is the default |

`tasks/list` and `tasks/result` are now unknown methods: HTTP 404 with `-32601`.

## Known limitations

**`notifications/tasks` is not implemented.** The extension can deliver status
notifications over a `subscriptions/listen` stream, which Ze does not implement.
Polling is the specification's default and is what Ze does.

**The `input_required` state is not implemented.** The extension defines it as a
state a server *may* enter, and Ze cannot enter it. Task eligibility is decided
by the `ze:task-support` annotation, every annotated command dispatches through
a path that never elicits, and the task worker is deliberately handed a zero
capability set so an elicitation degrades to a missing-argument error rather
than an interim result. Implementing the state would be dead code.

`tasks/update` is implemented in full regardless, because for a server that
raises no input requests the extension's own tolerance rule -- verify ownership,
acknowledge empty, ignore unknown keys -- is the complete implementation.

The trigger that brings the state back: a command annotated `ze:task-support
required` gains an elicitation, or a handcrafted tool that elicits becomes
task-eligible. At that point `input_required`, an `inputRequests` payload on
`tasks/get`, and real `inputResponses` matching in `tasks/update` all become
reachable and must be implemented together.

<!-- source: internal/component/mcp/task_state.go -- TaskState, the deliberately absent state -->

**Task limits have no operator surface.** See the table above.
