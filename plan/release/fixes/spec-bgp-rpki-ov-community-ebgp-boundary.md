# Spec: bgp-rpki-ov-community-ebgp-boundary

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P1 |
| Release reason | Security defect in an implemented capability, the BGP relay: an outside AS can inject origin validation state into ours. RFC 8097 Section 2 makes the EBGP drop a MUST. |
| RFC gaps | RFC8097-2-4 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-30 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze relays a received RPKI origin validation state extended community (type
0x43, sub-type 0x00, RFC 8097) from an EBGP peer unchanged to every IBGP peer.
Ze treats every extended community as eight opaque octets
(`internal/core/bgp/attribute/community.go`, `ExtendedCommunity` and
`ParseExtendedCommunities`): nothing reads type 0x43, and no path strips
non-transitive extended communities at an AS boundary. An outside AS can
therefore mark its routes Valid inside our AS for any RFC 8097 router that
derives the validation state from the community.

RFC 8097 Section 2 (`rfc/full/rfc8097.txt`): "By default, implementations MUST
drop the origin validation state extended community if received from an
External BGP (EBGP) peer, without processing it further."

The same paragraph adds: "Similarly, by default, an implementation SHOULD NOT
send the community to EBGP peers.  However, it SHOULD be possible to configure
an implementation to send or accept the community when warranted."

Goal: by default, a type 0x43 community received from an EBGP peer is dropped
on receipt, so it reaches no Adj-RIB-In, no policy and no IBGP peer. The
RFC-permitted configured exception (adjacent ASes under one administration)
is decided at DESIGN against FRR's and BIRD's source.

This spec owns the defect only. The other RFC 8097 rows (RFC8097-2-1, 2-2,
2-3 and 2-5: building and reading the community) are feature gaps recorded in
`rfc/short/rfc8097.md` and follow the owner's release timing.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `ai/doors/system.md` and the BGP door - the relay and attribute boundaries
  → Constraint: filled at RESEARCH

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc8097.md` - RFC8097-2-4 is the row this spec turns from gap to tested
  → Constraint: "By default, implementations MUST drop the origin validation state extended community if received from an External BGP (EBGP) peer, without processing it further." (RFC 8097 Section 2)
- [ ] `rfc/short/rfc4360.md` - extended community type octet and the Transitive bit
  → Constraint: filled at RESEARCH

**Key insights:** (minimal context to resume after compaction)
- filled at RESEARCH

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/core/bgp/attribute/community.go` - extended communities parsed and carried as opaque eight-octet values; no type 0x43 handling
- [ ] the receive and forward path of the reactor - filled at RESEARCH

**Behavior to preserve:** (unless the user explicitly said to change it)
- Every other extended community, transitive or not, is relayed as today unless an RFC says otherwise
- Communities received from IBGP peers are untouched

**Behavior to change:** (only what the user asked for)
- A type 0x43 community received from an EBGP peer is dropped by default

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- EBGP UPDATE carrying EXTENDED_COMMUNITIES (attribute type 16) with a 0x43/0x00 member

### Transformation Path
1. filled at RESEARCH

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire ↔ Adj-RIB-In | filled at RESEARCH | No |

### Integration Points
- filled at RESEARCH

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
| A-1 | No code path reads or strips type 0x43 today | Independent review of the BGP/VPN RFC batch; no 0x43 hit under `internal/core/bgp` or `internal/component/bgp` | The defect is narrower than stated | grep and a relay test at RESEARCH | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The drop rewrites the attribute on a zero-copy forward path | A forwarded UPDATE loses its shared wire bytes | Decide at DESIGN where the drop happens, on receipt, before the route is stored |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A 0x43 community from an EBGP peer keeps reaching IBGP peers (the defect), or a wrong drop removes another community |
| How is it reverted? | single commit revert |
| Who else touches this path? | filled at RESEARCH |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| filled at DESIGN | → | | |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | EBGP UPDATE carrying a 0x43/0x00 extended community, default config | The community is absent from the Adj-RIB-In and from the UPDATE sent to an IBGP peer; every other community of the route is relayed |
| AC-2 | IBGP UPDATE carrying the same community | Relayed unchanged |
| AC-3 | filled at DESIGN: the configured exception RFC 8097 Section 2 permits | filled at DESIGN |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs ze between an outside EBGP peer and an internal IBGP peer | wire -> receive -> Adj-RIB-In -> forward | filled at DESIGN |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| filled at DESIGN | | RFC8097-2-4 positive and negative | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | no numeric input | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| filled at DESIGN | `test/.../*.ci` | EBGP-received 0x43 community never reaches an IBGP peer | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| filled at DESIGN | `test/interop/scenarios/` | FRR or BIRD | the drop as seen by a peer | |

## Files to Modify
- filled at RESEARCH

## Files to Create
- filled at DESIGN

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | filled at DESIGN (the configured exception) |
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
| 6 | Has a user guide page? | | |
| 7 | Wire format changed? | | |
| 8 | Plugin SDK/protocol changed? | | |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc8097.md` (RFC8097-2-4 and the Support rows) |
| 10 | Test infrastructure changed? | | |
| 11 | Affects daemon comparison? | | |
| 12 | Internal architecture changed? | | |
| 13 | Route metadata keys added/changed? | | |
| 14 | Prometheus counters added/changed? | | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | | |
| 16 | Any changed source file referenced by existing doc source anchors? | | DERIVED: `./le spec citation anchors spec plan/release/fixes/spec-bgp-rpki-ov-community-ebgp-boundary.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- filled at DESIGN
2. **Phase: EBGP drop** -- filled at DESIGN

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | Only type 0x43 sub-type 0x00 is dropped, only from EBGP peers, before any processing |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| RFC8097-2-4 tested with both polarities | `./le rfc check` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | An EBGP peer cannot place an origin validation state into our AS by default |

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
| `scoped-attribute-relayed-past-its-boundary.md` | 2026-10-30 | EBGP-received type 0x43 origin validation state community relayed to IBGP peers |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- RFC8097-2-1, 2-2, 2-3 and 2-5 (building and reading the community) are feature gaps in `rfc/short/rfc8097.md`, not part of this defect fix

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
