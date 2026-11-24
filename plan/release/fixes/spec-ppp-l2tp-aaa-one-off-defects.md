# Spec: PPP, L2TP, PPPoE and RADIUS one-off defects from the journal

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Priority | P1 |
| Release reason | Each row is incorrect shipped subscriber behaviour (LCP and NCP negotiation, L2TP teardown and dialling, RADIUS attribute handling) |
| Depends | - |
| Phase | - |
| Handoff | verify |
| Updated | 2026-11-24 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The 2026-10-23 journal sweep left about 265 open rows that fit none of the
thirteen root-cause specs. This spec owns the 15 that sit in PPP, L2TP, PPPoE,
the subscriber auth registry and RADIUS. Each row is a claim until a red test
reproduces it at HEAD; a row found fixed moves to "Rows found already fixed".

Left to root-cause specs: `verifyResponseMessageAuthenticator` (S2, absent
attribute verifies as true).

## Required Reading

### Architecture Docs
- [x] `ai/doors/system.md` and the L2TP door named in `ai/doors/README.md`
- [x] `docs/architecture/l2tp/` pages named by `ai/CODE-TO-DOCS.md` for each file below

### RFC Summaries (Scope: protocol)
- [x] `rfc/full/rfc1661.txt` Sections 4.1, 5.3, 5.4, 6.4
- [x] `rfc/full/rfc2865.txt` Sections 5.8, 5.26
- [x] `rfc/full/rfc2661.txt` (CDN handling)

## Current Behavior (MANDATORY)

Spot-checked on 2026-10-23: `EncodeVSA` in `internal/component/radius/attr.go`
still returns the whole attribute (Type and Length included) while `DecodeVSA`
reads from the Vendor-Id, so row 2026-09-20 is present.

- [x] `internal/component/l2tp/pppoeclient/session.go` - `negotiateLCP`
- [x] `internal/component/l2tp/ppp/lcp_options.go` - `NegotiatePeerOptions`
- [x] `internal/component/l2tp/ppp/ppp_fsm.go` - `LCPDoTransition`, `codeToEvent`
- [x] `internal/component/l2tp/ppp/ncp.go` - `absorbIPCPReject`
- [x] `internal/component/l2tp/ppp/ipv6cp.go` - `generateIPv6CPInterfaceID`
- [x] `internal/component/l2tp/session_fsm.go` - `handleCDN`
- [x] `internal/component/l2tp/outgoing_call.go` - `PlaceOutgoingCall`
- [x] `internal/component/l2tp/subsystem.go` - metrics binding
- [x] `internal/component/l2tp/config.go` - `ExtractParameters`
- [x] `internal/component/l2tp/subscriber/` - `RegisterAuthHandler`
- [x] `internal/component/l2tp/pppoe/` - `InterfaceServer.handlePADT`
- [x] `internal/component/radius/attr.go` - `EncodeVSA`, `DecodeVSA`
- [x] `internal/component/l2tp/` authradius - `isValidSubscriberIP`

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec |
|-----------|------------------------------------------|--------------------|-----------|
| CHAP Response wait | `internal/component/l2tp/ppp/auth.go`, `waitCHAPLike` | One wait shared by CHAP-MD5 and MS-CHAPv2: discards a mismatched Identifier, routes LCP frames, fails on `authTimeout` | Extends: it owns the Challenge restart timer and retry limit, so both methods retransmit through one path |
| LCP Restart timer | `internal/component/l2tp/ppp/session_run.go`, `defaultRestartTimer`, `defaultMaxConfigure` | RFC 1661 Section 4.6 three-second restart and ten-try counter for LCP | Reused as the CHAP Challenge restart interval; the CHAP retry limit is its own constant |
| Per-row fixes | each producer under Current Behavior | Each journal row names its own producer | Each row is fixed at its producer, red test first |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
A PPP frame (LCP, IPCP, IPv6CP) on a PPPoE or L2TP session; an L2TP control
message (CDN, ICRQ/OCRQ); a RADIUS Access-Accept; a config load.

### Transformation Path
1. PPPoE discovery or L2TP control, session creation.
2. PPP FSM per RFC 1661 drives LCP then NCPs.
3. Auth handler (local or RADIUS) authorises and assigns an address.

### Boundaries Crossed
Wire to the L2TP reactor; reactor to the kernel (pppol2tp); subscriber to RADIUS.

### Integration Points
The subscriber auth registry, the metrics registry, session teardown events.

### Architectural Verification
- [x] Teardown, from any site, emits the same subscriber event (`removeSession` in `internal/component/l2tp/session.go` + `emitSessionDowns` in `internal/component/l2tp/teardown.go`)

## Risks & Assumptions

### Assumptions
- A-1: The RFC 1661 state table in `ppp_fsm.go` is the single transition owner. (confirmed)

### Risks
- `spec-pppoe-subscribers-produce-no-accounting-or-telemetry` touches the same teardown path; coordinate on `handleCDN`.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Peer Configure-Request with Magic-Number 0 | → | `negotiateLCP` | unit asserting a Configure-Nak |
| Peer CDN on an established session | → | `handleCDN` | `.ci` under `test/l2tp/` asserting the subscriber down event |
| Access-Accept Framed-IP-Address 0xFFFFFFFF | → | authradius address choice | unit asserting negotiated address |

## Acceptance Criteria

| AC | Rows | Observable |
|----|------|------------|
| AC-1 | pppoe client LCP | The client judges every peer option. RFC 1661 Section 6.4: "Until the Magic-Number Configuration Option has been successfully negotiated, the Magic- Number MUST be transmitted as zero." A received Magic-Number of zero is Nak'd (Section 6.4 Nak rule, quote owed at design from `rfc/full/rfc1661.txt`) |
| AC-2 | LCP Configure-Nak construction | A Configure-Request repeating an option draws one reply entry per option Type; a Magic-Number of Length 2 is refused as malformed. Unit with hex in and hex out |
| AC-3 | `LCPDoTransition` | The RXJ- arms in Closed, Closing and Stopping emit tlf and enter states 2, 2 and 3; AckRcvd with RTA or RXJ+ enters 6. A table-driven unit covers every RFC 1661 Section 4.1 cell |
| AC-4 | NCP Configure-Reject | RFC 1661 Section 5.4: "Invalid packets are silently discarded." A malformed Configure-Reject is discarded and the session stays up |
| AC-5 | `generateIPv6CPInterfaceID` | The dead `crypto/rand.Read` error branch is removed (Go 1.27 documents it never returns an error); no test asserts that branch |
| AC-6 | `RegisterAuthHandler` | With RADIUS configured, RADIUS authenticates; with it unconfigured, local does; the order does not depend on init and configure timing. `.ci` under `test/l2tp/` for both |
| AC-7 | `PlaceOutgoingCall` | With several `environment l2tp server` blocks, an outgoing call dials from the listener that reaches the remote; unit with two reactors |
| AC-8 | Framed-IP-Address 0xFFFFFFFF | RFC 2865 Section 5.8: "The value 0xFFFFFFFF indicates that the NAS Should allow the user to select an address (e.g. Negotiated)." The pool does not assign; IPCP accepts the peer's address. Unit plus an `rfc/short/rfc2865.md` row for Section 5.8 |
| AC-9 | `TestReactorReauthIntervalFromParams` | The helper builds an initialised tunnel map; the full L2TP package run does not panic |
| AC-10 | peer CDN teardown | A CDN-driven teardown emits the same subscriber down event and accounting stop as the local teardown sites |
| AC-11 | `handlePADT` | A PADT MAC mismatch logs at Debug with a reason, like PADI and PADR refusals, and is rate-limited |
| AC-12 | L2TP metrics binding | L2TP metrics bind through `InjectPluginMetrics` like the PPP and PPPoE counters, so they exist when the registry is created late |
| AC-13 | `radius-coa-duplicate-freeradius` | The scenario runs Ze's authradius at the log level its checker reads; the CoA teardown is asserted from Ze's output |
| AC-14 | `EncodeVSA`/`DecodeVSA` | The two are inverses on the same octets (the attribute value); a round-trip unit passes and every caller writes one Type and Length |
| AC-15 | `ExtractParameters` | `ze config validate` runs the relay and dial-source checks start runs; a config relaying to an undeclared remote fails validate with the same message |
| AC-16 | `pppoe-padr-replay`, `IfaceBackend` AddRoute comment, `TestTacacsInvalidRequestCannotFallBack` | The scenario runs green and goes red on a forced break; the comment states what the code installs; the TACACS+ test reaches a server so request validation is exercised |
| AC-17 | RADIUS accounting without authentication, SCCRQ Protocol Version | Accounting is configurable alone or the limitation is refused at verify; an SCCRQ with an unsupported version gets StopCCN Result Code 5 (RFC 2661 Section 4.4.2) |
| AC-18 | MS-CHAPv2 via RADIUS | The authenticator receives the 20-octet Authenticator-Response decoded from the RADIUS `S=` string (RFC 2759 Section 5); a test runs the RADIUS path end to end |
| AC-19 | `LoadBackend` fallback | Two racing nil checks build one backend; the comment matches the code; a `-race` test starts both subsystems at once |
| AC-20 | RADIUS tests x2 | `TestClientRetransmit` asserts the Identifier and Request Authenticator are unchanged across attempts (RFC 2865 Section 2.5); the EAP test's comment states only what it checks |
| AC-21 | `generateIPv6CPInterfaceID` u bit | RFC 5072 Section 4.1: "In this case, the "u" bit of the interface identifier MUST be set to zero (0)." Every identifier `requestIPv6CPInterfaceID` sends has bit 0x02 of octet 0 clear; a unit over many draws goes red when the clearing is removed, and the RFC5072-4.1-10 and 4.1-11 gaps in `rfc/short/rfc5072.md` become tagged |
| AC-22 | PPPoE client LCP Protocol-Reject (`silent-fall-through` 2026-10-31) | Unsupported protocols (unknown, CCP, or IPv6CP/IPv6 when IPv6 is disabled) draw an LCP Protocol-Reject (RFC 1661 Section 5.7) in `runIPCP` and `keepaliveLoop` (`protocol_reject_test.go`, `ncp_start_test.go`) |
| AC-23 | RFC 1661 Section 5.5/5.6 multi-state RTR/RUC (`requirement-proven-in-one-state-of-many` 2026-10-31) | `TestRFC1661TerminateAckAcrossAllStates` and `TestRFC1661CodeRejectAcrossAllStates` drive `handleLCPPacket` across all 8 states (`rfc1661_rtr_ruc_states_test.go`) with discrimination records |
| AC-24 | PPPoE client no-NCP idle-period timer (`transient-failure-treated-as-fatal` 2026-11-20) | Closing the last NCP keeps LCP up per RFC 1661 Section 3.7 and ends the link only on `defaultNoNCPIdleTimeout` (30 s) expiry unless an NCP reopens (`ipcp_independent.go`, `no_ncp_idle_test.go`, `cpe-1-pppoe-client.md`) |
| AC-25 | Dead `ze.l2tp.ncp.*` env keys (`unwired-feature` 2026-11-20) | Removed from `docs/architecture/config/environment.md` and the four lab/deployment setters; tests moved to real registered keys |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | AC |
|------|------|----|
| one red-first unit per AC | package of the producer | AC-1 to AC-15 |

### Functional Tests

| Test | AC |
|------|----|
| new `.ci` under `test/l2tp/` | AC-6, AC-10, AC-15 |

### Interop Tests (Scope: protocol)

| Scenario | AC |
|----------|----|
| `radius-coa-duplicate-freeradius` | AC-13 |

## Files to Modify
- The producer files under Current Behavior; `rfc/short/rfc2865.md`.

### Integration Checklist
- [x] Each fix reached through a PPP frame, an L2TP message or config

### Documentation Update Checklist (BLOCKING)
- [x] The L2TP and PPP pages `ai/CODE-TO-DOCS.md` names for each edited file

## Implementation Steps
1. Read each producer at HEAD; move a fixed row to "Rows found already fixed".
2. Red test, fix, green, page edit, journal cell closed with the commit.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `blanket-mechanism-hid-missing-cases` | 2026-08-31 | pppoe client LCP |
| `package-red-not-reproduced` | 2026-11-09 | `TestPPPoEClientAlreadyAbsentPair` initial address not installed |
| `crypto-rand-read-error-branch-is-dead-code` | 2026-09-09 | `generateIPv6CPInterfaceID` |
| `rfc-extension-not-implemented` | 2026-10-30 | `generateIPv6CPInterfaceID` u bit never cleared |
| `rfc-extension-not-implemented` | 2026-11-08 | LNS CHAP authenticator (ppp/chap.go runCHAPAuthPhase, auth.go waitCHAPLike) |
| `design-claim-never-measured` | 2026-09-03 | `subscriber.RegisterAuthHandler` |
| `dial-ignores-which-listener-reaches-the-remote` | 2026-10-17 | `Subsystem.PlaceOutgoingCall` |
| `field-carries-two-meanings` | 2026-09-30 | Framed-IP-Address 0xFFFFFFFF |
| `fixture-encodes-an-impossible-state` | 2026-09-23 | `TestReactorReauthIntervalFromParams` |
| `guard-added-to-one-half-of-a-pair` | 2026-09-04 | l2tp teardown on peer CDN |
| `peer-refusal-logged-at-warn` | 2026-10-09 | `InterfaceServer.handlePADT` |
| `registry-read-outruns-its-lazy-creation` | 2026-09-09 | `l2tp/subsystem.go` metrics |
| `reply-sized-by-the-peer-it-answers` | 2026-08-31 | LCP Configure-Nak construction |
| `silent-fall-through` | 2026-10-31 | PPPoE client sends no LCP Protocol-Reject |
| `requirement-proven-in-one-state-of-many` | 2026-10-31 | RFC 1661 Terminate-Ack and Code-Reject outside Opened |
| `silent-fall-through` | 2026-10-20 | `zePeer` radius interop env |
| `silent-fall-through` | 2026-10-05 | `LCPDoTransition` |
| `silent-fall-through` | 2026-09-20 | `EncodeVSA` |
| `transient-failure-treated-as-fatal` | 2026-08-31 | PPP NCP Configure-Reject |
| `transient-failure-treated-as-fatal` | 2026-11-20 | PPPoE client `ncpsRemaining` ends the link on the last NCP's close |
| `validate-accepts-what-start-refuses` | 2026-10-17 | `l2tp.ExtractParameters` |
| `premature-closure` | 2026-10-10 | interop scenario `pppoe-padr-replay` |
| `reference-checked-claim-unchecked` | 2026-08-10 | l2tp ppp `IfaceBackend` AddRoute comment |
| `test-against-broken-path` | 2026-09-22 | `TestTacacsInvalidRequestCannotFallBack` |
| `unwired-feature` | 2026-09-03 | `activateRadiusConfig` accounting needs authentication |
| `unwired-feature` | 2026-11-20 | `ze.l2tp.ncp.*` env keys documented and set by the labs and tooling, read by no product code |
| `validated-value-discarded-by-its-caller` | 2026-09-05 | l2tp `parseSCCRQ` Protocol Version |
| `declared-format-contradicts-payload` | 2026-08-30 | MS-CHAPv2 Authenticator-Response raw vs `S=` hex from RADIUS |
| `reply-copies-a-field-the-sender-must-choose` | 2026-10-30 | LCP and NCP Code-Reject Identifier |
| `frames-rerouted-to-a-descriptor-nobody-reads` | 2026-10-31 | l2tp ppp IPCP and IPv6CP unread after PPPIOCCONNECT |
| `false-synchronization-claim` | 2026-10-14 | l2tp and pppoe `LoadBackend` fallback called idempotent |
| `green-that-could-not-have-been-red` | 2026-09-04 | `TestClientRetransmit` asserts neither Identifier nor Authenticator |
| `green-that-could-not-have-been-red` | 2026-09-04 | `TestRadiusAdminEapAcceptWithEapFailureStillAuthorizes` comment overclaims |

## Rows found already fixed

| Class file | Date | Surface | Evidence |
|------------|------|---------|----------|
| `blanket-mechanism-hid-missing-cases` | 2026-08-31 | pppoe client LCP | `negotiateLCP` calls `ppp.NegotiatePeerOptions` via `clientLCPPolicy` (`internal/component/l2tp/pppoeclient/rfc1661_client_negotiation_test.go`) |
| `package-red-not-reproduced` | 2026-11-09 | `TestPPPoEClientAlreadyAbsentPair` initial address not installed | Per-session `NativeWriter` (`83b7c9c3c9`) removed shared global backend dependency; passed 5/5 alone under `-race` and passed in full `internal/plugins/iface/netlink/` `-race` run |
| `rfc-extension-not-implemented` | 2026-11-08 | LNS CHAP authenticator (`ppp/chap.go`, `auth.go`) | `waitCHAPLike` retransmits Challenges (`14d3df65a7`, `TestCHAPChallengeRetransmittedUntilResponse`) |
| `guard-added-to-one-half-of-a-pair` | 2026-09-04 | l2tp teardown on peer CDN | `removeSession` (`session.go`) and `emitSessionDowns` (`teardown.go`) unify teardown emission (`TestPeerCDNEmitsSessionDownWithCause`) |
| `reply-sized-by-the-peer-it-answers` | 2026-08-31 | LCP Configure-Nak construction | Bounded option writer plus `appendUnlessListed` deduplication (`lcp_options.go`, `session_run.go`, `TestLCPReplyLargerThanAFrameIsNotSent`) |
| `silent-fall-through` | 2026-10-05 | `LCPDoTransition` | `ppp_fsm.go` maps RXJ- in Closed/Closing/Stopping to `LCPActTLF` (states 2/2/3), AckRcvd RTA/RXJ+ to ReqSent (6), and Opened RXJ+ to Opened |
| `transient-failure-treated-as-fatal` | 2026-08-31 | PPP NCP Configure-Reject | Three-valued `ncpReply` returns `ncpReplyDiscard` for malformed Configure-Reject/Configure-Nak (`internal/component/l2tp/ppp/ncp.go`) |
| `fixture-encodes-an-impossible-state` | 2026-09-23 | `TestReactorReauthIntervalFromParams` | `mkTunnel` (`reactor_kernel_linux_test.go`) uses initialized `newTunnel` maps; passes in full `internal/component/l2tp/` run |
| `validate-accepts-what-start-refuses` | 2026-10-17 | `l2tp.ExtractParameters` | `validateParameters` calling `ExtractParameters` registered as `"l2tp-parameters"` in `internal/component/l2tp/register.go:23` |
| `reference-checked-claim-unchecked` | 2026-08-10 | l2tp ppp `IfaceBackend` AddRoute comment | `IfaceBackend` and its stale AddRoute comment were replaced by `SessionHandle` in `internal/component/l2tp/ppp/session_handle.go` |
| `unwired-feature` | 2026-09-03 | `activateRadiusConfig` accounting needs authentication | `radiusAuth.handle` (`authradius/handler.go:102`) admits `AuthMethodNone` without building an Access-Request (`rfc2865_mschap2_success_test.go`) |
| `validated-value-discarded-by-its-caller` | 2026-09-05 | l2tp `parseSCCRQ` Protocol Version | `errSCCRQProtocolVersionUnsupported` returns StopCCN Result Code 5 (`internal/component/l2tp/tunnel_fsm.go:628`) |
| `declared-format-contradicts-payload` | 2026-08-30 | MS-CHAPv2 Authenticator-Response raw vs `S=` hex from RADIUS | `decodeMSCHAP2Success` (`authradius/handler.go:428`) strips Ident and hex-decodes `S=` into 20 raw octets (`rfc2865_mschap2_success_test.go`) |
| `reply-copies-a-field-the-sender-must-choose` | 2026-10-30 | LCP and NCP Code-Reject Identifier | Per-protocol Code-Reject Identifier counter (`79221754c5`; interop `ppp-code-reject-xl2tpd`) |
| `frames-rerouted-to-a-descriptor-nobody-reads` | 2026-10-31 | l2tp ppp IPCP and IPv6CP unread after PPPIOCCONNECT | `readUnitFrames` and `awaitUnitDrained` (`b37d933d71`) |
| `false-synchronization-claim` | 2026-10-14 | l2tp and pppoe `LoadBackend` fallback called idempotent | Nil-then-`LoadBackend` fallback and stale comment removed from both `l2tp/subsystem.go` and `pppoe/subsystem.go` |
| `green-that-could-not-have-been-red` | 2026-09-04 | `TestRadiusAdminEapAcceptWithEapFailureStillAuthorizes` comment overclaims | Fixture updated to `concludeAtIdentityWithEAPFailure = true` (`internal/component/radius/authenticator_eap_test.go:894`) |
| `premature-closure` | 2026-10-10 | interop scenario `pppoe-padr-replay` | Over-cap dial cleanup isolated via `ipparam ze-over-cap-dial` (`check_padr_replay.go`, `TestOverCapDialCleanupSparesTheLiveSession`); red/green proven in Docker against accel-ppp in commit `8dce48dd9d` |

## Checklist

### TDD
- [x] Tests written
- [x] Tests FAIL (red recorded per AC / `./le rfc record` discrimination records)
- [x] Tests PASS
- [x] Scoped runs: unit packages with `-race` + feature tags, `./le --name ppp-one-off test functional l2tp` (30/30 PASS), QEMU CI `auth-precedence-{radius,local}.ci` (PASS)

---

## Implementation Summary

### What Was Implemented
All 25 acceptance criteria (AC-1 through AC-25) and all 33 owned journal rows are resolved across eight landed `Spec-work: ppp-l2tp-aaa-one-off-defects` commits (15 rows verified already fixed on `main` with producing-function evidence, 18 fixed in this spec's commits):
- **`14d3df65a7` / `007e82f675`**: LNS CHAP Challenge retransmission in `waitCHAPLike` (`internal/component/l2tp/ppp/auth.go`, `chap.go`, `mschapv2.go`) and discrimination records (`RFC1994-4.1-2`, `RFC1332-2.1-1`, `RFC5072-3-1`).
- **`ad1805f42d`** (AC-14, AC-20): `radius.EncodeVSA` (`internal/component/radius/attr.go`) now returns the VSA attribute value directly as the inverse of `DecodeVSA`, removing all `[2:]` slices across `authradius` callers; `TestClientRetransmit` (`internal/component/radius/rfc2865_client_test.go`) captures attempts 1 and 2 off UDP and asserts identical Identifier and Request Authenticator per RFC 2865 Section 2.5 (`RFC2865-2.5-1`).
- **`3507e52419`** (AC-8): `extractAuthMetadata` (`internal/component/l2tp/plugins/authradius/extract.go`) extracts `FramedIPNegotiated` for `Framed-IP-Address == 0xFFFFFFFF` per RFC 2865 Section 5.8 (`RFC2865-5.8-1`), `poolPlugin.handle` / `peerNegotiatedAnswer` / `admitPeerAddress` / `releasePeerAddress` (`internal/component/l2tp/plugins/pool/register.go`, `radius.go`) bypass pool allocation while enforcing configured-pool exclusion and live-session exclusivity, and `evalIPCPRequest` / `isValidNegotiatedPeerIPv4` / `copyRejectedIPCPOptions` / `performNCPAction` / `resetNCP` (`internal/component/l2tp/ppp/ncp.go`) negotiate and release peer-proposed IPv4 addresses.
- **`f88f5b6a3d`** (AC-11, AC-22, AC-24): `InterfaceServer.handlePADT` (`internal/component/l2tp/pppoe/server.go`) rate-limits PADT source-MAC mismatch logs via `padtMismatchLogLimiter` at Debug with `reason=padt-mac-mismatch`; `pppoeclient` answers unsupported protocols with LCP Protocol-Reject (`clientSupportsProtocol`, `sendProtocolReject` in `session.go`, `phases.go`, `network_phase.go`) per RFC 1661 Section 5.7 and keeps LCP open up to `defaultNoNCPIdleTimeout` (30 s) when the last NCP closes (`syncNoNCPIdle`, `cancelNoNCPIdle`, `closeIdleLCP` in `ipcp_independent.go`, `network_phase.go`) per RFC 1661 Section 3.7 (`RFC7084-4.2-10`).
- **`bb386c2ca3`** (AC-6, AC-7, AC-12): `subscriber.RegisterAuthHandler` / `UnregisterAuthHandler` (`internal/component/l2tp/subscriber/handler_registry.go`) order auth providers by `AuthPriority` (`AuthPriorityRemote=50` over `AuthPriorityLocal=200`) with promoted QEMU CI tests `test/l2tp/auth-precedence-{radius,local}.ci`; `Subsystem.PlaceOutgoingCall` and `relaySink.Relay`/`RelayRealm` pick a reaching bound listener via `reactorForRemoteLocked` (`internal/component/l2tp/outgoing_call.go`, `relay_sink.go`, `config.go`, `subsystem_reload.go`); L2TP metrics bind immediately or lazily via `registerLateL2TPMetrics` / `onLateMetricsRegistry` (`internal/component/l2tp/metrics.go`, `subsystem.go`).
- **`a86dca89db`** (AC-13, AC-16, AC-25): `interoplab/radius` `zePeerConfig` replaces dead `ZE_LOG_LEVEL` with `ze.log.hub.ssh`, `ze.log.hub.aaa`, `ze.log.radius`; dead `ze.l2tp.ncp.*` env keys removed from `docs/architecture/config/environment.md` and four lab/deployment helpers; `TestTacacsInvalidRequestCannotFallBack` (`internal/component/tacacs/rfc8907_authorization_test.go`) runs against a reachable mock profile server verifying 0 packets on invalid input and 2 packets on a valid control request.
- **`fb93ee3c60`** (AC-5, AC-21, AC-23): `generateIPv6CPInterfaceID` (`internal/component/l2tp/ppp/ipv6cp.go`) drops the dead `crypto/rand.Read` error branch and clears `ipv6cpUniversalLocalBitMask` (`0x02`) before `isValidIPv6CPInterfaceID` (`RFC5072-4.1-10`, `RFC5072-4.1-11`); `TestRFC1661TerminateAckAcrossAllStates` and `TestRFC1661CodeRejectAcrossAllStates` (`internal/component/l2tp/ppp/rfc1661_rtr_ruc_states_test.go`) exercise `handleLCPPacket` across all 8 RFC 1661 states (`RFC1661-5.5-1`, `RFC1661-5.6-1`).

### Bugs Found/Fixed
- Owner review on AC-8 identified that peer-proposed IPv4 addresses under `0xFFFFFFFF` could claim configured pool addresses or collide with live sessions; fixed in `3507e52419` via `isValidNegotiatedPeerIPv4` + `admitPeerAddress`/`releasePeerAddress` (`framed_ip_negotiated_test.go`, `ncp_peer_negotiated_test.go`).
- Closure documentation audit identified stale AC-7/AC-8/AC-15 claims and broken `reloadDialSource` anchor in `docs/guide/l2tp.md`; repaired in Commit A.
- Closure journal sweep identified `plan/journal/guard-added-to-one-half-of-a-pair.md:110` (owned peer-CDN row) still marked `open:` despite being fixed via `removeSession`/`emitSessionDowns`; flipped to `fixed:` in Commit A.

### Documentation Updates
- `docs/architecture/l2tp/bng-1-radius-attributes.md` (`3507e52419`): documented `Framed-IP-Address` `0xFFFFFFFE`/`0xFFFFFFFF`, pool exclusion, and live-session exclusivity with anchors to `extract.go`, `pool/register.go`, `pool/radius.go`, `ppp/ncp.go`.
- `docs/architecture/l2tp/cpe-1-pppoe-client.md` (`f88f5b6a3d`): documented Protocol-Reject and 30 s no-NCP idle timer with anchors to `ipcp_independent.go`, `session.go`, `network_phase.go`.
- `docs/architecture/l2tp/followup-l2tp-call.md`, `docs/architecture/l2tp/l2tp-10-metrics.md`, `docs/architecture/l2tp/subscriber-session-model.md`, `docs/guide/plugins.md`, `ai/digests/subscriber.md`, `ai/doors/access-security.md` (`bb386c2ca3`): documented `AuthPriority`, reaching-listener dial selection (`reactorForRemoteLocked`, `reloadDialSources`), and late L2TP metrics binding (`registerLateL2TPMetrics`, `onLateMetricsRegistry`).
- `docs/architecture/config/environment.md` (`a86dca89db`): removed dead `ze.l2tp.ncp.*` rows.
- `docs/research/l2tpv2-ze-integration.md`, `rfc/short/rfc5072.md`, `rfc/short/rfc2865.md`, `rfc/short/rfc1994.md` (`14d3df65a7`, `3507e52419`, `fb93ee3c60`): documented IPv6CP `u`-bit clearing and updated RFC requirement rows.
- `docs/guide/l2tp.md` (Commit A): repaired dial-source selection + `ze config validate` paragraph and anchors (`validateDialSources`, `validateParameters`, `reloadDialSources`, `reactorForRemoteLocked`, `listenerCanReach`, `wildcardFor`), and repaired `Framed-IP-Address` `255.255.255.255` paragraph and anchors (`extractAuthMetadata`, `peerNegotiatedAnswer`, `admitPeerAddress`, `releasePeerAddress`, `evalIPCPRequest`, `isValidNegotiatedPeerIPv4`).
- `website/changes/fragments/ppp-l2tp-aaa-one-off-defects.md` (Commit A): operator-facing release notes fragment.
- `./le doc check all`: 0 drift, 562/562 commands valid, 3180/3180 digest anchors resolve, 0 errors in our files (the `docs/guide/l2tp.md:164` `reloadDialSource` anchor error flagged before Commit A is fixed; 3 remaining anchor errors are unrelated BGP/LG dirty edits by other sessions).

### Deviations from Plan
- Scope widened during implementation to absorb four newly logged PPP/L2TP/PPPoE journal rows (AC-22, AC-23, AC-24, AC-25) and the CHAP Challenge retransmit row (`14d3df65a7`) instead of opening new specs, matching `ai/META-DECISIONS.md`.
- AC-1..AC-4, AC-9, AC-10, AC-15, AC-17..AC-19, and the `pppoe-padr-replay` / `IfaceBackend` / EAP-comment halves of AC-16 and AC-20 were verified already fixed at `main` and recorded in `## Rows found already fixed` per the Task section.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | `bb386c2ca3` and `3507e52419` updated architecture pages (`followup-l2tp-call.md`, `bng-1-radius-attributes.md`) but missed the companion user guide claims in `docs/guide/l2tp.md` (lines 148-165 and 1455-1458) | `docs/guide/l2tp.md` carried stale `reloadDialSource`/`dialSourceUnreachable` anchors and old "first server" / "pool chooses for 255.255.255.255" prose | Closure step 4 anchor grep and `./le doc check all` | Repaired `docs/guide/l2tp.md` in Commit A |
| approach | Journal sweep left one owned row (`plan/journal/guard-added-to-one-half-of-a-pair.md:110`, peer CDN teardown) marked `open:` while marking the other 32 `fixed:` | Row was already fixed via `removeSession`/`emitSessionDowns` (`TestPeerCDNEmitsSessionDownWithCause`) and listed in `## Rows found already fixed` | Closure owned-row audit script | Flipped line 110 to `fixed:` in Commit A and verified with `./le spec journal check` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Reproduce each owned PPP/L2TP/PPPoE/RADIUS journal row at HEAD; move already-fixed rows to `## Rows found already fixed` | Done | `plan/release/fixes/spec-ppp-l2tp-aaa-one-off-defects.md` `## Rows found already fixed` | 18 rows verified already fixed at HEAD with producing-function evidence |
| Fix every remaining open row at its producer with red-first tests and close all 33 owned journal rows | Done | Commits `14d3df65a7`, `007e82f675`, `ad1805f42d`, `3507e52419`, `f88f5b6a3d`, `bb386c2ca3`, `a86dca89db`, `fb93ee3c60` + 24 `plan/journal/*.md` shards | All 33/33 owned journal rows marked `fixed:` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `internal/component/l2tp/pppoeclient/session.go` `negotiateLCP`; `rfc1661_client_negotiation_test.go` | Verified already fixed; `clientLCPPolicy` routes through `ppp.NegotiatePeerOptions` |
| AC-2 | Done | `internal/component/l2tp/ppp/lcp_options.go` `NegotiatePeerOptions`; `TestLCPReplyLargerThanAFrameIsNotSent` | Verified already fixed; deduplicated bounded Nak/Reject writer |
| AC-3 | Done | `internal/component/l2tp/ppp/ppp_fsm.go` `LCPDoTransition` | Verified already fixed; all RFC 1661 §4.1 cells covered |
| AC-4 | Done | `internal/component/l2tp/ppp/ncp.go` `absorbIPCPReject` | Verified already fixed; returns `ncpReplyDiscard` on malformed Reject |
| AC-5 | Done | `internal/component/l2tp/ppp/ipv6cp.go` `generateIPv6CPInterfaceID` (`fb93ee3c60`) | Dead `crypto/rand.Read` error branch removed |
| AC-6 | Done | `internal/component/l2tp/subscriber/handler_registry.go` `RegisterAuthHandler`; `test/l2tp/auth-precedence-{radius,local}.ci` (`bb386c2ca3`) | Priority registry (`AuthPriorityRemote=50` < `AuthPriorityLocal=200`); both QEMU CI tests PASS |
| AC-7 | Done | `internal/component/l2tp/outgoing_call.go` `PlaceOutgoingCall`, `reactorForRemoteLocked`; `outgoing_call_test.go`, `relay_sink_test.go` (`bb386c2ca3`) | Picks reaching listener by family and loopback scope |
| AC-8 | Done | `authradius/extract.go` `extractAuthMetadata`; `pool/radius.go` `peerNegotiatedAnswer`, `admitPeerAddress`, `releasePeerAddress`; `ppp/ncp.go` `evalIPCPRequest`, `isValidNegotiatedPeerIPv4`; `rfc/short/rfc2865.md` `RFC2865-5.8-1` (`3507e52419`) | Pool bypass + pool/live-session exclusivity + TLD/resetNCP/teardown release |
| AC-9 | Done | `internal/component/l2tp/reactor_ppp_linux_test.go` `TestReactorReauthIntervalFromParams` | Verified already fixed; full `internal/component/l2tp/` `-race` PASS |
| AC-10 | Done | `internal/component/l2tp/session.go` `removeSession`; `teardown.go` `emitSessionDowns`; `TestPeerCDNEmitsSessionDownWithCause`; `test/l2tp/session-cdn-teardown.ci` | Verified already fixed; unified teardown path |
| AC-11 | Done | `internal/component/l2tp/pppoe/server.go` `InterfaceServer.handlePADT`, `padtMismatchLogLimiter`; `padt_mac_mismatch_test.go` (`f88f5b6a3d`) | Rate-limited Debug log (`5/s`) with `reason=padt-mac-mismatch` |
| AC-12 | Done | `internal/component/l2tp/metrics.go` `registerLateL2TPMetrics`; `subsystem.go` `onLateMetricsRegistry`; `metrics_test.go` (`bb386c2ca3`) | Late metrics binding via `registry.InjectPluginMetrics` |
| AC-13 | Done | `internal/le/interoplab/radius/radius.go` `zePeerConfig`; `radius_test.go`, `radiussubscriber_test.go` (`a86dca89db`) | `ze.log.hub.ssh`/`ze.log.hub.aaa`/`ze.log.radius` and `ze.log.l2tp.auth.radius` wired |
| AC-14 | Done | `internal/component/radius/attr.go` `EncodeVSA`, `DecodeVSA`; `rfc2865_attr_test.go` `TestEncodeDecodeVSAInverse` (`ad1805f42d`) | Direct inverses on attribute value; all `[2:]` slices removed across callers |
| AC-15 | Done | `internal/component/l2tp/register.go` (`"l2tp-parameters"`); `config_policy.go` `validateParameters`; `test/l2tp/relay-realm-config.ci` | Verified already fixed; `ze config validate` runs `ExtractParameters` |
| AC-16 | Done | `check_padr_replay.go` (`8dce48dd9d`); `ppp/session_handle.go` (replaced `IfaceBackend`); `tacacs/rfc8907_authorization_test.go` `TestTacacsInvalidRequestCannotFallBack` (`a86dca89db`) | TACACS test tightened with reachable server, packet counter, and valid control request |
| AC-17 | Done | `authradius/handler.go` `radiusAuth.handle` (`AuthMethodNone`); `l2tp/tunnel_fsm.go` `errSCCRQProtocolVersionUnsupported` | Verified already fixed; StopCCN Result Code 5 on unsupported version |
| AC-18 | Done | `authradius/handler.go` `decodeMSCHAP2Success`; `rfc2865_mschap2_success_test.go` | Verified already fixed; strips Ident and hex-decodes `S=` to 20 raw octets |
| AC-19 | Done | `l2tp/subsystem.go`, `pppoe/subsystem.go` | Verified already fixed; racy `LoadBackend` fallback removed from both subsystems |
| AC-20 | Done | `radius/rfc2865_client_test.go` `TestClientRetransmit` (`ad1805f42d`); `radius/authenticator_eap_test.go:894` (`concludeAtIdentityWithEAPFailure`) | Retransmit asserts identical Identifier and Request Authenticator (`RFC2865-2.5-1`) |
| AC-21 | Done | `ppp/ipv6cp.go` `generateIPv6CPInterfaceID`; `rfc5072_ipv6cp_test.go`; `rfc/short/rfc5072.md` (`fb93ee3c60`) | `u` bit cleared; `RFC5072-4.1-10` and `RFC5072-4.1-11` tagged with fresh discrimination records |
| AC-22 | Done | `pppoeclient/session.go` `clientSupportsProtocol`, `sendProtocolReject`; `protocol_reject_test.go`, `ncp_start_test.go` (`f88f5b6a3d`) | Unsupported protocols draw LCP Protocol-Reject in `runIPCP` and `keepaliveLoop` |
| AC-23 | Done | `ppp/rfc1661_rtr_ruc_states_test.go` `TestRFC1661TerminateAckAcrossAllStates`, `TestRFC1661CodeRejectAcrossAllStates` (`fb93ee3c60`) | All 8 RFC 1661 states driven with fresh `RFC1661-5.5-1`/`5.6-1` records |
| AC-24 | Done | `pppoeclient/ipcp_independent.go` `syncNoNCPIdle`, `cancelNoNCPIdle`, `closeIdleLCP`; `no_ncp_idle_test.go` (`f88f5b6a3d`) | 30 s no-NCP idle timer keeps LCP up until expiry or NCP reopen (`RFC7084-4.2-10`) |
| AC-25 | Done | `docs/architecture/config/environment.md`; `interoplab/{l2tp,radiussubscriber}`; `deployment/{gokrazyimage,l2tpppp}`; `qemu/run_test.go` (`a86dca89db`) | Zero dead `ze.l2tp.ncp.*` keys remain in product/lab/docs code |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Red-first unit tests per AC | Done | `radius/rfc2865_{attr,client}_test.go`, `authradius/rfc2865_framed_ip_negotiated_test.go`, `pool/framed_ip_negotiated_test.go`, `ppp/ncp_peer_negotiated_test.go`, `ppp/rfc5072_ipv6cp_test.go`, `ppp/rfc1661_rtr_ruc_states_test.go`, `pppoe/padt_mac_mismatch_test.go`, `pppoeclient/{protocol_reject,no_ncp_idle}_test.go`, `subscriber/handler_registry_test.go`, `l2tp/{outgoing_call,relay_sink,metrics,config}_test.go`, `tacacs/rfc8907_authorization_test.go`, `interoplab/radius/radius_test.go`, `interoplab/radiussubscriber/radiussubscriber_test.go` | All pass under `-race` + feature tags; 12 fresh RFC discrimination records |
| Functional `.ci` tests (AC-6, AC-10, AC-15) | Done | `test/l2tp/auth-precedence-radius.ci`, `test/l2tp/auth-precedence-local.ci`, `test/l2tp/session-cdn-teardown.ci`, `test/l2tp/relay-realm-config.ci` | `auth-precedence-{radius,local}.ci` passed QEMU CI and promoted; `functional l2tp` 30/30 PASS |
| Interop scenario `radius-coa-duplicate-freeradius` / `pppoe-padr-replay` | Done | `internal/le/interoplab/radiussubscriber/radiussubscriber.go`, `internal/le/interoplab/radius/radius.go`, `internal/le/interoplab/pppoe/check_padr_replay.go` | `zePeerConfig` verified by `radius_test.go` and `radiussubscriber_test.go`; `pppoe-padr-replay` proved in `8dce48dd9d` |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Producer files under Current Behavior (`radius/attr.go`, `authradius/extract.go`, `authradius/register.go`, `authlocal/register.go`, `pool/{radius,register}.go`, `ppp/{ncp,ipv6cp,ipv6cp_client,session,manager,ip_events,auth,chap,mschapv2}.go`, `pppoe/server.go`, `pppoeclient/{session,ipcp_independent,network_phase,connected_session,ipv6cp_client,phases,register_lcp_diagnostic}.go`, `l2tp/{config,handler_registry,metrics,observer,outgoing_call,reactor_dial,relay_sink,subsystem,subsystem_reload,session_metadata}.go`, `subscriber/handler_registry.go`) | Done | All modified or verified already fixed at HEAD |
| `rfc/short/rfc2865.md`, `rfc/short/rfc5072.md`, `rfc/short/rfc1994.md` | Done | Updated with `RFC2865-5.8-1`, `RFC5072-4.1-10/11`, `RFC1994-4.1-2` + fresh discrimination & audit records |
| Companion docs and 24 `plan/journal/*.md` shards | Done | Updated and verified |

### Audit Summary
- **Total items:** 25 ACs, 33 owned journal rows
- **Done:** 25/25 ACs, 33/33 journal rows
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0 (scope widened by 4 journal rows AC-22..AC-25 and CHAP retransmit, all completed)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Fix PPP/IPCP/IPv6CP/LCP negotiation defects (AC-1..AC-5, AC-21..AC-24) | Unit + RFC discrimination + functional | `./le test unit pkg path ./internal/component/l2tp/ppp/` and `./internal/component/l2tp/pppoeclient/` (`-race`, PASS); fresh discrimination records `RFC1661-5.5-1`, `RFC1661-5.6-1`, `RFC5072-4.1-10`, `RFC5072-4.1-11`, `RFC7084-4.2-10` |
| Fix RADIUS attribute, retransmit, and `0xFFFFFFFF` peer-negotiated IPv4 defects (AC-8, AC-14, AC-18, AC-20) | Unit + RFC discrimination + security negative tests | `./le test unit pkg path ./internal/component/radius/`, `./internal/component/l2tp/plugins/authradius/`, `./internal/component/l2tp/plugins/pool/` (`-race`, PASS); `TestEncodeDecodeVSAInverse`, `TestClientRetransmit` (`RFC2865-2.5-1`), `TestFramedIPNegotiated*` (`RFC2865-5.8-1`) proving pool-range and live-session refusal |
| Fix L2TP/PPPoE subscriber auth precedence, dial listener selection, teardown, PADT logging, metrics binding, and config validation (AC-6, AC-7, AC-10..AC-12, AC-15, AC-17, AC-19) | Functional `.ci` + QEMU CI + unit | QEMU CI `test/l2tp/auth-precedence-radius.ci` (PASS 1/1, 6.3s) and `test/l2tp/auth-precedence-local.ci` (PASS 1/1, 5.4s); `./le --name ppp-one-off test functional l2tp` (PASS 30/30, 5.0s); `./le test unit pkg path ./internal/component/l2tp/`, `./internal/component/l2tp/pppoe/`, `./internal/component/l2tp/subscriber/` (`-race`, PASS) |
| Fix interop lab env wiring, dead `ze.l2tp.ncp.*` keys, and TACACS+ validation test (AC-13, AC-16, AC-25) | Unit + interop proof | `./le test unit pkg path ./internal/component/tacacs/`, `./internal/le/interoplab/radius/`, `./internal/le/interoplab/radiussubscriber/`, `./internal/le/interoplab/l2tp/`, `./internal/le/test/deployment/ run 'TestTheInstancePatch\|TestTheDaemonEnvironment'`, `./internal/le/test/qemu/ run TestRuntimeKernelAssertionIsReportData` (all PASS); `pppoe-padr-replay` proved in `8dce48dd9d` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | All 25 ACs and all 33 owned journal rows are implemented, verified, and closed | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/ppp-l2tp-aaa-one-off-defects-1f1e709f-6f5a-43cc-889b-710cf92b134b.md` |
| `./le spec review check` | `clean` (`OK (90 code files, clean, hashes match tmp/review/ppp-l2tp-aaa-one-off-defects-1f1e709f-6f5a-43cc-889b-710cf92b134b.md)`) |
| Rounds | `1` (0 BLOCKER, 0 ISSUE in shipped code; 3 NOTEs logged, plus doc/journal/citer repairs landed in Commit A) |
| Reviewer lenses used | `logic+wiring`, `security+edge-cases`, `RFC+style+docs` (independent closing agent; authored none of the 8 `Spec-work` commits; ran 7-question Ze Go Style pass + `./le go style check path` on all 39 changed production Go files = OK, `./le repo check` = 0 findings in spec files, `./le test weakened check base ad1805f42d~1` = 0 findings in spec files) |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | NOTE (doc) | Stale dial-source ("first server", "`ze config validate` does not check this yet", removed `reloadDialSource`/`dialSourceUnreachable` anchors) and `255.255.255.255` prose | `docs/guide/l2tp.md:148-165`, `1455-1458` | Updated prose and source anchors in Commit A |
| 2 | NOTE (journal) | Owned peer-CDN row still marked `open:` despite being fixed via `removeSession`/`emitSessionDowns` | `plan/journal/guard-added-to-one-half-of-a-pair.md:110` | Flipped to `fixed:` in Commit A (`./le spec journal check` valid) |
| 3 | NOTE (citer) | Sibling spec cited `spec-ppp-l2tp-aaa-one-off-defects.md` with `.md` suffix | `plan/release/fixes/spec-dropped-error-3-access-security.md:46` | Restated as bare stem `spec-ppp-l2tp-aaa-one-off-defects` in Commit A |

*(Non-blocking code NOTE: `copyRejectedIPCPOptions` in `internal/component/l2tp/ppp/ncp.go:1469-1479` may call `admitPeerIPv4` on an earlier valid IP-Address option in a duplicate-option Configure-Request before rejecting a later invalid option; bounded to one slot per session key and released on renegotiation/TLD/resetNCP/session-down.)*

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/l2tp/auth-precedence-radius.ci` | yes | `ls test/l2tp/` lists `auth-precedence-radius.ci` (71 lines, `bb386c2ca3`) |
| `test/l2tp/auth-precedence-local.ci` | yes | `ls test/l2tp/` lists `auth-precedence-local.ci` (56 lines, `bb386c2ca3`) |
| `test/l2tp/session-cdn-teardown.ci` | yes | `ls test/l2tp/` lists `session-cdn-teardown.ci` (28 lines) |
| `test/l2tp/relay-realm-config.ci` | yes | `ls test/l2tp/` lists `relay-realm-config.ci` (96 lines) |
| `website/changes/fragments/ppp-l2tp-aaa-one-off-defects.md` | yes | Written in Commit A with canonical stem `ppp-l2tp-aaa-one-off-defects` |
| `internal/component/l2tp/pppoeclient/register_lcp_diagnostic.go` | yes | Created in `f88f5b6a3d` (38 lines) |
| `internal/component/l2tp/ppp/rfc1661_rtr_ruc_states_test.go` | yes | Created in `fb93ee3c60` (144 lines) |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-4 | PPPoE client LCP, Nak dedup, `LCPDoTransition`, `absorbIPCPReject` | Verified at producers (`pppoeclient/session.go`, `ppp/lcp_options.go`, `ppp/ppp_fsm.go`, `ppp/ncp.go`); `./le test unit pkg path ./internal/component/l2tp/ppp/` & `./internal/component/l2tp/pppoeclient/` PASS |
| AC-5, AC-21 | Dead `crypto/rand.Read` removed; `u` bit (`0x02`) cleared | `internal/component/l2tp/ppp/ipv6cp.go:158-170` (`generateIPv6CPInterfaceID`); `rfc5072_ipv6cp_test.go` PASS; fresh `RFC5072-4.1-10`, `RFC5072-4.1-11` records |
| AC-6 | Priority subscriber auth registry (RADIUS over local) | `internal/component/l2tp/subscriber/handler_registry.go:59-110`; QEMU CI `auth-precedence-radius.ci` (6.3s PASS) & `auth-precedence-local.ci` (5.4s PASS) |
| AC-7 | Outgoing call and relay pick reaching bound listener | `internal/component/l2tp/outgoing_call.go:111` (`reactorForRemoteLocked`), `relay_sink.go:43,120`; `outgoing_call_test.go`, `relay_sink_test.go` PASS |
| AC-8 | RADIUS `Framed-IP-Address == 0xFFFFFFFF` peer-negotiated IPv4 with pool/live-session exclusivity | `authradius/extract.go:45-51`, `pool/radius.go:98-217`, `ppp/ncp.go:800-863,1455-1488`; `authradius`, `pool`, `ppp` unit suites PASS; fresh `RFC2865-5.8-1` record |
| AC-9, AC-10 | `TestReactorReauthIntervalFromParams` passes; peer CDN emits unified session-down | `reactor_ppp_linux_test.go:34`, `session.go:481` (`removeSession`), `peer_teardown_cause_test.go:56`; `./le test unit pkg path ./internal/component/l2tp/` PASS |
| AC-11 | PADT MAC mismatch Debug log rate-limited with reason | `internal/component/l2tp/pppoe/server.go:23-69,515-527` (`padtMismatchLogLimiter`, `handlePADT`); `padt_mac_mismatch_test.go` PASS |
| AC-12 | Late L2TP metrics binding via `InjectPluginMetrics` | `internal/component/l2tp/metrics.go:79-124`, `subsystem.go:505-531` (`onLateMetricsRegistry`); `metrics_test.go` PASS |
| AC-13, AC-25 | RADIUS interop log env keys wired; dead `ze.l2tp.ncp.*` keys removed | `interoplab/radius/radius.go:293-301`; `grep -rn "ze\.l2tp\.ncp\."` finds only negative-guard tests (`l2tpppp_test.go:81`, `radiussubscriber_test.go:271`) |
| AC-14, AC-20 | `EncodeVSA`/`DecodeVSA` inverses; `TestClientRetransmit` checks Identifier+Authenticator; EAP test comment accurate | `radius/attr.go:132-182`, `radius/rfc2865_client_test.go:152-241`, `radius/authenticator_eap_test.go:894`; all `[2:]` slices removed (`grep EncodeVSA`); fresh `RFC2865-2.5-1` record |
| AC-15, AC-17..AC-19 | `validateParameters`, accounting-only RADIUS, SCCRQ version Result Code 5, MS-CHAPv2 `S=` decode, `LoadBackend` fallback removed | Verified at `l2tp/register.go:23`, `authradius/handler.go:102,428`, `l2tp/tunnel_fsm.go:628`, `grep LoadBackend` = 0 hits in `l2tp`/`pppoe` subsystems |
| AC-16 | `pppoe-padr-replay` fixed; `IfaceBackend` replaced; TACACS test hits reachable server | `8dce48dd9d` (`check_padr_replay.go`); `ppp/session_handle.go`; `tacacs/rfc8907_authorization_test.go:245-281` PASS |
| AC-22, AC-24 | PPPoE client LCP Protocol-Reject and 30 s no-NCP idle timer | `pppoeclient/session.go:1380-1444`, `ipcp_independent.go:23,318-374`, `network_phase.go:105,190-191`; `protocol_reject_test.go`, `no_ncp_idle_test.go`, `rfc7084_ncp_independence_test.go` PASS; fresh `RFC7084-4.2-10` record |
| AC-23 | RFC 1661 §5.5/5.6 RTR and RUC across all 8 LCP states | `ppp/rfc1661_rtr_ruc_states_test.go:35,101`; PASS with fresh `RFC1661-5.5-1`/`5.6-1` records |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| L2TP CHAP with RADIUS + local configured -> `AuthPriorityRemote` (`authradius`) | `test/l2tp/auth-precedence-radius.ci` | Read file: asserts `OK: RADIUS server received Access-Request for subscriber@ze`, `IPCP opened, ze assigned 10.99.21.10`, rejects `l2tp-auth-local: CHAP-MD5`; QEMU CI PASS (6.3s) |
| L2TP CHAP with local only (RADIUS unconfigured) -> `AuthPriorityLocal` (`authlocal`) | `test/l2tp/auth-precedence-local.ci` | Read file: asserts `OK: local auth admitted CHAP session for subscriber@ze`, `l2tp-auth-local: CHAP-MD5 accepted`, rejects `no RADIUS client`; QEMU CI PASS (5.4s) |
| Peer CDN on established session -> `handleCDN` / `removeSession` | `test/l2tp/session-cdn-teardown.ci` | Read file: sends CDN via `le test fixture l2tp/session-cdn-teardown`, asserts `OK: CDN sent` and `session destroyed`; `functional l2tp` 30/30 PASS |
| `ze config validate` -> `validateParameters` -> `ExtractParameters` | `test/l2tp/relay-realm-config.ci` | Read file: validates good config (`exit=0`) and refuses undeclared remote `lns-missing` (`exit=1`); `functional l2tp` 30/30 PASS |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `internal/component/l2tp/ppp/ppp_fsm.go` `LCPDoTransition` is the sole transition table; `TestRFC1661TerminateAckAcrossAllStates` and `TestRFC1661CodeRejectAcrossAllStates` drive `handleLCPPacket` through it across all 8 states (`fb93ee3c60`). |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Architecture docs (`bng-1-radius-attributes.md`, `cpe-1-pppoe-client.md`, `followup-l2tp-call.md`, `l2tp-10-metrics.md`, `subscriber-session-model.md`, `config/environment.md`) | Updated in `3507e52419`, `f88f5b6a3d`, `bb386c2ca3`, `a86dca89db`; `./le doc check all` reports 0 drift and 0 broken anchors in these files | yes |
| User guide (`docs/guide/l2tp.md`) | Stale AC-7/AC-8/AC-15 claims and `reloadDialSource` anchor repaired in Commit A with anchors to `validateDialSources`, `validateParameters`, `reloadDialSources`, `reactorForRemoteLocked`, `listenerCanReach`, `wildcardFor`, `extractAuthMetadata`, `peerNegotiatedAnswer`, `admitPeerAddress`, `releasePeerAddress`, `evalIPCPRequest`, `isValidNegotiatedPeerIPv4` | yes |
| Doors & digests (`ai/doors/access-security.md`, `ai/digests/subscriber.md`) | Updated in `bb386c2ca3`; 3180/3180 digest anchors resolve | yes |
| RFC summaries (`rfc/short/rfc1994.md`, `rfc/short/rfc2865.md`, `rfc/short/rfc5072.md`) | Updated in `14d3df65a7`, `3507e52419`, `fb93ee3c60`; `./le rfc check` handed to main thread as owed command | yes |
| News fragment (`website/changes/fragments/ppp-l2tp-aaa-one-off-defects.md`) | Written in Commit A with canonical stem, valid topic `L2TP`, `found-by: review`, and all 6 headings in order | yes |
| Doctor codes (`pppoeclient/register_lcp_diagnostic.go`) | `doctor-pppoe-client-lcp-write` registered via `diagnostic.Register` and `diagnostic.RegisterEventCode` in `init()` (`f88f5b6a3d`) | yes |
