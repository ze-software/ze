# Spec: home-cpe-16-lns-mru-and-auth-allow-list

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Priority | P1 |
| Release reason | An accepted configuration does the wrong thing: `auth-method chap-md5`, whose YANG description says "The default requires CHAP-MD5", authenticates a peer with PAP after that peer Naks CHAP, on the L2TP LNS and on the PPPoE access concentrator. A CHAP-only operator has no way to refuse the fallback. The LNS also has no MRU control, so every L2TP session requests 1500. |
| RFC gaps | - |
| Depends | Implementation: none (interplay with `spec-home-cpe-15-lns-lcp-renegotiation.md` described under Key Design Decisions; neither blocks the other). Closure: the three scenarios this spec owns run in the existing single-link `interop-l2tp` and `interop-pppoe` suites and wait on no other child. Children 14 and 19 depend on this spec for implementation |
| Phase | 4/4 |
| Handoff | - |
| Updated | 2026-11-04 |

Parent: plan/release/fixes/spec-home-cpe-0-umbrella.md

RFC gaps, in words: no new requirement row. RFC 1661 Section 6.2 and
Section 3.5 and RFC 1334 Section 2 behaviour is exercised by new tests.

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
- [ ] `docs/guide/l2tp.md` - "PPP negotiation", "Authentication", and the
  table in "Reload semantics"
  → Constraint: the guide states "If a peer rejects every acceptable
  Auth-Protocol, the PPP session is disconnected after LCP instead of falling
  through to the no-auth accounting path". The new refusal must keep the
  disconnect, and the page owes the allow-list.
  → Constraint: "The PPP MRU counts the Information field ... the kernel
  receive MRU stays at 1500 even when a smaller MRU was requested". A
  configured MRU below 1500 changes only what Ze requests, never what it can
  receive.
- [ ] `docs/guide/pppoe.md` - "Subscriber authentication"
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
- [ ] `internal/component/l2tp/reactor_kernel.go` - `handleKernelSuccess`
  builds `ppp.StartSession` for an L2TP session: sets `AuthMethod`, `AuthRequired`,
  `AuthTimeout`, `ReauthInterval`, NCP flags, the three proxy LCP slices and
  `EchoInterval`. It sets neither `MaxMRU` nor `AuthFallbackOrder`.
- [ ] `internal/component/l2tp/ppp/manager.go` - session construction in
  `Driver.spawnSession`: `MaxMRU` zero becomes `MaxFrameLen` (1500); an empty
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
- [ ] `internal/component/l2tp/ppp/session_run.go` - `runAuthPhase`: with
  `AuthMethodNone` and `authRequired`, fails the session
  ("no negotiated authentication method") AFTER LCP reached Opened.
  `sendConfigureRequest` requests MRU `localMRU` or `maxMRU`.
  The Configure-Nak and Configure-Reject branch of `handleLCPPacket` adopts a
  Nak'd MRU only when it is within `MinFrameLen..maxMRU`. The proxy
  short-circuit at the start of `run` (the branch taken when the LAC sent
  proxy LCP AVPs) takes
  `proxy.AuthProto` from the LAC's Last-Sent Configure-Request through
  `authMethodFromAuthProto` with no check against any policy, and takes the
  MRU from Last-Received only.
- [ ] `internal/component/l2tp/ppp/lcp_options.go` - `LCPNegPolicy.MaxMRU`
  and `effectiveMaxMRU`: a peer MRU above `MaxMRU` is Nak'd down to it, below
  64 is Nak'd up to 64.
- [ ] `internal/component/l2tp/ppp/proxy.go` - `EvaluateProxyLCP` reads MRU
  and Magic from Last-Received, Auth-Protocol and Magic from Last-Sent; it
  does not read Last-Sent's MRU (what the LAC asked the client to send).
- [ ] `internal/component/l2tp/pppoe/server.go` - PADS path in `handlePADR`:
  `MaxMRU: PPPoEMaxMTU` (1492), `AuthMethod`, `AuthRequired`; no
  `AuthFallbackOrder`.
- [ ] `internal/component/l2tp/config.go` - parses `auth-method` and
  `allow-no-auth` in `ExtractParameters`; `DefaultAuthMethod` is CHAP-MD5.
- [ ] `internal/component/l2tp/pppoe/config.go` - same leaves for PPPoE.
- [ ] `internal/component/l2tp/yang/ze-l2tp-conf.yang` - leaf `auth-method`
  "The default requires CHAP-MD5."; `authentication` container
  holds `timeout` and `reauth-interval` only.
- [ ] `internal/component/l2tp/pppoe/yang/ze-pppoe-conf.yang` - leaves
  `auth-method` and `allow-no-auth`.
- [ ] `internal/component/l2tp/subsystem_reload.go` - hot-applies
  `auth-method` and `allow-no-auth` to new sessions through reactor setters.

**Behavior to preserve:**
- `auth-method pap` offers CHAP-MD5 first and PAP after a Nak toward PAP.
- `auth-method none` requires `allow-no-auth true`.
- A Configure-Reject of the Auth-Protocol option clears the method for the
  current negotiation. With authentication required, LCP closes before Opened;
  otherwise the no-auth path remains available (RFC 1661 Section 5.4).
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

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Session MRU cap | `internal/component/l2tp/ppp/start_session.go` `StartSession.MaxMRU`; `internal/component/l2tp/ppp/lcp_options.go` `LCPNegPolicy`, `effectiveMaxMRU` | requested MRU and the Nak cap on the peer's MRU; zero means 1500; PPPoE sets 1492 (`pppoe/discovery.go` `PPPoEMaxMTU`) | extends: the L2TP reactor sets it from the new `mru` leaf; PPPoE unchanged |
| Auth fallback order | `internal/component/l2tp/ppp/start_session.go` `StartSession.AuthFallbackOrder`; `internal/component/l2tp/ppp/auth.go` `defaultAuthFallbackOrder`, `initialAuthMethod`, `selectAuthFallback`, `adjustAuthOnNakOrReject` | an order no transport sets; the driver substitutes the package default for an empty list | extends: both transports set it from the allow-list; replaces the implicit package default (deleted, no-layering) |
| Auth phase failure | `internal/component/l2tp/ppp/session_run.go` `runAuthPhase` | fails a required-auth session with no method after LCP Opened | replaces for the exhausted case: LCP closes before Opened |
| Proxy LCP evaluation | `internal/component/l2tp/ppp/proxy.go` `EvaluateProxyLCP`; `session_run.go` proxy branch | accepts the LAC's proxied Auth-Protocol and MRU with no policy check, falls back to full LCP on parse failure | extends: a method outside the list or a Last-Sent MRU above `mru` takes the same fall-back |
| Auth method names | `internal/component/l2tp/ppp/auth_events.go` `ParseAuthMethod`; `auth-method` YANG enumeration | the one parser and enumeration for method names | extends: the allow-list reuses both; no new name list |
| Config extraction and hot reload | `internal/component/l2tp/config.go` `ExtractParameters`, `reactor_setters.go`, `subsystem_reload.go`; `internal/component/l2tp/pppoe/config.go`, `pppoe/subsystem_reload.go` | parse `auth-method`, `allow-no-auth` and apply them to new sessions | extends: the new leaves travel the same path |
| Original-tree validation | `internal/component/config/tree_validators.go` `RegisterTreeValidator` | component-owned validators inspect sibling leaves before defaults and inheritance | extends: each transport registers policy validation; per-value `ze:validate` cannot inspect sibling `auth-method` |
| Global proxy LCP policy | `plan/release/fixes/spec-home-cpe-15-lns-lcp-renegotiation.md` | adds the operator's `proxy-lcp` choice | does not apply here: this spec's per-session checks use the same fall-back, neither blocks the other |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `l2tp { mru <n>; authentication { allow [ ... ] } }` and
  `pppoe { authentication-allow [ ... ] }`. PPPoE keeps its flat layout
  beside `auth-method` and `allow-no-auth`.
- Wire: the peer's LCP Configure-Nak carrying an Auth-Protocol suggestion;
  the LAC's proxy LCP AVPs 26-28 in ICCN.

### Transformation Path
1. YANG parse → `ExtractParameters` (`l2tp/config.go`) and the PPPoE config
   parser fill `Parameters.MRU` and `Parameters.AuthAllow`.
2. Validation refuses an allow-list that omits `auth-method`, that contains
   `none`, any allow-list while `auth-method` is `none`, or an `mru` outside
   the range.

#### Authentication list semantics

The list is a set of acceptable methods plus one ordering rule. Written order
matters only between the two CHAP variants.

| Question | Rule | Source |
|----------|------|--------|
| Which method is offered first | `auth-method` when it is a CHAP variant; when it is `pap`, the first CHAP variant in the list's written order, else PAP | RFC 1334 Section 2 (CHAP before PAP); `initialAuthMethod` already replaces a configured PAP with the first CHAP variant of its order |
| Order between `chap-md5` and `ms-chap-v2` | the list's written order, after `auth-method` | RFCs 1994, 2433 and 2759 state no preference between them; accel-ppp offers its auth modules in registration order (`ppp_auth_register_handler` appends to the handler list that `auth_init` walks), which the operator sets by module order, so the operator's written order is the established practice |
| Peer Naks with a suggestion in the list, not yet tried | adopt the suggestion | RFC 1661 Section 5.3: Nak'd options "MAY be modified as specified in the Configure-Nak"; today's `selectAuthFallback` |
| Peer Naks with PAP while an untried CHAP variant is in the list | offer that CHAP variant next; PAP only once no CHAP variant is left | RFC 1334 Section 2 "MUST offer to negotiate that method prior to PAP" |
| Peer Naks with a method outside the list or already tried | offer the next untried method in the order above | RFC 1661 Section 6.2, "SHOULD attempt the next most desirable protocol" |
| Peer Configure-Rejects Auth-Protocol | remove the option for this negotiation; close LCP before Opened when authentication is required, otherwise continue without it | RFC 1661 Section 5.4: "Reception of a valid Configure-Reject indicates that when a new Configure-Request is sent, it MUST NOT include any of the Configuration Options listed in the Configure-Reject." |
| No untried method left, authentication required | LCP closed with Terminate-Request before Opened (AC-5) | RFC 1661 Section 3.5; accel-ppp fails LCP ("cann't negotiate authentication type") |
| No untried method left, `allow-no-auth true` | the option is dropped and the no-auth phase runs (AC-10) | today's path |
| An empty list | cannot be written: a leaf-list with no entry is absent, and an absent list is the derived default. The derived default for `auth-method none` is the empty list, which the driver reads as "request no Auth-Protocol"; `manager.go`'s substitution of the full default order is deleted | YANG leaf-list semantics; `selectAuthFallback`'s own comment |
| `none` in the list | refused at verify: `none` is the absence of a method, not a fallback | AC-9 |
| `auth-method none` with a list | refused at verify, naming both leaves and the fix (delete the list, or set `auth-method` to a method in it) | the list must contain `auth-method` and may not contain `none`, so no list satisfies both |

Test matrix, `TestAuthAllowSelectionMatrix` (`internal/component/l2tp/ppp/auth_policy_driver_test.go`), one subtest per row:

| `auth-method` | list | Peer's answers | Ze offers, in order | Outcome |
|---------------|------|----------------|---------------------|---------|
| `chap-md5` | absent | Ack | CHAP-MD5 | CHAP-MD5 |
| `chap-md5` | absent | Nak PAP | CHAP-MD5 | Terminate-Request |
| `chap-md5` | `[chap-md5 pap]` | Nak PAP, Ack | CHAP-MD5, PAP | PAP |
| `chap-md5` | `[chap-md5 ms-chap-v2 pap]` | Nak PAP, Nak PAP, Ack | CHAP-MD5, MS-CHAPv2, PAP | PAP |
| `ms-chap-v2` | `[chap-md5 ms-chap-v2]` | Nak CHAP-MD5, Ack | MS-CHAPv2, CHAP-MD5 | CHAP-MD5 |
| `pap` | absent | Nak PAP, Ack | CHAP-MD5, PAP | PAP |
| `pap` | `[pap ms-chap-v2 chap-md5]` | Nak PAP, Nak PAP, Ack | MS-CHAPv2, CHAP-MD5, PAP | PAP |
| `chap-md5` | absent, `allow-no-auth true` | Reject | CHAP-MD5, then no option | no-auth phase |
| `none` | absent | - | no Auth-Protocol option | no authentication |
| `none` | `[chap-md5]` | - | - | refused at verify |
| `chap-md5` | `[pap]` | - | - | refused at verify (omits `auth-method`) |
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
| Config → L2TP subsystem | `ExtractParameters`, reload diff | PASS: `TestConfigPPPPolicy`, `TestProviderPPPPolicyBoundaries`, `TestReloadPPPPolicyRefusalKeepsEffectiveConfig` |
| Config → PPPoE subsystem | PPPoE config parser, reload | PASS: `TestPPPoEAuthAllowDefaults`, `TestPPPoEAuthAllowProviderShapes`, `TestPPPoEAuthAllowReloadIsolation` |
| Transport → PPP driver | `ppp.StartSession` fields | PASS: `TestReactorPPPPolicyReloadIsolation`, `TestPPPoEAuthAllowReachesDriver` |
| Ze ↔ PPP peer | LCP Configure-Request / Nak / Terminate-Request | Real-driver fixtures PASS; current pppd2.5.5 MRU and both transport refusal scenarios each completed baseline GREEN → rebuilt producer semantic RED → fresh normal RESTOREDGREEN; current PAP/CHAP compatibility also PASS |

### Integration Points
- `ppp.StartSession.MaxMRU` - L2TP supplies the configured value; PPPoE retains 1492.
- `ppp.StartSession.AuthFallbackOrder` - both transports supply the resolved policy; the driver supplies no hidden fallback.
- `reactor_setters.go` - `setPPPPolicy` publishes MRU/method/required/list together for new sessions.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Values travel config → transport → `StartSession` → driver, the path `auth-method` already uses |
| No unintended coupling (components stay isolated) | Yes | PPPoE and L2TP each fill their own `StartSession`; ppp stays transport-neutral |
| No duplicated functionality (extends existing, does not recreate) | Yes | Both transports explicitly supply the existing `AuthFallbackOrder` and `MaxMRU`; shared `ResolveAuthAllow` owns policy derivation |
| Zero-copy preserved where applicable (refs, not copies) | Yes | No wire path changes buffer handling |
| Registration over hardcoding, outbound | Yes | No new command, family or handler |
| Registration over hardcoding, inbound | Yes | The method names come from the existing `ppp.ParseAuthMethod` and the YANG enumeration that `auth-method` uses; no new switch learns a name |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No existing test or interop scenario relies on `chap-md5` falling back to PAP | Initial scenario-config scan missed legacy test dependence on implicit policy | Legacy tests require explicit intended policy or replacement of the deleted broad-default contract; preserve meaningful assertions under independent judgment | BROKEN: P exposed legacy fixture failures, including `TestRFC1661NakHandlesDifferentOptionLength`; old `TestDefaultAuthFallbackOrder` explicitly required the broad default. Independent judgment approved precise corrections/replacement; F PASS. Current pppd2.5.5 PAP compatibility also PASS bg45, separately; it cannot make the original universal claim true | broken |
| A-2 | Closing LCP before Opened on exhaustion interoperates: pppd and accel-ppp clients treat a Terminate-Request in Req-Sent/Ack-Sent as a normal failure | Both actual peer sources acknowledge/re-enter ReqSent, not immediate process exit; actual lower-layer completion required | A client loops redialling fast or TermAck is mistaken for completed lower-layer teardown | Both source FSMs confirmed Ack/ReqSent semantics. Current2.5.5 pppd L2TP normal baseline/restoration bg41/bg43 PASS with TermReq/Ack→natural CDN/pppd exit/empty state; current PPPoE bg43 PASS with TermReq/Ack→one PADT/daemon/kernel cleanup. This confirms the bounded interpretation below; accel evidence source-only, not runtime | confirmed |
| A-3 | Ze's receive side stays at 1500 when `mru` is lower | `docs/guide/l2tp.md` "PPP negotiation" (the kernel receive MRU sentence); `frame.go` `MaxFrameBufLen` | RFC 1661 Section 6.1 MUST breaks | PASS `TestReceiveFullFrameBelowConfiguredMRU`: real Driver receives/answers1500 at requested1492 and kernel receive MRU remains1500, product race log P | confirmed |
| A-4 | The cap on the peer's MRU (Nak above `MaxMRU`) is wanted together with the requested MRU | `LCPNegPolicy.MaxMRU` couples them today for PPPoE; accel-ppp `mru_init` caps both with `max_mtu` | An operator wants asymmetric MRUs | Approved Key Design Decisions retain coupling; PASS `TestConfiguredMRUNakInformationFloor` exercises requested values and peer1500 Nak to1492, log P | confirmed |

A-2 distinguishes protocol acknowledgment from completed subscriber teardown.
[pppd2.5.5 `fsm_rtermreq`](https://raw.githubusercontent.com/ppp-project/ppp/v2.5.5/pppd/fsm.c)
(lines577–599) returns AckRcvd/AckSent to ReqSent and sends TermAck with the
received Identifier; it does not exit the process. Likewise
[accel-ppp1.14.0 `ppp_fsm_recv_term_req`](https://raw.githubusercontent.com/accel-ppp/accel-ppp/1.14.0/accel-pppd/ppp/ppp_fsm.c)
(lines409–431) sends TermAck and stays/returns ReqSent in all three negotiating
states. A “normal failure” here therefore includes actual subsequent lower-layer
CDN/PADT completion, not an assumed immediate peer exit on TermReq alone.
Both current live transport scenarios use pppd; source evidence for accel is
not runtime accel-client proof. A-2 is now confirmed under this bounded
interpretation: actual source FSMs agree, and both latest-peer pppd transport
scenarios completed their real lower-layer teardown. No extra accel lab is
implied or needed; immediate exit on TermReq alone remains explicitly false.


### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The derived default narrows MS-CHAPv2 deployments that relied on a fallback from CHAP-MD5 | an interop or `.ci` test with an MS-CHAPv2-only client goes red | the operator writes `allow [ chap-md5 ms-chap-v2 ]`; the guide's migration note says so |
| R-2 | A proxied PAP session that used to come up now renegotiates, and the LAC's subscriber sees LCP restart | access-wholesale scenario logs a renegotiation | intended; documented under Behavior to change |
| R-3 | Terminating in LCP hides the reason from accounting | no Accounting-Stop for an exhausted negotiation | today's path also sends none before auth; the log line and a new `ze_ppp_auth_failures_total` counter with reason `auth-method-refused` carry the refusal |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | New PPP sessions fail to authenticate or come up with the wrong MRU; established sessions are untouched |
| How is it reverted? | Single commit revert; the leaves are new and optional |
| Who else touches this path? | `spec-home-cpe-15-lns-lcp-renegotiation.md` (same proxy short-circuit in `session_run.go`), `spec-home-cpe-14-lac-realm-relay.md` (LAC-mode PPPoE sessions use the same allow-list), `plan/post-release/features/spec-l2tp-proxy-authentication.md` (LNS proxy auth consumes the proxied method) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `l2tp { mru 1492 }` config | → | `reactor_kernel.go` StartSession `MaxMRU` → `sendConfigureRequest` | `TestReactorPPPPolicyReloadIsolation` (`internal/component/l2tp/reactor_ppp_linux_test.go`), plus wire request/cap `TestConfiguredMRUNakInformationFloor` |
| `l2tp { authentication { allow [ chap-md5 ] } }` | → | `StartSession.AuthFallbackOrder` → `adjustAuthOnNakOrReject` | `TestReactorPPPPolicyReloadIsolation`, plus real-driver refusal `TestAuthAllowEmptyRefusesPAPNak` |
| PPPoE `authentication-allow` config | → | `server.go` StartSession → driver | `TestPPPoEAuthAllowReachesDriver` (`internal/component/l2tp/pppoe/auth_allow_linux_test.go`) |
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
| AC-15 | `ze show l2tp config`, and `ze show l2tp session id <id>` on an LNS session | `show l2tp config` carries configured `mru` (1500 when absent) and the effective `authentication-allow` list. Session detail carries actual peer receive `mru` and `auth-method`, not configured defaults; unknown/down is explicit null. Unit/snapshot/promoted config PASS. Current source-built2.5.5 native MRU GREEN bg35→semantic RED bg40→fresh normal RESTOREDGREEN bg41 complete, including actual wire1492/Ack/CHAP/IPCP/MTU/traffic/RADIUS and current config MRU1492/effective[chap-md5], session1492/chap-md5. |

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
| `TestResolveAuthAllow` | `internal/component/l2tp/ppp/auth_policy_test.go` | Shared derived defaults, ordered-set validation and negative boundaries | PASS P |
| `TestProviderPPPPolicyBoundaries` | `internal/component/l2tp/config_policy_test.go` | Malformed provider policy/MRU refusal | PASS P |
| `TestConfigPPPPolicy` | `internal/component/l2tp/config_policy_external_test.go` | Original-tree/YANG AC-3/9, valid MRU64/1492/1500 | PASS P |
| `TestAuthAllowEmptyRefusesPAPNak` | `internal/component/l2tp/ppp/auth_allow_policy_test.go` | Real Driver AC-5; observed baseline red before producer fix | PASS P |
| `TestAuthAllowPAPFallbackAndOptionalNoAuth` | `internal/component/l2tp/ppp/rfc1661_proxy_policy_driver_test.go` | AC-6/10: actual PAP Authenticate-Ack/session-up and optional noauth handler/session-up | PASS P; carrier renamed byte-for-byte by Main |
| `TestInitialAuthMethodCHAPBeforePAPWhateverOrder` | `internal/component/l2tp/ppp/rfc1334_auth_test.go` | AC-7/8, safe defaults and written order | PASS P |
| `TestAuthAllowSelectionMatrix` | `internal/component/l2tp/ppp/auth_policy_driver_test.go` | Selection matrix, repeated/outside/unknown Nak, required Reject, actual LCPUp method; explicit RFC1334 PAP-default and PAP-first fallback claims | Behavior PASS P; both new RFC1334-x-1 native records and fresh independent audit-stamp rejudge SUCCEEDED |
| `TestAuthRefusalTerminationCleanup` | `internal/component/l2tp/ppp/auth_policy_driver_test.go` | All negotiating states, no premature LCPUp/auth request, normal TermAck/Restart cleanup | PASS P |
| `TestAuthOfferRetransmitAndLifetime` | `internal/component/l2tp/ppp/auth_policy_driver_test.go` | Retransmissions do not consume alternatives; new lifetime resets policy/correlation | PASS P |
| `TestProxyLCPPolicy` | `internal/component/l2tp/ppp/rfc1661_proxy_policy_driver_test.go` | AC-11/12 proxy admission/renegotiation, unknown/missing-required auth, MRU direction/default | PASS P; combined CHAP completion after refused proxy is covered separately |
| `TestProxyLCPDisallowedPAPAuthenticatesWithCHAP` | `internal/component/l2tp/ppp/proxy_auth_completion_test.go` | AC-11 complete real Driver: disallowed proxy PAP→fresh LCP→CHAP challenge/digest-verified callback acceptance→CHAP Success/SessionUp; actual fresh peer MRU, no PAP callback | Focused native race PASS C; not live proxy interop |
| `TestConfiguredMRUNakInformationFloor` | `internal/component/l2tp/ppp/auth_policy_driver_test.go` | AC-1, Information-field MRU64/65 and configured1492 peer cap | PASS P |
| `TestReceiveFullFrameBelowConfiguredMRU` | `internal/component/l2tp/ppp/rfc1661_proxy_policy_driver_test.go` | AC-4 real1500 receive at requested1492, kernel receive1500 | Behavior PASS P; bg29 renamed-carrier +/− native records SUCCEEDED; current keys bound, old-carrier history not credited |
| `TestFullInformationFieldReceived` | `internal/component/l2tp/ppp/rfc1661_magic_echo_test.go` | Existing positive full1500 receive claim preserved;1501 rejection retained solely as a local product-boundary assertion | Independently approved false negative RFC claim replaced with plain comment only; body/assertions unchanged, not a product fix |
| `TestAuthMethodRefusalCounter` | `internal/component/l2tp/ppp/auth_policy_metrics_test.go` | Real refusal/exporter scrape, bounded reason and exact operator commands | PASS P |
| `TestReactorPPPPolicyReloadIsolation` | `internal/component/l2tp/reactor_ppp_linux_test.go` | L2TP actual Reload→new StartSession policy; old request remains immutable | PASS P |
| `TestReloadPPPPolicyRefusalKeepsEffectiveConfig` | `internal/component/l2tp/config_policy_test.go` | Rejected reload preserves effective config; accepted reload visible | PASS P |
| `TestPPPoEAuthAllowDefaults`, `TestPPPoEAuthAllowProviderShapes`, `TestPPPoEAuthAllowOriginalTreeValidation` | `internal/component/l2tp/pppoe/auth_allow_test.go` | PPPoE defaults, strict input shapes and original-tree refusal | PASS P |
| `TestPPPoEAuthAllowReachesDriver` | `internal/component/l2tp/pppoe/auth_allow_linux_test.go` | Config→handlePADR→production Driver→wire, fixed1492 and generation isolation | PASS P |
| `TestPPPoEAuthAllowReloadIsolation` | `internal/component/l2tp/pppoe/auth_allow_test.go` | AC-14 PPPoE server generation isolation/refused reload | PASS P |
| `TestConfigJSONMRUAndAllowList` | `internal/component/l2tp/cmd/l2tp_test.go` | AC-15 effective config, derived defaults and explicit order | PASS P |
| `TestSessionJSONMRUAndAuthMethod`, `TestSessionSnapshotNegotiatedPPP` | `internal/component/l2tp/cmd/l2tp_test.go`, `internal/component/l2tp/snapshot_test.go` | AC-15 actual reactor event→snapshot→JSON, down/unknown/null/replacement | PASS P |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `l2tp mru` | 64..1500 (the code's Nak floor of 64 and `MaxFrameLen`) | 64 and 1500 | 63 | 1501 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `lns-mru-auth-allow-config` | `test/l2tp/lns-mru-auth-allow-config.ci` | Operator validates range/policy and reads effective defaults from a started daemon | Draft/stress80 PASS; actual promoted case PASS bg34 within L2TP suite27 PASS/2 SKIP; skipped cases did not run |
| `pppoe-auth-allow-config` | `test/pppoe/pppoe-auth-allow-config.ci` | Flat policy validation/refusal and ordered parsed readback | Draft/stress80 PASS; actual promoted case PASS bg34 within PPPoE suite1 PASS/8 SKIP; skipped netns-link cases did not run |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `l2tp-lns-mru-xl2tpd` | `test/interop-l2tp/scenarios/l2tp-lns-mru-xl2tpd/` | xl2tpd1.3.20 LAC, current pppd2.5.5 client | Peer-decoded MRU1492 and CHAP, measured MTU, ping, independent RADIUS, live config/session output | GREEN bg35 → actual wire1500 RED bg40 → fresh normal RESTOREDGREEN PASS1/1 bg41. bg37 setup timeout is non-proof. |
| `l2tp-lns-chap-only-xl2tpd` | `test/interop-l2tp/scenarios/l2tp-lns-chap-only-xl2tpd/` | xl2tpd1.3.20 LAC, current pppd2.5.5 PAP-only client | Correlated CHAP→PAP Nak→TermReq/Ack, no auth/backend request, natural process/session/kernel cleanup | GREEN bg41 → actual broad-default PAP/backend/IPCP RED bg42 → fresh normal RESTOREDGREEN PASS1/1 bg43. |
| `pppoe-chap-only-pap-client` | `test/interop-pppoe/scenarios/pppoe-chap-only-pap-client/` | Current pppd2.5.5 rp-pppoe PAP-only client | One Ze PADT after correlated refusal, no Opened/auth/NCP, daemon/kernel cleanup | GREEN bg43 → actual broad-default PAP/backend/IPCP RED bg44 → fresh normal RESTOREDGREEN PASS1/1 bg45. Red's final pcap probe also timed out30s; actual downgrade independently establishes semantic red. |

Every owned scenario was broken at its producing code and rebuilt before the
same named scenario failed, then restored and rebuilt before PASS. Current
pppd2.5.5 MRU and both transport auth-policy discriminations are complete.
Historical2.5.4 evidence remains historical, never relabeled. The existing
current-peer `pppoe-pap-ze-ac` and `02-ze-ac-pppd-client` scenarios also PASS
bg45, preserving opted-in PAP/replay and CHAP good/bad-credential behavior.

### Observed Verification (2026-11-03/04; not closure)

Log aliases below are under `tmp/session/2026-11-01-174538/scratch/`
unless a complete path is shown. `PASS P` in the unit table means that named
test passed in the product race run, not that the original PPP package run
was green: it failed four old fixtures. Their independently approved
corrections then passed in focused run F; no complete post-correction PPP
package rerun is claimed.

| Evidence | Observed result | Log / record |
|----------|-----------------|--------------|
| P: product package race tests | L2TP/cmd/PPPoE PASS; new PPP policy/matrix/proxy/full1500 tests PASS; four old PPP fixtures FAIL | `job-home-cpe16-product-b60d37f8.log` |
| F: approved four-fixture rerun | `TestRunAuthPhaseRequiredAuthRejectsNoMethod`, `TestConfiguredPAPStillOffersCHAPFirst`, `TestRFC1661NakHandlesDifferentOptionLength`, `TestRFC1661NakTooShortOptionNotDecoded` PASS | `job-home-cpe16-ppp-fixed-6fe5c199.log` |
| C: complete proxy-refusal authentication proof | `TestProxyLCPDisallowedPAPAuthenticatesWithCHAP` PASS in native focused race run; unit-level full LCP/CHAP/backend/SessionUp proof, not live proxy interop | `job-home-cpe16-proxy-chap-completion-1c1b948d.log` |
| L2TP checker/scenario-shape and native tooling race tests | `internal/le/interoplab/l2tp`, `test/interop-l2tp`, `internal/le/test/stressrepro`, `internal/le/test/functional` PASS | `job-home-cpe16-lab-tooling-34b839af.log` |
| PPPoE checker race tests | `internal/le/interoplab/pppoe` PASS; not a live pppd scenario | `job-home-cpe16-pppoe-checker-920f6bc0.log` |
| Config functional drafts | Both actual drafts PASS1/1; each stress run80 invocations, parallel2, burners1, no failures; outer exit1 means failure not reproduced. Promoted unchanged to the functional paths above | `tmp/stress-repro/l2tp-draft-lns-mru-auth-allow-config-20261103-233707.log`; `tmp/stress-repro/pppoe-draft-pppoe-auth-allow-config-20261103-234034.log` |
| Native RFC current bindings and audit stamps | Main reports all16 CURRENT bindings complete after bg29 receive +/− rebinding;18 native observations include two old-carrier historical observations, not18 credited bindings. bg30 fresh rejudge stamps SUCCEEDED for RFC1334 one RID and RFC1661 two RIDs, then38 mechanical SHIFT reseals | Thirteen foreign stale-unit judgments were correctly refused and remain unchanged; this is not a global RFC-gate-clean claim |
| Independent code/document reviews | Implementation/complete-AC11-unit, cleanup, original seven-page wiki and final latest-peer documentation/comment deltas CLEAN. Current policy scenarios all completed semantic red/restored green; current PAP/CHAP compatibility PASS | Final native closure Review Gate and core/wiki/closure commits remain separate active work, not inferred from implementation reviews |
| Historical pppd2.5.4 native discrimination | MRU baseline→wrong-wire1500 red→fresh unmodified restored PASS. Auth broad-default overlay bg31/artifact180 allowed CHAP→MSCHAPv2→PAP Naks, PAP Request/Ack/success and IPCP up with required TermReq absent: ACTUAL semantic RED. bg32 unmodified L2TP CHAP-only restored PASS1/1 | Historical2.5.4 proof remains separate. Current2.5.5 MRU and L2TP auth independently GREEN→semantic RED→RESTOREDGREEN complete |
| Tooling prerequisite commit | Main reports actual commit `b79832ab78` containing five prerequisite files with only owned documentation hunks; correctly attributed to the selected home spec | Earlier misbound prepared script was caught before execution; no protocol-core/closure commit or unrelated spec-marker change claimed here |
| Actual promoted functional suites | Main reports bg34 L2TP27 PASS/2 SKIP (radius-acct-wire and session-stopccn-cascade lacked net-admin); PPPoE1 PASS/8 SKIP (netns-link cases). Both new promoted config cases PASS | AC-3/9/config half AC-15 promoted execution fulfilled; skipped cases did not run |
| Latest-peer MRU discrimination and refusal baselines | Source-built2.5.5 MRU baseline bg35 PASS; bg40 only reactor MaxMRU1500 overlay produced actual wire1500 while CHAP Success/IPCP/session-up/RADIUS still worked; checker RED expected1492 absent. Fresh normal bg41 restored PASS1/1, including config/session output. Current L2TP refusal bg41/bg43 and PPPoE refusal bg43/bg45 PASS | All three current policy GREEN→semantic RED→RESTOREDGREEN sequences complete. No cleanup-warning cause inferred. |
| Current2.5.5 L2TP auth producer discrimination | bg42 broad-default overlay [chap,mschap,pap] built/ran; peer Naks CHAP/MS-v2 toward PAP, Ze offers PAP, PAP AuthReq/AuthAck/auth-success, IPCP10.100.0.2, backend AccessRequest/AccountingStart and PPP session-up; expected TermReq times out because actual downgrade happened | ACTUAL semantic RED, not setup failure. Fresh normal L2TP restoration bg43 PASS1/1; current GREEN bg41→RED bg42→RESTOREDGREEN bg43 complete |
| Current2.5.5 PPPoE semantic red/restoration and compatibility | bg43 refusal baseline PASS. bg44 broad-policy overlay actually negotiated PAP/Ack, PAP AuthReq/AuthAck/success, local backend acceptance and IPCP10.20.0.2; checker reported non-CHAP offer. Final pcap probe additionally timed out30s, not sole red basis. Fresh normal bg45 restored refusal PASS1/1, then existing PAP/replay and02 CHAP good/bad-credential cases each PASS1/1 | Current PPPoE discrimination and both requested compatibility scenarios complete; no source/assertion weakening used for those passes. |
| Earlier MRU red-attempt setup incident | bg37 Docker-info15s timeout before protocol,0 passed/0 failed, remains NOT semantic red. Targeted info recovered bg39; bg40 subsequently completed protocol and produced the distinct semantic wire-value red | BuildKit journal transport messages are not a proven cause of the earlier timeout; no shared Docker stop/restart/prune |
| Final native RFC maintenance/check bg59 | Fresh independent RFC1661-6.1-1 audit stamp SUCCEEDED; native RFC update SUCCEEDED. Final check exit1 reports93 violations, all foreign BGP/IKE/PPPoE-client inputs; owned bindings/audits have no reported finding | `artifact://301` personally read all violation entries. The intermediate187 snapshot is historical; neither count is conformance credit or a global PASS. |

### Acceptance Evidence

| Criteria | Observed coverage | Scope and limits |
|----------|-------------------|------------------|
| AC-1 | Request/cap units PASS P; current2.5.5 native GREEN bg35→semantic wrong-wire-MRU RED bg40→fresh normal RESTOREDGREEN bg41 complete | No MRU discrimination outcome still pending; final applicable gates separate |
| AC-2 | Effective default1500 and normal driver defaults covered by PASS P fixtures | No new live default-MRU claim |
| AC-3 | YANG/original-tree units, draft/stress and actual promoted config case PASS bg34 | No promoted-case run owed; broader skipped privilege-dependent cases not credited |
| AC-4 | Real Driver full1500 receive/kernel1500 PASS P; renamed-carrier +/− native records complete bg29 and fresh stamps/reseals completed bg30 | Not a live kernel peer-payload claim;13 foreign stale judgments remain unchanged outside this spec's proof credit |
| AC-5 | Required-refusal/no-callback/reason/termination units P; current2.5.5 L2TP GREEN bg41→actual broad-default RED bg42→RESTOREDGREEN bg43; PPPoE GREEN bg43→actual broad-policy RED bg44→RESTOREDGREEN bg45 | All requested current transport discrimination complete; red's ancillary pcap-probe timeout remains separately disclosed |
| AC-6/7/8 | PAP success and CHAP-before-PAP matrix P/F; RFC1334 matrix +/− records and fresh judgments; current2.5.5 existing PAP/replay compatibility bg45 PASS | No claim that skipped PPPoE netns-link cases executed |
| AC-9 | Both transports' units, drafts/stress and both actual promoted config cases PASS bg34 | No promoted-case execution owed |
| AC-10 | Real noauth handler/session-up after optional exhaustion PASS P | No extra live noauth claim |
| AC-11 | Proxy policy matrix PASS P; complete refused-PAP→fresh LCP→digest-verified CHAP→Success/SessionUp and fresh negotiated peer MRU PASS C; independent new-unit review CLEAN | No live proxy-interoperability claim |
| AC-12 | Absent Last-Sent MRU defaults1500 and renegotiates at1492; peer/local direction controls PASS P | No live proxy-MRU interop claim |
| AC-13 | Latest2.5.5 PPPoE baseline bg43 and fresh normal restoration bg45 PASS1/1: TermReq/Ack→one PADT, no PAP/NCP and daemon/kernel cleanup. Broad-policy bg44 independently produced actual PAP/backend/IPCP semantic red | All requested discrimination complete; red's final pcap probe also timed out30s, not sole failure evidence |
| AC-14 | L2TP actual Reload→new StartSession isolation and PPPoE server-generation/driver isolation PASS P | No claim of a live established-session reload experiment |
| AC-15 | Event→snapshot→JSON/promoted config PASS; current2.5.5 native actual config1492/effective[chap-md5] and session1492/chap-md5 PASS in baseline and fresh restored run; MRU discrimination complete | No MRU/live-output run outstanding; final applicable gates/landing separate |

### Current Verification Disposition

- All15 ACs now have their planned source/unit/functional/native evidence.
  Current pppd2.5.5 supplies all three policy baseline/red/restored-green
  sequences plus existing opted-in PAP and CHAP good/bad-credential
  compatibility. Historical2.5.4, setup-only failures, ancillary probe failure,
  broader functional skips and unit-only controls remain explicitly bounded.
- Planned16 current native proof bindings and38 mechanical SHIFT reseals
  completed before an intermediate global RFC snapshot reported187 findings,
  including two owned metadata issues. RFC2661's changed config-producer
  audit was independently rejudged and stamped. The approved plain1501
  note moved beside its unchanged assertion, restoring the original positive
  claim. Fresh native bg58 VERIFIED all three current RFC1661-6.1-1 bindings
  with no unproven unit and re-stamped five owned SHIFT audits. Bg59 stamped
  the fresh independent6.1-1 verdict and regenerated native RFC outputs.
  Its final check reported93 foreign findings, none against the owned
  bindings/audits; every violation entry was read in `artifact://301`.
- Thirteen foreign stale-unit audit judgments were refused unchanged. Other
  shared-checkout RFC findings remain foreign. No global RFC-clean claim is
  made, and a count is not a conformance percentage.
- A real unrelated inventory compile failure stopped bg53. Actual later
  source inspection found all three required producers present, including
  `Snapshot.assemblyPredefines`; fresh native bg58 built successfully.
  Main owns core/wiki commits and explicit
  closure clearance. No foreign source edit, retained-binary bypass,
  worktree/export or unapproved process action was used.
- Native core preparation bg60 found a direct tooling prerequisite:
  `weakened.detect` suppressed assertion-loss classification for an empty
  replacement. `weakenedUnits` therefore accounted for a complete deletion
  by its function name but not its assertion-loss kind; the whole-file
  fallback then misattributed that already-accounted loss to the file stem,
  despite `goUnresolvedText` correctly stripping the complete declaration.
  Independent judgment refused blanket approvals for
  `ppp.rfc1334_auth_test`29→28 and `interop-l2tp.parity_test`5→3.
  Bg61's unchanged two-unit regression also exposed a real fail-open:
  an unapproved `TestRetired` deletion while `TestKept` remains returned
  Proposed Notice without consulting the ledger. Main traced whole-file
  blocking selection in `proposed.go` and the write hook's Notice/Block
  consumer. Narrowing the fixture to a sole test was refused. The prerequisite
  includes minimal private per-unit blocking propagation, preserving
  advisory-only Notice and outside/incomplete controls; the original three
  regression fixtures remain unchanged.
  Main's five-file source/test/documentation fix now has independent CLEAN
  source/caller review, including the exact independently approved correction
  to the new regression's retained-finding expectation. Bg62 passed11 of12
  top-level tests; only that fixture-contract mismatch failed. Bg63 reran
  only the corrected top-level test: both subcases PASS, native exit0
  (`job-home-cpe16-deletion-fixed-eaa47beb.log`). Together these complete the
  twelve scoped controls, not a full-package green. Actual read-only CLI
  refusal/survivor smoke returned exit2/exit0. Bg64 core preparation passed
  the old false-weakening barrier, then correctly refused the separately
  selected journal-owner design's missing Existing Mechanisms section.
  That source-backed metadata section is now supplied for Main verification.
  The five-file fix landed as `22bccf6d2d67f58d3fde3a18e0b323c6e3c76257`,
  correctly attributed to this spec. No red is claimed for later extra
  edge controls; no blanket approval, scope waiver or native guard bypass.
- Native RFC/documentation-index updates and wiki LLMS generation ran;
  generated ignored artifacts are not hand-edited or staged. Seven targeted
  wiki pages and the final latest-peer delta have independent clean reviews.
- A-1 remains broken despite passing current PAP compatibility. A-2 is
  confirmed only in its bounded Ack/ReqSent plus actual current pppd
  CDN/PADT completion sense; no immediate TermReq-only process exit or runtime
  accel-client proof is claimed. Status stays in-progress until closure lands.

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
- `internal/component/l2tp/subsystem.go`, `reactor.go` - startup mappings for both controls
- `internal/component/l2tp/register.go`, `pppoe/register.go` - component-owned original-tree validators
- `internal/component/l2tp/ppp/start_session.go`, `ppp/session.go` - explicit policy contract and tried-method state
- `internal/component/l2tp/ppp/events.go`, `session.go`, `reactor_kernel.go` - negotiated MRU and method event publication and retention
- `internal/component/l2tp/snapshot.go`, `subsystem_snapshot.go`, `cmd/l2tp.go` - effective config and negotiated session output
- `internal/component/l2tp/ppp/metrics.go` - register the authentication refusal counter; no such counter exists before this work
- `internal/component/l2tp/ppp/rfc1334_auth_test.go`, `rfc1334_clauses_test.go`, `rfc1661_test.go`, `auth_dispatch_test.go`, `auth_events_test.go` - independently approved explicit-policy/defensive-guard fixture migrations.
- `internal/component/l2tp/ppp/rfc1661_magic_echo_test.go` - independently approved correction of false RFC1501 negative metadata; preserve positive claim and every assertion.
- `test/interop-l2tp/Dockerfile.lac`, `test/interop-pppoe/Dockerfile.client` - current official ppp2.5.5 API archive/SHA256/autogen prerequisites; no historical2.5.4 runtime proof relabeling.
- `docs/labs/l2tp-interop.md`, `docs/labs/pppoe-interop.md` - latest official2.5.5 peer provenance/build instructions, historical2.5.4 evidence clearly separated from current runtime verification.
- `test/interop-l2tp/parity_test.go` - independently approved obsolete `TestNativeConfigBytesArePinned` deletion applied; `TestL2TPTreeIsGoOnly` unchanged, no new shadow hash pins.
- `internal/le/test/qemu/netns_linux.go` - removed obsolete PPPoE registry exception/alternate command path and stale fallback comments after prerequisite suite registration; `runNetnsSuite` uses registered suite arguments, independent review CLEAN.
- `internal/le/interoplab/l2tp/`, `internal/le/interoplab/pppoe/` - typed scenario checkers and their registration
- `docs/guide/l2tp.md`, `docs/guide/pppoe.md` - leaves, behaviour, reload table rows
- `features/l2tpv2-bng.md`, `features/pppoe-access.md` - scope
- `docs/architecture/l2tp/bng-5-pppoe.md` (declared by `pppoe/config.go`, `pppoe/server.go`, `pppoe/subsystem_reload.go`) - nothing becomes wrong: its reload paragraph ("A reload applies the diff live") already covers a new leaf parsed by the same `ExtractParameters`; the allow-list is documented in `docs/guide/pppoe.md`
- `docs/research/l2tpv2-ze-integration.md` (declared by `config.go`, `reactor_kernel.go`, `ppp/manager.go`, `ppp/auth.go`, `ppp/session_run.go`) - its Section5 schema sketch uses historical `ppp-max-mtu`/1420; the shipped leaf is `mru`, YANG/runtime default1500, distinguished by a source-anchored note.
- `docs/research/l2tpv2-implementation-guide.md` (declared by `ppp/proxy.go`) - nothing becomes wrong: its Section 18 describes the RFC's proxy AVPs, not Ze's policy on them
- `internal/component/l2tp/reactor_setters.go`, `subsystem_reload.go` declare `docs/guide/l2tp.md`, already listed above

## Files to Create
- `internal/component/l2tp/ppp/auth_policy_driver_test.go` - mixed RFC1661/RFC1334 matrix, termination/lifetime and MRU-floor consumer tests; neutral filename intentional.
- `internal/component/l2tp/ppp/rfc1661_proxy_policy_driver_test.go` - current byte-pure-renamed RFC1661 carrier; both receive proof bindings completed under new keys by parent.
- `internal/component/l2tp/ppp/proxy_auth_completion_test.go`, `auth_policy_metrics_test.go` - complete AC11 cryptographic authentication path and refusal/exporter proof.
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
| Original-tree validators | Yes | each transport registers with `config.RegisterTreeValidator`; the shared policy check also runs during extraction and reload. A per-member `ze:validate` receives no sibling context |
| CLI commands/flags | No | config only |
| CLI grammar (keyword before value) | N-A | no command |
| Editor autocomplete | Yes | automatic from the enumeration |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | no output |
| Env var registration | N-A | not under `environment` |
| Doctor check for runtime dependencies | N-A | no new runtime dependency |
| Prometheus counters/metrics | Yes | add `ze_ppp_auth_failures_total` with reason `auth-method-refused` through the existing PPP metrics hook |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/l2tpv2-bng.md`, `features/pppoe-access.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/l2tp.md`, `docs/guide/pppoe.md` (configuration sections) |
| 3 | CLI command added/changed? | Yes, output only | Existing `show l2tp config` and session commands report effective policy and measured negotiation in `docs/guide/l2tp.md`; no new grammar/flag/completion |
| 4 | API/RPC added/changed? | Yes, output fields only | Same handlers expose MRU/method and explicit nulls through existing command dispatch; L2TP guide documents their meaning, no new RPC |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/guide/l2tp.md` (PPP negotiation, Authentication, reload table), `docs/guide/pppoe.md` |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc1661.md` (Section 6.2 and 3.5 rows' annotations), `rfc/short/rfc1334.md` (Section 2 row) |
| 10 | Test infrastructure changed? | Yes | Existing PPPoE suite registry/stress admission and native policy checkers/images documented in `docs/architecture/testing/ci-format.md`, `docs/functional-tests.md` and both lab guides |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | Yes | the PPP metrics section of `docs/guide/l2tp.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No new registration | Existing EventLCPUp payload gains actual auth method; both normal/proxy producers and negotiated-output guide are updated |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derived with `./le spec citation anchors spec plan/release/fixes/spec-home-cpe-16-lns-mru-and-auth-allow-list.md` at implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the `l2tp {}` example in `docs/guide/l2tp.md` "Configuration" |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- YANG leaves, `Parameters` fields,
   `StartSession` assignment in both transports, failing wiring tests.
   - Tests: `TestReactorPPPPolicyReloadIsolation`,
     `TestPPPoEAuthAllowReachesDriver`,
     `TestAuthAllowEmptyRefusesPAPNak`
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
| `mru` leaf reaches the wire | Current `l2tp-lns-mru-xl2tpd` actual1492/Ack PASS; rebuilt wrong-MaxMRU1500 semantic RED; fresh normal restoration PASS |
| CHAP-only refuses PAP on both transports | Both current native `chap-only` scenarios completed GREEN→actual broad-default downgrade RED→RESTOREDGREEN; Driver refusal/termination units P; opted-in PAP/CHAP compatibility bg45 PASS |

### Security Review Checklist
| Check | What to look for | Source and evidence |
|-------|------------------|---------------------|
| Downgrade | No Nak, Reject or proxy AVP can move a session to a method outside the list | Read `selectAuthFallback`, validated `handleLCPPacket` mutation and `acceptProxyPolicy`; P/C plus both current negative native scenarios and actual broad-default red controls |
| Fail-closed | Unreadable explicit policy refuses config rather than widening defaults | Read both strict transport decoders and `ResolveAuthAllow`; provider/original-tree negatives P and both promoted cases PASS |

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

No unresolved journal row is owned by this spec. The source-backed
`le rfc discriminate report` help/input mismatch discovered during verification
is separately owned by
`plan/release/validation/spec-rfc-evidence-strength-1-targeted-mutant-ratchet.md`
in `plan/journal/help-text-describes-superseded-behaviour.md` (2026-11-04).
Its owner spec lists the row; it is not omitted home-CPE implementation scope.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Derived default when no list: `chap-md5` → [chap-md5]; `ms-chap-v2` → [ms-chap-v2]; `pap` → [chap-md5, pap]; `none` → [] | Keep today's [chap-md5, ms-chap-v2, pap] default and only add the list | The YANG description promises "The default requires CHAP-MD5"; a default that accepts PAP is the defect. `pap` keeps CHAP-MD5 first because RFC 1334 Section 2 says an implementation with a stronger method "MUST offer to negotiate that method prior to PAP" and the guide documents that behaviour |
| Exhaustion with authentication required closes LCP before Opened | Today's drop-the-option, open, then fail in `runAuthPhase` | RFC 1661 Section 3.5: an implementation that desires authentication "MUST request the use of that authentication protocol during Link Establishment phase"; opening without it contradicts that. accel-ppp fails LCP the same way (`auth_recv_conf_nak`) |
| A proxied method outside the list, or a Last-Sent MRU above `mru`, makes the proxy LCP unusable and Ze negotiates LCP itself | Accept the proxy and refuse the session | The existing fallback path in `session_run.go` already runs full LCP when `EvaluateProxyLCP` fails; renegotiation (RFC 2661 Section 4.4.5: "the LNS policy may be to enter an additional round of LCP negotiation") lets the session succeed with an allowed method |
| One `mru` leaf drives both the requested MRU and the cap on the peer's MRU | Separate `mru` and `max-peer-mru` leaves | `LCPNegPolicy.MaxMRU` already couples them, accel-ppp caps both with one `max_mtu`, and no case in the umbrella needs asymmetry (A-4) |
| No `mru` leaf on the PPPoE AC | Add one capped at 1492 | The AC already requests 1492 (`PPPoEMaxMTU`), which RFC 2516 makes the maximum; lowering it is not needed by any child |
| CHAP variants offered before PAP whatever order the list is written in | Honour the written order literally | RFC 1334 Section 2 MUST |
| Configure-Reject never retries another Auth-Protocol in the same negotiation | Treat Reject like Nak and offer another method | RFC 1661 Section 5.4 forbids including the rejected option in a later Configure-Request; the matrix uses Naks to test ordered fallback |
| Cross-leaf validation registers at the original-tree boundary | Per-member `ze:validate` | `applyCustomValidators` receives only a path and one value; `RegisterTreeValidator` supplies sibling context and runs for both transport roots |
| PPPoE uses `authentication-allow` beside its existing flat auth leaves | Add a one-leaf authentication container | Extends the current schema layout without moving existing leaves |

The audit also found that `MinFrameLen` includes the two-octet Protocol field,
while the MRU leaf and Nak carry Information-field octets. Driver validation
must accept MRUs 64 and 65 without comparing them with the 66-octet frame floor.
Provider-delivered allow-lists must refuse malformed shapes or non-string
members rather than use `configvalue.LeafList`'s lossy coercion.

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

### Goal Gates (MUST pass)
- [ ] Every AC-N in the Acceptance Criteria table demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass: scoped package tests for the reached L2TP, PPP and PPPoE packages, the affected L2TP and PPPoE functional cases, and the three named interop scenarios. No agent runs `./le verify`; GitHub CI supplies whole-tree release evidence.
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
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-home-cpe-16-lns-mru-and-auth-allow-list.md` only, in the same `./le commit create` script

---

## Implementation Summary

### What Was Implemented

| Requirement | Producing source | Result |
|-------------|------------------|--------|
| Configurable LNS MRU | `config.go` `ExtractParameters`, `config_policy.go` `parseMRU`, `reactor_kernel.go` `handleKernelSuccess`, `ppp/session_run.go` `sendConfigureRequest` | L2TP requests the configured Information-field MRU, defaults to 1500 and Naks peer values above the session cap. Receive capacity remains 1500. |
| Explicit authentication policy | `ppp/auth_policy.go` `ResolveAuthAllow`, `ppp/auth.go` `initialAuthMethod`, `selectAuthFallback`, `refuseAuthMethod`, `acceptProxyPolicy` | Both transports derive least-permissive defaults, retain explicit permitted methods, offer CHAP variants before PAP, and terminate required-auth exhaustion before Opened. Proxy admission uses the same policy. |
| New-session reload and measured output | `reactor_setters.go` `setPPPPolicy`, `pppoe/subsystem_reload.go` `applyInterfaceConfig`, `reactor_kernel.go` `handleLCPUp`/`handleLCPDown`, `snapshot.go` `sessionSnapshot`, `cmd/l2tp.go` `handleConfig`/`sessionJSON` | Reload publishes a new immutable generation. Config output reports effective policy; session output reports actual negotiated values, with explicit null while unknown/down. |

Paths in this table are under `internal/component/l2tp/`. The independent
closure assessment covers the prerequisite commit, the complete core census,
selected shared-document hunks and the citation edits; final native Review Gate
and landing records remain active closure work, not an implementation claim.

### Bugs Found/Fixed

| Defect | Source fix | Regression evidence |
|--------|------------|---------------------|
| An implicit broad default admitted PAP under CHAP-only configuration | `ppp/manager.go` `spawnSession` no longer inserts a package default; transports pass `ResolveAuthAllow` output | Baseline `TestAuthAllowEmptyRefusesPAPNak` was red before the fix; P passed. Current live L2TP and PPPoE refusal scenarios each passed, failed with the rebuilt broad-default producer, then passed after restoration. |
| Required-auth exhaustion could reach Opened without Auth-Protocol | `ppp/session_run.go` `handleLCPPacket` intercepts exhaustion before SCR; `ppp/auth.go` `refuseAuthMethod` drives ordinary Close/Terminate-Ack/Restart handling | `TestAuthRefusalTerminationCleanup` P, including all negotiating states, no auth/LCP-up event and termination completion. |
| Proxy LCP could bypass authentication policy or the requested local MRU | `ppp/auth.go` `acceptProxyPolicy`; `ppp/proxy.go` `EvaluateProxyLCP` retains Last-Sent MRU separately | `TestProxyLCPPolicy` P; full rejected-proxy-PAP to digest-verified CHAP Success/SessionUp C. |
| The Information-field MRU floor was confused with total PPP frame length | `ppp/frame.go` `MinMRU`; `ppp/session_run.go` validated Nak bounds | `TestConfiguredMRUNakInformationFloor` P covers 64, 65, 1492 and 1500; `TestReceiveFullFrameBelowConfiguredMRU` P retains complete 1500 receive and kernel MRU. |
| Negotiated session output had no event-backed MRU/method | `reactor_kernel.go` `handleLCPUp` and `handleLCPDown`, then snapshot/JSON consumers | Observed output baseline red; `TestSessionSnapshotNegotiatedPPP` and both JSON units P; current native MRU scenario reads live config and the specific live session. |
| PPPoE drafts could not enter stress-repro through the suite registry | `internal/le/test/functional/suites.go` adds the existing non-gating runner | Prerequisite `b79832ab78`; actual draft 80/80 under load and native tooling race tests passed. No gating population or kernel requirement was relaxed. |

### Documentation Updates

| Surface | Change and source |
|---------|-------------------|
| `docs/guide/l2tp.md` | MRU/defaults/receive capacity, auth selection and proxy admission, reload, measured output and refusal counter. Anchors name `ResolveAuthAllow`, `initialAuthMethod`, `selectAuthFallback`, `refuseAuthMethod`, `acceptProxyPolicy`, `handleLCPUp`, `handleLCPDown`, `sessionSnapshot` and `initPPPMetrics`. |
| `docs/guide/pppoe.md`, selected `docs/guide/configuration.md` hunks | Flat `authentication-allow`, safe defaults, explicit PAP migration and new-session-only reload; fixed PPPoE1492 retained. Anchors name each transport's `ExtractParameters` and `applyInterfaceConfig`. |
| `docs/labs/l2tp-interop.md`, `docs/labs/pppoe-interop.md` | Three named native scenarios, checker assertions, current official pppd2.5.5 provenance and dated current observations; historical peer evidence remains distinct. |
| `docs/architecture/l2tp/bng-1-radius-attributes.md` | Ordinary Closed/Stopped remains User Request; authentication-policy refusal reports NAS Request. The source anchor names `applyTransition`; no pre-auth Accounting-Stop is implied. |
| `docs/research/l2tpv2-ze-integration.md` | Historical schema sketch distinguished from shipped `mru` and authentication policy; source anchors name `validateAuthPolicy`, `providerAuthAllow` and `ResolveAuthAllow`. |
| `features/l2tpv2-bng.md`, `features/pppoe-access.md`, selected `ai/INDEX.md` hunks | Feature scope and discovery name the controls without claiming complete RFC support or raising the existing experimental level. |
| RFC summaries and native records | RFC1334 ordering and RFC1661 selection/termination/receive notes, verified discrimination records and independently judged audit metadata; RFC2661 configurable-retransmit audit rejudged after its cited config producer changed. |
| Test-infrastructure pages | Original prerequisite `docs/architecture/testing/ci-format.md` and `docs/functional-tests.md` document registry-based PPPoE stress admission. `docs/architecture/testing/test-health.md` documents exact deletion attribution, per-unit blocking and existing advisory/commit distinction, anchored to `weakenedUnits` and `Proposed`/`proposedFindings`. |
| Wiki | Seven targeted pages updated; independent original reviews and the final three-page latest-peer delta were clean. Native LLMS generation reported 184 pages. Main owns the wiki commit. |
| Derived outputs/check scope | Main ran native RFC and documentation-index updates. Outputs are ignored generated artifacts, not hand-edited or staged. Whole-tree docs/repo/build checks are excluded by this assignment's scoped rule. |

### Deviations from Plan

- A-1 was broken: old fixtures and the deleted `TestDefaultAuthFallbackOrder`
  depended on the implicit broad policy. Independently approved explicit-policy
  corrections and the production-selector replacement preserve meaningful
  assertions. P's four legacy failures were corrected and passed in focused F;
  no full post-correction PPP package rerun is claimed.
- Current official pppd2.5.5 replaced historical2.5.4 peers after the old source
  download failed before protocol. Both images pin the official archive SHA256
  and use upstream autotools bootstrap. Every owned current policy scenario
  completed its own rebuilt producer red and restored green.
- Proxy refusal's initial offer-only fixture did not by itself prove AC-11
  authentication completion. The additional real-Driver C test verifies the
  CHAP digest, callback decision, correlated Success and SessionUp.
- False RFC credit for rejecting1501 was removed only under independent
  judgment; every input/assertion remains. Its plain product-boundary note is
  beside that assertion, outside the retained positive claim paragraph.
- The obsolete whole-file peer hash-freeze unit was deleted under independent
  judgment, not repinned or replaced by a shadow source-hash test.
- PPPoE suite registration was a separate prerequisite; obsolete qemu
  registry exception/alternate dispatch was removed after the registry cutover.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-1 claimed no existing test relied on implicit PAP fallback | Legacy fixtures and the old default-helper contract did rely on it | P's actual four failures and source review of `TestDefaultAuthFallbackOrder` | Independent JudgeTransportFixtures approval; precise fixture corrections/replacement; focused F PASS. Later PAP interop does not make A-1 true. |
| approach | An MRU negative attempt failed during Docker-info setup, and an old peer download failed before protocol | Neither was a semantic red | Native setup results reported no exercised protocol | Kept both as non-proof; the later current-peer rebuilt wrong-wire/broad-policy runs supplied actual semantic reds. |
| approach | A plain1501 product note immediately followed the retained RFC positive tag | `internal/le/rfc/tags.go` `extendClaim` treats contiguous comment prose as one claim | Intermediate RFC check reported the legacy positive claim changed | Independent approval moved only the plain note beside its unchanged assertion, restoring the original positive paragraph. Fresh bg58 VERIFIED all three current6.1-1 bindings, unproven empty; bg59 stamped the fresh independent audit and final check reports no owned finding. |
| approach | Final review found stale current-peer and termination-cause prose | Current pppd2.5.5 proof had completed; policy refusal uses NAS Request, not generic User Request | Independent source/doc review of Dockerfiles, checkers and `applyTransition` | Main corrected the existing lab/wiki pages, source comments and accounting architecture table; independent delta reviews clean. No new product defect or lesson-only journal row invented. |
| approach | Main treated LSP NoDefinition and no watcher event as proof that the inventory field remained absent after a real compile failure | Negative navigation/event results do not establish source absence; `internal/le/repo/inventory/source.go` contains `Snapshot.assemblyPredefines` | Direct source inspection, followed by successful fresh native bg58 build | Corrected the blocker account and owner chat; reported the navigation mismatch. No foreign edit, retained binary, new product bug or extra review round. |
| approach | Native core preparation attributed already-accounted complete-function assertion deletions to file-stem keys | `internal/le/test/weakened/detector.go` `detect` suppressed assertion-loss kind when replacement was empty; `scope.go` `weakenedUnits` then lacked that named kind despite exact residual stripping | bg60 refusal and independent source/diff review; blanket approvals refused | Direct five-file prerequisite landed `22bccf6d2d` after CLEAN independent review, twelve scoped race controls, actual CLI refusal/survivor and bg64 preparation passing that barrier. Separate journal-owner design metadata repaired without changing its Task/AC/Status. |
| approach | New approved-deletion regression expected approved findings to disappear from `Check.Result` | `Check` retains every finding; matching approval affects Problems/trailers, not the audit population | Bg62's two approved-deletion subcases failed while11 other top-level controls passed; source `Check` assignment and approval loop confirmed the contract | Independent judgment approved Exit0/Problems0 plus exactly one retained `TestRetired` finding, preserving sole trailer and all Proposed checks. Actual correction read; no production change to hide approved findings. |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Configurable LNS MRU | Done | `internal/component/l2tp/config.go` `ExtractParameters` → `reactor_kernel.go` `handleKernelSuccess` → `ppp/session_run.go` `sendConfigureRequest` | Current pppd2.5.5 wire1492 baseline/red1500/restored1492; peer-cap and receive-capacity unit controls. |
| Authentication allow-list including proxy LCP | Done | `internal/component/l2tp/ppp/auth_policy.go` `ResolveAuthAllow`; `auth.go` policy producers; `session_run.go` dispatcher | Both transports wired; default refusal, explicit PAP and CHAP-before-PAP proofs; complete AC-11 unit. |
| PPPoE retains1492 and shares policy | Done | `internal/component/l2tp/pppoe/server.go` `handlePADR`, `subsystem_reload.go` `applyInterfaceConfig` | Actual CHAP-only refusal and current PAP/CHAP compatibility, plus config/reload units. |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestConfiguredMRUNakInformationFloor` P; `l2tp-lns-mru-xl2tpd` current baseline bg35, wrong-wire red bg40, restored bg41 | Actual request1492 and peer1500 Nak cap are distinct assertions. |
| AC-2 | Done | `TestConfigJSONMRUAndAllowList` P; normal default request fixtures and `spawnSession` zero-MRU fallback | Default1500 source and unit evidence, not a new live default-MRU claim. |
| AC-3 | Done | `TestConfigPPPPolicy`, `TestProviderPPPPolicyBoundaries` P; promoted `lns-mru-auth-allow-config` bg34 | Refuses63/1501 with leaf/value/range; valid64/1500 controls. |
| AC-4 | Done | `TestReceiveFullFrameBelowConfiguredMRU` P and its current native +/− records | Real Driver receives/answers complete1500 at requested1492; kernel receive1500. Not a new live peer payload experiment. |
| AC-5 | Done | `TestAuthAllowEmptyRefusesPAPNak`, `TestAuthRefusalTerminationCleanup`, `TestAuthMethodRefusalCounter` P; both current live refusal scenarios | No PAP/auth callback/LCP-up before required refusal; both producer discriminations completed. |
| AC-6 | Done | `TestAuthAllowPAPFallbackAndOptionalNoAuth` P; current `pppoe-pap-ze-ac` bg45 | Explicitly permitted PAP reaches Authenticate-Ack and authenticated session; positive compatibility remains distinct from CHAP-only refusal. |
| AC-7 | Done | `TestConfiguredPAPStillOffersCHAPFirst` F, matrix P, current PAP compatibility bg45 | PAP default offers CHAP first, then permitted PAP after Nak. |
| AC-8 | Done | `TestInitialAuthMethodCHAPBeforePAPWhateverOrder`, `TestAuthAllowSelectionMatrix` P; new RFC1334 matrix +/− records | PAP-first written order cannot skip an untried permitted CHAP variant. |
| AC-9 | Done | Both original-tree/config boundary test groups P; both promoted config cases bg34 | Missing preferred method, `none`, no-auth/list conflict and malformed provider forms refuse. |
| AC-10 | Done | `TestAuthAllowPAPFallbackAndOptionalNoAuth` P and optional matrix controls | Actual no-wire-auth callback and SessionUp after explicit opt-in; no extra live noauth claim. |
| AC-11 | Done | `TestProxyLCPPolicy` P; `TestProxyLCPDisallowedPAPAuthenticatesWithCHAP` C | Actual fresh LCP/CHAP challenge/digest-verified backend acceptance/Success/SessionUp, not live proxy interop. |
| AC-12 | Done | `TestProxyLCPPolicy` P | Absent Last-Sent MRU means1500 and forces normal1492 request; local/peer MRU direction controls retained. |
| AC-13 | Done | `pppoe-chap-only-pap-client`: current baseline bg43, actual broad-policy red bg44, fresh normal restored bg45 | Exactly one Ze PADT and daemon/kernel cleanup; no PAP/NCP. Red's ancillary final pcap-probe timeout is disclosed, not its sole semantic failure. |
| AC-14 | Done | `TestReactorPPPPolicyReloadIsolation`, `TestReloadPPPPolicyRefusalKeepsEffectiveConfig`, `TestPPPoEAuthAllowReloadIsolation`, `TestPPPoEAuthAllowReachesDriver` P | Immutable old generation and new-session policy verified through actual reload/driver boundaries; no separate live established-session reload experiment claimed. |
| AC-15 | Done | Snapshot/JSON units P, promoted config bg34, current live MRU baseline/restoration bg35/bg41 | Actual config1492/effectiveCHAP and specific live session1492/CHAP; unknown/down null and measured-vs-configured controls are unit evidence. |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| All22 Unit Tests table rows | Done | Named canonical files in the Unit Tests table above | P named successes, F approved four-fixture correction, C complete proxy authentication. P's PPP package overall failed those four old fixtures; it is not relabeled green. |
| Both Functional Tests table rows | Done | `test/l2tp/lns-mru-auth-allow-config.ci`, `test/pppoe/pppoe-auth-allow-config.ci` | Actual drafts and80/80 under load; promoted cases passed bg34. L2TP27 PASS/2 SKIP and PPPoE1 PASS/8 SKIP; skipped privilege/netns cases did not execute. |
| All three Interop Tests table rows | Done | The three named canonical scenario directories above | All current pppd2.5.5 baselines and rebuilt producer reds/restored greens completed. Current PAP compatibility and existing02 CHAP-success/wrong-password scenario also passed bg45. |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| L2TP `yang/ze-l2tp-conf.yang` | Done | MRU and ordered authentication/allow, existing auth vocabulary. |
| PPPoE `yang/ze-pppoe-conf.yang` | Done | Flat ordered authentication-allow; no PPPoE MRU leaf. |
| L2TP `config.go`, `config_policy.go`, `register.go` | Done | Defaults, strict provider boundary and original-tree registration. |
| PPPoE `config.go`, `register.go` | Done | Same shared resolution and strict provider extraction. |
| L2TP `reactor.go`, `reactor_kernel.go`, `reactor_setters.go` | Done | Atomic new-session policy snapshot and negotiated event handling. |
| L2TP `subsystem.go`, `subsystem_reload.go`, `subsystem_snapshot.go` | Done | Startup, reload and detached effective snapshot. |
| PPPoE `server.go`, `subsystem_reload.go` | Done | Explicit StartSession policy and cloned server generation. |
| PPP `auth_policy.go`, `auth.go`, `manager.go`, `session.go`, `session_run.go`, `start_session.go` | Done | Shared enforcement, tried mask, ordinary termination and explicit contract; obsolete hidden default deleted. |
| PPP `events.go`, `proxy.go`, `frame.go`, `lcp_options.go`, `metrics.go` | Done | Actual method event, local proxy MRU, Information-field floor and refusal counter. |
| L2TP `session.go`, `snapshot.go`, `cmd/l2tp.go` | Done | Event-backed negotiated values and null output. |
| Config, transport/reload and output tests | Done | Named `config_policy*`, `auth_allow*`, `reactor_ppp_linux_test.go`, `subsystem_reload_test.go`, `snapshot_test.go`, `cmd/l2tp_test.go`. |
| New PPP policy/metrics/proxy tests | Done | `auth_policy*`, `auth_allow_policy_test.go`, `rfc1661_proxy_policy_driver_test.go`, `proxy_auth_completion_test.go`; current single-RFC carrier replaces its neutral old filename. |
| Existing PPP fixture/metadata migrations | Changed | Independent approvals cover the named auth/RFC test deltas; assertions preserved or deliberately replaced only for the obsolete policy contract. |
| Typed L2TP/PPPoE checkers, tests and registries | Done | `internal/le/interoplab/l2tp/lns_policy*`, `l2tp.go`; `pppoe/check_auth_allow*`, `checkers_l2tp.go`. |
| Three scenario directories | Done | Canonical fixture directories from Files to Create; independent peers, valid credentials/pools and real teardown. |
| Both promoted `.ci` files | Done | Exact canonical paths above; no mirror or weaker replacement. |
| Existing guides, labs, research, features and RFC records | Done | Documentation Updates table and the final core census identify the complete population. |
| Peer Dockerfiles, parity deletion, qemu registry cleanup and tooling prerequisite | Changed | Official latest peer build; approved obsolete hash-freeze deletion; one existing suite registry/runner rather than a special path. Scoped final tooling tests PASS. |

Paths without a repository prefix in this table are under
`internal/component/l2tp/`. Main's final core census and the native Review Gate
carry the exact individual paths, including the RFC audit/discrimination files.

### Audit Summary

- **Total items:**3 task requirements,15 acceptance criteria,27 planned test
  rows (22 unit,2 functional,3 interop) and18 planned-file groups.
- **Done:** all3 requirements,15 ACs and27 test rows;16 file groups as planned.
- **Partial:**0.
- **Skipped:**0 in-scope deliverables. Broader suite skips are disclosed above.
- **Changed:**2 file groups, with their reasons in Deviations from Plan.

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| An operator's LNS MRU reaches real PPP negotiation without reducing1500 receive capacity | Native interop plus Driver boundary | Current `l2tp-lns-mru-xl2tpd` actual1492/Ack/CHAP/IPCP/MTU1492/traffic/RADIUS/live output; rebuilt wrong-MaxMRU producer emitted1500 and failed; normal rebuild restored PASS. Full1500 Driver receive/kernel control passed P with native +/− records. |
| A CHAP-only LNS refuses PAP downgrade, including policy-unusable proxy LCP | Negative native interop plus complete Driver authentication | Current `l2tp-lns-chap-only-xl2tpd` TermReq/Ack→natural CDN/pppd exit/no backend request/empty kernel state; broad default actually admitted PAP/backend/IPCP and failed; normal restoration PASS. C separately proves refused proxy PAP can complete fresh digest-verified CHAP authentication. |
| PPPoE retains1492 and honors the same explicit policy without breaking opted-in PAP or CHAP credentials | Negative and positive native interop | Current `pppoe-chap-only-pap-client` baseline→actual broad-policy PAP/backend/IPCP red→fresh normal restored PASS. Current `pppoe-pap-ze-ac` and `02-ze-ac-pppd-client` passed, including PAP replay and good/bad CHAP credentials. |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None of the agreed implementation scope was omitted or deferred | All15 ACs and three task goals are implemented with their named proof. Existing Known Limitations remain outside this spec's agreed scope, not newly invented follow-up work. Final native Review Gate/landing work is still active until recorded below. | None |

## Review Gate

| Field | Value |
|-------|-------|
| Scope declared for the independent closure pass | Current156-path population:133 core paths, five original prerequisite paths from `b79832ab78`, five direct weakening-prerequisite paths from `22bccf6d2d`, four citer paths and two shared-document paths, plus seven wiki pages. Selected spec is already among133. Shared-document pins will use actual committed blobs, not foreign working hunks. Later deltas are read against producers; product AC scope is unchanged. |
| Artifact | Not recorded yet; Main has not cleared native closure commands or completed core/wiki landing. |
| `./le spec review check` | Not run yet. No CLEAN machine artifact is claimed. |
| Direct tooling prerequisite | Landed `22bccf6d2d67f58d3fde3a18e0b323c6e3c76257`: independent source/caller review CLEAN, exact fixture correction and direct documentation anchor read. Twelve scoped controls PASS across bg62/bg63; actual CLI refusal/survivor exit2/0; bg64 preparation passed the former weakening barrier before a separate owner-design metadata refusal. No full-package/whole-tree pass claimed. |
| Rounds | One independent inline closure pass, with logic/wiring, security/edge-cases and protocol/evidence/style lenses; final current-hash recording still owed. |
| Source/style review | Read the full Go style. New registration panics are reachable only from invalid/duplicate constant init registration, before listeners. Peer-driven enums decode to known methods or refuse; loops are bounded by three permitted methods, packet length, fixed retry counts or checker deadlines. No new product goroutine or runtime dependency. Immutable generation ownership is explicit. |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | Complete-function deletion could return edit Notice without consulting approvals while another function remained | `proposed.go` `Proposed`; `scope.go` `weakenedUnits` | Private blocking propagation reaches exact-unit gate. Compensated deletion/skip inputs preserved; source review CLEAN, twelve scoped controls PASS and actual CLI refusal/survivor exit2/0. Landed `22bccf6d2d`. |
| 2 | ISSUE | Approved complete deletion also demanded a duplicate file-stem assertion-loss approval | `detector.go` `detect`, then `weakenedUnits` named-kind fallback | Empty-replacement suppression removed; genuine outside/incomplete residuals retain separate stem. Real Proposed/Check controls PASS; bg64 preparation passed the old false gate. Landed `22bccf6d2d`, independent source review CLEAN. |

### Review notes

| Note | Disposition |
|------|-------------|
| Current-peer pending prose and pre-Opened termination-cause table became stale | Main corrected existing pages; independently reviewed clean against Dockerfiles/checkers and `applyTransition`. |
| Intermediate RFC check exposed owned metadata maintenance | RFC2661 config-producer verdict independently rejudged/stamped; approved plain-comment locality restored the legacy RFC1661 positive claim. Fresh bg58 VERIFIED all three current6.1-1 bindings and re-stamped five owned SHIFTs; bg59 stamped the fresh6.1-1 judgment and regenerated outputs. Final check reports no owned finding. |
| Unrelated shared-checkout reds | Final bg59 check exit1 reported93 foreign BGP/IKE/PPPoE-client findings, all entries read in `artifact://301`. Intermediate187 snapshot is historical. No global-clean claim, conformance percentage or foreign judgment rewrite. |
| Separately owned native help defect | `actions.go` describes `rfc discriminate report` as output, but `discriminationStatusOf` passes it to `proposeBreaks`/`loadGomuReport` as an existing input. The observed new-path read refusal matches source. One journal row is owned by the existing targeted-mutant-ratchet spec Task4; no new home-CPE scope, duplicate row or in-scope Work Not Done. |

## Security Review

| Surface | Source-based assessment | Evidence |
|---------|-------------------------|----------|
| Authentication downgrade / fail-closed | `ResolveAuthAllow` rejects unknown names, none conflicts and a missing configured method; strict transport decoders refuse malformed explicit policy. `selectAuthFallback` never selects an unlisted method; required exhaustion closes before publication. | Config original-tree/provider negatives, refusal matrix, both actual negative scenarios and rebuilt broad-default reds. |
| Proxy input | `EvaluateProxyLCP` parses bounded LCP option streams; `HasAuth` distinguishes absent from unknown/short auth. `acceptProxyPolicy` refuses unknown/unlisted auth and inadmissible local MRU, then normal negotiation runs. | Proxy matrix and complete C CHAP authentication with a fresh peer MRU distinct from the proxy. |
| Bounds and denial of service | Tried methods use a uint8 bitset over three methods; retransmission does not consume alternatives. Existing frame buffers remain1502 and restart/termination timers remain bounded. New config allocations occur at the trusted config boundary, not per data packet. | `TestAuthOfferRetransmitAndLifetime`, termination Ack/timeout controls, full1500 receive control; source `selectAuthFallback`, `sendConfigureRequest`, `applyTransition`. |
| Concurrency and ownership | L2TP snapshots/publishes under `tunnelsMu`; PPPoE clones the policy for each server generation under existing dispatch/reload ownership. Sessions retain an immutable slice; output snapshots detach values. | Both transport reload-isolation groups and measured snapshot controls P; scoped race evidence above. |
| Injection, traversal, privileges and secrets | New product input is numeric MRU or a closed auth enumeration; no shell/path operation, privilege grant or credential algorithm was added. Refusal logs contain protocol suggestions/permitted methods and commands, not subscriber secrets. Existing CHAP/PAP verification remains the credential boundary. | Read `parseMRU`, both strict list decoders, `ResolveAuthAllow`, `refuseAuthMethod` and the complete backend-digest C test. Native lab commands use isolated existing peers, not product execution of config text. |
| Dependencies / doctor | No new product binary/socket/module/certificate/listen dependency; existing PPP/kernel readiness checks and native lab preflights remain. Autotools additions are build-stage peer-image dependencies only. | Existing transport wiring; both Dockerfiles retain separate build/runtime stages and scanelf-derived runtime libraries. |

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/l2tp/ppp/auth_policy_driver_test.go`, `auth_policy_metrics_test.go`, `proxy_auth_completion_test.go` | Yes | Independent source reads of the selection/termination/MRU matrix, exporter scrape and full cryptographic CHAP test. |
| `internal/component/l2tp/ppp/rfc1661_proxy_policy_driver_test.go` | Yes | Independent current-carrier read; actual Driver proxy/PAP/noauth/full1500 assertions retained. Old neutral carrier is not credited. |
| Both promoted `.ci` files | Yes | Read each canonical file: per-command refusals, ordered parsed readback and L2TP live effective-config dispatch. Promoted runs passed bg34. |
| Three canonical scenario directories | Yes | Native scenario discovery/execution passed; read fixture policy, valid backend/pool, no-redial options and registered typed checker source. |
| Other new config/transport/checker tests and helper files | Yes | Exact paths named by the core census and source review above; native Review Gate will pin the complete source population after Main lands core. |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1/2 | Configured and default request/cap values | Read `ExtractParameters` default1500, `handleKernelSuccess` `MaxMRU: policy.MRU`, `sendConfigureRequest`; named floor/default tests P and current native MRU green/red/restored green. |
| AC-3/9 | Validation refuses bad range and policy | Read actual YANG range and original-tree registration; both canonical promoted cases passed with per-command exits, plus provider negatives P. |
| AC-4 | Full1500 receive survives smaller request | Read `readFrames` maximum buffer and `afterLCPOpen` `setMRU(..., MaxFrameLen)`; named Driver1500 unit P and current +/− records. |
| AC-5/13 | Required CHAP-only refusal never authenticates PAP | Read validated reply mutation→`refuseAuthMethod` before SCR/Open; current transport negative scenarios each complete green/red/restored green, with actual wire/backend red controls. |
| AC-6/7/8/10 | Explicit PAP, CHAP-before-PAP and deliberate optional noauth | Read `initialAuthMethod`, `selectAuthFallback`, auth dispatch and actual success tests P/F; current PAP compatibility bg45. |
| AC-11/12 | Proxy policy cannot bypass method/local-MRU limits | Read `EvaluateProxyLCP`, `acceptProxyPolicy`, normal startup fallback; matrix P and complete digest-verified CHAP C. |
| AC-14 | Existing generation retained across reload | Read `setPPPPolicy`, L2TP reload diff and PPPoE clone; actual reload/StartSession/Driver isolation tests P. |
| AC-15 | Measured output, not configured defaults | Read `handleLCPUp`/`handleLCPDown`→`sessionSnapshot`→`sessionJSON`; measured-vs-configured/null tests P and actual live current MRU output bg35/bg41. |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| L2TP MRU and effective policy | `test/l2tp/lns-mru-auth-allow-config.ci` | Reads real validation and starts the daemon; engine-steps dispatches `show l2tp config` and checks derived CHAP/PAP defaults absent from source. Driver transport seam and native current peer complete the wire path. |
| L2TP explicit allow/refusal | Same canonical L2TP case | Invalid cross-leaf policies refuse with exact correction; actual transport→Driver fixture and live LNS refusal prove enforcement beyond parser acceptance. |
| PPPoE flat authentication-allow | `test/pppoe/pppoe-auth-allow-config.ci` | Actual validation/readback, not an invented PPPoE config RPC. `TestPPPoEAuthAllowReachesDriver` plus current native refusal/PAP/CHAP cases prove production wire behavior. |
| New-session reload / negotiated output | L2TP canonical case plus named transport/snapshot/JSON units | Actual reload-to-StartSession and event-to-snapshot boundaries P, then live native MRU scenario reads config/list/specific session. No missing full-path behavior is inferred from helper-only tests. |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken | Legacy implicit-policy fixture failures P and deleted broad-default helper contract; independent approval and focused correction F. Current PAP compatibility is separately green and cannot confirm the original universal assumption. |
| A-2 | confirmed, bounded | Read actual pppd2.5.5 `fsm_rtermreq` and accel-ppp1.14.0 `ppp_fsm_recv_term_req`: TermAck and ReqSent, not immediate process exit. Actual current pppd L2TP CDN and PPPoE PADT teardown complete. Accel evidence remains source-only. |
| A-3 | confirmed | `TestReceiveFullFrameBelowConfiguredMRU` P; read maximum reader buffer/kernel1500 producer and current native +/− records. |
| A-4 | confirmed | Agreed one-knob coupling retained; `TestConfiguredMRUNakInformationFloor` P proves configured request and peer cap. No asymmetric-MRU scope added. |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Feature scope and level | Existing feature declarations remain experimental/partial; new policy prose maps to `ResolveAuthAllow` and transport extraction, not full RFC support | Source reviewed; Main owns final native feature/generated-output evidence. |
| Config syntax and examples | Actual two YANG schemas, `parseMRU`, both strict list decoders and original-tree validators; selected configuration-guide hunks | Yes; syntax grammar itself unchanged, both canonical config workflows passed. |
| CLI/API output | Existing `handleConfig` and `sessionJSON` gain fields; no new command, flag, RPC or completion branch | Yes; L2TP guide Read commands and measured-state paragraphs match current handlers. |
| Plugin SDK/inventory | No plugin, command registration, send type or SDK change; existing EventLCPUp gains actual method payload and both producers supply it | No new inventory entry owed; guide explains the event-backed output. |
| Wire format | Existing MRU/Auth-Protocol codecs are reused; policy/order/termination changed, not encoding layout | No new wire-layout page required; guide/labs/RFC notes document changed behavior. |
| RFC evidence | Read RFC1661 Sections3.5/5.4/6.1/6.2 and RFC1334 Section2 against producers; current records and fresh audit notes read | Final fresh bg59 stamp/update succeeded; personally read all93 foreign check findings in `artifact://301`, none against owned bindings/audits. No global-clean claim. |
| Test infrastructure | Existing registry now admits PPPoE drafts; native checkers, source-pinned images and obsolete special-route deletion are documented | Prerequisite pages, lab guides and final scoped tooling PASS support the claims; legacy Alpine/QEMU routes are not own-system proof here. |
| Architecture / source anchors | Searched `docs/`, `features/`, `ai/` for changed-source anchors; read subscriber lifecycle, PPPoE architecture, receive/kernel and accounting-cause claims | Existing layer boundaries remain; accounting table corrected at its existing page. Generic config grammar/comparison/routing metadata are unchanged. |
| Metrics | `initPPPMetrics` registers one reason label; `refuseAuthMethod` counts only the actual required policy close | Guide lists `ze_ppp_auth_failures_total{reason="auth-method-refused"}`; actual exporter test P. |
| Wiki and discovery | Seven targeted wiki pages and selected `ai/INDEX.md` task/keyword rows | Original independent reviews plus final latest-peer delta clean; native LLMS generated184 pages. Commit evidence not yet assumed. |
| Citations | Whole tracked-root fullpath/basename search; four live sibling specs repointed to existing guides; no Go Design header hit | Citation edits are prepared, not yet landed. The ignored generated roadmap is neither hand-edited nor committed. |

## Risks & Observations at Closure

| Item | Disposition |
|------|-------------|
| R-1: deployments relying on implicit weaker methods | Intentional safe-default change; explicit migration guidance names each permitted method. Current opted-in PAP compatibility passed. |
| R-2: previously accepted proxy PAP may renegotiate | Intended policy boundary; guide documents normal LCP fallback. Complete real-Driver CHAP authentication after refusal passed C. |
| R-3: pre-auth refusal has no Accounting-Stop | Refusal warning, retained down cause and bounded-reason counter carry it; architecture page does not claim a pre-auth accounting record. |
| Scoped verification limits | P overall PPP red/focused F correction, functional skips, unit-only proxy/full1500/reload controls, source-only accel semantics and historical peer results are kept distinct from current live evidence. |
| Ancillary setup/probe failures | Old download and Docker-info setup failures are non-proof. PPPoE semantic red independently contains PAP/backend/IPCP despite a later30s pcap-probe timeout. No causal diagnosis or new journal bug is invented from cleanup warnings. |
| Remaining closure machinery | Main owns core/wiki commits and command clearance. Closure commits and claim release are not yet executed; final owned RFC freshness is complete. |
