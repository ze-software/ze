# Spec: traffic-usage-ipv6

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin, docs |
| Priority | P3 |
| Release reason | Extends `traffic-usage` pure-Go TCX eBPF byte accounting to cover IPv6 (`ETH_P_IPV6`) alongside IPv4 across Prometheus `/metrics`, `show traffic usage`, and `observation.Global()`; existing IPv4 behavior and its obligations remain required. |
| RFC gaps | - |
| Depends | - |
| Phase | 2/2 |
| Handoff | - |
| Updated | 2026-11-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Extend the `traffic-usage` pure-Go TCX eBPF plugin (`internal/plugins/trafficusage`) to account IPv6 (`ETH_P_IPV6`) packets alongside IPv4 so operators running dual-stack or IPv6-heavy WAN links can identify top talkers and per-(port, protocol) traffic volumes (including TCP, UDP, and ICMPv6) in `show traffic usage`, Prometheus `/metrics`, and the anomaly/DDoS `observation.Global()` feed.

Today, `directionInsns` in `internal/plugins/trafficusage/program_linux.go` rejects every non-IPv4 EtherType (`JNE ethTypeIPv4 -> ret0`), `counts` keys per-IP totals by raw IPv4 `uint32`, and `metrics.go` / `show.go` / `monitor.go` assume IPv4-only addresses. On dual-stack routers, IPv6 traffic is silently invisible to `traffic-usage`. This spec adds IPv6 parsing (fixed 40-byte IPv6 base header, shared L4 TCP/UDP port extraction via `bpf_skb_load_bytes` so unprivileged `!CAP_PERFMON` verification stays intact), adds separate 16-byte LRU maps (`tu_ip6_in`, `tu_ip6_out`) only when `track-ip` is enabled, unifies `counts.ingressIP` / `counts.egressIP` onto `map[netip.Addr]uint64` so metrics, CLI output, and `observation.Global()` handle IPv4 and IPv6 through one code path with zero duplication, and updates all `traffic-usage` docs and YANG help strings.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform contracts, pure-Go registration and plugin boundaries
  → Constraint: `traffic-usage` remains a self-contained internal system plugin; no C source, no `.o` artifacts, no clang/LLVM build dependency.
- [ ] `ai/doors/system-plugins.md` - system plugin conventions, build-tag gating (`ze_trafficusage`), and package-internal unexported helpers
  → Constraint: keep new IPv6 eBPF and map helpers unexported inside `internal/plugins/trafficusage`; preserve `ze_trafficusage` compile-out behavior and `!linux` stub cleanliness.
- [ ] `ai/doors/api-surfaces.md` - single Prometheus registry (`/metrics`), metric naming stability, structured kebab-case CLI/API/MCP responses
  → Constraint: preserve existing `ze_traffic_usage_*` metric names and labels (`src_ip`/`dst_ip` accept canonical RFC 5952 IPv6 strings via `netip.Addr.String()`), preserve `show traffic usage` response keys (`ingress-ports`, `egress-ports`, `ingress-ips`, `egress-ips`, `map-entries`), and report `ip6_ingress` / `ip6_egress` under existing `ze_traffic_usage_map_entries`.
- [ ] `ai/doors/config-yang.md` - YANG schema as sole source of truth and backward-compatible schema help updates
  → Constraint: add no new config leaves or syntax changes; update `ze-traffic-usage-conf.yang` descriptions and `ze:help` text from IPv4-only to IPv4 and IPv6.
- [ ] `docs/architecture/traffic/traffic-usage.md` - pure-Go `asm.Instructions` assembly, `!CAP_PERFMON` verifier rules, `BPF_PROG_TEST_RUN`, `TCX_NEXT`, and `track-ip` map omission
  → Constraint: every packet-pointer read uses a fixed immediate offset before any helper call; L4 port reads stay on `bpf_skb_load_bytes`; `tu_ip6_in` / `tu_ip6_out` are omitted from the collection and instruction stream when `track-ip` is false; every exit returns `TCX_NEXT` (`-1`).
- [ ] `docs/guide/traffic-usage.md` - operator guide and limitations for `traffic-usage`
  → Constraint: update Limitations, How It Works, and Metrics tables to document IPv4 + IPv6 support and the fixed 40-byte IPv6 base-header extension-header rule (extension-header packets count per-IP and under `port=0` with the first `NextHeader` protocol number).
- [ ] `docs/plugin-development/metrics.md` - Prometheus metric inventory
  → Constraint: keep the five `ze_traffic_usage_*` series accurate for IPv4 and IPv6.

**Key insights:**
- `directionInsns` in `internal/plugins/trafficusage/program_linux.go` already separates L3 header parsing (`R7=proto`, `R9=L4 scalar offset`, stack-staged IP key) from L4 port extraction (`bpf_skb_load_bytes` at scalar `R9 + portOff`) and map accounting (`accountInsns`). Branching on `ethTypeIPv4` (`0x0008`) vs `ethTypeIPv6` (`0xDD86`, little-endian `htons(0x86DD)`) lets IPv6 reuse the entire `l4_parse` + `build_key` + `accountInsns` path with zero duplication.
- Staging a 16-byte IPv6 address from packet memory (`ethHdrLen+ipv6SrcOff` = `22` / `30`, or `ethHdrLen+ipv6DstOff` = `38` / `46`) requires only two `asm.DWord` loads/stores at fixed immediate offsets inside the checked 54-byte bound (`data + 54 > data_end -> ret0`), using zero register pointer arithmetic and staying strictly compliant with `!CAP_PERFMON` unprivileged verification (`TestTrafficUsageLoadsUnderTheUnitCapabilities`).
- Switching `counts.ingressIP` and `counts.egressIP` from `map[uint32]uint64` to `map[netip.Addr]uint64` eliminates `ipString(uint32)` and lets `monitor.go` (`publishLocked`, `observation.Global()`), `show.go` (`renderIPs`), and stale-series cleanup handle both IPv4 and IPv6 through one single loop each.
- `protoName` in `metrics.go` already maps protocol `58` to `"icmpv6"`.
- `bpf_prog_test_run_skb` in Linux checks `skb_headlen(skb) < sizeof(struct ipv6hdr)` for `ETH_P_IPV6` frames (returning `EINVAL` below 54 bytes), mirroring the existing 34-byte IPv4 boundary; truncated IPv6 frames (`14..53` bytes) are therefore tested over a veth pair with `link.AttachTCX` (`TestProgram_IPv6TruncatedPass`), while the exact 54-byte boundary is proven via `BPF_PROG_TEST_RUN` (`TestProgram_IPv6HeaderEndingTheFrameCounts`).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/trafficusage/program_linux.go` - `buildCollectionSpec` creates 4-byte LRU maps (`tu_port_in`, `tu_port_out`, and optionally `tu_ip_in`, `tu_ip_out`); `directionInsns` checks `ethTypeIPv4` only and exits `TCX_NEXT` on all other EtherTypes
- [ ] `internal/plugins/trafficusage/attach_linux.go` - `tcxAttachment.Counts` reads only `mapPortIngress`, `mapPortEgress`, and (when `trackIP`) `mapIPIngress`, `mapIPEgress` into `map[uint32]uint64`
- [ ] `internal/plugins/trafficusage/monitor.go` - `counts.ingressIP` and `counts.egressIP` are `map[uint32]uint64`; `publishLocked` decodes raw `uint32` via `ipString` and `netip.AddrFrom4` for `observation.Global()`
- [ ] `internal/plugins/trafficusage/show.go` - `renderIPs` iterates `map[uint32]uint64` and formats via `ipString`
- [ ] `internal/plugins/trafficusage/metrics.go` - registers 5 `ze_traffic_usage_*` `GaugeVec`s with IPv4-only help strings on `ingressBytes`/`egressBytes`, `protoName` (already includes `58 -> "icmpv6"`), and `ipString(uint32)`
- [ ] `internal/plugins/trafficusage/trafficusage.go` - package doc states `IPv4 only.`
- [ ] `internal/plugins/trafficusage/yang/ze-traffic-usage-conf.yang` - config descriptions and `ze:help` mention IPv4 only and two extra maps when `track-ip` is true
- [ ] `internal/plugins/trafficusage/program_test.go` and `attach_integration_linux_test.go` - `BPF_PROG_TEST_RUN` and veth TCX tests cover IPv4 only
- [ ] `internal/plugins/trafficusage/monitor_test.go` and `show_test.go` - unit tests construct `counts` with `map[uint32]uint64`

**Behavior to preserve:**
- Pure-Go `asm.Instructions` assembly loaded from memory with zero C, `.o`, or clang/LLVM dependencies.
- All exit paths return `TCX_NEXT` (`-1`), leaving downstream TCX programs and legacy `clsact` filters running.
- Unprivileged `!CAP_PERFMON` verification (`TestTrafficUsageLoadsUnderTheUnitCapabilities`): every packet-pointer read uses a fixed immediate offset before any helper call, and L4 ports are read via `bpf_skb_load_bytes`.
- `track-ip: false` omits ALL IP maps (`tu_ip_in`, `tu_ip_out`, `tu_ip6_in`, `tu_ip6_out`) and emits zero per-IP accounting instructions.
- All existing IPv4 byte counts, IPv4 map names (`tu_ip_in`, `tu_ip_out`, `ip_ingress`, `ip_egress`), port/protocol accounting, stale-series cleanup, and `show traffic usage` response keys remain unchanged.
- Non-IPv4/non-IPv6 frames (ARP, LLDP, etc.) and truncated IPv4/IPv6 headers pass with `TCX_NEXT` without writing to any map.

**Behavior to change:**
- Parameterize `newLRUMap(maxEntries, keySize uint32)` and add 16-byte LRU maps `tu_ip6_in` (`mapIP6Ingress`) and `tu_ip6_out` (`mapIP6Egress`) to `buildCollectionSpec` when `trackIP` is true.
- Extend `directionInsns` to accept both `ethTypeIPv4` (`0x0008`) and `ethTypeIPv6` (`0xDD86`), parsing the fixed 40-byte IPv6 base header (`NextHeader` at offset 6, `Src` at offset 8, `Dst` at offset 24, `L4 offset = 54`) and sharing the existing L4 port extraction (`l4_parse`) and map accounting (`accountInsns`) blocks with zero duplication.
- Extend `tcxAttachment.Counts` in `attach_linux.go` to read both 4-byte IPv4 maps and 16-byte IPv6 maps into unified `map[netip.Addr]uint64` maps (`c.ingressIP`, `c.egressIP`) and record `ip6_ingress` and `ip6_egress` entry counts in `c.mapEntries`.
- Switch `counts.ingressIP` / `counts.egressIP` to `map[netip.Addr]uint64` in `monitor.go` and `show.go`, remove obsolete `ipString(uint32)`, and publish IPv6 `netip.Addr` keys identically to IPv4 in `ze_traffic_usage_ingress_bytes_total`, `ze_traffic_usage_egress_bytes_total`, `show traffic usage`, and `observation.Global()`.
- Update help text and docs (`metrics.go`, `trafficusage.go`, `ze-traffic-usage-conf.yang`, `docs/architecture/traffic/traffic-usage.md`, `docs/guide/traffic-usage.md`, `docs/plugin-development/metrics.md`, `features/traffic-usage.md`).

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Pure-Go TCX eBPF assembly | `internal/plugins/trafficusage/program_linux.go` (`buildCollectionSpec`, `directionInsns`, `accountInsns`, `newLRUMap`) | Builds ingress/egress `SchedCLS` programs and 4-byte LRU maps for IPv4 + L4 ports | Extends: adds `ethTypeIPv6` parse branch feeding shared `l4_parse` and optional 16-byte `tu_ip6_in`/`tu_ip6_out` maps via parameterized `newLRUMap` and shared `accountInsns` |
| Linux TCX map reader | `internal/plugins/trafficusage/attach_linux.go` (`tcxAttachment.Counts`, `readIPMap`) | Iterates `tu_port_*` and `tu_ip_*` into `counts` | Extends: reads `tu_ip_*` (`[4]byte` -> `netip.AddrFrom4`) and `tu_ip6_*` (`[16]byte` -> `netip.AddrFrom16`) into unified `map[netip.Addr]uint64` and records `ip6_ingress` / `ip6_egress` in `mapEntries` |
| Traffic usage monitor & observation publisher | `internal/plugins/trafficusage/monitor.go` (`counts`, `Monitor.publishLocked`, `staleCleanupLocked`) | Publishes `ze_traffic_usage_*` gauges, deletes stale series, and publishes `observation.Observation` with `Flow.Src` | Extends: changes `counts.ingressIP`/`egressIP` key from `uint32` to `netip.Addr` so IPv4 and IPv6 flow through one loop with zero duplication |
| `show traffic usage` RPC handler | `internal/plugins/trafficusage/show.go` (`handleShowTrafficUsage`, `renderIPs`) | Renders `ingress-ports`, `egress-ports`, `ingress-ips`, `egress-ips`, `map-entries` | Extends: updates `renderIPs` to accept `map[netip.Addr]uint64` and format canonical `addr.String()` |
| Prometheus metrics registration | `internal/plugins/trafficusage/metrics.go` (`BindMetrics`, `protoName`) | Registers 5 `ze_traffic_usage_*` `GaugeVec`s and maps protocol numbers including `58 -> "icmpv6"` | Extends: updates `ingressBytes`/`egressBytes` help strings from IPv4-only to IP and deletes obsolete `ipString` |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | The core knows nothing, and everything registers | the door every change reads | "A feature registers from `init()` in its own `register.go` and is discovered." | agrees | All IPv6 eBPF, map readback, CLI, and metrics changes live inside `internal/plugins/trafficusage`; no core or shared package learns about IPv6 traffic-usage accounting |
| `ai/doors/system-plugins.md` | Boundaries | the door covering `internal/plugins/trafficusage/*` | "A plugin owns its whole surface." | agrees | `traffic-usage` owns its eBPF assembly, maps, YANG schema, RPC handler, metrics, and tests inside `internal/plugins/trafficusage` with zero central hardcoding |
| `docs/architecture/traffic/traffic-usage.md` | Pure-Go eBPF, not bpf2go | `// Design:` and anchors of `internal/plugins/trafficusage/{program_linux,attach_linux,monitor,show,metrics,trafficusage}.go`, `program_test.go`; modified by spec | "This plugin builds the program in `asm.Instructions`, loads it with `ebpf.NewCollection`, and attaches it with `link.AttachTCX`." | agrees | Adds IPv6 parsing and optional `tu_ip6_in`/`tu_ip6_out` maps in pure-Go `asm.Instructions` with fixed-offset packet loads before `bpf_skb_load_bytes` (preserving `!CAP_PERFMON` verification), omits all IP maps when `track-ip` is false, and returns `TCX_NEXT` on every path |
| `docs/architecture/observation-feed.md` | Observation Contract | anchored to `internal/plugins/trafficusage/monitor.go` | "`Observation` is a value type (embedded `FlowKey`, no pointers) for zero-allocation publish." | agrees | Unifying `counts.ingressIP` onto `map[netip.Addr]uint64` passes native IPv4 and IPv6 `netip.Addr` directly into `obs.Flow.Src` from `publishLocked` with zero change to the non-blocking zero-allocation publish contract |
| `docs/plugin-development/metrics.md` | Type-Specific Rules | modified by spec | "Ze therefore exports raw kernel counters as `GaugeVec` with the `_total` suffix so resets do not panic." | agrees | Preserves the existing five `ze_traffic_usage_*` `GaugeVec` families and updates inventory notes for IPv4/IPv6 `src_ip`/`dst_ip` and `ip6_ingress`/`ip6_egress` map entries |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- **Kernel data plane:** Linux TCX ingress/egress hooks attached by `tcxAttacher.Attach` (`internal/plugins/trafficusage/attach_linux.go`) running pure-Go eBPF programs `tu_ingress` and `tu_egress`.
- **Control/query plane:** 1 Hz `Monitor.poll` publishing to Prometheus `/metrics` (`ze_traffic_usage_*`) and `observation.Global()`, plus `show traffic usage [name <interface>]` (`ze-trafficusage:show-traffic-usage`).

### Transformation Path
1. A packet enters `tu_ingress` or `tu_egress` on an attached Linux interface.
2. Ethernet bounds (`data + 14 > data_end -> ret0`) and EtherType (`*(u16*)(data+12)`) dispatch to `parse_v4` (`0x0008`), `parse_v6` (`0xDD86`), or `ret0` (`TCX_NEXT`).
3. `parse_v6` checks `data + 54 > data_end -> ret0`, stages the 16-byte source (ingress) or destination (egress) IPv6 address onto `slotIP6Key` via two fixed-offset `DWord` loads/stores and stores `1` in `slotIsIPv6` when `trackIP` is true, loads `R7 = ip6hdr.nexthdr`, sets scalar `R9 = 54`, and enters the shared `l4_parse` path.
4. `l4_parse` checks `skb->len` against TCP (20B) or UDP (8B) header size, reads the 2-byte L4 port via `bpf_skb_load_bytes(skb, R9 + portOff, fp + slotPortBytes, 2)` (leaving port `0` for ICMPv6, extension headers, or truncated L4 headers), and runs `accountInsns` on `tu_port_in` / `tu_port_out`.
5. When `trackIP` is true, `ip_ops` branches on `slotIsIPv6` to run `accountInsns` on `tu_ip_in`/`tu_ip_out` (`slotIPKey`, 4 bytes) or `tu_ip6_in`/`tu_ip6_out` (`slotIP6Key`, 16 bytes), then returns `TCX_NEXT` (`-1`).
6. `tcxAttachment.Counts()` iterates `tu_port_*`, `tu_ip_*` (`[4]byte` -> `netip.AddrFrom4`), and `tu_ip6_*` (`[16]byte` -> `netip.AddrFrom16`) into unified `counts{ingressIP, egressIP map[netip.Addr]uint64}` and populates `mapEntries["ip6_ingress"]` / `mapEntries["ip6_egress"]`.
7. `Monitor.publishLocked` publishes both IPv4 and IPv6 entries to `ze_traffic_usage_ingress_bytes_total` (`src_ip`), `ze_traffic_usage_egress_bytes_total` (`dst_ip`), `ze_traffic_usage_map_entries` (`map`), and `observation.Global()` (`Flow.Src`), while `handleShowTrafficUsage` renders canonical IP strings in `ingress-ips` / `egress-ips`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Linux kernel TCX eBPF ↔ userspace `tcxAttachment` | `cilium/ebpf` `NewCollection`, `AttachTCX`, and `Map.Iterate` over 4-byte (`tu_ip_*`, `tu_port_*`) and 16-byte (`tu_ip6_*`) LRU hash maps | Yes |
| `trafficusage.Monitor` ↔ Prometheus `/metrics` & `observation.Global()` | `metrics.GaugeVec.With(...).Set(...)` and `observation.Global().Publish(obs)` with `netip.Addr` | Yes |
| `trafficusage` ↔ CLI / SSH / REST / gRPC / MCP | Registered RPC `ze-trafficusage:show-traffic-usage` returning kebab-case `plugin.Map` | Yes |

### Changed-Wire Census

**Search evidence:** Searched `internal/`, `cmd/`, `pkg/`, `test/`, and `docs/` with `grep -rn "show-traffic-usage\|ze_traffic_usage\|tu_ip_in\|ip_ingress"`. Single RPC writer `handleShowTrafficUsage` (`internal/plugins/trafficusage/show.go:27`), single Prometheus writer `Monitor.publishLocked` (`internal/plugins/trafficusage/monitor.go:405`), and showcase validator `checkTopicTrafficUsage` (`internal/le/site/terminaldemo/validate_topic_traffic.go`). No wire schema keys or metric names change; IPv6 addresses appear as canonical strings in existing `src_ip`/`dst_ip`/`ip` fields, and `ip6_ingress`/`ip6_egress` appear as additional keys in `map-entries` and `ze_traffic_usage_map_entries{map=...}` when `track-ip` is enabled.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `ze-trafficusage:show-traffic-usage` JSON response | writer | `internal/plugins/trafficusage/show.go` (`handleShowTrafficUsage`, `renderInterface`, `renderIPs`) | Single registered writer | Preserve shape; IPv6 addresses populate existing `ingress-ips`/`egress-ips` arrays as RFC 5952 strings and `map-entries` adds `ip6_ingress`/`ip6_egress` when `track-ip` is true |
| `ze-trafficusage:show-traffic-usage` JSON response | reader | Generic CLI/API/MCP dispatcher and `internal/le/site/terminaldemo/validate_topic_traffic.go` | Generic `plugin.Map` consumers + IPv4 showcase assertions | Preserve: existing IPv4 keys/values unchanged; additive IPv6 entries compatible with all readers |
| Prometheus `/metrics` `ze_traffic_usage_*` | writer | `internal/plugins/trafficusage/monitor.go` (`Monitor.publishLocked`) + `metrics.go` (`BindMetrics`) | Single writer for the 5 `ze_traffic_usage_*` families | Preserve metric names and label sets; `src_ip`/`dst_ip` now also carry IPv6 strings and `map` adds `ip6_ingress`/`ip6_egress` when `track-ip` is true |
| Prometheus `/metrics` `ze_traffic_usage_*` | reader | External Prometheus scrapers and `show metrics list` / `show metrics values` (`internal/core/metrics/prometheus.go` `Names`, `Handler`) | Standard Prometheus exposition readers | Preserve: standard Prometheus gauge exposition; existing IPv4 label values unchanged, IPv6 strings and `ip6_*` map names are additive |
| `observation.Global()` feed | writer | `internal/plugins/trafficusage/monitor.go` (`Monitor.publishLocked`) | `obs.Flow.Src` is already `netip.Addr` | Preserve: IPv6 source addresses flow into `ddos-detect` / anomaly observers using native `netip.Addr` |
| `observation.Global()` feed | reader | `internal/plugins/anomaly/` and `internal/plugins/ddos/detect/` (`observation.Global().Subscribe`) | Consumers read `obs.Flow.Src` as `netip.Addr` | Preserve: consumers already handle `netip.Addr` (`Is6()` / `/128`), so IPv6 source observations work with zero consumer changes |

### Integration Points
- `internal/plugins/trafficusage/program_linux.go` (`buildCollectionSpec`, `directionInsns`, `newLRUMap`) - pure-Go IPv4 + IPv6 TCX program assembly
- `internal/plugins/trafficusage/attach_linux.go` (`tcxAttachment.Counts`, `readIPMap`, `readIP6Map`) - reads IPv4 and IPv6 BPF LRU maps into `counts`
- `internal/plugins/trafficusage/monitor.go` (`counts`, `publishLocked`) - unified `map[netip.Addr]uint64` metrics and observation publication
- `internal/plugins/trafficusage/show.go` (`renderIPs`) - unified `netip.Addr` CLI/API rendering

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | IPv6 packets traverse the same TCX programs -> `tcxAttachment.Counts` -> `Monitor.publishLocked` / `handleShowTrafficUsage` path as IPv4 |
| No unintended coupling (components stay isolated) | Yes | All changes stay inside `internal/plugins/trafficusage`; zero new cross-package imports |
| No duplicated functionality (extends existing, does not recreate) | Yes | Shared `l4_parse`, `accountInsns`, parameterized `newLRUMap`, unified `map[netip.Addr]uint64` in `counts`, single `publishLocked` and `renderIPs` loops |
| Zero-copy preserved where applicable (refs, not copies) | Yes | eBPF stages IPv6 keys directly onto fixed aligned stack slots via two `DWord` loads with zero per-packet heap allocation |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | No core or shared package changes; uses existing `traffic-usage` plugin, RPC, and metric registrations |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | Searched `internal/` and `cmd/` for `tu_ip_in`, `ip_ingress`, and `traffic-usage`: all map names and metric labels stay private to `internal/plugins/trafficusage`; no central list exists |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Linux eBPF verifier under `!CAP_PERFMON` accepts two fixed-offset 8-byte `DWord` loads from packet memory (`ethHdrLen+8/16` and `ethHdrLen+24/32`) into aligned stack slot `slotIP6Key = -40` after checking `data + 54 > data_end` | `docs/architecture/traffic/traffic-usage.md` ("eBPF traps" & "The capabilities the daemon loads under") | Verifier rejection would require four `Word` loads or `bpf_skb_load_bytes` into stack | `TestProgram_Loads`, `TestProgram_IPv6IngressSrcIP`, `TestProgram_IPv6EgressDstIP`, and `TestTrafficUsageLoadsUnderTheUnitCapabilities` | confirmed |
| A-2 | Separate 16-byte LRU maps (`tu_ip6_in`, `tu_ip6_out`) alongside 4-byte IPv4 maps (`tu_ip_in`, `tu_ip_out`) are preferred over widening IPv4 maps to 16 bytes, and a fixed 40-byte IPv6 base header (extension-header packets counted under `port=0` with first `NextHeader`) is preferred over a multi-hop extension-header loop | Owner decision (2026-11-27, confirmed `(1a, 2a)`) | Would require unifying maps or adding bounded extension-header hops | Confirmed by Thomas (2026-11-27); proven by `TestProgram_TrackIPOffOmitsIPMaps` and `TestProgram_IPv6ExtHeaderPortZero` | confirmed |
| A-3 | `BPF_PROG_TEST_RUN` enforces `skb_headlen >= 54` (`ETH_HLEN + sizeof(struct ipv6hdr)`) for `ETH_P_IPV6` frames, so 54-byte frames run via `BPF_PROG_TEST_RUN` while `14..53`-byte truncated IPv6 frames must be tested over a veth pair with `link.AttachTCX` | `net/bpf/test_run.c` (`bpf_prog_test_run_skb`) and `docs/architecture/traffic/traffic-usage.md` ("eBPF traps") | Truncated test would fail with `EINVAL` if sent via `BPF_PROG_TEST_RUN` | `TestProgram_IPv6HeaderEndingTheFrameCounts` and `TestProgram_IPv6TruncatedPass` | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Endianness bug on `ethTypeIPv6` (`0xDD86` vs `0x86DD`) or 16-byte IPv6 address byte ordering in `DWord` load/store | `TestProgram_IPv6IngressSrcIP` or `TestProgram_IPv6EgressDstIP` reads wrong key or zero bytes | `BPF_PROG_TEST_RUN` asserts exact `[16]byte` key lookup with asymmetric IPv6 addresses (`2001:db8:1:2:3:4:5:6` -> `2001:db8:a:b:c:d:e:f`) so byte swaps fail immediately |
| R-2 | Unprivileged `!CAP_PERFMON` verifier rejects stack slot initialization when `trackIP` is true if `slotIPKey` or `slotIP6Key` is uninitialized on the opposite branch | `VerifierError` in `TestProgram_Loads` or `TestTrafficUsageLoadsUnderTheUnitCapabilities` | `ip_ops` checks `slotIsIPv6` and only passes `&slotIPKey` to `tu_ip_*` when `isIPv6 == 0` and `&slotIP6Key` to `tu_ip6_*` when `isIPv6 == 1`, so each map lookup reads only stack bytes written on that exact branch |
| R-3 | IPv6 `track-ip` doubles per-interface IP LRU map count (4 IP maps instead of 2) under high `max-entries` | Higher kernel BPF memory usage when `track-ip` is enabled | Documented clearly in `docs/guide/traffic-usage.md` and `ze-traffic-usage-conf.yang`; `track-ip` remains off by default and `tu_ip6_*` maps are omitted when `track-ip` is false |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | `traffic-usage` eBPF program could fail verifier load (reported via `doctor-traffic-usage-attach-unknown`, never blocking daemon start) or miscount IPv6/IPv4 bytes; programs are monitoring-only (`TCX_NEXT`) and never drop or alter packets. |
| How is it reverted? | Single commit revert; zero config syntax changes, zero metric name/label breaking changes, zero persisted state changes. |
| Who else touches this path? | `observation.Global()` consumers (`internal/plugins/anomaly/`, `internal/plugins/ddos/`) already consume `netip.Addr` in `obs.Flow.Src`; `internal/le/site/terminaldemo/validate_topic_traffic.go` reads IPv4 series which remain unchanged. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Linux TCX ingress/egress hooks (`tu_ingress` / `tu_egress`) | → | `directionInsns` IPv6 parse → `tu_port_*` + `tu_ip6_*` → `tcxAttachment.Counts` | `TestAttachTCX_CountsIPv6Traffic` and `TestProgram_IPv6*` |
| `Monitor.poll` → Prometheus `/metrics` & `observation.Global()` | → | `Monitor.publishLocked` with `netip.Addr` IPv6 keys → `ze_traffic_usage_*` + `obs.Flow.Src` | `TestMonitorPublishesIPv6MetricsAndObservations` and `TestMetricsScrape` |
| `ze show traffic usage` (`ze-trafficusage:show-traffic-usage`) | → | `handleShowTrafficUsage` → `renderInterface` → `renderIPs` | `TestShowTrafficUsageIPv6` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `buildCollectionSpec(maxEntries, false)` vs `buildCollectionSpec(maxEntries, true)` loaded into the Linux kernel | With `track-ip` false, collection contains only `tu_port_in` and `tu_port_out` (zero IPv4/IPv6 IP maps); with `track-ip` true, collection contains `tu_port_in`, `tu_port_out`, `tu_ip_in` (4B key), `tu_ip_out` (4B key), `tu_ip6_in` (16B key), and `tu_ip6_out` (16B key); both programs pass verification and return `TCX_NEXT` (`-1`) | None |
| AC-2 | IPv6 TCP/UDP packets run through `tu_ingress` and `tu_egress` via `BPF_PROG_TEST_RUN` with `track-ip` enabled | `tu_ingress` adds `skb->len` to `tu_ip6_in[srcIPv6]` and `tu_port_in[{dstPort, proto}]` without touching egress or IPv4 maps; `tu_egress` adds `skb->len` to `tu_ip6_out[dstIPv6]` and `tu_port_out[{srcPort, proto}]` without touching ingress or IPv4 maps; multiple packets accumulate | None |
| AC-3 | IPv6 ICMPv6 (`NextHeader=58`), IPv6 extension-header packet (e.g., Hop-by-Hop `0` or Fragment `44`), or truncated L4 header (`skb->len < 54+20` for TCP or `< 54+8` for UDP) run through `tu_ingress` | Counted under `{port: 0, proto: NextHeader}` in `tu_port_in` and under `srcIPv6` in `tu_ip6_in` (when `track-ip` is enabled); exact 54-byte header-only IPv6 frame counts under `{port: 0, proto: NextHeader}` | Extension-header inner TCP/UDP ports are intentionally not parsed per owner design decision `(2a)` |
| AC-4 | Truncated IPv6 frame (`14..53` bytes, shorter than Ethernet + 40-byte IPv6 header) traverses veth with TCX attached, or non-IPv4/non-IPv6 frame (ARP `0x0806`) runs through `tu_ingress` | Passes with `TCX_NEXT` (`-1`) without writing any entry to `tu_port_*`, `tu_ip_*`, or `tu_ip6_*` | None |
| AC-5 | Real IPv6 TCP/UDP/ICMPv6 frames are injected on a Linux veth pair with `tcxAttacher.Attach` | `tcxAttachment.Counts()` returns unified `ingressIP`/`egressIP` `map[netip.Addr]uint64` carrying exact IPv6 `netip.Addr` keys and byte counts, `ingressPort`/`egressPort` with exact ports/protocols, and `mapEntries` containing `ip6_ingress` and `ip6_egress` | None |
| AC-6 | `Monitor.publishLocked` and `handleShowTrafficUsage` run with both IPv4 and IPv6 entries in `counts` | Prometheus `/metrics` emits `ze_traffic_usage_ingress_bytes_total{interface=...,src_ip="2001:db8::1"}`, `ze_traffic_usage_egress_bytes_total{interface=...,dst_ip="2001:db8::2"}`, `ze_traffic_usage_ingress_port_bytes_total{...,protocol="icmpv6"}`, and `ze_traffic_usage_map_entries{...,map="ip6_ingress"}`; `observation.Global()` receives IPv6 `Flow.Src` (`netip.Addr`); `stale-timeout` deletes stale IPv6 series; `show traffic usage` returns canonical RFC 5952 IPv6 addresses in `ingress-ips` / `egress-ips` | None |
| AC-7 | Existing IPv4 tests (`TestProgram_IngressSrcIP`, `TestProgram_EgressDstIP`, `TestProgram_IPOptionsMoveThePort`, `TestAttachTCX_CountsTraffic`, `TestShowTrafficUsage`) run after the change | Every existing IPv4 behavior and metric/CLI output passes unchanged | None |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Enables `traffic { usage { enabled true; track-ip true; interfaces { interface eth0; } } }` on a dual-stack router and scrapes `/metrics` | `tu_ingress`/`tu_egress` TCX programs → `tu_port_*` + `tu_ip_*` + `tu_ip6_*` → `tcxAttachment.Counts` → `Monitor.publishLocked` → HTTP `GET /metrics` (`ze_traffic_usage_*`) | `TestAttachTCX_CountsIPv6Traffic` and `TestMetricsScrape` |
| 2 | Runs `ze show traffic usage name eth0 \| json` to find the top IPv6 talker during a WAN spike | `ze-trafficusage:show-traffic-usage` → `handleShowTrafficUsage` → `Monitor.Snapshot` → `renderInterface` / `renderIPs` (`netip.Addr.String()`) | `TestShowTrafficUsageIPv6` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMonitorPublishesIPv6MetricsAndObservations` | `internal/plugins/trafficusage/monitor_test.go` | AC-6: IPv4 + IPv6 `netip.Addr` entries publish to `ze_traffic_usage_*`, `ip6_ingress`/`ip6_egress` map entries, `observation.Global()` IPv6 `Flow.Src`, and stale-timeout cleanup | PASS |
| `TestShowTrafficUsageIPv6` | `internal/plugins/trafficusage/show_test.go` | AC-6: `show traffic usage` renders canonical RFC 5952 IPv6 addresses in `ingress-ips` / `egress-ips`, `icmpv6` in `ingress-ports`, and `ip6_ingress`/`ip6_egress` in `map-entries` | PASS |
| `TestProgram_Loads` & `TestProgram_TrackIPOffOmitsIPMaps` | `internal/plugins/trafficusage/program_test.go` | AC-1: `tu_ip6_in`/`tu_ip6_out` created only when `track-ip` is true; both programs load cleanly | PASS |
| `TestProgram_IPv6IngressSrcIP`, `TestProgram_IPv6EgressDstIP`, `TestProgram_IPv6IngressDstPortTCP`, `TestProgram_IPv6EgressSrcPortUDP`, `TestProgram_IPv6ICMPv6PortZero`, `TestProgram_IPv6ExtHeaderPortZero`, `TestProgram_IPv6TruncatedL4PortZero`, `TestProgram_IPv6HeaderEndingTheFrameCounts`, `TestProgram_IPv6TruncatedPass` | `internal/plugins/trafficusage/program_test.go` | AC-2, AC-3, AC-4: IPv6 `BPF_PROG_TEST_RUN` parsing, byte order, port extraction, ICMPv6/ext-header port 0, 54B exact boundary, and 53B veth truncated pass | PASS |
| `TestAttachTCX_CountsIPv6Traffic` & `TestMetricsScrape` | `internal/plugins/trafficusage/attach_integration_linux_test.go` | AC-5, AC-6: Real veth TCX attach counting live IPv6 frames through `tcxAttachment.Counts()` and HTTP `/metrics` scrape | PASS |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| IPv6 frame length for L3 parse (`ethHdrLen + ipv6HdrLen`) | `54 .. skb->len` | `54` (`TestProgram_IPv6HeaderEndingTheFrameCounts`, counted with `port=0`) | `53` (`TestProgram_IPv6TruncatedPass`, ignored with `TCX_NEXT`) | N/A |
| IPv6 TCP header fit (`54 + 20`) | `74 .. skb->len` | `74` (port extracted) | `73` (counted with `port=0`) | N/A |
| IPv6 UDP header fit (`54 + 8`) | `62 .. skb->len` | `62` (port extracted) | `61` (counted with `port=0`) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing `traffic-usage` `.ci` suite + Linux integration tests (`BPF_PROG_TEST_RUN` & veth TCX) | `test/plugin/traffic-usage-config.ci`, `test/parse/traffic-usage.ci`, `internal/plugins/trafficusage/program_test.go`, `internal/plugins/trafficusage/attach_integration_linux_test.go` | Verifies config parsing, daemon plugin lifecycle, real Linux eBPF verification (`BPF_PROG_TEST_RUN`), veth TCX IPv6 packet accounting, and `/metrics` scrape (note: TCX eBPF attach requires `CAP_BPF` + `CAP_NET_ADMIN`, tested in `attach_integration_linux_test.go` and `program_test.go` per `docs/architecture/traffic/traffic-usage.md`) | PASS |

## Files to Modify
- `internal/plugins/trafficusage/program_linux.go` - add `mapIP6Ingress`, `mapIP6Egress`, `ethTypeIPv6`, IPv6 header offsets, `slotIP6Key`/`slotIsIPv6`, parameterized `newLRUMap`, and `parse_v6` + `ip6_ops` in `directionInsns`
- `internal/plugins/trafficusage/attach_linux.go` - read `tu_ip_*` and `tu_ip6_*` into unified `map[netip.Addr]uint64` and record `ip6_ingress`/`ip6_egress` in `mapEntries`
- `internal/plugins/trafficusage/monitor.go` - unify `counts.ingressIP` / `counts.egressIP` to `map[netip.Addr]uint64`, simplify `publishLocked` and `observation.Global()` source publication
- `internal/plugins/trafficusage/show.go` - update `renderIPs` to accept `map[netip.Addr]uint64`
- `internal/plugins/trafficusage/metrics.go` - update `ingressBytes`/`egressBytes` help strings to IP (IPv4/IPv6) and delete obsolete `ipString`
- `internal/plugins/trafficusage/trafficusage.go` - update package doc comment from IPv4-only to IPv4 and IPv6
- `internal/plugins/trafficusage/yang/ze-traffic-usage-conf.yang` - update `description` and `ze:help` text for IPv4 + IPv6 and four IP maps when `track-ip` is enabled
- `internal/plugins/trafficusage/monitor_test.go` - update `counts` fixtures to `netip.Addr` and add `TestMonitorPublishesIPv6MetricsAndObservations`
- `internal/plugins/trafficusage/show_test.go` - update `counts` fixture to `netip.Addr` and add `TestShowTrafficUsageIPv6`
- `internal/plugins/trafficusage/program_test.go` - add IPv6 `BPF_PROG_TEST_RUN` and truncated veth TCX tests
- `internal/plugins/trafficusage/attach_integration_linux_test.go` - add `TestAttachTCX_CountsIPv6Traffic` and assert IPv6 series in `TestMetricsScrape`
- `docs/architecture/traffic/traffic-usage.md` - document IPv6 assembly path, 16-byte `tu_ip6_*` maps, fixed 40-byte base header, and 54-byte `BPF_PROG_TEST_RUN` boundary
- `docs/guide/traffic-usage.md` - remove IPv4-only limitation, document IPv4 + IPv6 per-port/per-IP accounting and extension-header behavior
- `docs/plugin-development/metrics.md` - update `ze_traffic_usage_*` descriptions for IPv4/IPv6 and `ip6_ingress`/`ip6_egress`
- `features/traffic-usage.md` - update feature description from IPv4-only to IPv4 and IPv6

## Files to Create
- None (all changes extend existing `internal/plugins/trafficusage` files and docs).

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No new YANG leaves or RPCs; only description/`ze:help` text updated in `internal/plugins/trafficusage/yang/ze-traffic-usage-conf.yang` |
| YANG validation constraints | N-A | No new config leaves or range changes |
| YANG custom validators | N-A | No new validators |
| CLI commands/flags | Yes | Existing `show traffic usage [name <interface>]` now includes canonical IPv6 entries in `ingress-ips` / `egress-ips` and `ip6_ingress`/`ip6_egress` in `map-entries` |
| CLI grammar (keyword before value) | N-A | Command grammar unchanged |
| Editor autocomplete | N-A | Command path and config leaves unchanged |
| Functional test for new RPC/API | Yes | Existing `test/plugin/traffic-usage-config.ci` + real kernel `BPF_PROG_TEST_RUN` and veth TCX integration tests in `program_test.go` / `attach_integration_linux_test.go` |
| Pipe completeness | Yes | Existing `show traffic usage` structured `plugin.Map` output routes through `ApplyPipes` unchanged |
| Env var registration | N-A | No new environment variables |
| Doctor check for runtime dependencies | N-A | Reuses existing `traffic-usage-attach` runtime doctor check and its 4 doctor codes |
| Doctor code per problem class | N-A | No new failure classes introduced |
| Prometheus counters/metrics | Yes | Existing 5 `ze_traffic_usage_*` `GaugeVec` families now carry IPv6 `src_ip`/`dst_ip`, `icmpv6` protocol, and `ip6_ingress`/`ip6_egress` map entries |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not a BGP address family |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/traffic-usage.md` (update description from IPv4-only to IPv4 and IPv6) |
| 2 | Config syntax changed? | No | Zero config syntax changes; YANG descriptions updated in `internal/plugins/trafficusage/yang/ze-traffic-usage-conf.yang` |
| 3 | CLI command added/changed? | No | `show traffic usage` command syntax and keys unchanged; `docs/guide/command-reference.md` has no IPv4-only claim |
| 4 | API/RPC added/changed? | No | `ze-trafficusage:show-traffic-usage` JSON keys unchanged; `docs/architecture/api/commands.md` has no IPv4-only claim |
| 5 | Plugin added/changed? | Yes | `docs/architecture/traffic/traffic-usage.md` (document IPv6 pure-Go eBPF parsing, `tu_ip6_*` maps, 54-byte boundary, and fixed 40B base header) |
| 6 | Has a user guide page? | Yes | `docs/guide/traffic-usage.md` (remove IPv4-only limitation, document IPv4 + IPv6 and extension-header port-0 rule) |
| 7 | Wire format changed? | No | No protocol wire format changes |
| 8 | Plugin SDK/protocol changed? | No | `pkg/plugin/sdk` and `pkg/plugin/rpc` untouched |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | Linux eBPF accounting, not a routing protocol RFC |
| 10 | Test infrastructure changed? | No | Uses existing `BPF_PROG_TEST_RUN` and veth TCX test harness |
| 11 | Affects daemon comparison? | No | No comparison matrix row changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/traffic/traffic-usage.md` updated for IPv6 assembly and `tu_ip6_*` maps |
| 13 | Route metadata keys added/changed? | N-A | Not route metadata |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` (update `ze_traffic_usage_*` inventory notes for IPv4/IPv6) |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No new plugin, event type, or command registered |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/architecture/traffic/traffic-usage.md`, `docs/guide/traffic-usage.md`, `docs/plugin-development/metrics.md`, `features/traffic-usage.md` (all updated); `docs/architecture/api/commands.md` and `docs/functional-tests.md` checked and unaffected |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/traffic-usage.md` updated to reflect IPv4 + IPv6 accounting |

## Design presentation

### What changes

`traffic-usage` now accounts IPv6 (`ETH_P_IPV6`) packets alongside IPv4 in its pure-Go TCX eBPF programs (`tu_ingress` and `tu_egress`). Per-port/protocol counters (`tu_port_in`, `tu_port_out`) aggregate both IPv4 and IPv6 traffic through one shared L4 parser (mapping TCP, UDP, ICMPv6, and extension-header packets cleanly), while opt-in `track-ip` adds two 16-byte LRU maps (`tu_ip6_in`, `tu_ip6_out`) alongside the existing 4-byte IPv4 maps (`tu_ip_in`, `tu_ip_out`). Userspace unifies `counts.ingressIP` and `counts.egressIP` onto `map[netip.Addr]uint64` so Prometheus `/metrics`, `show traffic usage`, stale-series eviction, and `observation.Global()` handle IPv4 and IPv6 through one code path with zero duplication.

### Before and after

1. **Dual-stack interface with `traffic { usage { enabled true; track-ip true; interfaces { interface wan; } } }`:**
   - Before: IPv6 packets hit `JNE ethTypeIPv4 -> ret0` and are silently ignored; only IPv4 bytes appear in `show traffic usage` and `/metrics`.
   - After: IPv6 TCP/UDP/ICMPv6 packets are accounted in `ze_traffic_usage_{ingress,egress}_port_bytes_total`, `ze_traffic_usage_{ingress,egress}_bytes_total` (`src_ip`/`dst_ip` in canonical RFC 5952 IPv6 format), `ze_traffic_usage_map_entries` (`ip6_ingress`/`ip6_egress`), `show traffic usage`, and `observation.Global()`.
2. **Dual-stack interface with `track-ip false` (default):**
   - Before: only IPv4 port/protocol traffic is counted; two 4-byte port maps exist.
   - After: both IPv4 and IPv6 port/protocol traffic are counted in the same two 4-byte port maps (`tu_port_in`, `tu_port_out`); zero IPv4 or IPv6 IP maps are created and zero per-IP instructions are emitted.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Widen `tu_ip_in` / `tu_ip_out` to unified 16-byte IPv4-mapped keys | Quadruples IPv4 key memory on IPv4-only deployments and conflates IPv4/IPv6 LRU eviction capacity; separate `tu_ip6_in` / `tu_ip6_out` preserves 4-byte IPv4 map efficiency and isolates IPv4 vs IPv6 LRU fill visibility in `ze_traffic_usage_map_entries`. |
| Walk a bounded loop of IPv6 extension headers in eBPF to find inner TCP/UDP ports | Adds branching and extra `bpf_skb_load_bytes` calls on every non-TCP/UDP IPv6 packet for marginal WAN benefit; fixed 40-byte IPv6 base header keeps the program compact, fast, and trivially verifiable under `!CAP_PERFMON` while still counting extension-header packets per-IP and under `port=0` with their first `NextHeader` protocol number. |

### Documents departed from

None — this spec extends `traffic-usage` cleanly within every architecture and design contract.

### Choices for Thomas

| # | Choice | Options | Decision |
|---|--------|---------|----------|
| C-1 | Per-IP BPF map layout when `track-ip` is enabled | (a) Separate 16-byte LRU maps (`tu_ip6_in` / `tu_ip6_out`) alongside 4-byte `tu_ip_in` / `tu_ip_out` (recommended); (b) Unified 16-byte IPv4-mapped keys | (a) Confirmed 2026-11-27 |
| C-2 | IPv6 extension-header handling under `!CAP_PERFMON` | (a) Fixed 40-byte IPv6 base header (`port=0` for extension headers, still counted per-IP and per-protocol) (recommended); (b) Bounded multi-hop extension-header walk | (a) Confirmed 2026-11-27 |

## Implementation Steps

1. **Phase: Pure-Go IPv6 eBPF Assembly & Map Readback** -- extend `program_linux.go` with `parse_v6`, `tu_ip6_in`/`tu_ip6_out`, and shared `l4_parse`/`ip6_ops`, update `attach_linux.go` to read IPv4 + IPv6 maps into `map[netip.Addr]uint64`, and prove with `BPF_PROG_TEST_RUN` and veth TCX tests
   - Tests: `TestProgram_Loads`, `TestProgram_TrackIPOffOmitsIPMaps`, `TestProgram_IPv6*`, `TestAttachTCX_CountsIPv6Traffic`
   - Files: `internal/plugins/trafficusage/program_linux.go`, `internal/plugins/trafficusage/attach_linux.go`, `internal/plugins/trafficusage/program_test.go`, `internal/plugins/trafficusage/attach_integration_linux_test.go`
   - Verify: all unit tests and Linux `integration` eBPF tests pass (and `GOOS=darwin go vet` passes)
2. **Phase: Unified `netip.Addr` Monitor, CLI, Metrics, YANG Help & Docs** -- unify `counts` on `netip.Addr` across `monitor.go`, `show.go`, `metrics.go`, delete `ipString`, update YANG descriptions and all 4 doc files, and add IPv6 unit tests
   - Tests: `TestMonitorPublishesIPv6MetricsAndObservations`, `TestShowTrafficUsageIPv6`, `TestMetricsScrape`, `./le doc check all`
   - Files: `internal/plugins/trafficusage/monitor.go`, `internal/plugins/trafficusage/show.go`, `internal/plugins/trafficusage/metrics.go`, `internal/plugins/trafficusage/trafficusage.go`, `internal/plugins/trafficusage/yang/ze-traffic-usage-conf.yang`, `internal/plugins/trafficusage/monitor_test.go`, `internal/plugins/trafficusage/show_test.go`, `docs/architecture/traffic/traffic-usage.md`, `docs/guide/traffic-usage.md`, `docs/plugin-development/metrics.md`, `features/traffic-usage.md`
   - Verify: scoped unit tests, Linux `integration` tests, lint across host/linux/darwin, `./le doc check all`, and `./le arch tier check` pass

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-7 has an implementation at `file:line` and a passing test |
| Verifier safety (`!CAP_PERFMON`) | Zero register pointer arithmetic; every packet-pointer read uses fixed immediate offset before any helper call; `slotIP6Key` written before any `tu_ip6_*` lookup |
| Byte order & offsets | `ethTypeIPv6 = 0xDD86`; `ipv6HdrLen = 40`; `ipv6NextHdrOff = 6`; `ipv6SrcOff = 8`; `ipv6DstOff = 24`; exact `[16]byte` asymmetric key test proves zero byte-order bug |
| Verdict contract | Every exit path returns `TCX_NEXT` (`-1`), never `TCX_PASS` (`0`) |
| `track-ip: false` invariant | Neither `tu_ip_*` nor `tu_ip6_*` maps or instructions exist when `track-ip` is false |
| Zero duplication | Single `l4_parse` block, single `accountInsns`, parameterized `newLRUMap`, unified `map[netip.Addr]uint64` in `monitor.go` and `show.go` |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Pure-Go IPv6 eBPF assembly and map readback | `./le test unit pkg path ./internal/plugins/trafficusage/` and `go test -tags integration` on Linux (where `CAP_BPF` is available) |
| Unified `netip.Addr` monitor, metrics, observation feed, and `show traffic usage` | `./le test unit pkg path ./internal/plugins/trafficusage/` |
| Cross-platform build & lint cleanliness | `./le go lint run scope ./internal/plugins/trafficusage/` and `GOOS=darwin go vet ./internal/plugins/trafficusage/...` |
| Documentation & YANG help updates | `./le doc check all` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Packet bounds validation | `data + 54 > data_end -> ret0` checked before every IPv6 header read; TCP (20B) and UDP (8B) `skb->len` bounds checked before `bpf_skb_load_bytes` |
| Bounded map & metric cardinality | `tu_ip6_in` / `tu_ip6_out` use `ebpf.LRUHash` capped at `maxEntries`, omitted when `track-ip` is false; `stale-timeout` evicts inactive IPv6 metric series |
| Read-only monitoring guarantee | Programs never modify packet bytes and always return `TCX_NEXT` (`-1`) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| eBPF verifier rejection | Fix instruction sequence / stack initialization in `program_linux.go` |
| Byte-order or offset mismatch | Fix constant in `program_linux.go` and verify with asymmetric IPv6 `BPF_PROG_TEST_RUN` |
| Lint or style finding | Fix inline; if architectural → DESIGN |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Separate 16-byte `tu_ip6_in` / `tu_ip6_out` LRU maps alongside 4-byte `tu_ip_in` / `tu_ip_out` | Unified 16-byte IPv4-mapped keys | Preserves 4-byte IPv4 map memory footprint, isolates IPv4/IPv6 LRU eviction, and keeps `ze_traffic_usage_map_entries` transparent |
| Fixed 40-byte IPv6 base header (`port=0` for extension headers) | Multi-hop extension-header walk in eBPF | Keeps assembly compact, fast, and trivially verifiable under `!CAP_PERFMON` while still counting extension-header packets per-IP and per-protocol |
| Unify `counts.ingressIP` / `counts.egressIP` onto `map[netip.Addr]uint64` | Separate IPv4 and IPv6 maps/loops in `monitor.go` and `show.go` | Eliminates duplicated metric, stale-cleanup, observation, and CLI loops and deletes obsolete `ipString(uint32)` |

## Known Limitations
- IPv6 packets carrying extension headers between the base 40-byte IPv6 header and TCP/UDP are counted per-IP and under `{port: 0, protocol: <first-next-header>}` rather than walking extension headers.

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
- Extended pure-Go eBPF TCX assembly in `internal/plugins/trafficusage/program_linux.go` (`buildCollectionSpec`, `newLRUMap`, `directionInsns`) with `ethTypeIPv6` (`0xDD86`), fixed 40-byte IPv6 base-header bounds (`data + 54 > data_end -> ret0`), 16-byte stack staging via two fixed-offset `DWord` loads into 8-byte-aligned `slotIP6Key` (with cross-family stack zeroing for `!CAP_PERFMON` speculative verification), optional 16-byte LRU maps `tu_ip6_in` and `tu_ip6_out` (omitted when `track-ip` is false), shared `l4_parse` (`bpf_skb_load_bytes`), and `TCX_NEXT` (`-1`) on every exit path.
- Extended `tcxAttachment.Counts` in `internal/plugins/trafficusage/attach_linux.go` (`readIPMap`, `readIP6Map`) to read 4-byte IPv4 maps (`netip.AddrFrom4`) and 16-byte IPv6 maps (`netip.AddrFrom16`) into unified `map[netip.Addr]uint64` maps and populate `mapEntries["ip6_ingress"]` / `mapEntries["ip6_egress"]`.
- Unified `counts.ingressIP` / `counts.egressIP` onto `map[netip.Addr]uint64` in `internal/plugins/trafficusage/monitor.go` (`Monitor.publishLocked`) and `internal/plugins/trafficusage/show.go` (`renderIPs`), updated `ze_traffic_usage_{ingress,egress}_bytes_total` help strings in `internal/plugins/trafficusage/metrics.go` (`BindMetrics`), deleted obsolete `ipString(uint32)` and `TestIPString` (`metrics_test.go`, independently approved by `ze-test-judge` `f6f98626-105c-46f9-8ae7-296677a5bfa4`), and updated `trafficusage.go` and `yang/ze-traffic-usage-conf.yang`.
- Added IPv6 unit and real-kernel `BPF_PROG_TEST_RUN` / veth TCX tests across `program_test.go` (9 new tests, 23 total), `attach_integration_linux_test.go` (`TestAttachTCX_CountsIPv6Traffic` + IPv6 assertions in `TestMetricsScrape`, 3 total; plus `disableIPv6` helper so veth DAD/RS/MLD noise never pollutes raw `AF_PACKET` counts), `monitor_test.go` (`TestMonitorPublishesIPv6MetricsAndObservations`), and `show_test.go` (`TestShowTrafficUsageIPv6`).

### Bugs Found/Fixed
- Cross-family stack initialization under `!CAP_PERFMON`: each L3 branch zeroes the opposite family's stack slot (`slotIP6Key` on IPv4 via `Mov.Imm` + two `StoreMem DWord`, avoiding the `StoreImm DWord` invalid-opcode trap; `slotIPKey` on IPv6 via `StoreImm Word`) so speculative verifier exploration of `ip_ops` / `ip6_ops` never sees uninitialized stack bytes. Proven by `TestTrafficUsageLoadsUnderTheUnitCapabilities` under `uid 65534` with `privilege.DaemonCapabilities`.
- Unsolicited kernel IPv6 DAD/RS/MLD frames on freshly brought-up test veth pairs: added `disableIPv6` in `attach_integration_linux_test.go` before `LinkSetUp` so `AF_PACKET` integration tests observe only injected frames. Proven by `TestAttachTCX_CountsIPv6Traffic`, `TestMetricsScrape`, and `TestProgram_IPv6TruncatedPass`.

### Documentation Updates
- `docs/architecture/traffic/traffic-usage.md`: updated for IPv4 + IPv6 pure-Go assembly, separate `tu_ip6_*` maps, cross-family stack zeroing, fixed 40-byte base header, and 54-byte `BPF_PROG_TEST_RUN` boundary (`<!-- source: internal/plugins/trafficusage/program_linux.go -- directionInsns, newLRUMap -->`, `<!-- source: internal/plugins/trafficusage/attach_linux.go -- readIPMap, readIP6Map -->`, `<!-- source: internal/plugins/trafficusage/program_test.go -- TestProgram_IPv6IngressSrcIP, ... -->`).
- `docs/guide/traffic-usage.md`: removed IPv4-only limitation; documented IPv4 + IPv6 accounting, `ip6_ingress`/`ip6_egress` map entries, `icmpv6`, and extension-header port-0 rule (`<!-- source: internal/plugins/trafficusage/metrics.go -- BindMetrics -->`, `<!-- source: internal/plugins/trafficusage/yang/ze-traffic-usage-conf.yang -- traffic-usage -->`).
- `docs/plugin-development/metrics.md`: updated the five `ze_traffic_usage_*` inventory rows for IPv4/IPv6 and `ip6_*` maps.
- `features/traffic-usage.md`: updated feature description from IPv4-only to IPv4 and IPv6.
- Additional stale IPv4-only claims fixed after Run 1 review: `docs/guide/configuration.md:3983`, `docs/guide/plugins.md:739`, `docs/architecture/traffic/traffic-usage-monitor.md:82`, `docs/architecture/ddos/cp-survival-5-detect-5-characterization.md:26`, `docs/guide/ddos-mitigation.md:120`, and `docs/functional-tests.md:5002-5003` (updated test counts to `23 tests` and `3 tests`).
- `website/changes/fragments/traffic-usage-ipv6.md`: operator-facing news fragment added (`topic: Telemetry`, `feature: traffic-usage`).
- `./le doc ste check` on all 6 shipped Go files, `ze-traffic-usage-conf.yang`, all 10 doc/feature files, and the news fragment: `OK -- no habit grew`.
- `./le doc check all`: 0 failures in any `traffic-usage` doc, source anchor, or YANG node (unrelated concurrent/pre-existing failures only in `ze-policy-conf` char-cap and foreign `plugin format` wiki catalog).

### Deviations from Plan
- Added `TestProgram_IPv6TruncatedL4PortZero` in `program_test.go` beyond the listed test names to explicitly prove the 74B/73B IPv6 TCP and 62B/61B IPv6 UDP boundary table rows.
- Modified `internal/plugins/trafficusage/metrics_test.go` to remove `TestIPString` after deleting obsolete `ipString(uint32)` (independently approved by `ze-test-judge` `f6f98626-105c-46f9-8ae7-296677a5bfa4`).
- Updated 6 additional doc files (`docs/guide/configuration.md`, `docs/guide/plugins.md`, `docs/architecture/traffic/traffic-usage-monitor.md`, `docs/architecture/ddos/cp-survival-5-detect-5-characterization.md`, `docs/guide/ddos-mitigation.md`, `docs/functional-tests.md`) found by Run 1 review to carry stale IPv4-only statements or old test counts.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Initial doc updates covered only the 4 primary `traffic-usage` docs and left 6 secondary docs claiming `traffic-usage` was IPv4-only plus STE run-on regressions in `ze-traffic-usage-conf.yang`, `docs/architecture/traffic/traffic-usage.md`, and `docs/plugin-development/metrics.md`. | Secondary guides/architecture pages (`configuration.md`, `plugins.md`, `traffic-usage-monitor.md`, `ddos-mitigation.md`, `cp-survival-5-detect-5-characterization.md`, `functional-tests.md`) also stated IPv4-only or old test counts, and YANG `description` sentences are checked by `./le doc ste check`. | Run 1 independent closure review (`4c196aac-991d-4037-a801-fd2feb999fbd`). | Updated all 6 secondary docs, rewrote YANG/doc sentences to pass `./le doc ste check`, and verified cleanly in Run 2. |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| IPv6 `ETH_P_IPV6` fixed 40B base-header parse + shared L4 `bpf_skb_load_bytes` + `!CAP_PERFMON` verification | Done | `internal/plugins/trafficusage/program_linux.go:183-338` (`directionInsns`) | Passes `TestTrafficUsageLoadsUnderTheUnitCapabilities` under `uid 65534` |
| Separate 16B LRU maps `tu_ip6_in`/`tu_ip6_out` only when `track-ip` is enabled | Done | `internal/plugins/trafficusage/program_linux.go:113-153` (`newLRUMap`, `buildCollectionSpec`) | Omitted when `track-ip` is false (`TestProgram_TrackIPOffOmitsIPMaps`) |
| Read IPv4 & IPv6 BPF maps into unified `map[netip.Addr]uint64` + `ip6_ingress`/`ip6_egress` map counts | Done | `internal/plugins/trafficusage/attach_linux.go:96-210` (`tcxAttachment.Counts`, `readIPMap`, `readIP6Map`) | Proven on real veth TCX (`TestAttachTCX_CountsIPv6Traffic`) |
| Unified `netip.Addr` publication to Prometheus `/metrics`, `show traffic usage`, stale cleanup, and `observation.Global()` | Done | `internal/plugins/trafficusage/monitor.go:28-35,403-445` (`counts`, `Monitor.publishLocked`); `show.go:102-111` (`renderIPs`); `metrics.go:29-41` (`BindMetrics`) | Obsolete `ipString(uint32)` removed |
| Update YANG descriptions and all `traffic-usage` docs | Done | `yang/ze-traffic-usage-conf.yang`, `trafficusage.go`, 10 doc/feature files, `website/changes/fragments/traffic-usage-ipv6.md` | STE and doc-anchor clean |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestProgram_Loads`, `TestProgram_TrackIPOffOmitsIPMaps`, `TestProgram_ReturnsTCXNext`, `TestTrafficUsageLoadsUnderTheUnitCapabilities` | Real kernel PASS including `!CAP_PERFMON` |
| AC-2 | Done | `TestProgram_IPv6IngressSrcIP`, `TestProgram_IPv6EgressDstIP`, `TestProgram_IPv6IngressDstPortTCP`, `TestProgram_IPv6EgressSrcPortUDP` | Asymmetric 16B keys + accumulation + direction/family isolation |
| AC-3 | Done | `TestProgram_IPv6ICMPv6PortZero`, `TestProgram_IPv6ExtHeaderPortZero`, `TestProgram_IPv6TruncatedL4PortZero`, `TestProgram_IPv6HeaderEndingTheFrameCounts` | ICMPv6 (58), Hop-by-Hop (0), Fragment (44), 73B/74B TCP, 61B/62B UDP, 54B header-only |
| AC-4 | Done | `TestProgram_IPv6TruncatedPass`, `TestProgram_NonIPv4Pass` | 14..53B IPv6 frames over veth TCX + ARP `0x0806` |
| AC-5 | Done | `TestAttachTCX_CountsIPv6Traffic` | Live veth TCX TCP/UDP/ICMPv6 readback + `ip6_ingress`/`ip6_egress` |
| AC-6 | Done | `TestMonitorPublishesIPv6MetricsAndObservations`, `TestShowTrafficUsageIPv6`, `TestMetricsScrape` | `/metrics`, `observation.Global()`, stale-timeout deletion, `show traffic usage` |
| AC-7 | Done | `TestProgram_IngressSrcIP`, `TestProgram_EgressDstIP`, `TestProgram_IPOptionsMoveThePort`, `TestAttachTCX_CountsTraffic`, `TestShowTrafficUsage` | All existing IPv4 tests pass unchanged |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestMonitorPublishesIPv6MetricsAndObservations` | Done | `internal/plugins/trafficusage/monitor_test.go:428` | Unit PASS |
| `TestShowTrafficUsageIPv6` | Done | `internal/plugins/trafficusage/show_test.go:78` | Unit PASS |
| `TestProgram_Loads` & `TestProgram_TrackIPOffOmitsIPMaps` | Done | `internal/plugins/trafficusage/program_test.go:152,180` | Real kernel integration PASS |
| `TestProgram_IPv6IngressSrcIP`, `TestProgram_IPv6EgressDstIP`, `TestProgram_IPv6IngressDstPortTCP`, `TestProgram_IPv6EgressSrcPortUDP`, `TestProgram_IPv6ICMPv6PortZero`, `TestProgram_IPv6ExtHeaderPortZero`, `TestProgram_IPv6TruncatedL4PortZero`, `TestProgram_IPv6HeaderEndingTheFrameCounts`, `TestProgram_IPv6TruncatedPass` | Done | `internal/plugins/trafficusage/program_test.go:688-970` | Real kernel integration PASS |
| `TestAttachTCX_CountsIPv6Traffic` & `TestMetricsScrape` | Done | `internal/plugins/trafficusage/attach_integration_linux_test.go:152,200` | Real kernel veth TCX PASS |
| `test/parse/traffic-usage.ci` & `test/plugin/traffic-usage-config.ci` | Done | `test/parse/traffic-usage.ci`, `test/plugin/traffic-usage-config.ci` | Functional `.ci` PASS (`2/2` parse, `1/1` plugin) |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugins/trafficusage/program_linux.go` | Done | IPv6 eBPF parse, `tu_ip6_*` maps, `newLRUMap` |
| `internal/plugins/trafficusage/attach_linux.go` | Done | `readIPMap` / `readIP6Map` into `map[netip.Addr]uint64` + `ip6_*` counts |
| `internal/plugins/trafficusage/monitor.go` | Done | Unified `map[netip.Addr]uint64` in `counts` and `publishLocked` |
| `internal/plugins/trafficusage/show.go` | Done | Unified `renderIPs(map[netip.Addr]uint64)` |
| `internal/plugins/trafficusage/metrics.go` | Done | IPv4/IPv6 help strings; deleted `ipString` |
| `internal/plugins/trafficusage/trafficusage.go` | Done | Package doc updated to IPv4 and IPv6 |
| `internal/plugins/trafficusage/yang/ze-traffic-usage-conf.yang` | Done | Descriptions & `ze:help` updated; STE-clean |
| `internal/plugins/trafficusage/monitor_test.go` | Done | `netip.Addr` fixtures + `TestMonitorPublishesIPv6MetricsAndObservations` |
| `internal/plugins/trafficusage/show_test.go` | Done | `netip.Addr` fixture + `TestShowTrafficUsageIPv6` |
| `internal/plugins/trafficusage/metrics_test.go` | Done | Removed obsolete `TestIPString` (`ze-test-judge` approved) |
| `internal/plugins/trafficusage/program_test.go` | Done | 9 new IPv6 `BPF_PROG_TEST_RUN` and veth TCX tests (23 total) |
| `internal/plugins/trafficusage/attach_integration_linux_test.go` | Done | `TestAttachTCX_CountsIPv6Traffic` + IPv6 `/metrics` scrape (3 total) |
| `docs/architecture/traffic/traffic-usage.md`, `docs/guide/traffic-usage.md`, `docs/plugin-development/metrics.md`, `features/traffic-usage.md` | Done | Primary docs updated |
| `docs/guide/configuration.md`, `docs/guide/plugins.md`, `docs/architecture/traffic/traffic-usage-monitor.md`, `docs/architecture/ddos/cp-survival-5-detect-5-characterization.md`, `docs/guide/ddos-mitigation.md`, `docs/functional-tests.md` | Done | Secondary stale IPv4-only claims and test counts updated after Run 1 |
| `website/changes/fragments/traffic-usage-ipv6.md` | Done | Operator news fragment created |

### Audit Summary
- **Total items:** 29 (5 task requirements, 7 ACs, 6 TDD test groups, 11 file groups)
- **Done:** 29
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 3 (added `TestProgram_IPv6TruncatedL4PortZero`, removed obsolete `TestIPString` with `ze-test-judge` approval, updated 6 additional stale docs)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Account IPv6 (`ETH_P_IPV6`) per-(port, protocol) and opt-in per-IP traffic in pure-Go TCX eBPF under `!CAP_PERFMON` | Kernel `BPF_PROG_TEST_RUN` + veth TCX + unit-capability verifier test | Real Linux kernel `go test -v -tags integration -count=1 ./internal/plugins/trafficusage/`: all 23 `program_test.go` tests (including `TestProgram_IPv6*`, `TestProgram_IPv6TruncatedPass`, `TestProgram_TrackIPOffOmitsIPMaps`), `TestAttachTCX_CountsIPv6Traffic`, `TestProgram_ReturnsTCXNext`, `TestProgram_LetsAClsactFilterRun`, and `TestTrafficUsageLoadsUnderTheUnitCapabilities` (`uid 65534`, `0.10s`) PASS |
| Surface IPv6 alongside IPv4 in Prometheus `/metrics`, `show traffic usage`, and `observation.Global()` through one unified `netip.Addr` path | Unit tests + real-kernel `/metrics` scrape + functional `.ci` | `./le test unit pkg path ./internal/plugins/trafficusage/` (`TestMonitorPublishesIPv6MetricsAndObservations`, `TestShowTrafficUsageIPv6`, PASS `1.069s`); `TestMetricsScrape` (live IPv4+IPv6 veth `/metrics` scrape PASS); `./le test bgp parse --pattern traffic-usage` (`2/2 PASS`) & `./le test bgp plugin --pattern traffic-usage-config` (`1/1 PASS`) |
| Preserve existing IPv4 accounting, cross-platform builds, and accurate documentation | Unit + integration + lint + darwin vet + doc checks | `TestProgram_IngressSrcIP`, `TestProgram_EgressDstIP`, `TestProgram_IPOptionsMoveThePort`, `TestAttachTCX_CountsTraffic`, `TestShowTrafficUsage` PASS; `./le go lint run scope ./internal/plugins/trafficusage/` (`0 issues`); `GOOS=darwin go vet ./internal/plugins/trafficusage/...` (`exit 0`); `./le doc ste check` (`OK`) |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None — all in-scope items completed | N/A | N/A |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/traffic-usage-ipv6-0cfcb484-f28a-4e4e-b1d1-85693900af06.md` |
| `./le spec review check` | `review_gate: OK (12 code files, clean, hashes match tmp/review/traffic-usage-ipv6-0cfcb484-f28a-4e4e-b1d1-85693900af06.md)` |
| Rounds | `2` (Run 1: all 11 Go files clean, 3 doc/YANG STE & stale-doc ISSUEs; Run 2: CLEAN — 0 BLOCKER, 0 ISSUE, 1 NOTE on test-only comment STE hits in `attach_integration_linux_test.go:34` and `program_test.go:4,915,939`) |
| Reviewer lenses used | Run 1 (`4c196aac-991d-4037-a801-fd2feb999fbd`): full diff + docs + STE; Run 2 (`934a2000-cd44-468a-92c8-ded70879300e`): logic+wiring, security+eBPF-verifier+edge-cases, design-documents, 7-question `ze-go-style.md` pass, `/ze-review-spec` completeness |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | `leaf max-entries` description had a 27-word sentence exceeding STE Rule 6.3 (25 words) | `internal/plugins/trafficusage/yang/ze-traffic-usage-conf.yang:134` | Split into two short sentences; verified by `./le doc ste check` (`OK`) and `./le test bgp parse --pattern traffic-usage` (`2/2 PASS`) |
| 2 | ISSUE | STE ratchets grew in updated architecture and metrics docs | `docs/architecture/traffic/traffic-usage.md`, `docs/plugin-development/metrics.md:246-248` | Rewrote sentences cleanly; verified by `./le doc ste check` (`OK`) |
| 3 | ISSUE | Six secondary docs retained stale IPv4-only claims or old integration test counts | `docs/guide/configuration.md:3983`, `docs/guide/plugins.md:739`, `docs/architecture/traffic/traffic-usage-monitor.md:82`, `docs/architecture/ddos/cp-survival-5-detect-5-characterization.md:26`, `docs/guide/ddos-mitigation.md:120`, `docs/functional-tests.md:5002-5003` | Updated all six pages to IPv4 and IPv6 and exact test counts (`23 tests`, `3 tests`); verified by grep across `docs/` and `features/` |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| All 12 code/YANG files (`internal/plugins/trafficusage/{program_linux,attach_linux,monitor,show,metrics,trafficusage,monitor_test,show_test,metrics_test,program_test,attach_integration_linux_test}.go`, `yang/ze-traffic-usage-conf.yang`), 10 doc/feature files, `website/changes/fragments/traffic-usage-ipv6.md`, `test/parse/traffic-usage.ci`, `test/plugin/traffic-usage-config.ci` | Yes | Verified via `git diff --stat`, `git status --porcelain`, and `./le spec review check` (`12 code files, clean, hashes match`); `hunks` patches verified with `git apply --check` against `HEAD` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | `track-ip` off omits all 4 IP maps; `track-ip` on creates 4B IPv4 + 16B IPv6 maps; `!CAP_PERFMON` verification and `TCX_NEXT` hold | `program_linux.go:buildCollectionSpec,directionInsns`; real kernel `TestProgram_Loads`, `TestProgram_TrackIPOffOmitsIPMaps`, `TestProgram_ReturnsTCXNext`, `TestTrafficUsageLoadsUnderTheUnitCapabilities` (`0.10s`) PASS |
| AC-2 | IPv6 TCP/UDP ingress/egress accounting, byte accumulation, direction/family isolation | `program_linux.go:directionInsns`; real kernel `TestProgram_IPv6IngressSrcIP`, `TestProgram_IPv6EgressDstIP`, `TestProgram_IPv6IngressDstPortTCP`, `TestProgram_IPv6EgressSrcPortUDP` PASS |
| AC-3 | ICMPv6 (58), extension headers (0, 44), truncated L4 (73B TCP, 61B UDP), and exact 54B frame count under port 0 | `program_linux.go:directionInsns`; real kernel `TestProgram_IPv6ICMPv6PortZero`, `TestProgram_IPv6ExtHeaderPortZero`, `TestProgram_IPv6TruncatedL4PortZero`, `TestProgram_IPv6HeaderEndingTheFrameCounts` PASS |
| AC-4 | Truncated IPv6 (14..53B) and ARP (`0x0806`) pass with `TCX_NEXT` without map writes | `program_linux.go:directionInsns`; real kernel `TestProgram_IPv6TruncatedPass` and `TestProgram_NonIPv4Pass` PASS |
| AC-5 | Live veth TCX attach reads IPv6 maps into `map[netip.Addr]uint64` with `ip6_ingress`/`ip6_egress` | `attach_linux.go:tcxAttachment.Counts,readIPMap,readIP6Map`; real kernel `TestAttachTCX_CountsIPv6Traffic` PASS |
| AC-6 | Unified IPv4+IPv6 Prometheus `/metrics`, `observation.Global()`, stale-timeout cleanup, and `show traffic usage` | `monitor.go:publishLocked`, `show.go:renderIPs`; `./le test unit pkg path ./internal/plugins/trafficusage/` (`1.069s`) + real kernel `TestMetricsScrape` PASS |
| AC-7 | Existing IPv4 tests pass unchanged | Real kernel `TestProgram_IngressSrcIP`, `TestProgram_EgressDstIP`, `TestProgram_IPOptionsMoveThePort`, `TestAttachTCX_CountsTraffic`, and unit `TestShowTrafficUsage` PASS |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `traffic { usage { ... } }` YANG config parse & validation | `test/parse/traffic-usage.ci` | Yes — read file; `./le test bgp parse --pattern traffic-usage` (`2/2 PASS`, `405ms`) |
| `show traffic usage` plugin command & RPC wiring | `test/plugin/traffic-usage-config.ci` | Yes — read file; `./le test bgp plugin --pattern traffic-usage-config` (`1/1 PASS`) |
| Linux TCX ingress/egress → `tcxAttachment.Counts` → `Monitor.publishLocked` → `/metrics` | `internal/plugins/trafficusage/attach_integration_linux_test.go` (`TestAttachTCX_CountsIPv6Traffic`, `TestMetricsScrape`) | Yes — read file; real kernel `go test -v -tags integration` PASS |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `TestProgram_Loads`, `TestProgram_IPv6IngressSrcIP`, `TestProgram_IPv6EgressDstIP`, and `TestTrafficUsageLoadsUnderTheUnitCapabilities` (`uid 65534`, `DaemonCapabilities`, `!CAP_PERFMON`) all PASS on real Linux kernel |
| A-2 | confirmed | Confirmed by Thomas on 2026-11-27 (`(1a, 2a)`); `TestProgram_TrackIPOffOmitsIPMaps` and `TestProgram_IPv6ExtHeaderPortZero` PASS |
| A-3 | confirmed | `TestProgram_IPv6HeaderEndingTheFrameCounts` (54B via `BPF_PROG_TEST_RUN`) and `TestProgram_IPv6TruncatedPass` (14..53B via veth TCX) PASS on real Linux kernel |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Architecture, guide, metrics, feature, and 6 secondary docs (`traffic-usage.md`, `guide/traffic-usage.md`, `metrics.md`, `features/traffic-usage.md`, `configuration.md`, `plugins.md`, `traffic-usage-monitor.md`, `cp-survival-5-detect-5-characterization.md`, `ddos-mitigation.md`, `functional-tests.md`) + `website/changes/fragments/traffic-usage-ipv6.md` | Verified via `git grep -in "traffic.usage\|trafficusage" docs/ features/` (0 remaining IPv4-only claims), `./le doc ste check` (`OK` on all 17 shipped/doc files + news fragment), and `./le doc check all` (0 failures in any `traffic-usage` doc, source anchor, or YANG node) | Yes |
