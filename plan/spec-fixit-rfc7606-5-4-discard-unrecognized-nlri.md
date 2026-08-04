# Spec: fixit-rfc7606-5-4-discard-unrecognized-nlri -- meet RFC 7606 Section 5.4

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 3/3 |
| Deferral shard | `-` |
| Updated | 2026-08-04 |

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
| AC-1 | An unrecognized NLRI type in a family RFC 7606 Section 5.4 binds (EVPN, MCAST-VPN, BGP-MUP) | The route is discarded, not installed and not propagated |
| AC-2 | An unrecognized NLRI type in a family whose specification overrides 5.4 (BGP-LS, RFC 9552 Section 5.2) | The route is retained and propagated, exactly as today |
| AC-3 | Every currently recognized NLRI type | Behavior is unchanged; no route that is relayed today stops being relayed |
| AC-4 | An UPDATE whose every route is discarded, so nothing is left to convey | The UPDATE is dropped whole, never rebuilt into an End-of-RIB marker and relayed |
| AC-5 | A typed NLRI section whose last route overruns its attribute | Session reset per Sections 5.3 and 3(j); the discard is never bypassed by breaking the framing |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Peers with a speaker sending an EVPN route type Ze does not recognize | receive, recognition, discard | (fill during design) |
| 2 | Runs BGP-LS with a peer sending a newer NLRI type | receive, override, propagate | (fill during design) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC7606Section54DiscardsUnrecognizedEVPNType` | `reactor/session_validation_nlritype_test.go` | AC-1, tagged `RFC requirement: RFC7606-5.4-1 positive` | written; ~~blocked on the reactor package compiling~~ **unblocked 2026-08-03, the package compiles (Correction block)** |
| `TestRFC7606Section54PropagatesUnknownBGPLSType` | `reactor/session_validation_nlritype_test.go` | AC-2, tagged `RFC requirement: RFC7606-5.4-1 negative` | written; same block |
| `TestRFC7606Section54DropsMPReachWhenNothingSurvives` | `reactor/session_validation_nlritype_test.go` | no empty MP_REACH is relayed | written; same block |
| `TestRFC7606Section54LeavesConformingUpdateZeroCopy` | `reactor/session_validation_nlritype_test.go` | AC-3, the zero-copy relay survives | written; same block |
| `TestRFC7606Section54DiscardsUnrecognizedEVPNWithdrawal` | `reactor/session_validation_nlritype_test.go` | MP_UNREACH filtered on the same terms | written; same block |
| `TestRFC7606Section54LeavesUnruledFamilyUntouched` | `reactor/session_validation_nlritype_test.go` | R-1, an unruled family is untouched | written; same block |
| 11 registry and `Retain` tests | `core/bgp/nlri/nlritype/nlritype_test.go` | the registry default, the carve, ADD-PATH, malformed framing | PASS |
| `TestImplementedMatchesParseEVPN` | `plugins/nlri/evpn/rfc7606_test.go` | the recognized set cannot drift from `ParseEVPN` | PASS |
| 4 recognizer tests | `plugins/nlri/evpn/rfc7606_test.go` | the ruling is registered and reads the wire correctly | PASS |
| `TestRFC7606Section54DropsMPUnreachOnlyUpdateRatherThanForgeEOR` | `reactor/session_validation_nlritype_test.go` | AC-4, no forged End-of-RIB; tagged `RFC requirement: RFC7606-5.4-1 positive` | PASS, mutation-verified |
| `TestRFC7606Section54SessionResetsUnparseableTypedNLRI` | `reactor/session_validation_nlritype_test.go` | AC-5, Sections 5.3 and 3(j) on a typed family; tagged | PASS, mutation-verified |
| `TestRFC7606Section54RewritesExtendedLengthMPReach` | `reactor/session_validation_nlritype_test.go` | the Extended Length re-encode path | PASS, mutation-verified |
| `TestRewriteMPNLRISectionsKeepsUnframeableTail` | `reactor/session_validation_nlritype_test.go` | no header octets lost when the attribute walk gives up | PASS, mutation-verified |
| `TestRFC7606Section54DiscardsUnrecognizedMVPNType` | `reactor/session_validation_nlritype_typed_test.go` | AC-1 for MCAST-VPN; tagged | PASS, mutation-verified |
| `TestRFC7606Section54DiscardsUnrecognizedMUPType` | `reactor/session_validation_nlritype_typed_test.go` | AC-1 for BGP-MUP, through its own four-octet envelope; tagged | PASS, mutation-verified |
| 6 MVPN recognizer tests | `plugins/nlri/mvpn/rfc7606_test.go` | the MCAST-VPN ruling, its registration, and the anti-drift binding; tagged | PASS, mutation-verified |
| 6 MUP recognizer tests | `plugins/nlri/mup/rfc7606_test.go` | the BGP-MUP ruling (architecture and route type), its registration; tagged | PASS, mutation-verified |
| 6 splitter tests | `core/bgp/nlri/nlrisplit/typelen_test.go` | MVPN and MUP framing, both Section 5.3 boundaries, registry presence | PASS, mutation-verified |
| 5 speaker-engine tests | `test/interop/speaker/test_engine.py` | `--family` negotiation and the EVPN route-type plugin, red and green | PASS |
| `TestRFC7606Section54FiltersWhenTheAttributeWalkIsAbandoned` | `reactor/session_validation_nlritype_bypass_test.go` | the closure-round BLOCKER: an abandoned Section 4 walk must not drop the MP location; tagged | PASS, mutation-verified |
| `TestRFC7606Section54ReadsTypedNLRIUnderAddPath` | `reactor/session_validation_nlritype_bypass_test.go` | the discard reads the route type past the RFC 7911 path identifier, and a valid ADD-PATH UPDATE is not session-reset; tagged | PASS, mutation-verified |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rfc7606-54-discard-unrecognized-nlri` | `test/plugin/rfc7606-54-discard-unrecognized-nlri.ci` | a peer sends an unrecognized typed NLRI in a family 5.4 binds; the route is NOT relayed to any other peer | PASS, and mutation-verified |
| `rfc7606-54-bgpls-override-propagates` | `test/plugin/rfc7606-54-bgpls-override-propagates.ci` | a peer sends an unrecognized BGP-LS NLRI type; the route IS still relayed, per RFC 9552 Section 5.2 | PASS |
| `rfc7606-54-discard-unrecognized-mup-nlri` | `test/plugin/rfc7606-54-discard-unrecognized-mup-nlri.ci` | the same relay proof through the BGP-MUP four-octet envelope, so `SplitMUP` and `mup.RecognizeNLRI` are exercised in a real daemon rather than against a stand-in ruling | PASS, mutation-verified |

**Mutation-verified (2026-08-01).** `applyTypedNLRIDiscard` was made an early
`return wu, pathAttrs`, the daemon rebuilt, and the suite re-run:
`rfc7606-54-discard-unrecognized-nlri` flipped to FAIL with the receiver's wire
carrying `6304DEADBEEF`, and `rfc7606-54-bgpls-override-propagates` stayed PASS,
which is correct because it asserts the behaviour that already existed. The
mutation was reverted and both went green again.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `53-rfc7606-54-typed-nlri-discard` | `test/interop/scenarios/53-rfc7606-54-typed-nlri-discard/` | raw injector (`ze-test peer`) plus the independent Python speaker (`test/interop/speaker/`) | an independent peer receives the assigned EVPN route type and never the unassigned one ze was sent in the same MP_REACH attribute | PASS, mutation-verified |

**Mutation evidence (2026-08-04).** `applyTypedNLRIDiscard` was made an early
`return wu, pathAttrs, typedNLRIKept`, the interop image was rebuilt, and the scenario
re-run. It flipped to FAIL with the speaker reporting `evpn-nlri: 2` and
`received EVPN route type 99, which is not assigned`, the relayed body carrying
`63080001AC1E00090002`. Restored and rebuilt, it reports `evpn-nlri: 1` and PASS. The
non-vacuity assertion carries its own weight in both directions: the count is 1 when the
fix is in, so a relay that delivered nothing would fail rather than pass quietly.

**Why not FRR, BIRD or GoBGP.** A conforming daemon discards the unassigned route type
itself, so its route table cannot tell "ze discarded it" from "the peer discarded it", and
GoBGP resets the session on an unknown type, which is the outcome Section 5.4 exists to
prevent rather than an observation of ze. The speaker reads the raw MP_REACH bytes ze put
on the wire, which is the only place the answer lives. It needed one harness change:
`engine.py` hardcoded an IPv4-unicast-only OPEN, and ze gates every announce on the
negotiated family set, so an EVPN check against the old engine would have passed on an
empty session. `--family AFI:SAFI` now names the families, defaulting to today's IPv4
unicast so no existing scenario changes.

## Goal Validation

One row per goal stated in the Task section, with the evidence that proves it beyond the
individual assertions (`ai/rules/interop-and-goal-validation.md`).

| Goal | Evidence |
|------|----------|
| Ze discards routes with unrecognized NLRI types in every family Section 5.4 binds | Three families carry a recognizer in a real build, each proven by its own registration test: `evpn.RecognizeNLRI` (`TestRecognizerIsRegisteredForEVPN`), `mvpn.RecognizeNLRI` (`TestRecognizerIsRegisteredForBothMVPNFamilies`), `mup.RecognizeNLRI` (`TestRecognizerIsRegisteredForBothMUPFamilies`). Each of the three envelopes is driven end to end through `enforceRFC7606` by a tagged reactor test. Section 5.4's own three examples are MCAST-VPN, MCAST-VPLS and EVPN; ze advertises no MCAST-VPLS family, so there is nothing to rule on for it. |
| The discard is on the propagation path, not only the RIB | `test/plugin/rfc7606-54-discard-unrecognized-nlri.ci` asserts the receiving peer's wire, and scenario `53-rfc7606-54-typed-nlri-discard` asserts the same thing from an independent peer ze does not control. Both were mutation-verified: the `.ci` file flips RED with `6304DEADBEEF` on the receiver's wire. |
| A family whose specification overrides 5.4 is untouched | `TestRFC7606Section54PropagatesUnknownBGPLSType` (tagged negative) and `test/plugin/rfc7606-54-bgpls-override-propagates.ci`. RFC 9552 Section 5.2 is quoted in the test. |
| No conforming UPDATE pays for the change | `TestRFC7606Section54LeavesConformingUpdateZeroCopy` asserts the SAME `*wireu.WireUpdate` and the same backing array come back when nothing is discarded. |
| The compliance fix introduces no new wire-visible defect | The two the review found are now themselves proven: AC-4 by `TestRFC7606Section54DropsMPUnreachOnlyUpdateRatherThanForgeEOR`, AC-5 by `TestRFC7606Section54SessionResetsUnparseableTypedNLRI`. Both mutation-verified: reverting the production line turns each RED. |

## Design (answered 2026-08-01)

### 1. Per-family binding, derived from the RFC text

| Family | Typed? | Does its own specification override 5.4? | Verdict |
|--------|--------|------------------------------------------|---------|
| `l2vpn/evpn` | yes, `[route-type:1][length:1][body]` (RFC 7432 Section 7.1) | **No.** RFC 7432 defines the framing and creates the "EVPN Route Types" IANA registry (Section 20, IANA Considerations) and states no deviation from RFC 7606. | **5.4 BINDS.** Discard a route whose type Ze does not implement (types 1..5 are implemented: RFC 7432 types 1-4, RFC 9136 type 5). |
| `ipv4/mvpn`, `ipv6/mvpn` | yes, `[route-type:1][length:1][body]` (RFC 6514 Section 4) | **No.** RFC 6514 Section 4 enumerates route types 1..5 (A-D routes) and 6..7 (C-multicast routes) and states no other handling for a type a speaker does not implement. Section 18 (IANA Considerations) creates no route-type registry and no deviation. | **5.4 BINDS.** RFC 7606 Section 5.4 names MCAST-VPN [RFC6514] as its FIRST example of a typed family. Discard a route type outside 1..7. |
| `ipv4/mup`, `ipv6/mup` | yes, `[arch:1][route-type:2][length:1][body]` (draft-ietf-bess-mup-safi Section 3.1) | **No.** The draft enumerates Architecture Type 1 (3gpp-5g) and Route Types 1..4 and invokes no escape clause. | **5.4 BINDS.** The Route Type specific encoding "depends on Architecture Type + Route Type", so the PAIR names the type. Discard anything outside arch 1 with route type 1..4. |
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
| `internal/component/bgp/message/rfc7606.go` | record MP family and NLRI value range on the existing walk; correct `validateMPNLRISyntax`'s doc, which read as though typed families were checked nowhere. **Closure round:** carry those two locations out of the four RFC 7606 Section 4 structural early returns as well, which is the BLOCKER below |
| `internal/component/bgp/reactor/session_validation.go` | apply the Section 5.4 filter before the action switch, and act on its two whole-UPDATE outcomes |
| `internal/component/bgp/reactor/session_validation_nlritype.go` | the three fixes the review found: drop an emptied UPDATE, session-reset unparseable framing, keep the header octets on an unframeable tail |
| `internal/component/bgp/plugins/nlri/evpn/register.go` | register the EVPN recognizer |
| `internal/component/bgp/plugins/nlri/evpn/rfc7606.go` | RFC 7432's IANA registry is Section 20, not Section 16 |
| `internal/component/bgp/plugins/nlri/mvpn/register.go` | register the MCAST-VPN recognizer for both AFIs |
| `internal/component/bgp/plugins/nlri/mup/register.go` | register the BGP-MUP recognizer for both AFIs |
| `internal/core/bgp/nlri/nlrisplit/register.go` | bind the MVPN and MUP splitters, without which no recognizer may register |
| `internal/core/bgp/nlri/nlrisplit/evpn.go` | delegate to the shared type-and-length walk |
| `internal/component/bgp/plugins/nlri/ls/unknown_type_skip_test.go` | correct the RFC 9552 section citation |
| `test/interop/speaker/engine.py` | `--family AFI:SAFI` so a speaker can negotiate a typed family; default unchanged |
| `test/interop/speaker/test_engine.py` | red and green fixtures for the flag and the new plugin |
| `rfc/short/rfc7606.md` | remove the `{gap}` on `RFC7606-5.4-1` |
| `docs/features/rfc-status.md` | RFC 7606 row: one gap remains, not two. **Still owed at closure:** the row names EVPN and BGP-LS as the two poles and must now also name MCAST-VPN and BGP-MUP as bound families. Not edited on 2026-08-04 because the working-tree copy carries another session's uncommitted RFC 7296 row, and staging it would have committed their line. |

## Files to Create

| File | Holds |
|------|-------|
| `internal/core/bgp/nlri/nlritype/nlritype.go` | the recognizer registry and `Retain` |
| `internal/core/bgp/nlri/nlritype/nlritype_test.go` | registry and `Retain` unit tests |
| `internal/component/bgp/reactor/session_validation_nlritype.go` | the ingress filter helper |
| `internal/component/bgp/reactor/session_validation_nlritype_test.go` | the tagged positive and negative tests |
| `test/plugin/rfc7606-54-discard-unrecognized-nlri.ci` | relay proof: unrecognized EVPN type is not relayed |
| `test/plugin/rfc7606-54-bgpls-override-propagates.ci` | relay proof: unrecognized BGP-LS type still is |
| `internal/core/bgp/nlri/nlrisplit/typelen.go` | the one type-and-length framing walk EVPN, MVPN and MUP share |
| `internal/core/bgp/nlri/nlrisplit/mvpn.go`, `mup.go` | the two new splitters, and where their framing is cited |
| `internal/core/bgp/nlri/nlrisplit/typelen_test.go` | framing and Section 5.3 boundary tests for both |
| `internal/component/bgp/plugins/nlri/mvpn/rfc7606.go`, `mup/rfc7606.go` | the two new Section 5.4 rulings, beside the families they bind |
| `internal/component/bgp/plugins/nlri/mvpn/rfc7606_test.go`, `mup/rfc7606_test.go` | the rulings, their registration, and the anti-drift binding |
| `internal/component/bgp/reactor/session_validation_nlritype_typed_test.go` | the MVPN and MUP wire filters, driven from `enforceRFC7606` |
| `test/interop/speaker/plugins/no_unrecognized_evpn_type.py` | the independent peer's judgement on the relayed EVPN route types |
| `test/interop/scenarios/53-rfc7606-54-typed-nlri-discard/` | the interop scenario: `ze.conf`, `inject.msg`, `inject-args`, `speaker-args`, `check.py` |
| `internal/component/bgp/reactor/session_validation_nlritype_bypass_test.go` | the two closure-round tests: the abandoned Section 4 walk, and ADD-PATH on a typed family |
| `test/plugin/rfc7606-54-discard-unrecognized-mup-nlri.ci` | relay proof for the BGP-MUP envelope |

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
| `plugins/nlri/evpn/rfc7606.go` | RFC 7432 creates the "EVPN Route Types" IANA registry in **Section 16**. | Section 16 is "Multicast and Broadcast". The registry is created in **Section 20**, IANA Considerations. Corrected 2026-08-04. |

### What the 2026-08-03 review found, and what it cost

| Finding | Root cause | How it was caught |
|---------|-----------|-------------------|
| Section 5.4 was skipped entirely when the same UPDATE also earned treat-as-withdraw, so `SynthesizeWithdrawFamilies` relayed the unrecognized types inside the synthesized MP_UNREACH. | The gate in `enforceRFC7606` excluded treat-as-withdraw on the reading that a withdrawn route needs no discard. The withdrawal carries the NLRI bytes, so it does. Section 6 of this design claimed symmetry for withdrawals; it was untrue on that one path. | Round-1 independent review of the fix pass. |
| Registering the MVPN and MUP splitters changed RIB behaviour with nothing asserting the result. | `nlrisplit.Supported` gates RIB installation as well as the Section 5.4 carve, and the two `.ci` files assert relay only. | Round-1 independent review. |
| `nlritype.ResetForTest` is fail-open as a teardown: it leaves the registry EMPTY, and the reactor test binary really does link real NLRI plugins. | A later test would find no recognizer for a family the daemon rules on, its filter would do nothing, and it would pass proving nothing. | Round-1 independent review. |
| `TestRewriteMPNLRISectionsKeepsUnframeableTail` named an attribute code its fixture did not contain, so `findEdit` never matched and the test was identical with no edits at all. | Copy of the wrong constant. | Round-1 independent review. |
| The speaker-engine tests were gated by nothing, and this spec added seven more to them. | No `pythonTestRoots` entry covered `test/interop/speaker`, the file is named `test_engine.py` while the glob was `*_test.py`, and it handed itself to a pytest that is not installed. | Round-1 independent review. |
| The MUST was proven for EVPN alone, while RFC 7606 Section 5.4's own first example, MCAST-VPN, still retained and relayed unrecognized types. So did BGP-MUP. | `nlrisplit/register.go` bound splitters for unicast, multicast, EVPN and labeled only, and `nlritype.Register` refuses a family with no splitter, so no recognizer for MVPN or MUP could have been registered. The gap was structural and invisible from the EVPN side. | Independent review reading Section 5.4's example list against the registry. |
| An MP_UNREACH-only UPDATE whose withdrawals were all unrecognized was rebuilt into a forged End-of-RIB and relayed. | `rewriteMPNLRISections` drops an attribute whose NLRI all went, leaving no attributes; `RebuildUpdateBody` then emits four zero octets, which `Update.IsEndOfRIB` and `WireUpdate.IsEOR` both read as an EoR. Every existing test used `mpReachAttrs`, which prepends ORIGIN and AS_PATH, so the section never emptied and the shape was unreachable. | Independent review, verified against the producing functions. |
| A peer could bypass the Section 5.4 MUST by appending one truncated NLRI. | `typedNLRIEdit` relayed everything on a split error, justified by a comment claiming Section 5.3 had already run. It had not: `validateMPNLRISyntax` returns nil for every family outside IPv4/IPv6 unicast and multicast, as its own doc comment says. | Independent review reading the comment against the function it cited. |
| `rewriteMPNLRISections` lost 2 to 4 header octets on every abandoned attribute walk. | `pos` had already stepped past flags and type code when `break` fired, and the trailing copy-through starts at `pos`. | Independent review. |

## Verification State (2026-08-04, review-fix pass)

| Gate | Result |
|------|--------|
| `nlrisplit` (MVPN, MUP, shared walk) | PASS |
| `nlritype` | PASS |
| `plugins/nlri/mvpn`, `plugins/nlri/mup`, `plugins/nlri/evpn` | PASS |
| Tagged and untagged reactor Section 5.4 tests | PASS (12) |
| `test/interop/speaker/test_engine.py` | PASS (17), run without pytest via a plain driver |
| `plugins/rib`, `plugins/rib/storage`, `plugins/rib/pool` | PASS, including the new typed-family installation test |
| `make ze-plugin-test` | PASS 590/590, including both `rfc7606-54-*.ci` |
| `make ze-rfc-check` | exit 0; `interop/nightly` evidence 3 -> 4 |
| `go test -run TestPythonUnitTests ./scripts/dev` | discovers and runs `test/interop/speaker/test_engine.py`, which no gate reached before |
| Interop scenario `53-rfc7606-54-typed-nlri-discard` | PASS, mutation-verified |
| Mutation verification | 12 of 12 mutations turned the owning test RED. See the table below |

**Mutation evidence (2026-08-04).** Each production line was reverted in place, the owning
tests were re-run, and the file was restored from a byte copy:

| Mutation | Test that turned RED |
|----------|----------------------|
| `outcome = typedNLRIEmptied` removed | `TestRFC7606Section54DropsMPUnreachOnlyUpdateRatherThanForgeEOR`, `TestRFC7606Section54DropsMPReachWhenNothingSurvives` |
| `typedNLRIEdit` returns true on a split error | `TestRFC7606Section54SessionResetsUnparseableTypedNLRI` |
| `Register(fam, SplitMVPN)` removed | `TestRFC7606Section54DiscardsUnrecognizedMVPNType`, `TestMVPNAndMUPAreRegistered` |
| `SplitMUP` reads the length octet at offset 1 | `TestRFC7606Section54DiscardsUnrecognizedMUPType`, `TestSplitMUPCarvesOnLengthAtOffsetThree`, `TestSplitMUPBoundaries` |
| MVPN recognizer registered as nil | `TestRecognizerIsRegisteredForBothMVPNFamilies` |
| MUP recognizer registered as nil | `TestRecognizerIsRegisteredForBothMUPFamilies` |
| `pos = attrStart` rewind removed | `TestRewriteMPNLRISectionsKeepsUnframeableTail` |
| Extended Length re-encode branch disabled | `TestRFC7606Section54RewritesExtendedLengthMPReach` |
| Section 5.4 gate skips treat-as-withdraw again | `TestRFC7606Section54FiltersTreatAsWithdrawSynthesis` |
| `Register(fam, SplitMVPN)` removed, judged at the RIB | `TestHandleReceived_PoolStorage_StoresTypedFamilies` |
| `rewriteMPNLRISections` copies the attribute through instead of editing it | `TestRewriteMPNLRISectionsKeepsUnframeableTail` |
| `SnapshotForTest`'s restore clears instead of restoring | `TestSnapshotForTestRestoresRegistrations` |

**Consequence worth stating.** Registering the MVPN and MUP splitters also makes
`nlrisplit.Supported` true for those families, and that predicate gates RIB installation
(`RIBManager.handleReceivedStructured`, `insertPoolNLRIs`, `rib.go`). Ze therefore now
installs MCAST-VPN and BGP-MUP routes as opaque RIB entries, the way it already installs
EVPN. That is a real behaviour change beyond the discard, and it is unavoidable: judging a
route type requires carving the section into individual NLRIs, and one registry owns that
carve (`ai/rules/no-layering.md`).

## Verification State (2026-08-01)

| Gate | Result |
|------|--------|
| `nlritype` unit tests | PASS (11), red-first: the file was written before the package existed |
| EVPN recognizer unit tests | PASS (5), including the anti-drift binding to `ParseEVPN` |
| Tagged reactor tests | PASS (6). Mutation-verified: an early return in `applyTypedNLRIDiscard` flipped 3 of the 6 RED |
| `test/plugin/rfc7606-54-*.ci` | PASS (2). Mutation-verified: the positive file flipped RED with `6304DEADBEEF` on the receiver's wire |
| `golangci-lint` over the changed packages | 0 issues |
| `make ze-rfc-check` | The two RFC7606-5.4-1 violations are CLEARED. ~~Two violations remain, both `go vet` failures in packages another session is mid-refactor on~~ **corrected 2026-08-03: those two packages now `go vet` clean, so re-run the gate rather than quoting this row** |
| `./internal/component/bgp/...` with feature tags | 80 packages ok, 0 failing tests, ~~1 package blocked (see below)~~ **corrected 2026-08-03: nothing is blocked, see the Correction block** |

> **CORRECTION 2026-08-03 (bookkeeping audit): the tree-contention block is GONE. The
> paragraphs below describe 2026-08-01 and are kept as history. Do not reach for the
> overlay, and do not wait on another session.**
>
> `spec-wire-edit-2-edit-apply` landed and its spec is closed and removed from `plan/`.
> `rfc8277_test.go` and `otc_test.go` both call the `func(*AttrPlan)` form now, and their
> comments say so. Both packages type-check with the full feature tags:
>
> ```
> $ TAGS="ze_core $(awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt | sort -u | tr '\n' ' ')"
> $ go vet -tags "$TAGS" ./internal/component/bgp/reactor/ ./internal/component/bgp/plugins/role/
> exit=0
> ```
>
> So the reactor evidence can be re-taken directly, with no overlay. The two failures the
> paragraph below attributes to the other session are its subject and are no longer this
> spec's excuse: re-run `make ze-test-pkg PKG=./internal/component/bgp/reactor` and read
> what it says now.

**~~Blocked on a concurrent session, not on this work.~~** `internal/component/bgp/reactor`
and `internal/component/bgp/plugins/role` ~~do not type-check in the working tree:
`spec-wire-edit-2-edit-apply` is changing the filter-delta handler signature to
`*filterapi.AttrPlan` and its test files still call the old four-argument form
(`rfc8277_test.go`, `otc_test.go`). Nothing in this spec touches `filterapi`,
`forward_build.go` or any attr-mod handler.~~

~~The reactor evidence above was therefore taken through a `go test -overlay`
that maps only those in-flight test files to empty stubs. The overlay changes
no file in the repository. With it, the reactor package type-checks, the six
tagged tests pass, and the only two failures in the whole package are
`TestMergeInsertAscendingOrder` and `TestModifyPathZeroAlloc` in
`forward_build_merge_test.go`, which is the other session's own subject.~~

~~`make ze-rfc-check` and the full `ze-test-bgp` will go green on their own once
that refactor compiles. Neither needs a change here.~~

## Known Limitations

- **Deferral shard row corrected to `-` on 2026-08-03 (bookkeeping audit).** It named
  `plan/deferrals/spec-fixit-rfc7606-5-4-discard-unrecognized-nlri.md`, which never existed
  and carries the `spec-` prefix a shard name does not take. This spec defers nothing: the
  row below is a sequencing note about two files that must not be edited early, not
  postponed work.
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

---

## Implementation Summary

### What Was Implemented

- A per-family Section 5.4 recognizer registry, `internal/core/bgp/nlri/nlritype`
  (`Register`, `Get`, `Retain`). `Get` returning nil means the family has not been
  ruled on, so nothing is discarded. That default is R-1's mitigation.
- One shared type-and-length framing walk, `splitTypeLength`
  (`internal/core/bgp/nlri/nlrisplit/typelen.go`), and the three splitters over it:
  `SplitEVPN`, `SplitMVPN` (RFC 6514 Section 4), `SplitMUP`
  (draft-ietf-bess-mup-safi Section 3.1, a 4-octet header with the length octet last).
- Three rulings, each beside the family it binds: `evpn.RecognizeNLRI`,
  `mvpn.RecognizeNLRI`, `mup.RecognizeNLRI`. BGP-LS registers none, because RFC 9552
  Section 5.2 requires preserve-and-propagate. Nothing central names a family.
- The ingress filter, `applyTypedNLRIDiscard`
  (`internal/component/bgp/reactor/session_validation_nlritype.go`), applied by
  `Session.enforceRFC7606` (`session_validation.go`) before the action switch and
  after the Section 3.g duplicate strip. It returns the same `*wireu.WireUpdate` and
  the same backing array when nothing is discarded, so the zero-copy relay survives.
- Two whole-UPDATE outcomes acted on by `enforceRFC7606`: `typedNLRIEmptied` drops the
  UPDATE rather than relay a four-zero-octet body an RFC 4724 receiver reads as
  End-of-RIB, and `typedNLRIUnparseable` session-resets per Sections 5.3 and 3(j).

### Bugs Found/Fixed

Four review BLOCKERs, all fixed in `a67205a61`:

1. The MUST was proven for EVPN alone while MCAST-VPN, Section 5.4's own first example,
   still relayed unrecognized types. `nlritype.Register` refuses a family with no
   splitter, so the gap was structural and invisible from the EVPN side. Covered by
   `TestRFC7606Section54DiscardsUnrecognizedMVPNType` and `...MUPType`.
2. An MP_UNREACH-only UPDATE whose withdrawals were all unrecognized was rebuilt into a
   forged End-of-RIB. `updateCarriesNoRoutes` now reports it and the UPDATE is dropped.
   Covered by `TestRFC7606Section54DropsMPUnreachOnlyUpdateRatherThanForgeEOR`.
3. A peer could bypass the MUST by appending one truncated NLRI: `typedNLRIEdit` relayed
   everything on a split error, justified by a comment claiming Section 5.3 had already
   run. `validateMPNLRISyntax` returns nil for every typed family. Now a session reset.
   Covered by `TestRFC7606Section54SessionResetsUnparseableTypedNLRI`.
4. No interop scenario. `53-rfc7606-54-typed-nlri-discard` now proves it from a peer ze
   does not control, mutation-verified in both directions.

Found by the closure round's independent review, and fixed here:

5. **BLOCKER. One octet still bypassed the Section 5.4 MUST.**
   `ValidateUpdateRFC7606AddPath` (`internal/component/bgp/message/rfc7606.go`) has four
   RFC 7606 Section 4 structural early returns that abandon the attribute walk. Each built
   a fresh result and dropped the `MPReachNLRI`/`MPUnreachNLRI` locations the walk had
   ALREADY recorded. Put MP_REACH first, append one attribute whose declared length
   overruns the section, and: the walk returns treat-as-withdraw with a zero location,
   `Session.typedNLRIEdit` finds `!loc.Present` and filters nothing, and
   `message.mpUnreachAttrList` (`message/rfc7606_withdraw.go`) then rescans the attributes
   with its OWN iterator, converts the untouched MP_REACH into an MP_UNREACH carrying the
   same unrecognized NLRI, and `processMessage` dispatches it to every peer that negotiated
   the family. Structurally the same shape as finding 3, reached by a different door.
   `TestRFC7606Section54FiltersTreatAsWithdrawSynthesis` could not see it: its malformed
   ORIGIN goes through `recordError` and `continue`, so that walk runs to completion and
   the location survives anyway. Fixed by carrying the locations out of all four returns.
   Covered by `TestRFC7606Section54FiltersWhenTheAttributeWalkIsAbandoned`,
   mutation-verified.
6. **ISSUE. Two peer-driven `Info` log lines, one per UPDATE.** `typedNLRIEdit` and the
   `typedNLRIEmptied` branch of `enforceRFC7606` both logged at Info on bytes a peer
   chooses, on the receive goroutine. Every sibling RFC 7606 outcome logs at Debug.
   Demoted, with the reason in a comment.
7. **ISSUE. No tagged test drove ADD-PATH through a typed family.** The discard made a
   wrong ADD-PATH verdict newly expensive: the walk reads the path identifier as a type
   and length, overruns, and now session-resets where it used to relay.
   `TestRFC7606Section54ReadsTypedNLRIUnderAddPath` holds the composed path down,
   mutation-verified against `head = 4` in `splitTypeLength`.
8. **ISSUE. BGP-MUP had no functional test.** The relay path is family-generic and proven
   for EVPN, but the per-family splitter and recognizer pair was proven only against a
   stand-in ruling in Go. `test/plugin/rfc7606-54-discard-unrecognized-mup-nlri.ci` now
   drives the four-octet envelope through a real daemon, mutation-verified: with the
   discard disabled the receiver's wire carries `01000504CAFEBABE`.

Found at closure and fixed here: `a67205a61` falsified every published claim that
explained a gap by naming the splitter ze did NOT register for SAFI 85. Four `{gap}`
reasons in `rfc/short/draft-ietf-bess-mup-safi.md` and the BGP-MUP row of
`docs/features/rfc-status.md` all read "nlrisplit registers no splitter for SAFI 85 ...
so a received MUP route is neither stored nor deleted on withdrawal".
`removePoolNLRIs` (`internal/component/bgp/plugins/rib/rib.go`) now splits and removes
it. `make ze-rfc-check` cannot see this: the ids and their classification never moved,
only the prose under them stopped being true.

### Documentation Updates

- `docs/features/rfc-status.md`, RFC 7606 row: names all three bound families and their
  recognizers, and states the RIB consequence of the new splitters.
- `docs/features/rfc-status.md`, RFC 6514 row: the Section 4 split, the opaque Adj-RIB-In
  storage it enables, and the Section 5.4 ruling.
- `docs/features/rfc-status.md`, draft-ietf-bess-mup-safi row: the false
  "registers no splitter" claim replaced with what the code now does; the four
  routing-instance obligations stay open, with the true reason.
- `rfc/short/draft-ietf-bess-mup-safi.md`: the same correction under
  `DRAFT-IETF-BESS-MUP-SAFI-3.3.3-2`, `-3.3.6-2`, `-3.3.9-1`, `-3.3.9-2`. All four keep
  `{gap}`, so the Remaining count on the public page is unchanged.
- `ai/RFC-REQUIREMENTS.md` regenerated (`make ze-rfc-index`): 4 lines, the four reasons.
- `plan/learned/1334-...md`: the new consequence and the reusable rule.
- `make ze-doc-test` PASSED.

### Deviations from Plan

- The spec's Data Flow proposed the RIB as the recognition point. It is not the
  propagation gate, so the filter went to `enforceRFC7606` instead. Recorded as A-3.
- `docs/features/rfc-status.md` was owed from the implementation commit and is paid here.
- `plan/learned/1335-cache-consumer-declared-before-reactor.md` gained the `## Files`
  section it was missing. Not this spec's summary, but its absence held
  `make ze-doc-test` red at 323 dead references against a ceiling of 322, and this
  closure needs that gate.
- The closure round was not bookkeeping. Its independent review found a fifth
  wire-visible way past the Section 5.4 MUST, structurally the same as one round 1
  had already closed, and three coverage gaps. Code, a new test file and a new `.ci`
  therefore land in the closure commit, not only prose.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Discard unrecognized typed NLRI per Section 5.4 | Done | `applyTypedNLRIDiscard`, `session_validation_nlritype.go` | Applied by `enforceRFC7606` before the action switch |
| Per family, honoring the escape clause | Done | `nlritype.Get`, `internal/core/bgp/nlri/nlritype/nlritype.go` | Unruled family is untouched; BGP-LS registers nothing |
| Prove it with an `RFC requirement:` tagged test | Done | `ai/RFC-REQUIREMENTS.md:4153` | 10 positive tags, 2 negative, no `{gap}` |
| Remove the `{gap}` from `rfc/short/rfc7606.md` | Done | `rfc/short/rfc7606.md:333` | One gap left in that summary, the §5.1 ordering |
| Update `docs/features/rfc-status.md` | Done | RFC 7606, RFC 6514 and MUP rows | Paid at closure, see Deviations |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestRFC7606Section54DiscardsUnrecognizedEVPNType`, `...MVPNType`, `...MUPType` | One per bound family, each driven through `enforceRFC7606` |
| AC-2 | Done | `TestRFC7606Section54PropagatesUnknownBGPLSType`, `test/plugin/rfc7606-54-bgpls-override-propagates.ci` | RFC 9552 §5.2 quoted in the test |
| AC-3 | Done | `TestRFC7606Section54LeavesConformingUpdateZeroCopy` | Same pointer and same backing array returned |
| AC-4 | Done | `TestRFC7606Section54DropsMPUnreachOnlyUpdateRatherThanForgeEOR` | `updateCarriesNoRoutes` reports `typedNLRIEmptied` |
| AC-5 | Done | `TestRFC7606Section54SessionResetsUnparseableTypedNLRI` | `typedNLRIEdit` returns false, `enforceRFC7606` resets |
| AC-1 (second door) | Done | `TestRFC7606Section54FiltersWhenTheAttributeWalkIsAbandoned` | the discard is not bypassable by abandoning the Section 4 walk after MP_REACH |
| AC-1 (ADD-PATH) | Done | `TestRFC7606Section54ReadsTypedNLRIUnderAddPath` | the route type is read past the RFC 7911 path identifier |
| AC-1 (BGP-MUP, through the daemon) | Done | `test/plugin/rfc7606-54-discard-unrecognized-mup-nlri.ci` | the four-octet envelope proven on a receiving peer's wire |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| 14 reactor Section 5.4 tests | Done | `session_validation_nlritype{,_typed,_diag}_test.go` | All green, 2026-08-04 |
| `nlritype` registry and `Retain` | Done | `internal/core/bgp/nlri/nlritype/` | Green |
| EVPN, MVPN, MUP recognizer tests | Done | `plugins/nlri/{evpn,mvpn,mup}/rfc7606_test.go` | Green, each with an anti-drift binding |
| 6 splitter tests | Done | `internal/core/bgp/nlri/nlrisplit/typelen_test.go` | Green |
| 2 `.ci` relay tests | Done | `test/plugin/rfc7606-54-*.ci` | PASS in `make ze-plugin-test` 590/590 (2026-08-04) |
| Interop `53-rfc7606-54-typed-nlri-discard` | Done | `test/interop/scenarios/` | PASS, mutation-verified both directions |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Every row of Files to Create | Done | 21 of 21 present, `ls` output in Pre-Commit Verification |
| Every row of Files to Modify | Done | All in `a67205a61` except `docs/features/rfc-status.md`, paid here |

### Audit Summary
- **Total items:** 18
- **Done:** 18
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (recognition point moved from the RIB to `enforceRFC7606`, see A-3)

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| No shard. Spec metadata carries `Deferral shard: -`, corrected 2026-08-03 | done | `ls plan/deferrals/` holds no shard for this stem, and no shard row names `rfc7606-5-4` or `RFC7606-5.4` |
| Sequencing note: `rfc/short/rfc7606.md` and `docs/features/rfc-status.md` must not be edited before the behavior lands | done | Both edited now the behavior is in: `rfc/short/rfc7606.md:333` carries no `{gap}`, and the three `docs/features/rfc-status.md` rows are updated |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-rfc7606-5-4-discard-unrecognized-nlri-c4c78ddb-c47b-4f1a-a85d-5911d7c65455.md`, 21 files pinned, verdict clean |
| `review_gate.py check` | OK, 8 code files, hashes match |
| RFC audit | `/ze-rfc-audit rfc7606` run twice by a separate agent. `RFC7606-5.4-1` enforced over 15 units, `RFC7606-3.g-1` over 7, `RFC7606-5.2-1` over 7. `make ze-rfc-check` exit 0 |
| Reviewer lenses used | Round 1: logic + wiring + RFC compliance; security + allocation + doc/ledger accuracy. Round 2: the fixes round 1 produced, and what they touched |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | The four RFC 7606 Section 4 structural early returns drop the MP NLRI location the same walk already recorded, so MP_REACH followed by one over-long attribute bypasses the Section 5.4 MUST and the unrecognized type rides out inside the synthesized withdrawal | `ValidateUpdateRFC7606AddPath`, `internal/component/bgp/message/rfc7606.go`; exploited through `mpUnreachAttrList`, `message/rfc7606_withdraw.go` | all four returns now carry `MPReachNLRI`/`MPUnreachNLRI`; `TestRFC7606Section54FiltersWhenTheAttributeWalkIsAbandoned`, mutation-verified |
| 2 | ISSUE | Two peer-driven `Info` log lines, one per UPDATE, on the receive goroutine; every sibling RFC 7606 outcome logs at Debug | `typedNLRIEdit` (`session_validation_nlritype.go`), the `typedNLRIEmptied` branch of `enforceRFC7606` (`session_validation.go`) | demoted to Debug with the reason in a comment |
| 3 | ISSUE | No tagged test drives `addPath=true` through `enforceRFC7606` for a typed family, and the discard made a wrong ADD-PATH verdict newly session-fatal | `typedNLRIEdit`, `splitTypeLength` | `TestRFC7606Section54ReadsTypedNLRIUnderAddPath`, mutation-verified against `head = 4` |
| 4 | ISSUE | BGP-MUP and MCAST-VPN have no functional test; the per-family splitter and recognizer pair is proven only against a stand-in ruling | `SplitMUP`, `mup.RecognizeNLRI` | `test/plugin/rfc7606-54-discard-unrecognized-mup-nlri.ci`, mutation-verified |
| 5 | ISSUE | `a67205a61` falsified four `{gap}` reasons and one public row that explained a gap by naming the splitter ze did NOT register for SAFI 85 | `rfc/short/draft-ietf-bess-mup-safi.md`, `docs/features/rfc-status.md` | reasons rewritten to the true mechanism, classifications unchanged; `make ze-rfc-check` and `make ze-doc-test` green |
| 6 | BLOCKER (round 2) | Fixing finding 1 exposed the next layer: the Section 3.g duplicate-MP check ALSO sat after the loop, so two MP_REACH attributes plus a framing error skipped the mandated NOTIFICATION, and `mpReachNLRI` (the LAST seen) then disagreed with `attribute.AttrFind` (the FIRST), applying one family's Section 5.4 recognizer to another family's NLRI | `ValidateUpdateRFC7606AddPath`, `internal/component/bgp/message/rfc7606.go` | the Section 3.g verdict is now reached the moment the duplicate is SEEN, inside the loop, so no exit added later can outrun it; `TestRFC7606Section3gDuplicateMPBeatsAnAbandonedWalk` and `...MPUnreachResetsWithRoutesPresent`, both mutation-verified |
| 7 | BLOCKER (round 2) | The same four returns also skipped the RFC 7606 Section 5.2 escalation, so an UPDATE with attributes, no reachable NLRI and a framing error got treat-as-withdraw. `SynthesizeWithdrawFamilies` produces no bodies for such a body, so ze consumed it and told the peer nothing where the RFC requires a NOTIFICATION | same function | one `structuralError` helper now owns all four returns and pays Sections 5.2 and 5.4 together; `TestRFC7606Section52EscalatesWhenTheAttributeWalkIsAbandoned` (positive, mutation-verified) and `...LeavesAnUpdateWithNLRIAlone` (negative) |
| 8 | ISSUE (round 2) | Two comments describing the malformed-tail copy and `updateCarriesNoRoutes`'s unparseable branch as unreachable became false the moment the abandoned walk started reporting MP locations | `rewriteMPNLRISections`, `updateCarriesNoRoutes` (`session_validation_nlritype.go`) | both rewritten to say the path is live and why copying the tail verbatim keeps the rebuild honest |
| 9 | NOTE (round 2) | The log-demotion comment claimed every sibling RFC 7606 outcome logs at Debug, contradicted by the Warn 16 lines above it | `typedNLRIEdit` | comment corrected: the louder levels are kept for outcomes a peer cannot repeat cheaply |

NOTEs recorded and not blocking: an unbounded per-peer opaque map now reachable for MVPN and MUP, bounded only by `prefix maximum` whose `countPrefixEntries` walk over-counts a typed family (pre-existing for EVPN, widened here); and `edits := make(..., 0, 2)` running for every non-session-reset UPDATE, MP-bearing or not (unchanged by this work).

### Open, and needing the owner rather than another round

Three items the closing session could not act on. `.claude/hooks/pretool-writeedit.py` refuses
any edit to an RFC-tagged test without the owner's approval, and the test-deletion hook refuses
`rm` on a `_test.go` file. Both guards are correct; these are the cost of respecting them.

| Item | What is needed |
|------|----------------|
| `TestRFC7606Section3gDuplicateMPUnreachBeatsAnAbandonedWalk` (`session_validation_dupmp_test.go`) is cascade-confounded: its body has no NLRI and no MP_REACH, which is also the Section 5.2 shape, so it stays green with the Section 3.g fix reverted. The independent audit measured this and recorded it in `rfc/audit/rfc7606.json` | Add reachable NLRI to the fixture, then delete `session_validation_dupmp_unreach_test.go`, which exists only because that edit was refused. Until then the requirement is genuinely proven by the companion, so the ledger is sound; the confounded unit is redundant rather than load-bearing |
| `TestRFC7606Section54ReadsTypedNLRIUnderAddPath` carries `RFC requirement: RFC7911-5-3 positive`. The reviewer reads RFC7911-5-3 as requiring ADD-PATH negotiated for BOTH directions, and the fixture negotiates receive only; `forward_body_test.go` already carries both polarities on `TestForwardBodyAddPath` | Decide whether the tag stays. Removing it is a coverage reduction the hook will not take without approval. The mutation (`head = 4` -> `head = 0`) does turn the test red, so it is not vacuous; the question is scope, not discrimination |
| `internal/component/bgp/reactor/zz_rv2_probe_test.go` is a reviewer scratch file whose own header says to delete it. Its two probes assert the pre-fix behavior and now FAIL, so the reactor package is red until it goes | `rm internal/component/bgp/reactor/zz_rv2_probe_test.go`. The hook refused both the reviewer and the closing session |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| Every file in Files to Create | yes | 21 checked with `ls` before this round, plus the 2 created in it, e.g. `internal/core/bgp/nlri/nlritype/nlritype.go` 186L, `internal/core/bgp/nlri/nlrisplit/typelen.go` 57L, `internal/component/bgp/plugins/nlri/mup/rfc7606.go` 63L |
| `test/plugin/rfc7606-54-discard-unrecognized-nlri.ci` | yes | 137L |
| `test/plugin/rfc7606-54-bgpls-override-propagates.ci` | yes | 124L |
| `test/interop/scenarios/53-rfc7606-54-typed-nlri-discard/` | yes | `check.py` 104L, `ze.conf` 55L, plus `inject.msg`, `inject-args`, `speaker-args` |
| `test/plugin/rfc7606-54-discard-unrecognized-mup-nlri.ci` | yes | 140L, created this round |
| `internal/component/bgp/reactor/session_validation_nlritype_bypass_test.go` | yes | created this round, two tagged tests |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | unrecognized type discarded in every bound family | `make ze-test-pkg PKG=./internal/component/bgp/reactor RUN='RFC7606Section54\|RewriteMPNLRISections\|TypedNLRI'` -> `ok ... 1.271s`, 14 tests |
| AC-2 | BGP-LS untouched | same run; `nlritype.Get` returns nil for bgp-ls because `plugins/nlri/ls` registers no recognizer |
| AC-3 | conforming UPDATE keeps its zero-copy relay | `applyTypedNLRIDiscard` returns `wu, pathAttrs` on `len(edits) == 0`, read at `session_validation_nlritype.go:104-106` |
| AC-4 | emptied UPDATE never relayed as an EoR | `updateCarriesNoRoutes` -> `typedNLRIEmptied` -> `enforceRFC7606` returns `RFC7606ActionTreatAsWithdraw` with no bodies |
| AC-5 | unparseable typed framing session-resets | `typedNLRIEdit` returns false; `enforceRFC7606` calls `s.rfc7606SessionReset` |
| all | the requirement is proven both ways in the ledger | `ai/RFC-REQUIREMENTS.md`, `RFC7606-5.4-1`: 13 positive tags (unit, functional, interop), 2 negative, no `{gap}` |
| all | the closure round's three new tests each discriminate | mutation: the "exceeds remaining data" return stripped of `MPReachNLRI` turns `...FiltersWhenTheAttributeWalkIsAbandoned` RED on the exact assertion; `head = 4` -> `head = 0` in `splitTypeLength` turns `...ReadsTypedNLRIUnderAddPath` RED; an early `return wu, pathAttrs, typedNLRIKept` in `applyTypedNLRIDiscard` turns the MUP `.ci` RED with `01000504CAFEBABE` on the receiver's wire |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| A peer sends an unrecognized EVPN type in a family 5.4 binds | `test/plugin/rfc7606-54-discard-unrecognized-nlri.ci` | yes, read: asserts the RECEIVING peer's wire, not the RIB |
| A peer sends an unrecognized BGP-LS type | `test/plugin/rfc7606-54-bgpls-override-propagates.ci` | yes, read: asserts the route IS still relayed |
| A peer sends an unrecognized BGP-MUP route type | `test/plugin/rfc7606-54-discard-unrecognized-mup-nlri.ci` | yes, written and run here: asserts the receiving peer's wire, with the unrecognized NLRI between two implemented ones so a give-up-on-first fix would fail the adjacency check |
| An independent peer judges what ze put on the wire | `test/interop/scenarios/53-rfc7606-54-typed-nlri-discard/check.py` | yes, tagged at `check.py:34`, counted as `interop/nightly` evidence by `make ze-rfc-check` |
| Registration reaches a real build | -- | `evpn`, `mvpn` and `mup` each call `nlritype.Register` in their plugin `init()`; `nlritype.Register` refuses a family with no `nlrisplit` splitter, and `nlrisplit/register.go` binds all three |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | RFC 9552 Section 5.2 names "Section 5.4 (paragraph 2) of [RFC7606]" and requires preserve-and-propagate. Citation corrected from 5.1 in four places |
| A-2 | confirmed | `typedNLRIEdit` stops at one `nlritype.Get` map read for a family with no ruling, before any NLRI is walked |
| A-3 | broken | `reactorForwardRS` (`reactor/forward_rs.go`) relays the received wire with no RIB involved. Mistake Log and Deviations both carry it |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| RFC status, RFC 7606 row: three bound families and their type ranges | `evpn.Implemented` 1..5, `MVPNRouteType.Implemented` 1..7, `MUPRouteType.Implemented` arch 1 with types 1..4 | yes, read each |
| RFC status, RFC 7606 and RFC 6514 rows: MCAST-VPN and BGP-MUP now install as opaque RIB entries | `insertPoolNLRIs` gates on `nlrisplit.Supported`, which `nlrisplit/register.go` now makes true for both | yes |
| RFC status and `rfc/short/draft-ietf-bess-mup-safi.md`: a MUP withdrawal deletes the NLRI it names | `removePoolNLRIs` (`internal/component/bgp/plugins/rib/rib.go`) splits then calls `peerRIB.Remove` | yes |
| No other repo claim was falsified by the splitter registration | `grep -rn nlrisplit docs/ rfc/short/ ai/` -- every remaining hit is about labeled unicast (RFC 8277) or is an inventory row | yes |
| Gate | `make ze-doc-test` PASSED, `make ze-rfc-check` exit 0 | yes |
| The three `ai/` discovery indexes are NOT in this commit | `make ze-discovery-index` regenerates `ai/DOCS-TO-CODE.md` with 18 new references, every one of them to an untracked file of another session's in-flight work (prefix-limit, IPsec dataplane inspection, PKI/TLS). None is this spec's. Committing the index would publish pointers to code absent from git, so the commit carries `--stale-index-ok` instead | yes, enumerated with `git ls-files --error-unmatch` per reference |
