# EAP as the IKE initiator

Ze drives EAP-MSCHAPv2 and EAP-TLS from the initiator seat, against an
authenticator such as strongSwan. The responder half is described in
`docs/architecture/ike/ipsec-14-responder.md`.

<!-- source: internal/core/eap/peer.go -- PeerSession, NewPeerSession, NewPeerSessionTLS, Process -->
<!-- source: internal/component/ike/engine/fsm.go -- startEAPExchange, handleEAPResponse, buildPeerTLSConfig -->
<!-- source: internal/component/ike/engine/auth.go -- buildAuthRequest, buildEAPResponse, buildEAPAuthMessage -->
<!-- source: internal/component/ike/engine/eap_auth.go -- computeEAPAuth, eapAuthSecret -->

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
- RFC 9190 Section 5.4 requires that "when EAP-TLS is used with TLS 1.3, the
  revocation status of all the certificates in the certificate chains MUST be
  checked (except the trust anchor)". The obligation binds both roles, so one
  function serves both: `checkChainRevocation` runs from the authenticator's
  `tls.Config.VerifyConnection` and from the peer's.
- RFC 9190 Section 2.1.3 leaves resumption to each end: "It is up to the EAP-TLS
  peer to use resumption". The peer offers a ticket when its peering carries a
  resumption store, which is what the `session-resumption` leaf decides.
- RFC 9190 Section 5.7 caps how long a peer keeps what a resumption needs at
  604800 seconds, "regardless of the PSK or ticket lifetime". `eap.Resumption`
  drops a stored ticket at that age on both `Put` and `Get`.

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

**The peer's ticket cache belongs to ONE peering, and that is a security
requirement.** `Conn.clientSessionCacheKey` keys on `ServerName` and falls back
to the transport's remote address when there is none. EAP-TLS carries no server
hostname, so ze sets none, and `eapTLSTransport.RemoteAddr` answers the constant
`"eap"`. Every EAP-TLS connection in the process therefore files its ticket under
one key, so a cache shared between peerings would offer one authenticator's
ticket, with its cached certificate chain, on another's session. The engine looks
the store up by peer name and rebuilds it whenever the peer's authentication
config changes.

<!-- source: internal/core/eap/resumption.go -- Resumption, Resumption.ClientCache -->
<!-- source: internal/component/ike/engine/resumption.go -- resumptionFor, forgetResumption -->

## Traps this code exists to avoid

**A resumed session skips every certificate check unless the peer rebuilds the
chain.** Go hands a resumed client no server Certificate message and calls
`VerifyPeerCertificate` never, so `serverChainCheck.chains` is empty; and
`Conn.loadSession` skips its own cached-chain sweep because this peer sets
`InsecureSkipVerify`. `serverChainCheck.rebuildResumedChains` is therefore the
ONLY thing that revalidates the cached chain on this role. It rebuilds from
`ConnectionState.PeerCertificates` at the CURRENT time and hands the result to
the Section 5.4 revocation gate. Reading `ConnectionState.VerifiedChains` instead
would answer nothing, because `InsecureSkipVerify` stops crypto/tls filling that
field on a full ze handshake too.

<!-- source: internal/core/eap/peer_chain.go -- serverChainCheck.verifyConnection, serverChainCheck.rebuildResumedChains -->

**A peer that stops reading after the handshake stores no ticket.** A
NewSessionTicket is a post-handshake message, and Go processes one only from
inside `Conn.Read`. `PeerSession.consumePostHandshakeRecords` is the reader that
keeps running after `HandshakeContext` returns; it also decrypts the RFC 9190
Section 2.5 indication, reports it in `PeerResult.Indication` for the operator's
log line, and requires nothing of it.

<!-- source: internal/core/eap/peer.go -- PeerSession.runTLSClient, PeerSession.consumePostHandshakeRecords -->

**EAP-TLS fragment reassembly needs a bound.** The reassembly buffer and the
peer-side buffered total are both capped. An unbounded reassembler is a memory
exhaustion path an unauthenticated peer can drive.

<!-- source: internal/core/eap/eap_tls.go -- tlsFragmenter.reassemble, eapTLSMaxReassembly, eapTLSMaxPeerBuffered -->

**A trust anchor must be checked, not assumed.** The peer verifies the server
chain against the configured roots through an explicit callback. `serverChainCheck`
holds that callback and the revocation gate together, because the two crypto/tls
callbacks see different halves of what the checks need: `verifyPeerCertificate`
is where the chain is BUILT (EAP carries no server hostname, so the config sets
`InsecureSkipVerify` and crypto/tls builds none), and `verifyConnection` is where
the NEGOTIATED VERSION is known, which is what Section 5.4's "when EAP-TLS is
used with TLS 1.3" turns on.

<!-- source: internal/core/eap/peer.go -- serverChainCheck, PeerTLSConfig -->

**A chain nobody can answer for is refused, not admitted.** On TLS 1.3 an end
holding no revocation list performs no check, and an unchecked chain is what the
Section 5.4 MUST forbids, so the handshake fails with a message naming the
remedy. A list that revokes nothing is a real answer and completes the session.
The two states are kept apart all the way from the config: `CACertEntry.CRLPEM`
answers nil for a CA with no list. TLS 1.2 is governed by RFC 5216 Section 5.4
instead, which asks only that the implementation "MUST support the use of
Certificate Revocation Lists (CRLs)", so a TLS 1.2 session with no list
completes.

<!-- source: internal/core/eap/revocation.go -- crlSet, parseCRLs, checkChainRevocation, crlSet.checkChain, crlSet.currentListFrom, revocationEntry -->
<!-- source: internal/component/ike/engine/responder_eap.go -- eapTLSServerConfig -->

**A round-trip cap keeps a broken authenticator from looping forever.**

<!-- source: internal/core/eap/peer.go -- maxEAPRounds -->

**An MS-CHAPv2 Success packet is a claim, not a proof.** The peer recomputes the
Authenticator Response from the Authenticator Challenge and the NT-Response it
retained, compares it with the S= value in constant time, and ends the session
on every other shape, the packet with no Message included. A peer that only
checks that 40 characters parse as hexadecimal authenticates against any
responder at all.

<!-- source: internal/core/eap/peer.go -- handleMSCHAPv2Success, parseAuthenticatorResponse -->

**An EAP-Success is a claim too, and the peer reads it only after the method
conversation concluded.** RFC 3748 Section 4.2 makes the peer discard a Success
sent before that point, so a rogue authenticator cannot skip the method by
answering "Success" first. `PeerSession.Process` switched on the Code before it
read the state until 2026-09-01, and a Success arriving at the identity round
returned `Done` with an all-zero MSK. Two packets share that guard: a Failure
arriving after both ends indicated success is dropped as well, and a Code
outside 1-4 is dropped by the peer and by the authenticator.

A discard is not an error. `handleEAPResponse` (`internal/component/ike/engine`)
puts the SA in `StateDead` for any non-nil `PeerResult.Err`, so a discard that
reported one would trade the bypass above for a denial of service: one forged
packet would end the exchange. The SA is left alone, it stays in
`StateEAPInProgress`, and `maxEAPRounds` still counts the round, so a flood ends
the exchange rather than holding it open.

**The silence is owed to the authenticator, not to the operator.** The drop is
`PeerResult.Discarded`, a field rather than the absence of the other three
outcomes, because dropping a packet and falling out of a branch nobody wrote
look identical on the wire: each sends nothing and each ends no exchange. The
caller logs `ike: EAP packet discarded` with the Code, so an operator whose peer
is being fed forged EAP-Success packets learns it.

<!-- source: internal/core/eap/peer.go -- PeerSession.Process, peerStateMethodDone, peerDiscard -->
<!-- source: internal/core/eap/eap.go -- Session.Process -->

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

`eap-nak-method-negotiation` puts strongSwan in front of ze offering an
authentication Type ze does not run. It proves one thing: strongSwan reads ze's
Type-3 Nak as a Nak rather than as a malformed Response. The scenario waits for
`initiating EAP_MD5 method`, which is charon offering Type 4, then for
`EAP/RES/NAK`, which is charon's own summary of the Response it decoded. Neither
end then installs an XFRM SA.

The scenario proves nothing about the Type the Nak asks for. The strongSwan
image includes no `eap-dynamic` plugin, which is the only charon plugin that
answers a received Nak by offering another method. So charon ends the exchange
instead of switching method. Two other tests prove the desired-Type octet.
`rfc3748_nak_test.go` `TestNakNamesTheConfiguredMethod` reads it from the
encoded Response. `test/ipsec/ipsec-eap-nak-unacceptable-type.ci` reads it from
ze's own authenticator, which logs `the peer refused type 13 with a Nak asking
for type 26`.

`responder-eap-tls13-revoked-client` is `responder-eap-tls13` with one file
changed: the CRL ze holds for the trusted CA lists the strongSwan client
certificate's serial number. Ze is the EAP-TLS SERVER in both, the client
certificate is valid and issued by that CA in both, and the sibling scenario
establishes with the same material. So the refusal this one asserts can only be
the revocation check. Ze logs the certificate, its serial number, the CA that
withdrew it and RFC 9190 Section 5.4, and neither end installs an XFRM SA.

Ze writes that line on the round that sends the fatal TLS alert, which is what
makes it readable here at all. charon abandons the exchange after the alert
rather than answering it, so the EAP-Failure round never happens, and a refusal
reported there would leave ze saying nothing but the 30s handshake timeout. The
scenario asserts both accounts: charon's, which is ze's wire output read by
another implementation, and ze's own, which is what an operator has.

<!-- source: internal/core/eap/eap_tls.go -- exportEAPTLSMSK, eapTLS12ExportRefused -->
<!-- source: internal/core/eap/peer.go -- naks, nakResponse -->
<!-- source: internal/le/interoplab/ipsec/checkers.go -- checkEAPNakMethodNegotiation, eapNakFacts, checkResponderEAPTLS13RevokedClient -->
