# The IKE responder role

Ze was initiator-only. The responder loop blocked on the stop channel, the
dispatcher dropped any packet with no SA table entry, and `connection-type
respond` was a silent black hole. The root cause was that message DIRECTION was
hardcoded to the initiator seat: the SK encrypt and decrypt keys, the key
derivation nonce order, the AUTH octets, and the ESP key roles.

## Direction is three concerns, not one

Generalizing "the SK crypto is the only hardcoded-direction piece" is not
enough. Three more sites depend on the role:

| Concern | Producer | Role rule |
|---------|----------|-----------|
| SK send and receive keys | `skSendEncKey`, `skRecvEncKey` | the initiator sends under SK_ei |
| Key derivation input order | `initiatorNonce`, `responderNonce` | absolute Ni then Nr, never Local then Remote |
| AUTH signed octets | `computeSignedOctets` | each party signs with its own ID |
| ESP KEYMAT halves | `ChildSA.LocalIsInitiator` | send and receive halves swap by role |

All four reduce to Remote and Local when the SA is the initiator, so an
initiator SA stays byte identical.

<!-- source: internal/component/ike/engine/auth.go -- computeSignedOctets, skSendEncKey, skRecvEncKey -->
<!-- source: internal/component/ike/engine/sa.go -- initiatorNonce, responderNonce -->
<!-- source: internal/component/ike/engine/child.go -- ChildSA, installChildSA -->

**A partial generalization compiles and passes same-role tests while silently
corrupting the other role.** When a hardcoded direction is generalized, every
role-dependent site has to move at once: send keys, receive keys, the key
derivation input order for both nonces and SPIs, and the downstream key-role
assignment.

## RFC obligations carried by this code

- RFC 7296 Section 2.15 requires the initiator's signed octets to carry Nr and
  the responder's signed octets to carry Ni, each party signing with its own ID.
  The earlier code appended Remote then Local nonces and picked the ID by
  `!isInitiator`. Both invert for a responder SA.
- RFC 7296 Section 2.7 and Section 3.3 require the SA payload of a response to
  carry exactly one proposal. The responder echoed the whole ESP group in SAr2
  and keyed the first child from proposal 0. `selectResponderESP` negotiates one
  proposal and narrows the SA's ESP group.

<!-- source: internal/component/ike/engine/responder.go -- selectResponderESP, matchOfferedESPProposal, buildAuthResponse -->

## Decisions

**Mirror the initiator FSM.** The responder SA is created by the dispatch
goroutine on an unsolicited IKE_SA_INIT request from a peer configured to
respond, advanced inline on that goroutine, and adopted into the owner loop by a
polling `runResponder` at establishment. The rejected alternative was to run the
handshake in the owner loop. Mirroring keeps one concurrency model. The SA
handoff is guarded by `setSA` and `getSA`, and one handshake per peer is
enforced by a busy flag.

<!-- source: internal/component/ike/engine/register.go -- tryResponderSAInit, matchResponderPeer -->
<!-- source: internal/component/ike/engine/responder.go -- handleResponderInbound, handleSAInitRequest, handleAuthRequest -->
<!-- source: internal/component/ike/engine/reconcile.go -- setSA, getSA -->

**Reuse the EAP server.** `eap.Session` was implemented and unit tested with no
callers. The responder wires it through `NewEAPSession`. The responder
authenticates itself with its long-term credential in the first IKE_AUTH, runs
EAP, then exchanges an MSK-derived AUTH.

<!-- source: internal/component/ike/engine/eap_auth.go -- NewEAPSession, ComputeAuthFromMSK -->
<!-- source: internal/component/ike/engine/responder_eap.go -- computeServerAuth, startResponderEAP, handleResponderEAP -->

**Responder child install is asymmetric.** The responder installs the first
Child SA inside `handleAuthRequest`, because it must reply with SAr2 and TSr.
`runEstablished` therefore adopts the existing child for a responder SA instead
of calling `createFirstChildSA`.

**IKE rekey responder makes before breaking.** `respondIKERekey` derives the new
IKE SA with the peer as the rekey initiator, replies under the OLD keys, holds
the new SA pending, and swaps when the peer's INFORMATIONAL Delete of the old SA
arrives.

<!-- source: internal/component/ike/engine/rekey.go -- respondIKERekey, applyIKERekeyResponse -->
<!-- source: internal/component/ike/engine/reconcile.go -- setPendingIKESwap -->

## Traps this code exists to avoid

**A new responder role needs a half-open timeout.** A peer that sends
IKE_SA_INIT, receives the response, then abandons before IKE_AUTH left the SA
stuck: the busy flag pinned true and every later IKE_SA_INIT from that peer was
dropped, including its restarted reconnect. That is a permanent per-peer wedge
and a one-packet denial of service. The initiator self-heals through its
retransmission budget; the responder had no equivalent. `reapStaleHandshake`
tears the SA down after 30 seconds. It must re-check the established state
first, because the dispatch goroutine can complete the handshake between the
state switch and the reap, which would orphan a just-established tunnel and leak
its Child SA.

<!-- source: internal/component/ike/engine/fsm.go -- runResponder, reapStaleHandshake -->

**An IP literal identity must be ID_IPV4_ADDR, not ID_FQDN.** `buildIDPayload`
always emitted ID_FQDN. Initiator scenarios masked it because the peer had no
identity constraint. A constrained peer rejects the responder IDr.
`encodeIKEID` picks the address type. Only an interop run found this: the
in-process tests use a non-IP peer name and passed either way.

<!-- source: internal/component/ike/engine/auth.go -- buildIDPayload, encodeIKEID -->

**An unlocked read of a mutable SA.** Making the peer session's SA mutable from
the dispatch goroutine left `TerminateAllSAs`, `TerminatePeerSA` and the
reconcile stop path reading it unlocked. `Stop()` joins the session goroutine,
not dispatch. Every reader goes through `getSA`.

<!-- source: internal/component/ike/engine/register.go -- TerminateAllSAs, TerminatePeerSA -->

**A superseded pending swap leaks keys.** A peer that re-initiates an IKE rekey
before deleting the old SA orphans the first new SA's keys. `setPendingIKESwap`
clears the superseded keys.

## Proof

`test/interop-ipsec/scenarios` carries `07-responder-psk` and
`09-responder-ike-rekey` against strongSwan 5.9.14. The deterministic,
host-independent proof is the in-process end-to-end handshake: both peers' last
sent message is driven into the other through `handleInbound`, for a full PSK
handshake, a full EAP-MSCHAPv2 handshake and an IKE rekey. Interop against a
real peer catches what a self-consistent implementation cannot, which here was
the identity type constraint.
