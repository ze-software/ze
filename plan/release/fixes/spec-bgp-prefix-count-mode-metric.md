# Spec: BGP prefix enforcement metric count-mode labels

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Priority | P1 |
| Release reason | Existing BGP prefix-limit enforcement metrics must identify whether `offered` or `installed` counting produced their values so scrapes do not mislead operators. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Owner decisions (2026-11-24)

| Topic | Decision |
|-------|----------|
| Split from `plan/release/fixes/spec-bgp-per-peer-received-counter.md` | Split AC-5 as a v1.0 release fix now so enforcement metrics identify their count mode (`offered` vs `installed`). Pre-policy `routes-received` stays in `plan/release/fixes/spec-bgp-per-peer-received-counter.md`. |

## Task

Ze's BGP prefix-limit enforcement supports two per-family counting modes configured via `session { family { <afi/safi> { prefix { count offered|installed; } } } }` (`internal/component/bgp/yang/ze-bgp-conf.yang`): `offered` (the default, an event tally that counts every announcement including re-announcements and dropped UPDATEs) and `installed` (the cardinality of a per-family set of registered route identities, immune to implicit-withdraw inflation and rolled back on refusal).

Today the four count-derived Prometheus metrics in `internal/component/bgp/reactor/reactor_metrics.go` (`ze_bgp_prefix_count`, `ze_bgp_prefix_ratio`, `ze_bgp_prefix_warning_exceeded`, and `ze_bgp_prefix_maximum_exceeded_total`) carry only `peer` and `family` labels. A Prometheus scrape cannot tell whether a peer/family's count, ratio, warning state, or exceeded counter came from `offered` or `installed`, even though the two modes measure different things. In addition, neither peer removal (`doRemovePeer` in `reactor_peers.go`) nor the shared locked removal choke point (`Reactor.removalLocked` in `peer_removal.go`, called by `doRemovePeer`, `reconcilePeersJournaled` remove, `reconcilePeersJournaled` add-undo rollback, and `removeDynamicPeer`) deletes per-family prefix metric series, so changing a family's count mode across a reload leaves stale series behind.

Goal: add the closed `mode` label (`offered` or `installed`, matching the YANG enum and structured log field in `reportPrefixExceeded`) to `ze_bgp_prefix_count`, `ze_bgp_prefix_ratio`, `ze_bgp_prefix_warning_exceeded`, and `ze_bgp_prefix_maximum_exceeded_total`; retire obsolete `(peer, family, mode)` series cleanly on mode changes and peer removal without racing a stopping session; preserve prefix-limit enforcement semantics and static/peer-level metrics (`ze_bgp_prefix_maximum`, `ze_bgp_prefix_warning`, `ze_bgp_prefix_teardown_total`, `ze_bgp_prefix_stale`); and document the label addition and PromQL query migration.

This spec owns AC-5 split out from `plan/release/fixes/spec-bgp-per-peer-received-counter.md` per Thomas's 2026-11-24 ruling (originally inherited from `fixit-prefix-count-metric-does-not-say-its-mode` and reconciled from `plan/post-release/features/spec-bgp-peer-metric-labels.md`). Optional descriptive labels (`description`, `hostname`, `peer-type`, `peer-group`) and native collector selection remain in `plan/post-release/features/spec-bgp-peer-metric-labels.md`.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->
- [ ] `ai/doors/system.md` - platform boundaries, registration, and shared machinery
  → Constraint: Prometheus metrics stay behind `internal/core/metrics.Registry` and follow the push-model registry pattern.
- [ ] `ai/doors/bgp-engine.md` - BGP engine boundaries and hot-path rules
  → Constraint: prefix metric publication stays off the per-NLRI loop and adds no new lock on the session read hot path.
- [ ] `docs/architecture/decisions/001-pull-model-metrics.md` - push-model `metrics.Registry` decision and series custody
  → Decision: continue using `metrics.Registry` (`GaugeVec` / `CounterVec`) and delete retired series via `Delete(...)` rather than introducing a pull-model adapter.
- [ ] `docs/architecture/core-design.md` - reactor metrics, prefix limit enforcement, and peer reload
  → Constraint: `PrefixCount` is not in `hotSwappableSettings` (`peer_settings_apply.go`), so changing a family's count mode triggers a peer remove-and-add restart in `reconcilePeersJournaled`.
- [ ] `docs/architecture/behavior/peer-lifecycle.md` - `Peer.Stop` asynchronous teardown vs `Peer.cleanup`
  → Constraint: `Peer.Stop` cancels asynchronously while `reconcilePeersJournaled` adds the replacement peer immediately; retiring old series must fence late writes from the stopping peer without deleting a replacement peer's active series.

**Key insights:**
- `reportPrefixExceeded` (`session_prefix.go`) already logs `"mode", s.settings.prefixCountFor(famName).String()`, so the label name `mode` and closed values `offered`/`installed` match the existing operator surface.
- `doRemovePeer` (`reactor_peers.go`) currently deletes only single-label `prefixTeardownTotal` and `prefixStale`, and `Reactor.removalLocked` (`peer_removal.go`, the shared locked choke point for all four peer-removal paths) deletes no per-family prefix metrics at all, leaving stale series behind on peer removal and mode-change reload.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/reactor_metrics.go` - `initReactorMetrics` registers `ze_bgp_prefix_count`, `ze_bgp_prefix_ratio`, `ze_bgp_prefix_warning_exceeded`, and `ze_bgp_prefix_maximum_exceeded_total` with labels `[]string{metricLabelPeer, metricLabelFamily}` only. `ze_bgp_prefix_maximum` and `ze_bgp_prefix_warning` carry `peer, family`; `ze_bgp_prefix_teardown_total` and `ze_bgp_prefix_stale` carry `peer`.
- [ ] `internal/component/bgp/reactor/session_prefix.go` - `setPrefixCountMetric`, `setPrefixWarningExceededMetric`, and `incrPrefixExceededMetric` emit with `(s.peerLabel(), family)` and omit `s.settings.prefixCountFor(family).String()`. `setPrefixConfigMetrics` publishes `prefixMaximum`, `prefixWarning`, and `prefixStale` on peer add without retiring opposite-mode series.
- [ ] `internal/component/bgp/reactor/peer_settings.go` - `PrefixCountMode` defines closed enum values `PrefixCountOffered` (`"offered"`) and `PrefixCountInstalled` (`"installed"`), with `PeerSettings.prefixCountFor(fam)` defaulting absent families to `PrefixCountOffered`.
- [ ] `internal/component/bgp/reactor/peer_settings_apply.go` - `hotSwappableSettings` excludes `PrefixCount`, so changing `prefix { count offered|installed; }` forces a peer restart in `reconcilePeersJournaled`.
- [ ] `internal/component/bgp/reactor/peer_removal.go` - `Reactor.removalLocked` (lines 69-105) is the shared locked removal entry point called by all four peer-removal paths (`doRemovePeer` in `reactor_peers.go:522`, `reconcilePeersJournaled` remove in `reactor_api.go:1413`, `reconcilePeersJournaled` add-undo rollback in `reactor_api.go:1466`, and `removeDynamicPeer` in `reactor_dynamic.go:257`); it sets `peer.stopping.Store(true)` (line 71) but does not delete per-family prefix metrics.
- [ ] `internal/component/bgp/reactor/reactor_peers.go` - `doRemovePeer` deletes `prefixTeardownTotal` and `prefixStale` and delegates to `r.removalLocked`, without deleting per-family prefix metrics.
- [ ] `internal/component/bgp/reactor/peer.go`, `peer_session_factory.go`, `peer_run.go` - `Peer` holds `stopping atomic.Bool` and wires `session.prefixMetrics = p.reactor.rmetrics` in `newSessionWithEpoch`; `Peer.Stop` cancels asynchronously and `Peer.cleanup` runs after `Session.Run` exits.

**Behavior to preserve:**
- Prefix-limit enforcement arithmetic, thresholds, warning/clear edges, report-bus events, Cease/MaxPrefixes NOTIFICATIONs, and warn-only (`teardown false`) drops remain completely unchanged for both `offered` and `installed`.
- Static configuration gauges (`ze_bgp_prefix_maximum`, `ze_bgp_prefix_warning`) and peer-level metrics (`ze_bgp_prefix_teardown_total`, `ze_bgp_prefix_stale`) keep their existing names and label sets.
- Hot-path per-NLRI counting takes no new lock and allocates nothing per NLRI.

**Behavior to change:**
- `ze_bgp_prefix_count`, `ze_bgp_prefix_ratio`, `ze_bgp_prefix_warning_exceeded`, and `ze_bgp_prefix_maximum_exceeded_total` carry the `mode` label (`offered` or `installed`).
- Obsolete `(peer, family, mode)` series are deleted on count-mode changes across reload and on peer removal, with a retirement fence preventing a stopping session from recreating an obsolete series after removal.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Reactor Prometheus metric registry | `internal/component/bgp/reactor/reactor_metrics.go` `reactorMetrics`, `initReactorMetrics`, `metricLabel*` constants | Registers push-model `GaugeVec` and `CounterVec` metrics behind `internal/core/metrics.Registry` with explicit label-name constants | Extends: adds `metricLabelMode = "mode"` to the four count-derived prefix limit vectors while keeping the existing registry and push model |
| Per-session prefix metric helpers | `internal/component/bgp/reactor/session_prefix.go` `setPrefixCountMetric`, `setPrefixWarningExceededMetric`, `incrPrefixExceededMetric`, `setPrefixConfigMetrics` | Publishes prefix count, ratio, warning-exceeded, and maximum-exceeded series per `(peer, family)` from `applyPrefixDelta`, `applyPrefixCheck`, and `reportPrefixExceeded` | Extends: passes `s.settings.prefixCountFor(family).String()` as the `mode` label, checks a per-peer atomic retirement fence, and records emitted `(family, mode)` entries for cleanup |
| Per-family count-mode enum | `internal/component/bgp/reactor/peer_settings.go` `PrefixCountMode`, `PrefixCountOffered`, `PrefixCountInstalled`, `PeerSettings.prefixCountFor` | Defines the closed two-value mode vocabulary (`offered`, `installed`) and resolves absent families to `offered` | Extends: reuses `prefixCountFor(fam).String()` as the sole source of `mode` label values and opposite-mode retirement |
| Peer removal and reload metric cleanup | `internal/component/bgp/reactor/peer_removal.go` `Reactor.removalLocked`; `internal/component/bgp/reactor/reactor_peers.go` `doRemovePeer`; `internal/component/bgp/reactor/peer_run.go` `Peer.cleanup` | `Reactor.removalLocked` is the shared locked removal choke point for all four removal paths (`doRemovePeer`, `reconcilePeersJournaled` remove, `reconcilePeersJournaled` add-undo rollback, `removeDynamicPeer`) and sets `peer.stopping.Store(true)`; `Peer.cleanup` runs after `Session.Run` exits | Extends: calls `deletePeerPrefixMetrics(r.rmetrics, peer)` from `Reactor.removalLocked` right after `peer.stopping.Store(true)`, fences session metric helpers on `&p.stopping`, and adds an owner-checked residual sweep in `Peer.cleanup` |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | Ze is a platform, and other people's code runs against it | the door every change reads | "Ze is pre-release, so a contract can still change. It changes on purpose, with its page, never as a side effect." | agrees | Adding the `mode` label to the four count-derived prefix metrics is an intentional, owner-ordered change documented in `docs/guide/monitoring.md`, `docs/guide/configuration.md`, and `docs/guide/bgp-peering.md`. |
| `ai/doors/bgp-engine.md` | Where it sits | the door covering `internal/component/bgp/reactor/*` | "The engine speaks the protocol and holds no policy." | agrees | Metric mode labeling stays inside the BGP reactor enforcement layer and adds no policy or RIB state to the engine. |
| `ai/doors/le-tooling.md` | Boundaries | the door covering `test/plugin/bgp-prefix-count-mode.ci` | "A compiled Go fixture returns an error when an assertion fails" | agrees | `test/plugin/bgp-prefix-count-mode.ci` follows the functional test harness contract and fails on any missing or stale mode label series. |
| `docs/architecture/core-design.md` | 18a. BGP Peer Reload: Swap or Restart | `// Design:` of `reactor_metrics.go`, `session_prefix.go`, `peer.go`, `peer_run.go`, `peer_removal.go` | "Every other field forces a restart, so a field added to `PeerSettings` tomorrow is restart-scoped until somebody classifies it on purpose." | agrees | `PrefixCount` remains restart-scoped; changing `count` between `offered` and `installed` runs `reconcilePeersJournaled`'s remove-and-add restart path through `Reactor.removalLocked` where obsolete series are retired. |
| `docs/architecture/decisions/001-pull-model-metrics.md` | Decision | anchored to `reactor_metrics.go` | "The Prometheus implementation should continue to use the existing push-model `metrics.Registry` path." | agrees | Extends the existing `metrics.Registry` `GaugeVec`/`CounterVec` handles in `reactorMetrics` and retires obsolete series via `Delete(...)` without introducing a pull-model adapter. |
| `docs/architecture/behavior/fsm-established.md` | Prefix-limit route identity | anchored to `session_prefix.go`, `peer_run.go` | "The default `offered` mode counts announcement and withdrawal events using the family's registered framing. The `installed` mode counts a set of logical routes using the registered semantic key, not the complete NLRI bytes." | agrees | Preserves both counting modes verbatim and exposes which mode produced each count-derived Prometheus series via the closed `mode` label. |
| `docs/architecture/behavior/fsm.md` | Peer lifecycle observers | anchored to `reactor_metrics.go`, `peer.go`, `peer_run.go` | "Reactor maintains a list of observers notified on state changes." | agrees | Does not alter FSM transitions, state-change callbacks, or lifecycle observer notifications. |
| `docs/architecture/behavior/peer-lifecycle.md` | Entry points | `// Design:` of `peer_session_factory.go`; anchored to `peer.go`, `peer_run.go`, `peer_removal.go` | "Cancel the peer context. The run loop notices at the next select and unwinds cleanly via `cleanup()`." | agrees | Respects asynchronous `Peer.Stop` by fencing late metric writes with `Peer.stopping` (already set in `Reactor.removalLocked`) and running an owner-checked residual cleanup sweep in `Peer.cleanup()`. |
| `docs/architecture/api/commands.md` | Overview | anchored to `session_prefix.go`, `peer.go`, `peer_run.go` | "Ze uses verb-first command paths with JSON or text encoding." | agrees | Changes no CLI or API commands; report-bus warnings (`prefix-threshold`, `prefix-stale`) remain untouched. |
| `docs/architecture/api/architecture.md` | RIB Ownership | anchored to `peer.go`, `peer_run.go`, `peer_removal.go` | "API programs own all RIB data and logic." | agrees | Keeps prefix enforcement metrics in the BGP reactor without moving RIB data into the engine. |
| `docs/architecture/api/process-protocol.md` | Overview | anchored to `peer.go`, `peer_run.go` | "The protocol is the same for all invocation modes (internal goroutine, external subprocess)." | agrees | Makes no change to plugin RPCs, event wire format, or process lifecycle. |
| `docs/architecture/plugin/rib-storage-design.md` | Overview | anchored to `session_prefix.go`, `peer.go`, `peer_run.go` | - | silent | Covers `bgp-rib` storage and best-path replay; silent on reactor Prometheus metric labels. |
| `docs/architecture/api/capability-contract.md` | Implementation Status | anchored to `peer.go` | - | silent | Covers BGP capability negotiation and OPEN injection; silent on Prometheus prefix metrics. |
| `docs/architecture/api/ipc_protocol.md` | Overview | anchored to `peer.go` | - | silent | Covers plugin IPC line framing; silent on Prometheus prefix metrics. |
| `docs/architecture/api/json-format.md` | Overview | anchored to `peer.go` | - | silent | Covers JSON event payloads; silent on Prometheus prefix metrics. |
| `docs/architecture/api/update-syntax.md` | Status | anchored to `peer.go` | - | silent | Covers `send bgp ... update text` grammar; silent on Prometheus prefix metrics. |
| `docs/architecture/behavior/fsm-open-sent.md` | TL;DR | anchored to `peer.go`, `peer_run.go` | - | silent | Covers OpenSent FSM state; silent on Prometheus prefix metrics. |
| `docs/architecture/bgp/structural-forwarding.md` | Four costs, and what replaced them | anchored to `peer.go` | - | silent | Covers forwarding pool structural dedup; silent on Prometheus prefix metrics. |
| `docs/architecture/config/syntax.md` | TL;DR (Read This First) | anchored to `peer.go` | - | silent | Covers configuration file syntax; silent on Prometheus prefix metric labels. |
| `docs/architecture/encoding-context.md` | TL;DR (Read This First) | anchored to `peer.go` | - | silent | Covers per-peer `EncodingContext` and `ContextID`; silent on Prometheus prefix metrics. |
| `docs/architecture/forward-congestion-pool.md` | Invariant | anchored to `peer.go` | - | silent | Covers forward pool weighting and congestion backpressure; silent on prefix metric labels. |
| `docs/architecture/meta/role.md` | Keys Set | anchored to `peer.go`, `peer_removal.go` | - | silent | Covers RFC 9234 role metadata keys; silent on Prometheus prefix metrics. |
| `docs/architecture/plugin/plugin-system.md` | Where each layer lives | anchored to `peer.go`, `peer_run.go` | - | silent | Covers plugin registration and startup phases; silent on reactor Prometheus metric labels. |
| `docs/architecture/testing/ci-format.md` | Syntax Overview | anchored to `peer.go` | - | silent | Covers `.ci` runner directives; silent on Prometheus prefix metric label semantics. |
| `docs/architecture/update-building.md` | TL;DR (Read This First) | anchored to `peer.go` | - | silent | Covers outbound UPDATE construction; silent on inbound prefix metric labels. |
| `docs/architecture/wire/capabilities.md` | TL;DR (Read This First) | anchored to `peer.go` | - | silent | Covers wire BGP capability encoding; silent on Prometheus prefix metrics. |
| `docs/architecture/behavior/fsm-active.md` | TL;DR | anchored to `peer_session_factory.go`, `peer_run.go` | - | silent | Covers Active FSM state; silent on Prometheus prefix metrics. |
| `docs/architecture/behavior/fsm-connect.md` | TL;DR | anchored to `peer_run.go` | - | silent | Covers Connect FSM state; silent on Prometheus prefix metrics. |
| `docs/architecture/behavior/fsm-idle.md` | TL;DR | anchored to `peer_run.go` | - | silent | Covers Idle FSM state; silent on Prometheus prefix metrics. |
| `docs/architecture/bgp/protocol-event-capture.md` | Enabling it | anchored to `peer_run.go`, `peer_removal.go` | - | silent | Covers protocol event capture files; silent on Prometheus prefix metrics. |
| `docs/architecture/diagnostics/event-history-rings.md` | The decisions | anchored to `peer_run.go` | - | silent | Covers diagnostic event rings; silent on Prometheus prefix metrics. |
| `docs/architecture/edge-cases/extended-message.md` | TL;DR | anchored to `peer_run.go` | - | silent | Covers RFC 8654 extended message buffers; silent on Prometheus prefix metrics. |
| `docs/architecture/memory/lifetime-contracts.md` | TL;DR | anchored to `peer_run.go` | - | silent | Covers pooled read buffer lifetime contracts; silent on Prometheus prefix metrics. |
| `docs/architecture/mrt.md` | Packages | anchored to `peer_run.go` | - | silent | Covers MRT table/update dumps; silent on Prometheus prefix metrics. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | Preserves data-plane mismatch validation untouched; BGP prefix metrics are data-plane agnostic. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | Preserves the unified mismatch refusal path untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | Preserves VPP firewall verification untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | Preserves data-plane selector naming untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | Preserves egress next-hop chain semantics untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | Preserves `policychain.Sets` next-hop authoring untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | Preserves built-in next-hop engine steps untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | Preserves next-hop provenance folding untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | Preserves explicit next-hop validation untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | Preserves originated vs forwarded next-hop behavior untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | Preserves the implied-filter seam untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | Preserves daemon start resilience and reload refusal semantics untouched; metric labeling adds no startup refusal. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | Preserves start vs commit/reload mismatch behavior untouched. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | Changes no command paths or pipe operations. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | Changes no pipe overrides. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | Changes no pipe semantics. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | Preserves `\| implied` rendering untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | Preserves bus participant registration untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | Preserves `send > to` semantics untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | Preserves `receive > from` semantics untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | Preserves bus allow-list enforcement untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | Preserves bus filter compilation untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | Preserves bus non-arbitration untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | Preserves effective-bus display untouched. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | Adds no engine operation and preserves plugin authorization profiles untouched. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-25 | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | Preserves lossless event delivery untouched. |
| `docs/architecture/plugin/plugin-system.md` | Design decisions | decision of 2026-11-23 | "With no daemon running, a command that parses a config starts each declared external program in query mode, never its run loop, and validates against its declaration" | agrees | Preserves offline query-mode schema validation untouched. |
| `docs/architecture/update-building.md` | Design decisions | decision of 2026-11-23 | "Gate draft-simpson 0x0800 FlowSpec MP_REACH next-hop emission behind an explicit per-session opt-in (`session { flowspec-redirect-next-hop true; }`, default `false`), written automatically by `ze exabgp migrate` when a migrated neighbor uses Simpson actions or serves an API process on a FlowSpec family" | agrees | Preserves FlowSpec MP_REACH next-hop gating untouched. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Configuration sets per-family `prefix { count offered|installed; }` (defaulting to `offered`) on a BGP peer or group, and inbound BGP UPDATEs arrive on the session read goroutine or a config reload changes a family's `count` mode.
- Prometheus scrapes `/metrics` from the telemetry exporter.

### Transformation Path
1. `initReactorMetrics` (`reactor_metrics.go`) registers `ze_bgp_prefix_count`, `ze_bgp_prefix_ratio`, `ze_bgp_prefix_warning_exceeded`, and `ze_bgp_prefix_maximum_exceeded_total` with label slice `(peer, family, mode)`.
2. When `Peer` constructs a `Session` (`peer_session_factory.go`), it binds `session.prefixMetrics`, `session.prefixMetricsRetired` (pointing to the existing `Peer.stopping` atomic bool), and `session.onPrefixMetricFamily` (recording emitted `(family, mode)` entries on `Peer` once per family).
3. On each UPDATE section settlement in `session_prefix.go` (`applyPrefixDelta`, `applyPrefixCheck`, `reportPrefixExceeded`), the metric helpers check `s.prefixMetricsRetired` (one atomic load, no lock); if not retired, they resolve `mode := s.settings.prefixCountFor(famName).String()`, record `(famName, mode)` once per family on the peer, and update `.With(peerAddr, famName, mode)` for count, ratio, warning exceeded, and maximum exceeded.
4. When any peer is removed or restarted (`doRemovePeer`, `reconcilePeersJournaled` remove including any `PrefixCount` change between `offered` and `installed`, `reconcilePeersJournaled` add-undo rollback, or `removeDynamicPeer`):
   - Under `r.mu`, the shared `Reactor.removalLocked` (`peer_removal.go`) sets `peer.stopping.Store(true)` and invokes `deletePeerPrefixMetrics(r.rmetrics, peer)`, deleting all four mode-labeled vectors across every recorded and configured `(peer, family, mode)` plus `(peer, family)` on `prefixMaximum` and `prefixWarning`.
   - When the replacement peer is added (`addPeerLocked` -> `setPrefixConfigMetrics`), it publishes the new `prefixMaximum`/`prefixWarning`/`prefixStale` and deletes the opposite mode for every configured family so no stale mode series can remain.
   - When the outgoing peer's goroutine exits in `Peer.cleanup()` (`peer_run.go`), after `Session.Run` has returned, any `(peer, family, mode)` written in the narrow window before `peer.stopping` was observed is deleted only if no active peer in `r.peers` currently owns that `(peer, family, mode)`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Session read goroutine <-> Prometheus registry | Push-model `metrics.GaugeVec`/`metrics.CounterVec` `.With(peer, family, mode)` guarded by atomic `Peer.stopping` fence | [ ] |
| Config reload / peer removal <-> Prometheus registry | Synchronous `deletePeerPrefixMetrics` in `Reactor.removalLocked` under `r.mu` plus post-exit fenced cleanup in `Peer.cleanup` | [ ] |
| Prometheus exporter <-> external scrapers / dashboards | OpenMetrics/Prometheus text exposition on `/metrics` carrying `mode="offered"` or `mode="installed"` | [ ] |

### Changed-Wire Census

**Search evidence:** Searched `internal/component/bgp/reactor/` (`reactor_metrics.go`, `session_prefix.go`, `peer_removal.go`, `reactor_peers.go`, `reactor_api.go`), `docs/` (`git grep ze_bgp_prefix_`), and `test/` for all writers and readers of `ze_bgp_prefix_*` metrics. Positive control: `initReactorMetrics` (`reactor_metrics.go`) and `setPrefixCountMetric` (`session_prefix.go`) are the sole writers; `docs/guide/monitoring.md` and `reactor_metrics_test.go` are the documented in-repo readers; external Prometheus scrapers/PromQL queries are the outside-repo readers. No BGP protocol wire format, plugin RPC wire format, or CLI/JSON command schema changes.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Prometheus `/metrics` exposition for `ze_bgp_prefix_*` | writer | `internal/component/bgp/reactor/reactor_metrics.go` (`initReactorMetrics`) and `internal/component/bgp/reactor/session_prefix.go` (`setPrefixCountMetric`, `setPrefixWarningExceededMetric`, `incrPrefixExceededMetric`) | `git grep ze_bgp_prefix_` confirmed these are the only producers of the four vectors | Change: add closed `mode` label (`offered` or `installed`) and delete obsolete series on mode change and peer removal |
| Prometheus `/metrics` exposition for `ze_bgp_prefix_*` | reader | `internal/component/bgp/reactor/reactor_metrics_test.go` (`TestInitReactorMetricsRegistersAllMetrics`) and `internal/component/bgp/reactor/session_prefix_test.go` | `git grep ze_bgp_prefix_` confirmed these unit tests inspect registered vectors and values | Change: update unit tests and add `test/plugin/bgp-prefix-count-mode.ci` to assert `mode` label values and obsolete-series retirement |
| Prometheus `/metrics` exposition for `ze_bgp_prefix_*` | reader | External Prometheus scrapers, alerting rules, and dashboards documented in `docs/guide/monitoring.md`, `docs/guide/configuration.md`, `docs/guide/bgp-peering.md` | `git grep ze_bgp_prefix_ docs/` found metric tables in `docs/guide/monitoring.md` and `docs/features/configuration.md` | Change: update `docs/guide/monitoring.md` label table and document PromQL migration (`{peer, family}` selectors still match; `by (peer, family)` aggregations should include `mode` if separating modes) |

### Integration Points
- `reactorMetrics` (`internal/component/bgp/reactor/reactor_metrics.go`) - adds `metricLabelMode = "mode"` to the four count-derived metric vectors.
- `Session` metric helpers (`internal/component/bgp/reactor/session_prefix.go`) - stamps `s.settings.prefixCountFor(family).String()` and checks `s.prefixMetricsRetired`.
- `Peer` lifecycle (`internal/component/bgp/reactor/peer.go`, `peer_session_factory.go`, `peer_run.go`) - tracks emitted `(family, mode)` entries, fences retired sessions, and cleans residual obsolete series after `Session.Run` exits.
- Peer removal and reload (`internal/component/bgp/reactor/peer_removal.go`) - `Reactor.removalLocked` synchronously retires prefix metrics across all four peer-removal paths (`doRemovePeer`, `reconcilePeersJournaled` remove, `reconcilePeersJournaled` add-undo rollback, and `removeDynamicPeer`).

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Metrics remain registered in `reactorMetrics` and updated through `session_prefix.go` helpers behind `internal/core/metrics.Registry` |
| No unintended coupling (components stay isolated) | Yes | Confined to `internal/component/bgp/reactor/`; no plugin, LG, or CLI package imports reactor internals |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses `PrefixCountMode.String()` and `PeerSettings.prefixCountFor()` rather than introducing a second mode enum or string table |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Per-NLRI walk untouched; metric update runs once per family section per UPDATE with one atomic retirement check |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | No new command, plugin, or core switch; label set uses `PrefixCountMode` enum values already owned by BGP reactor |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | `git grep ze_bgp_prefix_` verified the only label list is `initReactorMetrics` in `reactor_metrics.go` and cleanup derives its mode values from `PrefixCountMode` (`PrefixCountOffered`, `PrefixCountInstalled`) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Changing `prefix { count offered|installed; }` restarts the peer via `reconcilePeersJournaled` | `peer_settings_apply.go` omits `PrefixCount` from `hotSwappableSettings` | Mode change would happen in-place without hitting the removal loop | unit test + `.ci` reload from `offered` to `installed` | confirmed (2026-11-26) |
| A-2 | `PrefixCountMode` has a closed two-value set (`offered`, `installed`) | `ze-bgp-conf.yang` `leaf count` enum and `peer_settings.go` `PrefixCountMode` | Unbounded label cardinality on Prometheus series | `peer_settings.go` `parsePrefixCountMode` rejects any other value | confirmed (2026-11-26) |
| A-3 | `metrics.GaugeVec.Delete` and `metrics.CounterVec.Delete` remove retired label tuples from `/metrics` | `internal/core/metrics/metrics.go` (`Delete(labelValues ...string) bool`) and `reactor_metrics.go` `setInfo` | Obsolete mode series would remain visible after reload | unit test with real Prometheus registry + `.ci` scrape after reload | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Asynchronous `Peer.Stop` lets an outgoing session emit one last UPDATE metric after `Reactor.removalLocked` deleted old series | Stale `mode="offered"` reappears in `/metrics` under load during reload | `Reactor.removalLocked` already sets `peer.stopping.Store(true)` synchronously under `r.mu` before `deletePeerPrefixMetrics` deletes series; check `s.prefixMetricsRetired` (`&p.stopping`) in every prefix metric helper, and run an owner-checked cleanup sweep in `Peer.cleanup()` after `Session.Run` returns |
| R-2 | `Peer.cleanup()` of a restarted peer with unchanged mode accidentally deletes the replacement peer's live series | New session's `ze_bgp_prefix_count` vanishes briefly after an unrelated reload (e.g. router-id change) | In `Peer.cleanup()`, delete a `(peer, family, mode)` series only when no active peer in `r.peers` at that address currently runs that same `(family, mode)` |
| R-3 | Existing PromQL alerts/dashboards using strict label matching break silently | Operator scrapes show `mode` label unexpectedly | Document the new `mode` label and query migration explicitly in `docs/guide/monitoring.md`, `docs/guide/configuration.md`, and `docs/guide/bgp-peering.md` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Prometheus `With(...)` label-count mismatch panics if any call site omits `mode`; stale series mislead operators after reload; enforcement behavior itself is untouched |
| How is it reverted? | Single commit revert restores the 2-label metric signatures; no persisted config or wire state changes |
| Who else touches this path? | `plan/release/fixes/spec-bgp-per-peer-received-counter.md` also edits `session_prefix.go`, `peer.go`, `peer_session_factory.go`, and `peer_run.go` on disjoint fields/functions |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Prometheus `/metrics` scrape with one family in `count offered` and one in `count installed`, followed by config reload switching `offered` to `installed` | → | `initReactorMetrics`, `setPrefixCountMetric`, `setPrefixWarningExceededMetric`, `incrPrefixExceededMetric`, `deletePeerPrefixMetrics` | `test/plugin/bgp-prefix-count-mode.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Two peer/family combinations are configured with `count offered` and `count installed`, and each receives routes crossing warning and maximum thresholds (with `teardown false`) | A `/metrics` scrape exposes `ze_bgp_prefix_count`, `ze_bgp_prefix_ratio`, `ze_bgp_prefix_warning_exceeded`, and `ze_bgp_prefix_maximum_exceeded_total` with `mode="offered"` on the offered family and `mode="installed"` on the installed family, carrying the exact measured values for each mode | Static config gauges (`ze_bgp_prefix_maximum`, `ze_bgp_prefix_warning`) and peer-level metrics (`ze_bgp_prefix_teardown_total`, `ze_bgp_prefix_stale`) intentionally omit `mode` because they do not depend on a per-family count mode |
| AC-2 | A live reload changes one peer/family from `count offered` to `count installed` (and in unit tests also `installed` to `offered`), and the peer re-establishes and advertises routes | A subsequent `/metrics` scrape exposes the four metrics under the new `mode` label and contains no series for the retired `(peer, family, old_mode)` tuple across count, ratio, warning exceeded, and maximum exceeded | Unchanged peer/family series retain their configured mode; peers that are removed retain no per-family prefix series |
| AC-3 | Concurrent UPDATEs arrive while a peer is removed or restarted with a changed count mode under `go test -race` | Race detector is clean, no `With(...)` panic occurs, and after `Peer.cleanup()` completes no obsolete `(peer, family, old_mode)` series lingers while any active replacement peer's `(peer, family, new_mode)` series remains intact | None: zero races and zero surviving obsolete series are permitted |
| AC-4 | Existing prefix-limit enforcement tests run across both `offered` and `installed` modes | Warning edges, report-bus events, Cease/MaxPrefixes NOTIFICATIONs, warn-only drops, and rollback semantics behave identically to before | None: zero behavioral regression in enforcement is permitted |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Scrapes `/metrics` for peers using `count offered` and `count installed` | YANG `count` leaf -> `PeerSettings.PrefixCount` -> `session_prefix.go` metric helpers -> Prometheus scrape | `test/plugin/bgp-prefix-count-mode.ci` |
| 2 | Reloads a family from `count offered` to `count installed` and scrapes `/metrics` | reload -> `reconcilePeersJournaled` -> `deletePeerPrefixMetrics` + retirement fence -> new peer/session -> Prometheus scrape | `test/plugin/bgp-prefix-count-mode.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestInitReactorMetricsRegistersAllMetrics` | `internal/component/bgp/reactor/reactor_metrics_test.go` | All four count-derived prefix vectors register with `(peer, family, mode)` labels | |
| `TestPrefixMetricsCarryCountModeLabel` | `internal/component/bgp/reactor/session_prefix_test.go` | AC-1: `offered` and `installed` families stamp `mode="offered"` / `mode="installed"` on count, ratio, warning exceeded, and maximum exceeded | |
| `TestPrefixModeReloadRetiresObsoleteSeries` | `internal/component/bgp/reactor/session_prefix_test.go` | AC-2/AC-3: reloading `offered` <-> `installed` and removing a peer deletes obsolete `(peer, family, old_mode)` series without deleting an active replacement peer's series, `-race` clean | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `mode` label value | closed enum `offered`, `installed` | `installed` | N/A (YANG enum rejects unknown strings in `parsePrefixCountMode`) | N/A |
| `prefixRatio` when `maximum == 0` | guarded (`maximum > 0`) | `maximum = 1` | `0` (ratio metric skipped, no divide-by-zero panic) | `uint32` max |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-prefix-count-mode` | `test/plugin/bgp-prefix-count-mode.ci` | Scrapes `/metrics` across `offered` and `installed` peer/family configurations, verifies all four count-derived metrics carry `mode`, reloads `offered` to `installed`, and proves obsolete series disappear while enforcement stays intact | |

### Interop Tests
- N-A: Prometheus label exposition changes no BGP wire bytes or peer protocol behavior.

## Files to Modify
- `internal/component/bgp/reactor/reactor_metrics.go` - add `metricLabelMode = "mode"` to `ze_bgp_prefix_count`, `ze_bgp_prefix_ratio`, `ze_bgp_prefix_warning_exceeded`, and `ze_bgp_prefix_maximum_exceeded_total`.
- `internal/component/bgp/reactor/session_prefix.go` - stamp `s.settings.prefixCountFor(family).String()` on the four count-derived metrics, check `s.prefixMetricsRetired`, track emitted `(family, mode)` entries, add `deletePeerPrefixMetrics`, and retire opposite-mode series in `setPrefixConfigMetrics`.
- `internal/component/bgp/reactor/peer.go` - add emitted `(family, mode)` tracking on `Peer` (reusing existing `Peer.stopping` atomic bool as the retirement fence).
- `internal/component/bgp/reactor/peer_session_factory.go` - wire `session.prefixMetricsRetired = &p.stopping` and metric-family tracking from `Peer` onto `Session`.
- `internal/component/bgp/reactor/peer_run.go` - run owner-checked residual prefix-metric cleanup in `Peer.cleanup()` after `Session.Run` exits.
- `internal/component/bgp/reactor/peer_removal.go` - call `deletePeerPrefixMetrics(r.rmetrics, peer)` in `Reactor.removalLocked` (right after `peer.stopping.Store(true)`), covering all four peer-removal paths (`doRemovePeer`, `reconcilePeersJournaled` remove, `reconcilePeersJournaled` add-undo rollback, and `removeDynamicPeer`).
- `internal/component/bgp/reactor/reactor_metrics_test.go` - update label expectations for the four prefix vectors.
- `internal/component/bgp/reactor/session_prefix_test.go` - add mode-label and reload-retirement unit tests.
- `docs/guide/monitoring.md` - update the Prefix Limits metric table with the `mode` label and add a PromQL migration note.
- `docs/guide/configuration.md` - note that per-family `prefix { count offered|installed; }` appears as the `mode` label on count-derived Prometheus metrics.
- `docs/guide/bgp-peering.md` - note the `mode` label on prefix-limit metrics under the per-family prefix limits section.

## Files to Create
- `test/plugin/bgp-prefix-count-mode.ci` - functional test verifying `mode` labels on `/metrics` and obsolete-series cleanup across a live reload.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | `prefix { count offered\|installed; }` already exists in `internal/component/bgp/yang/ze-bgp-conf.yang` |
| YANG validation constraints | No | Existing `enumeration { enum offered; enum installed; }` already constrains the mode |
| YANG custom validators | No | Native YANG enum suffices |
| CLI commands/flags | No | No CLI command added or changed |
| CLI grammar (keyword before value) | No | No CLI grammar change |
| Editor autocomplete | No | Existing enum leaf already autocompletes |
| Functional test for new RPC/API | Yes | `test/plugin/bgp-prefix-count-mode.ci` exercises live `/metrics` scrape and reload |
| Pipe completeness | No | Prometheus exposition endpoint, not a CLI command |
| Env var registration | No | No env var added |
| Doctor check for runtime dependencies | No | No new file path, socket, port, service, or external dependency |
| Doctor code per problem class | No | No new refusal, warning, error log class, or report-bus event |
| Prometheus counters/metrics | Yes | Adds `mode` (`offered` / `installed`) label to `ze_bgp_prefix_count`, `ze_bgp_prefix_ratio`, `ze_bgp_prefix_warning_exceeded`, `ze_bgp_prefix_maximum_exceeded_total` in `reactor_metrics.go` |
| BGP family surface (new SAFI / capability / attribute) | No | No new family, capability, or path attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Corrects missing label identity on existing prefix-limit metrics; `docs/features/configuration.md` lists metric names only (unchanged) |
| 2 | Config syntax changed? | No | `prefix { count offered\|installed; }` syntax is unchanged; `docs/guide/configuration.md` receives a note connecting `count` to the metric `mode` label |
| 3 | CLI command added/changed? | No | No CLI command changes |
| 4 | API/RPC added/changed? | No | No API/RPC changes |
| 5 | Plugin added/changed? | No | No plugin added or changed |
| 6 | Has a user guide page? | Yes | `docs/guide/monitoring.md`, `docs/guide/configuration.md`, `docs/guide/bgp-peering.md` |
| 7 | Wire format changed? | No | No BGP wire format change |
| 8 | Plugin SDK/protocol changed? | No | No SDK or plugin protocol change |
| 9 | RFC behavior implemented, changed, or newly proven? | No | RFC 4486 enforcement behavior is unchanged |
| 10 | Test infrastructure changed? | No | Uses existing `.ci` runner |
| 11 | Affects daemon comparison? | No | No comparison row changes |
| 12 | Internal architecture changed? | No | Push-model `metrics.Registry` and peer lifecycle unchanged |
| 13 | Route metadata keys added/changed? | No | No route metadata keys touched |
| 14 | Prometheus counters added/changed? | Yes | `docs/guide/monitoring.md` (Prefix Limits table and PromQL migration note) |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registration inventory change |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Checked via `./le spec citation anchors spec plan/release/fixes/spec-bgp-prefix-count-mode-metric.md`; existing architecture pages remain accurate because metric label changes do not alter reactor/FSM/lifecycle contracts |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/monitoring.md`, `docs/guide/configuration.md`, `docs/guide/bgp-peering.md` |

## Design presentation

### What changes

The four count-derived BGP prefix-limit Prometheus metrics (`ze_bgp_prefix_count`, `ze_bgp_prefix_ratio`, `ze_bgp_prefix_warning_exceeded`, and `ze_bgp_prefix_maximum_exceeded_total`) gain a closed `mode` label carrying `"offered"` or `"installed"`, matching the per-family `prefix { count offered|installed; }` configuration and the existing enforcement error log. When a peer is removed or reloaded with a different count mode, obsolete `(peer, family, old_mode)` series are deleted and late writes from the stopping session are fenced so scrapes always reflect only the active mode.

### Before and after

1. **Scraping two peers/families with different count modes:**
   - **Before:** `/metrics` reports `ze_bgp_prefix_count{family="ipv4/unicast",peer="10.0.0.1"} 5` and `ze_bgp_prefix_count{family="ipv4/unicast",peer="10.0.0.2"} 5` (and likewise for `ze_bgp_prefix_ratio`, `ze_bgp_prefix_warning_exceeded`, `ze_bgp_prefix_maximum_exceeded_total`) with no indication that `10.0.0.1` counts offered wire events while `10.0.0.2` counts an installed identity set.
   - **After:** `/metrics` reports `ze_bgp_prefix_count{family="ipv4/unicast",mode="offered",peer="10.0.0.1"} 5` and `ze_bgp_prefix_count{family="ipv4/unicast",mode="installed",peer="10.0.0.2"} 5`, and the same `mode` label appears on `ze_bgp_prefix_ratio`, `ze_bgp_prefix_warning_exceeded`, and `ze_bgp_prefix_maximum_exceeded_total`.
2. **Reloading `10.0.0.1` `ipv4/unicast` from `count offered` to `count installed`:**
   - **Before:** old per-family series lingered in `/metrics` because neither `doRemovePeer` nor `Reactor.removalLocked` deleted them.
   - **After:** all four `mode="offered"` series for `(10.0.0.1, ipv4/unicast)` are deleted on reload and replaced by `mode="installed"` series once the re-established session receives routes, with no stale `mode="offered"` series left behind.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Separate metric names per mode (`ze_bgp_prefix_offered_count` vs `ze_bgp_prefix_installed_count`) | Splits one logical metric family across two names, complicates cross-peer alerting, and breaks existing metric-name queries more severely than adding a closed 2-value label |
| Adding `mode` to static/peer-level metrics (`ze_bgp_prefix_maximum`, `ze_bgp_prefix_warning`, `ze_bgp_prefix_teardown_total`, `ze_bgp_prefix_stale`) | Static limits and peer-level staleness/teardown counters are not derived from the per-family count mode; adding `mode` there would multiply series without semantic meaning |
| Waiting for post-release `spec-bgp-peer-metric-labels.md` optional descriptive labels | Thomas ruled on 2026-11-24 to ship count-mode identity as a v1.0 release fix now so scrapes do not mislead operators, independent of optional high-cardinality descriptive labels |

### Documents departed from

none

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- add `metricLabelMode = "mode"` to the four vectors in `reactor_metrics.go`, pass `mode` at all three `session_prefix.go` emitters, and add the failing `test/plugin/bgp-prefix-count-mode.ci` test plus `TestPrefixMetricsCarryCountModeLabel` and `TestPrefixModeReloadRetiresObsoleteSeries`.
   - Tests: `TestInitReactorMetricsRegistersAllMetrics`, `TestPrefixMetricsCarryCountModeLabel`, `TestPrefixModeReloadRetiresObsoleteSeries`, `test/plugin/bgp-prefix-count-mode.ci`
   - Files: `reactor_metrics.go`, `session_prefix.go`, `reactor_metrics_test.go`, `session_prefix_test.go`, `test/plugin/bgp-prefix-count-mode.ci`
   - Verify: mode label appears on all four metrics; reload test fails red until series retirement is wired.
2. **Phase: Series retirement and race safety** -- add emitted `(family, mode)` tracking on `Peer`, wire `session.prefixMetricsRetired = &p.stopping` through `peer_session_factory.go`, call `deletePeerPrefixMetrics(r.rmetrics, peer)` in `Reactor.removalLocked` (`peer_removal.go`), delete opposite mode in `setPrefixConfigMetrics`, and perform owner-checked post-exit cleanup in `Peer.cleanup()` (`peer_run.go`).
   - Tests: `TestPrefixModeReloadRetiresObsoleteSeries` under `-race`, `test/plugin/bgp-prefix-count-mode.ci`
   - Files: `peer.go`, `peer_session_factory.go`, `peer_run.go`, `peer_removal.go`, `session_prefix.go`
   - Verify: unit tests and `.ci` pass green; `-race` clean; existing prefix-limit enforcement tests unchanged.
3. **Phase: Documentation** -- update `docs/guide/monitoring.md`, `docs/guide/configuration.md`, and `docs/guide/bgp-peering.md` with the `mode` label and PromQL migration note.
   - Files: `docs/guide/monitoring.md`, `docs/guide/configuration.md`, `docs/guide/bgp-peering.md`
   - Verify: `./le doc check all` clean.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-4 has file:line implementation and passing proof |
| Correctness | Every `.With(...)` and `.Delete(...)` call on the four vectors supplies exactly 3 labels `(peer, family, mode)` so Prometheus never panics |
| Race safety | `Peer.stopping` retirement fence (`session.prefixMetricsRetired`) and `Peer.cleanup()` owner check prevent both late stale writes and accidental deletion of a replacement peer's active series |
| Naming | Label constant is `metricLabelMode = "mode"` and values come exclusively from `PrefixCountMode.String()` (`offered`, `installed`) |
| Hot path | No new lock or per-NLRI allocation added to `checkPrefixLimits` or `forEachPrefixEntry` |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Four count-derived vectors carry `mode` label | `git grep -n 'metricLabelMode' internal/component/bgp/reactor/` |
| Unit tests pass under `-race` | `./le test unit pkg path internal/component/bgp/reactor/ run 'Test(InitReactorMetrics|Prefix)'` |
| End-to-end `/metrics` scrape and reload retirement | `./le test functional plugin` (`test/plugin/bgp-prefix-count-mode.ci`) |
| Docs updated and anchors clean | `./le doc check all` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Cardinality / resource exhaustion | `mode` is strictly closed to `"offered"` and `"installed"` from YANG enum; obsolete series are deleted on reload and peer removal so metric memory stays bounded |
| Untrusted input | No peer-controlled wire string reaches the `mode` label |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Prometheus `inconsistent label cardinality` panic | Audit every `.With(...)` and `.Delete(...)` on `prefixCount`, `prefixRatio`, `prefixWarningExceeded`, `prefixExceededTotal` |
| Stale `mode="offered"` survives reload in `.ci` | Check `Reactor.removalLocked` (`peer_removal.go`), `Peer.stopping` retirement fence, and `Peer.cleanup()` sweep |
| Replacement peer series deleted after reload | Check owner/mode guard in `Peer.cleanup()` against active `r.peers` |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Add closed `mode` label (`offered`/`installed`) to the 4 count-derived metrics only | Separate metric names per mode; labeling static limit/teardown/stale metrics | Preserves metric family identity, matches `reportPrefixExceeded` `"mode"` log field, avoids meaningless cardinality on static gauges |
| Retire obsolete series via `deletePeerPrefixMetrics` in `Reactor.removalLocked` + `Peer.stopping` fence + owner-checked `Peer.cleanup()` sweep | Delete only in `doRemovePeer`; wait synchronously for `Peer.Stop` under `r.mu` | `Reactor.removalLocked` (`peer_removal.go`) is the single locked choke point for all four peer-removal paths (`doRemovePeer`, `reconcilePeersJournaled` remove, `reconcilePeersJournaled` add-undo rollback, `removeDynamicPeer`) and already sets `peer.stopping.Store(true)`; `Peer.Stop` is async (blocking under `r.mu` stalls all peers), so fencing + post-exit owner-checked sweep closes the race with zero hot-path lock |

## Known Limitations
- Pre-policy `routes-received` exposure on CLI and Looking Glass is owned separately by `plan/release/fixes/spec-bgp-per-peer-received-counter.md`.
- Optional descriptive peer labels (`description`, `hostname`, `peer-type`, `peer-group`) and native collector selection remain post-release in `plan/post-release/features/spec-bgp-peer-metric-labels.md`.

## Review Gate
Pending implementation and independent `/ze-review` + `/ze-review-spec`.

## Checklist

### Pre-Spec Verification
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior, Existing Mechanisms, Design documents, Data Flow, and Design presentation completed
- [ ] AC-N rows carry testable positive assertions and Permitted Survivors
- [ ] `./le spec design check spec plan/release/fixes/spec-bgp-prefix-count-mode-metric.md` reports only `no judge has judged this design`

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Scoped tests for changed packages and `test/plugin/bgp-prefix-count-mode.ci` pass in the main checkout
- [ ] Feature code integrated (`internal/component/bgp/reactor/`)
- [ ] Integration and Documentation checklists answered Yes/No with evidence
- [ ] Architectural Verification table filled
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric/enum inputs
- [ ] Functional `.ci` test for end-to-end behavior

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `./le spec review record`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-bgp-prefix-count-mode-metric.md` only, in the same `./le commit create` script
