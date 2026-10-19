# Spec: dhcpv6-client-1-gap-annotations

| Field | Value |
|-------|-------|
| Status | design |
| Scope | docs |
| Depends | - |
| Phase | 1/4 of `plan/spec-dhcpv6-client-0-umbrella.md` |
| Handoff | - |
| Updated | 2026-10-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

On 2026-10-19 the owner put DHCPv6 Confirm, stateless mode, Reconfigure and
authentication in scope. `rfc/short/rfc9915.md` still annotates their
obligations `{feature-declined}`, whose meaning is that the RFC makes the
feature optional AND ze declined it. The second half is now false.
`ai/rules/rfc-compliance.md` keeps `feature-out-of-scope` for "An OPTIONAL
feature outside the owner's agreed scope"; an applicable requirement of an
in-scope feature that the stack does not meet "is a conformance gap even when
its implementation is outside the agreed session scope". So the rows become
`{gap}` now, before any feature code, and the summary's Meta rows and Purpose
paragraph say what is true. This is the first task of the umbrella
`plan/spec-dhcpv6-client-0-umbrella.md`.

The brief that opened this work counted 39 rows for the four features. The
summary holds 39 `{feature-declined}` rows in total, and 22 of them belong to
the four features. One `{not-applicable}` row (RFC9915-20.2-1) also binds once
authentication is in scope. The other 17 `{feature-declined}` rows belong to
other features (umbrella A-4) and do not change.

### The 23 rows

| Row | Feature | Closed by | Today | Becomes |
|-----|---------|-----------|-------|---------|
| RFC9915-18.2.3-1 | Confirm | child 2 | feature-declined | gap |
| RFC9915-18.2.3-2 | Confirm | child 2 | feature-declined | gap |
| RFC9915-18.2.3-3 | Confirm | child 2 | feature-declined | gap |
| RFC9915-18.2.12-1 | Confirm | child 2 | feature-declined | gap |
| RFC9915-18.2.6-1 | stateless | child 3 | feature-declined | gap |
| RFC9915-18.2.6-2 | stateless | child 3 | feature-declined | gap |
| RFC9915-18.2.6-4 | stateless | child 3 | feature-declined | gap |
| RFC9915-18.2.9-4 | stateless (INF_MAX_RT in Advertise) | child 3 | feature-declined | gap |
| RFC9915-18.2.12-3 | stateless | child 3 | feature-declined | gap |
| RFC9915-21.23-1 | stateless | child 3 | feature-declined | gap |
| RFC9915-21.23-5 | stateless | child 3 | feature-declined | gap |
| RFC9915-21.23-6 | stateless | child 3 | feature-declined | gap |
| RFC9915-21.23-7 | stateless | child 3 | feature-declined | gap |
| RFC9915-21.25-2 | stateless | child 3 | feature-declined | gap |
| RFC9915-21.25-3 | stateless | child 3 | feature-declined | gap |
| RFC9915-21.25-4 | stateless | child 3 | feature-declined | gap |
| RFC9915-15-2 | Reconfigure | child 4 | feature-declined | gap |
| RFC9915-18.2.4-2 | Reconfigure | child 4 | feature-declined | gap |
| RFC9915-18.2.6-3 | Reconfigure | child 4 | feature-declined | gap |
| RFC9915-18.2.11-2 | Reconfigure | child 4 | feature-declined | gap |
| RFC9915-20.3-3 | authentication (replay detection) | child 4 | feature-declined | gap |
| RFC9915-20.3-4 | authentication (replay detection) | child 4 | feature-declined | gap |
| RFC9915-20.2-1 | authentication (one Authentication option per message) | child 4 | not-applicable | gap |

Four rows of these features already carry `{single-polarity: negative}` or
`{single-polarity: positive}` with the reason "ze accepts no Reconfigure at
all": RFC9915-16.11-2, RFC9915-18.2-1, RFC9915-18.2.11-1, RFC9915-22.3-1. The
reason is still true until child 4 lands, they are tested, and the ratchet
blocks losing a polarity, so they stay; child 4 adds the missing polarity and
removes the annotation.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` - "The feature-declined annotation", the ratchet table, "The public ledger's edges"
  → Constraint: `{gap}` forbids `positive` and `negative` tags on its row; no test tags any of the 23 ids today (grep of every `_test.go`), so no polarity is lost
  → Constraint: `{feature-declined}` and `{gap}` both sit inside the gated denominator and outside the proven numerator, so the published share does not move
  → Constraint: `checkGapCountAgreement` reads the spelled number immediately before MUST in `Support remaining`; the vocabulary reaches "twenty-three" (`internal/le/rfc/check_status.go`, tens list)

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc9915.md` - Meta and the 23 rows
  → Constraint: `Support remaining` today opens "No MUST gap on the kernel (netlink) backend"; it becomes a count of 23 and names the four features as in scope and not yet implemented, keeping the VPP sentence and the list of features still not offered
  → Constraint: Purpose says RFC 9915 "removes IA_TA, Server Unicast and the Reconfigure Accept negotiation"; RFC 9915 Appendix A lists "IA_TA option", "UNICAST option" and "UseMulticast status code"
- [ ] `rfc/full/rfc9915.txt` - Appendix A, Section 1.1, Section 21.20
  → Constraint: Section 21.20 "A client uses the Reconfigure Accept option to announce to the server whether the client is willing to accept Reconfigure messages"

**Key insights:**
- The rows need a `{gap: ...}` reason naming the producer that does the narrower thing (the same producer each `{feature-declined}` reason names today) and the owner decision of 2026-10-19.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `rfc/short/rfc9915.md` - the 22 `{feature-declined}` reasons name `dhcp_v6_exchange_linux.go::buildSolicit`, `dhcp_v6_attach_linux.go::noteMove` and the exchange list in `dhcp_v6_linux.go`; RFC9915-20.2-1 names `newV6Message` and `replyMatcher`

**Behavior to preserve:** the text of every row, its id and its level; the 17 other `{feature-declined}` rows; every tag and discrimination record.

**Behavior to change:** the annotation of 23 rows; three Meta cells (`Support coverage` unchanged, `Support remaining` rewritten, `Support status` stays `Partial`); the Purpose paragraph.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `rfc/short/rfc9915.md`, read by `./le rfc check` and by the generator of `docs/features/rfc-status.md`.

### Transformation Path
1. The summary's annotations are parsed into `Requirement.Annotation`.
2. `checkGapCountAgreement` compares the count with `Support remaining`.
3. The public page `docs/features/rfc-status.md` is rendered from the Meta rows.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Summary to public page | the generator named in `docs/contributing/rfc-conformance-gates.md` | No |

### Integration Points
- `internal/le/rfc` checks - they read the annotations; no code changes.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the page is regenerated, never hand-edited |
| No unintended coupling (components stay isolated) | Yes | one summary file and its generated page |
| No duplicated functionality (extends existing, does not recreate) | Yes | no new annotation kind |
| Zero-copy preserved where applicable (refs, not copies) | N-A | no code |
| Registration over hardcoding, outbound | N-A | no code |
| Registration over hardcoding, inbound | N-A | no code |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | moving `{feature-declined}` and `{not-applicable}` to `{gap}` trips no ratchet | the ratchet blocks a lost polarity; the rows carry none | the commit needs an owner ruling row | `./le rfc check` | unvalidated |
| A-2 | no audit verdict file exists for rfc9915 | `rfc/audit/` holds no rfc9915 file | a `not-applicable` verdict would need `unimplemented` | `ls rfc/audit` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | the Remaining count is miscounted | `checkGapCountAgreement` red | count the `{gap` rows with grep before writing the number |
| R-2 | a gap reason names a function that does not exist | reviewer or a producer check | name only producers the current reasons already name |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | the public RFC page misstates the DHCPv6 gaps |
| How is it reverted? | single commit revert |
| Who else touches this path? | any session editing `rfc/short/rfc9915.md` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `rfc/short/rfc9915.md` | → | `checkGapCountAgreement` | `./le rfc check` run over the edited summary |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | the 23 rows of the table | each carries `{gap: ...}` whose reason names the producer and "owner decision 2026-10-19", and none other kind |
| AC-2 | the 17 other `{feature-declined}` rows | byte-identical to HEAD |
| AC-3 | `Support remaining` | opens with "Twenty-three MUST gaps", names Confirm, Information-request (stateless DHCPv6), Reconfigure and the Authentication option as in scope and not implemented, and keeps the VPP sentence and the still-not-offered list |
| AC-4 | Purpose paragraph | names what Appendix A obsoletes (IA_TA, the Server Unicast option, the UseMulticast status code) and no longer says Reconfigure Accept was removed |
| AC-5 | `./le rfc check` | exits 0 |
| AC-6 | `docs/features/rfc-status.md` | regenerated, its RFC 9915 row matches the Meta rows |

### Planned Goal Validation
| Goal | Evidence owed |
|------|---------------|
| the ledger states the owner's current scope | `./le rfc check` exit 0 pasted, grep counts 23 `{gap:` and 17 `{feature-declined:` |
| the public page agrees | the regenerated `docs/features/rfc-status.md` RFC 9915 row quoted |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `./le rfc check` | `internal/le/rfc` | the annotation kinds and the gap count | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| spelled gap count | equals the `{gap}` row count | 23 | 22 | 24 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `./le rfc check` | repository | a reader of the RFC page sees the four features as gaps | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | annotations only, no wire change | |

## Files to Modify
- `rfc/short/rfc9915.md` - 23 annotations, `Support remaining`, Purpose
- `docs/features/rfc-status.md` - regenerated

## Files to Create
- none

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no code |
| YANG validation constraints | N-A | no code |
| YANG custom validators | N-A | no code |
| CLI commands/flags | N-A | no code |
| CLI grammar (keyword before value) | N-A | no code |
| Editor autocomplete | N-A | no code |
| Functional test for new RPC/API | N-A | no code |
| Pipe completeness | N-A | no code |
| Env var registration | N-A | no code |
| Doctor check for runtime dependencies | N-A | no code |
| Prometheus counters/metrics | N-A | no code |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | no behavior changes |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | No | none |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc9915.md`, `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | No | none |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | No | none |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | No | no source file changes |
| 17 | Existing docs show config/CLI/API examples for this area? | No | none |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- run `./le rfc check` on HEAD and keep its output as the baseline.
2. **Phase: Annotations** -- rewrite the 23 annotations, the Remaining cell and Purpose; regenerate the page; run `./le rfc check` once.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | 23 rows, counted by grep |
| Correctness | each gap reason's producer exists in HEAD |
| Rule: rfc-compliance | gap, not exclusion, for an in-scope feature |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| 23 `{gap}` rows | `grep -c "{gap:" rfc/short/rfc9915.md` returns 23 |
| 17 `{feature-declined}` rows | `grep -c "{feature-declined:" rfc/short/rfc9915.md` returns 17 |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | N-A, no code |

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
| `{gap}` | keep `{feature-declined}` until each child lands | the annotation would state a scope decision the owner reversed |
| RFC9915-20.2-1 to `{gap}` | keep `{not-applicable}` | a client receiving an Authentication option fills the role that sentence binds once authentication is in scope |

## Known Limitations
- The 17 other `{feature-declined}` rows are not touched.

## RFC Documentation (Scope: protocol)

No code; the summary rows carry the quotes.

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** summary + generated page + edited spec
- [ ] **Commit B:** `remove plan/pre-release/spec-dhcpv6-client-1-gap-annotations.md` only, in the same `./le commit create` script
