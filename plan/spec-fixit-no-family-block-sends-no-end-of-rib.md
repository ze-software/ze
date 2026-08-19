# Spec: fixit-no-family-block-sends-no-end-of-rib

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 5/5 (documentation written) |
| Deferral shard | - |
| Updated | 2026-08-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A peer configured with no `family` block exchanges IPv4 unicast in both
directions and never sends an End-of-RIB marker. The session establishes, routes
flow, and the peer waits forever for a barrier that never arrives.

Two producers, both read:

`buildOpen` (`internal/component/bgp/reactor/session_negotiate.go`) is the ONE
producer of ze's OPEN. It fills `caps` from the config's Multiprotocol
capabilities, and only when the config declares none does it fall back to
`s.pluginFamiliesGetter()`. When the config has no `family` block AND no plugin
supplies decode families, `caps` stays empty and the OPEN carries no
Multiprotocol capability at all. Measured OPEN in that state:
`04FDE8005A0102030408020641040000FDE8`, which is ASN4 and nothing else.

`sendInitialRoutes` (`internal/component/bgp/reactor/peer_initial_sync.go`)
iterates `families := nc.Families()` and sends one `message.BuildEOR(fam)` per
negotiated family. An empty set means the loop body never runs. No marker is
sent, no `IncrEORSent`, and nothing logs a warning: the absence is silent at
every layer.

**The session still works, which is what makes this expensive.** RFC 4271 gives
a speaker that negotiates no MP-BGP capability an implicit IPv4-unicast
capability, so routes are exchanged normally. The only missing thing is the
barrier, and a barrier that never arrives looks like a slow peer rather than a
misconfiguration.

**How it was found.** Seven `test/plugin/redistribution-*.ci` fixtures asserted
an IPv4-unicast End-of-RIB on the wire. Wiring their peer blocks up made the
assertion real, and it failed. Adding `family { ipv4/unicast { prefix { maximum
10000 } } }` to each peer fixed it. That is a fixture fix; whether the daemon
should behave this way is the question here, and nothing in that spec's goal
depended on the answer, so it was written up rather than settled there
(`ai/rules/completion.md`). Journal row: `plan/journal/silent-fall-through.md`.

**The question to settle, and it is a conformance question.** RFC 4724 Section 2
governs the End-of-RIB marker, and the implicit IPv4-unicast address family is
still an address family. Three answers are on the table and the spec must pick
one against the RFC text, not against convenience:

- **Refuse**: a peer with no reachable family is a configuration error, and the
  config validator rejects it.
- **Default**: an OPEN with no Multiprotocol capability negotiates implicit
  IPv4 unicast, so `nc.Families()` reports it and the marker is sent.
- **Warn**: keep the behavior, log at startup that this peer will send no
  End-of-RIB.

**Read the RFC before choosing.** `ai/rules/rfc-compliance.md` is blocking here:
when full conformance and full proof of it are reachable, that IS the answer and
Thomas is not asked to pick a narrower one. The ask is owed only if this spec
concludes something less than full conformance is right, and then it asks which
way to fix it.

## RFC Reading — SETTLED 2026-08-17 (answer: Default)

RFC 4724 Section 4, quoted verbatim from `rfc/full/rfc4724.txt`:

> The End-of-RIB marker MUST be sent by a BGP speaker to its peer once it
> completes the initial routing update (including the case when there is no
> update to send) for an address family after the BGP session is established.

Section 2 adds that for the IPv4 unicast address family the marker is "an UPDATE
message with the minimum length", so the marker is defined for that family
whether or not a Multiprotocol capability carried it.

RFC 4271 needs no capability for IPv4 unicast: the UPDATE message itself carries
Withdrawn Routes and NLRI natively, so a session that negotiates no
Multiprotocol capability still exchanges IPv4 unicast. A-1 is confirmed by the
seven `redistribution-*.ci` fixtures, which exchanged routes in exactly this
state. So IPv4 unicast IS "an address family" for the clause above, and the
marker is owed.

**Refuse is refused by the RFC**: a peer that advertises no Multiprotocol
capability is valid RFC 4271, so a validator rejecting it would make Ze reject a
conformant configuration. **Warn is less than the MUST.** **Default is the only
answer the text supports**, and full conformance plus full proof is reachable, so
no owner question is owed (`ai/rules/rfc-compliance.md`).

### Where the fix lands, and why it is ONE site

Read 2026-08-17: `Negotiate` (`internal/core/bgp/capability/negotiated.go`)
builds `localFamilies` and `remoteFamilies` from Multiprotocol capabilities only,
then intersects them (`:229`). A side that advertised no Multiprotocol capability
therefore contributes the EMPTY set instead of its implicit IPv4 unicast. That is
the single producer behind both symptoms, because `NewNegotiatedCapabilities`
(`internal/component/bgp/reactor/negotiated.go`) copies `neg.Families()` verbatim
and `sendInitialRoutes` iterates the copy.

**Fix: in `Negotiate`, a side that advertises NO Multiprotocol capability is
treated as advertising `ipv4/unicast`, on both sides, before the intersection.**

→ Decision: the fix goes in `Negotiate`, NOT in `buildOpen`. Adding a synthetic
Multiprotocol capability to the OPEN would change bytes Ze puts on the wire for
every such peer, which AC-2 exists to prevent and which the RFC does not ask for.
The implicit family is a fact about what was NEGOTIATED, so it belongs in the
producer that answers that question.

→ Decision: applying the rule to BOTH sides also closes a wider interop hole
found while reading. Today a peer that sends no capabilities at all (valid RFC
4271) intersects to the empty set even when Ze's config declares
`ipv4/unicast`, so Ze negotiates no family with it. FRR and BIRD both treat a
silent peer as supporting IPv4 unicast. The per-side rule fixes that case with
the same ten lines and needs no second spec.

| Local advertises | Remote advertises | Negotiated after the fix |
|------------------|-------------------|--------------------------|
| nothing | nothing | `ipv4/unicast` |
| nothing | `{ipv4/unicast, ipv6/unicast}` | `ipv4/unicast` |
| `{ipv4/unicast, ipv6/unicast}` | nothing | `ipv4/unicast` |
| `{ipv6/unicast}` | nothing | empty (correct: no common family) |
| any non-empty | any non-empty | unchanged, byte for byte |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/bgp/` -- the capability negotiation and initial-sync
      documents covering `buildOpen` and `sendInitialRoutes`
- [ ] `ai/rules/rfc-compliance.md` -- conformance is not negotiable; the ask is
      owed only before doing LESS
- [ ] `ai/rules/config.md` -- if the answer is Refuse, the validator is where it
      lands

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4724.md` -- Section 2, the End-of-RIB marker, one per family
      per session
- [ ] `rfc/short/rfc4271.md` -- the implicit IPv4-unicast family when no
      Multiprotocol capability is negotiated
- [ ] `rfc/short/rfc4760.md` -- what the Multiprotocol capability declares and
      what its absence means

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/session_negotiate.go` -- `buildOpen`
  → Constraint: it is the ONE producer of ze's OPEN, called a second time by the
    reload swap decision to answer "would the OPEN come out the same?". A second
    builder would let that decision be taken against an OPEN ze never sends, so
    any fix goes INSIDE it.
  → Constraint: config families and plugin families are exclusive by design. If
    config declares any, plugin families are ignored. A default must not break
    that rule.
- [ ] `internal/component/bgp/reactor/peer_initial_sync.go` -- `sendInitialRoutes`
  → Constraint: the EoR claim is taken BEFORE the send, because a route server
    reaches the same wire through `AnnounceEOR` and RFC 4724 Section 2 allows one
    marker per family per session. A default family must go through
    `ClaimInitialSyncEOR` like any other.
- [ ] whatever produces `NegotiatedCapabilities.Families()` -- the join between
      the two OPENs
  → Decision: if the implicit family is knowable here, Default is a change at
    one site rather than two.

**Behavior to preserve:**
- Config families win over plugin families.
- One End-of-RIB per family per session, across both producers.
- A peer that DOES declare families keeps its current wire behavior byte for
  byte.

**Behavior to change:**
- A peer with no declared family stops silently skipping the barrier.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator writes a `peer` block with no `family` section and starts ze.

### Transformation Path
1. Config is parsed; the peer's `Capabilities` carry no Multiprotocol entry.
2. `buildOpen` finds `configHasFamilies` false and consults
   `s.pluginFamiliesGetter`, which returns nothing when no plugin declares
   decode families.
3. The OPEN goes out with ASN4 only.
4. The peer's OPEN is joined with ze's into `NegotiatedCapabilities`.
5. `nc.Families()` is empty.
6. `sendInitialRoutes` iterates it, sends nothing, and returns.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| config -> reactor | `PeerSettings` and `configCaps`, passed separately for their lock rules | Yes, read in `buildOpen` |
| ze OPEN <-> peer OPEN | `NegotiatedCapabilities` | Not yet: whether the implicit family is representable there is the first read |
| reactor -> wire | `message.BuildEOR(fam)` per family | Yes, read in `sendInitialRoutes` |

### Integration Points
- `test/scripts/ze_api.py` `wait_peer_eor_sent` is the functional suite's barrier
  and depends on this.
- Graceful restart, which is what RFC 4724 defines the marker for.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | both producers are single-site |
| No unintended coupling (components stay isolated) | Yes | the fix stays in the reactor, or in the config validator |
| No duplicated functionality (extends existing, does not recreate) | Yes | `buildOpen`'s own comment forbids a second OPEN builder |
| Zero-copy preserved where applicable (refs, not copies) | Yes | no new per-message allocation is implied |
| Registration over hardcoding | Yes | families are already a registry lookup via `family.LookupFamily` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validation | Status |
|----|-----------|-------|----------|------------|--------|
| A-1 | Routes really are exchanged in this state, so only the barrier is missing | the fixtures exchanged routes before the `family` block was added | the defect is larger than the marker | run a two-peer fixture with no family block and check the peer's RIB | confirmed -- `test/interop/scenarios/no-family-peer-eor-frr` ran against FRR 10.3.1 with the fix REMOVED: FRR installed 10.10.0.0/24 from ze and logged no End-of-RIB. Routes flowed, only the marker was missing |
| A-2 | No operator config in `demos/` or `docs/` omits the family block | unchecked | published examples are affected and must move with the fix | grep the corpus for peer blocks with no `family` | broken -- 106 peer blocks under `docs/` and `demos/` declare no family (`demos/terminal/config-graph/router.conf`, `docs/config-reference.md`, `docs/DESIGN.md` among them). Under Default none must move: each one gains the marker it was already owed. This count is also why Refuse was refused, which R-2 predicted |
| A-3 | The implicit family is representable in `NegotiatedCapabilities` | not read yet | Default costs more than one site | read the producer of `Families()` | confirmed -- `NewNegotiatedCapabilities` (`internal/component/bgp/reactor/negotiated.go`) copies `neg.Families()` verbatim, so the implicit family reaches it with no edit at that layer |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | Default changes the wire for existing deployments that rely on no marker | an interop scenario goes red | the marker is what RFC 4724 requires; a peer that breaks on it is the finding |
| R-2 | Refuse breaks working configs at startup | a demo or a user config stops loading | validate A-2 before choosing Refuse |
| R-3 | The chosen answer is narrower than the RFC requires | the spec reaches for Warn | `ai/rules/rfc-compliance.md`: ask which way to fix it, never whether to skip it |

## Blast Radius

Every peer configured without a `family` block: its OPEN, its initial sync, and
its graceful-restart behavior. If Default is chosen, the wire changes for those
peers. Interop scenarios and the functional suite's EoR barrier both observe it.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| a peer block with no `family` section, session established | -> | `buildOpen` then `sendInitialRoutes` | a `.ci` under `test/encode/` asserting the OPEN and the End-of-RIB on the wire |
| the same against a real peer daemon | -> | the negotiated result | an interop scenario under `test/interop/scenarios/` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A peer with no `family` block and no plugin decode families | The behavior the spec's RFC reading selects: refuse at validation, or send the marker, or warn once at startup. Not silence |
| AC-2 | A peer that declares `family { ipv4/unicast }` | Wire output byte-identical to today |
| AC-3 | A peer with no config families but a plugin declaring decode families | Unchanged: plugin families still fill the gap |
| AC-4 | The chosen behavior, against a real peer daemon | An interop scenario passes with FRR or BIRD, per `ai/rules/interop-and-goal-validation.md` |
| AC-5 | Graceful restart with such a peer | Whatever RFC 4724 requires, proven by a tagged test |
| AC-6 | The RFC reading itself | Recorded with the section text quoted, and an `RFC requirement:` tagged test if it gates a MUST |

## End-to-End User Stories

- An operator writes a minimal peer block, starts ze, and either gets a clear
  refusal naming the missing family, or gets a session whose End-of-RIB arrives.
  Neither outcome is a peer that waits forever.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBuildOpenNoFamilyNegotiatesImplicitIPv4Unicast` | `internal/component/bgp/reactor/session_negotiate_test.go` | AC-1 | green; red with the fix removed |
| `TestBuildOpenConfigFamiliesUnchanged` | same | AC-2 | green; red when the default is applied unconditionally |
| `TestBuildOpenPluginFamiliesUnchanged` | same | AC-3 | green; red when the default is applied unconditionally |
| `TestNegotiateImplicitIPv4UnicastWhenNoMultiprotocol` (the five-row truth table) | `internal/core/bgp/capability/negotiated_test.go` | AC-1, AC-2 | green; three rows red with the fix removed, two rows red when it is unconditional |
| `TestNegotiateSilentPeerSatisfiesRequiredIPv4Unicast` | same | AC-1 | green; red with the fix removed |
| `TestInitialSyncEORSentWhenNeitherSideDeclaredAFamily` (`RFC requirement: RFC4724-4-1 positive`) | `internal/component/bgp/reactor/peer_initial_sync_test.go` | AC-1, AC-5, AC-6 | green; red with the fix removed |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `no-family-peer-open-and-eor.ci` | `test/encode/` | a peer block with no `family` section: the OPEN and the End-of-RIB are asserted as hex on the wire | green in `make ze-functional-encode-test` (57/57). With the fix removed the peer logs the identical OPEN and then nothing, and the fixture times out |
| `no-family-peer-refused.ci` | `test/parse/` | if the answer is Refuse, the config is rejected with a message naming the missing family | not written: the RFC reading refused Refuse, so this row has no subject |
| `no-family-peer-graceful-restart.ci` | `test/plugin/` | graceful restart with such a peer reaches whatever RFC 4724 requires | not written: the RFC 4724 Section 4 marker is the requirement, and `TestInitialSyncEORSentWhenNeitherSideDeclaredAFamily` plus the FRR scenario prove it through the producing function and against a peer. Ze's health EOR timer now arms for such a peer too, because `startEORTimer` returns early on a zero family count |

### Interop Tests (Scope: protocol)
| Scenario | Peer daemon | Assertion | Status |
|----------|-------------|-----------|--------|
| `no-family-peer-eor-frr` | FRR 10.3.1 | session establishes, FRR installs ze's route, FRR reports the IPv4-unicast capability as advertised by itself and not received from ze, and FRR's own decode log carries the End-of-RIB marker | PASS. Fails with the fix removed: same route, no marker |

## Files to Modify

- `internal/component/bgp/reactor/session_negotiate.go` -- `buildOpen`, if the
  answer is Default or Warn
- `internal/component/bgp/reactor/peer_initial_sync.go` -- `sendInitialRoutes`,
  if the implicit family is added at the negotiation join instead
- the config validator, if the answer is Refuse
- `docs/guide/configuration.md` -- whichever answer wins is operator-visible

## Files to Create

- a `test/encode/` fixture and a `test/interop/scenarios/` scenario

## Implementation Steps

1. **Phase: Read the RFCs and decide** -- RFC 4724 Section 2, RFC 4271's implicit
   family, RFC 4760. Write the reading down with the section text quoted
   - Verify: AC-6, and the choice is justified by text rather than by cost
2. **Phase: Validate the assumptions** -- A-1, A-2, A-3
   - Verify: all three confirmed or broken before any code
3. **Phase: Implement at the site the reading selects**
   - Verify: AC-1, AC-2, AC-3
4. **Phase: Prove it on the wire and against a peer**
   - Verify: AC-4, AC-5
5. **Phase: Documentation**
   - Verify: `make ze-doc-verify`

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Interop scenario passes with a named peer daemon

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] **Commit A:** code + tests + spec
- [ ] **Commit B:** `git rm plan/<spec>` only

## RFC Documentation (Scope: protocol)

- [ ] The reading of RFC 4724 Section 2 against the implicit IPv4-unicast family
      recorded in `rfc/short/rfc4724.md` if it adds a requirement row
- [ ] `RFC requirement:` tag on the test that proves it, if it gates a MUST

## Known Limitations

- The seven `test/plugin/redistribution-*.ci` fixtures already carry an explicit
  `family` block. They are correct either way and this spec does not revert them.
- `startEORTimer` (`internal/component/bgp/reactor/session_health.go`) returns
  early on a zero family count, so a GR-negotiated peer with no `family` block used
  to arm no End-of-RIB timer at all. It now arms one, and such a peer can raise an
  `eor-timeout` warning where it was silent before. That is correct and new: the
  peer owes Ze the same RFC 4724 Section 4 marker.

---

## Implementation Summary

### What Was Implemented

`Negotiate` (`internal/core/bgp/capability/negotiated.go`) now substitutes
`ipv4/unicast` for an EMPTY per-side family set, on each side independently,
immediately before the RFC 4760 Section 8 intersection. Both `localFamilies` and
`remoteFamilies` are built with `make`, so the substitution writes into an
allocated map and cannot panic. Ten lines at one site.

Three comments were corrected to describe the new answer: the `Negotiate` doc
comment, `SupportsFamily` and `Families`.

The OPEN builder was NOT touched. `buildOpen`
(`internal/component/bgp/reactor/session_negotiate.go`) still advertises no
Multiprotocol capability for such a peer, which is what AC-2 exists to protect
and what the FRR scenario asserts from the peer's side.

### Bugs Found/Fixed

- The defect itself: a side that advertised no Multiprotocol capability
  contributed the empty set, so `nc.Families()` was empty and the family loop in
  `sendInitialRoutes` (`internal/component/bgp/reactor/peer_initial_sync.go`)
  never ran. Covered by `TestInitialSyncEORSentWhenNeitherSideDeclaredAFamily`
  (`RFC requirement: RFC4724-4-1 positive`) and
  `test/encode/no-family-peer-open-and-eor.ci`.
- A wider interop hole closed by the same ten lines: a peer that sends no
  capabilities at all is valid RFC 4271, and Ze used to negotiate no family with
  it even when Ze's own config declared `ipv4/unicast`. Covered by the truth-table
  row "local advertises ipv4 and ipv6, remote silent".
- `TestNegotiateEmpty` asserted an empty family list and so pinned the defect. The
  expectation was CORRECTED, not relaxed: it now pins a length of one plus
  `SupportsFamily(ipv4/unicast)`, so the family set is still pinned exactly.
  `scripts/dev/audit-test-relaxation.py` reports nothing on that file.

### Documentation Updates

- `docs/config-reference.md` -- new subsection "A Peer That Declares No Family",
  with anchors on `negotiated.go` and `session_negotiate.go`.
- `docs/guide/configuration.md` -- the same subsection for the operator guide,
  with a third anchor on `peer_initial_sync.go`.
- `docs/architecture/wire/capabilities.md` -- added by this closure. Its
  "Capability Negotiation / Rules" list stated the rule as pure intersection,
  which is stale for the address-family half after this change. A second rule now
  names the implicit family, and two source anchors were added.
- `docs/features/rfc-status.md` -- added by this closure. The RFC 4724 row's
  Implemented coverage now states that the End-of-RIB send covers a session where
  neither speaker advertised a Multiprotocol capability, naming both producers and
  the FRR scenario. The Remaining cell and its spelled gap count are untouched, so
  `check_gap_count_agreement` still agrees.
- `make ze-doc-verify`: one red, and it is not this spec's. A source anchor in
  `docs/architecture/api/process-protocol.md` names `runHub` in
  `cmd/ze/hub/main.go`, where no such function is declared. That file is modified
  and uncommitted in this shared checkout, and `cmd/ze/hub/` carries two modified
  test files and one untracked one, so another session is mid-edit there. Neither
  closure commit carries either path.

### Deviations from Plan

- The `Files to Modify` list named `buildOpen` and `sendInitialRoutes` as the
  candidate sites. The RFC reading moved the fix to `Negotiate`, one layer down,
  which is one site instead of two and leaves the OPEN bytes untouched.
- Two rows of the TDD plan have no subject and were not written:
  `test/parse/no-family-peer-refused.ci` (the reading refused Refuse) and
  `test/plugin/no-family-peer-graceful-restart.ci` (RFC 4724 Section 4's marker IS
  the graceful-restart requirement, and the tagged unit test plus the FRR scenario
  prove it through the producing function and against a peer).
- New and correct, not planned: the `startEORTimer` arming recorded under Known
  Limitations above.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-2 assumed no published example omits the `family` block | 106 peer blocks under `docs/` and `demos/` declare none | grep over the corpus during Phase 2 | Broken in Ze's favour: under Default none must move, each gains the marker it was already owed. It is also why Refuse was refused, which R-2 predicted |
| approach | The spec expected the fix in `buildOpen`, `sendInitialRoutes`, or both | the implicit family is a fact about what was NEGOTIATED, so it belongs in `Negotiate`, which both consumers read | reading the producer of `Families()` in Phase 2 (A-3) | Fix moved one layer down; recorded in Deviations |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Decide Refuse / Default / Warn against the RFC text, not against cost | Done | spec section "RFC Reading -- SETTLED 2026-08-17" | RFC 4724 Section 4 quoted verbatim from `rfc/full/rfc4724.txt`; Default is the only answer the text supports |
| A peer with no `family` block stops silently skipping the barrier | Done | `internal/core/bgp/capability/negotiated.go` `Negotiate` | the marker is on the wire in `test/encode/no-family-peer-open-and-eor.ci` and decoded by FRR |
| Config families still win over plugin families | Done | `internal/component/bgp/reactor/session_negotiate.go` `buildOpen` (unchanged) | `TestBuildOpenPluginFamiliesUnchanged` |
| A peer that DOES declare families keeps its wire behavior byte for byte | Done | `buildOpen` unchanged; the fix is downstream of the OPEN | `TestBuildOpenConfigFamiliesUnchanged`, and the FRR scenario asserts Ze advertises no Multiprotocol capability |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestInitialSyncEORSentWhenNeitherSideDeclaredAFamily`, `TestBuildOpenNoFamilyNegotiatesImplicitIPv4Unicast`, `test/encode/no-family-peer-open-and-eor.ci` | the chosen behavior is Default, and the marker reaches the socket |
| AC-2 | Done | `TestBuildOpenConfigFamiliesUnchanged`, and the OPEN hex pinned in the `.ci` | `04FDE8005A0102030408020641040000FDE8` with and without the fix |
| AC-3 | Done | `TestBuildOpenPluginFamiliesUnchanged` | plugin decode families still fill the gap |
| AC-4 | Done | `make ze-interop-test INTEROP_SCENARIO=no-family-peer-eor-frr` PASS against FRR 10.3.1, re-run 2026-08-19 | FRR decodes the marker and reports the IPv4-unicast capability as advertised by itself alone |
| AC-5 | Done | `TestInitialSyncEORSentWhenNeitherSideDeclaredAFamily` plus the FRR scenario | RFC 4724 Section 4 IS the graceful-restart marker requirement; `startEORTimer` now arms for such a peer too |
| AC-6 | Done | spec section "RFC Reading -- SETTLED 2026-08-17"; the `RFC4724-4-1` row in `rfc/short/rfc4724.md`, no annotation; the tag on `TestInitialSyncEORSentWhenNeitherSideDeclaredAFamily` | `make ze-rfc-check` resolves it, interop evidence 34 |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestBuildOpenNoFamilyNegotiatesImplicitIPv4Unicast` | Done | `internal/component/bgp/reactor/session_negotiate_test.go` | landed in `df44d8d27` |
| `TestBuildOpenConfigFamiliesUnchanged` | Done | same | landed in `df44d8d27` |
| `TestBuildOpenPluginFamiliesUnchanged` | Done | same | landed in `df44d8d27` |
| `TestNegotiateImplicitIPv4UnicastWhenNoMultiprotocol` | Done | `internal/core/bgp/capability/negotiated_test.go` | five-row truth table, landed in `8f0d02e81` |
| `TestNegotiateSilentPeerSatisfiesRequiredIPv4Unicast` | Done | same | pins `CheckRequired` |
| `TestInitialSyncEORSentWhenNeitherSideDeclaredAFamily` | Done | `internal/component/bgp/reactor/peer_initial_sync_test.go` | `RFC requirement: RFC4724-4-1 positive` |
| `no-family-peer-open-and-eor.ci` | Done | `test/encode/` | PASS in a 59/59 suite run, 2026-08-19 |
| `no-family-peer-refused.ci` | Changed | -- | no subject: the RFC reading refused Refuse. Recorded in Deviations |
| `no-family-peer-graceful-restart.ci` | Changed | -- | no subject: Section 4's marker IS the GR requirement, proven twice over. Recorded in Deviations |
| `no-family-peer-eor-frr` | Done | `test/interop/scenarios/no-family-peer-eor-frr/` | PASS against FRR 10.3.1 |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/reactor/session_negotiate.go` | Changed | not modified: the fix moved to `Negotiate`, so the OPEN bytes are untouched |
| `internal/component/bgp/reactor/peer_initial_sync.go` | Changed | not modified: it reads the negotiated list and needed no edit |
| the config validator | Changed | not modified: Refuse was refused by the RFC |
| `internal/core/bgp/capability/negotiated.go` | Done | the one producer changed |
| `docs/guide/configuration.md` | Done | "A Peer That Declares No Family" |
| `test/encode/no-family-peer-open-and-eor.ci` | Done | created |
| `test/interop/scenarios/no-family-peer-eor-frr/` | Done | created, 3 files |

### Audit Summary
- **Total items:** 27
- **Done:** 22
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 5 (three planned files not modified because the fix moved one layer
  down, and two TDD rows whose subject the RFC reading removed; all five recorded
  in Deviations, none reduces coverage)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A peer with no `family` block sends its End-of-RIB, so the peer stops waiting forever | functional (wire hex) | `test/encode/no-family-peer-open-and-eor.ci` asserts `FFFF...00170200000000` as seq=2 on the socket. `make ze-functional-encode-test` 2026-08-19: 59/59 PASS, this fixture at 9.4s. With the fix removed the peer logs the identical OPEN and then nothing, and the fixture times out |
| Ze speaks this correctly to another implementation | interop | `make ze-interop-test INTEROP_SCENARIO=no-family-peer-eor-frr` PASS against FRR 10.3.1, re-run 2026-08-19. FRR's own `debug bgp updates in` decode of the marker is the assertion, so a peer that never decoded it cannot pass. Discriminating: with the fix removed FRR still installs 10.10.0.0/24 and logs no marker (measured 2026-08-17) |
| The wire is unchanged for a peer that DOES declare a family | interop (negative) | the same scenario fails if FRR reports the IPv4-unicast capability as `advertisedAndReceived`. Measured value 2026-08-19: `{'advertised': True}`, so Ze advertised none |
| The RFC 4724 Section 4 MUST is proven, not just implemented | RFC tagged test | the `RFC4724-4-1` row in `rfc/short/rfc4724.md` carries no annotation. `make ze-rfc-check` resolves four positive carriers for it, `TestInitialSyncEORSentWhenNeitherSideDeclaredAFamily` (unit/verify) and the `no-family-peer-eor-frr` `check.py` (interop/nightly) among them, and one negative in `internal/component/bgp/message/eor_test.go`. Gate green 2026-08-19: 2966 gated MUST-level requirements, 3581 tags resolved |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none | done | The spec metadata declares `Deferral shard: -` and no `plan/deferrals/fixit-no-family-block-sends-no-end-of-rib.md` exists. Nothing is owed to a later session. The two unwritten TDD rows lost their subject to the RFC reading, and both are recorded in Deviations |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-no-family-block-sends-no-end-of-rib-fa011c6d-dddd-408b-a46c-3ee25189f6a1.md`, written by `scripts/dev/review_gate.py record` over 8 files |
| `review_gate.py check` | `review_gate: OK (0 code files, clean, hashes match ...)`, exit 0 |
| Rounds | 2. Round 1 found two documentation-drift ISSUEs and fixed them; round 2 read the two edited files and the renumbered list and found nothing |
| Reviewer lenses used | wiring + functional coverage + documentation drift (steps 2-4); logic, edge cases, allocation, security (steps 11-15); altitude and simplicity (step 17); ze-style (step 18); interop and goal validation (step 20); RFC compliance (step 21). Three lenses because the diff touches a protocol path and a test that pins it |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The architecture doc anchored to the changed producer states the negotiation rule as pure intersection: "Negotiated capabilities = capabilities both peers advertise". That is now incomplete for the address-family half, and it is the document a reader consults before touching `Negotiate` | `docs/architecture/wire/capabilities.md`, "Capability Negotiation / Rules" | A second rule names the per-side implicit `ipv4/unicast`, states that it is one family and not a wildcard, and states that the OPEN bytes are unchanged. Two source anchors added on `negotiated.go` and `peer_initial_sync.go` |
| 2 | ISSUE | The public RFC ledger's RFC 4724 row claims "End-of-RIB send and detect" with no mention that the send used to skip a session where neither speaker advertised a Multiprotocol capability. `/ze-close` step 4 makes the row update BLOCKING when a change newly proves RFC-level behavior | `docs/features/rfc-status.md`, the RFC 4724 row | The Implemented coverage cell now names the case, both producers and the FRR scenario. The Remaining cell and its spelled count are untouched; `check_gap_count_agreement` re-run green |
| 3 | NOTE | `startEORTimer` returns early on a zero family count, so the fix arms an EoR timer for a GR peer that used to arm none. Correct and new, but unrecorded | `internal/component/bgp/reactor/session_health.go` `startEORTimer` | Recorded under Known Limitations and in Deviations from Plan. No code change: the peer owes Ze the same marker, so the warning is the detect half of the same MUST |
| 4 | NOTE | `make ze-rfc-index-update`, run to clear a foreign red, rewrote 15 tracked generated ledgers from a working tree holding another session's uncommitted `rfc/enrolled.txt`, `rfc/short/rfc1195.md` and `rfc/short/rfc5303.md`. Their annotation text is now in `rfc/requirements/rfc1195.md` and `rfc/requirements/rfc5303.md` | `rfc/requirements/`, `ai/RFC-REQUIREMENTS.md` | No ledger is in either closure commit. They stay in the working tree for the session that owns the sources. Row added to `plan/journal/concurrent-rfc-gate-stale.md` |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/encode/no-family-peer-open-and-eor.ci` | Yes | `ls -la` reports it, 2458 bytes, Aug 17 20:31 |
| `test/interop/scenarios/no-family-peer-eor-frr/check.py` | Yes | `ls -la` reports it, 4945 bytes, Aug 17 21:31 |
| `test/interop/scenarios/no-family-peer-eor-frr/frr.conf` | Yes | `ls -la` reports it, 744 bytes, Aug 17 21:20 |
| `test/interop/scenarios/no-family-peer-eor-frr/ze.conf` | Yes | `ls -la` reports it, 502 bytes, Aug 17 21:04 |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | the marker is sent | `TestInitialSyncEORSentWhenNeitherSideDeclaredAFamily` PASS, 2026-08-19; `make ze-unit-pkg-test PKG=./internal/component/bgp/reactor` ok 151.4s |
| AC-2 | the OPEN is unchanged | `TestBuildOpenConfigFamiliesUnchanged` PASS, and the `.ci` pins the OPEN hex at seq=1 |
| AC-3 | plugin families unchanged | `TestBuildOpenPluginFamiliesUnchanged` PASS |
| AC-4 | a real peer agrees | `make ze-interop-test INTEROP_SCENARIO=no-family-peer-eor-frr` exit 0, `PASS 1 scenario(s)`, 2026-08-19 |
| AC-5 | graceful restart | the same tagged test plus the FRR scenario; `startEORTimer` now arms for such a peer |
| AC-6 | the MUST is proven | `make ze-rfc-check` exit 0, 2026-08-19: `RFC4724-4-1` gated, four positive carriers, one negative, interop evidence 34 |
| all | the truth table | `make ze-unit-pkg-test PKG=./internal/core/bgp/capability` ok, all five rows of `TestNegotiateImplicitIPv4UnicastWhenNoMultiprotocol` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| a peer block with no `family` section, session established -> `buildOpen` then `sendInitialRoutes` | `test/encode/no-family-peer-open-and-eor.ci` | Yes. Read the file: it feeds a real `bgp { peer peer1 { ... } }` with no `family` block to `ze -`, runs `ze-peer` against it, and asserts both frames as full hex. `option=open:value=inspect-open-message` makes the OPEN seq=1, so the OPEN assertion and the marker assertion are on the same connection |
| the same against a real peer daemon | `test/interop/scenarios/no-family-peer-eor-frr/check.py` | Yes. Read the file: `wait_session`, then `wait_route` and `check_route` so the marker is not a barrier over an empty conversation, then a capability assertion that fails if Ze advertised Multiprotocol, then a 60s poll of FRR's own decode log, then `session_established` at the end |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | the FRR scenario run with the fix REMOVED installed 10.10.0.0/24 and logged no End-of-RIB. Routes flowed; only the marker was missing |
| A-2 | broken | 106 peer blocks under `docs/` and `demos/` declare no family. Broken in Ze's favour under Default, and the reason Refuse was refused. Mistake Log row above |
| A-3 | confirmed | `NewNegotiatedCapabilities` (`internal/component/bgp/reactor/negotiated.go`) copies `neg.Families()` verbatim, so the implicit family reaches every consumer with no edit at that layer |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Config syntax: a peer may omit `family` and still gets the marker | `docs/config-reference.md` and `docs/guide/configuration.md`, both anchored to `negotiated.go` `Negotiate`; the `.ci` proves the claim on the wire | Yes |
| Architecture: the capability negotiation rules | `docs/architecture/wire/capabilities.md`, the rule added by this closure, anchored to `Negotiate` | Yes |
| RFC compliance: the public RFC 4724 claim | `docs/features/rfc-status.md`, Implemented coverage extended, gap count untouched, `make ze-rfc-check` green | Yes |
| CLI reference, API/RPC docs, plugin SDK, wire format, comparison table, test infrastructure | No update needed. The change adds no command, no RPC, no SDK symbol and no new wire encoding: the marker's bytes are the RFC 4724 Section 2 form Ze already emitted for every declared family. `grep -rn "capability/negotiated.go" docs/` names four documents, and the two not edited here (`docs/architecture/edge-cases/as4.md`, `docs/exabgp/exabgp-code-map.md`) anchor on `Negotiated.ASN4` and on a file mapping, neither of which this change touches | Yes |
| Doctor checks | No update needed. The change adds no file path, socket, kernel module, listen port, external binary or TLS cert, so `ai/rules/repo-maintenance.md`'s runtime-dependency trigger does not fire | Yes |

## Core Insight

An intersection has no natural answer for a side that declared nothing, and the
empty set is not it. `Negotiate` intersected two sets built from Multiprotocol
capabilities alone, which reads "advertised no capability" as "supports no
family". The protocol's own answer is one family: RFC 4271 carries IPv4 unicast
in the UPDATE message with no capability at all, so silence is a declaration
rather than an absence. The failure was invisible because the base protocol kept
working: routes flowed and only the barrier was missing, which looks like a slow
peer rather than a bug. The general shape is `zero-value-as-valid-answer` over a
SET: when a set is built by collecting declarations, ask what the protocol says
the empty collection means before intersecting it.
