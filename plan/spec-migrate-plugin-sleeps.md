# Spec: migrate-plugin-sleeps

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | payload-predicate-waits (Layer 2 primitives; committed, learned 1120) |
| Phase | 8/8 |
| Updated | 2026-07-22 |

Phase corrected 2026-07-22: the 214-sleep migration is committed (`edfe4c0e1`,
test/plugin 305 -> 91 per the Implementation Summary) and the residue is
explicitly handed to `spec-fixit-migrate-sleeps-infra`. Awaiting closure per
`ai/rules/planning.md` Spec Closure (learned summary + two-commit sequence).

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file.
2. The durable worklist: `<scratchpad>/MIGRATION-WORKLIST.md` (every convertible file:line + recipe + status; DEFER/KEEP lists). If lost, re-derive per-file by reading each `.ci`.
3. `test/scripts/ze_api.py` primitives: `dispatch_until`(:951) `dispatch_until_done`(:969) `wait_until`(:983) `wait_for_event`(:999) `wait_for_events`(:1043) `quiesce`(:1182) `wait_for_ack`(:1207) `wait_for_post_startup`(:1275).
4. `test/plugin/prefix-filter-accept.ci` — the converted reference (inbound dispatch_until).
5. `test/.ci-sleep-baseline` (currently 460 via Layer 2, uncommitted in the shared tree).

## Task

Migrate the ~225 convertible `time.sleep()` calls in `test/plugin/*.ci` (of 305 total there;
460 baseline across all `test/**/*.ci`) to deterministic waits using the existing Layer 1
(`quiesce`/`wait_for_ack`, outbound barrier) and Layer 2 (`dispatch_until`/`wait_until`/
`wait_for_event`, payload predicates) primitives. Blind fixed sleeps are replaced by waits
that return exactly when the awaited state is observed, removing timing flakiness and
lowering the ci-sleep ratchet.

Scope confirmed by user (2026-07-14): "Full convertible campaign in one spec" — convert all
~225 D+Q+U+E cases (including the careful anchor-then-quiesce and RPKI-event cases). Ratchet
target 460 -> ~235. DEFER the ~32 that need new infra/redesign; KEEP the ~40 intentional /
no-`ze_api` cases. A SEPARATE follow-on spec (user request, 2026-07-14) will convert the
DEFER/KEEP buckets by adding the missing infrastructure.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. Capture insights as -> Decision: / -> Constraint: annotations. -->
- [ ] `docs/architecture/testing/ci-format.md` — `.ci` directive + embedded-observer model
  -> Constraint: observer Python imports `ze_api`; the ci-sleep ratchet counts `time.sleep(` in `test/**/*.ci` only (sleeps inside `ze_api.py` are exempt), so polling helpers may sleep internally.
- [ ] `ai/rules/testing.md` — Observer API table + sleep-ratchet rule
- [ ] `ai/rules/no-test-deletion.md` — `# // test-relax:` token semantics
  -> Constraint: the `.ci` line-count hook (`.claude/hooks/pretool-writeedit.py:1648-1653`) blocks any edit reducing non-comment/non-`option=`/non-blank lines; a `// test-relax:` token in the new content exempts the edit AND creates the audit trail (`grep -rn 'test-relax:'`). Every conversion carries one.

### RFC Summaries (MUST for protocol work)
- [ ] N/A — test infrastructure, no wire-protocol behaviour change.

**Key insights:**
- The engine quiesce barrier is OUTBOUND-only (`reactor_api.go:891` FlushForwardPool, `:922` DrainPeerSync). Inbound route processing (peer sends UPDATE -> ze -> adj-rib-in) is NOT covered; inbound assertions MUST use `dispatch_until` on a `show`, not `quiesce`.
- DrainPeerSync DOES wait on a still-establishing peer that already has routes queued (they drain when it comes up), but returns immediately when nothing is queued yet — so an outbound test whose work is triggered by a not-yet-arrived inbound route needs a `dispatch_until` anchor first (vacuous-barrier race, R-1).
- Reject/negative tests drop the route so there is no positive edge to poll — they DEFER (need a fence-route or an event subscription).

## Current Behavior (MANDATORY)

**Source files read (this session, via 8 sub-agents + direct reads):**
- [ ] `internal/component/bgp/reactor/reactor_api.go` — FlushForwardPool(:891)/DrainPeerSync(:922) both outbound; peersSynced(:937) polls PendingSync.
  -> Constraint: no inbound/adj-rib-in drain exists in any quiescer.
- [ ] `internal/component/bgp/reactor/forward_pool_barrier.go:18` — fwdPool.Barrier drains route-forwarding workers keyed by fwdKey; control messages (KEEPALIVE/ROUTE-REFRESH) do NOT go through it.
  -> Constraint: `api-raw`/`api-route-refresh` (control-message flush) are NOT clean-quiesce; defer/verify.
- [ ] `test/scripts/ze_api.py` — all 8 primitives above exist (verified line numbers). `dispatch_until` predicate receives the inner RPC "result" dict; returns first-match-or-last.
- [ ] `test/plugin/prefix-filter-accept.ci` — converted reference; 3x green.
- [ ] 205 `test/plugin/*.ci` categorized (worklist). Counts: D=137, Q=48, U=29, E=11, H=10, S=26, K=40, UNSURE=3, DONE=1 (=305).

**Behavior to preserve:**
- Every converted test keeps its EXACT assertions (`expect=`/`reject=` lines, observer fatal checks). Only the WAIT mechanism changes.
- KEEP-bucket sleeps (deliberate timers, raw-UDP peer-driver pacing, standalone-driver log-tail backoffs) are unchanged — the delay is intentional or the script has no `ze_api`.

**Behavior to change:**
- Replace blind `time.sleep` with a deterministic wait in the ~225 convertible cases — user requested.

## Data Flow (MANDATORY)

### Entry Point
- A `.ci` observer (`tmpfs=*.run`) imports `ze_api` and calls a deterministic-wait primitive instead of `time.sleep`.

### Transformation Path
1. Observer calls `dispatch_until`/`wait_until`/`wait_for_event`/`quiesce` which polls the daemon over `dispatch-command` / event stream (internal sleeps ratchet-exempt) until the predicate/ barrier is satisfied.
2. Assertion proceeds exactly as before.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| observer <-> engine | existing `dispatch-command` RPC / event stream; no new RPC | [ ] |
| `.ci` <-> ratchet | removed `time.sleep(` lowers the count; baseline updated to match | [ ] |

### Integration Points
- No production `internal/`/`cmd/` change. Test-infra only: `.ci` files + `test/.ci-sleep-baseline`.

### Architectural Verification
- [ ] No bypassed layers (predicate evaluated where the old assertion ran).
- [ ] No unintended coupling (test infra only).
- [ ] No duplicated functionality (primitives already exist from Layer 2).
- [ ] Registration over hardcoding — N/A.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | `dispatch_until` on `show bgp adj-rib-in status` returns when an inbound route lands | prefix-filter-accept proof | inbound tests flake | proof passed 3x | **confirmed** |
| A-2 | `quiesce()` drains outbound forward-pool + peer opQueue reliably for announce/withdraw | 8 existing barrier tests | outbound tests flake | run each converted Q test 3x | pending |
| A-3 | The show commands in the worklist recipes exist and expose the asserted fields | sub-agent reads | predicate never matches | read each file + run at conversion | per-file |
| A-4 | ci-sleep ratchet passes while count < baseline; baseline lowered at commit | Layer 2 spec | ratchet blocks | run verify_wiring_docs after each batch | pending |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|-----------|
| R-1 | Vacuous quiesce (barrier runs before awaited inbound work) -> false green | outbound test passes even when inbound failed | anchor with `dispatch_until` before `quiesce()`; per-test verify 3x |
| R-2 | A recipe's predicate mismatches the real JSON shape -> timeout | test hangs to timeout | read the actual `show` output at conversion; verify field names |
| R-3 | Removing a sleep exposes a real race the sleep was masking | converted test flakes 1/3 | that IS a bug to surface (per feedback_sleep_hides_races); investigate, do not re-add sleep |
| R-4 | Shared `test/.ci-sleep-baseline` collides with Layer 2's uncommitted change | commit conflict | do NOT touch baseline until Layer 2 lands; reconcile at commit time |
| R-5 | Batch too large to verify -> unverified conversions | — | convert in batches, run each batch 3x before moving on |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| observer calls `dispatch_until` (inbound) | -> | `ze_api.dispatch_until` | every Batch-3 `.ci` (run 3x) |
| observer calls `quiesce`/`wait_for_ack` (outbound) | -> | `ze_api.quiesce` | every Batch-1/4 `.ci` (run 3x) |
| observer calls `wait_until` (listener/marker) | -> | `ze_api.wait_until` | every Batch-5 `.ci` (run 3x) |
| observer calls `wait_for_event` (rpki) | -> | `ze_api.wait_for_event` | every Batch-6 `.ci` (run 3x) |
| ratchet lowered | -> | `test/.ci-sleep-baseline` | `verify_wiring_docs.py` green |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Batch 1 (trivial redundant deletions, ~19) | sleeps deleted; each test 3x green; existing barrier covers |
| AC-2 | Batch 2 (dispatch_until_done readiness, ~26) | poll-loops -> `dispatch_until_done`; each 3x green |
| AC-3 | Batch 3 (inbound dispatch_until, ~80) | blind sleeps -> `dispatch_until(show, predicate)`; each 3x green; assertions unchanged |
| AC-4 | Batch 4 (outbound quiesce, ~40) | `quiesce`/`wait_for_ack`; anchor-then-quiesce where R-1 applies; each 3x green |
| AC-5 | Batch 5 (wait_until listener/marker, ~21) | external readiness -> `wait_until` probe; each 3x green |
| AC-6 | Batch 6 (wait_for_event rpki, ~11) | `wait_for_event(predicate)`; each 3x green |
| AC-7 | Batch 7 (deletable pads/cruft, ~19) | removed; each 3x green |
| AC-8 | Ratchet | `test/.ci-sleep-baseline` lowered to the true post-conversion count; `verify_wiring_docs.py` green |
| AC-9 | DEFER/KEEP documented | worklist DEFER (~32) + KEEP (~40) recorded with reasons; follow-on spec noted |
| AC-10 | Full suite | `bin/ze-test bgp plugin --all` green (no regressions) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path | Test |
|---|-----------|------|------|
| 1 | writes an observer waiting for an inbound route | `dispatch_until('show bgp adj-rib-in status', pred)` | prefix-filter-accept.ci |
| 2 | waits for outbound propagation | `quiesce()` | rib-withdrawal.ci |
| 3 | waits for a listener/marker | `wait_until(probe)` | rest-execute.ci |
| 4 | waits for an rpki event | `wait_for_event(pred)` | rpki-event-valid.ci |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| N/A | — | no Go code changes; primitives already unit-tested in Layer 2 | — |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Notes |
|-------|-------|-------|
| N/A | — | no new numeric inputs |

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| every converted `.ci` | `test/plugin/` | its own end-user scenario, wait mechanism swapped | per-batch, 3x each |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Notes |
|----------|-------|
| N/A | test infra, no wire change |

### Future (if deferring any tests)
- DEFER bucket (~32) + KEEP re-examination -> separate follow-on spec (user request). Needs: daemon-stderr-wait helper, fib/dataplane reflecting `show`, reject fence-route/event pattern, `wait_for_daemon_ready()` for non-`ze_api` drivers.

## Files to Modify

Test-infra only (the "feature code" — the Layer 1 barrier + Layer 2 predicate primitives —
already exists and is NOT modified here):
- ~225 test/plugin/\*.ci files (per worklist batches).
- The ci-sleep ratchet file test/.ci-sleep-baseline — lowered to the true post-conversion count (ONLY after Layer 2 lands; R-4).

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| Test infra docs | maybe | `docs/functional-tests.md` note that plugin tests use deterministic waits |
| Discovery | no | primitives already documented by Layer 2 |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File |
|---|----------|----------|------|
| 10 | Test infrastructure changed? | conversions only (primitives unchanged) | none required beyond a possible note |
| 16 | Changed source referenced by doc anchors? | no source changed | — |

## Files to Create
- None (follow-on spec created later per user request).

## Implementation Steps

### /implement Stage Mapping
| Stage | Section |
|-------|---------|
| Audit | worklist |
| Implement | Batches 1-7 |
| Verify | 3x per test + `bin/ze-test bgp plugin --all` |
| Close | ratchet + two-commit closure (after Layer 2 lands) |

### Implementation Phases
1. Batch 1 — trivial redundant deletions (safest).
2. Batch 2 — dispatch_until_done readiness swaps.
3. Batch 3 — inbound dispatch_until (largest).
4. Batch 4 — outbound quiesce (anchor-then-quiesce per R-1).
5. Batch 5 — wait_until listeners/markers.
6. Batch 6 — wait_for_event rpki.
7. Batch 7 — deletable pads/cruft.
8. Ratchet + full-suite verify + close.

### Critical Review Checklist (/implement stage 6)
| Check | For this spec |
|-------|---------------|
| Assertions preserved | every converted test keeps its `expect=`/fatal checks |
| R-1 non-vacuous | outbound quiesce has an inbound anchor where needed |
| R-2 predicate shape | verified against real `show` output |
| No sleep re-added to mask a race (R-3) | flakes investigated, not papered over |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification |
|-------------|--------------|
| Batches converted | `grep -c time.sleep` per file dropped; each 3x green |
| Ratchet lowered | `cat test/.ci-sleep-baseline`; `verify_wiring_docs.py` green |
| No regressions | `bin/ze-test bgp plugin --all` green |

### Security Review Checklist (/implement stage 11)
| Check | Notes |
|-------|-------|
| Input validation | test-only; predicates bounded by attempts/timeout |
| Resource exhaustion | polls are attempts-bounded (no unbounded loop) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Converted test flakes | investigate race (R-3); do not re-add sleep |
| Predicate timeout | fix predicate vs real output (R-2) |
| 3 fix attempts fail | mark file DEFER in worklist, move on, report |

## Checklist

Migration-adapted TDD (each converted `.ci` IS its own functional test; assertions preserved):
- [ ] Tests written — each converted test keeps its exact `expect=`/fatal assertions; only the wait mechanism changes (no new test authored, the existing one is the test).
- [ ] Tests FAIL — N/A as a red-first step: conversions start from green. A converted test that FAILS is not expected; if one does, it surfaces a real race the sleep was masking (R-3) and is fixed at the source, never by re-adding the sleep.
- [ ] Tests PASS — each converted test runs 3x green with the deterministic wait.
- [ ] make ze-test / `bin/ze-test bgp plugin --all` — full plugin suite green (no regressions) before close.

## Mistake Log
### Wrong Assumptions
| Assumed | True | Discovered | Impact |
|---------|------|------------|--------|
| Layer 3 = FIB/tc/listener quiescers is the sleep hotspot | FIB/tc/listener touch ~1-40 sleeps; the real hotspot is ~225 BGP-state plugin tests convertible with existing Layer 1+2 primitives | 4 feasibility agents + 4 categorization agents | redirected the effort from building quiescers to migration |

### Failed Approaches
| Approach | Why | Replacement |
|----------|-----|-------------|

### Escalation Candidates
| Mistake | Frequency | Rule |
|---------|-----------|------|

## Design Insights
<!-- LIVE -->
- Conversion recipe (inbound): replace `time.sleep(N)` + single `show`+assert with `api.dispatch_until('show ...', lambda r: result_json_data(r,{}).get(<field>) <cmp>)`; keep the following assert. Add `# // test-relax:` token; drop `import time` if now unused.
- Conversion recipe (outbound): replace with `api.quiesce()`; if a prior inbound arrival is required, `api.dispatch_until(<inbound show>, pred)` first (R-1).
- Conversion recipe (redundant): if a `wait_for_ack`/`dispatch_until_done` already follows on the next line, just delete the sleep (replace with the token comment so the hook passes).

## Core Insight
The sleep problem was never a missing barrier for FIB/tc/listeners — it was ~225 BGP-state
plugin tests using blind sleeps where the Layer 1 barrier and Layer 2 payload predicates
(already built) give a deterministic wait. This spec applies them.

## Key Design Decisions
| Decision | Alternatives | Rationale |
|----------|-------------|-----------|
| Migrate with existing primitives, don't build new quiescers | Build FIB/tc/listener quiescers (original "Layer 3") | verified: those cover ~1-40 sleeps at high build cost; the ~225 hotspot needs no new infra |
| Batch by primitive/risk | one big sweep | per-batch 3x verification catches per-test surprises early |
| Anchor-then-quiesce for outbound-after-inbound | bare quiesce | avoids vacuous-barrier false green (R-1) |

## Known Limitations
- ~32 DEFER cases need new infra (follow-on spec).
- ~40 KEEP cases are intentional (deliberate timers, no-`ze_api` drivers) — the follow-on spec re-examines which are genuinely convertible.

## RFC Documentation
N/A.

## Implementation Summary
### What Was Implemented
- Migrated the convertible `time.sleep()` calls in `test/plugin/*.ci` to deterministic waits (Layer 1 `quiesce`/`wait_for_ack`, Layer 2 `dispatch_until`/`dispatch_until_done`/`wait_until`/`wait_for_event`).
- **test/plugin: 305 -> 91 real sleep calls = 214 eliminated.** Ratchet metric across all `test/**/*.ci`: 460 -> 246.
- Done directly (me): 41 files (Batch 1 redundant deletions, Batch 2 `dispatch_until_done`, Batch 3 filter-accept, `rib-reconnect`). Done via 6 parallel self-verifying agents: the remaining ~114 files.
- Every converted file verified 3x green at low load; **full plugin suite `--all`: 476/476 PASS (100%), 34 darwin-skips, 0 fail, 0 timeout.**
- Remaining 91 real sleeps are all legitimate DEFER (~35: fib-kernel/vpp async EventBus, external-warn no-stderr-visibility, reject/negative no-positive-edge, RS reflection inbound-gap, control-message path, bgp-redistribute forward-pool 10s block) or KEEP (~56: raw-UDP peer-driver pacing, deliberate timers, standalone-driver stderr backoffs). Worklist records each with its reason. -> follow-on spec.
### Bugs Found/Fixed
- Removing blind sleeps surfaced 4 genuinely under-synchronized tests (the sleep was masking a real race): `bfd-show-profile` (lacked `wait_for_post_startup`), `nexthop-self`, `nexthop-unchanged`, `rr-basic` (single-shot RIB read before the UPDATE landed). All fixed at the source with a deterministic `dispatch_until` poll (or the readiness barrier) -- never by re-adding a sleep. This is the intended `feedback_sleep_hides_races` outcome.
- Ratchet-counting correctness: `verify_wiring_docs.py:213` counts `time.sleep(` across full file text INCLUDING comments; test-relax notes quoting the removed call inflated the count. Rephrased all such comments (78 files) so the ratchet reflects only real sleep calls (raw == real == 91 in test/plugin).
### Documentation Updates
- Primitives already documented by Layer 2 (payload-predicate-waits). A note in `docs/functional-tests.md` that plugin tests use deterministic waits is optional (not blocking).
### Deviations from Plan
- Ratchet target was ~235; achieved 246 (the ~11-sleep gap is the bgp-redistribute-* group, verified un-convertible with today's primitives -- forward-pool `quiesce` blocks ~10s, overrunning the 15s test timeout; deferred to the follow-on infra spec).
- Baseline drop (460 -> 246) is STAGED, not applied: held until the other session commits Layer 2 (shared `test/.ci-sleep-baseline`, R-4).

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

## Goal Validation (BLOCKING)
| Goal | Evidence Type | Concrete Evidence |
|------|---------------|-------------------|
| convertible plugin sleeps migrated | ratchet + functional | test/plugin 305 -> 91 real sleeps (214 removed); each converted file 3x green; recipes verified against code |
| no regressions | full suite | `bin/ze-test bgp plugin --all`: 476/476 PASS, 34 darwin-skip, 0 fail, 0 timeout |
| deterministic (not just faster) | behavior | 4 masked races surfaced + fixed at source (bfd-show-profile, nexthop-self, nexthop-unchanged, rr-basic); converted tests return when state is observed, not on a timer |
| remaining sleeps are intentional | audit | 91 remaining all DEFER (need new infra) or KEEP (peer-driver pacing / deliberate timers); each documented in the worklist -> follow-on spec |

## Review Gate
### Run 1 (self-review during implementation)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | 4 tests were under-synchronized (single-shot RIB read races) exposed by removing the masking sleep | bfd-show-profile, nexthop-self, nexthop-unchanged, rr-basic | **fixed** at source with dispatch_until / wait_for_post_startup (no re-added sleep) |
| 2 | ISSUE | test-relax comments quoting `time.sleep(` inflated the ci-sleep ratchet (counts comments) | 78 test/plugin `.ci` | **fixed** — rephrased so raw==real==91 |
| 3 | NOTE | bgp-redistribute-* (5 files) not converted | test/plugin | verified un-convertible today (forward-pool quiesce blocks 10s > 15s timeout); deferred to follow-on infra spec |

### Closure prerequisites (NOT yet done)
- Other session must land Layer 2 (payload-predicate-waits) first: converted tests call `dispatch_until` etc. from its uncommitted `ze_api.py`.
- Then: lower `test/.ci-sleep-baseline` 460 -> 246, run `/ze-review`, two-commit closure.
