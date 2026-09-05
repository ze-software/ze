# Spec: fourteen RFC 3748 obligations Ze meets and does not declare

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | spec-rfcgate-6-supported-extraction-signoff |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Fourteen requirement-stating sites in RFC 3748 state obligations Ze MEETS, and
`rfc/short/rfc3748.md` declares none of them. The behavior exists. The ledger does
not say so, so nothing tests it and no reader can see it.

Found by the second RFC 3748 walk under `spec-rfcgate-6-supported-extraction-signoff`
on 2026-09-01. The sites, in the row's own words: fourteen RFC 3748 sites state
obligations Ze MEETS that `rfc/short/rfc3748.md` does not declare: `2:1`, `2.2:1`,
`4.1:1`, `4.1:3`, `4.1:6`, `4.1:13`, `4.2:4`, `4.2:7`, `4.2:8`, `4.2:9`, `4.2:14`,
`7.10:3`, `7.10:10`, `7.10:11`. The reason each is met, with its producer, is the
`RESIDUAL` table in `plan/rfcgate-6-partial-walks/classify-rfc3748.py`.

That script is being deleted with the deferral directory, so its fourteen reasons
are carried here whole. Each is the walk's own finding, and each must be
re-verified at the producer before a requirement id is written.

| Site | Why the walk called it MET |
|------|---------------------------|
| 2:1 | MET by construction: the authenticator holds no timer, so nothing can make it send a terminal packet on a retransmission or a timeout. `Session.Process` runs only on a received packet (`eap.go`) |
| 2.2:1 | MET: `Packet.Encode` gives Success and Failure four octets and copies no TypeData (`eap.go`), and Ze emits no Nak and no Notification, so none of the four named messages can carry method data |
| 4.1:1 | MET through the carrier: the EAP layer holds no retry counter, and IKEv2 retransmits the IKE_AUTH message carrying the Request (`sa.LastSentMsg` and `cacheResponse`, `internal/component/ike/engine`) |
| 4.1:3 | MET: `handleRequest` answers every valid Request with a Response, and returns an error rather than silence for one it cannot answer (`peer.go`) |
| 4.1:6 | MET by construction: `PeerSession.Process` is synchronous and takes one packet, driven from the SA's own goroutine, so a second Request cannot be inspected before the first completes |
| 4.1:13 | MET by construction: `Packet.Type` is one octet and `Encode` writes exactly one at offset 4 (`eap.go`) |
| 4.2:4 | MET: the only Success is the `result.Done` arm of `handleMethod` and the only Failure follows a method refusal or a protocol error, so neither leaves at a point the method did not reach |
| 4.2:7 | MET through the carrier: a lost EAP-Success is retransmitted with the IKE_AUTH message that carried it, which is what RFC 3748 Section 4.3 directs for a reliable lower layer |
| 4.2:8 | MET by `stateLastWord` (`eap.go`), which exists for this sentence and quotes it in its own doc comment: the exchange answers whatever comes back after a failure result indication with the EAP-Failure |
| 4.2:9 | MET: `handleMethod` sends EAP-Success on `result.Done`, which for MS-CHAPv2 is the round that receives the peer's Success acknowledgement, so both indications have been exchanged first |
| 4.2:14 | MET: `handleMethod` sends EAP-Failure for a method refusal and reaches the Success arm only through `result.Done`, so a failed peer is never granted access |
| 7.10:3 | MET: the MSK feeds the AUTH payload PRF (`ComputeAuthFromMSK`, `internal/component/ike/engine/eap_auth.go`) and never a data-protecting key. The Child SA keys come from the IKEv2 KEYMAT derivation |
| 7.10:10 | Vacuously MET: Ze derives no EMSK anywhere under `internal/core/eap`, which is the same fact RFC3748-7.10-2 already records as not-applicable. The row this site needs states the confinement rather than the size |
| 7.10:11 | MET: when one party discards the key state the IKEv2 AUTH verification fails, the SA is torn down, and the initiator re-authenticates rather than wedging (`verifyRemoteAuth`, `internal/component/ike/engine`) |

`ai/rules/rfc-compliance.md` decides what this work owes. Making Ze better proven
never needs permission: it must be done and then reported. Each of the fourteen
gets a requirement row in `rfc/short/rfc3748.md`, an extraction mapping from the
site to that id, and a tagged test that asserts what the behavior actually
produces, with a discrimination record proving the tagged unit goes red when the
producing code is broken.

The row's own destination note said "needs a spec; Thomas decides whether it
runs". That decision is about SCHEDULING this spec. It is not a decision about
whether to declare less: the requirements are met, and recording them is the
better-proven direction the rule takes without asking.

RFC 3748's Meta already carries `Support ipsec 70`, `Support status Supported in
IPsec`. A summary that promises conformance owes an extraction sign-off that
bounds it, so these fourteen undeclared sites are part of what the sign-off must
account for.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` - the enrolment walk, the extraction sign-off, the eight ratchets, and the discrimination record
  → Constraint: a tag added, or a claim reworded, owes a discrimination record in the same change

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc3748.md` - the Meta table, the checklist, and what the summary already declares
  → Constraint: the claim must state what the test body checks and never more
- [ ] `rfc/full/rfc3748.txt` - the fourteen sites' own sentences, at the sections listed above
  → Constraint: a finding citing only a summary line or a requirement id is unverified

**Key insights:** (minimal context to resume after compaction)
- Every one of the fourteen is a MET obligation, so this is declaration and proof, not implementation
- Several are met THROUGH the IKEv2 carrier, and a stack-level answer still owes a test at the boundary Ze owns

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/core/eap/eap.go` - carries `(*Packet).Encode`, `(*Session).Process`, `(*Session).handleMethod`, `(*Session).failure`, `(*Session).finalRequest`, and the `stateLastWord` session state that exists for site 4.2:8 and quotes its sentence. This is the authenticator half of eight of the fourteen
- [ ] `internal/core/eap/peer.go` - carries `PeerSession.Process` and `handleRequest`, the peer half of sites 4.1:3 and 4.1:6
- [ ] `internal/component/ike/engine/eap_auth.go` - `ComputeAuthFromMSK` is the only consumer of the MSK, which is what makes site 7.10:3 met
- [ ] `rfc/short/rfc3748.md` - Meta declares `Support ipsec 70` and `Support status Supported in IPsec`. Its checklist declares none of the fourteen sites

**Behavior to preserve:**
- Every one of the fourteen behaviors: this spec declares and proves them, it does not change them

**Behavior to change:**
- None in the product. The change is fourteen requirement rows, their extraction mappings, and their tagged tests

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An IKE_AUTH exchange carries an EAP packet into `internal/core/eap`, and the RFC gate reads `rfc/short/rfc3748.md` and `rfc/extraction/rfc3748.json`.
- Format at entry: an EAP packet, Code, Identifier, Length and Type, inside the IKEv2 SK payload; and the summary's Meta table and checklist rows.

### Transformation Path
1. The IKEv2 engine hands the EAP packet to the session (`startEAPExchange`, `internal/component/ike/engine/fsm.go`)
2. `Session.Process` or `PeerSession.Process` dispatches on Code and Type
3. `handleMethod` runs the configured method and decides Success or Failure
4. `./le rfc check` reads the summary, the extraction artifact and the tagged tests

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| IKEv2 engine ↔ EAP layer | the EAP packet inside the SK payload | No |
| Summary ↔ gate | requirement rows and extraction mappings | No |

### Integration Points
- `./le rfc discriminate-record` - writes the discrimination record each new tag owes
- `rfc/extraction/rfc3748.json` - the sign-off artifact each site's mapping lands in

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
| A-1 | Each of the fourteen is still met at the producer | The 2026-09-01 walk, recorded above | A site is a `{gap}` rather than a declaration, and the rule's ask applies | Re-read each producer before writing its row | unvalidated |
| A-2 | A "met by construction" site can carry a test that would fail if the construction changed | Sites 2:1, 4.1:6 and 4.1:13 are structural | The claim is wider than the assertion, which is the violation with a green bar | Write the test first and check it discriminates | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A claim written wider than its test body converts an unproven MUST into a proven one on the public ledger | The discrimination record cannot break the tagged unit | Narrow the claim to what the body checks |
| R-2 | A site met through the IKEv2 carrier gets a test over the EAP layer alone, which proves nothing about the carrier | The test passes with the carrier removed | Assert at the boundary Ze owns, per `ai/rules/rfc-compliance.md` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The public RFC ledger claims fourteen obligations are proven when they are not |
| How is it reverted? | Single commit revert; no product behavior changes |
| Who else touches this path? | `plan/pre-release/spec-rfcgate-6-supported-extraction-signoff.md`, and the EAP specs under `plan/` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le rfc check` over `rfc/short/rfc3748.md` | → | the fourteen new requirement rows and their tags | [test name, fill at design time] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Each of the fourteen sites | Carries a requirement id in `rfc/short/rfc3748.md` and a mapping in the extraction artifact |
| AC-2 | Each new requirement id | Carries a tagged test whose claim states what the test body checks and no more |
| AC-3 | Each new tag | Carries a discrimination record written by `./le rfc discriminate-record` from an observed red |
| AC-4 | `./le rfc check` after the change | Passes, with no site left unmapped and no claim unproven |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Reads the published RFC 3748 row to decide whether Ze meets an EAP obligation | summary → generated page → requirement row and its test | [test name] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [fill at design time] | `internal/core/eap/rfc3748_test.go` | one tagged unit per new requirement id | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| EAP terminal packet length | 4 octets | 4 | 3 | 5 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| [fill at design time] | `test/` | An EAP conversation over IKE_AUTH reaches the asserted outcome | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| [fill at design time] | `test/interop/scenarios/` | strongSwan | The carrier-met sites hold against another implementation | |

## Files to Modify
- `rfc/short/rfc3748.md` - the fourteen requirement rows and the Meta cells they move
- `rfc/extraction/rfc3748.json` - the site-to-id mappings
- `internal/core/eap/` - the tagged tests, and any producer comment a new requirement id anchors

## Files to Create
- `rfc/discrimination/` - one record per new tag, written by `./le rfc discriminate-record`

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
| BGP family surface (new SAFI / capability / attribute) | | N-A: EAP, not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | | |
| 2 | Config syntax changed? | | |
| 3 | CLI command added/changed? | | |
| 4 | API/RPC added/changed? | | |
| 5 | Plugin added/changed? | | |
| 6 | Has a user guide page? | | |
| 7 | Wire format changed? | | |
| 8 | Plugin SDK/protocol changed? | | |
| 9 | RFC behavior implemented, changed, or newly proven? | | `rfc/short/rfc3748.md` and the `docs/features/rfc-status.md` row |
| 10 | Test infrastructure changed? | | |
| 11 | Affects daemon comparison? | | |
| 12 | Internal architecture changed? | | |
| 13 | Route metadata keys added/changed? | | |
| 14 | Prometheus counters added/changed? | | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | | |
| 16 | Any changed source file referenced by existing doc source anchors? | | DERIVED: `./le spec citation anchors spec plan/pre-release/spec-rfc3748-undeclared-met-obligations.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- re-verify each of the fourteen at its producer and write the requirement rows, with the tests failing
   - Tests: [wiring test names]
   - Files: `rfc/short/rfc3748.md`, `rfc/extraction/rfc3748.json`
   - Verify: `./le rfc check` names each new id as declared and untested
2. **Phase: [name]** -- [fill at design time]

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All fourteen sites carry a row, a mapping, a test and a record |
| Correctness | A site re-verified as NOT met becomes a `{gap}`, and the rule's question goes to the owner |
| Rule: `ai/rules/rfc-compliance.md` | Every claim quotes the RFC's own sentence, read in `rfc/full/rfc3748.txt` |
| Rule: `ai/rules/interop-and-goal-validation.md` | Each new tag's discrimination record comes from an observed red |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Fourteen requirement ids, mapped and proven | `./le rfc check` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Authorization that could fail open | Sites 4.2:4, 4.2:9 and 4.2:14 are the ones that stop a rogue authenticator granting access |

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

Each of the fourteen requirements gets `// RFC 3748 Section X.Y: "<quoted
requirement>"` above the code that enforces it, quoted from `rfc/full/rfc3748.txt`
at the section the site is numbered by.

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
