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
