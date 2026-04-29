# Monitoring

Ze provides real-time BGP event monitoring and a live peer dashboard through the CLI. Commands follow verb-first syntax: `monitor <module>`.
<!-- source: internal/component/bgp/plugins/cmd/monitor/ -- monitor streaming RPCs -->

## Live Peer Dashboard

```
ze cli monitor bgp
```

Auto-refreshing dashboard showing router identity, sortable color-coded peer table with update rates. Navigate with j/k, sort with s/S, Enter for detail, Esc to exit. Refreshes every 2 seconds.
<!-- source: internal/component/cli/model_dashboard.go -- isDashboardCommand -->

## Event Streaming

```
ze cli monitor event
```

### Filters

| Filter | Example | Description |
|--------|---------|-------------|
| `peer` | `peer upstream1` | Show events for one peer |
| `include` | `include update,state` | Filter by event type (comma-separated) |
| `exclude` | `exclude keepalive` | Exclude event types |
| `direction` | `direction received` | Only received or sent events |

Combine filters:

```
ze cli monitor event peer upstream1 include update direction received
```

### Event Types

| Event | Has Direction | Description |
|-------|---------------|-------------|
| `update` | Yes | Route announcements and withdrawals |
| `open` | Yes | OPEN message exchange |
| `notification` | Yes | Session error notifications |
| `keepalive` | Yes | Keepalive exchanges |
| `refresh` | Yes | Route refresh requests |
| `state` | No | Peer state changes (up/down) |
| `negotiated` | No | Capability negotiation results |
| `eor` | Yes | End-of-RIB markers |
| `rpki` | Yes | RPKI validation results |
<!-- source: internal/component/bgp/event.go -- event type definitions -->

### Output Formats

Pipe the output through format operators:

```
ze cli monitor event | json      # Full JSON envelope
ze cli monitor event | table     # Tabular format
ze cli monitor event | match rx  # Regex filter on output
```
<!-- source: internal/component/command/ -- ApplyJSON, ApplyTable pipe operators -->

## JSON Event Format

All events follow the ze-bgp JSON envelope:

```json
{
  "type": "bgp",
  "bgp": {
    "peer": {
      "address": "10.0.0.1",
      "remote": {"as": 65001}
    },
    "message": {
      "id": 42,
      "direction": "received",
      "type": "update"
    }
  }
}
```

### UPDATE Event

```json
{
  "type": "bgp",
  "bgp": {
    "peer": {"address": "10.0.0.1", "remote": {"as": 65001}},
    "message": {"id": 1, "direction": "received", "type": "update"},
    "update": {
      "ipv4/unicast": [
        {
          "next-hop": "10.0.0.1",
          "action": "add",
          "nlri": ["10.0.0.0/24", "10.0.1.0/24"]
        }
      ]
    },
    "origin": "igp",
    "as-path": [65001, 65002],
    "local-preference": 100
  }
}
```

### State Event

```json
{
  "type": "bgp",
  "bgp": {
    "peer": {"address": "10.0.0.1", "remote": {"as": 65001}},
    "message": {"type": "state"},
    "state": "up"
  }
}
```

## Programmatic Access

Plugins can subscribe to events via the SDK:

```
process my-plugin {
    receive [ update state ]
}
```

The plugin receives events through its `OnEvent` callback. See [Plugins guide](plugins.md) for details.
<!-- source: internal/component/plugin/server/ -- event dispatch to plugins -->

## Prometheus Metrics

Ze exposes Prometheus metrics when `telemetry { prometheus { ... } }` is configured. BGP metrics are refreshed every 10 seconds. The `netdata` block only controls Netdata-compatible OS collector metrics. It does not rename Ze-native metrics such as `ze_bgp_*`, `ze_bfd_*`, or `ze_l2tp_*`.

```
telemetry {
    prometheus {
        enabled true;
        server main {
            ip 0.0.0.0;
            port 9273;
        }
        path /metrics;
        basic-auth {
            enabled true;
            username prometheus;
            plaintext-password "secret";
        }
        netdata {
            enabled true;
            prefix netdata;
            interval 1;
            collector diskspace {
                enabled false;
            }
            collector snmp6 {
                interval 10;
            }
        }
    }
}
```

| Path | Default | Description |
|------|---------|-------------|
| `enabled` | false | Enable Prometheus HTTP endpoint |
| `server` | `127.0.0.1:9273` | Listener list. Explicit `0.0.0.0` binds all interfaces |
| `path` | `/metrics` | HTTP metrics path |
| `basic-auth/enabled` | false | Require HTTP Basic Authentication for metrics and health endpoints |
| `basic-auth/realm` | `ze prometheus` | Basic Auth realm |
| `basic-auth/username` | unset | Basic Auth username |
| `basic-auth/password` | unset | Bcrypt-hashed Basic Auth password |
| `basic-auth/plaintext-password` | unset | Write-only password input, hashed on commit |
| `netdata/enabled` | true | Enable Netdata-compatible OS collectors |
| `netdata/prefix` | `netdata` | Prefix for Netdata-compatible OS collector metrics only |
| `netdata/interval` | 1 | Netdata-compatible OS collector sampling interval (1-60s) |
| `netdata/collector` | -- | Per-Netdata-collector enable and interval overrides |

Deprecated compatibility aliases remain accepted: `prefix`, `interval`, and `collector` directly under `prometheus`. Prefer `netdata/prefix`, `netdata/interval`, and `netdata/collector` in new config.

Per-collector overrides:

```
netdata {
    collector diskspace { enabled false; }
    collector snmp6 { interval 10; }
}
```
<!-- source: internal/component/bgp/reactor/reactor_metrics.go -- initReactorMetrics, metricsUpdateLoop -->
<!-- source: internal/component/telemetry/schema/ze-telemetry-conf.yang -->

### OS Metrics (Netdata-compatible)

Ze exports 138 OS metrics matching Netdata's Prometheus format exactly (same names, labels, values), acting as a drop-in replacement for Netdata's `/api/v1/allmetrics?format=prometheus` endpoint. Existing Grafana dashboards built against Netdata continue to work unchanged.

Metric name format: `{prefix}_{context}_{units}_average{chart="...",dimension="...",family="..."}`

| Collector | /proc or /sys source | Charts exposed |
|-----------|---------------------|----------------|
| CPU | /proc/stat | `system.cpu`, `cpu.cpu<N>` |
| cpufreq | /sys/devices/system/cpu/cpu*/cpufreq | `cpufreq.cpufreq`, `cpu.core_throttling` |
| cpuidle | /sys/devices/system/cpu/cpu*/cpuidle | `cpuidle.cpu<N>_cpuidle` |
| Memory | /proc/meminfo | `system.ram`, `system.swap`, `mem.available`, `mem.committed`, `mem.kernel`, `mem.slab`, `mem.thp`, `mem.writeback`, `mem.hugepages`, `mem.reclaiming`, `mem.swap_cached`, `mem.cma`, `mem.directmaps`, `mem.hwcorrupt`, `mem.zswap` |
| Load | /proc/loadavg | `system.load` |
| Processes | /proc/stat | `system.processes`, `system.forks`, `system.ctxt`, `system.intr` |
| Interrupts | /proc/softirqs | `system.softirqs` |
| Pressure (PSI) | /proc/pressure/* | `system.{cpu,memory,io}_{some,full}_pressure` |
| Network (per-iface) | /proc/net/dev, /sys/class/net | `net.net`, `net.packets`, `net.errors`, `net.drops`, `net.fifo`, `net.compressed`, `net.events`, `net.speed`, `net.duplex`, `net.operstate`, `net.carrier`, `net.mtu` |
| Network (aggregate) | /proc/net/dev, snmp, snmp6 | `system.net`, `system.ipv4`, `system.ipv6` |
| IPv4 | /proc/net/snmp | `ipv4.packets`, `ipv4.errors`, `ipv4.tcppackets`, `ipv4.tcperrors`, `ipv4.tcphandshake`, `ipv4.tcpsock`, `ipv4.udppackets`, `ipv4.udperrors`, `ipv4.icmp`, `ipv4.icmpmsg`, `ipv4.fragsout`, `ipv4.fragsin` |
| IPv4 netstat | /proc/net/netstat | `ipv4.mcast`, `ipv4.mcastpkts`, `ipv4.bcast`, `ipv4.bcastpkts`, `ipv4.ecnpkts`, `ip.tcpconnaborts`, `ip.tcpmemorypressures`, `ip.tcpreorders`, `ip.tcpofo` |
| IPv6 | /proc/net/snmp6 | `ipv6.packets`, `ipv6.errors`, `ipv6.udppackets`, `ipv6.udperrors`, `ipv6.mcast`, `ipv6.fragsout`, `ipv6.fragsin` |
| Sockets | /proc/net/sockstat, sockstat6 | `ip.sockstat_sockets`, `ipv4.sockstat_tcp_sockets`, `ipv4.sockstat_tcp_mem`, `ipv4.sockstat_udp_sockets`, `ipv4.sockstat_udp_mem`, `ipv6.sockstat6_*` |
| Conntrack | /proc/net/stat/nf_conntrack | `netfilter.conntrack_sockets`, `_new`, `_changes`, `_errors`, `_search`, `_expect` |
| Softnet | /proc/net/softnet_stat | `system.softnet_stat`, `cpu.cpu<N>_softnet_stat` |
| Disk I/O | /proc/diskstats | `disk.io`, `disk.ops`, `disk.mops`, `disk.iotime`, `disk.busy`, `disk.backlog`, `disk.await`, `disk.svctm`, `disk.avgsz`, `disk.qops`, `system.io` |
| Disk space | /proc/mounts + statfs | `disk_space.<mount>` |
| mdstat | /proc/mdstat | `md.health`, `md.disks`, `md.mismatch_cnt` |
| ZFS | /proc/spl/kstat/zfs/arcstats | `zfs.arc_size`, `zfs.reads`, `zfs.hits`, `zfs.hits_rate`, `zfs.l2_size`, `zfs.l2_hits_rate`, `zfs.memory_ops` |
| btrfs | /sys/fs/btrfs/*/allocation | `btrfs.disk`, `btrfs.data`, `btrfs.metadata`, `btrfs.system` |
| VMstat | /proc/vmstat | `mem.pgfaults`, `system.pgpgio`, `mem.swapio`, `mem.oom_kill`, `mem.numa`, `mem.balloon`, `mem.zswapio`, `mem.ksm_cow`, `mem.thp_faults`, `mem.thp_collapse` |
| SCTP | /proc/net/sctp/snmp | `sctp.snmp` |
| IPVS | /proc/net/ip_vs_stats | `ipvs.net` |
| Wireless | /proc/net/wireless | `net_wireless.*` |
| Other | /proc/uptime, /proc/sys/kernel/random/entropy_avail, /proc/sys/fs/file-nr | `system.uptime`, `system.entropy`, `system.file_nr_used` |

Collectors whose data sources are absent (no ZFS loaded, no btrfs mounts, no wireless NICs, etc.) skip silently.

Side-by-side validation against Netdata:

```
curl -s http://localhost:9273/api/v1/allmetrics?format=prometheus | grep "^netdata_" | sed 's/ .*//' | sort -u > nd.txt
curl -s http://localhost:9274/metrics | grep "^netdata_" | sed 's/ .*//' | sort -u > ze.txt
diff nd.txt ze.txt
```
<!-- source: internal/component/telemetry/collector/ -- Netdata-compatible OS collectors -->

### BGP Metrics
<!-- source: internal/component/bgp/reactor/reactor_metrics.go -- initReactorMetrics -->

#### Instance

| Metric | Type | Description |
|--------|------|-------------|
| `ze_info` | gauge | Instance info (labels: `version`, `router_id`, `local_as`) |
| `ze_uptime_seconds` | gauge | Seconds since reactor started |
| `ze_peers_configured` | gauge | Number of configured peers |
| `ze_cache_entries` | gauge | UPDATE cache entry count |

#### Per-Peer

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `ze_peer_state` | gauge | `peer` | FSM state (0=stopped, 1=connecting, 2=active, 3=established) |
| `ze_peer_messages_received_total` | counter | `peer`, `type` | Messages received (type: update, keepalive, open, notification, refresh, eor) |
| `ze_peer_messages_sent_total` | counter | `peer`, `type` | Messages sent (type: update, keepalive, open, notification, refresh, eor) |

#### Startup and Connection Timing

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `ze_plugin_startup_seconds` | histogram | - | WaitForPluginStartupComplete duration |
| `ze_api_ready_seconds` | histogram | - | WaitForAPIReady duration |
| `ze_peer_dial_seconds` | histogram | `peer`, `result` | TCP dial duration (result: ok, fail) |
| `ze_peer_connect_attempt_seconds` | histogram | `peer` | Full connection attempt (runOnce) duration |
| `ze_peer_connect_attempts_total` | counter | `peer` | Connection attempts |
| `ze_peer_backoff_seconds` | histogram | `peer` | Backoff wait duration before retry |
<!-- source: internal/component/bgp/reactor/reactor_metrics.go -- initReactorMetrics -->

#### Forward Pool / Congestion

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `ze_forward_workers_active` | gauge | - | Active forward pool workers |
| `ze_bgp_pool_used_ratio` | gauge | - | Global overflow pool utilization (0.0 = empty, 1.0 = full) |
| `ze_bgp_overflow_items` | gauge | `peer` | Items in per-destination overflow buffer |
| `ze_bgp_overflow_ratio` | gauge | `source` | Per-source overflow ratio: overflowed / (forwarded + overflowed) |

#### Prefix Limits (RFC 4486)

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `ze_bgp_prefix_count` | gauge | `peer`, `family` | Current prefix count |
| `ze_bgp_prefix_maximum` | gauge | `peer`, `family` | Configured hard maximum |
| `ze_bgp_prefix_warning` | gauge | `peer`, `family` | Warning threshold |
| `ze_bgp_prefix_warning_exceeded` | gauge | `peer`, `family` | 1 if count >= warning |
| `ze_bgp_prefix_ratio` | gauge | `peer`, `family` | count / maximum (0.0 to 1.0+) |
| `ze_bgp_prefix_maximum_exceeded_total` | counter | `peer`, `family` | Times maximum exceeded |
| `ze_bgp_prefix_teardown_total` | counter | `peer` | Sessions torn down for prefix limit |
| `ze_bgp_prefix_stale` | gauge | `peer` | 1 if prefix data older than 6 months |

---

## Single Command

For scripting, use `-c` to execute a single command and exit:

```
ze cli -c "bgp summary"
ze cli -c "rib routes received"
ze cli -c "rpki status"
```
<!-- source: cmd/ze/cli/main.go -- Run, Execute, StreamMonitor -->
