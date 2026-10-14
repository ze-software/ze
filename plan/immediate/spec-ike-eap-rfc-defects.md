# Spec: ike-eap-rfc-defects

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-14 |

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

## Status at HEAD (re-checked 2026-10-14, HEAD `efd8a5bbad`)

This is the only HEAD the spec cites: every verdict, test name and producer named in the spec was
re-checked at it. Every row below was read in `rfc/audit/rfc3948.json` / `rfc/audit/rfc5216.json` at
HEAD, and at the producer. `./le rfc check` reported no rfc3948 or rfc5216 refusal when last run
(2026-10-13); it is re-run at implementation.

| Requirement | Verdict at HEAD | Producer state | Owned here? |
|-------------|-----------------|----------------|-------------|
| RFC3948-2.1-2 | `wrong` | open: `installChildSA` (`child.go`) still defaults both encap ports to `transport.NATTPort` unless MOBIKE set `udpLocalPort`/`udpRemotePort` | yes, D1 |
| RFC5216-2.1.1-4 | `wrong` (re-judged 2026-10-12, note: "None of it bears on a sessionId") | open: go1.27.2 `crypto/tls` assigns `hs.hello.sessionId` only in `serverHandshakeState.doResumeHandshake` (`handshake_server.go`), echoing the client; `doFullHandshake` leaves it empty. Owner ruling 2026-10-14: Q-1 (b), "D2 design" | yes, D2 |
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
| Failing test first | D1: a Child SA whose IKE SA's remote port is not 4500 installs XFRM encap with that port. D2: a TLS 1.2 full handshake through `tlsMethod` carries a 32-octet server-chosen session_id (red at HEAD: length 0) |
| Both polarities | D1: a peer on 4500 keeps 4500 |
| Discrimination | `./le rfc discriminate-record` for every tagged unit |
| Real entry point | D1: the installed XFRM state read back on Linux (`ai/rules/platform-linux.md`) |
| Interop | D1: strongSwan behind a NAT that maps 4500 to another port |

## Owner decisions

### Owner questions (2026-10-13, design phase; all four are answered: Q-3 and Q-4 by the owner on 2026-10-13, Q-1 and Q-2 by the owner on 2026-10-14)

| Q | Question | Options | Recommendation and why | Answer |
|---|----------|---------|------------------------|--------|
| Q-1 | D2: what does Ze's EAP-TLS authenticator do about RFC 5216 Section 2.1.1 "the server MUST choose the sessionId to establish a new session" on a TLS 1.2 full handshake? | (a) Authenticator negotiates TLS 1.3 only (`MinVersion` 1.3); a TLS 1.2 peer is refused with a TLS alert. (b) Ze carries a patched copy of `crypto/tls` whose TLS 1.2 full handshake writes a fresh 32-octet server-chosen session_id. (c) Owner-approved deviation: Ze keeps the empty session_id, recorded with RFC 5246 Section 7.4.1.3 ("The server may return an empty session_id to indicate that the session will not be cached and therefore cannot be resumed.") and the reason; a deviation is never counted conformant, so RFC5216-2.1.1-4 leaves `wrong` for a recorded deviation, not for `enforced` | (c), stated with its cost. (a) refuses every strongSwan client left at its shipped default: `test/interop-ipsec/scenarios/responder-eap-tls13/strongswan.conf` records that charon ships `version_max = 1.2`. (b) forks a standard-library package whose TLS 1.2 server also issues a ticket ze never redeems (since `37dbe54095`), so the chosen id would identify a session ze refuses to resume. (c) is honest about what the wire carries. The ticket argument the two existing RFC5216-2.1.1-4 tags make does NOT hold any more: since `37dbe54095` a TLS 1.2 ticket is never redeemed, so "a new session can be established under an identity it chose" names an identity that is never usable | **answered by the owner, 2026-10-14: (b)**. Owner's words: "b - please look at [https://vincent.bernat.ch/en/blog/2026-go-netip-addrto6] on how we should patch the library", then "document this technique in the repository and document it in the spec and later code". The library is patched at build time with `go build -overlay`, never by a fork of the toolchain or a copy of `crypto/tls` in the tree. The ticket objection in the recommendation is removed by the design: the authenticator issues no TLS 1.2 ticket, so the chosen id names the only session there is, one that is never resumed, which RFC 5246 Section 7.4.1.3 allows ("Note that there is no requirement that the server resume any session even if it had formerly provided a session_id."). Designed under "D2 design"; the technique is documented in `docs/contributing/patching-the-standard-library.md` |
| Q-2 | D1: when a Child SA needs UDP encapsulation and the IKE SA has no authenticated peer endpoint (or the peer's port is 500), what does `childEncapPorts` do? It is the one port derivation every Child SA passes through (IKE_AUTH and CREATE_CHILD_SA through `newChildSA`, either rekey direction through `newRekeyedChild`, the rebind through `rebindChildSAs`; Files to Modify) | (A) Refuse the Child SA with an error naming the peer and the missing port; nothing installed. (B) Keep a fallback to port 4500 | (A). `ai/rules/principles.md`: code that cannot answer must say so. The fallback is exactly what hid D1b: the EAP path built its Child SA with no endpoint and nothing noticed. After D1b's reorder no production path reaches the refusal, so (A) costs nothing at runtime; it costs test fixtures (Q-4) | **answered by the owner, 2026-10-14: "revised A" (A′)**, replacing a first answer of plain (A) the same day. (1) `childEncapPorts` refuses the Child SA, with an error naming the peer and the reason and nothing installed, ONLY when the IKE SA has no authenticated peer endpoint; there is no 4500 fallback. (2) A peer endpoint port of 500 seen through a NAT, on Ze's 4500 NAT-T socket, is NOT refused: the Child SA follows it like any other port. (3) Ze's OWN local port is never 500 for encapsulation (preserved: "port 500 never requested"). The reading behind (2): RFC 7296 Section 2.23 "UDP encapsulation MUST NOT be done on port 500." binds the ports the endpoints send from, as the same paragraph's "An IPsec endpoint that discovers a NAT between it and its correspondent (as described below) MUST send all subsequent traffic from port 4500," shows, and the section also says responses "MUST be sent to the port from whence they came."; a NAT's external port is chosen by neither endpoint. Context the owner was given: strongSwan (`child_sa.c` `install_internal`, `ike_sa.c` `float_ports` / `update_hosts`) takes the Child SA ports from the IKE SA hosts (the `other_host` port), floats off 500 to 4500 on NAT detection, and has no fallback, so refusing a remote 500 would reject a connection strongSwan handles (the owner's objection); the no-endpoint state exists only in Ze. NOT verified: whether strongSwan's `float_ports` rewrites a remote 500 on the responder side (A-9) |
| Q-3 | A NAT rebinding AFTER establishment: `adoptAuthenticatedEndpoint` adopts a new peer port (no MOBIKE, Ze not behind a NAT), but installed Child SA states keep the old encap port, so ESP again leaves on a port IKE no longer uses. In this spec, or a journal row? | (i) In scope: migrate the installed states' encap ports when the IKE SA adopts a new endpoint, as `migrateMobikePair` does for MOBIKE. (ii) Journal row in `plan/journal/`, this spec fixes the ports at Child SA creation only | Owner's call: the MUST reads as a standing condition, so (ii) leaves RFC3948-2.1-2 violated after a rebind and the verdict could only be `enforced` for the creation-time claim. (i) adds a kernel migration path with its own failure modes (R-4) | **answered by the owner, 2026-10-13: (i), "migrate it (strongSwan)"**. strongSwan migrates every Child SA through `update_hosts` / `child_sa->update`, rekeying where the update is unsupported (research 2026-10-13; no strongSwan source is checked out on this host, so that behaviour is assumption A-7, verified at implementation). Designed under "Rebind design"; AC-11 and AC-13 to AC-18 are in scope |
| Q-4 | Approval to change the RFC-tagged tests listed under "RFC-tagged tests this spec changes" (TDD Test Plan) | approve / approve a subset / refuse | Needed under any answer to Q-1 and Q-2: the RFC3948-2.1-2 tag on `TestChildSANATTEncapPorts` asserts the defect. Units: `TestChildSANATTEncapPorts` (RFC3948-2.1-2, RFC7296-2.23-2: ports differ from 4500 in the fixture, 2.23-2 claim narrowed to "UDPEncap set when NAT is detected"), `TestEncapNeverRequestedOnPort500` (RFC7296-2.23-9), the five `TestBfm*` units (RFC7296-2.23-10/-11, helper only), and the two `rfc5216_tls_config_test.go` units (RFC5216-2.1.1-4 line withdrawn under every Q-1 answer). `TestChildSANoNATNoEncap` (RFC7296-2.23-2 negative) joins the list if Phase 3 changes its body (RFC-tagged test review, 2026-10-13) | **answered by the owner, 2026-10-13: "approve all RFC-tagged test changes"**. The approval covers every unit listed under "RFC-tagged tests this spec changes", including `TestChildSANoNATNoEncap` if Phase 3 changes its body. With Q-1 answered (b) and Q-2 answered (A) on 2026-10-14, every listed edit applies: `TestEncapNeverRequestedOnPort500` and the five `TestBfm*` units change for (A), and the RFC5216-2.1.1-4 lines move to the new wire test for (b). The commands are under "Owner approval commands" in the TDD plan |

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

- [ ] `docs/architecture/core-design.md` - the le toolchain environment (`internal/le/go/toolchain` declares it)
  → Constraint: every le action that starts a Go command takes its environment from `gotoolchain`; the overlay joins that environment there, once (D2 design, "Reach")
- [ ] `docs/architecture/testing/tracked-build-gate.md` - "Failure diagnostics and selftest deadlines"
  → Constraint: the tracked build "disable[s] persisted Go settings, ambient `GOFLAGS`, and enclosing workspaces", on purpose; the overlay therefore reaches it as an explicit `-overlay` argument from `stdpatch`, never through ambient GOFLAGS, and the page states it in the D2 phase
- [ ] `docs/architecture/appliance/builder.md` - the gok build environment (`SetGokGoEnv`, `ExecuteGok`)
  → Constraint: gok's go subprocesses inherit this process's environment and use the host go (`GOTOOLCHAIN=local`), so the overlay is generated for the host GOROOT and composed into GOFLAGS there; the page gains the item in the D2 phase
- [ ] `docs/contributing/patching-the-standard-library.md` - the technique (new, 2026-10-14)
  → Constraint: it states the mechanism is not built yet; the D2 phase restates it as built (AC-24)

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc3948.md`, `rfc/short/rfc5216.md`
  → Constraint: the RFC3948-2.1-2 row carries `{single-polarity: positive; ... no ze code path produces a non-4500 encap port ...}`, false since MOBIKE and false after this fix; the row annotation and the Enrolment-reason prose "2.1-2 (the ESP SA uses UDP port 4500 for encapsulation)" are corrected in the D1 phase
  → Constraint: RFC scope is frozen (2026-10-08): no new summary. `rfc/full/rfc5246.txt` was fetched 2026-10-13 as RFC TEXT only, to quote Section 7.4.1.3 for Q-1; it is not enrolled

### RFC text read (verbatim, `rfc/full/`)

| RFC, section | Sentence | Bears on |
|--------------|----------|----------|
| RFC 3948 Section 2.1 | "the Source Port and Destination Port MUST be the same as that used by IKE traffic," | D1 (both ports) |
| RFC 3948 Section 5.2 | "It reaches the server as UDP <Y,4500>, where Y is the dynamically assigned port." and "To Ari: Server to NAT, <traffic desc1>, UDP encap <4500, Y>" | D1, illustration only: this is Section 5.2's transport-mode conflict figure, not an obligation. It shows the RFC itself putting Y in the server's encap destination; the obligation is Section 2.1's sentence above |
| RFC 7296 Section 2.23 | "they MUST be accepted coming from any port and responses MUST be sent to the port from whence they came." | D1: IKE already answers Y (`remoteUDPAddr`); ESP must match |
| RFC 7296 Section 2.23 | "UDP encapsulation MUST NOT be done on port 500." | Q-2, R-3: read under Q-2 = A′ as binding Ze's own port (AC-10c) |
| RFC 7296 Section 2.23 | "An IPsec endpoint that discovers a NAT between it and its correspondent (as described below) MUST send all subsequent traffic from port 4500," | Q-2 = A′: the port-500 ban is about the ports an endpoint sends from |
| RFC 7296 Section 2.23 | "A host behind a NAT SHOULD NOT do this type of dynamic address update if a validated packet has different port and/or address values" | AC-17: no rebind while Ze is behind a NAT (the existing `sa.BehindNAT` refusal in `adoptAuthenticatedEndpoint`) |
| RFC 7296 Section 2.23 | "SHOULD send all packets (including retransmission packets) to the IP address and port in the validated packet," | AC-11, AC-13, AC-14: after an adopted move, ESP follows IKE to the new port, which RFC 3948 Section 2.1 makes a MUST for the encap ports |
| RFC 4555 Section 3.3 | "When an IPsec SA is created, the tunnel header IP addresses (and port, if doing UDP encapsulation) are taken from the IKE_SA, not the IP header of the IKEv2 message requesting the IPsec SA." | D1c |
| RFC 5216 Section 2.1.1 | "If the peer's sessionId is null or unrecognized by the server, the server MUST choose the sessionId to establish a new session." | D2 |
| RFC 5246 Section 7.4.1.3 | "The server may return an empty session_id to indicate that the session will not be cached and therefore cannot be resumed." | Q-1 (c) |
| RFC 5077 Section 3.4 | "If a server is planning on issuing a session ticket to a client that does not present one, it SHOULD include an empty Session ID in the ServerHello." | Q-1: why crypto/tls leaves it empty; D2 design: the patch fills the id only when no ticket is issued, so this SHOULD still holds for every server that issues one |
| RFC 5246 Section 7.4.1.3 | "Otherwise, this field will contain a different value identifying the new session." and "Note that there is no requirement that the server resume any session even if it had formerly provided a session_id." | D2 design: a chosen id that is never resumed is permitted, for the authenticator and for every other ze TLS server the patch reaches |
| RFC 5216 Section 2.1.2 | "Based on the sessionId chosen by the peer, and the time elapsed since the previous authentication, the EAP server will decide whether to allow the continuation or to choose a new session." | D2 design: refusing every TLS 1.2 continuation (owner ruling 2026-09-29) and issuing no TLS 1.2 ticket is the server's decision |
| RFC 9190 Section 2.1.2 | "the EAP-TLS server MUST send one or more post-handshake NewSessionTicket messages (each associated with a PSK, a PSK identity, a ticket lifetime, and other parameters) in the initial authentication." | D2 design: why `SessionTicketsDisabled` is set for TLS 1.2 only, never on the base config |

**Key insights:**
- D1 is three producer sites, not one: the port default (`installChildSA`), the EAP adoption order (`handleResponderEAP`), the rekey copy (`rekey.go`); plus the local port constant (D1d)
- `remoteUDPAddr` already returns the authenticated endpoint (`sa.peerEndpoint`) that IKE answers to; the Child SA must use the same value, once adopted
- D2 cannot be fixed inside `crypto/tls`'s public API: there is no hook for the ServerHello session_id. Q-1 (b) patches the package at build time through `go build -overlay` ("D2 design")
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
  → Decision: MOBIKE already re-derives ports from the IKE SA and migrates installed states (`TunnelMigrator.MigrateTunnel` with `LocalPort`/`RemotePort`). `migrateMobikeChild` walks the installed pairs (`ps.getChildSA()` and `ps.supersededChild`, skipping when `ps.getPendingChild()` is set) and `migrateMobikePair` moves one pair, taking the local endpoint from `sa.mobike.local`. The AC-11 rebind reuses both, with the local endpoint passed in (Rebind design)
  → Constraint: `mobikeAvailable` = `sa.mobike.canMigrate && !wantsTransportMode(sa) && !sa.UseTransportMode && sa.nattSocket != nil`; `canMigrate` is captured in `bindSockets` (`sa.go`) as "`dataplane.Get()` implements `TunnelMigrator`"
- [ ] `internal/component/ike/dataplane/xfrm_migrate_linux.go`, `xfrm_linux.go`, `dataplane.go` - `TunnelMigrator`
  → Constraint: only `xfrmMobikeBackend.MigrateTunnel` implements `TunnelMigrator`; `xfrm_linux.go` returns that backend only when `xfrmMigrationProbe` finds the kernel handles `XFRM_MSG_MIGRATE_STATE`. The VPP backend and a kernel without the message have no migrator
  → Constraint: `validateXFRMMigration` refuses IPv6 endpoints (wrapping `ErrNotSupported`), and with errors that wrap no sentinel a non-unicast or multicast endpoint, a missing SPI or ReqID, a local port other than 4500 (so the test-port hook's p+1 is refused) and a peer port of 0 or 500; `prepareMigration` accepts only tunnel-mode states and policies. The interface contract: a failed migration restores the old endpoints, or returns `ErrTunnelMigrationLost`, on which the caller MUST retire the Child SA and never reinstall its keys
- [ ] `internal/component/ike/engine/inbound.go` - `handleOwnedInbound`, `handleCreateChildSAOwned`
  → Constraint: the established-SA call of `adoptAuthenticatedEndpoint` is after `decryptAndParse` and the Message ID window, so only an authenticated, in-window message moves the endpoint; the rebind hooks there. A peer-initiated rekey that `respondChildRekey` cannot build is answered with `ps.respondError(..., notifyForRefusal(err), ...)`
- [ ] `internal/component/ike/engine/established.go` - `startChildRekey` and the owner tick
  → Constraint: the tick raises `startChildRekey` while `childLT.softExpired(now) && ps.pendingRekey == nil`, a level trigger: a held request window or a TEMPORARY_FAILURE / refusal hold returns without rekeying, and the next tick raises it again. `startChildRekey` rekeys `ps.getChildSA()` only
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

**Behavior to change:** D1, D1b, D1c, D1d; the post-establishment rebind (Q-3 answered (i), "Rebind design"); D2 per Q-1.

**Behavior to preserve:** send form follows NATDetected only; receive form follows UDPEncap; `AcceptBothESPForms` on every inbound state; MOBIKE port derivation and migration; Ze's own encap port is never 500 (Q-2 = A′); the EAP decrypt-error branches (other spec); RFC 9190 TLS 1.3 tickets and `redeemTLS13Only`; every ze TLS 1.2 server that issues a session ticket keeps an empty ServerHello session_id (RFC 5077 Section 3.4); TLS 1.3 handshakes and every TLS client are untouched by the patch.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- D1: an authenticated IKE_AUTH (responder: request; initiator: response) arriving on Ze's NAT-T socket from a translated source (A, Y); a CREATE_CHILD_SA or Child SA rekey on that IKE SA
- Rebind: an authenticated, in-window message on the established IKE SA from (A, Y2), Y2 not Y (`handleOwnedInbound`)
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
| EAP-TLS authenticator ↔ crypto/tls | `tls.Config`, its `GetConfigForClient` (TLS 1.2 ticket suppression) and the build-time overlay of `handshake_server.go` (D2 design) | No: `TestRFC5216TLS12FullHandshakeServerHelloSessionID` reads the ServerHello bytes (AC-2) |
| build tooling ↔ GOROOT | `go ... -overlay` from `internal/le/go/stdpatch` through every Go command that compiles ze (D2 design, "Reach") | No: AC-20, AC-21 |

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

## Rebind design (Q-3 answered (i), "migrate it (strongSwan)")

**Trigger.** On an established IKE SA, an authenticated, in-window message (any exchange; on a quiet
tunnel, the peer's DPD INFORMATIONAL) arrives from (A2, Y2), which differs from `sa.peerEndpoint`,
with MOBIKE not negotiated and Ze not behind a NAT. `adoptAuthenticatedEndpoint` adopts it under its
existing rule and now also reports that it moved the endpoint: a boolean result, true only on the
later-and-different branch it already logs at Info. The three pre-establishment callers
(`responder.go` twice, `fsm.go`) see a first observation and ignore the result.

**Producer that decides.** `(*PeerSession).rebindChildSAs` (new, `internal/component/ike/engine/rebind.go`),
called by `handleOwnedInbound` right after the adoption reports a move, under the
`childLifecycleMu` that call already holds for MOBIKE. It carries the RFC 7296 Section 2.23 quote
("SHOULD send all packets ...") and the RFC 3948 Section 2.1 quote. Mode and backend are per IKE SA,
so it decides once for the SA, reading the rows in order:

| # | Condition | Branch | Outcome |
|---|-----------|--------|---------|
| 1 | `childMigrationAvailable(sa)` holds: the capability half of today's `mobikeAvailable` (`canMigrate`, tunnel mode by `wantsTransportMode` and `UseTransportMode`, a NAT-T socket), extracted so `mobikeAvailable` calls it and one predicate serves both | migrate | the MOBIKE per-child loop (`migrateMobikeChild`, gaining the local endpoint as a parameter) moves the current pair and a distinct superseded pair through `migrateMobikePair`; local and remote come from `childEncapPorts` (the NAT-T socket's port, `sa.peerEndpoint`). On success the child's `udpLocalPort`/`udpRemotePort` hold the new ports, as MOBIKE leaves them. While `ps.getPendingChild()` is set the loop moves nothing, as for MOBIKE, and row 2's rekey is marked owed instead |
| 2 | the predicate is false: a transport-mode Child SA, the VPP backend, a kernel without `XFRM_MSG_MIGRATE_STATE` | rekey | `ps.rebindRekeyOwed` is set. The owner tick raises `startChildRekey` while `ps.rebindRekeyOwed && ps.pendingRekey == nil`, beside the soft-lifetime trigger, so a held window or a TEMPORARY_FAILURE or refusal hold defers the rekey and never drops it. The rekeyed child takes its ports from the IKE SA (D1c, AC-8), so it lands on Y2, and the existing rekey completion retires the old pair. The flag clears when a rekeyed Child SA installs, ze-initiated (`applyChildRekeyResponse`) or peer-initiated (`respondChildRekey`), since either takes the IKE SA's ports. A superseded pair present at the move is not rekeyed: it is already retiring, and outbound uses the current pair |
| 3 | row 1 ran and `MigrateTunnel` returned any error that is not `ErrTunnelMigrationLost` (tested with `errors.Is`, which sees through `errors.Join` and `%w`). The `TunnelMigrator` contract says every such error leaves the old endpoints standing. Most of them wrap no sentinel at all: `validateXFRMMigration` wraps `ErrNotSupported` for IPv6 endpoints only, and its unicast, SPI/ReqID and port refusals (a local port other than 4500 under the test-port hook, a peer port of 0 or 500) wrap nothing; a restored rollback returns the bare cause; `prepareMigration`, `errNoESPFormReceiver`, `espForms.Watch` and `migrateMobikePair`'s missing-endpoint error wrap nothing either. So the branch MUST NOT test for `ErrNotSupported` | rekey | as row 2, logged at Info with the migration error and the peer |
| 4 | row 1 ran and `MigrateTunnel` returned `ErrTunnelMigrationLost` | retire | the IKE SA closes the way `mobikeFatal` closes a MOBIKE SA (`StateDead`, reestablish), because the `TunnelMigrator` contract forbids reinstalling the lost keys |
| - | endpoint unchanged, MOBIKE negotiated, or Ze behind a NAT | none | `adoptAuthenticatedEndpoint` adopts nothing and reports no move, so `rebindChildSAs` is not called (AC-17) |

**One log line per branch.** `rebindChildSAs` logs the branch it took, once per move, with the attributes
`peer`, `old-endpoint` and `new-endpoint`. Each message names the problem first, then what Ze does about
it, then the operator's step where one is owed (`ai/rules/user-facing-errors.md`). They are distinct
from the soft-lifetime "child-sa: rekey initiated", which `startChildRekey` still logs for every rekey
and which cannot tell an owed rekey from a lifetime one. The interop checker matches these texts.

| Row | Level | Message | Extra attributes |
|-----|-------|---------|------------------|
| 1 | Info | "ike: peer endpoint moved, Child SAs migrated to the new address and port" | `child-sas` (pairs moved) |
| 2 | Info | "ike: peer endpoint moved, Child SA cannot be migrated here, so it is rekeyed onto the new address and port" | `reason`: "transport mode", "the dataplane cannot migrate Child SAs" or, for row 1's pending-child case, "a new IKE SA with the peer is being set up" |
| 3 | Info | "ike: peer endpoint moved, migrating the Child SA failed and the old endpoints stay in place, so it is rekeyed onto the new address and port" | `error` |
| 4 | Error | "ike: peer endpoint moved, the Child SA was lost while migrating it, so the IKE SA is closed and set up again. Check the kernel IPsec state with: ze doctor" | `error` |

**Owed flag and promotion.** `ps.rebindRekeyOwed` describes the pair the move left on the old ports. It
clears when `resolvePendingAfterOwnerLoop` (`fsm.go`) promotes a pending child to current: the promoted
child was built by `newChildSA` through `childEncapPorts` from the pending IKE SA's own endpoint, so the
owed rekey no longer describes the current child, and the old pair goes with the old IKE SA. If the
promoted SA's endpoint is itself stale, its next authenticated message from the new endpoint makes its
own adoption and its own rebind. Reaping a pending SA that never authenticated (`reapStalePending`, `established.go`) leaves the flag set,
and that is correct, not merely harmless: the reap keeps the current pair, which still sits on the old
ports, so the owed rekey is still due. Proof: `TestRFC3948RebindOwedRekeyClearsOnPromotion`.

**Flag left over by an SA death.** The flag lives on `PeerSession` and clears only on a rekey install or a
promotion. When the owner IKE SA dies with no pending SA to promote (`pendingReturn`), or row 4 or DPD
retires it, the flag stays set, and the next fresh IKE SA's Child SA takes one needless rekey on its
first tick. The cost is bounded at one rekey per move, the rekeyed child lands on the IKE SA's own ports
(D1c), and the rekey clears the flag, so this is accepted rather than cleared at each child-removal site.

**strongSwan reference (A-7, unverified on this host).** charon's `ike_sa.c::update_hosts` calls
`child_sa->update` for every Child SA, and calls `rekey_child_sa` for one whose update returns
`NOT_SUPPORTED`. Rows 1 to 3 are that rule in Ze's terms.

**Simpler designs considered.** Always rekey: one branch, but it rekeys on every NAT rebind where the
kernel could move the live SA with its replay window and sequence (`TunnelMigrator` contract), and
the owner's answer names migration. Always migrate: no migrator exists for transport mode, VPP or a
kernel without `XFRM_MSG_MIGRATE_STATE`, so those Child SAs would stay on the old port.

## D2 design (Q-1 answered (b), 2026-10-14: patch `crypto/tls` through `go build -overlay`)

The contributor page for the technique is `docs/contributing/patching-the-standard-library.md`
(written in the design phase, stating that the mechanism is not built yet and that this spec builds
it). This section is the design; the page is the how-to and must agree with it.

**Two changes, both needed.** The overlay patch alone fails for a client that offers the session
ticket extension: crypto/tls then issues a ticket, RFC 5077 Section 3.4 keeps the id empty, and RFC
5216 Section 2.1.1 is violated again. The authenticator change alone changes nothing: with no ticket,
go1.27.2 still sends an empty id (A-3).

| # | Where | What | Why |
|---|-------|------|-----|
| 1 | `crypto/tls` `serverHandshakeState.doFullHandshake` (`handshake_server.go`), by overlay | directly after `hs.hello.ticketSupported` is computed, when it is false, fill `hs.hello.sessionId` with 32 octets read from `c.config.rand()`, and return the read error | RFC 5216 Section 2.1.1's MUST; 32 octets is the `SessionID<0..32>` maximum of RFC 5246 Section 7.4.1.2 and what AC-2 reads |
| 2 | `crypto/tls`, new file `zz_ze_server_session_id.go`, by overlay | one exported constant, `ZeServerChoosesSessionID`, documented as the marker of this patch | the compile-time guard ("Loud failure") |
| 3 | `internal/core/eap/eap_tls.go` `newTLSMethod` | set `GetConfigForClient`: when `ClientHelloInfo.SupportedVersions` does not hold TLS 1.3, answer a `Clone` of the authenticator config with `SessionTicketsDisabled` true; otherwise answer nil (the base config) | no TLS 1.2 ticket, so step 1 fires for every TLS 1.2 client and RFC 5077 Section 3.4's SHOULD is not in play |
| 4 | `internal/core/eap/eap_tls.go` | a blank declaration that references `tls.ZeServerChoosesSessionID`, with the comment that names the page | the authenticator cannot compile without the patch |

**Why step 3 keys on the offered versions.** `MinVersion` is TLS 1.2 and `MaxVersion` TLS 1.3, so
crypto/tls negotiates 1.3 whenever the client offers it, and a client that does not offer it can only
reach 1.2. `SessionTicketsDisabled` on the base config is wrong: `handshake_server_tls13.go` reads the
same field, so it would also stop the TLS 1.3 NewSessionTicket that RFC 9190 Section 2.1.2 requires
("the EAP-TLS server MUST send one or more post-handshake NewSessionTicket messages").

**No conflict with RFC 5216 resumption or the existing code.** RFC 5216 Section 2.1.2 gives the
decision to the server ("the EAP server will decide whether to allow the continuation or to choose a
new session"), and the owner ruled on 2026-09-29 that the authenticator never resumes TLS 1.2
(`redeemTLS13Only`, `37dbe54095`). With tickets disabled, `checkForResumption` returns before
`UnwrapSession`, so `redeemTLS13Only` and `refuseResumption` are not reached on TLS 1.2 and keep
their TLS 1.3 role. The id is never resumed, which RFC 5246 Section 7.4.1.3 permits. Issuing no TLS
1.2 ticket also removes the ticket ze could never redeem that Q-1's recommendation objected to. The
comment in `newTLSMethod` "the ticket above goes out either way" becomes false for TLS 1.2 and is
rewritten (`ai/rules/stale-comments.md`). The implementer checks which `rfc/discrimination/rfc5216.json`
records the new `GetConfigForClient` stales (the enforced 2.1.1-5, 2.1.1-10 and 2.1.2-1 units drive
`newTLSMethod`) and re-records them.

**Reach of the patch.** The overlay changes `crypto/tls` for every TLS server in every ze binary (web,
gNMI, looking glass, and the others). The hunk is one statement group in one function, so its reach
is exactly this table:

| Handshake on any ze TLS server | Before | After |
|--------------------------------|--------|-------|
| TLS 1.2 full, a ticket is issued | empty id | empty id (unchanged; RFC 5077 Section 3.4) |
| TLS 1.2 full, no ticket (client offers none, or tickets disabled) | empty id | fresh 32-octet id |
| TLS 1.2 resumed | client's id echoed | unchanged (`doResumeHandshake`) |
| TLS 1.3 | `handshake_server_tls13.go` | unchanged (other file) |
| ze as a TLS client | `handshake_client.go` | unchanged (other file) |

The second row is safe for non-EAP servers: RFC 5246 Section 7.4.1.3 says "Otherwise, this field will
contain a different value identifying the new session." and that there is "no requirement that the
server resume any session even if it had formerly provided a session_id.". A client that offers the id
later gets a full handshake, because crypto/tls resumes only from tickets (`checkForResumption`). The
scratch probe of 2026-10-14 (A-3) showed all three server rows on go1.27.2.

**Narrower patch considered and rejected.** A per-`Config` opt-in would need a new `Config` field, so a
second hunk in `common.go` (the struct and `Clone`), the package's most edited file, which makes a
toolchain bump more likely to refuse. What it protects (non-EAP servers keeping an empty id when no
ticket is issued) has no consumer. What would overturn this: a ze component or a known peer that reads
or rejects a non-empty TLS 1.2 ServerHello session_id when no ticket follows.

**Where the change lives.** `internal/le/go/stdpatch/patches/crypto-tls-server-session-id.patch`: a
git unified diff with paths relative to GOROOT (`a/src/crypto/tls/handshake_server.go`, and a
`/dev/null` to `b/src/crypto/tls/zz_ze_server_session_id.go` creation). Its leading text, which `git
apply` skips, names the page and quotes RFC 5216 Section 2.1.1. The package `internal/le/go/stdpatch`
answers the overlay for one GOROOT. It imports only the standard library and nothing that references
the marker, so it compiles before any overlay exists. This follows the vendor patch precedent
(`internal/le/vendorpatch/patches/`), with one difference: a vendored file is in the tree and a
GOROOT file is not, so the patch is applied at build time, never committed as a patched copy.

**How the overlay is produced** (`stdpatch`, every call):

1. Resolve GOROOT by running `go env GOROOT` under the exact environment of the build that will use
   it. The `GOTOOLCHAIN` pin can select a downloaded toolchain whose GOROOT is under `GOMODCACHE`, and
   gokrazy runs the host go with `GOTOOLCHAIN=local`, so a GOROOT read anywhere else can be the wrong one.
2. Key the result by a SHA-256 over the `go version` string, the patch bytes and each target GOROOT
   file's bytes.
3. When `cache/stdpatch/<key>/overlay.json` exists, answer it. `cache/` is the checkout's cache link,
   beside GOCACHE (`gotoolchain.GoCache`); the key makes reuse safe and lets sessions share it.
4. Otherwise copy each target file into a new temporary directory under `cache/stdpatch/`, run `git
   apply` there with `GIT_CEILING_DIRECTORIES` set to its parent so git cannot find the ze repository,
   write `overlay.json` mapping each absolute GOROOT path (the patched file, and the new marker file
   that does not exist in GOROOT) to its copy, and rename the directory to the key (atomic, so two
   sessions racing produce one result).
5. When `git apply` refuses (it has no fuzz: a changed context line refuses), return an error naming
   the Go version, the patch file, git's output and the page section "When the Go version changes".
   No caller builds without the overlay after this error.

**Reach: every Go command that compiles ze.**

| Path | Today | Change |
|------|-------|--------|
| the `./le` launcher bootstrap (`le` script, `build_le`) | `go build -tags ze_le,... ./cmd/ze`; that binary links `internal/core/eap` (`go list -deps -tags ze_le ./cmd/ze`) | runs `go run ./internal/le/go/stdpatch/overlay` first and builds with its `-overlay`; a refusal stops the bootstrap |
| every le action that builds or tests (`./le test unit`, `./le go lint`, `./le build`, functional binaries, interop `zebuild.go`, `./le perf` on the Mac) | env from `gotoolchain.Toolchain.Environment` / `Overrides` | `Overrides` adds `-overlay=<path>` to GOFLAGS, appended to an inherited GOFLAGS, never replacing it |
| le sites that set GOFLAGS themselves: `repo/bootstraps/tracked.go` (`-mod=vendor`), `repo/compiles/repositorytrackedbuild.go` (`GOENV=off`, `-buildvcs=false`), `repo/changed/selector.go` (empty), `go/module/move.go`, `setup/proto_generate.go` | build their own env | each composes the overlay through the same `stdpatch` call (the tracked build, which refuses ambient GOFLAGS by design, takes it as an explicit `-overlay` argument), or, when it compiles nothing that links `internal/core/eap`, carries a comment saying so. The marker makes a missed compiling site fail, so the guard enumerates the sites |
| `./le rfc discriminate-record` (`writeOverlay`, `setGoFlagsOverlay`, `environment`) | writes its own overlay | merges the stdlib entries into its overlay JSON: a command-line `-overlay` REPLACES the one in GOFLAGS (A-11), so without the merge every discrimination run of a unit that links `internal/core/eap` stops compiling |
| gokrazy (`appliance.SetGokGoEnv`, both gok entry points) | composes GOFLAGS (`ensureModcacheRW`) | composes the overlay the same way, for the host go's GOROOT |
| QEMU guests that compile (`test/qemu/run.go`, `run_exec.go`) | export `GOFLAGS=-buildvcs=false` in the guest | the guest has its own GOROOT, so the guest script runs the `stdpatch` generator in the guest and exports its overlay |
| Docker images that compile ze (`docker/Dockerfile`, `docker/Dockerfile.lab`) | `FROM golang`, plain `go build` | a `RUN` of the generator before the build, its path in GOFLAGS |
| CI workflows that run go directly (`.github/workflows/codeql.yml`, `qemu-nightly.yml`, `govulncheck.yml`) | plain `go build`, `go test`, govulncheck | a step that exports GOFLAGS from the generator before the first go command (A-10 for govulncheck) |
| a developer's plain `go build`/`go test`, gopls, an agent's `gopls` from Bash | no le | fails with `undefined: tls.ZeServerChoosesSessionID` at the marker reference, whose comment names the page; the page gives the remedy: `./le go stdpatch overlay` prints the path, used in GOFLAGS or gopls `build.buildFlags`. Syntactic gopls queries (`gopls symbols`) are unaffected |

`./le go stdpatch overlay` is a new le command (`ai/rules/cli.md`): it prints the absolute overlay
path for the go on PATH and exits non-zero with the refusal of step 5.

**Loud failure.** Three guards, each covering a hole the others leave:

| Guard | Catches | Misses |
|-------|---------|--------|
| compile-time marker (steps 2 and 4) | any build of any binary that carries the EAP-TLS authenticator without the overlay: CI, a Dockerfile, a developer, a missed le site | an overlay that adds the marker but not the hunk |
| `TestRFC5216TLS12FullHandshakeServerHelloSessionID` and `TestPatchedCryptoTLSServerSessionID` | the hunk missing or wrong | a shipped binary built by a path the tests never ran |
| step 5 refusal | a toolchain bump that moved the patched code | nothing on its own: it is what turns a bump into a decision |

A runtime self-probe (a loopback TLS 1.2 handshake at `newTLSMethod` that refuses EAP-TLS when the id
is empty) was rejected: it keeps plain builds working, but it ships an unpatched binary that fails at
the operator's site. A test-only guard was rejected because a test proves its own build, never the
shipped one.

**When the Go version changes.** go.mod pins `go 1.27.2`. On a bump, the first build either applies
the patch cleanly (context unchanged; the two tests then decide) or refuses with step 5. On a refusal
the contributor reads upstream `doFullHandshake`: when it now chooses a session id itself, the patch,
the marker file, the marker reference and `stdpatch` are deleted (`ai/rules/no-layering.md`);
otherwise the patch is regenerated against the new GOROOT file with the procedure on the page and the
two tests run.

**Comments the code owes** (owner: "document it in the spec and later code"). Each of these carries a
comment that names `docs/contributing/patching-the-standard-library.md` and quotes RFC 5216 Section
2.1.1 "If the peer's sessionId is null or unrecognized by the server, the server MUST choose the
sessionId to establish a new session." (`ai/rules/rfc-compliance.md`): the patch file's leading text,
the patched hunk itself (directly above the statements), the marker file, the `stdpatch` package doc
and the function that applies the patch, and the EAP-TLS call site in `newTLSMethod`
(`GetConfigForClient` and the marker reference). A caller elsewhere carries `// RFC 5216 Section 2.1.1`.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| responder IKE_AUTH (PSK) from (A, 14500) on the NAT-T socket | → | `handleAuthRequest` → `newChildSA` → `installChildSA` | `TestRFC3948ResponderChildSAEncapsulatesToTheIKEPeerPort` (`internal/component/ike/engine/rfc3948_encap_port_test.go`) |
| responder final IKE_AUTH (AUTH-from-MSK) from (A, 14500) | → | `handleResponderEAP` → adoption → `buildAuthResponse` | `TestRFC3948EAPResponderChildSAEncapsulatesToTheIKEPeerPort` (same file) |
| initiator IKE_AUTH response from (B, 14501) | → | `fsm.go` adoption → `initiatorFirstChildSA` | `TestRFC3948InitiatorChildSAEncapsulatesToTheIKEPeerPort` (same file) |
| real Linux XFRM | → | `installChildSA` → `xfrmBackend.InstallSA` | `TestRFC3948InstalledStateCarriesTheIKEPeerPort` (`internal/component/ike/engine/rfc3948_encap_port_integration_linux_test.go`, `integration && linux`) |
| strongSwan behind a port-translating NAT | → | whole stack | interop `real-nat-port-translated-ze-responder`, `real-nat-port-translated-eap-ze-responder` |
| established-SA authenticated INFORMATIONAL from (A, Y2) after the SA was set up from (A, Y) | → | `handleOwnedInbound` → `adoptAuthenticatedEndpoint` (moved) → `rebindChildSAs` → migrate or owed rekey | `TestRFC3948RebindMigratesTunnelChildSAs`, `TestRFC3948RebindRekeysWithoutAMigrator` (`internal/component/ike/engine/rfc3948_rebind_test.go`) |
| strongSwan behind a NAT whose mapping changes port after establishment | → | whole stack, both branches | interop rebind step in `real-nat-port-translated-ze-responder` (tunnel) and `real-nat-port-translated-transport-ze-responder` (transport) |
| TLS 1.2 ClientHello to the authenticator | → | `newTLSMethod` (`GetConfigForClient`) → patched `doFullHandshake` | `TestRFC5216TLS12FullHandshakeServerHelloSessionID` (`internal/core/eap/rfc5216_session_id_test.go`) and interop `responder-eap-tls12` |
| any le build or test of a package linking `internal/core/eap` | → | `gotoolchain.Toolchain.Overrides` → `stdpatch` overlay | `TestToolchainEnvironmentCarriesTheStdlibOverlay` (`internal/le/go/toolchain/`), and every `./le test unit` run of `internal/core/eap` compiling at all |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | peer's IKE arrives from port Y behind a NAT | ESP encapsulated to port Y |
| AC-1 (design, 2026-10-13) | Ze responder (PSK), NAT detected, the authenticated IKE_AUTH arrives on Ze's NAT-T socket bound to port L from (A, Y), Y not 4500 | inbound XFRM state: encap espinudp sport Y dport L; outbound state: sport L dport Y. Proof: `TestRFC3948ResponderChildSAEncapsulatesToTheIKEPeerPort` (unit, the installed `SAParams`), `TestRFC3948InstalledStateCarriesTheIKEPeerPort` (Linux XFRM read back), interop `real-nat-port-translated-ze-responder` (traffic both ways through the tunnel, ze's state shows dport Y) |
| AC-2 | D2 | Q-1 answered (b), 2026-10-14 (rows below) |
| AC-2 (design, 2026-10-14) | TLS 1.2 ClientHello with an empty or unknown session_id reaches Ze's authenticator, in two cases: the client offers the session ticket extension, and the client offers none | ServerHello carries a 32-octet session_id that differs from the client's, and the server flight carries no NewSessionTicket. Proof: `TestRFC5216TLS12FullHandshakeServerHelloSessionID` reads the ServerHello bytes (two subtests, one per case; red at HEAD: length 0), interop `responder-eap-tls12` authenticates a TLS 1.2 strongSwan client under the patched build |
| AC-19 | patched `crypto/tls`, a plain TLS 1.2 server outside EAP | a server that issues a ticket sends an empty session_id (RFC 5077 Section 3.4); a server that issues none sends 32 octets; a TLS 1.3 handshake is byte-for-byte the unpatched shape in its legacy_session_id echo. Proof: `TestPatchedCryptoTLSServerSessionID`, three subtests |
| AC-20 | each Go command path that compiles a package linking `internal/core/eap` (D2 design, "Reach") | it builds with the overlay; a build of `internal/core/eap` without the overlay fails with `undefined: tls.ZeServerChoosesSessionID`. Proof: `TestToolchainEnvironmentCarriesTheStdlibOverlay` (an inherited GOFLAGS is kept and the overlay appended), `TestDiscriminateOverlayKeepsTheStdlibPatch` (the merged JSON holds both maps), and the `./le` bootstrap, a gokrazy build, a Docker image build and the CI workflows each run green once with the change |
| AC-21 | the patch no longer applies to the GOROOT file (a fixture GOROOT whose `handshake_server.go` differs in a context line) | `stdpatch` refuses with an error naming the Go version, the patch file, git's output and the page section; no overlay is written. Proof: `TestStdpatchRefusesAPatchThatDoesNotApply`; plus `TestStdpatchOverlayIsContentAddressed` (same inputs, same path; changed GOROOT file, new path) |
| AC-22 | the authenticator, TLS 1.3 client | the NewSessionTicket RFC 9190 Section 2.1.2 requires still goes out, and a TLS 1.2 client receives none. Proof: the existing RFC 9190 Section 2.1.2 ticket test stays green, plus the no-NewSessionTicket assertion of AC-2 |
| AC-23 | the patch file, the patched hunk, the marker file, `stdpatch` and `newTLSMethod` | each carries the comment of D2 design "Comments the code owes": the page path and the RFC 5216 Section 2.1.1 quote. Proof: review gate; `./le rfc check` for the implementing-function quote |
| AC-24 | `docs/contributing/patching-the-standard-library.md` | after this spec, the page states the mechanism as built, with source anchors on `stdpatch` and the patch file, and no sentence saying it is not built yet. Proof: `./le doc check verify` |
| AC-3 | the `wrong` verdicts of RFC3948-2.1-2 and RFC5216-2.1.1-4, after this spec's producer fix | RFC3948-2.1-2 reaches `enforced`: tagged tests prove the quoted sentence at creation (AC-1, AC-6 to AC-9) and after a rebind (AC-11, AC-13, AC-14), and an agent that did not write those tests re-judges it with `./le rfc audit-stamp ... mode rejudge`. RFC5216-2.1.1-4, Q-1 answered (b): it reaches `enforced` the same way, through the ServerHello wire test `TestRFC5216TLS12FullHandshakeServerHelloSessionID` (tag RFC5216-2.1.1-4 positive, discrimination record from removing step 3's `GetConfigForClient`, under which the ticket-offering subtest goes red), and an independent re-judge. The summary row names the overlay patch and the page. The two config-level 2.1.1-4 tags are withdrawn (TDD plan). Moved here from "Blocked by" in `plan/pre-release/spec-rfc-verdict-fix-ike-eap.md` (parent P-3, 2026-09-28) |
| AC-4 | D2 area, RFC5216-2.1.2-1 (weak), RFC 5216 Section 2.1.2: "If the EAP server is resuming a previously established session, then it MUST include only a TLS change_cipher_spec message and a TLS finished handshake message after the server_hello message." crypto/tls renews the session ticket on a TLS 1.2 resumption, so a NewSessionTicket follows server_hello | owner ruling 2026-09-29: ze's authenticator refuses TLS 1.2 resumption (a TLS 1.2 session is never redeemed; TLS 1.3 tickets stay, per RFC 9190). The untagged failing test `TestRFC5216ResumingServerSendsNothingButChangeCipherSpecAndFinished` (`internal/core/eap/rfc5216_resumption_defect_test.go`, uncommitted) is replaced by a tagged proof of the fix, and the verdict reaches `enforced` by an agent that did not write that test. The fix removes the TLS 1.2 resumed conversation that `TestRFC5216ResumedFlightsCarryOnlyChangeCipherSpecAndFinished` drives (RFC5216-2.1.1-10, 2.1.1-5), so those tags are re-proven or re-judged in the same work. Moved here from "Blocked by" in `plan/pre-release/spec-rfc-verdict-fix-ike-eap.md` (P-3) |

| AC-4 status (2026-10-13) | - | CLOSED at HEAD: producer `37dbe54095`, independent re-judge to `enforced` in `c1f83ffa35` and `96535408fa` for RFC5216-2.1.2-1, 2.1.1-5 and 2.1.1-10. Nothing further owed here |
| AC-5 | Ze responder or initiator, NAT detected, peer's authenticated IKE endpoint is (A, 4500) | both states keep encap 4500/4500 (no-translation polarity). Proof: `TestRFC3948PeerOn4500KeepsPort4500` (unit) and the existing scenarios `real-nat-tunnel-control`, `real-nat-transport-ze-responder`, `real-nat-transport-ze-initiator` staying green |
| AC-6 | EAP responder: final IKE_AUTH (AUTH-from-MSK) from (A, Y) | Child SA installed with sport/dport Y as AC-1; `sa.peerEndpoint` is set before `buildAuthResponse` runs. Proof: `TestRFC3948EAPResponderChildSAEncapsulatesToTheIKEPeerPort`, interop `real-nat-port-translated-eap-ze-responder` |
| AC-7 | Ze initiator, authenticated IKE_AUTH response arrives from (B, Y) | first Child SA installed with dport Y (outbound) / sport Y (inbound). Proof: `TestRFC3948InitiatorChildSAEncapsulatesToTheIKEPeerPort` |
| AC-8 | a CREATE_CHILD_SA Child SA and a rekeyed Child SA on an IKE SA whose endpoint is (A, Y) | both installed with Y, taken from the IKE SA (not copied from the retired child). Proof: `TestRFC3948CreatedAndRekeyedChildSATakeTheIKESAPorts` |
| AC-9 | Ze's NAT-T socket bound to a port L other than 4500 (test-port hook) | local encap port is L on both states. Proof: `TestRFC3948LocalEncapPortIsTheNATTSocketPort` |
| AC-10 | (Q-2 = A′) a Child SA that needs UDP encapsulation and whose IKE SA has no authenticated endpoint, on each path through `childEncapPorts`: IKE_AUTH, CREATE_CHILD_SA (ze initiating or responding), Child SA rekey (ze initiating or responding) | `childEncapPorts` refuses with an error naming the peer and the reason; no SA or policy installed. IKE_AUTH: the childless response `buildChildlessAuthResponse` already gives a refused install. CREATE_CHILD_SA or rekey ze answers: the error notify `ps.respondError(..., notifyForRefusal(err), ...)` already sends for a refused child, and the IKE SA stays up. CREATE_CHILD_SA or rekey ze initiated: the new child is not installed, the error is logged with the peer, and a rekey leaves the old Child SA in use. Proof: `TestRFC3948ChildSARefusesEncapWithoutAnIKEPort`, one subtest per path (negative polarity of RFC3948-2.1-2) |
| AC-10b | (Q-2 = A′) NAT detected, Ze on its NAT-T socket (local port 4500 or the test-port hook's port), the peer's authenticated endpoint is (A, 500) because a NAT mapped it there | the Child SA is NOT refused: inbound state encap sport 500 dport L, outbound sport L dport 500, as for any other peer port (RFC 3948 Section 2.1; RFC 7296 Section 2.23 "responses MUST be sent to the port from whence they came."). Proof: `TestRFC3948ChildSAFollowsANATMappedPeerPort500` (tag RFC3948-2.1-2 positive) |
| AC-10c | (Q-2 = A′) every path through `childEncapPorts`, both NAT states, any peer port including 500 | Ze's own encap port (outbound sport, inbound dport) is never 500. Proof: `TestEncapNeverRequestedOnPort500` with its claim narrowed to Ze's own port (RFC7296-2.23-9 positive), plus `TestRFC3948LocalEncapPortIsTheNATTSocketPort` |
| AC-11 | IKE SA adopts a new peer endpoint (A, Y2) after establishment, no MOBIKE, Ze not behind a NAT; tunnel mode; the backend implements `TunnelMigrator` | Rebind design row 1: the current and the superseded pair are migrated through `MigrateTunnel` with `RemotePort` Y2 and the NAT-T socket's port; no rekey is started. Proof: `TestRFC3948RebindMigratesTunnelChildSAs` (unit over a recording `TunnelMigrator`, tag RFC3948-2.1-2 positive); interop rebind step in `real-nat-port-translated-ze-responder`, which asserts the row 1 line (R-11: the lab runs only where `ipsec-mobike` is present) |
| AC-12 | docs and the summary | `docs/guide/ipsec.md` (`udp-encapsulation` sentence), `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` ("NAT traversal": ports come from the IKE SA), `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` (child build), `rfc/short/rfc3948.md` (RFC3948-2.1-2 annotation and Enrolment-reason prose) state the new behaviour in the phase that changes it; D2 per Q-1 in `rfc/short/rfc5216.md` and `docs/guide/ipsec.md`; the rebind (migrate or rekey) in `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` "NAT traversal" and `docs/architecture/ike/ipsec-13-rekey-wire.md` |
| AC-13 | as AC-11, but the backend implements no `TunnelMigrator` (VPP, or XFRM on a kernel without `XFRM_MSG_MIGRATE_STATE`) | Rebind design row 2: no migration is attempted; a Child SA rekey is started on the next tick; the rekeyed pair installs with sport/dport Y2 and the NAT-T socket's port; the old pair is retired by the rekey completion. Proof: `TestRFC3948RebindRekeysWithoutAMigrator` (tag RFC3948-2.1-2 positive). No interop run reaches a backend without a migrator: VPP has no IPsec lab (R-6), and the Docker host kernel check refuses a kernel without the message before any scenario starts (R-11). The owed-rekey path row 2 takes is proven on the wire by AC-14's transport scenario |
| AC-14 | as AC-11, but the Child SA is transport mode, whatever the backend | Rebind design row 2, as AC-13. Proof: `TestRFC3948RebindRekeysTransportChildSA` (a migrator is present and is never called; tag RFC3948-2.1-2 positive); interop `real-nat-port-translated-transport-ze-responder` |
| AC-15 | as AC-11, and `MigrateTunnel` returns any error that is not `ErrTunnelMigrationLost` (wrapped in `ErrNotSupported`, or wrapping no sentinel at all, as the port refusal and a restored rollback do) | Rebind design row 3: the old endpoints stand, the error is logged with the peer, and the rekey of AC-13 follows. Proof: `TestRFC3948RebindRekeysWhenMigrationRefused` |
| AC-16 | as AC-11, and `MigrateTunnel` returns `ErrTunnelMigrationLost` | Rebind design row 4: the IKE SA closes as a MOBIKE SA does on the same error; no key is reinstalled and no rekey is started. Proof: `TestRFC3948RebindRetiresALostChildSA` |
| AC-17 | an authenticated message from the stored endpoint; or from a new endpoint while MOBIKE is negotiated; or from a new endpoint while Ze is behind a NAT; or an unauthenticated datagram from a new endpoint | no migration, no rekey, ports unchanged (negative polarity). Proof: `TestRFC3948RebindNotRunWithoutAnAdoptedMove`, plus `TestNattUnauthenticatedPacketDoesNotMoveTheEndpoint` staying green |
| AC-18 | the rekey owed by AC-13 meets a held request window, a TEMPORARY_FAILURE hold or a refusal hold | the rekey is deferred, raised again on a later tick, and runs once the hold ends; the owed flag clears only when a rekeyed Child SA installs, ze- or peer-initiated, or when a pending child is promoted to current; reaping a pending SA that never authenticated leaves it set. Proof: `TestRFC3948RebindRekeyOwedSurvivesAHold`, `TestRFC3948RebindOwedRekeyClearsOnPromotion` |

## End-to-End User Stories

| Story | Path | Test |
|-------|------|------|
| An operator terminates a road-warrior strongSwan client behind a home NAT that rewrites UDP 4500 to a high port; traffic flows both ways | strongSwan → NAT (4500 → 14500) → ze responder → XFRM | `real-nat-port-translated-ze-responder` |
| Same client; the home NAT drops its mapping and re-maps the client to a new port while the tunnel is up; traffic resumes without re-authentication | NAT remap → strongSwan DPD from the new port → ze rebind (migrate or rekey) | rebind step of `real-nat-port-translated-ze-responder`, `real-nat-port-translated-transport-ze-responder` |
| Same, the client authenticates with EAP-TLS | as above, EAP rounds | `real-nat-port-translated-eap-ze-responder` |
| A TLS 1.2-only EAP-TLS client authenticates against a ze built with the patch | strongSwan client `version_max = 1.2` → ze authenticator | `responder-eap-tls12` |
| A contributor builds ze with plain `go build`, then with the overlay | shell → go | the "undefined" failure, then success, as the page describes (AC-20) |

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
| `TestRFC3948ChildSARefusesEncapWithoutAnIKEPort` | same | AC-10 (Q-2 = A′), the no-endpoint case only, one subtest per path: IKE_AUTH, CREATE_CHILD_SA initiated and answered, rekey initiated and answered; tag RFC3948-2.1-2 negative | to write, must fail at HEAD |
| `TestRFC3948ChildSAFollowsANATMappedPeerPort500` | same | AC-10b: authenticated endpoint (A, 500), NAT detected, Ze on its NAT-T socket; both states carry the peer port 500 and Ze's socket port; no refusal; tag RFC3948-2.1-2 positive | to write, must fail at HEAD (ports 4500/4500) |
| `TestRFC3948RebindMigratesTunnelChildSAs` | `internal/component/ike/engine/rfc3948_rebind_test.go` (new) | AC-11: a recording `TunnelMigrator` sees one `MigrateTunnel` per installed pair (current, distinct superseded) with `RemotePort` Y2; no CREATE_CHILD_SA is sent; tag RFC3948-2.1-2 positive | to write, must fail at HEAD (no migration on a non-MOBIKE move) |
| `TestRFC3948RebindRekeysWithoutAMigrator` | same | AC-13: a backend with no `TunnelMigrator`; the next tick sends a Child SA rekey, and the rekeyed pair's `SAParams` carry Y2; tag RFC3948-2.1-2 positive | to write, must fail at HEAD |
| `TestRFC3948RebindRekeysTransportChildSA` | same | AC-14: transport mode with a migrator present; the migrator is never called and the rekey runs | to write, must fail at HEAD |
| `TestRFC3948RebindRekeysWhenMigrationRefused` | same | AC-15, two subtests: the migrator returns an `ErrNotSupported`-wrapped error, and an error that wraps no sentinel (the shape of the local-port refusal); in both the old endpoints stand, the row 3 log line is emitted and the rekey follows | to write, must fail at HEAD |
| `TestRFC3948RebindRetiresALostChildSA` | same | AC-16: the migrator returns `ErrTunnelMigrationLost`; the IKE SA is `StateDead` with reestablish, no rekey, no reinstall | to write, must fail at HEAD |
| `TestRFC3948RebindNotRunWithoutAnAdoptedMove` | same | AC-17, four subtests (same endpoint, MOBIKE, behind a NAT, unauthenticated); tag RFC3948-2.1-2 negative | to write; green at HEAD by design (polarity guard) |
| `TestRFC3948RebindRekeyOwedSurvivesAHold` | same | AC-18: held window, TEMPORARY_FAILURE hold and refusal hold each defer, then the rekey runs; the flag clears on a peer-initiated rekey too | to write, must fail at HEAD |
| `TestRFC3948RebindOwedRekeyClearsOnPromotion` | same | AC-18, two subtests: a move while a pending child is set marks the rekey owed, then promotion clears the flag and no owed rekey is sent on the next tick; a reaped unauthenticated pending SA leaves the flag set and the owed rekey runs | to write, must fail at HEAD |
| `TestRFC5216TLS12FullHandshakeServerHelloSessionID` | `internal/core/eap/rfc5216_session_id_test.go` (new) | AC-2: two subtests (client offers the ticket extension, client offers none) drive a TLS 1.2 full handshake through `tlsMethod`; each reads the ServerHello session_id (32 octets, not the client's) and finds no NewSessionTicket in the server flight; tag RFC5216-2.1.1-4 positive (moved here from the two config-level tags) | to write, must fail at HEAD (length 0) |
| `TestPatchedCryptoTLSServerSessionID` | `internal/le/go/stdpatch/crypto_tls_session_id_test.go` (new) | AC-19: ticket issued → empty id; no ticket → 32 octets; TLS 1.3 → legacy echo unchanged | to write; red when run with an overlay that carries the marker but not the hunk |
| `TestStdpatchRefusesAPatchThatDoesNotApply` | `internal/le/go/stdpatch/stdpatch_test.go` (new) | AC-21: fixture GOROOT with a changed context line; error names version, patch, page; no overlay written | to write |
| `TestStdpatchOverlayIsContentAddressed` | same | AC-21: same inputs answer the same path; a changed GOROOT file answers a new one; the JSON maps absolute GOROOT paths, the marker file included | to write |
| `TestToolchainEnvironmentCarriesTheStdlibOverlay` | `internal/le/go/toolchain/gotoolchain_test.go` | AC-20: `Overrides` holds `-overlay=`; an inherited GOFLAGS (`-mod=vendor`) is kept | to write, must fail at HEAD |
| `TestDiscriminateOverlayKeepsTheStdlibPatch` | `internal/le/rfc/` (beside the `writeOverlay` tests) | AC-20: the observation overlay holds the broken file AND the stdlib entries | to write, must fail at HEAD |

### Integration Tests (Linux, real XFRM)
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC3948InstalledStateCarriesTheIKEPeerPort` | `internal/component/ike/engine/rfc3948_encap_port_integration_linux_test.go` (new, `integration && linux`, pattern of `child_policy_delete_integration_linux_test.go`: `dataplane.Load("xfrm")`) | AC-1: the kernel state's `XFRMA_ENCAP` sport/dport read back over netlink equal (Y, L) inbound and (L, Y) outbound | to write |

### RFC-tagged tests this spec changes (each needs the owner's approval, Q-4)

| Test | Tags it carries | Change | Needed under |
|------|-----------------|--------|--------------|
| `internal/component/ike/engine/child_test.go::TestChildSANATTEncapPorts` | RFC3948-2.1-2 positive, RFC7296-2.23-2 positive | fixture gains an authenticated peer endpoint (A, Y) and a NAT-T socket port; assertions move from the constant 4500 to the IKE SA's ports. The peer port Y and the local NAT-T socket port MUST both differ from 4500 (D1d), or the assertion cannot tell the fix from the constant. The tag claims are rewritten (they cite stale `child.go` line numbers and the false "4500 unconditionally" statement): the RFC7296-2.23-2 claim becomes "UDPEncap is set when NAT is detected", because Section 2.23's "However, if a NAT is detected, both devices MUST use UDP encapsulation for ESP." says nothing about port 4500; the RFC3948-2.1-2 claim ("the Source Port and Destination Port MUST be the same as that used by IKE traffic,") is scoped to creation time, because Q-3 is answered (i) and the after-rebind half is carried by the rebind tests (AC-11, AC-13, AC-14), which also take the RFC3948-2.1-2 tag | every answer |
| `internal/component/ike/engine/rfc7296_natt_test.go::TestEncapNeverRequestedOnPort500` | RFC7296-2.23-9 positive | fixture gains an authenticated endpoint, swept over a 4500 peer and a NAT-mapped peer on 500; the assertion narrows to Ze's OWN encap port (outbound sport, inbound dport), which is never 500, because under Q-2 = A′ the peer side follows a NAT-mapped 500 (AC-10b, AC-10c); the claim is rewritten to say so; comment "The only two writers assign transport.NATTPort" and "PREVENTS: a later change deriving the encapsulation ports from the SA's own port field" become false and are rewritten | Q-2 = A′ |
| `internal/component/ike/engine/rfc7296_natt_bothforms_test.go`: `TestBfmEncapsulatedESPAcceptedWithoutNAT`, `TestBfmEncapsulatedESPSentWhenNATDetected`, `TestBfmBareESPKeptForUnfloatedSA`, `TestBfmBothESPFormsAreReachable`, `TestBfmBothESPFormsReceivedOnOneChildSA` | RFC7296-2.23-10 positive/negative, RFC7296-2.23-11 positive/negative | shared helper `bfmInstall` gains an authenticated endpoint for every floated case; assertions unchanged | Q-2 = A′ |
| `internal/core/eap/rfc5216_tls_config_test.go::TestRFC5216TLSConfigBothRolesInstallWhatTheFlightsCarry` | RFC5216-2.1.1-4 positive (plus enforced 2.1.1-5, 2.1.1-10, 2.1.2-1 tags, not touched) | the RFC5216-2.1.1-4 tag line is WITHDRAWN under every answer to Q-1; "withdrawn or rewritten" is not a choice. Session ticket keys are not a sessionId, so the tag is wrong at HEAD (journal row in `plan/journal/reference-checked-claim-unchecked.md`). Under Q-1 (a) TLS 1.2 is refused and no config-level tag can prove a TLS 1.2 sessionId; under (c) a deviation is never conformant; under (b) the 2.1.1-4 positive moves to a new ServerHello wire test that reads the server-chosen 32-octet session_id off a TLS 1.2 full handshake | every answer to Q-1 |
| `internal/core/eap/rfc5216_tls_config_test.go::TestRFC5216TLSConfigRefusesWhatWouldBreakTheFlights` | RFC5216-2.1.1-4 negative (plus the enforced tags above) | the RFC5216-2.1.1-4 tag line is withdrawn under every answer, for the same reason; under Q-1 (b) the negative moves to the same new wire test | every answer to Q-1 |
| `internal/component/ike/engine/rfc7296_test.go::TestChildSANoNATNoEncap` (UNCHANGED but affected) | RFC7296-2.23-2 negative | no edit planned, but the implementer MUST decide its fate on purpose: it asserts ports == 0 without NAT, so if `newChildSA` fills the ports for every SA (Phase 3) and `installChildSA` no longer zeroes them when UDPEncap is false, it goes red. Its claim "ESP traffic does NOT float" is stronger than Section 2.23 ("Either side can decide whether or not to use UDP encapsulation for ESP irrespective of the choice made by the other side."); if its body changes, the prose presents no-encap-without-NAT as Ze's choice, not the RFC's, and the edit joins this Q-4 list | Q-2 = A and Phase 3's port fill |

Untagged test that changes under every answer: `TestMobikeMappedPortSurvivesChildRekey` (`mobike_test.go`) exercises the rekey copy of `udpLocalPort`/`udpRemotePort` that D1c replaces; its fixture already moves the IKE SA by a MOBIKE update before the rekey, so after D1c its assertions hold because the IKE SA carries the mapped port, not because the retired child did. The implementer re-reads its comments and assertions, rewrites any that name the copy, and keeps it green. Untagged tests that change with Q-2 = A: `TestRekeyKeepsBothESPFormAcceptance` (`rfc7296_rekey_collision_test.go`), and any `mobike_test.go` fixture that installs a Child SA without an endpoint (none found that assert ports beyond MOBIKE's own). Every changed tagged unit is re-recorded with `./le rfc discriminate-record`; records for RFC3948-2.1-2, RFC7296-2.23-2/-9/-10/-11 and RFC5216-2.1.1-4 go stale and their verdicts are re-judged by an agent that did not write the tests (`./le rfc audit-stamp ... mode rejudge`).

#### Owner approval commands

Approved by the owner on 2026-10-13 (Q-4). The gate reads approvals from the committing session's own file (`tmp/commit-rfc-approved-<session>.md`), and a row is used up by the commit that carries its `RFC-approved:` trailer. So the owner runs these at the prompt of the session that implements and commits the edits, and runs a unit's command again if a later commit changes that unit a second time. A unit is `<package directory>.<top-level test function>`; every name was checked to exist at the HEAD named under "Status at HEAD". The gate compares each tagged function's own text: if only `bfmInstall` changes and a `TestBfm*` body stays byte-identical, that unit is not asked for and its row goes unused. The last command applies only if Phase 3 changes `TestChildSANoNATNoEncap`. The new rebind tests (AC-11, AC-13 to AC-18) and the new negative test are new units and need no approval.

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
| peer encap port (from the authenticated endpoint) | 1-65535; 500 is followed when a NAT mapped the peer there (Q-2 = A′, AC-10b) | 65535, 500 and 1 | 0 / no endpoint (refused, AC-10) | N-A (uint16) |
| Ze's own encap port (NAT-T socket) | never 500 (RFC 7296 Section 2.23, AC-10c) | 4500 or the test-port hook's port | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A | - | the `.ci` harness has no port-translating NAT between two daemons; the user path is proven by the interop scenarios and the Linux integration test | - |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `real-nat-port-translated-ze-responder` (new) | `test/interop-ipsec/scenarios/real-nat-port-translated-ze-responder/` | strongSwan 5.9.14 initiator (PSK, tunnel, `dpd_delay` set in `swanctl.conf`) behind the NAT box; nat.conf row maps its UDP 4500 to public port 14500 with SNAT only, no DNAT | AC-1, AC-5 (other scenarios), then the rebind (AC-11, row 1). Phase one: ze's outbound state reads `encap type espinudp sport 4500 dport 14500`; ping across the tunnel both ways; strongSwan's ESP inbound byte counter rises. Rebind step: the checker replaces the NAT box's port SNAT so UDP 4500 maps to public port 14501, a DIFFERENT port, then flushes the NAT box's UDP conntrack entries with `conntrack -D -p udp` (A-8). The flush is needed because iptables NAT rules apply only to the first packet of a flow: strongSwan's NAT keepalives and DPD on its 4500 flow keep the old conntrack entry alive, so without the flush the swapped SNAT never applies. strongSwan's next DPD INFORMATIONAL is then an authenticated IKE message from (public, 14501), which reaches `adoptAuthenticatedEndpoint` (a NAT keepalive never does). The step needs no branch prediction: `Suite.Run` runs the Docker host kernel check before any image build and refuses a kernel on which any enrolled capability, `ipsec-mobike` included, is not present (R-11), so wherever this step runs `xfrm_linux.go` has loaded the migrator and a tunnel Child SA takes row 1. The checker waits for the row 1 log line (Rebind design, "One log line per branch") and fails when any other row's line appears, then asserts ze's outbound state reads dport 14501 on the unchanged SPI (migrated, not rekeyed) and ping works both ways. The scenario's `swanctl.conf` writes `reauth_time = 0s` and `rekey_time = 0` at both connection and child level, explicitly, as the existing mobike-* and responder-*-rekey scenarios do. `reauth_time = 0s` means no parallel IKE_SA_INIT re-authentication is set up during the step: Ze sets a pending child only when a parallel re-initiation authenticates (`finishResponderEstablish`), never for an IKE rekey sent as CREATE_CHILD_SA. The child-level `rekey_time = 0` keeps a Child SA rekey, which would change the SPI, from failing the same-SPI assertion; the connection-level one keeps the run free of IKE rekeys, which leave the child SPIs alone but add exchanges the checker does not expect. Were a parallel IKE SA set up anyway, ze logs the row 2 line with `reason` "a new IKE SA with the peer is being set up" and the step fails naming it, never passing on row 2's evidence. Red at HEAD expected for both phases: ESP to public:4500, then to public:14500, has no conntrack entry and is dropped | to create; revert-rebuild-red walk owed for each phase |
| `real-nat-port-translated-transport-ze-responder` (new) | `test/interop-ipsec/scenarios/real-nat-port-translated-transport-ze-responder/` | strongSwan 5.9.14 initiator (PSK, transport, `dpd_delay` set), same NAT row and rebind step | AC-14, and the owed-rekey path of row 2 on the wire (on the same hosts as every interop-ipsec scenario, R-11): after the remap ze logs the row 2 line with `reason` exactly "transport mode" (the checker compares the attribute, so the pending-IKE-SA reason cannot satisfy it) and never the row 1 line, rekeys the Child SA (new SPI), never migrates, and the new outbound state reads dport 14501; traffic flows both ways | to create; walk owed |
| `real-nat-port-translated-eap-ze-responder` (new) | `test/interop-ipsec/scenarios/real-nat-port-translated-eap-ze-responder/` | strongSwan 5.9.14, EAP-TLS 1.3 client (as `responder-eap-tls13`), same NAT | AC-6 | to create; walk owed |
| `responder-eap-tls12` (new) | `test/interop-ipsec/scenarios/responder-eap-tls12/` | strongSwan 5.9.14 EAP-TLS client at its shipped `version_max = 1.2` | AC-2 goal validation: a TLS 1.2 strongSwan client authenticates against the patched build with TLS 1.2 tickets suppressed. charon accepts an empty and a chosen id alike and is not known to log it (A-5), so this scenario cannot go red on the id: its revert walk is recorded as "no discriminating red reachable from the peer", and the id is proven by the unit test. It is the regression proof that the patch and the ticket suppression keep TLS 1.2 interoperating | to create |
| `real-nat-tunnel-control`, `real-nat-transport-ze-responder`, `real-nat-transport-ze-initiator` (existing) | `test/interop-ipsec/scenarios/` | strongSwan 5.9.14 | AC-5: unchanged 4500 behaviour stays green | existing |

The nat.conf format gains an optional third column (public port for the peer's UDP 4500) read by `readNATConfig`; `natSetupScript` emits, for such a row, a port SNAT for UDP source port 4500 before the address SNAT and NO DNAT. The rebind step is a checker action in `checkers.go`: it replaces that port SNAT with one to a second public port and deletes the NAT box's UDP conntrack entries (A-8). `docs/architecture/testing/interop.md` documents the column and the step.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|------------|---------------------------------|----------|--------------|--------|
| A-1 | After D1b, every production Child SA build runs after `adoptAuthenticatedEndpoint` | read: `handleAuthRequest` adopts before `buildAuthResponse`; `fsm.go` adopts on StateEstablished before `runEstablished` calls `initiatorFirstChildSA`; CREATE_CHILD_SA and rekey run on an established SA | Q-2 (A) refuses a Child SA in production | the three wiring unit tests plus interop | unvalidated |
| A-2 | The NAT-T socket's bound port is the port the kernel's ESP-in-UDP decap socket listens on | `transport/encap_linux.go` sets UDP_ENCAP on the NAT-T socket (read of the file owed at implementation) | D1d picks a port the kernel does not decap on | `TestRFC3948InstalledStateCarriesTheIKEPeerPort` plus a read of `encap_linux.go` | unvalidated |
| A-3 | go1.27.2 crypto/tls leaves the ServerHello session_id empty on every TLS 1.2 full handshake | grep of `handshake_server.go`: the only assignment is in `doResumeHandshake` | D2 is not a defect | `TestRFC5216TLS12FullHandshakeServerHelloSessionID` red at HEAD | validated 2026-10-14 by a scratch probe (`tmp/session/2026-10-13-9d2e6c21-74c1-4c42-b9ef-30145b95cfb1/scratch/overlay-probe/`): go1.27.2, TLS 1.2 full handshake, ServerHello session_id length 0 with tickets on and off; the unit test still proves it in the tree |
| A-4 | strongSwan 5.9.14 accepts ESP-in-UDP from ze's 4500 arriving through the NAT's conntrack reverse mapping | normal NAT-T operation; existing real-nat scenarios pass with an address NAT | the scenario fails for a lab reason | the scenario's green run | unvalidated |
| A-5 | charon does not log the TLS session_id it receives | not verified in strongSwan source | a stronger interop assertion is available for D2 | read `tls_peer.c` `process_server_hello` at implementation | unvalidated |
| A-6 | iptables `SNAT --to-source addr:port` for `-p udp --sport 4500` translates the port deterministically in the alpine NAT box | iptables semantics | the scenario cannot force port Y, or the rebind step's second port | the checker reads the port ze observed (ze log of the adopted endpoint) before asserting, and asserts it differs from the first | unvalidated |
| A-7 | strongSwan migrates every Child SA on a NAT rebind and rekeys one whose update is unsupported (`ike_sa.c::update_hosts` → `child_sa->update`, `rekey_child_sa` on `NOT_SUPPORTED`) | owner's answer to Q-3 and design research 2026-10-13; no strongSwan source is checked out on this host | the fallback rule differs from charon's; the owner is told, the design stays Ze's | read `src/libcharon/sa/ike_sa.c` and `child_sa.c` of strongSwan 5.9.14 at implementation | unvalidated |
| A-8 | the NAT box image does not carry a conntrack tool today, so this spec adds one: `test/interop-ipsec/Dockerfile.nat` is `FROM alpine:3.21` and runs `apk add --no-cache iproute2 iptables tini`, with no conntrack-tools; the spec adds `conntrack-tools` to that `apk add` so the rebind step can run `conntrack -D -p udp` | read of `test/interop-ipsec/Dockerfile.nat` at HEAD, 2026-10-14 | without the flush, the old UDP conntrack entry (kept alive by strongSwan's keepalives and DPD) keeps the old translation and the step forces no remap | the rebind step's first run shows the swapped port in ze's row log line | fact (the image half); the tool's behaviour in the box is validated by the step's first run |
| A-9 | strongSwan's `float_ports` (`ike_sa.c`) does not rewrite a remote port 500 on the responder side, so a NAT-mapped peer 500 reaches charon's Child SA install as 500 | owner context for Q-2 = A′; not read in strongSwan source | AC-10b still holds for Ze; only the claim that strongSwan behaves the same changes | read `ike_sa.c` `float_ports` of 5.9.14 at implementation | unvalidated |
| A-10 | Every tool that type-checks ze honours `-overlay` in GOFLAGS: `go vet`, golangci-lint (go/packages), govulncheck, gopls | golangci-lint and gopls load through go/packages, which runs `go list` under GOFLAGS; not run against an overlay that adds a file | a tool reports `undefined: tls.ZeServerChoosesSessionID` on a patched setup | run each once under the overlay at implementation (D2 phase) | unvalidated |
| A-11 | `go build -overlay` adds a NEW file to a standard-library package, and a command-line `-overlay` replaces the one in GOFLAGS (one overlay per command) | scratch probe 2026-10-14 (path in A-3): a program using the added constant built under the overlay and failed with "undefined" without it; `GOFLAGS=-overlay=A go run -overlay=B` built without A's file | the marker guard or the discrimination merge is wrong | probe | validated 2026-10-14 |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|-----------------------|
| R-1 | The NAT box's DNAT forwards ESP sent to public:4500 to the peer's 4500, masking D1 so the scenario is green at HEAD | the revert-rebuild walk stays green | port-translated rows carry no DNAT; the walk's red is recorded before the claim |
| R-2 | Q-2 (A′) refuses a Child SA on a path that never adopted an endpoint | childless IKE SA, the refusal error in the log, an interop scenario red | A-1's enumeration; the error names the peer and the missing port |
| R-3 | A NAT maps the peer to source port 500 on Ze's NAT-T socket: RFC 3948 Section 2.1 asks for 500, and RFC 7296 Section 2.23 says "UDP encapsulation MUST NOT be done on port 500." | a peer whose ESP never arrives while IKE is up | RESOLVED by Q-2 = A′ (owner, 2026-10-14): the 2.23 sentence is read as binding the ports the endpoints send from (same paragraph: "MUST send all subsequent traffic from port 4500,"), and 2.23 also requires "responses MUST be sent to the port from whence they came."; so Ze follows the NAT-mapped 500 (AC-10b) and keeps its own port off 500 (AC-10c). The reading is stated in the `rfc/short/rfc7296.md` RFC7296-2.23-9 row note and the `rfc/short/rfc3948.md` RFC3948-2.1-2 row note in the D1 phase. Residual: A-9 |
| R-4 | The rebind adds a kernel migration on a non-MOBIKE path and a rekey trigger outside the lifetime timer | ESP counters stop while IKE DPD succeeds; a rekey storm in the log | migration reuses the MOBIKE loop and its rollback contract; the owed rekey is a single flag cleared by the next installed rekey, so one move costs at most one rekey (AC-18) |
| R-5 | The `AcceptBothESPForms` side receiver keys on the encap ports; with port Y it might not match | bare-ESP arm of the both-forms tests red | `TestRFC3948InstalledStateCarriesTheIKEPeerPort` also sends bare ESP; existing `encap_dualform_integration_linux_test.go` re-run |
| R-6 | The VPP backend carries ports with no lab to prove them | none (no VPP lab) | unit assertion on `SAParams`; stated as the unproven part in the summary |
| R-7 | `spec-ike-responder-eap-preserves-state-on-unauthenticated-input` edits `handleResponderEAP` concurrently | merge conflict in `responder_eap.go` | D1b touches only the AUTH-from-MSK branch; whichever lands second rebases its hunk |
| R-8 | The overlay patch reaches every ze TLS 1.2 server, not only EAP-TLS | a ze TLS client or peer that misbehaves on a non-empty id with no ticket | the hunk fires only when no ticket is issued (D2 design, "Reach"); `TestPatchedCryptoTLSServerSessionID` pins all three server rows |
| R-12 | A Go command path that compiles ze is missed by the overlay wiring | `undefined: tls.ZeServerChoosesSessionID` in that build | that error IS the mitigation: the marker turns a missed path into a failed build, never a silent unpatched binary; the fix is to add the path to D2 design "Reach" |
| R-13 | A toolchain bump moves `doFullHandshake` | `stdpatch` refuses | the bump procedure on the page; upstream adoption deletes the patch (`ai/rules/no-layering.md`) |
| R-9 | Changing the tagged fixtures stales discrimination records and enforced verdicts of RFC7296-2.23-2/-9/-10/-11 | `./le rfc check` refusals | re-record every staled unit in the same change; independent re-judge |
| R-11 | No interop-ipsec scenario runs on this build host. `Suite.Run` (`internal/le/interoplab/lab.go`) runs `DockerKernel` before any image build, and `checkDockerKernel` (`kernelcheck.go`) refuses the run when any enrolled capability is not `present`; on 6.8 the check refuses `ipsec-mobike` (`docs/architecture/testing/interop.md`: the kernel check is under "The Docker host kernel check", and the requirement under "IPsec address movement": "The Docker host must support `XFRM_MSG_MIGRATE_STATE`"). Every interop proof in this spec, D1's included, therefore comes from a Docker daemon kernel with every feature Ze enrolls: the Docker lab's QEMU guest on the Ze kernel (`./le test qemu docker-lab`, `gokrazy/kernel/runtime.config` sets `CONFIG_XFRM_MIGRATE=y`) or a Linux host booted on the kernel `./le setup docker-kernel install` installs | the run is refused with the kernel check's message, never run on the wrong branch | on an accepted host the tunnel step proves row 1 (migrate) and the transport scenario proves row 2 (owed rekey, transport mode); each checker asserts its scenario's fixed row, so neither passes on the other's evidence. Row 2's no-migrator tunnel case (AC-13), row 3 (AC-15) and row 4 (AC-16) are proven by unit tests only, since no accepted host lacks the migrator and neither failure can be provoked from a peer |
| R-10 | Deriving the local port from the socket changes `.ci` IPsec tests that run under the test-port hook | `test/ipsec` red | the change makes the encap port equal the IKE port those tests already use; investigate each red under `ai/rules/pre-release.md` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | NAT-traversing tunnels carry no ESP (IKE stays up); with Q-2 (A′) a mis-ordered path yields a childless IKE SA |
| How is it reverted? | single commit revert per phase |
| Who else touches this path? | `spec-ike-responder-eap-preserves-state-on-unauthenticated-input` (`responder_eap.go`), `spec-eap-tls-identity-authorization` (`eap_tls.go`), MOBIKE code (`mobike.go`: its migration loop and predicate become shared with the rebind, MOBIKE behaviour unchanged, every `mobike_test.go` unit stays green) |

## Files to Modify

- the two producers and their tests
- `internal/component/ike/engine/create_child.go` - new `childEncapPorts(sa)`: the ONE port derivation (remote from `sa.peerEndpoint`, local from the NAT-T socket; under MOBIKE from `sa.mobike.local` and `remoteUDPAddr` as today) and the ONE place the Q-2 (A′) refusal (no authenticated endpoint only; a NAT-mapped peer port 500 is followed) lives, with the RFC 3948 Section 2.1 quote above it; `newChildSA` calls it for every Child SA (IKE_AUTH and CREATE_CHILD_SA), callers carry `// RFC 3948 Section 2.1`
- `internal/component/ike/engine/child.go` - `installChildSA`: drop the 4500 default and read the child's ports as built; it refuses nothing (the refusal is in `childEncapPorts`); rewrite the `UDPEncap` and `udpLocalPort` field comments and the port comments that state "Both ports below are the NAT-T constant"
- `internal/component/ike/engine/responder_eap.go` - `handleResponderEAP`: adopt the endpoint after `verifyRemoteAuth`, before `buildAuthResponse`
- `internal/component/ike/engine/responder.go` - `replayCachedResponse` comment on `peerEndpoint` during EAP
- `internal/component/ike/engine/rekey.go` - `newRekeyedChild` takes the IKE SA and its ports from `childEncapPorts` (both rekey directions), so a refusal surfaces there too; `applyChildRekeyResponse` and `respondChildRekey` clear `ps.rebindRekeyOwed` through their callers when the rekeyed pair installs
- `internal/component/ike/engine/mobike.go` - `childMigrationAvailable` extracted from `mobikeAvailable`; `migrateMobikeChild` / `migrateMobikePair` take the local endpoint as a parameter (MOBIKE passes `sa.mobike.local`, the rebind the NAT-T socket's address), so MOBIKE and the rebind share one migration loop
- `internal/component/ike/engine/sa.go` - `adoptAuthenticatedEndpoint` reports whether it moved the endpoint; its doc comment says so
- `internal/component/ike/engine/inbound.go` - `handleOwnedInbound` calls `rebindChildSAs` when the adoption reports a move
- `internal/component/ike/engine/established.go` - the owner tick raises `startChildRekey` while `ps.rebindRekeyOwed && ps.pendingRekey == nil`
- `internal/component/ike/engine/reconcile.go` - `PeerSession` gains `rebindRekeyOwed`
- `internal/core/eap/eap_tls.go` - `newTLSMethod`: `GetConfigForClient` (TLS 1.2 ticket suppression) and the marker reference, each with the page and RFC 5216 Section 2.1.1 comment; the "the ticket above goes out either way" comment rewritten for TLS 1.2 (D2 design)
- `internal/le/go/toolchain/gotoolchain.go` - `Toolchain.Overrides` appends the `stdpatch` overlay to GOFLAGS; the package doc's variable list names it
- `le` (launcher script, `build_le`) - runs the `stdpatch` generator and builds with its overlay
- `internal/le/rfc/discriminate_observe.go`, `discriminate_guest.go` - `writeOverlay`, `setGoFlagsOverlay` and `environment` merge the stdlib entries
- `internal/appliance/cmd_build.go` - `SetGokGoEnv` composes the overlay into GOFLAGS; its doc list gains the item
- `internal/le/repo/bootstraps/tracked.go`, `internal/le/repo/compiles/repositorytrackedbuild.go`, `internal/le/repo/changed/selector.go`, `internal/le/go/module/move.go`, `internal/le/setup/proto_generate.go`, `internal/le/test/qemu/run.go`, `internal/le/test/qemu/run_exec.go` - compose the overlay, or a comment saying the site compiles nothing that links `internal/core/eap` (D2 design, "Reach")
- `docker/Dockerfile`, `docker/Dockerfile.lab` - generator `RUN` before the ze build
- `.github/workflows/codeql.yml`, `.github/workflows/qemu-nightly.yml`, `.github/workflows/govulncheck.yml` - GOFLAGS step before the first go command
- `docs/contributing/patching-the-standard-library.md` - after implementation, restate the mechanism as built (AC-24)
- `docs/contributing/running-commands.md` - `./le go stdpatch overlay` and the GOFLAGS the toolchain environment now carries
- tagged and untagged tests listed in the TDD plan
- `internal/le/interoplab/ipsec/nat.go`, `internal/le/interoplab/ipsec/checkers.go` - port column, the rebind step (port SNAT swap, conntrack delete, the scenario's fixed row log line match), four checkers
- `test/interop-ipsec/Dockerfile.nat` - add `conntrack-tools` to the `apk add` line, and a header comment line saying the rebind step flushes UDP conntrack entries with it (A-8)
- `internal/component/ike/engine/fsm.go` - `resolvePendingAfterOwnerLoop` clears `ps.rebindRekeyOwed` when it promotes a pending child (Rebind design, "Owed flag and promotion")
- `rfc/short/rfc3948.md`, `rfc/short/rfc5216.md` (RFC5216-2.1.1-4: the overlay patch and the TLS 1.2 ticket suppression, page named), `rfc/short/rfc7296.md` (RFC7296-2.23-9 row note: the ban binds Ze's own port, a NAT-mapped peer 500 is followed, Q-2 = A′ with both RFC 7296 Section 2.23 sentences quoted), `rfc/discrimination/rfc3948.json`, `rfc7296.json`, `rfc5216.json`, `rfc/audit/*` via `./le rfc audit-stamp`
- `docs/guide/ipsec.md`, `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md`, `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`, `docs/architecture/ike/ipsec-14-responder.md` (EAP adoption order), `docs/architecture/testing/interop.md` (nat.conf port column)

## Files to Create

- `internal/component/ike/engine/rfc3948_encap_port_test.go`
- `internal/component/ike/engine/rfc3948_encap_port_integration_linux_test.go`
- `internal/component/ike/engine/rebind.go` - `rebindChildSAs` (Rebind design)
- `internal/component/ike/engine/rfc3948_rebind_test.go`
- `test/interop-ipsec/scenarios/real-nat-port-translated-transport-ze-responder/` (`nat.conf`, `swanctl.conf`, `ze.conf`)
- `internal/core/eap/rfc5216_session_id_test.go`
- `test/interop-ipsec/scenarios/real-nat-port-translated-ze-responder/` (`nat.conf`, `swanctl.conf`, `ze.conf`)
- `test/interop-ipsec/scenarios/real-nat-port-translated-eap-ze-responder/` (plus `pki/`, `strongswan.conf`)
- `test/interop-ipsec/scenarios/responder-eap-tls12/`
- `internal/le/go/stdpatch/stdpatch.go` - the overlay generator (D2 design, steps 1 to 5); standard-library imports only
- `internal/le/go/stdpatch/patches/crypto-tls-server-session-id.patch` - the patch, with the hunk of step 1 and the marker file of step 2
- `internal/le/go/stdpatch/overlay/main.go` - the bootstrap entry the `le` script runs with `go run`
- `internal/le/go/stdpatch/register.go` (or the le command registration the `go` group uses) - `./le go stdpatch overlay`
- `internal/le/go/stdpatch/stdpatch_test.go`, `internal/le/go/stdpatch/crypto_tls_session_id_test.go`
- `docs/contributing/patching-the-standard-library.md` - written in the design phase (2026-10-14), states the mechanism is not built yet

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no config change |
| YANG validation constraints | N-A | no leaf |
| YANG custom validators | N-A | no leaf |
| CLI commands/flags | Yes (le only) | `./le go stdpatch overlay` (`ai/rules/cli.md`); `show ipsec` output keys unchanged |
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
| 3 | CLI command added/changed? | Yes (le) | `docs/contributing/running-commands.md`: `./le go stdpatch overlay` |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/guide/ipsec.md` |
| 7 | Wire format changed? | No | the UDP header layout is unchanged; only the port values |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc3948.md`, `rfc/short/rfc5216.md`; `docs/features/rfc-status.md` regenerated if a Support row changes |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` (nat.conf port column); `docs/contributing/patching-the-standard-library.md` (new, design phase: the technique, stated as not built; restated as built at implementation, AC-24); `docs/contributing/README.md` and `ai/INDEX.md` rows for it (design phase) |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md`, `ipsec-8-ikev2-child-xfrm.md`, `ipsec-14-responder.md`; D2: `docs/architecture/core-design.md` (the toolchain environment carries the overlay), `docs/architecture/testing/tracked-build-gate.md` (the explicit `-overlay` argument), `docs/architecture/appliance/builder.md` (gok GOFLAGS), `docs/architecture/ike/ipsec-11-interop-eap.md` and `docs/guide/ipsec.md` (TLS 1.2: server-chosen session id, no TLS 1.2 ticket) |
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
4. **Phase: rebind** (Q-3 answered (i)) -- `adoptAuthenticatedEndpoint` reports a move, `rebindChildSAs` with its four rows, the shared MOBIKE loop and predicate, the owed-rekey trigger; the AC-11, AC-13 to AC-18 tests red first, then green; every `mobike_test.go` unit stays green
5. **Phase: interop** -- nat.conf port column and the rebind step, four scenarios, revert-rebuild-red walk recorded for each (for the two rebind steps, reverting the rebind phase alone)
6. **Phase: D2** (Q-1 answered (b)) -- (i) `stdpatch` with its two tests and the patch file, `TestPatchedCryptoTLSServerSessionID` red with a marker-only overlay, green with the patch; (ii) `gotoolchain` overlay and the discrimination merge, their tests red first; (iii) the marker reference and `GetConfigForClient` in `newTLSMethod`, `TestRFC5216TLS12FullHandshakeServerHelloSessionID` red at HEAD then green; (iv) every other site of D2 design "Reach" (launcher, gokrazy, QEMU guest, Dockerfiles, CI), each built once; A-10 checked; (v) `responder-eap-tls12`; (vi) the page restated as built
7. **Phase: RFC records** -- `./le rfc discriminate-record` for every new or changed tagged unit; summary rows; independent `./le rfc audit-stamp ... mode rejudge` for RFC3948-2.1-2, RFC5216-2.1.1-4 and every staled verdict
8. **Phase: docs** -- each page edited in the phase that changes its behaviour; this step only verifies

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N maps to a file and test |
| Correctness | send form still follows NATDetected only; Ze's own encap port never 500, a NAT-mapped peer 500 followed (Q-2 = A′); MOBIKE behaviour unchanged; a rebind never reinstalls keys after `ErrTunnelMigrationLost` |
| Data flow | ports come only from the IKE SA's authenticated endpoint and its NAT-T socket |
| Rule: stale-comments | the `UDPEncap`, `udpLocalPort` and port comments in `child.go` and `responder.go` restated |
| Rule: rfc-compliance | RFC 3948 Section 2.1 quote above the derivation; callers carry `// RFC 3948 Section 2.1`; the RFC 5216 Section 2.1.1 quote and page pointer on the five places of D2 design "Comments the code owes" |
| D2 reach | every row of D2 design "Reach" built once with the overlay; no site replaces an inherited GOFLAGS |

### Security Review Checklist
| Check | What to look for |
|-------|------------------|
| Input validation | the port comes from an AUTHENTICATED datagram only; an unauthenticated source never reaches `peerEndpoint` (`TestNattUnauthenticatedPacketDoesNotMoveTheEndpoint` stays green) |
| Amplification | moving the EAP adoption earlier must not change `replayCachedResponse`'s destination rule |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Take Child SA encap ports from the IKE SA (authenticated endpoint, NAT-T socket) for every Child SA | keep a 4500 fallback; refuse a NAT-mapped peer port 500; let the kernel learn the port from `XFRM_MSG_MAPPING` events | owner answer Q-2 = A′ (2026-10-14). Mapping events need the peer to send ESP first, have no VPP equivalent and add a netlink listener; the fallback is the silent default that hid D1b; refusing a remote 500 rejects a connection strongSwan handles, and RFC 7296 Section 2.23's ban binds Ze's own port (R-3). The derivation extends the MOBIKE one already in `newChildSA` |
| Fix the EAP adoption order rather than read the datagram source at Child SA build | pass `remote` into `buildAuthResponse` | adoption is the single writer of the authenticated endpoint; a second source would be a second declaration |
| D2: patch `crypto/tls` at build time with `go build -overlay`, one hunk in `doFullHandshake` that chooses a 32-octet id when no ticket is issued, plus TLS 1.2 ticket suppression in the authenticator | (a) TLS 1.3 only; (c) recorded deviation; fork the toolchain; a copy of `crypto/tls` in the tree; mirror the unexported handshake state with `unsafe`; a per-`Config` opt-in field | owner answer Q-1 (b), 2026-10-14, with the overlay technique he named. A fork or a copy must be rebased by hand at every bump and drifts silently; `unsafe` mirroring cannot reach a local of `doFullHandshake` at all; the opt-in doubles the patch for no consumer (D2 design). The patch is stored as a diff applied to the current GOROOT, so a bump refuses loudly rather than reverting it |
| D2 guard: compile-time marker, plus wire tests, plus refusal on bump | test-only; runtime self-probe at `newTLSMethod` | a test proves its own build only; a self-probe ships a broken binary to the operator. The marker costs plain `go` and gopls an overlay setting, documented on the page |
| Rebind: migrate where a `TunnelMigrator` can move the pair, else rekey that Child SA onto the new ports (Rebind design) | always rekey; always migrate; `XFRM_MSG_MAPPING` events | the owner's answer, "migrate it (strongSwan)", and charon's fallback (A-7); always-rekey discards a live replay window the kernel could keep; always-migrate strands transport mode, VPP and older kernels; mapping events see ESP only, need a netlink listener and have no VPP equivalent |

## Known Limitations
- VPP: ports reach `SAParams` but no lab proves the VPP SA (R-6); a VPP rebind takes the rekey branch, proven by unit test only
- No IKE interop evidence comes from this build host (Linux 6.8): the Docker host kernel check refuses every interop-ipsec run there, so the interop proofs run in the Docker lab's QEMU guest or on a host with a kernel carrying every enrolled feature, `XFRM_MSG_MIGRATE_STATE` included (R-11)
- Rebind rows 3 and 4, and row 2 for a tunnel Child SA without a migrator, are proven by unit tests only (R-11)

## Design Review Log

| Round | Date | Findings | State |
|-------|------|----------|-------|
| 1 | 2026-10-14 | 0 BLOCKER, 2 ISSUE, 5 NIT: AC-3 branched on Q-1; the rebind designed for every Child SA (migrate or rekey, `rebindChildSAs`, AC-11, AC-13 to AC-18, interop rebind step with a changed port and DPD); stale Q-3 conditionals removed; the Q-2 refusal placed in `childEncapPorts` and AC-10 extended to CREATE_CHILD_SA and rekey; one HEAD; the D2 table repeat of Q-1 dropped; RFC 3948 Section 5.2 marked illustration; `TestMobikeMappedPortSurvivesChildRekey` listed | applied |
| 2 | 2026-10-14 | 0 BLOCKER, 2 ISSUE, 2 NIT: rebind row 3 and AC-15 now cover any `MigrateTunnel` error that is not `ErrTunnelMigrationLost` (the port refusal and a restored rollback wrap no sentinel), the `validateXFRMMigration` constraint corrected, `TestRFC3948RebindRekeysWhenMigrationRefused` gains an unwrapped-error subtest; A-8 restated as a fact and `conntrack-tools` added to `Dockerfile.nat`, with the reason for the flush; one log line per rebind row and the checker's prediction from `ze doctor`'s `ipsec-mobike` answer; the owed flag clears on promotion, `TestRFC3948RebindOwedRekeyClearsOnPromotion` | applied |
| 3 | 2026-10-14 | 0 BLOCKER, 1 ISSUE, 3 NIT: R-11 rewritten, since `Suite.Run`'s Docker kernel check refuses any host lacking an enrolled capability, so no interop-ipsec scenario runs on this 6.8 host and every interop proof comes from the Docker lab's QEMU guest or a host on a kernel with every enrolled feature; the checker's `ze doctor` branch prediction dropped (its "absent" arm was unreachable), the tunnel step asserts row 1 and the transport scenario row 2; AC-11, AC-13, AC-14 and Known Limitations follow (AC-13, rows 3 and 4 by unit test only). The spec has no Goal Validation table; every End-to-End story stays provable on those hosts. Row 2's pending-child `reason` reworded and the transport checker compares it exactly; "If this repeats, run: ze doctor" dropped from row 3; the owed flag left over by an SA death and the `reapStalePending` case stated | applied |
| 4 | 2026-10-14 | 0 BLOCKER, 1 ISSUE, 2 NIT (`tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-ike-eap-design-r4.md`), applied by the main thread: the tunnel rebind row names connection-level `reauth_time = 0` as the guard against a parallel IKE SA (pending child), and keeps connection- and child-level `rekey_time` above the run time for the same-SPI assertion; R-11 cites the `XFRM_MSG_MIGRATE_STATE` quote under "IPsec address movement" | applied |
| 5 | 2026-10-14 | 0 BLOCKER, 0 ISSUE, 2 NIT (`tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-ike-eap-design-r5.md`), applied by the main thread: the scenario sets `reauth_time = 0s` and `rekey_time = 0` explicitly at both levels like the existing rekey scenarios, each with its own reason. Design converged; Q-1 and Q-2 stay with the owner | converged |

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (before the fix)
- [ ] Tests PASS (after the fix)

### Verification
- [ ] `./le verify worktree`
