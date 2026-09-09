# Spec: plugin-runner-inertness

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Depends | - |
| Phase | - |
| Updated | 2026-09-09 |

## Task

A plugin runner does its dirty work BEFORE it declares. Measured across 92
registered product runners on 2026-09-08, 19 of them reach outside their own
locals before `p.Run` sends Stage 1:

| Finding | Runners | What runs before Stage 1 |
|---------|---------|--------------------------|
| Mutates the HOST | `flowspec-firewall`, `ike` | nftables syscalls through `ApplyAll` under `LegacySweepPending`; four node-wide XFRM policies through `installIKEBypass`, whose defer beside a live IKE engine removes that daemon's too |
| Mutates the calling PROCESS | `trafficusage` | `rlimit.RemoveMemlock()`, a `setrlimit`, before its own `!p.IsInternal()` gate |
| Opens a kernel handle | `fib/kernel`, `fib/p4`, `sysctl` | `newBackend()`, `netlink.NewHandle` inside it, closed after the abort's `return 1` |
| Allocates the process-wide default Loc-RIB | `connected`, `rib`, `static`, `sysrib` | `locrib.Default()` |
| Starts a goroutine | `iface` (four), `rpki`, `adj_rib_in` | workers started before the declaration; `iface`'s stops are straight-line code after `p.Run`, so its `return 1` skips all six |
| Leaves a process global pointing at a dead plugin | `adj_rib_in`, `rib`, `redistribute_egress`, `sysrib`, `isis`, `ospf`, `iface`, `kernel`, `connected` | exactly ONE is a defer (`rib`'s `activeManager.Store(nil)`, which does not unwind its route-injector registration); `ospf` also registers an opaque type whose second registration returns `ErrOpaqueTypeRegistered` and is only logged |
| Constructs a manager and subscribes | `as112` | `newAS112Producer`, `newServerManager`, `subscribeReplay` |

The work is to split each runner into a declaration part that is a pure value
and a start part that does everything else, so a runner that aborts at Stage 1
leaves the host, the process and every process global as it found them.

**Provenance:** `spec-plugin-query-mode`, closed 2026-09-09. Query mode does not
depend on this: it never enters a runner body, which is what makes its inertness
a property rather than a promise (`plan/learned/013-inertness-is-unreachability.md`).
`plan/learned/007-declaration-on-the-registration.md` carries the same audit and
is the answer to any proposal to run engines for introspection. Both closed specs
named this item and neither did it.

## Required Reading

<!-- NEVER tick [ ] to [x]. -->

### Architecture Docs
- [ ] `docs/architecture/api/process-protocol.md` - the five startup stages, and what a Stage 1 abort does to a plugin
- [ ] `docs/architecture/plugin/plugin-system.md` - registration, discovery, the process boundary
- [ ] `ai/rules/plugins.md` - the directive that a plugin's declaration must be a pure value and every side effect must sit inside the function a declaration query never calls
- [ ] `ai/rules/goroutine-lifecycle.md` - a worker started before the declaration has no stop on the abort path
- [ ] `ai/patterns/plugin.md` - the structural template every runner follows

### RFC Summaries (Scope: protocol)
N-A. No wire protocol and no RFC obligation.

**Key insights:** (minimal context to resume after compaction)
- The abort path is the one nobody tests: `p.Run` returning an error runs the code AFTER it, and `iface`'s six stops are all after it.
- Seven runners never reach Stage 1 at all, so "declared nothing" and "sent nothing" are different states and stay so.
- One defer exists across nine process-global writers, so unwinding is the exception rather than the rule.

## Current Behavior (MANDATORY)

**Source files read:** (2026-09-08, at every producer, re-measuring the 2026-09-07 audit with three corrections and seven additions)
- [ ] The 19 runners named in the table above, each at its own `RunEngine` entry point
- [ ] `pkg/plugin/sdk/sdk.go` - `(*Plugin).Run` runs the five stages in a straight line; Stage 1 is the first engine call
- [ ] `internal/component/plugin/server/startup.go` - what the engine does with a plugin that fails at Stage 1

**Behavior to preserve:**
- The five stages and their order. No stage is added and no declaration is re-read.
- A plugin that starts successfully behaves exactly as it does today.
- `capa`, `loop` and `srpolicy` return without calling `p.Run`, and `as112`, `flowexport`, `vrrp` and `trafficusage` return 1 at `if !p.IsInternal()`. Those seven stay as they are: they never reach Stage 1, which is a different fact from declaring nothing.

**Behavior to change:**
- A runner that aborts at Stage 1 leaves no nftables rule, no XFRM policy, no changed rlimit, no open kernel handle, no running goroutine and no process global pointing at a dead plugin.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `RunEngine func(conn net.Conn) int`, the runner the plugin registers.

### Transformation Path
(fill during design: whether the split is a convention each runner adopts, an SDK entry point that owns the order the way `sdk.RunOrDeclare` does for a third-party binary, or a check that refuses a runner whose body reaches outside its locals before `p.Run`.)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| runner body -> host state | nftables, XFRM, netlink, setrlimit | [ ] |
| runner body -> process state | process globals, goroutines, the default Loc-RIB | [ ] |

### Integration Points
- `pkg/plugin/sdk` - where an order-owning entry point would live
- The 19 runner packages named above

### Architectural Verification
(fill during design)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The audit above still holds | Re-measured at every producer 2026-09-08 for `spec-plugin-query-mode`, with three corrections | The spec is the wrong size | A re-read at design time; A-3 of the closed spec says the findings are re-read rather than trusted forever | unvalidated |
| A-2 | Each of the 19 can be split without changing what a successful start does | (fill during design) | The split is a behavior change per runner and each needs its own test | Read each runner | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | 19 runners in one commit is one review nobody can hold | The diff spans 19 packages | One runner per commit, or one commit per finding row |
| R-2 | A split that only moves code leaves the abort path untested | No test drives an abort | Each runner owes a test that aborts at Stage 1 and asserts the state it did not leave behind |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| a runner whose Stage 1 fails | -> | the split runner's start part, never entered | (fill during design) |
| a runner that starts successfully | -> | the same behavior it has today | (fill during design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Each of the 19 runners is started with a Stage 1 that fails | The host, the calling process and every process global are as they were before the start |
| AC-2 | Each of the 19 runners is started normally | Behavior is unchanged from today |
| AC-3 | (fill during design: whether a check refuses a new runner that reaches outside its locals before `p.Run`) | (fill during design) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (fill during design: one per runner) | | AC-1 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| (fill during design) | `test/plugin/` | | |

### Interop Tests (Scope: protocol)
N-A. No wire-visible change.

## Files to Modify
- The 19 runner packages named in the Task table
- `pkg/plugin/sdk` - only if the design puts the order in an entry point
- `ai/rules/plugins.md` - only if the design adds a rule a check enforces

## Implementation Steps

1. **Phase: Audit.** Re-measure the table above at every producer and record what changed.
2. **Phase: Shape.** Decide between convention, an SDK entry point, and a check. Put the choice to Thomas: it decides whether this is 19 edits or one.
3. (fill during design)

## Known Limitations
- (fill during design)

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1 and AC-2 demonstrated for every runner the spec touches
- [ ] Wiring Test table complete
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests for end-to-end behavior
