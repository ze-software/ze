# EAP authentication and NAT traversal

Site-to-site peers authenticate with a pre-shared key or with X.509. A road
warrior client uses the IKEv2 client built into its operating system, and those
default to EAP. NAT traversal is the other half: without it an IPsec tunnel
fails whenever either peer sits behind a NAT device.

<!-- source: internal/component/ike/eap/eap.go -- Session, Method, MethodResult, Packet -->
<!-- source: internal/component/ike/eap/eap_mschapv2.go -- EAP-MSCHAPv2 method -->
<!-- source: internal/component/ike/eap/eap_tls.go -- tlsMethod, tlsFragmenter, exportEAPTLSMSK -->
<!-- source: internal/component/ike/transport/nat.go -- NATDetectionHash, DetectNAT, AddNonESPMarker, StripNonESPMarker -->
<!-- source: internal/component/ike/transport/keepalive.go -- Keepalive -->
<!-- source: internal/component/ike/engine/eap_auth.go -- ComputeAuthFromMSK, VerifyAuthFromMSK, NewEAPSession -->

## RFC obligations carried by this code

- RFC 3748 defines the EAP framework: the packet format, the identity exchange,
  and the request and response alternation. The framework tests are tagged
  against it.
- RFC 5216 Section 2.1.3 requires a fatal alert to be delivered to the peer
  before the exchange ends. See the trap on two-round producers below.
- RFC 2759 Section 9 gives the MS-CHAPv2 test vectors. The magic constants are
  used WITHOUT the trailing null byte. The RFC's C declarations size the arrays
  at 39 and then give 40 initializers, which is a defect in the RFC text.
  hostapd and strongSwan both exclude the null, and including it fails interop.
- RFC 1320 defines MD4, which MS-CHAPv2 needs for the NT password hash.

<!-- source: internal/component/ike/eap/mschapv2.go -- ntPasswordHash, GenerateNTResponse, GenerateAuthenticatorResponse, DeriveMSK -->
<!-- source: internal/component/ike/eap/md4.go -- md4Sum -->

## Decisions

**MD4 is implemented here, not taken from a dependency.** Go removed
`crypto/md4` from the standard library and from `x/crypto`. The implementation
is short and is checked against the RFC 1320 test vectors. Any protocol that
needs MD4, MS-CHAPv2 and NTLM among them, has to bring its own.

**EAP session state is stored on the SA as `any`.** Importing the eap package
into the engine package would create an import cycle.

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
