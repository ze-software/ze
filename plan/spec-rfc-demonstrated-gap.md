# Spec: rfc-demonstrated-gap

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 3/3 |
| Handoff | - |
| Updated | 2026-09-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A requirement Ze knowingly does not meet carries `{gap: reason}` in
`rfc/short/<stem>.md`. The annotation is prose: nothing runs it, so nothing
notices the day the behavior lands, and the public ledger keeps calling a
fixed requirement a gap. 682 gated rows carry `{gap}` today, and none carries a
test, because `annotationBarsATest` refuses any tag on a gap row.

ExaBGP's ledger (`qa/rfc/README.md`, "Demonstrating a gap instead of describing
it") lets a test assert the CORRECT behavior and be marked strict
expected-to-fail. The ledger reports the requirement as a demonstrated gap, and
the day the behavior arrives the test passes unexpectedly and the suite goes red.

This spec gives Ze the same thing: a test that demonstrates a gap, a tag that
names it, a gate that ties the two together, and a published count of
demonstrated gaps beside described ones.

Owner goal, 2026-09-26: write, implement and close this spec without pausing.
The design decisions below are the main thread's, recorded in Key Design
Decisions with the rejected alternatives, in place of the usual gate questions.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` - tag format, discrimination record, coverage
  → Constraint: a tag is `RFC requirement: <ID> <polarity>` plus claim prose; `./le rfc check` is static and never runs a test
- [ ] `docs/contributing/rfc-implementation-guide.md` - how authors tag tests

### Source
- [ ] `internal/le/rfc/tags.go` - `parseTagRest`, `extendClaim`
  → Constraint: the second token must be in `polarities` (`positive`, `negative`); every other word is refused, so a new marker word cannot collide with an existing tag
- [ ] `internal/le/rfc/check_core.go` - `annotationBarsATest`, `evaluate`
  → Constraint: `evaluate` refuses any tag on a `{gap}` row as a stale annotation; the new arm must accept only a gap tag there and keep refusing positive/negative
- [ ] `internal/le/rfc/discriminate.go` - `discriminationOwedTags`
  → Constraint: filters on `gated[RID]` only, so a gap tag would be billed a discrimination record; a gap test is red by construction and has no observable green half, so it is exempt until the gap closes
- [ ] `internal/le/rfc/coverage.go` - `CoverageRows`; `internal/le/site/rfcevidence.go` - `rfcGapRows`; `internal/le/site/rfcledger.go` - `rfcLedgerCoverageOf`
  → Constraint: `{gap}` stays the summary annotation; `checkStatusAgreement`, `checkGapCountAgreement`, `checkAuditNote` and the site buckets key on it
- [ ] `internal/le/rfc/carriers.go`, `internal/le/rfc/goscope.go` - `carriersFor`, `UnitAt`
  → Constraint: `UnitAt` answers the enclosing Go function, so the gate can read the unit's body for the helper call
- [ ] `internal/test/runner/must_fail_test.go` - `runMustFailFixture`, the one existing must-fail mechanism (runner self-tests only)

### Reference
- [ ] `~/Code/github.com/exa-networks/exabgp/main/qa/rfc/README.md` - "Demonstrating a gap instead of describing it"
  → Decision: strict, like pytest `xfail(strict=True)`: an unexpected pass is a red, never a warning

## Current Behavior (MANDATORY)

Source files read:
- [ ] `internal/le/rfc/tags.go`
- [ ] `internal/le/rfc/check_core.go`
- [ ] `internal/le/rfc/summary.go`

| Fact | Evidence |
|------|----------|
| 682 gated `{gap}` rows (610 MUST, 59 MUST NOT, 12 SHALL, 1 SHALL NOT) in 82 summaries | research count 2026-09-26 |
| 0 of them carry a tag | `annotationBarsATest` refuses it in `evaluate` |
| No recording `testing.TB` exists in the repo | research grep 2026-09-26 |
| The gate is static; only the discrimination observer runs `go test` | `discriminate_observe.go` |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
Two: `./le test unit` (runs the gap test, which fails the moment the gap closes)
and `./le rfc check` (static: reads the tag, the unit body and the summary).

### Transformation Path
1. Test author writes a Go test whose body calls the gap helper with the requirement id and asserts the RFC-correct behavior
2. The helper runs the body against a recorder; recorded failures mean the gap stands (test passes, failures logged); no failure means the gap closed (test fails naming the id and the fix)
3. `scanGoTags` reads `RFC requirement: <ID> gap`; `parseTagRest` returns a gap tag
4. `evaluate` accepts a gap tag only on a `{gap}` row whose unit calls the helper with that id
5. Coverage and the public ledger count the row as a demonstrated gap

### Boundaries Crossed
Test code to the recorder (goroutine for FailNow); summary annotation to tag scan; `internal/le/rfc` to `internal/le/site`.

### Integration Points
`parseTagRest`, `Tag`, `evaluate`, `discriminationOwedTags`, `CoverageRows`, `rfcGapRows`, `rfcLedgerCoverageOf`, the check report text.

## Risks & Assumptions

### Assumptions

| # | Assumption | Basis | If wrong | Validation | State |
|---|------------|-------|----------|------------|-------|
| A-1 | A recorder embedding `testing.TB` and overriding the failure methods captures every assertion style in the repo (Errorf, Fatalf, testify require/assert) | `testing.TB` has an unexported method, so embedding is the only way to satisfy it; testify needs Errorf plus FailNow | some assertion escapes and fails the parent | unit tests per style | confirmed 2026-09-26: `TestDemonstrateGapCountsEveryFailureStyle` covers Error, Errorf, Fail, Fatal, Fatalf, FailNow, testify assert.Equal/True and require.Equal/NoError, parent stays green; ending styles stop the body via runtime.Goexit |
| A-2 | `UnitAt` gives the gate the enclosing function body, so a static read can confirm the helper call carries the tag's id | `goscope.go` | gate cannot tie tag to helper | read `UnitAt` and the unit-body readers in phase 2 | confirmed 2026-09-26: `UnitAt` answers the enclosing top-level func span (doc comment to closing brace) as `ScopeFunc`, and `ScopeFile` when the line is not inside exactly one function; `gapDemonstration` (`internal/le/rfc/gaps.go`) refuses the `ScopeFile` case and reads the call in the span with `go/ast`; `TestCheckRefusesGapTagWithoutHelper` goes red when the id match is mutated |
| A-3 | A helper package under `internal/test/` is allowed by the tier rules for test-only code | `internal/test/runner` exists | package placement refused by `./le tier check` | read `ai/rules/architecture.md` tiers in phase 1 | confirmed 2026-09-26: `internal/test/` is in `NonFeaturePrefixes` and `DisableableNonProdPrefixes` (`internal/le/arch/tier/gates.go`); `./le arch tier check` clean with `internal/test/rfcgap` present |

### Risks

| # | Risk | Early signal | Mitigation |
|---|------|--------------|------------|
| R-1 | A gap test fails for the wrong reason (a panic, a missing fixture) and still reads as "gap stands" | a demonstrated gap whose logged failure is a nil dereference | a panic in the body fails the parent (never counts as the gap); only assertion failures count |
| R-2 | A closed gap flips a unit test red in an unrelated session | `./le test unit` red naming a gap id | the failure message names the summary row and the two edits owed (remove `{gap}`, retag positive/negative), so the fixer needs no context |
| R-3 | A gap tag outlives its `{gap}` (someone removes the annotation and forgets the tag) | gap tag on a non-gap row | the gate refuses a gap tag on a row that is not `{gap}` |
| R-4 | A gap tag on a unit that never calls the helper passes as demonstrated | tag present, helper absent | the gate reads the unit body and refuses a gap tag whose unit does not call the helper with the same id |
| R-5 | FailNow inside the body kills the parent goroutine | test hangs or parent fails | the helper runs the body in its own goroutine and ends it with `runtime.Goexit` on FailNow |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing in the binary. A wrong gate either refuses valid gap tags or publishes a gap as demonstrated that is not |
| How is it reverted? | single commit revert |
| Who else touches this path? | `internal/le/rfc` check sessions; `plan/pre-release/spec-rfc-requirement-quote-hand-backfill.md` edits summaries only |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le test unit` over a test calling the helper whose body fails | → | gap helper | `TestDemonstrateGapPassesWhileBodyFails` in `internal/test/rfcgap/rfcgap_test.go` |
| `./le test unit` over a test calling the helper whose body passes | → | gap helper | `TestDemonstrateGapFailsWhenBodyPasses` in `internal/test/rfcgap/rfcgap_test.go` |
| `./le rfc check` over a fixture with a gap tag on a `{gap}` row whose unit calls the helper | → | `evaluate` gap arm | `TestCheckAcceptsDemonstratedGap` in `internal/le/rfc/check_gap_test.go` |
| real pilot: a `{gap}` row in the corpus demonstrated by a real test | → | helper plus gate | `TestRFC7606Section51MPAttributeEncodedFirst` in `internal/component/bgp/message/rfc7606_mp_first_test.go` (RFC7606-5.1-1, through `BuildUnicast`), and the real-tree gap count: 1 demonstrated, 683 described |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Helper body records an assertion failure (Errorf, Error, Fatalf, Fatal, FailNow, testify assert or require) | the test passes and logs the recorded failures with the requirement id |
| AC-2 | Helper body records no failure | the test fails with a message naming the id, the summary file, and the two edits owed |
| AC-3 | Helper body panics | the test fails as a panic, never counted as the gap standing |
| AC-4 | Tag `RFC requirement: <ID> gap -- claim` | parsed as a gap tag; any other second word stays refused as before |
| AC-5 | Gap tag on a `{gap}` row whose unit calls the helper with that id | accepted; the row counts as a demonstrated gap |
| AC-6 | Gap tag on a row not annotated `{gap}` | refused, naming the row and the retag owed |
| AC-7 | Gap tag whose unit does not call the helper with the tag's id | refused |
| AC-8 | positive or negative tag on a `{gap}` row | still refused as today |
| AC-9 | Gap tag in a `.ci` or `.et` file | refused: a gap is demonstrated in a Go test |
| AC-10 | Gap tag on a gated requirement | owes no discrimination record |
| AC-11 | `./le rfc check` output and the public ledger | show demonstrated gaps separately from described gaps, per stem and in total |
| AC-12 | Pilot | at least one real `{gap}` row carries a demonstrated-gap test that passes today because the behavior is still absent |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDemonstrateGapPassesWhileBodyFails` | `internal/test/rfcgap/rfcgap_test.go` | AC-1 | |
| `TestDemonstrateGapFailsWhenBodyPasses` | same | AC-2 (driven through a child process or a fake parent TB) | |
| `TestDemonstrateGapCountsEveryFailureStyle` | same | AC-1, A-1 | |
| `TestDemonstrateGapPanicIsNotAGap` | same | AC-3 | |
| `TestParseTagGapMarker` | `internal/le/rfc/tags_test.go` | AC-4 | |
| `TestCheckAcceptsDemonstratedGap` | `internal/le/rfc/check_gap_test.go` | AC-5 | |
| `TestCheckRefusesGapTagOnNonGapRow` | same | AC-6 | |
| `TestCheckRefusesGapTagWithoutHelper` | same | AC-7 | |
| `TestCheckStillRefusesPolarityTagOnGapRow` | same | AC-8 | |
| `TestCheckRefusesGapTagInCI` | same | AC-9 | |
| `TestGapTagOwesNoDiscrimination` | same | AC-10 | |
| `TestCheckReportsDemonstratedGaps` | same | AC-11 | |

### Boundary Tests (numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | no numeric input | - | - | - |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestRFC7606Section51MPAttributeEncodedFirst` | `internal/component/bgp/message/rfc7606_mp_first_test.go` | a developer runs `./le test unit` and the gap test passes while Ze still lacks the behavior | |

## Files to Modify

- `internal/le/rfc/tags.go` - gap marker in `parseTagRest`, a field on `Tag`
- `internal/le/rfc/rfc.go` - the marker constant
- `internal/le/rfc/check_core.go` - gap arm in `evaluate`
- `internal/le/rfc/discriminate.go` - exempt gap tags in `discriminationOwedTags`
- `internal/le/rfc/coverage.go`, `internal/le/rfc/check.go` - demonstrated-gap counts
- `internal/le/site/rfcevidence.go`, `internal/le/site/rfcledger.go` - publish demonstrated vs described
- `docs/contributing/rfc-conformance-gates.md`, `docs/contributing/rfc-implementation-guide.md` - the gap tag and helper
- `ai/skills/ze-rfc.md` - one line on demonstrating a gap
- one `rfc/short/<stem>.md` pilot row (annotation unchanged) and its test file

## Files to Create

- `internal/test/rfcgap/rfcgap.go` - the helper
- `internal/test/rfcgap/rfcgap_test.go`
- `internal/le/rfc/check_gap_test.go`

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | tooling |
| YANG validation constraints | N-A | tooling |
| YANG custom validators | N-A | tooling |
| CLI commands/flags | N-A | no new command; `./le rfc check` output gains figures |
| CLI grammar (keyword before value) | N-A | no new command |
| Editor autocomplete | N-A | no YANG |
| Functional test for new RPC/API | N-A | no RPC; pilot test is the end-to-end proof |
| Pipe completeness | N-A | le output only |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | none |
| Prometheus counters/metrics | N-A | tooling |
| BGP family surface (new SAFI / capability / attribute) | N-A | not a family |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | tooling |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | No | none |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | the pilot row's public ledger entry shows a demonstrated gap |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/rfc-conformance-gates.md`, `docs/contributing/rfc-implementation-guide.md` |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `website/AI.md` is declared by `rfcevidence.go` and `rfcledger.go`: update where it describes what the RFC ledger page shows per requirement, to include a demonstrated gap. `docs/architecture/core-design.md` (declared by `internal/le/rfc` headers) describes the obligation registry, unchanged in shape |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derive with `./le spec citation anchors spec plan/spec-rfc-demonstrated-gap.md` in phase 2 |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | tag examples in `docs/contributing/rfc-implementation-guide.md` |

## Implementation Steps

1. **Phase: Helper** -- `internal/test/rfcgap` with the recorder and `Demonstrate`; validate A-1 and A-3
   - Tests: the four `rfcgap_test.go` rows; AC-2 needs the parent's failure observed without failing the real run (child `go test` process or an injected parent TB)
2. **Phase: Gate** -- gap marker in the tag grammar, the `evaluate` arm, the helper-call check over the unit body (A-2), the discrimination exemption, demonstrated-gap counts in the check report and the site
   - Tests: the eight `check_gap_test.go` rows and `TestParseTagGapMarker`
   - Can run in parallel with phase 1: it only needs the helper's package path and function name, fixed here as `rfcgap.Demonstrate(t, "<ID>", func(tb testing.TB) {...})`
3. **Phase: Pilot and docs** -- demonstrate one real `{gap}` (candidate: RFC7606-5.1-1, Ze emits MP_UNREACH_NLRI first and MP_REACH_NLRI last where §5.1 requires the MP attribute to be encoded first; the implementer confirms against `rfc/full/rfc7606.txt` and picks another row if the gap is not a behavior a test can assert); docs and skill
   - Verify: the pilot test passes today, and forcing the behavior (temporarily) makes it fail with the AC-2 message

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | only assertion failures count as the gap standing; a panic never does |
| Correctness | the helper-call check matches the tag's id, not any id |
| Data flow | `{gap}` stays the single summary fact; the tag adds evidence, never a second annotation |
| Rule: principles (no silent zero) | a gap tag the gate cannot tie to a unit is refused, never counted as demonstrated |
| Rule: no-layering | no second gap marker in the summary |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Helper exists and inverts | `rfcgap_test.go` passes |
| Gate accepts and refuses | `check_gap_test.go` passes |
| Pilot demonstrated | `./le rfc check` prints demonstrated gaps 1 or more |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | tag parsing of the new word cannot swallow a claim or accept a malformed id |

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

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Marker is the tag's second word `gap` (`RFC requirement: <ID> gap`) | a third token after the polarity; a marker in the summary row | the second word is already a closed set every parser refuses outside of, so no existing tag changes meaning; a demonstrated gap proves no polarity; a summary marker would be a second gap fact beside `{gap}` (no-layering) |
| Strictness enforced by a Go helper inverting the result at run time | a static-only gate; a `t.Skip` convention | the gate never runs tests, and a skip never goes red when the gap closes |
| Only assertion failures count; a panic fails the parent | count any failure | a crash is a different defect and must not read as the gap standing (R-1) |
| Gap tags only in Go tests; `.ci`/`.et` refused | a `.ci` runner directive modelled on `# must-fail:` | one mechanism; any daemon behavior a `.ci` shows can be driven from a Go test that calls the helper, and a `.ci` failure reason is harder to pin to the assertion than a Go one. Deliberate boundary, not deferred work |
| Gap tag exempt from discrimination | owe a record | the test is red by construction; its green half cannot be observed until the fix, when the retag to positive/negative owes a normal record |
| Pilot one real row, not a mass conversion | convert many of the 682 | the mechanism is the deliverable; converting gaps is authoring work each owning spec does |

## Known Limitations

- `.ci` and `.et` tests cannot demonstrate a gap (Key Design Decisions).
- The 681 other `{gap}` rows stay described until their owners write gap tests; the published count makes the split visible.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated, not library-only
- [ ] Integration and Documentation checklists answered with evidence
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (N-A: none)
- [ ] Functional tests for end-to-end behavior (the pilot)
- [ ] Interop tests: N-A, tooling with no protocol peer

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** remove the spec, in the same `./le commit create` script

---

## Implementation Summary

### What Was Implemented
- `internal/test/rfcgap`: `Demonstrate` runs the body against a recording `testing.TB` in its own goroutine and inverts the verdict (failure recorded: pass and log; none: fail naming the summary and the retag; panic, skip or malformed id: fail).
- `internal/le/rfc`: `gap` accepted in the polarity slot (`parseTagRest`); `scanGoTags` resolves `Tag.Demonstration` through `gapDemonstration` (go/ast over the `UnitAt` span); `splitGapTags` keeps gap tags out of every proof reader (`Collect`, `tagCoversIn`, `baselineTaggedAt`); `evaluateGapTags` refuses through `gapTagRefusal`; `demonstratedGaps` and `gapCounts` feed `CheckReport.GapsDemonstrated/GapsDescribed/GapsByStem` and the `gaps:` line; gap-tag packages join the compile check.
- `internal/le/site`: `DemonstratedBy` on each ledger requirement, `DemonstratedGaps` per stem, "demonstrated by <unit>" in the gaps table, a counter row and an index column.
- Pilot: `TestRFC7606Section51MPAttributeEncodedFirst` demonstrates RFC7606-5.1-1 through `BuildUnicast`.

### Bugs Found/Fixed
- The MP_REACH ordering drift (journal row 2026-09-21 in `plan/journal/published-value-drifts-from-the-behavior-it-describes.md`): every description now names `attribute.OrderAttributes`, including four builder comments and one test comment found at closure. The pilot test covers the behavior.

### Documentation Updates
- `docs/contributing/rfc-conformance-gates.md` "Demonstrated gaps", with anchors to `Demonstrate`, `gapTagRefusal`, `gapDemonstration`, `splitGapTags`, `check`, `gapCounts`, `rfcGapRows`.
- `docs/contributing/rfc-implementation-guide.md` (9.7 bullet and worked example), `website/AI.md`, `docs/contributing/gh-pages.md`, `ai/skills/ze-rfc.md`.
- `docs/architecture/wire/mp-nlri-ordering.md`, `docs/architecture/wire/update-packing.md`, `docs/architecture/rfc-may-decisions.md` (anchor `OrderAttributes`), `rfc/short/rfc7606.md` gap reason and Support remaining row.
- `./le doc index check`: all references valid, `ai/DOCS-TO-CODE.md` up to date.

### Deviations from Plan
- The gap arm lives in `internal/le/rfc/gaps.go` (`evaluateGapTags`) beside `evaluate`, not inside `check_core.go`; `evaluate` keeps refusing proof tags on `{gap}` rows unchanged.
- AC-10 holds by construction: gap tags never enter a cover map, so `discriminationOwedTags` is unchanged.
- Gap figures print only on a passing `./le rfc check`, like the gated and tag counts; the JSON and the fixture test carry them.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Phase 3 first reordered the `BuildUnicast` sort to close the pilot gap | `WriteAttributesOrdered` re-orders through `attribute.OrderAttributes`, the real producer | the pilot stayed green | closed-path proof run against `OrderAttributes` |
| approach | Phase 2 comments named `gapTagIssue`, and the helper header named `check_core.go` | the function is `gapTagRefusal` in `gaps.go` | closure review | fixed at closure |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| a test that demonstrates a gap | Done | `internal/test/rfcgap/rfcgap.go` `Demonstrate` | |
| a tag that names it | Done | `internal/le/rfc/tags.go` `parseTagRest` | |
| a gate that ties the two | Done | `internal/le/rfc/gaps.go` `gapTagRefusal`, `evaluateGapTags` | |
| a published count | Done | `internal/le/rfc/check.go` `check`, `internal/le/site/rfcledger.go` `rfcLedgerCoverageOf` | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestDemonstrateGapPassesWhileBodyFails`, `TestDemonstrateGapCountsEveryFailureStyle` | |
| AC-2 | Done | `TestDemonstrateGapFailsWhenBodyPasses` | |
| AC-3 | Done | `TestDemonstrateGapPanicIsNotAGap` | a skip also fails: `TestDemonstrateGapSkipIsNotAGap` |
| AC-4 | Done | `TestParseTagGapMarker` | |
| AC-5 | Done | `TestCheckAcceptsDemonstratedGap` | |
| AC-6 | Done | `TestCheckRefusesGapTagOnNonGapRow` | |
| AC-7 | Done | `TestCheckRefusesGapTagWithoutHelper` | |
| AC-8 | Done | `TestCheckStillRefusesPolarityTagOnGapRow` | |
| AC-9 | Done | `TestCheckRefusesGapTagInCI` | |
| AC-10 | Done | `TestGapTagOwesNoDiscrimination` | |
| AC-11 | Done | `TestCheckReportsDemonstratedGaps`, `TestADemonstratedGapNamesItsTest` | the ledger counts per stem, as it does declared gaps; the check report carries the total |
| AC-12 | Done | `TestRFC7606Section51MPAttributeEncodedFirst` | passes today; the forced-closed run failed with the AC-2 message |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| the five `rfcgap_test.go` tests | Done | `internal/test/rfcgap/rfcgap_test.go` | |
| the seven `check_gap_test.go` tests | Done | `internal/le/rfc/check_gap_test.go` | |
| `TestParseTagGapMarker` | Done | `internal/le/rfc/tags_test.go` | |
| `TestRFC7606Section51MPAttributeEncodedFirst` | Done | `internal/component/bgp/message/rfc7606_mp_first_test.go` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/le/rfc/check_core.go` | Changed | the gap arm is in `gaps.go` |
| `internal/le/rfc/coverage.go` | Changed | counts in `gaps.go` and `check.go`; coverage readers see proof tags only |
| every other planned file | Done | |

### Audit Summary
- **Total items:** 12 AC, 4 requirements
- **Done:** all
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 file placements (Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| a gap test goes red the day the behavior lands | unit test over the real builder, closed-path run | `TestRFC7606Section51MPAttributeEncodedFirst` passes today and logs `gap RFC7606-5.1-1 stands: first path attribute is type 1 ...`; with `OrderAttributes` forced to put MP_REACH first it failed `gap RFC7606-5.1-1 closed: the behavior now conforms. Remove {gap} from rfc/short/rfc7606.md ...` (phase 3) |
| the gate ties tag to test | fixture `Check` at its entry point | the seven `check_gap_test.go` tests green (closure run over the commit content) |
| demonstrated gaps are published beside described ones | report and ledger | real tree: 1 demonstrated (rfc7606), 683 described (phase 3 driver); `TestADemonstratedGapNamesItsTest` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | every AC is Done; the other `{gap}` rows are authoring work for their owning specs (Known Limitations) | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | recorded by `./le spec review record` at closure |
| `./le spec review check` | clean |
| Rounds | 2 |
| Reviewer lenses used | wiring, logic and guard audit, removed behavior, docs drift, style pass, security (tag parsing), simplicity |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | the `Tag` comment names `gapTagIssue`, which does not exist | `internal/le/rfc/rfc.go` | names `gapTagRefusal` |
| 2 | ISSUE | the helper header points at `check_core.go`; the gate is in `gaps.go` | `internal/test/rfcgap/rfcgap.go` | header repointed |
| 3 | ISSUE | the new doc section carries no source anchors | `docs/contributing/rfc-conformance-gates.md` | anchors added |
| 4 | ISSUE | `TestADemonstratedGapNamesItsTest` passes when the gap row vanishes | `internal/le/site/rfcdetail_test.go` | `listed` check |
| 5 | ISSUE | four builder comments and one test comment still say MP_REACH last; the journal row still said pending | `update_build_{evpn,labeled,vpn,plugin}.go`, `attribute_test.go`, the journal row | comments name `OrderAttributes`; row marked fixed |
| 6 | ISSUE | `GapCounts` and `DemonstratedGaps` exported with no cross-package caller (`./le repo check`) | `internal/le/rfc/gaps.go` | unexported |

NOTEs: a gap tag in a never-called non-Test function counts, as a proof tag does. A session with a stale `bin/le` refuses `rfc/` commands once a gap tag exists, until `./le --update`.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/test/rfcgap/rfcgap.go`, `rfcgap_test.go` | yes | ls at closure |
| `internal/le/rfc/gaps.go`, `check_gap_test.go` | yes | ls at closure |
| `internal/component/bgp/message/rfc7606_mp_first_test.go` | yes | ls at closure |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-3, AC-12 | helper and pilot | `go test -race ./internal/test/rfcgap/ ./internal/component/bgp/message/ ./internal/core/bgp/attribute/`: ok (closure) |
| AC-4..AC-11 | gate and site | `go test` with an overlay of the commit content, `-run 'Gap|ParseTag|Demonstrat|TestCheck|Tags|Discriminat'` over `./internal/le/rfc/`: ok; site subset ok (closure) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le rfc check` | none (tooling) | `TestCheckAcceptsDemonstratedGap` drives `Check` over a module fixture |
| `./le test unit` | none (tooling) | the pilot test runs through `BuildUnicast` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `TestDemonstrateGapCountsEveryFailureStyle` |
| A-2 | confirmed | `gapDemonstration` reads the `UnitAt` span; `TestCheckRefusesGapTagWithoutHelper` |
| A-3 | confirmed | `./le arch tier check` clean (phase 1) |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| gate refusals and counts | `gapTagRefusal`, `gapCounts`, `check` read at closure | yes |
| MP_REACH order | `OrderAttributes` in `internal/core/bgp/attribute/origin.go` read at closure | yes |
| `./le doc index check` | all references valid | yes |
