# Configuration Syntax

## TL;DR (Read This First)

| Concept | Description |
|---------|-------------|
| **Style** | JUNOS-like: `{}` blocks, `;` terminators, `#` comments |
| **Top-Level** | `environment`, `plugin`, `bgp` |
| **BGP Block** | `bgp { group <name> { peer <ip> { ... } } }` - wraps all BGP config |
| **Groups** | `group <name> { ... peer <ip> { } }` - named peer groups with shared defaults |
| **Inheritance** | 3 levels: bgp globals, group defaults, peer overrides (deep-merge for containers) |
| **Schema** | YANG-driven: parser dispatches by node type (leaf, container, list, leaf-list) |
| **Migration** | `ze bgp config migrate` converts ExaBGP syntax to ze-native |

**When to read full doc:** Config keywords, parsing bugs, new config sections.

---

**Purpose:** Document complete ze configuration file syntax

---

## Overview

Ze configuration uses a JUNOS-like hierarchical syntax with sections, keywords, and values terminated by semicolons or braces. The parser is YANG-driven: each config node's type (leaf, leaf-list, container, list) determines how it is parsed. No custom `ze:syntax` annotations are used in ze-native config.

---

## Basic Syntax Patterns

### Leaf (single value)

```
keyword value;
```

### Leaf-List (single or multiple values)

A leaf-list accepts either a single value or a bracket list:

```
community 65001:100;                           # single value
community [ 65001:100 65001:200 65001:300 ];   # bracket list
as-path [ 65001 65002 65003 ];                 # bracket list
```

Both forms are equivalent for a single item: `community 65001:100;` and `community [ 65001:100 ];` produce the same result.

### Container (block)

```
keyword {
    child1 value1;
    child2 value2;
}
```

### List (keyed entries)

```
keyword key1 {
    child1 value1;
}
keyword key2 {
    child1 value2;
}
```

---

## Top-Level Structure

```
# Comment
plugin {
    external <name> {
        ...
    }
}

bgp {
    router-id <ip>;                         # BGP-level global (inherited by all peers)
    local-as <asn>;                         # BGP-level global (inherited by all peers)

    group <name> {
        <peer-fields>                       # Group-level defaults (shared by all peers in group)
        peer <ip-address> {
            name <display-name>;            # Optional: CLI selector name
            <peer-fields>                   # Peer-level overrides
        }
    }

    peer <ip-address> {                     # Standalone peer (no group inheritance)
        name <display-name>;
        <peer-fields>
    }
}
```

---

## Section Types

### environment

Ze-specific block for setting environment configuration from the config file.
See [ENVIRONMENT_BLOCK.md](ENVIRONMENT_BLOCK.md) for full documentation.

```
environment {
    log { level DEBUG; }
    tcp { port 1179; }
}
```

### plugin

Container for plugin definitions. Supports `external` for subprocess plugins.
Future: `builtin` and `wasm` for other plugin types.

```
plugin {
    external <name> {
        run <path>;
        encoder json;           # or text (v4 only)
        timeout 10s;            # stage timeout (default: 5s)
    }
}
```

| Keyword | Type | Description |
|---------|------|-------------|
| run | string | Command to execute |
| encoder | string | `json` or `text` |
| timeout | duration | Per-stage timeout (e.g., `5s`, `1m`, `500ms`). Default: 5s. 0 = use default. Negative rejected. |

**Timeout semantics:** During startup, all plugins synchronize at each stage. The timeout controls how long this plugin waits for all plugins to complete each stage. With multiple plugins, use the same timeout for all, or set the longest timeout on all plugins to avoid fast plugins timing out while waiting for slow ones.

### Peer Groups

Peers are organized into named groups under `bgp`. Groups provide shared defaults inherited by all member peers.

#### Group Structure

```
bgp {
    group <name> {
        <peer-fields>               # Group-level defaults
        peer <ip-address> {
            name <display-name>;    # Optional CLI selector
            <peer-fields>           # Peer-level overrides
        }
    }
}
```

| Element | Type | Purpose |
|---------|------|---------|
| `group` | list (key: name) | Named collection of peers with shared defaults |
| `peer` inside group | list (key: IP) | Peer with optional overrides of group defaults |
| `peer` at bgp level | list (key: IP) | Standalone peer (no group inheritance) |
| `name` on peer | leaf (optional) | Display name usable as CLI selector instead of IP |

#### 3-Level Inheritance

| Priority | Source | Scope |
|----------|--------|-------|
| Lowest | `bgp` block globals | `local-as`, `router-id` -- inherited by all peers |
| Middle | Group defaults | All `peer-fields` set on the group |
| Highest | Peer overrides | All `peer-fields` set on the peer |

Containers (like `capability`) deep-merge at key level -- both group and peer capabilities are combined. Leaves (like `hold-time`) override -- peer value wins over group value.

#### Example

```
bgp {
    router-id 1.2.3.4
    local-as 65000

    group rr-clients {
        hold-time 180
        capability { route-refresh enable; }

        peer 10.0.0.1 {
            name router-east
            peer-as 65001
            local-address 10.0.0.1
        }
        peer 10.0.0.2 {
            peer-as 65002
            local-address 10.0.0.2
            hold-time 90               # Overrides group's 180
        }
    }

    group edge-peers {
        hold-time 30
        peer 192.168.1.1 {
            peer-as 64500
            local-address 192.168.1.254
        }
    }
}
```

#### Peer Name Rules

| Rule | Detail |
|------|--------|
| Optional | Peers may have no name |
| Unique | Two peers with the same name produce a config validation error |
| Characters | Alphanumeric, hyphens, underscores. No spaces, dots, colons, commas, or `*` |
| CLI usage | `peer <name> <command>` works identically to `peer <ip> <command>` |
| Disambiguation | A name must not parse as a valid IP address |

#### Migration

`ze bgp config migrate` converts old syntax:
- `template { bgp { peer <pattern> { inherit-name X; } } }` becomes `group X { }`
- `inherit X` in peers moves the peer into `group X`
- `neighbor` blocks become `peer` blocks inside groups

### bgp

Container for all BGP-related configuration (peers, groups, global settings).

```
bgp {
    router-id <ip>;         # Global router-id (inherited by all peers)
    local-as <asn>;         # Global local-as (inherited by all peers)

    group <name> {
        # Group-level defaults (inherited by all peers in group)
        hold-time <seconds>;
        capability { ... }
        family { ... }

        peer <ip-address> {
            name <display-name>;    # Optional CLI selector

            # Peer-level overrides
            router-id <ip>;
            local-address <ip>;
            local-as <asn>;
            peer-as <asn>;
            hold-time <seconds>;

            # Capabilities
            capability { ... }

            # Address families
            family { ... }

            # Process bindings
            process <plugin-name> { ... }

            # Route announcements
            update { ... }
        }
    }
}
```

**Migration:** `ze bgp config migrate` converts old syntax:
- `neighbor` to `bgp { group <name> { peer } }`
- `template { inherit-name X }` to `group X { }`
- `inherit X` in peers moves peer into `group X`

---

## Peer Keywords

### Session

| Keyword | Type | Description |
|---------|------|-------------|
| router-id | IP | BGP router ID |
| local-address | IP | Local address to bind |
| local-as | ASN | Local AS number |
| peer-as | ASN | Peer AS number |
| hold-time | int | Hold time (seconds) |
| connection | enum | `both` (default), `passive`, `active` |
| port | int | Per-peer listen port |
| ttl-security | int | TTL security hop count |
| md5-password | string | MD5 authentication password |
| md5-ip | IP | MD5 authentication IP |
| group-updates | bool | Group updates for efficiency |
| auto-flush | bool | Auto-flush after each update |
| outgoing-ttl | int | Outgoing TTL |
| incoming-ttl | int | Incoming TTL |

### Capability Section

All capabilities support a four-mode vocabulary:

| Mode | Advertise? | Enforcement | Aliases |
|------|------------|-------------|---------|
| `enable` | Yes | None | `true` |
| `disable` | No | None | `false` |
| `require` | Yes | Reject peer if capability missing | |
| `refuse` | No | Reject peer if capability present | |

**Simple capabilities** -- mode is the value:

```
capability {
    asn4 require;                  # Reject peers without 4-byte ASN
    route-refresh enable;          # Advertise, no enforcement
    graceful-restart enable;       # Advertise GR support
    extended-message require;      # Reject peers without extended message
    software-version disable;      # Don't advertise
}
```

**Removed capabilities** -- these are not supported and will be rejected:
`multi-session`, `operational`, `aigp`. These were ExaBGP-era capabilities with no ze runtime implementation.

**Block capabilities** -- `mode` key inside block (for capabilities with sub-parameters):

```
capability {
    graceful-restart {
        mode require;              # Reject peers without GR
        restart-time 120;
    }
}
```

**Nexthop** -- structured inline list keyed by family:

```
capability {
    nexthop {
        ipv4/unicast ipv6;         # IPv4 unicast with IPv6 next-hop (enable)
        ipv4/mpls-vpn ipv6 require; # IPv4 VPN with IPv6 next-hop (require mode)
    }
}
```

Each line is parsed as: `<family> <next-hop-afi> [<mode>]` where family is the list key, `next-hop-afi` is `ipv4` or `ipv6`, and mode defaults to `enable`.

**ADD-PATH** -- trailing mode token (global or per-family):

```
capability {
    add-path send/receive require; # Global: require ADD-PATH for all families
}
add-path {
    ipv4/unicast send require;     # Per-family: require ADD-PATH for IPv4 unicast
    ipv6/unicast send/receive;     # Per-family: enable (no enforcement)
}
```

The last token is interpreted as a mode if it matches `require`|`refuse`|`enable`|`disable`. Otherwise the existing direction parsing applies unchanged.

**Defaults:** ASN4 defaults to `enable`. All other capabilities are absent (opt-in) -- they only participate in negotiation when explicitly configured.

**Backwards compatibility:** `true` is accepted as `enable`, `false` as `disable`. Bare capability names (e.g., `route-refresh;`) mean `enable`.

### Family Section

```
family {
    ipv4/unicast;
    ipv4/multicast;
    ipv4/nlri-mpls;
    ipv4/mpls-vpn;
    ipv4/mup;
    ipv6/unicast;
    ipv6/multicast;
    ipv6/nlri-mpls;
    ipv6/mpls-vpn;
    ipv6/mup;
    l2vpn/vpls;
    l2vpn/evpn;
}
```

Block syntax is also supported: `ipv4 { unicast; multicast require; }`.

### ADD-PATH Section

Structured inline list keyed by family:

```
add-path {
    ipv4/unicast;                    # Both send and receive (enable)
    ipv4/unicast send;               # Send only (enable)
    ipv4/unicast receive;            # Receive only (enable)
    ipv4/unicast send require;       # Send only, reject peer if missing
    ipv6/unicast send/receive refuse; # Refuse if peer has it
}
```

Each line is parsed as: `<family> [<direction>] [<mode>]` where family is the list key, direction is `send`, `receive`, or `send/receive` (default: `send/receive`), and mode defaults to `enable`.

### Process Section

```
# Named process binding (preferred)
process <plugin-name> {
    content {
        encoding json;       # json | text (default: inherit from plugin)
        format parsed;       # parsed | raw | full (default: parsed)
        attribute all;       # all | none | "as-path next-hop ..." (default: all)
    }
    receive [ update state negotiated ];  # enum list of message types
    send [ update ];                      # enum list of sendable types
}
```

#### Receive enum values

| Value | Description |
|-------|-------------|
| `all` | Shorthand for all message types |
| `update` | Route announcements |
| `open` | Session open messages |
| `notification` | Error notifications |
| `keepalive` | Keepalive messages |
| `refresh` | Route refresh requests |
| `state` | Peer up/down events |
| `sent` | Sent UPDATE confirmations |
| `negotiated` | Capability negotiation results |

#### Send enum values

| Value | Description |
|-------|-------------|
| `all` | Shorthand for all sendable types |
| `update` | Can inject routes |
| `refresh` | Can request route refresh |

Invalid enum values are rejected at parse time.

---

## Update Block (Ze-Native Routes)

The `update { attribute {} nlri {} }` block is the ze-native way to announce routes in configuration files. All ze-native route config uses this format.

### Attribute Block

Path attributes shared by all NLRI in the update block.

```
update {
    attribute {
        origin igp;
        next-hop 10.0.0.1;
        local-preference 100;
        med 200;
        as-path [ 65001 65002 ];
        community [ 65001:100 65001:200 ];
        extended-community [ target:65001:100 ];
        large-community [ 65001:100:200 ];
        aggregator 65001:10.0.0.1;
        atomic-aggregate enable;
        originator-id 10.0.0.1;
        cluster-list [ 3.3.3.3 192.168.201.1 ];
        path-information 1.2.3.4;
        label 16000;
        labels [ 100 200 300 ];
        split /25;
        attribute [ 0x04 0x80 0x00000064 ];   # generic hex attributes
    }
    nlri {
        ...
    }
}
```

### NLRI Grammar

```
<nlri-line> := <family> [rd <rd>] [label <label>] <op> <payload> ;
<op>        := add | del | eor
<payload>   := <bracket-list> | <structured-payload>
```

- `<family>` -- address family, e.g. `ipv4/unicast`, `ipv4/mpls-vpn`, `ipv4/flow`
- `rd` / `label` -- optional VPN qualifiers, placed before the operation keyword
- `<op>` -- **mandatory** operation: `add` (announce), `del` (withdraw), `eor` (end-of-rib, no payload)
- `<bracket-list>` -- `[ prefix1 prefix2 ... ]` -- for prefix-based families, one route per entry
- `<structured-payload>` -- for complex families (FlowSpec, VPLS, EVPN), one route per line

### Payload dispatch

| Family Category | Bracket List? | Example |
|----------------|---------------|---------|
| Prefix (unicast, multicast, mpls, mpls-vpn) | Yes | `ipv4/unicast add [ 10.0.0.0/24 10.0.1.0/24 ];` |
| FlowSpec (flow, flow-vpn) | No -- one per line | `ipv4/flow add source-ipv4 10.0.0.2/32;` |
| VPLS | No -- one per line | `l2vpn/vpls rd X add ve-id 5 ve-block-offset 1 ...;` |
| EVPN | No -- one per line | `l2vpn/evpn add <route-type-specific>;` |

After `add`, if the next token is `[` the parser reads a bracket list of single-token entries (one route per token). Otherwise it reads one structured NLRI until `;`.

### NLRI Examples

```
update {
    attribute {
        origin igp;
        next-hop 10.0.0.1;
        local-preference 100;
    }
    nlri {
        # Simple unicast
        ipv4/unicast add 10.0.0.0/24;
        ipv4/unicast add [ 10.0.1.0/24 10.0.2.0/24 10.0.3.0/24 ];

        # Withdrawal
        ipv4/unicast del 10.0.99.0/24;

        # End-of-RIB
        ipv4/unicast eor;
    }
}

# VPN with rd and label qualifiers before add
update {
    attribute { origin igp; next-hop 192.168.0.1; }
    nlri {
        ipv4/mpls-vpn rd 100:100 label 20012 add 10.0.0.0/24;
        ipv4/mpls-vpn rd 100:100 label 20012 add [ 10.0.1.0/24 10.0.2.0/24 ];
    }
}

# VPLS
update {
    attribute { origin igp; next-hop 192.168.201.1; }
    nlri {
        l2vpn/vpls rd 192.168.201.1:123 add ve-id 5 ve-block-offset 1
               ve-block-size 8 label-base 10702;
    }
}
```

### FlowSpec

FlowSpec routes (RFC 8955) use the same `update { nlri { } }` syntax. The legacy `flow { route { match {} then {} } }` block is not supported -- use `ze bgp config migrate` to convert ExaBGP-format FlowSpec configs.

FlowSpec actions are expressed as extended-community attributes. Match criteria are inline after the family and `add` keyword.

```
# Simple discard rule
update {
    attribute {
        extended-community [ rate-limit:0 ];
    }
    nlri {
        ipv4/flow add source-ipv4 10.0.0.1/32;
    }
}

# Complex match with redirect
update {
    attribute {
        extended-community [ redirect:65500:12345 ];
    }
    nlri {
        ipv4/flow add destination-ipv4 192.168.0.1/32 source-ipv4 10.0.0.2/32
               protocol [ =tcp =udp ] destination-port [ >8080&<8088 =3128 ]
               source-port >1024;
    }
}

# FlowSpec VPN (with rd qualifier before add)
update {
    attribute {
        extended-community [ rate-limit:0 ];
    }
    nlri {
        ipv4/flow-vpn rd 65535:65536 add source-ipv4 10.0.0.1/32;
    }
}
```

#### Match Criteria

| Criterion | Description |
|-----------|-------------|
| `source-ipv4` / `source-ipv6` | Source prefix |
| `destination-ipv4` / `destination-ipv6` | Destination prefix |
| `protocol` | IP protocol (`=tcp`, `=udp`, or number) |
| `port` | Source or destination port |
| `destination-port` | Destination port |
| `source-port` | Source port |
| `next-header` | IPv6 next header |
| `tcp-flags` | TCP flags (`syn`, `rst`, `fin`, `ack`, `urg`, `push`) |
| `icmp-type` | ICMP type |
| `icmp-code` | ICMP code |
| `fragment` | Fragment flags (`first-fragment`, `last-fragment`) |
| `dscp` | DSCP value |
| `packet-length` | Packet length |
| `traffic-class` | IPv6 traffic class |
| `flow-label` | IPv6 flow label |

#### Match Operators

| Operator | Meaning |
|----------|---------|
| `=` | Equal |
| `>` | Greater than |
| `>=` | Greater or equal |
| `<` | Less than |
| `<=` | Less or equal |
| `!=` | Not equal |
| `&` | AND with previous |

#### FlowSpec Actions (Extended Communities)

| Extended Community | Action |
|--------------------|--------|
| `rate-limit:0` | Discard (rate-limit to zero) |
| `rate-limit:<bps>` | Rate limit |
| `redirect:<asn>:<value>` | Redirect to VRF |
| `redirect-to-nexthop <ip>` | Redirect to IP (RFC 7674) |
| `redirect-to-nexthop-draft` | Redirect to next-hop (draft) |
| `copy-to-nexthop` | Copy to next-hop |
| `action sample-terminal` | Sampling action |
| `mark <dscp>` | Set DSCP value |

### Watchdog

Routes can be tagged with a watchdog group and held until explicitly announced:

```
update {
    attribute { origin igp; next-hop 10.0.0.1; }
    watchdog { name mypool; withdraw true; }
    nlri {
        ipv4/unicast add 10.0.0.0/24;
    }
}
```

Standalone watchdog commands (via API):
```
watchdog announce <name>   # send all routes in pool to peers
watchdog withdraw <name>   # withdraw all routes in pool from peers
```

---

## Value Types

### IP Address

```
192.168.1.1
2001:db8::1
```

### Prefix

```
10.0.0.0/8
2001:db8::/32
```

### ASN

```
65001           # 2-byte
4200000001      # 4-byte
auto            # Use local-as
```

### Community

```
65001:100       # Standard (AS:value)
0x12345678      # Numeric
no-export
no-advertise
no-export-subconfed
nopeer
```

### Extended Community

```
target:65001:100
origin:65001:100
redirect:65001:100
l2info:19:0:1500:111
0x0002fde800000001      # Raw hex (8 bytes)
```

### Large Community

```
65001:100:200   # ASN:value:value
```

### Route Distinguisher

```
65001:100       # Type 0 (ASN2:value)
192.168.1.1:100 # Type 1 (IP:value)
4200000001:100  # Type 2 (ASN4:value)
```

### Boolean

```
enable          # or true
disable         # or false
require         # capability mode: reject session if peer lacks it
refuse          # capability mode: reject session if peer has it
```

The `require` and `refuse` values are accepted by boolean fields to support capability mode enforcement. The parser normalizes `enable` to `true` and `disable` to `false` internally; `require` and `refuse` pass through unchanged.

### Origin

```
igp
egp
incomplete
```

---

## Inheritance

### Peer Group Inheritance

```
bgp {
    router-id 1.2.3.4         # BGP-level global
    local-as 65000             # BGP-level global

    group rr-clients {
        hold-time 90           # Group default
        family {
            ipv4/unicast
        }

        peer 192.168.1.2 {
            peer-as 65002      # Peer override
            local-address 192.168.1.1
        }
    }
}
```

### Key Concepts

| Concept | Description |
|---------|-------------|
| BGP globals | `local-as` and `router-id` at bgp level, inherited by all peers |
| Group defaults | Any `peer-fields` set on the group, inherited by all peers in that group |
| Peer overrides | Any `peer-fields` set on the peer, takes highest precedence |
| Deep merge | Containers (e.g., `capability`) merge keys from both group and peer |
| Leaf override | Scalar values (e.g., `hold-time`) at peer level replace group values |

### Standalone Peers

Peers directly under `bgp` (not inside a group) inherit only BGP-level globals:

```
bgp {
    local-as 65000

    peer 10.0.0.5 {
        peer-as 65001
        local-address 10.0.0.1
        hold-time 180
    }
}
```

---

## Related

- [ExaBGP Legacy Syntax](exabgp-syntax.md) -- `static`, `announce`, `flow` blocks accepted for migration
- [API Update Syntax](../api/UPDATE_SYNTAX.md) -- API command syntax for route injection (not yet implemented)

---

**Last Updated:** 2026-02-22
