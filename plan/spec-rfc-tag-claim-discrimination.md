# Spec: rfc-tag-claim-discrimination

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `-` |
| Handoff | - |
| Updated | 2026-08-31 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

An RFC evidence tag is `RFC requirement: <ID> <polarity>` followed by free prose
stating what the test demonstrates. Every gate in `internal/le/rfc/` reads the
STRUCTURED half: the id must resolve to a summary row, and the polarity is
mandatory and never inferred (`parseTagRest` in `internal/le/rfc/tags.go`).
Nothing reads the PROSE half. Nothing can: it is a sentence.

So a tag can advertise an assertion its body never makes, and the whole gate
stays green. The population is 3,977 tags. `spec-restore-bespoke-interop-assertions`
examined roughly 20 of them and found five that over-claimed, including one
whose scenario omitted the command that put the route on the wire, so the
assertion could not have found anything, and one published as met while the
producing function was unreachable on the rail the tag named.

The goal is a GATE, not an audit. An audit is stale the day it lands: this
repository already runs that route (`rfc/audit/<stem>.json`, `/ze-rfc-audit`)
and after its whole life it covers 1 of 172 enrolled RFCs, and 2 of that one
RFC's verdicts are stale in the tree today. The deliverable is a ninth ratchet
in `./le rfc check` that refuses a tag whose test does not DISCRIMINATE the
claim the tag makes, and a native action that produces the proof a tag needs.

Nobody can read 3,977 prose claims. The question this spec settles is what
replaces reading, and how a gate over that many tags starts without demanding
all of them on day one.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` - the eight ratchets, what each refuses, and the section "What the ratchets cannot see"
  → Decision: the page already NAMES this hole. It says none of the eight catches "a tagged test whose assertions are weakened IN PLACE while the shape stays the same", and lists three mechanisms that partly cover it: `writeWeakening`, `./le commit audit`, `checkAuditFreshness`. All three are CHANGE detectors. None judges a tag that was over-claiming from the first commit.
  → Constraint: a ratchet MUST fire only on a real downgrade and MUST judge nothing where git cannot answer. "A rule that reds the gate on unrelated work gets removed rather than obeyed" is stated on this page, and several sessions share this checkout, so a corpus-wide day-one demand is out.
  → Constraint: the honest-debt pattern is already fixed here. `checkSuperseded` accepts four dispositions and CHECKS a precondition for each, and `checkSummaryDisposition` refuses a summary with no recorded disposition. A new escape hatch follows that shape or it is a blanket opt-out.
- [ ] `docs/contributing/testing.md` - the gomu section and the section "A test that exists is not a test that gates"
  → Constraint: gomu runs UNIT tests only. It never executes `.ci` or `.et`. The 94 `.ci` tags and the 37 interoplab tags are therefore outside any generic mutation route, whatever it costs.
  → Constraint: mutation is currently ADVISORY and "never gates `./le verify current mode full` or CI". A gate that RUNS gomu changes that status and adds a full mutation run to every verify. The design must consume a stored result instead.
  → Decision: this page already prescribes the manual discipline for the non-unit half: "disable the producing function, confirm the test flips to red, and revert". That is the `revert` proof route, already written down as the house method and applied by hand with no record kept.
- [ ] `docs/architecture/testing/test-health.md` - the test-sensitivity ratchet
  → Decision: `testsensitivity` is the closest existing shape: a static detector plus a committed floor in `test/health/sensitivity-baseline.json` that may only go DOWN. It proves a floor file is an accepted Ze pattern for exactly this problem class.
  → Constraint: it scans the WORKING TREE for the ratchet, because "the ratchet must catch an inert test before it is committed, not blame the next change". The discrimination check owes the same population choice.
- [ ] `ai/rules/interop-and-goal-validation.md` - the vacuity clause
  → Constraint: it ALREADY requires the revert-rebuild-red-restore walk before claiming an interop test validates a change, and requires the RED result to be recorded. Today that record has no home and no gate reads it. This spec gives it one.
- [ ] `ai/rules/rfc-compliance.md` - the conformance obligation
  → Constraint: "a test, a golden file, a code comment or an `rfc/audit` verdict that pins non-conformant behavior is the violation with a green bar on top". An over-claiming tag is that failure at tag level: it converts an unproven MUST into a proven one on the public ledger.
  → Constraint: making Ze better proven never needs permission. This work is entirely inside that direction, so no question is owed to Thomas before it runs.
- [ ] `ai/rules/simplicity.md` - the simplest fully correct answer
  → Constraint: the fix cuts machinery, never correctness. A second policy file, a second artifact tree and a second scanner each need to earn their place against the ratchets that already exist.

**Key insights:** (minimal context to resume after compaction)
- The prose half of a tag is unread by every gate. That is the defect class.
- gomu already exists, is vendored, and its `Result` carries per-test kill attribution. What is missing is a link from a requirement to the code that produces it: none exists machine-readably today.
- 4 of the 5 known over-claims sit at interop tier, where generic mutation cannot reach. A mutation-only design would have caught one of them.
- The proof a tag owes is not "the prose is true" (unfalsifiable) but "there exists a recorded, replayable break of the producer under which THIS tagged unit goes red".

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/le/rfc/tags.go` - parses `RFC requirement:` in three shapes. `parseTagRest` validates the id and the mandatory polarity and stops. `ChangedTags` answers which tags a proposed edit disturbs, ignoring comment-only, whitespace-only and Go import-only edits, and treating a DROPPED tag as a change on its own. `behaviorBytes` is the normalization that strips comments and whitespace before the comparison.
- [ ] `internal/le/rfc/carriers.go` - decides whether a tag is evidence at all. Two axes: KIND (`unit`, `functional`, `interop`) and TIER (`verify`, `nightly`, `unrun`). `carrierFor` answers false for `test/draft/` and for `internal/le/` outside `interoplab/`. `ScanTree` walks `internal`, `pkg`, `test` only. An unrun carrier's tag is REFUSED, not counted (`refuseUnrun`).
- [ ] `internal/le/rfc/check.go` - `check` is the whole gate: it assembles baselines, calls every checker in turn, concatenates violations, and computes the report only when there are none. `CheckReport` carries the published counters.
- [ ] `internal/le/rfc/check_ratchets.go` - `checkIDAllocation`, `checkEnrolment`, `checkCoverageRatchet`, `checkEvidenceRatchet`, `checkRetiredRequirements`, `checkLevelRatchet`, `checkNewSummaries`. Every one reasons about ids, polarities, evidence kind and tier, and levels. None reads a tag's prose.
- [ ] `internal/le/rfc/check_extraction.go` - `checkExtractionRatchet` is monotonic against HEAD. `checkDrainFloor` with `requiredFloor` is the one SCHEDULED floor, `ceil(rate x whole months since start)` capped at the enrolled set.
- [ ] `internal/le/rfc/goscope.go` - `UnitAt` is the single definition of "the tagged unit": the enclosing function for Go, the whole file for a `.ci`. `ScopeFunc` and `ScopeFile` name which resolution a lookup got.
- [ ] `internal/le/rfc/audit.go`, `rfc/audit/rfc7606.json` - the human-audit route. Per requirement it stores `verdict`, `note`, `requirement_sha`, and `tests` and `units` maps keyed `path::TestName`. It records NO producer code location machine-readably; the producers appear only inside the prose `note`.
- [ ] `internal/le/verify/engine/stages.go` - `fullStages` lists `stage("rfc", "check")` third, and it is NOT `structural`, so its red is reported but does not refuse a commit outright.
- [ ] `internal/le/testsensitivity/testsensitivity.go` - the assert-nothing and tag-orphan detectors, and the may-only-go-DOWN floor at `test/health/sensitivity-baseline.json`.
- [ ] `internal/le/mutation/actions.go`, `internal/le/mutation/json.go`, `test/mutation/README.md` - `./le mutation combine` merges per-package gomu reports, `record-history` appends per-package scores to `test/mutation/history.ndjson`. Neither reads a mutant descriptor.
- [ ] `vendor/github.com/sivchari/gomu/internal/mutation/engine.go` - `Mutant` carries `ID`, `FilePath`, `Line`, `Column`, `Type`, `Original`, `Mutated`, `Description`, `Context`, `Function`; `Result` carries `Mutant`, `Status`, `TestsRun`, `TestsFailed`, and `TestOutput` of `TestInfo{Name, Package, Status}`. The per-test attribution a claim-scoped check needs is already in the report shape.
- [ ] `internal/le/interoplab/bgp/check_rfc.go`, `internal/le/interoplab/bgp/check_engine.go` - interop checkers tag at function scope and number their assertions through `fail(N, cause)` reaching `checkerFailure`, which renders `scenario %s assertion %d`. 99 numbered assertion sites exist across the bgp and ipsec checkers.
- [ ] `docs/contributing/rfc-implementation-guide.md` - the tag-format section, where an author learns the shape.
- [ ] `.gomuignore` - excludes `cmd/ze/`, generated files, `vendor/`, `gokrazy/modcache/`, `test/`, `tmp/`, `internal/appliance/`. It does NOT exclude `internal/component/` or `internal/plugins/`.

**Measured population (this checkout, 2026-08-31):**

| Set | Count | Carrier | In scope |
|-----|-------|---------|----------|
| `RFC requirement:` in `*_test.go` outside `internal/le/` | 3,802 | `unit`, tier verify | Yes |
| `RFC requirement:` in `internal/le/interoplab/**/*.go` | 37 | `interop`, tier nightly | Yes |
| `RFC requirement:` in `*.ci` | 94 | `functional`, tier verify | Yes |
| `RFC requirement:` in `*.et` | 0 | `editor` | N-A, empty |
| `RFC requirement:` in `internal/le/` outside `interoplab/` | 34 | none, `carrierFor` returns false | No, scanner fixtures |
| `RFC requirement:` in non-test `*.go` | 10 | none, no carrier matches a production path | Reported, not proven (AC-9) |
| Total | 3,977 | | 3,933 provable |

| Fact | Value |
|------|-------|
| Distinct requirement ids carrying a tag | 1,743 |
| Distinct RFC stems carrying a tag | 281 |
| Polarity split over unit tags | 2,128 positive, 1,638 negative |
| Tagged unit test files | 565 |
| Of those, carrying a `//go:build` constraint | 6, being 5 `linux` and 1 `integration && linux` |
| Enrolled RFCs / summaries | 172 / 180 |
| Extraction sign-offs | 41 |
| Recorded audit artifacts | 1, `rfc/audit/rfc7606.json` |
| `// RFC NNNN Section` comments in production Go | 2,550 across 486 files |
| Numbered interop assertion sites | 99 |
| Violations `./le rfc check` reports in this tree today | 3, all from other sessions' uncommitted work |

**Behavior to preserve:** (unless the user explicitly said to change it)
- `./le rfc check` stays read-only and stays a non-structural verify stage.
- Every existing ratchet keeps its current verdict on this corpus. A run over an unchanged tree must report the same violations before and after this work.
- gomu stays advisory. Nothing in `./le verify` gains a mutation run.
- `ScanTree`'s population, `carrierFor`'s refusals, and `UnitAt`'s definition of the tagged unit are consumed as they are, not re-implemented.
- The two-way link stays one-sided: the tag lives in the test and dies with the test, which the header of `internal/le/rfc/tags.go` states. A discrimination record is keyed so it dies with its tag too.

**Behavior to change:**
- `./le rfc check` gains a ninth ratchet reading a new artifact tree.
- `./le rfc` gains one verb that produces and re-verifies that artifact.
- A tag added, or a tagged unit whose behavior changed, owes a discrimination proof in the same change.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le rfc check`, run directly and as the third stage of `./le verify current mode full`. Input is the working tree plus the git HEAD baseline.
- `./le rfc discriminate stem <stem>` and `./le rfc discriminate id <ID>`, run by an author who is adding or repairing a tag. Input is the working tree plus, for the `mutant` route, a gomu report path.

### Transformation Path
1. `ScanTree` answers every tag with its carrier, kind and tier. Unchanged.
2. `UnitAt` resolves each tag to its tagged unit: a function name for Go, the file for a `.ci`.
3. `LoadDiscrimination` reads `rfc/discrimination/<stem>.json` into one record per requirement id, polarity and unit key, where the unit key is `path::UnitName`.
4. `checkDiscriminationRatchet` compares three sets: the tags in the tree, the records on disk, and the records at HEAD. It emits a violation for a record that no longer verifies, for a tag that owes a record and has none, and for a proven count that fell.
5. `./le rfc discriminate` in PROPOSE mode reads a gomu report, keeps the mutants whose file and line fall inside code the tagged unit covers, and prints them as candidate breaks ranked by proximity to the producer the claim names.
6. `./le rfc discriminate` in RECORD mode applies one candidate through the same overlay gomu uses, runs ONLY the tagged unit, requires RED, restores, re-runs, requires GREEN, and writes the record. It refuses to write a proof it did not observe.
7. `render.go` publishes the counts into `ai/RFC-REQUIREMENTS.md`, alongside the extraction and audit counts already there.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `rfc` area to gomu | read a gomu JSON report from a path the caller names; the area NEVER spawns gomu inside `check` | No |
| `rfc` area to the Go toolchain | `go test` over one package with one unit selected and a coverage profile, only inside `discriminate`, never inside `check` | No |
| `rfc` area to git | the HEAD baseline, through the existing readers in `check_baseline.go` | No |
| `check` to the artifact tree | `rfc/discrimination/<stem>.json`, one file per RFC stem, mirroring `rfc/audit/` and `rfc/extraction/` | No |
| Author to gate | the record is authored only through `discriminate`; a hand-written record whose proof does not replay is a violation | No |

### Integration Points
- `check` in `internal/le/rfc/check.go` - one more append of violations, placed with the other ratchets and behind the same `intersects` guard where a HEAD baseline is required.
- `actions` in `internal/le/rfc/actions.go` - one more `leaction.Action`, keyword before value per `ai/rules/cli.md`.
- `selftest` in `internal/le/rfc/selftest_core.go` - one property row per new refusal, which is how every other RFC engine concern is proven in-process.
- `CheckReport` in `internal/le/rfc/check.go` - three counters: proven, owed, escaped.
- `render.go` - the published backlog gains a discrimination section.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | gomu can generate mutants for essentially the whole unit-tag population | measured: 565 tagged unit test files, 6 with a `//go:build` constraint; `.gomuignore` excludes no `internal/component/` or `internal/plugins/` path | the `mutant` route covers less than claimed and more tags fall to the `revert` route, which is hand-written | run gomu over three tagged packages in Phase 2 and count the files it skipped | unvalidated |
| A-2 | a gomu `Result` attributes a kill to a NAMED test, so a mutant killed by an untagged sibling can be told apart from one killed by the tagged unit | the `TestOutput` field of `Result` in `vendor/github.com/sivchari/gomu/internal/mutation/engine.go`, holding `TestInfo{Name, Status}` | claim-scoped kill is not derivable from the report, and `discriminate` must run the tagged unit itself under the overlay: slower, still correct | run gomu on one package with JSON output and read the test attribution back | unvalidated |
| A-3 | the tagged unit is resolvable for every carrier in scope | `UnitAt` returns `ScopeFunc` for Go and `ScopeFile` for a `.ci`, and both are already consumed by the write-time guard | a `.ci` proof cannot name what to run, and the functional route needs a scenario-level key instead | assert `UnitAt` over all 3,933 in-scope tags in a unit test | unvalidated |
| A-4 | an interop tag can cite a numbered assertion, and a `.ci` tag can cite a directive line | measured: 99 `fail(N,` sites in the bgp and ipsec checkers, and `checkerFailure` renders `assertion %d`. NOT verified for `.ci`, which has no numbering | the citation half of AC-8 needs a different key for `.ci`, for example the `expect=` directive text | enumerate the assertion sites and the `.ci` directive shapes in Phase 1 | unvalidated |
| A-5 | roughly 1 tag in 4 over-claims | `spec-restore-bespoke-interop-assertions`: 5 over-claims in a sample of about 20, drawn from ONE package by one session, and two later batches came back clean | the ratchet is either over-built for a rare defect or under-built for a common one; neither changes whether the gate is correct | the proven set measures the true rate as it climbs, and the first 100 records are the first honest sample | unvalidated |
| A-6 | no machine-readable requirement-to-producer link exists today | the `units` and `tests` maps in `rfc/audit/rfc7606.json` hold TESTS only, and producers appear in prose `note` fields. 10 production tags exist, 5 carry no polarity, and none is on a carrier | a producer index exists and the `mutant` route can be scoped without a coverage profile | grep the audit schema and `internal/le/rfc/audit.go` for any producer field | unvalidated |
| A-7 | a mutant descriptor keyed on the producer function name plus the tagged unit's normalized text hash survives ordinary refactoring better than a line number | the re-stamp history in `rfc/audit/rfc7606.json`: two whole paragraphs exist because `tagged_unit_shas` hashes the enclosing FILE and every key shifted by the height of an inserted header | records rot on mechanical edits, and this work reproduces the re-stamp burden it was meant to avoid | replay the records across a mechanical rename in Phase 4 | unvalidated |
| A-8 | the 3 violations `./le rfc check` reports in this tree are other sessions' work and will be gone before this spec is implemented | `check_rfc.go` was edited by the restore work and by session `rfc-gate`, both uncommitted elsewhere | the implementing session cannot tell its own red from the tree's | re-run `./le rfc check` at the start of implementation and record the baseline | unvalidated |
| A-9 | the interop tier costs 21 to 150 seconds per scenario warm and 353 seconds cold | measured during `spec-restore-bespoke-interop-assertions`, quoted by the commissioning thread, not re-measured here | the `revert` route at interop tier is more expensive than budgeted, and the record must be produced by a nightly job rather than by hand | time one `./le integration interop` scenario in Phase 3 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Generic mutation cannot express a behavioural claim. 4 of the 5 known over-claims are of that shape: "FRR installs the route via the link-local next hop", "neither speaker advertised a Multiprotocol capability", "installed by a real FRR", and a scenario whose setup omitted the route entirely. An arithmetic, conditional or return-value operator falsifies none of them | the proposer returns no candidate whose mutated text touches any symbol the claim's prose names | the `revert` route is a first-class proof, not a fallback: it names a producer symbol to disable and records the observed red. The `mutant` route is the CHEAP route for the 3,802 unit tags, never the only one |
| R-2 | A floor set low enough to be reachable proves nothing. A ratchet that starts at 0 proven and only forbids going below 0 is inert, and this corpus already carries one inert quota: `rfc/drain-budget.txt` ships at rate 0 and is STILL 0 by owner ruling of 2026-08-31 | the proven count sits unchanged over several weeks of RFC work | the floor is not a number. The obligation is CHANGE-SCOPED: a tag added since HEAD, or a tagged unit whose behavior changed under the `ChangedTags` predicate, owes a record now. The count then climbs at the rate RFC work happens, and it cannot stall while that work continues |
| R-3 | Mutation at interop tier is expensive: 21 to 150 seconds per scenario warm and 353 cold (A-9), over 37 interop tags | a `check` run gets measurably slower | `check` NEVER runs a scenario or a mutant. It reads a recorded proof and verifies the fingerprints the proof was taken against, exactly as `checkAuditFreshness` does. Producing a proof is a deliberate `discriminate` run by an author |
| R-4 | The 5-in-20 rate comes from one package and one session, and two later batches were clean. The real rate over 3,977 is uncounted | the first 100 records find almost no survivors | the design does not depend on the rate. A ratchet that finds nothing is cheap, and the same ratchet is what makes a future over-claim impossible to land |
| R-5 | Making a gate depend on gomu changes gomu's status. Today it is advisory and never gates | `./le verify current mode full` grows a mutation run | `check` consumes a STORED descriptor and never invokes gomu. The dependency is on the report FORMAT, which is vendored, not on a run |
| R-6 | A stored break rots. The audit artifact already burns whole re-stamp paragraphs on mechanical edits that shifted every key by a fixed offset | verifying the records reports mass staleness after an unrelated rename | key the record on the producer function NAME plus the tagged unit's normalized text hash, using the `behaviorBytes` normalization that already strips comments and whitespace, never on a line number |
| R-7 | A lazy break passes the gate and proves little. Negating a condition nothing observes yields a red for the wrong reason | records cluster on trivial operators far from the claim's subject | the proposer RANKS candidates by whether the mutated text touches a symbol the claim's prose names, and the record stores the mutated text so a reviewer can judge it. This reduces the judgement, it does not remove it, and the spec says so rather than pretending otherwise |
| R-8 | The ratchet reds the tree on unrelated work. `docs/contributing/rfc-conformance-gates.md` states that such a rule gets removed rather than obeyed, and several sessions share this checkout | a session that touched no RFC test sees a discrimination violation | scope is HEAD-relative and change-scoped, like every other ratchet. Where git cannot answer, judge nothing |
| R-9 | The escape hatch becomes the answer. `{gap}` is cheap, and an unchecked `no-break` would be cheaper | the escaped count rises faster than the proven count | the escape takes a closed vocabulary with a checked precondition, mirroring the four dispositions of `checkSuperseded`, and it is REFUSED outright for a unit-tier tag whose producer gomu can mutate. The count is published in `ai/RFC-REQUIREMENTS.md` as debt |
| R-10 | Coverage-scoped mutation cannot see an UNREACHED producer, and one of the five over-claims, the BMP one, was exactly a producer unreachable on the rail the tag named | a claim names a symbol that appears in no coverage profile of its own tagged unit | that is itself the violation, and it is cheaper to detect than a mutant: a `revert` record naming a symbol the tagged unit never executes is refused (AC-6) |
| R-11 | The artifact tree grows to 281 files and becomes a merge-conflict surface across concurrent sessions | two sessions edit the same `rfc/discrimination/<stem>.json` | one file per RFC stem, records sorted by requirement id then unit key, one record per line-oriented object, matching how `rfc/audit/` and `test/mutation/history.ndjson` already partition |
| R-12 | The work does not fit one package. 3,933 tags, three carriers, a new artifact, a new action and a new ratchet is a large surface | Phase 2 runs long and Phase 3 has not started | the phases are ordered so the gate lands correct-but-inert first (ratchet, artifact, verification), and the proving of the standing corpus is what the change-scoped obligation does over time, not a phase of this spec |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | `./le rfc check` is the third stage of `fullStages` in `internal/le/verify/engine/stages.go`. A false violation reds the verify stage for every session in this checkout. It is NOT `structural`, so it does not refuse a commit outright, which bounds the damage to a reported red |
| How is it reverted? | single commit revert. The artifact tree is additive and no existing ratchet reads it, so removing the check and the tree returns the gate to its current verdict |
| Who else touches this path? | `internal/le/rfc/` is being edited concurrently: `check_extraction.go`, `check_status.go`, `carriers.go` and `check_test.go` all carry recent or uncommitted work, and 3 violations stand in the tree from other sessions. `internal/le/interoplab/bgp/check_rfc.go` carries an uncommitted repair from session `rfc-gate` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le rfc check` over a fixture tree holding an unverifiable record | → | `checkDiscriminationRatchet` | `TestCheckDiscriminationRatchetReportsBrokenProof` |
| `./le rfc check` over a fixture tree holding a tag added since HEAD with no record | → | `checkDiscriminationRatchet` | `TestCheckDiscriminationRequiresProofForNewTag` |
| `./le rfc discriminate stem <stem>` from the action table | → | `discriminateAnswer` | `TestRFCActionsCarryDiscriminateVerb` |
| `./le rfc discriminate` record mode over a fixture that stays GREEN under the break | → | `recordDiscrimination` | `TestDiscriminateRefusesUnobservedRed` |
| `./le rfc selftest` | → | the discrimination property rows | `TestSelftestCoversDiscriminationProperties` |
| `./le verify current mode full` | → | `stage("rfc", "check")` in `fullStages` | the existing stage-list test in `internal/le/verify/engine/verifyengine_test.go` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | a `mutant` record whose stored break no longer makes its tagged unit fail | `./le rfc check` reports a violation naming the requirement id, the polarity, the tagged unit key, the mutated text and the producer function; exit code 2 |
| AC-2 | a tag on an enrolled RFC's gated requirement, present in the tree and absent at HEAD, with no discrimination record | a violation naming the file, line, requirement id and polarity, and stating which proof route applies to its carrier kind |
| AC-3 | a tagged unit whose behavior changed since HEAD under the `ChangedTags` predicate, whose record was not re-verified in the same change | a violation naming the unit and the stale record. A comment-only, whitespace-only or Go import-only edit produces NO violation |
| AC-4 | a record present at HEAD and absent in the tree, while its tag is still present | a violation: the proven set is monotonic. When the TAG is gone too, no violation, and the orphaned record is reported as removable |
| AC-5 | `./le rfc discriminate stem <stem>` over a stem with unproven tags, given a gomu report path | it prints one candidate break per unproven unit-tier tag, drawn from mutants inside the code that tagged unit covers, ranked by whether the mutated text touches a symbol the claim's prose names, and writes nothing |
| AC-6 | a `revert` record naming a producer symbol that does not resolve, or that the tagged unit's coverage profile never executes | a violation. An unreachable producer is the defect, not a proof of one |
| AC-7 | a `no-break` disposition on a unit-tier tag whose producer resolves and lies in a package gomu can mutate | refused. The escape is accepted only for a reason in the closed vocabulary whose precondition holds, and every accepted escape is counted and published in `ai/RFC-REQUIREMENTS.md` |
| AC-8 | an interop or functional record citing an assertion the carrier does not contain: an assertion number beyond the checker's numbered sites, or a `.ci` directive absent from the file | a violation naming the citation and the carrier |
| AC-9 | a non-test Go file outside every carrier containing `RFC requirement:` | reported by `./le rfc check` as a tag nothing scans. This covers the 10 such tags in the tree today, 5 of which carry no polarity and would fail `parseTagRest` if any scanner read them |
| AC-10 | a tree whose records all verify, whose new tags all carry proofs, and whose escapes all hold | exit 0, and the report text carries three counters, proven, owed and escaped, in the same line format as the existing evidence and sign-off counts |
| AC-11 | `./le rfc discriminate` record mode where the break does NOT make the tagged unit red | it refuses to write, names what stayed green, and exits non-zero. A proof it did not observe is never recorded |
| AC-12 | the whole existing corpus, unchanged, before and after this work | every pre-existing ratchet reports the identical verdict, and the discrimination ratchet reports 0 owed, because no tag in the tree is new against its own HEAD |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCheckDiscriminationRatchetReportsBrokenProof` | `internal/le/rfc/check_test.go` | AC-1 | |
| `TestCheckDiscriminationRequiresProofForNewTag` | `internal/le/rfc/check_test.go` | AC-2 | |
| `TestCheckDiscriminationIgnoresCommentOnlyEdit` | `internal/le/rfc/check_test.go` | AC-3, negative half | |
| `TestCheckDiscriminationFiresOnChangedUnit` | `internal/le/rfc/check_test.go` | AC-3, positive half | |
| `TestCheckDiscriminationProvenSetIsMonotonic` | `internal/le/rfc/check_test.go` | AC-4 | |
| `TestCheckDiscriminationOrphanRecordIsRemovable` | `internal/le/rfc/check_test.go` | AC-4, second half | |
| `TestDiscriminateProposesCoveredMutantsOnly` | `internal/le/rfc/discriminate_test.go` | AC-5 | |
| `TestDiscriminateRanksBySymbolInClaim` | `internal/le/rfc/discriminate_test.go` | AC-5, ranking | |
| `TestDiscriminationRevertRequiresReachableProducer` | `internal/le/rfc/discriminate_test.go` | AC-6, R-10 | |
| `TestDiscriminationEscapeRefusedForMutatableUnitTag` | `internal/le/rfc/discriminate_test.go` | AC-7 | |
| `TestDiscriminationEscapeVocabularyIsClosed` | `internal/le/rfc/discriminate_test.go` | AC-7 | |
| `TestDiscriminationCitationMustExistInCarrier` | `internal/le/rfc/discriminate_test.go` | AC-8 | |
| `TestCheckReportsUnscannedProductionTags` | `internal/le/rfc/check_test.go` | AC-9 | |
| `TestCheckReportCarriesDiscriminationCounters` | `internal/le/rfc/check_test.go` | AC-10 | |
| `TestDiscriminateRefusesUnobservedRed` | `internal/le/rfc/discriminate_test.go` | AC-11 | |
| `TestDiscriminationRecordKeyedOnUnitHashNotLine` | `internal/le/rfc/discriminate_test.go` | R-6, A-7 | |
| `TestRFCActionsCarryDiscriminateVerb` | `internal/le/rfc/actions_test.go` | wiring | |
| `TestSelftestCoversDiscriminationProperties` | `internal/le/rfc/selftest_test.go` | one selftest row per refusal | |
| `TestUnitAtResolvesEveryInScopeTag` | `internal/le/rfc/goscope_test.go` | A-3, over the real tree | |
| `TestGomuReportTestAttributionParses` | `internal/le/rfc/discriminate_test.go` | A-2, against a checked-in gomu report fixture | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| interop assertion citation | 1 to the count of numbered `fail` sites in the cited checker | that count | 0 | count plus 1 |
| proven count against HEAD | 0 to the in-scope tag count | equal to HEAD's count | HEAD count minus 1, a violation | N/A, a rise is always allowed |
| mutant line within the producer function span | the function's first to last line | last line | first line minus 1 | last line plus 1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A | N-A | `./le` is the developer tool. Its surface is proven by unit tests and by `./le rfc selftest`, which is the established route for every other RFC engine concern. No `ze` daemon behavior changes, so no `.ci` can reach this work | N-A |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | N-A | N-A | Scope is `tooling`. No wire-visible behavior changes: this work reads tags and records, and changes no protocol code, which is the pure-tooling exemption in `ai/rules/interop-and-goal-validation.md`. What it does for interop is make the EXISTING interop evidence checkable, proven by AC-8 over the 37 interop tags | N-A |

## Files to Modify
- `internal/le/rfc/check.go` - call the new ratchet; three counters on `CheckReport`; the report text line
- `internal/le/rfc/check_ratchets.go` - `checkDiscriminationRatchet`, beside the eight it joins
- `internal/le/rfc/actions.go` - the `discriminate` verb, keyword before value
- `internal/le/rfc/rfc.go` - the artifact path and the closed sets: proof routes and escape vocabulary
- `internal/le/rfc/check_baseline.go` - the HEAD-side reader for the record set
- `internal/le/rfc/selftest_core.go` - one property row per refusal
- `internal/le/rfc/render.go` - publish proven, owed and escaped in the generated backlog
- `internal/le/rfc/check_test.go`, `internal/le/rfc/selftest_test.go` - the tests above
- `docs/architecture/core-design.md` - named because every file in `internal/le/rfc/` declares it in its `// Design:` header, so the spec-citation anchor audit requires it here
- `docs/contributing/rfc-conformance-gates.md` - a ninth row in "The ratchets", a new section for the proof routes and the escape vocabulary, and a correction to "What the ratchets cannot see"
- `docs/contributing/testing.md` - the gomu section gains its scoped consumer, and "A test that exists is not a test that gates" gains the recorded-revert route
- `docs/contributing/rfc-implementation-guide.md` - the tag-authoring section gains the proof a new tag owes
- `ai/INDEX.md` - a keyword row for discrimination, proof route, over-claim, and `./le rfc discriminate`
- `ai/CODE-TO-DOCS.md`, `ai/DOCS-TO-CODE.md` - the new source files
- `ai/rules/points/rfc-compliance/directives/` - the authoring obligation as a point file, then `./le rules render-update`. `ai/rules/rfc-compliance.md` is generated and is never hand-edited
- `ai/skills/ze-rfc.md`, `ai/skills/ze-rfc-audit.md` - the tag author's and the auditor's routes, then `./le ai skills-sync`

## Files to Create
- `internal/le/rfc/discriminate.go` - the record type, the two proof routes, the escape vocabulary, the loader
- `internal/le/rfc/discriminate_action.go` - propose, record and verify modes
- `internal/le/rfc/discriminate_test.go` - the tests above
- `rfc/discrimination/README.md` - the artifact contract, in the shape of `rfc/extraction/README.md`
- `rfc/discrimination/<stem>.json` - written by `discriminate`, one per RFC stem, as tags are proven
- `internal/le/rfc/testdata/gomu-report.json` - a checked-in gomu report fixture for A-2

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | `./le` is the developer tool. It has no YANG surface and reads no daemon config |
| YANG validation constraints | No | no YANG leaf is added |
| YANG custom validators | No | no YANG leaf is added |
| CLI commands/flags | Yes | `internal/le/rfc/actions.go`, one `leaction.Action` in the existing `rfc` area |
| CLI grammar (keyword before value) | Yes | `stem <stem>`, `id <ID>` and `report <path>` as `leaction.Parameter` rows, per `ai/rules/cli.md`, matching `extraction-create` and `tagged-scope` |
| Editor autocomplete | N-A | the `le` action table is not a YANG-completed surface; the verb is discovered through `leaction.List` |
| Functional test for new RPC/API | N-A | no RPC and no API. `./le rfc selftest` is the established proof route for this area, and it gains rows |
| Pipe completeness | Yes | the action returns structured data through `leaction`, which is what renders `\| json`, `\| yaml` and `\| table`; no bespoke printing |
| Env var registration | No | no `ze.*` env var. The artifact path is a constant in `rfc.go`, beside the other corpus paths |
| Doctor check for runtime dependencies | N-A | no new runtime dependency for the daemon. gomu is already vendored and already recorded in `tools.go`, and `check` never invokes it |
| Prometheus counters/metrics | N-A | a developer gate emits no daemon metrics. The three counters are report fields, published in `ai/RFC-REQUIREMENTS.md` |
| BGP family surface (new SAFI / capability / attribute) | N-A | no SAFI, capability or attribute. The work reads tags; it does not implement protocol |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | `docs/features.md` lists product features of the daemon. A developer gate is not one |
| 2 | Config syntax changed? | No | no YANG leaf and no config file syntax changes |
| 3 | CLI command added/changed? | N-A | `docs/guide/command-reference.md` documents the `ze` CLI. The verb is added to `le`, whose surface is documented in the `./le rfc` row of `ai/INDEX.md` and in `docs/contributing/rfc-conformance-gates.md`, both named in Files to Modify |
| 4 | API/RPC added/changed? | No | no RPC. `docs/architecture/api/commands.md` describes the daemon's command API |
| 5 | Plugin added/changed? | No | no plugin |
| 6 | Has a user guide page? | N-A | the audience is a contributor, so the page is `docs/contributing/rfc-conformance-gates.md`, not one under `docs/guide/` |
| 7 | Wire format changed? | No | no wire-visible change |
| 8 | Plugin SDK/protocol changed? | No | no SDK and no process-protocol change |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | no requirement changes level, polarity or coverage. What changes is what a proof MEANS, which is `docs/contributing/rfc-conformance-gates.md`, not `rfc/short/` and not a `docs/features/rfc-status.md` row |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/testing.md` for the gomu consumer and the recorded-revert route, and `docs/architecture/testing/test-health.md` for the relationship to the sensitivity ratchet. `docs/functional-tests.md` only if a route reaches a functional suite |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` compares daemons, and no daemon behavior changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md`, which every file in `internal/le/rfc/` declares as its design owner |
| 13 | Route metadata keys added/changed? | No | no route metadata |
| 14 | Prometheus counters added/changed? | No | no metric is registered |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | one registered `le` action is added, so the `./le rfc` row of `ai/INDEX.md` is extended. `docs/plugin-overview.md`, `docs/features/plugins.md` and `docs/guide/status.md` are daemon inventories and are unaffected |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED at implementation time by `./le spec citation anchors spec plan/spec-rfc-tag-claim-discrimination.md`. Known now: every file in `internal/le/rfc/` declares `// Design: docs/architecture/core-design.md`, which BLOCKS, and it is named in Files to Modify. `ai/DOCS-TO-CODE.md` carries advisory source rows for the whole `internal/le/rfc/` file list and gains the two new files |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the tag-format section of `docs/contributing/rfc-implementation-guide.md` shows the tag in Go and in `.ci`. It must show what proof a new tag now owes, or an author copies an example that no longer passes the gate |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the ratchet exists, is called, and is proven inert on the current corpus
   - Tests: `TestRFCActionsCarryDiscriminateVerb`, `TestCheckReportCarriesDiscriminationCounters`, `TestSelftestCoversDiscriminationProperties`
   - Files: `internal/le/rfc/actions.go`, `internal/le/rfc/check.go`, `internal/le/rfc/rfc.go`, `internal/le/rfc/discriminate.go` for the types only, `internal/le/rfc/selftest_core.go`
   - Verify: `./le rfc check` still reports exactly the violations it reported before this phase (AC-12), and the three counters appear. The wiring tests fail until the verb and the counters exist
2. **Phase: The record and its verification** -- what a proof IS, and how `check` re-checks one
   - Tests: `TestCheckDiscriminationRatchetReportsBrokenProof`, `TestDiscriminationRecordKeyedOnUnitHashNotLine`, `TestUnitAtResolvesEveryInScopeTag`, `TestGomuReportTestAttributionParses`
   - Files: `internal/le/rfc/discriminate.go`, `internal/le/rfc/check_ratchets.go`, `internal/le/rfc/testdata/gomu-report.json`, `rfc/discrimination/README.md`
   - Verify: A-1, A-2, A-3 and A-7 each move off `unvalidated`. Record what gomu actually skipped over three tagged packages
3. **Phase: The two proof routes and the escape** -- `discriminate` produces a proof it observed
   - Tests: `TestDiscriminateProposesCoveredMutantsOnly`, `TestDiscriminateRanksBySymbolInClaim`, `TestDiscriminateRefusesUnobservedRed`, `TestDiscriminationRevertRequiresReachableProducer`, `TestDiscriminationEscapeRefusedForMutatableUnitTag`, `TestDiscriminationEscapeVocabularyIsClosed`, `TestDiscriminationCitationMustExistInCarrier`
   - Files: `internal/le/rfc/discriminate_action.go`, `internal/le/rfc/discriminate_test.go`
   - Verify: A-4 and A-9 move off `unvalidated`. Prove one tag of each carrier kind end to end: one unit tag by `mutant`, one `.ci` tag by `revert`, one interop tag by `revert` with an assertion citation
4. **Phase: The ratchet's obligations** -- new tags and changed units owe a proof
   - Tests: `TestCheckDiscriminationRequiresProofForNewTag`, `TestCheckDiscriminationFiresOnChangedUnit`, `TestCheckDiscriminationIgnoresCommentOnlyEdit`, `TestCheckDiscriminationProvenSetIsMonotonic`, `TestCheckDiscriminationOrphanRecordIsRemovable`, `TestCheckReportsUnscannedProductionTags`
   - Files: `internal/le/rfc/check_ratchets.go`, `internal/le/rfc/check_baseline.go`
   - Verify: replay the records across a mechanical rename and confirm no mass staleness (R-6). Force the ratchet RED by breaking one recorded proof, then restore
5. **Phase: Documentation and discovery** -- the pages that describe the gate describe this one
   - Tests: `./le doc check verify`, `./le spec citation anchors`
   - Files: every row of the Documentation Update Checklist answered Yes
   - Verify: `./le rules render-update` and `./le ai skills-sync` run after the point-file and skill edits, never a hand edit of a generated file

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol, and AC-12 is demonstrated by a before-and-after run over the unchanged corpus |
| Correctness | The ratchet judges NOTHING where git cannot answer, matching every other ratchet in `check_ratchets.go`. A tree with no HEAD baseline produces no discrimination violation |
| Correctness | `check` invokes no gomu run, no `go test` and no interop scenario. Prove it by timing `./le rfc check` before and after |
| Fail closed | A record that cannot be READ is a violation, never an absent record. A zero proven count over a non-empty tag set is a broken scan, not a clean corpus (`ai/rules/principles.md`) |
| Escape integrity | Every `no-break` reason has a precondition the gate CHECKS, in the shape of the four dispositions of `checkSuperseded`. An unconditioned reason is the blanket opt-out this spec exists to avoid |
| Data flow | The record is keyed so it dies with its tag. An orphaned record is reported, never silently kept, and never counted as proven |
| Naming | JSON keys kebab-case, and the artifact mirrors `rfc/extraction/` and `rfc/audit/` in layout and in "only dispositions are authored, everything else is derived" |
| Rule: `ai/rules/simplicity.md` | No second policy file. The floor is change-scoped, not scheduled, and the spec states why (R-2, and `rfc/drain-budget.txt` still at rate 0) |
| Rule: `ai/rules/principles.md` | Nothing is declared twice: the carrier table, the tagged-unit definition and the tag scanner are consumed, not re-implemented |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The ninth ratchet exists and is called | `grep -n checkDiscriminationRatchet internal/le/rfc/check.go` |
| The verb is registered | `./le rfc` prints `discriminate` in its Subs line |
| The gate is inert on the unchanged corpus | `./le rfc check` before and after report the identical violation list (AC-12) |
| The gate discriminates | break one recorded proof, `./le rfc check` goes RED naming it, restore, GREEN |
| One proof of each carrier kind exists | `rfc/discrimination/*.json` holds a `mutant` record, a `.ci` `revert` record, and an interop `revert` record with an assertion citation |
| The counts are published | `ai/RFC-REQUIREMENTS.md` carries proven, owed and escaped after `./le rfc index-update` |
| Docs updated | `./le doc check verify` clean, and `./le spec citation anchors spec plan/spec-rfc-tag-claim-discrimination.md` names no unlisted owner |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | `rfc/discrimination/<stem>.json` is repository data, but a malformed record must be a REFUSAL rather than a skipped entry, or a corrupt file reads as a clean corpus |
| Command execution | `discriminate` runs `go test` and reads a gomu report. The unit name and package path reaching an argv come from `ScanTree` and `UnitAt`, never from the record's free-text fields |
| Resource exhaustion | `check` performs no execution. `discriminate` bounds one `go test` invocation per tag with a deadline, in the shape of the git-list deadline in `testsensitivity` |
| Fail open | a record whose producer symbol cannot be resolved must be a violation, never a pass. R-10 is exactly this failure mode |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| `./le rfc check` red on a tag this session never touched | Another session's work (A-8). Record the baseline, do not fix it, do not wait for it |
| gomu cannot mutate a package A-1 assumed it could | Route those tags to `revert`, record the measured skip list, and correct A-1 rather than widening the escape |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The repository already states this hole in its own words. The section "What the ratchets cannot see" in `docs/contributing/rfc-conformance-gates.md` names the three mechanisms that partly cover it, and all three detect CHANGE. A tag that over-claimed in its first commit passes every one of them forever.
- The five over-claims found by `spec-restore-bespoke-interop-assertions` split two ways, and the split decides the design. One, the vacuous ADD-PATH scenario, is a test that asserts nothing reachable. Four are claims whose prose says more than the body checks, and three of those four are at interop tier. A design that only mutates unit code would have caught one of five.
- Ten `RFC requirement:` tags sit in production Go, on no carrier, where no scanner reads them. Five carry no polarity and would be REFUSED by `parseTagRest` if any scanner did. They read as evidence to a human opening the file and are counted by nothing. Found while measuring the population for this spec, and inside this spec's own subject, so it is AC-9 rather than a report elsewhere.
- Interop checkers already number their assertions through `fail(N, cause)`, at 99 sites. That numbering was built for error messages and turns out to be the citation target the interop half of this gate needs. Nothing new has to be authored to point at an interop assertion.
- The audit route measures the cost of the alternative precisely: 1 artifact for 172 enrolled RFCs, and 2 of that one artifact's verdicts are stale in the tree today because a checker function moved. A record that must be re-judged by a human on every edit does not scale past one RFC.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The proof obligation is "a recorded, replayable break under which THIS tagged unit goes red", not "the prose is true" | prove the prose | "The prose is true" is unfalsifiable by a machine. "This break makes this test red" is decidable and replayable, and it is exactly what the hand method in `spec-restore-bespoke-interop-assertions` did 43-plus times to find every in-package over-claim |
| Two proof routes, `mutant` and `revert`, neither subordinate | mutation only | Generic operators cannot falsify "FRR installs the route via the link-local next hop". gomu also runs unit tests only, so 131 of the in-scope tags are outside it by construction. `revert` is the house method already written into `docs/contributing/testing.md`, and this work gives it a record and a gate |
| The break is GENERATED for unit tags and SELECTED by hand | hand-write 3,977 breaks | 3,977 hand-written breaks is not a project anyone completes. gomu already generates the mutants, the coverage profile of the tagged unit narrows them to the code that unit reaches, and ranking by symbols the claim's prose names puts the plausible candidate first. The human decision shrinks from "invent a break" to "pick one and read it" |
| The floor is CHANGE-SCOPED and monotonic, not a dated quota | a schedule shaped like `rfc/drain-budget.txt`, with a start date and a rate | The one dated quota this corpus has shipped inert at rate 0 and is STILL 0, by owner ruling of 2026-08-31, because a quota over incomplete code buys a signature rather than conformance. A monotonic ratchet needs no rate guess and no arming commit, and seven of the eight existing ratchets already have that shape. The count climbs at the rate RFC work touches tags, which is the rate at which new over-claims can be introduced |
| One escape, `no-break`, with a closed vocabulary and a checked precondition | a free-text opt-out; no escape at all | No escape makes the gate unlandable, because some claims genuinely have no expressible break. A free-text opt-out becomes the answer within a week (R-9). The four dispositions of `checkSuperseded`, each with a precondition the gate verifies, is this repository's settled answer to honest debt |
| `check` never executes anything; it re-verifies a stored record | `check` runs the mutant or the scenario | Interop scenarios cost 21 to 150 seconds warm and 353 cold, and `rfc check` is a stage of every full verify. Storing the proof and re-verifying the fingerprints is what `checkAuditFreshness` already does |
| Records key on producer function name plus the tagged unit's normalized text hash | file and line, as the audit artifact does | The audit artifact's own history shows the cost: whole re-stamp paragraphs exist because a 9-line inserted header shifted every key. `behaviorBytes` already normalizes away comments and whitespace and is the right hash input |
| REJECTED: constrain tag prose to a checkable grammar | -- | It rewrites 3,977 prose halves, and a grammar expressive enough for "FRR installs the route via the link-local next hop" is a language nobody wants to write. Worse, it checks the SHAPE of the claim rather than its truth: a well-formed sentence can still name an assertion the body never makes. Closing that means comparing the claim to the body, which is the citation option below, and then the grammar has bought nothing |
| REJECTED: require each tag to cite the assertion line it rests on | -- | Cheap to check, and genuinely useful at interop tier where 99 numbered assertions already exist, so it is ADOPTED there as part of the record (AC-8) rather than as the whole design. As the whole design it fails twice: a cited line rots on every edit above it, which is the re-stamp burden `rfc/audit/` already pays, and a cited assertion can still be too weak for the claim. It proves a pointer exists, not that the pointer discriminates |
| REJECTED: hand-sampling with a documented rate | -- | Already implemented, already running, already measured: `/ze-rfc-audit` plus `rfc/audit/<stem>.json` covers 1 of 172 enrolled RFCs, and 2 of that one's verdicts are stale today. An audit is a photograph, and the defect it photographs re-enters on the next commit. It also gates nothing: a new over-claiming tag lands green whatever last month's sample said |
| REJECTED: package-wide gomu score as the gate | -- | gomu attributes a kill to the package's whole test set, so a mutant killed by an untagged sibling proves nothing about the tag. It also excludes `.ci`, `.et` and everything under `test/`. A package score is a useful trend, which is why `test/mutation/history.ndjson` exists, and it is not a statement about any one claim |

## Known Limitations

- The gate cannot judge whether a break is a GOOD break. It judges that a recorded break exists, lies inside the producer, and makes the tagged unit red. A reviewer still reads the stored mutated text. R-7 states this rather than hiding it.
- The 3,933 in-scope tags that exist today are not proven by this spec. The change-scoped obligation proves a tag when it is added, or when its unit's behavior changes. Proving the standing corpus is a separate decision for Thomas: it is a rate question, and this spec deliberately declines to guess a rate, for the reason `rfc/drain-budget.txt` records.
- `.et` carries zero tags today, so the editor carrier is designed for and untested against real data.
- The `mutant` route depends on a gomu report the author supplies. A stale report proposes stale candidates. The RECORD is still verified by observation (AC-11), so a stale proposal cannot produce a false proof, only a wasted run.

## RFC Documentation (Scope: protocol)

N-A. Scope is `tooling`. This work implements no protocol behavior and changes
no requirement's level, polarity or coverage, so no `// RFC NNNN Section X.Y:`
comment and no wire-format diagram is owed. What it changes is what a PROOF of
a requirement means, which is documented in `docs/contributing/rfc-conformance-gates.md`.

## Review Gate

<!-- Filled at implementation time by /ze-review, never now. Round 1 reviews the
     WHOLE diff with at least two lenses; round N+1 reviews only the fixes round
     N made plus the sibling call sites they touched, and each round's scope is
     written here BEFORE it runs (`ai/rules/planning.md`). -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file, symbol | fixed / deferred / acknowledged |

### Fixes applied
- [per BLOCKER/ISSUE]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE

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
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1 through AC-12 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated in `internal/le/rfc/`, not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### Goal Validation
| Goal | Evidence that proves it |
|------|------------------------|
| A tag can no longer advertise an assertion its body never makes | Take a tag whose recorded proof is `mutant`. Apply the stored break: the tagged unit goes RED. Now WEAKEN that test until it no longer checks the claim and re-apply the break: the unit stays GREEN and `./le rfc check` reports the violation. Paste both runs |
| The new gate is forced RED, so it is proven to discriminate | Three forced reds, each with pasted output: edit one stored mutant descriptor so it no longer kills and see exit 2 naming that record; add a new tag on a gated requirement with no record and see exit 2 naming file, line and id; delete a record whose tag remains and see exit 2 on the monotonic branch. Restore each and confirm exit 0 |
| The gate is inert on the standing corpus, so it does not red unrelated work | `./le rfc check` on the unchanged tree before and after this work reports the IDENTICAL violation list, with 0 owed. Paste both |
| Each of the three carrier kinds is provable in practice, not only in design | One recorded proof per kind, listed by path and requirement id: a unit tag by `mutant`, a `.ci` tag by `revert`, an interop tag by `revert` with an assertion citation. For each, paste the RED the recording run observed |
| The escape is not a blanket opt-out | For each reason in the closed vocabulary, a test that supplies the reason WITHOUT its precondition and gets a refusal. Plus a unit-tier tag whose producer gomu can mutate, refused the escape (AC-7) |
| An over-claim of the kind actually found is caught | Reconstruct one of the five from `spec-restore-bespoke-interop-assertions` in a fixture, the tag's claim naming an observation the body never makes, and show the ratchet reports it. Name which of the five was reconstructed, and why the other four are or are not reachable by the same route |
| gomu stays advisory and the gate stays cheap | `./le rfc check` wall time before and after, from the same warm tree, plus a grep proving `check` reaches no exec path |

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior. N-A here: `./le rfc selftest` rows, per the TDD plan
- [ ] Interop tests for protocol features. N-A here: Scope is tooling and no wire-visible behavior changes

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
