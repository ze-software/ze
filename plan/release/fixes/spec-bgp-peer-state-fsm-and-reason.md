# Spec: bgp-peer-state-fsm-and-reason

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | cli |
| Priority | P1 |
| Release reason | The peer state an operator reads today is Ze's lifecycle name, not the RFC 4271 FSM state, so a peer sitting in Idle between connect retries reads "connecting" and Ze's "active" means the opposite of the RFC's Active. Operators used to other BGP implementations, which show the RFC state, misread every down peer. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

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
"changes its state to Idle"). The lab peer at 172.29.0.2 later in the same recording showed
`state connecting` beside `fsm-state IDLE`.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/behavior/fsm.md` - the FSM states, events and BFD sub-states
  → Constraint: FSM names come from one ungated core leaf (`internal/core/bgp/fsmstate`, parallel to `internal/core/bgp/msgtype`) consumed by `internal/component/bgp/fsm/state.go` and every always-on or separately gated reader (`cli`, `lg`, `web`, `exabgp/bridge`): upper-case (IDLE, CONNECT, ACTIVE, OPENSENT, OPENCONFIRM, ESTABLISHED), lower-case JSON spelling, and RFC 4273 `bgpPeerState` 1..6 are all derived from that one table, never a second list (`ai/rules/principles.md`, declared once) and never by importing gated `internal/component/bgp/fsm` across `ze_bgp`
  → Decision: BFD sub-state is already shown as `bfd-sub-state` by `show bgp peer list` and `detail`; the reason column extends that precedent of "FSM state plus why"
- [ ] `docs/architecture/behavior/peer-lifecycle.md` - lifecycle versus FSM table (lines 30-50)
  → Constraint: this page states the two-level model; it becomes wrong the moment `state` changes meaning, so it is edited in the same phase
- [ ] `docs/architecture/api/birdwatcher-compat.md` - LG JSON contract
  → Constraint: `state` and `last_error` MUST be present; `last_error` names the NOTIFICATION that ended the session. A connect error MUST NOT be written into `last_error` without changing that normative page
- [ ] `ai/doors/system.md`, `bgp-engine.md`, `commands-cli.md`, `api-surfaces.md`, `plugin-infra.md`, `bgp-plugins.md`, `system-plugins.md`, `config-yang.md`, `le-tooling.md`
  → Constraint: a JSON key consumed by scripts, the LG, MCP and the ExaBGP bridge is a contract; it changes on purpose, with its page, and every consumer moves in the same change (no-layering: no period where both meanings exist)
  → Constraint: the LG and web hold no peer import; they read the `show bgp` command answer, so the new fields must be in that answer, not computed in the adapter
  → Constraint: the ExaBGP bridge can run as an external process; it reads peer state only through the `show bgp peer detail` command answer, so it follows the key rename and nothing else

### RFC Summaries
- [ ] `rfc/short/rfc4271.md` - Section 8.2.2 state names (display-only reference; `Scope: cli`, no wire change)
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
- [ ] `internal/component/config/yang/modules/ze-types.yang` - `grouping peer-info` already describes `leaf state` as lower-case RFC 4271 FSM state; lacks `peer-state`, `state-reason`, `reason-detail`, `next-retry`
- [ ] `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` - `show bgp peer detail` description currently describes `'state'` as lifecycle and `'fsm-state'` as RFC 4271 state
- [ ] `internal/le/interoplab/bgp/check_special.go`, `check_delayopen.go`, `helper.go` - `zePeerState` reads `state` from `show bgp peer list` as the local admission barrier (`PeerStateEstablished` published after encoding contexts and forward facts) and `check_delayopen.go` checks `idle-hold`; moving `zePeerState` to `peer-state` preserves both

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
| Session FSM state | `fsm/state.go` `State.String`; `PeerInfo.FSMState`; `peer detail` `fsm-state` | RFC state, shown only in detail, upper case | Extends: lifts the pure six-state RFC 4271/4273 vocabulary into ungated `internal/core/bgp/fsmstate` (parallel to `internal/core/bgp/msgtype`) so `fsm/state.go` and always-on/separately gated consumers (`cli`, `lg`, `web`, `exabgp/bridge`) share one table without importing `ze_bgp`; becomes `state` on every answer and removes `fsm-state` (no-layering) |
| BFD sub-state display | `Peer.bfdSubState`, `PeerInfo.BFDSubState`, `peer list`/`detail` `bfd-sub-state` | Explains why OpenSent is waiting | Extends: the reason derivation reads it for "waiting for BFD"; the key stays |
| Last NOTIFICATION | `peer_stats.go` `recordNotification`, `PeerInfo.LastNotif*`, `summary.go` `lastErrorString` | `last-error` and LG `last_error` | Extends: the reason derivation reads it for a session that ended by NOTIFICATION; `last-error` keeps its meaning |
| Admin hold | `Peer.adminDown`, `settings.AdminShutdown.Message` (`peer_admin.go`) | Sets lifecycle `idle-hold` | Extends: surfaced as a distinct reason, with the operator's shutdown message |
| Prefix-limit hold and timer | `holdDownAfterPrefixTeardown`, `raisePrefixHold`, `prefixReconnectDecision` (`peer_run.go`) | Sets `idle-hold` or waits `plan.Delay`; the family reaches `ze show warnings` only | Extends: the family and the wait deadline are stored on the peer so the reason can name them |
| Reconnect backoff | `Peer.run` local `delay`, `reconnectMin`/`reconnectMax` | Waits with jitter, deadline not stored | Extends: the deadline is stored so the reason can say "retry in Ns" |
| Dial error | `Session.Connect` returns the wrapped `net.OpError` | Logged at Debug, dropped | Extends: classified (refused, timeout, unreachable, other) and stored on the peer |
| FSM transition history | `peer_history.go` ring of `FSMTransition` | `show bgp peer history` | Does not apply: history stays a log of transitions; the reason is current state, not history |
| Command column registration | `cmd/peer/peer.go` `registerColumns`, `summary.go` column orders | Declares table columns per answer | Extends: the new keys are registered as columns there, not hard-coded in the dashboard |
| YANG peer-info grouping | `internal/component/config/yang/modules/ze-types.yang` `grouping peer-info`; `cmd/peer/yang/ze-peer-cmd.yang` | Models `state` as lower-case RFC 4271 FSM state on `peer-list`, `peer-show`, `overview`; `ze-peer-cmd.yang` prose names `'state'` lifecycle and `'fsm-state'` | Extends: adds `peer-state`, `state-reason`, `reason-detail`, `next-retry` to `grouping peer-info` and updates `ze-peer-cmd.yang` prose |
| Prometheus peer-state metrics | `internal/component/bgp/reactor/reactor_metrics.go` (`ze_peer_state`, `ze_peer_state_transitions_total`), `internal/component/bgp/reactor/peer_stats.go` (`peerStateNames`, `updatePeerStateMetric`), backed by `docs/architecture/decisions/001-pull-model-metrics.md` | Push-model `metrics.Registry` lifecycle gauge and transition counter labelled `stopped`, `connecting`, `active`, `established`, `idle-hold` (help text omits `idle-hold`) | Extends: keeps both lifecycle metrics on the push-model registry, renames label `active` → `passive`, fixes help text to list `idle-hold`, and adds push-model gauge `ze_peer_fsm_state{peer}` reporting RFC 4273 `bgpPeerState` 1..6 on every FSM transition |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | The core knows nothing, and everything registers | the door every change reads | Always-on code MUST NOT import a gated package. | agrees | Consumers outside `ze_bgp` (`cli`, `lg`, `web`, `exabgp/bridge`) import only the ungated pure-stdlib leaf `internal/core/bgp/fsmstate`, never gated `internal/component/bgp/fsm` |
| `ai/doors/bgp-engine.md` | Where it sits | the door covering `reactor/*.go`, `fsm/state.go`, `internal/core/bgp/fsmstate/*` | Every package under `internal/component/bgp` is gated by `ze_bgp` through the generated `internal/component/plugin/all/all_ze_bgp.go`. | agrees | Lifting the pure six-state RFC 4271/4273 vocabulary into `internal/core/bgp/fsmstate` (parallel to `internal/core/bgp/msgtype`) keeps `internal/component/bgp` compile-out clean |
| `ai/doors/plugin-infra.md` | Where it sits | the door covering `internal/component/plugin/types_bgp.go` | Outside the generated `all/` files, no package under `internal/component/plugin/` imports `internal/component/bgp/` | agrees | `plugin.PeerInfo` gains only plain string and `time.Time` value fields (`StateReason`, `ReasonDetail`, `NextRetry`) and renames `PeerStateActive` to `PeerStatePassive`, with zero `bgp` import |
| `ai/doors/bgp-plugins.md` | Boundaries | the door covering `internal/component/bgp/plugins/cmd/peer/*` | Reach the engine through the bus, DirectBridge or a text command, never a plain call into `reactor` or another component. | agrees | `cmd/peer` reads `PeerInfo` through the existing `reactorAPIAdapter.Peers` boundary and does not call into `Peer` directly |
| `ai/doors/config-yang.md` | Where it sits | the door covering `internal/component/config/yang/modules/ze-types.yang` | It holds zero BGP imports: the dependency runs from the generic loader into `bgp/config` | agrees | `ze-types.yang` adds pure YANG output leaves to `grouping peer-info` with no Go import change |
| `ai/doors/commands-cli.md` | Boundaries | the door covering `internal/component/cli/model_dashboard*.go` | A handler answers data, never text. | agrees | `cmd/peer` emits structured fields (`state`, `peer-state`, `state-reason`, `reason-detail`, `next-retry`) through `ApplyPipes`; the dashboard formats "retry Ns" only at render time |
| `ai/doors/api-surfaces.md` | Boundaries | the door covering `internal/component/lg/*`, `internal/component/web/*` | **Never import a protocol package to read state.** Ask the dispatcher. The LG and the web UI hold no RIB or peer import | agrees | LG and web continue querying `show bgp` through `CommandDispatcher` and import only the ungated `internal/core/bgp/fsmstate` vocabulary leaf, matching `internal/core/bgp/asn` |
| `ai/doors/system-plugins.md` | Boundaries | the door covering `internal/exabgp/bridge/bridge_status.go` | **Always-on code never imports a gated plugin.** Extract a shared helper to an `internal/core/` leaf first | agrees | `bridge_status.go` maps `state` back to ExaBGP's upper-case FSM spelling through the ungated `internal/core/bgp/fsmstate` leaf, never importing gated `internal/component/bgp/fsm` |
| `ai/doors/le-tooling.md` | Where it sits | the door covering `internal/le/interoplab/bgp/*`, `test/plugin/*.ci` | A shipped `ze` never links an `internal/le` package. | agrees | Interop helper updates (`check_special.go`, `check_delayopen.go`, `helper.go`, `names.go`) and `.ci` tests stay strictly under `internal/le/` and `test/` |
| `docs/architecture/behavior/peer-lifecycle.md` | Peer states (distinct from FSM states) | anchored to `reactor/peer.go`, `peer_run.go`, `reactor_api.go`, `peer_stats.go` | `Peer` has its own state enum that is coarser than `fsm.State` and reflects operator-visible peer status, not protocol detail. | departs | Owner approved showing RFC 4271 FSM state under `state`, moving lifecycle `PeerState` to `peer-state`, and renaming `PeerStateActive` (`active`) to `PeerStatePassive` (`passive`). This page is updated in the same phase |
| `docs/architecture/behavior/fsm.md` | TL;DR (Read This First) | `// Design:` page of `fsm/state.go`; anchored to `reactor/*`, `cmd/peer/peer.go` | IDLE→ACTIVE→CONNECT→OPENSENT→OPENCONFIRM→ESTABLISHED | agrees | Every peer answer's `state` uses these exact six RFC 4271 Section 8.2.2 states derived from one table (`internal/core/bgp/fsmstate` delegated by `fsm/state.go`); the page is updated only to document the derived lower-case spelling and RFC 4273 numbering |
| `docs/architecture/api/birdwatcher-compat.md` | 5. Protocol object | anchored to `cmd/peer/summary.go`, `lg/handler_api.go`, `lg/handler_ui.go` | The value names the NOTIFICATION code and subcode that ended the session, for example "Cease/BFD Down". | agrees | `last_error` keeps its NOTIFICATION-only meaning (connect errors never enter it), `state` stays present carrying FSM state, and adding `info` (FSM state + reason code only, never `reason-detail`) is permitted by Section 4 and recorded in Section 5 |
| `docs/architecture/api/commands.md` | Command Verb Taxonomy | `// Design:` page of `cmd/peer/*`, `cli/model_dashboard*.go`; anchored to `reactor/*`, `ze-peer-cmd.yang` | `monitor bgp` polls summary data every second and renders a dashboard. | agrees | `monitor bgp` keeps polling `show bgp` data and renders the new State and Reason columns from registered command columns; page example outputs are updated in the same phase |
| `docs/architecture/exabgp-bridge.md` | `status` is answered by the bridge | `// Design:` page and anchor of `internal/exabgp/bridge/bridge_status.go` | each state is the peer's `fsm-state`, the RFC 4271 Section 8.2.2 state as ExaBGP prints it (`IDLE`, `ACTIVE`, `CONNECT`, `OPENSENT`, `OPENCONFIRM`, `ESTABLISHED`). | departs | `fsm-state` is removed (no-layering) and `StatusPeers` reads `state` instead, mapping it back to ExaBGP's upper-case spelling via `internal/core/bgp/fsmstate` so the ExaBGP-facing output is byte-identical. This page is updated in the same phase |
| `docs/architecture/testing/interop.md` | GoBGP export-reload successor proof | `// Design:` page and anchor of `interoplab/bgp/check_special.go`, `check_delayopen.go`, `helper.go`, `names.go` | Ze's peer-list producer `handleBgpPeerList` serializes `reactorAPIAdapter.Peers`'s `Peer.State()`, not the raw session FSM state. | departs | `handleBgpPeerList` now serializes `Peer.State()` under `peer-state` (and FSM state under `state`), and `zePeerState` reads `peer-state` so `waitZePeerState` preserves the local admission barrier after encoding contexts and forwarding facts publish. This page is updated in the same phase |
| `docs/architecture/web-interface.md` | LG Data Access | `// Design:` page and anchor of `lg/handler_api.go`, `render.go`, `handler_ui.go` | All BGP data is queried via `CommandDispatcher` (same `func(string) (string, error)` as the web UI's admin handlers). The LG never imports RIB or peer plugin packages. | agrees | LG reads `state`, `state-reason`, and `last-error` strictly from `show bgp` via `CommandDispatcher` and updates state colouring through `internal/core/bgp/fsmstate` |
| `docs/architecture/web-workbench-pages.md` | BGP | `// Design:` page and anchor of `web/page_bgp_peers.go`, `page_bgp_summary.go` | When reactor state is wired in, that function changes to FSM state and most peers stop being green. | agrees | `peerFlagFromState` now switches on FSM states from `internal/core/bgp/fsmstate`, matching this page's stated intent |
| `docs/architecture/core-design.md` | 19. Component Boundaries | `// Design:` page of `reactor/peer.go`, `peer_run.go`, `peer_admin.go`, `reactor_api.go`, `reactor_metrics.go`; anchored to `peer_admin.go`, `peer_stats.go` | Each component under `internal/component/` is independently removable. Cross-component coupling follows a strict hierarchy: | agrees | No component gains a forbidden import; cross-surface FSM state names live in `internal/core/bgp/fsmstate` and peer answers stay on the existing command dispatcher boundary |
| `docs/architecture/api/architecture.md` | RIB Ownership | `// Design:` page of `reactor/peer_stats.go`; anchored to `reactor/*`, `plugin/types_bgp.go` | **API programs own all RIB data and logic.** The engine is a minimal BGP speaker. | agrees | Only session/peer display state and failure reasons owned by the reactor are surfaced; no RIB or policy logic enters the engine |
| `docs/architecture/api/process-protocol.md` | Overview | `// Design:` page and anchor of `plugin/types_bgp.go`; anchored to `reactor/*` | The protocol is the same for all invocation modes (internal goroutine, external subprocess). | agrees | Plugin events (`up`/`down` and FSM `from`/`to`) are untouched (A-1 confirmed), and external plugins receive the updated command answers identically over RPC |
| `docs/architecture/api/json-format.md` | Field naming | anchored to `reactor/peer.go` | JSON keys are lowercase kebab-case, never camelCase and never snake_case. | agrees | All new and updated command keys (`state`, `peer-state`, `state-reason`, `reason-detail`, `next-retry`) and reason codes are lowercase kebab-case; LG keeps its documented `snake_case` exception |
| `docs/architecture/decisions/001-pull-model-metrics.md` | Consequences | anchored to `reactor/reactor_metrics.go` | New Ze Prometheus metrics should follow the registry-backed push model unless this decision is explicitly reopened. | agrees | New gauge `ze_peer_fsm_state` registers on the existing `metrics.Registry` and is pushed from the FSM transition callback in `peer_run.go` / `peer_stats.go` |
| `docs/architecture/config/yang-config-design.md` | Type Library: `ze-types.yang` | anchored to `internal/component/config/yang/modules/ze-types.yang` | Defines reusable types and groupings imported by other modules. Contains no tree nodes (no containers, lists, or leaves). | agrees | New peer output leaves are added inside the existing reusable `grouping peer-info` in `ze-types.yang` with no top-level tree nodes |
| `docs/architecture/cli/command-verbs.md` | The axis: does the command change the system | anchored to `ze-peer-cmd.yang`, `ze-types.yang` | `show`, `monitor` and `resolve` change nothing. | agrees | All updated peer views (`show bgp`, `show bgp peer *`, `show bgp health`, `monitor bgp`) remain strictly read-only snapshots/polls |
| `docs/architecture/api/capability-contract.md` | Implementation Status | anchored to `reactor/peer.go` | - | silent | Governs OPEN capability injection via `rpc.CapabilityDecl`; this spec does not change capability negotiation or wire OPENs |
| `docs/architecture/api/ipc_protocol.md` | Overview | anchored to `reactor/peer.go` | - | silent | Governs IPC request/answer wire framing; command answers keep the same framing and change only peer-row payload fields |
| `docs/architecture/api/update-syntax.md` | Status | anchored to `reactor/peer.go` | - | silent | Governs `send bgp ... update text` route syntax; this spec touches no route announcement grammar |
| `docs/architecture/behavior/fsm-open-sent.md` | TL;DR | anchored to `reactor/peer.go`, `peer_run.go`, `peer_stats.go`, `fsm/state.go` | - | silent | Per-state runbook for OpenSent wire transitions and timers; this spec alters no OpenSent transition, only displays its existing state and BFD wait reason |
| `docs/architecture/bgp/structural-forwarding.md` | Four costs, and what replaced them | anchored to `reactor/peer.go` | - | silent | Governs structural UPDATE forwarding across peers; this spec makes no forwarding or wire change |
| `docs/architecture/config/syntax.md` | TL;DR (Read This First) | anchored to `reactor/peer.go` | - | silent | Governs configuration file syntax; this spec adds, removes, or renames no configuration statement |
| `docs/architecture/encoding-context.md` | TL;DR (Read This First) | anchored to `reactor/peer.go` | - | silent | Governs per-peer `EncodingContext` and `ContextID` zero-copy forwarding; untouched by peer state display |
| `docs/architecture/forward-congestion-pool.md` | Invariant | anchored to `reactor/peer.go` | - | silent | Governs outbound UPDATE congestion pooling and backpressure; untouched by peer state display |
| `docs/architecture/meta/role.md` | Keys Set | anchored to `reactor/peer.go` | - | silent | Governs RFC 9234 BGP role route metadata; untouched by peer state display |
| `docs/architecture/plugin/plugin-system.md` | Where each layer lives | anchored to `reactor/peer.go`, `peer_run.go` | - | silent | Governs plugin registration, startup stages, and exclusive claims; this spec adds no plugin, stage, or claim and adds only plain value fields to `PeerInfo` |
| `docs/architecture/plugin/rib-storage-design.md` | Overview | anchored to `reactor/peer.go`, `peer_run.go` | - | silent | Governs `bgp-rib` route storage and best-path election; untouched by peer state display |
| `docs/architecture/testing/ci-format.md` | Syntax Overview | anchored to `reactor/peer.go` | - | silent | Governs `.ci` runner directives and syntax; this spec adds standard `.ci` tests without changing the format |
| `docs/architecture/update-building.md` | TL;DR (Read This First) | anchored to `reactor/peer.go` | - | silent | Governs building outbound BGP UPDATE wire packets; this spec makes no wire change |
| `docs/architecture/wire/capabilities.md` | TL;DR (Read This First) | anchored to `reactor/peer.go` | - | silent | Governs BGP capability wire encoding and decoding; untouched by peer state display |
| `docs/architecture/behavior/fsm-active.md` | TL;DR | anchored to `reactor/peer_run.go`, `fsm/state.go` | - | silent | Per-state runbook for Active wire transitions and timers; this spec alters no Active transition, only reports the existing FSM Active state and `passive-wait` reason |
| `docs/architecture/behavior/fsm-connect.md` | TL;DR | anchored to `reactor/peer_run.go`, `fsm/state.go` | - | silent | Per-state runbook for Connect wire transitions and DelayOpen; this spec alters no Connect transition, only records dial error class and retry deadline after failure |
| `docs/architecture/behavior/fsm-established.md` | TL;DR | anchored to `reactor/peer_run.go`, `fsm/state.go` | - | silent | Per-state runbook for Established wire behavior; this spec alters no Established transition, only clears stored dial error and reports reason `none` |
| `docs/architecture/behavior/fsm-idle.md` | TL;DR | anchored to `reactor/peer_run.go`, `fsm/state.go` | - | silent | Per-state runbook for Idle wire transitions; this spec alters no Idle transition, only surfaces why a peer is in Idle via `state-reason` |
| `docs/architecture/bgp/protocol-event-capture.md` | Enabling it | anchored to `reactor/peer_run.go` | - | silent | Governs raw protocol event capture; this spec changes no wire packet or captured protocol event |
| `docs/architecture/diagnostics/event-history-rings.md` | The decisions | anchored to `reactor/peer_run.go` | **BGP history records `PeerState`, not the internal `fsm.State`.** `PeerState` is the state an operator observes, and it carries fewer values than the FSM's own. The transition is appended where the peer's state actually changes. | departs | The ring stays a record of `PeerState` transitions (with `PeerStatePassive` replacing `PeerStateActive`), but `PeerState` is no longer the primary operator-visible `state`: `state` becomes RFC 4271 `fsm.State` while `PeerState` moves to `peer-state`. This page is updated in the same phase so its rationale names `peer-state` rather than claiming `PeerState` is "the state an operator observes" |
| `docs/architecture/edge-cases/extended-message.md` | TL;DR | anchored to `reactor/peer_run.go` | - | silent | Governs RFC 8654 extended (>4096-byte) message negotiation; untouched by peer state display |
| `docs/architecture/memory/lifetime-contracts.md` | TL;DR | anchored to `reactor/peer_run.go` | - | silent | Governs borrowed wire buffer ownership and pooling lifetimes; this spec touches no wire buffers, storing only bounded error strings under `p.mu` |
| `docs/architecture/mrt.md` | Packages | anchored to `reactor/peer_run.go` | - | silent | Governs RFC 6396 MRT dump and stream recording; untouched by peer state display |
| `docs/architecture/api/text-format.md` | Current Format | anchored to `reactor/reactor_api.go` | - | silent | Governs legacy text event line format; A-1 confirmed no event serializes lifecycle `PeerState` |
| `docs/architecture/config/transaction-protocol.md` | 1. Design Principles | anchored to `reactor/reactor_api.go` | - | silent | Governs config verify/apply/rollback transactions; this spec adds no config transaction participant or root |
| `docs/architecture/wire/nlri-bgpls.md` | MP_REACH next hop | anchored to `reactor/reactor_api.go` | - | silent | Governs BGP-LS NLRI wire encoding; untouched by peer state display |
| `docs/architecture/behavior/fsm-open-confirm.md` | TL;DR | anchored to `fsm/state.go` | - | silent | Per-state runbook for OpenConfirm wire transitions; this spec alters no OpenConfirm transition, only displays its existing state and any BFD wait reason |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now. | agrees | Peer state and reason display touches neither VPP nor kernel dataplane backend validation |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each. | agrees | Peer state display adds no configuration leaf and does not alter config validation verdicts |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation. | agrees | Unrelated firewall backend verification; untouched by peer state display |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it". | agrees | Data-plane backend naming is untouched by BGP peer state reporting |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it." | agrees | Peer state display touches no export filter chain or next-hop rewriting |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs"). | agrees | Policy chain next-hop authoring is untouched by peer state display |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)"). | agrees | Export filter compilation is untouched by peer state display |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self"). | agrees | Export next-hop folding and raw-override provenance are untouched by peer state display |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load"). | agrees | Import/export next-hop validation is untouched by peer state display |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes"). | agrees | Forwarded vs originated next-hop handling is untouched by peer state display |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?". | agrees | Implied-filter publishing is untouched by peer state display |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it. | agrees | Peer state and reason reporting adds no startup refusal condition and leaves daemon startup fault-isolation intact |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch. | agrees | Startup VPP/kernel mismatch behavior is untouched by peer state display |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added". | agrees | Config `\| implied` formatting is untouched by peer state display |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs | agrees | No bus participant registration or message name changes; peer state changes only the fields inside existing peer command answers |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody | agrees | Bus participant `send > to` semantics are untouched |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody | agrees | Bus participant `receive > from` semantics are untouched |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message | agrees | Bus allow-list enforcement is untouched |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines | agrees | Bus filter compilation is untouched |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself | agrees | Bus non-arbitration is untouched |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry | agrees | Bus effective-filter display is untouched |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 | The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included | agrees | No subcommand or selector is added; `state`, `peer-state`, `state-reason`, `reason-detail`, and `next-retry` flow through existing `show bgp` commands and their generic pipe operators (`\| json`, `\| yaml`, `\| table`) |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 | A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed." | agrees | Peer commands keep using the generic pipe operations and formats unchanged |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 | A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does | agrees | All peer views remain read-only answers operated on by standard pipes without altering command behavior |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 | Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter | agrees | No engine RPC operation or authorization profile is added or altered |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-25 | Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly | agrees | Event delivery queuing is untouched; peer state and reason changes apply only to command answers and Prometheus metrics |
| `docs/architecture/plugin/plugin-system.md` | Design decisions | decision of 2026-11-23 | With no daemon running, a command that parses a config starts each declared external program in query mode, never its run loop, and validates against its declaration | agrees | External plugin query-mode startup is untouched |
| `docs/architecture/update-building.md` | Design decisions | decision of 2026-11-23 | Gate draft-simpson 0x0800 FlowSpec MP_REACH next-hop emission behind an explicit per-session opt-in (`session { flowspec-redirect-next-hop true; }`, default `false`), written automatically by `ze exabgp migrate` when a migrated neighbor uses Simpson actions or serves an API process on a FlowSpec family | agrees | FlowSpec next-hop wire emission is untouched by peer state display |
| `docs/architecture/exabgp-bridge.md` | Design decisions | decision of 2026-11-23 | An ExaBGP script is an ordinary bus endpoint that runs the wrapper; the bridge config section and the internal bridge mode do not exist | agrees | The ExaBGP wrapper remains an ordinary external bus endpoint reading `show bgp peer * detail` over the command dispatcher |
| `docs/architecture/exabgp-bridge.md` | Design decisions | decision of 2026-11-26 | ExaBGP wrapper endpoints request `base64` (`plugin.FormatBase64`) UPDATE delivery at Stage 5 `ready` using Ze's positional counted wire grammar (`upd <flags> <local-as> <remote-as> <conn-id> <n>:<peer> <n>:<local> <n>:<router-id> <n>:<extra-addpath> <n>:<b64-body>`, zero key names, `0:` when absent) over pooled `WriteBatchFrame` `MuxConn` batching while keeping low-frequency control/session events as JSON; `ze exabgp plugin` renders hot IPv4/IPv6 unicast UPDATEs directly into a `[]byte` buffer from wire bytes and falls back to the wire-backed `Event` renderer for non-unicast families and error cases | agrees | ExaBGP UPDATE/event wire delivery is untouched; only `StatusPeers` (`status` command) switches its input key from removed `fsm-state` to `state` |
| `docs/architecture/testing/interop.md` | Design decisions | decision of 2026-11-23 | Each Docker interop suite and scenario checks only the kernel capabilities it actually requires (`Suite.KernelCapabilities` unioned with selected `ScenarioPlan.KernelCapabilities`); suites whose selected scenarios require none run without unrelated kernel features (such as SRv6 or IPsec) | agrees | Docker interop kernel capability checks are untouched; only `zePeerState` moves its JSON read from `state` to `peer-state` |

## ExaBGP Bridge Requirements

ExaBGP 6.0, 5.0 and 4.2 compatibility is agreed (`ai/META-DECISIONS.md`,
"ExaBGP compatibility"): a script sees what ExaBGP would print. Sources read:
ExaBGP `main` (6.0, commit d6adb5ddf), `5.0` and `4.2` checkouts under
`~/Code/github.com/exa-networks/exabgp/`.

| ID | Requirement | ExaBGP source | Ze today | This spec |
|----|-------------|---------------|----------|-----------|
| E-1 | `status` lists each peer with `peer.fsm.name()`: the upper-case RFC 4271 state name | 6.0 `src/exabgp/reactor/api/command/reactor.py` `status`: `state = peer.fsm.name()`; `src/exabgp/bgp/fsm.py` `FSMState` (IDLE 0x01, ACTIVE 0x02, CONNECT 0x04, OPENSENT 0x08, OPENCONFIRM 0x10, ESTABLISHED 0x20), `FSM.name` returns `self.state.name` | `bridge_status.go` `StatusPeers` copies detail `fsm-state` (already upper case) | `fsm-state` is removed, so `StatusPeers` reads `state` and maps it back to the upper-case name through the ungated `internal/core/bgp/fsmstate` table (the same table `fsm/state.go` uses), never by upper-casing a string, never through a second list, and never by importing gated `internal/component/bgp/fsm`. Output unchanged |
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
- `internal/core/bgp/fsmstate` (backed into `fsm/state.go`) - the single ungated pure-stdlib table for upper-case FSM names, lower-case JSON spellings, RFC 4273 numbers (1..6), and consumer lookup (`cli`, `lg`, `web`, `exabgp/bridge`) without crossing the `ze_bgp` compile-out gate
- `peer_stats.go` `updatePeerStateMetric` and the FSM callback in `runOnce` - where the new FSM gauge is set

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | to verify at review: LG, web, MCP, dashboard read the command answer, never the reactor |
| No unintended coupling (components stay isolated) | No | to verify: `internal/component/plugin` gains only value fields, no `bgp` import |
| No duplicated functionality (extends existing, does not recreate) | No | to verify: one reason derivation in the reactor; one FSM name table |
| Zero-copy preserved where applicable (refs, not copies) | No | N-A: no wire path touched; command answers only |
| Registration over hardcoding, outbound: new columns register through `registerColumns` | No | to verify at review |
| Registration over hardcoding, inbound: the dashboard `stateRank`/`stateColor`, LG `stateClass`, web `peerFlagFromState`, and ExaBGP bridge `StatusPeers` derive their FSM spellings from the single ungated `internal/core/bgp/fsmstate` table rather than literal lists or gated `bgp/fsm` imports | No | to verify: grep each for literal `opensent`/`established` and run `./le arch tier check` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No plugin event serialises the lifecycle `PeerState`, so renaming the JSON key leaves the plugin event contract intact | `internal/component/bgp/format/text.go` (`appendPeerJSON` emits only `group`, `local`, `name`, `router-id`, `remote`, `peer-owner`, `connection-id`); `format/session_events.go` (`AppendFSM` uses `fsm.State.String()`); `server/events.go` (`deliverPeerState` emits `rpc.SessionState` `"up"`/`"down"`) | A third-party plugin breaks silently | Audited `format/`, `server/`, and `pkg/plugin/` for `PeerState` and lifecycle names: zero hits | confirmed |
| A-2 | Birdwatcher clients (Alice-LG) accept FSM-style values in `state` and free text in `info` | `docs/architecture/api/birdwatcher-compat.md` Section 4 (server MAY add members; clients MUST ignore unrecognized members) and Section 5 (`state` MUST be present with no lifecycle enum; `last_error` holds NOTIFICATION); `lg/render.go` `stateClass` already expects lower-case FSM names; `lg/handler_api.go` `transformProtocolsShort` already writes `info`; upstream `alice-lg/birdwatcher` reads BIRD's `BGP state` (`Established`, `Active`, `Connect`, `Idle`) | Alice-LG shows peers wrongly | Audited `birdwatcher-compat.md`, `lg/handler_api.go`, `lg/render.go`; `transformProtocols` adds `info` with FSM state and reason code only (never `reason-detail`) and Section 5 records it | confirmed |
| A-3 | gNMI, telemetry and doctor read no peer state value | Direct grep of `internal/component/{gnmi,telemetry,doctor}` for `PeerState`, `PeerInfo`, `fsm-state`, `show bgp`, `idle-hold`, `connecting` | A collector silently changes meaning | Zero hits across all three packages | confirmed |
| A-4 | `net.OpError` from the dialer can be classified into refused, timeout, unreachable on both Linux and macOS | `internal/core/network/network.go` (`RealDialer.DialContext` delegates directly to stdlib `net.Dialer.DialContext`); `reactor/session_connection.go` (`Session.Connect` wraps with `fmt.Errorf("connect to %s: %w", addr, err)`) | Reason reads "other" on one platform | Source verified; standard POSIX `syscall.ECONNREFUSED`/`ETIMEDOUT`/`EHOSTUNREACH`/`ENETUNREACH`, `os.ErrDeadlineExceeded`/`context.DeadlineExceeded`, and `net.Error.Timeout()` work on Linux and darwin; unknown errors map to `connect-failed` with bounded OS text | confirmed |
| A-5 | A passive peer (connect false) waiting for inbound is in RFC Active while listening on a session and Idle when stopped/held/between cycles | Traced `peer_run.go`, `session.go`, `peer_connection.go`, `fsm/fsm.go`: `runOnce` creates passive `Session` (`f.passive = true`), fires start → FSM enters `Active`, publishes `p.session`, skips outbound dial (`!IsActive()`), and awaits `Accept` (`SessionState()` returns `Active`); while stopped, admin-held (`holdAdminDown`), prefix-held (`holdDownAfterPrefixTeardown`), or between cycles, `p.session == nil` so `SessionState()` returns `Idle` | The display contradicts the RFC for passive peers | Traced passive path across `peer_run.go`, `session.go`, `peer_connection.go`, `fsm.go`: matches RFC 4271 Section 8.2.2 in every state (`state: active`, `peer-state: passive`, `state-reason: passive-wait` while listening; `state: idle` while held/stopped) | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The ExaBGP bridge `status` changes what an ExaBGP script sees when its source key changes from upper-case `fsm-state` to lower-case `state` | `./le test functional exabgp` status case shows a changed state spelling | The bridge maps `state` back through `internal/core/bgp/fsmstate`; a bridge unit test pins the exact output spelling before and after |
| R-2 | A consumer still reads `state` as the lifecycle (an `established` match keeps working by coincidence, `connecting`/`idle-hold` matches silently never fire) | grep for the lifecycle names outside `peer-state` readers finds a hit | Phase 1 inventory: every reader in the research table moves in the same phase; a review grep for the five lifecycle literals |
| R-3 | The reason derivation reads Peer fields set by different goroutines and shows a stale or contradictory reason (retry deadline from a cycle that already ended) | A reason with a `next-retry` in the past, or `connect-refused` beside `established` | All reason fields written and read under `p.mu`; the deadline is cleared when the wait ends; established clears connect error; a unit test drives the sequence |
| R-4 | Reason precedence is wrong when several apply (admin hold while a connect error is still stored) | Unit table test of combinations | One fixed precedence, documented in the spec and the page: operator-shutdown, prefix-limit, bfd-wait, notification, connect error, passive-wait, none |
| R-5 | Renaming lifecycle `active` to `passive` breaks a Prometheus dashboard keyed on `to="active"` | Metric label change in the changelog | Owner approved the rename; `docs/guide/monitoring.md` states it |
| R-6 | Classifying OS dial errors differs by platform (macOS vs Linux errno) | Unit test red on darwin | Classify with `errors.Is` on the `syscall` errno set and `os.ErrDeadlineExceeded`/`net.Error.Timeout()`; unknown errors map to `connect-failed` with the OS text in `reason-detail`, never to a guessed class |
| R-7 | Golden or `.ci` expectations change (`peer-admin-shutdown.ci` matches `idle-hold` in `state`) | Functional red | Each changed expectation goes to an independent `ze-test-judge` before it lands (`ai/META-DECISIONS.md`, test changes) |
| R-8 | The LG `info` field contents change for Alice-LG users | LG birdwatcher consumer reads unexpected text in `info` | A-2 confirmed; `transformProtocols` writes FSM state and reason code only (never `reason-detail`) into `info`, and Section 5 of `birdwatcher-compat.md` records both `state` and `info` |

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

### Interop Tests
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | `Scope: cli` — no wire-visible change, only what Ze reports about its own state. Interop helpers (`zePeerState` in `check_special.go`, `helper.go`) move to `peer-state` to preserve the existing admission barrier | |

## Files to Modify
- `internal/component/bgp/reactor/peer.go` - lifecycle name `active` → `passive`; reason fields on Peer
- `internal/component/bgp/reactor/peer_run.go` - store dial error class, backoff and prefix-timer deadlines; clear on established
- `internal/component/bgp/reactor/peer_admin.go` - record operator-shutdown reason
- `internal/component/bgp/reactor/reactor_api.go` - fill PeerInfo reason fields
- `internal/component/bgp/reactor/peer_stats.go`, `reactor_metrics.go` - FSM gauge; lifecycle label names; help text
- `internal/component/plugin/types_bgp.go` - PeerInfo reason fields; lifecycle `passive` name
- `internal/component/bgp/fsm/state.go` - delegate upper-case, lower-case, and RFC 4273 number lookups to `internal/core/bgp/fsmstate`
- `internal/component/bgp/plugins/cmd/peer/peer.go`, `summary.go`, `health.go` - `state`, `peer-state`, `state-reason`, `reason-detail`, `next-retry`; drop `fsm-state`; columns
- `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` - update `show bgp peer detail` and peer command prose descriptions (`state` is FSM, `peer-state` is lifecycle, `fsm-state` removed, reason fields documented)
- `internal/component/config/yang/modules/ze-types.yang` - add `peer-state`, `state-reason`, `reason-detail`, `next-retry` leaves to `grouping peer-info`
- `internal/component/cli/model_dashboard.go`, `model_dashboard_sort.go`, `model_dashboard_render.go` - State and Reason columns using `internal/core/bgp/fsmstate`
- `internal/component/lg/handler_api.go`, `render.go`, `handler_ui.go` - birdwatcher `state`/`info` fields and colouring using `internal/core/bgp/fsmstate`
- `internal/component/web/page_bgp_peers.go`, `page_bgp_summary.go` - flag and colour from FSM state using `internal/core/bgp/fsmstate`
- `internal/component/mcp/ui/bgp-peer/app.js` - CSS class from FSM state
- `internal/exabgp/bridge/bridge_status.go` - read `state`, map back to ExaBGP's upper-case spelling via `internal/core/bgp/fsmstate`
- `internal/le/interoplab/bgp/check_special.go`, `check_delayopen.go`, `helper.go`, `names.go`; `internal/le/site/terminaldemo/*` validators; `internal/test/fixture/*` readers of `state`/`fsm-state` (`zePeerState` reads `peer-state` to preserve the admission barrier and `idle-hold` checks)
- `test/plugin/peer-admin-shutdown.ci`, `test/plugin/metrics-session-lifecycle.ci`
- Docs: `docs/architecture/behavior/peer-lifecycle.md`, `docs/architecture/behavior/fsm.md`, `docs/architecture/diagnostics/event-history-rings.md`, `docs/architecture/api/birdwatcher-compat.md`, `docs/architecture/api/commands.md`, `docs/architecture/exabgp-bridge.md`, `docs/architecture/testing/interop.md`, `docs/architecture/web-interface.md`, `docs/architecture/web-workbench-pages.md`, `docs/guide/monitoring.md`, `docs/guide/bgp-peering.md`, `docs/guide/configuration.md`, `docs/guide/looking-glass-howto.md`, `docs/features/cli-commands.md`, `docs/features/configuration.md`

## Files to Create
- `internal/core/bgp/fsmstate/fsmstate.go` - ungated pure-stdlib leaf declaring the six RFC 4271 Section 8.2.2 FSM states, upper/lower spellings, and RFC 4273 `bgpPeerState` 1..6 numbers (parallel to `internal/core/bgp/msgtype`)
- `internal/core/bgp/fsmstate/fsmstate_test.go`
- `internal/component/bgp/reactor/peer_reason.go` - the one reason derivation and dial error classification
- `internal/component/bgp/reactor/peer_reason_test.go`
- `test/plugin/peer-state-connect-refused.ci`
- `test/plugin/peer-state-prefix-limit-hold.ci`
- `test/plugin/metrics-peer-fsm-state.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/config/yang/modules/ze-types.yang` (`grouping peer-info` already defines `leaf state` as lower-case RFC 4271 FSM state; add `peer-state`, `state-reason`, `reason-detail`, `next-retry`) and `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` (update prose descriptions, removing `fsm-state`) |
| YANG validation constraints | Yes | `state`, `peer-state`, and `state-reason` leaves documented/typed in `ze-types.yang` `grouping peer-info` (read-only RPC output; no new config input) |
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
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` if it compares peer state display with other implementations |
| 12 | Internal architecture changed? | Yes | `docs/architecture/behavior/peer-lifecycle.md`, `docs/architecture/behavior/fsm.md` |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | Yes | `docs/guide/monitoring.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Commands keep their names |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Affected: `peer-lifecycle.md`, `fsm.md`, `docs/architecture/diagnostics/event-history-rings.md` (`PeerState` moves to `peer-state` with `active` → `passive`, updating the rationale that `PeerState` is "the state an operator observes"), `birdwatcher-compat.md`, `commands.md`, `exabgp-bridge.md`, `docs/architecture/testing/interop.md` (`zePeerState` reads `peer-state` to preserve the admission barrier), `docs/architecture/web-interface.md` (LG state colouring and `info`), `docs/architecture/web-workbench-pages.md` (BGP peer flag wired to FSM state). Checked and unaffected: `docs/architecture/api/architecture.md`, `docs/architecture/api/process-protocol.md` (events carry up/down only, A-1 confirmed), `docs/architecture/core-design.md`, `docs/architecture/config/yang-config-design.md`, `docs/architecture/cli/command-verbs.md`. Run `./le spec citation anchors spec plan/release/fixes/spec-bgp-peer-state-fsm-and-reason.md` at implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/architecture/api/commands.md`, `docs/guide/looking-glass-howto.md` example outputs show `"state": "established"`; regenerate from a run |

## Design presentation

### What changes

Every BGP peer command answer (`show bgp`, `show bgp peer list`, `show bgp peer detail`, `show bgp health`, peer `statistics`, peer `capabilities`) reports the RFC 4271 Section 8.2.2 FSM state under `state` (`idle`, `connect`, `active`, `opensent`, `openconfirm`, `established`), moves Ze's outer lifecycle enum to `peer-state` (with lifecycle `active` renamed `passive`), drops the redundant `fsm-state` key, and adds structured reason fields (`state-reason`, `reason-detail`, `next-retry`) derived once in the reactor under `p.mu` with a fixed precedence (`operator-shutdown`, `prefix-limit`, `bfd-wait`, `notification-received`/`notification-sent`, `connect-refused`/`connect-timeout`/`connect-unreachable`/`connect-failed`, `passive-wait`, `none`).

A single ungated pure-stdlib core leaf (`internal/core/bgp/fsmstate`, parallel to `internal/core/bgp/msgtype`) holds the six RFC 4271 state names (upper case and lower case) and RFC 4273 `bgpPeerState` numbers (1..6). `internal/component/bgp/fsm/state.go` delegates to it, and every consumer outside `ze_bgp` (`monitor bgp` dashboard, web peers page, looking glass, and ExaBGP bridge `status`) imports only that core leaf so no consumer duplicates state spellings or crosses the `ze_bgp` compile-out gate. Prometheus keeps `ze_peer_state` and `ze_peer_state_transitions_total` on lifecycle labels (with `passive`) and adds the push-model gauge `ze_peer_fsm_state{peer}` reporting RFC 4273 values 1..6.

### Before and after

1. **Unreachable outbound peer (`127.0.0.4`, port closed) between dial retries**:
   - Before: `show bgp | json` emits `"state": "connecting"` with no reason and no retry deadline; `show bgp peer detail` emits `"state": "connecting"` beside `"fsm-state": "IDLE"`; `monitor bgp` shows `connecting`.
   - After: `show bgp | json` and `show bgp peer detail` emit `"state": "idle"`, `"peer-state": "connecting"`, `"state-reason": "connect-refused"`, `"reason-detail": "connection refused"`, `"next-retry": "<RFC3339>"`, and no `"fsm-state"`; `monitor bgp` renders `Idle` in the State column and `connect-refused (retry 12s)` in the Reason column.
2. **Operator-disabled peer vs. prefix-limit held peer**:
   - Before: both show `"state": "idle-hold"` with nothing distinguishing them on peer commands.
   - After: operator disable shows `"state": "idle"`, `"peer-state": "idle-hold"`, `"state-reason": "operator-shutdown"`, `"reason-detail": "<message>"` (no `next-retry`); prefix-limit hold shows `"state": "idle"`, `"peer-state": "idle-hold"`, `"state-reason": "prefix-limit"`, `"reason-detail": "<afi/safi>"` (plus `"next-retry"` when waiting on a reconnect timer).
3. **Passive peer (`connect false`) waiting for inbound TCP**:
   - Before: `"state": "active"` (lifecycle), contradicting RFC 4271 when stopped or held.
   - After: while listening on a passive `Session`, `"state": "active"`, `"peer-state": "passive"`, `"state-reason": "passive-wait"`; while stopped or held down, `"state": "idle"`, `"peer-state": "stopped"` or `"idle-hold"`.
4. **Looking Glass birdwatcher API, Web peers page, ExaBGP bridge, and Prometheus**:
   - Before: LG `transformProtocols` relays lifecycle strings into `state` (which LG `stateClass` and web `peerFlagFromState` do not match for non-established peers) and omits `info`; ExaBGP `status` reads `fsm-state`; Prometheus exports lifecycle metrics only.
   - After: LG emits FSM state in `state`, FSM state plus reason code (never `reason-detail`) in `info`, and keeps NOTIFICATION-only `last_error`; LG and web colour every FSM state cleanly; ExaBGP `status` reads `state` and prints byte-identical upper-case `IDLE`/`ACTIVE`/`CONNECT`/`OPENSENT`/`OPENCONFIRM`/`ESTABLISHED`; Prometheus adds `ze_peer_fsm_state{peer}` (1..6) alongside lifecycle metrics using `passive`.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Keep `state` as Ze's lifecycle and add a separate FSM column beside it | Operators and scripts expect `state` to mean the RFC 4271 FSM state; owner decided `state` becomes FSM and lifecycle moves to `peer-state` |
| Single free-text `info` string or per-view reason derivation instead of structured fields | Owner chose structured design A (`state-reason`, `reason-detail`, `next-retry`) derived once in the reactor so scripts match stable kebab-case codes and views cannot drift |
| Keep `fsm-state` alongside `state` during a transition window | Violates Ze's no-layering rule; all internal and external consumers move in the same phase |
| Have `cli`, `lg`, `web`, and `exabgp/bridge` import `internal/component/bgp/fsm` directly | Crosses the `ze_bgp` compile-out gate (`ai/doors/system.md`, `./le arch tier check`); lifting the pure six-state vocabulary into ungated `internal/core/bgp/fsmstate` (like `internal/core/bgp/msgtype`) satisfies both declared-once and compile-out |
| Point interop `zePeerState` (`check_special.go`) at the new FSM `state` field | `waitZePeerState` relies on `PeerStateEstablished` being published only after `enterEstablished` publishes encoding contexts and forwarding facts (`docs/architecture/testing/interop.md`), and `check_delayopen.go` checks `idle-hold`; reading `peer-state` preserves the exact admission barrier |
| Write dial errors into `last-error` / LG `last_error` | Violates normative `docs/architecture/api/birdwatcher-compat.md` Section 5, which reserves `last_error` for the NOTIFICATION that ended the last session |

### Documents departed from

1. **`docs/architecture/behavior/peer-lifecycle.md` — section `Peer states (distinct from FSM states)`**
   - Current statement: `` `Peer` has its own state enum that is coarser than `fsm.State` and reflects operator-visible peer status, not protocol detail. `` (with `PeerStateActive` shown as `active` and lifecycle shown as `state`).
   - Why this design departs: Owner approved presenting the RFC 4271 FSM state as the operator-visible `state`, moving `PeerState` to `peer-state`, and renaming `PeerStateActive` (`active`) to `PeerStatePassive` (`passive`) so it no longer collides with RFC 4271 `Active`.
   - What the page says after: `PeerState` is the outer lifecycle reported under `peer-state` (with `PeerStatePassive` / `passive` replacing `PeerStateActive` / `active`), while `state` reports the session's RFC 4271 FSM state and `state-reason`/`reason-detail`/`next-retry` explain why a peer is not established.
2. **`docs/architecture/exabgp-bridge.md` — section `` `status` is answered by the bridge ``**
   - Current statement: `` each state is the peer's `fsm-state`, the RFC 4271 Section 8.2.2 state as ExaBGP prints it (`IDLE`, `ACTIVE`, `CONNECT`, `OPENSENT`, `OPENCONFIRM`, `ESTABLISHED`). ``
   - Why this design departs: `fsm-state` is removed under no-layering once `state` itself carries the FSM state; `StatusPeers` reads `state` and maps it through `internal/core/bgp/fsmstate` so the ExaBGP-facing `status` output stays byte-identical.
   - What the page says after: Each peer's state in `status` is read from `state` in `show bgp peer * detail` and formatted via `internal/core/bgp/fsmstate` into ExaBGP's upper-case RFC 4271 spelling (`IDLE`, `ACTIVE`, `CONNECT`, `OPENSENT`, `OPENCONFIRM`, `ESTABLISHED`).
3. **`docs/architecture/testing/interop.md` — section `GoBGP export-reload successor proof`**
   - Current statement: `` Ze's peer-list producer `handleBgpPeerList` serializes `reactorAPIAdapter.Peers`'s `Peer.State()`, not the raw session FSM state. ``
   - Why this design departs: `handleBgpPeerList` now serializes the FSM state under `state` and `Peer.State()` under `peer-state`; `zePeerState` reads `peer-state` so `waitZePeerState` retains the exact local admission barrier after encoding contexts and forwarding facts are published.
   - What the page says after: `handleBgpPeerList` serializes `Peer.State()` under `peer-state` (beside the session FSM state under `state`), and `zePeerState` reads `peer-state` as the local admission barrier.
4. **`docs/architecture/diagnostics/event-history-rings.md` — section `The decisions`**
   - Current statement: `` **BGP history records `PeerState`, not the internal `fsm.State`.** `PeerState` is the state an operator observes, and it carries fewer values than the FSM's own. The transition is appended where the peer's state actually changes. ``
   - Why this design departs: `PeerState` is no longer the primary operator-visible `state` (which becomes RFC 4271 `fsm.State`); `PeerState` moves to `peer-state` and renames `PeerStateActive` (`active`) to `PeerStatePassive` (`passive`), while the ring continues recording outer `PeerState` transitions.
   - What the page says after: BGP history continues to record outer `PeerState` (`peer-state`, with `passive` replacing `active`) transitions where the peer's lifecycle changes, distinct from the RFC 4271 `fsm.State` reported under `state`.

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- Create ungated `internal/core/bgp/fsmstate` leaf (wiring `fsm/state.go` to it), add `PeerInfo` reason fields (zero reason `none`), update `cmd/peer` answers to write `state` (FSM), `peer-state`, `state-reason`, `reason-detail`, `next-retry` and drop `fsm-state`, update `ze-types.yang` and `ze-peer-cmd.yang`, and move every reader in the research table in the same phase (dashboard, web, LG, MCP app, ExaBGP bridge, interop `zePeerState`/`helper.go`, demo validators, fixtures, `.ci`)
   - Tests: `TestPeersInfoCarriesFSMStateAndReason`, `TestPeerAnswersCarryStateKeys`, `TestStatusPeersReadsFSMState`, `peer-state-connect-refused` (fails: reason still `none`)
   - Verify: no reader of the lifecycle under `state` remains (grep for the five lifecycle literals); `./le arch tier check` confirms no consumer imports gated `internal/component/bgp/fsm`
2. **Phase: Reason recording** -- dial error classification, backoff and prefix-timer deadlines, admin and prefix holds on the Peer, the one derivation with its precedence; lifecycle `active` → `passive`
   - Tests: `TestPeerReasonPrecedence`, `TestDialErrorClassification`, `TestBackoffDeadlineStoredAndCleared`, `peer-state-connect-refused`, `peer-admin-shutdown`, `peer-state-prefix-limit-hold`
   - Verify: passive peer (`Connect: false`) reports `state: active`, `peer-state: passive`, `state-reason: passive-wait` while listening and `state: idle` while stopped/held
3. **Phase: Views** -- dashboard State and Reason columns with "retry Ns"; web flag; LG colour, `info` (FSM state + reason code only, never `reason-detail`); MCP class; colour and sort derived from `internal/core/bgp/fsmstate`
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
| Rule: declared once | FSM spellings (upper case, lower case, RFC 4273 numbers 1..6, ExaBGP spelling) all derive from `internal/core/bgp/fsmstate` (which `fsm/state.go` delegates to) without crossing the `ze_bgp` compile-out gate |

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
| `state` is the FSM state, lifecycle moves to `peer-state` (owner) | Keep `state` as lifecycle and add a column | Operators and scripts read `state` as the RFC state, which is what other BGP implementations show under that name |
| Structured reason: `state-reason` code plus `reason-detail` and `next-retry` (owner chose A) | B: one free-text `info` string combining state and last error; C: each view derives its own | Scripts match stable codes; one derivation; showing the FSM state beside a separate peer condition (admin down, prefix limit, passive) is the common practice |
| Lifecycle `active` renamed `passive` (owner) | Keep `active` | Clashes with RFC Active; "passive" is the usual name for a peer that waits for the remote to connect |
| New gauge `ze_peer_fsm_state` with RFC 4273 values (owner) | Keep metrics lifecycle only; switch the existing gauge | Lifecycle dashboards keep working; FSM visible to Prometheus with standard MIB numbering |
| Connect error gets its own field, `last-error` unchanged | Reuse `last-error` | `last-error` is the NOTIFICATION under the normative LG contract |
| Reason derived in the reactor, next to both states | Derive in `cmd/peer` | The reactor holds the Peer fields under `p.mu`; the answer must be a self-contained value |

## Known Limitations
- ExaBGP `show neighbor` / `peer show` is not translated by the bridge (refused today). When it is, its `state` column must print `DISABLED` exactly when this spec's answer has `state` `idle` and `state-reason` `operator-shutdown`, and ExaBGP's `FSM.name()` spelling otherwise (E-6). That translation is not in this spec: Thomas agreed it gets its own spec in `plan/release/fixes/`, depending on this one, started from `plan/handover/01-exabgp-show-neighbor-spec.md`
- No FSM metric beyond the gauge (no FSM transition counter); the transition history stays in `show bgp peer history`
- The reason reports only the current cause; earlier causes stay in history and logs

## RFC Documentation

`Scope: cli` — no wire-visible protocol behavior changes. Cite `// RFC 4271 Section 8.2.2` above the state vocabulary in `internal/core/bgp/fsmstate/fsmstate.go` and `// RFC 4273 Section 4` above the `bgpPeerState` numbering used by `ze_peer_fsm_state`.

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
- [ ] The scoped tests covering what changed pass, run in the main checkout by the main thread (`./le test unit pkg path <pkg>/`, `./le test functional plugin`, `./le test functional ui`, `./le test functional exabgp`, `./le arch tier check`, `./le doc check all`); no agent runs any `./le verify` command
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
