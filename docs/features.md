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
| IPv4 VPN | `ipv4/mpls-vpn` | 1/128 | Yes | Yes | Yes |
| IPv6 VPN | `ipv6/mpls-vpn` | 2/128 | Yes | Yes | Yes |
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
| 4-byte ASN | 65 | RFC 6793 | 32-bit AS numbers |
| Route Refresh | 2 | RFC 2918 | Request full route re-advertisement |
| Enhanced Route Refresh | 70 | RFC 7313 | Bounded clear and re-send |
| ADD-PATH | 69 | RFC 7911 | Multiple paths per prefix |
| Extended Message | 6 | RFC 8654 | 65535-byte messages |
| Extended Next Hop | 5 | RFC 8950 | IPv6 next-hop for IPv4 NLRI |
| Graceful Restart | 64 | RFC 4724 | Session preservation across restarts |
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
| `connection` | Connect mode: both / passive / active | Default: both |
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

<!-- source: internal/component/bgp/plugins/rib/register.go -- bgp-rib -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/register.go -- bgp-adj-rib-in -->
<!-- source: internal/component/bgp/plugins/persist/register.go -- bgp-persist -->
<!-- source: internal/component/bgp/plugins/rs/register.go -- bgp-rs -->
<!-- source: internal/component/bgp/plugins/watchdog/register.go -- bgp-watchdog -->

### Protocol

| Plugin | Description |
|--------|-------------|
| bgp-gr | Graceful Restart (RFC 4724) and Long-Lived GR (RFC 9494) state machine |
| bgp-rpki | RPKI origin validation via RTR protocol (RFC 6811, RFC 8210). [Guide](guide/rpki.md) |
| bgp-rpki-decorator | Merged UPDATE+RPKI events (correlates update and rpki streams into update-rpki) |
| bgp-route-refresh | Route Refresh handling (RFC 2918, RFC 7313) |
| role | BGP Role capability enforcement (RFC 9234) |
| bgp-llnh | Link-local next-hop for IPv6 (RFC 2545) |
| bgp-hostname | FQDN capability for peer identification |
| bgp-softver | Software version capability advertisement |
| bgp-llnh | Link-local next-hop for IPv6 (RFC 2545) |

<!-- source: internal/component/bgp/plugins/gr/register.go -- bgp-gr -->
<!-- source: internal/component/bgp/plugins/rpki/register.go -- bgp-rpki -->
<!-- source: internal/component/bgp/plugins/rpki_decorator/register.go -- bgp-rpki-decorator -->
<!-- source: internal/component/bgp/plugins/route_refresh/register.go -- bgp-route-refresh -->
<!-- source: internal/component/bgp/plugins/role/register.go -- role -->
<!-- source: internal/component/bgp/plugins/llnh/register.go -- bgp-llnh -->
<!-- source: internal/component/bgp/plugins/hostname/register.go -- bgp-hostname -->
<!-- source: internal/component/bgp/plugins/softver/register.go -- bgp-softver -->

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

<!-- source: cmd/ze/config/main.go -- config subcommand dispatch -->
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
| `ze signal stop` | Send SIGTERM — graceful shutdown |
| `ze signal quit` | Send SIGQUIT — goroutine dump + exit |
| `ze status` | Check if daemon is running |

<!-- source: cmd/ze/signal/main.go -- signal subcommand dispatch -->

### Runtime Interaction

| Command | Description |
|---------|-------------|
| `ze cli` | Interactive CLI (with `--run <cmd>` for single command) |
| `ze show <command>` | Read-only daemon commands |
| `ze run <command>` | All daemon commands |

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
| `bgp peer <addr> add <config>` | Dynamic peer addition |
| `bgp peer <addr> remove` | Remove peer |
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

## ExaBGP Compatibility

<!-- source: cmd/ze/exabgp/main.go -- ze exabgp subcommands -->
<!-- source: internal/exabgp/migration/migrate.go -- ExaBGP config migration -->

- Automatic detection and migration of ExaBGP configuration files
- `ze exabgp plugin` runs ExaBGP processes with ze as the BGP engine
- `ze exabgp migrate` converts ExaBGP configs to ze format
