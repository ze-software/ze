# Configuration

### Peer Settings

Peers are keyed by name (`peer <name> { }`) with IP and AS in nested containers:

| Setting | Description | Validation |
|---------|-------------|------------|
| `remote { ip; as; }` | Peer IP and AS number | Required |
| `local { ip; as; }` | Local bind address and AS | Required (ip can be `auto`) |
| `router-id` | Per-peer router ID override | Optional (or inherited) |
| `name` | Peer key (must start with letter) | Required |
| `hold-time` | Hold timer (0 or 3-65535 seconds) | 1-2 rejected |
| `local { connect; }` | Initiate outbound connections (boolean) | Default: true |
| `remote { accept; }` | Accept inbound connections (boolean) | Default: true |
| `md5-password` | TCP MD5 authentication | Optional |
| `outgoing-ttl` | TTL for outgoing packets | Optional |
| `ttl-security` | Minimum TTL for incoming packets | Optional |
| `group-updates` | Enable/disable UPDATE grouping | Default: enabled |

<!-- source: internal/component/bgp/config/resolve.go -- ResolveBGPTree config resolution -->
<!-- source: internal/component/bgp/config/peers.go -- peer config parsing -->
<!-- source: internal/component/bgp/schema/ze-bgp-conf.yang -- BGP config YANG schema -->

### Required Field Validation

The `ze:required` and `ze:suggest` YANG extensions declare which fields must be present in a peer after config inheritance resolution (bgp -> group -> peer merge).

| Extension | Behavior |
|-----------|----------|
| `ze:required` | Field must have a value after inheritance. Validated at `ze config validate`, editor commit, and daemon startup. |
| `ze:suggest` | Field shown in the web creation form with inherited defaults but not mandatory. |

Peer required fields: `connection/remote/ip`, `session/asn/local`, `session/asn/remote`. Suggested: `connection/local/ip`.

Fields can be satisfied by inheritance: `session/asn/local` set at bgp level satisfies the requirement for all peers; `session/asn/remote` set at group level satisfies it for group members.

<!-- source: internal/component/config/yang/modules/ze-extensions.yang -- ze:required, ze:suggest extensions -->
<!-- source: internal/component/bgp/config/resolve.go -- CheckRequiredFields -->

### Prefix Limits (RFC 4486)

Per-peer per-family prefix maximum enforcement. Mandatory for every negotiated family.

| Setting | Scope | Description |
|---------|-------|-------------|
| `prefix { maximum N; }` | Per family | Hard maximum prefix count. Mandatory. |
| `prefix { warning N; }` | Per family | Warning threshold. Default: 90% of maximum. |
| `prefix { teardown true/false; }` | Per peer | Tear down on exceed (default: true) or warn-only. |
| `prefix { idle-timeout N; }` | Per peer | Seconds before auto-reconnect after teardown (0 = no reconnect). |

When a peer exceeds the maximum: NOTIFICATION Cease/MaxPrefixes (subcode 1) is sent and the session is torn down. With `teardown false`, the session stays up but further NLRIs for the exceeded family are dropped.

Auto-reconnect uses exponential backoff: idle-timeout x 2^(N-1), capped at 1 hour. Backoff resets on stable session.

**PeeringDB integration:** `ze update bgp peer * prefix` queries PeeringDB for each peer's ASN and updates prefix maximums automatically. A configurable margin (default 10%) is added to PeeringDB values. The PeeringDB URL is configurable under `system { peeringdb { url; margin; } }` for private mirrors. Staleness warnings appear when prefix data is older than 6 months.

**Prometheus metrics:** `ze_bgp_prefix_count`, `ze_bgp_prefix_maximum`, `ze_bgp_prefix_warning`, `ze_bgp_prefix_warning_exceeded`, `ze_bgp_prefix_ratio`, `ze_bgp_prefix_maximum_exceeded_total`, `ze_bgp_prefix_teardown_total`, `ze_bgp_prefix_stale`.
<!-- source: internal/component/bgp/reactor/session_prefix.go -- prefix limit enforcement -->
<!-- source: internal/component/bgp/reactor/peer.go -- idle-timeout and reconnect logic -->
<!-- source: internal/component/bgp/plugins/cmd/peer/prefix_update.go -- PeeringDB update command -->

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
| Send Hold Timer (RFC 9687) | Detects when local side cannot write to peer. Duration: max(8min, 2x hold-time). Sends NOTIFICATION code 8 on expiry. |
| Hold timer congestion extension | If data was recently read when the hold timer fires, ze is CPU-congested, not the peer. Resets hold timer instead of tearing down. |
| Write deadline | Forward pool batch writes use a 30s TCP write deadline (configurable via `ze.fwd.write.deadline`) to prevent stuck peers from blocking workers. |
| Bounded overflow pool | Global token pool (default: 100,000, configurable via `ze.fwd.pool.size`) bounds overflow memory across all forward workers. Falls back to unbounded on exhaustion. |
| Congestion backpressure | Two-threshold enforcement: pool > 80% denies buffers to the worst destination peer (natural TCP backpressure). Pool > 95% with peer > 2x weight share for 5s triggers forced teardown. |
| GR-aware congestion teardown | Forced teardown is GR-aware: GR peers get TCP close (route retention), non-GR peers get Cease/OutOfResources NOTIFICATION. |
| Pool headroom | `ze.fwd.pool.headroom` adds extra memory beyond auto-sized baseline, trading memory for delayed teardown decisions. |

**Prometheus metrics:** `ze_bgp_pool_used_ratio`, `ze_bgp_overflow_items{peer}`, `ze_bgp_overflow_ratio{source}`, `ze_forward_buffer_denied_total`, `ze_forward_congestion_teardown_total`.
<!-- source: internal/component/bgp/reactor/session_connection.go -- TCP_NODELAY, IP_TOS, closeConn -->
<!-- source: internal/component/bgp/reactor/session_write.go -- Send Hold Timer -->
<!-- source: internal/component/bgp/reactor/session.go -- recentRead congestion extension -->
<!-- source: internal/component/bgp/reactor/forward_pool.go -- write deadline, overflow pool -->
<!-- source: internal/component/bgp/reactor/forward_pool_congestion.go -- two-threshold enforcement -->

### Route Loop Detection

| Check | RFC | Scope | What it detects |
|-------|-----|-------|-----------------|
| AS loop | RFC 4271 Section 9 | All sessions | Local ASN in received AS_PATH (AS_SEQUENCE or AS_SET) |
| ORIGINATOR_ID loop | RFC 4456 Section 8 | iBGP only | ORIGINATOR_ID matches local Router ID |
| CLUSTER_LIST loop | RFC 4456 Section 8 | iBGP only | Local Router ID found in CLUSTER_LIST |

All three checks run after RFC 7606 structural validation but before prefix limit counting.
Routes failing any check are silently treated as withdrawn (no NOTIFICATION, session stays up).
Cluster ID defaults to Router ID per RFC 4456 Section 7.

<!-- source: internal/component/bgp/reactor/session_validation.go -- detectLoops -->

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
| Role Strict | `role-strict` | true / false |

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
