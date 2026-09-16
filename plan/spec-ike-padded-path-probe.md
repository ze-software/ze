# Spec: ike-padded-path-probe

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`show mtu` (`docs/architecture/diagnostics/path-mtu.md`, closed as spec-path-mtu-diagnostic) measures a path with ICMP echo, and prints
the caveat that the figure is optimistic: a path can treat ICMP, UDP/4500 and
ESP differently, and the number that matters for a tunnel is the one the ESP
traffic actually meets. The VyOS tool it was ported from prints the same caveat
and cannot do better, because it is not the IKE daemon.

Ze is. An INFORMATIONAL exchange padded to a chosen size elicits an
authenticated reply from the peer over exactly the port, the encapsulation and
the path the ESP traffic rides. The reply coming back proves the request fit the
path. That removes the caveat for any peer with a live IKE SA rather than
printing it.

Scope: a padded INFORMATIONAL probe on the IKE engine, offered to the MTU module
as the prober used when an SA is up. ICMP stays the path for
`show mtu host <address>`, for the reference measurement, and for a tunnel that
is down, so this replaces no existing prober.

Open questions this spec answers before design:

- Which padding mechanism. RFC 7296 Section 3.14 pads the encrypted payload; a
  Notify payload carrying opaque data is another route. The choice is made
  against the RFC text and must not be one a peer can read as malformed.
- What a non-answer means. An ICMP probe separates "too big" from "lost" using
  the kernel error queue. A missing INFORMATIONAL reply has more causes, and
  RFC 4821 Section 7.6.4, which forbids moving the search bounds when loss may be
  congestion, binds here too.
- The retransmission interaction. IKE already retransmits an unanswered request,
  so a probe must not be retried by two mechanisms with two budgets.
- Whether a peer that is not Ze answers a padded INFORMATIONAL at all. That is an
  interop question against strongSwan and libreswan before it is a design
  question, and it decides whether this is general or Ze-to-Ze only.

Owner decision, 2026-09-11: ICMP ships first, and this is homed here so it is
scheduled rather than forgotten.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/ike/ipsec-7-ikev2-engine.md` - the exchange machinery this extends
  → Constraint: filled at RESEARCH

### RFC Summaries (Scope: protocol)
- [ ] `rfc/full/rfc7296.txt` - IKEv2, Section 3.14 and the INFORMATIONAL exchange
  → Constraint: filled at RESEARCH

**Key insights:** (minimal context to resume after compaction)
- filled at RESEARCH

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `path/to/file.go` - filled at RESEARCH

**Behavior to preserve:**
- filled at RESEARCH

**Behavior to change:**
- filled at RESEARCH

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- filled at RESEARCH

### Transformation Path
1. filled at RESEARCH

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| filled at RESEARCH | filled at RESEARCH | No |

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
| A-1 | A third-party IKE implementation answers a padded INFORMATIONAL request rather than rejecting it | unestablished; this is the first thing RESEARCH settles | the feature is Ze-to-Ze only, which for a CPE talking to a customer head-end is close to useless | an interop scenario against strongSwan and libreswan, run before any design work | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A padded INFORMATIONAL is read by a peer as malformed and drops the IKE SA, so a diagnostic takes a tunnel down | a peer logs a parse failure during the first interop run | the interop scenario runs before implementation, and the probe is abandoned rather than shipped if any peer reacts by tearing down |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A live IKE SA. This sends traffic on a production tunnel's control channel, which is a sharper blast radius than the ICMP prober has |
| How is it reverted? | Single commit revert; the MTU module falls back to the ICMP prober it already has |
| Who else touches this path? | `internal/component/mtu/cmd` (`docs/architecture/diagnostics/path-mtu.md`) is the consumer, `plan/immediate/spec-rfc4301-architecture-gaps.md` touches the same engine |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show mtu` with a live SA to the peer | → | the padded INFORMATIONAL prober | `TestShowMTUUsesIKEProbeWhenSAIsUp` |
| `show mtu` with the tunnel down | → | the ICMP prober, unchanged | `TestShowMTUFallsBackToICMPWhenSAIsDown` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A live IKE SA and a path clamped below the interface MTU | the measured figure comes from the padded exchange and equals the clamp |
| AC-2 | A peer that does not answer a padded INFORMATIONAL | the run falls back to ICMP and says which prober produced each figure |
| AC-3 | A padded probe that goes unanswered once | the search bounds are not moved on that single silence |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | runs `show mtu` on a CPE whose peer path filters ICMP but carries ESP | CLI → inventory → padded INFORMATIONAL → measured figure with no optimism caveat | `test-show-mtu-ike-probe` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPaddedInformationalReachesRequestedSize` | `internal/component/ike/engine/probe_test.go` | the emitted datagram is the size asked for | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| padded request size | 68-65535 | 65535 | 67 | 65536 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-show-mtu-ike-probe` | `test/plugin/*.ci` | a figure is produced over a path that filters ICMP | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ike-padded-probe-strongswan` | `test/interop/scenarios/` | strongSwan | a third-party responder answers a padded INFORMATIONAL and does not tear the SA down (A-1, R-1) | |

## Files to Modify
- `internal/component/ike/engine/` - the padded INFORMATIONAL exchange, files named at RESEARCH
- `internal/component/mtu/cmd/` - prober selection when an SA is up

## Files to Create
- `test/interop/scenarios/ike-padded-probe-strongswan/` - the interop scenario that runs before design

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no new command; `show mtu` already exists and this changes only which prober it uses |
| YANG validation constraints | N-A | no new leaf |
| YANG custom validators | N-A | no new leaf |
| CLI commands/flags | N-A | no new command |
| CLI grammar (keyword before value) | N-A | no new command |
| Editor autocomplete | N-A | no new leaf |
| Functional test for new RPC/API | Yes | `test/plugin/*.ci`, listed above |
| Pipe completeness | N-A | the payload shape is `show mtu`'s (`docs/architecture/diagnostics/path-mtu.md`, "The payload") and is unchanged apart from naming the prober |
| Env var registration | N-A | no leaf under `environment/` |
| Doctor check for runtime dependencies | N-A | no new runtime dependency: the IKE socket already exists and already has its checks |
| Prometheus counters/metrics | N-A | operator-invoked, no continuous state |
| BGP family surface (new SAFI / capability / attribute) | N-A | no BGP surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` |
| 2 | Config syntax changed? | N-A | no config leaf |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`, the optimism caveat changes |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md`, the payload names the prober |
| 5 | Plugin added/changed? | N-A | no plugin |
| 6 | Has a user guide page? | Yes | `docs/architecture/diagnostics/path-mtu.md` |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/` gains the padded exchange's shape |
| 8 | Plugin SDK/protocol changed? | N-A | no SDK surface |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc7296.md` for the padding mechanism chosen |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, a new interop scenario |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ike/ipsec-7-ikev2-engine.md` |
| 13 | Route metadata keys added/changed? | N-A | no route metadata |
| 14 | Prometheus counters added/changed? | N-A | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | filled at RESEARCH once the prober's registration shape is known |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: run `./le spec citation anchors spec plan/spec-ike-padded-path-probe.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/ipsec.md` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- prober selection reaches the engine
   - Tests: `TestShowMTUUsesIKEProbeWhenSAIsUp`, `TestShowMTUFallsBackToICMPWhenSAIsDown`
   - Files: filled at DESIGN
   - Verify: the selection happens and the padded prober is a stub
2. **Phase: interop first** -- settle A-1 before anything else is built
   - Tests: `ike-padded-probe-strongswan`
   - Files: the scenario directory
   - Verify: a third-party responder answers and does not tear the SA down. If it does not, this spec stops here and is closed as not viable

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | A probe never drives IKE state; an unanswered probe is not a liveness failure and must not feed the DPD path |
| Naming | The payload names which prober produced each figure, in the same key on every row |
| Data flow | The MTU module asks for a measurement and never constructs an IKE message itself |
| Rule: `ai/rules/rfc-compliance.md` | The padding mechanism is quoted from RFC 7296 above the code that builds it |

## Review Gate

<!-- Filled by /ze-review at implementation time, per .claude/rules/planning.md. -->

### Run 1
| Severity | Finding | File | Resolution |
|----------|---------|------|------------|

### Run 2
| Severity | Finding | File | Resolution |
|----------|---------|------|------------|

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| A third-party peer answers | the `ike-padded-probe-strongswan` scenario passes |
| No SA is torn down by a probe | the same scenario asserts the SA survives every probe size |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | A probe run must not exhaust the IKE message ID window or the retransmission queue |
| Authorization failing open | The probe is authenticated by the existing SA; it must not be reachable before authentication completes |

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
| Homed as its own spec rather than built into the MTU diagnostic | build both probers at once | `show mtu host <address>` and the reference measurement need ICMP regardless, so this removes no prober; it adds accuracy to one half. Owner decision, 2026-09-11 |

## Known Limitations

- Only measures a peer with a live IKE SA. A tunnel that is down is exactly the case an operator most wants measured, and ICMP remains the only answer there.

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
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

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
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
