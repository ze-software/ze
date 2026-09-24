# Spec: RFC evidence strength 3 -- close the owed MUST rows and tell the truth about enrolment

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Depends | `plan/pre-release/spec-rfc-evidence-strength-0-umbrella.md` (decision D-1); `plan/pre-release/spec-rfc-requirement-reattribution.md` (29 rows); `plan/spec-ipsec-rfc9190.md` (33 rows); `plan/spec-fixit-dns-rfc1035-conformance.md` (2 rows) |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-24 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The owner asked to close the 257 MUST-level rows that still owe a test or an
annotation, and to enroll the 8 RFCs the ledger calls "enrollable now". Both figures
come from "Coverage by RFC" in `ai/RFC-REQUIREMENTS.md`, rendered by `renderRollup`
(`internal/le/rfc/sections.go`). Measured 2026-09-24, they break down as follows.

### The 257 outstanding rows

| Stem | Outstanding | Implementation | Enrolment | Owner of the work |
|------|-------------|----------------|-----------|-------------------|
| rfc6514 | 133 | ze | out-of-scope (owner, 2026-09-01) | D-1: stays a disclosed gap unless the owner changes the ruling |
| rfc8362 | 37 | ze | out-of-scope (owner, 2026-09-01) | D-1: same |
| rfc9190 | 33 (26 no test, 7 one polarity) | ze | backlog | `plan/spec-ipsec-rfc9190.md` (in-progress) |
| rfc9582 | 22 | third-party | enrolled, out of count | `plan/pre-release/spec-rfc-requirement-reattribution.md` (skeleton) |
| rfc7627 | 17 | third-party | backlog, superseded by RFC 9846 | this spec |
| draft-ietf-sidrops-8210bis | 7 | ze | backlog | `plan/pre-release/spec-rfc-requirement-reattribution.md` |
| rfc3031 | 6 | third-party | enrolled, out of count | this spec |
| rfc1035 | 2 | ze | backlog | `plan/spec-fixit-dns-rfc1035-conformance.md` (blocked) |

So 170 rows are owner-declined, 45 sit in `third-party` documents, and 42 are Ze-owned
rows that three live specs already own. This spec does not duplicate those three. It
owns the 23 third-party rows no other spec owns (rfc7627 17, rfc3031 6), the ledger
defect below, and the enrolment of each Ze-owned document once its owning spec lands.

### The 8 "enrollable" RFCs

rfc2003, rfc2473, rfc2784, rfc2890, rfc4862, rfc6071, rfc6482 and rfc9319 ALREADY
declare `| Enrolment | enrolled |`. Six declare `Implementation` `third-party` and two
declare `foundation`. `Meta.Enrolled` (`internal/le/rfc/meta.go`) ANDs the enrolment
row with `implementationCounts`, which admits only `ze` and `mixed` (owner directive,
2026-09-21). So the eight are out of count, and declaring enrolment again changes
nothing. `renderRollup` computes "ready" as `Outstanding() == 0 && !Enrolled`, so it
lists them as "Enrollable now" and says that declaring enrolment "would gate them
without any new work". That sentence is false, and it is published. The page also
counts their 75 MUSTs among the 391 "in un-enrolled RFCs".

All 75 carry `{not-applicable}`, with reasons that name the kernel module or the cache
that performs the behavior. `ai/rules/rfc-compliance.md` says `{not-applicable}` MUST
NOT be used for a requirement whose role Ze fills through a layer acting on its behalf,
and that a requirement out of the count MUST still carry a test wherever Ze can observe
the behavior. For a tunnel RFC, Ze builds the netlink link descriptor
(`buildIptun`, `buildIp6tnl`, `buildGretun` in
`internal/plugins/iface/netlink/tunnel_linux.go`), which Ze CAN observe.

### The same defect for rfc7627

rfc7627 carries the superseded marker "superseded by RFC9846", but
`rfc/short/rfc9846.md` does not exist. The marker names a home for 17 obligations that
has no summary, so no reader can follow it.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` "Who implements the document", "The superseded marker", "The lower-layer annotation"
  → Constraint: `third-party` and `foundation` leave the count; `mixed` counts Ze's part; a declaration names the implementer and the mechanism
  → Constraint: a requirement out of the count still carries a test wherever Ze can observe the behavior, asserting the state Ze installs for the layer below
  → Constraint: the superseded marker is a fact about the DOCUMENT; its requirements stay gated, counted and ratcheted
- [ ] `ai/rules/rfc-compliance.md`
  → Constraint: `{not-applicable}` MUST NOT be used for a requirement whose role Ze fills, including through a layer acting on Ze's behalf
  → Constraint: a prior owner scope decision stays in force (rfc6514, rfc8362)
- [ ] `docs/architecture/core-design.md` - declared by `sections.go` and `meta.go`
  → Constraint: the generated page is a view; its sentence comes from the render, never from a hand edit

**Key insights:**
- The "enrollable" defect is a guard with two jobs: `!in.Enrolled[rfc]` means both "not declared" and "declared but out of count"
- 316 of the 391 "un-enrolled" MUSTs are in the 8 backlog documents; 75 are in the 8 out-of-count documents

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/rfc/sections.go` - `renderRollup`: "ready" list, the `enrollable` state
- [ ] `internal/le/rfc/meta.go` - `Meta.Enrolled`, `implementationCounts`, `dispositionsFrom`
- [ ] `rfc/short/rfc2003.md`, `rfc2473.md`, `rfc2784.md`, `rfc2890.md`, `rfc4862.md`, `rfc6071.md`, `rfc6482.md`, `rfc9319.md` - Meta rows and annotations
- [ ] `rfc/short/rfc7627.md`, `rfc/short/rfc3031.md` - the 23 third-party rows

**Behavior to preserve:**
- `Meta.Enrolled` and the count it drives (owner directive 2026-09-21)
- the Gated, Both, One polarity, Annotated, No test and Outstanding columns

**Behavior to change:**
- a summary declared enrolled whose kind is out of count is never "enrollable"; the page names it "out of count (third-party)" or "out of count (foundation)"
- the header sentence that counts un-enrolled MUSTs splits "not enrolled" from "out of count"

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `rfc/short/<stem>.md` Meta rows; `./le rfc index-update`

### Transformation Path
1. `Collect` reads the Meta tables
2. `enrolledFrom` builds `in.Enrolled`
3. `renderRollup` renders the state column and the "Enrollable now" sentence

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| summaries and the generated page | `./le rfc index-update` | No |

### Integration Points
- `RenderInput` gains the Meta map or an out-of-count set; no new registry

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the page stays generated |
| No unintended coupling (components stay isolated) | Yes | `internal/le/rfc` only, plus tests at Ze's netlink boundary |
| No duplicated functionality (extends existing, does not recreate) | Yes | reads `implementationCounts`, the one declaration of the division |
| Zero-copy preserved where applicable (refs, not copies) | N-A | tooling |
| Registration over hardcoding, outbound | N-A | none |
| Registration over hardcoding, inbound | Yes | no stem list in code; the state derives from each summary's Meta |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The owner keeps rfc6514 and rfc8362 out of scope (D-1) | Meta `Enrolment` rows dated 2026-09-01 | this spec grows by 170 rows of feature implementation, which needs its own specs | owner answer on D-1 | unvalidated |
| A-2 | RFC 9846 is the document that obsoletes RFC 7627 | the superseded marker in `rfc/short/rfc7627.md` | the marker is wrong and must be corrected | fetch `rfc/full/rfc9846.txt` and read its header | unvalidated |
| A-3 | Ze can observe the tunnel link descriptor it builds for rfc2003, rfc2473, rfc2784 and rfc2890 | `tunnel_linux.go` builds it | a test at Ze's boundary is impossible and the `{not-applicable}` stands with that reason | a unit test over `buildIptun` output | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Rewriting `{not-applicable}` to a stronger annotation trips `checkCoverageRatchet` or the audit ratchet | a `./le rfc check` refusal naming the stem | an annotation change that ADDS a test never loses a polarity; run `./le rfc check` after each stem |
| R-2 | The three owning specs stall, so the Ze-owned 42 never close | the owning spec stays blocked or skeleton | this spec closes on its own ACs and names the three in Work Not Done; it does not absorb their work |
| R-3 | Writing `rfc/short/rfc9846.md` is a new summary, which `checkNewSummaries` requires to enroll or declare | a refusal on the summary commit | declare `Implementation` `third-party` and its enrolment together, with an extraction walk |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | the public ledger states a wrong count; no product behavior |
| How is it reverted? | a single commit revert |
| Who else touches this path? | `spec-rfc-requirement-reattribution`, `spec-followup-rfc-enrollment`, `spec-rfc-ledger-rows-for-the-thirty-two-undeclared` (lists rfc3031) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le rfc index-update` | → | `renderRollup` state column | `TestRollupNeverCallsAnOutOfCountSummaryEnrollable` in `internal/le/rfc/render_test.go` |
| `./le rfc check` page freshness | → | the regenerated `ai/RFC-REQUIREMENTS.md` | existing freshness check in `check_status.go` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | a summary declares `enrolled` and `Implementation` `third-party` or `foundation`, with 0 outstanding | the rollup state reads `out of count (third-party)` or `out of count (foundation)`, and the stem is not in any "Enrollable now" list |
| AC-2 | a summary declares `backlog` and `ze`, with 0 outstanding | still listed as "Enrollable now" (the true case is preserved) |
| AC-3 | the coverage header | states the MUST count in not-enrolled summaries and the MUST count in out-of-count summaries as two figures; on this checkout 316 and 75 |
| AC-4 | each of the 75 MUST rows of the 8 out-of-count summaries | carries a test tag at the boundary Ze owns (the descriptor, sysctl or payload Ze produces), or keeps `{not-applicable}` with a reason stating that Ze produces nothing observable for it; no row keeps a reason that names a layer acting on Ze's behalf without a test |
| AC-5 | rfc3031, 6 outstanding rows | each carries a positive and a negative test at Ze's boundary (`addMPLSSwap` output), or an annotation that meets AC-4 |
| AC-6 | rfc7627, 17 outstanding rows | A-2 resolved; each row carries a test of what Ze reads from `crypto/tls` (`internal/core/eap/eap_tls.go` refuses a key export without extended master secret), or an annotation that meets AC-4; the superseded marker names a document whose summary exists |
| AC-7 | rfc9190, draft-ietf-sidrops-8210bis, rfc1035 | enrolled in this spec's commit that follows the owning spec's closure, and not before; each owning spec's Work Not Done names this spec |
| AC-8 | `plan/pre-release/spec-followup-rfc-enrollment.md` | its rows for the stems above are repointed to this spec |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRollupNeverCallsAnOutOfCountSummaryEnrollable` | `internal/le/rfc/render_test.go` | AC-1 | |
| `TestRollupStillListsATrueEnrollableSummary` | same | AC-2 | |
| `TestRollupHeaderSplitsNotEnrolledFromOutOfCount` | same | AC-3 | |
| boundary tests per stem, named by each requirement id | beside the producer, for example `internal/plugins/iface/netlink/tunnel_linux_test.go` | AC-4 to AC-6 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Outstanding per summary | 0 to Gated | 0 is "enrollable" only when the kind counts | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A | - | tooling; the boundary tests are unit tests over Ze's own producer, which is the boundary the rule names | |

## Files to Modify
- `internal/le/rfc/sections.go` - state column and header
- `internal/le/rfc/render.go` - `RenderInput` carries the out-of-count set
- `rfc/short/rfc2003.md`, `rfc2473.md`, `rfc2784.md`, `rfc2890.md`, `rfc4862.md`, `rfc6071.md`, `rfc6482.md`, `rfc9319.md`, `rfc3031.md`, `rfc7627.md` - annotations per AC-4 to AC-6
- `ai/RFC-REQUIREMENTS.md`, `rfc/requirements/*.md`, `docs/features/rfc-status.md` - regenerated
- `plan/pre-release/spec-followup-rfc-enrollment.md` - AC-8
- `docs/contributing/rfc-conformance-gates.md` - "Who implements the document": the rollup state for an out-of-count summary
- `docs/architecture/core-design.md` - declared by `sections.go`; checked

## Files to Create
- `rfc/full/rfc9846.txt` and `rfc/short/rfc9846.md`, if A-2 holds
- the boundary test files AC-4 to AC-6 name, where none exists beside the producer

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | tooling and tests |
| YANG validation constraints | N-A | none |
| YANG custom validators | N-A | none |
| CLI commands/flags | N-A | none |
| CLI grammar (keyword before value) | N-A | none |
| Editor autocomplete | N-A | none |
| Functional test for new RPC/API | N-A | none |
| Pipe completeness | N-A | none |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | none |
| Prometheus counters/metrics | N-A | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | the ten summaries above and `docs/features/rfc-status.md` (generated) |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/rfc-conformance-gates.md` |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | run `./le spec citation anchors spec plan/pre-release/spec-rfc-evidence-strength-3-owed-musts-and-enrolment.md` at implementation start |
| 17 | Existing docs show config/CLI/API examples for this area? | No | - |

## Implementation Steps

1. **Phase: Wiring** -- `TestRollupNeverCallsAnOutOfCountSummaryEnrollable` red, then the state column
2. **Phase: Header** -- AC-3
3. **Phase: Out-of-count rows** -- AC-4, one stem per commit, tests before annotation changes
4. **Phase: rfc3031 and rfc7627** -- AC-5, AC-6
5. **Phase: Enrolment** -- AC-7 as each owning spec closes; AC-8

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | "enrollable" is computed from the same `implementationCounts` the count uses, with no second list |
| Correctness | no `{not-applicable}` reason still says "the kernel does it" for a row Ze could observe |
| Rule: `ai/rules/rfc-compliance.md` | every new tag's claim states only what its body checks, and carries a discrimination record by the mutant route (child 1) |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No false "enrollable" sentence | `grep -c 'Enrollable now' ai/RFC-REQUIREMENTS.md` and the list it prints |
| 23 third-party rows closed | the Outstanding column for rfc3031 and rfc7627 reads 0 |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Overstated claim | a boundary test tagged as proving the kernel's behavior; the tag states only what Ze produces |

### Failure Routing

| Failure | Route To |
|---------|----------|
| A-2 wrong | correct the superseded marker in the same commit |
| An owning spec is blocked | AC-7 waits; this spec closes on AC-1 to AC-6 and AC-8 and names the rest in Work Not Done |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- A condition that answers two questions ("not declared" and "not counted") produced a published false sentence. The fix reads the one declaration of the division, `implementationCounts`.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Fix the render, not the eight summaries | Change their `Implementation` to `mixed` so they count | the kind is a fact about who implements the document; changing it to reach a count is the fabrication the 2026-09-21 directive forbids |
| Leave the 42 Ze-owned rows with their owning specs | Absorb them here | three live specs own them; a second owner is the "belongs to nobody" failure |

## Known Limitations

- 170 rows in rfc6514 and rfc8362 stay open under D-1 (a).

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced (rfc9846 does not: AC-6)
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
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior (N-A: tooling and boundary unit tests)
- [ ] Interop tests for protocol features (N-A: no wire behavior changes)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] **Commit A:** code + tests + summaries + regenerated pages + edited spec
- [ ] **Commit B:** `remove plan/pre-release/spec-rfc-evidence-strength-3-owed-musts-and-enrolment.md` only
