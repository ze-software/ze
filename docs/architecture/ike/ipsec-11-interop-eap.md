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
- RFC 2759 Section 5 requires the peer to verify the authenticator response in
  the MS-CHAPv2 Success packet, and to end the session when that response is
  missing or incorrect. This is the mutual half of MS-CHAPv2: only a party that
  knows the password hash can produce the S= value.

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

**An MS-CHAPv2 Success packet is a claim, not a proof.** The peer recomputes the
Authenticator Response from the Authenticator Challenge and the NT-Response it
retained, compares it with the S= value in constant time, and ends the session
on every other shape, the packet with no Message included. A peer that only
checks that 40 characters parse as hexadecimal authenticates against any
responder at all.

<!-- source: internal/component/ike/eap/peer.go -- handleMSCHAPv2Success, authenticatorResponse -->

## Proof

`test/interop-ipsec/scenarios` carries `eap-mschapv2` and `eap-tls`, with
Ze as the initiator and strongSwan as the authenticator. The MS-CHAPv2 and MD4
primitives worked against strongSwan on the first run; what interop found was
elsewhere. See `docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md` for the five
defects that a same-implementation suite could not see.

`eap-tls` runs against a STOCK strongSwan, which lands on TLS 1.2 and
negotiates no RFC 7627 extended master secret. Ze cannot derive the RFC 5216
Section 2.3 MSK there, so the scenario asserts the refusal rather than a tunnel:
the TLS handshake and every EAP-TLS fragment complete, ze logs one line naming
the peer, the negotiated version, RFC 7627 and the three remedies, and neither
end installs an XFRM SA. `eap-tls13` is the same exchange with
`charon.tls.version_max = 1.3` on the same image, and it carries the ESP
data-plane assertions.

<!-- source: internal/component/ike/eap/eap_tls.go -- exportEAPTLSMSK, eapTLS12ExportRefused -->
