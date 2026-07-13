# Flow Export

<!-- source: internal/plugins/flowexport/exporter.go -- Exporter, Status, per-collector senders -->
<!-- source: internal/plugins/flowexport/yang/ze-flowexport-conf.yang -- YANG config -->
<!-- source: internal/plugins/flowexport/cmd_show.go -- ze-show:flow-export RPC -->
<!-- source: internal/plugins/flowexport/metrics.go -- ze_flowexport_* Prometheus metrics -->

Ze exports interface counters and per-flow records to external collectors over
UDP. The `flowexport` component is a registered component: it loads only when a
`flow-export { }` section is present in the config. Counter polling is driven by
the interface rate tracker's snapshot callback; per-flow records come from
packet sampling and from periodic conntrack table dumps.

Because counter export consumes the interface rate tracker, configuring
`flow-export { }` automatically pulls in the interface plugin (a registered
dependency). You do not need to add a separate `interface { }` section just to
get counter datagrams flowing.

Three export protocols are supported, selected per collector:

| Protocol | Reference | Carries |
|----------|-----------|---------|
| `sflow` | sFlow v5 | Interface counters, packet samples (flow samples) |
| `netflow9` | NetFlow v9 (RFC 3954) | Interface counters, per-flow records (template-based) |
| `ipfix` | IPFIX (RFC 7011) | Interface counters, per-flow records (template-based) |

## Configuration

All flow export configuration lives under a single `flow-export { }` section.

### Collectors

A collector is an export endpoint. Each collector names one protocol; you may
configure several collectors at once (for example one sFlow and one IPFIX).

| Field | Default | Description |
|-------|---------|-------------|
| `name` | - | Collector name (list key) |
| `address` | - | Collector IP address (mandatory) |
| `port` | 6343 | Collector UDP port |
| `protocol` | - | `sflow`, `netflow9`, or `ipfix` (mandatory) |
| `polling-interval` | 20 | Counter polling interval in seconds (1-3600) |
| `template-refresh` | 600 | Template refresh interval in seconds, NetFlow v9 / IPFIX (1-86400) |
| `sub-agent-id` | 0 | sFlow sub-agent identifier |
| `observation-domain` | 0 | IPFIX / NetFlow v9 observation domain ID |
| `agent-address` | - | sFlow agent address (the device's own stable IP, for example a loopback) |

An sFlow counter collector:

```
flow-export {
    collector edge-sflow {
        address 192.0.2.10;
        port 6343;
        protocol sflow;
        polling-interval 20;
        sub-agent-id 0;
        agent-address 198.51.100.1;
    }
}
```

A NetFlow v9 collector plus an IPFIX collector on the same device:

```
flow-export {
    collector nf9 {
        address 192.0.2.20;
        port 2055;
        protocol netflow9;
        polling-interval 30;
        template-refresh 600;
        observation-domain 1;
    }
    collector ipfix {
        address 192.0.2.21;
        port 4739;
        protocol ipfix;
        polling-interval 30;
        template-refresh 600;
        observation-domain 1;
    }
}
```

### Sampling

Packet sampling uses the Linux `tc sample` action plus the kernel `psample`
module and exports sampled packet headers as sFlow flow samples.

| Field | Default | Description |
|-------|---------|-------------|
| `name` | - | Interface to sample (list key) |
| `rate` | - | Sampling rate, 1-in-N packets (1-1000000) |
| `trunc-size` | 128 | Bytes of each sampled packet header to capture (64-1500) |
| `group` | 1 | psample group ID (1-2147483647) |

```
flow-export {
    sampling {
        interface eth0 {
            rate 1024;
            trunc-size 128;
            group 1;
        }
    }
}
```

### Conntrack

Conntrack-based per-flow records are exported via NetFlow v9 and IPFIX. The
exporter walks the conntrack table on a fixed interval and emits one record per
tracked flow.

| Field | Default | Description |
|-------|---------|-------------|
| `enabled` | false | Export per-flow records from the conntrack table |
| `active-timeout` | 60 | Seconds between conntrack table dumps (1-3600) |

```
flow-export {
    conntrack {
        enabled true;
        active-timeout 60;
    }
}
```

### Enrichment

Flow records can be enriched from the BGP RIB. With `bgp true`, the exporter
consumes the RIB best-change event and attaches the next-hop for each flow's
destination prefix.

| Field | Default | Description |
|-------|---------|-------------|
| `bgp` | false | Enrich flow records with the BGP next-hop |

```
flow-export {
    enrichment {
        bgp true;
    }
}
```

## Operational Command

`show flow export [name <collector>]` reports per-collector export statistics.
With no argument it lists every collector; with `name <collector>` it returns
that one collector, or an error if the name is not configured. When no `flow-export`
section is configured the command returns `{"status": "not-configured"}`.

```
ze cli -c 'show flow export'
```

```json
[
  {
    "name": "edge-sflow",
    "address": "192.0.2.10",
    "port": 6343,
    "protocol": "sflow",
    "datagrams-sent": 1842,
    "bytes-sent": 1325760,
    "errors": 0,
    "sequence": 1842,
    "last-export-time": 1748430000
  }
]
```

| Field | Description |
|-------|-------------|
| `name` | Collector name |
| `address` | Collector IP address |
| `port` | Collector UDP port |
| `protocol` | Export protocol |
| `datagrams-sent` | UDP datagrams sent to this collector |
| `bytes-sent` | Total bytes sent to this collector |
| `errors` | Send errors |
| `sequence` | Current export sequence number |
| `last-export-time` | Unix timestamp of the most recent poll (omitted before the first poll) |

The command produces JSON by default and supports the full set of pipe
operators.

## Prometheus Metrics

The component registers the following metrics:

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `ze_flowexport_datagrams_total` | counter | collector, protocol | Flow export datagrams sent |
| `ze_flowexport_bytes_total` | counter | collector, protocol | Flow export bytes sent |
| `ze_flowexport_errors_total` | counter | collector, protocol | Flow export send errors |
| `ze_flowexport_samples_total` | counter | interface | Packet samples received and exported |
| `ze_flowexport_flows_total` | counter | collector | Per-flow records exported |
| `ze_flowexport_flows_active` | gauge | - | Conntrack flows currently tracked for export |

## Limitations

- **BGP enrichment** attaches next-hop, origin AS, and the AS path from the RIB
  best-change event to flow records. Full-table replay events omit the AS data
  (the stored best-path record does not keep the AS-path handle); those entries
  are corrected by the next incremental best-path change.
- **Extended sFlow `if_counters`.** ifType, ifIndex, status, octet/packet/
  error/discard counters, promiscuous mode, ifSpeed, ifDirection (duplex), and
  inbound multicast packets are populated. ifSpeed and ifDirection come from
  sysfs (`/sys/class/net/<if>/speed` and `/duplex`) and read zero / unknown for
  virtual or down links, where the kernel does not report them. Outbound
  multicast and both inbound and outbound broadcast packet counts are left zero:
  `rtnl_link_stats64` exposes only an inbound multicast counter, with no
  transmit-multicast and no broadcast counters at all.
- **Conntrack export combines periodic dumps with destroy events.** Records
  are emitted on each table dump (`active-timeout`), and a
  `NFNLGRP_CONNTRACK_DESTROY` netlink listener exports each torn-down flow's
  residual counters immediately, so flows that begin and end between two dumps
  are not lost. The destroy listener needs `CAP_NET_ADMIN` and the
  `nf_conntrack_netlink` module; without them the worker logs that it is
  running in dump-only mode and continues with the periodic path alone.
- **Sampling requires Linux with `CAP_NET_ADMIN` and the kernel `psample`
  module.** On platforms without these, the sampling worker degrades and no
  flow samples are produced.

## Scale: conntrack vs sampling

Conntrack per-flow export gives **exact** per-flow accounting and suits edge /
moderate scale (up to low-thousands of new flows per second). Its cost scales
with flow **churn**: the delta tracker holds one entry per recently-seen flow,
the kernel table dump serialises every tracked flow over netlink each
`active-timeout`, and the destroy-event socket must keep up with the teardown
rate. A torn-down flow's tracking state is reclaimed within a few seconds of its
destroy event (or at `2 × active-timeout` if the event is missed), so memory is
bounded by `churn × residency`. Raising `active-timeout` on a busy device
lengthens residency for any flow whose destroy event is missed, so keep it
modest under high churn.

For **high-rate** links (100G internet mix, tens of thousands to millions of new
flows per second), do **not** rely on conntrack per-flow export: the table dump,
the destroy-event rate, and the export bandwidth all become bottlenecks. Use
**packet sampling** instead (`sampling { }`, sFlow flow samples), whose cost is
independent of flow rate because it processes only 1-in-N packets. Sampling is
the scalable mechanism for line-rate flow visibility; conntrack export is for
exact accounting where flow rates are modest.

## Security

The export protocols (sFlow, NetFlow v9, IPFIX) are unencrypted UDP and carry
no authentication. Run flow export over a dedicated management VLAN, and keep
the collector endpoints off any untrusted path.
