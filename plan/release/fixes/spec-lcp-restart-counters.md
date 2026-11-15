# Spec: lcp-restart-counters

| Field | Value |
|-------|-------|
| Status | in-progress |
| Priority | P1 |
| Release reason | Implemented PPP negotiation owes the RFC restart-counter and configurable timer behavior. |
| Scope | protocol |
| Depends | - |
| Phase | 8/8 |
| Handoff | - |
| Updated | 2026-11-15 |
| RFC gaps | RFC1661-4.6-1, RFC1661-4.6-2, RFC1661-4.6-3, RFC1661-4.6-4, RFC1661-4.4-2 |

<!-- Release fix: configurable LCP timing and counters on implemented PPP roles,
     preserving existing restart and upper-layer cleanup behavior. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

### Full completion requested 2026-11-14

The owner requested completion of the entire spec and closure. The first client
slice below is committed in `3fe3b0c13b`. Server configuration, both roles'
Max-Failure policy, independent client IPv6CP timing and their tests are now
implemented. Linux wire, rebuilt pppd contrasts, CLI promotion and post-fix
regressions have passed. All 69 native RFC observations, affected audit
refreshes and final scoped checks are complete. Phase 8/8 is ready for the
two closure commits; the spec is not closed until those commits land.

| Current implementation, verified from source | Observed proof and limits |
|----------------------------------------------|---------------------------------|
| `pppSession.applyTransition`, `spendRestartCount` and `handleRestartTimeout` retain IRC/ZRC, per-transmission spending and one-shot timing, now reading immutable `LCPConfig`. Accepted proxy Opened initializes the retained request budget in `run` | Eight affected race-enabled package populations passed. Current native records/audits verify. Final RFC check names none of the seven owned stems |
| `performAction(LCPActTLD)` resets NCP, publishes `EventLCPDown` and waits for transport cleanup before subsequent wire actions | Ordering and Echo contrast passed in the complete PPP regression |
| `updateLCPConvergence` replaces the fixed initial timeout with an edge-owned deadline of twice Restart interval times Max-Configure | Both configured-bound and nested-restart native redwalks observed clean PASS and broken FAIL |
| `sendConfigureNakOrReject` and client `negotiateLCP` count successfully sent Naks and convert further unacceptable peer options to Reject | Threshold, renewal and original-instance regression passed. Native observations/audits are current |
| Client `clientIPv6CP` owns a configured one-shot timer through initial and connected phases; IPCP retains its independent three-second timer | Strong kernel normal/100 ms/3 s/restored proof passed. All five CLI tests passed normal and 80 loaded runs, discrimination and restored live proof |

Both server transports use the shared `restart-timer` leaf with
`units milliseconds`. AC-9 uses the RFC table's Stopped destination. Runtime
defaults come only from the shared YANG grouping, so AC-15 checks that authority
through both consumers instead of pairing duplicate Go bootstrap constants.

Assumption A-2 is broken for a peer sending valid Naks: IRC can refresh the
counter on each reply. Removing every convergence bound would leave such a
session unbounded. A fixed timeout beside configurable timing [workaround]
would truncate long configured budgets. A deadline derived from those settings
[source] preserves a lifecycle bound without truncating the silent-peer budget.
The owning producer is `pppSession.run`; the counter remains owned by its FSM.
R-1's earlier claim that the counter bounds every responding peer is superseded.

Scoped tests run in the main checkout. The main thread records actual runs in
the owning session state. Authored tests and prepared RFC commands do not count
as executed proof. No scope has been removed from AC-1 through AC-22.

### First client slice: historical scope and evidence, 2026-11-14

The current implementation slice completes the three remaining PPPoE-client
review items: discriminating proof of the RCN Restart-counter reset, configurable
Restart timer and Max-Configure, and an accurate validation history in the client
architecture page. The server obligations below remain owned here; this slice
does not close the whole spec. The client's one-shot Restart timer and real
Restart counter (`a02374d9ad`) are configured through the existing timing carrier.

| Client criterion | Observable result |
|------------------|-------------------|
| C-0 | The native runner starts: its payload-command Summary fits the existing 60-character contract. The separate one-line repair preserves the other task's uncommitted tooling implementation. |
| C-1 | After nine unanswered requests, a valid Nak or Reject in Req-Sent or Ack-Sent gives a fresh budget of ten requests. Subsequent silence ends LCP after that budget; deleting the reset makes this test fail. |
| C-2 | `interface pppoe-client <name> lcp restart-timer` (milliseconds) and `max-configure` reach the client's initial and restarted LCP negotiation through the existing timing carrier. Defaults remain 3000 and 10, read from YANG. Native schema constraints and Go validation refuse invalid values. |
| C-3 | The convergence deadline is twice the configured silent-peer budget, so configured intervals/counts are usable without the previous fixed 60-second bound truncating them. Default convergence behavior remains 60 seconds. |
| C-4 | A timing-only reload preserves the running session and affects the next Dial, as the existing keepalive/reconnect settings do. The connected session retains its timing snapshot for renegotiation. |
| C-5 | The client architecture page distinguishes historical 7.2.9 evidence, the later 7.3 closure record, and checks run for this slice; missing scratch logs are not presented as retained evidence. |

The configuration extends `PPPoEClientTiming`, its existing schema-backed parser,
and the dialer's `sessionConfig`; it adds no independent default registry or
environment override. The schema owns both defaults. The direct negotiator tests
must supply explicit valid timing rather than relying on zero-value defaults.
Wiring proof parses real client configuration and observes Configure-Request
counts and intervals through the same timing carrier the dialer uses. Positive
configured values and omitted-leaf defaults, invalid bounds, RCN reset contrasts,
and a successful negotiation before exhaustion preserve useful behavior.

Files reached: `internal/component/iface/yang/ze-iface-conf.yang`,
`internal/component/iface/pppoe_client_timing.go`, its tests and direct timing
fixtures; `internal/component/l2tp/pppoeclient/dialer.go`, `session.go`,
`lcp_restart_timer_test.go` and direct session fixtures; the client architecture
page and the PPPoE operator guide. This slice alone did not prove server-wide
configurability.

### Client completion evidence

| Criterion | Observed evidence, 2026-11-14 |
|-----------|-------------------------------|
| C-0 | Native runner starts after shortening the payload Summary to 53 characters; its existing startup guard enforces 60. Only this Summary hunk is carried from the concurrent tooling file. |
| C-1 | `TestLCPReplyRenewsSpentRestartCounter` passes normally; removing only the RCN reset through a Go overlay fails all four Req-Sent/Ack-Sent and Nak/Reject cases at 27.5 s instead of 54.5 s. Final normal `pppoeclient` and `iface` package tests pass with race detection. |
| C-2 | All three configured-LCP tests in `internal/component/iface/pppoe_lcp_timing_integration_linux_test.go` pass on Ze 7.3.0-ze. Forcing Dial's timer to three seconds fails both budget tests at the second request; the normal binary passes again. CLI ranges pass six boundary commands and 80 invocations under load. Forcing `outputValidateText` to report invalid input as valid fails the CLI test at the 99 ms refusal; the normal binary passes afterward. |
| C-3 | `TestLCPConfiguredBudget` observes configured gaps and exactly three requests for 500 ms and 60 s timers, including a 180 s silent-peer budget. `TestLCPConfiguredConvergenceDeadline` observes the configured 100 s convergence bound. |
| C-4 | `TestPPPoEClientLCPReloadKeepsSessionSnapshot` keeps the same client, retains its initial timing snapshot, and hands the new timing to exactly the next Dial. |
| C-5 | The architecture page cites immutable closure history and separately records the new scripted-AC clean/broken/clean proof with the exact test names. No fresh third-party interop result is claimed. |

Independent review ran all three source lenses. Its two test findings were
fixed: the actual Dial timer must be observed, and namespace entry must be the
first statement of a top-level test. Review of those fixes found no remaining
source ISSUE. The independent test judge approved all 17 directly or indirectly
changed RFC-tagged units. The 21 affected existing discrimination records were
retaken; ten RFC 1661 judgments and RFC2516-x-9 were rejudged, and five
byte-identical option-test judgments were mechanically resealed.

Verification limits: the package lint reports 16 existing findings in each
profile, including value-copy warnings already above their configured thresholds
before the two new timing fields. The final RFC check reports 136 repository-wide
violations and none for RFC 1661, RFC 2516 or the client paths. The review's
mechanical repository check also reports nine existing export/tooling/VPP
findings. These are recorded limits of the checkout, not passing whole-tree gates.

The first client slice is committed. Its execution record does not replace the
final regression and proof completed for the later server, Max-Failure and IPv6CP edits.

The owning current state and smallest retained test evidence are under
`tmp/session/2026-11-14-213245/`. Completed test binaries and source overlays are
disposable once superseded by retained proof and the corresponding commit.

Diagnosis: the Nak-loop test continually resets the Restart timer before expiry,
so its deadline assertion cannot observe a missing counter reset. A weaker
comment-only repair [workaround] would leave the reset unprotected. Spending the
counter before a reply, then observing timeout under silence [source], exercises
the reset itself. The configuration defect belongs to the existing schema-to-
session timing path; an environment-only knob would leave operators without
schema validation and is rejected. The stale validation sentence belongs to the
architecture page and must be corrected against the closure commits.

| Client assumption/risk | Validation/mitigation |
|------------------------|-----------------------|
| Adding fields to Timing preserves reload semantics | Read reconciliation and cover the timing snapshot carried to the next Dial; retained connected config must not be mutated. |
| A fixed convergence bound could truncate configured budgets | Derive it from validated bounded interval and count; test a budget above 60 seconds using the virtual clock. |
| Historical scratch evidence may have been removed | Cite immutable closure commits and label them historical, without claiming a fresh kernel/interop run. |

RFC 1661 Section 4.6 states four MUST-level obligations on the LCP restart
machinery, and Section 4.4 states a fifth on Zero-Restart-Count. This spec owns
all five. The original audit predated the September restart-counter and TLD
fixes. Those fixes already supplied the automaton behavior when this work
resumed. The summary now declares the implemented requirements without a gap
annotation. Their final recorded discrimination and audits are current.

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
2. Preserve and prove the existing Restart counter, so `irc`, `zrc`, `TO+` and `TO-` use the configured limits and retain their RFC meaning.
3. Preserve and prove `tld`: upper layers remove the old network lifetime before the responses accompanying departure from Opened.
4. Carry applicable settings to the PPPoE client: LCP Restart timer, Max-Configure and Max-Failure, plus an independent IPv6CP Restart timer. The client sends no local LCP Terminate-Request and closes its transport with PADT, so it gains no unused Max-Terminate leaf. The 2026-11-07 journal row is fixed by both configurable timer paths and their Linux normal/cut/restored proof.

### The automaton question, and why it is in scope

The commissioning audit described `LCPActTLD` as a no-op. That observation is
historical: current `performAction` already performs the cleanup. The Section 4.4
description of `tld` ("This action indicates to the upper layers that the
automaton is leaving the Opened state") carries no RFC 2119 keyword. That reading
of the text is correct. Preserving and testing TLD remains in scope:

- The one edge that carries `zrc` is `Opened + RTR -> tld,zrc,sta / Stopping`. `RFC1661-4.4-2` is enforced on that edge and nowhere else, so implementing the fifth MUST means editing the same transition whose first action is `tld`. Landing `zrc` beside a `tld` that stays silent leaves the transition half-implemented in the same `performAction` loop.
- `TO-` fires `tlf` from Closing and from the three negotiating states, and every edge leaving Opened for those states carries `tld` first. A counter whose expiry drives a state change that no upper layer hears is a counter that changes nothing an operator or a subscriber can observe, which is the shape `ai/rules/principles.md` names: a value that is silently wrong is reachable.

AC-11 and AC-12 retain that obligation. This work adds no new TLD event or TLS
behavior.

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
  → Decision: `rfc1661` is enrolled and `rfc/extraction/rfc1661.json` records sign-off on 2026-09-21. This spec changes no extraction scope and owes the current tagged tests' proof and audit refresh.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc1661.md` - the five requirement rows, their capability boundaries and current test bindings
  → Constraint: the summary is the authored declaration; `rfc/requirements/rfc1661.md` and the `docs/features/rfc-status.md` row are GENERATED by `./le rfc update` and must never be hand-edited
- [ ] `rfc/full/rfc1661.txt` Sections 4.1, 4.2, 4.4, 4.6 - the transition table, the timer rule, the action semantics, the counters
  → Constraint: `irc` "sets the Restart counter to the appropriate value (Max-Terminate or Max-Configure)", so which value is appropriate is decided per transition, not per state

**Key insights:** (minimal context to resume after compaction)
- All five obligations already had requirement ids. Current summary edits remove the gaps, but records and audits must prove the current tagged assertions.
- The existing one-shot timer and Restart counter are extended, not replaced. The RFC binds timer arming to SCR, STR and ZRC.
- The deleted fixed 30-second bound is replaced by counter-driven silence exhaustion and a separate, derived bound for responsive nonconvergence.

## Current Behavior (MANDATORY)

The following describes the working source on 2026-11-15, not a claim that all
final execution has passed.

| Producer | Current behavior |
|----------|------------------|
| `pppconfig/lcp.go` `ParseLCP`, `loadLCPDefaults` | Reads shared YANG defaults, rejects malformed and unknown provider settings, returns validated immutable `ppp.LCPConfig` |
| `l2tp/config.go` `ExtractParameters`, `reactor_kernel.go` `handleKernelSuccess`; `pppoe/config.go` `ExtractParameters`, `server.go` `handlePADR` | Both transports carry their admitted configuration through `StartSession.LCP` |
| `ppp/manager.go` `spawnSession` | Refuses an invalid snapshot, closes its already-transferred descriptors and copies an admitted value into `pppSession` |
| `ppp/session_run.go` `applyTransition`, `spendRestartCount`, `handleRestartTimeout` | IRC chooses the companion SCR/STR limit, each request spends a count, expiry raises TO+ or TO-, and ZRC arms the configured grace with count zero |
| `ppp/session_run.go` `updateLCPConvergence` | Entering Req-Sent, Ack-Rcvd or Ack-Sent starts a derived lifetime bound. Entering Opened or a non-negotiating state stops it. Replies within one lifetime cannot renew it |
| `ppp/session_run.go` `sendConfigureNakOrReject`, `sendConfigureAck`, `LCPRejectNakOptions` | Only successful Naks spend Max-Failure. A successful Ack renews it. Conversion preserves only the original refused option instance |
| `ppp/session_run.go` `performAction`; `ppp/ncp.go` `resetNCP` | Existing TLD clears the old network state and waits for transport cleanup before SCR/SCA/SCN |
| `pppoeclient/session.go` `negotiateLCP` | Configured LCP timer, request budget, derived convergence deadline and Max-Failure apply to initial and restarted negotiation |
| `pppoeclient/ipv6cp_client.go` `sendRequest`, `receiveNak`, `tick` | IPv6CP owns the configured one-shot timer. Req-Sent/Ack-Sent RCN renews its ten-request silence budget. Ack-Rcvd RCN does not |

The shared server default remains three seconds, ten Configure-Requests, two
Terminate-Requests and five Naks. A silent default peer exhausts negotiation
after 30 seconds. A responsive server Nak loop instead reaches the derived
60-second convergence bound. Accepted proxy LCP starts Opened without initial
requests. CHAP and the proxied PAP wait keep their separate authentication timing.

## Existing Mechanisms (MANDATORY)

The September restart-counter and TLD fixes already existed when implementation
resumed. This work extends those mechanisms and does not add a parallel FSM or
another LCP-down event.

| Mechanism | Where | What it does today | This spec |
|-----------|-------|--------------------|-----------|
| LCP Restart counter and timer | `internal/component/l2tp/ppp/session_run.go` `spendRestartCount`, `stopRestartTimer`, `restartTimerRuns`, `handleRestartTimeout` | books each Configure-Request and Terminate-Request against `restartCount`, re-arms a one-shot Restart timer, raises TO+ or TO- into the table, stops the timer in states without a TO edge | extends: the timer period, Max-Configure and Max-Terminate become configured values the counter and timer read, instead of constants |
| RFC 1661 automaton | `internal/component/l2tp/ppp/ppp_fsm.go` `LCPDoTransition` | the full ten-state table, pure | does not apply: the table is correct and is not edited |
| Per-session PPP inputs | `internal/component/l2tp/ppp/start_session.go` `StartSession.LCP` | carries one immutable configuration snapshot | extends: both transports supply the validated four-setting snapshot |
| L2TP config parse | `internal/component/l2tp/config.go` `ExtractParameters` | reads `lcp` through the shared `pppconfig.ParseLCP` | extends the existing configuration path |
| PPPoE client LCP negotiation | `internal/component/l2tp/pppoeclient/session.go` `negotiateLCP`; `ipv6cp_client.go` `clientIPv6CP` | LCP and IPv6CP own separate one-shot timers | extends the existing timing snapshot and ordered reader, retaining IPCP's independent timing |
| PPPoE client timing leaves | `internal/component/iface/yang/ze-iface-conf.yang` `pppoe-client` `keepalive`, `reconnect`; `internal/component/iface/config.go` `PPPoEClientTiming` | parse, default and carry the echo and redial timing to the client | extends (goal 4): the precedent for adding the client's LCP timing leaves |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Operator config text: `lcp` under `l2tp` and `pppoe`, carrying `restart-timer`, `max-terminate`, `max-configure` and `max-failure`.
- Client config: `interface pppoe-client <name> lcp` and `ipv6cp restart-timer` through the existing timing carrier.
- Wire: an LCP packet arriving on the chan fd, and the expiry of the Restart timer.

### Transformation Path
1. Config text parses through the YANG tree; the `lcp` container resolves from the shared grouping with its schema defaults applied.
2. Both transport readers call `pppconfig.ParseLCP` and receive one immutable `LCPConfig`.
3. `reactor_kernel.go` and `pppoe/server.go` copy it onto `ppp.StartSession.LCP`.
4. Queue delivery transfers descriptor ownership. `Driver.spawnSession` validates the snapshot before launching the session, closes refused descriptors and copies admitted values into `pppSession`.
5. IRC and ZRC assign the counter. SCR and STR spend it, including the first transmission, and arm the one-shot timer. Expiry selects TO+ or TO- from the remaining count.
6. `LCPDoTransition` answers with the state and actions; `performAction` executes them; `tld` emits the LCP-down notification.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree ↔ L2TP subsystem | `ExtractParameters` calls `pppconfig.ParseLCP` | Both named Linux parser/admission tests passed on Ze 7.3 |
| Config tree ↔ PPPoE subsystem | `ExtractParameters` calls the same parser | Both named Linux parser/admission tests passed on Ze 7.3 |
| Transport ↔ PPP | `StartSession.LCP` → `spawnSession` → `pppSession.lcpConfig` | All eight affected race-enabled package populations passed after the proxy fix |
| PPP FSM ↔ upper layers | Existing `EventLCPDown` and cleanup acknowledgment | Ordering and Echo contrast passed in the complete PPP regression |
| Ze ↔ pppd | Relay injection and real LCP retransmissions | Actual clean/broken/restored scenario passed its discrimination walk |

### Integration Points
- `ppp.StartSession.LCP` extends the existing transport-to-PPP contract with one immutable value, not a new channel.
- `LCPDoTransition` stays pure and unchanged. Existing timeout delivery now uses configured settings.
- The existing session event channel retains `EventLCPDown` and its acknowledgment contract.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Source and Linux admission tests hold | Shared schema → transport parser → immutable snapshot → existing driver |
| No unintended coupling | Source holds | `pppconfig` bridges schema and leaf `ppp` without making `ppp` import config |
| No duplicated functionality | Source holds | Existing counter, event and timer actions remain the producers |
| Zero-copy preserved where applicable | Source holds | Packet encoders and ownership stay on their existing paths |
| Registration over hardcoding | Source holds | Shared YANG module registers through its package. No new operator command, event type or core feature list. Native glue check passed 168 directories |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | One shared grouping resolves defaults through both consumers | Both transport modules use `plcp:lcp`; `loadLCPDefaults` reads that grouping | Startup refuses missing or invalid schema; do not duplicate defaults | `TestPPPLCPGroupingDefaultsResolveInBothModules` and the shared-schema host package passed | confirmed |
| A-2 | Removing the fixed timeout leaves every responding negotiation bounded by the Restart counter | Valid RCN edges carry IRC in the RFC table | A valid Nak loop renews the counter forever | `TestConfiguredLCPConvergenceBound` and `TestConfiguredLCPLongSilentBudget` cover the replacement derived deadline | broken |
| A-3 | A transparent relay can inject a restart and selectively lose real pppd LCP replies while keeping the tunnel alive | `avprelay/lcprestart.go` and `checkLCPRestart` implement this | The real-peer run must observe an actual pppd reply and preserve tunnel control | Normal and restored pppd scenarios passed. Rebuilt fixed-three-second cut failed the checker | confirmed |
| A-4 | IRC chooses Max-Terminate from companion STR, otherwise Max-Configure | `applyTransition` reads the transition's actions | A second state-to-limit table would duplicate the FSM | `TestConfiguredLCPMaxTerminate`, including exact protocol and Code assertions | confirmed |
| A-5 | PPPoE subscribers use the shared driver and counter | `InterfaceServer.handlePADR` passes `StartSession.LCP` to `SessionsIn` | A disconnected snapshot would leave that transport misconfigured | `TestPPPoEStartSessionCarriesLCPRestartConfig` passed on Ze 7.3 | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Valid Naks refresh IRC and can keep a negotiating lifetime alive | Silence budget passes while a responsive Nak loop persists | The derived convergence timer belongs to actual FSM edges, including nested authentication restarts. Replies within that lifetime cannot renew it |
| R-2 | Changing TLD or cleanup acknowledgment order could remove replacement network state | NCP or lifetime test fails | Preserve existing cleanup and prove it precedes SCR/SCA/SCN. Do not emit a second LCP-down event |
| R-3 | A sub-second Restart timer on a busy LNS multiplies wakeups per session | `le perf` or a lab run shows CPU growth with session count | The schema `range` floor is 100 ms and the `ze:help` states the cost; the timer is per-session and already exists, so the change is its period rather than its count |
| R-4 | Max-Failure converting Nak to Reject can end a negotiation the peer would otherwise have converged | pppd logs a rejected option it needs | This is the RFC's prescribed behavior, the default of 5 is the RFC's, and the leaf lets an operator raise it |
| R-5 | A wire deadline alone could accept an incorrectly fast timer | Fixed 100 ms implementations survived the earlier IPv6CP and client LCP assertions | Strong 350..900 ms receipt assertions pass normally and reject 100 ms. IPv6CP also rejects 3 s. Both pass restored on Ze 7.3 |
| R-6 | Accepted proxy LCP skipped IRC and entered Opened with no request budget | A peer RCR produced one request instead of configured three or five | `run` initializes the admitted Max-Configure budget without arming the timer. New real-Driver silent/answering tests failed before the fix and now pass, including the complete PPP regression. Both native proxy observations passed |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every L2TP and PPPoE subscriber session. A counter that expires early tears down healthy sessions; one that never expires leaves half-negotiated sessions open. A `tld` notification sent on the wrong edge tears down IP state under a live subscriber. |
| How is a defect repaired? | Fix the owning producer and keep configuration, fixtures and docs consistent. No destructive git rollback is authorized. Absent leaves retain the RFC defaults. |
| Who else touches this path? | Both subscriber transports and the PPPoE client. Independent NCP lifetime work shares cleanup and reads the admitted LCP values. Shared-checkout edits must be preserved. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| An `lcp` container under `l2tp` in config text | → | `l2tp.ExtractParameters` fills `Parameters.LCP` | `TestL2TPParseLCPContainer` |
| An `lcp` container under `pppoe` in config text | → | the PPPoE config reader fills its parameters | `TestPPPoEParseLCPContainer` |
| `l2tp.Parameters` | → | `reactor_kernel.go` sets the fields on `ppp.StartSession` | `TestL2TPStartSessionCarriesLCPRestartConfig` |
| PPPoE parameters | → | `pppoe/server.go` sets the fields on `ppp.StartSession` | `TestPPPoEStartSessionCarriesLCPRestartConfig` |
| `ppp.StartSession` | → | `manager.go` sets them on the `pppSession` | `TestPPPManagerAppliesLCPRestartConfig` |
| Restart timer expiry in `run` | → | the session raises `TO+` or `TO-` into `LCPDoTransition` | `TestConfiguredLCPRestartTimer`, `TestConfiguredLCPMaxConfigure` |
| Client `lcp` and `ipv6cp` settings | → | `parsePPPoEClientTiming` → `Dial` → retained negotiation snapshot | `TestPPPoEClientLCPTimingFromConfig`, `TestPPPoEClientLCPMaxFailureFromConfig`, `TestPPPoEClientIPv6CPConfiguredTimerEndToEnd` |
| A server configuration and live subscriber | → | The admitted interval controls actual requests | `TestPPPoEStartSessionCarriesLCPRestartConfig` and `lcp-restart-timer-pppd` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `restart-timer 500` under `l2tp lcp`, and a peer that never answers the Configure-Request | Ze retransmits the Configure-Request about every 500 ms, not every 3000 ms |
| AC-2 | No `lcp` container in the config | The Restart timer period is 3000 ms, Max-Terminate is 2, Max-Configure is 10, and Max-Failure is 5 |
| AC-3 | `max-configure 3`, and a peer that never answers, during initial local negotiation or peer-triggered renegotiation after accepted proxy LCP | Exactly 3 Configure-Requests are transmitted, then LCP leaves the negotiating states for Stopped and the session goes down |
| AC-4 | `max-configure 3`, and a peer that answers with a Configure-Ack on the second request | Retransmission stops, fewer than 3 requests were sent, and the session reaches Opened |
| AC-5 | `max-terminate 2`, a Close, and a peer that never sends Terminate-Ack | Exactly 2 Terminate-Requests are transmitted, then LCP reaches Closed |
| AC-6 | `max-terminate 2`, and a peer that answers the first Terminate-Request with a Terminate-Ack | Exactly 1 Terminate-Request was transmitted and LCP reaches Closed |
| AC-7 | `max-failure 5`, and a peer that repeats a Configure-Request carrying the same unacceptable option | The first 5 answers are Configure-Nak; the 6th is a Configure-Reject naming that option, and locally desired options are not appended to it |
| AC-8 | `max-failure 5`, and four repeats of the same unacceptable option | The 4th answer is still a Configure-Nak |
| AC-9 | A Terminate-Request received while LCP is Opened | The Restart counter is zero and the Restart timer is armed at the configured period, so the next expiry is `TO-` and LCP moves from Stopping to Stopped rather than retransmitting |
| AC-10 | A valid Configure-Nak or Configure-Reject received in Req-Sent or Ack-Sent, whose RCN transition carries `irc,scr` | IRC renews the Max-Configure budget before SCR spends its first transmission. The next timeout selects TO+ while that budget remains. Ack-Rcvd RCN does not renew it |
| AC-11 | A Configure-Request received while LCP is Opened | An LCP-down notification reaches the upper layers before `scr` and the accompanying `sca` or `scn` action |
| AC-12 | An Echo-Request received while LCP is Opened | No LCP-down notification is emitted, and LCP stays Opened |
| AC-13 | `restart-timer 50`, or `max-configure 0` | Config validation refuses the value and names the accepted range |
| AC-14 | A default-configured session whose peer never answers | The session is abandoned after about 30 seconds, unchanged from today, with no `negoTimer` anywhere in the code |
| AC-15 | Runtime defaults for either server transport, with no LCP settings supplied | The shared YANG grouping is the sole runtime default authority, with no production Go bootstrap constants. A test resolves both consumers and compares their defaults with the runtime parser result |
| AC-16 | A valid responding peer continually Naks the server's Configure-Request | The derived convergence deadline ends the attempt; a configured silent-peer budget longer than 30 seconds is allowed in full |
| AC-17 | A client configures `ipv6cp restart-timer` | The initial and connected IPv6CP automata use that interval, defaults and range are schema-backed, IPCP timing is independent, and timing-only reload affects the next Dial |
| AC-18 | A late valid IPv6CP Configure-Nak arrives in Req-Sent or Ack-Sent after nine unanswered requests | The peer receives a fresh ten-request budget; the configured timer and Ack-Rcvd transition retain their RFC semantics |
| AC-19 | Max-Failure is exhausted and a server request contains duplicate or malformed duplicate options | The Reject carries only the original instance that earned the Nak, once per Type, and never includes an acceptable earlier instance |
| AC-20 | An operator configures `interface pppoe-client <name> lcp max-failure` | Its schema default is five, its range is 1..255, and the client sends that many consecutive Configure-Naks before rejecting further unacceptable peer options; a sent Configure-Ack renews the budget |
| AC-21 | A direct provider supplies an invalid server LCP value | The refusal is an `opererr` error and its final repair command names the actual `l2tp` or `pppoe` root and a valid integer value |
| AC-22 | A direct provider supplies an unknown LCP key | Parsing refuses it, suggests the closest declared setting when one exists, and supplies a valid transport-specific repair instead of silently using defaults |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Sets a 500 ms restart timer and `max-configure 3` on a low-latency access network, so a dead peer fails fast | config text -> YANG -> `l2tp.Parse` -> `StartSession` -> `pppSession` -> Restart timer -> `TO-` -> Stopped | the `lcp-restart-timer-pppd` interop scenario |
| 2 | Sets a 6000 ms restart timer under `pppoe` on a lossy access link | config text -> shared YANG/parser -> `StartSession.LCP` -> existing driver | `TestPPPoEParseLCPContainer` plus the configured shorter interval observed by `TestPPPoEStartSessionCarriesLCPRestartConfig` |
| 3 | Sets `max-failure 2`, so a repeated unacceptable option is rejected after two Naks | LCP Configure-Request -> `handleLCPPacket` -> `sendConfigureNakOrReject` -> Configure-Reject | `TestConfiguredLCPMaxFailure` |
| 4 | Runs a subscriber whose peer renegotiates LCP mid-session and expects the old IP state to end first | LCP Configure-Request in Opened -> TLD -> NCP cleanup -> LCP response | `TestLCPDownPrecedesRestartWireResponse` |
| 5 | Tunes a PPPoE client's IPv6CP wait independently of its IPv4 negotiation | registered config -> timing snapshot -> `Dial` -> `clientIPv6CP` | `TestPPPoEClientIPv6CPConfiguredTimerEndToEnd` |

## 🧪 TDD Test Plan

### Acceptance-to-producer and test map

These are the actual test names in the current source. The map records coverage
intent. The execution table that follows records observed runs and their
limits. Grouped ACs retain each criterion in the Acceptance Criteria table.

| AC | Producing function or declaration | Actual test and file |
|----|----------------------------------|----------------------|
| 1 | `pppSession.armRestartTimer` | `TestConfiguredLCPRestartTimer`, `ppp/rfc1661_configured_restart_test.go`; live `lcp-restart-timer-pppd` |
| 2, 15 | `pppconfig.loadLCPDefaults`, shared `ze-ppp-lcp.yang` | `TestPPPLCPGroupingDefaultsResolveInBothModules`, `ppp/yang/schema_test.go` |
| 3, 4 | `pppSession.run` accepted-proxy initialization, `applyTransition`, `spendRestartCount`, `handleRestartTimeout` | `TestConfiguredLCPMaxConfigure`, `ppp/rfc1661_configured_restart_test.go`; `TestProxyLCPRenegotiationUsesConfiguredRestartBudget`, `TestProxyLCPRenegotiationAnswerStopsRestartBudget`, `ppp/rfc1661_proxy_restart_budget_test.go` |
| 5, 6 | `pppSession.applyTransition`, `sendTerminateRequest` | `TestConfiguredLCPMaxTerminate`, same file; each frame must be LCP Terminate-Request |
| 7, 8 | `pppSession.sendConfigureNakOrReject`, `sendConfigureAck` | `TestConfiguredLCPMaxFailure`, same file |
| 9 | `pppSession.applyTransition` ZRC, `armRestartTimer` | `TestConfiguredLCPZeroRestartPeriod`, same file, with Closed's STA-only contrast |
| 10 | `pppSession.applyTransition` IRC | `TestLCPReplyRenewsConfiguredCounter`, same file, with Ack-Rcvd contrast |
| 11, 12 | `pppSession.performAction` TLD, `resetNCP` | `TestLCPDownPrecedesRestartWireResponse`, same file, accepted/refused RCR and Echo contrast |
| 13 | Shared YANG ranges, `ppp.NewLCPConfig`, `pppconfig.ParseLCP` | `TestLCPConfigRefusesInvalidSettings`, `ppp/lcp_config_test.go`; `TestParseLCPRefusesMalformedValues`, `ppp/yang/schema_test.go`; promoted CLI range tests |
| 14 | `pppSession.run`, `handleRestartTimeout` | `TestLCPRestartCounterBoundsConfigureRequests`, `ppp/rfc1661_lcp_restart_counter_test.go`; no `defaultNegoTimeout` producer remains |
| 16 | `pppSession.updateLCPConvergence` | `TestConfiguredLCPConvergenceBound`, `TestConfiguredLCPLongSilentBudget`, configured restart test file; `TestLCPAuthenticationRestartReceivesFreshConvergenceBound`, `ppp/lcp_convergence_lifetime_test.go` |
| 17 | `parsePPPoEClientTiming`, `Dial`, `clientIPv6CP.sendRequest`, `keepaliveLoop` | `pppoeclient/ipv6cp_timing_test.go`; `iface/pppoe_client_ipv6cp_timing_test.go`; `TestPPPoEClientIPv6CPConfiguredTimerEndToEnd`, `iface/pppoe_ipv6cp_timing_integration_linux_test.go` |
| 18 | `clientIPv6CP.receiveNak` | `TestIPv6CPLateNakRenewsTheSpentBudget`, `TestIPv6CPNakInAckReceivedKeepsTheCounter`, `pppoeclient/ipv6cp_restart_counter_test.go` |
| 19 | Shared `ppp.LCPRejectNakOptions` | `TestLCPMaxFailureRejectsOnlyTheNakInstance`, `TestLCPMaxFailureStillAcknowledgesAcceptableOptions`, `ppp/lcp_max_failure_instances_test.go` |
| 20 | Client schema and `parsePPPoEClientTiming` → `Dial` → `negotiateLCP` | `iface/pppoe_client_lcp_max_failure_test.go`; `pppoeclient/rfc1661_max_failure_test.go`, `lcp_max_failure_options_test.go`; promoted client CLI tests |
| 21 | `pppconfig.ParseLCP`, `lcpLeafError`, `lcpSchemaError` | `TestLCPProviderRefusalNamesAWorkingRepair`, `TestLCPMissingSchemaDoesNotSuggestEditingASetting`, `pppconfig/lcp_errors_test.go` |
| 22 | `pppconfig.lcpUnknownSetting` | `TestLCPProviderRejectsUnknownSetting`, `TestLCPProviderRejectsUnrelatedSetting`, `TestLCPProviderAcceptsKnownSettings`, `pppconfig/lcp_unknown_test.go` |

Wiring tests also remain required: both transport parser tests, both
StartSession tests, `TestPPPManagerAppliesLCPRestartConfig` and
`TestPPPManagerRefusesMissingLCPConfig`. They cover startup, admission and
reload snapshots through the existing paths.

### Final execution state, 2026-11-15

| Scope | Observed result | Limit or remaining action |
|-------|-----------------|------------|
| Eight affected package populations | All eight passed with race instrumentation in `job-unit-pkg-447d2696.log`, after the proxy and lint fixes. Full PPP passed again after the ZRC assertion strengthening in `job-unit-pkg-cce92ca1.log` | None. No production change followed |
| Accepted-proxy restart | New real-Driver three/five-request and answering-peer tests produced three red assertions before `run` initialized Max-Configure. Targeted and full PPP race runs passed. Both native proxy records passed | None. The reused targeted log no longer retains the earlier red |
| Duplicate Reject, late IPv6CP Nak, provider errors and unknown keys | Targeted reds/greens and the seven-package regression passed. Current affected native observations passed | None; this does not claim another unrun package pass |
| Server Linux configuration path | All four named L2TP/PPPoE parser/admission tests passed on Ze 7.3 in `job-lcp-server-kernel-and-client-cut-da6c467e.log`. Native QEMU population/tag checks passed in `job-unit-pkg-bb2bc6d2.log` | None |
| Client wire behavior | Strong LCP initial/restarted 350..900 ms bounds and success control passed. A Dial-only 100 ms cut fails both budget cases, then all three restored cases pass. Strong IPv6CP normal/100 ms failure/3 s failure/restored normal passed on Ze 7.3. Max-Failure normal, forced-five failure and restored normal passed | None; the closure reader checked the final helper and actual kernel logs |
| Termination assertions | Wrong-Code implementations survived earlier frame-count assertions. Max-Terminate now checks protocol/Code. ZRC's sole response now checks protocol/Code/Identifier. `job-lcp-ack-strong-proof-98c1c7db.log` observes both ZRC branches fail for Code 2 after both pass normally. Both native ZRC records are refreshed | None |
| Five CLI tests | All five normal runs and five sets of 80 loaded iterations passed in `job-lcp-five-cli-loaded-ac663beb.log`. Three refusal cuts and two acceptance cuts failed at their named assertions. All five promoted live tests passed restored normal in `job-lcp-five-cli-live-restored-1561c140.log` | None |
| Real pppd interop | Normal scenario passed, rebuilt three-second cut failed at the expected count, and restored normal passed in `job-lcp-pppd-current-cut-473cc355.log` and `job-lcp-pppd-restored-3604fa8e.log` | None; final review preserved the answering-peer, relay, CDN and cleanup assertions |
| RFC evidence | All 69 native commands passed: fourteen new meaningful-mutant keys and 55 existing-record retakes. All clean named units pass. Forty-one broken contrasts show named FAIL; 28 existing revert contrasts terminate at their expected producer-disabled panic. All 47 shifted audits were resealed; 18 changed audits were independently rejudged | Final check has 172 unrelated diagnostics and none for the seven owned stems. Three honest weak judgments retain separately owned pre-existing proof gaps. The 28 revert retakes add no claim of new semantic mutant coverage |
| Other scoped checks | YANG glue passed 168 directories. Config coercion, final `doc check all` and eight-document STE passed. Introduced lint findings were fixed. Full scoped lint has 55 unique findings in unchanged code or copies already larger than the threshold at HEAD. Post-review PPP/iface lint reports 16 existing findings per host/Linux profile | Only the two closure commits and claim release remain |

The main thread owns these observed runs. A prior pass does not prove later
source edits. The final native receipts above cover the frozen source. The
configuration corpus reports 31 unrelated
host/fixture findings, source-classified against unchanged HEAD producers and
configs. This is not a separate HEAD execution or a passing whole-tree check.

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `restart-timer` (milliseconds) | 100-60000 | 60000 | 99 | 60001 |
| `max-terminate` | 1-255 | 255 | 0 | 256 |
| `max-configure` | 1-255 | 255 | 0 | 256 |
| `max-failure` | 1-255 | 255 | 0 | 256 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `l2tp-lcp-restart` | `test/parse/l2tp-lcp-restart.ci` | An operator writes the server container and validates it | Normal, 80 loaded runs, acceptance cut and restored live proof passed |
| `l2tp-lcp-restart-range` | `test/parse/l2tp-lcp-restart-range.ci` | Out-of-range values are refused with the accepted range | Normal, 80 loaded runs, refusal cut and restored live proof passed |
| `pppoe-lcp-restart` | `test/parse/pppoe-lcp-restart.ci` | The same server container under `pppoe` is accepted | Normal, 80 loaded runs, acceptance cut and restored live proof passed |
| `pppoe-client-ipv6cp-timing` | `test/parse/pppoe-client-ipv6cp-timing.ci` | Client IPv6CP timer bounds | Normal, 80 loaded runs, refusal cut and restored live proof passed |
| `pppoe-client-lcp-max-failure` | `test/parse/pppoe-client-lcp-max-failure.ci` | Client Max-Failure default and bounds | Normal, 80 loaded runs, refusal cut and restored live proof passed |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `lcp-restart-timer-pppd` | `test/interop-l2tp/scenarios/lcp-restart-timer-pppd/` | xl2tpd and pppd 2.5.5 | After a successful session and ping, a relay injects one legal restart and drops actual pppd LCP replies. Exactly three requests, 350..900 ms gaps and final wait, matching CDN and kernel cleanup are required | Normal and restored scenarios passed. Rebuilt fixed-three-second cut failed the checker as required |

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

| Area | Files and change |
|------|------------------|
| Server runtime | `internal/component/l2tp/ppp/session_run.go`, `internal/component/l2tp/ppp/session.go`: configured timer/counters, Max-Failure, convergence lifetime. Preserve existing TLD and NCP cleanup |
| PPP boundary | `internal/component/l2tp/ppp/start_session.go`, `internal/component/l2tp/ppp/manager.go`: validate and carry one admitted snapshot |
| L2TP transport | `internal/component/l2tp/config.go`, `internal/component/l2tp/subsystem_reload.go`, `internal/component/l2tp/reactor.go`, `internal/component/l2tp/reactor_setters.go`, `internal/component/l2tp/reactor_kernel.go`: parse and retain the admission snapshot |
| PPPoE transport | `internal/component/l2tp/pppoe/config.go`, `internal/component/l2tp/pppoe/server.go`, `internal/component/l2tp/pppoe/subsystem_reload.go`: the same snapshot path |
| Server schema | `internal/component/l2tp/yang/ze-l2tp-conf.yang`, `internal/component/l2tp/pppoe/yang/ze-pppoe-conf.yang`: use the shared grouping |
| Client configuration | `internal/component/iface/yang/ze-iface-conf.yang`, `internal/component/iface/pppoe_client_timing.go`: applicable LCP limits and independent IPv6CP timing |
| Client negotiation | `internal/component/l2tp/pppoeclient/dialer.go`, `internal/component/l2tp/pppoeclient/session.go`, `internal/component/l2tp/pppoeclient/ipv6cp_client.go`, `internal/component/l2tp/pppoeclient/network_phase.go`, `internal/component/l2tp/pppoeclient/connected_session.go`: retained settings and timer ownership |
| Existing fixtures | Explicit validated snapshots on all affected direct callers. RFC-tagged migrations require the independent approval and refreshed evidence |
| Interop | `internal/le/interoplab/l2tp/l2tp.go`, `test/interop-l2tp/Dockerfile.lac`: scenario discovery and pinned real peer, without new runtime grants |
| Linux test placement | `internal/component/l2tp/lcp_config_test.go`, `internal/component/l2tp/pppoe/lcp_timing_linux_test.go`: `integration && linux` headers for syscall fixtures, no changed bodies. `internal/le/test/qemu/alltests.go`: add the PPPoE package to `integrationPackages` |
| RFC declaration | `rfc/short/rfc1661.md`: five implemented rows, honest role capability notes and tagged bindings through the native index |
| Documentation | The Documentation Update Checklist names each affected durable page |

The test map above names each new behavior's actual carrier.

## Files to Create

| Area | Files |
|------|-------|
| Shared server authority | `internal/component/l2tp/ppp/yang/ze-ppp-lcp.yang`, its registration/glue and `schema_test.go`; `internal/component/l2tp/ppp/lcp_config.go`; `internal/component/l2tp/pppconfig/lcp.go` |
| Server proof | `ppp/rfc1661_configured_restart_test.go`, `lcp_config_test.go`, `lcp_config_manager_test.go`, `lcp_convergence_lifetime_test.go`, `lcp_max_failure_instances_test.go`; `internal/component/l2tp/ppp/rfc1661_proxy_restart_budget_test.go`: accepted-proxy silent and answering real-Driver tests; `pppconfig/lcp_errors_test.go`, `lcp_unknown_test.go`; both transport LCP configuration tests |
| Client proof | `pppoeclient/ipv6cp_timing_test.go`, `ipv6cp_restart_counter_test.go`, `rfc1661_max_failure_test.go`, `lcp_max_failure_options_test.go`; corresponding iface config/reload and Linux wire tests |
| CLI | Five tests named above, promoted byte-identically to `test/parse/` after their required proof |
| Real-peer proof | `test/interop-l2tp/scenarios/lcp-restart-timer-pppd/`; `internal/le/interoplab/l2tp/lcp_restart.go`, its tests, `avprelay/lcprestart.go` and tests |
| Operator news | `website/changes/fragments/lcp-restart-counters.md` |

There is no production Go bootstrap-default file or pairing test. AC-15 is
covered by the actual shared-schema consumer test.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/l2tp/ppp/yang/ze-ppp-lcp.yang`, used by `ze-l2tp-conf.yang` and `ze-pppoe-conf.yang` |
| YANG validation constraints | Yes | `range "100..60000"` on the timer, `range "1..255"` on each counter, `uint16` and `uint8` types |
| YANG custom validators | No | Native `range` is sufficient; no cross-leaf constraint exists |
| CLI commands/flags | N-A | Config-only. No command is added; `ze config show` renders the container from the schema |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | Yes | Automatic for typed leaves; no `CompleteFn` needed |
| Functional test for configuration | Yes | The five promoted CLI tests above and committed client LCP timing test |
| Pipe completeness | N-A | No new command output |
| Env var registration | N-A | The leaves sit under `l2tp` and `pppoe`, not under `environment/` |
| Doctor check for runtime dependencies | N-A | No new file path, socket, service, kernel module, port or certificate |
| Prometheus counters/metrics | No | The Restart counter is per-session negotiation state rather than an operator-facing aggregate; `internal/component/l2tp/metrics.go` gains nothing |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes, evidence | `features/l2tpv2-bng.md` adds the passing `l2tp/lcp-restart-timer-pppd` scenario. Both feature scopes and experimental levels remain unchanged. Do not hand-edit generated `docs/features.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/guide/l2tp.md`, `docs/guide/pppoe.md` |
| 3 | CLI command added/changed? | No | No command added |
| 4 | API/RPC added/changed? | No | No RPC added |
| 5 | Plugin added/changed? | No | L2TP and PPPoE are components, not plugins |
| 6 | Has a user guide page? | Yes | Both guides describe the server container, defaults and admission-only reload. The PPPoE client section describes Max-Failure and independent IPv6CP timing |
| 7 | Wire format changed? | No | Retransmission timing changes; no packet layout does |
| 8 | Plugin SDK/protocol changed? | No | `ppp.StartSession` is internal to `internal/component/l2tp/` |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc1661.md` and native records/audits. Derived rendering with `./le rfc update` is optional, never a validation prerequisite or a committed ledger |
| 10 | Test infrastructure changed? | Yes | `docs/labs/l2tp-interop.md`, `docs/functional-tests.md` and `docs/architecture/testing/interop.md` describe `checkLCPRestart`, relay observations, answering-peer control and cleanup assertions, without claiming an unrun result |
| 11 | Affects daemon comparison? | No | Current `docs/comparison.md` has no PPP LCP timer comparison claim to change |
| 12 | Internal architecture changed? | Yes | the PPP FSM description in `docs/research/l2tpv2-implementation-guide.md`, which `ppp_fsm.go` declares in its `// Design:` header |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | No | None added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | `EventLCPDown` already existed. The shared YANG module uses existing registration and glue |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/research/l2tpv2-ze-integration.md` anchors `session_run.go` actions, `NewLCPConfig`, `pppconfig.ParseLCP` and `StartSession.LCP`. The implementation guide and both PPP architecture pages anchor the changed producers. The configuration guide anchors both schemas. Source claims were reread. Final `doc check all` passed after the closure edits |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/l2tp.md` shows the `authentication` and `ncp` containers; the `lcp` example goes beside them |

## What This Spec Owes `rfc/short/rfc1661.md`

`rfc/short/rfc1661.md` declares `Enrolment: enrolled` and names the implemented
PPP roles. `rfc/extraction/rfc1661.json` exists and records sign-off on
2026-09-21. The old unsigned-stem and `rfc/enrolled.txt` instructions are stale.
This spec changes no extraction scope. It owes proof of its five requirements
and refreshes only the existing records or audits that its source changes stale.

The current summary declarations and remaining evidence:

| Row | Change |
|-----|--------|
| `RFC1661-4.6-1` | Plain implemented row. Prove configured server/client timers, including IPv6CP |
| `RFC1661-4.6-2` | Plain implemented row. Prove server termination limits and matching-Ack contrast. The client emits no local LCP Terminate-Request |
| `RFC1661-4.6-3` | Plain implemented row. Prove configured server and client request limits and answering-peer controls |
| `RFC1661-4.6-4` | Plain implemented row. Prove both roles' thresholds and sent-Ack renewal |
| `RFC1661-4.4-2` | Plain implemented row. Prove configured ZRC grace and the STA-only contrast |
| Derived requirement index | Native generation binds tagged tests. Never hand-edit generated coverage |
| `Enrolment reason` in `rfc/short/rfc1661.md` | Current role/capability statement replaces stale missing-counter wording |

`rfc/requirements/rfc1661.md`, `ai/RFC-REQUIREMENTS.md` and the
`docs/features/rfc-status.md` RFC 1661 row are generated, ignored outputs.
`./le rfc update` can render them for reading, but it is not a validation
prerequisite. Never hand-edit or commit these derived ledgers.

Each of the five requirements owes a discrimination record in the same change,
written by `./le rfc record` and stored under `rfc/discrimination/`.
`./le rfc check` refuses a tagged unit that the commit under test added against
`HEAD^` and that carries none.

## Implementation Steps

The original eight-phase plan has been reconciled with the existing counter and
TLD implementation. All eight work packages remain represented below.

| Phase | Implementation | Verify in the main thread |
|-------|----------------|---------------------------|
| 1/8 Wiring and sole default authority | Shared YANG grouping → `pppconfig.ParseLCP` → both transports → `StartSession.LCP` → driver. No production Go bootstrap defaults | Both parser/admission tests, manager acceptance/refusal and `TestPPPLCPGroupingDefaultsResolveInBothModules` |
| 2/8 Configured Restart timing | Existing SCR/STR/ZRC actions arm the admitted one-shot interval | `TestConfiguredLCPRestartTimer`, `TestConfiguredLCPZeroRestartPeriod`; timer stops on Opened |
| 3/8 Configured request counters | Existing IRC and timeout logic use configured Max-Configure or Max-Terminate. Accepted proxy Opened retains Max-Configure for a later RCR without arming its timer | Configured counter/termination tests and both new proxy tests. Exact frame Code, protocol, budget and answering-peer control |
| 4/8 Negotiating lifetime | Delete fixed `defaultNegoTimeout`. Silence uses the counter. Responsive LCP loops use the derived deadline owned by actual edges | Default counter regression, configured long silent budget, Nak loop and nested authentication restart tests |
| 5/8 Max-Failure in both roles | Count successfully sent Naks, reset on sent Ack, convert only original refused instances after the threshold | Server/client threshold, duplicate/malformed-instance, Ack renewal and suppressed-write tests |
| 6/8 Existing TLD preservation | Retain cleanup acknowledgment before accompanying responses and restart authentication/NCP on the same transport | `TestLCPDownPrecedesRestartWireResponse`, existing lifetime tests and Echo contrast |
| 7/8 Independent client IPv6CP timing | Extend retained timing carrier. One IPv6CP timer crosses IPCP/connected ownership. Correct IRC renewal on late Nak | Config/reload tests, timer lifecycle tests, late-Nak contrast and strong Linux receipt interval |
| 8/8 Operator and protocol proof | Five promoted CLI tests, real pppd scenario, current RFC records/audits, accurate docs/news | Required loaded CLI runs and promotion, rebuilt interop discrimination, native records/checks, final scoped regression and independent closure review |

Implementation review found and corrected three product defects: duplicate
Reject instance selection, IPv6CP RCN counter renewal, and unstructured provider
errors. It also found two weak assertions: a too-fast IPv6CP timer and an
incorrect Terminate Code could survive. The test map and execution state above
retain both findings and the exact remaining proof.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC has an implementation at file and symbol, and each of the five requirement ids has both polarities bound |
| Feature completeness | Both transports reach the leaves. An L2TP-only implementation leaves PPPoE subscribers on compiled-in constants, and the MUST unmet there |
| Correctness | The timer is armed only by `scr`, `str` and `zrc`, and stopped on leaving a timer-running state, per Section 4.1. `irc` picks Max-Terminate on a `str` edge and Max-Configure on a `scr` edge |
| Correctness | The counter decrements on EVERY transmission including the first, per Section 4.4, so `max-configure 3` yields 3 requests rather than 4 |
| Naming | The YANG leaf, the Go field, the `Parameters` field and the `StartSession` field are derivable from each other, and leaf names carry no abbreviation |
| Data flow | Each negotiator owns its counter and timer on its existing session goroutine. `LCPDoTransition` stays pure. IPv6CP timer ownership survives the IPCP-to-connected handoff |
| Rule: `ai/rules/no-layering.md` | `defaultNegoTimeout` is DELETED, not left standing beside Max-Configure |
| Rule: `ai/rules/principles.md` | No zero is a valid-looking answer: a zero Max-Configure is refused by the schema, and a session that cannot read its configured period says so rather than falling back silently |
| Convergence lifetime | `updateLCPConvergence` sees nested Opened/restart transitions, stops in non-negotiating states and grants a fresh bound only to a new lifetime |
| Error handling | Direct provider values and unknown keys cannot silently become defaults. Repairs name the actual transport and valid schema value |
| Assertion strength | Timer proof rejects both slower and faster constants. Termination proof decodes protocol and Code as well as count/state |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Four server defaults declared once | Shared grouping has four defaults. `TestPPPLCPGroupingDefaultsResolveInBothModules` compares both consumer schemas and runtime results. No production bootstrap default duplicates them. Independent CHAP and proxy PAP timing remains valid |
| Both transports carry the container | `./le config coercion check`, both parser/admission tests and three server CLI tests |
| Fixed initial bound gone | `session_run.go` has no `defaultNegoTimeout` or `negoTimer`. Silent and responsive cases are proven separately |
| Five requirements proven in both polarities | `./le rfc check`, and `rfc/requirements/rfc1661.md` shows a test in both columns for each id |
| Discrimination records written | Native `./le rfc record` for every added or changed tagged unit, plus refresh of affected existing records and audit verdicts |
| Interop scenario discovered and run | `ZE_L2TP_INTEROP_SCENARIO=lcp-restart-timer-pppd ./le --name session test deployment docker-l2tp-ppp`, including rebuilt discrimination |
| Documentation carries the container | grep `docs/guide/l2tp.md` and `docs/guide/pppoe.md` for the `lcp` block |
| Client applicable settings reach Dial | Current config/reload and Linux tests cover LCP timing/limits and independent IPv6CP timing |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Schema ranges and constructor validation reject zero, overflow, fractional milliseconds and invalid direct provider values. Unknown keys are refused |
| Resource exhaustion | The 100 ms timer floor bounds the fastest configured retry. New convergence and IPv6CP timers stop on their owning lifecycle exits and session cleanup |
| Responsive peer | A valid LCP Nak can renew IRC. The derived convergence deadline must end that lifetime without truncating longer configured silence budgets |
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

- The RFC supplies the arming rule. Existing SCR, STR and ZRC timer ownership is preserved while the configured interval replaces fixed timing.
- `irc` needs no per-state table. The transition already carries the accompanying action: `str` beside `irc` means Max-Terminate, and `scr` beside it means Max-Configure. A table would be a second declaration of `LCPDoTransition`.
- The RFC's SHOULD defaults retain a 30-second silent-peer budget. Valid replies can reset IRC, so the separate convergence deadline derives from settings rather than duplicating a fixed 30-second limit.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `rfc-extension-not-implemented.md` | 2026-11-07 | Fixed: separate client LCP and IPv6CP Restart leaves reach Dial; both Linux timing contrasts reject fixed intervals (goal 4) |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One YANG grouping in a new `ze-ppp-lcp` module, used by both conf modules | Four leaves declared twice, once per transport; or the leaves in `ze-types.yang` | Twice is two declarations of the RFC's SHOULD values, and they will disagree. `ze-types` is a central module, and a PPP spelling there is what `ai/rules/plugins.md` forbids. The `ppp` package owns PPP, and both transports already import it in Go |
| Milliseconds for the Restart timer | Seconds, matching `hello-interval` and `authentication/timeout` | Section 4.6's Implementation Note asks for faster retransmission on low-latency links, and a seconds-only leaf cannot express it. The name is spelled out in full per `ai/rules/config.md` |
| Replace fixed initial timeout with separate silence/convergence bounds | Keep fixed 30 seconds, or remove every lifetime bound | Fixed timing truncates configured budgets. IRC can renew indefinitely, so responsive nonconvergence needs the derived higher-level bound |
| `tld` in scope, `tls` out | Implement all four `tl*` actions; or defer `tld` to a separate spec | `tld` shares a transition with `zrc` and terminates the counter's path. `tls` fires only on entering Starting, which nothing in this spec reaches |
| A new named interop scenario rather than extending `01-ppp-ipv4` | Add assertions to the existing scenario | `01-ppp-ipv4` drives the path where no retransmission happens, so nothing in it can go red on this change. The retransmission path needs a silenced peer, which changes the scenario's whole shape |

## Known Limitations
- Restart timer backoff is not implemented. `RFC1661-4.4-1` remains a gap in proof in the current summary: the configured timer uses a constant period and the conditional backoff case never occurs, but no test proves that absence. This spec adds no backoff scope.
- The four existing L2TP interop scenario directories keep their banned numeric prefixes. Renaming them touches four directories, four constants in `internal/le/interoplab/l2tp/l2tp.go`, and every citation of those names. It is its own work and is not folded into a conformance change.
- Client IPv6CP's ten-request limit bounds silence. Valid Naks can renew that budget after IPCP opens. No claim here says that every responsive IPv6CP loop is bounded.

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
- [ ] AC-1..AC-22 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Main-thread scoped package, Linux, functional, interop and RFC proof runs pass for the final source in this checkout. No agent runs `./le verify` or develops/tests in a worktree or exported copy
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
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Lessons, if any, routed to their owning durable page instead of a mandatory closure summary
- [ ] **Commit A:** remaining code + tests + docs + spec and required evidence, with all citations repointed
- [ ] **Commit B:** spec removal through `./le commit create ... remove <spec>` (commit A preserves its record)

---

## Implementation Summary

### What Was Implemented

- Both server transports read one shared YANG grouping through `pppconfig.ParseLCP` and admit immutable `LCPConfig` values.
- The existing LCP actions use the admitted timer, Max-Terminate and Max-Configure. Accepted proxy LCP retains a full request budget.
- Server and client Max-Failure count transmitted Naks, renew on a transmitted Ack and preserve the original refused option bytes.
- Responsive LCP loops have a derived convergence deadline. Each new negotiating lifetime receives a fresh bound.
- Client IPv6CP retains its configured one-shot timer across IPCP and connected ownership. IPCP keeps its independent timing.
- Five promoted CLI tests, real pppd loss injection, Linux wire tests and native RFC discrimination cover the operator paths.

### Bugs Found/Fixed

| Defect | Producing function | Regression |
|--------|--------------------|------------|
| A valid Nak loop can renew the Restart counter forever | `session_run.go` `updateLCPConvergence` | Configured bound, long silence and nested authentication restart tests |
| A Type-only Reject conversion selected a permitted earlier duplicate | `session_run.go` `LCPRejectNakOptions` | Original-instance server/client byte assertions |
| IPv6CP RCN did not renew a spent budget in Req-Sent/Ack-Sent | `ipv6cp_client.go` `receiveNak` | Nineteen observed requests and Ack-Rcvd contrast |
| Provider refusals lacked a transport-specific repair, and unknown keys became defaults | `pppconfig/lcp.go` `ParseLCP`, `lcpUnknownSetting` | Structured refusal, unknown-name and useful-output controls |
| Accepted proxy LCP entered Opened with a zero request budget | `session_run.go` `run` | Real Driver silent budgets of three/five and answering-second-request control |
| Termination frame counts accepted the wrong Code | `rfc1661_configured_restart_test.go` configured termination tests | Wrong-Code survivors now fail protocol/Code assertions. ZRC also checks Identifier |
| Client LCP's wire deadline accepted a fast Dial timer | `iface/pppoe_lcp_timing_integration_linux_test.go` `checkConfiguredLCPBudget` | Forced 100 ms passes the old initial/restart tests and fails both stronger receipt bounds |

### Documentation Updates

Both operator guides, the configuration reference, both PPP architecture pages,
the two research pages, interop/functional-test documentation and the news
fragment describe the changed behavior with source anchors. The L2TP feature
declaration names the new real-peer scenario. The NCP lifetime spec cites the
durable guide instead of this removable spec.

The final `doc check all` passed in `job-lcp-final-doc-check-49d1a341.log`,
after refreshing the ignored discovery index. The final STE check reports no
habit grew in eight changed documents.

### Deviations from Plan

A-2 was broken: the Restart counter bounds silence, not a responsive Nak loop.
The fixed initial timeout was replaced with a configured higher-level bound.
Defaults come directly from the shared grouping rather than Go bootstrap
constants. CHAP and proxy PAP retain separate authentication timing.

The planned `connected_session.go` edit was unnecessary: its existing retained
`sessionConfig` already carries the new client limit. The pinned pppd image
already uses 2.5.5, so `Dockerfile.lac` needed no change. The relay image adds
the new source file. No acceptance criterion was removed.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-2 treated the counter as a bound on every negotiation | RCN can renew IRC indefinitely | Responsive Nak-loop regression | Derived convergence lifetime at actual FSM edges |
| approach | Receipt deadlines accepted incorrectly fast IPv6CP and LCP timers | Both lower and upper receipt bounds are needed | Forced 100 ms implementations survived earlier assertions | Strong receipt bounds reject 100 ms on both protocols. IPv6CP also rejects 3 s. Restored tests pass |
| approach | Frame counts accepted the wrong termination Code | A count does not identify a protocol response | Max-Terminate and ZRC wrong-Code survivor runs | Assert protocol/Code, plus ZRC Identifier, and observe native redwalks |
| approach | A no-op reload fixture omitted newly required LCP configuration | Production now supplies a schema-derived snapshot | `TestReloadNoOpOnIdentical` failed | Independent judge approved schema-derived fixture input with all assertions preserved |
| approach | Accepted proxy startup skipped the counter left by completed LCP | Opened RCR carries SCR without IRC | New real-Driver tests stopped after one request | Initialize Max-Configure at accepted proxy startup only |
| approach | Added documentation had unclear wording and long paragraphs | Claims needed shorter factual paragraphs | Scoped STE and closure source review | Preserve source anchors, clarify defaults and repoint closing-spec citers |
| approach | Commit A carried a ready sibling spec with repaired citations but no Existing Mechanisms section | The native gate judges every edited carried spec, including citation-only companions | Commit preparation refused the NCP lifetime spec before printing a script | Add a current source-aware mechanism census, preserve its status/scope/ACs and inspect all three carried specs against the native predicate. This metadata NOTE adds no product-review round |
| approach | The first final RFC pass refreshed discrimination but omitted audits affected by fixture migrations | Native check found 47 byte-identical SHIFTED audits and 18 function-scoped STALE audits across the owned PPP stems | All 69 observations passed, then the final freshness check named 65 audit diagnostics | Main resealed all 47 identical units and stamped independent rejudgments of the 18 changed units. Final check names no owned stem |
| approach | The inherited RFC1332 MTU unit's two polarities were credited as full enforcement | Its single fake-backend MTU observation neither separates those polarities nor observes Linux fragmentation | Independent source-aware rejudgment of the fixture migration | Record both verdicts honestly as weak. Existing spec-rfc-weak-verdict-proof-gaps owns AC-3/AC-4. Preserve every tag, assertion and record; no production defect or LCP scope reduction is claimed |
| approach | The inherited Magic-Number audit credited the server's deterministic negative to the separate client implementation | The client's random-draw unit rarely exercises its repeated-draw guard | Independent RFC1661 rejudgment of the Max-Failure fixture migration | Record RFC1661-6.4-7 as weak. Existing spec-rfc-weak-verdict-proof-gaps AC-5 owns deterministic client guard proof. Preserve current wire assertions/records; no runtime defect or counter-scope reduction is claimed |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Goal 1: four configured server settings | Done | Shared `ze-ppp-lcp.yang`, `pppconfig.ParseLCP`, both transport admissions | Linux wiring, schema and CLI proof |
| Goal 2: preserve configured IRC/ZRC/TO semantics | Done | `pppSession.applyTransition`, `spendRestartCount`, `handleRestartTimeout` | Silent, answering, late-RCN and termination contrasts |
| Goal 3: preserve TLD before responses | Done | `pppSession.performAction`, `resetNCP` | `TestLCPDownPrecedesRestartWireResponse`, including Echo contrast |
| Goal 4: applicable client limits and IPv6CP timing | Done | Timing parser, `Dial`, `negotiateLCP`, `clientIPv6CP` | Config/reload, Linux wire and timer-handoff tests |
| RFC1661-4.6-1 through RFC1661-4.6-4 and RFC1661-4.4-2 | Done | Native tagged units, discrimination and audits | Current receipts and enforced audits, no owned-stem diagnostic in final check |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestConfiguredLCPRestartTimer`; `lcp-restart-timer-pppd` | Configured 500 ms wire gaps |
| AC-2 | Done | `TestPPPLCPGroupingDefaultsResolveInBothModules` | 3000 ms / 2 / 10 / 5 |
| AC-3 | Done | `TestConfiguredLCPMaxConfigure`; both proxy-budget tests | Initial and accepted-proxy budgets |
| AC-4 | Done | Configured and proxy answering-second-request controls | Exactly two requests, then Opened |
| AC-5 | Done | `TestConfiguredLCPMaxTerminate` | Exact two/four LCP Terminate-Requests |
| AC-6 | Done | Same test's matching-Ack control | One Terminate-Request, then Closed |
| AC-7 | Done | `TestConfiguredLCPMaxFailure` | Sixth answer Rejects original MRU |
| AC-8 | Done | Same test's through-threshold assertions | First five remain Naks |
| AC-9 | Done | `TestConfiguredLCPZeroRestartPeriod` | Zero count, configured grace, Stopped |
| AC-10 | Done | `TestLCPReplyRenewsConfiguredCounter` | Req-Sent/Ack-Sent renew, Ack-Rcvd does not |
| AC-11 | Done | `TestLCPDownPrecedesRestartWireResponse` | Cleanup observed at both SCR and SCA/SCN |
| AC-12 | Done | Same test's Echo control | One Echo-Reply, no LCP-down |
| AC-13 | Done | Constructor/provider bounds and `l2tp-lcp-restart-range.ci` | Both adjacent invalid boundaries |
| AC-14 | Done | `TestLCPRestartCounterBoundsConfigureRequests` | Ten default requests, counter-driven 30 seconds |
| AC-15 | Done | Shared-grouping consumer/default test | No production bootstrap defaults |
| AC-16 | Done | Configured convergence, long-silence and nested-auth tests | Native bounds discriminate fixed/old lifetime cuts |
| AC-17 | Done | Client IPv6CP configuration/reload and Linux interval tests | Independent timer survives ownership handoff |
| AC-18 | Done | `TestIPv6CPLateNakRenewsTheSpentBudget`, Ack-Rcvd contrast | Nine spent plus ten fresh wire requests |
| AC-19 | Done | `TestLCPMaxFailureRejectsOnlyTheNakInstance` | Duplicate and malformed original bytes |
| AC-20 | Done | Client Max-Failure config, tagged threshold/renewal and Linux tests | Configured two Naks, then original-byte Reject |
| AC-21 | Done | `TestLCPProviderRefusalNamesAWorkingRepair` | Both roots and final integer repair line |
| AC-22 | Done | Unknown, unrelated and known-setting provider tests | Misspellings refuse, valid settings work |

### Tests from TDD Plan

| Test population | Status | Location | Notes |
|-----------------|--------|----------|-------|
| Configured Restart, Configure, Terminate, Max-Failure and ZRC | Done | `ppp/rfc1661_configured_restart_test.go` | Complete PPP race pass, plus stronger ZRC native walk |
| IRC, cleanup ordering and Echo contrast | Done | Same carrier | Existing behavior retained |
| Convergence, long silent budget and nested restart | Done | Configured carrier, `ppp/lcp_convergence_lifetime_test.go` | Both convergence native cuts fail |
| Accepted proxy silent/answering budgets | Done | `ppp/rfc1661_proxy_restart_budget_test.go` | Actual Driver, pipe and timer |
| Bounds, default authority and manager refusal | Done | `ppp/lcp_config*_test.go`, `ppp/yang/schema_test.go` | Schema package passed in the earlier mixed run |
| Provider repair and unknown setting controls | Done | `pppconfig/lcp_errors_test.go`, `lcp_unknown_test.go` | Race pass |
| L2TP and PPPoE parser/admission | Done | Transport `lcp*_test.go` files | Four named Linux tests pass on Ze 7.3 |
| IPv6CP timer, handoff, stop and counter renewal | Done | Client `ipv6cp_timing_test.go`, `ipv6cp_restart_counter_test.go` | Actual frames and virtual deadlines |
| Client Max-Failure threshold, renewal and option bytes | Done | Client `rfc1661_max_failure_test.go`, `lcp_max_failure_options_test.go` | Includes failed/suppressed reply contrast |
| Client schema and next-Dial snapshots | Done | Iface LCP/IPv6CP timing and Max-Failure tests | Same running session, exactly next Dial changed |
| Client configured wire tests | Done | Iface integration carriers | LCP and IPv6CP 100 ms cuts fail. IPv6CP 3 s and fixed-Max-Failure cuts fail. Restored cases pass |
| Five registered CLI tests | Done | `test/parse/` named Functional Tests | Each normal, 80 loaded, meaningful cut and restored live pass |
| Real pppd retransmission/exhaustion | Done | `lcp-restart-timer-pppd` scenario | Healthy ping, actual dropped reply, exact wire timing, CDN and cleanup |
| Native integration package population | Done | `internal/le/test/qemu/alltests.go` | Exact PPPoE package included; population/tag tests pass |

### Files from Plan

| File population | Status | Notes |
|-----------------|--------|-------|
| Server runtime and PPP boundary | Done | `session_run.go`, `session.go`, `start_session.go`, `manager.go`; CHAP/PAP timing separated |
| Shared authority | Done | New `ppp/lcp_config.go`, `ppp/yang/` and `pppconfig/` |
| Both transports and consumer schemas | Done | Config, reactor/admission and reload paths carry validated values |
| Client configuration and negotiation | Done | Iface schema/timing, Dial, LCP, IPv6CP and network-phase ownership |
| `connected_session.go` and `Dockerfile.lac` | Changed | Existing retained snapshot and pinned 2.5.5 image already meet the plan |
| Existing fixtures and all new proof carriers | Done | Historical defaults explicit, judged RFC migrations, Linux placement |
| Real-peer checker and relay | Done | New checker/relay sources, registry, relay Dockerfile and four scenario inputs |
| Five CLI tests | Done | Promoted byte-identically to `test/parse/` |
| RFC declaration and evidence | Done | Canonical summary and native metadata only, no derived ledger. All affected receipts are current; three pre-existing proof gaps retain honest weak verdicts |
| Documentation, feature evidence and news | Done | Source-anchored operator/design pages, passing-scenario declaration and six-heading fragment |

### Audit Summary

- Four task goals and all 22 acceptance criteria are implemented and have scoped passing evidence.
- Fourteen test populations and ten planned file populations are accounted for above.
- No acceptance criterion was skipped or reduced. Two planned edits were unnecessary because existing producers already carry their contract.
- All 69 native observations passed. The 47 shifted audits were resealed without refusal and the 18 changed audits were independently rejudged. Current candidate audits contain 70 enforced and three honest weak verdicts. Final check has zero diagnostics for the seven owned stems.

## Goal Validation (BLOCKING)

All job-log basenames below are under
`tmp/session/2026-11-14-213245/scratch/`. The closing reader inspected the
named source and retained output. These are scoped results, not a whole-tree
release claim.

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| 1: operator-configurable server limits | CLI + Linux + pppd interop | All five CLI tests pass 80 loaded executions each in `job-lcp-five-cli-loaded-ac663beb.log`. The four server parser/admission tests pass on Ze 7.3. `job-lcp-pppd-current-cut-473cc355.log` observes healthy pppd traffic, normal pass and rebuilt fixed-three-second failure. Restored scenario passes |
| 2: configured Restart semantics | Wire + virtual timer + native redwalk | `TestConfiguredLCPMaxConfigure`, `TestConfiguredLCPMaxTerminate` and proxy tests pass in `job-unit-pkg-447d2696.log`. `job-lcp-ack-strong-proof-98c1c7db.log` shows clean ZRC PASS and both wrong-Code branches FAIL |
| 3: upper-layer lifetime ends first | State observed during wire writes | `TestLCPDownPrecedesRestartWireResponse` checks disconnected channel, Initial IPCP and the single acknowledged LCP-down event before both emitted responses. Its Echo control requires Opened, one Echo-Reply and no down event. Complete PPP race population passes |
| 4: applicable client settings | Real registered Dial + wire + reload | Strong initial/restarted LCP receipt bounds reject a Dial-only 100 ms constant in `job-lcp-wire-fast-kernel-91bde825.log`. All three normal and restored LCP tests pass. Strong IPv6CP rejects 100 ms and 3 s, then passes restored. Max-Failure 2 rejects forced 5 at the third reply, then passes restored. Reload changes exactly next Dial |

## Work Not Done

No in-scope item remains unimplemented. Scoped execution, native receipts and
closure checks are complete. The existing
backoff proof gap and separately owned NCP lifetime design remain outside the
four task goals.

The independent audit also identified three pre-existing proof gaps outside
those goals: RFC1332's MTU/fragmentation pair and the client's deterministic
Magic-Number repeated-draw control. Their honest weak verdicts are owned by
AC-3 through AC-5 of `plan/release/validation/spec-rfc-weak-verdict-proof-gaps.md`.
This ownership records no skipped LCP counter work and no production defect.

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None within this spec's four goals | Every AC is implemented and scoped behavior passes | Not applicable |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/lcp-restart-counters-01a1bc50-a7a3-7e81-8312-0922204509cf.md` |
| `./le spec review check` | PASS: 120 code files, clean, hashes match; artifact records the exact 249-path candidate population |
| Rounds | 2 |
| Reviewer lenses used | Wiring/logic, security/lifecycle/edge cases, performance/simplicity/style, RFC text, assertion discrimination and full AC audit |
| Final source finding count | 0 BLOCKER, 0 ISSUE after both assertion fixes. Native CLEAN artifact and hash check recorded |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | ZRC's single-frame assertion accepted a wrong protocol/Code despite claiming Terminate-Ack | `ppp/rfc1661_configured_restart_test.go` `TestConfiguredLCPZeroRestartPeriod` | Independent approval, protocol/Code/Identifier assertion, native clean PASS / wrong-Code FAIL |
| 2 | ISSUE | The client LCP wire test accepted any second request within two seconds, so a Dial-only fixed 100 ms timer passed the configured 500 ms case | `iface/pppoe_lcp_timing_integration_linux_test.go` `checkConfiguredLCPBudget` | Native kernel survivor reproduced. Strong 350..900 ms bounds reject both initial and restarted fast cases. All three normal/restored tests pass |

### Run 2

The second pass covered both assertion fixes and their effects on the claims.
All old ZRC timer, state and count assertions remain. Both Opened and Closed
wrong-Code cases now fail. Client LCP keeps its missing-request deadline,
request count, PADT and healthy-peer control. Its stronger receipt bounds now
reject the fast constant in both initial and restarted negotiation. All three
normal/restored kernel tests pass. No production source changed. The result is
0 BLOCKER and 0 ISSUE. Documentation record corrections do not add a round.

### Security and style result

The new numeric settings are bounded before duration conversion. Private
`LCPConfig` fields preserve construction validity. The driver refuses a missing
snapshot and closes transferred descriptors. No peer input reaches a new panic,
shell evaluation or allocation sized without a frame bound.

The existing session owner drives the new timers. They stop at inactive
states and session end. Reload replaces future admission values under existing
locks and preserves running snapshots. No new worker, queue, runtime dependency,
privilege or external package was added. The style pass covered every changed
Go file: panic reachability, explicit outcomes, validity, input bounds,
goroutine ownership and handled errors. No unresolved source style ISSUE remains.

### Check limits

- Native repository check reports 15 warnings. Four new public getter warnings are false positives: `session_run.go` calls each getter in production in the same package. Eleven other warnings name existing or foreign exports and VPP parsing.
- Scoped lint reports 55 unique findings across host, Linux and FreeBSD. Introduced findings are fixed. Remaining lines are unchanged or were above their copy-size thresholds at HEAD.
- Post-review PPP/iface lint reports 16 existing findings in each host/Linux profile in `job-lcp-post-review-scoped-lint-7225c585.log`, with no new finding. Full post-review PPP race tests pass in `job-unit-pkg-cce92ca1.log`.
- The configuration corpus reports 31 host/fixture findings: 27 backend refusals, two RFC 8212 policy absences and two Linux XFRM refusals on Darwin. Their carriers and diagnostic producers are unchanged against HEAD. No separate HEAD run is claimed.
- The weakening audit reports one unrelated BGP tagged-test change without an approval in this session: `reactor/rfc4271_fsm_error_peer_test.go` `TestRFC4271OpenConfirmUpdateIsAnFSMError`. It reports no owned weakening. The BGP file is excluded from this closure.
- Independent RFC1332 rejudgment found pre-existing proof gaps for `RFC1332-2.1-3` and `RFC1332-2.1-4`. The installed-MTU test has one input/observation for both polarities and observes no actual fragmentation. Both honest weak verdicts are owned by AC-3/AC-4 in `plan/release/validation/spec-rfc-weak-verdict-proof-gaps.md`. No prior test, tag or record was removed, and no production fragmentation defect is asserted.
- `RFC1661-6.4-7` also has an honestly weak client proof: random entropy almost never repeats the previous Magic-Number, and the deterministic server test reaches another implementation. The same proof-gap spec owns AC-5 for deterministic client guard discrimination. The correct producer, current positive wire assertions and reachability records remain unchanged.
- Final RFC check in `job-lcp-final-rfc-check-645b74de.log` reports 172 diagnostics across unrelated stems and none for RFC1332, RFC1334, RFC1661, RFC1994, RFC2516, RFC2661 or RFC5072. Unrelated diagnostics include concurrent work and are not all called pre-existing. No `./le verify` or whole-tree compile result is claimed or owed by this closure.

## Pre-Commit Verification

### Files Exist (ls)

The closing reader ran `ls -1` over every new file population and the five
promoted CLI tests. All named files exist, including the four scenario inputs.

| File | Exists | Evidence |
|------|--------|----------|
| `ppp/lcp_config.go`, `ppp/yang/`, `pppconfig/` | Yes | Source/header/default/grouping files and both provider test files read |
| Configured, convergence, duplicate-instance and proxy carriers | Yes | All listed `ppp/*lcp*_test.go` and both new RFC carriers returned by ls |
| Both transport configuration/admission tests and fixture helpers | Yes | `l2tp/lcp_config_test.go`, `pppoe/lcp_config_test.go`, `lcp_timing_linux_test.go` |
| Client timer/counter/Max-Failure tests | Yes | Both IPv6CP and both Max-Failure carriers read |
| Four new iface configuration/integration carriers | Yes | All four exact paths returned by ls and read |
| Five `test/parse/` files | Yes | Each current live file read, with accepted/refused inputs and exit assertions |
| Checker, relay and scenario | Yes | New Go files and README/ppp-options/xl2tpd.conf/ze.conf read |
| `website/changes/fragments/lcp-restart-counters.md` | Yes | Exact six-heading operator fragment read |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Configured Restart interval | Live pppd normal/cut/restored output and configured timer body inspected |
| AC-2 | RFC defaults | `ppp/yang` passed in `job-unit-pkg-e4dcd2b3.log`; four schema defaults and both consumers inspected |
| AC-3 | Exact initial/proxy budget | Complete PPP race pass and both real-Driver budget bodies inspected |
| AC-4 | Answering peer stops retries | Proxy control advances two seconds after reopen and requires exactly two requests |
| AC-5 | Exact termination limit | Configured Max-Terminate body decodes every frame's protocol/Code; PPP race pass |
| AC-6 | Matching Ack ends termination | Same test requires Closed after exactly one correctly typed request |
| AC-7 | Sixth default response is Reject | Actual original MRU bytes compared after five Naks; PPP race pass |
| AC-8 | Through-threshold responses are Naks | Maximums 2 and 5 checked for every reply through the limit |
| AC-9 | Configured ZRC grace | Strong native clean PASS / both wrong-Code FAIL, plus exact expiry and zero-count assertions |
| AC-10 | IRC renews only on table edges | Actual spent configured budget, Req-Sent/Ack-Sent and Ack-Rcvd contrast inspected; PPP pass |
| AC-11 | Cleanup precedes both restart writes | Writer inspects cleanup state on each response; PPP pass |
| AC-12 | Echo preserves lifetime | Exact Echo-Reply, Opened and zero-down-event assertions; PPP pass |
| AC-13 | Bounds refuse with ranges | Eight CLI refusal commands and meaningful refusal cut read; restored live pass |
| AC-14 | Default silence ends at 30 seconds | Existing restart-counter test and absence of production fixed negotiation timer inspected; PPP pass |
| AC-15 | One default authority | Shared schema test compares both expanded consumers and runtime parser values |
| AC-16 | Responsive lifetime bounded | Both configured and nested-auth native clean/broken outputs inspected; long silence passes |
| AC-17 | Independent IPv6CP interval/handoff | Both forced timer failures and restored kernel output, timer-handoff and reload bodies inspected |
| AC-18 | Late Nak renews budget | Nineteen wire requests and exact five-second fresh budget asserted; client race pass |
| AC-19 | Original refused instance only | All three duplicate/malformed cases compare exact bytes; PPP race pass |
| AC-20 | Client configured Max-Failure | Registered Dial kernel test checks Nak/Nak/Reject then healthy CHAP/IPCP; forced-five failure and restored pass |
| AC-21 | Structured transport repair | Both roots, opererr type, exact valid command and final repair line asserted; parser race pass |
| AC-22 | Unknown settings cannot default | Misspelled/unrelated refusal and accepted-known-setting controls inspected; parser race pass |

### Wiring Verified (end-to-end)

| Entry Point | .ci File or Linux carrier | Verified |
|-------------|---------------------------|----------|
| L2TP operator text | `test/parse/l2tp-lcp-restart.ci`, range companion and Linux parser/admission tests | Actual grouping/parser, reload snapshot and StartSession path |
| PPPoE operator text | `test/parse/pppoe-lcp-restart.ci`, Linux parser/live-admission tests | Actual PADR driver emits configured wire retries |
| StartSession admission | `TestPPPManagerAppliesLCPRestartConfig`, refusal companion | Valid snapshot reaches live session, omitted snapshot refuses |
| Restart expiry | Configured/proxy tests and `lcp-restart-timer-pppd` | Exact requests, observed periods, final state and cleanup |
| Client LCP limits | Existing LCP timing CLI plus `pppoe-client-lcp-max-failure.ci` and configured Dial integration | Schema → retained timing → Dial → actual replies |
| Client IPv6CP interval | `pppoe-client-ipv6cp-timing.ci` and configured Dial integration | Independent actual request receipts, unchanged options, both wrong constants refused |
| Upper-layer cleanup | `TestLCPDownPrecedesRestartWireResponse` | Existing event and acknowledgment complete before SCR/SCA/SCN |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Both consumer modules resolve the shared defaults and runtime parser agrees |
| A-2 | broken | RCN renews IRC indefinitely. Configured/Nak-loop, long-silence and nested restart regressions prove the replacement lifetime bound |
| A-3 | confirmed | Real pppd normal/cut/restored scenario observes its actual dropped reply and final cleanup |
| A-4 | confirmed | `applyTransition` selects Max-Terminate from companion STR; exact termination test passes |
| A-5 | confirmed | Production PADR → StartSession → driver path emits three configured 150 ms requests on Ze 7.3 |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Feature evidence | `features/l2tpv2-bng.md` names the new passing scenario; partial/experimental scope retained | Yes |
| Config syntax and both operator guides | Shared grouping ranges/defaults, both parsers/admissions and reload source inspected | Yes |
| PPP architecture/research | Counter actions, convergence edges, original-option conversion and retained snapshots inspected | Yes |
| Client architecture/guide | Dial carrier, one-shot IPv6CP owner, Max-Failure and independent IPCP timing inspected | Yes |
| Interop/functional-test documentation | Registered checker, relay policy, monotonic clock and cleanup source inspected | Yes |
| RFC capability declaration | Five rows match current producers; client no-local-STR statement matches PADT cleanup | Source verified; final native check has no owned-stem diagnostic |
| No CLI/RPC/plugin/SDK or packet-layout change | Diff changes configuration leaves and timing, no new wire method, command handler or message layout | No update needed |
| No new runtime dependency/doctor check | Production diff adds settings/timers, no new path, binary, service, socket, port, kernel module or certificate | No check added |
| Comparison, route metadata and Prometheus inventory | No corresponding new declaration or changed comparison claim | No update needed |
| Source anchors and closing citations | Source-anchored claims read, both external full-spec-path citations repointed | Final doc check passed; whole tracked-tree search has no external full-path citer |

## Core Insight

The Restart counter bounds unanswered requests. It does not bound a responding
negotiation that repeatedly renews IRC. The convergence deadline belongs to the
actual negotiating lifetime, not to an outer loop's delayed view of Opened.
