# Spec: bgp-session-ready-contract

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 4/4 |
| Updated | 2026-09-02 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/evidence.md`, `ai/rules/plugins.md`, `ai/rules/plugins.md`, `rfc/short/rfc4724.md`
4. `internal/component/bgp/reactor/peer_run.go`, `peer_initial_sync.go`, `peer.go`, `api_sync.go`, `internal/component/bgp/plugins/rib/rib_replay.go`, `internal/component/bgp/config/peers.go`

## Task

A BGP session withholds its EOR until every plugin that "may send updates" has
signalled readiness. **The set that is COUNTED is not the set that SIGNALS.**

The counter (`peer_run.go`) counts, per peer, every config `process`
binding carrying `send [ update ]`. The signal (`request peer <addr> plugin
session ready`) has exactly ONE production emitter in the tree: `bgp-rib`. Every
other plugin an operator can legally bind with `send [ update ]` is counted and
never speaks, so the session waits out `waitForAPISync(2s)`
(`peer_initial_sync.go`) after a 500ms floor and its EOR lands at
+2.5s.

The question this spec exists to answer, and which is Thomas's call, not the
implementer's:

**Who is entitled to delay a session's EOR, and how does a plugin declare that
it will signal?**

Scope note. The `bgp-rib` empty-replay dead branch (nil-vs-empty conflation) was
FIXED in `5c4421541` and is NOT this spec's subject. That commit closed one
member of the counted set. This spec is about the shape of the set itself.

Not an RFC violation. RFC 4724 Section 2 only RECOMMENDS sending EOR and sets no
deadline (`rfc/short/rfc4724.md`, `:415`). The cost is convergence latency at
the peer, which defers route selection until our EOR or its own
Selection_Deferral_Timer (`rfc/short/rfc4724.md`).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
- [ ] `ai/rules/evidence.md` - the wait is a guard; this is arguably a fail-OPEN wait
  → Constraint: "A guard must fail closed or say something." The API-sync timeout neither denies nor speaks: `peer.go` logs the timeout at **Debug**, then proceeds. Compare `api_sync.go`, where the unknown-peer miss on the SAME chain was deliberately raised to **Warn** with the reason recorded in the comment. The two ends of one chain disagree about loudness.
  → Constraint: "Make the miss explicit at the producer." The layer that knows a counted plugin never signalled is the reactor at timeout. Nothing downstream can see it.
- [ ] `ai/rules/plugins.md` - Registration Fields table; the contract would be a new registration field
  → Constraint: no `Registration` field today expresses "this plugin signals session readiness". The nearest fields (`SendTypes`, `EventTypes`, `Features`) are about wire/event surface, not establishment participation.
  → Decision: the signal is already generic and needs no new protocol. It is the RPC `ze-plugin:session-peer-ready` (`plugins/cmd/peer/session.go`), reachable by ANY plugin, internal or external, via `DispatchCommand("request peer <addr> plugin session ready")`.
- [ ] `ai/rules/plugins.md` - the counter is plugin knowledge held in the reactor
  → Constraint: the reactor must not learn plugin names. Any remedy must stay a registry/declaration lookup, not a spelling of `bgp-rib` in `reactor/`.
- [ ] `docs/architecture/config/syntax.md` - the design doc `reactor/config.go` declares, where `attach process <name>` and `plugin { internal <alias> { use <plugin> } }` are written down
  → Constraint: the peer names a PROCESS and the plugin block names an IMPLEMENTATION. The two namespaces meet nowhere in the reactor, which is why the declaration lookup had to resolve one to the other.
- [ ] `docs/architecture/core-design.md` - the design doc the plugin server's own files declare
  → Constraint: the registry is keyed on `Registration.Name`, and the process manager on the config name. A lookup that crosses them goes through the plugin server, which owns both.
- [ ] `docs/architecture/api/architecture.md` - documents "RIB signals ready" as the design
  → Decision: the doc describes the bgp-rib path only. It does not claim other plugins signal, so it is not wrong; it is silent on the gap. A remedy changes what this doc must say.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4724.md` - EOR semantics
  → Constraint: Section 2 is `[RECOMMENDED]`: send EOR on completion of the initial update even without GR. No deadline is set, so no delay here violates the RFC.
  → Constraint: Section 4.1: the receiver defers route selection until EOR from all peers OR its Selection_Deferral_Timer expires. Our late EOR is therefore a real convergence cost at the peer, not a cosmetic one.
  → Constraint: Section 4: EOR MUST be sent once the initial routing update completes. An EOR sent BEFORE a counted plugin's routes is a violation of this in spirit: it claims a completion that has not happened.
- [ ] `rfc/short/rfc2918.md` - route-refresh, the capability whose config rule creates the forced binding

**Key insights:**
- The signal mechanism is generic and already public; only its USE is single-plugin.
- The counted set is chosen by CONFIG (per peer), not declared by plugin code. No plugin anywhere states whether it participates.
- Failing "wait too long" is latency. Failing "do not wait" is correctness. The two failures are not symmetric, and the current design fails in the safe direction.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/bgp/reactor/peer_run.go` - FSM Established callback; `:361-367` counts the bindings and calls `ResetAPISync`
  → Constraint: the predicate is exactly `binding.SendUpdate` over `p.settings.ProcessBindings`. Nothing else narrows it. Comment `:359-360` states the intent: "count plugins with SendUpdate permission. They will signal 'plugin session ready' after replaying routes." That sentence is the bug: it is true of bgp-rib and false of every other member.
- [ ] `internal/component/bgp/reactor/peer_initial_sync.go` - `sendInitialRoutes`; `:171-178` the wait, `:329-337` the EOR
  → Constraint: the wait is armed by `apiSyncExpected > 0` and costs an UNCONDITIONAL 500ms floor plus up to 2s. The 500ms is deliberate and separate: comment `:164-170` explains it covers external plugins whose state=up handling is pipe round-trips not tracked by apiSync. Even a perfect signaller cannot bring the EOR below +500ms.
  → Constraint: the EOR for every negotiated family is sent only after that wait returns, under `session.HoldWrites()`.
- [ ] `internal/component/bgp/reactor/peer.go` - `:392-446` the sync primitive
  → Constraint: `ResetAPISync` stores the count and a fresh channel. `SignalAPIReady` increments an ANONYMOUS counter and closes the channel only when `count >= expected`. Signals are NOT attributed to a plugin: two signals from one plugin satisfy two slots.
  → Constraint: `waitForAPISync` returns on the channel or the FULL timeout. Nothing shortens it. The timeout branch logs Debug and returns.
- [ ] `internal/component/bgp/reactor/api_sync.go` - `:191-209` `SignalPeerAPIReady` routes a signal to a peer
  → Decision: `:187-190` already argues, in prose, that a miss on this chain "must be loud" because "the peer simply waits out waitForAPISync and sends its EOR 2.5s late", citing `ai/rules/evidence.md`. The precedent for the remedy's diagnostic half is already in the file.
  → Constraint: naming collision, do not confuse. `Reactor.SignalAPIReady` (`:117`, process-wide startup gate) and `Peer.SignalAPIReady` (`peer.go`, per-session) are different functions with the same name. `plugins/rs/server.go` mentions the FORMER and is not a session-ready signaller.
- [ ] `internal/component/bgp/plugins/rib/rib_replay.go` - the only signaller
  → Constraint: `replayRoutesWithCursor` emits at `:259` (nothing to replay) and `:283` (after replay). These are the ONLY two production emitters in the tree.
  → Constraint: `resendRoutesWithCursor` and `rib_commands.go` deliberately do NOT signal (manual resend is not establishment).
- [ ] `internal/component/bgp/plugins/rib/rib.go` - `:1075-1080` calls the replay on the down-to-up transition (`cameUp`), including with zero groups
  → Decision: this is the `5c4421541` fix. Closed, not this spec's subject.
- [ ] `internal/component/bgp/plugins/cmd/peer/session.go` - `:11-28` registers `ze-plugin:session-peer-ready` and calls `ctx.Reactor().SignalPeerAPIReady(ctx.Peer)`
  → Decision: generic, plugin-agnostic, available to every plugin. Option B needs no new protocol surface.
- [ ] `internal/component/bgp/reactor/config.go` - `:723-763` builds `ProcessBindings` from the peer tree's `process` map; `:829-837` `parseOneSendFlag` sets `SendUpdate = true` for the token `update`
  → Constraint: membership of the counted set is decided by OPERATOR CONFIG per peer, not by plugin code. A plugin cannot opt out today, and a peer with no `process` block gets `apiSendCount == 0` and no wait at all.
- [ ] `internal/component/bgp/config/peers.go` - `:540-589` `validatePeerProcessCaps`
  → Decision: **this does NOT force `send [ update ]` onto route_refresh.** It requires that a peer carrying the route-refresh or graceful-restart capability has AT LEAST ONE binding with `SendUpdate`, else config is REJECTED. Comment `:537-539`: "These capabilities require a process to resend routes on demand."
- [ ] `internal/component/bgp/plugins/route_refresh/route_refresh.go`, `register.go` - what bgp-route-refresh actually is
  → Constraint: it is a CAPABILITY DECODER. `register.go`: `Features: "capa yang"`, `SupportsCapa: true`, `CapabilityCodes: [2, 70]`. `RunRouteRefreshPlugin` (`route_refresh.go`) registers only `WantsConfig: ["bgp"]` and a NO-OP `OnConfigure` whose comment says "Route-refresh has no payload. Config just enables the capability. Engine handles config-driven capability advertisement in reactor/config.go." It never sends a route and has no replay path.
  → Decision: `send [ update ]` on this plugin is therefore SEMANTICALLY VACUOUS. It grants a permission the plugin cannot exercise. The validator asks "does this peer have a route sender" and accepts a "yes" from a capability decoder.

**The two sets, enumerated (this is the spec's subject, so it is measured, not estimated):**

| Set | Definition (producer) | Members |
|-----|----------------------|---------|
| COUNTED | `peer_run.go`: every `ProcessBinding` with `SendUpdate == true`, per peer. Populated from config by `reactor/config.go` + `:831-832`. | Config-chosen. In-tree internal plugins observed bound this way: `bgp-rib`, `bgp-gr`, `bgp-persist`, `bgp-rs`, `bgp-route-refresh`. External/test plugins observed bound this way: `my-process`, `cursor-test`, `text-test`, `test-plugin`, `cli-grammar-test`, `commit-lifecycle-test`, `commit-workflow-test`, `acme-traffic-filter`, `flowspec`, `summary-check`. |
| SIGNALS | Emitters of `request peer <addr> plugin session ready` (grep of the literal across `internal/`, `pkg/`, `cmd/`, `test/`, excluding `_test.go` and mocks) | `bgp-rib` ONLY, at `rib_replay.go` and `rib_replay.go`, both inside `replayRoutesWithCursor`, reached only from `rib.go` and the `handleState` equivalent. |

**Gap:** 4 of the 5 in-tree plugins and 10 of 10 external test plugins observed
in the counted set never emit the signal. Method: repository-wide scan of `.ci`,
`.conf`, and doc configs for `process <name> { ... send [ ... update ... ] }`
inside a `peer` block, resolving each binding name through
`plugin { internal <name> { use <plugin> } }`. Membership was NOT inferred from
plugin names.

**Behavior to preserve:** (unless user explicitly said to change)
- The default direction of failure: when readiness is unknown, WAIT. Never emit an EOR ahead of a bound plugin's routes.
- `bgp-rib`'s signal on the down-to-up edge, including the empty-Adj-RIB-Out case (`5c4421541`, `TestPeerUpEmptyRibOutSignalsReady`, `TestPeerUpEmptyRibOutSignalsReadyOnceOnly`).
- The 500ms floor (`peer_initial_sync.go`) and its stated purpose, unless the spec explicitly reopens it.
- `resendRoutesWithCursor` and `rib_commands.go` sendRoutes NOT signalling.
- The generic, plugin-agnostic `ze-plugin:session-peer-ready` RPC as the signal transport.
- Config rejection of a route-refresh/GR peer with no route sender, IF the validator's intent survives the review in Open Question Q4.

**Behavior to change:** UNDECIDED. This is a skeleton. Options are laid out below and the choice is Thomas's.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- BGP FSM reaches `Established`: `peer_run.go` (`to == fsm.StateEstablished` in the FSM callback set at `:332`).
- Config entry: the peer's `process <name> { send [ update ] }` leaf-list, parsed at `reactor/config.go`.

### Transformation Path
1. `peer_run.go` counts `ProcessBindings` where `SendUpdate` is true into `apiSendCount`.
2. `peer_run.go` `ResetAPISync(apiSendCount)` stores `apiSyncExpected` and a fresh `apiSyncReady` channel (`peer.go`).
3. `peer_run.go` sets `sendingInitialRoutes`; `:383-384` notifies plugins of peer-established; `:397` spawns `sendInitialRoutes`.
4. Plugin side: `bgp-rib` sees the state event, `rib.go` calls `replayRoutesWithCursor`, which dispatches `request peer <addr> plugin session ready` (`rib_replay.go` or `:283`). No other plugin does anything here.
5. Engine side: the dispatcher routes it to `handlePeerSessionReady` (`plugins/cmd/peer/session.go`), which calls `Reactor.SignalPeerAPIReady` (`api_sync.go`), which resolves the peer and calls `Peer.SignalAPIReady` (`peer.go`).
6. `Peer.SignalAPIReady` increments an anonymous count and closes `apiSyncReady` only when `count >= expected` (`peer.go`).
7. `peer_initial_sync.go`: sleep 500ms, then `waitForAPISync(2s)` (`peer.go`) blocks on that channel or times out (`peer.go`, Debug log).
8. `peer_initial_sync.go` sends EOR for every negotiated family.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> Reactor | `process <name> { send [ update ] }` -> `ProcessBinding.SendUpdate` (`reactor/config.go`, `:831-832`) | [ ] |
| Reactor -> Plugin | peer-established event (`peer_run.go`) -> `rib.go` handleState / handleStructuredState | [ ] |
| Plugin -> Engine | `DispatchCommand("request peer <addr> plugin session ready")` -> RPC `ze-plugin:session-peer-ready` (`plugins/cmd/peer/session.go`) | [ ] |
| Engine -> Peer | `SignalPeerAPIReady` (`api_sync.go`) -> `Peer.SignalAPIReady` (`peer.go`) -> channel close | [ ] |
| Registry -> Reactor (PROPOSED, does not exist) | a declaration that a plugin participates in session readiness | [ ] |

### Integration Points
- `registry.Registration` (`internal/component/plugin/registry/`) - where a declaration would live under Option A/D.
- `pkg/plugin/sdk/` - external plugins would need the same declaration path (stage 1 `declare-registration`) under Option A/D.
- `ProcessBinding` (`reactor/peer_settings.go`) - the field the counter reads.
- `internal/component/bgp/config/peers.go` - the validator that creates the forced binding.

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)
- [ ] Registration over hardcoding — any remedy must be a registry declaration the reactor discovers, never a plugin name spelled in `reactor/`

## Blast Radius (measured)

Method: repository scan of `test/`, `etc/`, `demos/`, `docs/` for peer blocks
containing at least one `process` binding with `send [ update ]`, resolving
binding names via `internal <name> { use <plugin> }`. 80 peer blocks qualify.

| Counted set for the peer | Peers | Can the wait end early? | Status |
|--------------------------|-------|------------------------|--------|
| `[bgp-rib]` only | 47 | Yes, since `5c4421541` | Healthy |
| `[bgp-gr, bgp-rib]` | 10 | **No.** `expected == 2`, only bgp-rib ever signals, so `count >= expected` (`peer.go`) is never satisfied and the channel never closes. | **Broken, and NOT fixed by `5c4421541`** |
| No `bgp-rib` at all | 23 | No. Nothing in the counted set has a signaller. | Broken |

The 23 with no bgp-rib: `bgp-gr` alone (6: `graceful-restart.ci`,
`gr-cli-restart.ci`, `logging-{syslog,stderr,level-filter,config-file}.ci`),
`bgp-persist` (2 peer blocks in `test/reload/persist-across-restart.ci`),
`bgp-route-refresh` (1: `test/plugin/api-route-refresh.ci`), `bgp-rs` (2 doc
examples in `docs/guide/route-reflection.md`), an unresolved `gr` binding
(`test/parse/graceful-restart-llgr.ci`), and 11 external test plugins.

**Demonstrated vs theoretical. Stated plainly.**

| Case | Status | Evidence |
|------|--------|----------|
| `bgp-route-refresh` alone (`api-route-refresh.ci`) | **DEMONSTRATED** | Recorded in `spec-fixit-redistribute-establishment-stall` <!-- doc-links: ignore (spec closed on 2026-08-23 by 8f3a80bf9 and deleted from plan/) --> rows E1/E2 (run 2026-07-16 by the investigating session; **NOT re-run by this spec's author**). E1: log shows `sleeping for API routes duration=500ms` @14.187, `waiting for API sync expected=1` @14.688, then NO `API sync complete` and NO `sent EOR`. E2: raising the test plugin's sleep to 2.0/3.0/4.0s makes the EOR arrive and both expectations match. The EOR is LATE, not absent. |
| `[bgp-gr, bgp-rib]` two-binding peers (10) | **THEORETICAL** | Producer chain read end to end (`peer_run.go` -> `peer.go` -> `peer.go`); no `bgp-gr` signaller exists (grep). No repro run. This is the highest-value experiment to run next: it would show `5c4421541` did not close these. |
| `bgp-persist`, `bgp-rs`, `bgp-gr` alone | **THEORETICAL** | Same chain, no signaller in those packages (grep). No repro run. |
| External/test plugins bound `send [ update ]` (10) | **THEORETICAL** | No `plugin session ready` emitter in any `.ci` python body (grep over `test/`). No repro run. Suspected cost: 2.5s per affected peer of functional-suite wall clock. |

## Why `send [ update ]` is forced on route_refresh at all

**Correction to the framing.** `peers.go` is not a force targeted at
route_refresh. It is the ERROR ARM of `validatePeerProcessCaps`
(`peers.go`), which requires that a peer carrying the route-refresh
capability or graceful-restart has AT LEAST ONE
binding with `SendUpdate` anywhere in its `process` set. If none
does, config is rejected. The rule names no plugin.

The force lands on route_refresh in `test/plugin/api-route-refresh.ci`
only because that peer binds nothing else that could satisfy it: `bgp-rib` is
not bound there, so the only way to make the config load is to put
`send [ update ]` on the route-refresh binding itself. Row E5 of
`spec-fixit-redistribute-establishment-stall.md` records that removing it makes
ze reject the config, which is why the obvious probe is not a probe.

**Is the force itself wrong?** Partly, and it should be reviewed, but fixing it
does NOT dissolve the mismatch.

| Finding | Basis |
|---------|-------|
| The validator's stated intent is sound | `peers.go`: route-refresh and GR "require a process to resend routes on demand". A peer that can be asked to resend and has nothing that can resend is a real misconfiguration. |
| Its test is too weak for its intent | It accepts ANY binding with `SendUpdate`, including `bgp-route-refresh` itself, which is a capability decoder (`register.go`: `Features: "capa yang"`, `CapabilityCodes: [2, 70]`) with a no-op `OnConfigure` (`route_refresh.go`) and no route-sending code at all. The permission is vacuous: it cannot be exercised. |
| Tightening it would not close the gap | If the validator demanded a genuine route sender, `api-route-refresh.ci` would become invalid config (correctly: it has no route sender). But `bgp-gr`, `bgp-persist`, `bgp-rs` and every external plugin would still be counted and still never signal. The 10 `[bgp-gr, bgp-rib]` peers would be untouched. |

Conclusion: the force is a real, separate defect worth its own decision (Q4
below), and it explains why route_refresh is the case that got noticed first.
It is not the cause of the contract question and removing it leaves the question
standing.

## Options (Thomas decides; this spec does NOT choose)

| ID | Option | Failure mode when someone forgets | Direction of that failure | Cost |
|----|--------|-----------------------------------|--------------------------|------|
| A | Count only plugins that declare they will signal (an explicit `SignalsReady`-style registration field). | A plugin author omits the declaration. The plugin is not counted, the wait is not armed for it, and the EOR races out ahead of its routes. **Silent.** | **Correctness.** Wrong direction. | Every plugin author must opt in. External plugins need the declaration in stage 1 `declare-registration` too. |
| B | Make every counted plugin signal (each plugin bound `send [ update ]` emits the existing generic RPC, including a "nothing to do" signal at establishment). | A plugin author omits the signaller. The peer burns 2s. **Silent today, but see B+.** | **Latency.** Same as today's status quo. No regression. | More code per plugin. Awkward for `bgp-route-refresh`, which has nothing to be ready ABOUT (see Q4). |
| C | Drop the wait for plugins that cannot signal, and document the race. | Any counted-but-non-signalling plugin's routes may follow the EOR. **Silent, and by design.** | **Correctness.** Wrong direction, chosen deliberately. | Cheapest. Trades a known latency for an unbounded correctness hazard. |
| D | Hybrid, fail-closed variant of A: the declaration is MANDATORY. Startup REJECTS a `send [ update ]` binding whose plugin has declared neither "I signal" nor "I never send at establishment". | Cannot be forgotten. Startup fails loudly with the plugin named. | **Neither.** The omission is caught before a session exists. | Highest: touches registry validation, the SDK, and the stage-1 protocol. A new plugin cannot be bound until it answers. |

**Recommendation: B, plus the loud-timeout half (call it B+). Thomas decides.**

The asymmetry is the whole argument:

1. Today's failure is a **2.5s convergence delay**. It is recoverable, bounded, and costs the peer some deferral time (RFC 4724 S4.1, `rfc4724.md`). No RFC is violated (S2 sets no deadline, `rfc4724.md`).
2. The opposite failure is an **EOR sent before a plugin's routes**. That is a claim of completion that is false (RFC 4724 S4, `rfc4724.md`), it is unbounded (the peer may make selection decisions on an incomplete view and propagate them), and nothing anywhere notices.
3. A remedy should therefore make a MISTAKE LOUD rather than fast. Option A and Option C both convert a forgotten step into the correctness failure, silently. Option B converts a forgotten step into the status quo latency: the worst case of B is exactly today.
4. `ai/rules/evidence.md` names this shape: the wait IS the guard, and A/C make a miss "fall through to the permissive branch". B keeps the deny.
5. B's own weakness (the forgotten signaller is silent) is cheap to close and the codebase already argues for it. `api_sync.go` raised the unknown-peer miss on this exact chain to Warn precisely because "the peer simply waits out waitForAPISync and sends its EOR 2.5s late". The timeout branch at `peer.go` is still Debug. Making it a Warn that names the peer, the expected count, and the received count turns every remaining instance of this bug class into an operator-visible line instead of an archaeology exercise. That is the "or say something" half of the rule.
6. D is the strongest alternative and the one to consider if Thomas wants the forgotten opt-in to be impossible rather than merely harmless. It is the only option that makes the omission a build-or-boot failure. Its cost is the SDK and stage-1 protocol change; its benefit is that the counter set becomes self-describing AND cannot silently shrink.

Argument AGAINST B, recorded honestly: it asks `bgp-route-refresh` to signal
readiness for work it never does. That is not a signaller, it is a formality to
satisfy a counter. If Q4 concludes the validator should demand a genuine route
sender, B's awkward case disappears and B becomes clean. If Q4 concludes
otherwise, B leaves a plugin emitting a ceremonial signal, which is a smell that
D does not have.

## Open Questions (for Thomas)

| # | Question | Why it matters |
|---|----------|----------------|
| Q1 | Which option (A/B/C/D)? | The whole spec. |
| Q2 | Should the API-sync timeout be loud (Warn naming the peer and the shortfall)? Independent of Q1 and cheap. | `peer.go` is Debug; `api_sync.go` on the same chain is Warn. The inconsistency is why the 2.5s stall survived so long. |
| Q3 | Signals are anonymous (`peer.go` counts, does not attribute). Should a signal carry the plugin identity so two signals from one plugin cannot satisfy two slots? | Under B this becomes load-bearing: the 10 `[bgp-gr, bgp-rib]` peers need TWO distinct signallers, and an anonymous double-signal from bgp-rib would fake it. |
| Q4 | Should `validatePeerProcessCaps` demand a genuine route sender rather than any `SendUpdate` binding? | It would invalidate `api-route-refresh.ci`'s config (arguably correctly) and remove B's awkward case. It is a config-surface break for any operator relying on the loose check. |
| Q5 | The 500ms floor (`peer_initial_sync.go`) is unconditional whenever the count is above zero. Even a perfect signaller cannot beat +500ms. In scope or not? | It is half the "fast" case's cost and is a blind sleep, which `spec-fixit-migrate-sleeps-infra` is removing elsewhere. |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `bgp-rib` is the ONLY production emitter of the ready signal. | Grep of the literal `plugin session ready` across `internal/`, `pkg/`, `cmd/`, `test/`, `docs/`. Only non-test hits that DISPATCH are `rib_replay.go` and `:283`. | Every count in the Blast Radius table is wrong. | Re-grep at implementation start; also grep the external SDK and python plugins for the RPC name `session-peer-ready`. | **broken** (2026-09-02). Four in-tree emitters: `bgp-rib` (`rib_replay.go`), `bgp-rs` (`rs/server_handlers.go`), `bgp-adj-rib-in` (`adj_rib_in/rib.go`), `bgp-watchdog` (`watchdog/watchdog.go`). |
| A-2 | The counted set is decided by per-peer CONFIG, not by any plugin declaration. | `peer_run.go` reads `p.settings.ProcessBindings`; `reactor/config.go` builds them solely from the peer tree's `process` map. | Option A/D may already have a partial home. | Read `peer_settings.go` and confirm no registry field feeds `SendUpdate`. | **confirmed at read, then CHANGED by this spec.** `Registration.PeerUpBarrier` was the partial home the row anticipated: a voluntary declaration the engine intersects with the delivery set. This spec adds its sibling. |
| A-3 | A peer with `[bgp-gr, bgp-rib]` burns the full 2s despite `5c4421541`, because `count >= expected` (`peer.go`) needs 2 signals and only 1 arrives. | `peer.go` read; no bgp-gr signaller found by grep. | The blast radius shrinks by 10 peers and the "not fixed by 5c4421541" claim is false. | Run `test/plugin/gr-mark-stale.ci` ... **NOT run for this skeleton.** | **confirmed, in the equivalent shape.** `./le functional plugin` before the change logged the stall four times: `api sync timed out ... silent=adj-rib-in` on three peers and `silent=bgpls-withdraw` on one. Same mechanism, a barrier naming a process that never reports. |
| A-4 | `bgp-route-refresh` cannot send a route, so `send [ update ]` on it is vacuous. | `register.go` (capa/decoder registration), `route_refresh.go` (`RunRouteRefreshPlugin`, no-op `OnConfigure`, no send path). | Q4's premise collapses and the validator is fine as written. | Grep the package for any reactor send/announce call. | **confirmed, and MOOT.** No config in the repository binds `bgp-route-refresh` with a route-push grant any more: `test/plugin/api-route-refresh.ci` now satisfies `validatePeerProcessCaps` through `bgp-rib`. Q4 has no case left to decide. |
| A-5 | RFC 4724 sets no EOR deadline, so no option here is an RFC violation on the latency axis. | `rfc/short/rfc4724.md`, `:415` ([RECOMMENDED], Section 2). | Option B/D's latency is a conformance issue, not just a cost. | Re-read `rfc/short/rfc4724.md` Sections 2 and 4. | **confirmed** (2026-09-02), read in `rfc/full/rfc4724.txt`. Section 2: "the generation of such a marker upon completion of the initial update would be useful for routing convergence in general, and thus the practice is recommended". Section 4: "The End-of-RIB marker MUST be sent by a BGP speaker to its peer once it completes the initial routing update (including the case when there is no update to send) for an address family after the BGP session is established". The obligation is on ORDER, not on elapsed time, which is what the barrier preserves and what an unresolved declaration broke. |
| A-6 | The 500ms floor is deliberate and covers external plugins not tracked by apiSync. | Comment `peer_initial_sync.go`. Note per `ai/rules/evidence.md` this is a COMMENT, i.e. its author's belief, not a decision record. | Q5's framing is wrong. | Search `plan/learned/` and `plan/deferrals.md` for the decision that introduced the 500ms sleep. | **broken, and MOOT.** The floor is gone. `sendInitialRoutes` takes ONE bounded wait and no unconditional sleep (`peer_initial_sync.go`, the comment at the `needsAPIWait` branch records why). Q5 has nothing left to decide. |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Option A/D ships and one plugin's declaration is forgotten, producing a premature EOR that no test asserts. | Nothing. That is the risk. | Do not ship A without D's mandatory-declaration gate. Add a test that a NON-declaring bound plugin fails startup rather than racing. |
| R-2 | Anonymous signal counting (`peer.go`) lets a duplicate signal from one plugin satisfy another's slot, converting a latency bug into a correctness bug under B. | A peer whose EOR arrives fast despite a known non-signaller in its counted set. | Q3. Attribute signals per plugin, or keep `TestPeerUpEmptyRibOutSignalsReadyOnceOnly`'s once-per-edge invariant as a hard rule for every signaller. |
| R-3 | The remedy spells plugin names in `reactor/`, breaking `ai/rules/plugins.md`. | A `case "bgp-rib"` or an import of a plugin package appearing under `reactor/`. | Keep the declaration in the registry; the reactor reads a bool it does not interpret. |
| R-4 | Fixing the latency makes existing `.ci` tests that pass BECAUSE of the delay start failing (a test that polls long enough today may race a fast EOR tomorrow). | `.ci` reds in `test/plugin/` appearing only after the fix. | Expect it. Per `ai/rules/completion.md`, fix the test's synchronisation, never re-add a sleep. |
| R-5 | The 2.5s stall is currently load-bearing for some test's timing and its removal exposes an unrelated race. | Flaky reds in the GR/LLGR suites after the change. | Land the loud-timeout diagnostic (Q2) FIRST and separately, so the before/after picture is legible. |
| R-6 | Whichever option lands, `docs/architecture/api/architecture.md` still describes the bgp-rib-only design and goes stale. | A doc review finding. | Documentation Update Checklist row 12/15. |

## Wiring Test (MANDATORY — NOT deferrable)

<!-- Provisional. Names are concrete, but the entry-point column depends on the option chosen (Q1). -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Peer with a counted binding whose plugin never signals reaches Established | → | `waitForAPISync` timeout path (`reactor/peer.go`) | `TestAPISyncTimeoutIsLoud` (`internal/component/bgp/reactor/api_sync_test.go`) |
| Peer bound `[bgp-gr, bgp-rib]` reaches Established | → | `Peer.SignalAPIReady` count vs expected (`reactor/peer.go`) | `TestAPISyncTwoBindingsOneSignaller` (`internal/component/bgp/reactor/api_sync_test.go`) |
| Session establishment with a non-signalling counted plugin, end to end on the wire | → | EOR emission (`reactor/peer_initial_sync.go`) | `test/plugin/session-ready-contract.ci` |

## Acceptance Criteria

<!-- Provisional. AC-1..AC-3 hold under EVERY option and can be written now. AC-4..AC-6 depend on Q1. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A peer's API-sync wait reaches its timeout with `count < expected`. | The reactor logs at Warn, naming the peer, the expected count, and the received count. The miss is visible without a debug build. (Independent of Q1; matches `api_sync.go`'s stated principle.) |
| AC-2 | A peer is bound to two plugins with `send [ update ]`, only one of which signals. | The behavior is DEFINED and tested, not incidental. Whichever option lands, a test pins what happens. |
| AC-3 | A plugin bound `send [ update ]` sends its ready signal twice for one establishment. | It cannot satisfy another plugin's slot. (Depends on Q3.) |
| AC-4 | A peer bound only to plugins that participate correctly. | Its EOR is sent without waiting for the 2s timeout. |
| AC-5 | A peer bound to a plugin that will never send routes at establishment. | The EOR is not delayed on its account. The mechanism by which this is known is Q1's answer. |
| AC-6 | `test/plugin/api-route-refresh.ci` runs. | It passes without a blind sleep sized to the stall, and without weakening any expectation. (Per `spec-fixit-redistribute-establishment-stall.md`: do NOT close it by bumping the sleep.) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Brings up a BGP peer bound to a route-sending plugin and expects prompt convergence | FSM Established -> counter -> plugin replay -> ready signal -> EOR | `test/plugin/session-ready-contract.ci` |
| 2 | Binds a plugin that never signals and wonders why convergence is slow | FSM Established -> counter -> timeout -> Warn naming the peer and shortfall | `TestAPISyncTimeoutIsLoud` |
| 3 | Configures a route-refresh peer per `docs/guide/` | config validation (`peers.go`) -> establishment -> EOR | `test/plugin/api-route-refresh.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAPISyncTimeoutIsLoud` | `internal/component/bgp/reactor/api_sync_test.go` | AC-1: the timeout path speaks at Warn with peer, expected, received | |
| `TestAPISyncTwoBindingsOneSignaller` | `internal/component/bgp/reactor/api_sync_test.go` | AC-2: the `[bgp-gr, bgp-rib]` shape has defined, tested behavior | |
| `TestAPISyncDuplicateSignalDoesNotSatisfySecondSlot` | `internal/component/bgp/reactor/api_sync_test.go` | AC-3 / R-2: anonymous counting cannot be gamed | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `apiSyncExpected` | 0..N bindings | N | N/A (0 means no wait, `peer.go`) | N/A |
| API-sync timeout | 2s today (`peer_initial_sync.go`) | 2s | (fill during design, Q5) | (fill during design, Q5) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `session-ready-contract` | `test/plugin/session-ready-contract.ci` | A peer bound to a non-signalling plugin: assert the defined EOR behavior on the wire | |
| `api-route-refresh` | `test/plugin/api-route-refresh.ci` | Existing test, currently red for this reason. Must pass without a blind sleep. | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| (fill during design) | `test/interop/scenarios/` | FRR or BIRD | EOR timing observed by a real speaker's selection-deferral behavior. Likely NOT required: no wire format changes, only timing. Decide at design. | |

### Future (if deferring any tests)
- None. The skeleton defers the DECISION, not the tests.

## Files to Modify
<!-- Provisional. The exact set depends on Q1. -->
- `internal/component/bgp/reactor/peer.go` - the timeout path's diagnostic; possibly the counting/attribution
- `internal/component/bgp/reactor/peer_run.go` - the counter predicate if the counted set changes
- `internal/component/bgp/reactor/api_sync.go` - `SignalPeerAPIReady` if signals gain identity
- `internal/component/plugin/registry/` - a participation declaration (Options A/D only)
- `internal/component/bgp/config/peers.go` - `validatePeerProcessCaps` if Q4 says yes
- `internal/component/bgp/plugins/route_refresh/` - only if Option B and only if Q4 says the binding stays
- `docs/architecture/api/architecture.md` - `:1278-1288` describes the bgp-rib-only design

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] | Only if the `send [ update ]` leaf-list semantics change |
| CLI commands/flags | [ ] | No new command expected |
| Functional test for new RPC/API | [ ] | `test/plugin/session-ready-contract.ci` |
| Prometheus counters/metrics | [ ] | Consider a counter for API-sync timeouts per peer: the stall is currently invisible to monitoring |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | (answer at design) |
| 2 | Config syntax changed? | [ ] | Only if Q4 tightens `send [ update ]` validation |
| 3 | CLI command added/changed? | [ ] | (answer at design) |
| 4 | API/RPC added/changed? | [ ] | `docs/architecture/api/commands.md` (`:295` lists `plugin session ready`) |
| 5 | Plugin added/changed? | [ ] | `docs/guide/plugins.md` |
| 8 | Plugin SDK/protocol changed? | [ ] | `ai/rules/plugins.md` Registration Fields table (Options A/D) |
| 9 | RFC behavior implemented, changed, or newly proven? | [ ] | `rfc/short/rfc4724.md`, `docs/features/rfc-status.md` |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/api/architecture.md` |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | [ ] | `docs/plugin-overview.md` (Options A/D add a registration field) |
| 16 | Any changed source file is referenced by existing doc source anchors? | [ ] | Grep `docs/` for anchors on `peer_run.go`, `api_sync.go`, `rib_replay.go` |

## Files to Create
- `test/plugin/session-ready-contract.ci` - functional test for the chosen contract. **Written 2026-09-02.** It binds `bgp-rib` under the alias `ribout`, turns on `ze.log.bgp.routes=debug`, and asserts the barrier armed and completed. Its red was observed with the alias resolution removed.
- (Options A/D) a registration-field declaration and its self-containment test, location decided at design. **Done:** `Registration.SignalsSessionReady` with `TestInitialUpdateReporters`, `TestDeclaresSessionReadyResolvesTheProcessAlias` and `TestDeclaresSessionReadyReadsTheStageOneDeclaration`.

## Implementation Steps

<!-- THIN BY DESIGN. Status is `skeleton`; the approach is undecided (Q1). -->
<!-- Do NOT expand these into phases before Thomas answers Q1..Q5. -->

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Blast Radius, Options, Open Questions. Re-validate A-1..A-6 before any code. |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Phases below, which do not exist yet |
| 13. /ze-review gate | Review Gate section |

### Implementation Phases

1. **Phase: DECIDE (BLOCKING, not code)** - Thomas answers Q1..Q5. Status moves `skeleton` to `design`. Nothing below may start first.
2. **Phase: validate assumptions** - run A-3's experiment (the `[bgp-gr, bgp-rib]` repro) and re-grep A-1. Record results in the Assumptions table. This is the only work that may proceed before Q1.
3. **Phase: (fill during design)** - depends entirely on Q1.
4. **Functional tests** → Create after the contract is chosen. Cover user-visible EOR timing.
5. **RFC refs** → Add `// RFC 4724 Section 2 / Section 4` comments at the EOR emission and the wait.
6. **Full verification** → `./le verify current mode full`
7. **Complete spec** → learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | The counted set and the signalling set are provably identical, or their difference is declared and tested |
| Data flow | No plugin name appears in `reactor/`; the reactor reads a declaration it does not interpret |
| Registration over hardcoding | Any declaration is a registry field the core discovers (`ai/rules/plugins.md`) |
| Rule: fail-closed-guards | The guard still denies on a miss, and the miss says something |
| Rule: no-workarounds | No `.ci` test was closed by lengthening a sleep |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| The counted set and the signaller set agree, or disagree by declaration only | Re-run the config scan from Blast Radius; every counted plugin either signals or is declared non-participating |
| The API-sync timeout is operator-visible | Grep `peer.go` timeout branch for a Warn-level log naming the peer |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | A plugin-supplied ready signal is already unauthenticated beyond the plugin token. Under B, more plugins send it. Confirm a signal for peer X from a plugin not bound to peer X cannot shorten X's wait. |
| Resource exhaustion | A plugin that floods the signal must not corrupt other peers' counts (`peer.go` is an unbounded increment) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Assumption A-3 proves false | Update Blast Radius and re-present to Thomas. The recommendation may change. |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| `peers.go` forces `send [ update ]` onto route_refresh specifically (task framing, and `5c4421541`'s trailer). | It is the error arm of `validatePeerProcessCaps` (`peers.go`), which requires ANY binding with `SendUpdate` on a route-refresh/GR peer and names no plugin. It lands on route_refresh in `api-route-refresh.ci` only because that peer binds nothing else. | Read the producer at `config/peers.go`. | The remedy space is wider than "fix route_refresh". Recorded so the next reader does not go looking for a route_refresh special case that is not there. |
| `5c4421541` closed the `bgp-rib` case. | It closed the `[bgp-rib]`-only case (47 peers). The 10 `[bgp-gr, bgp-rib]` peers still cannot satisfy `count >= expected` (`peer.go`) because bgp-gr never signals. | Config scan + reading `peer.go`. | THEORETICAL (A-3), not demonstrated. Must be run before this claim is relied on. |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Enumerate the counted set from plugin names or registry metadata. | Membership is per-peer config (`reactor/config.go`), not a plugin property. A name-based list would be a guess. | Scanned every in-repo peer config and resolved binding names through `internal <name> { use <plugin> }`. |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| A wait/guard whose two ends disagree about loudness (`api_sync.go` Warn vs `peer.go` Debug) let a 2.5s stall live indefinitely. | 2nd instance on this chain in one day (the fail-open auth empty-profiles class). | Possible addition to `ai/rules/evidence.md`: when a guard has a timeout arm and an error arm, both must speak at the same level. | Propose after Q2 is answered. |

## Design Insights
- The signal transport is already generic and public (`ze-plugin:session-peer-ready`, `plugins/cmd/peer/session.go`). The gap is not a missing mechanism, it is a missing DECLARATION and a missing convention. That makes Option B far cheaper than it looks and Option D a protocol change rather than a plumbing change.
- The counter's comment (`peer_run.go`, "They will signal 'plugin session ready' after replaying routes") is a promise the type system does not keep. It reads as documentation and functions as an assumption. This is the shape `ai/rules/evidence.md` warns about: a comment states its author's belief, and here the belief is true of exactly one plugin.
- The bug hid for so long because BOTH its failure mode (slow) and its detection surface (Debug logs) are quiet. Fixing the loudness is separable from fixing the contract and is worth landing first.

## Core Insight
A wait whose participants are chosen by one authority (operator config) and whose
terminators are chosen by another (plugin code) is not a contract, it is a
coincidence. It has held only because one plugin, `bgp-rib`, happens to be in
almost every config that arms the wait.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| **Option A**, declaration VOLUNTARY (Thomas, 2026-09-02) | A / B / C / D above | "count only plugins that declare they will signal ... but do not enforce it on other plugin which may have no reason to do". |
| The barrier needs THREE facts, not one | declaration alone | Declaration alone leaves `bgp-adj-rib-in` counted on a peer it is never told about, which is the stall being removed. The peer must also GRANT the route-push rail and DELIVER the peer-up event, because the report answers that event. |
| The declaration has two transports, one meaning | registry only | An external plugin is in no compile-time registry. `Registration.SignalsSessionReady` is the in-tree route, `DeclareRegistrationInput.SignalsSessionReady` the Stage-1 route, and `registry.SetRuntimeSessionReady` is the seam that gives the reactor one answer. Same shape as `Claims`, which is already declared both ways. |
| Q2 needed no work | raise Debug to Warn | `391fcee8d1` already did it, and better than asked: `waitForAPISync` Warns naming the peer and the SILENT PROCESS NAMES, which is the shortfall rather than a count of it. |

### Per-plugin verdicts (owner asked for one per in-tree plugin that can be bound `send [ update ]`)

| Plugin | Declares | Reason |
|--------|----------|--------|
| `bgp-rib` | yes | Adj-RIB-Out replay on the down-to-up edge IS the initial update; already reported (`rib_replay.go`). |
| `bgp-rs` | yes | Route-server replay to an establishing client; already reported (`server_handlers.go`). |
| `bgp-adj-rib-in` | yes | Peer-up replay of stored routes; already reported (`rib.go`). |
| `bgp-watchdog` | yes | Resends the peer's announced pool on peer-up; already reported (`watchdog.go`). |
| `bgp-persist` | yes, **and gained the report** | `replayForPeer` replays the stored ribOut and sends its own EOR on peer-up, and never reported. Added at both completion points, and at neither generation abort. |
| `bgp-rr` | yes, **and gained the report** | `replayForPeer` drives the adj-rib-in replay and sends its own EOR on peer-up, and never reported. Added as a deferred report on every path out, generation-checked the way `sendEOR` is. |
| `bgp-gr` | no | Puts no route of its own into the establishing peer's update. It dispatches `request bgp rib retain/mark-stale/purge-stale/release-routes`, which act on routes RECEIVED from that peer and reach OTHER peers, on down and timer edges. |
| `redistribute-orchestrator` | no | Its late-join replay is a request onto the redistribute event bus, answered by producers on their own schedule under a 30s TTL (`replay.go`), with no completion event. There is no moment at which it can say its routes are out. |
| `exabgp-bridge` | no | Bridges an external ExaBGP-compatible process that announces on its own schedule; the bridge cannot know when that process is finished. |
| `bgp-route-refresh` | no | A capability decoder with no send path (A-4), and no config binds it with a route-push grant any more. |

## Known Limitations
- **`test/plugin/api-raw.ci` is RED in 2 of 5 post-change runs and this change exposed it.** The `raw-test` fixture waits for establishment, then dispatches `peer peer1 raw hex ...`, and the dispatch answers `not connected`. The peer's two expectations are ze's own KEEPALIVE and the End-of-RIB, and an injected KEEPALIVE is byte-identical to ze's own, so the peer completes and closes before the injection lands. The 2.5s stall used to hold the session open past that race. Two defects sit under it: the assertion cannot tell the injected frame from ze's own, and the fixture races the peer's completion. Not repaired here: it would be the FOURTH test-scaffolding repair in one session, which `ai/rules/pre-release.md` stops.
- Only in-repo configs were scanned. Operator configs in the wild may bind other plugins with a route-push grant; a third-party plugin that pushes into the initial update and does not declare is not waited for, which is Option A's accepted cost (R-1).
- **The peer-up barrier still has the namespace defect this spec fixed for the session-ready declaration.** `countPeerUpBarrier` (`internal/component/bgp/server/events.go`) asks `registry.RequiresPeerUpBarrier(proc.Name())`, `bgp-rs` is the only plugin that declares `PeerUpBarrier`, and the scenarios attach it as `rs`, so the count is zero and that barrier never arms on those peers. Both producers were read on 2026-09-02. Not fixed here: it is a second barrier with its own before/after measurement, and the row is in `plan/journal/gate-excludes-part-of-its-population.md`.
- The suite's residual failures differ from run to run in a checkout several sessions are editing at once. Five `./le functional plugin` runs on 2026-09-02 answered 638, 640, 640, 642 and 639 of 650, and their failing sets share only five ids out of eleven. That is the class `plan/journal/parallel-copies-collide-on-a-deterministic-port.md` records for `test/plugin` under a high `-p`, which names `bgp-rs-fastpath-ebgp-shared` and `rfc7606-54-*` among its members. `api sync timed out` appears ZERO times in all five runs, so the barrier this work arms never runs to its bound, and the most a peer's marker can now be delayed is that bound. Only `session-ready-contract` is read as this work's signal.

## RFC Documentation

Add `// RFC 4724 Section 2: "<quoted requirement>"` above the EOR emission
(`peer_initial_sync.go`) and `// RFC 4724 Section 4` above the wait,
recording that the deferral is a local policy choice and not an RFC deadline.

## Implementation Summary

### What Was Implemented
- `registry.Registration.SignalsSessionReady` and `registry.SignalsSessionReady(name)`, the compile-time half of the declaration, modelled on `PeerUpBarrier` / `RequiresPeerUpBarrier`.
- `rpc.DeclareRegistrationInput.SignalsSessionReady` (= `sdk.Registration`), `plugin.PluginRegistration.SignalsSessionReady`, `registrationFromRPC`, `(*Server).declaresSessionReady` and `registry.SetRuntimeSessionReady`: the Stage-1 half, which is the only route an external plugin has.
- `ProcessBinding.ReceivesPeerState` and `Peer.initialUpdateReporters`, which replace the bare `MayPushRoutes` walk at Established.
- Declarations on `bgp-rib`, `bgp-rs`, `bgp-adj-rib-in`, `bgp-watchdog`; declarations AND new reports on `bgp-persist` and `bgp-rr`.
- Three fixtures declare and report: `plugin/initial-sync-barrier-raw`, `plugin/plugin-announce`, `plugin/plugin-attributes`, each with `receive [ state ]` added to its binding.

### Second pass, after the independent review of `7f7a2453c` (2026-09-02)
- The declaration lookup crossed two namespaces and answered false.
  `Peer.initialUpdateReporters` asks under the binding's PROCESS name, which is
  the operator's `attach process <name>` key, and the registry is keyed on
  `Registration.Name`. `plugin { internal rs { use bgp-rs } }` therefore answered
  "declares nothing" for every aliased binding of a declaring plugin, which is
  the form the repository's own scenarios use. `(*Server).declaresSessionReady`
  (`internal/component/plugin/server/events.go`) now resolves the alias through
  `implementationNames` (`startup_claims.go`), the resolution `claimsForPlugin`
  already used, and the reactor still learns no plugin name.
- `bgp-persist` and `bgp-rr` reported and declared nothing. Both `register.go`
  files now carry `SignalsSessionReady: true`, so the six declarers the commit
  message, this spec and three doc pages name are the six the tree holds.
- `Peer.SignalAPIReady`'s uncredited-report line asserted "it pushes no route
  into this initial routing update", which the engine cannot know and which was
  false for exactly the case the defect produced. It now states what it
  observed, and prints the barrier's own names beside the sender's.

### Bugs Found/Fixed
- The stall itself. Before: `./le functional plugin` logged `api sync timed out ... silent=adj-rib-in` on three peers and `silent=bgpls-withdraw` on one. After: zero, across four runs.
- Suite: 643/650 best after, against 634/650 with the change reverted and rebuilt.
- The alias defect above. `test/plugin/session-ready-contract.ci` is its
  measurement: with the resolution removed and the daemon rebuilt, the run logs
  `plugin session ready from a process this peer's barrier does not name ...
  process=ribout barrier=[]` and then the marker, and the file fails on `stderr
  does not contain "API sync complete"`. With the resolution in place the same
  run logs `waiting for API sync expected=1`, `API sync complete`, `sent EOR`.
  Suite 640/650 in both runs; the file itself is the discriminator.
- FOUND, NOT FIXED: `countPeerUpBarrier` (`internal/component/bgp/server/events.go`)
  has the same namespace defect against `registry.RequiresPeerUpBarrier`, and
  `bgp-rs` is the only plugin that declares `PeerUpBarrier`. Row written in
  `plan/journal/gate-excludes-part-of-its-population.md`.

### Documentation Updates
- `docs/architecture/api/architecture.md` (API Sync Protocol steps 2-3-5, plus the declaration and grant paragraphs and three source anchors).
- `docs/architecture/plugin/plugin-system.md` (Registration Fields row, a new "Session-ready reports" section, and the stale `apiSync counts plugins ... 500 ms IPC grace` paragraph).
- `docs/guide/plugins.md` (new "Session-Ready Report" section written for a plugin author).
- `docs/architecture/api/process-protocol.md` (Stage-1 `signals-session-ready`).

Second pass, for the namespace resolution:
- `docs/architecture/api/architecture.md` (the process-name paragraph under the API Sync Protocol, plus anchors on `(*Server).declaresSessionReady` and `implementationNames`).
- `docs/architecture/plugin/plugin-system.md` (the two-namespace paragraph closing "Session-ready reports", plus the seam anchor).
- `docs/guide/plugins.md` (the paragraph claiming an external plugin "never declares and never has to" contradicted the Stage-1 route this work added; it now names `signals-session-ready`, and a third paragraph tells a plugin author which NAME it is waited for under).

### Deviations from Plan
- Q3 and Q5 were already answered by the tree and needed no decision. The barrier holds NAMES and dedups by process (`391fcee8d1`), so a duplicate report cannot fill another slot; the 500ms floor no longer exists.
- Q4 has no case left to decide (A-4).
- NOT DONE: `test/plugin/api-raw.ci` fails in 2 of 5 post-change runs. See Known Limitations.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Answer "who is entitled to delay a session's EOR" | Done | `Registration.SignalsSessionReady`, `rpc.DeclareRegistrationInput.SignalsSessionReady` | Option A, voluntary (Thomas, 2026-09-02) |
| Answer "how does a plugin declare it" | Done | `registry.SignalsSessionReady` plus the runtime seam `(*Server).declaresSessionReady` | Two transports, one meaning; the seam also resolves the process alias |
| The counted set is the set that reports | Done | `Peer.initialUpdateReporters` (`reactor/peer_run.go`) | Three facts: grant, declaration, peer-state delivery |
| Per in-tree plugin verdict | Done | "Per-plugin verdicts" table above | Six declare; four do not, each with a reason |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done, before this spec | `Peer.waitForAPISync` Warn with `silent=` (`391fcee8d1`) | Names the silent processes rather than a shortfall count, which is more than AC-1 asked |
| AC-2 | Done | `TestInitialUpdateReporters` case "one qualifying binding beside one that does not" | A two-binding peer waits for the declarer only, and the behavior is pinned rather than incidental |
| AC-3 | Done, by construction | `Peer.SignalAPIReady`: `apiSyncSignalled` is a SET keyed by process | A second report from one process is not credited twice (`391fcee8d1`) |
| AC-4 | Done | `test/plugin/session-ready-contract.ci`: `API sync complete` then `sent EOR`, 767 ms for the whole file | The barrier releases on the report, not on the timeout |
| AC-5 | Done | `TestInitialUpdateReporters` cases "declaring plugin with no peer-state grant" and "non-declaring plugin" | A plugin that will not send at establishment is not waited for |
| AC-6 | Done | `test/plugin/api-route-refresh.ci` passes in both suite runs of 2026-09-02, no sleep added | The file is unchanged by this work |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestAPISyncTimeoutIsLoud` | Superseded | `Peer.waitForAPISync` | The loud timeout landed in `391fcee8d1` before this spec; `apiSyncSilent` names the processes |
| `TestAPISyncTwoBindingsOneSignaller` | Superseded by `TestInitialUpdateReporters` | `internal/component/bgp/reactor/initial_update_reporters_test.go` | The AC-2 shape is a case in the table test, over the predicate that decides it |
| `TestAPISyncDuplicateSignalDoesNotSatisfySecondSlot` | Superseded | `Peer.SignalAPIReady` | The set replaced the count in `391fcee8d1`; two reports from one process cannot fill two slots |
| `TestInitialUpdateReporters` | Done | same file | 8 shapes; 4 went RED against the bare `MayPushRoutes` predicate |
| `TestInitialUpdateReportersReadsTheRuntimeDeclaration` | Done | same file | The external plugin's route through the seam |
| `TestInitialUpdateReportersAsksUnderTheProcessName` | Done (added 2026-09-02) | same file | Pins WHICH name the reactor asks under, which is the name the barrier holds |
| `TestDeclaresSessionReadyResolvesTheProcessAlias` | Done (added 2026-09-02) | `internal/component/plugin/server/events_session_ready_test.go` | RED with the resolution removed: "the alias must be answered from the registration its use spelling names" |
| `TestDeclaresSessionReadyReadsTheStageOneDeclaration` | Done (added 2026-09-02) | same file | Keeps the external route open beside the alias resolution |
| `session-ready-contract` | Done (added 2026-09-02) | `test/plugin/session-ready-contract.ci` | The end-to-end file the plan named; its red is recorded in its own header |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/reactor/peer.go` | Changed | The uncredited-report line says what it observed and prints the barrier |
| `internal/component/bgp/reactor/peer_run.go` | Changed | `initialUpdateReporters` replaced the bare `MayPushRoutes` walk |
| `internal/component/bgp/reactor/api_sync.go` | Unchanged | Signals gained identity in `391fcee8d1`, before this spec |
| `internal/component/plugin/registry/` | Changed | The declaration, its lookup, and the runtime seam |
| `internal/component/bgp/config/peers.go` | Unchanged | Q4 has no case left to decide (A-4) |
| `internal/component/bgp/plugins/route_refresh/` | Unchanged | Does not declare; no config binds it with a route-push grant |
| `docs/architecture/api/architecture.md` | Changed | Both passes, listed under Documentation Updates |
| `test/plugin/session-ready-contract.ci` | Created | The Files to Create row |

### Audit Summary
- **Total items:** 27 (4 requirements, 6 ACs, 9 tests, 8 files)
- **Done:** 23
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 4 (three planned unit tests superseded by work that landed in `391fcee8d1`, and one planned file left unchanged because its question dissolved)

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| The counted set and the signalling set agree, or differ only by an explicit declaration | functional test + unit | `test/plugin/initial-sync-barrier-raw.ci` proves a DECLARING external process is waited for (all three facts load-bearing, stated in its header). `TestInitialUpdateReporters` pins the predicate over 8 shapes and went RED on 4 of them when the predicate was reverted to bare `MayPushRoutes`. |
| The stall the spec exists to remove is gone | suite log | `./le functional plugin` before: four `api sync timed out; these processes never reported ... silent=adj-rib-in` (three peers) and `silent=bgpls-withdraw` (one). After: that string appears zero times in four consecutive runs. Suite 643/650 against a 634/650 baseline measured with `peer_run.go` reverted to HEAD and rebuilt. |
| A non-signalling counted plugin is operator-visible | source | `Peer.waitForAPISync` Warns with `silent=` from `Peer.apiSyncSilent()`. Already true before this work (`391fcee8d1`), which is why Q2 needed no change. |
| The marker still follows the routes it claims | suite log | `out-of-order marker accepted in silence`: 0 in both pre-change runs, 2 per run after the barrier narrowed, 0 in both runs after the three fixtures declared and reported. |
| The declaration reaches a plugin the operator RENAMED, which is how nearly every scenario binds one | functional test, both directions | `test/plugin/session-ready-contract.ci` binds `bgp-rib` as `ribout`. Resolution removed and the daemon rebuilt: `plugin session ready from a process this peer's barrier does not name ... process=ribout barrier=[]`, then the marker, and the file fails on `stderr does not contain "API sync complete"`. Resolution restored: `waiting for API sync expected=1`, `API sync complete`, `sent EOR`, and the file passes in 767 ms. `TestDeclaresSessionReadyResolvesTheProcessAlias` was observed RED against the same removal. |
| Every plugin whose code claims it declares does declare | source | `SignalsSessionReady: true` in six `register.go` files: `rib`, `rs`, `adj_rib_in`, `watchdog`, `persist`, `rr`. `grep -rn SignalsSessionReady internal/component/bgp/plugins/*/register.go` answers six, which is the number the commit message, the per-plugin table and three doc pages state. |

**A test was NOT written for `bgp-persist` and `bgp-rr`'s new reports.** `./le functional reload` is 40/40 and carries `test/reload/persist-across-restart.ci`, and `plugin/rr-basic` passes, but neither asserts the report. Both plugins were previously counted and stalled, so the reports restore an ordering nothing pins.

## Review Gate

### Run 1 (initial) -- independent review of `7f7a2453c`, 2026-09-02
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | The declaration lookup crosses two namespaces and answers false. `binding.PluginName` is the `attach process <name>` key; the registry is keyed on `Registration.Name`. Every aliased binding of a declaring plugin is dropped from the barrier | `Peer.initialUpdateReporters` (`reactor/peer_run.go`) into `registry.SignalsSessionReady` | Fixed. The seam `(*Server).declaresSessionReady` resolves the alias through `implementationNames`, the resolution `claimsForPlugin` uses |
| 2 | BLOCKER | `bgp-persist` and `bgp-rr` report and declare nothing; both carry a comment claiming they declare | `persist/register.go`, `rr/register.go` | Fixed. Both now set `SignalsSessionReady: true`; six declarers, as every doc page says |
| 3 | ISSUE | The uncredited-report diagnostic asserts a conclusion it cannot know ("it pushes no route into this initial routing update") | `Peer.SignalAPIReady` (`reactor/peer.go`) | Fixed. It states what it observed and prints the barrier's names |
| 4 | ISSUE | `docs/guide/plugins.md` says an external plugin "never declares and never has to", contradicting the Stage-1 route the same change added | `docs/guide/plugins.md` | Fixed. The three pages now agree with the tree |
| 5 | NOTE | `countPeerUpBarrier` has the same namespace defect against `registry.RequiresPeerUpBarrier`, and `bgp-rs` is the only plugin declaring `PeerUpBarrier` | `internal/component/bgp/server/events.go` | NOT fixed. Row in `plan/journal/gate-excludes-part-of-its-population.md`; arming a barrier inert on every aliased peer owes its own measurement |

### Fixes applied
- Findings 1 to 4, each with its own test or page edit; the evidence is in Implementation Summary, second pass.

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| | | | | |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/plugin/session-ready-contract.ci` | yes | Ran as case 630 of `./le functional plugin`, PASS in 767 ms |
| `internal/component/plugin/server/events_session_ready_test.go` | yes | `go test` over the package with the feature tag set: ok |
| `internal/component/bgp/reactor/initial_update_reporters_test.go` | yes | Same run: ok |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-2, AC-5 | The predicate keeps a binding only when all three facts hold | `TestInitialUpdateReporters`, 8 cases, green under the feature tags |
| AC-3 | A duplicate report cannot fill a second slot | `apiSyncSignalled` is a `map[string]struct{}` written once per process (`reactor/peer.go`) |
| AC-4 | The barrier releases on the report | `session-ready-contract.ci`: `waiting for API sync expected=1`, `API sync complete`, `sent EOR` |
| AC-6 | `api-route-refresh.ci` passes with no sleep added | Green in both suite runs of 2026-09-02; the file is untouched by this work |
| Six declarers | The tree matches every claim about the number | `grep -rn "SignalsSessionReady: true" internal/component/bgp/plugins/*/register.go` answers 6 |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| A peer that attaches a declaring plugin under an operator alias reaches Established | `test/plugin/session-ready-contract.ci` | yes, and RED with the alias resolution removed |
| A peer that attaches a DECLARING external process through `send [ raw ]` | `test/plugin/initial-sync-barrier-raw.ci` | yes (green in both suite runs) |
| A peer that attaches an aliased `bgp-rs` with `receive [ state ] send [ update ]` | `test/plugin/adj-rib-in-replay-on-peerup.ci` | yes; it passed before and after, so it is a regression guard here rather than the discriminator |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken | Four in-tree emitters, not one. Row A-1 carries them. |
| A-2 | confirmed, then changed by this spec | `Registration.PeerUpBarrier` was the partial home; `SignalsSessionReady` is its sibling. |
| A-3 | confirmed in the equivalent shape | Pre-change suite logged `api sync timed out ... silent=adj-rib-in` on three peers. |
| A-4 | confirmed, and moot | No config binds `bgp-route-refresh` with a route-push grant. |
| A-5 | confirmed | `rfc/full/rfc4724.txt` Sections 2 and 4, quoted in the Assumptions table. |
| A-6 | broken, and moot | The 500 ms floor no longer exists. |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| "The in-tree declarers are bgp-rib, bgp-rs, bgp-adj-rib-in, bgp-watchdog, bgp-persist and bgp-rr" (`architecture.md`, `plugin-system.md`) | Six `register.go` files carry the field, listed under AC Verified | yes |
| "An external plugin declares `signals-session-ready` at Stage 1" (`process-protocol.md`, `guide/plugins.md`) | `rpc.DeclareRegistrationInput.SignalsSessionReady`, read by `(*Server).declaresSessionReady`; `initial-sync-barrier-raw.ci` drives it | yes; the guide contradicted this and was corrected |
| "The declaration is looked up under the process name, and the seam resolves the alias" (all three pages) | `(*Server).declaresSessionReady` and `implementationNames` | yes |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `./le verify worktree` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` — no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-bgp-session-ready-contract.md`
