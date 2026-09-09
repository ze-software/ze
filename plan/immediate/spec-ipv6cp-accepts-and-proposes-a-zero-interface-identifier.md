# Spec: ipv6cp-accepts-and-proposes-a-zero-interface-identifier

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | plan/spec-l2tp-ipv6-subscriber.md |
| Phase | 5/5 |
| Handoff | - |
| Updated | 2026-09-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**IPv6CP reaches Opened with the peer's interface identifier still all-zero, and
nothing downstream can tell that zero from a negotiated value.**

`evalIPv6CPRequest` returns an acceptable verdict when the peer's Configure-Request
carries no Interface-Identifier option at all, so Ze Acks it. RFC 5072 Section 4.1
states that "A Configure-Request MUST contain exactly one instance of the
interface-identifier option". `pppSession.peerInterfaceID` is a bare 8-byte array
with no companion flag, and it is assigned on only two paths: an accepted request
that carried the option, and a pool handler reply that set `hasPeerInterface`.
Neither runs in this case, so the field keeps its zero value and the session
proceeds as if negotiation had succeeded.

Two consequences follow, and both are fail-open in the sense of
`ai/rules/principles.md`.

First, the Nak path proposes the zero. `buildNakOrReject` answers an unacceptable
request with a Configure-Nak carrying `peerInterfaceID`. When the peer's own first
request carried a zero identifier, Ze's `isValidIPv6CPInterfaceID` rejects it and
the Nak then proposes Ze's stored zero. RFC 5072 Section 4.1 requires the opposite:
"If the two interface identifiers are different but the received interface
identifier is zero, a Configure-Nak is sent with a non-zero interface-identifier
value suggested for use by the remote peer." Ze transmits a value its own validator
would refuse.

Second, the zero is used as an address. `peerLinkLocal` turns it into `fe80::`,
`installRoute` installs that as the next hop for the delegated prefix, and
`onNCPOpened` publishes it in the session-up event. The subscriber is terminated
onto an address that is not theirs.

The same section also fixes two cases Ze does not implement: identifiers that are
equal and non-zero take a Configure-Nak with a different non-zero suggestion, and
identifiers that are both zero take a Configure-Reject with the identifier set to
zero, which ends the negotiation. Ze answers the first with a Nak carrying a
possibly-stale value and does not distinguish the second at all.

**Reachability: latent today, and this spec exists so the fix precedes the path
going live.** The only registered pool handler answers every non-IPv4 request with
`Accept: false`, `runNCPPhase` reads that decline and sets `disableIPv6CP`, and
`evalIPv6CPRequest` is therefore unreachable in a shipped daemon.
`plan/spec-l2tp-ipv6-subscriber.md` is the work that makes it reachable, and this
spec is named in the `Depends` row above for that reason.

The goal: a zero peer interface identifier must be impossible to confuse with a
negotiated one, every RFC 5072 Section 4.1 comparison case must be implemented,
and no address derived from an unnegotiated identifier may reach the kernel or an
event.

## Required Reading

### Architecture Docs
- [ ] `docs/research/l2tpv2-ze-integration.md` - the design document every file in this spec declares: the NCP coordinator, the IPv6CP codec, the IPv6 service lifecycle and per-session state ownership
  → Decision: each NCP is independent, and a declined or failed IPv6CP must not tear down a good IPv4 session, so a refusal here ends IPv6CP alone
  → Constraint: per-session state lives on `pppSession` and is owned by the session goroutine, so a "peer identifier is negotiated" fact belongs on that struct rather than in a package-level map
  → Constraint: the page is silent on the Section 4.1 comparison cases and on what an unnegotiated identifier means downstream, so it gains both statements in this work
- [ ] `docs/architecture/l2tp/bng-5-pppoe.md` - the AC that carries PPP sessions for PPPoE subscribers, so the same negotiation runs under the access concentrator
  → Constraint: a PPPoE subscriber and an L2TP subscriber share this PPP code, so a fix here must not assume an L2TP tunnel exists

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc5072.md` - IPv6 over PPP: the Interface-Identifier option and its negotiation
  → Constraint: RFC 5072 Section 4.1: "A Configure-Request MUST contain exactly one instance of the interface-identifier option". A request with none is not conformant and must not be Acked as if it were
  → Constraint: RFC 5072 Section 4.1: "If the two interface identifiers are different but the received interface identifier is zero, a Configure-Nak is sent with a non-zero interface-identifier value suggested for use by the remote peer. Such a suggested interface identifier MUST be different from the interface identifier of the last Configure-Request sent to the peer."
  → Constraint: RFC 5072 Section 4.1: "If the two interface identifiers are equal and are not zero, Configure-Nak MUST be sent specifying a different non-zero interface-identifier value suggested for use by the remote peer."
  → Constraint: RFC 5072 Section 4.1: "If the two interface identifiers are equal to zero, the interface identifier's negotiation MUST be terminated by transmitting the Configure-Reject with the interface-identifier value set to zero."
  → Constraint: RFC 5072 Section 4.1: "The 'u' (universal/local) bit of the suggested identifier MUST be set to zero (0) regardless of its source unless the globally unique EUI-48/EUI-64 derived identifier is provided for the exclusive use by the remote peer."
- [ ] `rfc/short/rfc1661.md` - the PPP option negotiation automaton that carries these packets
  → Constraint: RFC 1661 Section 6 governs what a Configure-Reject means: the option is not negotiable and must not appear again, so rejecting the identifier ends IPv6CP rather than looping

### Other Implementations (read 2026-09-08, source quoted)
- [ ] accel-ppp `accel-pppd/ppp/ipv6cp_opt_intfid.c` and `accel-pppd/ppp/ppp_ipv6cp.c`
  → Decision: its Nak suggests a fixed configured value, `opt64->val = ipv6cp->ppp->ses.ipv6->peer_intf_id`, whose default is `INTF_ID_FIXED` with the value 2, so it offers `::2` against its own `::1`. Ze's `generateIPv6CPInterfaceID` already draws from `crypto/rand`, which is the stronger shape, so Ze keeps its generator rather than adopting a constant
  → Constraint: it ACKS a Configure-Request carrying no interface-identifier option, in `ipv6cp_recv_conf_req`, whose walk covers only received options and whose local-option block is commented out. That is exactly Ze's current behavior, so Ze is not alone in it and the change is a deliberate move to pppd's reading
  → Constraint: it enforces difference from its own identifier only on the ACK test, never on the value it suggests, so the suggestion can collide. Ze enforces it on the suggestion
- [ ] pppd `pppd/ipv6cp.c`, `ipv6cp_reqci` and `ipv6cp_nakci`
  → Decision: the suggested value is random and looped until valid: `while (eui64_iszero(ifaceid) || eui64_equals(ifaceid, go->ourid)) eui64_magic(ifaceid);`. That is the shape this spec adopts, and Ze's generator plus a rejection of the local identifier reproduces it
  → Constraint: both identifiers zero takes a Configure-Reject, which matches RFC 5072 Section 4.1 and this spec's AC-4
  → Constraint: **a request with no option is Naked ONCE, not forever.** `ipv6cp_reqci` appends the option under `!ho->neg_ifaceid && wo->req_ifaceid`, then clears `wo->req_ifaceid` so it never asks again. Without that one-shot guard, a peer that never sends the option and a Ze that always Naks it negotiate forever. This spec owes the guard, and AC-10 is it
  → Constraint: neither implementation clears the "u" bit on the value it suggests. RFC 5072 Section 4.1 addresses that MUST to the implementation making the suggestion, so Ze clears it and is more conformant than both

**Key insights:**
- The defect is a missing fact, not a missing check: nothing on the session records whether the peer identifier was ever negotiated, so every reader of the field is guessing.
- The missing-option case has two live answers in the field: accel-ppp Acks, pppd Naks once and then stops asking. Naking forever is neither, and it is what this spec would have built without the comparison.
- Ze's own `isValidIPv6CPInterfaceID` already rejects all-zero and all-ones. The transmit path does not consult it, which is why Ze can send a value it would refuse to receive.
- The comparison in Section 4.1 is against "the interface identifier of the last Configure-Request sent to the peer", so the local identifier Ze last requested is part of the state the decision reads.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/ppp/ncp.go` - `runNCPPhase` disables IPv6CP when the handler declines; `requestIPv6CPInterfaceID` generates the local identifier and stores a peer identifier only when the reply carries `hasPeerInterface`; `evalIPv6CPRequest` returns acceptable when the request carries no identifier option, unacceptable when the value fails `isValidIPv6CPInterfaceID` or equals the local identifier, and stores the value otherwise; `buildNakOrReject` answers with a Configure-Nak carrying `peerInterfaceID`; `onNCPOpened` publishes the session's addresses
- [ ] `internal/component/l2tp/ppp/ipv6cp.go` - the option codec: `parseIPv6CPOptions`, `writeIPv6CPOptions`, `isValidIPv6CPInterfaceID` (rejects all-zero and all-ones), `generateIPv6CPInterfaceID`
- [ ] `internal/component/l2tp/ppp/session.go` - `pppSession.peerInterfaceID` and `localInterfaceID` are bare 8-byte arrays with no negotiated flag
- [ ] `internal/component/l2tp/ppp/ipv6_service.go` - `peerLinkLocal` builds `fe80::` plus the identifier with no validity test; `installRoute` uses it as the next hop; `handleSolicit`, `handleRequest`, `handleRenew` and `handleRelease` nil-check the DHCPv6 fields but not the identifier
- [ ] `internal/component/l2tp/ppp/session_run.go` - `afterLCPOpen` starts the IPv6 service only when `ipv6cpState` is Opened, and passes the identifiers into the service config
- [ ] `internal/component/l2tp/plugins/pool/register.go` - `poolPlugin.handle` answers every non-IPv4 family with `Accept: false` and the reason "IPv6 not supported by static pool", which is what makes this path latent today
- [ ] `rfc/full/rfc5072.txt` - Section 4.1, read in full for the four comparison cases and the "u" bit rule

**Behavior to preserve:**
- A declined IPv6CP continues the session IPv4-only. An independent NCP must not tear down a good session.
- `isValidIPv6CPInterfaceID` keeps rejecting all-zero and all-ones on receive.
- The local identifier is generated once per session by `generateIPv6CPInterfaceID` and does not change under renegotiation.
- The option codec's bounds behavior is unchanged: `parseIPv6CPOptions` already rejects a length below 2 and a length past the buffer.

**Behavior to change:**
- A Configure-Request carrying no Interface-Identifier option is no longer Acked as acceptable.
- A Configure-Nak never proposes a zero identifier, and its suggestion differs from the last identifier Ze requested.
- The equal-and-zero case is answered with a Configure-Reject carrying a zero identifier, which ends the identifier negotiation.
- The session records whether the peer identifier was negotiated, and every downstream reader consults that fact rather than the value.
- No address derived from an unnegotiated identifier reaches `installRoute` or the session-up event.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An IPv6CP Configure-Request from the subscriber, carried inside a PPP frame on an L2TP session or a PPPoE session, read by the session's frame reader and dispatched by the NCP coordinator.
- Format at entry: a PPP control packet, protocol `0x8057`, code 1, whose data field is a list of TYPE(1) LENGTH(1) VALUE options, where type 1 is the 8-byte Interface-Identifier.

### Transformation Path
1. `ParseLCPPacket` (`lcp.go`) validates the header length and slices the data field to the declared length.
2. `evalIPv6CPRequest` (`ncp.go`) scans the options, parses them, and returns a verdict.
3. `buildNakOrReject` (`ncp.go`) turns an unacceptable verdict into a Configure-Nak or a Configure-Reject.
4. On Opened, `afterLCPOpen` (`session_run.go`) starts the IPv6 service with the local and peer identifiers.
5. `peerLinkLocal` and `installRoute` (`ipv6_service.go`) derive the peer's link-local address and install the delegated prefix route through it.
6. `onNCPOpened` (`ncp.go`) publishes `EventSessionIPAssigned` to the plugin bus.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire → PPP | IPv6CP control packet parsed by `ParseLCPPacket` and `parseIPv6CPOptions` | No |
| PPP → address handler | `EventIPRequest` with family IPv6, answered by a registered pool or RADIUS handler | No |
| PPP → kernel | The delegated prefix route installed through the peer's link-local address | No |
| PPP → plugin bus | `EventSessionIPAssigned` carrying the addresses a plugin acts on | No |

### Integration Points
- `pppSession` (`session.go`) - the negotiated fact lives beside the identifier it qualifies.
- `evalIPv6CPRequest` and `buildNakOrReject` (`ncp.go`) - the single decision point for the Section 4.1 cases.
- `IPv6Service` (`ipv6_service.go`) - refuses to start, or starts without a route, when the identifier was never negotiated.
- `poolPlugin.handle` (`internal/component/l2tp/plugins/pool/register.go`) - the handler whose IPv6 decline keeps this path latent; `plan/spec-l2tp-ipv6-subscriber.md` owns changing it.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The verdict is still decided once, in `evalIPv6CPRequest` (`ncp.go`), and turned into a packet once, in `buildNakOrReject` (`ncp.go`). `ipv6_service.go` gained no branch: it consumes `iPv6ServiceConfig.PeerInterfaceID`, populated only from the now-guarded `afterLCPOpenIPv6Service` (`session_run.go`) |
| No unintended coupling (components stay isolated) | Yes | Every new symbol is unexported and inside `internal/component/l2tp/ppp`. `./le repository check` names no symbol in this package |
| No duplicated functionality (extends existing, does not recreate) | Yes | `suggestIPv6CPInterfaceID` (`ipv6cp.go`) calls the existing `generateIPv6CPInterfaceID` and the existing `isValidIPv6CPInterfaceID` rather than reimplementing either; the counter uses `registry.InjectPluginMetrics`, the same route `internal/component/l2tp/pppoe/metrics.go` already takes |
| Zero-copy preserved where applicable (refs, not copies) | Yes | The Nak and Reject are written into the caller's frame buffer by `writeIPv6CPOptions(buf, off, ...)`; the equal-zero Reject echoes `req.Data` with one `copy` into that same buffer. No allocation is added on the packet path; `suggestIPv6CPInterfaceID` returns an `[8]byte` value |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | Nothing registers a command or a family. The one registration is the metrics hook, through `registry.InjectPluginMetrics(metricsHookName, bindPPPMetrics)` from `Driver.Start` (`manager.go`), so no central list names this counter |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `evalIPv6CPRequest` is unreachable in a shipped daemon today | `poolPlugin.handle` declines every non-IPv4 family; `runNCPPhase` sets `disableIPv6CP` on that decline | The defect is live rather than latent and the bucket judgement was too generous | `gopls references` on `RegisterPoolHandler` and on `evalIPv6CPRequest`, plus a QEMU run with a peer that offers IPv6CP | unvalidated |
| A-2 | No peer Ze must interoperate with sends a Configure-Request with no Interface-Identifier option as normal practice | RFC 5072 Section 4.1 requires exactly one instance | Refusing such a request breaks a real client, and the answer becomes a Nak carrying a suggestion rather than a rejection | The interop scenario against accel-ppp and against pppd 2.5.1, with the option omitted | unvalidated |
| A-3 | The local identifier Ze last requested is available at the point the Nak is built, so the "MUST be different from the interface identifier of the last Configure-Request sent" rule can be enforced | `localInterfaceID` is set once by `requestIPv6CPInterfaceID` before the FSM starts | The suggestion cannot be proven different and the rule is unenforceable without new state | Read `requestIPv6CPInterfaceID` and `startNCP` ordering, and assert it in a unit test | confirmed -- `buildNakOrReject` (`ncp.go`) already has `s.localInterfaceID` in scope at the point it builds the Nak (no new state needed); `suggestIPv6CPInterfaceID` (`ipv6cp.go`) takes it as a parameter and rejects a redraw equal to it, and `TestIPv6CPSuggestionDiffersFromLocalIdentifier` asserts the rule holds over 64 draws |
| A-4 | Nothing outside `ipv6_service.go` and `ncp.go` reads `peerInterfaceID` | `gopls references` on the field | A reader elsewhere keeps consuming the raw zero after this fix | `gopls references` on `peerInterfaceID`, each caller read | broken (harmlessly) -- `gopls references` on `pppSession.peerInterfaceID` (session.go) returns exactly 5, not the assumed 2 files: `ncp.go` writers at `requestIPv6CPInterfaceID` and `evalIPv6CPRequest`, `ncp.go` readers at `buildNakOrReject` (deferred to phase 3, unchanged) and `onNCPOpened` (now gated on `peerInterfaceIDNegotiated`), and one reader in `session_run.go`: `afterLCPOpen` (now gated). `ipv6_service.go`/`ipv6_service_linux.go` read no field of `pppSession` at all -- they consume `iPv6ServiceConfig.PeerInterfaceID`, a copy populated only from the now-gated `afterLCPOpen` call, so they are safe by construction without their own copy of the fact. The assumption under-named `session_run.go`, but the reader set is closed and every member is now accounted for |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Rejecting a request with no identifier option ends IPv6CP for a peer that would have converged, so a subscriber loses IPv6 they used to get | The interop scenario's IPv6CP never opens | The verdict is a Configure-Nak carrying a non-zero suggestion, which invites the peer to send the option, rather than a Reject |
| R-2 | The suggested identifier collides with the local one, or a peer that will not send the option is Naked forever, so the two ends never converge | The FSM hits its Configure-Nak counter and IPv6CP fails | The suggestion is drawn from the generator and rejected when it equals the local identifier, which is pppd's loop condition, and a missing option is Naked once and then Acked (AC-10), which is pppd's one-shot guard |
| R-3 | A negotiated flag is added but a downstream reader is missed, so the zero still reaches the kernel on one path | A QEMU test sees `fe80::` installed as a next hop | Every reader found by `gopls references` is listed in Files to Modify, and the service refuses to start rather than starting without the route |
| R-4 | The equal-and-zero Configure-Reject terminates IPv6CP where the peer expected a Nak, and the peer drops the whole session | The peer sends LCP Terminate after the Reject | The Reject is what RFC 5072 Section 4.1 requires; the interop scenario proves the peer's reaction, and the local session stays IPv4-up either way |
| R-5 | The fix lands before `plan/spec-l2tp-ipv6-subscriber.md` makes the path reachable, so no functional test can drive it end to end | The wiring test has no live entry point | The unit tests drive `evalIPv6CPRequest` and `buildNakOrReject` directly, and the functional and interop rows are written against the enabling spec and named in its text |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A subscriber's IPv6CP fails to converge, or a route is installed toward an address that is not the peer's |
| How is it reverted? | Single commit revert. No config migration and no wire-visible change to a converged session |
| Who else touches this path? | `plan/spec-l2tp-ipv6-subscriber.md` (skeleton) makes IPv6CP reachable; the RA and DHCPv6-PD work in the same package reads the same identifiers |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| An IPv6CP Configure-Request with no Interface-Identifier option | → | `evalIPv6CPRequest` verdict (`ncp.go`) | `TestIPv6CPRequestWithoutIdentifierIsNotAcked` |
| An IPv6CP Configure-Request carrying a zero identifier | → | `buildNakOrReject` suggestion (`ncp.go`) | `TestIPv6CPNakSuggestsNonZeroIdentifier` |
| An IPv6CP Configure-Request whose identifier equals Ze's and is zero | → | `buildNakOrReject` reject branch (`ncp.go`) | `TestIPv6CPBothZeroIsRejected` |
| IPv6CP reaching Opened with no negotiated peer identifier | → | `afterLCPOpen` → `startIPv6Service` (`session_run.go`) | `TestIPv6ServiceRefusesUnnegotiatedIdentifier` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A Configure-Request carrying no Interface-Identifier option | Ze does not send a Configure-Ack; it sends a Configure-Nak carrying an Interface-Identifier option whose value is non-zero and differs from the identifier Ze last requested |
| AC-2 | A Configure-Request whose identifier is zero and differs from Ze's | Ze sends a Configure-Nak whose suggested identifier is non-zero, differs from Ze's own, and has the "u" bit clear |
| AC-3 | A Configure-Request whose identifier equals Ze's and is non-zero | Ze sends a Configure-Nak with a different non-zero identifier |
| AC-4 | A Configure-Request whose identifier equals Ze's and both are zero | Ze sends a Configure-Reject carrying an Interface-Identifier option with the value zero, and IPv6CP identifier negotiation ends |
| AC-5 | A Configure-Request whose identifier is non-zero and differs from Ze's | Ze sends a Configure-Ack and records the identifier as negotiated |
| AC-6 | A Configure-Request carrying two Interface-Identifier options | Ze does not Ack it; the request is treated as it is for any other unacceptable option list |
| AC-7 | IPv6CP reaches Opened without a negotiated peer identifier | No IPv6 service starts, no route is installed, and no session-up event carries a peer address; the session stays up IPv4-only and the refusal is logged with the reason |
| AC-8 | A session whose peer identifier was negotiated | `peerLinkLocal` yields `fe80::` plus that identifier, the delegated-prefix route is installed through it, and the session-up event carries it |
| AC-9 | The value Ze proposes in any Configure-Nak | Passes `isValidIPv6CPInterfaceID`, which is the same validator Ze applies on receive |
| AC-10 | A peer that repeatedly sends a Configure-Request carrying no Interface-Identifier option | Ze Naks with a suggestion ONCE. On the peer's next request that still carries no option, Ze Acks rather than Naking again, so the negotiation terminates instead of looping. The one-shot state is per session and resets with the session |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Connects a subscriber whose client offers a zero identifier and gets a working IPv6 session | IPv6CP Configure-Request → Nak with a non-zero suggestion → peer's second request → Ack → IPv6 service | `ipv6cp-zero-identifier` interop scenario |
| 2 | Connects a client that omits the option and sees IPv6CP converge rather than open on a zero | Configure-Request → Nak → peer supplies the option → Ack | `TestIPv6CPRequestWithoutIdentifierIsNotAcked` plus the interop scenario |
| 3 | Sees no route toward `fe80::` on any session | NCP Opened → service start refusal → no `installRoute` call | `TestIPv6ServiceRefusesUnnegotiatedIdentifier` and the QEMU route assertion |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestIPv6CPRequestWithoutIdentifierIsNotAcked` | `internal/component/l2tp/ppp/ncp_test.go` | AC-1 | |
| `TestIPv6CPNakSuggestsNonZeroIdentifier` | `internal/component/l2tp/ppp/ncp_test.go` | AC-2 and AC-9 | |
| `TestIPv6CPNakOnEqualNonZeroIdentifiers` | `internal/component/l2tp/ppp/ncp_test.go` | AC-3 | |
| `TestIPv6CPBothZeroIsRejected` | `internal/component/l2tp/ppp/ncp_test.go` | AC-4 | |
| `TestIPv6CPAcksDistinctNonZeroIdentifier` | `internal/component/l2tp/ppp/ncp_test.go` | AC-5 | |
| `TestIPv6CPDuplicateIdentifierOptionIsNotAcked` | `internal/component/l2tp/ppp/ncp_test.go` | AC-6 | |
| `TestIPv6CPSuggestionDiffersFromLocalIdentifier` | `internal/component/l2tp/ppp/ncp_test.go` | A-3 and the Section 4.1 difference rule | |
| `TestIPv6CPSuggestionHasUniversalBitClear` | `internal/component/l2tp/ppp/ipv6cp_test.go` | The "u" bit rule | |
| `TestIPv6CPMissingOptionIsNakedOnce` | `internal/component/l2tp/ppp/ncp_test.go` | AC-10: the second tagless request is Acked, so the negotiation ends rather than looping | |
| `TestIPv6ServiceRefusesUnnegotiatedIdentifier` | `internal/component/l2tp/ppp/ipv6_service_test.go` | AC-7 | |
| `TestPeerLinkLocal` | `internal/component/l2tp/ppp/ipv6_service_test.go` | AC-8, the derivation half | pre-existing, unchanged |
| `TestHandleDHCPv6RequestInstallsRoute` | `internal/component/l2tp/ppp/ipv6_service_test.go` | AC-8, the route half | pre-existing, unchanged |
| `TestIPv6CPOpenedEmitsAssigned` | `internal/component/l2tp/ppp/ncp_test.go` | AC-8, the event half | pre-existing, unchanged |
| `TestIPv6CPNakSuggestionFailureRejectsAndKeepsIPv4Session` | `internal/component/l2tp/ppp/ncp_test.go` | AC-9 under a draw failure; the session survives | added beyond the plan |
| `TestSuggestionDrawFailureCountsIdentifierRefusal` | `internal/component/l2tp/ppp/ncp_test.go` | the third refusal counter site | added beyond the plan |
| `TestIdentifierRefusalCounterBindsWhenTheRegistryArrivesLast` | `internal/component/l2tp/ppp/metrics_test.go` | the counter binds on an L2TP/PPPoE-only daemon | added beyond the plan |
| `TestIdentifierRefusalCounterCountsAfterLCPOpenAndSessionEventRefusals` | `internal/component/l2tp/ppp/metrics_test.go` | the two session-side refusal counter sites | added beyond the plan |

The plan named `TestPeerLinkLocalFromNegotiatedIdentifier` for AC-8. No such test
was written and none was needed: AC-8 is a preservation criterion (a session
whose identifier WAS negotiated keeps behaving as before), and the three
pre-existing tables rows above already cover its three halves. Corrected at
closure rather than left naming a test that does not exist.

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Interface-Identifier option length | 10 octets total (type, length, 8 value) | 10 | 9 | 11 |
| Identifier value | any 64-bit value except all-zero and all-ones | `00:00:00:00:00:00:00:01` | all-zero | all-ones |
| Interface-Identifier options per Configure-Request | exactly 1 | 1 | 0 | 2 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ppp-ipv6cp-negotiation` | `test/l2tp/` | A subscriber session negotiates IPv6CP and no route toward `fe80::` exists in the kernel afterwards | NOT OWNED HERE -- moved to `plan/spec-l2tp-ipv6-subscriber.md` at closure; see Work Not Done |

`test/qemu/` does not exist. A QEMU-backed functional test for a PPP subscriber
session is a `.ci` under `test/l2tp/` carrying `option=needs-linux:caps=net-admin`,
the shape `test/l2tp/radius-acct-wire.ci` sets and the shape
`plan/spec-l2tp-ipv6-subscriber.md` already names in its "Constraints on the test
work" section.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ipv6cp-zero-identifier` | `test/interop-pppoe/scenarios/` | pppd 2.5.1 | A peer offering a zero identifier is Naked with a usable suggestion and converges | written, registered, BLOCKED -- see Known Limitations |
| `ipv6cp-missing-option` | `test/interop-pppoe/scenarios/` | pppd 2.5.1 with the option suppressed | A request with no identifier option is not Acked and the peer recovers | written, registered, BLOCKED -- see Known Limitations |

## Files to Modify
- `internal/component/l2tp/ppp/ncp.go` - the four Section 4.1 comparison cases in `evalIPv6CPRequest` and `buildNakOrReject`, and the negotiated fact set on accept
- `internal/component/l2tp/ppp/ipv6cp.go` - a suggestion generator that yields a non-zero identifier differing from the local one with the "u" bit clear, reusing `isValidIPv6CPInterfaceID` on the transmit path
- `internal/component/l2tp/ppp/session.go` - the negotiated flag beside `peerInterfaceID`
- `internal/component/l2tp/ppp/session_run.go` - `afterLCPOpen` starts the IPv6 service only for a negotiated identifier
- `internal/component/l2tp/ppp/ipv6_service.go` and `ipv6_service_linux.go` - NOT CHANGED, and phase 1 proved why: neither reads a `pppSession` field, only a copy carried on `iPv6ServiceConfig`, so the single guard upstream in `afterLCPOpen` covers them both. Recorded here because the reader who expects an edit needs to know it was considered
- `docs/research/l2tpv2-ze-integration.md` - the design document every file above declares: the Section 4.1 cases and what an unnegotiated identifier means downstream
- `docs/architecture/l2tp/bng-5-pppoe.md` - the AC page, where the same PPP negotiation is described for PPPoE subscribers
- `rfc/short/rfc5072.md` - the Section 4.1 requirement rows this work implements and proves

## Files to Create
- `internal/component/l2tp/ppp/ipv6_service_test.go` - if absent, the service-side tests for AC-7 and AC-8

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | The RFC decides the behavior; there is nothing for an operator to choose |
| YANG validation constraints | N-A | No new leaf |
| YANG custom validators | N-A | No new leaf |
| CLI commands/flags | N-A | No new verb; the existing session show output already carries the addresses |
| CLI grammar (keyword before value) | N-A | No new command |
| Editor autocomplete | N-A | No new leaf |
| Functional test for new RPC/API | N-A | No new RPC; the QEMU scenario covers the behavior |
| Pipe completeness | N-A | No new command output |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No new file path, socket, port, module, binary or sysctl |
| Prometheus counters/metrics | Yes | A counter for IPv6CP negotiations refused for want of a usable identifier, so an operator sees the refusal rather than silence |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | The behavior is conformance, not a feature an operator selects |
| 2 | Config syntax changed? | No | No leaf changes |
| 3 | CLI command added/changed? | No | No verb changes |
| 4 | API/RPC added/changed? | No | No RPC change |
| 5 | Plugin added/changed? | No | The pool plugin is read but not changed here |
| 6 | Has a user guide page? | Yes | `docs/guide/l2tp.md` ("PPP negotiation", the IPv6CP phase and its `ncp.go` source anchor) and `docs/guide/pppoe.md`. Both edited at closure; the implementation phase missed them |
| 7 | Wire format changed? | Yes | The IPv6CP option behavior is described where the PPP control protocols are documented |
| 8 | Plugin SDK/protocol changed? | No | `EventSessionIPAssigned` keeps its shape; only the values it may carry are constrained |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc5072.md` Section 4.1 rows, and the `docs/features/rfc-status.md` row generated from them, with source anchors |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` for the two new interop scenarios |
| 11 | Affects daemon comparison? | No | No comparison row changes |
| 12 | Internal architecture changed? | Yes | `docs/research/l2tpv2-ze-integration.md` |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | Yes | `ze_ppp_ipv6cp_identifier_refusals_total`, in `docs/guide/l2tp.md` "Prometheus metrics" with its three reasons, cross-referenced from `docs/guide/pppoe.md` "Metrics". Both edited at closure; the implementation phase missed them |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Nothing registers |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: run `./le spec citation anchors spec plan/immediate/spec-ipv6cp-accepts-and-proposes-a-zero-interface-identifier.md`. Every `ppp/` file above declares `docs/research/l2tpv2-ze-integration.md`, which this spec names and edits |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Check the IPv6CP and subscriber examples in `docs/guide/l2tp.md` against the negotiation after the change |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the negotiated fact exists and every reader consults it
   - Tests: `TestIPv6ServiceRefusesUnnegotiatedIdentifier`
   - Files: `session.go`, `session_run.go`, `ipv6_service.go`, `ipv6_service_linux.go`
   - Verify: the flag exists, `gopls references` on `peerInterfaceID` shows no reader that skips it, and the wiring test fails because the service still starts
2. **Phase: the receive-side comparison cases** -- Section 4.1's four outcomes in `evalIPv6CPRequest`
   - Tests: `TestIPv6CPRequestWithoutIdentifierIsNotAcked`, `TestIPv6CPAcksDistinctNonZeroIdentifier`, `TestIPv6CPDuplicateIdentifierOptionIsNotAcked`, `TestIPv6CPBothZeroIsRejected`
   - Files: `ncp.go`
   - Verify: each case produces the code the RFC names, with the quoted requirement above the branch that enforces it
3. **Phase: the suggestion** -- a non-zero identifier differing from the local one, "u" bit clear, validated on transmit
   - Tests: `TestIPv6CPNakSuggestsNonZeroIdentifier`, `TestIPv6CPNakOnEqualNonZeroIdentifiers`, `TestIPv6CPSuggestionDiffersFromLocalIdentifier`, `TestIPv6CPSuggestionHasUniversalBitClear`
   - Files: `ipv6cp.go`, `ncp.go`
   - Verify: every value Ze transmits passes `isValidIPv6CPInterfaceID`
4. **Phase: the downstream refusal and the counter** -- no service, no route, no event on an unnegotiated identifier
   - Tests: `TestPeerLinkLocalFromNegotiatedIdentifier`, the refusal counter assertion
   - Files: `ipv6_service.go`, `session_run.go`, the telemetry registration
   - Verify: the refusal is logged and counted rather than silent
5. **Phase: proof against a real peer** -- the two interop scenarios and the QEMU route assertion
   - Tests: `ipv6cp-zero-identifier`, `ipv6cp-missing-option`, `ppp-ipv6cp-negotiation`
   - Files: `test/interop-pppoe/scenarios/`, `test/qemu/`
   - Verify: revert each enforcing branch in turn, record the red, restore it, confirm green. Where a unit carries an `RFC requirement:` tag, the walk runs through `./le rfc discriminate-record`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All four Section 4.1 comparison cases have a branch and a test, and each branch carries the quoted requirement |
| Feature completeness | The negotiated fact reaches every consumer: the service, the route, the event and the counter |
| Correctness | The suggested identifier is non-zero, differs from the local identifier, and has the "u" bit clear |
| Naming | The negotiated flag reads as a fact about negotiation, not as a nullable value, and the counter names the refusal reason |
| Data flow | The decision is made once in `ncp.go`; `ipv6_service.go` consumes a fact rather than re-deriving one |
| Rule: `ai/rules/principles.md` | The zero is no longer a valid-looking answer, and the guard fails closed: an unnegotiated identifier refuses the service rather than starting it with a default |
| Rule: `ai/rules/rfc-compliance.md` | Every MUST implemented is proven by a tagged test with a discrimination record, and no row claims more than the test body checks |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The four comparison cases exist | `grep -n "RFC 5072 Section 4.1" internal/component/l2tp/ppp/ncp.go` returns one quote per branch |
| No reader consumes a raw peer identifier | `gopls references` on `peerInterfaceID`, each caller reads the negotiated fact first |
| Every transmitted identifier is valid | `TestIPv6CPNakSuggestsNonZeroIdentifier` and `TestIPv6CPSuggestionHasUniversalBitClear` pass |
| The interop scenarios discriminate | The recorded red from each reverted branch |
| The RFC ledger reflects what is proven | `./le rfc check` passes over the changed stems |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The option list is already bounds-checked; the new work is semantic validation of the value, applied on both receive and transmit |
| Fail-open guard | The negotiated flag is a guard: absent, it must refuse the service, never default to a usable-looking address |
| Error leakage | The refusal log names the reason and the session, not peer-supplied bytes verbatim |
| Resource exhaustion | The Nak loop is bounded by the existing FSM Configure-Nak counter; the suggestion generator allocates nothing per packet |
| Authorization | None: IPv6CP runs after authentication, so no new trust boundary is crossed |

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

- A bare array with no companion fact is the shape this defect takes. The check that was missing is not a bounds check but a record of whether negotiation happened, which is why no length validation would have caught it.
- Ze already owned the right validator and applied it on one side only. A validator used on receive and not on transmit lets an implementation send what it would refuse.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Record the negotiated fact on the session | Treat all-zero as the sentinel for "not negotiated" | A sentinel is the defect in a new spelling: a legitimate value can never be zero here, but the reader still cannot distinguish a failure from a value, and the next reader repeats the mistake (`ai/rules/principles.md`) |
| Answer a missing option with a Configure-Nak carrying a suggestion | Answer with a Configure-Reject | A Reject says the option is not negotiable, which is false: Ze does negotiate it. A Nak invites the peer to send it and is what converges |
| Draw the suggestion from `generateIPv6CPInterfaceID` and reject a collision with the local identifier | accel-ppp's fixed configured value, which suggests `::2` against its own `::1` | pppd loops a random value until it is neither zero nor its own, and Ze already owns the generator that does it. A constant is reproducible across restarts, which the RFC prefers, but it is also guessable and identical on every session, and Ze's own validator would still have to gate it |
| Clear the "u" bit on the suggested identifier | Leave it as the generator produced it, which is what accel-ppp and pppd both do | RFC 5072 Section 4.1 addresses that MUST to the implementation making the suggestion. Neither upstream clears it; both are non-conformant on this line and Ze is not going to copy that |
| Nak a missing option ONCE, then Ack | Nak every time (the shape before the comparison), or Ack immediately as accel-ppp does | Naking every time never terminates against a peer that will not send the option. pppd's one-shot guard converges, and Acking immediately is the fail-open this spec exists to close |
| Refuse the IPv6 service rather than starting it without a route | Start the service and skip only the route installation | A service running without the route publishes a session-up event carrying an address that is not the peer's, which is the fail-open path this spec exists to close |

## Known Limitations
- This spec does not make IPv6CP reachable. The pool handler's IPv6 decline is `plan/spec-l2tp-ipv6-subscriber.md`, and until that lands the acceptance criteria are proven by unit tests and by two interop scenarios that are written and registered but cannot reach IPv6CP against a live subscriber, confirmed at both producers:
  `poolPlugin.handle` (`internal/component/l2tp/plugins/pool/register.go`) answers every `EventIPRequest` whose `Family != AddressFamilyIPv4` with `Accept: false`, unconditionally; `runNCPPhase` (`internal/component/l2tp/ppp/ncp.go`) reads that decline BEFORE the session reads a single client frame and sets `disableIPv6CP`, so `startNCP(AddressFamilyIPv6)` never runs and `evalIPv6CPRequest` is unreachable in a shipped daemon today, on every configuration, regardless of what a client offers.
  `ipv6cp-zero-identifier` and `ipv6cp-missing-option` (`test/interop-pppoe/scenarios/`, checkers in `internal/le/interoplab/pppoe/check_ipv6cp.go`, registered in `checkers()`, `internal/le/interoplab/pppoe/pppoe.go`) drive a real pppd 2.5.1 client and assert the real target wire behavior (a non-zero, valid Configure-Nak suggestion and convergence; a one-shot Nak-then-Ack on a tagless request), not a weakened stand-in. Each returns a citing error the moment it fails to observe any IPv6CP frame from Ze, rather than passing vacuously. They are ready for `plan/spec-l2tp-ipv6-subscriber.md` to switch on; that spec is what turns them from a documented block into a proof.
  Neither could be executed on the implementing host: the Docker lab refuses with `host kernel missing PPPoE requirements: pppoe (PPPoE pppox kernel module)`, and `./le qemu pppoe-test` exits 1 with `qemu guest evidence requires Linux`. The code compiles and vets clean (`go vet ./internal/le/interoplab/...`) but has not been run once, on any host, by this work.
- The "u" bit exception in RFC 5072 Section 4.1, where a globally unique EUI-derived identifier is provided for the peer's exclusive use, is not implemented: Ze always clears the bit. That is the conformant choice for a suggestion Ze generates, and it is recorded here rather than left silent.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT. Each of the four
comparison branches carries its own quoted sentence from RFC 5072 Section 4.1,
and the "u" bit rule is quoted above the suggestion generator.

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
- [ ] AC-1..AC-10 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

## Implementation Summary

### What Was Implemented
- `pppSession.peerInterfaceIDNegotiated` (`internal/component/l2tp/ppp/session.go`),
  the companion fact `peerInterfaceID` never carried. Set on both legitimate
  origins: `evalIPv6CPRequest` accepting a Configure-Request that carried the
  option, and `requestIPv6CPInterfaceID` taking the address handler's
  `HasPeerInterface` override (both `ncp.go`).
- Every RFC 5072 Section 4.1 comparison outcome in `evalIPv6CPRequest`
  (`ncp.go`): a missing option Naked ONCE then Acked, guarded by
  `pppSession.ipv6cpMissingIdentifierNaked`; a received zero or an equal
  identifier unacceptable; a distinct non-zero identifier Acked AND recorded.
- Duplicate-option detection in `parseIPv6CPOptions` (`ipv6cp.go`), which
  previously kept the last of two conflicting values with `HasInterfaceID`
  still true.
- `suggestIPv6CPInterfaceID` (`ipv6cp.go`): draws from `generateIPv6CPInterfaceID`,
  clears the "u" bit, then re-validates AFTER clearing (clearing can turn a
  valid draw all-zero) and rejects a collision with `localInterfaceID`.
- `buildNakOrReject` (`ncp.go`) now proposes that draw instead of the stale
  `peerInterfaceID`, answers the equal-and-zero case with a Configure-Reject
  echoing `req.Data`, and answers a draw failure with a Configure-Reject
  carrying a zero identifier rather than a Nak carrying a value Ze would refuse
  on receive.
- Two downstream guards on the negotiated fact: `afterLCPOpenIPv6Service`
  (`session_run.go`) starts no IPv6 service, and `onNCPOpened` (`ncp.go`)
  publishes no `EventSessionIPAssigned`, for an unnegotiated identifier.
- `ze_ppp_ipv6cp_identifier_refusals_total`, one bounded `reason` label over
  three sites (`metrics.go`), bound through `registry.InjectPluginMetrics` from
  `Driver.Start` (`manager.go`) so it binds on an L2TP- or PPPoE-only daemon
  whose telemetry registry is created after the engine starts.
- Two interop scenarios written, registered in `checkers()`
  (`internal/le/interoplab/pppoe/pppoe.go`) and blocked; see Known Limitations.
- Seven discrimination records in `rfc/discrimination/rfc5072.json`, each an
  observed red via `route revert`.

### Bugs Found/Fixed
- `parseIPv6CPOptions` silently accepted a duplicate Interface-Identifier
  option, keeping the last of two conflicting values. Covered by
  `TestIPv6CPDuplicateIdentifierOptionIsNotAcked`.
- Clearing the "u" bit can turn a valid draw into the all-zero value. Found
  while writing `suggestIPv6CPInterfaceID`; the validity check runs after the
  clear, and `TestIPv6CPSuggestionHasUniversalBitClear` plus
  `TestIPv6CPNakSuggestionFailureRejectsAndKeepsIPv4Session` cover both halves.
- **Found at closure, fixed at closure:** eight sites cited "RFC 5072 Section
  3.2" as the authority for rejecting an all-zero Interface-Identifier. RFC
  5072 has NO Section 3.2 -- its sections are 1, 1.1, 2, 3, 4, 4.1, 5, 6, 7, 8,
  9, 9.1, 9.2, read from `rfc/full/rfc5072.txt`. Five of the eight were added
  by this spec. A ninth site,
  `isUsableSuggestedIdentifier` (`internal/le/interoplab/pppoe/check_ipv6cp.go`),
  attributed a verbatim quotation to Section 4.1 -- the identifier `"MUST NOT be
  all zeros or all ones"` -- that appears nowhere in the document: `grep -i
  "all zero\|all ones\|zeros" rfc/full/rfc5072.txt` returns nothing. Every
  site now cites Section 4.1's real comparison text, and the all-ones exclusion
  is named as Ze's own rather than the RFC's. Row in
  `plan/journal/claim-outlives-the-evidence-it-cites.md`.
- **Found at closure, fixed at closure:** the spec's TDD table named
  `TestPeerLinkLocalFromNegotiatedIdentifier` for AC-8 and no such test exists.
  The table now names the three pre-existing tests that do cover AC-8.

### Documentation Updates
- `docs/research/l2tpv2-ze-integration.md` -- the design document every changed
  `ppp/` file declares in its `// Design:` header. Gained the negotiated-fact
  paragraph, the Section 4.1 comparison outcomes, and the suggestion generator.
- `docs/labs/pppoe-interop.md` -- the two new scenario directories, their
  `ZE_PPPOE_INTEROP_SCENARIO` invocations, `check_ipv6cp.go` in the file map,
  and a section stating why neither can pass today and what unblocks them.
- `rfc/short/rfc5072.md` and `rfc/requirements/rfc5072.md` -- `RFC5072-4.1-1`,
  `-4.1-4`, `-4.1-5`, `-4.1-6` and `-4.1-9` moved off `{gap}` /
  `{not-applicable}`, and `RFC5072-4.1-19` extracted as a new MUST NOT.
- `./le rfc check`: no rfc5072 finding (the 109 lines it prints are other
  stems, other sessions).

### Deviations from Plan
- The spec's Files to Modify named `ipv6_service.go` and `ipv6_service_linux.go`
  as NOT CHANGED, and they are not. A-4 was under-named rather than wrong: the
  reader set is `ncp.go` and `session_run.go`, closed by `gopls references`.
- `internal/component/l2tp/ppp/metrics.go` and `metrics_test.go` were created,
  which the plan implied ("the telemetry registration") without naming files.
- `afterLCPOpen`'s IPv6 block was extracted into `afterLCPOpenIPv6Service`
  (`session_run.go`) at closure, so the guard reads as a guard clause with an
  early return rather than an `else`-wrapped happy path
  (`docs/contributing/ze-go-style.md`, "Control flow a reader can simulate").
- The `ppp-ipv6cp-negotiation` functional test moved to
  `plan/spec-l2tp-ipv6-subscriber.md`; see Work Not Done.

## Mistake Log

<!-- One table, one place. Ship the `none` row and either replace it or leave it
     deliberately: three separate empty tables produced three separate 67-82%
     untouched rates, because an empty table asks nothing.
     Kind: assumption (a broken A-N) | approach (a route abandoned) | escalation
     (a mistake frequent enough to deserve a rule). -->
| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-4 assumed only `ipv6_service.go` and `ncp.go` read `peerInterfaceID` | Five references across `ncp.go` and `session_run.go`; `ipv6_service.go` reads no `pppSession` field at all, only a copy on `iPv6ServiceConfig` | `gopls references` on the field during phase 1 | A-4 marked broken (harmlessly); the guard went upstream in `afterLCPOpen` rather than into the service |
| approach | Every code comment and the design page cited "RFC 5072 Section 3.2" for the all-zero rule, and one interop checker quoted a sentence as Section 4.1 text | RFC 5072 has no Section 3.2, and the quoted sentence appears nowhere in the document | Closure review read `rfc/full/rfc5072.txt` Section 4.1 end to end to check every quoted branch comment | All nine sites rewritten to Section 4.1's real text; journal row in `plan/journal/claim-outlives-the-evidence-it-cites.md` |
| escalation | A pre-existing `§3.2` citation was copied forward into five new comments rather than checked | The citation had been wrong since before this spec and nothing reads a section number | Same read | The transferable habit: a citation you COPY is a citation you own. `ai/rules/rfc-compliance.md` already requires reading the RFC's own text; the gap is that it is easy to read Section 4.1 for the rule you are implementing and never open the section you are citing beside it |

## Implementation Audit

<!-- BLOCKING before the learned summary. See ai/rules/completion.md.
     Status: Done (with file:line) | Partial | Skipped | Changed.
     Partial and Skipped both require explicit user approval. -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| A zero peer identifier is impossible to confuse with a negotiated one | Done | `session.go:193` `peerInterfaceIDNegotiated`; written at `ncp.go:217` and `ncp.go:621` only | Both write sites are the two legitimate origins; no third writer exists (`grep -n peerInterfaceID internal/component/l2tp/ppp/*.go`) |
| Every RFC 5072 Section 4.1 comparison case is implemented | Done | `evalIPv6CPRequest` and `buildNakOrReject` (`ncp.go`) | Each branch carries its quoted sentence, verified against `rfc/full/rfc5072.txt` |
| No address derived from an unnegotiated identifier reaches the kernel or an event | Done | `afterLCPOpenIPv6Service` (`session_run.go`), `onNCPOpened` IPv6 arm (`ncp.go`) | The route is installed only by `installRoute`, reachable only from a service the first guard refuses to start |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestIPv6CPRequestWithoutIdentifierIsNotAcked` | Verdict is `ncpRequestUnacceptable`; `buildNakOrReject` then draws a suggestion |
| AC-2 | Done | `TestIPv6CPNakSuggestsNonZeroIdentifier` | Asserts non-zero AND `isValidIPv6CPInterfaceID` |
| AC-3 | Done | `TestIPv6CPNakOnEqualNonZeroIdentifiers` | Asserts the suggestion differs from the collision and is non-zero |
| AC-4 | Done | `TestIPv6CPBothZeroIsRejected` | Configure-Reject with a zero identifier; the branch is defensive in production because `localInterfaceID` is never legitimately zero |
| AC-5 | Done | `TestIPv6CPAcksDistinctNonZeroIdentifier` | Checks both halves: the verdict AND `peerInterfaceIDNegotiated` |
| AC-6 | Done | `TestIPv6CPDuplicateIdentifierOptionIsNotAcked` | `parseIPv6CPOptions` returns `errIPv6CPDuplicateInterfaceID` |
| AC-7 | Done | `TestIPv6ServiceRefusesUnnegotiatedIdentifier` | No service, no `EventSessionIPAssigned`, `EventSessionUp` still fires, refusal logged with its reason |
| AC-8 | Done | `TestPeerLinkLocal`, `TestHandleDHCPv6RequestInstallsRoute`, `TestIPv6CPOpenedEmitsAssigned` | Preservation criterion; all three pre-date this work and still pass |
| AC-9 | Done | `TestIPv6CPNakSuggestsNonZeroIdentifier`, `TestIPv6CPNakSuggestionFailureRejectsAndKeepsIPv4Session` | The second covers the draw-failure branch: a Configure-Reject, never a Nak carrying an invalid value |
| AC-10 | Done | `TestIPv6CPMissingOptionIsNakedOnce` | Second tagless request is Acked; per-session, resets with the session |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestIPv6CPRequestWithoutIdentifierIsNotAcked` | Done | `ncp_test.go` | |
| `TestIPv6CPNakSuggestsNonZeroIdentifier` | Done | `ncp_test.go` | |
| `TestIPv6CPNakOnEqualNonZeroIdentifiers` | Done | `ncp_test.go` | |
| `TestIPv6CPBothZeroIsRejected` | Done | `ncp_test.go` | |
| `TestIPv6CPAcksDistinctNonZeroIdentifier` | Done | `ncp_test.go` | |
| `TestIPv6CPDuplicateIdentifierOptionIsNotAcked` | Done | `ncp_test.go` | |
| `TestIPv6CPSuggestionDiffersFromLocalIdentifier` | Done | `ncp_test.go` | 64 draws |
| `TestIPv6CPSuggestionHasUniversalBitClear` | Done | `ipv6cp_test.go` | 64 draws |
| `TestIPv6CPMissingOptionIsNakedOnce` | Done | `ncp_test.go` | |
| `TestIPv6ServiceRefusesUnnegotiatedIdentifier` | Done | `ipv6_service_test.go` | |
| `TestPeerLinkLocalFromNegotiatedIdentifier` | Changed | -- | Never written and not needed; AC-8's three halves are covered by three pre-existing tests. Recorded in Deviations |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/l2tp/ppp/ncp.go` | Done | The four comparison cases, the negotiated fact, the suggestion, the event guard |
| `internal/component/l2tp/ppp/ipv6cp.go` | Done | Duplicate detection, `zeroInterfaceID`, `suggestIPv6CPInterfaceID` |
| `internal/component/l2tp/ppp/session.go` | Done | Two new fields with their doc comments |
| `internal/component/l2tp/ppp/session_run.go` | Done | `afterLCPOpenIPv6Service` |
| `internal/component/l2tp/ppp/ipv6_service.go`, `ipv6_service_linux.go` | Done (unchanged, as planned) | Neither reads a `pppSession` field; the upstream guard covers both |
| `docs/research/l2tpv2-ze-integration.md` | Done | |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Changed | Not edited. The page describes PPPoE discovery and the AC's session table, and says nothing about IPv6CP option negotiation, so it carries no claim this change makes wrong. `docs/labs/pppoe-interop.md` took the PPPoE-side edit instead, because that is where the two new scenarios live |
| `rfc/short/rfc5072.md` | Done | Plus `rfc/requirements/rfc5072.md`, its per-stem generated sibling |
| `internal/component/l2tp/ppp/ipv6_service_test.go` (create) | Done (already existed) | Extended rather than created |

### Audit Summary
- **Total items:** 33 (3 requirements, 10 ACs, 11 planned tests, 9 planned files)
- **Done:** 31
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (`TestPeerLinkLocalFromNegotiatedIdentifier`, `docs/architecture/l2tp/bng-5-pppoe.md`), both recorded in Deviations

## Goal Validation (BLOCKING)

<!-- Maps each goal from the Task section to proof it was achieved. "Tests pass"
     is not evidence for a goal; a named test with its output is.
     See ai/rules/interop-and-goal-validation.md for the required evidence per
     goal type, and for the vacuity traps: a test that would still pass with the
     behavior reverted proves nothing. -->
| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A zero peer interface identifier is impossible to confuse with a negotiated one | unit, wiring | `TestIPv6CPAcksDistinctNonZeroIdentifier` asserts the fact is SET on the accept path; `TestIPv6ServiceRefusesUnnegotiatedIdentifier` asserts the whole downstream refuses when it is not. `./le job run label ipv6cp-close-unit command go test -race -count=1 ./internal/component/l2tp/ppp/...` -> `ok github.com/ze-software/ze/internal/component/l2tp/ppp 16.736s` |
| Every RFC 5072 Section 4.1 comparison case is implemented | RFC conformance, discrimination-recorded | Seven records in `rfc/discrimination/rfc5072.json`, each an OBSERVED red under `route revert` on the producing function, not a claimed one. `./le rfc check` reports no rfc5072 finding. `rfc/short/rfc5072.md` moved four rows off `{gap}`/`{not-applicable}` and extracted `RFC5072-4.1-19` |
| No address derived from an unnegotiated identifier reaches the kernel or an event | unit + counter scrape | `TestIdentifierRefusalCounterCountsAfterLCPOpenAndSessionEventRefusals` drives a real session to IPv6CP Opened with no negotiated identifier and reads BOTH refusal series off the rendered `/metrics` page, so an operator sees the refusal rather than silence |
| **Interop: proof against a real peer** | interop | **NOT ACHIEVED, and this is the honest state.** `ipv6cp-zero-identifier` and `ipv6cp-missing-option` are written, registered in `checkers()` (`internal/le/interoplab/pppoe/pppoe.go`) and CANNOT PASS TODAY. That is correct rather than a defect: IPv6CP is unreachable in a shipped daemon, because `poolPlugin.handle` (`internal/component/l2tp/plugins/pool/register.go`) declines every non-IPv4 family and `runNCPPhase` (`ncp.go`) sets `disableIPv6CP` on that decline before any client frame arrives, so both scenarios fail fast with `errIPv6CPNeverEngaged`. **Latency dependency: `plan/spec-l2tp-ipv6-subscriber.md` is the work that switches the path on, and its "What this spec owes when it runs" now names running these two.** Separately, this host can run neither lab at all: Docker refuses with `host kernel missing PPPoE requirements: pppoe (PPPoE pppox kernel module)`, and `./le qemu pppoe-test` exits 1 with `qemu guest evidence requires Linux`. The scenarios have therefore never executed on any host. `ai/rules/interop-and-goal-validation.md` MAY omit an interop test only for a change with no wire-visible effect; this change IS wire-visible, so the row stays open rather than being marked N/A |

## Work Not Done

<!-- Every in-scope item this spec did not do. Each one is a spec of its own by
     now, in the bucket that item belongs to (plan/README.md), and this table
     names it. There is no shard and no deferral row: a row nobody can count is a
     row nobody schedules, which is why plan/deferrals/ was deleted on 2026-09-05
     holding 103 live rows, 29 of which named no destination at all.
     Scope reduction is the owner's decision, never the author's
     (ai/rules/completion.md). -->
| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| The `ppp-ipv6cp-negotiation` functional test | A subscriber session cannot negotiate IPv6CP at all while `poolPlugin.handle` declines the family, so the test can neither pass nor fail meaningfully here. `test/qemu/` does not exist either; the shape is a `.ci` under `test/l2tp/` with `option=needs-linux:caps=net-admin`, which that spec already names | `plan/spec-l2tp-ipv6-subscriber.md` (row added in this same commit, under "What this spec owes when it runs") |
| RUNNING `ipv6cp-zero-identifier` and `ipv6cp-missing-option` | Both are written, registered and correct; they fail fast with `errIPv6CPNeverEngaged` because Ze never engages IPv6CP. Writing them was in scope and is done; running them needs the pool decline lifted | `plan/spec-l2tp-ipv6-subscriber.md` (row added in this same commit) |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md). The review is INDEPENDENT: reviewer
     subagents or a fresh session over the actual diff, never your own inline
     reasoning about code you just wrote.

     The machine-checked artifact is the deliverable, not this table:
     internal/le/spec/session/review.go record --spec <spec> --rounds <N> ... then check.
     --rounds is the pass count and is required; more than five needs
     --rounds-reason naming the PRODUCT defect a later round found, AND
     --owner-authorised carrying Thomas's word, because more than five passes
     is his decision (owner ruling 2026-08-17). At the cap you stop and ask him;
     you never set that flag on your own initiative. A false statement in this
     record is a NOTE, never a reason for another round (ai/rules/planning.md).
     commit_helper.py runs `review_gate.py check` on the closure commit and
     refuses without a fresh, hash-pinned, CLEAN artifact. Record the artifact
     first; this table exists only to carry what was FOUND and FIXED forward
     into the learned summary. -->

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/ipv6cp-accepts-and-proposes-a-zero-interface-identifier-8a072de8-ce8e-47de-bdd2-015f5b44ca51.md` (32 files, verdict=clean) |
| `review check` | clean |
| Rounds | 3 |
| Reviewer lenses used | Round 1: (a) RFC conformance -- every quoted requirement in the diff read back against `rfc/full/rfc5072.txt` Section 4.1 in full; (b) fail-closed guards and wiring -- every writer and reader of `peerInterfaceID` traced to its producing function. Round 2: the fixes round 1 made and their sibling call sites, plus the eight always-in-scope classes over the whole diff, plus every `<!-- source: -->` anchor pointing at a changed file. Round 3: the two documentation pages round 2 found stale |

**Round 1 scope (written before it ran):** the complete uncommitted diff of this
spec's files -- `internal/component/l2tp/ppp/{ncp,ipv6cp,session,session_run,events,manager,metrics}.go`
and their tests, `internal/le/interoplab/pppoe/{check_ipv6cp,check_service_name,pppoe}.go`,
`test/interop-pppoe/scenarios/ipv6cp-*`, `rfc/short/rfc5072.md`,
`rfc/requirements/rfc5072.md`, `rfc/discrimination/rfc5072.json`,
`docs/research/l2tpv2-ze-integration.md`, `docs/labs/pppoe-interop.md`.

**Round 2 scope (written before it ran):** the nine RFC-citation sites round 1
rewrote, the `afterLCPOpenIPv6Service` extraction and every comment that named
`afterLCPOpen` as the IPv6 refusal site, the spec and lab-page corrections, and
the eight always-in-scope classes over the whole diff.

**Round 3 scope (written before it ran):** the two documentation pages round 2
found stale (`docs/guide/l2tp.md`, `docs/guide/pppoe.md`), every
`<!-- source: -->` anchor in them that names a file this change edits, and the
`docs/architecture/l2tp/bng-5-pppoe.md` claim the spec's Files to Modify named.
Round 3 found nothing further and is the last round.

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | A verbatim quotation attributed to RFC 5072 Section 4.1 -- the identifier `"MUST NOT be all zeros or all ones"` -- appears nowhere in the RFC. `grep -i "all zero\|all ones\|zeros" rfc/full/rfc5072.txt` returns no line | `isUsableSuggestedIdentifier` (`internal/le/interoplab/pppoe/check_ipv6cp.go`), and the error string at the AC-9 assertion in the same file | Both rewritten to quote Section 4.1's real sentence about a non-zero suggestion, and to name the all-ones exclusion as Ze's own |
| 2 | BLOCKER | Five new citations of "RFC 5072 Section 3.2" as the authority for rejecting an all-zero identifier. RFC 5072 has no Section 3.2: its sections are 1, 1.1, 2, 3, 4, 4.1, 5, 6, 7, 8, 9, 9.1, 9.2 | `ipv6cp.go` file header, `ipv6cp.go` `zeroInterfaceID`, `session.go` file header, `session.go` `peerInterfaceIDNegotiated`, `docs/research/l2tpv2-ze-integration.md` | All rewritten to Section 4.1's comparison outcomes |
| 3 | ISSUE | Three PRE-EXISTING sites carried the same `§3.2` citation, and one of them additionally claimed RFC 5072's "security section flags [all-ones] as a red-flag-value", which Section 6 does not say | `ipv6cp.go` option-type const block, `ipv6cp.go` `isValidIPv6CPInterfaceID` doc, `ipv6cp_test.go` `TestIPv6CPInterfaceIDValidity` doc | Fixed on the spot as code related to the problem in hand (`ai/rules/completion.md`); the all-ones exclusion now reads as Ze's own with no RFC behind it |
| 4 | ISSUE | The Interop Tests table named accel-ppp as a client peer for `ipv6cp-zero-identifier`. This lab's client image is pppd/rp-pppoe only and accel-ppp-as-client is not a supported role | this spec's Interop Tests table; `docs/labs/pppoe-interop.md` repeated the claim | Table corrected to pppd 2.5.1; the lab page's sentence rewritten |
| 5 | ISSUE | The Functional Tests row named `test/qemu/`, a directory that does not exist, for a test nobody owned | this spec's Functional Tests table | Row rehomed to `plan/spec-l2tp-ipv6-subscriber.md`, with the `test/l2tp/` shape named; Work Not Done row added |
| 6 | BLOCKER (round 2) | `docs/guide/l2tp.md` describes the IPv6CP phase and carries a `<!-- source: -->` anchor into `ncp.go`, the file this change rewrites. The page still said only that a declined IPv6CP drops the NCP, and said nothing about what Ze now answers for each Section 4.1 outcome, nor that a session can reach Opened with no negotiated identifier and get no IPv6 service. `ai/rules/documentation.md` puts that edit in the same work as the code | `docs/guide/l2tp.md`, "PPP negotiation" | The five outcomes added as a table, plus the unnegotiated-identifier paragraph; the anchor now names `evalIPv6CPRequest`, `buildNakOrReject` and `afterLCPOpenIPv6Service` |
| 7 | BLOCKER (round 2) | The new counter had a documented home and was not in it. `docs/guide/l2tp.md` has a Prometheus metrics section and `docs/guide/pppoe.md` a Metrics section, both enumerating counters by name with a per-reason table; the spec's own checklist item 14 answered Yes and no page was edited | `docs/guide/l2tp.md`, `docs/guide/pppoe.md` | `ze_ppp_ipv6cp_identifier_refusals_total` documented with all three reasons in `l2tp.md`, cross-referenced from `pppoe.md`, both anchored to `internal/component/l2tp/ppp/metrics.go` |

### Findings recorded, not blocking (NOTE)
| # | Finding | Disposition |
|---|---------|-------------|
| N1 | `afterLCPOpen` wrapped the IPv6-service happy path in an `else` block, which `docs/contributing/ze-go-style.md` names as the shape to avoid | Fixed anyway: extracted `afterLCPOpenIPv6Service` (`session_run.go`), a guard clause with an early return. Six comments that named `afterLCPOpen` as the refusal site were refreshed with it |
| N2 | The TDD table named `TestPeerLinkLocalFromNegotiatedIdentifier`, never written | Table corrected to name the three pre-existing tests that cover AC-8. A record defect, so it earned no further round (`ai/rules/planning.md`) |
| N3 | `buildNakOrReject` calls `parseIPv6CPOptions(req.Data)` a second time, after `evalIPv6CPRequest` already parsed the same bytes | Left as is. This is a control-plane path that runs once per Configure-Request, and threading the parsed options through would couple two functions the RFC keeps separate (`ai/rules/simplicity.md`) |
| N4 | After Ze transmits the equal-zero or draw-failure Configure-Reject, a later tagless Configure-Request still draws one Configure-Nak asking for the option Ze just rejected | Left as is, and no MUST is violated. Section 4.1's "MUST be terminated" is discharged by transmitting the Reject, and "A new Configure-Request MUST NOT contain the interface-identifier option if a valid Interface-Identifier Configure-Reject is received" binds the peer, not Ze. The clause that would make the later Nak wrong is a SHOULD conditional on negotiation being "required". Both reject branches are unreachable in production: `localInterfaceID` is never legitimately zero, and the draw failure needs `crypto/rand` to defeat eight retries. A "negotiation terminated" latch would be machinery for a path nothing reaches |

## Pre-Commit Verification

<!-- BLOCKING. Do NOT trust the audit above: re-verify independently and paste
     the evidence. For each row run a command (ls, grep, go test -run) now.

     EVERY sub-table needs at least one data row: pre_commit_verification_gaps
     in internal/le/commit/prepare.go checks them one by one and names the empty
     ones. A row in Files Exist is not evidence for AC Verified.
     Not acceptable: "already checked", "should work", a pointer to the audit. -->

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/l2tp/ppp/ipv6_service_test.go` | Yes | `ls -l` -> present; extended rather than created |
| `internal/component/l2tp/ppp/metrics.go` | Yes | `ls -l` -> present, untracked, in commit A |
| `internal/component/l2tp/ppp/metrics_test.go` | Yes | `ls -l` -> present, untracked, in commit A |
| `internal/le/interoplab/pppoe/check_ipv6cp.go` | Yes | `ls -l` -> present, untracked, in commit A |
| `test/interop-pppoe/scenarios/ipv6cp-zero-identifier/{role,ze.conf}` | Yes | `cat role` -> `ze-ac`; `ze.conf` carries the pppoe/l2tp/api-server blocks |
| `test/interop-pppoe/scenarios/ipv6cp-missing-option/{role,ze.conf}` | Yes | `cat role` -> `ze-ac`; `diff` against its sibling shows only the header comment differs |
| `rfc/discrimination/rfc5072.json` | Yes | 7 records parsed; each carries `route: revert`, a producer and a `producer-sha` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | A tagless Configure-Request is not Acked | `TestIPv6CPRequestWithoutIdentifierIsNotAcked`; package run `ok ... 16.736s` |
| AC-2, AC-9 | The Nak suggestion is non-zero and passes `isValidIPv6CPInterfaceID` | `TestIPv6CPNakSuggestsNonZeroIdentifier`; same run |
| AC-3 | Equal non-zero draws a different non-zero Nak | `TestIPv6CPNakOnEqualNonZeroIdentifiers`; same run |
| AC-4 | Equal-and-zero draws a Configure-Reject with a zero identifier | `TestIPv6CPBothZeroIsRejected`; same run |
| AC-5 | A distinct non-zero identifier is Acked AND recorded | `TestIPv6CPAcksDistinctNonZeroIdentifier` asserts `peerInterfaceIDNegotiated`; same run |
| AC-6 | A duplicated option is not Acked | `TestIPv6CPDuplicateIdentifierOptionIsNotAcked`; same run |
| AC-7 | No service, no route, no event, session stays up, refusal logged | `TestIPv6ServiceRefusesUnnegotiatedIdentifier`; same run |
| AC-8 | A negotiated identifier still yields `fe80::`, the route and the event | `TestPeerLinkLocal`, `TestHandleDHCPv6RequestInstallsRoute`, `TestIPv6CPOpenedEmitsAssigned`; same run |
| AC-10 | Naked once, Acked thereafter | `TestIPv6CPMissingOptionIsNakedOnce`; same run |
| counter | Both session-side refusals reach `/metrics` | `TestIdentifierRefusalCounterCountsAfterLCPOpenAndSessionEventRefusals` scrapes the rendered page; same run |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| A Configure-Request with no Interface-Identifier option | no `.ci`: `TestIPv6CPRequestWithoutIdentifierIsNotAcked` (`ncp_test.go`) drives `evalIPv6CPRequest` | Yes -- read; it calls the producer directly and asserts the verdict, not a stand-in |
| A Configure-Request carrying a zero identifier | `TestIPv6CPNakSuggestsNonZeroIdentifier` (`ncp_test.go`) | Yes -- read; it drives `evalIPv6CPRequest` then `buildNakOrReject` and parses the emitted option back off the buffer |
| Identifier equals Ze's and is zero | `TestIPv6CPBothZeroIsRejected` (`ncp_test.go`) | Yes -- read; asserts code `LCPConfigureReject` and a zero option value |
| IPv6CP Opened with no negotiated peer identifier | `TestIPv6ServiceRefusesUnnegotiatedIdentifier` (`ipv6_service_test.go`) | Yes -- read; it drives a REAL session through `completeIPv6CPMissingOption`, both rounds of the one-shot exchange, and reads the driver's event channel and log |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `poolPlugin.handle` (`internal/component/l2tp/plugins/pool/register.go`) answers `Accept: false` for every `Family != AddressFamilyIPv4`, unconditionally; `runNCPPhase` (`ncp.go`) reads that decline and sets `disableIPv6CP` before the session reads a client frame. Both producers read at closure |
| A-2 | unvalidatable here, and the risk is retired by design rather than by test | The interop scenario that would answer it cannot run (see Goal Validation). The design no longer depends on the assumption: R-1's mitigation is what shipped -- a missing option draws a Configure-Nak with a suggestion, never a Reject, and AC-10's one-shot guard makes a peer that never sends the option converge on an Ack. So the answer either way costs the peer nothing worse than an IPv4-only session |
| A-3 | confirmed | `buildNakOrReject` has `s.localInterfaceID` in scope; `suggestIPv6CPInterfaceID` takes it and rejects a redraw equal to it; `TestIPv6CPSuggestionDiffersFromLocalIdentifier` holds over 64 draws |
| A-4 | broken (harmlessly) | `gopls references` returns five sites, not two. Every one is named in the field's doc comment and gated. Mistake Log row written |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| #12 internal architecture -> `docs/research/l2tpv2-ze-integration.md` | The added paragraphs name `peerInterfaceIDNegotiated` (`session.go`), `evalIPv6CPRequest`, `parseIPv6CPOptions`, `generateIPv6CPInterfaceID`, `suggestIPv6CPInterfaceID` and `afterLCPOpen`; each read at its producer | Yes |
| #10 test infrastructure -> `docs/labs/pppoe-interop.md` | The scenario directories, the `checkers()` registration and the two producers behind the block were each read; `docs/functional-tests.md` lists no PPPoE interop scenario (`grep -n "pppoe-padr-replay" docs/functional-tests.md` -> no hit), so the lab page is the right page | Yes |
| #9 RFC -> `rfc/short/rfc5072.md`, `rfc/requirements/rfc5072.md` | `./le rfc check` reports no rfc5072 finding; the four `Support` cells were edited in the summary's `## Meta` table, never in the generated `docs/features/rfc-status.md` | Yes |
| #6 user guide -> `docs/guide/l2tp.md`, `docs/guide/pppoe.md` | Yes, and the implementation had MISSED it. the "PPP negotiation" section of `docs/guide/l2tp.md` describes the IPv6CP phase and carries `<!-- source: internal/component/l2tp/ppp/ncp.go -- requestIPv6CPInterfaceID declined path -->`, an anchor into a file this change edits. The page now carries the five Section 4.1 outcomes as a table, and states that a session which reaches Opened with no negotiated identifier gets no IPv6 service, no route and no address; the anchor names `evalIPv6CPRequest`, `buildNakOrReject` and `afterLCPOpenIPv6Service` | Yes (fixed in round 3) |
| #14 Prometheus counters | Yes, and the implementation had MISSED it. `docs/guide/l2tp.md` has a "Prometheus metrics" section and `docs/guide/pppoe.md` a "Metrics" section, both enumerating counters by name with a reason table. `ze_ppp_ipv6cp_identifier_refusals_total` is now in both, with its three reasons in `l2tp.md` and a cross-reference from `pppoe.md`, each anchored to `internal/component/l2tp/ppp/metrics.go` | Yes (fixed in round 3) |
| `docs/architecture/l2tp/bng-5-pppoe.md` (spec's Files to Modify) | No edit owed. The page describes PPPoE discovery, the AC session table, and why `ze_pppoe_discovery_refusals_total` binds through `registry.InjectPluginMetrics` rather than `GetMetricsRegistry`. This change makes none of those claims wrong; it takes the same route for a second counter | Yes (No is correct) |
| #7 wire format | The IPv6CP option's byte layout is documented in `ipv6cp.go` above the codec and in the design page's new paragraphs. The option's wire SHAPE did not change; only which values Ze transmits | Yes |
| `./le doc check verify` | Run at closure; result recorded in the closure report | Yes |

## Core Insight
<!-- Optional: the single most important design revelation from this work.
     Not every spec has one. Delete the section if nothing qualifies.
     Feeds the Decisions section of the learned summary. -->
