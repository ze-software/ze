# Spec: home-cpe-16-lns-mru-and-auth-allow-list

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P1 |
| Release reason | An accepted configuration does the wrong thing: `auth-method chap-md5`, whose YANG description says "The default requires CHAP-MD5", authenticates a peer with PAP after that peer Naks CHAP, on the L2TP LNS and on the PPPoE access concentrator. A CHAP-only operator has no way to refuse the fallback. The LNS also has no MRU control, so every L2TP session requests 1500. |
| RFC gaps | - (no new requirement row; RFC 1661 Section 6.2 and Section 3.5 and RFC 1334 Section 2 behaviour is exercised by new tests) |
| Depends | - (interplay with `spec-home-cpe-15-lns-lcp-renegotiation.md` described under Key Design Decisions; neither blocks the other) |
| Phase | 0/4 |
| Handoff | - |
| Updated | 2026-10-31 |

Parent: plan/home-cpe/spec-home-cpe-0-umbrella.md

Recovery after compaction: `.claude/rules/post-compaction.md`.

All values in this spec are invented (documentation prefixes, `example.net`).

## Task

Two LNS controls a home-CPE access chain needs, and one defect behind the
second:

1. **MRU.** An L2TP session has no MRU leaf. `ppp.Driver` takes
   `StartSession.MaxMRU`, and the L2TP reactor never sets it, so every LNS
   session requests MRU 1500 in its LCP Configure-Request and Naks a peer MRU
   above 1500. A subscriber that reaches the LNS through a PPPoE LAC can carry
   at most 1492 (RFC 2516), so the operator needs to set 1492 (the umbrella
   lab's LNS is "MRU 1492"). Add an `mru` leaf under `l2tp` that feeds
   `MaxMRU` for every L2TP session.
2. **Authentication allow-list.** The fallback order on a Configure-Nak of the
   Auth-Protocol option is always the package default
   (CHAP-MD5, MS-CHAPv2, PAP). No configuration narrows it, so `auth-method
   chap-md5` accepts a Nak toward PAP and authenticates in clear text. Add an
   ordered allow-list of authentication methods, derive a safe default from
   `auth-method` when it is absent, and refuse a fallback outside the list by
   ending LCP rather than opening the link without authentication. The same
   list governs a proxied Auth-Protocol (proxy LCP from a LAC) on the LNS.
3. Check PPPoE-server sessions for both: the PPPoE AC already sets `MaxMRU`
   to 1492 (`PPPoEMaxMTU`), so it needs no MRU leaf; it shares the
   authentication defect and gets the same allow-list.

What "refuse" means needs care, because the authenticator does not Nak or
Reject here. The LNS sends the Configure-Request carrying Auth-Protocol, and
the PEER Naks it with a suggestion. The LNS's answer to a suggestion outside
the list is to not adopt it. When the list holds nothing the peer accepts,
the LNS ends the link (RFC 1661 Section 3.5) instead of reaching Opened
without the option.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/l2tp.md` - PPP negotiation (l.210-235), authentication
  (l.358-385), reload table (l.1180-1200)
  → Constraint: the guide states "If a peer rejects every acceptable
  Auth-Protocol, the PPP session is disconnected after LCP instead of falling
  through to the no-auth accounting path". The new refusal must keep the
  disconnect, and the page owes the allow-list.
  → Constraint: "The PPP MRU counts the Information field ... the kernel
  receive MRU stays at 1500 even when a smaller MRU was requested". A
  configured MRU below 1500 changes only what Ze requests, never what it can
  receive.
- [ ] `docs/guide/pppoe.md` - Subscriber authentication (l.80-100)
  → Constraint: `auth-method pap` still offers CHAP-MD5 first and offers PAP
  only after a Nak toward PAP (RFC 1334 Section 2). The derived default for
  `pap` must keep that.
- [ ] `docs/architecture/l2tp/followup-l2tp-call.md` - LAC/LNS roles
  → Decision: the PPP driver is shared by L2TP and PPPoE; transports fill
  `ppp.StartSession`.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc1661.md` - exists. RFC 1661 Section 6.2: "An
  implementation MUST NOT include multiple Authentication-Protocol
  Configuration Options in its Configure-Request packets.  Instead, it
  SHOULD attempt to configure the most desirable protocol first.  If that
  protocol is Configure-Nak'd, then the implementation SHOULD attempt the
  next most desirable protocol in the next Configure-Request."
  → Constraint: "most desirable" is the implementation's policy. The
  allow-list is that policy; a protocol outside it is not "next most
  desirable" at all.
- [ ] RFC 1661 Section 3.5: "If an implementation desires that the peer
  authenticate with some specific authentication protocol, then it MUST
  request the use of that authentication protocol during Link Establishment
  phase." and "If authentication fails, the authenticator SHOULD proceed
  instead to the Link Termination phase."
  → Decision: with authentication required and the list exhausted, LCP is
  closed (Terminate-Request) before Opened, rather than opened with the
  Auth-Protocol option dropped.
- [ ] RFC 1661 Section 6.1: "The default value is 1500 octets.  If smaller
  packets are requested, an implementation MUST still be able to receive the
  full 1500 octet information field in case link synchronization is lost."
  → Constraint: the leaf lowers the requested MRU only; Ze's receive buffers
  (`MaxFrameBufLen`) stay sized for 1500.
- [ ] `rfc/short/rfc1334.md` - exists. RFC 1334 Section 2: "Any
  implementations which include a stronger authentication method (such as
  CHAP, described below) MUST offer to negotiate that method prior to PAP."
  → Constraint: an allow-list containing a CHAP variant and PAP offers the
  CHAP variant first whatever order the operator wrote.
- [ ] `rfc/short/rfc1994.md` - exists; no row changes.
- [ ] `rfc/short/rfc2516.md` - exists; PPPoE MRU 1492 is already enforced.

**Key insights:**
- The defect is a policy gap, not a wire bug: Ze's Nak handling is correct
  for its list, and the list is unconfigurable.
- accel-ppp (local source, `accel-pppd/ppp/ppp_auth.c`,
  `auth_recv_conf_nak`) keeps only the configured auth modules in its list;
  a Nak marks the current one, and when every configured method has been
  Nak'd it logs "cann't negotiate authentication type" and fails LCP. That is
  the behaviour this spec adopts for the exhausted case.
- accel-ppp's L2TP LNS has `ppp-max-mtu` (`accel-pppd/ctrl/l2tp/l2tp.c`,
  `DEFAULT_PPP_MAX_MTU 1420`, fed into `ctrl.max_mtu`) and the generic `mru`
  (`accel-pppd/ppp/lcp_opt_mru.c`, `mru_init`), so an LNS MRU knob is
  established practice.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/reactor_kernel.go` - builds `ppp.StartSession`
  for an L2TP session (l.125-144): sets `AuthMethod`, `AuthRequired`,
  `AuthTimeout`, `ReauthInterval`, NCP flags, the three proxy LCP slices and
  `EchoInterval`. It sets neither `MaxMRU` nor `AuthFallbackOrder`.
- [ ] `internal/component/l2tp/ppp/manager.go` - session construction
  (l.470-497): `MaxMRU` zero becomes `MaxFrameLen` (1500); an empty
  `AuthFallbackOrder` becomes `defaultAuthFallbackOrder()`;
  `configuredAuthMethod` is `initialAuthMethod(start.AuthMethod, fallback)`.
- [ ] `internal/component/l2tp/ppp/auth.go` - `defaultAuthFallbackOrder`
  returns CHAP-MD5, MS-CHAPv2, PAP. `adjustAuthOnNakOrReject` maps a Nak's
  suggestion through `selectAuthFallback`; a suggestion outside the order
  becomes `AuthMethodNone`, so the next Configure-Request omits the option.
  `selectAuthFallback`'s comment says "an empty list means no fallback is
  acceptable", which `manager.go` contradicts by replacing an empty list with
  the default. `initialAuthMethod` replaces a configured PAP with the first
  CHAP variant in the order.
- [ ] `internal/component/l2tp/ppp/session_run.go` - `runAuthPhase`
  (l.740-758): with `AuthMethodNone` and `authRequired`, fails the session
  ("no negotiated authentication method") AFTER LCP reached Opened.
  `sendConfigureRequest` (l.1845-1870) requests MRU `localMRU` or `maxMRU`.
  The Nak handler (l.1340-1365) adopts a Nak'd MRU only when it is within
  `MinFrameLen..maxMRU`. The proxy short-circuit (l.199-260) takes
  `proxy.AuthProto` from the LAC's Last-Sent Configure-Request through
  `authMethodFromAuthProto` with no check against any policy, and takes the
  MRU from Last-Received only.
- [ ] `internal/component/l2tp/ppp/lcp_options.go` - `LCPNegPolicy.MaxMRU`
  and `effectiveMaxMRU`: a peer MRU above `MaxMRU` is Nak'd down to it, below
  64 is Nak'd up to 64.
- [ ] `internal/component/l2tp/ppp/proxy.go` - `EvaluateProxyLCP` reads MRU
  and Magic from Last-Received, Auth-Protocol and Magic from Last-Sent; it
  does not read Last-Sent's MRU (what the LAC asked the client to send).
- [ ] `internal/component/l2tp/pppoe/server.go` - PADS path (l.335-355):
  `MaxMRU: PPPoEMaxMTU` (1492), `AuthMethod`, `AuthRequired`; no
  `AuthFallbackOrder`.
- [ ] `internal/component/l2tp/config.go` - parses `auth-method` and
  `allow-no-auth` (l.240-250); `DefaultAuthMethod` is CHAP-MD5.
- [ ] `internal/component/l2tp/pppoe/config.go` - same leaves for PPPoE.
- [ ] `internal/component/l2tp/yang/ze-l2tp-conf.yang` - `auth-method`
  (l.50-61) "The default requires CHAP-MD5."; `authentication` container
  holds `timeout` and `reauth-interval` only.
- [ ] `internal/component/l2tp/pppoe/yang/ze-pppoe-conf.yang` - `auth-method`
  (l.56), `allow-no-auth` (l.75).
- [ ] `internal/component/l2tp/subsystem_reload.go` - hot-applies
  `auth-method` and `allow-no-auth` to new sessions through reactor setters.

**Behavior to preserve:**
- `auth-method pap` offers CHAP-MD5 first and PAP after a Nak toward PAP.
- `auth-method none` requires `allow-no-auth true`.
- A Configure-Reject of the Auth-Protocol option still clears the method;
  with authentication required the session still ends.
- With no `mru` configured, L2TP sessions request 1500 as today.
- PPPoE AC sessions keep `MaxMRU` 1492.
- Reload: both new leaves hot-apply to new sessions only, like `auth-method`.

**Behavior to change:**
- With no allow-list, the accepted fallbacks are derived from `auth-method`
  (table under Key Design Decisions), so `chap-md5` no longer falls back to
  PAP or MS-CHAPv2. This makes the code match the YANG description.
- Exhausting the list with authentication required ends LCP before Opened
  (Terminate-Request, logged cause), instead of opening without
  Auth-Protocol and failing in the auth phase.
- A proxied Auth-Protocol outside the list makes the proxy LCP unusable: the
  LNS runs LCP itself and requests an allowed method.
- A proxy LCP whose Last-Sent MRU (1500 when absent) exceeds the configured
  `mru` is likewise unusable, so the LNS renegotiates.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `l2tp { mru <n>; authentication { allow [ ... ] } }` and
  `pppoe { authentication-allow [ ... ] }` (exact PPPoE spelling decided in
  Phase 1 against the PPPoE YANG layout, which has no `authentication`
  container today).
- Wire: the peer's LCP Configure-Nak carrying an Auth-Protocol suggestion;
  the LAC's proxy LCP AVPs 26-28 in ICCN.

### Transformation Path
1. YANG parse → `ExtractParameters` (`l2tp/config.go`) and the PPPoE config
   parser fill `Parameters.MRU` and `Parameters.AuthAllow`.
2. Validation refuses an allow-list that omits `auth-method`, that contains
   `none`, or an `mru` outside the range.
3. Reactor (`reactor_kernel.go`) and PPPoE server (`server.go`) copy both
   into `ppp.StartSession.MaxMRU` and `ppp.StartSession.AuthFallbackOrder`.
4. `ppp.Driver` builds the session; `initialAuthMethod` and
   `selectAuthFallback` read the list; the Nak handler closes LCP when the
   list is exhausted and authentication is required.
5. The proxy short-circuit checks the proxied Auth-Protocol and Last-Sent MRU
   against the session's list and `maxMRU` and falls back to normal LCP when
   either is outside.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config → L2TP subsystem | `ExtractParameters`, reload diff | No |
| Config → PPPoE subsystem | PPPoE config parser, reload | No |
| Transport → PPP driver | `ppp.StartSession` fields | No |
| Ze ↔ PPP peer | LCP Configure-Request / Nak / Terminate-Request | No |

### Integration Points
- `ppp.StartSession.MaxMRU` - already honoured by the driver for PPPoE.
- `ppp.StartSession.AuthFallbackOrder` - already honoured; never set.
- `reactor_setters.go` - new setters beside `setPPPAuthRequired`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Values travel config → transport → `StartSession` → driver, the path `auth-method` already uses |
| No unintended coupling (components stay isolated) | Yes | PPPoE and L2TP each fill their own `StartSession`; ppp stays transport-neutral |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses `AuthFallbackOrder` and `MaxMRU`, which exist and are unset |
| Zero-copy preserved where applicable (refs, not copies) | Yes | No wire path changes buffer handling |
| Registration over hardcoding, outbound | Yes | No new command, family or handler |
| Registration over hardcoding, inbound | Yes | The method names come from the existing `ppp.ParseAuthMethod` and the YANG enumeration that `auth-method` uses; no new switch learns a name |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No existing test or interop scenario relies on `chap-md5` falling back to PAP | grep of `test/interop-pppoe/scenarios/pppoe-pap-ze-ac` and `test/interop-l2tp/scenarios/*/ze.conf` for `auth-method` | A scenario configured `chap-md5` with a PAP-only client goes red; it is reconfigured to `auth-method pap`, which the test judge reviews | grep in Phase 1, then the scenario runs | unvalidated |
| A-2 | Closing LCP before Opened on exhaustion interoperates: pppd and accel-ppp clients treat a Terminate-Request in Req-Sent/Ack-Sent as a normal failure | RFC 1661 Section 4.1 state table (RTR in those states: sta) | A client loops redialling fast | interop scenario `pppoe-chap-only-pap-client` asserts one PADT and the logged cause | unvalidated |
| A-3 | Ze's receive side stays at 1500 when `mru` is lower | `docs/guide/l2tp.md` l.232-235; `frame.go` `MaxFrameBufLen` | RFC 1661 Section 6.1 MUST breaks | unit test sends a 1500-octet frame to a session with `mru 1492` | unvalidated |
| A-4 | The cap on the peer's MRU (Nak above `MaxMRU`) is wanted together with the requested MRU | `LCPNegPolicy.MaxMRU` couples them today for PPPoE; accel-ppp `mru_init` caps both with `max_mtu` | An operator wants asymmetric MRUs | Key Design Decisions records it; no second leaf added | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The derived default narrows MS-CHAPv2 deployments that relied on a fallback from CHAP-MD5 | an interop or `.ci` test with an MS-CHAPv2-only client goes red | the operator writes `allow [ chap-md5 ms-chap-v2 ]`; the guide's migration note says so |
| R-2 | A proxied PAP session that used to come up now renegotiates, and the LAC's subscriber sees LCP restart | access-wholesale scenario logs a renegotiation | intended; documented under Behavior to change |
| R-3 | Terminating in LCP hides the reason from accounting | no Accounting-Stop for an exhausted negotiation | today's path also sends none before auth; the log line and a `ze_ppp_auth_failures_total` reason label carry it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | New PPP sessions fail to authenticate or come up with the wrong MRU; established sessions are untouched |
| How is it reverted? | Single commit revert; the leaves are new and optional |
| Who else touches this path? | `spec-home-cpe-15-lns-lcp-renegotiation.md` (same proxy short-circuit in `session_run.go`), `spec-home-cpe-14-lac-realm-relay.md` (LAC-mode PPPoE sessions use the same allow-list), `plan/post-release/features/spec-l2tp-proxy-authentication.md` (LNS proxy auth consumes the proxied method) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `l2tp { mru 1492 }` config | → | `reactor_kernel.go` StartSession `MaxMRU` → `sendConfigureRequest` | `TestL2TPStartSessionCarriesConfiguredMRU` (`internal/component/l2tp/reactor_ppp_linux_test.go`) |
| `l2tp { authentication { allow [ chap-md5 ] } }` | → | `StartSession.AuthFallbackOrder` → `adjustAuthOnNakOrReject` | `TestL2TPStartSessionCarriesAuthAllowList` (`internal/component/l2tp/reactor_ppp_linux_test.go`) |
| PPPoE `authentication-allow` config | → | `server.go` StartSession → driver | `TestPPPoEStartSessionCarriesAuthAllowList` (`internal/component/l2tp/pppoe/server_test.go`) |
| `ze config validate` with both leaves | → | `ExtractParameters` validation | `test/l2tp/lns-mru-auth-allow-config.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `l2tp { mru 1492 }`, a new L2TP session | Ze's LCP Configure-Request carries MRU 1492; a peer MRU of 1500 is Nak'd with 1492 |
| AC-2 | No `mru` leaf | L2TP sessions request 1500 exactly as today |
| AC-3 | `mru` 63 or 1501 | `ze config validate` refuses it, naming the leaf, the value and the range |
| AC-4 | `mru 1492`, the peer sends a 1500-octet Information field | Ze receives it (RFC 1661 Section 6.1) |
| AC-5 | `auth-method chap-md5`, no allow-list, peer Naks with PAP | Ze does not offer PAP; LCP is closed with a Terminate-Request before Opened; the log names the peer's suggestion and the allowed methods; no auth request reaches the auth handler |
| AC-6 | `auth-method chap-md5`, `allow [ chap-md5 pap ]`, peer Naks with PAP | Ze offers PAP next and authenticates with PAP (today's behaviour, now opted into) |
| AC-7 | `auth-method pap`, no allow-list | First Configure-Request offers CHAP-MD5; a Nak toward PAP leads to PAP (RFC 1334 Section 2 order kept) |
| AC-8 | `allow [ pap chap-md5 ]` written in that order | CHAP-MD5 is still offered first (RFC 1334 Section 2) |
| AC-9 | allow-list omits `auth-method`, or contains `none` | `ze config validate` refuses it with the fix in the message |
| AC-10 | `allow-no-auth true`, list exhausted | Today's path: the option is dropped and the session runs the no-auth phase |
| AC-11 | LNS with `allow [ chap-md5 ]` receives proxy LCP whose Last-Sent Auth-Protocol is PAP | The proxy LCP is not used; Ze sends its own Configure-Request with CHAP-MD5 and the session authenticates with CHAP |
| AC-12 | LNS with `mru 1492` receives proxy LCP whose Last-Sent carries no MRU (1500) | The proxy LCP is not used; Ze renegotiates with MRU 1492 |
| AC-13 | PPPoE AC with `chap-md5`, a pppd client that refuses CHAP | Session ends in LCP with PADT; no PAP authentication happens |
| AC-14 | Reload changes `mru` or the allow-list | New sessions use the new value; established sessions keep theirs |
| AC-15 | `ze show l2tp` / session snapshot | The session shows the requested MRU and the negotiated method (existing fields) with the configured value |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Sets `l2tp mru 1492` on an LNS behind a PPPoE LAC | config → reactor → StartSession → LCP Configure-Request | interop `l2tp-lns-mru-xl2tpd` |
| 2 | Runs a CHAP-only LNS; a client offers only PAP | LCP Nak → allow-list → Terminate-Request | interop `l2tp-lns-chap-only-xl2tpd` |
| 3 | Runs a CHAP-only PPPoE AC; a client offers only PAP | PADS → LCP Nak → Terminate → PADT | interop `pppoe-chap-only-pap-client` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAuthAllowDefaultFromAuthMethod` | `internal/component/l2tp/config_test.go` | Derived list per `auth-method` (table in Key Design Decisions) | |
| `TestAuthAllowValidation` | `internal/component/l2tp/config_test.go` | Refusals of AC-9 and their messages | |
| `TestMRULeafRange` | `internal/component/l2tp/config_test.go` | AC-3 | |
| `TestAuthNakOutsideAllowListClosesLCP` | `internal/component/l2tp/ppp/auth_dispatch_test.go` | AC-5: Terminate-Request sent, never Opened, no `EventAuthRequest` | |
| `TestAuthNakInsideAllowListFallsBack` | `internal/component/l2tp/ppp/auth_dispatch_test.go` | AC-6 | |
| `TestInitialAuthMethodCHAPBeforePAPWhateverOrder` | `internal/component/l2tp/ppp/auth_test.go` | AC-7, AC-8 | |
| `TestEmptyAuthFallbackOrderMeansNoFallback` | `internal/component/l2tp/ppp/manager_test.go` | Removes the hidden default; `selectAuthFallback`'s documented contract holds | |
| `TestProxyLCPAuthOutsideAllowListRenegotiates` | `internal/component/l2tp/ppp/manager_test.go` | AC-11 | |
| `TestProxyLCPLastSentMRUAboveMaxRenegotiates` | `internal/component/l2tp/ppp/proxy_test.go` | AC-12 | |
| `TestConfiguredMRURequestedAndCaps` | `internal/component/l2tp/ppp/lcp_options_test.go` | AC-1 | |
| `TestReceiveFullFrameBelowConfiguredMRU` | `internal/component/l2tp/ppp/rfc1332_mtu_test.go` | AC-4 | |
| `TestL2TPStartSessionCarriesConfiguredMRU` | `internal/component/l2tp/reactor_ppp_linux_test.go` | wiring | |
| `TestL2TPStartSessionCarriesAuthAllowList` | `internal/component/l2tp/reactor_ppp_linux_test.go` | wiring | |
| `TestPPPoEStartSessionCarriesAuthAllowList` | `internal/component/l2tp/pppoe/server_test.go` | wiring | |
| `TestReloadAppliesMRUAndAllowToNewSessions` | `internal/component/l2tp/subsystem_reload_test.go` | AC-14 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `l2tp mru` | 64..1500 (the code's Nak floor of 64 and `MaxFrameLen`) | 64 and 1500 | 63 | 1501 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `lns-mru-auth-allow-config` | `test/l2tp/lns-mru-auth-allow-config.ci` | Operator validates a config with `mru` and an allow-list; bad values refused with a fix | |
| `pppoe-auth-allow-config` | `test/pppoe/pppoe-auth-allow-config.ci` | Same for the PPPoE AC | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `l2tp-lns-mru-xl2tpd` | `test/interop-l2tp/scenarios/` | xl2tpd LAC, pppd client | pppd logs Ze's MRU 1492 request; the `pppN` MTU is what was negotiated | |
| `l2tp-lns-chap-only-xl2tpd` | `test/interop-l2tp/scenarios/` | xl2tpd LAC, pppd with `refuse-chap` | session refused in LCP, no PAP exchange on the wire | |
| `pppoe-chap-only-pap-client` | `test/interop-pppoe/scenarios/` | pppd rp-pppoe with `refuse-chap` | AC-13 | |

Each scenario is broken once on purpose (the derived default restored to
the old order, the MRU leaf ignored) to show it goes red.

## Files to Modify
- `internal/component/l2tp/yang/ze-l2tp-conf.yang` - `mru` leaf; `allow`
  leaf-list under `authentication`; correct the `auth-method` description
- `internal/component/l2tp/pppoe/yang/ze-pppoe-conf.yang` - allow-list leaf
- `internal/component/l2tp/config.go` - parse, derive default, validate
- `internal/component/l2tp/pppoe/config.go` - same for PPPoE
- `internal/component/l2tp/reactor_kernel.go` - set `MaxMRU`, `AuthFallbackOrder`
- `internal/component/l2tp/reactor_setters.go`, `subsystem_reload.go` - hot-apply
- `internal/component/l2tp/pppoe/server.go`, `pppoe/subsystem_reload.go` - set and hot-apply the list
- `internal/component/l2tp/ppp/manager.go` - drop the implicit default for an empty list
- `internal/component/l2tp/ppp/auth.go` - CHAP-before-PAP whatever the order; exhaustion signal
- `internal/component/l2tp/ppp/session_run.go` - exhaustion closes LCP; proxy checks against list and `maxMRU`
- `internal/component/l2tp/ppp/proxy.go` - read Last-Sent MRU
- `docs/guide/l2tp.md`, `docs/guide/pppoe.md` - leaves, behaviour, reload table rows
- `features/l2tpv2-bng.md`, `features/pppoe-access.md` - scope

## Files to Create
- `test/l2tp/lns-mru-auth-allow-config.ci`
- `test/pppoe/pppoe-auth-allow-config.ci`
- `test/interop-l2tp/scenarios/l2tp-lns-mru-xl2tpd/`
- `test/interop-l2tp/scenarios/l2tp-lns-chap-only-xl2tpd/`
- `test/interop-pppoe/scenarios/pppoe-chap-only-pap-client/`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | two leaves in `ze-l2tp-conf.yang`, one in `ze-pppoe-conf.yang` |
| YANG validation constraints | Yes | `range "64..1500"`; leaf-list of the existing auth enumeration minus `none` |
| YANG custom validators | Yes | allow-list must contain `auth-method`; `ze:validate` with the same check `ExtractParameters` runs |
| CLI commands/flags | No | config only |
| CLI grammar (keyword before value) | N-A | no command |
| Editor autocomplete | Yes | automatic from the enumeration |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | no output |
| Env var registration | N-A | not under `environment` |
| Doctor check for runtime dependencies | N-A | no new runtime dependency |
| Prometheus counters/metrics | Yes | reason label `auth-method-refused` on the existing PPP auth failure counter (name confirmed in Phase 1 from `ppp/metrics.go`) |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/l2tpv2-bng.md`, `features/pppoe-access.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/l2tp.md`, `docs/guide/pppoe.md` (configuration sections) |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/guide/l2tp.md` (PPP negotiation, Authentication, reload table), `docs/guide/pppoe.md` |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc1661.md` (Section 6.2 and 3.5 rows' annotations), `rfc/short/rfc1334.md` (Section 2 row) |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | Yes | the PPP metrics section of `docs/guide/l2tp.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derived with `./le spec citation anchors spec plan/home-cpe/spec-home-cpe-16-lns-mru-and-auth-allow-list.md` at implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the `l2tp {}` example at `docs/guide/l2tp.md` l.22-34 |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- YANG leaves, `Parameters` fields,
   `StartSession` assignment in both transports, failing wiring tests.
   - Tests: `TestL2TPStartSessionCarriesConfiguredMRU`,
     `TestL2TPStartSessionCarriesAuthAllowList`,
     `TestPPPoEStartSessionCarriesAuthAllowList`
   - Files: YANG, `config.go` (both), `reactor_kernel.go`, `pppoe/server.go`
   - Verify: tests fail on the stubbed value, then pass.
2. **Phase: Defaults and validation** -- derived default, refusals, `.ci`.
3. **Phase: Driver** -- empty list means no fallback, CHAP before PAP,
   exhaustion closes LCP, proxy checks; unit tests.
4. **Phase: Docs and interop** -- guide pages, feature files, RFC summary
   annotations, three interop scenarios with a recorded red.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | No path reaches `AuthMethodPAP` unless PAP is in the session's list |
| Correctness | The exhausted case sends Terminate-Request and never emits `EventLCPUp` |
| Naming | YANG leaf names kebab-case; enumeration values identical to `auth-method` |
| Data flow | ppp package never reads config; transports pass values |
| Rule: no-layering | the package-default fallback order is deleted, not kept as a hidden fallback |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `mru` leaf reaches the wire | interop `l2tp-lns-mru-xl2tpd` |
| CHAP-only refuses PAP on both transports | the two `chap-only` interop scenarios and `TestAuthNakOutsideAllowListClosesLCP` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Downgrade | No Nak, Reject or proxy AVP can move a session to a method outside the list |
| Fail-closed | An unreadable allow-list in config refuses the config; it never widens to the default order |

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

- The YANG text "The default requires CHAP-MD5" is the contract the code
  breaks; the fix makes the code honour the documented default rather than
  re-documenting the fallback.
- `selectAuthFallback` documents "an empty list means no fallback", and
  `manager.go` silently replaces an empty list with the full default. That is
  a zero standing in for an answer (`ai/rules/principles.md`).

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

None: `grep -rn -i "MaxMRU\|fallback.*PAP\|auth.*allow" plan/journal/` found
no open row for this defect.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Derived default when no list: `chap-md5` → [chap-md5]; `ms-chap-v2` → [ms-chap-v2]; `pap` → [chap-md5, pap]; `none` → [] | Keep today's [chap-md5, ms-chap-v2, pap] default and only add the list | The YANG description promises "The default requires CHAP-MD5"; a default that accepts PAP is the defect. `pap` keeps CHAP-MD5 first because RFC 1334 Section 2 says an implementation with a stronger method "MUST offer to negotiate that method prior to PAP" and the guide documents that behaviour |
| Exhaustion with authentication required closes LCP before Opened | Today's drop-the-option, open, then fail in `runAuthPhase` | RFC 1661 Section 3.5: an implementation that desires authentication "MUST request the use of that authentication protocol during Link Establishment phase"; opening without it contradicts that. accel-ppp fails LCP the same way (`auth_recv_conf_nak`) |
| A proxied method outside the list, or a Last-Sent MRU above `mru`, makes the proxy LCP unusable and Ze negotiates LCP itself | Accept the proxy and refuse the session | The existing fallback path in `session_run.go` already runs full LCP when `EvaluateProxyLCP` fails; renegotiation (RFC 2661 Section 4.4.5: "the LNS policy may be to enter an additional round of LCP negotiation") lets the session succeed with an allowed method |
| One `mru` leaf drives both the requested MRU and the cap on the peer's MRU | Separate `mru` and `max-peer-mru` leaves | `LCPNegPolicy.MaxMRU` already couples them, accel-ppp caps both with one `max_mtu`, and no case in the umbrella needs asymmetry (A-4) |
| No `mru` leaf on the PPPoE AC | Add one capped at 1492 | The AC already requests 1492 (`PPPoEMaxMTU`), which RFC 2516 makes the maximum; lowering it is not needed by any child |
| CHAP variants offered before PAP whatever order the list is written in | Honour the written order literally | RFC 1334 Section 2 MUST |

## Known Limitations

- PPPoE AC MRU stays fixed at 1492; RFC 4638 larger MTUs are outside this
  spec and have no owner in the home-CPE set.
- Per-user allowed methods (from RADIUS) are not offered; the list is per
  transport.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code:
RFC 1661 Section 3.5 above the exhaustion close, RFC 1334 Section 2 above
the CHAP-first ordering, RFC 1661 Section 6.1 above the MRU request.

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
