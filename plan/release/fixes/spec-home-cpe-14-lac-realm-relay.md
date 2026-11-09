# Spec: home-cpe-14-lac-realm-relay

| Field | Value |
|-------|-------|
| Status | blocked |
| Scope | protocol |
| Priority | P3 |
| Release reason | A new LAC mode, the way wholesale broadband LACs work: LCP and authentication with the subscriber at the LAC, the LNS chosen by the realm of the username, proxy LCP and proxy authentication AVPs in the ICCN. It also corrects a false test claim in the L2TP guide. |
| RFC gaps | RFC2661-4.4.5-1, RFC2661-4.4.5-2, RFC2661-4.4.5-3, RFC2661-4.4.5-4, RFC2661-4.4.5-5, RFC2661-4.4.5-6, RFC2661-4.3-1, RFC2661-4.3-3 |
| Depends | Implementation: none; the PPPoE AC allow-list is implemented (`docs/guide/pppoe.md`, "Subscriber authentication") and decides which method the LAC's own authentication starts with and refuses a fallback outside it. Closure: the two interop scenarios this spec owns need `spec-home-cpe-18-interoplab-multi-link-topology.md`. No dependency on `plan/post-release/features/spec-l2tp-proxy-authentication.md`: this spec writes every LAC-side proxy AVP itself, including hidden AVP 33 (Key Design Decisions, "AVP writer ownership") |
| Phase | 5/5 |
| Handoff | - |
| Updated | 2026-11-09 |

Parent: plan/release/fixes/spec-home-cpe-0-umbrella.md

RFC gaps, in words: rows RFC2661-4.4.5-1 to -6 are the LAC half, emitted here.
RFC2661-4.3-1 (a Random Vector AVP precedes the first hidden AVP) and
RFC2661-4.3-3 (the H bit only with a shared secret) become owed here, because
AVP 33 for PAP is the first AVP Ze sends hidden.
The M-bit 0 rows for AVPs 26 to 28 are still to be added to
`rfc/short/rfc2661.md` (a forward-walk gap), so they have no id yet.

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

Already owned elsewhere, and verified here:
- The relay section of `docs/guide/l2tp.md` ("Initiator: dial targets and
  outgoing calls") says the LAC bridge "is Linux-only and
  exercised through `./le test deployment l2tp-ppp`". That run
  (`internal/le/test/deployment/l2tpppp.go`) has xl2tpd dial Ze as LNS with
  pppd over it. No PPPoE and no Ze LAC are involved, so it never reaches
  `bridgeChannels`. `TestLACChannelBridge`
  (`internal/component/l2tp/bridge_integration_linux_test.go`) checks only
  that the ioctl wrappers reach the kernel. Its comment says the end-to-end
  path "is driven by `./le test deployment gokrazy-l2tp-ppp`", and
  `gokrazyl2tp.go` holds no PPPoE or relay either. Both claims are corrected
  to say what is proven. Child 19 owns that correction (its AC-10) and the
  bridge test comment, because its `access-up` is the first end-to-end proof
  of the relay data path and it lands first. This spec's AC-13 adds its own
  scenarios as the proof of the realm path.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/l2tp.md` "Initiator: dial targets and outgoing calls" - initiator, remote and relay
  → Constraint: every dial uses the first `environment l2tp server`; the
  relay's remote must be declared under `l2tp remote`.
  → Decision: a Service-Name relay fires at PADS; that path is unchanged and
  keeps precedence.
- [ ] `docs/guide/pppoe.md` - subscriber authentication section
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
- [ ] RFC 2661 Section 4.3: "If the H bit is set in any AVP(s) in a given
  control message, a Random Vector AVP must also be present in the message
  and MUST precede the first AVP having an H bit of 1." and "The H bit MUST
  only be set if a shared secret exists between the LAC and LNS."
  → Constraint: the ICCN of a PAP realm call carries Random Vector (AVP 36)
  before AVP 33; no other AVP is hidden.
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
- [ ] `internal/component/l2tp/pppoe/server.go` - after PADS, `handlePADR`
  calls `relayToL2TP`, which calls the callsink with the pppox fd as
  `ChannelFD`; on acceptance the server closes the PPP unit fd and starts no
  local PPP; on a match whose dial fails `relayToL2TP` logs "L2TP relay
  failed; terminating locally" and returns false, and local PPP starts. Local PPP is
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
- [ ] `internal/component/l2tp/hidden.go` - `HiddenEncrypt` and
  `HiddenDecrypt` implement the Section 4.3 cipher; their only callers are
  tests (`hidden_test.go`, `hidden_fuzz_test.go`,
  `accelppp_ng_regression_test.go`). No writer sets the H bit and nothing emits
  `AVPRandomVector` (`avp.go`; the CLI decoder only prints it), which
  `rfc/short/rfc2661.md` records as gap RFC2661-4.3-1.
- [ ] `internal/component/l2tp/bridge_linux.go` - `maybeBridgePPPoE`
  bridges when the session is not LNS and carries a PPPoE channel fd, from
  the kernel-setup event of the L2TP session; `bridgeChannels` issues
  PPPIOCBRIDGECHAN. On an ioctl error it still returns `bridged` true with
  the error (a userspace relay fall-back), so PPP is not run.
- [ ] `internal/component/l2tp/pppoe/drain.go` - reads `EventAuthRequest`
  from the driver and calls the registered auth handler, then
  `Driver.AuthResponse`.
- [ ] `internal/component/l2tp/ppp/auth_events.go` - `EventAuthRequest`
  fields listed above.
- [ ] `internal/component/l2tp/ppp/session_run.go` - normal LCP path; no
  capture of the Configure-Requests exchanged.
- [ ] `internal/component/l2tp/yang/ze-l2tp-conf.yang` - `list relay` keyed
  by `service`, leaf `remote`.
- [ ] `internal/component/l2tp/cmd/l2tp.go` - `handleSession` answers
  `show l2tp session id <id>` with `sessionJSON` over `SessionSnapshot`,
  which carries no relay or proxy field today.
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

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Neutral relay call-sink | `internal/core/callsink/callsink.go` `Request`, `Sink`; `docs/architecture/l2tp/followup-l2tp-call.md` AC-3 | lets pppoe ask for a relay without importing l2tp | extends: `Request` gains realm, username, captured LCP bodies, auth data and an outcome callback; no second path |
| Service-Name relay at PADS | `internal/component/l2tp/pppoe/server.go` `relayToL2TP`; `internal/component/l2tp/relay_sink.go` `relayTargetLocked` | exact Service-Name match, dial, incoming call, local fall-back on dial failure | extends: realm matching sits beside it; Service-Name keeps precedence and its fall-back |
| Incoming call placement | `internal/component/l2tp/session_initiator.go` `callParams`, `placeIncomingCall`, `writeICCNBody` | places ICRQ, ICRP, ICCN; ICCN carries Message Type, Tx Connect Speed, Framing Type | extends: `callParams` carries proxy fields and `writeICCNBody` writes AVPs 26 to 33 when set |
| Proxy Authen ID writer | `internal/component/l2tp/avp_compound.go` `writeAVPProxyAuthenID` | writes AVP 32 with the high octet 0, no non-test caller | extends: gains its first caller |
| Hidden AVP encryption | `internal/component/l2tp/hidden.go` `HiddenEncrypt` | RFC 2661 hidden-AVP encryption, test callers only; no writer sets the H bit and no Random Vector is emitted | extends: gets its first production caller through a hidden-AVP writer and a Random Vector AVP writer added beside the `WriteAVP*` helpers, used for AVP 33 on PAP only |
| Session show | `internal/component/l2tp/cmd/l2tp.go` `handleSession`, `sessionJSON`; `internal/component/l2tp/snapshot.go` `SessionSnapshot` | `show l2tp session id <id>` | extends: the snapshot and the JSON gain the relay fields of AC-12 |
| LAC channel bridge | `internal/component/l2tp/bridge_linux.go` `maybeBridgePPPoE`, `bridgeChannels` | PPPIOCBRIDGECHAN between the PPPoE and pppol2tp channels on a non-LNS session | extends: reports the outcome; bridges after local LCP instead of at PADS |
| PPPoE auth drain and auth handler slot | `internal/component/l2tp/pppoe/drain.go` `startPPPoEAuthDrain`; `internal/component/l2tp/subscriber/handler_registry.go` `RegisterAuthHandler` | reads `EventAuthRequest` and calls the one registered auth handler | extends: the realm decision runs before the handler; no match falls through to it unchanged |
| PPP driver auth event | `internal/component/l2tp/ppp/auth_events.go` `EventAuthRequest` | carries Method, Identifier, Username, Challenge, Response | extends: carries the LCP capture; the driver gains release and refusal calls |
| Proxy authentication, LNS half | `plan/post-release/features/spec-l2tp-proxy-authentication.md` | owns the consumer side and row RFC2661-9.5-2 | does not apply here: that spec keeps the LNS half (Key Design Decisions) |

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
6. Reactor: ICCN written with AVPs 26 to 28 (and 29 to 33, AVP 33 hidden
   after a Random Vector for PAP); when the L2TP session's kernel setup is
   ready, the sink reports "ready to bridge" through the outcome callback and
   waits, holding the session, for the drain's answer. It does not bridge yet.
7. pppoe drain, on "ready to bridge": tells the driver to quiesce. The driver
   stops its reader on the subscriber's `/dev/ppp` channel descriptor and
   waits until that reader has returned, stops answering LCP Echo, and keeps
   its LCP state and the descriptor for writing. The drain answers the sink
   "quiesced".
8. l2tp sink, on "quiesced": `maybeBridgePPPoE` issues PPPIOCBRIDGECHAN and
   reports the result.
9. Bridge succeeded: the drain tells the driver to release: it forgets the
   session with no authentication reply and no Terminate-Request, closes its
   channel and unit descriptors (the unit was never connected, NCP never ran).
   From here the L2TP session owns the subscriber's PPP; a later L2TP
   teardown (CDN, StopCCN) ends it and the PPPoE server sends PADT as for a
   Service-Name relay.
10. Failure at any step before the bridge succeeded (no remote, dial or
    tunnel refused, CDN, a bridge ioctl error): the driver, still holding its
    LCP state and the descriptor for writing, sends CHAP Failure (or PAP
    Authenticate-Nak) naming the realm and a Terminate-Request; the PPPoE
    server sends PADT; the sink sends CDN for any call it placed. A bridge
    ioctl error after quiesce takes this path too: the userspace relay
    fall-back `maybeBridgePPPoE` offers a Service-Name relay is not used for
    a realm call, because the driver has already answered nothing and must
    refuse.
11. No match: the drain hands the request to the auth handler as today
    (local termination).

#### Handoff ownership and cancellation

| Phase | Reads the subscriber's channel | Writes to it | Cancelled by subscriber PADT or LCP Terminate-Request | Cancelled by LNS CDN or tunnel loss | Cancelled by daemon stop or config removal |
|-------|-------------------------------|--------------|--------------------------------------------------------|-------------------------------------|---------------------------------------------|
| Auth Response received, call being placed | PPP driver (answers Echo; a retransmitted Response or Configure-Request is answered from its state, a retransmitted CHAP Response is not re-reported) | PPP driver | drain cancels the request; sink sends CDN if ICRQ was sent; no bridge | step 10 refusal | sink sends CDN; driver closes LCP with Terminate-Request; PADT |
| "Ready to bridge", driver quiescing | PPP driver until its reader returns | PPP driver | as above | step 10 refusal | as above |
| Quiesced, bridge not yet issued | nobody; the kernel queues frames on the channel | PPP driver (refusal only) | sink drops the bridge, sends CDN; PADT | step 10 refusal | as above |
| Bridged | the kernel forwards to the pppol2tp channel | the LNS, through the bridge | L2TP session torn down with CDN, as for a Service-Name relay | PADT to the subscriber | L2TP teardown, PADT |

Frames the subscriber sends while the channel is quiesced are not delivered
to the LNS: Linux queues them on the channel descriptor the driver no longer
reads, and they are discarded when it closes (A-1). PPP recovers them by
retransmission: the subscriber's Restart timer resends an unanswered
Configure-Request, and a CHAP peer answers the LNS's next Challenge
(RFC 1994 Section 4.1). The window is two ioctl calls, not a round trip.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| pppoe → l2tp | `internal/core/callsink` Request (extended) and an outcome callback | No |
| pppoe → PPP driver | `ppp.StartSession` capture flag; a new driver release call | No |
| Ze LAC ↔ LNS | ICCN AVPs 26 to 33 | No |
| Kernel | PPPIOCBRIDGECHAN only after the PPP driver's channel reader has returned (step 7 before step 8) | No |

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
| A-1 | Once PPPIOCBRIDGECHAN is set, frames on the PPPoE channel go to the bridged channel and no longer reach the `/dev/ppp` channel fd the driver read | Linux `ppp_generic.c` `ppp_input` checks the bridge first | the LAC's driver keeps consuming the subscriber's frames | `TestLACRealmBridgeAfterLocalLCP` (Linux integration, `internal/component/l2tp/bridge_realm_integration_linux_test.go`) | validated by `TestLACRealmBridgeAfterLocalLCP` |
| A-2 | A subscriber waiting for CHAP Success answers a fresh Challenge from the LNS | RFC 1994 Section 4.1 ("Additional Challenge packets MUST be sent until a valid Response packet is received"), pppd behaviour, scenario `pppoe-chap-rechallenge-ze-client` for the Ze client | the session stalls at the LNS's challenge | interop `l2tp-lac-realm-relay-accel-ppp` | confirmed: accel-ppp runs its own LCP and CHAP after the bridge and the session comes up (`tmp/session/2026-11-07-104251/scratch/rwinv/c14-accel-clean.log`, 1 passed) |
| A-3 | The Section 4.3 cipher in `hidden.go` is correct for an encrypting writer | `HiddenEncrypt` and `HiddenDecrypt` round-trip in `hidden_test.go` and `hidden_fuzz_test.go`; read 2026-11-02: no writer sets the H bit and no Random Vector is emitted, so the writer is built here (AC-3 holds, no fall-back) | AVP 33 is unreadable at the LNS | `TestWriteICCNBodyHiddenAVP33` decrypts with `HiddenDecrypt` and the tunnel secret; the xl2tpd scenario's LNS reads it (xl2tpd decrypts hidden AVPs) | confirmed: `TestWriteICCNBodyHiddenAVP33` PASS (`tmp/session/2026-11-07-104251/scratch/rwinv/out-gb3/c15-l2tp.log`); xl2tpd interop 1 passed (`rwinv/interop-1319-b.log`) |
| A-4 | The dial and the ICRQ, ICRP, ICCN exchange fit inside the subscriber's authentication patience | pppd CHAP waits several seconds per retry | subscriber retries or gives up | scenario measures nothing by time; it asserts the session comes up | confirmed: both interop scenarios bring the relayed session up under pppd's default CHAP timers |
| A-5 | Realm is the text after the last `@`, compared case-insensitively | common usage; DNS names are case-insensitive | a deployment uses `realm/user` or `user%realm` | Known Limitations | confirmed: `LookupRealmRelay` (`config.go`) folds case and the accel-ppp scenario relays `pppd@lab.example.net` by its realm; `realm/user` and `user%realm` stay a Known Limitation |

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
| `relay-realm example.net { remote lns-a }` + PPPoE subscriber `cpe@example.net` | → | pppoe drain → callsink → `relaySink` realm match → `placeIncomingCall` with proxy fields | `TestRelaySinkRealmMatchPlacesCallWithProxyLCP` (`internal/component/l2tp/relay_realm_sink_test.go`) |
| PPPoE session with capture flag | → | ppp driver capture → `EventAuthRequest` | `TestDriverCapturesProxyLCPForRelay` (`internal/component/l2tp/ppp/relay_handoff_test.go`) |
| ICCN for a realm call | → | `writeICCNBody` | `test/l2tp-wire/iccn-proxy-avps.ci` |
| `ze config validate` | → | `ExtractParameters` realm checks | `test/l2tp/relay-realm-config.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `relay-realm example.net { remote lns-a }`, subscriber authenticates as `cpe@example.net` with CHAP | LAC runs LCP, sends a Challenge, receives the Response, sends neither Success nor Failure, dials `lns-a`, places an incoming call, bridges; the subscriber's next PPP frames reach the LNS |
| AC-2 | AC-1's ICCN | Carries AVP 26 (first Configure-Request options from the subscriber), 27 (Ze's Acked Configure-Request options), 28 (the subscriber's Acked Configure-Request options), each with M=0; and AVPs 29 (type 2), 30, 31, 32 (most significant octet 0), 33 |
| AC-3 | PAP subscriber, remote with `shared-secret` | AVP 29 type 3, 30, 32 (the Authenticate-Request Identifier), 33 with the H bit set, a Random Vector AVP before it and after every unhidden AVP that precedes it in the message order Ze writes; AVP 33 decrypts with the tunnel secret to the password; no other AVP carries the H bit; the LAC sends no Authenticate-Ack |
| AC-4 | PAP subscriber, remote without `shared-secret` | AVPs 26 to 28 only; no proxy authentication; the log says why |
| AC-5 | Username with no `@`, or a realm no binding names | Local termination: the request goes to the auth handler as for any AC session |
| AC-6 | Realm matched, remote unreachable, tunnel refused, or the LNS answers CDN before the bridge | CHAP Failure (or PAP Authenticate-Nak) with a message naming the realm, LCP Terminate-Request, PADT; no local authentication |
| AC-7 | A Service-Name relay binding matches at PADS | Service-Name relay wins; no local LCP (today's path) |
| AC-8 | No `relay-realm` configured | PPPoE AC behaviour identical to today |
| AC-9 | `relay-realm` naming an undeclared remote, an empty realm, a realm with `@` | `ze config validate` refuses it with the fix; daemon start refuses it too |
| AC-10 | Two subscribers of different realms | Each reaches its own remote; one tunnel per remote, shared by sessions |
| AC-11 | Reload adds or removes a `relay-realm` | Applies to the next authentication; relayed sessions stay up |
| AC-12 | `ze show l2tp session id <id>` on the LAC for a realm call | the JSON object carries `relay-realm` (`example.net`), `relay-remote` (`lns-a`), `proxy-lcp-sent` (`true`) and `proxy-auth-sent` (`true`, or `false` with `proxy-auth-reason` `no-shared-secret` for AC-4); a Service-Name relay or a local session carries none of these keys |
| AC-13 | `docs/guide/l2tp.md` and the bridge test comment | State which tests prove the LAC bridge (child 19 corrects the existing claim); the new relay-realm section names `l2tp-lac-realm-relay-accel-ppp` and `l2tp-lac-realm-relay-xl2tpd` as the realm-path proof; no reference to `l2tp-ppp` or `gokrazy-l2tp-ppp` as a bridge proof |
| AC-14 | A frame from the subscriber arrives while the driver is quiesced (between steps 7 and 8) | it is not delivered to the LNS; the subscriber's retransmission after the bridge reaches the LNS, and the session comes up |
| AC-15 | Subscriber PADT, LNS CDN, or daemon stop arrives in each handoff phase of the table | the outcome in that row: no bridge after a cancellation, a CDN for any placed call, PADT to the subscriber, no leaked descriptor or goroutine |
| AC-16 | PPPIOCBRIDGECHAN fails after the driver quiesced | refusal per step 10 (CHAP Failure or PAP Authenticate-Nak, Terminate-Request, PADT), CDN to the LNS, no userspace relay |

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
| `TestRealmFromUsername` | `internal/component/l2tp/pppoe/realm_relay_test.go` | last `@`, case folding, no `@` | |
| `TestRelaySinkRealmMatchPlacesCallWithProxyLCP` | `internal/component/l2tp/relay_realm_sink_test.go` | wiring, AC-1 | |
| `TestRelaySinkRealmNoMatchReturnsUnaccepted` | `internal/component/l2tp/relay_realm_sink_test.go` | AC-5 | |
| `TestRelaySinkRealmDialFailureReportsRefusal` | `internal/component/l2tp/relay_realm_sink_test.go` | AC-6 | |
| `TestDriverCapturesProxyLCPForRelay` | `internal/component/l2tp/ppp/relay_handoff_test.go` | first received, final Acked pair (including after a Nak round) | |
| `TestDriverRelayReleaseSendsNoAuthReply` | `internal/component/l2tp/ppp/relay_handoff_test.go` | AC-1: no Success, Failure, Ack or Terminate on release | |
| `TestDriverRelayRefusalClosesLCP` | `internal/component/l2tp/ppp/relay_handoff_test.go` | AC-6 | |
| `TestMSCHAPv2RelayRefusalIsStructured` | `internal/component/l2tp/ppp/mschapv2_test.go` | AC-6 for an MS-CHAPv2 subscriber: the refusal reaches the peer as an RFC 2759 Section 6 Failure (M= carrying the text, C= this exchange's Authenticator Challenge), then LCP Terminate-Request | |
| `TestWriteICCNBodyProxyAVPs` | `internal/component/l2tp/rfc2661_session_initiator_test.go` | AC-2 byte layout, M=0, ID high octet 0. ICCN wiring is proven in three parts: this test proves the writer, `iccn-proxy-avps.ci` proves the decode, and the live call is the interop scenarios' | |
| `TestWriteICCNBodyPAPProxyHiddenOrAbsent` | `internal/component/l2tp/rfc2661_session_initiator_test.go` | AC-3, AC-4 | |
| `TestWriteICCNBodyHiddenAVP33` | `internal/component/l2tp/rfc2661_session_initiator_test.go` | AC-3: Random Vector precedes AVP 33, H bit on AVP 33 only, `HiddenDecrypt` with the secret yields the password (RFC2661-4.3-1, RFC2661-4.3-3) | |
| `TestDriverQuiesceStopsReaderBeforeReturning` | `internal/component/l2tp/ppp/relay_handoff_test.go` | step 7: quiesce returns only after the reader exited; no Echo answered after it | |
| `TestRelayRealmHandoffCancelledInEachPhase` | `internal/component/l2tp/pppoe/realm_relay_test.go` | AC-15, one subtest per row of the handoff table with fake sink and driver | |
| `TestLACRealmHandoffRetransmission` | `internal/component/l2tp/bridge_realm_integration_linux_test.go` (package `l2tp_test`: the PPPoE package it drives imports `l2tp`, so an in-package test would be an import cycle) | AC-14 with real pppox sockets: a frame written during quiesce is lost, its retransmission after the bridge arrives on the pppol2tp side | |
| `TestLACRealmBridgeFailureRefuses` | `internal/component/l2tp/pppoe/realm_relay_test.go` | AC-16 with a fake bridge returning an error | |
| `TestSessionJSONRelayRealmFields` | `internal/component/l2tp/cmd/l2tp_test.go` | AC-12: keys present for a realm call, absent for a Service-Name relay and a local session | |
| `TestRelayRealmConfigValidation` | `internal/component/l2tp/config_test.go` | AC-9 | |
| `TestReloadAppliesRelayRealms` | `internal/component/l2tp/subsystem_reload_test.go` | AC-11: a reload adding `relay-realm` makes the sink report realm relay configured and the realm resolve to its remote (case folded); a reload removing it reverts both | written at closure; run owed |
| `TestServiceNameRelayPrecedesRealm` | `internal/component/l2tp/pppoe/server_test.go` | AC-7 | |
| `TestLACRealmBridgeAfterLocalLCP` | `internal/component/l2tp/bridge_realm_integration_linux_test.go` (package `l2tp_test`, same import cycle) | A-1 with real pppox sockets | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| realm length | 1..253 (DNS name) | 253 | 0 (empty) | 254 |
| Proxy Authen ID | 0..255 in the low octet | 255 | N/A | high octet non-zero never written |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `relay-realm-config` | `test/l2tp/relay-realm-config.ci` | validate accepts a realm binding; refuses AC-9 cases | promoted |
| `iccn-proxy-avps` | `test/l2tp-wire/iccn-proxy-avps.ci` | decoded ICCN shows AVPs 26 to 33 with explicit hex | promoted |
| `pppoe-realm-relay-nomatch` | `test/pppoe/pppoe-realm-relay-nomatch.ci`, fixture `le test fixture pppoe/realm-nomatch` | unmatched realm terminates locally and is refused with no local users | passes in netns (`tmp/session/2026-11-07-104251/scratch/rwinv/out-gb6/pppoe-clean.log`); red with the realm match forced true in `config.go` `LookupRealmRelay`: step 3 finds no `ze_l2tp_relay_total{kind="realm",outcome="no-match"} 1` (`rwinv/out-gb6/c14-nomatch-broken.log`) |
| (AC-12 on a live call) | - | a `.ci` cannot host the PPPoE subscriber and the LNS; the `l2tp-lac-realm-relay-accel-ppp` checker runs `ze show l2tp session id <id>` on the LAC and asserts the AC-12 keys | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `l2tp-lac-realm-relay-accel-ppp` | `test/interop-topology/scenarios/l2tp-lac-realm-relay-accel-ppp/` (topology suite: the LAC needs two links), registered in `internal/le/interoplab/topology/register_wholesale.go` beside `l2tp-lns-lcp-renegotiate-pppd`, reusing its pppd subscriber node with accel-ppp in place of the Ze LNS | pppd PPPoE client; accel-ppp LNS (ignores proxy AVPs, renegotiates) | realm selection, local LCP and CHAP at Ze, bridge, IPCP from the LNS, ping across; AC-12 keys on the LAC session; a second pppd (`pppd@other.example.net`) refused locally with `no auth handler configured` and never named in the LNS log | passes: `realmRelayChain` and `accelLNSFor` (`register_wholesale.go`, `register_wholesale_accel.go`), checker `checkRealmRelayAccel` (`check_realm_relay.go`). Producers read: accel-ppp `ppp_lcp.c` `lcp_recv_conf_req` and `lcp_recv_conf_ack`, `auth_chap_md5.c` `chap_recv_response` and `chap_send_success`; pppd 2.5.5 `chap.c` `chap_handle_status`; Ze `pppoe/drain.go` `callPPPoEAuthHandler`. Clean run `1 passed` (`tmp/session/2026-11-07-104251/scratch/rwinv/c14-accel-clean.log`); broken with the realm match disabled: FAIL `pppd holds 203.0.113.1`, the ze-lac log sends `pppd@lab.example.net` to `l2tp-auth-local` (`rwinv/c14-accel-broken.log`) |
| `l2tp-lac-realm-relay-xl2tpd` | `test/interop-topology/scenarios/l2tp-lac-realm-relay-xl2tpd/`, registered the same way | pppd PPPoE client; xl2tpd 1.3.20 and pppd 2.5.5 LNS (image `ze-radius-subscriber-lac`) | the same against a second LNS implementation: xl2tpd logs the call established from the LAC's ICCN, and its pppd runs its own LCP and CHAP | passes: `xl2tpdLNS` (`register_wholesale.go`), checker `checkRealmRelayXL2TPD` (`check_realm_relay.go`). Producers read: xl2tpd `control.c` `control_finish` (ICCN, "Call established with"), `avp.c` (no handler for AVPs 26 to 28, `ignore_avp` for 29 to 33), `aaa.c` `get_secret`, `misc.c` `l2tp_log`; pppd 2.5.5 `utils.c` `dump_packet`, `chap.c` `chap_print_pkt`, `ipcp.c` `ipcp_up`. Clean run `1 passed`; broken with `maybeBridgePPPoE` (`bridge_linux.go`) returning `true, nil` without calling `bridgeChannels`: FAIL `pppd holds 203.0.113.1` (`tmp/session/2026-11-07-104251/scratch/rwinv/interop-1319-b.log`) |
| `access-wholesale` | child 19 | Ze CPE, Ze LAC, Ze LNS | umbrella variant | |

Each scenario is broken once (realm match disabled; bridge skipped) and the
red is recorded. These scenarios need two links per node, so they live in
child 18's topology suite, not in `test/interop-l2tp/`.
`l2tp-lns-lcp-renegotiate-pppd`, already registered in `register_wholesale.go`,
is child 15's scenario (pppd subscriber, Ze LAC, Ze LNS with `proxy-lcp
renegotiate`) and is not a substitute for these two.

## Files to Modify
- `internal/component/l2tp/yang/ze-l2tp-conf.yang` - `list relay-realm`
- `internal/component/l2tp/config.go` - parse and validate
- `internal/core/callsink/callsink.go` - realm request fields, outcome callback
- `internal/component/l2tp/relay_sink.go` - realm matching, outcome reporting
- `internal/component/l2tp/session_initiator.go` - `callParams` proxy fields, `writeICCNBody`
- `internal/component/l2tp/avp_compound.go` - callers for the proxy writers
- `internal/component/l2tp/avp.go`, `hidden.go` - a hidden-AVP writer (H bit, `HiddenEncrypt` into the message buffer) and a Random Vector AVP writer
- `internal/component/l2tp/snapshot.go` `SessionSnapshot`, `internal/component/l2tp/cmd/l2tp.go` `sessionJSON` - the AC-12 keys
- `internal/component/l2tp/bridge_linux.go` - report bridge outcome
- `internal/component/l2tp/pppoe/server.go` - start local PPP with capture when realm bindings exist
- `internal/component/l2tp/pppoe/drain.go` - realm decision before the auth handler
- `internal/component/l2tp/ppp/start_session.go`, `ppp/session_run.go`, `ppp/manager.go`, `ppp/auth_events.go` - capture, release, refusal
- `docs/guide/l2tp.md` - relay-realm section naming this spec's scenarios as its proof (the existing bridge-proof sentence is child 19's correction)
- `docs/guide/pppoe.md` - relay note
- `docs/architecture/l2tp/followup-l2tp-call.md` - realm relay flow
- `rfc/short/rfc2661.md` - rows 4.4.5-1 to -6 annotations; RFC2661-4.3-1 and -4.3-3 annotations (hidden AVP 33); new M-bit rows for AVPs 26 to 28; row 4.4-2 annotation corrected
- `features/l2tpv2-bng.md`, `features/pppoe-access.md` - LAC realm relay
- `docs/architecture/l2tp/bng-5-pppoe.md` (declared by `pppoe/server.go`) - gains the realm-relay start: with realm bindings, an AC session no Service-Name relay took starts local PPP with capture. Nothing it states becomes wrong: its Service-Name text describes discovery, not relay
- `docs/architecture/l2tp/subscriber-session-model.md` (declared by `pppoe/drain.go`) - the drain section (`startPPPoEAuthDrain`) gains the realm decision before the auth handler; a relayed session never reaches the handler, so "PPP records the request username only when the authentication handler accepts the request" needs the relayed case named (no username is recorded, the LAC hands the session off)
- `docs/architecture/wire/l2tp.md` (declared by `avp_compound.go`) - nothing becomes wrong: its package-surface table already lists `ProxyAuthenIDValue`; the ICCN AVP layout diagram added under RFC Documentation goes in this page
- `docs/research/l2tpv2-implementation-guide.md` (declared by `session_initiator.go`) - nothing becomes wrong: Section 18 describes AVPs 26 to 33 from the RFC, not Ze's behaviour; its "last sent / last received" wording matches this spec's Acked pair
- `docs/research/l2tpv2-ze-integration.md` (declared by `config.go`, `ppp/start_session.go`, `ppp/session_run.go`, `ppp/manager.go`, `ppp/auth_events.go`) - nothing becomes wrong: Section 2 lists LCP and authentication as userspace work, which a realm LAC still does; Section 5's schema sketch never listed `relay`, so `relay-realm` adds no contradiction

## Files to Create
- `test/l2tp/relay-realm-config.ci` (drafted at `test/draft/l2tp/relay-realm-config.ci`)
- `test/l2tp-wire/iccn-proxy-avps.ci` (drafted at `test/draft/l2tp-wire/iccn-proxy-avps.ci`)
- `test/pppoe/pppoe-realm-relay-nomatch.ci`, with the fixture that takes a session past PADS
- `internal/component/l2tp/pppoe/realm_relay_test.go` - the realm unit tests
- `internal/component/l2tp/bridge_realm_integration_linux_test.go` - the realm bridge integration tests, in package `l2tp_test`; `internal/component/l2tp/export_realm_bridge_linux_test.go` (package `l2tp`) exports what they reach
- `test/interop-topology/scenarios/l2tp-lac-realm-relay-accel-ppp/`
- `test/interop-topology/scenarios/l2tp-lac-realm-relay-xl2tpd/`

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
| 3 | CLI command added/changed? | Yes | `show l2tp session id <id>` gains the AC-12 keys: `docs/guide/l2tp.md` "Read commands" and `docs/guide/command-reference.md` |
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
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derived with `./le spec citation anchors spec plan/release/fixes/spec-home-cpe-14-lac-realm-relay.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the `remote`/`relay` example in `docs/guide/l2tp.md` "Initiator: dial targets and outgoing calls" |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- YANG list, `Parameters`,
   callsink fields, sink realm match stub, drain hook; failing wiring tests.
   - Tests: `TestRelaySinkRealmMatchPlacesCallWithProxyLCP`,
     `relay-realm-config`
   - Files: YANG, `config.go`, `callsink`, `relay_sink.go`, `pppoe/drain.go`
2. **Phase: Capture** -- driver records the Configure-Requests; capture flag
   from `server.go`.
3. **Phase: ICCN** -- `callParams` proxy fields, `writeICCNBody`, the
   hidden-AVP and Random Vector writers, hidden AVP 33 for PAP; wire `.ci`.
4. **Phase: Hand-off** -- outcome callback, quiesce, bridge, release, refusal
   and cancellation per the handoff table (AC-14 to AC-16), Linux
   integration tests; the AC-12 show keys.
5. **Phase: Docs, RFC rows, interop** -- guide, architecture doc, summary
   rows, two scenarios with a recorded red.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at a named function |
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
| AVP writer ownership: this spec delivers every LAC-side writer, AVPs 26 to 28, AVPs 29 to 33, the hidden-AVP writer and the Random Vector writer, with rows RFC2661-4.4.5-1 to -6, RFC2661-4.3-1 and RFC2661-4.3-3 on the sending side. `spec-l2tp-proxy-authentication.md` keeps the LNS half only: consuming AVPs 29 to 33, decrypting a received hidden AVP, the `proxy-authentication` leaf and row RFC2661-9.5-2 | Wait for that spec to emit them | That spec's LAC half needs an authentication exchange at the LAC, which only this spec creates, so nothing of it waits on that spec. The narrowing of that spec's Task to the LNS half is listed in the umbrella's edits owed outside the home-CPE specs |
| PAP proxy data sent only hidden; without a tunnel secret no proxy authentication for PAP | Send cleartext | RFC 2661 Section 4.4.5: "In the case of cleartext passwords, AVP hiding is recommended"; the LNS restarts authentication instead, which the RFC states as the result of an absent Proxy Authen Type |
| The driver keeps answering LCP Echo until the outcome arrives | Release at the auth Response | Keeps the subscriber's keepalive alive across the dial; bounded by the authentication timeout |
| Handoff order: the driver's channel reader stops and returns before PPPIOCBRIDGECHAN; the driver releases only after the bridge succeeded | Bridge first, then stop the driver (the earlier step 6 wording) | Bridging first leaves two readers racing for the same frames until the driver stops, and a bridge that succeeds while the driver still answers can put two replies on the wire. Quiescing first gives one owner at every instant, and keeping the driver's LCP state until the bridge succeeds lets a failed bridge still refuse the subscriber properly (RFC 1661 Section 3.5) |
| A realm call never uses the userspace relay fall-back of `maybeBridgePPPoE` | Use it as a Service-Name relay does | The fall-back exists for a kernel without PPPIOCBRIDGECHAN; Ze's kernel has it, and a realm call that reaches a half-working relay after the driver went quiet cannot refuse the subscriber any more |
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

### Goal Gates (MUST pass)
- [ ] AC-1..AC-16 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass: the scoped package tests for `internal/component/l2tp/...` and `internal/core/callsink`, `./le test functional l2tp`, `./le test pppoe`, the `l2tp-wire` `.ci` above, and `./le verify changed` over the closing commits. The full verification is Thomas's release check
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
- [ ] **Commit B:** `remove plan/release/fixes/spec-home-cpe-14-lac-realm-relay.md` only, in the same `./le commit create` script

## Blocker

Closure stops before commit A for two pieces of evidence the main thread owns
(the closure agent may not run tests or guests):

| What | Producer | What clears it |
|------|----------|----------------|
| The RFC2661-6.1-1 discrimination record is stale (producer sha `7b911f10b735d280` no longer matches) | `internal/component/l2tp/tunnel_fsm.go` `handleSCCRQ`, unit `test/l2tp/rfc2661-sccrq-mandatory-avp.ci` | `./le rfc record id RFC2661-6.1-1 polarity positive unit test/l2tp/rfc2661-sccrq-mandatory-avp.ci route revert producer internal/component/l2tp/tunnel_fsm.go::handleSCCRQ citation 'expect=stderr:contains=new tunnel created from SCCRQ'` on Linux (guest batch 7, `rwinv/out-gb7/rec-2661.log`) |
| `TestReloadAppliesRelayRealms` (AC-11, closure review ISSUE) is written but has not run | `internal/component/l2tp/subsystem_reload.go` `Reload` (relay-realm swap), `extractFromProvider` | `./le job run -- go test ./internal/component/l2tp -run 'TestReloadAppliesRelayRealms$' -count=1` green |

When both are in, set Status back to `in-progress` and resume closure at the
Review Gate record (`./le spec review record`), `./le rfc check`, `./le doc
check all`, then commits A and B.

## Implementation Summary

### What Was Implemented
- Realm relay at the LAC: `relay-realm` list (`yang/ze-l2tp-conf.yang`, `config.go` `validateRelayRealms`, `LookupRealmRelay`), PPPoE auth drain routing a matched realm to the relay sink (`pppoe/drain.go`, `pppoe/realm_relay.go` `decide`), proxy LCP capture and relay handoff in the PPP driver (`ppp/proxy_capture.go`, `ppp/relay_handoff.go`), ICCN proxy AVPs 26 to 33 with hidden AVP 33 (`iccn_proxy.go`), the bridge (`bridge_linux.go`, `kernel_linux.go`), AC-12 session JSON keys (`snapshot.go`, `cmd/l2tp.go`). Commits 79e3205310, b6e77fe607 (interop), e3dc7b57f5, 119e88db46, b049c10915; guide e295e416f9.
- Closure: `TestReloadAppliesRelayRealms` (`subsystem_reload_test.go`) for AC-11.

### Bugs Found/Fixed
- AC-11 had no test (closure review ISSUE): fixed by `TestReloadAppliesRelayRealms`, run owed (Blocker).

### Documentation Updates
- `docs/guide/l2tp.md` "Relay by realm": the stale "under construction" paragraph replaced by what proves the path (e295e416f9).

### Deviations from Plan
- Test files differ from the plan's names: realm sink tests live in `relay_realm_sink_test.go`, driver handoff tests in `ppp/relay_handoff_test.go`, ICCN tests in `rfc2661_session_initiator_test.go`. The TDD and Wiring rows now name them.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The implementation phase left `docs/guide/l2tp.md` saying no test had run the realm path | Both interop scenarios and the Linux bridge tests had run | Closure documentation review | Guide fixed in e295e416f9 |
| approach | AC-11 (reload) shipped with no test | `Reload` swaps `RelayRealms`, unproven | Closure review | `TestReloadAppliesRelayRealms` added |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Realm-selected LNS with LAC-side LCP and auth | Done | `pppoe/realm_relay.go` `decide`, `relay_sink.go` `RelayRealm` | |
| Proxy LCP and proxy auth AVPs in ICCN | Done | `iccn_proxy.go` | |
| Guide claim corrected | Done | `docs/guide/l2tp.md` | e295e416f9 |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestRelaySinkRealmMatchPlacesCallWithProxyLCP`, `TestDriverRelayReleaseSendsNoAuthReply`, interop `l2tp-lac-realm-relay-accel-ppp` | |
| AC-2 | Done | `TestWriteICCNBodyProxyAVPs`, `test/l2tp-wire/iccn-proxy-avps.ci` | |
| AC-3 | Done | `TestWriteICCNBodyHiddenAVP33`, `TestWriteICCNBodyPAPProxyHiddenOrAbsent` | |
| AC-4 | Done | `TestWriteICCNBodyPAPProxyHiddenOrAbsent` | |
| AC-5 | Done | `TestRelaySinkRealmNoMatchReturnsUnaccepted`, `test/pppoe/pppoe-realm-relay-nomatch.ci` | |
| AC-6 | Done | `TestRelaySinkRealmDialFailureReportsRefusal`, `TestDriverRelayRefusalClosesLCP`, `TestMSCHAPv2RelayRefusalIsStructured` | |
| AC-7 | Done | `TestServiceNameRelayPrecedesRealm` | |
| AC-8 | Done | `TestServiceNameRelayPrecedesRealm`, `relaySink.RealmRelayConfigured` false path | |
| AC-9 | Done | `TestRelayRealmConfigValidation`, `test/l2tp/relay-realm-config.ci` | |
| AC-10 | Done | `TestRelaySinkRealmMatchPlacesCallWithProxyLCP` | |
| AC-11 | Partial | `TestReloadAppliesRelayRealms` | run owed (Blocker) |
| AC-12 | Done | `TestSessionJSONRelayRealmFields`, accel-ppp checker `checkRealmRelayAccel` | |
| AC-13 | Done | `docs/guide/l2tp.md` | |
| AC-14 | Done | `TestLACRealmHandoffRetransmission` (QEMU, `rwinv/out-qemu/clean-l2tp.log`) | |
| AC-15 | Done | `TestRelayRealmHandoffCancelledInEachPhase` | |
| AC-16 | Done | `TestLACRealmBridgeFailureRefuses` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| every TDD row | Done | names in the TDD table; each function found by `grep -rl 'func <name>(' internal/` on 2026-11-09 | `TestReloadAppliesRelayRealms` run owed |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Files to Modify / Create | Done | landed in the commits named above |

### Audit Summary
- **Total items:** 16 AC
- **Done:** 15
- **Partial:** 1 (AC-11, run owed; not a scope reduction)
- **Skipped:** 0
- **Changed:** test file names (Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A subscriber's realm selects the LNS and the session comes up through it | interop | `l2tp-lac-realm-relay-accel-ppp` 1 passed, red with the realm match disabled (`rwinv/c14-accel-{clean,broken}.log`); `l2tp-lac-realm-relay-xl2tpd` 1 passed, red with the bridge skipped (`rwinv/interop-1319-b.log`) |
| The kernel bridge carries the subscriber's frames after local LCP | Linux integration | `TestLACRealmBridgeAfterLocalLCP`, `TestLACRealmHandoffRetransmission` PASS (`rwinv/out-qemu/clean-l2tp.log`), both FAIL with `bridgeChannels` not issuing the ioctl (`rwinv/out-qemu/c14-bridgechannels-no-ioctl.log`) |
| An unmatched realm terminates locally | functional | `pppoe-realm-relay-nomatch` PASS / red (`rwinv/out-gb6/pppoe-clean.log`, `c14-nomatch-broken.log`) |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | | |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | not recorded: closure blocked before the record (Blocker) |
| `./le spec review check` | not run |
| Rounds | 1 (closure agent 1 over the committed diff); round 2 owed after `TestReloadAppliesRelayRealms` runs |
| Reviewer lenses used | logic+wiring, security (no password logged in `RelayRealm`/`decide`), RFC 2661 Section 4.3 hiding, handoff ordering |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | AC-11 has no test | `subsystem_reload.go` `Reload` | `TestReloadAppliesRelayRealms` (`subsystem_reload_test.go`), run owed |

NOTEs (record only): `pppoe/realm_relay.go` `decide` registers the handoff in
`r.calls` only after `RelayRealm` returns, so a PADT in that window finds no
handoff; the quiesce of a dropped session fails and `kernel_linux.go` CDNs the
call. The YANG realm pattern relies on anchoring; `validateRelayRealms` refuses
`@` and whitespace regardless (`test/l2tp/relay-realm-config.ci`). Not yet
reviewed: `ppp/relay_handoff.go`, `ppp/relay_reader_linux.go`, `callsink.go`,
`snapshot.go`/`cmd` JSON, `check_realm_relay.go`.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/l2tp/relay-realm-config.ci`, `test/l2tp-wire/iccn-proxy-avps.ci`, `test/pppoe/pppoe-realm-relay-nomatch.ci` | yes | `ls` 2026-11-09 |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-2, AC-3 | ICCN proxy AVPs and hidden 33 | `TestWriteICCNBodyProxyAVPs`, `TestWriteICCNBodyHiddenAVP33` PASS in `rwinv/out-gb3/c15-l2tp.log` |
| AC-11 | reload applies relay-realm | owed (Blocker) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze config validate` | `test/l2tp/relay-realm-config.ci` | exists |
| ICCN decode | `test/l2tp-wire/iccn-proxy-avps.ci` | exists |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `TestLACRealmBridgeAfterLocalLCP` PASS, red without the ioctl |
| A-2 | confirmed | accel-ppp interop 1 passed |
| A-3 | confirmed | `TestWriteICCNBodyHiddenAVP33` PASS; xl2tpd interop 1 passed |
| A-4 | confirmed | both interop scenarios pass under default pppd timers |
| A-5 | confirmed | `LookupRealmRelay` EqualFold; accel-ppp relays `pppd@lab.example.net` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/guide/l2tp.md` "Relay by realm" | anchored to `check_realm_relay.go` `checkRealmRelayAccel`/`checkRealmRelayXL2TPD` and `bridge_realm_integration_linux_test.go` | yes (e295e416f9); `./le doc check all` owed at resume |
