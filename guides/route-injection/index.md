# Route Injection

Ze supports injecting routes at runtime through text, hex, or base64 encoded UPDATE commands. Routes can be sent from the CLI, from external plugins, or from process scripts.
<!-- source: internal/component/bgp/plugins/cmd/update/ -- update text/hex/b64 command parsing -->

## Injecting into the Local RIB

`request bgp rib inject` inserts a synthetic candidate into Ze's BGP RIB without
requiring a live BGP session. If the route wins best-path selection, it continues
through the system RIB and configured FIB backends.

```bash
ze cli -c "request bgp rib inject 192.0.2.10 ipv4/unicast 198.51.100.0/24 origin igp nexthop 127.0.0.1 med 42"
ze cli -c "show bgp rib best prefix 198.51.100.0/24"
ze cli -c "show rib"
```

Withdraw the same candidate with the matching synthetic peer, family, and prefix:

```bash
ze cli -c "request bgp rib withdraw 192.0.2.10 ipv4/unicast 198.51.100.0/24"
```

The `bgp-rib` and `rib` plugins provide best-path and system-RIB selection.
On Linux, the `fib-kernel` plugin programs selected routes through netlink.
<!-- source: internal/component/bgp/plugins/rib/rib_commands.go -- injectRoute, withdrawRoute -->
<!-- source: internal/component/sysrib/sysrib.go -- system RIB arbitration -->
<!-- source: internal/plugins/fib/kernel/fibkernel.go -- Linux FIB programming -->

### IPv6 next hops and extended next hop

An injected IPv6 route carries its IPv6 next hop in MP_REACH_NLRI. An injected
IPv4 unicast route can also use an IPv6 next hop when the destination peer
negotiated Extended Next Hop, the RFC 8950 capability historically defined by
RFC 5549.

```bash
ze cli -c "request bgp rib inject 192.0.2.10 ipv4/unicast 198.51.100.0/24 origin igp nexthop 2001:db8::1"
ze cli -c "show bgp rib best prefix 198.51.100.0/24"
```

Ze stores the MP_REACH attribute with the synthetic candidate and recovers the
next hop when selecting and forwarding it. `show bgp rib best` and
`show bgp rib received` both display the recovered next hop: the attribute
renderer falls back to the stored MP_REACH_NLRI whenever the legacy IPv4-only
NEXT_HOP attribute (type 3) is absent, which is the case for every IPv6 next
hop, native or extended.

<!-- source: internal/component/bgp/plugins/rib/rib_commands.go -- injectRoute MP_REACH emission -->
<!-- source: internal/component/bgp/plugins/rib/rib_attr_format.go -- enrichRouteMapFromEntry MP_REACH next-hop fallback -->
<!-- source: internal/component/bgp/plugins/rib/rib_bestchange.go -- extractMPNextHopAddr -->
<!-- source: internal/component/bgp/rib/commit.go -- extended next-hop encoding -->


### Demo: Follow a route from BGP RIB to Linux FIB

Inject one route, inspect BGP best-path selection, and verify Linux installed it with Ze's route protocol ID. Validation also proves withdrawal removes it.

[Download the asciicast recording](../../assets/demos/rib-fib.cast?v=908cbeb9d1) · [Plain-text transcript](../../assets/demos/rib-fib.txt?v=ca05c09bc8)

Recorded with Ze 26.08.25 in a Linux namespace lab using Ze recorder. Duration: 49 seconds.

```console
$ ze cli -c 'request bgp rib inject 192.0.2.10 ipv4/unicast 198.51.100.0/24 origin igp nexthop 127.0.0.1 med 42'
$ ze cli -c 'show bgp rib best prefix 198.51.100.0/24 | no-more'
198.51.100.0/24
$ ip -details route show exact 198.51.100.0/24
198.51.100.0/24 ... proto 250

The route enters Ze's BGP RIB, wins best-path selection, reaches the protocol-independent system RIB, and is programmed into Linux with Ze's route protocol ID. The validator withdraws it and confirms kernel removal.
```


## Text Format

Human-readable format with flat attribute declarations:

```bash
ze cli -c "peer upstream1 update text \
    origin igp \
    nhop 192.168.1.1 \
    local-preference 200 \
    as-path [ 65001 65002 ] \
    community [ 65000:100 no-export ] \
    nlri ipv4/unicast add 10.0.0.0/24 10.0.1.0/24"
```

### Attribute Keywords

Attributes are flat: keyword followed by value. No `set`/`add`/`del` on attributes.
`add` and `del` are NLRI-only operations (announce vs. withdraw).

| Attribute | Syntax | Delete |
|-----------|--------|--------|
| `origin` | `origin igp` / `egp` / `incomplete` | -- |
| `nhop` | `nhop 192.168.1.1` or `nhop self` | -- |
| `med` | `med 100` | -- |
| `local-preference` | `local-preference 200` | -- |
| `as-path` | `as-path [ 65000 65001 ]` | -- |
| `community` | `community [ 65000:100 no-export ]` | -- |
| `large-community` | `large-community [ 65000:1:1 ]` | -- |
| `extended-community` | `extended-community [ rt:65000:100 ]` | -- |

### Well-Known Communities

All IANA-registered well-known communities are accepted by name:

| Name | Value | Reference |
|------|-------|-----------|
| `no-export` | 0xFFFFFF01 | RFC 1997 |
| `no-advertise` | 0xFFFFFF02 | RFC 1997 |
| `no-export-subconfed` | 0xFFFFFF03 | RFC 1997 |
| `nopeer` | 0xFFFFFF04 | RFC 3765 |
| `graceful-shutdown` | 0xFFFF0000 | RFC 8326 |
| `accept-own` | 0xFFFF0001 | RFC 7611 |
| `route-filter-translated-v4` | 0xFFFF0002 | IANA |
| `route-filter-v4` | 0xFFFF0003 | IANA |
| `route-filter-translated-v6` | 0xFFFF0004 | IANA |
| `route-filter-v6` | 0xFFFF0005 | IANA |
| `llgr-stale` | 0xFFFF0006 | RFC 9494 |
| `no-llgr` | 0xFFFF0007 | RFC 9494 |
| `accept-own-nexthop` | 0xFFFF0008 | IANA |
| `standby-pe` | 0xFFFF0009 | RFC 9026 |
| `blackhole` | 0xFFFF029A | RFC 7999 |

Underscore variants (e.g., `no_export`, `graceful_shutdown`) and the shorthand
`gshut` are also accepted. Tab completion in the config editor suggests all names.
<!-- source: internal/core/bgp/attribute/text.go -- wellKnownCommunityNames map -->

### Colon-Less FlowSpec Actions

Most extended communities carry their value after a colon, so `extended-community`
splits on the first one. Three FlowSpec traffic actions (RFC 8955 Section 7) carry
their whole meaning in the name and have no colon. They are accepted as written,
in `update text` and in config alike, from one keyword table both parsers read.

| Keyword | Encoding | Meaning |
|---------|----------|---------|
| `discard` | type 0x80, subtype 0x06, traffic-rate 0 | Drop matching traffic |
| `copy-to-nexthop` | type 0x08, subtype 0x00, value 1 | Copy and redirect to next hop |
| `redirect-to-nexthop-draft` | type 0x08, subtype 0x00, value 0 | Redirect to next hop, pre-IETF draft |

```bash
ze cli -c "peer upstream update text \
    origin igp nhop self \
    extended-community [ discard ] \
    nlri ipv4/flow add <flowspec-nlri>"
```

`copy-to-nexthop` and `redirect-to-nexthop-draft` differ in one bit of the last
octet, which is the copy semantic. An unknown keyword is refused with an error
naming the keywords Ze accepts, derived from the table.
<!-- source: internal/core/bgp/attribute/flowspec_action.go -- FlowSpecActionKeyword, FlowSpecActionKeywords -->

### One Encoder Per FlowSpec Action

The actions that DO carry a value -- traffic-rate, traffic-action,
traffic-marking, rt-redirect and redirect-to-IP -- are built in one place as
well. Config, `update text` and the FlowSpec NLRI plugin call the same
constructors, so the three surfaces accept the same values and put the same
octets on the wire. Three copies of this vocabulary disagreed before: one wrote
an out-of-range DSCP into the reserved bits, one dropped a zero-valued action,
and one refused a 4-octet AS in a redirect the others encoded.
<!-- source: internal/core/bgp/attribute/flowspec_encode.go -- FlowSpecTrafficRate, FlowSpecTrafficAction, FlowSpecTrafficMarking, FlowSpecRedirect -->

### Next-Hop Self

`nhop self` resolves to the local address of each destination peer at wire time.

### A Peer Never Receives Its Own Address as Next Hop

RFC 4271 Section 5.1.3: "A route originated by a BGP speaker SHALL NOT be
advertised to a peer using an address of that peer as NEXT_HOP." Ze asks that
question of the built body, after the export filter chain, on every rail that
originates a next hop: configured static routes, default-originate, the RIB
op-queue drain, the announce batch, and the RFC 9494 stale re-advertise.

The route is withheld from that one peer, and the refusal logs the peer, the next
hop and the section. Ze does not rewrite the address: rewriting would invent a
next hop nobody configured. Other peers matched by the same selector still
receive the route, and withdrawals in the same UPDATE still reach the withheld
peer.

The same question is asked of a relayed route, where the address arrives as the
third-party next hop Section 5.1.3 case 2 permits.
<!-- source: internal/component/bgp/reactor/forward_next_hop.go -- originatedNextHopIsPeerOwn, egressNextHopIsPeerOwn -->

### NLRI Operations

`add` and `del` are NLRI operations (MP_REACH and MP_UNREACH):

```bash
nlri ipv4/unicast add 10.0.0.0/24 10.0.1.0/24    # Announce prefixes
nlri ipv4/unicast del 10.0.2.0/24                  # Withdraw prefix
nlri ipv4/unicast eor                               # End-of-RIB marker
```

Multiple families in one command:

```bash
nlri ipv4/unicast add 10.0.0.0/24 \
nlri ipv6/unicast add 2001:db8::/32
```

## Hex Format

Wire-encoded bytes for debugging or replay:

```bash
ze cli -c "peer upstream1 update hex \
    attr set 40010100400200400304c0a80101 \
    nhop set c0a80101 \
    nlri ipv4/unicast add 180a0000"
```

## Base64 Format

Compact encoding for scripts:

```bash
ze cli -c "peer upstream1 update b64 \
    attr set QAEBAAQDAsCoBQE= \
    nlri ipv4/unicast add GAoAAA=="
```

## Peer Selector

Routes are sent to peers matching the selector:

| Selector | Example | Description |
|----------|---------|-------------|
| `*` | `peer *` | All peers |
| Name | `peer upstream1` | By configured peer name |
| IP address | `peer 10.0.0.1` | Exact peer IP |
| Glob | `peer 192.168.*.*` | Pattern match |
| Exclusion | `peer !10.0.0.1` | All except this peer |

<!-- source: internal/component/bgp/plugins/cmd/raw/ -- raw message injection -->

## Commit Workflow

For atomic multi-route updates:

```bash
ze cli -c "request commit start my-batch"
ze cli -c "peer * update text nhop 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24"
ze cli -c "peer * update text nhop 10.0.0.1 nlri ipv4/unicast add 10.0.1.0/24"
ze cli -c "request commit end my-batch"    # All routes sent together
```
<!-- source: internal/component/bgp/plugins/cmd/commit/ -- commit command RPCs; internal/component/bgp/transaction/ -- commit manager -->

## From Plugins

External plugins send routes through the SDK:

```python
from ze_api import API

api = API()
# ... 5-stage startup ...
api.send("peer * update text nhop 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24")
```

Go plugins use the SDK method:

```go
p.UpdateRoute(ctx, "*", "update text nhop 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24")
```
<!-- source: pkg/plugin/sdk/ -- SDK DispatchCommand; test/scripts/ze_api.py -- Python SDK -->
