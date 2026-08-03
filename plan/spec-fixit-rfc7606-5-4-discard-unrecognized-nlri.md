# Spec: fixit-rfc7606-5-4-discard-unrecognized-nlri -- meet RFC 7606 Section 5.4

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 3/3 |
| Deferral shard | `plan/deferrals/spec-fixit-rfc7606-5-4-discard-unrecognized-nlri.md` |
| Updated | 2026-08-01 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

RFC 7606 Section 5.4: "A BGP speaker advertising support for a typed address
family MUST discard routes with unrecognized NLRI types, unless the relevant
specification for that address family specifies otherwise."

Ze does not do this. Unrecognized typed NLRI is RETAINED and PROPAGATED
opaquely. The requirement is tracked as `RFC7606-5.4-1` and currently carries a
`{gap}` annotation in `rfc/short/rfc7606.md`.

**→ Decision (2026-08-01, Thomas): IMPLEMENT FULL COMPLIANCE.** Discard
unrecognized typed NLRI per Section 5.4, and prove it with an
`RFC requirement:` tagged test.

**This REVERSES the ruling of 2026-07-20** recorded in the `{gap}` annotation.
That ruling is void twice over: `ai/rules/rfc-compliance.md` voided every answer
pointing away from full compliance on 2026-07-27, a week after it was made; and
Thomas was re-asked on 2026-08-01 with the full rationale in front of him and
chose compliance.

The annotation is deliberately NOT edited yet. It describes what the code does
TODAY and is accurate until this spec lands. Removing it first would make the
ledger claim a conformance Ze does not have. Delete it in the same commit as the
behaviour change.

## Why this is not a small change

The retention is EMERGENT, not written. Nothing decides to keep these routes, so
there is no single branch to flip. Two byte-opaque paths never read the route
type. Both were verified against their producing symbol on 2026-08-01:

| Path | Producing symbol | Why it retains |
|------|------------------|----------------|
| RIB insert | `FamilyRIB.Insert` (`internal/component/bgp/plugins/rib/storage/familyrib.go`) | for a non-CIDR family it calls `insertOpaque` with the raw `nlriBytes` and never parses a route type |
| Same-context forward | `buildFwdBody` (`internal/component/bgp/reactor/forward_body.go`) | appends `peerWire.Payload()` verbatim when the ContextID matches, the zero-copy fast path |

`ParseEVPN` / `ParseMVPN` are NOT on either path. Their only non-test caller is
the display/JSON decoder (`internal/component/bgp/plugins/nlri/evpn`), which
emits `parsed: false` rather than dropping. Type recognition therefore does not
happen anywhere a discard decision could be made today, and this spec has to
introduce it.

## The counter-argument that must be answered, not ignored

The 2026-07-20 rationale was substantive. The design must address it rather than
pretend it did not exist:

- Ze has no EVPN forwarding plane (no MAC-VRF, no MAC learning, no DF election),
  so it is only ever a control-plane relay for these routes. Discarding removes
  function from the PEs on either side while improving nothing locally.
- RFC 7606 Section 6 warns that asymmetric dropping inside an AS causes
  "long-lived forwarding loops and black holes". Section 5.4 compliance on a
  pure relay is exactly that asymmetry.
- Precedent cuts both ways. RFC 9552 Section 5.1 uses 5.4's own "unless the
  relevant specification specifies otherwise" clause to require
  preserve-and-propagate for BGP-LS. RFC 7432 never did so for EVPN.
- ExaBGP retains and re-advertises opaquely. GoBGP session-resets on an unknown
  type, which is the reading 5.4 exists to prevent.

**The design question this spec must answer first:** which families does 5.4
actually bind for Ze, given the "unless the relevant specification specifies
otherwise" clause? A blanket discard across every typed family would break
BGP-LS, where RFC 9552 requires the opposite. The likely shape is per-family and
registry-driven, so a family whose specification overrides 5.4 declares that
rather than being special-cased (`ai/rules/plugins.md`).

## Task list

- [ ] Re-read RFC 7606 Sections 5.4 and 6 and RFC 9552 Section 5.1 in full
- [ ] Determine per family whether 5.4 binds or the family's own spec overrides
- [ ] Decide where recognition happens, since neither current path parses a type
- [ ] Implement the discard at the owning layer, registry-driven, not a switch
- [ ] `RFC requirement: RFC7606-5.4-1 positive` tagged test, written red first
- [ ] Negative test: a family whose specification overrides 5.4 stays propagated
- [ ] Remove the `{gap}` annotation from `rfc/short/rfc7606.md` in the same commit
- [ ] `make ze-rfc-index`, then re-audit with `/ze-rfc-audit rfc7606`
- [ ] Update the RFC 7606 row in `docs/features/rfc-status.md`, which currently
      discloses the divergence
- [ ] Interop: confirm a real peer's behaviour, since this changes what Ze relays

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7606.md` - Sections 5.4 and 6
  → Constraint: 5.4's "unless the relevant specification specifies otherwise" clause is the whole design question. Section 6 warns that asymmetric dropping inside an AS causes long-lived loops.
- [ ] `rfc/short/rfc9552.md` - Section 5.1
  → Constraint: uses 5.4's own escape clause to REQUIRE preserve-and-propagate for BGP-LS. A blanket discard would violate this.
- [ ] `rfc/short/rfc7432.md` - EVPN
  → Constraint: never invoked 5.4's escape clause, which is why EVPN was the disclosed divergence rather than conformance by another route.

### Architecture Docs
- [ ] `ai/rules/rfc-compliance.md`
  → Decision: the 2026-07-20 divergence ruling is VOID. Do not cite it as authority.
- [ ] `ai/rules/plugins.md`
  → Constraint: per-family behaviour is registry-driven; no family spelled in a central switch.

**Key insights:** (minimal context to resume after compaction)
- Retention is EMERGENT, not written. There is no branch to flip.
- Neither retaining path parses a route type, so recognition must be introduced.
- A blanket discard breaks BGP-LS. The answer is per-family.

## Current Behavior (MANDATORY)

**Source files read:** (verified 2026-08-01 against the producing symbol)
- [ ] `internal/component/bgp/plugins/rib/storage/familyrib.go` - `FamilyRIB.Insert`: for a non-CIDR family calls `insertOpaque` with raw `nlriBytes`; never parses a route type.
- [ ] `internal/component/bgp/reactor/forward_body.go` - `buildFwdBody`: appends `peerWire.Payload()` verbatim on a ContextID match.
- [ ] `internal/component/bgp/plugins/nlri/evpn` - `ParseEVPN`: only non-test caller is the display/JSON decoder, which emits `parsed: false` rather than dropping.

**Behavior to preserve:** every family whose own specification overrides 5.4, BGP-LS foremost, keeps propagating unchanged.

**Behavior to change:** an unrecognized NLRI type in a family that 5.4 binds is discarded instead of retained and propagated.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

The central question is WHERE recognition happens, since neither retaining path
parses a route type today.

### Entry Point
- Peer-received UPDATE carrying a typed-family NLRI (EVPN, MVPN, BGP-LS, MUP,
  RTC), validated by `enforceRFC7606` then delivered to the RIB and the forward
  path.

### Transformation Path
1. Receive and validate. `enforceRFC7606` walks the attribute section. It does
   NOT parse NLRI route types.
2. RIB insert. `FamilyRIB.Insert` sends a non-CIDR family to `insertOpaque`,
   keyed on the raw NLRI bytes. **Proposed:** a recognition point here, or
   earlier, that a family can bind to.
3. Forward. `buildFwdBody` appends the payload verbatim on a ContextID match.
   A route discarded at step 2 never reaches this.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Session read goroutine to RIB | raw NLRI bytes, family-keyed | No |
| RIB to forward path | opaque entry, never type-parsed | No |
| Family registry to the discard decision | per-family predicate, registry-driven | No |

### Integration Points
- `internal/core/family` - the registry a per-family 5.4 binding should hang off,
  rather than a switch in a central package.
- `FamilyRIB.Insert` - the non-CIDR branch that currently retains.
- `rfc/short/rfc7606.md` and `docs/features/rfc-status.md` - both disclose the
  divergence and must be updated when it ends.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: no per-family spelling in a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | 5.4 does not bind every typed family; at least BGP-LS overrides it. | RFC 9552 invokes 5.4's escape clause explicitly. | A blanket discard is correct and simpler. | Re-read RFC 9552. | **confirmed**, with the citation corrected: the override is Section 5.**2**, not 5.1. Section 5.2 names "Section 5.4 (paragraph 2) of [RFC7606]" and requires unknown Link-State NLRI types to be preserved and propagated. Section 5.1 is the TLV-level analog, about TLVs inside an NLRI. |
| A-2 | Discarding is reachable without parsing every NLRI on the hot path. | Recognition can be a per-family registry predicate rather than a full parse. | The cost lands on the receive path and must be measured. | Design, then a receive-path benchmark. | **confirmed by construction.** The recognizer reads one byte per NLRI, and only for a family with a ruling. An UPDATE with no MP attribute never enters the check; one whose family has no ruling stops at a map read. No NLRI is parsed. |
| A-3 | A discard at the RIB would meet the requirement. | The spec's own Data Flow proposed the RIB insert as the recognition point. | The discard has to move upstream of the forward rails. | Read the forward path's producing symbols. | **broken.** `reactorForwardRS` (`reactor/forward_rs.go`) relays the RECEIVED wire with no RIB involvement, and `buildFwdBody` appends `peerWire.Payload()` verbatim. A RIB discard would have stopped installation and left the relay intact. Recorded in the Mistake Log. |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Over-broad discard drops BGP-LS or MVPN routes an operator relies on. | An interop scenario loses routes it used to relay. | Per-family opt-in, defaulting to today's behaviour until a family is ruled on. |
| R-2 | Section 6's asymmetric-dropping warning materialises: Ze drops inside an AS while its neighbours propagate. | Loops or black holes in an interop topology with mixed implementations. | This is the substance of the reversed 2026-07-20 ruling. Raise to Thomas if the design cannot avoid it. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Routes an operator expects relayed are silently dropped. Over-broad application breaks BGP-LS, where RFC 9552 requires propagation. Under-broad leaves the MUST unmet. |
| How is it reverted? | Single commit revert, but a peer that stopped receiving routes will have withdrawn them downstream, so the control-plane effect outlives the revert. |
| Who else touches this path? | `plan/learned/1322-wire-edit-0-umbrella.md` child 1 rewrites the receive-time attribute walk, and this spec adds a decision to that same path. Sequence them. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A peer sends a route with an unrecognized NLRI type in a family 5.4 binds | → | the discard decision | (fill during design) |
| A peer sends an unrecognized BGP-LS NLRI type | → | the override path, route still propagated | (fill during design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An unrecognized NLRI type in a family RFC 7606 Section 5.4 binds | The route is discarded, not installed and not propagated |
| AC-2 | An unrecognized NLRI type in a family whose specification overrides 5.4 (BGP-LS, RFC 9552 Section 5.1) | The route is retained and propagated, exactly as today |
| AC-3 | Every currently recognized NLRI type | Behavior is unchanged; no route that is relayed today stops being relayed |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Peers with a speaker sending an EVPN route type Ze does not recognize | receive, recognition, discard | (fill during design) |
| 2 | Runs BGP-LS with a peer sending a newer NLRI type | receive, override, propagate | (fill during design) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC7606Section54DiscardsUnrecognizedEVPNType` | `reactor/session_validation_nlritype_test.go` | AC-1, tagged `RFC requirement: RFC7606-5.4-1 positive` | written; blocked on the reactor package compiling |
| `TestRFC7606Section54PropagatesUnknownBGPLSType` | `reactor/session_validation_nlritype_test.go` | AC-2, tagged `RFC requirement: RFC7606-5.4-1 negative` | written; same block |
| `TestRFC7606Section54DropsMPReachWhenNothingSurvives` | `reactor/session_validation_nlritype_test.go` | no empty MP_REACH is relayed | written; same block |
| `TestRFC7606Section54LeavesConformingUpdateZeroCopy` | `reactor/session_validation_nlritype_test.go` | AC-3, the zero-copy relay survives | written; same block |
| `TestRFC7606Section54DiscardsUnrecognizedEVPNWithdrawal` | `reactor/session_validation_nlritype_test.go` | MP_UNREACH filtered on the same terms | written; same block |
| `TestRFC7606Section54LeavesUnruledFamilyUntouched` | `reactor/session_validation_nlritype_test.go` | R-1, an unruled family is untouched | written; same block |
| 11 registry and `Retain` tests | `core/bgp/nlri/nlritype/nlritype_test.go` | the registry default, the carve, ADD-PATH, malformed framing | PASS |
| `TestImplementedMatchesParseEVPN` | `plugins/nlri/evpn/rfc7606_test.go` | the recognized set cannot drift from `ParseEVPN` | PASS |
| 4 recognizer tests | `plugins/nlri/evpn/rfc7606_test.go` | the ruling is registered and reads the wire correctly | PASS |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rfc7606-54-discard-unrecognized-nlri` | `test/plugin/rfc7606-54-discard-unrecognized-nlri.ci` | a peer sends an unrecognized typed NLRI in a family 5.4 binds; the route is NOT relayed to any other peer | PASS, and mutation-verified |
| `rfc7606-54-bgpls-override-propagates` | `test/plugin/rfc7606-54-bgpls-override-propagates.ci` | a peer sends an unrecognized BGP-LS NLRI type; the route IS still relayed, per RFC 9552 Section 5.2 | PASS |

**Mutation-verified (2026-08-01).** `applyTypedNLRIDiscard` was made an early
`return wu, pathAttrs`, the daemon rebuilt, and the suite re-run:
`rfc7606-54-discard-unrecognized-nlri` flipped to FAIL with the receiver's wire
carrying `6304DEADBEEF`, and `rfc7606-54-bgpls-override-propagates` stayed PASS,
which is correct because it asserts the behaviour that already existed. The
mutation was reverted and both went green again.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| (fill during design) | `test/interop/scenarios/` | (fill) | a real peer's view of the changed relay behaviour | |

## Design (answered 2026-08-01)

### 1. Per-family binding, derived from the RFC text

| Family | Typed? | Does its own specification override 5.4? | Verdict |
|--------|--------|------------------------------------------|---------|
| `l2vpn/evpn` | yes, `[route-type:1][length:1][body]` (RFC 7432 Section 7.1) | **No.** RFC 7432 defines the framing and an "EVPN Route Types" IANA registry (Section 16) and states no deviation from RFC 7606. | **5.4 BINDS.** Discard a route whose type Ze does not implement (types 1..5 are implemented: RFC 7432 types 1-4, RFC 9136 type 5). |
| `bgp-ls/bgp-ls`, `bgp-ls/bgp-ls-vpn` | yes, `[NLRI type:2][total length:2][body]` | **Yes, in terms.** RFC 9552 **Section 5.2**: "this document deviates from the default handling behavior specified by Section 5.4 (paragraph 2) of [RFC7606] for Link-State address family. An implementation MUST handle unknown Link-State NLRI types as opaque objects and MUST preserve and propagate them." | **5.4 DOES NOT BIND.** Propagate unchanged. |
| every other family | not typed, or Ze registers no recognizer | -- | No recognizer registered, so nothing is discarded and today's behaviour is unchanged. |

**Correction to the spec's Required Reading and to `ls/unknown_type_skip_test.go`:** the
BGP-LS override is RFC 9552 **Section 5.2**, not Section 5.1. Section 5.1 states the
TLV-level analogue ("Unknown and unsupported types MUST be preserved and propagated
within both the NLRI and the BGP-LS Attribute"), which governs TLVs INSIDE an NLRI.
The NLRI-TYPE-level override that names RFC 7606 Section 5.4 is Section 5.2.

### 2. Where recognition happens, and why nowhere cheaper works

`Session.enforceRFC7606` (`internal/component/bgp/reactor/session_validation.go`),
the ingress RFC 7606 enforcement point, called from `session_read.go` `processMessage`.

The RIB is NOT the propagation gate, so a discard there meets half the requirement:

| Fact | Producing symbol |
|------|------------------|
| Relay is wire-driven, off the session read goroutine, with no RIB event involved | `Reactor.notifyMessageReceiver` (`reactor/reactor_notify.go`) calls `reactorForwardRS` with the RECEIVED `*ReceivedUpdate` |
| The route-server rail fans the received wire to every eligible peer and never filters by family | `reactorForwardRS` (`reactor/forward_rs.go`), whose peer loop tests only `forwardFacts` and `exportFilters` |
| The same-context branch appends the received payload verbatim | `buildFwdBody` (`reactor/forward_body.go`), `result.rawBodies = append(result.rawBodies, peerWire.Payload())` |
| For every family with no `nlrisplit` splitter (bgp-ls, mvpn, vpn, mup, rtc, flowspec) the RIB ALREADY installs nothing | `RIBManager.handleReceivedStructured` and `insertPoolNLRIs` both gate on `nlrisplit.Supported` |

So the RIB discards those families today and Ze relays them anyway. Only a point
upstream of `notifyMessageReceiver` governs both installation and propagation, and
`enforceRFC7606` is the one that already owns wire rewriting at that point: its
Section 3.g keep-first strip rebuilds the body with `message.RebuildUpdateBody` and
wraps it in a fresh `wireu.NewWireUpdate`, and `publishBase` runs last so the
attribute index is built over the final bytes.

### 3. Registry-driven, no family named in a central package

New core leaf `internal/core/bgp/nlri/nlritype`, sibling of `nlrisplit`:

| Symbol | Role |
|--------|------|
| `Recognizer func(nlriBytes []byte, addPath bool) bool` | reports whether one carved NLRI's type is one this speaker implements |
| `Register(fam, fn)` | called from the owning NLRI plugin's `init()` |
| `Retain(fam, data, addPath)` | carves with `nlrisplit`, drops the unrecognized, returns `data` unchanged (same backing array) when nothing was dropped |

`Get(fam) == nil` means the family has not been ruled on, so nothing is discarded.
That is R-1's mitigation expressed as the registry's default. The EVPN plugin
registers its recognizer beside `family.MustRegister(AFIL2VPN, SAFIEVPN, ...)`
(`plugins/nlri/evpn/types.go`), so the recognizer's presence tracks the family's
advertisement: compile the plugin out and Ze neither advertises `l2vpn/evpn` nor
owes 5.4 for it. BGP-LS registers nothing, and records why in its own package.

### 4. Receive-path cost

Gated three ways, so the common case pays two field stores and one map read:

1. Only an UPDATE carrying MP_REACH or MP_UNREACH enters the check. The validator's
   EXISTING attribute walk records the family and value range (no second walk, the
   discipline `RFC7606ValidationResult.PrefixSIDPresent` already states).
2. Only a family with a registered recognizer proceeds. One `RWMutex` read plus a map
   lookup. IPv6 unicast, the large MP family, stops here.
3. Only then is the NLRI section walked, reading the type byte that sits beside the
   length byte the framing walk already reads.

The rewrite (`make` plus copy) happens only when an unrecognized type is actually
present, the same tier as `ApplyAttrDiscard`'s rebuild. Zero-copy relay is preserved
for every UPDATE that carries none.

### 5. RFC 7606 Section 6, weighed rather than ignored

Section 6's warning is scoped to treat-as-withdraw of malformed ATTRIBUTES: "if an
UPDATE message received on an IBGP session is subjected to this treatment", and "When
a malformed attribute is indeed detected over an IBGP session". It never mentions
Section 5.4 or NLRI types. Reading a non-normative Operational Considerations
paragraph as defeating a normative MUST in the same document is not a legitimate
construction.

The IETF supplied the authorized escape for this exact concern: 5.4's own "unless the
relevant specification for that address family specifies otherwise". RFC 9552 used it
for BGP-LS, for the propagate-through-a-route-reflector reason. RFC 7432 did not use
it for EVPN. Honoring the clause per family IS the mitigation the RFC prescribes, so
the design does not trade one violation for another: BGP-LS keeps its RFC 9552 MUST.

The residual hazard is real and bounded: as a route reflector Ze will drop an EVPN
type its IBGP siblings accept. That is the behaviour the standard mandates.

### 6. Withdrawals

MP_UNREACH is filtered on the same terms. A withdrawal naming an unrecognized type
refers to a route Ze never installed and never relayed, so relaying the withdrawal
would be gratuitous. Symmetry is the point: one rule for both directions.

## Files to Modify

| File | Change |
|------|--------|
| `internal/component/bgp/message/rfc7606.go` | record MP family and NLRI value range on the existing walk |
| `internal/component/bgp/reactor/session_validation.go` | apply the Section 5.4 filter before the action switch |
| `internal/component/bgp/plugins/nlri/evpn/register.go` | register the EVPN recognizer |
| `internal/component/bgp/plugins/nlri/ls/unknown_type_skip_test.go` | correct the RFC 9552 section citation |
| `rfc/short/rfc7606.md` | remove the `{gap}` on `RFC7606-5.4-1` |
| `docs/features/rfc-status.md` | RFC 7606 row: one gap remains, not two |

## Files to Create

| File | Holds |
|------|-------|
| `internal/core/bgp/nlri/nlritype/nlritype.go` | the recognizer registry and `Retain` |
| `internal/core/bgp/nlri/nlritype/nlritype_test.go` | registry and `Retain` unit tests |
| `internal/component/bgp/reactor/session_validation_nlritype.go` | the ingress filter helper |
| `internal/component/bgp/reactor/session_validation_nlritype_test.go` | the tagged positive and negative tests |
| `test/plugin/rfc7606-54-discard-unrecognized-nlri.ci` | relay proof: unrecognized EVPN type is not relayed |
| `test/plugin/rfc7606-54-bgpls-override-propagates.ci` | relay proof: unrecognized BGP-LS type still is |

## Implementation Steps

1. **Phase 1:** `nlritype` registry plus `Retain`, red-first unit tests.
2. **Phase 2:** the ingress filter in `enforceRFC7606`, the EVPN recognizer, the tagged tests.
3. **Phase 3:** the two `.ci` relay tests, the ledger and the public-status updates.

## Design Insights

- The retention nobody wrote is harder to remove than a retention someone chose:
  there is no branch to flip, so compliance means ADDING a decision point to a
  path that is currently, deliberately, byte-opaque and zero-copy.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Implement full 5.4 compliance | keep the disclosed divergence | Thomas, 2026-08-01, re-asked with the full counter-argument in front of him. The 2026-07-20 ruling is void under the 2026-07-27 directive. |

## Mistake Log

### Wrong Assumptions

| Assumption | What was actually true | How it was caught | Cost |
|-----------|------------------------|-------------------|------|
| A-3: discarding at the RIB meets Section 5.4. The spec's own Data Flow proposed `FamilyRIB.Insert` as the recognition point. | The RIB is not the propagation gate. `reactorForwardRS` (`reactor/forward_rs.go`) fans the RECEIVED wire to every eligible peer off the session read goroutine with no RIB event involved, and `buildFwdBody` appends `peerWire.Payload()` verbatim. Worse, for bgp-ls, mvpn, vpn, mup, rtc and flowspec the RIB installs NOTHING today (every insert site gates on `nlrisplit.Supported`, and only 7 families have a splitter) while ze relays them anyway. A RIB discard would have been invisible on the wire. | Traced the receive-to-forward chain to its producing symbols before writing code. | None: caught in design. Had it not been, the spec would have shipped a change that passed a RIB-level test and left the wire behaviour untouched. |

### Wrong Citations Found

| Where | Said | Actually |
|-------|------|----------|
| This spec's Required Reading, `docs/features/rfc-status.md`, `rfc/short/rfc7606.md`'s `{gap}`, and `plugins/nlri/ls/unknown_type_skip_test.go` | RFC 9552 **Section 5.1** overrides RFC 7606 Section 5.4 for BGP-LS. | Section 5.1 is the TLV-level rule, about unknown TLVs INSIDE an NLRI. The NLRI-TYPE-level override, the one that names "Section 5.4 (paragraph 2) of [RFC7606]", is **Section 5.2**. Corrected in all four places. |

## Verification State (2026-08-01)

| Gate | Result |
|------|--------|
| `nlritype` unit tests | PASS (11), red-first: the file was written before the package existed |
| EVPN recognizer unit tests | PASS (5), including the anti-drift binding to `ParseEVPN` |
| Tagged reactor tests | PASS (6). Mutation-verified: an early return in `applyTypedNLRIDiscard` flipped 3 of the 6 RED |
| `test/plugin/rfc7606-54-*.ci` | PASS (2). Mutation-verified: the positive file flipped RED with `6304DEADBEEF` on the receiver's wire |
| `golangci-lint` over the changed packages | 0 issues |
| `make ze-rfc-check` | The two RFC7606-5.4-1 violations are CLEARED. Two violations remain, both `go vet` failures in packages another session is mid-refactor on |
| `./internal/component/bgp/...` with feature tags | 80 packages ok, 0 failing tests, 1 package blocked (see below) |

**Blocked on a concurrent session, not on this work.** `internal/component/bgp/reactor`
and `internal/component/bgp/plugins/role` do not type-check in the working tree:
`spec-wire-edit-2-edit-apply` is changing the filter-delta handler signature to
`*filterapi.AttrPlan` and its test files still call the old four-argument form
(`rfc8277_test.go`, `otc_test.go`). Nothing in this spec touches `filterapi`,
`forward_build.go` or any attr-mod handler.

The reactor evidence above was therefore taken through a `go test -overlay`
that maps only those in-flight test files to empty stubs. The overlay changes
no file in the repository. With it, the reactor package type-checks, the six
tagged tests pass, and the only two failures in the whole package are
`TestMergeInsertAscendingOrder` and `TestModifyPathZeroAlloc` in
`forward_build_merge_test.go`, which is the other session's own subject.

`make ze-rfc-check` and the full `ze-test-bgp` will go green on their own once
that refactor compiles. Neither needs a change here.

## Known Limitations

- Until this lands, `rfc/short/rfc7606.md` still carries the `{gap}` annotation
  and `docs/features/rfc-status.md` still discloses the divergence. Both are
  accurate descriptions of current behaviour and must not be edited early.

## RFC Documentation (Scope: protocol)

| RFC | Section | Requirement | Site |
|-----|---------|-------------|------|
| 7606 | 5.4 | discard routes with unrecognized NLRI types unless the family's specification says otherwise | (fill during design) |
| 9552 | 5.1 | BGP-LS invokes 5.4's escape clause and requires preserve-and-propagate | (fill during design) |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-3 all demonstrated
- [ ] `make ze-verify` passes
- [ ] `make ze-rfc-check` passes with the `{gap}` removed
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-fixit-rfc7606-5-4-discard-unrecognized-nlri.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm` this spec
