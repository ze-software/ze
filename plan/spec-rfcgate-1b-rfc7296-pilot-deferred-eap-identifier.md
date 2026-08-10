# Spec: rfcgate-1b-rfc7296-pilot-deferred-eap-identifier

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1. -->

| Field | Value |
|-------|-------|
| Status | blocked |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | - |
| Updated | 2026-08-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Provenance.** Found by an independent reviewer during round 7 of
the rfcgate-1b RFC 7296 pilot (closed), while it was making the EAP-TLS
termination follow RFC 5216 Section 2.1.3. The pilot is closed and its shard is
deleted, so this file is the tracker.

**The problem.** RFC 3748 Section 4.2 (`rfc/full/rfc3748.txt`, the Success and Failure
packet format) states that the Identifier field "MUST match the Identifier field of the
Response packet that it is sent in response to". `Session.failure`
(`internal/component/ike/eap/eap.go`) increments `s.identifier` and THEN stamps the
packet, and the `CodeSuccess` arm of `Session.handleMethod` does the same, so both
terminal packets carry the response Identifier plus one.

**Why it went unseen.** `PeerSession.Process`
(`internal/component/ike/eap/peer.go`) switches on `request.Code` alone and never
compares the Identifier, so Ze talking to Ze agrees with itself. A peer that enforces
Section 4.2 discards both the EAP-Failure and, by the same producer, the EAP-Success.

**Why it is here rather than fixed in the pilot.** The pilot's own goal does not depend
on it: the round-7 fix changed WHICH round the Failure lands on, not what it carries, so
the defect predates that work and survives it unchanged (`ai/rules/rule-precedence.md`,
the closing-order clause). It is a different RFC from the pilot's subject, and the
obligation is UNEXTRACTED: `rfc/short/rfc3748.md` carries `4.2-1` (retransmission),
`4.2-2` (format) and `4.2-3` (implicit success), none of which is this sentence.

## STATUS 2026-08-05: the RFC obligation is MET. One owner decision remains.

**Steps 1, 2, 3, 5 and 6 are done and committed (`ee305d5bc`). Step 4 is an owner
decision and is the only thing holding this spec open.**

The sender-side violation is fixed. `Session.failure` now takes the packet it
answers and stamps that Identifier, and the `result.Done` arm of
`Session.handleMethod` assigns `response.Identifier` instead of incrementing
(`internal/component/ike/eap/eap.go`). All five `failure` call sites were audited
in the same sweep and every one had the answered Response in scope, so no producer
was left behind. A Request still increments, because it opens a new exchange, and
`TestRequestIdentifierStillIncrements` pins that boundary: freezing the Identifier
everywhere would break retransmission matching and a Failure-only assertion would
not notice.

Extracted as `RFC3748-4.2-4` in `rfc/short/rfc3748.md`. The three existing
Section 4.2 ordinals were NOT renumbered, as step 2 required; `4.2-3` was already
taken by the implicit-success SHOULD, so the new row took the next free one.
Tagged single-polarity positive, with the reason recorded in the row: the
obligation binds the SENDER, so a negative case would need a receiver that
discards a mismatched terminal packet, which is step 4 below and not something
Section 4.2 asks of a sender.

Five tests, in `internal/component/ike/eap/rfc3748_identifier_test.go`. Both
halves mutation-verified at package scope with no `-run` filter. **The first round
had four tests and was not enough**: reverting the Success arm alone passed the
entire package, because every test drove a Failure. `doneMethod` and
`TestSuccessIdentifierMatchesResponse` exist because of that survivor. One mutant
per CLAIM, and "both terminal packets" is two claims.

`make ze-rfc-check` green: 2950 gated MUST-level requirements, 3264 tags resolved.
Step 6 needed no edit. `rfc3748` was already enrolled, so `check_new_summaries`
does not fire, and the gate's own status checks passed, which is what would have
caught a `docs/features/rfc-status.md` disagreement.

### The one open item: step 4, and it is Thomas's

**Should `PeerSession.Process` REJECT a terminal packet whose Identifier does not
match the Request it answers?**

It does not today. It switches on `request.Code` alone
(`internal/component/ike/eap/peer.go`), which is exactly why ze talking to ze
never noticed the sender-side bug for as long as it lived.

This is NOT a conformance question. RFC 3748 Section 4.2 binds the sender, and ze
is now conformant as a sender. A receive-side check is defensive hardening, and it
has a real cost: a strict peer refuses to complete against any implementation
carrying the bug ze just fixed, and ze cannot know how many of those exist. That
trade is the owner's to make, which is what the original step 4 says.

Do not close this spec by answering step 4 unilaterally in either direction.

**The work.**

1. Read RFC 3748 Section 4.2 and confirm the obligation and its exact wording.
2. Extract it as a new row in `rfc/short/rfc3748.md` with the next free ordinal for
   that section. Do NOT renumber the three existing ids.
3. Make `Session.failure` and the `CodeSuccess` arm of `Session.handleMethod` stamp the
   Identifier of the Response they answer. Check every other producer of a terminal EAP
   packet in the same sweep (`ai/rules/architecture.md`, sibling call-site audit).
4. Decide whether `PeerSession.Process` should REJECT a mismatched Identifier. Section
   4.2 binds the sender; a receive-side check is a separate decision and may belong to
   the owner, because a strict peer breaks against implementations with this same bug.
5. Tag the behaviour in both polarities and mutation-verify each tag at package scope
   (`go -overlay`, copies under `tmp/`, no `-run` filter).
6. Reconcile `rfc/enrolled.txt` and `docs/features/rfc-status.md` if the row changes what
   either claims (`ai/rules/rfc-compliance.md`, the seven ratchets).

**Constraint.** `rfc3748` enrolment status must be read before step 2: enrolling a
summary that declares gated MUSTs is itself gated by `check_new_summaries`, and adding a
gated row to an enrolled RFC without both polarities reds `make ze-rfc-check`.

## A second item, same file, same layer

**A NAK after the EAP-TLS alert loses the reported cause.** `Session.handleMethod`
(`internal/component/ike/eap/eap.go`) returns `s.failure()` for `TypeNAK` before
`tlsMethod.Process` runs, so the cause parked on `tlsMethod.alertSent` is never
consulted and the operator sees no reason for the rejection.

This is observability, not conformance: the EAP-Failure is still correct and still
sent, and a NAK legitimately means the peer refuses the method. It is recorded here
because it is the one reply shape a rejected peer can send that
`TestEAPTLSRejectedPeerCannotSteerTheReportedCause` cannot reach -- the other two, a
malformed TLS message and a foreign EAP type, are both covered -- and it is
unreachable from inside the method because the Session layer short-circuits above it.

Fixing it means the Session asking the method for a cause before it answers a NAK,
which is a change to the `Method` interface. Weigh that against the value of the log
line before doing it.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/architecture/<doc>.md` - [why relevant]
  → Decision: [specific architectural decision that constrains this spec]
  → Constraint: [specific rule from the doc that applies here]

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfcNNNN.md` - [why relevant]
  → Constraint: [specific RFC rule that applies here]

**Key insights:** (minimal context to resume after compaction)
- [insight from docs]

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `path/to/file.go` - [what it currently does]

**Behavior to preserve:** (unless the user explicitly said to change it)
- [output format, function signature, or `.ci` expectation callers depend on]

**Behavior to change:** (only what the user asked for)
- [list, or "None - preserve all existing behavior"]

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [Where data enters: wire bytes, API command, config, plugin message]
- [Format at entry]

### Transformation Path
1. [Stage 1: for example "Wire parsing in internal/component/bgp/message/"]
2. [Stage 2: ...]

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
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

<!-- LIVE: written during RESEARCH/DESIGN, statuses updated during implementation.
     Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis)
     land HERE, not only in conversation. -->

### Assumptions
<!-- Every row needs a validation method. `unvalidated` is not a valid final
     status: closure re-checks each one. A broken assumption also gets a
     Mistake Log row and a Deviations entry. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | [what this design assumes] | [where the assumption comes from] | [impact on design] | [test/grep/user confirmation] | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | [what goes wrong] | [how we notice it] | [what we do about it] |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | [live sessions dropped / routes mis-encoded / config rejected / nothing user-visible] |
| How is it reverted? | [single commit revert / needs config migration / not revertible once peers see it] |
| Who else touches this path? | [other plugins, components, or specs working the same files] |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by .claude/hooks/validate-spec.sh, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| [config/CLI/event that triggers it] | → | [function that actually runs] | [test name proving the chain] |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | [what triggers the behavior] | [observable outcome] |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | [for example "receives SR-Policy UPDATE from peer"] | [wire -> mpnlri -> splitter -> Parse -> RIB] | [test name] |

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
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-xxx` | `test/.../*.ci` | [what the user expects to happen] | |

### Interop Tests (Scope: protocol)
<!-- REQUIRED when wire-visible behavior changes. See
     ai/rules/interop-and-goal-validation.md, including the vacuity traps: prove
     the test FAILS when the behavior under test is reverted. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-feature-peer` | `test/interop/scenarios/` | [FRR/BIRD/GoBGP/strongSwan] | [protocol behavior validated] | |

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `internal/...` - [feature changes]

## Files to Create
- `internal/...` - [new feature file]
- `test/.../*.ci` - [functional test for end-user behavior]

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | `internal/component/<name>/yang/` or the owning plugin's `yang/`. Read `ai/rules/config.md` (YANG vs env var) and `ai/rules/config.md` (naming) |
| YANG validation constraints | | Every leaf takes maximum native validation: `range`, `length`, `pattern`, `enumeration`, `type` from `ze-types.yang`. See `ai/patterns/config-option.md` |
| YANG custom validators | | Where native constraints are insufficient: `ze:validate` + `ValidateFn` + `CompleteFn` for completion |
| CLI commands/flags | | `cmd/ze/*/main.go` or subcommand files |
| CLI grammar (keyword before value) | | `ai/rules/cli.md` |
| Editor autocomplete | | Automatic for YANG enum/type leaves. Dynamic values need `CompleteFn` |
| Functional test for new RPC/API | | `test/plugin/*.ci` or `test/decode/*.ci` |
| Pipe completeness | | Route output through `ApplyPipes`/`ProcessPipes` per `ai/rules/cli.md` |
| Env var registration | | YANG leaves under `environment/` need a matching `ze.<name>.<leaf>` via `env.MustRegister()` |
| Doctor check for runtime dependencies | | Any new file path, socket, service, kernel module, listen port, procfs/sysctl, netlink, binary, or certificate: owning-package check + `internal/core/diagnostic/codes.go` + unit and functional test (`ai/rules/repo-maintenance.md`) |
| Prometheus counters/metrics | | Observable state: define, register, and list the metric names and labels here |
| BGP family surface (new SAFI / capability / attribute) | | The 12-section checklist in `ai/patterns/bgp-family.md` -- read it and record the answers there, not inline |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | | `docs/features.md` |
| 2 | Config syntax changed? | | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | | `docs/guide/<topic>.md` |
| 7 | Wire format changed? | | `docs/architecture/wire/*.md` |
| 8 | Plugin SDK/protocol changed? | | `ai/rules/plugins.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | | `rfc/short/rfcNNNN.md` and the `docs/features/rfc-status.md` row, with source anchors |
| 10 | Test infrastructure changed? | | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | | `docs/comparison.md` |
| 12 | Internal architecture changed? | | `docs/architecture/core-design.md` or subsystem doc |
| 13 | Route metadata keys added/changed? | | `docs/architecture/meta/README.md`, `docs/architecture/meta/<plugin>.md` |
| 14 | Prometheus counters added/changed? | | `docs/plugin-development/metrics.md` or subsystem telemetry doc |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | | `docs/plugin-overview.md`, `docs/features/plugins.md`, `docs/guide/status.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | | Grep `docs/` for `source: <changed-file>` and update each stale claim |
| 17 | Existing docs show config/CLI/API examples for this area? | | Verify examples against YANG/parser/handler and update stale syntax |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

1. **Phase: Wiring (MANDATORY FIRST)** -- register entry points, write failing wiring tests
   - Tests: [wiring test names from the Wiring Test table]
   - Files: [register.go, handler skeleton, route registration]
   - Verify: the entry point exists and is reachable. The wiring test fails because the feature is a stub
2. **Phase: [name]** -- [what to implement]
   - Tests: [test names from the TDD Plan]
   - Files: [files from Files to Modify]
   - Verify: tests fail → implement → tests pass → wiring test progresses

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | [feature-specific, for example "merge order correct", "error messages name the offending value"] |
| Naming | [feature-specific, for example "JSON keys kebab-case", "YANG leaf matches env var leaf"] |
| Data flow | [feature-specific, for example "resolution in X only, reactor unaware of Y"] |
| Rule: [relevant rule] | [what to check] |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| [concrete thing that must exist] | [grep/ls/test command] |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | [what inputs need validation and how] |

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
<!-- LIVE: write immediately when you learn something. At closure these route to
     a subsystem arch doc, a rule, or the learned summary. -->

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     needs a row in the deferral shard named in the metadata table. -->
- [What was deliberately not done and why]

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
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
