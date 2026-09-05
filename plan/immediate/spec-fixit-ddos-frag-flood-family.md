# Spec: fixit-ddos-frag-flood-family

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**The problem.** `FamilyFragFlood` is declared and never produced.
`internal/core/ddosevent/event.go` declares
`FamilyFragFlood AttackFamily = "fragment-flood"` alongside the five families
that do have producers: `FamilyUDPFlood`, `FamilySYNFlood`, `FamilyICMPFlood`,
`FamilyReflection` and the `FamilyGenericFlood` fallback. Grepping the tree for
the constant on 2026-09-05 returns exactly one Go site, the declaration itself:
no non-test code assigns it, and the string `fragment-flood` appears in no YANG
module, no `.ci` fixture and no documentation page.

**Why the classifier cannot produce it.** `classifyFlows`
(`internal/plugins/ddos/detect/characterize.go`) says so in its own doc comment:
"Fragment floods have no on-box conntrack signal (defrag runs before conntrack)
and therefore fall here into generic". Defragmentation runs before conntrack, so
a fragment flood reaches the detector already reassembled, and the flow records
`classifyFlows` reads carry no fragment counter. It falls to
`FamilyGenericFlood`.

**Why it matters.** The constant is a promise the code does not keep. Anything
reading the family set, an operator, a dashboard, or a responder policy keyed on
family, is told ze classifies fragment floods, and it does not. It is not a
silent-wrong-answer defect, because generic-flood is an honest classification of
what conntrack sees. It is a declared capability with no producer.

**The decision, which is the owner's.** Two directions, both defensible:

1. **Delete the constant** and stop claiming the capability. A one-line change
   plus whatever reads the family list.
2. **Implement the classifier** against pre-defrag counters. This needs a
   counter source upstream of defragmentation, which the current `flowRecord`
   does not carry, so it is a data-path change and not a classifier change.

The spec must put both to the owner with their costs, take the answer, and
implement it. Whichever direction is chosen, the family set has to end the work
with a producer for every member (`ai/rules/principles.md`: a feature that is
declared and never reached is not a feature).

**Checked, and it is a single gap rather than a pattern.** The other five
families were checked against the same question on 2026-08-15: `FamilyUDPFlood`,
`FamilySYNFlood`, `FamilyICMPFlood` and `FamilyReflection` all have producers in
`classifyFlows`, and `FamilyGenericFlood` is the fallback every unmatched set
lands on. Fragment flood is the only orphan.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/ddos/ddos-detect-enhancements.md` - what the detector claims to classify
  → Decision: [fill during research]
  → Constraint: [fill during research]
- [ ] `docs/guide/ddos-mitigation.md` - what an operator is told about attack families
  → Constraint: [fill during research]

**Key insights:** (minimal context to resume after compaction)
- One declaration site, no producer, no reader outside Go: the blast radius of deleting is small and measurable.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/core/ddosevent/event.go` - declares the six `AttackFamily` constants; `FamilyFragFlood` is one of them and is assigned nowhere else in the tree.
- [ ] `internal/plugins/ddos/detect/characterize.go` - `classifyFlows` returns a family from the dominant protocol, the port shares and the TCP half-open share, falling to `FamilyGenericFlood`; its doc records that a fragment flood arrives reassembled and lands there.

**Behavior to preserve:** (unless the user explicitly said to change it)
- Every family that has a producer keeps its classification and its confidence contribution.
- A fragment flood keeps being detected and mitigated, whatever it is called.

**Behavior to change:** (only what the user asked for)
- The declared family set holds only families ze can produce, or `FamilyFragFlood` gains a producer.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Flow records arrive at the detector from conntrack or from the flow-export source, as `flowRecord` values.
- The classification leaves as an `AttackFamily` on a published `ddosevent`.

### Transformation Path
1. The detector's tick narrows flows to the victim.
2. `classifyFlows` derives the family, the vector tuple, the top sources and the source entropy.
3. The family reaches the event bus and every consumer keyed on it.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Detector ↔ event bus | the family string travels on `ddosevent` | No |
| Event bus ↔ responder | a policy keyed on family reads the string | No |

### Integration Points
- `ddosevent.AttackFamily` (`internal/core/ddosevent/event.go`) - the declared set every consumer reads.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No consumer outside the tree keys on `fragment-flood` | grep over Go, YANG, `.ci` and docs on 2026-09-05 found only the declaration | deleting the constant breaks a reader | re-running the grep at implementation | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Implementing the classifier needs a pre-defrag counter the data path does not carry | the design cannot name a source for the counter | put the cost to the owner beside the deletion option before any code |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a responder policy keyed on a family name stops matching |
| How is it reverted? | single commit revert |
| Who else touches this path? | the ddos detect specs under `plan/immediate/` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| every declared `AttackFamily` | → | the producer that assigns it | `TestEveryAttackFamilyHasAProducer` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | the declared family set is enumerated | every member has a non-test producer |
| AC-2 | a fragment flood arrives at the detector | it is classified under a family ze can produce, and the operator is told which |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEveryAttackFamilyHasAProducer` | `internal/plugins/ddos/detect/characterize_test.go` | AC-1 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ddos-attack-family-set` | `test/plugin/ddos-attack-family-set.ci` | the operator sees only families ze produces | | <!-- doc-links: ignore (file this skeleton plans and has not created yet) -->

## Files to Modify
- `internal/core/ddosevent/event.go` - the declared family set
- `internal/plugins/ddos/detect/characterize.go` - `classifyFlows` and its doc comment

## Files to Create
- [named at design]

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | [answered at design] |
| Functional test for new RPC/API | | [answered at design] |
| Prometheus counters/metrics | | [answered at design] |

### Documentation Update Checklist
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | | `docs/features.md` |
| 6 | Has a user guide page? | | `docs/guide/ddos-mitigation.md` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- write the failing test that every declared family has a producer
   - Tests: `TestEveryAttackFamilyHasAProducer`
   - Files: `internal/plugins/ddos/detect/characterize_test.go`
   - Verify: the test fails on `FamilyFragFlood`
2. **Phase: [named at design, after the owner chooses delete or implement]**

## Known Limitations
- [filled at design]

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Current Behavior and Data Flow sections completed

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] `./le verify worktree` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
