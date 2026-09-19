# Spec: rfc-evidence-deferred-unbootable-suite-musts

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Updated | 2026-09-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

**Status set to `blocked` on 2026-08-05**, from `skeleton`, because the first
deliverable was a decision only the owner could make. **Unblocked on 2026-09-07:
he made it on 2026-09-05 and the carrier half is built.** The remaining proof
backfill belongs here. The contract is in `design` until the per-requirement
test map and the separate DNS decision below are recorded; the BFD, DHCP and
VRRP suite decision is settled and must not be asked again.

## Task

At the 2026-08-02 measurement, 242 gated MUST-level requirements could not be proven at verify tier at all,
because no functional suite booted their subsystem: BFD 98, VRRP 80, dhcpserver
28, geodns 18, dnsserver 18. `internal/le/functional.Gating` is the only input
`carriers` (`internal/le/rfc/carriers.go`) reads to grant a verify tier, so a
`.ci` written outside it resolved to `functional-unrun` and the scanner REFUSED
the tag.

**The decision was the owner's and he made it on 2026-09-05:** *"bfd (98 MUSTs),
vrrp (80) and dhcp (28) have no suite: not acceptable, all RFC MUSTs need tests,
so we need to add them."* That is route 1 below, for three of the five
subsystems. The ranking and the selection rule that produced the question are in
`plan/learned/006-rfc-evidence-oracle-selection-rule.md`.

**The carrier half is built and is not this spec's work.** `bfd` and `dhcp` are
declared new and `vrrp` gained the `Gating` membership it lacked, so `CarrierFor`
answers `functional-bfd`, `functional-dhcp` and `functional-vrrp` at `verify`.
`TestTheBFDDHCPAndVRRPSuitesCarryAVerifyTier` (`internal/le/rfc/tags_test.go`)
pins it, and each suite carries one `.ci` proving it discriminates:
`test/bfd/bfd-detection-interval.ci`, `test/dhcp/dhcp-range-inside-subnet.ci` and
`test/vrrp/vrrp-config-invalid.ci`.

This spec owes functional verify-tier proofs for all 206 BFD, VRRP and DHCP
obligations in that snapshot: RFC 5880/5881/5883, RFC 5798 and RFC 2131/2132.
Every obligation owes positive and negative tagged assertions and a valid
discrimination record from `./le rfc discriminate-record` for each binding.
The number describes obligations, not a required number of files; sharing a
test never excuses an untested requirement or polarity.

The implementation inventory must reconcile those original obligations against
the current summaries under their permanent ids and include newly extracted
obligations. A changed count is not authority to drop any of the original 206.
`geodns` and `dnsserver`, measured at 36 MUSTs together, remain a separate owner
decision in this spec. The recorded instruction did not select their proof
carrier, and neither their unit-only evidence nor the three delivered suites
discharges that remainder.

### The three routes, and which one was taken

| Route | What it costs | What it buys |
|-------|---------------|--------------|
| TAKEN for bfd, dhcp and vrrp. Add a verify-tier suite per subsystem | suite infrastructure, now delivered | every original obligation has a verify-tier proof carrier; the tests still have to be written and run |
| Accept nightly-only tier for these | a tier that is scheduled and advisory, not merge-gating | reachable today for VRRP, which has `ze-qemu-vrrp-keepalived-test`; the others have no nightly path either |
| Leave them unit-only by decision | the obligation stays proven at the wrong altitude | nothing new to build |

Do not write `{gap}` for any of the 242: an annotation that lowers what Ze owes
is a compliance decision, not bookkeeping.

### Constraints

- `FunctionalSuites` and `suiteCarriers` (`internal/le/rfc/carriers.go`) read
  `functional.GatingNames()`, backed by `Gating` in
  `internal/le/functional/suites.go`. BFD, DHCP and VRRP are in that owner set.
  A suite outside it confers no verify tier.
- A verify tier records the gating runner's ownership. It does not prove that a
  particular selected gating run executed a particular test.
- Counts are the 2026-08-02 snapshot. Before implementation, reconcile requirement
  ids from the summaries and evidence bindings from the native RFC collector;
  record the dated population and every addition or correction without lowering
  the owner's full-proof obligation.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - the canonical architecture reference: the design principles all new code follows
- [ ] `docs/architecture/testing/ci-format.md` - the `.ci` test file format: embedded files, options, expectations and commands
- [ ] `internal/le/rfc/carriers.go` - `FunctionalSuites`, `suiteCarriers`, `CarrierFor`
  → Constraint: an unrun carrier is refused; suite membership alone proves no requirement.
- [ ] `internal/le/functional/suites.go` - `Gating`
  → Constraint: this owner set supplies the functional verify tier.
- [ ] `plan/pre-release/spec-rfcgate-2-deferred-unrun-interop-trees.md` - the sibling problem for interop trees
  → Decision: its scheduled interop carriers do not replace the verify-tier proof selected here.
- [ ] `ai/rules/rfc-compliance.md` - who decides when full proof is not reachable
  → Constraint: ask which way to fix it, never whether to skip it.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc5880.md`, `rfc/short/rfc5881.md`, `rfc/short/rfc5883.md`, `rfc/short/rfc5798.md`, `rfc/short/rfc2131.md`, `rfc/short/rfc2132.md`
  → Constraint: map every gated obligation to positive and negative functional assertions; retain the separate DNS population pending the owner's carrier decision.

**Key insights:** (minimal context to resume after compaction)
- The infrastructure blocker is gone for bfd, dhcp and vrrp: a `.ci` in `test/bfd/` now earns `functional-bfd` at `verify`. What is left is writing 206 tagged tests in both polarities.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/le/rfc/carriers.go` - derives verify-tier rows from `FunctionalSuites`; `CarrierFor` matches the suite prefix.

**Behavior to preserve:**
- The `TIER_UNRUN` refusal. It keeps false evidence out and must not be softened to make these 242 look proven.

**Behavior to change:**
- Backfill the BFD, DHCP and VRRP functional proofs and discrimination records. Carrier registration is already delivered.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A `# RFC requirement:` tag in a `.ci` under a subsystem directory.

### Transformation Path
1. `CarrierFor` resolves the `.ci` path against the suite-derived carrier table.
2. The scanner reads the requirement tags and binds each to its evidence kind
   and execution tier.
3. BFD, DHCP and VRRP have verify-tier carriers. An unrun carrier's tag is
   refused; an accepted tag still owes its behaviour and discrimination proof.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Test tree ↔ ledger | `ScanTree` over suite-resolved `.ci` tags | Source-derived carrier membership; execution and proof remain owed |

### Integration Points
- `internal/le/functional/suites.go` `Gating` and `functional.GatingNames()` supply existing carrier membership.

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
| A-1 | Every remaining obligation is reachable by its existing functional harness | suites exist, but the initial DHCP proof covers config validation and live DHCP needs privileged UDP 67 | a suite smoke test cannot discharge the wire obligation | map each requirement to its enforcing producer and executable stimulus, including any privileged harness needed | unvalidated |
| A-2 | The historical 206 obligations map completely to current permanent ids | 2026-08-02 count and the 2026-09-05 owner instruction | obligations vanish behind a new denominator | reconcile original and current populations before implementation; retain every original obligation | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The full proof backfill exceeds the existing suites' runtime allowance | owning suite runtime rises | measure and address runtime without dropping obligations or replacing functional proof with unit-only tags |
| R-2 | Carrier membership is presented as completed requirement proof | a suite exists but the requirement has no discriminating assertion | reconcile every obligation and both polarities against the actual test body and discrimination record |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A public proof claim outruns the behaviour its tests exercise, or the required full-proof backfill is silently reduced |
| How is it reverted? | Preserve carrier membership and existing evidence; a removed proof is an evidence loss that must be reported rather than hidden by changing the suite set |
| Who else touches this path? | Sessions working the BFD, DHCP and VRRP suites, their producers and RFC evidence records |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `.ci` under `test/bfd/`, `test/dhcp/` or `test/vrrp/` | → | `FunctionalSuites`, `suiteCarriers`, `CarrierFor` | `TestTheBFDDHCPAndVRRPSuitesCarryAVerifyTier` (existing carrier check; it does not replace the requirement proofs) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The original 206 BFD/VRRP/DHCP obligations and the current summary ids are reconciled | A dated requirement-to-test map accounts for every original obligation and every newly extracted obligation; no count change, annotation or carrier downgrade removes a proof owed |
| AC-2 | Each obligation in AC-1 is exercised through the running subsystem | Positive and negative tagged functional assertions in `test/bfd/*.ci`, `test/dhcp/*.ci` or `test/vrrp/*.ci` observe its required behaviour at verify tier; a config-only assertion cannot stand in for an unexercised wire requirement |
| AC-3 | Each requirement/polarity/test binding from AC-2 | `./le rfc discriminate-record` records a break that makes that named test fail; the restored producer passes and the record verifies against the final tree |
| AC-4 | The proof inventory is checked after the backfill | Every AC-1 obligation has both functional polarities and valid discrimination evidence, with no loss of existing evidence; each owning suite has a recorded execution result |
| AC-5 | The separate geodns/dnsserver remainder is reconciled | Thomas's carrier decision is recorded for the historical 36 obligations and their current id set. Any resulting proof work remains open here until completed, or is named in an owner-approved live destination spec before this spec can close |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | A contributor checks one BFD, DHCP or VRRP MUST | running subsystem → functional assertion → requirement tag → discrimination record | the requirement-to-test map required by AC-1 names both polarity proofs |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTheBFDDHCPAndVRRPSuitesCarryAVerifyTier` | `internal/le/rfc/tags_test.go` | existing suite membership; no substitute for AC-2 | Existing |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Per-requirement protocol bounds | From the six RFC summaries | Required legal edge | Required refusal edge | Required refusal edge |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| BFD requirement proofs, positive and negative | `test/bfd/*.ci` | RFC 5880/5881/5883 behaviour through the running subsystem | Owed |
| DHCP requirement proofs, positive and negative | `test/dhcp/*.ci` | RFC 2131/2132 behaviour, including live exchanges where the requirement binds wire behaviour | Owed |
| VRRP requirement proofs, positive and negative | `test/vrrp/*.ci` | RFC 5798 behaviour through the running subsystem | Owed |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ze-qemu-vrrp-keepalived-test` | `test/` | keepalived | the one existing nightly path, for VRRP only | |

## Files to Modify
- `test/bfd/*.ci`, `test/dhcp/*.ci`, `test/vrrp/*.ci` - extend existing proofs where they cover the required behaviour.
- `rfc/discrimination/<stem>.json` - native-recorded proofs for the six in-scope RFCs.
- `rfc/short/<stem>.md` - only evidence-related factual corrections justified by the requirement walk; no reduced obligation.

## Files to Create
- Requirement-specific `.ci` files under `test/bfd/`, `test/dhcp/` and `test/vrrp/`, as named by the AC-1 map. Any missing harness capability must be designed before claiming its obligation is reachable.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | test infrastructure, no config surface |
| YANG validation constraints | No | no leaf added |
| YANG custom validators | No | no leaf added |
| CLI commands/flags | No | no command added |
| CLI grammar (keyword before value) | No | no command added |
| Editor autocomplete | No | no leaf added |
| Functional test for new RPC/API | Yes | existing BFD, DHCP and VRRP suites carry the required subsystem proofs |
| Pipe completeness | No | no command output added |
| Env var registration | No | none added |
| Doctor check for runtime dependencies | No | no new runtime dependency |
| Prometheus counters/metrics | No | no observable state added |
| BGP family surface (new SAFI / capability / attribute) | N-A | not a BGP change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | test infrastructure |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `ai/RFC-REQUIREMENTS.md`, and `docs/features/rfc-status.md` if a support level moves |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, if a suite is added |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | No | fill at design time |
| 17 | Existing docs show config/CLI/API examples for this area? | No | - |

## Implementation Steps

1. Reconcile the original population and current ids, inspect each enforcing
   producer, and name both test polarities in the AC-1 map. Resolve harness
   prerequisites, including privileged wire exchanges, before implementation.
2. Write and run the BFD, DHCP and VRRP functional proofs in their existing
   suites. Preserve unit and interop evidence already attached to those ids.
3. Record each binding's discriminating break through the native writer, run
   the restored proof, and reconcile the final inventory against AC-1.
4. Put only the still-open geodns/dnsserver carrier decision to Thomas. Record
   its resulting work and ownership under AC-5; do not repeat the settled
   BFD/DHCP/VRRP infrastructure question.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every original obligation has both functional polarities and discrimination evidence; DNS remains explicitly owned until its decision and resulting work are resolved |
| Tier honesty | No carrier gains a tier before a runner exists |
| Rule: `ai/rules/rfc-compliance.md` | No `{gap}` written for any of the 242 |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Reconciled requirement-to-test map | original obligations matched to current permanent ids and named proofs |
| BFD, DHCP and VRRP full-proof backfill | both tagged polarities, suite execution and native discrimination records |
| DNS decision and resulting work | recorded owner decision and completed work or an approved live destination |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | a new suite boots daemons on loopback; check no test binds a routable address |

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

## Known Limitations
- The existing suites prove only the behaviours their test bodies exercise. Their presence does not complete the 206-obligation backfill, and the DNS carrier decision remains open.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
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

## Work Inherited From a Deferral Row

<!-- The deferral directory was deleted on 2026-09-05. A row that named this spec as
     its destination is reproduced here, so the item and the reasoning behind it
     survive the directory. Each row is outstanding work this spec owns. -->

### From `rfcgate-2-deferred-nonunit-evidence-backfill.md`, 2026-08-02

Deferred by spec-rfcgate-2-deferred-nonunit-evidence-backfill.

Decide what to do about 242 gated MUSTs whose subsystem no `./le functional` suite boots (BFD 98, VRRP 80, dhcpserver 28, geodns 18, dnsserver 18)
