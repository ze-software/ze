# Spec: mcp-4-tasks -- Task-Augmented tools/call

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-04-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/spec-mcp-0-umbrella.md` -- umbrella; Phase 4 owns AC-16..AC-21
4. `plan/learned/636-mcp-1-streamable-http.md` -- P1 (session registry, SSE, GET stream)
5. `plan/learned/638-mcp-2-remote-oauth.md` -- P2 (Identity value type, per-session auth)
6. `plan/learned/640-mcp-3-elicitation.md` -- P3 (correlation map pattern, reply sink, input_required integration)
7. `internal/component/mcp/{session,streamable,handler,elicit,reply_sink}.go` -- current impl
8. `docs/architecture/mcp/overview.md` -- transport shape, headers, session lifecycle, roadmap
9. `modelcontextprotocol.io/specification/2025-11-25/basic/utilities/tasks` -- authoritative

## Task

Deliver MCP 2025-11-25 task-augmented `tools/call`: a client may request a
long-running tool invocation run as a task, receive `CreateTaskResult`
immediately, poll status via `tasks/get`, subscribe to status frames on
the GET SSE stream, retrieve the final `CallToolResult` via
`tasks/result`, and cancel via `tasks/cancel`. The same infrastructure
supports the `input_required` sub-state, which plugs into Phase 3
elicitation: a worker that needs input mid-run elicits, the task
transitions to `input_required`, and resumes on the client's response.

Scope: AC-16..AC-21 from the umbrella. Adds the first real consumer of
the GET `/mcp` SSE stream, closing the deferrals from Phase 1
(`deferrals.md` L223-224).

`rules/integration-completeness.md`: at least one existing YANG command
MUST be declared `taskSupport: required` (candidates: `bgp rib dump`,
`rib routes received`, `monitor bgp`) so `make ze-verify-fast`
exercises the full chain end-to-end.

`rules/no-layering.md`: this phase also deletes
`internal/component/mcp/handler.go` (the 2024-11-05 single-POST profile)
and rewrites `tools_test.go` to use the Streamable transport. Production
already uses `NewStreamable`; the older Handler path is only preserved
for a handful of legacy tests.

## Required Reading

### Architecture Docs

- [ ] `docs/architecture/mcp/overview.md` -- Transport Shape, Capability Negotiation, Roadmap
  -> Constraint: task status notifications ride on the GET `/mcp` SSE stream (one per session); POST->SSE upgrade is reserved for elicitation and other per-call server-initiated requests
  -> Decision: task identity scope comes from `session.Identity()` populated in Phase 2; `tasks/*` methods reject cross-identity lookups
- [ ] `docs/architecture/api/commands.md` -- Server-Initiated Methods section (added Phase 3)
  -> Constraint: `notifications/tasks/status` is added to the same section
- [ ] `plan/learned/640-mcp-3-elicitation.md` -- correlation map + reply sink pattern
  -> Decision: rename the session correlation map from `elicit`-specific to a generic "pending" map keyed by `PendingID`; task status subscriptions reuse the infrastructure
  -> Constraint: per-session pending cap (currently 32 for elicits) applies to in-flight tasks too; separate cap per category so a 32-task-cap does not starve elicits
- [ ] `internal/component/mcp/session.go` -- session struct, `CreateWithCapabilities`, `Identity()`, correlation map
  -> Constraint: `session.outbound` remains the SINGLE GET-stream queue; task notifications enqueue via `session.Send(frame)`; no new channel

### MCP Spec Pages (external)

- [ ] `modelcontextprotocol.io/specification/2025-11-25/basic/utilities/tasks`
  -> Constraint: task states: `working`, `input_required`, `completed`, `failed`, `cancelled`; terminal = {completed, failed, cancelled}; `input_required` is non-terminal
  -> Constraint: `tasks/list` / `tasks/get` / `tasks/result` / `tasks/cancel` are the four required methods; `tasks/subscribe` is OPTIONAL
  -> Constraint: `tools/list` tool entries declare `execution.taskSupport`: `required` | `optional` | `forbidden`; `required` means the tool MUST be called as a task
  -> Constraint: `tasks/result` on a non-`completed` task returns a typed error; the result is NOT emptied from the registry until TTL expiry
  -> Constraint: task IDs MUST be cryptographically secure (our session-id RNG: 128 bits base64url)
  -> Constraint: `_meta.io.modelcontextprotocol/related-task` is the correlation key embedded in every message belonging to a task (notifications/tasks/status frames and any mid-task elicitation)
- [ ] `modelcontextprotocol.io/specification/2025-11-25/client/capabilities`
  -> Constraint: `capabilities.tasks = {}` declared by client; server checks `session.ClientSupportsTasks()` before accepting `params.task` on `tools/call`
  -> Constraint: `capabilities.tasks.requests = { "elicitation/create": {} }` (nested) declares "I can answer elicitation while a task is in input_required"; server honors elicit-mid-task only if set

### Ze Rules

- [ ] `.claude/rules/goroutine-lifecycle.md`
  -> Constraint: task workers are per-task (per-lifecycle), bounded by per-identity cap; OK per the rule's "goroutine per lifecycle" allowance
- [ ] `.claude/rules/enum-over-string.md`
  -> Constraint: task state is a typed `uint8` enum with `MarshalText`/`UnmarshalText` (zero-invalid); no string comparisons on hot path
- [ ] `.claude/rules/memory.md`
  -> Constraint: `CallToolResult` stored in the registry is a value type (map/slice); no pointers into session or request structs
- [ ] `.claude/rules/integration-completeness.md`
  -> Constraint: a `.ci` functional test MUST exercise the full POST-task / poll / resume / cancel / result path with a real YANG command declared `taskSupport: required`

**Key insights:**
- Tasks is the first phase where a single MCP session owns N concurrent worker goroutines. The bounding surface is per-identity concurrency cap (default 8) plus a per-session task-result storage cap (default 128 terminal tasks retained for TTL).
- `input_required` + `elicitation/create` unify cleanly: when a task worker calls `session.Elicit`, it first transitions the task to `input_required`, emits `notifications/tasks/status`, then suspends on the correlation channel from Phase 3. On reply, the task flips back to `working` and the worker resumes.
- `execution.taskSupport` is derived from YANG `ze:command` metadata (new extension leaf `ze:task-support`), so tools auto-generate with the correct value and no hardcoded tool-name list drifts.
- The GET SSE stream finally has a production consumer: `notifications/tasks/status` frames. The Phase 1 deferrals around GET-stream `.ci` testing become actionable here.

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE finalizing this spec)

- [ ] `internal/component/mcp/session.go` (~550 L) -- session + sessionRegistry; correlation map is `map[string]chan elicitResponse`
- [ ] `internal/component/mcp/elicit.go` -- Elicit suspends on correlation channel; `RegisterElicit`/`ResolveElicit`/`CancelElicit` primitives
- [ ] `internal/component/mcp/streamable.go` -- `handlePOST` dispatches methods; `handleGET` writes SSE frames from `session.outbound`
- [ ] `internal/component/mcp/handler.go` -- `server.dispatch(cmd)` is the sole entry into the reactor; legacy 2024-11-05 path still present
- [ ] `internal/component/mcp/tools.go` -- auto-generates tool descriptors from `CommandLister`; groups by command prefix; emits JSON Schema from YANG param types
- [ ] `internal/component/config/yang/modules/ze-extensions.yang` -- `ze:command` extension; `ze:task-support` leaf will land here

**Behavior to preserve:**
- Auto-generated tools derived at each `tools/list` call (no cache)
- `ze_execute` handcrafted tool remains; it advertises `taskSupport: optional` (can run sync or as a task)
- Elicitation correlation map API shape (callers are Phase 3 code)
- `session.outbound` queue size and backpressure semantics (non-blocking `Send`)
- `Identity` value type; task scoping leans on `Identity.Name`

**Behavior to change:**
- Session correlation map generalises from `map[string]chan elicitResponse` to `map[PendingID]pendingEntry` with typed kind (elicit | task-status). Implementation details hidden behind `RegisterPending`/`ResolvePending`/`CancelPending`; existing `RegisterElicit` etc become thin wrappers.
- `tools/list` output grows an `execution.taskSupport` field per tool, derived from YANG `ze:task-support`.
- Legacy `handler.go` 2024-11-05 Handler deleted; `tools_test.go` migrates to Streamable (umbrella deferral).

## Data Flow (MANDATORY)

### Entry Points

- `POST /mcp` with `tools/call` and `params.task = {}` -> CreateTaskResult path
- `POST /mcp` with `tasks/list` | `tasks/get` | `tasks/result` | `tasks/cancel` -> registry lookup
- GET `/mcp` with `Accept: text/event-stream` -> subscribe to session's notification stream (tasks/status + future notifications)

### Transformation Path

1. POST enters `handlePOST`. After session resolve + auth check, the method switch routes `tools/call` with `params.task != nil` into `Streamable.createTask(ctx, req, sess)`.
2. `createTask` validates the tool supports tasks (reject if `taskSupport: forbidden`), generates a task ID, inserts into the task registry, launches a worker goroutine, returns `CreateTaskResult{taskId, status: "working"}` on the POST.
3. Worker goroutine calls `s.dispatch(cmd)` under a task-scoped ctx. On return it transitions the task to `completed` or `failed`, stores the `CallToolResult` in the registry entry, and emits `notifications/tasks/status` to the session.
4. On `tasks/cancel`: registry marks cancellation flag; worker's ctx cancels; worker transitions task to `cancelled`; status notification emitted.
5. On `tasks/get`: registry lookup by (identity, taskId); returns current state + metadata. Unknown id -> typed error with `code=-32602, message=task not found` (no leak of other identities' ids).
6. On `tasks/result`: only returns the stored `CallToolResult` when status is terminal; `-32602 task not terminal` when still working / input_required.
7. Mid-task elicit: worker calls `session.Elicit(ctx, msg, schema)`. Task transitions to `input_required` BEFORE the elicit frame is emitted; status notification fires; on elicit reply, task transitions back to `working`; on decline/cancel/ctx-cancel, worker chooses fallback (transition to `failed`/`cancelled`).

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| HTTP POST `tools/call` -> task registry | `createTask` inserts entry + launches worker | [ ] |
| Task registry -> worker goroutine | Shared ctx + result channel; registry holds the handle | [ ] |
| Worker -> session notification stream | `session.Send(statusFrame)` (non-blocking) | [ ] |
| Worker <-> elicitation correlation | `session.Elicit` suspends on the existing correlation channel; task state machine flips around it | [ ] |
| Session close -> task GC | `session.close()` cancels all tasks' ctx and drains the registry for that session | [ ] |
| Client POST `tasks/*` -> identity scope | `tasks/list` and `tasks/get` reject tasks whose identity differs | [ ] |

### Integration Points

- `CommandLister` + YANG `ze:task-support` extension -- drives `tools/list` enrichment
- `session.Identity()` -- task ownership
- `session.Elicit` -- called from worker without modification
- `session.Send` -- task status notifications ride the existing outbound queue

### Architectural Verification

- [ ] No bypassed layers: task POST goes through session + auth + identity scope
- [ ] No unintended coupling: task registry lives at `internal/component/mcp/tasks.go`; does NOT import plugin code
- [ ] No duplicated functionality: reuses the correlation-map primitive from Phase 3, not a parallel implementation
- [ ] No allocation on hot paths beyond necessary registry book-keeping; status frames built into pooled buffers where practical (JSON marshal is one-shot and OK)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| POST `tools/call` with `params.task={}` on a YANG command declared `taskSupport: required` (e.g., `bgp rib dump`) | -> | `handlePOST` -> `createTask` -> worker -> `tasks/result` returns the real output | `test/plugin/task-rib-dump-accept.ci` |
| POST `tasks/cancel` for a `working` task | -> | Registry marks; worker ctx cancels; `notifications/tasks/status` with `cancelled` fires on GET stream | `test/plugin/task-cancel.ci` |
| POST `tools/call` with `params.task={}` on a tool with `taskSupport: forbidden` | -> | `-32602` typed error; no task created | `test/plugin/task-forbidden.ci` |
| Worker mid-task calls `session.Elicit`; client answers | -> | Task flips `working` -> `input_required` -> `working` -> `completed`; two status notifications observed | `test/plugin/task-input-required.ci` |
| Task exceeds TTL | -> | `tasks/get` returns `-32602 expired`; registry entry gone | `test/plugin/task-ttl-expiry.ci` |
| Identity A's `tasks/list` does not see identity B's tasks | -> | Two parallel sessions, each runs a task, each `tasks/list` returns only its own | `test/plugin/task-identity-scope.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-16 | Client posts `tools/call` with `task: {"ttl": 60000}` against a task-capable tool | Server returns `CreateTaskResult` with `status: working` on the POST; task registered; worker goroutine launched |
| AC-16a | Client posts `tools/call` with `task: {}` against a tool whose `taskSupport: forbidden` | `-32602` typed error; task NOT created; no worker launched |
| AC-16b | Client posts `tools/call` without `task` against a tool whose `taskSupport: required` | `-32602` typed error naming the tool as task-required |
| AC-17 | Client posts `tasks/get` for a `working` task | Current state returned (`working` | `input_required`); metadata includes `createdAt`, `lastUpdateAt`, `identity` |
| AC-18 | Client posts `tasks/result` for a `completed` task | Returns the stored `CallToolResult` (same shape as unbundled `tools/call`) |
| AC-18a | Client posts `tasks/result` for a non-terminal task | `-32602 task not terminal` |
| AC-19 | Client posts `tasks/cancel` for a `working` task | Status transitions to `cancelled`; worker ctx signalled; a `notifications/tasks/status` frame with `cancelled` appears on the session's GET stream |
| AC-19a | Client posts `tasks/cancel` for a terminal task | Idempotent no-op; current state echoed back |
| AC-20 | Task exceeds configured TTL (default 300 s after terminal) | Server GC removes entry; subsequent `tasks/get` returns `-32602 expired` |
| AC-21 | `tasks/list` under auth context A does not return tasks owned by context B | List scoped strictly to caller identity; cross-identity `tasks/get` also returns `-32602 task not found` |
| AC-21a | Worker goroutine calls `session.Elicit` mid-run | Task transitions `working` -> `input_required` BEFORE the elicit frame; `notifications/tasks/status` fires; on accept, transitions back to `working` then `completed`; on decline/cancel, task `failed` or `cancelled` |
| AC-21b | Session closes (DELETE or TTL) while tasks are running | All in-flight tasks for that session are cancelled; workers' ctx signalled; terminal `cancelled` notifications skipped (client is gone) |
| AC-21c | Two concurrent tasks on the same session up to per-identity cap (default 8) | Both run; `tasks/list` shows both; the 9th concurrent create returns `-32602 task concurrency cap reached` |

## TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| `TestTaskRegistry_CreateGetCancel` | `internal/component/mcp/tasks_test.go` | Basic state machine: working -> cancelled |
| `TestTaskRegistry_TTLExpiry` | same | AC-20 GC removes terminal entries after TTL |
| `TestTaskRegistry_IdentityScope` | same | AC-21 cross-identity lookups are invisible |
| `TestTaskRegistry_ConcurrencyCap` | same | AC-21c per-identity cap rejects over-limit |
| `TestTaskState_StringerAndMarshal` | `internal/component/mcp/task_state_test.go` | Typed enum + MarshalText/UnmarshalText roundtrip; zero value is invalid |
| `TestTaskWorker_DispatchCompletesAndStores` | `internal/component/mcp/tasks_test.go` | Worker runs dispatcher, stores CallToolResult, transitions completed |
| `TestTaskWorker_DispatchErrorFails` | same | Dispatcher error -> task failed with error message retained |
| `TestTaskWorker_CtxCancelTransitions` | same | Cancel during dispatch -> cancelled state, worker exits |
| `TestTaskWorker_ElicitFlipsInputRequired` | `internal/component/mcp/task_elicit_test.go` | AC-21a state transitions around Elicit |
| `TestStreamable_ToolsCallWithTaskParam` | `internal/component/mcp/streamable_test.go` | AC-16 CreateTaskResult returned synchronously |
| `TestStreamable_ToolsCallForbiddenRejected` | same | AC-16a forbidden tool + task param |
| `TestStreamable_ToolsCallRequiredWithoutTaskRejected` | same | AC-16b required tool without task param |
| `TestStreamable_TasksGetWorking` | same | AC-17 |
| `TestStreamable_TasksResultTerminalOnly` | same | AC-18 + AC-18a |
| `TestStreamable_TasksCancelWorking` | same | AC-19 |
| `TestStreamable_TasksCancelTerminalNoop` | same | AC-19a idempotence |
| `TestTaskNotifications_StatusFrameShape` | `internal/component/mcp/tasks_test.go` | Frame method is `notifications/tasks/status`; `_meta.io.modelcontextprotocol/related-task` carries taskId |
| `TestToolDescriptor_TaskSupportField` | `internal/component/mcp/tools_test.go` | `execution.taskSupport` derived from YANG `ze:task-support` extension; default `optional` |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| task TTL (ms, terminal retention) | 1 000 -- 3 600 000 (1 sec -- 1 hour) | 3 600 000 | 999 | 3 600 001 |
| per-identity concurrent tasks | 1 -- 32 (configurable via YANG `mcp.task.max-concurrent`) | 32 | 0 | 33 |
| per-session stored terminal tasks | 1 -- 1024 | 1024 | 0 | 1025 |
| task ID length | 22 base64url chars (same as session id) | 22 | 21 | 23 |

### Functional Tests

| Test | Location | End-User Scenario |
|------|----------|-------------------|
| `task-rib-dump-accept` | `test/plugin/task-rib-dump-accept.ci` | Big RIB dump as a task; client polls until completed; result matches sync equivalent |
| `task-cancel` | `test/plugin/task-cancel.ci` | Task cancelled mid-run; status notification observed on GET stream |
| `task-forbidden` | `test/plugin/task-forbidden.ci` | `taskSupport: forbidden` + `task:{}` rejected |
| `task-input-required` | `test/plugin/task-input-required.ci` | Mid-task elicit; client accepts; task completes |
| `task-ttl-expiry` | `test/plugin/task-ttl-expiry.ci` | Tight TTL; GC sweeps; `tasks/get` returns `expired` |
| `task-identity-scope` | `test/plugin/task-identity-scope.ci` | Two auth-mode=bearer-list identities; each sees only its own tasks |

## Files to Modify

- `internal/component/mcp/session.go` -- rename correlation map to `pending`; `RegisterPending`/`ResolvePending`/`CancelPending`; existing `*Elicit` methods become thin wrappers. Add `clientTasks bool` capability bit + `ClientSupportsTasks`.
- `internal/component/mcp/streamable.go` -- add method dispatch for `tasks/list|get|result|cancel`; extend `callTool` to branch on `params.task != nil` into `createTask`; `doInitialize` parses `capabilities.tasks`.
- `internal/component/mcp/handler.go` -- DELETE (legacy 2024-11-05 profile). Callers migrate to `NewStreamable`.
- `internal/component/mcp/tools.go` -- enrich tool descriptors with `execution.taskSupport` derived from YANG `ze:task-support` extension
- `internal/component/mcp/tools_test.go` -- migrate off legacy Handler (uses Streamable)
- `internal/component/config/yang/modules/ze-extensions.yang` -- new `ze:task-support` extension on `ze:command`
- `cmd/ze/*/yang_command.go` (or wherever YANG commands register) -- tag long-running commands with `ze:task-support required`
- `docs/architecture/mcp/overview.md` -- new Task section; updated Roadmap row; deprecate legacy Handler row
- `docs/architecture/api/commands.md` -- add `tasks/list|get|result|cancel` + `notifications/tasks/status` rows
- `docs/guide/mcp/overview.md` -- client-facing task capability advert
- `docs/functional-tests.md` -- `task-*.ci` scenarios

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new extension + per-command tagging) | Yes | `ze-extensions.yang` + existing `ze-*-conf.yang` files for long-running commands |
| Env vars | Yes -- `ze.mcp.task.ttl`, `ze.mcp.task.max-concurrent`, `ze.mcp.task.store-cap` | `internal/component/mcp/env.go` |
| CLI commands/flags | No -- feature is MCP-protocol only | -- |
| Editor autocomplete | Yes -- YANG-driven | automatic once YANG updated |
| Functional test per capability | Yes -- 6 `.ci` scenarios above | `test/plugin/task-*.ci` |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- MCP tasks (2025-11-25) row |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` -- `mcp.task.*` config leaves |
| 3 | CLI command added/changed? | No | -- |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` -- MCP server-initiated methods + tasks/* section |
| 5 | Plugin added/changed? | No | -- |
| 6 | Has a user guide page? | Yes | `docs/guide/mcp/tasks.md` (new) |
| 7 | Wire format changed? | No (MCP is not BGP wire) | -- |
| 8 | Plugin SDK/protocol changed? | No | -- |
| 9 | RFC behavior implemented? | No -- MCP spec is the reference | -- |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` -- `task-*.ci` |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` -- MCP tasks row (ze-only differentiator) |
| 12 | Internal architecture changed? | Yes | `docs/architecture/mcp/overview.md` -- Task section, roadmap |

## Files to Create

- `internal/component/mcp/tasks.go` -- task registry, state machine, TTL GC, worker orchestration
- `internal/component/mcp/task_state.go` -- typed `TaskState uint8` enum + `MarshalText`/`UnmarshalText`
- `internal/component/mcp/tasks_test.go` -- unit tests listed above
- `internal/component/mcp/task_elicit_test.go` -- mid-task elicit integration tests
- `internal/component/mcp/env.go` -- `env.MustRegister` for task-related env vars (if not already present)
- `test/plugin/task-rib-dump-accept.ci`, `task-cancel.ci`, `task-forbidden.ci`, `task-input-required.ci`, `task-ttl-expiry.ci`, `task-identity-scope.ci`
- `docs/guide/mcp/tasks.md` -- user guide
- `rfc/short/rfcNNNN-mcp-tasks.md` -- not a true RFC; use `docs/mcp/2025-11-25-tasks.md` as the "short reference" document instead

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation Phases below |
| 4. /ze-review gate | Review Gate |
| 5. Full verification | `make ze-verify-fast` |
| 6. Critical review | Critical Review Checklist |
| 7. Fix issues | Per finding |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Max 2 review passes |
| 10. Deliverables review | Deliverables Checklist |
| 11. Security review | Security Review Checklist |
| 12. Re-verify | Re-run stage 5 |
| 13. Present summary | Executive Summary Report |

### Implementation Phases

1. **Phase 1 -- Typed TaskState + Registry primitives**
   - Tests: `TestTaskState_*`, `TestTaskRegistry_CreateGetCancel`, `TestTaskRegistry_TTLExpiry`, `TestTaskRegistry_IdentityScope`, `TestTaskRegistry_ConcurrencyCap`
   - Files: `task_state.go`, `tasks.go` (registry + state machine, no worker yet)
   - Verify: tests fail -> implement -> tests pass
2. **Phase 2 -- Worker goroutine + dispatcher integration**
   - Tests: `TestTaskWorker_*`, `TestTaskNotifications_StatusFrameShape`
   - Files: `tasks.go` (worker loop, status emission), extends `session.Send` usage
   - Verify: worker drives dispatch; cancellation propagates via ctx
3. **Phase 3 -- Session correlation-map generalisation + capability bit**
   - Tests: ensure existing elicit tests still pass under the renamed API; add `TestRegistry_CreateWithTasksCapability`
   - Files: `session.go` (rename `correlations` to `pending`; kind-tag; back-compat wrappers)
   - Verify: no regression in Phase 3 elicit tests
4. **Phase 4 -- HTTP dispatch: `createTask`, `tasks/list|get|result|cancel`**
   - Tests: `TestStreamable_*`
   - Files: `streamable.go`
   - Verify: unit tests pass; manual `curl` smoke against `bin/ze --mcp`
5. **Phase 5 -- `tools/list` enrichment + YANG extension**
   - Tests: `TestToolDescriptor_TaskSupportField`
   - Files: `ze-extensions.yang`, `tools.go`; tag long-running commands
   - Verify: `make ze-inventory` or manual JSON inspection shows `execution.taskSupport` per tool
6. **Phase 6 -- Mid-task elicit integration**
   - Tests: `TestTaskWorker_ElicitFlipsInputRequired`, `task-input-required.ci`
   - Files: `tasks.go` adds pre-/post-state hooks around `session.Elicit`
   - Verify: input_required transition visible in tests; elicit reply flips back to working
7. **Phase 7 -- Delete legacy Handler + migrate tools_test.go**
   - Tests: rewrite ~26 HTTP tests using Streamable
   - Files: DELETE `handler.go`; rewrite `tools_test.go`
   - Verify: no test references Handler(); `make ze-unit-test` green
8. **Phase 8 -- Functional `.ci` tests**
   - Files: `test/plugin/task-*.ci`
   - Verify: `bin/ze-test plugin -p task` all pass
9. **Phase 9 -- Docs**
   - Per Documentation Update Checklist
10. **Phase 10 -- Full verify + learned summary + two-commit sequence per spec-preservation**

### Critical Review Checklist

| Check | What to verify |
|-------|---------------|
| Completeness | Every AC has a named test + file:line |
| Correctness | State-machine transitions match spec: only `working`/`input_required` can go to any of the 3 terminal states; terminal -> terminal is invalid (cancel a completed task is a no-op, not a transition) |
| Naming | MCP dialect keeps camelCase: `taskId`, `taskSupport`, `createTaskResult` (external spec); Ze-internal types use Go camelcase |
| Data flow | Task status notifications ride `session.outbound` (GET stream); no new channel; no POST upgrade for task status |
| Rule: no-layering | Legacy `handler.go` DELETED, not kept alongside Streamable |
| Rule: enum-over-string | `TaskState` is typed `uint8`; no string compares on hot path |
| Rule: goroutine-lifecycle | Workers are per-task (per-lifecycle), bounded by per-identity cap |
| Rule: exact-or-reject | `taskSupport: forbidden` + task param -> reject at verify time; `taskSupport: required` without task param -> reject |
| Rule: derive-not-hardcode | `taskSupport` derived from YANG extension; no hardcoded tool-name lists |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Task registry exists | `ls internal/component/mcp/tasks.go` |
| State enum typed | `grep 'type TaskState uint8' internal/component/mcp/task_state.go` |
| Legacy Handler deleted | `! test -f internal/component/mcp/handler.go` |
| At least one YANG command tagged `task-support: required` | `grep 'task-support required' internal/component/config/**/*.yang` |
| `execution.taskSupport` in tools/list | Run `ze-test mcp --port N` + `@tools/list`, confirm field present |
| `.ci` tests pass | `bin/ze-test plugin -p task` |
| `make ze-verify-fast` passes | tmp/ze-verify.log shows PASS |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | `params.task.ttl` bounded; task ID format validated on every `tasks/*` method; method dispatch rejects unknown actions |
| Identity scope | `tasks/list`/`get`/`result`/`cancel` reject cross-identity ids; error message DOES NOT leak whether the id exists for another identity (returns `not found` uniformly) |
| Task ID unpredictability | 128-bit crypto/rand base64url (same RNG as session id) |
| Resource exhaustion | Per-identity concurrency cap; per-session stored-terminal cap; worker goroutines bounded; task result body size capped via `session.maxBody` |
| Cancellation leaks | Worker ctx MUST be cancelled on `tasks/cancel` AND on session close; verify no worker goroutine outlives its session |
| Log hygiene | Task input (the tool args) may contain secrets; log at debug with redacted args; never log full tool results |
| Result storage | Terminal results are held in RAM only; lost on daemon restart (documented); no on-disk persistence |
| DoS via long-running tools | `taskSupport: required` tools MUST honor ctx; a tool ignoring ctx effectively disables cancellation -- audit each one that gets tagged |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Phase that introduced it |
| State machine transition wrong | Re-read `modelcontextprotocol.io/specification/2025-11-25/basic/utilities/tasks` |
| Worker goroutine leaks (go test -race) | Back to Phase 2; verify every path cancels ctx |
| Legacy Handler tests fail after migration | Re-check Streamable test setup helpers |
| Mid-task elicit state bounces | Back to Phase 6; review ordering of state transition vs frame emission |
| 3 fix attempts fail | STOP. Report all 3. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Per-session task store only | `tasks/list` is scoped to identity across sessions per spec; per-session would fragment one operator's view | Identity-scoped store with per-identity caps |
| Polling-only (no GET stream notifications) | Clients that want reactive UIs would have to hot-loop; GET stream exists unused | Status notifications on GET stream; polling remains the fallback |
| Store task results on disk | Adds persistence concerns (cleanup, crash recovery, identity binding across restart) well out of scope | In-RAM with TTL; document that daemon restart clears terminal tasks |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- **Tasks are the first multi-goroutine-per-session surface in ze.** Session registry has always been per-session single-threaded (goroutine per SSE reader, handlers run per-POST). Tasks introduce N worker goroutines per session, bounded by per-identity cap. Memory lifecycle tracing (see `rules/before-writing-code.md`): each worker owns its task's buffer; registry holds the terminal `CallToolResult` as a value copy; session close cancels all workers before draining outbound.
- **`session.outbound` as the GET-stream queue finally gets a real consumer.** Phase 1 built the pipe; Phase 3 didn't use it (elicit rides the POST); Phase 4 does. Flow control: `session.Send` is non-blocking and returns `errSessionQueueFull` on backpressure; the worker treats queue-full on a status frame as a soft fail -- drops the frame, logs at debug, the client can still poll `tasks/get` for state. Terminal notifications are NOT dropped silently; a full queue on a terminal frame escalates to a diagnostic.
- **`tasks/list` pagination is deferred** to a later spec. Current design returns all tasks for the identity (bounded by the cap). If the cap grows beyond ~100, pagination becomes necessary.
- **YANG `ze:task-support` extension is the canonical source** for `execution.taskSupport`. Alternatives considered:
  - Per-tool Go constant table -- rejected (`rules/derive-not-hardcode.md`: two hardcoded lists will drift).
  - Client-side flag -- rejected (spec requires server advertisement at `tools/list`).
- **Legacy Handler deletion is scoped here** rather than spread across phases. Phase 1 deferred it because `tools_test.go` used it; this phase does the migration as a natural part of the test rewrite.
- **Task state enum zero-value:** `TaskUnspecified = 0`. Every newly-created task sets `TaskWorking = 1` before insertion into the registry; a read that sees `TaskUnspecified` indicates corruption.
- **`notifications/tasks/status` carries `_meta.io.modelcontextprotocol/related-task`** per spec. The `_meta.*` prefix is reserved for implementation-defined metadata; this is its first use in ze.

## RFC Documentation

MCP 2025-11-25 is the authoritative spec, not an IETF RFC. Inline comments at state-machine transitions, `createTask`, and the notification sender cite:

- `modelcontextprotocol.io/specification/2025-11-25/basic/utilities/tasks` -- state machine, method contracts, metadata keys
- `modelcontextprotocol.io/specification/2025-11-25/client/capabilities` -- capability declarations

## Implementation Summary

_Filled after implementation per `/implement` stage 13._

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates
- [ ] AC-16..AC-21c all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name
- [ ] `/ze-review` gate clean
- [ ] `make ze-verify-fast` passes
- [ ] `make ze-test` passes
- [ ] Legacy `handler.go` deleted; no `Handler()` references in tests
- [ ] At least one YANG command tagged `task-support: required`; end-to-end `.ci` proves the chain

### Quality Gates
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (registry primitives kept concrete, not turned into a general work-queue)
- [ ] No speculative features (pagination deferred; persistence deferred)
- [ ] Single responsibility per new file (tasks.go owns registry + worker; task_state.go owns enum; task_elicit_test.go owns integration tests)
- [ ] Minimal coupling (no plugin/component imports; session registry unchanged except for the rename)

### TDD
- [ ] Tests written before code in each phase
- [ ] Tests FAIL (paste output per phase)
- [ ] Tests PASS (paste output per phase)
- [ ] Boundary tests for TTL, concurrency cap, stored-terminal cap, task id length
- [ ] Functional `.ci` tests for all 6 wiring rows

### Completion
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Learned summary written to `plan/learned/NNN-mcp-4-tasks.md`
