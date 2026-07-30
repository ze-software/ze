# 1301 -- mcp2026-3-tasks-extension

## Context

Phase 3 of the `2026-07-28` cutover. Tasks were experimental core in
`2025-11-25` and are the `io.modelcontextprotocol/tasks` extension in
`2026-07-28`. Both the negotiation and the method set changed: `tasks/list` and
`tasks/result` are removed, task creation becomes **server-directed** rather than
client-flagged, and the client opts in once through the extension capability
rather than per call via `params.task`.

## Decisions

- **`ze:task-support` survives unchanged in vocabulary and grammar, with inverted
  semantics.** `required` now means the server always returns a task handle,
  `forbidden` that it never does, `optional` that the call is synchronous. The
  annotation always described the command's relationship to task execution rather
  than who initiates it, so the three words still read correctly; only the actor
  moved. Just the YANG `description` changed.
- **`tasks/cancel` acknowledges with an empty result**, matching the extension
  rather than keeping Ze's richer `{taskId, status}`. Beyond conformance:
  cancellation is cooperative, so a status in the acknowledgement is a snapshot
  that can be stale before the client parses it -- `tasks/get` is the fresh
  answer. `tasks/update` already acknowledged empty, so the two are now
  consistent.
- **A server-side execution deadline replaces the session reaper.** `sweep()`
  deletes only *terminal* entries, and `CancelAllForSession` -- the only path
  that used to force a stuck task terminal -- died with sessions in Phase 1.
  Without a deadline a wedged task is immortal.
- **`input_required` is not implemented for tasks.** No Ze task can reach it: task
  workers are given a zero capability set deliberately, so an interim result can
  never surface through `tasks/result`. `tasks/update` is implemented anyway,
  because the extension defines it and a client may call it.

## Consequences

- The advertise/serve contradiction is closed. `server/discover` previously
  returned `"extensions": {}` -- claiming no extension support -- while
  `runMethod` served `tasks/*`. It now advertises
  `io.modelcontextprotocol/tasks`, and no task handle is returned to a client
  that did not declare it.
- Polling is the only way to observe a task. `notifications/tasks` rides
  `subscriptions/listen`, which Ze does not implement, and the extension names
  polling as the default.
- `groupTaskSupport`'s precedence had to change with the semantics: it previously
  had required-wins *and* forbidden-only-if-every-action-forbidden, both of which
  are wrong under inversion. Any `forbidden` action now returns immediately.

## Gotchas

- **An ordering race that only the new polling model could expose.**
  `runTaskWorker` transitioned a task to terminal *before* storing its result.
  Harmless while `tasks/result` was a blocking call that returned the stored map
  directly; under polling it opens a window where a client sees `completed`,
  reads an empty payload, and stops. Reordered. The lesson is that a race can be
  latent behind a synchronous API and become reachable the moment the access
  pattern changes.
- **The bare `tasks` capability member had to go, not just be supplemented.**
  Accepting it still declared task support -- and under server-directed creation
  that pushes an *unsolicited* task handle at a `2025-11-25` client which only
  ever agreed to ask per call. Extension-only now.
- **A deadline that forces an entry terminal must also stop later writes.**
  `storeResult` and `setErrorMsg` had no terminal check, so a task the sweep had
  already failed could still receive its result, and `toWire` would then emit
  `error` **and** `result` on the same entry -- contradicting its own godoc. Both
  writers now carry the check `Transition` always had: first terminal state wins.
- **A test can drive a helper and miss the entry point entirely.** The TTL clamp
  moved into `newTaskRegistry`, but the boundary test still called `clampTaskTTL`
  directly. `retentionHints` re-clamps, so the *wire* value survived deleting the
  constructor call -- while `r.ttl`, the sweep window, would not have. Testing
  the guard from where it fires is the rule (`fail-closed-guards.md`), and this
  is what it looks like when you do not.
- **A claim that `ze:task-support forbidden` was inert was false**, and was
  recorded before being checked. `tools/list` carries it on `ze_clear_bgp` and
  `ze_request_bgp`, and a task-augmented call is refused with
  `-32602 tool ze_clear_bgp does not support task-augmented calls`. Advertisement
  and enforcement cannot diverge because both read the same `g.taskSupport`
  field. The test named for that gate now asserts both directions.

## Files

- `internal/component/mcp/{tasks,task_state,streamable_tools,discover,meta}.go`
- `internal/component/config/yang/modules/ze-extensions.yang` (description only)
- `test/plugin/{task-*,mcp-*}.ci`
- `docs/guide/mcp/tasks.md`, `docs/architecture/api/commands.md`
