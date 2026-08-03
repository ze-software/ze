# Spec: fixit-l2tp-sccrq-tunnel-id-zero

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `-` |
| Updated | 2026-08-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**An SCCRQ carrying Assigned Tunnel ID 0 is dropped in silence. `RFC2661-24.10-1`
is a gated MUST that requires a StopCCN reply, its published coverage claims the
Reactor sends one, and both of its tagged tests prove the OTHER half of the
requirement.**

Found on 2026-08-02 while closing `plan/learned/1313-rfcgate-1b-rfc7296-pilot.md`.

**The way to fix this is the owner's decision, not this spec's.** The question is
stated in full below and must be answered before any code changes.
`ai/rules/rfc-compliance.md` reserves it: every route on the table either lowers
what Ze owes or adds an unauthenticated reply surface.

### What the code does

`parseSCCRQ` (`internal/component/l2tp/tunnel_fsm.go:321`) rejects a zero Assigned
Tunnel ID with an error, `"l2tp: Assigned Tunnel ID AVP must be non-zero"`.

`Reactor.handlePacket` (`internal/component/l2tp/reactor.go:353`) calls it for every
packet arriving with `TunnelID == 0`, and on error logs at Debug and returns:

- `:355` logs `"l2tp: TunnelID=0 packet with malformed body dropped"`.
- `:357` returns.

Nothing is sent. The peer receives silence.

`reactor.go:671` carries the comment `"sccrq.AssignedTunnelID is guaranteed non-zero
by parseSCCRQ"`, which is true and is exactly how the drop became invisible: the
validation moved to the parser, and the parser's only failure mode is a dropped
packet.

### What the requirement says

`rfc/short/rfc2661.md:479` states the obligation:

- `[RFC2661-24.10-1] [MUST] Assigned Tunnel ID of 0 in SCCRQ/SCCRP is a protocol
  error; reject with StopCCN (§24.10)`

The requirement names BOTH messages, SCCRQ and SCCRP.

`rfc/short/rfc2661.md:433` states the implemented coverage as `"Reactor rejects with
StopCCN Result Code 2"`. The Reactor is precisely the path that sends nothing.

### What the evidence actually proves

`rfc2661` is ENROLLED (`rfc/enrolled.txt:162`), so this MUST is gated.

Both tags sit on the initiator side, over SCCRP:

- `internal/component/l2tp/tunnel_initiator_test.go:107` carries the negative polarity.
- `internal/component/l2tp/tunnel_initiator_test.go:135` carries the positive polarity.

`ai/RFC-REQUIREMENTS.md:1361` records both as `unit/verify`.

So the requirement is fully gated in both polarities, and both polarities test the
SCCRP half. The SCCRQ half has no test at all. The ledger cannot see this, because
it counts polarities per requirement id and not per message type.

This is a public claim standing ahead of its evidence, which is the failure class
the pilot existed to find. It is not a false tag: the two tests are honest about
what they drive. The gap is that one requirement id covers two message types and
only one of them is met.

### The owner question (BLOCKING, answer before any code lands)

**Both routes are legitimate and this spec may not choose between them.**

Route A, send StopCCN on the SCCRQ path. This is full compliance with §24.10 as the
summary states it. It makes ze answer a datagram that has not been authenticated
and whose source address is trivially spoofed, so ze becomes a reflector: one
forged SCCRQ produces one StopCCN to a victim of the attacker's choosing. The
control channel has no return-routability check at this point in the exchange.

Route B, keep the silent drop and narrow the published claim to the SCCRP half.
RFC 2661 Section 4.4.3 permits, rather than requires, a reply here, and declining to
answer an unauthenticated datagram is the conservative reading. This route LOWERS
what Ze publicly owes, so `ai/rules/rfc-compliance.md` forbids a subagent from
choosing it and requires the owner's explicit answer.

Route C, a bounded middle. Send StopCCN but rate-limit it per source, or only to a
source that has an existing tunnel. This is more code and needs its own threat
model.

**What full compliance costs, stated so the answer is informed.** Route A needs a
StopCCN emission path reachable before any tunnel state exists, because the current
drop happens before a tunnel entry is allocated, and that ordering is deliberate:
the comment at `reactor.go:349` says the body is parsed before `tunnelsMu` is taken
so a malformed body allocates nothing. Emitting StopCCN needs a peer identity and a
control-channel sequence, so it either allocates the state the drop avoids or sends
a synthetic reply outside the tunnel machinery. It also needs a tagged test on the
SCCRQ path in both polarities, and the reproducer already exists in
`test/draft/l2tp/`.

**Do not write a `{gap}`, a `partial`, or a `{not-applicable}` for this.** Writing
one IS the decision, and `check_coverage_ratchet` plus `check_retired_requirements`
correctly refuse the cheaper escapes. The requirement stays as it is until the
owner answers.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/rfc-compliance.md` - who may decide anything short of full compliance
  → Constraint: a subagent may not classify this row away. Quote the requirement, name the producer, state the cost, and ask which way to fix it
- [ ] `ai/rules/fail-closed-guards.md` - a guard that neither denies nor speaks
  → Constraint: the current drop denies correctly but says nothing to the peer, which is why the behavior was invisible
- [ ] `ai/rules/testing.md` - RFC-tagged tests and the draft incubator
  → Constraint: a tagged test IS the requirement. Write the SCCRQ test in `test/draft/l2tp/` first, and promote it when green

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2661.md` - the obligation at `:479`, the coverage claim at `:433`
  → Constraint: `RFC2661-24.10-1` names SCCRQ AND SCCRP. Ze meets the SCCRP half only
  → Decision: Section 4.4.3 permits rather than requires the reply, which is what makes route B arguable rather than plainly non-conformant

**Key insights:** (minimal context to resume after compaction)
- The requirement is gated and tagged in both polarities, and both tags test SCCRP.
- The SCCRQ path silently drops at `reactor.go:353-357`.
- The published coverage line names the Reactor, which is the path that sends nothing.
- The fix direction is the OWNER's call. Do not narrow the claim and do not write a gap.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/l2tp/tunnel_fsm.go` - `parseSCCRQ` at `:321`, the zero-ID rejection in the `AVPAssignedTunnelID` arm
- [ ] `internal/component/l2tp/reactor.go` - `handlePacket` parses at `:353`, logs at `:355`, returns at `:357`; the guarantee comment at `:671`
- [ ] `internal/component/l2tp/tunnel_initiator_test.go` - the two tags at `:107` and `:135`, both over SCCRP

**Behavior to preserve:** (unless the user explicitly said to change it)
- `parseSCCRQ` keeps rejecting a zero Assigned Tunnel ID. The validation is correct and only the response to it is in question.
- The parse-before-lock ordering stays. Parsing before `tunnelsMu` is taken is what stops a malformed body allocating a tunnel entry or consuming a local TID.
- The SCCRP half keeps working, and its two tagged tests keep their ids and polarities.
- Every other `TunnelID == 0` drop reason keeps its current behavior unless the owner's answer covers it too.

**Behavior to change:** (only what the user asked for)
- Undecided, and deliberately so. The owner's answer selects route A, B or C above. No code changes until then.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A UDP datagram arrives on the L2TP control port with `TunnelID == 0`.
- Format at entry: an L2TP control header followed by an AVP body, unauthenticated and source-spoofable.

### Transformation Path
1. `Reactor.handlePacket` reads the header and selects the `TunnelID == 0` branch.
2. `parseSCCRQ` walks the AVP body and validates the Assigned Tunnel ID.
3. On a zero ID the parser returns an error, before `tunnelsMu` is taken.
4. `handlePacket` logs at Debug and returns. Nothing is transmitted.
5. The peer waits, retransmits, and eventually times out.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Peer ↔ Reactor | unauthenticated UDP control datagram | No |
| Reactor ↔ tunnel table | `tunnelsMu`, deliberately taken AFTER the parse | No |
| Ze ↔ published claim | `rfc/short/rfc2661.md` and `ai/RFC-REQUIREMENTS.md` | No |

### Integration Points
- `internal/component/l2tp/reactor.go` - the drop site and any future emission point.
- `teardownStopCCN` and the existing StopCCN paths, which today all require a tunnel.
- `test/draft/l2tp/` - the incubator already holding two RFC 2661 control-shape drafts.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugin-self-containment.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No other code path answers a zero-ID SCCRQ with StopCCN | read, `reactor.go` `handlePacket` returns at `:357` | The requirement is already met and only the test is missing, which makes this a test-only spec | grep every StopCCN emission site and check reachability with no tunnel | unvalidated |
| A-2 | Both `RFC2661-24.10-1` tags really do drive SCCRP and never SCCRQ | read, `tunnel_initiator_test.go:107` and `:135` | The SCCRQ half is already proven and the finding dissolves | read both test bodies in full | unvalidated |
| A-3 | Sending StopCCN before any tunnel exists needs new machinery | read, existing StopCCN paths all hang off a tunnel | Route A is far cheaper than stated, which should change the owner's answer | trace `teardownStopCCN` and check whether a tunnel-free emission already exists | unvalidated |
| A-4 | The reflection concern is real, meaning no return-routability check precedes this point | read, the control exchange order | Route A carries no amplification risk and is the obvious answer | confirm no cookie or challenge precedes SCCRQ handling | unvalidated |
| A-5 | A StopCCN reply is not larger than the SCCRQ that triggers it | not checked | Route A is an amplification vector and not merely a reflection one | compare the encoded sizes | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Route A makes ze a reflector for spoofed SCCRQ traffic | a rise in StopCCN transmissions with no matching tunnel | this is why the question goes to the owner. Route C exists for exactly this |
| R-2 | Emitting StopCCN allocates the tunnel state the current ordering avoids, creating a memory exhaustion path | tunnel table grows under a flood of malformed SCCRQ | emit without allocating, or bound the emission, and keep the parse-before-lock ordering |
| R-3 | Someone narrows the summary or writes a gap instead of asking | a diff touching `rfc/short/rfc2661.md` classifications | the ratchets refuse the cheap escapes, and this spec says the decision is the owner's |
| R-4 | The fix lands with a tagged test on the SCCRQ path that duplicates the existing id and disturbs the SCCRP tags | `make ze-rfc-check` reports an id or polarity change | decide with the owner whether the SCCRQ half needs its own requirement id, then regenerate the ledger in the SAME commit |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Route A wrongly applied turns ze into a packet reflector reachable by any spoofed source. Route B wrongly applied leaves a published MUST claim standing over behavior that does not exist. Both are worse than the current state, which is why the owner decides. |
| How is it reverted? | Route A is a single commit revert. A narrowed published claim is also revertible, but it is visible to readers in the meantime. |
| Who else touches this path? | `plan/spec-finish-l2tp.md` (L2TP residuals), `plan/spec-rfcgate-2-deferred-nonunit-evidence-backfill.md` (evidence carriers and tiers) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| SCCRQ with Assigned Tunnel ID 0 | → | `handlePacket` zero-ID branch | `TestSCCRQWithZeroAssignedTunnelIDIsAnswered` |
| SCCRQ with a non-zero Assigned Tunnel ID | → | `handlePacket` accept path | `TestSCCRQWithNonZeroAssignedTunnelIDEstablishes` |
| Zero-ID SCCRQ over the wire | → | the Reactor control path | `test/draft/l2tp/rfc2661-sccrq-tunnel-id-zero.ci` |

## Acceptance Criteria

<!-- AC-1 is deliberately the owner's answer. Every later AC is conditional on it,
     and the conditional wording is intentional: this spec must not presume a route. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The owner question above is put to Thomas | A recorded answer selecting route A, B or C, captured in Key Design Decisions with the rejected alternatives |
| AC-2 | Route A or C is chosen, and an SCCRQ carrying Assigned Tunnel ID 0 arrives | Ze transmits StopCCN with Result Code 2 and allocates no tunnel entry |
| AC-3 | Route B is chosen | The published coverage at `rfc/short/rfc2661.md:433` states the SCCRQ half is a deliberate silent drop, and the reason, so no reader is misled |
| AC-4 | Any route is chosen | The SCCRQ path carries an `RFC requirement:` tag in both polarities, with the id the owner's answer settles |
| AC-5 | `make ze-rfc-check` runs after the change | It passes, and `ai/RFC-REQUIREMENTS.md` is regenerated in the same commit |
| AC-6 | A flood of zero-ID SCCRQ datagrams arrives | The tunnel table does not grow, whichever route was chosen |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | A peer misconfigures its tunnel ID and dials ze | UDP → `handlePacket` → `parseSCCRQ` → reply or drop | `test/draft/l2tp/rfc2661-sccrq-tunnel-id-zero.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSCCRQWithZeroAssignedTunnelIDIsAnswered` | `internal/component/l2tp/reactor_test.go` | AC-2 or AC-3, per the owner's route | |
| `TestSCCRQWithNonZeroAssignedTunnelIDEstablishes` | `internal/component/l2tp/reactor_test.go` | AC-4 positive polarity | |
| `TestZeroIDSCCRQFloodAllocatesNoTunnel` | `internal/component/l2tp/reactor_test.go` | AC-6 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Assigned Tunnel ID | 1..65535 | 65535 | 0 (the case this spec is about) | N/A, the AVP is uint16 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rfc2661-sccrq-tunnel-id-zero` | `test/draft/l2tp/*.ci` then promoted to `test/l2tp/` | A peer dials with tunnel ID 0 and gets the behavior the owner chose | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| l2tp control | `test/l2tp-interop/scenarios/` | xl2tpd | Only if route A or C is chosen. Note that nothing runs this tree automatically today, per `plan/spec-rfcgate-2-deferred-unrun-interop-trees.md`, so a tag placed here earns no tier | |

## Files to Modify
- `internal/component/l2tp/reactor.go` - the drop site at `:353` to `:357`, if route A or C is chosen
- `internal/component/l2tp/tunnel_fsm.go` - `parseSCCRQ`, only if the owner's route needs the zero-ID case distinguished from other parse failures
- `rfc/short/rfc2661.md` - the coverage line at `:433` and the requirement at `:479`, ONLY as the owner's answer directs
- `ai/RFC-REQUIREMENTS.md` - regenerated by `make ze-rfc-index`, never hand-edited

## Files to Create
- `test/draft/l2tp/rfc2661-sccrq-tunnel-id-zero.ci` - the functional reproducer, drafted in the incubator first

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | N-A unless route C adds a rate-limit leaf. Decide after the owner answers |
| YANG validation constraints | | Same condition as the row above |
| YANG custom validators | N-A | No new leaf expected |
| CLI commands/flags | N-A | No command changes |
| CLI grammar (keyword before value) | N-A | No command changes |
| Editor autocomplete | N-A | No new leaf expected |
| Functional test for new RPC/API | Yes | `test/draft/l2tp/rfc2661-sccrq-tunnel-id-zero.ci` |
| Pipe completeness | N-A | No command output |
| Env var registration | N-A | No new env var expected |
| Doctor check for runtime dependencies | N-A | No new runtime dependency |
| Prometheus counters/metrics | | A counter for refused zero-ID SCCRQ is the early signal R-1 and R-2 both need. Decide at design time |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Conformance fix |
| 2 | Config syntax changed? | | Only if route C adds a rate-limit leaf |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | | Check the L2TP guide at design time |
| 7 | Wire format changed? | | Route A and C emit a StopCCN that is not emitted today |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc2661.md` and the `docs/features/rfc-status.md:204` RFC 2661 row, with source anchors |
| 10 | Test infrastructure changed? | | A new `.ci` in an existing suite. Confirm at design time |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | | Depends on the Integration Checklist row above |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/features/rfc-status.md:204` already anchors L2TP source files. Re-verify each |
| 17 | Existing docs show config/CLI/API examples for this area? | | Verify the L2TP guide at design time |

## Implementation Steps

1. **Phase: Ask (MANDATORY FIRST, BLOCKING)** - put the owner question to Thomas
   - Tests: none. No code changes in this phase
   - Files: Key Design Decisions records the answer and the rejected routes
   - Verify: AC-1. Validate A-1 through A-5 first, so the answer is informed rather than guessed
2. **Phase: Wiring** - the failing reproducer in the incubator
   - Tests: `test/draft/l2tp/rfc2661-sccrq-tunnel-id-zero.ci`
   - Files: the draft `.ci`
   - Verify: it reproduces today's silence, per `ai/rules/testing.md` draft-first
3. **Phase: Implement the chosen route**
   - Tests: the three unit tests above
   - Files: `internal/component/l2tp/reactor.go`, and `tunnel_fsm.go` if needed
   - Verify: AC-2 or AC-3, plus AC-6 under a flood
4. **Phase: Tag and publish** - align the evidence with the claim
   - Tests: the SCCRQ tags in both polarities
   - Files: `rfc/short/rfc2661.md`, then `make ze-rfc-index` regenerates the ledger
   - Verify: AC-4 and AC-5. Commit the regenerated ledger in the SAME commit
5. **Phase: Promote** - move the draft into the live suite
   - Tests: the promoted `.ci`
   - Files: `test/l2tp/`
   - Verify: green under `scripts/dev/stress-repro.py` before promotion

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file plus symbol |
| Feature completeness | The SCCRQ half is covered, and the SCCRP half still is |
| Correctness | The parse-before-lock ordering survives, so a flood allocates nothing |
| Naming | The new test names say SCCRQ, so nobody confuses them with the SCCRP pair |
| Data flow | No tunnel entry is allocated on the refusal path |
| Rule: `ai/rules/rfc-compliance.md` | The owner answered. No `{gap}`, `partial` or `{not-applicable}` was written by anyone else |
| Rule: `ai/rules/testing.md` | The existing `RFC2661-24.10-1` tags kept their ids and polarities |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Owner answer recorded | Key Design Decisions names the chosen route and the rejected ones |
| Behavior matches the claim | `grep -n "24.10" rfc/short/rfc2661.md` agrees with the code |
| SCCRQ tagged in both polarities | `grep -rn "RFC2661-24.10" internal/component/l2tp/` |
| Ledger fresh | `make ze-rfc-check` |
| No tunnel growth under flood | `go test -run TestZeroIDSCCRQFlood ./internal/component/l2tp/` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The Assigned Tunnel ID is peer-controlled and unauthenticated. The refusal must not depend on any later-validated field |
| Resource exhaustion | R-2. A reply path must not allocate tunnel state, or a malformed-SCCRQ flood becomes a memory attack |
| Amplification and reflection | R-1 and A-5. If a reply is sent, confirm it is not larger than the datagram that triggered it, and consider a per-source bound |
| Error leakage | A StopCCN Result Code must not reveal more about ze than the peer already knows |

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
- One requirement id covering two message types can be fully gated in both polarities while half of it is untested. The ledger counts polarities per id, so it cannot see this shape. Worth raising as a general pattern if a second instance appears.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- No code changes until the owner answers. That is the design, not a delay.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
