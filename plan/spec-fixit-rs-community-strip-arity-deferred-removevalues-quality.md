# Spec: fixit-rs-community-strip-arity-deferred-removevalues-quality

| Field | Value |
|-------|-------|
| Status | done |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | - |
| Updated | 2026-08-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Provenance.** Three rows of `plan/deferrals/fixit-rs-community-strip-arity.md`,
dated 2026-07-28, all Status `open`. (That shard was named
`plan/deferrals/spec-fixit-rs-community-strip-arity.md` until 2026-08-03. The doubled
`spec-` prefix hid it from every gate pairing a shard with `plan/spec-<stem>.md`, and
it was renamed to the conventional spelling.) They named `spec-wire-edit-2-edit-apply` as their
destination (written here without its `plan/` path, because `spec-citation-check.py` reads
any such path as a LIVE citation and this sentence is about a file that is gone), on the
reading that the whole `removeValues` contract was about to be
replaced wholesale. That spec CLOSED on 2026-08-02 (`plan/learned/1318-wire-edit-2-edit-apply.md`)
without doing this work, so the three rows were homeless: a deferral whose destination is
gone is a deletion with a polite name (`ai/rules/planning.md`).

They were briefly repointed at the learned summary itself, by a mechanical sweep that was
repairing dead spec citations elsewhere. That is worse than homeless, because a learned
summary records FINISHED work and `deferral_destination_problem` accepts any existing
file: the row then reads as homed and the gate goes quiet. This spec is the real home.

**The three items**, all in `internal/component/bgp/plugins/filter_community/handler.go`
around `removeValues`:

1. **RF-1, the one with a user-visible consequence.** A peer-controlled quadratic on the
   route-server forward path. Read the row for the measured shape before designing a fix.
2. **RF-2.** Three stale `file:line` citations inside the `removeValues` doc comment. The
   row names each one and what it should say.
3. **A vacuous test.** The only test of the refusal message asserts `Contains(out, "3")`,
   which passes on almost any output. Replace it with an assertion that discriminates.

**Order.** RF-1 first: it is the only one a peer can reach. The other two are hygiene on
the same function and are cheap to land beside it.

**Do not assume the wire-edit replacement is coming.** It closed without touching this
code, so these three are owed against `handler.go` as it stands today.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/architecture/<doc>.md` - [why relevant]
  → Decision: [specific architectural decision that constrains this spec]
  → Constraint: [specific rule from the doc that applies here]

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfcNNNN.md` - [why relevant]
  → Constraint: [specific RFC rule that applies here]

**Key insights:** (minimal context to resume after compaction)
- [insight from docs]

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `path/to/file.go` - [what it currently does]

**Behavior to preserve:** (unless the user explicitly said to change it)
- [output format, function signature, or `.ci` expectation callers depend on]

**Behavior to change:** (only what the user asked for)
- [list, or "None - preserve all existing behavior"]

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [Where data enters: wire bytes, API command, config, plugin message]
- [Format at entry]

### Transformation Path
1. [Stage 1: for example "Wire parsing in internal/component/bgp/message/"]
2. [Stage 2: ...]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | [JSON format, command syntax] | No |

### Integration Points
- [Existing function/type this connects to] - [how it integrates]

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

<!-- LIVE: written during RESEARCH/DESIGN, statuses updated during implementation.
     Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis)
     land HERE, not only in conversation. -->

### Assumptions
<!-- Every row needs a validation method. `unvalidated` is not a valid final
     status: closure re-checks each one. A broken assumption also gets a
     Mistake Log row and a Deviations entry. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | [what this design assumes] | [where the assumption comes from] | [impact on design] | [test/grep/user confirmation] | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | [what goes wrong] | [how we notice it] | [what we do about it] |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | [live sessions dropped / routes mis-encoded / config rejected / nothing user-visible] |
| How is it reverted? | [single commit revert / needs config migration / not revertible once peers see it] |
| Who else touches this path? | [other plugins, components, or specs working the same files] |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by .claude/hooks/validate-spec.sh, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| [config/CLI/event that triggers it] | → | [function that actually runs] | [test name proving the chain] |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | [what triggers the behavior] | [observable outcome] |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | [for example "receives SR-Policy UPDATE from peer"] | [wire -> mpnlri -> splitter -> Parse -> RIB] | [test name] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestXxx` | `internal/.../xxx_test.go` | [description] | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| [field] | [min-max] | [value] | [value or N/A] | [value or N/A] |

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-xxx` | `test/.../*.ci` | [what the user expects to happen] | |

### Interop Tests (Scope: protocol)
<!-- REQUIRED when wire-visible behavior changes. See
     ai/rules/interop-and-goal-validation.md, including the vacuity traps: prove
     the test FAILS when the behavior under test is reverted. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-feature-peer` | `test/interop/scenarios/` | [FRR/BIRD/GoBGP/strongSwan] | [protocol behavior validated] | |

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `internal/...` - [feature changes]

## Files to Create
- `internal/...` - [new feature file]
- `test/.../*.ci` - [functional test for end-user behavior]

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | `internal/component/<name>/yang/` or the owning plugin's `yang/`. Read `ai/rules/config.md` (YANG vs env var) and `ai/rules/config.md` (naming) |
| YANG validation constraints | | Every leaf takes maximum native validation: `range`, `length`, `pattern`, `enumeration`, `type` from `ze-types.yang`. See `ai/patterns/config-option.md` |
| YANG custom validators | | Where native constraints are insufficient: `ze:validate` + `ValidateFn` + `CompleteFn` for completion |
| CLI commands/flags | | `cmd/ze/*/main.go` or subcommand files |
| CLI grammar (keyword before value) | | `ai/rules/cli.md` |
| Editor autocomplete | | Automatic for YANG enum/type leaves. Dynamic values need `CompleteFn` |
| Functional test for new RPC/API | | `test/plugin/*.ci` or `test/decode/*.ci` |
| Pipe completeness | | Route output through `ApplyPipes`/`ProcessPipes` per `ai/rules/cli.md` |
| Env var registration | | YANG leaves under `environment/` need a matching `ze.<name>.<leaf>` via `env.MustRegister()` |
| Doctor check for runtime dependencies | | Any new file path, socket, service, kernel module, listen port, procfs/sysctl, netlink, binary, or certificate: owning-package check + `internal/core/diagnostic/codes.go` + unit and functional test (`ai/rules/repo-maintenance.md`) |
| Prometheus counters/metrics | | Observable state: define, register, and list the metric names and labels here |
| BGP family surface (new SAFI / capability / attribute) | | The 12-section checklist in `ai/patterns/bgp-family.md` -- read it and record the answers there, not inline |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | | `docs/features.md` |
| 2 | Config syntax changed? | | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | | `docs/guide/<topic>.md` |
| 7 | Wire format changed? | | `docs/architecture/wire/*.md` |
| 8 | Plugin SDK/protocol changed? | | `ai/rules/plugins.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | | `rfc/short/rfcNNNN.md` and the `docs/features/rfc-status.md` row, with source anchors |
| 10 | Test infrastructure changed? | | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | | `docs/comparison.md` |
| 12 | Internal architecture changed? | | `docs/architecture/core-design.md` or subsystem doc |
| 13 | Route metadata keys added/changed? | | `docs/architecture/meta/README.md`, `docs/architecture/meta/<plugin>.md` |
| 14 | Prometheus counters added/changed? | | `docs/plugin-development/metrics.md` or subsystem telemetry doc |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | | `docs/plugin-overview.md`, `docs/features/plugins.md`, `docs/guide/status.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | | Grep `docs/` for `source: <changed-file>` and update each stale claim |
| 17 | Existing docs show config/CLI/API examples for this area? | | Verify examples against YANG/parser/handler and update stale syntax |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

1. **Phase: Wiring (MANDATORY FIRST)** -- register entry points, write failing wiring tests
   - Tests: [wiring test names from the Wiring Test table]
   - Files: [register.go, handler skeleton, route registration]
   - Verify: the entry point exists and is reachable. The wiring test fails because the feature is a stub
2. **Phase: [name]** -- [what to implement]
   - Tests: [test names from the TDD Plan]
   - Files: [files from Files to Modify]
   - Verify: tests fail → implement → tests pass → wiring test progresses

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | [feature-specific, for example "merge order correct", "error messages name the offending value"] |
| Naming | [feature-specific, for example "JSON keys kebab-case", "YANG leaf matches env var leaf"] |
| Data flow | [feature-specific, for example "resolution in X only, reactor unaware of Y"] |
| Rule: [relevant rule] | [what to check] |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| [concrete thing that must exist] | [grep/ls/test command] |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | [what inputs need validation and how] |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
<!-- LIVE: write immediately when you learn something. At closure these route to
     a subsystem arch doc, a rule, or the learned summary. -->

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     needs a row in the deferral shard named in the metadata table. -->
- [What was deliberately not done and why]

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

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
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented

All three items this spec owns are closed. Two were already resolved by work that
landed elsewhere and were verified against source today; RF-1 was implemented today.

- **RF-1, the peer-reachable quadratic.** `newRemovalSet` and `removalSet`
  (`internal/component/bgp/plugins/filter_community/handler.go`) choose the
  membership representation ONCE per attribute, above the loop over source values,
  and answer from a map above `removalIndexThreshold`. The threshold reads
  `min(source values, removal values)`. The map collapses duplicates as it is built.
- **RF-2, three stale `file:line` citations.** Moot: `removeValues` and its doc
  comment no longer exist, and no line citation remains in the file.
- **RF-3, a vacuous test.** Already fixed. The assertions name KEY=VALUE pairs and
  carry a comment recording why the bare `Contains(out, "3")` matched the slog
  timestamp.

### Bugs Found/Fixed

- **The defect had survived the rewrite that was expected to remove it.** The row
  named `removeValues`, which no longer exists, so the row read as stale. The
  per-value call had moved into `removedByAny` and stayed inside the loop.
- **A comment asserted the safety property that was false.** `containsValue` said
  the removal sets "hold a handful of values (the control communities on one route,
  or one configured strip value)". `StripControlCommunities` derives the buffer from
  the peer's own attribute, so a peer sizes it. The comment is quoted in place rather
  than deleted, because it is what kept the defect open.

### Documentation Updates

None. No `docs/` file carries a source anchor to the handler:
`grep -rn "filter_community" docs/` returns no match. The change adds no config,
CLI or wire surface.

### Deviations from Plan

- The deferral row's remedy asked for deduplication at the producer
  (`StripControlCommunities`). Not done, and not needed: the index collapses
  duplicates for free, which is the property whose absence made the reverted
  candidate worse than the defect. Producer deduplication would now only save
  repeated map insertions, and it would allocate in the common tiny case.
- The row's remedy also asked to hoist the set out of the per-destination loop.
  Done within the handler, not across the reactor boundary: the structure is built
  once per attribute rather than once per UPDATE. The fan-out multiplier on an
  O(n + m) build is not the defect.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | The three rows looked stale, because `removeValues` no longer exists and two of the three items were genuinely already fixed | RF-1 was live. The call had moved, not gone | Read the loop that CALLS the helper rather than searching for the helper's name | Fixed RF-1. Recorded the trap in `plan/learned/1351` |
| approach | An owner direction of 2026-07-28 said not to ship a cost trade because the handler was being replaced wholesale | The replacing spec closed on 2026-08-02 without touching this code, so the direction's premise was gone | This spec's own Task section says so in writing | Treated the work as owed |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| RF-1, remove the peer-controlled quadratic | Done | `newRemovalSet` (`internal/component/bgp/plugins/filter_community/handler.go`) | O(n + m) per destination, was O(n * m) |
| RF-2, stale citations in the doc comment | Done | n/a | The function and its comment no longer exist |
| RF-3, replace the vacuous assertion | Done | `internal/component/bgp/plugins/filter_community/handler_test.go` | Asserts KEY=VALUE pairs |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 (derived): a peer cannot drive the retained-run loop quadratic | Done | `TestRemovalSetIndexesOnlyAboveThreshold` | Includes the measured 16383 by 16383 attack shape |
| AC-2 (derived): indexing changes no answer | Done | `TestRemovalSetAnswersAgreeAcrossRepresentations` | Both representations agreed on every present and absent value |
| AC-3 (derived): a duplicate-heavy buffer stays bounded | Done | `TestRemovalSetDeduplicatesIndexEntries` | 4096 identical values collapse to one entry |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| A test that discriminates on the refusal message | Done | `internal/component/bgp/plugins/filter_community/handler_test.go` | Pre-existing, verified today |
| A guard against the quadratic | Done | `internal/component/bgp/plugins/filter_community/handler_test.go` | Four tests, both claims mutation-verified |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/plugins/filter_community/handler.go` | Done | The set, the threshold, the corrected comment |
| `internal/component/bgp/plugins/filter_community/handler_test.go` | Done | Four new tests |

### Audit Summary
- **Total items:** 10
- **Done:** 10
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (both recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A peer cannot spend the router's CPU quadratically on the route-server forward path | unit test plus mutation | `TestRemovalSetIndexesOnlyAboveThreshold` covers the measured 16383 by 16383 shape. Forcing the scan fails it at `both one above` and `the measured attack shape`; thresholding on the removal count alone fails it at `large removals against a two-value attribute` |
| The change is not a behavior change | unit test | `TestRemovalSetAnswersAgreeAcrossRepresentations`: both representations return the same answer for every present value and three absent ones |
| The route-server strip still works end to end | functional | `make ze-plugin-test`, 597 of 597, zero failures, including `bgp-rs-community-strip-multi` and `bgp-rs-community-strip-multi-fastpath` |
| The guard cannot go decorative or flake | design plus mutation | The assertion is on the representation `newRemovalSet` picked, which is deterministic. Both mutants were killed at named subtests |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| RF-1, `removeValues` scans the removal set once per retained value | done | Implemented today. `plan/deferrals/fixit-rs-community-strip-arity.md` carries the evidence |
| RF-2, three stale `file:line` citations | done | Moot, verified at source today |
| RF-3, the vacuous refusal-message test | done | Already fixed, verified at source today |

The shard `plan/deferrals/fixit-rs-community-strip-arity.md` now holds no live row,
but its source spec `plan/spec-fixit-rs-community-strip-arity.md` is still open, so
the shard is NOT removed by this closure. That spec's closure removes it.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-rs-community-strip-arity-deferred-removevalues-quality-c4c78ddb-c47b-4f1a-a85d-5911d7c65455.md` |
| `review_gate.py check` | clean |
| Reviewer lenses used | peer-reachable cost, allocation behavior, membership semantics, guard vacuity, comment accuracy |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | - | No BLOCKER and no ISSUE survived the pass | - | - |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/plugins/filter_community/handler.go` | yes | `grep -n "func newRemovalSet"` resolves |
| `internal/component/bgp/plugins/filter_community/handler_test.go` | yes | Carries the four new tests |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | No peer-driven quadratic | `make ze-test-pkg PKG=./internal/component/bgp/plugins/filter_community` green; both mutants killed |
| AC-2 | Answers unchanged | `TestRemovalSetAnswersAgreeAcrossRepresentations` passes |
| AC-3 | Duplicates bounded | `TestRemovalSetDeduplicatesIndexEntries` asserts one entry for 4096 identical values |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| A route-server forward carrying control communities | `test/plugin/bgp-rs-community-strip-multi.ci` and `test/plugin/bgp-rs-community-strip-multi-fastpath.ci` | yes, both PASS in a full `make ze-plugin-test` run of 597 tests with zero failures |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1: the removal set is operator-configured, so it is small | broken | `StripControlCommunities` (`internal/component/bgp/wireu/community.go`) builds it from the forwarded route's own COMMUNITY attribute. This is the assumption the old `containsValue` comment stated, and it is the defect |
| A-2: the rewrite that deleted `removeValues` also removed the quadratic | broken | The per-value call moved into `removedByAny` and stayed inside the loop |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| No doc describes the community filter handler internals | `grep -rn "filter_community" docs/` returns no match | yes |
| No RFC status row changes | The change alters cost, not wire behavior. RFC 7947 places no normative requirement on stripping control communities, as `StripControlCommunities` records | yes |

## Core Insight

**The defect was the placement of a decision, not the speed of an operation.**
Choosing how to answer a membership question inside the loop that asks it is
quadratic however fast each answer is. Moving the choice above the loop is the
entire fix, and it is why the guard asserts which representation was chosen rather
than how long the loop took: the representation is the mechanism, and the two
representations agree on every answer by construction.
