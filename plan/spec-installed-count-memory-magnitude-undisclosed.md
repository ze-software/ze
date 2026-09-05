# Spec: `count installed` discloses its bound and not its magnitude

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | docs |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`docs/guide/configuration.md` tells an operator that `count installed` costs
memory, and never tells them how much. Its own worked example commits a router to
tens of megabytes per family per peer, and the page does not say so.

Found by an independent review of commit `2eb6a3dda` (the prefix-set counter),
carried into the follow-up fix session, on 2026-08-08. In the reviewer's words:

`docs/guide/configuration.md` says `installed` "keeps one entry per prefix for
that family, so it costs memory in proportion to what the peer sends, bounded by
`maximum`", and its own worked example is `maximum 1000000` with `count
installed`. The set is `prefixCounts.sets`
(`internal/component/bgp/reactor/session_prefix.go`), a `map[string]struct{}`
keyed on each NLRI's wire encoding, so that example is roughly 60 to 80 MB per
family per peer at steady state. An operator who copies the example onto four
peers with two families each is committing to a number the page never names. The
bound is disclosed; the magnitude is not.

The example and the sentence are both in the same section of the page, "What the
count holds: `offered` or `installed`". The example configures `maximum 1000000`
with `count installed` on one peer. Four peers with two families each is eight
sets, which the review put at roughly half a gigabyte.

The scope is a documentation change: state the per-entry cost and the resulting
magnitude for the example the page itself gives, so an operator sizing an
appliance can do the arithmetic. Whether the page should also change its example
to a smaller `maximum`, and whether Ze should refuse or warn on a configuration
whose sets cannot fit, are decisions this spec must put to the owner rather than
take.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/configuration.md` - the section "What the count holds: `offered` or `installed`"
  → Constraint: the page already discloses the bound, so the edit adds the magnitude and does not restate the bound

**Key insights:** (minimal context to resume after compaction)
- The number must be DERIVED from the actual key, which is the NLRI wire encoding, not from a guess at a prefix's size
- The page's own example is the arithmetic that has to be shown

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/reactor/session_prefix.go` - `prefixCounts.sets` is `map[uint32]map[string]struct{}`, one inner set per family that asked for `PrefixCountInstalled`. The inner key is the wire identity of every NLRI the family holds in the session's Adj-RIB-In, and `counts[fk]` is `len(sets[fk])` always. The map is nil unless a family asked for the mode, so a peer that states nothing carries no set and pays nothing
- [ ] `docs/guide/configuration.md` - the section carries the worked example with `maximum 1000000` and `count installed`, the sentence "keeps one entry per prefix for that family, so it costs memory in proportion to what the peer sends, bounded by `maximum` when one is configured", and a block quote about `offered` being drivable below the routes Ze holds

**Behavior to preserve:**
- The set as the counting mechanism: the count is a set cardinality, never a tally of wire events, which is what FRR and BIRD also count
- The existing disclosure of the bound

**Behavior to change:**
- The page must state the magnitude the example commits to

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator reads `docs/guide/configuration.md` while sizing an appliance, and copies the worked example into a peer configuration.
- Format at entry: the configuration text on the page, and the `prefix` container's `maximum` and `count` leaves.

### Transformation Path
1. The operator's config sets `count installed` for a family
2. `prefixCounts` resolves the family key once and allocates the inner set
3. Every accepted NLRI inserts its wire encoding into that set
4. `len(sets[fk])` is the count enforcement reads

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Documentation ↔ code | the page's claim about `prefixCounts.sets` | No |

### Integration Points
- `prefixCounts` (`session_prefix.go`) - the structure whose size the page must describe

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | 60 to 80 MB per million entries is the right order for this map | The review's own estimate, 2026-08-08 | The page publishes a wrong number, which is worse than none | Measure a million-entry set with a benchmark and paste the result | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A published number goes stale when the key or the map implementation changes | The page disagrees with a later measurement | Anchor the paragraph to `session_prefix.go` and derive the number from a benchmark that stays in the tree |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | An operator sizes an appliance from a wrong published number |
| How is it reverted? | Single commit revert; documentation only |
| Who else touches this path? | `plan/deferrals/fixit-bgp-per-family-prefix-enforcement.md` is cited in the same section |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| a benchmark over `prefixCounts.sets` at one million entries | → | the number the page publishes | [benchmark name, fill at design time] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An operator reads the `count installed` section | The page states the per-entry cost and the total for its own worked example |
| AC-2 | The published number | It comes from a measurement in the tree, not from an estimate |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Sizes an appliance for four peers with two families each | page → arithmetic → memory budget | N-A: documentation |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [benchmark name, fill at design time] | `internal/component/bgp/reactor/session_prefix_test.go` | the measured bytes per entry | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `maximum` | as the YANG leaf declares | [value] | [value] | [value] |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A | N-A | Scope is docs; the measurement is a benchmark | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | N-A | N-A | Scope is docs, no wire-visible change | |

## Files to Modify
- `docs/guide/configuration.md` - the `count installed` section
- `internal/component/bgp/reactor/session_prefix.go` - the source anchor and, if the design adds one, the benchmark's subject

## Files to Create
- [fill at design time]

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | |
| YANG validation constraints | | only if the design adds a refusal, which is an owner decision |
| YANG custom validators | | |
| CLI commands/flags | | |
| CLI grammar (keyword before value) | | |
| Editor autocomplete | | |
| Functional test for new RPC/API | | |
| Pipe completeness | | |
| Env var registration | | |
| Doctor check for runtime dependencies | | |
| Prometheus counters/metrics | | a gauge for the set size would make the cost observable |
| BGP family surface (new SAFI / capability / attribute) | | N-A |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | | |
| 2 | Config syntax changed? | | `docs/guide/configuration.md` |
| 3 | CLI command added/changed? | | |
| 4 | API/RPC added/changed? | | |
| 5 | Plugin added/changed? | | |
| 6 | Has a user guide page? | | |
| 7 | Wire format changed? | | |
| 8 | Plugin SDK/protocol changed? | | |
| 9 | RFC behavior implemented, changed, or newly proven? | | |
| 10 | Test infrastructure changed? | | |
| 11 | Affects daemon comparison? | | `docs/comparison.md` |
| 12 | Internal architecture changed? | | |
| 13 | Route metadata keys added/changed? | | |
| 14 | Prometheus counters added/changed? | | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | | |
| 16 | Any changed source file referenced by existing doc source anchors? | | DERIVED: `./le spec citation anchors spec plan/spec-installed-count-memory-magnitude-undisclosed.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | | the worked example on the same page |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- measure the real cost per entry, write the benchmark
   - Tests: [benchmark name]
   - Files: `internal/component/bgp/reactor/session_prefix_test.go`
   - Verify: the benchmark reports bytes per entry over a realistic NLRI mix
2. **Phase: [name]** -- [fill at design time]

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | The published number matches the benchmark output, and the page names the measurement |
| Rule: `ai/rules/evidence.md` | The magnitude is read off a measurement, never off an estimate |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The page states a measured magnitude | `grep` the section for the number and the anchor |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | A peer that sends up to `maximum` decides how much memory Ze commits |

### Failure Routing
| Failure | Route To |
|---------|----------|
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- [fill at closure]

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled
- [ ] Critical Review passes
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only
