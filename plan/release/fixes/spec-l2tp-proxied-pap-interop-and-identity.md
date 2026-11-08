# Spec: l2tp-proxied-pap-interop-and-identity

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Priority | P1 |
| Release reason | A LAC's unverified name becomes the subscriber's authenticated identity, an IPv4-only decline disables IPv6CP for good, and proxied PAP has no proof against a real PPP client |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-06 |

Scope is protocol: the wire change is which name a RADIUS server receives as User-Name, and
IPv6CP returning on a later LCP lifetime. No config or CLI command changes.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Three defects in how an L2TP subscriber session treats what its LAC said.

**D-1: proxied PAP has no proof against a real client.** Since `926f01f732`, a
proxied LCP naming PAP waits `proxyPAPRequestWait`, then renegotiates LCP
(`renegotiateProxiedLCP`), because a peer that finished PAP with the LAC never sends
another request. `TestProxiedPAPWithoutRequestRenegotiatesLCP` proves it against a
scripted peer. The lab's one proxy scenario, `l2tp-proxy-mru-transmit-cap`, runs CHAP
from Ze's own fixture. Whether a real PPP client survives the renegotiation is
unproven.

**D-2: a handler decline disables IPv6CP for the session's life.** `runNCPPhase`
sets `s.disableIPv6CP = true` when the handler declines IPv6. That field is also the
operator's `DisableIPv6CP`, and `resetNCP` does not restore it. After LCP
renegotiates, the session keeps Protocol-Rejecting IPv6CP, and the handler is never
asked again, although the next lifetime may authenticate as another identity with
another pool. It is not a one-line fix: the field carries two meanings, so it takes
one new field, its reset in `resetNCP`, and two readers.

**D-3: the LAC's name becomes the subscriber's identity.** `session_fsm.go` copies
the unverified Proxy Authen Name into `sess.username`. For a no-auth session, or an
authenticated empty name, `handleSessionIPAssigned` keeps it, and it reaches RADIUS
accounting as User-Name (`buildAcctPacket`), the CoA session match (`coa.go`), and
the CLI as `Username`. `l2tp/session.go` calls the field "the PPP-authenticated peer
identity", which is false. Ze never authenticates from the Proxy Authen AVPs.

**Owner decision: RADIUS accounting carries the LAC's name.** The fix keeps the claim out
of the identity, and does not drop it from accounting. When nothing authenticated a name,
User-Name carries the LAC's claimed name, so billing keeps the name the LAC gave. An
authenticated name always wins over the claim, and a session with neither omits
User-Name. A CoA by User-Name matches the same name accounting sent, so the operator's
RADIUS can act on the session it was told about.

Goal: `username` means what its comment says, a declined IPv6 lasts one LCP lifetime,
and proxied PAP is proven against a real client.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/l2tp.md` - proxy LCP and the Proxy Authen AVPs
  → Constraint: the page states today's D-3 as behavior: "keeps the LAC's unverified Name, and RADIUS accounting sends it as User-Name". It changes with this spec.
- [ ] `docs/architecture/l2tp/subscriber-session-model.md` - identity through a session's life
  → Decision: "Explicit no-auth admission has an empty username." The transport sentence after it ("L2TP retains a proxy identity...") changes with this spec.
- [ ] `docs/labs/l2tp-interop.md`, `docs/architecture/testing/interop.md` - scenario layout, the relay, vacuity traps
  → Constraint: the scenario is named with no numeric prefix, and proves red with the behavior reverted and the image rebuilt.
- [ ] `docs/guide/radius.md` - accounting attributes
  → Constraint: subscriber billing reads User-Name.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2661.md` - Proxy Authen AVPs
  → Constraint: RFC 2661 Section 4.4.5: "The Proxy Authen Name AVP, Attribute Type 30, specifies the name of the authenticating client when using proxy authentication." Ze does not use proxy authentication, so the name is a claim.
  → Constraint: RFC 2661 Section 4.4.5: "Note that the LNS policy may be to enter an additional round of LCP negotiation and/or authentication if the LAC is not trusted."
- [ ] `rfc/short/rfc2865.md`, `rfc/short/rfc2866.md` - User-Name
  → Constraint: RFC 2865 Section 5.1: "This Attribute indicates the name of the user to be authenticated." RFC 2866's attribute table lists User-Name as 0-1 in an Accounting-Request, so omitting it is valid, and Section 5 already omits zero-length text. Sending the LAC's claim there is the owner's decision; the guide says the name is the LAC's, not authenticated.
- [ ] `rfc/short/rfc1661.md`, `rfc/short/rfc1334.md` - renegotiation and PAP
  → Constraint: RFC 1661 Section 4.3: "The Down followed by an Up will cause an orderly renegotiation of the link, by progressing through the Starting to the Request-Sent state."
- [ ] `rfc/short/rfc5072.md` - IPv6CP
  → Constraint: RFC 5072 Section 3: IPv6CP is exactly the same as LCP with the listed exceptions, so it runs anew on each LCP lifetime.

**Key insights:**
- `xl2tpd` never sends a proxy AVP. Its ICCN builder (`control.c`, the ICRP branch) carries the comment "We don't need any kind of proxy PPP stuff". No real LAC in the lab can carry this case.
- `pppd` does renegotiate. `fsm_rconfreq` (`pppd/fsm.c`), state OPENED: "Go down and restart negotiation", then `fsm_sconfreq` and REQSENT.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/ppp/ncp.go` - `runNCPPhase` sets `s.disableIPv6CP = true` on `declined`. `resetNCP` clears the lifetime's state and leaves that field
- [ ] `internal/component/l2tp/ppp/session.go` - the `disableIPv6CP` field
- [ ] `internal/component/l2tp/ppp/session_run.go` - `handleFrame` Protocol-Rejects IPv6CP on `disableIPv6CP`; `supportsProtocol` reads it. A comment there names MRU as a precedent: "for this LCP lifetime"
- [ ] `internal/component/l2tp/ppp/auth.go` - `acceptProxyPolicy` admits PAP when it is in the allow list
- [ ] `internal/component/l2tp/ppp/pap.go` - `proxyPAPRequestWait`, `runPAPAuthPhase`, `renegotiateProxiedLCP`
- [ ] `internal/component/l2tp/ppp/proxy_pap_renegotiate_test.go` - the scripted proof of the renegotiation
- [ ] `internal/component/l2tp/session_fsm.go` - copies `info.proxyAuthenName` to `sess.proxyAuthenName` and to `sess.username`
- [ ] `internal/component/l2tp/session.go` - the false comment on `username`; `proxyAuthenName` already exists as its own field
- [ ] `internal/component/l2tp/reactor_kernel.go` - `handleSessionIPAssigned` overwrites `username` only with a non-empty `ev.Username`. The LCP-down path clears it only on `ev.NetworkPhase`
- [ ] `internal/component/l2tp/plugins/authradius/acct.go` - `buildAcctPacket` sends `sess.username`, omitting it when empty. Its comment names the proxy name as a source
- [ ] `internal/component/l2tp/plugins/authradius/coa.go` - a CoA matches on `sess.Username`
- [ ] `internal/component/l2tp/snapshot.go` - `SessionSnapshot.Username`, the CLI's source
- [ ] `internal/le/interoplab/l2tp/l2tp.go` - scenario table, `scenarioProxyMRU`
- [ ] `internal/le/interoplab/l2tp/proxy_mru.go` - runs a compiled fixture inside the Ze container; CHAP only
- [ ] `internal/le/interoplab/l2tp/avprelay/main.go`, `internal/le/interoplab/l2tp/avprelay/pppinject.go` - the UDP relay between `xl2tpd` and Ze: appends an AVP to a control message, and originates PPP data frames on a marker

**Behavior to preserve:**
- A proxied CHAP or MS-CHAPv2 session gets a fresh Challenge. A proxied PAP session waits, then renegotiates.
- Ze never authenticates from the Proxy Authen AVPs. `plan/post-release/features/spec-l2tp-proxy-authentication.md` owns that feature.
- An operator's `enable-ipv6cp false` stays for the session's whole life.
- The Protocol-Reject that stops a peer retransmitting IPv6CP in the lifetime the handler declined.
- User-Name omitted when there is neither an identity nor a LAC claim (RFC 2866 Section 5).
- The LAC's name in User-Name for a session nothing authenticated, as today.
- The relay originates no control datagram: `xl2tpd` and Ze own sequencing.

**Behavior to change:**
- `sess.username` holds only an accepted identity. The Proxy Authen Name stays in `proxyAuthenName`.
- A declined IPv6CP ends with its LCP lifetime, and the next lifetime asks the handler again.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| LCP lifetime reset | `ncp.go` `resetNCP` | Clears each lifetime's NCP state | Extends: the decline joins that list |
| Per-lifetime IPv6 refusal | `session_run.go` `supportsProtocol`, `ipv6MRUSupported` | A low MRU refuses IPv6 "for this LCP lifetime" | Extends: the decline is read the same way, beside it |
| Accepted identity | `reactor_kernel.go` `handleSessionIPAssigned`, `ppp` `authenticatedUsername` | PPP reports the accepted name, cleared on LCP down | Extends: it becomes the only writer of `username` |
| Proxy AVP capture | `session.go` `proxyAuthenName`, `session_fsm.go` | Stores the LAC's name in its own field | Extends: the CLI reads it from there, labelled as the LAC's claim |
| Attribute omission | `radius.AppendTextAttr`, `buildAcctPacket` | Omits zero-length text | Extends unchanged: an empty identity with no claim already gives the valid packet |
| AVP relay | `internal/le/interoplab/l2tp/avprelay` | Transparent relay with injection on a marker | Extends: a proxied mode holds the ICCN, answers `pppd`'s LCP and PAP as the LAC would, then forwards the ICCN with the proxy AVPs. It still originates no control datagram: `xl2tpd`'s retransmit carries the ICCN |
| Compiled proxy fixture | `internal/test/fixture/tunnel_fixture_l2tp_proxy_mru.go` | Plays a whole LAC with a scripted PPP peer | Does not apply to the client proof: the point is a real `pppd`. It carries the `.ci` below |
| Proxy authentication feature | `plan/post-release/features/spec-l2tp-proxy-authentication.md` | Skeleton for consuming the AVPs | Does not apply: post-release, and this spec consumes nothing |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- ICCN from a LAC, with AVPs 26 to 33: `parseICCN`.
- A handler's decline in `requestIPv6CPInterfaceID`.
- A RADIUS accounting event for a subscriber session.

### Transformation Path
1. `session_fsm.go` stores the Proxy Authen Name in `proxyAuthenName` only.
2. PPP runs. An accepted, non-empty name reaches `sess.username` through `handleSessionIPAssigned`.
3. `l2tpevents` carries `username` and, separately, the LAC's claim. `buildAcctPacket` sends the identity, else the claim, else omits User-Name. A CoA by User-Name matches the name accounting sent.
4. `SessionSnapshot` exposes the accepted identity and, separately, the LAC's claim.
5. A decline sets the per-lifetime field. `resetNCP` clears it. The next `runNCPPhase` asks the handler.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| L2TP to PPP | `StartSession`, proxy LCP AVPs only | Yes: `start_session.go` |
| PPP to L2TP | `EventSessionIPAssigned.Username` | Yes: `reactor_kernel.go` |
| L2TP to `authradius` | `l2tpevents` payload `Username` | Yes: `acct.go` reads `payload.Username` |
| Ze to RADIUS | Accounting-Request | Yes: `test/l2tp/radius-acct-wire.ci` reads what the server decoded |

### Changed-Wire Census

**Search evidence:** scope `internal`, `test`, `docs`. Terms: `sess.username`, `proxyAuthenName`, `Username:`, `User-Name`, `AttrUserName`. Control: `buildAcctPacket` is found as the writer of the attribute, and `coa.go` as a reader of the snapshot field.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| RADIUS User-Name in accounting | writer | `acct.go` `buildAcctPacket` | sends `sess.username` | Change: the accepted identity, else the LAC's claim, else nothing |
| RADIUS User-Name in Access-Request | writer | `handler.go` | sends `req.Username`, what the peer presented | Preserve: a credential, not this field |
| CoA User-Name | reader | `coa.go` session match | reads `sess.Username` | Change: matches the name accounting sent, identity or claim |
| CLI session output | reader | `snapshot.go`, `l2tp/cli` | prints `Username` | Change: accepted identity, plus a separate field for the claim |
| Subscriber route log | reader | `route_observer.go` | logs `username` | Change: accepted identity |
| IPv6CP to a peer | writer | `session_run.go` `handleFrame` | Protocol-Reject | Change: per lifetime |
| RADIUS User-Name in accounting | reader | the operator's RADIUS accounting server, outside the repository (FreeRADIUS in `interop-radius-subscriber`) | joins Start, Interim and Stop by the User-Name that `acct.go` `acctSession.username` holds | Change: it receives the accepted identity, else the LAC's claim, else no attribute |
| RADIUS User-Name in Access-Request | reader | the operator's RADIUS server, outside the repository | authenticates the name `handler.go` writes with `radius.AttrUserName` from `req.Username` | Preserve |
| CoA User-Name | writer | the operator's RFC 5176 dynamic-authorization client, outside the repository | sends the User-Name that `coa.go` compares with `sess.Username` | Change: it sends the name accounting sent, which `coa.go` then matches |
| CLI session output | writer | `cmd/l2tp.go` `sessionJSON` | writes `"username"` from `snapshot.go` `Username` | Change: accepted identity, plus a separate field for the claim |
| Subscriber route log | writer | `route_observer.go`, the log calls that write the `username` key | writes `r.username` | Change: accepted identity |
| IPv6CP to a peer | reader | the PPP peer, outside the repository (`pppd` in the interop lab) | receives the Protocol-Reject `handleFrame` sends | Change: per lifetime |

### Integration Points
- `resetNCP` - the reset.
- `handleSessionIPAssigned` - already the writer; the seed goes away.
- `scenarioCheckerMap` in `internal/le/interoplab/l2tp/l2tp.go` - registers the scenario.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Identity reaches accounting by the existing event |
| No unintended coupling (components stay isolated) | Yes | `ppp` stays a leaf |
| No duplicated functionality (extends existing, does not recreate) | Yes | `proxyAuthenName` already exists; the copy into `username` is what goes |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Session setup |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | The scenario adds a row in the lab's own table |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | `test/interop-l2tp/parity_test.go` compares the scenario directory with `scenarioCheckerMap`, which stays the one list |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `xl2tpd` retransmits an unacknowledged ICCN long enough for the relay's PPP exchange | `control.c` sends ICCN with `control_xmit`, then starts `pppd` | The hold times out before PAP ends | The scenario's first run. If it fails, the relay answers PPP from the ICRP on, and the hold shortens | unvalidated |
| A-2 | `pppd` 2.5.5, in Network phase, renegotiates and repeats PAP on a Configure-Request | `fsm_rconfreq` OPENED on upstream master; the lab builds 2.5.5 | The scenario can never pass, and reads as a Ze defect | Read `pppd/fsm.c` and `upap.c` at 2.5.5 before writing the check | unvalidated |
| A-3 | Every RADIUS and accounting consumer in the tree that reads the LAC's name in User-Name keeps reading it | The census above | A scenario loses the name it asserts | `04-radius-acct-attrs`, `l2tp-radius-acct-wire-fixture`, `test/l2tp/radius-acct-wire.ci`: this `.ci` is CHAP through proxy, so its name still arrives accepted | unvalidated |
| A-4 | A second lifetime should ask the handler again | RFC 5072 Section 3, and identity may change between lifetimes | A handler is asked more often than it expects | `TestIPv6CPDeclineEndsWithLCPLifetime` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An operator reads the LAC's name in User-Name as authenticated | A billing dispute over a no-auth session | The guide says User-Name is the LAC's claim when nothing authenticated; the CLI labels identity and claim apart |
| R-2 | The relay grows into a LAC | Its size and its comment | It keeps no L2TP state: PPP only, and the hold is a dropped datagram |
| R-3 | The scenario depends on 6 s of timing | Flakes under load | It asserts the outcome, with the lab's own timeout, never the delay |
| R-4 | `spec-home-cpe-15-lns-lcp-renegotiation` (closed 2026-11-08) listed this seeding as behavior to preserve | A review of that spec | Its line was repointed here when this spec was written |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | An accounting record lacks a name it used to carry, or a session re-offers IPv6CP. Live forwarding is not touched |
| How is it reverted? | One commit per defect; no persistent state |
| Who else touches this path? | `spec-ppp-ncp-independent-lifetime` (ready) edits `ncp.go` and `session_run.go`; `spec-home-cpe-15-lns-lcp-renegotiation`; `spec-ppp-l2tp-aaa-one-off-defects` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| ICCN with Proxy Authen Name, no-auth admission | → | `session_fsm.go`, `handleSessionIPAssigned`, `buildAcctPacket` | `test/l2tp/proxy-name-accounted.ci` |
| A handler declines IPv6, then LCP restarts | → | `runNCPPhase`, `resetNCP` | `TestIPv6CPDeclineEndsWithLCPLifetime` |
| `xl2tpd` and `pppd` through the proxied relay | → | `runPAPAuthPhase`, `renegotiateProxiedLCP` | `l2tp-proxy-pap-pppd` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | ICCN names `lac-claim`, the session is admitted with no authentication | User-Name is `lac-claim`, in Start and Stop, with `Acct-Session-Id` and `Calling-Station-Id` as usual, while the session's identity stays empty | None: the owner wants the LAC's name accounted |
| AC-2 | ICCN names `lac-claim`, the peer authenticates with an empty name | User-Name is `lac-claim` | None |
| AC-3 | ICCN names `lac-claim`, the peer authenticates as `alice` | User-Name is `alice`, in Start and Stop | None |
| AC-4 | A CoA by User-Name `lac-claim` against AC-1's session | It matches AC-1's session, the name accounting sent | A CoA by session id still matches |
| AC-5 | `show l2tp` for AC-1's session | The identity is empty, and the LAC's claim shows in its own labelled field | The claim stays visible: an operator diagnosing a LAC needs it |
| AC-6 | A handler declines IPv6 in LCP lifetime 1 | The peer's IPv6CP gets Protocol-Reject in lifetime 1 | None |
| AC-7 | LCP renegotiates to lifetime 2 | The handler is asked again. If it grants, IPv6CP opens | A second decline behaves as AC-6 |
| AC-8 | `enable-ipv6cp false` | IPv6CP stays refused in every lifetime, and the handler is never asked | None: the operator chose |
| AC-9 | Real `pppd` finishes PAP with the LAC role, then ICCN carries proxy LCP naming PAP | Ze renegotiates LCP, `pppd` authenticates again, IPCP opens, and an ICMP round trip crosses the session | None |
| AC-10 | The same with `pppd` still sending its PAP request | Ze answers that request and no renegotiation happens | None |
| AC-11 | No Proxy Authen Name and no authenticated name | Ze sends the Accounting-Request Start and Stop with `Acct-Session-Id` and `Calling-Station-Id` as usual, and with the User-Name attribute left out | None: there is no name to send, and every other attribute of the record stays |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | A subscriber connects through a LAC that finished PAP | LAC ICCN -> proxy LCP -> wait -> renegotiate -> PAP -> IPCP -> traffic | `l2tp-proxy-pap-pppd` |
| 2 | An operator reads accounting for a no-auth session | ICCN -> session up -> Accounting-Request with the LAC's name as User-Name | `test/l2tp/proxy-name-accounted.ci` |
| 3 | A subscriber's LCP restarts after an IPv4-only pool answered | reset -> asked again -> IPv6CP opens | `test/l2tp/ipv6cp-after-decline.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestIPv6CPDeclineEndsWithLCPLifetime` | `internal/component/l2tp/ppp/ncp_lifetime_test.go` | AC-6, AC-7 | |
| `TestOperatorIPv6CPDisableSurvivesLCPRestart` | same | AC-8 | |
| `TestProxyAuthenNameDoesNotSeedUsername` | `internal/component/l2tp/session_fsm_test.go` | AC-1, AC-2: `username` stays empty after ICCN | |
| `TestAcceptedNameReachesUsername` | `internal/component/l2tp/reactor_kernel_test.go` | AC-3 | |
| `TestAcctUserNameIdentityElseClaim` | `internal/component/l2tp/plugins/authradius/acct_test.go` | AC-1, AC-2, AC-3, AC-11: identity wins, else the claim, else no attribute | |
| `TestCoAUserNameMatchesAccountedClaim` | `internal/component/l2tp/plugins/authradius/coa_test.go` | AC-4 | |
| `TestSnapshotSeparatesClaimFromIdentity` | `internal/component/l2tp/snapshot_test.go` | AC-5 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| None new | N/A | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `proxy-name-accounted` | `test/l2tp/proxy-name-accounted.ci` | AC-1, reading what the RADIUS server decoded | |
| `ipv6cp-after-decline` | `test/l2tp/ipv6cp-after-decline.ci` | AC-7 through the real daemon | |
| `radius-acct-wire` | `test/l2tp/radius-acct-wire.ci` | Stays green: its proxied session authenticates | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `l2tp-proxy-pap-pppd` | `test/interop-l2tp/scenarios/` | `xl2tpd` 1.3.20 and `pppd` 2.5.5, through `avprelay` | AC-9, AC-10: a real client survives Ze's renegotiation. The L2TP proxy AVPs come from the relay, because `xl2tpd` sends none, so this proves the client side only | |

## Files to Modify
- `internal/component/l2tp/ppp/ncp.go` - decline sets the per-lifetime field; `resetNCP` clears it
- `internal/component/l2tp/ppp/session.go` - the field
- `internal/component/l2tp/ppp/session_run.go` - `handleFrame`, `supportsProtocol`, the comment
- `internal/component/l2tp/session_fsm.go` - the seed goes
- `internal/component/l2tp/session.go` - the comment says what is true
- `internal/component/l2tp/reactor_kernel.go` - clear on every LCP down path that held an identity
- `internal/component/l2tp/snapshot.go` - the claim as its own field
- `internal/component/l2tp/plugins/authradius/acct.go` - the comment
- `internal/component/l2tp/plugins/authradius/coa.go` - matches the name accounting sent
- `internal/le/interoplab/l2tp/l2tp.go` - scenario row
- `internal/le/interoplab/l2tp/avprelay/main.go` - proxied mode, and its header comment
- `docs/guide/l2tp.md` - the Proxy Authen paragraph
- `docs/architecture/l2tp/subscriber-session-model.md` - the transport identity sentence
- `docs/guide/radius.md` - User-Name is the authenticated name, else the LAC's claim, else absent
- `docs/labs/l2tp-interop.md` - the scenario and the relay mode

## Files to Create
- `internal/le/interoplab/l2tp/proxy_pap.go` - the scenario's checker
- `internal/le/interoplab/l2tp/avprelay/pppproxy.go` - the relay's LCP and PAP role
- `test/interop-l2tp/scenarios/l2tp-proxy-pap-pppd/ze.conf` - with `xl2tpd.conf`, `ppp-options` and `l2tp-secrets`
- `test/l2tp/proxy-name-accounted.ci` - AC-1
- `test/l2tp/ipv6cp-after-decline.ci` - AC-7

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | The session's `show` output gains a field, if the command declares it in YANG: `internal/component/l2tp/` |
| YANG validation constraints | No | No config leaf |
| YANG custom validators | No | No config leaf |
| CLI commands/flags | No | No new command |
| CLI grammar (keyword before value) | No | None |
| Editor autocomplete | No | None |
| Functional test for new RPC/API | Yes | The `.ci` files above |
| Pipe completeness | No | The output path is unchanged |
| Env var registration | No | None |
| Doctor check for runtime dependencies | No | None |
| Prometheus counters/metrics | No | Per-session labels read accepted identity, see below |
| BGP family surface (new SAFI / capability / attribute) | No | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` if it shows the session fields |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/l2tp.md`, `docs/guide/radius.md` |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc2661.md` |
| 10 | Test infrastructure changed? | Yes | `docs/labs/l2tp-interop.md` |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/l2tp/subscriber-session-model.md` |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md`: a username label on a no-auth session becomes empty |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/research/l2tpv2-implementation-guide.md`, `docs/research/l2tpv2-ze-integration.md`: research records, read for any statement that the Proxy Authen Name becomes the username. Others: `./le spec citation anchors spec plan/release/fixes/spec-l2tp-proxied-pap-interop-and-identity.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/l2tp.md` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the scenario row and the failing tests
   - Tests: `TestProxyAuthenNameDoesNotSeedUsername`, `TestIPv6CPDeclineEndsWithLCPLifetime`, `l2tp-proxy-pap-pppd` selected and refused for the missing checker
   - Files: `internal/le/interoplab/l2tp/l2tp.go`
   - Verify: tests red for the reasons in the Task
2. **Phase: IPv6CP lifetime** -- D-2
   - Tests: `TestIPv6CPDeclineEndsWithLCPLifetime`, `TestOperatorIPv6CPDisableSurvivesLCPRestart`, `test/l2tp/ipv6cp-after-decline.ci`
   - Files: `ncp.go`, `ppp/session.go`, `session_run.go`
   - Verify: tests fail, implement, tests pass
3. **Phase: Identity** -- D-3
   - Tests: `TestProxyAuthenNameDoesNotSeedUsername`, `TestAcceptedNameReachesUsername`, `TestAcctUserNameIdentityElseClaim`, `TestCoAUserNameMatchesAccountedClaim`, `TestSnapshotSeparatesClaimFromIdentity`, `test/l2tp/proxy-name-accounted.ci`, `test/l2tp/radius-acct-wire.ci`
   - Files: `session_fsm.go`, `session.go`, `reactor_kernel.go`, `snapshot.go`, `acct.go`, `coa.go`
   - Verify: tests fail, implement, tests pass; every scenario that names a proxy session is re-read
4. **Phase: Client proof** -- D-1
   - Tests: `l2tp-proxy-pap-pppd`
   - Files: `avprelay/pppproxy.go`, `avprelay/main.go`, `proxy_pap.go`, the scenario directory
   - Verify: revert `renegotiateProxiedLCP`, rebuild the image, see red, restore, see green, and record it
5. **Phase: Pages** -- the docs above

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | The operator's disable and the handler's decline are two facts in two fields |
| Correctness | `username` has one writer, the accepted identity from PPP |
| Naming | The CLI field says it is the LAC's claim, never "user" alone |
| Data flow | The Proxy Authen Name reaches no RADIUS attribute |
| Rule: stale comments | `session.go`, `acct.go` and the mirror comment in `session_fsm.go` say what is now true |
| Rule: repository users | Every config, `.ci` and scenario with a proxy session is re-validated (`ai/META-DECISIONS.md`) |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Scoped unit tests | `./le job run label l2tp-unit command go test -tags 'ze_l2tp ze_radius' ./internal/component/l2tp/...` |
| Functional | `./le test functional l2tp` |
| Peer proof | `./le test deployment docker-l2tp-ppp` with `ZE_L2TP_INTEROP_SCENARIO=l2tp-proxy-pap-pppd` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Trust | A name from the LAC is never recorded as the authenticated identity; it reaches User-Name only where nothing authenticated |
| Input validation | The Proxy Authen Name stays bounded by the AVP parse, and is shown as data |
| Fail-closed | Neither identity nor claim omits the attribute, and invents nothing |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The seeding is documented as intended behavior in three places: `docs/guide/l2tp.md`, `docs/architecture/l2tp/subscriber-session-model.md`, and the behavior-to-preserve list of `spec-home-cpe-15-lns-lcp-renegotiation`. This spec overrides them, on the commissioning brief's finding and on `ai/rules/principles.md` (a silently wrong value). Thomas decided the LAC's name stays in accounting, so the override separates identity from claim and leaves accounting carrying the claim where nothing authenticated.
- Handover item 20 (`plan/handover/2026-11-06-unpushed-review-findings.md`) reads as a stall, and `926f01f732` already fixed it. What remains is the proof.
- No real LAC in the lab can carry this case, so the scenario proves the client half only. Reading freeRtr's and accel-ppp's LAC for proxy AVPs is the way to a whole-LAC proof, and neither is in this lab today.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `fixture-encodes-an-impossible-state.md` | 2026-11-06 | proxied PAP proven only against a scripted peer, no real client |
| `field-carries-two-meanings.md` | 2026-11-06 | `disableIPv6CP` holds the operator's choice and a per-lifetime decline |
| `mirrored-field-asserts-the-wrong-sender.md` | 2026-11-06 | LAC's unverified Proxy Authen Name seeds `sess.username`, reaches User-Name |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| User-Name is the accepted identity, else the LAC's claim (owner decision) | Omit User-Name for no accepted identity; keep seeding the identity from the claim | Thomas wants the LAC's name accounted. Keeping it out of `username` fixes the false comment and the CLI, while billing keeps the name |
| Relay plays the LAC's PPP role | Fixture with `pppd` on a pty; wait for a real LAC | Keeps `xl2tpd`'s L2TP and `pppd`'s PPP real. A pty fixture replaces `xl2tpd` with Ze's own code on the wire |
| New per-lifetime field | Restore `disableIPv6CP` in `resetNCP` from the start value | Two facts stay two fields, and `supportsProtocol` can say which applies |

## Known Limitations
- The L2TP half of proxy AVPs comes from the relay, not a third-party LAC. See Design Insights.
- `plan/post-release/features/spec-l2tp-proxy-authentication.md` still owns authenticating from those AVPs.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code. `session_fsm.go` carries RFC 2661 Section 4.4.5 for why the name is only a claim; `resetNCP` carries RFC 5072 Section 3 at the decline's reset.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests pass, run in the main checkout: `./le test functional l2tp`, `./le test deployment docker-l2tp-ppp` for the scenario, and the scoped package tests. GitHub CI verifies the pushed commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script
