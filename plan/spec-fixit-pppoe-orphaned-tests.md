# Spec: fixit-pppoe-orphaned-tests

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-16 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/functional-test-gate.md`, `ai/rules/no-workarounds-for-missing-behavior.md`
4. `internal/test/runner/record_parse.go` (`parseOption`), `internal/test/cli/register.go`

## Task

**`test/pppoe/` is orphaned dead code.** It cannot parse, and nothing runs it. The
PPPoE Access feature row in `docs/features.md:88` (marked `Partial`) therefore has no
functional test behind it.

Two independent reasons, both verified at the producer on 2026-07-16:

| # | Claim | Verification |
|---|-------|--------------|
| 1 | The `.ci` files use an option type the parser rejects | `parseOption` (`internal/test/runner/record_parse.go:295-448`) switches on `optType`. Its cases are: `env`, `skip-os`, `needs-linux`, `skip-env`, `require-tag`. There is **no `netns` case**, so `option=netns:veth=...` falls to the `default:` branch at `:444-445`, which returns `fmt.Errorf("unknown option type %q", optType)`. |
| 2 | No suite registers pppoe | `internal/test/cli/register.go` calls `registerCIRoot` 20 times at `:17-36` (appliance, firewall, flow-export, install, ipsec, isis, isis-wire, ospf-wire, ospf, ospfv3, l2tp, l2tp-wire, ldp, managed, policy, rsvpte, static, traffic, ui, vrrp). **pppoe is not among them**; `grep -rni pppoe internal/test/cli/` returns nothing. |

All **3** files are affected, and all 3 carry the rejected directive:

| File | `option=netns` line |
|------|---------------------|
| `test/pppoe/pppoe-basic.ci` | `:149` — `option=netns:veth=veth-bng,veth-sub` |
| `test/pppoe/pppoe-vlan.ci` | `:110` — `option=netns:veth=veth-bng,veth-sub:vlan=100` |
| `test/pppoe/pppoe-concurrent-l2tp.ci` | `:145` — `option=netns:veth=veth-bng,veth-sub` |

→ Constraint: `test/pppoe/` are the **only** consumers of `option=netns` in the whole
tree (`grep -rln "option=netns" test/` returns exactly these three). Nothing else
depends on the directive, so repairing it serves only these tests.

### Options to consider (this spec RECORDS them; it does NOT choose)

| Option | What it means | Notes already known |
|--------|---------------|---------------------|
| A. Repair the directive | Implement a `netns` case in `parseOption` plus the veth/netns setup it implies | The largest option: needs root/CAP_NET_ADMIN, is Linux-only (`ai/rules/qemu-testing.md` applies), and the runner has no netns primitive today |
| B. Re-mark the tests | Rewrite the 3 `.ci` onto directives that already exist | All 3 already carry `option=skip-os:value=darwin` (`pppoe-basic.ci:9`, `pppoe-vlan.ci:7`, `pppoe-concurrent-l2tp.ci:7`); the question is what replaces the topology setup |
| C. Register a suite | Add `registerCIRoot("pppoe", ...)` to `internal/test/cli/register.go` | Necessary for A and B, insufficient alone: without a `netns` case the files still fail to parse |
| D. Delete them | Remove `test/pppoe/` | Requires user approval per `ai/rules/never-destroy-work.md`; would leave `docs/features.md:88` PPPoE Access with no functional coverage and the `Partial` marking unexplained |

→ Decision needed from the user before design proceeds: which option. Do not pick one
by default. Note that C is a prerequisite for A and B, not an alternative to them.

→ Constraint (`ai/rules/no-workarounds-for-missing-behavior.md`): if the tests are
repaired, they must actually exercise the PPPoE discovery path. Making them parse and
skip is a false green, which is what the current state already is in effect.

## Required Reading

### Architecture Docs
- [ ] `docs/functional-tests.md` - the `.ci` directive contract and how suites are registered
  → Constraint: a `.ci` that no suite roots is never discovered, so it cannot fail visibly.
- [ ] `docs/guide/pppoe.md` - the feature these tests were meant to cover
  → Decision: `docs/features.md:88` marks PPPoE Access `Partial`; this orphaning is part of why.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc2516.md` - PPPoE discovery (PADI/PADO/PADR/PADS/PADT), if the tests are repaired to assert wire behavior
  → Constraint: verify this summary exists before citing it; create via `/ze-rfc` if missing.

**Key insights:** (summary of all checkpoint lines — minimal context to resume after compaction)
- Two independent failures (unparseable directive AND unregistered suite) mean neither alone explains the silence; fixing one leaves the tests still dead.
- The tests were never running, so there is no regression to fear and no baseline to preserve.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/test/runner/record_parse.go` - `parseOption` (`:295-448`) accepts only `env`, `skip-os`, `needs-linux`, `skip-env`, `require-tag`; `default:` at `:444-445` returns `unknown option type %q`. `parseLine` (`:243`) dispatches `option` at `:273`
- [ ] `internal/test/cli/register.go` - `registerCIRoot` called for 20 suites at `:17-36`; no pppoe entry
- [ ] `test/pppoe/pppoe-basic.ci` - `option=skip-os:value=darwin` (`:9`), `option=env:var=TEST_IFACE:value=veth-sub` (`:148`), `option=netns:veth=veth-bng,veth-sub` (`:149`)
- [ ] `test/pppoe/pppoe-vlan.ci` - `option=netns:veth=veth-bng,veth-sub:vlan=100` (`:110`)
- [ ] `test/pppoe/pppoe-concurrent-l2tp.ci` - `option=netns:veth=veth-bng,veth-sub` (`:145`), plus `ze.l2tp.skip-kernel-probe` (`:143`)

**Behavior to preserve:** (unless user explicitly said to change)
- The 20 registered suites in `register.go:17-36` and their behavior — this spec adds at most one row, changes none.
- `parseOption`'s fail-closed `default:` branch. An unknown option type MUST stay an error; do not relax it to a warning to make these files parse (`ai/rules/fail-closed-guards.md`).
- The existing 5 option types and their semantics.

**Behavior to change:** (only if user explicitly requested)
- Depends entirely on which of options A-D the user picks. Under D, `test/pppoe/` is deleted and nothing in `internal/` changes.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A `.ci` file on disk under `test/<suite>/`, discovered only if a `registerCIRoot` call names its directory.
- The `option=<type>:<k>=<v>` directive line inside that file.

### Transformation Path
1. `registerCIRoot` (`internal/test/cli/register.go:17-36`) roots a suite name to a `test/` subdirectory — pppoe is absent, so `test/pppoe/` is never walked
2. `EncodingTests.Discover` (`record_parse.go:68`) walks a rooted directory for `.ci` files
3. `parseAndAdd` (`:111`) reads a file and calls `parseLine` per line
4. `parseLine` (`:243`) dispatches the `option` keyword at `:273` to `parseOption`
5. `parseOption` (`:295`) switches on option type; `netns` matches no case and hits `default:` at `:444`, returning `unknown option type "netns"` at `:445`
6. The parse error aborts the record; the test never becomes runnable

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Suite registry ↔ filesystem | `registerCIRoot` name → `test/<dir>/` walk | [ ] |
| `.ci` text ↔ runner Record | `parseLine` / `parseOption` keyword dispatch | [ ] |
| Runner ↔ kernel netns/veth | **does not exist today** — this is the missing primitive under option A | [ ] |

### Integration Points
- `registerCIRoot` (`internal/test/cli/register.go`) - the registration seam a pppoe suite would use; registration over hardcoding already holds here.
- `option=skip-os` / `option=needs-linux` - existing gating primitives the repaired tests would reuse.

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)
- [ ] Registration over hardcoding — a pppoe suite registers via `registerCIRoot`; no per-suite switch case is added to the runner (small-core/registration; `ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `option=netns` was never implemented, rather than removed later | `parseOption` has no `netns` case and no dead handler; the 3 `.ci` are its only users | The primitive exists elsewhere and the tests should be pointed at it | `git log -S "netns:veth" -- internal/test/` | unvalidated |
| A-2 | The 3 `.ci` describe PPPoE behavior that the current code still has | `docs/features.md:88` describes a live PPPoE subsystem (`internal/component/l2tp/pppoe/`) | The tests assert a design that changed; repairing them means rewriting them | Read the 3 `.ci` against `internal/component/l2tp/pppoe/` | unvalidated |
| A-3 | Nothing else waits on a `netns` runner primitive | `grep -rln "option=netns" test/` returns only `test/pppoe/` | Option A has more value than this spec credits | Re-grep at design time | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Registering the suite (option C) without repairing the directive turns silent death into a loud parse failure across CI | `make ze-functional-test` newly red on 3 files | Land C only together with A or B; never C alone |
| R-2 | Option D deletes the only written record of intended PPPoE test topology | — | `ai/rules/never-destroy-work.md`: ask the user first; the `.ci` remain in git history either way |
| R-3 | Option A quietly becomes a large test-infrastructure project (netns + veth + VLAN + root) hiding behind a one-word directive | Design keeps growing past a parser case | Scope A explicitly before starting; `ai/rules/qemu-testing.md` may make QEMU the right host |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-functional-test` discovers `test/pppoe/` | → | a `registerCIRoot("pppoe", ...)` entry in `internal/test/cli/register.go` | `test/pppoe/pppoe-basic.ci` (must run, not skip) |
| A `.ci` declares the topology directive | → | the option handler that accepts it in `parseOption` | `TestParseOptionNetns` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `make ze-functional-test` | `test/pppoe/*.ci` are either discovered and RUN, or absent from the tree — never present-and-ignored |
| AC-2 | Each of the 3 `.ci` parses | No `unknown option type` error (or the file no longer exists, under option D) |
| AC-3 | `docs/features.md:88` PPPoE Access row | Its `Partial` marking matches reality: either functional coverage now exists, or the row states that it does not |
| AC-4 | An unknown option type in any `.ci` | Still an error. `parseOption`'s fail-closed default is not weakened |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs the functional test suite and sees PPPoE covered (or honestly declared uncovered) | `make ze-functional-test` → suite registry → `test/pppoe/` walk → parse → run | `test/pppoe/pppoe-basic.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseOptionNetns` | `internal/test/runner/record_parse_test.go` | AC-2: the topology directive parses (option A only) | |
| `TestParseOptionUnknownStillErrors` | `internal/test/runner/record_parse_test.go` | AC-4: the fail-closed default survives | |
| `TestCIRootsRegistered` | `internal/test/cli/register_test.go` | AC-1: every `test/` subdirectory holding `.ci` files has a registered root — the general guard that would have caught this | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `vlan=` in `option=netns` (option A only) | 1-4094 | 4094 | 0 | 4095 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `pppoe-basic` | `test/pppoe/pppoe-basic.ci` | Operator's PPPoE client completes discovery against ze's access concentrator | |
| `pppoe-vlan` | `test/pppoe/pppoe-vlan.ci` | PPPoE over a VLAN sub-interface | |
| `pppoe-concurrent-l2tp` | `test/pppoe/pppoe-concurrent-l2tp.ci` | PPPoE and L2TP run concurrently on one daemon | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| (fill during design) | `test/interop/scenarios/` | `pppoe-client` / `accel-ppp` | Real client completes RFC 2516 discovery against ze | |

### Future (if deferring any tests)
- (fill during design)

## Files to Modify
- `internal/test/cli/register.go` - add a `registerCIRoot("pppoe", ...)` row (options A/B/C)
- `internal/test/runner/record_parse.go` - add a `netns` case to `parseOption` (option A only)
- `test/pppoe/pppoe-basic.ci` - `:149` directive
- `test/pppoe/pppoe-vlan.ci` - `:110` directive
- `test/pppoe/pppoe-concurrent-l2tp.ci` - `:145` directive
- `docs/features.md` - `:88` PPPoE Access row, if its coverage claim changes

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| CLI commands/flags | [ ] | A new `ze-test pppoe` verb comes free from `registerCIRoot` |
| Functional test for new RPC/API | [ ] | `test/pppoe/*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md:88` PPPoE Access row coverage claim |
| 6 | Has a user guide page? | [ ] | `docs/guide/pppoe.md` |
| 10 | Test infrastructure changed? | [ ] | `docs/functional-tests.md` — a new option type or suite must be documented |

## Files to Create
- (fill during design — depends on the chosen option)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify; confirm the 3 `.ci` still match today's PPPoE code (A-2) |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. Full verification | `make ze-verify` |
| 13. /ze-review gate | Review Gate section |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Decide** — put options A-D to the user; record the ruling here before any code
   - Tests: none
   - Files: this spec
   - Verify: the user has chosen; scope is agreed (`ai/rules/no-partial-completion.md` — no unilateral scope reduction)
2. **Phase: Wiring (MANDATORY FIRST)** — write `TestCIRootsRegistered`, the guard that makes an unrooted `.ci` directory impossible in future
   - Tests: `TestCIRootsRegistered`
   - Files: `internal/test/cli/register_test.go`
   - Verify: RED against today's tree (pppoe unrooted)
3. **Phase: Execute the chosen option** — A, B, C+A, C+B, or D
   - Tests: per the TDD table
   - Files: per Files to Modify
   - Verify: the 3 `.ci` run and assert real PPPoE behavior, or are gone
4. **Functional tests** → the 3 `.ci` green for the right reason, not skipped
5. **Full verification** → `make ze-verify`
6. **Complete spec** → learned summary, two commits

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | The tests exercise PPPoE discovery, not just parse-and-skip |
| Rule: no-workarounds | The `.ci` were not weakened to go green (`ai/rules/no-workarounds-for-missing-behavior.md`) |
| Rule: fail-closed | `parseOption`'s `default:` still errors on unknown types |
| Registration over hardcoding | The pppoe suite registers via `registerCIRoot`; no per-suite branch in the runner (`ai/rules/plugin-self-containment.md`) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| No orphaned `.ci` directory | `TestCIRootsRegistered` passes |
| No unparseable directive | `grep -rn "option=netns" test/` matches a `parseOption` case, or returns nothing |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Privilege | Option A needs netns/veth creation (root or CAP_NET_ADMIN) in the test runner; confirm it cannot escape the test host or leak namespaces on failure |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

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

- Two independent defects (unparseable directive, unregistered suite) produced one silence. Either alone would have been noticed: an unrooted suite fails no build, and an unknown option type errors only when something parses the file. Together they cancel out into "no signal at all", which is why this survived from May to July.
- The generalisable guard is `TestCIRootsRegistered`: assert that every `test/` subdirectory containing `.ci` files is rooted. That catches the next orphaned suite without anyone noticing it.

## Core Insight
(fill during design)

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- Until this spec closes, `docs/features.md:88` PPPoE Access `Partial` has no functional test behind it. The subsystem may work; nothing in CI proves it.

## RFC Documentation

Add `// RFC 2516 Section X.Y: "<quoted requirement>"` above enforcing code, if the repaired tests pin discovery behavior.
MUST document: validation rules, error conditions, state transitions.

## Implementation Summary

### What Was Implemented
- (fill during implementation)

### Bugs Found/Fixed
- (fill during implementation)

### Documentation Updates
- (fill during implementation)

### Deviations from Plan
- (fill during implementation)

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| PPPoE Access has functional coverage, or honestly declares it has none | functional test | (fill during implementation) |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | (fill during implementation) | file:line | (fill during implementation) |

### Fixes applied
- (fill during implementation)

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
- [ ] AC-1..AC-4 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-fixit-pppoe-orphaned-tests.md` only
