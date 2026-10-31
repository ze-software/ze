# Spec: home-cpe-14-lac-realm-relay

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P3 |
| Release reason | A new LAC mode, the way wholesale broadband LACs work: LCP and authentication with the subscriber at the LAC, the LNS chosen by the realm of the username, proxy LCP and proxy authentication AVPs in the ICCN. It also corrects a false test claim in the L2TP guide. |
| RFC gaps | RFC2661-4.4.5-1 to -6 (LAC half: emitted here); M-bit 0 rows for AVPs 26 to 28 to be added to `rfc/short/rfc2661.md` (forward-walk gap) |
| Depends | `plan/post-release/features/spec-l2tp-proxy-authentication.md` (AVP 29-33 writers and their rows; see Key Design Decisions for the split), `spec-home-cpe-16-lns-mru-and-auth-allow-list.md` (allow-list the LAC's own authentication obeys), `spec-home-cpe-18-interoplab-multi-link-topology.md` and `spec-home-cpe-19-access-chain-scenario.md` (lab proof) |
| Phase | 0/5 |
| Handoff | - |
| Updated | 2026-10-31 |

Parent: plan/home-cpe/spec-home-cpe-0-umbrella.md

Recovery after compaction: `.claude/rules/post-compaction.md`.

All values in this spec are invented (documentation prefixes, `example.net`).

## Task

Ze's LAC today relays a PPPoE subscriber into an L2TP incoming call at PADS,
keyed on an exact Service-Name match, and runs no PPP at all: the
subscriber's PPP channel is bridged to the pppol2tp channel and the LNS does
LCP and authentication. A wholesale access network instead answers the call
itself. Its LAC:

1. runs LCP with the subscriber,
2. starts authentication (CHAP Challenge, or receives the PAP
   Authenticate-Request) to learn the username,
3. selects the retail LNS from the realm of the username (`user@realm`),
4. places the incoming call carrying the proxy LCP AVPs (26 Initial Received
   LCP CONFREQ, 27 Last Sent LCP CONFREQ, 28 Last Received LCP CONFREQ) and
   the proxy authentication AVPs (29 to 33),
5. bridges the subscriber's channel to the session, so the LNS continues the
   PPP session (it may accept the proxy data, or renegotiate and re-challenge).

Add this as a second relay key kind beside Service-Name: a realm binding.
Define what happens when no realm matches, and when a matched realm cannot
be reached.

Also owed, in the same change:
- `docs/guide/l2tp.md` l.105-110 says the LAC bridge "is Linux-only and
  exercised through `./le test deployment l2tp-ppp`". That run
  (`internal/le/test/deployment/l2tpppp.go`) has xl2tpd dial Ze as LNS with
  pppd over it. No PPPoE and no Ze LAC are involved, so it never reaches
  `bridgeChannels`. `TestLACChannelBridge`
  (`internal/component/l2tp/bridge_integration_linux_test.go`) checks only
  that the ioctl wrappers reach the kernel. Its comment says the end-to-end
  path "is driven by `./le test deployment gokrazy-l2tp-ppp`", and
  `gokrazyl2tp.go` holds no PPPoE or relay either. Both claims are corrected
  to say what is proven. The first end-to-end proof of the relay data path
  is the umbrella's child 19 scenario `access-up`, plus this spec's
  scenarios.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/l2tp.md` - initiator, remote and relay (l.52-113)
  → Constraint: every dial uses the first `environment l2tp server`; the
  relay's remote must be declared under `l2tp remote`.
  → Decision: a Service-Name relay fires at PADS; that path is unchanged and
  keeps precedence.
- [ ] `docs/guide/pppoe.md` - subscriber authentication (l.80-132)
  → Constraint: the PPPoE AC's auth handler slot (local users or RADIUS) is
  shared with L2TP; a subscriber terminated locally is authenticated there.
- [ ] `docs/architecture/l2tp/followup-l2tp-call.md` - AC-3 PPPoE → L2TP relay
  → Decision: pppoe never imports l2tp; the decision crosses through the
  neutral `internal/core/callsink` registry. This spec extends that
  interface rather than adding a second path.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2661.md` - exists; rows RFC2661-4.4.5-1 to -6 are gaps
  pointing at `spec-l2tp-proxy-authentication.md`; row RFC2661-4.4-2 claims
  Tie Breaker is "the one M=0 AVP Ze emits", which becomes false here.
- [ ] RFC 2661 Section 4.4.5: "The LAC may have answered the call and
  negotiated LCP with the remote system, perhaps in order to establish the
  system's apparent identity. In this case, these AVPs may be included to
  indicate the link properties the remote system initially requested,
  properties the remote system and LAC ultimately negotiated, as well as PPP
  authentication information sent and received by the LAC."
  → Decision: this spec is that case; Ze includes all three LCP AVPs.
- [ ] RFC 2661 Section 4.4.5, AVP 26: "LCP CONFREQ is a copy of the body of
  the initial CONFREQ received, starting at the first option within the body
  of the LCP message." and, for each of AVPs 26, 27 and 28: "The M-bit for
  this AVP MUST be set to 0."
  → Constraint: the value is the options only (no LCP header); M=0 on
  every proxy AVP, which the summary has no row for yet.
- [ ] RFC 2661 Section 4.4.5, AVP 27: "The LCP CONFREQ is a copy of the body
  of the final CONFREQ sent to the client to complete LCP negotiation"; AVP
  28: "a copy of the body of the final CONFREQ received from the client to
  complete LCP negotiation".
  → Constraint: "final" means the Configure-Requests that were Acked, so
  the capture records the Acked pair, not the last ones seen.
- [ ] RFC 2661 Section 4.4.5, Proxy Authen Type: "This AVP MUST be present if
  proxy authentication is to be utilized." Proxy Authen Response: "For types
  1 or 3, it contains the clear text password received from the client by
  the LAC.  In the case of cleartext passwords, AVP hiding is recommended."
  → Decision: PAP proxy data is sent only hidden (needs the remote's
  `shared-secret`); without a secret the LAC sends no proxy authentication
  for PAP and the LNS restarts authentication.
- [ ] RFC 2661 Section 9.5: "This implies a direct trust relationship of the
  LAC on behalf of the LNS."
  → Constraint: the LAC never answers authentication itself on a relayed
  session (no CHAP Success or Failure, no PAP Ack) except to refuse it.
- [ ] `rfc/short/rfc1661.md`, `rfc/short/rfc1994.md`, `rfc/short/rfc1334.md`
  exist. RFC 1661 Section 3.5: "If authentication fails, the authenticator
  SHOULD proceed instead to the Link Termination phase."

**Key insights:**
- The PPP driver already runs LCP and CHAP or PAP for PPPoE AC sessions, and
  every `EventAuthRequest` carries Method, Identifier, Username, Challenge
  and Response. Those are the inputs of AVPs 29 to 33.
- The pppoe drain (`pppoe/drain.go`) is the one place that sees the
  username before any credential check, so the realm decision belongs there.
- accel-ppp, the LNS VyOS uses, ignores AVPs 26 to 33 (`l2tp.c`, ICCN
  attribute loop: bare `break`) and so renegotiates. A Ze LAC must
  interoperate with an LNS that uses none of the proxy data.
- No local source shows a realm LAC (xl2tpd and accel-ppp have none). The
  no-match behaviour below is a Ze decision, flagged for the owner.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/pppoe/server.go` - after PADS (l.318-331)
  `relayToL2TP` calls the callsink; on acceptance it closes the PPP unit fd
  and starts no local PPP; on a match whose dial fails it logs "L2TP relay
  failed; terminating locally" and starts local PPP (l.455-482). Local PPP is
  `ppp.StartSession` with `MaxMRU` 1492, `AuthMethod`, `AuthRequired`.
- [ ] `internal/core/callsink` - `Request` carries Service, Interface,
  SubscriberMAC, SessionID, ChannelFD; `Sink.Relay` returns
  (accepted, error).
- [ ] `internal/component/l2tp/relay_sink.go` - `Relay` matches
  `req.Service` through `relayTargetLocked` (exact string), dials the remote
  through the first reactor and calls `placeIncomingCall` with the subscriber
  MAC as Calling Number and the pppox fd. An unmatched service returns
  (false, nil); no listener returns `errRelayNoListener`.
- [ ] `internal/component/l2tp/session_initiator.go` - `callParams` has no
  proxy fields; `writeICCNBody` writes Message Type, Tx Connect Speed and
  Framing Type only.
- [ ] `internal/component/l2tp/avp_compound.go` - `writeAVPProxyAuthenID`
  exists with no non-test caller.
- [ ] `internal/component/l2tp/bridge_linux.go` - `maybeBridgePPPoE`
  bridges when the session is not LNS and carries a PPPoE channel fd;
  `bridgeChannels` issues PPPIOCBRIDGECHAN.
- [ ] `internal/component/l2tp/pppoe/drain.go` - reads `EventAuthRequest`
  from the driver and calls the registered auth handler, then
  `Driver.AuthResponse`.
- [ ] `internal/component/l2tp/ppp/auth_events.go` - `EventAuthRequest`
  fields listed above.
- [ ] `internal/component/l2tp/ppp/session_run.go` - normal LCP path; no
  capture of the Configure-Requests exchanged.
- [ ] `internal/component/l2tp/yang/ze-l2tp-conf.yang` - `list relay` keyed
  by `service` (l.293-318).
- [ ] `internal/le/test/deployment/l2tpppp.go` - header comment: xl2tpd dials
  Ze, pppd negotiates; no PPPoE.
- [ ] `internal/component/l2tp/bridge_integration_linux_test.go` -
  `TestLACChannelBridge` and its comment.

**Behavior to preserve:**
- Service-Name relay at PADS: same matching, same precedence, same
  fall-back-to-local on a failed dial.
- With no realm binding configured, a PPPoE AC session is terminated locally
  exactly as today.
- Calling Number carries the subscriber MAC.

**Behavior to change:**
- With realm bindings configured, a PPPoE session that no Service-Name
  binding took runs LCP and starts authentication at the LAC; a username
  whose realm matches is relayed with proxy data instead of authenticated.
- ICCN gains AVPs 26 to 28 (and 29 to 33 per Key Design Decisions) on such
  calls.
- The two false test claims named in the Task.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `l2tp { relay-realm <realm> { remote <name>; } }` beside the
  existing `relay <service-name> { remote <name>; }`.
- Wire: PPPoE discovery, then LCP and the authentication exchange from the
  subscriber.

### Transformation Path
1. PADS: Service-Name relay is tried first (unchanged). If it does not take
   the subscriber and any `relay-realm` exists, local PPP starts with a
   capture flag on `ppp.StartSession`.
2. PPP driver: LCP runs as for any AC session; the session records the first
   Configure-Request received, and the Configure-Request pair that ended in
   Opened (Ze's Acked request, the peer's request Ze Acked).
3. Authentication: CHAP Challenge sent (or PAP Authenticate-Request
   received); the driver emits `EventAuthRequest` carrying the capture.
4. pppoe drain: splits the realm from the username and asks the callsink
   with a realm request before any auth handler runs.
5. l2tp relay sink: matches the realm; on a match dials and places the
   incoming call with proxy fields in `callParams`; returns accepted.
6. Reactor: ICCN written with AVPs 26 to 28 (and 29 to 33); on the session's
   kernel creation the PPPoE channel is bridged (`maybeBridgePPPoE`), and the
   relay outcome (bridged, or failed with a cause) is reported back.
7. pppoe drain: on bridged, tells the driver to release the session without
   any authentication reply and without Terminate-Request; the unit fd is
   closed (the unit was never connected, NCP never ran). On failure, the
   driver answers the authentication with a refusal and closes LCP; PADT
   follows.
8. No match: the drain hands the request to the auth handler as today
   (local termination).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| pppoe → l2tp | `internal/core/callsink` Request (extended) and an outcome callback | No |
| pppoe → PPP driver | `ppp.StartSession` capture flag; a new driver release call | No |
| Ze LAC ↔ LNS | ICCN AVPs 26 to 33 | No |
| Kernel | PPPIOCBRIDGECHAN after the PPP driver stops reading the channel | No |

### Integration Points
- `callsink.Request` gains: realm, username, the three captured LCP option
  bodies, and the authentication data (method, identifier, challenge,
  response); plus an outcome callback.
- `relay_sink.go` gains realm matching beside `relayTargetLocked`.
- `callParams` gains the proxy fields; `writeICCNBody` writes them when set.
- `ExtractParameters` validates `relay-realm` (remote declared, realm
  syntax) on the same path `ze config validate` runs for `relay`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The relay still crosses only through callsink; ICCN still written by the reactor |
| No unintended coupling (components stay isolated) | Yes | pppoe and ppp never import l2tp; the ppp package only captures bytes and releases a session |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses the driver's LCP and auth, the relay sink, `placeIncomingCall`, `maybeBridgePPPoE` |
| Zero-copy preserved where applicable (refs, not copies) | Yes | The captured option bodies are copied once at capture; AVPs written into the ICCN buffer with the existing `WriteAVP*` helpers |
| Registration over hardcoding, outbound | Yes | No new handler; the sink stays the one registered callsink |
| Registration over hardcoding, inbound | Yes | No switch learns a realm; bindings are config data |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Once PPPIOCBRIDGECHAN is set, frames on the PPPoE channel go to the bridged channel and no longer reach the `/dev/ppp` channel fd the driver read | Linux `ppp_generic.c` `ppp_input` checks the bridge first | the LAC's driver keeps consuming the subscriber's frames | `TestLACRealmBridgeAfterLocalLCP` (Linux integration) | unvalidated |
| A-2 | A subscriber waiting for CHAP Success answers a fresh Challenge from the LNS | RFC 1994 Section 4.1 ("Additional Challenge packets MUST be sent until a valid Response packet is received"), pppd behaviour, scenario `pppoe-chap-rechallenge-ze-client` for the Ze client | the session stalls at the LNS's challenge | interop `l2tp-lac-realm-relay-accel-ppp` | unvalidated |
| A-3 | Hidden AVP writing works for AVP 33 | `hidden.go` exists; `rfc/short/rfc2661.md` says "Hidden-AVP integration remains a gap" | PAP proxy data cannot be sent hidden | Phase 1 reads `hidden.go`; if writing is missing, PAP sessions send no proxy auth (decision below) and the gap stays with its owner | unvalidated |
| A-4 | The dial and the ICRQ, ICRP, ICCN exchange fit inside the subscriber's authentication patience | pppd CHAP waits several seconds per retry | subscriber retries or gives up | scenario measures nothing by time; it asserts the session comes up | unvalidated |
| A-5 | Realm is the text after the last `@`, compared case-insensitively | common usage; DNS names are case-insensitive | a deployment uses `realm/user` or `user%realm` | Known Limitations | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A wholesale subscriber's credential reaches the LAC operator's auth backend when no realm matches | RADIUS logs at the LAC | documented; the operator declares an `l2tp auth` backend only if he wants local termination; with none, the session is refused (AuthRequired) |
| R-2 | Frames the subscriber sends between the auth Response and the bridge are lost | LCP Echo misses at the CPE | the driver keeps answering LCP Echo until release; the gap is one ICRQ, ICRP, ICCN round trip |
| R-3 | The LNS renegotiates LCP and the subscriber's PPP stack does not cope | LCP failure at the CPE | not this spec's defect; child 12 for a Ze CPE, RFC 1661 Section 4.3 for the rest |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | PPPoE subscribers on a LAC with realm bindings fail to come up, or a realm-less subscriber is relayed; Service-Name relays and local termination without realm bindings are untouched |
| How is it reverted? | Single commit revert; the list is new |
| Who else touches this path? | `spec-ppp-l2tp-aaa-one-off-defects.md` (relay validation in `ExtractParameters`, AC-15), `spec-l2tp-proxy-authentication.md`, `spec-home-cpe-15` and `-16` (LNS consumers), `spec-home-cpe-19` (scenario) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `relay-realm example.net { remote lns-a }` + PPPoE subscriber `cpe@example.net` | → | pppoe drain → callsink → `relaySink` realm match → `placeIncomingCall` with proxy fields | `TestRelaySinkRealmMatchPlacesCallWithProxyLCP` (`internal/component/l2tp/relay_sink_test.go`) |
| PPPoE session with capture flag | → | ppp driver capture → `EventAuthRequest` | `TestDriverCapturesProxyLCPForRelay` (`internal/component/l2tp/ppp/manager_test.go`) |
| ICCN for a realm call | → | `writeICCNBody` | `test/l2tp-wire/iccn-proxy-avps.ci` |
| `ze config validate` | → | `ExtractParameters` realm checks | `test/l2tp/relay-realm-config.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `relay-realm example.net { remote lns-a }`, subscriber authenticates as `cpe@example.net` with CHAP | LAC runs LCP, sends a Challenge, receives the Response, sends neither Success nor Failure, dials `lns-a`, places an incoming call, bridges; the subscriber's next PPP frames reach the LNS |
| AC-2 | AC-1's ICCN | Carries AVP 26 (first Configure-Request options from the subscriber), 27 (Ze's Acked Configure-Request options), 28 (the subscriber's Acked Configure-Request options), each with M=0; and AVPs 29 (type 2), 30, 31, 32 (most significant octet 0), 33 |
| AC-3 | PAP subscriber, remote with `shared-secret` | AVP 29 type 3, 30, 32 (the Authenticate-Request Identifier), 33 hidden; the LAC sends no Authenticate-Ack |
| AC-4 | PAP subscriber, remote without `shared-secret` | AVPs 26 to 28 only; no proxy authentication; the log says why |
| AC-5 | Username with no `@`, or a realm no binding names | Local termination: the request goes to the auth handler as for any AC session |
| AC-6 | Realm matched, remote unreachable, tunnel refused, or the LNS answers CDN before the bridge | CHAP Failure (or PAP Authenticate-Nak) with a message naming the realm, LCP Terminate-Request, PADT; no local authentication |
| AC-7 | A Service-Name relay binding matches at PADS | Service-Name relay wins; no local LCP (today's path) |
| AC-8 | No `relay-realm` configured | PPPoE AC behaviour identical to today |
| AC-9 | `relay-realm` naming an undeclared remote, an empty realm, a realm with `@` | `ze config validate` refuses it with the fix; daemon start refuses it too |
| AC-10 | Two subscribers of different realms | Each reaches its own remote; one tunnel per remote, shared by sessions |
| AC-11 | Reload adds or removes a `relay-realm` | Applies to the next authentication; relayed sessions stay up |
| AC-12 | `ze show l2tp session` on the LAC | Shows the realm, the remote and that proxy LCP and proxy authentication were sent |
| AC-13 | `docs/guide/l2tp.md` and the bridge test comment | State which tests prove the LAC bridge; no reference to `l2tp-ppp` or `gokrazy-l2tp-ppp` as its proof |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures a wholesale LAC with two retail realms | PADS → LCP → CHAP Challenge/Response → realm → ICRQ/ICRP/ICCN → bridge | interop `l2tp-lac-realm-relay-accel-ppp` |
| 2 | Runs the wholesale lab | Ze CPE → Ze LAC (realm) → Ze LNS | umbrella `access-wholesale` (child 19) |
| 3 | A subscriber types the wrong realm | no match → local auth handler → refused (no local users) | `test/pppoe/pppoe-realm-relay-nomatch.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRealmFromUsername` | `internal/component/l2tp/pppoe/drain_test.go` | last `@`, case folding, no `@` | |
| `TestRelaySinkRealmMatchPlacesCallWithProxyLCP` | `internal/component/l2tp/relay_sink_test.go` | wiring, AC-1 | |
| `TestRelaySinkRealmNoMatchReturnsUnaccepted` | `internal/component/l2tp/relay_sink_test.go` | AC-5 | |
| `TestRelaySinkRealmDialFailureReportsRefusal` | `internal/component/l2tp/relay_sink_test.go` | AC-6 | |
| `TestDriverCapturesProxyLCPForRelay` | `internal/component/l2tp/ppp/manager_test.go` | first received, final Acked pair (including after a Nak round) | |
| `TestDriverRelayReleaseSendsNoAuthReply` | `internal/component/l2tp/ppp/auth_dispatch_test.go` | AC-1: no Success, Failure, Ack or Terminate on release | |
| `TestDriverRelayRefusalClosesLCP` | `internal/component/l2tp/ppp/auth_dispatch_test.go` | AC-6 | |
| `TestWriteICCNBodyProxyAVPs` | `internal/component/l2tp/session_initiator_test.go` | AC-2 byte layout, M=0, ID high octet 0 | |
| `TestWriteICCNBodyPAPProxyHiddenOrAbsent` | `internal/component/l2tp/session_initiator_test.go` | AC-3, AC-4 | |
| `TestRelayRealmConfigValidation` | `internal/component/l2tp/config_test.go` | AC-9 | |
| `TestServiceNameRelayPrecedesRealm` | `internal/component/l2tp/pppoe/server_test.go` | AC-7 | |
| `TestLACRealmBridgeAfterLocalLCP` | `internal/component/l2tp/bridge_integration_linux_test.go` | A-1 with real pppox sockets | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| realm length | 1..253 (DNS name) | 253 | 0 (empty) | 254 |
| Proxy Authen ID | 0..255 in the low octet | 255 | N/A | high octet non-zero never written |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `relay-realm-config` | `test/l2tp/relay-realm-config.ci` | validate accepts a realm binding; refuses AC-9 cases | |
| `iccn-proxy-avps` | `test/l2tp-wire/iccn-proxy-avps.ci` | decoded ICCN shows AVPs 26 to 33 with explicit hex | |
| `pppoe-realm-relay-nomatch` | `test/pppoe/pppoe-realm-relay-nomatch.ci` | unmatched realm terminates locally and is refused with no local users | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `l2tp-lac-realm-relay-accel-ppp` | `test/interop-l2tp/scenarios/` | pppd PPPoE client; accel-ppp LNS (ignores proxy AVPs, renegotiates) | realm selection, local LCP and CHAP at Ze, bridge, IPCP from the LNS, ping across | |
| `l2tp-lac-realm-relay-xl2tpd` | `test/interop-l2tp/scenarios/` | pppd PPPoE client; xl2tpd and pppd LNS | the same against a second LNS implementation | |
| `access-wholesale` | child 19 | Ze CPE, Ze LAC, Ze LNS | umbrella variant | |

Each scenario is broken once (realm match disabled; bridge skipped) and the
red is recorded. These scenarios need two links per node, so they wait on
child 18.

## Files to Modify
- `internal/component/l2tp/yang/ze-l2tp-conf.yang` - `list relay-realm`
- `internal/component/l2tp/config.go` - parse and validate
- `internal/core/callsink/callsink.go` - realm request fields, outcome callback
- `internal/component/l2tp/relay_sink.go` - realm matching, outcome reporting
- `internal/component/l2tp/session_initiator.go` - `callParams` proxy fields, `writeICCNBody`
- `internal/component/l2tp/avp_compound.go` - callers for the proxy writers
- `internal/component/l2tp/bridge_linux.go` - report bridge outcome
- `internal/component/l2tp/pppoe/server.go` - start local PPP with capture when realm bindings exist
- `internal/component/l2tp/pppoe/drain.go` - realm decision before the auth handler
- `internal/component/l2tp/ppp/start_session.go`, `ppp/session_run.go`, `ppp/manager.go`, `ppp/auth_events.go` - capture, release, refusal
- `internal/component/l2tp/bridge_integration_linux_test.go` - correct the comment
- `docs/guide/l2tp.md` - relay-realm section; l.105-110 corrected
- `docs/guide/pppoe.md` - relay note
- `docs/architecture/l2tp/followup-l2tp-call.md` - realm relay flow
- `rfc/short/rfc2661.md` - rows 4.4.5-1 to -6 annotations; new M-bit rows for AVPs 26 to 28; row 4.4-2 annotation corrected
- `features/l2tpv2-bng.md`, `features/pppoe-access.md` - LAC realm relay

## Files to Create
- `test/l2tp/relay-realm-config.ci`
- `test/l2tp-wire/iccn-proxy-avps.ci`
- `test/pppoe/pppoe-realm-relay-nomatch.ci`
- `test/interop-l2tp/scenarios/l2tp-lac-realm-relay-accel-ppp/`
- `test/interop-l2tp/scenarios/l2tp-lac-realm-relay-xl2tpd/`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `list relay-realm` keyed by `realm`, leaf `remote` |
| YANG validation constraints | Yes | realm `length "1..253"` and a pattern refusing `@` and whitespace; remote `length "1..64"` |
| YANG custom validators | Yes | remote must name an `l2tp remote` (the existing relay check) |
| CLI commands/flags | No | config only; the session show gains fields |
| CLI grammar (keyword before value) | N-A | no command |
| Editor autocomplete | Yes | `CompleteFn` for `remote` reusing the relay's |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | existing show command |
| Env var registration | N-A | not under `environment` |
| Doctor check for runtime dependencies | N-A | no new runtime dependency beyond the existing relay's |
| Prometheus counters/metrics | Yes | `ze_l2tp_relay_total{kind="realm",outcome=...}` (name confirmed in Phase 1 against existing relay metrics) |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/l2tpv2-bng.md`, `features/pppoe-access.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/l2tp.md` |
| 3 | CLI command added/changed? | No | session show fields only, documented in the guide |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/guide/l2tp.md`, `docs/guide/pppoe.md` |
| 7 | Wire format changed? | Yes | `docs/guide/l2tp.md` ICCN AVPs; `docs/architecture/l2tp/followup-l2tp-call.md` |
| 8 | Plugin SDK/protocol changed? | No | callsink is internal |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc2661.md` |
| 10 | Test infrastructure changed? | No | scenarios use child 18's harness |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/l2tp/followup-l2tp-call.md` |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | Yes | metrics section of `docs/guide/l2tp.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derived with `./le spec citation anchors spec plan/home-cpe/spec-home-cpe-14-lac-realm-relay.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the `remote`/`relay` example at `docs/guide/l2tp.md` l.60-73 |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- YANG list, `Parameters`,
   callsink fields, sink realm match stub, drain hook; failing wiring tests.
   The guide l.105-110 and the bridge test comment are corrected in this
   phase (a page that is wrong is repaired before the next code edit).
   - Tests: `TestRelaySinkRealmMatchPlacesCallWithProxyLCP`,
     `relay-realm-config`
   - Files: YANG, `config.go`, `callsink`, `relay_sink.go`, `pppoe/drain.go`
2. **Phase: Capture** -- driver records the Configure-Requests; capture flag
   from `server.go`.
3. **Phase: ICCN** -- `callParams` proxy fields, `writeICCNBody`, hidden
   AVP 33 for PAP; wire `.ci`.
4. **Phase: Hand-off** -- outcome callback, release without reply, refusal
   path, bridge after local LCP (Linux integration test).
5. **Phase: Docs, RFC rows, interop** -- guide, architecture doc, summary
   rows, two scenarios with a recorded red.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | AVP 27 and 28 hold the Acked pair, not the last frames seen |
| Correctness | No CHAP Success, Failure or PAP Ack leaves the LAC on a relayed session |
| Correctness | A matched-but-failed realm never authenticates locally |
| Data flow | pppoe still never imports l2tp; ppp stays transport-neutral |
| Rule: rfc-compliance | every new implementing function quotes Section 4.4.5 |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Realm relay end to end | interop `l2tp-lac-realm-relay-accel-ppp` |
| Proxy AVPs on the wire | `iccn-proxy-avps.ci` hex assertions |
| Guide corrected | grep `docs/guide/l2tp.md` for `l2tp-ppp` near "bridge" |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Credential exposure | PAP passwords never leave unhidden; never logged |
| Realm spoofing | Realm comparison exact after case folding; no suffix match, so `x.example.net.evil` never matches `example.net` |
| Resource exhaustion | A subscriber cycling realms causes at most one dial per remote; the existing max-sessions caps apply |

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

- The guide's bridge claim and the bridge test's comment cite two different
  runs as the proof, and neither run carries PPPoE.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

None: `grep -rn -i "realm\|l2tp-ppp\|LAC bridge\|relay.*bridge" plan/journal/`
found no open row for these surfaces.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| No realm match: terminate locally (hand the request to the auth handler, as any AC session) | Reject at once; a default remote | Same semantics as an unmatched Service-Name today, so the two relay kinds read alike. On a pure wholesale LAC with no local users and no RADIUS, the auth handler refuses and the subscriber gets CHAP Failure, so "reject" falls out without a second rule. A LAC that also serves its own retail users keeps working. The RFC is silent; this follows Juniper, the best-documented vendor: its LAC test output "authentication success, locally terminated" is explained as "the user was terminated locally rather than tunneled; this means that no tunnel was found to be associated with the user" (Junos "Testing L2TP Tunnel Configurations from the LAC"). Nokia SR OS decides from the authentication reply: tunnel attributes from RADIUS or the local user database extend the session to the LNS, otherwise the session is a locally terminated (PTA) session; its only documented refusal is for MLPPPoX, "because the 7750 SR does not support directly terminated MLPPPoX sessions" |
| Matched realm whose call fails: refuse (CHAP Failure or PAP Nak, then Terminate and PADT), never local | Fall back locally, like a failed Service-Name relay | The subscriber has named a network this LAC does not serve; authenticating him against the LAC's backend authenticates someone else's customer. RFC 1661 Section 3.5: "the authenticator SHOULD proceed instead to the Link Termination phase". Juniper does the same when every tunnel of the selected profile fails: "the router rejects the PPP user session without attempting to contact the remote router" (Junos LAC tunnel selection overview) |
| Service-Name relay keeps precedence and still fires at PADS | Realm first | Realm needs LCP and authentication; Service-Name is known at PADS. Precedence keeps today's configs unchanged |
| Separate `list relay-realm` keyed by realm | One `relay` list with a match-kind leaf | The existing list is keyed by `service`; re-keying it breaks configs and mixes two key spaces in one list |
| Realm = text after the last `@`, compared case-insensitively, exact | Suffix matching; regex | Exact match is predictable and cannot be spoofed by a longer name |
| LAC half of proxy authentication (emitting AVPs 29 to 33 from the LAC's own exchange) is delivered here; `spec-l2tp-proxy-authentication.md` keeps the LNS half (consuming them, the `proxy-authentication` leaf, row RFC2661-9.5-2) | Wait for that spec to emit them | That spec's LAC half needs an authentication exchange at the LAC, which only this spec creates. **Owed coordination**: when this spec moves to `ready`, that spec's Task is narrowed to the LNS half in the same commit |
| PAP proxy data sent only hidden; without a tunnel secret no proxy authentication for PAP | Send cleartext | RFC 2661 Section 4.4.5: "In the case of cleartext passwords, AVP hiding is recommended"; the LNS restarts authentication instead, which the RFC states as the result of an absent Proxy Authen Type |
| The driver keeps answering LCP Echo until the outcome arrives | Release at the auth Response | Keeps the subscriber's keepalive alive across the dial; bounded by the authentication timeout |
| Proxy LCP AVPs always sent on a realm call | Make them optional | Section 4.4.5 describes this exact case; an LNS that does not use them (accel-ppp) ignores them |

## Known Limitations

- Realm selection by RADIUS tunnel attributes (RFC 2868 Tunnel-Type,
  Tunnel-Server-Endpoint returned to the LAC) is not offered. Real
  wholesale networks often use it, but no home-CPE child needs it, and it is
  not inside this umbrella's scope. If the owner wants it, it is a spec of
  its own in `plan/post-release/features/`.
- Prefix realms (`realm/user`) and other delimiters are not parsed (A-5).

## RFC Documentation (Scope: protocol)

Every new implementing function quotes RFC 2661 Section 4.4.5 for the AVP it
writes (including "The M-bit for this AVP MUST be set to 0."), and the
refusal path quotes RFC 1661 Section 3.5. ICCN layout gains an ASCII diagram
of the added AVPs with offsets.

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
