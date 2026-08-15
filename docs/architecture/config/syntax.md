# Configuration Syntax

## TL;DR (Read This First)

| Concept | Description |
|---------|-------------|
| **Style** | JUNOS-like: `{}` blocks, `;` terminators, `#` comments |
| **Top-Level** | `environment`, `plugin`, `bgp`, `ospf` |
| **BGP Block** | `bgp { group <name> { peer <ip> { ... } } }` - wraps all BGP config |
| **OSPF Block** | `ospf { areas { area <id> { ... } } interfaces { interface <name> { area <id> } } }` - native OSPFv2 config root |
| **Inheritance** | 3 levels: bgp globals, group defaults, peer overrides (deep-merge for containers) |
| **Schema** | YANG-driven: parser dispatches by node type (leaf, container, list, leaf-list) |
| **Migration** | `ze config migrate` converts ExaBGP syntax to ze-native |

**When to read full doc:** Config keywords, parsing bugs, new config sections.

---

**Purpose:** Document complete ze configuration file syntax

---

## Overview

Ze configuration uses a JUNOS-like hierarchical syntax with sections, keywords, and values terminated by semicolons or braces. The parser is YANG-driven: each config node's type (leaf, leaf-list, container, list) determines how it is parsed. No custom `ze:syntax` annotations are used in ze-native config.
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- module ze-bgp-conf structure -->

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

### Inactive prefix (deactivate / activate)

Any structural statement may be prefixed with `inactive: ` to mark it
inactive. The node stays in the file and round-trips through save/load,
but is removed by `PruneInactive` before consumers see the tree, so the
runtime treats it as absent.

```
bgp {
    inactive: router-id 10.0.0.1;       # leaf -- value preserved verbatim
    inactive: peer scratch { ... }      # list entry -- whole subtree skipped
    inactive: filter { ... }            # container -- whole subtree skipped
    filter {
        import [ no-self-as reject-bogons ];
        inactive: import no-self-as;    # single leaf-list member
    }
}
```

A single deactivated **leaf-list member** keeps its bare value in the
leaf line and is declared inactive by a follow-up `inactive: <leaf>
<member>` statement (above: `no-self-as` stays listed, marked
inactive). The member statement only has this meaning when the value is
already an active member of the leaf-list; otherwise `inactive: <leaf>
<values...>` keeps its whole-leaf meaning. The internal
`inactive:<member>` token never appears in serialized output -- it
would fail per-item type validation (e.g. `ip-address`) on reparse.

The CLI verbs `ze config deactivate <file> <path>` and `ze config
activate <file> <path>` add and remove the prefix. The TUI accepts the
same `deactivate <path>` / `activate <path>` commands while editing.
The mechanism is engine-level: every YANG node is deactivatable, no
schema annotation is required.

In the **set / single-line format**, the `nop` keyword replaces `set`
on a deactivated line. "nop" means "no operation": the config entry
exists in the file but produces no operational effect. Toggling
activation is a 3-byte in-place edit (`set` <-> `nop`):
<!-- source: internal/component/config/setparser.go -- cmdNop, parseNop -->

```
nop bgp router-id 10.0.0.1
```

For leaf-lists with per-member deactivation, each member gets its own
line: active members use `set`, deactivated members use `nop`:

```
set system name-server 8.8.8.8
nop system name-server 1.1.1.1
```

Container deactivation uses a structural `nop <container-path>` line
before its children. Children retain their own `set`/`nop` state:

```
nop bgp
set bgp router-id 10.0.0.1
set bgp neighbor 192.0.2.1 peer-as 65001
```

**Backward compatibility:** the `inactive <path>` keyword from older
configs is still accepted by the parser. On save, `inactive` lines
are migrated to the `nop` form automatically.

Coexists with per-feature `disable` semantics (e.g. a peer's
`admin-state disable` leaf), which are protocol-aware and operationally
distinct from "no such node". Use `inactive:` when you want the node
absent at apply time; use the per-feature disable when you want the
protocol to know about the node but treat it as administratively down.

---

### Schema Stamp

Committed config files carry a schema stamp as the first line:

```
# ze-schema: 1
```

The stamp is a comment (ignored by all parsers) that records which schema
revision produced the file. It is re-emitted from a binary constant on every
commit, not stored in the YANG tree. Files without a stamp are treated as
revision 0 (pre-stamping).

#### Downgrade Recovery

When ze starts and fails to parse `config.conf`, it checks whether the stamp
is newer than the binary's own `SchemaStamp`. If so, the binary was downgraded
and the config was written by a newer version. Ze walks the rollback directory
(newest-first), skipping files with stamps above its own, and attempts a full
parse on each candidate. The first rollback file that parses successfully
becomes the active config. Ze writes it back to `config.conf` (stamped with
the current binary's revision) so the running config matches what is on disk.

| Step | What happens |
|------|-------------|
| 1 | `LoadConfig` fails, stamp on `config.conf` > binary's `SchemaStamp` |
| 2 | Walk `rollback/` newest-first, skip files with stamp > `SchemaStamp` |
| 3 | Attempt full parse on each candidate (stamp is a hint, parse is the gate) |
| 4 | First successful parse: write it back to `config.conf` with current stamp |
| 5 | If none parse: refuse to start with a clear error |

Recovery runs at startup only, not on SIGHUP reload. A reload failure during
runtime surfaces as an error rather than silently reverting to old config.

<!-- source: internal/component/config/stamp.go -->

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
    session { asn { local <asn>; } }        # BGP-level local AS (inherited by all peers)

    group <name> {
        <peer-fields>                       # Group-level defaults (shared by all peers in group)
        peer <name> {
            <peer-fields>                   # Peer-level overrides
        }
    }

    peer <name> {                           # Standalone peer (no group inheritance)
        <peer-fields>
    }
}
```
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- container bgp, list peer -->

---

## Section Types

### environment

Ze-specific block for setting environment configuration from the config file.
See [environment-block.md](environment-block.md) for full documentation.

```
environment {
    log { level DEBUG; }
    tcp { attempts 3; }
}
```
<!-- source: internal/component/config/environment.go -- ListenEndpoint, ParseCompoundListen, parseOneEndpoint -->

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
| timeout | duration | Startup stall timeout (e.g., `5s`, `1m`, `500ms`): how long a startup stage may go with no plugin completing one, not a budget for the stage itself. Default: 5s. 0 = use default. Negative rejected. |
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- list process, leaf run, leaf encoding -->

**Timeout semantics:** During startup, all plugins synchronize at each stage. The timeout controls how long this plugin waits for all plugins to complete each stage. With multiple plugins, use the same timeout for all, or set the longest timeout on all plugins to avoid fast plugins timing out while waiting for slow ones.

### ospf

Top-level native OSPFv2 configuration. The `ospf` container is a YANG config
root and auto-loads the OSPF edge plugin. Interfaces bind to areas explicitly by
setting `interfaces/interface/area`; Ze does not use FRR-style `network <prefix>
area <id>` matching. `network-type` accepts `broadcast`, `point-to-point`, and
`loopback`; passive and loopback records do not open raw sockets. The NSM
consumes `mtu-ignore`, `retransmit-interval`, `transmit-delay`, and dead timers
from the same interface list.

```
ospf {
    router-id 10.0.0.1
    areas {
        area 0.0.0.0 { area-type normal; }
    }
    interfaces {
        interface eth0 {
            area 0.0.0.0
            network-type point-to-point
            mtu-ignore true
        }
        interface lo0 {
            area 0.0.0.0
            network-type loopback
        }
    }
}
```

`router-id` uses `ze:validate "ospf-router-id"`. `area-id` and interface `area`
use `ze:validate "ospf-area-id"` and are checked again by the in-process config
verifier so an interface cannot reference an undeclared area.
<!-- source: internal/plugins/ospf/yang/ze-ospf-conf.yang -- ospf container, area and interface lists -->
<!-- source: internal/plugins/ospf/register.go -- InProcessConfigVerifier -->

### Peer Groups

Peers are organized into named groups under `bgp`. Groups provide shared defaults inherited by all member peers.

#### Group Structure

```
bgp {
    group <name> {
        <peer-fields>               # Group-level defaults
        peer <name> {
            <peer-fields>           # Peer-level overrides
        }
    }
}
```

| Element | Type | Purpose |
|---------|------|---------|
| `group` | list (key: name) | Named collection of peers with shared defaults |
| `peer` inside group | list (key: name) | Peer with optional overrides of group defaults |
| `peer` at bgp level | list (key: name) | Standalone peer (no group inheritance) |
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- list group, list peer -->

#### 3-Level Inheritance

| Priority | Source | Scope |
|----------|--------|-------|
| Lowest | `bgp` block globals | `session { asn { local; } }`, `router-id` -- inherited by all peers |
| Middle | Group defaults | All `peer-fields` set on the group |
| Highest | Peer overrides | All `peer-fields` set on the peer |
<!-- source: internal/component/bgp/config/resolve.go -- ResolveBGPTree, deepMergeMaps -->

Containers (like `capability`, `timer`) deep-merge at key level -- both group and peer capabilities are combined. Leaves (like `hold-time` inside `timer`) override -- peer value wins over group value.

#### Example

Every peer below gives `remote` and `local` DIFFERENT addresses, and that is a
property of BGP rather than a style choice: the two ends of one session are two
speakers, so they hold two addresses. A configuration naming one address for both
describes a session that cannot exist, and Ze refuses to advertise a route whose
NEXT_HOP is the peer's own address (RFC 4271 Section 5.1.3), so such a peer
receives nothing while looking established. The refusal is
`originatedNextHopIsPeerOwn` and `egressNextHopIsPeerOwn`, both in
`internal/component/bgp/reactor/forward_next_hop.go`.

```
bgp {
    router-id 1.2.3.4
    session { asn { local 65000; } }

    group rr-clients {
        timer { receive-hold-time 180; }
        session { capability { route-refresh enable; } }

        peer router-east {
            connection {
                remote { ip 10.0.0.1; }
                local { ip 10.0.0.254; }
            }
            session { asn { remote 65001; } }
        }
        peer client-b {
            connection {
                remote { ip 10.0.0.2; }
                local { ip 10.0.0.254; }
            }
            session { asn { remote 65002; } }
            timer { receive-hold-time 90; }    # Overrides group's 180
        }
    }

    group edge-peers {
        timer { receive-hold-time 30; }
        peer edge-gw {
            connection {
                remote { ip 192.168.1.1; }
                local { ip 192.168.1.254; }
            }
            session { asn { remote 64500; } }
        }
    }
}
```
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- peer-fields grouping -->

#### Peer Name Rules

| Rule | Detail |
|------|--------|
| Required | Every peer must have a name (it is the list key) |
| Unique | Two peers with the same name produce a config validation error |
| First character | ASCII letter, digit, or underscore (`[a-zA-Z0-9_]`) |
| Subsequent characters | ASCII letter, digit, underscore, hyphen, or dot (`[a-zA-Z0-9_.-]`) |
| CLI usage | `peer <name> <command>` selects the peer |
<!-- source: internal/component/bgp/config/resolve.go -- validatePeerName -->

#### Migration

`ze config migrate` converts old syntax:
- `template { bgp { peer <pattern> { inherit-name X; } } }` becomes `group X { }`
- `inherit X` in peers moves the peer into `group X`
- `neighbor` blocks become `peer` blocks inside groups

### bgp

Container for all BGP-related configuration (peers, groups, global settings).

```
bgp {
    router-id <ip>;           # Global router-id (inherited by all peers)
    session { asn { local <asn>; } }  # Global local AS (inherited by all peers)

    group <name> {
        # Group-level defaults (inherited by all peers in group)
        session { asn { remote <asn>; } }
        timer { receive-hold-time <seconds>; connect-retry <seconds>; }
        session { capability { ... } }
        session { family { ... } }

        peer <name> {
            # Connection (transport-level)
            connection {
                remote { ip <ip>; port <port>; connect <bool>; }
                local { ip <ip>; port <port>; accept <bool>; }
                md5 { password <string>; ip <ip>; }
                ttl { max <0-255>; set <0-255>; min <0-255>; }
                link-local <bool>;
            }

            # Session (BGP protocol)
            session {
                asn { local <asn>; remote <asn>; }
                router-id <ip>;
                link-local <ipv6>;
                family { ... }
                capability { ... }
            }

            # Behavior (operational knobs)
            behavior {
                group-updates <bool>;
                manual-eor <bool>;
                auto-flush <bool>;
            }

            # Other
            description <string>;
            timer { receive-hold-time <seconds>; send-hold-time <seconds>; connect-retry <seconds>; }
            rib { adj { in <bool>; out <bool>; } out { ... } }
            attach process <plugin-name> { ... }
            update { ... }
        }
    }
}
```
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- container bgp structure -->

**Migration:** `ze config migrate` converts old syntax:
- `neighbor` to `bgp { group <name> { peer } }`
- `template { inherit-name X }` to `group X { }`
- `inherit X` in peers moves peer into `group X`

---

## Peer Keywords

Peer configuration is organized into nested containers by concern.
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- peer-fields grouping -->

### connection (transport-level)

| Keyword | Type | Description |
|---------|------|-------------|
| `connection { local { ip; port; accept; } }` | container | Local address (`auto` or IP), bind port, accept inbound connections (default: true) |
| `connection { remote { ip; port; connect; } }` | container | Peer IP address, connection port, initiate outbound connections (default: true) |
| `connection { md5 { password; ip; } }` | container | TCP MD5 authentication (RFC 2385) |
| `connection { ttl { max; set; min; } }` | container | GTSM max (RFC 5082), outgoing TTL, minimum incoming TTL |
| `connection { link-local; }` | bool | Auto-discover IPv6 link-local for TCP |

### session (BGP protocol)

| Keyword | Type | Description |
|---------|------|-------------|
| `session { asn { local; remote; } }` | container | Local and remote AS numbers |
| `session { router-id; }` | IP | BGP router ID override |
| `session { link-local; }` | IPv6 | IPv6 link-local address for MP_REACH_NLRI next-hop (RFC 2545) |
| `session { family { ... } }` | list | Address families with per-family prefix enforcement |
| `session { capability { ... } }` | container | BGP capabilities (see below) |
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- add-path inside capability container -->

### behavior (operational knobs)

| Keyword | Type | Description |
|---------|------|-------------|
| `behavior { group-updates; }` | bool | Group UPDATE messages for efficiency (default: true) |
| `behavior { manual-eor; }` | bool | Manual End-of-RIB control |
| `behavior { auto-flush; }` | bool | Auto-flush routes |

### Other peer-level keywords

| Keyword | Type | Description |
|---------|------|-------------|
| `description` | string | Peer description |
| `timer { receive-hold-time; send-hold-time; connect-retry; }` | container | Timer settings (defaults: 90s, 0/auto, 120s) |
| `rib { adj { in; out; } out { ... } }` | container | RIB configuration (adj-rib-in/out, outbound batching) |
| `attach process <name> { ... }` | list | Plugin process bindings |
| `update { ... }` | container | Route announcements |
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- grouping peer-fields, containers connection, session, behavior -->

### Capability Section

All capabilities support a four-mode vocabulary:

| Mode | Advertise? | Enforcement | Aliases |
|------|------------|-------------|---------|
| `enable` | Yes | None | `true` |
| `disable` | No | None | `false` |
| `require` | Yes | Reject peer if capability missing | |
| `refuse` | No | Reject peer if capability present | |
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- container capability, enum enable/disable/require/refuse -->

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
        long-lived-stale-time 3600;  # RFC 9494 LLGR period (0-16777215 seconds)
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

**ADD-PATH** -- unified block with default direction, per-family overrides, and PATHS-LIMIT:

```
capability {
    add-path {
        direction send/receive;           # default for all families
        limit 10;                         # default PATHS-LIMIT
        family {
            ipv4/unicast { mode require; } # per-family mode
        }
    }
}
```

The `direction` and `limit` on the container are inherited by all negotiated families. Per-family entries override direction, limit, or mode.
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- add-path capability container -->

**Defaults:** ASN4 defaults to `enable`. All other capabilities are absent (opt-in) -- they only participate in negotiation when explicitly configured.
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- leaf asn4 default true -->

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
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- list family -->

### ADD-PATH Section

ADD-PATH and PATHS-LIMIT are configured inside `session > capability > add-path`:

```
capability {
    add-path {
        direction send/receive;      # default for all families
        limit 10;                    # default PATHS-LIMIT (inherited)
        family {
            ipv4/unicast {
                direction send;      # per-family override
                limit 5;            # per-family limit override
                mode require;       # reject peer if missing
            }
            ipv6/unicast             # inherits direction + limit from container
        }
    }
}
```

The `direction` and `limit` leaves on the container apply to all negotiated families unless overridden per-family. Per-family `mode` supports `enable` (default), `require`, `refuse`, `disable`.
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- container add-path, list family, leaf direction, leaf limit, leaf mode -->

### Process Section

```
# Named process binding (preferred)
attach process <plugin-name> {
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
| `update` | Route announcements |
| `open` | Session open messages |
| `notification` | Error notifications |
| `keepalive` | Keepalive messages |
| `refresh` | Route refresh requests |
| `state` | Peer up/down events |
| `negotiated` | Capability negotiation results |
| `*` | Every event type, in both directions |
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- leaf-list receive, ze:validate receive-event-type -->

Plugins may register additional event types (e.g., `rpki`, `update-rpki`) that can also appear in receive lists. These are validated at runtime against the plugin registry.

**Direction is part of the type.** A plain type means both directions. A `-received` or `-sent` suffix means one: `receive [ update-received ]` asks for the UPDATEs the peer sends to ze, and `receive [ update-sent ]` asks for the ones ze sends to the peer. The two together mean both, which is what the plain type already says.

A token is resolved against the event registry whole before any suffix is cut, so a plugin type whose name ends in `-sent` keeps its name and is never read as a direction.

**`sent` on its own is not a type** and is refused. It named a direction, and the direction now belongs to the type it applies to, so `receive [ sent state ]` becomes `receive [ update-sent state ]`.

**`all` is not accepted.** `*` says the same thing without reading like a type name, and it says it deliberately: a config that names `*` accepts every event type a plugin registers later, which is exactly what the word `all` used to do by accident.

**`receive` decides delivery, and an absent block feeds nothing.** ze builds a
peer-to-process index from the resolved settings of every peer and consults it
on each peer-scoped event. A process is handed an event when the peer's block
grants that type in that direction AND the process subscribed to it. A peer
that states no block for a running process feeds it nothing, whatever the
process asked for at startup. The two halves are reported when they disagree,
at plugin ready and after every config apply.

<!-- source: internal/component/plugin/server/delivery_graph.go -- DeliveryGraph, PeerScopedProcs -->
<!-- source: internal/component/plugin/server/delivery_reconcile.go -- deliveryDisagreements -->

**A group's block reaches its members, and a member's list replaces it.**
`attach process` lives in the `peer-fields` grouping, so it is legal at
`bgp/group`, `bgp/group/peer` and `bgp/peer`. `receive` and `send` are
leaf-lists outside `cumulativePaths`, so a member that restates a process block
REPLACES the group's list for that member rather than adding to it. The index
is built after that merge, which is why a peer a dynamic group creates carries
its group's blocks under the address its connection arrived from: no config
document names its generated `dyn-<address>` identity, and none needs to.

<!-- source: internal/component/bgp/config/resolve.go -- ResolveBGPTree -->
<!-- source: internal/component/bgp/reactor/reactor_dynamic.go -- buildDynamicPeerSettings -->

#### Send enum values

| Value | Description |
|-------|-------------|
| `update` | Can inject routes |
| `refresh` | Can request route refresh |
| `*` | Every message type, including every one a plugin registers later |
<!-- source: internal/core/bgp/events/events.go -- BaseSendTypes -->
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- leaf-list send, ze:validate send-message-type -->

`update` and `refresh` are the two BASE types. A plugin registers more through
`Registration.SendTypes`, and naming one in a send list auto-loads the plugin
that enables it: `send [ enhanced-refresh ]` starts bgp-route-refresh, which
sends the RFC 7313 BoRR and EoRR markers. `*` names no type, so it auto-loads
nothing.
<!-- source: internal/component/plugin/registry/registry.go -- Registration.SendTypes -->

A direction suffix has no meaning in a send list, because every send type is sent.

**`all` is not accepted.** List send types explicitly, or write `*`.

Invalid enum values are rejected at parse time.

**`send` is enforced, not documentation.** A process reaches only the peers that
attach it with the type the command puts on the wire. A peer the command names
but the permission refuses is dropped and reported; a command whose every peer
refuses fails. `update` covers an announce, a withdrawal, an End-of-RIB marker, a
named commit, a cache forward and a stored-route relay; `refresh` covers a route
refresh, a BoRR, an EoRR and a soft clear. A command an operator types is not a
process and is not gated.

The reactor applies it in two places, and a peer-naming command reaches one of
them. Six commands resolve a peer SELECTOR and are gated there. Four more name
their peers directly, without a selector, and each applies the same check:
`cache forward`, the `forward-cached` and `relay-stored-route` plugin RPCs, and
`peer <addr> raw`.

**Raw is the one exception to the type table, and it is gated on ATTACHMENT.**
`bgp peer <addr> raw ...` carries a whole BGP message the caller chose, an OPEN
or a NOTIFICATION included, so no `send` type describes it and the `send` list
has no word to write. What ze requires instead is that the peer attaches the
process at all: a bare `attach process <name> { }` is enough, and a peer that
names the process nowhere refuses it. Write the block for any program that
injects raw messages.

<!-- source: internal/component/bgp/reactor/send_permission.go -- Peer.maySend, sendOrigin, filterPermittedPeers -->
<!-- source: internal/component/bgp/reactor/reactor_api.go -- getMatchingPeersSel, SendRawMessage -->

**A truthful `send [ update ]` moves the peer's End-of-RIB.** The session's
initial sync counts the bindings that carry it and holds the marker until each
one has sent `plugin session ready`, so the marker means what RFC 4724 Section 2
says it means. A program granted the permission that never sends that signal
delays the marker by the sync timeout instead. ze's own plugins send it
(bgp-rib, bgp-watchdog); a plugin author who originates routes at peer-up
should.

---

## Filter Block

Route filtering via named filter instances. Appears in `peer-fields` grouping
(group and peer levels) and the `bgp` container (global level).

```
filter {
    import [ rpki:validate community:scrub ];
    export [ aspath:prepend ];
}
```

| Keyword | Type | Description |
|---------|------|-------------|
| `import` | leaf-list of string | Import filter chain. Values are `<plugin>:<filter>` references |
| `export` | leaf-list of string | Export filter chain. Values are `<plugin>:<filter>` references |

Chains are cumulative across config levels (bgp > group > peer). Mandatory
filters (e.g., `rfc:otc`) always run first and cannot be configured. Default
filters (e.g., `rfc:no-self-as`) run unless overridden by a user filter that
declares `overrides`.

Validation checks local policy filter names, canonicalizes short filter-type
references through the plugin registry, and leaves explicit `<plugin>:<filter>`
references for runtime dispatch. Runtime calls use the `filter-update` callback
on the selected plugin.

<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- policy and filter containers -->
<!-- source: internal/component/bgp/config/redistribution.go -- redistribution config parsing -->
<!-- source: internal/component/bgp/config/filter_registry.go -- local policy filter validation -->
<!-- source: internal/component/plugin/registry/registry.go -- FilterTypes registry -->

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
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- container attribute, leaf origin, leaf next-hop, leaf-list community -->

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
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- list nlri, leaf content -->

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

FlowSpec routes (RFC 8955) use the same `update { nlri { } }` syntax. The legacy `flow { route { match {} then {} } }` block is not supported -- use `ze config migrate` to convert ExaBGP-format FlowSpec configs.

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
| `rate-limit:<bps>` | Rate limit in bytes per second |
| `rate-limit:<bps>:bytes` | Explicit bytes-per-second form, canonicalized to `rate-limit:<bps>` |
| `rate-limit:<pps>:packets` | Rate limit in packets per second |
| `redirect:<asn>:<value>` | Redirect to VRF |
| `redirect-to-nexthop <ip>` | Redirect to IP (RFC 7674) |
| `redirect-to-nexthop-draft` | Redirect to next-hop (draft) |
| `copy-to-nexthop` | Copy to next-hop |
| `action sample-terminal` | Sampling action |
| `mark <dscp>` | Set DSCP value |

Legacy ExaBGP 5.0 packet-rate syntax `rate-limit-packets:<pps>` is still accepted during parsing. ExaBGP's explicit byte form `rate-limit:<bps>:bytes` is also accepted and normalized to bare `rate-limit:<bps>`.

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
request bgp watchdog announce <name>   # send all routes in pool to peers
request bgp watchdog withdraw <name>   # withdraw all routes in pool from peers
```
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- container watchdog, leaf name, leaf withdraw -->

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
<!-- source: internal/component/config/environment.go -- ParseBoolStrict -->

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
    router-id 1.2.3.4                   # BGP-level global
    session { asn { local 65000; } }     # BGP-level global

    group rr-clients {
        timer { receive-hold-time 90; }  # Group default
        session {
            family { ipv4/unicast { prefix { maximum 1000000; } } }
        }

        peer client-a {
            connection {
                remote { ip 192.168.1.2; }
                local { ip 192.168.1.1; }
            }
            session { asn { remote 65002; } }
        }
    }
}
```

### Key Concepts

| Concept | Description |
|---------|-------------|
| BGP globals | `session { asn { local; } }` and `router-id` at bgp level, inherited by all peers |
| Group defaults | Any `peer-fields` set on the group, inherited by all peers in that group |
| Peer overrides | Any `peer-fields` set on the peer, takes highest precedence |
| Deep merge | Containers (e.g., `session`, `timer`) merge keys from both group and peer |
| Leaf override | Scalar values (e.g., `receive-hold-time` inside `timer`) at peer level replace group values |
<!-- source: internal/component/bgp/config/resolve.go -- ResolveBGPTree, deepMergeMaps -->

### Standalone Peers

Peers directly under `bgp` (not inside a group) inherit only BGP-level globals:

```
bgp {
    session { asn { local 65000; } }

    peer my-peer {
        connection {
            remote { ip 10.0.0.5; }
            local { ip 10.0.0.1; }
        }
        session { asn { remote 65001; } }
        timer { receive-hold-time 180; }
    }
}
```

---

## Custom YANG Extensions

Ze defines custom extensions in `ze-extensions.yang` that control config parsing, validation, and UI behavior:

| Extension | Argument | Applied to | Purpose |
|-----------|----------|------------|---------|
| `ze:syntax` | mode | any node | Parser syntax mode (flex, freeform, inline-list, etc.) |
| `ze:key-type` | type | inline-list | Key type for inline lists |
| `ze:sensitive` | -- | leaf | Marks leaf as containing sensitive data |
| `ze:validate` | name | leaf, leaf-list | References a custom validator function |
| `ze:cumulative` | -- | leaf-list | Accumulate values across config inheritance levels |
| `ze:decorate` | name | leaf | Attaches display-time decorator for web UI |
| `ze:required` | path | list | Descendant field must have value after config inheritance resolution. Path argument is mandatory; bare `ze:required;` is rejected at schema load (use `mandatory true` for leaf-level required). Enforced generically for any list at validate, editor, and startup. |
| `ze:suggest` | path | list | Field shown in creation form with inherited defaults |
| `ze:listener` | -- | list | Marks a named `list server { key "name"; }` as a network listener. `CollectListeners` walks every enabled service with this extension at parse time and feeds `ValidateListenerConflicts`, which rejects overlapping `ip:port` pairs with a message naming both services. Covers web, ssh, mcp, looking-glass, prometheus, plugin hub, and api-server rest/grpc. |
| `ze:command` | handler | container | Marks a config-false container as executable CLI command |
| `ze:edit-shortcut` | -- | command | Available in edit mode as shortcut |
| `ze:display-key` | -- | leaf | Display label for keyless list entries in web UI |
| `ze:hidden` | value | leaf | Hidden from config display and web editor |
| `ze:allow-unknown-fields` | -- | container | Container accepts arbitrary key-value pairs |
| `ze:route-attributes` | -- | inline-list | Node accepts standard BGP route attributes |
| `ze:os` | goos | any node | Restricts node to a specific OS (any GOOS value); excluded from schema on other platforms |

<!-- source: internal/component/config/yang/modules/ze-extensions.yang -- all ze: extensions -->

---

## Related

- [ExaBGP Legacy Syntax](exabgp-syntax.md) -- `static`, `announce`, `flow` blocks accepted for migration
- [API Update Syntax](../api/update-syntax.md) -- API command syntax for route injection (not yet implemented)

---

**Last Updated:** 2026-04-02
