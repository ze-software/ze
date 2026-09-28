# RFC 7296 - Internet Key Exchange Protocol Version 2 (IKEv2)

Partial. Every requirement this repository extracted from RFC 7296, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 223 of 223 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 223 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 223 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 223 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 10.1% | 63 of 624 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 223 | of 229 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 223 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 223 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 223 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 223 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Audit verdicts | 87 | of 223 gated MUSTs judged | 37 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 223 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 229 |
| Gated MUST-level | 223 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 627 |
| Tagged units | 624 |
| Recorded audit verdicts | 87 |
| Discrimination records | 63 |
| Summary | `rfc/short/rfc7296.md` |
| Requirement shard | `rfc/requirements/rfc7296.md` |
| RFC text | `rfc/full/rfc7296.txt` |

## Enrolment

Enrolled: Internet Key Exchange Protocol Version 2 / IKEv2 (RFC 7296): 227 rows, 222 of them gated (179 MUST + 43 MUST NOT). All 222 are MET and proven in both polarities by 574 `RFC requirement:` tagged tests across 75 files. Zero annotations: no gap, no not-applicable, no single-polarity, no partial. The other 6 rows are SHOULD- or MAY-level, ungated and untagged. The 2026-08 pilot grew the summary from 23 rows and implemented what was absent: COOKIE (engine/cookie.go), the authenticated and out-of-SA error notifications (engine/notify_error.go), traffic-selector narrowing (engine/ts_narrow.go), per-SPI Child SA Delete (engine/delete.go), Message ID exhaustion (engine/msgid.go), and dual ESP form reception (dataplane/espform_linux.go). Two obligation sets are extracted and stay gated in their destination specs instead of here: the Configuration payload / IRAC role (11 rows, plan/immediate/spec-ipsec-remote-access.md; ze takes no IRAC role, which Section 4 permits) and IPComp (4 rows, plan/spec-ipsec-ipcomp.md). The Section 3.2 extraction walk added the two Critical-bit SENDER obligations the summary never carried, RFC7296-3.2-5 (zero asks the recipient to skip an unrecognized payload) and RFC7296-3.2-6 (one asks it to reject the whole message). The same walk added two more the summary never carried, RFC7296-1.3.3-2 (a CREATE_CHILD_SA that replaces an ESP or AH SA carries the REKEY_SA notification) and RFC7296-4-5 (the four-message IKE_SA_INIT and IKE_AUTH exchange establishes two SAs). rfc/extraction/rfc7296.json is signed off (2026-08-02): 261 sites in 104 sections, all classified, so the bound is a recorded walk of the source rather than the 222 extracted rows alone. 12 sites are relocated-to-spec, carrying obligations owner ruling D-1 moved to plan/immediate/spec-ipsec-remote-access.md and plan/spec-ipsec-ipcomp.md, and 26 requirements are unsourced-ids read from indicative prose that carries no keyword

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- Wire codec, cryptographic primitives, initiator and responder FSM, IKE_SA_INIT, IKE_AUTH, CREATE_CHILD_SA, INFORMATIONAL, rekey, DPD. Section 1.4 authenticated Delete on operator `clear` (graceful bounce)
- Section 2.4 state synchronization: the responder accepts a fresh IKE_SA_INIT in parallel with an established SA and supersedes it only once the new SA authenticates (never on the unauthenticated init)
- INITIAL_CONTACT emitted on the first IKE_AUTH request and honored on receipt. Section 1.4 liveness probes carry the Encrypted payload and authenticate under the negotiated keys. Section 2.4 refuses a verdict about the peer from an unauthenticated message, because an established SA is served by one handler that decrypts first. Section 2.5 accepts an IKE_AUTH whose payloads arrive in any order, on both the initiator and the responder path. Section 3.6 reads the first CERT payload as the peer certificate. It reads every later one as a chain intermediate. Ze sends its configured intermediate after the device certificate. Section 2.2 Message ID exhaustion: both counters stop at the 32-bit ceiling instead of wrapping. The SA rekeys itself in the headroom below the ceiling. The owner loop closes it at the ceiling (engine/msgid.go, engine/established.go). Section 2.3 window: Ze declares a window of one and accepts exactly one request id. It reads a peer SET_WINDOW_SIZE during IKE_AUTH, refuses a body that is not 4 octets, and reports the value as `peer-window-size` in `show vpn ipsec sa`. A peer request that crosses one of ours is accepted and answered, and a request outside the window is never acknowledged. Section 2.3 INVALID_MESSAGE_ID: an out-of-window request that authenticates draws the notification. Ze sends it as a NEW INFORMATIONAL request. That request carries the four-octet invalid Message ID. Ze never sends it as a response. Three bounds apply. Ze decrypts the request first, so an unauthenticated datagram draws nothing at all. Ze sends the notification only when its one request window is free. The notification therefore never displaces a liveness probe, a Delete or a rekey. A token bucket on the SA rate limits it. That bucket satisfies the MUST of Section 2.3 (engine/notify_invalid_msgid.go). Section 2.25: a rekey the peer answers with TEMPORARY_FAILURE waits 60 seconds before it is retried, per rekey kind. Sections 2.21.2, 2.21.3 and 2.21.4 error notifications: a request that fails on an authenticated IKE SA draws an encrypted error notify instead of silence. A refused Child SA rekey draws NO_PROPOSAL_CHOSEN, a malformed request draws INVALID_SYNTAX, and an unrecognized critical payload draws UNSUPPORTED_CRITICAL_PAYLOAD with the one-octet payload type. A datagram that matches no IKE SA draws an unprotected INVALID_IKE_SPI that copies the SPIs, Exchange Type and Message ID. That answer is rate limited. It is never sent in reply to a message marked as a response. It is never sent for an IKE_SA_INIT. An unprotected datagram at a CACHED message id draws no cached response and changes no SA state. This holds on the established path and on the mid-EAP responder path alike (engine/notify_error.go, engine/responder.go replayCachedResponse). The claim is bounded to that message id on purpose. An undecryptable datagram at a message id the responder is still expecting is a different path, and it does change SA state (engine/responder_eap.go handleResponderEAP). Section 3.10.1: an unrecognized error notify in a response fails the request it answers, and an unrecognized status notify is ignored and logged. Section 3.10.1 sender side: Ze adds one status notify of its own, the private-use ZE_PATH_PROBE_PADDING (65280), to the INFORMATIONAL request of the padded path probe that `show mtu` runs over a live SA
- its Notification Data pads the datagram to a requested size, and a peer that does not recognize it ignores it and answers an empty response (engine/probe.go, wire/payload_notify.go). Section 3.14 is why SK padding is not that carrier: the Pad Length is one octet, so SK padding cannot pass 255 octets, and an AEAD suite pads nothing (RFC 5282)
- under CBC the padding rule fixes the encrypted span to the block size, so the probe reaches sizes on a 16-octet grid and refuses the rest by name. Section 2.1: the probe is an ordinary request
- its retransmission is the same bytes from the IKE header on with only the IP header's DF bit changed, and a probe unanswered through the full budget deems the SA failed exactly as an unanswered Delete does (engine/established.go serviceRequestWindow, unchanged). Sections 1.2 and 2.6 corrected retry: an initiator that guesses the wrong Diffie-Hellman group retries the IKE_SA_INIT under the group the responder names, and refuses a group it never proposed, so a forged unauthenticated notify cannot steer it. The retry re-offers the whole configured suite set and re-anchors the signed IKE_SA_INIT octets. Section 2.6 COOKIE: the responder answers an inbound IKE_SA_INIT with a COOKIE challenge before it commits a half-open slot, and the initiator echoes the cookie as the first payload of a retry that changes nothing else. The cookie is an HMAC over the nonce, the source address and the initiator SPI under a rotating secret, is bounded to 1..64 octets on both the mint and the echo path, and a cookie that does not match is ignored rather than rejected. `cookie-threshold` sets how many half-open IKE SAs are tolerated before challenging, and defaults to 0 (challenge every initiation). Section 2.6.1: a second challenge replaces the first without failing, and the retry budget is bounded (engine/cookie.go, engine/sa_init_retry.go). Section 3.15.1 Configuration payload codec: the attribute type is 15 bits. Ze masks the Reserved bit on the read path and on the write path. A peer that sets that bit still has its INTERNAL_IP4_ADDRESS recognized (wire/payload_cp.go). Sections 2.19, 2.20 and 3.15.1: Ze builds no Configuration payload. It takes no IRAC role, which Section 4 permits. It ignores CFG_SET, which Section 3.15.1 permits. It sends no CFG_REQUEST, no CFG_REPLY and no CFG_ACK, and it gives out no version string. Section 3.4 key exchange pairing: the initiator refuses an IKE_SA_INIT or IKE-rekey response whose KE payload names a Diffie-Hellman group that no proposal of the same message specifies, and refuses a KE payload at all when no proposal specifies a group. A Transform ID of NONE names no group for both rules (wire/payload_sa.go, engine/fsm.go). Section 3.3.6: an IKE SA negotiation never selects the group NONE, and the one exchange that does select it, a Child SA rekey without PFS, ignores any KE payload the peer sends and omits one from the response (engine/rekey.go). Section 2.4: INITIAL_CONTACT is withheld by an identity that may be replicated. An EAP credential names a user rather than a device, so an EAP peer sends none
- a pre-shared secret or X.509 device identity still sends it (engine/auth.go). Section 2.8: an IKE SA whose negotiated lifetime has expired is refused for use at the point every protected message is built, not merely scheduled for teardown, and hard expiry is no longer deferred by an in-flight rekey. The rekey trigger is placed a full retransmit budget before the hard time so the replacement exchange still has room (engine/sa.go, engine/auth.go, engine/established.go, engine/rekey.go). Section 4: a rekey the peer refuses with NO_ADDITIONAL_SAS is answered by deleting the old SA and creating a new one through the initial exchanges, instead of retrying an exchange that peer will never accept (engine/rekey.go, engine/inbound.go). Section 3.6 certificate count: `certificate-count` bounds the chain Ze sends and the chain it accepts, and it defaults to the four the section names. A peer that sends more is refused, never truncated (engine/cert_payload.go). Section 3.6 Hash and URL: `hash-and-url` turns on both encodings. Ze sends encoding 12 for one certificate and encoding 13 for a bundle, and it advertises HTTP_CERT_LOOKUP_SUPPORTED. It resolves a payload a peer sends through a bounded http fetch. That fetch permits the http scheme only. It caps the body at 64 KiB and times out after 5 seconds. It follows no redirect. It denies loopback, private, link-local and metadata addresses. It verifies the SHA-1 before any parser reads the bytes. The leaf defaults to false, so no fetch is reachable until an operator asks for one (engine/certurl.go, engine/certbundle.go). Section 2.9 traffic-selector narrowing: the responder narrows the initiator's proposed TSi/TSr to a subset its configured `traffic-selector` list allows, leads that subset with the initiator's first choices, and answers TS_UNACCEPTABLE when nothing is acceptable. A peer with no configured selectors accepts whatever it is offered, which is the behavior of every configuration written before the list existed. The selectors Ze puts on the wire are the selectors it programs: a proposal it cannot program exactly is narrowed further, never rounded outward (engine/ts_narrow.go, ipsec/traffic_selector.go). Section 2.9.2: a rekey is never narrowed below the scope currently in use (engine/rekey.go). Sections 1.3.1 and 2.23.1 transport mode: `mode transport` sends USE_TRANSPORT_MODE with the Child SA request and pins TSi and TSr to the IKE SA's own OBSERVED address pair, one address each. A responder accepts the request only when its own configuration asks for transport mode, and echoes the notification when it does. A peer that declines leaves a tunnel-mode Child SA, and `transport-required` makes that decline delete the SA instead (engine/transport_mode.go, engine/child.go). Section 2.23.1 address substitution: with a NAT on the path, both roles replace the transport-mode selector addresses before those selectors are used for anything except storing the originals. The responder substitutes the TSi address with the observed remote address when the peer is behind a NAT, and the TSr address with its own local address when it is, before the policy lookup that is Ze's SPD lookup. The initiator substitutes the mirror pair before the answer is checked against what it proposed and before the SAD entry is created. The substituted address is always one this node observed, never one the peer asserted, so the narrowing guard still refuses an answer outside the proposal. The SA records which SIDE each translation is on, separately from whether a NAT is present, and stores the pre-substitution addresses the section requires be kept. Tunnel mode and a NAT-free path are unchanged. Proven against strongSwan across a netfilter box that really rewrites both addresses, in test/interop-ipsec/scenarios/real-nat-transport-ze-initiator, real-nat-transport-ze-responder and real-nat-tunnel-control (engine/ts_nat_substitute.go, engine/ts_narrow.go, engine/sa.go). Section 3.13.1 port selectors: a selector carries all ports (0/65535) or one port under a protocol that defines ports (ipsec/traffic_selector.go). Section 4 conformance set: Ze accepts PKIX certificates signed by RSA keys of 1024 and 2048 bits. The identity passed can be ID_FQDN, ID_RFC822_ADDR or ID_DER_ASN1_DN, which binds against the certificate subject exactly. `remote-id-type key-id` lets an operator accept ID_KEY_ID, which corresponds to no certificate field. The same leaf pins one identity type per peer (engine/remote_id.go). Section 3.3 transform alternatives: one proposal CAN carry several transforms of the same Transform Type. Ze reads every one of them as an alternative, and it does not keep only the last. A peer CAN offer two Diffie-Hellman groups, two key lengths, two PRFs or two integrity algorithms in one proposal. Ze considers all of them, in the order the peer listed them. Ze selects the peer's first choice that it supports. The number of combinations one proposal expands to is bounded, so an unauthenticated peer cannot turn a cross product into unbounded work (engine/initiator.go). Section 1.4.1 Delete: an inbound Child SA Delete is resolved to the pair the peer named by SPI and that pair is closed, and the response carries a Delete payload naming the paired SA going in the other direction. A Delete that crosses one Ze already sent for the same pair is answered without a Delete payload, and the two halves of the pair go at the two points the section names. An IKE SA Delete still draws an empty response (engine/delete.go). Section 1.5: the out-of-SA notification emitter is a fixed point, so its own output fed back to it produces nothing and two nodes cannot trade messages forever (engine/notify_error.go). Section 2.12: closing an IKE SA erases SK_d and the rest of SK_*, the Diffie-Hellman private value and the EAP MSK, and releases the nonces, on every path that ends an SA including an abandoned half-open handshake. The two holders that live on the peer SESSION rather than on the SA are released on the same exit: the Diffie-Hellman private value of a rekey that never got its answer, and a whole IKE SA built while answering the peer's rekey and never confirmed by the peer's Delete. The session outlives the SA, so either one left behind would carry key material into the next reconnect cycle (engine/sa.go, engine/established.go, engine/fsm.go). Section 2.16: the EAP shared key generates the AUTH payloads that follow EAP Success, and the responder sends Success on a completed method and Failure on a refused one (eap/eap.go, engine/eap_auth.go). Section 2.24 ECN: no config leaf and no negotiated parameter can reach an ECN knob, and the netlink state Ze fills carries no flags field, so XFRM_STATE_NOECN is unreachable and the kernel's ECN propagation stands (dataplane/xfrm_linux.go). Section 3.1: a liveness probe sets the I bit from the ORIGINAL initiator role rather than hardcoding it (engine/dpd.go). Section 3.5: an ID_FQDN or ID_RFC822_ADDR carrying a terminator octet is refused, on the receive path against a peer's asserted identity and at commit against local-id and remote-id (engine/remote_id.go, ipsec/validate.go). Section 3.3 ESP alternatives: every transform of a type in one ESP proposal is read as an alternative, and a key length is only ever compared beside the id of the transform that carried it (engine/responder.go). Section 2.7 Extended Sequence Numbers: Ze keys a 32-bit ESP sequence space, so it offers and answers Transform Type 5 value 0 and selects that value from a proposal that offers both. A proposal offering value 1 alone is refused with NO_PROPOSAL_CHOSEN rather than answered with a transform the peer never proposed, and an answer selecting value 1 ends the exchange on the initiator side (engine/responder.go, engine/initiator.go). Proven against strongSwan in test/interop-ipsec/scenarios/esn-extended-only-refused and esn-both-offered. Section 2.23 NAT-T: one Child SA receives BOTH ESP forms, and the form Ze SENDS follows the NAT verdict alone. Reception and transmission are two decisions. The inbound XFRM state carries the encapsulation template when a NAT was detected or the IKE SA runs on port 4500, so the kernel serves that form on its fast path. The other form is served beside the kernel: a raw IPPROTO_ESP reader takes the bare datagram XFRM refused and re-presents it through port 4500, which carries UDP_ENCAP, so the kernel hands XFRM the encapsulation type the template wants. Ze encapsulates on transmission only when it detected a NAT, which RFC 7296 Section 2.23 makes mandatory there and leaves free otherwise. Reading the port as a transmission signal broke interop against a strongSwan that floats to port 4500 for MOBIKE with no NAT present and then sends bare ESP (engine/child.go, dataplane/espform.go, dataplane/espform_linux.go, transport/encap_linux.go). Section 2.2 two counters: each end keeps its own next Message ID. A responder-role SA raises its first request at the id a conforming peer expects. It took the peer's id before this fix, so it raised no DPD, no Delete and no rekey (engine/responder.go, engine/msgid.go). Section 2.9 narrowing orientation: the responder narrows against the orientation of the exchange in hand. The IKE SA role is a different question. A Child SA rekey the peer starts on a tunnel Ze initiated is answered rather than refused with TS_UNACCEPTABLE (engine/ts_narrow.go). Section 2.8 rekey on the XFRM dataplane: the replacement Child SA inherits the retired pair's selectors and their orientation. The make-before-break window holds one policy, and retiring the superseded pair removes no selector the live pair still needs. [`test/ipsec/ipsec-child-rekey-xfrm.ci`](https://github.com/ze-software/ze/blob/main/test/ipsec/ipsec-child-rekey-xfrm.ci) measures it on the real backend with the two roles crossed (engine/rekey.go, engine/child.go).


**What the ledger says remains**

No MUST gap remains gated in [`rfc/short/rfc7296.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc7296.md). The Delete-payload echo that [`RFC7296-1.4-1`](#rfc7296-1.4-1) recorded is implemented: an inbound Child SA Delete now closes the pair the peer designated by SPI and the response names the paired SA going the other way (engine/delete.go). Two platform limits are gated and disclosed, and neither is left as a gap. Section 3.13.1 OPAQUE ports: Ze implements the 65535/0 encoding and proves it. Ze refuses OPAQUE at commit and at negotiation. The kernel policy selector derives its port mask from the port value, so an exact match on port 0 would install as any-port. That is wider than the selector negotiated. Section 2.23 ESP forms: the platform limit is LIFTED. One established Child SA now receives both UDP-encapsulated and bare ESP, so a peer that changes form on that SA is served. TestEncapOneStateAcceptsBothForms measures it against a real kernel: ONE state on ONE SPI, the encapsulated form on the kernel fast path and the bare form read off a raw IPPROTO_ESP socket and re-presented, both reaching the crypto check (dataplane/encap_hybrid_integration_linux_test.go). That test re-presents the datagram by hand, so it measures the kernel mechanism and not the shipped path. TestEncapEstablishedSAServesAPeerFormChange measures the shipped path: the SA is installed through the production backend, a live SA carries one form, the peer switches to the other, then back, and the kernel's state table shows ONE state with an unchanged add time throughout (dataplane/encap_formchange_integration_linux_test.go). The claim is also proven against strongSwan, over a real interface, in test/interop-ipsec/scenarios/esp-form-change: a live Child SA whose peer sends the form Ze's kernel state refuses carries traffic in both directions and is neither rekeyed nor deleted. That scenario is what caught the one defect that made this claim false in a real deployment. Linux runs xfrm4_policy_check before it queues a packet to a raw socket, and Ze's own inbound Child SA policy rejected the datagrams the receiver exists to recover, so the kernel dropped every one of them. The reader now carries a per-socket inbound bypass, scoped to that socket alone (dataplane/espform_linux.go). No loopback test can see that defect, because a loopback dst entry carries DST_NOPOLICY and the check is skipped. One Linux XFRM state still binds one form, which TestEncapKernelBindsOneESPFormPerState records, so the second form is served beside the kernel rather than through it (dataplane/encap_integration_linux_test.go). Two states on one SPI do NOT help, and that is now MEASURED rather than reasoned. The kernel refuses to install the second state with `file exists`, both with identical addresses and with a differing source, because the uniqueness key and the lookup key are the same tuple. The earlier text said the lookup returns the first match and the encapsulation check then drops the packet.

- **That mechanism is wrong:** the second state never exists (dataplane/encap_dualform_integration_linux_test.go). One combination stays unreachable and is bounded by the RFC rather than by Ze: a template-free inbound state cannot receive the encapsulated form, because the port-4500 socket carries UDP_ENCAP and the kernel consumes that datagram before any userspace reader sees it (TestEncapEncapsulatedESPHiddenFromUserspaceWhenSocketDecapsulates). No conforming peer produces it. RFC 3948 Section 2.1 requires the ESP-in-UDP ports to equal the IKE ports, and RFC 7296 Section 2.23 forbids encapsulation on port 500, so a peer that encapsulates ESP runs its IKE on port 4500 and Ze templates that SA.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 223 | one part of the gated population |
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **223** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (223):** [`RFC7296-1.2-1`](#rfc7296-1.2-1), [`RFC7296-2.6-1`](#rfc7296-2.6-1), [`RFC7296-2.7-1`](#rfc7296-2.7-1), [`RFC7296-3.3-1`](#rfc7296-3.3-1), [`RFC7296-3.3-2`](#rfc7296-3.3-2), [`RFC7296-3.3.2-1`](#rfc7296-3.3.2-1), [`RFC7296-3.3.6-1`](#rfc7296-3.3.6-1), [`RFC7296-1.3.3-1`](#rfc7296-1.3.3-1), [`RFC7296-1.3.3-2`](#rfc7296-1.3.3-2), [`RFC7296-2.9-1`](#rfc7296-2.9-1), [`RFC7296-2.9-2`](#rfc7296-2.9-2), [`RFC7296-2.9.2-1`](#rfc7296-2.9.2-1), [`RFC7296-2.9.2-2`](#rfc7296-2.9.2-2), [`RFC7296-1.3.1-1`](#rfc7296-1.3.1-1), [`RFC7296-1.3.1-2`](#rfc7296-1.3.1-2), [`RFC7296-2.23.1-1`](#rfc7296-2.23.1-1), [`RFC7296-2.23.1-2`](#rfc7296-2.23.1-2), [`RFC7296-2.23.1-3`](#rfc7296-2.23.1-3), [`RFC7296-3.13.1-1`](#rfc7296-3.13.1-1), [`RFC7296-3.13.1-2`](#rfc7296-3.13.1-2), [`RFC7296-3.13.1-3`](#rfc7296-3.13.1-3), [`RFC7296-2.23-1`](#rfc7296-2.23-1), [`RFC7296-2.23-2`](#rfc7296-2.23-2), [`RFC7296-2.23-3`](#rfc7296-2.23-3), [`RFC7296-2.4-1`](#rfc7296-2.4-1), [`RFC7296-1.4-1`](#rfc7296-1.4-1), [`RFC7296-2.8-2`](#rfc7296-2.8-2), [`RFC7296-1-1`](#rfc7296-1-1), [`RFC7296-1.2-2`](#rfc7296-1.2-2), [`RFC7296-1.2-3`](#rfc7296-1.2-3), [`RFC7296-1.2-4`](#rfc7296-1.2-4), [`RFC7296-1.2-5`](#rfc7296-1.2-5), [`RFC7296-1.2-6`](#rfc7296-1.2-6), [`RFC7296-2.4-11`](#rfc7296-2.4-11), [`RFC7296-2.4-12`](#rfc7296-2.4-12), [`RFC7296-2.4-13`](#rfc7296-2.4-13), [`RFC7296-2.7-2`](#rfc7296-2.7-2), [`RFC7296-2.10-4`](#rfc7296-2.10-4), [`RFC7296-2.11-1`](#rfc7296-2.11-1), [`RFC7296-2.11-2`](#rfc7296-2.11-2), [`RFC7296-2.11-3`](#rfc7296-2.11-3), [`RFC7296-2.16-11`](#rfc7296-2.16-11), [`RFC7296-2.16-6`](#rfc7296-2.16-6), [`RFC7296-2.16-7`](#rfc7296-2.16-7), [`RFC7296-2.23-7`](#rfc7296-2.23-7), [`RFC7296-2.23-8`](#rfc7296-2.23-8), [`RFC7296-2.23-9`](#rfc7296-2.23-9), [`RFC7296-2.23-10`](#rfc7296-2.23-10), [`RFC7296-2.23-11`](#rfc7296-2.23-11), [`RFC7296-2.23-12`](#rfc7296-2.23-12), [`RFC7296-3.9-2`](#rfc7296-3.9-2), [`RFC7296-1.3-2`](#rfc7296-1.3-2), [`RFC7296-2.8.2-1`](#rfc7296-2.8.2-1), [`RFC7296-3.3.6-3`](#rfc7296-3.3.6-3), [`RFC7296-1.4-3`](#rfc7296-1.4-3), [`RFC7296-1.4-4`](#rfc7296-1.4-4), [`RFC7296-1.4-5`](#rfc7296-1.4-5), [`RFC7296-1.4.1-6`](#rfc7296-1.4.1-6), [`RFC7296-1.4.1-7`](#rfc7296-1.4.1-7), [`RFC7296-1.5-1`](#rfc7296-1.5-1), [`RFC7296-2.12-1`](#rfc7296-2.12-1), [`RFC7296-2.24-1`](#rfc7296-2.24-1), [`RFC7296-2.24-2`](#rfc7296-2.24-2), [`RFC7296-2.16-12`](#rfc7296-2.16-12), [`RFC7296-2.16-13`](#rfc7296-2.16-13), [`RFC7296-2.16-14`](#rfc7296-2.16-14), [`RFC7296-2.16-15`](#rfc7296-2.16-15), [`RFC7296-3.1-13`](#rfc7296-3.1-13), [`RFC7296-3.5-5`](#rfc7296-3.5-5), [`RFC7296-1.4.1-1`](#rfc7296-1.4.1-1), [`RFC7296-1.4.1-4`](#rfc7296-1.4.1-4), [`RFC7296-1.4.1-5`](#rfc7296-1.4.1-5), [`RFC7296-2.4-9`](#rfc7296-2.4-9), [`RFC7296-2.4-10`](#rfc7296-2.4-10), [`RFC7296-2.16-5`](#rfc7296-2.16-5), [`RFC7296-3.4-1`](#rfc7296-3.4-1), [`RFC7296-1.3-1`](#rfc7296-1.3-1), [`RFC7296-2.1-3`](#rfc7296-2.1-3), [`RFC7296-2.1-4`](#rfc7296-2.1-4), [`RFC7296-2.1-5`](#rfc7296-2.1-5), [`RFC7296-2.1-6`](#rfc7296-2.1-6), [`RFC7296-2.1-7`](#rfc7296-2.1-7), [`RFC7296-2.1-8`](#rfc7296-2.1-8), [`RFC7296-2.2-1`](#rfc7296-2.2-1), [`RFC7296-2.2-2`](#rfc7296-2.2-2), [`RFC7296-2.2-3`](#rfc7296-2.2-3), [`RFC7296-2.3-2`](#rfc7296-2.3-2), [`RFC7296-2.3-4`](#rfc7296-2.3-4), [`RFC7296-2.3-5`](#rfc7296-2.3-5), [`RFC7296-2.3-7`](#rfc7296-2.3-7), [`RFC7296-2.3-8`](#rfc7296-2.3-8), [`RFC7296-2.3-9`](#rfc7296-2.3-9), [`RFC7296-2.25-1`](#rfc7296-2.25-1), [`RFC7296-2.8-5`](#rfc7296-2.8-5), [`RFC7296-2.8-6`](#rfc7296-2.8-6), [`RFC7296-2.8-7`](#rfc7296-2.8-7), [`RFC7296-2.8.1-1`](#rfc7296-2.8.1-1), [`RFC7296-2.18-2`](#rfc7296-2.18-2), [`RFC7296-2.18-3`](#rfc7296-2.18-3), [`RFC7296-3.16-1`](#rfc7296-3.16-1), [`RFC7296-3.16-2`](#rfc7296-3.16-2), [`RFC7296-3.16-3`](#rfc7296-3.16-3), [`RFC7296-3.16-4`](#rfc7296-3.16-4), [`RFC7296-1.7-2`](#rfc7296-1.7-2), [`RFC7296-2-1`](#rfc7296-2-1), [`RFC7296-2.5-1`](#rfc7296-2.5-1), [`RFC7296-2.5-2`](#rfc7296-2.5-2), [`RFC7296-2.5-6`](#rfc7296-2.5-6), [`RFC7296-2.5-7`](#rfc7296-2.5-7), [`RFC7296-2.5-8`](#rfc7296-2.5-8), [`RFC7296-2.5-9`](#rfc7296-2.5-9), [`RFC7296-2.5-11`](#rfc7296-2.5-11), [`RFC7296-2.5-13`](#rfc7296-2.5-13), [`RFC7296-2.5-14`](#rfc7296-2.5-14), [`RFC7296-2.5-15`](#rfc7296-2.5-15), [`RFC7296-2.5-16`](#rfc7296-2.5-16), [`RFC7296-2.5-17`](#rfc7296-2.5-17), [`RFC7296-2.5-18`](#rfc7296-2.5-18), [`RFC7296-2.21.2-1`](#rfc7296-2.21.2-1), [`RFC7296-2.21.2-2`](#rfc7296-2.21.2-2), [`RFC7296-2.21.2-3`](#rfc7296-2.21.2-3), [`RFC7296-2.21.3-1`](#rfc7296-2.21.3-1), [`RFC7296-2.21.4-1`](#rfc7296-2.21.4-1), [`RFC7296-2.21.4-2`](#rfc7296-2.21.4-2), [`RFC7296-2.21.4-3`](#rfc7296-2.21.4-3), [`RFC7296-2.21.4-4`](#rfc7296-2.21.4-4), [`RFC7296-2.21.4-5`](#rfc7296-2.21.4-5), [`RFC7296-2.21.4-6`](#rfc7296-2.21.4-6), [`RFC7296-2.21.4-7`](#rfc7296-2.21.4-7), [`RFC7296-3.10.1-1`](#rfc7296-3.10.1-1), [`RFC7296-3.10.1-2`](#rfc7296-3.10.1-2), [`RFC7296-3.10.1-3`](#rfc7296-3.10.1-3), [`RFC7296-2.6-2`](#rfc7296-2.6-2), [`RFC7296-2.6-3`](#rfc7296-2.6-3), [`RFC7296-2.6-4`](#rfc7296-2.6-4), [`RFC7296-2.6-5`](#rfc7296-2.6-5), [`RFC7296-2.6.1-1`](#rfc7296-2.6.1-1), [`RFC7296-2.10-2`](#rfc7296-2.10-2), [`RFC7296-2.10-3`](#rfc7296-2.10-3), [`RFC7296-2.13-1`](#rfc7296-2.13-1), [`RFC7296-2.13-2`](#rfc7296-2.13-2), [`RFC7296-2.13-3`](#rfc7296-2.13-3), [`RFC7296-2.13-4`](#rfc7296-2.13-4), [`RFC7296-2.15-1`](#rfc7296-2.15-1), [`RFC7296-2.15-2`](#rfc7296-2.15-2), [`RFC7296-2.17-1`](#rfc7296-2.17-1), [`RFC7296-2.17-2`](#rfc7296-2.17-2), [`RFC7296-3.1-1`](#rfc7296-3.1-1), [`RFC7296-3.1-2`](#rfc7296-3.1-2), [`RFC7296-3.1-3`](#rfc7296-3.1-3), [`RFC7296-3.1-4`](#rfc7296-3.1-4), [`RFC7296-3.1-5`](#rfc7296-3.1-5), [`RFC7296-3.1-6`](#rfc7296-3.1-6), [`RFC7296-3.1-7`](#rfc7296-3.1-7), [`RFC7296-3.1-8`](#rfc7296-3.1-8), [`RFC7296-3.1-9`](#rfc7296-3.1-9), [`RFC7296-3.1-11`](#rfc7296-3.1-11), [`RFC7296-3.1-12`](#rfc7296-3.1-12), [`RFC7296-3.2-2`](#rfc7296-3.2-2), [`RFC7296-3.2-3`](#rfc7296-3.2-3), [`RFC7296-3.2-4`](#rfc7296-3.2-4), [`RFC7296-3.2-5`](#rfc7296-3.2-5), [`RFC7296-3.2-6`](#rfc7296-3.2-6), [`RFC7296-3.3-3`](#rfc7296-3.3-3), [`RFC7296-3.3-4`](#rfc7296-3.3-4), [`RFC7296-3.3-5`](#rfc7296-3.3-5), [`RFC7296-3.3-6`](#rfc7296-3.3-6), [`RFC7296-3.3-7`](#rfc7296-3.3-7), [`RFC7296-3.3.1-1`](#rfc7296-3.3.1-1), [`RFC7296-3.3.1-2`](#rfc7296-3.3.1-2), [`RFC7296-3.3.3-1`](#rfc7296-3.3.3-1), [`RFC7296-3.3.4-2`](#rfc7296-3.3.4-2), [`RFC7296-3.3.4-3`](#rfc7296-3.3.4-3), [`RFC7296-3.3.5-1`](#rfc7296-3.3.5-1), [`RFC7296-3.3.5-2`](#rfc7296-3.3.5-2), [`RFC7296-3.3.5-3`](#rfc7296-3.3.5-3), [`RFC7296-3.3.5-4`](#rfc7296-3.3.5-4), [`RFC7296-3.3.5-5`](#rfc7296-3.3.5-5), [`RFC7296-3.3.6-4`](#rfc7296-3.3.6-4), [`RFC7296-3.3.6-5`](#rfc7296-3.3.6-5), [`RFC7296-3.3.6-7`](#rfc7296-3.3.6-7), [`RFC7296-3.9-1`](#rfc7296-3.9-1), [`RFC7296-3.10-3`](#rfc7296-3.10-3), [`RFC7296-3.10-4`](#rfc7296-3.10-4), [`RFC7296-3.10-5`](#rfc7296-3.10-5), [`RFC7296-3.11-1`](#rfc7296-3.11-1), [`RFC7296-3.11-2`](#rfc7296-3.11-2), [`RFC7296-3.12-2`](#rfc7296-3.12-2), [`RFC7296-3.12-3`](#rfc7296-3.12-3), [`RFC7296-3.12-4`](#rfc7296-3.12-4), [`RFC7296-3.14-2`](#rfc7296-3.14-2), [`RFC7296-3.14-3`](#rfc7296-3.14-3), [`RFC7296-3.14-4`](#rfc7296-3.14-4), [`RFC7296-3.14-5`](#rfc7296-3.14-5), [`RFC7296-3.14-6`](#rfc7296-3.14-6), [`RFC7296-3.14-7`](#rfc7296-3.14-7), [`RFC7296-5-2`](#rfc7296-5-2), [`RFC7296-5-3`](#rfc7296-5-3), [`RFC7296-2.19-1`](#rfc7296-2.19-1), [`RFC7296-2.19-4`](#rfc7296-2.19-4), [`RFC7296-2.20-1`](#rfc7296-2.20-1), [`RFC7296-3.15.1-2`](#rfc7296-3.15.1-2), [`RFC7296-3.15.1-5`](#rfc7296-3.15.1-5), [`RFC7296-3.15.1-6`](#rfc7296-3.15.1-6), [`RFC7296-3.15.1-7`](#rfc7296-3.15.1-7), [`RFC7296-2.4-3`](#rfc7296-2.4-3), [`RFC7296-2.4-4`](#rfc7296-2.4-4), [`RFC7296-3.4-2`](#rfc7296-3.4-2), [`RFC7296-3.4-3`](#rfc7296-3.4-3), [`RFC7296-3.3.6-8`](#rfc7296-3.3.6-8), [`RFC7296-2.4-14`](#rfc7296-2.4-14), [`RFC7296-2.8-8`](#rfc7296-2.8-8), [`RFC7296-4-1`](#rfc7296-4-1), [`RFC7296-2.15-3`](#rfc7296-2.15-3), [`RFC7296-3.3.4-4`](#rfc7296-3.3.4-4), [`RFC7296-3.5-2`](#rfc7296-3.5-2), [`RFC7296-3.5-3`](#rfc7296-3.5-3), [`RFC7296-3.5-4`](#rfc7296-3.5-4), [`RFC7296-3.6-1`](#rfc7296-3.6-1), [`RFC7296-3.6-2`](#rfc7296-3.6-2), [`RFC7296-3.6-3`](#rfc7296-3.6-3), [`RFC7296-4-4`](#rfc7296-4-4), [`RFC7296-4-5`](#rfc7296-4-5)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7296-1.2-1` | The first pair of messages (IKE_SA_INIT) negotiate cryptographic algorithms, exchange nonces, and do a Diffie-Hellman exchange [DH]. The second pair of messages (IKE_AUTH) authenticate the previous messages, exchange identities and certificates, and establish the first Child SA. Parts of these messages are encrypted and integrity protected with keys established through the IKE_SA_INIT exchange, so the identities are hidden from eavesdroppers and all fields in all the messages are authenticated. (§1.2) | MUST | 1.2 - The initial exchanges | **positive:** `unit/verify` [`TestInitialExchangeEncryptionBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_test.go#L32). **negative:** `unit/verify` [`TestInitialExchangeEncryptionBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_test.go#L35) |
| `RFC7296-2.6-1` | The initial two eight-octet fields in the header, called the "IKE SPIs", are used as a connection identifier at the beginning of IKE packets. (§2.6) | MUST | 2.6 | **positive:** `unit/verify` [`TestHeaderRoundtrip`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/header_test.go#L9). **negative:** `unit/verify` [`TestDecodeTruncatedHeader`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/header_test.go#L75) |
| `RFC7296-2.7-1` | The responder MUST accept a single proposal or reject them all and return an error. The error is given in a notification of type NO_PROPOSAL_CHOSEN. Each IPsec protocol proposal contains one or more transforms. Each transform contains a Transform Type. The accepted cryptographic suite MUST contain exactly one transform of each type included in the proposal. (§2.7) | MUST | 2.7 | **positive:** `unit/verify` [`TestEsnResponderAnswersOnlyAValueTheOfferCarried`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_esn_test.go#L95). **positive:** `unit/verify` [`TestProposalNegotiationFirstMatch`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/proposal_test.go#L8). **negative:** `unit/verify` [`TestEsnResponderAnswersOnlyAValueTheOfferCarried`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_esn_test.go#L87). **negative:** `unit/verify` [`TestProposalNegotiationNoMatch`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/proposal_test.go#L63). **positive:** `interop/nightly` [`checkESNBothOffered`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/ipsec/checkers.go#L1050). **negative:** `interop/nightly` [`checkESNExtendedOnlyRefused`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/ipsec/checkers.go#L1019) |
| `RFC7296-3.3-1` | If an initiator wants to propose both combined- mode ciphers and normal ciphers, it must include two proposals: one will have all the combined-mode ciphers, and the other will have all the normal ciphers with the integrity algorithms. (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestAeadAloneInItsOwnProposalIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_aead_mix_test.go#L60). **positive:** `unit/verify` [`TestESPProposalsNeverMixAEADClass`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_test.go#L172). **negative:** `unit/verify` [`TestAeadMixInOneProposalIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_aead_mix_test.go#L32) |
| `RFC7296-3.3-2` | Combined-mode ciphers include both integrity and encryption in a single encryption algorithm, and MUST either offer no integrity algorithm or a single integrity algorithm of "NONE" (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestESPWireProposalAEADIntegNone`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_test.go#L148). **negative:** `unit/verify` [`TestESPWireProposalAEADIntegNone`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_test.go#L151) |
| `RFC7296-3.3.2-1` | An SA payload proposing the establishment of an SA has the following mandatory and optional Transform Types. A compliant implementation MUST understand all mandatory and optional types for each protocol it supports (though it need not accept proposals with unacceptable suites). A proposal MAY omit the optional types if the only value for them it will accept is NONE. Protocol Mandatory Types Optional Types --------------------------------------------------- IKE ENCR, PRF, INTEG*, D-H (§3.3.3) | MUST | 3.3.3 | **positive:** `unit/verify` [`TestIKEWireProposalHasAllTransforms`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_test.go#L195). **negative:** `unit/verify` [`TestIKEWireProposalHasAllTransforms`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_test.go#L198) |
| `RFC7296-3.3.6-1` | A compliant implementation MUST understand all mandatory and optional types for each protocol it supports (though it need not accept proposals with unacceptable suites). A proposal MAY omit the optional types if the only value for them it will accept is NONE. Protocol Mandatory Types Optional Types --------------------------------------------------- IKE ENCR, PRF, INTEG*, D-H (§3.3.3) | MUST | 3.3.3 | **positive:** `unit/verify` [`TestResponderRequiresKEForDH`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_test.go#L226). **negative:** `unit/verify` [`TestResponderRequiresKEForDH`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_test.go#L229) |
| `RFC7296-1.3.3-1` | The KEi payload MUST be included. (§1.3.2) | MUST | 1.3.2 - Rekeying the IKE SA | **positive:** `unit/verify` [`TestRespondIKERekey`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/responder_test.go#L265). **negative:** `unit/verify` [`TestRespondIKERekeyRejectsMissingKE`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_test.go#L123) |
| `RFC7296-1.3.3-2` | The REKEY_SA notification MUST be included in a CREATE_CHILD_SA exchange if the purpose of the exchange is to replace an existing ESP or AH SA (§1.3.3) | MUST | 1.3.3 - Rekeying a Child SA | **positive:** `unit/verify` [`TestRksaChildRekeyCarriesTheRekeySANotify`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_rekeysa_test.go#L40). **negative:** `unit/verify` [`TestRksaChildRekeyCarriesTheRekeySANotify`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_rekeysa_test.go#L46) |
| `RFC7296-2.9-1` | When the responder chooses a subset of the traffic proposed by the initiator, it narrows the Traffic Selectors to some subset of the initiator's proposal (provided the set does not become the null set). (§2.9) | MUST | 2.9 | **positive:** `unit/verify` [`TestChildRekeyAnswerWithoutTrafficSelectorsIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_rekey_initiator_answer_test.go#L273). **positive:** `unit/verify` [`TestChildRekeyInitiatorInstallsTheAnsweredSelectors`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_rekey_initiator_answer_test.go#L105). **positive:** `unit/verify` [`TestRekeyWithoutTrafficSelectorsIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rekey_test.go#L345). **positive:** `unit/verify` [`TestTSUnacceptableIsSentWhenNothingIsAcceptable`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_test.go#L60). **negative:** `unit/verify` [`TestChildRekeyAnswerWithoutTrafficSelectorsIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_rekey_initiator_answer_test.go#L267). **negative:** `unit/verify` [`TestChildRekeyInitiatorInstallsTheAnsweredSelectors`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_rekey_initiator_answer_test.go#L112). **negative:** `unit/verify` [`TestRekeyWithoutTrafficSelectorsIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rekey_test.go#L337). **negative:** `unit/verify` [`TestTSUnacceptableIsSentWhenNothingIsAcceptable`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_test.go#L76) |
| `RFC7296-2.9-2` | If the responder's policy allows it to accept the first selector of TSi and TSr, then the responder MUST narrow the Traffic Selectors to a subset that includes the initiator's first choices (§2.9) | MUST | 2.9 | **positive:** `unit/verify` [`TestAuthResponsePayloadsCarryTheNarrowedSelectors`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_test.go#L371). **positive:** `unit/verify` [`TestNarrowingIncludesFirstChoice`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ts_narrow_test.go#L99). **positive:** `unit/verify` [`TestPeerInitiatedRekeyIsNarrowedInTheExchangeOrientation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_rekey_orientation_test.go#L133). **negative:** `unit/verify` [`TestNarrowingIncludesFirstChoice`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ts_narrow_test.go#L143) |
| `RFC7296-2.9.2-1` | Thus, the new SA MUST NOT have narrower selectors than the original (§2.9.2) | MUST NOT | 2.9.2 | **positive:** `unit/verify` [`TestChildRekeyAnswerBelowTheScopeInUseIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_rekey_initiator_answer_test.go#L183). **positive:** `unit/verify` [`TestRekeyAnswerMatchesTheInstalledSelectors`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_rekey_orientation_test.go#L322). **positive:** `unit/verify` [`TestRekeyFloorIsNotNarrowed`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ts_narrow_test.go#L222). **negative:** `unit/verify` [`TestChildRekeyAnswerBelowTheScopeInUseIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_rekey_initiator_answer_test.go#L194). **negative:** `unit/verify` [`TestRekeyFloorIsNotNarrowed`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ts_narrow_test.go#L233) |
| `RFC7296-2.9.2-2` | The responder MUST NOT narrow down the Traffic Selectors narrower than the scope currently in use (§2.9.2) | MUST NOT | 2.9.2 | **positive:** `unit/verify` [`TestChildRekeyAnswerBelowTheScopeInUseIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_rekey_initiator_answer_test.go#L188). **positive:** `unit/verify` [`TestRekeyFloorIsNotNarrowed`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ts_narrow_test.go#L247). **positive:** `unit/verify` [`TestRekeyProposalBelowTheFloorIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_rekey_orientation_test.go#L378). **negative:** `unit/verify` [`TestChildRekeyAnswerBelowTheScopeInUseIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_rekey_initiator_answer_test.go#L198). **negative:** `unit/verify` [`TestRekeyFloorIsNotNarrowed`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ts_narrow_test.go#L261). **negative:** `unit/verify` [`TestRekeyProposalBelowTheFloorIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_rekey_orientation_test.go#L384) |
| `RFC7296-1.3.1-1` | If the request is accepted, the response MUST also include a notification of type USE_TRANSPORT_MODE (§1.3.1) | MUST | 1.3.1 - Creating a new Child SA | **positive:** `unit/verify` [`TestUseTransportModeNotifyIsEchoedOnlyWhenAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_test.go#L209). **negative:** `unit/verify` [`TestUseTransportModeNotifyIsEchoedOnlyWhenAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_test.go#L220) |
| `RFC7296-1.3.1-2` | If the responder declines the request, the Child SA will be established in tunnel mode. If this is unacceptable to the initiator, the initiator MUST delete the SA (§1.3.1) | MUST | 1.3.1 - Creating a new Child SA | **positive:** `unit/verify` [`TestTransportRequiredDeletesTheSAOnDecline`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_test.go#L249). **negative:** `unit/verify` [`TestTransportRequiredDeletesTheSAOnDecline`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_test.go#L264) |
| `RFC7296-2.23.1-1` | For transport mode, it MUST use exactly one IP address in the TSi and TSr payloads (§2.23.1) | MUST | 2.23.1 | **positive:** `unit/verify` [`TestResponderAnswersTheIKESAAddressesBehindANAT`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_nat_test.go#L107). **positive:** `unit/verify` [`TestTransportModeUsesExactlyOneAddress`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_test.go#L116). **negative:** `unit/verify` [`TestResponderAnswersTheIKESAAddressesBehindANAT`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_nat_test.go#L117). **negative:** `unit/verify` [`TestTransportModeUsesExactlyOneAddress`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_test.go#L129) |
| `RFC7296-2.23.1-2` | The TSi entries MUST have exactly one IP address, and that MUST match the source address of the IKE SA (§2.23.1) | MUST | 2.23.1 | **positive:** `unit/verify` [`TestTransportModeUsesExactlyOneAddress`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_test.go#L140). **positive:** `unit/verify` [`TestTransportSelectorsMatchTheIKESAAddressesBehindANAT`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_nat_test.go#L39). **negative:** `unit/verify` [`TestTransportModeUsesExactlyOneAddress`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_test.go#L170). **negative:** `unit/verify` [`TestTransportSelectorsMatchTheIKESAAddressesBehindANAT`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_nat_test.go#L62) |
| `RFC7296-2.23.1-3` | The TSr entries MUST have exactly one IP address, and that MUST match the destination address of the IKE SA (§2.23.1) | MUST | 2.23.1 | **positive:** `unit/verify` [`TestTransportModeUsesExactlyOneAddress`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_test.go#L150). **positive:** `unit/verify` [`TestTransportSelectorsMatchTheIKESAAddressesBehindANAT`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_nat_test.go#L51). **negative:** `unit/verify` [`TestTransportModeUsesExactlyOneAddress`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_test.go#L176). **negative:** `unit/verify` [`TestTransportSelectorsMatchTheIKESAAddressesBehindANAT`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_nat_test.go#L71) |
| `RFC7296-3.13.1-1` | For protocols for which port is undefined (including protocol 0), or if all ports are allowed, this field MUST be zero. (§3.13.1) | MUST | 3.13.1 | **positive:** `unit/verify` [`TestPortEncodingFollowsSection3131`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ts_narrow_test.go#L341). **negative:** `unit/verify` [`TestPortEncodingFollowsSection3131`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ts_narrow_test.go#L350) |
| `RFC7296-3.13.1-2` | For protocols for which port is undefined (including protocol 0), or if all ports are allowed, this field MUST be 65535. (§3.13.1) | MUST | 3.13.1 | **positive:** `unit/verify` [`TestPortEncodingFollowsSection3131`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ts_narrow_test.go#L358). **negative:** `unit/verify` [`TestPortEncodingFollowsSection3131`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ts_narrow_test.go#L364) |
| `RFC7296-3.13.1-3` | Systems working with [IPSECARCH] that wish to indicate "OPAQUE" ports, but not "ANY" ports, MUST set the start port to 65535 and the end port to 0. (§3.13.1) | MUST | 3.13.1 | **positive:** `unit/verify` [`TestPortEncodingFollowsSection3131`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ts_narrow_test.go#L372). **negative:** `unit/verify` [`TestPortEncodingFollowsSection3131`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ts_narrow_test.go#L400) |
| `RFC7296-2.23-1` | The IKE initiator MUST check the NAT_DETECTION_SOURCE_IP or NAT_DETECTION_DESTINATION_IP payloads if present (§2.23) | MUST | 2.23 | **positive:** `unit/verify` [`TestNATDetectionPresent`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/nat_test.go#L43). **negative:** `unit/verify` [`TestNATDetectionAbsent`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/nat_test.go#L63) |
| `RFC7296-2.23-2` | An IPsec endpoint that discovers a NAT between it and its correspondent (as described below) MUST send all subsequent traffic from port 4500 (§2.23) | MUST | 2.23 | **positive:** `unit/verify` [`TestChildSANATTEncapPorts`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L277). **negative:** `unit/verify` [`TestChildSANoNATNoEncap`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_test.go#L276) |
| `RFC7296-2.23-3` | To tunnel IKE packets over UDP port 4500, the IKE header has four octets of zeros prepended and the result immediately follows the UDP header. (§2.23) | MUST | 2.23 | **positive:** `unit/verify` [`TestNonESPMarker`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/nat_test.go#L80). **negative:** `unit/verify` [`TestNonESPMarkerESPPacket`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/nat_test.go#L109) |
| `RFC7296-2.8-1` | If redundant SAs are created through such a collision, the SA created with the lowest of the four nonces used in the two exchanges SHOULD be closed by the endpoint that created it (§2.8, §2.8.1) | SHOULD | 2.8 | **positive:** `unit/verify` [`TestRekeyCollision`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rekey_test.go#L23). **positive:** `unit/verify` [`TestRekeyCollisionIKEBranchLowestNonceAbandons`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rekey_test.go#L101). **positive:** `unit/verify` [`TestRekeyCollisionLowestNonceAbandons`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rekey_test.go#L46). **negative:** `unit/verify` [`TestRekeyCollision`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rekey_test.go#L27). **negative:** `unit/verify` [`TestRekeyCollisionIKEBranchLowestNonceAbandons`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rekey_test.go#L104). **negative:** `unit/verify` [`TestRekeyCollisionLowestNonceAbandons`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rekey_test.go#L50) |
| `RFC7296-2.4-1` | To check whether the other side is alive, IKE specifies an empty INFORMATIONAL request that (like all IKE requests) requires an acknowledgement (§2.4) | MUST | 2.4 | **positive:** `unit/verify` [`TestDPDEmptyInformationalGetsEmptyResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_test.go#L70). **negative:** `unit/verify` [`TestDPDEmptyInformationalGetsEmptyResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_test.go#L73) |
| `RFC7296-1.4-1` | Normally, the response in the INFORMATIONAL exchange will contain Delete payloads for the paired SAs going in the other direction. (§1.4.1) | MUST | 1.4.1 - Deleting an SA | **positive:** `unit/verify` [`TestDelResponseCarriesThePairedDelete`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_delete_test.go#L111). **negative:** `unit/verify` [`TestDelIKEDeleteDrawsAnEmptyResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_delete_test.go#L132) |
| `RFC7296-2.8-2` | A difference between IKEv1 and IKEv2 is that in IKEv1 SA lifetimes were negotiated. In IKEv2, each end of the SA is responsible for enforcing its own lifetime policy on the SA and rekeying the SA when necessary. (§2.8) | MUST NOT | 2.8 | **positive:** `unit/verify` [`TestSALifetimeTime`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rekey_test.go#L151). **negative:** `unit/verify` [`TestLifetimesNotNegotiatedOnWire`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_test.go#L300) |
| `RFC7296-1-1` | In all cases, all IKE_SA_INIT exchanges MUST complete before any other exchange type, then all IKE_AUTH exchanges MUST complete, and following that, any number of CREATE_CHILD_SA and INFORMATIONAL exchanges may occur in any order (§1) | MUST | 1 | **positive:** `unit/verify` [`TestAutExchangesRunInRFCOrder`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_auth_test.go#L168). **negative:** `unit/verify` [`TestAutExchangesRunInRFCOrder`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_auth_test.go#L170) |
| `RFC7296-1.2-2` | If any CERT payloads are included, the first certificate provided MUST contain the public key used to verify the AUTH field (§1.2, §3.6) | MUST | 1.2 - The initial exchanges | **positive:** `unit/verify` [`TestAutFirstCertificateCarriesTheAuthKey`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_auth_test.go#L250). **positive:** `unit/verify` [`TestRccTwoLevelChainAuthenticates`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/remote_cert_chain_test.go#L210). **negative:** `unit/verify` [`TestAutFirstCertificateCarriesTheAuthKey`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_auth_test.go#L252). **negative:** `unit/verify` [`TestRccTwoLevelChainAuthenticates`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/remote_cert_chain_test.go#L213) |
| `RFC7296-1.2-3` | Both parties in the IKE_AUTH exchange MUST verify that all signatures and Message Authentication Codes (MACs) are computed correctly (§1.2) | MUST | 1.2 - The initial exchanges | **positive:** `unit/verify` [`TestAutIKEAuthVerifiesEveryMAC`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_auth_test.go#L301). **positive:** `unit/verify` [`TestAutIKEAuthVerifiesSignatures`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_auth_test.go#L366). **negative:** `unit/verify` [`TestAutIKEAuthVerifiesEveryMAC`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_auth_test.go#L303). **negative:** `unit/verify` [`TestAutIKEAuthVerifiesSignatures`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_auth_test.go#L368) |
| `RFC7296-1.2-4` | If either side uses a shared secret for authentication, the names in the ID payload MUST correspond to the key used to generate the AUTH payload (§1.2) | MUST | 1.2 - The initial exchanges | **positive:** `unit/verify` [`TestAutSharedSecretAuthBindsTheIDName`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_auth_test.go#L428). **negative:** `unit/verify` [`TestAutSharedSecretAuthBindsTheIDName`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_auth_test.go#L430) |
| `RFC7296-1.2-5` | If the initiator guesses wrong, the responder will respond with a Notify payload of type INVALID_KE_PAYLOAD indicating the selected group. In this case, the initiator MUST retry the IKE_SA_INIT with the corrected Diffie-Hellman group (§1.2) | MUST | 1.2 - The initial exchanges | **positive:** `unit/verify` [`TestKegInitiatorRetriesOnInvalidKEPayload`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_kegroup_test.go#L103). **negative:** `unit/verify` [`TestKegInitiatorRefusesUnofferedGroup`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_kegroup_test.go#L142) |
| `RFC7296-1.2-6` | The initiator MUST again propose its full set of acceptable cryptographic suites because the rejection message was unauthenticated and otherwise an active attacker could trick the endpoints into negotiating a weaker suite than a stronger one that they both prefer (§1.2) | MUST | 1.2 - The initial exchanges | **positive:** `unit/verify` [`TestKegRetryReproposesEveryConfiguredSuite`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_kegroup_test.go#L190). **negative:** `unit/verify` [`TestKegRetryOfferIsNotNarrowedByTheNotify`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_kegroup_test.go#L225) |
| `RFC7296-2.4-11` | An endpoint MUST conclude that the other endpoint has failed only when repeated attempts to contact it have gone unanswered for a timeout period or when a cryptographically protected INITIAL_CONTACT notification is received on a different IKE SA to the same authenticated identity (§2.4) | MUST | 2.4 | **positive:** `unit/verify` [`TestDPDVerdictEndsAProbeThatCannotBeRepeated`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/dpd_test.go#L277). **positive:** `unit/verify` [`TestDPDVerdictNeedsARepeatedAttempt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/dpd_test.go#L233). **positive:** `unit/verify` [`TestSesPeerFailedOnlyAfterRepeatedSilence`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_session_test.go#L306). **negative:** `unit/verify` [`TestDPDNoTransportTakesNoWindow`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/dpd_test.go#L104). **negative:** `unit/verify` [`TestDPDVerdictNeedsARepeatedAttempt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/dpd_test.go#L231). **negative:** `unit/verify` [`TestSesPeerFailedOnlyAfterRepeatedSilence`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_session_test.go#L311) |
| `RFC7296-2.4-12` | Implementations MUST limit the rate at which they take actions based on unprotected messages (§2.4) | MUST | 2.4 | **positive:** `unit/verify` [`TestErrResponderWindowDoesNotReflectToObservedSource`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L515). **positive:** `unit/verify` [`TestErrUnprotectedMessageDrawsNoCachedResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L228). **positive:** `unit/verify` [`TestSesLimitsWorkOnUnprotectedMessages`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_session_test.go#L543). **negative:** `unit/verify` [`TestErrResponderWindowDoesNotReflectToObservedSource`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L522). **negative:** `unit/verify` [`TestErrUnprotectedMessageDrawsNoCachedResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L233). **negative:** `unit/verify` [`TestSesLimitsWorkOnUnprotectedMessages`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_session_test.go#L547) |
| `RFC7296-2.4-13` | To be a good network citizen, retransmission times MUST increase exponentially to avoid flooding the network and making an existing congestion situation worse (§2.4) | MUST | 2.4 | **positive:** `unit/verify` [`TestSesRetransmitWaitIncreasesExponentially`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_session_test.go#L260). **negative:** `unit/verify` [`TestSesRetransmitWaitIncreasesExponentially`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_session_test.go#L262) |
| `RFC7296-2.7-2` | Each proposal contains one protocol. If a proposal is accepted, the SA response MUST contain the same protocol (§2.7) | MUST | 2.7 | **positive:** `unit/verify` [`TestSesAcceptedProposalKeepsItsProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_session_test.go#L396). **negative:** `unit/verify` [`TestSesAcceptedProposalKeepsItsProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_session_test.go#L399) |
| `RFC7296-2.10-4` | Nonces used in IKEv2 MUST be randomly chosen (§2.10) | MUST | 2.10 | **positive:** `unit/verify` [`TestSesNoncesAreRandomlyChosenAndNeverReused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_session_test.go#L93). **negative:** `unit/verify` [`TestSesNoncesAreRandomlyChosenAndNeverReused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_session_test.go#L95) |
| `RFC7296-2.11-1` | An implementation MUST accept incoming requests even if the source port is not 500 or 4500 (§2.11, §2.23) | MUST | 2.11 | **positive:** `unit/verify` [`TestPrtAcceptsDatagramFromAnySourcePort`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/rfc7296_port_test.go#L43). **positive:** `unit/verify` [`TestSesAcceptsRequestFromAnySourcePort`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_session_test.go#L483). **negative:** `unit/verify` [`TestPrtAcceptsDatagramFromAnySourcePort`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/rfc7296_port_test.go#L46). **negative:** `unit/verify` [`TestSesAcceptsRequestFromAnySourcePort`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_session_test.go#L486) |
| `RFC7296-2.11-2` | MUST respond to the address and port from which the request was received (§2.11) | MUST | 2.11 | **positive:** `unit/verify` [`TestNattRepliesToTheObservedSourcePort`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_test.go#L111). **negative:** `unit/verify` [`TestNattUnauthenticatedPacketDoesNotMoveTheEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_test.go#L173) |
| `RFC7296-2.11-3` | It MUST specify the address and port at which the request was received as the source address and port in the response (§2.11) | MUST | 2.11 | **positive:** `unit/verify` [`TestNattReplyLeavesFromTheArrivalSocket`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_test.go#L225). **negative:** `unit/verify` [`TestNattReplyRefusesWithoutADestination`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_test.go#L330) |
| `RFC7296-2.16-11` | these protocols are typically used to authenticate the initiator to the responder and MUST be used in conjunction with a public-key-signature-based authentication of the responder to the initiator. (§2.16) | MUST | 2.16 | **positive:** `unit/verify` [`TestEapAuthConfigAcceptsPreSharedKeyPeer`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_eap_auth_test.go#L324). **positive:** `unit/verify` [`TestEapAuthNonEAPPreSharedKeyStillAuthenticates`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_eap_auth_test.go#L240). **positive:** `unit/verify` [`TestEapAuthResponderSignsWithPublicKey`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_eap_auth_test.go#L82). **negative:** `unit/verify` [`TestEapAuthConfigRejectsEAPWithoutCertificate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_eap_auth_test.go#L206). **negative:** `unit/verify` [`TestEapAuthInitiatorRefusesPreSharedKeyResponder`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_eap_auth_test.go#L267). **negative:** `unit/verify` [`TestEapAuthResponderRefusesWithoutCertificate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_eap_auth_test.go#L153) |
| `RFC7296-2.16-6` | Extensible authentication is implemented in IKE as additional IKE_AUTH exchanges that MUST be completed in order to initialize the IKE SA (§2.16) | MUST | 2.16 | **positive:** `unit/verify` [`TestAutEAPRunsAsExtraIKEAuthExchanges`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_auth_test.go#L562). **negative:** `unit/verify` [`TestAutEAPRunsAsExtraIKEAuthExchanges`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_auth_test.go#L564) |
| `RFC7296-2.16-7` | This shared key generated during an IKE exchange MUST NOT be used for any other purpose (§2.16) | MUST NOT | 2.16 | **positive:** `unit/verify` [`TestAutEAPSharedKeyServesAuthAlone`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_auth_test.go#L619). **negative:** `unit/verify` [`TestAutEAPSharedKeyServesAuthAlone`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_auth_test.go#L621) |
| `RFC7296-2.23-7` | Both the IKE initiator and responder MUST include in their IKE_SA_INIT packets Notify payloads of type NAT_DETECTION_SOURCE_IP and NAT_DETECTION_DESTINATION_IP (§2.23) | MUST | 2.23 | **positive:** `unit/verify` [`TestSesBothEndsSendNATDetectionNotifies`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_session_test.go#L631). **negative:** `unit/verify` [`TestSesBothEndsSendNATDetectionNotifies`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_session_test.go#L633) |
| `RFC7296-2.23-8` | An IPsec endpoint that discovers a NAT between it and its correspondent (as described below) MUST send all subsequent traffic from port 4500 (§2.23) | MUST | 2.23 | **positive:** `unit/verify` [`TestNattFloatsEverySenderToPort4500`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_test.go#L355). **positive:** `unit/verify` [`TestNattRekeyedChildKeepsUDPEncap`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_test.go#L549). **negative:** `unit/verify` [`TestNattFloatedSAWithoutNATTSocketSendsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_test.go#L459). **negative:** `unit/verify` [`TestNattNoFloatWithoutNAT`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_test.go#L425) |
| `RFC7296-2.23-9` | UDP encapsulation MUST NOT be done on port 500 (§2.23) | MUST NOT | 2.23 | **positive:** `unit/verify` [`TestEncapNeverRequestedOnPort500`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_test.go#L481). **negative:** `unit/verify` [`TestEncapPortsAreExpressible`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_test.go#L581) |
| `RFC7296-2.23-10` | If Network Address Translation Traversal (NAT-T) is supported (that is, if NAT_DETECTION_*_IP payloads were exchanged during IKE_SA_INIT), all devices MUST be able to receive and process both UDP-encapsulated ESP and non-UDP-encapsulated ESP packets at any time. (§2.23) | MUST | 2.23 | **positive:** `unit/verify` [`TestBfmBothESPFormsAreReachable`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_bothforms_test.go#L153). **negative:** `unit/verify` [`TestBfmBothESPFormsReceivedOnOneChildSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_bothforms_test.go#L209) |
| `RFC7296-2.23-11` | Implementations MUST process received UDP-encapsulated ESP packets even when no NAT was detected (§2.23) | MUST | 2.23 | **positive:** `unit/verify` [`TestBfmEncapsulatedESPAcceptedWithoutNAT`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_bothforms_test.go#L63). **positive:** `unit/verify` [`TestBfmEncapsulatedESPSentWhenNATDetected`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_bothforms_test.go#L112). **negative:** `unit/verify` [`TestBfmBareESPKeptForUnfloatedSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_bothforms_test.go#L136) |
| `RFC7296-2.23-12` | To tunnel ESP packets over UDP port 4500, the ESP header immediately follows the UDP header. Since the first four octets of the ESP header contain the SPI, and the SPI cannot validly be zero, it is always possible to distinguish ESP and IKE messages. (§2.23) | MUST | 2.23 | **positive:** `unit/verify` [`TestNonESPMarker`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/nat_test.go#L83). **negative:** `unit/verify` [`TestNonESPMarkerESPPacket`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/nat_test.go#L112) |
| `RFC7296-3.9-2` | Nonce values MUST NOT be reused (§3.9) | MUST NOT | 3.9 | **positive:** `unit/verify` [`TestSesNoncesAreRandomlyChosenAndNeverReused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_session_test.go#L97). **positive:** `unit/verify` [`TestSesRekeyDrawsFreshNoncesOnBothSides`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_session_test.go#L145). **negative:** `unit/verify` [`TestSesNoncesAreRandomlyChosenAndNeverReused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_session_test.go#L99). **negative:** `unit/verify` [`TestSesRekeyDrawsFreshNoncesOnBothSides`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_session_test.go#L147) |
| `RFC7296-1.3-2` | If the responder selects a proposal using a different Diffie-Hellman group (other than NONE), the responder MUST reject the request and indicate its preferred Diffie-Hellman group in the INVALID_KE_PAYLOAD Notify payload (§1.3, §3.4) | MUST | 1.3 - The CREATE_CHILD_SA exchange | **positive:** `unit/verify` [`TestNegRekeyRejectsMismatchedKEGroup`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_negotiation_test.go#L276). **negative:** `unit/verify` [`TestNegRekeyRejectsMismatchedKEGroup`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_negotiation_test.go#L280) |
| `RFC7296-2.8.2-1` | The new IKE SA containing the lowest nonce SHOULD be deleted by the node that created it, and the other surviving new IKE SA MUST inherit all the Child SAs (§2.8.2) | MUST | 2.8.2 | **positive:** `unit/verify` [`TestNegIKERekeyCollisionResolves`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_negotiation_test.go#L399). **positive:** `unit/verify` [`TestNegSurvivingSAInheritsChildren`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_negotiation_test.go#L476). **negative:** `unit/verify` [`TestNegIKERekeyCollisionResolves`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_negotiation_test.go#L403). **negative:** `unit/verify` [`TestNegSurvivingSAInheritsChildren`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_negotiation_test.go#L479) |
| `RFC7296-3.3.6-3` | The initiator of an exchange MUST check that the accepted offer is consistent with one of its proposals, and if not MUST terminate the exchange (§3.3.6) | MUST | 3.3.6 | **positive:** `unit/verify` [`TestEsnInitiatorRefusesAnESNValueItNeverOffered`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_esn_test.go#L150). **positive:** `unit/verify` [`TestKlnInitiatorRefusesLongerAcceptedKey`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_keylength_test.go#L52). **positive:** `unit/verify` [`TestNegInitiatorRejectsUnproposedOffer`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_negotiation_test.go#L542). **positive:** `unit/verify` [`TestVerInitiatorRefusesUnsentESPKeyLength`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_verify_test.go#L76). **positive:** `unit/verify` [`TestVerInitiatorRefusesUnsentIKEKeyLength`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_verify_test.go#L41). **negative:** `unit/verify` [`TestEsnInitiatorRefusesAnESNValueItNeverOffered`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_esn_test.go#L144). **negative:** `unit/verify` [`TestKlnInitiatorRefusesLongerAcceptedKey`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_keylength_test.go#L46). **negative:** `unit/verify` [`TestNegInitiatorRejectsUnproposedOffer`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_negotiation_test.go#L546). **negative:** `unit/verify` [`TestVerInitiatorRefusesUnsentESPKeyLength`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_verify_test.go#L74). **negative:** `unit/verify` [`TestVerInitiatorRefusesUnsentIKEKeyLength`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_verify_test.go#L34) |
| `RFC7296-1.4-3` | Control messages that pertain to an IKE SA MUST be sent under that IKE SA. Control messages that pertain to Child SAs MUST be sent under the protection of the IKE SA that generated them (§1.4) | MUST | 1.4 - The INFORMATIONAL exchange | **positive:** `unit/verify` [`TestLcyControlMessagesRideTheirOwnIKESA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_lifecycle_test.go#L116). **negative:** `unit/verify` [`TestLcyControlMessagesRideTheirOwnIKESA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_lifecycle_test.go#L120) |
| `RFC7296-1.4-4` | The recipient of an INFORMATIONAL exchange request MUST send some response; otherwise, the sender will assume the message was lost in the network and will retransmit it (§1.4, §4) | MUST | 1.4 - The INFORMATIONAL exchange | **positive:** `unit/verify` [`TestLcyEveryInformationalRequestDrawsAResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_lifecycle_test.go#L176). **negative:** `unit/verify` [`TestLcyEveryInformationalRequestDrawsAResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_lifecycle_test.go#L179) |
| `RFC7296-1.4-5` | INFORMATIONAL exchanges MUST ONLY occur after the initial exchanges and are cryptographically protected with the negotiated keys (§1.4) | MUST | 1.4 - The INFORMATIONAL exchange | **positive:** `unit/verify` [`TestDpdProbeIsEncrypted`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_dpd_test.go#L63). **negative:** `unit/verify` [`TestDpdProbeIsEncrypted`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_dpd_test.go#L67) |
| `RFC7296-1.4.1-6` | To delete an SA, an INFORMATIONAL exchange with one or more Delete payloads is sent listing the SPIs (as they would be expected in the headers of inbound packets) of the SAs to be deleted. The recipient MUST close the designated SAs (§1.4.1) | MUST | 1.4.1 - Deleting an SA | **positive:** `unit/verify` [`TestDelPeerDeleteClosesTheDesignatedChildSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_delete_test.go#L61). **negative:** `unit/verify` [`TestDelUnknownSPIClosesNothing`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_delete_test.go#L81) |
| `RFC7296-1.4.1-7` | If a node receives a delete request for SAs for which it has already issued a delete request, it MUST delete the outgoing SAs while processing the request and the incoming SAs while processing the response (§1.4.1) | MUST | 1.4.1 - Deleting an SA | **positive:** `unit/verify` [`TestDelCrossingDeleteAnswersWithoutAPairedDelete`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_delete_test.go#L159). **negative:** `unit/verify` [`TestDelWithoutOwnDeleteTheSameRequestIsPaired`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_delete_test.go#L277) |
| `RFC7296-1.5-1` | This message is not part of an INFORMATIONAL exchange, and the receiving node MUST NOT respond to it because doing so could cause a message loop (§1.5) | MUST NOT | 1.5 - Informational messages outside an IKE SA | **positive:** `unit/verify` [`TestWp2OutOfSAEmitterIsAFixedPoint`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_wp2_test.go#L21). **negative:** `unit/verify` [`TestWp2OutOfSAAnswersARequest`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_wp2_test.go#L44) |
| `RFC7296-2.12-1` | Achieving perfect forward secrecy requires that when a connection is closed, each endpoint MUST forget not only the keys used by the connection but also any information that could be used to recompute those keys (§2.12) | MUST | 2.12 | **positive:** `unit/verify` [`TestRunEstablishedClearsPendingIKESwapOnExit`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/established_test.go#L492). **positive:** `unit/verify` [`TestWp2ForgetKeysErasesEverySecret`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_wp2_test.go#L85). **negative:** `unit/verify` [`TestWp2OpenSAKeepsItsKeys`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_wp2_test.go#L116) |
| `RFC7296-2.24-1` | tunnel encapsulators and decapsulators for all tunnel mode SAs created by IKEv2 MUST support the ECN full-functionality option for tunnels specified in [ECN] (§2.24) | MUST | 2.24 | **positive:** `unit/verify` [`TestEcnInstalledChildSAAsksForNoECNChange`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_ecn_test.go#L70). **positive:** `unit/verify` [`TestEcnInstalledStateDisablesNothing`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc7296_ecn_linux_test.go#L86). **positive:** `unit/verify` [`TestVPPInstallSACopiesECN`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/vpp_message_test.go#L937). **negative:** `unit/verify` [`TestEcnTheInstalledSAsAreRealAndDirectional`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_ecn_test.go#L91). **negative:** `unit/verify` [`TestEcnTheScannedStateIsTheOneZeInstalls`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc7296_ecn_linux_test.go#L109). **negative:** `unit/verify` [`TestVPPInstallSAECNIsOnTheSAZeInstalls`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/vpp_message_test.go#L961) |
| `RFC7296-2.24-2` | MUST implement the tunnel encapsulation and decapsulation processing specified in [IPSECARCH] to prevent discarding of ECN congestion indications. (§2.24) | MUST | 2.24 | **positive:** `unit/verify` [`TestEcnInstalledChildSAAsksForNoECNChange`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_ecn_test.go#L74). **positive:** `unit/verify` [`TestEcnInstalledStateDisablesNothing`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc7296_ecn_linux_test.go#L90). **positive:** `unit/verify` [`TestVPPInstallSACopiesECN`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/vpp_message_test.go#L941). **negative:** `unit/verify` [`TestEcnTheInstalledSAsAreRealAndDirectional`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_ecn_test.go#L95). **negative:** `unit/verify` [`TestEcnTheScannedStateIsTheOneZeInstalls`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc7296_ecn_linux_test.go#L113). **negative:** `unit/verify` [`TestVPPInstallSAECNIsOnTheSAZeInstalls`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/vpp_message_test.go#L965) |
| `RFC7296-2.16-12` | For EAP methods that create a shared key as a side effect of authentication, that shared key MUST be used by both the initiator and responder to generate AUTH payloads in messages 7 and 8 using the syntax for shared secrets specified in Section 2.15 (§2.16) | MUST | 2.16 | **positive:** `unit/verify` [`TestEapAuthProducerIsKeyedByTheNegotiatedMSK`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_eap_auth_producer_test.go#L43). **negative:** `unit/verify` [`TestEapAuthProducerOutputIsRefusedUnderAnotherKey`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_eap_auth_producer_test.go#L82) |
| `RFC7296-2.16-13` | Following such an extended exchange, the EAP AUTH payloads MUST be included in the two messages following the one containing the EAP Success message (§2.16) | MUST | 2.16 | **positive:** `unit/verify` [`TestEapAuthFollowsTheSuccessMessage`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_eap_auth_producer_test.go#L232). **negative:** `unit/verify` [`TestEapAuthFollowsTheSuccessMessage`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_eap_auth_producer_test.go#L235) |
| `RFC7296-2.16-14` | Once the protocol exchange defined by the chosen EAP authentication method has successfully terminated, the responder MUST send an EAP payload containing the Success message (§2.16) | MUST | 2.16 | **positive:** `unit/verify` [`TestEapResultSuccessIsSent`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc7296_eap_result_test.go#L55). **negative:** `unit/verify` [`TestEapResultFailureIsSent`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc7296_eap_result_test.go#L81) |
| `RFC7296-2.16-15` | Similarly, if the authentication method has failed, the responder MUST send an EAP payload containing the Failure message (§2.16) | MUST | 2.16 | **positive:** `unit/verify` [`TestEapResultFailureIsSent`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc7296_eap_result_test.go#L78). **negative:** `unit/verify` [`TestEapResultSuccessIsNotFailure`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc7296_eap_result_test.go#L101) |
| `RFC7296-3.1-13` | * I (Initiator) - This bit MUST be set in messages sent by the original initiator of the IKE SA and MUST be cleared in messages sent by the original responder. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestWp2DPDProbeIBitFollowsRole`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_wp2_test.go#L259). **negative:** `unit/verify` [`TestWp2DPDProbeIBitDiffersByRole`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_wp2_test.go#L276) |
| `RFC7296-3.5-5` | The string MUST NOT contain any terminators (e.g., NULL, CR, etc.). All characters in the ID_FQDN are ASCII; for an "internationalized domain name", the syntax is as defined in [IDNA], for example "xn--tmonesimerkki-bfbb.example.net". ID_RFC822_ADDR 3 A fully-qualified RFC 822 email address string. An example of a ID_RFC822_ADDR is "jsmith@example.com". The string MUST NOT contain any terminators. (§3.5) | MUST NOT | 3.5 | **positive:** `unit/verify` [`TestWp2IDTerminatorRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_wp2_test.go#L299). **negative:** `unit/verify` [`TestWp2IDWithoutTerminatorAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_wp2_test.go#L339) |
| `RFC7296-1.4.1-1` | When an SA is closed, both members of the pair MUST be closed (that is, deleted). Each endpoint MUST close its incoming SAs and allow the other endpoint to close the other SA in each pair (§1.4.1) | MUST | 1.4.1 - Deleting an SA | **positive:** `unit/verify` [`TestLcyClosingAChildSAClosesBothHalvesAndDeletesOurInbound`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_lifecycle_test.go#L232). **negative:** `unit/verify` [`TestLcyClosingAChildSAClosesBothHalvesAndDeletesOurInbound`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_lifecycle_test.go#L236) |
| `RFC7296-1.4.1-4` | In that case, the responses MUST NOT include Delete payloads for the deleted SAs, since that would result in duplicate deletion and could in theory delete the wrong SA. (§1.4.1) | MUST NOT | 1.4.1 - Deleting an SA | **positive:** `unit/verify` [`TestLcyInformationalResponseCarriesNoDeletePayload`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_lifecycle_test.go#L289). **negative:** `unit/verify` [`TestLcyInformationalResponseCarriesNoDeletePayload`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_lifecycle_test.go#L294) |
| `RFC7296-1.4.1-5` | A node MAY refuse to accept incoming data on half-closed connections but MUST NOT unilaterally close them and reuse the SPIs (§1.4.1) | MUST NOT | 1.4.1 - Deleting an SA | **positive:** `unit/verify` [`TestLcyRetiredSPIsAreNeverReused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_lifecycle_test.go#L344). **negative:** `unit/verify` [`TestLcyRetiredSPIsAreNeverReused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_lifecycle_test.go#L348) |
| `RFC7296-2.4-9` | If a system creates Child SAs that can fail independently from one another without the associated IKE SA being able to send a delete message, then the system MUST negotiate such Child SAs using separate IKE SAs (§2.4) | MUST | 2.4 | **positive:** `unit/verify` [`TestLcyOneChildSALivesUnderOneIKESA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_lifecycle_test.go#L417). **negative:** `unit/verify` [`TestLcyOneChildSALivesUnderOneIKESA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_lifecycle_test.go#L422) |
| `RFC7296-2.4-10` | If an IKE endpoint chooses to delete Child SAs, it MUST send Delete payloads to the other end notifying it of the deletion (§2.4) | MUST | 2.4 | **positive:** `unit/verify` [`TestLcyRetiringAChildSASendsADeletePayload`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_lifecycle_test.go#L453). **negative:** `unit/verify` [`TestLcyRetiringAChildSASendsADeletePayload`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_lifecycle_test.go#L456) |
| `RFC7296-2.16-5` | If EAP methods that do not generate a shared key are used, the AUTH payloads in messages 7 and 8 MUST be generated using SK_pi and SK_pr, respectively (§2.16) | MUST | 2.16 | **positive:** `unit/verify` [`TestEAPAuthOfNonKeyDerivingMethodUsesSKpiAndSKpr`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_eap_nonkeying_auth_test.go#L126). **negative:** `unit/verify` [`TestEAPAuthOfKeyDerivingMethodStillUsesTheMSK`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_eap_nonkeying_auth_test.go#L174) |
| `RFC7296-2.16-16` | EAP methods that do not establish a shared key SHOULD NOT be used, as they are subject to a number of man-in-the-middle attacks [EAPMITM] if these EAP methods are used in other protocols that do not use a server-authenticated tunnel. (§2.16) | SHOULD NOT | 2.16 | **positive:** `unit/verify` [`TestRFC3748IKEv2EAPModesSelectAKeyDerivingMethod`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc3748_ikev2_method_selection_test.go#L120). **negative:** `unit/verify` [`TestRFC3748IKEv2NoAuthModeSelectsAKeylessMethod`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc3748_ikev2_method_selection_test.go#L171) |
| `RFC7296-3.4-1` | The length of the Diffie-Hellman public value for MODP groups MUST be equal to the length of the prime modulus over which the exponentiation was performed, prepending zero bits to the value if necessary (§3.4) | MUST | 3.4 | **positive:** `unit/verify` [`TestRFC7296MODPPublicValueMatchesModulusLength`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_dh_test.go#L30). **negative:** `unit/verify` [`TestRFC7296MODPShortPublicValueIsRefusedOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_dh_test.go#L91) |
| `RFC7296-1.3-1` | If a CREATE_CHILD_SA exchange includes a KEi payload, at least one of the SA offers MUST include the Diffie-Hellman group of the KEi (§1.3) | MUST | 1.3 - The CREATE_CHILD_SA exchange | **positive:** `unit/verify` [`TestChildRekeyProposesDiffieHellmanWhenPFSIsEnabled`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_rekey_pfs_test.go#L62). **positive:** `unit/verify` [`TestRkyIKERekeyOffersTheKEiGroup`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_rekey_test.go#L167). **negative:** `unit/verify` [`TestChildRekeyProposesDiffieHellmanWhenPFSIsEnabled`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_rekey_pfs_test.go#L66). **negative:** `unit/verify` [`TestChildRekeyProposesNoDiffieHellmanWhenPFSIsDisabled`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_rekey_pfs_test.go#L100). **negative:** `unit/verify` [`TestRkyIKERekeyOffersTheKEiGroup`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_rekey_test.go#L171) |
| `RFC7296-2.1-3` | The responder MUST never retransmit a response unless it receives a retransmission of the request (§2.1) | MUST | 2.1 - Retransmission timers | **positive:** `unit/verify` [`TestEapRtxResponderReplaysCachedResponseMidEAP`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_eap_retransmit_test.go#L96). **positive:** `unit/verify` [`TestRtxResponderReplaysCachedResponseOnlyForDuplicate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_retransmit_test.go#L206). **negative:** `unit/verify` [`TestEapRtxMidEAPReplayRefusesUnprotected`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_eap_retransmit_test.go#L150). **negative:** `unit/verify` [`TestRtxResponderReplaysCachedResponseOnlyForDuplicate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_retransmit_test.go#L209) |
| `RFC7296-2.1-4` | In that event, the responder MUST ignore the retransmitted request except insofar as it causes a retransmission of the response (§2.1) | MUST | 2.1 - Retransmission timers | **positive:** `unit/verify` [`TestEapRtxResponderReplaysCachedResponseMidEAP`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_eap_retransmit_test.go#L89). **positive:** `unit/verify` [`TestRtxResponderReplaysCachedResponseOnlyForDuplicate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_retransmit_test.go#L211). **negative:** `unit/verify` [`TestEapRtxResponderReplaysCachedResponseMidEAP`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_eap_retransmit_test.go#L93). **negative:** `unit/verify` [`TestRtxResponderReplaysCachedResponseOnlyForDuplicate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_retransmit_test.go#L213) |
| `RFC7296-2.1-5` | The initiator MUST remember each request until it receives the corresponding response. The responder MUST remember each response until it receives a request whose sequence number is larger than or equal to the sequence number in the response plus its window size (§2.1, §2.3) | MUST | 2.1 - Retransmission timers | **positive:** `unit/verify` [`TestRtxEachSideRemembersWhatItSent`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_retransmit_test.go#L147). **positive:** `unit/verify` [`TestWinDeleteIsRememberedUntilAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_window_test.go#L419). **negative:** `unit/verify` [`TestRtxEachSideRemembersWhatItSent`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_retransmit_test.go#L150). **negative:** `unit/verify` [`TestWinDeleteIsRememberedUntilAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_window_test.go#L424) |
| `RFC7296-2.1-6` | If the responder receives a retransmitted request for which it has already forgotten the response, it MUST ignore the request (and not, for example, attempt constructing a new response) (§2.1) | MUST | 2.1 - Retransmission timers | **positive:** `unit/verify` [`TestRtxResponderIgnoresRequestWithForgottenResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_retransmit_test.go#L272). **negative:** `unit/verify` [`TestRtxResponderIgnoresRequestWithForgottenResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_retransmit_test.go#L275) |
| `RFC7296-2.1-7` | IKE is a reliable protocol: the initiator MUST retransmit a request until it either receives a corresponding response or deems the IKE SA to have failed (§2.1) | MUST | 2.1 - Retransmission timers | **positive:** `unit/verify` [`TestRtxInitiatorResendsUnansweredRekeyRequest`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_retransmit_test.go#L331). **positive:** `unit/verify` [`TestRtxInitiatorResendsUnansweredSAInit`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_retransmit_test.go#L395). **positive:** `unit/verify` [`TestWinUnansweredRequestFailsTheSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_window_test.go#L326). **negative:** `unit/verify` [`TestRtxInitiatorResendsUnansweredRekeyRequest`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_retransmit_test.go#L334). **negative:** `unit/verify` [`TestRtxInitiatorResendsUnansweredSAInit`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_retransmit_test.go#L398). **negative:** `unit/verify` [`TestWinUnansweredRequestFailsTheSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_window_test.go#L331) |
| `RFC7296-2.1-8` | A retransmission from the initiator MUST be bitwise identical to the original request (§2.1) | MUST | 2.1 - Retransmission timers | **positive:** `unit/verify` [`TestRtxRetransmissionIsBitwiseIdenticalAndReusesMessageID`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_retransmit_test.go#L446). **negative:** `unit/verify` [`TestRtxRetransmissionIsBitwiseIdenticalAndReusesMessageID`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_retransmit_test.go#L448) |
| `RFC7296-2.2-1` | Retransmission of a message MUST use the same Message ID as the original message (§2.2) | MUST | 2.2 - Message ID sequence numbers | **positive:** `unit/verify` [`TestRtxRetransmissionIsBitwiseIdenticalAndReusesMessageID`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_retransmit_test.go#L450). **negative:** `unit/verify` [`TestRtxRetransmissionIsBitwiseIdenticalAndReusesMessageID`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_retransmit_test.go#L452) |
| `RFC7296-2.2-2` | In the unlikely event that Message IDs grow too large to fit in 32 bits, the IKE SA MUST be closed or rekeyed (§2.2) | MUST | 2.2 - Message ID sequence numbers | **positive:** `unit/verify` [`TestMidInboundCounterFreezesAtTheCeiling`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_msgid_test.go#L211). **positive:** `unit/verify` [`TestMidNearExhaustionRekeysTheIKESA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_msgid_test.go#L112). **positive:** `unit/verify` [`TestMidOutboundCounterFreezesAtTheCeiling`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_msgid_test.go#L42). **positive:** `unit/verify` [`TestMidResponderEstablishDoesNotWrapTheCounter`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_msgid_test.go#L456). **negative:** `unit/verify` [`TestMidInboundCounterFreezesAtTheCeiling`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_msgid_test.go#L217). **negative:** `unit/verify` [`TestMidNearExhaustionRekeysTheIKESA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_msgid_test.go#L118). **negative:** `unit/verify` [`TestMidOutboundCounterFreezesAtTheCeiling`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_msgid_test.go#L49). **negative:** `unit/verify` [`TestMidResponderEstablishDoesNotWrapTheCounter`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_msgid_test.go#L461) |
| `RFC7296-2.2-3` | Each endpoint in the IKE Security Association maintains two "current" Message IDs: the next one to be used for a request it initiates and the next one it expects to see in a request from the other end. (§2.2) | MUST | 2.2 - Message ID sequence numbers | **positive:** `unit/verify` [`TestResponderFirstRequestMatchesWhatTheInitiatorExpects`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_msgid_responder_request_test.go#L35). **negative:** `unit/verify` [`TestMidResponderEstablishDoesNotWrapTheCounter`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_msgid_test.go#L466) |
| `RFC7296-2.3-2` | An IKE endpoint MUST wait for a response to each of its messages before sending a subsequent message unless it has received a SET_WINDOW_SIZE Notify message from its peer (§2.3) | MUST | 2.3 - Window size for overlapping requests | **positive:** `unit/verify` [`TestWinOneRequestPerTick`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_window_test.go#L84). **positive:** `unit/verify` [`TestWinTeardownDoesNotHang`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_window_test.go#L531). **negative:** `unit/verify` [`TestDPDNoTransportTakesNoWindow`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/dpd_test.go#L106). **negative:** `unit/verify` [`TestWinOneRequestPerTick`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_window_test.go#L89). **negative:** `unit/verify` [`TestWinTeardownDoesNotHang`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_window_test.go#L537) |
| `RFC7296-2.3-4` | An IKE endpoint MUST NOT exceed the peer's stated window size for transmitted IKE requests (§2.3) | MUST NOT | 2.3 - Window size for overlapping requests | **positive:** `unit/verify` [`TestWinResponseReleasesSlot`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_window_test.go#L165). **negative:** `unit/verify` [`TestWinResponseReleasesSlot`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_window_test.go#L171) |
| `RFC7296-2.3-5` | This Notify message MUST NOT be sent in a response; the invalid request MUST NOT be acknowledged (§2.3) | MUST NOT | 2.3 - Window size for overlapping requests | **positive:** `unit/verify` [`TestImiNotificationCarriesTheFourOctetMessageID`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_invalidmsgid_test.go#L199). **positive:** `unit/verify` [`TestOsrOutOfWindowRequestIsNotAcknowledged`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_outstanding_test.go#L182). **negative:** `unit/verify` [`TestOsrOutOfWindowRequestIsNotAcknowledged`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_outstanding_test.go#L188) |
| `RFC7296-2.3-7` | The data associated with a SET_WINDOW_SIZE notification MUST be 4 octets long and contain the big endian representation of the number of messages the sender promises to keep (§2.3) | MUST | 2.3 - Window size for overlapping requests | **positive:** `unit/verify` [`TestNtfySetWindowSizeDataIsFourOctets`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_notify_test.go#L161). **positive:** `unit/verify` [`TestSwzPeerWindowSizeIsRead`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_setwindow_test.go#L97). **negative:** `unit/verify` [`TestNtfySetWindowSizeDataIsFourOctets`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_notify_test.go#L167). **negative:** `unit/verify` [`TestSwzMalformedWindowSizeIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_setwindow_test.go#L139). **negative:** `unit/verify` [`TestSwzPeerWindowSizeIsRead`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_setwindow_test.go#L104) |
| `RFC7296-2.3-8` | An IKE endpoint MUST be prepared to accept and process a request while it has a request outstanding in order to avoid a deadlock in this situation (§2.3) | MUST | 2.3 - Window size for overlapping requests | **positive:** `unit/verify` [`TestOsrOwnerLoopRetiresTheStrandedWindow`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_outstanding_test.go#L134). **positive:** `unit/verify` [`TestOsrRequestAcceptedWhileOursIsOutstanding`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_outstanding_test.go#L51). **negative:** `unit/verify` [`TestImiHeldWindowSuppressesTheNotification`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_invalidmsgid_test.go#L257). **negative:** `unit/verify` [`TestOsrOwnerLoopKeepsAForeignWindowHeld`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_outstanding_test.go#L270). **negative:** `unit/verify` [`TestOsrRequestAcceptedWhileOursIsOutstanding`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_outstanding_test.go#L58). **negative:** `unit/verify` [`TestOsrRetireOnlyFreesItsOwnWindow`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_outstanding_test.go#L353) |
| `RFC7296-2.3-9` | Sending this notification is OPTIONAL, and notifications of this type MUST be rate limited (§2.3) | MUST | 2.3 - Window size for overlapping requests | **positive:** `unit/verify` [`TestImiNotificationCarriesTheFourOctetMessageID`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_invalidmsgid_test.go#L192). **positive:** `unit/verify` [`TestImiRateLimitCapsTheNotification`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_invalidmsgid_test.go#L94). **negative:** `unit/verify` [`TestImiHeldWindowSuppressesTheNotification`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_invalidmsgid_test.go#L253). **negative:** `unit/verify` [`TestImiUnauthenticatedRequestDrawsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_invalidmsgid_test.go#L143) |
| `RFC7296-2.25-1` | When a peer receives a TEMPORARY_FAILURE notification, it MUST NOT immediately retry the operation; it MUST wait so that the sender may complete whatever operation caused the temporary condition (§2.25) | MUST NOT | 2.25 | **positive:** `unit/verify` [`TestMidTemporaryFailureDefersTheIKERekey`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_msgid_test.go#L382). **positive:** `unit/verify` [`TestMidTemporaryFailureDefersTheRetry`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_msgid_test.go#L268). **negative:** `unit/verify` [`TestMidTemporaryFailureDefersTheIKERekey`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_msgid_test.go#L386). **negative:** `unit/verify` [`TestMidTemporaryFailureDefersTheRetry`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_msgid_test.go#L276) |
| `RFC7296-2.8-5` | If an SA has expired or is about to expire and rekeying attempts using the mechanisms described here fail, an implementation MUST close the IKE SA and any associated Child SAs and then MAY start new ones (§2.8) | MUST | 2.8 | **positive:** `unit/verify` [`TestRkyExhaustedRekeyClosesTheChildSAs`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_rekey_test.go#L238). **negative:** `unit/verify` [`TestRkyExhaustedRekeyClosesTheChildSAs`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_rekey_test.go#L241) |
| `RFC7296-2.8-6` | After the new equivalent IKE SA is created, the initiator deletes the old IKE SA, and the Delete payload to delete itself MUST be the last request sent over the old IKE SA (§2.8) | MUST | 2.8 | **positive:** `unit/verify` [`TestRkyIKERekeyDeleteIsTheLastRequestOnTheOldSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_rekey_test.go#L303). **negative:** `unit/verify` [`TestRkyIKERekeyDeleteIsTheLastRequestOnTheOldSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_rekey_test.go#L306) |
| `RFC7296-2.8-7` | The responder to a CREATE_CHILD_SA MUST be prepared to accept messages on an SA before sending its response to the creation request, so there is no ambiguity for the initiator (§2.8) | MUST | 2.8 | **positive:** `unit/verify` [`TestRkyResponderInstallsTheNewChildBeforeItAnswers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_rekey_test.go#L412). **negative:** `unit/verify` [`TestRkyResponderInstallsTheNewChildBeforeItAnswers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_rekey_test.go#L415) |
| `RFC7296-2.8.1-1` | When there are two SAs eligible to receive packets, a node MUST accept incoming packets through either SA (§2.8.1) | MUST | 2.8.1 | **positive:** `unit/verify` [`TestRkyOldAndNewChildBothReceiveUntilThePeerDeletes`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_rekey_test.go#L525). **negative:** `unit/verify` [`TestRkyOldAndNewChildBothReceiveUntilThePeerDeletes`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_rekey_test.go#L528) |
| `RFC7296-2.18-2` | The new IKE SA MUST reset its message counters to 0 (§2.18) | MUST | 2.18 | **positive:** `unit/verify` [`TestRtxRekeyedIKESAResetsMessageCounters`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_retransmit_test.go#L506). **negative:** `unit/verify` [`TestRtxRekeyedIKESAResetsMessageCounters`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_retransmit_test.go#L509) |
| `RFC7296-2.18-3` | implementations MUST perform a new Diffie-Hellman exchange when rekeying the IKE SA. In other words, an initiator MUST NOT propose the value "NONE" for the Diffie-Hellman transform, and a responder MUST NOT accept such a proposal. (§2.18) | MUST NOT | 2.18 | **positive:** `unit/verify` [`TestPropDHNoneRefusedForIKESA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L313). **negative:** `unit/verify` [`TestPropDHNoneRefusedForIKESA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L309) |
| `RFC7296-3.16-1` | In a response message, this octet MUST be set to match the identifier in the corresponding request. (§3.16) | MUST | 3.16 - The EAP payload and the EAP message format | **positive:** `unit/verify` [`TestEapfmtResponseIdentifierMatchesRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc7296_eap_test.go#L20). **negative:** `unit/verify` [`TestEapfmtResponseIdentifierMatchesRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc7296_eap_test.go#L25) |
| `RFC7296-3.16-2` | MUST be four less than the Payload Length of the encapsulating payload. (§3.16) | MUST | 3.16 - The EAP payload and the EAP message format | **positive:** `unit/verify` [`TestEapfmtEAPLengthIsFourLessThanPayloadLength`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_eap_test.go#L47). **negative:** `unit/verify` [`TestEapfmtEAPLengthIsFourLessThanPayloadLength`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_eap_test.go#L52) |
| `RFC7296-3.16-3` | For other codes, the EAP message length MUST be four octets and the Type and Type_Data fields MUST NOT be present. (§3.16) | MUST | 3.16 - The EAP payload and the EAP message format | **positive:** `unit/verify` [`TestEapfmtSuccessAndFailureCarryNoTypeField`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc7296_eap_test.go#L84). **negative:** `unit/verify` [`TestEapfmtSuccessAndFailureCarryNoTypeField`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc7296_eap_test.go#L88) |
| `RFC7296-3.16-4` | In a Response (2) message, Type MUST either be Nak or match the type of the data requested (§3.16) | MUST | 3.16 - The EAP payload and the EAP message format | **positive:** `unit/verify` [`TestEapfmtPeerResponseTypeIsNakOrMatchesRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc7296_eap_test.go#L118). **negative:** `unit/verify` [`TestEapfmtPeerResponseTypeIsNakOrMatchesRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc7296_eap_test.go#L122) |
| `RFC7296-1.7-2` | This document adds the restriction in Section 2.13 that all pseudorandom functions (PRFs) used with IKEv2 MUST take variable- sized keys. (§1.7) | MUST | 1.7 - The change list against RFC 4306 | **positive:** `unit/verify` [`TestPRFTakesVariableSizedKeys`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_test.go#L16). **negative:** `unit/verify` [`TestPRFTakesVariableSizedKeys`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_test.go#L20) |
| `RFC7296-2-1` | All IKEv2 implementations MUST be able to send, receive, and process IKE messages that are up to 1280 octets long (§2) | MUST | 2 - IKE protocol details and variations | **positive:** `unit/verify` [`TestMessageHandles1280Octets`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L52). **negative:** `unit/verify` [`TestMessageHandles1280Octets`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L56) |
| `RFC7296-2.5-1` | The minor version number indicates new capabilities, and MUST be ignored by a node with a smaller minor version number, but used for informational purposes by the node with the larger minor version number (§2.5, §3.1) | MUST | 2.5 | **positive:** `unit/verify` [`TestMinorVersionIgnoredMajorIsNot`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L100). **negative:** `unit/verify` [`TestMinorVersionIgnoredMajorIsNot`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L104) |
| `RFC7296-2.5-2` | If an endpoint receives a message with a higher major version number, it MUST drop the message (§2.5, §3.1) | MUST | 2.5 | **positive:** `unit/verify` [`TestHigherMajorVersionDropped`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L116). **negative:** `unit/verify` [`TestHigherMajorVersionDropped`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L119) |
| `RFC7296-2.5-6` | Also, for forward compatibility, all fields marked RESERVED MUST be set to zero by an implementation running version 2.0 (§2.5, §3.2, §3.3.1, §3.3.2, §3.5, §3.8, §3.13, §3.15, §3.15.1) | MUST | 2.5 | **positive:** `unit/verify` [`TestConfigAttributeReservedBitSentAsZero`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_cp_test.go#L125). **positive:** `unit/verify` [`TestReservedFieldsSentAsZero`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L145). **negative:** `unit/verify` [`TestConfigAttributeReservedBitSentAsZero`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_cp_test.go#L130). **negative:** `unit/verify` [`TestReservedFieldsSentAsZero`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L151) |
| `RFC7296-2.5-7` | their content MUST be ignored by an implementation running version 2.0 (§2.5) | MUST | 2.5 | **positive:** `unit/verify` [`TestConfigAttributeReservedBitIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_cp_test.go#L38). **positive:** `unit/verify` [`TestReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L209). **negative:** `unit/verify` [`TestConfigAttributeReservedBitIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_cp_test.go#L48). **negative:** `unit/verify` [`TestReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L213) |
| `RFC7296-2.5-8` | Similarly, payload types that are not defined are reserved for future use; implementations of a version where they are undefined MUST skip over those payloads and ignore their contents. (§2.5) | MUST | 2.5 | **positive:** `unit/verify` [`TestInnerChainSkipsUndefinedType`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_innerchain_test.go#L141). **positive:** `unit/verify` [`TestUndefinedPayloadTypeSkipped`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L261). **negative:** `unit/verify` [`TestInnerChainSkipsUndefinedType`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_innerchain_test.go#L144). **negative:** `unit/verify` [`TestUndefinedPayloadTypeSkipped`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L265) |
| `RFC7296-2.5-9` | If the critical flag is set and the payload type is unrecognized, the message MUST be rejected (§2.5, §4) | MUST | 2.5 | **positive:** `unit/verify` [`TestCriticalUnrecognizedPayloadRejected`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L305). **positive:** `unit/verify` [`TestInnerChainRejectsCriticalUnrecognized`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_innerchain_test.go#L65). **negative:** `unit/verify` [`TestCriticalUnrecognizedPayloadRejected`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L308). **negative:** `unit/verify` [`TestInnerChainRejectsCriticalUnrecognized`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_innerchain_test.go#L69) |
| `RFC7296-2.5-11` | If the critical flag is not set and the payload type is unsupported, that payload MUST be ignored (§2.5) | MUST | 2.5 | **positive:** `unit/verify` [`TestInnerChainIgnoresNonCriticalUnsupported`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_innerchain_test.go#L95). **positive:** `unit/verify` [`TestNonCriticalUnsupportedPayloadIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L332). **negative:** `unit/verify` [`TestInnerChainIgnoresNonCriticalUnsupported`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_innerchain_test.go#L98). **negative:** `unit/verify` [`TestNonCriticalUnsupportedPayloadIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L335) |
| `RFC7296-2.5-13` | implementations MUST NOT reject as invalid a message with those payloads in any other order. (§2.5) | MUST NOT | 2.5 | **positive:** `unit/verify` [`TestPayloadOrderNotRejected`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L369). **positive:** `unit/verify` [`TestPodAuthResponseAcceptsAuthBeforeIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_payload_order_test.go#L97). **negative:** `unit/verify` [`TestPayloadOrderNotRejected`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L372). **negative:** `unit/verify` [`TestPodAuthResponseAcceptsAuthBeforeIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_payload_order_test.go#L99) |
| `RFC7296-2.5-14` | If an endpoint supports major version n, and major version m, it MUST support all versions between n and m (§2.5) | MUST | 2.5 | **positive:** `unit/verify` [`TestSupportedMajorVersionSetIsSingleton`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_version_test.go#L213). **negative:** `unit/verify` [`TestNATTDispatchAppliesTheSameVersionGate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_version_test.go#L245) |
| `RFC7296-2.5-15` | If it receives a message with a major version that it supports, it MUST respond with that version number (§2.5) | MUST | 2.5 | **positive:** `unit/verify` [`TestResponderEchoesTheSupportedMajorVersion`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_version_test.go#L256). **negative:** `unit/verify` [`TestResponderEchoesTheSupportedMajorVersion`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_version_test.go#L262) |
| `RFC7296-2.5-16` | If they mistakenly (perhaps through an active attacker sending error messages) negotiate to version n, then both will notice that the other side can support a higher version number, and they MUST break the connection and reconnect using version n+1 (§2.5) | MUST | 2.5 | **positive:** `unit/verify` [`TestSupportedMajorVersionSetIsSingleton`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_version_test.go#L218). **negative:** `unit/verify` [`TestNATTDispatchAppliesTheSameVersionGate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_version_test.go#L249) |
| `RFC7296-2.5-17` | Payloads sent in IKE response messages MUST NOT have the critical flag set (§2.5) | MUST NOT | 2.5 | **positive:** `unit/verify` [`TestResponsePayloadsAreNeverCritical`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_critical_bit_test.go#L183). **negative:** `unit/verify` [`TestResponsePayloadsAreNeverCritical`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_critical_bit_test.go#L189) |
| `RFC7296-2.5-18` | the response to the IKE request containing that payload MUST include a Notify payload UNSUPPORTED_CRITICAL_PAYLOAD, indicating an unsupported critical payload was included. In that Notify payload, the Notification Data contains the one-octet payload type. (§2.5) | MUST | 2.5 | **positive:** `unit/verify` [`TestCritUnknownCriticalPayloadNamesItsType`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_critpayload_test.go#L201). **negative:** `unit/verify` [`TestCritUnknownCriticalPayloadNamesItsType`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_critpayload_test.go#L206) |
| `RFC7296-2.21.2-1` | Note, however, that request messages that contain an unsupported critical payload, or where the whole message is malformed (rather than just bad payload contents), MUST be rejected in their entirety, and MUST only lead to an UNSUPPORTED_CRITICAL_PAYLOAD or INVALID_SYNTAX Notification sent as a response. (§2.21.2) | MUST | 2.21.2 | **positive:** `unit/verify` [`TestCritChainReportsTruncationButNotBadContents`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_critpayload_test.go#L278). **positive:** `unit/verify` [`TestErrInnerParseFailureDrawsInvalidSyntaxAndOuterDrawsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L160). **negative:** `unit/verify` [`TestCritChainReportsTruncationButNotBadContents`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_critpayload_test.go#L284) |
| `RFC7296-2.21.2-2` | Specifically, a responder may include all the payloads associated with authentication (IDr, CERT, and AUTH) while sending error notifications for the piggybacked exchanges (FAILED_CP_REQUIRED, NO_PROPOSAL_CHOSEN, and so on), and the initiator MUST NOT fail the authentication because of this. (§2.21.2) | MUST NOT | 2.21.2 | **positive:** `unit/verify` [`TestErrInitiatorSurvivesPiggybackedErrorNotify`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L396). **negative:** `unit/verify` [`TestErrInitiatorSurvivesPiggybackedErrorNotify`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L406) |
| `RFC7296-2.21.2-3` | Extension documents may define new error notifications with these semantics, but MUST NOT use them unless the peer has been shown to understand them, such as by using the Vendor ID payload (§2.21.2) | MUST NOT | 2.21.2 | **positive:** `unit/verify` [`TestMobikeAuthExtensionErrorsRequirePeerUnderstanding`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L139). **negative:** `unit/verify` [`TestMobikeAuthExtensionErrorsRequirePeerUnderstanding`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L141) |
| `RFC7296-2.21.3-1` | After the IKE SA is authenticated, all requests having errors MUST result in a response notifying the other end of the error (§2.21.3) | MUST | 2.21.3 | **positive:** `unit/verify` [`TestDelMalformedSPISizeDrawsInvalidSyntax`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_delete_test.go#L220). **positive:** `unit/verify` [`TestErrNewChildRequestIsAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L120). **positive:** `unit/verify` [`TestErrRefusedChildRekeyIsAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L23). **positive:** `unit/verify` [`TestErrRefusedIKERekeyIsAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L586). **negative:** `unit/verify` [`TestErrNewChildRequestIsAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L124). **negative:** `unit/verify` [`TestErrRefusedChildRekeyIsAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L30). **negative:** `unit/verify` [`TestErrRefusedIKERekeyIsAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L591) |
| `RFC7296-2.21.4-1` | If the message is marked as a response, the node can audit the suspicious event but MUST NOT respond (§2.21.4) | MUST NOT | 2.21.4 | **positive:** `unit/verify` [`TestNtfOutOfSAIgnoresResponses`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/notify_error_test.go#L162). **positive:** `unit/verify` [`TestNtfOutOfSASkipsSAInitAndRateLimits`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/notify_error_test.go#L180). **negative:** `unit/verify` [`TestNtfOutOfSAIgnoresResponses`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/notify_error_test.go#L165). **negative:** `unit/verify` [`TestNtfOutOfSASkipsSAInitAndRateLimits`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/notify_error_test.go#L184). **positive:** `functional/verify` [`ipsec-error-notify-no-loop.ci`](https://github.com/ze-software/ze/blob/main/test/ipsec/ipsec-error-notify-no-loop.ci#L21). **negative:** `functional/verify` [`ipsec-error-notify-no-loop.ci`](https://github.com/ze-software/ze/blob/main/test/ipsec/ipsec-error-notify-no-loop.ci#L24) |
| `RFC7296-2.21.4-2` | If a response is sent, the response MUST be sent to the IP address and port from where it came with the same IKE SPIs and the Message ID copied. (§2.21.4) | MUST | 2.21.4 | **positive:** `unit/verify` [`TestNtfOutOfSAAnswerCarriesTheSocketFraming`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/notify_error_test.go#L309). **positive:** `unit/verify` [`TestNtfOutOfSAAnswersWithInvalidIKESPI`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/notify_error_test.go#L75). **negative:** `unit/verify` [`TestNtfOutOfSAAnswerCarriesTheSocketFraming`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/notify_error_test.go#L313). **negative:** `unit/verify` [`TestNtfOutOfSAAnswersWithInvalidIKESPI`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/notify_error_test.go#L79). **positive:** `functional/verify` [`ipsec-error-notify-no-loop.ci`](https://github.com/ze-software/ze/blob/main/test/ipsec/ipsec-error-notify-no-loop.ci#L26). **negative:** `functional/verify` [`ipsec-error-notify-no-loop.ci`](https://github.com/ze-software/ze/blob/main/test/ipsec/ipsec-error-notify-no-loop.ci#L28) |
| `RFC7296-2.21.4-3` | The response MUST NOT be cryptographically protected (§2.21.4) | MUST NOT | 2.21.4 | **positive:** `unit/verify` [`TestNtfOutOfSAAnswerIsUnprotected`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/notify_error_test.go#L232). **negative:** `unit/verify` [`TestNtfOutOfSAAnswerIsUnprotected`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/notify_error_test.go#L238) |
| `RFC7296-2.21.4-4` | MUST contain an INVALID_IKE_SPI Notify payload. (§2.21.4) | MUST | 2.21.4 | **positive:** `unit/verify` [`TestNtfOutOfSAAnswersWithInvalidIKESPI`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/notify_error_test.go#L82). **negative:** `unit/verify` [`TestNtfOutOfSAAnswersWithInvalidIKESPI`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/notify_error_test.go#L84). **positive:** `functional/verify` [`ipsec-error-notify-no-loop.ci`](https://github.com/ze-software/ze/blob/main/test/ipsec/ipsec-error-notify-no-loop.ci#L30). **negative:** `functional/verify` [`ipsec-error-notify-no-loop.ci`](https://github.com/ze-software/ze/blob/main/test/ipsec/ipsec-error-notify-no-loop.ci#L32) |
| `RFC7296-2.21.4-5` | A peer receiving such an unprotected Notify payload MUST NOT respond (§2.21.4) | MUST NOT | 2.21.4 | **positive:** `unit/verify` [`TestNtfEmitterIsAFixedPoint`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/notify_error_test.go#L282). **negative:** `unit/verify` [`TestNtfEmitterIsAFixedPoint`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/notify_error_test.go#L287). **positive:** `functional/verify` [`ipsec-error-notify-no-loop.ci`](https://github.com/ze-software/ze/blob/main/test/ipsec/ipsec-error-notify-no-loop.ci#L34). **negative:** `functional/verify` [`ipsec-error-notify-no-loop.ci`](https://github.com/ze-software/ze/blob/main/test/ipsec/ipsec-error-notify-no-loop.ci#L37) |
| `RFC7296-2.21.4-6` | MUST NOT change the state of any existing SAs. (§2.21.4) | MUST NOT | 2.21.4 | **positive:** `unit/verify` [`TestErrUnprotectedNotifyChangesNoState`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L285). **negative:** `unit/verify` [`TestErrUnprotectedNotifyChangesNoState`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L291) |
| `RFC7296-2.21.4-7` | A node receiving a suspicious message from an IP address (and port, if NAT traversal is used) with which it has an IKE SA SHOULD send an IKE Notify payload in an IKE INFORMATIONAL exchange over that SA. The recipient MUST NOT change the state of any SAs as a result, but may wish to audit the event to aid in diagnosing malfunctions. (§2.21.4) | MUST NOT | 2.21.4 | **positive:** `unit/verify` [`TestErrProtectedInformationalNotifyChangesNoState`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L341). **negative:** `unit/verify` [`TestErrProtectedInformationalNotifyChangesNoState`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L348) |
| `RFC7296-3.10.1-1` | Types in the range 0 - 16383 are intended for reporting errors. An implementation receiving a Notify payload with one of these types that it does not recognize in a response MUST assume that the corresponding request has failed entirely. (§3.10.1) | MUST | 3.10.1 | **positive:** `unit/verify` [`TestErrUnrecognizedNotifyHandling`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L435). **negative:** `unit/verify` [`TestErrUnrecognizedNotifyHandling`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L439) |
| `RFC7296-3.10.1-2` | Unrecognized error types in a request and status types in a request or response MUST be ignored, and they should be logged (§3.10.1) | MUST | 3.10.1 | **positive:** `unit/verify` [`TestCritNotifyTypeClassification`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_critpayload_test.go#L379). **positive:** `unit/verify` [`TestErrUnrecognizedNotifyHandling`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L442). **negative:** `unit/verify` [`TestCritNotifyTypeClassification`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_critpayload_test.go#L383). **negative:** `unit/verify` [`TestErrUnrecognizedNotifyHandling`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L446) |
| `RFC7296-3.10.1-3` | To avoid leaking information to someone probing a node, this status MUST be sent in response to any error not covered by one of the other status types. (§3.10.1) | MUST | 3.10.1 | **positive:** `unit/verify` [`TestErrInnerParseFailureDrawsInvalidSyntaxAndOuterDrawsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L163). **negative:** `unit/verify` [`TestErrInnerParseFailureDrawsInvalidSyntaxAndOuterDrawsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L166) |
| `RFC7296-3.10.1-4` | Notify payloads with status types MAY be added to any message and MUST be ignored if not recognized (§3.10.1) | MAY | 3.10.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7296-2.6-2` | Each endpoint chooses one of the two SPIs and MUST choose them so as to be unique identifiers of an IKE SA (§2.6) | MUST | 2.6 | **positive:** `unit/verify` [`TestSPIsAreUniqueIdentifiers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L285). **negative:** `unit/verify` [`TestSPIsAreUniqueIdentifiers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L288) |
| `RFC7296-2.6-3` | The data associated with this notification MUST be between 1 and 64 octets in length (inclusive) (§2.6) | MUST | 2.6 | **positive:** `unit/verify` [`TestCkeMintedCookieIsWithinTheLengthBound`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cookie_test.go#L58). **negative:** `unit/verify` [`TestCkeEchoedCookieIsBoundedToo`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cookie_test.go#L101) |
| `RFC7296-2.6-4` | If the IKE_SA_INIT response includes the COOKIE notification, the initiator MUST then retry the IKE_SA_INIT request, and include the COOKIE notification containing the received data as the first payload, and all other payloads unchanged (§2.6) | MUST | 2.6 | **positive:** `unit/verify` [`TestCkeRetryCarriesCookieFirstAndNothingElseChanged`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cookie_test.go#L140). **negative:** `unit/verify` [`TestCkeCookieIsAbsentWithoutAChallenge`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cookie_test.go#L204) |
| `RFC7296-2.6-5` | When one party receives an IKE_SA_INIT request containing a cookie whose contents do not match the value expected, that party MUST ignore the cookie and process the message as if no cookie had been included; usually this means sending a response containing a new cookie (§2.6) | MUST | 2.6 | **positive:** `unit/verify` [`TestCkeMismatchedCookieIsIgnoredNotRejected`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cookie_test.go#L223). **negative:** `unit/verify` [`TestCkeValidCookieReachesTheHandshake`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cookie_test.go#L258) |
| `RFC7296-2.6.1-1` | Implementations SHOULD support this shorter exchange, but MUST NOT fail if other implementations do not support this shorter exchange (§2.6.1) | MUST NOT | 2.6.1 | **positive:** `unit/verify` [`TestCkeSecondCookieReplacesTheFirstWithoutFailing`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cookie_test.go#L304). **negative:** `unit/verify` [`TestCkeCookieAndInvalidKECombineWithoutFailing`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cookie_test.go#L350) |
| `RFC7296-2.10-2` | MUST be at least 128 bits in size (§2.10) | MUST | 2.10 | **positive:** `unit/verify` [`TestNonceLengthBounds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L609). **negative:** `unit/verify` [`TestNonceLengthBounds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L611) |
| `RFC7296-2.10-3` | MUST be at least half the key size of the negotiated pseudorandom function (PRF) (§2.10) | MUST | 2.10 | **positive:** `unit/verify` [`TestNonceMeetsHalfPRFKeySize`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_encrypt_test.go#L421). **negative:** `unit/verify` [`TestNonceMeetsHalfPRFKeySize`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_encrypt_test.go#L426) |
| `RFC7296-2.13-1` | For algorithms that accept a variable-length key, a fixed key size MUST be specified as part of the cryptographic transform negotiated (§2.13) | MUST | 2.13 | **positive:** `unit/verify` [`TestTransformRegistryStatesKeySizes`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_test.go#L60). **negative:** `unit/verify` [`TestTransformRegistryStatesKeySizes`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_test.go#L64) |
| `RFC7296-2.13-2` | For algorithms for which not all values are valid keys (such as DES or 3DES with key parity), the algorithm by which keys are derived from arbitrary values MUST be specified by the cryptographic transform. (§2.13) | MUST | 2.13 | **positive:** `unit/verify` [`TestSKKeyLengthsComeFromTransforms`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_test.go#L113). **negative:** `unit/verify` [`TestSKKeyLengthsComeFromTransforms`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_test.go#L117) |
| `RFC7296-2.13-3` | The preferred key size MUST be used as the length of SK_d, SK_pi, and SK_pr (§2.13, §2.14) | MUST | 2.13 | **positive:** `unit/verify` [`TestSKKeyLengthsComeFromTransforms`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_test.go#L120). **negative:** `unit/verify` [`TestSKKeyLengthsComeFromTransforms`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_test.go#L123) |
| `RFC7296-2.13-4` | Other types of PRFs MUST specify their preferred key size (§2.13) | MUST | 2.13 | **positive:** `unit/verify` [`TestTransformRegistryStatesKeySizes`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_test.go#L67). **negative:** `unit/verify` [`TestTransformRegistryStatesKeySizes`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_test.go#L70) |
| `RFC7296-2.15-1` | The management interface by which the shared secret is provided MUST accept ASCII strings of at least 64 octets (§2.15) | MUST | 2.15 | **positive:** `unit/verify` [`TestPSKAcceptsAtLeast64ASCIIOctets`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc7296_test.go#L34). **negative:** `unit/verify` [`TestPSKAcceptsAtLeast64ASCIIOctets`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc7296_test.go#L39) |
| `RFC7296-2.15-2` | MUST NOT add a null terminator before using them as shared secrets (§2.15) | MUST NOT | 2.15 | **positive:** `unit/verify` [`TestPSKHasNoNullTerminatorAdded`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc7296_test.go#L81). **negative:** `unit/verify` [`TestPSKHasNoNullTerminatorAdded`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc7296_test.go#L86) |
| `RFC7296-2.17-1` | keying material for each Child SA MUST be taken from the expanded KEYMAT using the following rules: o All keys for SAs carrying data from the initiator to the responder are taken before SAs going from the responder to the initiator. (§2.17) | MUST | 2.17 | **positive:** `unit/verify` [`TestChildSAKeymatOrder`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_test.go#L226). **negative:** `unit/verify` [`TestChildSAKeymatOrder`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_test.go#L234) |
| `RFC7296-2.17-2` | For ESP and AH, [IPSECARCH] defines the order, namely: the encryption key (if any) MUST be taken from the first bits and the integrity key (if any) MUST be taken from the remaining bits. (§2.17) | MUST | 2.17 | **positive:** `unit/verify` [`TestChildSAKeymatOrder`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_test.go#L230). **negative:** `unit/verify` [`TestChildSAKeymatOrder`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_test.go#L238) |
| `RFC7296-3.1-1` | An Encrypted payload MUST be the last payload in a packet (§3.1, §3.14) | MUST | 3.1 | **positive:** `unit/verify` [`TestSKIsLastAndNeverNested`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_encrypt_test.go#L87). **negative:** `unit/verify` [`TestSKIsLastAndNeverNested`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_encrypt_test.go#L95) |
| `RFC7296-3.1-2` | an Encrypted payload MUST NOT contain another Encrypted payload (§3.1) | MUST NOT | 3.1 | **positive:** `unit/verify` [`TestSKIsLastAndNeverNested`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_encrypt_test.go#L91). **negative:** `unit/verify` [`TestSKIsLastAndNeverNested`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_encrypt_test.go#L98) |
| `RFC7296-3.1-3` | Initiator's SPI (8 octets) - A value chosen by the initiator to identify a unique IKE Security Association. This value MUST NOT be zero. (§3.1) | MUST NOT | 3.1 | **positive:** `unit/verify` [`TestSPIZeroRules`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L201). **negative:** `unit/verify` [`TestSPIZeroRules`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L209) |
| `RFC7296-3.1-4` | Responder's SPI (8 octets) - A value chosen by the responder to identify a unique IKE Security Association. This value MUST be zero in the first message of an IKE initial exchange (including repeats of that message including a cookie). (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestSPIZeroRules`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L204). **negative:** `unit/verify` [`TestSPIZeroRules`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L212) |
| `RFC7296-3.1-5` | Implementations based on this version of IKE MUST set the major version to 2 (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestBuiltMessagesCarryVersion2Point0`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L79). **negative:** `unit/verify` [`TestBuiltMessagesCarryVersion2Point0`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L87) |
| `RFC7296-3.1-6` | Implementations based on this version of IKE MUST set the minor version to 0 (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestBuiltMessagesCarryVersion2Point0`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L84). **negative:** `unit/verify` [`TestBuiltMessagesCarryVersion2Point0`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L90) |
| `RFC7296-3.1-7` | 'X' bits MUST be cleared when sending (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestBuiltMessagesClearXAndVBits`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L328). **negative:** `unit/verify` [`TestBuiltMessagesClearXAndVBits`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L335) |
| `RFC7296-3.1-8` | MUST be ignored on receipt (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestXBitsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L420). **negative:** `unit/verify` [`TestXBitsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L428) |
| `RFC7296-3.1-9` | This bit MUST be cleared in all request messages and MUST be set in all responses. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestResponseBitMatchesDirection`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L373). **negative:** `unit/verify` [`TestResponseBitMatchesDirection`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L377) |
| `RFC7296-3.1-11` | Implementations of IKEv2 MUST clear this bit when sending and MUST ignore it in incoming messages. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestBuiltMessagesClearXAndVBits`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L332). **negative:** `unit/verify` [`TestBuiltMessagesClearXAndVBits`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L337). **negative:** `unit/verify` [`TestXBitsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L424) |
| `RFC7296-3.1-12` | An IKE endpoint MUST NOT generate a response to a message that is marked as being a response (with one exception; see Section 2.21.2) (§3.1) | MUST NOT | 3.1 | **positive:** `unit/verify` [`TestNrsInformationalHandlerRefusesAResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_noresponse_test.go#L252). **positive:** `unit/verify` [`TestNrsResponseNeverDrawsAResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_noresponse_test.go#L46). **negative:** `unit/verify` [`TestNrsInformationalHandlerRefusesAResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_noresponse_test.go#L257). **negative:** `unit/verify` [`TestNrsResponseNeverDrawsAResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_noresponse_test.go#L56) |
| `RFC7296-3.2-2` | MUST be ignored by the recipient if the recipient understands the payload type code. (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestCriticalBitIgnoredForKnownType`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L414). **positive:** `unit/verify` [`TestInnerChainIgnoresCriticalOnKnownType`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_innerchain_test.go#L174). **negative:** `unit/verify` [`TestCriticalBitIgnoredForKnownType`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L417). **negative:** `unit/verify` [`TestInnerChainIgnoresCriticalOnKnownType`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_innerchain_test.go#L177) |
| `RFC7296-3.2-3` | all implementations MUST understand all payload types defined in this document (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestAllDefinedPayloadTypesUnderstood`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L454). **negative:** `unit/verify` [`TestAllDefinedPayloadTypesUnderstood`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L458) |
| `RFC7296-3.2-4` | MUST be set to zero for payload types defined in this document. (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestDefinedPayloadTypesAreSentUncritical`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_critical_bit_test.go#L112). **positive:** `unit/verify` [`TestEngineSourceNeverSetsTheCriticalField`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_critical_bit_test.go#L385). **negative:** `unit/verify` [`TestDefinedPayloadTypesAreSentUncritical`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_critical_bit_test.go#L119) |
| `RFC7296-3.2-5` | o Critical (1 bit) - MUST be set to zero if the sender wants the recipient to skip this payload if it does not understand the payload type code in the Next Payload field of the previous payload. (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestCritSenderZeroBitRequestsSkip`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_critpayload_test.go#L101). **negative:** `unit/verify` [`TestCritSenderZeroBitRequestsSkip`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_critpayload_test.go#L106) |
| `RFC7296-3.2-6` | MUST be set to one if the sender wants the recipient to reject this entire message if it does not understand the payload type. (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestCritSenderOneBitRequestsWholeMessageRejection`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_critpayload_test.go#L155). **negative:** `unit/verify` [`TestCritSenderOneBitRequestsWholeMessageRejection`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_critpayload_test.go#L159) |
| `RFC7296-3.3-3` | An SA payload MAY contain multiple proposals. If there is more than one, they MUST be ordered from most preferred to least preferred (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestProposalOrderPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L510). **negative:** `unit/verify` [`TestProposalOrderPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L514) |
| `RFC7296-3.3-4` | When parsing an SA, an implementation MUST check that the total Payload Length is consistent with the payload's internal lengths and counts (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestSAInternalLengthConsistency`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L559). **negative:** `unit/verify` [`TestSAInternalLengthConsistency`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L563) |
| `RFC7296-3.3-5` | Each structure MUST have a proposal number one (1) greater than the previous structure. The first Proposal in the initiator's SA payload MUST have a Proposal Num of one (1) (§3.3, §3.3.1) | MUST | 3.3 | **positive:** `unit/verify` [`TestPropProposalNumbering`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_sa_test.go#L303). **negative:** `unit/verify` [`TestPropProposalNumbering`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_sa_test.go#L307) |
| `RFC7296-3.3-6` | A transform MUST NOT have multiple attributes of the same type (§3.3) | MUST NOT | 3.3 | **positive:** `unit/verify` [`TestPropDuplicateAttributeRejected`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_sa_test.go#L80). **negative:** `unit/verify` [`TestPropDuplicateAttributeRejected`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_sa_test.go#L76) |
| `RFC7296-3.3-7` | To propose alternate values for an attribute (for example, multiple key sizes for the AES encryption algorithm), an implementation MUST include multiple transforms with the same Transform Type each with a single Attribute. (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestPropAlternateKeyLengthsUseSeparateTransforms`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_sa_test.go#L98). **negative:** `unit/verify` [`TestPropAlternateKeyLengthsUseSeparateTransforms`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_sa_test.go#L102) |
| `RFC7296-3.3.1-1` | When a proposal is accepted, the proposal number in the SA payload MUST match the number on the proposal sent that was accepted (§3.3.1) | MUST | 3.3.1 | **positive:** `unit/verify` [`TestPropAcceptedProposalNumberMatchesOffer`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L43). **negative:** `unit/verify` [`TestPropAcceptedProposalNumberMatchesOffer`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L47) |
| `RFC7296-3.3.1-2` | o SPI Size (1 octet) - For an initial IKE SA negotiation, this field MUST be zero; the SPI is obtained from the outer header. (§3.3.1) | MUST | 3.3.1 | **positive:** `unit/verify` [`TestPropSPISizeMatchesProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_sa_test.go#L250). **positive:** `unit/verify` [`TestSpzInitialIKESANegotiationNeedsZeroSPISize`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_spisize_test.go#L49). **negative:** `unit/verify` [`TestPropSPISizeMatchesProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_sa_test.go#L254). **negative:** `unit/verify` [`TestSpzInitialIKESANegotiationNeedsZeroSPISize`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_spisize_test.go#L45) |
| `RFC7296-3.3.3-1` | A compliant implementation MUST understand all mandatory and optional types for each protocol it supports (§3.3.3) | MUST | 3.3.3 | **positive:** `unit/verify` [`TestPropTransformTypesUnderstoodPerProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L70). **positive:** `unit/verify` [`TestTftUnknownTransformTypeMakesProposalUnacceptable`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transformtype_test.go#L40). **negative:** `unit/verify` [`TestPropTransformTypesUnderstoodPerProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L76). **negative:** `unit/verify` [`TestTftUnknownTransformTypeMakesProposalUnacceptable`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transformtype_test.go#L37) |
| `RFC7296-3.3.4-2` | Upon receipt of a payload with a set of Transform IDs, the implementation MUST compare the transmitted Transform IDs against those locally configured via the management controls, to verify that the proposed suite is acceptable based on local policy (§3.3.4) | MUST | 3.3.4 | **positive:** `unit/verify` [`TestPropTransformIDsComparedAgainstLocalPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L119). **negative:** `unit/verify` [`TestPropTransformIDsComparedAgainstLocalPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L125) |
| `RFC7296-3.3.4-3` | The implementation MUST reject SA proposals that are not authorized by these IKE suite controls (§3.3.4) | MUST | 3.3.4 | **positive:** `unit/verify` [`TestPropUnauthorizedProposalRejected`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L157). **negative:** `unit/verify` [`TestPropUnauthorizedProposalRejected`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L161) |
| `RFC7296-3.3.5-1` | Attributes described as fixed length MUST NOT be encoded using the variable-length encoding unless that length exceeds two bytes (§3.3.5) | MUST NOT | 3.3.5 | **positive:** `unit/verify` [`TestPropFixedLengthAttributeRejectsTLVEncoding`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_sa_test.go#L136). **negative:** `unit/verify` [`TestPropFixedLengthAttributeRejectsTLVEncoding`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_sa_test.go#L132) |
| `RFC7296-3.3.5-2` | Variable-length attributes MUST NOT be encoded as fixed-length even if their value can fit into two octets (§3.3.5) | MUST NOT | 3.3.5 | **positive:** `unit/verify` [`TestPropVariableLengthAttributeRejectsTVEncoding`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_sa_test.go#L161). **negative:** `unit/verify` [`TestPropVariableLengthAttributeRejectsTVEncoding`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_sa_test.go#L156) |
| `RFC7296-3.3.5-3` | The Key Length attribute specifies the key length in bits (MUST use network byte order) (§3.3.5) | MUST | 3.3.5 | **positive:** `unit/verify` [`TestPropKeyLengthUsesNetworkByteOrder`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_sa_test.go#L184). **negative:** `unit/verify` [`TestPropKeyLengthUsesNetworkByteOrder`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_sa_test.go#L187) |
| `RFC7296-3.3.5-4` | The Key Length attribute MUST NOT be used with transforms that use a fixed-length key (§3.3.5) | MUST NOT | 3.3.5 | **positive:** `unit/verify` [`TestPropKeyLengthRejectedOnFixedKeyTransform`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_sa_test.go#L223). **negative:** `unit/verify` [`TestPropKeyLengthRejectedOnFixedKeyTransform`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_sa_test.go#L219) |
| `RFC7296-3.3.5-5` | o Some transforms specify that the Key Length attribute MUST be always included (omitting the attribute is not allowed, and proposals not containing it MUST be rejected). (§3.3.5) | MUST | 3.3.5 | **positive:** `unit/verify` [`TestPropKeyLengthRequiredTransformRejectedWithoutIt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L193). **negative:** `unit/verify` [`TestPropKeyLengthRequiredTransformRejectedWithoutIt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L188) |
| `RFC7296-3.3.6-4` | If the responder receives a proposal that contains a Transform Type it does not understand, or a proposal that is missing a mandatory Transform Type, it MUST consider this proposal unacceptable; however, other proposals in the same SA payload are processed as usual (§3.3.6) | MUST | 3.3.6 | **positive:** `unit/verify` [`TestPropProposalMissingMandatoryTransformTypeUnacceptable`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L253). **positive:** `unit/verify` [`TestTftForeignTransformTypeRefusedInESPOffer`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transformtype_test.go#L80). **positive:** `unit/verify` [`TestTftUnknownTransformTypeMakesProposalUnacceptable`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transformtype_test.go#L34). **negative:** `unit/verify` [`TestPropProposalMissingMandatoryTransformTypeUnacceptable`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L247). **negative:** `unit/verify` [`TestTftForeignTransformTypeRefusedInESPOffer`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transformtype_test.go#L77). **negative:** `unit/verify` [`TestTftUnknownTransformTypeMakesProposalUnacceptable`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transformtype_test.go#L29) |
| `RFC7296-3.3.6-5` | Similarly, if the responder receives a transform that it does not understand, or one that contains a Transform Attribute it does not understand, it MUST consider this transform unacceptable; other transforms with the same Transform Type are processed as usual. (§3.3.6) | MUST | 3.3.6 | **positive:** `unit/verify` [`TestAltDHAlternativesAreBothOffered`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transform_alternatives_test.go#L52). **positive:** `unit/verify` [`TestPropUnknownTransformIDMakesTransformUnacceptable`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L287). **negative:** `unit/verify` [`TestAltUnusableAlternativesAreStillRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transform_alternatives_test.go#L89). **negative:** `unit/verify` [`TestPropUnknownTransformIDMakesTransformUnacceptable`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L283) |
| `RFC7296-3.3.6-7` | Any attributes of a selected transform MUST be returned unmodified (§3.3.6) | MUST | 3.3.6 | **positive:** `unit/verify` [`TestPropSelectedTransformAttributesUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L214). **negative:** `unit/verify` [`TestPropSelectedTransformAttributesUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L218) |
| `RFC7296-3.9-1` | The size of the Nonce Data MUST be between 16 and 256 octets, inclusive (§3.9) | MUST | 3.9 | **positive:** `unit/verify` [`TestNonceLengthBounds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L604). **negative:** `unit/verify` [`TestNonceLengthBounds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L607) |
| `RFC7296-3.10-3` | For a notification concerning the IKE SA, the SPI Size MUST be zero and the field must be empty. (§3.10) | MUST | 3.10 | **positive:** `unit/verify` [`TestNotifyIKESAHasEmptySPI`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L633). **negative:** `unit/verify` [`TestNotifyIKESAHasEmptySPI`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L639) |
| `RFC7296-3.10-4` | For notifications concerning Child SAs, this field MUST contain either (2) to indicate AH or (3) to indicate ESP. (§3.10) | MUST | 3.10 | **positive:** `unit/verify` [`TestNtfyChildSAProtocolIDIsAHOrESP`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_notify_test.go#L9). **negative:** `unit/verify` [`TestNtfyChildSAProtocolIDIsAHOrESP`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_notify_test.go#L14) |
| `RFC7296-3.10-5` | If the SPI field is empty, this field MUST be sent as zero and MUST be ignored on receipt. (§3.10) | MUST | 3.10 | **positive:** `unit/verify` [`TestNtfyEmptySPIProtocolIDIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_notify_test.go#L113). **positive:** `unit/verify` [`TestNtfyEmptySPISendsProtocolIDZero`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_notify_test.go#L60). **negative:** `unit/verify` [`TestNtfyEmptySPIProtocolIDIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_notify_test.go#L117). **negative:** `unit/verify` [`TestNtfyEmptySPISendsProtocolIDZero`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_notify_test.go#L65) |
| `RFC7296-3.11-1` | each SPI MUST be for the same protocol. Mixing of protocol identifiers MUST NOT be performed in the Delete payload. (§3.11) | MUST NOT | 3.11 | **positive:** `unit/verify` [`TestDeletePayloadSingleProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L680). **negative:** `unit/verify` [`TestDeletePayloadSingleProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L684) |
| `RFC7296-3.11-2` | It MUST be zero for IKE (SPI is in message header) or four for AH and ESP. (§3.11) | MUST | 3.11 | **positive:** `unit/verify` [`TestDelWellFormedSPISizeIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_delete_test.go#L257). **positive:** `unit/verify` [`TestDeleteSPISizeByProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L729). **negative:** `unit/verify` [`TestDelMalformedSPISizeDrawsInvalidSyntax`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_delete_test.go#L217). **negative:** `unit/verify` [`TestDeleteSPISizeByProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L733) |
| `RFC7296-3.12-2` | Unfamiliar Vendor IDs MUST be ignored (§3.12) | MUST | 3.12 | **positive:** `unit/verify` [`TestVendorIDIgnoredButPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L766). **negative:** `unit/verify` [`TestVendorIDIgnoredButPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L774) |
| `RFC7296-3.12-3` | Writers of documents who wish to extend this protocol MUST define a Vendor ID payload to announce the ability to implement the extension in the document (§3.12) | MUST | 3.12 | **positive:** `unit/verify` [`TestVendorIDIgnoredButPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L770). **negative:** `unit/verify` [`TestVendorIDIgnoredButPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L776) |
| `RFC7296-3.12-4` | A Vendor ID payload MUST NOT change the interpretation of any information defined in this specification (i.e., the critical bit MUST be set to 0) (§3.12) | MUST NOT | 3.12 | **positive:** `unit/verify` [`TestVendorIDDoesNotChangeInterpretation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_critical_bit_test.go#L256). **negative:** `unit/verify` [`TestVendorIDDoesNotChangeInterpretation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_critical_bit_test.go#L265) |
| `RFC7296-3.14-2` | Senders MUST select a new unpredictable IV for every message (§3.14) | MUST | 3.14 | **positive:** `unit/verify` [`TestSKSelectsFreshIVPerMessage`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_encrypt_test.go#L149). **negative:** `unit/verify` [`TestSKSelectsFreshIVPerMessage`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_encrypt_test.go#L153) |
| `RFC7296-3.14-3` | recipients MUST accept any value (§3.14) | MUST | 3.14 | **positive:** `unit/verify` [`TestSKAcceptsAnyIVOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_encrypt_test.go#L194). **negative:** `unit/verify` [`TestSKAcceptsAnyIVOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_encrypt_test.go#L197) |
| `RFC7296-3.14-4` | Padding MAY contain any value chosen by the sender, and MUST have a length that makes the combination of the payloads, the Padding, and the Pad Length to be a multiple of the encryption block size (§3.14) | MUST | 3.14 | **positive:** `unit/verify` [`TestSKPaddingAlignsAndChecksumCoversCiphertext`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_encrypt_test.go#L250). **negative:** `unit/verify` [`TestSKPaddingAlignsAndChecksumCoversCiphertext`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_encrypt_test.go#L259) |
| `RFC7296-3.14-5` | the recipient MUST accept any length that results in proper alignment (§3.14) | MUST | 3.14 | **positive:** `unit/verify` [`TestSKAcceptsAnyAligningPadLength`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_encrypt_test.go#L333). **negative:** `unit/verify` [`TestSKAcceptsAnyAligningPadLength`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_encrypt_test.go#L337) |
| `RFC7296-3.14-6` | The checksum MUST be computed over the encrypted message (§3.14) | MUST | 3.14 | **positive:** `unit/verify` [`TestSKPaddingAlignsAndChecksumCoversCiphertext`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_encrypt_test.go#L254). **negative:** `unit/verify` [`TestSKPaddingAlignsAndChecksumCoversCiphertext`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_encrypt_test.go#L262) |
| `RFC7296-3.14-7` | Peers MUST NOT negotiate transforms for which no such specification exists (§3.14) | MUST NOT | 3.14 | **positive:** `unit/verify` [`TestPropUnspecifiedTransformRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L336). **negative:** `unit/verify` [`TestPropUnspecifiedTransformRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L332) |
| `RFC7296-5-2` | Though the security of negotiated Child SAs does not depend on the strength of the encryption and integrity protection negotiated in the IKE SA, implementations MUST NOT negotiate NONE as the IKE integrity protection algorithm or ENCR_NULL as the IKE encryption algorithm. (§5) | MUST NOT | 5 - Security considerations | **positive:** `unit/verify` [`TestIKENeverNegotiatesNullAlgorithms`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_test.go#L307). **negative:** `unit/verify` [`TestIKENeverNegotiatesNullAlgorithms`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_test.go#L311) |
| `RFC7296-5-3` | For this reason, a PRF whose output is less than 128 bits (e.g., 3DES-CBC) MUST NOT be used with this protocol. (§5) | MUST NOT | 5 - Security considerations | **positive:** `unit/verify` [`TestPrfFloorRefusesOutputBelow128Bits`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_prffloor_test.go#L18). **positive:** `unit/verify` [`TestPropPRFOutputBelow128BitsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L366). **negative:** `unit/verify` [`TestPrfFloorRefusesOutputBelow128Bits`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_prffloor_test.go#L14). **negative:** `unit/verify` [`TestPropPRFOutputBelow128BitsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L361) |
| `RFC7296-2.19-1` | Since the IKE_AUTH exchange creates an IKE SA and a Child SA, the IRAC MUST request the IRAS-controlled address (and optionally other information concerning the protected network) in the IKE_AUTH exchange (§2.19, §4) | MUST | 2.19 | **positive:** `unit/verify` [`TestZeSendsNoConfigurationRequest`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cp_test.go#L64). **negative:** `unit/verify` [`TestZeSendsNoConfigurationRequest`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cp_test.go#L75) |
| `RFC7296-2.19-4` | CP(CFG_REQUEST) MUST contain at least an INTERNAL_ADDRESS attribute (either IPv4 or IPv6) but MAY contain any number of additional attributes the initiator wants returned in the response (§2.19) | MUST | 2.19 | **positive:** `unit/verify` [`TestZeSendsNoConfigurationRequest`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cp_test.go#L80). **negative:** `unit/verify` [`TestZeSendsNoConfigurationRequest`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cp_test.go#L89) |
| `RFC7296-2.20-1` | An IKE implementation MAY decline to give out version information prior to authentication or even after authentication in case some implementation is known to have some security weakness. In that case, it MUST either return an empty string or no CP payload if CP is not supported. (§2.20) | MUST | 2.20 | **positive:** `unit/verify` [`TestZeDeclinesApplicationVersion`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cp_test.go#L127). **negative:** `unit/verify` [`TestZeDeclinesApplicationVersion`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cp_test.go#L134) |
| `RFC7296-3.15.1-2` | An empty INTERNAL_IP4_NETMASK attribute can be included in a CFG_REQUEST to request this information (although the gateway can send the information even when not requested). Non-empty values for this attribute in a CFG_REQUEST do not make sense and thus MUST NOT be included. (§3.15.1) | MUST NOT | 3.15.1 | **positive:** `unit/verify` [`TestZeSendsNoConfigRequestNetmask`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cp_test.go#L171). **negative:** `unit/verify` [`TestZeSendsNoConfigRequestNetmask`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cp_test.go#L177) |
| `RFC7296-3.15.1-5` | The responder MUST return a Configuration payload if it accepted any of the configuration data, and the Configuration payload MUST contain the attributes that the responder accepted with zero-length data (§3.15.1) | MUST | 3.15.1 | **positive:** `unit/verify` [`TestCFGSetIsIgnoredAndDrawsNoCFGACK`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cp_test.go#L259). **negative:** `unit/verify` [`TestCFGSetIsIgnoredAndDrawsNoCFGACK`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cp_test.go#L267) |
| `RFC7296-3.15.1-6` | Those attributes that it did not accept MUST NOT be in the CFG_ACK Configuration payload (§3.15.1) | MUST NOT | 3.15.1 | **positive:** `unit/verify` [`TestCFGSetIsIgnoredAndDrawsNoCFGACK`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cp_test.go#L272). **negative:** `unit/verify` [`TestCFGSetIsIgnoredAndDrawsNoCFGACK`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cp_test.go#L276) |
| `RFC7296-3.15.1-7` | If no attributes were accepted, the responder MUST return either an empty CFG_ACK payload or a response message without a CFG_ACK payload (§3.15.1) | MUST | 3.15.1 | **positive:** `unit/verify` [`TestCFGSetIsIgnoredAndDrawsNoCFGACK`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cp_test.go#L280). **negative:** `unit/verify` [`TestCFGSetIsIgnoredAndDrawsNoCFGACK`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cp_test.go#L284) |
| `RFC7296-2.1-2` | they SHOULD be able to send, receive, and process messages that are up to 3000 octets long. (§2) | SHOULD | 2 - IKE protocol details and variations | **positive:** no positive test. **negative:** no negative test |
| `RFC7296-2.4-2` | If there has only been outgoing traffic on all of the SAs associated with an IKE SA, it is essential to confirm liveness of the other endpoint to avoid black holes.  If no cryptographically protected messages have been received on an IKE SA or any of its Child SAs recently, the system needs to perform a liveness check in order to prevent sending messages to a dead peer. (§2.4) | SHOULD | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7296-2.4-3` | Since IKE is designed to operate in spite of DoS attacks from the network, an endpoint MUST NOT conclude that the other endpoint has failed based on any routing information (e.g., ICMP messages) or IKE messages that arrive without cryptographic protection (e.g., Notify messages complaining about unknown SPIs). (§2.4) | MUST NOT | 2.4 | **positive:** `unit/verify` [`TestResponderAcceptsReinitAfterStaleSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/responder_test.go#L680). **positive:** `unit/verify` [`TestRteUnownedEstablishedSATrustsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_routing_test.go#L53). **negative:** `unit/verify` [`TestResponderKeepsOldSAOnUnauthenticatedInit`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/responder_test.go#L731). **negative:** `unit/verify` [`TestRteUnownedEstablishedSATrustsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_routing_test.go#L47) |
| `RFC7296-2.4-4` | The INITIAL_CONTACT notification, if sent, MUST be in the first IKE_AUTH request or response, not as a separate exchange afterwards (§2.4) | MUST | 2.4 | **positive:** `unit/verify` [`TestInitiatorEmitsInitialContact`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/responder_test.go#L829). **negative:** `unit/verify` [`TestInitialContactAbsentFromRekey`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_test.go#L330) |
| `RFC7296-2.8-3` | To reduce the probability of this happening, the timing of rekeying requests SHOULD be jittered (delayed by a random amount of time after the need for rekeying is noticed). (§2.8.1) | SHOULD | 2.8.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7296-3.4-2` | This Diffie-Hellman Group Num MUST match a Diffie-Hellman group specified in a proposal in the SA payload that is sent in the same message (§3.4) | MUST | 3.4 | **positive:** `unit/verify` [`TestKesaKEGroupOfferedIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_kesa_test.go#L38). **negative:** `unit/verify` [`TestKesaKEGroupNotOfferedIsRejected`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_kesa_test.go#L70). **negative:** `unit/verify` [`TestResInitiatorRejectsKEGroupOutsideTheAcceptedOffer`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_residual_test.go#L332) |
| `RFC7296-3.4-3` | If none of the proposals in that SA payload specifies a Diffie-Hellman group, the KE payload MUST NOT be present (§3.4) | MUST NOT | 3.4 | **positive:** `unit/verify` [`TestKesaAbsentKEIsAlwaysAllowed`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_kesa_test.go#L127). **negative:** `unit/verify` [`TestKesaKEWithoutDHProposalIsRejected`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_kesa_test.go#L100) |
| `RFC7296-3.3.6-8` | If one of the proposals offered is for the Diffie-Hellman group of NONE, and the responder selects that Diffie-Hellman group, then it MUST ignore the initiator's KE payload and omit the KE payload from the response (§3.3.6) | MUST | 3.3.6 | **positive:** `unit/verify` [`TestResSelectedDHGroupNoneOmitsKEFromTheResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_residual_test.go#L405). **negative:** `unit/verify` [`TestResIKESANeverSelectsDHGroupNone`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_residual_test.go#L364) |
| `RFC7296-2.4-14` | This notification MUST NOT be sent by an entity that may be replicated (§2.4) | MUST NOT | 2.4 | **positive:** `unit/verify` [`TestResInitialContactSentByNonReplicableIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_residual_test.go#L52). **negative:** `unit/verify` [`TestResInitialContactNotSentByReplicableIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_residual_test.go#L77) |
| `RFC7296-2.8-8` | When the lifetime of a Security Association expires, the Security Association MUST NOT be used (§2.8) | MUST NOT | 2.8 | **positive:** `unit/verify` [`TestResRekeyLeadLeavesRoomBeforeHardExpiry`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_residual_test.go#L169). **positive:** `unit/verify` [`TestResUnexpiredSAIsUsed`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_residual_test.go#L97). **negative:** `unit/verify` [`TestResExpiredSAIsNotUsed`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_residual_test.go#L132) |
| `RFC7296-4-1` | If the responder rejects the CREATE_CHILD_SA request with a NO_ADDITIONAL_SAS notification, the implementation MUST be capable of instead deleting the old SA and creating a new one (§4) | MUST | 4 - Conformance requirements | **positive:** `unit/verify` [`TestResNoAdditionalSAsTriggersReestablish`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_residual_test.go#L260). **negative:** `unit/verify` [`TestResOtherRekeyFailuresDoNotReestablish`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_residual_test.go#L279) |
| `RFC7296-2.15-3` | It MUST also accept a hex encoding of the shared secret (§2.15) | MUST | 2.15 | **positive:** `unit/verify` [`TestPSKAcceptsHexEncoding`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc7296_test.go#L283). **negative:** `unit/verify` [`TestHexEncodingIsExplicitAndNeverGuessed`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc7296_test.go#L328) |
| `RFC7296-3.3.4-4` | All implementations of IKEv2 MUST include a management facility that enables a user or system administrator to specify the suites that are acceptable for use with IKE (§3.3.4) | MUST | 3.3.4 | **positive:** `unit/verify` [`TestIKESuitePolicyIsOperatorSpecified`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc7296_test.go#L126). **negative:** `unit/verify` [`TestIKESuitePolicyRejectsAnUnhonourableSuite`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc7296_test.go#L198) |
| `RFC7296-3.5-2` | To assure maximum interoperability, implementations MUST be configurable to send at least one of ID_IPV4_ADDR, ID_FQDN, ID_RFC822_ADDR, or ID_KEY_ID (§3.5) | MUST | 3.5 | **positive:** `unit/verify` [`TestLocalIDTypeFollowsConfiguredIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_identity_test.go#L40). **negative:** `unit/verify` [`TestLocalIDIsOperatorControlledNotDerived`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_identity_test.go#L93) |
| `RFC7296-3.5-3` | MUST be configurable to accept all of these four types. (§3.5) | MUST | 3.5 | **positive:** `unit/verify` [`TestRemoteIDAcceptsEveryMandatoryType`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_identity_test.go#L133). **negative:** `unit/verify` [`TestRemoteIDRefusesTypesItCannotCompare`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_identity_test.go#L188) |
| `RFC7296-3.5-4` | IPv6-capable implementations MUST additionally be configurable to accept ID_IPV6_ADDR (§3.5) | MUST | 3.5 | **positive:** `unit/verify` [`TestRemoteIDAcceptsIPv6Identity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_identity_test.go#L217). **negative:** `unit/verify` [`TestIPv6IdentityLengthIsEnforced`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_identity_test.go#L256) |
| `RFC7296-3.6-1` | Implementations MUST be capable of being configured to send and accept up to four X.509 certificates in support of authentication (§3.6) | MUST | 3.6 | **positive:** `unit/verify` [`TestCcnCertificateCountReachesFourInBothDirections`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cert_chain_test.go#L230). **negative:** `unit/verify` [`TestCcnCertificateCountIsBoundedAndConfigurable`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cert_chain_test.go#L293). **negative:** `unit/verify` [`TestCcnCertificateCountReachesFourInBothDirections`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cert_chain_test.go#L232). **negative:** `unit/verify` [`TestCcnOverlongChainKillsTheSAOnBothRoles`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cert_chain_test.go#L794). **negative:** `functional/verify` [`ipsec-certificate-count-range.ci`](https://github.com/ze-software/ze/blob/main/test/parse/ipsec-certificate-count-range.ci#L9) |
| `RFC7296-3.6-2` | and also MUST be capable of being configured to send and accept the two Hash and URL formats (with HTTP URLs). (§3.6) | MUST | 3.6 | **positive:** `unit/verify` [`TestChuBothHashAndURLFormatsAreConfigurable`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cert_chain_test.go#L348). **negative:** `unit/verify` [`TestChuBothHashAndURLFormatsAreConfigurable`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cert_chain_test.go#L350). **negative:** `unit/verify` [`TestChuHashAndURLIsOffByDefault`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cert_chain_test.go#L439). **positive:** `functional/verify` [`ipsec-hash-and-url-accepted.ci`](https://github.com/ze-software/ze/blob/main/test/parse/ipsec-hash-and-url-accepted.ci#L25) |
| `RFC7296-3.6-3` | Implementations MUST support the "http:" scheme for hash-and-URL lookup (§3.6, §1.7) | MUST | 3.6 | **positive:** `unit/verify` [`TestChuHashURLLookupCacheIsContentAddressed`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cert_chain_test.go#L717). **positive:** `unit/verify` [`TestChuHashURLLookupUsesHTTPAndVerifiesTheHash`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cert_chain_test.go#L548). **negative:** `unit/verify` [`TestChuHashURLLookupRefusesEverythingOutsideTheBound`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cert_chain_test.go#L609). **negative:** `unit/verify` [`TestChuHashURLLookupUsesHTTPAndVerifiesTheHash`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cert_chain_test.go#L550) |
| `RFC7296-4-4` | For an implementation to be called conforming to this specification, it MUST be possible to configure it to accept the following: o  Public Key Infrastructure using X.509 (PKIX) Certificates containing and signed by RSA keys of size 1024 or 2048 bits, where the ID passed is any of ID_KEY_ID, ID_FQDN, ID_RFC822_ADDR, or ID_DER_ASN1_DN. o  Shared key authentication where the ID passed is any of ID_KEY_ID, ID_FQDN, or ID_RFC822_ADDR. (§4) | MUST | 4 - Conformance requirements | **positive:** `unit/verify` [`TestCfmConformanceConfigurationSetIsAcceptable`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_conformance_test.go#L132). **negative:** `unit/verify` [`TestCfmConformanceConfigurationSetIsAcceptable`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_conformance_test.go#L134). **negative:** `unit/verify` [`TestCfmConformanceSetDoesNotAcceptWhatItMustNot`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_conformance_test.go#L226). **positive:** `functional/verify` [`ipsec-hash-and-url-accepted.ci`](https://github.com/ze-software/ze/blob/main/test/parse/ipsec-hash-and-url-accepted.ci#L28). **negative:** `functional/verify` [`ipsec-remote-id-type-enum.ci`](https://github.com/ze-software/ze/blob/main/test/parse/ipsec-remote-id-type-enum.ci#L14) |
| `RFC7296-4-5` | Every implementation MUST be capable of doing four-message IKE_SA_INIT and IKE_AUTH exchanges establishing two SAs (one for IKE, one for ESP or AH) (§4) | MUST | 4 - Conformance requirements | **positive:** `unit/verify` [`TestResponderHandshakePSKEndToEnd`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/responder_test.go#L345). **negative:** `unit/verify` [`TestFourmFirstPairEstablishesNeitherSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_fourmessage_test.go#L26) |

## Gaps and untested MUSTs

RFC 7296 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7296-1.2-1`](#rfc7296-1.2-1)

The first pair of messages (IKE_SA_INIT) negotiate cryptographic algorithms, exchange nonces, and do a Diffie-Hellman exchange [DH]. The second pair of messages (IKE_AUTH) authenticate the previous messages, exchange identities and certificates, and establish the first Child SA. Parts of these messages are encrypted and integrity protected with keys established through the IKE_SA_INIT exchange, so the identities are hidden from eavesdroppers and all fields in all the messages are authenticated. (§1.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Clauses of the quoted span: IKE_SA_INIT negotiates algorithms, exchanges nonces and does a Diffie-Hellman exchange; IKE_AUTH authenticates the previous messages, exchanges identities and certificates and establishes the first Child SA. TestInitialExchangeEncryptionBoundary asserts only the encryption boundary (IKE_SA_INIT carries no SK and decryptAndParse refuses it; IKE_AUTH carries SK and decrypts). No assertion in the tagged unit covers the negotiation, nonce, DH, authentication, identity or Child SA clauses.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInitialExchangeEncryptionBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_test.go#L35) | unit/verify | unproven |
| positive | [`TestInitialExchangeEncryptionBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_test.go#L32) | unit/verify | unproven |

### [`RFC7296-2.6-1`](#rfc7296-2.6-1)

The initial two eight-octet fields in the header, called the "IKE SPIs", are used as a connection identifier at the beginning of IKE packets. (§2.6)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestHeaderRoundtrip only round-trips the two 8-octet SPI fields and TestDecodeTruncatedHeader refuses a short header; neither asserts the SPI pair is used to identify/map the IKE SA, which is what the quote states

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDecodeTruncatedHeader`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/header_test.go#L75) | unit/verify | unproven |
| positive | [`TestHeaderRoundtrip`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/header_test.go#L9) | unit/verify | unproven |

### [`RFC7296-2.7-1`](#rfc7296-2.7-1)

The responder MUST accept a single proposal or reject them all and return an error. The error is given in a notification of type NO_PROPOSAL_CHOSEN. Each IPsec protocol proposal contains one or more transforms. Each transform contains a Transform Type. The accepted cryptographic suite MUST contain exactly one transform of each type included in the proposal. (§2.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. NegotiateIKE picks one proposal or returns ErrNoProposalChosen; TestEsnResponderAnswersOnlyAValueTheOfferCarried asserts SAr2 carries one proposal with exactly one transform of the type and maps refusal to NO_PROPOSAL_CHOSEN

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestProposalNegotiationNoMatch`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/proposal_test.go#L63) | unit/verify | unproven |
| negative | [`TestEsnResponderAnswersOnlyAValueTheOfferCarried`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_esn_test.go#L87) | unit/verify | unproven |
| negative | [`checkESNExtendedOnlyRefused`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/ipsec/checkers.go#L1019) | interop/nightly | unproven |
| positive | [`TestProposalNegotiationFirstMatch`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/proposal_test.go#L8) | unit/verify | unproven |
| positive | [`TestEsnResponderAnswersOnlyAValueTheOfferCarried`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_esn_test.go#L95) | unit/verify | unproven |
| positive | [`checkESNBothOffered`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/ipsec/checkers.go#L1050) | interop/nightly | unproven |

### [`RFC7296-3.3-1`](#rfc7296-3.3-1)

If an initiator wants to propose both combined- mode ciphers and normal ciphers, it must include two proposals: one will have all the combined-mode ciphers, and the other will have all the normal ciphers with the integrity algorithms. (§3.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. send side emits AEAD and non-AEAD in separate proposals (TestESPProposalsNeverMixAEADClass); receive side refuses a mixed proposal and accepts AEAD alone (rfc7296_aead_mix_test.go)

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAeadMixInOneProposalIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_aead_mix_test.go#L32) | unit/verify | unproven |
| positive | [`TestAeadAloneInItsOwnProposalIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_aead_mix_test.go#L60) | unit/verify | unproven |
| positive | [`TestESPProposalsNeverMixAEADClass`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_test.go#L172) | unit/verify | unproven |

### [`RFC7296-3.3-2`](#rfc7296-3.3-2)

Combined-mode ciphers include both integrity and encryption in a single encryption algorithm, and MUST either offer no integrity algorithm or a single integrity algorithm of "NONE" (§3.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. AEAD ESP proposal built with no INTEG transform, non-AEAD keeps it (TestESPWireProposalAEADIntegNone)

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestESPWireProposalAEADIntegNone`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_test.go#L151) | unit/verify | unproven |
| positive | [`TestESPWireProposalAEADIntegNone`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_test.go#L148) | unit/verify | unproven |

### [`RFC7296-3.3.2-1`](#rfc7296-3.3.2-1)

An SA payload proposing the establishment of an SA has the following mandatory and optional Transform Types. A compliant implementation MUST understand all mandatory and optional types for each protocol it supports (though it need not accept proposals with unacceptable suites). A proposal MAY omit the optional types if the only value for them it will accept is NONE. Protocol Mandatory Types Optional Types --------------------------------------------------- IKE ENCR, PRF, INTEG*, D-H (§3.3.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The row now quotes the Section 3.3.3 span through the IKE line of the table (IKE mandatory: ENCR, PRF, INTEG*, D-H) and the sentence that a compliant implementation MUST understand all mandatory and optional types for each protocol it supports. Asserted: Ze's IKE proposal carries exactly one ENCR, PRF, INTEG and DH (countTransform != 1 fails), and a peer IKE proposal without PRF draws ErrNoProposalChosen. Not asserted: a peer proposal missing ENCR, INTEG or D-H, and the understand-every-type clause (tagged under RFC7296-3.3.3-1 only).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIKEWireProposalHasAllTransforms`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_test.go#L198) | unit/verify | unproven |
| positive | [`TestIKEWireProposalHasAllTransforms`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_test.go#L195) | unit/verify | unproven |

### [`RFC7296-3.3.6-1`](#rfc7296-3.3.6-1)

A compliant implementation MUST understand all mandatory and optional types for each protocol it supports (though it need not accept proposals with unacceptable suites). A proposal MAY omit the optional types if the only value for them it will accept is NONE. Protocol Mandatory Types Optional Types --------------------------------------------------- IKE ENCR, PRF, INTEG*, D-H (§3.3.3)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. The row now quotes the Section 3.3.3 span that makes D-H a mandatory Transform Type for IKE and understanding every mandatory type a MUST. TestResponderRequiresKEForDH's negative removes the KE PAYLOAD from an IKE_SA_INIT whose SA proposals still carry the D-H transform (buildWireIKEProposals), and asserts the SA goes dead: that is the KE-payload rule (Sections 1.2, 3.4), a neighbouring obligation. No tagged unit offers an IKE proposal missing the D-H transform, so the tags attribute the neighbouring rule's evidence to this row.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestResponderRequiresKEForDH`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_test.go#L229) | unit/verify | unproven |
| positive | [`TestResponderRequiresKEForDH`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_test.go#L226) | unit/verify | unproven |

### [`RFC7296-1.3.3-1`](#rfc7296-1.3.3-1)

The KEi payload MUST be included. (§1.3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. initiateIKERekey emits KEi (TestRespondIKERekey) and respondIKERekey refuses a rekey without KE naming KEi; test comment still cites Section 1.3.3 where the sentence is 1.3.2

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRespondIKERekeyRejectsMissingKE`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_test.go#L123) | unit/verify | unproven |
| positive | [`TestRespondIKERekey`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/responder_test.go#L265) | unit/verify | unproven |

### [`RFC7296-1.3.3-2`](#rfc7296-1.3.3-2)

The REKEY_SA notification MUST be included in a CREATE_CHILD_SA exchange if the purpose of the exchange is to replace an existing ESP or AH SA (§1.3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRksaChildRekeyCarriesTheRekeySANotify`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_rekeysa_test.go#L46) | unit/verify | unproven |
| positive | [`TestRksaChildRekeyCarriesTheRekeySANotify`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_rekeysa_test.go#L40) | unit/verify | unproven |

### [`RFC7296-2.9-1`](#rfc7296-2.9-1)

When the responder chooses a subset of the traffic proposed by the initiator, it narrows the Traffic Selectors to some subset of the initiator's proposal (provided the set does not become the null set). (§2.9)

Audit verdict: enforced (the tests do what the requirement demands), fresh. narrowChildSelectors answers a disjoint proposal with TS_UNACCEPTABLE and accepts an overlapping one; initiator refuses a widened answer (errTSWidened). Two further tagged units (TestChildRekeyAnswerWithoutTrafficSelectorsIsRefused, TestRekeyWithoutTrafficSelectorsIsRefused) test TS payload presence in rekey, a neighbouring obligation

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestChildRekeyAnswerWithoutTrafficSelectorsIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_rekey_initiator_answer_test.go#L267) | unit/verify | unproven |
| negative | [`TestChildRekeyInitiatorInstallsTheAnsweredSelectors`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_rekey_initiator_answer_test.go#L112) | unit/verify | unproven |
| negative | [`TestRekeyWithoutTrafficSelectorsIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rekey_test.go#L337) | unit/verify | unproven |
| negative | [`TestTSUnacceptableIsSentWhenNothingIsAcceptable`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_test.go#L76) | unit/verify | unproven |
| positive | [`TestChildRekeyAnswerWithoutTrafficSelectorsIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_rekey_initiator_answer_test.go#L273) | unit/verify | unproven |
| positive | [`TestChildRekeyInitiatorInstallsTheAnsweredSelectors`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_rekey_initiator_answer_test.go#L105) | unit/verify | unproven |
| positive | [`TestRekeyWithoutTrafficSelectorsIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rekey_test.go#L345) | unit/verify | unproven |
| positive | [`TestTSUnacceptableIsSentWhenNothingIsAcceptable`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_test.go#L60) | unit/verify | unproven |

### [`RFC7296-2.9-2`](#rfc7296-2.9-2)

If the responder's policy allows it to accept the first selector of TSi and TSr, then the responder MUST narrow the Traffic Selectors to a subset that includes the initiator's first choices (§2.9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNarrowingIncludesFirstChoice`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ts_narrow_test.go#L143) | unit/verify | unproven |
| positive | [`TestPeerInitiatedRekeyIsNarrowedInTheExchangeOrientation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_rekey_orientation_test.go#L133) | unit/verify | unproven |
| positive | [`TestAuthResponsePayloadsCarryTheNarrowedSelectors`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_test.go#L371) | unit/verify | unproven |
| positive | [`TestNarrowingIncludesFirstChoice`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ts_narrow_test.go#L99) | unit/verify | unproven |

### [`RFC7296-2.9.2-1`](#rfc7296-2.9.2-1)

Thus, the new SA MUST NOT have narrower selectors than the original (§2.9.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestChildRekeyAnswerBelowTheScopeInUseIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_rekey_initiator_answer_test.go#L194) | unit/verify | unproven |
| negative | [`TestRekeyFloorIsNotNarrowed`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ts_narrow_test.go#L233) | unit/verify | unproven |
| positive | [`TestChildRekeyAnswerBelowTheScopeInUseIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_rekey_initiator_answer_test.go#L183) | unit/verify | unproven |
| positive | [`TestRekeyAnswerMatchesTheInstalledSelectors`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_rekey_orientation_test.go#L322) | unit/verify | unproven |
| positive | [`TestRekeyFloorIsNotNarrowed`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ts_narrow_test.go#L222) | unit/verify | unproven |

### [`RFC7296-2.9.2-2`](#rfc7296-2.9.2-2)

The responder MUST NOT narrow down the Traffic Selectors narrower than the scope currently in use (§2.9.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestChildRekeyAnswerBelowTheScopeInUseIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_rekey_initiator_answer_test.go#L198) | unit/verify | unproven |
| negative | [`TestRekeyProposalBelowTheFloorIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_rekey_orientation_test.go#L384) | unit/verify | unproven |
| negative | [`TestRekeyFloorIsNotNarrowed`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ts_narrow_test.go#L261) | unit/verify | unproven |
| positive | [`TestChildRekeyAnswerBelowTheScopeInUseIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_rekey_initiator_answer_test.go#L188) | unit/verify | unproven |
| positive | [`TestRekeyProposalBelowTheFloorIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_rekey_orientation_test.go#L378) | unit/verify | unproven |
| positive | [`TestRekeyFloorIsNotNarrowed`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ts_narrow_test.go#L247) | unit/verify | unproven |

### [`RFC7296-1.3.1-1`](#rfc7296-1.3.1-1)

If the request is accepted, the response MUST also include a notification of type USE_TRANSPORT_MODE (§1.3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestUseTransportModeNotifyIsEchoedOnlyWhenAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_test.go#L220) | unit/verify | unproven |
| positive | [`TestUseTransportModeNotifyIsEchoedOnlyWhenAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_test.go#L209) | unit/verify | unproven |

### [`RFC7296-1.3.1-2`](#rfc7296-1.3.1-2)

If the responder declines the request, the Child SA will be established in tunnel mode. If this is unacceptable to the initiator, the initiator MUST delete the SA (§1.3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTransportRequiredDeletesTheSAOnDecline`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_test.go#L264) | unit/verify | unproven |
| positive | [`TestTransportRequiredDeletesTheSAOnDecline`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_test.go#L249) | unit/verify | unproven |

### [`RFC7296-2.23.1-1`](#rfc7296-2.23.1-1)

For transport mode, it MUST use exactly one IP address in the TSi and TSr payloads (§2.23.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestResponderAnswersTheIKESAAddressesBehindANAT`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_nat_test.go#L117) | unit/verify | revert, verified |
| negative | [`TestTransportModeUsesExactlyOneAddress`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_test.go#L129) | unit/verify | unproven |
| positive | [`TestResponderAnswersTheIKESAAddressesBehindANAT`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_nat_test.go#L107) | unit/verify | revert, verified |
| positive | [`TestTransportModeUsesExactlyOneAddress`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_test.go#L116) | unit/verify | unproven |

### [`RFC7296-2.23.1-2`](#rfc7296-2.23.1-2)

The TSi entries MUST have exactly one IP address, and that MUST match the source address of the IKE SA (§2.23.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTransportSelectorsMatchTheIKESAAddressesBehindANAT`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_nat_test.go#L62) | unit/verify | revert, verified |
| negative | [`TestTransportModeUsesExactlyOneAddress`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_test.go#L170) | unit/verify | unproven |
| positive | [`TestTransportSelectorsMatchTheIKESAAddressesBehindANAT`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_nat_test.go#L39) | unit/verify | revert, verified |
| positive | [`TestTransportModeUsesExactlyOneAddress`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_test.go#L140) | unit/verify | unproven |

### [`RFC7296-2.23.1-3`](#rfc7296-2.23.1-3)

The TSr entries MUST have exactly one IP address, and that MUST match the destination address of the IKE SA (§2.23.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTransportSelectorsMatchTheIKESAAddressesBehindANAT`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_nat_test.go#L71) | unit/verify | revert, verified |
| negative | [`TestTransportModeUsesExactlyOneAddress`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_test.go#L176) | unit/verify | unproven |
| positive | [`TestTransportSelectorsMatchTheIKESAAddressesBehindANAT`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_nat_test.go#L51) | unit/verify | revert, verified |
| positive | [`TestTransportModeUsesExactlyOneAddress`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transport_test.go#L150) | unit/verify | unproven |

### [`RFC7296-3.13.1-1`](#rfc7296-3.13.1-1)

For protocols for which port is undefined (including protocol 0), or if all ports are allowed, this field MUST be zero. (§3.13.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. selectorsToWire encodes protocol-0 all-ports Start Port 0; single-port selector proves the encoder can emit non-zero

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPortEncodingFollowsSection3131`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ts_narrow_test.go#L350) | unit/verify | unproven |
| positive | [`TestPortEncodingFollowsSection3131`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ts_narrow_test.go#L341) | unit/verify | unproven |

### [`RFC7296-3.13.1-2`](#rfc7296-3.13.1-2)

For protocols for which port is undefined (including protocol 0), or if all ports are allowed, this field MUST be 65535. (§3.13.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. all-ports End Port 65535; single-port 443 discriminator

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPortEncodingFollowsSection3131`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ts_narrow_test.go#L364) | unit/verify | unproven |
| positive | [`TestPortEncodingFollowsSection3131`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ts_narrow_test.go#L358) | unit/verify | unproven |

### [`RFC7296-3.13.1-3`](#rfc7296-3.13.1-3)

Systems working with [IPSECARCH] that wish to indicate "OPAQUE" ports, but not "ANY" ports, MUST set the start port to 65535 and the end port to 0. (§3.13.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. PortOpaque encodes 65535/0 and ANY is not emitted in that form

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPortEncodingFollowsSection3131`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ts_narrow_test.go#L400) | unit/verify | unproven |
| positive | [`TestPortEncodingFollowsSection3131`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ts_narrow_test.go#L372) | unit/verify | unproven |

### [`RFC7296-2.23-1`](#rfc7296-2.23-1)

The IKE initiator MUST check the NAT_DETECTION_SOURCE_IP or NAT_DETECTION_DESTINATION_IP payloads if present (§2.23)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestNATDetectionPresent/Absent call DetectNAT on hand-built hashes; no tagged unit shows the initiator checks the received NAT_DETECTION payloads during IKE_SA_INIT, so the tests stay green if the engine stopped calling DetectNAT

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNATDetectionAbsent`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/nat_test.go#L63) | unit/verify | unproven |
| positive | [`TestNATDetectionPresent`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/nat_test.go#L43) | unit/verify | unproven |

### [`RFC7296-2.23-2`](#rfc7296-2.23-2)

An IPsec endpoint that discovers a NAT between it and its correspondent (as described below) MUST send all subsequent traffic from port 4500 (§2.23)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestChildSANATTEncapPorts asserts ESP UDP encap on 4500 when NAT is detected; the quote says ALL subsequent traffic moves to 4500, and no tagged unit asserts IKE traffic floats to 4500

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestChildSANoNATNoEncap`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_test.go#L276) | unit/verify | unproven |
| positive | [`TestChildSANATTEncapPorts`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_test.go#L277) | unit/verify | unproven |

### [`RFC7296-2.23-3`](#rfc7296-2.23-3)

To tunnel IKE packets over UDP port 4500, the IKE header has four octets of zeros prepended and the result immediately follows the UDP header. (§2.23)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Re-read 2026-09-27 after the RFC3948-2.2-1 tags were deleted and the RFC3948-2.1-3 tags moved to RFC7296-2.23-12 in these units (comment lines only; bodies unchanged). Forbidden behaviour: an IKE message sent on UDP port 4500 without the four zero octets prepended, or not immediately after the UDP header. TestNonESPMarker exercises AddNonESPMarker/StripNonESPMarker in isolation and TestNonESPMarkerESPPacket shows a non-zero SPI is not stripped; no tagged unit shows an IKE message Ze sends on port 4500 carries the four zero octets

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNonESPMarkerESPPacket`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/nat_test.go#L109) | unit/verify | unproven |
| positive | [`TestNonESPMarker`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/nat_test.go#L80) | unit/verify | unproven |

### [`RFC7296-2.8-1`](#rfc7296-2.8-1)

If redundant SAs are created through such a collision, the SA created with the lowest of the four nonces used in the two exchanges SHOULD be closed by the endpoint that created it (§2.8, §2.8.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRekeyCollision`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rekey_test.go#L27) | unit/verify | unproven |
| negative | [`TestRekeyCollisionIKEBranchLowestNonceAbandons`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rekey_test.go#L104) | unit/verify | unproven |
| negative | [`TestRekeyCollisionLowestNonceAbandons`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rekey_test.go#L50) | unit/verify | unproven |
| positive | [`TestRekeyCollision`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rekey_test.go#L23) | unit/verify | unproven |
| positive | [`TestRekeyCollisionIKEBranchLowestNonceAbandons`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rekey_test.go#L101) | unit/verify | unproven |
| positive | [`TestRekeyCollisionLowestNonceAbandons`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rekey_test.go#L46) | unit/verify | unproven |

### [`RFC7296-2.4-1`](#rfc7296-2.4-1)

To check whether the other side is alive, IKE specifies an empty INFORMATIONAL request that (like all IKE requests) requires an acknowledgement (§2.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. empty INFORMATIONAL request answered by an encrypted INFORMATIONAL response with zero inner payloads echoing the message ID; a response is not answered

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDPDEmptyInformationalGetsEmptyResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_test.go#L73) | unit/verify | unproven |
| positive | [`TestDPDEmptyInformationalGetsEmptyResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_test.go#L70) | unit/verify | unproven |

### [`RFC7296-1.4-1`](#rfc7296-1.4-1)

Normally, the response in the INFORMATIONAL exchange will contain Delete payloads for the paired SAs going in the other direction. (§1.4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Child SA Delete response carries a Delete naming the paired inbound SPI; IKE SA Delete draws an empty response

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDelIKEDeleteDrawsAnEmptyResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_delete_test.go#L132) | unit/verify | unproven |
| positive | [`TestDelResponseCarriesThePairedDelete`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_delete_test.go#L111) | unit/verify | unproven |

### [`RFC7296-2.8-2`](#rfc7296-2.8-2)

A difference between IKEv1 and IKEv2 is that in IKEv1 SA lifetimes were negotiated. In IKEv2, each end of the SA is responsible for enforcing its own lifetime policy on the SA and rekeying the SA when necessary. (§2.8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. no lifetime transform or attribute reaches the wire (TestLifetimesNotNegotiatedOnWire); lifetime state derives from the local value (TestSALifetimeTime)

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLifetimesNotNegotiatedOnWire`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_test.go#L300) | unit/verify | unproven |
| positive | [`TestSALifetimeTime`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rekey_test.go#L151) | unit/verify | unproven |

### [`RFC7296-1-1`](#rfc7296-1-1)

In all cases, all IKE_SA_INIT exchanges MUST complete before any other exchange type, then all IKE_AUTH exchanges MUST complete, and following that, any number of CREATE_CHILD_SA and INFORMATIONAL exchanges may occur in any order (§1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAutExchangesRunInRFCOrder`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_auth_test.go#L170) | unit/verify | unproven |
| positive | [`TestAutExchangesRunInRFCOrder`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_auth_test.go#L168) | unit/verify | unproven |

### [`RFC7296-1.2-2`](#rfc7296-1.2-2)

If any CERT payloads are included, the first certificate provided MUST contain the public key used to verify the AUTH field (§1.2, §3.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRccTwoLevelChainAuthenticates`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/remote_cert_chain_test.go#L213) | unit/verify | unproven |
| negative | [`TestAutFirstCertificateCarriesTheAuthKey`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_auth_test.go#L252) | unit/verify | unproven |
| positive | [`TestRccTwoLevelChainAuthenticates`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/remote_cert_chain_test.go#L210) | unit/verify | unproven |
| positive | [`TestAutFirstCertificateCarriesTheAuthKey`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_auth_test.go#L250) | unit/verify | unproven |

### [`RFC7296-1.2-3`](#rfc7296-1.2-3)

Both parties in the IKE_AUTH exchange MUST verify that all signatures and Message Authentication Codes (MACs) are computed correctly (§1.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAutIKEAuthVerifiesEveryMAC`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_auth_test.go#L303) | unit/verify | unproven |
| negative | [`TestAutIKEAuthVerifiesSignatures`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_auth_test.go#L368) | unit/verify | unproven |
| positive | [`TestAutIKEAuthVerifiesEveryMAC`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_auth_test.go#L301) | unit/verify | unproven |
| positive | [`TestAutIKEAuthVerifiesSignatures`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_auth_test.go#L366) | unit/verify | unproven |

### [`RFC7296-1.2-4`](#rfc7296-1.2-4)

If either side uses a shared secret for authentication, the names in the ID payload MUST correspond to the key used to generate the AUTH payload (§1.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAutSharedSecretAuthBindsTheIDName`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_auth_test.go#L430) | unit/verify | unproven |
| positive | [`TestAutSharedSecretAuthBindsTheIDName`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_auth_test.go#L428) | unit/verify | unproven |

### [`RFC7296-1.2-5`](#rfc7296-1.2-5)

If the initiator guesses wrong, the responder will respond with a Notify payload of type INVALID_KE_PAYLOAD indicating the selected group. In this case, the initiator MUST retry the IKE_SA_INIT with the corrected Diffie-Hellman group (§1.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestKegInitiatorRefusesUnofferedGroup`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_kegroup_test.go#L142) | unit/verify | unproven |
| positive | [`TestKegInitiatorRetriesOnInvalidKEPayload`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_kegroup_test.go#L103) | unit/verify | unproven |

### [`RFC7296-1.2-6`](#rfc7296-1.2-6)

The initiator MUST again propose its full set of acceptable cryptographic suites because the rejection message was unauthenticated and otherwise an active attacker could trick the endpoints into negotiating a weaker suite than a stronger one that they both prefer (§1.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestKegRetryOfferIsNotNarrowedByTheNotify`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_kegroup_test.go#L225) | unit/verify | unproven |
| positive | [`TestKegRetryReproposesEveryConfiguredSuite`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_kegroup_test.go#L190) | unit/verify | unproven |

### [`RFC7296-2.4-11`](#rfc7296-2.4-11)

An endpoint MUST conclude that the other endpoint has failed only when repeated attempts to contact it have gone unanswered for a timeout period or when a cryptographically protected INITIAL_CONTACT notification is received on a different IKE SA to the same authenticated identity (§2.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDPDNoTransportTakesNoWindow`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/dpd_test.go#L104) | unit/verify | unproven |
| negative | [`TestDPDVerdictNeedsARepeatedAttempt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/dpd_test.go#L231) | unit/verify | unproven |
| negative | [`TestSesPeerFailedOnlyAfterRepeatedSilence`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_session_test.go#L311) | unit/verify | unproven |
| positive | [`TestDPDVerdictEndsAProbeThatCannotBeRepeated`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/dpd_test.go#L277) | unit/verify | unproven |
| positive | [`TestDPDVerdictNeedsARepeatedAttempt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/dpd_test.go#L233) | unit/verify | unproven |
| positive | [`TestSesPeerFailedOnlyAfterRepeatedSilence`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_session_test.go#L306) | unit/verify | unproven |

### [`RFC7296-2.4-12`](#rfc7296-2.4-12)

Implementations MUST limit the rate at which they take actions based on unprotected messages (§2.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestErrResponderWindowDoesNotReflectToObservedSource`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L522) | unit/verify | unproven |
| negative | [`TestErrUnprotectedMessageDrawsNoCachedResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L233) | unit/verify | unproven |
| negative | [`TestSesLimitsWorkOnUnprotectedMessages`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_session_test.go#L547) | unit/verify | unproven |
| positive | [`TestErrResponderWindowDoesNotReflectToObservedSource`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L515) | unit/verify | unproven |
| positive | [`TestErrUnprotectedMessageDrawsNoCachedResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L228) | unit/verify | unproven |
| positive | [`TestSesLimitsWorkOnUnprotectedMessages`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_session_test.go#L543) | unit/verify | unproven |

### [`RFC7296-2.4-13`](#rfc7296-2.4-13)

To be a good network citizen, retransmission times MUST increase exponentially to avoid flooding the network and making an existing congestion situation worse (§2.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSesRetransmitWaitIncreasesExponentially`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_session_test.go#L262) | unit/verify | unproven |
| positive | [`TestSesRetransmitWaitIncreasesExponentially`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_session_test.go#L260) | unit/verify | unproven |

### [`RFC7296-2.7-2`](#rfc7296-2.7-2)

Each proposal contains one protocol. If a proposal is accepted, the SA response MUST contain the same protocol (§2.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSesAcceptedProposalKeepsItsProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_session_test.go#L399) | unit/verify | unproven |
| positive | [`TestSesAcceptedProposalKeepsItsProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_session_test.go#L396) | unit/verify | unproven |

### [`RFC7296-2.10-4`](#rfc7296-2.10-4)

Nonces used in IKEv2 MUST be randomly chosen (§2.10)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSesNoncesAreRandomlyChosenAndNeverReused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_session_test.go#L95) | unit/verify | unproven |
| positive | [`TestSesNoncesAreRandomlyChosenAndNeverReused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_session_test.go#L93) | unit/verify | unproven |

### [`RFC7296-2.11-1`](#rfc7296-2.11-1)

An implementation MUST accept incoming requests even if the source port is not 500 or 4500 (§2.11, §2.23)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSesAcceptsRequestFromAnySourcePort`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_session_test.go#L486) | unit/verify | unproven |
| negative | [`TestPrtAcceptsDatagramFromAnySourcePort`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/rfc7296_port_test.go#L46) | unit/verify | unproven |
| positive | [`TestSesAcceptsRequestFromAnySourcePort`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_session_test.go#L483) | unit/verify | unproven |
| positive | [`TestPrtAcceptsDatagramFromAnySourcePort`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/rfc7296_port_test.go#L43) | unit/verify | unproven |

### [`RFC7296-2.11-2`](#rfc7296-2.11-2)

MUST respond to the address and port from which the request was received (§2.11)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestNattRepliesToTheObservedSourcePort asserts the stored peerEndpoint/remoteUDPAddr port, never reads the reply datagram on the odd-port listener it opens; the negative (forged packet does not move the endpoint) is the 2.23 dynamic-update bound, not a violation of this row

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNattUnauthenticatedPacketDoesNotMoveTheEndpoint`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_test.go#L173) | unit/verify | revert, verified |
| positive | [`TestNattRepliesToTheObservedSourcePort`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_test.go#L111) | unit/verify | revert, verified |

### [`RFC7296-2.11-3`](#rfc7296-2.11-3)

It MUST specify the address and port at which the request was received as the source address and port in the response (§2.11)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNattReplyRefusesWithoutADestination`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_test.go#L330) | unit/verify | unproven |
| positive | [`TestNattReplyLeavesFromTheArrivalSocket`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_test.go#L225) | unit/verify | unproven |

### [`RFC7296-2.16-11`](#rfc7296-2.16-11)

these protocols are typically used to authenticate the initiator to the responder and MUST be used in conjunction with a public-key-signature-based authentication of the responder to the initiator. (§2.16)

Audit verdict: enforced (the tests do what the requirement demands), fresh. EAP responder AUTH is an RFC 7427 signature verified under the cert key; no-cert EAP refused at setup and config; initiator refuses PSK AUTH from EAP responder; PSK peers unaffected

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEapAuthConfigRejectsEAPWithoutCertificate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_eap_auth_test.go#L206) | unit/verify | unproven |
| negative | [`TestEapAuthInitiatorRefusesPreSharedKeyResponder`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_eap_auth_test.go#L267) | unit/verify | unproven |
| negative | [`TestEapAuthResponderRefusesWithoutCertificate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_eap_auth_test.go#L153) | unit/verify | unproven |
| positive | [`TestEapAuthConfigAcceptsPreSharedKeyPeer`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_eap_auth_test.go#L324) | unit/verify | unproven |
| positive | [`TestEapAuthNonEAPPreSharedKeyStillAuthenticates`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_eap_auth_test.go#L240) | unit/verify | unproven |
| positive | [`TestEapAuthResponderSignsWithPublicKey`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_eap_auth_test.go#L82) | unit/verify | unproven |

### [`RFC7296-2.16-6`](#rfc7296-2.16-6)

Extensible authentication is implemented in IKE as additional IKE_AUTH exchanges that MUST be completed in order to initialize the IKE SA (§2.16)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAutEAPRunsAsExtraIKEAuthExchanges`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_auth_test.go#L564) | unit/verify | unproven |
| positive | [`TestAutEAPRunsAsExtraIKEAuthExchanges`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_auth_test.go#L562) | unit/verify | unproven |

### [`RFC7296-2.16-7`](#rfc7296-2.16-7)

This shared key generated during an IKE exchange MUST NOT be used for any other purpose (§2.16)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAutEAPSharedKeyServesAuthAlone`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_auth_test.go#L621) | unit/verify | unproven |
| positive | [`TestAutEAPSharedKeyServesAuthAlone`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_auth_test.go#L619) | unit/verify | unproven |

### [`RFC7296-2.23-7`](#rfc7296-2.23-7)

Both the IKE initiator and responder MUST include in their IKE_SA_INIT packets Notify payloads of type NAT_DETECTION_SOURCE_IP and NAT_DETECTION_DESTINATION_IP (§2.23)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSesBothEndsSendNATDetectionNotifies`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_session_test.go#L633) | unit/verify | unproven |
| positive | [`TestSesBothEndsSendNATDetectionNotifies`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_session_test.go#L631) | unit/verify | unproven |

### [`RFC7296-2.23-8`](#rfc7296-2.23-8)

An IPsec endpoint that discovers a NAT between it and its correspondent (as described below) MUST send all subsequent traffic from port 4500 (§2.23)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNattFloatedSAWithoutNATTSocketSendsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_test.go#L459) | unit/verify | revert, verified |
| negative | [`TestNattNoFloatWithoutNAT`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_test.go#L425) | unit/verify | revert, verified |
| positive | [`TestNattFloatsEverySenderToPort4500`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_test.go#L355) | unit/verify | revert, verified |
| positive | [`TestNattRekeyedChildKeepsUDPEncap`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_test.go#L549) | unit/verify | unproven |

### [`RFC7296-2.23-9`](#rfc7296-2.23-9)

UDP encapsulation MUST NOT be done on port 500 (§2.23)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEncapPortsAreExpressible`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_test.go#L581) | unit/verify | unproven |
| positive | [`TestEncapNeverRequestedOnPort500`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_test.go#L481) | unit/verify | unproven |

### [`RFC7296-2.23-10`](#rfc7296-2.23-10)

If Network Address Translation Traversal (NAT-T) is supported (that is, if NAT_DETECTION_*_IP payloads were exchanged during IKE_SA_INIT), all devices MUST be able to receive and process both UDP-encapsulated ESP and non-UDP-encapsulated ESP packets at any time. (§2.23)

Audit verdict: enforced (the tests do what the requirement demands), fresh. every Child SA accepts both ESP forms inbound (AcceptBothESPForms) and both forms are programmable; kernel behavior measured by the cited QEMU tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBfmBothESPFormsReceivedOnOneChildSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_bothforms_test.go#L209) | unit/verify | unproven |
| positive | [`TestBfmBothESPFormsAreReachable`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_bothforms_test.go#L153) | unit/verify | unproven |

### [`RFC7296-2.23-11`](#rfc7296-2.23-11)

Implementations MUST process received UDP-encapsulated ESP packets even when no NAT was detected (§2.23)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBfmBareESPKeptForUnfloatedSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_bothforms_test.go#L136) | unit/verify | unproven |
| positive | [`TestBfmEncapsulatedESPAcceptedWithoutNAT`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_bothforms_test.go#L63) | unit/verify | unproven |
| positive | [`TestBfmEncapsulatedESPSentWhenNATDetected`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_natt_bothforms_test.go#L112) | unit/verify | unproven |

### [`RFC7296-2.23-12`](#rfc7296-2.23-12)

To tunnel ESP packets over UDP port 4500, the ESP header immediately follows the UDP header. Since the first four octets of the ESP header contain the SPI, and the SPI cannot validly be zero, it is always possible to distinguish ESP and IKE messages. (§2.23)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Second sentence enforced: TestNonESPMarker fails ('StripNonESPMarker returned false for valid IKE packet') when a zero-marked packet is not recognised as IKE, and TestNonESPMarkerESPPacket fails ('ESP packet should not be recognized as IKE') when a non-zero SPI is taken for IKE. First sentence unproven: no tagged unit asserts that a UDP-encapsulated ESP packet Ze sends carries the ESP header immediately after the UDP header (the kernel XFRM encapsulation Ze configures).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNonESPMarkerESPPacket`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/nat_test.go#L112) | unit/verify | revert, verified |
| positive | [`TestNonESPMarker`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/nat_test.go#L83) | unit/verify | revert, verified |

### [`RFC7296-3.9-2`](#rfc7296-3.9-2)

Nonce values MUST NOT be reused (§3.9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSesNoncesAreRandomlyChosenAndNeverReused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_session_test.go#L99) | unit/verify | unproven |
| negative | [`TestSesRekeyDrawsFreshNoncesOnBothSides`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_session_test.go#L147) | unit/verify | unproven |
| positive | [`TestSesNoncesAreRandomlyChosenAndNeverReused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_session_test.go#L97) | unit/verify | unproven |
| positive | [`TestSesRekeyDrawsFreshNoncesOnBothSides`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_session_test.go#L145) | unit/verify | unproven |

### [`RFC7296-1.3-2`](#rfc7296-1.3-2)

If the responder selects a proposal using a different Diffie-Hellman group (other than NONE), the responder MUST reject the request and indicate its preferred Diffie-Hellman group in the INVALID_KE_PAYLOAD Notify payload (§1.3, §3.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegRekeyRejectsMismatchedKEGroup`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_negotiation_test.go#L280) | unit/verify | unproven |
| positive | [`TestNegRekeyRejectsMismatchedKEGroup`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_negotiation_test.go#L276) | unit/verify | unproven |

### [`RFC7296-2.8.2-1`](#rfc7296-2.8.2-1)

The new IKE SA containing the lowest nonce SHOULD be deleted by the node that created it, and the other surviving new IKE SA MUST inherit all the Child SAs (§2.8.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegIKERekeyCollisionResolves`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_negotiation_test.go#L403) | unit/verify | unproven |
| negative | [`TestNegSurvivingSAInheritsChildren`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_negotiation_test.go#L479) | unit/verify | unproven |
| positive | [`TestNegIKERekeyCollisionResolves`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_negotiation_test.go#L399) | unit/verify | unproven |
| positive | [`TestNegSurvivingSAInheritsChildren`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_negotiation_test.go#L476) | unit/verify | unproven |

### [`RFC7296-3.3.6-3`](#rfc7296-3.3.6-3)

The initiator of an exchange MUST check that the accepted offer is consistent with one of its proposals, and if not MUST terminate the exchange (§3.3.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestVerInitiatorRefusesUnsentESPKeyLength`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_verify_test.go#L74) | unit/verify | unproven |
| negative | [`TestVerInitiatorRefusesUnsentIKEKeyLength`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_verify_test.go#L34) | unit/verify | unproven |
| negative | [`TestEsnInitiatorRefusesAnESNValueItNeverOffered`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_esn_test.go#L144) | unit/verify | unproven |
| negative | [`TestKlnInitiatorRefusesLongerAcceptedKey`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_keylength_test.go#L46) | unit/verify | unproven |
| negative | [`TestNegInitiatorRejectsUnproposedOffer`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_negotiation_test.go#L546) | unit/verify | unproven |
| positive | [`TestVerInitiatorRefusesUnsentESPKeyLength`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_verify_test.go#L76) | unit/verify | unproven |
| positive | [`TestVerInitiatorRefusesUnsentIKEKeyLength`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_verify_test.go#L41) | unit/verify | unproven |
| positive | [`TestEsnInitiatorRefusesAnESNValueItNeverOffered`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_esn_test.go#L150) | unit/verify | unproven |
| positive | [`TestKlnInitiatorRefusesLongerAcceptedKey`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_keylength_test.go#L52) | unit/verify | unproven |
| positive | [`TestNegInitiatorRejectsUnproposedOffer`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_negotiation_test.go#L542) | unit/verify | unproven |

### [`RFC7296-1.4-3`](#rfc7296-1.4-3)

Control messages that pertain to an IKE SA MUST be sent under that IKE SA. Control messages that pertain to Child SAs MUST be sent under the protection of the IKE SA that generated them (§1.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLcyControlMessagesRideTheirOwnIKESA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_lifecycle_test.go#L120) | unit/verify | unproven |
| positive | [`TestLcyControlMessagesRideTheirOwnIKESA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_lifecycle_test.go#L116) | unit/verify | unproven |

### [`RFC7296-1.4-4`](#rfc7296-1.4-4)

The recipient of an INFORMATIONAL exchange request MUST send some response; otherwise, the sender will assume the message was lost in the network and will retransmit it (§1.4, §4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLcyEveryInformationalRequestDrawsAResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_lifecycle_test.go#L179) | unit/verify | unproven |
| positive | [`TestLcyEveryInformationalRequestDrawsAResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_lifecycle_test.go#L176) | unit/verify | unproven |

### [`RFC7296-1.4-5`](#rfc7296-1.4-5)

INFORMATIONAL exchanges MUST ONLY occur after the initial exchanges and are cryptographically protected with the negotiated keys (§1.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDpdProbeIsEncrypted`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_dpd_test.go#L67) | unit/verify | unproven |
| positive | [`TestDpdProbeIsEncrypted`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_dpd_test.go#L63) | unit/verify | unproven |

### [`RFC7296-1.4.1-6`](#rfc7296-1.4.1-6)

To delete an SA, an INFORMATIONAL exchange with one or more Delete payloads is sent listing the SPIs (as they would be expected in the headers of inbound packets) of the SAs to be deleted. The recipient MUST close the designated SAs (§1.4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDelUnknownSPIClosesNothing`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_delete_test.go#L81) | unit/verify | unproven |
| positive | [`TestDelPeerDeleteClosesTheDesignatedChildSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_delete_test.go#L61) | unit/verify | unproven |

### [`RFC7296-1.4.1-7`](#rfc7296-1.4.1-7)

If a node receives a delete request for SAs for which it has already issued a delete request, it MUST delete the outgoing SAs while processing the request and the incoming SAs while processing the response (§1.4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDelWithoutOwnDeleteTheSameRequestIsPaired`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_delete_test.go#L277) | unit/verify | unproven |
| positive | [`TestDelCrossingDeleteAnswersWithoutAPairedDelete`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_delete_test.go#L159) | unit/verify | unproven |

### [`RFC7296-1.5-1`](#rfc7296-1.5-1)

This message is not part of an INFORMATIONAL exchange, and the receiving node MUST NOT respond to it because doing so could cause a message loop (§1.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestWp2OutOfSAAnswersARequest`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_wp2_test.go#L44) | unit/verify | unproven |
| positive | [`TestWp2OutOfSAEmitterIsAFixedPoint`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_wp2_test.go#L21) | unit/verify | unproven |

### [`RFC7296-2.12-1`](#rfc7296-2.12-1)

Achieving perfect forward secrecy requires that when a connection is closed, each endpoint MUST forget not only the keys used by the connection but also any information that could be used to recompute those keys (§2.12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestWp2OpenSAKeepsItsKeys`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_wp2_test.go#L116) | unit/verify | unproven |
| positive | [`TestRunEstablishedClearsPendingIKESwapOnExit`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/established_test.go#L492) | unit/verify | unproven |
| positive | [`TestWp2ForgetKeysErasesEverySecret`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_wp2_test.go#L85) | unit/verify | unproven |

### [`RFC7296-2.24-1`](#rfc7296-2.24-1)

tunnel encapsulators and decapsulators for all tunnel mode SAs created by IKEv2 MUST support the ECN full-functionality option for tunnels specified in [ECN] (§2.24)

Audit verdict: enforced (the tests do what the requirement demands), fresh. installed Child SAs are tunnel mode with no ECN-disabling field (engine and XFRM scans); VPP InstallSA sets ENCAP/DECAP_COPY_ECN. Negatives are non-vacuity checks rather than violating inputs

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEcnTheScannedStateIsTheOneZeInstalls`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc7296_ecn_linux_test.go#L109) | unit/verify | unproven |
| negative | [`TestVPPInstallSAECNIsOnTheSAZeInstalls`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/vpp_message_test.go#L961) | unit/verify | unproven |
| negative | [`TestEcnTheInstalledSAsAreRealAndDirectional`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_ecn_test.go#L91) | unit/verify | unproven |
| positive | [`TestEcnInstalledStateDisablesNothing`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc7296_ecn_linux_test.go#L86) | unit/verify | unproven |
| positive | [`TestVPPInstallSACopiesECN`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/vpp_message_test.go#L937) | unit/verify | unproven |
| positive | [`TestEcnInstalledChildSAAsksForNoECNChange`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_ecn_test.go#L70) | unit/verify | unproven |

### [`RFC7296-2.24-2`](#rfc7296-2.24-2)

MUST implement the tunnel encapsulation and decapsulation processing specified in [IPSECARCH] to prevent discarding of ECN congestion indications. (§2.24)

Audit verdict: enforced (the tests do what the requirement demands), fresh. same units: XFRM state carries no NOECN path, VPP sets DECAP_COPY_ECN; transport mode refused by VPP backend

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEcnTheScannedStateIsTheOneZeInstalls`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc7296_ecn_linux_test.go#L113) | unit/verify | unproven |
| negative | [`TestVPPInstallSAECNIsOnTheSAZeInstalls`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/vpp_message_test.go#L965) | unit/verify | unproven |
| negative | [`TestEcnTheInstalledSAsAreRealAndDirectional`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_ecn_test.go#L95) | unit/verify | unproven |
| positive | [`TestEcnInstalledStateDisablesNothing`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc7296_ecn_linux_test.go#L90) | unit/verify | unproven |
| positive | [`TestVPPInstallSACopiesECN`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/vpp_message_test.go#L941) | unit/verify | unproven |
| positive | [`TestEcnInstalledChildSAAsksForNoECNChange`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_ecn_test.go#L74) | unit/verify | unproven |

### [`RFC7296-2.16-12`](#rfc7296-2.16-12)

For EAP methods that create a shared key as a side effect of authentication, that shared key MUST be used by both the initiator and responder to generate AUTH payloads in messages 7 and 8 using the syntax for shared secrets specified in Section 2.15 (§2.16)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEapAuthProducerOutputIsRefusedUnderAnotherKey`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_eap_auth_producer_test.go#L82) | unit/verify | unproven |
| positive | [`TestEapAuthProducerIsKeyedByTheNegotiatedMSK`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_eap_auth_producer_test.go#L43) | unit/verify | unproven |

### [`RFC7296-2.16-13`](#rfc7296-2.16-13)

Following such an extended exchange, the EAP AUTH payloads MUST be included in the two messages following the one containing the EAP Success message (§2.16)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEapAuthFollowsTheSuccessMessage`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_eap_auth_producer_test.go#L235) | unit/verify | unproven |
| positive | [`TestEapAuthFollowsTheSuccessMessage`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_eap_auth_producer_test.go#L232) | unit/verify | unproven |

### [`RFC7296-2.16-14`](#rfc7296-2.16-14)

Once the protocol exchange defined by the chosen EAP authentication method has successfully terminated, the responder MUST send an EAP payload containing the Success message (§2.16)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEapResultFailureIsSent`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc7296_eap_result_test.go#L81) | unit/verify | unproven |
| positive | [`TestEapResultSuccessIsSent`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc7296_eap_result_test.go#L55) | unit/verify | unproven |

### [`RFC7296-2.16-15`](#rfc7296-2.16-15)

Similarly, if the authentication method has failed, the responder MUST send an EAP payload containing the Failure message (§2.16)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEapResultSuccessIsNotFailure`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc7296_eap_result_test.go#L101) | unit/verify | unproven |
| positive | [`TestEapResultFailureIsSent`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc7296_eap_result_test.go#L78) | unit/verify | unproven |

### [`RFC7296-3.1-13`](#rfc7296-3.1-13)

* I (Initiator) - This bit MUST be set in messages sent by the original initiator of the IKE SA and MUST be cleared in messages sent by the original responder. (§3.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. only the DPD probe is asserted (TestWp2DPDProbeIBitFollowsRole/DiffersByRole); flags are passed per call site (buildEncryptedMessageEx flags argument), so one message kind does not prove every message the original initiator/responder sends

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestWp2DPDProbeIBitDiffersByRole`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_wp2_test.go#L276) | unit/verify | unproven |
| positive | [`TestWp2DPDProbeIBitFollowsRole`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_wp2_test.go#L259) | unit/verify | unproven |

### [`RFC7296-3.5-5`](#rfc7296-3.5-5)

The string MUST NOT contain any terminators (e.g., NULL, CR, etc.). All characters in the ID_FQDN are ASCII; for an "internationalized domain name", the syntax is as defined in [IDNA], for example "xn--tmonesimerkki-bfbb.example.net". ID_RFC822_ADDR 3 A fully-qualified RFC 822 email address string. An example of a ID_RFC822_ADDR is "jsmith@example.com". The string MUST NOT contain any terminators. (§3.5)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Clauses: the ID_FQDN string MUST NOT contain terminators; all characters are ASCII, with IDNA syntax for internationalized names. The terminator clause is asserted (refuseIDTerminators refuses NUL/CR/LF in ID_FQDN and ID_RFC822_ADDR, ValidateIdentities refuses them in config; clean ids accepted). The ASCII/IDNA clause has no assertion: a non-ASCII FQDN is neither sent nor refused in any tagged unit.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestWp2IDWithoutTerminatorAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_wp2_test.go#L339) | unit/verify | unproven |
| positive | [`TestWp2IDTerminatorRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_wp2_test.go#L299) | unit/verify | unproven |

### [`RFC7296-1.4.1-1`](#rfc7296-1.4.1-1)

When an SA is closed, both members of the pair MUST be closed (that is, deleted). Each endpoint MUST close its incoming SAs and allow the other endpoint to close the other SA in each pair (§1.4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLcyClosingAChildSAClosesBothHalvesAndDeletesOurInbound`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_lifecycle_test.go#L236) | unit/verify | unproven |
| positive | [`TestLcyClosingAChildSAClosesBothHalvesAndDeletesOurInbound`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_lifecycle_test.go#L232) | unit/verify | unproven |

### [`RFC7296-1.4.1-4`](#rfc7296-1.4.1-4)

In that case, the responses MUST NOT include Delete payloads for the deleted SAs, since that would result in duplicate deletion and could in theory delete the wrong SA. (§1.4.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestLcyInformationalResponseCarriesNoDeletePayload never sets up crossed deletes (the exception the MUST NOT governs); its premise that ze attaches no Delete to any response is now false (TestDelResponseCarriesThePairedDelete), so the comment is stale and the unit asserts an unknown-SPI Delete draws none

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLcyInformationalResponseCarriesNoDeletePayload`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_lifecycle_test.go#L294) | unit/verify | unproven |
| positive | [`TestLcyInformationalResponseCarriesNoDeletePayload`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_lifecycle_test.go#L289) | unit/verify | unproven |

### [`RFC7296-1.4.1-5`](#rfc7296-1.4.1-5)

A node MAY refuse to accept incoming data on half-closed connections but MUST NOT unilaterally close them and reuse the SPIs (§1.4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLcyRetiredSPIsAreNeverReused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_lifecycle_test.go#L348) | unit/verify | unproven |
| positive | [`TestLcyRetiredSPIsAreNeverReused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_lifecycle_test.go#L344) | unit/verify | unproven |

### [`RFC7296-2.4-9`](#rfc7296-2.4-9)

If a system creates Child SAs that can fail independently from one another without the associated IKE SA being able to send a delete message, then the system MUST negotiate such Child SAs using separate IKE SAs (§2.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLcyOneChildSALivesUnderOneIKESA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_lifecycle_test.go#L422) | unit/verify | unproven |
| positive | [`TestLcyOneChildSALivesUnderOneIKESA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_lifecycle_test.go#L417) | unit/verify | unproven |

### [`RFC7296-2.4-10`](#rfc7296-2.4-10)

If an IKE endpoint chooses to delete Child SAs, it MUST send Delete payloads to the other end notifying it of the deletion (§2.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLcyRetiringAChildSASendsADeletePayload`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_lifecycle_test.go#L456) | unit/verify | unproven |
| positive | [`TestLcyRetiringAChildSASendsADeletePayload`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_lifecycle_test.go#L453) | unit/verify | unproven |

### [`RFC7296-2.16-5`](#rfc7296-2.16-5)

If EAP methods that do not generate a shared key are used, the AUTH payloads in messages 7 and 8 MUST be generated using SK_pi and SK_pr, respectively (§2.16)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEAPAuthOfKeyDerivingMethodStillUsesTheMSK`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_eap_nonkeying_auth_test.go#L174) | unit/verify | revert, verified |
| positive | [`TestEAPAuthOfNonKeyDerivingMethodUsesSKpiAndSKpr`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_eap_nonkeying_auth_test.go#L126) | unit/verify | revert, verified |

### [`RFC7296-2.16-16`](#rfc7296-2.16-16)

EAP methods that do not establish a shared key SHOULD NOT be used, as they are subject to a number of man-in-the-middle attacks [EAPMITM] if these EAP methods are used in other protocols that do not use a server-authenticated tunnel. (§2.16)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Non-compliant behaviour: an IKEv2 exchange selecting an EAP method that establishes no key when the operator did not name one. Positive: TestRFC3748IKEv2EAPModesSelectAKeyDerivingMethod fails ('auth mode %s selected an EAP method that derives no key') for any EAP mode other than AuthEAPMD5 whose newEAPSession session answers false to DerivesKey. Negative: TestRFC3748IKEv2NoAuthModeSelectsAKeylessMethod sweeps every AuthMode value through newEAPSession and startEAPExchange, and assertKeylessModeIsNamedAndAnnounced goes red when a keyless method is reached by an unnamed value or without the adoption warning. The SHOULD NOT is met as a deviation the operator must name, which the warning makes visible.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3748IKEv2NoAuthModeSelectsAKeylessMethod`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc3748_ikev2_method_selection_test.go#L171) | unit/verify | revert, verified |
| positive | [`TestRFC3748IKEv2EAPModesSelectAKeyDerivingMethod`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc3748_ikev2_method_selection_test.go#L120) | unit/verify | revert, verified |

### [`RFC7296-3.4-1`](#rfc7296-3.4-1)

The length of the Diffie-Hellman public value for MODP groups MUST be equal to the length of the prime modulus over which the exponentiation was performed, prepending zero bits to the value if necessary (§3.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7296MODPShortPublicValueIsRefusedOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_dh_test.go#L91) | unit/verify | unproven |
| positive | [`TestRFC7296MODPPublicValueMatchesModulusLength`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_dh_test.go#L30) | unit/verify | unproven |

### [`RFC7296-1.3-1`](#rfc7296-1.3-1)

If a CREATE_CHILD_SA exchange includes a KEi payload, at least one of the SA offers MUST include the Diffie-Hellman group of the KEi (§1.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestChildRekeyProposesDiffieHellmanWhenPFSIsEnabled`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_rekey_pfs_test.go#L66) | unit/verify | unproven |
| negative | [`TestChildRekeyProposesNoDiffieHellmanWhenPFSIsDisabled`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_rekey_pfs_test.go#L100) | unit/verify | revert, verified |
| negative | [`TestRkyIKERekeyOffersTheKEiGroup`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_rekey_test.go#L171) | unit/verify | unproven |
| positive | [`TestChildRekeyProposesDiffieHellmanWhenPFSIsEnabled`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/child_rekey_pfs_test.go#L62) | unit/verify | revert, verified |
| positive | [`TestRkyIKERekeyOffersTheKEiGroup`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_rekey_test.go#L167) | unit/verify | unproven |

### [`RFC7296-2.1-3`](#rfc7296-2.1-3)

The responder MUST never retransmit a response unless it receives a retransmission of the request (§2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEapRtxMidEAPReplayRefusesUnprotected`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_eap_retransmit_test.go#L150) | unit/verify | revert, verified |
| negative | [`TestRtxResponderReplaysCachedResponseOnlyForDuplicate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_retransmit_test.go#L209) | unit/verify | revert, verified |
| positive | [`TestEapRtxResponderReplaysCachedResponseMidEAP`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_eap_retransmit_test.go#L96) | unit/verify | revert, verified |
| positive | [`TestRtxResponderReplaysCachedResponseOnlyForDuplicate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_retransmit_test.go#L206) | unit/verify | revert, verified |

### [`RFC7296-2.1-4`](#rfc7296-2.1-4)

In that event, the responder MUST ignore the retransmitted request except insofar as it causes a retransmission of the response (§2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEapRtxResponderReplaysCachedResponseMidEAP`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_eap_retransmit_test.go#L93) | unit/verify | revert, verified |
| negative | [`TestRtxResponderReplaysCachedResponseOnlyForDuplicate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_retransmit_test.go#L213) | unit/verify | revert, verified |
| positive | [`TestEapRtxResponderReplaysCachedResponseMidEAP`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_eap_retransmit_test.go#L89) | unit/verify | revert, verified |
| positive | [`TestRtxResponderReplaysCachedResponseOnlyForDuplicate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_retransmit_test.go#L211) | unit/verify | revert, verified |

### [`RFC7296-2.1-5`](#rfc7296-2.1-5)

The initiator MUST remember each request until it receives the corresponding response. The responder MUST remember each response until it receives a request whose sequence number is larger than or equal to the sequence number in the response plus its window size (§2.1, §2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRtxEachSideRemembersWhatItSent`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_retransmit_test.go#L150) | unit/verify | unproven |
| negative | [`TestWinDeleteIsRememberedUntilAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_window_test.go#L424) | unit/verify | revert, verified |
| positive | [`TestRtxEachSideRemembersWhatItSent`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_retransmit_test.go#L147) | unit/verify | unproven |
| positive | [`TestWinDeleteIsRememberedUntilAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_window_test.go#L419) | unit/verify | revert, verified |

### [`RFC7296-2.1-6`](#rfc7296-2.1-6)

If the responder receives a retransmitted request for which it has already forgotten the response, it MUST ignore the request (and not, for example, attempt constructing a new response) (§2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRtxResponderIgnoresRequestWithForgottenResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_retransmit_test.go#L275) | unit/verify | revert, verified |
| positive | [`TestRtxResponderIgnoresRequestWithForgottenResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_retransmit_test.go#L272) | unit/verify | revert, verified |

### [`RFC7296-2.1-7`](#rfc7296-2.1-7)

IKE is a reliable protocol: the initiator MUST retransmit a request until it either receives a corresponding response or deems the IKE SA to have failed (§2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRtxInitiatorResendsUnansweredRekeyRequest`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_retransmit_test.go#L334) | unit/verify | revert, verified |
| negative | [`TestRtxInitiatorResendsUnansweredSAInit`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_retransmit_test.go#L398) | unit/verify | unproven |
| negative | [`TestWinUnansweredRequestFailsTheSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_window_test.go#L331) | unit/verify | revert, verified |
| positive | [`TestRtxInitiatorResendsUnansweredRekeyRequest`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_retransmit_test.go#L331) | unit/verify | revert, verified |
| positive | [`TestRtxInitiatorResendsUnansweredSAInit`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_retransmit_test.go#L395) | unit/verify | unproven |
| positive | [`TestWinUnansweredRequestFailsTheSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_window_test.go#L326) | unit/verify | revert, verified |

### [`RFC7296-2.1-8`](#rfc7296-2.1-8)

A retransmission from the initiator MUST be bitwise identical to the original request (§2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRtxRetransmissionIsBitwiseIdenticalAndReusesMessageID`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_retransmit_test.go#L448) | unit/verify | revert, verified |
| positive | [`TestRtxRetransmissionIsBitwiseIdenticalAndReusesMessageID`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_retransmit_test.go#L446) | unit/verify | revert, verified |

### [`RFC7296-2.2-1`](#rfc7296-2.2-1)

Retransmission of a message MUST use the same Message ID as the original message (§2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRtxRetransmissionIsBitwiseIdenticalAndReusesMessageID`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_retransmit_test.go#L452) | unit/verify | revert, verified |
| positive | [`TestRtxRetransmissionIsBitwiseIdenticalAndReusesMessageID`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_retransmit_test.go#L450) | unit/verify | revert, verified |

### [`RFC7296-2.2-2`](#rfc7296-2.2-2)

In the unlikely event that Message IDs grow too large to fit in 32 bits, the IKE SA MUST be closed or rekeyed (§2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMidInboundCounterFreezesAtTheCeiling`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_msgid_test.go#L217) | unit/verify | unproven |
| negative | [`TestMidNearExhaustionRekeysTheIKESA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_msgid_test.go#L118) | unit/verify | revert, verified |
| negative | [`TestMidOutboundCounterFreezesAtTheCeiling`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_msgid_test.go#L49) | unit/verify | revert, verified |
| negative | [`TestMidResponderEstablishDoesNotWrapTheCounter`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_msgid_test.go#L461) | unit/verify | unproven |
| positive | [`TestMidInboundCounterFreezesAtTheCeiling`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_msgid_test.go#L211) | unit/verify | unproven |
| positive | [`TestMidNearExhaustionRekeysTheIKESA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_msgid_test.go#L112) | unit/verify | revert, verified |
| positive | [`TestMidOutboundCounterFreezesAtTheCeiling`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_msgid_test.go#L42) | unit/verify | revert, verified |
| positive | [`TestMidResponderEstablishDoesNotWrapTheCounter`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_msgid_test.go#L456) | unit/verify | unproven |

### [`RFC7296-2.2-3`](#rfc7296-2.2-3)

Each endpoint in the IKE Security Association maintains two "current" Message IDs: the next one to be used for a request it initiates and the next one it expects to see in a request from the other end. (§2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. finishResponderEstablish advances only the inbound counter; responder's first request id 0 is classified as new by the initiator

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMidResponderEstablishDoesNotWrapTheCounter`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_msgid_test.go#L466) | unit/verify | unproven |
| positive | [`TestResponderFirstRequestMatchesWhatTheInitiatorExpects`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_msgid_responder_request_test.go#L35) | unit/verify | unproven |

### [`RFC7296-2.3-2`](#rfc7296-2.3-2)

An IKE endpoint MUST wait for a response to each of its messages before sending a subsequent message unless it has received a SET_WINDOW_SIZE Notify message from its peer (§2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDPDNoTransportTakesNoWindow`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/dpd_test.go#L106) | unit/verify | unproven |
| negative | [`TestWinOneRequestPerTick`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_window_test.go#L89) | unit/verify | revert, verified |
| negative | [`TestWinTeardownDoesNotHang`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_window_test.go#L537) | unit/verify | revert, verified |
| positive | [`TestWinOneRequestPerTick`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_window_test.go#L84) | unit/verify | revert, verified |
| positive | [`TestWinTeardownDoesNotHang`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_window_test.go#L531) | unit/verify | revert, verified |

### [`RFC7296-2.3-4`](#rfc7296-2.3-4)

An IKE endpoint MUST NOT exceed the peer's stated window size for transmitted IKE requests (§2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestWinResponseReleasesSlot`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_window_test.go#L171) | unit/verify | revert, verified |
| positive | [`TestWinResponseReleasesSlot`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_window_test.go#L165) | unit/verify | revert, verified |

### [`RFC7296-2.3-5`](#rfc7296-2.3-5)

This Notify message MUST NOT be sent in a response; the invalid request MUST NOT be acknowledged (§2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOsrOutOfWindowRequestIsNotAcknowledged`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_outstanding_test.go#L188) | unit/verify | unproven |
| positive | [`TestImiNotificationCarriesTheFourOctetMessageID`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_invalidmsgid_test.go#L199) | unit/verify | unproven |
| positive | [`TestOsrOutOfWindowRequestIsNotAcknowledged`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_outstanding_test.go#L182) | unit/verify | unproven |

### [`RFC7296-2.3-7`](#rfc7296-2.3-7)

The data associated with a SET_WINDOW_SIZE notification MUST be 4 octets long and contain the big endian representation of the number of messages the sender promises to keep (§2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSwzMalformedWindowSizeIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_setwindow_test.go#L139) | unit/verify | unproven |
| negative | [`TestSwzPeerWindowSizeIsRead`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_setwindow_test.go#L104) | unit/verify | unproven |
| negative | [`TestNtfySetWindowSizeDataIsFourOctets`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_notify_test.go#L167) | unit/verify | unproven |
| positive | [`TestSwzPeerWindowSizeIsRead`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_setwindow_test.go#L97) | unit/verify | unproven |
| positive | [`TestNtfySetWindowSizeDataIsFourOctets`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_notify_test.go#L161) | unit/verify | unproven |

### [`RFC7296-2.3-8`](#rfc7296-2.3-8)

An IKE endpoint MUST be prepared to accept and process a request while it has a request outstanding in order to avoid a deadlock in this situation (§2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestImiHeldWindowSuppressesTheNotification`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_invalidmsgid_test.go#L257) | unit/verify | unproven |
| negative | [`TestOsrOwnerLoopKeepsAForeignWindowHeld`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_outstanding_test.go#L270) | unit/verify | unproven |
| negative | [`TestOsrRequestAcceptedWhileOursIsOutstanding`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_outstanding_test.go#L58) | unit/verify | unproven |
| negative | [`TestOsrRetireOnlyFreesItsOwnWindow`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_outstanding_test.go#L353) | unit/verify | unproven |
| positive | [`TestOsrOwnerLoopRetiresTheStrandedWindow`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_outstanding_test.go#L134) | unit/verify | unproven |
| positive | [`TestOsrRequestAcceptedWhileOursIsOutstanding`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_outstanding_test.go#L51) | unit/verify | unproven |

### [`RFC7296-2.3-9`](#rfc7296-2.3-9)

Sending this notification is OPTIONAL, and notifications of this type MUST be rate limited (§2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestImiHeldWindowSuppressesTheNotification`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_invalidmsgid_test.go#L253) | unit/verify | unproven |
| negative | [`TestImiUnauthenticatedRequestDrawsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_invalidmsgid_test.go#L143) | unit/verify | unproven |
| positive | [`TestImiNotificationCarriesTheFourOctetMessageID`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_invalidmsgid_test.go#L192) | unit/verify | unproven |
| positive | [`TestImiRateLimitCapsTheNotification`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_invalidmsgid_test.go#L94) | unit/verify | unproven |

### [`RFC7296-2.25-1`](#rfc7296-2.25-1)

When a peer receives a TEMPORARY_FAILURE notification, it MUST NOT immediately retry the operation; it MUST wait so that the sender may complete whatever operation caused the temporary condition (§2.25)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMidTemporaryFailureDefersTheIKERekey`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_msgid_test.go#L386) | unit/verify | revert, verified |
| negative | [`TestMidTemporaryFailureDefersTheRetry`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_msgid_test.go#L276) | unit/verify | revert, verified |
| positive | [`TestMidTemporaryFailureDefersTheIKERekey`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_msgid_test.go#L382) | unit/verify | revert, verified |
| positive | [`TestMidTemporaryFailureDefersTheRetry`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_msgid_test.go#L268) | unit/verify | revert, verified |

### [`RFC7296-2.8-5`](#rfc7296-2.8-5)

If an SA has expired or is about to expire and rekeying attempts using the mechanisms described here fail, an implementation MUST close the IKE SA and any associated Child SAs and then MAY start new ones (§2.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRkyExhaustedRekeyClosesTheChildSAs`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_rekey_test.go#L241) | unit/verify | unproven |
| positive | [`TestRkyExhaustedRekeyClosesTheChildSAs`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_rekey_test.go#L238) | unit/verify | unproven |

### [`RFC7296-2.8-6`](#rfc7296-2.8-6)

After the new equivalent IKE SA is created, the initiator deletes the old IKE SA, and the Delete payload to delete itself MUST be the last request sent over the old IKE SA (§2.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRkyIKERekeyDeleteIsTheLastRequestOnTheOldSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_rekey_test.go#L306) | unit/verify | revert, verified |
| positive | [`TestRkyIKERekeyDeleteIsTheLastRequestOnTheOldSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_rekey_test.go#L303) | unit/verify | revert, verified |

### [`RFC7296-2.8-7`](#rfc7296-2.8-7)

The responder to a CREATE_CHILD_SA MUST be prepared to accept messages on an SA before sending its response to the creation request, so there is no ambiguity for the initiator (§2.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRkyResponderInstallsTheNewChildBeforeItAnswers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_rekey_test.go#L415) | unit/verify | revert, verified |
| positive | [`TestRkyResponderInstallsTheNewChildBeforeItAnswers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_rekey_test.go#L412) | unit/verify | revert, verified |

### [`RFC7296-2.8.1-1`](#rfc7296-2.8.1-1)

When there are two SAs eligible to receive packets, a node MUST accept incoming packets through either SA (§2.8.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRkyOldAndNewChildBothReceiveUntilThePeerDeletes`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_rekey_test.go#L528) | unit/verify | unproven |
| positive | [`TestRkyOldAndNewChildBothReceiveUntilThePeerDeletes`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_rekey_test.go#L525) | unit/verify | unproven |

### [`RFC7296-2.18-2`](#rfc7296-2.18-2)

The new IKE SA MUST reset its message counters to 0 (§2.18)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRtxRekeyedIKESAResetsMessageCounters`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_retransmit_test.go#L509) | unit/verify | unproven |
| positive | [`TestRtxRekeyedIKESAResetsMessageCounters`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_retransmit_test.go#L506) | unit/verify | unproven |

### [`RFC7296-2.18-3`](#rfc7296-2.18-3)

implementations MUST perform a new Diffie-Hellman exchange when rekeying the IKE SA. In other words, an initiator MUST NOT propose the value "NONE" for the Diffie-Hellman transform, and a responder MUST NOT accept such a proposal. (§2.18)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestPropDHNoneRefusedForIKESA covers the responder refusing DH NONE in NegotiateIKE; the initiator half (MUST NOT propose NONE when rekeying) and the rekey context are not asserted

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPropDHNoneRefusedForIKESA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L309) | unit/verify | unproven |
| positive | [`TestPropDHNoneRefusedForIKESA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L313) | unit/verify | unproven |

### [`RFC7296-3.16-1`](#rfc7296-3.16-1)

In a response message, this octet MUST be set to match the identifier in the corresponding request. (§3.16)

Audit verdict: enforced (the tests do what the requirement demands), fresh. EAP peer copies request Identifier into each response over three rounds with unrelated ids

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEapfmtResponseIdentifierMatchesRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc7296_eap_test.go#L25) | unit/verify | unproven |
| positive | [`TestEapfmtResponseIdentifierMatchesRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc7296_eap_test.go#L20) | unit/verify | unproven |

### [`RFC7296-3.16-2`](#rfc7296-3.16-2)

MUST be four less than the Payload Length of the encapsulating payload. (§3.16)

Audit verdict: enforced (the tests do what the requirement demands), fresh. PayloadEAP EAP Length equals Payload Length minus four across five body sizes

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEapfmtEAPLengthIsFourLessThanPayloadLength`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_eap_test.go#L52) | unit/verify | unproven |
| positive | [`TestEapfmtEAPLengthIsFourLessThanPayloadLength`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_eap_test.go#L47) | unit/verify | unproven |

### [`RFC7296-3.16-3`](#rfc7296-3.16-3)

For other codes, the EAP message length MUST be four octets and the Type and Type_Data fields MUST NOT be present. (§3.16)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Success/Failure encode to four octets with no Type; Request keeps Type

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEapfmtSuccessAndFailureCarryNoTypeField`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc7296_eap_test.go#L88) | unit/verify | unproven |
| positive | [`TestEapfmtSuccessAndFailureCarryNoTypeField`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc7296_eap_test.go#L84) | unit/verify | unproven |

### [`RFC7296-3.16-4`](#rfc7296-3.16-4)

In a Response (2) message, Type MUST either be Nak or match the type of the data requested (§3.16)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEapfmtPeerResponseTypeIsNakOrMatchesRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc7296_eap_test.go#L122) | unit/verify | unproven |
| positive | [`TestEapfmtPeerResponseTypeIsNakOrMatchesRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc7296_eap_test.go#L118) | unit/verify | unproven |

### [`RFC7296-1.7-2`](#rfc7296-1.7-2)

This document adds the restriction in Section 2.13 that all pseudorandom functions (PRFs) used with IKEv2 MUST take variable- sized keys. (§1.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. HMAC PRFs accept 1..200-octet keys and consume the key

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPRFTakesVariableSizedKeys`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_test.go#L20) | unit/verify | unproven |
| positive | [`TestPRFTakesVariableSizedKeys`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_test.go#L16) | unit/verify | unproven |

### [`RFC7296-2-1`](#rfc7296-2-1)

All IKEv2 implementations MUST be able to send, receive, and process IKE messages that are up to 1280 octets long (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMessageHandles1280Octets`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L56) | unit/verify | unproven |
| positive | [`TestMessageHandles1280Octets`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L52) | unit/verify | unproven |

### [`RFC7296-2.5-1`](#rfc7296-2.5-1)

The minor version number indicates new capabilities, and MUST be ignored by a node with a smaller minor version number, but used for informational purposes by the node with the larger minor version number (§2.5, §3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMinorVersionIgnoredMajorIsNot`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L104) | unit/verify | unproven |
| positive | [`TestMinorVersionIgnoredMajorIsNot`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L100) | unit/verify | unproven |

### [`RFC7296-2.5-2`](#rfc7296-2.5-2)

If an endpoint receives a message with a higher major version number, it MUST drop the message (§2.5, §3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestHigherMajorVersionDropped`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L119) | unit/verify | unproven |
| positive | [`TestHigherMajorVersionDropped`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L116) | unit/verify | unproven |

### [`RFC7296-2.5-6`](#rfc7296-2.5-6)

Also, for forward compatibility, all fields marked RESERVED MUST be set to zero by an implementation running version 2.0 (§2.5, §3.2, §3.3.1, §3.3.2, §3.5, §3.8, §3.13, §3.15, §3.15.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestConfigAttributeReservedBitSentAsZero`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_cp_test.go#L130) | unit/verify | unproven |
| negative | [`TestReservedFieldsSentAsZero`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L151) | unit/verify | unproven |
| positive | [`TestConfigAttributeReservedBitSentAsZero`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_cp_test.go#L125) | unit/verify | unproven |
| positive | [`TestReservedFieldsSentAsZero`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L145) | unit/verify | unproven |

### [`RFC7296-2.5-7`](#rfc7296-2.5-7)

their content MUST be ignored by an implementation running version 2.0 (§2.5)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. reserved-bit ignore is asserted for the generic payload header flags and the CP reserved bit only; the quote covers every RESERVED field (header, proposal, transform, ID, AUTH, TS, etc.)

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestConfigAttributeReservedBitIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_cp_test.go#L48) | unit/verify | unproven |
| negative | [`TestReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L213) | unit/verify | unproven |
| positive | [`TestConfigAttributeReservedBitIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_cp_test.go#L38) | unit/verify | unproven |
| positive | [`TestReservedFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L209) | unit/verify | unproven |

### [`RFC7296-2.5-8`](#rfc7296-2.5-8)

Similarly, payload types that are not defined are reserved for future use; implementations of a version where they are undefined MUST skip over those payloads and ignore their contents. (§2.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. undefined payload type decoded as payloadRaw and the chain continues, outer and inner chain; defined type not demoted

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInnerChainSkipsUndefinedType`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_innerchain_test.go#L144) | unit/verify | unproven |
| negative | [`TestUndefinedPayloadTypeSkipped`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L265) | unit/verify | unproven |
| positive | [`TestInnerChainSkipsUndefinedType`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_innerchain_test.go#L141) | unit/verify | unproven |
| positive | [`TestUndefinedPayloadTypeSkipped`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L261) | unit/verify | unproven |

### [`RFC7296-2.5-9`](#rfc7296-2.5-9)

If the critical flag is set and the payload type is unrecognized, the message MUST be rejected (§2.5, §4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInnerChainRejectsCriticalUnrecognized`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_innerchain_test.go#L69) | unit/verify | unproven |
| negative | [`TestCriticalUnrecognizedPayloadRejected`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L308) | unit/verify | unproven |
| positive | [`TestInnerChainRejectsCriticalUnrecognized`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_innerchain_test.go#L65) | unit/verify | unproven |
| positive | [`TestCriticalUnrecognizedPayloadRejected`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L305) | unit/verify | unproven |

### [`RFC7296-2.5-11`](#rfc7296-2.5-11)

If the critical flag is not set and the payload type is unsupported, that payload MUST be ignored (§2.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInnerChainIgnoresNonCriticalUnsupported`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_innerchain_test.go#L98) | unit/verify | unproven |
| negative | [`TestNonCriticalUnsupportedPayloadIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L335) | unit/verify | unproven |
| positive | [`TestInnerChainIgnoresNonCriticalUnsupported`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_innerchain_test.go#L95) | unit/verify | unproven |
| positive | [`TestNonCriticalUnsupportedPayloadIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L332) | unit/verify | unproven |

### [`RFC7296-2.5-13`](#rfc7296-2.5-13)

implementations MUST NOT reject as invalid a message with those payloads in any other order. (§2.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Nonce/KE/SA order parses with all three; engine handleAuthResponse accepts unconventional order

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPodAuthResponseAcceptsAuthBeforeIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_payload_order_test.go#L99) | unit/verify | unproven |
| negative | [`TestPayloadOrderNotRejected`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L372) | unit/verify | unproven |
| positive | [`TestPodAuthResponseAcceptsAuthBeforeIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_payload_order_test.go#L97) | unit/verify | unproven |
| positive | [`TestPayloadOrderNotRejected`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L369) | unit/verify | unproven |

### [`RFC7296-2.5-14`](#rfc7296-2.5-14)

If an endpoint supports major version n, and major version m, it MUST support all versions between n and m (§2.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNATTDispatchAppliesTheSameVersionGate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_version_test.go#L245) | unit/verify | unproven |
| positive | [`TestSupportedMajorVersionSetIsSingleton`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_version_test.go#L213) | unit/verify | unproven |

### [`RFC7296-2.5-15`](#rfc7296-2.5-15)

If it receives a message with a major version that it supports, it MUST respond with that version number (§2.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestResponderEchoesTheSupportedMajorVersion`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_version_test.go#L262) | unit/verify | unproven |
| positive | [`TestResponderEchoesTheSupportedMajorVersion`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_version_test.go#L256) | unit/verify | unproven |

### [`RFC7296-2.5-16`](#rfc7296-2.5-16)

If they mistakenly (perhaps through an active attacker sending error messages) negotiate to version n, then both will notice that the other side can support a higher version number, and they MUST break the connection and reconnect using version n+1 (§2.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNATTDispatchAppliesTheSameVersionGate`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_version_test.go#L249) | unit/verify | unproven |
| positive | [`TestSupportedMajorVersionSetIsSingleton`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_version_test.go#L218) | unit/verify | unproven |

### [`RFC7296-2.5-17`](#rfc7296-2.5-17)

Payloads sent in IKE response messages MUST NOT have the critical flag set (§2.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestResponsePayloadsAreNeverCritical`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_critical_bit_test.go#L189) | unit/verify | unproven |
| positive | [`TestResponsePayloadsAreNeverCritical`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_critical_bit_test.go#L183) | unit/verify | unproven |

### [`RFC7296-2.5-18`](#rfc7296-2.5-18)

the response to the IKE request containing that payload MUST include a Notify payload UNSUPPORTED_CRITICAL_PAYLOAD, indicating an unsupported critical payload was included. In that Notify payload, the Notification Data contains the one-octet payload type. (§2.5)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestCritUnknownCriticalPayloadNamesItsType asserts the parse error carries the payload type; no tagged unit asserts the RESPONSE carries an UNSUPPORTED_CRITICAL_PAYLOAD Notify whose data is that one-octet type (producer inbound.go:289 untested)

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCritUnknownCriticalPayloadNamesItsType`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_critpayload_test.go#L206) | unit/verify | unproven |
| positive | [`TestCritUnknownCriticalPayloadNamesItsType`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_critpayload_test.go#L201) | unit/verify | unproven |

### [`RFC7296-2.21.2-1`](#rfc7296-2.21.2-1)

Note, however, that request messages that contain an unsupported critical payload, or where the whole message is malformed (rather than just bad payload contents), MUST be rejected in their entirety, and MUST only lead to an UNSUPPORTED_CRITICAL_PAYLOAD or INVALID_SYNTAX Notification sent as a response. (§2.21.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Clauses: a request with an unsupported critical payload, or a wholly malformed request, is rejected in its entirety and draws only UNSUPPORTED_CRITICAL_PAYLOAD or INVALID_SYNTAX. The malformed half is asserted (truncated inner chain answered with INVALID_SYNTAX only; bad payload contents not treated as malformed). The unsupported-critical-payload half has no assertion in the tagged units: no request carrying one is shown to draw exactly an UNSUPPORTED_CRITICAL_PAYLOAD response (see RFC7296-2.5-18, weak).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCritChainReportsTruncationButNotBadContents`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_critpayload_test.go#L284) | unit/verify | unproven |
| positive | [`TestErrInnerParseFailureDrawsInvalidSyntaxAndOuterDrawsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L160) | unit/verify | unproven |
| positive | [`TestCritChainReportsTruncationButNotBadContents`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_critpayload_test.go#L278) | unit/verify | unproven |

### [`RFC7296-2.21.2-2`](#rfc7296-2.21.2-2)

Specifically, a responder may include all the payloads associated with authentication (IDr, CERT, and AUTH) while sending error notifications for the piggybacked exchanges (FAILED_CP_REQUIRED, NO_PROPOSAL_CHOSEN, and so on), and the initiator MUST NOT fail the authentication because of this. (§2.21.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. initiator establishes despite piggybacked error notify; AUTHENTICATION_FAILED still kills the SA

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestErrInitiatorSurvivesPiggybackedErrorNotify`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L406) | unit/verify | unproven |
| positive | [`TestErrInitiatorSurvivesPiggybackedErrorNotify`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L396) | unit/verify | unproven |

### [`RFC7296-2.21.2-3`](#rfc7296-2.21.2-3)

Extension documents may define new error notifications with these semantics, but MUST NOT use them unless the peer has been shown to understand them, such as by using the Vendor ID payload (§2.21.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMobikeAuthExtensionErrorsRequirePeerUnderstanding`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L141) | unit/verify | revert, verified |
| positive | [`TestMobikeAuthExtensionErrorsRequirePeerUnderstanding`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/mobike_test.go#L139) | unit/verify | revert, verified |

### [`RFC7296-2.21.3-1`](#rfc7296-2.21.3-1)

After the IKE SA is authenticated, all requests having errors MUST result in a response notifying the other end of the error (§2.21.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestErrNewChildRequestIsAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L124) | unit/verify | unproven |
| negative | [`TestErrRefusedChildRekeyIsAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L30) | unit/verify | unproven |
| negative | [`TestErrRefusedIKERekeyIsAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L591) | unit/verify | unproven |
| positive | [`TestDelMalformedSPISizeDrawsInvalidSyntax`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_delete_test.go#L220) | unit/verify | unproven |
| positive | [`TestErrNewChildRequestIsAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L120) | unit/verify | unproven |
| positive | [`TestErrRefusedChildRekeyIsAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L23) | unit/verify | unproven |
| positive | [`TestErrRefusedIKERekeyIsAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L586) | unit/verify | unproven |

### [`RFC7296-2.21.4-1`](#rfc7296-2.21.4-1)

If the message is marked as a response, the node can audit the suspicious event but MUST NOT respond (§2.21.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNtfOutOfSAIgnoresResponses`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/notify_error_test.go#L165) | unit/verify | unproven |
| negative | [`TestNtfOutOfSASkipsSAInitAndRateLimits`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/notify_error_test.go#L184) | unit/verify | unproven |
| negative | [`ipsec-error-notify-no-loop.ci`](https://github.com/ze-software/ze/blob/main/test/ipsec/ipsec-error-notify-no-loop.ci#L24) | functional/verify | unproven |
| positive | [`TestNtfOutOfSAIgnoresResponses`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/notify_error_test.go#L162) | unit/verify | unproven |
| positive | [`TestNtfOutOfSASkipsSAInitAndRateLimits`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/notify_error_test.go#L180) | unit/verify | unproven |
| positive | [`ipsec-error-notify-no-loop.ci`](https://github.com/ze-software/ze/blob/main/test/ipsec/ipsec-error-notify-no-loop.ci#L21) | functional/verify | unproven |

### [`RFC7296-2.21.4-2`](#rfc7296-2.21.4-2)

If a response is sent, the response MUST be sent to the IP address and port from where it came with the same IKE SPIs and the Message ID copied. (§2.21.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. INVALID_IKE_SPI answer copies SPIs, Message ID and exchange type with Response flag, per request; answered over the arrival socket framing; .ci covers arrival socket

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNtfOutOfSAAnswerCarriesTheSocketFraming`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/notify_error_test.go#L313) | unit/verify | unproven |
| negative | [`TestNtfOutOfSAAnswersWithInvalidIKESPI`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/notify_error_test.go#L79) | unit/verify | unproven |
| negative | [`ipsec-error-notify-no-loop.ci`](https://github.com/ze-software/ze/blob/main/test/ipsec/ipsec-error-notify-no-loop.ci#L28) | functional/verify | unproven |
| positive | [`TestNtfOutOfSAAnswerCarriesTheSocketFraming`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/notify_error_test.go#L309) | unit/verify | unproven |
| positive | [`TestNtfOutOfSAAnswersWithInvalidIKESPI`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/notify_error_test.go#L75) | unit/verify | unproven |
| positive | [`ipsec-error-notify-no-loop.ci`](https://github.com/ze-software/ze/blob/main/test/ipsec/ipsec-error-notify-no-loop.ci#L26) | functional/verify | unproven |

### [`RFC7296-2.21.4-3`](#rfc7296-2.21.4-3)

The response MUST NOT be cryptographically protected (§2.21.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNtfOutOfSAAnswerIsUnprotected`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/notify_error_test.go#L238) | unit/verify | unproven |
| positive | [`TestNtfOutOfSAAnswerIsUnprotected`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/notify_error_test.go#L232) | unit/verify | unproven |

### [`RFC7296-2.21.4-4`](#rfc7296-2.21.4-4)

MUST contain an INVALID_IKE_SPI Notify payload. (§2.21.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. answer holds exactly one INVALID_IKE_SPI Notify with empty SPI and data

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNtfOutOfSAAnswersWithInvalidIKESPI`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/notify_error_test.go#L84) | unit/verify | unproven |
| negative | [`ipsec-error-notify-no-loop.ci`](https://github.com/ze-software/ze/blob/main/test/ipsec/ipsec-error-notify-no-loop.ci#L32) | functional/verify | unproven |
| positive | [`TestNtfOutOfSAAnswersWithInvalidIKESPI`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/notify_error_test.go#L82) | unit/verify | unproven |
| positive | [`ipsec-error-notify-no-loop.ci`](https://github.com/ze-software/ze/blob/main/test/ipsec/ipsec-error-notify-no-loop.ci#L30) | functional/verify | unproven |

### [`RFC7296-2.21.4-5`](#rfc7296-2.21.4-5)

A peer receiving such an unprotected Notify payload MUST NOT respond (§2.21.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNtfEmitterIsAFixedPoint`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/notify_error_test.go#L287) | unit/verify | unproven |
| negative | [`ipsec-error-notify-no-loop.ci`](https://github.com/ze-software/ze/blob/main/test/ipsec/ipsec-error-notify-no-loop.ci#L37) | functional/verify | unproven |
| positive | [`TestNtfEmitterIsAFixedPoint`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/notify_error_test.go#L282) | unit/verify | unproven |
| positive | [`ipsec-error-notify-no-loop.ci`](https://github.com/ze-software/ze/blob/main/test/ipsec/ipsec-error-notify-no-loop.ci#L34) | functional/verify | unproven |

### [`RFC7296-2.21.4-6`](#rfc7296-2.21.4-6)

MUST NOT change the state of any existing SAs. (§2.21.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. unprotected notify leaves state, counters, child SA and DPD timer untouched; same notify protected reaches handlers

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestErrUnprotectedNotifyChangesNoState`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L291) | unit/verify | unproven |
| positive | [`TestErrUnprotectedNotifyChangesNoState`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L285) | unit/verify | unproven |

### [`RFC7296-2.21.4-7`](#rfc7296-2.21.4-7)

A node receiving a suspicious message from an IP address (and port, if NAT traversal is used) with which it has an IKE SA SHOULD send an IKE Notify payload in an IKE INFORMATIONAL exchange over that SA. The recipient MUST NOT change the state of any SAs as a result, but may wish to audit the event to aid in diagnosing malfunctions. (§2.21.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Clauses: a node receiving a suspicious message from an address with which it has an IKE SA SHOULD send a Notify in an INFORMATIONAL exchange over that SA; the recipient MUST NOT change the state of any SAs. The MUST NOT is asserted (a protected INFORMATIONAL notify is answered and changes no SA state; a protected Delete does change state, the control). The SHOULD-send clause has no assertion: no unit feeds Ze a suspicious message and checks a Notify goes out over the existing SA.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestErrProtectedInformationalNotifyChangesNoState`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L348) | unit/verify | unproven |
| positive | [`TestErrProtectedInformationalNotifyChangesNoState`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L341) | unit/verify | unproven |

### [`RFC7296-3.10.1-1`](#rfc7296-3.10.1-1)

Types in the range 0 - 16383 are intended for reporting errors. An implementation receiving a Notify payload with one of these types that it does not recognize in a response MUST assume that the corresponding request has failed entirely. (§3.10.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. unrecognized error-range notify in an IKE_AUTH response ends the exchange; unrecognized status type ignored and SA establishes

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestErrUnrecognizedNotifyHandling`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L439) | unit/verify | unproven |
| positive | [`TestErrUnrecognizedNotifyHandling`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L435) | unit/verify | unproven |

### [`RFC7296-3.10.1-2`](#rfc7296-3.10.1-2)

Unrecognized error types in a request and status types in a request or response MUST be ignored, and they should be logged (§3.10.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestErrUnrecognizedNotifyHandling`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L446) | unit/verify | unproven |
| negative | [`TestCritNotifyTypeClassification`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_critpayload_test.go#L383) | unit/verify | unproven |
| positive | [`TestErrUnrecognizedNotifyHandling`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L442) | unit/verify | unproven |
| positive | [`TestCritNotifyTypeClassification`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_critpayload_test.go#L379) | unit/verify | unproven |

### [`RFC7296-3.10.1-3`](#rfc7296-3.10.1-3)

To avoid leaking information to someone probing a node, this status MUST be sent in response to any error not covered by one of the other status types. (§3.10.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Clause: INVALID_SYNTAX MUST be sent in response to ANY error not covered by another status type. One uncovered error class is asserted (a truncated inner chain in an authenticated request draws INVALID_SYNTAX); the negative covers the encrypted-and-verified precondition. Other uncovered errors (a bad payload length, an invalid field value in a protected request) have no case, so a responder answering them with another type or nothing stays green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestErrInnerParseFailureDrawsInvalidSyntaxAndOuterDrawsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L166) | unit/verify | unproven |
| positive | [`TestErrInnerParseFailureDrawsInvalidSyntaxAndOuterDrawsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_notify_error_test.go#L163) | unit/verify | unproven |

### [`RFC7296-2.6-2`](#rfc7296-2.6-2)

Each endpoint chooses one of the two SPIs and MUST choose them so as to be unique identifiers of an IKE SA (§2.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSPIsAreUniqueIdentifiers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L288) | unit/verify | unproven |
| positive | [`TestSPIsAreUniqueIdentifiers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L285) | unit/verify | unproven |

### [`RFC7296-2.6-3`](#rfc7296-2.6-3)

The data associated with this notification MUST be between 1 and 64 octets in length (inclusive) (§2.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCkeEchoedCookieIsBoundedToo`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cookie_test.go#L101) | unit/verify | unproven |
| positive | [`TestCkeMintedCookieIsWithinTheLengthBound`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cookie_test.go#L58) | unit/verify | unproven |

### [`RFC7296-2.6-4`](#rfc7296-2.6-4)

If the IKE_SA_INIT response includes the COOKIE notification, the initiator MUST then retry the IKE_SA_INIT request, and include the COOKIE notification containing the received data as the first payload, and all other payloads unchanged (§2.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCkeCookieIsAbsentWithoutAChallenge`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cookie_test.go#L204) | unit/verify | unproven |
| positive | [`TestCkeRetryCarriesCookieFirstAndNothingElseChanged`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cookie_test.go#L140) | unit/verify | unproven |

### [`RFC7296-2.6-5`](#rfc7296-2.6-5)

When one party receives an IKE_SA_INIT request containing a cookie whose contents do not match the value expected, that party MUST ignore the cookie and process the message as if no cookie had been included; usually this means sending a response containing a new cookie (§2.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCkeValidCookieReachesTheHandshake`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cookie_test.go#L258) | unit/verify | unproven |
| positive | [`TestCkeMismatchedCookieIsIgnoredNotRejected`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cookie_test.go#L223) | unit/verify | unproven |

### [`RFC7296-2.6.1-1`](#rfc7296-2.6.1-1)

Implementations SHOULD support this shorter exchange, but MUST NOT fail if other implementations do not support this shorter exchange (§2.6.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCkeCookieAndInvalidKECombineWithoutFailing`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cookie_test.go#L350) | unit/verify | unproven |
| positive | [`TestCkeSecondCookieReplacesTheFirstWithoutFailing`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cookie_test.go#L304) | unit/verify | unproven |

### [`RFC7296-2.10-2`](#rfc7296-2.10-2)

MUST be at least 128 bits in size (§2.10)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Clause: nonces used in IKEv2 are at least 128 bits. The obligation binds the nonce Ze sends. TestNonceLengthBounds asserts only the receive floor (ReadFrom refuses 15 octets, accepts 16/32/256, refuses 257) and the NonceMinLen constant; the nonce Ze generates (nonceLen in newInitiatorSA/newResponderSA/rekey) is not asserted in the tagged unit, so a sender emitting an 8-octet nonce stays green here.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNonceLengthBounds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L611) | unit/verify | unproven |
| positive | [`TestNonceLengthBounds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L609) | unit/verify | unproven |

### [`RFC7296-2.10-3`](#rfc7296-2.10-3)

MUST be at least half the key size of the negotiated pseudorandom function (PRF) (§2.10)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. positive tag claims every rekey nonce uses nonceLen (rekey.go) but only newInitiatorSA/newResponderSA nonces are asserted; PRF list is hand-written in the test, not read from prfRegistry; negative is a tightness guard, not a violating input

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNonceMeetsHalfPRFKeySize`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_encrypt_test.go#L426) | unit/verify | unproven |
| positive | [`TestNonceMeetsHalfPRFKeySize`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_encrypt_test.go#L421) | unit/verify | unproven |

### [`RFC7296-2.13-1`](#rfc7296-2.13-1)

For algorithms that accept a variable-length key, a fixed key size MUST be specified as part of the cryptographic transform negotiated (§2.13)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTransformRegistryStatesKeySizes`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_test.go#L64) | unit/verify | unproven |
| positive | [`TestTransformRegistryStatesKeySizes`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_test.go#L60) | unit/verify | unproven |

### [`RFC7296-2.13-2`](#rfc7296-2.13-2)

For algorithms for which not all values are valid keys (such as DES or 3DES with key parity), the algorithm by which keys are derived from arbitrary values MUST be specified by the cryptographic transform. (§2.13)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. the tags assert SK_* key sizing, which is RFC7296-2.13-3; no supported cipher has invalid key values (AES only), so nothing tests this row's rule. no supported transform has invalid key values (encryptionRegistry is AES CBC/GCM/CCM only, no DES/3DES); the tags assert SK_* key sizing, which is RFC7296-2.13-3

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSKKeyLengthsComeFromTransforms`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_test.go#L117) | unit/verify | unproven |
| positive | [`TestSKKeyLengthsComeFromTransforms`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_test.go#L113) | unit/verify | unproven |

### [`RFC7296-2.13-3`](#rfc7296-2.13-3)

The preferred key size MUST be used as the length of SK_d, SK_pi, and SK_pr (§2.13, §2.14)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSKKeyLengthsComeFromTransforms`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_test.go#L123) | unit/verify | unproven |
| positive | [`TestSKKeyLengthsComeFromTransforms`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_test.go#L120) | unit/verify | unproven |

### [`RFC7296-2.13-4`](#rfc7296-2.13-4)

Other types of PRFs MUST specify their preferred key size (§2.13)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTransformRegistryStatesKeySizes`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_test.go#L70) | unit/verify | unproven |
| positive | [`TestTransformRegistryStatesKeySizes`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_test.go#L67) | unit/verify | unproven |

### [`RFC7296-2.15-1`](#rfc7296-2.15-1)

The management interface by which the shared secret is provided MUST accept ASCII strings of at least 64 octets (§2.15)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPSKAcceptsAtLeast64ASCIIOctets`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc7296_test.go#L39) | unit/verify | unproven |
| positive | [`TestPSKAcceptsAtLeast64ASCIIOctets`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc7296_test.go#L34) | unit/verify | unproven |

### [`RFC7296-2.15-2`](#rfc7296-2.15-2)

MUST NOT add a null terminator before using them as shared secrets (§2.15)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. asserts parseAuthConfig keeps the PSK length and adds no NUL; the claimed use path computePSKAuth (engine/auth.go) is not exercised, so a NUL appended at use would stay green

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPSKHasNoNullTerminatorAdded`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc7296_test.go#L86) | unit/verify | unproven |
| positive | [`TestPSKHasNoNullTerminatorAdded`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc7296_test.go#L81) | unit/verify | unproven |

### [`RFC7296-2.17-1`](#rfc7296-2.17-1)

keying material for each Child SA MUST be taken from the expanded KEYMAT using the following rules: o All keys for SAs carrying data from the initiator to the responder are taken before SAs going from the responder to the initiator. (§2.17)

Audit verdict: enforced (the tests do what the requirement demands), fresh. DeriveChildSAKeys slices recomputed prf+ KEYMAT with both initiator keys before responder keys

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestChildSAKeymatOrder`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_test.go#L234) | unit/verify | unproven |
| positive | [`TestChildSAKeymatOrder`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_test.go#L226) | unit/verify | unproven |

### [`RFC7296-2.17-2`](#rfc7296-2.17-2)

For ESP and AH, [IPSECARCH] defines the order, namely: the encryption key (if any) MUST be taken from the first bits and the integrity key (if any) MUST be taken from the remaining bits. (§2.17)

Audit verdict: enforced (the tests do what the requirement demands), fresh. encryption key then integrity key per direction, positional, asserted against recomputed KEYMAT

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestChildSAKeymatOrder`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_test.go#L238) | unit/verify | unproven |
| positive | [`TestChildSAKeymatOrder`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_test.go#L230) | unit/verify | unproven |

### [`RFC7296-3.1-1`](#rfc7296-3.1-1)

An Encrypted payload MUST be the last payload in a packet (§3.1, §3.14)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSKIsLastAndNeverNested`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_encrypt_test.go#L95) | unit/verify | unproven |
| positive | [`TestSKIsLastAndNeverNested`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_encrypt_test.go#L87) | unit/verify | unproven |

### [`RFC7296-3.1-2`](#rfc7296-3.1-2)

an Encrypted payload MUST NOT contain another Encrypted payload (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. IKE_AUTH request and response inner chains hold no SK payload; negative is a vacuity guard

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSKIsLastAndNeverNested`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_encrypt_test.go#L98) | unit/verify | unproven |
| positive | [`TestSKIsLastAndNeverNested`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_encrypt_test.go#L91) | unit/verify | unproven |

### [`RFC7296-3.1-3`](#rfc7296-3.1-3)

Initiator's SPI (8 octets) - A value chosen by the initiator to identify a unique IKE Security Association. This value MUST NOT be zero. (§3.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. GenerateSPI and new SAs asserted non-zero; the negative tag claims dispatchInbound drops a zero initiator SPI on receipt (register.go) but the body never exercises dispatchInbound

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSPIZeroRules`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L209) | unit/verify | unproven |
| positive | [`TestSPIZeroRules`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L201) | unit/verify | unproven |

### [`RFC7296-3.1-4`](#rfc7296-3.1-4)

Responder's SPI (8 octets) - A value chosen by the responder to identify a unique IKE Security Association. This value MUST be zero in the first message of an IKE initial exchange (including repeats of that message including a cookie). (§3.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Clauses: Responder's SPI is zero in the first message of an initial exchange, including repeats of that message including a cookie. TestSPIZeroRules asserts octets 8..15 are zero in Ze's IKE_SA_INIT request and non-zero in the response. The cookie-repeat clause of the quote has no case: a retried IKE_SA_INIT carrying a COOKIE is never checked for a zero Responder's SPI.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSPIZeroRules`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L212) | unit/verify | unproven |
| positive | [`TestSPIZeroRules`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L204) | unit/verify | unproven |

### [`RFC7296-3.1-5`](#rfc7296-3.1-5)

Implementations based on this version of IKE MUST set the major version to 2 (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBuiltMessagesCarryVersion2Point0`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L87) | unit/verify | unproven |
| positive | [`TestBuiltMessagesCarryVersion2Point0`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L79) | unit/verify | unproven |

### [`RFC7296-3.1-6`](#rfc7296-3.1-6)

Implementations based on this version of IKE MUST set the minor version to 0 (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBuiltMessagesCarryVersion2Point0`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L90) | unit/verify | unproven |
| positive | [`TestBuiltMessagesCarryVersion2Point0`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L84) | unit/verify | unproven |

### [`RFC7296-3.1-7`](#rfc7296-3.1-7)

'X' bits MUST be cleared when sending (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. every engine-built message has X bits clear; I and R observed so the mask is not vacuous

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBuiltMessagesClearXAndVBits`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L335) | unit/verify | unproven |
| positive | [`TestBuiltMessagesClearXAndVBits`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L328) | unit/verify | unproven |

### [`RFC7296-3.1-8`](#rfc7296-3.1-8)

MUST be ignored on receipt (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. IKE_SA_INIT with all X bits set is processed as the clean one; R bit set changes treatment

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestXBitsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L428) | unit/verify | unproven |
| positive | [`TestXBitsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L420) | unit/verify | unproven |

### [`RFC7296-3.1-9`](#rfc7296-3.1-9)

This bit MUST be cleared in all request messages and MUST be set in all responses. (§3.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Clauses: R cleared in all requests; set in all responses. The positive asserts both on engine-built messages. The negative tag (the responder refuses a request whose R bit is set) exercises the receive-side rule of RFC7296-3.1-12, a neighbouring one, and is not a discriminator of the sender assertion; the row carries no {single-polarity} marker.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestResponseBitMatchesDirection`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L377) | unit/verify | unproven |
| positive | [`TestResponseBitMatchesDirection`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L373) | unit/verify | unproven |

### [`RFC7296-3.1-11`](#rfc7296-3.1-11)

Implementations of IKEv2 MUST clear this bit when sending and MUST ignore it in incoming messages. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. V bit clear on every built message; set V bit on receipt changes nothing

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBuiltMessagesClearXAndVBits`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L337) | unit/verify | unproven |
| negative | [`TestXBitsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L424) | unit/verify | unproven |
| positive | [`TestBuiltMessagesClearXAndVBits`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_header_test.go#L332) | unit/verify | unproven |

### [`RFC7296-3.1-12`](#rfc7296-3.1-12)

An IKE endpoint MUST NOT generate a response to a message that is marked as being a response (with one exception; see Section 2.21.2) (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNrsInformationalHandlerRefusesAResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_noresponse_test.go#L257) | unit/verify | revert, verified |
| negative | [`TestNrsResponseNeverDrawsAResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_noresponse_test.go#L56) | unit/verify | revert, verified |
| positive | [`TestNrsInformationalHandlerRefusesAResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_noresponse_test.go#L252) | unit/verify | revert, verified |
| positive | [`TestNrsResponseNeverDrawsAResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_noresponse_test.go#L46) | unit/verify | revert, verified |

### [`RFC7296-3.2-2`](#rfc7296-3.2-2)

MUST be ignored by the recipient if the recipient understands the payload type code. (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. critical Nonce parsed like uncritical at outer and inner chain; critical unknown type refused

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInnerChainIgnoresCriticalOnKnownType`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_innerchain_test.go#L177) | unit/verify | unproven |
| negative | [`TestCriticalBitIgnoredForKnownType`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L417) | unit/verify | unproven |
| positive | [`TestInnerChainIgnoresCriticalOnKnownType`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_innerchain_test.go#L174) | unit/verify | unproven |
| positive | [`TestCriticalBitIgnoredForKnownType`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L414) | unit/verify | unproven |

### [`RFC7296-3.2-3`](#rfc7296-3.2-3)

all implementations MUST understand all payload types defined in this document (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. decodePayload yields a concrete type for all sixteen types 33-48; type 204 is ErrUnknownPayload

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAllDefinedPayloadTypesUnderstood`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L458) | unit/verify | unproven |
| positive | [`TestAllDefinedPayloadTypesUnderstood`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L454) | unit/verify | unproven |

### [`RFC7296-3.2-4`](#rfc7296-3.2-4)

MUST be set to zero for payload types defined in this document. (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. sweep of engine-built outer and inner chains plus a source walk refusing any Critical write in engine

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDefinedPayloadTypesAreSentUncritical`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_critical_bit_test.go#L119) | unit/verify | unproven |
| positive | [`TestDefinedPayloadTypesAreSentUncritical`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_critical_bit_test.go#L112) | unit/verify | unproven |
| positive | [`TestEngineSourceNeverSetsTheCriticalField`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_critical_bit_test.go#L385) | unit/verify | unproven |

### [`RFC7296-3.2-5`](#rfc7296-3.2-5)

o Critical (1 bit) - MUST be set to zero if the sender wants the recipient to skip this payload if it does not understand the payload type code in the Next Payload field of the previous payload. (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. zero bit on the wire from the encoded bytes; recipient skips as PayloadRaw; set bit refused

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCritSenderZeroBitRequestsSkip`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_critpayload_test.go#L106) | unit/verify | unproven |
| positive | [`TestCritSenderZeroBitRequestsSkip`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_critpayload_test.go#L101) | unit/verify | unproven |

### [`RFC7296-3.2-6`](#rfc7296-3.2-6)

MUST be set to one if the sender wants the recipient to reject this entire message if it does not understand the payload type. (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. one bit on the wire; recipient rejects the whole message with ErrUnsupportedCrit

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCritSenderOneBitRequestsWholeMessageRejection`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_critpayload_test.go#L159) | unit/verify | unproven |
| positive | [`TestCritSenderOneBitRequestsWholeMessageRejection`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_critpayload_test.go#L155) | unit/verify | unproven |

### [`RFC7296-3.3-3`](#rfc7296-3.3-3)

An SA payload MAY contain multiple proposals. If there is more than one, they MUST be ordered from most preferred to least preferred (§3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestProposalOrderPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L514) | unit/verify | unproven |
| positive | [`TestProposalOrderPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L510) | unit/verify | unproven |

### [`RFC7296-3.3-4`](#rfc7296-3.3-4)

When parsing an SA, an implementation MUST check that the total Payload Length is consistent with the payload's internal lengths and counts (§3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSAInternalLengthConsistency`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L563) | unit/verify | unproven |
| positive | [`TestSAInternalLengthConsistency`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L559) | unit/verify | unproven |

### [`RFC7296-3.3-5`](#rfc7296-3.3-5)

Each structure MUST have a proposal number one (1) greater than the previous structure. The first Proposal in the initiator's SA payload MUST have a Proposal Num of one (1) (§3.3, §3.3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPropProposalNumbering`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_sa_test.go#L307) | unit/verify | unproven |
| positive | [`TestPropProposalNumbering`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_sa_test.go#L303) | unit/verify | unproven |

### [`RFC7296-3.3-6`](#rfc7296-3.3-6)

A transform MUST NOT have multiple attributes of the same type (§3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPropDuplicateAttributeRejected`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_sa_test.go#L76) | unit/verify | unproven |
| positive | [`TestPropDuplicateAttributeRejected`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_sa_test.go#L80) | unit/verify | unproven |

### [`RFC7296-3.3-7`](#rfc7296-3.3-7)

To propose alternate values for an attribute (for example, multiple key sizes for the AES encryption algorithm), an implementation MUST include multiple transforms with the same Transform Type each with a single Attribute. (§3.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. the obligation binds the proposer; the unit only proves PayloadSA.ReadFrom parses two ENCR transforms and refuses one transform with two key lengths; no Ze-built proposal offering alternate key lengths is asserted

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPropAlternateKeyLengthsUseSeparateTransforms`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_sa_test.go#L102) | unit/verify | unproven |
| positive | [`TestPropAlternateKeyLengthsUseSeparateTransforms`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_sa_test.go#L98) | unit/verify | unproven |

### [`RFC7296-3.3.1-1`](#rfc7296-3.3.1-1)

When a proposal is accepted, the proposal number in the SA payload MUST match the number on the proposal sent that was accepted (§3.3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPropAcceptedProposalNumberMatchesOffer`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L47) | unit/verify | unproven |
| positive | [`TestPropAcceptedProposalNumberMatchesOffer`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L43) | unit/verify | unproven |

### [`RFC7296-3.3.1-2`](#rfc7296-3.3.1-2)

o SPI Size (1 octet) - For an initial IKE SA negotiation, this field MUST be zero; the SPI is obtained from the outer header. (§3.3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Ze's IKE_SA_INIT proposals carry SPI Size 0; responder kills an initial request with SPI Size 8 and answers nothing

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSpzInitialIKESANegotiationNeedsZeroSPISize`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_spisize_test.go#L45) | unit/verify | unproven |
| negative | [`TestPropSPISizeMatchesProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_sa_test.go#L254) | unit/verify | unproven |
| positive | [`TestSpzInitialIKESANegotiationNeedsZeroSPISize`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_spisize_test.go#L49) | unit/verify | unproven |
| positive | [`TestPropSPISizeMatchesProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_sa_test.go#L250) | unit/verify | unproven |

### [`RFC7296-3.3.3-1`](#rfc7296-3.3.3-1)

A compliant implementation MUST understand all mandatory and optional types for each protocol it supports (§3.3.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. transformTypeUnderstood per protocol matches the Section 3.3.3 table; engine refuses ESN in an IKE proposal

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPropTransformTypesUnderstoodPerProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L76) | unit/verify | unproven |
| negative | [`TestTftUnknownTransformTypeMakesProposalUnacceptable`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transformtype_test.go#L37) | unit/verify | unproven |
| positive | [`TestPropTransformTypesUnderstoodPerProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L70) | unit/verify | unproven |
| positive | [`TestTftUnknownTransformTypeMakesProposalUnacceptable`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transformtype_test.go#L40) | unit/verify | unproven |

### [`RFC7296-3.3.4-2`](#rfc7296-3.3.4-2)

Upon receipt of a payload with a set of Transform IDs, the implementation MUST compare the transmitted Transform IDs against those locally configured via the management controls, to verify that the proposed suite is acceptable based on local policy (§3.3.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPropTransformIDsComparedAgainstLocalPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L125) | unit/verify | unproven |
| positive | [`TestPropTransformIDsComparedAgainstLocalPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L119) | unit/verify | unproven |

### [`RFC7296-3.3.4-3`](#rfc7296-3.3.4-3)

The implementation MUST reject SA proposals that are not authorized by these IKE suite controls (§3.3.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPropUnauthorizedProposalRejected`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L161) | unit/verify | unproven |
| positive | [`TestPropUnauthorizedProposalRejected`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L157) | unit/verify | unproven |

### [`RFC7296-3.3.5-1`](#rfc7296-3.3.5-1)

Attributes described as fixed length MUST NOT be encoded using the variable-length encoding unless that length exceeds two bytes (§3.3.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPropFixedLengthAttributeRejectsTLVEncoding`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_sa_test.go#L132) | unit/verify | unproven |
| positive | [`TestPropFixedLengthAttributeRejectsTLVEncoding`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_sa_test.go#L136) | unit/verify | unproven |

### [`RFC7296-3.3.5-2`](#rfc7296-3.3.5-2)

Variable-length attributes MUST NOT be encoded as fixed-length even if their value can fit into two octets (§3.3.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPropVariableLengthAttributeRejectsTVEncoding`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_sa_test.go#L156) | unit/verify | unproven |
| positive | [`TestPropVariableLengthAttributeRejectsTVEncoding`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_sa_test.go#L161) | unit/verify | unproven |

### [`RFC7296-3.3.5-3`](#rfc7296-3.3.5-3)

The Key Length attribute specifies the key length in bits (MUST use network byte order) (§3.3.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. 256 encodes 01 00; swapped octets read as 1

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPropKeyLengthUsesNetworkByteOrder`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_sa_test.go#L187) | unit/verify | unproven |
| positive | [`TestPropKeyLengthUsesNetworkByteOrder`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_sa_test.go#L184) | unit/verify | unproven |

### [`RFC7296-3.3.5-4`](#rfc7296-3.3.5-4)

The Key Length attribute MUST NOT be used with transforms that use a fixed-length key (§3.3.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPropKeyLengthRejectedOnFixedKeyTransform`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_sa_test.go#L219) | unit/verify | unproven |
| positive | [`TestPropKeyLengthRejectedOnFixedKeyTransform`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_sa_test.go#L223) | unit/verify | unproven |

### [`RFC7296-3.3.5-5`](#rfc7296-3.3.5-5)

o Some transforms specify that the Key Length attribute MUST be always included (omitting the attribute is not allowed, and proposals not containing it MUST be rejected). (§3.3.5)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Clauses: for such transforms the Key Length attribute MUST always be included (sender); proposals not containing it MUST be rejected (receiver). The receiver clause is asserted (AES-CBC and AES-CTR offers without Key Length refused with ErrKeyLengthMissing, accepted with it). The sender clause has no assertion: no Ze-built proposal is checked for a Key Length attribute on AES-CBC/CTR.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPropKeyLengthRequiredTransformRejectedWithoutIt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L188) | unit/verify | unproven |
| positive | [`TestPropKeyLengthRequiredTransformRejectedWithoutIt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L193) | unit/verify | unproven |

### [`RFC7296-3.3.6-4`](#rfc7296-3.3.6-4)

If the responder receives a proposal that contains a Transform Type it does not understand, or a proposal that is missing a mandatory Transform Type, it MUST consider this proposal unacceptable; however, other proposals in the same SA payload are processed as usual (§3.3.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPropProposalMissingMandatoryTransformTypeUnacceptable`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L247) | unit/verify | unproven |
| negative | [`TestTftForeignTransformTypeRefusedInESPOffer`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transformtype_test.go#L77) | unit/verify | unproven |
| negative | [`TestTftUnknownTransformTypeMakesProposalUnacceptable`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transformtype_test.go#L29) | unit/verify | unproven |
| positive | [`TestPropProposalMissingMandatoryTransformTypeUnacceptable`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L253) | unit/verify | unproven |
| positive | [`TestTftForeignTransformTypeRefusedInESPOffer`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transformtype_test.go#L80) | unit/verify | unproven |
| positive | [`TestTftUnknownTransformTypeMakesProposalUnacceptable`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transformtype_test.go#L34) | unit/verify | unproven |

### [`RFC7296-3.3.6-5`](#rfc7296-3.3.6-5)

Similarly, if the responder receives a transform that it does not understand, or one that contains a Transform Attribute it does not understand, it MUST consider this transform unacceptable; other transforms with the same Transform Type are processed as usual. (§3.3.6)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. the engine positive uses D-H NONE, a transform Ze understands and refuses, not one it does not understand; the crypto unit's positive proves another PROPOSAL is accepted (3.3.6-4), not a sibling transform; the Transform Attribute not-understood clause is not exercised

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPropUnknownTransformIDMakesTransformUnacceptable`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L283) | unit/verify | unproven |
| negative | [`TestAltUnusableAlternativesAreStillRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transform_alternatives_test.go#L89) | unit/verify | unproven |
| positive | [`TestPropUnknownTransformIDMakesTransformUnacceptable`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L287) | unit/verify | unproven |
| positive | [`TestAltDHAlternativesAreBothOffered`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_transform_alternatives_test.go#L52) | unit/verify | unproven |

### [`RFC7296-3.3.6-7`](#rfc7296-3.3.6-7)

Any attributes of a selected transform MUST be returned unmodified (§3.3.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPropSelectedTransformAttributesUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L218) | unit/verify | unproven |
| positive | [`TestPropSelectedTransformAttributesUnmodified`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L214) | unit/verify | unproven |

### [`RFC7296-3.9-1`](#rfc7296-3.9-1)

The size of the Nonce Data MUST be between 16 and 256 octets, inclusive (§3.9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNonceLengthBounds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L607) | unit/verify | unproven |
| positive | [`TestNonceLengthBounds`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L604) | unit/verify | unproven |

### [`RFC7296-3.10-3`](#rfc7296-3.10-3)

For a notification concerning the IKE SA, the SPI Size MUST be zero and the field must be empty. (§3.10)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. PayloadNotify codec round-trips a test-built struct with SPISize 0; no Ze producer of an IKE-SA notification (INITIAL_CONTACT, error notifies) is asserted to send SPI Size 0

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNotifyIKESAHasEmptySPI`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L639) | unit/verify | unproven |
| positive | [`TestNotifyIKESAHasEmptySPI`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L633) | unit/verify | unproven |

### [`RFC7296-3.10-4`](#rfc7296-3.10-4)

For notifications concerning Child SAs, this field MUST contain either (2) to indicate AH or (3) to indicate ESP. (§3.10)

Audit verdict: enforced (the tests do what the requirement demands), fresh. SPI-bearing notify with AH/ESP round-trips; Protocol ID 0,1,4,255 beside an SPI refused with ErrNotifyProtocolID

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNtfyChildSAProtocolIDIsAHOrESP`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_notify_test.go#L14) | unit/verify | unproven |
| positive | [`TestNtfyChildSAProtocolIDIsAHOrESP`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_notify_test.go#L9) | unit/verify | unproven |

### [`RFC7296-3.10-5`](#rfc7296-3.10-5)

If the SPI field is empty, this field MUST be sent as zero and MUST be ignored on receipt. (§3.10)

Audit verdict: enforced (the tests do what the requirement demands), fresh. WriteTo zeroes Protocol ID for an empty SPI; ReadFrom discards it; kept when an SPI is present

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNtfyEmptySPIProtocolIDIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_notify_test.go#L117) | unit/verify | unproven |
| negative | [`TestNtfyEmptySPISendsProtocolIDZero`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_notify_test.go#L65) | unit/verify | unproven |
| positive | [`TestNtfyEmptySPIProtocolIDIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_notify_test.go#L113) | unit/verify | unproven |
| positive | [`TestNtfyEmptySPISendsProtocolIDZero`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_notify_test.go#L60) | unit/verify | unproven |

### [`RFC7296-3.11-1`](#rfc7296-3.11-1)

each SPI MUST be for the same protocol. Mixing of protocol identifiers MUST NOT be performed in the Delete payload. (§3.11)

Audit verdict: enforced (the tests do what the requirement demands), fresh. PayloadDelete holds one ProtocolID by construction; two protocols need two payloads

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDeletePayloadSingleProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L684) | unit/verify | unproven |
| positive | [`TestDeletePayloadSingleProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L680) | unit/verify | unproven |

### [`RFC7296-3.11-2`](#rfc7296-3.11-2)

It MUST be zero for IKE (SPI is in message header) or four for AH and ESP. (§3.11)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. the engine refuses an ESP Delete with SPI Size 3 (INVALID_SYNTAX) and accepts 4; the wire positive round-trips test-chosen sizes and the wire negative is truncation, a neighbouring rule; IKE Delete with non-zero SPI Size and AH with a wrong size are not exercised on receipt

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDelMalformedSPISizeDrawsInvalidSyntax`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_delete_test.go#L217) | unit/verify | unproven |
| negative | [`TestDeleteSPISizeByProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L733) | unit/verify | unproven |
| positive | [`TestDelWellFormedSPISizeIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_delete_test.go#L257) | unit/verify | unproven |
| positive | [`TestDeleteSPISizeByProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L729) | unit/verify | unproven |

### [`RFC7296-3.12-2`](#rfc7296-3.12-2)

Unfamiliar Vendor IDs MUST be ignored (§3.12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestVendorIDIgnoredButPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L774) | unit/verify | unproven |
| positive | [`TestVendorIDIgnoredButPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L766) | unit/verify | unproven |

### [`RFC7296-3.12-3`](#rfc7296-3.12-3)

Writers of documents who wish to extend this protocol MUST define a Vendor ID payload to announce the ability to implement the extension in the document (§3.12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestVendorIDIgnoredButPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L776) | unit/verify | unproven |
| positive | [`TestVendorIDIgnoredButPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_test.go#L770) | unit/verify | unproven |

### [`RFC7296-3.12-4`](#rfc7296-3.12-4)

A Vendor ID payload MUST NOT change the interpretation of any information defined in this specification (i.e., the critical bit MUST be set to 0) (§3.12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestVendorIDDoesNotChangeInterpretation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_critical_bit_test.go#L265) | unit/verify | unproven |
| positive | [`TestVendorIDDoesNotChangeInterpretation`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_critical_bit_test.go#L256) | unit/verify | unproven |

### [`RFC7296-3.14-2`](#rfc7296-3.14-2)

Senders MUST select a new unpredictable IV for every message (§3.14)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSKSelectsFreshIVPerMessage`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_encrypt_test.go#L153) | unit/verify | unproven |
| positive | [`TestSKSelectsFreshIVPerMessage`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_encrypt_test.go#L149) | unit/verify | unproven |

### [`RFC7296-3.14-3`](#rfc7296-3.14-3)

recipients MUST accept any value (§3.14)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Clause: recipients MUST accept any IV. The positive asserts all-zero, all-ones and ascending IVs decrypt. The negative tag exercises checksum corruption, a neighbouring integrity rule, not a violation of IV acceptance, and the row carries no {single-polarity} marker, so there is no pair.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSKAcceptsAnyIVOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_encrypt_test.go#L197) | unit/verify | unproven |
| positive | [`TestSKAcceptsAnyIVOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_encrypt_test.go#L194) | unit/verify | unproven |

### [`RFC7296-3.14-4`](#rfc7296-3.14-4)

Padding MAY contain any value chosen by the sender, and MUST have a length that makes the combination of the payloads, the Padding, and the Pad Length to be a multiple of the encryption block size (§3.14)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSKPaddingAlignsAndChecksumCoversCiphertext`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_encrypt_test.go#L259) | unit/verify | unproven |
| positive | [`TestSKPaddingAlignsAndChecksumCoversCiphertext`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_encrypt_test.go#L250) | unit/verify | unproven |

### [`RFC7296-3.14-5`](#rfc7296-3.14-5)

the recipient MUST accept any length that results in proper alignment (§3.14)

Audit verdict: enforced (the tests do what the requirement demands), fresh. minimum pad and +16/+32 accepted; over-long Pad Length refused with a valid MAC

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSKAcceptsAnyAligningPadLength`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_encrypt_test.go#L337) | unit/verify | unproven |
| positive | [`TestSKAcceptsAnyAligningPadLength`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_encrypt_test.go#L333) | unit/verify | unproven |

### [`RFC7296-3.14-6`](#rfc7296-3.14-6)

The checksum MUST be computed over the encrypted message (§3.14)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSKPaddingAlignsAndChecksumCoversCiphertext`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_encrypt_test.go#L262) | unit/verify | unproven |
| positive | [`TestSKPaddingAlignsAndChecksumCoversCiphertext`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_encrypt_test.go#L254) | unit/verify | unproven |

### [`RFC7296-3.14-7`](#rfc7296-3.14-7)

Peers MUST NOT negotiate transforms for which no such specification exists (§3.14)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPropUnspecifiedTransformRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L332) | unit/verify | unproven |
| positive | [`TestPropUnspecifiedTransformRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L336) | unit/verify | unproven |

### [`RFC7296-5-2`](#rfc7296-5-2)

Though the security of negotiated Child SAs does not depend on the strength of the encryption and integrity protection negotiated in the IKE SA, implementations MUST NOT negotiate NONE as the IKE integrity protection algorithm or ENCR_NULL as the IKE encryption algorithm. (§5)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. asserts LookupIntegrity/LookupEncryption refuse none/null names and no registry entry maps AUTH_NONE; NegotiateIKE is never given a peer offer carrying ENCR_NULL or AUTH_NONE beside a non-AEAD cipher, and specifiedIntegrity (proposal.go) admits AUTH_NONE

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIKENeverNegotiatesNullAlgorithms`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_test.go#L311) | unit/verify | unproven |
| positive | [`TestIKENeverNegotiatesNullAlgorithms`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_test.go#L307) | unit/verify | unproven |

### [`RFC7296-5-3`](#rfc7296-5-3)

For this reason, a PRF whose output is less than 128 bits (e.g., 3DES-CBC) MUST NOT be used with this protocol. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. NegotiateIKE refuses a registered 64-bit PRF; prfOutputAcceptable floor at 128; every specified PRF at or above it

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPrfFloorRefusesOutputBelow128Bits`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_prffloor_test.go#L14) | unit/verify | unproven |
| negative | [`TestPropPRFOutputBelow128BitsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L361) | unit/verify | unproven |
| positive | [`TestPrfFloorRefusesOutputBelow128Bits`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_prffloor_test.go#L18) | unit/verify | unproven |
| positive | [`TestPropPRFOutputBelow128BitsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc7296_proposal_test.go#L366) | unit/verify | unproven |

### [`RFC7296-2.19-1`](#rfc7296-2.19-1)

Since the IKE_AUTH exchange creates an IKE SA and a Child SA, the IRAC MUST request the IRAS-controlled address (and optionally other information concerning the protected network) in the IKE_AUTH exchange (§2.19, §4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestZeSendsNoConfigurationRequest`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cp_test.go#L75) | unit/verify | unproven |
| positive | [`TestZeSendsNoConfigurationRequest`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cp_test.go#L64) | unit/verify | unproven |

### [`RFC7296-2.19-4`](#rfc7296-2.19-4)

CP(CFG_REQUEST) MUST contain at least an INTERNAL_ADDRESS attribute (either IPv4 or IPv6) but MAY contain any number of additional attributes the initiator wants returned in the response (§2.19)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestZeSendsNoConfigurationRequest`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cp_test.go#L89) | unit/verify | unproven |
| positive | [`TestZeSendsNoConfigurationRequest`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cp_test.go#L80) | unit/verify | unproven |

### [`RFC7296-2.20-1`](#rfc7296-2.20-1)

An IKE implementation MAY decline to give out version information prior to authentication or even after authentication in case some implementation is known to have some security weakness. In that case, it MUST either return an empty string or no CP payload if CP is not supported. (§2.20)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. sweep over engineBuiltChains finds no APPLICATION_VERSION; no unit sends Ze a CP(CFG_REQUEST) carrying APPLICATION_VERSION and asserts the reply carries no CP or an empty string

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestZeDeclinesApplicationVersion`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cp_test.go#L134) | unit/verify | unproven |
| positive | [`TestZeDeclinesApplicationVersion`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cp_test.go#L127) | unit/verify | unproven |

### [`RFC7296-3.15.1-2`](#rfc7296-3.15.1-2)

An empty INTERNAL_IP4_NETMASK attribute can be included in a CFG_REQUEST to request this information (although the gateway can send the information even when not requested). Non-empty values for this attribute in a CFG_REQUEST do not make sense and thus MUST NOT be included. (§3.15.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Clause: a CFG_REQUEST MUST NOT include a non-empty INTERNAL_IP4_NETMASK. The positive asserts no engine-built message carries CP(CFG_REQUEST). The negative tag asserts the receive side tolerates a peer's non-empty netmask, a neighbouring rule (Section 2.5 no-rejection), not a discriminator of this one; no {single-polarity} marker.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestZeSendsNoConfigRequestNetmask`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cp_test.go#L177) | unit/verify | unproven |
| positive | [`TestZeSendsNoConfigRequestNetmask`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cp_test.go#L171) | unit/verify | unproven |

### [`RFC7296-3.15.1-5`](#rfc7296-3.15.1-5)

The responder MUST return a Configuration payload if it accepted any of the configuration data, and the Configuration payload MUST contain the attributes that the responder accepted with zero-length data (§3.15.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCFGSetIsIgnoredAndDrawsNoCFGACK`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cp_test.go#L267) | unit/verify | unproven |
| positive | [`TestCFGSetIsIgnoredAndDrawsNoCFGACK`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cp_test.go#L259) | unit/verify | unproven |

### [`RFC7296-3.15.1-6`](#rfc7296-3.15.1-6)

Those attributes that it did not accept MUST NOT be in the CFG_ACK Configuration payload (§3.15.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCFGSetIsIgnoredAndDrawsNoCFGACK`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cp_test.go#L276) | unit/verify | unproven |
| positive | [`TestCFGSetIsIgnoredAndDrawsNoCFGACK`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cp_test.go#L272) | unit/verify | unproven |

### [`RFC7296-3.15.1-7`](#rfc7296-3.15.1-7)

If no attributes were accepted, the responder MUST return either an empty CFG_ACK payload or a response message without a CFG_ACK payload (§3.15.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCFGSetIsIgnoredAndDrawsNoCFGACK`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cp_test.go#L284) | unit/verify | unproven |
| positive | [`TestCFGSetIsIgnoredAndDrawsNoCFGACK`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cp_test.go#L280) | unit/verify | unproven |

### [`RFC7296-2.4-2`](#rfc7296-2.4-2)

If there has only been outgoing traffic on all of the SAs associated with an IKE SA, it is essential to confirm liveness of the other endpoint to avoid black holes.  If no cryptographically protected messages have been received on an IKE SA or any of its Child SAs recently, the system needs to perform a liveness check in order to prevent sending messages to a dead peer. (§2.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. No test carries an RFC requirement: RFC7296-2.4-2 tag; the only mentions are prose in plan/immediate/spec-ike-dpd-demand-driven.md, so nothing can go red on non-compliance and the row is unproven. The behaviour exists: newDPDState (internal/component/ike/engine/dpd.go) arms a periodic empty-INFORMATIONAL probe for every peer, the YANG dead-peer-detection interval defaulting to 30 seconds with range 1..3600, so an IKE SA that has received nothing protected recently is still checked. Owed: a tagged unit in both polarities (probe sent when no protected message arrived within the interval; none while a probe is outstanding), which the demand-driven DPD spec already plans.

No test carries RFC7296-2.4-2, so no unit is bound to it.

### [`RFC7296-2.4-3`](#rfc7296-2.4-3)

Since IKE is designed to operate in spite of DoS attacks from the network, an endpoint MUST NOT conclude that the other endpoint has failed based on any routing information (e.g., ICMP messages) or IKE messages that arrive without cryptographic protection (e.g., Notify messages complaining about unknown SPIs). (§2.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Re-read 2026-09-27; the quote was trimmed to the MUST NOT sentence (the MUST-conclude-only sentence is RFC7296-2.4-11). Two cases. Unprotected IKE messages: TestRteUnownedEstablishedSATrustsNothing fails if a cleartext Delete or CREATE_CHILD_SA moves the SA off Established; TestResponderAcceptsReinitAfterStaleSA and TestResponderKeepsOldSAOnUnauthenticatedInit fail if an unauthenticated IKE_SA_INIT disturbs the established SA. Routing information (e.g., ICMP): no tagged unit injects an ICMP error or unreachable and asserts the SA survives, so that clause has no assertion.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestResponderKeepsOldSAOnUnauthenticatedInit`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/responder_test.go#L731) | unit/verify | unproven |
| negative | [`TestRteUnownedEstablishedSATrustsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_routing_test.go#L47) | unit/verify | unproven |
| positive | [`TestResponderAcceptsReinitAfterStaleSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/responder_test.go#L680) | unit/verify | unproven |
| positive | [`TestRteUnownedEstablishedSATrustsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_routing_test.go#L53) | unit/verify | unproven |

### [`RFC7296-2.4-4`](#rfc7296-2.4-4)

The INITIAL_CONTACT notification, if sent, MUST be in the first IKE_AUTH request or response, not as a separate exchange afterwards (§2.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read 2026-09-27. Forbidden: INITIAL_CONTACT sent outside the first IKE_AUTH. TestInitiatorEmitsInitialContact fails if the responder did not parse it from the first IKE_AUTH; TestInitialContactAbsentFromRekey fails if an IKE rekey CREATE_CHILD_SA carries it. buildAuthRequest (auth.go) is the only producer of NotifyInitialContact, so no other exchange can carry it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInitialContactAbsentFromRekey`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_test.go#L330) | unit/verify | unproven |
| positive | [`TestInitiatorEmitsInitialContact`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/responder_test.go#L829) | unit/verify | unproven |

### [`RFC7296-3.4-2`](#rfc7296-3.4-2)

This Diffie-Hellman Group Num MUST match a Diffie-Hellman group specified in a proposal in the SA payload that is sent in the same message (§3.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestResInitiatorRejectsKEGroupOutsideTheAcceptedOffer`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_residual_test.go#L332) | unit/verify | unproven |
| negative | [`TestKesaKEGroupNotOfferedIsRejected`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_kesa_test.go#L70) | unit/verify | unproven |
| positive | [`TestKesaKEGroupOfferedIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_kesa_test.go#L38) | unit/verify | unproven |

### [`RFC7296-3.4-3`](#rfc7296-3.4-3)

If none of the proposals in that SA payload specifies a Diffie-Hellman group, the KE payload MUST NOT be present (§3.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestKesaKEWithoutDHProposalIsRejected`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_kesa_test.go#L100) | unit/verify | unproven |
| positive | [`TestKesaAbsentKEIsAlwaysAllowed`](https://github.com/ze-software/ze/blob/main/internal/component/ike/wire/rfc7296_kesa_test.go#L127) | unit/verify | unproven |

### [`RFC7296-3.3.6-8`](#rfc7296-3.3.6-8)

If one of the proposals offered is for the Diffie-Hellman group of NONE, and the responder selects that Diffie-Hellman group, then it MUST ignore the initiator's KE payload and omit the KE payload from the response (§3.3.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestResIKESANeverSelectsDHGroupNone`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_residual_test.go#L364) | unit/verify | unproven |
| positive | [`TestResSelectedDHGroupNoneOmitsKEFromTheResponse`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_residual_test.go#L405) | unit/verify | unproven |

### [`RFC7296-2.4-14`](#rfc7296-2.4-14)

This notification MUST NOT be sent by an entity that may be replicated (§2.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestResInitialContactNotSentByReplicableIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_residual_test.go#L77) | unit/verify | unproven |
| positive | [`TestResInitialContactSentByNonReplicableIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_residual_test.go#L52) | unit/verify | unproven |

### [`RFC7296-2.8-8`](#rfc7296-2.8-8)

When the lifetime of a Security Association expires, the Security Association MUST NOT be used (§2.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestResExpiredSAIsNotUsed`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_residual_test.go#L132) | unit/verify | unproven |
| positive | [`TestResRekeyLeadLeavesRoomBeforeHardExpiry`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_residual_test.go#L169) | unit/verify | unproven |
| positive | [`TestResUnexpiredSAIsUsed`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_residual_test.go#L97) | unit/verify | unproven |

### [`RFC7296-4-1`](#rfc7296-4-1)

If the responder rejects the CREATE_CHILD_SA request with a NO_ADDITIONAL_SAS notification, the implementation MUST be capable of instead deleting the old SA and creating a new one (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestResOtherRekeyFailuresDoNotReestablish`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_residual_test.go#L279) | unit/verify | unproven |
| positive | [`TestResNoAdditionalSAsTriggersReestablish`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_residual_test.go#L260) | unit/verify | unproven |

### [`RFC7296-2.15-3`](#rfc7296-2.15-3)

It MUST also accept a hex encoding of the shared secret (§2.15)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestHexEncodingIsExplicitAndNeverGuessed`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc7296_test.go#L328) | unit/verify | unproven |
| positive | [`TestPSKAcceptsHexEncoding`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc7296_test.go#L283) | unit/verify | unproven |

### [`RFC7296-3.3.4-4`](#rfc7296-3.3.4-4)

All implementations of IKEv2 MUST include a management facility that enables a user or system administrator to specify the suites that are acceptable for use with IKE (§3.3.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIKESuitePolicyRejectsAnUnhonourableSuite`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc7296_test.go#L198) | unit/verify | unproven |
| positive | [`TestIKESuitePolicyIsOperatorSpecified`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc7296_test.go#L126) | unit/verify | unproven |

### [`RFC7296-3.5-2`](#rfc7296-3.5-2)

To assure maximum interoperability, implementations MUST be configurable to send at least one of ID_IPV4_ADDR, ID_FQDN, ID_RFC822_ADDR, or ID_KEY_ID (§3.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLocalIDIsOperatorControlledNotDerived`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_identity_test.go#L93) | unit/verify | unproven |
| positive | [`TestLocalIDTypeFollowsConfiguredIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_identity_test.go#L40) | unit/verify | unproven |

### [`RFC7296-3.5-3`](#rfc7296-3.5-3)

MUST be configurable to accept all of these four types. (§3.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read 2026-09-27. Forbidden: refusing ID_IPV4_ADDR, ID_FQDN, ID_RFC822_ADDR or ID_KEY_ID under configuration. TestRemoteIDAcceptsEveryMandatoryType fails if checkRemoteIdentity refuses any of the four with a matching remote-id, and (anti-vacuity) if it accepts a wrong one. TestRemoteIDRefusesTypesItCannotCompare shows acceptance is per-type; its comment names ID_DER_ASN1_DN as refused but the body tests only ID_DER_ASN1_GN (stale comment).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRemoteIDRefusesTypesItCannotCompare`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_identity_test.go#L188) | unit/verify | unproven |
| positive | [`TestRemoteIDAcceptsEveryMandatoryType`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_identity_test.go#L133) | unit/verify | unproven |

### [`RFC7296-3.5-4`](#rfc7296-3.5-4)

IPv6-capable implementations MUST additionally be configurable to accept ID_IPV6_ADDR (§3.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPv6IdentityLengthIsEnforced`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_identity_test.go#L256) | unit/verify | unproven |
| positive | [`TestRemoteIDAcceptsIPv6Identity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_identity_test.go#L217) | unit/verify | unproven |

### [`RFC7296-3.6-1`](#rfc7296-3.6-1)

Implementations MUST be capable of being configured to send and accept up to four X.509 certificates in support of authentication (§3.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCcnCertificateCountIsBoundedAndConfigurable`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cert_chain_test.go#L293) | unit/verify | unproven |
| negative | [`TestCcnCertificateCountReachesFourInBothDirections`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cert_chain_test.go#L232) | unit/verify | unproven |
| negative | [`TestCcnOverlongChainKillsTheSAOnBothRoles`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cert_chain_test.go#L794) | unit/verify | unproven |
| negative | [`ipsec-certificate-count-range.ci`](https://github.com/ze-software/ze/blob/main/test/parse/ipsec-certificate-count-range.ci#L9) | functional/verify | unproven |
| positive | [`TestCcnCertificateCountReachesFourInBothDirections`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cert_chain_test.go#L230) | unit/verify | unproven |

### [`RFC7296-3.6-2`](#rfc7296-3.6-2)

and also MUST be capable of being configured to send and accept the two Hash and URL formats (with HTTP URLs). (§3.6)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Re-read 2026-09-27. Two clauses: send and accept both Hash and URL formats. Send: TestChuBothHashAndURLFormatsAreConfigurable fails unless buildCertPayloads emits encoding 12 and 13 with the SHA-1 and URL. Accept: the tagged units prove only the OFF default (TestChuHashAndURLIsOffByDefault drops a received encoding 12); no 3.6-2-tagged unit asserts that a received encoding 12 or 13 is resolved and accepted when configured. The .ci proves only that the leaves commit.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestChuBothHashAndURLFormatsAreConfigurable`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cert_chain_test.go#L350) | unit/verify | unproven |
| negative | [`TestChuHashAndURLIsOffByDefault`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cert_chain_test.go#L439) | unit/verify | unproven |
| positive | [`TestChuBothHashAndURLFormatsAreConfigurable`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cert_chain_test.go#L348) | unit/verify | unproven |
| positive | [`ipsec-hash-and-url-accepted.ci`](https://github.com/ze-software/ze/blob/main/test/parse/ipsec-hash-and-url-accepted.ci#L25) | functional/verify | unproven |

### [`RFC7296-3.6-3`](#rfc7296-3.6-3)

Implementations MUST support the "http:" scheme for hash-and-URL lookup (§3.6, §1.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestChuHashURLLookupRefusesEverythingOutsideTheBound`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cert_chain_test.go#L609) | unit/verify | unproven |
| negative | [`TestChuHashURLLookupUsesHTTPAndVerifiesTheHash`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cert_chain_test.go#L550) | unit/verify | unproven |
| positive | [`TestChuHashURLLookupCacheIsContentAddressed`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cert_chain_test.go#L717) | unit/verify | unproven |
| positive | [`TestChuHashURLLookupUsesHTTPAndVerifiesTheHash`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_cert_chain_test.go#L548) | unit/verify | unproven |

### [`RFC7296-4-4`](#rfc7296-4-4)

For an implementation to be called conforming to this specification, it MUST be possible to configure it to accept the following: o  Public Key Infrastructure using X.509 (PKIX) Certificates containing and signed by RSA keys of size 1024 or 2048 bits, where the ID passed is any of ID_KEY_ID, ID_FQDN, ID_RFC822_ADDR, or ID_DER_ASN1_DN. o  Shared key authentication where the ID passed is any of ID_KEY_ID, ID_FQDN, or ID_RFC822_ADDR. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read 2026-09-27. Forbidden: a configuration that cannot accept RSA-1024/2048 PKIX under ID_KEY_ID, ID_FQDN, ID_RFC822_ADDR, ID_DER_ASN1_DN, or PSK under ID_KEY_ID, ID_FQDN, ID_RFC822_ADDR. TestCfmConformanceConfigurationSetIsAcceptable fails if verifyRemoteAuth refuses RSA-1024+FQDN, RSA-2048+RFC822, RSA-2048+DN, RSA-2048+KEY_ID or PSK with each of the three; TestCfmConformanceSetDoesNotAcceptWhatItMustNot shows a mismatched identity is refused; the .ci tests show the DN remote-id commits and remote-id-type is a closed set. Every key size and every ID type is covered, not the full cross product.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCfmConformanceConfigurationSetIsAcceptable`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_conformance_test.go#L134) | unit/verify | unproven |
| negative | [`TestCfmConformanceSetDoesNotAcceptWhatItMustNot`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_conformance_test.go#L226) | unit/verify | unproven |
| negative | [`ipsec-remote-id-type-enum.ci`](https://github.com/ze-software/ze/blob/main/test/parse/ipsec-remote-id-type-enum.ci#L14) | functional/verify | unproven |
| positive | [`TestCfmConformanceConfigurationSetIsAcceptable`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_conformance_test.go#L132) | unit/verify | unproven |
| positive | [`ipsec-hash-and-url-accepted.ci`](https://github.com/ze-software/ze/blob/main/test/parse/ipsec-hash-and-url-accepted.ci#L28) | functional/verify | unproven |

### [`RFC7296-4-5`](#rfc7296-4-5)

Every implementation MUST be capable of doing four-message IKE_SA_INIT and IKE_AUTH exchanges establishing two SAs (one for IKE, one for ESP or AH) (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFourmFirstPairEstablishesNeitherSA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7296_fourmessage_test.go#L26) | unit/verify | unproven |
| positive | [`TestResponderHandshakePSKEndToEnd`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/responder_test.go#L345) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | six /ze-implement agents, one per section range, spec-rfcgate-1b-rfc7296-pilot AC-5; merged and re-checked in the main thread |
| Signed off | 2026-08-18 |
| Register | rfc2119 |
| Source | rfc/full/rfc7296.txt |
| Source fingerprint | a6f1a101b818977b |
| Record | rfc/extraction/rfc7296.json |
| Mapped sentences | 230 |
| Declined as scope | 19 |
| Relocated to a spec, which Ze OWES | 12 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | The abstract, the status of this memo, the copyright notice and the table of contents. No protocol obligation appears before Section 1. |
| `1` | not stated | 1 | walked | The introduction names the exchange types and their order. Its one keyword site is the exchange-order sentence, mapped to RFC7296-1-1. The rest of the section describes what each exchange carries. |
| `1.1` | not stated | 0 | walked | One paragraph introducing the four usage scenarios below it. It states no obligation. |
| `1.1.1` | not stated | 0 | walked | A figure and one paragraph on the security gateway to security gateway tunnel. It describes a deployment, not protocol behavior. |
| `1.1.2` | The endpoint-to-endpoint transport mode scenario | 0 | walked | The endpoint-to-endpoint transport mode scenario. Its only keyword is a MAY on application-layer access control, which gates nothing. |
| `1.1.3` | not stated | 0 | walked | The endpoint to security gateway tunnel scenario, including why the initiator asks for an address through configuration payloads. It states no obligation. |
| `1.1.4` | not stated | 0 | walked | One paragraph on nested combinations of the earlier scenarios. It states no obligation. |
| `1.2` | The initial exchanges | 5 | walked | The initial exchanges. Five keyword sites map to rows RFC7296-1.2-2 through RFC7296-1.2-6. Row RFC7296-1.2-1 is read from two indicative sentences, that the initial exchanges are four messages and that every message after IKE_SA_INIT is cryptographically protected, and neither carries a keyword. |
| `1.3` | The CREATE_CHILD_SA exchange | 3 | walked | The CREATE_CHILD_SA exchange. Two of its three keyword sites map to RFC7296-1.3-1 and RFC7296-1.3-2; the third restates the first. The NO_ADDITIONAL_SAS paragraph carries no keyword and no row. |
| `1.3.1` | Creating a new Child SA | 2 | walked | Creating a new Child SA. Both keyword sites sit in the USE_TRANSPORT_MODE paragraph and map to RFC7296-1.3.1-1 and RFC7296-1.3.1-2. The TFC padding and non-first-fragment paragraphs carry no keyword. |
| `1.3.2` | Rekeying the IKE SA | 1 | walked | Rekeying the IKE SA. Its one keyword site makes the KEi payload mandatory, and row RFC7296-1.3.3-1 holds that obligation under an id and a citation that name Section 1.3.3 instead. |
| `1.3.3` | Rekeying a Child SA | 1 | walked | Rekeying a Child SA. Its one keyword site binds the REKEY_SA notification into a CREATE_CHILD_SA exchange that replaces an ESP or AH SA. No row stated that obligation when the walk began: it was extracted during the walk as RFC7296-1.3.3-2, which site 1.3.3:1 now maps to. Note that RFC7296-1.3.3-1 is a different row whose obligation sits in section 1.3.2; its anchor is a recorded Known Limitation and was deliberately left alone. |
| `1.4` | The INFORMATIONAL exchange | 4 | walked | The INFORMATIONAL exchange. Four keyword sites map to RFC7296-1.4-5, to RFC7296-1.4-3 twice, and to RFC7296-1.4-4. |
| `1.4.1` | Deleting an SA | 6 | walked | Deleting an SA. Six keyword sites map to RFC7296-1.4.1-1 twice and to RFC7296-1.4.1-4 through RFC7296-1.4.1-7. Row RFC7296-1.4-1 is read from the sentence that the response carries Delete payloads for the paired SAs in the other direction, which carries no keyword; that row cites Section 1.4. |
| `1.5` | Informational messages outside an IKE SA | 1 | walked | Informational messages outside an IKE SA. Its one keyword site bars an answer to the unprotected message and maps to RFC7296-1.5-1. The closing paragraphs that build the response carry no keyword, and row RFC7296-2.21.4-2 cites Section 2.21.4 first. |
| `1.6` | Requirements terminology | 0 | walked | Requirements terminology. It points at [IPSECARCH] for the primitive terms and gives the RFC 2119 reading of the keywords. It states no protocol obligation. |
| `1.7` | The change list against RFC 4306 | 7 | walked | The change list against RFC 4306. Three sites carry obligations that live in Sections 2.5, 2.13 and 3.6, two are prose about the keywords themselves, one quotes the Section 1.3.2 change, and one states the configuration attribute type 5 rule that no row in rfc/short/rfc7296.md holds. |
| `1.8` | The change list against RFC 5996 | 0 | walked | The change list against RFC 5996. Every entry names a section that was added, clarified or corrected. It states no obligation. |
| `2` | IKE protocol details and variations | 2 | walked | IKE protocol details and variations. Two keyword sites: the 1280-octet message size and the four zero octets that prefix an IKE message on port 4500. |
| `2.1` | Retransmission timers | 7 | walked | Retransmission timers. Seven keyword sites map to rows RFC7296-2.1-3 through RFC7296-2.1-8. The half-open IKE SA paragraphs and the one-way message paragraph that follow carry no keyword and no row. |
| `2.2` | Message ID sequence numbers | 2 | walked | Message ID sequence numbers. Two keyword sites map to RFC7296-2.2-1 and RFC7296-2.2-2. The counter paragraph carries no keyword and states an obligation the capitalised scan cannot see: each endpoint maintains two INDEPENDENT current Message IDs. It is recorded as RFC7296-2.2-3 in unsourced-ids. The original-initiator paragraph carries no keyword and states no obligation. |
| `2.3` | Window size for overlapping requests | 9 | walked | Window size for overlapping requests. Six sites map to the Section 2.3 rows, two map to RFC7296-2.1-5 which cites Section 2.1 and Section 2.3, and one restates the window limit that site 2.3:4 maps. |
| `2.4` | not stated | 8 | walked | Walked the whole section: state loss after a crash, INITIAL_CONTACT, liveness detection, retransmission, and Child SA deletion. The eight sites carry every capitalised obligation. The empty INFORMATIONAL liveness exchange is described in indicative prose with no keyword. |
| `2.5` | not stated | 11 | walked | Walked the version number and forward compatibility rules, including the critical flag paragraph. Eleven sites carry the obligations. Two sentences each carry two obligations while deriving one site, so the second obligation of each has no site. |
| `2.6` | not stated | 4 | walked | Walked the SPI text, the cookie exchange, the worked cookie construction, and the terminology note. Four sites carry the obligations. The identification of an IKE SA by the SPI pair is stated in indicative prose in the first paragraph. |
| `2.6.1` | not stated | 1 | walked | Walked the interaction of COOKIE and INVALID_KE_PAYLOAD, including the three round trip diagram. The section states one obligation, in its closing sentence, and the site carries it. |
| `2.7` | not stated | 5 | walked | Walked the algorithm negotiation rules. The five sites state one rule set that the summary captures in two rows, so three sites restate obligations those rows already carry. |
| `2.8` | not stated | 4 | walked | Walked the rekeying section: lifetime expiry, rekey failure, IKE SA replacement, proactive rekey, and the window where the two ends disagree about an SA. Four sites carry the capitalised obligations. Independent lifetime policy is written as a difference from IKEv1 in indicative prose. |
| `2.8.1` | not stated | 1 | walked | Walked the simultaneous Child SA rekeying rules and both packet loss traces. The site carries the accept-on-either-SA obligation. The lowest-of-four-nonces collision rule is written as a SHOULD in this section, so it derives no MUST-level site. |
| `2.8.2` | not stated | 1 | walked | Walked the simultaneous IKE SA rekeying case, the TEMPORARY_FAILURE path, and the message trace. The site carries the one capitalised obligation, that the surviving IKE SA inherits the Child SAs. |
| `2.8.3` | not stated | 0 | walked | Walked the comparison of IKE SA rekeying against reauthentication. The section states what each mechanism does and how reauthentication is built from the existing exchanges. It imposes no obligation and derives no site. |
| `2.9` | not stated | 1 | walked | Walked the traffic selector negotiation text, including the four narrowing bullets and the SINGLE_PAIR_REQUIRED discussion. One bullet carries a capitalised MUST and is mapped. The TS_UNACCEPTABLE bullet and the narrowing-not-widening rule are written in indicative prose. |
| `2.9.1` | not stated | 0 | walked | Walked the section on selectors that violate the initiator's own policy. It is a worked example of dropped traffic and closes with a general statement of the hazard. It imposes no obligation and derives no site. |
| `2.9.2` | not stated | 2 | walked | Walked the rekeying selector rules. The two sites carry the two prohibitions, one binding the new SA and one binding the responder. |
| `2.10` | not stated | 1 | walked | Walked the nonce section. One sentence carries three obligations and derives one site, so the two size floors have no site of their own. |
| `2.11` | not stated | 2 | walked | Walked the address and port agility section, four sentences long. It derives two sites. The first sentence carries two obligations, so the second has no site of its own. |
| `2.12` | not stated | 1 | walked | Walked the Diffie-Hellman exponential reuse section. Its one obligation is the forget-the-keys sentence, which the site carries. The reuse strategies are stated as choices that do not affect interoperability. |
| `2.13` | not stated | 4 | walked | Walked the keying material section through the prf+ definition and its 255-iteration limit. The four sites carry the key size and key derivation obligations. The prf+ construction is stated as a definition. |
| `2.14` | not stated | 1 | walked | The section derives SKEYSEED and the seven SK_ values with prf and prf+. Its one MUST-level sentence fixes the length of SK_d, SK_pi and SK_pr, and the summary carries that obligation as RFC7296-2.13-3, which cites both sections. |
| `2.15` | not stated | 2 | walked | The section defines the signed octets and the pre-shared-key AUTH computation. Its three MUST-level obligations sit in one management-interface passage that the extractor cut into two sites, so the null-terminator half has no site of its own and is recorded unsourced. |
| `2.16` | not stated | 8 | walked | Every MUST-level sentence of the EAP section derives a site, and the eight sites map one to one onto the eight requirement ids anchored to this section. The remaining text is the message diagram and SHOULD NOT or MAY guidance. |
| `2.17` | not stated | 2 | walked | The section gives KEYMAT and three ordering bullets. Both MUST-level sites map to the two §2.17 rows, and the first bullet's ordering rule is already inside the text of RFC7296-2.17-1. |
| `2.18` | not stated | 3 | walked | The section derives SKEYSEED for a rekeyed IKE SA and states two obligations: a new Diffie-Hellman exchange, and a message-counter reset. Both §2.18 rows are the target of a site. |
| `2.19` | not stated | 6 | walked | Two of the six MUST-level sentences map to the two §2.19 rows the summary carries. The other four are the Configuration payload obligations that the 2026-07-31 owner ruling moved to plan/immediate/spec-ipsec-remote-access.md as RFC7296-2.19-2, -3, -5 and -6; those ids are not in rfc/short/rfc7296.md, so their sites stay unclassified. |
| `2.20` | not stated | 1 | walked | The section shows the APPLICATION_VERSION exchange. Its one MUST states what a peer that declines to give version information returns, and RFC7296-2.20-1 carries it. |
| `2.21` | not stated | 0 | walked | The section states the general error-handling rule in indicative prose and MAY constructions: a badly formatted or unacceptable request draws a Notify, and a bad response draws none. It derives no MUST-level site and no summary row cites §2.21. |
| `2.21.1` | not stated | 0 | walked | IKE_SA_INIT error handling is written with 'should' throughout, because every notification at that point is unauthenticated. The section derives no site and no summary row cites §2.21.1. |
| `2.21.2` | not stated | 3 | walked | Three MUST-level sentences and three rows: whole-message rejection, the initiator not failing authentication over a piggybacked error, and the limit on new error notifications defined by extension documents. |
| `2.21.3` | not stated | 1 | walked | The section's one MUST requires a response for every errored request once the IKE SA is authenticated. The rest is SHOULD NOT guidance against starting new exchanges to report errors. |
| `2.21.4` | not stated | 5 | walked | Five sites carry the out-of-SA rules for a message on port 500 or 4500 with no known IKE SA. Two of those sentences state two obligations each, so RFC7296-2.21.4-4 and -6 have no site of their own and are recorded unsourced. |
| `2.22` | not stated | 2 | walked | Both MUST-level sentences are IPComp obligations: where an IPCOMP_SUPPORTED notification can appear, and what an implementation accepts and compresses with. The 2026-07-31 owner ruling moved the four §2.22 rows to plan/spec-ipsec-ipcomp.md, so no id exists in rfc/short/rfc7296.md and both sites stay unclassified. |
| `2.23` | not stated | 10 | walked | Nine of the ten sites map to a summary row and one is the section's own applicability statement. RFC7296-2.23-3, the four zero octets prepended to a tunnelled IKE header, and RFC7296-2.23-12, the SPI rule that lets a receiver tell ESP from IKE on port 4500, are stated in indicative prose with no MUST-level keyword, so they are recorded unsourced. |
| `2.23.1` | not stated | 3 | walked | Read the transport mode NAT traversal scenario and the client and responder rule lists. Three MUST sites, all mapped; every other rule in the section is SHOULD or MAY. |
| `2.24` | not stated | 1 | walked | Read the ECN section. Its one normative sentence carries two MUSTs and yields one site, so RFC7296-2.24-2 (the [IPSECARCH] encapsulation and decapsulation processing) has no site of its own. |
| `2.25` | not stated | 1 | walked | Read the exchange collision section. The TEMPORARY_FAILURE retry ban is its only MUST; the TEMPORARY_FAILURE and CHILD_SA_NOT_FOUND sending rules are SHOULD. |
| `2.25.1` | not stated | 0 | walked | Read the Child SA rekey and close collision rules. Every rule in the section is SHOULD, so the section holds no MUST-level site and no requirement row cites it. |
| `2.25.2` | not stated | 0 | walked | Read the IKE SA rekey and close collision rules. Every rule in the section is SHOULD, so the section holds no MUST-level site and no requirement row cites it. |
| `3` | not stated | 0 | walked | Read the section head. Its one normative sentence is a SHOULD NOT about items marked UNSPECIFIED, so the section holds no MUST-level site. |
| `3.1` | not stated | 13 | walked | Read the IKE header section and every field description. Two rows share a source sentence with the row mapped from the same site: RFC7296-3.1-2 (no Encrypted payload inside an Encrypted payload) sits in site 3.1:1, and RFC7296-3.1-8 (ignore X bits on receipt) sits in site 3.1:9. |
| `3.2` | not stated | 6 | walked | Read the generic payload header section, the payload type table and the Critical bit paragraph. All six sites map. Sites 3.2:1 and 3.2:2 state the sender obligations for the Critical bit; they were unextracted and are now carried by RFC7296-3.2-5 and RFC7296-3.2-6. |
| `3.3` | not stated | 7 | walked | Read the Security Association payload section, its nesting rules and its proposal example. RFC7296-3.3-1 (combined-mode ciphers and normal ciphers go in separate proposals) is written with a lowercase must, so it has no capitalised site. |
| `3.3.1` | not stated | 4 | walked | Read the proposal substructure and every field description. Four sites, all mapped, two of them to the RESERVED rows that cite this section. |
| `3.3.2` | not stated | 1 | walked | Read the transform substructure, the Transform Type table and the Transform ID tables. RFC7296-3.3.2-1 (an IKE SA proposal carries ENCR, PRF, INTEG and D-H) is read from the Used In column of that table, which carries no keyword. |
| `3.3.3` | not stated | 1 | walked | Read the valid Transform Types section and its mandatory and optional table. Its one MUST is mapped. |
| `3.3.4` | not stated | 5 | walked | Read the mandatory Transform IDs section. Three MUSTs bind the implementation and are mapped; two sentences use the keyword to describe other documents' suite specifications and are excluded. |
| `3.3.5` | not stated | 5 | walked | Read the transform attribute section, the attribute format figure and the Key Length rules. Five sites, all mapped; the interoperability note at the end is SHOULD. |
| `3.3.6` | not stated | 8 | walked | Read the whole section: the responder selection rules, the handling of an unknown Transform Type or Transform Attribute, and the three Diffie-Hellman negotiation paragraphs. RFC7296-3.3.6-1 has no site here, because Section 3.3.6 treats Diffie-Hellman negotiation in indicative prose and the mandate that an IKE proposal carries a D-H transform is the mandatory Transform Type table of Section 3.3.3. |
| `3.4` | not stated | 4 | walked | Read the Key Exchange payload section, the payload figure and the two paragraphs that follow it. All four sites map, and the INVALID_KE_PAYLOAD rejection maps to the row the summary anchors at Section 1.3. |
| `3.5` | not stated | 5 | walked | Read the Identification payload section, the ID Type table and the interoperability paragraph. RFC7296-3.5-3 has no site of its own: one sentence carries the send obligation and the accept obligation together, and site 3.5:4 maps the send half. |
| `3.6` | not stated | 3 | walked | Read the Certificate payload section, the Certificate Encoding table, the CertBundle ASN.1 module and the two closing paragraphs. RFC7296-3.6-2 has no site of its own: one sentence carries the four-certificate obligation and the Hash and URL obligation together, and site 3.6:1 maps the first half. |
| `3.7` | not stated | 0 | walked | Read the Certificate Request payload section in full. It derives no site: its obligations are written at SHOULD and MAY level, such as CERTREQ payloads MAY be included and a certificate chain SHOULD be sent back to the requestor. The summary declares no requirement anchored at Section 3.7. |
| `3.8` | not stated | 1 | walked | Read the Authentication payload section and the Auth Method table. Its one site is the RESERVED field rule, which restates the forward compatibility rule of Section 2.5. The only summary row anchored at Section 3.8 is a SHOULD and never gates. |
| `3.9` | not stated | 2 | walked | Read the Nonce payload section. Both sites map: the 16 to 256 octet size range, and the ban on reusing a nonce value. |
| `3.10` | not stated | 3 | walked | Read the Notify payload section and its field list. All three sites map to the Protocol ID and SPI Size rules that the field descriptions state. |
| `3.10.1` | not stated | 4 | walked | Read the Notify Message Types section, including the error type table and the status type table. Three sites map to the error handling paragraph and the INVALID_SYNTAX entry, and the fourth repeats the ignore rule for status types. |
| `3.11` | not stated | 3 | walked | Read the Delete payload section and its field list. Two sites carry the same one-protocol-per-payload rule, and the third carries the SPI Size rule. |
| `3.12` | not stated | 3 | walked | Read the Vendor ID payload section. All three sites map: the critical bit rule, the ignore rule for an unfamiliar Vendor ID, and the obligation on writers of documents that extend the protocol. |
| `3.13` | not stated | 1 | walked | Read the Traffic Selector payload section, its field list and the TSi/TSr matching example. Its one site is the RESERVED field rule of Section 2.5. The summary declares no requirement anchored at Section 3.13. |
| `3.13.1` | not stated | 4 | walked | Read the Traffic Selector substructure section, the Start Port and End Port descriptions, and the ANY and OPAQUE paragraph. Three sites map and one restates the port encoding that the first two carry. |
| `3.14` | not stated | 6 | walked | Read the Encrypted payload section, its field list, and the padding and checksum rules. RFC7296-3.14-3 has no site of its own: one sentence binds the sender to a new unpredictable IV and the recipient to accept any value, and site 3.14:3 maps the sender half. |
| `3.15` | Configuration payload header | 1 | walked | Configuration payload header. The one site is the 3-octet RESERVED field; the rest of the section names payload type 47, lists the four CFG Type values and says the attribute list can be empty. |
| `3.15.1` | not stated | 8 | walked | Configuration Attribute format, the attribute-type table, the per-attribute entries and the CFG_REQUEST/CFG_REPLY and CFG_SET/CFG_ACK paragraphs. Eight sites. Three of them state obligations rfc/short/rfc7296.md carries no row for, recorded on those sites. |
| `3.15.2` | not stated | 0 | walked | The meaning of INTERNAL_IP4_SUBNET and INTERNAL_IP6_SUBNET, given as four worked CFG_REPLY examples. It says what a gateway conveys with the attribute and closes by saying the attribute cannot be used reliably in a CFG_REQUEST. No sentence obliges a speaker. |
| `3.15.3` | Configuration payloads for IPv6 | 0 | walked | Configuration payloads for IPv6. One worked exchange, the gateway's address-selection behavior at MAY level, and the limitation that an IKEv2 tunnel is not a full IPv6 interface. No obligation is stated. |
| `3.15.4` | Address assignment failures | 0 | walked | Address assignment failures. The section states in indicative prose that a responder which cannot assign an address responds with INTERNAL_ADDRESS_FAILURE, and sends it only when no address can be assigned. That obligation carries no capitalised keyword and rfc/short/rfc7296.md declares no row for it, so unsourced-ids cannot name it. It needs a checklist row. |
| `3.16` | The EAP payload and the EAP message format | 4 | walked | The EAP payload and the EAP message format. Four sites, one per field rule: Identifier, Length, Type presence and Type value. Each has its own row. |
| `4` | Conformance requirements | 10 | walked | Conformance requirements. Ten sites: the section opener, the two payload-handling rules, the minimal-implementation paragraph, the three Configuration payload conditionals and the configurable-authentication list. RFC7296-3.2-3 is listed unsourced because Section 4 states it in the same sentence as site 4:2, which is mapped to RFC7296-2.5-8. |
| `5` | Security considerations | 3 | walked | Security considerations. Three capitalised sites: the PRF output floor, the ban on NONE and ENCR_NULL inside the IKE SA, and public-key authentication of the server before EAP starts. The rest of the section is analysis and SHOULD-level advice. |
| `5.1` | Traffic selector authorization | 0 | walked | Traffic selector authorization. It describes the Peer Authorization Database of [IPSECARCH] and reads an assigned inner address as a temporary PAD entry. The constraints it names belong to that document; the section states none of its own. |
| `6` | IANA considerations | 0 | skipped (iana) | IANA considerations. It records that IANA registered the IKEv2 types and values in [IKEV2IANA], deprecated the Raw RSA Key certificate encoding and repointed the RFC 5996 references. |
| `7` | References heading | 0 | skipped (references) | References heading. |
| `7.1` | Normative references | 0 | skipped (references) | Normative references. |
| `7.2` | Informative references | 0 | skipped (references) | Informative references. |
| `A` | Summary of changes from IKEv1 | 0 | skipped (appendix-non-normative) | Summary of changes from IKEv1. Twelve numbered goals of the revision, written as a design record. It obliges no implementation. |
| `B` | Diffie-Hellman groups | 0 | walked | Diffie-Hellman groups. It defines two groups for use in IKE and says group 1 is kept for historic reasons. The prime and generator it carries are data, not a sentence that binds a speaker, so the section yields no site. |
| `B.1` | not stated | 0 | walked | Group 1, 768-bit MODP: the assigned id, the prime and the generator 2. Constants only. |
| `B.2` | not stated | 0 | walked | Group 2, 1024-bit MODP: the assigned id, the prime and the generator 2. Constants only. |
| `C` | Exchanges and payloads | 0 | skipped (appendix-non-normative) | Exchanges and payloads. The appendix says of itself that it is purely informative, and that the body of the document is correct wherever the two disagree. |
| `C.1` | not stated | 0 | skipped (appendix-non-normative) | IKE_SA_INIT payload diagram: request, normal response, cookie response and the INVALID_KE_PAYLOAD response. Diagram only, under Appendix C's informative statement. |
| `C.2` | not stated | 0 | skipped (appendix-non-normative) | IKE_AUTH without EAP: request, response and the Child SA error response. Diagram only, under Appendix C's informative statement. |
| `C.3` | not stated | 0 | skipped (appendix-non-normative) | IKE_AUTH with EAP: first pair, the repeated EAP pair and the last pair. Diagram only, under Appendix C's informative statement. |
| `C.4` | not stated | 0 | skipped (appendix-non-normative) | CREATE_CHILD_SA for creating or rekeying Child SAs, with the error and INVALID_KE_PAYLOAD responses. Diagram only, under Appendix C's informative statement. |
| `C.5` | not stated | 0 | skipped (appendix-non-normative) | CREATE_CHILD_SA for rekeying the IKE SA, where KEi and KEr are mandatory. Diagram only, under Appendix C's informative statement. |
| `C.6` | not stated | 0 | skipped (appendix-non-normative) | INFORMATIONAL exchange: notify, delete and Configuration payloads in request and response. Diagram only, under Appendix C's informative statement. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1.3:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The sentence restates the one before it from the side of the KEi: the group the initiator expects the responder to accept is the group set it proposes, which the parenthesis about additional groups confirms. Site 1.3:1 maps RFC7296-1.3-1. | The Diffie-Hellman group of the KEi MUST be an element of the group the initiator expects the responder to accept (additional Diffie-Hellman groups can be proposed). |
| `1.7:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The sentence reports what IKEv2 developers observed about RFC 4306. The keyword names a class of requirement in that earlier document and imposes nothing. | They also have noted that there are MUST- level requirements that are not related to interoperability. |
| `1.7:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The sentence sets how a reader interprets the lower-case words should and must in this document. It is a reading convention, and the keyword is the word being defined. | All non-capitalized uses of the words SHOULD and MUST now mean their normal English sense, not the interoperability sense of [MUSTSHOULD]. |
| `1.7:3` | `relocated-to-spec` (Ze owes it): the obligation is real and unbuilt, and a named spec owes it | The obligation, ignore proposals carrying configuration attribute type 5, was moved out of rfc/short/rfc7296.md by owner ruling D-1 (2026-07-31) and is reserved as RFC7296-1.7-3 in plan/immediate/spec-ipsec-remote-access.md. It is gated at that destination, so nothing is dropped: this site records where the obligation went, and the tripwire reds if that spec or that id disappears. (relocated to plan/immediate/spec-ipsec-remote-access.md as RFC7296-1.7-3) | Implementations that conform to this document MUST ignore proposals that have configuration attribute type 5, the old value for INTERNAL_ADDRESS_EXPIRY. |
| `1.7:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The sentence quotes the Section 1.3.2 change that turned the KEi payload from SHOULD to MUST. Site 1.3.2:1 maps RFC7296-1.3.3-1, which is that obligation. | In Section 1.3.2, "The KEi payload SHOULD be included" was changed to be "The KEi payload MUST be included". |
| `2.3:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The RFC opens this sentence with 'In other words' and restates the window limit as a wait for the responses up through request X-N. Site 2.3:4 maps RFC7296-2.3-4. | In other words, if the responder stated its window size is N, then when the initiator needs to make a request X, it MUST wait until it has received responses to all requests up through request X-N. |
| `2.7:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | This sentence introduces the negotiation rules that follow it, and each of those rules restates the single-suite obligation with its own site. RFC7296-2.7-1 carries it, mapped from the exactly-one-transform-per-type site. | The responder MUST choose a single suite, which may be any subset of the SA proposal following the rules below. |
| `2.7:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Accepting one proposal or rejecting them all with an error is the second half of RFC7296-2.7-1, whose first half the exactly-one-transform-per-type site maps. | The responder MUST accept a single proposal or reject them all and return an error. |
| `2.7:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The sentence opens with 'For example' and applies the preceding rule to a named ESP proposal. It states no obligation the preceding sentence does not, and RFC7296-2.7-1 carries that obligation. | For example: if an ESP proposal includes transforms ENCR_3DES, ENCR_AES w/keysize 128, ENCR_AES w/keysize 256, AUTH_HMAC_MD5, and AUTH_HMAC_SHA, the accepted suite MUST contain one of the ENCR_ transforms and one of the AUTH_ transforms. |
| `2.19:2` | `relocated-to-spec` (Ze owes it): the obligation is real and unbuilt, and a named spec owes it | The obligation, the CP payload's position in the responder's payload chain, was moved out of rfc/short/rfc7296.md by owner ruling D-1 (2026-07-31) and is reserved as RFC7296-2.19-2 in plan/immediate/spec-ipsec-remote-access.md. It is gated at that destination, so nothing is dropped: this site records where the obligation went, and the tripwire reds if that spec or that id disappears. (relocated to plan/immediate/spec-ipsec-remote-access.md as RFC7296-2.19-2) | Initiator Responder ------------------------------------------------------------------- HDR, SK {IDi, [CERT,] [CERTREQ,] [IDr,] AUTH, CP(CFG_REQUEST), SAi2, TSi, TSr} --> <-- HDR, SK {IDr, [CERT,] AUTH, CP(CFG_REPLY), SAr2, TSi, TSr} In all cases, the CP payload MUST be inserted before the SA payload. |
| `2.19:3` | `relocated-to-spec` (Ze owes it): the obligation is real and unbuilt, and a named spec owes it | The obligation, never send an unsolicited CFG_REPLY, was moved out of rfc/short/rfc7296.md by owner ruling D-1 (2026-07-31) and is reserved as RFC7296-2.19-3 in plan/immediate/spec-ipsec-remote-access.md. It is gated at that destination, so nothing is dropped: this site records where the obligation went, and the tripwire reds if that spec or that id disappears. (relocated to plan/immediate/spec-ipsec-remote-access.md as RFC7296-2.19-3) | In variations of the protocol where there are multiple IKE_AUTH exchanges, the CP payloads MUST be inserted in the messages containing the SA payloads. |
| `2.19:5` | `relocated-to-spec` (Ze owes it): the obligation is real and unbuilt, and a named spec owes it | The obligation, emit FAILED_CP_REQUIRED when a required CP is absent, was moved out of rfc/short/rfc7296.md by owner ruling D-1 (2026-07-31) and is reserved as RFC7296-2.19-5 in plan/immediate/spec-ipsec-remote-access.md. It is gated at that destination, so nothing is dropped: this site records where the obligation went, and the tripwire reds if that spec or that id disappears. (relocated to plan/immediate/spec-ipsec-remote-access.md as RFC7296-2.19-5) | The responder MUST NOT send a CFG_REPLY without having first received a CP(CFG_REQUEST) from the initiator, because we do not want the IRAS to perform an unnecessary configuration lookup if the IRAC cannot process the REPLY. |
| `2.19:6` | `relocated-to-spec` (Ze owes it): the obligation is real and unbuilt, and a named spec owes it | The obligation, the SA state after a failed configuration exchange, was moved out of rfc/short/rfc7296.md by owner ruling D-1 (2026-07-31) and is reserved as RFC7296-2.19-6 in plan/immediate/spec-ipsec-remote-access.md. It is gated at that destination, so nothing is dropped: this site records where the obligation went, and the tripwire reds if that spec or that id disappears. (relocated to plan/immediate/spec-ipsec-remote-access.md as RFC7296-2.19-6) | In the case where the IRAS's configuration requires that CP be used for a given identity IDi, but IRAC has failed to send a CP(CFG_REQUEST), IRAS MUST fail the request, and terminate the Child SA creation with a FAILED_CP_REQUIRED error. |
| `2.22:1` | `relocated-to-spec` (Ze owes it): the obligation is real and unbuilt, and a named spec owes it | The obligation, IPComp association negotiation, was moved out of rfc/short/rfc7296.md by owner ruling D-1 (2026-07-31) and is reserved as RFC7296-2.22-1 in plan/spec-ipsec-ipcomp.md. It is gated at that destination, so nothing is dropped: this site records where the obligation went, and the tripwire reds if that spec or that id disappears. (relocated to plan/spec-ipsec-ipcomp.md as RFC7296-2.22-1) | These payloads MUST NOT occur in messages that do not contain SA payloads. |
| `2.22:2` | `relocated-to-spec` (Ze owes it): the obligation is real and unbuilt, and a named spec owes it | The obligation, IPComp CPI handling, was moved out of rfc/short/rfc7296.md by owner ruling D-1 (2026-07-31) and is reserved as RFC7296-2.22-2 in plan/spec-ipsec-ipcomp.md. It is gated at that destination, so nothing is dropped: this site records where the obligation went, and the tripwire reds if that spec or that id disappears. (relocated to plan/spec-ipsec-ipcomp.md as RFC7296-2.22-2) | Although there has been discussion of allowing multiple compression algorithms to be accepted and to have different compression algorithms available for the two directions of a Child SA, implementations of this specification MUST NOT accept an IPComp algorithm that was not proposed, MUST NOT accept more than one, and MUST NOT compress using an algorithm other than one proposed and accepted in the setup of the Child SA. |
| `2.23:6` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | An applicability statement, not an obligation: it tells the reader that the MUSTs in the bullet list below bind only implementations that support NAT traversal, which the preceding sentence calls optional. | In this section only, requirements listed as MUST apply only to implementations supporting NAT traversal. |
| `3.1:5` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The sentence sets the major version for implementations of previous IKE versions and of ISAKMP. Ze implements IKEv2 only, so the obligation binds an IKEv1 or ISAKMP implementation. The producer that would act as it if ze did is the IKE header codec, which writes and reads the version octet: `MajorVersion` (`internal/component/ike/wire/header.go`) is encoded at byte 17 and ze sets it for IKEv2 alone, so no IKEv1 or ISAKMP exchange exists here to carry the obligation. | Implementations based on previous versions of IKE and ISAKMP MUST set the major version to 1. |
| `3.3.4:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The keywords name the suite specifications that were removed from this document. The sentence records an editorial change and binds no implementation. | The specification of suites that MUST and SHOULD be supported for interoperability has been removed from this document because they are likely to change more rapidly than this document evolves. |
| `3.3.4:5` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The sentence removes an expectation: a suite that another document makes mandatory to implement need not sit in local policy. The keyword names that other document's obligation, not one this sentence imposes. | Note that cryptographic suites that MUST be implemented need not be configured as acceptable to local policy. |
| `3.3.6:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The sentence narrows the same selection rule to the proposal level. Site 3.3.6:1 maps RFC7296-2.7-1, which already binds the responder to one proposal or a rejection of all of them. | If there are multiple proposals, the responder MUST choose a single proposal. |
| `3.3.6:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The sentence narrows the same selection rule to one transform per Transform Type. Section 2.7 states it as the accepted suite containing exactly one transform of each type, which RFC7296-2.7-1 captures. | If the selected proposal has multiple transforms with the same type, the responder MUST choose a single one. |
| `3.5:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The ID_RFC822_ADDR entry repeats the terminator ban that the ID_FQDN entry states. Site 3.5:2 maps RFC7296-3.5-5, whose text names both identification types. | The string MUST NOT contain any terminators. |
| `3.10.1:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The paragraph on status types repeats the ignore rule of the preceding paragraph. Site 3.10.1:2 maps RFC7296-3.10.1-2, whose text already covers an unrecognized status type in a request or a response. | Notify payloads with status types MAY be added to any message and MUST be ignored if not recognized. |
| `3.11:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The mixing ban is the same obligation as the preceding sentence, written as a prohibition. Site 3.11:1 maps RFC7296-3.11-1, whose text carries both sentences. | Mixing of protocol identifiers MUST NOT be performed in the Delete payload. |
| `3.13.1:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | ANY ports means start port 0 and end port 65535, which is the port encoding the two field descriptions already state. Site 3.13.1:1 maps RFC7296-3.13.1-1 and site 3.13.1:2 maps RFC7296-3.13.1-2. | Systems that are complying with [IPSECARCH] that wish to indicate "ANY" ports MUST set the start port to 0 and the end port to 65535; note that according to [IPSECARCH], "ANY" includes "OPAQUE". |
| `3.15.1:2` | `relocated-to-spec` (Ze owes it): the obligation is real and unbuilt, and a named spec owes it | The obligation, at most one netmask, and only beside an INTERNAL_IP4_ADDRESS, was moved out of rfc/short/rfc7296.md by owner ruling D-1 (2026-07-31) and is reserved as RFC7296-3.15.1-1 in plan/immediate/spec-ipsec-remote-access.md. It is gated at that destination, so nothing is dropped: this site records where the obligation went, and the tripwire reds if that spec or that id disappears. (relocated to plan/immediate/spec-ipsec-remote-access.md as RFC7296-3.15.1-1) | Only one netmask is allowed in the request and response messages (e.g., 255.255.255.0), and it MUST be used only with an INTERNAL_IP4_ADDRESS attribute. |
| `3.15.1:4` | `relocated-to-spec` (Ze owes it): the obligation is real and unbuilt, and a named spec owes it | The obligation, a SUPPORTED_ATTRIBUTES request carries zero length, was moved out of rfc/short/rfc7296.md by owner ruling D-1 (2026-07-31) and is reserved as RFC7296-3.15.1-3 in plan/immediate/spec-ipsec-remote-access.md. It is gated at that destination, so nothing is dropped: this site records where the obligation went, and the tripwire reds if that spec or that id disappears. (relocated to plan/immediate/spec-ipsec-remote-access.md as RFC7296-3.15.1-3) | o SUPPORTED_ATTRIBUTES - When used within a Request, this attribute MUST be zero-length and specifies a query to the responder to reply back with all of the attributes that it supports. |
| `3.15.1:5` | `relocated-to-spec` (Ze owes it): the obligation is real and unbuilt, and a named spec owes it | The obligation, ignore attributes the responder does not recognize, was moved out of rfc/short/rfc7296.md by owner ruling D-1 (2026-07-31) and is reserved as RFC7296-3.15.1-4 in plan/immediate/spec-ipsec-remote-access.md. It is gated at that destination, so nothing is dropped: this site records where the obligation went, and the tripwire reds if that spec or that id disappears. (relocated to plan/immediate/spec-ipsec-remote-access.md as RFC7296-3.15.1-4) | Unrecognized or unsupported attributes MUST be ignored in both requests and responses. |
| `4:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The opening sentence of Section 4, which describes what the section contains. Its capitalised words sit inside the quoted phrase "MUST support", naming the class of requirements that follow. It binds nobody. | In order to assure that all implementations of IKEv2 can interoperate, there are "MUST support" requirements in addition to those listed elsewhere. |
| `4:8` | `relocated-to-spec` (Ze owes it): the obligation is real and unbuilt, and a named spec owes it | The obligation, parse the CFG_REQUEST and recognize the address attribute, was moved out of rfc/short/rfc7296.md by owner ruling D-1 (2026-07-31) and is reserved as RFC7296-4-2 in plan/immediate/spec-ipsec-remote-access.md. It is gated at that destination, so nothing is dropped: this site records where the obligation went, and the tripwire reds if that spec or that id disappears. (relocated to plan/immediate/spec-ipsec-remote-access.md as RFC7296-4-2) | If an implementation supports responding to such requests, it MUST parse the CP payload of type CFG_REQUEST in the first message in the IKE_AUTH exchange and recognize a field of type INTERNAL_IP4_ADDRESS or INTERNAL_IP6_ADDRESS. |
| `4:9` | `relocated-to-spec` (Ze owes it): the obligation is real and unbuilt, and a named spec owes it | The obligation, return a CFG_REPLY carrying an address of the requested type, was moved out of rfc/short/rfc7296.md by owner ruling D-1 (2026-07-31) and is reserved as RFC7296-4-3 in plan/immediate/spec-ipsec-remote-access.md. It is gated at that destination, so nothing is dropped: this site records where the obligation went, and the tripwire reds if that spec or that id disappears. (relocated to plan/immediate/spec-ipsec-remote-access.md as RFC7296-4-3) | If it supports leasing an address of the appropriate type, it MUST return a CP payload of type CFG_REPLY containing an address of the requested type. |

## Superseded

No document obsoletes RFC 7296, so its obligations are stated where they were written.
