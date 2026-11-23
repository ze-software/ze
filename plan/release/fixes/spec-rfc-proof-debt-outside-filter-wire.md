# Spec: rfc-proof-debt-outside-filter-wire

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol \| tooling |
| Priority | P1 |
| Release reason | A release profile needs every mandatory requirement covered, and a requirement counts as covered only when a recorded break proved its test goes red (`ai/META-DECISIONS.md`, "Goal and scope"). Every row below is a requirement whose proof or audit verdict `./le rfc check` no longer accepts, so the public ledger carries it as unproven until the debt is paid |
| RFC gaps | - |
| Depends | - |
| Phase | D-2 document-author evidence, D-1 raw VPNv6 family admission, remaining host proof debt |
| Handoff | - |
| Updated | 2026-11-22 |

Scope is tooling and protocol. The proof-debt work re-takes discrimination
records, re-judges audit verdicts, and adds the two RFC 4271 negative tests
of AC-32; it changes no product behaviour. A product defect an audit or a
re-take exposes is fixed under `ai/rules/completion.md`, inside this spec,
because it blocks the verdict.

Two items this spec carries are designs kept here by D-1 and D-2 (2026-11-21)
and approved by Thomas on 2026-11-22:
- **AC-27 (D-1, approved 2026-11-22):** Enforce `negotiated.SupportsFamily(fam)`
  on valid raw UPDATE writes (`SendRawMessage` / `writeOriginatedRawUpdate`),
  following the RFC 8212 / RFC 7607 precedent in `session_write.go`, and document
  the rule in `docs/architecture/api/update-syntax.md`.
- **AC-35..AC-39 (D-2, approved 2026-11-22):** Keep `subjectBreakers` strict in
  `internal/le/rfc/notapplicable_roles.go` and add a dedicated, closed
  document-author evidence shape (parallel to `roleSentenceRefusal`) that admits
  `{not-applicable}` for a document/specification role only when the quote is the
  row's complete sentence in the cited section, explicitly names a document or
  specification as subject or passive agent, holds zero protocol-actor words, and
  carries an independent audit confirming no runtime code path exists.

## Task

The final `./le rfc check` run of the filter-wire umbrella printed 98
violations on 2026-11-02, and the umbrella's RFC records agent classified each
against the commit that caused it. The filter-wire work repaid the rows its
own commits caused. This spec owns the rest: proof debt that commits outside
the filtering changes left behind, which the owner put on the release fix
list. On 2026-11-19 the owner also gave this spec the whole stale-proof
backlog that accumulated after that run.

The rule the work runs under is the owner's (`ai/META-DECISIONS.md`, "Test
changes"): a proof belongs to its test. It goes stale only when the tagged
test or its claim changes, and a change to the code the test checks, the
function the record broke included, leaves it standing. The tooling already
implements it (6db7cb8964, `internal/le/rfc/discriminate.go`
`verifyOneDiscrimination`), and `./le commit create` refuses a commit that
changes a tagged test or its claim without the re-taken record (6eec153998,
narrowed in 6db7cb8964). This spec owes no gate change.

The goal: `./le rfc check` names no stale record, no SHIFTED verdict and no
STALE verdict outside the rows AC-33 leaves to their owners, every record was
observed red by `./le rfc record`, and every STALE verdict was re-made by an
independent auditor.

### The live set

The set is whatever `./le rfc check` names. Refreshed on 2026-11-22 over the
working tree (down from 414 violations on 2026-11-19 to 270 violations; after
`cf5b241072` GR/MRT re-audits, 14 fewer STALE and 2 fewer SHIFTED):

| Class | Lines (2026-11-22) | Owner |
|-------|--------------------|-------|
| Discrimination records that no longer verify: 10 Linux-guest (AC-40 RFC4861 ×9, AC-44 RFC4659 ×1), 5 RFC 9252 SRv6 (AC-45), 1 RFC 7606-5.1-1 negative (AC-47). AC-41, AC-42, AC-43, AC-46, AC-48, and RFC8955-6-5 are already MET | 16 | AC-28, AC-40, AC-44, AC-45, AC-47 |
| SHIFTED audit verdicts, 17 stems | 54 | AC-29 |
| STALE audit verdicts, 45 stems (rfc4271 41, rfc5176 24, rfc4301 11, ...) | 177 | AC-30, with AC-7, AC-8 and AC-16 inside it |
| RFC4271-5-3 and RFC4271-5-4 with no negative test | 2 | AC-32 |
| RFC9252-3.3-2 partial scope with no current verified break, both polarities | 2 | AC-32, cleared by the AC-45 re-take |
| RFC7752-3.4-1 and RFC9552-5.5-1, partial scope missing a negative | 2 | AC-33 |
| RFC7871-11.2-1 missing negative (DNS resolver work) | 1 | AC-33 |

What earlier work already paid, so the sizes above are not the handover's:

| Commit | What it paid |
|--------|--------------|
| 4dfbf85808, 6bbd7fd8e2 | 109 of 141 stale revert records re-taken on their recorded route |
| 82ab3fb5a5 | the RFC7606-5.1-2 and RFC8092-3-1 functional records |
| f30e0bd589 | 50 stale mutant records re-taken from hand-written breaks, each read by a judge first |
| 68d21a1c7f | 211 SHIFTED verdicts resealed in 40 stems |

Three more RFC 7606 entries were recorded and are not committed
(`rfc/discrimination/rfc7606/RFC7606-5.1-1.json` and `RFC7606-5.1-3.json` are
modified in the tree): AC-48.

## Owner decisions

Decided 2026-11-21: no to both moves ("go back to design"). Both items stay
in this spec, the spec returns to `design`, and Scope gains protocol, as
each row's "If no" column states.

| ID | Decision | Evidence | If yes | If no (taken) |
|----|----------|----------|--------|-------|
| D-1 | Move AC-27 (raw VPNv6 family admission) and its journal row to `plan/release/validation/spec-bgp-mp-attribute-first-wire-proof.md` | That spec is `ready`, Scope protocol, and already owns the sibling 2026-11-15 finding on the same producers (`Session.SendRawMessage`, `writeOriginatedRawUpdate`, `writeUpdateBody`), the same operator page ("Raw Passthrough Commands") and a row in the same journal class. AC-27 may change what a sender emits, so it owes the bgp-engine door, a design approval and a wire or interop test, none of which a tooling spec carries | This spec is pure proof debt and its `ready` status is true. AC-27, A-5, R-4 and the journal row go with the item | This spec holds an unapproved protocol design, so it goes back to `design` until AC-27's design is presented and approved, and its Scope gains protocol |
| D-2 | Move AC-35 to AC-39 (document-author evidence, formerly DA-1 to DA-5) and their journal row to `plan/release/validation/spec-rfc-evidence-strength-3-owed-musts-and-enrolment.md` | That spec is `design`, Scope tooling, and its Task is the MUST rows that still owe a test or an annotation. RFC7606-8-1 and RFC5492-4-3 are two such rows whose obligation no runtime test can prove. Second candidate: `plan/release/validation/spec-excluded-requirement-owes-an-observable-test.md` (`skeleton`, Scope tooling), which changes the same exclusion machinery but asks the opposite question (a test owed by an excluded row). No existing spec names `boundRoleRefusal` or either row | As D-1: the evidence-rule design is approved where the evidence specs are designed | This spec goes back to `design` until the change to "The exclusion evidence ratchet" is presented and approved |

Not an owner decision, and why:

| Question | What settles it |
|----------|-----------------|
| RFC 2545 citation 5 (AC-4 to AC-6) | The interop records were re-taken by hand-written mutants citing assertions 4 and 6 (29f15481fa, last written by 22f651cf53) and `./le rfc check` names no RFC 2545 record. `checkRFC2545NextHops` still asserts 5 (FRR installs the route via the link-local) and no record cites it. Whether the positive claim's installation clause says more than a record citing 4 proves is the auditor's question under AC-7, and a claim found too wide is narrowed through `ze-test-judge` |
| RFC5176-3.1-1 staying green under a `handleDisconnect` halt (AC-42) | `ai/META-DECISIONS.md`, "Test changes": a break is the violation the claim names, and a test shown wrong goes to `ze-test-judge`, never to the owner |
| RFC 4271 5-3 and 5-4 polarity (AC-32) | Same section: the negative test is owed, so the annotation route is not taken |

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` - every gate this spec clears
  → Decision: a record says a named tagged unit was OBSERVED to fail under a named break; `./le rfc check` replays nothing and compares fingerprints, so a moved unit or claim fingerprint means the red must be observed again; a producer change leaves the proof standing ("The ratchets", the discrimination row; `ai/META-DECISIONS.md` "Test changes")
  → Constraint: `./le rfc reseal stem <stem>` re-stamps only a verdict whose unit is byte-identical and shifted, in the stems named and no other; a shift in a stem another session is editing is not the caller's to re-stamp ("The artifacts", the `rfc/audit` row)
  → Constraint: `reseal ... source head` and `audit-stamp ... mode rejudge source head` read committed source, so they are safe while other sessions edit the checkout; the committed route refuses a partial row or verdict
  → Constraint: a STALE verdict is re-made by a judge and stored with `./le rfc audit-stamp stem <stem> from <path> mode rejudge`; an upgrade from `weak`, `wrong` or `partial` to `enforced` over unchanged units needs an `upgrade_reason`
  → Constraint: an independent judge reads the `break` text of every hand-written mutant before it is committed, and the commit body says the break was written by hand
- [ ] `docs/architecture/testing/interop.md` - the interop and guest records
  → Constraint: a record rebuilds the artifact the scenario drives so the break takes effect, and a Docker start under load times out rather than proving anything
- [ ] `ai/skills/ze-rfc-audit.md` - the audit a STALE row owes
  → Constraint: the auditor is independent of whoever changed the unit, and reads the RFC's own text in `rfc/full/`
- [ ] `ai/skills/ze-test-judge.md` - the judge of a changed test and of a hand-written break
  → Constraint: the agent that made the change never judges it, and a refusal stands
- [ ] `ai/doors/le-tooling.md`, `ai/doors/routing-protocols.md`, `ai/doors/access-security.md`, `ai/doors/bgp-engine.md`, `ai/doors/bgp-plugins.md`, `ai/doors/hub-engine.md`, `ai/doors/forwarding.md` - the subsystems whose tagged units this spec re-proves
  → Constraint: a bare `go test` omits feature tags; run a package through `./le test unit pkg path <dir>/` or `./le job run` with the tags written out (le-tooling, "Boundaries")
  → Constraint: the public conformance ledger is an outside party of every protocol subsystem (routing-protocols, "Who depends on it from outside")
  → Constraint: an untagged package test does not link BGP (bgp-engine, "Traps")
- [ ] `plan/handover/49-rfc-proof-debt-retakes.md` - what the re-take batches did and refused
  → Decision: the refused records fall in classes with different routes, so each class is its own acceptance criterion (AC-40 to AC-47)

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/<stem>.md` for each of the 47 stems of AC-30 and the stems of AC-40 to AC-47 - the rows re-judged and re-proved
  → Constraint: a verdict is judged against `rfc/full/<stem>.txt`, never the summary (`ai/rules/rfc-compliance.md`)

**Key insights:**
- Every row is settled by `./le rfc check`. A row is met only with the commit that paid it and a check run that names no line for it.
- A producer that moved or vanished stales nothing. A record whose unit changed is re-taken against the producer the unit exercises today, which may be a different function from the one recorded.
- RFC8212-3-1 is not in this spec: its producer moved in 2fd7958d6f and fa54cd4285, both filter-wire commits, and the umbrella re-took it.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `./le rfc check` output of 2026-11-19 over the working tree at HEAD c0ecf3f3b7 - 414 violations, classified in "The live set"
- [ ] `internal/le/rfc/discriminate.go` `verifyOneDiscrimination` - compares the unit and the claim fingerprints only; its comment says the producer and the paired producers are not read, and the run above prints `unit-changed` for all 32 stale records
- [ ] `internal/le/rfc/discriminate_commit.go` `judgeStaleRecords` - the commit-time refusal; a producer file is not judged, and a record already stale at the parent refuses nothing
- [ ] `rfc/discrimination/rfc2545/RFC2545-3-3.json` - the interop records of `checkRFC2545NextHops` are on the mutant route, positive citing 4 and negative citing 6 with a paired break and two green solo runs; they verify
- [ ] `internal/le/interoplab/bgp/check_rfc.go` `checkRFC2545NextHops`, `check_special.go` (the scenario map) - seven numbered assertions for scenario `bgp-rfc2545-linklocal-nexthop-frr`; assertion 5 reads FRR's installed route
- [ ] `internal/component/bgp/reactor/link_scope.go` `sharesNextHopSubnet`, `linkLocalNextHop`, `thirdPartyLinkLocal` - the producers the RFC 2545 records break
- [ ] `rfc/discrimination/rfc4861/RFC4861-9-1.json` - the subscriber unit's record names `internal/component/l2tp/ppp/ra_linux.go::rsReaderLoop`; that file is gone
- [ ] `internal/component/l2tp/ppp/ra_carrier.go` `rsReaderLoop` - where the function lives now
- [ ] `internal/component/l2tp/plugins/authradius/rfc5176_proxy_state_opaque_test.go` `TestRFC5176ProxyStateContentIsNeverRead` - asserts a Disconnect-NAK, the same Error-Cause with and without a session-shaped Proxy-State, and zero teardowns, for an unknown session
- [ ] `internal/component/bgp/reactor/session_write.go` `writeUpdateBody`, `session_ownership.go` `adjOutWrite.section` - read for AC-27 only (see its section)
- [ ] 6c22530c23 commit body - three `RFC-approval-owed:` lines, for `storage.TestRFC4271PartialBitSurvivesLengthReframing`, `reactor.TestRFC4271PartialNotSetOnRecognizedOrNonTransitive` and `attribute.TestRFC4271PartialNotStampedOnExcludedClasses`
- [ ] `internal/le/rfc/selftest_test.go` `TestRFCSelftestRealTreeRowReportsOwnedCheckFailures` - hit its limit under load twice with no failing assertion; no run of it alone on a quiet host is on record (closed `spec-lint-backlog-3-test-tooling` at `948f5717a9` records the loaded runs)

**Behavior to preserve:**
- Every requirement's level, text, tags and claim prose. A record or verdict is retaken against the unit as it is; no tag, claim or assertion is edited to make a record or verdict pass
- The refusals of `./le rfc check` and of the commit gate, unchanged

**Behavior to change:**
- None by design. Whatever product or test defect a re-take or an audit exposes is fixed at its producer, through `ze-test-judge` where a test changes

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| `./le rfc record` | `internal/le/rfc`, gates page "Producing a record: the two proof routes" | Applies a break through a Go overlay, runs the tagged unit, writes a record only for a red that names the unit | Uses it unchanged for every re-take; no record is written by hand |
| `./le rfc discriminate stem <stem> report <path>` | same package | Proposes candidate breaks for a tagged unit | Uses it to find the producer a changed unit exercises today (AC-41, AC-42, AC-46, AC-47) |
| `./le rfc check` | gates page "The gate's own answer" | Compares record and verdict fingerprints with the tree and names every one that moved | The settling command of every AC |
| Proof staleness rule | `internal/le/rfc/discriminate.go` `verifyOneDiscrimination` | A record is stale only when its unit or claim fingerprint moved | Does not apply as work: already implemented (6db7cb8964). It decides which records are owed |
| Commit-time stale-record refusal | `internal/le/rfc/discriminate_commit.go` `judgeStaleRecords`, gates page "The commit carries the record it stales" | Refuses a commit that changes a tagged test or its claim without the re-taken record, printing the `./le rfc record` line for each | Stops the backlog growing while this spec repays it; its printed lines are the command form of a recorded-route re-take |
| `./le rfc audit-stamp ... mode rejudge` | gates page "The artifacts", `rfc/audit` row | Replaces a re-made verdict in place with fresh fingerprints; `source head` stamps from committed source | Stores every STALE re-judge |
| `./le rfc reseal stem <stem>` | gates page "The artifacts" | Re-stamps a byte-identical, shifted unit's verdict for the named stems; `source head` binds it to committed source | Clears every SHIFTED verdict of AC-29, one run per stem |
| `/ze-rfc-audit` skill | `ai/skills/ze-rfc-audit.md` | An independent judge re-reads the RFC and the tests and writes a verdict | Every STALE row is re-judged through it by an agent that did not change the unit |
| `/ze-test-judge` skill | `ai/skills/ze-test-judge.md` | An independent agent judges a test change that could lower what a test proves, and reads a hand-written break | Reads every hand-written mutant before commit; judges any test this spec changes; gives the three owed verdicts of AC-32 |
| QEMU guest runs | `./le test qemu`, `ai/rules/platform-linux.md` | Runs a Linux-only unit in a guest with a named kernel | Carries the Linux-only re-takes of AC-40 and AC-44 |
| Job admission | `./le job run`, `docs/contributing/running-commands.md` | Admits and logs one CPU-intensive run at a time | Every package run and every record goes through it, in the main thread |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le rfc record`, `./le rfc audit-stamp`, `./le rfc reseal`, each reading `rfc/short/<stem>.md` and the tagged units
- Output: JSON under `rfc/discrimination/<stem>/` and `rfc/audit/<stem>/`

### Transformation Path
1. The tagged unit and its claim are fingerprinted; the producer is recorded as provenance
2. `./le rfc record` overlays the break, runs the unit, observes the red, writes the record with the new fingerprints
3. The independent auditor writes a pending verdict outside `rfc/audit/`; `audit-stamp mode rejudge` stamps it
4. `./le rfc check` re-reads every record and verdict and names none of this spec's rows

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| le tooling to the Go toolchain | overlay build and `go test` of the tagged package | No |
| le tooling to a QEMU guest (AC-40, AC-44) | the Linux-only unit runs in the guest | No |

### Changed-Wire Census

**Search evidence:** The proof-debt ACs change no wire. They write only JSON
evidence files under `rfc/discrimination/` and `rfc/audit/` through their own
writers, and two negative unit tests (AC-32). Scope searched: the Files to
Modify list below against `internal/core/bgp`, `internal/component/bgp` encode
and decode entry points; no producer is edited by them. AC-27 may change the
wire: `writeUpdateBody` (`internal/component/bgp/reactor/session_write.go`)
skips family admission for raw writes, and AC-27's design owes this census
for that writer, its `SendRawMessage` and `writeOriginatedRawUpdate` callers,
and every path that reaches it with `raw=true`, before it is presented.

### Integration Points
- `rfc/discrimination/<stem>/<id>.json` and `rfc/audit/<stem>/<id>.json` - the files the ACs rewrite, through the tools only

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | every artifact is written by its own `./le rfc` action, none by hand |
| No unintended coupling (components stay isolated) | Yes | no product package gains an import |
| No duplicated functionality (extends existing, does not recreate) | Yes | Existing Mechanisms: every tool is reused |
| Zero-copy preserved where applicable (refs, not copies) | N-A | no encoding code changes |
| Registration over hardcoding, outbound | N-A | nothing new registers |
| Registration over hardcoding, inbound | N-A | no list learns a new name |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Each cause commit named in AC-1 to AC-24 is not a filter-wire commit | umbrella stale classification | the row belongs to the umbrella | `git log -1 --format=%B` of b5fdf9a87f, c782411791, 1f4e1e81f1, 8c68fcf514, 8f704db860, f23e06435c, 898caba3f6 and c44e1bdc8c read 2026-11-19: none carries a `Spec-work: filter-wire` line | confirmed |
| A-2 | The RFC 2545 interop records failed only from load, not from a defect | records agent handoff | a real defect blocks the record | the records were taken (29f15481fa, later re-takes to 22f651cf53), each `solo` entry of the paired negative shows the scenario green and the pair red, and the 2026-11-19 check names none | confirmed |
| A-3 | `TestRFCSelftestRealTreeRowReportsOwnedCheckFailures` timed out only from load | it hit its limit with no failing assertion printed, twice, at load 17 to 36 | a real failure or a cost defect is fixed here | AC-25 | unvalidated |
| A-4 | Each STALE unit still proves its requirement after the change that staled it | most causes are renames, lint fixes and fixture migrations | an auditor's `weak` or `wrong` verdict makes the test or producer owed work in this spec | AC-30, row by row | unvalidated |
| A-5 | Belongs to AC-27's design (kept here by D-1). The diagnostic contract's malformed-input freedom does not settle valid raw VPNv6 admission | "Raw Passthrough Commands" (`docs/architecture/api/update-syntax.md`) is silent on family negotiation; RFC 4659 Section 3.4 states no diagnostic exception | a sender capability defect is treated as an exclusion | the design review of AC-27 | unvalidated |
| A-6 | The producer a changed unit exercises today can be named for every record of AC-41, AC-42, AC-46 and AC-47 | `./le rfc discriminate` proposes candidates from the unit's package | the unit no longer reaches any producer of its claim: a finding on the test, to `ze-test-judge` | each AC's re-take | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A guest or interop record is run on a loaded host and times out | container or guest start timeout | run only when the owner says the host is quiet; never retry the same run hoping it passes |
| R-2 | An auditor re-judges a unit it changed | the auditor's session authored the cause commit | the main thread assigns each STALE stem to an agent with no part in its cause commits |
| R-3 | A re-take is written against a unit another session is still changing | `./le rfc check` names the row again after the record | take the record after that session's commit, and re-take it once |
| R-4 | Belongs to AC-27's design (kept here by D-1). AS width, next-hop refusal or blanket dropping hides the VPNv6 family subject | no useful VPNv6 or same-session IPv4 output even in the negotiated control | actual capability sets, matching AS_PATH width, a healthy third-party next hop and accepting policy |
| R-5 | A hand-written mutant is a break the claim does not name | the judge refuses the `break` text | the break is rewritten; nothing is committed before the judge reads it |
| R-6 | The stale set grows while the spec runs, from other sessions' uncommitted tagged-test edits | a new line in the next `./le rfc check` | the commit gate refuses such a commit without its record, so a new line is that session's; this spec re-reads the set at each phase start |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The public ledger shows a requirement as proven on a record that did not observe its red, or a verdict that no judge re-made |
| How is it reverted? | One commit per stem's records or verdicts, corrected by a fix commit |
| Who else touches this path? | Every session that changes a tagged test writes into the same `rfc/discrimination` and `rfc/audit` stems. Today: the IKE work holds uncommitted verdicts in rfc7296, the DNS work holds rfc2181, rfc7871 and rfc7535, and the next-hop work holds rfc7752 and rfc9552 |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le rfc check` | → | `verifyOneDiscrimination` in `internal/le/rfc/discriminate.go`: a producer edit leaves the proof standing, a unit edit stales it | `TestCheckDiscriminationProducerChangeLeavesTheProofStanding` |
| `./le commit create` | → | `judgeStaleRecords` in `internal/le/rfc/discriminate_commit.go` | `TestCommitGateIsRegistered` |
| `./le rfc selftest` path over the real tree | → | owned check failure reporting in `internal/le/rfc` | `TestRFCSelftestRealTreeRowReportsOwnedCheckFailures` |

## Acceptance Criteria

A row is met only with the commit that paid it and the 2026-11-19 check run
naming no line for it. Record ACs observe a red through `./le rfc record`.
Audit ACs are re-made by an independent `/ze-rfc-audit` agent and stamped with
`./le rfc audit-stamp stem <stem> from <pending> mode rejudge`. A met row that
`./le rfc check` names again before closure, because its unit changed again,
falls to AC-28, AC-29 or AC-30.

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Record RFC2328-4.4-3 positive, cause b5fdf9a87f | MET. The record verifies: re-taken in 5d5fb6d60c, and the check run lists no RFC 2328 record | None: met |
| AC-2 | Record RFC2328-D.3-1 positive, cause b5fdf9a87f | MET. The record verifies: re-taken in 5d5fb6d60c, and the check run lists no RFC 2328 record | None: met |
| AC-3 | Misnamed `internal/plugins/ospf/rfc5340_instance_enable_test.go`, cause b5fdf9a87f | MET. The file carries its descriptive name `instance_enable_test.go` since 58d47a9f96, and the check run lists no naming line for it | None: met |
| AC-4 | Interop record RFC2545-3-2 positive of `checkRFC2545NextHops`, cause c782411791 | MET. The record verifies: last written by 22f651cf53, and the check run lists no RFC 2545 record | None: met. The revert route the first draft named was refused by the recorder; the record stands on a hand-written mutant that keeps the route |
| AC-5 | Interop record RFC2545-3-3 positive, cause c782411791 | MET. The record verifies on the mutant route citing assertion 4 (`link_scope.go` `linkLocalNextHop`), first taken in 29f15481fa, last written by 22f651cf53 | None: met. Assertion 5 is cited by no record; AC-7 judges the claim against that |
| AC-6 | Interop record RFC2545-3-3 negative, cause c782411791 | MET. The record verifies on the mutant route citing assertion 6, with a paired break and both `solo` runs green; the check run lists no partial-scope line for RFC2545-3-3 | None: met |
| AC-7 | STALE audit RFC2545-3-3 | OPEN, inside AC-30. The verdict is re-made and stamped in stem rfc2545. The auditor also judges whether the interop positive claim's installation clause (assertion 5 of `checkRFC2545NextHops`) is proven by a record that cites assertion 4; a claim found too wide is narrowed through `ze-test-judge` and its record re-taken. Today's cause is a change to `TestSendAnnounceAppendsLinkLocalWhenSection3Holds` and its sibling | The three other STALE rfc2545 verdicts (3-1, 3-4, 3-6) stay until AC-30 takes the stem in the same audit |
| AC-8 | STALE audit RFC4659-3.2.1.1-3 | OPEN, inside AC-30. The verdict is re-made and stamped in stem rfc4659; the unit that changed is `forward_next_hop.go` `applyEgressNextHopScope` | The three other STALE rfc4659 verdicts stay until the same audit; RFC4659-3.4-1's re-judge stays bounded to its tested entry points and does not prove the raw API (AC-27) |
| AC-9 | STALE audit RFC1661-5.1-3, cause 1f4e1e81f1 | MET. The verdict is fresh: last stamped in c2dbc32f04, and the check run lists no line for it | None: met |
| AC-10 | STALE audit RFC1661-5.2-3 | MET. Last stamped in c2dbc32f04; the check run lists no line for it | None: met |
| AC-11 | STALE audit RFC1661-5.2-4 | MET. Last stamped in c2dbc32f04; the check run lists no line for it | None: met |
| AC-12 | STALE audit RFC1661-5.3-7 | MET. Last stamped in c2dbc32f04; the check run lists no line for it | None: met |
| AC-13 | STALE audit RFC1661-5.4-3 | MET. Last stamped in c2dbc32f04; the check run lists no line for it | None: met |
| AC-14 | STALE audit RFC1661-5.4-4 | MET. Last stamped in c2dbc32f04; the check run lists no line for it | None: met |
| AC-15 | STALE audit RFC1661-5.4-5 | MET. Last stamped in c2dbc32f04; the check run lists no line for it | None: met |
| AC-16 | STALE audit RFC1661-5.8-2 | OPEN, inside AC-30. The verdict is re-made and stamped in stem rfc1661; the unit that changed is `internal/component/l2tp/ppp/session_run.go` `run` | The three other STALE rfc1661 verdicts (4.3-1, 4.6-3, 4.6-4) stay until the same audit |
| AC-17 | STALE audit RFC1661-6.4-7 | MET. Last stamped in c2dbc32f04; the check run lists no line for it | None: met |
| AC-18 | STALE audit RFC2516-x-9, cause 8c68fcf514 | MET. The verdict is fresh: last stamped in 68d21a1c7f, and the check run lists no line for it | None: met. RFC2516-5.4-3 and RFC2516-7-4 are other rows, in AC-30 |
| AC-19 | STALE audit RFC4271-5.1.4-1, cause 8f704db860 | MET. Last stamped in 68d21a1c7f; the check run lists no line for it | None: met |
| AC-20 | STALE audit RFC4271-5.1.4-2, cause f23e06435c | MET. Re-judged in daab5549b7; the check run lists no line for it | None: met |
| AC-21 | STALE audit RFC4271-8.2.2-13, cause 898caba3f6 | MET. Re-judged with its record in 86b381c3b4; the check run lists no line for it | None: met |
| AC-22 | STALE audit RFC8907-8.3-4, cause c44e1bdc8c | MET. Last stamped in 22f651cf53; the check run lists no line for it | None: met |
| AC-23 | STALE audits RFC2328-4.4-3 and RFC2328-D.3-1, cause b5fdf9a87f | MET. Both re-judged in d887e713bf; the check run lists no RFC 2328 line | None: met |
| AC-24 | SHIFTED audit RFC1997-Well-1, cause c782411791 | MET. Resealed, last in 68d21a1c7f; the check run lists no RFC 1997 line | None: met |
| AC-25 | `TestRFCSelftestRealTreeRowReportsOwnedCheckFailures`, which hit its limit under load | OPEN. Run alone on a quiet host with a 30-minute limit. A pass inside the test's own limit records the earlier red as load and confirms A-3; a failing assertion is a defect fixed at its producer; a run that still exceeds the limit alone is a defect in the test's cost, fixed so it completes without dropping what it asserts | A pass that needs the longer limit is not a pass of the test as written: it is the cost defect |
| AC-26 | `TestPriorEvidenceReadsTheNewestCommitHoldingTheBacklog`, failing deterministically | MET. Fixed in c5b52ce3f1 (`check_evidence_prior.go`), whose body records five failing assertions before the change and a pass after it, test unchanged | None: met |
| AC-27 | DESIGN, approval owed (kept here by D-1). Explicit raw UPDATE body and full forms carrying valid labeled VPNv6 NLRI, with and without negotiated AFI 2 / SAFI 128 | Not implemented before its design is presented and approved. The finding, its probes and its constraints are in "Raw VPNv6 family-admission finding". Once approved and built, a negotiated peer receives the exact VPNv6 route, a peer that negotiated only IPv4 still receives a useful IPv4 route on the same session, and the planned interop scenario `bgp-raw-vpnv6-family-admission-frr` shows both against FRR | Whatever the approved design permits |
| AC-28 | Every discrimination record `./le rfc check` names as unit-changed or claim-changed (32 on 2026-11-19, classes AC-40 to AC-47) | Each record is re-taken once by `./le rfc record` against the unit as it is and observed red; the check lists every one as verified and names no stale record outside AC-33 | A record another spec owns under AC-33 stays with that spec |
| AC-29 | Every SHIFTED verdict `./le rfc check` names: 39 in 11 stems (rfc4555 12, rfc7296 5, rfc2661 4, rfc2866 4, rfc7854 4, rfc9069 4, rfc5176 2, rfc2865 1, rfc3948 1, rfc8671 1, rfc8955 1) | Each stem is resealed with `./le rfc reseal stem <stem> source head` and its verdicts read fresh in the check | rfc7296 waits until the IKE session commits its two uncommitted verdict files there; a stem is never resealed over another session's uncommitted evidence |
| AC-30 | Every STALE verdict `./le rfc check` names: 165 in 47 stems (rfc4271 38, rfc5176 24, the rest 5 or fewer each) | Each is re-made by an independent `/ze-rfc-audit` agent (R-2), one agent per stem, after that stem's records, and stamped with `mode rejudge`; the check lists every one as fresh. A `weak` or `wrong` verdict sends its test or producer back to be fixed here | A verdict in a stem another session holds uncommitted waits for that commit; a verdict of a row AC-33 leaves to another spec stays with it |
| AC-31 | Records whose producer was removed | NOT OWED. A proof belongs to its test, so `./le rfc check` reports no producer-gone state (6db7cb8964). A record whose unit also changed is in AC-40 | None: nothing is owed |
| AC-32 | RFC4271-5-3 and RFC4271-5-4 carry no negative test since 6c22530c23 moved four negative tags; that commit owes three judge verdicts. RFC9252-3.3-2 partial scope has no current verified break | RFC 4271: one negative test per row that takes the case just outside the clause, tagged, recorded red by `./le rfc record`; a `ze-test-judge` verdict for each of `storage.TestRFC4271PartialBitSurvivesLengthReframing`, `reactor.TestRFC4271PartialNotSetOnRecognizedOrNonTransitive` and `attribute.TestRFC4271PartialNotStampedOnExcludedClasses`, carried as `RFC-approved:` lines. RFC 9252: both records of `TestRFC9252EffectiveNextHopControlsServiceTLVs` re-taken (AC-45), which clears both partial-scope lines. RFC8955-4-3 is met: its record was re-taken in 6bbd7fd8e2 and only its verdict remains, in AC-30 | `{single-polarity}` is not used where a negative exists. A judge refusal stands, and the test it refuses is corrected at the code or the tag |
| AC-33 | Rows another spec or session owns | Each is cleared by its owner and listed fresh by the check at this spec's closure: RFC7752-3.4-1 and RFC9552-5.5-1 (`spec-bgp-next-hop-auto-rewrites-for-ebgp.md` AC-12, in-progress); the rfc2181, rfc7871 and rfc7535 lines (the DNS resolver work, uncommitted, `plan/handover/51-dns-resolver-scope.md`); the `spd_forward_integration_linux_test.go` naming marker (`spec-rfc4301-architecture-gaps.md`, 0c283fffc6) | A row whose owning spec closes without clearing it falls to this spec. An uncommitted row of a live session is never edited here |
| AC-34 | Closure | `./le rfc check` at HEAD lists every record as verified and every verdict as fresh outside AC-33, and the commit-time refusal is registered (`TestCommitGateIsRegistered` passes) | The AC-33 rows, named in the closing commit with their owners |
| AC-35 | DESIGN, approval owed (kept here by D-2), formerly DA-1. The complete RFC 7606 Section 8 and RFC 5492 Section 4 sentences | Each can represent its document obligation through the reviewed evidence route | Every other speaker obligation keeps the existing refusals: no arbitrary obligation gains that route |
| AC-36 | DESIGN, approval owed (kept here by D-2), formerly DA-2. The restrictive-subject and passive-document forms | Tests in `internal/le/rfc/notapplicable_roles_test.go` cover each form separately and pass | Nearby receiver obligations stay applicable, RFC 5492 Section 4's acceptance of multiple Capabilities Optional Parameters included |
| AC-37 | DESIGN, approval owed (kept here by D-2), formerly DA-3. A missing source, wrong section, incomplete sentence, dropped condition, borrowed obligation, altered level, generic actor or role Ze fills | Tests in `internal/le/rfc/check_evidence_test.go` show each refused, and each new guard has an observed fault that makes its test fail | Existing valid role evidence stays accepted |
| AC-38 | DESIGN, approval owed (kept here by D-2), formerly DA-4. Rows RFC7606-8-1 and RFC5492-4-3 | An independent RFC auditor reads the defining sentences and Ze's roles, checks the historical owner ruling, and records the disposition through the native writers | Ids and MUST levels are kept; no whole-RFC completion or runtime behaviour is inferred |
| AC-39 | DESIGN, approval owed (kept here by D-2), formerly DA-5. "The exclusion evidence ratchet" in `docs/contributing/rfc-conformance-gates.md` | The page states the admitted evidence form and its refusal cases, in the same change as the code | The decided shapes it already documents stay as written |
| AC-40 | 9 revert records of `TestRFC4861SubscriberSolicitationReaderIgnoresReservedAndOptions` (RFC4861-4.1-1, 4.1-3, 6.1.1-3, 6.1.1-4, 9-1, 9-3, 9-4, 9-5, 9-8). The recorded producer file `internal/component/l2tp/ppp/ra_linux.go` is gone and the unit is Linux-only | Each is re-taken on the revert route against `internal/component/l2tp/ppp/ra_carrier.go::rsReaderLoop`, where the function lives now, in a Linux guest, and observed red | The sibling records of the `internal/plugins/iface/ra` unit verify and are not touched |
| AC-41 | 4 revert records of RFC8277-2.4-1 (`TestVPNWithdrawalLeavesExactReflectorInventory`, `TestLabeledWithdrawalLeavesTheWithdrawalMap`, both polarities). The changed tests no longer execute the recorded producers `walkVPNNLRIs` and `wrapNLRI` | The producer each test exercises today is named from `./le rfc discriminate` and the test body, and each record is re-taken against it and observed red | If a test reaches no producer of its claim, that is a finding on the test, corrected through `ze-test-judge`; the recorded producers are not restored to make the old record pass |
| AC-42 | Revert record RFC5176-3.1-1 negative of `TestRFC5176ProxyStateContentIsNeverRead`, green under a whole-function halt of `handleDisconnect` | The record is re-taken on a one-line mutant that is the violation the claim names (the session lookup reads the Proxy-State content), read by a judge before commit, and observed red | A halt of `handleDisconnect` leaves the unknown session without a teardown either way, so its green is not by itself a test defect. If no producer-side break of the claim reddens the test, the test is wrong about what it asserts and is corrected through `ze-test-judge` |
| AC-43 | 9 revert records over 4 FlowSpec units: RFC5575-4-8 (2) and RFC8955-4-4 (2) of `TestRFC8955NextHopIgnoredForFlowSpec`; RFC8955-4.2-3 (5) of `TestFlowSpecUnmatchableByValueReceivedRuleIsNotPropagated`, `TestFlowSpecUnmatchableReceivedRuleIsNotPropagated` and `TestFlowSpecUnmatchableReceivedRuleWithdraw`. A whole-function halt did not finish in 15 minutes | Each is re-taken on a one-line mutant in the recorded producer, read by a judge before commit, and observed red inside the recorder's limit | A unit that also hangs under a one-line break has a wait with no bound: a test defect corrected through `ze-test-judge`, never a longer timeout |
| AC-44 | Revert record RFC4659-3.2.1.1-1 positive of `TestRFC4659ConfiguredIPv6TransportNextHopWire`, Linux-only and capability-gated | Re-taken on its recorded route in a Linux guest with the kernel named, and observed red | None: one record |
| AC-45 | 6 revert records not in any refused class of handover 49: RFC9252-3.3-2 (2, `TestRFC9252EffectiveNextHopControlsServiceTLVs`), RFC9252-5-1 (3, `TestIsSRv6Ineligible_InvalidSIDStructure`, `TestSRv6TranspositionWiderThanLabelFieldIsIneligible`, `TestSRv6TranspositionAtLabelWidthIsEligible`), RFC8955-6-5 negative (`TestRFC8955FlowSpecLosingPathIsNeverPublished`) | Each is re-taken on its recorded route and observed red | A record the recorded route refuses moves to the class its refusal matches (AC-41 to AC-43), with the refusal recorded here |
| AC-46 | Mutant record RFC5176-2.3-6 positive of `TestRFC5176CoANAKMakesNoPartialChange`, green under its recorded break | A break that is the violation the claim names (a NAK that leaves a partial change) is written against today's producer, read by a judge, and observed red | If no such break reddens the test, the test no longer checks its claim and is corrected through `ze-test-judge` |
| AC-47 | Mutant record RFC7606-5.1-1 negative of `TestForwardLeadingMPAttributeKeepsZeroCopy`, red under its break without naming the unit | The cause of the unnamed red is found (the break fails the package before the unit runs, or another test), and the record is re-taken on a break under which the unit itself is the named red | The record is not written from a red that does not name the unit: the recorder's refusal stands |
| AC-48 | Three recorded and uncommitted RFC 7606 entries: RFC7606-5.1-1 (two `TestForwardMovesTrailingMPAttributeFirst*` units) and RFC7606-5.1-3 (`.ci`), on hand-written breaks | A judge reads each `break` text and the entries are committed with a body that says the breaks were written by hand; the check lists them as verified | A break the judge refuses is rewritten and re-recorded |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCheckDiscriminationProducerChangeLeavesTheProofStanding` | `internal/le/rfc/check_test.go` | the staleness rule this spec runs under | |
| `TestCommitGateIsRegistered` | `internal/le/rfc/discriminate_commit_test.go` | the commit-time refusal is wired (AC-34) | |
| `TestRFCSelftestRealTreeRowReportsOwnedCheckFailures` | `internal/le/rfc/selftest_test.go` | the selftest reports owned check failures over the real tree (AC-25) | |
| RFC 4271 Section 5 negative for 5-3 | the package holding its positive, named by the phase that writes it | an attribute outside the clause does not get the Partial handling the clause requires (AC-32) | |
| RFC 4271 Section 5 negative for 5-4 | the package holding its positive, named by the phase that writes it | the case just outside the MUST NOT is permitted (AC-32) | |
| Every tagged unit named in AC-40 to AC-48 | as named | its record's observed red | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | no numeric input is added | N-A | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| RFC7606-5.1-3 carrier | the `.ci` its record names | its hand-written break is judged and committed (AC-48); the test is not changed | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-rfc2545-linklocal-nexthop-frr` | `test/interop/scenarios/` | FRR | Met (AC-4 to AC-6): its records verify and no run is owed. The proof-debt ACs change no wire-visible behaviour | |
| `bgp-raw-vpnv6-family-admission-frr` (planned) | `test/interop/scenarios/` | FRR | AC-27, once its design is approved: a raw VPNv6 UPDATE reaches FRR only over a session that negotiated AFI 2 / SAFI 128, and an IPv4-only session still carries a useful IPv4 route. FRR's receive handling of an unnegotiated family is read in its source before the check is written | |

## Files to Modify
- `rfc/discrimination/<stem>/` for the stems of AC-40 to AC-48 (rfc4861, rfc8277, rfc5176, rfc5575, rfc8955, rfc4659, rfc9252, rfc7606) and of the AC-32 negatives (rfc4271) - written only by `./le rfc record`
- `rfc/audit/<stem>/` for the 47 stems of AC-30 and the 11 stems of AC-29 - written only by `./le rfc audit-stamp` and `./le rfc reseal`
- The RFC 4271 negative tests of AC-32, in the packages holding the 5-3 and 5-4 positives (`internal/component/bgp/plugins/rib/storage/`, `internal/component/bgp/reactor/`, `internal/core/bgp/attribute/`)
- `internal/le/rfc/selftest_test.go` or the code it drives - only if AC-25 finds a cost defect
- Any test a judge shows wrong (AC-41, AC-42, AC-43, AC-46) and any producer an audit or record shows defective, named in the commit that fixes it

## Files to Create
- None planned. A negative test of AC-32 goes in an existing test file of its package unless the package's naming rule needs a new one

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no config |
| YANG validation constraints | N-A | no config |
| YANG custom validators | N-A | no config |
| CLI commands/flags | N-A | no command changes |
| CLI grammar (keyword before value) | N-A | no command changes |
| Editor autocomplete | N-A | no command changes |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | no output |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | none |
| Doctor code per problem class | N-A | no refusal, log line or event is added |
| Prometheus counters/metrics | N-A | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | none in the work this spec keeps; AC-27 owes it where it lands |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | proofs are restored, not widened |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes, if an audit verdict changes level or AC-32 restores a polarity | `rfc/short/<stem>.md` where a verdict moves a row; the derived pages regenerate |
| 10 | Test infrastructure changed? | Yes, only if AC-25 changes the selftest | `docs/contributing/rfc-conformance-gates.md` |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Derived | `./le spec citation anchors spec plan/release/fixes/spec-rfc-proof-debt-outside-filter-wire.md`, run by the main thread once a phase edits a test or a producer |
| 17 | Existing docs show config/CLI/API examples for this area? | No | - |

## Implementation Steps

Every record, package run and guest run is the main thread's, one CPU job at a
time. An agent prepares the commands and lists them as owed. Each stem's
records and each stem's verdicts land in a commit of their own. Each phase
starts by re-reading the live set from a fresh `./le rfc check`.

1. **Phase: Wiring (MANDATORY FIRST)** -- confirm the gates the work relies on, and settle AC-25
   - Tests: `TestCheckDiscriminationProducerChangeLeavesTheProofStanding`, `TestCommitGateIsRegistered`, `TestRFCSelftestRealTreeRowReportsOwnedCheckFailures`
   - Needs: a quiet host for the selftest run (the owner says when)
   - Verify: the first two pass; the third run alone is classified as AC-25 states
2. **Phase: Reseal** -- AC-29, 39 verdicts in 11 stems, mechanical, runs no test
   - Needs: rfc7296 waits for the IKE session's commit
   - Verify: `./le rfc check` names no SHIFTED verdict in the stems resealed
3. **Phase: Pending judged entries** -- AC-48, 3 entries
   - Needs: one `ze-test-judge` agent that did not write the breaks
   - Verify: the judge's verdict on each `break` text; the entries committed; the check lists them as verified
4. **Phase: Host re-takes** -- AC-41 (4), AC-42 (1), AC-43 (9), AC-45 (6), AC-46 (1), AC-47 (1): 22 records
   - Order: AC-45 first (recorded route, no design), then the classes that need a new producer or a hand-written break
   - Needs: `ze-test-judge` for every hand-written break (AC-42, AC-43, AC-46, and AC-47 if its break changes) and for any test a finding changes
   - Verify: each record observed red by `./le rfc record`; per stem, `./le rfc check` names none of that stem's stale records
5. **Phase: Guest re-takes** -- AC-40 (9) and AC-44 (1): 10 records
   - Needs: a Linux guest with the kernel named, on a quiet host
   - Verify: each record observed red in the guest; the check names no rfc4861 or rfc4659 stale record
6. **Phase: RFC 4271 negatives** -- AC-32
   - Tests: the two negatives, written to fail first against a break of the clause
   - Needs: `ze-test-judge` for the three owed verdicts
   - Verify: both negatives recorded red; the check names neither row's polarity line; three `RFC-approved:` lines in the commit
7. **Phase: Audits** -- AC-30 with AC-7, AC-8 and AC-16: 165 verdicts in 47 stems
   - Order: a stem is audited after its records in phases 3 to 6; rfc4271 (38) and rfc5176 (24) are the two large stems
   - Needs: one independent `/ze-rfc-audit` agent per stem, with no part in the stem's cause commits (R-2)
   - Verify: stamped with `mode rejudge`; `./le rfc check` names no STALE verdict in the stem; a `weak` or `wrong` verdict goes back to phase 4 as a fix
8. **Phase: Closure check** -- AC-33 and AC-34
   - Verify: a final `./le rfc check` pasted in the closing commit, every remaining line matched to its AC-33 owner

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every open AC is cleared by name in a `./le rfc check` run pasted in the closing commit; every met AC still reads met in that run |
| Correctness | No record, verdict or fingerprint file was edited by hand |
| Independence | No STALE row was re-judged by the author of its cause commit; no hand-written break was judged by its author |
| Route honesty | Every record that left its recorded route says why in its commit body (producer moved, halt hangs, halt is not the violation) |
| Rule: `ai/rules/testing.md` | No tag, claim or assertion was weakened to obtain a red or a verdict |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| 32 stale records re-taken (AC-40 to AC-47) and 3 pending entries committed (AC-48) | `./le rfc check` names no `no longer verifies` line outside AC-33 |
| 39 SHIFTED verdicts resealed | `./le rfc check` names no SHIFTED verdict |
| 165 STALE verdicts re-made | `./le rfc check` names no STALE verdict outside AC-33 |
| Two RFC 4271 negatives and three judge verdicts | the check names no polarity line for RFC4271-5-3 or RFC4271-5-4; `git log` shows the three `RFC-approved:` lines |
| The selftest classified | the AC-25 run's log |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | N-A: tooling over the repository's own files |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup, through `ze-test-judge` |
| Record observes no red | The break is not the violation the claim names, or the unit no longer discriminates: a new break read by a judge, then a test or producer fix in this spec |
| Record red does not name the unit | Find what failed first (AC-47); never record it |
| Guest or interop run times out | Load, not a verdict: wait for a quiet host |
| Audit verdict weak or wrong | Fix the test or the producer in this spec |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The refused re-takes are not one class. A record whose unit changed is re-proved against what the unit exercises today, and "the recorded route" is only the first attempt: a producer that moved, a halt that hangs and a halt that is not the claim's violation each need a different route, so each has its own acceptance criterion.
- The live set moves under the spec. Rows the first draft listed one by one were mostly paid by other sessions' commits before this spec ran, and new rows appeared from their work. The ACs are therefore written as classes settled against `./le rfc check`, with the snapshot counts recorded only to size the phases.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `guard-demands-what-the-model-cannot-supply.md` | 2026-11-15 | RFC document-author evidence cannot pass the role-binding model |
| `guard-added-to-one-half-of-a-pair.md` | 2026-11-15 | Explicit raw UPDATE VPNv6 family admission |

Both rows stay owned here (D-1 and D-2 kept their items in this spec): the
first closes with AC-35 to AC-39, the second with AC-27.

## Raw VPNv6 family-admission finding (source only, 2026-11-15) -- design owed here (D-1)

The authoritative sentence read in `rfc/full/rfc4659.txt`, Section 3.4:
"In order for two PEs to exchange labeled IPv6 VPN NLRIs, they MUST use BGP
Capabilities Negotiation to ensure that they both are capable of properly
processing such NLRIs." Its next sentence specifies capability code 1 and
the AFI/SAFI from Section 3.2. Diagnostic intent supplies no exception in
that sentence. An unsettled raw-operation boundary is a research question,
not a waiver or a reason to exclude this implemented sender capability.

| Producer read | Current source evidence | Scope of the finding |
|---------------|-------------------------|----------------------|
| `internal/component/bgp/reactor/session_write.go`, `Session.SendRawMessage`; `session_aigp.go`, `writeOriginatedRawUpdate` | Recognized UPDATE body and full-packet forms reach `writeUpdateBody` with `raw=true`. A healthy third-party next hop with no AIGP reaches it directly. | Both explicit raw forms need the probe below. Use valid framing, negotiated-width AS_PATH and an accepting policy so another guard cannot decide the result. |
| `internal/component/bgp/reactor/session_ownership.go`, `adjOutWrite.section` | `SupportsFamily` admission is conditional on the write not being raw; a registered VPNv6 splitter can account for its NLRI without adding a capability check. | Source evidence permits a valid raw VPNv6 announcement toward a recipient that negotiated only IPv4. No wire counterexample has been observed. |
| `internal/component/bgp/reactor/session_write.go`, `writeUpdateBody` | Managed output applies the ownership filter results. Raw output only observes them and retains its body. | Generic `writeMessageHeld` passes `raw=false` and therefore has family admission; it is a useful control, not another claimed family bypass. |

Earlier rulings on the same function, which a design for this finding must be
checked against (`writeUpdateBody`, read 2026-11-19):

| Rule | What the code does for a raw write | Recorded source |
|------|------------------------------------|-----------------|
| RFC 8212 Section 3 | Raw does not waive export admission: denied routes are turned into withdrawals on every rail | the comment above the policy step in `writeUpdateBody`, and `SendRawMessage` |
| RFC 7607 Section 2 | No override: AS 0 is refused on every rail, explicit raw injection included | the comment above the AS-zero step |
| RFC 9774 Section 3 | Raw is exempt: it is the operator override | owner ruling Q-4 (b), 2026-10-18, in the comment above the AS_SET step |
| RFC 4271 Section 4.3, RFC 8092 Section 3, RFC 8669 Section 3.1, RFC 8277 Section 2.4 normalization | Applied on every write, the raw diagnostic one included | owner ruling Q-3 (a), 2026-10-14, in the comment above the normalization step |

The operator page `docs/architecture/api/update-syntax.md`, "Raw Passthrough
Commands", lists the guards that still apply to raw (the RFC 4271 Section
5.1.3 own-address refusal, AIGP origination policy, PATHS-LIMIT filtering,
Label-Index normalization) and is silent on family negotiation. The design
decides which of the precedents family admission follows, and corrects that
page in the same work.

### Minimal TDD probe owed before a design is accepted

| Working test name | File / entry point | Input, discriminator and useful controls |
|-------------------|--------------------|-----------------------------------------|
| `TestRawVPNv6RequiresNegotiatedFamily` | New descriptive test in `internal/component/bgp/reactor/`; actual `Session.SendRawMessage`, both body and full-packet forms | Use valid registered labeled VPNv6 NLRI, a healthy third-party next hop, accepting final policy and nonzero AS_PATH matching the session width. Compare a real AFI 2 / SAFI 128 negotiation with actual IPv4-only negotiation. Read final frames: negotiated VPNv6 emits the exact useful route; the IPv4-only peer emits no VPNv6 route. A successful useful IPv4 announcement on that same session proves ordinary output remains available. Preserve the six existing OPEN/negotiation/configured-send/receive subjects. |
| `TestGenericVPNv6FamilyAdmissionControl` | The same test carrier; generic `writeMessage` with `raw=false` | With the same valid route and capability sets, confirm the existing final family gate is reached and the useful IPv4 control survives. This distinguishes raw bypass from a broken fixture or unsupported splitter. |

These are research probes, not an approved implementation approach. No
probe, build, proof, audit stamp or ledger update ran for this finding. This
spec owes the design review (FRR's and BIRD's source read for
what each sends to a peer that did not negotiate the family), the repair of
any confirmed defect, a wire or interop test, and an independent whole-row
judgment of RFC4659-3.4-1. The managed six-unit evidence stays bounded to its
tested entry points; it does not prove the raw API.

## Document-author evidence finding -- design owed here (D-2)

The RFC test campaign completed this investigation on 2026-11-15. It changes
no summary, extraction site, audit verdict, requirement level or support claim.

| Source | Full obligation | Current evidence defect |
|--------|-----------------|-------------------------|
| RFC 7606 Section 8, `RFC7606-8-1` | "A document that specifies a new BGP attribute MUST provide specifics regarding what constitutes an error for that attribute and how that error is to be handled." | The summary records a gap. Its audit names `message/rfc7606.go::validateAttribute`, although that dispatcher cannot prove what the defining document specifies. The audit records an earlier owner-invalidated exclusion; that history is not authority for restoring it. |
| RFC 5492 Section 4, `RFC5492-4-3` | "Processing of these capability instances is specific to the Capability Code and MUST be described in the document introducing the new capability." | The summary records a gap and no row audit exists. Extraction site `4:2` still says that the row is not applicable. That reason disagrees with the current summary and cannot authorize an exclusion. |

### Cause and correction boundary

`internal/le/rfc/notapplicable_roles.go::boundRoleRefusal` takes the text before
the first BCP 14 keyword as the subject. `subjectRefusal` rejects a subject
containing a `subjectBreakers` word. RFC 7606's restrictive subject contains
`that`. RFC 5492's pre-MUST text contains `of` and `and`; its defining document
appears after MUST in a passive complement. Neither section supplies a separate
scoping sentence accepted by the current model.

`docs/contributing/rfc-conformance-gates.md`, "The exclusion evidence ratchet",
explicitly calls these shapes decided rules. Its suggested gap-and-test route
does not resolve these document obligations: a decoder assertion can prove a
receiver's behavior, but cannot prove what another document contains. The
correction must change that contract through review, not force either sentence
into a runtime requirement or bypass the source checks.

| Existing mechanism | Treatment in the correction |
|--------------------|-----------------------------|
| `boundRoleRefusal` and `subjectRefusal` | Review a narrow document-author form against a general clause parser. Prefer the smallest source-bound representation that preserves the complete condition; do not remove breaker words globally. No alternative is approved here. |
| `check_evidence.go::evidenceReader.roleQuoteGap` and `ownObligationRefusal` | Preserve complete-sentence, cited-section, own-clause and original-level checks. A title, borrowed clause or shortened quote remains insufficient. |
| Native extraction and audit writers | Independently rejudge both rows after the reviewed correction. Reconcile summary, extraction reason and audit together; retain permanent ids and MUST levels. Do not create runtime discrimination records for document content. |

The acceptance criteria for the reviewed correction are AC-35 to AC-39.
Until the design and independent rejudgment are complete, both rows remain
disclosed findings. No decoder test is invented to clear them. The repair needs
no protocol, SDK, schema, daemon command or wire-format change.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One acceptance criterion per class of refused record (AC-40 to AC-47) | One "recorded route" criterion for all (the first draft's AC-28) | The recorder already refused the recorded route for 30 of them, for five different reasons; a single criterion could only be met by a hand-written record, which the gate forbids |
| RFC5176-3.1-1 is re-taken on a mutant before the test is called wrong | Treat the green under the halt as a test defect | The test asserts a NAK and no teardown for an unknown session, and a halted handler also tears nothing down, so the halt is not the violation the claim names; the test is judged wrong only if no break of the claim reddens it |
| AC-4 to AC-6 no longer wait on `spec-rfc2545-next-hop-truncated-to-sixteen-bytes.md` | Keep the wait | The wait was for a producer change, which stales nothing under the owner rule, and the records verify today |
| Rows of live sessions are left to them (AC-33) | Clear every line the check prints | A row another session holds uncommitted is not this spec's to edit, and the commit gate makes that session carry its own record |
| The two design items stay here and the spec returns to `design` (owner, D-1 and D-2, 2026-11-21) | Move AC-27 to the BGP MP-attribute wire-proof spec and AC-35 to AC-39 to the evidence-strength spec, keeping this spec `ready` | The owner's ruling. The proof-debt ACs need no design and keep running; the two design items wait for approval |
| RFC8212-3-1 interop record left to the filter-wire umbrella | listing it here as first briefed | its producer moved in 2fd7958d6f and fa54cd4285, both filter-wire commits |

## Known Limitations
- This spec does not drop `producer-sha` from the records. It is still written and validated as provenance and no verdict reads it; removing it rewrites about 2,700 files and is an owner decision not taken.
- The 171 lines of the DNS work and the IKE naming marker are not classified further here: each is cleared by the session that owns its cause (AC-33).

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
- [ ] AC-1..AC-48 all demonstrated, met rows re-confirmed by the closing check; AC-27 and AC-35..AC-39 demonstrated here once their designs are approved (D-1 and D-2 kept them)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests: N-A, no numeric input
- [ ] Interop tests: `bgp-raw-vpnv6-family-admission-frr` for AC-27, proven red with the fix reverted and the image rebuilt; N-A for the proof-debt ACs, no wire-visible change

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** records + verdicts + code + edited spec
- [ ] **Commit B:** `remove plan/release/fixes/spec-rfc-proof-debt-outside-filter-wire.md` only
