# API Commands

**Source:** ExaBGP `reactor/api/command/`, `reactor/api/dispatch/`
**Purpose:** Document all API commands for compatibility

---

## Overview

ExaBGP supports two API versions:
- **API v6** (current): Target-first syntax, JSON only
- **API v4** (legacy): Action-first syntax, JSON or text

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

## v6 Syntax (Target-First)

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
session reset            # Reset session state
session ping             # Health check
session bye              # Close CLI connection
```

### System Commands

```
system help              # Show help
system version           # Show version
system api version       # Show API version
system queue-status      # Show write queue status
system crash             # Debug: trigger crash
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

### Peer Selectors

```
peer *                   # All peers
peer 192.168.1.2         # Specific peer by IP
peer [local-as 65001]    # Peers matching filter
peer [peer-as 65002]     # Peers by peer AS
peer [local-ip 1.1.1.1]  # Peers by local IP
peer [id 1.1.1.1]        # Peers by router-id
peer [family-allowed ipv4 unicast]  # Peers with family
```

### Announce Commands

```
peer <selector> announce route <prefix> next-hop <ip> [attributes...]
peer <selector> announce ipv4 unicast <prefix> next-hop <ip> [attributes...]
peer <selector> announce ipv6 unicast <prefix> next-hop <ip> [attributes...]
peer <selector> announce flow <flow-spec> [attributes...]
peer <selector> announce vpls <name> endpoint <id> ... [attributes...]
peer <selector> announce eor <afi> <safi>
peer <selector> announce route-refresh <afi> <safi>
peer <selector> announce operational <type> <afi> <safi>
```

### Withdraw Commands

```
peer <selector> withdraw route <prefix> [attributes...]
peer <selector> withdraw ipv4 unicast <prefix> [attributes...]
peer <selector> withdraw ipv6 unicast <prefix> [attributes...]
peer <selector> withdraw flow <flow-spec>
peer <selector> withdraw vpls <name>
peer <selector> withdraw watchdog <name>
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
peer <selector> announce route ...
peer <selector> announce route ...
group end                         # End batch, send all
```

---

## v4 Syntax (Action-First)

### Show Commands

```
show neighbor [summary|extensive|configuration]
show adj-rib in [<afi> <safi>]
show adj-rib out [<afi> <safi>]
```

### Announce/Withdraw

```
announce route <prefix> next-hop <ip> [attributes...]
withdraw route <prefix>
announce ipv4 unicast <prefix> next-hop <ip>
announce ipv6 unicast <prefix> next-hop <ip>
announce flow <flow-spec>
announce eor <afi> <safi>
announce route-refresh <afi> <safi>
```

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
announce route 10.0.0.0/23 next-hop 1.2.3.4 split /24
# → 10.0.0.0/24 next-hop 1.2.3.4
# → 10.0.1.0/24 next-hop 1.2.3.4

# With MPLS label - label applies to each prefix
announce ipv4 nlri-mpls 10.0.0.0/22 label 100 next-hop 1.2.3.4 split /24
# → 10.0.0.0/24 label 100
# → 10.0.1.0/24 label 100
# → 10.0.2.0/24 label 100
# → 10.0.3.0/24 label 100

# With L3VPN - RD and label apply to each prefix
announce ipv4 mpls-vpn 10.0.0.0/23 rd 100:1 label 200 next-hop 1.2.3.4 split /24
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
announce ipv4 flow \
  destination 10.0.0.0/8 \
  destination-port =80 \
  protocol =tcp \
  then discard

withdraw ipv4 flow \
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

### v6 Tree Structure

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
    {"peer * announce route", true, []string{"next-hop", "origin", ...}},
    // ...
}
```

---

**Last Updated:** 2025-12-23
