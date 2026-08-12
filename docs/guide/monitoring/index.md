# Monitoring

Ze provides real-time BGP event monitoring and a live peer dashboard through the CLI. Commands follow verb-first syntax: `monitor <module>`.
<!-- source: internal/component/bgp/plugins/cmd/monitor/ -- monitor streaming RPCs -->

## Live Peer Dashboard

```
ze cli -c "monitor bgp"
```

The dashboard refreshes every 2 seconds and shows router identity plus a sortable, colour-coded peer table with update rates. Keys: j/k moves, s/S sorts, Enter opens detail, and Esc exits.
<!-- source: internal/component/cli/model_dashboard.go -- isDashboardCommand -->

## Event Streaming

```
ze cli -c "monitor event"
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
ze cli -c "monitor event peer upstream1 include update direction received"
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
ze cli -c "monitor event | json"      # Full JSON envelope
ze cli -c "monitor event | table"     # Tabular format
ze cli -c "monitor event | match rx"  # Regex filter on output
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
      "local": {"address": "10.0.0.2", "as": 65000},
      "remote": {"address": "10.0.0.1", "as": 65001}
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
    "peer": {"address": "10.0.0.1", "local": {"address": "10.0.0.2", "as": 65000}, "remote": {"address": "10.0.0.1", "as": 65001}},
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
    "peer": {"address": "10.0.0.1", "local": {"address": "10.0.0.2", "as": 65000}, "remote": {"address": "10.0.0.1", "as": 65001}},
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

The plugin receives events through its `OnEvent` callback. See [Plugins guide](../plugins/index.md) for details.
<!-- source: internal/component/plugin/server/ -- event dispatch to plugins -->

## Prometheus Metrics

Ze exposes Prometheus metrics when `telemetry { prometheus { ... } }` is configured. BGP metrics are refreshed every 10 seconds. By default the HTTP listener binds to `127.0.0.1:9273`; configure an explicit server address to expose it to remote scrapers.

The `netdata` block only controls Netdata-compatible OS collector metrics. It does not rename Ze-native metrics such as `ze_bgp_*`, `ze_bfd_*`, or `ze_l2tp_*`.

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

### HTTP Basic Authentication

When `basic-auth/enabled` is true, Ze requires HTTP Basic Authentication for every handler on the Prometheus service, including both `/metrics` and `/health`. The password is stored as a bcrypt hash in the persisted config. Use `plaintext-password` when editing the config and the commit hook will replace it with `password`. If automation already has a hash from `ze passwd`, set `password` directly.

Prometheus scrape configuration:

```yaml
scrape_configs:
  - job_name: ze
    static_configs:
      - targets: ["router.example.net:9273"]
    basic_auth:
      username: prometheus
      password: secret
```

Basic Auth does not provide transport encryption. Keep the listener on loopback, use a trusted management network, or put TLS in front of the service if the scrape crosses an untrusted network.

Per-collector overrides:

```
netdata {
    collector diskspace { enabled false; }
    collector snmp6 { interval 10; }
}
```
<!-- source: internal/component/bgp/reactor/reactor_metrics.go -- initReactorMetrics, metricsUpdateLoop -->
<!-- source: internal/component/telemetry/exporter/yang/ze-telemetry-conf.yang -->

### OS Metrics (Netdata-compatible)

Ze exports 138 OS metrics matching Netdata's Prometheus format exactly (same names, labels, values), acting as a drop-in replacement for Netdata's `/api/v1/allmetrics?format=prometheus` endpoint. Existing Grafana dashboards built against Netdata continue to work unchanged.

Metric name format: `{prefix}_{context}_{units}_average{chart="...",dimension="...",family="..."}`, where `{prefix}` is `telemetry.prometheus.netdata.prefix`.

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

### Host Inventory Metrics
<!-- source: internal/component/host/metrics.go -- RegisterMetrics, collectFrom -->

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `ze_host_memory_total_bytes` | gauge | - | Total physical memory in bytes |
| `ze_host_memory_available_bytes` | gauge | - | Available physical memory in bytes |
| `ze_host_cpu_logical_count` | gauge | - | Number of logical CPUs |
| `ze_host_cpu_physical_cores` | gauge | - | Number of physical CPU cores |
| `ze_host_uptime_seconds` | gauge | - | Host uptime in seconds |
| `ze_host_ecc_correctable_errors_total` | gauge | - | ECC correctable error count |
| `ze_host_ecc_uncorrectable_errors_total` | gauge | - | ECC uncorrectable error count |
| `ze_host_nic_link_speed_mbps` | gauge | `name` | NIC link speed in Mbps |
| `ze_host_nic_carrier` | gauge | `name` | NIC carrier state (1=up, 0=down) |
| `ze_host_storage_size_bytes` | gauge | `name` | Block device size in bytes |
| `ze_host_thermal_temp_mc` | gauge | `name`, `device` | Thermal sensor reading in millicelsius |

Host metrics are refreshed on a configurable interval (default 60 seconds). Linux only; on other platforms no host metrics are registered.

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

#### Session Lifecycle

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `ze_peer_sessions_established_total` | counter | `peer` | Times the session reached Established |
| `ze_peer_session_flaps_total` | counter | `peer` | Sessions dropped from Established |
| `ze_peer_state_transitions_total` | counter | `peer`, `from`, `to` | Peer FSM state transitions |
| `ze_peer_notifications_sent_total` | counter | `peer`, `code`, `subcode` | NOTIFICATION messages sent |
| `ze_peer_notifications_received_total` | counter | `peer`, `code`, `subcode` | NOTIFICATION messages received |
| `ze_peer_session_duration_seconds` | gauge | `peer` | Seconds since the session established |
| `ze_bgp_open_in_established_total` | counter | `peer` | OPEN messages refused because the connection was already in Established or OpenConfirm |
| `ze_bgp_connect_retry_counter` | gauge | `peer` | RFC 4271 ConnectRetryCounter: times this peer has tried to establish a session since the last operator start or stop |

A non-zero `ze_bgp_open_in_established_total` names a peer that tried to
re-negotiate mid-session. Ze answers it with a Cease and closes the connection.

`ze_bgp_connect_retry_counter` is RFC 4271 Section 8.1.1 mandatory session
attribute 2, "the number of times a BGP peer has tried to establish a peer
session". The BGP FSM raises it by one on each teardown RFC 4271 Section 8.2.2
gives an increment clause: a hold-timer expiry, a header or OPEN error, a
NOTIFICATION, an UPDATE error, a mid-session TCP failure, and any event the
state does not expect. Two events set it back to zero, and only these two: the
operator starts the peer, and the operator stops it. A reconnect does not,
which is what makes the value a history rather than a flag.

It is a GAUGE, not a counter. Those operator resets make the value go down, and
a Prometheus counter that goes down reads as a counter reset to `rate()`. Use
the value directly, not a rate: it already IS a count.

Read the same number per peer with `show bgp peer <address> detail`, field
`connect-retry-counter`.
<!-- source: internal/component/bgp/reactor/reactor_metrics.go -- initReactorMetrics -->
<!-- source: internal/component/bgp/reactor/session_handlers.go -- handleOpen -->
<!-- source: internal/component/bgp/fsm/connect_retry_counter.go -- ConnectRetryCounter -->

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

#### Attribute Index

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `ze_bgp_update_span_spill_total` | counter | `peer` | Received UPDATEs whose attribute count exceeded the inline span capacity |

Ze indexes the path attributes of every received UPDATE once, on the receive
goroutine, and holds the first 8 spans inline. A 9th attribute puts the remainder
on the heap and increments this counter. A public-internet corpus of 112M routes
has a maximum of 10 attributes and 99.9% at 8 or fewer, so a steady rate here
means either an unusual peer or an attribute set worth raising the inline size
for.
<!-- source: internal/core/bgp/attribute/span.go -- SpanInline -->
<!-- source: internal/component/bgp/reactor/session_validation.go -- publishBase -->

#### Egress Modification Failures

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `ze_bgp_update_modify_failed_total` | counter | `reason` | An egress modification could not be applied to a route |

A non-zero value means a configured modification (a next-hop rewrite, a community
strip, a private-ASN removal) did not fit the route it applied to. The `reason`
label set is closed: `malformed`, `overflow`, `attr-length-range`,
`withdrawn-size`. Two further values exist and both indicate a defect rather than
peer input: `no-failure` must never be emitted, and `unclassified` means a reason
reached the counter that no constant produced.
<!-- source: internal/component/bgp/reactor/forward_modify_failure.go -- modifyFailure -->

Treat any increment as a policy that did not take effect. Ze counts the failure
at the point the modification is built, on all five paths that build one: the
forward rail, the route-server rail, the ingress and egress policy chains, and
the RFC 9494 stale re-advertise rail.
<!-- source: internal/component/bgp/reactor/forward_build.go -- buildModifiedPayload -->

#### Well-Known Community Suppressions

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `ze_bgp_wellknown_community_suppressed_total` | counter | `community` | A route was withheld from one destination peer by an RFC 1997 well-known community |

An increment means a route ze received carrying NO_EXPORT, NO_ADVERTISE or
NO_EXPORT_SUBCONFED was not advertised to a peer that community forbids. One
observation is one destination, so a route withheld from 20 peers counts 20. The
suppression is not configurable and is never logged per route, so this counter is
the only place an operator sees it.
<!-- source: internal/component/bgp/reactor/forward_wellknown.go -- wellKnownAllowsEgress -->

The `community` label set is closed and holds three values: `no-advertise`,
`no-export-subconfed`, `no-export`. A route carrying more than one is counted
under the strictest, so one suppressed route is always one observation.
<!-- source: internal/component/bgp/wireu/wellknown.go -- WellKnown.BlockingName -->

The counter says nothing about withdrawals. RFC 1997 forbids ADVERTISING such a
route, so a suppressed destination still receives the withdrawals of the same
UPDATE and keeps no route ze cannot take back.

#### Announces Refused For Size

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `ze_bgp_announce_dropped_oversize_total` | counter | `rail`, `stage` | An announce did not fit its build buffer and was not sent to that peer |

An increment means a route never reached one peer. Ze queries the encoded size
before it writes, so no truncated UPDATE goes out: the announce is abandoned
whole. Nothing else reports this to the operator, because the peer is not
notified and the session is not disturbed.
<!-- source: internal/component/bgp/reactor/announce_metrics.go -- recordAnnounceDroppedOversize -->

Both label sets are closed. `rail` is `batch` for an announce built from the API
and `queued` for one built from the RIB. `stage` is `nlri` when the prefix block
did not fit and `attributes` when the path-attribute block did not.
<!-- source: internal/component/bgp/reactor/reactor_api_batch.go -- logAnnounceTooLarge -->
<!-- source: internal/component/bgp/reactor/peer_rib_routes.go -- logRIBRouteTooLarge -->

A `batch` increment also reaches the caller: `AnnounceNLRIBatch` returns
`errAnnounceTooLarge`, which the dispatcher turns into a `StatusError` response,
so a plugin sees its own announce refused. A `queued` increment has no such
channel and the counter is the only signal besides the log line. Act on either by
sending fewer prefixes per announce, or by reducing the attributes on the route.

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
ze cli -c "show bgp summary"
ze cli -c "show bgp rib received"
ze cli -c "show bgp rpki status"
```
<!-- source: internal/component/cli/client/main.go -- Run, Execute, StreamMonitor -->
