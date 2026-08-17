# EAP as the IKE initiator

Ze drives EAP-MSCHAPv2 and EAP-TLS from the initiator seat, against an
authenticator such as strongSwan. The responder half is described in
`docs/architecture/ike/ipsec-14-responder.md`.

<!-- source: internal/component/ike/eap/peer.go -- PeerSession, NewPeerSession, NewPeerSessionTLS, Process -->
<!-- source: internal/component/ike/engine/fsm.go -- startEAPExchange, handleEAPResponse, buildPeerTLSConfig -->
<!-- source: internal/component/ike/engine/auth.go -- buildAuthRequest, buildEAPResponse, buildEAPAuthMessage -->
<!-- source: internal/component/ike/engine/eap_auth.go -- ComputeAuthFromMSK -->

## RFC obligations carried by this code

- RFC 7296 Section 2.16 requires the initiator to omit the AUTH payload from the
  first IKE_AUTH request when it wants to use EAP. Omitting AUTH is the signal
  of EAP willingness. `buildAuthRequest` always included AUTH before this, so
  the EAP path needed a conditional.
- RFC 7296 Section 2.16 requires the initiator to verify the responder's AUTH
  payload BEFORE it processes any EAP request. Without that order a rogue server
  harvests credentials. `handleAuthResponse` verifies the server AUTH first,
  then creates the EAP session.
- RFC 7296 Section 2.3 governs the message ID, which must increment across the
  EAP round trips inside the same IKE_AUTH exchange.

## Decisions

**The peer module is separate from the server module.** `eap/peer.go` handles
initiator-side dispatch. `eap/eap.go` stays the authenticator side. Keeping them
apart is what let the responder role reuse the server session unchanged.

**One test PKI is shared across scenarios.** Both EAP-MSCHAPv2 and EAP-TLS need
strongSwan to present a server certificate, so a single PKI directory with a
generation script serves every scenario.

**`StateEAPInProgress` is an explicit SA state.** `handleInbound` routes on it,
so the multi-round EAP loop is a state rather than a flag.

<!-- source: internal/component/ike/engine/sa.go -- SAState, SAState.String -->

## Traps this code exists to avoid

**EAP-TLS fragment reassembly needs a bound.** The reassembly buffer and the
peer-side buffered total are both capped. An unbounded reassembler is a memory
exhaustion path an unauthenticated peer can drive.

<!-- source: internal/component/ike/eap/eap_tls.go -- tlsFragmenter.reassemble, eapTLSMaxReassembly, eapTLSMaxPeerBuffered -->

**A trust anchor must be checked, not assumed.** The peer verifies the server
chain against the configured roots through an explicit callback.

<!-- source: internal/component/ike/eap/peer.go -- verifyServerChain, PeerTLSConfig -->

**A round-trip cap keeps a broken authenticator from looping forever.**

<!-- source: internal/component/ike/eap/peer.go -- maxEAPRounds -->

## Proof

`test/interop-ipsec/scenarios` carries `03-eap-mschapv2` and `04-eap-tls`, with
Ze as the initiator and strongSwan as the authenticator. The MS-CHAPv2 and MD4
primitives worked against strongSwan on the first run; what interop found was
elsewhere. See `docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md` for the five
defects that a same-implementation suite could not see.
