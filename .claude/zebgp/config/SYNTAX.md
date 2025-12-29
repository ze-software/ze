# Configuration Syntax

## TL;DR (Read This First)

| Concept | Description |
|---------|-------------|
| **Style** | JUNOS-like: `{}` blocks, `;` terminators, `#` comments |
| **Sections** | `process`, `template`, `neighbor` (top-level) |
| **Inheritance** | `inherit <template-name>` applies template config |
| **Pattern** | Registry/dispatch: `sectionParsers` map routes to handlers |
| **Key Types** | `Parser`, `Tokenizer`, `Scope`, `Validator` |

**When to read full doc:** Config keywords, parsing bugs, new config sections.

---

**Source:** ExaBGP `configuration/` directory
**Purpose:** Document complete configuration file syntax

---

## Overview

ExaBGP configuration uses a JUNOS-like hierarchical syntax with sections, keywords, and values terminated by semicolons or braces.

---

## Basic Structure

```
# Comment
process <name> {
    ...
}

template <name> neighbor <name> {
    ...
}

neighbor <ip-address> {
    ...
}
```

---

## Section Types

### process

Defines an external process for API communication.

```
process <name> {
    run <path>;
    encoder json;           # or text (v4 only)
}
```

### template

Reusable configuration templates.

```
template <name> neighbor <name> {
    router-id <ip>;
    local-as <asn>;
    peer-as <asn>;
    ...
}
```

### neighbor

BGP peer configuration.

```
neighbor <ip-address> {
    inherit <template-name>;

    # Session settings
    router-id <ip>;
    local-address <ip>;
    local-as <asn>;
    peer-as <asn>;
    hold-time <seconds>;

    # Capabilities
    capability { ... }

    # Address families
    family { ... }

    # API
    api { ... }

    # Static routes
    static { ... }

    # FlowSpec
    flow { ... }

    # L2VPN
    l2vpn { ... }
}
```

---

## Neighbor Keywords

### Session

| Keyword | Type | Description |
|---------|------|-------------|
| router-id | IP | BGP router ID |
| local-address | IP | Local address to bind |
| local-as | ASN | Local AS number |
| peer-as | ASN | Peer AS number |
| hold-time | int | Hold time (seconds) |
| passive | bool | Passive mode (wait for connection) |
| listen | int | Port to listen on |
| connect | int | Port to connect to |
| ttl-security | int | TTL security hop count |
| md5-password | string | MD5 authentication password |
| md5-base64 | bool | Password is base64 encoded |
| group-updates | bool | Group updates for efficiency |
| auto-flush | bool | Auto-flush after each update |

### Capability Section

```
capability {
    asn4 enable;
    route-refresh enable;
    graceful-restart <seconds>;
    add-path receive;
    add-path send;
    multi-session enable;
    operational enable;
    aigp enable;
    extended-message enable;
    nexthop enable;
    software-version <string>;
}
```

### Family Section

```
family {
    ipv4 unicast;
    ipv4 multicast;
    ipv4 nlri-mpls;
    ipv4 mpls-vpn;
    ipv4 flow;
    ipv4 flow-vpn;
    ipv6 unicast;
    ipv6 mpls-vpn;
    ipv6 flow;
    ipv6 flow-vpn;
    l2vpn vpls;
    l2vpn evpn;
    bgpls bgpls;
    bgpls bgpls-vpn;
}
```

### ADD-PATH Section

```
add-path {
    ipv4 unicast;            # Both send and receive
    ipv4 unicast send;       # Send only
    ipv4 unicast receive;    # Receive only
}
```

### API Section

```
api {
    processes [ <process-name>, ... ];

    neighbor-changes;

    receive {
        parsed;
        packets;
        consolidate;
        open;
        update;
        keepalive;
        notification;
        refresh;
        operational;
    }

    send {
        packets;
        consolidate;
        open;
        update;
        keepalive;
        notification;
        refresh;
        operational;
    }
}
```

---

## Static Routes

```
static {
    route <prefix> {
        next-hop <ip>;
        origin igp;
        as-path [ <asn>, ... ];
        local-preference <int>;
        med <int>;
        community [ <community>, ... ];
        extended-community [ <ext-community>, ... ];
        large-community [ <large-community>, ... ];
        label [ <label>, ... ];
        rd <route-distinguisher>;
        split /<size>;
        watchdog <name>;
        withdraw;
    }
}
```

### Shorthand

```
static {
    route <prefix> next-hop <ip>;
    route <prefix> next-hop <ip> as-path [ 65001 65002 ];
}
```

---

## FlowSpec

```
flow {
    route <name> {
        match {
            source <prefix>;
            destination <prefix>;
            source-port <op><port>;
            destination-port <op><port>;
            port <op><port>;
            protocol <protocol>;
            next-header <protocol>;
            tcp-flags <flags>;
            icmp-type <type>;
            icmp-code <code>;
            fragment <flags>;
            dscp <value>;
            packet-length <op><length>;
            flow-label <value>;
        }
        then {
            accept;
            discard;
            rate-limit <bps>;
            redirect <rt>;
            redirect-next-hop;
            mark <dscp>;
            community [ <community>, ... ];
        }
        scope {
            neighbor <ip>;
        }
    }
}
```

### Match Operators

| Operator | Meaning |
|----------|---------|
| `=` | Equal |
| `>` | Greater than |
| `>=` | Greater or equal |
| `<` | Less than |
| `<=` | Less or equal |
| `!=` | Not equal |
| `&` | AND with previous |

### Examples

```
match {
    destination 10.0.0.0/8;
    destination-port =80;
    destination-port =443;
    protocol =tcp;
    tcp-flags =syn;
}
```

---

## L2VPN / EVPN

```
l2vpn {
    vpls <name> {
        endpoint <int>;
        base <int>;
        offset <int>;
        size <int>;
        next-hop <ip>;
        origin igp;
        as-path [ ... ];
        rd <rd>;
        route-target [ <rt>, ... ];
    }
}
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
enable
disable
true
false
```

### Origin

```
igp
egp
incomplete
```

---

## Inheritance

```
template mytemplate neighbor myneighbor {
    hold-time 90;
    family {
        ipv4 unicast;
    }
}

peer 192.168.1.2 {
    inherit myneighbor;
    local-as 65001;
    peer-as 65002;
}
```

---

## Multiple Values

### Lists

```
as-path [ 65001 65002 65003 ];
community [ 65001:100 65001:200 ];
processes [ process1, process2 ];
```

### Inline

```
route 10.0.0.0/8 next-hop 192.168.1.1 community [ 65001:100 ];
```

---

## Include (if supported)

```
include "/etc/exabgp/neighbors.conf";
```

---

## ZeBGP Implementation Notes

### Parser Architecture

```go
type Parser struct {
    tokenizer *Tokenizer
    scope     *Scope
    current   string  // Current section
}

func (p *Parser) Parse() (*Config, error) {
    for {
        tokens := p.tokenizer.NextLine()
        if tokens == nil {
            break
        }

        keyword := tokens[0]
        switch keyword {
        case "process":
            p.parseProcess(tokens)
        case "template":
            p.parseTemplate(tokens)
        case "neighbor":
            p.parseNeighbor(tokens)
        }
    }
}
```

### Section Dispatch

```go
var sectionParsers = map[string]func(*Parser, []string) error{
    "capability":   parseCapability,
    "family":       parseFamily,
    "add-path":     parseAddPath,
    "api":          parseAPI,
    "static":       parseStatic,
    "flow":         parseFlow,
    "l2vpn":        parseL2VPN,
}
```

### Value Validators

```go
type Validator interface {
    Validate(s string) (any, error)
}

var validators = map[string]Validator{
    "ip-address": &IPValidator{},
    "asn":        &ASNValidator{},
    "community":  &CommunityValidator{},
    // ...
}
```

---

**Last Updated:** 2025-12-19
