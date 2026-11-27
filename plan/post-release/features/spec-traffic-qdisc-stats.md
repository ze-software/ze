# Spec: traffic-qdisc-stats

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli, plugin, docs |
| Priority | P3 |
| Release reason | Adds live traffic-control qdisc and per-class runtime statistics (queue length, backlog, drops, overlimits, requeues, bytes, packets) to `show traffic control` and Prometheus `/metrics` across both shaped and unshaped Linux routers; existing shipped behavior and its obligations remain required. |
| RFC gaps | - |
| Depends | - |
| Phase | 3/3 |
| Handoff | - |
| Updated | 2026-11-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Expose live kernel traffic-control qdisc and per-class runtime statistics (`bytes`, `packets`, `qlen`, `backlog`, `drops`, `overlimits`, `requeues`) through `show traffic control`, `ze traffic control show interface <name>`, and Prometheus `/metrics` so operators can diagnose WAN edge queuing, bufferbloat, and micro-burst drops directly from Ze.

Today, Ze exposes interface-level drop counters (`ze_interface_rx_dropped_total` / `ze_interface_tx_dropped_total`), but `trafficnetlink.ListQdiscs` discards `Qdisc.Attrs().Statistics`, never calls `classList` (leaving `class-count` permanently `0` on Linux), silently misreports kernel default qdiscs (`noqueue`, `pfifo_fast`, `mq`) as `"htb"`, and `show traffic control` refuses to run unless `traffic { control { } }` is explicitly configured. On Linux routers running default kernel qdiscs (`fq_codel`, `pfifo_fast`) as well as routers running Ze-configured shaping (`htb`, `tbf`, `hfsc`, `fq`), operators need live per-qdisc and per-class queue depth and drop visibility without configuring shaping first and without fabricating zeros on backends that do not expose Linux qdisc queues (such as VPP).

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform contracts, process boundary, registration over enumeration
  → Constraint: every new metric, reader, or collector registers with its owner; no central backend switch and no second metrics HTTP server.
- [ ] `ai/doors/core.md` - leaf tier import direction and shared registries
  → Constraint: `internal/core/metrics` imports only stdlib and metrics types, never `internal/component/*` or `internal/plugins/*`; push collectors register into `internal/core/metrics` via a generic callback seam.
- [ ] `ai/doors/forwarding.md` - `traffic` pluggable backend (`kernel` vs `vpp`), exact-or-refuse, logical-to-OS interface resolution
  → Constraint: logical interface names resolve through `iface.Resolve` inside `trafficnetlink` (`resolveOSName`); VPP has no Linux egress qdisc queue and must leave `Stats` nil so no synthetic zero is ever reported (`ai/rules/principles.md`).
- [ ] `ai/doors/api-surfaces.md` - single metrics registry (`/metrics`), metric naming stability, MCP/REST/CLI parity through the command dispatcher
  → Constraint: `show traffic control` returns structured `plugin.Map` data with kebab-case keys so CLI, JSON, YAML, table, REST, gRPC, and MCP all receive the same payload with zero surface-specific code; `telemetry/exporter` starts registered push collectors and stops them cleanly on close.
- [ ] `ai/doors/commands-cli.md` - read-only `show` verb contract, `ResponseData`, pipe completeness
  → Constraint: `show traffic control` stays strictly read-only, keeps its existing YANG path `ze-traffic:show-traffic`, and routes structured output through `ApplyPipes`.
- [ ] `ai/doors/system-plugins.md` - backend plugin registration and test registry cleanup
  → Constraint: backend and reader registrations live in `internal/plugins/traffic/netlink/register.go`; tests that mutate package-level registries restore them in `t.Cleanup`.
- [ ] `ai/doors/le-tooling.md` - functional `.ci` test conventions and `option=needs-linux`
  → Constraint: Linux netlink tests carry `option=needs-linux` or `userns.Enter(t)` so macOS builds stay green.
- [ ] `docs/architecture/decisions/001-pull-model-metrics.md` - push-model metrics decision
  → Constraint: new Ze Prometheus metrics follow the registry-backed push model rather than introducing a scrape-time pull hook beside `metrics.Registry`.
- [ ] `docs/guide/traffic-control.md` - operator documentation for `traffic control` and `show traffic control`
  → Constraint: kernel TBF root rate is reported in bits/s (`rate-bps`); class HTB readback rates must likewise convert kernel bytes/s to bits/s with overflow checks.
- [ ] `docs/plugin-development/metrics.md` - Ze Prometheus metric naming and daemon-owned registration
  → Decision: raw cumulative kernel counters (`bytes`, `packets`, `drops`, `overlimits`, `requeues`) are exported as `GaugeVec` with `_total` suffix (matching `ze_interface_rx_dropped_total` in `internal/component/iface/rate.go`) because kernel counters reset on qdisc replace and wrap at `uint32`; instantaneous queue gauges (`qlen`, `backlog_bytes`) carry no `_total` suffix (confirmed by Thomas at DESIGN gate, 2026-11-27; codified in `docs/plugin-development/metrics.md` as a documented departure from "Never `_total`" for raw kernel counters).
- [ ] `docs/guide/monitoring.md` - Prometheus `/metrics` listener and Netdata-compatible OS collectors
  → Constraint: `netdata` collector prefix and `netdata.enabled` govern only Netdata-compatible OS collectors, never `ze_traffic_*` native metrics.

**Key insights:**
- `vishvananda/netlink` already decodes `Qdisc.Attrs().Statistics` (`*netlink.QdiscStatistics`, a type definition over `ClassStatistics`) and `Class.Attrs().Statistics` (`*netlink.ClassStatistics`) with `Basic` (`Bytes uint64`, `Packets uint32`) and `Queue` (`Qlen`, `Backlog`, `Drops`, `Requeues`, `Overlimits` `uint32`).
- `netlink.QdiscList(nil)` (`vendor/github.com/vishvananda/netlink/qdisc_linux.go:355`) issues a single `RTM_GETQDISC | NLM_F_DUMP` across all interfaces and stamps `Attrs().LinkIndex` on every returned qdisc. Combined with skipping `ClassList` on classless root qdiscs (`noqueue`, `fq_codel`, `pfifo_fast`, `fq`, `tbf`, `sfq`, `netem`, `clsact`, `ingress`, `pfifo`, `bfifo`), `ListAllQdiscs` takes 2 netlink dumps total (`LinkList` + `QdiscList(nil)`) plus 1 `ClassList` only for classful roots, instead of `1 + 2N` syscalls per second.
- `traffic.Backend` has external test mocks (`internal/component/l2tp/plugins/shaper/shaper_test.go`, `internal/component/l2tp/plugins/authradius/rfc5176_owner_apply_test.go`); keeping `Backend` unchanged and using one single 2-method `QdiscReader` interface (`ListQdiscs`, `ListAllQdiscs`) implemented by `trafficnetlink`, its stateless read-only reader, and `trafficvpp` avoids adding a second `QdiscLister` interface and preserves every existing mock without churn.
- `trafficnetlink.ListQdiscs` currently ignores `Statistics`, skips `classList`, and falls back to `QdiscHTB` for unrecognized kernel qdiscs (`noqueue`, `pfifo_fast`, `mq`).
- `docs/architecture/decisions/001-pull-model-metrics.md` explicitly defers pull-model scrape hooks and requires new metrics to use the registry-backed push model; a 1s periodic push collector registered via `metrics.RegisterPushCollector` and driven by `exporter.Start` / stopped by `exporterCloser.Close` obeys that decision, keeps `PrometheusRegistry` untouched, starts only when `telemetry.prometheus.enabled` is true (independent of `netdata.enabled` and `traffic { control { } }`), and catches 1-second queue depth spikes.
- `plan/post-release/features/spec-traffic-ingress-shaping.md` (`Status: ready`) also extends `ListQdiscs` and `show traffic control`; attaching `*QdiscStats`, `Handle`, and `Kind` additively to `Qdisc` and `TrafficClass` keeps the two specs composable.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/metrics/exporter_hook.go` - `StartExporter` seam wired by `ze_telemetry`; no push-collector registration seam for daemon-level periodic Ze-native push metrics
- [ ] `internal/component/telemetry/exporter/exporter.go` - `Start` creates `PrometheusRegistry`, starts HTTP server and (when `cfg.Netdata.Enabled`) OS collectors, and stops both in `exporterCloser.Close()`
- [ ] `internal/component/traffic/model.go` - `InterfaceQoS`, `Qdisc{Type, RateBps, DefaultClass, Classes}`, `TrafficClass{Name, Rate, Ceil, Priority, Filters}`; no `QdiscStats`, `Handle`, or raw kernel `Kind` field
- [ ] `internal/component/traffic/backend.go` - `Backend{Apply, ListQdiscs, Close}`, `OriginalStateRestorer`, `RegisterBackend`, `LoadBackend`, `GetBackend`; no read-only reader fallback when `activeBackend == nil`
- [ ] `internal/component/traffic/register.go` - registers `traffic` plugin without metrics collection
- [ ] `internal/component/traffic/cmd/traffic.go` - `handleShowTraffic` (`ze-traffic:show-traffic`) fails when `traffic.GetBackend() == nil` or `iface.ListInterfaces()` fails, emits `interface`, `qdisc`, `rate-bps`, `class-count`, `default-class` (detail) or `interface`, `qdisc`, `rate-bps`, `class-count`, `filter-count` (summary) with no stats or classes list
- [ ] `internal/component/traffic/cmd/show.go` - `FormatQoS` renders configured `InterfaceQoS` tree for offline CLI `ze traffic control show interface <name>`
- [ ] `internal/plugins/traffic/netlink/ops_linux.go` - `tcOps` has `linkByName`, `qdiscList`, `classList`, `filterList`, etc., but no `linkList`
- [ ] `internal/plugins/traffic/netlink/backend_linux.go` - `Apply` installs classes at handles `1:(i+1)` without storing handle-to-name map; `ListQdiscs` reads root qdisc type and TBF rate only, ignoring `Statistics` and classes
- [ ] `internal/plugins/traffic/netlink/translate_linux.go` - `raiseQdiscType` falls back to `traffic.QdiscHTB` for unrecognized kernel qdiscs
- [ ] `internal/plugins/traffic/netlink/register.go` - registers `"kernel"` backend factory only
- [ ] `internal/plugins/traffic/vpp/backend_linux.go` - `ListQdiscs` returns in-memory `interfaceOutputPolicers` and `interfaceQdiscTypes` with no queue statistics
- [ ] `internal/plugins/traffic-cmd/yang/ze-traffic-cmd.yang` and `internal/component/cmd/show/yang/ze-cli-show-api.yang` - declare `show traffic control` (`ze-traffic:show-traffic`)

**Behavior to preserve:**
- `traffic.Backend` 3-method interface signature (`Apply`, `ListQdiscs`, `Close`) so all existing mocks in `traffic`, `l2tp/plugins/shaper`, and `l2tp/plugins/authradius` continue compiling and passing unchanged.
- Existing `show traffic control` summary keys (`interface`, `qdisc`, `rate-bps`, `class-count`, `filter-count`, `count`) and single-interface keys (`interface`, `qdisc`, `rate-bps`, `class-count`, `default-class`).
- Existing `FormatQoS` output byte-for-byte whenever `Qdisc.Stats == nil` and every class's `Stats == nil` (`TestFormatQoS`, `TestFormatQoSRootTBFRate`).
- Existing `traffic` config apply, TBF overflow refusal, original qdisc snapshot/restore, and VPP verifier rejections.
- `PrometheusRegistry` remains pure push-model with zero scrape-time pull hooks.
- Non-Linux hosts with no registered backend/reader continue returning `"traffic control not available on this platform"`.

**Behavior to change:**
- Add `QdiscStats`, `Handle`, and raw kernel `Kind` (`KindName()`) to `traffic.Qdisc`, and `Handle` + `Stats` to `traffic.TrafficClass`.
- Fix `raiseQdiscType` to return `qdiscUnknown` (zero value) rather than `QdiscHTB` for unrecognized kernel qdiscs, while `Qdisc.Kind` records `root.Type()` verbatim.
- Extend `trafficnetlink.ListQdiscs` to read root qdisc stats and (only for classful roots) all classes under the root qdisc via one shared `extractQdiscStats` helper, and implement `ListAllQdiscs` using a single `QdiscList(nil)` bulk dump plus `LinkList` (skipping `ClassList` for classless roots and skipping links that vanish with `ENOENT`/`ENODEV` mid-dump).
- Register a stateless read-only `"kernel"` `QdiscReader` (`ListQdiscs` + `ListAllQdiscs`, the one shared readback interface also implemented by `trafficnetlink` and `trafficvpp`) so `traffic.ReadQdiscs` / `traffic.ReadAllQdiscs` work on Linux even when `traffic { control { } }` and `interface { }` are absent from config.
- Implement `ListAllQdiscs` on `trafficvpp` returning tracked VPP interfaces with `Stats: nil`.
- Include live `stats`, `handle`, and `classes` in `show traffic control` responses when `Stats != nil` via one shared stats-map helper (omitting `stats` on VPP / nil-stats backends).
- Add a generic `RegisterPushCollector` / `StartPushCollectors` seam in `internal/core/metrics/push_collector.go` wired into `internal/component/telemetry/exporter/exporter.go` (`Start` and `exporterCloser.Close`), and register a 1s push poller in `internal/component/traffic/metrics.go` using one shared `statsGaugeSet` helper (instantiated for qdisc and class label schemas) and reusable preallocated key maps (`clear` + swap per tick) for stale-label cleanup.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Traffic backend & optional extension interfaces | `internal/component/traffic/backend.go` (`Backend`, `OriginalStateRestorer`, `RegisterBackend`, `GetBackend`) | Pluggable backend registry with optional capability interfaces | Extends: adds one unified `QdiscReader` interface (`ListQdiscs`, `ListAllQdiscs`) and `RegisterReader` / `ReadQdiscs` / `ReadAllQdiscs` without breaking existing `Backend` mocks |
| Traffic QoS data model | `internal/component/traffic/model.go` (`InterfaceQoS`, `Qdisc`, `TrafficClass`) | Models qdisc type, TBF root rate, default class, and class rates/filters | Extends: adds `QdiscStats` (`*QdiscStats`), `Handle`, and raw kernel `Kind` (`KindName()`) directly on `Qdisc` and `TrafficClass` |
| Kernel netlink tc adapter | `internal/plugins/traffic/netlink/ops_linux.go` (`tcOps`), `backend_linux.go` (`ListQdiscs`), `translate_linux.go` (`raiseQdiscType`) | Reads root qdisc type and TBF rate only; ignores `Statistics` and `classList`; falls back to `QdiscHTB` | Extends: adds `linkList` to `tcOps`, uses `qdiscList(nil)` single bulk dump for `ListAllQdiscs`, calls `classList` only for classful roots, shares one `extractQdiscStats` helper, tracks Ze `classNames`/`defaultClasses` on `Apply`, and removes bogus `QdiscHTB` fallback |
| VPP traffic backend | `internal/plugins/traffic/vpp/backend_linux.go` (`ListQdiscs`) | Returns tracked VPP policer classes and configured qdisc type | Extends: implements `ListAllQdiscs` over tracked VPP interfaces while leaving `Stats` nil so no fake zero queue stats are reported |
| `show traffic control` RPC & CLI formatter | `internal/component/traffic/cmd/traffic.go` (`handleShowTraffic`), `show.go` (`FormatQoS`) | Dispatches `ze-traffic:show-traffic` across CLI, SSH, web, REST, gRPC, and MCP | Extends: switches to `traffic.ReadQdiscs` / `traffic.ReadAllQdiscs`, appends `stats`, `handle`, and `classes` on structured output, and appends stats line in `FormatQoS` when present |
| Shared Prometheus registry & exporter lifecycle | `internal/core/metrics/exporter_hook.go` (`StartExporter`), `internal/component/telemetry/exporter/exporter.go` (`Start`, `exporterCloser`) | Creates one `PrometheusRegistry`, starts `/metrics` HTTP server + OS collectors, and tears them down cleanly on `Close()` | Extends: adds a generic `metrics.RegisterPushCollector` / `StartPushCollectors` seam started by `exporter.Start` (independent of `netdata.enabled`) and stopped by `exporterCloser.Close()` so `internal/component/traffic` pushes `ze_traffic_*` at 1s using the push model |
| Netdata OS collectors | `internal/component/telemetry/collector/collector.go` (`Manager`) | Runs 1s Netdata-named OS collectors gated by `netdata.enabled` | Does not apply: `telemetry/collector` cannot import `traffic`, uses `netdata_*` naming, and turns off when `netdata.enabled false`; Ze-native `ze_traffic_*` metrics belong in `internal/component/traffic` |
| Interface 1 Hz rate tracker | `internal/component/iface/rate.go` (`rateTracker`, `cleanStaleIfaceMetrics`) | Exports `ze_interface_*` raw kernel counters as `GaugeVec` with `_total` and deletes stale labels | Does not apply for driving the tick (inactive without `interface` plugin), but this spec mirrors its exact 1s push cadence, `GaugeVec` + `_total`, and stale-label-cleanup pattern |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | The core knows nothing, and everything registers | the door every change reads | "A feature registers from `init()` in its own `register.go` and is discovered." | agrees | `trafficnetlink` registers its read-only reader from `register.go`, `traffic/metrics.go` registers its push collector into `internal/core/metrics`, and neither core nor exporter hardcodes `traffic` |
| `ai/doors/core.md` | Where it sits | the door covering `internal/core/metrics/push_collector.go`, `push_collector_test.go` | "`internal/core/` is the leaf tier." | agrees | `internal/core/metrics/push_collector.go` imports only stdlib and metrics interfaces, adding zero upward imports |
| `ai/doors/api-surfaces.md` | Boundaries | the door covering `internal/component/telemetry/exporter/exporter.go` | "Never add a command in an adapter." | agrees | `telemetry/exporter` only starts and stops generic registered `metrics.PushCollector` hooks on the existing `/metrics` server; no command or traffic import is added |
| `ai/doors/forwarding.md` | Boundaries | the door covering `internal/component/traffic/*`, `internal/plugins/traffic/{netlink,vpp}/*` | "Interface names go through the shared resolver" | agrees | `trafficnetlink.ListQdiscs` resolves logical-to-OS names via `resolveOSName`; VPP leaves `Stats` nil rather than fabricating zeros |
| `ai/doors/le-tooling.md` | Boundaries | the door covering `test/traffic/traffic-qdisc-stats.ci` | "A `.ci` that needs a Linux kernel feature carries `option=needs-linux`" | agrees | `test/traffic/traffic-qdisc-stats.ci` carries `option=needs-linux` and runs real kernel qdisc readback and `/metrics` scrape |
| `docs/architecture/core-design.md` | 14b. Firewall and Traffic Control | `// Design:` of `traffic/{model,backend,cmd/show}.go`, `traffic/netlink/{translate,backend}_linux.go`, `metrics/push_collector.go`, `telemetry/exporter/exporter.go` | "Ze manages nftables firewall tables and Linux traffic control through the same pluggable backend pattern as interfaces." | agrees | Preserves `traffic.Backend`, extends readback via one unified `QdiscReader` interface (`ListQdiscs`, `ListAllQdiscs`) and `RegisterReader`, and keeps `trafficnetlink` / `trafficvpp` cleanly separated |
| `docs/architecture/decisions/001-pull-model-metrics.md` | Decision | `internal/core/metrics/` push-model architecture | "The Prometheus implementation should continue to use the existing push-model `metrics.Registry` path." | agrees | Uses a 1s periodic push poller writing into `metrics.Registry` `GaugeVec`s rather than a scrape-time pull hook, leaving `PrometheusRegistry` pure push-model |
| `docs/architecture/api/commands.md` | Operational Report Bus (ze-cmd:show-warnings, ze-cmd:show-errors) | `// Design:` of `internal/component/traffic/cmd/traffic.go`; modified by spec | "`ze-traffic:show-traffic`" | agrees | Updates the `ze-traffic:show-traffic` response-shape table row additively to document `handle`, `stats`, and `classes` while keeping every existing key intact |
| `docs/architecture/traffic/fw-7-traffic-vpp.md` | Exact or reject | `// Design:` of `internal/plugins/traffic/vpp/backend_linux.go`; anchored to `internal/component/traffic/backend.go` | "If a backend cannot apply EXACTLY what the operator's config asks for, the verifier fails at commit with a clear error." | agrees | VPP `ListQdiscs` and `ListAllQdiscs` leave `Stats` nil rather than approximating Linux queue depth or drops with zeros; VPP apply/verify logic is untouched |
| `docs/architecture/traffic/fw-7b-backend-hardening.md` | `Backend.Apply` takes a context | anchored to `internal/component/traffic/backend.go`, `internal/plugins/traffic/netlink/backend_linux.go` | - | silent | This spec does not alter `Backend.Apply`, `vppOps`, GoVPP channel timeouts, or `sdk.SignalContext` |
| `docs/architecture/traffic/tc-original-qdisc-restore.md` | The `tcOps` seam | `// Design:` of `internal/plugins/traffic/netlink/ops_linux.go`; anchored to `backend_linux.go` | "`tcOps` is the narrow unexported interface over the ten netlink calls the backend makes" | agrees | Extends `tcOps` by one read-only method (`linkList`) for `ListAllQdiscs` while leaving snapshot persistence, validation, and original-qdisc restoration completely intact (and read-only reader never touches snapshots) |
| `docs/architecture/traffic/cp-survival-3-egress-cs6-sched.md` | The fix is at the source | anchored to `internal/plugins/traffic/netlink/{translate,backend}_linux.go` | - | silent | DSCP and protocol u32 filter translation and priority assignment are untouched |
| `docs/architecture/traffic/followup-vpp-traffic.md` | Reconcile | anchored to `internal/plugins/traffic/vpp/backend_linux.go` | - | silent | VPP DSCP/protocol classify chains, startup cleanup, and policer reconciliation are untouched; `ListAllQdiscs` is read-only over existing tracked maps |
| `docs/plugin-development/metrics.md` | Type-Specific Rules | modified by spec | "No `_total`" | departs | Raw cumulative kernel counters (`ze_traffic_qdisc_{bytes,packets,drops,overlimits,requeues}_total` and `ze_traffic_class_*_total`) use `GaugeVec` with `_total` (matching `ze_interface_*_total` in `internal/component/iface/rate.go`) because kernel counters reset on qdisc replace and wrap at `uint32`; this spec updates `docs/plugin-development/metrics.md` to document this raw-kernel-counter rule explicitly |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now. | agrees | Adds no config leaves or kernel/VPP mismatch bypasses; VPP data-plane validation stays unchanged |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each. | agrees | Does not alter config validation or data-plane mismatch verdict paths |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation. | agrees | Firewall verifiers and `ze:backend` annotations are untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it". | agrees | Read-only kernel reader registers under `"kernel"`, matching `defaultBackendName` and `traffic control backend kernel` |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it." | agrees | BGP export chains and next-hop rewriting are untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs"). | agrees | `policychain.Sets` and BGP policy configuration are untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)"). | agrees | BGP export policy compilation and `FilterInsertion` are untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self"). | agrees | BGP UPDATE forwarding and `wireOverride` provenance are untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load"). | agrees | BGP next-hop validation and MP_REACH encoding are untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes"). | agrees | BGP route origination and forwarding next-hop rules are untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?". | agrees | Implied filter-chain publication and `\| implied` are untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it. | agrees | Push-collector read failures log WARN and never abort daemon start or telemetry serving |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch. | agrees | Does not change daemon start refusal behavior or kernel/VPP mismatch handling |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 | The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included | agrees | Keeps existing `show traffic control` path without adding new command-path flags or presentation switches; format and filtering remain pipe operations |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 | A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed." | agrees | Adds no pipe overrides; standard generic pipe operators operate on the structured `plugin.Map` |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 | A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does | agrees | `show traffic control` produces one pure structured snapshot regardless of downstream pipe operators |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- **CLI / API / MCP query:** `show traffic control` or `show traffic control <ifname>` (wire method `ze-traffic:show-traffic`), or offline `ze traffic control show interface <name>`.
- **Prometheus metrics push & scrape:** `telemetry/exporter.Start` invokes `metrics.StartPushCollectors(reg, log)` (1s ticker inside `internal/component/traffic/metrics.go`), populating `GaugeVec`s served by HTTP `GET /metrics` (`PrometheusRegistry.Handler()`) and `show metrics list` / `show metrics values`.

### Transformation Path
1. **CLI / API / MCP path:** `handleShowTraffic` calls `traffic.ReadQdiscs(ifName)` (detail) or `traffic.ReadAllQdiscs()` (summary).
2. `traffic.ReadQdiscs` / `ReadAllQdiscs` selects `activeBackend` when loaded (e.g. `kernel` with Ze class-name map, or `vpp`), else falls back to the registered read-only `QdiscReader` for `defaultBackendName` (`"kernel"` on Linux, nil on non-Linux).
3. On Linux (`trafficnetlink`), `ListQdiscs` resolves logical-to-OS name via `resolveOSName`, reads `qdiscList(link)`, extracts root `Kind`, `Handle`, `Type`, TBF `RateBps`, and `QdiscStats` via one shared `extractQdiscStats` helper, and calls `classList(link, root.Attrs().Handle)` only when the root qdisc is classful. `ListAllQdiscs` performs one `linkList()` dump and one bulk `qdiscList(nil)` dump across all interfaces, calling `classList` only for links with classful roots and skipping any link that vanishes (`ENOENT`/`ENODEV`) mid-dump.
4. `handleShowTraffic` marshals the `InterfaceQoS` values into a kebab-case `plugin.Map` using one shared stats-map helper (omitting `stats` when `Stats == nil`) and returns it through `ApplyPipes` to CLI, SSH, web, REST, gRPC, and MCP.
5. **Prometheus push path:** `exporter.Start` calls `metrics.StartPushCollectors(reg, log)` unconditionally whenever `telemetry.prometheus.enabled` is true (independent of `cfg.Netdata.Enabled`). `traffic`'s registered push collector instantiates one shared `statsGaugeSet` helper twice (for the 7 `ze_traffic_qdisc_*` and 7 `ze_traffic_class_*` `GaugeVec`s), performs an immediate initial sample, and runs a 1-second `clock.Clock` ticker goroutine (`sync.WaitGroup`-joined on `stop()`) that calls `traffic.ReadAllQdiscs()`, updates live series where `Stats != nil`, and swaps two preallocated key maps (`clear` + swap) to `Delete()` stale label tuples without per-tick map allocation. `exporterCloser.Close()` stops and joins every push collector cleanly on shutdown.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Linux kernel netlink ↔ `trafficnetlink` | `vishvananda/netlink` `LinkList`, `LinkByName`, `QdiscList(nil\|link)`, `ClassList` (classful roots only) via `tcOps` seam | No |
| `trafficnetlink` / `trafficvpp` ↔ `internal/component/traffic` | Registered `Backend` / `QdiscReader` interfaces (`RegisterBackend`, `RegisterReader`) | No |
| `internal/component/traffic` ↔ `internal/core/metrics` ↔ `internal/component/telemetry/exporter` | `metrics.RegisterPushCollector` (called from `traffic/metrics.go` `init()`) started by `exporter.Start` via `metrics.StartPushCollectors` and stopped by `exporterCloser.Close()` | No |
| `internal/component/traffic/cmd` ↔ CLI / SSH / web / REST / gRPC / MCP | Registered RPC `ze-traffic:show-traffic` returning structured `plugin.Map` | No |

### Changed-Wire Census

**Search evidence:** Searched `internal/`, `cmd/`, `pkg/`, `test/`, and `scripts/` with `grep -rn "show-traffic\|show traffic control\|ListQdiscs"` (positive controls: `internal/component/traffic/cmd/traffic.go:23` RPC handler, `internal/plugins/traffic-cmd/yang/ze-traffic-cmd.yang:26` YANG command declaration, `internal/component/traffic/cli/main.go:65` offline CLI caller, `internal/component/l2tp/plugins/authradius/rfc5176_owner_apply_test.go:70` mock backend, `internal/le/site/testdata/published-command-paths.txt:348`). No shell, Python, or external-language parsers read `ze-traffic:show-traffic` output; Prometheus scrapers read `/metrics` text exposition.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `ze-traffic:show-traffic` JSON response | writer | `internal/component/traffic/cmd/traffic.go` (`handleShowTraffic`) | `grep` confirmed single registered writer for `ze-traffic:show-traffic` | Change (additive): preserve all existing keys (`interface`, `qdisc`, `rate-bps`, `class-count`, `filter-count`, `default-class`, `count`) and add `handle`, `stats`, `classes` (detail) and flat stats fields (summary) when `Stats != nil` |
| `ze-traffic:show-traffic` JSON response | reader | CLI/SSH/web/REST/gRPC/MCP generic dispatcher & pipe renderers (`internal/component/plugin/server/command.go`, `internal/component/command/`) and external API/MCP clients | Generic `plugin.Map` consumers derive fields dynamically; no hardcoded consumer in repo | Preserve: additive kebab-case fields remain backward-compatible for all existing clients and render automatically in `\| json`, `\| yaml`, `\| table` |
| Prometheus `/metrics` exposition | writer | `internal/component/traffic/metrics.go` (`qdiscMetricsPoller.collect`) via `internal/core/metrics/push_collector.go` and `internal/component/telemetry/exporter/exporter.go` | New `ze_traffic_qdisc_*` and `ze_traffic_class_*` series; no existing `ze_traffic_qdisc` or `ze_traffic_class` writers in repo | Change (additive): new metric families only; zero existing metric names or labels modified |
| Prometheus `/metrics` exposition | reader | External Prometheus scrapers and `show metrics list` / `show metrics values` (`internal/core/metrics/prometheus.go` `Names`, `Handler`) | Standard Prometheus exposition readers | Preserve: standard Prometheus gauge exposition with `ze_` prefix |

### Integration Points
- `internal/core/metrics/push_collector.go` (`PushCollectorStarter`, `RegisterPushCollector`, `StartPushCollectors`) - generic push-collector registry seam with explicit `stop()` cleanup
- `internal/component/telemetry/exporter/exporter.go` (`Start`, `exporterCloser.Close`) - starts registered push collectors on the shared `PrometheusRegistry` independent of `cfg.Netdata.Enabled` and stops them on shutdown
- `internal/component/traffic/backend.go` (`QdiscReader`, `RegisterReader`, `ReadQdiscs`, `ReadAllQdiscs`) - single readback interface and seam unifying active backend and unconfigured Linux kernel reader
- `internal/component/traffic/metrics.go` (`init` -> `metrics.RegisterPushCollector`, `statsGaugeSet`) - registers 14 `ze_traffic_qdisc_*` / `ze_traffic_class_*` `GaugeVec`s via one shared gauge-set helper and runs a 1s `clock.Clock` push poller with preallocated map-swap stale-label cleanup
- `internal/plugins/traffic/netlink/register.go` - registers read-only `"kernel"` `QdiscReader` alongside `RegisterBackend`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | CLI and Prometheus push poller both read through `traffic.ReadQdiscs` / `ReadAllQdiscs` -> registered backend/reader -> `tcOps` netlink seam |
| No unintended coupling (components stay isolated) | Yes | `internal/core/metrics` stays leaf-only (generic push-collector callback); `telemetry/exporter` never imports `traffic`; `traffic` never imports `trafficnetlink` |
| No duplicated functionality (extends existing, does not recreate) | Yes | Single `extractQdiscStats` and `readQdiscForLink` path in `trafficnetlink`, single `QdiscReader` interface, single `statsGaugeSet` helper in `traffic/metrics.go`, and single `qdiscStatsMap` helper in `cmd/traffic.go` |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Not a BGP wire hot path; `ListAllQdiscs` uses 1 bulk `QdiscList(nil)` dump and skips `ClassList` for classless roots, and `qdiscMetricsPoller` reuses preallocated tracking maps (`clear` + swap) each tick |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | `trafficnetlink` registers its read-only reader via `traffic.RegisterReader`; `traffic/metrics.go` registers its push collector via `metrics.RegisterPushCollector`; `internal/core/metrics` and `telemetry/exporter` hold zero knowledge of traffic |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | Searched `internal/component/traffic/`, `internal/plugins/traffic/`, `internal/component/telemetry/`, and `internal/component/cmd/show/`: no central metric list or backend switch exists; `show metrics list` derives from `PrometheusRegistry`, and `show traffic control` uses the existing `ze-traffic:show-traffic` registration |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `vishvananda/netlink.QdiscList` and `ClassList` populate `Attrs().Statistics.Basic` (`Bytes`, `Packets`) and `Attrs().Statistics.Queue` (`Qlen`, `Backlog`, `Drops`, `Requeues`, `Overlimits`) on Ze's Linux kernel for both default qdiscs (`fq_codel`, `pfifo_fast`, `noqueue`) and configured qdiscs (`htb`, `tbf`, `fq`, `hfsc`, `prio`) | `go doc github.com/vishvananda/netlink.{QdiscAttrs,ClassAttrs,ClassStatistics,GnetStatsBasic,GnetStatsQueue}` | Missing fields would need direct netlink TCA_STATS2 parsing or be explicitly flagged unavailable | Linux netns integration test `TestNetlinkIntegration_QdiscAndClassLiveStats` sending real packets over a veth pair (`PASS` on real Linux kernel via `userns`) | confirmed |
| A-2 | Operators want `show traffic control` and `/metrics` to report live Linux qdisc stats even when `traffic { control { } }` is not configured | Owner decision at RESEARCH gate (2026-11-27) | Would require gating behind `traffic { control { } }` | Confirmed by Thomas (2026-11-27); proven by `TestReadQdiscsUsesActiveBackendElseDefaultReader`, `TestShowTrafficControlWiredWithStats`, and `test/traffic/traffic-qdisc-stats.ci` (`PASS 5`) | confirmed |
| A-3 | VPP has no Linux egress qdisc queue (`qlen`/`backlog`/`requeues`), so `show traffic control` on VPP must omit `stats` (`Stats == nil`) and emit no `ze_traffic_qdisc_*` metrics rather than reporting fake zeros | `internal/plugins/traffic/vpp/backend_linux.go` (`ListQdiscs`, `ListAllQdiscs`) and `ai/rules/principles.md` (zero-value-as-valid-answer) | Reporting zero would mislead operators investigating VPP drops | Unit tests `TestVPPListAllQdiscsLeavesStatsNil`, `TestShowTrafficControlWiredWithStats`, and `TestQdiscMetricsOmitsNilStatsAndLifecycleStop` (`PASS`) | confirmed |
| A-4 | Kernel `GnetStatsQueue.Drops`, `Overlimits`, `Requeues`, and `GnetStatsBasic.Packets` are `uint32` counters in netlink that may wrap or reset to zero on qdisc replace, so exporting them as `GaugeVec` with `_total` (matching `ze_interface_rx_dropped_total`) avoids Prometheus `Counter` decrease panics | `go doc github.com/vishvananda/netlink.GnetStatsQueue`, `internal/component/iface/rate.go`, and Owner decision at DESIGN gate (2026-11-27) | Using `CounterVec` would panic or fail on qdisc replace | Unit test `TestQdiscMetricsStaleLabelCleanupAndReset` (`PASS`) | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Background 1s push poller goroutine leaks on daemon shutdown or exporter restart in tests | Goroutine leak / `-race` failure in `TestQdiscMetricsLifecycleStop` | Own stop channel and `sync.WaitGroup` inside `qdiscMetricsPoller`, return `Stop` closure from `StartPushCollectors`, and invoke it in `exporterCloser.Close()` (`ai/rules/goroutine-lifecycle.md`) |
| R-2 | Stale Prometheus label combinations accumulate forever when dynamic interfaces (`pppN`, `veth`, `xfrm`) disappear or qdiscs/classes are replaced | Series for deleted interface/handle remain present after the next 1s tick | Track active `(interface, qdisc, handle)` and `(interface, qdisc, class, handle)` tuples on the single poller goroutine and call `Delete(...)` on tuples missing from the current tick (matching `netdev_linux.go` and `iface/rate.go`) |
| R-3 | Kernel `ClassList` returns classes for a non-Ze qdisc (or returns `Statistics == nil` for a virtual `noqueue` qdisc) | Nil-pointer dereference or empty class name in JSON/Prometheus labels | Nil-check `Statistics`, `Basic`, and `Queue` separately; leave `Stats == nil` when `Statistics == nil`; fall back to formatted handle (e.g. `1:1`) when no Ze class name is recorded |
| R-4 | HTB class `Rate`/`Ceil` kernel readback in bytes/s overflows `uint64` when multiplied by 8 | Silent wrap in `Rate * 8` or `Ceil * 8` | Guard with `rate > math.MaxUint64/8` check matching TBF readback in `backend_linux.go:343` |
| R-5 | Merge conflict with `plan/post-release/features/spec-traffic-ingress-shaping.md` (`Status: ready`) which also edits `ListQdiscs` and `show traffic control` | Overlapping fields on `Qdisc` / `TrafficClass` | Keep `QdiscStats`, `Handle`, and `Kind` purely additive on `Qdisc` and `TrafficClass`, and keep `show traffic control` summary/detail top-level keys additive |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | `show traffic control` output or `/metrics` scrape could show wrong qdisc stats or fail; zero impact on packet forwarding or qdisc programming (`Apply` only records handle-to-name metadata in memory). |
| How is it reverted? | Single commit revert; no config syntax, YANG config schema, or persisted ZeFS state changes. |
| Who else touches this path? | `plan/post-release/features/spec-traffic-ingress-shaping.md` (same `ListQdiscs` and `show traffic control` files); `internal/component/l2tp/plugins/{shaper,authradius}` (consumes `traffic.Backend` — untouched because `Backend` interface does not change). |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze show traffic control` / `ze show traffic control <ifname>` (`ze-traffic:show-traffic`) | → | `handleShowTraffic` → `traffic.ReadAllQdiscs` / `traffic.ReadQdiscs` → `trafficnetlink` `ListQdiscs` / `ListAllQdiscs` | `TestShowTrafficControlWiredWithStats` and `test/traffic/traffic-qdisc-stats.ci` |
| `exporter.Start` → `GET /metrics` | → | `metrics.StartPushCollectors` → `traffic` 1s `qdiscMetricsPoller` → `traffic.ReadAllQdiscs` → `ze_traffic_qdisc_*` / `ze_traffic_class_*` | `TestQdiscMetricsEndToEndHTTPScrape` and `test/traffic/traffic-qdisc-stats.ci` |
| `ze traffic control show interface <name>` (offline CLI) | → | `traffic/cli.cmdShow` → `b.ListQdiscs` → `tccmd.FormatQoS` | `TestFormatQoSWithStats` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `trafficnetlink.ListQdiscs` and `ListAllQdiscs` called on Linux interfaces carrying root qdiscs (`fq_codel`, `tbf`, `noqueue`, `pfifo_fast`, or `htb`) with `Attrs().Statistics` populated | `ListAllQdiscs` performs one `LinkList` and one bulk `QdiscList(nil)` dump across all interfaces, skips `ClassList` for classless qdiscs, skips links that vanish with `ENOENT`/`ENODEV` mid-dump, returns `Qdisc.Kind` matching the kernel qdisc name (never misreporting `noqueue`/`pfifo_fast`/`mq` as `htb`), `Qdisc.Handle` formatted as `<major>:<minor>`, and non-nil `Qdisc.Stats` carrying exact `Bytes`, `Packets`, `Qlen`, `Backlog`, `Drops`, `Overlimits`, and `Requeues` values | When kernel `Attrs().Statistics` is `nil`, `Qdisc.Stats` remains `nil` so absent kernel telemetry is distinguishable from real zeros; real non-`ENOENT`/`ENODEV` netlink errors still fail explicitly |
| AC-2 | `trafficnetlink.Apply` installs an HTB qdisc with named classes (e.g., `control`, `default`) and `ListQdiscs` is called afterwards | Returns all installed kernel classes sorted by handle, each carrying its Ze class `Name` (falling back to handle string for non-Ze classes), `Handle`, `Rate` and `Ceil` in bits/s (refusing `bytes/s * 8` overflow), `Priority`, and live `Stats`, plus `Qdisc.DefaultClass` populated | Filter list on readback remains empty until filter readback is added (this spec owns qdisc/class stats, not filter disassembly) |
| AC-3 | `show traffic control` and `show traffic control <ifname>` run on a Linux daemon with NO `traffic { control { } }` and NO `interface { }` config blocks | Both succeed via the registered read-only `"kernel"` `QdiscReader`, returning real kernel qdisc kinds and live `bytes`, `packets`, `qlen`, `backlog`, `drops`, `overlimits`, and `requeues` counts (plus `classes` and `handle` on single-interface detail) | On non-Linux hosts where no backend or read-only reader is registered, `show traffic control` still returns `"traffic control not available on this platform"` |
| AC-4 | `show traffic control` and `show traffic control <ifname>` run when `traffic { control { backend vpp } }` is active | Returns tracked VPP interfaces with their configured qdisc type and class names, omitting `stats` on both summary and detail rows rather than reporting synthetic zeros | Existing keys (`interface`, `qdisc`, `rate-bps`, `class-count`, `filter-count`, `default-class`) remain present with their existing semantics |
| AC-5 | HTTP `GET /metrics` runs on a Linux daemon with `telemetry { prometheus { enabled true; } }`, both with and without `traffic { control { } }` and even with `netdata { enabled false; }` | Emits all 7 `ze_traffic_qdisc_*` `GaugeVec` series (labeled `interface`, `qdisc`, `handle`) for every interface with `Stats != nil` and all 7 `ze_traffic_class_*` `GaugeVec` series (labeled `interface`, `qdisc`, `class`, `handle`) for every class with `Stats != nil`, refreshed every second by the push poller and cleanly stopped on `exporterCloser.Close()` | Interfaces or backends whose `Stats == nil` (such as VPP) emit no `ze_traffic_qdisc_*` or `ze_traffic_class_*` samples, preventing fake zeros |
| AC-6 | An interface disappears, or its root qdisc/classes are replaced between two 1s poller ticks | The next tick updates the new label tuples (handling counter resets to zero cleanly without error) and removes every stale label tuple from all 14 `ze_traffic_qdisc_*` and `ze_traffic_class_*` vectors | None — every disappeared interface/qdisc/class label tuple must be deleted to keep Prometheus cardinality bounded |
| AC-7 | Linux netns integration test sends real packets across a veth pair carrying a configured HTB qdisc with classes (and a default `fq_codel` interface) | `ListQdiscs` reports `Bytes > 0` and `Packets > 0` on both the root qdisc and the matched class, matching kernel netlink state on Ze's Linux kernel | Exact packet/byte counts may include kernel-generated ND/ARP frames on the veth pair; assertions check monotonic increase and real non-zero counters |
| AC-8 | `FormatQoS` is called on an `InterfaceQoS` with `Stats == nil` vs non-nil `Stats` | Produces byte-for-byte identical output when `Stats == nil` (preserving `TestFormatQoS` and `TestFormatQoSRootTBFRate`), and appends a readable `stats` summary line when `Stats != nil` | None — both nil-stats preservation and non-nil stats rendering are strictly verified |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs `ze show traffic control` on an unshaped Linux router to check WAN queue drops | CLI/MCP → `ze-traffic:show-traffic` → `handleShowTraffic` → `traffic.ReadAllQdiscs` → `trafficnetlink` read-only reader → `LinkList` + `QdiscList` + `ClassList` → kebab-case `plugin.Map` | `test/traffic/traffic-qdisc-stats.ci` |
| 2 | Runs `ze show traffic control eth0 \| json` on a shaped Linux router with HTB classes | CLI/MCP → `ze-traffic:show-traffic` → `handleShowTraffic` → `traffic.ReadQdiscs("eth0")` → `trafficnetlink` active backend (with Ze class names) → structured detail with `stats` and `classes` | `test/traffic/traffic-qdisc-stats.ci` |
| 3 | Scrapes `/metrics` from Prometheus to alert on WAN qdisc drops and backlog buildup | `exporter.Start` → `metrics.StartPushCollectors` → `traffic` 1s push poller → `traffic.ReadAllQdiscs` → `PrometheusRegistry` → HTTP `GET /metrics` (`ze_traffic_qdisc_*` / `ze_traffic_class_*`) | `test/traffic/traffic-qdisc-stats.ci` and `TestQdiscMetricsEndToEndHTTPScrape` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPushCollectorRegisterStartAndStop` | `internal/core/metrics/push_collector_test.go` | `RegisterPushCollector` rejects duplicates, `StartPushCollectors` starts all registered collectors, and the returned stop closure stops every one cleanly | |
| `TestReadQdiscsUsesActiveBackendElseDefaultReader` | `internal/component/traffic/backend_test.go` | `ReadQdiscs` / `ReadAllQdiscs` prefer active backend, fall back to registered default reader, and error cleanly when neither exists | |
| `TestQdiscKindNamePreservesKernelKind` | `internal/component/traffic/model_test.go` | `Qdisc.KindName()` prefers raw `Kind` over `Type.String()`, preserving `"noqueue"`/`"pfifo_fast"`/`"fq_codel"` while falling back to `Type.String()` when `Kind == ""` | |
| `TestQdiscMetricsEndToEndHTTPScrape` | `internal/component/traffic/metrics_test.go` | All 14 `ze_traffic_qdisc_*` and `ze_traffic_class_*` metrics appear on HTTP `/metrics` with exact labels and values after initial sample and fake-clock ticks | |
| `TestQdiscMetricsStaleLabelCleanupAndReset` | `internal/component/traffic/metrics_test.go` | Disappeared interfaces/classes/handles are deleted on the next tick and counter decreases on qdisc replace update cleanly | |
| `TestQdiscMetricsOmitsNilStatsAndLifecycleStop` | `internal/component/traffic/metrics_test.go` | Backends returning `Stats == nil` emit zero series; `Stop()` cleanly joins the poller goroutine with `-race` | |
| `TestExporterStartsAndStopsPushCollectorsWithoutNetdata` | `internal/component/telemetry/exporter/exporter_test.go` | `exporter.Start` starts registered push collectors even when `netdata.enabled` is false and stops them on `Close()` | |
| `TestShowTrafficControlWiredWithStats` | `internal/component/traffic/cmd/traffic_test.go` | `handleShowTraffic` summary and detail include `stats`, `handle`, `classes`, real qdisc kind, and omit `stats` on VPP (`Stats == nil`) | |
| `TestFormatQoSWithStats` | `internal/component/traffic/cmd/show_test.go` | `FormatQoS` renders stats when non-nil and preserves existing output unchanged when nil | |
| `TestNetlinkListQdiscsStatsClassesAndKind` | `internal/plugins/traffic/netlink/stats_linux_test.go` | `fakeTCOps` test verifying `QdiscStats`, `ClassList` readback with Ze class names, non-Ze handle fallback, `noqueue`/`pfifo_fast` real kind, HTB rate overflow refusal, and `ListAllQdiscs` | |
| `TestNetlinkIntegration_QdiscAndClassLiveStats` | `internal/plugins/traffic/netlink/stats_linux_test.go` | Real Linux netns veth test sending packets through HTB classes and verifying live `Bytes > 0`, `Packets > 0`, and queue stats via `ListQdiscs` | |
| `TestVPPListAllQdiscsLeavesStatsNil` | `internal/plugins/traffic/vpp/apply_test.go` | VPP `ListQdiscs` and `ListAllQdiscs` return tracked interfaces with `Stats == nil` | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| HTB class readback `Rate` (kernel bytes/s → bps) | `0 .. math.MaxUint64/8` | `math.MaxUint64/8` | N/A (`uint64`) | `math.MaxUint64/8 + 1` (refused with overflow error) |
| HTB class readback `Ceil` (kernel bytes/s → bps) | `0 .. math.MaxUint64/8` | `math.MaxUint64/8` | N/A (`uint64`) | `math.MaxUint64/8 + 1` (refused with overflow error) |
| `GnetStatsQueue` (`Qlen`, `Backlog`, `Drops`, `Overlimits`, `Requeues`) | `0 .. math.MaxUint32` | `math.MaxUint32` | N/A (`uint32`) | N/A (`uint32`) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `traffic-qdisc-stats` | `test/traffic/traffic-qdisc-stats.ci` | Boots Ze on Linux with `telemetry { prometheus { enabled true; } }` (both with HTB shaping and on an unshaped interface), runs `ze show traffic control`, `ze show traffic control <iface> \| json`, and scrapes `/metrics` confirming live `ze_traffic_qdisc_*` and `ze_traffic_class_*` series (`option=needs-linux`) | |

## Files to Modify
- `internal/component/telemetry/exporter/exporter.go` - start registered push collectors via `metrics.StartPushCollectors` in `Start` (independent of `cfg.Netdata.Enabled`) and stop them in `exporterCloser.Close()`
- `internal/component/traffic/model.go` - add `QdiscStats`, `Qdisc.Kind`, `Qdisc.Handle`, `Qdisc.Stats`, `Qdisc.KindName()`, `TrafficClass.Handle`, `TrafficClass.Stats`
- `internal/component/traffic/backend.go` - add one unified `QdiscReader` interface (`ListQdiscs`, `ListAllQdiscs`), `RegisterReader`, and `ReadQdiscs` / `ReadAllQdiscs` helpers
- `internal/component/traffic/cmd/traffic.go` - switch `handleShowTraffic` to `ReadQdiscs`/`ReadAllQdiscs`, include `handle`, `stats`, `classes`, and real qdisc kind
- `internal/component/traffic/cmd/show.go` - format optional `Stats` summary in `FormatQoS` while preserving nil-stats formatting
- `internal/plugins/traffic/netlink/ops_linux.go` - add `linkList() ([]netlink.Link, error)` to `tcOps` and `netlinkOps`
- `internal/plugins/traffic/netlink/translate_linux.go` - fix `raiseQdiscType` fallback from `QdiscHTB` to `qdiscUnknown` and add `formatTCHandle`
- `internal/plugins/traffic/netlink/backend_linux.go` - retain Ze `classNames`/`defaultClasses` on `Apply` (cleared on `RestoreOriginal`/`Close`), populate live qdisc/class stats and classes in `ListQdiscs`, implement `ListAllQdiscs` and stateless `readOnlyReader`
- `internal/plugins/traffic/netlink/register.go` - register stateless read-only `"kernel"` `QdiscReader` in `init()`
- `internal/plugins/traffic/vpp/backend_linux.go` - implement `ListAllQdiscs` returning tracked interfaces with `Stats: nil`
- `docs/guide/traffic-control.md` - document live qdisc/class stats readback and unconfigured Linux readback
- `docs/guide/command-reference.md` - update `### show traffic control` fields and behavior
- `docs/plugin-development/metrics.md` - document the 14 `ze_traffic_qdisc_*` / `ze_traffic_class_*` `GaugeVec` series and codify the raw-kernel-counter `GaugeVec` + `_total` rule
- `docs/architecture/api/commands.md` - update `ze-traffic:show-traffic` response shape
- `features/traffic-control-lifecycle.md` - note live qdisc/class stats readback and Prometheus export

## Files to Create
- `internal/core/metrics/push_collector.go` - generic `PushCollectorStarter`, `RegisterPushCollector`, `StartPushCollectors` seam
- `internal/core/metrics/push_collector_test.go` - unit tests for push-collector registration, start, and stop
- `internal/component/traffic/metrics.go` - 1s `qdiscMetricsPoller` registered via `metrics.RegisterPushCollector` exporting `ze_traffic_qdisc_*` and `ze_traffic_class_*` with stale-label cleanup
- `internal/component/traffic/metrics_test.go` - unit tests for metrics scrape, stale-label cleanup, VPP nil omission, and stop lifecycle
- `internal/component/traffic/cmd/traffic_test.go` - unit tests for `handleShowTraffic` summary/detail stats, classes, and VPP nil omission
- `internal/plugins/traffic/netlink/stats_linux_test.go` - unit and Linux netns integration tests for kernel qdisc/class live stats
- `test/traffic/traffic-qdisc-stats.ci` - end-to-end functional test for `show traffic control` and `/metrics`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | Uses existing `ze-traffic:show-traffic` command (`internal/plugins/traffic-cmd/yang/ze-traffic-cmd.yang`); adds no new config leaves or RPCs |
| YANG validation constraints | N-A | No new YANG config or command input leaves added |
| YANG custom validators | N-A | No new YANG leaves added |
| CLI commands/flags | Yes | `internal/component/traffic/cmd/traffic.go`, `internal/component/traffic/cmd/show.go` extend existing `show traffic control` and `ze traffic control show interface <name>` output |
| CLI grammar (keyword before value) | Yes | Preserves existing `show traffic control` path; no new positional arguments or `--flags` added (`./le cli grammar`) |
| Editor autocomplete | N-A | Command path `show traffic control` unchanged |
| Functional test for new RPC/API | Yes | `test/traffic/traffic-qdisc-stats.ci` covers `show traffic control` and `/metrics` end-to-end |
| Pipe completeness | Yes | `handleShowTraffic` returns structured `plugin.Map` routed through `ApplyPipes` (`\| json`, `\| yaml`, `\| table`) |
| Env var registration | N-A | No new environment variables |
| Doctor check for runtime dependencies | N-A | Reuses existing Linux netlink `RTM_GETQDISC` / `RTM_GETTCLASS` interface already owned by `trafficnetlink`; no new file path, socket, kernel module, or binary |
| Doctor code per problem class | N-A | No new report-bus issues or doctor problem classes; netlink read errors return structured operator errors on CLI and log WARN on tick |
| Prometheus counters/metrics | Yes | `internal/component/traffic/metrics.go` registers 7 `ze_traffic_qdisc_{bytes_total,packets_total,qlen,backlog_bytes,drops_total,overlimits_total,requeues_total}` (`interface`, `qdisc`, `handle`) and 7 `ze_traffic_class_{bytes_total,packets_total,qlen,backlog_bytes,drops_total,overlimits_total,requeues_total}` (`interface`, `qdisc`, `class`, `handle`) `GaugeVec`s |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not a BGP address family, capability, or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/traffic-control-lifecycle.md` (note live qdisc/class stats readback and Prometheus export) |
| 2 | Config syntax changed? | No | Zero YANG config syntax changes |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` (`### show traffic control`) |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` (`ze-traffic:show-traffic` response shape) |
| 5 | Plugin added/changed? | No | No new plugin; existing `traffic` component and `trafficnetlink`/`trafficvpp` backends extended |
| 6 | Has a user guide page? | Yes | `docs/guide/traffic-control.md` (live qdisc/class stats readback section) |
| 7 | Wire format changed? | No | No protocol wire format changes |
| 8 | Plugin SDK/protocol changed? | No | `pkg/plugin/sdk` and `pkg/plugin/rpc` untouched |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | Linux tc netlink statistics have no IETF RFC |
| 10 | Test infrastructure changed? | No | Uses existing `.ci` runner and `userns` Linux netns helper |
| 11 | Affects daemon comparison? | No | No comparison matrix row changes |
| 12 | Internal architecture changed? | No | Preserves `docs/architecture/core-design.md` (14b) and `docs/architecture/decisions/001-pull-model-metrics.md` push model; `docs/architecture/traffic/fw-7-traffic-vpp.md` and `docs/architecture/traffic/tc-original-qdisc-restore.md` are unaffected (VPP apply/verify and snapshot restore behavior do not change) |
| 13 | Route metadata keys added/changed? | N-A | Not route metadata |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` (add the 14 `ze_traffic_qdisc_*` / `ze_traffic_class_*` series to Full Inventory and codify the raw-kernel-counter `GaugeVec` + `_total` rule) |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No new plugin, event type, or command path registered |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived via `./le spec citation anchors`: `docs/architecture/api/commands.md` (updated), `docs/architecture/traffic/fw-7-traffic-vpp.md` (declared by `vpp/backend_linux.go`, unaffected: VPP apply/verify/orphan behavior unchanged and `Stats` stays nil), `docs/architecture/traffic/tc-original-qdisc-restore.md` (declared by `netlink/ops_linux.go`, unaffected: snapshot/restore behavior unchanged; `tcOps` only adds read-only `linkList`), plus advisory mentions `docs/architecture/traffic/fw-7b-backend-hardening.md`, `docs/architecture/traffic/cp-survival-3-egress-cs6-sched.md`, `docs/architecture/traffic/followup-vpp-traffic.md`, `docs/functional-tests.md`, `docs/guide/pppoe.md` (all unaffected) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/command-reference.md` and `docs/guide/traffic-control.md` examples updated for live stats output |

## Design presentation

### What changes

`show traffic control` and Prometheus `/metrics` now report live Linux kernel queueing discipline and per-class statistics (`bytes`, `packets`, `qlen`, `backlog`, `drops`, `overlimits`, and `requeues`), real kernel qdisc names (fixing the silent fallback to `htb` on `noqueue`/`pfifo_fast`/`mq`), and kernel class lists annotated with Ze's configured class names. On Linux routers with no `traffic { control { } }` section in `ze.conf`, both `show traffic control` and `/metrics` still work automatically through a stateless read-only kernel reader sharing one `QdiscReader` interface and one bulk `QdiscList(nil)` readback path (2 netlink dumps per second total, skipping `ClassList` on classless qdiscs and skipping links that vanish mid-dump). Prometheus metrics use a 1-second periodic push poller (registered into `internal/core/metrics` and owned by `telemetry/exporter`'s start/close lifecycle, obeying `001-pull-model-metrics.md`), independent of `netdata.enabled`, with one shared `statsGaugeSet` helper and preallocated map-swap stale-label cleanup. On VPP (which has no Linux egress qdisc queues), stats are omitted rather than fabricated as zeros.

### Before and after

1. **Unshaped Linux router running `ze show traffic control`:**
   - Before: returns error `traffic control not available on this platform` whenever `traffic { control { } }` is absent from `ze.conf`.
   - After: lists every Linux interface with its real kernel qdisc kind (`fq_codel`, `noqueue`, `pfifo_fast`), `class-count`, and live `bytes`, `packets`, `qlen`, `backlog`, `drops`, `overlimits`, and `requeues`.
2. **Shaped Linux router running `ze show traffic control eth0 | json` (HTB with classes `control` and `default`):**
   - Before: returns `{"interface": "eth0", "qdisc": "htb", "rate-bps": 0, "class-count": 0, "default-class": ""}` with no class entries and no drop or queue counters.
   - After: returns `interface`, `qdisc: "htb"`, `handle: "1:0"`, `rate-bps: 0`, `class-count: 2`, `default-class: "default"`, a `stats` object (`bytes`, `packets`, `qlen`, `backlog`, `drops`, `overlimits`, `requeues`), and a `classes` array where each entry carries `name` (`"control"`, `"default"`), `handle` (`"1:1"`, `"1:2"`), `rate-bps`, `ceil-bps`, `priority`, and its own `stats` object.
3. **Prometheus `GET /metrics` on a Linux router with `telemetry { prometheus { enabled true; } }`:**
   - Before: exposes interface-level `ze_interface_tx_dropped_total` and `netdata_net_drops_drops_persec_average`, with zero per-qdisc or per-class queue depth, backlog, overlimit, or drop metrics.
   - After: exposes 7 `ze_traffic_qdisc_*` `GaugeVec` series (`interface`, `qdisc`, `handle`) and 7 `ze_traffic_class_*` `GaugeVec` series (`interface`, `qdisc`, `class`, `handle`) pushed every second, independent of `netdata.enabled`, with stale interface/class series removed automatically.
4. **VPP router running `ze show traffic control`:**
   - Before: returns configured qdisc type and class count with no stats.
   - After: preserves configured qdisc type and class names and omits `stats` (and emits no `ze_traffic_qdisc_*` / `ze_traffic_class_*` Prometheus series), never reporting synthetic zeros.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Refresh qdisc metrics on scrape via a `PrometheusRegistry.OnGather` pull hook | Conflicts with `docs/architecture/decisions/001-pull-model-metrics.md` ("Do not introduce a half pull-model beside the registry model"); a 1s periodic push poller obeys the push-model decision and also samples instantaneous `qlen`/`backlog` at 1 Hz instead of only on 15s scrapes. |
| Add a Netdata-style 1s `qdisc` collector in `internal/component/telemetry/collector/` alongside CLI readback in `trafficnetlink` | Duplicates netlink qdisc/class parsing in two packages (`telemetry/collector` cannot import `traffic`), loses Ze class names (`control`/`default`) in Prometheus, and turns off when `netdata.enabled false`. |
| Export raw kernel qdisc/class counters as Prometheus `CounterVec` | Kernel `uint32` counters wrap and reset to zero whenever a qdisc is replaced or an interface bounces, which violates `Counter`'s monotonic invariant; `GaugeVec` with `_total` matches `internal/component/iface/rate.go` (`ze_interface_rx_dropped_total`). |
| Add `ListAllQdiscs` directly onto `traffic.Backend`, or split `QdiscLister` and `QdiscReader` into two interfaces | Breaks existing test mocks in `internal/component/l2tp/plugins/shaper` and `internal/component/l2tp/plugins/authradius`, or adds redundant interface ceremony; one unified `QdiscReader` interface (`ListQdiscs`, `ListAllQdiscs`) covers both active backends and the stateless read-only reader. |
| Call `QdiscList(link)` + `ClassList(link)` in a loop over every interface (`1 + 2N` syscalls/s) | Scales poorly on routers with hundreds of `pppN`/`veth` links and creates vanished-link races; `QdiscList(nil)` dumps every interface's qdiscs in 1 syscall and `ClassList` runs only for classful roots. |

### Documents departed from

- `docs/plugin-development/metrics.md`, section `Type-Specific Rules`: currently states `Gauge | ze_{scope}_{subject}_{qualifier} | Never _total` and `GaugeVec | Same as Gauge, labels for dimensions | No _total`. After this work, `Type-Specific Rules` explicitly documents the raw-kernel-counter rule already practiced by `internal/component/iface/rate.go` (`ze_interface_rx_dropped_total`, `ze_interface_tx_dropped_total`): raw cumulative OS/kernel counters that can reset on interface bounce or qdisc replacement are exported as `GaugeVec` with `_total`, while instantaneous gauges (`qlen`, `backlog_bytes`) carry no `_total`.

### Choices for Thomas

| # | Choice | Options | Decision |
|---|--------|---------|----------|
| C-1 | Should live Linux qdisc stats work when `traffic { control { } }` is absent? | (a) Yes, via stateless read-only kernel reader (recommended); (b) Only when `traffic { control { } }` is configured | (a) Confirmed 2026-11-27 |
| C-2 | Metric type and naming for raw cumulative kernel counters (`bytes`, `packets`, `drops`, `overlimits`, `requeues`) | (a) `GaugeVec` with `_total` matching `iface/rate.go`, no `_total` on `qlen`/`backlog_bytes` (recommended); (b) `GaugeVec` without `_total` | (a) Confirmed 2026-11-27 |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- add `QdiscStats`/`Kind`/`Handle` model fields, unified `QdiscReader` + `RegisterReader`/`ReadQdiscs`/`ReadAllQdiscs` seam on `traffic`, `metrics.RegisterPushCollector` / `StartPushCollectors` seam wired in `telemetry/exporter`, and failing wiring tests
   - Tests: `TestPushCollectorRegisterStartAndStop`, `TestExporterStartsAndStopsPushCollectorsWithoutNetdata`, `TestReadQdiscsUsesActiveBackendElseDefaultReader`, `TestShowTrafficControlWiredWithStats`, `TestQdiscMetricsEndToEndHTTPScrape`
   - Files: `internal/core/metrics/push_collector.go`, `internal/component/telemetry/exporter/exporter.go`, `internal/component/traffic/model.go`, `internal/component/traffic/backend.go`, `internal/component/traffic/metrics.go`, `internal/component/traffic/cmd/traffic.go`, `internal/plugins/traffic/netlink/register.go`
   - Verify: entry points are registered and reachable; wiring tests fail until kernel readback and metrics refresh are implemented
2. **Phase: Kernel & VPP Readback** -- implement live qdisc/class stats readback, real kernel qdisc kind, Ze class-name tracking, `ListAllQdiscs`, stateless read-only kernel reader, and VPP `ListAllQdiscs` (`Stats: nil`)
   - Tests: `TestQdiscKindNamePreservesKernelKind`, `TestNetlinkListQdiscsStatsClassesAndKind`, `TestNetlinkIntegration_QdiscAndClassLiveStats`, `TestVPPListAllQdiscsLeavesStatsNil`
   - Files: `internal/plugins/traffic/netlink/ops_linux.go`, `internal/plugins/traffic/netlink/translate_linux.go`, `internal/plugins/traffic/netlink/backend_linux.go`, `internal/plugins/traffic/netlink/register.go`, `internal/plugins/traffic/vpp/backend_linux.go`
   - Verify: unit tests and Linux netns integration test pass against real kernel veth qdiscs and classes
3. **Phase: CLI Output & Prometheus Metrics** -- complete `handleShowTraffic` summary/detail payload, `FormatQoS` stats formatting, 1s `ze_traffic_qdisc_*` / `ze_traffic_class_*` push poller with stale-label cleanup and clean `Stop()`, functional `.ci` test, and documentation updates
   - Tests: `TestShowTrafficControlWiredWithStats`, `TestFormatQoSWithStats`, `TestQdiscMetricsEndToEndHTTPScrape`, `TestQdiscMetricsStaleLabelCleanupAndReset`, `TestQdiscMetricsOmitsNilStatsAndLifecycleStop`, `test/traffic/traffic-qdisc-stats.ci`
   - Files: `internal/component/traffic/cmd/traffic.go`, `internal/component/traffic/cmd/show.go`, `internal/component/traffic/metrics.go`, `test/traffic/traffic-qdisc-stats.ci`, `docs/guide/traffic-control.md`, `docs/guide/command-reference.md`, `docs/plugin-development/metrics.md`, `docs/architecture/api/commands.md`, `features/traffic-control-lifecycle.md`
   - Verify: scoped unit tests, Linux integration test, `.ci` functional test, `./le cli grammar`, and `./le doc check all` pass

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-8 has an implementation at `file:line` and a passing test |
| Feature completeness | Every user story (unshaped CLI, shaped CLI with Ze class names, `/metrics` scrape, VPP omission) works end-to-end |
| Correctness | `noqueue`/`pfifo_fast`/`mq` never misreported as `htb`; HTB class readback checks `bytes/s * 8` overflow; `Stats == nil` omitted on VPP and nil-statistics qdiscs |
| Naming | All JSON keys lowercase kebab-case (`rate-bps`, `ceil-bps`, `class-count`, `default-class`, `bytes`, `packets`, `qlen`, `backlog`, `drops`, `overlimits`, `requeues`); all Prometheus series `ze_traffic_qdisc_*` / `ze_traffic_class_*` |
| Data flow | `internal/core/metrics` has zero upward imports; `telemetry/exporter` never imports `traffic`; `traffic` never hardcodes backend names; `trafficnetlink` resolves logical names via `resolveOSName` |
| Operator errors and doctor codes | Missing backend/reader on non-Linux returns clear operator error; no silent zero fabrication |
| Rule: `ai/rules/goroutine-lifecycle.md` | 1s push poller owns explicit `stopCh` + `sync.WaitGroup`, joined synchronously in `exporterCloser.Close()` |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Live kernel qdisc/class stats readback and Ze class-name mapping | `./le test unit pkg path ./internal/plugins/traffic/netlink/` (including netns integration test) |
| Read-only `QdiscReader`, `metrics.RegisterPushCollector`, and `ze_traffic_qdisc_*` / `ze_traffic_class_*` push poller | `./le test unit pkg path ./internal/component/traffic/...`, `./le test unit pkg path ./internal/core/metrics/`, `./le test unit pkg path ./internal/component/telemetry/exporter/` |
| VPP `ListAllQdiscs` omitting stats (`Stats == nil`) | `./le test unit pkg path ./internal/plugins/traffic/vpp/` |
| End-to-end CLI and `/metrics` proof | `./le test traffic` (`test/traffic/traffic-qdisc-stats.ci`) |
| Documentation and command/metric inventory updates | `./le doc check all` and `./le cli grammar` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Bounded polling concurrency | Single poller goroutine per exporter instance with deterministic `Stop()` + `wg.Wait()` cleanup |
| Unbounded metric cardinality | Stale label tuples deleted on every 1s tick; label values are kernel interface names, qdisc kinds, handles, and Ze class names (never untrusted packet contents) |
| Read-only guarantee | Read-only `"kernel"` `QdiscReader` holds no mutating methods and never touches `statestore` or qdisc snapshots |

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
| Refresh `ze_traffic_qdisc_*` / `ze_traffic_class_*` via 1s push poller registered with `metrics.RegisterPushCollector` and driven by `exporter.Start` / `exporterCloser.Close` | Scrape-time `OnGather` pull hook; Netdata 1s collector in `telemetry/collector` | Complies with `001-pull-model-metrics.md` push model, keeps `PrometheusRegistry` untouched, samples `qlen`/`backlog` at 1 Hz, works without `traffic { control { } }` and with `netdata.enabled false`, and cleanly joins on shutdown |
| Export raw cumulative kernel counters as `GaugeVec` with `_total` suffix | `CounterVec`; `GaugeVec` without `_total` | Matches `internal/component/iface/rate.go` (`ze_interface_rx_dropped_total`); safely handles kernel `uint32` wrap and reset on qdisc replace |
| Single unified `QdiscReader` interface + registered read-only kernel reader | Adding `ListAllQdiscs` to `traffic.Backend`; splitting `QdiscLister` and `QdiscReader` | Preserves every external mock of `traffic.Backend`, eliminates redundant interface ceremony, and never touches snapshot persistence for read-only queries |
| Bulk `QdiscList(nil)` dump + skip `ClassList` on classless roots + shared `extractQdiscStats`/`statsGaugeSet`/`qdiscStatsMap` helpers | Per-interface `QdiscList(link)` + `ClassList` loop (`1 + 2N` syscalls/s); 14x duplicated gauge Set/Delete calls | Reduces 1s polling to 2 netlink dumps plus 1 per classful root, eliminates vanished-link errors on classless interfaces, removes metric/CLI boilerplate, and reuses preallocated key maps each tick |

## Known Limitations
- Filter disassembly on kernel readback (`TrafficClass.Filters`) is out of scope for this spec; `filter-count` reflects readback filter entries as today.
- VPP policer conform/exceed/violate counters are not Linux egress qdisc queues and remain out of scope (tracked separately under VPP telemetry work such as `plan/post-release/features/spec-vpp-telemetry-stat-groups.md`).

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
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI reports gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N/A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented
- Added generic push-collector lifecycle seam (`PushCollectorStarter`, `RegisterPushCollector`, `StartPushCollectors`, `SnapshotPushCollectorsForTest`) in `internal/core/metrics/push_collector.go` with unit tests in `push_collector_test.go`.
- Wired `metrics.StartPushCollectors(reg, log)` in `internal/component/telemetry/exporter/exporter.go` (`Start`) independent of `cfg.Netdata.Enabled`, and stopped push collectors in `exporterCloser.Close()`, with unit test in `exporter_test.go`.
- Extended `internal/component/traffic/model.go` with `QdiscStats`, `Qdisc.Kind`, `Qdisc.Handle`, `Qdisc.Stats`, `Qdisc.KindName()`, `TrafficClass.Handle`, and `TrafficClass.Stats` (tested in `model_test.go`).
- Added unified `QdiscReader` interface (`ListQdiscs`, `ListAllQdiscs`), `ErrNoBackendAvailable`, `RegisterReader`, `ReadQdiscs`, `ReadAllQdiscs`, and `ResetBackendsForTest` in `internal/component/traffic/backend.go` (preserving `Backend`'s 3-method signature; tested in `backend_test.go`).
- Implemented the 1-second `qdiscMetricsPoller` and shared `statsGaugeSet` helper in `internal/component/traffic/metrics.go` (registered from `register.go` `init()`), exporting all 14 `ze_traffic_qdisc_*` and `ze_traffic_class_*` `GaugeVec` series with preallocated map-swap stale-label cleanup, nil-stats omission, registered doctor code `doctor-traffic-qdisc-metrics-read-failed`, 60-second WARN rate-limiting, and `report.RaiseWarningFix`/`report.ClearWarning` lifecycle (tested in `metrics_test.go`).
- Updated `internal/component/traffic/cmd/traffic.go` (`handleShowTraffic`, `buildTrafficDetail`, `buildTrafficSummaryRow`, `addQdiscStatsFields`, `qdiscStatsMap`) to query `ReadQdiscs`/`ReadAllQdiscs` and emit live `stats`, `handle`, `classes`, and real qdisc kind (omitting `stats` when nil), plus `FormatQoS` / `formatStatsLine` in `show.go` (tested in `traffic_test.go`, `show_test.go`).
- Extended `internal/plugins/traffic/netlink/{ops,translate,backend}_linux.go`, `backend_other.go`, and `register.go` with `linkList`, bulk `qdiscList(nil)` readback, classless-root `classList` skip, vanished-link `ENOENT`/`ENODEV` tolerance, `extractQdiscStats`, Ze `classNames`/`defaultClasses` tracking (cleared on root replacement, `RestoreOriginal`, and `Close`), HTB rate/ceil overflow guards, `raiseQdiscType` zero-value fix (removing bogus `QdiscHTB` fallback), and stateless `"kernel"` `readOnlyReader` registration (tested in `backend_linux_test.go` and `stats_linux_test.go`, including real Linux kernel `userns` integration).
- Implemented `ListAllQdiscs` on `internal/plugins/traffic/vpp/backend_linux.go` returning sorted tracked VPP interfaces and classes with `Stats == nil` (tested in `apply_test.go`).
- Added end-to-end functional test `test/traffic/traffic-qdisc-stats.ci`, updated docs (`docs/guide/traffic-control.md`, `docs/guide/command-reference.md`, `docs/plugin-development/metrics.md`, `docs/architecture/api/commands.md`, `features/traffic-control-lifecycle.md`), and wrote news fragment `website/changes/fragments/traffic-qdisc-stats.md`.

### Bugs Found/Fixed
- **Doctor-coverage & 1 Hz WARN spam (`internal/component/traffic/metrics.go`, Run 1 BLOCKER 1):** `qdiscMetricsPoller.collect` initially logged `p.log.Warn("traffic qdisc metrics read failed")` without a registered doctor code or rate limit. Fixed by registering `doctor-traffic-qdisc-metrics-read-failed` via `diagnostic.Register` + `diagnostic.RegisterEventCode`, attaching `"code"`, rate-limiting to 60s via `p.clk.Now()`, raising/clearing `report.RaiseWarningFix`/`report.ClearWarning`, and adding regression test `TestQdiscMetricsReadErrorDoctorCodeAndRateLimit`.
- **Weakened-test detector false positive (`internal/plugins/traffic/netlink/backend_linux_test.go`, Run 1 BLOCKER 2):** `fakeTCOps.qdiscList` opened with an early `if link == nil` return before existing per-link lines, triggering `./le test weakened check`. Fixed by inverting to positive `if link != nil` so the per-link path remains primary; `./le test weakened check` now reports 0 findings in spec files.
- **Stale class metadata on partial apply failure (`internal/plugins/traffic/netlink/backend_linux.go`, Run 1 NOTE 1):** `applyInterface` now clears `b.classNames[osName]` and `b.defaultClasses[osName]` immediately after `replaceRootQdisc` succeeds, before installing new classes, so a failed `classAdd` cannot leave old Ze class names attached to the new root qdisc.

### Documentation Updates
- `docs/guide/traffic-control.md` — added `## Live Qdisc and Class Statistics` with source anchors (`backend.go`, `metrics.go`, `cmd/traffic.go`, `cmd/show.go`, `netlink/backend_linux.go`, `vpp/backend_linux.go`).
- `docs/guide/command-reference.md` — `### show traffic control` updated with live stats, `handle`, `classes`, unconfigured Linux readback, and VPP omission (already landed in HEAD commit `ad2c5de4b7`; omitted from Commit A so foreign hunks are not carried).
- `docs/plugin-development/metrics.md` — updated `Type-Specific Rules` (departed-from rule: raw cumulative OS/kernel counters as `GaugeVec` with `_total`) and added all 14 `ze_traffic_qdisc_*` / `ze_traffic_class_*` rows to Full Inventory with source anchors (`hunks` patch isolating sibling-session `ze_aaa_login_service_refused_total` hunk).
- `docs/architecture/api/commands.md` — updated `ze-traffic:show-traffic` response shape row with source anchor (`hunks` patch isolating sibling-session hunks).
- `features/traffic-control-lifecycle.md` — added `test/traffic/traffic-qdisc-stats.ci` to Real-path tests and updated Description with live stats and Prometheus push collector anchors.
- `website/changes/fragments/traffic-qdisc-stats.md` — created operator news fragment (`topic: Telemetry`, `feature: traffic-control-lifecycle`).
- `./le doc check all`: 0 failures in any spec-owned doc or anchor (all source anchors resolved; only unrelated pre-existing `plugin format` catalog drift, `ze-policy-conf` char-cap, and unrelated BGP/policy anchors failed).
- `./le doc ste check` on `website/changes/fragments/traffic-qdisc-stats.md`, `docs/guide/traffic-control.md`, `features/traffic-control-lifecycle.md`, and `docs/plugin-development/metrics.md`: `ste check: OK -- no habit grew in 4 changed document(s)`.

### Deviations from Plan
- **Functional `.ci` vs `userns` Linux integration split:** `test/traffic/traffic-qdisc-stats.ci` boots an unprivileged Linux daemon verifying unshaped `lo` (`noqueue`, `handle="0:0"`, all 7 live counters via `show traffic control`, `show traffic control lo`, and HTTP `/metrics` with `netdata.enabled false`), while privileged kernel HTB shaping + DSCP UDP packet forwarding + live `ze_traffic_qdisc_*` and `ze_traffic_class_*` HTTP `/metrics` scrape is proven in `TestNetlinkIntegration_QdiscAndClassLiveStats` (`userns.Enter(t)`). Local VM-privileged `.ci` execution was unavailable because runtime kernel 7.3 is not cached locally.
- **Additional doctor code and regression test:** Run 1 closure review surfaced missing doctor-code coverage on the 1s metrics poller WARN path; `doctor-traffic-qdisc-metrics-read-failed` and `TestQdiscMetricsReadErrorDoctorCodeAndRateLimit` were added beyond the original TDD table.
- **`docs/guide/command-reference.md` omitted from Commit A:** our `### show traffic control` edit already landed in HEAD (`ad2c5de4b7`), leaving only unrelated sibling-session hunks in the working tree.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | `internal/component/traffic/metrics.go` logged `p.log.Warn("traffic qdisc metrics read failed")` without a doctor code or rate limit on the 1s ticker | Every shipped `Warn`/`Error` under `internal/component/` requires a declared doctor code (`./le arch doctor-coverage check`) and periodic ticker errors must rate-limit and raise/clear a report-bus warning | Run 1 independent closure review (`./le arch doctor-coverage check`) | Registered `doctor-traffic-qdisc-metrics-read-failed` (`diagnostic.Register` + `RegisterEventCode`), added 60s rate limit + `report.RaiseWarningFix`/`ClearWarning`, and added `TestQdiscMetricsReadErrorDoctorCodeAndRateLimit` |
| approach | New doc prose in `docs/guide/traffic-control.md`, `docs/plugin-development/metrics.md`, `features/traffic-control-lifecycle.md`, and the news fragment exceeded STE 25-word sentence limits | `./le doc ste check` enforces ASD-STE100 sentence length and active voice on every edited doc file | Run 2 closure check (`./le doc ste check`) | Tightened prose across all 4 files and refreshed `docs-metrics.patch`; `./le doc ste check` passed with 0 growth |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Expose live qdisc and per-class stats (`bytes`, `packets`, `qlen`, `backlog`, `drops`, `overlimits`, `requeues`) on Linux | Done | `internal/plugins/traffic/netlink/backend_linux.go` (`extractQdiscStats`, `buildInterfaceQoS`, `readClassesForRoot`) | All 7 counters read from `Attrs().Statistics` |
| Report real kernel qdisc kind instead of `htb` fallback | Done | `internal/plugins/traffic/netlink/translate_linux.go` (`raiseQdiscType`), `internal/component/traffic/model.go` (`Qdisc.KindName`) | Returns `0` for unknown enum while preserving `Qdisc.Kind = root.Type()` |
| Work on unshaped Linux routers without `traffic { control { } }` or `interface { }` | Done | `internal/component/traffic/backend.go` (`QdiscReader`, `RegisterReader`, `ReadQdiscs`, `ReadAllQdiscs`), `internal/plugins/traffic/netlink/register.go` | Stateless read-only `"kernel"` reader registered in `init()` |
| Expose live stats via `show traffic control`, `ze traffic control show interface <name>`, and Prometheus `/metrics` | Done | `internal/component/traffic/cmd/{traffic,show}.go`, `internal/core/metrics/push_collector.go`, `internal/component/telemetry/exporter/exporter.go`, `internal/component/traffic/metrics.go` | Structured kebab-case `plugin.Map` + 14 `GaugeVec`s via 1s push poller |
| Omit stats on backends without Linux qdisc queues (VPP) without fabricating zeros | Done | `internal/plugins/traffic/vpp/backend_linux.go` (`ListQdiscs`, `ListAllQdiscs`), `internal/component/traffic/cmd/traffic.go`, `internal/component/traffic/metrics.go` | `Stats == nil` omitted from JSON and Prometheus |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestNetlinkListQdiscsStatsClassesAndKind`, `TestQdiscKindNamePreservesKernelKind` | Verified bulk `QdiscList(nil)`, classless skip, vanished `ENOENT`/`ENODEV` skip, real kind, handle, 7 stats, nil preservation |
| AC-2 | Done | `TestNetlinkListQdiscsStatsClassesAndKind` (`htb-apply-and-class-readback`, `htb-class-rate-ceil-boundary-and-overflow`) | Verified Ze class names, handle fallback, sorted classes, HTB rate/ceil bps conversion + overflow refusal, `DefaultClass` |
| AC-3 | Done | `TestReadQdiscsUsesActiveBackendElseDefaultReader`, `TestShowTrafficControlWiredWithStats`, `test/traffic/traffic-qdisc-stats.ci` | Verified unconfigured Linux read-only reader fallback, summary/detail payload, and non-Linux error contract |
| AC-4 | Done | `TestVPPListAllQdiscsLeavesStatsNil`, `TestShowTrafficControlWiredWithStats` | Verified VPP tracked interfaces/classes preserve existing keys and omit `stats` |
| AC-5 | Done | `TestPushCollectorRegisterStartAndStop`, `TestExporterStartsAndStopsPushCollectorsWithoutNetdata`, `TestQdiscMetricsEndToEndHTTPScrape`, `TestQdiscMetricsOmitsNilStatsAndLifecycleStop`, `test/traffic/traffic-qdisc-stats.ci` | Verified all 14 `GaugeVec`s on `/metrics`, `netdata.enabled false`, clean `Close()` stop, VPP omission |
| AC-6 | Done | `TestQdiscMetricsStaleLabelCleanupAndReset` | Verified stale interface/class deletion and downward counter resets |
| AC-7 | Done | `TestNetlinkIntegration_QdiscAndClassLiveStats` | Real Linux `userns` veth HTB + CS6 UDP traffic proving `Bytes > 0`, `Packets > 0` on root & `control` class + live `/metrics` scrape |
| AC-8 | Done | `TestFormatQoSWithStats` + existing `TestFormatQoS`, `TestFormatQoSRootTBFRate` | Verified byte-identical nil-stats output and readable non-nil `stats` line |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestPushCollectorRegisterStartAndStop` | Done | `internal/core/metrics/push_collector_test.go` | PASS |
| `TestReadQdiscsUsesActiveBackendElseDefaultReader` | Done | `internal/component/traffic/backend_test.go` | PASS |
| `TestQdiscKindNamePreservesKernelKind` | Done | `internal/component/traffic/model_test.go` | PASS |
| `TestQdiscMetricsEndToEndHTTPScrape` | Done | `internal/component/traffic/metrics_test.go` | PASS |
| `TestQdiscMetricsStaleLabelCleanupAndReset` | Done | `internal/component/traffic/metrics_test.go` | PASS |
| `TestQdiscMetricsOmitsNilStatsAndLifecycleStop` | Done | `internal/component/traffic/metrics_test.go` | PASS |
| `TestQdiscMetricsReadErrorDoctorCodeAndRateLimit` | Done | `internal/component/traffic/metrics_test.go` | Added for Run 1 BLOCKER 1 fix; PASS |
| `TestExporterStartsAndStopsPushCollectorsWithoutNetdata` | Done | `internal/component/telemetry/exporter/exporter_test.go` | PASS |
| `TestShowTrafficControlWiredWithStats` | Done | `internal/component/traffic/cmd/traffic_test.go` | PASS |
| `TestFormatQoSWithStats` | Done | `internal/component/traffic/cmd/show_test.go` | PASS |
| `TestNetlinkListQdiscsStatsClassesAndKind` | Done | `internal/plugins/traffic/netlink/stats_linux_test.go` | PASS |
| `TestNetlinkIntegration_QdiscAndClassLiveStats` | Done | `internal/plugins/traffic/netlink/stats_linux_test.go` | Real Linux `userns` integration; PASS |
| `TestVPPListAllQdiscsLeavesStatsNil` | Done | `internal/plugins/traffic/vpp/apply_test.go` | PASS |
| `traffic-qdisc-stats` | Done | `test/traffic/traffic-qdisc-stats.ci` | 80/80 stress pass + promoted suite PASS |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/core/metrics/{push_collector.go,push_collector_test.go}` | Done | Created |
| `internal/component/telemetry/exporter/{exporter.go,exporter_test.go}` | Done | Modified |
| `internal/component/traffic/{model.go,model_test.go,backend.go,backend_test.go,register.go}` | Done | Modified |
| `internal/component/traffic/{metrics.go,metrics_test.go}` | Done | Created |
| `internal/component/traffic/cmd/{traffic.go,show.go,show_test.go}` | Done | Modified |
| `internal/component/traffic/cmd/traffic_test.go` | Done | Created |
| `internal/plugins/traffic/netlink/{ops_linux.go,translate_linux.go,backend_linux.go,backend_other.go,register.go,backend_linux_test.go}` | Done | Modified |
| `internal/plugins/traffic/netlink/stats_linux_test.go` | Done | Created |
| `internal/plugins/traffic/vpp/{backend_linux.go,apply_test.go}` | Done | Modified |
| `test/traffic/traffic-qdisc-stats.ci` | Done | Created and promoted |
| `docs/guide/traffic-control.md` | Done | Modified (`ste check: OK`) |
| `docs/guide/command-reference.md` | Done | Already in HEAD (`ad2c5de4b7`); omitted from Commit A |
| `docs/plugin-development/metrics.md` | Done | Modified (Commit A carries our 2 hunks via `hunks` patch; `ste check: OK`) |
| `docs/architecture/api/commands.md` | Done | Modified (Commit A carries our 1 hunk via `hunks` patch) |
| `features/traffic-control-lifecycle.md` | Done | Modified (`ste check: OK`) |
| `website/changes/fragments/traffic-qdisc-stats.md` | Done | Created news fragment (`ste check: OK`) |

### Audit Summary
- **Total items:** 31 planned files + 1 news fragment (1 doc already in HEAD), 8 ACs, 13 unit/integration tests + 1 `.ci` functional test
- **Done:** All items
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 3 minor items recorded in Deviations (unprivileged `.ci` + `userns` integration split; added `doctor-traffic-qdisc-metrics-read-failed` + test; `docs/guide/command-reference.md` already in HEAD)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Live Linux kernel qdisc and per-class stats (`bytes`, `packets`, `qlen`, `backlog`, `drops`, `overlimits`, `requeues`) with real qdisc kinds and Ze class names | unit + real Linux `userns` integration | `./le test unit pkg path ./internal/plugins/traffic/netlink/` (`PASS`, `11.809s`): `TestNetlinkListQdiscsStatsClassesAndKind` + `TestNetlinkIntegration_QdiscAndClassLiveStats` sending 20 CS6 UDP packets through HTB `control`/`default` classes and asserting `Bytes > 0`, `Packets > 0` |
| `show traffic control` works on unshaped Linux routers without `traffic { control { } }` and omits stats on VPP | unit + functional `.ci` | `./le test unit pkg path ./internal/component/traffic/cmd/` (`PASS`, `1.045s`, `TestShowTrafficControlWiredWithStats`); `./le test unit pkg path ./internal/plugins/traffic/vpp/` (`PASS`, `1.172s`, `TestVPPListAllQdiscsLeavesStatsNil`); `test/traffic/traffic-qdisc-stats.ci` (`PASS 5 traffic-qdisc-stats`, 80/80 under 64 burners + 16 workers) |
| Prometheus `/metrics` exports 14 `ze_traffic_qdisc_*` / `ze_traffic_class_*` `GaugeVec`s via 1s push poller (independent of `netdata.enabled`) with stale-label cleanup and clean shutdown | unit + real Linux `userns` HTTP scrape + functional `.ci` | `./le test unit pkg path ./internal/core/metrics/` (`1.089s`), `./internal/component/telemetry/exporter/` (`1.276s`), `./internal/component/traffic/` (`1.437s`), `TestNetlinkIntegration_QdiscAndClassLiveStats` live `/metrics` scrape, and `test/traffic/traffic-qdisc-stats.ci` HTTP `/metrics` assertions on all 7 `ze_traffic_qdisc_*` series |
| Offline CLI `ze traffic control show interface <name>` displays live stats while preserving nil-stats formatting | unit | `./le test unit pkg path ./internal/component/traffic/cmd/` (`TestFormatQoSWithStats`, `TestFormatQoS`, `TestFormatQoSRootTBFRate` PASS) |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None — all in-scope items (AC-1..AC-8) are complete. (Out-of-scope VPP policer counters remain separately tracked.) | All acceptance criteria, tests, and documentation updates delivered in this spec | `plan/post-release/features/spec-vpp-telemetry-stat-groups.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/traffic-qdisc-stats-b59e6ce7-d0a1-495c-a70c-96280d21b344.md` |
| `./le spec review check` | `review_gate: OK (25 code files, clean, hashes match tmp/review/traffic-qdisc-stats-b59e6ce7-d0a1-495c-a70c-96280d21b344.md)` |
| Rounds | 2 (Run 1 found 2 BLOCKERs + 1 NOTE; all fixed and re-tested; Run 2 fresh independent closer/reviewer CLEAN: 0 BLOCKER, 0 ISSUE, 1 minor NOTE) |
| Reviewer lenses used | logic+wiring, security+edge-cases, design-documents (14 pages + 16 decision rows), `ze-go-style.md` 7-question pass, `/ze-review-spec` completeness |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | `./le arch doctor-coverage check` failed on `p.log.Warn("traffic qdisc metrics read failed")` (no `code` attribute) and unbounded 1 Hz WARN spam on persistent read errors | `internal/component/traffic/metrics.go` (`qdiscMetricsPoller.collect`) | Declared & registered `doctor-traffic-qdisc-metrics-read-failed` (`diagnostic.Register` + `RegisterEventCode`), attached `"code"` to WARN, rate-limited to 60s via `p.clk.Now()`, added `report.RaiseWarningFix`/`ClearWarning`, added `TestQdiscMetricsReadErrorDoctorCodeAndRateLimit` |
| 2 | BLOCKER | `./le test weakened check` flagged `fakeTCOps.qdiscList` after adding an early `if link == nil` guard ahead of existing per-link statements | `internal/plugins/traffic/netlink/backend_linux_test.go` (`fakeTCOps.qdiscList`) | Inverted to positive `if link != nil` so the existing per-link branch stays primary; `./le test weakened check` now reports 0 findings in spec files |
| 3 | NOTE | `applyInterface` could retain stale `b.classNames`/`b.defaultClasses` if `classAdd` failed after `replaceRootQdisc` replaced the kernel root | `internal/plugins/traffic/netlink/backend_linux.go` (`backend.applyInterface`) | Cleared `b.classNames[osName]` and `b.defaultClasses[osName]` immediately after `replaceRootQdisc` succeeds, before installing classes |

### Run 2 (Final Independent Review)
- **Result:** CLEAN (0 BLOCKER, 0 ISSUE, 1 NOTE).
- **Verified gates:** `./le arch doctor-coverage check` OK, `./le arch operator-error check` OK, `./le arch error-wrap check` OK, `./le arch error-inventory check` OK, `./le arch compound-guard check` (0 in spec files), `./le arch refuse-by-default check` (0 in spec files), `./le test weakened check` (0 in spec files), `./le repo check` (0 in spec files), `./le go style check` across all 24 Go files OK, `./le spec design documents` matches all 14 pages + 16 decision rows (1 `departs` row in `docs/plugin-development/metrics.md` approved by Thomas on 2026-11-27 and updated in Commit A), `./le doc ste check` OK on all 4 edited docs + fragment.
- **7-question `ze-go-style.md` pass:** Ran across all 24 Go files; 0 peer-reachable panics (`init()` registration `BUG:` panics only), bounded worker/maps/rate-limit, conceptual names, `MUST` lifecycle obligations on `StartPushCollectors` and `qdiscMetricsPoller`, no duplicate state, validated constructors, minimal return types.
- **NOTE (non-blocking):** `qdiscMetricsPoller.stop()` does not call `p.clearReadFailure()` on exporter teardown if a read failure warning happened to be active right as Prometheus export was disabled on reload.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/core/metrics/push_collector.go` | Yes | `-rw-r--r-- 1 thomas thomas 2963 Nov 27 18:26 internal/core/metrics/push_collector.go` |
| `internal/core/metrics/push_collector_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 2906 Nov 27 18:23 internal/core/metrics/push_collector_test.go` |
| `internal/component/traffic/metrics.go` | Yes | `-rw-r--r-- 1 thomas thomas 9477 Nov 27 19:44 internal/component/traffic/metrics.go` |
| `internal/component/traffic/metrics_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 15387 Nov 27 19:47 internal/component/traffic/metrics_test.go` |
| `internal/component/traffic/cmd/traffic_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 9019 Nov 27 19:18 internal/component/traffic/cmd/traffic_test.go` |
| `internal/plugins/traffic/netlink/stats_linux_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 22660 Nov 27 19:21 internal/plugins/traffic/netlink/stats_linux_test.go` |
| `test/traffic/traffic-qdisc-stats.ci` | Yes | `-rw-r--r-- 1 thomas thomas 4000 Nov 27 18:57 test/traffic/traffic-qdisc-stats.ci` |
| `website/changes/fragments/traffic-qdisc-stats.md` | Yes | `-rw-r--r-- 1 thomas thomas 1342 Nov 27 20:02 website/changes/fragments/traffic-qdisc-stats.md` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Bulk `QdiscList(nil)`, classless skip, vanished link skip, real kind, handle, 7 stats, nil preservation | `internal/plugins/traffic/netlink/backend_linux.go` (`readAllQdiscs`, `buildInterfaceQoS`, `extractQdiscStats`, `isLinkVanishedErr`), `translate_linux.go` (`raiseQdiscType`, `isClasslessQdisc`, `formatTCHandle`); `./le test unit pkg path ./internal/plugins/traffic/netlink/` PASS (`11.809s`) + `./internal/component/traffic/` PASS (`1.437s`) |
| AC-2 | HTB Ze class names, handle fallback, sorted classes, bps conversion + overflow refusal, `DefaultClass` | `internal/plugins/traffic/netlink/backend_linux.go` (`applyInterface`, `readClassesForRoot`, `resolveDefaultClass`); `TestNetlinkListQdiscsStatsClassesAndKind` PASS (`11.809s`) |
| AC-3 | Unconfigured Linux `show traffic control` works via read-only `"kernel"` reader; non-Linux returns sentinel error | `internal/component/traffic/backend.go` (`RegisterReader`, `ReadQdiscs`, `ReadAllQdiscs`), `cmd/traffic.go` (`handleShowTraffic`), `netlink/register.go`; `TestReadQdiscsUsesActiveBackendElseDefaultReader`, `TestShowTrafficControlWiredWithStats` PASS (`1.045s`), `test/traffic/traffic-qdisc-stats.ci` PASS (80/80 stress + promoted suite) |
| AC-4 | VPP omits `stats` on summary and detail | `internal/plugins/traffic/vpp/backend_linux.go` (`ListQdiscs`, `ListAllQdiscs`, `listQdiscsLocked`); `TestVPPListAllQdiscsLeavesStatsNil` PASS (`1.172s`), `TestShowTrafficControlWiredWithStats` PASS |
| AC-5 | 14 `ze_traffic_qdisc_*` / `ze_traffic_class_*` `GaugeVec`s pushed at 1s independent of `netdata.enabled`, cleanly stopped on `Close()`, omitted when `Stats == nil` | `internal/core/metrics/push_collector.go`, `internal/component/telemetry/exporter/exporter.go`, `internal/component/traffic/metrics.go`; unit suites `metrics/` (`1.089s`), `telemetry/exporter/` (`1.276s`), `traffic/` (`1.437s`) PASS + `test/traffic/traffic-qdisc-stats.ci` PASS |
| AC-6 | Stale interface/class/handle labels deleted on next tick and counter resets downward succeed | `internal/component/traffic/metrics.go` (`deleteStaleAndSwap`); `TestQdiscMetricsStaleLabelCleanupAndReset` PASS (`1.437s`) |
| AC-7 | Real Linux kernel netns integration proves live non-zero `Bytes` and `Packets` on HTB root and `control` class | `internal/plugins/traffic/netlink/stats_linux_test.go` (`TestNetlinkIntegration_QdiscAndClassLiveStats`); PASS (`11.809s`, real `userns` kernel veth + CS6 UDP + HTTP `/metrics` scrape) |
| AC-8 | `FormatQoS` preserves nil-stats output byte-for-byte and appends `stats` line when non-nil | `internal/component/traffic/cmd/show.go` (`FormatQoS`, `formatStatsLine`); `TestFormatQoSWithStats` + `TestFormatQoS` + `TestFormatQoSRootTBFRate` PASS (`1.045s`) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze show traffic control` and `ze show traffic control lo` (`ze-traffic:show-traffic`, unconfigured Linux daemon) | `test/traffic/traffic-qdisc-stats.ci` (seq 3-4) | Yes — read file; boots real Linux daemon with no `traffic` or `interface` blocks, verifies `"interface": "lo"`, `"qdisc": "noqueue"`, `"handle": "0:0"`, `"stats"`, and all 7 counter keys; 80/80 stress pass + promoted suite `PASS 5` |
| `exporter.Start` -> `GET /metrics` with `netdata { enabled false; }` | `test/traffic/traffic-qdisc-stats.ci` (http wait + seq 1-6) | Yes — read file; scrapes live `/metrics` asserting all 7 `ze_traffic_qdisc_*` series for `lo`; plus `TestNetlinkIntegration_QdiscAndClassLiveStats` scrapes live HTB `ze_traffic_qdisc_*` and `ze_traffic_class_*` series against real Linux kernel |
| `ze traffic control show interface <name>` (offline CLI) | `internal/component/traffic/cli/main.go` (`cmdShow` -> `tccmd.FormatQoS`) | Yes — read `cli/main.go` and verified `FormatQoS` call chain tested by `TestFormatQoSWithStats` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `TestNetlinkIntegration_QdiscAndClassLiveStats` (`PASS`, `11.809s`) proved `vishvananda/netlink` `QdiscList`/`ClassList` populate live `Basic` and `Queue` statistics on Ze's Linux kernel |
| A-2 | confirmed | Confirmed by Thomas (2026-11-27); `TestReadQdiscsUsesActiveBackendElseDefaultReader`, `TestShowTrafficControlWiredWithStats`, and `test/traffic/traffic-qdisc-stats.ci` (`PASS 5`) prove unconfigured Linux readback |
| A-3 | confirmed | `TestVPPListAllQdiscsLeavesStatsNil`, `TestShowTrafficControlWiredWithStats`, and `TestQdiscMetricsOmitsNilStatsAndLifecycleStop` prove VPP leaves `Stats == nil` and emits zero fake metrics |
| A-4 | confirmed | Confirmed by Thomas (2026-11-27); `TestQdiscMetricsStaleLabelCleanupAndReset` proves `GaugeVec` with `_total` handles downward counter resets without error |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Feature declaration (`features/traffic-control-lifecycle.md`) | Verified against `traffic/{register,backend,metrics,cmd/traffic}.go` and `test/traffic/traffic-qdisc-stats.ci`; `./le doc ste check` OK | Yes |
| User guide (`docs/guide/traffic-control.md`) | Verified against `traffic/{backend,metrics,cmd/traffic,cmd/show}.go`, `netlink/backend_linux.go`, `vpp/backend_linux.go`; all 6 source anchors resolve; `./le doc ste check` OK | Yes |
| CLI reference (`docs/guide/command-reference.md`) | Verified live `git show HEAD:docs/guide/command-reference.md` already carries our `### show traffic control` hunk (`ad2c5de4b7`); omitted from Commit A | Yes |
| API/RPC docs (`docs/architecture/api/commands.md`) | Verified against `internal/component/traffic/cmd/traffic.go` (`handleShowTraffic`, `buildTrafficDetail`, `buildTrafficSummaryRow`); `hunks` patch applies cleanly to HEAD | Yes |
| Prometheus metrics docs (`docs/plugin-development/metrics.md`) | Verified all 14 metric names/labels and departed `Type-Specific Rules` raw-kernel-counter rule against `internal/component/traffic/metrics.go` (`newStatsGaugeSet`); `hunks` patch applies cleanly to HEAD; `./le doc ste check` OK | Yes |
| News fragment (`website/changes/fragments/traffic-qdisc-stats.md`) | Verified against `internal/le/commit/newsfragment.go` (`spec: traffic-qdisc-stats`, `topic: Telemetry`, `feature: traffic-control-lifecycle`, 6 headings in order); `./le doc ste check` OK | Yes |
| Spec citers (`git grep -n "spec-traffic-qdisc-stats"`) | Only bare-stem comments exist (`apply_test.go`, `stats_linux_test.go`, `traffic-qdisc-stats.ci`); 0 external `plan/post-release/features/spec-traffic-qdisc-stats.md` path citations remain | Yes |
