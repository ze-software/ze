# API Commands

**Source:** ExaBGP `reactor/api/command/`, `reactor/api/dispatch/`
**Purpose:** Document all API commands for compatibility

---

## Overview

ZeBGP uses target-first syntax with JSON or text encoding.

### ExaBGP Differences

| Aspect | ExaBGP | ZeBGP |
|--------|--------|-------|
| Syntax styles | v4 (action-first) and v6 (target-first) | Target-first only |
| Encoder | json or text (v4), json only (v6) | json or text |
| Peer selectors | `*`, IP, filters (`[local-as ...]`) | `*`, IP, negated (`!IP`) |
| Multi-session filters | Supported (draft) | Not supported |
| Forward command | Not available | `forward update-id` for route reflection |

See [JSON_FORMAT.md](JSON_FORMAT.md#exabgp-differences) for output format differences.

---

## Command Categories

| Category | Commands |
|----------|----------|
| Daemon | shutdown, reload, restart, status |
| Session | ack, sync, reset, ping, bye |
| System | help, version, api version |
| Peer | list, show, create, delete, teardown |
| Announce | route, flow, vpls, eor, operational |
| Withdraw | route, flow, vpls, watchdog |
| RIB | show, flush, clear |
| Group | start, end (batching) |

---

## Target-First Syntax

### Daemon Commands

```
daemon shutdown          # Graceful shutdown
daemon reload            # Reload configuration
daemon restart           # Restart all peers
daemon status            # Get daemon status
```

### Session Commands

```
session ack enable       # Enable command acknowledgment
session ack disable      # Disable acknowledgment
session ack silence      # Silence all output
session sync enable      # Enable synchronous mode
session sync disable     # Disable synchronous mode
plugin session ready     # Signal plugin init complete
plugin session ping      # Health check
plugin session bye       # Disconnect
```

### System Commands

```
system help              # Show help (uses dispatcher, includes plugin commands)
system version software  # Show ZeBGP version
system version api       # Show IPC protocol version
system shutdown          # Graceful application shutdown
system subsystem list    # List available subsystems
system command list      # List all commands (builtin + plugin)
system command list verbose  # List with source (builtin/process name)
system command help "<name>" # Show command details
system command complete "<partial>"  # Complete command names
system command complete "<cmd>" args [<completed>...] "<partial>"  # Arg completion
```

### Peer Commands

```
peer list                # List all peers
peer show                # Show all peers (detailed)
peer <ip> show           # Show specific peer
peer <ip> show summary   # Show peer summary
peer <ip> show extensive # Show extensive detail
peer <ip> teardown <code> [<reason>]  # Disconnect peer
peer create <config>     # Create dynamic peer
peer <ip> delete         # Delete dynamic peer
```

### Forward Commands (ZeBGP)

> **Implementation spec:** `docs/plan/spec-route-id-forwarding.md`

```
peer <selector> forward update-id <id>    # Forward received UPDATE by ID
peer !<ip> forward update-id <id>         # Forward to all except source peer
```

The `forward` command enables route reflection via API:
1. Received UPDATEs are assigned a unique update-id (per-UPDATE, not per-NLRI)
2. API outputs UPDATE info with update-id
3. External process decides routing
4. Forward command references update-id (zero-copy when contexts match)
5. Update-ids expire after configurable TTL (default 60s)

### Peer Selectors

```
peer *                   # All peers
peer 192.168.1.2         # Specific peer by IP
peer !192.168.1.2        # All peers EXCEPT this IP (for route reflection)
```

The `!<ip>` negated selector is useful for route reflection:
```
# Forward update to all peers except the source
peer !10.0.0.1 forward update-id 12345
```

> **Note:** Filter selectors (`[local-as ...]`, `[peer-as ...]`) from ExaBGP multi-session
> draft are not supported — the draft never became an RFC.

### Route Commands (update text)

All route operations use unified `update text` syntax:

```bash
# Announce routes
peer <selector> update text nhop set <ip> [attributes...] nlri <family> add <prefix>...

# Withdraw routes
peer <selector> update text nlri <family> del <prefix>...

# End-of-RIB marker (RFC 4724)
peer <selector> update text nlri <family> eor

# VPLS (L2VPN/VPLS)
peer <selector> update text nlri l2vpn/vpls add rd <rd> ve-id <n> ve-block-offset <n> ve-block-size <n> label-base <n>

# EVPN (L2VPN/EVPN)
peer <selector> update text nlri l2vpn/evpn add mac-ip rd <rd> mac <mac> [ip <ip>] label <n>
peer <selector> update text nlri l2vpn/evpn add ip-prefix rd <rd> prefix <prefix> label <n>
peer <selector> update text nlri l2vpn/evpn add multicast rd <rd> ip <ip>
```

### Removed Commands (use update text instead)

The following legacy commands have been removed:

| Old Command | Replacement |
|-------------|-------------|
| `announce ipv4/unicast <p> next-hop <nh>` | `update text nhop set <nh> nlri ipv4/unicast add <p>` |
| `announce ipv6/unicast <p> next-hop <nh>` | `update text nhop set <nh> nlri ipv6/unicast add <p>` |
| `announce eor <afi> <safi>` | `update text nlri <family> eor` |
| `announce vpls ...` | `update text nlri l2vpn/vpls add ...` |
| `announce l2vpn ...` | `update text nlri l2vpn/evpn add ...` |
| `withdraw ipv4/unicast <p>` | `update text nlri ipv4/unicast del <p>` |
| `withdraw ipv6/unicast <p>` | `update text nlri ipv6/unicast del <p>` |
| `withdraw vpls ...` | `update text nlri l2vpn/vpls del ...` |
| `withdraw l2vpn ...` | `update text nlri l2vpn/evpn del ...` |

### Watchdog Commands

```
watchdog announce <name>   # Send all routes in pool to peers
watchdog withdraw <name>   # Withdraw all routes in pool from peers
```

Routes are tagged with a pool when announced:
```bash
update text nhop set 10.0.0.1 nlri ipv4/unicast add 1.0.0.0/24 watchdog set mypool
```

### RIB Commands

```
rib show in [<afi> <safi>]       # Show Adj-RIB-In
rib show out [<afi> <safi>]      # Show Adj-RIB-Out
rib flush out                     # Flush Adj-RIB-Out
rib clear in                      # Clear Adj-RIB-In
rib clear out                     # Clear Adj-RIB-Out
```

### Group Commands (Batching)

```
group start [attributes ...]     # Start batch with shared attributes
peer <selector> update text ...
peer <selector> update text ...
group end                         # End batch, send all
```

---

## Action-First Syntax (Legacy)

### Show Commands

```
show neighbor [summary|extensive|configuration]
show adj-rib in [<afi> <safi>]
show adj-rib out [<afi> <safi>]
```

### Announce/Withdraw

All route operations now use `update text` syntax:

```bash
update text nhop set <ip> [attributes...] nlri <family> add <prefix>
update text nlri <family> del <prefix>
update text nlri <family> eor
```

> **Note:** Legacy `announce`/`withdraw` commands have been removed.
> See "Removed Commands" section above for migration table.

### Control

```
teardown <peer-ip> <code> [<reason>]
shutdown
reload
restart
reset
enable-ack
disable-ack
silence-ack
help
version
```

---

## API Content Configuration (ZeBGP)

### Attribute Filtering

Limit which attributes are parsed for API output:

```
api route-server {
    content {
        encoding json;
        attribute as-path community next-hop;  # Only parse these
    }
    receive { update; }
}
```

Available attribute names:
| Name | Code | Description |
|------|------|-------------|
| `origin` | 1 | ORIGIN |
| `as-path` | 2 | AS_PATH |
| `next-hop` | 3 | NEXT_HOP |
| `med` | 4 | MULTI_EXIT_DISC |
| `local-pref` | 5 | LOCAL_PREF |
| `atomic-aggregate` | 6 | ATOMIC_AGGREGATE |
| `aggregator` | 7 | AGGREGATOR |
| `community` | 8 | COMMUNITIES |
| `originator-id` | 9 | ORIGINATOR_ID |
| `cluster-list` | 10 | CLUSTER_LIST |
| `extended-community` | 16 | EXTENDED_COMMUNITIES |
| `large-community` | 32 | LARGE_COMMUNITIES |
| `all` | - | All attributes (default) |

Benefits of partial parsing:
- Reduced CPU (only parse what's needed for routing decision)
- Reduced memory (don't store full parsed attributes)
- Wire bytes preserved for zero-copy forwarding

### NLRI Family Filtering

Limit which address families are included in API output:

```
api route-server {
    content {
        encoding json;
        attribute as-path community next-hop;
        nlri ipv4/unicast;
        nlri ipv6/unicast;
    }
    receive { update; }
}
```

Available families:
| Config Syntax | Canonical Name |
|---------------|----------------|
| `ipv4/unicast` | ipv4/unicast |
| `ipv6/unicast` | ipv6/unicast |
| `ipv4/multicast` | ipv4/multicast |
| `ipv6/multicast` | ipv6/multicast |
| `ipv4 mpls` | ipv4 mpls |
| `ipv6 mpls` | ipv6 mpls |
| `ipv4/mpls-vpn` | ipv4/mpls-vpn |
| `ipv6/mpls-vpn` | ipv6/mpls-vpn |
| `ipv4/flowspec` | ipv4/flowspec |
| `ipv6/flowspec` | ipv6/flowspec |
| `l2vpn/evpn` | l2vpn/evpn |
| `l2vpn/vpls` | l2vpn/vpls |

Special values: `all` (default), `none`

---

## Route Attributes

```
next-hop <ip>                    # Next-hop IP (required)
origin igp|egp|incomplete        # Origin attribute
as-path [<asn> ...]             # AS path
local-preference <int>           # Local preference
med <int>                        # Multi-exit discriminator
community [<comm> ...]           # Standard communities
extended-community [<ext> ...]   # Extended communities
large-community [<lc> ...]       # Large communities
originator-id <ip>               # Originator ID
cluster-list [<ip> ...]          # Cluster list
label [<label> ...]              # MPLS labels
rd <rd>                          # Route distinguisher
path-information <id>            # ADD-PATH path ID
atomic-aggregate                 # Atomic aggregate flag
aggregator <asn> <ip>            # Aggregator
aigp <value>                     # AIGP
split /<len>                     # ZeBGP: prefix expansion (see below)
```

---

## Split Keyword (ZeBGP Extension)

The `split` keyword expands a prefix into smaller prefixes. All attributes apply to each generated prefix.

### Syntax

```
split /<target-length>
```

### Example

```
# Announce 2 prefixes with one command
update text nhop set 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/23 split /24
# → 10.0.0.0/24 next-hop 1.2.3.4
# → 10.0.1.0/24 next-hop 1.2.3.4

# With MPLS label - label applies to each prefix
announce ipv4/nlri-mpls 10.0.0.0/22 label 100 next-hop 1.2.3.4 split /24
# → 10.0.0.0/24 label 100
# → 10.0.1.0/24 label 100
# → 10.0.2.0/24 label 100
# → 10.0.3.0/24 label 100

# With L3VPN - RD and label apply to each prefix
announce ipv4/mpls-vpn 10.0.0.0/23 rd 100:1 label 200 next-hop 1.2.3.4 split /24
# → 10.0.0.0/24 rd 100:1 label 200
# → 10.0.1.0/24 rd 100:1 label 200
```

### Supported Families

| Family | Split Support | Notes |
|--------|---------------|-------|
| IPv4/IPv6 unicast | ✅ | Standard prefix expansion |
| IPv4/IPv6 nlri-mpls | ✅ | Label copied to each prefix |
| IPv4/IPv6 mpls-vpn | ✅ | RD + label copied to each prefix |
| FlowSpec | ❌ | N/A - uses match rules, not prefixes |
| VPLS/EVPN | ❌ | Different NLRI structure |

### Constraints

- Target length must be longer than source prefix (e.g., /23 → /24, not /24 → /23)
- Maximum expansion: implementation-dependent (avoid /8 → /32)

---

## FlowSpec Commands

```
announce ipv4/flow \
  destination 10.0.0.0/8 \
  destination-port =80 \
  protocol =tcp \
  then discard

withdraw ipv4/flow \
  destination 10.0.0.0/8 \
  destination-port =80
```

### Match Components

| Keyword | Description |
|---------|-------------|
| destination | Destination prefix |
| source | Source prefix |
| destination-port | Destination port |
| source-port | Source port |
| port | Any port |
| protocol | IP protocol |
| next-header | IPv6 next header |
| tcp-flags | TCP flags |
| icmp-type | ICMP type |
| icmp-code | ICMP code |
| fragment | Fragment flags |
| dscp | DSCP value |
| packet-length | Packet length |
| flow-label | IPv6 flow label |

### Actions (then)

| Keyword | Description |
|---------|-------------|
| accept | Accept traffic |
| discard | Drop traffic |
| rate-limit <bps> | Rate limit |
| redirect <rt> | Redirect to VRF |
| redirect-next-hop | Redirect to next-hop |
| mark <dscp> | Set DSCP |
| community [...] | Add community |

---

## Response Format

### Success (with ACK enabled)

```json
{ "type": "done" }
```

### Error

```json
{ "type": "error", "error": "description" }
```

### Show Neighbor

```json
{
  "neighbor": {
    "address": "192.168.1.2",
    "local-address": "192.168.1.1",
    "local-as": 65001,
    "peer-as": 65002,
    "router-id": "1.1.1.1",
    "state": "established"
  }
}
```

### Show Adj-RIB

```json
{
  "routes": [
    {
      "nlri": "10.0.0.0/8",
      "next-hop": "192.168.1.2",
      "origin": "igp",
      "as-path": [65002]
    }
  ]
}
```

---

## Command Dispatch

### Command Tree Structure

```
daemon
├── shutdown
├── reload
├── restart
└── status

session
├── ack
│   ├── enable
│   ├── disable
│   └── silence
├── sync
│   ├── enable
│   └── disable
├── reset
├── ping
└── bye

peer
├── list
├── show
├── create
├── delete
└── <selector>
    ├── show
    ├── teardown
    ├── announce
    ├── withdraw
    └── group

rib
├── show
├── flush
└── clear

group
├── start
└── end
```

---

## ZeBGP Implementation Notes

### Command Dispatcher

```go
type Handler func(ctx *Context, peers []string, remaining string) error

type DispatchTree map[string]interface{}  // Handler or nested DispatchTree

func Dispatch(tree DispatchTree, tokens *Tokenizer, reactor *Reactor) (Handler, []string) {
    // Walk tree consuming tokens
    // Return handler and matched peers
}
```

### Peer Selector Parsing

```go
type Selector struct {
    All       bool
    IP        netip.Addr
    Filters   map[string]string  // local-as, peer-as, local-ip, id, family
}

func ParseSelector(s string) (*Selector, error) {
    if s == "*" {
        return &Selector{All: true}, nil
    }
    if strings.HasPrefix(s, "[") {
        return parseFilteredSelector(s)
    }
    ip, err := netip.ParseAddr(s)
    return &Selector{IP: ip}, err
}
```

### Command Registry

```go
var Commands = []CommandInfo{
    {"daemon shutdown", false, nil},
    {"peer * update text", true, []string{"nhop", "origin", ...}},
    // ...
}
```

---

**Last Updated:** 2025-12-23
