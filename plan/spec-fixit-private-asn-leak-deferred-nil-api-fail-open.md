# Spec: fixit-private-asn-leak-deferred-nil-api-fail-open

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-07-19 |

> **Readiness pass (2026-07-17, autonomous).** Status advanced `skeleton` → `ready`.
> Every `file:line` in this spec was re-opened and confirmed against the committed
> tree; the open design decisions (fail-closed direction, the fused `facts == nil`,
> the ingress guard, `SetPluginServerAny`) are resolved append-only under
> "Autonomous Resolutions" after the Acceptance Criteria table. [STAKES: security]
>
> **Dependency check (answers "does this need the parent to land first?"): NO.**
> The shared chain body this spec fixes is already **committed**, not pending:
> `1bf31e316` ("apply export filters to originated routes") and `afb068cc0` ("stop
> applying export filters twice") landed the parent's `runEgressPolicyChainASN4` /
> `exportFilterForBody` delegation. The guard sites (`filter_ordered.go:196,222`,
> `egress_inject_filter.go:43`, `filter_ordered.go:139`) exist at stable committed
> line numbers, verified clean in the working tree. This spec edits those functions
> in place; it does not wait on any unlanded parent work. Not a blocker to readiness.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/fail-closed-guards.md` - the rule this violates
4. `internal/component/bgp/reactor/filter_ordered.go:196,222` - the two fail-open guards
5. `internal/component/bgp/reactor/filter_chain.go:368-371` - the fail-CLOSED guard on the same condition, shadowed by the above
6. `internal/component/bgp/reactor/reactor.go:966-1045` - `StartWithContext`, where the `r.mu` barrier that incidentally saves us lives

## Task

Resolve the `r.api == nil` silent fail-open on the BGP egress filter path: a peer WITH
export filters and no API server sends **unfiltered, silently**.

**Provenance:** deferred from `plan/spec-fixit-private-asn-leak.md` risk R1, ruled by Thomas
2026-07-16: *trace reachability first, then decide fix vs fail-closed*. R1 was left open
there because the path was never traced to a producer, and inventing a fix for an untraced
path is what `ai/rules/no-fabrication.md` forbids.

**The trace is now largely done (2026-07-16) and it changes the framing.** Reachability is
the weaker argument. The stronger one needs no reachability proof at all:

> The same `r.api == nil` condition is guarded in three places in this one package. Two fail
> **closed and loudly**. One fails **open and silently**. And the fail-open pre-empts the
> fail-closed, so the correct guard is unreachable.

| Site | Behavior on `r.api == nil` | Evidence |
|------|---------------------------|----------|
| `filter_chain.go:368-371` (`policyFilterFunc`) | **fail-CLOSED** + `Warn("policy filter: no API server", ...)`, returns `PolicyResponse{Action: PolicyReject}` | reads `// fail-closed` in the source |
| `peer_initial_sync.go:718-722` | **fail-CLOSED** + `Warn("default-originate: no reactor API -- fail-closed")` | |
| `filter_ordered.go:196` (`runEgressPolicyChain`) | **fail-OPEN**, silent: `if len(exportFilters) == 0 \|\| r.api == nil { return egressStepResult{accept: true} }` | no log, no metric, no error |
| `filter_ordered.go:222` (`runEgressPolicyChainASN4`) | **fail-OPEN**, silent, identical guard | **this is the one that matters** -- the shared chain body `exportFilterForBody` actually reaches |
| `egress_inject_filter.go:43` (`exportFilterForBody`) | **fail-OPEN**, silent: `if facts == nil \|\| len(facts.exportFilters) == 0 \|\| r.api == nil { return false, nil }` | note `facts == nil` is a **second** fail-open fused into the same condition |
| `filter_ordered.go:139` (`runIngressPolicyChain`) | **fail-OPEN**, silent | same defect on the ingress side; **not** named in R1, found by this trace |

`policyFilterFunc` is only invoked *through* `PolicyFilterChain` at `filter_ordered.go:147`
and `:232` -- both **after** the early returns at `:196`/`:222`. So when api is nil the
loud, correct guard never runs. The package contradicts itself, and the wrong side wins.

**Correction to R1's citation, for whoever implements this:** R1 says the guard is in
"`exportFilterForBody` and `runEgressPolicyChain` (`filter_ordered.go:196`)". `:196` is
exact for `runEgressPolicyChain`, but `exportFilterForBody`'s own guard is at
`egress_inject_filter.go:43`, and R1 misses `:222` (the shared body) and `:139` (ingress).
Five sites, not two.

### Reachability: traced, and the answer is "no, but only by accident"

I could not construct a path where a session reaches Established with `r.api == nil` in a
**successfully-started** reactor. Recorded in full so nobody re-does this trace:

| Mode | Finding | Evidence |
|------|---------|----------|
| Borrow (production) | **Not reachable.** api is set before start, and a nil api hard-fails before anything runs | `register.go:145` `SetPluginServerAny` then `:160` `StartWithContext`; `reactor.go:1165` `if r.externalServer && r.api == nil { return errBorrowModeNoServer }` |
| Standalone | **Not reachable, incidentally.** Listeners bind at `reactor.go:1006` and `:1012` **before** `startAPIServer()` at `:1017` -- the ordering is genuinely inverted. What closes the window is that `StartWithContext` holds `r.mu.Lock()` from `:966` unbroken to `:1045`, and `handleConnection` blocks on `r.mu.RLock()` (`reactor_connection.go:39`) as its first action | outbound peers start at `:1095`, also after `:1017`; borrow mode defers peers to `StartPeers()` |
| Teardown | `reactor.go:1290` nils api in `abortStartup`, before `stopAllListeners()` at `:1297`. Safe only because `r.mu` is held and `r.running` is never set | no `Stop()`/shutdown path nils api |

**The standalone safety rests on an undocumented, incidental `r.mu` barrier.** Nothing
states the lock is what closes the window. Any future change that releases `r.mu` earlier,
or accepts a connection off a path that does not take `r.mu`, reopens it silently -- and
the failure mode is "routes leak unfiltered", with no log.

**Not established:** reachability for `&Reactor{}` literals constructed outside
`StartWithContext`. Tests do exactly this, and every embedder outside
`internal/component/bgp` was not audited. See the test finding below.

### A third fail-open, found by this trace

`reactor.go:576` (`SetPluginServerAny`): `if srv, ok := s.(*pluginserver.Server); ok { r.api = srv }`.
**A failed type assertion silently leaves api nil** -- an unguarded fail-open feeding the
fail-open chain above. It is the one plausible producer of a nil api in a reactor that
otherwise started fine, and `register.go:145` is its only caller.

### The tests are the fail-open's best customer

`forward_update_test.go:83,189,270,366,472,568,657,795,1086` and `forward_split_test.go:76`
construct `&Reactor{...}` literals with `api` unset. They drive
`ForwardUpdate`/`forwardUpdateCore`, so **every one of them silently skips the egress policy
chain**. No test asserts what happens when api is nil; `forward_rs_test.go:233` sets
`ExportFilters` but tests the fast-path skip, not the chain. This is
`fail-closed-guards.md`'s Test corollary in the flesh: *"A green unit test on an uncalled
guard is worse than no test."* Making the guard fail closed will surface here first, and
that fallout is this spec's real cost.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/fail-closed-guards.md` - the rule this breaks
  → Constraint (verbatim): "**BLOCKING.** A guard must fail closed or say something. Silent degradation into a permissive no-op is the bug, and a zero value that downstream reads as a legitimate answer is how it hides."
  → Constraint: "On a miss, an unmapped input, an empty set, or an error, deny. Never fall through to the permissive branch."
  → Constraint: "A guard that genuinely cannot deny MUST log, error, or fail its gate. A guard that neither denies nor speaks does not exist."
  → Applied: an export filter chain is a guard whose purpose is to reject. `r.api == nil` is exactly "a miss". `accept: true` is the zero-value trap verbatim -- downstream reads it as "the filters ran and passed".
- [ ] `ai/rules/no-fabrication.md` - why R1 was left open rather than fixed blind
  → Constraint: do not invent a fix for an untraced path. The trace above is what unlocks this spec.

**Key insights:**
- The fix does not need a reachability proof: two sibling guards already establish the house answer for this exact condition (`filter_chain.go:368-371`, `peer_initial_sync.go:718-722`). This spec is mostly *making the outlier agree with them*.
- `len(exportFilters) == 0` and `r.api == nil` are fused in one condition at `:196`/`:222`. They are opposite cases: no filters configured is a legitimate accept; filters configured but no API server is a miss. Splitting them is likely the whole fix.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/filter_ordered.go` - `:139`, `:196`, `:222` fail-open guards; `:147`, `:232` the `PolicyFilterChain` calls they pre-empt
- [ ] `internal/component/bgp/reactor/filter_chain.go` - `:368-371` the fail-closed guard on the same condition
- [ ] `internal/component/bgp/reactor/egress_inject_filter.go` - `:43` fail-open, with `facts == nil` fused in
- [ ] `internal/component/bgp/reactor/reactor.go` - `:247` the field; `:569`, `:576`, `:1193`, `:1290` every assignment; `:1006-1017` the inverted bind/start ordering; `:1165` the borrow-mode hard fail
- [ ] `internal/component/bgp/reactor/reactor_connection.go` - `:39` the `r.mu.RLock()` that incidentally closes the standalone window

**Behavior to preserve:**
- A peer with **no** export filters configured still accepts (that is not a miss).
- Every currently-passing filter test keeps its result, or its change is justified as the fail-open being removed.

**Behavior to change:**
- `r.api == nil` **with** export filters configured must stop silently accepting. Deny or speak, per the rule.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A route reaches a peer's egress path: forwarded (`forwardUpdateCore` -> `reactor_api_forward.go:480-499`) or originated/injected (`exportFilterForBody`, `egress_inject_filter.go`)
- The peer's `ExportFilters` are non-empty (config-parse-time, `config/peers.go:143-168`)

### Transformation Path
1. Egress chain entered: `runEgressPolicyChain` (`filter_ordered.go:195-203`) or, for the originated path, `runEgressPolicyChainASN4` directly (`egress_inject_filter.go:56`)
2. **The guard fires:** `if len(exportFilters) == 0 || r.api == nil` (`:196`, `:222`) -- two unrelated cases fused into one condition
3. **Fail-open:** returns `egressStepResult{accept: true}` silently. `exportFilterForBody` returns `(false, nil)`: no suppress, no override
4. **The correct guard is never reached:** `PolicyFilterChain` (`filter_ordered.go:147`, `:232`) -> `policyFilterFunc` (`filter_chain.go:368-371`) would `Warn` and `PolicyReject`, but sits after step 2's early return
5. Route goes to the wire unfiltered; downstream reads `accept: true` as "the filters ran and passed"

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor ↔ plugin API server | `r.api *pluginserver.Server` (`reactor.go:247`); nil is the whole subject | [ ] |
| Config ↔ per-peer runtime | `ExportFilters` frozen onto `PeerSettings` (`peersettings.go:392-394`) | [ ] |

### Integration Points
- `internal/component/bgp/reactor/filter_chain.go:368-371` - the house answer for this exact condition; the fix should agree with it rather than invent a third behavior
- `internal/component/bgp/reactor/peer_initial_sync.go:718-722` - the second precedent, same shape (`"default-originate: no reactor API -- fail-closed"`)
- `internal/component/bgp/reactor/reactor.go:1165` - borrow mode already hard-fails a nil api (`errBorrowModeNoServer`); the fix extends that stance rather than introducing it

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Peer with export filters + `r.api == nil`, route forwarded | → | the guard at `filter_ordered.go:222` (`runEgressPolicyChainASN4`) | Go unit test in `filter_ordered_test.go` — MUST be RED before the fix |
| Peer with export filters + `r.api == nil`, route originated | → | the guard at `egress_inject_filter.go:43` (`exportFilterForBody`) | Go unit test in `egress_inject_filter_test.go` |
| `SetPluginServerAny` handed a non-`*pluginserver.Server` | → | `reactor.go:576` | Go unit test in `reactor_test.go` |

**On the missing `.ci` row (the validator warns, and it is right to):** this spec has **no
user-facing surface** — there is no config, no CLI, no operator-visible feature to drive
from a `.ci`. The subject is an internal guard on a state (`r.api == nil`) that the trace
says is **not reachable** through any entry point a `.ci` can reach (see the reachability
table: borrow mode hard-fails at `reactor.go:1165`, standalone is closed by the `r.mu`
barrier). A `.ci` that cannot construct the precondition would be theatre.

That is precisely why this is a `skeleton` and not `ready`: **if design finds a real entry
point that reaches the nil-api state, this spec MUST grow a `.ci` row here** — because that
discovery would also mean the leak is live rather than hygienic, and A-1 is broken. Treat
this note as the trigger, not as an exemption.

### Architectural Verification
- [ ] No bypassed layers (the fix lives in the guard, not in a caller working around it)
- [ ] No unintended coupling (the reactor's egress path does not learn about API-server internals; it only stops trusting a nil one)
- [ ] No duplicated functionality — **the load-bearing one here:** `filter_chain.go:368-371` already answers this exact condition. The fix makes the outlier agree with it; it must NOT introduce a third, differently-worded behavior for the same nil check
- [ ] Registration over hardcoding — N/A by inspection: this spec adds no command, view, family, or handler, and no new field, switch case, or factory in a core/shared package (`ai/rules/plugin-self-containment.md`). It removes a permissive branch from an existing guard and touches no registry

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | Not reachable in a successfully-started reactor | the trace above | The leak is live in production, and this is urgent rather than hygienic | the trace; re-verify `reactor.go:1006-1017` ordering has not changed | confirmed for both modes, **incidentally** (rests on the `r.mu` barrier) |
| A-2 | `&Reactor{}` literals outside `StartWithContext` bypass every ordering guarantee | `forward_update_test.go` builds exactly these | Test fallout is larger than expected | audit embedders outside `internal/component/bgp` | unvalidated |
| A-3 | Failing closed here breaks no legitimate deployment | borrow mode hard-fails a nil api at `:1165` already | A standalone deployment relying on the no-op starts rejecting routes | design review | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Failing closed turns ~10 nil-api unit tests red at once | `make ze-unit-test` after the guard flips | Expected and correct: they were passing *because* of the bug. Give each a real api or an explicit no-filters setup |
| R-2 | `r.mu` barrier changes later and reopens the window silently | none today -- that is the point | Whatever the fix, leave the ordering comment `reactor.go:1006-1017` deserves and never got |
| R-3 | Fixing only the egress sites leaves `:139` (ingress) fail-open | grep after the change | Fix all five, or state why ingress differs |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Peer with export filters, `r.api == nil`, route forwarded | Route is NOT sent unfiltered: deny, or log loudly -- matching `filter_chain.go:368-371`'s answer for the same condition |
| AC-2 | Peer with NO export filters, `r.api == nil` | Unchanged: accepts (not a miss) |
| AC-3 | `SetPluginServerAny` handed a non-`*pluginserver.Server` | Does not silently leave api nil (`reactor.go:576`) |
| AC-4 | The ingress guard `filter_ordered.go:139` | Same treatment, or a documented reason it differs |
| AC-5 | The nil-api unit tests | Each either gets a real api or explicitly asserts the fail-closed behavior |
| AC-6 | Whole-repo grep for `api == nil` in the reactor package | Every site fails closed or speaks; no silent permissive branch survives |

### Autonomous Resolutions (2026-07-17) -- [STAKES: security]

These resolve the decisions AC-1/AC-3/AC-4 and the fused `facts == nil` left undecided
above. All are grounded in re-read producers; append-only, superseding no prior text.

**→ AUTONOMOUS DEFAULT (2026-07-17): the direction is FAIL CLOSED, and it is DENY *and*
Warn, not "or".** AC-1's wording "deny, **or** log loudly" is superseded: when
`r.api == nil` while the peer HAS export filters (`facts != nil && len(facts.exportFilters) > 0`),
the egress guards MUST both suppress the route AND emit a `Warn`, exactly as the two
siblings already do: `filter_chain.go:368-371` (`Warn("policy filter: no API server", ...)`
+ `PolicyReject`) and `peer_initial_sync.go:718-722` (`Warn("... no reactor API -- fail-closed")`
+ `return false`). Concretely: `runEgressPolicyChain`/`runEgressPolicyChainASN4`
(`filter_ordered.go:196`/`:222`) return `egressStepResult{}` (accept == false) after a Warn;
`exportFilterForBody` (`egress_inject_filter.go:43`) returns `(true, nil)` (suppress) after a
Warn. Rationale: an export chain is a guard whose purpose is to reject; `r.api == nil` with
filters configured is a miss; `accept: true` / `(false, nil)` is the zero-value trap
(`fail-closed-guards.md`). The house answer for this exact condition already exists twice and
is deny+Warn; the fix makes the outlier agree, not invent a third behavior. Thomas: override
if wrong.

**→ AUTONOMOUS DEFAULT (2026-07-17): `facts == nil` and empty-filters are legitimate ACCEPTS,
not misses; split them OFF the `r.api == nil` branch.** The three conditions fused at
`egress_inject_filter.go:43` (`facts == nil || len(facts.exportFilters) == 0 || r.api == nil`)
are not the same case. `peer_forward_facts.go:35` documents the contract verbatim: the facts
pointer is "Stored via atomic.Pointer on Peer; **nil means not established.**" A not-established
peer has no session on which a route reaches the wire, and no known export policy to run:
that is an absent precondition, not a guard miss, so it keeps its accept. `len(exportFilters) == 0`
is "no export policy configured", also a legitimate accept. Only `r.api == nil` while facts is
present and filters are non-empty is the miss. The fix gives each its OWN early return so a
genuine api-miss can never be masked as "not established." This is the spec's own recorded
insight ("no filters configured is a legitimate accept; filters configured but no API server is
a miss") applied to all three fused terms. Thomas: override if wrong.

**→ AUTONOMOUS DEFAULT (2026-07-17): the ingress guard (`filter_ordered.go:139`,
`runIngressPolicyChain`) gets the SAME fail-closed treatment; it does NOT differ.** AC-4's
"same treatment, or a documented reason it differs" is answered: same. Split the fused
`len(filters) == 0 || r.api == nil`; on `r.api == nil` with import filters configured, drop the
route (accept == false) + Warn. Rationale: an import filter is equally a guard (it can be
security/ACL policy); silently accepting unfiltered inbound routes when the filter engine is
absent is the identical fail-open, and the sibling `filter_chain.go:368-371` is direction-agnostic
(it already serves both import and export via `policyFilterFunc`). This is the smaller,
self-contained option (an identical one-line split), satisfying R-3. Thomas: override if wrong.

**→ AUTONOMOUS DEFAULT (2026-07-17): `SetPluginServerAny` (`reactor.go:574-577`) LOGS LOUDLY on
a failed type assertion; the signature does NOT change.** AC-3 resolved: on
`s.(*pluginserver.Server)` failing, emit `reactorLogger().Error(...)` naming the received type
instead of the silent no-op that leaves `r.api` nil. Rationale: `fail-closed-guards.md` "make the
miss explicit at the producer": the producer of a nil api is this method, so it must speak here
rather than rely on the downstream guards (which now also fail closed, giving defense in depth).
Chosen over changing the method to `error`-returning: that is the larger, caller-touching change
(the sole caller is `register.go:145`), and logging + the now-fail-closed guards already closes
the leak. The "make the scrub not depend on `r.api`" alternative from the parent's ruling is
rejected as out of scope: remove-private-as is an EXTERNAL plugin (`filter_remove_private_as`)
reached only through the plugin server, so it cannot run with a nil api by construction. Thomas:
override if wrong (return `error` instead of log if you want the wiring bug to hard-fail startup).

**Test-feasibility note (grounds the TDD table below).** Asserting the "or say something" Warn
is feasible and has a precedent in this package: `TestSignalPeerAPIReadyUnknownPeerWarns`
(`api_sync_test.go`) captures logs via `slog.SetDefault(slog.New(rec))` with a `warnRecorder`
and `t.Cleanup`, because `reactorLogger` is `slogutil.LazyLogger("bgp.reactor")` (`reactor.go:80`)
and routes through the slog default. The new fail-closed tests mirror that helper.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEgressPolicyChainNilAPIWithExportFiltersFailsClosed` | `internal/component/bgp/reactor/filter_ordered_test.go` (new) | `runEgressPolicyChainASN4`/`runEgressPolicyChain` with export filters + nil api return `accept == false` (suppress) and Warn, not `accept: true` (AC-1) | |
| `TestRunIngressPolicyChainNilAPIWithImportFiltersFailsClosed` | `internal/component/bgp/reactor/filter_ordered_test.go` (new) | `runIngressPolicyChain` with import filters + nil api returns `accept == false` and Warn, not accept (AC-4) | |
| `TestExportFilterForBodyNilAPIWithExportFiltersSuppresses` | `internal/component/bgp/reactor/egress_inject_filter_test.go` (new) | `exportFilterForBody` with facts present, export filters non-empty, nil api returns `(suppress=true, nil)` and Warn; and returns `(false, nil)` when `facts == nil` (not established) or filters empty (AC-1, AC-2, fused-condition split) | |
| `TestSetPluginServerAnyWrongTypeLogsAndDoesNotSilentlyLeaveNil` | `internal/component/bgp/reactor/reactor_test.go` | `SetPluginServerAny` handed a non-`*pluginserver.Server` emits an Error log (captured via the `warnRecorder`/`slog.SetDefault` pattern from `api_sync_test.go`) instead of a silent no-op (AC-3) | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N/A -- no user-facing surface; `.ci` deferred to a trigger, not skipped | n/a | see justification below | N/A |

**→ AUTONOMOUS DEFAULT (2026-07-17): N/A, and this is the Wiring Test opt-out, not a gap.**
There is no config/CLI/operator-visible feature to drive from a `.ci`; the subject is an
internal guard on `r.api == nil`, a state the reachability table shows is not reachable
through any `.ci` entry point (borrow mode hard-fails at `reactor.go:1165`; standalone is
closed by the `r.mu` barrier). A `.ci` that cannot construct the precondition would be
theatre. This inherits the Wiring Test section's standing trigger verbatim: **if design finds
a real entry point that reaches the nil-api state, this row MUST become a concrete `.ci`**,
because that discovery would also mean the leak is live and A-1 is broken. Rationale:
`fail-closed-guards.md` "drive the guard from the entry point that triggers it": here the
triggering entry point is a Go-level state, so the Go unit tests above ARE that entry point.
Thomas: override if wrong.

### Interop Tests
N/A — no wire protocol change (the point is that routes stop reaching the wire unfiltered). Justify at closure.

## Files to Modify
- `internal/component/bgp/reactor/filter_ordered.go` - `:139`, `:196`, `:222`: split `len(exportFilters) == 0` from `r.api == nil`
- `internal/component/bgp/reactor/egress_inject_filter.go` - `:43`: same, plus the fused `facts == nil`
- `internal/component/bgp/reactor/reactor.go` - `:576`: the silent type-assertion failure
- `internal/component/bgp/reactor/forward_update_test.go`, `forward_split_test.go` - the nil-api literals (R-1)
- **(new, 2026-07-17)** `internal/component/bgp/reactor/filter_ordered_test.go` - RED-first fail-closed tests for the egress and ingress guards (file does not exist yet)
- **(new, 2026-07-17)** `internal/component/bgp/reactor/egress_inject_filter_test.go` - RED-first fail-closed test for `exportFilterForBody`, incl. the `facts == nil`/empty-filters accept split (file does not exist yet)
- `internal/component/bgp/reactor/reactor_test.go` - **(exists)** add `SetPluginServerAny` wrong-type log assertion (AC-3)

## Implementation Steps

### Implementation Phases

1. **Phase 1: Wiring (MANDATORY FIRST)** — write the RED tests from the Wiring Test table:
   a peer with export filters and a nil api must not send unfiltered. They will pass
   trivially today (the fail-open accepts), so each needs a mutation proof that it actually
   observes the wire, not just the return value.
   - Files: `internal/component/bgp/reactor/filter_ordered_test.go`
   - Verify: tests fail for the RIGHT reason -- the route reaches the wire unfiltered
2. **Phase 2: Split the fused condition** — `len(exportFilters) == 0` (a legitimate accept)
   and `r.api == nil` (a miss) stop sharing a branch, at `:196`, `:222`, and
   `egress_inject_filter.go:43` (which also fuses `facts == nil`).
   - Verify: Phase 1 tests go GREEN; AC-2 (no filters + nil api still accepts) stays GREEN
3. **Phase 3: Agree with the siblings** — deny or speak, matching
   `filter_chain.go:368-371`'s `Warn` + `PolicyReject`. Do not invent a third behavior for
   the same condition in the same package.
4. **Phase 4: The ingress guard** (`filter_ordered.go:139`) and `SetPluginServerAny`
   (`reactor.go:576`) — same treatment, or a documented reason each differs.
5. **Phase 5: The test fallout** (R-1) — the ~10 `&Reactor{}` literals with nil api were
   passing *because* of this bug. Each gets a real api or an explicit no-filters setup.
   This is expected, not collateral.
6. **Full verification** → `make ze-verify`
7. **Complete spec** → audit tables, learned summary, two-commit closure

### Failure Routing
| Failure | Route To |
|---------|----------|
| Phase 1 test passes before the fix | The test is not observing the wire. Fix the test, not the code |
| A nil-api unit test goes red in Phase 5 | Expected (R-1). Give it a real api; do NOT restore the fail-open |
| Fixing `:196` alone leaves the leak | You missed `:222` -- the shared body is what the originated path reaches |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
- [ ] Every `api == nil` site in the reactor package re-grepped at the end (AC-6)

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only

## Known Limitations
- Reachability for reactors constructed outside `StartWithContext` is not established (A-2).
- This spec does not fix the inverted bind/start ordering at `reactor.go:1006-1017`; it only records that the `r.mu` barrier is load-bearing and undocumented (R-2).
