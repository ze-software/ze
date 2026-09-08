# Spec: test-peer-open-mirrors-five-more-sender-facts

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`spec-test-peer-open-inherits-zes-identity` states one property: **every fact
ze-peer's OPEN asserts about ze-peer is resolved once from the test's own
configuration, and no octet of that fact is inherited from ze's OPEN.** It
implemented that property for the AS, the BGP Identifier, the Role (capability
9), the ADD-PATH directions (69) and the FQDN (73). Five sender-describing
values are still mirrored, so the property does not yet hold, and this spec is
the rest of it. This spec's provenance is that one, and it exists because an
in-scope item a spec does not do becomes a spec of its own rather than a note
(`ai/rules/planning.md`).

The five are Graceful Restart (64), Long-Lived Graceful Restart (70 and 71),
software version (75), PATHS-LIMIT (76), and the two-octet Hold Time of the OPEN
body, which no `option=` can set today.

Graceful Restart is the one verified at its producer. `Negotiate`
(`internal/core/bgp/capability/negotiated.go`) stores the REMOTE speaker's
Graceful Restart capability whole, and `runPeer`
(`internal/component/bgp/reactor/peer_run.go`) feeds
`neg.GracefulRestart.RestartTime` into `startEORTimer`. A mirrored capability
therefore makes ze time the PEER's restart by ze's OWN configured restart time,
and every graceful-restart `.ci` in the tree is measuring ze against itself.

The other four are NOT producer-verified, and the first work this spec owes is
that verification: for each one, read the function that consumes the received
capability and state what a mirrored value makes ze believe. Software version
(75) is the loudest by inspection, because ze-peer reports ze's own build as its
own, but "loudest" is a reading of the code rather than a reading of its
consumer.

The reason none of the five was done in the first spec is recorded there and is
not a scope judgement to repeat here: no acceptance criterion reached them, and
changing the restart time changes what `startEORTimer` waits for in every
graceful-restart test. That blast radius is what this spec has to size and
absorb.

The class is recorded in
`plan/journal/mirrored-field-asserts-the-wrong-sender.md`, which stays: the
journal holds the pattern, this spec holds the work.

**Every section below this line is unwritten.** This spec is a skeleton, raised
so the work has a home and a name rather than a row in a report. The research
phase fills them, and nothing here is a design decision yet.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/ci-format.md` - the `.ci` format reference, and
  the "Capability Control" section that states which values ze-peer owns
- [ ] `ai/rules/principles.md` - the zero-value and single-declaration directives

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4724.md` - Graceful Restart, the Restart Time field, and what
  a receiver does with it
- [ ] `rfc/short/rfc9494.md` - Long-Lived Graceful Restart and its stale time
- [ ] `rfc/short/rfc4271.md` - the OPEN Hold Time field and its negotiation

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE the design is written)
- [ ] `internal/test/peer/open.go` - `ownedCapabilities` resolves codes 9, 65, 69
  and 73 and mirrors every other capability; `encodeOpen` copies ze's Hold Time
- [ ] `internal/core/bgp/capability/negotiated.go` - `Negotiate` stores the remote
  Graceful Restart capability whole
- [ ] `internal/component/bgp/reactor/peer_run.go` - `runPeer` feeds
  `neg.GracefulRestart.RestartTime` into `startEORTimer`
- [ ] the consumers of capabilities 70, 71, 75 and 76, which this spec has NOT
  identified yet

**Behavior to preserve:**
- To be written.

**Behavior to change:**
- To be written.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- To be written.

### Transformation Path
1. To be written.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| To be written | To be written | No |

### Integration Points
- To be written.

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
| A-1 | Four of the five mirrored values have a consumer that acts on them, as Graceful Restart does | only Graceful Restart is producer-verified; the other four are read from the capability list by inspection | some of the five are inert, and reconciling them changes nothing an operator or a test can observe | read the consumer of each received capability and name what it decides | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Reconciling the Graceful Restart restart time changes what `startEORTimer` waits for in every graceful-restart `.ci` | those files change verdict | read each one before landing, and size the change against them rather than against the capability |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every functional test that drives a BGP session. Nothing an operator can reach; the shipped daemon is not touched |
| How is it reverted? | A single commit revert. The harness has no state and no on-disk format |
| Who else touches this path? | Any session writing a `.ci`. `plan/journal/mirrored-field-asserts-the-wrong-sender.md` carries the class |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| a `.ci` peer block against a ze that offers Graceful Restart | → | the OPEN builder's restart-time resolution | `TestPeerOpenGracefulRestartIsTheHarnessOwn` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Ze's OPEN carries a Graceful Restart capability with a restart time | The OPEN ze-peer sends carries ze-peer's OWN restart time, and `startEORTimer` waits on that value rather than on ze's |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPeerOpenGracefulRestartIsTheHarnessOwn` | `internal/test/peer/open_test.go` | AC-1 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Graceful Restart restart time | 0-4095 | 4095 | N/A | 4096 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| To be written | `test/plugin/*.ci` | To be written | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | N-A | N-A | Ze's wire behavior does not change. The change is confined to `internal/test/peer/`, which no shipped binary links | |

## Files to Modify
- `internal/test/peer/open.go` - `ownedCapabilities` gains the remaining
  sender-describing codes, and `encodeOpen` stops copying ze's Hold Time
- `docs/architecture/testing/ci-format.md` - the "Capability Control" section
  lists which values ze-peer owns

## Files to Create
- To be written.

## Implementation Steps

1. **Phase: verify each consumer** -- read the function that acts on each received
   capability and record what a mirrored value makes ze believe
2. **Phase: reconcile what the verification found**, one capability at a time

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every one of the five is either reconciled or recorded as inert, with the consumer named |
| Correctness | No fact the OPEN asserts is written in one place and inherited in another |
| Rule: `ai/rules/evidence.md` | Each verdict cites the consuming function, not the capability's presence |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| To be written | To be written |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | N-A: a test harness with no privileged input |

### Failure Routing

| Failure | Route To |
|---------|----------|
| A gating test that passed before now fails | It was passing on the mirrored value. Read the test, then decide |

## Design Insights

- To be written.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| To be written | To be written | To be written |

## Known Limitations
- To be written.

## RFC Documentation (Scope: protocol)

The harness is not protocol-implementing code and adds no `RFC requirement:` tag.

| Fact | Citation the code carries |
|------|---------------------------|
| The restart time | RFC 4724 Section 3, on the Restart Time field of the sender |

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
- [ ] Every AC demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only
