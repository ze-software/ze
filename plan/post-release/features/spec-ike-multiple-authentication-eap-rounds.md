# Spec: ike-multiple-authentication-eap-rounds

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P3 |
| Release reason | - (RFC 4739 is reached only by the vpn-concentrator profile, which is not a release profile) |
| RFC gaps | - |
| Depends | spec-ike-multiple-authentication |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-30 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`plan/post-release/features/spec-ike-multiple-authentication.md` gives a
site-to-site peer one additional authentication round per direction, by
certificate or pre-shared secret, and refuses a second round on a peer whose
first round is EAP. Three parts of RFC 4739 stay unbuilt after it:

| Part | RFC 4739 text | What is missing |
|------|---------------|-----------------|
| EAP rounds | §2.2 and §2.3 (Examples 1 and 2): the initiator "sends a new IDi payload but no AUTH payload (message 11), indicating that EAP will be used" | An EAP round after a certificate or PSK round, or a second EAP round after the first, in both roles |
| Remote-access peers | §1.1 usage scenario (NAP plus third-party AAA) | `vpn ipsec remote-access` peers are EAP-only today, so they get no second round |
| More than two rounds | §2.1: either peer "can announce that it wishes to have a second authentication by including an ANOTHER_AUTH_FOLLOWS notification in any IKE_AUTH message that contains an AUTH payload" | Ze configures at most one additional round per direction |

None of these is a gated MUST of `rfc/short/rfc4739.md`: both gated rows are
closed by the parent spec. This spec is optional feature work under the RFC,
and Thomas decides whether it runs.

## Required Reading

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4739.md`, `rfc/full/rfc4739.txt` §2.1 to §2.3 - EAP round message shapes
  → Constraint: to be written at research

**Key insights:** (minimal context to resume after compaction)
- The round model, accessors and state come from `plan/post-release/features/spec-ike-multiple-authentication.md`.
- strongSwan 6.1.0 `src/libcharon/sa/ikev2/tasks/ike_auth.c` chains an EAP round after a pubkey round; read it at research.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/ike/engine/responder_eap.go` - [to be written at research]

**Behavior to preserve:** (unless the user explicitly said to change it)
- [to be written at research]

**Behavior to change:** (only what the user asked for)
- [to be written at research]

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [to be written at research]

### Transformation Path
1. [to be written at research]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze ↔ IKE peer | [IKE_AUTH EAP rounds] | No |

### Integration Points
- [to be written at research]

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
| What breaks if this is wrong? | [to be written at design] |
| How is it reverted? | [to be written at design] |
| Who else touches this path? | [to be written at design] |

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
| 1 | [to be written at design] | [path] | [test name] |

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
| `[name]` | `test/interop-ipsec/scenarios/` | strongSwan | [protocol behavior validated] | |

## Files to Modify
- `internal/component/ike/engine/...` - [feature changes]

## Files to Create
- `internal/component/ike/engine/...` - [new feature file]
- `test/.../*.ci` - [functional test for end-user behavior]

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | [to be answered at research] |
| YANG validation constraints | | [to be answered at research] |
| YANG custom validators | | [to be answered at research] |
| CLI commands/flags | | [to be answered at research] |
| CLI grammar (keyword before value) | | [to be answered at research] |
| Editor autocomplete | | [to be answered at research] |
| Functional test for new RPC/API | | [to be answered at research] |
| Pipe completeness | | [to be answered at research] |
| Env var registration | | [to be answered at research] |
| Doctor check for runtime dependencies | | [to be answered at research] |
| Prometheus counters/metrics | | [to be answered at research] |
| BGP family surface (new SAFI / capability / attribute) | | [to be answered at research] |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | | [to be answered at research] |
| 2 | Config syntax changed? | | [to be answered at research] |
| 3 | CLI command added/changed? | | [to be answered at research] |
| 4 | API/RPC added/changed? | | [to be answered at research] |
| 5 | Plugin added/changed? | | [to be answered at research] |
| 6 | Has a user guide page? | | [to be answered at research] |
| 7 | Wire format changed? | | [to be answered at research] |
| 8 | Plugin SDK/protocol changed? | | [to be answered at research] |
| 9 | RFC behavior implemented, changed, or newly proven? | | [to be answered at research] |
| 10 | Test infrastructure changed? | | [to be answered at research] |
| 11 | Affects daemon comparison? | | [to be answered at research] |
| 12 | Internal architecture changed? | | [to be answered at research] |
| 13 | Route metadata keys added/changed? | | [to be answered at research] |
| 14 | Prometheus counters added/changed? | | [to be answered at research] |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | | [to be answered at research] |
| 16 | Any changed source file referenced by existing doc source anchors? | | [derived at research with `./le spec citation anchors`] |
| 17 | Existing docs show config/CLI/API examples for this area? | | [to be answered at research] |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- register entry points, write failing wiring tests
   - Tests: [wiring test names from the Wiring Test table]
   - Files: [to be written at design]
   - Verify: the entry point exists and is reachable. The wiring test fails because the feature is a stub
2. **Phase: [name]** -- [what to implement]
   - Tests: [test names from the TDD Plan]
   - Files: [files from Files to Modify]
   - Verify: tests fail → implement → tests pass → wiring test progresses

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | [to be written at design] |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| [concrete thing that must exist] | [grep/ls/test command] |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | [to be written at design] |

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

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- [to be written at design]

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
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches, and `./le verify changed` over the closing commits (`/ze-close` step 6f). The full verification is Thomas's release check: no agent runs it, and one that is owed is handed to him as its exact command. A red there gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
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
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
