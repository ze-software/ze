# Spec: RFC evidence strength umbrella -- targeted proofs, owed MUSTs, second walks, black-box conformance

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-24 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Owner approval, 2026-09-24:** make Ze's RFC MUST compliance evidence stronger and
complete. The approved direction, in priority order:

1. A ratchet that moves discrimination proofs from `revert` to a targeted `mutant`.
   New tags use `mutant` or a documented stronger route. Existing `revert` records
   are upgraded one document at a time, against a measured backlog. The gate
   reports the route mix.
2. Close the MUST rows that still owe a test or an annotation, and enroll the RFCs
   the ledger calls enrollable, one document at a time.
3. An independent second walk of the extractions signed in the `prose` register,
   running both arithmetics, so that compliance is measured against the RFC text.
4. Later: black-box conformance with external suites and protocol fuzzers, and
   negative-path fuzzing for each RFC error-handling clause (RFC 7606 and similar).

This umbrella holds the measurements, the child specs, the execution order and the
decisions the owner must make. Each child carries its own acceptance criteria.

### Why the current evidence is weaker than its count

A `revert` record replaces the producer function's whole body with a panic. The
tagged unit goes red, and that proves only that the unit REACHES the producer. It
does not prove that the unit checks the obligation the requirement names: any test
that calls the function goes red under a panic, whatever it asserts. A `mutant`
record changes one operator or operand inside the producer (gomu) and requires the
tagged unit itself to fail. That proves the unit observes the value the mutated
line computes.

On 2026-09-24 a `revert` record was found proven by a package-`init` crash:
`decodeSRv6EndXSID` runs from `init` through `register_attr.go`, so every test in
the package shows that red (`plan/journal/green-that-could-not-have-been-red.md`,
2026-09-24 row). A concurrent session is fixing `killedByTheBreak`
(`internal/le/rfc/discriminate_observe.go`). The records that tool fix does not
reach are the reason this programme re-observes every `revert` record it upgrades.

### Measured 2026-09-24 (every figure re-derived by this spec's author)

| Measurement | Value | Derived from |
|-------------|-------|--------------|
| Requirements, summaries | 6354 across 194 | `ai/RFC-REQUIREMENTS.md` header |
| MUST-level | 4459 | same |
| Gated MUST-level in enrolled RFCs | 4068 across 174 RFCs, 6058 tags resolved | `./le rfc check`, `rfc-requirements OK` line |
| MUST-level in un-enrolled or out-of-count summaries | 391 = 4459 - 4068 | sum of the Gated column over the 16 non-enrolled rows of "Coverage by RFC" |
| Outstanding MUST rows | 257, all in 8 non-enrolled summaries | sum of the Outstanding column |
| "Enrollable now" | 8: rfc2003, rfc2473, rfc2784, rfc2890, rfc4862, rfc6071, rfc6482, rfc9319 (75 MUSTs, every one `{not-applicable}`, zero tests) | "Coverage by RFC" |
| Discrimination records on disk | 1651: revert 1576, mutant 73, no-break 2 | `jq` over `rfc/discrimination/*.json` |
| Revert records by carrier | Go unit 1559, `.ci` 16, interop 1 | same, keyed on the unit path |
| Revert records: distinct producers, producer packages | 597 functions in 251 files across 72 packages; 6 of the 251 files carry a build constraint | same, plus the file header |
| Verified proofs | 1643 proven (mutant 73, revert 1570), 2 escaped, 6 orphans removable | `./le rfc check`; `ai/RFC-REQUIREMENTS.md` "Claim discrimination" |
| Unproven backlog | 4206 of 5738 tagged units on a gated requirement of an enrolled RFC | "Claim discrimination" |
| Tagged units added since origin/main without a proof | 511 | `./le rfc check` |
| Extraction sign-offs | 174 enrolled, 174 signed, backlog 0; register rfc2119 98, prose 73, manual-walk 3 | `./le rfc extraction-status` |
| Signed walks uncounted because the stem is not enrolled | 13 | `./le rfc check`, `extraction:` line |
| `unsourced-ids` entries in enrolled prose-register walks | 731, in 71 of the 72 prose stems the ledger lists as enrolled | `jq` over `rfc/extraction/*.json` joined to the enrolled rows |
| `unsourced-ids` entries in enrolled rfc2119-register walks | 794, in 87 of 98 stems | same |
| Reviewer field of the enrolled prose walks | 44 read `claude`; every other one names an agent phase | same |
| Go fuzz targets | 88 `func Fuzz` in 51 files under `internal/` and `pkg/` | `grep` |

**Two numbers in the brief do not mean what they appear to mean.** Both change the
work in child 3, and both are owner decisions below.

| Brief figure | What the tree says |
|--------------|--------------------|
| "257 MUST-level still owe a test or annotation" | 170 of them sit in documents the owner ruled OUT OF SCOPE on 2026-09-01 (rfc6514 133, rfc8362 37). 45 sit in documents declared `third-party`, which leave the conformance count (rfc9582 22, rfc7627 17, rfc3031 6). 42 are Ze-owned (rfc9190 33, draft-ietf-sidrops-8210bis 7, rfc1035 2) |
| "8 RFCs are enrollable now with no new work" | All 8 ALREADY declare `\| Enrolment \| enrolled \|`. `Meta.Enrolled` (`internal/le/rfc/meta.go`) answers false because their `Implementation` is `third-party` (6) or `foundation` (2), which `implementationCounts` keeps out of the count by the 2026-09-21 directive. `renderRollup` (`internal/le/rfc/sections.go`) still labels them "enrollable" and tells the reader that declaring enrolment "would gate them without any new work". Declaring it changes nothing, so the ledger sentence is false |

## Child Specs

| # | Spec | Bucket | Priority | Status |
|---|------|--------|----------|--------|
| 1 | `plan/pre-release/spec-rfc-evidence-strength-1-targeted-mutant-ratchet.md` | pre-release | 1 | design |
| 2 | `plan/pre-release/spec-rfc-evidence-strength-2-revert-upgrade-burndown.md` | pre-release | 1 | skeleton |
| 3 | `plan/pre-release/spec-rfc-evidence-strength-3-owed-musts-and-enrolment.md` | pre-release | 2 | design |
| 4 | `plan/pre-release/spec-rfc-evidence-strength-4-prose-second-walk.md` | pre-release | 3 | design |
| 5 | `plan/spec-rfc-evidence-strength-5-black-box-conformance.md` | plan | 4 | skeleton |
| 6 | `plan/spec-rfc-evidence-strength-6-error-handling-fuzz.md` | plan | 4 | skeleton |

Children 1 to 4 are `pre-release`: `plan/README.md` puts "the RFC evidence Ze owes a
reader outside this repository" there. Children 5 and 6 sit in `plan/`, because the
owner called them later phases and the release can go out without them.

## Execution Order

| Step | Child | Why this order |
|------|-------|----------------|
| 1 | 1 | Every proof recorded before the ratchet lands is recorded by the weak route. The ratchet stops the backlog from growing, and the pilot measures the cost per record that child 2's schedule needs |
| 2 | 3 | Independent of 1. It fixes a false ledger sentence and routes 42 Ze-owned rows to the specs that already own them. It can run in parallel with 1 |
| 3 | 2 | Needs child 1's tooling and its measured cost per record |
| 4 | 4 | Independent of 1 to 3. A second walk that finds a fabricated row changes the requirement list that children 2 and 3 measure against, so the earlier it runs, the less proof is spent on rows that do not exist. It is third only because the owner ranked it third |
| 5 | 5, 6 | After 1 to 4 |

## Related Work (not duplicated here)

| Spec | Relation |
|------|----------|
| `plan/pre-release/spec-followup-rfc-enrollment.md` | Owns the enrolment programme. Child 3 closes the enrolment items this umbrella names and repoints that spec's rows to child 3 |
| `plan/pre-release/spec-rfcgate-6-supported-extraction-signoff.md` | Signs first walks. Child 4 is a SECOND walk of already-signed stems and must not re-sign a stem that spec still walks |
| `plan/pre-release/spec-rfc-requirement-reattribution.md` | Owns the rfc9582 to draft-ietf-sidrops-8210bis move. Child 3 depends on it for 29 of the 257 rows |
| `plan/spec-ipsec-rfc9190.md` | Owns the rfc9190 implementation behind 33 of the 257 rows |
| `plan/spec-fixit-dns-rfc1035-conformance.md` | Owns the 2 rfc1035 rows |
| `plan/pre-release/spec-rfcgate-5-should-level.md` | Waits on MUST-level completion. Nothing here moves it |
| `plan/spec-improve-4-conformance-fixtures.md` | File-driven protocol fixtures. Child 5 decides whether an external suite reuses that format |

## Owner Decisions

| # | Decision | Options | Recommendation |
|---|----------|---------|----------------|
| D-1 | The 170 outstanding rows in rfc6514 and rfc8362 | (a) keep them out of scope, as ruled on 2026-09-01, and disclosed as gaps; (b) implement MVPN and the OSPFv3 Extended LSAs | (a). `ai/rules/rfc-compliance.md` keeps a prior scope decision in force until the owner changes it. Child 3 then closes 87 rows, not 257 |
| D-2 | What makes a `mutant` break "targeted" | (a) the mutated line lies in the recorded producer function, which `functionAtLine` already guarantees; (b) (a) plus a gate check that the tag's claim names at least one symbol the mutated line touches (`claimSymbols` and `touchedSymbols` in `discriminate_propose.go`); (c) (a) plus a sampled human audit | (b) for NEW records. It is decidable, and the proposer already computes it. Its cost: a claim that names no symbol must be reworded, and a reworded claim stales `claim-sha` |
| D-3 | `.ci` and interop carriers, which gomu cannot reach (17 revert records) | (a) keep `revert` with its `citation` for these carriers, which is the documented stronger route; (b) extend the `mutant` route to `.ci` carriers through the overlay `functional.Prepare` already takes | (a) in child 1. (b) for `.ci` only, as its own spec in `plan/pre-release/` if the owner chooses it. The interop observation costs about 576 s per record and has one record |
| D-4 | A tagged test that kills no targeted mutant of its producer | (a) strengthen the test, which `writeWeakening` blocks until the owner approves each unit with `./le rfc approve`; (b) record it as a finding row and leave the revert record standing | (a). A standing revert record over a test that checks nothing is the weak claim this programme exists to remove. The owner's per-unit approval is the cost, and child 2 batches the approvals per document |
| D-5 | A requirement row the second walk finds in no sentence of the RFC | (a) a new `{retracted}` disposition, recorded in `rfc/corrections/<stem>.md` and leaving the gated population, owner-approved; (b) keep the id and rewrite its text to an obligation the RFC does state | (a) for a row with no source sentence, (b) where a real obligation sits nearby. `checkRetiredRequirements` refuses a deleted id, and the level-correction escape needs a verbatim quote that a fabricated row cannot give |
| D-6 | The 794 `unsourced-ids` entries in rfc2119-register walks | (a) leave them to a later second walk; (b) put them in child 4's scope | (a) as its own spec after child 4 measures the fabrication rate on the prose stems |
| D-8 | Who performs the second walk in child 4 | (a) a fresh agent context that is not given the first walk; (b) (a) plus one stem per tranche walked by a person, to calibrate the agent's agreement rate | (b). Two model walks can share a model's blind spot, and one human-walked stem per tranche measures it |
| D-7 | The 4206 tagged units that carry no proof at all | (a) leave them grandfathered, as today; (b) a separate burn-down after child 2 | Not in the approved brief. Named here so that it is a decision and not an omission |

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` - the gate, its ratchets, the extraction sign-off and the discrimination record
  → Constraint: a ratchet compares against committed HEAD (or `HEAD^` for what the tip commit added) and never against the working tree, because several sessions share the checkout (owner decisions 2026-08-31 and 2026-09-01)
  → Constraint: a new ratchet grandfathers the standing backlog by SCOPE (new since HEAD), never by an allowlist; a rule that reds the tree over standing debt gets removed rather than obeyed
  → Decision: the backlog is PUBLISHED as a measurement line that enforces nothing; enforcement is change-scoped
- [ ] `docs/contributing/testing.md` "Mutation tests (gomu)" - gomu is advisory, runs through `go run`, has no `--tags`, and reads `.gomuignore`
  → Constraint: nothing in `./le rfc check` starts a mutation run; the gate reads stored records only
- [ ] `ai/rules/rfc-compliance.md` - the directives this programme strengthens
  → Constraint: a model-produced requirement list is a claim; both arithmetics run; a claim states what the body checks and no more
  → Constraint: a prior owner scope decision stays in force until the owner changes it (D-1)
- [ ] `plan/README.md` - release buckets
  → Decision: children 1 to 4 in `plan/pre-release/`, children 5 and 6 in `plan/`

**Key insights:**
- The route is chosen by the author today: `recordDiscrimination` takes `request.route` as given, and only the `no-break` escape is refused for a mutatable unit (`escapeCheck.gomuCanBreak`)
- `mergeDiscrimination` replaces a record with the same cover, so an upgrade overwrites the revert record in place
- The reverse arithmetic is enforced structurally (`signoff.go`: every gated row is mapped or listed in `unsourced-ids`), but an `unsourced-ids` entry needs no quote, so a fabricated row passes it

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/rfc/discriminate.go` - record schema, `loadDiscrimination`, `verifyDiscrimination`, `discriminationRouteCounts` (proven and escaped only, no split by route)
- [ ] `internal/le/rfc/discriminate_action.go` - `recordDiscrimination`, `mutantBreak` (needs a gomu report and a selector; the producer is `functionAtLine` of the mutant), `revertBreak`, `mergeDiscrimination`
- [ ] `internal/le/rfc/discriminate_escape.go` - `gomuCanBreak`, the predicate "unit carrier, producer resolves, file not in `.gomuignore`"
- [ ] `internal/le/rfc/check_ratchets.go` - `checkDiscriminationRatchet`, `discriminationOwedErrors`, `proofRouteFor` (offers `revert` to a unit carrier)
- [ ] `internal/le/rfc/sections.go` - `renderRollup`, the "Enrollable now" sentence and the `enrollable` state
- [ ] `internal/le/rfc/meta.go` - `Meta.Enrolled`, `implementationCounts`
- [ ] `internal/le/rfc/signoff.go` - the reverse arithmetic over `unsourced-ids`

**Behavior to preserve:** every existing ratchet, the change-scoped obligation, the published backlog lines, and the record schema's closed vocabulary.

**Behavior to change:** see each child.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le rfc discriminate-record` writes `rfc/discrimination/<stem>.json`; `./le rfc extraction-classify` writes `rfc/extraction/<stem>.json`; authors write `rfc/short/<stem>.md`
- `./le rfc check` reads all three plus the tags in test files, and prints the summary lines; `./le rfc index-update` renders `ai/RFC-REQUIREMENTS.md`

### Transformation Path
1. `Collect` parses summaries and tags
2. `loadDiscrimination` and `verifyDiscrimination` judge records against HEAD
3. The ratchets in `check_ratchets.go` compare HEAD and `HEAD^`
4. `render.go` and `sections.go` publish counts

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| le tool and git | committed blobs read through the baseline readers in `check_baseline.go` | No |
| le tool and gomu | a gomu JSON report path passed by the author | No |

### Integration Points
- `./le verify` runs `./le rfc check`, so every new refusal reaches the pre-commit gate with no new wiring

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | every change is inside `internal/le/rfc` and its artifacts |
| No unintended coupling (components stay isolated) | Yes | no product package is touched by children 1, 3 (tool part) and 4 |
| No duplicated functionality (extends existing, does not recreate) | Yes | children extend `gomuCanBreak`, `renderRollup`, `signoff.go` |
| Zero-copy preserved where applicable (refs, not copies) | N-A | tooling |
| Registration over hardcoding, outbound | N-A | no new command registers outside the existing `rfc` action table |
| Registration over hardcoding, inbound | Yes | route and carrier kinds stay the closed sets in `rfc.go` and `carriers.go` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The generated `ai/RFC-REQUIREMENTS.md` matches the tree today | `./le rfc check` reports the same 4068 and 174, and its freshness check did not fire | the 257 and 8 figures are stale | child 3 re-derives both from `./le rfc index-update` output before it starts | unvalidated |
| A-2 | Most revert producers are gomu-mutatable | 245 of 251 producer files carry no build constraint and none sits under a `.gomuignore` path | child 2's backlog is smaller than 1559 and the rest need D-3 | child 1 AC-7 publishes the exact figure | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Other sessions edit `internal/le/rfc` (discriminate-record) while a child runs | a hunk in a child's file that it did not write | children judge against HEAD; a child starts from the committed tool fix and names it in `Depends` |
| R-2 | The upgrade exposes many tests that kill no targeted mutant | child 1's pilot finds more than 10% of records without a killed candidate | D-4 decides the route before child 2 starts; the pilot rate is written into child 2 |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible. A wrong ratchet reds `./le verify` for every session, or passes a weak proof |
| How is it reverted? | A single commit revert per child |
| Who else touches this path? | the concurrent `discriminate-record` fix; `spec-rfcgate-6`; `spec-rfc-requirement-reattribution` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le rfc check` | → | each child's ratchet or render | named in each child's own Wiring Test table; the umbrella has no code of its own |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Every child spec | exists at the path in the Child Specs table, and its status is `ready` or later before its implementation starts |
| AC-2 | Owner decisions D-1 to D-8 | each has a recorded answer in this spec's Key Design Decisions table, quoting the owner, before the child that depends on it starts |
| AC-3 | Programme close | `./le rfc check` prints a route mix with `revert` on unit carriers at 0 or at the count D-4 leaves standing; 87 Ze-owned and third-party outstanding rows closed or D-1 changed; every enrolled prose walk carries a second walk; children 5 and 6 closed or explicitly deferred by the owner |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (none in the umbrella) | - | each child holds its own | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | the umbrella takes no numeric input | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A | - | tooling; each child names its `./le rfc check` fixtures | |

## Files to Modify
- `plan/pre-release/spec-followup-rfc-enrollment.md` - repoint the enrolment rows child 3 now owns (child 3's commit)

## Files to Create
- the six child specs in the Child Specs table

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | tooling under `internal/le/` |
| YANG validation constraints | N-A | no YANG |
| YANG custom validators | N-A | no YANG |
| CLI commands/flags | N-A | children add keywords to existing `./le rfc` actions; each child answers this row |
| CLI grammar (keyword before value) | N-A | answered per child |
| Editor autocomplete | N-A | no editor surface |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | le actions, not ze CLI |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no runtime dependency in the product |
| Prometheus counters/metrics | N-A | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | tooling |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | ze CLI untouched |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | per child: `rfc/short/<stem>.md` and the generated `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/rfc-conformance-gates.md`, `docs/contributing/testing.md`, per child |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | N-A | the umbrella changes no source; each child answers it |
| 17 | Existing docs show config/CLI/API examples for this area? | N-A | answered per child |

## Implementation Steps

1. **Phase: Wiring** -- none in the umbrella; each child's Phase 1 is its wiring
2. **Phase: Children** -- run the children in the Execution Order table, one spec per session

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every brief item maps to a child, and every child is named in this table |
| Correctness | every measured figure is re-derived at the start of the child that uses it |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Six child specs | `ls plan/pre-release/spec-rfc-evidence-strength-* plan/spec-rfc-evidence-strength-*` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Fail-open gate | a new ratchet that judges nothing when git or a report cannot be read must say so on its own line, never render as green (`ai/rules/principles.md`) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| A child's measurement disagrees with this umbrella | the child's figure wins; correct this table in the child's commit |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The brief's two headline counts each mix populations: owner-declined, third-party and Ze-owned rows in the 257, and out-of-count documents in the 8. A programme that measures progress against them reports work that nobody can do or that changes nothing.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| An umbrella plus six children | One spec | four independent work streams with different buckets and different sessions; `ai/rules/planning.md` wants one spec per session |
| Split the revert upgrade (child 2) from the ratchet (child 1) | One child | the ratchet is a few days of tooling; the upgrade is 1559 observations whose cost the pilot must measure first |

## Known Limitations

- D-6 and D-7 are open. If the owner rules them in, each becomes its own spec in `plan/pre-release/`.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written (in each child)
- [ ] Tests FAIL (paste output, in each child)
- [ ] Tests PASS (paste output, in each child)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] **Commit A:** edited spec
- [ ] **Commit B:** `remove plan/pre-release/spec-rfc-evidence-strength-0-umbrella.md` only
