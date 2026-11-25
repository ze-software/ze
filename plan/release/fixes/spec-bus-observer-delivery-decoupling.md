# Spec: Decouple External Observer Event Delivery From BGP Forwarding

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | plugin |
| Priority | P1 |
| Release reason | Q-E7 benchmark on Apple M4 Max (2026-11-25) proved that 5 to 20 external `ze exabgp plugin` observers subscribing to `bgp:update` saturate the fixed 64-slot `Process.Deliver` channel, filling `deliverChan`, stalling the BGP peer TCP read goroutine, and degrading live BGP forwarding throughput by 58% to 84% (`628k -> 263k -> 102k r/s`). |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-25 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Replace the fixed 64-slot `Process.eventChan` channel with a bounded, dynamically growable, **100% lossless** FIFO delivery queue (`64` initial slots growing up to a configurable high-water ceiling — default `65,536` for non-cache-consumer endpoints, `256` for `CacheConsumer` plugins — and shrinking back to `64` once drained) and increase `deliveryLoop`'s per-RPC batch drain cap from `64` to `512` events. This absorbs full-table bursts (~15,625 packed BGP UPDATEs per 1,000,000 routes, sharing one `fmtCache` formatted string across all endpoints) without blocking `onMessageBatchReceived`, `deliverChan`, `reactorForwardRS`, sibling `DirectBridge` routing plugins (`bgp-rs`, `adj-rib-in`, `bgp-rib`), or `onMessageSent`, while strictly honoring the owner's zero-drop invariant and exposing queue depth and backpressure waits through Prometheus metrics and `ze doctor`.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/plugin-infra.md`, `ai/doors/bgp-engine.md`, `ai/doors/config-yang.md`, `ai/doors/core.md`, `ai/doors/system-plugins.md`
  → Constraint: `pkg/plugin/rpc` and `pkg/plugin/sdk` wire framing stays unchanged; external and internal plugins share the same FIFO delivery contract (`ai/doors/plugin-infra.md`).
  → Constraint: Cache-consumer forwarding plugins (`bgp-rs`, `bgp-rr`) rely on cumulative/tracked cache acknowledgements and tight forward-path backpressure (`ai/doors/bgp-engine.md`, Traps).
  → Constraint: Every capacity-planning tunable is a YANG leaf (`delivery-queue-max` on `bus > endpoint`), not an env-only knob (`ai/patterns/config-option.md`, `ai/rules/config.md`).
  → Constraint: Long-lived worker on a queue, zero per-event goroutines (`ai/rules/goroutine-lifecycle.md`).
- [ ] `docs/architecture/api/process-protocol.md`
  → Decision: Owner ruling (2026-11-25, `## Design decisions`): engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly.
  → Constraint: `DrainEvents` / `DrainEventsApplied` FIFO barrier semantics and `SendDeliverBatch` RPC framing must remain unchanged.
- [ ] `docs/architecture/exabgp-bridge.md`
  → Decision: Q-E7 baseline (`fb9fce5cb8`) established 4-vCPU saturation and lockstep blocking at 5 and 20 scripts (`628k -> 263k -> 102k r/s` on 1M routes, ~15,625 packed UPDATEs).
- [ ] `plan/learned/DESIGN-HISTORY.md` (BGP engine, "Backpressure on the forward path is deliberately IMPLICIT")
  → Constraint: Tight implicit backpressure for `CacheConsumer` forwarding plugins (`bgp-rs`, `bgp-rr`) must be preserved via their tight default ceiling (`256`).
- [ ] `plan/post-release/engineering/spec-review-bus-async-fanout.md` and `plan/release/fixes/spec-bgp-session-write-wedge-plugin-parked-writer.md`
  → Constraint: No second queue beside the per-plugin queue; extend the single per-process queue in `internal/component/plugin/process/` so every emitter (`onMessageBatchReceived`, `onMessageSent`, `deliverPeerLine`, `Server.deliverEvent`) benefits uniformly.

**Key insights:**
- A 1,000,000-route full table in `./le perf` is ~15,625 packed BGP UPDATE messages; `fmtCache` (`events.go:436-444`) formats each event once so all 20 external endpoints share one immutable `string` backing array, meaning 15,625 queued items cost only ~1 MiB of ring slots per script (~20 MiB total across 20 scripts) plus one shared string copy.
- Today's static 64-slot `eventChan` fills in <0.2 ms and caps every `SendDeliverBatch` RPC at 64 events (~244 synchronous TLS round-trips per script per 1M routes); growing to `65,536` with a `512`-event batch drain eliminates lockstep blocking during full-table bursts and cuts TLS round-trips 8x while keeping idle memory at 64 slots (~3 KiB).
- Backpressure waits at the ceiling are lossless (never drop) and become observable via `ze_plugin_delivery_queue_depth`, `ze_plugin_delivery_backpressure_waits_total`, and `ze doctor` (`CodeDoctorPluginDeliveryFailed`).

## Current Behavior (MANDATORY)

**Source files read:** (2026-11-25)
- [ ] `internal/component/bgp/reactor/reactor_notify.go` (lines 955-1070) - runs `reactorForwardRS` on the peer TCP read goroutine, then blocks on `deliverChan <- deliveryItem` (cap 256 in `internal/component/bgp/reactor/delivery.go:26`).
- [ ] `internal/component/bgp/reactor/peer_run.go` (lines 670-780) - per-peer delivery worker drains `deliverChan` and calls `receiver.OnMessageBatchReceived`, then `recentUpdates.Activate`.
- [ ] `internal/component/bgp/server/events.go` (lines 140-1369) - `onMessageBatchReceived` formats JSON once (`fmtCache`) and loops over all matching `procs` calling `proc.Deliver` (`Result == nil`); `onMessageSent` and `deliverPeerLine` also call `proc.Deliver` (`Result == nil`); `deliverPeerState`, `onEORReceived`, `deliverToProcs`, and single `onMessageReceived` pass non-nil `Result`.
- [ ] `internal/component/plugin/process/delivery.go` (lines 1-497) - fixed `eventDeliveryCapacity = 64` channel; `Process.DeliverContext` blocks at 64 items; `deliveryLoop` drains at most 64 items per `SendDeliverBatch`.
- [ ] `internal/component/plugin/process/process.go` (lines 125-145), `delivery_context_test.go`, `recovery_delivery_export_test.go` - `Process` holds `eventChan chan EventDelivery`; two package tests construct or swap `eventChan` directly.
- [ ] `internal/component/plugin/process/manager.go` (lines 72-168), `metric_hooks.go` (lines 1-64), `doctor_records.go` (lines 1-370), `internal/component/plugin/doctor/checks_delivery.go` (lines 240-340) - tracks `ze_plugin_events_delivered_total`, `ze_plugin_delivery_failures_total`, `ze_plugin_delivery_slow_batches_total`, and `DeliveryWindowSummary`; no queue-depth or backpressure-wait observability exists.
- [ ] `internal/component/plugin/yang/ze-plugin-conf.yang` (lines 172-316), `internal/component/plugin/types.go` (lines 595-660), `internal/component/config/bus_validate.go` (lines 25-310), `internal/component/plugin/server/reload_declarations.go` (lines 120-200) - parses `bus > endpoint` leaves into `plugin.PluginConfig` and checks immutable declaration leaves on reload.

**Behavior to preserve:**
- Zero dropped events on engine-to-plugin delivery under all conditions (owner ruling, 2026-11-25).
- Strict FIFO ordering, `Barrier`, `DrainEvents`, `DrainEventsApplied`, `TestSessionDownFollowsBufferedDeliveries`, and `PeerUpBarrier`.
- Prompt cancellation of blocked `DeliverContext` calls on caller `ctx.Done()` or `p.ctx.Done()`, leaving prior queued items intact and preserving FIFO order.
- Tight backpressure bound (`256`) for `CacheConsumer` plugins (`bgp-rs`, `bgp-rr`) unless explicitly overridden by operator `delivery-queue-max`.
- Unchanged `SendDeliverBatch` / `MuxConn` wire framing and unchanged `DirectBridge` `StructuredEvent` pooling.

**Behavior to change:**
- Replace fixed 64-slot `eventChan` in `Process` with a bounded, dynamically growable lossless FIFO ring queue (`eventQueue`: initial cap `64`, ceiling `65,536` for non-cache-consumer endpoints and `256` for `CacheConsumer` plugins unless overridden via `delivery-queue-max`, shrinking back to `64` once drained empty).
- Drain up to `512` events per `deliverBatch` call (respecting `CallBatchRPC`'s 16 MiB split).
- Add optional `delivery-queue-max` YANG leaf (`uint32`, range `64..1048576`) on `bus > endpoint`.
- Add `ze_plugin_delivery_queue_depth` gauge, `ze_plugin_delivery_backpressure_waits_total` counter, and recent-window backpressure wait reporting in `DeliveryWindowSummary` and `ze doctor` (`CodeDoctorPluginDeliveryFailed`).

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Per-plugin delivery queue and worker | `internal/component/plugin/process/delivery.go`, `Process.Deliver`, `DeliverContext`, `deliveryLoop` | Fixed 64-slot Go channel `eventChan` with single batch-draining worker | Replaces internal queue container with growable lossless `eventQueue` inside `Process` while preserving the single worker and public `Deliver`/`DeliverContext`/`DrainEvents` contract |
| Per-peer async delivery channel | `internal/component/bgp/reactor/peer_run.go`, `deliverChan` (`deliveryChannelCapacity = 256`) | Buffers bursts between TCP reader and `onMessageBatchReceived` | Preserves unchanged; deep per-process ring prevents external observers from stalling the worker draining it |
| `CallBatchRPC` oversized frame splitter | `pkg/plugin/rpc/conn.go`, `Conn.CallBatchRPC` | Automatically splits `deliver-batch` frames exceeding `rpc.MaxMessageSize` (16 MiB) into sub-batches | Extends use: raising `deliverBatch` drain cap from 64 to 512 relies safely on this existing splitter |
| Plugin delivery metrics and doctor window | `internal/component/plugin/process/manager.go` (`pluginMetrics`), `metric_hooks.go`, `doctor_records.go` (`DeliveryWindowSummary`), `internal/component/plugin/doctor/checks_delivery.go` (`deliveryFailedRows`) | Counts and reports failed and slow (>500ms) batches under `CodeDoctorPluginDeliveryFailed` | Extends: adds `ze_plugin_delivery_queue_depth`, `ze_plugin_delivery_backpressure_waits_total`, and backpressure wait counts in `DeliveryWindowSummary` / `ze doctor` |
| Bus endpoint YANG config and reload declaration guard | `internal/component/plugin/yang/ze-plugin-conf.yang`, `internal/component/config/bus_validate.go`, `internal/component/plugin/types.go` (`PluginConfig`), `internal/component/plugin/server/reload_declarations.go` | Defines, extracts, and validates `bus > endpoint` process/transport leaves | Extends: adds optional `delivery-queue-max` leaf (`range 64..1048576`) extracted into `PluginConfig.DeliveryQueueMax` |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | A plugin can run in another process | the door every change reads | "Every plugin, ours included, runs either internal (a goroutine in the daemon) or external (a forked subprocess that talks only over TLS and RPC)." | agrees | The growable lossless `eventQueue` sits inside `Process` and serves both `DirectBridge` and TLS RPC identically. |
| `ai/doors/plugin-infra.md` | Boundaries | the door covering `internal/component/plugin/*` | "A compiled-in and an external plugin differ only in how they reach the bus." | agrees | Both internal and external endpoints use the same `eventQueue` implementation and FIFO barrier semantics; `CacheConsumer` plugins take the tighter `256` default based on declared cache-consumer role, not transport. |
| `ai/doors/config-yang.md` | Boundaries | the door covering `internal/component/config/bus_validate.go` | "A tunable defaults to a YANG leaf, and env-only is the exception." | agrees | Exposes `delivery-queue-max` as a validated YANG leaf (`uint32`, range `64..1048576`) on `bus > endpoint`. |
| `ai/doors/le-tooling.md` | Where it sits | the door covering `test/parse/bus-delivery-queue-max.ci` and `test/plugin/bus-delivery-queue-burst-lossless.ci` | "Functional `.ci` files live under `test/<suite>/` and run through `./le test <suite>` on the functional runner" | agrees | New `.ci` tests live under `test/parse/` and `test/plugin/` and follow the standard `.ci` format. |
| `docs/architecture/api/process-protocol.md` | Event Delivery | `// Design:` of `types.go`, `process.go`, `delivery.go`, `manager.go`, `metric_hooks.go`, `doctor_records.go`, `delivery_context_test.go`; modified by this spec | "Events that pass both halves are enqueued into a per-process channel. The delivery goroutine drains all available events into a batch and sends them in a single `deliver-batch` RPC, reducing syscalls and goroutine churn." | agrees | Extends the per-process queue from fixed 64-slot channel to growable lossless ring (`64 -> 65,536`, `256` for `CacheConsumer`) and drains up to `512` events per `deliver-batch` RPC; page updated in the same change. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | Adds no engine operation and preserves bus filter authorization unchanged. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-25 | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | Strictly lossless: `eventQueue` grows up to `maxCapacity` and backpressures at the ceiling; zero events are ever dropped. |
| `docs/architecture/doctor-and-health-checks.md` | Plugin registration and delivery checks | `// Design:` of `doctor_records.go` and `checks_delivery.go`; modified by this spec | "the batches that failed with no caller waiting, and the batches over 500 ms, in the last five minutes (`DeliveryWindow`)" | agrees | Extends `plugin-delivery` (`doctor-plugin-delivery-failed`) and this table to also report ceiling backpressure waits in `DeliveryWindow`. |
| `docs/architecture/hub-architecture.md` | What the daemon does at start | `// Design:` of `reload_declarations.go` and `reload_declarations_bus_test.go`; modified by this spec | "A reload (SIGHUP, `request reload`, a commit, a managed push) reads a config whose external plugins differ from the ones the daemon started with: one added, one removed, or one whose `run`, `encoder`, `respawn` or `timeout` changed" | agrees | Adds `delivery-queue-max` to the immutable external-endpoint declaration set checked by `RefuseChangedDeclarations` and updates this page alongside it. |
| `docs/architecture/exabgp-bridge.md` | Measured per-script cost (Q-E7) | anchored to `reload_declarations.go`, `delivery.go`; modified by this spec | "Each script adds roughly `~1.6–1.8 µs` of CPU per received UPDATE across TLS send/receive, `json.Unmarshal` into `map[string]any`, wire attribute decoding, ExaBGP JSON formatting, and pipe write" | agrees | Removes lockstep 64-slot channel blocking and 64-event RPC round-trips for ExaBGP endpoints and updates the Q-E7 note on this page. |
| `docs/architecture/exabgp-bridge.md` | Design decisions | decision of 2026-11-23 | "An ExaBGP script is an ordinary bus endpoint that runs the wrapper; the bridge config section and the internal bridge mode do not exist" | agrees | `delivery-queue-max` is a generic `bus > endpoint` leaf, not an ExaBGP-specific section or runner. |
| `docs/plugin-development/metrics.md` | Naming Policy | modified by this spec | "Every metric name is built from four parts in fixed order:" | agrees | New metrics `ze_plugin_delivery_queue_depth` (GaugeVec) and `ze_plugin_delivery_backpressure_waits_total` (CounterVec) follow the `ze_plugin_delivery_*` family and naming rules. |
| `docs/architecture/api/architecture.md` | The model | `// Design:` of `bus_validate.go`, `bus_validate_test.go`; anchored to `types.go`, `process.go` | "Participant = any party on the bus, named by its configuration path" | agrees | `delivery-queue-max` sits on `bus > endpoint` without altering participant paths, filter compilation, or send/receive grant semantics. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 (`implied` inline comments) | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | Does not touch `\| implied` rendering. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 (participant registration) | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | Preserves participant registration and default routing unchanged. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 (`send > to`) | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | Preserves `send > to` semantics unchanged. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 (`receive > from`) | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | Preserves `receive > from` semantics unchanged. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 (`allow`-lists) | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | Preserves `send > allow` and `receive > allow` enforcement unchanged. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 (compiled filter) | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | `delivery-queue-max` is a process queue leaf on `bus > endpoint`, outside filter compilation. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 (no exclusive-role arbitration) | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | Adds no arbitration between participants. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 (effective display) | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | Does not alter bus effective-filter display. |
| `docs/architecture/plugin/rib-storage-design.md` | Source-policy lifecycle owners | `// Design:` of `recovery_delivery_export_test.go`; anchored to `delivery.go` | - | silent | Anchored only via `recovery_delivery_export_test.go`'s FIFO delivery gate helper; `eventQueue` preserves identical FIFO barrier and drain ordering. |
| `docs/architecture/config/syntax.md` | TL;DR (Read This First) | anchored to `ze-plugin-conf.yang` | - | silent | Syntax grammar (braces, leaf syntax) is unchanged; `delivery-queue-max` is an ordinary `uint32` leaf. |
| `docs/architecture/api/commands.md` | Overview | anchored to `types.go`, `manager.go` | - | silent | No CLI command path, verb, or pipe operator changes. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | Adds no CLI command or selector syntax. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | Touches no pipe operation or command answer shape. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | Touches no pipe operation. |
| `docs/architecture/api/ipc_protocol.md` | Overview | anchored to `types.go`, `process.go` | - | silent | Line framing and answer framing on `MuxConn` are unchanged. |
| `docs/architecture/api/json-format.md` | Overview | anchored to `types.go` | - | silent | JSON event payloads are unchanged by Spec 1. |
| `docs/architecture/cli/command-verbs.md` | The axis: does the command change the system | anchored to `types.go` | - | silent | No CLI verb added or modified. |
| `docs/architecture/cli/error-surface.md` | The rule | anchored to `types.go` | - | silent | Error surface format unchanged; out-of-range `delivery-queue-max` uses existing YANG range refusal and `opererr`. |
| `docs/architecture/config/transaction-protocol.md` | 1. Design Principles | anchored to `types.go` | - | silent | Config transaction protocol (`verify`, `apply`, `rollback`) is unchanged. |
| `docs/architecture/core-design.md` | Executive Summary | anchored to `types.go` | - | silent | Component boundaries and core architecture are unchanged. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 (VPP kernel mismatch) | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | Does not touch data-plane or VPP/kernel validation. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 (one mismatch verdict) | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | Preserves uniform validation across start, commit, reload, and `ze config validate`. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 (VPP firewall verifier) | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | Does not touch firewall or VPP verifiers. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 (kernel vs vpp selector) | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | Does not touch data-plane selectors. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 (egress next-hop rewrite) | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | Does not touch BGP next-hop rewriting. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 (policy modify next-hop) | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | Does not touch BGP policy chains. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 (built-in next-hop engine steps) | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | Does not touch BGP policy chains. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 (next-hop raw override provenance) | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | Does not touch BGP next-hop provenance. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 (explicit next-hop IPv4/IPv6) | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | Does not touch BGP policy chains. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 (forwarded vs originated next-hop) | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | Does not touch BGP next-hop rewriting. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 (implied filter display) | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | Does not touch filter-chain implied display. |
| `docs/architecture/encoding-context.md` | TL;DR (Read This First) | anchored to `types.go` | - | silent | Encoding contexts and wire forwarding are unchanged. |
| `docs/architecture/plugin-manager-wiring.md` | Problem | anchored to `types.go` | - | silent | Plugin manager startup wiring is unchanged; metrics registration extends existing `SetMetricsRegistry`. |
| `docs/architecture/web-interface.md` | Source Files | anchored to `types.go` | - | silent | Web UI is unaffected; YANG leaf is discovered automatically by schema-driven config editor. |
| `docs/architecture/plugin/plugin-system.md` | Where each layer lives | anchored to `reload_declarations.go`, `process.go`, `manager.go` | - | silent | Plugin registration and process-boundary rules are unchanged; `eventQueue` is internal to `plugin/process`. |
| `docs/architecture/plugin/plugin-system.md` | Design decisions | decision of 2026-11-23 | "With no daemon running, a command that parses a config starts each declared external program in query mode, never its run loop, and validates against its declaration" | agrees | Offline validation of `delivery-queue-max` happens through YANG and `bus_validate.go` with unchanged declare-mode query behavior. |
| `docs/architecture/api/capability-contract.md` | Implementation Status | anchored to `process.go` | - | silent | Capability declaration and OPEN injection are unchanged. |
| `docs/architecture/config/environment.md` | Overview | anchored to `process.go` | - | silent | No new environment variable is added (`delivery-queue-max` is a YANG leaf). |
| `docs/architecture/ike/ipsec-7-ikev2-engine.md` | Decisions | anchored to `process.go` | - | silent | IKE engine is unaffected; inherits lossless `eventQueue` transparently. |
| `docs/architecture/plugin/show-enrichers.md` | The decisions | anchored to `process.go` | - | silent | Show enrichers use RPC callbacks, not the event queue. |
| `docs/architecture/testing/ci-format.md` | Syntax Overview | anchored to `process.go` | - | silent | `.ci` test runner syntax is unchanged. |
| `docs/architecture/update-cache.md` | Purpose | anchored to `process.go` | - | silent | `CacheConsumer` plugins retain tight `256` default queue bound so `RecentUpdateCache` lifetime is unchanged. |
| `docs/architecture/decisions/001-pull-model-metrics.md` | Consequences | anchored to `manager.go` | "New Ze Prometheus metrics should follow the registry-backed push model unless this decision is explicitly reopened." | agrees | `ze_plugin_delivery_queue_depth` and `ze_plugin_delivery_backpressure_waits_total` register on the existing push-model `metrics.Registry` in `ProcessManager.SetMetricsRegistry` and update through `processMetricHooks`. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config load: `bus { endpoint <name> { delivery-queue-max <n>; ... } }` parsed by `internal/component/config/bus_validate.go` into `plugin.PluginConfig.DeliveryQueueMax`.
- Runtime events: `onMessageBatchReceived`, `onMessageSent`, `deliverPeerLine`, `deliverPeerState`, `onEORReceived`, `deliverToProcs`, `onMessageReceived`, `Server.deliverEvent`, and `Process.DrainEvents` / `DrainEventsApplied`.

### Transformation Path
1. `bus_validate.go` parses optional `delivery-queue-max` (`64..1048576`) into `PluginConfig.DeliveryQueueMax` (`0` = default).
2. `Process.startDeliveryLocked` initializes `p.eventQueue` with initial backing slice `64` and effective ceiling (`DeliveryQueueMax` if set; else `256` when `p.IsCacheConsumer()`, else `65,536`). `SetCacheConsumer(true)` updates the default ceiling to `256` when no explicit `DeliveryQueueMax` was configured.
3. `Process.Deliver` / `DeliverContext` acquires the queue lock:
   - If `len < maxCapacity`, appends to the ring (growing `2x` up to `maxCapacity` as needed), signals `deliveryLoop`, updates `ze_plugin_delivery_queue_depth`, and returns `true` without blocking.
   - If `len == maxCapacity`, increments `ze_plugin_delivery_backpressure_waits_total`, records a backpressure wait in `Process.doctor`, and waits cleanly for space, `ctx.Done()`, `p.ctx.Done()`, or queue close.
4. `Process.deliveryLoop` wakes on signal, drains up to `512` items into `batchBuf`, shrinks the backing slice back to `64` when the ring drains empty from `cap > 256`, updates `ze_plugin_delivery_queue_depth`, and dispatches via `DirectBridge` or `SendDeliverBatch`.
5. `Process.stopEventChan` closes the queue, wakes all blocked callers (returning `false`), and drains any un-delivered items after `deliveryLoop` exits (returning pooled `*rpc.StructuredEvent` and notifying `Result` / `OnFailure`).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config YANG ↔ PluginConfig | `ze-plugin-conf.yang` -> `bus_validate.go` -> `plugin.PluginConfig.DeliveryQueueMax` | No |
| BGP EventDispatcher / EventBus ↔ Plugin Process | `Process.Deliver` / `DeliverContext` -> `eventQueue` (lossless FIFO ring) | No |
| Plugin Process ↔ External Plugin | Unchanged `SendDeliverBatch` over TLS `MuxConn` (batches up to 512 events, split at 16 MiB by `CallBatchRPC`) | No |

### Changed-Wire Census

**Search evidence:** Searched `SendDeliverBatch`, `CallBatchRPC`, and `deliver-batch` across `internal/component/plugin/` and `pkg/plugin/` (positive controls: `internal/component/plugin/ipc/rpc.go:339`, `pkg/plugin/rpc/conn.go:586`, `pkg/plugin/sdk/sdk.go:731`). No plugin RPC wire format, frame delimiter, JSON field, or BGP wire encoding changes. Only the in-daemon queue container and the `ze-plugin-conf.yang` `bus > endpoint > delivery-queue-max` config leaf are added.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `ze-plugin-callback:deliver-batch` (unchanged wire) | writer | `internal/component/plugin/ipc/rpc.go` (`PluginConn.SendDeliverBatch`) + `pkg/plugin/rpc/conn.go` (`Conn.CallBatchRPC`) | `delivery.go:385` calls `conn.SendDeliverBatch`; `CallBatchRPC` already splits frames above `MaxMessageSize` (16 MiB) | Preserve wire format unchanged; only batch count increases from 64 to 512 |
| `ze-plugin-callback:deliver-batch` (unchanged wire) | reader | `pkg/plugin/sdk/sdk.go` (SDK batch handler) and third-party external plugins | Reads JSON array of event strings per `docs/architecture/api/process-protocol.md` | Preserve unchanged; already accepts variable-size batches up to `MaxMessageSize` |

### Integration Points
- `internal/component/plugin/process/event_queue.go` - new lossless growable ring queue used by `Process`.
- `internal/component/plugin/process/delivery.go` & `process.go` - replaces `eventChan` field with `eventQueue`.
- `internal/component/plugin/process/manager.go` & `metric_hooks.go` - registers `ze_plugin_delivery_queue_depth` and `ze_plugin_delivery_backpressure_waits_total`.
- `internal/component/plugin/process/doctor_records.go` & `internal/component/plugin/doctor/checks_delivery.go` - records and renders recent backpressure waits.
- `internal/component/plugin/yang/ze-plugin-conf.yang`, `internal/component/config/bus_validate.go`, `internal/component/plugin/server/reload_declarations.go` - config leaf, validation, and reload handling.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | All events continue through `Process.Deliver` -> `deliveryLoop` -> `DirectBridge` / `SendDeliverBatch` |
| No unintended coupling (components stay isolated) | Yes | Queue change stays inside `internal/component/plugin/process`; `plugin` still imports nothing from `bgp` |
| No duplicated functionality (extends existing, does not recreate) | Yes | Single per-process queue replaces `eventChan`; no second queue or fan-out worker |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Shared `fmtCache` strings and pooled `*rpc.StructuredEvent` pointers are stored by reference, never copied |
| Registration over hardcoding, outbound | Yes | Metrics register through existing `ProcessManager.SetMetricsRegistry`; YANG leaf loads through existing `ze-plugin-conf.yang` |
| Registration over hardcoding, inbound | Yes | Searched `bus_validate.go` and `reload_declarations.go` for `on-exit` / `encoding` / `timeout`; `delivery-queue-max` joins the same endpoint extractor and declaration comparator |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A 1,000,000-route full table in `./le perf` produces ~15,625 packed BGP UPDATEs, well below the `65,536` default ceiling | Observed `message-id=15619` in Mac Q-E7 1M run (`fb9fce5cb8`) | Unpacked 1-prefix-per-UPDATE feeds (1M individual UPDATEs) exceed 65,536 and will hit lossless backpressure unless operator raises `delivery-queue-max` | Document `delivery-queue-max` (up to `1048576`) for unpacked full-table feeds | confirmed |
| A-2 | `pkg/plugin/rpc/conn.go` (`CallBatchRPC`) safely handles 512-event batches even when serialized JSON exceeds 16 MiB | Read `conn.go:586-634` (`TestConn_CallBatchRPC_OversizedSplit`) | Oversized batch write fails instead of splitting | Verified in `conn.go:604-632`; run `go test ./pkg/plugin/rpc/...` | confirmed |
| A-3 | `SetCacheConsumer(true)` is called during Stage 1 registration before peer traffic arrives, so adjusting the default ceiling to `256` takes effect before any burst | Checked `rs/server.go:1120`, `rr/rr.go:1297`, `startup.go` Stage 1 | Cache-consumer could run with 65,536 ceiling if `SetCacheConsumer` did not update the live queue ceiling | Have `SetCacheConsumer` call `eventQueue.SetMaxCapacity` under lock when no explicit `DeliveryQueueMax` was configured | confirmed |
| A-4 | On a 4-vCPU host with 20 JSON-parsing `ze exabgp plugin` wrappers, CPU contention across the 20 OS processes remains until Spec 2 eliminates JSON/reflection overhead | Q-E7 Mac physics analysis (`fb9fce5cb8`) | Expecting Spec 1 alone to restore 628k r/s at 20 scripts on 4 vCPUs would misread the CPU bottleneck | Documented in Spec 1 AC-6 and paired with Spec 2 (`spec-exabgp-raw-text-event-transport.md`) | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Blocked `DeliverContext` misses `ctx.Done()` or leaks a waiter goroutine/channel when waking from ceiling backpressure | `TestDeliveryContextCancelRetainsFIFO` hangs or `-race` reports goroutine leak | Use per-waiter buffered notification channel (`chan struct{}` cap 1) in a FIFO waiter list under mutex so `select` waits directly on `<-waitCh`, `<-ctx.Done()`, and `<-p.ctx.Done()` with zero goroutines and zero polling |
| R-2 | Ring backing array holds 65,536 slots (~3 MiB) forever after a burst | Elevated idle RSS after burst drains | Shrink backing slice back to `64` slots when ring drains to `len == 0` and `cap > 256` |
| R-3 | `StructuredEvent` pool leak or hung `Result` waiter if queue closes with buffered items | Race/leak test failure on `Stop` during burst | On `Close()` / `deliveryLoop` exit, drain remaining buffered items, call `rpc.PutStructuredEvent` on any `*rpc.StructuredEvent`, and notify `Result` (`ErrConnectionClosed`) or `OnFailure` |
| R-4 | Updating `p.eventChan`-touching unit tests (`delivery_context_test.go`, `recovery_delivery_export_test.go`, `delivery_test.go`) accidentally weakens their mutation assertions | Test judge refusal or weakened mutation test | Preserve exact `deliveryAdmissionContext`, `RecoveryDeliveryGate`, and FIFO/mutation semantics through package-private queue test helpers; run independent `ze-test-judge` before commit |
| R-5 | Permanently wedged plugin fills all `65,536` slots and stalls producers | `ze_plugin_delivery_backpressure_waits_total` rises and `ze doctor` reports backpressure waits | Existing `deliveryTimeout` (5 s) + `writeWatchdog` (30 s) fails/closes the wedged connection and unblocks producers |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A bug in `eventQueue` wakeup/close logic could deadlock `Process.Deliver` or lose FIFO order on `DrainEvents` barriers; caught immediately by `-race` unit tests and `test/plugin` functional tests |
| How is it reverted? | Single commit revert; `delivery-queue-max` is optional with safe defaults |
| Who else touches this path? | `internal/component/plugin/process`, `internal/component/plugin/doctor`, `internal/component/config`, `internal/component/plugin/server`; coordinates with `spec-exabgp-raw-text-event-transport.md` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `bus { endpoint <name> { delivery-queue-max 1024; ... } }` config parse | → | `bus_validate.go` -> `PluginConfig.DeliveryQueueMax` -> `Process.startDeliveryLocked` -> `eventQueue` ceiling | `test/parse/bus-delivery-queue-max.ci` |
| High-rate BGP UPDATE burst with paused external plugin | → | `onMessageBatchReceived` -> `Process.Deliver` -> growable `eventQueue` -> `deliveryLoop` 512-batch drain | `test/plugin/bus-delivery-queue-burst-lossless.ci` |
| Blocked `DeliverContext` at queue ceiling | → | `eventQueue.enqueue` backpressure wait -> `ze_plugin_delivery_backpressure_waits_total` + `DeliveryWindowSummary` -> `ze doctor` | `TestDeliveryBackpressureWaitReportedByDoctor` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Non-cache-consumer `Process` with paused consumer receives 16,000 sequential `Deliver` calls | All 16,000 `Deliver` calls return `true` immediately without blocking; when consumer resumes, all 16,000 events arrive in exact FIFO order in batches up to 512 with 0 drops | None: zero-drop invariant forbids any missing or reordered event |
| AC-2 | `DeliverContext` is called when `eventQueue` is at `maxCapacity` and caller cancels `ctx` before space frees | `DeliverContext` returns `false` promptly, increments `ze_plugin_delivery_backpressure_waits_total`, leaves all previously admitted items intact in FIFO order, and admits subsequent calls once space frees | None: blocked caller must never leak or corrupt FIFO order |
| AC-3 | `Process` with `SetCacheConsumer(true)` and no explicit `DeliveryQueueMax` | Effective queue ceiling is `256` (tight forward-path backpressure); explicit `DeliveryQueueMax` overrides it when configured | Explicit operator `delivery-queue-max` survives because operator config takes precedence over default |
| AC-4 | `eventQueue` grows past `256` during a burst and then drains completely empty | Backing ring slice shrinks back to `64` slots (`cap == 64`), and `ze_plugin_delivery_queue_depth` returns to `0` | None: burst slab must not be retained permanently after drain |
| AC-5 | `bus { endpoint <name> { delivery-queue-max <n>; } }` parsed by `ze config validate` and daemon boot | Accepts `64..1048576` into `PluginConfig.DeliveryQueueMax`, refuses `< 64` and `> 1048576` with YANG range error, and refuses live reload change naming `delivery-queue-max` | Unset leaf (`0`) remains permitted and selects the role-appropriate default (`65536` or `256`) |
| AC-6 | Producer blocks at queue ceiling (`maxCapacity`) on a slow plugin | Increments `ze_plugin_delivery_backpressure_waits_total`, records the wait in `DeliveryWindowSummary`, and `ze doctor` reports a `CodeDoctorPluginDeliveryFailed` warning naming the backpressure wait count | Non-blocked bursts below ceiling emit zero backpressure warnings because no producer stalled |
| AC-7 | Real external plugin receives a >256-event BGP UPDATE burst while temporarily paused (`bus-delivery-queue-burst-lossless.ci`) | BGP peer finishes sending without stalling at 64 events and the external plugin receives every single UPDATE in order with 0 drops | None: zero-drop lossless delivery is required end to end |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures `delivery-queue-max 4096;` on a `bus` endpoint and validates | `ze-plugin-conf.yang` -> `bus_validate.go` -> `PluginConfig` -> `Process.eventQueue` | `test/parse/bus-delivery-queue-max.ci` |
| 2 | Runs an external observer endpoint during a >256-UPDATE burst | Peer read -> `deliverChan` -> `onMessageBatchReceived` -> `Process.eventQueue` -> `deliveryLoop` (512-batch) -> external plugin | `test/plugin/bus-delivery-queue-burst-lossless.ci` |
| 3 | Runs `ze doctor` after a wedged plugin hits its queue ceiling | `eventQueue` wait -> `noteDeliveryBackpressure` -> `DoctorRecord.Delivery` -> `deliveryFailedRows` | `TestDeliveryBackpressureWaitReportedByDoctor` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEventQueueGrowDrainShrinkFIFO` | `internal/component/plugin/process/event_queue_test.go` | AC-1, AC-4: grows from 64 to 16,384, drains in 512-item FIFO batches, shrinks back to 64 when empty | |
| `TestEventQueueBackpressureAndCancel` | `internal/component/plugin/process/event_queue_test.go` | AC-2: blocks at ceiling, cancels cleanly on `ctx.Done()` and `procCtx.Done()`, preserves FIFO under `-race` | |
| `TestEventQueueCloseCleansUpPooledEvents` | `internal/component/plugin/process/event_queue_test.go` | R-3: closing with buffered `StructuredEvent` and `Result` items recycles events and wakes all waiters | |
| `TestDeliveryContextCancelRetainsFIFO` | `internal/component/plugin/process/delivery_context_test.go` | AC-2: updated seam for `eventQueue`, retains exact mutation contract | |
| `TestProcessCacheConsumerDefaultQueueBound` | `internal/component/plugin/process/delivery_test.go` | AC-3: `SetCacheConsumer(true)` sets default ceiling 256; non-cache-consumer defaults to 65,536; explicit `DeliveryQueueMax` wins | |
| `TestBusValidateDeliveryQueueMax` | `internal/component/config/bus_validate_test.go` | AC-5: extracts valid `delivery-queue-max` and rejects out-of-range values | |
| `TestReloadRefusesChangedDeliveryQueueMax` | `internal/component/plugin/server/reload_declarations_bus_test.go` | AC-5: reload refusal names `delivery-queue-max` | |
| `TestDeliveryBackpressureWaitReportedByDoctor` | `internal/component/plugin/doctor/checks_delivery_test.go` | AC-6: doctor warning names backpressure waits under `CodeDoctorPluginDeliveryFailed` | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `delivery-queue-max` | `64..1048576` | `64`, `1048576` | `63`, `0` | `1048577` |
| `deliverBatch` drain cap | `1..512` | `512` | N/A (internal const) | `513` (splits into second batch) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bus-delivery-queue-max` | `test/parse/bus-delivery-queue-max.ci` | Accepts valid `delivery-queue-max` (`64`, `65536`, `1048576`), refuses `63` and `1048577` | |
| `bus-delivery-queue-burst-lossless` | `test/plugin/bus-delivery-queue-burst-lossless.ci` | External plugin receives a 300-route burst without stalling at 64 events and loses 0 routes | |

### Interop Tests (Scope: plugin)
Not applicable: no BGP wire protocol behavior changes. Q-E7 Mac `./le perf run dut ze` re-measurement at 0/1/5/20 scripts provides the before/after throughput evidence.

## Files to Modify
- `internal/component/plugin/yang/ze-plugin-conf.yang` - add optional `delivery-queue-max` leaf (`uint32`, range `64..1048576`) under `bus > endpoint`.
- `internal/component/plugin/types.go` - add `DeliveryQueueMax int` to `PluginConfig` plus default/bound constants (`DefaultDeliveryQueueMax = 65536`, `DefaultCacheConsumerQueueMax = 256`, `MinDeliveryQueueMax = 64`, `MaxDeliveryQueueMax = 1048576`).
- `internal/component/config/bus_validate.go` - extract and validate `delivery-queue-max` into `PluginConfig.DeliveryQueueMax`.
- `internal/component/plugin/server/reload_declarations.go` - compare `DeliveryQueueMax` in `externalDeclarationsEqual` and name `delivery-queue-max` in `changedLeaves`.
- `internal/component/plugin/process/process.go` - replace `eventChan chan EventDelivery` with `eventQueue *eventQueue` and wire `SetCacheConsumer` default ceiling update.
- `internal/component/plugin/process/delivery.go` - route `Deliver`, `DeliverContext`, `deliveryLoop` (batch drain up to `512`), `DrainEvents`, `DrainEventsApplied`, `startDeliveryLocked`, and `stopEventChan` through `eventQueue`.
- `internal/component/plugin/process/manager.go` - register `ze_plugin_delivery_queue_depth` gauge and `ze_plugin_delivery_backpressure_waits_total` counter.
- `internal/component/plugin/process/metric_hooks.go` - add `deliveryQueueDepth` and `deliveryBackpressure` callbacks on `processMetricHooks`.
- `internal/component/plugin/process/doctor_records.go` - add `BackpressureWaits int` to `DeliveryWindowSummary` and `noteBackpressureWait()` recorder.
- `internal/component/plugin/doctor/checks_delivery.go` - include backpressure wait counts in `deliveryFailedRows` (`CodeDoctorPluginDeliveryFailed`).
- `internal/component/plugin/process/delivery_context_test.go`, `internal/component/plugin/process/recovery_delivery_export_test.go`, `internal/component/plugin/process/delivery_test.go` - adapt direct `eventChan` access to `eventQueue` test helpers preserving exact assertions.
- `internal/component/plugin/server/reload_declarations_bus_test.go`, `internal/component/config/bus_validate_test.go`, `internal/component/plugin/doctor/checks_delivery_test.go` - unit tests for config, reload, and doctor reporting.
- `docs/architecture/api/process-protocol.md` - document growable lossless delivery queue, 512-event batch drain, and `delivery-queue-max`.
- `docs/architecture/doctor-and-health-checks.md` - add backpressure wait reporting to `plugin-delivery` (`doctor-plugin-delivery-failed`).
- `docs/architecture/hub-architecture.md` - add `delivery-queue-max` to the external-endpoint declaration leaves listed under `What the daemon does at start` (`RefuseChangedDeclarations`).
- `docs/plugin-development/metrics.md` - document `ze_plugin_delivery_queue_depth` and `ze_plugin_delivery_backpressure_waits_total`.
- `docs/guide/plugins.md` - document `delivery-queue-max` endpoint leaf.
- `docs/architecture/exabgp-bridge.md` - update Q-E7 follow-up note with lossless queue sizing.

## Files to Create
- `internal/component/plugin/process/event_queue.go` - bounded growable lossless FIFO ring queue with cancellable backpressure wait and shrink-on-empty.
- `internal/component/plugin/process/event_queue_test.go` - unit and `-race` tests for grow, drain, shrink, backpressure wait, cancel, and close cleanup.
- `test/parse/bus-delivery-queue-max.ci` - parse/range validation functional test for `delivery-queue-max`.
- `test/plugin/bus-delivery-queue-burst-lossless.ci` - end-to-end lossless burst delivery functional test.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/plugin/yang/ze-plugin-conf.yang` (`bus > endpoint > delivery-queue-max`) |
| YANG validation constraints | Yes | `uint32` with `range "64..1048576"` in `ze-plugin-conf.yang` |
| YANG custom validators | N-A | Native `uint32` + `range` constraint is sufficient |
| CLI commands/flags | N-A | No new CLI verb or flag |
| CLI grammar (keyword before value) | N-A | No new CLI command |
| Editor autocomplete | Yes | Automatic from YANG leaf `delivery-queue-max` in `ze-plugin-conf.yang` |
| Functional test for new RPC/API | Yes | `test/parse/bus-delivery-queue-max.ci`, `test/plugin/bus-delivery-queue-burst-lossless.ci` |
| Pipe completeness | N-A | No new show/RPC output |
| Env var registration | N-A | Not under `environment/`; per-endpoint YANG leaf only |
| Doctor check for runtime dependencies | N-A | No new file path, socket, binary, or kernel dependency |
| Doctor code per problem class | Yes | Backpressure waits at queue ceiling report under existing `CodeDoctorPluginDeliveryFailed` (`checks_delivery.go`, runtime phase) |
| Prometheus counters/metrics | Yes | `ze_plugin_delivery_queue_depth` (gauge, label `plugin`) and `ze_plugin_delivery_backpressure_waits_total` (counter, label `plugin`) in `internal/component/plugin/process/manager.go` |
| BGP family surface (new SAFI / capability / attribute) | N-A | No new BGP family, capability, or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Internal queueing reliability fix + optional endpoint tuning leaf; no feature level change |
| 2 | Config syntax changed? | Yes | `docs/guide/plugins.md` (bus endpoint `delivery-queue-max` leaf) |
| 3 | CLI command added/changed? | No | No CLI command changed |
| 4 | API/RPC added/changed? | No | `SendDeliverBatch` RPC wire format unchanged |
| 5 | Plugin added/changed? | No | Plugin infrastructure only; no plugin added |
| 6 | Has a user guide page? | Yes | `docs/guide/plugins.md` |
| 7 | Wire format changed? | No | No BGP or RPC wire format change |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC requirement changed |
| 10 | Test infrastructure changed? | No | Uses existing `.ci` runner |
| 11 | Affects daemon comparison? | No | Stock 0-script baseline unchanged; Q-E7 overlay note updated in `docs/architecture/exabgp-bridge.md` |
| 12 | Internal architecture changed? | Yes | `docs/architecture/api/process-protocol.md`, `docs/architecture/exabgp-bridge.md` |
| 13 | Route metadata keys added/changed? | No | No route metadata change |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No new registration or event type |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived via `./le spec citation anchors`: modified `docs/architecture/api/process-protocol.md`, `docs/architecture/doctor-and-health-checks.md`, `docs/architecture/hub-architecture.md`, `docs/architecture/exabgp-bridge.md`, `docs/plugin-development/metrics.md`, `docs/guide/plugins.md`; declared-unaffected: `docs/architecture/api/architecture.md` (bus participant/filter model unchanged by queue sizing leaf) and `docs/architecture/plugin/rib-storage-design.md` (FIFO recovery gate ordering preserved unchanged) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Verify `bus > endpoint` examples in `docs/guide/plugins.md` |

## Design presentation

### What changes
Replaces each plugin process's fixed 64-slot `eventChan` with a bounded, dynamically growable lossless FIFO ring queue (`64` initial slots growing to `65,536` for non-cache-consumer endpoints and `256` for `CacheConsumer` plugins, shrinking back to `64` once drained), increases `deliveryLoop` batch draining from `64` to `512` events per `SendDeliverBatch` RPC, exposes an optional `delivery-queue-max` YANG leaf (`64..1048576`) on `bus > endpoint`, and reports queue depth and backpressure waits via Prometheus (`ze_plugin_delivery_queue_depth`, `ze_plugin_delivery_backpressure_waits_total`) and `ze doctor`. Zero events are ever dropped.

### Before and after
1. **Full-table burst with external observers (e.g. `ze exabgp plugin`):**
   - *Before:* Fixed 64-slot `eventChan` fills in <0.2 ms; `onMessageBatchReceived` blocks every 64 UPDATEs, filling `deliverChan` (256 slots) and stalling `perf-sender` / `reactorForwardRS` in lockstep with 64-event synchronous TLS RPC round-trips.
   - *After:* Growable lossless ring absorbs the entire ~15,625-UPDATE full-table burst in ~1 MiB of shared-string ring slots per endpoint without blocking `onMessageBatchReceived` or `deliverChan`, draining in 512-event batches (8x fewer TLS round-trips) with 0 dropped events, then shrinking back to 64 slots.
2. **Configuring a custom queue bound on a bus endpoint:**
   - *Before:* No config leaf existed; 64 slots was hardcoded.
   - *After:* Operator may optionally set `delivery-queue-max 131072;` inside `bus { endpoint <name> { ... } }`; values outside `64..1048576` are refused by YANG validation.
3. **Diagnosing a genuinely wedged plugin (`ze doctor` & Prometheus):**
   - *Before:* A producer parked on a full 64-slot queue left no queue-depth metric or backpressure-wait record.
   - *After:* `ze_plugin_delivery_queue_depth{plugin="<name>"}` shows current depth, `ze_plugin_delivery_backpressure_waits_total{plugin="<name>"}` counts ceiling waits, and `ze doctor` reports backpressure waits alongside slow/failed batches under `doctor-plugin-delivery-failed`.

### Alternatives set aside
| Alternative | Why set aside |
|-------------|---------------|
| Drop-on-full overflow policy | Rejected by owner ruling (2026-11-25): endpoints expect every subscribed network event losslessly; any drop is unacceptable |
| Two-tier split queue (separate external fan-out goroutine) | Adds a second queue beside `Process`'s delivery queue, complicating `DrainEvents` / `sessionDown` ordering and duplicating fan-out for `onMessageSent` and `Server.deliverEvent` |
| Static 65,536-slot Go channel (`make(chan EventDelivery, 65536)`) | Permanently allocates ~3.1 MiB per idle plugin (~300+ MiB wasted across ~100 compiled-in plugins with no peers) |

### Documents departed from
none

### Choices for Thomas
| # | Choice | Options | Decision |
|---|--------|---------|----------|
| C-1 | Queue architecture | (a) Growable lossless ring `64 -> 65,536` (`256` for `CacheConsumer`) + 512 batch drain; (b) two-tier split queue; (c) static 65,536 channel | (a) Approved at Design gate (2026-11-25) |
| C-2 | Configurability | (a) Optional `delivery-queue-max` YANG leaf (`64..1048576`); (b) internal constants only | (a) Approved at Design gate (2026-11-25) |

## Implementation Steps
1. **Phase: Wiring (MANDATORY FIRST)** -- add `delivery-queue-max` to `ze-plugin-conf.yang`, `PluginConfig.DeliveryQueueMax`, `bus_validate.go`, and `reload_declarations.go`; regenerate YANG glue if needed; write failing parse/validation/reload tests.
   - Tests: `TestBusValidateDeliveryQueueMax`, `TestReloadRefusesChangedDeliveryQueueMax`, `test/parse/bus-delivery-queue-max.ci`
   - Files: `internal/component/plugin/yang/ze-plugin-conf.yang`, `internal/component/plugin/types.go`, `internal/component/config/bus_validate.go`, `internal/component/plugin/server/reload_declarations.go`
   - Verify: `go test -race ./internal/component/config/... ./internal/component/plugin/server/...` and `./le test functional parse` on `bus-delivery-queue-max.ci`
2. **Phase: Growable Lossless Ring Queue** -- implement `internal/component/plugin/process/event_queue.go` and wire it into `Process` (`process.go`, `delivery.go`), raising batch drain cap to `512` and preserving `CacheConsumer` `256` default bound, `Barrier`, `DrainEvents`, `DrainEventsApplied`, and `DeliverContext` cancellation.
   - Tests: `TestEventQueueGrowDrainShrinkFIFO`, `TestEventQueueBackpressureAndCancel`, `TestEventQueueCloseCleansUpPooledEvents`, `TestDeliveryContextCancelRetainsFIFO`, `TestProcessCacheConsumerDefaultQueueBound`
   - Files: `internal/component/plugin/process/event_queue.go`, `event_queue_test.go`, `process.go`, `delivery.go`, `delivery_context_test.go`, `recovery_delivery_export_test.go`, `delivery_test.go`
   - Verify: `go test -race ./internal/component/plugin/process/...`
3. **Phase: Metrics, Doctor, Functional Proof & Docs** -- add `ze_plugin_delivery_queue_depth`, `ze_plugin_delivery_backpressure_waits_total`, `DeliveryWindowSummary.BackpressureWaits`, `checks_delivery.go` warning, `test/plugin/bus-delivery-queue-burst-lossless.ci`, and update docs (`process-protocol.md`, `doctor-and-health-checks.md`, `hub-architecture.md`, `metrics.md`, `plugins.md`, `exabgp-bridge.md`).
   - Tests: `TestDeliveryBackpressureWaitReportedByDoctor`, `test/plugin/bus-delivery-queue-burst-lossless.ci`, plus Q-E7 Mac re-measurement
   - Files: `manager.go`, `metric_hooks.go`, `doctor_records.go`, `checks_delivery.go`, `checks_delivery_test.go`, docs, `.ci` test
   - Verify: `go test -race ./internal/component/plugin/...`, `./le test functional plugin` on `bus-delivery-queue-burst-lossless.ci`, `./le doc check all`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-7 has implementation + passing test |
| Correctness | Zero dropped events on any path; `DeliverContext` cancels cleanly at ceiling without losing FIFO order |
| Naming | YANG leaf `delivery-queue-max`, Go field `DeliveryQueueMax`, metrics `ze_plugin_delivery_queue_depth` and `ze_plugin_delivery_backpressure_waits_total` |
| Data flow | Single per-process queue; no second queue or per-event goroutine |
| Operator errors and doctor codes | Out-of-range `delivery-queue-max` refused by YANG; backpressure waits report under `CodeDoctorPluginDeliveryFailed` |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Growable lossless `eventQueue` with 512 batch drain | `go test -race ./internal/component/plugin/process/...` |
| `delivery-queue-max` YANG leaf + reload check | `go test -race ./internal/component/config/...` + `test/parse/bus-delivery-queue-max.ci` |
| Queue depth & backpressure metrics + doctor row | `go test -race ./internal/component/plugin/doctor/...` |
| End-to-end lossless burst proof | `test/plugin/bus-delivery-queue-burst-lossless.ci` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion (OOM) | Queue growth is strictly bounded by `delivery-queue-max` (`<= 1,048,576`, default `65,536`) and shrinks back to `64` on empty drain |
| Input validation | YANG `uint32` range `64..1048576` rejects `0`, negative, and unbounded values at parse time |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| `-race` failure or deadlock in `eventQueue` | Inspect lock order and waiter channel signaling in `event_queue.go` |
| Existing `test/plugin` regression | Check `DrainEvents` / `Barrier` / `stopEventChan` close-drain behavior |

## Design Insights
- Recorded owner zero-drop ruling in `docs/architecture/api/process-protocol.md` (`## Design decisions`, 2026-11-25): engine-to-plugin event delivery never drops events.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Bounded growable lossless ring (`64 -> 65,536`, `256` for `CacheConsumer`) + `512` batch drain | Drop-on-full overflow; two-tier split queue; static 65,536 Go channel | Satisfies owner zero-drop ruling, absorbs 1M-route packed bursts (~15.6k UPDATEs) without stalling BGP forwarding or wasting idle RAM, keeps one FIFO rail |
| Optional `delivery-queue-max` YANG leaf (`64..1048576`) on `bus > endpoint` | Hardcoded internal constant | Lets operators tune burst headroom for unpacked full-table feeds through validated YANG config |

## Known Limitations
- On CPU-constrained hosts (e.g. 4 vCPUs with 20 JSON-parsing `ze exabgp plugin` wrappers), OS process CPU contention during burst drain is addressed by Spec 2 (`plan/release/fixes/spec-exabgp-raw-text-event-transport.md`), which removes the 20x JSON/reflection overhead.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist and Documentation Update Checklist answered completely

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Scoped unit and functional tests pass in main checkout
- [ ] Feature code integrated (`internal/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for `delivery-queue-max`
- [ ] Functional `.ci` tests for end-to-end behavior

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/release/fixes/spec-bus-observer-delivery-decoupling.md`
