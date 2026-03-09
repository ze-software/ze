# Ze Features

Ze is a BGP daemon written in Go. This document lists all user-facing features.

## BGP Protocol

### Address Families

| Family | AFI/SAFI | Encode | Decode | Route Config |
|--------|----------|--------|--------|--------------|
| IPv4 Unicast | 1/1 | Yes | Yes | Yes |
| IPv6 Unicast | 2/1 | Yes | Yes | Yes |
| IPv4 VPN | 1/128 | Yes | Yes | Yes |
| IPv6 VPN | 2/128 | Yes | Yes | Yes |
| IPv4 FlowSpec | 1/133 | Yes | Yes | Yes |
| IPv6 FlowSpec | 2/133 | Yes | Yes | Yes |
| IPv4 MPLS Label | 1/4 | Yes | Yes | Yes |
| IPv6 MPLS Label | 2/4 | Yes | Yes | Yes |
| L2VPN EVPN | 25/70 | Yes | Yes | Yes |
| L2VPN VPLS | 25/65 | Yes | Yes | Yes |
| BGP-LS | 16/71 | No | Yes | No |
| BGP-LS VPN | 16/72 | No | Yes | No |
| IPv4 MVPN | 1/5 | No | Yes | No |
| IPv6 MVPN | 2/5 | No | Yes | No |
| IPv4 RTC | 1/132 | No | Yes | No |
| IPv4 MUP | 1/85 | Yes | Yes | Yes |
| IPv6 MUP | 2/85 | Yes | Yes | Yes |

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
| BGP Role | 9 | RFC 9234 | Peer relationship role |
| Hostname | 73 | draft | FQDN capability |
| Software Version | 75 | draft | Software version advertisement |
| Link-Local Next Hop | 77 | — | IPv6 link-local as next-hop |

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

## Configuration

### Peer Settings

| Setting | Description | Validation |
|---------|-------------|------------|
| peer-address | BGP neighbor IP | Required |
| local-address | Local bind address | Required |
| local-as | Local AS number | Required |
| peer-as | Peer AS number | Required |
| router-id | Per-peer router ID override | Optional |
| hold-time | Hold timer (0 or 3-65535 seconds) | 1-2 rejected |
| connection | Connect mode: both / passive / active | Default: both |
| md5-password | TCP MD5 authentication | Optional |
| outgoing-ttl | TTL for outgoing packets | Optional |
| ttl-security | Minimum TTL for incoming packets | Optional |
| group-updates | Enable/disable UPDATE grouping | Default: enabled |

### Capabilities Configuration

| Capability | Config Key | Values |
|------------|-----------|--------|
| 4-byte ASN | `asn4` | true / false |
| Route Refresh | `route-refresh` | true / false |
| ADD-PATH | `add-path` | Per-family send/receive/both |
| Extended Message | `extended-message` | true / false |
| Extended Next Hop | `nexthop` | Per-family AFI mapping |
| Graceful Restart | `graceful-restart` | restart-time (0-4095s) |
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
| bgp-rib | Route Information Base — stores received/sent routes |
| bgp-adj-rib-in | Adj-RIB-In — raw hex replay of received routes |
| bgp-persist | Route persistence across restarts |
| bgp-rs | Route server — client-to-client route reflection (RFC 7947) |
| bgp-watchdog | Deferred route announcement with named watchdog groups |

### Protocol

| Plugin | Description |
|--------|-------------|
| bgp-gr | Graceful Restart state machine (RFC 4724) |
| bgp-route-refresh | Route Refresh handling (RFC 2918, RFC 7313) |
| role | BGP Role capability enforcement (RFC 9234) |
| bgp-hostname | FQDN capability for peer identification |
| bgp-softver | Software version capability advertisement |
| bgp-llnh | Link-local next-hop for IPv6 (RFC 2545) |

## CLI Commands

### Protocol Tools

| Command | Description |
|---------|-------------|
| `ze bgp decode` | Decode BGP message from hex to JSON |
| `ze bgp encode` | Encode text route command to BGP wire hex |

### Configuration Management

| Command | Description |
|---------|-------------|
| `ze validate <file>` | Validate configuration file |
| `ze config edit` | Interactive configuration editor |
| `ze config check` | Check config for deprecated patterns |
| `ze config migrate` | Convert ExaBGP config to ze format |
| `ze config fmt` | Format and normalize config file |
| `ze config dump` | Dump parsed configuration tree |
| `ze config diff <a> <b>` | Compare two configuration files |
| `ze config set` | Set a configuration value programmatically |

### Schema Discovery

| Command | Description |
|---------|-------------|
| `ze schema list` | List all registered YANG schemas |
| `ze schema show <module>` | Show YANG content for a module |
| `ze schema handlers` | List handler→module mapping |
| `ze schema methods [module]` | List RPCs from YANG modules |
| `ze schema events` | List notifications from YANG |
| `ze schema protocol` | Show protocol version and format info |

### Daemon Control

| Command | Description |
|---------|-------------|
| `ze <config-file>` | Start daemon with configuration |
| `ze signal reload` | Send SIGHUP — reload configuration |
| `ze signal stop` | Send SIGTERM — graceful shutdown |
| `ze signal quit` | Send SIGQUIT — goroutine dump + exit |
| `ze status` | Check if daemon is running |

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
| `ze completion bash/zsh` | Generate shell completion scripts |
| `ze --plugins` | List available internal plugins |

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

Peer selector supports: `*` (all), exact IP, glob patterns (`192.168.*.*`), exclusion (`!addr`).

### Route Updates

| Command | Description |
|---------|-------------|
| `bgp peer * update text <attrs> nlri <family> <op> <prefix>` | Text-format UPDATE |
| `bgp peer * update hex <hex>` | Hex-format UPDATE |

Text attribute syntax: `origin set igp`, `nhop set 1.1.1.1`, `local-preference set 100`, `med set 50`, `as-path set [65000 65001]`, `community set [no-export]`, `large-community set [65000:1:1]`.

NLRI operations: `add`, `del`, `eor` per address family.

### RIB Operations

| Command | Description |
|---------|-------------|
| `rib show-in [peer] [family]` | Show Adj-RIB-In |
| `rib show-out [peer] [family]` | Show Adj-RIB-Out |
| `rib clear-in [peer] [family]` | Clear Adj-RIB-In |
| `rib clear-out [peer] [family]` | Clear Adj-RIB-Out |

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

### Raw & Introspection

| Command | Description |
|---------|-------------|
| `bgp peer * raw <hex>` | Send raw BGP message bytes |
| `route-refresh <family>` | Send route refresh request |
| `help` | Show available commands |
| `command-list` | List all commands with descriptions |
| `command-help <name>` | Detailed help for command |

## Configuration Reload

Ze supports live configuration reload via SIGHUP or `ze signal reload`:

- Add/remove peers without restart
- Update peer settings with automatic reconciliation
- Graceful failure on invalid config (keeps running)
- Rapid successive reloads handled correctly

## ExaBGP Compatibility

- Automatic detection and migration of ExaBGP configuration files
- `ze exabgp plugin` runs ExaBGP processes with ze as the BGP engine
- `ze exabgp migrate` converts ExaBGP configs to ze format
