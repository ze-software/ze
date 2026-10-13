# Spec: ike-eap-rfc-defects

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-13 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Two defects in Ze's IKEv2 NAT traversal and EAP-TLS, found by the strict re-read
of `spec-rfc-requirement-quote-hand-backfill`. A peer behind a
NAT that rewrote its IKE source port receives ESP on the wrong port, and an
EAP-TLS 1.2 full handshake carries no session_id. Both read at the producer in
HEAD on 2026-09-27.

## Defects

| ID | RFC section | Verbatim quote | Producer | What Ze does wrong | Audit verdict / record |
|----|-------------|----------------|----------|--------------------|------------------------|
| D1 | RFC 3948 Section 2.1 (RFC3948-2.1-2) | "the Source Port and Destination Port MUST be the same as that used by IKE traffic," | `internal/component/ike/engine/child.go::installChildSA` (port defaulting) and the Child SA build that sets `udpLocalPort`/`udpRemotePort` only when MOBIKE is enabled | Without MOBIKE both ports default to 4500, so a NAT-translated peer whose IKE arrives from port Y receives ESP on 4500 | wrong; lead confirmed by reading |
| D2 | RFC 5216 Section 2.1.1 (RFC5216-2.1.1-4) | "If the peer's sessionId is null or unrecognized by the server, the server MUST choose the sessionId to establish a new session." | `internal/core/eap/eap_tls.go::newTLSMethod` over Go `crypto/tls` | crypto/tls sets the ServerHello session_id only when resuming, echoing the client, so a TLS 1.2 full handshake sends an empty session_id and the server chooses no sessionId | wrong since 2026-09-27; journal `invariant-enforced-by-an-absent-call-site.md` |

## Status at HEAD (re-checked 2026-10-13, HEAD `3e2770062b`)

Every row below was read in `rfc/audit/rfc3948.json` / `rfc/audit/rfc5216.json` at HEAD, and at the
producer. `./le rfc check` at HEAD reports no rfc3948 or rfc5216 refusal.

| Requirement | Verdict at HEAD | Producer state | Owned here? |
|-------------|-----------------|----------------|-------------|
| RFC3948-2.1-2 | `wrong` | open: `installChildSA` (`child.go`) still defaults both encap ports to `transport.NATTPort` unless MOBIKE set `udpLocalPort`/`udpRemotePort` | yes, D1 |
| RFC5216-2.1.1-4 | `wrong` (re-judged 2026-10-12, note: "None of it bears on a sessionId") | open: go1.27.2 `crypto/tls` assigns `hs.hello.sessionId` only in `serverHandshakeState.doResumeHandshake` (`handshake_server.go`), echoing the client; `doFullHandshake` leaves it empty. No owner ruling recorded | yes, D2 |
| RFC5216-2.1.2-1 | `enforced` | CLOSED. Producer fix `37dbe54095` ("eap: the EAP-TLS authenticator never resumes a TLS 1.2 session", `authenticatorTickets.redeemTLS13Only` in `resumption.go`); independent re-judge `c1f83ffa35`, `96535408fa` | no; AC-4 is closed |
| RFC5216-2.1.1-5 | `enforced` | CLOSED by the same commits | no |
| RFC5216-2.1.1-10 | `enforced` | CLOSED by the same commits | no |

`plan/pre-release/spec-rfc-verdict-fix-ike-eap.md` (its "Blocked by" table and its AC-C1 row) still
points all four ids here. The three closed ids need that spec's rows updated by its owner; this spec
does not edit another spec.

### Defects found during this research (same requirement, RFC3948-2.1-2)

| ID | Producer | What Ze does wrong |
|----|----------|--------------------|
| D1b | `handleResponderEAP` (`responder_eap.go`), AUTH-from-MSK branch | The final IKE_AUTH builds and installs the Child SA (`buildAuthResponse`) BEFORE `finishResponderEstablish` calls `adoptAuthenticatedEndpoint`. `sa.peerEndpoint` is nil while the Child SA is built, so a port taken from the IKE SA would be the CONFIGURED remote's. The non-EAP path in `handleAuthRequest` adopts first on purpose (its comment: "Taking it only in finishResponderEstablish installs the Child SA one step too early to see the float"); the EAP path does not. A D1 fix alone leaves the EAP responder wrong |
| D1c | `rekey.go`, the rekeyed `ChildSA` literal | A rekeyed Child SA copies `udpLocalPort`/`udpRemotePort` from the retired Child SA instead of taking them from the IKE SA, against RFC 4555 Section 3.3 ("the tunnel header IP addresses (and port, if doing UDP encapsulation) are taken from the IKE_SA"). Without MOBIKE it copies zeros and so inherits D1 |
| D1d | `installChildSA` (`child.go`) | The LOCAL encap port is the constant 4500, not the port of the NAT-T socket the IKE SA sends from. Under the test-port hook (`nattAddr`, `testport.go`, port p+1) the two differ, so the local half of the same MUST holds only while the default holds |

## Overlap with neighbouring specs

| Spec | Its scope | Overlap with this spec |
|------|-----------|------------------------|
| `plan/immediate/spec-ike-responder-eap-preserves-state-on-unauthenticated-input.md` | `handleResponderEAP` sets `StateDead` on every `decryptAndParse` error (RFC 7296 Section 2.4, RFC7296-2.4-12) | Same function, different branch. D1b touches only the AUTH-from-MSK success branch, after `verifyRemoteAuth` passed; the decrypt-error branches stay theirs. Sequencing risk only (R-7) |
| `plan/immediate/spec-eap-tls-identity-authorization.md` | RFC 5216 Sections 5.2/5.3: Peer-Id / Server-Id authorization in `newTLSMethod`'s `VerifyConnection` and `peer_chain.go` | Same file `eap_tls.go`, disjoint requirement. D2 touches only the TLS version or session id behaviour of `newTLSMethod`, never an identity check |

## What a fix must prove

| Obligation | Detail |
|------------|--------|
| Failing test first | D1: a Child SA whose IKE SA's remote port is not 4500 installs XFRM encap with that port. D2: per the owner's ruling |
| Both polarities | D1: a peer on 4500 keeps 4500 |
| Discrimination | `./le rfc discriminate-record` for every tagged unit |
| Real entry point | D1: the installed XFRM state read back on Linux (`ai/rules/platform-linux.md`) |
| Interop | D1: strongSwan behind a NAT that maps 4500 to another port |

## Owner decisions

| Row | Question |
|-----|----------|
| D2 | Fix the crypto/tls limit (a server-chosen session_id on full handshakes), or rule that ticket-based semantics meet the obligation and record it as such |

### Owner questions (2026-10-13, design phase; only Q-3 and Q-4 are answered, by the owner on 2026-10-13, and no other row may be read as approved)

| Q | Question | Options | Recommendation and why | Answer |
|---|----------|---------|------------------------|--------|
| Q-1 | D2: what does Ze's EAP-TLS authenticator do about RFC 5216 Section 2.1.1 "the server MUST choose the sessionId to establish a new session" on a TLS 1.2 full handshake? | (a) Authenticator negotiates TLS 1.3 only (`MinVersion` 1.3); a TLS 1.2 peer is refused with a TLS alert. (b) Ze carries a patched copy of `crypto/tls` whose TLS 1.2 full handshake writes a fresh 32-octet server-chosen session_id. (c) Owner-approved deviation: Ze keeps the empty session_id, recorded with RFC 5246 Section 7.4.1.3 ("The server may return an empty session_id to indicate that the session will not be cached and therefore cannot be resumed.") and the reason; a deviation is never counted conformant, so RFC5216-2.1.1-4 leaves `wrong` for a recorded deviation, not for `enforced` | (c), stated with its cost. (a) refuses every strongSwan client left at its shipped default: `test/interop-ipsec/scenarios/responder-eap-tls13/strongswan.conf` records that charon ships `version_max = 1.2`. (b) forks a standard-library package whose TLS 1.2 server also issues a ticket ze never redeems (since `37dbe54095`), so the chosen id would identify a session ze refuses to resume. (c) is honest about what the wire carries. The ticket argument the two existing RFC5216-2.1.1-4 tags make does NOT hold any more: since `37dbe54095` a TLS 1.2 ticket is never redeemed, so "a new session can be established under an identity it chose" names an identity that is never usable | open |
| Q-2 | D1: when a Child SA needs UDP encapsulation and the IKE SA has no authenticated peer endpoint (or its port is 500), what does `newChildSA` do? | (A) Refuse the Child SA with an error naming the peer and the missing port; nothing installed. (B) Keep a fallback to port 4500 | (A). `ai/rules/principles.md`: code that cannot answer must say so. The fallback is exactly what hid D1b: the EAP path built its Child SA with no endpoint and nothing noticed. After D1b's reorder no production path reaches the refusal, so (A) costs nothing at runtime; it costs test fixtures (Q-4) | open |
| Q-3 | A NAT rebinding AFTER establishment: `adoptAuthenticatedEndpoint` adopts a new peer port (no MOBIKE, Ze not behind a NAT), but installed Child SA states keep the old encap port, so ESP again leaves on a port IKE no longer uses. In this spec, or a journal row? | (i) In scope: migrate the installed states' encap ports when the IKE SA adopts a new endpoint, as `migrateMobikePair` does for MOBIKE. (ii) Journal row in `plan/journal/`, this spec fixes the ports at Child SA creation only | Owner's call: the MUST reads as a standing condition, so (ii) leaves RFC3948-2.1-2 violated after a rebind and the verdict could only be `enforced` for the creation-time claim. (i) adds a kernel migration path with its own failure modes (R-4) | **answered by the owner, 2026-10-13: (i), "migrate it (strongSwan)"**. strongSwan migrates every Child SA through `update_hosts` / `child_sa->update`, rekeying where the update is unsupported (research 2026-10-13); AC-11 is in scope |
| Q-4 | Approval to change the RFC-tagged tests listed under "RFC-tagged tests this spec changes" (TDD Test Plan) | approve / approve a subset / refuse | Needed under any answer to Q-1 and Q-2: the RFC3948-2.1-2 tag on `TestChildSANATTEncapPorts` asserts the defect. Units: `TestChildSANATTEncapPorts` (RFC3948-2.1-2, RFC7296-2.23-2: ports differ from 4500 in the fixture, 2.23-2 claim narrowed to "UDPEncap set when NAT is detected"), `TestEncapNeverRequestedOnPort500` (RFC7296-2.23-9), the five `TestBfm*` units (RFC7296-2.23-10/-11, helper only), and the two `rfc5216_tls_config_test.go` units (RFC5216-2.1.1-4 line withdrawn under every Q-1 answer). `TestChildSANoNATNoEncap` (RFC7296-2.23-2 negative) joins the list if Phase 3 changes its body (RFC-tagged test review, 2026-10-13) | **answered by the owner, 2026-10-13: "approve all RFC-tagged test changes"**. The approval covers every unit listed under "RFC-tagged tests this spec changes", including `TestChildSANoNATNoEncap` if Phase 3 changes its body. Q-1 and Q-2 are still open: the approval covers the listed edit for whichever answer is recorded (`TestEncapNeverRequestedOnPort500` and the five `TestBfm*` units change only under Q-2 = A; under Q-1 (b) the RFC5216-2.1.1-4 lines move to the new wire test). It answers neither question. The commands are under "Owner approval commands" in the TDD plan |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/interop.md` - IPsec suite, scenario naming, discrimination walk
  → Constraint: a scenario directory is NAMED, no numeric prefix; the checker registers in `scenarioCheckers` (`internal/le/interoplab/ipsec/checkers.go`)
  → Constraint: an interop claim needs a revert, a REBUILD of the ze image, a red, then green, recorded
- [ ] `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` - "NAT traversal" section
  → Constraint: the page says the child install sets the UDP encapsulation attribute when NAT is detected; it names no port source, and must after this change
- [ ] `docs/guide/ipsec.md` - `show ipsec` SA payload and EAP-TLS resumption paragraphs
  → Constraint: "`udp-encapsulation` says the SA receives ESP inside UDP on port 4500" becomes false for a translated peer and is rewritten in the same phase
  → Constraint: the EAP-TLS paragraph states TLS 1.2 stays reachable under the 1.3 cap; Q-1 (a) would make that false

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc3948.md`, `rfc/short/rfc5216.md`
  → Constraint: the RFC3948-2.1-2 row carries `{single-polarity: positive; ... no ze code path produces a non-4500 encap port ...}`, false since MOBIKE and false after this fix; the row annotation and the Enrolment-reason prose "2.1-2 (the ESP SA uses UDP port 4500 for encapsulation)" are corrected in the D1 phase
  → Constraint: RFC scope is frozen (2026-10-08): no new summary. `rfc/full/rfc5246.txt` was fetched 2026-10-13 as RFC TEXT only, to quote Section 7.4.1.3 for Q-1; it is not enrolled

### RFC text read (verbatim, `rfc/full/`)

| RFC, section | Sentence | Bears on |
|--------------|----------|----------|
| RFC 3948 Section 2.1 | "the Source Port and Destination Port MUST be the same as that used by IKE traffic," | D1 (both ports) |
| RFC 3948 Section 5.2 | "It reaches the server as UDP <Y,4500>, where Y is the dynamically assigned port." and "To Ari: Server to NAT, <traffic desc1>, UDP encap <4500, Y>" | D1: the RFC's own example puts Y in the server's encap destination |
| RFC 7296 Section 2.23 | "they MUST be accepted coming from any port and responses MUST be sent to the port from whence they came." | D1: IKE already answers Y (`remoteUDPAddr`); ESP must match |
| RFC 7296 Section 2.23 | "UDP encapsulation MUST NOT be done on port 500." | Q-2, R-3 |
| RFC 7296 Section 2.23 | "A host behind a NAT SHOULD NOT do this type of dynamic address update if a validated packet has different port and/or address values" | Q-3 |
| RFC 4555 Section 3.3 | "When an IPsec SA is created, the tunnel header IP addresses (and port, if doing UDP encapsulation) are taken from the IKE_SA, not the IP header of the IKEv2 message requesting the IPsec SA." | D1c |
| RFC 5216 Section 2.1.1 | "If the peer's sessionId is null or unrecognized by the server, the server MUST choose the sessionId to establish a new session." | D2 |
| RFC 5246 Section 7.4.1.3 | "The server may return an empty session_id to indicate that the session will not be cached and therefore cannot be resumed." | Q-1 (c) |
| RFC 5077 Section 3.4 | "If a server is planning on issuing a session ticket to a client that does not present one, it SHOULD include an empty Session ID in the ServerHello." | Q-1: why crypto/tls leaves it empty |

**Key insights:**
- D1 is three producer sites, not one: the port default (`installChildSA`), the EAP adoption order (`handleResponderEAP`), the rekey copy (`rekey.go`); plus the local port constant (D1d)
- `remoteUDPAddr` already returns the authenticated endpoint (`sa.peerEndpoint`) that IKE answers to; the Child SA must use the same value, once adopted
- D2 cannot be fixed inside `crypto/tls`'s public API: there is no hook for the ServerHello session_id; only Q-1 decides
- AC-4 and its three ids are closed at HEAD; do not re-open them

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/engine/child.go` - Child SA build and `installChildSA`
  → Constraint: `installChildSA` takes `child.udpLocalPort`/`udpRemotePort`, replaces a zero with `transport.NATTPort`, puts (remote, local) on the inbound state when `child.UDPEncap` and (local, remote) on the outbound state when `child.NATDetected`. The send form follows NATDetected only and the receive form follows UDPEncap: that asymmetry is a measured interop fix (strongSwan floats with no NAT) and MUST be preserved
  → Constraint: `inbound.AcceptBothESPForms = true` always (RFC7296-2.23-10); the side receiver keys on the encap ports (`xfrm_linux.go` passes `UDPEncapSPort` as peerPort), so it must receive with the new port too
- [ ] `internal/component/ike/engine/create_child.go` - `newChildSA`
  → Constraint: the ONLY writer of `udpLocalPort`/`udpRemotePort` at creation; sets them only when `sa.mobike.enabled` (local from `sa.mobike.local`, remote from `sa.remoteUDPAddr()`). `UDPEncap = sa.NATDetected || sa.localPort == transport.NATTPort`
- [ ] `internal/component/ike/engine/sa.go` - `remoteUDPAddr`, `adoptAuthenticatedEndpoint`, `floatToNATTPort`, `sendPath`
  → Decision: `remoteUDPAddr` returns a copy of `sa.peerEndpoint` when set, else the CONFIGURED remote on 500, or on `nattAddr` (4500) when floated. It never returns an unauthenticated datagram source. The Child SA port source is `sa.peerEndpoint`, not `remoteUDPAddr`'s fallback
  → Constraint: `adoptAuthenticatedEndpoint` is the only writer of `peerEndpoint`; it floats the SA on a NAT-T arrival; a later different endpoint is adopted only without MOBIKE and when Ze is not behind a NAT
- [ ] `internal/component/ike/engine/responder.go` - `handleAuthRequest`, `buildAuthResponse`, `finishResponderEstablish`
  → Constraint: non-EAP path adopts BEFORE `buildAuthResponse` (deliberate); `finishResponderEstablish` adopts again (idempotent for the same endpoint). `replayCachedResponse`'s comment states `peerEndpoint` stays nil for the whole EAP exchange; that statement stays true up to the final AUTH and must be re-read when D1b moves the adoption
- [ ] `internal/component/ike/engine/responder_eap.go` - `handleResponderEAP`
  → Constraint: AUTH-from-MSK branch: `verifyRemoteAuth`, then `buildAuthResponse(..., fromEAP=true)`, then `finishResponderEstablish`. D1b inserts the adoption between the first two, mirroring `handleAuthRequest`
- [ ] `internal/component/ike/engine/fsm.go` - initiator IKE_AUTH response
  → Constraint: the initiator adopts on `StateEstablished` right after `handleAuthResponse`; `initiatorFirstChildSA` runs later from `runEstablished`, so the initiator already sees the endpoint
- [ ] `internal/component/ike/engine/rekey.go` - rekeyed `ChildSA`
  → Constraint: copies `UDPEncap`, `NATDetected`, `udpLocalPort`, `udpRemotePort` from the old child; the UDPEncap inheritance is a recorded fix and stays
- [ ] `internal/component/ike/engine/mobike.go` - `migrateMobikePair`
  → Decision: MOBIKE already re-derives ports from the IKE SA and migrates installed states (`TunnelMigrator.MigrateTunnel` with `LocalPort`/`RemotePort`); that is the model for Q-3 (i)
- [ ] `internal/component/ike/dataplane/xfrm_linux.go`, `vpp.go` - SA install
  → Constraint: both backends already carry `UDPEncapSPort`/`UDPEncapDPort` through; no dataplane change is needed for D1
- [ ] `internal/component/ike/engine/child_test.go` - `TestChildSANATTEncapPorts`
  → Constraint: tagged RFC3948-2.1-2 positive AND RFC7296-2.23-2 positive; asserts both ports equal `transport.NATTPort` with no endpoint, i.e. asserts the defect (audit note)
- [ ] `internal/core/eap/eap_tls.go` - `newTLSMethod`
  → Constraint: `MinVersion` TLS 1.2, `MaxVersion` TLS 1.3 (RFC 9190 Section 1 cap); `SetSessionTicketKeys` with the peering's keys; `UnwrapSession` is `redeemTLS13Only` or `refuseResumption`. crypto/tls issues a TLS 1.2 NewSessionTicket on a full handshake whenever the client offered the extension (`hs.hello.ticketSupported = hs.clientHello.ticketSupported && !c.config.SessionTicketsDisabled`), and `SessionTicketsDisabled` would also stop the TLS 1.3 ticket RFC 9190 Section 2.1.2 requires
- [ ] `internal/core/eap/rfc5216_tls_config_test.go` - the two RFC5216-2.1.1-4 tags
  → Constraint: `TestRFC5216TLSConfigBothRolesInstallWhatTheFlightsCarry` and `TestRFC5216TLSConfigRefusesWhatWouldBreakTheFlights` also carry RFC5216-2.1.1-5, 2.1.1-10 and 2.1.2-1 tags that are `enforced`; a D2 edit touches only the 2.1.1-4 claims and must re-record every record the edit stales
- [ ] `internal/le/interoplab/ipsec/nat.go` - `readNATConfig`, `natSetupScript`
  → Constraint: today every translation is a 1:1 address DNAT in PREROUTING plus SNAT in POSTROUTING with no port. A DNAT for the translated peer would deliver ESP sent to public:4500 to the peer's 4500 and MASK D1, so a port-translated row must carry no DNAT (R-1)

**Behavior to change:** D1, D1b, D1c, D1d; D2 per Q-1. Q-3 decides whether post-establishment rebinding is in scope.

**Behavior to preserve:** send form follows NATDetected only; receive form follows UDPEncap; `AcceptBothESPForms` on every inbound state; MOBIKE port derivation and migration; no encapsulation on port 500; the EAP decrypt-error branches (other spec); RFC 9190 TLS 1.3 tickets and `redeemTLS13Only`.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- D1: an authenticated IKE_AUTH (responder: request; initiator: response) arriving on Ze's NAT-T socket from a translated source (A, Y); a CREATE_CHILD_SA or Child SA rekey on that IKE SA
- D2: an EAP-TLS ClientHello at TLS 1.2 reaching Ze's authenticator (`tlsMethod`) through IKE_AUTH EAP rounds

### Transformation Path
1. UDP datagram → `transport.UDPTransport` (NAT-T socket) → `Packet{RemoteAddr, NATT}`
2. decrypt and AUTH verification (`verifyRemoteAuth`) → `adoptAuthenticatedEndpoint` writes `sa.peerEndpoint` = (A, Y) and floats `sa.localPort`
3. `newChildSA` copies the IKE SA's ports onto the `ChildSA` (today: MOBIKE only) → `installChildSA` → `dataplane.SAParams{UDPEncapSPort, UDPEncapDPort}`
4. `xfrmBackend.InstallSA` → netlink `XFRM_MSG_NEWSA` with `XFRMA_ENCAP` (type espinudp, sport, dport); VPP: `UDPSrcPort`/`UDPDstPort`
5. D2: `tls.Server` over the EAP fragmenter → ServerHello bytes → EAP-Request

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| IKE engine ↔ kernel XFRM | netlink XFRM_MSG_NEWSA, `XFRMA_ENCAP` | No: Linux integration test reads the state back (AC-1) |
| IKE engine ↔ VPP | binary API SA add | No VPP lab; unit assertion on `SAParams` only (R-6) |
| ze ↔ strongSwan across a port-translating NAT | UDP 4500 ↔ public:Y | No: new interop scenarios (AC-1, AC-2) |
| EAP-TLS authenticator ↔ crypto/tls | `tls.Config` | Read only; Q-1 |

### Integration Points
- `sa.peerEndpoint` (authenticated endpoint), `sa.nattSocket`'s local address, `sa.mobike` (unchanged), `ChildSA.udpLocalPort`/`udpRemotePort`
- `internal/le/interoplab/ipsec/nat.go` (port-translating row), `scenarioCheckers`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | ports flow IKE SA → `newChildSA` → `installChildSA` → dataplane, the same path MOBIKE already uses |
| No unintended coupling (components stay isolated) | Yes | engine-internal; dataplane API unchanged |
| No duplicated functionality (extends existing, does not recreate) | Yes | extends the MOBIKE port derivation to every IKE SA instead of adding a second path |
| Zero-copy preserved where applicable (refs, not copies) | N-A | control plane, once per Child SA |
| Registration over hardcoding, outbound | N-A | no new command, family or handler |
| Registration over hardcoding, inbound | Yes | no list learns a name; the only enumeration touched is `scenarioCheckers`, which is the IPsec suite's registry by design |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| responder IKE_AUTH (PSK) from (A, 14500) on the NAT-T socket | → | `handleAuthRequest` → `newChildSA` → `installChildSA` | `TestRFC3948ResponderChildSAEncapsulatesToTheIKEPeerPort` (`internal/component/ike/engine/rfc3948_encap_port_test.go`) |
| responder final IKE_AUTH (AUTH-from-MSK) from (A, 14500) | → | `handleResponderEAP` → adoption → `buildAuthResponse` | `TestRFC3948EAPResponderChildSAEncapsulatesToTheIKEPeerPort` (same file) |
| initiator IKE_AUTH response from (B, 14501) | → | `fsm.go` adoption → `initiatorFirstChildSA` | `TestRFC3948InitiatorChildSAEncapsulatesToTheIKEPeerPort` (same file) |
| real Linux XFRM | → | `installChildSA` → `xfrmBackend.InstallSA` | `TestRFC3948InstalledStateCarriesTheIKEPeerPort` (`internal/component/ike/engine/rfc3948_encap_port_integration_linux_test.go`, `integration && linux`) |
| strongSwan behind a port-translating NAT | → | whole stack | interop `real-nat-port-translated-ze-responder`, `real-nat-port-translated-eap-ze-responder` |
| TLS 1.2 ClientHello to the authenticator | → | `newTLSMethod` per Q-1 | `TestRFC5216TLS12FullHandshakeServerHelloSessionID` (`internal/core/eap/rfc5216_session_id_test.go`) and interop `responder-eap-tls12` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | peer's IKE arrives from port Y behind a NAT | ESP encapsulated to port Y |
| AC-1 (design, 2026-10-13) | Ze responder (PSK), NAT detected, the authenticated IKE_AUTH arrives on Ze's NAT-T socket bound to port L from (A, Y), Y not 4500 | inbound XFRM state: encap espinudp sport Y dport L; outbound state: sport L dport Y. Proof: `TestRFC3948ResponderChildSAEncapsulatesToTheIKEPeerPort` (unit, the installed `SAParams`), `TestRFC3948InstalledStateCarriesTheIKEPeerPort` (Linux XFRM read back), interop `real-nat-port-translated-ze-responder` (traffic both ways through the tunnel, ze's state shows dport Y) |
| AC-2 | D2 | as the owner rules |
| AC-2 (design, 2026-10-13) | TLS 1.2 ClientHello with an empty session_id reaches Ze's authenticator | per Q-1. (a): handshake refused with a TLS protocol_version alert, TLS 1.3 unaffected; proof: unit reads the alert, interop `responder-eap-tls12` sees charon refused. (b): ServerHello carries a 32-octet session_id that differs from the client's; proof: unit reads the ServerHello bytes. (c): ServerHello carries an empty session_id, and the deviation is recorded in `rfc/short/rfc5216.md` with RFC 5216 Section 2.1.1 and RFC 5246 Section 7.4.1.3 quoted and the owner's ruling named; proof: unit reads the bytes and asserts the documented deviation, interop `responder-eap-tls12` authenticates a TLS 1.2 strongSwan client |
| AC-3 | the `wrong` verdicts of RFC3948-2.1-2 and RFC5216-2.1.1-4, after this spec's producer fix | each verdict reaches `enforced`: a tagged test proves the quoted sentence, and an agent that did not write that test re-judges it with `./le rfc audit-stamp ... mode rejudge`. Moved here from "Blocked by" in `plan/pre-release/spec-rfc-verdict-fix-ike-eap.md` (parent P-3, 2026-09-28) |
| AC-4 | D2 area, RFC5216-2.1.2-1 (weak), RFC 5216 Section 2.1.2: "If the EAP server is resuming a previously established session, then it MUST include only a TLS change_cipher_spec message and a TLS finished handshake message after the server_hello message." crypto/tls renews the session ticket on a TLS 1.2 resumption, so a NewSessionTicket follows server_hello | owner ruling 2026-09-29: ze's authenticator refuses TLS 1.2 resumption (a TLS 1.2 session is never redeemed; TLS 1.3 tickets stay, per RFC 9190). The untagged failing test `TestRFC5216ResumingServerSendsNothingButChangeCipherSpecAndFinished` (`internal/core/eap/rfc5216_resumption_defect_test.go`, uncommitted) is replaced by a tagged proof of the fix, and the verdict reaches `enforced` by an agent that did not write that test. The fix removes the TLS 1.2 resumed conversation that `TestRFC5216ResumedFlightsCarryOnlyChangeCipherSpecAndFinished` drives (RFC5216-2.1.1-10, 2.1.1-5), so those tags are re-proven or re-judged in the same work. Moved here from "Blocked by" in `plan/pre-release/spec-rfc-verdict-fix-ike-eap.md` (P-3) |

| AC-4 status (2026-10-13) | - | CLOSED at HEAD: producer `37dbe54095`, independent re-judge to `enforced` in `c1f83ffa35` and `96535408fa` for RFC5216-2.1.2-1, 2.1.1-5 and 2.1.1-10. Nothing further owed here |
| AC-5 | Ze responder or initiator, NAT detected, peer's authenticated IKE endpoint is (A, 4500) | both states keep encap 4500/4500 (no-translation polarity). Proof: `TestRFC3948PeerOn4500KeepsPort4500` (unit) and the existing scenarios `real-nat-tunnel-control`, `real-nat-transport-ze-responder`, `real-nat-transport-ze-initiator` staying green |
| AC-6 | EAP responder: final IKE_AUTH (AUTH-from-MSK) from (A, Y) | Child SA installed with sport/dport Y as AC-1; `sa.peerEndpoint` is set before `buildAuthResponse` runs. Proof: `TestRFC3948EAPResponderChildSAEncapsulatesToTheIKEPeerPort`, interop `real-nat-port-translated-eap-ze-responder` |
| AC-7 | Ze initiator, authenticated IKE_AUTH response arrives from (B, Y) | first Child SA installed with dport Y (outbound) / sport Y (inbound). Proof: `TestRFC3948InitiatorChildSAEncapsulatesToTheIKEPeerPort` |
| AC-8 | a CREATE_CHILD_SA Child SA and a rekeyed Child SA on an IKE SA whose endpoint is (A, Y) | both installed with Y, taken from the IKE SA (not copied from the retired child). Proof: `TestRFC3948CreatedAndRekeyedChildSATakeTheIKESAPorts` |
| AC-9 | Ze's NAT-T socket bound to a port L other than 4500 (test-port hook) | local encap port is L on both states. Proof: `TestRFC3948LocalEncapPortIsTheNATTSocketPort` |
| AC-10 | (Q-2 = A) a Child SA that needs UDP encapsulation and whose IKE SA has no authenticated endpoint, or whose endpoint port is 500 | Child SA refused with an error naming the peer and the reason; no SA or policy installed; the IKE_AUTH answer is the childless response `buildChildlessAuthResponse` already gives a refused install. Proof: `TestRFC3948ChildSARefusesEncapWithoutAnIKEPort` (negative polarity of RFC3948-2.1-2 and RFC7296-2.23-9). (Q-2 = B): this row is replaced by a test that the fallback is 4500 and never 500 |
| AC-11 | (Q-3 = i) IKE SA adopts a new peer endpoint (A, Y2) after establishment, no MOBIKE, Ze not behind a NAT | every installed Child SA's states are migrated to Y2. Proof: unit over `TunnelMigrator`, interop step in `real-nat-port-translated-ze-responder` that flushes the NAT's conntrack and remaps. (Q-3 = ii): this row is replaced by a journal row and the RFC3948-2.1-2 claim states creation time only |
| AC-12 | docs and the summary | `docs/guide/ipsec.md` (`udp-encapsulation` sentence), `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` ("NAT traversal": ports come from the IKE SA), `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` (child build), `rfc/short/rfc3948.md` (RFC3948-2.1-2 annotation and Enrolment-reason prose) state the new behaviour in the phase that changes it; D2 per Q-1 in `rfc/short/rfc5216.md` and `docs/guide/ipsec.md` |

## End-to-End User Stories

| Story | Path | Test |
|-------|------|------|
| An operator terminates a road-warrior strongSwan client behind a home NAT that rewrites UDP 4500 to a high port; traffic flows both ways | strongSwan → NAT (4500 → 14500) → ze responder → XFRM | `real-nat-port-translated-ze-responder` |
| Same, the client authenticates with EAP-TLS | as above, EAP rounds | `real-nat-port-translated-eap-ze-responder` |
| A TLS 1.2-only EAP-TLS client authenticates (or is refused, per Q-1) | strongSwan client `version_max = 1.2` → ze authenticator | `responder-eap-tls12` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| encap ports from the IKE SA | `internal/component/ike/engine/` | D1 | [to fill in design] |
| `TestRFC3948ResponderChildSAEncapsulatesToTheIKEPeerPort` | `internal/component/ike/engine/rfc3948_encap_port_test.go` (new) | AC-1; tag RFC3948-2.1-2 positive | to write, must fail at HEAD (ports 4500/4500) |
| `TestRFC3948EAPResponderChildSAEncapsulatesToTheIKEPeerPort` | same | AC-6; tag RFC3948-2.1-2 positive | to write, must fail at HEAD |
| `TestRFC3948InitiatorChildSAEncapsulatesToTheIKEPeerPort` | same | AC-7 | to write, must fail at HEAD |
| `TestRFC3948CreatedAndRekeyedChildSATakeTheIKESAPorts` | same | AC-8 | to write, must fail at HEAD |
| `TestRFC3948PeerOn4500KeepsPort4500` | same | AC-5 | to write; green at HEAD by design (polarity guard) |
| `TestRFC3948LocalEncapPortIsTheNATTSocketPort` | same | AC-9 | to write, must fail at HEAD |
| `TestRFC3948ChildSARefusesEncapWithoutAnIKEPort` | same | AC-10 (Q-2 = A); tag RFC3948-2.1-2 negative | to write, must fail at HEAD |
| `TestRFC5216TLS12FullHandshakeServerHelloSessionID` | `internal/core/eap/rfc5216_session_id_test.go` (new) | AC-2 per Q-1; reads the ServerHello session_id octets of a TLS 1.2 full handshake driven through `tlsMethod` | to write after Q-1 |

### Integration Tests (Linux, real XFRM)
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC3948InstalledStateCarriesTheIKEPeerPort` | `internal/component/ike/engine/rfc3948_encap_port_integration_linux_test.go` (new, `integration && linux`, pattern of `child_policy_delete_integration_linux_test.go`: `dataplane.Load("xfrm")`) | AC-1: the kernel state's `XFRMA_ENCAP` sport/dport read back over netlink equal (Y, L) inbound and (L, Y) outbound | to write |

### RFC-tagged tests this spec changes (each needs the owner's approval, Q-4)

| Test | Tags it carries | Change | Needed under |
|------|-----------------|--------|--------------|
| `internal/component/ike/engine/child_test.go::TestChildSANATTEncapPorts` | RFC3948-2.1-2 positive, RFC7296-2.23-2 positive | fixture gains an authenticated peer endpoint (A, Y) and a NAT-T socket port; assertions move from the constant 4500 to the IKE SA's ports. The peer port Y and the local NAT-T socket port MUST both differ from 4500 (D1d), or the assertion cannot tell the fix from the constant. The tag claims are rewritten (they cite stale `child.go` line numbers and the false "4500 unconditionally" statement): the RFC7296-2.23-2 claim becomes "UDPEncap is set when NAT is detected", because Section 2.23's "However, if a NAT is detected, both devices MUST use UDP encapsulation for ESP." says nothing about port 4500; the RFC3948-2.1-2 claim ("the Source Port and Destination Port MUST be the same as that used by IKE traffic,") is scoped to creation time, because Q-3 is answered (i) and the after-rebind half is carried by the AC-11 migration test, which also takes the RFC3948-2.1-2 tag | every answer |
| `internal/component/ike/engine/rfc7296_natt_test.go::TestEncapNeverRequestedOnPort500` | RFC7296-2.23-9 positive | fixture gains an authenticated endpoint on 4500; comment "The only two writers assign transport.NATTPort" and "PREVENTS: a later change deriving the encapsulation ports from the SA's own port field" become false and are rewritten; the port-500 refusal moves to the new negative test | Q-2 = A (under B only the comment changes) |
| `internal/component/ike/engine/rfc7296_natt_bothforms_test.go`: `TestBfmEncapsulatedESPAcceptedWithoutNAT`, `TestBfmEncapsulatedESPSentWhenNATDetected`, `TestBfmBareESPKeptForUnfloatedSA`, `TestBfmBothESPFormsAreReachable`, `TestBfmBothESPFormsReceivedOnOneChildSA` | RFC7296-2.23-10 positive/negative, RFC7296-2.23-11 positive/negative | shared helper `bfmInstall` gains an authenticated endpoint for every floated case; assertions unchanged | Q-2 = A |
| `internal/core/eap/rfc5216_tls_config_test.go::TestRFC5216TLSConfigBothRolesInstallWhatTheFlightsCarry` | RFC5216-2.1.1-4 positive (plus enforced 2.1.1-5, 2.1.1-10, 2.1.2-1 tags, not touched) | the RFC5216-2.1.1-4 tag line is WITHDRAWN under every answer to Q-1; "withdrawn or rewritten" is not a choice. Session ticket keys are not a sessionId, so the tag is wrong at HEAD (journal row in `plan/journal/reference-checked-claim-unchecked.md`). Under Q-1 (a) TLS 1.2 is refused and no config-level tag can prove a TLS 1.2 sessionId; under (c) a deviation is never conformant; under (b) the 2.1.1-4 positive moves to a new ServerHello wire test that reads the server-chosen 32-octet session_id off a TLS 1.2 full handshake | every answer to Q-1 |
| `internal/core/eap/rfc5216_tls_config_test.go::TestRFC5216TLSConfigRefusesWhatWouldBreakTheFlights` | RFC5216-2.1.1-4 negative (plus the enforced tags above) | the RFC5216-2.1.1-4 tag line is withdrawn under every answer, for the same reason; under Q-1 (b) the negative moves to the same new wire test | every answer to Q-1 |
| `internal/component/ike/engine/rfc7296_test.go::TestChildSANoNATNoEncap` (UNCHANGED but affected) | RFC7296-2.23-2 negative | no edit planned, but the implementer MUST decide its fate on purpose: it asserts ports == 0 without NAT, so if `newChildSA` fills the ports for every SA (Phase 3) and `installChildSA` no longer zeroes them when UDPEncap is false, it goes red. Its claim "ESP traffic does NOT float" is stronger than Section 2.23 ("Either side can decide whether or not to use UDP encapsulation for ESP irrespective of the choice made by the other side."); if its body changes, the prose presents no-encap-without-NAT as Ze's choice, not the RFC's, and the edit joins this Q-4 list | Q-2 = A and Phase 3's port fill |

Untagged tests that change with Q-2 = A: `TestRekeyKeepsBothESPFormAcceptance` (`rfc7296_rekey_collision_test.go`), and any `mobike_test.go` fixture that installs a Child SA without an endpoint (none found that assert ports beyond MOBIKE's own). Every changed tagged unit is re-recorded with `./le rfc discriminate-record`; records for RFC3948-2.1-2, RFC7296-2.23-2/-9/-10/-11 and RFC5216-2.1.1-4 go stale and their verdicts are re-judged by an agent that did not write the tests (`./le rfc audit-stamp ... mode rejudge`).

#### Owner approval commands

Approved by the owner on 2026-10-13 (Q-4). The gate reads approvals from the committing session's own file (`tmp/commit-rfc-approved-<session>.md`), and a row is used up by the commit that carries its `RFC-approved:` trailer. So the owner runs these at the prompt of the session that implements and commits the edits, and runs a unit's command again if a later commit changes that unit a second time. A unit is `<package directory>.<top-level test function>`; every name was checked to exist at HEAD `051876c3f4`. The gate compares each tagged function's own text: if only `bfmInstall` changes and a `TestBfm*` body stays byte-identical, that unit is not asked for and its row goes unused. The last command applies only if Phase 3 changes `TestChildSANoNATNoEncap`. The new AC-11 migration test and the new negative test are new units and need no approval.

10 commands:

```
./le rfc approve unit engine.TestChildSANATTEncapPorts reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-ike-eap-rfc-defects, fixture gains authenticated endpoint and NAT-T port, ports from the IKE SA, tag claims rewritten)'
./le rfc approve unit engine.TestEncapNeverRequestedOnPort500 reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-ike-eap-rfc-defects, fixture gains authenticated endpoint on 4500, comments rewritten)'
./le rfc approve unit engine.TestBfmEncapsulatedESPAcceptedWithoutNAT reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-ike-eap-rfc-defects, bfmInstall gains an authenticated endpoint)'
./le rfc approve unit engine.TestBfmEncapsulatedESPSentWhenNATDetected reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-ike-eap-rfc-defects, bfmInstall gains an authenticated endpoint)'
./le rfc approve unit engine.TestBfmBareESPKeptForUnfloatedSA reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-ike-eap-rfc-defects, bfmInstall gains an authenticated endpoint)'
./le rfc approve unit engine.TestBfmBothESPFormsAreReachable reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-ike-eap-rfc-defects, bfmInstall gains an authenticated endpoint)'
./le rfc approve unit engine.TestBfmBothESPFormsReceivedOnOneChildSA reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-ike-eap-rfc-defects, bfmInstall gains an authenticated endpoint)'
./le rfc approve unit eap.TestRFC5216TLSConfigBothRolesInstallWhatTheFlightsCarry reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-ike-eap-rfc-defects, RFC5216-2.1.1-4 positive tag line withdrawn)'
./le rfc approve unit eap.TestRFC5216TLSConfigRefusesWhatWouldBreakTheFlights reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-ike-eap-rfc-defects, RFC5216-2.1.1-4 negative tag line withdrawn)'
./le rfc approve unit engine.TestChildSANoNATNoEncap reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-ike-eap-rfc-defects, only if Phase 3 changes its body, prose states no-encap-without-NAT as Ze choice)'
```

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| peer encap port (from the authenticated endpoint) | 1-65535, excluding 500 (RFC 7296 Section 2.23) | 65535 and 501 | 0 (unknown) and 500 (refused, AC-10) | N-A (uint16) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A | - | the `.ci` harness has no port-translating NAT between two daemons; the user path is proven by the interop scenarios and the Linux integration test | - |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `real-nat-port-translated-ze-responder` (new) | `test/interop-ipsec/scenarios/real-nat-port-translated-ze-responder/` | strongSwan 5.9.14 initiator (PSK, tunnel) behind the NAT box; nat.conf row maps its UDP 4500 to public port 14500 with SNAT only, no DNAT | AC-1, AC-5 (other scenarios), AC-11 if Q-3 = i. Checker: ze's outbound state reads `encap type espinudp sport 4500 dport 14500`; ping across the tunnel both ways; strongSwan's ESP inbound byte counter rises. Red at HEAD expected: ESP to public:4500 has no conntrack entry and is dropped | to create; revert-rebuild-red walk owed |
| `real-nat-port-translated-eap-ze-responder` (new) | `test/interop-ipsec/scenarios/real-nat-port-translated-eap-ze-responder/` | strongSwan 5.9.14, EAP-TLS 1.3 client (as `responder-eap-tls13`), same NAT | AC-6 | to create; walk owed |
| `responder-eap-tls12` (new) | `test/interop-ipsec/scenarios/responder-eap-tls12/` | strongSwan 5.9.14 EAP-TLS client at its shipped `version_max = 1.2` | AC-2 per Q-1: (c) authenticates at TLS 1.2; (a) charon logs the refusal. charon is not known to log the session_id (A-5), so the session_id itself is proven by the unit test | to create after Q-1 |
| `real-nat-tunnel-control`, `real-nat-transport-ze-responder`, `real-nat-transport-ze-initiator` (existing) | `test/interop-ipsec/scenarios/` | strongSwan 5.9.14 | AC-5: unchanged 4500 behaviour stays green | existing |

The nat.conf format gains an optional third column (public port for the peer's UDP 4500) read by `readNATConfig`; `natSetupScript` emits, for such a row, a port SNAT for UDP source port 4500 before the address SNAT and NO DNAT. `docs/architecture/testing/interop.md` documents the column.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|------------|---------------------------------|----------|--------------|--------|
| A-1 | After D1b, every production Child SA build runs after `adoptAuthenticatedEndpoint` | read: `handleAuthRequest` adopts before `buildAuthResponse`; `fsm.go` adopts on StateEstablished before `runEstablished` calls `initiatorFirstChildSA`; CREATE_CHILD_SA and rekey run on an established SA | Q-2 (A) refuses a Child SA in production | the three wiring unit tests plus interop | unvalidated |
| A-2 | The NAT-T socket's bound port is the port the kernel's ESP-in-UDP decap socket listens on | `transport/encap_linux.go` sets UDP_ENCAP on the NAT-T socket (read of the file owed at implementation) | D1d picks a port the kernel does not decap on | `TestRFC3948InstalledStateCarriesTheIKEPeerPort` plus a read of `encap_linux.go` | unvalidated |
| A-3 | go1.27.2 crypto/tls leaves the ServerHello session_id empty on every TLS 1.2 full handshake | grep of `handshake_server.go`: the only assignment is in `doResumeHandshake` | D2 is not a defect | `TestRFC5216TLS12FullHandshakeServerHelloSessionID` red at HEAD | unvalidated |
| A-4 | strongSwan 5.9.14 accepts ESP-in-UDP from ze's 4500 arriving through the NAT's conntrack reverse mapping | normal NAT-T operation; existing real-nat scenarios pass with an address NAT | the scenario fails for a lab reason | the scenario's green run | unvalidated |
| A-5 | charon does not log the TLS session_id it receives | not verified in strongSwan source | a stronger interop assertion is available for D2 | read `tls_peer.c` `process_server_hello` at implementation | unvalidated |
| A-6 | iptables `SNAT --to-source addr:port` for `-p udp --sport 4500` translates the port deterministically in the alpine NAT box | iptables semantics | the scenario cannot force port Y | the checker reads the port ze observed (ze log of the adopted endpoint) before asserting | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|-----------------------|
| R-1 | The NAT box's DNAT forwards ESP sent to public:4500 to the peer's 4500, masking D1 so the scenario is green at HEAD | the revert-rebuild walk stays green | port-translated rows carry no DNAT; the walk's red is recorded before the claim |
| R-2 | Q-2 (A) refuses a Child SA on a path that never adopted an endpoint | childless IKE SA, the refusal error in the log, an interop scenario red | A-1's enumeration; the error names the peer and the missing port |
| R-3 | A NAT maps the peer to source port 500 on Ze's NAT-T socket: RFC 3948 asks for 500, RFC 7296 forbids encapsulation on 500 | the refusal log | refuse the Child SA (AC-10) and say so; the conflict is stated in the summary row |
| R-4 | NAT rebinding after establishment leaves installed states on the old port (Q-3) | ESP counters stop while IKE DPD succeeds | Q-3 decides; (i) uses the MOBIKE migration path |
| R-5 | The `AcceptBothESPForms` side receiver keys on the encap ports; with port Y it might not match | bare-ESP arm of the both-forms tests red | `TestRFC3948InstalledStateCarriesTheIKEPeerPort` also sends bare ESP; existing `encap_dualform_integration_linux_test.go` re-run |
| R-6 | The VPP backend carries ports with no lab to prove them | none (no VPP lab) | unit assertion on `SAParams`; stated as the unproven part in the summary |
| R-7 | `spec-ike-responder-eap-preserves-state-on-unauthenticated-input` edits `handleResponderEAP` concurrently | merge conflict in `responder_eap.go` | D1b touches only the AUTH-from-MSK branch; whichever lands second rebases its hunk |
| R-8 | Q-1 (a) refuses every default-configured strongSwan EAP-TLS client (`version_max = 1.2`) | `responder-eap-tls12` red, operator reports | stated in Q-1; the guide's TLS 1.2 sentence changes with it |
| R-9 | Changing the tagged fixtures stales discrimination records and enforced verdicts of RFC7296-2.23-2/-9/-10/-11 | `./le rfc check` refusals | re-record every staled unit in the same change; independent re-judge |
| R-10 | Deriving the local port from the socket changes `.ci` IPsec tests that run under the test-port hook | `test/ipsec` red | the change makes the encap port equal the IKE port those tests already use; investigate each red under `ai/rules/pre-release.md` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | NAT-traversing tunnels carry no ESP (IKE stays up); with Q-2 (A) a mis-ordered path yields a childless IKE SA |
| How is it reverted? | single commit revert per phase |
| Who else touches this path? | `spec-ike-responder-eap-preserves-state-on-unauthenticated-input` (`responder_eap.go`), `spec-eap-tls-identity-authorization` (`eap_tls.go`), MOBIKE code (`mobike.go`, untouched) |

## Files to Modify

- the two producers and their tests
- `internal/component/ike/engine/create_child.go` - `newChildSA`: ports from the IKE SA for every Child SA (remote from `sa.peerEndpoint`, local from the NAT-T socket), MOBIKE branch unchanged; RFC 3948 Section 2.1 quote above the derivation
- `internal/component/ike/engine/child.go` - `installChildSA`: drop the 4500 default; refusal per Q-2; rewrite the `UDPEncap` and `udpLocalPort` field comments and the port comments that state "Both ports below are the NAT-T constant"
- `internal/component/ike/engine/responder_eap.go` - `handleResponderEAP`: adopt the endpoint after `verifyRemoteAuth`, before `buildAuthResponse`
- `internal/component/ike/engine/responder.go` - `replayCachedResponse` comment on `peerEndpoint` during EAP
- `internal/component/ike/engine/rekey.go` - rekeyed `ChildSA` takes the IKE SA's ports
- `internal/component/ike/engine/mobike.go` - only if Q-3 = i (share the migration)
- `internal/core/eap/eap_tls.go` - per Q-1
- tagged and untagged tests listed in the TDD plan
- `internal/le/interoplab/ipsec/nat.go`, `internal/le/interoplab/ipsec/checkers.go` - port column, three checkers
- `rfc/short/rfc3948.md`, `rfc/short/rfc5216.md` (per Q-1), `rfc/discrimination/rfc3948.json`, `rfc7296.json`, `rfc5216.json`, `rfc/audit/*` via `./le rfc audit-stamp`
- `docs/guide/ipsec.md`, `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md`, `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`, `docs/architecture/ike/ipsec-14-responder.md` (EAP adoption order), `docs/architecture/testing/interop.md` (nat.conf port column)

## Files to Create

- `internal/component/ike/engine/rfc3948_encap_port_test.go`
- `internal/component/ike/engine/rfc3948_encap_port_integration_linux_test.go`
- `internal/core/eap/rfc5216_session_id_test.go`
- `test/interop-ipsec/scenarios/real-nat-port-translated-ze-responder/` (`nat.conf`, `swanctl.conf`, `ze.conf`)
- `test/interop-ipsec/scenarios/real-nat-port-translated-eap-ze-responder/` (plus `pki/`, `strongswan.conf`)
- `test/interop-ipsec/scenarios/responder-eap-tls12/`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no config change |
| YANG validation constraints | N-A | no leaf |
| YANG custom validators | N-A | no leaf |
| CLI commands/flags | No | `show ipsec` output keys unchanged |
| CLI grammar (keyword before value) | N-A | no command |
| Editor autocomplete | N-A | no leaf |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | no command |
| Env var registration | No | the test-port hook already exists |
| Doctor check for runtime dependencies | No | no new path, port, module or binary: UDP 4500 and XFRM are already checked |
| Prometheus counters/metrics | No | none added |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | the IPsec NAT-T feature evidence gains the port-translated scenarios: the `features/` entry that cites `real-nat-*` (located with `./le feature report` at implementation) |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/guide/ipsec.md` |
| 7 | Wire format changed? | No | the UDP header layout is unchanged; only the port values |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc3948.md`, `rfc/short/rfc5216.md`; `docs/features/rfc-status.md` regenerated if a Support row changes |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` (nat.conf port column) |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md`, `ipsec-8-ikev2-child-xfrm.md`, `ipsec-14-responder.md` |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED at implementation with `./le spec citation anchors spec plan/immediate/spec-ike-eap-rfc-defects.md`; the `CODE-TO-DOCS` rows for `child.go`, `create_child.go` and `eap_tls.go` name `ipsec-13-rekey-wire.md`, `ipsec-14-responder.md`, `ipsec-8-ikev2-child-xfrm.md`, `ipsec-dataplane-inspection.md`, `ipsec-11-interop-eap.md`, `ipsec-9-ikev2-eap-nat.md`, `docs/guide/ipsec.md`. `docs/architecture/ike/ipsec-7-ikev2-engine.md` is declared by `sa.go`'s `// Design:` header: it is re-read if `sa.go` gains a NAT-T socket port accessor (D1d); at design time it states nothing about encap ports, so it is named as likely unaffected, with that reason. Advisory anchors the command listed on 2026-10-13, each re-read in the phase that edits the mentioning file: `docs/architecture/ike/ipsec-11-interop-eap.md` (`responder_eap.go`, `eap_tls.go`), `docs/architecture/ike/ipsec-13-rekey-wire.md` (`child.go`, `responder.go`, `rekey.go`, `mobike.go`: rekeyed Child SA ports, D1c, likely affected), `docs/architecture/ike/ipsec-3-data-model.md` and `docs/architecture/ike/ipsec-6-ikev2-crypto.md` (`responder.go`, unaffected: data model and crypto), `docs/architecture/ike/ipsec-dataplane-inspection.md` (`child.go`, affected if it states the encap port), `docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md` (`rekey.go`, historical pilot, unaffected) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/ipsec.md` `show ipsec` payload text; verify against `saToMap` |

### Discovery (`ai/rules/repo-maintenance.md`)
| Question | Answer |
|----------|--------|
| Where does an agent look first? | `ai/INDEX.md` IPsec rows → `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` "NAT traversal" |
| What prevents regression? | RFC3948-2.1-2 positive and negative tags with discrimination records; the port-translated interop scenarios |
| What registry prevents drift? | `scenarioCheckers` (IPsec suite); `rfc/discrimination/rfc3948.json` |
| What verification proves it? | `./le rfc check`, the Linux integration test, the interop scenarios |

## Implementation Steps

1. Owner ruling on D2; 2. failing tests; 3. fixes; 4. discrimination and verdicts.

Design-phase steps (2026-10-13), each phase TDD with a self-review before the next:

1. **Phase: Wiring** -- write the three wiring unit tests and the Linux integration test; confirm each is red at HEAD for the stated reason (4500 where Y is expected)
2. **Phase: D1b** -- move the adoption in `handleResponderEAP`; EAP wiring test still red (ports), adoption assertion green
3. **Phase: D1, D1c, D1d** -- `newChildSA` ports for every SA, `installChildSA` default removed, rekey from the IKE SA, local port from the NAT-T socket; Q-2's refusal; the approved tagged-test changes (Q-4); all unit and integration tests green
4. **Phase: Q-3** -- only if the owner chose (i)
5. **Phase: interop** -- nat.conf port column, three scenarios, revert-rebuild-red walk recorded for each
6. **Phase: D2** -- per Q-1
7. **Phase: RFC records** -- `./le rfc discriminate-record` for every new or changed tagged unit; summary rows; independent `./le rfc audit-stamp ... mode rejudge` for RFC3948-2.1-2, RFC5216-2.1.1-4 and every staled verdict
8. **Phase: docs** -- each page edited in the phase that changes its behaviour; this step only verifies

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N maps to a file and test |
| Correctness | send form still follows NATDetected only; port 500 never requested; MOBIKE untouched |
| Data flow | ports come only from the IKE SA's authenticated endpoint and its NAT-T socket |
| Rule: stale-comments | the `UDPEncap`, `udpLocalPort` and port comments in `child.go` and `responder.go` restated |
| Rule: rfc-compliance | RFC 3948 Section 2.1 quote above the derivation; callers carry `// RFC 3948 Section 2.1` |

### Security Review Checklist
| Check | What to look for |
|-------|------------------|
| Input validation | the port comes from an AUTHENTICATED datagram only; an unauthenticated source never reaches `peerEndpoint` (`TestNattUnauthenticatedPacketDoesNotMoveTheEndpoint` stays green) |
| Amplification | moving the EAP adoption earlier must not change `replayCachedResponse`'s destination rule |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Take Child SA encap ports from the IKE SA (authenticated endpoint, NAT-T socket) for every Child SA | (B) keep 4500 fallback; (C) let the kernel learn the port from `XFRM_MSG_MAPPING` events | (C) needs the peer to send ESP first, has no VPP equivalent and adds a netlink listener; (B) is the silent default that hid D1b. Recommended (A) extends the MOBIKE derivation already in `newChildSA` |
| Fix the EAP adoption order rather than read the datagram source at Child SA build | pass `remote` into `buildAuthResponse` | adoption is the single writer of the authenticated endpoint; a second source would be a second declaration |
| D2 left to Q-1 | - | no public crypto/tls hook exists; every option changes behaviour or records a deviation, which is the owner's |

## Known Limitations
- VPP: ports reach `SAParams` but no lab proves the VPP SA (R-6)
- Post-establishment NAT rebinding is out of scope unless Q-3 = i

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (before the fix)
- [ ] Tests PASS (after the fix)

### Verification
- [ ] `./le verify worktree`
