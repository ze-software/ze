# Spec: fixit-ike-negotiation-conformance

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `-` |
| Updated | 2026-08-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Found on 2026-07-30 by phase 2b of the rfcgate-1b RFC 7296 pilot spec. Group D
refused to tag three rows. A green tag over the IKE_SA_INIT half alone would publish
"proven" over a wire-visible gap in the very exchange each row cites. The
supervising session then read every producer named below and confirmed all three. Owner
rulings taken the same day, and recorded per row under Key Design Decisions.

One theme joins them: **the initiator and the responder can disagree about what was
negotiated, and nothing detects it.**

## Task

Three MUST-level obligations of RFC 7296, each verified against the producing code.

### `RFC7296-1.3-2` -- a mismatched DH group yields wrong keys, not a rejection

<!-- ste: ignore -->
> If the responder selects a proposal using a different Diffie-Hellman group (other than NONE), the responder MUST reject the request and indicate its preferred Diffie-Hellman group in the INVALID_KE_PAYLOAD Notify payload

`respondIKERekey` checks only that the KEi payload is not empty
(`internal/component/ike/engine/rekey.go`). It negotiates a proposal at `:459`. It then
builds `crypto.NewDHExchange(chosen.DHGroup.ID)` at `:464`, from the group IT chose, and it
never compares that against the group the initiator's KEi was computed for.

`NotifyInvalidKEPayload` has exactly ONE use in the tree,
`internal/component/ike/engine/responder.go`, on the IKE_SA_INIT path. The
CREATE_CHILD_SA path builds no Notify at all.

`DHExchange.SharedSecret` (`internal/component/ike/crypto/dh.go`) accepts any peer
value in the open interval (1, p-1). A value for another group is read as a big integer and
exponentiated. The result is a shared secret both sides compute differently. Silent wrong
keys, where the RFC requires INVALID_KE_PAYLOAD.

### `RFC7296-2.8.2-1` -- simultaneous IKE rekeys both proceed

<!-- ste: ignore -->
> The new IKE SA containing the lowest nonce SHOULD be deleted by the node that created it, and the other surviving new IKE SA MUST inherit all the Child SAs

`resolveRekeyCollision` (`internal/component/ike/engine/rekey.go`) has exactly one
caller, `internal/component/ike/engine/inbound.go`, and that call sits inside the CHILD
rekey branch. The IKE rekey branch never reads `ps.pendingRekey`, so Ze answers an inbound
IKE rekey while its own IKE rekey is in flight. No nonce is compared, and no losing SA is
deleted.

The inheritance half is the MUST. Child SAs hang off `PeerSession`, so inheritance is
incidental rather than expressed, and nothing proves it.

### `RFC7296-3.3.6-3` -- the initiator never re-checks the accepted offer, except once

<!-- ste: ignore -->
> The initiator of an exchange MUST check that the accepted offer is consistent with one of its proposals, and if not MUST terminate the exchange

`crypto.NegotiateIKE` has three call sites. `internal/component/ike/engine/fsm.go`
re-checks the accepted offer at IKE_SA_INIT and sets `StateDead` on failure.
`internal/component/ike/engine/responder.go` is the responder negotiating.
`internal/component/ike/engine/rekey.go` is the responder re-negotiating on IKE rekey.

The INITIATOR performs no check at IKE_AUTH SAr2
(`internal/component/ike/engine/fsm.go`, which takes the SPI only), at
`applyChildRekeyResponse` (`internal/component/ike/engine/rekey.go`, which takes
keys from `old.ESPGroup.Proposals[0]`), or at `applyIKERekeyResponse`
(`internal/component/ike/engine/rekey.go`, which reuses `oldSA.Proposal`).

Because `respondIKERekey` genuinely re-negotiates, a responder that picks a different suite
leaves the two sides deriving the new IKE SA with different algorithms.

## Required Reading

| Document | Why |
|----------|-----|
| `rfc/short/rfc7296.md` | The three checklist rows this spec unblocks |
| `rfc/full/rfc7296.txt` sections 1.3, 2.8.2, 3.3.6 | The obligations, verbatim. The file is line-wrapped |
| `ai/rules/rfc-compliance.md` | Conformance is not negotiable, and who decides a deviation |
| `ai/rules/evidence.md` | Cite the producing function, never the caller |
| `spec-fixit-ike-request-window` (closed 2026-08-12) | The pattern this spec follows, and the shared request-window slot that landed with it. `SA.reserveRequestWindow` (`internal/component/ike/engine/msgid.go`) is that slot |

## Current Behavior (MANDATORY)

Source files read on 2026-07-30, with every `file:line` confirmed against the tree:

- [ ] `internal/component/ike/engine/rekey.go`
- [ ] `internal/component/ike/engine/fsm.go`
- [ ] `internal/component/ike/engine/responder.go`
- [ ] `internal/component/ike/engine/inbound.go`
- [ ] `internal/component/ike/crypto/dh.go`
- [ ] `internal/component/ike/wire/payload_notify.go`

Every claim in the Task section was verified by reading these files, not inferred from a
caller. `NotifyInvalidKEPayload` was confirmed to have one use tree-wide.
`resolveRekeyCollision` was confirmed to have one caller. `crypto.NegotiateIKE` was
confirmed to have three call sites, only one of which is an initiator re-check.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

An inbound CREATE_CHILD_SA request or response, reaching
`internal/component/ike/engine/inbound.go`. Format at entry is a decrypted payload chain.

### Transformation Path

Responder side: the payload chain yields a peer SA payload and a KEi.
`crypto.NegotiateIKE` picks a proposal. `crypto.NewDHExchange` builds our half from the
CHOSEN group. `DHExchange.SharedSecret` then combines it with the peer value.

Initiator side: the response yields an accepted proposal and a KEr. The keys are derived
from whatever the initiator already believed the proposal was.

Each of the three fixes inserts a comparison that is missing today. They are the chosen
group against the requested group, our own in-flight rekey against the peer's, and the
accepted offer against our proposals.

### Boundaries Crossed

| Boundary | Direction | Carries |
|----------|-----------|---------|
| engine -> crypto | out | The chosen group and the peer public value |
| crypto -> engine | in | The shared secret, or an error when the value is refused |
| engine -> transport | out | An INVALID_KE_PAYLOAD Notify, where today nothing is sent |

### Integration Points

The rfcgate-1b RFC 7296 pilot spec phase 2b owns rows `RFC7296-1.3-2`,
`RFC7296-2.8.2-1` and `RFC7296-3.3.6-3`. All three stay untagged until this spec closes.

## Key Design Decisions

Each row's approach is an owner ruling taken on 2026-07-30.

| Row | Ruling | Over | Because |
|-----|--------|------|---------|
| `RFC7296-1.3-2` | Reject at the exchange AND harden the DH primitive | Rejecting at the exchange alone | The exchange fix satisfies the MUST. The primitive fix means a future path that forgets the comparison fails closed, rather than deriving garbage from a mismatched value |
| `RFC7296-2.8.2-1` | Full collision resolution for IKE rekey | Answering TEMPORARY_FAILURE, or resolving without inheritance | It implements both halves as written, and it reuses a comparison that already exists and is already tested. Inheritance is the MUST half, so it cannot be deferred |
| `RFC7296-3.3.6-3` | One shared verify helper, called at all four sites | Three in-place checks | One producer for the rule. A fifth response path added later is then an obvious omission rather than a silent one, and the asymmetry that caused this is removed |

## Blast Radius

`internal/component/ike/engine` and `internal/component/ike/crypto`. The DH hardening is
the widest edge: `SharedSecret` is on the key-derivation path of every exchange, so a wrong
length bound would break working handshakes. Bound it by the group's modulus length, which
`padBigInt` (`crypto/dh.go`) already produces on the send side.

## Risks & Assumptions

| Id | Statement | Basis | Validation |
|----|-----------|-------|------------|
| A-1 | The KE payload carries the group it was computed for | `wire.PayloadKE` has a `DHGroup` field, read at `responder.go` | Read `wire/payload_ke.go` first, then rely on it |
| A-2 | `padBigInt` already fixes the send-side length per group | Verified 2026-07-30: `crypto/dh.go` pads to 256 for MODP-2048 | A test asserts the length equals the modulus length, derived not hardcoded |
| R-1 | A length bound in `SharedSecret` rejects a legitimate peer | Some peers send an unpadded value, which is shorter than the modulus | Decide explicitly whether to accept a SHORT value and left-pad it, or refuse it. Prove the chosen behaviour against a value with a natural short encoding |
| R-2 | Collision resolution deletes the wrong SA | The nonce comparison decides the winner | Drive both orderings in a test, and assert the survivor from each side |

## Acceptance Criteria

| Id | Criterion |
|----|-----------|
| AC-1 | The CREATE_CHILD_SA responder compares the request's KE group against the chosen group |
| AC-2 | On mismatch it sends INVALID_KE_PAYLOAD naming its preferred group, and derives no keys |
| AC-3 | `crypto.SharedSecret` refuses a peer value whose length does not match the group's modulus, with a named error |
| AC-4 | The IKE rekey branch calls `resolveRekeyCollision` when our own IKE rekey is in flight |
| AC-5 | The new IKE SA carrying the lowest nonce is deleted by the node that created it |
| AC-6 | The surviving new IKE SA inherits every Child SA, asserted rather than incidental |
| AC-7 | One named helper performs the accepted-offer check, and all four initiator response paths call it |
| AC-8 | An accepted offer inconsistent with our proposals stops the exchange at each of the four sites |
| AC-9 | `RFC7296-1.3-2` carries a positive AND a negative tagged test |
| AC-10 | `RFC7296-2.8.2-1` carries a positive AND a negative tagged test |
| AC-11 | `RFC7296-3.3.6-3` carries a positive AND a negative tagged test |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | | Feature Code | Test |
|-------------|---|--------------|------|
| CREATE_CHILD_SA request whose KEi group differs from the chosen group | -> | the group comparison in the responder | `TestNegRekeyRejectsMismatchedKEGroup` |
| Inbound IKE rekey while ours is in flight | -> | the collision call in the IKE branch | `TestNegIKERekeyCollisionResolves` |
| IKE rekey response naming a suite we never proposed | -> | the shared verify helper | `TestNegInitiatorRejectsUnproposedOffer` |

## 🧪 TDD Test Plan

Each test is written first and must fail against the current tree.

### Unit Tests

| Test | Proves |
|------|--------|
| `TestNegRekeyRejectsMismatchedKEGroup` | AC-1, AC-2, AC-9 |
| `TestNegSharedSecretRefusesWrongLength` | AC-3 |
| `TestNegIKERekeyCollisionResolves` | AC-4, AC-5, AC-10 |
| `TestNegSurvivingSAInheritsChildren` | AC-6 |
| `TestNegInitiatorRejectsUnproposedOffer` | AC-7, AC-8, AC-11 |

### Functional Tests

The IPsec functional suite covers an established SA with rekey, and it stays as the
regression net. These three rows are negotiation-failure paths that no user-facing surface
exposes, so the proof is a Go test that drives the exchange directly. The `.ci` suite proves
the daemon still establishes and rekeys with all three checks in place.

## Files to Modify

| File | Change |
|------|--------|
| `internal/component/ike/engine/rekey.go` | The group comparison, the collision call, the accepted-offer check at both apply sites |
| `internal/component/ike/engine/inbound.go` | Route the IKE rekey branch through collision resolution |
| `internal/component/ike/engine/fsm.go` | Call the shared helper at IKE_AUTH SAr2, and use it at IKE_SA_INIT |
| `internal/component/ike/crypto/dh.go` | Refuse a peer value of the wrong length |

## Files to Create

| File | Holds |
|------|-------|
| `internal/component/ike/engine/rfc7296_negotiation_test.go` | The tagged pairs for all three rows |

## Implementation Steps

1. Write the failing tests from the TDD plan, and record the red output.
2. Add the shared accepted-offer helper, and call it from all four initiator sites.
3. Add the KE group comparison and the INVALID_KE_PAYLOAD response.
4. Add the length refusal to `SharedSecret`, and settle R-1 explicitly.
5. Route the IKE rekey branch through `resolveRekeyCollision`, and assert inheritance.
6. Confirm each test goes green, then mutation-verify all three tagged rows.

## Goal Gates

`make ze-verify`.

## Quality Gates

`make ze-lint-changed`, `make ze-rfc-check`, `python3 scripts/dev/ste_check.py --check`.

## RFC Documentation (Scope: protocol)

`rfc/short/rfc7296.md` gains the three rows named in AC-9, AC-10 and AC-11. The supervising
session generates those rows centrally, so this work adds the tagged tests only.

## Checklist

- [ ] Tests written first, run, and Tests FAIL output pasted into the spec
- [ ] The shared accepted-offer helper exists and all four sites call it
- [ ] The KE group comparison sends INVALID_KE_PAYLOAD and derives no keys
- [ ] `SharedSecret` refuses a wrong-length value, and R-1 is settled in writing
- [ ] IKE rekey collisions resolve, and inheritance is asserted
- [ ] Tests PASS output pasted into the spec
- [ ] All three rows tagged and mutation-verified
- [ ] `make ze-verify` green

## R-1 Settled: refuse a peer value of the wrong length

**Decision: `SharedSecret` refuses any MODP peer value whose length is not the modulus
length. It refuses a short value and a long value alike.**

The two branches of R-1 are not equal. One of them is a no-op.

`big.Int.SetBytes` reads its input as an unsigned big-endian integer, so zero octets in
front do not change the number. "Accept a short value and left-pad it" therefore produces
the same integer that `SetBytes(remotePublic)` produces today. That branch changes no
behaviour, and AC-3 asks for a named refusal. The branch cannot implement AC-3.

The repository already holds the proof. `TestRFC7296MODPShortPublicValueHasNoSecondGuard`
(`crypto/rfc7296_dh_test.go`) asserts that the short value and the padded value
give the SAME secret. That is the left-pad branch, measured.

Three more reasons support the refusal:

| Reason | Detail |
|--------|--------|
| The RFC puts the obligation on the sender | Section 3.4 makes the pad a MUST. A short value is a peer that did not conform, and `ai/rules/evidence.md` says deny rather than guess |
| A short value cannot be told apart from another group | An ECP-256 public value is 65 octets. To a MODP-2048 exchange it is "short". Accepting short is therefore accepting the mismatched group the owner ruling exists to close |
| ECP is already strict | `ecdh.PXXX().NewPublicKey` refuses any length but its own. The refusal makes MODP behave the same way, so the primitive is uniform |

### The cost, stated plainly

**This makes Ze stricter on receive than the RFC requires of a receiver.** Section 3.4
binds the SENDER: it says the public value MUST have the length of the modulus. It places
no matching obligation on the receiver, and a receiver that accepts a short value breaks no
rule. Ze now refuses one.

The interop cost is bounded. A conformant sender always pads. A sender that does not pad
emits a short value only when its public value carries a leading zero octet. That is about
one exchange in 256 against an already non-conformant peer.

`ErrPublicKeyLength` wraps `ErrInvalidPublicKey`. A wrong length is one form of invalid
public key, so a caller that tests the general case still matches.

### What this changed in the `RFC7296-3.4-1` pair

The negative half of `RFC7296-3.4-1` argued that the send-side pad was the only code that
held the rule. It proved that with an unpadded value, which `SharedSecret` accepted. AC-3
removed that premise.

The negative was rewritten, with the coordinator's approval recorded in the file as
`rfc-test-change-approved`. The new argument is stronger. The pad on the send side is
load-bearing BECAUSE a conforming receiver refuses a wrong-length value, and Ze now applies
that rule itself. The test drives Ze's own receiver as the stand-in for such a peer. The
positive half is untouched. `TestRFC7296MODPShortPublicValueIsRefusedOnReceipt` was
mutation-verified: with the length check removed it fails at the first assertion.

## Evidence

### Tests FAIL, before the implementation

```
--- FAIL: TestNegRekeyRejectsMismatchedKEGroup (0.07s)
    rfc7296_negotiation_test.go:311: a mismatched KE group still built an IKE SA, so keys were derived
--- FAIL: TestNegSharedSecretRefusesWrongLength (0.05s)
    rfc7296_negotiation_test.go:377: one octet short (255 octets): error = <nil>, want ErrPublicKeyLength
    rfc7296_negotiation_test.go:381: one octet short: a secret was returned beside the refusal
    rfc7296_negotiation_test.go:377: one octet long (257 octets): error = <nil>, want ErrPublicKeyLength
    rfc7296_negotiation_test.go:381: one octet long: a secret was returned beside the refusal
    rfc7296_negotiation_test.go:377: the natural encoding of the group generator (1 octets): error = <nil>, want ErrPublicKeyLength
    rfc7296_negotiation_test.go:381: the natural encoding of the group generator: a secret was returned beside the refusal
    rfc7296_negotiation_test.go:377: an ECP-256 public value (65 octets): error = <nil>, want ErrPublicKeyLength
    rfc7296_negotiation_test.go:381: an ECP-256 public value: a secret was returned beside the refusal
--- FAIL: TestNegIKERekeyCollisionResolves (0.07s)
    rfc7296_negotiation_test.go:434: the losing peer exchange still built a second new IKE SA
    rfc7296_negotiation_test.go:439: peer IKE rekey that lost the collision: ze wrote 432 unexpected bytes before the sentinel
--- FAIL: TestNegSurvivingSAInheritsChildren (0.05s)
    rfc7296_negotiation_test.go:498: our own exchange survived beside the peer exchange, so two new IKE SAs exist
--- FAIL: TestNegInitiatorRejectsUnproposedOffer (0.23s)
    --- FAIL: TestNegInitiatorRejectsUnproposedOffer/ike-auth-response (0.05s)
        rfc7296_negotiation_test.go:603: state = established, want dead because the accepted ESP offer is unproposed
    --- FAIL: TestNegInitiatorRejectsUnproposedOffer/child-rekey-response (0.03s)
        rfc7296_negotiation_test.go:623: an unproposed ESP offer was accepted, so the exchange did not stop
    --- FAIL: TestNegInitiatorRejectsUnproposedOffer/ike-rekey-response (0.11s)
        rfc7296_negotiation_test.go:636: an unproposed IKE offer was accepted, so the exchange did not stop
FAIL
FAIL	github.com/ze-software/ze/internal/component/ike/engine	0.492s
```

The `sa-init-response` and `shared-helper` cases were green at this point, and that was
expected. `handleSAInitResponse` was the one initiator site that already re-checked, and
the helper was in place before its four call sites were wired.

### Tests PASS, after the implementation

```
ok  	github.com/ze-software/ze/internal/component/ike/cmd	1.043s
ok  	github.com/ze-software/ze/internal/component/ike/dataplane	1.025s
ok  	github.com/ze-software/ze/internal/component/ike/eap	1.215s
ok  	github.com/ze-software/ze/internal/component/ike/engine	8.005s
ok  	github.com/ze-software/ze/internal/component/ike/ipsec	1.066s
ok  	github.com/ze-software/ze/internal/component/ike/transport	1.177s
ok  	github.com/ze-software/ze/internal/component/ike/wire	1.033s
ok  	github.com/ze-software/ze/internal/component/ike/yang	1.046s
```

The four packages the change touches are green on their own:

```
ok  	github.com/ze-software/ze/internal/component/ike/crypto	0.088s
ok  	github.com/ze-software/ze/internal/component/ike/eap	0.129s
ok  	github.com/ze-software/ze/internal/component/ike/engine	6.152s
ok  	github.com/ze-software/ze/internal/component/ike/wire	0.006s
```

The ipsec functional suite is the regression net, and it runs two real ze daemons:

```
pass  8/8  100.0%  13.7s
timing: 1:13.7s 3:4.1s 2:4.0s 4:3.9s 5:3.8s 6:3.8s 7:3.8s 8:2.7s
```

`ipsec-child-rekey` drives a real CREATE_CHILD_SA rekey through the new accepted-offer
check. The check is therefore proven on the daemon path, and not only in a Go test.

### Mutation verification

Each producer was broken in turn, and the tagged tests were re-run.

| Row | Producer broken | Result |
|-----|-----------------|--------|
| `RFC7296-1.3-2` | the KE group comparison (`rekey.go`) | RED: `TestNegRekeyRejectsMismatchedKEGroup` |
| `RFC7296-1.3-2` | the length refusal (`crypto/dh.go`) | RED: `TestNegSharedSecretRefusesWrongLength`, all four values |
| `RFC7296-2.8.2-1` | the collision branch (`inbound.go`) | RED: `TestNegIKERekeyCollisionResolves` AND `TestNegSurvivingSAInheritsChildren` |
| `RFC7296-3.3.6-3` | the refusal inside `verifyAcceptedOffer` | RED at all five cases: `shared-helper`, `sa-init-response`, `ike-auth-response`, `child-rekey-response`, `ike-rekey-response` |

The last mutation was deliberately surgical. The helper kept returning a valid proposal and
lost only its refusal, so each of the four sites is proven to gate the mismatch itself.

Every mutation was reverted, and the suite is green again.

## CLOSURE REFUSED 2026-08-03 -- three findings, one of them a live defect

**Superseded on 2026-08-12. All three findings are settled at HEAD; see the closure
sections at the end of this file. This section is kept as the record of the round that
found them.**

`/ze-close` ran the deliverables review, re-verified every AC against its producing
function, and put the changeset to three independent reviewer subagents. **This spec is NOT
closed.** Every finding below was reproduced against source before it was graded.

AC-1 through AC-11 are each implemented and each has a passing tagged test. That is not the
same as the spec's goal being met, and the goal is the sentence in the Task section: "the
initiator and the responder can disagree about what was negotiated, and nothing detects it."

### BLOCKER 1 -- the accepted offer is verified, then thrown away

`verifyAcceptedOffer` returns the local proposal it agreed with. Two of its four call sites
discard that return value and then key from `Proposals[0]`:

| Site | Producer | What it keys from |
|------|----------|-------------------|
| Child SA rekey response | `internal/component/ike/engine/rekey.go` `applyChildRekeyResponse` | `if _, err := verifyAcceptedOffer(...)`, then `prop := old.ESPGroup.Proposals[0]` |
| IKE_AUTH response | `internal/component/ike/engine/fsm.go` `handleAuthResponse` | `if _, err := verifyAcceptedOffer(childOffer, ...)`, then `internal/component/ike/engine/child.go` `createFirstChildSA` reads `espGroup.Proposals[0]` |

`buildWireESPProposals` (`internal/component/ike/engine/initiator.go`) puts EVERY configured
proposal on the wire, and `verifyAcceptedOffer` matches the accepted offer against all of
them. So a peer that accepts proposal #2 passes the check, and Ze then keys the Child SA
with proposal #1's cipher and key length. The tunnel comes up and black-holes ESP.

The responder half of this same defect is already fixed and already tested:
`selectResponderESP` (`internal/component/ike/engine/responder.go`) narrows
`sa.ESPGroup.Proposals` to the one it chose, and `responder_test.go` carries a
multi-proposal test for exactly this. **The initiator never narrows.** This spec added the
detection of one disagreement and left a second one in place, in the very code path it
touched.

The fix is to bind the result: carry `offer.ESP` into `crypto.DeriveChildSAKeys` and
`installChildTolerant`, or narrow `sa.ESPGroup` to the accepted proposal at both initiator
sites. The regression test is a two-proposal ESP group whose peer accepts the second, with
the installed suite asserted.

### BLOCKER 2 -- AC-3 silenced the only test of the MODP small-value guard

`SharedSecret` (`internal/component/ike/crypto/dh.go`) applies
`remotePub.Cmp(one) <= 0 || remotePub.Cmp(pMinusOne) >= 0` on the MODP branch. That guard is
now reachable ONLY by a value of exactly `modp2048Len` octets, because AC-3's length refusal
runs first. The one test named for it, `TestDHInvalidPublicKey`
(`internal/component/ike/crypto/dh_test.go`), passes `[]byte{0}`, one octet. It used to
reach the bounds check; it now short-circuits at `ErrPublicKeyLength`, which WRAPS
`ErrInvalidPublicKey`, so `errors.Is` still matches and the test stayed green while the
behaviour it proved stopped being exercised. Every other `SharedSecret` call in the tree
passes a real public key, a short value, or an ECP value.

This is a removed guard, and this spec removed it. The fix is cases in
`TestDHInvalidPublicKey` passing `padBigInt(0, modp2048Len)`, `padBigInt(1, ...)` and
`padBigInt(p-1, ...)`, asserting `ErrInvalidPublicKey` AND `!errors.Is(err,
ErrPublicKeyLength)`, so the next length-check change cannot re-silence it the same way.

### ISSUE 3 -- a tag attaches `RFC7296-1.3-2` evidence to a different rule

`internal/component/ike/engine/rfc7296_negotiation_test.go`, the tag above
`TestNegSharedSecretRefusesWrongLength`, reads `RFC requirement: RFC7296-1.3-2 positive`.
`RFC7296-1.3-2` is the responder's duty to "reject the request and indicate its preferred
Diffie-Hellman group in the INVALID_KE_PAYLOAD Notify payload". A length refusal inside
`SharedSecret` produces no such Notify. The requirement IS genuinely proven, by
`TestNegRekeyRejectsMismatchedKEGroup` over `respondIKERekey`; this second tag inflates the
row's proof with evidence for another rule. Its case name "the natural encoding of the group
generator" also reads as a degeneracy test when it is a length test: the PADDED generator is
accepted, and `crypto/rfc7296_dh_test.go` asserts that acceptance.

### Recorded, not a blocker: the receive-side length refusal stays over-strict by choice

The RFC lens graded AC-3 OVER-STRICT. RFC 7296 Section 3.4 binds the SENDER ("The length of
the Diffie-Hellman public value for MODP groups MUST be equal to the length of the prime
modulus ... prepending zero bits to the value if necessary") and places no matching
obligation on a receiver. Against a peer that does not pad, `g^x mod p` carries a leading
zero octet about one time in 256, so the tunnel fails intermittently with no Notify.

This spec's "R-1 Settled" section already states that cost in the same terms and chooses the
refusal deliberately. The finding adds no new fact, so it is recorded rather than reopened.
It is worth putting to the owner beside BLOCKER 2, because both bear on the same guard.

## BLOCKER 1 and BLOCKER 2 fixed, 2026-08-04

### BLOCKER 1 -- the accepted offer now keys the Child SA

`acceptedOffer` (`internal/component/ike/engine/initiator.go`) gains `ESPConfig`, the
CONFIGURED proposal the accepted offer agrees with. `espConfigForAccepted` resolves it from
the transforms, because RFC 7296 Section 3.3.1 makes the Proposal Num the peer's. The field
is the ESP equivalent of `IKE`. `crypto.matchIKE` returns the LOCAL proposal, so `offer.IKE`
already carries the local key lengths. `crypto.matchESP` returns the REMOTE one, whose
integrity transform carries an id and no key length.

| Site | Producer | Now |
|------|----------|-----|
| IKE_SA_INIT response | `handleSAInitResponse` (`fsm.go`) | unchanged: already keyed from `offer.IKE` |
| IKE_AUTH response | `handleAuthResponse` (`fsm.go`) | narrows `sa.ESPGroup.Proposals` to `offer.ESPConfig`, as `selectResponderESP` does on the responder side |
| Child SA rekey response | `applyChildRekeyResponse` (`rekey.go`) | keys and installs from `offer.ESPConfig`, and narrows the replacement child's own group |
| IKE SA rekey response | `applyIKERekeyResponse` (`rekey.go`) | unchanged: already keyed from `offer.IKE` |

`runEstablished` (`established.go`) called `createFirstChildSA` with the SESSION's
unnarrowed group, so the narrowing alone would not have reached the first Child SA. The call
moved into `initiatorFirstChildSA` (`child.go`), which reads `sa.ESPGroup` and takes no group
argument. The test drives that same function.

`TestNegInitiatorInstallsAcceptedESPSuite` (`rfc7296_negotiation_test.go`) drives a peer that
accepts the SECOND of two ESP proposals, at both sites. It asserts the algorithms of the ESP
states ze programs. It carries no `RFC requirement:` tag. No row of `rfc/short/rfc7296.md`
states the obligation it proves, and ISSUE 3 above is what an invented tag would repeat.

### BLOCKER 2 -- the MODP range guard has a fixture that reaches it

`TestDHInvalidPublicKey` (`internal/component/ike/crypto/dh_test.go`) passed one octet, which
AC-3's length refusal answers first. Each case now pads to `modp2048Len`, and each asserts
`ErrInvalidPublicKey` AND `NOT ErrPublicKeyLength`. The cases are 0, 1, p-1, and p.

### Mutation verification

| Producer broken | Result |
|-----------------|--------|
| `prop := old.ESPGroup.Proposals[0]` restored in `applyChildRekeyResponse` | RED: `child-rekey` installed aes256/sha256, want aes128/sha512 |
| the narrowing dropped from `handleAuthResponse` | RED: `ike-auth-first-child`, same message |
| the `(1, p-1)` comparison disabled in `SharedSecret` | RED at all four cases of `TestDHInvalidPublicKey`. The one-octet fixture it replaced stayed GREEN under the same mutation |

### Still open

ISSUE 3 is untouched. Correcting it edits an `RFC requirement:` tag, which this work is not
authorized to do.

## Findings for the supervising session

### Superseded 2026-08-03: the direction was corrected, and a deeper defect was found under it

The subsection below was written on 2026-07-30 and is kept for the record. Two things have
changed since.

`resolveRekeyCollision` no longer exists. `plan/spec-fixit-ike-rekey-collision-inverted.md`
renamed it `localNonceIsLower` and inverted the decision at both call sites, so the endpoint
whose exchange carries the LOWER nonce now abandons it. AC-5 as worded ("The new IKE SA
carrying the lowest nonce is deleted by the node that created it") therefore HOLDS against
the tree, and the conflict this subsection describes is gone.

What replaced it is worse, and it is recorded in that spec's own closure-refusal section:
RFC 7296 Section 2.8.1 resolves the collision from "the lowest of THE FOUR nonces used in
the two exchanges", after both responses have arrived, and it says the receiving endpoint
"cannot yet know which of the exchanges will have the lowest nonce, so it will just note the
situation and respond as usual". Ze compares TWO nonces at request-receipt time and answers
nothing when it keeps its own exchange. AC-6's inheritance MUST still holds. AC-4's routing
still holds. The nonce arithmetic underneath them does not.

### The 2026-07-30 record: `resolveRekeyCollision` reads Section 2.8.1 in the opposite direction

RFC 7296 Section 2.8.1 says the SA created with the LOWEST of the four nonces "SHOULD be
closed by the endpoint that created it". `resolveRekeyCollision` (`rekey.go`) documents the
opposite: "the exchange with the lowest nonce wins". The Child branch has used that reading
since it was written, and `RFC7296-2.8-1` is already enrolled with the same words.

The IKE branch reuses the existing comparison, exactly as the Key Design Decision directs.
The outcome the MUST asks for is met either way. One new IKE SA survives, and it inherits
every Child SA. Both peers run one comparison, so they agree on the survivor and no
redundant SA is left. Only the choice of WHICH exchange survives differs, and that half is
a SHOULD.

AC-5 is worded from the RFC and reads "the lowest nonce is deleted". Under the repository
convention the abandoned exchange is the one with the HIGHER nonce. The two cannot both
hold. A change of direction would break the enrolled `RFC7296-2.8-1` tagged test. This work
is not authorized to make that change on its own.

## Known Limitations

~~`RFC7296-1.4.1-3` and `RFC7296-1.4.1-2` stay unimplemented and are NOT in scope here.
Both need per-direction Child SA teardown, which Ze does not have. `removeChildSA`
(`internal/component/ike/engine/child.go`) always drops both halves together.
`handleDeletePayload` (`internal/component/ike/engine/inbound.go`) never reads the
Delete payload's SPIs. That is separate machinery and it belongs to its own spec.~~

**RESOLVED 2026-08-03 (bookkeeping audit). The machinery this limitation said was absent
now exists, so the limitation carries no outstanding work and the Deferral shard row is `-`.**
Producers read, not inferred:

| Producer | Where | What it does |
|----------|-------|--------------|
| `removeChildSAOutgoing`, `removeChildSAIncoming` | `internal/component/ike/engine/child.go` | drop one half of a Child SA pair each. `removeChildSA` now calls both in turn instead of being the only shape available |
| `handleDeletePayload` -> `closeDesignatedChildSAs` | `internal/component/ike/engine/delete.go` | reads the peer's SPI list through `deleteSPIs` and closes the pair each SPI names |
| `crossOwnDelete` | `internal/component/ike/engine/delete.go` | carries the section's ordering MUST for a Delete that crosses one Ze already sent |

The two ids the paragraph names do not exist in `rfc/short/rfc7296.md` and never did: a grep
of `plan/`, `rfc/` and `docs/` on 2026-08-03 returns this spec and nothing else. Section
1.4.1 is enrolled as `RFC7296-1.4.1-1` and `-4` through `-7`. So there is no requirement row
to home either, and `docs/features/rfc-status.md` already discloses the Section 1.4.1 Delete
behaviour as implemented.

---

## Implementation Summary

### What Was Implemented
- The CREATE_CHILD_SA responder compares the request's KE group against the group it
  chose. `respondIKERekey` (`internal/component/ike/engine/rekey.go`) answers
  `wire.NotifyInvalidKEPayload` naming its preferred group, returns a nil new SA, and
  derives no keys.
- `SharedSecret` (`internal/component/ike/crypto/dh.go`) refuses a peer value whose
  length is not the group's modulus length, with the named `ErrPublicKeyLength`, on the
  MODP branch and the ECP branch.
- The IKE rekey branch of `handleCreateChildSAOwned`
  (`internal/component/ike/engine/inbound.go`) resolves a simultaneous IKE rekey through
  `localNonceIsLower`, so one new IKE SA survives and inherits the session's Child SAs.
- One helper holds RFC 7296 Section 3.3.6: `verifyAcceptedOffer`
  (`internal/component/ike/engine/initiator.go`). All four initiator response paths call
  it, and each stops the exchange when it refuses.
- `acceptedOffer.ESPConfig` and `espConfigForAccepted` (`initiator.go`) carry the
  CONFIGURED proposal the accepted offer agrees with, so the Child SA is keyed from what
  the peer accepted rather than from `Proposals[0]`.
- Rows `RFC7296-1.3-2`, `RFC7296-2.8.2-1` and `RFC7296-3.3.6-3` each carry a positive and
  a negative tagged test.

### Bugs Found/Fixed
- **The accepted offer was verified and then discarded.** Two of the four call sites keyed
  from `Proposals[0]`, so a peer that accepted the second proposal got a tunnel that
  black-holes ESP. Covered by `TestNegInitiatorInstallsAcceptedESPSuite`.
- **The MODP small-value guard lost its only fixture.** The length refusal added by AC-3
  answered the one-octet input first, and `ErrPublicKeyLength` wraps `ErrInvalidPublicKey`,
  so the test stayed green while the guard stopped being exercised. `TestDHInvalidPublicKey`
  (`internal/component/ike/crypto/dh_test.go`) now pads each case to `modp2048Len` and
  asserts `ErrInvalidPublicKey` AND not `ErrPublicKeyLength`.
- **A tag credited `RFC7296-1.3-2` with evidence for another rule.** At HEAD
  `TestNegSharedSecretRefusesWrongLength` carries no `RFC requirement:` tag at all, and the
  comment above it states why.

### Documentation Updates
- None. `grep -rn "spec-fixit-ike-negotiation-conformance" docs/` returns nothing, and no
  `docs/` claim is anchored to `initiator.go`, `fsm.go`, `rekey.go` or `crypto/dh.go`.
- `rfc/short/rfc7296.md` carries the three rows. They were generated centrally, as the RFC
  Documentation section says.

### Deviations from Plan
- `verifyAcceptedOffer` gained a return value the plan did not name. Verifying the offer
  and then keying from a different proposal was the defect the 2026-08-03 review found, so
  the helper now returns the local proposal it agreed with and the callers bind it.
- `createFirstChildSA` moved behind `initiatorFirstChildSA` (`child.go`), which reads
  `sa.ESPGroup` rather than taking a group argument. Without that move the narrowing at
  `handleAuthResponse` would not have reached the session's first Child SA.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The accepted-offer check was treated as a validation, so its result was discarded | A check whose result is thrown away only detects the disagreement it was written for. Ze still keyed from `Proposals[0]` | The 2026-08-03 independent review, BLOCKER 1 | `acceptedOffer.ESPConfig` binds the result, and the two initiator sites narrow to it |
| assumption | A new receive-side length refusal was assumed not to affect existing guards | It short-circuited the MODP small-value guard, and the wrapped error kept the test green | The same review, BLOCKER 2 | Each fixture pads to the modulus length and asserts the absence of `ErrPublicKeyLength` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| `RFC7296-1.3-2` a mismatched DH group is rejected with INVALID_KE_PAYLOAD | Done | `respondIKERekey` (`engine/rekey.go`) | Tagged positive and negative in `rfc7296_negotiation_test.go` |
| `RFC7296-2.8.2-1` one new IKE SA survives a collision and inherits the Child SAs | Done | `handleCreateChildSAOwned` IKE branch (`engine/inbound.go`) | The nonce ARITHMETIC under it is a separate open defect; see "What this closure does not claim" |
| `RFC7296-3.3.6-3` the initiator checks the accepted offer and terminates when it disagrees | Done | `verifyAcceptedOffer` (`engine/initiator.go`), four call sites | This is the row round 1 of the D-bucket triage did not check. It is checked here |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `respondIKERekey` (`engine/rekey.go`) | Compares the request's KE group against the chosen group |
| AC-2 | Done | same, `wire.NotifyInvalidKEPayload` | Returns a nil new SA, so no keys are derived. `handleCreateChildSAOwned` logs the refusal and answers nothing further |
| AC-3 | Done | `ErrPublicKeyLength` (`crypto/dh.go`), returned on the MODP and the ECP branch | R-1 settled in writing in this spec |
| AC-4 | Done | `handleCreateChildSAOwned` IKE branch calls `localNonceIsLower` when `p.kind == rekeyIKE` | |
| AC-5 | Done | the same call site abandons our exchange when our nonce is the lower one | Corrected by `spec-fixit-ike-rekey-collision-inverted` |
| AC-6 | Done | `TestNegSurvivingSAInheritsChildren` | Inheritance asserted, not incidental |
| AC-7 | Done | `verifyAcceptedOffer` (`engine/initiator.go`), called at `fsm.go` twice and `rekey.go` twice | Four initiator response paths, one helper |
| AC-8 | Done | `handleSAInitResponse` sets `StateDead`; `handleAuthResponse` sends NO_PROPOSAL_CHOSEN then `StateDead`; `applyChildRekeyResponse` and `applyIKERekeyResponse` return the wrapped error | Each of the four stops the exchange |
| AC-9 | Done | `RFC7296-1.3-2` positive and negative in `rfc7296_negotiation_test.go` | |
| AC-10 | Done | `RFC7296-2.8.2-1` positive and negative, twice over, in `rfc7296_negotiation_test.go` | |
| AC-11 | Done | `RFC7296-3.3.6-3` positive and negative in `rfc7296_negotiation_test.go`, `rfc7296_keylength_test.go` and `crypto/rfc7296_verify_test.go` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestNegRekeyRejectsMismatchedKEGroup` | Done | `engine/rfc7296_negotiation_test.go` | |
| `TestNegSharedSecretRefusesWrongLength` | Done | same | Untagged at HEAD, deliberately |
| `TestNegIKERekeyCollisionResolves` | Done | same | |
| `TestNegSurvivingSAInheritsChildren` | Done | same | |
| `TestNegInitiatorRejectsUnproposedOffer` | Done | same | |
| `TestNegInitiatorInstallsAcceptedESPSuite` | Done, beyond plan | same | The BLOCKER 1 regression test |
| `TestDHInvalidPublicKey` | Done, beyond plan | `crypto/dh_test.go` | The BLOCKER 2 fixture repair |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/ike/engine/rekey.go` | Done | |
| `internal/component/ike/engine/inbound.go` | Done | |
| `internal/component/ike/engine/fsm.go` | Done | |
| `internal/component/ike/crypto/dh.go` | Done | |
| `internal/component/ike/engine/rfc7296_negotiation_test.go` | Done, created | |
| `internal/component/ike/engine/initiator.go` | Done, beyond plan | `acceptedOffer.ESPConfig`, `espConfigForAccepted` |
| `internal/component/ike/engine/child.go` | Done, beyond plan | `initiatorFirstChildSA` |

### Audit Summary
- **Total items:** 28 (3 requirements, 11 ACs, 7 tests, 7 files)
- **Done:** 28
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Goal Validation (BLOCKING)

The Task's one theme: the initiator and the responder can disagree about what was
negotiated, and nothing detects it.

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A disagreement about the Diffie-Hellman group is detected and answered | functional (Go, over the wire encoder) | `TestNegRekeyRejectsMismatchedKEGroup` asserts `NotifyInvalidKEPayload` and the two octets naming the preferred group |
| A disagreement about the accepted suite is detected at every initiator response path | functional (Go) | `TestNegInitiatorRejectsUnproposedOffer` drives `verifyAcceptedOffer` over an unproposed IKE offer and an unproposed ESP offer, and over nil and empty payloads |
| A detected disagreement is not merely detected: the Child SA is keyed from what the peer accepted | functional (Go), mutation-verified | `TestNegInitiatorInstallsAcceptedESPSuite` drives a peer that accepts the SECOND of two ESP proposals at both sites and asserts the algorithms of the ESP states ze programs. Restoring `prop := old.ESPGroup.Proposals[0]` reddens it with "installed aes256/sha256, want aes128/sha512" |
| A simultaneous IKE rekey leaves one SA, and it inherits the Child SAs | functional (Go) | `TestNegIKERekeyCollisionResolves` and `TestNegSurvivingSAInheritsChildren` |
| A wrong-length peer Diffie-Hellman value is refused with a named error, and the small-value guard is still reached | functional (Go), mutation-verified | `TestDHInvalidPublicKey`, four padded cases. Disabling the `(1, p-1)` comparison reddens all four; the one-octet fixture it replaced stayed green under the same mutation |
| The daemon still establishes and rekeys with all three checks in place | functional (`.ci`) | The `ipsec` suite, 8/8, recorded under Evidence above. NOT re-run by this closure |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| (no shard) | n/a | The metadata row reads `-`. This spec deferred nothing, and no shard file exists for it |

No shard is removed by this closure. No foreign shard was emptied by it.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-ike-negotiation-conformance-640fa955-f03a-45e8-a58f-4b367f5859e6.md` |
| `review_gate.py check` | `review_gate: OK (clean, hashes match)`, 2026-08-12 |
| Rounds | 2. Round 1 was the three-lens round of 2026-08-03 recorded above. Round 2 is the closure re-verification of 2026-08-12, over code already at HEAD |
| Verdict | 0 BLOCKER, 0 ISSUE |
| Reviewer lenses used | Round 1: three independent subagents through `/ze-review` (deliverables plus AC re-verification; logic and wiring; RFC 7296 conformance). Round 2: AC-1 to AC-11 and all three round-1 findings re-read at their producing functions |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | The accepted offer is verified, then thrown away, and the Child SA is keyed from `Proposals[0]` | `applyChildRekeyResponse` (`engine/rekey.go`), `handleAuthResponse` (`engine/fsm.go`) | Fixed 2026-08-04. `acceptedOffer.ESPConfig` and `espConfigForAccepted` (`engine/initiator.go`). `rekey.go` keys from `offer.ESPConfig`; `fsm.go` narrows `sa.ESPGroup.Proposals` to it. Verified at HEAD |
| 2 | BLOCKER | AC-3's length refusal silenced the only test of the MODP small-value guard | `TestDHInvalidPublicKey` (`crypto/dh_test.go`) | Fixed 2026-08-04. Each case pads to `modp2048Len` and asserts `ErrInvalidPublicKey` AND not `ErrPublicKeyLength`. Verified at HEAD |
| 3 | ISSUE | A tag credited `RFC7296-1.3-2` with evidence for a different rule | `TestNegSharedSecretRefusesWrongLength` (`engine/rfc7296_negotiation_test.go`) | Fixed. The test carries no `RFC requirement:` tag at HEAD, and the comment above it states why. `RFC7296-1.3-2` is proven by `TestNegRekeyRejectsMismatchedKEGroup` |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/ike/engine/rfc7296_negotiation_test.go` | Yes | `git grep -n "^func TestNeg"` returns six tests in it |
| `internal/component/ike/engine/initiator.go` | Yes | `espConfigForAccepted` and `verifyAcceptedOffer` both defined in it |
| `internal/component/ike/crypto/dh.go` | Yes | `ErrPublicKeyLength` defined in it |
| `internal/component/ike/crypto/dh_test.go` | Yes | `padBigInt(c.value, modp2048Len)` in `TestDHInvalidPublicKey` |
| `internal/component/ike/crypto/rfc7296_verify_test.go` | Yes | carries two `RFC7296-3.3.6-3` polarities |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2 | The responder answers a KE mismatch with INVALID_KE_PAYLOAD and derives no keys | `git grep -n "NotifyInvalidKEPayload" internal/component/ike/engine/rekey.go` -> one hit inside `respondIKERekey`. Its caller in `inbound.go` returns on a nil new SA |
| AC-3 | A wrong-length peer value is refused with a named error | `git grep -n "ErrPublicKeyLength" internal/component/ike/crypto/dh.go` -> the definition plus two return sites, one per branch |
| AC-4, AC-5 | The IKE rekey branch resolves the collision, and the lower nonce abandons | `git grep -n "localNonceIsLower" internal/component/ike/engine/inbound.go` -> two call sites; the IKE one sits under `p.kind == rekeyIKE` |
| AC-6 | The survivor inherits every Child SA | `TestNegSurvivingSAInheritsChildren`, tagged `RFC7296-2.8.2-1` on both polarities |
| AC-7 | One helper, four initiator response paths | `git grep -n "verifyAcceptedOffer" internal/component/ike/engine/` -> the definition at `initiator.go` plus exactly four non-test call sites: `fsm.go` twice, `rekey.go` twice |
| AC-8 | Each of the four stops the exchange | Read all four: `handleSAInitResponse` sets `StateDead` and returns; `handleAuthResponse` calls `sendIKESATeardown` with NO_PROPOSAL_CHOSEN, sets `StateDead` and returns; both apply paths return `fmt.Errorf("...: %w", err)` |
| AC-9, AC-10, AC-11 | Each row carries both polarities | `git grep -n "RFC7296-1.3-2 \|RFC7296-2.8.2-1 \|RFC7296-3.3.6-3"` -> positive and negative for each; `RFC7296-3.3.6-3` in three files |
| (beyond plan) | The Child SA is keyed from the accepted offer | `git grep -n "ESPConfig"` -> `rekey.go` reads it into `prop`, `fsm.go` narrows `sa.ESPGroup.Proposals` to it |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| CREATE_CHILD_SA request whose KEi group differs from the chosen group | none | Proven in Go by `TestNegRekeyRejectsMismatchedKEGroup`. No user-facing surface exposes a refused rekey negotiation, and the spec's Functional Tests section says so |
| Inbound IKE rekey while ours is in flight | none | Proven in Go by `TestNegIKERekeyCollisionResolves`. A simultaneous collision needs both ends to start inside one round trip, which no `.ci` can force |
| IKE rekey response naming a suite we never proposed | none | Proven in Go by `TestNegInitiatorRejectsUnproposedOffer`. The `.ci` suite is the regression net that the daemon still establishes and rekeys |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `wire.PayloadKE` carries `DHGroup`, and `respondIKERekey` reads it to make the comparison AC-1 names |
| A-2 | confirmed | `padBigInt` pads to the group's modulus length. `TestDHInvalidPublicKey` derives `modp2048Len` rather than hardcoding 256 |
| R-1 | confirmed and settled in writing | The "R-1 Settled" section above chooses refusal over left-padding, and states the cost. The 2026-08-03 RFC lens graded the choice OVER-STRICT and added no new fact, so it was recorded rather than reopened |
| R-2 | confirmed | `TestNegIKERekeyCollisionResolves` drives both orderings and asserts the survivor from each side |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| RFC compliance disclosure | `rfc/short/rfc7296.md` carries rows `RFC7296-1.3-2`, `RFC7296-2.8.2-1` and `RFC7296-3.3.6-3`, each `[MUST]` | Yes |
| No anchored `docs/` claim points at a changed producer | `grep -rn "spec-fixit-ike-negotiation-conformance" docs/` returns nothing, and no `<!-- source: -->` anchor names `initiator.go`, `fsm.go`, `rekey.go` or `crypto/dh.go` | Yes |
| Config, CLI, API, plugin SDK, wire format, comparison table, test infrastructure, architecture | No change. These are negotiation-failure paths with no operator-visible surface | Yes |

## Closure 2026-08-12

The code this spec owns was committed by earlier sessions. This closure reviewed the tree
at HEAD, not a working-tree diff.

### Gates

| Gate | Run? |
|------|------|
| `make ze-verify` | NOT run. The closure session was instructed to run no test suite |
| `make ze-verify-changed`, `ze-plugin-test`, QEMU, Docker, `bin/ze-test` | NOT run, same instruction |
| `make ze-rfc-check` | NOT run |
| `make ze-tracked-build-check` | Not applicable. This closure carries no Go |
| `review_gate.py check` | Run, clean |
| `python3 scripts/dev/spec-citation-check.py` | Run, OK |

The commit script is prepared with `--unverified`, because this is a shared checkout whose
working tree carries several other sessions' half-finished edits, and this closure changes
no code.

### What this closure does not claim

RFC 7296 Section 2.8.1 resolves a collision from the lowest of the FOUR nonces, after both
responses have arrived. `handleCreateChildSAOwned` compares TWO at request-receipt time, and
the arm that keeps our own exchange answers the peer nothing, against Section 2.21.3.
Neither defect is this spec's, and neither is closed here. Both are recorded in
`spec-fixit-ike-rekey-collision-inverted`, which stays OPEN and which needs an owner ruling
before it can be fixed. This spec's AC-4, AC-5 and AC-6 are met as they are worded: the
branch routes, the lower nonce abandons, and the survivor inherits.

## Core Insight

A check whose result is discarded detects a disagreement and then commits the same
disagreement one line later. `verifyAcceptedOffer` matched the accepted offer against every
proposal ze sent, passed, and the caller then keyed from `Proposals[0]`. The tunnel came up
and black-holed ESP, which is worse than a refused negotiation because it looks like
success. A verifier that returns nothing but an error is the shape to distrust: what it
learned has nowhere to go.
