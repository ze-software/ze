# Spec: ike-unbound-peer-inner-address

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/templates/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1. -->

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P0 |
| Release reason | Shipped IKEv2 tunnel-mode responder violates an RFC 3948 Section 5.1 MUST for every peer bound to no virtual-IP pool: two peers can hold Child SAs and kernel policies for one inner address |
| RFC gaps | RFC3948-5.1-1 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

RFC 3948 Section 5.1: "Because SGW will now see two possible SAs that lead to
10.1.2.3, it can become confused about where to send packets coming from Suzy's
server.  Implementors MUST devise ways of preventing this from occurring."

`spec-ike-virtual-ip-assignment` met this for a responder peer bound to a
virtual-IP pool (lease registry, TSi substitution, Ze-initiated rekey proposing
the lease only). An independent ze-rfc-audit judge (2026-10-26, verdict `weak`,
`rfc/audit/rfc3948/RFC3948-5.1-1.json`) found the MUST unmet for a tunnel-mode
peer bound to no pool, and the row is now `{partial}` in `rfc/short/rfc3948.md`.

The defect, at the producers:

| Producer | What it does |
|----------|--------------|
| `internal/component/ike/engine/ts_narrow.go::narrowSelectors` | reads an empty configured selector list as allow-everything ("An unconfigured peer allows everything"), so an unbound responder answers whatever inner TSi the peer proposes |
| `internal/component/ike/dataplane/policy_owner.go::claim` | refuses a second owner only for a byte-identical policy key; overlap is compared for transport mode only (RFC 3948 Section 5.2), and tunnel mode is skipped as "a deliberate configuration" |

Probe evidence (the judge's): two unbound peers both proposing TSi 10.1.2.3/32,
one with TSr 0.0.0.0/0 and one with TSr 192.0.2.10/32, were both answered, the
two policy keys differ, and both tunnel policies installed. Configured remote
selectors that overlap another peer's are not refused either.

Two fix options, to be decided at design with evidence:

| Option | Behavior | Evidence to gather |
|--------|----------|--------------------|
| A | Refuse a tunnel Child SA whose peer-proposed remote (inner) selector overlaps another peer's live remote selector (TS_UNACCEPTABLE), extending `claim`'s overlap check, or an engine-level registry, to the tunnel population that is peer-chosen rather than operator-configured | Whether an operator-configured overlap ordered by priority must stay legal, and how a strongSwan initiator answers the refusal |
| B | Narrow an unconfigured responder's TSi to the peer's observed address, as strongSwan's `%dynamic` default does | strongSwan source quote for `%dynamic` resolution on the responder; whether a NATed peer's observed address breaks site-to-site use |

Goal: no two peers can hold a tunnel Child SA (or a kernel policy) whose inner
remote selectors overlap unless the operator configured both, and
`RFC3948-5.1-1` loses its `{partial}` marker with both polarities proven over the
unbound path.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` and the IKE door named in `ai/doors/README.md`
- [ ] `docs/architecture/ike/ipsec-14-responder.md` - virtual-IP narrowing and the responder selector path
- [ ] `docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md` - "selectors of a peer bound to no pool pass through unchanged"
- [ ] `docs/guide/ipsec.md` - virtual IP pool section, which now discloses this gap
  → Constraint: [to fill at design]

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc3948.md` - RFC3948-5.1-1 (`{partial}`) and Section 5.2 transport overlap
- [ ] `rfc/full/rfc3948.txt` Sections 5.1, 5.2
- [ ] `rfc/full/rfc7296.txt` Sections 2.9, 2.19 (narrowing may only answer a subset of the proposal)
  → Constraint: [to fill at design]

**Key insights:** (minimal context to resume after compaction)
- [insight from docs]

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/ike/engine/ts_narrow.go` - `narrowSelectors`, `narrowChildSelectors`
- [ ] `internal/component/ike/dataplane/policy_owner.go` - `claim`, `selectorsOverlap`
- [ ] `internal/component/ike/engine/cp.go` - `substituteVirtualIP` (the pool-bound path that already prevents it)

**Behavior to preserve:** (unless the user explicitly said to change it)
- The pool-bound path and its tests (`TestTwoPeersSameRequestedAddressDistinctLeases`, `TestNoChildSAWithoutLeaseOnBoundPeer`, `TestRFC3948ZeInitiatedRekeyNamesOnlyTheLease`)
- Transport-mode overlap refusal (RFC 3948 Section 5.2)

**Behavior to change:** (only what the user asked for)
- [list, or "None - preserve all existing behavior"]

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [Where data enters: wire bytes, API command, config, plugin message]
- [Format at entry]

### Transformation Path
1. [Stage 1]
2. [Stage 2]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | [JSON format, command syntax] | No |

### Integration Points
- [Existing function/type this connects to] - [how it integrates]

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | [what this design assumes] | [where the assumption comes from] | [impact on design] | [test/grep/user confirmation] | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | [what goes wrong] | [how we notice it] | [what we do about it] |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | [live sessions dropped / config rejected / nothing user-visible] |
| How is it reverted? | [single commit revert / needs config migration] |
| Who else touches this path? | [other specs working the same files] |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| [config/CLI/event that triggers it] | → | [function that actually runs] | [test name proving the chain] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | [what triggers the behavior] | [observable outcome] |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | [user operation] | [path] | [test name] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestXxx` | `internal/.../xxx_test.go` | [description] | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| [field] | [min-max] | [value] | [value or N/A] | [value or N/A] |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-xxx` | `test/.../*.ci` | [what the user expects to happen] | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| [named scenario] | `test/interop/scenarios/` | strongSwan | [two unbound clients claiming one inner address] | |

## Files to Modify
- `internal/component/ike/engine/ts_narrow.go` - [option B]
- `internal/component/ike/dataplane/policy_owner.go` - [option A]
- `rfc/short/rfc3948.md` - drop `{partial}` on RFC3948-5.1-1 once both polarities cover the unbound path

## Files to Create
- `internal/...` - [new test file]

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | |
| YANG validation constraints | | |
| YANG custom validators | | |
| CLI commands/flags | | |
| CLI grammar (keyword before value) | | |
| Editor autocomplete | | |
| Functional test for new RPC/API | | |
| Pipe completeness | | |
| Env var registration | | |
| Doctor check for runtime dependencies | | |
| Prometheus counters/metrics | | |
| BGP family surface (new SAFI / capability / attribute) | | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | | |
| 2 | Config syntax changed? | | |
| 3 | CLI command added/changed? | | |
| 4 | API/RPC added/changed? | | |
| 5 | Plugin added/changed? | | |
| 6 | Has a user guide page? | | `docs/guide/ipsec.md` (remove the unbound-peer disclosure) |
| 7 | Wire format changed? | | |
| 8 | Plugin SDK/protocol changed? | | |
| 9 | RFC behavior implemented, changed, or newly proven? | | `rfc/short/rfc3948.md`, `plan/learned/035-ike-virtual-ip-assignment.md` |
| 10 | Test infrastructure changed? | | |
| 11 | Affects daemon comparison? | | |
| 12 | Internal architecture changed? | | `docs/architecture/ike/ipsec-14-responder.md`, `docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md` |
| 13 | Route metadata keys added/changed? | | |
| 14 | Prometheus counters added/changed? | | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | | |
| 16 | Any changed source file referenced by existing doc source anchors? | | DERIVED: `./le spec citation anchors spec plan/release/fixes/spec-ike-unbound-peer-inner-address.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- [to fill at design]
2. **Phase: [name]** -- [what to implement]

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | [feature-specific] |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| [concrete thing that must exist] | [grep/ls/test command] |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Peer-proposed traffic selectors are untrusted input that can capture another peer's inner address |

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

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `guard-added-to-one-half-of-a-pair.md` | 2026-10-26 | IKE tunnel inner address: unbound peer keeps any proposed TSi, `claim` checks tunnel overlap never |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- [What was deliberately not done and why]

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

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
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches, and `./le verify changed` over the closing commits (`/ze-close` step 6f). The full verification is Thomas's release check: no agent runs it, and one that is owed is handed to him as its exact command. A red there gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
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
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-ike-unbound-peer-inner-address.md` only, in the same `./le commit create` script (commit A preserves the spec in history)
