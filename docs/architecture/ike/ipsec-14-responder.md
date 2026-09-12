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

`installChildSA` marks the dataplane busy for the length of its kernel writes,
so a read that spans an install answers unknown rather than naming a
half-installed pair as drift. It clears the child's removing flag once the pair
is in place.

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

- RFC 9190 Section 2.1.2 requires the EAP-TLS server to send one or more
  post-handshake NewSessionTicket messages in the initial authentication. The
  purpose clause ("To enable resumption") is not an antecedent a server escapes
  by declining resumption (owner ruling, 2026-09-05), so ze issues a ticket
  whatever the `session-resumption` leaf says.
- RFC 9190 Section 2.1.3 permits the server to require a full handshake instead.
  That is what `session-resumption false` does, and it refuses by answering no
  session from `tls.Config.UnwrapSession`, which crypto/tls turns into a full
  handshake. Refusing from `VerifyConnection` would send a fatal alert where
  Section 5.7 asks for the full handshake.

## Decisions

**The session-ticket key belongs to the PEERING, not to the EAP exchange.**
`newTLSMethod` builds a fresh `tls.Config` per EAP session and Go keys ticket
encryption on the Config instance, so a per-session key issues tickets nothing
can ever redeem. The engine holds one `eap.Resumption` per configured peer,
keyed by peer name, and `eapTLSServerConfig` REFUSES an SA that reached it
without one rather than falling back to that dead key.

The store outlives the peer session on purpose. `clear vpn ipsec sa` destroys
every session and rebuilds it, so a store held on the session alone would be
discarded by the operator command most likely to be followed by a second
authentication. It is discarded when the peer leaves the config, and rebuilt
whenever the peer's authentication config changes: a resumed handshake presents
no certificate, so an operator who replaces the certificate, the CA or the
revocation lists must not have the old decision carried past the edit.

The key ROTATES. `Config.SetSessionTicketKeys` turns Go's own rotation off, so
`Resumption.TicketKeys` reproduces its schedule, minting a new encryption key
every 24 hours and keeping a retired one for 7 days. No key outlives the tickets
it protects, because crypto/tls refuses a ticket older than the same 7 days.

<!-- source: internal/core/eap/resumption.go -- Resumption.TicketKeys, resumptionKeyRotation -->
<!-- source: internal/component/ike/engine/resumption.go -- resumptionFor, resumptionStates -->
<!-- source: internal/component/ike/engine/responder_eap.go -- eapTLSServerConfig -->

**The authenticator answers a Certificate Status Request with what the operator
configured, and with nothing else.** RFC 9190 Section 5.4 requires an EAP-TLS
server on TLS 1.3 to implement Certificate Status Requests, and crypto/tls owns
the extension mechanics: it sends the response in the leaf CertificateEntry when
the client asked for one. What `eapTLSServerConfig` supplies is that response,
read from the `ocsp-response` leaf of the pki certificate this peer presents,
because the response is about that CERTIFICATE rather than about this peering.
An entry holding none answers with no status, which RFC 6066 Section 8 permits,
and the peer then checks the chain against the CA's `crl`. Nothing here judges
the response: RFC 6960 Section 3.2 makes that the relying party's job, and ze
does it as a peer (`eap.CheckCertificateStatus`).

**Resumption is one operator setting for both roles.** The `session-resumption`
leaf sits in the peer's `authentication` container, defaults to true, and gates
accepting a resumed session as the authenticator and offering a ticket as the
peer. It never gates issuing one. `parseSessionResumption` writes the default
rather than leaving it to the Go zero value, because `config.Tree.Get` answers
"absent" for a leaf the operator never set.

Whether an authentication resumed is reported in one log line per exchange and
in `ze_ipsec_eap_tls_resumption_hits_total` and
`ze_ipsec_eap_tls_resumption_misses_total`, both labelled by peer. Nothing else
reports it: no CLI command prints `DidResume`, and `show vpn ipsec sa` carries no
EAP field.

<!-- source: internal/component/ike/ipsec/config_auth_policy.go -- parseSessionResumption -->
<!-- source: internal/component/ike/engine/resumption.go -- recordEAPTLSAuthentication -->
<!-- source: internal/component/ike/engine/metrics.go -- countEAPTLSAuthentication -->

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
callers. The responder wires it through `newEAPSession`. The responder
authenticates itself with its long-term credential in the first IKE_AUTH, runs
EAP, then exchanges an MSK-derived AUTH.

<!-- source: internal/component/ike/engine/eap_auth.go -- newEAPSession, eapAuthSecret, computeEAPAuth -->
<!-- source: internal/component/ike/engine/responder_eap.go -- computeServerAuth, startResponderEAP, handleResponderEAP -->

**Responder child install is asymmetric.** The responder installs the first
Child SA inside `handleAuthRequest`, because it must reply with SAr2 and TSr.
`runEstablished` therefore adopts the existing child for a responder SA instead
of calling `createFirstChildSA`.

**The responder substitutes transport-mode selector addresses before its policy
lookup.** RFC 7296 Section 2.23.1: "the server should first check that the
initiator requested transport mode, and then do address substitution on the
Traffic Selectors", after which "the server does SPD lookup based on those new
Traffic Selectors". In Ze the policy match inside `narrowChildSelectors` IS that
SPD lookup, so `substituteResponderSelectors` runs immediately above it.

The verdict it reads is on the SA before any selector arrives: `detectResponderNAT`
runs during IKE_SA_INIT and writes `PeerBehindNAT` on a NAT_DETECTION_SOURCE_IP
mismatch and `BehindNAT` on a NAT_DETECTION_DESTINATION_IP mismatch. The observed
remote address it substitutes into TSi is `sa.peerEndpoint`, and on the EAP path
that is still nil when the first IKE_AUTH narrows, so it falls back to the
configured remote address. The two are the same address on this role:
`matchResponderPeer` accepts an unsolicited IKE_SA_INIT only from a source equal
to `remote-address`, so a peer behind a NAT is configured with its post-NAT
address.

Without this a conforming client behind a NAT proposes its pre-NAT address, the
policy naming the observed address intersects it nowhere, and the responder answers
TS_UNACCEPTABLE to every such client. `real-nat-transport-ze-responder` is the
scenario that measures it.

<!-- source: internal/component/ike/engine/ts_nat_substitute.go -- substituteResponderSelectors, observedRemoteAddress -->
<!-- source: internal/component/ike/engine/responder.go -- detectResponderNAT -->

**IKE rekey responder makes before breaking.** `respondIKERekey` derives the new
IKE SA with the peer as the rekey initiator, replies under the OLD keys, holds
the new SA pending, and swaps when the peer's INFORMATIONAL Delete of the old SA
arrives.

The pending slot MUST be empty once the owner loop that filled it has returned.
`runEstablished` releases it in the same deferred call that releases
`ps.pendingRekey`. The session outlives the SA -- `ps.run` loops `runOnce` on one
`PeerSession` per peer -- so a swap the peer never confirmed would otherwise keep
its `SK_*` material past the close of its connection, which RFC 7296 Section 2.12
forbids, and the next reconnect cycle would promote it: the swap branch keys on
the slot being occupied, never on which cycle filled it.

<!-- source: internal/component/ike/engine/rekey.go -- respondIKERekey, applyIKERekeyResponse -->
<!-- source: internal/component/ike/engine/reconcile.go -- setPendingIKESwap -->
<!-- source: internal/component/ike/engine/established.go -- runEstablished teardown defer -->

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
not dispatch. Every reader goes through `getSA`. Both terminate functions
advance the dataplane generation while they hold `peersMu`, so a kernel read
that spans a teardown answers unknown rather than drift.

<!-- source: internal/component/ike/engine/register.go -- TerminateAllSAs, TerminatePeerSA -->

**A superseded pending swap leaks keys.** A peer that re-initiates an IKE rekey
before deleting the old SA orphans the first new SA's keys. `setPendingIKESwap`
clears the superseded keys.

## Proof

`test/interop-ipsec/scenarios` carries `responder-psk` and
`responder-ike-rekey` against strongSwan 5.9.14. The deterministic,
host-independent proof is the in-process end-to-end handshake: both peers' last
sent message is driven into the other through `handleInbound`, for a full PSK
handshake, a full EAP-MSCHAPv2 handshake and an IKE rekey. Interop against a
real peer catches what a self-consistent implementation cannot, which here was
the identity type constraint.
