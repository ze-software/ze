# Ze Features

Ze is a BGP daemon written in Go. This document lists all user-facing features.

## BGP Protocol

### Address Families

| Family | Config Name | AFI/SAFI | Encode | Decode | Route Config |
|--------|-------------|----------|--------|--------|--------------|
| IPv4 Unicast | `ipv4/unicast` | 1/1 | Yes | Yes | Yes |
| IPv6 Unicast | `ipv6/unicast` | 2/1 | Yes | Yes | Yes |
| IPv4 Multicast | `ipv4/multicast` | 1/2 | Yes | Yes | Yes |
| IPv6 Multicast | `ipv6/multicast` | 2/2 | Yes | Yes | Yes |
| IPv4 VPN | `ipv4/vpn` | 1/128 | Yes | Yes | Yes |
| IPv6 VPN | `ipv6/vpn` | 2/128 | Yes | Yes | Yes |
| IPv4 FlowSpec | `ipv4/flow` | 1/133 | Yes | Yes | Yes |
| IPv6 FlowSpec | `ipv6/flow` | 2/133 | Yes | Yes | Yes |
| IPv4 FlowSpec VPN | `ipv4/flow-vpn` | 1/134 | Yes | Yes | Yes |
| IPv6 FlowSpec VPN | `ipv6/flow-vpn` | 2/134 | Yes | Yes | Yes |
| IPv4 MPLS Label | `ipv4/mpls-label` | 1/4 | Yes | Yes | Yes |
| IPv6 MPLS Label | `ipv6/mpls-label` | 2/4 | Yes | Yes | Yes |
| L2VPN EVPN | `l2vpn/evpn` | 25/70 | Yes | Yes | Yes |
| L2VPN VPLS | `l2vpn/vpls` | 25/65 | Yes | Yes | Yes |
| BGP-LS | `bgp-ls/bgp-ls` | 16/71 | No | Yes | No |
| BGP-LS VPN | `bgp-ls/bgp-ls-vpn` | 16/72 | No | Yes | No |
| IPv4 MVPN | `ipv4/mvpn` | 1/5 | No | Yes | No |
| IPv6 MVPN | `ipv6/mvpn` | 2/5 | No | Yes | No |
| IPv4 RTC | `ipv4/rtc` | 1/132 | No | Yes | No |
| IPv4 MUP | `ipv4/mup` | 1/85 | Yes | Yes | Yes |
| IPv6 MUP | `ipv6/mup` | 2/85 | Yes | Yes | Yes |

<!-- source: internal/component/bgp/plugins/nlri/evpn/register.go -- EVPN family registration -->
<!-- source: internal/component/bgp/plugins/nlri/flowspec/register.go -- FlowSpec family registration -->
<!-- source: internal/component/bgp/plugins/nlri/vpn/register.go -- VPN family registration -->
<!-- source: internal/component/bgp/plugins/nlri/mup/register.go -- MUP family registration -->
<!-- source: internal/component/bgp/plugins/nlri/ls/register.go -- BGP-LS family registration -->
<!-- source: internal/component/bgp/plugins/nlri/labeled/register.go -- MPLS label family registration -->
<!-- source: internal/component/bgp/plugins/nlri/vpls/register.go -- VPLS family registration -->
<!-- source: internal/component/bgp/plugins/nlri/mvpn/register.go -- MVPN family registration -->
<!-- source: internal/component/bgp/plugins/nlri/rtc/register.go -- RTC family registration -->

### Capabilities

| Capability | Code | RFC | Description |
|------------|------|-----|-------------|
| Multiprotocol Extensions | 1 | RFC 4760 | Multi-protocol BGP (AFI/SAFI negotiation) |
| 4-byte ASN | 65 | RFC 6793 | 32-bit AS numbers |
| Route Refresh | 2 | RFC 2918 | Request full route re-advertisement |
| Enhanced Route Refresh | 70 | RFC 7313 | Bounded clear and re-send |
| ADD-PATH | 69 | RFC 7911 | Multiple paths per prefix |
| Extended Message | 6 | RFC 8654 | 65535-byte messages |
| Extended Next Hop | 5 | RFC 8950 | IPv6 next-hop for IPv4 NLRI |
| Graceful Restart | 64 | RFC 4724 | Session preservation across restarts (Restarting Speaker: R-bit via zefs marker on `ze signal restart`) |
| Long-Lived GR | 71 | RFC 9494 | Extended stale route retention with LLGR_STALE community and depreference |
| BGP Role | 9 | RFC 9234 | Peer relationship role |
| Hostname | 73 | draft | FQDN capability |
| Software Version | 75 | draft | Software version advertisement |
| Link-Local Next Hop | 77 | — | IPv6 link-local as next-hop |

<!-- source: internal/component/bgp/capability/capability.go -- capability code constants -->
<!-- source: internal/component/bgp/capability/encoding.go -- ASN4, AddPath, ExtMsg, ExtNH -->
<!-- source: internal/component/bgp/capability/session.go -- GR, RouteRefresh, Role -->
<!-- source: internal/component/bgp/plugins/hostname/register.go -- Hostname capability plugin -->
<!-- source: internal/component/bgp/plugins/softver/register.go -- Software Version capability plugin -->
<!-- source: internal/component/bgp/plugins/llnh/register.go -- Link-Local NH capability plugin -->

### Path Attributes

| Attribute | Code | JSON Key | Description |
|-----------|------|----------|-------------|
| ORIGIN | 1 | `origin` | igp / egp / incomplete |
| AS_PATH | 2 | `as-path` | AS path segments |
| NEXT_HOP | 3 | `next-hop` | Next hop IP address |
| MED | 4 | `med` | Multi-Exit Discriminator |
| LOCAL_PREF | 5 | `local-preference` | Local preference |
| ATOMIC_AGGREGATE | 6 | `atomic-aggregate` | Atomic aggregate flag |
| AGGREGATOR | 7 | `aggregator` | Aggregator ASN:IP |
| COMMUNITY | 8 | `community` | Standard communities |
| ORIGINATOR_ID | 9 | `originator-id` | Route reflector originator |
| CLUSTER_LIST | 10 | `cluster-list` | Route reflector cluster list |
| MP_REACH_NLRI | 14 | — | Multiprotocol reachable NLRI |
| MP_UNREACH_NLRI | 15 | — | Multiprotocol unreachable NLRI |
| EXTENDED_COMMUNITY | 16 | `extended-community` | Extended communities |
| LARGE_COMMUNITY | 32 | `large-community` | Large communities (RFC 8092) |
| PREFIX_SID | 40 | `prefix-sid` | Segment Routing prefix SID |

<!-- source: internal/component/bgp/attribute/attribute.go -- attribute code constants -->
<!-- source: internal/component/bgp/attribute/origin.go -- ORIGIN -->
<!-- source: internal/component/bgp/attribute/aspath.go -- AS_PATH -->
<!-- source: internal/component/bgp/attribute/community.go -- COMMUNITY, EXT_COMMUNITY, LARGE_COMMUNITY -->

## Configuration

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

### Templates

Peer templates allow shared configuration with pattern-based matching:

- Named templates with `inherit-name`
- Pattern-based peer matching
- Field inheritance (capabilities, families, routes, processes)

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

## Plugins

### Storage & Policy

| Plugin | Description |
|--------|-------------|
| bgp-rib | Route Information Base -- stores received/sent routes |
| bgp-adj-rib-in | Adj-RIB-In -- raw hex replay of received routes |
| bgp-persist | Route persistence across restarts |
| bgp-rs | Route server -- client-to-client route reflection (RFC 7947) |
| bgp-watchdog | Deferred route announcement with named watchdog groups |

### Redistribution Filters (planned)

External plugins can act as route filters on import and export. Filters are
configured per peer/group/globally via `redistribution { import [...] export [...] }`
using `<plugin>:<filter>` references. Multiple filters chain as piped transforms
(each sees previous filter's output). Filters respond accept/reject/modify with
delta-only attribute changes and dirty tracking for efficient re-encoding.

Three filter categories:

| Category | Behavior | Example |
|----------|----------|---------|
| Mandatory | Always on, cannot be overridden | `rfc:otc` (RFC 9234) |
| Default | On by default, can be overridden per-peer | `rfc:no-self-as` (loop prevention) |
| User | Only present when explicitly configured | `rpki:validate`, `community:scrub` |

<!-- source: plan/spec-redistribution-filter.md -- redistribution filter design -->

<!-- source: internal/component/bgp/plugins/rib/register.go -- bgp-rib -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/register.go -- bgp-adj-rib-in -->
<!-- source: internal/component/bgp/plugins/persist/register.go -- bgp-persist -->
<!-- source: internal/component/bgp/plugins/rs/register.go -- bgp-rs -->
<!-- source: internal/component/bgp/plugins/watchdog/register.go -- bgp-watchdog -->

### Protocol

| Plugin | Description |
|--------|-------------|
| bgp-gr | Graceful Restart (RFC 4724) and Long-Lived GR (RFC 9494) state machine |
| bgp-aigp | Accumulated IGP Metric (RFC 7311) |
| bgp-rpki | RPKI origin validation via RTR protocol (RFC 6811, RFC 8210). [Guide](guide/rpki.md) |
| bgp-rpki-decorator | Correlates UPDATE + RPKI events into merged update-rpki events |
| bgp-route-refresh | Route Refresh handling (RFC 2918, RFC 7313) |
| role | BGP Role capability enforcement (RFC 9234) |
| bgp-llnh | Link-local next-hop for IPv6 (RFC 2545) |
| bgp-hostname | FQDN capability for peer identification |
| bgp-softver | Software version capability advertisement |
| filter-community | Community tag/strip filter (standard, large, extended) |
| loop | Route loop detection (RFC 4271 S9, RFC 4456 S8) |

<!-- source: internal/component/bgp/plugins/gr/register.go -- bgp-gr -->
<!-- source: internal/component/bgp/plugins/rpki/register.go -- bgp-rpki -->
<!-- source: internal/component/bgp/plugins/rpki_decorator/register.go -- bgp-rpki-decorator -->
<!-- source: internal/component/bgp/plugins/route_refresh/register.go -- bgp-route-refresh -->
<!-- source: internal/component/bgp/plugins/role/register.go -- role -->
<!-- source: internal/component/bgp/plugins/llnh/register.go -- bgp-llnh -->
<!-- source: internal/component/bgp/plugins/hostname/register.go -- bgp-hostname -->
<!-- source: internal/component/bgp/plugins/softver/register.go -- bgp-softver -->
<!-- source: internal/component/bgp/plugins/aigp/register.go -- bgp-aigp -->
<!-- source: internal/component/bgp/plugins/filter_community/register.go -- filter-community -->
<!-- source: internal/component/bgp/reactor/filter/register.go -- loop -->

## CLI Commands

### Protocol Tools

| Command | Description |
|---------|-------------|
| `ze bgp decode` | Decode BGP message from hex to JSON |
| `ze bgp encode` | Encode text route command to BGP wire hex |

<!-- source: cmd/ze/bgp/main.go -- bgp decode/encode dispatch -->

### Configuration Management

| Command | Description |
|---------|-------------|
| `ze config validate <file>` | Validate configuration file |
| `ze config edit` | Interactive configuration editor |
| `ze config migrate` | Convert ExaBGP config to ze format |
| `ze config fmt` | Format and normalize config file |
| `ze config dump` | Dump parsed configuration tree |
| `ze config diff <a> <b>` | Compare two configuration files |
| `ze config set` | Set a configuration value programmatically |
| `ze config import` | Import a configuration file into ze |
| `ze config rename` | Rename a configuration element |
| `ze config archive <name>` | Archive config to a named destination ([guide](guide/config-archive.md)) |
| `ze config history` | List rollback revisions |
| `ze config rollback <N>` | Restore revision N |

<!-- source: cmd/ze/config/main.go -- config subcommand dispatch -->
<!-- source: cmd/ze/config/cmd_archive.go -- archive subcommand -->
<!-- source: cmd/ze/config/cmd_validate.go -- validate command -->
<!-- source: cmd/ze/config/cmd_migrate.go -- migrate command -->
<!-- source: cmd/ze/config/cmd_dump.go -- dump command -->
<!-- source: cmd/ze/config/cmd_diff.go -- diff command -->

### Schema Discovery

| Command | Description |
|---------|-------------|
| `ze schema list` | List all registered YANG schemas |
| `ze schema show <module>` | Show YANG content for a module |
| `ze schema handlers` | List handler→module mapping |
| `ze schema methods [module]` | List RPCs from YANG modules |
| `ze schema events` | List notifications from YANG |
| `ze schema protocol` | Show protocol version and format info |

<!-- source: cmd/ze/yang/main.go -- schema subcommand dispatch -->

### Daemon Control

| Command | Description |
|---------|-------------|
| `ze <config-file>` | Start daemon with configuration |
| `ze signal reload` | Send SIGHUP — reload configuration |
| `ze signal stop` | Graceful shutdown (no GR marker) |
| `ze signal restart` | Graceful restart (writes GR marker, then shuts down) |
| `ze signal quit` | Send SIGQUIT — goroutine dump + exit |
| `ze status` | Check if daemon is running |

<!-- source: cmd/ze/signal/main.go -- signal subcommand dispatch -->

### Runtime Interaction

| Command | Description |
|---------|-------------|
| `ze cli` | Interactive CLI (with `--run <cmd>` for single command) |
| `ze show <command>` | Read-only daemon commands |
| `ze run <command>` | All daemon commands |

**Live peer dashboard:** `monitor bgp` in the interactive CLI enters a live dashboard showing router identity, a sortable color-coded peer table with update rates, and drill-down detail view. Auto-refreshes every 2 seconds. Navigate with j/k, sort with s/S, Enter for detail, Esc to exit.
<!-- source: internal/component/cli/model_dashboard.go -- isDashboardCommand -->

**Commit confirmed:** The editor supports `commit confirmed <seconds>` for safe remote changes. The config is applied immediately but auto-reverts if `confirm` is not issued within the timeout window (1-3600 seconds). Use `abort` to revert manually. Modeled after Junos/VyOS commit confirmed.
<!-- source: internal/component/cli/model_load.go -- cmdCommitConfirmed -->

**Command history persistence:** Both `ze config edit` and `ze cli` persist command history to the zefs blob store. History survives application restarts, is stored per-mode (edit vs command) and per-user, with consecutive dedup and a configurable rolling window (default 100, max 10000). Graceful degradation when no blob store is available (in-memory only).
<!-- source: internal/component/cli/history.go -- History type -->

**Login warnings:** When an operator connects via SSH, ze checks for conditions requiring attention and displays warnings in the welcome area. Each warning includes a message and an actionable command. Currently checks for stale prefix data (peers with `prefix-updated` older than 6 months).
<!-- source: internal/component/ssh/session.go -- createSessionModel login warning collection -->

### Other

| Command | Description |
|---------|-------------|
| `ze plugin <name>` | Run a registered plugin |
| `ze exabgp plugin` | Run ExaBGP plugin with ze bridge |
| `ze exabgp migrate` | Convert ExaBGP config to ze |
| `ze completion bash/zsh/fish/nushell` | Generate shell completion scripts |
| `ze --plugins` | List available internal plugins |

<!-- source: cmd/ze/completion/main.go -- completion subcommand -->
<!-- source: cmd/ze/plugin/main.go -- plugin subcommand -->
<!-- source: cmd/ze/exabgp/main.go -- exabgp subcommand -->

## API Commands

Commands sent through `ze cli`, `ze run`, `ze show`, or process stdin.

### Peer Management

| Command | Description |
|---------|-------------|
| `bgp peer * list` | List peers (brief) |
| `bgp peer * show` | Show peer details and statistics |
| `bgp peer <addr> teardown <code>` | Graceful session closure with NOTIFICATION |
| `set bgp peer <name> with <config>` | Dynamic peer creation |
| `del bgp peer <name>` | Remove peer |
| `bgp peer <addr> pause` | Pause reading from peer (flow control) |
| `bgp peer <addr> resume` | Resume reading from peer |
| `bgp peer <addr> capabilities` | Show negotiated capabilities |
| `bgp summary` | BGP summary table with statistics |

Peer selector supports: `*` (all), exact IP, peer name, ASN (`as65001`), glob patterns (`192.168.*.*`), exclusion (`!addr`, `!as65001`). Tab completion for peer selectors in `ze show` and `ze run` when daemon is running.
<!-- source: internal/component/bgp/plugins/cmd/peer/peer.go -- peer management RPC handlers -->

### Route Updates

| Command | Description |
|---------|-------------|
| `bgp peer * update text <attrs> nlri <family> <op> <prefix>` | Text-format UPDATE |
| `bgp peer * update hex <hex>` | Hex-format UPDATE |

Text attribute syntax: `origin set igp`, `nhop set 1.1.1.1`, `local-preference set 100`, `med set 50`, `as-path set [65000 65001]`, `community set [no-export]`, `large-community set [65000:1:1]`.

NLRI operations: `add`, `del`, `eor` per address family.
<!-- source: internal/component/bgp/plugins/cmd/update/update_text_test.go -- text update parsing -->
<!-- source: internal/component/bgp/attribute/builder_parse.go -- text attribute parsing -->

### RIB Operations

| Command | Description |
|---------|-------------|
| `rib routes received [peer] [family]` | Show Adj-RIB-In |
| `rib routes sent [peer] [family]` | Show Adj-RIB-Out |
| `rib clear-in [peer] [family]` | Clear Adj-RIB-In |
| `rib clear-out [peer] [family]` | Clear Adj-RIB-Out |

<!-- source: internal/component/bgp/plugins/cmd/rib/ -- RIB command handlers -->

### Cache Management

| Command | Description |
|---------|-------------|
| `cache list` | List cached messages |
| `cache retain` | Retain message in cache |
| `cache release` | Release from cache |
| `cache expire` | Set cache expiration |
| `cache forward` | Forward cached message to peer(s) |

### Event Subscription

| Command | Description |
|---------|-------------|
| `subscribe <filter>` | Subscribe to BGP events |
| `unsubscribe <id>` | Unsubscribe from events |

### Commit Workflow

Named update windows for atomic route changes:

| Command | Description |
|---------|-------------|
| `commit start <name>` | Begin named update window |
| `commit end <name>` | End window and send updates |
| `commit eor <name>` | Send End-of-RIB for window |
| `commit rollback <name>` | Discard changes |
| `commit show <name>` | Show commit status |
| `commit withdraw <name>` | Withdraw all routes in window |
| `commit list` | List named commits |

<!-- source: internal/component/cmd/commit/commit.go -- commit workflow handlers -->

### Raw & Introspection

| Command | Description |
|---------|-------------|
| `bgp peer * raw <hex>` | Send raw BGP message bytes |
| `route-refresh <family>` | Send route refresh request |
| `help` | Show available commands |
| `command-list` | List all commands with descriptions |
| `command-help <name>` | Detailed help for command |

<!-- source: internal/component/bgp/plugins/cmd/raw/ -- raw BGP message handler -->

## Configuration Reload

Ze supports live configuration reload via SIGHUP or `ze signal reload`:

- Add/remove peers without restart
- Update peer settings with automatic reconciliation
- Graceful failure on invalid config (keeps running)
- Rapid successive reloads handled correctly

## Fleet Management

Ze supports centralized configuration for multi-node deployments. A central hub
serves configuration to remote ze instances over TLS.

- Named hub blocks: `server <name> { host; port; secret; }` for listeners, `client <name> { host; port; secret; }` for outbound
- Per-client secrets: each managed client authenticates with its own token
- Config fetch with version hashing: clients only download when config changes
- Two-phase config change: hub notifies, client fetches when ready
- Partition resilience: clients cache config locally and start from cache when hub is unreachable
- Exponential backoff reconnect with jitter (1s to 60s cap)
- Heartbeat liveness detection (30s interval, 90s timeout)
- CLI overrides: `--server`, `--name`, `--token` flags for troubleshooting
- Managed mode toggle: `meta/instance/managed` blob flag controls hub connection

<!-- source: internal/component/managed/client.go -- RunManagedClient lifecycle -->
<!-- source: internal/component/plugin/server/managed.go -- hub-side config handlers -->
<!-- source: pkg/fleet/ -- version hash and RPC envelope types -->

## Performance Benchmarking

Ze includes `ze-perf`, a standalone BGP propagation latency benchmark tool. It
measures route forwarding performance through a device under test (DUT) by
establishing sender and receiver BGP sessions, injecting routes, and timing
their propagation.

<!-- source: cmd/ze-perf/main.go -- ze-perf CLI entry point -->

| Feature | Description |
|---------|-------------|
| Cross-implementation comparison | Docker runner for Ze, FRR, BIRD, GoBGP, rustbgpd (or any BGP speaker) |
| Multi-iteration with statistics | Median/stddev from repeated runs, outlier removal |
| Three encoding modes | IPv4 inline NLRI, IPv4 force-MP (MP_REACH_NLRI), IPv6 MP |
| Comparison reports | Markdown and HTML side-by-side reports from result files |
| History tracking | NDJSON history files with stddev-aware regression detection |
| CI integration | `--check` flag exits non-zero on performance regression |

<!-- source: internal/perf/metrics.go -- aggregation and outlier removal -->
<!-- source: internal/perf/regression.go -- regression detection -->

See [Benchmarking Guide](guide/benchmarking.md) for usage instructions.

## Web Interface

Ze includes an HTTPS web interface for configuration viewing, editing, and runtime command execution through a browser.

| Feature | Description |
|---------|-------------|
| YANG-driven UI | Config tree navigation generated from YANG schemas |
| Finder navigation | macOS-style column browser; named containers above unnamed with separator |
| List table view | Lists with YANG `unique` constraints shown as interactive tables with inline editing |
| Config viewing | Browse the config tree with breadcrumb navigation |
| Config editing | Set and delete leaf values with per-user draft sessions |
| Inline diff | Review pending changes before committing |
| Session authentication | Login page with session cookies; same user database as SSH |
| JSON API | Content negotiation via `Accept` header or `?format=json` query parameter; Basic Auth for API clients |
| CLI bar | Integrated command bar with the same grammar as the SSH CLI (edit, set, delete, show, commit, discard) |
| Terminal mode | Full terminal mode in the browser with scrollback and prompt |
| Tab completion | Autocomplete candidates served via JSON endpoint |
| Live updates | SSE notifications when another user commits config changes |
| HTTPS only | TLS 1.2 minimum; auto-generated ECDSA P-256 self-signed certificate when no cert is provided |
| Security headers | HSTS, CSP, X-Frame-Options DENY, no-store cache on all authenticated responses |
| YANG decorators | Leaves with `ze:decorate` extension show enriched display text (e.g., ASN numbers annotated with organization name via Team Cymru DNS) |

<!-- source: internal/component/web/server.go -- WebServer, TLS config, cert generation -->
<!-- source: internal/component/web/decorator.go -- Decorator registry and interface -->
<!-- source: internal/component/web/decorator_asn.go -- ASN name decorator via Team Cymru DNS -->
<!-- source: internal/component/web/auth.go -- SessionStore, AuthMiddleware, LoginHandler -->
<!-- source: internal/component/web/handler.go -- URL routing, content negotiation, three-tier scheme -->
<!-- source: internal/component/web/handler_config.go -- Config view and edit handlers -->
<!-- source: internal/component/web/handler_admin.go -- Admin command handlers -->
<!-- source: internal/component/web/cli.go -- CLI bar and terminal mode -->
<!-- source: internal/component/web/sse.go -- EventBroker SSE broadcast -->
<!-- source: internal/component/web/editor.go -- EditorManager per-user sessions -->

See [Web Interface Guide](guide/web-interface.md) for usage instructions.

## MCP Integration

<!-- source: internal/component/mcp/handler.go -- MCP HTTP handler -->
<!-- source: cmd/ze-test/mcp.go -- MCP test client -->

Ze includes an MCP (Model Context Protocol) server for AI-assisted BGP operations. The server runs inside the daemon and exposes typed tools for route management and peer control.

| Feature | Description |
|---------|-------------|
| Route announcement | `ze_announce` tool with typed parameters (origin, next-hop, communities, prefixes) |
| Route withdrawal | `ze_withdraw` tool |
| Peer monitoring | `ze_peers` tool shows state, ASN, uptime |
| Peer control | `ze_peer_control` for teardown, pause, resume, flush |
| Generic commands | `ze_execute` runs any CLI command via MCP |
| AI reference | `ze help --ai` generates machine-readable command reference from code |
| Testing | `ze-test mcp` client for functional tests with `wait-established` synchronization |

Start with `ze start --mcp <port>` or `ze --mcp <port> config.conf`. Binds to 127.0.0.1 only.

See [MCP Guide](guide/mcp/overview.md) for details and [MCP Remote Access](guide/mcp/remote-access.md) for tunneling.

## DNS Resolver

<!-- source: internal/component/dns/resolver.go -- miekg/dns resolver with cache -->
<!-- source: internal/component/dns/cache.go -- O(1) LRU cache with TTL -->
<!-- source: internal/component/dns/schema/ze-dns-conf.yang -- YANG config schema -->

Built-in DNS resolver component providing cached DNS queries to all Ze components.
Uses `github.com/miekg/dns` (the library CoreDNS is built on).

| Feature | Description |
|---------|-------------|
| Configurable server | YANG `environment/dns/server` sets upstream DNS |
| Query types | A, AAAA, TXT, PTR, CNAME, MX, NS, SRV |
| LRU cache | O(1) operations, configurable size and max TTL |
| TTL-aware | Respects response TTL, caps at configured maximum, honors TTL=0 (do not cache) |
| Concurrent safe | Mutex-protected cache, safe for multi-goroutine use |
| System fallback | Empty server config uses `/etc/resolv.conf` (resolved once at startup) |
| Timeout control | Per-resolver configurable timeout (1-60 seconds) |
| Env override | `ze.dns.server` overrides config file DNS server |

Configured under `environment { dns { } }` in the config file.

## ExaBGP Compatibility

<!-- source: cmd/ze/exabgp/main.go -- ze exabgp subcommands -->
<!-- source: internal/exabgp/migration/migrate.go -- ExaBGP config migration -->

- Automatic detection and migration of ExaBGP configuration files
- `ze exabgp plugin` runs ExaBGP processes with ze as the BGP engine
- `ze exabgp migrate` converts ExaBGP configs to ze format
