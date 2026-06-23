# Spec: OSPFv3 Interop-Coverage Completion and ospfv3 Export Cleanup

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-ospf-af-unify |
| Phase | impl + 6-reviewer review fixes + #5 regression recovery + docs all complete & verified; commit needs cross-session coordination -- see note |
| Updated | 2026-06-24 |

> **STATUS (2026-06-24):** ALL work done and verified. Original 5 ACs + a 6-reviewer OSPF review
> (1 BLOCKER, 8 ISSUEs, lower items) fully fixed with regression tests; E1/E2 and #8 assessed as
> deliberate/not-bugs; the four perf NOTEs left (negligible at scale). A self-inflicted #5
> tentative-link-local regression (broke v6 binding in the bridged-container harness) was caught by
> interop and fixed (prefer-ready / fall-back-to-tentative). Verification: 18 ospf+ospfv3+iface unit
> packages green, GOOS=linux vet clean, ze-lint-changed clean, ze-doc-test green, ALL 6 OSPF interop
> scenarios pass (v4-broadcast + v6 broadcast/p2p/multiarea/stub/nssa-redist). Learned summary
> appended to `plan/learned/973-ospfv3-6-interop-coverage.md`.
>
> **COMMIT needs cross-session coordination (confirmed 2026-06-24):** the working tree holds 455
> uncommitted entries -- 385 OSPFv3 (the parallel `spec-ospf-af-unify` session's ENTIRE untracked
> implementation + this spec's delta) plus ~70 from other parallel work (BGP, IS-IS web, sysrib,
> dhcp, web-snapshot, comparison docs, junk). The OSPFv3 build depends on MODIFIED shared-infra
> files (`all.go`, `validators.go`, `codes.go`, `iface.go`, `sysrib`, `locrib`, redistribute) that
> are interleaved with non-OSPF changes and cannot be split (`git add -p` is forbidden from Bash). A
> clean OSPFv3-only commit that also builds is therefore not producible from one session. User chose
> "prepare one all-OSPFv3 commit script"; given the entanglement this needs the user's cross-session
> judgement on the shared files (or af-unify commits the implementation as a unit first, then this
> delta). Run `python3 tmp/ensure-v6stub-call.py && python3 tmp/ensure-frrospf6-helpers.py`
> immediately before committing (both keep getting reverted by the parallel session).

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `test/interop/interop.py`, `test/interop/run.py` - the interop harness (FRROSPF / FRROSPF6 helpers)
4. `test/interop/scenarios/ospf-v6-*` - the existing v6 scenarios

## Task

Close the remaining OSPFv3 **interop-coverage gaps** and one **validation-hygiene** item left after
`plan/spec-ospf-af-unify.md`. None of these are control-plane feature gaps -- the OSPFv3 features are
implemented, unit-tested, and the existing 9 OSPF interop scenarios pass against FRR. These are the
test-coverage and cleanup items that were deferred:

1. **v6 multi-area interop.** Inter-area summaries (Type 0x2003 Inter-Area-Prefix / 0x2004
   Inter-Area-Router) are implemented and unit-tested, but there is no v6 FRR scenario proving an ABR
   advertises an area-0 prefix into a non-backbone area (and vice versa). The blocker is the
   shared-LAN/unique-prefix problem: Ze needs a unique area-0 prefix to advertise, which the shared
   docker subnet does not provide.
2. **v6 stub-area interop.** v6 stub/NSSA receive-suppression is implemented and unit-tested; there is
   no v6 FRR stub scenario.
3. **Broadcast data-plane route assertion.** `ospf-v6-broadcast-frr` (and `ospf-broadcast-frr`) verify
   adjacency + the DR Network-LSA, but assert no installed route. A route assertion needs a non-shared
   prefix on the segment.
4. **`make ze-validate` is RED.** ~28 unwired-export ISSUEs, all pre-existing in `ospfv3/types`,
   `ospfv3/packet`, `ospfv3/transport` (e.g. `DecodeInterAreaPrefixLSA`, `EnableInterfaceInstance`,
   `FloodScope`, `ParseInstanceID`) -- package-level decoders/helpers wrapped by same-package methods.
   They predate the af-unify work but keep `ze-validate` from passing clean.

Out of scope (already covered): NSSA Type-7 redistribution + v6 redist interop un-pend are in
`spec-ospfv3-5-nssa-redist`; the broadcast Link-LSA is in `spec-ospfv3-4-link-lsa`.

## Required Reading

### Architecture Docs
- [ ] `plan/spec-ospf-af-unify.md` - the unified engine; the synthetic-handle SPF graph and v6 seams.
  → Constraint: any harness change must keep all 9 existing OSPF interop scenarios green.
- [ ] `docs/functional-tests.md` - how interop scenarios are declared and run.
  → Constraint: new scenarios follow the `{ze.conf, frr.conf, check.py}` shape; per-process unique
    container/network names (`_SUFFIX = os.getpid()`).
- [ ] `ai/rules/interop-and-goal-validation.md` - when interop is required and what evidence counts.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc5340.md` - OSPFv3.
  → Constraint: Inter-Area-Prefix-LSA (0x2003) carries the summarized prefix; an ABR originates it into
    each attached area for prefixes reachable in others.

**Key insights:**
- These are test/validation items, not protocol changes; the risk is harness plumbing, not correctness.
- The recurring blocker across (1)-(3) is "Ze needs a unique prefix to advertise on a shared docker LAN."

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `test/interop/interop.py` - `FRROSPF` (v4) / `FRROSPF6` (v6) helpers: `wait_adjacency`,
      `has_route`, `has_external_lsa` (fixed this session to match a real entry, not the section header),
      `has_network_lsa`, `has_dr_bdr`. `_create_network(dual_stack=...)` and `_needs_ipv6_wire()` build
      the docker network.
  → Constraint: route assertions go through `show ip[v6] [ospf6] route` helpers; add an inter-area /
    summary helper if needed.
- [ ] `test/interop/scenarios/ospf-v6-frr/`, `ospf-v6-broadcast-frr/` - the existing v6 scenarios; both
      assert adjacency (+ broadcast: DR Network-LSA), neither asserts an installed route.
  → Constraint: a route assertion needs a prefix that exists on exactly one side.
- [ ] `internal/plugins/ospfv3/{types,packet,transport}/*.go` - the unwired exports `ze-validate` flags;
      determine per symbol: prune (dead), make unexported (same-package only), or keep + justify (public
      codec API used by tests / future external decoders).

**Behavior to preserve:**
- All 9 existing OSPF interop scenarios stay green.
- No production OSPFv3 behavior change (this spec is tests + export hygiene only).

**Behavior to change:**
- Add v6 multi-area + stub interop scenarios; add data-plane route assertions where a unique prefix is
  available; bring `ze-validate` to green for the ospfv3 packages.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Interop: `python3 test/interop/run.py <scenario>` builds the docker network, starts Ze + FRR, runs
  `check.py`. Validation flows FRR-vtysh -> assertion.

### Transformation Path
1. Provision a unique advertisable prefix for Ze (loopback/dummy interface with a global v6 address, or a
   second veth segment) so inter-area/broadcast routes become assertable.
2. `check.py` waits for adjacency, then asserts the summarized / segment prefix in FRR's route table.
3. For the export cleanup: `make ze-validate` -> per-symbol prune/unexport/justify -> validate green.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Harness ↔ Ze/FRR | docker network + vtysh | [ ] |
| Ze area-0 source ↔ ABR summary | unique prefix on a Ze-only interface | [ ] |

### Integration Points
- `test/interop/interop.py` `FRROSPF6` helpers - extend with an inter-area/summary route assertion.
- The existing ABR inter-area summary origination (`spec-ospf-af-unify`) - exercised, not modified.
- `make ze-validate` ospfv3 export check - the cleanup target.

### Architectural Verification
- [ ] No production code change for the interop scenarios (config + harness only)
- [ ] Export cleanup does not remove a symbol with a real (non-test) caller
- [ ] No bypassed layers; assertions exercise the real route table

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Ze can be given a unique advertisable v6 prefix in a scenario (loopback/dummy iface with a global address, or a second segment). | docker `--cap-add NET_ADMIN`; OSPF already opens eth0. | Use a 3-container topology so the prefix lives on a Ze-only link. | a scenario where FRR installs the prefix | unvalidated |
| A-2 | The `ze-validate` unwired exports can be pruned/unexported without breaking the codec public surface used by tests. | They are flagged as having no cross-package non-test caller. | Keep + add a justification annotation instead of pruning. | `make ze-validate` green + `make ze-ospf-test` 13/13 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A multi-area/broadcast route still cannot be asserted in a 2-node shared-LAN topology. | No unique prefix to advertise. | Use a 3-container topology (Ze-only link carries the area-0/segment prefix), or assert via FRR LSDB-content (`show ipv6 ospf6 database inter-prefix`) instead of the route table. |
| R-2 | Pruning an ospfv3 export breaks a test-only or future-decoder caller. | compile/test failure. | Prefer unexport (lowercase) over delete; keep + justify where the symbol is a deliberate public codec entry. |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `run.py ospf-v6-multiarea-frr` | → | ABR inter-area summary origination (existing code) | `ospf-v6-multiarea-frr/check.py` asserts the summarized prefix in FRR |
| `make ze-validate` | → | ospfv3 package exports | validate exits 0 for ospfv3 packages |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ospf-v6-multiarea-frr` against FRR | FRR learns Ze's area-0 prefix as an OSPFv3 inter-area route (and/or Ze learns FRR's) |
| AC-2 | `ospf-v6-stub-frr` against FRR | v6 stub area forms; no Type-5 AS-External leaks into the stub; default reachable |
| AC-3 | broadcast scenario with a unique segment prefix | FRR installs the segment/route originated by the DR (data-plane assertion) |
| AC-4 | `make ze-validate` | exits 0 for `ospfv3/types`, `ospfv3/packet`, `ospfv3/transport` (no unwired-export ISSUEs) |
| AC-5 | all existing scenarios | the 9 current OSPF interop scenarios remain green; `ze-ospf-test` 13/13 |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | runs Ze as a v6 ABR between area 0 and area 1 next to FRR | area-0 prefix -> Inter-Area-Prefix-LSA -> FRR route table | `ospf-v6-multiarea-frr` |
| 2 | runs Ze in a v6 stub area next to FRR | stub suppression -> no Type-5 in stub; default present | `ospf-v6-stub-frr` |
| 3 | maintains the OSPFv3 codec library | exported codec API has callers or is justified | `make ze-validate` green |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (existing) `TestOSPFv6OriginateSummaries*` | `internal/plugins/ospf/*_test.go` | inter-area summary origination (already covered) | done |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| (config parse) | `test/parse/*.ci` | v6 multi-area / stub config resolves | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-v6-multiarea-frr` | `test/interop/scenarios/` | FRR ospf6d | inter-area summary install across areas | |
| `ospf-v6-stub-frr` | `test/interop/scenarios/` | FRR ospf6d | stub suppression + default | |
| broadcast route assertion | extend `ospf-v6-broadcast-frr` or new | FRR ospf6d | DR-originated segment route installed | |

### Future (if deferring any tests)
- If a 2-node topology cannot assert a route, fall back to FRR LSDB-content assertions and record the
  limitation (mirrors `spec-ospfv3-4-link-lsa` R-1).

## Files to Modify
- `test/interop/interop.py` - add an inter-area / summary route helper if needed.
- `test/interop/scenarios/ospf-v6-broadcast-frr/check.py` - add a route assertion if a unique prefix is provisioned.
- `internal/plugins/ospfv3/{types,packet,transport}/*.go` - unexport/prune/justify the flagged symbols.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | - |
| CLI commands | No | - |
| Functional test | Yes | `test/interop/scenarios/`, `test/parse/` |
| Doctor check | No | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 10 | Test infrastructure changed? | Yes (new scenarios + helper) | `docs/functional-tests.md` |
| 12 | Internal architecture changed? | No (tests + export hygiene only) | - |
| 16 | Changed source referenced by doc anchors? | Check | grep `docs/` for the changed files |

## Files to Create
- `test/interop/scenarios/ospf-v6-multiarea-frr/{ze.conf,frr.conf,check.py}`
- `test/interop/scenarios/ospf-v6-stub-frr/{ze.conf,frr.conf,check.py}`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create |
| 3. Wiring phase | Wiring Test table |
| 4. Implement | Implementation Phases |
| 5. /ze-review gate | Review Gate |
| 6-14 | per template |

### Implementation Phases
1. **Phase: unique-prefix topology (MANDATORY FIRST)** — decide and implement how Ze advertises a unique
   prefix (loopback/dummy iface or 3-container link); prove it with one route assertion.
   - Verify: FRR installs a Ze-originated prefix in at least one scenario.
2. **Phase: v6 multi-area scenario** — add `ospf-v6-multiarea-frr`; assert the inter-area summary.
   - Verify: AC-1.
3. **Phase: v6 stub scenario** — add `ospf-v6-stub-frr`; assert no Type-5 leak + default.
   - Verify: AC-2.
4. **Phase: broadcast route assertion** — extend the broadcast scenario.
   - Verify: AC-3.
5. **Phase: ze-validate cleanup** — per-symbol unexport/prune/justify in the ospfv3 packages.
   - Verify: AC-4; `ze-ospf-test` 13/13 unchanged.
6. **Full verification** → `make ze-verify` + the full OSPF interop suite.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N demonstrated |
| No production drift | Only tests/config + export visibility changed; no OSPFv3 behavior change |
| Regression | All 9 existing interop scenarios + ze-ospf-test still green |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| v6 multiarea scenario | `python3 test/interop/run.py ospf-v6-multiarea-frr` |
| v6 stub scenario | `python3 test/interop/run.py ospf-v6-stub-frr` |
| validate green | `make ze-validate` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| N/A (test + export hygiene) | no untrusted input introduced |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Route still not assertable in 2-node | Switch to 3-container topology or LSDB-content assertion (R-1) |
| Export prune breaks a caller | Unexport instead of delete (R-2) |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## Core Insight

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Provision a unique advertisable prefix for Ze | rely on the shared LAN subnet (not assertable) | A route can only be asserted if it exists on exactly one side; the shared subnet is connected on both. |

## Known Limitations
- If a 2-node topology cannot assert a given route, validation falls back to FRR LSDB-content checks
  (documented per scenario), as in `spec-ospfv3-4-link-lsa`.
- The ospfv3 export cleanup may keep some symbols exported with an explicit justification rather than
  pruning, where they are a deliberate public codec entry point.

## RFC Documentation
- `// RFC 5340 ...` on any new enforcing code (none expected; this spec is tests + hygiene).

## Implementation Summary

### What Was Implemented
- [pending]

### Bugs Found/Fixed
- [pending]

### Documentation Updates
- [pending]

### Deviations from Plan
- [pending]

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

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| v6 multi-area interop | interop test | `ospf-v6-multiarea-frr` installs the inter-area route |
| v6 stub interop | interop test | `ospf-v6-stub-frr` (no Type-5 leak, default present) |
| validate green | command | `make ze-validate` exits 0 for ospfv3 packages |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | | | | |

### Fixes applied
-

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

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

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] All 9 existing OSPF interop scenarios remain green
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added (or N/A)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional/interop tests for end-to-end behavior
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only
