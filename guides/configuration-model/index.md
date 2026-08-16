# Configuration

Ze uses a JUNOS-like hierarchical configuration format.

## Syntax Rules

| Element | Syntax | Example |
|---------|--------|---------|
| Blocks | `name { ... }` | `bgp { ... }` |
| Values | `key value` or `key value;` | `router-id 1.2.3.4` |
| Comments | `#` to end of line | `# this is a comment` |
| Lists | `[ item1 item2 ]` | `receive [ update state ]` |
| Strings | Unquoted or `"double quoted"` | `run "/usr/bin/my-plugin"` |
| Terminators | Optional semicolons (`;`) | Both `router-id 1.2.3.4` and `router-id 1.2.3.4;` work |
| Inline blocks | `name { key value; key value; }` | `remote { ip 10.0.0.1; as 65001; }` |

Indentation is not significant. Unknown keys are rejected with a suggestion for the closest valid key.
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- BGP config YANG schema; internal/component/bgp/config/resolve.go -- ResolveBGPTree -->

## File Format

```
# Global BGP settings
bgp {
    router-id 1.2.3.4;
    local { as 65000; }

    # Peer group with shared defaults
    group upstream {
        timer {
            receive-hold-time 180;
        }
        remote { connect false; }   # passive: don't initiate outbound

        capability {
            asn4;
            route-refresh;
        }

        family {
            ipv4/unicast;
            ipv6/unicast;
        }

        peer transit-a {
            remote { ip 10.0.0.1; as 65001; }
        }

        peer transit-b {
            remote { ip 10.0.0.2; as 65002; }
            timer {
                receive-hold-time 90;    # overrides group value
            }
        }
    }

    # Standalone peer (no group)
    peer internal {
        remote { ip 192.168.1.1; as 65000; }
        local { ip 192.168.1.2; as 65000; }
        router-id 192.168.1.2

        family {
            ipv4/unicast
        }
    }
}
```

By default Ze enforces AS-wide BGP Identifier uniqueness: the first peer of an AS to
present a router-id holds it, and a second peer of the same AS presenting the same
router-id is rejected with a Bad BGP Identifier NOTIFICATION (RFC 6286 Section 2.1
makes this only a SHOULD). The identifier is claimed while the peer's OPEN is
validated and released when its session ends, so which peer wins does not depend on
which one establishes first. Set `bgp { session { allow-shared-router-id true; } }`
to accept the duplication when it is intentional, for example one anycast speaker
(AS112) peering over both IPv4 and IPv6 with the same router-id. When enabled, Ze
performs no AS-wide router-id uniqueness check at all.

<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang — allow-shared-router-id; internal/component/bgp/reactor/routerid_unique.go — routerIDClaims; internal/component/bgp/reactor/peer.go — validateOpen -->

`allow-shared-router-id` does not relax RFC 6286 Section 2.2, which Ze always
enforces on reception: an OPEN whose BGP Identifier is `0.0.0.0`, or whose BGP
Identifier is Ze's own router-id when the peer is in the same AS (an internal peer),
is answered with OPEN Message Error / Bad BGP Identifier and the connection is
closed. The same identifier from an *external* peer is accepted, as the RFC
requires; if that leads to a connection collision, Section 2.3 preserves the
connection initiated by the speaker with the larger AS number. Ze also refuses to
start with its own `router-id 0.0.0.0`, since Section 2.1 defines the BGP Identifier
as a non-zero integer and every conformant peer would reject such an OPEN.

<!-- source: internal/component/bgp/reactor/session_open_validation.go — validateOpenIdentifier; internal/component/bgp/message/open.go — ValidateBGPIdentifier; internal/component/bgp/reactor/session.go — DetectCollision; internal/component/bgp/reactor/config.go — parseRouterID -->

## OSPF

The top-level `ospf` block enables Ze's native OSPF engine. The root container
configures OSPFv2 for IPv4; `ospf { address-family ipv6 { ... } }` enables the
OSPFv3 IPv6 instance on the same engine. Interface and neighbor state, LSDB
flooding, SPF, inter-area logic, NSSA policy, redistribution, and route
installation are shared. The wire codec, raw transport, and prefix strategy are
address-family specific.

For IPv4, the engine opens active non-passive broadcast and point-to-point
interfaces through the raw IPv4 transport. For IPv6, it uses the OSPFv3 raw IPv6
transport (`ff02::5` / `ff02::6`) and the OSPFv3 prefix model
(Intra-Area-Prefix, Link, AS-External, and NSSA LSAs). Both families install
selected routes through the shared Loc-RIB -> sysrib -> fibkernel path.
<!-- source: internal/plugins/ospf/register.go -- registerOSPF, runOSPFEngine -->
<!-- source: internal/plugins/ospf/neighbor/table.go -- Hello -->
<!-- source: internal/plugins/ospf/neighbor/dd.go -- HandleDBDesc -->
<!-- source: internal/plugins/ospf/neighbor/lsreq.go -- HandleLSUpdate -->
<!-- source: internal/plugins/ospf/lsdb/lsdb.go -- LSDB, Summary -->
<!-- source: internal/plugins/ospf/lsdb/flooding.go -- ReceiveUpdate, ReceiveAck -->
<!-- source: internal/plugins/ospf/yang/ze-ospf-conf.yang -- module ze-ospf-conf -->

```
ospf {
    router-id 10.0.0.1
    reference-bandwidth 100000
    maximum-paths 8

    areas {
        area 0.0.0.0 {
            area-type normal
        }
    }

    interfaces {
        interface eth0 {
            area 0.0.0.0
            network-type point-to-point
            hello-interval 10
            dead-interval 40
            priority 1
        }
        interface lo0 {
            area 0.0.0.0
            network-type loopback
        }
    }
}
```

Area binding is per interface: `interfaces/interface/area` must reference a
declared `areas/area`. `network-type` accepts `broadcast`, `point-to-point`, or
`loopback`; loopback records send no Hellos and open no raw socket. The NSM
consumes interface `mtu-ignore`, retransmit, transmit-delay, and dead-interval
settings during DD exchange and inactivity tracking. `router-id` must be dotted
quad when configured; if it is omitted, Ze derives one from the highest loopback
IPv4 address, then the highest interface IPv4 address. Numeric leaves are
range-checked by YANG, including `maximum-paths 1..32`, interface cost
`1..65535`, priority `0..255`, and OSPF metrics `0..16777215`.
<!-- source: internal/plugins/ospf/config.go -- parseOSPFConfig, validateConfig, deriveRouterIDFromInterfaces -->

The IPv6 address family mirrors the area and interface model under
`address-family ipv6`. Presence of the `ipv6` container enables the OSPFv3
instance; `instance-id` defaults to 0 and is used for RFC 5340 per-link
demultiplexing.
<!-- source: internal/plugins/ospf/yang/ze-ospf-conf.yang -- address-family ipv6 -->
<!-- source: internal/plugins/ospf/config.go -- parseOSPFConfig, v6AFConfig, v6Families -->

The `router-information` container advertises this router's optional capabilities
in the RFC 7770 Router Information LSA. `enabled true` originates it; the `scope`
leaf-list (`link` / `area` / `as`, multiple allowed) selects the flooding scope and
defaults to area + AS when enabled with no scope listed. It applies to both address
families (a top-level container drives the OSPFv3 family unless that family sets its
own); OSPFv2 additionally requires `opaque true`, since the OSPFv2 RI LSA is an opaque
LSA. See the OSPF guide's Router Information section for the advertised capability bits.
<!-- source: internal/plugins/ospf/yang/ze-ospf-conf.yang -- router-information, scope -->
<!-- source: internal/plugins/ospf/config.go -- parseRouterInformation -->

The `extended-prefix` and `extended-link` boolean leaves gate origination of the RFC 7684
Extended Prefix (Opaque type 7) and Extended Link (Opaque type 8) Opaque LSAs. Both default
false and require `opaque true`; they are the prefix/link attribute containers a Segment
Routing (or other prefix/link-attribute) application fills, so they stay off until such a
producer needs them. Reception and decode of a peer's Extended Prefix/Link LSAs is always on
once the plugin is built, regardless of these leaves. See the OSPF guide's Extended
Prefix/Link Attributes section.
<!-- source: internal/plugins/ospf/yang/ze-ospf-conf.yang -- extended-prefix, extended-link -->
<!-- source: internal/plugins/ospf/config.go -- cfg.ExtendedPrefix, cfg.ExtendedLink -->

The `segment-routing` container enables OSPF Segment Routing over the MPLS data plane
(RFC 8665 for the IPv4 family, RFC 8666 for the IPv6 family under `address-family ipv6`).
It carries an `enable` toggle, an `srgb` and `srlb` range (each `lower-bound`/`upper-bound`
MPLS labels), a `prefix-sid` list (keyed by prefix, with an `index` into the SRGB plus the
`node-sid`, `no-php`, and `explicit-null` flags), and an optional `srms-preference`. The
SRGB and SRLB must be disjoint contiguous ranges within 16..1048575 and must not overlap the
LDP or RSVP-TE label space; the `ze doctor` check `doctor-ospf-segment-routing-overlap`
reports an unsound range. When enabled the RI LSA advertises SR-Algorithm 0 plus the SRGB/SRLB,
the Extended Prefix LSA carries the node Prefix-SIDs, and the Extended Link LSA carries the
Adjacency-SIDs. See the OSPF guide's Segment Routing section.

```
ospf {
    opaque true;
    segment-routing {
        enable true;
        srgb { lower-bound 16000; upper-bound 23999; }
        srlb { lower-bound 40000; upper-bound 40999; }
        prefix-sid 10.0.0.1/32 { index 1; node-sid true; }
    }
}
```
<!-- source: internal/plugins/ospf/yang/ze-ospf-conf.yang -- segment-routing grouping -->
<!-- source: internal/plugins/ospf/sr_config.go -- parseSegmentRouting, applySRConfig -->

The `fast-reroute` container enables RFC 5286 LFA / TI-LFA IP fast reroute: SPF
pre-computes a loop-free backup next-hop per primary and programs it into the FIB
as a link-down backup. `enable` (default `false`) turns it on; `mode` selects
`lfa` (default, base loop-free alternates only) or `ti-lfa` (adds a Segment-Routing
repair-list fallback, which requires `segment-routing` and carries repair labels on
IPv4 only); `node-protection` (default `true`) prefers node-protecting alternates.
It is a router-wide policy that also drives the OSPFv3 family (base-LFA selection).

```
ospf {
    fast-reroute {
        enable true
        mode ti-lfa
        node-protection true
    }
}
```
<!-- source: internal/plugins/ospf/yang/ze-ospf-conf.yang -- fast-reroute container -->
<!-- source: internal/plugins/ospf/config.go -- parseFastReroute -->

At the root IPv4 level, `area-type` selects `normal`, `stub`, or `nssa`. A stub
or NSSA area sets `default-cost` (the injected default metric) and may set
`no-summary true` for a totally-stubby/totally-NSSA area. An NSSA area takes an
`nssa { ... }` block with `translate-role` (`candidate` electing by highest
Router ID, `always`, or `never`), `stability-interval` (translator hysteresis in
seconds, `0..65535`, default 40), and `default-originate` (originate a Type 7
default into the NSSA). In `address-family ipv6`, the area entry currently
selects only `area-type`.

```
ospf {
    router-id 10.0.0.1
    areas {
        area 0.0.0.0 {
            area-type normal
        }
        area 0.0.0.9 {
            area-type nssa
            no-summary true
            default-cost 20
            nssa {
                translate-role candidate
                stability-interval 40
                default-originate true
            }
        }
    }
    interfaces {
        interface eth0 {
            area 0.0.0.0
        }
        interface eth1 {
            area 0.0.0.9
        }
    }
}
```

```
ospf {
    router-id 172.30.0.2
    address-family ipv6 {
        instance-id 0
        areas {
            area 0.0.0.0 {
                area-type normal
            }
            area 0.0.0.9 {
                area-type nssa
            }
        }
        interfaces {
            interface eth0 {
                area 0.0.0.0
                network-type point-to-point
            }
            interface eth1 {
                area 0.0.0.9
                network-type point-to-point
            }
        }
    }
}
```
<!-- source: internal/plugins/ospf/yang/ze-ospf-conf.yang -- area-type, no-summary, default-cost, nssa -->
<!-- source: internal/plugins/ospf/config.go -- parseArea, validNSSATranslateRole -->

Interface authentication uses named key chains. A `key-chains <name>` block holds one
or more keys (each with an `algorithm`, a `$9$`-encoded `secret`, and optional send/
accept lifetimes); `extended-sequence true` selects RFC 7474 AuType 3. Bind a chain to
an interface directly, or set the interface `authentication { mode inherit }` and the
chain bound to its area (`area { authentication { key-chain } }`) is used. `algorithm`
is one of `simple`, `md5`, or `hmac-sha-1/256/384/512`; secrets are masked and
`$9$`-encoded at rest, never shown in plaintext.

```
ospf {
    router-id 10.0.0.1
    key-chains core {
        extended-sequence true
        key 1 {
            algorithm hmac-sha-256
            secret $9$encoded
        }
    }
    areas {
        area 0.0.0.0 {
            authentication {
                key-chain core
            }
        }
    }
    interfaces {
        interface eth0 {
            area 0.0.0.0
            authentication {
                mode inherit
            }
        }
    }
}
```
<!-- source: internal/plugins/ospf/yang/ze-ospf-conf.yang -- key-chains, authentication, extended-sequence -->
<!-- source: internal/plugins/ospf/auth_keystore.go -- configure, decodeSecret -->

Redistribution and default-route origination make the router an ASBR. The `ospf`
container sets the per-source external metric, metric-type (`type-1`/`type-2`),
and route tag, and configures `default-information originate`. The shared
top-level `redistribute` block enrols the actual route flow: `destination ospf`
imports routes as OSPFv2 Type 5 LSAs for IPv4, OSPFv3 AS-External-LSAs for
normal IPv6 areas, or OSPFv3 NSSA-LSAs for attached IPv6 NSSA areas.
`destination bgp { import ospf }` exports OSPF routes. `import ospf` into OSPF
itself is a runtime no-op (loop prevention).

```
ospf {
    router-id 10.0.0.1

    default-information {
        originate true
        always false
        metric 10
        metric-type type-2
    }

    redistribute {
        connected {
            metric 20
            metric-type type-2
            tag 100
        }
        static {
        }
    }

    areas {
        area 0.0.0.0 {
        }
    }
    interfaces {
        interface eth0 {
            area 0.0.0.0
        }
    }
}

redistribute {
    destination ospf {
        import connected
        import static
        import bgp
    }
    destination bgp {
        import ospf
    }
}
```

With `default-information originate` and `always false`, the Type 5 default
(`0.0.0.0/0`) is advertised only while a non-OSPF default route exists in the
RIB; `always true` advertises it unconditionally. The `redistribute` source list
accepts `connected`, `static`, `kernel`, `bgp`, and `isis`; the
`default-information` and per-source `metric` are bound to `0..16777215`.

Each `import` is scoped to the `destination` protocol that encloses it: an
`import connected` under `destination bgp` feeds only BGP, and does NOT also leak
into OSPF or IS-IS. To feed the same source into two protocols, list it under both
destinations. Routes injected by a redistribute source (connected, static, l2tp,
...) into `destination bgp` also replay to a BGP peer that establishes AFTER the
injection (e.g. a dynamic/inbound peer), matching how received/best-path routes are
delivered to a new peer.

The `as112` DNS plugin is also a redistribute source: `redistribute { destination bgp { import as112 } }` originates its four fixed covering prefixes (192.175.48.0/24, 192.31.196.0/24, 2620:4f:8000::/48, 2001:4:112::/48) into BGP. Three leaves under `service { as112 { ... } }` tune the announcement: `asn` (origin AS on the wire, default 112), `community` (leaf-list of well-known names or AA:NN values attached to the routes), and `watchdog` (boolean, default true: announce only while the DNS node is serving, RFC 7534 Section 3.3).
<!-- source: internal/plugins/as112/yang/ze-as112-conf.yang -- asn/community/watchdog leaves -->
<!-- source: internal/plugins/ospf/yang/ze-ospf-conf.yang -- default-information, redistribute -->
<!-- source: internal/plugins/ospf/default.go -- applyDefaultInformation -->
<!-- source: internal/plugins/ospf/redistribute/consumer.go -- Consumer InjectRoute -->
<!-- source: internal/component/config/redistribute/route.go -- ImportRule.Destination, destination-scoped Accept -->
<!-- source: internal/component/bgp/plugins/redistribute_egress/replay.go -- late-join replay-on-peer-up -->

## Inheritance

Configuration uses 3-level inheritance: BGP globals, group defaults, peer overrides.

| Level | Scope | Example |
|-------|-------|---------|
| BGP | All peers | `bgp { router-id 1.2.3.4; }` |
| Group | Peers in group | `group upstream { timer { receive-hold-time 180; } }` |
| Peer | Single peer | `peer transit-a { timer { receive-hold-time 90; } }` |

Containers (like `capability`, `family`, `timer`) are deep-merged across levels. Leaf values (like `receive-hold-time` inside `timer`) override.
<!-- source: internal/component/bgp/config/resolve.go -- ResolveBGPTree, inheritance merging -->

## Peer Settings

Peers are keyed by name (`peer <name> { }`) where the name must start with a letter.

| Setting | Description | Required |
|---------|-------------|----------|
| `remote { ip; as; }` | Peer IP and AS number | Yes |
| `local { ip; as; }` | Local bind address and AS | Yes (ip can be `auto`) |
| `router-id` | BGP router ID | Yes (or inherited) |
| `description` | Human-readable description | No |
| `timer { }` | Timer container: `receive-hold-time` (seconds, 0 or 3-65535, default 90), `send-hold-time` (seconds, 0 or 480-65535, default 0), `connect-retry` (seconds, default 120) | No |
| `remote { connect }` | Initiate outbound TCP connections: `true` or `false` (default: true) | No |
| `local { accept }` | Accept inbound TCP connections: `true` or `false` (default: true) | No |
| `port` | TCP port | No (default: 179) |
| `md5-password` | TCP MD5 authentication | No |
| `ttl-security` | Minimum TTL for incoming packets | No |
| `outgoing-ttl` | TTL for outgoing packets | No |
| `group-updates` | Enable/disable UPDATE grouping | No (default: enable) |
| `rs-fast-path` | Enable reactor-native RS forwarding (bypasses plugin dispatch for UPDATE forwarding) | No (default: disable) |
| `blackhole { }` | Honor RFC 7999's BLACKHOLE community from this peer. See [Blackhole Honoring](#blackhole-honoring-rfc-7999) | No (default: off) |
<!-- source: internal/component/bgp/config/peers.go -- PeersFromTree; internal/component/bgp/yang/ze-bgp-conf.yang -- peer settings, container timer -->

## Blackhole Honoring (RFC 7999)

A peer can ask Ze to discard the traffic for a prefix. It does this by tagging
the announcement with the BLACKHOLE community, 65535:666. Ze installs a discard
route for that prefix, and the traffic is dropped in the kernel FIB or in VPP.

Ze installs the discard route itself. There is no next-hop to allocate and no
static route to pre-create: the community is read on the session and the prefix
is programmed as a discard directly.

Honoring is off by default and is configured per session. Each leaf-list is a
separate condition from RFC 7999 Section 3.3.

| Leaf-list | Meaning |
|-----------|---------|
| `communities` | The communities this session agreed to honor. `blackhole` and `65535:666` are the same well-known value; any `ASN:VAL` works. Unset means the well-known value when `prefixes` is set, and nothing otherwise |
| `prefixes` | The prefixes this peer is authorized to advertise. A received blackhole is honored only when one of them covers it and is equal or shorter, which is RFC 7999 Section 3.3's first condition. An empty list authorizes nothing |

Standard RTBH takes one line. Writing the prefixes a neighbour may blackhole
within IS the explicit configuration directive RFC 7999 Section 4 asks for, so
`communities` defaults to the well-known value:

```
bgp {
    peer transit-a {
        blackhole {
            prefixes [ 192.0.2.0/24 198.51.100.0/24 ];
        }
    }
}
```

Most operators run RTBH on their own community rather than on the well-known
one. Name it, and name both when a peer sends either:

```
bgp {
    peer transit-a {
        blackhole {
            communities [ blackhole 65001:666 ];
            prefixes    [ 192.0.2.0/24 198.51.100.0/24 ];
        }
    }
}
```

A stated list is taken exactly: the well-known value is NOT added to it. An
operator who names `65001:666` alone honors that community and not 65535:666.

The four combinations:

| `prefixes` | `communities` | Behavior |
|------------|---------------|----------|
| set | unset | The well-known 65535:666 only |
| set | set | Exactly the stated set |
| unset | set | Nothing is honored: a community with no authorized prefix covers nothing |
| unset | unset | Nothing, which is RFC 7999 Section 4's default |

With the second configuration above, a BLACKHOLE-tagged announcement of
192.0.2.1/32 installs a discard route. The same announcement for 203.0.113.1/32
installs an ordinary route, because no listed prefix covers it.

`prefixes` asks a different question from a prefix list. A
prefix list bounds the LENGTH of an announcement, so a `192.0.2.0/24 le 24`
entry rejects the /32 inside it. This leaf-list asks whether the peer is authorized to
advertise a prefix that CONTAINS the announcement. The listed prefix must be equal to or
shorter than the announced one. So `192.0.2.0/24` covers every host route inside
192.0.2.0/24, and it covers no part of 198.51.100.0/24.

The container is available at the `bgp`, `group` and `peer` levels. Both
leaf-lists ACCUMULATE across the levels, so a peer that names one community
keeps the ones its group named. A peer cannot drop a community its group agreed:
where one session must be excluded, name the community on the peers rather than
on the group.

A listen-range group states the agreement for every session it accepts. Such a
session has no `peer` block, so the group is where an IXP route server writes
one `blackhole` container for all of its members:

```
bgp {
    group ix {
        connection {
            remote {
                ip    dynamic;
                range 192.0.2.0/24;
            }
        }
        blackhole {
            prefixes 192.0.2.0/24;
        }
    }
}
```

### Announcing a blackhole to a peer

The same `communities` list decides which peers Ze may ADVERTISE the well-known
BLACKHOLE community to. RFC 7999 Section 3.1 requires the two networks to agree
on use of the community before it is advertised, and naming it on the peer is
Ze's half of that agreement.

`announce blackhole <prefix>` reaches only the sessions whose resolved list holds
65535:666, under either spelling, which includes a session configured with
`prefixes` alone. A peer with no `blackhole` block, or one that named only
its own value such as `65001:666`, is left OUT of the announcement. It is not
sent the prefix untagged: an ordinary announcement of a host route under attack
attracts the traffic the operator asked to have discarded. When no selected peer
has agreed, the command fails and names the peers that have not.

`announce unicast <prefix> community 65535:666` meets the same gate, because the
obligation is about the community rather than the verb. Any other community is
untouched.

To advertise your own RTBH value to a peer, name that value on the announcement:

```
announce unicast 192.0.2.1/32 community 65001:666
```

### A blackhole on a community Ze does not read

The knob above answers the standard case. For anything else, a policy modifier
conditioned on a community rewrites whatever attribute the operator chooses,
and a next-hop rewrite toward an address routed to a discard is the vendor
two-step form of the same idea:

```
bgp {
    policy {
        modify RTBH {
            match { community 65001:666; }
            set { next-hop 192.0.2.1; }
        }
    }
    peer transit-a {
        filter import prefix-list:AUTHORIZED modify:RTBH;
    }
}
```

A route that meets no stated value passes through unchanged, so the rest of the
session forwards normally. On Linux the rewritten next-hop MUST resolve through a
device: route it to a dummy interface. A kernel `blackhole` route for that
address makes the gateway unresolvable and the route is never installed.

### Blackhole with origin validation

An operator who runs origin validation with `rpki { action { invalid reject } }`
must also set `blackhole-exempt`, or no blackhole announcement is honored.

A blackhole prefix is as long as possible, usually a /32 or a /128. A ROA for
the covering block carries its maxLength at the aggregate, often a /24. RFC 6811
then makes the /32 Invalid on length alone, and a session that rejects Invalid
routes drops the announcement before the blackhole logic sees it. The symptom is
a peer that reports the announcement as sent and a Ze that installs nothing.

```
bgp {
    peer transit-a {
        rpki {
            action { invalid reject; }
            blackhole-exempt true;
        }
        blackhole {
            communities blackhole;
            prefixes    192.0.2.0/24;
        }
    }
}
```

The exemption is narrow. It keeps the route only when a covering VRP names the
route's own origin AS and disagrees on nothing but prefix length. A wrong origin
AS stays Invalid, which is the hijack RFC 6811 exists to catch. A prefix with no
covering VRP is NotFound rather than Invalid, so the exemption never reaches it.

Set `blackhole-exempt` on the same session that names a blackhole community. The
exemption reads the communities THAT SESSION agreed to, so a peer running RTBH on
`65001:666` gets it for `65001:666`. On a session that names no community at all
the exemption does nothing, and Ze logs that at configure time naming the peer.
<!-- source: internal/component/bgp/plugins/rib/yang/ze-rib.yang -- blackhole-honor-fields; internal/component/bgp/blackholecfg/blackholecfg.go -- Rule, Parse, Carries, Agreed; internal/component/bgp/plugins/rib/rib_blackhole.go -- coveredByAuthorized, blackholeRouteType; internal/component/bgp/plugins/cmd/announce/blackhole_agreement.go -- agreedSelector; internal/component/bgp/plugins/filter_modify/yang/ze-filter-modify.yang -- match; internal/component/bgp/plugins/filter_modify/match.go -- matchCond; internal/component/bgp/plugins/rpki/yang/ze-rpki.yang -- blackhole-exempt; internal/component/bgp/plugins/rpki/blackhole.go -- invalidByLengthOnly, carriesAgreedBlackhole -->

## Capabilities

Configured under `capability { }` at any inheritance level.

| Capability | Config | Values |
|------------|--------|--------|
| 4-byte ASN | `asn4` | presence (enabled by default) |
| Route Refresh | `route-refresh` | presence |
| Extended Message | `extended-message` | presence |
| Graceful Restart | `graceful-restart { restart-time 120; }` | See [Graceful Restart guide](../graceful-restart/index.md) |
| ADD-PATH | `add-path send/receive` | See [ADD-PATH guide](../add-path/index.md) |
| Extended Next Hop | `nexthop { ipv4/unicast ipv6; }` | Per-family NH mapping |
| BGP Role | `role provider` | provider, customer, rs, rs-client, peer |
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- capability definitions -->

## Address Families

Configured under `family { }`. Each family requires a `prefix { maximum N; }` block.

```
family {
    ipv4/unicast { prefix { maximum 1000000; } }
    ipv6/unicast { prefix { maximum 200000; } }
    ipv4/mpls-vpn { prefix { maximum 500; } }
    l2vpn/evpn { prefix { maximum 10000; } }
}
```

Use `ze --plugins` to see available families from registered plugins.
<!-- source: internal/component/bgp/plugins/nlri/ -- NLRI plugin Families registration -->

### Prefix Limits

Every family must have a prefix maximum. Ze refuses to start without one.

```
family {
    ipv4/unicast {
        prefix {
            maximum 1000000
            warning 900000
        }
    }
}
```

Warning defaults to 90% of maximum when not set. Enforcement behavior is set in
the same per-family `prefix` block, so each family gets its own policy:

```
peer transit-a {
    family {
        ipv4/unicast {
            prefix {
                maximum 1000000
                teardown false
                idle-timeout 30
            }
        }
        ipv6/unicast {
            prefix {
                maximum 200000
                teardown true
            }
        }
    }
}
```

This peer warns on an IPv4 overflow and keeps the session, and stops the session
on an IPv6 overflow. Each family reads its own leaves. A family that sets no
value uses the default.

| Setting | Scope | Default | Description |
|---------|-------|---------|-------------|
| `teardown` | Per family | `true` | Send NOTIFICATION and close on exceed. `false` = warn only, and the UPDATE that crossed the maximum is dropped whole, not NLRI by NLRI. |
| `idle-timeout` | Per family | `0` | Seconds to wait before reconnect after this family stopped the session. 0 keeps the peer down. |
| `reconnect` | Per family | derived | `never`, `backoff` or `timer`. No value means `timer` when `idle-timeout` is above 0, and `never` when it is 0. |
| `count` | Per family | `offered` | Which prefixes the count compared against `maximum` holds. See below. |

#### What the count holds: `offered` or `installed`

The two values count different things. `offered` counts wire events, and
`installed` counts prefixes. RFC 4271 Section 6.7 does not say whether a prefix
limit governs what the peer offered or what the receiver kept, so this is an
operator choice rather than a fixed answer.

```
peer transit-a {
    family {
        ipv4/unicast {
            prefix {
                maximum 1000000
                teardown false
                count installed
            }
        }
    }
}
```

| Value | What the count holds | What a re-announced prefix does |
|-------|---------------------|--------------------------------|
| `offered` | Every announcement the peer sends, the announcements of a dropped UPDATE included. It rises on an announcement and falls on a withdrawal, whatever prefix each one names | Raises the count again. The peer holds the same route, and the count says it holds two |
| `installed` | The set of prefixes this family currently holds. The count is that set's size | Nothing. The prefix is already in the set |

The difference shows up on a peer with route churn, which is every transit
session. BGP has no way to refresh one route: a peer changing an attribute sends
the same prefix again, and that replaces the path it sent before (RFC 4271
Section 3.1). `offered` reads that second announcement as a second prefix.
`installed` reads it as the prefix it already has.

Pick `offered` to bound how much a peer may SEND you, announcement by
announcement, and accept that a peer which re-announces will reach the bound
without ever holding that many routes. Pick `installed` to bound how many
distinct prefixes a peer may advertise to you at once, which is the number that
stops moving when the peer only churns attributes.

Neither value is the size of the RIB, and neither is the number
`show bgp summary` reports. Both counts are taken before import policy runs.
`show bgp summary` reports `routes-received` and `routes-accepted`, and both are
the Adj-RIB-In size AFTER that policy, because Ze stores no route the policy
rejected. A peer whose prefixes the policy rejects therefore reads lower there
than in either count. Neither value changes enforcement, either: both drop the
same UPDATE whole, and both send the same NOTIFICATION under `teardown true`.

`installed` keeps one entry per prefix for that family, so it costs memory in
proportion to what the peer sends, bounded by `maximum` when one is configured.
`offered` keeps a number.

> **`offered` can be driven below the routes Ze holds.** A withdrawal for a
> prefix the peer never announced still lowers the count, so a peer sending N of
> them frees N slots that Ze is still using and the Adj-RIB-In can then pass the
> maximum. `installed` is immune: a withdrawal of a prefix that is not in the set
> removes nothing. This is recorded in
> `plan/deferrals/fixit-bgp-per-family-prefix-enforcement.md` and is not yet
> fixed.

#### After a prefix teardown, the peer stays down

A session stopped by a prefix limit does not come back on its own. The peer
state reads `idle-hold`, `ze show warnings` names the family that stopped it,
and the log says `peer held down`. An operator brings the peer back by
recreating it: raise that family's `maximum` and commit, or delete and add the
peer.

This is the default because a peer that reconnects into the same limit exceeds
it again and flaps. To ask for the reconnect anyway:

```
peer transit-a {
    family {
        ipv4/unicast {
            prefix {
                maximum 1000000
                reconnect backoff
            }
        }
        ipv6/unicast {
            prefix {
                maximum 200000
                idle-timeout 30
            }
        }
    }
}
```

IPv4 comes back on the usual connect backoff, 5 to 60 seconds. IPv6 waits 30
seconds, and the wait doubles on each repeat teardown up to one hour. Ze refuses
a `reconnect` value that contradicts `idle-timeout` in the same block, for
example `reconnect never` beside `idle-timeout 30`.

### Protocol Event Capture

Ze can write every message a peer sends to a file, together with the config
operations applied while the capture runs. Replay the file on a developer
machine with `ze-test replay <file>` to drive the same state machine over the
same input. Capture is off by default. Turn it on for one peer when you need to
reproduce a session bug, and turn it off again.

```
peer transit-a {
    capture {
        enabled true
        directory /var/lib/ze/capture
        maximum-size 100
        on-limit rotate
    }
}
```

| Setting | Default | Description |
|---------|---------|-------------|
| `enabled` | `false` | Capture this peer's inbound protocol events. |
| `directory` | `/var/lib/ze/capture` | Holds one file per peer, named `bgp-<peer-address>.jsonl`. Created if absent. |
| `maximum-size` | `100` | Hard cap on one file, in megabytes, range 1 to 1024. The writer refuses a record that would cross the cap, so a file never exceeds this value. |
| `on-limit` | `rotate` | `rotate` renames the full file to `<file>.1` and starts a fresh one, so the newest events are kept and the peer uses at most twice `maximum-size` on disk. `stop` closes the file, so the oldest events are kept. |

`ze doctor` reports a capture directory the daemon cannot write. When the writer
queue fills, Ze sheds events rather than stopping the BGP read loop, counts them
in `ze_bgp_capture_dropped_events_total`, and writes the gap into the file.

The file holds the peer's routing data: prefixes, AS paths, communities and
next-hops. It holds no local secret. TCP-MD5 keys never travel on the wire, and
captured config payloads are redacted. Handle the file like a routing-table
dump.

<!-- source: internal/component/bgp/reactor/capture_replay.go -- sessionCapture, startCapture -->
<!-- source: internal/component/bgp/reactor/config.go -- parseCaptureSettings -->
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- container capture -->

### PeeringDB Prefix Data

Ze can query PeeringDB to suggest prefix maximums. Configure the PeeringDB API URL and margin in the system block:

```
system {
    peeringdb {
        url "https://www.peeringdb.com"
        margin 10
    }
}
```

| Setting | Default | Description |
|---------|---------|-------------|
| `url` | `https://www.peeringdb.com` | PeeringDB-compatible API base URL. |
| `margin` | `10` | Percentage added above PeeringDB prefix count (0-100). |

Use `resolve peeringdb max-prefix <asn>` to look up prefix limits, then apply them via the config editor.
<!-- source: internal/component/bgp/reactor/session_prefix.go -- prefix limit enforcement; internal/component/bgp/yang/ze-bgp-conf.yang -- prefix config -->
<!-- source: internal/component/config/system/yang/ze-system-conf.yang -- peeringdb config -->

## Hardware Tuning

Ze can apply hardware tuning at startup and on config commit. Tuning operations are Linux-only and require root or `CAP_SYS_ADMIN`.

```
system {
    tuning {
        cpu {
            governor performance;
        }
        irq-affinity eth0 {
            cpus 0,2,4-7;
        }
        ethtool eth0 {
            ring {
                rx 4096;
                tx 4096;
            }
        }
    }
}
```

| Path | Description |
|------|-------------|
| `tuning/cpu/governor` | CPU scaling governor: `performance`, `powersave`, `ondemand`, `conservative`, `schedutil` |
| `tuning/irq-affinity` | Per-interface IRQ CPU affinity (CPU list format: `0,2,4-7`) |
| `tuning/ethtool/ring/rx` | Receive ring buffer size (1-65535) |
| `tuning/ethtool/ring/tx` | Transmit ring buffer size (1-65535) |

Tuning is idempotent: only changed parameters are written. Write failures are reported but do not block the config commit. On non-Linux platforms the tuning block is accepted but no operations are applied.
<!-- source: internal/component/config/system/yang/ze-system-conf.yang -- tuning config -->
<!-- source: internal/component/host/tuning.go -- ApplyTuning engine -->

## Process Bindings

A peer attaches a program in an `attach process` block. The block names a
plugin and states the relationship in two directions: `receive` is what the
program is fed for this peer, and `send` is what it may originate toward it.

```
peer transit-a {
    ...
    attach process adj-rib-in {
        receive [ update-received state ];
    }
    attach process rpki {
        receive [ update-received ];
    }
}
```

**A peer that attaches nothing feeds nothing.** These blocks decide delivery.
A running plugin that a peer does not attach receives no event from that peer,
whatever the plugin asked for when it started, and it may send that peer
nothing. Loading a plugin and attaching it to a peer are two separate acts.

Base event types for `receive`: `update`, `open`, `notification`, `keepalive`,
`refresh`, `state`, `negotiated`, `eor`, `rpki`. Plugins may register more (for
example `update-rpki` from bgp-rpki-decorator). A plain type is fed in both
directions; `update-received` and `update-sent` name one. `*` names every
registered type. Values are validated at config parse time.

Base message types for `send`: `update` to originate routes, `refresh` to ask
the peer to re-advertise. `*` grants both, and every send type registered
later. A send a peer's block does not grant is refused and reported.

Write the block on a group and every peer of that group carries it, dynamic
members included. A member that restates the block replaces the group's list
for itself alone. `show event delivery` prints the edges the running config
produces, peer by peer.
<!-- source: internal/component/bgp/event.go -- event type definitions; internal/component/plugin/registry/registry.go -- EventTypes registration -->
<!-- source: internal/component/plugin/server/delivery_graph.go -- DeliveryGraph, PeerScopedProcs -->

## Route Filters

Route filtering is configured via `filter` blocks at bgp, group, or peer level.
Values are named filter instances from `bgp policy` or explicit `<plugin>:<filter>`
references to plugin-declared filters.

```
bgp {
    filter {
        import [ rpki:validate ]
    }
    group customers {
        filter {
            import [ community:scrub ]
        }
        peer customer-a {
            remote { ip 10.0.0.1; as 65001; }
            filter {
                export [ aspath:prepend ]
            }
        }
    }
}
```

Chains are cumulative: bgp-level filters run first, then group, then peer.
Mandatory filters (e.g., `rfc:otc`) always run before user filters and cannot
be removed. Default filters (e.g., `rfc:no-self-as`) run unless overridden by
a filter that declares `overrides`.

See [Route Filters](../redistribution/index.md) for details.
<!-- source: internal/component/bgp/config/redistribution.go -- extractFilterChain and canonicalizeFilterRefs -->

## Cross-Protocol Redistribute

The `redistribute` block nests import rules under a destination protocol.
Each destination protocol registers a `RedistConsumer`; the orchestrator
dispatches accepted routes to the matching consumer.

| Consumer | Purpose |
|----------|---------|
| BGP `IngressFilter` | Ingress ACL: when the source is `ibgp` or `ebgp`, gates received UPDATEs from intra-BGP sources. |
| `redistribute-orchestrator` plugin | Dispatches non-BGP route-change events to registered consumers (BGP, IS-IS). |

```
redistribute {
    destination bgp {
        import ebgp { family [ ipv4/unicast ipv4/vpn ]; }    # intra-BGP ACL
        import l2tp;                                          # egress, all families
        import connected { family [ ipv4/unicast ]; }         # egress, IPv4 unicast only
        import isis;                                          # IS-IS SPF routes into BGP
    }
    destination isis {
        import connected;                                    # connected/static/BGP into
        import static;                                       # IS-IS LSPs as TLV 135
        import bgp;                                          # Extended IP Reachability
    }
}
```

`import isis` pulls IS-IS SPF routes (both levels, single source `isis`) into BGP.
`destination isis` injects connected/static/BGP prefixes into the IS-IS link-state
database as Extended IP Reachability (TLV 135); `destination isis { import isis }`
is accepted by the schema but is a runtime no-op (loop prevention rejects
redistributing IS-IS into itself).

Both consumers read the same `redistribute.Global()` evaluator, so a single
config block governs both behaviors. The producer side (per-protocol route-
change events) lives in each protocol component; the orchestrator
(`redistribute-orchestrator`) enumerates non-consumer producers at startup and subscribes.

The orchestrator auto-loads when `redistribute {}` is present in the
config (it claims `ConfigRoots: ["redistribute"]`). No `plugin { external
redistribute-orchestrator { ... } }` block is required. The intra-BGP
IngressFilter rides the registry's filter chain at init time and needs no
plugin spin-up.

<!-- source: internal/component/config/redistribute/evaluator.go -- shared Global evaluator -->
<!-- source: internal/component/config/redistribute/consumer.go -- consumer registry -->
<!-- source: internal/component/bgp/plugins/redistribute_ingress/filter.go -- ingress ACL consumer -->
<!-- source: internal/component/bgp/plugins/redistribute_egress/redistribute.go -- orchestrator -->

### Prefix-List Filter

Named prefix-list filters live under `bgp { policy { prefix-list NAME { ... } } }`
and are invoked via peer filter chains. Each list is an ordered sequence
of match entries, first-match-wins, with an implicit deny on no match.

```
bgp {
    policy {
        prefix-list CUSTOMERS {
            entry 10.0.0.0/8 {
                ge 16
                le 24
                action accept
            }
            entry 192.168.0.0/16 {
                ge 24
                le 24
                action reject
            }
        }
    }

    peer customer-a {
        connection { remote { ip 10.0.0.1; as 65001; } }
        filter {
            import [ CUSTOMERS ]
        }
    }
}
```

Each `entry` has:

| Field | Type | Purpose |
|-------|------|---------|
| `ge` | uint | Minimum prefix length (inclusive). Defaults to the entry's own CIDR length. |
| `le` | uint | Maximum prefix length (inclusive). Defaults to 32 (IPv4) / 128 (IPv6). |
| `action` | enum | `accept` or `reject`. |

A route matches an entry when its address is contained in the entry's CIDR
AND its prefix length satisfies `ge <= length <= le`. Entries are evaluated
in the order they appear; the first match determines the action. A route
that matches no entry is implicitly rejected.

**Multi-prefix UPDATEs.** When an UPDATE carries several prefixes and the
prefix-list accepts some but rejects others, the filter returns `modify`:
the UPDATE is rewritten to carry only the accepted prefixes. The rejected
ones are silently removed. In v1 this applies to the legacy IPv4 unicast
NLRI only; IPv6 and labeled families still use whole-update accept/reject
semantics.

Filter instance names are globally unique under `bgp policy`. Use the plain
name as the primary reference form. Prefixed forms are accepted for
disambiguation or advanced use:

| Form | Example |
|------|---------|
| Plain filter name (preferred) | `CUSTOMERS` |
| `<filter-type>:<filter>` | `prefix-list:CUSTOMERS` |
| `<plugin>:<filter>` | `bgp-filter-prefix:CUSTOMERS` |

<!-- source: internal/component/bgp/plugins/filter_prefix/yang/ze-filter-prefix.yang -- prefix-list YANG container -->
<!-- source: internal/component/bgp/plugins/filter_prefix/config.go -- parsePrefixLists -->
<!-- source: internal/component/bgp/plugins/filter_prefix/filter_prefix.go -- handleFilterUpdate, per-prefix partition modify path -->

### IRR Prefix-List Filter

The `bgp-filter-irr` plugin generates prefix-list filters from IRR (Internet
Routing Registry) data. It queries a whois server for an AS-SET's prefixes
and applies them as import filters, replacing the external bgpq4 workflow.

```
bgp {
    policy {
        irr {
            server whois.radb.net    // IRR whois server (default)
            peeringdb-url https://www.peeringdb.com  // PeeringDB API base URL (default)
            refresh-interval 3600   // seconds between re-queries (60-86400)
        }
    }

    peer customer-a {
        session {
            asn { local 65000; remote 65001; }
            irr {
                as-set AS-CUSTOMER    // explicit AS-SET (omit for PeeringDB auto-discovery)
            }
        }
        filter {
            import [ bgp-filter-irr:65001 ]
        }
    }
}
```

The suffix in `bgp-filter-irr:65001` is the peer's remote ASN. Use the
corresponding ASN in each peer's filter reference. The plugin queries IRR for
the AS-SET's prefixes and builds a prefix-list keyed by ASN. Routes with
prefixes in the list are accepted; all others are rejected (implicit deny).

When `irr { as-set }` is omitted, the plugin auto-discovers the AS-SET from
PeeringDB using the peer's remote ASN. If PeeringDB has no AS-SET registered
for the ASN, the plugin falls back to querying IRR directly using
`AS<number>` (e.g. `AS65001`). If the IRR query also fails, the existing
prefix-list is preserved and the error is reported via `show bgp irr`.
Set `irr { enable disable; }` to opt a peer out of IRR filtering entirely.

For a complete configuration, verification workflow, troubleshooting table,
and local recording, see [Filter BGP Imports with IRR](../irr-filtering/index.md).

The plugin refreshes prefix-lists automatically at the configured interval and
on demand via `update bgp irr all`. Resolved prefixes are stored as operational
data in zefs (not in the config tree) and survive restarts.

| Command | Purpose |
|---------|---------|
| `show bgp irr` | Per-ASN status: AS-SET, prefix count, last/next refresh |
| `show bgp irr prefix <peer>` | List all prefixes for a peer |
| `show bgp irr check <peer> <prefix>` | Test if a prefix would be accepted |
| `update bgp irr all` | Refresh all peers immediately |
| `update bgp irr asn <asn>` | Refresh a specific ASN |
| `update bgp irr as-set <as-set>` | Refresh peers using a specific AS-SET |

<!-- source: internal/component/bgp/plugins/filter_irr/yang/ze-filter-irr.yang -- IRR filter YANG schema -->
<!-- source: internal/component/bgp/plugins/filter_irr/config.go -- parseIRRConfig -->
<!-- source: internal/component/bgp/plugins/filter_irr/filter_irr.go -- runFilterIRR, handleFilterUpdate, handleConfigure -->

### AS-Path Filter

Named AS-path regex filters live under `bgp { policy { as-path-list NAME { ... } } }`.
Each list is an ordered sequence of regex entries with accept/reject action.
The AS-path is converted to a space-separated decimal string and matched
using Go RE2 (linear time, inherently ReDoS-safe). Use `[0-9]` instead of
`\d` in regex strings (ze's config parser interprets backslash as escape).

```
bgp {
    policy {
        as-path-list ALLOW-PEER {
            entry "^65001" { action accept }
        }
        as-path-list TRANSIT {
            entry "^[0-9]+( [0-9]+)+" { action accept }
        }
    }

    peer customer-a {
        filter { import [ as-path-list:ALLOW-PEER ] }
    }
}
```

<!-- source: internal/component/bgp/plugins/filter_aspath/yang/ze-filter-aspath.yang -- as-path-list YANG container -->
<!-- source: internal/component/bgp/plugins/filter_aspath/config.go -- parseAsPathLists -->

### Community Match Filter

Named community-match filters live under `bgp { policy { community-match NAME { ... } } }`.
Each entry checks for presence of a specific community value in the route's
standard, large, or extended community attributes.

```
bgp {
    policy {
        community-match BLOCK-NO-EXPORT {
            entry no-export { type standard; action reject }
        }
        community-match ALLOW-CUSTOMER {
            entry 65001:100 { type standard; action accept }
            entry 65001:100:200 { type large; action accept }
        }
    }

    peer transit-a {
        filter { import [ community-match:BLOCK-NO-EXPORT ] }
    }
}
```

<!-- source: internal/component/bgp/plugins/filter_community_match/yang/ze-filter-community-match.yang -- community-match YANG container -->
<!-- source: internal/component/bgp/plugins/filter_community_match/config.go -- parseCommunityLists -->

### Route Attribute Modifier

Named modifier definitions live under `bgp { policy { modify NAME { set { ... } } } }`.
Only present leaves are applied; undeclared attributes pass through unchanged.
A definition applies to every route that reaches it. A definition that states a
`match` container applies its operations only to the routes that meet the
condition, and every other route leaves the filter unchanged. Use a match filter
earlier in the chain when you want the rest dropped, because a rejected route
leaves the pipe.

```
bgp {
    policy {
        modify PREFER-LOCAL {
            set {
                local-preference 200
            }
        }
        modify PREPEND-3 {
            set {
                as-path-prepend 3
            }
        }
    }

    peer transit-a {
        filter {
            import [ prefix-list:CUSTOMERS modify:PREFER-LOCAL ]
            export [ modify:PREPEND-3 ]
        }
    }
}
```

| Leaf | Type | Range | Purpose |
|------|------|-------|---------|
| `local-preference` | uint32 | 0-4294967295 | Set LOCAL_PREF |
| `med` | uint32 | 0-4294967295 | Set MED |
| `med-remove` | boolean | true, false | Remove MULTI_EXIT_DISC from the route |
| `origin` | enum | igp, egp, incomplete | Set ORIGIN |
| `next-hop` | IP address | IPv4 | Set NEXT_HOP |
| `as-path-prepend` | uint8 | 1-32 | Prepend local AS N times |

`med-remove` is the mechanism RFC 4271 Section 5.1.4 requires a speaker to
implement, so it takes effect on an `import` chain only. That section requires
the removal to happen before Decision Process phases 1 and 2, and the import
chain is the only chain that runs there. On an `export` chain the directive is
refused and logged. It is mutually exclusive with `med`, `increment med` and
`decrement med`, and the configuration is refused when both are stated.

A metric received from one neighboring AS is already kept off a session toward
another with no configuration at all, which is the same section's propagation
rule. `med-remove` is for the operator who wants the metric gone from the route
itself.

The `match` container holds three leaf-lists, `community`, `large-community` and
`extended-community`. Values across all three are alternatives: any one present
in the route satisfies the condition.

<!-- source: internal/component/bgp/plugins/filter_modify/yang/ze-filter-modify.yang -- modify YANG container, match -->
<!-- source: internal/component/bgp/plugins/filter_modify/config.go -- parseModifyDefs -->
<!-- source: internal/component/bgp/plugins/filter_modify/filter_modify.go -- appendMEDRemove -->
<!-- source: internal/component/bgp/reactor/forward_med.go -- applyFactsMED -->
<!-- source: internal/component/bgp/plugins/filter_modify/match.go -- matchCond, matches -->

## Static Routes

Routes can be configured directly per peer:

```
peer transit-a {
    ...
    update {
        attribute {
            origin igp
            next-hop 10.0.0.2
            local-preference 100
            community [ no-export ]
        }
        nlri {
            ipv4/unicast add 10.0.0.0/24
            ipv4/unicast add 10.0.1.0/24
        }
    }
}
```

<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- static route config, update/attribute/nlri blocks -->

## MPLS

Enable MPLS label processing on an interface (Linux LSR). This sets
`net.mpls.conf.<iface>.input=1`; the global label-table size is set via
the `net.mpls.platform_labels` sysctl.

```
interface ethernet eth0 {
    unit 0 {
        mpls { enable true }
    }
}
```

BGP labeled-unicast routes are then programmed into the kernel MPLS FIB
(label push). Inspect installed entries with `show mpls forwarding`.

### RSVP-TE (traffic-engineered LSPs)

The `rsvp-te {}` container configures explicitly-routed MPLS tunnels with
bandwidth admission control. RSVP runs directly on IP (protocol 46, requires
`CAP_NET_RAW`).

```
rsvp-te {
    router-id 10.0.0.1
    interface eth0 {
        max-bandwidth 10e9
        max-reservable-bandwidth 8e9
    }
    tunnel to-egress {
        destination 10.0.0.9
        tunnel-id 1
        bandwidth 1e9
        explicit-route 1 {
            address 10.0.0.5/32
        }
        explicit-route 2 {
            address 10.0.0.9/32
        }
        fast-reroute {
            backup          facility
            node-protection false
        }
    }
    bypass around-eth0 {
        merge-point 10.0.0.5
        explicit-route 1 {
            address 10.1.0.5/32
        }
    }
}
```

`fast-reroute` on a tunnel requests RFC 4090 local protection (facility backup);
`bypass` defines a facility-backup LSP from this Point of Local Repair to a
`merge-point`, explicitly routed around the protected resource (ze has no CSPF).

Inspect LSP, interface, tunnel, and protection state with the component's
`show rsvp-te session`, `show rsvp-te interface`, `show rsvp-te tunnel`, and
`show rsvp-te fast-reroute` commands. See [RSVP-TE](../rsvp-te/index.md) for details and
current status.

<!-- source: internal/component/iface/yang/ze-iface-conf.yang -- unit/mpls/enable -->
<!-- source: internal/plugins/rsvpte/yang/ze-rsvp-te-conf.yang -- rsvp-te -->

## IS-IS

The `isis {}` top-level container configures the native IS-IS link-state IGP
(ISO/IEC 10589, RFC 1195 / 5305 / 5301). IS-IS runs directly over Layer 2 and
requires `CAP_NET_RAW`. At least one Network Entity Title (`net`) is required;
the System ID is derived from the 6 octets before the NSEL of the first NET
unless an explicit `system-id` is given.

```
isis {
    net 49.0001.0000.0000.0001.00
    level l1-l2
    lsp-lifetime 1200
    hostname r1
    interfaces {
        interface eth0 {
            circuit-type point-to-point
            metric 10
            hello-interval 10
            hold-multiplier 3
            priority 64
            address-family ipv4-unicast { }
        }
        interface eth1 {
            passive true
        }
    }
}
```

Per-interface `level-1 {}` / `level-2 {}` containers override metric, timers, and
DIS priority per level.

**Authentication** is configured as named `key-chains {}`. Each key has a
`key-id`, an `algorithm` (`cleartext`, `hmac-md5`, or `hmac-sha-1`/`224`/`256`/
`384`/`512`), a `secret` (stored `$9$`-encoded, like PPPoE/WireGuard keys), and
optional `send-lifetime` / `accept-lifetime` windows for hitless rotation. Chains
are referenced by per-interface (IIH Hellos) and per-level (LSP/CSNP/PSNP)
`auth-key-chain` leaves: the *area* key for Level 1 and the *domain* key for
Level 2:

```
isis {
    net 49.0001.0000.0000.0001.00
    key-chains area-key {
        key 1 {
            algorithm hmac-sha-256
            secret $9$....               # entered plaintext, $9$-encoded on commit
        }
    }
    key-chains iih-key {
        key 1 { algorithm hmac-sha-256  secret ... }
    }
    level-1 { auth-key-chain area-key }       # L1 LSP/CSNP/PSNP
    interfaces {
        interface eth0 {
            level-1 { auth-key-chain iih-key }    # L1 Hellos on this circuit
        }
    }
}
```

Under configured auth, a PDU with no, wrong, or misordered Authentication TLV is
rejected (no adjacency, not stored) and `ze_isis_auth_failures_total` increments.
See [`docs/guide/isis.md`](../isis/index.md) for the full authentication behaviour.
Adjacency, LSDB, SPF, and route install are delivered by the runtime IS-IS
feature set; this release wires the component, config, and circuits.

<!-- source: internal/plugins/isis/yang/ze-isis-conf.yang -- isis config root -->

## PKI Certificate Store

The `pki {}` top-level container holds CA and device certificates for IPsec,
TLS, and other certificate-based features.

```
pki {
    ca <name> {
        certificate <base64-DER>
    }
    certificate <name> {
        certificate <base64-DER>
        intermediate <base64-DER>        # optional intermediate CA
        private {
            key $9$...                   # auto-encoded via ze:sensitive
        }
    }
}
```

CA certificates are trusted roots for chain validation. Device certificates
include the certificate itself and optionally a private key (PKCS8, SEC1/ECDSA,
or PKCS1/RSA in base64-encoded DER). Private keys use `$9$` sensitive encoding
and are never shown in CLI output.

Chain validation runs at config load: device certificates must chain to a loaded
CA. Expired certificates are rejected with a descriptive error.

<!-- source: internal/component/pki/yang/ze-pki-conf.yang -- PKI YANG schema -->
<!-- source: internal/component/pki/config.go -- PKI config parser -->

## IPsec VPN Configuration

The `vpn { ipsec {} }` container configures site-to-site IPsec VPN tunnels. The data model
defines ESP groups, IKE groups, and peers. Ze validates the config at load time; strongSwan
handles runtime negotiation.

```
vpn {
    ipsec {
        interface pppoe0

        esp-group ESP-RW {
            lifetime 86400
            pfs disable
            proposal 10 {
                encryption aes128gcm
                hash sha256
            }
        }

        ike-group IKE-RW {
            key-exchange ikev2
            lifetime 0
            close-action start
            dead-peer-detection {
                action restart
                interval 10
                timeout 30
            }
            proposal 10 {
                encryption aes128gcm
                hash sha256
                dh-group 14
            }
        }

        site-to-site {
            peer management-bridge {
                ike-group IKE-RW
                esp-group ESP-RW
                connection-type initiate
                remote-address mgmt.example.com
                authentication {
                    mode x509
                    local-id EXAFO000000400
                    remote-id management-bridge
                    x509 {
                        ca-certificate exa-vpn-ca
                        certificate EXAFO000000400
                    }
                }
                vti {
                    bind vti0
                }
            }
        }
    }
}
```

Encryption algorithms: `aes128`, `aes256`, `aes128gcm`, `aes256gcm`. The schema also names
`chacha20poly1305` and `3des`, and no build carries a transform for either. A proposal
that names one is refused at commit, and the refusal lists the implemented set.
Hash algorithms: `sha256`, `sha384`, `sha512`. The schema also names `sha1`, and no build
carries a transform for it, so the same refusal applies.

An IKE proposal requires `hash` beside every cipher, because it names the PRF that
RFC 7296 Section 3.3.3 makes mandatory. An ESP proposal requires `hash` beside a non-AEAD
cipher and refuses it beside an AEAD cipher, because an AEAD cipher carries its own
integrity.
DH groups: 1-31 (14 = MODP-2048 recommended minimum).
Authentication modes: `pre-shared-secret` (with a `$9$`-encoded key), `x509` (PKI store
references), `eap-tls`, or `eap-mschapv2`.

Cross-reference validation runs at config load: peer IKE/ESP group references must name
defined groups, `ca-certificate` and `certificate` names must exist in the PKI store,
and `local-id` must match the certificate's subject CN.

A `ca-certificate` is mandatory for `x509` and for every EAP mode. It is the trust anchor
the remote certificate must chain to, and without one any self-signed certificate would
authenticate. Every EAP mode also needs a `certificate`, which the responder signs its
AUTH with (RFC 7296 Section 2.16). Set `remote-id` as well whenever the authority issues
to more than one client: it is what binds the certificate to this peer. See
[IPsec VPN](../ipsec/index.md) for the identity rules.

This validation runs at commit and at reload. It does not run under
`ze config validate`, which reads the schema and does not run a plugin's config verifier.

<!-- source: internal/component/ike/ipsec/yang/ze-ipsec-conf.yang -- IPsec YANG schema -->
<!-- source: internal/component/ike/ipsec/config.go -- IPsec config parser -->
<!-- source: internal/component/ike/ipsec/validate.go -- cross-reference validation -->
<!-- source: internal/component/ike/engine/config.go -- validateIPsecSections is the ike plugin's OnConfigVerify body -->

## Interface Configuration

Ze manages network interfaces with a descriptive-name model. Each interface type is a
YANG list keyed by a user-chosen name. The MAC address serves as the binding between the
config entry and the physical (or virtual) hardware.

<!-- source: internal/component/iface/yang/ze-iface-conf.yang -- interface container, list definitions -->

### Interface Types

| Type | Description | MAC required |
|------|-------------|--------------|
| `ethernet` | Physical ethernet interface | Yes |
| `veth` | Virtual ethernet pair | Yes |
| `bridge` | Bridge interface | Yes |
| `dummy` | Virtual dummy interface | No |
| `loopback` | Loopback (container, no key) | No |

<!-- source: internal/component/iface/yang/ze-iface-conf.yang -- ethernet, veth, bridge, dummy, loopback definitions -->

### Logical Name and the `os-name` Selector

An interface `name` is a logical, human-readable handle chosen by the operator. By
default the logical name is also used as the OS/kernel device name, so every interface
whose name already matches its kernel device resolves unchanged. To alias a logical name
to a *different* kernel device, set the `os-name` selector on an ethernet interface:

```
interface {
    ethernet uplink {
        os-name eth0;   # logical "uplink" binds to kernel device eth0
    }
}
```

Every subsystem that takes an interface (IS-IS, routing, and others) refers to the
logical name. A shared resolver in the `iface` component maps the logical name to its
kernel device via the `os-name` selector (defaulting to the name itself) and serves the
ifindex, MAC, MTU, and addresses, so operator-facing names are decoupled from kernel
device names. IS-IS, for example, resolves `isis { interface uplink { } }` to kernel
device `eth0` through this resolver. The device can also be selected by its hardware MAC
instead of by name (see [Binding by Hardware MAC](#binding-by-hardware-mac-macmatch)).

The `os-name` selector applies to **ethernet** interfaces, the kind Ze matches against
pre-existing kernel devices. Created kinds (dummy, veth, bridge, tunnels, wireguard,
xfrm) are made by Ze under their logical name, so `os-name` is ignored on them and the
logical name is always the kernel device name.

<!-- source: internal/component/iface/resolve.go -- Resolve, Addresses, osDeviceFor -->
<!-- source: internal/component/iface/yang/ze-iface-conf.yang -- leaf os-name -->

### Binding by Hardware MAC (`mac/match`)

Names and OS device names can change between boots (NIC reordering, slot moves). To pin a
logical interface to a physical NIC regardless of the name the kernel gives it, select the
device by its hardware MAC with `mac { match }`:

```
interface {
    ethernet uplink {
        mac {
            match a0:36:9f:12:34:56;   # bind "uplink" to the NIC with this MAC
        }
    }
}
```

The resolver scans every interface and binds `uplink` to the one carrying that MAC. It
matches the device's **permanent (factory) address** (`IFLA_PERM_ADDRESS`) when the NIC
reports one, so the binding **survives an operational MAC override** (`mac { address }`) on
the very same interface; for virtual devices that report no permanent address it matches
the current address instead. When no device carries the MAC the binding **defers** until
the device appears (the resolver fires a link event then). `mac/match` **takes precedence
over `os-name`** and, like `os-name`, applies to **ethernet** only.

<!-- source: internal/component/iface/resolve.go -- matchByMAC, deviceMatchMAC -->
<!-- source: internal/component/iface/yang/ze-iface-conf.yang -- leaf match (container mac) -->

### MAC Address Binding

For ethernet, veth, and bridge interfaces, the MAC container carries two independent
leaves. `mac { address }` **overrides** the operational MAC (omit it to keep the
hardware-assigned one); when set it must be unique within each type. `mac { match }`
**selects** the kernel device by its hardware MAC (see above). The two are independent: a
NIC can be matched by its permanent MAC and have its operational MAC overridden at the same
time. Names are descriptive labels chosen by the operator; `mac { address }` ties the named
config entry to a specific operational hardware address.

<!-- source: internal/component/iface/yang/ze-iface-conf.yang -- unique "mac/address", container mac -->

### Discovery During Init

Running `ze init` discovers OS interfaces via netlink (Linux) or stdlib (other platforms)
and writes initial config to `ze.conf`. Each discovered interface gets an entry named
after its OS name, with `mac { address }` populated and the `os-name` selector recording
the original OS device name (so renaming the config entry still maps back to the kernel
device). Loopback appears as an empty `loopback { }` container.

The `--seed` flag skips this discovery entirely: an appliance-image seed database must not
bake the build host's interfaces into the active config (they belong to the wrong machine
and would shadow the appliance's first-boot template). The appliance discovers its own
interfaces at first boot instead.

<!-- source: internal/plugins/init/main.go -- runInit interface discovery, seedFlag -->
<!-- source: internal/component/iface/discover.go -- DiscoverInterfaces -->

### Example

```
interface {
    ethernet uplink {
        mac {
            address 00:1a:2b:3c:4d:5e;
        }
    }
    ethernet mgmt {
        mac {
            address 00:1a:2b:3c:4d:5f;
        }
        mtu 1500;
    }
    bridge fabric {
        mac {
            address 00:1a:2b:3c:4d:60;
        }
        stp true;
    }
    dummy blackhole {
    }
    loopback {
    }
}
```

The MAC address validator provides format checking and live autocomplete from currently
discovered OS interfaces when editing config interactively.

<!-- source: internal/component/config/validators.go -- MACAddressValidator -->

### Interface Offload and Steering

L2 interfaces (ethernet, dummy, veth, bridge) support an `offload` container with
boolean leaves for hardware offload and software packet steering features. Each leaf
uses three-state semantics: `true` enables, `false` disables, absent preserves the OS
default (no kernel call is made).

<!-- source: internal/component/iface/yang/ze-iface-conf.yang -- offload container in interface-l2 grouping -->

| Feature | Mechanism | Description |
|---------|-----------|-------------|
| `gro` | kernel ioctl | Generic Receive Offload: aggregates incoming packets |
| `gso` | kernel ioctl | Generic Segmentation Offload: delays outgoing segmentation |
| `sg` | kernel ioctl | Scatter-Gather I/O: multi-buffer frame assembly |
| `tso` | kernel ioctl | TCP Segmentation Offload: hardware TCP segmentation |
| `lro` | kernel ioctl | Large Receive Offload: hardware RX coalescing (disable on routers) |
| `hw-tc-offload` | kernel ioctl | Hardware TC offload: NIC executes TC rules in hardware |
| `rps` | sysfs | Receive Packet Steering: distribute RX across CPUs |
| `rfs` | sysfs | Receive Flow Steering: steer RX to application CPU |

Ze talks to the kernel directly via ethtool ioctls and sysfs writes. The `ethtool(8)`
CLI program is not required.

```
interface {
    ethernet uplink {
        mac {
            address 00:1a:2b:3c:4d:5e;
        }
        offload {
            gro true;
            tso true;
            lro false;
            rps true;
        }
    }
}
```

Tunnel, wireguard, and PPPoE interfaces do not support offloads (the `offload` container
is part of the `interface-l2` YANG grouping, not `interface-common`).

### Backend Capability Errors

The `interface`, `traffic/control`, and `firewall` components carry a `backend`
leaf. The YANG schema annotates feature nodes
with a list of supporting backends via the `ze:backend` extension. When the
config selects a backend that does not implement a used feature, commit and
`ze config validate` reject the config before any Apply call runs.

For `traffic/control` today the gate enforces only the "no backend configured"
case (non-Linux startup, or an explicit empty `backend` leaf); per-feature
annotations for tc-specific qdiscs and filter types land with
`spec-fw-7-traffic-vpp` when a second backend exists to reject against.

Example: the `bridge`, `tunnel`, `wireguard`, `veth`, and `mirror` nodes are
annotated `ze:backend "netlink"`. Selecting `backend vpp` while using any of
them produces a diagnostic naming the YANG path, the active backend, and the
list of backends that DO implement the feature:

```
/interface/bridge: feature not supported by backend "vpp" (supported: netlink)
/interface/tunnel: feature not supported by backend "vpp" (supported: netlink)
```

Fix: change `backend` back to `netlink` (the default on Linux), or remove the
unsupported entries. The same gate runs at daemon reload (`SIGHUP`) and on
first-apply at startup, so the daemon cannot boot into an unworkable state.

<!-- source: internal/component/config/backend_gate.go -- ValidateBackendFeatures, walkBackendNode -->
<!-- source: internal/component/config/yang/modules/ze-extensions.yang -- extension backend -->
<!-- source: internal/component/iface/yang/ze-iface-conf.yang -- ze:backend annotations on bridge/tunnel/wireguard/veth/mirror -->

### Route Priority

The `route-priority` leaf on a unit sets the Linux route metric for default routes
on that interface. Lower values are preferred by the kernel. When a link goes down,
the metric is increased by 1024 to deprioritize the interface, allowing traffic to
shift to an alternative uplink. When the link comes back up, the original metric is
restored.

The leaf defaults to 254. A default route ze learns from the network (a DHCP lease,
a PPPoE session) is installed at that metric, so it ranks below a static route and
below every route a routing protocol produces. The number matches the order
`rib admin-distance` uses for protocols: connected 0, static 10, ebgp 20, ospf 110,
isis 115, ibgp 200. It is also the administrative distance a Cisco IOS DHCP client
gives the default route it learns, which is the same ranking decision on another
vendor. That distance is not a Linux metric, and ze does not read it: 254 is the
metric ze writes to the kernel.
The `pppoe-client` list carries its own `route-priority` leaf with the same default,
which ranks the route a PPPoE session installs when IPCP completes.

The metric decides ownership, not only preference. ze installs a learned default
route with `RTM_NEWROUTE` in replace mode, and the kernel matches such a route on
destination, metric and table. It does not match on the protocol that installed it.
Two default routes at different metrics are two kernel routes, and the lower metric
forwards. Two default routes at the SAME metric are one kernel route, owned by
whoever wrote it last: a learned route at metric 0 replaces an operator's static
default at metric 0, gateway included.

For IPv4, ze installs the default route with that metric when a DHCP lease provides
a gateway. For IPv6, ze leaves the router advertisement default routes to the kernel
until the unit WRITES `route-priority`. Writing it above 0 makes ze suppress the
kernel's automatic RA default route (`accept_ra_defrtr=0`) and install `::/0` routes
with the configured metric when NDP neighbor events indicate a router (NTF_ROUTER
flag). Multiple routers on the same link are each installed with the same metric. On
clean shutdown or config removal, `accept_ra_defrtr` is restored to 1.

The written leaf is the whole test, and DHCP is no part of it. A unit that takes its
IPv6 address from SLAAC or from a static address, with no DHCP of either family, hands
ze the same ownership: ze suppresses the kernel's RA default route AND installs its
own at the written metric. The two go together on every unit, because one map answers
for both.

<!-- source: internal/component/iface/yang/ze-iface-conf.yang -- route-priority leaf, unit and pppoe-client -->
<!-- source: internal/component/iface/config.go -- defaultLearnedRouteMetric, parseUnits, parsePPPoEClientEntry -->
<!-- source: internal/component/iface/register.go -- handleLinkDown, handleLinkUp, handleLinkDownIPv6, handleLinkUpIPv6 -->
<!-- source: internal/component/iface/register.go -- writtenRoutePriorities, suppressRAForConfig, suppressAcceptRaDefrtr, handleRouterDiscovered -->

```
interface {
    ethernet uplink {
        mac {
            address 00:1a:2b:3c:4d:5e;
        }
        unit default {
            route-priority 1;
            ipv4 {
                dhcp {
                    enabled true;
                }
            }
        }
    }
    ethernet backup {
        mac {
            address 00:1a:2b:3c:4d:5f;
        }
        unit default {
            route-priority 5;
            ipv4 {
                dhcp {
                    enabled true;
                }
            }
        }
    }
}
```

With this config, uplink (metric 1) is preferred over backup (metric 5). If uplink
goes down, its metric becomes 1025 (1 + 1024), so backup (metric 5) takes over. When
uplink recovers, its metric returns to 1 and traffic shifts back. A unit that writes
no `route-priority` installs its learned default routes at 254 and leaves the IPv6
RA default routes to the kernel.

#### Upgrading from a release that installed learned routes at metric 0

**A DHCP, PPPoE or dhcp-auto default route now lands at metric 254.** Before this
release ze installed it at metric 0, where a plain static default route also lands.
The kernel holds one route per destination and metric, so the learned route replaced
the operator's static default, took its gateway, and re-stamped it `proto 253`. The
operator saw one route where they configured two. Both routes now exist and the
static one forwards.

**What changes on the box.** Read the routes with `show route default`. A learned
default that read `metric 0` yesterday reads `metric 254` today. Any policy that
matches on that metric (a `ip rule`, a firewall mark, a monitoring check) must name
254. A static default route that ze had silently taken over reappears with its own
gateway after the next lease, because ze no longer writes over it.

**To keep the old metric, write `route-priority 0` on the unit.** An explicit 0 is
not the same as an absent leaf: it puts learned routes back on metric 0, with the
takeover that metric carries. The `pppoe-client` list takes the same leaf with the
same meaning.

**A unit that writes no `route-priority` still leaves IPv6 to the kernel.** The
non-zero default did not hand ze the RA default routes of every interface. Writing
the leaf above 0 is what sets `accept_ra_defrtr=0` and installs `::/0`, exactly as
before.

### VLAN 802.1p QoS Maps

VLAN units (units with `vlan-id`) accept two QoS maps that translate between
the 3-bit 802.1p PCP field of the 802.1Q tag header and the kernel's internal
packet priority. `ingress-qos-map` (keyed by PCP) classifies received tagged
frames: a frame arriving with the listed PCP gets the mapped internal priority,
which tc filters and nftables can then act on. `egress-qos-map` (keyed by
priority) stamps outgoing frames: a packet leaving with the listed internal
priority gets the mapped PCP on the wire, so downstream L2 switches honor the
class of service without inspecting IP headers. Both sides of every entry are
0-7; unmapped values fall back to 0. The maps require `vlan-id` and are
applied when the VLAN sub-interface is created.

<!-- source: internal/plugins/cos/yang/ze-cos-conf.yang -- ingress-qos-map, egress-qos-map lists (qos-maps grouping) -->
<!-- source: internal/plugins/iface/netlink/manage_linux.go -- CreateVLAN IngressQosMap/EgressQosMap -->

```
interface {
    ethernet eth0 {
        unit v100 {
            vlan-id 100;
            ingress-qos-map 6 {
                priority 6;
            }
            egress-qos-map 6 {
                pcp 6;
            }
            ipv4 {
                address [10.0.0.1/30]
            }
        }
    }
}
```

This mirrors a BNG-style CoS setup: traffic classified to internal priority 6
(for example by a tc class matching DSCP CS6) leaves `eth0.100` with PCP 6 in
the 802.1Q header, and tagged frames arriving with PCP 6 are classified to
internal priority 6 for upstream scheduling. The configured maps are visible
in `show interface` output as `ingress-qos-map` / `egress-qos-map`.

**VPP backend limitation:** VPP's `qos record` copies the VLAN PCP value
verbatim into internal QoS bits, so ingress maps must be identity
(pcp == priority for every entry, e.g. `pcp 6 { priority 6; }`). Non-identity
ingress maps (e.g. `pcp 6 { priority 3; }`) are rejected at config validation.
Egress maps are fully supported via VPP's `qos egress-map` + `qos mark`
pipeline. Use the netlink backend if remapped ingress is required.

<!-- source: internal/plugins/iface/vpp/ifacevpp.go -- enableVLANQoS qos-record + egress-map + mark -->
<!-- source: internal/component/iface/config.go -- validateVPPQoSMaps identity check -->

### Class-of-Service Profiles

Named 802.1p profiles define PCP-to-priority mappings that can be referenced
from interfaces instead of repeating inline `ingress-qos-map` / `egress-qos-map`
lists on every unit. The `class-of-service` section is provided by the cos
plugin. Profiles are inherited by all VLAN units on the interface; individual
units can override with a different profile or opt out with `none`.

<!-- source: internal/plugins/cos/yang/ze-cos-conf.yang -- ieee-802.1p profile definitions -->
<!-- source: internal/component/iface/config.go -- parseUnits class-of-service resolution -->

```
class-of-service {
    ieee-802.1p residential {
        ingress {
            pcp 0 { priority 0; }
            pcp 5 { priority 5; }
            pcp 6 { priority 6; }
        }
        egress {
            priority 0 { pcp 0; }
            priority 5 { pcp 5; }
            priority 6 { pcp 6; }
        }
    }
}

interface {
    ethernet eth0 {
        class-of-service residential;
        unit v100 { vlan-id 100; }
        unit v200 { vlan-id 200; class-of-service none; }
    }
}
```

The profile `residential` is applied to unit `v100` (inherited from the
parent interface). Unit `v200` opts out with `class-of-service none` and
gets no QoS maps. A unit cannot have both a class-of-service reference and
inline qos maps; the two are mutually exclusive.

Use `show class-of-service` to list the registered profiles and their
ingress/egress map entries.

<!-- source: internal/plugins/cos/register.go -- showProfiles, OnExecuteCommand "show class-of-service" -->

### Reverse Path Filtering (rpf-check)

The `rpf-check` leaf in the `ipv4` and `ipv6` unit containers controls unicast
Reverse Path Forwarding verification. Three modes are available:

| Value | Linux sysctl | Behavior |
|-------|-------------|----------|
| `disable` | `rp_filter=0` | No source address validation |
| `strict` | `rp_filter=1` | Packet must arrive on the interface the kernel would use to reach the source |
| `loose` | `rp_filter=2` | Source address must be reachable via any interface |

```
interface {
    ethernet eth0 {
        mac {
            address 02:00:00:00:00:01;
        }
        unit default {
            ipv4 {
                rpf-check strict;
            }
        }
    }
}
```

IPv6 `rpf-check` is accepted in config but only enforced with the VPP data plane.
On Linux without VPP, a warning is logged and the setting has no effect.

The legacy `rp-filter 0|1|2` integer syntax is still accepted for backward
compatibility but emits a deprecation warning. Use `rpf-check` in new configs.

<!-- source: internal/component/iface/config.go -- parseIPv4Settings, parseIPv6Settings, rpfMode -->
<!-- source: internal/component/iface/config_sysctl.go -- applySysctl -->

### IPv6 Router Advertisements

The `router-advertisement` container in a unit's `ipv6` container makes Ze send
IPv6 Router Advertisements on that unit (RFC 4861). Hosts on the link build
addresses by stateless address autoconfiguration (SLAAC), learn a default
router, and learn DNS resolvers. This is the job radvd does on other systems.

The container is Linux only and netlink only. A tree with `backend vpp` rejects
it at config verify with `feature not supported by backend "vpp"`.

```
interface {
    backend netlink;
    ethernet eth0 {
        unit 0 {
            ipv6 {
                address [ 2001:db8:1::1/64 ];
                forwarding true;
                router-advertisement {
                    enabled true;
                    maximum-interval 900;
                    minimum-interval 300;
                    router-lifetime 1800;
                    hop-limit 64;
                    managed false;
                    other-config false;
                    reachable-time 30000;
                    retransmit-timer 1000;
                    prefix 2001:db8:1::/64 {
                        on-link true;
                        autonomous true;
                        valid-lifetime 86400;
                        preferred-lifetime 43200;
                    }
                    rdnss {
                        server [ 2001:db8:1::53 2001:db8:1::54 ];
                        lifetime 3600;
                    }
                }
            }
        }
    }
}
```

| Leaf | Range | Unit | Default | Meaning |
|------|-------|------|---------|---------|
| `enabled` | boolean | | `false` | Send advertisements on this unit |
| `maximum-interval` | 4..1800 | seconds | 600 | Longest wait between unsolicited advertisements |
| `minimum-interval` | 3..1350 | seconds | 200 | Shortest wait between unsolicited advertisements |
| `router-lifetime` | 0..9000 | seconds | 1800 | How long hosts keep Ze in their default router list |
| `hop-limit` | 0..255 | | 64 | Hop Limit hosts put in their outgoing packets. 0 states no value |
| `managed` | boolean | | `false` | The M flag: hosts get their addresses from DHCPv6 |
| `other-config` | boolean | | `false` | The O flag: hosts get other configuration from DHCPv6 |
| `reachable-time` | 0..3600000 | milliseconds | 0 | Neighbor reachable time for hosts. 0 states no value |
| `retransmit-timer` | uint32 | milliseconds | 0 | Neighbor Solicitation retransmit time. 0 states no value |
| `prefix <cidr>` | list | | | One Prefix Information option for each entry |
| `prefix <cidr> on-link` | boolean | | `true` | The L flag |
| `prefix <cidr> autonomous` | boolean | | `true` | The A flag, which turns SLAAC on for this prefix |
| `prefix <cidr> valid-lifetime` | uint32 | seconds | 2592000 | How long the prefix stays valid |
| `prefix <cidr> preferred-lifetime` | uint32 | seconds | 604800 | How long addresses from the prefix stay preferred |
| `rdnss server` | up to 8 | | | Resolver addresses advertised to hosts (RFC 8106) |
| `rdnss lifetime` | uint32 | seconds | none | How long hosts use these resolvers |

<!-- source: internal/component/iface/yang/ze-iface-conf.yang -- container router-advertisement -->

Set `ipv6 forwarding true` on an advertising unit. Hosts send Ze their off-link
traffic. The kernel drops that traffic while `net.ipv6.conf.<device>.forwarding`
is 0. `ze doctor` reports `doctor-iface-ra-forwarding` for each advertising
interface in that state. Leave `accept-ra` at 0 unless the same interface also learns from another
router.

<!-- source: internal/plugins/iface/ra/doctor.go -- checkRAForwarding -->

Two zero values are legal input and each one carries its own meaning.
`router-lifetime 0` says that Ze advertises its prefixes and its resolvers while
it is not a default router (RFC 4861 Section 4.2). `rdnss lifetime 0` tells hosts
to stop using the resolvers (RFC 8106 Section 5.1). `rdnss lifetime` has no
default: leave the leaf out and Ze sends 3 x `maximum-interval` instead, which
keeps "retire the resolvers" apart from "say nothing about a lifetime".

Config verify applies four cross-leaf rules and rejects the commit on each one:

| Rule | Rejected input |
|------|----------------|
| `minimum-interval` is at most 0.75 x `maximum-interval` | `minimum-interval 500` with `maximum-interval 600` |
| `router-lifetime` is 0, or at least `maximum-interval` | `router-lifetime 300` with `maximum-interval 600` |
| `preferred-lifetime` is at most `valid-lifetime` | `valid-lifetime 3600` with the default `preferred-lifetime 604800` |
| A prefix carries no host bits, and is not link-local | `prefix 2001:db8:1::1/64`, `prefix fe80::/64` |

The third row is the common trap. `preferred-lifetime` defaults to 604800, so
lowering only `valid-lifetime` below that number is rejected. Set both leaves
together.

<!-- source: internal/component/iface/config_ra.go -- raValidate, raParsePrefixEntry -->

See [Interface Management](../../features/interfaces/index.md#ipv6-router-advertisements)
for the send loop, the counters, and the RFC references.

## Storage SMART Management

Ze can monitor disk health via direct ATA/NVMe ioctls (no `smartctl` binary needed).

```
storage {
    smart {
        enabled true;
        check-interval 1800;
        temperature {
            informational 45;
            critical 55;
            difference 4;
        }
        self-test {
            short {
                interval 24h;
                time 02:00;
            }
            long {
                interval 7d;
                time 03:00;
                day sunday;
            }
        }
    }
}
```

| Leaf | Type | Default | Description |
|------|------|---------|-------------|
| `enabled` | boolean | `false` | Enable SMART monitoring |
| `check-interval` | uint32 (60-86400) | `1800` | Health poll interval in seconds |
| `temperature/informational` | uint8 (1-100) | `45` | Informational warning threshold (Celsius) |
| `temperature/critical` | uint8 (1-100) | `55` | Critical error threshold (Celsius) |
| `temperature/difference` | uint8 (1-50) | `4` | Rate-of-change warning threshold (Celsius per interval) |
| `self-test/short/interval` | duration | `24h` | Short self-test interval |
| `self-test/short/time` | HH:MM | `02:00` | Preferred time of day for short tests |
| `self-test/long/interval` | duration | `7d` | Extended self-test interval |
| `self-test/long/time` | HH:MM | `03:00` | Preferred time of day for extended tests |
| `self-test/long/day` | weekday | `sunday` | Preferred day for extended tests |

Temperature alerts are emitted to the report bus: `temp-high` (above informational),
`temp-rising` (rate of change exceeds difference), `temp-critical` (above critical),
`smart-failing` (SMART health status failed). Live status via `show storage smart`.

<!-- source: internal/component/storage/yang/ze-storage-conf.yang -->
<!-- source: internal/component/storage/manager.go -- Manager -->

## Authentication Users

Local SSH login users are declared under `system.authentication.user`. The
list key is the username; each entry has a one-way bcrypt-hashed password
and an optional list of authorization profile names.

```
system {
    authentication {
        user alice {
            plaintext-password "secret"     # write-only; hashed at commit and at load
            profile [ admin ]
        }
        user bob {
            password "$2a$10$..."           # canonical form, paste from `ze passwd`
            profile [ read-only ]
        }
    }
}
```

| Leaf | Stored on disk | Notes |
|------|---------------|-------|
| `password` | bcrypt hash (`$2a$10$...`) | Canonical, displayed in `show config`. Hand-editing a literal plaintext here triggers a `ze config validate` warning. |
| `plaintext-password` | only in a file you wrote | Junos-style write-only input. Ze bcrypt-hashes the value into `password` and removes this leaf. Ze never writes this leaf itself. |

Both leaves are inputs and only `password` reaches the running tree. Ze applies the
same transform on two paths:

| Path | What happens to the file |
|------|--------------------------|
| Editor `commit`, `ze config set`, `ze config import` | The file Ze writes carries the hash. It never carries the plaintext |
| A config file loaded at daemon start or at SIGHUP | Ze hashes the leaf in memory and leaves your file as you wrote it. It warns once, naming the file, because the plaintext is still in it |

For end-to-end usage (login, hashing, multi-user setup) see
[authentication.md](../authentication/index.md).
<!-- source: internal/component/ssh/yang/ze-ssh-conf.yang -- system.authentication.user -->
<!-- source: internal/component/config/password_hash.go -- ApplyPasswordHashing -->
<!-- source: internal/component/config/loader.go -- LoadConfig, warnPlaintextOnDisk -->

Remote AAA backends authenticate operators against a central server, with local
users as the fallback: `system.authentication.tacacs` (RFC 8907, see
[tacacs.md](../tacacs/index.md)) and `system.authentication.radius` (RFC 2865 operator
login, see [radius.md](../radius/index.md)). Both store their shared secret as a
`ze:sensitive` leaf and fall through to local bcrypt when the server is
unreachable.
<!-- source: internal/component/radius/yang/ze-radius-conf.yang -- system.authentication.radius -->
<!-- source: internal/component/tacacs/yang/ze-tacacs-conf.yang -- system.authentication.tacacs -->

### Authorization and web/API effects

Authorization profiles apply to command dispatch and web config mutation.
The web config editor checks the user's profile before `set`, `add`,
`delete`, `rename`, `commit`, or `discard`. Read-only users can still load
`/show/` pages. Terminal-mode web config commands use the same checks before
they mutate a draft.

REST and gRPC without a token or per-user authenticator use the `api`
identity as read-only. Reads such as `show version` and `show bgp summary` work;
write commands and config sessions return 403. Configure
`environment.api-server.token` or per-user credentials when API clients need
write access.
<!-- source: internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang -- module ze-peer-cmd -->

Config commits, discards, daemon reloads, and failed logins are recorded in
the local audit log. See [audit.md](../audit/index.md) for storage and query details.

<!-- source: internal/component/web/handler_config.go -- web config RBAC -->
<!-- source: internal/component/api/engine.go -- read-only API caller enforcement -->
<!-- source: internal/core/audit/audit.go -- audit Entry and actions -->

### TACACS+ AAA

`system.authentication.tacacs` adds RFC 8907 TACACS+ as the first backend
in the AAA chain; local bcrypt remains the fallback. Servers are tried in
declaration order on connection failure. Explicit FAIL replies stop the
chain (no fallthrough to local). See [tacacs.md](../tacacs/index.md) for the full
flow and operational notes.

```
system {
    authentication {
        tacacs {
            server 10.0.0.1 { port 49; key "$9$encrypted-key"; }
            server 10.0.0.2 { port 49; key "$9$encrypted-key"; }
            timeout 5
            accounting true
        }
        tacacs-profile 15 { profile [ admin ]; }
        tacacs-profile 1  { profile [ read-only ]; }
    }
}
```

| Leaf | Type | Default | Notes |
|------|------|---------|-------|
| `tacacs.server <ip>` | list, ordered-by-user | - | Failover order |
| `tacacs.server <ip>.port` | uint16 | 49 | TCP |
| `tacacs.server <ip>.key` | string (`ze:sensitive`) | required | Stored as `$9$` ciphertext |
| `tacacs.timeout` | uint16 (1-300) | 5 | Per-server connect timeout (s) |
| `tacacs.source-address` | ip-address | none | Local source IP for outbound TCP |
| `tacacs.authorization` | boolean | false | Per-command TACACS+ authorization |
| `tacacs.accounting` | boolean | false | START/STOP records on every CLI command |
| `tacacs-profile <N>.profile` | leaf-list | required | Map priv-lvl 0-15 to local authz profile name(s) |

<!-- source: internal/component/tacacs/yang/ze-tacacs-conf.yang -- system.authentication.tacacs -->
<!-- source: internal/component/tacacs/config.go -- ExtractConfig -->

## Sysctl Configuration

Kernel tunables are managed by the `sysctl` plugin via a generic key/value list:

```
sysctl {
    setting net.ipv4.conf.all.forwarding {
        value 1
    }
    setting net.core.somaxconn {
        value 4096
    }
}
```

Keys use kernel-native names (e.g., the `/proc/sys/` path with `/` replaced by `.`
on Linux, or MIB names on Darwin). Known keys get type and range validation on
commit; unknown keys are validated by attempting the write.

The sysctl plugin uses three-layer precedence: config values (above) are authoritative
and override both transient values (`set sysctl` from CLI) and plugin defaults
(e.g., fib-kernel declaring forwarding=1). See the
[command reference](../command-reference/index.md#sysctl-kernel-tunables) for CLI usage.
<!-- source: internal/component/sysctl/sysctl.go -- parseSysctlConfig, applyConfig -->
<!-- source: internal/component/sysctl/yang/ze-sysctl-conf.yang -- sysctl container -->

### Sysctl Profiles

Named profiles group co-dependent kernel tunables applied per interface unit.
Five built-in profiles cover common network operator use cases:

| Profile | Purpose | Keys set |
|---------|---------|----------|
| `dsr` | Direct Server Return ARP tuning | arp_announce=2, arp_ignore=1 |
| `router` | Enable IPv4/IPv6 forwarding | forwarding=1 (both) |
| `hardened` | Anti-spoofing | rp_filter=1, log_martians=1, arp_filter=1 |
| `multihomed` | Prevent ARP flux | arp_filter=1 |
| `proxy` | Proxy ARP | proxy_arp=1, arp_accept=1 |

Apply profiles to an interface unit:

```
interface {
    ethernet eth0 {
        unit default {
            sysctl-profile [ dsr hardened ]
        }
    }
}
```

Profiles emit as defaults: explicit `sysctl { setting ... }` config overrides them.
Multiple profiles per unit are composable; last wins on key overlap.

User-defined profiles are declared in the `sysctl` config block:

```
sysctl {
    profile my-edge {
        setting net.ipv4.conf.<iface>.forwarding {
            value 1
        }
        setting net.ipv4.conf.<iface>.rp_filter {
            value 2
        }
    }
}
```

The `<iface>` placeholder is substituted with the actual interface name at apply time.
<!-- source: internal/core/sysctl/profiles.go -- ProfileDef, builtinProfiles, ResolveProfileSettings -->
<!-- source: internal/component/iface/config_sysctl.go -- applySysctlProfiles -->

## Connection Tracking (Conntrack)

Declarative connection tracking management under `system { conntrack {} }`. Covers helper module loading, table sizing, protocol timeouts, and TCP behavior flags:

```
system {
    conntrack {
        module ftp
        module sip
        module tftp

        table-size 262144
        hash-size 65536
        expect-max 1024

        accounting
        timestamp
        log-invalid tcp

        timeout {
            generic 600
            tcp {
                established 432000
                close-wait 60
            }
            udp {
                timeout 30
                stream 120
            }
        }

        tcp {
            be-liberal false
            loose true
            max-retrans 3
        }
    }
}
```

**Helper modules:** Load kernel conntrack ALG helpers via `modprobe`. Valid modules: ftp, h323, sip, pptp, tftp, sane, irc, amanda, netbios-ns, snmp. Modules are load-only (removing from config stops loading on next boot but does not unload at runtime). On gokrazy, modules are built-in and loading is skipped.

**Dual-setting prevention:** Sysctl keys managed by conntrack (e.g., `nf_conntrack_max`) are rejected if also set in `sysctl { setting { ... } }`. The error message names the friendly conntrack config option to use instead.

**Monitoring:** Use `show system conntrack` to view current entry count, configured max, loaded modules, per-protocol timeouts, and TCP behavior flags.
<!-- source: internal/component/config/system/conntrack.go -- ConntrackConfig, sysctl key mapping -->
<!-- source: internal/component/config/system/yang/ze-system-conf.yang -- conntrack container -->

## Flow Export

Export interface counters and per-flow records to external collectors over UDP
via sFlow v5, NetFlow v9 (RFC 3954), or IPFIX (RFC 7011). Configured under a
single `flow-export { }` section. The component loads only when this section is
present. See the [Flow Export guide](../flow-export/index.md) for the full reference.

```
flow-export {
    collector edge-sflow {
        address 192.0.2.10;
        port 6343;             // default 6343
        protocol sflow;        // sflow | netflow9 | ipfix
        polling-interval 20;   // counter polling seconds, default 20
        agent-address 198.51.100.1;
        source-address 198.51.100.1;  // optional: bind UDP datagrams to this local IP
    }
    collector ipfix {
        address 192.0.2.21;
        port 4739;
        protocol ipfix;
        template-refresh 600;  // NetFlow v9 / IPFIX, default 600
        observation-domain 1;
    }
    sampling {
        interface eth0 {
            rate 1024;         // 1-in-N packets
            trunc-size 128;    // header bytes, default 128
            group 1;           // psample group, default 1
        }
    }
    conntrack {
        enabled true;
        active-timeout 60;     // seconds between table dumps, default 60
    }
    enrichment {
        bgp true;              // attach BGP next-hop to flow records
    }
}
```

**Operational:** `show flow export [<collector>]` reports per-collector
datagrams-sent, bytes-sent, errors, sequence, and last-export-time. Prometheus
metrics use the `ze_flowexport_*` prefix.

**Note:** these protocols are unencrypted UDP. Run flow export over a dedicated
management VLAN. Per-flow records are IPv4-only; sampling requires Linux with
`CAP_NET_ADMIN` and the kernel `psample` module.
<!-- source: internal/plugins/flowexport/yang/ze-flowexport-conf.yang -- flow-export container -->
<!-- source: internal/plugins/flowexport/cmd_show.go -- ze-show:flow-export RPC -->

## Traffic Usage

Account per-(port, protocol) and, optionally, per-IP byte totals on selected
interfaces using eBPF programs attached via TCX. Configured under a single
`traffic { usage { } }` section. The plugin loads only when this section is present
and is Linux only (no-op elsewhere). See the
[Traffic Usage guide](../traffic-usage/index.md) for the full reference.

```
traffic {
    usage {
        enabled true
        interfaces {             // keyed list, like ospf/isis; names are ze
            interface eth0 {     // interface names, resolved to the OS device
            }
            interface wan {
                track-ip false   // per-interface override of the global default
                max-entries 65536 // ... and a larger top-talker map for the WAN
            }
        }
        interval 1000            // map poll interval ms (global), default 1000, 100..3600000
        track-ip true            // global default, inherited per interface
        max-entries 10240        // global default per-map LRU capacity
    }
}
```

`stale-timeout` (default 300000 ms; 0 disables) removes metric series unseen for
that long, bounding `/metrics` cardinality. `track-ip`, `stale-timeout`, and
`max-entries` are global defaults; any interface may override them under its own
`interface { }` block, inheriting the global value when unset. See the
[Traffic Usage guide](../traffic-usage/index.md) for the full reference.

**Operational:** `show traffic usage [name <interface>]` reports per-interface
ingress/egress port totals (and per-IP totals when `track-ip` is on) plus map
fill levels. Prometheus metrics use the `ze_traffic_usage_*` prefix.

**Note:** accounting is IPv4 only and monitoring only (never drops or modifies
traffic). TCX requires Linux >= 6.6 with `CAP_BPF` and `CAP_NET_ADMIN`; a
`ze doctor` check (`doctor-traffic-usage-ebpf`) warns when enabled but eBPF/TCX
is unavailable.
<!-- source: internal/plugins/trafficusage/yang/ze-traffic-usage-conf.yang -- traffic-usage -->
<!-- source: internal/plugins/trafficusage/show.go -- ze-show:traffic-usage RPC -->

## Environment Block

Global settings outside BGP:

```
environment {
    tcp {
        attempts 3;        # connection retry attempts
    }
    log {
        level warn;        # default log level
        bgp.routes debug;  # per-subsystem override
    }
    cli {
        format {
            default text;  # default output format: text, table, json, yaml, ndjson
        }
        transcript enabled;  # record session commands and output to local file
    }
}
```

The `cli { format { default } }` leaf controls the output format when no explicit
pipe operator is specified. The default is `text`. Override per-session with
`set cli format <value>` in operational mode; explicit pipe operators always win.

The `cli { transcript }` leaf enables local transcript recording. When set to
`enabled`, `ze cli` and `ze config edit` sessions write all commands and their
output to `$XDG_DATA_HOME/ze/transcripts/` (defaults to
`~/.local/share/ze/transcripts/`). Transcripts include a header with
timestamp, username, and remote host. Transcript writes are best-effort and
never block CLI operation. Default is `disabled`.

<!-- source: internal/component/config/environment.go -- environment block parsing; internal/core/slogutil/slogutil.go -- log level config -->
<!-- source: internal/component/command/pipe.go -- configuredDefault -->
<!-- source: internal/component/cli/transcript.go -- TranscriptWriter, TranscriptEnabled -->

### TLS Certificates From the PKI Store

Three TLS listeners can serve a certificate held in the `pki {}` store instead
of a self-signed one: the web/API HTTPS listener, and the DoT/DoH listeners of
the as112 and geodns services. Each has a `certificate` leaf that names a
`pki { certificate <name> }` entry.

```
pki {
    ca corp-ca {
        certificate <base64-DER>;
    }
    certificate lan {
        certificate <base64-DER>;
        intermediate <base64-DER>;
        private {
            key <base64-DER>;
        }
    }
}

environment {
    web {
        enabled true;
        certificate lan;
    }
}

service {
    as112 {
        enabled true;
        tls {
            enabled true;
            certificate lan;
        }
    }
}
```

The listener sends the leaf certificate and every intermediate the store entry
holds, so a client can build the chain to the trust anchor. A store entry with
no intermediate serves the leaf alone.

| Rule | Detail |
|------|--------|
| Default | No `certificate` leaf means ze generates and serves a self-signed certificate, which is the behavior of every release before this leaf existed. |
| Fail closed | A `certificate` that names no store entry, or names one with no `private { key }`, stops the listener. ze never falls back to a self-signed certificate for a name the operator configured. Web: the daemon refuses to start, and a reload naming a bad certificate is rejected. DoT/DoH: the secure listeners do not start, the error is logged, and cleartext DNS is unaffected. |
| Mutually exclusive | In a `tls {}` container, `certificate` and `cert-file`/`key-file` are two sources of the same material. Setting both is rejected at commit. |
| Name | 1 to 255 characters, `A-Z a-z 0-9 . _ -`. It is a store key, never a file path. |
| Rotation | Changing the referenced certificate's material and reloading rotates it live. The web listener serves the new chain from the next handshake without rebinding, so open SSE streams survive. DoT/DoH rebind, because their listener signature folds in the certificate fingerprint. |
| One commit | A single commit can add a certificate AND reference it. The reload installs the store before any consumer applies its config. |
| Env override | `ze.web.certificate` sets the web certificate and takes precedence over the config file. |
| Pre-flight | `ze doctor` reports a reference that is missing, keyless, expired, or whose intermediate does not reach a configured CA, as `doctor-tls-reference` or `doctor-tls-expired`. Run it before deploying. |

An external geodns plugin process cannot read the in-process store: a
`certificate` reference there fails loudly and the secure listeners stay down.

<!-- source: internal/component/pki/tls.go -- ServerTLSMaterial, CheckCertReference -->
<!-- source: cmd/ze/hub/service_web.go -- webTLSMaterial (fail-closed selection) -->
<!-- source: cmd/ze/hub/listener_migrate.go -- UpdateWebCertificate (rotation seam) -->
<!-- source: internal/core/dnsserver/secure.go -- SecureConfig.Certificate, buildSecureTLS resolver branch -->

### Named Listeners

Every service that accepts inbound connections models its listen endpoints
as a named YANG list:

```
environment {
    web {
        enabled true;
        server primary {
            ip 0.0.0.0;
            port 3443;
        }
        server admin {
            ip 127.0.0.1;
            port 13443;
        }
    }
}
```

Each `server <name> { ... }` block becomes a bound TCP listener on the same
service. The same pattern applies to `environment.ssh`, `environment.mcp`,
`environment.looking-glass`, `telemetry.prometheus`, `environment.api-server.rest`,
`environment.api-server.grpc`, and `plugin.hub`.

<!-- source: internal/component/config/yang/modules/ze-types.yang — grouping listener -->
<!-- source: internal/component/config/yang/modules/ze-extensions.yang — extension listener -->
<!-- source: internal/component/config/loader_extract.go — ExtractWebConfig, ExtractLGConfig, ExtractMCPConfig, ExtractAPIConfig -->

Binder lifetime rules are uniform across every service:

| Rule | Detail |
|------|--------|
| Minimum | At least one entry is required when a service is enabled. An enabled block with no `server {}` uses the YANG `refine` default (e.g. `0.0.0.0:3443` for web). |
| Bind order | Entries are bound in config declaration order for live trees; in `map[string]any` roundtrips (after `ToMap()`) the ordering falls back to alphabetical key order. |
| Failure mode | Bind is all-or-nothing: if any listener fails to bind, the already-bound listeners are closed and the service fails to start. Partial binding is never accepted. |
| Shutdown | Every listener closes when the service is stopped; there is no "primary plus extras" asymmetry. |
| Insecure web | `insecure true` forces every entry's ip to `127.0.0.1` with a warning logged per rewrite. |
| MCP | MCP is localhost-only. Non-loopback entries are rewritten to `127.0.0.1` with a warning. |
| Prometheus | The implicit telemetry listener defaults to `127.0.0.1:9273`. Configure `server <name> { ip 0.0.0.0; ... }` explicitly when a remote scraper must connect. |

<!-- source: internal/component/web/server.go — WebServer.ListenAndServe multi-listener bind-and-rollback -->
<!-- source: internal/component/lg/server.go — LGServer.ListenAndServe multi-listener bind-and-rollback -->
<!-- source: internal/component/telemetry/exporter/server.go — exporter.Server.Start multi-listener bind-and-rollback (ze_telemetry) -->
<!-- source: internal/component/api/rest/server.go — RESTServer.ListenAndServe multi-listener bind-and-rollback -->
<!-- source: internal/component/api/grpc/server.go — GRPCServer.Serve multi-listener bind-and-rollback -->
<!-- source: cmd/ze/hub/service_mcp.go — startMCPServer multi-listener bind-and-rollback (ze_mcp) -->

#### Port Conflict Detection

`ze config validate` runs `CollectListeners` over every enabled service and
rejects the config if two listeners would bind overlapping `ip:port` pairs.
Wildcard addresses (`0.0.0.0`, `::`) conflict with any address in the same
family; cross-family entries (`0.0.0.0` vs `::1`) never conflict. The check
covers `web`, `ssh`, `mcp`, `looking-glass`, `prometheus`, `plugin.hub`,
`api-server.rest`, and `api-server.grpc`.

<!-- source: internal/component/config/listener.go — knownListenerServices, CollectListeners, ValidateListenerConflicts -->

#### Environment Variable Override

Each service exposes a `ze.<svc>.listen` variable that accepts one or more
comma-separated `ip:port` pairs. IPv6 entries use bracket notation:

| Variable | Example | Notes |
|----------|---------|-------|
| `ze.web.listen` | `0.0.0.0:3443,[::1]:3443` | Overrides web `server` list entirely |
| `ze.looking-glass.listen` | `0.0.0.0:8443` | Overrides LG `server` list |
| `ze.mcp.listen` | `127.0.0.1:8080,127.0.0.1:18080` | MCP enforces 127.0.0.1 on every entry |
| `ze.api-server.rest.listen` | `0.0.0.0:8081` | Overrides REST `server` list |
| `ze.api-server.grpc.listen` | `0.0.0.0:50051` | Overrides gRPC `server` list |

Env vars replace the config-file list when set; partial merging is not
supported so the precedence is predictable. Precedence per service is:
env var > CLI flag > config file > YANG default.

<!-- source: internal/component/config/environment.go — ParseCompoundListen -->
<!-- source: cmd/ze/hub/main.go — runYANGConfig env/CLI/config resolution for webAddrs, lgAddrs, mcpAddrs -->

### DNS Name Servers

Configure static DNS name servers and resolver tuning under `system {}`:

```
system {
    name-server [8.8.8.8 1.1.1.1]
    dns {
        resolv-conf-path /tmp/resolv.conf
        timeout 5
        cache-size 10000
        cache-ttl 86400
    }
}
```

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `name-server` | leaf-list ip-address | (none) | Static DNS servers. First server used by ze internal resolver. All written to resolv.conf at startup and after every config commit/reload. |
| `resolv-conf-path` | string | `/tmp/resolv.conf` | Path for resolv.conf output. Default suits gokrazy (read-only rootfs). Empty disables. |
| `timeout` | uint16 | `5` | Query timeout in seconds. Range: 1-60. |
| `cache-size` | uint32 | `10000` | Maximum cached entries. 0 disables caching entirely. |
| `cache-ttl` | uint32 | `86400` | Maximum cache entry TTL in seconds. 0 means use only the response TTL. |

The cache respects DNS response TTLs: entries expire at `min(response TTL, cache-ttl)`.
Records with TTL=0 from the server are not cached (per RFC 1035).

When static name servers are configured, DHCP-discovered DNS servers do not
overwrite resolv.conf. When no static servers are configured, DHCP writes DNS
to resolv-conf-path as before.

The internal resolver never falls back to a public recursive resolver. If no
static name server exists and the configured resolv.conf path is missing or
empty, DNS queries fail with `no DNS server configured` until DHCP or config
provides a server.

<!-- source: internal/component/config/system/yang/ze-system-conf.yang -- system DNS config -->
<!-- source: internal/component/resolve/dns/resolver.go -- NewResolver, Resolve -->
<!-- source: cmd/ze/hub/main.go -- newResolvers wiring -->

### Firmware Update Check and Self-Update

Configure periodic version checks and optional automated self-update under `system { update-check {} }`:

**Check-only (default, existing behavior):**

```
system {
    update-check {
        url https://update.example.com/version.json
    }
}
```

Daily check, report only. No download, no staging, no restart.

**Fully automated (fleet):**

```
system {
    update-check {
        url https://update.example.com/version.json
        interval 3600
        auto-apply true
        spread 1800
        maintenance-window {
            start 02:00
            end 06:00
        }
        restart {
            time 03:00
        }
    }
}
```

Check every hour, auto-download with up to 30 minutes random spread,
replace binary only between 2 AM and 6 AM, restart at 3 AM.

**Immediate restart (lab/single device):**

```
system {
    update-check {
        url https://update.example.com/version.json
        interval 300
        auto-apply true
        spread 0
        restart {
            immediate
        }
    }
}
```

Check every 5 minutes, no spread, no maintenance window, restart as soon as staged.

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `url` | string | (none) | HTTPS URL serving version manifest JSON. HTTP allowed only for localhost. |
| `interval` | uint32 | `86400` | Check interval in seconds. Range: 60-604800. |
| `auto-apply` | boolean | `false` | When true, automatically download, verify, and stage updates. When false, only check and report. |
| `spread` | uint32 | `3600` | Maximum random delay in seconds before downloading after a new version is detected. Prevents thundering herd. 0 disables. Range: 0-86400. |
| `maintenance-window start` | string | (none) | Start time HH:MM in local time. Binary replacement only occurs during the window. Download and verification proceed at any time. |
| `maintenance-window end` | string | (none) | End time HH:MM in local time. Windows crossing midnight are valid (e.g., 22:00-06:00). |
| `restart immediate` | empty | (none) | Restart automatically after staging (5s drain delay). Mutually exclusive with `restart time`. |
| `restart time` | string | (none) | Daily restart time HH:MM in local time. Mutually exclusive with `restart immediate`. |

When `auto-apply` is false (default), ze only checks for new versions and reports via `show system update` and `show warnings`. No binary is downloaded.

When `auto-apply` is true, the server manifest must include a `sha256` field. Ze refuses to auto-download binaries without a checksum. Manual commands (`update system firmware apply`) warn but proceed without verification.

See the [Self-Update Guide](../self-update/index.md) for server setup and fleet deployment details.

<!-- source: internal/component/config/system/yang/ze-system-conf.yang -- update-check config -->
<!-- source: internal/component/config/system/selfupdate.go -- SelfUpdater -->
<!-- source: cmd/ze/hub/main_system.go -- startUpdateChecker lifecycle -->

### NTP Client

Configure NTP under `environment { ntp { } }`:

```
environment {
    ntp {
        enabled true
        interval 300
        max-step 3600
        server pool0 {
            address 0.pool.ntp.org
        }
    }
}
```

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `enabled` | boolean | `false` | Enable NTP time synchronization. |
| `interval` | uint32 | `3600` | Sync interval in seconds. Range: 60-86400. |
| `max-step` | uint32 | `3600` | Maximum accepted NTP clock step in seconds. `0` explicitly allows unlimited steps. |
| `persist-path` | string | `/perm/ze/timefile` | Path used to persist recovered time across restarts. |
| `server <name>.address` | string | (none) | NTP server hostname or IP address. Configured servers take priority over DHCP option 42 servers. |

NTP responses are validated and timestamps outside years 2020-2100 are
rejected. `max-step` is checked before `settimeofday`; responses whose clock
offset exceeds the cap are rejected and logged.

<!-- source: internal/plugins/ntp/yang/ze-ntp-conf.yang -- NTP config schema -->
<!-- source: internal/plugins/ntp/ntp.go -- parseNTPConfig, doSync -->

### DHCP Server

Ze includes a built-in DHCP server plugin (RFC 2131/2132) for CPE deployments.

```
service {
    dhcp-server {
        enabled true;
        listen-interface br0;
        shared-network LAN {
            subnet 192.168.1.0/24 {
                range pool1 {
                    start 192.168.1.100;
                    stop  192.168.1.150;
                }
                range pool2 {
                    start 192.168.1.200;
                    stop  192.168.1.250;
                }
                lease-time 3600;
                default-router 192.168.1.1;
                dns-server 8.8.8.8;
                dns-server 8.8.4.4;
                domain-name home.lan;
                static-mapping printer {
                    mac-address aa:bb:cc:dd:ee:ff;
                    ip-address 192.168.1.10;
                }
            }
        }
    }
}
```

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `enabled` | boolean | `false` | Enable DHCP server. |
| `listen-interface` | leaf-list | (none) | Interfaces to serve DHCP on. |
| `shared-network <name>` | list | (none) | Named grouping of subnets. |
| `subnet <prefix>` | list | (none) | Subnet with address pool and options. |
| `range <name>` | list | (none) | Named dynamic address pool. Multiple ranges per subnet for disjoint pools. |
| `range <name>.start` | string | (none) | First allocatable address. |
| `range <name>.stop` | string | (none) | Last allocatable address. |
| `lease-time` | uint32 | `86400` | Lease duration in seconds (60-604800). |
| `default-router` | string | (none) | Default gateway (option 3). |
| `dns-server` | leaf-list | (none) | DNS servers (option 6). |
| `domain-name` | string | (none) | Domain name for clients (option 15). |
| `static-mapping <name>` | list | (none) | Static MAC-to-IP binding (excluded from dynamic allocation). |

Multiple named ranges allow disjoint address pools within a single subnet.
Ranges must not overlap and are allocated in order (first range fills before
the second is used). Up to 10 ranges per subnet are supported.

#### PXE Boot Support

The DHCP server supports PXE boot provisioning (RFC 4578). When enabled, the
server detects PXE clients via option 60 (`PXEClient:` prefix), reads client
architecture from option 93, and responds with the appropriate bootfile.

```
service {
    dhcp-server {
        enabled true;
        listen-interface eth0;
        pxe {
            enabled true;
            tftp-server 192.168.1.1;
            bootfile-bios ipxe.pxe;
            bootfile-uefi ipxe.efi;
        }
        shared-network install {
            subnet 192.168.1.0/24 {
                range pool1 {
                    start 192.168.1.100;
                    stop 192.168.1.200;
                }
                default-router 192.168.1.1;
            }
        }
    }
}
```

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `pxe.enabled` | boolean | `false` | Enable PXE boot option injection. |
| `pxe.tftp-server` | string | (none) | TFTP server IP for PXE boot (option 66, siaddr). Required when enabled. |
| `pxe.bootfile-bios` | string | (none) | Boot file path for BIOS clients (option 67). Required when enabled. |
| `pxe.bootfile-uefi` | string | (none) | Boot file path for UEFI clients (option 67). Required when enabled. |

Non-PXE clients receive standard DHCP responses regardless of PXE configuration.
When a PXE client has no option 93 (architecture), the server defaults to the
BIOS bootfile.

<!-- source: internal/plugins/dhcpserver/yang/ze-dhcp-server-conf.yang -- DHCP server config schema -->
<!-- source: internal/plugins/dhcpserver/config.go -- parseConfig, parseRanges, parsePXEConfig -->

### Reactor Settings

Configure reactor behavior under `environment { reactor { } }`:

```
environment {
    reactor {
        update-groups true;           # cross-peer UPDATE grouping (default: true)
        forward-queue-size 256;       # per-destination forward channel capacity
        forward-batch-limit 1024;     # max items per drain batch (0=unlimited)
        forward-pool-max-bytes 0;     # buffer pool byte budget (0=unlimited/auto)
        forward-pool-headroom 0;      # extra bytes beyond auto-sized baseline
        forward-teardown-grace 5s;    # congestion teardown grace period
        read-buffer-size 65536;       # TCP read buffer (bytes)
        write-buffer-size 16384;      # TCP write buffer (bytes)
    }
}
```

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `update-groups` | boolean | `true` | Enable cross-peer UPDATE grouping. When enabled, peers with identical encoding contexts share a single UPDATE build. |
| `forward-queue-size` | uint32 | `256` | Per-destination forward channel capacity (range: 1-1000000). |
| `forward-batch-limit` | uint32 | `1024` | Max items per drain batch, bounds write lock hold time (range: 0-1000000, 0=unlimited). |
| `forward-pool-max-bytes` | uint32 | `0` | Combined byte budget for 4K+64K buffer pools (0=unlimited, auto-sized from peer prefix maximums). |
| `forward-pool-headroom` | uint32 | `0` | Extra bytes beyond auto-sized pool baseline. Ignored when forward-pool-max-bytes is set. |
| `forward-teardown-grace` | string | `5s` | Grace period before forced teardown on congestion (duration, e.g. 5s, 1m). |
| `read-buffer-size` | uint32 | `65536` | Per-session TCP read buffer size in bytes (range: 4096-16777216). |
| `write-buffer-size` | uint32 | `16384` | Per-session TCP write buffer size in bytes (range: 4096-16777216). |

When `update-groups` is true (the default), the reactor groups established peers by their outbound encoding context (ContextID + policy). UPDATEs are built once per group and the wire bytes are fanned out to all group members. This reduces CPU usage proportionally to group size -- for a route server with 100 peers sharing the same capabilities, UPDATE building work is reduced by approximately 100x.

Disable update groups when:

- **ExaBGP compatibility:** migrated configs need per-peer UPDATE behavior matching ExaBGP's model. The `ze exabgp migrate` command automatically injects `update-groups false` into migrated configs.
- **Debugging:** isolating per-peer UPDATE building to diagnose encoding issues.

When disabled (or when all peers have unique encoding contexts), behavior is identical to per-peer building with negligible overhead.

#### Environment Variable Override

| Variable | Type | Description |
|----------|------|-------------|
| `ze.bgp.reactor.update-groups` | bool | Cross-peer UPDATE grouping (default: true) |

<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- reactor container, leaf update-groups -->
<!-- source: internal/component/config/environment.go -- ze.bgp.reactor.update-groups registration -->
<!-- source: internal/component/bgp/reactor/update_group.go -- NewUpdateGroupIndexFromEnv -->
<!-- source: internal/exabgp/migration/migrate.go -- injectUpdateGroupsDisabled -->

### L2TP Tunnels

L2TPv2 (RFC 2661) tunnel support uses two config blocks. Protocol settings
live under the root-level `l2tp {}` block. Listener endpoints follow the
standard named-server pattern under `environment { l2tp { } }`.

```
l2tp {
    enabled true;
    shared-secret "my-tunnel-secret";
    auth-method chap-md5;
    allow-no-auth false;
    hello-interval 60;
    max-tunnels 1000;
    max-sessions 100;
}

environment {
    l2tp {
        server main {
            ip 0.0.0.0;
            port 1701;
        }
    }
}
```
<!-- source: internal/component/l2tp/yang/ze-l2tp-conf.yang -- L2TP YANG schema -->

Protocol settings (root `l2tp {}`):

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `enabled` | boolean | `true` | Presence of `l2tp {}` implies enabled. Use `enabled false` to disable. Use `enabled true` as a filler when no other settings are needed. |
| `shared-secret` | string | (unset) | CHAP-MD5 challenge/response secret (RFC 2661 S4.2). If unset and a peer sends a Challenge AVP, the tunnel is rejected with StopCCN Result Code 4. |
| `auth-method` | enum | `chap-md5` | PPP Auth-Protocol first advertised in LCP. Values: `chap-md5`, `ms-chap-v2`, `pap`, `none`. `none` requires `allow-no-auth true`. |
| `allow-no-auth` | boolean | `false` | Allows PPP LCP to proceed without an Auth-Protocol after explicit operator opt-in. Default false disconnects peers that reject all configured authentication methods. |
| `hello-interval` | uint16 | (unset) | Seconds of peer silence before sending HELLO (1-3600). RFC 2661 recommends 60. |
| `max-tunnels` | uint16 | 1024 | Maximum concurrent tunnels. `0` explicitly requests unbounded admission by this knob. New SCCRQs beyond a positive limit receive StopCCN Result Code 2. |
| `max-sessions` | uint16 | 1024 | Maximum concurrent sessions per tunnel. `0` explicitly requests unbounded admission on each tunnel. New ICRQs/OCRQs beyond a positive limit receive CDN Result Code 4 (no resources). |

Listener endpoints (`environment { l2tp { } }`):

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `server <name>` | list | `0.0.0.0:1701` | Named UDP listen endpoints, same pattern as other services. |

PPP authentication (env var, YANG pending in spec-l2tp-7-subsystem):

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `ze.l2tp.auth.timeout` | duration | `30s` | Upper bound on the PPP authentication phase per session. If no auth handler responds within this window, the session fails closed (LCP Terminate + session down). Malformed values fall back to 30s with a WARN log. |

<!-- source: internal/component/l2tp/config.go -- ExtractParameters -->
<!-- source: internal/component/l2tp/subsystem.go -- L2TPSubsystem lifecycle -->
<!-- source: internal/component/l2tp/reactor.go -- handleKernelSuccess auth-timeout read -->

### PPPoE Access

PPPoE (RFC 2516) access concentrator configuration. PPPoE uses raw Ethernet
sockets on configured access interfaces, with no UDP listener needed.
Sessions use the same PPP Driver, auth, pool, and shaper plugins as L2TP.

```
pppoe {
    enabled true;
    ac-name "ze";
    service-name "internet";
    auth-method chap-md5;
    cookie-timeout 5;
    max-sessions 65535;
    padi-rate-limit 100;
    interface eth0 {
    }
    interface eth0.100 {
        service-name "vlan100-service";
        max-sessions 1000;
    }
}
```
<!-- source: internal/component/l2tp/pppoe/yang/ze-pppoe-conf.yang -- PPPoE YANG schema -->

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `enabled` | boolean | `true` | Presence of `pppoe {}` implies enabled. Use `enabled false` to disable. |
| `ac-name` | string | `ze` | Access Concentrator Name advertised in PADO (RFC 2516 S5.2). |
| `service-name` | leaf-list | (empty) | Accepted Service-Name values. Empty list means accept any Service-Name. |
| `auth-method` | enumeration | `chap-md5` | PPP Auth-Protocol the AC advertises in its own LCP Configure-Request: `none`, `pap`, `chap-md5`, `ms-chap-v2`. The credential is verified by `l2tp-auth-local` or `l2tp-auth-radius`. |
| `allow-no-auth` | boolean | `false` | Accept a subscriber whose LCP ends with no Auth-Protocol. Required beside `auth-method none`, which is otherwise refused at startup. |
| `cookie-timeout` | uint16 | 5 | AC-Cookie validity in seconds (1-300). Older cookies rejected in PADR. |
| `max-sessions` | uint16 | 65535 | Maximum concurrent PPPoE sessions per interface. |
| `padi-rate-limit` | uint16 | 100 | Maximum PADI packets per second per source MAC (1-10000). |
| `interface <name>` | list | (none) | Access interfaces for PPPoE discovery. Each gets independent SID space. |
| `interface / service-name` | leaf-list | (global) | Per-interface Service-Name filter, overrides global list. |
| `interface / max-sessions` | uint16 | (global) | Per-interface session limit, defaults to global max-sessions. |

<!-- source: internal/component/l2tp/pppoe/config.go -- ExtractParameters -->
<!-- source: internal/component/l2tp/pppoe/subsystem.go -- PPPoE subsystem lifecycle -->

### L2TP Address Pool

The `l2tp-pool` plugin provides IPv4 address allocation for PPP sessions.
A default pool and optional named pools are configured under `l2tp { pool { } }`.
Named pools are selected per-session via the RADIUS `Framed-Pool` attribute;
when `Framed-IP-Address` is present in Access-Accept, the pool is bypassed
and the RADIUS-assigned address is used directly.

```
l2tp {
    pool {
        ipv4 {
            gateway 10.255.0.1
            start 10.255.0.10
            end 10.255.0.254
            dns-primary 8.8.8.8
            dns-secondary 8.8.4.4
        }
        named-pool gold {
            gateway 10.255.1.1
            start 10.255.1.10
            end 10.255.1.254
            dns-primary 1.1.1.1
            dns-secondary 1.0.0.1
        }
    }
}
```

| Setting | Type | Description |
|---------|------|-------------|
| `gateway` | IPv4 address | NAS-side IP sent to the subscriber as the PPP peer address |
| `start` | IPv4 address | First allocatable address in the pool range |
| `end` | IPv4 address | Last allocatable address in the pool range |
| `dns-primary` | IPv4 address | Primary DNS server sent via IPCP |
| `dns-secondary` | IPv4 address | Secondary DNS server sent via IPCP |

Named pools use the same settings nested under `named-pool <name> { }`.
The gateway must not overlap the pool range.

<!-- source: internal/component/l2tp/plugins/pool/register.go -- parseIPv4Pool, parseNamedPools -->
<!-- source: internal/component/l2tp/plugins/pool/yang/ze-l2tp-pool-conf.yang -- YANG schema -->

## Hub Configuration

The plugin hub provides TLS transport for plugin communication and fleet management.
Named `server` blocks declare listeners; hub-level `client` blocks declare outbound
connections to a remote hub (managed mode).

```
plugin {
    hub {
        server local {
            ip 127.0.0.1;
            port 1790;
            secret "local-secret-minimum-32-characters";
        }
        server central {
            ip 0.0.0.0;
            port 1791;
            secret "central-secret-min-32-characters!";
            client edge-01 { secret "per-client-token-min-32-chars!!!"; }
        }
        client edge-01 {
            host 10.0.0.1;
            port 1791;
            secret "per-client-token-min-32-chars!!!";
            source-address 198.51.100.1;  // optional: bind outbound hub TLS to this local IP
        }
    }
}
```

Every ze instance has at least one `server` block (for local plugins and SSH).
Secrets must be at least 32 characters. See [Fleet Configuration](../fleet-config/index.md) for details.

<!-- source: internal/component/plugin/yang/ze-plugin-conf.yang -- hub YANG schema -->
<!-- source: internal/component/config/loader_extract.go -- ExtractHubConfig -->

## Outbound Source Address

Operators who deploy Ze behind loopback or management interfaces can pin the
local source address of a service's outbound connections. When `source-address`
is set, the socket binds to that local IP before connecting; when it is omitted,
the OS selects the source per its routing table (unchanged default behavior). The
address is validated as an `ip-address` at config time, and must be assigned to a
local interface or the connection fails with `cannot assign requested address`.

| Service | Config location | Leaf | Transport |
|---------|-----------------|------|-----------|
| BMP | `bgp bmp sender collector <name>` | `source-address` | TCP |
| RPKI/RTR | `bgp rpki cache-server <addr>` | `source-address` | TCP |
| Flow Export | `flow-export collector <name>` | `source-address` | UDP |
| IRR (filter) | `bgp policy irr` | `source-address` | TCP |
| Managed hub | `plugin hub client <name>` | `source-address` | TLS |
| LDP | `ldp` | `transport-address` | TCP |

LDP reuses its existing `transport-address` leaf: RFC 5036 defines the transport
address as the endpoint of the LDP TCP session, so Ze binds the outbound session
to it (rather than adding a separate `source-address`). When `transport-address`
is unset, the OS selects the source.

TACACS+ (`system authentication tacacs source-address`) and L2TP RADIUS already
bind an outbound source; BGP peers bind their mandatory per-peer `local ip`.
These predate this feature and keep their existing leaf names.

<!-- source: internal/core/network/network.go -- RealDialer.LocalAddr source binding -->
<!-- source: internal/component/bgp/plugins/bmp/sender.go -- BMP collector source-address -->
<!-- source: internal/component/bgp/plugins/rpki/rtr_session.go -- RTR source-address -->
<!-- source: internal/plugins/flowexport/sender.go -- flow-export UDP source bind -->
<!-- source: internal/component/resolve/irr/client.go -- IRR whois source-address -->
<!-- source: internal/component/managed/client.go -- managed hub TLS source-address -->
<!-- source: internal/plugins/ldp/register.go -- LDP transport-address binding -->

## Validation

Validate a configuration file without starting ze:

```
ze config validate myconfig.conf
```

Unknown keys are rejected with a suggestion for the closest valid key.

### Upgrading from a release that validated only on demand

The daemon now applies the same value rules at startup that `ze config validate`
applies, so a config that started a daemon yesterday can stop one today. Read
this before you upgrade.

**A value a registered validator refuses now stops the daemon.** Until this
release the rules ran in `ze config validate`, in the API commit hook and in the
web editor, and nowhere else. A file edited by hand and loaded with `ze start`
or `ze -` reached the protocol encoders unchecked. It no longer does: ze exits
with status 1, prints one line for each refused value, and binds nothing, so the
daemon does not half-start. The line names the section, the leaf and the rule:

```
error: load config: config validation failed: isis: isis/hostname: "café.example" is not a valid IS-IS hostname for isis/hostname: octet 4 is 0xc3. RFC 5301 section 3 encodes the value in 7-bit ASCII, so each character must be a printable ASCII character from space (0x20) to tilde (0x7e)
```

The rules cover `interface`, `sysctl`, `fib`, `plugin`, `telemetry`, `vpp`,
`vpn`, `pki`, `l2tp`, `isis` and `ospf`. A BGP value is unaffected here because
BGP has always had its own deeper check at startup. A missing mandatory field
stays a warning and does not stop the daemon.

**A reload refuses the same way, and the daemon keeps running.** Send SIGHUP
with a config a validator refuses and the reload fails, the staged candidate is
cleared, and the daemon keeps serving the config it already runs. The listeners
do not rebind and open sessions are not disturbed. The refusal is on stderr:

```
reload error: reload: parse config: config validation failed: isis: isis/net: "49.0001.00" is not a valid IS-IS NET for isis/net: length 4 octets, want 8..20
```

There is no override and no force flag. Fix the value and send SIGHUP again.

**Check before you upgrade.** Run `ze config validate` over the config first.
It runs the same walk over the same sections, so what it refuses is what the
daemon will refuse. It reports every refusal in one run, one line each, and it
starts nothing:

```
ze config validate /etc/ze/ze.conf
```

`ze doctor` loads the config the same way the daemon does, so a config a
validator refuses now stops doctor at `doctor-config-parse` with that same
message, and doctor's other checks do not run. That tells you the daemon will
not start. It does not tell you what else is wrong with the machine, so run
`ze config validate` first and `ze doctor` after the config is clean.

<!-- source: internal/component/config/validate_sections.go -- ValidateCustomSections, the one walk -->
<!-- source: internal/component/config/loader.go -- LoadConfig refusal -->
<!-- source: cmd/ze/hub/main_reload.go -- runReload keeps the running config on a load error -->
<!-- source: internal/component/doctor/doctor.go -- runChecks reports a load failure as doctor-config-parse and returns -->

## Healthcheck

Service healthcheck probes monitor availability and control BGP route announcement via watchdog groups:

```
bgp {
    healthcheck {
        probe dns {
            command "dig @127.0.0.1 example.com +short"
            group hc-dns
            interval 5
            rise 3
            fall 3
            withdraw-on-down false
            up-metric 100
            down-metric 1000
        }
    }
}
```

Each probe runs a shell command periodically. When the service is UP, routes in the watchdog group are announced with `up-metric` as MED. When DOWN, routes are either withdrawn (`withdraw-on-down true`) or re-announced with `down-metric` as MED (default). See [Healthcheck Guide](../bgp-healthcheck/index.md) for full configuration reference.
<!-- source: internal/component/bgp/plugins/healthcheck/yang/ze-healthcheck-conf.yang -- YANG schema -->

## Static Routes

Static routes are programmed directly to the kernel or VPP. The plugin
auto-loads when the config contains a `static { }` section. Routes are
grouped under named tables for policy-based routing support.

```
static {
    table default {
        route 0.0.0.0/0 {
            next {
                hop 10.0.0.1 { weight 3; }
                hop 10.0.0.2 { weight 1; }
            }
        }
        route 192.0.2.0/24 {
            blackhole { }
        }
    }
}
```

Named tables require a `routing-table` section mapping names to kernel
table IDs. Interface-only next-hops (no gateway) are supported for
point-to-point links. Multiple next-hops create an ECMP multipath route.
Weight controls traffic distribution (higher = more). BFD failover
removes a next-hop from the ECMP group on session DOWN and re-adds on UP.

See [Static Routes Guide](../static-routes/index.md) for named tables,
interface-only next-hops, mixed ECMP, BFD failover, blackhole/reject,
IPv6, and redistribute examples.
<!-- source: internal/plugins/static/yang/ze-static-conf.yang -- YANG schema -->
<!-- source: internal/plugins/static/register.go -- plugin registration -->

## Policy Routing

Policy-based routing steers traffic to alternate routing tables or
next-hops based on L3/L4 match criteria. Configured under `policy { route <name> { ... } }`.

See [Policy Routing Guide](../policy-routing/index.md) for configuration syntax,
match criteria, actions, reserved ranges, and CLI usage.
<!-- source: internal/plugins/policyroute/yang/ze-policyroute-conf.yang -- YANG schema -->
<!-- source: internal/plugins/policyroute/register.go -- plugin registration -->

## ExaBGP Migration

Ze auto-detects and migrates ExaBGP configuration files:

```
ze config migrate exabgp.conf > ze.conf
```
<!-- source: internal/component/config/cli/cmd_migrate.go -- cmdMigrate; internal/exabgp/migration/ -- ExaBGP config conversion -->
