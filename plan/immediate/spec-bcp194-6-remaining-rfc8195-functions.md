# Spec: BCP 194 child 6 -- the remaining RFC 8195 functions

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | spec-bcp194-0-umbrella |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze produces one RFC 8195 function and no other. RFC 8195 Functions 5, 7 and 8 to
12 have no producer anywhere in the tree, and no spec claims them. This spec is
their destination.

The umbrella spec `plan/immediate/spec-bcp194-0-umbrella.md` recorded the split
on 2026-08-08, in its Known Limitations: child 1
(`plan/immediate/spec-bcp194-1-communities.md`) covers Functions 1 to 4 and 6,
and Functions 5, 7 and 8 to 12 were left with no destination. The umbrella gave
the reason for the split: "The LOCAL_PREF family is a separate design problem,
and both Section 4.3.3 and RFC 4264 warn about it."

The seven functions, as `rfc/short/rfc8195.md` tabulates them:

| Function | Parameter | Action |
|----------|-----------|--------|
| 5 | ISO 3166-1 numeric country code | Do not export to EBGP neighbors in that country |
| 7 | ISO 3166-1 numeric country code | Prepend own ASN once toward EBGP neighbors in that country |
| 8 | 0, or a UN M.49 region code | Normal customer route LOCAL_PREF |
| 9 | 0, or a UN M.49 region code | Backup customer route LOCAL_PREF |
| 10 | 0, or a UN M.49 region code | Peering route LOCAL_PREF |
| 11 | 0, or a UN M.49 region code | Upstream transit route LOCAL_PREF |
| 12 | 0, or a UN M.49 region code | Fallback route LOCAL_PREF |

Parameter `0` means "apply globally within the AS". A UN M.49 region code means
"apply only in that region".

Two design problems separate this work from child 1, and both are why it was
split off rather than folded in.

The LOCAL_PREF family (8 to 12) manipulates local preference across preference
classes. RFC 8195 Section 4.3.3 and RFC 4264 both warn that this creates BGP
Wedgies: unintended stable states, route oscillation, and routing anomalies that
are hard to debug. A design that lets an operator define five preference classes
must state what happens when a route carries two of them, and what the resulting
preference ordering is.

Functions 5 and 7 act on the COUNTRY of an EBGP neighbor. Ze holds no per-peer
country today, so the function needs a source for that fact before it can act.
RFC 8195 Section 4.1.2 also names the limit of the mechanism: "[T]his might not
prevent one of those EBGP neighbors from learning that route in another country
and making it available in the country specified by the BGP Large Community."

RFC 8195 is enrolled `non-normative` (`rfc/short/rfc8195.md`, Meta). A
capitalized MUST / MUST NOT / SHALL / SHALL NOT / REQUIRED scan over
`rfc/full/rfc8195.txt` hits zero keywords, and the document invokes neither RFC
2119 nor RFC 8174 nor BCP 14. Thomas ruled on 2026-08-12 that it is
non-normative rather than backlog. So these seven functions are OPERATOR
FEATURES, not conformance obligations, and their absence is not an RFC gap. The
function numbers themselves are a local convention the RFC states as "could
assign", so each stays configuration rather than a constant.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/meta/filter-community.md` - the meta key the ingress pass reads
  → Constraint: [fill at design time]

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc8195.md` - the function table and the LOCAL_PREF pitfalls
  → Constraint: enrolment is `non-normative`, so nothing here is a conformance gap
- [ ] `rfc/short/rfc4264.md` - BGP Wedgies, which Section 4.3.3 points at
  → Constraint: [fill at design time]

**Key insights:** (minimal context to resume after compaction)
- Function 3 is the only RFC 8195 function Ze produces today
- The function NUMBER is configuration, never a constant: RFC 8195 says an AS "could assign" it

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/plugins/filter_community/relation.go` - `relationParameterFor` maps a resolved peer role ("customer", "peer", "provider") onto the RFC 8195 Section 3.2 parameters 2, 3 and 4, and returns 0 for an unresolved role, for "rs" and for "rs-client". It fails closed: 0 means write nothing. This is the whole of Ze's RFC 8195 production
- [ ] `internal/component/bgp/plugins/filter_community/filter.go` - `applyRelationTag` writes that one large community on the ingress pass, after the scrub
- [ ] `internal/component/bgp/plugins/filter_community/config.go` - `relationFunctionNumber` is the only function-number leaf, defaulting to 3. No leaf holds a country code, a region code, or a local-preference class
- [ ] `rfc/short/rfc8195.md` - Meta declares enrolment `non-normative`, Support `-`

**Behavior to preserve:**
- The Function 3 relation tag and its fail-closed 0 return
- The function number staying configuration rather than a constant

**Behavior to change:**
- [fill at design time]

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator configures large-community functions under a peer's community filter in the YANG config tree, and an EBGP UPDATE reaches the `filter_community` plugin's ingress pass carrying large communities whose Global Administrator is the local ASN.
- Format at entry: RFC 8092 large communities, three 4-octet fields, read as ASN:Function:Parameter.

### Transformation Path
1. Ingress: the community filter reads the route's large communities and the peer facts
2. [Stage 2: fill at design time]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ BGP plugin | YANG leaves under the community filter | No |

### Integration Points
- `applyRelationTag` (`filter.go`) - the existing RFC 8195 writer this joins

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
| A-1 | Functions 5 and 7 need a per-peer country that Ze does not hold today | `config.go` carries no country leaf | The design gains a data source it does not have | Read the peer config schema | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Local-preference classes 8 to 12 create a BGP Wedgie (RFC 4264) | Route oscillation in a functional test | State the class ordering and refuse a route carrying two classes |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Routes are exported, prepended, or preferred against operator intent |
| How is it reverted? | Single commit revert; the functions are opt-in configuration |
| Who else touches this path? | `plan/immediate/spec-bcp194-1-communities.md` (Functions 1 to 4 and 6) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| route carries own-GA Function 8 on egress | → | local-preference class applier | [test name, fill at design time] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | [fill at design time] | [fill at design time] |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | [fill at design time] | [fill at design time] | [test name] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [fill at design time] | `internal/component/bgp/plugins/filter_community/` | [description] | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| ISO 3166-1 numeric country code | 1-999 | [value] | 0 | 1000 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| [fill at design time] | `test/plugin/*.ci` | [what the user expects to happen] | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| [fill at design time] | `test/interop/scenarios/` | [FRR/BIRD/GoBGP] | [protocol behavior validated] | |

## Files to Modify
- `internal/component/bgp/plugins/filter_community/` - the functions and their configuration

## Files to Create
- [fill at design time]

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | `internal/component/bgp/plugins/filter_community/yang/` |
| YANG validation constraints | | country and region codes take `range` |
| YANG custom validators | | |
| CLI commands/flags | | |
| CLI grammar (keyword before value) | | |
| Editor autocomplete | | |
| Functional test for new RPC/API | | |
| Pipe completeness | | |
| Env var registration | | |
| Doctor check for runtime dependencies | | |
| Prometheus counters/metrics | | |
| BGP family surface (new SAFI / capability / attribute) | | N-A: no new family, capability or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | | `docs/features.md` |
| 2 | Config syntax changed? | | `docs/guide/configuration.md` |
| 3 | CLI command added/changed? | | |
| 4 | API/RPC added/changed? | | |
| 5 | Plugin added/changed? | | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | | |
| 7 | Wire format changed? | | |
| 8 | Plugin SDK/protocol changed? | | |
| 9 | RFC behavior implemented, changed, or newly proven? | | `rfc/short/rfc8195.md` |
| 10 | Test infrastructure changed? | | |
| 11 | Affects daemon comparison? | | `docs/comparison.md` |
| 12 | Internal architecture changed? | | |
| 13 | Route metadata keys added/changed? | | `docs/architecture/meta/filter-community.md` |
| 14 | Prometheus counters added/changed? | | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | | |
| 16 | Any changed source file referenced by existing doc source anchors? | | DERIVED: `./le spec citation anchors spec plan/immediate/spec-bcp194-6-remaining-rfc8195-functions.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- register the configuration entry points, write failing wiring tests
   - Tests: [wiring test names]
   - Files: [config leaves, filter hook]
   - Verify: the entry point exists and is reachable; the wiring test fails because the feature is a stub
2. **Phase: [name]** -- [fill at design time]

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | A route carrying two local-preference classes has one stated outcome |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| [fill at design time] | [command] |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | A neighbor must not use Function 5 or 7 to suppress or prepend a route it does not originate |

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

## RFC Documentation (Scope: protocol)

RFC 8195 states no RFC 2119 keyword, so no `// RFC NNNN Section X.Y:` comment
records an obligation here. A comment naming the section that describes a
function is still owed, as `relation.go` already does for Section 3.2.

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
