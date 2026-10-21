# Spec: ike-eap-rfc-defects

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 6/8 (D2: code and pages landed; runs owed, see step 6 status) |
| Handoff | - |
| Updated | 2026-10-21 |

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

### Owner questions (2026-10-13, design phase; Q-1 to Q-4 are answered: Q-3 and Q-4 by the owner on 2026-10-13, Q-1 and Q-2 by the owner on 2026-10-14; Q-5, raised by the D2 design review round 1, decided (A) by the main thread on 2026-10-14 under the owner's direction)

| Q | Question | Options | Recommendation and why | Answer |
|---|----------|---------|------------------------|--------|
| Q-1 | D2: what does Ze's EAP-TLS authenticator do about RFC 5216 Section 2.1.1 "the server MUST choose the sessionId to establish a new session" on a TLS 1.2 full handshake? | (a) Authenticator negotiates TLS 1.3 only (`MinVersion` 1.3); a TLS 1.2 peer is refused with a TLS alert. (b) Ze carries a patched copy of `crypto/tls` whose TLS 1.2 full handshake writes a fresh 32-octet server-chosen session_id. (c) Owner-approved deviation: Ze keeps the empty session_id, recorded with RFC 5246 Section 7.4.1.3 ("The server may return an empty session_id to indicate that the session will not be cached and therefore cannot be resumed.") and the reason; a deviation is never counted conformant, so RFC5216-2.1.1-4 leaves `wrong` for a recorded deviation, not for `enforced` | (c), stated with its cost. (a) refuses every strongSwan client left at its shipped default: `test/interop-ipsec/scenarios/responder-eap-tls13/strongswan.conf` records that charon ships `version_max = 1.2`. (b) forks a standard-library package whose TLS 1.2 server also issues a ticket ze never redeems (since `37dbe54095`), so the chosen id would identify a session ze refuses to resume. (c) is honest about what the wire carries. The ticket argument the two existing RFC5216-2.1.1-4 tags make does NOT hold any more: since `37dbe54095` a TLS 1.2 ticket is never redeemed, so "a new session can be established under an identity it chose" names an identity that is never usable | **answered by the owner, 2026-10-14: (b)**. Owner's words: "b - please look at [https://vincent.bernat.ch/en/blog/2026-go-netip-addrto6] on how we should patch the library", then "document this technique in the repository and document it in the spec and later code". The library is patched at build time with `go build -overlay`, never by a fork of the toolchain or a copy of `crypto/tls` in the tree. The ticket objection in the recommendation is removed by the design: the authenticator issues no TLS 1.2 ticket, so the chosen id names the only session there is, one that is never resumed, which RFC 5246 Section 7.4.1.3 allows ("Note that there is no requirement that the server resume any session even if it had formerly provided a session_id."). Designed under "D2 design"; the technique is documented in `docs/contributing/patching-the-standard-library.md`. The delivery by `-overlay` is superseded by Q-5 (A): the same diff, applied at build time, reaches the tools through a patched GOROOT |
| Q-2 | D1: when a Child SA needs UDP encapsulation and the IKE SA has no authenticated peer endpoint (or the peer's port is 500), what does `childEncapPorts` do? It is the one port derivation every Child SA passes through (IKE_AUTH and CREATE_CHILD_SA through `newChildSA`, either rekey direction through `newRekeyedChild`, the rebind through `rebindChildSAs`; Files to Modify) | (A) Refuse the Child SA with an error naming the peer and the missing port; nothing installed. (B) Keep a fallback to port 4500 | (A). `ai/rules/principles.md`: code that cannot answer must say so. The fallback is exactly what hid D1b: the EAP path built its Child SA with no endpoint and nothing noticed. After D1b's reorder no production path reaches the refusal, so (A) costs nothing at runtime; it costs test fixtures (Q-4) | **answered by the owner, 2026-10-14: "revised A" (A′)**, replacing a first answer of plain (A) the same day. (1) `childEncapPorts` refuses the Child SA, with an error naming the peer and the reason and nothing installed, ONLY when the IKE SA has no authenticated peer endpoint; there is no 4500 fallback. (2) A peer endpoint port of 500 seen through a NAT, on Ze's 4500 NAT-T socket, is NOT refused: the Child SA follows it like any other port. (3) Ze's OWN local port is never 500 for encapsulation (preserved: "port 500 never requested"). (4) Main-thread decision 2026-10-14 under the owner's delegation, same principle as A′ (never guess a port): a Child SA that needs encapsulation while the IKE SA has NO NAT-T socket is refused the same way, with no 4500 fallback for the local port (`localEncapPort`, `create_child.go`). The reading behind (2): RFC 7296 Section 2.23 "UDP encapsulation MUST NOT be done on port 500." binds the ports the endpoints send from, as the same paragraph's "An IPsec endpoint that discovers a NAT between it and its correspondent (as described below) MUST send all subsequent traffic from port 4500," shows, and the section also says responses "MUST be sent to the port from whence they came."; a NAT's external port is chosen by neither endpoint. Context the owner was given: strongSwan (`child_sa.c` `install_internal`, `ike_sa.c` `float_ports` / `update_hosts`) takes the Child SA ports from the IKE SA hosts (the `other_host` port), floats off 500 to 4500 on NAT detection, and has no fallback, so refusing a remote 500 would reject a connection strongSwan handles (the owner's objection); the no-endpoint state exists only in Ze. NOT verified: whether strongSwan's `float_ports` rewrites a remote 500 on the responder side (A-9) |
| Q-3 | A NAT rebinding AFTER establishment: `adoptAuthenticatedEndpoint` adopts a new peer port (no MOBIKE, Ze not behind a NAT), but installed Child SA states keep the old encap port, so ESP again leaves on a port IKE no longer uses. In this spec, or a journal row? | (i) In scope: migrate the installed states' encap ports when the IKE SA adopts a new endpoint, as `migrateMobikePair` does for MOBIKE. (ii) Journal row in `plan/journal/`, this spec fixes the ports at Child SA creation only | Owner's call: the MUST reads as a standing condition, so (ii) leaves RFC3948-2.1-2 violated after a rebind and the verdict could only be `enforced` for the creation-time claim. (i) adds a kernel migration path with its own failure modes (R-4) | **answered by the owner, 2026-10-13: (i), "migrate it (strongSwan)"**. strongSwan migrates every Child SA through `update_hosts` / `child_sa->update`, rekeying where the update is unsupported (research 2026-10-13; no strongSwan source is checked out on this host, so that behaviour is assumption A-7, verified at implementation). Designed under "Rebind design"; AC-11 and AC-13 to AC-18 are in scope |
| Q-4 | Approval to change the RFC-tagged tests listed under "RFC-tagged tests this spec changes" (TDD Test Plan) | approve / approve a subset / refuse | Needed under any answer to Q-1 and Q-2: the RFC3948-2.1-2 tag on `TestChildSANATTEncapPorts` asserts the defect. Units: `TestChildSANATTEncapPorts` (RFC3948-2.1-2, RFC7296-2.23-2: ports differ from 4500 in the fixture, 2.23-2 claim narrowed to "UDPEncap set when NAT is detected"), `TestEncapNeverRequestedOnPort500` (RFC7296-2.23-9), the five `TestBfm*` units (RFC7296-2.23-10/-11, helper only), and the two `rfc5216_tls_config_test.go` units (RFC5216-2.1.1-4 line withdrawn under every Q-1 answer). `TestChildSANoNATNoEncap` (RFC7296-2.23-2 negative) joins the list if Phase 3 changes its body (RFC-tagged test review, 2026-10-13) | **answered by the owner, 2026-10-13: "approve all RFC-tagged test changes"**. The approval covers every unit listed under "RFC-tagged tests this spec changes", including `TestChildSANoNATNoEncap` if Phase 3 changes its body. With Q-1 answered (b) and Q-2 answered (A′) on 2026-10-14, every listed edit applies: `TestEncapNeverRequestedOnPort500` and the five `TestBfm*` units change for (A′), and the RFC5216-2.1.1-4 lines move to the new wire test for (b). The commands are under "Owner approval commands" in the TDD plan |
| Q-5 | D2: how is the patched `crypto/tls` put in front of every Go tool? The A-10 probe (2026-10-14, D2 design "A-10 probe") shows that `go build`/`go vet`/`go list` honour a GOFLAGS `-overlay` that ADDS a file under GOROOT, but golangci-lint, staticcheck and govulncheck stop with "open .../crypto/tls/zz_ze_server_session_id.go: no such file or directory", and gopls reports the marker undefined: they load through go/packages and read the source from disk. `./le go lint` and `./le verify deps vulnerability` (CI `govulncheck.yml`) would refuse every package linking `internal/core/eap` | (A) A patched GOROOT: a link tree under `cache/stdpatch/<key>/goroot` whose entries are symlinks into the real GOROOT except the patched and the added file, selected with `GOROOT=` (and `GOTOOLCHAIN=local`) by the toolchain environment; the overlay is not used. (B) Keep `go build -overlay`, the article's technique, and drop the compile-time marker (the added GOROOT file is what the disk-reading tools cannot open); an unpatched build then compiles and ships silently, the guard falls to the tests alone, which "Loud failure" rejected. (C) Keep the overlay and the marker, and keep the disk-reading tools off every package linking `internal/core/eap`: not viable, `./le go lint` is a commit gate and govulncheck a CI gate | **(A)**, said plainly: it replaces the overlay the owner named, so it is the owner's call. The probe ran (A) as a 52 KiB symlink tree on go1.27.2: `go env GOROOT`, `go vet`, `go run` (TLS 1.2 no-ticket handshake: 32-octet id; with a ticket: empty id), golangci-lint, staticcheck and govulncheck all passed with the marker referenced. It keeps every part of the article's idea that matters (the standard library changed at build time from a diff applied to the toolchain in use, no fork, no copy in the tree, refusal on a bump) and changes only the delivery: a path every tool reads from disk instead of a JSON map only the go command reads. It also removes B-1 at its root (one `-overlay` per command stops mattering: discrimination keeps its own overlay untouched) and gives `gotoolchain.Overrides` a plain variable it SETS. gopls reported the marker undefined under (A) as well (D2 design "A-10 probe"), so gopls stays unsolved by both options and is a documented limit. Until the owner answers, the D2 design below is written for the overlay with the round-1 fixes, and "Q-5 (A): what changes" lists every row (A) rewrites | **decided (A), main thread, 2026-10-14**, under the owner's direction (Q-1: patch crypto/tls, the article as the reference for how) and delegation ("Do what you think is right"); the owner noted the direction was already given. The same diff, refusal on a Go bump and compile-time marker, delivered through a patched GOROOT link tree; the D2 rows listed in "Q-5 (A): what changes" were rewritten in D2 design round 2 (2026-10-14), which also changed the tree's shape (every `src` directory real, embedded files copied) after a probe of the round-1 shape, and found that gopls sees the tree when it runs with the tree's environment. The recommendation in this row's fourth column predates the decision and is kept as the record of the question |

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
  → Constraint: many le actions start a Go command from `os.Environ()` and never reach `gotoolchain`; the patched GOROOT therefore joins le's own process environment, once, as `GOROOT`, `GOTOOLCHAIN=local` and a PATH entry, set by `stdpatch.Activate` in the le root handler `run`, and `gotoolchain` inherits it rather than declaring it again (D2 design, "One producer" and "Reach")
- [ ] `docs/architecture/testing/tracked-build-gate.md` - "Failure diagnostics and selftest deadlines"
  → Constraint: the tracked build "disable[s] persisted Go settings, ambient `GOFLAGS`, and enclosing workspaces", on purpose; it does so by appending overrides to `os.Environ()` (`goEnv`, read 2026-10-14) and scrubs no `GOROOT`, `GOTOOLCHAIN` or PATH, and `GOENV=off` disables only the go env file, so the patched GOROOT reaches it by inheritance from le's process environment with no code change, and the page states that in the D2 phase
- [ ] `docs/architecture/appliance/builder.md` - the gok build environment (`SetGokGoEnv`, `ExecuteGok`)
  → Constraint: gok's go subprocesses inherit this process's environment (`GOTOOLCHAIN=local`); `SetGokGoEnv` activates nothing (a shipped ze links no `internal/le` package) and refuses unless the environment already carries the tree (`GOROOT`, `GOTOOLCHAIN=local`, `$GOROOT/bin/go` first on PATH, the marker file); under le it passes by inheritance, and ze-setup's `ze appliance build` refuses with the command line that sets the three values (design round 4); both routes then compile with the pinned toolchain through the tree; the page gains the item in the D2 phase
- [ ] `docs/architecture/testing/qemu-integration.md` - the guest entry points and `all-tests`, declared by `internal/le/test/qemu/run.go`
  → Constraint: a guest has its own GOROOT and runs an arbitrary command, so it never inherits le's process environment; each guest profile runs the generator itself and exports the three values (D2 design, "Reach"), and the page states it in the D2 phase
- [ ] `docs/architecture/testing/verify-freshness-scope.md` - "Certificate" (the stage population of each mode), declared by `internal/le/verify/engine/stages.go`
  → Constraint: `changedStages` drops only the whole-tree stages, so `go patch-stamps` runs in both modes; the page's account of what each mode covers gains the stage in the D2 phase
- [ ] `docs/contributing/patching-the-standard-library.md` - the technique (new, 2026-10-14; rewritten for the patched GOROOT in D2 design round 2, and for one producer and the review stamps in round 3)
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
- D2 cannot be fixed inside `crypto/tls`'s public API: there is no hook for the ServerHello session_id. Q-1 (b) patches the package at build time ("D2 design"); the A-10 probe found the disk-reading tools (golangci-lint, staticcheck, govulncheck) refuse an `-overlay`'s added file, so Q-5 (A) delivers the patch as a patched GOROOT link tree that every tool reads from disk
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
  → Constraint (corrected in D2 review round 1, read at HEAD 2026-10-14): `peerEndpoint` has four writers, not one: `adoptAuthenticatedEndpoint` (`sa.go`), `inheritSendPath` (`sa.go`, an IKE rekey copies the old SA's), `setMobikePath` (`mobike.go`, a MOBIKE update) and the responder IKE_AUTH MOBIKE branch (`responder.go`, "RFC 4555 Section 3.3 initializes the IKE tuple from the first IKE_AUTH request"). All four write an authenticated or inherited endpoint, so `childEncapPorts` reading `sa.peerEndpoint` holds for MOBIKE and non-MOBIKE SAs alike. `adoptAuthenticatedEndpoint` floats the SA on a NAT-T arrival; a later different endpoint is adopted only without MOBIKE and when Ze is not behind a NAT
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
| EAP-TLS authenticator ↔ crypto/tls | `tls.Config`, its `GetConfigForClient` (TLS 1.2 ticket suppression) and the patched `handshake_server.go` in the stdpatch GOROOT (D2 design) | No: `TestRFC5216TLS12FullHandshakeServerHelloSessionID` reads the ServerHello bytes (AC-2) |
| build tooling ↔ GOROOT | le's root handler `run` → `stdpatch.Activate` → le's process environment (`GOROOT=<tree>`, `GOTOOLCHAIN=local`, `<tree>/bin` first on PATH), inherited by every le child and every `Toolchain`; the launcher, QEMU guests, Docker and CI run the same generator through `go run ./internal/le/go/stdpatch/goroot` (D2 design, "One producer" and "Reach") | No: AC-20, AC-21, AC-25, AC-26 |
| build tooling ↔ patch review stamps | `Ze-Reviewed-Against` in each patch's leading text, compared with the tree's `GOVERSION` (step 1a), with `vendor/modules.txt` (`applyVendorPatches`), and by `./le go patch-stamps check` in `./le verify` (D2 design, "The review stamp") | No: AC-26, AC-27 |

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

## D2 design (Q-1 answered (b), 2026-10-14: patch `crypto/tls` at build time; Q-5 (A): delivered as a patched GOROOT)

The contributor page for the technique is `docs/contributing/patching-the-standard-library.md`
(written in the design phase, stating that the mechanism is not built yet and that this spec builds
it). This section is the design; the page is the how-to and must agree with it.

**Two changes, both needed.** The standard-library patch alone fails for a client that offers the session
ticket extension: crypto/tls then issues a ticket, RFC 5077 Section 3.4 keeps the id empty, and RFC
5216 Section 2.1.1 is violated again. The authenticator change alone changes nothing: with no ticket,
go1.27.2 still sends an empty id (A-3).

| # | Where | What | Why |
|---|-------|------|-----|
| 1 | `crypto/tls` `serverHandshakeState.doFullHandshake` (`handshake_server.go`), in the patched GOROOT | directly after `hs.hello.ticketSupported` is computed, when it is false, fill `hs.hello.sessionId` with 32 octets read from `c.config.rand()`, and return the read error | RFC 5216 Section 2.1.1's MUST; 32 octets is the `SessionID<0..32>` maximum of RFC 5246 Section 7.4.1.2 and what AC-2 reads |
| 2 | `crypto/tls`, new file `zz_ze_server_session_id.go`, in the patched GOROOT | one exported constant, `ZeServerChoosesSessionID`, documented as the marker of this patch | the compile-time guard ("Loud failure") |
| 3 | `internal/core/eap/eap_tls.go` `newTLSMethod` | set `GetConfigForClient`: unless the ClientHello carries the supported_versions extension (`ClientHelloInfo.Extensions` holds 43) AND that extension lists TLS 1.3 (`ClientHelloInfo.SupportedVersions`), answer a `Clone` of the authenticator config with `SessionTicketsDisabled` true; otherwise answer nil (the base config) | no TLS 1.2 ticket, so step 1 fires for every TLS 1.2 client and RFC 5077 Section 3.4's SHOULD is not in play |
| 4 | `internal/core/eap/eap_tls.go` | a blank declaration that references `tls.ZeServerChoosesSessionID`, with the comment that names the page | the authenticator cannot compile without the patch |

**Why step 3 keys on extension 43, and not on `SupportedVersions` alone or on the negotiated version.**
`SessionTicketsDisabled` on the base config is wrong: `handshake_server_tls13.go` reads the same field,
so it would also stop the TLS 1.3 NewSessionTicket that RFC 9190 Section 2.1.2 requires ("the EAP-TLS
server MUST send one or more post-handshake NewSessionTicket messages"). So the decision is per
handshake, and it must equal the version crypto/tls will negotiate. Read in go1.27.2
`handshake_server.go` (D2 review round 1, I-1):

- `clientHelloInfo` fills `SupportedVersions` with `supportedVersionsFromMax(clientHello.vers)` when the
  extension is absent, so a ClientHello with legacy_version 0x0304 and no supported_versions extension
  shows TLS 1.3 in `SupportedVersions`.
- `readClientHello` then negotiates for that same ClientHello with `supportedVersionsFromMax(VersionTLS12)`
  (its comment: "RFC 8446 4.2.1 indicates when the supported_versions extension is not sent, compatible
  servers MUST negotiate TLS 1.2 or earlier"). Keyed on `SupportedVersions` alone, that client keeps
  tickets on, gets a TLS 1.2 ticket and an empty id, and the MUST fails.
- With the extension present, `SupportedVersions` IS the extension's list and `mutualVersion` walks the
  server's versions highest first, so 1.3 is chosen exactly when the list holds it (`MaxVersion` 1.3, no
  QUIC). An empty extension is refused at unmarshal. So "extension 43 present and listing 1.3" is
  exactly crypto/tls's TLS 1.3 decision for this config.
- Keying on the negotiated version would be the direct form, and the public API has no place for it:
  `GetConfigForClient` runs before `mutualVersion` (`c.vers` is unset), and the TLS 1.2 ticket decision
  (`hs.hello.ticketSupported = hs.clientHello.ticketSupported && !c.config.SessionTicketsDisabled` in
  `doFullHandshake`) reads only the config that callback answered. Moving the decision into the patch
  would widen the hunk to every ze TLS server's ticket behaviour, which "Reach" keeps unchanged.

The predicate's equality with crypto/tls's negotiation is pinned by AC-2's third subtest, so a toolchain
bump that changes the negotiation rule goes red there.

**No conflict with RFC 5216 resumption or the existing code.** RFC 5216 Section 2.1.2 gives the
decision to the server ("the EAP server will decide whether to allow the continuation or to choose a
new session"), and the owner ruled on 2026-09-29 that the authenticator never resumes TLS 1.2
(`redeemTLS13Only`, `37dbe54095`). With tickets disabled, `checkForResumption` returns before
`UnwrapSession` (its first statement: `if c.config.SessionTicketsDisabled { return nil }`), so
`redeemTLS13Only` and `refuseResumption` are not reached on a TLS 1.2 handshake whose ClientHello step
3's predicate classifies as TLS 1.2. That holds for every TLS 1.2 handshake only because the predicate
equals crypto/tls's negotiation (above); with the earlier `SupportedVersions`-only key, a legacy-0x0304
client without the extension reached `UnwrapSession` on TLS 1.2. `redeemTLS13Only` stays the refusal
of any TLS 1.2 session that does reach it, and keeps its TLS 1.3 role. The id is never resumed, which RFC 5246 Section 7.4.1.3 permits. Issuing no TLS
1.2 ticket also removes the ticket ze could never redeem that Q-1's recommendation objected to. The
comment in `newTLSMethod` "the ticket above goes out either way" becomes false for TLS 1.2 and is
rewritten (`ai/rules/stale-comments.md`). The implementer checks which `rfc/discrimination/rfc5216.json`
records the new `GetConfigForClient` stales (the enforced 2.1.1-5, 2.1.1-10 and 2.1.2-1 units drive
`newTLSMethod`) and re-records them.

**Reach of the patch.** The patch changes `crypto/tls` for every TLS server in every ze binary (web,
gNMI, looking glass, the DNS-over-TLS server `internal/core/dnsserver/secure.go`, the hub's MCP service
`cmd/ze/hub/service_mcp.go`, and the others). The hunk is one statement group in one function, so its reach
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
answers the patched GOROOT for one toolchain. It imports only the standard library and nothing that
references the marker, so it compiles under the unpatched GOROOT, before any tree exists. This follows
the vendor patch precedent (`internal/le/vendorpatch/patches/`), with one difference: a vendored file is
in the tree and a GOROOT file is not, so the patch is applied at build time, never committed as a
patched copy.

**Delivery: a patched GOROOT (Q-5 (A), decided 2026-10-14).** The patched source has to exist on disk,
because golangci-lint, staticcheck, govulncheck and gopls read it from disk (A-10 probe). So `stdpatch`
builds a GOROOT that is the toolchain's own, with the patched file and the marker file real, and every
compile site selects it with `GOROOT=`. No `-overlay` carries the standard-library patch anywhere. The
article's idea is kept whole: the standard library is changed at build time from a diff applied to the
toolchain in use, with no fork and no copy in the tree, and a bump that moves the code refuses. Only the
delivery differs: a directory every tool reads, instead of a JSON map only the go command reads.

**How the tree is produced** (`stdpatch`, every call):

1. Resolve the real GOROOT and version: `go env GOROOT GOVERSION`, under the environment of the build that
   will use the tree, with `GOTOOLCHAIN` at that build's pin, and with an inherited `GOROOT` dropped when
   it names a path under `cache/stdpatch/` (a developer's exported value; the generator never depends on
   its own output, and the launcher's `go run` of the generator drops it the same way). The pin decides
   which GOROOT is real: when it names a version other than the go on PATH, the go command execs the
   pinned toolchain and sets `GOROOT` to that toolchain's directory (`execGoToolchain`,
   `cmd/go/internal/toolchain/exec.go`, go1.27.2, read 2026-10-14), so the real GOROOT is then under
   `GOMODCACHE`. An inherited PATH that starts with a tree's `bin/` is harmless here: that `bin` is a
   symlink, and a go command with no `GOROOT` finds its root from its resolved executable (`findGOROOT`,
   `cmd/go/internal/cfg/cfg.go`: `os.Executable`, then `filepath.EvalSymlinks`).
1a. Read each patch's review stamp (D2 design, "The review stamp") and compare it with the `GOVERSION`
   of step 1. A difference refuses here, before any tree is built and before `git apply` runs, EVEN
   WHEN THE DIFF WOULD STILL APPLY: a clean apply proves the context lines are unchanged, never that
   upstream left the function's behaviour alone. The error names both versions, the patch file and
   the re-review step of the page section "When the Go version changes" (a plain edit that needs no
   le, "One producer"), and ends with the command line to run after the edit, `./le go stdpatch goroot`.
2. Key the result by a SHA-256 over a layout version constant of `stdpatch` (bumped whenever the tree's
   shape changes, so an old tree is never reused under a new rule), the `GOVERSION`, the absolute real
   GOROOT path, the patch bytes and each target GOROOT file's bytes. The path is in the key because every
   symlink in the tree names an absolute path into that GOROOT and `cache/` resolves to the per-user
   `~/.cache/ze` that every checkout shares: two toolchains of one version at two paths (a `/usr/local/go`
   and a `GOMODCACHE` download) need two trees.
3. When `cache/stdpatch/<key>/goroot` exists, answer it. Step 4's rename makes existence mean complete.
4. Otherwise build it in a new temporary directory `cache/stdpatch/tmp-<random>/`, on the same
   filesystem as the key directories, so the final rename is atomic:
   1. every top-level GOROOT entry except `src` (`bin`, `pkg`, `lib`, `misc`, `VERSION`, `go.env`, and
      whatever else the toolchain carries) is a symlink to the real one, so the tree runs the real go
      binary and the prebuilt tools under `pkg/tool`;
   2. under `src`, every directory is a REAL directory and every file a symlink to the real file.
      Directories are never symlinked: the go command's `...` pattern walk does not follow a symlinked
      directory (`cmd/go/internal/search/search.go` and `modload/search.go`, "warning: ignoring symlink"),
      and the round-1 tree, which mirrored only the path down to `src/crypto/tls`, answered `go list std`
      with two packages, `crypto` and `crypto/tls`, against 383 (round-2 probe);
   3. every file that a `//go:embed` pattern of a package in `std` or `cmd` names is a COPY: the go command
      refuses to embed a symlink ("cannot embed irregular file", `cmd/go/internal/load/pkg.go`), and the
      round-2 probe's first all-symlink tree failed `go list std` with that error on
      `encoding/json/internal/jsontest` and `internal/trace/traceviewer`. The set is read from the real
      GOROOT with `go list -e` over `std cmd` and each package's `EmbedFiles`, and the generator takes the
      UNION of that answer over `CGO_ENABLED=0` and `CGO_ENABLED=1` and over every GOOS/GOARCH of the
      target list (linux/amd64, linux/arm64, darwin/arm64, freebsd/amd64, windows/amd64; the list AC-25
      runs), because `EmbedFiles` follows build constraints and one tree serves every target and both
      cgo settings (step 4's guest route cross-compiles under the host tree). The probe found 9 files in
      `std` and 35 in `cmd`, 2.9 MiB on go1.27.2, the same set for each of those five targets; the union
      keeps that true by construction when a later release adds a constrained embed. `cmd` is included because go1.27.2 ships only
      `asm cgo compile cover fix link preprofile vet` prebuilt and builds the other tools
      (`go tool covdata`, `go tool test2json`, both built under the probe tree) from `src/cmd` on demand;
   4. each file the patch targets is a copy; `git apply` runs in the temporary `goroot` with
      `GIT_CEILING_DIRECTORIES` set to its parent, so git cannot find the ze repository; the marker file
      the patch creates is a real file;
   5. under the new tree, `go list -e std cmd` must answer the package set the real GOROOT answers, with no
      package error; a difference is an error naming the first package that differs, and the temporary
      directory is removed, never renamed into place;
   6. rename the temporary directory to the key. The rename loser gets ENOTEMPTY or EEXIST: it removes
      its own temporary directory and answers the winner's tree, after checking that the winner's
      `goroot/VERSION` resolves; any other rename error is returned.

   The tree is 30 MiB on go1.27.2 (1371 directories, 12030 symlinks and the copies; round-2 probe).
   Symlinks, not hard links: a symlink crosses filesystems and needs only read access to its target,
   while Linux refuses a hard link to a root-owned GOROOT file the user does not own
   (`fs.protected_hardlinks`).
4a. `git` is therefore a build dependency wherever the generator runs. The QEMU guest already installs it
   (`internal/le/test/qemu/run.go`, `apk add --no-cache git curl musl-dev`; `run_exec.go`'s keep-alive
   profile runs in a guest bootstrapped the same way, re-read at implementation). The `golang:1.27-alpine`
   builder stages of `docker/Dockerfile` and `docker/Dockerfile.lab` carry no git, so each gains
   `apk add --no-cache git` before the generator runs. Those stages float on `1.27`, not `1.27.2`; that
   reaches neither step 1a nor step 5, because step 1 resolves the real GOROOT under `GOTOOLCHAIN` at
   go.mod's pin, so a newer image release only runs the generator and the tree is built from the pinned
   release (a download inside the stage). The generator, like `build_le`, falls back from a dangling
   `cache/` to `tmp/` (the QEMU guest mounts only `/workspace`, `le` script `build_le`).
4b. `CleanCaches` (`internal/le/scratch/cacheclean.go`) leaves `cache/stdpatch/` in place and names each
   key directory with its size in its report. Another session's build in flight runs with `GOROOT` inside
   it, and a GOROOT removed under a running compile fails that build with "no such file" errors that never
   name the cause. A key costs about 30 MiB, against the gigabytes of the Go caches the clean empties; a
   new key appears only with a new toolchain version or path, and a stale key is never selected again,
   because the key carries the version and the bytes. The report line is what lets the owner remove a
   key whose toolchain is gone.
5. When `git apply` refuses (it has no fuzz: a changed context line refuses), return an error naming
   the Go version, the patch file, git's output and the page section "When the Go version changes".
   No tree is written, and no caller builds without one after this error.

**What the tree answers.** Under it, `go env GOROOT` answers the tree; `go version` and `go env GOVERSION`
answer the real toolchain's version, because `VERSION` and the go binary are the real ones (probe:
`go version go1.27.2 linux/amd64`, `GOTOOLDIR` under the tree's `pkg/tool`). govulncheck therefore
matches standard-library advisories against the true version (probe: "No vulnerabilities found."). A
binary built without `-trimpath` records the tree as its default `runtime.GOROOT()`; no production code
calls `runtime.GOROOT()` (grep of `cmd/`, `internal/`, `pkg/` at HEAD, 2026-10-14). A non-trimpath build
keys standard-library objects by directory, so they compile once per tree path, and checkouts that share
`cache/` share them. cgo and the race detector work under the tree (round-2 probe: `CGO_ENABLED=1 go run
-race` of the handshake program, and a cgo program calling C and `os/user`).

**Selection: what every compile site sets.** All three values come from one `stdpatch` answer:

| Variable | Value | Why |
|----------|-------|-----|
| `GOROOT` | the tree | the patched source on disk, for the go command and every go/packages tool |
| `GOTOOLCHAIN` | `local` | a toolchain switch sets `GOROOT` to the switched-to toolchain (step 1), which would discard the tree. The pin still holds: step 1 built the tree from the pinned toolchain, so `local` runs exactly the pinned version |
| `PATH` | `<tree>/bin` first | a tool that starts `go` itself (golangci-lint, staticcheck and govulncheck through go/packages, CodeQL's extractor) must find the pinned go; under `GOTOOLCHAIN=local` a different go on PATH would compile with the tree's `pkg/tool` compiler of another version |

**One producer: le sets the three values in its own process environment (design round 3).** Many compile
sites build a child environment from `os.Environ()` and run a literal `go`, never through `gotoolchain`
(the inheritance rows below list them), and the launcher's `exec "$binary"` hands le no `GOROOT`: the
script sets the pin only inside `build_le`, for its own `go build`. So the values are set once, where le
starts, and every child inherits them:

- `stdpatch.Activate(root)` answers the tree for go.mod's pin (steps 1 to 5) and then, with `os.Setenv`,
  sets `GOROOT=<tree>`, `GOTOOLCHAIN=local` and a PATH whose first entry is `<tree>/bin`. It does not add
  that entry again when PATH already starts with it, so a le started by le keeps one entry. Step 1 drops
  an inherited `GOROOT` under `cache/stdpatch/` (compared after `filepath.EvalSymlinks`, since `cache/`
  is a link to `~/.cache/ze`), so a nested le resolves the same tree rather than reading its own output.
- The le root handler `run` (`internal/le/register.go`, the entry of every le command and native hook,
  for `le` and for `ze le`) calls it FIRST, before `leroot.RunBeforeDispatch` and `leroot.Dispatch`. At
  that point the process runs one goroutine, so `os.Setenv` races nothing; a before-dispatch hook (the
  scratch trim) may start goroutines and runs after it. It is not a `leroot.BeforeDispatch` hook: that
  contract forbids slow work and gives a hook no way to fail (`internal/le/le/root/beforedispatch.go`),
  and the first call per toolchain builds the tree. The cached call costs one `go env` (8 ms measured
  on this host, 2026-10-14) plus hashing the patch and its target files.
- `exec.Command` resolves a bare `go` with `exec.LookPath` against le's CURRENT `PATH`, so after
  `Activate` every le-built argv starting with `go` runs `<tree>/bin/go` and every `os.Environ()` child
  holds the three values. The per-argv rewrite of round 2 (`Toolchain.Go` replacing `goProgram`) is
  deleted from the design: a second route to the same choice is the layering `ai/rules/no-layering.md`
  bans. `goProgram` stays `go`.
- A refusal (step 1a or step 5) does not stop the dispatch of a le that is already built: the hooks,
  `./le spec`, the commit flow and `./le go stdpatch goroot` keep working on the day a bump refuses.
  `Activate` then sets none of the three values and keeps its error. Two things fail, and nothing else
  (design round 4, ISSUE 2): every action that compiles through `gotoolchain.New` refuses with the step
  1a or step 5 text, because `New` returns that error (the `Toolchain` users of the inheritance row: unit
  tests, lint, `./le build`, interop image builds, perf, the vulnerability scan); and any other command
  that compiles a package linking `internal/core/eap` (an `os.Environ()` site, a plain `go`) runs the
  ambient go and fails at the marker, whose comment names the page. `./le go stdpatch goroot` prints the
  tree, or the refusal with a non-zero exit.
- The repair needs no le (design round 4, ISSUE 1). On a checkout with no usable `bin/le` (a fresh
  checkout, `./le --name`, `./le --update`, a per-platform binary) the launcher must BUILD le; le links
  `internal/core/eap` and its marker, and the launcher's generator refuses at step 1a, so no le exists
  until the patch is repaired, and the hooks that run through `./le` fail on that checkout too. The
  repair is therefore a plain edit of the patch file, made without le: read the upstream function in the
  pinned toolchain's GOROOT (`grep -n doFullHandshake "$(env -u GOROOT -u GOTOOLCHAIN go env GOROOT)/src/crypto/tls/handshake_server.go"`,
  run in the repository so the go.mod pin selects the toolchain; design round 5, ISSUE 3: a shell that
  ran the AC-28 command line, or a session under le, holds `GOROOT=<tree>` and `GOTOOLCHAIN=local`, and a
  bare `go env GOROOT` then answers the tree, whose file is the patched one; a path under
  `cache/stdpatch/` means `<tree>/bin` is first on PATH, and the step is rerun without it); when upstream
  now chooses the id, delete the patch, the marker reference in `newTLSMethod` and `stdpatch`
  (`ai/rules/no-layering.md`), AND the AC-28 marker path constant in `internal/appliance/cmd_build.go`,
  its check in `SetGokGoEnv` and `TestApplianceMarkerPathMatchesThePatch` (a marker check left behind
  refuses every `ze appliance build` forever); with no patch left, the rest of the AC-28 check goes too
  (its refusal names the deleted `./le go stdpatch goroot`), `SetGokGoEnv` sets `GOTOOLCHAIN=local`
  itself again and `TestGokBuildsWithTheHostToolchain` returns to its HEAD assertion;
  otherwise regenerate the diff with `git diff --no-index <original> <patched>` if git no longer applies
  it, and change the `Ze-Reviewed-Against` line in the patch header to the new version. The next `./le`
  call then builds le. The step 1a and step 5 refusals carry this procedure and end with the command
  line to run after the edit (`./le go stdpatch goroot`), as `ai/rules/user-facing-errors.md` requires.
- `gotoolchain.New` calls the same `Activate`. Under le it answers the tree the process already
  carries; in a test binary it is the activation. One function, so there is one answer. ze-setup's
  `ze appliance build` does NOT call it, because a shipped ze never links an `internal/le` package
  (`docs/architecture/core-design.md`); it refuses an environment that does not already carry the tree
  ("Reach", gokrazy row).

`gotoolchain` follows: `Overrides` stops setting `GOTOOLCHAIN=<pin>` and sets no `GOROOT`, because the
process environment holds both and a second declaration in the overrides could only disagree with it.
Its contract ("the variables that this toolchain SETS") then names what it still sets (GOCACHE,
GOLANGCI_LINT_CACHE, CGO_ENABLED, GOMAXPROCS, GOMEMLIMIT, GOOS, GOARCH), and `Environment` inherits the
three values with the rest of `os.Environ()`. The report-only callers (`verify/deps` `commandPlan`,
functional `exabgp.go`) show the shorter `Overrides`. The package doc's GOTOOLCHAIN paragraph is
restated: the pin chooses the toolchain the tree is built from, and `Activate` selects it.

**Reach: every Go command that compiles ze.**

| Path | Today | Change |
|------|-------|--------|
| the `./le` launcher bootstrap (`le` script, `build_le`) | `go build -tags ze_le,... ./cmd/ze` under the pin it exports; that binary links `internal/core/eap` (`go list -deps -tags ze_le ./cmd/ze`) | le does not exist yet, so the script has its own route: `go run ./internal/le/go/stdpatch/goroot` first (the same resolution `Activate` runs; it prints the tree), under the same pin and the unpatched GOROOT, then `GOROOT="$tree" GOTOOLCHAIN=local PATH="$tree/bin:$PATH" "$tree/bin/go" build ...`; a refusal stops the bootstrap. The values are set for that command only; le sets its own at `run` |
| **inherits from le's process environment**: every le action, and every child process le starts, that builds its environment from `os.Environ()` (or `Toolchain.Environment`) and runs `go` or `exec.LookPath("go")` | each builds its env and runs `go` from PATH | covered by `Activate` at `run`, no per-site change. The sites (round-2 review ISSUE-1 and ISSUE-2, 2026-10-14; the implementer re-greps `os.Environ()` beside an `exec.Command*(..., "go"` and adds any new one to this row): `internal/test/runner/runner.go` (`childEnv`; the `.ci` runner runs inside le), `internal/test/cli/cmd_bgp.go`, `internal/le/test/deployment/daemonbuild.go` and `vppevidence.go`, `internal/appliance/cmd_initrd.go`, `kernelbuilder/qemu.go`, `internal/le/doc/yangcontract/command_surfaces.go`, `internal/le/test/functional/binaries.go` (two sites), `run.go` and `reach.go`, `internal/le/test/qemu/dockerlab.go` and `mobike.go` (host-side builds), `internal/le/test/integration/stress.go`, `internal/le/linuxle/linuxle.go`, `internal/le/terminaldemo/actions.go` (two sites), `internal/le/verify/deps/verifydeps.go` (two sites), `gotoolchain.goRun`; and every `Toolchain` user (`./le test unit`, `./le go lint`, `./le build`, interop `zebuild.go`, `./le perf`, `./le verify deps vulnerability`) |
| the tracked build (`repo/compiles/repositorytrackedbuild.go` `goEnv`), `repo/bootstraps/tracked.go`, `repo/changed/selector.go`, `setup/install.go`, `appliance/gokpreflight.go` | refuse or pin ambient settings by appending to `os.Environ()` | inherits, read 2026-10-14: `goEnv` is `append(os.Environ(), "CGO_ENABLED=0", "GOENV=off", "GOFLAGS=-buildvcs=false", "GOWORK=off")` plus GOOS/GOARCH, so it overrides GOFLAGS and the workspace and scrubs no `GOROOT`, `GOTOOLCHAIN` or PATH; `GOENV=off` disables the go env FILE, never an environment variable. `tracked.go` (`GOFLAGS=-mod=vendor`) and `selector.go` (`GOFLAGS=`) set GOFLAGS only; `install.go` drops only `GOMODCACHE=` and `GOFLAGS=` from what it inherits; `gokpreflight.go` appends `GOTOOLCHAIN=local`, which equals le's value. No code change; the tracked build's refusal of ambient GOFLAGS is unchanged, and `docs/architecture/testing/tracked-build-gate.md` states that the tree arrives through the inherited environment. `go/module/move.go` and `setup/proto_generate.go` set no GOFLAGS and inherit the same way |
| `./le rfc discriminate-record` (`writeOverlay`, `setGoFlagsOverlay`, `environment`) | writes its own overlay of the broken repository file, through this process's GOFLAGS before `testfunctional.Prepare`, `GOFLAGS=-overlay=` for interop and functional children, and a `-overlay` flag on a unit run | inherits; unchanged. Its overlay maps only the broken repository file and the standard library comes from the inherited `GOROOT`, so each command holds one `-overlay` by construction and B-1 does not arise. The guest route (`discriminate_guest.go`) cross-compiles on the host under `Environment` with the guest's GOOS and GOARCH, so it carries the host tree, which serves every target (step 4.3) |
| gokrazy, two entry points (`appliance.SetGokGoEnv` in `internal/appliance/cmd_build.go`) | sets `GOTOOLCHAIN=local`, GOMODCACHE, GOPROXY and GOFLAGS in this process for gok's go subprocesses | `./le build gokrazy` (`internal/le/build/gokrazy/gokrazy.go` `Run`) inherits from le. The second entry is NOT under le: `ze appliance build <name>` in the ze-setup binary (`cmd/ze/setup_features_setup.go` blank-imports `internal/appliance`; the `build` row of `internal/appliance/main.go` → `runBuild` → `gokBuildFn = runGokInProcess`). It cannot call `stdpatch.Activate`: the ze-setup binary ships, `docs/architecture/core-design.md` ("The rule is directional") says `ze` never links an `internal/le` package, and no non-test file of `internal/appliance` imports one (read 2026-10-14: only `cmd_build_test.go` imports `internal/le/repo/featuretags`; round 3 claimed `cache.go` and `cmd_build.go` did, which was false). Design round 4: `SetGokGoEnv` ACTIVATES NOTHING and REFUSES, before it touches GOMODCACHE or GOPROXY, unless its environment already carries the tree: `GOROOT` is set, `GOTOOLCHAIN` is `local`, `exec.LookPath("go")` resolves to `$GOROOT/bin/go`, and `$GOROOT/src/crypto/tls/zz_ze_server_session_id.go` exists. The check is by file existence and imports nothing from `internal/le`; the marker path is a constant in `internal/appliance` whose comment names its source (the patch under `internal/le/go/stdpatch/patches/`), and an `internal/appliance` test (a test file may import `internal/le`) compares it with the file the patch adds. Its own `os.Setenv("GOTOOLCHAIN", "local")` is deleted: a refused environment never reaches it and an accepted one already holds `local` (`ai/rules/no-layering.md`). The refusal says what (this environment does not select Ze's patched standard library, naming the `GOROOT` and `GOTOOLCHAIN` it found), why (the image's ze links the EAP-TLS authenticator, which compiles only against the patched `crypto/tls`; the page path), and ends with the command line `GOROOT="$(./le go stdpatch goroot)" && export GOROOT GOTOOLCHAIN=local PATH="$GOROOT/bin:$PATH" && ze appliance build <name>`, naming `./le build gokrazy <gok args>` as the le route that carries the tree. Every route through le passes by inheritance: `./le build gokrazy`, and the le actions that run `ze appliance build` as a child from `os.Environ()` (`internal/le/test/qemu/install_build.go` and `hugepages.go`, `applianceEnv`). Both routes compile the image with the pinned toolchain through the tree, where the ze-setup route compiled with the host go before; `CheckGoToolchain` (`gokpreflight.go`) and the `SetGokGoEnv` doc paragraph on the host go are restated at implementation. `TestNormalZeLinksNoInternalLe` measures only `ze_core`+`ze_distro` (`normalZeTags`), so it would not have caught the round-3 edge; AC-29 extends it to `ze_setup` |
| QEMU guests that compile (the guest profile in `internal/le/test/qemu/run.go`; the keep-alive profile in `run_exec.go` `keepAlive`) | export `GOROOT=/usr/local/go` (the guest's own go, `PATH=/usr/local/go/bin:$PATH`) and GOFLAGS; the guest command is arbitrary (`keywordCommand`) | own environment: the guest has its own GOROOT and its command may run go directly. Each profile keeps `GOROOT=/usr/local/go` as the real root, then runs `tree=$(go run ./internal/le/go/stdpatch/goroot)` and exports `GOROOT="$tree"`, `GOTOOLCHAIN=local` and `PATH="$tree/bin:$PATH"`; GOFLAGS stays `-buildvcs=false`; the guest has git (step 4a); its `cache/` dangles, so the tree lands under `tmp/` (step 4a). A guest command that runs `./le` activates again and answers the same tree |
| Docker images that compile ze (`docker/Dockerfile`, `docker/Dockerfile.lab`) | `FROM golang:1.27-alpine`, plain `go build` | `apk add --no-cache git`, then ONE `RUN` that resolves the tree and builds with it: `tree=$(go run ./internal/le/go/stdpatch/goroot) && GOROOT="$tree" GOTOOLCHAIN=local PATH="$tree/bin:$PATH" "$tree/bin/go" build ...`, because `ENV` cannot hold a command's output. `cache/` is in `.dockerignore`, so the generator writes a real `cache/stdpatch/` inside the stage |
| CI workflows that run go directly (`.github/workflows/codeql.yml`, `qemu-nightly.yml`, every job that compiles after `actions/setup-go`) | plain `go build`, `go test` | a step after `setup-go` runs the generator, appends `GOROOT=<tree>` and `GOTOOLCHAIN=local` to `$GITHUB_ENV` and `<tree>/bin` to `$GITHUB_PATH`, before the first go command. CodeQL's Go extractor loads through go/packages and reads source from disk, which the tree serves as it served golangci-lint |
| `.github/workflows/govulncheck.yml` | runs `./le verify deps vulnerability` | reached through the le row; govulncheck passes under the tree (probe) |
| a developer's plain `go build`/`go test`, an editor's gopls, an agent's `gopls` from Bash | no le | a build fails with `undefined: tls.ZeServerChoosesSessionID` at the marker reference, whose comment names the page. The remedy is the page's three exports (`GOROOT="$(./le go stdpatch goroot)"`, `GOTOOLCHAIN=local`, `PATH="$GOROOT/bin:$PATH"`). gopls sees the patch when it is STARTED with that environment (round-2 probe: gopls v0.23.0 run directly under the tree reports nothing). A gopls daemon shared across clients holds the environment of the client that started it: this host's `~/bin/gopls` forwards to `gopls -remote=auto`, whose view env read `GOROOT:/usr/local/go GOTOOLCHAIN:auto` in the trace, which is why round 1 saw the marker undefined under the tree as well. An agent's `gopls` from Bash on this host therefore shows that one diagnostic; `gopls symbols`, definitions and references are unaffected |

`./le go stdpatch goroot` is a new le command (`ai/rules/cli.md`): it prints the absolute tree path for
this checkout's pinned toolchain and exits non-zero with the refusal of step 5.

**A-10 probe (D2 review round 1, 2026-10-14), and its round-2 follow-up.** Scripts and logs in
`tmp/session/2026-10-13-9d2e6c21-74c1-4c42-b9ef-30145b95cfb1/scratch/overlay-probe/` (`probe2.sh`,
`probe3.sh`, `out-*.log`, `out3-*.log`, `out4-race.log`; round 2 ran its commands from the shell over
`goroot-mirror/`, the step 4 layout): a module whose `marker.go` holds
`const _ = tls.ZeServerChoosesSessionID`, go1.27.2 at `/usr/local/go`, `GOTOOLCHAIN=local`, the patch
of A-3 (patched `handshake_server.go` plus the added marker file).

| Tool | plain | `GOFLAGS=-overlay=...` | round-1 tree (only `src/crypto/tls` mirrored) | step 4 tree (round 2) |
|------|-------|------------------------|-----------------------------------------------|-----------------------|
| `go vet` | undefined marker | passes | passes | passes |
| `go list std` / `go list cmd` | 383 / 359 | - | 2 (`crypto`, `crypto/tls`) | 383 / 359, no package error |
| `go run` (the A-3 handshakes) | - | (A-3) | TLS 1.2 with ticket: id 0 octets; without: 32 | the same; also with `CGO_ENABLED=1 -race` |
| cgo program (C call, `os/user`) | - | - | - | passes |
| `go tool covdata`, `go tool test2json` (built from `src/cmd`) | - | - | - | build and run |
| golangci-lint 2.14.0 (`--enable=govet`) | undefined marker | "open /usr/local/go/src/crypto/tls/zz_ze_server_session_id.go: no such file or directory (typecheck)" | 0 issues | 0 issues |
| staticcheck | undefined marker | the same "no such file or directory" | passes | passes |
| govulncheck | - | "loading packages", then the added file: "no such file or directory" | "No vulnerabilities found." | "No vulnerabilities found." |
| gopls v0.23.0 `check marker.go`, through the shared `-remote=auto` daemon | undefined marker | undefined marker | undefined marker | undefined marker (the daemon's own env) |
| gopls v0.23.0 `check marker.go`, binary run directly | undefined marker | - | passes | passes |
| `GOTOOLCHAIN=go1.27.2+auto go env GOROOT` | - | - | the tree (pin equals the local version, so no switch) | - |

So the overlay is broken for every go/packages loader that reads source itself, and ze runs two of them as
gates (`./le go lint`, `./le verify deps vulnerability`). Dropping only the added file does not help: a
marker in a REPLACED file is read by those tools from the unpatched disk copy. The step 4 tree serves
every tool, and gopls too when gopls runs with the tree's environment.

The round-1 "Q-5 (A): what changes" table is applied: every row it listed is rewritten above (D2 design
round 2 in the Design Review Log), so the table is removed rather than kept beside the design it described.

**Loud failure.** Three guards, each covering a hole the others leave:

| Guard | Catches | Misses |
|-------|---------|--------|
| compile-time marker (the "Two changes" rows 2 and 4) | any build of any binary that carries the EAP-TLS authenticator under an unpatched GOROOT: CI, a Dockerfile, a developer, a missed le site; and any disk-reading tool that loads it under one | a tree that carries the marker but not the hunk |
| `TestRFC5216TLS12FullHandshakeServerHelloSessionID` and `TestPatchedCryptoTLSServerSessionID` | the hunk missing or wrong | a shipped binary built by a path the tests never ran |
| step 5 refusal | a toolchain bump that moved the patched code | a bump whose diff still applies |
| step 1a stamp refusal, and `./le go patch-stamps check` in `./le verify` | ANY toolchain other than the one the patch was reviewed against, even when the diff applies; for vendor patches, any vendored module version other than the reviewed one | nothing on its own: it is what turns every bump into a re-review |

A runtime self-probe (a loopback TLS 1.2 handshake at `newTLSMethod` that refuses EAP-TLS when the id
is empty) was rejected: it keeps plain builds working, but it ships an unpatched binary that fails at
the operator's site. A test-only guard was rejected because a test proves its own build, never the
shipped one.

**The review stamp (owner requirement, 2026-10-14: "we should detect if go is updated and we have not
updated our embedded go libraries").** A clean `git apply` proves only that the context lines did not
move. Upstream can fix the defect, or change the function's behaviour, in lines the hunk never touches,
and the patch then applies over code nobody re-read. So every patch records what it was reviewed
against, and a difference refuses whether or not the diff applies:

| Patch family | Stamp, in the patch's leading text (which `git apply` skips) | Compared with | Refused by |
|--------------|-------------------------------------------------------------|---------------|------------|
| standard library, `internal/le/go/stdpatch/patches/*.patch` | `Ze-Reviewed-Against: go1.27.2` | the `GOVERSION` of the toolchain the tree is built from (step 1, under go.mod's pin) | the generator at step 1a, so `Activate`, the launcher, the guests, Docker and CI all refuse; and the check below |
| vendored module, `internal/le/vendorpatch/patches/*.patch` | `Ze-Reviewed-Against: <module path> <version>`, e.g. `github.com/vishvananda/netlink v1.3.1`, `charm.land/bubbles/v2 v2.2.1` (the versions `vendor/modules.txt` lists at HEAD, read 2026-10-14) | the `# <module path> <version>` line of `vendor/modules.txt`, which the go command keeps consistent with go.mod (`-mod=vendor` refuses an inconsistent vendor directory) | `applyVendorPatches` (`internal/le/setup/install.go`) before it runs `git apply`, so a re-vendor at a new version refuses before patching; and the check below |

A patch with no stamp, or a stamp that names no module the vendor directory holds, is refused the same
way: an unstamped patch is a patch nobody can say was reviewed. The refusal names both versions, the
patch file, and the re-review step: re-read the upstream function the patch changes (for the
standard-library patch, `doFullHandshake` in the new GOROOT); when upstream now does what the patch
did, delete the patch and what depends on it (`ai/rules/no-layering.md`); otherwise regenerate the
patch if needed, run its tests, and change the stamp to the new version. Each refusal ends with the
command line to run once the patch is repaired (`ai/rules/user-facing-errors.md`): `./le go stdpatch goroot`
for a standard-library stamp, `./le setup install` for a vendor stamp.

In `applyVendorPatches` (design round 4, NOTE) the stamp check runs FIRST for each patch, before the
`git apply --reverse --check` probe whose success today reports "vendor patch already applied" and
continues: an applied patch with a stale stamp is still an unreviewed patch, so it refuses. A patch the
set names but that does not exist on disk refuses as well: the `os.IsNotExist` `continue` at the top of
the loop is deleted, because with the set read from the directory (`vendorpatch.Patches`) a missing file
means the set and the disk disagree, never that the patch is optional.

The check is a new le action, `./le go patch-stamps check`, run by `./le verify` as
`stage("go patch-stamps", "check")` in `fullStages` (`internal/le/verify/engine/stages.go`) and kept in
`changedStages`, which excludes only the three whole-tree stages. It reads every patch in both
`patches/` directories (the directory is the set; `setup/install.go`'s hand-written `vendorPatchPaths`
is replaced by the same directory read, `vendorpatch.Patches`, so the set is declared once), and it
compares each standard-library stamp with go.mod's pin (`goversionpin.DeclaredRelease`, the existing one
reader of it) and with the `GOVERSION` of the activated environment, and each vendor stamp with
`vendor/modules.txt`. It costs file reads and one `go env`, so it is cheap in both verify modes. Stamp
parsing lives once per family: `stdpatch.Stamp` and `vendorpatch.Stamp`, each used by its refusal site
and by the check.

**When the Go version changes.** go.mod pins `go 1.27.2`. On a bump the generator refuses at step 1a,
because the stamp names go1.27.2. A le that is already built keeps dispatching; what refuses is every
action that compiles through `gotoolchain.New` (unit tests, lint, `./le build`, interop image builds,
perf, the vulnerability scan) and `./le verify` through the check, and any other command that compiles
a package linking `internal/core/eap` fails at the marker ("One producer"). A launcher that must build le
stops, so the repair is the plain edit of "One producer", made without le. The contributor re-reads
upstream `doFullHandshake` in the new GOROOT: when it now chooses a
session id itself, the patch, the marker file, the marker reference and `stdpatch` are deleted
(`ai/rules/no-layering.md`); otherwise the patch is regenerated if git refuses it (step 5, the
procedure on the page), the stamp is changed to the new version, and the two tests run. A re-vendor at
a new module version takes the same path through `applyVendorPatches` and the check.

**Comments the code owes** (owner: "document it in the spec and later code"). Each of these carries a
comment that names `docs/contributing/patching-the-standard-library.md` and quotes RFC 5216 Section
2.1.1 "If the peer's sessionId is null or unrecognized by the server, the server MUST choose the
sessionId to establish a new session." (`ai/rules/rfc-compliance.md`): the patch file's leading text,
the patched hunk itself (directly above the statements), the marker file, the `stdpatch` package doc
and the function that applies the patch, and the EAP-TLS call site in `newTLSMethod`
(`GetConfigForClient` and the marker reference). The `GetConfigForClient` comment also cites RFC 9190
Section 2.1.2 and quotes "the EAP-TLS server MUST send one or more post-handshake NewSessionTicket
messages", since that MUST is why the ticket is suppressed per handshake and never on the base config.
A caller elsewhere carries `// RFC 5216 Section 2.1.1`.

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
| any le build or test of a package linking `internal/core/eap` | → | le root handler `run` → `stdpatch.Activate` (process `GOROOT`, `GOTOOLCHAIN=local`, PATH) → inherited by `Toolchain.Environment` and every `os.Environ()` child → `go` resolved from PATH as `<tree>/bin/go` | `TestActivateSetsTheProcessEnvironment` (`internal/le/go/stdpatch/`), `TestLeChildInheritsThePatchedGOROOT` (`internal/le/register_test.go`), `TestToolchainOverridesLeaveTheToolchainToTheProcess` (`internal/le/go/toolchain/`), and every `./le test unit` run of `internal/core/eap` compiling at all |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | peer's IKE arrives from port Y behind a NAT | ESP encapsulated to port Y |
| AC-1 (design, 2026-10-13) | Ze responder (PSK), NAT detected, the authenticated IKE_AUTH arrives on Ze's NAT-T socket bound to port L from (A, Y), Y not 4500 | inbound XFRM state: encap espinudp sport Y dport L; outbound state: sport L dport Y. Proof: `TestRFC3948ResponderChildSAEncapsulatesToTheIKEPeerPort` (unit, the installed `SAParams`), `TestRFC3948InstalledStateCarriesTheIKEPeerPort` (Linux XFRM read back), interop `real-nat-port-translated-ze-responder` (traffic both ways through the tunnel, ze's state shows dport Y) |
| AC-2 | D2 | Q-1 answered (b), 2026-10-14 (rows below) |
| AC-2 (design, 2026-10-14) | TLS 1.2 ClientHello with an empty or unknown session_id reaches Ze's authenticator, in three cases: the client offers the session ticket extension; the client offers none; and the client sends legacy_version 0x0304 with the session ticket extension and NO supported_versions extension, so crypto/tls negotiates TLS 1.2 while `ClientHelloInfo.SupportedVersions` lists 1.3 (D2 design, "Why step 3 keys on extension 43") | ServerHello carries a 32-octet session_id that differs from the client's, and the server flight carries no NewSessionTicket. Proof: `TestRFC5216TLS12FullHandshakeServerHelloSessionID` reads the ServerHello bytes (three subtests, one per case; red at HEAD: length 0; the third goes red under a `SupportedVersions`-only predicate, its discrimination break), interop `responder-eap-tls12` completes a TLS 1.2 handshake with strongSwan under the patched build, and Ze then refuses the MSK export for want of RFC 7627 (owner re-scope, 2026-10-21). The third ClientHello is written as raw bytes, since `tls.Client` always sends supported_versions when it offers 1.3 |
| AC-19 | patched `crypto/tls`, a plain TLS 1.2 server outside EAP | a server that issues a ticket sends an empty session_id (RFC 5077 Section 3.4); a server that issues none sends 32 octets; a TLS 1.3 handshake is byte-for-byte the unpatched shape in its legacy_session_id echo. Proof: `TestPatchedCryptoTLSServerSessionID`, three subtests |
| AC-20 | each Go command path that compiles a package linking `internal/core/eap` (D2 design, "Reach"), under Q-5 (A) | every such command runs `<tree>/bin/go` with `GOROOT=<tree>`, `GOTOOLCHAIN=local` and `<tree>/bin` first on PATH, each inherited from le's process environment (or set by the site's own route: launcher, QEMU guest, Docker, CI; ze-setup's gokrazy build sets none and refuses without them, AC-28); no command carries a standard-library `-overlay`; a build of `internal/core/eap` under the real GOROOT fails with `undefined: tls.ZeServerChoosesSessionID`. Proof: `TestActivateSetsTheProcessEnvironment` (`stdpatch`): after `Activate`, `os.Getenv` holds `GOROOT=<tree>` and `GOTOOLCHAIN=local`, PATH starts with exactly one `<tree>/bin` and a second `Activate` (the nested-le case, inherited tree) adds none, and a refusal sets none of the three and is returned again by the accessor `gotoolchain.New` reads. `TestLeChildInheritsThePatchedGOROOT` (`internal/le`): the root handler `run`, driven with `go stdpatch goroot`, activates before dispatch; then a child started the way the inheritance sites start one (`exec.Command("go", "env", "GOROOT")` with `os.Environ()`, and `exec.LookPath("go")`) answers the tree and `<tree>/bin/go`, compared after `filepath.EvalSymlinks` on both sides (`cache/` is a link to `~/.cache/ze`). `TestToolchainOverridesLeaveTheToolchainToTheProcess`: `Overrides` holds no `GOROOT`, no `GOTOOLCHAIN` and no GOFLAGS, and `Environment` carries the process's three values once each. `TestDiscriminateEnvironmentCarriesThePatchedGOROOT`: the environment a functional observation's `Prepare` builds under, and the one `environment` gives an interop child, hold `GOROOT=<tree>` and exactly one `-overlay`, naming the observation file. Then the `./le` bootstrap, a gokrazy build, a Docker image build, the QEMU guest and the CI workflows each run green once with the change, and golangci-lint, staticcheck and govulncheck each run once on `internal/core/eap` through `./le go lint` and `./le verify deps vulnerability` and pass with the marker referenced (A-10 under (A)) |
| AC-21 | the patch no longer applies to the GOROOT file (a fixture GOROOT whose `handshake_server.go` differs in a context line) | `stdpatch` refuses with an error naming the Go version, the patch file, git's output and the page section; no tree is written and no temporary directory is left. Proof: `TestStdpatchRefusesAPatchThatDoesNotApply`; plus `TestStdpatchTreeIsContentAddressed` (same inputs, same path; a changed GOROOT file, GOROOT path or layout constant, a new path) and `TestStdpatchConcurrentBuildsAnswerOneTree` (two builds racing for one key answer the same tree and leave no temporary directory) |
| AC-25 | the tree `stdpatch` builds for the real toolchain | `go list -e std cmd` under the tree answers the real GOROOT's package set with no package error, for each GOOS/GOARCH ze builds for; every `src` directory is a real directory; the embedded files, the patched file and the marker file are regular files and every other `src` file a symlink into the real GOROOT; `go env GOROOT` answers the tree and `go env GOVERSION` the real version; an inherited `GOROOT` under `cache/stdpatch/` does not change the answer (step 1; the inherited and the answered paths compared after `filepath.EvalSymlinks`, since `cache/` is a link to `~/.cache/ze`); the copied embed set is the union over `CGO_ENABLED=0` and `1` and over the five targets of step 4.3, and each of those ten environments lists `std cmd` under the tree with no package error. Proof: `TestStdpatchTreeServesTheWholeStdlib`, red against the round-1 shape (symlinked directories) and against a tree whose embedded files are symlinks |
| AC-26 | a standard-library patch whose `Ze-Reviewed-Against` stamp differs from the `GOVERSION` the tree is built from, while its diff still applies (a fixture GOROOT whose `VERSION` says go1.27.3 and whose `handshake_server.go` is go1.27.2's); and a patch with no stamp | the generator refuses at step 1a, before `git apply`, naming both versions, the patch file and the re-review step; no tree and no temporary directory is written; `Activate` sets none of the three values; `./le go patch-stamps check` exits non-zero with the same versions; with the stamp equal to the version, both pass. Proof: `TestStdpatchRefusesAStaleReviewStamp` (three subtests: stale, missing, equal), `TestPatchStampsCheckRefusesAStaleToolchainStamp`; `stage("go patch-stamps", "check")` present in both verify modes, by `internal/le/verify/engine` `stages_structural_test.go` |
| AC-28 | `ze appliance build <name>` (ze-setup binary) run with an environment that lacks any one of: `GOROOT` set, `GOTOOLCHAIN=local`, `exec.LookPath("go")` resolving to `$GOROOT/bin/go`, the marker file `$GOROOT/src/crypto/tls/zz_ze_server_session_id.go` | `SetGokGoEnv` refuses before it sets GOMODCACHE or GOPROXY, with a message naming what it found, why the tree is needed (the page path), and ending with the command line `GOROOT="$(./le go stdpatch goroot)" && export GOROOT GOTOOLCHAIN=local PATH="$GOROOT/bin:$PATH" && ze appliance build <name>` and `./le build gokrazy` as the le route; with all four present it proceeds and sets no `GOTOOLCHAIN` itself; the LookPath check compares `filepath.Clean(path)` with `filepath.Clean(filepath.Join(GOROOT, "bin", "go"))`, so a `GOROOT` with a trailing slash or a `//` is not refused (design round 5, NOTE 1); `internal/appliance` imports no `internal/le` package outside its tests. The three existing tests this refusal reaches are REWRITTEN, never deleted (design round 5, ISSUE 1; each is an expectation change that follows from the owner-approved Q-5 (A) design, not a weakened proof): `TestRunRefusesAnOldGoBeforeTheKernel` provides a fixture tree first (`GOROOT` holding the marker file and the fake `bin/go` answering go1.26.3, `GOTOOLCHAIN=local`, `PATH=$GOROOT/bin`) so `SetGokGoEnv` accepts and the run still reaches `CheckGoToolchain`, and it still asserts the old-Go refusal names both versions before the kernel resolves; `TestGokBuildsWithTheHostToolchain` keeps le's pin `GOTOOLCHAIN=go1.27.0` and asserts `runGokInProcess` now refuses, names `GOTOOLCHAIN` and ends with the command line, and never calls `gokExecuteFn` (the 2026-10-10 arm64 failure it PREVENTS is now stopped by the refusal instead of an override), plus an accepting arm over the fixture tree in which gok runs with a `GOTOOLCHAIN=local` it did not set; `TestGokExitReachesStderr` runs its child under a fixture tree's environment (`GOROOT`, `GOTOOLCHAIN=local`, `$GOROOT/bin` first on PATH, set in `cmd.Env`), so the child reaches the fake gok whatever environment launched the test and still exits 1 with gok's line on stderr, instead of exiting 4 at the refusal. Proof: `TestSetGokGoEnvRefusesWithoutThePatchedTree` (four refusing subtests, one accepting, over a fixture GOROOT, and one accepting with a trailing-slash `GOROOT`), `TestApplianceMarkerPathMatchesThePatch`, and the three rewritten tests |
| AC-29 | every shipped build flavour, `ze_setup` included | `go list -deps` of `cmd/ze` under each shipped flavour's tags contains no `internal/le` package, so an `internal/le` import from `internal/appliance` or any ze-setup package can never land. The flavour set is DERIVED, never named by hand (design round 5, NOTE 1): the test globs `cmd/ze/build_tag_*_test.go`, parses each file's `//go:build` line with `go/build/constraint`, and takes as the flavour's tags the identifiers the expression requires true (today `ze_core ze_appliance`, `ze_core ze_distro`, all four, and `ze_setup`), plus the feature tags `personalityFeatureTags` already adds; a matched file with no constraint, or one whose positive tags fail to satisfy its own expression, fails the test, so a new flavour file joins the check without an edit. Proof: `TestNormalZeLinksNoInternalLe` extended from `normalZeTags` alone to one subtest per derived flavour. It is GREEN at HEAD (no shipped flavour links `internal/le` today), so it owns no red at HEAD; its discrimination is a scratch blank import of `internal/le/go/stdpatch` in `internal/appliance/cmd_build.go`, under which the `ze_setup` subtest goes red, recorded, then removed |
| AC-27 | a vendor patch whose `Ze-Reviewed-Against` module version differs from `vendor/modules.txt` (a fixture tree), a vendor patch with no stamp, one naming a module the vendor directory lacks, a stale-stamped patch that is already applied, and a patch the set names that is missing on disk | `applyVendorPatches` refuses each before `git apply` and before the `--reverse --check` "already applied" probe, naming the module, both versions, the patch and the re-review step; `./le go patch-stamps check` refuses the same; with the versions equal both pass; the vendor patch set is the `patches/` directory, read once (`vendorpatch.Patches`). Proof: `TestVendorPatchRefusesAStaleReviewStamp` (`internal/le/setup`), `TestPatchStampsCheckRefusesAStaleVendorStamp`; the two shipped vendor patches carry the stamps of HEAD's `modules.txt` and the check is green on the tree |
| AC-22 | the authenticator, TLS 1.3 client | the NewSessionTicket RFC 9190 Section 2.1.2 requires still goes out, and a TLS 1.2 client receives none. Proof: the existing RFC 9190 Section 2.1.2 ticket test stays green, plus the no-NewSessionTicket assertion of AC-2 |
| AC-23 | the patch file, the patched hunk, the marker file, `stdpatch` and `newTLSMethod` | each carries the comment of D2 design "Comments the code owes": the page path and the RFC 5216 Section 2.1.1 quote. Proof: review gate; `./le rfc check` for the implementing-function quote |
| AC-24 | `docs/contributing/patching-the-standard-library.md` | after this spec, the page states the mechanism as built, with source anchors on `stdpatch` and the patch file, and no sentence saying it is not built yet; the `doc-links: ignore` markers the design phase put on not-yet-built paths are removed. Proof: `./le doc check verify`, `./le doc check links` |
| AC-3 | the `wrong` verdicts of RFC3948-2.1-2 and RFC5216-2.1.1-4, after this spec's producer fix | RFC3948-2.1-2 reaches `enforced`: tagged tests prove the quoted sentence at creation (AC-1, AC-6 to AC-9) and after a rebind (AC-11, AC-13, AC-14), and an agent that did not write those tests re-judges it with `./le rfc audit-stamp ... mode rejudge`. RFC5216-2.1.1-4, Q-1 answered (b): it reaches `enforced` the same way, through the ServerHello wire test `TestRFC5216TLS12FullHandshakeServerHelloSessionID` (tag RFC5216-2.1.1-4 positive, discrimination record from removing step 3's `GetConfigForClient`, under which the ticket-offering subtest goes red), and an independent re-judge. The summary row names the standard-library patch and the page. The two config-level 2.1.1-4 tags are withdrawn (TDD plan). Moved here from "Blocked by" in `plan/pre-release/spec-rfc-verdict-fix-ike-eap.md` (parent P-3, 2026-09-28) |
| AC-4 | D2 area, RFC5216-2.1.2-1 (weak), RFC 5216 Section 2.1.2: "If the EAP server is resuming a previously established session, then it MUST include only a TLS change_cipher_spec message and a TLS finished handshake message after the server_hello message." crypto/tls renews the session ticket on a TLS 1.2 resumption, so a NewSessionTicket follows server_hello | owner ruling 2026-09-29: ze's authenticator refuses TLS 1.2 resumption (a TLS 1.2 session is never redeemed; TLS 1.3 tickets stay, per RFC 9190). The untagged failing test `TestRFC5216ResumingServerSendsNothingButChangeCipherSpecAndFinished` (`internal/core/eap/rfc5216_resumption_defect_test.go`, uncommitted) is replaced by a tagged proof of the fix, and the verdict reaches `enforced` by an agent that did not write that test. The fix removes the TLS 1.2 resumed conversation that `TestRFC5216ResumedFlightsCarryOnlyChangeCipherSpecAndFinished` drives (RFC5216-2.1.1-10, 2.1.1-5), so those tags are re-proven or re-judged in the same work. Moved here from "Blocked by" in `plan/pre-release/spec-rfc-verdict-fix-ike-eap.md` (P-3) |

| AC-4 status (2026-10-13) | - | CLOSED at HEAD: producer `37dbe54095`, independent re-judge to `enforced` in `c1f83ffa35` and `96535408fa` for RFC5216-2.1.2-1, 2.1.1-5 and 2.1.1-10. Nothing further owed here |
| AC-5 | Ze responder or initiator, NAT detected, peer's authenticated IKE endpoint is (A, 4500) | both states keep encap 4500/4500 (no-translation polarity). Proof: `TestRFC3948PeerOn4500KeepsPort4500` (unit) and the existing scenarios `real-nat-tunnel-control`, `real-nat-transport-ze-responder`, `real-nat-transport-ze-initiator` staying green |
| AC-6 | EAP responder: final IKE_AUTH (AUTH-from-MSK) from (A, Y) | Child SA installed with sport/dport Y as AC-1; `sa.peerEndpoint` is set before `buildAuthResponse` runs. Proof: `TestRFC3948EAPResponderChildSAEncapsulatesToTheIKEPeerPort`, interop `real-nat-port-translated-eap-ze-responder` |
| AC-7 | Ze initiator, authenticated IKE_AUTH response arrives from (B, Y) | first Child SA installed with dport Y (outbound) / sport Y (inbound). Proof: `TestRFC3948InitiatorChildSAEncapsulatesToTheIKEPeerPort` |
| AC-8 | a CREATE_CHILD_SA Child SA and a rekeyed Child SA on an IKE SA whose endpoint is (A, Y) | both installed with Y, taken from the IKE SA (not copied from the retired child). Proof: `TestRFC3948CreatedAndRekeyedChildSATakeTheIKESAPorts` |
| AC-9 | Ze's NAT-T socket bound to a port L other than 4500 (test-port hook) | local encap port is L on both states. Proof: `TestRFC3948LocalEncapPortIsTheNATTSocketPort` |
| AC-10 | (Q-2 = A′) a Child SA that needs UDP encapsulation and whose IKE SA has no authenticated endpoint, on each path through `childEncapPorts`: responder IKE_AUTH (`createFirstChildSA` from `responder.go`), initiator IKE_AUTH (`initiatorFirstChildSA` from `runEstablished`), CREATE_CHILD_SA (ze initiating or responding), Child SA rekey (ze initiating or responding), each with and without MOBIKE | `childEncapPorts` refuses with an error naming the peer and the reason; no SA or policy installed. MOBIKE refuses the same way: its remote port comes from `sa.peerEndpoint`, never from `remoteUDPAddr`, whose nil-endpoint fallback is the CONFIGURED remote (`sa.go`, `remoteUDPAddr`), the very default A′ removes. Paths where ze ANSWERS (responder IKE_AUTH, CREATE_CHILD_SA or rekey ze answers) refuse before anything is sent: the childless response `buildChildlessAuthResponse`, or the error notify `ps.respondError(..., notifyForRefusal(err), ...)`, and the IKE SA stays up. Paths where ze INITIATED (initiator IKE_AUTH, CREATE_CHILD_SA or rekey ze initiated) cannot refuse before sending, because the endpoint is adopted from the authenticated response itself; by then the peer has installed its half, so ze sends an INFORMATIONAL Delete listing its OWN inbound ESP SPI, the SPI the peer puts in the headers of the packets it sends to ze, because RFC 7296 Section 1.4.1 says what a Delete names: "To delete an SA, an INFORMATIONAL exchange with one or more Delete payloads is sent listing the SPIs (as they would be expected in the headers of inbound packets) of the SAs to be deleted." The peer then closes that SA and its pair ("The recipient MUST close the designated SAs."). Ze installs nothing, logs the error with the peer, keeps the IKE SA (RFC 7296 Section 2.21.2: "This failure does not automatically cause the IKE SA to be deleted."), and a rekey leaves the old Child SA in use. For the initiator IKE_AUTH this replaces today's `return err` from `runEstablished` on a child failure only for this refusal. No production path reaches the refusal after D1b (A-1); the Delete is what keeps the peer's state honest if one ever does. Proof: `TestRFC3948ChildSARefusesEncapWithoutAnIKEPort`, one subtest per path and MOBIKE state; each initiated-path subtest asserts the Delete payload carries ze's inbound SPI of the refused pair, as RFC 7296 Section 1.4.1 names it (negative polarity of RFC3948-2.1-2). The session-level subtests live in `TestRFC3948ChildSARefusalDeletesThePeersHalf` (same file, untagged), because the tagged unit's body is not on the Q-4 approval list |
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
| A TLS 1.2-only EAP-TLS client without RFC 7627 completes the TLS handshake against a ze built with the patch, and ze refuses the MSK with a log line naming the fix | strongSwan client `version_max = 1.2` → ze authenticator | `responder-eap-tls12` |
| A contributor builds ze with plain `go build`, then under the patched GOROOT | shell → go | the "undefined" failure, then success after the page's three exports (AC-20) |

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
| `TestRFC3948ChildSARefusesEncapWithoutAnIKEPort` | same | AC-10 (Q-2 = A′), the no-endpoint case only, one subtest per path and MOBIKE state: responder IKE_AUTH, initiator IKE_AUTH (`initiatorFirstChildSA`), CREATE_CHILD_SA initiated and answered, rekey initiated and answered; the initiated paths assert an INFORMATIONAL Delete listing ze's inbound ESP SPI (RFC 7296 Section 1.4.1) and the IKE SA kept, in the session-level `TestRFC3948ChildSARefusalDeletesThePeersHalf`; tag RFC3948-2.1-2 negative | to write, must fail at HEAD |
| `TestRFC3948ChildSAFollowsANATMappedPeerPort500` | same | AC-10b: authenticated endpoint (A, 500), NAT detected, Ze on its NAT-T socket; both states carry the peer port 500 and Ze's socket port; no refusal; tag RFC3948-2.1-2 positive | to write, must fail at HEAD (ports 4500/4500) |
| `TestRFC3948RebindMigratesTunnelChildSAs` | `internal/component/ike/engine/rfc3948_rebind_test.go` (new) | AC-11: a recording `TunnelMigrator` sees one `MigrateTunnel` per installed pair (current, distinct superseded) with `RemotePort` Y2; no CREATE_CHILD_SA is sent; tag RFC3948-2.1-2 positive | to write, must fail at HEAD (no migration on a non-MOBIKE move) |
| `TestRFC3948RebindRekeysWithoutAMigrator` | same | AC-13: a backend with no `TunnelMigrator`; the next tick sends a Child SA rekey, and the rekeyed pair's `SAParams` carry Y2; tag RFC3948-2.1-2 positive | to write, must fail at HEAD |
| `TestRFC3948RebindRekeysTransportChildSA` | same | AC-14: transport mode with a migrator present; the migrator is never called and the rekey runs | to write, must fail at HEAD |
| `TestRFC3948RebindRekeysWhenMigrationRefused` | same | AC-15, two subtests: the migrator returns an `ErrNotSupported`-wrapped error, and an error that wraps no sentinel (the shape of the local-port refusal); in both the old endpoints stand, the row 3 log line is emitted and the rekey follows | to write, must fail at HEAD |
| `TestRFC3948RebindRetiresALostChildSA` | same | AC-16: the migrator returns `ErrTunnelMigrationLost`; the IKE SA is `StateDead` with reestablish, no rekey, no reinstall | to write, must fail at HEAD |
| `TestRFC3948RebindNotRunWithoutAnAdoptedMove` | same | AC-17, four subtests (same endpoint, MOBIKE, behind a NAT, unauthenticated); tag RFC3948-2.1-2 negative | to write; green at HEAD by design (polarity guard) |
| `TestRFC3948RebindRekeyOwedSurvivesAHold` | same | AC-18: held window, TEMPORARY_FAILURE hold and refusal hold each defer, then the rekey runs; the flag clears on a peer-initiated rekey too | to write, must fail at HEAD |
| `TestRFC3948RebindOwedRekeyClearsOnPromotion` | same | AC-18, two subtests: a move while a pending child is set marks the rekey owed, then promotion clears the flag and no owed rekey is sent on the next tick; a reaped unauthenticated pending SA leaves the flag set and the owed rekey runs | to write, must fail at HEAD |
| `TestRFC5216TLS12FullHandshakeServerHelloSessionID` | `internal/core/eap/rfc5216_session_id_test.go` (new) | AC-2: three subtests (client offers the ticket extension; client offers none; a raw ClientHello with legacy_version 0x0304, the ticket extension and no supported_versions extension) drive a TLS 1.2 full handshake through `tlsMethod`; each reads the ServerHello session_id (32 octets, not the client's) and finds no NewSessionTicket in the server flight; tag RFC5216-2.1.1-4 positive (moved here from the two config-level tags) | to write, must fail at HEAD (length 0) |
| `TestPatchedCryptoTLSServerSessionID` | `internal/le/go/stdpatch/gen/crypto_tls_session_id_test.go` (new) | AC-19: ticket issued → empty id; no ticket → 32 octets; TLS 1.3 → legacy echo unchanged | to write; red when run under a tree that carries the marker but not the hunk |
| `TestStdpatchRefusesAPatchThatDoesNotApply` | `internal/le/go/stdpatch/gen/stdpatch_test.go` (new) | AC-21: fixture GOROOT with a changed context line; error names version, patch, page; no tree written, no temporary directory left | to write |
| `TestStdpatchTreeIsContentAddressed` | same | AC-21: same inputs answer the same path; a changed GOROOT file, a changed GOROOT path and a changed layout constant each answer a new one | to write |
| `TestStdpatchConcurrentBuildsAnswerOneTree` | same | AC-21: two builds racing for one key (step 4.6) answer the same tree; the loser's temporary directory is gone | to write |
| `TestStdpatchTreeServesTheWholeStdlib` | same | AC-25: over the real toolchain, `go list -e std cmd` under the tree equals the real set with no package error for each GOOS/GOARCH ze builds for; directory, embed-copy, patched-file, marker and symlink shape; `go env GOROOT` and `GOVERSION`; an inherited stdpatch `GOROOT` ignored | to write; red against the round-1 shape and against symlinked embed files |
| `TestToolchainOverridesLeaveTheToolchainToTheProcess` | `internal/le/go/toolchain/gotoolchain_test.go` | AC-20: `Overrides` holds no `GOROOT`, no `GOTOOLCHAIN` and no GOFLAGS; `Environment` carries the activated process's `GOROOT`, `GOTOOLCHAIN=local` and PATH once each; `New` returns the activation's refusal | to write, must fail at HEAD (`GOTOOLCHAIN=<pin>` in `Overrides`) |
| `TestActivateSetsTheProcessEnvironment` | `internal/le/go/stdpatch/gen/stdpatch_test.go` | AC-20: the three values after `Activate`; one PATH entry after a second `Activate` under the inherited tree; a refusal sets none and is returned again | to write |
| `TestLeChildInheritsThePatchedGOROOT` | `internal/le/register_test.go` | AC-20: `run` activates before dispatch; a child's `go env GOROOT` and `exec.LookPath("go")` answer the tree, compared after `filepath.EvalSymlinks` | to write, must fail at HEAD (child sees the real GOROOT) |
| `TestStdpatchRefusesAStaleReviewStamp` | `internal/le/go/stdpatch/gen/stdpatch_test.go` | AC-26: stale stamp with an applying diff, missing stamp, equal stamp | to write |
| `TestPatchStampsCheckRefusesAStaleToolchainStamp`, `TestPatchStampsCheckRefusesAStaleVendorStamp` | `internal/le/go/patchstamps/patchstamps_test.go` | AC-26, AC-27: the check over fixture trees, both families, red and green | to write |
| `TestVendorPatchRefusesAStaleReviewStamp` | `internal/le/setup/install_test.go` | AC-27: `applyVendorPatches` refuses before `git apply` on a stale, missing or unknown-module stamp; a stale-stamped patch already applied refuses (stamp before the `--reverse --check` probe); a missing patch file refuses instead of being skipped; each refusal ends with `./le setup install` | to write, must fail at HEAD (no stamp read; the missing file is skipped) |
| `TestSetGokGoEnvRefusesWithoutThePatchedTree` | `internal/appliance/cmd_build_test.go` | AC-28: refuses with no `GOROOT`, with `GOTOOLCHAIN` other than `local`, with `go` on PATH outside `$GOROOT/bin`, and with no marker file, each before GOMODCACHE or GOPROXY is set and each message ending with the command line; accepts a fixture GOROOT holding all four and sets no `GOTOOLCHAIN` | to write, must fail at HEAD (`SetGokGoEnv` checks nothing) |
| `TestApplianceMarkerPathMatchesThePatch` | `internal/appliance/cmd_build_test.go` | AC-28: the marker path constant equals the file the `crypto/tls` patch adds (read through `stdpatch`, a test-only import) | to write |
| `TestNormalZeLinksNoInternalLe` (extended) | `cmd/ze/ze_le_personality_test.go` | AC-29: one subtest per shipped flavour, the set derived from the `//go:build` constraints of `cmd/ze/build_tag_*_test.go` (`go/build/constraint`), `ze_setup` included, none linking `internal/le` | to extend; green at HEAD, so no red to own; discriminated by a scratch `internal/le/go/stdpatch` blank import in `internal/appliance/cmd_build.go`, red recorded, import removed |
| `TestRunRefusesAnOldGoBeforeTheKernel` (rewritten, design round 5) | `internal/le/build/gokrazy/gotoolchain_test.go` | AC-28: with a fixture tree in place first (`GOROOT` holding the marker and the fake `bin/go` answering go1.26.3, `GOTOOLCHAIN=local`, `PATH=$GOROOT/bin`), `SetGokGoEnv` accepts and the old-Go refusal from `CheckGoToolchain` still names both versions before the kernel resolves | to rewrite: turns red under AC-28 as written at HEAD (clears `GOTOOLCHAIN`, fake go outside any `GOROOT`, so the tree refusal wins); expectation change from the owner-approved Q-5 (A) design |
| `TestGokBuildsWithTheHostToolchain` (rewritten, design round 5) | `internal/appliance/gok_toolchain_test.go` | AC-28: under le's pin `GOTOOLCHAIN=go1.27.0`, `runGokInProcess` refuses, the message names `GOTOOLCHAIN` and ends with the command line, and `gokExecuteFn` is never called; over a fixture tree with `GOTOOLCHAIN=local`, gok runs and sees `local` | to rewrite: at HEAD it asserts the `os.Setenv("GOTOOLCHAIN", "local")` AC-28 deletes; expectation change from the owner-approved Q-5 (A) design |
| `TestGokExitReachesStderr` (rewritten, design round 5) | `internal/appliance/gok_stderr_test.go` | AC-28: the child runs under a fixture tree's environment set in `cmd.Env` (`GOROOT`, `GOTOOLCHAIN=local`, `$GOROOT/bin` first on PATH), so it reaches the fake gok and still exits 1 with gok's line on stderr | to rewrite: without the tree the child exits 4 at the refusal; expectation change from the owner-approved Q-5 (A) design |
| `TestDiscriminateEnvironmentCarriesThePatchedGOROOT` | `internal/le/rfc/` (beside the `writeOverlay` tests) | AC-20: the `Prepare` environment of a functional observation and an interop child's `environment` hold `GOROOT=<tree>` and exactly one `-overlay`, naming the observation file | to write, must fail at HEAD (no `GOROOT`) |

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
| `internal/core/eap/rfc5216_flight_content_test.go::TestRFC5216AuthenticatorAnswersATLS12TicketWithAFullHandshake` | RFC5216-2.1.2-1 positive | found during D2 (iii): ze's authenticator no longer issues a TLS 1.2 ticket (Q-1 (b), `tls12WithoutTickets`), so the minting exchange runs under a new `issueTLS12Tickets` tune (GetConfigForClient removed) that issues one under ze's ticket keys; the attempt that offers it still runs against the production authenticator, every assertion unchanged | Q-1 (b) |
| `internal/core/eap/rfc5216_flight_content_test.go::TestRFC5216ResumedFlightsCarryOnlyChangeCipherSpecAndFinished` | RFC5216-2.1.1-10 positive, RFC5216-2.1.1-5 positive | found during D2 (iii): the shared `stockTLS12Resumption` tune also removes GetConfigForClient, so the test server it builds still issues and redeems TLS 1.2 tickets; the final attempt against the production authenticator and every assertion unchanged | Q-1 (b) |
| `internal/component/ike/engine/rfc7296_test.go::TestChildSANoNATNoEncap` (UNCHANGED but affected) | RFC7296-2.23-2 negative | no edit planned, but the implementer MUST decide its fate on purpose: it asserts ports == 0 without NAT, so if `newChildSA` fills the ports for every SA (Phase 3) and `installChildSA` no longer zeroes them when UDPEncap is false, it goes red. Its claim "ESP traffic does NOT float" is stronger than Section 2.23 ("Either side can decide whether or not to use UDP encapsulation for ESP irrespective of the choice made by the other side."); if its body changes, the prose presents no-encap-without-NAT as Ze's choice, not the RFC's, and the edit joins this Q-4 list | Q-2 = A′ and Phase 3's port fill |

Untagged test that changes under every answer: `TestMobikeMappedPortSurvivesChildRekey` (`mobike_test.go`) exercises the rekey copy of `udpLocalPort`/`udpRemotePort` that D1c replaces; its fixture already moves the IKE SA by a MOBIKE update before the rekey, so after D1c its assertions hold because the IKE SA carries the mapped port, not because the retired child did. The implementer re-reads its comments and assertions, rewrites any that name the copy, and keeps it green. Untagged tests that change with Q-2 = A′: `TestRekeyKeepsBothESPFormAcceptance` (`rfc7296_rekey_collision_test.go`), and any `mobike_test.go` fixture that installs a Child SA without an endpoint (none found that assert ports beyond MOBIKE's own). Every changed tagged unit is re-recorded with `./le rfc discriminate-record`; records for RFC3948-2.1-2, RFC7296-2.23-2/-9/-10/-11 and RFC5216-2.1.1-4 go stale and their verdicts are re-judged by an agent that did not write the tests (`./le rfc audit-stamp ... mode rejudge`).

#### Owner approval commands

Approved by the owner on 2026-10-13 (Q-4). The gate reads approvals from the committing session's own file (`tmp/commit-rfc-approved-<session>.md`), and a row is used up by the commit that carries its `RFC-approved:` trailer. So the owner runs these at the prompt of the session that implements and commits the edits, and runs a unit's command again if a later commit changes that unit a second time. A unit is `<package directory>.<top-level test function>`; every name was checked to exist at the HEAD named under "Status at HEAD". The gate compares each tagged function's own text: if only `bfmInstall` changes and a `TestBfm*` body stays byte-identical, that unit is not asked for and its row goes unused. The last command applies only if Phase 3 changes `TestChildSANoNATNoEncap`. The new rebind tests (AC-11, AC-13 to AC-18) and the new negative test are new units and need no approval.

12 commands (the last two eap ones added during D2 (iii)):

```
./le rfc approve unit engine.TestChildSANATTEncapPorts reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-ike-eap-rfc-defects, fixture gains authenticated endpoint and NAT-T port, ports from the IKE SA, tag claims rewritten)'
./le rfc approve unit engine.TestEncapNeverRequestedOnPort500 reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-ike-eap-rfc-defects, fixture gains an authenticated endpoint swept over a 4500 peer and a NAT-mapped peer on 500, assertion narrowed to the own encap port, claim and comments rewritten)'
./le rfc approve unit engine.TestBfmEncapsulatedESPAcceptedWithoutNAT reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-ike-eap-rfc-defects, bfmInstall gains an authenticated endpoint)'
./le rfc approve unit engine.TestBfmEncapsulatedESPSentWhenNATDetected reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-ike-eap-rfc-defects, bfmInstall gains an authenticated endpoint)'
./le rfc approve unit engine.TestBfmBareESPKeptForUnfloatedSA reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-ike-eap-rfc-defects, bfmInstall gains an authenticated endpoint)'
./le rfc approve unit engine.TestBfmBothESPFormsAreReachable reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-ike-eap-rfc-defects, bfmInstall gains an authenticated endpoint)'
./le rfc approve unit engine.TestBfmBothESPFormsReceivedOnOneChildSA reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-ike-eap-rfc-defects, bfmInstall gains an authenticated endpoint)'
./le rfc approve unit eap.TestRFC5216TLSConfigBothRolesInstallWhatTheFlightsCarry reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-ike-eap-rfc-defects, RFC5216-2.1.1-4 positive tag line withdrawn)'
./le rfc approve unit eap.TestRFC5216TLSConfigRefusesWhatWouldBreakTheFlights reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-ike-eap-rfc-defects, RFC5216-2.1.1-4 negative tag line withdrawn)'
./le rfc approve unit eap.TestRFC5216AuthenticatorAnswersATLS12TicketWithAFullHandshake reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-ike-eap-rfc-defects Q-4); the authenticator issues no TLS 1.2 ticket under Q-1 (b), so the minting exchange runs under issueTLS12Tickets; the attempt and its assertions are unchanged'
./le rfc approve unit eap.TestRFC5216ResumedFlightsCarryOnlyChangeCipherSpecAndFinished reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-ike-eap-rfc-defects Q-4); stockTLS12Resumption also removes GetConfigForClient so the test server still issues TLS 1.2 tickets under Q-1 (b); assertions unchanged'
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
| `real-nat-port-translated-ze-responder` (new) | `test/interop-ipsec/scenarios/real-nat-port-translated-ze-responder/` | strongSwan 5.9.14 initiator (PSK, tunnel, `dpd_delay` set in `swanctl.conf`) behind the NAT box; nat.conf row maps its UDP 4500 to public port 14500 with SNAT only, no DNAT | AC-1, AC-5 (other scenarios), then the rebind (AC-11, row 1). Phase one: ze's outbound state reads `encap type espinudp sport 4500 dport 14500`; ping across the tunnel both ways; strongSwan's ESP inbound byte counter rises. Rebind step: the checker replaces the NAT box's port SNAT so UDP 4500 maps to public port 14501, a DIFFERENT port, then flushes the NAT box's UDP conntrack entries with `conntrack -D -p udp` (A-8). The flush is needed because iptables NAT rules apply only to the first packet of a flow: strongSwan's NAT keepalives and DPD on its 4500 flow keep the old conntrack entry alive, so without the flush the swapped SNAT never applies. strongSwan's next DPD INFORMATIONAL is then an authenticated IKE message from (public, 14501), which reaches `adoptAuthenticatedEndpoint` (a NAT keepalive never does). The step needs no branch prediction: `Suite.Run` runs the Docker host kernel check before any image build and refuses a kernel on which any enrolled capability, `ipsec-mobike` included, is not present (R-11), so wherever this step runs `xfrm_linux.go` has loaded the migrator and a tunnel Child SA takes row 1. The checker waits for the row 1 log line (Rebind design, "One log line per branch") and fails when any other row's line appears, then asserts ze's outbound state reads dport 14501 on the unchanged SPI (migrated, not rekeyed) and ping works both ways. The scenario's `swanctl.conf` writes `reauth_time = 0s` and `rekey_time = 0` at both connection and child level, explicitly, as the existing mobike-* and responder-*-rekey scenarios do. `reauth_time = 0s` means no parallel IKE_SA_INIT re-authentication is set up during the step: Ze sets a pending child only when a parallel re-initiation authenticates (`finishResponderEstablish`), never for an IKE rekey sent as CREATE_CHILD_SA. The child-level `rekey_time = 0` keeps a Child SA rekey, which would change the SPI, from failing the same-SPI assertion; the connection-level one keeps the run free of IKE rekeys, which leave the child SPIs alone but add exchanges the checker does not expect. Were a parallel IKE SA set up anyway, ze logs the row 2 line with `reason` "a new IKE SA with the peer is being set up" and the step fails naming it, never passing on row 2's evidence. Red at HEAD expected for both phases: ESP to public:4500, then to public:14500, has no conntrack entry and is dropped | written 2026-10-14, NOT RUN: the Docker host kernel check refuses this host (R-11); first run and the revert-rebuild-red walk for each phase owed. Topology as built: Ze is NOT translated (a translated Ze is behind a NAT and adopts no moved endpoint, AC-17), so strongSwan reaches 172.28.0.2 by a host route via its public 172.28.0.7, which `natPortEstablish` installs before it initiates (`start_action = none`) |
| `real-nat-port-translated-transport-ze-responder` (new) | `test/interop-ipsec/scenarios/real-nat-port-translated-transport-ze-responder/` | strongSwan 5.9.14 initiator (PSK, transport, `dpd_delay` set), same NAT row and rebind step | AC-14, and the owed-rekey path of row 2 on the wire (on the same hosts as every interop-ipsec scenario, R-11): after the remap ze logs the row 2 line with `reason` exactly "transport mode" (the checker compares the attribute, so the pending-IKE-SA reason cannot satisfy it) and never the row 1 line, rekeys the Child SA (new SPI), never migrates, and the new outbound state reads dport 14501; traffic flows both ways | written 2026-10-14, NOT RUN (R-11); first run and walk owed; same topology as the tunnel scenario |
| `real-nat-port-translated-eap-ze-responder` (new) | `test/interop-ipsec/scenarios/real-nat-port-translated-eap-ze-responder/` | strongSwan 5.9.14, EAP-TLS 1.3 client (as `responder-eap-tls13`), same NAT | AC-6 | written 2026-10-14, NOT RUN (R-11); first run and walk owed; same topology as the tunnel scenario |
| `responder-eap-tls12` (new) | `test/interop-ipsec/scenarios/responder-eap-tls12/` | strongSwan 5.9.14 EAP-TLS client at its shipped `version_max = 1.2` | AC-2 goal validation, re-scoped by Thomas on 2026-10-21 ("ok to both"): a TLS 1.2 strongSwan client completes the handshake through Ze's Finished against the patched build with TLS 1.2 tickets suppressed, Ze refuses to export the RFC 5216 Section 2.3 MSK without RFC 7627, and the IKE SA does not authenticate. EMS stays required, because no IKEv2 EAP-TLS initiator sends it (row "peer with RFC 7627 EMS" below). charon accepts an empty and a chosen id alike and is not known to log it (A-5), so this scenario cannot go red on the id: its revert walk is recorded as "no discriminating red reachable from the peer", and the id is proven by the unit test. It is the regression proof that the patch and the ticket suppression keep TLS 1.2 interoperating | written 2026-10-14 (b21fb1de8e; `checkResponderEAPTLS12` in `internal/le/interoplab/ipsec/natport.go` first asserted an authenticated tunnel; since 2026-10-21 it asserts, numbered: charon `negotiated TLS 1.2`, charon `received TLS Finished handshake`, charon `received EAP_FAILURE, EAP authentication failed`, Ze's refusal facts, and no XFRM state on either peer). D2 has landed, and the lab cross-compiles ze from le (`stageBinary`, `internal/le/interoplab/zebuild.go`), so the build inherits the patched GOROOT. NOT RUN on 2026-10-14 (R-11): the Docker kernel on this host refuses every interop-ipsec scenario, and an install is pending. It runs on a host whose kernel the lab accepts |
| `real-nat-tunnel-control`, `real-nat-transport-ze-responder`, `real-nat-transport-ze-initiator` (existing) | `test/interop-ipsec/scenarios/` | strongSwan 5.9.14 | AC-5: unchanged 4500 behaviour stays green | existing |

#### Runs of 2026-10-21 (Docker lab in the Ze-kernel QEMU guest, `git archive` export of `5785852cbb` plus the two fixture fixes below)

| Scenario | Result | Evidence (scratch logs summarised) |
|----------|--------|------------------------------------|
| `real-nat-port-translated-ze-responder` | GREEN 19:03, phase one and the rebind (row 1, migrated) | first run 18:47 red at the rebind step only: the scenario had no `ze-env`, and the lab-wide `ZE_LOG_LEVEL=debug` names a subsystem "level", so the ike subsystem stayed at WARN and the INFO rebind line was never printed (known trap, `plan/journal/silent-fall-through.md`). Fixed by `ze-env` `ZE_LOG_IKE=info` |
| `real-nat-port-translated-transport-ze-responder` | RED, product defect | phase one green; the rebind takes row 2 with `reason="transport mode"` as designed; the owed rekey is refused. With `pfs enable` charon answered NO_PROPOSAL_CHOSEN (its `esp_proposals = aes256gcm16` has no KE; every other rekey scenario sets `pfs disable`, now so here). With `pfs disable` charon answers TS_UNACCEPTABLE ("no acceptable traffic selectors found"): Ze's rekey request names the peer's NAT public address 172.28.0.7 (the substituted selector it installed, RFC 7296 Section 2.23.1), while charon behind the NAT holds 172.28.0.3. Ze retries every ~14 s and never moves the SA to 14501. AC-14 is not met on the wire |
| `real-nat-port-translated-eap-ze-responder` | GREEN 18:51 | dport 14500, traffic both ways |
| `responder-eap-tls12` (old checker) | RED, design defect | strongSwan 5.9.14 negotiates TLS 1.2 without RFC 7627 Extended Master Secret, so Ze refuses the MSK export. This scenario can never authenticate with strongSwan 5.9.14; re-scoped in the next row |
| `responder-eap-tls12` (re-scoped checker, `git archive` export of `a48a721a4c` plus the checker) | GREEN 22:07 | Ze logged `level=WARN msg="ike: EAP authentication failed" subsystem=ike peer=swan error="eap-tls: cannot export the RFC 5216 Section 2.3 MSK for peer CN=swan-eap-client on TLS 1.2. The export needs TLS 1.3, or a TLS 1.2 session that negotiated the RFC 7627 extended master secret. Move the peer to TLS 1.3 (RFC 9190), add RFC 7627 to its TLS 1.2 stack, or configure another EAP method: crypto/tls: ExportKeyingMaterial is unavailable when neither TLS 1.3 nor Extended Master Secret are negotiated"` (producer `eapTLS12ExportRefused`, `internal/core/eap/eap_tls.go`); charon logged `received TLS Finished handshake`, then `received EAP_FAILURE, EAP authentication failed`; no XFRM state on either peer |
| Walk: crypto/tls exports without EMS (a second stdpatch patch in the export deleting the `noEKMBecauseNoEMS` branch of `connectionStateLocked`, `conn.go`) | RED 22:10 as predicted | assertion 3 timed out: charon logged `EAP method EAP_TLS succeeded, MSK established`, `IKE_SA ze[1] established` and `CHILD_SA ze-child{1} established`. Export deleted |
| `real-nat-tunnel-control`, `real-nat-transport-ze-responder`, `real-nat-transport-ze-initiator` | GREEN 18:56, 18:58, 18:59 | AC-5 |
| Walk M1: `childEncapPorts` returns 4500/4500 | RED as predicted (tunnel scenario, phase one: `dport 14500` never appears) | 19:17 |
| Walk M2: `rebindChildSAs` returns at once | RED as predicted (tunnel scenario, rebind step; Ze logged the adoption 4 times, never the migration) | 19:21 |
| Walk M3: D1b adoption line in `handleResponderEAP` removed | STAYED GREEN | the earlier EAP rounds reach `handleOwnedInbound`, which adopts the endpoint after decrypt (`inbound.go`), so `sa.peerEndpoint` is already set when AUTH-from-MSK builds the Child SA. No interop red is reachable for D1b in this topology; the unit test carries it |
| `TestRFC3948InstalledStateCarriesTheIKEPeerPort` | PASS (not SKIP), `./le test qemu run` on runtime kernel `7.2.9-runtime-arm64-runtime-docker-387d6b76` | 19:31 |
| `real-nat-port-translated-transport-ze-responder` over `9d60c1d841` (the USE_TRANSPORT_MODE rekey fix), `git archive` export | GREEN 20:55, phase one and the rebind (row 2, rekeyed onto dport 14501) | AC-14 met on the wire. The export needs the ignored generated `gokrazy/docker-host/builddir/github.com/ze-software/ze/go.sum` copied in (else the docker-host image build fails, `GOPROXY=off`), and a checkout path short enough for the forwarded Docker socket (103 bytes): the run went through a symlink `~/.zt/ikw` to the scratch export |
| Walk M4: the request's USE_TRANSPORT_MODE hunk disabled in `initiateChildRekey` | RED 20:59 as predicted | ze logs "peer refused the rekey with TS_UNACCEPTABLE (38)" every ~14 s, charon logs "no acceptable traffic selectors found", dport 14501 never appears. Fix restored, export deleted |
| `responder-eap-tls12` peer with RFC 7627 EMS | NONE EXISTS | strongSwan 5.9.14 to 6.1.0 and master define only the constant `TLS_EXT_EXTENDED_MASTER_SECRET` (`src/libtls/tls.h`); `tls_peer.c` never sends it and `tls_crypto.c` `derive_master` uses the RFC 5246 seed; Libreswan's EAP-TLS has "no initiator code" (`programs/pluto/ikev2_eap.c`); OpenIKED: "The only supported EAP type is currently MSCHAP-V2"; `eapol_test` is an 802.1X supplicant over RADIUS. Decided by Thomas, 2026-10-21 ("ok to both"): keep EMS required and assert the refusal |

The nat.conf format gains an optional third column (public port for the peer's UDP 4500) read by `readNATConfig`; `natSetupScript` emits, for such a row, a port SNAT for UDP source port 4500 before the address SNAT and NO DNAT. The rebind step is a checker action in `checkers.go`: it replaces that port SNAT with one to a second public port and deletes the NAT box's UDP conntrack entries (A-8). `docs/architecture/testing/interop.md` documents the column and the step.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|------------|---------------------------------|----------|--------------|--------|
| A-1 | After D1b, every production Child SA build runs after `adoptAuthenticatedEndpoint` | read: `handleAuthRequest` adopts before `buildAuthResponse`; `fsm.go` adopts on StateEstablished before `runEstablished` calls `initiatorFirstChildSA`; CREATE_CHILD_SA and rekey run on an established SA | Q-2 (A′) refuses a Child SA in production | the three wiring unit tests plus interop | confirmed at unit level 2026-10-14 (`8233b4e305`): `TestRFC3948ResponderChildSAEncapsulatesToTheIKEPeerPort` (`handleAuthRequest`), `TestRFC3948EAPResponderChildSAEncapsulatesToTheIKEPeerPort` (`handleResponderEAP` via `handleInbound`), `TestRFC3948InitiatorChildSAEncapsulatesToTheIKEPeerPort` (`handleInbound` adoption, then `runEstablished`) each build the Child SA after the adoption and install it, no refusal; the interop half is NOT RUN |
| A-2 | The NAT-T socket's bound port is the port the kernel's ESP-in-UDP decap socket listens on | `transport/encap_linux.go` sets UDP_ENCAP on the NAT-T socket (read of the file owed at implementation) | D1d picks a port the kernel does not decap on | `TestRFC3948InstalledStateCarriesTheIKEPeerPort` plus a read of `encap_linux.go` | confirmed by reading 2026-10-14: `apply.go` calls `transport.EnableESPInUDP(natt.Conn())` (`encap_linux.go`, `UDP_ENCAP_ESPINUDP`) on the NAT-T socket the SA binds as `nattSocket`, whose port `localEncapPort` reads. `TestRFC3948InstalledStateCarriesTheIKEPeerPort` is written (`9448e8550d`) and NOT RUN here: it needs root for the network namespace |
| A-3 | go1.27.2 crypto/tls leaves the ServerHello session_id empty on every TLS 1.2 full handshake | grep of `handshake_server.go`: the only assignment is in `doResumeHandshake` | D2 is not a defect | `TestRFC5216TLS12FullHandshakeServerHelloSessionID` red at HEAD | validated 2026-10-14 by a scratch probe (`tmp/session/2026-10-13-9d2e6c21-74c1-4c42-b9ef-30145b95cfb1/scratch/overlay-probe/`): go1.27.2, TLS 1.2 full handshake, ServerHello session_id length 0 with tickets on and off; the unit test still proves it in the tree |
| A-4 | strongSwan 5.9.14 accepts ESP-in-UDP from ze's 4500 arriving through the NAT's conntrack reverse mapping | normal NAT-T operation; existing real-nat scenarios pass with an address NAT | the scenario fails for a lab reason | the scenario's green run | unvalidated |
| A-5 | charon does not log the TLS session_id it receives | not verified in strongSwan source | a stronger interop assertion is available for D2 | read `tls_peer.c` `process_server_hello` at implementation | **confirmed 2026-10-21**, strongSwan 5.9.14 `src/libtls/tls_peer.c` `process_server_hello` (fetched from the 5.9.14 tag): below TLS 1.3 it compares the received id with its own (`chunk_equals(this->session, session)`) to attempt resumption, and otherwise logs only `"negotiated %N using suite %N"` and stores the id (`this->session = chunk_clone(session)`); no DBG line prints the id, so the interop scenario cannot see it and the unit test carries the proof |
| A-6 | iptables `SNAT --to-source addr:port` for `-p udp --sport 4500` translates the port deterministically in the alpine NAT box | iptables semantics | the scenario cannot force port Y, or the rebind step's second port | the checker reads the port ze observed (ze log of the adopted endpoint) before asserting, and asserts it differs from the first | unvalidated |
| A-7 | strongSwan migrates every Child SA on a NAT rebind and rekeys one whose update is unsupported (`ike_sa.c::update_hosts` → `child_sa->update`, `rekey_child_sa` on `NOT_SUPPORTED`) | owner's answer to Q-3 and design research 2026-10-13; no strongSwan source is checked out on this host | the fallback rule differs from charon's; the owner is told, the design stays Ze's | read `src/libcharon/sa/ike_sa.c` and `child_sa.c` of strongSwan 5.9.14 at implementation | **confirmed 2026-10-21**, strongSwan 5.9.14 `src/libcharon/sa/ike_sa.c` `update_hosts` (fetched from the 5.9.14 tag): for every Child SA, `if (child_sa->update(child_sa, this->my_host, this->other_host, vips, has_condition(this, COND_NAT_ANY)) == NOT_SUPPORTED)` then `this->public.rekey_child_sa(...)`; `child_sa.c` `update` returns `NOT_SUPPORTED` when the kernel interface cannot update the SA. charon adopts a new remote only `if ... has_condition(this, COND_NAT_THERE)` (the peer behind a NAT), the same situation as Ze's "not behind a NAT" rule. ipsec-9 "The rebind" still calls the reading unverified and is corrected in the closure doc pass |
| A-8 | the NAT box image does not carry a conntrack tool today, so this spec adds one: `test/interop-ipsec/Dockerfile.nat` is `FROM alpine:3.21` and runs `apk add --no-cache iproute2 iptables tini`, with no conntrack-tools; the spec adds `conntrack-tools` to that `apk add` so the rebind step can run `conntrack -D -p udp` | read of `test/interop-ipsec/Dockerfile.nat` at HEAD, 2026-10-14 | without the flush, the old UDP conntrack entry (kept alive by strongSwan's keepalives and DPD) keeps the old translation and the step forces no remap | the rebind step's first run shows the swapped port in ze's row log line | fact (the image half); the tool's behaviour in the box is validated by the step's first run |
| A-9 | strongSwan's `float_ports` (`ike_sa.c`) does not rewrite a remote port 500 on the responder side, so a NAT-mapped peer 500 reaches charon's Child SA install as 500 | owner context for Q-2 = A′; not read in strongSwan source | AC-10b still holds for Ze; only the claim that strongSwan behaves the same changes | read `ike_sa.c` `float_ports` of 5.9.14 at implementation | **confirmed 2026-10-21**, strongSwan 5.9.14: `float_ports` (`ike_sa.c`) does rewrite a remote 500 ("even if the remote port is not 500 (e.g. because the response was natted) we switch the remote port if we used port 500"), but its only caller is `ike_natd.c` `process_i`, the INITIATOR's IKE_SA_INIT handler (`float_ports` appears in no other fetched task, `ike_mobike.c` included). A charon responder therefore keeps the NAT-mapped remote port it observed, 500 included, as Ze does under A′ |
| A-10 | Every tool that type-checks ze honours `-overlay` in GOFLAGS: `go vet`, golangci-lint (go/packages), govulncheck, gopls | golangci-lint and gopls load through go/packages, which runs `go list` under GOFLAGS; not run against an overlay that adds a file | a tool reports `undefined: tls.ZeServerChoosesSessionID` on a patched setup | run each once under the overlay at implementation (D2 phase) | **BROKEN, probed 2026-10-14** (D2 design "A-10 probe"): `go vet` and `go list -compiled` honour the GOFLAGS overlay; golangci-lint 2.14.0, staticcheck and govulncheck fail opening the added file under GOROOT ("no such file or directory"); the round-1 gopls result (marker undefined) came from this host's shared `-remote=auto` gopls daemon, not from gopls itself (corrected in design round 4). The basis was wrong: go/packages runs `go list` under GOFLAGS, but these tools then read and hash each listed file from disk. The patched-GOROOT link tree passed every tool; gopls v0.23.0 run directly under the tree passed, and only the shared daemon, which keeps its starting client's environment, failed. Owner question Q-5, decided (A). Round 2 (2026-10-14): under the step 4 tree every tool passes, gopls included when the gopls binary runs with the tree's environment; round 1's gopls result came from this host's shared `-remote=auto` daemon, whose environment is its own |
| A-12 | Under `GOROOT=<tree>` and `GOTOOLCHAIN=local`, the go command and every go/packages tool use the tree, and no ze build path switches toolchain | read 2026-10-14: `execGoToolchain` (`cmd/go/internal/toolchain/exec.go`) sets `GOROOT` to the switched-to toolchain, so only `GOTOOLCHAIN=local` keeps the tree; `findGOROOT` resolves the executable through symlinks; probe: `go env GOROOT`, vet, run, golangci-lint, staticcheck, govulncheck under the tree | a site that sets its own `GOTOOLCHAIN` (or none, under `auto`, with a go.mod newer than the local go) switches and builds unpatched; the marker fails that build loudly | `TestActivateSetsTheProcessEnvironment`, `TestLeChildInheritsThePatchedGOROOT`, AC-20's one run per site | validated for the go command and the probed tools; the per-site runs are owed at implementation |
| A-13 | No tool ze runs locates the standard library from its own binary (`runtime.GOROOT()`, a build-time default) instead of the environment's `GOROOT` and the `go` on PATH | golangci-lint, staticcheck and govulncheck load through go/packages, which runs `go list` from PATH under the environment (probe passes); no production ze code calls `runtime.GOROOT()` (grep, 2026-10-14) | such a tool reads the unpatched standard library and reports the marker undefined, loudly (R-14) | AC-20's tool runs; any new tool added to a gate is run once under the tree | validated for the three probed tools |
| A-11 | `go build -overlay` adds a NEW file to a standard-library package, and a command-line `-overlay` replaces the one in GOFLAGS (one overlay per command). Under Q-5 (A) the standard-library patch uses no overlay, so this bears only on discrimination, whose single overlay is untouched | scratch probe 2026-10-14 (path in A-3): a program using the added constant built under the overlay and failed with "undefined" without it; `GOFLAGS=-overlay=A go run -overlay=B` built without A's file | the marker guard or the discrimination merge is wrong | probe | validated 2026-10-14 |

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
| R-8 | The standard-library patch reaches every ze TLS 1.2 server, not only EAP-TLS | a ze TLS client or peer that misbehaves on a non-empty id with no ticket | the hunk fires only when no ticket is issued (D2 design, "Reach"); `TestPatchedCryptoTLSServerSessionID` pins all three server rows |
| R-12 | A Go command path that compiles ze is missed by the GOROOT wiring | `undefined: tls.ZeServerChoosesSessionID` in that build | that error IS the mitigation for every binary that links `internal/core/eap`: the marker turns a missed path into a failed build, never a silent unpatched binary. A binary that does not link it (a helper program, a test binary of a package that does not import it) builds unpatched with no error, and its TLS 1.2 servers keep the empty id of row 2 of "Reach"; that is the unpatched upstream behaviour, not a defect this spec owns, because no EAP-TLS MUST applies there. The fix is to add the path to D2 design "Reach" |
| R-13 | A toolchain bump moves `doFullHandshake` | `stdpatch` refuses | the bump procedure on the page; upstream adoption deletes the patch (`ai/rules/no-layering.md`) |
| R-14 | A tool resolves the standard library from its own binary (`runtime.GOROOT()` or a build-time default) or starts a `go` that is not first on PATH, and so reads the unpatched GOROOT | that tool reports `undefined: tls.ZeServerChoosesSessionID` | loud by the marker, never a silent pass (A-13); the three gate tools were probed under the tree; the fix is to give that tool the environment of "Selection" |
| R-15 | A shared gopls daemon (this host's `~/bin/gopls` runs `gopls -remote=auto`) keeps the environment of the client that started it | gopls shows the marker undefined while `./le` builds pass | documented on the page: start gopls (or the daemon) with the tree's environment; navigation and `gopls symbols` are unaffected either way; ze's gates do not run gopls |
| R-16 | The real GOROOT changes under the tree: a toolchain upgraded or removed in place at the same path, while another session builds under the old tree | "no such file" errors from a build in flight; a dangling tree | the key carries `GOVERSION` and the target files' bytes, so the next call builds a new tree and never selects the old one; a build already running loses its GOROOT exactly as it would on any in-place toolchain upgrade. The tree and its temporary directory live under `cache/stdpatch/`, one filesystem, so the rename stays atomic; the symlinks into the real GOROOT may cross filesystems, which symlinks allow and hard links would not |
| R-17 | The standard library compiles once more per tree path (GOCACHE keys a non-trimpath build by directory) | one cold standard-library build after a tree is created | bounded: one per toolchain version and path, shared by every checkout through `cache/` |
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
- `internal/component/ike/engine/create_child.go` - new `childEncapPorts(sa)`: the ONE port derivation (remote from `sa.peerEndpoint`, local from the NAT-T socket; under MOBIKE local from `sa.mobike.local` and remote from `sa.peerEndpoint`, no longer `remoteUDPAddr`, whose nil-endpoint fallback is the configured remote, so MOBIKE refuses too) and the ONE place the Q-2 (A′) refusal (no authenticated endpoint only; a NAT-mapped peer port 500 is followed) lives, with the RFC 3948 Section 2.1 quote above it; `newChildSA` calls it for every Child SA (IKE_AUTH and CREATE_CHILD_SA), callers carry `// RFC 3948 Section 2.1`
- `internal/component/ike/engine/child.go` - `installChildSA`: drop the 4500 default and read the child's ports as built; it refuses nothing (the refusal is in `childEncapPorts`); rewrite the `UDPEncap` and `udpLocalPort` field comments and the port comments that state "Both ports below are the NAT-T constant"
- `internal/component/ike/engine/responder_eap.go` - `handleResponderEAP`: adopt the endpoint after `verifyRemoteAuth`, before `buildAuthResponse`
- `internal/component/ike/engine/responder.go` - `replayCachedResponse` comment on `peerEndpoint` during EAP
- `internal/component/ike/engine/established.go`, `create_child.go`, `rekey.go` (ze-initiated paths) - on a `childEncapPorts` refusal after the peer's response, send an INFORMATIONAL Delete for the peer's ESP SPI and keep the IKE SA (AC-10); `runEstablished` keeps the IKE SA for this refusal
- `internal/component/ike/engine/rekey.go` - `newRekeyedChild` takes the IKE SA and its ports from `childEncapPorts` (both rekey directions), so a refusal surfaces there too; `applyChildRekeyResponse` and `respondChildRekey` clear `ps.rebindRekeyOwed` through their callers when the rekeyed pair installs
- `internal/component/ike/engine/mobike.go` - `childMigrationAvailable` extracted from `mobikeAvailable`; `migrateMobikeChild` / `migrateMobikePair` take the local endpoint as a parameter (MOBIKE passes `sa.mobike.local`, the rebind the NAT-T socket's address), so MOBIKE and the rebind share one migration loop
- `internal/component/ike/engine/sa.go` - `adoptAuthenticatedEndpoint` reports whether it moved the endpoint; its doc comment says so
- `internal/component/ike/engine/inbound.go` - `handleOwnedInbound` calls `rebindChildSAs` when the adoption reports a move
- `internal/component/ike/engine/established.go` - the owner tick raises `startChildRekey` while `ps.rebindRekeyOwed && ps.pendingRekey == nil`
- `internal/component/ike/engine/reconcile.go` - `PeerSession` gains `rebindRekeyOwed`
- `internal/core/eap/eap_tls.go` - `newTLSMethod`: `GetConfigForClient` (TLS 1.2 ticket suppression) and the marker reference, each with the page and RFC 5216 Section 2.1.1 comment; the "the ticket above goes out either way" comment rewritten for TLS 1.2 (D2 design)
- `internal/le/register.go` - the le root handler `run` calls `stdpatch.Activate(root)` FIRST, before `leroot.RunBeforeDispatch` and `leroot.Dispatch`; a refusal is kept, never stops the dispatch (D2 design, "One producer")
- `internal/le/go/toolchain/gotoolchain.go` - `New` calls `stdpatch.Activate` (under le it answers the inherited tree, in a test binary it activates) and returns its refusal as New's error; `Overrides` stops setting `GOTOOLCHAIN=<pin>` and sets no `GOROOT`, and its contract names what it still sets; `Environment` inherits the three values with `os.Environ()`; `goProgram` stays `go`, and no `Toolchain.GOROOT` or `Toolchain.Go` field is added; the package doc's GOTOOLCHAIN paragraph restated (D2 design, "One producer")
- `internal/le/scratch/cacheclean.go` - `CleanCaches` leaves `cache/stdpatch/` and names each key directory with its size in its report (D2 design step 4b)
- `internal/le/build/gokrazy/gokrazy.go` - its doc's account of gok's environment says the patched `GOROOT`, `GOTOOLCHAIN=local` and PATH entry are inherited from le's process (D2 design "Reach")
- `internal/le/setup/install.go` - `applyVendorPatches` reads each patch's `Ze-Reviewed-Against` stamp through `vendorpatch.Stamp` and refuses, before `git apply`, a stamp whose module version differs from `vendor/modules.txt`, a missing stamp, or a stamp naming no vendored module, the stamp read placed before the `git apply --reverse --check` "already applied" `continue`, and the `os.IsNotExist` `continue` replaced by a refusal; each refusal ends with `./le setup install` (design round 4); the hand-written `vendorPatchPaths` array is deleted and replaced by `vendorpatch.Patches` (the directory read); the `GOFLAGS=-modcacherw` comment says it serves `go mod download`, which compiles nothing (D2 design, "The review stamp")
- `internal/le/vendorpatch/patches/netlink-xfrm-fixes.patch`, `internal/le/vendorpatch/patches/bubbles-textinput-cursor.patch` - each gains a stamp line in its leading text: `Ze-Reviewed-Against: github.com/vishvananda/netlink v1.3.1` and `Ze-Reviewed-Against: charm.land/bubbles/v2 v2.2.1`, after the contributor re-reads each patched function at that version
- `internal/le/verify/engine/stages.go` - `stage("go patch-stamps", "check")` in `fullStages`, kept in `changedStages` (D2 design, "The review stamp")
- `le` (launcher script, `build_le`) - runs `go run ./internal/le/go/stdpatch/goroot` under the pin, then builds with `GOROOT="$tree" GOTOOLCHAIN=local PATH="$tree/bin:$PATH" "$tree/bin/go"` for that command only; a refusal stops the bootstrap
- `internal/le/rfc/discriminate_observe.go`, `discriminate_guest.go` - no code change: the overlay stays the broken repository file alone and `GOROOT` arrives by inheritance from le's process environment; their comments on the overlay route are re-read, and any that would say the overlay carries the standard library is corrected
- `internal/appliance/cmd_build.go` - `SetGokGoEnv` imports no `internal/le` package and calls no `Activate`: FIRST, before it points GOMODCACHE at the checked-in modcache and sets `GOPROXY=off`, it refuses unless `GOROOT` is set, `GOTOOLCHAIN` is `local`, `exec.LookPath("go")` is `$GOROOT/bin/go` and the marker file exists under `$GOROOT` (a path constant whose comment names the patch), with the what/why/command-line message of D2 design "Reach"; its `os.Setenv("GOTOOLCHAIN", "local")` is deleted; its doc paragraph on the host go, and `CheckGoToolchain` in `gokpreflight.go`, are re-read and restated (both routes now compile with the pinned toolchain through the tree) (design round 4)
- `internal/appliance/cmd_build_test.go` - `TestSetGokGoEnvRefusesWithoutThePatchedTree`, `TestApplianceMarkerPathMatchesThePatch` (AC-28)
- `internal/le/build/gokrazy/gotoolchain_test.go`, `internal/appliance/gok_toolchain_test.go`, `internal/appliance/gok_stderr_test.go` - `TestRunRefusesAnOldGoBeforeTheKernel`, `TestGokBuildsWithTheHostToolchain`, `TestGokExitReachesStderr` rewritten as AC-28 states (design round 5, ISSUE 1), never deleted
- `cmd/ze/ze_le_personality_test.go` - `TestNormalZeLinksNoInternalLe` runs one subtest per shipped flavour, the set derived from the `cmd/ze/build_tag_*_test.go` constraints, `ze_setup` included, instead of `normalZeTags` alone (AC-29)
- `docs/guide/appliance.md` (every bare `ze appliance build <name>` command line: "Build an image", "Reserving hugepages for VPP", the disk image and ISO example, "Quick start", "ISO installer media", and the "Updating" step that says to run it again), `docs/guide/ze-install.md` (the `image.arch` build example and "Appliance ISO Install") - each command line gains the tree environment line of AC-28 or is shown as `./le build gokrazy`, with one sentence naming why and linking `docs/contributing/patching-the-standard-library.md` (design round 5, ISSUE 2; re-grep `ze appliance build` over `docs/guide/` at implementation); the `docs/guide/vpp.md` "System prerequisites" mention was read: prose saying what the build bakes, not a command line, so no change
- `docs/architecture/core-design.md` - "The rule is directional": `TestNormalZeLinksNoInternalLe` measures every shipped flavour, `ze_setup` included (AC-29); its current "over every build flavor" is true only after this spec
- `internal/le/test/qemu/run.go`, `internal/le/test/qemu/run_exec.go` - each guest profile keeps `GOROOT=/usr/local/go` as the real root, then runs `tree=$(go run ./internal/le/go/stdpatch/goroot)` and exports `GOROOT="$tree"`, `GOTOOLCHAIN=local` and `PATH="$tree/bin:$PATH"` (D2 design, "Reach")
- `internal/le/repo/bootstraps/tracked.go`, `internal/le/repo/compiles/repositorytrackedbuild.go`, `internal/le/repo/changed/selector.go`, `internal/appliance/gokpreflight.go`, `internal/le/go/module/move.go`, `internal/le/setup/proto_generate.go` - no code change: each appends to `os.Environ()` and scrubs no `GOROOT`, `GOTOOLCHAIN` or PATH, so each inherits the tree (D2 design, "Reach", read 2026-10-14); the implementer re-reads each at implementation and moves any that has changed into this list
- `docker/Dockerfile`, `docker/Dockerfile.lab` - `apk add --no-cache git` and one `RUN` that resolves the tree and builds under it (D2 design step 4a, "Reach")
- `.github/workflows/codeql.yml`, `.github/workflows/qemu-nightly.yml` - a step after each compiling job's `setup-go` that writes `GOROOT` and `GOTOOLCHAIN=local` to `$GITHUB_ENV` and `<tree>/bin` to `$GITHUB_PATH`; `.github/workflows/govulncheck.yml` unchanged (reached through `./le verify deps vulnerability`)
- `docs/contributing/patching-the-standard-library.md` - after implementation, restate the mechanism as built and drop its `doc-links: ignore` markers (AC-24)
- `docs/contributing/running-commands.md` - `./le go stdpatch goroot`, `./le go patch-stamps check`, and the `GOROOT`, `GOTOOLCHAIN=local` and PATH entry le's process environment now carries
- tagged and untagged tests listed in the TDD plan
- `internal/le/interoplab/ipsec/nat.go`, `internal/le/interoplab/ipsec/checkers.go` - port column, the rebind step (port SNAT swap, conntrack delete, the scenario's fixed row log line match), four checkers
- `test/interop-ipsec/Dockerfile.nat` - add `conntrack-tools` to the `apk add` line, and a header comment line saying the rebind step flushes UDP conntrack entries with it (A-8)
- `internal/component/ike/engine/fsm.go` - `resolvePendingAfterOwnerLoop` clears `ps.rebindRekeyOwed` when it promotes a pending child (Rebind design, "Owed flag and promotion")
- `rfc/short/rfc3948.md`, `rfc/short/rfc5216.md` (RFC5216-2.1.1-4: the standard-library patch and the TLS 1.2 ticket suppression, page named), `rfc/short/rfc7296.md` (RFC7296-2.23-9 row note: the ban binds Ze's own port, a NAT-mapped peer 500 is followed, Q-2 = A′ with both RFC 7296 Section 2.23 sentences quoted), `rfc/discrimination/rfc3948.json`, `rfc7296.json`, `rfc5216.json`, `rfc/audit/*` via `./le rfc audit-stamp`
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
- `internal/le/go/stdpatch/gen/stdpatch.go` (plus `stamp.go`, `tree.go`; moved from `internal/le/go/stdpatch`, see "Deviations from the plan") - the patched-GOROOT generator (D2 design, "How the tree is produced", steps 1 to 5, with the step 1a stamp refusal), `Activate` (D2 design, "One producer") and `Stamp`; standard-library imports only
- `internal/le/go/stdpatch/patches/crypto-tls-server-session-id.patch` - the patch, with the hunk of "Two changes" row 1 and the marker file of row 2; its leading text carries `Ze-Reviewed-Against: go1.27.2`, the page path and the RFC 5216 Section 2.1.1 quote
- `internal/le/go/stdpatch/goroot/main.go` - the bootstrap entry the `le` script, the QEMU guests, the Dockerfiles and CI run with `go run`; prints the tree path
- `internal/le/go/stdpatch/register.go`, `internal/le/go/stdpatch/actions.go` - `./le go stdpatch goroot`
- `internal/le/go/patchstamps/patchstamps.go` plus its `register.go` (see "Deviations from the plan") - `./le go patch-stamps check`: every patch in both `patches/` directories, each standard-library stamp against go.mod's pin (`goversionpin.DeclaredRelease`) and the activated `GOVERSION`, each vendor stamp against `vendor/modules.txt` (D2 design, "The review stamp")
- `internal/le/vendorpatch/stamp.go` - `Stamp` (parses a vendor patch's `Ze-Reviewed-Against` line) and `Patches` (the patch set as a read of `internal/le/vendorpatch/patches/`), used by `applyVendorPatches` and the check
- `internal/le/go/stdpatch/gen/stdpatch_test.go`, `internal/le/go/stdpatch/gen/crypto_tls_session_id_test.go`, `internal/le/go/patchstamps/patchstamps_test.go`
- `internal/le/setup/install_test.go` - the `applyVendorPatches` stamp refusals of AC-27 over a fixture tree
- `docs/contributing/patching-the-standard-library.md` - written in the design phase (2026-10-14), rewritten for the patched GOROOT in D2 design round 2 and for one producer and the review stamps in round 3; states the mechanism is not built yet

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no config change |
| YANG validation constraints | N-A | no leaf |
| YANG custom validators | N-A | no leaf |
| CLI commands/flags | Yes (le only) | `./le go stdpatch goroot` and `./le go patch-stamps check` (`ai/rules/cli.md`); `show ipsec` output keys unchanged |
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
| 3 | CLI command added/changed? | Yes (le) | `docs/contributing/running-commands.md`: `./le go stdpatch goroot` and `./le go patch-stamps check` |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/guide/ipsec.md`; D2 (design round 5, ISSUE 2): `docs/guide/appliance.md` and `docs/guide/ze-install.md`, whose bare `ze appliance build <name>` command lines gain the AC-28 environment line or the `./le build gokrazy` route |
| 7 | Wire format changed? | No | the UDP header layout is unchanged; only the port values |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc3948.md`, `rfc/short/rfc5216.md`; `docs/features/rfc-status.md` regenerated if a Support row changes |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` (nat.conf port column); `docs/contributing/patching-the-standard-library.md` (new, design phase: the technique, stated as not built; restated as built at implementation, AC-24); `docs/contributing/README.md` and `ai/INDEX.md` rows for it (design phase) |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md`, `ipsec-8-ikev2-child-xfrm.md`, `ipsec-14-responder.md`; D2: `docs/architecture/core-design.md` (le's root handler sets the patched `GOROOT`, `GOTOOLCHAIN=local` and the PATH entry in its own process environment, and every Go command inherits them), `docs/architecture/testing/tracked-build-gate.md` (the tree arrives through the inherited environment; `goEnv` scrubs no `GOROOT`, and its refusal of ambient GOFLAGS is unchanged), `docs/architecture/appliance/builder.md` (`SetGokGoEnv` refuses an environment without the tree and sets no `GOTOOLCHAIN` itself, so gok's go subprocesses inherit `GOROOT`, `GOTOOLCHAIN=local` and PATH from the caller), `docs/architecture/testing/qemu-integration.md` (each guest profile runs the generator and exports the three values), `docs/architecture/testing/verify-freshness-scope.md` (the `go patch-stamps` stage in both modes), `docs/guide/appliance.md` (declared by `internal/le/build/gokrazy/gokrazy.go`; its `./le build gokrazy` paragraph lists what the wrapper sets up, and gains the patched standard library), `docs/architecture/ike/ipsec-11-interop-eap.md` and `docs/guide/ipsec.md` (TLS 1.2: server-chosen session id, no TLS 1.2 ticket) |
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
6. **Phase: D2** (Q-1 answered (b), Q-5 (A)) -- (i) `stdpatch` with its tests (AC-21, AC-25) and the patch file, `TestPatchedCryptoTLSServerSessionID` red under a marker-only tree, green with the patch; (ii) `stdpatch.Activate` called first in the le root handler `run`, `gotoolchain.New` calling it, `Overrides` without `GOTOOLCHAIN` or `GOROOT`, and the discrimination environment inheriting, with `TestActivateSetsTheProcessEnvironment`, `TestLeChildInheritsThePatchedGOROOT`, `TestToolchainOverridesLeaveTheToolchainToTheProcess` and `TestDiscriminateEnvironmentCarriesThePatchedGOROOT` red first; (ii-a) the review stamps: `Ze-Reviewed-Against` on the standard-library patch and the two vendor patches (each re-read at its stamped version), `stdpatch.Stamp` and the step 1a refusal, `vendorpatch.Stamp`, `vendorpatch.Patches` replacing `vendorPatchPaths` and the `applyVendorPatches` refusal, `./le go patch-stamps check` and its verify stage, with the AC-26 and AC-27 tests red first; (iii) the marker reference and `GetConfigForClient` in `newTLSMethod`, `TestRFC5216TLS12FullHandshakeServerHelloSessionID` red at HEAD then green; (iv) every site of D2 design "Reach" with its own route (launcher, QEMU guest profiles, Dockerfiles, CI) wired, the `SetGokGoEnv` refusal with `TestSetGokGoEnvRefusesWithoutThePatchedTree` and `TestApplianceMarkerPathMatchesThePatch` red first, `TestNormalZeLinksNoInternalLe` extended to every shipped flavour (AC-28, AC-29), and every inheriting site (the inheritance row, the tracked build, discrimination, `./le build gokrazy`) re-read and left without code change; each built once under the tree, and golangci-lint, staticcheck and govulncheck each run once on `internal/core/eap` under it (A-10, A-13); (v) `responder-eap-tls12`; (vi) the page restated as built
   - **D2 status (2026-10-14):** (i) to (iv) and (vi) are committed, through bc44c99119, 816a0c4a83, 92dc823143, 26dd24bda5, 77b56f0a33, d040e19521 and dead39fa03. The last of those carries the core-design, tracked-build-gate, verify-freshness-scope and ze-install pages. (v) is written and NOT RUN (R-11, see "Interop Tests"). Still owed: the `docker/Dockerfile` proof build (label `d2docker`, image `ze-d2-stdpatch-probe:36ef9191`, to be removed by name afterwards), which was still at `go mod download` at 17:46; a QEMU guest run under the patched tree; a recorded CI run of the codeql and qemu-nightly steps; the rfc5216 audit re-judge by an independent agent (step 7); and the audit reseal, blocked while `rfc/audit/rfc4456.json` and `rfc6793.json` carry foreign dirty hunks
   - **Status 2026-10-21:** the rfc5216 re-judge is done (RFC5216-2.1.1-4 and RFC3948-2.1-2 `enforced` in `rfc/audit/`); the CI steps ran green (qemu-nightly 39402173269, CodeQL Advanced 39576850399, govulncheck 39407444997); a QEMU guest run under the tree passed (`f0493deda2`). Closure review (four independent rounds) found and fixed a BLOCKER, the rebind rekey keeping the retired pair's peer address (`5754eac4f4`, `25938b3000`). Owed: the interop runs and walks, the Linux integration run, the Docker proof build and the full verify, all waiting for the shared heavy slot; listed in `plan/handover/14-ike-eap-rfc-defects-closure.md`
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
| D2 reach | every row of D2 design "Reach" built once under the tree, with `GOROOT=<tree>`, `GOTOOLCHAIN=local` and `<tree>/bin` first on PATH; the three values set in one place for le (`stdpatch.Activate` at `run`) and no second declaration in `Overrides` or per argv; no site carries a standard-library `-overlay`; no site leaves a toolchain switch able to replace `GOROOT` (A-12) |
| D2 review stamps | every patch in both `patches/` directories carries a `Ze-Reviewed-Against` stamp; a stale or missing stamp refuses at step 1a, in `applyVendorPatches` and in `./le go patch-stamps check` even when the diff applies; the vendor patch set is a directory read, never a hand-written list |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Child SA encap ports from the IKE SA (D1, D1c, D1d), refusal without an endpoint (Q-2 A′) | `ls internal/component/ike/engine/rfc3948_encap_port_test.go`; `go test ./internal/component/ike/...` green under the patched tree (the full race run is `./le verify worktree`'s); `grep -n childEncapPorts internal/component/ike/engine/create_child.go` |
| EAP responder adopts the endpoint before building the Child SA (D1b) | `TestRFC3948EAPResponderChildSAEncapsulatesToTheIKEPeerPort` green |
| Rebind: migrate or owed rekey | `ls internal/component/ike/engine/rebind.go internal/component/ike/engine/rfc3948_rebind_test.go`; the rebind tests green |
| Linux XFRM read-back test | `ls internal/component/ike/engine/rfc3948_encap_port_integration_linux_test.go`; `TestRFC3948InstalledStateCarriesTheIKEPeerPort` run in the Ze-kernel QEMU guest |
| Four new interop scenarios and the nat.conf port column | `ls test/interop-ipsec/scenarios/{real-nat-port-translated-ze-responder,real-nat-port-translated-transport-ze-responder,real-nat-port-translated-eap-ze-responder,responder-eap-tls12}`; each green in `./le test qemu docker-lab lab "test integration interop-ipsec" env "IPSEC_INTEROP_SCENARIO=<name>"` |
| D2 patched GOROOT generator, patch, bootstrap, le commands | `ls internal/le/go/stdpatch/gen/stdpatch.go internal/le/go/stdpatch/patches/crypto-tls-server-session-id.patch internal/le/go/stdpatch/goroot/main.go internal/le/go/patchstamps/patchstamps.go`; `./le go stdpatch goroot`; `./le go patch-stamps check` |
| D2 wire proof | `TestRFC5216TLS12FullHandshakeServerHelloSessionID` (`internal/core/eap/rfc5216_session_id_test.go`) green |
| D2 reaches the Docker build | `docker build -f docker/Dockerfile` under the patched tree succeeds |
| RFC records | `rfc/audit/rfc3948.json` RFC3948-2.1-2 and `rfc/audit/rfc5216.json` RFC5216-2.1.1-4 `enforced`; `./le rfc check` |
| Pages | `docs/contributing/patching-the-standard-library.md` states the mechanism as built; `./le doc check verify` |

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
| D2: patch `crypto/tls` at build time, one hunk in `doFullHandshake` that chooses a 32-octet id when no ticket is issued, plus TLS 1.2 ticket suppression in the authenticator; delivered as a patched GOROOT link tree (real `src` directories, symlinked files, copied embed files) selected with `GOROOT=`, `GOTOOLCHAIN=local` and `<tree>/bin` first on PATH, set once in le's own process environment by `stdpatch.Activate` at the root handler `run`; every patch carries a `Ze-Reviewed-Against` stamp refused on any version change | per-argv selection in `gotoolchain` (`Toolchain.Go`, round 2); a `leroot.BeforeDispatch` hook; per-site environment entries; (a) TLS 1.3 only; (c) recorded deviation; fork the toolchain; a copy of `crypto/tls` in the tree; mirror the unexported handshake state with `unsafe`; a per-`Config` opt-in field; delivery by `go build -overlay` (the article's); a tree of symlinked directories (round 1); a full copy of GOROOT; hard links | owner answer Q-1 (b), 2026-10-14, with the article he named as the reference; Q-5 (A), main thread under the owner's direction, for the delivery: golangci-lint, staticcheck and govulncheck cannot open a file an overlay adds (A-10), a symlinked directory hides its packages from `go list std`, a copy costs the whole GOROOT per key, and a hard link to a root-owned file is refused. A fork or a copy must be rebased by hand at every bump and drifts silently; `unsafe` mirroring cannot reach a local of `doFullHandshake` at all; the opt-in doubles the patch for no consumer (D2 design). The patch is stored as a diff applied to the current GOROOT, so a bump refuses loudly rather than reverting it. One producer (round 3): many le sites run a literal `go` from `os.Environ()` and never reach `gotoolchain`, so a per-argv rewrite misses them and is a second route to the same choice (`ai/rules/no-layering.md`); a before-dispatch hook may not do slow work or fail. The stamp (owner, 2026-10-14): a clean apply proves only that the context lines did not move, so any version change forces a re-read of upstream |
| D2 guard: compile-time marker, plus wire tests, plus refusal on bump | test-only; runtime self-probe at `newTLSMethod` | a test proves its own build only; a self-probe ships a broken binary to the operator. The marker costs plain `go` and gopls the tree's environment (three exports), documented on the page |
| Rebind: migrate where a `TunnelMigrator` can move the pair, else rekey that Child SA onto the new ports (Rebind design) | always rekey; always migrate; `XFRM_MSG_MAPPING` events | the owner's answer, "migrate it (strongSwan)", and charon's fallback (A-7); always-rekey discards a live replay window the kernel could keep; always-migrate strands transport mode, VPP and older kernels; mapping events see ESP only, need a netlink listener and have no VPP equivalent |

### Deviations from the plan

| Planned | Built | Why |
|---------|-------|-----|
| The generator in `internal/le/go/stdpatch` (`stdpatch.go`, `stamp.go`, `tree.go`, their tests), its le command in a `register.go` beside it | The generator in `internal/le/go/stdpatch/gen` (package `gen`, standard-library imports only: `gen.Activate`, `gen.Tree`, `gen.Pin`); the le command `go stdpatch` (`actions.go`, `register.go`) at `internal/le/go/stdpatch`; the bootstrap entry stays `internal/le/go/stdpatch/goroot`, the patch stays in `internal/le/go/stdpatch/patches` | le requires a command to register from the path its name predicts (`TestEveryCommandIsFoundAtThePathItsNamePredicts`), and its registering-package scan reads two levels under `internal/le` (`TestCompositionEqualsLiveRegisteringPackagePopulation`), so the surface must be `internal/le/go/stdpatch`. That package imports le, so the std-only generator moved out of it. The interim `stdpatchcmd` package (e4d1d1229c) is deleted, not kept beside it (main-thread decision, 2026-10-14) |
| `./le go patch-stamps check` in `internal/le/go/stdpatch/patchstamps.go` | `internal/le/go/patchstamps` (`patchstamps.go`, `register.go`, `patchstamps_test.go`) | The same path rule: `go patch-stamps` is found at `internal/le/go/patchstamps` |

## Design Insights

| Insight | Where it bites |
|---------|----------------|
| A Delete names the deleting node's OWN inbound SPI, never the peer's. RFC 7296 Section 1.4.1: "To delete an SA, an INFORMATIONAL exchange with one or more Delete payloads is sent listing the SPIs (as they would be expected in the headers of inbound packets) of the SAs to be deleted." The earlier "Delete for the peer's SPI" wording described the same pair from the other side; corrected 2026-10-14 | AC-10 initiated paths: `sendDeleteESP(sa.ChildInboundSPI)` (`established.go`), `sendDeleteESP(p.newInboundSPI)` (`create_child.go` `finishChildCreate`, `inbound.go` rekey response) |
| The rebind owes its rekey through a level trigger, not a call: `serviceChildRekeyTrigger` (`established.go`, extracted from the owner tick) raises `startChildRekey` while `rebindRekeyOwed` or the soft lifetime stands and no rekey is pending, so every hold in `startChildRekey` defers it. The tests drive that same function | `rebind.go`, `established.go`, `rfc3948_rebind_test.go` |
| `migrateMobikeChild` gained the local endpoint and reports the pairs it moved; MOBIKE passes `sa.mobike.local`, the rebind passes the child's unchanged local address on the NAT-T socket's port (`childEncapPorts`). Three untagged tests' call sites changed with it | `mobike.go`, `rebind.go` |
| A rebind fixture must send from an address the host owns: the PSK fixture's configured 10.0.0.1 makes every `SendFrom` fail at Debug, which reads as "the datagram did not arrive" | `rbOwner` (`rfc3948_rebind_test.go`) |
| Never guess a port: a missing authenticated peer endpoint AND a missing NAT-T socket both refuse the Child SA, with no 4500 fallback on either side (Q-2 A′, point 4; main-thread decision 2026-10-14) | `childEncapPorts`, `localEncapPort` (`create_child.go`) |
| AC-14 TS_UNACCEPTABLE (2026-10-21): the cause was the missing USE_TRANSPORT_MODE in ze's Child SA rekey request, not the substituted TSr address. RFC 7296 Section 1.3.3: "Usually, these will be the same notifications that were used in the original exchange; for example, when rekeying a transport mode SA, the USE_TRANSPORT_MODE notification will be used." Section 1.3.1: "Except when using this option to negotiate transport mode, all Child SAs will use tunnel mode." strongSwan 5.9.14 (`src/libcharon/sa/ikev2/tasks/child_create.c`, fetched from the tag): `process_payloads` starts "/* defaults to TUNNEL mode */ this->mode = MODE_TUNNEL;" and only `handle_notify`'s `case USE_TRANSPORT_MODE: this->mode = MODE_TRANSPORT;` changes it; `narrow_ts` substitutes the NATed side only `if (this->mode == MODE_TRANSPORT && this->ike_sa->has_condition(this->ike_sa, cond))` through `get_transport_nat_ts`, which does `ts->set_address(ts, ike)` with charon's own IKE address; otherwise `select_and_install` finds no match and logs "no acceptable traffic selectors found" (the line the lab showed), and the rekey responder takes its config from the old Child SA (`child_rekey.c`, `set_config`), so `select_child_cfg` never runs. With the notify, charon substitutes 172.28.0.7 to its own 172.28.0.3 itself. The brief's candidate (send the peer's stored original address 172.28.0.3) is rejected: Section 2.23.1, "For the client proposing transport mode", says "The TSr entries MUST have exactly one IP address, and that MUST match the destination address of the IKE SA", which is 172.28.0.7 as ze sees it, and without the notify charon would have installed a tunnel-mode SA opposite ze's transport one. Fix: `initiateChildRekey` sends the notify when the retired SA runs transport, `respondChildRekey` echoes it ("If the request is accepted, the response MUST also include a notification of type USE_TRANSPORT_MODE", Section 1.3.1; charon's `build_i` always sends it for a transport rekey), and `applyChildRekeyResponse` refuses an answer without it (`errChildRekeyTransportDeclined`; "If this is unacceptable to the initiator, the initiator MUST delete the SA"), the handler deleting the new pair. Tests: `rfc7296_child_rekey_transport_test.go`, red before the fix (three failures on the missing notify and the accepted tunnel answer). A request without the notify answered by ze is journaled (`plan/journal/silent-fall-through.md`) | `rekey.go`, `inbound.go`, `create_child.go` (`childRekeyFixTransport`) |

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
| D2 design review, round 1 (after Q-1 (b) and Q-2 A′) | 2026-10-14 | 1 BLOCKER, 5 ISSUE, 5 NIT, each read at the producer before it was applied, none refuted. B-1: a second `-overlay` in GOFLAGS breaks discrimination (`setGoFlagsOverlay` then `testfunctional.Prepare`; `environment`) and `Overrides`'s "variables that this toolchain SETS" contract; the overlay now lives in `Environment` with a one-overlay rule, `writeOverlay` merges the stdlib entries, AC-20 and its tests rewritten. I-1: a legacy-0x0304 ClientHello without supported_versions shows 1.3 in `SupportedVersions` but negotiates 1.2 (`clientHelloInfo`, `readClientHello`); step 3 keys on extension 43 listing 1.3, the negotiated-version alternative argued unreachable through the public API, AC-2 gains a third subtest, the "not reached on TLS 1.2" sentence corrected. I-2: A-10 PROBED and BROKEN (golangci-lint, staticcheck, govulncheck fail on the added GOROOT file; gopls reports the marker); a patched GOROOT link tree passed every tool but gopls; owner question Q-5 raised, recommendation (A); the page's gopls `build.buildFlags` remedy withdrawn. I-3: AC-10 and its test cover the initiator IKE_AUTH path; initiated paths send a Delete for the peer's SPI and keep the IKE SA (RFC 7296 Section 2.21.2); MOBIKE refuses too (remote from `peerEndpoint`, not `remoteUDPAddr`); the "only writer of `peerEndpoint`" constraint corrected to four writers. I-4: cache key carries the GOROOT path, the rename loser answers the winner's, `CleanCaches` keeps `cache/stdpatch/`, the generator ignores a stdpatch selection in its own environment. I-5: Reach rows corrected (QEMU sites, `govulncheck.yml` through `./le verify deps vulnerability`, `move.go` and `proto_generate.go` set no GOFLAGS, `setup/install.go` and `internal/le/build/gokrazy/gokrazy.go` added). N-1 TLS servers named; N-2 answer-(A) leftovers fixed; N-4 the marker guard scoped to binaries linking `internal/core/eap` (spec R-12 and page); N-6 git is a generator dependency, the QEMU guest has it, the Docker builder stages gain it | applied; Q-5 open |
| D2 design round 2 (Q-5 (A), decided by the main thread under the owner's direction) | 2026-10-14 | Every row of "Q-5 (A): what changes" rewritten in place and the table removed: the generator builds `cache/stdpatch/<key>/goroot` (step 4: top-level entries symlinked; every `src` directory REAL and every file a symlink; the embedded files of `std` and `cmd`, the patched file and the marker real; a `go list -e std cmd` self-check; atomic rename, the loser answers the winner); the key gains a layout version constant; "Selection" sets `GOROOT`, `GOTOOLCHAIN=local` (a toolchain switch overwrites `GOROOT`, `execGoToolchain`) and `<tree>/bin` first on PATH, and le's argv starts with `Toolchain.Go` (`exec.Command` resolves `argv[0]` on le's PATH); "Reach" rewritten for each site (launcher, le actions, discrimination unchanged and B-1 gone, tracked build, gokrazy, QEMU guests, Dockerfiles in one `RUN`, CI through `$GITHUB_ENV`/`$GITHUB_PATH`, developer); `CleanCaches` keeps the trees (in-flight builds use them) and reports their size; AC-20, AC-21 rewritten, AC-25 added, TDD rows renamed (`TestToolchainOverridesSelectThePatchedGOROOT`, `TestToolchainEnvironmentPutsTheTreeFirstOnPath`, `TestDiscriminateEnvironmentCarriesThePatchedGOROOT`, `TestStdpatchTreeIsContentAddressed`) and added (`TestStdpatchConcurrentBuildsAnswerOneTree`, `TestStdpatchTreeServesTheWholeStdlib`); A-12, A-13, R-14 to R-17 added; the page rewritten for the tree and its broken `internal/le/go/stdpatch` reference marked not built. Probes (round 2, same scratch directory): the round-1 tree (symlinked directories) answers `go list std` with 2 packages of 383; an all-symlink `src` fails "cannot embed irregular file"; the step 4 tree answers 383/359, passes vet, run, `-race`, cgo, `go tool covdata`/`test2json`, golangci-lint, staticcheck, govulncheck, and gopls when run directly; round 1's gopls result came from this host's shared `-remote=auto` daemon (trace: `GOROOT:/usr/local/go`) | applied; independent review of this round owed |
| D2 design round 3 (review round 2's ISSUE-1, ISSUE-2, NOTE-2, NOTE-3; the owner's review-stamp requirement of 2026-10-14) | 2026-10-14 | ISSUE-1+2: one producer, `stdpatch.Activate`, called first in the le root handler `run`, sets `GOROOT`, `GOTOOLCHAIN=local` and PATH in le's own process environment, so every `os.Environ()` child and every `LookPath("go")` inherits the tree; the per-argv `Toolchain.Go` rewrite deleted (no-layering); `Overrides` drops `GOTOOLCHAIN=<pin>`; a refusal leaves dispatch running and is `gotoolchain.New`'s error; "Reach" rewritten as the launcher, an inheritance row listing the round-2 sites, the tracked build (read: `goEnv` scrubs no `GOROOT`, `GOENV=off` touches only the file), discrimination, gokrazy (ze-setup's `ze appliance build` is not under le, so `SetGokGoEnv` calls `Activate` first), QEMU guests, Docker, CI, developer; AC-20 and its tests rewritten (`TestActivateSetsTheProcessEnvironment`, `TestLeChildInheritsThePatchedGOROOT`, `TestToolchainOverridesLeaveTheToolchainToTheProcess`). NOTE-2: embed set is the union over `CGO_ENABLED` 0/1 and the five targets; EvalSymlinks comparison in step 1 and AC-25. NOTE-3: `GetConfigForClient` comment cites RFC 9190 Section 2.1.2. Owner requirement: "The review stamp" (`Ze-Reviewed-Against`) for standard-library and vendor patches, step 1a refusal even when the diff applies, `applyVendorPatches` refusal, `./le go patch-stamps check` as a verify stage in both modes, AC-26, AC-27 and their tests; step 4a's Docker sentence corrected (step 1 builds from the pin, so a floating image never reaches 1a or 5) | applied. First half committed as "plan: IKE-EAP D2 round 3 (partial): one producer, patch stamps"; second half (2026-10-14) done: Required Reading constraints (gotoolchain, tracked build inherits, gok), qemu-integration.md and verify-freshness-scope.md named, Data Flow rows (one producer, review stamps), Wiring Test row, A-12 test names, Files to Modify and Create (register.go, install.go stamp refusal and `vendorpatch.Patches`, stages.go, the three stamped patches, `patchstamps.go`, `vendorpatch/stamp.go`, `install_test.go`; the inheriting sites marked no code change), CLI and doc checklist rows, Implementation step 6, Critical Review, Key Design Decisions, and the page rewritten for one producer and the stamps |
| D2 design round 4 (review round 3: 1 BLOCKER, 2 ISSUE, 1 NOTE; decided by the main thread under the owner's 2026-10-14 delegation) | 2026-10-14 | BLOCKER: `SetGokGoEnv` calling `stdpatch.Activate` would link `internal/le` into the shipped ze-setup build, against `docs/architecture/core-design.md` ("The rule is directional"); round 3's claim that `internal/appliance` already imports `internal/le` was false (read: only `cmd_build_test.go`). Now `SetGokGoEnv` activates nothing and refuses unless `GOROOT`, `GOTOOLCHAIN=local`, `$GOROOT/bin/go` first on PATH and the marker file are present (file existence, no le import), with a what/why message ending in the command line and naming `./le build gokrazy`; its own `GOTOOLCHAIN` Setenv deleted; le routes pass by inheritance; AC-28 and two tests; AC-29 extends `TestNormalZeLinksNoInternalLe` (which measured only `ze_core`+`ze_distro`) to every shipped flavour, `ze_setup` included. ISSUE 1: a launcher that must build le cannot after a refusing bump, so the repair is defined as a plain edit needing no le (read the upstream function, regenerate with `git diff --no-index`, restamp, or delete the patch and marker reference); "le still starts" corrected. ISSUE 2: one wording everywhere: dispatch continues, actions compiling through `gotoolchain.New` refuse (named by class), anything else compiling `internal/core/eap` fails at the marker. NOTE: the vendor stamp check runs before the `--reverse --check` "already applied" continue, a missing patch file refuses, every refusal ends with its command line. A-10 corrected: gopls run directly passed, only the shared daemon failed. Page updated to match | applied |
| D2 design round 5 (review round 4: 3 ISSUE, 1 NOTE; decided by the main thread under the owner's 2026-10-14 delegation, each read at its producer) | 2026-10-14 | ISSUE 1: AC-28 turns three existing tests red, read: `TestRunRefusesAnOldGoBeforeTheKernel` clears `GOTOOLCHAIN` and puts its fake go outside any `GOROOT`, and `Run` calls `SetGokGoEnv` before `CheckGoToolchain`; `TestGokBuildsWithTheHostToolchain` asserts the `GOTOOLCHAIN=local` Setenv AC-28 deletes; `TestGokExitReachesStderr`'s child would exit 4 at the refusal. Each is REWRITTEN in AC-28 and the test table (fixture tree first; refusal naming the command line plus an accepting arm; child under the tree env), an expectation change following from the owner-approved Q-5 (A) design, none deleted. ISSUE 2: the bare `ze appliance build <name>` command lines of `docs/guide/appliance.md` (two more than the review named: "Quick start" and "ISO installer media") and `docs/guide/ze-install.md` added to Files to Modify and doc checklist row 6; the `docs/guide/vpp.md` mention read and left, as prose and not a command line. ISSUE 3: the no-le repair reads upstream with `env -u GOROOT -u GOTOOLCHAIN go env GOROOT` (a path under `cache/stdpatch/` means `<tree>/bin` leads PATH); the delete branch also deletes the AC-28 marker constant, its `SetGokGoEnv` check and `TestApplianceMarkerPathMatchesThePatch`, and with no patch left the rest of the AC-28 check, `SetGokGoEnv` setting `GOTOOLCHAIN=local` again. NOTE 1: AC-29 is green at HEAD and owns no red, its scratch-import discrimination kept; its flavour set derived from the `cmd/ze/build_tag_*_test.go` constraints through `go/build/constraint`; the AC-28 LookPath comparison runs `filepath.Clean` on both sides. Page updated (repair steps 2 and 3). The D2 design is treated as CONVERGED after this round: the next review reopens it only for a product BLOCKER | applied |

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (before the fix)
- [ ] Tests PASS (after the fix)

### Verification
- [ ] `./le verify worktree`

## Implementation Summary

### What Was Implemented
- D1, D1c, D1d: every Child SA takes its UDP encapsulation ports from the IKE SA, the authenticated peer endpoint for the remote port and the bound NAT-T socket for the local one (`childEncapPorts`, `localEncapPort`, `create_child.go`); a rekeyed Child SA takes them from the IKE SA, never from the retired child (`rekey.go`); without an endpoint or a socket the Child SA is refused (Q-2 A′), answered paths before sending, initiated paths with a Delete of ze's own inbound SPI
- D1b: `handleResponderEAP` adopts the authenticated endpoint before `buildAuthResponse` (`responder_eap.go`)
- Rebind (Q-3 (i)): an authenticated move of a non-MOBIKE peer after establishment migrates tunnel Child SAs through `TunnelMigrator`, else owes a rekey onto the new ports (`rebind.go`, `serviceChildRekeyTrigger` in `established.go`); closure review added the address-move case (`5754eac4f4`, `25938b3000`)
- D2 (Q-1 (b), Q-5 (A)): a patched GOROOT link tree (`internal/le/go/stdpatch/gen`), one `crypto/tls` hunk choosing a 32-octet session_id on a TLS 1.2 full handshake with no ticket, activated once in le's root handler; TLS 1.2 tickets suppressed in the authenticator (`newTLSMethod`, `GetConfigForClient`); review stamps on every standard-library and vendor patch (`./le go patch-stamps check`); `SetGokGoEnv` refuses an environment without the tree
- Interop: nat.conf port column, the rebind step with conntrack flush, four scenarios (`real-nat-port-translated-ze-responder`, `real-nat-port-translated-transport-ze-responder`, `real-nat-port-translated-eap-ze-responder`, `responder-eap-tls12`)

### Bugs Found/Fixed
- Closure review round 1 BLOCKER: a rekey after a NAT rebind kept the retired pair's peer ADDRESS; fixed in `5754eac4f4`, covered by `rebind_address_test.go`
- RFC5216-2.1.1-4 config-level tags were wrong at HEAD (ticket keys are not a sessionId): withdrawn, the proof moved to `TestRFC5216TLS12FullHandshakeServerHelloSessionID`

### Documentation Updates
- `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`, `ipsec-9-ikev2-eap-nat.md`, `ipsec-13-rekey-wire.md`, `docs/guide/ipsec.md`, `docs/architecture/testing/interop.md`, `docs/contributing/patching-the-standard-library.md` and the D2 pages of the Documentation Update Checklist, each edited in the commit that changed the behaviour (commits listed under Implementation Steps, D2 status)
- `./le doc check verify`: run by the full verify below

### Deviations from Plan
- See "Deviations from the plan" above (stdpatch generator package path, patchstamps path)

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-10: every type-checking tool honours a GOFLAGS `-overlay` | golangci-lint, staticcheck and govulncheck fail opening a file an overlay adds under GOROOT | probe 2026-10-14 | Q-5 raised and decided (A): patched GOROOT link tree instead of an overlay |
| approach | the rebind rekey copied the retired pair's peer address | a port-only rebind hid it; an address move sent ESP to the old address | closure review round 1 | fixed in `5754eac4f4` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| D1: ESP to the peer's NAT-translated IKE port | Done | `internal/component/ike/engine/create_child.go` `childEncapPorts` | with D1b, D1c, D1d |
| D2: server-chosen sessionId on a TLS 1.2 full handshake | Done | `internal/le/go/stdpatch/patches/crypto-tls-server-session-id.patch`, `internal/core/eap/eap_tls.go` `newTLSMethod` | Q-1 (b) |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestRFC3948ResponderChildSAEncapsulatesToTheIKEPeerPort`, `TestRFC3948InstalledStateCarriesTheIKEPeerPort`, interop `real-nat-port-translated-ze-responder` | |
| AC-2, AC-19, AC-22 | Done | `TestRFC5216TLS12FullHandshakeServerHelloSessionID`, `TestPatchedCryptoTLSServerSessionID`, interop `responder-eap-tls12` (TLS 1.2 handshake through Finished, then Ze's RFC 7627 refusal; re-scoped 2026-10-21) | |
| AC-3 | Done | `rfc/audit/rfc3948.json` RFC3948-2.1-2 and `rfc/audit/rfc5216.json` RFC5216-2.1.1-4 `enforced`, independent re-judge | |
| AC-4 | Done | closed before this spec (`37dbe54095`) | |
| AC-5 | Done | `TestRFC3948PeerOn4500KeepsPort4500`, interop `real-nat-tunnel-control`, `real-nat-transport-ze-responder`, `real-nat-transport-ze-initiator` | |
| AC-6 | Done | `TestRFC3948EAPResponderChildSAEncapsulatesToTheIKEPeerPort`, interop `real-nat-port-translated-eap-ze-responder` | |
| AC-7, AC-8, AC-9, AC-10, AC-10b, AC-10c | Done | the unit tests the AC rows name, in `rfc3948_encap_port_test.go` and `rfc7296_natt_test.go` | |
| AC-11, AC-13 to AC-18 | Done | `rfc3948_rebind_test.go`, `rebind_address_test.go`; interop rebind steps for AC-11 and AC-14 | rows 3, 4 and AC-13 unit only (Known Limitations) |
| AC-12, AC-24 | Done | pages above; `./le doc check verify` | |
| AC-20, AC-21, AC-23, AC-25 to AC-29 | Done | the tests the AC rows name under `internal/le/go/stdpatch`, `internal/le/go/patchstamps`, `internal/le/setup`, `internal/appliance`; CI runs 39402173269, 39576850399, 39407444997; QEMU guest `f0493deda2`; Docker proof build (Pre-Commit Verification) | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Unit tests of the TDD plan | Done | `internal/component/ike/engine/`, `internal/core/eap/`, `internal/le/go/...`, `internal/appliance` | red then green recorded per unit in the implementation commits |
| `TestRFC3948InstalledStateCarriesTheIKEPeerPort` | Done | `internal/component/ike/engine/rfc3948_encap_port_integration_linux_test.go` | Ze-kernel QEMU guest run, Pre-Commit Verification |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Files to Create | Done | listed under Files Exist below |
| Files to Modify | Done | in the implementation commits |

### Audit Summary
- **Total items:** 2 requirements, 29 AC rows
- **Done:** all
- **Partial:** none
- **Skipped:** none
- **Changed:** the two package paths under "Deviations from the plan"

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A peer behind a NAT that rewrote its IKE source port receives ESP on that port | interop + Linux integration | GOAL-D1 |
| An EAP-TLS 1.2 full handshake carries a server-chosen session_id | wire unit test + interop | `TestRFC5216TLS12FullHandshakeServerHelloSessionID` reads the ServerHello bytes (32-octet id, red at the unpatched tree); GOAL-D2 |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | | |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | REVIEW-ARTIFACT |
| `./le spec review check` | REVIEW-CHECK |
| Rounds | 6 (round 6 authorised by Thomas, 2026-10-21: "ok to both") |
| Round 6 scope | the round-5 fixes in `a1cbd95e05` (`create_child.go`, `inbound.go`, `rfc7296_child_rekey_transport_test.go`, `ipsec-8-ikev2-child-xfrm.md`) and the sibling call sites they touch (`rekeyRefusedWait` and every rekey-refusal branch of `handleOwnedInbound`, `initiateChildRekey`); the `responder-eap-tls12` re-scope (`checkResponderEAPTLS12` in `internal/le/interoplab/ipsec/natport.go`, the scenario's `strongswan.conf`, its rows in `docs/architecture/testing/interop.md`) |
| Reviewer lenses used | logic and wiring, RFC 3948 / RFC 7296 conformance, rebind state machine, security of the endpoint adoption |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | a rekey after a NAT rebind kept the retired pair's peer address | `rebind.go`, `rekey.go`, `create_child.go` | `5754eac4f4` |
| 2-7 | ISSUE | round 1 two, round 2 four (address-move handling, log wording, test coverage of the address case) | `rebind.go`, `child.go`, `xfrm_linux.go`, pages | `5754eac4f4` |
| 8-9 | ISSUE | round 3 two (the rebind log names the installed mode and the fix command) | `rebind.go`, `ipsec-9-ikev2-eap-nat.md` | `25938b3000` |
| 10 | ISSUE | round 5 (over `9d60c1d841`): the `errChildRekeyTransportDeclined` branch set no rekey hold, so a tunnel-mode peer drew a rekey and a Delete every second | `inbound.go` | the hold `rekeyRefusedWait`, `TestChildRekeyTransportRefusalDeletesThePairAndHolds` (red with the hold line removed); the same gap in the `errChildEncapPort` and `default` branches is journaled (`silent-fall-through.md`) |
| 11 | ISSUE | round 5: a test comment said the refusal reaches the default branch | `rfc7296_child_rekey_transport_test.go` | comment corrected |
| 12 | ISSUE | round 6: `interop.md` claimed every scenario's walk was recorded while the spec still said `responder-eap-tls12` was red and owed a decision | `docs/architecture/testing/interop.md`, this spec's runs table | page reworded to the observed results; runs table carries the 22:07 green and the 22:10 red |

Round 6 (authorised by Thomas, 2026-10-21): 0 BLOCKER, 1 ISSUE (row 12), 5 NOTEs. The a1cbd95e05 hold is correct and covers both level triggers through `startChildRekey`; the re-scoped checker's strings match the producers. NOTEs: N1 the second tick in `TestChildRekeyTransportRefusalDeletesThePairAndHolds` would pass without the hold, because the unanswered Delete holds the request window (the `rekeyHeld` assertion does prove it); N2 `delete.go` says `sendDeleteESP` has one caller, it has seven; N3 four spec rows said the scenario authenticates (fixed); N4 `checkResponderEAPTLS12` carries no RFC7627-5.4 tag; N5 the shared index stages a reversal of `9d60c1d841` and `a1cbd95e05`. Round 6 found an ISSUE, so a seventh round needs Thomas before closure.

Round 4: 0 BLOCKER, 0 ISSUE, three NOTEs (the "ze defect" refusal in `installChildSA` names no next step; the lost-tunnel row has no configured-remote qualifier; a constant comment says "the peer declined it", which fits only the initiator). The round 1 to 4 findings files were session scratch; their content is summarised here and in `plan/handover/14-ike-eap-rfc-defects-closure.md`.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| Files to Create (24 paths: the 14 entries expanded, the 4 scenario directories, `gen/{stdpatch,stamp,tree}.go`, both `register.go`, `actions.go`, the test files) | yes | `ls -d` of all 24 on 2026-10-21 printed 24 paths and no "No such file" |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-5, AC-6, AC-11, AC-14, AC-2 | whole stack against strongSwan 5.9.14 | INTEROP-TABLE |
| AC-1 | Linux XFRM state read back | LINUX-INT |
| all unit ACs | unit suites green | FULL-VERIFY |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| strongSwan behind a port-translating NAT, and its rebind | no `.ci` (Functional Tests: N-A); interop scenarios instead | INTEROP-TABLE |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | unit wiring tests (`8233b4e305`); interop half: A1-EVIDENCE |
| A-2 | confirmed | read of `apply.go` / `encap_linux.go`; LINUX-INT |
| A-3 | confirmed | scratch probe 2026-10-14 and `TestRFC5216TLS12FullHandshakeServerHelloSessionID` |
| A-4 | A4-STATUS | A4-EVIDENCE |
| A-5 | confirmed | strongSwan 5.9.14 `tls_peer.c` `process_server_hello` (row above) |
| A-6 | A6-STATUS | A6-EVIDENCE |
| A-7 | confirmed | strongSwan 5.9.14 `ike_sa.c` `update_hosts` (row above) |
| A-8 | confirmed | A8-EVIDENCE |
| A-9 | confirmed | strongSwan 5.9.14 `float_ports` caller (row above) |
| A-10 | broken | probe 2026-10-14; Mistake Log row 1; Q-5 (A) |
| A-11 | confirmed | scratch probe 2026-10-14 |
| A-12 | confirmed | `TestActivateSetsTheProcessEnvironment`, `TestLeChildInheritsThePatchedGOROOT`; CI runs above |
| A-13 | confirmed | golangci-lint, staticcheck, govulncheck runs under the tree (CI govulncheck 39407444997, CodeQL 39576850399) |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| ipsec-9 rebind: charon's `update_hosts` reading | `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` line naming `update_hosts`, against A-7 | yes |
| pages and anchors | `./le doc check verify` in the full verify | FULL-VERIFY |
