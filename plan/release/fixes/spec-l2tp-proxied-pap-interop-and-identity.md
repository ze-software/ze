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
| Updated | 2026-11-26 |

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
| CoA/DM session matching | `internal/component/l2tp/plugins/authradius/coa.go` (`findSessions`, `sessionSelectorsMatch`) | Compares `AttrUserName` against `sess.Username` | Extends: compares against the effective accounted name (authenticated `Username`, else LAC `ProxyAuthenName` claim), coordinated with `spec-rfc5176-remaining-defect-classes.md` |
| AVP relay | `internal/le/interoplab/l2tp/avprelay` | Transparent relay with injection on a marker | Extends: a proxied mode holds the ICCN, answers `pppd`'s LCP and PAP as the LAC would, then forwards the ICCN with the proxy AVPs. It still originates no control datagram: `xl2tpd`'s retransmit carries the ICCN |
| Compiled proxy fixture | `internal/test/fixture/tunnel_fixture_l2tp_proxy_mru.go` | Plays a whole LAC with a scripted PPP peer | Does not apply to the client proof: the point is a real `pppd`. It carries the `.ci` below |
| Proxy authentication feature | `plan/post-release/features/spec-l2tp-proxy-authentication.md` | Skeleton for consuming the AVPs | Does not apply: post-release, and this spec consumes nothing |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | Ze is a platform, and other people's code runs against it | the door every change reads | "The RFC, then ExaBGP API compatibility, then ExaBGP's behavior, in that order" | agrees | Enforces RFC 2661 Section 4.4.5 (Proxy Authen Name is an unverified claim, not an authenticated identity), RFC 5072 Section 3 (IPv6CP runs anew per LCP lifetime), RFC 1661 Section 4.3 (orderly LCP renegotiation proven against real `pppd`), and RFC 2866 Section 5 (omits empty `User-Name`) while keeping L2TP-to-`authradius` communication on the EventBus |
| `ai/doors/access-security.md` | Who depends on it from outside | the door covering `ncp.go`, `ppp/session.go`, `session_run.go`, `pap.go`, `session_fsm.go`, `session.go`, `reactor_kernel.go`, `snapshot.go`, `events/events.go`, `cmd/l2tp.go`, `acct.go`, and `coa.go` | "Subscriber auth and accounting through `l2tp/plugins/authradius`" | agrees | Keeps `ppp` a dependency-free leaf, preserves subscriber accounting and CoA/DM ownership in `l2tp/plugins/authradius`, and preserves billing continuity and CoA matchability for LAC-claimed sessions per Thomas's owner decision |
| `ai/doors/le-tooling.md` | Boundaries | the door covering `internal/le/interoplab/l2tp/l2tp.go`, `register_scenarios.go`, `proxy_pap.go`, `avprelay/main.go`, `avprelay/pppproxy.go`, and `test/l2tp/*.ci` | "A compiled Go fixture returns an error when an assertion fails" | agrees | Adds functional `.ci` proofs under `test/l2tp/` and the `l2tp-proxy-pap-pppd` interop scenario with typed Go assertions in `proxy_pap.go` and a bounded relay mode in `avprelay/pppproxy.go`, keeping all test tooling outside product packages |
| `docs/research/l2tpv2-ze-integration.md` | 6.1 l2tp-auth (authentication and authorization) | the `// Design:` page of `ncp.go`, `ppp/session.go`, `session_run.go`, `pap.go`, `reactor_kernel.go`, `acct.go`, and `coa.go` | "Handle RADIUS CoA (Change of Authorization) for live sessions." | agrees | Keeps RADIUS accounting `User-Name` fallback and CoA `User-Name` matching inside `l2tp-auth-radius`, and keeps per-lifetime IPv6CP decline tracking inside the PPP session driver without leaking policy into L2TP control transport |
| `docs/research/l2tpv2-implementation-guide.md` | 18.4 LNS Behavior | the `// Design:` page of `session_fsm.go` and `session.go` | "Accept proxy LCP but reject proxy auth (re-authenticate the user)." | agrees | Captures Proxy Authen AVPs on ICCN in `proxyAuthenName` without authenticating from them or seeding `sess.username`, and proves the proxied-PAP LCP renegotiation path against real `pppd` |
| `docs/architecture/l2tp/bng-1-radius-attributes.md` | What each teardown path reports | anchored to `ncp.go`, `session_run.go`, `session_fsm.go`, `session.go`, `events/events.go`, and `acct.go` | "An absent value sends NO attribute." | agrees | Preserves `AppendTextAttr` omission of `User-Name` when neither an accepted PPP identity nor a LAC claim is present (AC-11), and retains the resolved accounting `User-Name` on `acctSession` for Start, Interim-Update, and Stop records |
| `docs/architecture/l2tp/subscriber-session-model.md` | Traps this code exists to avoid | anchored to `ncp.go`, `session_run.go`, `session_fsm.go`, `session.go`, `reactor_kernel.go`, `events/events.go`, `cmd/l2tp.go`, and `acct.go`; modified by this spec | "L2TP retains a proxy identity supplied for the current transport until PPP supplies an accepted username." | departs | Removes seeding of `sess.username` from `proxyAuthenName` so `sess.username` holds only an accepted PPP identity (matching the preceding sentence "Explicit no-auth admission has an empty username"); the LAC's unverified claim stays separate in `proxyAuthenName` and reaches RADIUS accounting and CoA only as the fallback when nothing authenticated. Phase 5 updates this sentence in `subscriber-session-model.md` |
| `docs/architecture/l2tp/cpe-1-pppoe-client.md` | Decisions | anchored to `session_run.go` | - | silent | This page documents the PPPoE client in `internal/component/l2tp/pppoeclient/`, anchoring `session_run.go` only for `LCPRejectNakOptions`; it says nothing about L2TP proxy AVPs, server-side `pppSession` IPv6CP decline lifetime, or L2TP `sess.username` |
| `docs/architecture/wire/l2tp.md` | AVP layout | anchored to `session_fsm.go` | - | silent | This page documents L2TPv2 header/AVP wire framing, hidden AVPs, and reliable delivery, anchoring `session_fsm.go` only for `handleWEN`/`handleSLI`; it says nothing about Proxy Authen Name identity seeding or accounting `User-Name` |
| `docs/guide/l2tp.md` | Authentication | the `// Design:` page of `snapshot.go`, `events/events.go`, and `cmd/l2tp.go`; modified by this spec | "The Proxy Authen Name only seeds the displayed username until an authenticated identity replaces it. A session with no such identity (no authentication ran, or the peer authenticated with an empty name) keeps the LAC's unverified Name, and RADIUS accounting sends it as User-Name." | departs | Stops seeding `sess.username` and CLI `username` from the LAC's unverified Proxy Authen Name; displays the claim in its own labelled field (`proxy-authen-name`) while preserving Thomas's decision that RADIUS accounting sends the claim as `User-Name` (and CoA matches it) only when no PPP identity authenticated. Phase 5 updates this paragraph and the `l2tp-auth-radius` accounting/CoA text in `docs/guide/l2tp.md` |
| `docs/architecture/traffic/cos-dynamic.md` | Consequences | anchored to `coa.go` | - | silent | This page documents dynamic Class of Service via RADIUS `Filter-Id` and VSAs, anchoring `coa.go` only for `readCoAChange`, `setRate`, and `setCoSProfile`; it says nothing about CoA `User-Name` session matching or L2TP proxy identity |
| `docs/architecture/testing/interop.md` | The suites, one per protocol area | the `// Design:` page of `l2tp.go`, `register_scenarios.go`, and `avprelay/main.go`; anchored to `l2tp.go` and `avprelay/main.go` | "A scenario directory is NAMED and carries no numeric prefix." | agrees | Names the new scenario `l2tp-proxy-pap-pppd` with no numeric prefix, registers its checker, plan, kernel requirement, and images through `registerStandardScenario`, keeps assertions in Go (`proxy_pap.go`), and proves red with `renegotiateProxiedLCP` reverted and the image rebuilt |
| `docs/architecture/testing/interop.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/testing/interop.md` | "Each Docker interop suite and scenario checks only the kernel capabilities it actually requires (`Suite.KernelCapabilities` unioned with selected `ScenarioPlan.KernelCapabilities`); suites whose selected scenarios require none run without unrelated kernel features (such as SRv6 or IPsec)" | agrees | Registers `l2tp-proxy-pap-pppd` with `needsKernel: true` in the L2TP suite so `plansAt` attaches only `{l2tp, l2tp-ppp}` as `ScenarioPlan.KernelCapabilities` and requires no unrelated kernel features |
| `docs/labs/l2tp-interop.md` | Layout | modified by this spec | "Each scenario registers its checker, plan, kernel prerequisite and image selection in the owning L2TP lab." | agrees | Registers `l2tp-proxy-pap-pppd` via `registerStandardScenario` in `register_scenarios.go` with `prepareScenario` / `standardImageBuilds` in `l2tp.go`, keeps all containers unprivileged (`NET_ADMIN` + `/dev/ppp`), and documents the new scenario and `avprelay` proxied mode in `docs/labs/l2tp-interop.md` |

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
- `registerStandardScenario` in `internal/le/interoplab/l2tp/register_scenarios.go` (read by `scenarioCheckerMap` in `scenarios.go`) and `prepareScenario` / `standardImageBuilds` in `internal/le/interoplab/l2tp/l2tp.go` - registers and prepares the scenario.

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
| Who else touches this path? | `plan/release/fixes/spec-rfc5176-remaining-defect-classes.md` (Status: design, P0) also edits `coa.go` and `acct.go` (unifying CoA/DM session selection onto `subscriber.Registry`); whichever spec lands second ensures `subscriber.Session` carries the LAC's unverified claim when unauthenticated and CoA `User-Name` matching compares against the same effective accounted name (authenticated `Username`, else LAC claim). `plan/post-release/features/spec-l2tp-proxy-authentication.md` (post-release) owns authenticating from Proxy Authen AVPs. |

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
- `internal/component/l2tp/ppp/session.go` - the per-lifetime decline field
- `internal/component/l2tp/ppp/session_run.go` - `handleFrame`, `supportsProtocol`, the comment
- `internal/component/l2tp/ppp/pap.go` - update the stale comment on `renegotiateProxiedLCP` that says Proxy Authen Name seeds the displayed username
- `internal/component/l2tp/session_fsm.go` - the seed into `sess.username` goes; `proxyAuthenName` stays its own field
- `internal/component/l2tp/session.go` - the comment on `username` says what is true
- `internal/component/l2tp/reactor_kernel.go` - emit the LAC claim separately on `SessionIPAssignedPayload`; clear `username` on every LCP down path that held an identity
- `internal/component/l2tp/events/events.go` - carry the LAC's unverified Proxy Authen Name separately from `Username` on `SessionIPAssignedPayload`
- `internal/component/l2tp/snapshot.go` - expose the LAC claim as its own field on `SessionSnapshot`
- `internal/component/l2tp/cmd/l2tp.go` - emit the LAC claim under its own labelled JSON key (`proxy-authen-name`) in `sessionJSON`
- `internal/component/l2tp/plugins/authradius/acct.go` - fall back to the LAC claim when `payload.Username` is empty in `onSessionIPAssigned` / `buildAcctPacket`, and update the comment
- `internal/component/l2tp/plugins/authradius/coa.go` - match `User-Name` against the name accounting sent (authenticated identity, else LAC claim), coordinated with `spec-rfc5176-remaining-defect-classes.md`
- `internal/le/interoplab/l2tp/l2tp.go` - `scenarioProxyPAP` constant, `prepareScenario`, and `standardImageBuilds`
- `internal/le/interoplab/l2tp/register_scenarios.go` - register `l2tp-proxy-pap-pppd` via `registerStandardScenario`
- `internal/le/interoplab/l2tp/avprelay/main.go` - proxied mode, and its header comment
- `docs/guide/l2tp.md` - the Proxy Authen paragraph, `show l2tp` session field, and `l2tp-auth-radius` accounting/CoA text
- `docs/architecture/l2tp/subscriber-session-model.md` - the transport identity sentence
- `docs/guide/radius.md` - keep the subscriber-accounting pointer to `docs/guide/l2tp.md` accurate
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
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/guide/l2tp.md`, `docs/architecture/l2tp/subscriber-session-model.md`, `docs/labs/l2tp-interop.md` (modified by this spec); `docs/research/l2tpv2-implementation-guide.md`, `docs/research/l2tpv2-ze-integration.md` (checked; neither claims Proxy Authen Name seeds `sess.username`). Advisory anchored pages checked and unaffected: `docs/architecture/l2tp/bng-1-radius-attributes.md` (empty-attribute omission and `acctSession` lifecycle unchanged), `docs/architecture/l2tp/cpe-1-pppoe-client.md` (PPPoE client only; `LCPRejectNakOptions` untouched), `docs/architecture/traffic/cos-dynamic.md` (CoS Filter-Id/VSA parsing untouched), `docs/architecture/wire/l2tp.md` (L2TP wire framing and `handleWEN`/`handleSLI` untouched), `docs/comparison.md` (no feature matrix change), `docs/functional-tests.md` (suite layout unchanged), `docs/guide/pppoe.md` (PPPoE guide; server-side IPv6CP decline lifetime already matches per-lifetime expectation), `docs/features.md` (summary unaffected). |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/l2tp.md` |

## Design presentation

### What changes

This fix resolves three defects in how an L2TP LNS session handles LAC-supplied proxy data and per-lifetime IPv6CP declines. First, the LAC's unverified Proxy Authen Name no longer seeds `sess.username` or the CLI `username` field: `username` holds only an accepted PPP-authenticated identity (empty on explicit no-auth admission or an empty authenticated name), while the LAC's claim stays in `proxyAuthenName` and is shown in its own separately labelled `show l2tp` field (`proxy-authen-name`). Per Thomas's owner decision, RADIUS accounting still sends the LAC's claim as `User-Name` when no PPP identity authenticated (an authenticated identity always wins, and neither omits `User-Name`), and RADIUS CoA/DM `User-Name` matching compares against that same accounted name. Second, a pool handler's IPv6 decline sets a dedicated per-lifetime field cleared by `resetNCP` instead of overwriting the operator's `disableIPv6CP` config flag, so LCP renegotiation asks the handler again while `enable-ipv6cp false` keeps refusing IPv6CP for the session's whole life. Third, proxied PAP LCP renegotiation (`renegotiateProxiedLCP`) gains end-to-end proof against real `xl2tpd` 1.3.20 and `pppd` 2.5.5 through a new `avprelay` proxied mode in the L2TP Docker interop lab (`l2tp-proxy-pap-pppd`).

### Before and after

1. **No-auth L2TP session whose ICCN carries Proxy Authen Name `lac-claim`:**
   - Before: `handleICCN` copies `lac-claim` into `sess.username`, so `show l2tp sessions` and `show l2tp session id <id>` display `username: "lac-claim"` identically to a PPP-authenticated subscriber.
   - After: `sess.username` stays `""`; `show l2tp` displays `username: ""` alongside a separate labelled claim field `proxy-authen-name: "lac-claim"`. RADIUS Accounting-Start and Accounting-Stop still send `User-Name = lac-claim` when nothing authenticated (whereas an authenticated `alice` always wins, and having neither omits `User-Name`), and CoA/DM by `User-Name = lac-claim` still matches that accounted session.
2. **Pool handler declines IPv6 in LCP lifetime 1, then LCP renegotiates to lifetime 2:**
   - Before: `runNCPPhase` writes `s.disableIPv6CP = true`, overwriting the operator config field, and `resetNCP` does not clear it; lifetime 2 continues to Protocol-Reject IPv6CP and never asks the pool handler again.
   - After: the decline sets a separate per-lifetime field that `resetNCP` clears; lifetime 1 still Protocol-Rejects IPv6CP, lifetime 2 asks the pool handler again and opens IPv6CP if granted, while `l2tp ncp enable-ipv6cp false` stays in force across every lifetime without querying the handler.
3. **Real `pppd` peer behind a LAC that already finished PAP:**
   - Before: `renegotiateProxiedLCP` is proven only against a scripted Go peer in `TestProxiedPAPWithoutRequestRenegotiatesLCP`; no interop scenario verifies a real `pppd` survives the renegotiation.
   - After: `l2tp-proxy-pap-pppd` runs real `xl2tpd` 1.3.20 and `pppd` 2.5.5 through `avprelay`, proving both renegotiation after LAC-completed PAP (with ICMP traffic over the established session) and the direct-request path with no unnecessary renegotiation, backed by a forced-red run with `renegotiateProxiedLCP` reverted.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Omit `User-Name` in RADIUS accounting whenever no PPP identity authenticated | Set aside by Thomas's owner decision: billing pipelines still expect the LAC's claimed name in `User-Name` for no-auth sessions when nothing authenticated, while keeping the claim out of `sess.username` and CLI `username` |
| Keep seeding `sess.username` from `proxyAuthenName` | Violates `sess.username`'s contract ("PPP-authenticated peer identity"), misleads operators in `show l2tp`, and contradicts `subscriber-session-model.md` ("Explicit no-auth admission has an empty username") |
| Save the initial `disableIPv6CP` config value and restore it inside `resetNCP` | Conflates operator configuration with a per-lifetime handler outcome in one mutable field; two distinct fields let `handleFrame` and `supportsProtocol` tell the two facts apart cleanly |
| Drive `pppd` on a pty from a compiled Go fixture, or wait for a third-party LAC that emits proxy AVPs | A pty fixture replaces `xl2tpd` on the wire with Ze's own code, and `xl2tpd` never emits proxy AVPs; adding a bounded proxied mode to `avprelay` keeps both `xl2tpd` and `pppd` real |

### Documents departed from

- `docs/guide/l2tp.md`, section `Authentication`: currently states `"The Proxy Authen Name only seeds the displayed username until an authenticated identity replaces it. A session with no such identity (no authentication ran, or the peer authenticated with an empty name) keeps the LAC's unverified Name, and RADIUS accounting sends it as User-Name."` After this work, the page states that the Proxy Authen Name never seeds `sess.username` or the displayed `username` field; `show l2tp` displays the LAC's unverified claim in its own labelled field (`proxy-authen-name`), while RADIUS accounting still falls back to that claim as `User-Name` (and CoA matches it) only when no PPP identity authenticated, omitting `User-Name` when neither is present.
- `docs/architecture/l2tp/subscriber-session-model.md`, section `Traps this code exists to avoid`: currently states `"L2TP retains a proxy identity supplied for the current transport until PPP supplies an accepted username."` After this work, the page states that `sess.username` holds only an accepted PPP identity (empty on explicit no-auth admission), while L2TP keeps the LAC's unverified Proxy Authen Name separately as a claim used by RADIUS accounting and CoA only when nothing authenticated.

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the scenario row and the failing tests
   - Tests: `TestProxyAuthenNameDoesNotSeedUsername`, `TestIPv6CPDeclineEndsWithLCPLifetime`, `l2tp-proxy-pap-pppd` selected and refused for the missing checker
   - Files: `internal/le/interoplab/l2tp/l2tp.go`, `internal/le/interoplab/l2tp/register_scenarios.go`
   - Verify: tests red for the reasons in the Task
2. **Phase: IPv6CP lifetime** -- D-2
   - Tests: `TestIPv6CPDeclineEndsWithLCPLifetime`, `TestOperatorIPv6CPDisableSurvivesLCPRestart`, `test/l2tp/ipv6cp-after-decline.ci`
   - Files: `ncp.go`, `ppp/session.go`, `session_run.go`
   - Verify: tests fail, implement, tests pass
3. **Phase: Identity** -- D-3
   - Tests: `TestProxyAuthenNameDoesNotSeedUsername`, `TestAcceptedNameReachesUsername`, `TestAcctUserNameIdentityElseClaim`, `TestCoAUserNameMatchesAccountedClaim`, `TestSnapshotSeparatesClaimFromIdentity`, `test/l2tp/proxy-name-accounted.ci`, `test/l2tp/radius-acct-wire.ci`
   - Files: `session_fsm.go`, `session.go`, `reactor_kernel.go`, `events/events.go`, `snapshot.go`, `cmd/l2tp.go`, `ppp/pap.go`, `acct.go`, `coa.go`
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
| Data flow | The Proxy Authen Name never enters `sess.username`; it reaches RADIUS accounting `User-Name` and CoA `User-Name` matching only when no PPP identity authenticated (authenticated identity always wins, neither omits `User-Name`), and never enters RADIUS `Access-Request` |
| Rule: stale comments | `session.go`, `acct.go`, `ppp/pap.go`, and the mirror comment in `session_fsm.go` say what is now true |
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
- Item 20 of the 2026-11-06 review read as a stall, and `926f01f732` already fixed it. What remains is the proof.
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
