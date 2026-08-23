# Plugin Metrics

Plugins that have observable runtime state register their own Prometheus metrics
via the existing `ConfigureMetrics` callback in the `Registration` struct.

<!-- source: internal/component/plugin/registry/registry.go -- Registration.ConfigureMetrics -->

## Implementation Pattern

Each plugin follows the same three-step pattern used by bgp-rib and bgp-gr:

1. **Registration:** Set `ConfigureMetrics` in the `Registration` struct inside
   `init()`. The callback receives a typed `metrics.Registry`; no assertion is
   needed.

2. **Storage:** Declare a package-level `atomic.Pointer` holding a metrics struct.
   The struct contains the counters and gauges created from the registry.

3. **Usage:** At metric update points, load the pointer. If nil (metrics disabled),
   skip. Otherwise update the metric.

<!-- source: internal/component/bgp/plugins/rib/register.go -- ConfigureMetrics callback -->
<!-- source: internal/component/bgp/plugins/rib/rib.go -- ribMetrics struct, metricsPtr, SetMetricsRegistry -->

## Naming Policy

Every metric name is built from four parts in fixed order:

```
ze_{scope}_{subject}_{detail}
```

| Part | Rule | Examples |
|------|------|---------|
| `ze` | Always. Global prefix for all ze metrics. | |
| scope | Plugin name, hyphens stripped, lowercase. Reactor uses functional area. | `rib`, `gr`, `fibkernel`, `systemrib`, `rpki`, `persist`, `watchdog` |
| subject | Singular noun: the type of thing being measured. | `route`, `peer`, `timer`, `session`, `vrp` |
| detail | What aspect. Depends on metric type (see below). | `inserts`, `active`, `expired` |

<!-- source: internal/component/bgp/plugins/rib/rib.go -- ze_rib_* metric names -->
<!-- source: internal/component/bgp/plugins/gr/gr.go -- ze_gr_* metric names -->
<!-- source: internal/component/bgp/reactor/reactor_metrics.go -- ze_peer_* metric names -->
<!-- source: internal/component/bgp/filterapi/metrics.go -- ze_bgp_attr_mod_* metric names -->

### Type-Specific Rules

| Type | Pattern | Suffix | Examples |
|------|---------|--------|---------|
| Counter | `ze_{scope}_{subject}_{event}_total` | Always `_total` | `ze_rib_route_inserts_total`, `ze_gr_timer_expired_total` |
| Gauge | `ze_{scope}_{subject}_{qualifier}` | Never `_total` | `ze_gr_active_peers`, `ze_fibkernel_routes_installed` |
| Histogram | `ze_{scope}_{subject}_{action}_seconds` | Always unit suffix | `ze_peer_dial_seconds` |
| CounterVec | Same as Counter, labels for dimensions | `_total` | `ze_rpki_validation_outcomes_total` with `{result}` label |
| GaugeVec | Same as Gauge, labels for dimensions | No `_total` | `ze_rib_routes_in` with `{peer}` label |

### Subject Rules

The subject is always a **singular noun** describing the category of thing measured.
When the metric counts a quantity of that noun, the grammatical plural appears
naturally in the detail.

| Subject (singular) | Gauge (current count) | Counter (cumulative events) |
|--------------------|-----------------------|-----------------------------|
| route | `routes_installed` | `route_installs_total` |
| peer | `peers_up` | `peer_flaps_total` |
| vrp | `vrps_cached` | |
| session | `sessions_active` | `session_teardowns_total` |

### Detail Rules

For **counters**, the detail is the event that happened, as a noun or past participle:
`inserts`, `withdrawals`, `expired`, `installs`, `removals`, `errors`.

For **gauges**, the detail is a qualifier distinguishing this gauge from others
in the same scope: `installed`, `active`, `up`, `cached`, `in`, `out`.

### Label Rules

Use labels for runtime dimensions. Never encode variable data in metric names.

| Label | When to use | Example |
|-------|-------------|---------|
| `peer` | Per-peer breakdown | `ze_rib_routes_in{peer="10.0.0.1"}` |
| `family` | Per-address-family breakdown | `ze_rib_route_inserts_total{family="ipv4/unicast"}` |
| `operation` | Error categorization | `ze_fibkernel_errors_total{operation="add"}` |
| `result` | Outcome categorization | `ze_rpki_validation_outcomes_total{result="valid"}` |

### Scope-to-Prefix Mapping

| Plugin (Registration Name) | Metric scope | Rationale |
|-----------------------------|-------------|-----------|
| `bgp-rib` | `rib` | Established; `bgp` prefix would be redundant |
| `bgp-gr` | `gr` | Established |
| `fib-kernel` | `fibkernel` | Hyphen stripped |
| `rib` (sysrib) | `systemrib` | Registration name is `rib` but `ze_rib_` is taken by bgp-rib; `systemrib` reads naturally |
| `bgp-watchdog` | `watchdog` | `bgp` prefix redundant |
| `bgp-rpki` | `rpki` | `bgp` prefix redundant |
| `bgp-persist` | `persist` | `bgp` prefix redundant |
| `bgp-role` | `role` | `bgp` prefix redundant |

### Full Inventory

| Metric Name | Type | Labels | Owner |
|-------------|------|--------|-------|
| `ze_rib_routes_in_total` | Gauge | | bgp-rib |
| `ze_rib_routes_out_total` | Gauge | | bgp-rib |
| `ze_rib_routes_in` | GaugeVec | peer | bgp-rib |
| `ze_rib_routes_out` | GaugeVec | peer | bgp-rib |
| `ze_rib_route_inserts_total` | CounterVec | peer, family | bgp-rib |
| `ze_rib_route_withdrawals_total` | CounterVec | peer, family | bgp-rib |
| `ze_attr_pool_intern_total` | GaugeVec | pool | bgp-rib |
| `ze_attr_pool_dedup_hits_total` | GaugeVec | pool | bgp-rib |
| `ze_attr_pool_slots_used` | GaugeVec | pool | bgp-rib |
| `ze_bgp_attr_mod_remove_buffer_refused_total` | CounterVec | attribute | bgp reactor (filterapi) |
| `ze_gr_active_peers` | Gauge | | bgp-gr |
| `ze_gr_stale_routes` | GaugeVec | peer | bgp-gr |
| `ze_gr_timer_expired_total` | CounterVec | peer | bgp-gr |
| `ze_fibkernel_routes_installed` | Gauge | | fib-kernel |
| `ze_fibkernel_route_installs_total` | Counter | | fib-kernel |
| `ze_fibkernel_route_updates_total` | Counter | | fib-kernel |
| `ze_fibkernel_route_removals_total` | Counter | | fib-kernel |
| `ze_fibkernel_errors_total` | CounterVec | operation | fib-kernel |
| `ze_systemrib_routes_best` | Gauge | | rib (sysrib) |
| `ze_systemrib_route_changes_total` | CounterVec | action | rib (sysrib) |
| `ze_systemrib_events_received_total` | Counter | | rib (sysrib) |
| `ze_watchdog_peers_up` | Gauge | | bgp-watchdog |
| `ze_watchdog_route_announcements_total` | Counter | | bgp-watchdog |
| `ze_watchdog_route_withdrawals_total` | Counter | | bgp-watchdog |
| `ze_rpki_vrps_cached` | Gauge | | bgp-rpki |
| `ze_rpki_sessions_active` | Gauge | | bgp-rpki |
| `ze_rpki_validation_outcomes_total` | CounterVec | result | bgp-rpki |
| `ze_role_route_rejects_total` | CounterVec | reason | bgp-role |
| `ze_role_route_suppressions_total` | CounterVec | reason | bgp-role |
| `ze_persist_routes_stored` | Gauge | | bgp-persist |
| `ze_persist_peers_tracked` | Gauge | | bgp-persist |
| `ze_persist_route_replays_total` | Counter | | bgp-persist |
| `ze_isis_adjacencies_up` | GaugeVec | level, interface | isis |
| `ze_isis_adjacencies_total` | GaugeVec | level | isis |
| `ze_isis_lsps` | GaugeVec | level | isis (lsdb) |
| `ze_isis_lsp_fragments` | GaugeVec | level | isis (lsdb) |
| `ze_isis_lsp_originations_total` | CounterVec | level | isis (lsdb) |
| `ze_isis_sequence_wraps_total` | CounterVec | level | isis (lsdb) |
| `ze_isis_purges_total` | CounterVec | level | isis (lsdb) |
| `ze_isis_lsps_received_total` | CounterVec | level | isis (flooding) |
| `ze_isis_lsps_transmitted_total` | CounterVec | level | isis (flooding) |
| `ze_isis_csnp_sent_total` | CounterVec | level | isis (flooding) |
| `ze_isis_csnp_received_total` | CounterVec | level | isis (flooding) |
| `ze_isis_psnp_sent_total` | CounterVec | level | isis (flooding) |
| `ze_isis_psnp_received_total` | CounterVec | level | isis (flooding) |
| `ze_isis_srm_resends_total` | CounterVec | level | isis (flooding) |
| `ze_isis_lsps_dropped_total` | CounterVec | level, reason | isis (flooding) |
| `ze_isis_dis_elections_total` | CounterVec | level | isis (dis) |
| `ze_isis_pseudonode_lsps` | GaugeVec | level | isis (dis) |
| `ze_isis_auth_failures_total` | CounterVec | level, interface | isis (auth) |
| `ze_isis_redist_injected_total` | CounterVec | source, afi | isis (redistribute) |
| `ze_isis_redist_withdrawn_total` | CounterVec | source, afi | isis (redistribute) |
| `ze_isis_redist_inject_failures_total` | CounterVec | source | isis (redistribute) |
| `ze_isis_lsp_reoriginations_total` | CounterVec | level | isis (redistribute) |
| `ze_isis_spf_runs_total` | CounterVec | level | isis (spf) |
| `ze_isis_spf_duration_seconds` | HistogramVec | level | isis (spf) |
| `ze_isis_spf_nodes` | GaugeVec | level | isis (spf) |
| `ze_isis_routes_installed` | GaugeVec | level, afi | isis (spf) |
| `ze_isis_frames_sent_total` | CounterVec | interface | isis (transport) |
| `ze_isis_frames_received_total` | CounterVec | interface | isis (transport) |
| `ze_isis_frames_dropped_total` | CounterVec | interface, reason | isis (transport) |
| `ze_isis_sockets_open` | Gauge | | isis (transport) |
| `ze_ospf_packets_sent_total` | CounterVec | interface, type | ospf, ospfv3 (transport) |
| `ze_ospf_packets_received_total` | CounterVec | interface, type | ospf, ospfv3 (transport) |
| `ze_ospf_packets_dropped_total` | CounterVec | interface, reason | ospf, ospfv3 (transport) |
| `ze_ospf_sockets_open` | Gauge | | ospf, ospfv3 (transport) |
| `ze_ospf_interface_up` | GaugeVec | area, interface | ospf (iface) |
| `ze_ospf_dr_elections_total` | CounterVec | interface | ospf (iface) |
| `ze_ospf_neighbors` | GaugeVec | area, interface, state | ospf (neighbor) |
| `ze_ospf_adjacencies_full` | GaugeVec | area | ospf (neighbor) |
| `ze_ospf_nsm_events_total` | CounterVec | event | ospf (neighbor) |
| `ze_ospf_lsdb_lsas` | GaugeVec | area, type | ospf (lsdb) |
| `ze_ospf_lsa_originations_total` | CounterVec | type | ospf (lsdb) |
| `ze_ospf_lsa_refreshes_total` | CounterVec | type | ospf (lsdb) |
| `ze_ospf_lsa_purges_total` | CounterVec | type | ospf (lsdb) |
| `ze_ospf_lsupdates_sent_total` | CounterVec | interface | ospf (lsdb) |
| `ze_ospf_lsupdates_received_total` | CounterVec | interface | ospf (lsdb) |
| `ze_ospf_lsacks_sent_total` | CounterVec | interface | ospf (lsdb) |
| `ze_ospf_retransmissions_total` | CounterVec | area | ospf (lsdb) |
| `ze_ospf_spf_runs_total` | CounterVec | area | ospf (spf) |
| `ze_ospf_spf_duration_seconds` | HistogramVec | area | ospf (spf) |
| `ze_ospf_routes_installed` | GaugeVec | type, af | ospf (spf) |
| `ze_ospf_abr` | Gauge | | ospf (spf) |
| `ze_ospf_summary_lsas` | GaugeVec | area | ospf (spf) |
| `ze_ospf_asbr` | Gauge | | ospf (engine) |
| `ze_ospf_external_lsas` | Gauge | | ospf (engine) |
| `ze_ospf_redist_injected_total` | CounterVec | source | ospf (redistribute) |
| `ze_ospf_redist_withdrawn_total` | CounterVec | source | ospf (redistribute) |
| `ze_ospf_nssa_translations_total` | CounterVec | area | ospf (engine) |
| `ze_ospf_auth_failures_total` | CounterVec | interface, reason | ospf (engine) |
| `ze_ospf_af_instances` | GaugeVec | af | ospf (RFC 5838 multi-AF) |
| `ze_ospf_af_bit_mismatch_total` | CounterVec | af | ospf (RFC 5838 multi-AF) |
| `ze_traffic_usage_ingress_port_bytes_total` | GaugeVec | interface, dst_port, protocol | traffic-usage |
| `ze_traffic_usage_egress_port_bytes_total` | GaugeVec | interface, src_port, protocol | traffic-usage |
| `ze_traffic_usage_ingress_bytes_total` | GaugeVec | interface, src_ip | traffic-usage (track-ip) |
| `ze_traffic_usage_egress_bytes_total` | GaugeVec | interface, dst_ip | traffic-usage (track-ip) |
| `ze_traffic_usage_map_entries` | GaugeVec | interface, map | traffic-usage |
| `ze_managed_clients_connected` | Gauge | | managed (hub server) |
| `ze_managed_config_fetch_total` | CounterVec | result | managed (hub server) |
| `ze_managed_config_changed_pushed_total` | Counter | | managed (hub server) |
| `ze_plugin_write_watchdog_total` | CounterVec | transport | plugin (server) |
| `ze_iface_owned_devices` | GaugeVec | owner | iface (owned-device registry) |
| `ze_iface_link_events_coalesced_total` | CounterVec | name | iface (link event queue) |
| `ze_iface_carrier_resyncs_total` | CounterVec | name | iface (carrier resync) |
| `ze_iface_resolver_events_dropped_total` | CounterVec | name | iface (resolver fan-out) |
| `ze_iface_ra_sent_total` | CounterVec | interface | iface-ra |
| `ze_iface_ra_solicited_total` | CounterVec | interface | iface-ra |

The three iface link-event counters answer the same question from different
points on the path, and none of them reports a lost final state. A rising
`ze_iface_link_events_coalesced_total` says the queue's worker fell behind the
kernel, which a config commit causes because it holds the lock that worker
takes. Nothing is lost when it rises: the queue keeps the state each interface
ENDED in. A rising `ze_iface_carrier_resyncs_total` says the recorded route
metric contradicted live carrier and ze moved the route to agree, which happens
when a route install failed or a device was already down at start. A rising
`ze_iface_resolver_events_dropped_total` names a LOGICAL interface whose
subscriber was too slow, so the fan-out discarded the OLDEST event buffered for
it to make room for the newest. That subscriber has lost the middle of a burst
and still sees the state the interface ended in.

Only the first two are read together. `ze_iface_link_events_coalesced_total`
rising while `ze_iface_carrier_resyncs_total` stays flat is the healthy shape
under load: the worker was behind and the event path still carried every
interface to its final state. The resync counter moving is the one that says a
transition never arrived at all.

<!-- source: internal/component/iface/rate.go -- ownedDevices gauge registration -->
<!-- source: internal/component/iface/link_queue.go -- coalesced and resync counters -->
<!-- source: internal/component/iface/resolve.go -- onLinkEvent and sendLatest, the discard counter -->
<!-- source: internal/component/iface/device_owner.go -- updateOwnedDeviceGauge -->
<!-- source: internal/plugins/iface/ra/ifacera.go -- SetMetricsRegistry, incSent, incSolicited -->
<!-- source: internal/component/plugin/server/managed_serve.go -- NewManagedServer metric registration -->
<!-- source: internal/component/plugin/server/server.go -- NewServer write-watchdog hook -->
<!-- source: pkg/plugin/rpc/conn.go -- fireWatchdog, SetWriteWatchdogHook -->

The two `iface-ra` counters carry one `interface` label each.
`ze_iface_ra_sent_total` counts every Router Advertisement put on the wire.
`ze_iface_ra_solicited_total` counts the subset that answered a Router
Solicitation, and each of those increments both counters, so the first one stays
the total. A rise in the solicited counter with a flat send rate means that
hosts join the link.

<!-- source: internal/plugins/iface/ra/ifacera.go -- incSent, incSolicited -->
<!-- source: internal/plugins/iface/ra/sender_linux.go -- send call sites -->

`ze_plugin_write_watchdog_total` increments when a plugin RPC write on a
transport that does not support `SetWriteDeadline` (SSH channels, `io.Pipe`)
stalls past the write-watchdog window (default 30s); the connection is then
closed (fail-fast). Deadline-capable transports (`net.Conn`) use write
deadlines instead and never contribute to this counter. The `transport` label
is a low-cardinality bucket (`stream`, `pipe`, `file`).

<!-- source: internal/plugins/trafficusage/metrics.go -- BindMetrics -->

<!-- source: internal/plugins/ospf/spf/computer.go -- SetMetrics -->
<!-- source: internal/plugins/ospf/spf/install.go -- SetMetrics and publishCounts -->

`ze_ospf_packets_dropped_total` counts transport drops before packet dispatch or
after a failed send. The Linux receive path currently emits reason
`malformed-ipv4` when a raw datagram is too short for an IPv4 header or its IHL
overruns the received bytes; `SendPacket` emits reason `send-error` when the
backend transmit call fails.
<!-- source: internal/plugins/ospf/transport/backend_linux.go -- deliverDatagram -->
<!-- source: internal/plugins/ospf/transport/transport.go -- SendPacket -->

The two `ospf (iface)` series track the OSPFv2 Interface State Machine:
`ze_ospf_interface_up{area,interface}` is 1 for active point-to-point or
broadcast interfaces and 0 for Down, passive, or loopback records;
`ze_ospf_dr_elections_total{interface}` increments when a broadcast interface's
elected DR or BDR changes. The three `ospf (neighbor)` series track the Neighbor
State Machine: `ze_ospf_neighbors{area,interface,state}` gauges neighbours by
NSM state, `ze_ospf_adjacencies_full{area}` gauges Full adjacencies, and
`ze_ospf_nsm_events_total{event}` counts state-machine events.

The eight `ospf (lsdb)` series track the RFC 2328 Section 13 flooding and
Section 14 aging machinery: `ze_ospf_lsdb_lsas{area,type}` gauges current LSDB
entries, `ze_ospf_lsa_originations_total{type}`,
`ze_ospf_lsa_refreshes_total{type}`, and `ze_ospf_lsa_purges_total{type}` count
self-LSA lifecycle events, `ze_ospf_lsupdates_sent_total{interface}` and
`ze_ospf_lsupdates_received_total{interface}` count flooding packets,
`ze_ospf_lsacks_sent_total{interface}` counts explicit acknowledgements, and
`ze_ospf_retransmissions_total{area}` counts retransmit-list resends.
<!-- source: internal/plugins/ospf/lsdb/lsdb.go -- SetMetrics -->
<!-- source: internal/plugins/ospf/lsdb/flooding.go -- ReceiveUpdate, RetransmitTick, sendAck -->
<!-- source: internal/plugins/ospf/instance.go -- setMetrics -->
<!-- source: internal/plugins/ospf/iface/iface.go -- setUpLocked, runElectionLocked -->
<!-- source: internal/plugins/ospf/neighbor/table.go -- setStateLocked, recordEventLocked -->

The five `ospf (spf)` series track route computation and install state:
`ze_ospf_spf_runs_total{area}` and `ze_ospf_spf_duration_seconds{area}` report
SPF scheduling and runtime, `ze_ospf_routes_installed{type,af}` reports the current
Loc-RIB-owned route count by OSPF path type and address family (RFC 5838), `ze_ospf_abr` is 1 only while this
router has an active backbone attachment and at least one other active area, and
`ze_ospf_summary_lsas{area}` reports current self-originated Summary-LSA count by
area.
<!-- source: internal/plugins/ospf/spf/computer.go -- SetMetrics and Run -->
<!-- source: internal/plugins/ospf/spf/install.go -- SetMetrics and publishCounts -->

The two `isis (dis)` series are the Designated IS / pseudo-node metrics (broadcast
LANs): `ze_isis_dis_elections_total` counts DIS elections that changed the elected
DIS per level, and `ze_isis_pseudonode_lsps` gauges the pseudo-node LSPs this node
currently originates as the elected DIS per level.

The `isis (auth)` series `ze_isis_auth_failures_total` counts IS-IS PDUs rejected
because authentication verification failed, by level and receiving interface. It
is incremented at the receive verify site (a PDU with no, wrong, or misordered
authentication TLV under configured auth) and is the operator's signal for a
key mismatch, a downgrade attempt, or a spoofed purge.

The four `isis (redistribute)` series track redistribution into IS-IS:
`ze_isis_redist_injected_total{source,afi}` and
`ze_isis_redist_withdrawn_total{source,afi}` count routes imported into IS-IS LSPs
(as TLV 135 IPv4 reachability or TLV 236 IPv6 reachability) and withdrawn, by
originating source (`connected`, `static`, `bgp`) and address family
(`afi=ipv4`|`ipv6`); `ze_isis_redist_inject_failures_total{source}` counts
injections whose LSP re-origination failed; and
`ze_isis_lsp_reoriginations_total{level}` counts the own-LSP re-originations that
redistribution drove, by level. Exporting IS-IS routes *out* to BGP uses the
generic `redistribute-orchestrator` counters, not an IS-IS-specific series.

The four `isis (spf)` series track the shortest-path computation and its Loc-RIB
install: `ze_isis_spf_runs_total{level}` counts Dijkstra runs per level,
`ze_isis_spf_duration_seconds{level}` is a histogram of each run's wall-clock
duration (buckets span 50us..500ms), `ze_isis_spf_nodes{level}` gauges the node
count of the last run's graph, and `ze_isis_routes_installed{level,afi}` gauges
the prefixes currently installed into the shared Loc-RIB by level and address
family (`afi=ipv4`|`ipv6`). The IPv6 install pass runs over the same shared SPF
tree, so the gauge is the only SPF series carrying an `afi` label.

The four `isis (transport)` series track the raw Layer-2 socket I/O:
`ze_isis_frames_sent_total{interface}` and
`ze_isis_frames_received_total{interface}` count frames transmitted and received
per circuit, `ze_isis_frames_dropped_total{interface,reason}` counts frames
dropped on receive by reason, and `ze_isis_sockets_open` gauges the number of
open IS-IS raw L2 sockets process-wide (unlabelled, a single process-level gauge).

The four `ospf (transport)` series track raw IPv4 protocol 89 socket I/O:
`ze_ospf_packets_sent_total{interface,type}` and
`ze_ospf_packets_received_total{interface,type}` count OSPF packets transmitted
and received per interface, `ze_ospf_packets_dropped_total{interface,reason}`
counts receive rejects and send failures, and `ze_ospf_sockets_open` gauges open
OSPF raw sockets process-wide. The `ospf (iface)` and `ospf (neighbor)` series
track interface state, DR/BDR election changes, NSM states, Full adjacencies,
and NSM events.

Dual-stack IPv6 (RFC 5308) adds **no new IS-IS series**: it sets `afi=ipv6` on the
labelled series above and on the SPF route-install gauge
`ze_isis_routes_installed{level,afi}`. IPv4 uses `afi=ipv4`; the IPv6 install pass
runs over the same shared SPF tree.

## When ConfigureMetrics Runs

The hook does **not** fire while the plugin is being spawned. `runPluginPhase`
starts every process of a startup phase before it runs the tier-ordered
handshake, and the Prometheus registry is not created until the bgp plugin's
stage-2 `OnConfigure` builds the reactor, so at spawn time there is usually no
registry yet. `registry.InjectPluginMetrics` therefore defers the hook and
`registry.SetMetricsRegistry` drains the deferred set the moment a registry
exists; whichever of the two happens second performs the injection, exactly once
per plugin.

<!-- source: internal/component/plugin/registry/registry.go -- InjectPluginMetrics, SetMetricsRegistry -->
<!-- source: internal/component/plugin/server/startup.go -- runPluginPhase spawns before the tier handshake -->

Two consequences for a plugin author:

- Do not read your metrics struct during `init()` or at the top of `RunEngine`;
  load the atomic pointer at each metric update point, as step 3 above says.
- Declaring a `Dependency` on `bgp` does not order the injection. Dependencies
  order the 5-stage handshake, not the spawn the hook used to be bound to.

## NopRegistry Fallback

When telemetry is disabled in config, plugins receive a `NopRegistry` that returns
no-op metric implementations. The nil-check on the atomic pointer handles the case
where `ConfigureMetrics` is never called at all (plugin loaded without a metrics
registry).

<!-- source: internal/core/metrics/nop.go -- NopRegistry -->

## Which Plugins Need Metrics

Not every plugin needs metrics. NLRI codec plugins (flowspec, evpn, labeled, etc.)
are stateless encoders/decoders with no runtime state to observe. Only plugins
with state machines, caches, or I/O operations benefit from metrics.

| Has metrics | No metrics needed |
|-------------|-------------------|
| Plugins with route state (rib, sysrib, persist) | NLRI codecs (bgp-nlri-*) |
| Plugins with external I/O (fib-kernel, rpki) | Pure capability-only plugins |
| Plugins with timers/state machines (gr, watchdog) | Format/encoding plugins |
| Plugins that drop or withhold routes (bgp-role) | |

A plugin that can refuse a route needs metrics even if it holds no state: a
suppression is invisible to the operator otherwise. `bgp-role` was once listed
as capability-only, but it filters every UPDATE on ingress and egress, so each
of its refusal paths carries a reason-labeled counter.

<!-- source: internal/component/bgp/plugins/role/metrics.go -- recordDrop -->
<!-- source: internal/component/bgp/plugins/role/otc.go -- OTCIngressFilter, OTCEgressFilter -->

## Reference Implementation

The bgp-rib plugin (`internal/component/bgp/plugins/rib/`) is the reference
implementation. It demonstrates all aspects: registration callback, metrics struct,
atomic pointer, periodic update loop, and per-peer label cleanup.
