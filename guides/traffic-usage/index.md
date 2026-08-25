# Traffic Usage

<!-- source: internal/plugins/trafficusage/monitor.go -- reconcile, lifecycle, poller -->
<!-- source: internal/plugins/trafficusage/yang/ze-traffic-usage-conf.yang -- traffic-usage config -->
<!-- source: internal/plugins/trafficusage/show.go -- ze-show:traffic-usage RPC -->
<!-- source: internal/plugins/trafficusage/metrics.go -- ze_traffic_usage_* Prometheus metrics -->

Ze accounts per-(port, protocol) and, optionally, per-IP byte totals on
operator-selected interfaces using eBPF programs attached via TCX. The counters
are exported as Prometheus metrics and viewed with `show traffic usage`. The
`trafficusage` plugin loads only when a `traffic { usage { } }` section is present in
the config.

Traffic usage is **monitoring only**. The eBPF programs read packet headers and
update counters; they never drop, redirect, or modify traffic. Accounting is
IPv4 only. The plugin is Linux only and is a no-op on other platforms.

## How It Works

The plugin attaches one ingress and one egress eBPF program to each configured
interface using TCX (the kernel's traffic-control express attach point, kernel
6.6 and later). Each packet's IPv4 header is inspected and a byte count is added
to per-key maps:

- **Per-port (always on):** ingress traffic is keyed by destination port and
  protocol, egress traffic by source port and protocol. Packets with no L4 port
  (ICMP and other non-port protocols) aggregate under port `0`.
- **Per-IP (opt-in via `track-ip`):** ingress traffic is additionally keyed by
  source IPv4 address, egress traffic by destination IPv4 address. This is off
  by default to bound metric cardinality.

A user-space poller reads the maps on a fixed interval (`interval`), publishes
the absolute byte totals as Prometheus gauges, and removes metric series that
have not been seen within `stale-timeout` so that `/metrics` cardinality stays
bounded. Each map is an LRU of capacity `max-entries`; when a map fills, the
kernel evicts the least-recently-used keys, so the live keys are the current top
talkers.

### Pure-Go eBPF

The eBPF programs are assembled in **pure Go** as `asm.Instructions` using the
`github.com/cilium/ebpf` library and loaded from memory. There is no C source,
no committed `.o` object file, and no `clang`/`LLVM` toolchain in the build. This
deliberately differs from the L2TP XDP-policing plugin, which commits `bpf2go`
generated `.o` files. The cost of the pure-Go approach is hand-written assembly,
which is validated by `BPF_PROG_TEST_RUN` tests that feed crafted packets through
the loaded programs and assert on the resulting map state.
<!-- source: internal/plugins/trafficusage/program_linux.go -- pure-Go asm.Instructions program assembly -->

## Configuration

All traffic usage configuration lives under a single `traffic { usage { } }` section.

```
traffic {
    usage {
        enabled true
        interfaces {
            interface eth0 {
            }
            interface wan {           // a ze interface name (resolved to its OS device)
                track-ip false        // override the global track-ip just for wan
                max-entries 65536     // and give it a larger top-talker map
            }
        }
        interval 1000
        track-ip true                 // global default, inherited by eth0
        stale-timeout 300000
        max-entries 10240
    }
}
```

Interfaces are listed under `interfaces { }`, keyed by name, mirroring the OSPF
and IS-IS interface blocks. Each name is a ze interface name (the logical name
configured under `interface { }`); it is resolved to its OS/kernel device
(honoring the `os-name` / `mac-match` selectors) before the eBPF programs are
attached.

`track-ip`, `stale-timeout`, and `max-entries` are global defaults that each
interface inherits unless it sets its own value, so you can (for example) track
top-talker IPs with a large map on the WAN while leaving the LAN to port/protocol
counters only. `interval` is global (a single poll loop). Changing an
interface's `track-ip` or `max-entries` rebuilds and re-attaches its eBPF program
(its counters reset); changing `interval` takes effect on the next poll.

| Field | Type | Default | Range | Description |
|-------|------|---------|-------|-------------|
| `enabled` | boolean | `false` | - | Enable accounting. |
| `interfaces` / `interface <name>` | keyed list | - | - | ze interface names to account on, resolved to their OS devices. |
| `interfaces` / `interface <name>` / `enabled` | boolean | `true` | - | Account traffic on this interface. |
| `interfaces` / `interface <name>` / `track-ip` | boolean | global | - | Per-interface override of `track-ip`. |
| `interfaces` / `interface <name>` / `stale-timeout` | uint32 (ms) | global | 0 disables | Per-interface override of `stale-timeout`. |
| `interfaces` / `interface <name>` / `max-entries` | uint32 | global | 1..4294967295 | Per-interface override of `max-entries`. |
| `interval` | uint32 (ms) | `1000` | 100..3600000 | Map poll interval in milliseconds (global). |
| `stale-timeout` | uint32 (ms) | `300000` | 0 disables | Global default: remove a metric series unseen for this long. |
| `track-ip` | boolean | `false` | - | Global default: also account per source (ingress) / destination (egress) IPv4. Off by default to bound metric cardinality. |
| `max-entries` | uint32 | `10240` | 1..4294967295 | Global default: per-map LRU capacity; the kernel evicts the least-recently-used keys (top-talker retention) when a map fills. |
<!-- source: internal/plugins/trafficusage/yang/ze-traffic-usage-conf.yang -- traffic-usage -->

## Operational Command

`show traffic usage [name <interface>]` reports the accounted byte totals. With
no argument it lists every monitored interface; with `name <interface>` it
returns one interface. Output is structured per interface: `ingress-ports`,
`egress-ports`, and (only when `track-ip` is enabled) `ingress-ips` and
`egress-ips`, plus `map-entries` reporting per-map fill levels.

```
ze cli -c 'show traffic usage'
ze cli -c 'show traffic usage name eth0'
```

The command answers structured data, which `ze cli -c` renders in the format
`environment cli format default` names. The registered value is `text`. Append
`| json`, `| table`, `| yaml` or `| ndjson` to select another one, and the full
set of pipe operators applies.
<!-- source: internal/plugins/trafficusage/show.go -- ze-show:traffic-usage -->

### Demo: Attribute a live traffic burst

Attach Ze's pure-Go eBPF accounting to a local veth, generate ICMP and HTTP traffic, and inspect source, protocol, port, and byte totals.

[Download the asciicast recording](../../assets/demos/traffic-anomaly.cast?v=de2a0154a3) · [Plain-text transcript](../../assets/demos/traffic-anomaly.txt?v=ebe3635e22)

Recorded with Ze 26.08.25 in a Linux namespace lab using Ze recorder. Duration: 1 minute 24 seconds.

```console
An operator sees an unexpected burst on `traffic0` and needs to identify the source and application without capturing payloads.

$ ze config show demos/terminal/traffic-anomaly/ze.conf traffic usage
The daemon configuration shows eBPF accounting enabled on `traffic0`, with per-IP tracking and bounded maps.

$ ze cli -c 'show traffic usage name traffic0'
The complete baseline snapshot is displayed.

$ ip netns exec traffic-peer ping -c 4 10.77.0.1
$ ip netns exec traffic-peer curl -s -o /dev/null http://10.77.0.1:8080/payload.txt
The isolated workload sends ICMP and HTTP traffic.

$ ze cli -c 'show traffic usage name traffic0'
The complete live snapshot attributes bytes to source 10.77.0.2, ICMP, TCP destination port 8080, and reports map occupancy. The accounting path observes traffic only and never modifies or drops packets.
```


## Prometheus Metrics

The plugin registers the following metrics. All are GaugeVec carrying absolute
cumulative byte totals.

| Metric | Type | Labels | When | Description |
|--------|------|--------|------|-------------|
| `ze_traffic_usage_ingress_port_bytes_total` | GaugeVec | interface, dst_port, protocol | always | Ingress bytes per destination port and protocol. |
| `ze_traffic_usage_egress_port_bytes_total` | GaugeVec | interface, src_port, protocol | always | Egress bytes per source port and protocol. |
| `ze_traffic_usage_ingress_bytes_total` | GaugeVec | interface, src_ip | track-ip only | Ingress bytes per source IPv4. |
| `ze_traffic_usage_egress_bytes_total` | GaugeVec | interface, dst_ip | track-ip only | Egress bytes per destination IPv4. |
| `ze_traffic_usage_map_entries` | GaugeVec | interface, map | always | Live entry count per BPF map (LRU fill level). |

The `protocol` label is one of `tcp`, `udp`, `icmp`, `gre`, `esp`, `ah`, or
`icmpv6`; unknown protocols render as their decimal protocol number. ICMP and
other packets with no L4 port aggregate under `dst_port`/`src_port` `0`.
<!-- source: internal/plugins/trafficusage/metrics.go -- BindMetrics -->

## Doctor Check

The `doctor-traffic-usage-ebpf` check warns when `traffic-usage` is enabled in
the config but eBPF/TCX is unavailable on the running kernel, so the operator
learns that accounting is configured but inert rather than silently producing no
metrics.
<!-- source: internal/plugins/trafficusage/doctor.go -- checkTrafficUsageBPF, doctor-traffic-usage-ebpf -->

## Limitations

- **IPv4 only.** Per-port and per-IP accounting cover IPv4 traffic; IPv6 is not
  counted.
- **Monitoring only.** The eBPF programs only read headers and update counters.
  They never drop, redirect, or modify traffic.
- **Per-IP requires `track-ip`.** The `ze_traffic_usage_ingress_bytes_total` and
  `ze_traffic_usage_egress_bytes_total` series are only produced when `track-ip`
  is enabled. Per-port accounting is always on.
- **Linux >= 6.6, no-op elsewhere.** TCX attach requires Linux 6.6 or later, plus
  `CAP_BPF` and `CAP_NET_ADMIN`. On other platforms the plugin is a no-op. The
  `doctor-traffic-usage-ebpf` check reports when the kernel cannot support the
  feature.
- **Bounded cardinality.** Stale metric series (unseen within `stale-timeout`)
  are deleted, and each map is an LRU capped at `max-entries`; under high key
  churn only the current top talkers are retained.
