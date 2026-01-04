# Configuration Syntax

## TL;DR (Read This First)

| Concept | Description |
|---------|-------------|
| **Style** | JUNOS-like: `{}` blocks, `;` terminators, `#` comments |
| **Sections** | `environment`, `process`, `template`, `peer` (top-level) |
| **Inheritance** | `inherit <template-name>` applies template config |
| **Pattern** | Registry/dispatch: `sectionParsers` map routes to handlers |
| **Key Types** | `Parser`, `Tokenizer`, `Scope`, `Validator` |
| **Migration** | `zebgp config migrate` converts ExaBGP → ZeBGP syntax |

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

### environment

ZeBGP-specific block for setting environment configuration from the config file.
See [ENVIRONMENT_BLOCK.md](ENVIRONMENT_BLOCK.md) for full documentation.

```
environment {
    log { level DEBUG; }
    tcp { port 1179; }
}
```

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

### peer (ZeBGP) / neighbor (ExaBGP)

BGP peer configuration. ZeBGP uses `peer`, ExaBGP uses `neighbor`.

```
peer <ip-address> {
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

    # API bindings
    api <process-name> { ... }

    # Static routes (use announce block)
    announce { ... }

    # FlowSpec
    flow { ... }

    # L2VPN
    l2vpn { ... }
}
```

**Migration:** `zebgp config migrate` converts `neighbor` → `peer`.

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
    ipv4/unicast;
    ipv4/multicast;
    ipv4/nlri-mpls;
    ipv4/mpls-vpn;
    ipv4 mcast-vpn;
    ipv4 flow;
    ipv4 flow-vpn;
    ipv4/mup;
    ipv6/unicast;
    ipv6/multicast;
    ipv6/nlri-mpls;
    ipv6/mpls-vpn;
    ipv6 mcast-vpn;
    ipv6 flow;
    ipv6 flow-vpn;
    ipv6/mup;
    l2vpn/vpls;
    l2vpn/evpn;
    bgpls bgpls;
    bgpls bgpls-vpn;
}
```

### ADD-PATH Section

```
add-path {
    ipv4/unicast;            # Both send and receive
    ipv4/unicast send;       # Send only
    ipv4/unicast receive;    # Receive only
}
```

### API Section (ZeBGP New Syntax)

```
# Named API binding (preferred)
api <process-name> {
    content {
        encoding json;       # json | text (default: inherit from process)
        format parsed;       # parsed | raw | full (default: parsed)
        attribute all;       # all | none | "as-path next-hop ..." (default: all)
        nlri ipv4/unicast;   # <afi> <safi>; (can have multiple)
        nlri ipv6/unicast;   # all | none | multiple nlri statements
    }
    receive {
        update;              # route announcements
        open;                # session open messages
        notification;        # error notifications
        keepalive;           # keepalive messages
        refresh;             # route refresh requests
        state;               # peer up/down events
        all;                 # shorthand for all message types
    }
    send {
        update;              # can inject routes
        refresh;             # can request route refresh
        all;                 # shorthand for all
    }
}
```

### API Section (Old Syntax - Still Supported)

```
# Anonymous API block (ExaBGP compatibility)
api {
    processes [ <process-name>, ... ];
    neighbor-changes;        # maps to receive { state; }
}
```

The migration tool (`zebgp config migrate`) converts old syntax to new:
```
# Old:
api { processes [ foo ]; neighbor-changes; }

# Becomes:
api foo { receive { state; } }
```

---

## Update Block (NOT IMPLEMENTED)

**Status:** Design specification - not yet implemented.

New unified syntax for announcing routes with chained attribute modifications.

### Syntax

```
update {
    <encoding> {
        attr {
            <set|add|del> <attribute>;
            ...
            nlri <family> add <nlri>... [del <nlri>...];
            ...
        }
        attr { ... }  # Each top-level attr block = clean state
    }
}
```

- `attr { }` block = clean state (no inherited attributes)
- `set <attr>` = replace value (`k = v`)
- `add <attr>` = append to list (`k.append(v)`) - community, large-community, extended-community
- `del <attr>` = remove from list (`k.remove(v)`)
- `nlri` uses current accumulated attributes

### Encodings

| Encoding | Attributes | NLRI |
|----------|------------|------|
| `text` | Parsed keywords | Prefixes |
| `hex` | Hex wire bytes | Hex wire bytes |
| `b64` | Base64 wire bytes | Base64 wire bytes |
| `cbor` | CBOR binary | CBOR binary |

### Example (text)

```
update {
    text {
        # Route group 1
        attr {
            set next-hop 10.0.0.1;
            set origin igp;
            set community [65000:1 65000:2];

            nlri ipv4/unicast add 1.0.0.0/24 2.0.0.0/24;

            add community [65000:3];
            nlri ipv4/unicast add 3.0.0.0/24;

            del community [65000:1];
            nlri ipv4/unicast add 4.0.0.0/24 del 5.0.0.0/24;
        }

        # Route group 2 - clean state
        attr {
            set next-hop 10.0.0.2;
            set med 200;

            nlri ipv4/unicast add 6.0.0.0/24;
        }
    }
}
```

### Example (hex)

```
update {
    hex {
        attr {
            set 400101400206020100001f94400304050607;
            # Spaces help track NLRI boundaries for UPDATE size splitting
            nlri ipv4/unicast add 18010a00 18020b00;
        }
    }
}
```

See `.claude/zebgp/api/UPDATE_SYNTAX.md` for full specification.

---

## Static Routes (Current Implementation)

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

## MPLS-VPN (L3VPN)

RFC 4364 L3VPN routes are configured under `announce { ipv4/ipv6 { mpls-vpn ... } }`.

### Syntax

```
announce {
    ipv4 {
        mpls-vpn <prefix> {
            rd <route-distinguisher>;
            label <mpls-label>;
            next-hop <ip>;
            origin igp;
            local-preference <int>;
            med <int>;
            as-path [ <asn>, ... ];
            community [ <community>, ... ];
            extended-community [ <ext-community>, ... ];
            originator-id <ip>;       # RFC 4456 route reflector
            cluster-list <ip> <ip>;   # RFC 4456 route reflector
        }
    }
    ipv6 {
        mpls-vpn <prefix> { ... }
    }
}
```

### Shorthand

```
announce {
    ipv4 {
        mpls-vpn 10.0.0.0/24 rd 100:100 label 1000 next-hop 192.168.1.1;
        mpls-vpn 10.0.1.0/24 rd 100:100 label 1001 next-hop 192.168.1.1 extended-community target:100:100;
    }
}
```

### Example

```
announce {
    ipv4 {
        mpls-vpn 10.0.0.0/24 {
            rd 65000:100;
            label 16000;
            next-hop 192.168.1.1;
            origin igp;
            local-preference 100;
            extended-community target:65000:100;
        }
    }
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

## MVPN (Multicast VPN)

RFC 6514 MVPN routes are configured under `announce { ipv4/ipv6 { mcast-vpn ... } }`.

### Route Types

| Type | Description |
|------|-------------|
| `source-ad` | Source Active A-D (Type 5) |
| `shared-join` | Shared Tree Join (Type 6) |
| `source-join` | Source Tree Join (Type 7) |

### Syntax

```
announce {
    ipv4 {
        mcast-vpn <route-type> {
            rd <route-distinguisher>;
            source-as <asn>;
            source <ip>;              # or rp <ip> for shared-join
            group <multicast-group>;
            next-hop <ip>;
            origin igp;
            local-preference <int>;
            med <int>;
            community [ <community>, ... ];
            extended-community [ <ext-community>, ... ];
            originator-id <ip>;       # RFC 4456 route reflector
            cluster-list <ip> <ip>;   # RFC 4456 route reflector
        }
    }
    ipv6 {
        mcast-vpn <route-type> { ... }
    }
}
```

### Example

```
announce {
    ipv4 {
        mcast-vpn source-ad {
            rd 100:100;
            source-as 65000;
            source 10.0.0.1;
            group 239.1.1.1;
            next-hop 192.168.1.1;
            extended-community target:100:100;
        }
        mcast-vpn shared-join {
            rd 100:100;
            rp 10.0.0.1;
            group 239.1.1.1;
            next-hop 192.168.1.1;
        }
    }
}
```

---

## MUP (Mobile User Plane)

SRv6 Mobile User Plane routes (draft-mpmz-bess-mup-safi) are configured under `announce { ipv4/ipv6 { mup ... } }`.

### Route Types

| Type | Description |
|------|-------------|
| `mup-isd` | Interwork Segment Discovery (Type 1) |
| `mup-dsd` | Direct Segment Discovery (Type 2) |
| `mup-t1st` | Type 1 Session Transformed (Type 3) |
| `mup-t2st` | Type 2 Session Transformed (Type 4) |

### Syntax

```
announce {
    ipv4 {
        mup <route-type> <prefix-or-addr> rd <rd> [options...];
    }
    ipv6 {
        mup <route-type> <prefix-or-addr> rd <rd> [options...];
    }
}
```

### Options

| Option | Used by | Description |
|--------|---------|-------------|
| `rd` | All | Route distinguisher |
| `teid` | T1ST, T2ST | Tunnel Endpoint ID |
| `qfi` | T1ST | QoS Flow Identifier |
| `endpoint` | T1ST | GTP tunnel endpoint |
| `source` | T1ST | Source address (optional) |
| `next-hop` | All | Next hop address |
| `extended-community` | All | Extended communities (e.g., `mup:10:10`) |
| `bgp-prefix-sid` | All | SRv6 Prefix SID |

### Examples

```
announce {
    ipv4 {
        # Interwork Segment Discovery
        mup mup-isd 10.0.0.0/24 rd 100:100 next-hop 192.168.1.1;

        # Type 1 Session Transformed
        mup mup-t1st 192.168.0.2/32 rd 100:100 teid 12345 qfi 9 endpoint 10.0.0.1 next-hop 192.168.1.1;

        # Direct Segment Discovery
        mup mup-dsd 10.0.0.1 rd 100:100 next-hop 192.168.1.1;
    }
    ipv6 {
        mup mup-isd 2001:db8::/48 rd 100:100 next-hop 2001::1 extended-community mup:10:10;
    }
}
```

---

## L2VPN / EVPN

### VPLS

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
        originator-id <ip>;       # RFC 4456 route reflector
        cluster-list <ip> <ip>;   # RFC 4456 route reflector
    }
}
```

### EVPN

EVPN routes use freeform syntax (complex format, passed through to ExaBGP-compatible parser):

```
announce {
    l2vpn {
        evpn <route-type> <nlri> <attributes>;
    }
}
```

See ExaBGP documentation for EVPN route type details.

---

## BGP-LS

BGP Link-State (RFC 7752) is supported as a family but route injection uses ExaBGP-compatible freeform syntax.

```
family {
    bgpls bgpls;
    bgpls bgpls-vpn;
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
        ipv4/unicast;
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

**Last Updated:** 2026-01-01
