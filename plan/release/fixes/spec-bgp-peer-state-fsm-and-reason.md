# Spec: bgp-peer-state-fsm-and-reason

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | cli |
| Priority | P1 |
| Release reason | The peer state an operator reads today is Ze's lifecycle name, not the RFC 4271 FSM state, so a peer sitting in Idle between connect retries reads "connecting" and Ze's "active" means the opposite of the RFC's Active. Operators coming from FRR, BIRD or Cisco misread every down peer. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-31 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Every BGP peer view shows two things: the RFC 4271 Section 8.2.2 FSM state
(Idle, Connect, Active, OpenSent, OpenConfirm, Established) and the reason the
peer is in it, for example "connect refused, retry in 12s", "shut down by
operator", "prefix limit", "waiting for BFD".

Owner decisions, in his words and choices:

| Decision | Owner's answer |
|----------|----------------|
| Show the FSM? | "should we not present the fsm? users probably expect it" |
| FSM only, or more? | "The information is useful, but we need both." Then: "peer state or reason for being idle for example" |
| Machine-readable outputs | JSON `state` is renamed too: it becomes the FSM state, and Ze's lifecycle moves to another field |
| Bucket | release/fixes |
| Connect failure reason | Recorded: why the last outbound connect failed (refused, timeout, no route) and the time until the next retry |
| Design | A, structured: `state-reason` code, `reason-detail`, `next-retry` |
| Lifecycle `active` | Renamed `passive` |
| Metrics | Lifecycle metrics stay; a new FSM gauge is added |
| When | Spec now; implementation in a later session |

Trigger: the website showcase recording of `monitor bgp` showed peer
127.0.0.4 as "connecting" while no speaker listened there, so the FSM was
cycling Connect to Idle (RFC 4271 Section 8.2.2, Connect state, Event 18:
"changes its state to Idle"). The FRR peer later in the same recording showed
`state connecting` beside `fsm-state IDLE`.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/behavior/fsm.md` - the FSM states, events and BFD sub-states
  → Constraint: FSM names come from `fsm.State.String()` (`internal/component/bgp/fsm/state.go`, `stateNames`), upper case (IDLE, CONNECT, ACTIVE, OPENSENT, OPENCONFIRM, ESTABLISHED). Any lower-case JSON spelling is derived from that one table, never a second list (`ai/rules/principles.md`, declared once)
  → Decision: BFD sub-state is already shown as `bfd-sub-state` by `show bgp peer list` and `detail`; the reason column extends that precedent of "FSM state plus why"
- [ ] `docs/architecture/behavior/peer-lifecycle.md` - lifecycle versus FSM table (lines 30-50)
  → Constraint: this page states the two-level model; it becomes wrong the moment `state` changes meaning, so it is edited in the same phase
- [ ] `docs/architecture/api/birdwatcher-compat.md` - LG JSON contract
  → Constraint: `state` and `last_error` MUST be present; `last_error` names the NOTIFICATION that ended the session. A connect error MUST NOT be written into `last_error` without changing that normative page
- [ ] `ai/doors/system.md`, `bgp-engine.md`, `commands-cli.md`, `api-surfaces.md`, `plugin-infra.md`, `bgp-plugins.md`, `system-plugins.md`, `le-tooling.md`
  → Constraint: a JSON key consumed by scripts, the LG, MCP and the ExaBGP bridge is a contract; it changes on purpose, with its page, and every consumer moves in the same change (no-layering: no period where both meanings exist)
  → Constraint: the LG and web hold no peer import; they read the `show bgp` command answer, so the new fields must be in that answer, not computed in the adapter
  → Constraint: the ExaBGP bridge can run as an external process; it reads peer state only through the `show bgp peer detail` command answer, so it follows the key rename and nothing else

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md` - Section 8.2.2 state names
  → Constraint: RFC 4271 Section 8.2.2, Connect state, Event 18 with no DelayOpenTimer: "changes its state to Idle." A peer retrying an unreachable address is in Idle between attempts, and the display must say Idle, not a Ze-invented name

**Key insights:** (minimal context to resume after compaction)
- Two state values exist per peer: lifecycle `plugin.PeerState` (stopped, connecting, active, established, idle-hold) and session FSM `PeerInfo.FSMState`. Every summary surface shows only the lifecycle under the key `state`; only `show bgp peer detail` shows `fsm-state`
- `Peer.SessionState()` returns Idle whenever no session object exists, which is the whole backoff wait: hence "connecting" beside IDLE
- The outbound dial error is preserved as a wrapped `net.OpError` up to `Peer.run`, logged at Debug and dropped; the backoff deadline is a local in `Peer.run` and never stored
- `idle-hold` covers both operator shutdown and prefix-limit hold; the two cannot be told apart from any output today
- Three consumers already expect FSM names nothing produces: LG `stateClass`, web `peerFlagFromState` (capitalised), dashboard `stateRank` (`idle`, `opensent`, `openconfirm`)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/peer.go` - lifecycle `PeerState` and its names; the names also label `ze_peer_state_transitions_total` and `peerStateNames` (`peer_stats.go`) must match them
- [ ] `internal/component/bgp/reactor/reactor_api.go` - `reactorAPIAdapter.Peers` fills `PeerInfo.State` (lifecycle) and `FSMState` (`p.SessionState().String()`) for every peer
- [ ] `internal/component/bgp/reactor/peer_connection.go` - `Peer.SessionState` returns `fsm.StateIdle` when `p.session == nil`
- [ ] `internal/component/bgp/reactor/peer_run.go` - `Peer.run` backoff loop: local `delay`, `p.clock.After(fsm.Jitter(delay))`, prefix-limit timer `plan.Delay`; dial errors logged at Debug only
- [ ] `internal/component/bgp/reactor/session_connection.go` - `Session.Connect` dials, fires Event 18 on failure, returns the wrapped error
- [ ] `internal/component/bgp/plugins/cmd/peer/summary.go` - `handleBgpSummary` writes `state`, `last-error` (`lastErrorString`, last NOTIFICATION only), `state-changed`
- [ ] `internal/component/bgp/plugins/cmd/peer/peer.go` - `handleBgpPeerList` (`state`, `bfd-sub-state`), `handleBgpPeerDetail` (`state`, `fsm-state`), `handlePeerHistory`
- [ ] `internal/component/bgp/plugins/cmd/peer/health.go` - `state` in `show bgp health`
- [ ] `internal/component/cli/model_dashboard*.go` - `monitor bgp` parses `show bgp`, sorts by `stateRank`, colours by `stateColor`
- [ ] `internal/component/lg/handler_api.go` - `transformProtocols` copies `state` and `last-error` into birdwatcher `state`, `info`, `last_error`
- [ ] `internal/component/lg/render.go` - `stateClass` expects lower-case FSM names
- [ ] `internal/component/web/page_bgp_peers.go` - `peerFlagFromState` matches `Established` and `Idle`, capitalised
- [ ] `internal/exabgp/bridge/bridge_status.go` - `StatusPeers` reads `fsm-state` from detail and refuses a row without it
- [ ] `internal/component/mcp/ui/bgp-peer/app.js` - lower-cases `state` into a CSS class

**Behavior to preserve:**
- Plugin state events (`up`/`down`, `rpc.SessionState`) and FSM events (`fsm.from`, `fsm.to`): the lifecycle enum is never serialised into an event, so the event contract is untouched
- `show bgp peer history` rows (`from`, `to`, `reason` in FSM upper case)
- `last-error` and LG `last_error` keep meaning "the NOTIFICATION that ended the last session" (`birdwatcher-compat.md`)
- `bfd-sub-state` key and values
- Prometheus metric names `ze_peer_state` and `ze_peer_state_transitions_total` (scrapers depend on them); their label values are a design decision below

**Behavior to change:**
- JSON `state` on every peer answer (summary, list, detail, health, statistics, capabilities) becomes the RFC 4271 FSM state; Ze's lifecycle moves to another key (owner decision)
- A reason is added to every peer answer and shown beside the state in `monitor bgp`, the summary table, the web and LG views
- The last outbound connect failure and the next retry time are recorded per peer and reach the reason
- Operator shutdown and prefix-limit hold become distinguishable
- Consumers that read `state` or `fsm-state` move to the new keys in the same change: dashboard, web, LG, MCP app, ExaBGP bridge, interop helper, terminal demo validators, fixtures, `.ci` tests

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Lifecycle peer state | `reactor/peer.go` `PeerState`, mirrored by `plugin/types_bgp.go` `PeerState` | Ze's own run state, shown as `state` everywhere | Extends: kept as the internal model and moved to its own JSON key; it stops being "the state" |
| Session FSM state | `fsm/state.go` `State.String`; `PeerInfo.FSMState`; `peer detail` `fsm-state` | RFC state, shown only in detail, upper case | Extends: becomes the `state` key on every answer; the separate `fsm-state` key is replaced, not kept beside it (no-layering) |
| BFD sub-state display | `Peer.bfdSubState`, `PeerInfo.BFDSubState`, `peer list`/`detail` `bfd-sub-state` | Explains why OpenSent is waiting | Extends: the reason derivation reads it for "waiting for BFD"; the key stays |
| Last NOTIFICATION | `peer_stats.go` `recordNotification`, `PeerInfo.LastNotif*`, `summary.go` `lastErrorString` | `last-error` and LG `last_error` | Extends: the reason derivation reads it for a session that ended by NOTIFICATION; `last-error` keeps its meaning |
| Admin hold | `Peer.adminDown`, `settings.AdminShutdown.Message` (`peer_admin.go`) | Sets lifecycle `idle-hold` | Extends: surfaced as a distinct reason, with the operator's shutdown message |
| Prefix-limit hold and timer | `holdDownAfterPrefixTeardown`, `raisePrefixHold`, `prefixReconnectDecision` (`peer_run.go`) | Sets `idle-hold` or waits `plan.Delay`; the family reaches `ze show warnings` only | Extends: the family and the wait deadline are stored on the peer so the reason can name them |
| Reconnect backoff | `Peer.run` local `delay`, `reconnectMin`/`reconnectMax` | Waits with jitter, deadline not stored | Extends: the deadline is stored so the reason can say "retry in Ns" |
| Dial error | `Session.Connect` returns the wrapped `net.OpError` | Logged at Debug, dropped | Extends: classified (refused, timeout, unreachable, other) and stored on the peer |
| FSM transition history | `peer_history.go` ring of `FSMTransition` | `show bgp peer history` | Does not apply: history stays a log of transitions; the reason is current state, not history |
| Command column registration | `cmd/peer/peer.go` `registerColumns`, `summary.go` column orders | Declares table columns per answer | Extends: the new keys are registered as columns there, not hard-coded in the dashboard |

## ExaBGP Bridge Requirements

ExaBGP 6.0, 5.0 and 4.2 compatibility is agreed (`ai/META-DECISIONS.md`,
"ExaBGP compatibility"): a script sees what ExaBGP would print. Sources read:
ExaBGP `main` (6.0, commit d6adb5ddf), `5.0` and `4.2` checkouts under
`~/Code/github.com/exa-networks/exabgp/`.

| ID | Requirement | ExaBGP source | Ze today | This spec |
|----|-------------|---------------|----------|-----------|
| E-1 | `status` lists each peer with `peer.fsm.name()`: the upper-case RFC 4271 state name | 6.0 `src/exabgp/reactor/api/command/reactor.py` `status`: `state = peer.fsm.name()`; `src/exabgp/bgp/fsm.py` `FSMState` (IDLE 0x01, ACTIVE 0x02, CONNECT 0x04, OPENSENT 0x08, OPENCONFIRM 0x10, ESTABLISHED 0x20), `FSM.name` returns `self.state.name` | `bridge_status.go` `StatusPeers` copies detail `fsm-state` (already upper case) | `fsm-state` is removed, so `StatusPeers` reads `state` and maps it back to the upper-case name through the FSM name table in `fsm/state.go`, never by upper-casing a string and never through a second list. Output unchanged |
| E-2 | `status` exists in 6.0 only (5.0 and 4.2 have no `status` command); the bridge answers it per `_answers_in_json` | 6.0 `reactor.py` `status`; absent from `5.0/src/exabgp/reactor/api/command` and `4.2/lib/exabgp/reactor/api/command` | `docs/architecture/exabgp-bridge.md`, "`status` is answered by the bridge" | Unchanged. The page's sentence "each state is the peer's `fsm-state`" is rewritten to name `state` in the same phase |
| E-3 | `fsm` event: ExaBGP writes the state entered alone, `{'state': fsm.name()}` | 6.0 `src/exabgp/reactor/api/response/json.py` `fsm`; 4.2 and 5.0 `fsm.py` `STATE.names` give the same upper-case spellings | `bridge_event_fsm.go` `fsmState` reads the `fsm` event's `to`, which ze writes from `fsm.State.String()` | Not touched: this spec changes command answers, not FSM events. AC-15b pins it |
| E-4 | `state` event values `connected`, `up`, `down` | 6.0 `response/json.py` (`connected`, `up`, `down`) | Built from ze's plugin state events (`up`/`down`) and the `connected` event, never from the lifecycle | Not touched; AC-15b pins it |
| E-5 | No ExaBGP output carries a ze lifecycle name | All of the above | True today (A-1) | The `active` → `passive` rename and `peer-state` reach no ExaBGP output; a bridge test greps the status and event outputs for the five lifecycle names |
| E-6 | ExaBGP marks an operator-disabled idle peer `DISABLED` in its neighbor data | 6.0 `src/exabgp/reactor/peer/peer.py` (`'state': 'DISABLED' if self._disable is not None and self.fsm == FSM.IDLE else self.fsm.name()`), read by `neighbor.py` `show neighbor` | The bridge refuses `show neighbor` / `peer show` as untranslated (`bridge_control_test.go`, refused table) | Out of this spec's change, but this spec makes it possible: `state` `idle` with `state-reason` `operator-shutdown` is exactly ExaBGP's DISABLED condition. Recorded as a known gap below, with the mapping the translation must use |

ExaBGP `status` does NOT print DISABLED: it prints `peer.fsm.name()` only, so
a disabled peer reads `IDLE` in `status` both in ExaBGP and through the bridge.
The bridge must not invent DISABLED there.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Operator or program runs a peer command (`show bgp`, `show bgp peer list`, `show bgp peer detail`, `show bgp health`, peer `statistics`, `capabilities`), or `monitor bgp`, the web peers page, the LG, MCP, or the ExaBGP bridge `status`, all of which dispatch one of those commands
- Prometheus scrape of the reactor metrics registry

### Transformation Path
1. Outbound dial fails in `Session.Connect`; the error returns through `runOnce` to `Peer.run`, which classifies it and stores the class, the OS error text and the time under `p.mu` on the Peer
2. `Peer.run` stores the backoff deadline (and the prefix-limit timer deadline) on the Peer before it waits, and clears it when the wait ends
3. Admin hold and prefix-limit hold store which hold applies, with the shutdown message or the family
4. `reactorAPIAdapter.Peers` builds `plugin.PeerInfo`: `FSMState` (already), lifecycle `State` (already), and a derived reason (code, detail, next retry). The derivation is ONE function in the reactor, reading Peer state in a fixed precedence
5. `cmd/peer` handlers write `state` (lower-case FSM, derived from `fsm.State` names), `peer-state` (lifecycle), `state-reason`, `reason-detail`, `next-retry` on every peer answer; `fsm-state` is removed
6. Consumers read the answer: dashboard renders State and Reason columns; web and LG colour and flag from `state`; LG writes `info` from state and reason; ExaBGP bridge reads `state` and maps it back to the FSM's own spelling
7. Reactor metrics: `ze_peer_state` and `ze_peer_state_transitions_total` keep lifecycle labels (with `passive`); new gauge `ze_peer_fsm_state` set on every FSM transition

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor ↔ cmd/peer plugin | `plugin.PeerInfo` value through `reactorAPIAdapter.Peers` (self-contained value, no pointer) | No |
| Daemon ↔ CLI, web, LG, MCP, REST, gRPC | JSON command answer (kebab-case keys) through the shared dispatcher | No |
| Daemon ↔ ExaBGP bridge (may be external) | `show bgp peer * detail` JSON answer | No |
| Daemon ↔ Prometheus | metrics registry | No |

### Integration Points
- `reactorAPIAdapter.Peers` (`reactor_api.go`) - fills the new `PeerInfo` reason fields beside `State` and `FSMState`
- `cmd/peer` `registerColumns` and the summary column orders - register `peer-state`, `state-reason` columns
- `fsm.State` name table (`fsm/state.go` `stateNames`) - the single source for the lower-case `state` spelling and the bridge's mapping back
- `peer_stats.go` `updatePeerStateMetric` and the FSM callback in `runOnce` - where the new FSM gauge is set

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | to verify at review: LG, web, MCP, dashboard read the command answer, never the reactor |
| No unintended coupling (components stay isolated) | No | to verify: `internal/component/plugin` gains only value fields, no `bgp` import |
| No duplicated functionality (extends existing, does not recreate) | No | to verify: one reason derivation in the reactor; one FSM name table |
| Zero-copy preserved where applicable (refs, not copies) | No | N-A: no wire path touched; command answers only |
| Registration over hardcoding, outbound: new columns register through `registerColumns` | No | to verify at review |
| Registration over hardcoding, inbound: the dashboard `stateRank`/`stateColor`, LG `stateClass`, web `peerFlagFromState` derive their FSM spellings from the FSM name table rather than literal lists | No | to verify: grep each for literal `opensent`/`established` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No plugin event serialises the lifecycle `PeerState`, so renaming the JSON key leaves the plugin event contract intact | Research: `appendPeerJSON` has no state key; state events carry `up`/`down` (`rpc.SessionState`) | A third-party plugin breaks silently | grep of `format/` and `pkg/plugin` for `PeerState` and the lifecycle names before implementation | unvalidated |
| A-2 | Birdwatcher clients (Alice-LG) accept FSM-style values in `state` and free text in `info` | Upstream birdwatcher copies BIRD `show protocols`, where `info` carries "Established", "Active", "Connect" | Alice-LG shows peers wrongly | Read Alice-LG source for its `state`/`info` handling; record in `birdwatcher-compat.md` | unvalidated |
| A-3 | gNMI, telemetry and doctor read no peer state value | Research grep found none | A collector silently changes meaning | grep `internal/component/{gnmi,telemetry,doctor}` for `State`, `PeerInfo`, lifecycle names | unvalidated |
| A-4 | `net.OpError` from the dialer can be classified into refused, timeout, unreachable on both Linux and macOS | `Session.Connect` wraps with `%w` | Reason reads "other" on one platform | Unit test with `syscall.ECONNREFUSED`, `ETIMEDOUT`, `EHOSTUNREACH`, `ENETUNREACH` wrapped as the dialer returns them | unvalidated |
| A-5 | A passive peer (connect false) waiting for inbound is in RFC Idle or Active depending on whether a session object exists | `Peer.SessionState` returns Idle without a session | The display contradicts the RFC for passive peers | Read `runOnce` passive path and `fsm` passive handling during DESIGN | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The ExaBGP bridge `status` changes what an ExaBGP script sees when its source key changes from upper-case `fsm-state` to lower-case `state` | `./le test functional exabgp` status case shows a changed state spelling | The bridge maps `state` back through the FSM name table; a bridge unit test pins the exact output spelling before and after |
| R-2 | A consumer still reads `state` as the lifecycle (an `established` match keeps working by coincidence, `connecting`/`idle-hold` matches silently never fire) | grep for the lifecycle names outside `peer-state` readers finds a hit | Phase 1 inventory: every reader in the research table moves in the same phase; a review grep for the five lifecycle literals |
| R-3 | The reason derivation reads Peer fields set by different goroutines and shows a stale or contradictory reason (retry deadline from a cycle that already ended) | A reason with a `next-retry` in the past, or `connect-refused` beside `established` | All reason fields written and read under `p.mu`; the deadline is cleared when the wait ends; established clears connect error; a unit test drives the sequence |
| R-4 | Reason precedence is wrong when several apply (admin hold while a connect error is still stored) | Unit table test of combinations | One fixed precedence, documented in the spec and the page: operator-shutdown, prefix-limit, bfd-wait, notification, connect error, passive-wait, none |
| R-5 | Renaming lifecycle `active` to `passive` breaks a Prometheus dashboard keyed on `to="active"` | Metric label change in the changelog | Owner approved the rename; `docs/guide/monitoring.md` states it |
| R-6 | Classifying OS dial errors differs by platform (macOS vs Linux errno) | Unit test red on darwin | Classify with `errors.Is` on the `syscall` errno set and `os.ErrDeadlineExceeded`/`net.Error.Timeout()`; unknown errors map to `connect-failed` with the OS text in `reason-detail`, never to a guessed class |
| R-7 | Golden or `.ci` expectations change (`peer-admin-shutdown.ci` matches `idle-hold` in `state`) | Functional red | Each changed expectation goes to an independent `ze-test-judge` before it lands (`ai/META-DECISIONS.md`, test changes) |
| R-8 | The LG `info` field contents change for Alice-LG users | A-2 not validated | Validate A-2 before Phase 4; record the mapping in `birdwatcher-compat.md` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing on the wire. Operators and scripts read a wrong or missing state; the ExaBGP bridge `status` line could change; LG clients could mis-colour peers |
| How is it reverted? | Single-commit reverts per phase; no config or stored state changes |
| Who else touches this path? | `spec-bgp-fsm-delayopen-and-automatic-start` (in progress, FSM and `peer_run.go`); BFD strict-mode work (`bfd-sub-state`); terminal demo validators |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show bgp peer list` with a peer whose remote address refuses TCP | → | `Peer.run` stores the dial error and deadline; `reactorAPIAdapter.Peers` derives the reason; `handleBgpPeerList` writes it | `test/plugin/peer-state-connect-refused.ci` |
| `show bgp peer detail` after `request peer disable` (operator shutdown) | → | admin hold reason; `handleBgpPeerDetail` | `test/plugin/peer-admin-shutdown.ci` (updated expectations) |
| `show bgp` summary over a peer held by a prefix limit with reconnect never | → | prefix-limit reason with family | `test/plugin/peer-state-prefix-limit-hold.ci` |
| `monitor bgp` dashboard | → | `parseDashboardSnapshot`, State and Reason columns | `TestDashboardRendersFSMStateAndReason` |
| ExaBGP bridge `status` | → | `StatusPeers` reads `state` | `TestStatusPeersReadsFSMState` and the existing `./le test functional exabgp` status case |
| Prometheus scrape | → | `ze_peer_fsm_state` set on FSM transition | `test/plugin/metrics-peer-fsm-state.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Any peer answer (`show bgp`, peer list, detail, health, statistics, capabilities) | Carries `state` with one of `idle`, `connect`, `active`, `opensent`, `openconfirm`, `established`, equal to the lower-cased `fsm.State` name of the peer's session (idle when no session) |
| AC-2 | Same answers | Carry `peer-state` with one of `stopped`, `connecting`, `passive`, `established`, `idle-hold`; no answer carries `fsm-state` any more |
| AC-3 | Outbound peer to an address where nothing listens, read between attempts | `state` is `idle`, `state-reason` is `connect-refused`, `reason-detail` carries the OS error text, `next-retry` is an RFC 3339 time in the future |
| AC-4 | Outbound dial times out / host or network unreachable | `state-reason` is `connect-timeout` / `connect-unreachable`; an unclassified error is `connect-failed` with the OS text |
| AC-5 | Peer disabled by the operator, with a shutdown message | `state` `idle`, `peer-state` `idle-hold`, `state-reason` `operator-shutdown`, `reason-detail` the message; no `next-retry` |
| AC-6 | Peer held by a prefix limit with reconnect never | `state` `idle`, `peer-state` `idle-hold`, `state-reason` `prefix-limit`, `reason-detail` names the family; distinguishable from AC-5 |
| AC-7 | Peer waiting on the prefix-limit reconnect timer | `state-reason` `prefix-limit`, `next-retry` the timer deadline |
| AC-8 | Last session ended by a NOTIFICATION and the peer is now retrying with no newer connect error | `state-reason` `notification-received` or `notification-sent`; `reason-detail` the NOTIFICATION text as `last-error` renders it |
| AC-9 | Session in OpenSent or OpenConfirm with a BFD sub-state pending | `state-reason` `bfd-wait`, `reason-detail` the sub-state name |
| AC-10 | Passive peer (connect false) waiting for the remote | `peer-state` `passive`, `state-reason` `passive-wait` |
| AC-11 | Established peer | `state-reason` `none`; no `next-retry`; a stored connect error from an earlier attempt is cleared |
| AC-12 | `monitor bgp` | Shows a State column with the FSM state and a Reason column with the reason code plus "retry Ns" when `next-retry` is set; sort and colour follow the FSM state |
| AC-13 | Web peers page and LG HTML | An established peer is flagged and coloured up; idle/connect/active/opensent/openconfirm are coloured down; no peer falls into the unknown class |
| AC-14 | LG birdwatcher API | `state` carries the FSM state; `info` carries the FSM state and reason; `last_error` keeps the NOTIFICATION meaning |
| AC-15 | ExaBGP bridge `status` / `daemon status`, text and JSON | For every peer, prints ExaBGP's `FSM.name()` spelling of the peer's FSM state (`IDLE`, `ACTIVE`, `CONNECT`, `OPENSENT`, `OPENCONFIRM`, `ESTABLISHED`), byte-identical to the answer before this spec for the same FSM state; a row without `state` still answers `error`, never a guessed state (E-1, E-2) |
| AC-15b | ExaBGP `fsm` and `state` events (`connected`, `up`, `down`) through the bridge | Byte-identical before and after this spec, in text and JSON, for API 4.2, 5.0 and 6.0 scripts (E-3, E-4) |
| AC-15c | Lifecycle rename `active` → `passive` | No ExaBGP-facing output contains a lifecycle name, before or after (E-5) |
| AC-16 | Prometheus | `ze_peer_fsm_state{peer}` reports RFC 4273 `bgpPeerState` values (idle 1, connect 2, active 3, opensent 4, openconfirm 5, established 6); `ze_peer_state` and `ze_peer_state_transitions_total` keep lifecycle labels with `passive` instead of `active`, and their help text lists idle-hold |
| AC-17 | The reason when several conditions hold | Follows the precedence operator-shutdown, prefix-limit, bfd-wait, notification, connect error, passive-wait, none |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator opens `monitor bgp` while a peer is unreachable | dial error → Peer → PeerInfo → `show bgp` → dashboard | `test/plugin/peer-state-connect-refused.ci` plus `TestDashboardRendersFSMStateAndReason` |
| 2 | Operator shuts a peer down and checks why it is idle | `request peer disable` → admin hold → reason → `show bgp peer detail` | `test/plugin/peer-admin-shutdown.ci` |
| 3 | Script polls `show bgp | json` for peers not established | `state` FSM value | `test/plugin/peer-state-connect-refused.ci` |
| 4 | Alice-LG user views the looking glass | `show bgp` → LG `transformProtocols` | `TestTransformProtocolsCarriesFSMStateAndReason` |
| 5 | ExaBGP script sends `status` | bridge → `show bgp peer * detail` → `StatusPeers` | `TestStatusPeersReadsFSMState` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPeerReasonPrecedence` | `internal/component/bgp/reactor/peer_reason_test.go` | AC-5 to AC-11, AC-17: table of combined conditions | |
| `TestDialErrorClassification` | `internal/component/bgp/reactor/peer_reason_test.go` | AC-4: refused, timeout, host/net unreachable, unknown → class and text | |
| `TestBackoffDeadlineStoredAndCleared` | `internal/component/bgp/reactor/peer_run_test.go` | AC-3, AC-7, AC-11: deadline set before the wait, cleared after; connect error cleared on established | |
| `TestPeersInfoCarriesFSMStateAndReason` | `internal/component/bgp/reactor/reactor_api_test.go` | AC-1, AC-2 at the adapter | |
| `TestPeerAnswersCarryStateKeys` | `internal/component/bgp/plugins/cmd/peer/peer_test.go` | AC-1, AC-2 on every peer handler; no `fsm-state` | |
| `TestDashboardRendersFSMStateAndReason` | `internal/component/cli/model_dashboard_test.go` | AC-12 | |
| `TestStateClassCoversEveryFSMState` | `internal/component/lg/render_test.go` | AC-13 LG, derived from the FSM name table | |
| `TestPeerFlagFromFSMState` | `internal/component/web/page_bgp_peers_test.go` | AC-13 web | |
| `TestTransformProtocolsCarriesFSMStateAndReason` | `internal/component/lg/handler_api_test.go` | AC-14 | |
| `TestStatusPeersReadsFSMState` | `internal/exabgp/bridge/bridge_status_test.go` | AC-15, E-1: every FSM state in the new `state` key prints ExaBGP's upper-case name; an operator-disabled peer prints `IDLE`, not `DISABLED`; a row without `state` answers `error` | |
| `TestStatusReplyUnchangedAcrossStateRename` | `internal/exabgp/bridge/bridge_status_test.go` | AC-15, E-1, E-2: text and JSON `status` answers for a fixed peer set are byte-identical to the answers pinned before the rename | |
| `TestBridgeOutputCarriesNoLifecycleName` | `internal/exabgp/bridge/bridge_status_test.go` | AC-15c, E-5: status and event outputs contain none of `stopped`, `connecting`, `passive`, `idle-hold` | |
| existing `fsm` / `state` event tests | `internal/exabgp/bridge/bridge_event_*_test.go` | AC-15b, E-3, E-4: run unchanged and stay green | |
| `TestPeerFSMStateGauge` | `internal/component/bgp/reactor/peer_stats_test.go` | AC-16 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `ze_peer_fsm_state` value | 1-6 | 6 | 0 (never emitted) | 7 (never emitted) |
| `next-retry` | now to now + reconnect-max (or prefix timer) | deadline equal to now | past time never shown (cleared) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `peer-state-connect-refused` | `test/plugin/peer-state-connect-refused.ci` | Peer to a closed loopback port: list and summary show `idle` or `connect`, reason `connect-refused`, a future `next-retry` | |
| `peer-admin-shutdown` | `test/plugin/peer-admin-shutdown.ci` | Updated: `peer-state idle-hold`, `state idle`, `state-reason operator-shutdown` | |
| `peer-state-prefix-limit-hold` | `test/plugin/peer-state-prefix-limit-hold.ci` | Prefix limit with reconnect never: reason `prefix-limit` with the family | |
| `metrics-peer-fsm-state` | `test/plugin/metrics-peer-fsm-state.ci` | Scrape shows `ze_peer_fsm_state` 6 once established | |
| `metrics-session-lifecycle` | `test/plugin/metrics-session-lifecycle.ci` | Updated if it names `active` | |
| ExaBGP suite | `./le test functional exabgp` | Every ExaBGP compatibility case, including the `status` answer and `fsm`/`state` events, passes unchanged (AC-15, AC-15b) | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | No wire-visible change: only what Ze reports about its own state. Interop rule exemption: no protocol behaviour changes | |

## Files to Modify
- `internal/component/bgp/reactor/peer.go` - lifecycle name `active` → `passive`; reason fields on Peer
- `internal/component/bgp/reactor/peer_run.go` - store dial error class, backoff and prefix-timer deadlines; clear on established
- `internal/component/bgp/reactor/peer_admin.go` - record operator-shutdown reason
- `internal/component/bgp/reactor/reactor_api.go` - fill PeerInfo reason fields
- `internal/component/bgp/reactor/peer_stats.go`, `reactor_metrics.go` - FSM gauge; lifecycle label names; help text
- `internal/component/plugin/types_bgp.go` - PeerInfo reason fields; lifecycle `passive` name
- `internal/component/bgp/fsm/state.go` - expose the lower-case name and the RFC 4273 number from the one table, if not already derivable
- `internal/component/bgp/plugins/cmd/peer/peer.go`, `summary.go`, `health.go` - `state`, `peer-state`, `state-reason`, `reason-detail`, `next-retry`; drop `fsm-state`; columns
- `internal/component/cli/model_dashboard.go`, `model_dashboard_sort.go`, `model_dashboard_render.go` - State and Reason columns
- `internal/component/lg/handler_api.go`, `render.go`, `handler_ui.go` - birdwatcher fields and colouring
- `internal/component/web/page_bgp_peers.go`, `page_bgp_summary.go` - flag and colour from FSM state
- `internal/component/mcp/ui/bgp-peer/app.js` - CSS class from FSM state
- `internal/exabgp/bridge/bridge_status.go` - read `state`, map back to the FSM spelling
- `internal/le/interoplab/bgp/helper.go`, `names.go`; `internal/le/site/terminaldemo/*` validators; `internal/test/fixture/*` readers of `state`/`fsm-state`
- `test/plugin/peer-admin-shutdown.ci`, `test/plugin/metrics-session-lifecycle.ci`
- Docs: `docs/architecture/behavior/peer-lifecycle.md`, `docs/architecture/behavior/fsm.md`, `docs/architecture/api/birdwatcher-compat.md`, `docs/architecture/api/commands.md`, `docs/architecture/exabgp-bridge.md`, `docs/guide/monitoring.md`, `docs/guide/bgp-peering.md`, `docs/guide/configuration.md`, `docs/guide/looking-glass-howto.md`, `docs/features/cli-commands.md`, `docs/features/configuration.md`

## Files to Create
- `internal/component/bgp/reactor/peer_reason.go` - the one reason derivation and dial error classification
- `internal/component/bgp/reactor/peer_reason_test.go`
- `test/plugin/peer-state-connect-refused.ci`
- `test/plugin/peer-state-prefix-limit-hold.ci`
- `test/plugin/metrics-peer-fsm-state.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes, if the peer answers are YANG-modelled | Check the `cmd/peer` command YANG output nodes for `state` and `fsm-state`; add `peer-state`, `state-reason`, `reason-detail`, `next-retry` and remove `fsm-state` there |
| YANG validation constraints | Yes, if modelled | `state` and `state-reason` as enumerations |
| YANG custom validators | N-A | No new input |
| CLI commands/flags | No | Existing commands; answers change |
| CLI grammar (keyword before value) | N-A | No grammar change |
| Editor autocomplete | N-A | No new input |
| Functional test for new RPC/API | Yes | `test/plugin/peer-state-connect-refused.ci`, `peer-state-prefix-limit-hold.ci` |
| Pipe completeness | Yes | Answers stay data through `ApplyPipes`; table, json, yaml all show the new keys |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No runtime dependency added |
| Prometheus counters/metrics | Yes | New `ze_peer_fsm_state{peer}` gauge; `ze_peer_state` and `ze_peer_state_transitions_total` label value `active` → `passive`, help text fixed |
| BGP family surface (new SAFI / capability / attribute) | N-A | No family, capability or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | The feature page that covers BGP peer monitoring, found with `./le feature report` |
| 2 | Config syntax changed? | No | No config change; `docs/guide/configuration.md` and `docs/features/configuration.md` only change where they say "state reads idle-hold" |
| 3 | CLI command added/changed? | Yes | `docs/features/cli-commands.md` (dashboard columns and colours) |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` (example output), `docs/architecture/api/birdwatcher-compat.md` (`state`, `info`) |
| 5 | Plugin added/changed? | No | No plugin registration change |
| 6 | Has a user guide page? | Yes | `docs/guide/bgp-peering.md`, `docs/guide/monitoring.md`, `docs/guide/looking-glass-howto.md` |
| 7 | Wire format changed? | No | Nothing on the wire |
| 8 | Plugin SDK/protocol changed? | No | Plugin events untouched (A-1); `docs/architecture/api/process-protocol.md` named in row 16 as unaffected once A-1 holds |
| 9 | RFC behavior implemented, changed, or newly proven? | No | Display only; state names quote RFC 4271 Section 8.2.2 and RFC 4273 `bgpPeerState` |
| 10 | Test infrastructure changed? | No | Fixtures follow the key rename; no runner change |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` if it compares peer state display with FRR/BIRD |
| 12 | Internal architecture changed? | Yes | `docs/architecture/behavior/peer-lifecycle.md`, `docs/architecture/behavior/fsm.md` |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | Yes | `docs/guide/monitoring.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Commands keep their names |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Affected: `peer-lifecycle.md`, `fsm.md`, `birdwatcher-compat.md`, `commands.md`, `exabgp-bridge.md`, `docs/architecture/web-interface.md` (LG and peer page state colouring). Checked and expected unaffected, to be confirmed at implementation: `docs/architecture/api/architecture.md` (engine/plugin split, no state keys), `docs/architecture/api/process-protocol.md` (events carry up/down only, A-1), `docs/architecture/core-design.md` (no state key), `docs/architecture/testing/interop.md` (no scenario change), `docs/architecture/web-workbench-pages.md` (unless it describes the peer flag). Run `./le spec citation anchors spec plan/release/fixes/spec-bgp-peer-state-fsm-and-reason.md` at implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/architecture/api/commands.md`, `docs/guide/looking-glass-howto.md` example outputs show `"state": "established"`; regenerate from a run |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- PeerInfo gains the reason fields (zero reason `none`), `cmd/peer` answers write `state` (FSM), `peer-state`, `state-reason`, `reason-detail`, `next-retry` and drop `fsm-state`; every reader in the research table moves to the new keys in the same phase (dashboard, web, LG, MCP app, ExaBGP bridge, interop helper, demo validators, fixtures, `.ci`). Validate A-1, A-3 first
   - Tests: `TestPeersInfoCarriesFSMStateAndReason`, `TestPeerAnswersCarryStateKeys`, `TestStatusPeersReadsFSMState`, `peer-state-connect-refused` (fails: reason still `none`)
   - Verify: no reader of the lifecycle under `state` remains (grep for the five lifecycle literals)
2. **Phase: Reason recording** -- dial error classification, backoff and prefix-timer deadlines, admin and prefix holds on the Peer, the one derivation with its precedence; lifecycle `active` → `passive`
   - Tests: `TestPeerReasonPrecedence`, `TestDialErrorClassification`, `TestBackoffDeadlineStoredAndCleared`, `peer-state-connect-refused`, `peer-admin-shutdown`, `peer-state-prefix-limit-hold`
   - Verify: Validate A-4 and A-5 here
3. **Phase: Views** -- dashboard State and Reason columns with "retry Ns"; web flag; LG colour, `info`; MCP class; colour and sort derived from the FSM name table. Validate A-2 before the LG change
   - Tests: `TestDashboardRendersFSMStateAndReason`, `TestStateClassCoversEveryFSMState`, `TestPeerFlagFromFSMState`, `TestTransformProtocolsCarriesFSMStateAndReason`
4. **Phase: Metrics** -- `ze_peer_fsm_state` gauge with RFC 4273 values; lifecycle label `passive`; help text
   - Tests: `TestPeerFSMStateGauge`, `metrics-peer-fsm-state`, `metrics-session-lifecycle`
5. **Phase: Docs** -- each phase edits the pages its change makes wrong (Documentation Update Checklist); this phase only checks with `./le doc check all` and regenerates the example outputs. The showcase recording on the website is re-recorded so `monitor bgp` shows `Idle  connect-refused` for 127.0.0.4

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | `state` always equals the session FSM, never the lifecycle; a past `next-retry` is never shown; precedence matches AC-17 |
| Naming | Keys kebab-case: `state`, `peer-state`, `state-reason`, `reason-detail`, `next-retry`; reason codes kebab-case |
| Data flow | Reason derived once in the reactor; views only render |
| Rule: no-layering | `fsm-state` removed, not kept beside `state`; no consumer reads both |
| Rule: declared once | FSM spellings (lower case, RFC 4273 numbers, ExaBGP spelling) all derive from `fsm/state.go` |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| FSM `state` on every peer answer | `TestPeerAnswersCarryStateKeys` |
| Reason recorded and derived | `TestPeerReasonPrecedence`, `peer-state-connect-refused.ci` |
| No `fsm-state` key left | grep `fsm-state` in `internal/`, `test/`, `docs/` returns only history notes |
| FSM gauge | `metrics-peer-fsm-state.ci` |
| Showcase shows the reason | re-recorded cast shows `connect-refused` for 127.0.0.4 |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | No new input |
| Information leakage | `reason-detail` carries OS error text and the operator's shutdown message; both are already visible to the same readers (logs, config). The public LG `info` field carries only the reason code, not the detail, unless `birdwatcher-compat.md` decides otherwise |
| Resource exhaustion | Stored error text is bounded (truncate to a fixed length) |

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

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| `state` is the FSM state, lifecycle moves to `peer-state` (owner) | Keep `state` as lifecycle and add a column | Operators and scripts read `state` as the RFC state (FRR `state`, BIRD info column) |
| Structured reason: `state-reason` code plus `reason-detail` and `next-retry` (owner chose A) | B: one BIRD-style free-text `info`; C: each view derives its own | Scripts match stable codes; one derivation; FRR `peerState` precedent |
| Lifecycle `active` renamed `passive` (owner) | Keep `active` | Clashes with RFC Active; FRR and BIRD say Passive |
| New gauge `ze_peer_fsm_state` with RFC 4273 values (owner) | Keep metrics lifecycle only; switch the existing gauge | Lifecycle dashboards keep working; FSM visible to Prometheus with standard MIB numbering |
| Connect error gets its own field, `last-error` unchanged | Reuse `last-error` | `last-error` is the NOTIFICATION under the normative LG contract |
| Reason derived in the reactor, next to both states | Derive in `cmd/peer` | The reactor holds the Peer fields under `p.mu`; the answer must be a self-contained value |

## Known Limitations
- ExaBGP `show neighbor` / `peer show` is not translated by the bridge (refused today). When it is, its `state` column must print `DISABLED` exactly when this spec's answer has `state` `idle` and `state-reason` `operator-shutdown`, and ExaBGP's `FSM.name()` spelling otherwise (E-6). That translation is not in this spec; it needs a home in an ExaBGP bridge spec (to be decided with Thomas)
- No FSM metric beyond the gauge (no FSM transition counter); the transition history stays in `show bgp peer history`
- The reason reports only the current cause; earlier causes stay in history and logs

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.

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
- [ ] The targeted tests for what changed pass: the scoped package tests (`./le job run label <label> command go test -tags ze_bgp <package>`), `./le test functional plugin`, `./le test functional ui`, and `./le verify changed` over the closing commits
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
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
