# Spec: lcp-restart-counters

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-05 |

<!-- Bucket: plan/immediate/. An operator meets both halves of this on the first
     release. The Restart timer, Max-Terminate, Max-Configure and Max-Failure are
     compiled-in constants no config leaf reaches, so an operator on a high-latency
     or a low-latency access link cannot tune LCP retransmission at all; that is
     config fidelity. The tld half is a live session defect: LCP leaves Opened and
     no upper layer is told, so a renegotiating subscriber keeps IP state its link
     no longer holds. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

RFC 1661 Section 4.6 states four MUST-level obligations on the LCP restart
machinery, and Section 4.4 states a fifth on the Zero-Restart-Count action. Ze
implements none of the five. They are enrolled and declared as gaps
(`RFC1661-4.6-1` through `RFC1661-4.6-4` and `RFC1661-4.4-2` in
`rfc/short/rfc1661.md`), and no spec owns them: `plan/spec-finish-l2tp.md` names
"LCP restart-counter (L163)" in one bullet of a consolidation skeleton that names
no requirement id and may be dropped. This spec is that home.

The five obligations, quoted from `rfc/full/rfc1661.txt`:

| Requirement | Section | Quoted obligation |
|-------------|---------|-------------------|
| `RFC1661-4.6-1` | 4.6 | "The Restart timer MUST be configurable, but SHOULD default to three (3) seconds." |
| `RFC1661-4.6-2` | 4.6 | "Max-Terminate MUST be configurable, but SHOULD default to two (2) transmissions." |
| `RFC1661-4.6-3` | 4.6 | "Max-Configure MUST be configurable, but SHOULD default to ten (10) transmissions." |
| `RFC1661-4.6-4` | 4.6 | "Max-Failure MUST be configurable, but SHOULD default to five (5) transmissions." |
| `RFC1661-4.4-2` | 4.4 | "In addition to zeroing the Restart counter, the implementation MUST set the timeout period to an appropriate value." |

Section 4.2 states the rule that binds the timer to the automaton: "Only the
Send-Configure-Request, Send-Terminate-Request and Zero-Restart-Count actions
start or re-start the Restart timer. The Restart timer is stopped when
transitioning from any state where the timer is running to a state where the
timer is not running."

Goals:

1. Make the Restart timer, Max-Terminate, Max-Configure and Max-Failure operator-configurable, with the RFC's SHOULD values as the schema defaults, on both PPP transports (L2TP and PPPoE).
2. Build the Restart counter the automaton depends on, so `irc`, `zrc`, `TO+` and `TO-` carry their RFC meaning instead of being unreachable table entries.
3. Make the counter's terminal actions reachable: `tld` tells the upper layers that LCP left Opened.

### The automaton question, and why it is in scope

The commissioning audit found that `LCPActTLD` is a no-op in `performAction`, and
that the transition out of Opened assigns `s.state` and notifies nobody. It judged
that an automaton defect rather than a quotable MUST, because the Section 4.4
description of `tld` ("This action indicates to the upper layers that the
automaton is leaving the Opened state") carries no RFC 2119 keyword. That reading
of the text is correct, and the item is still in scope, for a reason that is
structural rather than rhetorical:

- The one edge that carries `zrc` is `Opened + RTR -> tld,zrc,sta / Stopping`. `RFC1661-4.4-2` is enforced on that edge and nowhere else, so implementing the fifth MUST means editing the same transition whose first action is `tld`. Landing `zrc` beside a `tld` that stays silent leaves the transition half-implemented in the same `performAction` loop.
- `TO-` fires `tlf` from Closing and from the three negotiating states, and every edge leaving Opened for those states carries `tld` first. A counter whose expiry drives a state change that no upper layer hears is a counter that changes nothing an operator or a subscriber can observe, which is the shape `ai/rules/principles.md` names: a value that is silently wrong is reachable.

The item is therefore carried as AC-11 and AC-12, scoped to `tld` only. `tls` stays
a no-op: nothing in this spec reaches the Starting state.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/l2tp.md` - the operator-facing page for the `l2tp` config block, the PPP phase description at the "LCP" heading, and the hot-apply table at the end
  → Decision: a new config container under `l2tp` owes a documented block AND a row in the hot-apply table; the page already documents `authentication` and `ncp` that way
  → Constraint: the page carries `<!-- source: ... -->` anchors naming `internal/component/l2tp/yang/ze-l2tp-conf.yang`, so any leaf added there makes the page's containers list incomplete until it is edited in the same work
- [ ] `docs/guide/pppoe.md` - the operator-facing page for the `pppoe` config block
  → Constraint: the same PPP implementation serves PPPoE, so the leaves are owed on both pages or the requirement is unmet on the PPPoE path
- [ ] `docs/architecture/config/yang-config-design.md` - how a YANG grouping, its defaults, and the delivered JSON shape reach Go
  → Constraint: every delivered config value arrives as a JSON string, so each coercion needs a `case string:` arm
- [ ] `ai/patterns/config-option.md` - the structural template for a config leaf
  → Constraint: every leaf takes maximum native validation (`range`, `type`), and carries both a `description` and a differing `ze:help`
- [ ] `docs/contributing/rfc-conformance-gates.md` - the tag, the claim and the discrimination record
  → Decision: `rfc1661` is enrolled and carries no `rfc/extraction/rfc1661.json`, so it is a grandfathered stem; the extraction sign-off is not a precondition of this spec (see "What This Spec Owes")

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc1661.md` - the checklist rows for `RFC1661-4.6-1` .. `4.6-4` and `RFC1661-4.4-2`, all five `{gap}`
  → Constraint: the summary is the authored declaration; `rfc/requirements/rfc1661.md` and the `docs/features/rfc-status.md` row are GENERATED by `./le rfc index-update` and must never be hand-edited
- [ ] `rfc/full/rfc1661.txt` Sections 4.1, 4.2, 4.4, 4.6 - the transition table, the timer rule, the action semantics, the counters
  → Constraint: `irc` "sets the Restart counter to the appropriate value (Max-Terminate or Max-Configure)", so which value is appropriate is decided per transition, not per state

**Key insights:** (minimal context to resume after compaction)
- All five obligations are already declared with ids and `{gap}` reasons. The work is implementation and proof, not declaration.
- The RFC binds the timer to three actions. A free-running ticker gated by a state check is the shape being replaced, not extended.
- `defaultNegoTimeout` (30s) is a second, non-RFC bound on the same negotiation. Max-Configure times the Restart timer at the RFC defaults is 10 x 3s = 30s, so the replacement preserves today's observable bound.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/ppp/session_run.go` - `run` creates `time.NewTicker(3 * time.Second)` as `restartTicker` with no config input, guarded only by `!isProxy`. Its `case <-restartTickerC:` arm reads `s.state`, and calls `s.sendConfigureRequest()` directly when the state is ReqSent or AckSent. It never enters the FSM, never counts, and never fires in Closing or Stopping, so a Terminate-Request is transmitted exactly once. `performAction` returns true for `LCPActIRC`, `LCPActZRC`, `LCPActTLU`, `LCPActTLD`, `LCPActTLS` and `LCPActTLF` with the comment "IRC/ZRC: restart-counter management deferred to a 6a hardening pass (see plan/deferrals/, sharded per source)", naming a directory `plan/README.md` records as deleted on 2026-09-05. `handleLCPPacket` notifies upper layers only on entering Opened (`EventLCPUp` plus `afterLCPOpen`) and on entering Closed or Stopped (`EventSessionDown`); every other transition assigns `s.state` and returns. `defaultNegoTimeout` is the 30-second `negoTimer` that bounds LCP negotiation today.
- [ ] `internal/component/l2tp/ppp/ppp_fsm.go` - `LCPDoTransition` implements the full ten-state table. `LCPEventTOPlus` and `LCPEventTOMinus` appear on the ReqSent, AckRcvd, AckSent, Closing and Stopping edges. No caller outside the table and `lcp_fsm_test.go` raises either event. `Opened + RTR` returns `tld,zrc,sta -> Stopping`; `Opened + RCR+` returns `tld,scr,sca -> AckSent`.
- [ ] `internal/component/l2tp/ppp/start_session.go` - `StartSession` carries `EchoInterval`, `EchoFailures`, `AuthTimeout`, `ReauthInterval`, `IPTimeout` and no restart-timer or counter field.
- [ ] `internal/component/l2tp/ppp/session_run.go`, `sendConfigureNakOrReject` - calls `LCPNakOrReject(w, s.negPolicy())` on every request and picks Nak or Reject from that verdict alone. No count of Naks sent exists, so no threshold converts a Nak into a Reject.
- [ ] `internal/component/l2tp/config.go` - `Parameters` carries `AuthTimeout`, `ReauthInterval`, `NCPTimeout`; `Parse` reads the `authentication` and `ncp` containers; `Defaults` seeds Go constants `DefaultAuthTimeoutSecs` and `DefaultNCPTimeoutSecs` that duplicate the YANG `default` values.
- [ ] `internal/component/l2tp/reactor_kernel.go` - builds `ppp.StartSession` from `r.params`, the single L2TP-side construction site.
- [ ] `internal/component/l2tp/pppoe/server.go` - builds `ppp.StartSession` for a PPPoE subscriber and sets no timing field at all, so PPPoE sessions run entirely on the ppp package's compiled-in defaults.
- [ ] `internal/component/l2tp/yang/ze-l2tp-conf.yang` - `authentication` and `ncp` containers sit under `l2tp`; the module imports `ze-types` as `zt` and already `uses zt:listener`, so a cross-module grouping is an established shape here.
- [ ] `internal/component/l2tp/pppoe/yang/ze-pppoe-conf.yang` - the `pppoe` container carries no PPP timing leaves.
- [ ] `internal/component/l2tp/yang/embed.go`, `internal/component/l2tp/yang/register.go` - the embed-and-register pattern a new YANG module follows.
- [ ] `internal/component/sysrib/distance_bootstrap_test.go` - the check that a bootstrap Go constant equals the YANG default it stands in for, with its population derived from the code rather than a hand-written list.

**Behavior to preserve:**
- With no leaf set, the effective Restart timer stays 3 seconds and LCP negotiation is still abandoned after about 30 seconds, so no existing deployment sees a timing change.
- The `lcp_fsm_test.go` transition table assertions: the table is correct today and is not edited by this spec.
- The proxy-LCP path (`isProxy`) starts no restart timer, because the LAC already completed negotiation.
- `EventLCPUp` and `afterLCPOpen` on entering Opened, and `EventSessionDown` on entering Closed or Stopped.

**Behavior to change:**
- The Restart timer becomes configurable and is driven by the `scr`, `str` and `zrc` actions rather than free-running.
- The restart ticker stops calling `sendConfigureRequest` directly; it raises `TO+` or `TO-` into `LCPDoTransition`.
- `irc` and `zrc` set a real counter; `TO-` becomes reachable, so Max-Configure and Max-Terminate bound retransmission.
- `sendConfigureNakOrReject` counts Naks sent and converts to Reject at Max-Failure.
- `tld` emits an LCP-down notification to the upper layers.
- `defaultNegoTimeout` and its `negoTimer` are DELETED. Max-Configure times the Restart timer is the RFC's bound on the same thing, and keeping both is the layering `ai/rules/no-layering.md` forbids.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Operator config text: an `lcp` container under `l2tp` carrying `restart-timer-milliseconds` and `max-configure`, and the matching container under `pppoe`.
- Wire: an LCP packet arriving on the chan fd, and the expiry of the Restart timer.

### Transformation Path
1. Config text parses through the YANG tree; the `lcp` container resolves from the shared grouping with its schema defaults applied.
2. `l2tp.Parse` reads the container into new `Parameters` fields; the PPPoE config reader does the same for its own container.
3. `reactor_kernel.go` and `pppoe/server.go` copy those fields onto `ppp.StartSession`.
4. `ppp.Manager` copies them onto the `pppSession` struct.
5. `run` arms the Restart timer from the session's configured period; `irc` and `zrc` set the session's Restart counter; timer expiry decrements it and raises `TO+` while it is above zero, `TO-` when it reaches zero.
6. `LCPDoTransition` answers with the state and actions; `performAction` executes them; `tld` emits the LCP-down notification.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree ↔ L2TP subsystem | `configvalue` string coercion in `l2tp.Parse` | No |
| Config tree ↔ PPPoE subsystem | the same coercion in the PPPoE config reader | No |
| Transport ↔ PPP | new fields on `ppp.StartSession` | No |
| PPP FSM ↔ upper layers | the LCP-down notification `tld` emits, on the existing session event channel | No |
| Ze ↔ pppd | LCP Configure-Request retransmissions on the wire | No |

### Integration Points
- `ppp.StartSession` - the existing transport-to-PPP contract; four new fields, no new channel.
- `LCPDoTransition` - unchanged; this spec supplies the two events it already answers.
- The `pppSession` event channel that already carries `EventLCPUp` - carries the LCP-down notification too.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A YANG grouping declared in a new `ze-ppp-lcp` module and used by both `ze-l2tp-conf` and `ze-pppoe-conf` resolves its `default` values through both consumers | `ze-l2tp-conf.yang` already `uses zt:listener` from `ze-types` | The defaults get declared twice, which is the duplication this spec exists to avoid; fall back to one module owning the leaves and the other referencing it | `TestPPPLCPGroupingDefaultsResolveInBothModules` | unvalidated |
| A-2 | Deleting `defaultNegoTimeout` leaves no negotiation state unbounded | `session_run.go`: the timer is armed only for the pre-Opened window, and all four negotiating states carry TO edges in `ppp_fsm.go` | An unanswered peer holds a session open forever; restore a bound derived from Max-Configure rather than a second constant | `TestRFC1661MaxConfigureBoundsNegotiationWithoutNegoTimer` | unvalidated |
| A-3 | The L2TP interop LAC container can drop ze's LCP Configure-Request and can count packets | `interoplab.Lab.Exec` runs a command in a named peer container (`internal/le/interoplab/lab.go`) | The interop assertion cannot be made from the LAC; assert from ze's own capture surface (`internal/component/l2tp/raw_capture.go`) instead | the scenario's own first run | unvalidated |
| A-4 | The "appropriate value" for `irc` is derivable from the actions beside it in the same transition: `irc,str` means Max-Terminate and `irc,scr` means Max-Configure | `rfc/full/rfc1661.txt` Section 4.4 and the Section 4.1 table | A per-state table is needed, which is a second declaration of the transition table | `TestRFC1661InitializeRestartCountPicksMaxTerminateOnTerminateEdge` | unvalidated |
| A-5 | PPPoE subscribers reach the same `pppSession` code and so are covered by the same counter implementation | `pppoe/server.go` sends `ppp.StartSession` on the same `SessionsIn` channel | The PPPoE path needs its own implementation, doubling the work | `TestPPPoEStartSessionCarriesLCPRestartConfig` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A Restart timer driven by actions rather than a ticker re-arms on every `scr`, so a peer that floods Configure-Requests keeps the timer alive | The interop scenario's teardown never happens | The counter, not the timer, bounds the exchange: each `scr` decrements it, so a flood exhausts Max-Configure faster rather than slower |
| R-2 | Making `tld` emit a notification changes upper-layer behavior for a renegotiating subscriber that today keeps its IP state | An existing NCP or IPCP test goes red | That red is the defect surfacing. Read the producer, fix the product, never the assertion (`ai/rules/pre-release.md`) |
| R-3 | A sub-second Restart timer on a busy LNS multiplies wakeups per session | `ze-perf` or a lab run shows CPU growth with session count | The schema `range` floor is 100 ms and the `ze:help` states the cost; the timer is per-session and already exists, so the change is its period rather than its count |
| R-4 | Max-Failure converting Nak to Reject can end a negotiation the peer would otherwise have converged | pppd logs a rejected option it needs | This is the RFC's prescribed behavior, the default of 5 is the RFC's, and the leaf lets an operator raise it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every L2TP and PPPoE subscriber session. A counter that expires early tears down healthy sessions; one that never expires leaves half-negotiated sessions open. A `tld` notification sent on the wrong edge tears down IP state under a live subscriber. |
| How is it reverted? | Single commit revert. The config leaves are additive and absent leaves resolve to the RFC defaults, so no config migration is owed. |
| Who else touches this path? | `internal/component/l2tp/**` outside `ppp/` was edited on 2026-09-05 for RFC 2661 mandatory-AVP handling (commit `e396d7424`); read `config.go` and `reactor_kernel.go` fresh. `plan/spec-finish-l2tp.md` names the same restart-counter work in one bullet and must be edited to point here. `plan/spec-l2tp-ipv6-subscriber.md` shares the NCP path `tld` now notifies. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| An `lcp` container under `l2tp` in config text | → | `l2tp.Parse` fills the new `Parameters` fields | `TestL2TPParseLCPContainer` |
| An `lcp` container under `pppoe` in config text | → | the PPPoE config reader fills its parameters | `TestPPPoEParseLCPContainer` |
| `l2tp.Parameters` | → | `reactor_kernel.go` sets the fields on `ppp.StartSession` | `TestL2TPStartSessionCarriesLCPRestartConfig` |
| PPPoE parameters | → | `pppoe/server.go` sets the fields on `ppp.StartSession` | `TestPPPoEStartSessionCarriesLCPRestartConfig` |
| `ppp.StartSession` | → | `manager.go` sets them on the `pppSession` | `TestPPPManagerAppliesLCPRestartConfig` |
| Restart timer expiry in `run` | → | the session raises `TO+` or `TO-` into `LCPDoTransition` | `TestRFC1661RestartTimeoutRaisesFSMEvent` |
| A configuration an operator writes, end to end | → | the parsed value is the interval ze retransmits at | `test/parse/l2tp-lcp-restart.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `restart-timer-milliseconds 500` under `l2tp lcp`, and a peer that never answers the Configure-Request | Ze retransmits the Configure-Request about every 500 ms, not every 3000 ms |
| AC-2 | No `lcp` container in the config | The Restart timer period is 3000 ms, Max-Terminate is 2, Max-Configure is 10, and Max-Failure is 5 |
| AC-3 | `max-configure 3`, and a peer that never answers | Exactly 3 Configure-Requests are transmitted, then LCP leaves the negotiating states for Stopped and the session goes down |
| AC-4 | `max-configure 3`, and a peer that answers with a Configure-Ack on the second request | Retransmission stops, fewer than 3 requests were sent, and the session reaches Opened |
| AC-5 | `max-terminate 2`, a Close, and a peer that never sends Terminate-Ack | Exactly 2 Terminate-Requests are transmitted, then LCP reaches Closed |
| AC-6 | `max-terminate 2`, and a peer that answers the first Terminate-Request with a Terminate-Ack | Exactly 1 Terminate-Request was transmitted and LCP reaches Closed |
| AC-7 | `max-failure 5`, and a peer that repeats a Configure-Request carrying the same unacceptable option | The first 5 answers are Configure-Nak; the 6th is a Configure-Reject naming that option, and locally desired options are not appended to it |
| AC-8 | `max-failure 5`, and four repeats of the same unacceptable option | The 4th answer is still a Configure-Nak |
| AC-9 | A Terminate-Request received while LCP is Opened | The Restart counter is zero and the Restart timer is armed at the configured period, so the next expiry is `TO-` and LCP moves from Stopping to Closed rather than retransmitting |
| AC-10 | A Configure-Request received in a negotiating state whose transition carries `irc,scr` | The Restart counter is set to Max-Configure, so the next expiry is `TO+` and a Configure-Request is retransmitted |
| AC-11 | A Configure-Request received while LCP is Opened | An LCP-down notification reaches the upper layers before the `scr` and `sca` actions are performed |
| AC-12 | An Echo-Request received while LCP is Opened | No LCP-down notification is emitted, and LCP stays Opened |
| AC-13 | `restart-timer-milliseconds 50`, or `max-configure 0` | Config validation refuses the value and names the accepted range |
| AC-14 | A default-configured session whose peer never answers | The session is abandoned after about 30 seconds, unchanged from today, with no `negoTimer` anywhere in the code |
| AC-15 | Each Go bootstrap constant standing in for one of the four leaves | Its value equals the YANG `default` of the leaf it stands in for, checked by a test that derives the pairing from the code |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Sets a 500 ms restart timer and `max-configure 3` on a low-latency access network, so a dead peer fails fast | config text -> YANG -> `l2tp.Parse` -> `StartSession` -> `pppSession` -> Restart timer -> `TO-` -> Stopped | the `lcp-restart-timer-pppd` interop scenario |
| 2 | Sets a 6000 ms restart timer under `pppoe` on a lossy access link, so a subscriber survives a lost Configure-Ack | config text -> YANG -> PPPoE config reader -> `StartSession` -> `pppSession` | `TestPPPoEStartSessionCarriesLCPRestartConfig` plus `test/parse/pppoe-lcp-restart.ci` |
| 3 | Sets `max-failure 2`, so a peer proposing an unacceptable option is Rejected rather than Naked indefinitely | LCP Configure-Request -> `handleLCPPacket` -> `sendConfigureNakOrReject` -> Configure-Reject | `TestRFC1661MaxFailureConvertsNakToReject` |
| 4 | Runs a subscriber whose peer renegotiates LCP mid-session, and expects IP state to be rebuilt rather than left stale | LCP Configure-Request in Opened -> `tld` -> LCP-down notification -> NCP teardown | `TestRFC1661ThisLayerDownOnLeavingOpened` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC1661RestartTimerUsesConfiguredValue` | `internal/component/l2tp/ppp/rfc1661_restart_test.go` | AC-1, `RFC1661-4.6-1` positive | |
| `TestRFC1661RestartTimerDefaultsToThreeSeconds` | `internal/component/l2tp/ppp/rfc1661_restart_test.go` | AC-2, `RFC1661-4.6-1` negative | |
| `TestRFC1661MaxConfigureLimitsConfigureRequests` | `internal/component/l2tp/ppp/rfc1661_restart_test.go` | AC-3, `RFC1661-4.6-3` positive | |
| `TestRFC1661MaxConfigureNotConsumedWhenAckArrives` | `internal/component/l2tp/ppp/rfc1661_restart_test.go` | AC-4, `RFC1661-4.6-3` negative | |
| `TestRFC1661MaxTerminateLimitsTerminateRequests` | `internal/component/l2tp/ppp/rfc1661_restart_test.go` | AC-5, `RFC1661-4.6-2` positive | |
| `TestRFC1661MaxTerminateStopsOnTerminateAck` | `internal/component/l2tp/ppp/rfc1661_restart_test.go` | AC-6, `RFC1661-4.6-2` negative | |
| `TestRFC1661MaxFailureConvertsNakToReject` | `internal/component/l2tp/ppp/rfc1661_restart_test.go` | AC-7, `RFC1661-4.6-4` positive | |
| `TestRFC1661MaxFailureBelowThresholdStillNaks` | `internal/component/l2tp/ppp/rfc1661_restart_test.go` | AC-8, `RFC1661-4.6-4` negative | |
| `TestRFC1661ZeroRestartCountSetsTimeoutPeriod` | `internal/component/l2tp/ppp/rfc1661_restart_test.go` | AC-9, `RFC1661-4.4-2` positive | |
| `TestRFC1661InitializeRestartCountSetsMaxConfigure` | `internal/component/l2tp/ppp/rfc1661_restart_test.go` | AC-10, `RFC1661-4.4-2` negative: the contrast that proves `zrc` zeroed the counter rather than the counter being absent | |
| `TestRFC1661InitializeRestartCountPicksMaxTerminateOnTerminateEdge` | `internal/component/l2tp/ppp/rfc1661_restart_test.go` | A-4 | |
| `TestRFC1661ThisLayerDownOnLeavingOpened` | `internal/component/l2tp/ppp/rfc1661_restart_test.go` | AC-11 | |
| `TestRFC1661NoThisLayerDownWhenStayingOpened` | `internal/component/l2tp/ppp/rfc1661_restart_test.go` | AC-12 | |
| `TestRFC1661RestartTimeoutRaisesFSMEvent` | `internal/component/l2tp/ppp/rfc1661_restart_test.go` | wiring: the timer arm calls the FSM, not `sendConfigureRequest` | |
| `TestRFC1661MaxConfigureBoundsNegotiationWithoutNegoTimer` | `internal/component/l2tp/ppp/rfc1661_restart_test.go` | AC-14, A-2 | |
| `TestPPPManagerAppliesLCPRestartConfig` | `internal/component/l2tp/ppp/manager_test.go` | wiring: `StartSession` to `pppSession` | |
| `TestL2TPParseLCPContainer` | `internal/component/l2tp/config_test.go` | wiring: config text to `Parameters` | |
| `TestL2TPStartSessionCarriesLCPRestartConfig` | `internal/component/l2tp/reactor_kernel_linux_test.go` | wiring: `Parameters` to `StartSession` | |
| `TestPPPoEParseLCPContainer` | the PPPoE config test file under `internal/component/l2tp/pppoe/` | wiring: PPPoE config text to parameters | |
| `TestPPPoEStartSessionCarriesLCPRestartConfig` | `internal/component/l2tp/pppoe/server_test.go` | AC-2 on the PPPoE path, A-5 | |
| `TestPPPLCPGroupingDefaultsResolveInBothModules` | `internal/component/l2tp/ppp/yang/schema_test.go` | A-1 | |
| `TestLCPBootstrapConstantsMatchSchemaDefaults` | `internal/component/l2tp/ppp/lcp_bootstrap_test.go` | AC-15, in the shape of `internal/component/sysrib/distance_bootstrap_test.go` | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `restart-timer-milliseconds` | 100-60000 | 60000 | 99 | 60001 |
| `max-terminate` | 1-255 | 255 | 0 | 256 |
| `max-configure` | 1-255 | 255 | 0 | 256 |
| `max-failure` | 1-255 | 255 | 0 | 256 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `l2tp-lcp-restart` | `test/parse/l2tp-lcp-restart.ci` | An operator writes the `lcp` container under `l2tp` and `ze config validate` accepts it | |
| `l2tp-lcp-restart-range` | `test/parse/l2tp-lcp-restart-range.ci` | An out-of-range value is refused and the message names the accepted range (AC-13) | |
| `pppoe-lcp-restart` | `test/parse/pppoe-lcp-restart.ci` | The same container under `pppoe` is accepted | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `lcp-restart-timer-pppd` | `test/interop-l2tp/scenarios/lcp-restart-timer-pppd/` | xl2tpd and pppd | With a 500 ms restart timer and `max-configure 3`, ze puts exactly 3 Configure-Requests on the wire about 500 ms apart against a real peer whose LCP has been silenced, then tears the session down. It discriminates the fixed 3-second ticker and the 30-second `negoTimer`, which produce a different count and a different elapsed time. | |

The existing `01-ppp-ipv4` scenario is the one that exercises LCP negotiation
today: ze runs as LNS and the pppd inside the xl2tpd LAC container is the LCP
peer. It cannot prove any of these acceptance criteria, because it drives only
the path where every Configure-Request is answered on its first transmission and
no retransmission or counter expiry occurs. The new scenario silences the peer's
LCP so the retransmission path is the path under test.

**Finding, recorded and not fixed here:** all four existing L2TP interop scenario
directories carry numeric prefixes (`01-ppp-ipv4`, `02-ppp-bgp-redistribute-frr`,
`03-ze-lac-xl2tpd-lns`, `04-radius-acct-attrs`), and their names are constants in
`internal/le/interoplab/l2tp/l2tp.go`. `ai/rules/interop-and-goal-validation.md`
bans the numeric prefix outright. Renaming them is not this spec's work; this spec
adds a NAMED scenario and does not extend the numbered set.

## Files to Modify
- `internal/component/l2tp/ppp/session_run.go` - replace the free-running restart ticker with an action-driven Restart timer; raise `TO+` and `TO-` into `LCPDoTransition`; implement `irc`, `zrc` and `tld` in `performAction`; count Naks in `sendConfigureNakOrReject`; delete `defaultNegoTimeout` and `negoTimer`
- `internal/component/l2tp/ppp/session.go` - the Restart counter, the configured period, the three limits, and the Nak count on `pppSession`
- `internal/component/l2tp/ppp/start_session.go` - four new fields on the transport-to-PPP contract
- `internal/component/l2tp/ppp/manager.go` - copy them onto the session
- `internal/component/l2tp/ppp/events.go` - the LCP-down notification `tld` emits
- `internal/component/l2tp/ppp/ncp.go` - consume that notification and tear down the NCP state
- `internal/component/l2tp/config.go` - `Parameters` fields, `Parse` of the `lcp` container, and `Defaults`
- `internal/component/l2tp/reactor_kernel.go` - set the fields on `StartSession`
- `internal/component/l2tp/yang/ze-l2tp-conf.yang` - use the shared grouping under `l2tp`
- `internal/component/l2tp/pppoe/yang/ze-pppoe-conf.yang` - use the same grouping under `pppoe`
- `internal/component/l2tp/pppoe/server.go` - set the fields on `StartSession`
- `internal/le/interoplab/l2tp/l2tp.go` - register the new scenario name and its checker
- `test/interop-l2tp/Dockerfile.lac` - the packet-filter and packet-count tools the new scenario runs (A-3)
- `rfc/short/rfc1661.md` - five checklist rows from `{gap}` to test bindings, and the `Support remaining` row
- `docs/guide/l2tp.md` - the `lcp` container block and its hot-apply table rows
- `docs/guide/pppoe.md` - the same container on the PPPoE page
- `docs/research/l2tpv2-ze-integration.md` - declared by `start_session.go`; the transport-to-PPP boundary gains four fields
- `docs/architecture/l2tp/bng-5-pppoe.md` - declared by `pppoe/server.go`; the PPPoE session's LCP timing becomes operator-configurable
- `plan/spec-finish-l2tp.md` - point its "LCP restart-counter (L163)" bullet at this spec

## Files to Create
- `internal/component/l2tp/ppp/yang/ze-ppp-lcp.yang` - the module declaring the `lcp` grouping, its four leaves, and the RFC's SHOULD values as `default`
- `internal/component/l2tp/ppp/yang/embed.go`, `doc.go`, `register.go`, `schema_test.go` - the embed-and-register pattern copied from `internal/component/l2tp/yang/`
- `internal/component/l2tp/ppp/rfc1661_restart_test.go` - the tagged tests
- `internal/component/l2tp/ppp/lcp_bootstrap_test.go` - AC-15
- `test/parse/l2tp-lcp-restart.ci`, `test/parse/l2tp-lcp-restart-range.ci`, `test/parse/pppoe-lcp-restart.ci`
- `test/interop-l2tp/scenarios/lcp-restart-timer-pppd/` - `ze.conf`, `xl2tpd.conf`, `ppp-options`, `l2tp-secrets`, `README.md`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/l2tp/ppp/yang/ze-ppp-lcp.yang`, used by `ze-l2tp-conf.yang` and `ze-pppoe-conf.yang` |
| YANG validation constraints | Yes | `range "100..60000"` on the timer, `range "1..255"` on each counter, `uint32` and `uint8` types |
| YANG custom validators | No | Native `range` is sufficient; no cross-leaf constraint exists |
| CLI commands/flags | N-A | Config-only. No command is added; `ze config show` renders the container from the schema |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | Yes | Automatic for typed leaves; no `CompleteFn` needed |
| Functional test for new RPC/API | Yes | `test/parse/l2tp-lcp-restart.ci` and the two beside it |
| Pipe completeness | N-A | No new command output |
| Env var registration | N-A | The leaves sit under `l2tp` and `pppoe`, not under `environment/` |
| Doctor check for runtime dependencies | N-A | No new file path, socket, service, kernel module, port or certificate |
| Prometheus counters/metrics | No | The Restart counter is per-session negotiation state rather than an operator-facing aggregate; `internal/component/l2tp/metrics.go` gains nothing |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - configurable LCP retransmission |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/guide/l2tp.md`, `docs/guide/pppoe.md` |
| 3 | CLI command added/changed? | No | No command added |
| 4 | API/RPC added/changed? | No | No RPC added |
| 5 | Plugin added/changed? | No | L2TP and PPPoE are components, not plugins |
| 6 | Has a user guide page? | Yes | `docs/guide/l2tp.md`, `docs/guide/pppoe.md` |
| 7 | Wire format changed? | No | Retransmission timing changes; no packet layout does |
| 8 | Plugin SDK/protocol changed? | No | `ppp.StartSession` is internal to `internal/component/l2tp/` |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc1661.md`; `rfc/requirements/rfc1661.md` and the `docs/features/rfc-status.md` row regenerate through `./le rfc index-update` |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` and `docs/architecture/testing/interop.md` gain the named scenario |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` - verify whether an LCP timer row exists rather than assuming |
| 12 | Internal architecture changed? | Yes | the PPP FSM description in `docs/research/l2tpv2-implementation-guide.md`, which `ppp_fsm.go` declares in its `// Design:` header |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | No | None added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | a new PPP session event type is added; check `docs/plugin-overview.md` and `docs/guide/status.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | DERIVED | `./le spec citation anchors spec plan/immediate/spec-lcp-restart-counters.md` lists them; answer from that output, never from memory. Three documents are DECLARED by the `// Design:` header of a file this spec modifies, so each is named here with its verdict. `docs/research/l2tpv2-ze-integration.md` is declared by `start_session.go` and describes the transport-to-PPP boundary: AFFECTED, because four fields join that contract. `docs/architecture/l2tp/bng-5-pppoe.md` is declared by `pppoe/server.go` and describes the PPPoE access concentrator: AFFECTED, because the PPPoE session now carries LCP restart config. `plan/spec-le-is-a-ze-binary.md` is declared by `internal/le/interoplab/l2tp/l2tp.go` and governs where `le` code lives rather than what the L2TP lab asserts: UNAFFECTED, because registering one more scenario name changes no binary boundary |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/l2tp.md` shows the `authentication` and `ncp` containers; the `lcp` example goes beside them |

## What This Spec Owes `rfc/short/rfc1661.md`

`rfc1661` is ENROLLED. `rfc/enrolled.txt` line 31 carries its enrolment reason and
the summary's `Enrolment` row reads `enrolled`. It is UNSIGNED: no
`rfc/extraction/rfc1661.json` exists, so the stem is one of the grandfathered
summaries `docs/contributing/rfc-conformance-gates.md` describes, published as a
counted backlog rather than blocked.

Two consequences decide what this spec must do, and the first corrects the premise
this spec was commissioned under:

1. **The summary already declares all five obligations.** `rfc/short/rfc1661.md` carries `RFC1661-4.6-1` through `4.6-4` and `RFC1661-4.4-2` as `[MUST]` checklist rows with `{gap:}` reasons that name the producing symbols, and its `Support remaining` row lists them as gated gap group (7). The obligations are visible on the public ledger today. What is missing is the implementation and its proof, not the declaration. The homelessness this spec fixes is that no SPEC owns the work.
2. **The extraction sign-off is NOT a precondition of this spec.** The sign-off gates a NEW enrolment, and `rfc1661` enrolled before that gate existed. `checkExtractionRatchet` compares a stem against its own HEAD row, and this stem has no baseline, so nothing here is blocked on it. Writing `rfc/extraction/rfc1661.json` is separate work on the corpus-wide drain schedule in `rfc/drain-budget.txt`, and this spec MUST NOT attempt it as a side effect: a walk of a 60-page RFC done to unblock five requirements is the quota-driven signature that schedule exists to prevent.

The summary edits this spec owes:

| Row | Change |
|-----|--------|
| `RFC1661-4.6-1` | `{gap}` removed, positive and negative test bindings written from the tagged tests |
| `RFC1661-4.6-2` | the same |
| `RFC1661-4.6-3` | the same |
| `RFC1661-4.6-4` | the same |
| `RFC1661-4.4-2` | the same |
| `Support remaining` | gap group (7) deleted, and the count "Twenty-four MUST gaps" becomes nineteen |
| `Enrolment reason`, in both `rfc/short/rfc1661.md` and `rfc/enrolled.txt` | the "no configurable Restart timer / Max-Terminate / Max-Configure / Max-Failure, zrc no-op" clause moves out of the gap list |

`rfc/requirements/rfc1661.md`, `ai/RFC-REQUIREMENTS.md` and the
`docs/features/rfc-status.md` RFC 1661 row are GENERATED. They are refreshed with
`./le rfc index-update` and are never hand-edited.

Each of the five requirements owes a discrimination record in the same change,
written by `./le rfc discriminate-record` and stored under `rfc/discrimination/`.
`./le rfc check` refuses a tagged unit that the commit under test added against
`HEAD^` and that carries none.

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the config path exists end to end and the session can see the values
   - Tests: `TestL2TPParseLCPContainer`, `TestPPPoEParseLCPContainer`, `TestL2TPStartSessionCarriesLCPRestartConfig`, `TestPPPoEStartSessionCarriesLCPRestartConfig`, `TestPPPManagerAppliesLCPRestartConfig`, `TestPPPLCPGroupingDefaultsResolveInBothModules`
   - Files: `ze-ppp-lcp.yang` and its embed and register files, both conf modules, `config.go`, `reactor_kernel.go`, `pppoe/server.go`, `start_session.go`, `manager.go`, `session.go`
   - Verify: the values reach `pppSession` and nothing reads them yet, so every behavior test below fails
2. **Phase: the Restart timer** -- the timer becomes action-driven and configurable
   - Tests: `TestRFC1661RestartTimerUsesConfiguredValue`, `TestRFC1661RestartTimerDefaultsToThreeSeconds`, `TestRFC1661RestartTimeoutRaisesFSMEvent`
   - Files: `session_run.go`
   - Verify: `scr`, `str` and `zrc` arm the timer, leaving a timer-running state stops it, and the tick raises an FSM event rather than calling `sendConfigureRequest`
3. **Phase: the Restart counter** -- `irc` and `zrc` set it, expiry decrements it, and `TO-` becomes reachable
   - Tests: `TestRFC1661MaxConfigureLimitsConfigureRequests`, `TestRFC1661MaxConfigureNotConsumedWhenAckArrives`, `TestRFC1661MaxTerminateLimitsTerminateRequests`, `TestRFC1661MaxTerminateStopsOnTerminateAck`, `TestRFC1661ZeroRestartCountSetsTimeoutPeriod`, `TestRFC1661InitializeRestartCountSetsMaxConfigure`, `TestRFC1661InitializeRestartCountPicksMaxTerminateOnTerminateEdge`
   - Files: `session_run.go`, `session.go`
   - Verify: Max-Configure and Max-Terminate bound retransmission in both directions
4. **Phase: `defaultNegoTimeout` deleted** -- one bound on LCP negotiation, the RFC's
   - Tests: `TestRFC1661MaxConfigureBoundsNegotiationWithoutNegoTimer`
   - Files: `session_run.go`
   - Verify: the constant and the timer are gone, and a default session is still abandoned after about 30 seconds
5. **Phase: Max-Failure** -- Naks are counted and converted
   - Tests: `TestRFC1661MaxFailureConvertsNakToReject`, `TestRFC1661MaxFailureBelowThresholdStillNaks`
   - Files: `session_run.go`
   - Verify: the threshold converts, and locally desired options are not appended past it
6. **Phase: `tld`** -- the upper layers hear that LCP left Opened
   - Tests: `TestRFC1661ThisLayerDownOnLeavingOpened`, `TestRFC1661NoThisLayerDownWhenStayingOpened`
   - Files: `session_run.go`, `events.go`, `ncp.go`
   - Verify: the notification precedes the accompanying actions on every edge leaving Opened, and is absent on the edges that stay
7. **Phase: bootstrap defaults** -- the SHOULD values are declared once
   - Tests: `TestLCPBootstrapConstantsMatchSchemaDefaults`
   - Files: `lcp_bootstrap_test.go`
   - Verify: the test derives its population from the code, and fails when a constant and its leaf disagree
8. **Phase: interop and the ledger**
   - Tests: `lcp-restart-timer-pppd`, then `./le rfc discriminate-record` for each of the five requirements
   - Files: the scenario directory, `internal/le/interoplab/l2tp/l2tp.go`, `Dockerfile.lac`, `rfc/short/rfc1661.md`, the doc pages
   - Verify: the scenario is RED with the change reverted and the LAC image rebuilt, GREEN with it restored, and the RED result is recorded

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC has an implementation at file and symbol, and each of the five requirement ids has both polarities bound |
| Feature completeness | Both transports reach the leaves. An L2TP-only implementation leaves PPPoE subscribers on compiled-in constants, and the MUST unmet there |
| Correctness | The timer is armed only by `scr`, `str` and `zrc`, and stopped on leaving a timer-running state, per Section 4.2. `irc` picks Max-Terminate on a `str` edge and Max-Configure on a `scr` edge |
| Correctness | The counter decrements on EVERY transmission including the first, per Section 4.4, so `max-configure 3` yields 3 requests rather than 4 |
| Naming | The YANG leaf, the Go field, the `Parameters` field and the `StartSession` field are derivable from each other, and leaf names carry no abbreviation |
| Data flow | The Restart counter lives on `pppSession` only, and `LCPDoTransition` stays a pure function of state and event |
| Rule: `ai/rules/no-layering.md` | `defaultNegoTimeout` is DELETED, not left standing beside Max-Configure |
| Rule: `ai/rules/principles.md` | No zero is a valid-looking answer: a zero Max-Configure is refused by the schema, and a session that cannot read its configured period says so rather than falling back silently |
| Rule: `ai/rules/stale-comments.md` | The "restart-counter management deferred to a 6a hardening pass" comment names `plan/deferrals/`, a directory deleted on 2026-09-05; it goes with the code it described |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Four leaves declared once | `grep -c default internal/component/l2tp/ppp/yang/ze-ppp-lcp.yang` returns 4, and a grep for a literal three-second duration in `internal/component/l2tp/ppp/` returns nothing |
| Both transports carry the container | `./le config coercion check`, plus the three `.ci` tests |
| `defaultNegoTimeout` gone | `grep -rn defaultNegoTimeout internal/` returns nothing |
| Five requirements proven in both polarities | `./le rfc check`, and `rfc/requirements/rfc1661.md` shows a test in both columns for each id |
| Discrimination records written | `./le rfc discriminate-record`, then five entries under `rfc/discrimination/` |
| Interop scenario discovered and run | `./le integration` with the scenario selector `lcp-restart-timer-pppd` |
| Documentation carries the container | grep `docs/guide/l2tp.md` and `docs/guide/pppoe.md` for the `lcp` block |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The three counters and the timer come from config, so the schema `range` is the whole guard; a zero or a negative period must be unreachable rather than clamped silently |
| Resource exhaustion | A 100 ms Restart timer across the configured `max-sessions` is the worst case. The per-session timer count is unchanged, so only its period moves |
| Off-path forgery | A peer that floods Configure-Requests must not extend the negotiation indefinitely: each `scr` decrements the counter, so a flood shortens the exchange |
| Error leakage | The refusal message for an out-of-range value names the range, never internal state |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| An NCP or IPCP test goes red once `tld` notifies | That red is R-2 surfacing. Read the producer and fix the product, never the assertion |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The RFC's own text supplies the arming rule, so no design choice is needed for it. Section 4.2 names `scr`, `str` and `zrc` as the three actions that start or re-start the timer, and states that it stops on leaving a timer-running state. The current ticker is a different mechanism that happens to retransmit.
- `irc` needs no per-state table. The transition already carries the accompanying action: `str` beside `irc` means Max-Terminate, and `scr` beside it means Max-Configure. A table would be a second declaration of `LCPDoTransition`.
- The RFC's SHOULD defaults multiply out to today's behavior. Ten transmissions at three seconds is the 30 seconds `defaultNegoTimeout` currently enforces by a different route, which is why the second bound can be deleted rather than reconciled.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One YANG grouping in a new `ze-ppp-lcp` module, used by both conf modules | Four leaves declared twice, once per transport; or the leaves in `ze-types.yang` | Twice is two declarations of the RFC's SHOULD values, and they will disagree. `ze-types` is a central module, and a PPP spelling there is what `ai/rules/plugins.md` forbids. The `ppp` package owns PPP, and both transports already import it in Go |
| Milliseconds for the Restart timer | Seconds, matching `hello-interval` and `authentication/timeout` | Section 4.6's Implementation Note asks for faster retransmission on low-latency links, and a seconds-only leaf cannot express it. The name is spelled out in full per `ai/rules/config.md` |
| Delete `defaultNegoTimeout` | Keep it as an outer safety bound | `ai/rules/no-layering.md`: Max-Configure is the RFC's bound on the same thing. Two bounds means an operator who raises `max-configure` is still cut off at 30 seconds with nothing naming why |
| `tld` in scope, `tls` out | Implement all four `tl*` actions; or defer `tld` to a separate spec | `tld` shares a transition with `zrc` and terminates the counter's path. `tls` fires only on entering Starting, which nothing in this spec reaches |
| A new named interop scenario rather than extending `01-ppp-ipv4` | Add assertions to the existing scenario | `01-ppp-ipv4` drives the path where no retransmission happens, so nothing in it can go red on this change. The retransmission path needs a silenced peer, which changes the scenario's whole shape |

## Known Limitations
- Restart timer backoff, which Section 4.6 offers as a MAY, stays unimplemented. `RFC1661-4.4-1` records it as `{not-applicable}` on the ground that ze applies no backoff, and that record stays true after this spec. Offering backoff is a MAY clause and belongs to the user, not to this spec.
- The four existing L2TP interop scenario directories keep their banned numeric prefixes. Renaming them touches four directories, four constants in `internal/le/interoplab/l2tp/l2tp.go`, and every citation of those names. It is its own work and is not folded into a conformance change.
- The `rfc/extraction/rfc1661.json` sign-off stays unwritten, on the corpus-wide drain schedule in `rfc/drain-budget.txt`.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

The five obligations quoted in the Task table go above their enforcing sites: the
timer arm, and each of the four configured values. Section 4.2's rule about which
actions start the timer goes above the arming helper, and Section 4.4's `irc`
sentence above the counter assignment. No wire format changes, so no new ASCII
diagram is owed and the LCP packet diagrams in `rfc/short/rfc1661.md` stay correct.

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
- [ ] AC-1..AC-15 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`)
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
