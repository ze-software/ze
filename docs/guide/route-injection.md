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
next hop when selecting and forwarding it. `show bgp rib best` displays the
recovered next hop. `show bgp rib received` currently renders only the legacy
IPv4 NEXT_HOP attribute, so it omits MP next hops.

<!-- source: internal/component/bgp/plugins/rib/rib_commands.go -- injectRoute MP_REACH emission -->
<!-- source: internal/component/bgp/plugins/rib/rib_bestchange.go -- extractMPNextHopAddr -->
<!-- source: internal/component/bgp/rib/commit.go -- extended next-hop encoding -->


<!-- terminal-demo: rib-fib -->

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

### Next-Hop Self

`nhop self` resolves to the local address of each destination peer at wire time.

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
