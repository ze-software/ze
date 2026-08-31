# Configuration

This page walks through BGP peer configuration specifically, since it is
the most-configured surface. For every other subsystem's config syntax
(interfaces, firewall, L2TP, DHCP, and the rest of Ze's 38 plugin groups),
see the generated [Configuration
Reference](https://ze-software.net/reference/configuration/), built
straight from each plugin's own YANG module.

### Peer Settings

Peers are keyed by name (`peer <name> { }`) with IP and AS in nested containers:

| Setting | Description | Validation |
|---------|-------------|------------|
| `connection/remote/ip` | Peer IP address | Required (`ze:required`) |
| `session/asn/remote` | Peer AS number | Required (`ze:required`) |
| `session/asn/local` | Local AS number | Required (`ze:required`, inheritable from bgp level) |
| `connection/local/ip` | Local bind address | Suggested (`ze:suggest`, can be `auto`) |
| `router-id` | Per-peer router ID override | Optional (or inherited) |
| `name` | Peer key (must start with letter) | Required |
| `timer/receive-hold-time` | Hold timer (0 or 3-65535 seconds) | 1-2 rejected |
| `connection/remote/connect` | Initiate outbound connections (boolean) | Default: true |
| `connection/local/accept` | Accept inbound connections (boolean) | Default: true |
| `md5-password` | TCP MD5 authentication | Optional |
| `outgoing-ttl` | TTL for outgoing packets | Optional |
| `ttl-security` | Minimum TTL for incoming packets | Optional |
| `group-updates` | Enable/disable UPDATE grouping | Default: enabled |

<!-- source: internal/component/bgp/config/resolve.go -- ResolveBGPTree config resolution -->
<!-- source: internal/component/bgp/config/peers.go -- peer config parsing -->
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- BGP config YANG schema -->

### Required Field Validation

The `ze:required` and `ze:suggest` YANG extensions declare which fields must be present in a peer after config inheritance resolution (bgp -> group -> peer merge).

| Extension | Behavior |
|-----------|----------|
| `ze:required "path"` | Descendant field must have a value in each list entry after inheritance. Validated at `ze config validate`, editor commit, and daemon startup for any list (generic, not BGP-only). |
| `ze:suggest` | Field shown in the web creation form with inherited defaults but not mandatory. |

The registered `ze:validate` value validators run on the same bytes at
`ze config validate`, at daemon start, and at SIGHUP reload. One walk serves the
offline check and the daemon, so they cannot reach different verdicts. A value a
validator refuses stops the daemon and fails a reload, with no override and no
force flag, and the message names the section, the leaf, and the rule. A refusal
declines rollback recovery, so an operator's typo cannot start the daemon on a
config they never wrote. A missing mandatory field is graded a warning on both
surfaces rather than a refusal.
<!-- source: internal/component/config/validate_sections.go -- ValidateCustomSections, ErrCustomValidation -->
<!-- source: internal/component/config/loader.go -- LoadConfig calls ValidateCustomSections -->
<!-- source: cmd/ze/hub/main.go -- recoverableLoadError declines recovery for ErrCustomValidation -->

Peer required fields: `connection/remote/ip`, `session/asn/local`, `session/asn/remote`. Suggested: `connection/local/ip`.

Fields can be satisfied by inheritance: `session/asn/local` set at bgp level satisfies the requirement for all peers; `session/asn/remote` set at group level satisfies it for group members.

<!-- source: internal/component/config/yang/modules/ze-extensions.yang -- ze:required, ze:suggest extensions -->
<!-- source: internal/component/config/required.go -- CheckRequired (generic, any list) -->
<!-- source: internal/component/bgp/config/resolve.go -- CheckRequiredFields (BGP-specific) -->

### Prefix Limits (RFC 4486)

Per-peer per-family prefix maximum enforcement. Mandatory for every negotiated family.

| Setting | Scope | Description |
|---------|-------|-------------|
| `prefix { maximum N; }` | Per family | Hard maximum prefix count. Mandatory. |
| `prefix { warning N; }` | Per family | Warning threshold. Default: 90% of maximum. |
| `prefix { teardown true/false; }` | Per family | Tear down on exceed (default: true) or warn-only. |
| `prefix { idle-timeout N; }` | Per family | Seconds to wait before reconnect after this family caused a teardown. Default 0, which keeps the peer down. |
| `prefix { reconnect never\|backoff\|timer; }` | Per family | What the peer does after this family stopped the session. No value means `timer` when `idle-timeout` is above 0, and `never` when it is 0. |
| `prefix { count offered\|installed; }` | Per family | Which prefixes the count compared against `maximum` holds. Default `offered`. |

When a family exceeds its maximum: NOTIFICATION Cease/MaxPrefixes (subcode 1) is sent and the session is torn down. With `teardown false` on that family, the session stays up and the UPDATE that crossed the maximum is dropped. The drop is per UPDATE, not per NLRI: Ze consumes the whole message and delivers none of it, so routes of other families in that same UPDATE are dropped with it. Each family reads its own `teardown` value, so one family can warn while another stops the session.
<!-- source: internal/component/bgp/reactor/session_read.go -- processMessage returns before plugin delivery when prefixDrop is set -->

`count` states which prefixes the number compared against `maximum` holds, and it changes what happens after `teardown false` drops an UPDATE. RFC 4271 Section 6.7 does not say whether a prefix limit governs what the peer offered or what the receiver kept, so the operator chooses. `offered`, the default, keeps a dropped UPDATE's prefixes in the count: the count stays above the maximum, and Ze drops every later announce of that family until the peer withdraws them. `installed` leaves the count where it was, so the family accepts the next announce that fits. Neither value is the size of the RIB, because import policy can reject a counted prefix. The choice never changes enforcement: both values drop the same UPDATE and send the same NOTIFICATION.
<!-- source: internal/component/bgp/reactor/session_prefix.go -- applyInstalledPrefixDeltas settles an installed family before the count moves -->


A peer stopped by a prefix limit STAYS DOWN by default. Its state reads `idle-hold`, `ze show warnings` carries a `prefix-hold` warning that names the family, and the log line says `peer held down`. The peer comes back when an operator recreates it: change that peer's config and commit, or delete and add the peer. This is what Cisco and Juniper do for the same event.

`reconnect backoff` asks for the opposite: the peer comes back on its usual connect backoff, 5 to 60 seconds. `reconnect timer`, or an `idle-timeout` above 0, waits idle-timeout x 2^(N-1), capped at 1 hour. The wait comes from the family that exceeded its maximum, and resets on a stable session. A `reconnect` value that contradicts `idle-timeout` in the same block is a config error.

**PeeringDB integration:** `resolve peeringdb max-prefix <asn>` queries PeeringDB for a peer's ASN and updates prefix maximums automatically. A configurable margin (default 10%) is added to PeeringDB values. The PeeringDB URL is configurable under `system { peeringdb { url; margin; } }` for private mirrors. Staleness warnings appear when prefix data is older than 6 months.

**Prometheus metrics:** `ze_bgp_prefix_count`, `ze_bgp_prefix_maximum`, `ze_bgp_prefix_warning`, `ze_bgp_prefix_warning_exceeded`, `ze_bgp_prefix_ratio`, `ze_bgp_prefix_maximum_exceeded_total`, `ze_bgp_prefix_teardown_total`, `ze_bgp_prefix_stale`.
<!-- source: internal/component/bgp/reactor/session_prefix.go -- prefix limit enforcement -->
<!-- source: internal/component/bgp/reactor/peer_run.go -- prefixReconnectDecision and holdDownAfterPrefixTeardown, per-family reconnect -->
<!-- source: internal/component/resolve/cmd/resolve.go -- handlePeeringDBMaxPrefix -->

### Cross-Peer Update Groups

Peers with identical outbound encoding contexts (same ContextID, same policy) are automatically grouped. The reactor builds each UPDATE once per group and fans out the wire bytes to all members, eliminating redundant per-peer UPDATE construction. GroupKey combines the peer's `sendCtxID` (which encodes ASN4, ADD-PATH, Extended Message, Extended Next Hop, iBGP/eBGP, and ASN values) with a policy key (uniform today, extensible for per-peer export policy).

Groups are maintained by the reactor: peers are added on session establishment and removed on session close. When disabled or when all peers have unique contexts, behavior is identical to per-peer building with negligible overhead (one map lookup per peer lifecycle event).

Default enabled. Configurable via `ze.bgp.reactor.update-groups` (boolean, default true). ExaBGP migrated configs automatically set `update-groups false` to preserve per-peer UPDATE behavior.
<!-- source: internal/component/bgp/reactor/update_group.go -- UpdateGroupIndex, GroupKey, Add, Remove, GroupsForPeers -->
<!-- source: internal/component/bgp/reactor/reactor_notify.go -- updateGroups.Add on established, Remove on closed -->
<!-- source: internal/component/bgp/reactor/reactor_api_batch.go -- group-aware AnnounceNLRIBatch -->
<!-- source: internal/component/bgp/reactor/reactor_api_forward.go -- group-aware ForwardUpdate with fwdBodyCache -->
<!-- source: internal/component/config/environment.go -- ze.bgp.reactor.update-groups env var registration -->
<!-- source: internal/exabgp/migration/migrate.go -- injectUpdateGroupsDisabled -->

### Session Resilience

| Feature | Description |
|---------|-------------|
| TCP_NODELAY | Disables Nagle's algorithm. BGP messages are application-framed; Nagle only adds latency. |
| DSCP CS6 (RFC 4271 S5.1) | Sets IP_TOS/IPV6_TCLASS to 0xC0 so network QoS policies prioritize BGP traffic. |
| Graceful TCP close | Half-close (CloseWrite) before Close sends FIN instead of RST, ensuring remote peers read pending NOTIFICATIONs. |
| Send Hold Timer (RFC 9687) | Detects when local side cannot write to peer. Duration: max(8min, 2x hold-time), or the per-peer `send-hold-time`, which must exceed the hold time. Sends NOTIFICATION code 8 on expiry. Stopped when the negotiated hold time is zero, since such a session sends nothing. |
| Hold timer expiry (RFC 4271 Section 8.2.2, Event 10) | Every expiry sends NOTIFICATION code 4 (Hold Timer Expired) and stops the session. Ze grants no reprieve for CPU congestion. |
| Write deadline | Forward pool batch writes use a 30s TCP write deadline (configurable via `ze.fwd.write.deadline`) to prevent stuck peers from blocking workers. |
| Bounded overflow pool | Two-tier pool: per-peer pools (64 slots) absorb steady-state traffic, shared MixedBufMux overflow pool (auto-sized from peer prefix maximums, overridable via `ze.fwd.pool.size` byte budget) bounds overflow memory. |
| Congestion backpressure | Two-threshold enforcement: pool > 80% denies buffers to the worst destination peer (natural TCP backpressure). Pool > 95% with peer > 2x weight share for 5s triggers forced teardown. |
| GR-aware congestion teardown | Forced teardown is GR-aware: GR peers get TCP close (route retention), non-GR peers get Cease/OutOfResources NOTIFICATION. |
| Pool headroom | `ze.fwd.pool.headroom` adds extra memory beyond auto-sized baseline, trading memory for delayed teardown decisions. |

**Prometheus metrics:** `ze_bgp_pool_used_ratio`, `ze_bgp_overflow_items{peer}`, `ze_bgp_overflow_ratio{source}`, `ze_forward_buffer_denied_total`, `ze_forward_congestion_teardown_total`.
<!-- source: internal/component/bgp/reactor/session_connection.go -- TCP_NODELAY, IP_TOS, closeConn -->
<!-- source: internal/component/bgp/reactor/session_write.go -- Send Hold Timer -->
<!-- source: internal/component/bgp/reactor/session.go -- OnHoldTimerExpires callback -->
<!-- source: internal/component/bgp/reactor/forward_pool.go -- write deadline, overflow pool -->
<!-- source: internal/component/bgp/reactor/forward_pool_congestion.go -- two-threshold enforcement -->

### Route Loop Detection

| Check | RFC | Scope | What it detects |
|-------|-----|-------|-----------------|
| AS loop | RFC 4271 Section 9 | All sessions | Local ASN in received AS_PATH (AS_SEQUENCE or AS_SET) |
| ORIGINATOR_ID loop | RFC 4456 Section 8 | iBGP only | ORIGINATOR_ID matches local Router ID |
| CLUSTER_LIST loop | RFC 4456 Section 8 | iBGP only | Local Router ID found in CLUSTER_LIST |

All three checks run in one ingress filter at `FilterStageProtocol`, so they come
after RFC 7606 structural validation and after prefix limit counting, which both
run on the session read path before the UPDATE reaches the filter pipeline.
A route failing any check is dropped silently (no NOTIFICATION, session stays up).
Cluster ID defaults to Router ID per RFC 4456 Section 7.

<!-- source: internal/component/bgp/reactor/filter/loop.go -- LoopIngress -->
<!-- source: internal/component/bgp/filterapi/filterapi.go -- FilterStageProtocol -->
<!-- source: internal/component/bgp/reactor/session_validation.go -- enforceRFC7606; internal/component/bgp/reactor/session_prefix.go -- checkPrefixLimits -->

### Capabilities Configuration

| Capability | Config Key | Values |
|------------|-----------|--------|
| 4-byte ASN | `asn4` | true / false |
| Route Refresh | `route-refresh` | true / false |
| ADD-PATH | `add-path` | Per-family send/receive/both |
| Extended Message | `extended-message` | true / false |
| Extended Next Hop | `nexthop` | Per-family AFI mapping |
| Graceful Restart | `graceful-restart` | restart-time (0-4095s), long-lived-stale-time (0-16777215s) |
| Role | `role` | provider / rs / rs-client / customer / peer |
| Role Strict | `role/strict` | true / false |

### Route Configuration

Static routes configured per-peer with full attribute control:

- Per-family NLRI with add/del/eor operations
- All standard path attributes
- Watchdog-controlled deferred announcement
- MPLS labels (single and multi-label)
- Route Distinguisher for VPN routes
- Prefix-SID and SRv6 attributes

### Process Bindings

External processes receive BGP events and send commands:

- JSON event encoding (peer-up, peer-down, route updates)
- Text command protocol (route announce/withdraw)
- Configurable message filtering (receive-update, receive-open, etc.)
- Neighbor change notifications

## Dependency Graph

`ze config graph <file>` exposes configuration groups, peers, plugin dependencies, and their relationships as machine-readable nodes and edges. Use it to identify which peers inherit a shared value before changing that group.

Plugin dependency expansion keys on the REGISTERED plugin name, not on the
operator's list label. `plugin { internal rs { use bgp-rs } }` names the instance
`rs` and runs the registered plugin `bgp-rs`, and the dependencies `bgp-rs`
declares are pulled in. Keying on the label made the resolver treat the plugin as
external and skip expansion, so both hard and optional dependencies were dropped
with no error: a route server configured that way ran with no `bgp-adj-rib-in`
and therefore no peer-up Adj-RIB-In replay.
<!-- source: internal/component/config/loader.go -- registryName, ExpandDependencies -->

A config root that no plugin and no hub handler claims is stored and delivered to
nobody, so it has no effect. `ze doctor` reports it as
`doctor-config-root-unclaimed`, naming the path and the two causes: the owning
plugin is not in this binary or did not load, or its config root is missing. The
check fails closed and reports `doctor-config-claims-unavailable` when no plugin
in the build declares a config root at all.
<!-- source: internal/component/doctor/checks_config_claims.go -- checkConfigClaims, configClaimDiagnostics -->

### Demo: Find every peer affected by a group change

Inspect and validate a BGP group, then use Ze's dependency graph to prove which peers inherit the value before scheduling maintenance.

[Download the asciicast recording](../../assets/demos/config-graph.cast?v=1ea1811466) · [Plain-text transcript](../../assets/demos/config-graph.txt?v=7a64ac5a0c)

Recorded with Ze 26.08.31 on macOS and Linux using Ze recorder. Duration: 1 minute 10 seconds.

```console
An operator needs to change the transit group's remote ASN and identify every peer that inherits it before scheduling maintenance.

$ ze config show router.conf bgp group transit
The scoped configuration shows `upstream-a` and `upstream-b` inside the transit group.

$ ze config validate router.conf
configuration valid

$ ze config graph router.conf | ze pipe text | ze pipe match peer/upstream
$ ze config graph router.conf | ze pipe text | ze pipe match group/transit
$ ze config graph router.conf | ze pipe text | ze pipe match inherits
The graph answer holds two lists, `nodes` and `edges`. A row operator such as `match` has no single set of rows there, so Ze refuses it by name instead of picking one list. `| text` renders both lists as aligned rows, one relationship to a line. `| match` then keeps the lines that name the two peers, the group they share, and the two `inherits` relationships.

No reporting helper creates the displayed relationships. The command filters Ze's graph output directly through Ze's format pipeline.
```

