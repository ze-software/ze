# Spec: Traffic Usage Monitor (lazy aggregation service + `monitor traffic`)

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-observation-feed -- LANDED in commit 293ab1400 (`internal/core/observation.Feed`; spec being closed) |
| Phase | 1/8 |
| Updated | 2026-06-29 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-observation-feed.md` - the layer-2 dispatch this consumes (in-flight; `internal/core/observation/observation.go` may still change)
4. Source: `internal/core/observation/observation.go` (the feed), `internal/component/iface/rate.go` (per-interface rates + tick fan-out), `internal/plugins/trafficusage/{show.go,monitor.go}` (per-IP/port collector), `internal/plugins/ddos/detect/{register.go,detector.go}` (the Depth-1 refactor target), `internal/component/cli/model_dashboard.go` + `internal/component/iface/cmd/interface_rate.go` (TUI + streaming-handler patterns to mirror)

## Task

Ze can already render almost everything a live traffic monitor (glances / NetHawk) shows, but the
data is scattered across collectors and, worse, the **aggregation of it is entangled with DDoS
detection**: the per-interface rate aggregation and rolling baseline live inside `ddos/detect`
(`detector.go:70-129`), so the "time-based usage" view cannot be obtained without starting the
detection logic. The correct breakdown is three layers: **collection** (eBPF / conntrack / rate
tracker, already exist), **dispatch** (the new `observation.Feed`, just built), and **aggregation /
time-based usage** (per-second rates, top-N talkers, protocol mix, rolling window, severity) -- which
does not exist as a standalone, reusable thing. **Action** (detect + respond) is a separate fourth
concern that should merely *consume* the usage layer.

This spec builds the missing aggregation layer as a **lazy, consumer-reference-counted service** that
subscribes to the `observation.Feed`, maintains a time-windowed ranked usage view, and runs only
while it has at least one consumer. It then delivers the first consumers -- a `monitor traffic`
full-screen CLI, a `show traffic` one-shot, and a `ze-monitor:traffic` stream -- and refactors
`ddos/detect` to take its per-interface rate input from this service instead of re-deriving it from
the raw tick (Depth 1). A hardcoded `/etc/services`-style port→name table (a new core leaf) labels
the ports in the view.

### Goals
1. A standalone usage-aggregation service usable with **zero DDoS logic running** (proves the layer split).
2. `monitor traffic [name <iface>]` shows live per-interface rate, top talkers (IP + port), protocol mix, severity -- glances/NetHawk-style.
3. The service is a reusable substrate: CLI, `show`, stream, and (future) analytics / detectors all consume the same snapshot.
4. `ddos/detect` consumes the service for its rate input (Depth 1); no behavioural regression.
5. Ports are labeled by a shared, hardcoded port→service-name table.

### Agreed framing (settled with user -- treat as fixed)
- **D1:** one spec delivers the service + its CLI/show/stream consumers + the ddos-detect refactor.
- **D2 = Depth 1:** `ddos/detect` consumes the service's per-interface rate instead of subscribing to the raw iface tick and re-deriving PPS. Its rolling baseline, state machine, and responders STAY. Extracting the baseline into a shared core leaf (Depth 2) is delegated to `spec-cp-survival-5-detect-6-behavioural` (which already plans that extraction) and is OUT of this spec.
- **D3:** the service is a component (`internal/component/trafficstat`), not a core leaf, because it imports the `iface` component for per-interface rates. The port-name table IS a core leaf.
- **D4 = D4a, expanded:** a hardcoded port→service-name table like `/etc/services` (new core leaf), used to label ports. A small amplification-vector overlay (DNS/53, NTP/123, Memcached/11211, SSDP/1900, LDAP/389, SNMP/161, CharGEN/19) annotates known reflection ports. This is presentation only; no detection/scoring/action lives in the monitor.

### Explicit non-goals
- No new packet capture (no libpcap); the service is a consumer of existing collectors via `observation.Feed` + `iface` rates.
- No extraction of the ddos baseline into a core leaf (that is `spec-6`).
- No per-source behavioural scoring or response (that is `spec-6` / `detect-5`).
- No new wire protocol; no YANG operator config for the service itself (lifecycle is consumer-driven, window/top-N are internal safety constants).

## Required Reading

### Architecture Docs
- [ ] `ai/rules/module-tiers.md` - tier placement of the service vs the table
  → Decision: aggregation service is a **component** (imports `internal/component/iface`); core leaf is forbidden from importing a component. The port→name table is config-free pure data → `internal/core` leaf.
  → Constraint: `scripts/dev/dep_audit.py --check` (`make ze-tier-check`) fails exit 2 on a misplaced package.
- [ ] `ai/rules/plugin-self-containment.md` - the service owns its whole surface (service + cmd + table consumer)
  → Constraint: a consumer never imports a collector; consumers read the `observation.Feed` / the service, not `trafficusage` internals.
- [ ] `docs/architecture/cli/color-system.md` - 7 semantic color roles for the TUI
  → Constraint: severity coloring uses the existing roles (value/caution/danger), no ad-hoc ANSI.
- [ ] `ai/rules/cli-grammar.md` - `monitor` verb, action-before-identifier, typed selectors
  → Constraint: `monitor traffic` (verb=monitor, noun=traffic); drill-down selector is `name <iface>`.
- [ ] `ai/rules/cli-patterns.md` - flag.NewFlagSet, exit codes, stderr, tab-completion
  → Constraint: `monitor traffic` and `show traffic` must appear in tab-completion (not `Hidden`).
- [ ] `ai/rules/no-sprintf-alloc.md` + `ai/rules/buffer-first.md` - the per-tick aggregation + render path
  → Constraint: aggregation runs ~1 Hz over potentially many keys; per-tick dispatch and render build via `textbuf.Buffer`, no `fmt.Sprintf` on the per-key path.

### RFC Summaries (MUST for protocol work)
- [ ] N/A - no wire-protocol change. The port→name table follows the IANA service-name registry convention (the `/etc/services` data set), which is not an RFC conformance requirement.

**Key insights:**
- The `observation.Feed` (`observation.go:88`) is a transport, not a store: a new subscriber sees only *future* observations (`Subscribe` at `:104` adds to fan-out; no replay). The service therefore warms up over ~one window after its first consumer attaches.
- Feed values are per-`Feature`, NOT uniformly cumulative: `FeatureRxBytes` (trafficusage per-source-IP) is a CUMULATIVE counter, so the service derives per-second rates by diffing consecutive `Value`+`At` per key (as `iface/rate.go:240 rateDelta` does); `FeatureFlowBytes` (flowexport conntrack) is a per-publish DELTA, so the service SUMS it over the window. Rate derivation MUST branch on `Feature` (this corrects an earlier assumption that all feed values were cumulative).
- Per-interface rates already exist independent of ddos-detect: `iface.GetRate`/`ListRates` are fed by the always-on rate tracker (`iface/register.go:651-653`). So the interface panel needs no detector.
- As of commit `293ab1400` the feed already carries two talker sources: `KindSourceIP`/`FeatureRxBytes` (trafficusage, cumulative, IPv4, `monitor.go:321`) AND `KindFlow`/`FeatureFlowBytes` (flowexport conntrack, full 5-tuple Src/Dst/SrcPort/DstPort/Proto, delta, v4+v6, `exporter.go ExportFlows`). `KindInterface` is defined (`observation.go:38`) but NOT published, so the service reads `iface.GetRate` for the interface panel until the iface tick publishes interface observations (coordination point with `spec-observation-feed`).
- The streaming-handler lifecycle is the natural lazy-lifecycle binding: a handler returns when its `ctx` is done = client disconnect (`iface/cmd/interface_rate.go:92`), so subscribe-on-enter / unsubscribe-on-return drives the service refcount.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/observation/observation.go` - typed in-proc multi-subscriber `Feed`; `Subscribe(name, fn) int` (`:104`) runs `fn` on a per-subscriber goroutine draining a 1024-buffer channel; `Publish` is non-blocking, drops + counts on full buffer (`:172-187`); `Observation{Kind,Iface,Flow,Feature,Value,At}` (`:62`); `Global()` process feed (`:212`).
  → Constraint: drop-on-full means the service is statistical, not lossless; acceptable for monitoring.
- [ ] `internal/component/iface/rate.go` - `rateTracker.collect()` (`:176`) computes `InterfaceRate{RxBps,TxBps,RxPps,TxPps,Stats}` from kernel deltas every 1 s; `GetRate(name)`/`ListRates()` read the snapshot; multi-subscriber `SubscribeCollectNotify` (`:83`). Tracker started unconditionally (`register.go:651-653`).
- [ ] `internal/plugins/trafficusage/show.go` - `ze-show:traffic-usage [name <iface>]` (`:17,26`) → `renderInterface` (`:64`) yields `{interface, ingress-ports/egress-ports:[{port,protocol,bytes}], ingress-ips/egress-ips:[{ip,bytes}] (track-ip only), map-entries}`; `{status:"not-configured"}` when idle (`:28`).
- [ ] `internal/plugins/trafficusage/monitor.go` - eBPF poller `Start`/`Stop` (`:215`,`:274`) is itself lazy-ish (idempotent, runs while configured); publishes per-source-IP `Observation`s (`KindSourceIP`/`FeatureRxBytes`, cumulative, IPv4) to `observation.Global()` at `publishLocked` (`:321-332`).
- [ ] `internal/plugins/flowexport/exporter.go` - publishes per-flow `Observation`s (`KindFlow`/`FeatureFlowBytes`, full 5-tuple Src/Dst/SrcPort/DstPort/Proto, per-publish delta, IPv4+IPv6) to `observation.Global()` at `ExportFlows` (added in commit `293ab1400`). Richest talker source: dest IP + ports + proto, both families. Gated on flow-export conntrack being enabled.
- [ ] `internal/plugins/ddos/detect/register.go` - subscribes `det.onRate` to the iface tick ONLY when `cfg.Enabled` (`:91-97`,`:125-127`); unsubscribes on stop (`:144-151`).
- [ ] `internal/plugins/ddos/detect/detector.go` - `onRate([]iface.InterfaceInfo)` (`:70`) derives per-interface PPS, runs the baseline (`:118-123`), tracks peak (`:125-129`), state machine. **This is the layer-2.5 aggregation living in a layer-3 plugin.**
- [ ] `internal/component/iface/cmd/interface_rate.go` - `RegisterStreamingHandler("monitor interface rate", streamInterfaceRate)` (`:24`); `streamInterfaceRate` ticks 1 s, `enc.Encode`s rates, returns on `ctx.Done()` (`:78-114`). The streaming-handler shape to mirror.
- [ ] `internal/component/cli/model_dashboard.go` - `monitor bgp` full-screen TUI polling `commandExecutor` every 2 s (`:236`); `internal/component/cli/model_ping.go`/`model_traceroute.go` - streaming-channel TUI drained on a 50 ms tick; `model_render.go:279 paddedAltView` alt-screen.
- [ ] (grep) no existing port→service-name table; `protoName` (IP-proto→name) exists at `flowspec-firewall/translate.go:259` and `trafficusage/metrics.go:46`.

**Behavior to preserve:**
- `ddos/detect` volumetric detection + responder behaviour after the Depth-1 refactor (no regression).
- `show traffic-usage`, `show interface rate`, `monitor interface rate` keep working unchanged.
- `observation.Feed` publish path stays zero-alloc / non-blocking.

**Behavior to change:**
- `ddos/detect` stops subscribing to the raw iface tick and instead consumes the new service for its per-interface rate input.

## Data Flow (MANDATORY)

### Entry Point
- Operator runs `monitor traffic [name <iface>]` (TUI), `show traffic [name <iface>]` (one-shot), or a remote/web client opens the `ze-monitor:traffic` stream. Each is a CONSUMER that attaches to the service.
- In-daemon consumers (ddos-detect; future analytics) attach via the service's Go subscription API.

### Transformation Path (target)
1. **Collection (unchanged):** eBPF/conntrack publish `Observation`s to `observation.Global()`; iface rate tracker computes per-interface rates.
2. **Service attach (lazy):** first consumer → `trafficstat` increments its refcount, subscribes to `observation.Feed` (and reads `iface.ListRates` each tick), starts its windowing goroutine.
3. **Aggregation:** per tick, fold observations into bounded per-key windows; derive per-second rates from cumulative deltas; rank top-N source/dest IPs and ports; aggregate protocol mix; track per-interface peak + a rolling history ring (sparkline); compute a severity tier from rate-vs-recent-window.
4. **Snapshot fan-out:** the service serves a `Snapshot` (one-shot) and pushes per-tick snapshots to subscribers (TUI/stream/ddos-detect).
5. **Labeling:** ports rendered through the `portname` core-leaf table (+ amplification overlay).
6. **Service detach (lazy):** last consumer leaves → unsubscribe from feed, stop goroutine, free windows.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| observation.Feed ↔ service | `Feed.Subscribe`/`Unsubscribe` (in-proc, typed) | [ ] |
| iface ↔ service | `iface.ListRates`/`GetRate` (component→component) | [ ] |
| service ↔ CLI (remote) | `ze-monitor:traffic` streaming handler + `ze-show:traffic` RPC | [ ] |
| service ↔ ddos-detect | service Go subscription API (in-proc plugin→component) | [ ] |
| service/CLI ↔ portname | `portname.Lookup(port, proto)` (core leaf) | [ ] |

### Integration Points
- `internal/core/observation` `Feed` - the upstream the service subscribes to
- `internal/component/iface` `ListRates`/`GetRate` - per-interface rate input
- `internal/plugins/ddos/detect/register.go`/`detector.go` - swap the data source (Depth-1)
- `internal/component/cli` - new `model_traffic.go` + key/render/factory wiring, mirroring dashboard/ping

### Architectural Verification
- [ ] Service runs with ddos-detect disabled (layer split proven)
- [ ] Core leaf `portname` has no component/plugin imports (tier-check)
- [ ] No consumer imports a collector (self-containment)
- [ ] No per-tick heap alloc on the service dispatch / render hot path
- [ ] ddos-detect behaviour unchanged after the refactor

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | `observation.Feed` is the right substrate (all consumers in-process) | `spec-observation-feed` A-1 CONFIRMED; `observation.go` is in-proc typed feed | service needs a different transport | read producer (done) | confirmed |
| A-2 | A plugin (ddos-detect) may consume a component service in-process | `ddos/detect/register.go:9` already imports + calls the `iface` component | refactor needs a different seam | read producer (done) | confirmed |
| A-3 | Per-interface rates are available with ddos-detect disabled | rate tracker started unconditionally `iface/register.go:651-653`; `GetRate` independent | interface panel needs the detector | read producer (done) | confirmed |
| A-4 | Per-IP/port data is available with ddos-detect disabled | `trafficusage` is a separate plugin publishing to the feed (`monitor.go:321`) | talker panels need the detector | read producer (done) | confirmed |
| A-5 | A streaming handler's `ctx` is canceled on client disconnect (lazy detach signal) | `streamInterfaceRate` returns on `ctx.Done()` `interface_rate.go:92` | refcount never decremable from the stream | read producer (done) | confirmed |
| A-6 | A hardcoded compiled-in port table is acceptable (no runtime `/etc/services`) | Ze targets gokrazy appliance (no guaranteed `/etc/services`) | table must be read at runtime | user statement + appliance constraint | confirmed |
| A-7 | The `observation.Feed` API (Subscribe/Publish/Observation) is stable enough to build on | LANDED in commit 293ab1400; working tree matches HEAD (re-checked) | service churns with feed changes | read producer (done); re-confirm at implement | confirmed (landed; user may still iterate, keep coupling to the documented surface) |
| A-8 | Per-source-IP cardinality under a spoofed flood is boundable with top-N + cold eviction | `trafficusage` caps eBPF map entries; `spec-6` R-3 same concern | unbounded memory under attack | design eviction + cap; load test | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Depth-1 edit to `detector.go`/`register.go` collides with `spec-observation-feed` (migrating the same subscription) and `spec-6` (R-1) | merge conflict on ddos-detect | keep the edit minimal (swap data source only); sequence after observation-feed lands the multi-subscriber tick; coordinate via the shared note in `spec-6` |
| R-2 | Cardinality blow-up: one window per (entity) under a spoofed-source flood | memory/CPU spike during attack | bounded top-N + cold-entity eviction (A-8); cap tracked keys; drop-tolerant feed |
| R-3 | Feed drops under load make the view lossy/inaccurate during the exact event operators care about | `ze_observation_dropped_total` rises; ranks look wrong | document it; size the subscriber buffer; the view is statistical; lossless accounting is a separate (future) consumer concern |
| R-4 | `observation.go` changes mid-implementation (A-7) | feed API compile break | depend only on the documented Feed surface; re-confirm at audit; small adapter if shape shifts |
| R-5 | Lazy-lifecycle race: last consumer detaches while a new one attaches → service stop/start flap or a lost subscription | flapping logs; missed first snapshot | refcount under one mutex; start-on-0→1 / stop-on-1→0 atomic with subscription registration; unit test the race |
| R-6 | Interface rates not on the feed yet (`KindInterface` unpublished) means the service has two input paths (feed + `iface.ListRates`) | divergence between panels | read interface rates from `iface` now; migrate to feed when observation-feed publishes `KindInterface`; documented coordination |
| R-7 | TUI over SSH: terminal width/resize/Ctrl-C handling for a new full-screen view | garbled render; stuck alt-screen | reuse the dashboard/ping Bubble Tea plumbing (`paddedAltView`, `WindowSizeMsg`, Ctrl-C/q stop), do not roll new raw-mode code |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `monitor traffic` opened (stream) with ddos-detect DISABLED | → | `trafficstat` service starts on first consumer, emits a snapshot | `TestTrafficstatStartsWithoutDetector` |
| `show traffic name <iface>` | → | `ze-show:traffic` handler → `trafficstat.Snapshot()` | `test/plugin/traffic-monitor.ci` |
| last consumer disconnects | → | service unsubscribes from feed + stops goroutine | `TestTrafficstatLazyStopOnLastConsumer` |
| ddos-detect enabled | → | detector consumes `trafficstat` rate (not the raw tick) and still triggers | `TestDetectorConsumesTrafficstat` + existing ddos functional test |
| render a port row | → | `portname.Lookup` labels it | `TestPortnameLookup` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `monitor traffic` run while `ddos-detect` is disabled | Live per-interface rate view renders; no DDoS logic started |
| AC-2 | first consumer attaches | service subscribes to `observation.Feed` + starts windowing; before that it holds no subscription and no goroutine |
| AC-3 | last consumer detaches | service unsubscribes from the feed, stops its goroutine, frees windows (verified by no goroutine leak) |
| AC-4 | `monitor traffic name <iface>` under traffic | shows top-N egress/ingress ports (with service name) and top talker IPs, ranked by per-second rate |
| AC-5 | traffic on well-known ports | port column shows the service name from the hardcoded table (e.g. 443→https, 53→domain); known reflection ports also show the amplification-vector label |
| AC-6 | `traffic-usage` plugin not configured | interface-rate panel still renders; port/IP panels show a hint that `traffic-usage`/`track-ip` is required (graceful degradation) |
| AC-7 | `show traffic` and `show traffic name <iface>` | one-shot JSON snapshot with the same fields the TUI renders |
| AC-8 | `ddos-detect` enabled after the refactor | detector consumes the service's per-interface rate; volumetric detection still triggers/clears (no regression vs current functional test) |
| AC-9 | rates derivation | per-second rates correct per `Feature`: cumulative (`FeatureRxBytes`) diffed over time / elapsed; per-publish delta (`FeatureFlowBytes`) summed over the window; interface totals from `iface.GetRate` |
| AC-10 | spoofed high-cardinality sources | tracked-key count stays bounded (top-N + eviction); memory does not grow without bound |
| AC-11 | Ctrl-C / q / Esc in `monitor traffic` | clean exit from alt-screen, final frame to scrollback, consumer detaches |
| AC-12 | `portname.Lookup(unknown-port, proto)` | returns the numeric port (no crash, no empty label) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | runs `monitor traffic` on a box with NO ddos-detect | consumer → `trafficstat` start → `observation.Feed` + `iface.ListRates` → snapshot → TUI | `TestTrafficstatStartsWithoutDetector` + `test/ui` or `test/plugin/traffic-monitor.ci` |
| 2 | drills into `monitor traffic name eth0` during an attack | feed per-IP/port → top-N rank → `portname` label + amp overlay → TUI panels | `test/plugin/traffic-monitor.ci` |
| 3 | enables ddos-detect; it now shares the service | detector subscribes to `trafficstat` → baseline/state unchanged → responder acts | `TestDetectorConsumesTrafficstat` + existing ddos `.ci` |
| 4 | scripts `show traffic name eth0 | json` | `ze-show:traffic` → `Snapshot()` → JSON | `test/plugin/traffic-monitor.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTrafficstatStartsWithoutDetector` | `internal/component/trafficstat/*_test.go` | lazy start on first consumer; no detector (AC-1/AC-2) | |
| `TestTrafficstatLazyStopOnLastConsumer` | `internal/component/trafficstat/*_test.go` | unsubscribe + goroutine stop on refcount→0 (AC-3) | |
| `TestTrafficstatRefcountRace` | `internal/component/trafficstat/*_test.go` | concurrent attach/detach no flap/leak (R-5) | |
| `TestTrafficstatDerivesRates` | `internal/component/trafficstat/*_test.go` | cumulative→per-second deltas (AC-9) | |
| `TestTrafficstatTopNAndEviction` | `internal/component/trafficstat/*_test.go` | bounded top-N + cold eviction (AC-10) | |
| `TestTrafficstatDegradedNoTrafficUsage` | `internal/component/trafficstat/*_test.go` | interface-only view when collector idle (AC-6) | |
| `TestPortnameLookup` | `internal/core/portname/portname_test.go` | known/unknown port → name/number (AC-5/AC-12) | |
| `TestPortnameAmplificationOverlay` | `internal/core/portname/portname_test.go` | reflection-port labels present (AC-5) | |
| `TestDetectorConsumesTrafficstat` | `internal/plugins/ddos/detect/*_test.go` | detector triggers off service rate (AC-8) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| top-N | 1..cap | cap | 0 | cap+1 clamped |
| tracked-key cap | 1..max | max | 0 | max → eviction |
| port number | 0..65535 | 65535 | N/A | 65536 (uint16 wrap guarded) |
| history-ring length | 1..60 | 60 | 0 | >60 clamped |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `traffic-monitor` | `test/plugin/traffic-monitor.ci` | configure traffic-usage; drive loopback traffic; `show traffic name lo` reports the talker/port with service name; ddos-detect disabled | |
| `traffic-monitor-detect-coexist` | `test/plugin/*.ci` | enable ddos-detect + monitor together; both work; detection still fires | |

### Interop Tests (MANDATORY for protocol features)
N/A - no wire-protocol change (internal aggregation + CLI only).

### Future (if deferring any tests)
- A `test/ui/*.ci` snapshot of the full TUI layout may be added once the render format is frozen (the `.ci` above covers the data path).
- Full-screen TUI for `monitor traffic` (model_traffic.go with factory + piped mode, same design flow as ping/traceroute) -- deferred to a follow-up spec; v1 uses the generic streaming monitor path.

## Files to Modify
- `internal/plugins/ddos/detect/register.go` - replace `iface.SubscribeCollectNotify(det.onRate)` (`:95`,`:126`) with a `trafficstat` subscription
- `internal/plugins/ddos/detect/detector.go` - `onRate` input becomes the service's per-interface rate (Depth-1); baseline/state unchanged
- `internal/component/cli/model_keys.go` - dispatch `monitor traffic`
- `internal/component/cli/model_render.go` - alt-screen mode for the traffic view
- `internal/component/cli/client/main.go` - traffic session factory wiring (mirror `streamingPingFactory`)
- `docs/guide/command-reference.md`, `docs/guide/ddos-mitigation.md`, `docs/architecture/cli/*` - new command + layer doc
- `docs/features.md` - traffic monitor feature row

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | service lifecycle is consumer-driven; no operator config (window/top-N are internal safety caps, config-surface exception) |
| CLI commands/flags | Yes | `monitor traffic`, `show traffic` in `internal/component/cli` + `internal/component/trafficstat/cmd` |
| CLI grammar | Yes | `ai/rules/cli-grammar.md` - verb=monitor, noun=traffic, selector `name <iface>` |
| Tab-completion | Yes | command tree entry for `monitor traffic` / `show traffic` (not Hidden) |
| Functional test for new RPC | Yes | `test/plugin/traffic-monitor.ci` (`ze-show:traffic`, `ze-monitor:traffic`) |
| Pipe completeness | Yes | `show traffic` output routes through `ApplyPipes` (json/grep/...) |
| Doctor check | No | no NEW runtime dependency (reuses feed + iface; `traffic-usage` already owns its eBPF doctor surface) |
| Prometheus counters | Yes | service subscriber gauge + per-tick aggregate cost; reuse feed counters; register in telemetry |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - live traffic monitor |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` - `monitor traffic`, `show traffic` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` - `ze-show:traffic`, `ze-monitor:traffic` |
| 5 | Plugin added/changed? | Yes | plugin/component overview - new `trafficstat` component |
| 6 | Has a user guide page? | Yes | `docs/guide/<traffic-monitoring>.md` |
| 12 | Internal architecture changed? | Yes | layer doc: collection / dispatch / aggregation / action; cross-link `observation-feed` |
| 14 | Prometheus counters added? | Yes | telemetry doc: trafficstat subscriber/aggregate counters |
| 16 | Changed source referenced by doc anchors? | Yes | grep docs for `ddos/detect` rate-derivation anchors; update after the Depth-1 refactor |

## Files to Create
- `internal/core/portname/portname.go` - hardcoded `/etc/services`-style port→name table + amplification-vector overlay + `Lookup`
- `internal/core/portname/portname_test.go`
- `internal/core/portname/services.txt` (or generated `portname_table.go`) - committed IANA/`/etc/services` snapshot; regen via `make generate`
- `internal/component/trafficstat/service.go` - lazy refcounted aggregation service (subscribe/snapshot/window/top-N/severity)
- `internal/component/trafficstat/window.go` - per-key rolling window + rate derivation (branch on `Feature`: cumulative `FeatureRxBytes` diffed; delta `FeatureFlowBytes` summed) + eviction
- `internal/component/trafficstat/register.go` - component registration + metrics binding
- `internal/component/trafficstat/cmd/traffic.go` - `ze-show:traffic` + `RegisterStreamingHandler("monitor traffic", ...)` (mirror `iface/cmd/interface_rate.go`)
- `internal/component/trafficstat/*_test.go`
- `internal/component/cli/model_traffic.go` - Bubble Tea full-screen model (mirror `model_dashboard.go`/`model_ping.go`)
- `test/plugin/traffic-monitor.ci` - functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, validate A-1..A-8 |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7-10. Critical review loop | Critical Review Checklist |
| 11. Deliverables | Deliverables Checklist |
| 12. Security | Security Review Checklist |
| 14. Summary | Executive Summary |

### Implementation Phases
1. **Phase: Wiring (FIRST)** - register `internal/component/trafficstat` + `ze-show:traffic` stub + `monitor traffic` streaming-handler stub + CLI key dispatch; failing `TestTrafficstatStartsWithoutDetector`.
   - Verify: command reaches a stub that returns "not implemented"; wiring test fails on stub.
2. **Phase: portname core leaf** - committed table + `Lookup` + amplification overlay; tests AC-5/AC-12. (Independent; can land first.)
3. **Phase: aggregation service** - lazy refcount lifecycle, feed subscription, `iface.ListRates` interface panel, per-key windows + rate derivation, top-N + eviction, severity, snapshot. Tests AC-2/3/9/10, R-5 race.
4. **Phase: CLI consumers** - `model_traffic.go` TUI (alt-screen, panels, color, Ctrl-C), `show traffic` one-shot, stream factory. Tests AC-1/4/6/7/11.
5. **Phase: ddos-detect Depth-1 refactor** - swap data source to the service; keep baseline/state/responders. Tests AC-8 + existing ddos functional test stays green.
6. **Phase: functional tests + docs** - `traffic-monitor.ci`, coexist test, doc updates + source anchors.
7. **Full verification** → `make ze-verify`.
8. **Complete spec** → learned summary + two-commit closure.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Layer split | service runs with ddos-detect disabled (AC-1); detector now consumes it (AC-8) |
| Lazy lifecycle | start on 0→1, stop on 1→0, no goroutine leak, race-safe (AC-2/3, R-5) |
| Tier | `portname` is a clean core leaf; `trafficstat` component imports only iface+observation (ze-tier-check) |
| No regression | ddos-detect functional test green after refactor |
| Rates | per-second derivation correct for feed (cumulative) and iface inputs (AC-9) |
| Cardinality | bounded top-N + eviction proven under high key count (AC-10) |
| CLI grammar | `monitor traffic` / `show traffic` action-before-identifier; tab-completion present |
| Hot path | per-tick aggregation + render zero-alloc, textbuf-based |
| Self-containment | no consumer imports a collector |
| Registration over hardcoding (BLOCKER) | the rich `monitor traffic` view must register through a CLI view registry; the core `cli.Model` must NOT gain a per-feature field/factory/state/dispatch. Today dashboard/traceroute/ping/monitor each hardcode this (`Model` fields like `trafficMonitor *trafficState`; `SetXFactory` in `model_*.go`; wiring at `cmd/ze/hub/session_factory.go:91-93,118-120` + `client/main.go:127-134`; per-command dispatch in `model_keys.go`). Traffic must NOT repeat that anti-pattern -- introduce a `{prefix, factory, renderer}` registry (mirroring the server-side `pluginserver.RegisterStreamingHandler`, `handler.go:29`) and migrate the existing rich views onto it. Violates small-core/registration (CLAUDE.md) + `ai/rules/plugin-self-containment.md` |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `internal/core/portname` table + Lookup | `go test ./internal/core/portname/...`; `ze-tier-check` |
| `trafficstat` lazy service | `TestTrafficstat*` pass; goroutine-leak check |
| `monitor traffic` TUI + `show traffic` + stream | `test/plugin/traffic-monitor.ci` green; manual TUI smoke |
| ddos-detect Depth-1 refactor | existing ddos `.ci` + `TestDetectorConsumesTrafficstat` green |
| Docs + anchors | `make ze-doc-test`; grep source anchors |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | top-N cap + eviction bound memory under spoofed-source flood (AC-10/R-2) |
| Goroutine leak | service goroutine + per-subscriber drain exit on detach (AC-3) |
| Untrusted values | port/IP from kernel maps; `portname.Lookup` total over 0..65535; no format-string use |
| Info exposure | `show traffic` reveals talker IPs; same trust boundary as existing `show traffic-usage` (no new exposure) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| feed API drift breaks compile | A-7/R-4: adapt to documented Feed surface; re-confirm with user |
| ddos functional test regresses | Phase 5: re-read `detector.go`; the refactor changed only the input, not the policy |
| tier-check fails | move misplaced code; `portname`=core, `trafficstat`=component |
| 3 fix attempts fail | STOP, report, ask user |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
- The `observation.Feed` makes the lazy property cheap: `Publish` over zero subscribers is a no-op loop, so collectors can publish unconditionally while the EXPENSIVE aggregation is gated on consumer count. "Lazy service" = gate the aggregator, not the collectors. (True lazy *collection* is a separate, larger lever, out of scope.)

## Core Insight
The user-visible feature ("a monitor command") is the cheap part; the real work is recognizing that
"aggregate / time-based usage data" is a missing architectural layer wedged inside `ddos/detect`.
Extracting it as a lazy, consumer-driven service makes the monitor, analytics, and the detectors all
peers consuming one substrate, and is what lets `monitor traffic` run with no detection logic.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Aggregation service is a `component` | core leaf (D3 first idea) | it imports the `iface` component for rates; core may not import a component (tier-check) |
| Build on `observation.Feed` | re-collect via libpcap (NetHawk-style); CLI-side composition | feed is the just-built dispatch layer; CLI-only recomputes per client and isn't reusable |
| Lazy lifecycle = consumer refcount; streaming handler is a consumer | always-on service; CLI-local | matches user requirement: run when consumed, idle otherwise; stream ctx-cancel is the detach signal |
| Depth-1 ddos-detect refactor only | Depth 2 (move baseline too) | baseline extraction is owned by `spec-6`; avoid three specs editing `baseline.go` (R-1) |
| port→name in a `core/portname` leaf, shared | inline map in the CLI; per-consumer copy | `detect-5` classifier needs the same table; single source of truth; tier-valid pure data |
| Interface rates from `iface.GetRate`, not the feed (for now) | wait for `KindInterface` on the feed | `KindInterface` not published yet; avoid blocking on observation-feed; migrate later (R-6) |

## Known Limitations
- Interface panel reads `iface.GetRate` until the feed carries `KindInterface` (coordination with observation-feed).
- Per-source-IP byte data (trafficusage `KindSourceIP`) is IPv4-only and needs `traffic-usage track-ip`. Full 5-tuple + IPv6 talker data is available from flowexport conntrack `KindFlow` observations when flow-export is enabled (landed in `293ab1400`). With neither collector enabled the view degrades to interface rates only (AC-6).
- View is statistical (feed drop-on-full); not a lossless accounting source.
- No attack classification/scoring/response in the monitor (presentation only); that stays in the detectors.

## RFC Documentation
N/A - no protocol/wire behavior.

## Implementation Summary
(Filled during implementation.)

### What Was Implemented
1. **`internal/core/portname`** -- hardcoded IANA-style port-to-service-name table with 130+ entries. Amplification-vector overlay for 7 known reflection ports (label derived from service name). `Lookup(port) Info` returns name + amplification label.
2. **`internal/component/trafficstat/service.go`** -- lazy consumer-refcounted aggregation service. Subscribes to `observation.Feed` on first consumer (0->1), unsubscribes on last detach (1->0). Publishes `Snapshot` with interface rates, top-N source IPs, top ports, severity tier. Global singleton via `SetGlobal`/`Global`.
3. **`internal/component/trafficstat/window.go`** -- per-key rolling aggregator. Handles both cumulative (`FeatureRxBytes`, diffed) and delta (`FeatureFlowBytes`, summed) feed values. Bounded top-N with cold-entity eviction. Severity from rolling history ring.
4. **`internal/component/trafficstat/cmd/traffic.go`** -- `ze-show:traffic-stat` RPC handler (one-shot snapshot) + `RegisterStreamingHandler("monitor traffic", ...)` for generic streaming monitor. Attach/detach lifecycle managed per-request.
5. **`internal/component/trafficstat/register.go`** -- `Init()` creates and installs the global service.
6. **ddos-detect Depth-1 refactor** -- `detector.onRates([]trafficstat.InterfaceEntry)` accepts pre-computed rates. `register.go` prefers `trafficstat.Global().SubscribeRates()`, falls back to `iface.SubscribeCollectNotify` if unavailable. Common baseline/state-machine logic extracted to `applyTick`.
### Bugs Found/Fixed
- `ze-show:traffic` wire method collision with QoS traffic control (renamed to `ze-show:traffic-stat`).
### Documentation Updates
### Deviations from Plan
- `show traffic` wire method renamed to `ze-show:traffic-stat` because `ze-show:traffic` is already used by the QoS traffic control component (`internal/component/traffic/cmd`). CLI command is `show traffic-stat`.
- `monitor traffic` uses the generic streaming monitor path (model_monitor.go) instead of a custom full-screen TUI (model_traffic.go). User approved this simplification; full-screen TUI deferred to a follow-up spec.
- No changes to `model_keys.go`, `model_render.go`, or `client/main.go` needed (generic streaming path handles everything).

## Implementation Audit
(Filled at closure.)

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Usage view with zero DDoS logic | functional test | `TestTrafficstatStartsWithoutDetector` + `traffic-monitor.ci` (detector disabled) |
| glances/NetHawk-style live view | functional + manual | `traffic-monitor.ci` panels; TUI smoke |
| Reusable substrate | unit | service consumed by CLI + ddos-detect in tests |
| ddos-detect consumes service, no regression | functional | existing ddos `.ci` + `TestDetectorConsumesTrafficstat` |
| Shared port→name table | unit | `TestPortnameLookup` used by monitor (and available to detect-5) |

## Review Gate
### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| | | | | |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification
(Filled during the Completion Checklist.)

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete - every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled - 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)

### Completion (BLOCKING - before ANY commit)
- [ ] Critical Review passes - all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-traffic-usage-monitor.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-traffic-usage-monitor.md` only
