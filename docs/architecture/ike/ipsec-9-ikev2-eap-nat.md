# EAP authentication and NAT traversal

Site-to-site peers authenticate with a pre-shared key or with X.509. A road
warrior client uses the IKEv2 client built into its operating system, and those
default to EAP. NAT traversal is the other half: without it an IPsec tunnel
fails whenever either peer sits behind a NAT device.

<!-- source: internal/core/eap/eap.go -- Session, Method, MethodResult, Packet -->
<!-- source: internal/core/eap/eap_mschapv2.go -- EAP-MSCHAPv2 method -->
<!-- source: internal/core/eap/eap_md5challenge.go -- EAP MD5-Challenge method -->
<!-- source: internal/core/eap/eap_tls.go -- tlsMethod, tlsFragmenter, exportEAPTLSKeys -->
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
- RFC 3748 Section 7.10 asks a key-deriving method for two keys: "an EAP method
  supporting key derivation MUST export a Master Session Key (MSK) of at least 64
  octets, and an Extended Master Session Key (EMSK) of at least 64 octets."
  `exportEAPTLSKeys` (`internal/core/eap/eap_tls.go`) exports both for EAP-TLS. It
  asks the TLS exporter for the whole 128-octet Key_Material and cuts it where RFC
  5216 Section 2.3 cuts it, `MSK = Key_Material(0,63)` and `EMSK =
  Key_Material(64,127)`; RFC 9190 Section 2.3 keeps that split for TLS 1.3 and
  changes only the label and the context.
- Section 7.10 then confines the second key: "The EMSK is reserved for future use
  and MUST remain on the EAP peer and EAP server where it is derived; it MUST NOT
  be transported to, or shared with, additional parties, or used to derive any
  other keys." So the EMSK is an unexported field of `Session` and of
  `PeerSession`, `MethodResult` carries it in an unexported field and `PeerResult`
  carries it not at all, there is no accessor beside `Session.MSK`, and `Close`
  erases it. Go's visibility rule is the confinement: the IKEv2 carrier is in
  another package, so publishing the EMSK takes an edit inside
  `internal/core/eap`.
- EAP-MSCHAPv2 exports an EMSK too, and ze defines the derivation because no
  document does: draft-kamath-pppext-eap-mschapv2-02 Section 1 routes key
  derivation to RFC 3079, and neither writes the word EMSK. Nothing compares the
  value, because Section 7.10 says "The EMSK is not shared with the authenticator
  or any other third party" and Section 7.2.1 says "Use of the EMSK is reserved",
  so the key never reaches the wire and no interop rests on it.
- What DOES constrain it is Section 7.10's separation rule: "an attacker
  recovering the MSK or EMSK MUST NOT be able to recover the other quantity with
  a level of effort less than brute force." So `deriveEMSK`
  (`internal/core/eap/mschapv2.go`) does NOT derive the EMSK from the MSK. Both
  descend from the RFC 3079 Section 3 MPPE master key as siblings: the MSK by the
  two truncated SHA-1 MPPE keys, the EMSK by `HKDF-Expand(SHA-256, MasterKey,
  label, 64)`. Reaching either key from the other means inverting a one-way
  function to get back to the master key. Splitting the existing 64-octet MSK in
  two was not an option: Section 7.10 needs 64 octets on each side.
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

<!-- source: internal/core/eap/mschapv2.go -- ntPasswordHash, GenerateNTResponse, GenerateAuthenticatorResponse, DeriveMSK -->
<!-- source: internal/core/eap/md4.go -- md4Sum -->

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

<!-- source: internal/core/eap/peer.go -- handleRequest, notificationResponse, nakResponse, naks -->
<!-- source: internal/core/eap/eap.go -- TypeNotification, TypeNAK, typeAuthenticationLow, typeAuthenticationHigh -->

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

<!-- source: internal/core/eap/peer.go -- handleRequest, commitMethod, methodCommitted -->
<!-- source: internal/core/eap/eap.go -- Session.nakRefused, nakRefusal -->

**EAP-TLS runs Go's `crypto/tls` over a custom `net.Conn`.** The transport pipes
TLS records through EAP request and response packets. Implementing TLS again was
rejected.

**The virtual IP pool allocates sequentially from the CIDR base.** Random
allocation buys nothing here and makes debugging harder.

<!-- source: internal/core/eap/pool.go -- Pool, Allocate, Release -->

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
5216 Section 2.1.3 exists to deliver was dropped for two commits. Ze as EAP PEER
needs the mirror of the same fix, because the authenticator waits for a reply
that the peer was discarding.

**A method's last word has its own field, `MethodResult.FinalRequest`.** The
paragraph above is why: a packet returned in `Response` beside a non-nil `Err`
is discarded. `FinalRequest` is read first, it fails the exchange as it goes
out, and it obliges `Err` to be set beside it. Both methods that owe a refused
peer a packet use it: MS-CHAPv2 for the Failure packet RFC 2759 Section 6
defines, and EAP-TLS for the fatal alert of RFC 5216 Section 2.1.3.

**The cause is recorded on the round it is COMPUTED, never on the round after.**
EAP-TLS parked its certificate failure on a field the next round read, and that
next round is the one Section 2.1.3 makes the server wait for. The peer decides
whether it ever arrives: strongSwan abandons the exchange after ze's alert, so
ze's whole account of a revoked client certificate was the 30s handshake timeout,
and a refused certificate read exactly like a dead network. `FinalRequest` closed
it, because `Session.finalRequest` writes `Session.err` as the last word goes out.
`handleResponderEAP` (`internal/component/ike/engine`) reads that error on every
round and writes `ike: EAP authentication failed` on the round it first appears,
which is one line for one refusal whether or not the peer answers.

**A refusal still costs two rounds, and the lower layer is why.** RFC 3748
Section 4.2 makes the authenticator send an EAP-Failure after a failure result
indication, "regardless of the response from the peer", and RFC 7296 Section 2.16
gives each IKE_AUTH message one EAP payload. So the MS-CHAPv2 Failure packet and
the EAP-Failure cannot share a round. The authenticator parks in `stateLastWord`
and answers whatever comes back with the EAP-Failure; the peer acknowledges the
Failure packet with the OpCode alone so that round exists, and reports the E=
error code when the EAP-Failure arrives. A peer that ended the conversation on
the Failure packet would leave the authenticator no round to meet Section 4.2 in.

**The peer answers the closing round before it judges it.** On TLS 1.3 the
authenticator's last EAP-Request carries the RFC 9190 Section 2.5 protected
success result indication, and `readAndSendTLS` answers it with the no-data
EAP-Response step 4 asks for whatever the record held. The verdict is taken one
round later, on the EAP-Success, because RFC 5216 Section 2.1.3 makes the
authenticator wait for that response before it may conclude: a peer that refused
on the spot would leave the wait unsatisfied, which is the same reply-first
discipline `pendingErr` exists for. What the verdict requires is in
`ipsec-11-interop-eap.md`.

<!-- source: internal/core/eap/peer.go -- PeerSession.readAndSendTLS -->
<!-- source: internal/core/eap/peer_indication.go -- PeerSession.requireSuccessIndication -->

<!-- source: internal/core/eap/eap.go -- MethodResult, Session.handleMethod, Session.failure -->
<!-- source: internal/core/eap/eap_tls.go -- tlsMethod.Process, tlsMethod.Close -->
<!-- source: internal/core/eap/peer.go -- PeerSession.handleTLSRequest, readAndSendTLS -->
<!-- source: internal/component/ike/engine/responder_eap.go -- handleResponderEAP -->

## NAT traversal

When NAT is detected, the SA carries the flag and the child install sets the UDP
encapsulation attribute on the XFRM SA, which is what puts ESP inside UDP. The
flag propagates to child creation without a second decision.

The peer's `nat-traversal` policy separates capability from permission. `allow`
is the existing default. `prohibit` retains NAT detection but rejects translation
before establishment or MOBIKE migration. Its address-updating requests carry
`NO_NATS_ALLOWED`; the receiver checks the complete protected tuple against
packet metadata. Merely opening UDP 4500 or exchanging NAT detection hashes does
not cancel an explicit prohibition.

<!-- source: internal/component/ike/engine/udpencap.go -- UDP encapsulation readiness -->
<!-- source: internal/component/ike/transport/encap_linux.go -- UDP encapsulation of ESP on Linux -->
<!-- source: internal/component/ike/dataplane/dataplane.go -- SAParams UDP encapsulation fields -->

## The two IKE sockets

`UDPTransport` (`internal/component/ike/transport/udp.go`) is one UDP socket
with a read loop. The engine opens two: the plain one on port 500 and the
NAT-T one on port 4500 (`NewUDPTransport`, `NewNATTTransport`). Each socket
knows its own role at construction (`IsNATT`) and stamps it on every inbound
`Packet.NATT`, so no handler infers the role from a port number, which reads
wrong under the `ze.test.ike.port` override. RFC 7296 Section 2.23: "The UDP
payload of all packets containing IKE messages sent on port 4500 MUST begin
with the prefix of four zeros", so a sender holding the NAT-T socket frames
its message with the non-ESP marker of RFC 3948 Section 2.2
(`AddNonESPMarker`) and the read loop strips it. Both sockets are IPv4
(`net.ListenUDP("udp4")`), so the socket options below are the IPv4 ones.

On a migration-capable backend, the NAT-T listener binds the wildcard so it can
receive traffic after an address disappears. `Packet.LocalAddr` is the actual
destination from packet-info ancillary data, not `0.0.0.0`. `SendFrom` and `SendDF`
select the SA's source through the same ancillary data. A supplied local port must
match the socket's bound port; the source address must match a concretely bound
socket. This also keeps non-MOBIKE peers on their configured source when they
share the wildcard listener.

All writes hold the transport's lock across the write, so no two datagrams from
different SAs interleave on the socket. The NAT keepalive uses `SendFrom` with
the SA's local source. `Conn()` exists for the socket option `EnableESPInUDP`
sets; nothing writes through it.

| Write | DF policy | Use |
|-------|-----------|-----|
| `Send` | the kernel default (`IP_PMTUDISC_WANT`: DF set on a datagram that fits the cached path MTU, fragmented otherwise) | callers without an explicit source |
| `SendFrom` | the kernel default | ordinary IKE messages and NAT keepalives, with an explicit local source |
| `SendDF` | one `probe.DFMode` for that datagram: `DFHonorCache` is `IP_PMTUDISC_DO`, `DFBypassCache` is `IP_PMTUDISC_PROBE`, `DFOff` is `IP_PMTUDISC_DONT` | the padded path probe (`docs/architecture/diagnostics/path-mtu.md`): the first copy with DF, the retransmission with DF clear |

`SendDF` sets `IP_MTU_DISCOVER` on the socket through `probe.WithDFMode`
(`docs/architecture/diagnostics/active-probes.md`), writes, and restores the
value the socket held before, on every exit path including a refused write.
The option is socket-wide, which is why `Send` and `SendFrom` take the same lock:
a datagram from another SA that left while the option was toggled
would carry the probe's DF setting. `TestDFSendRestoresSocketMode` reads the
option back after each mode, and `TestPlainSendNeverLeavesUnderTheProbeOption`
(`udp_df_integration_linux_test.go`, root, three namespaces) races plain
sends against DF-off sends and reads the DF bit of every datagram off an
AF_PACKET capture on the router. Off Linux `SendDF` answers
`probe.ErrDFUnsupported` and writes nothing (`udp_other.go`).

Both sockets carry `IP_RECVERR` from creation (`probe.EnableErrorQueue`), so
a router's Fragmentation Needed for a DF datagram, and the kernel's own
`EMSGSIZE` for a send larger than its cached path MTU, are queued on the
socket's error queue rather than dropped. The read loop, on a read error that
is not the close, drains the queue with `probe.DrainErrorQueue` and delivers
each `EMSGSIZE` entry on `Refusals()` as a `SizeRefusal`: the peer the datagram
was sent to (`Peer`, address and port, the entry's own destination), the
reported next-hop MTU under `Outcome`, the router as `Offender`, `Local` for
a cache refusal, and which socket it arrived on (`NATT`). The engine matches
`Peer` against the SA that holds a probe; the socket is shared by every SA,
and the destination is the one thing the kernel records per refused datagram.
Every established SA's owner loop reads the one channel, so an entry reaches
one of them: an entry another SA's loop read is dropped there, and the probe
it was about is repeated with DF clear at the retransmit timer instead
(`handleSizeRefusal`, `engine/probe.go`).
A `Local` entry carries the address and port 0, because the kernel fills it
from the socket's connected port, so the engine learns a local refusal from
`SendDF`'s own `EMSGSIZE` and reads the event as confirmation. An entry that is
not a size refusal (a port unreachable, a host unreachable) is logged at Debug
and dropped. The channel holds `refusalQueueDepth` (16) entries; an entry that
finds it full is dropped with a log line, because the read loop must return to
the socket and the engine's retransmit timer carries the DF-clear copy in any
case. `TestOversizedDFSendQueuesRefusalForThePeer` proves the router's
refusal and the local one against a clamped Linux router.

`IP_RECVERR` has one side effect the transport absorbs. With it set, the
kernel hands an ICMP error about an EARLIER datagram to the next send on the
socket as that send's failure (`net/core/sock.c sock_alloc_send_pskb` returns
the pending `sk_err` before it allocates), and that datagram is never sent.
Without the option an unconnected UDP socket drops the error, which is why
`Send` never failed this way before. So every failed write first drains the
queue: entries found mean the error was a report about earlier traffic, now
delivered on `Refusals`, and the write is made once more; none found means the
error is about this write; a `LOCAL` entry found means this write was refused
by the cache, and a second attempt would draw the same answer.
`TestSendSurvivesAnErrorQueuedForAnEarlierDatagram` drives it on loopback.

<!-- source: internal/component/ike/transport/udp.go -- UDPTransport, Send, SendFrom, SendDF, Run, SizeRefusal, Refusals -->
<!-- source: internal/component/ike/transport/udp_linux.go -- installErrorQueue, writeWithDF, drainErrorQueue -->
<!-- source: internal/component/ike/transport/udp_other.go -- the non-Linux stubs -->
