# Spec: rfcgate-2-deferred-nonunit-evidence-backfill

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 1/4 |
| Deferral shard | `plan/deferrals/rfcgate-2-deferred-nonunit-evidence-backfill.md` |
| Updated | 2026-08-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Deferred out of `plan/spec-rfcgate-2-evidence.md` (see
`plan/deferrals/rfcgate-2-evidence.md`, row 4).

**The problem.** `plan/spec-rfcgate-2-evidence.md` built the machinery for
non-unit RFC evidence: four declared carriers, an execution tier per carrier, a
`kind/tier` cell on every ledger link, and a per-tier monotonic ratchet. It did
not re-bind the existing corpus. `ai/rules/completion.md` states
that a unit test proves the algorithm while only a functional test proves the
feature is reachable. A wire obligation proven only by a Go table test is proven
at the wrong altitude, which is the thesis the parent spec exists to act on.

**Why this spec exists rather than a row on the umbrella.**
`ai/rules/testing.md` "Back-Fill New Test Types" obliges the session introducing
a new evidence kind to name the applicable set and either back-fill it or record
the remainder as tracked backlog. Umbrella decision D4 says "This spec set is
MACHINERY ONLY", and Constraint(D4) adds "no child may open the backlog". A
destination that disclaims the work is not a home
(`ai/rules/planning.md`), so the row moved here.

**Sizing input, and a hard constraint on how it may be used.** A keyword
classifier over requirement text, hand-calibrated on a 40-item sample at ~97%
precision and poor recall, puts **at least 76%** of the gated MUSTs in the
wire-visible class. That number sizes the problem and nothing else. The parent
spec's A-4 forbids deriving any pass/fail decision from it. **A gate built on the
classifier would manufacture obligations for every mis-classified requirement.
Do not build one.**

**What stays unit-only on purpose.** Requirements that are genuinely internal,
with no wire form, keep unit-only evidence and are correct as they stand. The
goal is not "every requirement gets a `.ci`". It is "every requirement whose
obligation is about bytes on a wire is proven by something that puts bytes on a
wire, and judged by an oracle that is not the code under test".

### Measured distribution (2026-08-02, this session)

The spec previously quoted `unit/verify 2571`. That figure is a TAG count taken
at the parent spec's closure. The requirement count is different and is the
number that matters. Measured by importing `internal/le/rfc/rfc.go` and
folding `carrier_for` over every tag, WITHOUT rendering `ai/RFC-REQUIREMENTS.md`
(three concurrent sessions own that file this week):

| Population | Count |
|------------|-------|
| Gated MUST-level requirements in enrolled RFCs | 2922 |
| Proven ONLY by unit tests | 1536 |
| Holding any non-unit evidence | 16 |
| Carrying no tag at all (annotated `{gap}` / `{not-applicable}`) | 1370 |

Reachability, derived from the directory each requirement's unit tags live in,
mapped against the suite list `internal/le/functional/suites.go` `all_suites` actually
names. A `.ci` outside that list earns `TIER_UNRUN` and the scanner REFUSES it,
so "could a `.ci` prove this today" is a real constraint, not a preference:

| Bucket | Unit-only requirements |
|--------|------------------------|
| Subsystem a runnable verify-tier suite already boots | 1185 |
| No runnable suite exists at all | 242 |
| Miscellaneous packages, no single owning suite | 109 |

The 242 with no runnable suite are: BFD 98, VRRP 80, dhcpserver 28, geodns 18,
dnsserver 18. There is no `bfd`, `vrrp`, or `dhcp` suite in `all_suites`, so
these cannot be backfilled at verify tier without first adding a suite. That is
an owner question, recorded below, and not a `{gap}` this spec writes.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/completion.md` - the altitude argument this spec acts on
  → Constraint: a unit test proves the algorithm, a `.ci` proves the daemon exposes it. Both are required; neither substitutes.
- [ ] `ai/rules/interop-and-goal-validation.md` - the vacuity traps
  → Constraint: a test asserting the ABSENCE of something passes when the mechanism is deleted. Every binding this spec lands must be a POSITIVE assertion.
- [ ] `ai/rules/rfc-compliance.md` - what to do with a requirement that cannot be proven
  → Decision: a requirement that is unprovable at any tier is an owner question. This spec writes no `{gap}`.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2661.md` - the tranche's RFC
  → Constraint: RFC2661-4.1-2 puts Message Type first in EVERY control message; RFC2661-4.1-1 clears AVP reserved bits 2-5 on send; RFC2661-x-1 clears header reserved bits 8-11.
  → Constraint: §4.2 tunnel authentication is `response = MD5(message-type-byte || shared-secret || challenge)`. The requirement id RFC2661-4.2-1 is a MAY, so it is not gated and cannot carry a binding.

**Key insights:**
- The discriminator is not "unit versus functional". It is whether the test's expected value is re-derived from the RFC or produced by the code under test's own dependency.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/rfc/rfc.go` - `CARRIERS`, `carrier_for`, `functional_suites`; the evidence kind and tier model, and the refusal of a tag in a suite nothing runs
- [ ] `internal/component/l2tp/auth.go` - `ChallengeResponse` and `VerifyChallengeResponse`; the verifier CALLS the producer, so the two cannot disagree by construction
- [ ] `internal/component/l2tp/tunnel_fsm.go` - `writeSCCRPBody`, `parseSCCRQ`; the SCCRP AVP write order and the SCCRQ validation
- [ ] `internal/component/l2tp/avp.go` - `WriteAVPHeader`; the AVP first word, where reserved bits 2-5 live
- [ ] `internal/component/l2tp/header.go` - `controlFlagsFixed = 0xC802`; the control header ze emits
- [ ] `internal/component/l2tp/reactor.go` - `handlePacket`; a TunnelID=0 datagram whose body fails to parse is logged at Debug and dropped, with no reply built
- [ ] `test/l2tp/bad-challenge-response.ci` - proves ze REJECTS a wrong tunnel-auth digest; no test proved ze accepts a right one it did not compute itself

**Behavior to preserve:**
- Every existing `test/l2tp/*.ci` expectation. The suite was green before and after.
- The `kind/tier` cell semantics in `internal/le/rfc/rfc.go`. This spec adds tags, it does not touch the model.

**Behavior to change:**
- None. This spec adds test evidence only. No production Go file is modified.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A Python peer in `test/l2tp/rfc2661-emitted-control-shape.ci` sends an L2TPv2 SCCRQ as a UDP datagram to the port `ze -` is listening on. Format at entry: raw bytes, `struct.pack("!HHHHHH", 0xC802, ...)` plus hand-packed AVPs.

### Transformation Path
1. `Reactor.handlePacket` (`internal/component/l2tp/reactor.go`) reads the datagram, parses the header, and for TunnelID=0 calls `parseSCCRQ` before taking `tunnelsMu`.
2. `parseSCCRQ` (`internal/component/l2tp/tunnel_fsm.go`) walks the AVPs via `NewAVPIterator` and returns an `sccrqInfo`, or an error that causes a silent Debug-level drop.
3. `writeSCCRPBody` (`internal/component/l2tp/tunnel_fsm.go`) writes the SCCRP AVPs in order, Message Type first, each through `WriteAVPHeader` (`internal/component/l2tp/avp.go`).
4. `ChallengeResponse` (`internal/component/l2tp/auth.go`) computes the Challenge Response AVP as `MD5(chapID || secret || challenge)`.
5. The datagram leaves the UDP socket and the Python peer parses it with its own `split_avps`, sharing no code with ze.
6. The peer computes the expected digest with `hashlib.md5` and compares, then sends an SCCCN whose response it computed the same way. `VerifyChallengeResponse` accepts it and the tunnel reaches established.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| ze ↔ external peer | L2TPv2 control messages over UDP | Yes, by `test/l2tp/rfc2661-emitted-control-shape.ci` |
| Go encoder ↔ non-Go decoder | Python `struct` parses ze's emitted bytes | Yes, same test |
| RFC text ↔ implementation | digest re-derived in Python from §4.2 | Yes, same test |

### Integration Points
- `internal/le/rfc/rfc.go` `scan_ci_tags` reads the `# RFC requirement:` lines out of the `.ci` and `carrier_for` resolves the file to the `functional-l2tp` carrier, `functional/verify`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The test speaks UDP to the daemon's real listener; no in-process shortcut |
| No unintended coupling (components stay isolated) | Yes | Test-only change; no production file modified |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses the suite's established daemon-plus-Python-observer shape |
| Zero-copy preserved where applicable (refs, not copies) | N-A | No encoding path changed |
| Registration over hardcoding: new commands, views, families, handlers register and the core discovers them | Yes | The tag is discovered by `scan_tree`; no list names this test |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The unit-only population is dominated by requirements whose oracle is ze's own code, so functional evidence adds real discrimination | today's five IKE defects, all unit-green for months | the backfill adds tier without adding discrimination | mutation: break the producer, check whether unit tests survive | **broken** for the L2TP cluster. `TestChallengeResponseKnown` and `TestCHAPAuthenticationKnownVector` re-derive the digest in the test body, so they DO catch a swapped field order. See Mistake Log |
| A-2 | A `.ci` in `test/l2tp/` earns `functional/verify` | `internal/le/functional/suites.go` `all_suites` lists `l2tp` | the binding is refused as `TIER_UNRUN` | `carrier_for` on the landed file | **confirmed**: `functional-l2tp -> functional/verify`, runner `./le functional` |
| A-3 | The l2tp suite runs natively on darwin, so the tranche executes in `./le verify current mode full` | only `session-stopccn-cascade.ci` carries `option=needs-linux` | the tier claim is true only inside QEMU | full suite run on this host | **confirmed**: 17/17 pass, 1 skip |
| A-4 | RFC 2661 §4.2 tunnel auth can carry a gated binding | the summary lists a §4.2 requirement | the strongest available evidence binds no gated id | read `rfc/short/rfc2661.md` | **broken**: RFC2661-4.2-1 is `[MAY]`, so it is not gated. The digest assertion still ships, binding no id |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The backfill raises tier without raising discrimination, buying a greener ledger for nothing | a mutation that reddens the `.ci` also reddens the unit suite | mutation-verify BOTH layers, and report honestly when the unit layer already catches it. Done: see Goal Validation |
| R-2 | `ai/RFC-REQUIREMENTS.md` is stale until regenerated, and `./le rfc check` fails on staleness | `check_ledger_fresh` byte comparison | deliberate. Three sessions own that file this week; the regen is owed and named in the report |
| R-3 | A future session reads the 76% classifier number as a work list | the number is quoted without its caveat | the caveat is restated in Task and the rule below forbids deriving decisions from it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible. The change is one new `.ci` and no production Go. The worst case is a test that passes vacuously, which the mutation table below is designed to exclude |
| How is it reverted? | Delete `test/l2tp/rfc2661-emitted-control-shape.ci`. The evidence ratchet would then fire on the three requirements, which is the intended alarm |
| Who else touches this path? | A session reviewing `internal/component/ike/**`, and two owning `internal/component/bgp/**` and `internal/plugins/ospf/**`. None of them touch `internal/component/l2tp/**` or `test/l2tp/` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| UDP SCCRQ to ze's L2TP listener | → | `Reactor.handlePacket` then `writeSCCRPBody` (`internal/component/l2tp/tunnel_fsm.go`) | `rfc2661-emitted-control-shape` |
| SCCRQ carrying a Challenge AVP | → | `ChallengeResponse` (`internal/component/l2tp/auth.go`) | `rfc2661-emitted-control-shape` |
| SCCCN carrying an externally computed response | → | `VerifyChallengeResponse` (`internal/component/l2tp/auth.go`) | `rfc2661-emitted-control-shape` |
| `# RFC requirement:` line in the landed `.ci` | → | `scan_ci_tags` then `carrier_for` (`internal/le/rfc/rfc.go`) | `rfc2661-emitted-control-shape` resolves to `functional/verify` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The unit-only population is measured without rendering `ai/RFC-REQUIREMENTS.md` | A per-RFC and per-package distribution exists, with a reachability split against `all_suites` |
| AC-2 | A selection rule is proposed and recorded | The rule is stated as a test on the ORACLE, not on requirement text, and it names what it excludes |
| AC-3 | The risk ranking is backed by measurement | The ranking cites counts and a mutation result, not protocol-family intuition |
| AC-4 | A bounded tranche is bound at non-unit tier | At least three gated requirements move from unit-only to `functional/verify`, each with a recorded RED |
| AC-5 | Every landed assertion is positive | No check in the landed test passes because something failed to happen |
| AC-6 | The remainder is left navigable | The ranked remainder and the owner questions are recorded in this spec and its deferral shard |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (none) | - | This spec adds no production Go, so it adds no unit test. The mutation table below is how its claims are tested | N-A |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| AVP reserved bits 2-5 | mask 0x3C00 | 0 | N-A | any non-zero bit, exercised by the `avp-reserved` mutation |
| Header reserved bits 8-11 | mask 0x00F0 | 0 | N-A | any non-zero bit, exercised by the `hdr-reserved` mutation |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rfc2661-emitted-control-shape` | `test/l2tp/rfc2661-emitted-control-shape.ci` | An L2TP peer that is not ze connects, and every byte ze sends back has the shape RFC 2661 requires, including a tunnel-auth digest the peer recomputes itself | PASS |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | The tranche is bound at `functional/verify` on purpose. Umbrella D3 and parent AC-18 prefer a `.ci` over an interop binding, because a `.ci` runs on every push and interop is nightly and advisory | N-A |

## Files to Modify
- None. No production file is changed by this spec.

## Files to Create
- `test/l2tp/rfc2661-emitted-control-shape.ci` - the tranche: three gated bindings plus the independent digest oracle

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No config surface added |
| YANG validation constraints | No | No leaf added |
| YANG custom validators | No | No leaf added |
| CLI commands/flags | No | No command added |
| CLI grammar (keyword before value) | No | No command added |
| Editor autocomplete | No | No leaf added |
| Functional test for new RPC/API | Yes | `test/l2tp/rfc2661-emitted-control-shape.ci` |
| Pipe completeness | No | No command output added |
| Env var registration | No | The test uses existing `ze.l2tp.skip-kernel-probe` and `ze.log.l2tp` |
| Doctor check for runtime dependencies | No | No new runtime dependency; the test uses the existing UDP listener |
| Prometheus counters/metrics | No | No observable state added |
| BGP family surface | N-A | Not a BGP change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Test-only change |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | No encoder changed; the test observes the existing format |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | **Yes, and OWED** | Three RFC 2661 requirements are newly proven at a new tier. `ai/RFC-REQUIREMENTS.md` needs `./le rfc index-update`, deliberately NOT run this session (R-2). `docs/features/rfc-status.md` support level is unchanged, so no row edit is due |
| 10 | Test infrastructure changed? | No | Uses the existing suite shape |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | No | No source file changed |
| 17 | Existing docs show config/CLI/API examples for this area? | No | - |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - prove a `.ci` tag in `test/l2tp/` resolves to `functional/verify`
   - Tests: `carrier_for("test/l2tp/...")` returns the `functional-l2tp` carrier
   - Files: `test/draft/l2tp/rfc2661-emitted-control-shape.ci`
   - Verify: DONE. `functional-l2tp -> functional/verify`, runner `./le functional`
2. **Phase: Measure** - derive the distribution without rendering the tracked ledger
   - Files: session scratch only
   - Verify: DONE. Table in Task above
3. **Phase: Bind the tranche** - write the test in the draft incubator, prove it green, mutation-verify, promote
   - Verify: DONE. Suite 17/17, mutation table below
4. **Phase: Record the remainder** - the rule, the ranking, the owner questions, the deferral shard
   - Verify: DONE below

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Each of AC-1..AC-6 has evidence in this spec |
| Vacuity | Every check in the landed `.ci` is a positive assertion. The two negatives that would have been absence assertions were NOT landed |
| Oracle independence | The landed test's expected values come from Python, not from a ze helper |
| Tier honesty | The claimed tier is read from `carrier_for`, not asserted |
| Rule: `ai/rules/rfc-compliance.md` | No `{gap}` written. The two conformance questions are raised, not annotated |
| Rule: `ai/rules/evidence.md` | A-1 was BROKEN by measurement and is recorded as broken rather than quietly dropped |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Measured distribution | Table in Task; reproduce with the scratch scripts named in Design Insights |
| Selection rule | "The selection rule" section below |
| Risk ranking | "Risk ranking" section below |
| Three bound requirements | `python3 -c` over `rfc_requirements.scan_ci_tags` on the landed file |
| Mutation evidence | Mutation table below |
| Suite still green | `./le functional l2tp` |

### Security Review Checklist
| Check | What to look for |
|-------|------------------|
| Input validation | The test sends unauthenticated datagrams to a loopback listener on an ephemeral port, which the suite already does in 17 other files. No new exposure |
| Secret handling | The shared secret is the suite's existing `s3cr3t` literal in a test config, never a real credential |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior; if misunderstood → RESEARCH |
| Lint failure | Fix inline; if architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## The selection rule

**A gated unit-only requirement is a backfill candidate when no test bound to it
carries an oracle independent of the code under test, AND its obligation is
observable at a boundary a runnable suite can reach.**

Three tests, applied in order. All three must hold.

| # | Test | Fails when |
|---|------|-----------|
| 1 | **Oracle independence.** Read every test tagged with the requirement id. Does at least one derive its expected value from the RFC text, rather than by calling the production helper that computes it? | Every bound test computes `want` by calling the same code the assertion exercises. `TestBuildCHAPResponse` (`internal/component/l2tp/pppoeclient/session_test.go`) does exactly this: `want := chapMD5Response(...)`, the same helper `buildCHAPResponse` calls |
| 2 | **Boundary observability.** Can the obligation be seen at a socket, a file, or a command output, rather than only inside a function? | The obligation is an internal invariant with no external form |
| 3 | **Tier reachability.** Does a suite named in `internal/le/functional/suites.go` `all_suites`, or a scheduled interop tree, already boot the owning subsystem? | No suite runs it. The scanner refuses the tag as `TIER_UNRUN`, so the requirement is an infrastructure question, not a backfill candidate |

**Defence.** The rule tests the ORACLE, not the requirement text, and that is
the whole point. The parent spec's A-4 forbids driving decisions from the keyword
classifier, and the reason is now measured rather than assumed: the classifier
answers "is this about the wire", which is nearly always yes for a protocol RFC,
and yes is not a reason to write a test. What actually distinguished today's five
IKE defects was not that they were wire obligations. It was that ze's producer
and ze's verifier were the same code path, so every assertion about one was
satisfied by construction from the other. `VerifyChallengeResponse` CALLING
`ChallengeResponse` (`internal/component/l2tp/auth.go`) is that shape written
plainly. A rule keyed on the oracle finds those and leaves alone the large
population of requirements that are wire-visible and already honestly proven. It
also survives contact with evidence, which the text-keyword approach did not:
applying it to this tranche BROKE the spec's own starting assumption, because the
L2TP digests turned out to carry known-answer vectors already.

**Unit of analysis is the REQUIREMENT, not the test.** A self-oracled test
sitting beside a known-answer test is harmless. `TestBuildCHAPResponse` is
vacuous on its own, but `TestCHAPAuthenticationKnownVector`
(`internal/component/l2tp/pppoeclient/auth_test.go`) pins the same digest to a
hardcoded hex string, so RFC 1994's digest requirement is honestly proven and is
NOT a candidate. Judging test-by-test would have manufactured work here.

## Risk ranking, and the evidence that changed it

The starting hypothesis, from today's five IKE defects, was that obligations
about what ze puts on the wire or accepts from a peer are the highest-yield
class, because ze's encoder and decoder agree with each other and with nobody
else. That hypothesis is **structurally right and numerically wrong**, and the
correction is the most useful thing this session produced.

| Rank | Class | Unit-only | Evidence |
|------|-------|-----------|----------|
| 1 | IKE and EAP (`rfc7296`, `rfc3748`, `rfc5216`, `rfc2759`, `rfc7427`, `rfc4301`, `rfc4303`) | 254 | Five defects shipped here on 2026-08-01, every one unit-green for months, every one found only by running against strongSwan. Highest measured defect density in the repo. NOT taken this session: another session is reviewing `internal/component/ike/**` |
| 2 | Subsystems with no runnable suite (BFD 98, VRRP 80, dhcpserver 28, geodns 18, dnsserver 18) | 242 | Cannot be bound at verify tier at all. An owner question, below |
| 3 | L2TP, PPP, PPPoE | 113 | Taken this session. Turned out BETTER covered than predicted: the crypto carries known-answer vectors, and the structural mutations are caught by ze's own strict round-trip parser |
| 4 | IS-IS 52, RSVP-TE 25, LDP 7 | 84 | Runnable suites exist (`isis`, `isis-wire`, `rsvpte`, `ldp`). Unexamined |

**What the mutation evidence actually showed.** Ranking by protocol family is
weaker than ranking by oracle. Of five mutations run, four were caught by the
existing unit suite. The one genuinely vacuous test found
(`TestBuildCHAPResponse`) was covered by a sibling. A future tranche should run
the rule's test 1 as a cheap scan across a whole RFC BEFORE picking it, rather
than reasoning from how wire-facing the protocol feels.

## Mutation verification

Every mutation was applied with `go build -overlay` against a copy under
`tmp/s/<session>/mut-rfc2661/`. The working tree was never edited.

| Mutation | What it breaks | Landed `.ci` | Unit suite |
|----------|----------------|--------------|------------|
| `msgtype-not-first` | `writeSCCRPBody` emits Protocol Version before Message Type | **RED** `Message Type AVP is first in SCCRP (first type=2)` | RED (9 tests) |
| `hdr-reserved` | `controlFlagsFixed` 0xC802 → 0xC812 | **RED** `header reserved bits 8-11 zero (word0=0xc812)` | RED (1 test) |
| `avp-reserved` | `WriteAVPHeader` sets 0x2000 on every AVP | **RED** `offenders=[0, 2, 3, 4, 7, 9, 10, 11, 13]` | RED (45 tests) |
| `digest-order` | `ChallengeResponse` computes MD5(secret ‖ chapID ‖ challenge) | **RED** digest mismatch | RED (`TestChallengeResponseKnown`) |
| `chap-digest-order` | `chapMD5Response` swaps id and secret | not bound | RED (`TestCHAPAuthenticationKnownVector`) |

Read honestly: the landed test discriminates on all four mutations that touch it,
and the unit suite also catches all four. The `.ci` therefore buys altitude
(`ai/rules/completion.md`: the daemon really emits these bytes from
a real socket) and oracle diversity (a non-Go parser cannot share a Go-level
misreading), but for THIS cluster it did not buy discrimination the unit tests
lacked.

## Design Insights
- The spec's own headline figure was a tag count read as a requirement count. Any future session sizing this work should fold `carrier_for` over tags and count REQUIREMENTS.
- Tier reachability is a hard gate that is easy to miss: `functional_suites()` refuses a `.ci` in a suite `all_suites` does not name, so 242 unit-only requirements cannot be backfilled at verify tier no matter how well written the test is.
- Measurement scripts live in `tmp/s/5f02ca42-5e01-481a-bd32-1d8e5029c764/`: `measure_evidence.py`, `reach.py`, `rfc2661_mutate.py`. They import `internal/le/rfc/rfc.go` rather than shelling out, so they never write `ai/RFC-REQUIREMENTS.md`.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Rule keyed on the test's ORACLE | Keyword classifier over requirement text; "every wire requirement gets a `.ci`" | The classifier is forbidden by parent A-4 and answers the wrong question. The oracle test is what actually separated today's five defects from the honestly-proven majority |
| Unit of analysis is the requirement, not the test | Flag every self-oracled test | A vacuous test beside a known-answer test is harmless. Test-level judging would have manufactured work on RFC 1994 |
| Tranche = L2TP, not IKE | IKE is rank 1 by defect density | Another session is reviewing `internal/component/ike/**`. Editing under a live review costs more than it buys. IKE is named as the next tranche |
| Landed only positive assertions | Also land the zero-tunnel-id and reserved-bit-on-receive negatives | Both would have been absence assertions, the exact vacuity trap in `ai/rules/interop-and-goal-validation.md`. Both revealed conformance questions instead, raised below |
| Did not strip an RFC tag to reshape my own draft | Edit the draft in place | `.claude/hooks/pretool-writeedit.py` (retired; now `internal/le/hookruntime/writeedit.go`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) --> refused it, correctly: it cannot tell my uncommitted draft from a committed proof, and self-approval is not user approval. Wrote a second file instead |

## Owner questions (`ai/rules/rfc-compliance.md`: raised, not annotated)

Neither of these is written as a `{gap}`, and neither test was weakened to make
ze look conformant. Both need a decision on which way to fix.

**Q1. RFC2661-24.10-1: ze silently drops an SCCRQ whose Assigned Tunnel ID is 0
instead of replying with StopCCN.** Verified at source: `parseSCCRQ`
(`internal/component/l2tp/tunnel_fsm.go`) returns
`"l2tp: Assigned Tunnel ID AVP must be non-zero"`, and `Reactor.handlePacket`
(`internal/component/l2tp/reactor.go`) logs at Debug and returns. No StopCCN is
built. The summary line reads `[MUST] ... reject with StopCCN (§24.10)`. RFC 2661
§4.4.3 explicitly contemplates this reply: "permitting the peer to identify the
appropriate tunnel even if a StopCCN is sent before an Assigned Tunnel ID AVP is
received", so it is feasible. Against that, the current silent drop avoids
emitting a control message in response to an unauthenticated datagram, which is a
reflection-amplification consideration the code comment appears to be deliberate
about. Which way do you want this fixed? A reproducer is at
`test/draft/l2tp/rfc2661-control-wire-shape.ci` (draft, gitignored).

**Q2. 242 gated MUSTs cannot be proven at verify tier because no suite boots
their subsystem** (BFD 98, VRRP 80, dhcpserver 28, geodns 18, dnsserver 18).
`internal/le/functional/suites.go` `all_suites` names no `bfd`, `vrrp`, or `dhcp` suite, so
`carrier_for` resolves any `.ci` there to `TIER_UNRUN` and the scanner refuses
the tag. VRRP has a nightly interop path (`ze-qemu-vrrp-keepalived-test`), the
others have none. Add suites, accept nightly-only tier for these, or leave them
unit-only by decision?

## Known Limitations
- Three requirements moved. 1533 remain unit-only. The rule and the ranking are the durable output; the tranche is a worked example of applying them.
- `ai/RFC-REQUIREMENTS.md` is not regenerated this session (R-2), so `./le rfc check` will report ledger staleness until `./le rfc index-update` runs.
- The reserved-bit-on-receive behaviour was not established either way: the probe produced no ze log line and no reply, and the cause was not isolated. It is not claimed as a finding.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes (the pre-commit gate; `ai/rules/git-safety.md`)
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
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

## Mistake Log

### Wrong Assumptions
| Assumption | Why it was wrong | What it cost | Corrected by |
|-----------|------------------|--------------|--------------|
| A-1: unit-only requirements are dominated by self-oracled tests | The L2TP crypto carries known-answer vectors (`TestChallengeResponseKnown`, `TestCHAPAuthenticationKnownVector`) that re-derive the RFC formula in the test body. Four of five mutations were caught by the unit suite | Two mutation cycles, and it inverted the tranche's headline claim | Mutation table above; the ranking now says to scan for the oracle before picking an RFC, rather than reasoning from protocol family |
| A-4: RFC 2661 §4.2 tunnel auth carries a gated requirement | `RFC2661-4.2-1` is `[MAY]`, so it is not gated | The strongest assertion in the landed test binds no requirement id | Reading `rfc/short/rfc2661.md` line 487 |
