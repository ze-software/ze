# Spec: fixit-load-dependent-functional-failures

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 2/2 |
| Updated | 2026-07-24 |

> **Final scope (2026-07-24):** this spec now covers ONLY the two completed, validated,
> reviewed fixes — **Phase 1 (Class B harness load-resistance)** and **345 (router-id
> opt-in)**. The **forward-path family (372/378/394/351)** was carved into
> `plan/spec-fixit-bgp-egress-rail-divergence.md` (owner decision: it is a spec-sized new
> primitive, not a redirect) and recorded in `plan/deferrals/fixit-load-dependent-functional-failures.md`.
> AC-2..AC-5 below are therefore DEFERRED to that spec; AC-1 and AC-6 are the scope here.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md`, `ai/rules/no-parking.md`, `ai/rules/flaky-under-load.md`
3. `plan/learned/1161-bgp-export-filter-applied-twice.md` (the 372 double-apply prior fix)
4. `plan/known-failures/reload-transaction-tests-load-sensitive.md` (owner-confirmed harness-deadline class)
5. Reproduction captures: `tmp/stress-repro/bgp-plugin-{372,378,394,345}-*.log`

## Task

A `make ze-verify` run surfaced 17 functional-test failures "under load". Investigation
(stress-repro.py, 32 burners / 16 cores) shows they are **two distinct problems**, not one
flaky-timeout class:

**Class A — real BGP concurrency bugs the tests correctly catch (must NOT be timeout-widened):**
- `372` remove-private-as-replace-peer: forwarded route's export chain applied **twice** →
  ze's own local AS `65000` rewritten to peer AS `65002` (`AS_PATH [65002 64496 65002 64497]`
  vs expected `[65000 …]`). **Regression of `plan/learned/1161`** (the `writeUpdate` /
  `writeUpdatePreFiltered` split).
- `378` rfc7606-relay-one-field: a **duplicate** announce frame reaches the receiver under
  load (already flagged in the test header as an un-investigated defect).
- `394` role-otc-egress-filter / `395` role-otc-egress-stamp: a **spurious `WITHDRAWN`**
  reaches the dest peer (racy: 1 pass / 1 fail across two stress invocations).
- `351` redistribute-l2tp-multi-peer-nexthop: same multi-peer-forward mismatch shape.
- `345` redistribute-as112-announce: one session receives a **NOTIFICATION (OPEN error,
  subcode 3 "Bad BGP Identifier")** and never establishes, so the observer's
  "both peers established" wait times out.

**Class B — genuine harness fixed-deadline-under-starvation flakes (SHOULD be made load-resistant):**
- `253` lg-ui-pages, `160` update-serve, `162/163/164` web-*: `http-check-failed` because the
  `http=wait` inner timeout (15s/10s) is NOT widened by the parallel headroom the outer budget
  already gets.
- `71/100/102/150/151` ui: `timeout` under the full 22-suite peak (71 would NOT reproduce in
  60 stress runs → pure starvation, no bug).
- `366` reload-listener-rejected: `exit-code-mismatch` (observer fixed polls / bind deadline).

Goal: make Class B load-resistant by scaling the fixed inner readiness gates with the SAME
`withParallelHeadroom` already applied to the outer per-test budget and engine-steps; and FIX
each Class A race at source with regression-hardening tests. Never widen a Class A test to green.

## Required Reading

### Architecture Docs
- [ ] `plan/learned/1161-bgp-export-filter-applied-twice.md` — the 372 double-apply, prior fix
  → Constraint: forwarded routes MUST write via `writeUpdatePreFiltered` (gate=false); any path
     that sends a forwarded/already-EBGP-prepended wire through the GATED write re-applies the
     export chain. The exemption is a property of the CALLER (has this wire been filtered?).
- [ ] `plan/known-failures/reload-transaction-tests-load-sensitive.md` — Class B, owner-confirmed
  → Constraint: the fixed 5s bind deadline (`peer_contract.go:169`, `runner_exec.go:893`) and
     kin are "harness giving a process a fixed wall-clock budget on an oversubscribed machine".
     Owner confirmed this class is planned work; fix at the deadline, not the test.
- [ ] `ai/rules/flaky-under-load.md` — stress-repro is the reproduction tool; static-clear before trusting

**Key insights:**
- The runner ALREADY widens the outer per-test budget 3× under parallelism
  (`ParallelTimeoutHeadroom = 3`, `parallel.go:28`; applied `runner_exec.go:377`) and engine-steps
  (`runner_exec_util.go:141`), but NOT the fixed inner readiness gates. Class B is that gap.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/test/runner/runner_exec.go` — bind barrier `context.WithTimeout(testCtx, 5*time.Second)`
  (:893), `waitReady(testCtx, …, 5*time.Second)` (:915, :978); outer timeout widened at :377.
  → Constraint: `withParallelHeadroom` is a `*Runner` method; both sites have `r` in scope.
- [ ] `internal/test/runner/runner_exec_util.go` — `withParallelHeadroom` (:127), `engineStepsForRun` (:141)
- [ ] `internal/test/runner/runner_validate.go` — `executeOneHTTPCheck` retry budget `maxRetries=20 × 200ms`
  (:579), client `Timeout: 5s` (:565); `executeOneHTTPWait` default 15s / `chk.Timeout` (:697).
- [ ] `internal/test/runner/parallel.go` — `ParallelTimeoutHeadroom = 3` (:28)
- [ ] `internal/component/bgp/reactor/session_write.go` — `writeUpdate`/`writeUpdatePreFiltered`/
  `writeUpdateGated` (:246-315); the egress gate at :276-289.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` — `forwardUpdateCore` (:249),
  `AnnounceEOR` "no established peers to send to" (:70).

**Behavior to preserve:**
- Serial runs (`-p 1`, single-test debug) keep the TIGHT authored deadlines so real slowdowns
  surface fast — `withParallelHeadroom` returns the value unchanged when `concurrency <= 1`.
- Wire correctness of forwarded routes (exactly-once export chain) — Class A fixes must not
  change on-wire output for the passing (unloaded) case.

**Behavior to change:**
- Class B: fixed inner readiness gates gain the parallel headroom.
- Class A: eliminate the load-triggered races (double-apply, duplicate/spurious frame, OPEN race).

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | Class B fix (headroom on inner gates) cannot mask Class A | Class A fails on wire CONTENT, not a deadline; daemon-internal timing is unchanged by test deadlines | Would hide real bugs | stress-repro of a Class A test stays RED after the Class B change | unvalidated |
| A-2 | 372's double-apply is the same class as learned/1161, reached via a different load path | reproduced AS_PATH corruption identical to 1161's description | fix misdirected | read the forward/initial-sync write-path selection; `--race` capture | unvalidated |
| A-3 | 378/394/395/351 share one forward-path root (duplicate/ordering during establishment) | same multi-peer forward shape, same "no established peers" EOR log | separate fixes needed | per-test reproduction + trace | unvalidated |
| A-4 | 345 is an OPEN-processing / BGP-ID collision race, independent of the forward-path races | NOTIFICATION OPEN subcode 3 on one of two same-BGP-ID sessions | separate fix | trace OPEN validation + collision detection | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|-----------|
| R-1 | Widening inner gates hides a genuinely slow/hung path | a Class B test starts passing that used to catch a real hang | keep serial deadlines tight; headroom only under concurrency |
| R-2 | Class A fix introduces a new reactor race | `make ze-race-reactor` red, or stress-repro red post-fix | -race + stress-repro green gate on every reactor change |
| R-3 | 395/351 don't reproduce → fixing a phantom | stress-repro "not reproduced" | reproduce each before claiming its fix; if unreproducible + statically clear, say so (flaky-under-load.md) |

## Data Flow (MANDATORY)

### Entry Point
- Class B: a `.ci` test's readiness gate (peer bind, `daemon.ready`, HTTP server listen) evaluated
  inside the parallel runner (`internal/test/runner/`).
- Class A: a BGP UPDATE received from a route-server client (conn=1) forwarded to a second peer
  (conn=2) that is establishing concurrently.

### Transformation Path
1. Class B: resolved outer timeout → `withParallelHeadroom` (3× when parallel) → `testCtx`. The
   inner gates currently take a RAW fixed duration instead of the headroomed one — the defect.
2. Class A: wire → `received_update` → `forwardUpdateCore` (export chain + EBGP prepend on the
   ORIGINAL wire) → forward pool → session write. Correct forwarded write is
   `writeUpdatePreFiltered` (gate=false). The load race sends an already-filtered wire through the
   GATED write (`writeUpdate`) or emits it twice.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Runner ↔ child process | fixed wall-clock deadlines (bind, ready, HTTP) | [ ] |
| forwardUpdateCore ↔ session write | pre-filtered vs gated write selection | [ ] |

### Integration Points
- `withParallelHeadroom` (`runner_exec_util.go:127`) — the single scaling primitive to reuse.
- `writeUpdatePreFiltered` / `writeUpdateGated` (`session_write.go:250-289`) — the exactly-once gate.

### Architectural Verification
- [ ] No bypassed layers (inner gates reuse the existing headroom primitive, not a new one)
- [ ] No duplicated functionality (Class A fix hardens the existing exactly-once path)

## Wiring Test (MANDATORY)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| parallel runner resolves an inner gate deadline | → | `withParallelHeadroom(fixed)` | `TestWithParallelHeadroomInnerGates` |
| forwarded route to a concurrently-establishing peer | → | exactly-once export chain | stress-repro `bgp plugin 372` green |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | parallel run (`concurrency>1`) | bind barrier, `waitReady`, `http=wait` timeout, and HTTP-check retry budget are each scaled by `ParallelTimeoutHeadroom`; serial runs keep the tight value |
| AC-2 | 372 under stress-repro (40 iters) | export chain applied exactly once; AS_PATH `[65000 64496 65002 64497]`; 0 reproductions |
| AC-3 | 378 under stress-repro | receiver sees each field exactly once; no duplicate announce; 0 reproductions |
| AC-4 | 394 / 395 under stress-repro | dest peer sees only the EOR (no spurious withdraw); 0 reproductions |
| AC-5 | 351 under stress-repro | multi-peer next-hop forward matches; 0 reproductions |
| AC-6 | 345 under stress-repro (60 iters) | both sessions establish; no spurious OPEN NOTIFICATION; observer passes; 0 reproductions |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestWithParallelHeadroomInnerGates` | `internal/test/runner/runner_exec_util_test.go` | headroom applied to inner gate durations under concurrency, identity when serial | |
| (Class A: prefer the existing reactor unit tests + the `.ci` regressions; add targeted unit tests where a pure function is implicated) | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| existing `.ci` (372/378/394/395/351/345) | `test/plugin/` | pinned by the Class A fixes; must be stress-repro-green | |

## Files to Modify
- `internal/test/runner/runner_exec.go` — headroom on bind barrier + waitReady (Class B)
- `internal/test/runner/runner_validate.go` — headroom on http-wait + http-check retry budget (Class B)
- `internal/test/runner/runner_exec_util.go` — small factor helper if needed (Class B)
- `internal/component/bgp/reactor/…` — Class A fixes (files TBD per root cause)

## Implementation Steps

1. **Phase 1 — Class B harness load-resistance.** Scale the fixed inner readiness gates (bind
   barrier, waitReady, http-wait, http-check retry budget) through `withParallelHeadroom`. Unit
   test for the scaling. Verify serial deadlines unchanged.
2. **Phase 2 — 372 double-apply.** Root-cause the load path where a forwarded route reaches the
   GATED write; fix at source; stress-repro green; `make ze-race-reactor`.
3. **Phase 3 — 378/394/395/351 forward duplicate/spurious frame.** Root-cause the shared
   establishment/forward race; fix; stress-repro each green.
4. **Phase 4 — 345 OPEN / BGP-ID establishment race.** Root-cause; fix; stress-repro green.
5. **Phase 5 — verification.** `make ze-verify`; targeted stress-repro of every fixed test.
6. **Phase 6 — review + closure.** Independent `/ze-review`; learned summary; two-commit close.

## Design Insights (VERIFIED root causes)

### Class A forward-path family (372/378/394/351) — egress-rail divergence, NOT a learned/1161 regression
A route received from peer A destined for peer B can reach B by **two rails** that transform egress differently; under concurrent establishment the buggy one fires:
- **Forward rail (correct):** `forwardUpdateCore` runs the export policy on the RECEIVED wire, THEN prepends local AS, writes PRE-FILTERED. Order: filter → prepend. (`reactor_api_forward.go:493-547`; `forward_pool.go:186`, `forward_rs.go:71` use `writeUpdatePreFiltered`.)
- **Replay rail (buggy):** on peer-up, adj-rib-in replays the stored RAW route as `update hex … add` (`rib.go:563-572` → `formatHexCommand:774-778`). The announce builder prepends local AS FIRST (`reactor_api_batch.go:396-454`), THEN the session write gate runs ONLY `facts.exportFilters` on the already-prepended wire (`egress_inject_filter.go:43-91`, esp. `:76`, SEND-context `:66`), and NOT the in-process role/OTC/community filters (`role/register.go:22-31` registers OTC via `filterapi`, not `facts.exportFilters`). Order: prepend → filter, and an INCOMPLETE filter set.
- **372:** replay rail prepends 65000 → `[65000 64496 64512 64497]`, then remove-private-as REPLACE on the prepended wire rewrites private 65000 → peer-AS 65002 → `[65002 64496 65002 64497]`. (Forward rail keeps 65000 because it filters first.)
- **378:** no export filter → gate is a no-op → replay bytes equal forward bytes → DUPLICATE announce, amplified by a DOUBLE replay trigger (adj-rib-in self-replay `rib.go:563-572` AND bgp-rs `request bgp adj-rib-in replay` `server_handlers.go:149-208`).
- **394:** adj-rib-in + bgp-role only; OTC egress is an in-process `filterapi` step run only in the forward rail's `orderedEgressSteps`, so the replay rail never suppresses the OTC route. (The exact bare-withdraw producer is UNVERIFIED — hypothesis.)
- **Fix fork (owner's design call):** (a) route peer-up replay through the FORWARD rail (`ForwardCached`/`forwardUpdateCore`) so relayed routes use one egress transform; or (b) make the announce-rail egress IDENTICAL to the forward rail (filter-before-prepend + include the in-process filters). Plus dedupe the double replay trigger. Interacts with learned/1161, /1231 (private-ASN leak, the reason the gate exists), /1245.
- **Owner chose (a). Mechanism investigation (VERIFIED) found (a) is a NEW PRIMITIVE, not a redirect:**
  - `ForwardCached`/`ForwardUpdatesDirect`/`forwardUpdateCore` are all keyed to the `recentUpdates` cache (`reactor_api_forward.go:177,249`, `reactor_api_forward_batch.go:53,111`). `forwardUpdateCore` CAN target a single peer (explicit `[]*Peer`), but adj-rib-in routes are NEVER cache-resident (cache is consumer-ack + 5-min valve, `recent_cache.go:25-67`; a peer connecting later cannot have the route cached). So "point replay at ForwardCached" is INFEASIBLE.
  - Feasible only via a new reactor primitive `RelayStoredRoute(source, dest, fam, attrs, nhop, nlri)` on `ReactorCacheCoordinator` (`types_bgp.go:361`): reconstruct received-shape `ReceivedUpdate` (synthesize MP_REACH per family for MP), set `SourceCtxID` from the source peer's `recvCtxID` (`peer.go:512`), Add+Activate into `recentUpdates` for buffer lifecycle, resolve `srcInfo`, call `forwardUpdateCore` with the single dest. New SDK method + RPC mirroring `ForwardCached` (`sdk_engine.go:69`, `dispatch_cached.go:24`, `bridge.go:452`). adj-rib-in `buildReplayCommands` (`rib.go:750`) yields `(sourcePeer, *RawRoute)` instead of hex strings; its 3 consumers call the primitive.
  - **Dedupe (378 amplifier, orthogonal):** bgp-rs owns replay when present (it needs the synchronous completion signal for EOR, `server_handlers.go:212-263`); adj-rib-in gates its self-replay branches (`rib.go:563-571,736-743`) on a `replay-owner` flag rs sets at startup. When rs absent, adj-rib-in self-replays (its only trigger).
  - Sub-gaps: add-path path-id lost on structured ingest (`installStructuredNLRIs`, `rib.go:314-323`) vs legacy (`prefixToWireHex`, `rib.go:861-868`); UNVERIFIED keystones: MP_REACH presence in `AttrsWire.Packed()`, `WriteAnnounceUpdate` prepend order.
  - **Magnitude:** this is its own spec-sized effort (new plugin-protocol primitive + per-family MP synthesis + add-path fix + dedupe + interop test). Larger than the harness/345 fixes. Owner to confirm scope vs option (b)'s smaller "keep two rails in sync" change.

### Class A 345 — router-id RFC 6286 policy race
`checkRouterIDConflict` (`routerid_unique.go:53-85`) rejects (OPEN subcode 3) any OTHER same-AS peer already Established with the same BGP Identifier — a check-then-act race (fires only when the other peer establishes first). RFC 6286 §2.2 permits subcode 3 ONLY for a zero identifier or a local-match on an internal peer; §2.1 makes AS-wide uniqueness a SHOULD; §2.3 resolves shared IDs via collision detection, not rejection. Fixing this REVERSES a deliberate, tested feature (`routerid_unique_test.go`, 342 lines, 10 tests) → owner decision.

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| 372 is a regression of learned/1161 (forward-pool double-gate) | Both forward-pool halves correctly use `writeUpdatePreFiltered`; 372 is the adj-rib-in REPLAY rail prepend-then-filter, a distinct defect | forward-path agent traced the replay rail; verified at `egress_inject_filter.go:76`, `rib.go:563-572`, `role/register.go:22-31` | Fix targets the replay/egress-rail divergence, not the forward pool |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Split the failures into two classes; fix Class B at the deadline and Class A at source | Widen all timeouts to green | Widening Class A hides data corruption (no-parking, no-workarounds); the tests are correct |
| Forward-path family: route adj-rib-in peer-up REPLAY through the forward rail (`forwardUpdateCore`/`ForwardCached`), one egress transform; dedupe the adj-rib-in + bgp-rs double replay trigger | (b) make the announce-rail egress identical (keep two rails in sync) | Owner decision 2026-07-24: a replay IS relaying, not originating; one egress rail is the principled fix (single source of truth for filter-then-prepend + in-process filters) |
| 345: keep reject-duplicate-router-id as DEFAULT; add a config option to opt into ACCEPTING a shared BGP Identifier (345/AS112 enables it) | (a) relax unconditionally per RFC 6286 §2.2 (delete feature+tests); (c) fix test only | Owner decision 2026-07-24 ("default to reject, make it an option to accept"): preserves the deliberate uniqueness feature + its 342-line test, fixes the test via opt-in, gives operators control. RFC 6286 §2.1 makes uniqueness a SHOULD, so both default-reject and opt-in-accept are conformant |

## Review Gate

Two independent adversarial reviewer subagents over the diff (`tmp/review-diff.patch`), distinct
lenses (harness masking-risk/completeness; 345 fail-closed/config-plumbing/RFC-conformance).
**Neither found a BLOCKER.**

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | foreground `daemon.ready` wait left unwidened (sibling of the widened background one) | runner_exec.go:986 | fixed — widened via `withParallelHeadroom` |
| 2 | ISSUE | `await=stderr` fence (10s hard-fail) blows before the widened outer budget | await_stderr.go:83 | fixed — `awaitDaemonStderr` now a `*Runner` method, timeout widened |
| 3 | ISSUE | non-orchestrated peer bind barrier left unwidened (named in known-failures doc) | runner_exec.go:143 | fixed — widened + dynamic timeout in the error message |
| 4 | NOTE | comment overstated coverage (two bind barriers, two ready waits) | runner_exec_util.go | fixed — comment now enumerates all widened gates |
| 5 | NOTE | `ze bgp decode` fork (5s, unbounded by testCtx) + per-request HTTP client (5s) unscaled | runner_validate.go:504,565 | fixed — both widened |
| 6 | ISSUE | comments claim RFC 6286 §2.3 collision detection ze does NOT implement | peer.go, reactor.go, ze-bgp-conf.yang | fixed — reframed to §2.1 SHOULD; state ze runs no check when opted in |
| 7 | NOTE | stale sibling comment: `checkRouterIDConflict` still asserts RFC 4271 §4.2 MUST | routerid_unique.go:46 | fixed — reframed to RFC 6286 §2.1 SHOULD + opt-out |
| 8 | NOTE | YANG placement couples opt-in to mandatory `asn/local` sibling | ze-bgp-conf.yang | kept under `bgp/session` (validated; ze walker lenient — coupling theoretical); top-level move fought the `.ci` guard for no real gain. Known limitation. |
| 9 | NOTE | docs gap: new config leaf + RFC 6286 support | docs/ | config leaf documented in `docs/guide/configuration.md` (with source anchor); rfc-status.md row + `rfc/short/rfc6286.md` summary tracked as follow-up (RFC 6286 support is partial — see learned summary) |
| 10 | NOTE | RFC 6286 §2.2 zero/self-id reject unimplemented (PRE-EXISTING, not a regression); default-path race unchanged for non-opted-in duplicate configs | peer.go / session_handlers.go | acknowledged as known limitations in the learned summary; not introduced here |

### Fixes applied
- Phase 1 completeness: widened every structurally-identical inner readiness gate (both bind
  barriers, both `daemon.ready` waits, the `await=stderr` fence, the `ze bgp decode` fork, the
  HTTP wait/retry/client budgets); corrected the coverage comment.
- 345: removed the §2.3 collision-detection overclaim from all three comment sites; fixed the
  stale `checkRouterIDConflict` MUST comment; added the config-leaf doc.

### Final status
- [x] Fixes re-verified: `make ze-lint-changed` 0 issues; `make ze-doc-test` PASSED; runner
  headroom tests + 345 unit tests pass; session-placement config validates; 345 stress 0/80.
- [x] All findings BLOCKER/ISSUE fixed; NOTEs recorded (8, 9, 10 as documented known limitations / follow-up).

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| internal/test/runner/runner_exec_util.go (parallelFactor) | yes | `grep -n "func (r \*Runner) parallelFactor"` → :127 |
| internal/test/runner/runner_exec.go (both bind + both ready widened) | yes | `grep withParallelHeadroom(5` → :143(bind), :923(ready bg), :986(ready fg), :898(bind orch) |
| internal/test/runner/await_stderr.go (method + widen) | yes | `grep "func (r \*Runner) awaitDaemonStderr"` present |
| internal/test/runner/runner_validate.go (decode + http client + retry widened) | yes | `grep parallelFactor / withParallelHeadroom` present |
| internal/component/bgp/reactor/reactor.go (AllowSharedRouterID) | yes | field present in Config |
| internal/component/bgp/reactor/peer.go (validateOpen gate) | yes | `if !r.config.AllowSharedRouterID` at :675 |
| internal/component/bgp/config/loader_create.go (session extraction) | yes | reads `bgp/session/allow-shared-router-id` |
| internal/component/bgp/yang/ze-bgp-conf.yang (leaf) | yes | leaf under global `session` |
| test/plugin/redistribute-as112-announce.ci (opt-in) | yes | `session { allow-shared-router-id true }` |
| docs/guide/configuration.md + ai/CODE-TO-DOCS.md | yes | config note + regenerated index (doc-test PASSED) |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | inner gates scale with parallel headroom, tight serial | `TestParallelFactor` + `TestWithParallelHeadroom` PASS; all sibling gates widened (review Run 1 ISSUEs 1-3,5 fixed) |
| AC-6 | 345 both sessions establish; no spurious OPEN NOTIFICATION; 0 reproductions | stress-repro `bgp plugin 345` **not reproduced in 80** (was ~1/8); unloaded PASS; `TestValidateOpenAllowSharedRouterID` PASS (default rejects, opt-in accepts); session config validates |
| AC-2..AC-5 | forward-path family | DEFERRED → `plan/spec-fixit-bgp-egress-rail-divergence.md` (not in this spec's scope) |

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Completion (BLOCKING — before ANY commit)
- [ ] Every AC stress-repro-green
- [ ] `make ze-race-reactor` green for every reactor change
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Independent review clean
- [ ] Learned summary written
