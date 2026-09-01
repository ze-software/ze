# EAP authentication and NAT traversal

Site-to-site peers authenticate with a pre-shared key or with X.509. A road
warrior client uses the IKEv2 client built into its operating system, and those
default to EAP. NAT traversal is the other half: without it an IPsec tunnel
fails whenever either peer sits behind a NAT device.

<!-- source: internal/component/ike/eap/eap.go -- Session, Method, MethodResult, Packet -->
<!-- source: internal/component/ike/eap/eap_mschapv2.go -- EAP-MSCHAPv2 method -->
<!-- source: internal/component/ike/eap/eap_md5challenge.go -- EAP MD5-Challenge method -->
<!-- source: internal/component/ike/eap/eap_tls.go -- tlsMethod, tlsFragmenter, exportEAPTLSMSK -->
<!-- source: internal/component/ike/transport/nat.go -- NATDetectionHash, DetectNAT, AddNonESPMarker, StripNonESPMarker -->
<!-- source: internal/component/ike/transport/keepalive.go -- Keepalive -->
<!-- source: internal/component/ike/engine/eap_auth.go -- computeEAPAuth, eapAuthSecret, computeAuthFromSharedSecret, verifyAuthFromSharedSecret, newEAPSession -->
<!-- source: internal/component/ike/ipsec/validate.go -- IsEAPMode, IsEAPPasswordMode -->

## RFC obligations carried by this code

- RFC 3748 defines the EAP framework: the packet format, the identity exchange,
  and the request and response alternation. The framework tests are tagged
  against it.
- RFC 3748 Section 5 states: "All EAP implementations MUST support Types 1-4,
  which are defined in this document, and SHOULD support Type 254." Ze supports
  Type 1 (Identity), Type 2 (Notification), Type 3 (Nak) and Type 4
  (MD5-Challenge). Type 4 carried an authorized deviation dated 2026-08-30.
  Thomas withdrew that authorization on 2026-09-01 and ordered Type 4
  implemented. The withdrawal is recorded in
  `plan/journal/gate-excludes-part-of-its-population.md`.
- The withdrawn deviation rested on RFC 7296 Section 2.16: "EAP methods that do
  not establish a shared key SHOULD NOT be used, as they are subject to a number
  of man-in-the-middle attacks". That sentence governs USE. It discharges no
  obligation to SUPPORT the Type. Section 5.4 applies the MD5-Challenge
  requirement to an authenticator that authenticates peers locally, and ze's
  authenticator does that.
- Type 4 is SELECTABLE, by `authentication { mode eap-md5 }` (2026-09-01). It is
  never a default and no other mode reaches it. `eapMethodType`
  (`internal/component/ike/engine/eap_auth.go`) is the one place a mode becomes a
  method Type. The authenticator, the peer and the warning all read it.
  `warnKeylessEAPModes` writes that warning once for each configuration that
  adopts a keyless method. Section 2.16 states a SHOULD NOT rather than a MUST
  NOT, so the choice is the operator's. The warning is what ze owes them.
- MD5-Challenge and MS-CHAPv2 both authenticate on one configured password.
  `ipsec.IsEAPPasswordMode` (`internal/component/ike/ipsec/validate.go`) is the
  one place that fact is declared. Five layers read it:

  | Layer | What it does with the answer |
  |-------|------------------------------|
  | `parseEAPUser` (`ipsec/config.go`) | reads the eap-user `password` leaf |
  | `parseAuthConfig` (`ipsec/config.go`) | reads the peer `pre-shared-secret` leaf |
  | `ValidateRemoteAccess` (`ipsec/validate.go`) | refuses an empty credential |
  | `eapMethodConfig` (`responder_eap.go`) | builds the authenticator's method |
  | `startEAPExchange` (`fsm.go`) | builds the peer's session |

  Each layer held its own copy of the list until 2026-09-01. A further password
  method then needed five edits, and one missed edit failed in silence.
- The AUTH payloads of an MD5-Challenge exchange come from SK_pi and SK_pr, not
  from an MSK. RFC 7296 Section 2.16: "If EAP methods that do not generate a
  shared key are used, the AUTH payloads in messages 7 and 8 MUST be generated
  using SK_pi and SK_pr, respectively." `eapAuthSecret` (`eap_auth.go`) asks the
  method through `eap.TypeDerivesKey` rather than reading the MSK array, because
  an all-zero MSK is also what a failed derivation leaves behind.
- RFC 3748 Section 5.2 makes the peer answer a Notification Request with a
  Notification Response, and forbids a Nak in answer to one. A Notification is
  not an error indication, so the peer state and every method field stay
  unchanged across it.
- RFC 3748 Section 5.3.1 makes the peer answer a Request for an authentication
  Type it does not run (4-253 and 255) with a Nak naming the Type it does run.
- RFC 3748 Section 5.7 sends a peer that cannot interpret an Expanded Type to
  Section 5.3.1, so a Type-254 Request draws the legacy Nak. Ze reads the Type
  octet of such a Request and nothing else of it, and composes no Expanded Nak.
- RFC 3748 Section 2.1 closes the Nak: "A peer MUST NOT send a Nak (legacy or
  expanded) in reply to a Request after an initial non-Nak Response has been
  sent."
- RFC 5216 Section 2.1.3 requires a fatal alert to be delivered to the peer
  before the exchange ends. See the trap on two-round producers below.
- RFC 2759 Section 6 defines the MS-CHAPv2 Failure packet. A refused
  NT-Response is answered with it, carrying `E=691`, `R=0`, a fresh 32-digit
  `C=` challenge, `V=3` and `M=`, and the exchange ends behind that packet.
- RFC 2759 Section 9 gives the MS-CHAPv2 test vectors. The magic constants are
  used WITHOUT the trailing null byte. The RFC's C declarations size the arrays
  at 39 and then give 40 initializers, which is a defect in the RFC text.
  hostapd and strongSwan both exclude the null, and including it fails interop.
- RFC 1320 defines MD4, which MS-CHAPv2 needs for the NT password hash.

<!-- source: internal/component/ike/eap/mschapv2.go -- ntPasswordHash, GenerateNTResponse, GenerateAuthenticatorResponse, DeriveMSK -->
<!-- source: internal/component/ike/eap/md4.go -- md4Sum -->

## The two packets the framework composes

Type 2 and Type 3 are answered by the framework rather than by a method, so
`PeerSession.handleRequest` builds both packets and no method sees the Request.

A Notification Response is five octets and carries no Type-Data. RFC 3748
Section 5.2: "A Response MUST be sent in reply to the Request with a Type field
of 2 (Notification).  The Type-Data field of the Response is zero octets in
length."

| Offset | Bytes | Field | Value |
|--------|-------|-------|-------|
| 0 | 1 | Code | 2 (Response) |
| 1 | 1 | Identifier | The Notification Request's Identifier |
| 2 | 2 | Length | 5 |
| 4 | 1 | Type | 2 (Notification) |

A legacy Nak is six octets and names one desired Type. RFC 3748 Section 5.3.1:
"The Type-Data field of the Nak Response (Type 3) MUST contain one or more
octets indicating the desired authentication Type(s), one octet per Type, or the
value zero (0) to indicate no proposed alternative."

| Offset | Bytes | Field | Value |
|--------|-------|-------|-------|
| 0 | 1 | Code | 2 (Response) |
| 1 | 1 | Identifier | The refused Request's Identifier |
| 2 | 2 | Length | 6 |
| 4 | 1 | Type | 3 (Nak) |
| 5 | 1 | Desired Type | 26 for EAP-MSCHAPv2, 13 for EAP-TLS |

The desired octet is the configured method and is never the zero. Zero means "no
proposed alternative", and ze always holds the method the operator configured,
so naming it turns a refusal into the negotiation the section intends. A
Type-254 Request draws this same packet.

<!-- source: internal/component/ike/eap/peer.go -- handleRequest, notificationResponse, nakResponse, naks -->
<!-- source: internal/component/ike/eap/eap.go -- TypeNotification, TypeNAK, typeAuthenticationLow, typeAuthenticationHigh -->

## Decisions

**MD4 is implemented here, not taken from a dependency.** Go removed
`crypto/md4` from the standard library and from `x/crypto`. The implementation
is short and is checked against the RFC 1320 test vectors. Any protocol that
needs MD4, MS-CHAPv2 and NTLM among them, has to bring its own.

**EAP session state is stored on the SA as `any`.** Importing the eap package
into the engine package would create an import cycle.

**The Request Type decides the outcome before any method sees the packet.**
`PeerSession.handleRequest` produces four outcomes: a Notification Response, a
legacy Nak, a method dispatch, or a silent discard. Three of the four belong to
the framework rather than to a method. A router that sent every Type into the
method first, which this did until 2026-09-01, gave all four situations one
answer: an error that killed the IKE SA. Each method now reads its own opcode or
flags, because the Type is settled above it.

**The peer can Nak until it answers a method Request.** `methodCommitted` is RFC
3748 Section 2.1's "initial non-Nak Response", and the Identity Response does not
set it. Section 5.4 describes a Nak sent in answer to the Request that follows
the Identity Response, so a boundary that counted the Identity Response would
make the Section 5.3.1 Nak unreachable in every conversation ze can have. The
flag is set when the method ANSWERS a Request, not when one arrives, because a
method that discarded or failed has sent nothing. After the commitment an
authentication Request is discarded rather than refused with a Nak.

**The authenticator records what a Nak asked for.** `Session.nakRefused` writes
the desired Types and the offered Type into `Session.err` before it sends the
EAP-Failure. RFC 3748 Section 4.2 gives an EAP-Failure a Code, an Identifier and
a Length and no field for a reason, so those octets are the only word the peer
gets about why. Discarding them, which this did until 2026-09-01, left an
operator reading "authentication failed" with no way to learn that the far end
wanted a method ze does not run.

<!-- source: internal/component/ike/eap/peer.go -- handleRequest, commitMethod, methodCommitted -->
<!-- source: internal/component/ike/eap/eap.go -- Session.nakRefused, nakRefusal -->

**EAP-TLS runs Go's `crypto/tls` over a custom `net.Conn`.** The transport pipes
TLS records through EAP request and response packets. Implementing TLS again was
rejected.

**The virtual IP pool allocates sequentially from the CIDR base.** Random
allocation buys nothing here and makes debugging harder.

<!-- source: internal/component/ike/eap/pool.go -- Pool, Allocate, Release -->

**NAT detection hashes compare in constant time.** The comparison is not
security sensitive, because a SHA-1 NAT detection hash is public. The
constant-time form is what the rest of this codebase uses, so it is what this
uses.

## Traps this code exists to avoid

**MD4 round-function rotation indexes by the loop counter.** Using the word
index from the permutation table produces a hash that is wrong and looks
plausible.

**A hash length mismatch reads as "NAT present".** `DetectNAT` first took an
8-byte array for the received hash. NAT detection hashes are 20-byte SHA-1, so
every comparison failed and every peer looked NATed. The parameter is a slice.

**EAP-TLS is asynchronous by construction.** TLS runs in a goroutine that reads
and writes through the transport, so the transport needs mutex-protected buffers
and a notification channel.

**A result type with two fields whose consumer branches on one silently loses
the other.** `MethodResult` carries a response and an error. `Session.handleMethod`
tests the error first and answers with a failure packet. A fix that set BOTH
looked complete and put nothing on the wire: the EAP-TLS fatal alert that RFC
5216 Section 2.1.3 exists to deliver was dropped for two commits. Where a
protocol spends two rounds, the producer has to spend two rounds: park the
cause, send the packet, and report on the round after. Ze as EAP PEER needs the
mirror of the same fix, because the authenticator now waits for a reply that the
peer was discarding.

**A method's last word has its own field, `MethodResult.FinalRequest`.** The
paragraph above is why: a packet returned in `Response` beside a non-nil `Err`
is discarded. `FinalRequest` is read first, it fails the exchange as it goes
out, and it obliges `Err` to be set beside it. MS-CHAPv2 uses it for the Failure
packet RFC 2759 Section 6 defines. The difference from the parked cause above is
WHEN the exchange knows: with a last word the failure is recorded at once and no
later packet can undo it, where a parked cause leaves the session ignorant for a
round.

**A refusal still costs two rounds, and the lower layer is why.** RFC 3748
Section 4.2 makes the authenticator send an EAP-Failure after a failure result
indication, "regardless of the response from the peer", and RFC 7296 Section 2.16
gives each IKE_AUTH message one EAP payload. So the MS-CHAPv2 Failure packet and
the EAP-Failure cannot share a round. The authenticator parks in `stateLastWord`
and answers whatever comes back with the EAP-Failure; the peer acknowledges the
Failure packet with the OpCode alone so that round exists, and reports the E=
error code when the EAP-Failure arrives. A peer that ended the conversation on
the Failure packet would leave the authenticator no round to meet Section 4.2 in.

<!-- source: internal/component/ike/eap/eap.go -- MethodResult, Session.handleMethod, Session.failure -->
<!-- source: internal/component/ike/eap/eap_tls.go -- tlsMethod.Process, tlsMethod.Close -->
<!-- source: internal/component/ike/eap/peer.go -- PeerSession.handleTLSRequest, readAndSendTLS -->

## NAT traversal

When NAT is detected, the SA carries the flag and the child install sets the UDP
encapsulation attribute on the XFRM SA, which is what puts ESP inside UDP. The
flag propagates to child creation without a second decision.

<!-- source: internal/component/ike/engine/udpencap.go -- UDP encapsulation readiness -->
<!-- source: internal/component/ike/transport/encap_linux.go -- UDP encapsulation of ESP on Linux -->
<!-- source: internal/component/ike/dataplane/dataplane.go -- SAParams UDP encapsulation fields -->
