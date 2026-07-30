# Spec: mcp2026-3-tasks-extension

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | spec-mcp2026-2-mrtr |
| Phase | 3/4 |
| Deferral shard | `plan/deferrals/mcp2026-3-tasks-extension.md` |
| Updated | 2026-07-30 |

Parent: `plan/spec-mcp2026-0-umbrella.md`.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Move Ze's task support out of the core protocol and onto the official
`io.modelcontextprotocol/tasks` extension, in its redesigned stateless shape.

Tasks were experimental core in `2025-11-25` and are an extension in
`2026-07-28`. Both the negotiation and the method set changed.

**Changes from Ze's current model:**

| Aspect | Ze today | `2026-07-28` extension |
|--------|----------|------------------------|
| Negotiation | `capabilities.tasks = {}` at initialize, stored as a session bit (`session.go:56`, `parseTasksCapability` `streamable.go:740-754`) | `extensions["io.modelcontextprotocol/tasks"]` inside per-request `clientCapabilities`; server advertises the same in `server/discover` capabilities |
| Who decides a call is a task | The **client**, per call, via `params.task` (`streamable_tools.go:78-98`), constrained by the `ze:task-support` YANG extension (required / optional / forbidden) | The **server**, per request, from the same `ze:task-support` annotation read with inverted semantics (D-1). The client opts in once via the extension capability and handles whichever result shape arrives |
| Creation result | `{"taskId":..., "status":...}` (`streamable_tools.go:296-299`) | `CreateTaskResult` with `resultType: "task"`, carrying `taskId`, status, `ttlMs`, `pollIntervalMs` |
| Fetch result | `tasks/result`, a blocking call (`streamable_tools.go:203-216`) | **Removed.** Poll `tasks/get`; terminal states carry `result` or `error` |
| Enumerate | `tasks/list` (`streamable_tools.go:175-186`) | **Removed** (unsafe without sessions) |
| Mid-flight input | `TaskElicit` (`tasks.go:456-513`) transitions to `input_required` and elicits over the session SSE stream. It has **no production caller**: the only `Elicit` call site in the tree is `tools.go:747`, inside the handcrafted `ze_execute` handler (`tools.go:722`) | The extension defines `input_required` plus `tasks/update`. Ze implements `tasks/update` and does **not** implement the `input_required` state, because no Ze task can reach it (D-4) |
| Scoping | `byIdentity` **and** `sessionID` (`tasks.go:52`, `:120`), with `CancelAllForSession` (`tasks.go:277`) wired to session expiry at `streamable.go:182-184` | Principal only. Sessions no longer exist, so the forced-terminal path they provided is replaced by an execution deadline (D-3) |
| Status push | `notifications/tasks/status` on the session GET stream (`tasks.go:520`) | `notifications/tasks` via `subscriptions/listen`, which Ze does not implement (umbrella A-4). Polling is the spec default and is what Ze does |

**Delete:**
- `tasks/list` (`streamable_tools.go:175-186`) and `tasks/result` (`:203-216`) handlers, and their registry methods `List` (`tasks.go:198`) and `Result` (`tasks.go:155`)
- `sessionID` on `taskEntry` (`tasks.go:35`), `CancelAllForSession` (`tasks.go:277`), and the `sessReg.onExpire` wire (`streamable.go:182-184`), to whatever extent Phase 1 has not already removed them
- The `notifications/tasks/status` push path (`buildTaskStatusNotification`, `tasks.go:517`) and `TaskElicit` (`tasks.go:456-513`). Their transport dies in Phase 1, and `TaskElicit` is an exported symbol with no production caller today, so it is dead either way (`ai/rules/wiring-completeness.md`)
- Client-directed task opt-in via `params.task` (`streamable_tools.go:78-98`), the `Task` field on `callParams` (`tools.go:716`), and with them the client-requested TTL branch (`streamable_tools.go:257-265`)
- `TaskInputRequired` (`task_state.go:17`) and its wire name (`task_state.go:30`): unreachable, per D-4

**Add:**
- `resultType: "task"` `CreateTaskResult` carrying `ttlMs` and `pollIntervalMs`
- `tasks/update`: ownership check by principal, then an empty acknowledgement, ignoring unknown or already-satisfied `inputResponses` keys
- Extension advertisement in `server/discover` capabilities
- Per-request extension-capability check before returning any task handle: never return a task to a client that did not declare support
- A server-side execution deadline on every task worker, replacing the session reaper as the only path that forces a stuck task terminal (D-3)

**Design questions, resolved.** These were the four open questions this spec was
created to answer. Each is now a recorded decision. `Q-N` is the question;
`D-N` is the decision row in Key Design Decisions that carries the reasoning and
the rejected alternatives.

| ID | Question | Resolution |
|----|----------|------------|
| Q-1 | What becomes of `ze:task-support`? | **D-1.** It survives unchanged in vocabulary and grammar, with **inverted semantics**: `required` now means the server always returns a task handle, `forbidden` that it never does, `optional` (the default) that the call is synchronous. The annotation always described the command's relationship to task execution, not who asks, so the three words still read correctly. Only the `description` text in `ze-extensions.yang:120-126` changes, which is why this is not the YANG surface change the question feared. Keeping `forbidden` is what mitigates R-1 |
| Q-2 | What is the server's rule for deciding a call is long-running? | **D-1, D-1b.** The annotation, and nothing else. The annotated set is small and closed: 9 commands carry `required` and 4 carry `forbidden` across 9 YANG modules (A-2, confirmed). No wall-clock promotion, no heuristic. The rule is evaluated per tool, not per command, because `lookupTaskSupport` resolves a tool name to a command **group** (`streamable_tools.go:117-135`), which is why D-1b has to fix that group's precedence |
| Q-3 | What bounds the task registry now that sessions are gone? | **D-3.** The TTL GC is already independent of sessions: `runGC` is launched unconditionally by `newTaskRegistry` (`tasks.go:91`) and ticks on `taskGCInterval` (`tasks.go:347`), stopping only at `Close` (`tasks.go:301-304`). It is not enough on its own, because `sweep` deletes only **terminal** entries (`tasks.go:366-367`) and `CancelAllForSession` was the only path that forced a non-terminal task terminal. So this phase adds a per-task execution deadline. `byIdentity` self-prunes when a principal's last task is swept (`tasks.go:372-377`), so the index cannot outlive its tasks |
| Q-4 | Does the task TTL interact with the Phase 2 `requestState` TTL? | **No, and they must not be unified.** They are different mechanisms: `requestState` is a client-carried continuation token bound to principal, request and expiry (`plan/spec-mcp2026-2-mrtr.md:189-195`), while the task TTL is a server-side **retention** timer for an already-terminal result (`tasks.go:366-367`). Unifying them would give a retention timer authorization semantics. In this phase they cannot even meet: no Ze task can reach `input_required` (A-4, D-4), so no task ever mints or consumes a `requestState`. The question is closed, not deferred |

Umbrella AC covered: AC-7.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/mcp/tasks.md` - the user-facing model being rewritten
  → Constraint: every example on the page shows the client sending `task: {}`, which D-1 deletes. The page is rewritten, not patched.
- [ ] `docs/architecture/mcp/overview.md` - tasks section
  → Constraint: it is the `// Design:` target of `tasks.go:1` and `task_state.go:1`, so a structural change here forces a rewrite of that section in the same work (`ai/rules/design-doc-references.md`).
- [ ] `plan/learned/681-mcp-4-tasks.md` - why the current design is shaped as it is
  → Decision: the session coupling recorded there was a consequence of `2025-11-25` sessions, not a Ze design preference, so removing it is a return to the intended shape rather than a regression.

### Protocol Specification (Scope: protocol)
- [ ] `https://modelcontextprotocol.io/extensions/tasks/overview` - lifecycle, methods, server implementation steps
  → Constraint: the server decides; `tasks/list` and `tasks/result` do not exist; terminal states carry `result` or `error` on `tasks/get`.
  → Constraint: `tasks/update` tolerates unknown and already-satisfied keys and acknowledges with an empty result. That tolerance rule is what makes the method fully implementable for a server that raises no input requests (D-4).
- [ ] `https://github.com/modelcontextprotocol/ext-tasks` - the normative extension specification
  → Constraint: `input_required` is a state the server **may** enter, not one it must produce. A server that never elicits inside a task is conformant without it.
- [ ] `https://modelcontextprotocol.io/specification/2026-07-28/basic/versioning#extension-negotiation` - the `extensions` capability map and the fallback rule
  → Decision: a client that does not declare the extension still gets its answer, synchronously. The extension is an optimization over a synchronous call, never a precondition for the work (D-2, AC-6).

**Key insights:** (minimal context to resume after compaction)
- The whole phase turns on one small closed annotation set: 9 `required` and 4 `forbidden` `ze:task-support` statements across 9 YANG modules. Nothing else in the tree decides what a task is.
- The `input_required` half of the extension is unreachable in Ze. The only elicitation call site is `tools.go:747` inside `ze_execute`, which is handcrafted and therefore resolves to `TaskSupportOptional` (`streamable_tools.go:117-124`), which D-1 maps to synchronous. `TaskElicit` (`tasks.go:464`), the one function that could put a task into `input_required`, has no production caller at all.
- Deleting the session reaper removes the only path that forced a stuck task terminal. The TTL sweep only reaps tasks that already reached a terminal state, so without a new execution deadline a hung dispatch would hold a concurrency slot forever.
- MCP functional tests live in `test/plugin/`, not `test/mcp/`. There is no `mcp` suite: `bgpCIRunnerDirs` (`internal/test/cli/cmd_bgp.go:39-47`) is the closed registered set, and `TestCIRootsRegistered` (`internal/test/cli/register_test.go:66`) fails on any `test/` directory holding `.ci` files that no runner claims.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/mcp/tasks.go` (530L) - registry, workers, GC, session coupling. Caps and timers at `:15-21` (`defaultMaxConcurrentTasks` 8, `defaultMaxTerminalTasks` 128, `defaultTaskTTL` 5m, `minTaskTTL` 1s, `maxTaskTTL` 1h, `taskGCInterval` 30s). `Create` (`:98`) caps per identity via `activeCount` (`:311`) and hands the worker a `context.WithCancel(context.Background())` with **no deadline** (`:115`). `sweep` (`:359`) deletes only terminal entries and self-prunes `byIdentity` (`:372-377`). `TaskElicit` (`:464`) is the only producer of `TaskInputRequired` and has no production caller.
- [ ] `internal/component/mcp/task_state.go` (79L) - `TaskState` enum (`:14-21`), wire names (`:28-34`), `IsTerminal` (`:60`). `TaskInputRequired` is `:17`, wire `:30`.
- [ ] `internal/component/mcp/streamable_tools.go` (368L) - `runMethod` switch (`:22-45`) carrying four `tasks/*` methods; `callTool` (`:71`) with the client-directed branch at `:89-98`; `lookupTaskSupport` (`:117-135`) resolving a tool name to a group; `createTask` (`:235`) including the client-requested TTL read at `:257-265`.
- [ ] `internal/component/mcp/tools.go` (855L) - `groupTaskSupport` (`:404-421`) derives a group's level from its actions, with **required winning over forbidden**; `toolName` (`:250`); `callParams.Task` (`:716`); the sole `Elicit` call site (`:747`) inside handcrafted `ze_execute` (`:722`).
- [ ] `internal/component/mcp/streamable.go` - `newTaskRegistry(cfg.Tasks)` at `:181` and the `sessReg.onExpire` wire to `CancelAllForSession` at `:182-184`. Nothing in production sets `StreamableConfig.Tasks`, so the registry always runs on its defaults.
- [ ] `internal/component/mcp/auth.go` - `Identity` (`:87`), whose zero value is anonymous and is what `auth-mode none` yields (`:84-86`). `bearer.go:130` yields a config-defined name; `oauth.go:58` yields the JWT `sub` claim with no Ze-side allowlist.
- [ ] `internal/component/config/yang/command.go` - the `ze:task-support` walker: `PathToTaskSupport` (`:107`), `GetTaskSupportExtension` (`:432`), and the argument allowlist `validTaskSupportValues` (`:423`).
- [ ] `internal/component/config/yang/modules/ze-extensions.yang` - the extension declaration at `:120-126`, whose `description` is the only part D-1 edits.
- [ ] `internal/test/cli/cmd_mcp.go` (785L) - the test client: `--tasks` flag (`:34`), capability declaration (`:365-366`), and the `task-*` subcommands (`:707-756`) including `task-result` and `task-list`, which lose their methods.

**Behavior to preserve:**
- The worker model: a task runs the same dispatch a synchronous call would, on its own goroutine with its own cancellable context.
- Terminal-state retention with a TTL so a client that polls late still gets the result.
- Per-server, per-principal concurrency and terminal-retention caps, and their fail-closed rejection (`errTaskConcurrencyCap`, `tasks.go:27`).
- Cross-principal isolation: a foreign task id is indistinguishable from an unknown one (`tasks.go:148-150`).

**Behavior to change:**
- Everything in the Delete and Add lists above.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- HTTP POST `tools/call` from a client whose per-request `clientCapabilities.extensions` includes `io.modelcontextprotocol/tasks`.
- HTTP POST `tasks/get`, `tasks/update`, `tasks/cancel` carrying a `taskId`.

### Transformation Path
1. Phase 1 validation and per-request auth establish the principal (`Identity.Name`).
2. `tools/call`: resolve the tool to a command group, read its `ze:task-support` level (`lookupTaskSupport`), and decide. `forbidden` or `optional` runs synchronously. `required` runs as a task **only if** the request declared the extension; if it did not, it also runs synchronously (AC-6).
3. Task path: register the entry **before** responding, launch the worker under a deadline-bearing context, and return `CreateTaskResult` with `taskId`, status, `ttlMs` and `pollIntervalMs`.
4. `tasks/get`: look up by (principal, taskId); return current status, plus `result` on `completed` and `error` on `failed`.
5. `tasks/update`: verify ownership, then acknowledge with an empty result. Any `inputResponses` keys are unknown by construction and are ignored (D-4).
6. `tasks/cancel`: cooperative; acknowledge, do not guarantee.
7. GC: the ticker sweeps terminal entries past their TTL, and forces terminal any worker still running past its execution deadline (D-3).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Client ↔ task registry | `taskId` as a durable handle across independent HTTP requests | Yes: `tasks.go:139-152` already looks up by (identity, taskId) with no session in the key |
| MCP ↔ command dispatcher | Unchanged; the worker runs the same dispatch a synchronous call runs (`streamable_tools.go:279-292`) | Yes: read, and the captured-dispatch shape is preserved |
| Task worker ↔ MRTR types | None. No task can reach `input_required`, so Phase 2's `InputRequests` are never produced inside a task (D-4) | Yes: the only `Elicit` call site is `tools.go:747`, on a handcrafted tool that never becomes a task |
| Registry ↔ authenticated principal | `Identity.Name` from `bearer.go:130` (config-bounded) or `oauth.go:58` (JWT `sub`, not bounded by Ze config) | Yes: read both producers; see A-3 |

### Integration Points
- `internal/component/config/yang/command.go` - the `ze:task-support` walker. D-1 keeps its argument vocabulary (`validTaskSupportValues`, `:423`), so the walker is untouched.
- `internal/component/mcp/tools.go` - `groupTaskSupport` (`:404-421`), whose precedence rule this phase inverts to fail closed (D-1b).
- `internal/test/cli/cmd_mcp.go` - a polling loop replaces the blocking `tasks/result` call, and `task-list` loses its method.
- Phase 2's MRTR types are **not** an integration point for this phase (D-4). Recorded here because the skeleton assumed they were.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | The task path and the synchronous path share one decision point (`callTool`) and one dispatch shape; the worker adds a goroutine, not a second dispatch route |
| No unintended coupling | Yes | The phase removes coupling (task registry to session registry, `streamable.go:182-184`) and adds none. The registry's only inputs become the principal and the annotation |
| No duplicated functionality | Yes | The eligibility rule reuses the existing `ze:task-support` walker and `lookupTaskSupport` rather than adding a second per-command policy surface |
| Zero-copy preserved where applicable | N-A | JSON-RPC control plane, not a wire-encoding path. No pooled buffer is touched |
| Registration over hardcoding | Yes | Task eligibility stays a per-command YANG annotation discovered through the command registry. No plugin name is spelled in `internal/component/mcp/`, and adding a task-eligible command needs one YANG statement, no MCP change (`ai/rules/plugin-self-containment.md`) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `subscriptions/listen` is not needed for tasks | Umbrella A-4 (confirmed): polling is the spec default and Ze has nothing else to advertise | A fifth phase is needed before this one closes | Umbrella A-4 | confirmed |
| A-2 | A small closed set of Ze commands is genuinely long-running | The current `ze:task-support` annotations name them | The server-side promotion rule is harder than a per-command annotation | Enumerate the annotated commands during research | confirmed |
| A-3 | Principal-keyed task indexing cannot grow unbounded | Original basis: auth modes yield a bounded identity set (`bearer-list` entries, OAuth subjects). **Half of that basis is wrong**: OAuth subjects are bounded by the authorization server's user population, not by Ze config (`oauth.go:58`). The assumption still holds, but for a different reason, given below | The registry needs a per-principal cap or an eviction policy | Read `internal/component/mcp/auth.go` for the identity space | confirmed on a corrected basis, with a caveat that became D-3 |
| A-4 | No Ze task can reach `input_required`, so implementing that state would be dead code | The only `Elicit` call site is `tools.go:747` inside handcrafted `ze_execute` (`tools.go:722`); handcrafted tools resolve to `TaskSupportOptional` (`streamable_tools.go:117-124`), which D-1 maps to synchronous. The 9 `required` commands all dispatch through `dispatchGenerated` (`streamable_tools.go:109`, `:291`), which never elicits. `TaskElicit` (`tasks.go:464`), the only producer of the state, has no production caller | The `input_required` state and `inputRequests` on `tasks/get` must be implemented after all, and AC-7 reverts to the skeleton's wording | Re-run the two greps in the Deliverables Checklist before implementation starts; both must still return exactly one and zero production hits | confirmed |

**A-2 evidence.** `grep -rn "ze:task-support" --include=*.yang .` returns 13 annotations
in 9 modules: 9 `required` and 4 `forbidden`, with no explicit `optional`.
Required: `internal/plugins/ping-cmd/yang/ze-ping-cmd.yang:50`,
`internal/plugins/traceroute-cmd/yang/ze-traceroute-cmd.yang:74`,
`internal/plugins/meta/yang/ze-command-monitor-cmd.yang:15`,
`internal/component/ike/yang/ze-ipsec-cmd.yang:92`,
`internal/component/cmd/subscribe/yang/ze-cli-subscribe-cmd.yang:14`,
`internal/component/bgp/plugins/cmd/rib/yang/ze-rib-cmd.yang:18`,
`internal/component/bgp/plugins/cmd/monitor/yang/ze-monitor-cmd.yang:15`,
`internal/component/iface/yang/ze-iface-interface-cmd.yang:104`,
`internal/component/iface/yang/ze-iface-monitor-cmd.yang:19`. Forbidden: all four
in `internal/component/bgp/plugins/cmd/rib/yang/ze-rib-cmd.yang` at `:74`, `:84`,
`:108`, `:117`, namely `clear bgp rib in`, `clear bgp rib out`,
`request bgp rib inject` and `request bgp rib withdraw`. They are the mutating
commands, which is exactly the set R-1 exists to protect. The set is small,
closed, and semantically coherent: long reads and streams are `required`, state
mutations are `forbidden`.

**A-3 evidence.** The index self-prunes: `sweep` deletes a principal's map entry
when its last task is removed (`tasks.go:372-377`), and `Create` caps in-flight
tasks per principal at `maxConcurrent` (`tasks.go:103-107` via `activeCount`,
`:311-330`). So `byIdentity` is bounded by the number of principals holding at
least one live-or-recently-terminal task. The caveat: in `auth-mode oauth` the
principal is the JWT `sub` claim with no Ze-side allowlist (`oauth.go:58`), so the
principal *space* is the authorization server's user population. That is
acceptable only because every task terminates and is then swept. The one case
where it is not is a task that never terminates, which is precisely what D-3
adds an execution deadline for. Separately, in `auth-mode none` every client is
the same anonymous principal (`auth.go:84-86`), so the per-principal cap degrades
to a global cap and cross-principal isolation is vacuous; that is why AC-10's
test must use `bearer-list` with two identities.

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Retiring `ze:task-support` silently drops the forbidden case, letting a destructive command be promoted to a task | Review finds no replacement for `TaskSupportForbidden` | Closed by D-1: the annotation and its `forbidden` value survive, and AC-12 asserts the four mutating rib commands are never handed back as task handles |
| R-1b | A command group that mixes a `required` action with a `forbidden` one resolves to `required`, because `groupTaskSupport` (`tools.go:404-421`) lets required win. Under D-1 that would make the server auto-task a `forbidden` action | A group is authored with both levels. No group is mixed today: the four `forbidden` rib actions sit under the `clear` and `request` roots while the `required` one sits under `show` (`ze-rib-cmd.yang:18` against `:74`, `:84`, `:108`, `:117`), so they land in different tools | Invert the precedence to forbidden-wins (fail closed, `ai/rules/fail-closed-guards.md`) and unit-test a mixed group. The residual cost is that a genuinely long-running action sharing a group with a forbidden one runs synchronously; the fix for that is to split the tool, not to relax the guard |
| R-2 | Tasks are returned to clients that did not declare the extension | A client without the extension gets `resultType: "task"` and cannot interpret it | Explicit AC and negative test; the extension spec calls this out as a step. AC-6 asserts the positive half too (the call still succeeds synchronously), so the test cannot pass vacuously by the server simply failing the request |
| R-3 | Session removal leaves orphaned tasks with no reaper | A task sits in `working` forever, holding one of its principal's 8 concurrency slots, and is never swept because `sweep` only deletes terminal entries (`tasks.go:366-367`) | Confirmed as a real hole, not a hypothetical: `Create` gives the worker a context with no deadline (`tasks.go:115`), and `CancelAllForSession` was the only forced-terminal path. D-3 adds a per-task execution deadline; AC-13 tests it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Long-running MCP tool calls hang, return uninterpretable results, or leak registry entries. No dataplane impact |
| How is it reverted? | Single revert of the phase |
| Who else touches this path? | Phase 2 owns the `InputRequests` types this phase reuses |

## Wiring Test (MANDATORY -- NOT deferrable)

Test locations are `test/plugin/`, the suite that already owns every MCP `.ci`
file. `test/mcp/` does not exist and must not be created: `bgpCIRunnerDirs`
(`internal/test/cli/cmd_bgp.go:39-47`) is the closed registered-suite set, and
`TestCIRootsRegistered` (`internal/test/cli/register_test.go:66`) fails on a
`test/` directory holding `.ci` files that no runner claims. Four of these files
already exist and are rewritten rather than created.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Client with the tasks extension POSTs a `tools/call` on a `required` command, then polls `tasks/get` | → | `callTool` eligibility → registry `Create` → worker → terminal state → `tasks/get` | `test/plugin/task-rib-routes.ci` |
| Client without the extension POSTs the same call | → | extension-capability check → synchronous dispatch | `test/plugin/task-no-extension.ci` |
| Client POSTs `tools/call` on a `forbidden` command with the extension declared | → | eligibility check → synchronous dispatch, never a task handle | `test/plugin/task-forbidden.ci` |
| Client POSTs `tasks/update` for a task it owns | → | `tasks/update` → ownership check → empty acknowledgement | `test/plugin/task-update-ack.ci` |
| Client POSTs `tasks/list` or `tasks/result` | → | `runMethod` unknown-method path | `test/plugin/task-removed-methods.ci` |
| Principal B POSTs `tasks/get` for principal A's taskId | → | registry identity check (`tasks.go:148-150`) | `test/plugin/task-identity-scope.ci` |
| Client POSTs `server/discover` | → | discover handler → capabilities `extensions` map | `test/plugin/task-extension-advertised.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `tools/call` on a `ze:task-support required` command, from a client declaring the extension | `resultType: "task"` with `taskId`, status, `ttlMs`, `pollIntervalMs` |
| AC-2 | `tasks/get` on a working task | Current status, no result |
| AC-3 | `tasks/get` on a completed task | Terminal status with `result` |
| AC-4 | `tasks/get` on a failed task | Terminal status with `error` |
| AC-5 | `tasks/list` or `tasks/result` | Unknown method: HTTP 404 with `-32601` |
| AC-6 | Client that did not declare the tasks extension makes the same call | Never receives `resultType: "task"`, **and** the command still runs: the response is the ordinary synchronous result carrying `resultType: "complete"` |
| AC-7 | `tasks/update` naming a task the caller owns | Acknowledged with an empty result; the task's state is unchanged. Ze raises no `inputRequests`, so there is nothing to satisfy (D-4) |
| AC-8 | `tasks/update` carrying `inputResponses` with unknown or already-satisfied keys | Ignored, acknowledged with the same empty result. A malformed or foreign `taskId` is still rejected as not-found |
| AC-9 | `tasks/cancel` | Acknowledged with an empty result; the task may still reach a non-cancelled terminal state |
| AC-10 | `tasks/get` for a taskId minted for another principal, under `auth-mode bearer-list` with two identities | Not found, indistinguishable from a genuinely unknown id |
| AC-11 | `server/discover` | Advertises `extensions["io.modelcontextprotocol/tasks"]` |
| AC-12 | A command annotated `ze:task-support forbidden` (the four mutating rib commands), called with the extension declared | Never returned as a task handle; runs synchronously (R-1) |
| AC-13 | A task whose work never returns | Forced to a terminal state at the execution deadline, then swept by the TTL sweep, releasing its concurrency slot (D-3, R-3) |
| AC-14 | A command group mixing `required` and `forbidden` actions | Resolves to `forbidden`: the group is never auto-tasked (R-1b, fail closed) |
| AC-15 | Any `TaskState` rendered to the wire | The vocabulary is exactly `working`, `completed`, `failed`, `cancelled`. `input_required` is not a value Ze can produce (D-4) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Starts a long-running operation and polls it to completion | `tools/call` → `CreateTaskResult` → `tasks/get` × N → terminal | `test/plugin/task-rib-routes.ci` |
| 2 | Runs the same operation from a client that has not adopted the extension, and still gets an answer | `tools/call` → no extension declared → synchronous dispatch → `resultType: "complete"` | `test/plugin/task-no-extension.ci` |
| 3 | Cancels a running operation | `tasks/cancel` → cooperative stop | `TestTasksCancelAcknowledgesWithAnEmptyResult/working_task` (`internal/component/mcp/tasks_test.go`) for the RUNNING task, over the real HTTP transport with the dispatcher blocked on `ctx.Done()`; `test/plugin/task-cancel.ci` for the empty acknowledgement and terminal-state idempotence. Corrected 2026-07-30: the `.ci` alone was cited, and it cancels an already-terminal task. No `ze:task-support required` command blocks, so a `.ci` cannot win that race deterministically and asserting `cancelled` there would be a timing assumption (`ai/rules/fix-dont-record.md`) |
| 4 | Tries to run a route-injecting command as a task and cannot | `tools/call` on a `forbidden` command → synchronous dispatch, no task handle | `test/plugin/task-forbidden.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCreateTaskResultShape` | `internal/component/mcp/tasks_test.go` | AC-1 field set, including `ttlMs` and `pollIntervalMs` | |
| `TestTaskNotReturnedWithoutExtension` | `internal/component/mcp/tasks_test.go` | AC-6, both halves: no task handle, and a `complete` result | |
| `TestTaskScopedToPrincipal` | `internal/component/mcp/tasks_test.go` | AC-10 | |
| `TestTasksUpdateAcknowledgesAndIgnores` | `internal/component/mcp/tasks_test.go` | AC-7, AC-8 | |
| `TestTasksCancelAcknowledgesWithAnEmptyResult` | `internal/component/mcp/tasks_test.go` | AC-9, both halves: the acknowledgement is empty, and the cancellation still took effect | done 2026-07-30 (written late: an independent review found AC-9's result shape had NO wire-level test, and the handler had drifted to `{"taskId":…, "status":…}` against an AC and an extension that both say "empty result". The handler is now empty-acknowledging and this test pins it; the running-task half also covers user story 3) |
| `TestRemovedTaskMethods` | `internal/component/mcp/streamable_test.go` | AC-5 | |
| `TestTaskGCIndependentOfSessions` | `internal/component/mcp/tasks_test.go` | R-3: the sweep runs with no session registry in the picture | |
| `TestStuckTaskForcedTerminalAtDeadline` | `internal/component/mcp/tasks_test.go` | AC-13, and that the concurrency slot is released afterwards | |
| `TestGroupTaskSupportForbiddenWins` | `internal/component/mcp/tools_test.go` | AC-14, R-1b | |
| `TestTaskStateWireVocabulary` | `internal/component/mcp/task_state_test.go` | AC-15 | |
| `TestTaskEligibilityFromAnnotation` | `internal/component/mcp/tools_test.go` | D-1 mapping: required always, forbidden never, optional synchronous | |

### Boundary Tests (numeric inputs)

Both surviving numeric inputs enter at the registry constructor. The
client-requested TTL branch (`streamable_tools.go:257-265`) dies with
`params.task`, so this phase moves the `clampTTL` bound (`tasks.go:332-343`) from
the per-create path to `newTaskRegistry`, where `TaskRegistryConfig.TTL` is now
the only TTL input. `pollIntervalMs` is derived rather than configured, so its
boundary is an invariant over every legal TTL rather than an input range.

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `TaskRegistryConfig.TTL` (reported as `ttlMs`) | `minTaskTTL` 1s .. `maxTaskTTL` 1h (`tasks.go:18-20`); `<= 0` means "use `defaultTaskTTL`" 5m | 1h holds | 999ms clamps up to 1s; 0 selects the 5m default rather than clamping | 1h plus one nanosecond clamps down to 1h |
| `pollIntervalMs` (derived: `min(defaultPollIntervalMs 1000ms, ttlMs / 2)`) | 1 .. `ttlMs / 2` for every legal TTL | 1000ms at any TTL of 2s or more | 0 would make a conforming client busy-loop | anything above `ttlMs / 2` lets a client obeying the hint miss the terminal result. At the 1s minimum TTL the derived value is 500ms |
| `TaskRegistryConfig.MaxConcurrent` (in-flight tasks per principal) | 1 .. `maxConcurrent`, default 8 when `<= 0` (`tasks.go:69-72`), enforced at `tasks.go:103-107` | the 8th concurrent create succeeds | 0 concurrent tasks is legal, so there is no invalid-below | the 9th concurrent create returns `errTaskConcurrencyCap`; no upper bound on the configured value itself is enforced, which is acceptable while the field has no operator surface |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `task-rib-routes` | `test/plugin/task-rib-routes.ci` (rewrite) | Long operation polled to a result | |
| `task-cancel` | `test/plugin/task-cancel.ci` (rewrite) | Cancellation acknowledged | |
| `task-forbidden` | `test/plugin/task-forbidden.ci` (rewrite) | A mutating rib command is never handed back as a task | |
| `task-identity-scope` | `test/plugin/task-identity-scope.ci` (rewrite) | One principal cannot see another's task | |
| `task-no-extension` | `test/plugin/task-no-extension.ci` (new) | Client without the extension is never handed a task and still gets its answer | |
| `task-removed-methods` | `test/plugin/task-removed-methods.ci` (new) | `tasks/list` and `tasks/result` are gone | |
| `task-update-ack` | `test/plugin/task-update-ack.ci` (new) | `tasks/update` acknowledges and changes nothing | |
| `task-extension-advertised` | `test/plugin/task-extension-advertised.ci` (new) | `server/discover` names the tasks extension | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | - | - | No third-party MCP peer in-tree (umbrella Key Design Decisions) | |

## Files to Modify
- `internal/component/mcp/tasks.go` - principal-only scoping, execution deadline, TTL clamp moved to the constructor, `List` / `Result` / `CancelAllForSession` / `TaskElicit` / `buildTaskStatusNotification` removed
- `internal/component/mcp/task_state.go` - `TaskInputRequired` and its wire name removed (D-4)
- `internal/component/mcp/streamable_tools.go` - handler set: `tasks/list` and `tasks/result` out, `tasks/update` in; `callTool` becomes the server-side eligibility decision
- `internal/component/mcp/tools.go` - `groupTaskSupport` precedence inverted to forbidden-wins (R-1b); `callParams.Task` removed
- `internal/component/mcp/streamable.go` - `newTaskRegistry` call site and the `sessReg.onExpire` wire, to whatever extent Phase 1 leaves them
- `internal/component/mcp/discover.go` - extension advertisement (file created by Phase 1)
- `internal/component/config/yang/modules/ze-extensions.yang` - `ze:task-support` description only; the argument vocabulary and the walker in `internal/component/config/yang/command.go` are unchanged (D-1)
- `internal/test/cli/cmd_mcp.go` - per-request extension declaration replaces the initialize-time `--tasks` capability; polling loop replaces `task-result`; `task-list` removed
- `test/plugin/task-rib-routes.ci`, `task-cancel.ci`, `task-forbidden.ci`, `task-identity-scope.ci` - rewritten for the server-directed model
- `docs/guide/mcp/tasks.md`, `docs/architecture/mcp/overview.md`, `ai/digests/mcp.md`, `docs/architecture/api/commands.md`, `docs/features/mcp-integration.md`, `docs/functional-tests.md`, `docs/comparison.md`, `docs/features.md`

## Files to Create
- `test/plugin/task-no-extension.ci`, `task-removed-methods.ci`, `task-update-ack.ci`, `task-extension-advertised.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/config/yang/modules/ze-extensions.yang:120-126`: the `ze:task-support` `description` is rewritten for server-directed semantics. No new statement, no new leaf, no new argument value (D-1) |
| YANG validation constraints | No | `validTaskSupportValues` (`internal/component/config/yang/command.go:423`) accepts the same three arguments before and after. Nothing to add or tighten |
| YANG custom validators | N-A | No new leaves |
| CLI commands/flags | Yes | `internal/test/cli/cmd_mcp.go:34` `--tasks` becomes a per-request extension declaration (`:365-366`), and the `task-result` / `task-list` subcommands (`:732`, `:746`) lose their methods |
| CLI grammar | N-A | No operator-facing command surface. `ze:task-support` annotates existing commands and adds no token |
| Editor autocomplete | N-A | No new leaves |
| Functional test for new RPC/API | Yes | `test/plugin/*.ci`: four rewritten, four new, listed under Functional Tests |
| Pipe completeness | N-A | JSON-RPC over HTTP, not a CLI display surface |
| Env var registration | No | No `environment/mcp/` leaf is added. The task caps stay Go constants (`internal/component/mcp/tasks.go:15-21`) and `internal/component/mcp/yang/ze-mcp-conf.yang` has no task leaf to mirror |
| Doctor check for runtime dependencies | No | No new file path, socket, port, module or certificate. The MCP listener check is unchanged |
| Prometheus counters/metrics | No | `internal/component/mcp/` registers no metric today (a grep for `metrics.` and `prometheus` across the package, excluding tests, returns nothing) and this phase adds none. Introducing the subsystem's first counters is a separate decision from a conformance cutover, and the umbrella answers this row No |
| BGP family surface | N-A | Not a BGP change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` |
| 2 | Config syntax changed? | No | `ze:task-support` is a schema annotation inside `config false` `-cmd.yang` command modules, not operator-writable configuration, and `internal/component/mcp/yang/ze-mcp-conf.yang` gains no leaf. Nothing an operator types changes |
| 3 | CLI command added/changed? | Yes | `docs/functional-tests.md` for the `ze-test mcp` driver flags and the removed `task-list` / `task-result` subcommands |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md`: remove `tasks/list`, `tasks/result`; add `tasks/update` |
| 5 | Plugin added/changed? | No | MCP is a component |
| 6 | Has a user guide page? | Yes | `docs/guide/mcp/tasks.md`, rewritten |
| 7 | Wire format changed? | Yes | `docs/architecture/mcp/overview.md` |
| 8 | Plugin SDK/protocol changed? | No | Untouched |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `docs/features/mcp-integration.md` |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` tasks row |
| 12 | Internal architecture changed? | Yes | `docs/architecture/mcp/overview.md`, `ai/digests/mcp.md` |
| 13 | Route metadata keys added/changed? | N-A | Not routing |
| 14 | Prometheus counters added/changed? | No | None exist in `internal/component/mcp/` and none are added; see the Integration Checklist row for the grep |
| 15 | Registered plugin/event/command/capability changed? | Yes | The tasks capability moves into the `extensions` map |
| 16 | Changed source referenced by doc source anchors? | Yes | The overview anchors every MCP file |
| 17 | Docs show config/CLI/API examples for this area? | Yes | `docs/guide/mcp/tasks.md` examples are all the old shape |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - register `tasks/update`, advertise the extension in `server/discover`, failing wiring tests for both.
2. **Phase: Negotiation** - per-request extension capability check; a non-declaring client never receives a task handle and its call still runs synchronously (AC-6).
3. **Phase: Eligibility (D-1)** - `lookupTaskSupport` becomes the server's decision rule with inverted semantics; `groupTaskSupport` precedence inverted to forbidden-wins; `ze-extensions.yang` description updated. AC-12 and AC-14 land here, before any result-shape work, so the forbidden guard is never absent from a buildable tree.
4. **Phase: Result shapes** - `CreateTaskResult` with `resultType: "task"`, `ttlMs` and derived `pollIntervalMs`; `tasks/get` terminal payloads carrying `result` or `error`.
5. **Phase: Registry bounds (D-3)** - per-task execution deadline, TTL clamp moved to `newTaskRegistry`, AC-13.
6. **Phase: Removals** - `tasks/list`, `tasks/result`, `params.task`, `sessionID`, `CancelAllForSession`, the notification push, `TaskElicit`, and `TaskInputRequired` (D-4).
7. **Phase: Consumers and docs** - test client polling loop, guide, digest, and the Known Limitation recording why `input_required` is absent.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | No task handle reaches a client that did not declare the extension, and that client's call still succeeds synchronously (AC-6, both halves) |
| Correctness | Task lookup by (principal, taskId) cannot cross principals, and a foreign id is indistinguishable from an unknown one |
| Correctness | `ze:task-support forbidden` is honoured, including for a group that mixes levels (AC-12, AC-14) |
| Correctness | `pollIntervalMs` is at most half of `ttlMs` for every legal TTL, so a client obeying the hint cannot miss a terminal result (D-6) |
| Data flow | The GC path terminates without a session lifecycle to trigger it, **and** a task whose work never returns is forced terminal (AC-13). The first without the second is the R-3 hole |
| Rule: `ai/rules/enum-over-string.md` | Task state stays a typed enum; the wire string is produced at the boundary only |
| Rule: `ai/rules/wiring-completeness.md` | No exported symbol survives without a production caller. `TaskElicit` fails this today and must be gone |
| Rule: `ai/rules/no-parking.md` | No method is advertised in `server/discover` capabilities before it is implemented. `tasks/update` is implemented, not stubbed: it verifies ownership and returns the extension's specified acknowledgement |
| Rule: `ai/rules/interop-and-goal-validation.md` | No AC is proven by a test that would pass with the mechanism deleted. AC-6 and AC-12 assert the positive outcome alongside the absent one; AC-15 asserts an enumeration, not an absence |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Removed methods gone | `grep -n 'tasks/list\|tasks/result' internal/component/mcp/streamable_tools.go` is empty |
| Session coupling gone | `grep -n 'sessionID\|CancelAllForSession' internal/component/mcp/tasks.go` is empty |
| Client-directed opt-in gone | `grep -rn 'params.Task\|TaskSupportRequired' internal/component/mcp/streamable_tools.go` shows no client-supplied branch |
| Dead elicitation path gone | `grep -rn 'TaskElicit\|TaskInputRequired\|input_required' --include=*.go internal/component/mcp/` is empty (D-4) |
| A-4 still holds at implementation time | `grep -rn '\.Elicit(' --include=*.go internal/ \| grep -v _test.go` returns exactly one hit, `tools.go:747`, and it is still inside a handcrafted tool. If it returns more, re-open D-4 before coding |
| Extension advertised | `server/discover` response contains `io.modelcontextprotocol/tasks` |
| Tests run where the runner looks | The new and rewritten `.ci` files are under `test/plugin/`, and `go test ./internal/test/cli/...` (`TestCIRootsRegistered`) is green |
| Full gate | `make ze-verify` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Cross-principal task access | Lookup keyed by principal; a foreign taskId returns not-found, not a different error. The test must use `auth-mode bearer-list` with two identities: under `auth-mode none` every client is the same anonymous principal (`auth.go:84-86`) and the assertion is vacuous |
| Registry growth | Per-principal indexing bounded and self-pruning (`tasks.go:372-377`); in-flight tasks capped per principal (`tasks.go:103-107`); GC independent of sessions **and** able to reap a task whose work never returns (D-3). Note the principal space itself is unbounded under `auth-mode oauth`, where the principal is the JWT `sub` (`oauth.go:58`) |
| Destructive commands as tasks | The never-a-task annotation is honoured, including in a mixed group (R-1, R-1b) |
| `tasks/update` input | `inputResponses` are attacker-controlled. Ze has no outstanding `inputRequests` to match them against, so every key is unknown and every value is discarded unread; the only thing the handler acts on is the `taskId`, which is ownership-checked before anything else. Discarding is safe precisely because nothing consumes it: if D-4 is ever reversed, this row becomes a real validation requirement |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read Current Behavior; if misunderstood → RESEARCH |
| Lint failure | Fix inline; if architectural → DESIGN |
| Functional test fails | Wrong AC → DESIGN, correct AC → IMPLEMENT |
| D-1 turns out to need a YANG argument change after all | STOP. It is a YANG surface decision. Ask the user |
| A-4 breaks: an elicitation appears on a task-eligible path | STOP before coding. D-4, AC-7, AC-15 and the Known Limitation all change together; `input_required` becomes required work, not a limitation |
| A spec MUST cannot be met as designed | STOP. Escalate per `ai/rules/rfc-compliance.md` |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- **The phase inverts a sentence, not a schema.** `ze:task-support` reads as a
  statement about the command ("task execution is required / forbidden for this
  command"), never about who asks for it. Once that is seen, the whole
  server-decides migration costs one `description` edit and one changed
  consumer, and the 13 annotations in 9 modules keep their meaning. The question
  that looked like a YANG surface change was a reading problem.
- **The eligibility decision is per tool, not per command.** `lookupTaskSupport`
  (`streamable_tools.go:117-135`) resolves a tool name to a command **group**, and
  `groupTaskSupport` (`tools.go:404-421`) folds the group's actions into one
  level with required winning over forbidden. Under client-directed semantics
  that precedence was harmless, because the client still had to ask and the
  per-action check ran on the way in. Under server-directed semantics it becomes
  a promotion rule, and required-wins would auto-task an action explicitly marked
  forbidden. Nothing mixes today, so the fix is free now and expensive later.
- **Deleting the session reaper removes a bound nobody registered as a bound.**
  `CancelAllForSession` looked like session cleanup. It was also the only path
  that could force a non-terminal task terminal, because the worker context has
  no deadline (`tasks.go:115`) and the sweep only reaps entries that already
  reached a terminal state (`tasks.go:366-367`). Removing sessions therefore
  removes a liveness guarantee, not just a cleanup hook, and the replacement has
  to be explicit.
- **The unreachable half of the extension is already dead code in the tree.**
  `TaskElicit` (`tasks.go:464`) is exported, transitions a task into
  `input_required`, and has no caller outside `task_elicit_test.go`. So the
  question "should we implement `input_required`" is really "should we resurrect
  a function nothing calls, to serve a state nothing can enter". Implementing it
  would satisfy the extension's vocabulary and violate
  `ai/rules/wiring-completeness.md`; the honest move is to delete the state and
  record the trigger that brings it back.
- **`test/mcp/` does not exist and cannot be created cheaply.** Every MCP `.ci`
  test lives in `test/plugin/`. The runner's suite set is closed
  (`internal/test/cli/cmd_bgp.go:39-47`) and `TestCIRootsRegistered`
  (`internal/test/cli/register_test.go:66`) fails on an unclaimed `test/`
  directory holding `.ci` files, so a new suite is a runner change, not a
  `mkdir`. The umbrella and this spec's skeleton both named `test/mcp/*.ci`; the
  paths here are corrected to the ones the runner actually executes.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| D-1: keep `ze:task-support`, its three arguments and its walker, and invert only the semantics: `required` means the server always tasks the call, `forbidden` never, `optional` synchronous | (a) Retire the extension and always answer synchronously. (b) Add a wall-clock threshold that promotes a slow synchronous call into a task mid-flight. (c) Rename the arguments to `always` / `never` / `optional` to match the new decider | (a) loses the only structural guard against auto-tasking the four mutating rib commands (R-1) and forces 9 genuinely long operations to hold an HTTP request open for their whole duration. (b) cannot be expressed: a `tools/call` yields exactly one JSON-RPC result, so promotion would need the response shape to change after it was chosen, plus speculative task registration on every call. It is also YAGNI against a closed 13-annotation set. (c) is 13 YANG edits plus walker and allowlist churn for no behavioural gain, and the existing words already describe the command rather than the decider |
| D-1b: when a command group mixes `required` and `forbidden` actions, the group resolves to `forbidden` | Keep required-wins (`tools.go:404-421`); or reject a mixed group at tool-generation time | Fail closed: a command marked forbidden must never be auto-tasked, and the cost of being wrong in that direction is a synchronous long request, while the cost in the other direction is auto-tasking a route injection (`ai/rules/fail-closed-guards.md`). Rejecting at generation time is stricter still but `generateTools` has no error channel and runs per `tools/list`; a mixed group is a schema smell whose real fix is splitting the tool, and AC-14 makes it visible |
| D-2: a client that did not declare the extension still gets its answer, synchronously | Reject the call with an error telling the client to declare the extension | The extension is an optimization over a synchronous call, not a precondition for the work. Rejecting would make 9 commands unreachable to any client that has not adopted an optional extension, which is a conformance regression wearing strictness as a disguise. It would also make AC-6 vacuous: a test asserting "no task handle" would pass on a server that simply failed every such request |
| D-3: every task worker runs under a server-side execution deadline, and the sweep forces past-deadline tasks terminal | Rely on the TTL sweep alone; or add a per-principal cap on total (not in-flight) tasks | The sweep cannot see a task that never terminates (`tasks.go:366-367`), and such a task holds one of its principal's 8 in-flight slots forever (`tasks.go:103-107`). A deadline restores exactly what the deleted session reaper provided, at the layer that owns the worker. A total-task cap would bound memory but still leak concurrency slots |
| D-4: implement `tasks/update` fully; do not implement the `input_required` state, and delete `TaskInputRequired` | Implement the state anyway for vocabulary completeness; or omit `tasks/update` as well since nothing can be answered through it | No Ze task can reach `input_required` (A-4), so implementing the state is dead code (`ai/rules/wiring-completeness.md`) and a test for it could only assert an absence. `tasks/update` is different: for a server that raises no input requests, "verify ownership, acknowledge empty, ignore unknown keys" is the extension's own tolerance rule and is a complete implementation, so advertising the extension without the method would be the parking violation (`ai/rules/no-parking.md`) |
| D-5: functional tests live in `test/plugin/`, not `test/mcp/` | Create a `test/mcp/` suite as the umbrella and this spec's skeleton both assumed | `test/mcp/` has no runner. `bgpCIRunnerDirs` (`internal/test/cli/cmd_bgp.go:39-47`) is the closed set and `TestCIRootsRegistered` (`internal/test/cli/register_test.go:66`) fails on an orphaned directory, so the tests would either not run or turn the gate red. Every MCP `.ci` file already lives in `test/plugin/` |
| D-6: `pollIntervalMs` is derived from the TTL, not configured | A fixed constant; or an operator-tunable leaf | A fixed constant can exceed the retention window at the 1s minimum TTL, letting a conforming client miss the result. Deriving it keeps one invariant testable across every legal TTL (`ai/rules/derive-not-hardcode.md`), and an operator leaf is YAGNI while no operator surface for task limits exists at all |

## Known Limitations

- **`notifications/tasks` via `subscriptions/listen` is not implemented**; polling is the spec default (umbrella A-4, confirmed).
- **The `input_required` task state is not implemented.** The extension defines it as a state a server **may** enter, and Ze cannot enter it: the only elicitation call site in the tree is `tools.go:747`, inside the handcrafted `ze_execute` tool, which resolves to `TaskSupportOptional` (`streamable_tools.go:117-124`) and therefore runs synchronously under D-1. `tasks/update` is implemented in full, so a client that sends one gets the acknowledgement the extension specifies. **The trigger that reintroduces the state:** a command annotated `ze:task-support required` gains an elicitation, or a handcrafted tool that elicits becomes task-eligible. Either makes a task able to raise `inputRequests`, at which point `TaskInputRequired`, the `inputRequests` payload on `tasks/get`, and real `inputResponses` matching in `tasks/update` all become reachable and must be implemented together, reusing Phase 2's MRTR types. The two greps in the Deliverables Checklist are what detect the trigger.
- **Task limits have no operator surface.** `maxConcurrent`, `maxTerminal` and the TTL are Go constants with a `TaskRegistryConfig` seam that nothing in production populates (`streamable.go:181` reads `cfg.Tasks`, and no caller sets it). This phase does not add YANG leaves for them; if an operator ever needs to tune them, that is a config-surface decision governed by `ai/rules/config-surface.md`, not part of a conformance cutover.

## RFC Documentation (Scope: protocol)

Add `// MCP 2026-07-28 ext-tasks Section X: "<quoted requirement>"` above each
enforcing path: the extension capability check before returning a task handle,
durable creation before responding, the terminal-state payload rule, and the
`tasks/update` unknown-key tolerance.

Also record the deliberate omission next to the state enum: a comment naming
`input_required` as a state the extension permits and Ze cannot enter, pointing
at the Known Limitation, so a future reader does not mistake its absence for an
oversight (`ai/rules/self-documenting.md`).

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-15 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes
- [ ] Feature code integrated, not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled
- [ ] Critical Review passes
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only
