# Spec: RFC evidence strength 1 -- a ratchet from revert proofs to targeted mutant proofs

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Depends | `plan/pre-release/spec-rfc-evidence-strength-0-umbrella.md` (decisions D-2, D-3, D-4); the committed `killedByTheBreak` package-init fix in `internal/le/rfc/discriminate_observe.go` (concurrent session, 2026-09-24) |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-24 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Make a discrimination proof say that the tagged test checks the obligation, not only
that it reaches the producer. Today 1576 of 1651 records (95%) use the `revert` route:
`revertBreak` replaces the producer's whole body with a panic, and any test that
calls the function goes red, whatever it asserts. 73 records use `mutant`: one gomu
mutation of one line inside the producer, which the tagged test must kill.

`recordDiscrimination` (`internal/le/rfc/discriminate_action.go`) records whatever
route the author names. The only route refusal today is for the `no-break` escape:
`escapeCheck.gomuCanBreak` refuses the escape for a unit-carrier tag whose producer
gomu can mutate. `proofRouteFor` (`check_ratchets.go`) even tells the author of an
owed unit tag that `revert` is acceptable.

This child delivers five things:

1. A route ratchet: a record the tip commit adds or changes on a unit carrier whose
   producer gomu can mutate MUST take the `mutant` route. A HEAD `mutant` record
   MUST NOT become a `revert` record.
2. A targeting check on a new `mutant` record (owner decision D-2).
3. The route mix and the upgrade backlog on `./le rfc check`, on
   `./le rfc discriminate stem <stem>`, and per RFC on `ai/RFC-REQUIREMENTS.md`.
4. An upgrade mode for the proposer, which offers mutant candidates for a unit that
   already holds a `revert` record.
5. A pilot upgrade of two documents, rfc7606 (8 revert records) and rfc4271 (29),
   which measures the cost per record and the rate of tests that kill no targeted
   mutant. Child 2 needs both figures for its schedule.

The burn-down of the other revert records is child 2
(`plan/pre-release/spec-rfc-evidence-strength-2-revert-upgrade-burndown.md`).

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` "The discrimination record", "Producing a record: the two proof routes", "The escape"
  → Constraint: the gate runs no test and no mutant; it reads stored records and re-checks fingerprints against COMMITTED code (owner decision 2026-08-31)
  → Constraint: the obligation is change-scoped: a unit the tip commit added against `HEAD^` owes its proof; a grandfathered unit does not
  → Decision: a `.ci` or interop carrier takes `revert` with a `citation`, because no generated break reaches it; the citation ties the red to one assertion
  → Constraint: the escape refusal for a mutatable producer runs on the GATE's path (`escapeCheck.verdict`), not only where records are written, so a hand-written record is caught; the route refusal needs the same placement
- [ ] `rfc/discrimination/README.md` - the artifact contract
  → Constraint: an unknown JSON key is refused; a new field needs a schema change in `validateDiscrimination` and this README together
- [ ] `docs/contributing/testing.md` "Mutation tests (gomu)"
  → Constraint: gomu has no `--tags`; files with build constraints and `.gomuignore` paths are not mutated, so `gomuCanBreak` must stay the one predicate for "a break can be generated"
- [ ] `docs/architecture/core-design.md` - declared by every `internal/le/rfc` file this child edits (`// Design:` header)
  → Constraint: the rfc area stays one command family under `./le rfc`; no new top-level action

**Key insights:**
- `mutantBreak` derives the record's producer with `functionAtLine`, so a mutant record's producer is always the function that holds the mutated line. That is targeting to the FUNCTION. Targeting to the REQUIREMENT is D-2
- `mergeDiscrimination` drops any record with the same cover before it appends, so a mutant record recorded over a revert record replaces it
- `sections.go` already renders "Proven: N (mutant a, revert b)" on the ledger page; `./le rfc check` prints proven and escaped only (`check.go`)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/rfc/rfc.go` - `RouteMutant`, `RouteRevert`, `RouteNoBreak`, the closed route set
- [ ] `internal/le/rfc/discriminate.go` - record schema, `verifyOneDiscrimination`, `discriminationRouteCounts` (returns proven and escaped, no route split), `discriminationStatusOf` (the `discriminate stem` report)
- [ ] `internal/le/rfc/discriminate_action.go` - `recordDiscrimination` takes the route from the request; `mutantBreak` needs `report <path>` and `mutant <file:line:column#n>`; `revertBreak`; `mergeDiscrimination`
- [ ] `internal/le/rfc/discriminate_escape.go` - `gomuCanBreak`: unit carrier, producer non-empty, producer file declares a function, file not ignored by `.gomuignore`
- [ ] `internal/le/rfc/discriminate_propose.go` - `proposeBreaks`: KILLED mutants inside the tagged unit's coverage, ranked by `claimSymbols` against `touchedSymbols`; it proposes for UNPROVEN tags only
- [ ] `internal/le/rfc/check_ratchets.go` - `checkDiscriminationRatchet`, `discriminationOwedErrors`, `proofRouteFor`, `discriminationWithdrawnErrors`
- [ ] `internal/le/rfc/check.go` - the `discrimination:` summary lines
- [ ] `internal/le/rfc/sections.go` - the "Claim discrimination" section and `routePhrase`

**Behavior to preserve:**
- every one of the eleven refusals listed in `docs/contributing/rfc-conformance-gates.md`
- `revert` with `citation` for `.ci` and interop carriers (D-3 option a)
- the existing 1570 verified revert records stay verified: the ratchet is change-scoped and grandfathers them
- the exit code of `./le rfc check` for a tree whose tip commit adds no record

**Behavior to change:**
- a new or changed record on a mutatable unit carrier with route `revert` is refused
- a HEAD `mutant` record replaced by a `revert` record is refused
- `proofRouteFor` names only `mutant` for a mutatable unit carrier
- the summary line and the per-stem report print the route mix and the upgrade backlog

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le rfc discriminate-record rid <ID> polarity <p> route mutant report <gomu.json> mutant <selector>` writes one record
- `./le rfc check` reads `rfc/discrimination/*.json` in the tree and at HEAD and `HEAD^`

### Transformation Path
1. `loadDiscrimination` parses records; `verifyDiscrimination` produces one `DiscriminationVerdict` for each
2. `baselineRecordBlobs` reads the same files at HEAD
3. NEW: `checkDiscriminationRatchet` compares each verified record's cover and route with the HEAD record of the same cover and applies the route rules
4. NEW: a route-mix count over the verdicts feeds `check.go` and `sections.go`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| tree and git HEAD | `check_baseline.go` blob readers | No |
| record writer and gomu report | the author passes the report path | No |

### Integration Points
- `gomuCanBreak` - the route ratchet calls the same predicate, so "a break can be generated" has one definition
- `proposeBreaks` - gains the upgrade population

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the refusal sits in `checkDiscriminationRatchet`, on the gate's path |
| No unintended coupling (components stay isolated) | Yes | `internal/le/rfc` only |
| No duplicated functionality (extends existing, does not recreate) | Yes | reuses `gomuCanBreak`, `mergeDiscrimination`, `proposeBreaks`, `routePhrase` |
| Zero-copy preserved where applicable (refs, not copies) | N-A | tooling |
| Registration over hardcoding, outbound | N-A | no new action; new keywords on the existing `discriminate` action |
| Registration over hardcoding, inbound | Yes | the route set stays `discriminationRoutes` in `rfc.go`; carrier kinds stay in `carriers.go` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A gomu run can be scoped to one package, so an upgrade does not need a whole-repo mutation run | `docs/contributing/testing.md` lists whole-repo and incremental runs only | each pilot document costs a whole-repo run; the pilot measures that cost | run gomu on `internal/component/bgp/message` in the pilot and record the command in `docs/contributing/rfc-conformance-gates.md` | unvalidated |
| A-2 | Most of the 1559 unit-carrier revert records have a producer gomu mutates | 245 of the 251 producer files carry no build constraint, none sits in a `.gomuignore` path | child 2's backlog is smaller and the remainder needs its own route | AC-7 publishes the exact split | unvalidated |
| A-3 | `functionAtLine` never answers a function other than the one that holds the mutated line | `mutantBreak` in `discriminate_action.go` | a mutant record could name the wrong producer | a unit test with two adjacent functions, `TestMutantProducerIsTheEnclosingFunction` | unvalidated |
| A-4 | The claim-symbol tie (D-2 b) is not satisfied by most existing claims without rewording | the proposer ranks by it and ranks zero when nothing matches | a large rewording cost in child 2 | the pilot counts rank-0 best candidates for rfc7606 and rfc4271 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The ratchet reds every session that re-records an unrelated revert record | a `discrimination:` refusal on a commit that did not touch that stem | the ratchet judges only covers whose record CHANGED between `HEAD^` and the tip, the same scope as the owed rule |
| R-2 | A flaky test kills a mutant once and survives on replay, so the record is not reproducible | a second `discriminate-record` of the same selector stays green | `discriminate-record` already demands a red naming the unit; add a second run under the same overlay and refuse a record whose two runs disagree (AC-10) |
| R-3 | A test kills a mutant only by timeout or by a panic in another goroutine | the red output carries no `--- FAIL: <Func>` line | the existing attribution rule (`judgeRed`, `killedByTheBreak`) applies to the mutant route unchanged; the pilot counts how often it is the only evidence |
| R-4 | gomu generation cost per package is too high for 72 producer packages | pilot wall-clock per package above 10 minutes | child 2 schedules per package, not per record, and reuses one report for every record in that package |
| R-5 | The mutant a proposer ranks first is an equivalent or trivial mutant (a log line, an error message string) | the break text changes only a string literal | the targeting check (D-2 b) refuses a break whose touched symbols the claim does not name |
| R-6 | A concurrent session changes `discriminate_observe.go` while this child edits neighbouring files | an unrecognised hunk in the package | start after the init-crash fix is committed (Depends); judge against HEAD |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | `./le verify` for every session (a false refusal), or a weak proof admitted (a false pass). No product behavior |
| How is it reverted? | a single commit revert |
| Who else touches this path? | the concurrent `killedByTheBreak` fix; any session that records a discrimination proof |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le rfc check` over a fixture repo whose tip commit adds a revert record on a mutatable unit carrier | → | `checkDiscriminationRatchet` route rule | `TestCheckRefusesNewRevertOnMutatableUnit` in `internal/le/rfc/check_test.go` |
| `./le rfc check` summary output | → | route-mix count in `check.go` | `TestCheckPrintsDiscriminationRouteMix` |
| `./le rfc discriminate stem <stem> report <path> upgrade` | → | `proposeBreaks` upgrade population | `TestDiscriminateUpgradeProposesForRevertRecords` in `internal/le/rfc/discriminate_test.go` |
| `./le rfc index-update` | → | per-RFC route table in `sections.go` | `TestRenderLedgerPerRFCRouteTable` in `internal/le/rfc/render_ledger_test.go` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | tip commit adds a record, route `revert`, unit carrier, producer in a file gomu mutates | `./le rfc check` exits non-zero; the message names the record's rid, polarity and unit, and names the `mutant` route with the `./le rfc discriminate id <ID> report <gomu.json>` command |
| AC-2 | tip commit changes an existing record on the same cover from `mutant` to `revert` | refused with a message that names the HEAD route and the new route |
| AC-3 | tip commit adds a record, route `revert`, carrier `.ci` or interop, with a valid `citation` | admitted, exactly as today |
| AC-4 | tip commit adds a record, route `revert`, unit carrier, producer in a file with a build constraint or under `.gomuignore` | admitted; `./le rfc check` counts it in a separate "revert, not mutatable" figure |
| AC-5 | a grandfathered revert record that the tip commit did not touch | still verified; no new refusal; exit code unchanged |
| AC-6 | tip commit adds a tag on a mutatable unit with no record | the owed message (`proofRouteFor`) names only the `mutant` route |
| AC-7 | `./le rfc check` on any tree | prints one line: proven by route (mutant, revert on unit mutatable, revert on unit not mutatable, revert on `.ci`, revert on interop), escaped, and the upgrade backlog, which is the count of verified revert records on mutatable unit carriers. On this checkout today the backlog is at most 1559 and the line agrees with a `jq` count over `rfc/discrimination/` |
| AC-8 | `./le rfc discriminate stem <stem>` | lists each revert record on a mutatable unit carrier under an "upgrade owed" heading, and prints the stem's route mix |
| AC-9 | `./le rfc index-update` | `ai/RFC-REQUIREMENTS.md` "Claim discrimination" carries a per-RFC table: RFC, mutant, revert upgradable, revert other, escaped. Sorted by revert upgradable, largest first |
| AC-10 | `./le rfc discriminate-record ... route mutant` whose tagged unit fails on the first run and passes on a second run under the same overlay | refused as not reproducible; no record is written |
| AC-11 | a new `mutant` record whose break touches no symbol the tag's claim names (D-2 option b, if the owner chooses it) | refused, naming the claim symbols and the touched symbols; an existing mutant record is not re-judged |
| AC-12 | `./le rfc discriminate stem <stem> report <path> upgrade` | proposes KILLED mutants in the unit's coverage for each unit whose record is `revert`, ranked as today; a unit with no candidate is listed as "no targeted mutant" |
| AC-13 | `./le rfc discriminate-record` route `mutant` over a cover that holds a verified revert record | the file afterwards holds one record for that cover, route `mutant`; the revert record is gone |
| AC-14 | Pilot: rfc7606 and rfc4271 | every revert record on a mutatable unit carrier in both stems is `mutant`, or its unit is listed as a finding under D-4. The spec records wall-clock per record and per package, and the rate of units with no targeted mutant, in Design Insights |
| AC-15 | the 6 orphan records `./le rfc check` names today | removed in this child's first commit, since each proves a tag that is gone |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCheckRefusesNewRevertOnMutatableUnit` | `internal/le/rfc/check_test.go` | AC-1 | |
| `TestCheckRefusesMutantDowngradedToRevert` | same | AC-2 | |
| `TestCheckAdmitsRevertOnFunctionalAndInteropCarriers` | same | AC-3 | |
| `TestCheckAdmitsRevertOnUnmutatableProducer` | same | AC-4, both the build-constraint and the `.gomuignore` case | |
| `TestCheckGrandfathersUntouchedRevertRecords` | same | AC-5 | |
| `TestOwedMessageNamesOnlyMutantForUnitCarrier` | same | AC-6 | |
| `TestCheckPrintsDiscriminationRouteMix` | same | AC-7, counts against a fixture with every route and carrier | |
| `TestDiscriminateStemListsUpgradeOwed` | `internal/le/rfc/discriminate_test.go` | AC-8 | |
| `TestRenderLedgerPerRFCRouteTable` | `internal/le/rfc/render_ledger_test.go` | AC-9 | |
| `TestDiscriminateRecordRefusesUnreproducibleRed` | `internal/le/rfc/discriminate_action_test.go` | AC-10 | |
| `TestDiscriminateRecordRefusesUntargetedMutant` | same | AC-11 | |
| `TestDiscriminateUpgradeProposesForRevertRecords` | `internal/le/rfc/discriminate_test.go` | AC-12 | |
| `TestMutantRecordReplacesRevertOnSameCover` | `internal/le/rfc/discriminate_action_test.go` | AC-13 | |
| `TestMutantProducerIsTheEnclosingFunction` | same | A-3 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| mutant selector ordinal `#n` | 1 to the number of mutants at that position | the last mutant gomu reports at the position | 0 | one past the last |
| claim-symbol overlap (AC-11) | 0 to N | 1 | 0 | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rfc selftest` rows for the route ratchet | `internal/le/rfc/selftest_state.go` fixtures, run by `./le rfc selftest` | an author commits a revert proof on a unit test and the gate refuses it; a `.ci` revert proof passes | |

## Files to Modify
- `internal/le/rfc/check_ratchets.go` - route rule in `checkDiscriminationRatchet`; `proofRouteFor`
- `internal/le/rfc/discriminate.go` - route-mix counts; `discriminationStatusOf` upgrade list
- `internal/le/rfc/discriminate_escape.go` - expose `gomuCanBreak` as a predicate the ratchet shares (no second definition)
- `internal/le/rfc/discriminate_action.go` - reproducibility re-run; targeting check; `upgrade` keyword
- `internal/le/rfc/discriminate_propose.go` - upgrade population
- `internal/le/rfc/check.go` - summary line
- `internal/le/rfc/sections.go` - per-RFC route table
- `internal/le/rfc/selftest_state.go` - fixtures for the new rule
- `internal/le/rfc/actions.go` - the `upgrade` keyword on the `discriminate` action's help
- `rfc/discrimination/*.json` - remove the 6 orphans; pilot upgrades of `rfc7606.json` and `rfc4271.json`
- `docs/contributing/rfc-conformance-gates.md` - the route ratchet, the route mix, the upgrade mode, the gomu command per package
- `docs/contributing/testing.md` - the gomu per-package command
- `rfc/discrimination/README.md` - the route rule
- `docs/architecture/core-design.md` - declared by the edited files; checked, edited only if the rfc-area paragraph states the route rule
- `ai/rules/points/rfc-compliance/` - the directive "A tag you ADD ... MUST carry a discrimination record" gains "by the mutant route where gomu can break its producer"; then `./le ai rules condensed-update`
- `ai/RFC-REQUIREMENTS.md` - regenerated by `./le rfc index-update`, never by hand

## Files to Create
- none; every test lives in an existing test file

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | le tooling |
| YANG validation constraints | N-A | no YANG |
| YANG custom validators | N-A | no YANG |
| CLI commands/flags | Yes | `upgrade` keyword on `./le rfc discriminate`, `internal/le/rfc/actions.go` |
| CLI grammar (keyword before value) | Yes | `upgrade` is a bare keyword after `report <path>`; `ai/rules/cli.md` |
| Editor autocomplete | N-A | le action |
| Functional test for new RPC/API | N-A | no RPC; `./le rfc selftest` fixtures |
| Pipe completeness | N-A | le action output, not the ze CLI |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | gomu is vendored and runs through `go run` |
| Prometheus counters/metrics | N-A | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | tooling |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | the ze CLI is untouched |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | the pilot changes proofs for rfc7606 and rfc4271; `ai/RFC-REQUIREMENTS.md` regenerated |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/rfc-conformance-gates.md`, `docs/contributing/testing.md`, `rfc/discrimination/README.md` |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | `docs/architecture/core-design.md` checked, see Files to Modify |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | run `./le spec citation anchors spec plan/pre-release/spec-rfc-evidence-strength-1-targeted-mutant-ratchet.md` at implementation start; `docs/architecture/core-design.md` and `docs/contributing/rfc-conformance-gates.md` are declared |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the `discriminate-record` command lines in `docs/contributing/rfc-conformance-gates.md` |

## Implementation Steps

1. **Phase: Wiring** -- the route-mix line and the fixture for the ratchet
   - Tests: `TestCheckPrintsDiscriminationRouteMix`, `TestCheckRefusesNewRevertOnMutatableUnit` (red)
   - Files: `check.go`, `discriminate.go`, `check_test.go`
   - Verify: the summary line prints the split; the refusal test fails because no rule exists
2. **Phase: Route ratchet** -- AC-1 to AC-6
   - Tests: the six `TestCheck*` rows
   - Files: `check_ratchets.go`, `discriminate_escape.go`
   - Verify: red, then green; `./le rfc check` on this checkout keeps its exit code (AC-5)
3. **Phase: Recorder** -- AC-10, AC-11, AC-13 and A-3
   - Files: `discriminate_action.go`
4. **Phase: Proposer and reports** -- AC-8, AC-9, AC-12
   - Files: `discriminate_propose.go`, `discriminate.go`, `sections.go`, `actions.go`
5. **Phase: Docs and rule** -- every row of Files to Modify under `docs/`, `rfc/discrimination/README.md`, `ai/rules/points/`
6. **Phase: Pilot** -- AC-14, AC-15
   - Run gomu per producer package for rfc7606 and rfc4271, propose with `upgrade`, record, and write the cost and the no-candidate rate into Design Insights and into the umbrella's measurement table
   - A unit with no targeted mutant follows D-4

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has an implementation site |
| Correctness | the route rule reads the same `gomuCanBreak` predicate as the escape refusal, with no second copy of the `.gomuignore` read |
| Correctness | the route rule judges only covers whose record changed between `HEAD^` and the tip; a grandfathered record never refuses |
| Correctness | an unreadable baseline prints that it judged nothing, on its own line, and is never green by silence |
| Naming | the summary line uses the route names from `rfc.go` verbatim |
| Rule: `ai/rules/principles.md` | a zero route count is printed only when the verdicts were read; a failed read is an error |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Route ratchet | `TestCheckRefusesNewRevertOnMutatableUnit` passes; `./le rfc selftest` row present |
| Route mix on the gate | `./le rfc check` output line, compared with a `jq` count |
| Pilot | `jq` over `rfc/discrimination/rfc7606.json` and `rfc4271.json` shows no revert record on a mutatable unit carrier outside the D-4 finding list |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Fail-open guard | the route rule must not answer "admitted" when the producer file cannot be read; that case is a refusal naming the unreadable file |
| Hand-written record | a record written without `discriminate-record` still meets the route rule, because the rule runs in `./le rfc check` |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Pilot unit kills no targeted mutant | D-4; never keep the revert record silently and never weaken the claim to fit |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The escape refusal already encodes "a break can be generated for this producer". The route rule is the same fact applied to the weaker proof route, so it must reuse that predicate and not restate it.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Change-scoped route rule plus a published backlog | Refuse every revert record at once | 1559 grandfathered records would red the tree; the repository's rule is that a ratchet over standing debt gets removed rather than obeyed |
| Keep `revert` with citation for `.ci` and interop (D-3 a) | Refuse revert everywhere | no generated break reaches those carriers today; the citation already ties the red to one assertion |
| Replace a revert record in place on upgrade | Keep both records | `mergeDiscrimination` already keys on the cover, and two records for one cover are refused as a duplicate |
| A second observation run for every new mutant record | Trust one red | a flaky red is the cheapest false proof, and the second run costs one test execution |

## Known Limitations

- The 1559 grandfathered records outside the pilot are child 2.
- The mutant route for `.ci` carriers (D-3 b) is not in this child. If the owner chooses it, it is its own spec in `plan/pre-release/`.

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior (N-A: tooling; `./le rfc selftest` fixtures instead)
- [ ] Interop tests for protocol features (N-A: no protocol behavior changes)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/pre-release/spec-rfc-evidence-strength-1-targeted-mutant-ratchet.md` only
